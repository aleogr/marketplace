package identity

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// Sealer seals what is about a person with that person's own key, and opens
// it again ((*audit.Keys).SealFor and OpenFor). Destroying the key for an
// erasure request makes what it sealed unreadable (F14 spec, D5).
type Sealer interface {
	SealFor(ctx context.Context, tx pgx.Tx, person string, plaintext []byte) ([]byte, error)
	OpenFor(ctx context.Context, tx pgx.Tx, person string, sealed []byte) ([]byte, error)
}

// WithSealer returns the service sealing second factors' secrets with sealer.
// Without one, adding or answering with an authenticator app is an error.
func (s *Service) WithSealer(sealer Sealer) *Service {
	s.sealer = sealer
	return s
}

// MaxLabelLength bounds what a person calls a second factor, in Unicode code
// points after trimming, as MaxNameLength bounds a name.
const MaxLabelLength = 60

var (
	// ErrNotPermitted is a method the policy does not permit this account
	// for this use (docs/requirements.md, section 18.2).
	ErrNotPermitted = errors.New("identity: method not permitted for this account")
	// ErrCodeWrong is a code that does not prove the factor: a wrong or
	// spent app code, or a code of another step.
	ErrCodeWrong = errors.New("identity: code wrong")
	// ErrLabelInvalid is a label longer than MaxLabelLength or holding a
	// control or format character.
	ErrLabelInvalid = errors.New("identity: label too long or not plain text")
	// ErrFactorUnknown is a factor the account does not have.
	ErrFactorUnknown = errors.New("identity: no such second factor")
	// ErrFactorRequired is removing the last factor of an account the
	// policy requires to have one.
	ErrFactorRequired = errors.New("identity: this account must keep a second factor")
	// ErrNoSecondFactor is asking for what only an account with a second
	// factor has, such as recovery codes.
	ErrNoSecondFactor = errors.New("identity: the account has no second factor")

	errNoSealer = errors.New("identity: no sealer for second factors' secrets")
)

// CheckLabel returns what a person called a factor, trimmed, or
// ErrLabelInvalid. An empty label is kept: the page then names the factor by
// its kind.
func CheckLabel(label string) (string, error) {
	label = strings.TrimSpace(label)
	if !utf8.ValidString(label) || utf8.RuneCountInString(label) > MaxLabelLength {
		return "", ErrLabelInvalid
	}
	for _, r := range label {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", ErrLabelInvalid
		}
	}
	return label, nil
}

// Factor is a second factor as the security page shows it.
type Factor struct {
	ID         string
	Method     Method
	Label      string
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

// Security is what the security page shows about an account.
type Security struct {
	Factors []Factor
	// RecoveryIssued and RecoveryLeft count the current set of recovery
	// codes: how many were issued and how many are unused.
	RecoveryIssued, RecoveryLeft int
}

// Security reads an account's second factors and recovery codes.
func (s *Service) Security(ctx context.Context, v Visit, session Session) (Security, error) {
	var security Security
	err := s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		factors, err := factorsOf(ctx, tx, session.Account.ID)
		if err != nil {
			return err
		}
		for _, f := range factors {
			security.Factors = append(security.Factors, Factor{ID: f.ID, Method: f.Method, Label: f.Label,
				CreatedAt: f.CreatedAt, LastUsedAt: f.LastUsedAt})
		}
		security.RecoveryIssued, security.RecoveryLeft, err = recoveryCodes(ctx, tx, session.Account.ID)
		return err
	})
	return security, err
}

// AppEnrolment is an authenticator app being added: the secret as a person
// types it, and the otpauth URI the QR code carries.
type AppEnrolment struct {
	Key string
	URI string
}

func (s *Service) appEnrolment(v Visit, session Session, secret []byte) AppEnrolment {
	return AppEnrolment{Key: TOTPKey(secret), URI: TOTPURI(v.MarketplaceName, session.Account.Email, secret)}
}

// BeginApp starts adding an authenticator app to the session's account, and
// returns what the page shows. The secret is sealed with the person's key
// from the moment it exists (D5); the app becomes a second factor only when
// ConfirmApp receives a code it made.
func (s *Service) BeginApp(ctx context.Context, v Visit, session Session) (AppEnrolment, error) {
	if !s.policy.Permits(session.Account.Kind, MethodApp, Enrol) {
		return AppEnrolment{}, ErrNotPermitted
	}
	if s.sealer == nil {
		return AppEnrolment{}, errNoSealer
	}
	secret, err := NewTOTPSecret()
	if err != nil {
		return AppEnrolment{}, err
	}
	err = s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		sealed, err := s.sealer.SealFor(ctx, tx, session.Account.ID, secret)
		if err != nil {
			return err
		}
		return putEnrolment(ctx, tx, v.Marketplace, session.ID, session.Account.ID, MethodApp, sealed, nil,
			s.now().Add(EnrolmentLifetime))
	})
	if err != nil {
		return AppEnrolment{}, err
	}
	return s.appEnrolment(v, session, secret), nil
}

// PendingApp returns the app enrolment the session has in progress, so the
// page can be shown again after a wrong code; ErrNoEnrolment when there is
// none, or it expired.
func (s *Service) PendingApp(ctx context.Context, v Visit, session Session) (AppEnrolment, error) {
	if s.sealer == nil {
		return AppEnrolment{}, errNoSealer
	}
	var secret []byte
	err := s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		e, err := enrolmentOf(ctx, tx, session.ID, MethodApp, s.now())
		if err != nil {
			return err
		}
		secret, err = s.sealer.OpenFor(ctx, tx, session.Account.ID, e.Secret)
		return err
	})
	if err != nil {
		return AppEnrolment{}, err
	}
	return s.appEnrolment(v, session, secret), nil
}

// ConfirmApp makes the app the session is adding a second factor, once code
// proves it was set up, and returns the recovery codes when this is the
// account's first app or key: they are shown once and never again. A wrong
// code is ErrCodeWrong and leaves the enrolment in place for another try.
func (s *Service) ConfirmApp(ctx context.Context, v Visit, session Session, label, code string) ([]string, error) {
	if !s.policy.Permits(session.Account.Kind, MethodApp, Enrol) {
		return nil, ErrNotPermitted
	}
	if s.sealer == nil {
		return nil, errNoSealer
	}
	label, err := CheckLabel(label)
	if err != nil {
		return nil, err
	}
	codes, hashes, err := newRecoverySet()
	if err != nil {
		return nil, err
	}
	account := session.Account.ID
	var issued bool
	var wrong bool
	err = s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		now := s.now()
		e, err := enrolmentOf(ctx, tx, session.ID, MethodApp, now)
		if err != nil {
			return err
		}
		secret, err := s.sealer.OpenFor(ctx, tx, account, e.Secret)
		if err != nil {
			return err
		}
		step, ok := matchTOTP(secret, code, now, 0)
		if !ok {
			wrong = true
			return nil
		}
		if err := insertApp(ctx, tx, v.Marketplace, account, label, e.Secret, step, now); err != nil {
			return err
		}
		if err := dropEnrolment(ctx, tx, session.ID); err != nil {
			return err
		}
		if issued, err = s.firstRecoverySet(ctx, tx, v, account, hashes); err != nil {
			return err
		}
		return s.recordWith(ctx, tx, v, account, "identity.second_factor_added", map[string]string{"method": string(MethodApp)})
	})
	switch {
	case err != nil:
		return nil, err
	case wrong:
		return nil, ErrCodeWrong
	case !issued:
		return nil, nil
	}
	return codes, nil
}

// newRecoverySet makes a set of recovery codes and the hashes that are all
// that is stored of them.
func newRecoverySet() ([]string, [][]byte, error) {
	codes, err := NewRecoveryCodes()
	if err != nil {
		return nil, nil, err
	}
	hashes := make([][]byte, len(codes))
	for i, code := range codes {
		hashes[i], _ = recoveryHash(code)
	}
	return codes, hashes, nil
}

// firstRecoverySet stores hashes as the account's recovery codes when it has
// none yet, which is the first app or key it adds (§18.2), and reports whether
// it did.
func (s *Service) firstRecoverySet(ctx context.Context, tx pgx.Tx, v Visit, account string, hashes [][]byte) (bool, error) {
	issued, _, err := recoveryCodes(ctx, tx, account)
	if err != nil || issued > 0 {
		return false, err
	}
	return true, replaceRecoveryCodes(ctx, tx, v.Marketplace, account, hashes)
}

// RemoveFactor removes one of the session's account's second factors.
// Removing the last one turns two-factor authentication off and deletes the
// recovery codes, unless the policy requires the account to keep one.
func (s *Service) RemoveFactor(ctx context.Context, v Visit, session Session, id string) error {
	account := session.Account.ID
	return s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		removed, err := deleteFactor(ctx, tx, account, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrFactorUnknown
		}
		if err != nil {
			return err
		}
		left, err := countFactors(ctx, tx, account)
		if err != nil {
			return err
		}
		if left == 0 {
			if s.policy.Required(session.Account.Kind) {
				return ErrFactorRequired
			}
			if err := deleteRecoveryCodes(ctx, tx, account); err != nil {
				return err
			}
		}
		return s.recordWith(ctx, tx, v, account, "identity.second_factor_removed",
			map[string]string{"method": string(removed.Method)})
	})
}

// RegenerateRecoveryCodes replaces the account's recovery codes with a fresh
// set and returns it, to be shown once: the old codes stop working.
func (s *Service) RegenerateRecoveryCodes(ctx context.Context, v Visit, session Session) ([]string, error) {
	codes, hashes, err := newRecoverySet()
	if err != nil {
		return nil, err
	}
	account := session.Account.ID
	err = s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		n, err := countFactors(ctx, tx, account)
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNoSecondFactor
		}
		if err := replaceRecoveryCodes(ctx, tx, v.Marketplace, account, hashes); err != nil {
			return err
		}
		return s.record(ctx, tx, v, account, "identity.recovery_codes_regenerated")
	})
	if err != nil {
		return nil, err
	}
	return codes, nil
}

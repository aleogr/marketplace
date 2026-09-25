package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5"
)

// ErrKeyRefused is a key's answer that does not verify: made for another
// site or another challenge, malformed, or a signature that fails.
var ErrKeyRefused = errors.New("identity: the key's answer was refused")

// relyingParty is WebAuthn as a visit's marketplace speaks it (F14 spec, D1,
// D6): its host is the relying party, so a credential enrolled on one
// marketplace never works on another, which is what accounts belonging to a
// marketplace mean (F13, D2). Any authenticator is accepted: no attestation
// is asked for, and user verification is preferred, not required. It is a
// second factor, so no discoverable credential is asked for either.
func relyingParty(v Visit) (*webauthn.WebAuthn, error) {
	base, err := url.Parse(v.BaseURL)
	if err != nil || base.Hostname() == "" {
		return nil, fmt.Errorf("identity: %q names no host to be the relying party", v.BaseURL)
	}
	return webauthn.New(&webauthn.Config{
		RPID:                  base.Hostname(),
		RPDisplayName:         v.MarketplaceName,
		RPOrigins:             []string{v.BaseURL},
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementDiscouraged,
			UserVerification: protocol.VerificationPreferred,
		},
	})
}

// keyUser is an account as WebAuthn sees it: its id, its names, and the keys
// it already has.
type keyUser struct {
	account     Account
	credentials []webauthn.Credential
}

func (u keyUser) WebAuthnID() []byte                         { return []byte(u.account.ID) }
func (u keyUser) WebAuthnName() string                       { return u.account.Email }
func (u keyUser) WebAuthnDisplayName() string                { return u.account.Name }
func (u keyUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

// keyCredential is a stored key as WebAuthn checks it.
func keyCredential(f factor) webauthn.Credential {
	return webauthn.Credential{
		ID:        f.CredentialID,
		PublicKey: f.PublicKey,
		Flags:     webauthn.NewCredentialFlags(protocol.AuthenticatorFlags(f.Flags)), // #nosec G115 -- stored from one byte.
		Authenticator: webauthn.Authenticator{
			SignCount: uint32(f.SignCount), // #nosec G115 -- stored from a uint32.
		},
	}
}

// keysOf is an account and its keys, as WebAuthn wants them.
func keysOf(ctx context.Context, tx pgx.Tx, account Account) (keyUser, []factor, error) {
	factors, err := factorsOf(ctx, tx, account.ID)
	if err != nil {
		return keyUser{}, nil, err
	}
	var keys []factor
	for _, f := range factors {
		if f.Method == MethodKey {
			keys = append(keys, f)
		}
	}
	return keyUserOf(account, keys), keys, nil
}

// keyUserOf is an account with the keys given, as WebAuthn wants it.
func keyUserOf(account Account, keys []factor) keyUser {
	user := keyUser{account: account}
	for _, key := range keys {
		user.credentials = append(user.credentials, keyCredential(key))
	}
	return user
}

// BeginKey starts adding a security key or the device's own authenticator,
// and returns the options the browser's navigator.credentials.create takes,
// as JSON. Keys the account already has are excluded, so one authenticator
// is not added twice.
func (s *Service) BeginKey(ctx context.Context, v Visit, session Session) ([]byte, error) {
	if !s.policy.Permits(session.Account.Kind, MethodKey, Enrol) {
		return nil, ErrNotPermitted
	}
	if s.NeedsStepUp(session, ActionFactors) {
		return nil, ErrStepUpNeeded
	}
	rp, err := relyingParty(v)
	if err != nil {
		return nil, err
	}
	var options []byte
	err = s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		user, _, err := keysOf(ctx, tx, session.Account)
		if err != nil {
			return err
		}
		creation, data, err := rp.BeginRegistration(user,
			webauthn.WithExclusions(webauthn.Credentials(user.credentials).CredentialDescriptors()))
		if err != nil {
			return err
		}
		ceremony, err := json.Marshal(data)
		if err != nil {
			return err
		}
		if options, err = json.Marshal(creation); err != nil {
			return err
		}
		return putEnrolment(ctx, tx, v.Marketplace, session.ID, session.Account.ID, MethodKey, nil, ceremony,
			s.now().Add(EnrolmentLifetime))
	})
	return options, err
}

// ConfirmKey adds the key whose registration the browser answered with,
// once it verifies against the ceremony BeginKey started, and returns the
// recovery codes when it is the account's first app or key.
func (s *Service) ConfirmKey(ctx context.Context, v Visit, session Session, label string, response []byte) ([]string, error) {
	if !s.policy.Permits(session.Account.Kind, MethodKey, Enrol) {
		return nil, ErrNotPermitted
	}
	if s.NeedsStepUp(session, ActionFactors) {
		return nil, ErrStepUpNeeded
	}
	label, err := CheckLabel(label)
	if err != nil {
		return nil, err
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(response)
	if err != nil {
		s.log.InfoContext(ctx, "a key's registration could not be read", "error", err)
		return nil, ErrKeyRefused
	}
	rp, err := relyingParty(v)
	if err != nil {
		return nil, err
	}
	codes, hashes, err := newRecoverySet()
	if err != nil {
		return nil, err
	}
	account := session.Account.ID
	var issued, refused bool
	err = s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		now := s.now()
		e, err := enrolmentOf(ctx, tx, session.ID, MethodKey, now)
		if err != nil {
			return err
		}
		var ceremony webauthn.SessionData
		if err := json.Unmarshal(e.WebAuthnSession, &ceremony); err != nil {
			return err
		}
		user, _, err := keysOf(ctx, tx, session.Account)
		if err != nil {
			return err
		}
		credential, err := rp.CreateCredential(user, ceremony, parsed)
		if err != nil {
			s.log.InfoContext(ctx, "a key's registration was refused", "error", err)
			refused = true
			return nil
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO second_factor (account_id, marketplace_id, kind, label, credential_id, public_key,
			                           sign_count, credential_flags, created_at)
			VALUES ($1, $2, 'webauthn', $3, $4, $5, $6, $7, $8)`,
			account, v.Marketplace, label, credential.ID, credential.PublicKey,
			int64(credential.Authenticator.SignCount), int16(credential.Flags.ProtocolValue()), now); err != nil {
			return err
		}
		if err := dropEnrolment(ctx, tx, session.ID); err != nil {
			return err
		}
		if issued, err = s.firstRecoverySet(ctx, tx, v, account, hashes); err != nil {
			return err
		}
		return s.recordWith(ctx, tx, v, account, "identity.second_factor_added", map[string]string{"method": string(MethodKey)})
	})
	switch {
	case err != nil:
		return nil, err
	case refused:
		return nil, ErrKeyRefused
	case !issued:
		return nil, nil
	}
	return codes, nil
}

// KeyOptions returns the options the browser's navigator.credentials.get
// takes to answer the challenge a token opened with a key, as JSON, and keeps
// the ceremony with the challenge: the answer must carry it.
func (s *Service) KeyOptions(ctx context.Context, v Visit, token, sessionID string) ([]byte, error) {
	rp, err := relyingParty(v)
	if err != nil {
		return nil, err
	}
	var options []byte
	err = s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		_, account, err := s.challenge(ctx, tx, token, sessionID)
		if err != nil {
			return err
		}
		user, keys, err := keysOf(ctx, tx, account)
		if err != nil {
			return err
		}
		if len(keys) == 0 || !s.policy.Permits(account.Kind, MethodKey, Verify) {
			return ErrNotPermitted
		}
		assertion, data, err := rp.BeginLogin(user)
		if err != nil {
			return err
		}
		ceremony, err := json.Marshal(data)
		if err != nil {
			return err
		}
		if options, err = json.Marshal(assertion); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE sign_in_challenge SET webauthn_session = $2 WHERE token_hash = $1`,
			HashToken(token), ceremony)
		return err
	})
	return options, err
}

// checkKey accepts a key's answer to a challenge. A signature counter that
// did not move forward is refused and audited as a possible clone (D6). The
// account's keys are locked before the counter is compared, so answers to two
// challenges are checked one after the other, each against the counter the
// other left.
func (s *Service) checkKey(ctx context.Context, tx pgx.Tx, v Visit, account Account, ch challenge, response []byte, now time.Time) (bool, error) {
	if len(ch.WebAuthnSession) == 0 {
		return false, nil
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(response)
	if err != nil {
		s.log.InfoContext(ctx, "a key's answer could not be read", "error", err)
		return false, nil
	}
	var ceremony webauthn.SessionData
	if err := json.Unmarshal(ch.WebAuthnSession, &ceremony); err != nil {
		return false, err
	}
	rp, err := relyingParty(v)
	if err != nil {
		return false, err
	}
	keys, err := keysForUpdate(ctx, tx, account.ID)
	if err != nil {
		return false, err
	}
	credential, err := rp.ValidateLogin(keyUserOf(account, keys), ceremony, parsed)
	if err != nil {
		s.log.InfoContext(ctx, "a key's answer was refused", "error", err)
		return false, nil
	}
	for _, key := range keys {
		if !bytes.Equal(key.CredentialID, credential.ID) {
			continue
		}
		if credential.Authenticator.CloneWarning {
			return false, s.refusalWith(ctx, tx, v, account.ID, "identity.second_factor_clone_suspected",
				map[string]string{"stored": fmt.Sprint(key.SignCount), "presented": fmt.Sprint(parsed.Response.AuthenticatorData.Counter)})
		}
		_, err := tx.Exec(ctx, `
			UPDATE second_factor SET sign_count = $2, credential_flags = $3, last_used_at = $4 WHERE id = $1`,
			key.ID, int64(credential.Authenticator.SignCount), int16(credential.Flags.ProtocolValue()), now)
		return err == nil, err
	}
	return false, nil
}

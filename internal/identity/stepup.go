package identity

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
)

// StepUpLifetime is how long a proved second factor lets a session make
// sensitive changes without being asked again, as GitHub's "sudo mode" does,
// so that one change does not ask twice (F14 spec, D4).
const StepUpLifetime = 10 * time.Minute

// ErrStepUpNeeded is a sensitive action on a session with no recent step-up:
// the person proves a second factor again, and comes back.
var ErrStepUpNeeded = errors.New("identity: a recent step-up is needed")

// actions is every action a step-up may be asked for.
var actions = []Action{ActionPassword, ActionFactors, ActionAddCard, ActionChangeEmail, ActionStore}

// ParseAction reads an action named in a request, reporting false for one
// the policy does not name.
func ParseAction(name string) (Action, bool) {
	action := Action(name)
	return action, slices.Contains(actions, action)
}

// NeedsStepUp reports whether action on session needs a step-up first: the
// policy asks for one (§18.2, D2) and the session has not proved a second
// factor in the last StepUpLifetime, nor, for an action whose step-up
// accepts a code to the account's address, answered one.
func (s *Service) NeedsStepUp(session Session, action Action) bool {
	asks := s.policy.StepUpFor(session.Account.Kind, action, session.SecondFactor)
	if !asks.Asked || s.recent(session.SteppedUpAt) {
		return false
	}
	return !asks.Email || !s.recent(session.EmailConfirmedAt)
}

// recent reports whether at is less than StepUpLifetime ago.
func (s *Service) recent(at *time.Time) bool {
	return at != nil && s.now().Sub(*at) < StepUpLifetime
}

// BeginStepUp opens a challenge for session to prove a second factor before
// action, and returns its token. An account with nothing to answer with is
// ErrNoSecondFactor.
func (s *Service) BeginStepUp(ctx context.Context, v Visit, session Session, action Action) (string, error) {
	token, hash, err := NewToken()
	if err != nil {
		return "", err
	}
	err = s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		ch := challenge{Account: session.Account.ID, Session: session.ID, Action: action}
		o, err := s.offers(ctx, tx, session.Account, ch)
		if err != nil {
			return err
		}
		// A locked account with nothing else to answer with still opens
		// one, so that its page can say why (D8).
		if len(o.Methods) == 0 && !o.Recovery && !o.CodesLocked {
			return ErrNoSecondFactor
		}
		return insertChallenge(ctx, tx, v.Marketplace, session.Account.ID, hash, session.ID, action, s.now())
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// StepUp answers session's step-up challenge. A right answer with one of the
// account's second factors marks the session stepped up now; a code to the
// account's address that is not one of them proves only the address, for the
// actions that accept it (§18.2), and never lets the password or the factors
// change (D2, D4). Either way it audits the method and the action it was for.
func (s *Service) StepUp(ctx context.Context, v Visit, session Session, token string, answer Answer) error {
	return s.answerChallenge(ctx, v, token, session.ID, answer, func(tx pgx.Tx, ch challenge, account Account, now time.Time) error {
		mark := markSteppedUp
		if answer.Method == MethodEmail {
			factor, err := hasEmailFactor(ctx, tx, account.ID)
			if err != nil {
				return err
			}
			if !factor {
				mark = markEmailConfirmed
			}
		}
		if err := mark(ctx, tx, session.ID, now); err != nil {
			return err
		}
		return s.recordWith(ctx, tx, v, account.ID, "identity.stepped_up",
			map[string]string{"method": string(answer.Method), "action": string(ch.Action)})
	})
}

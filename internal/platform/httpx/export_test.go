package httpx

import (
	"context"
	"net/http"

	"github.com/aleogr/marketplace/internal/identity"
)

// LinkBase exposes linkBase to the external tests.
var LinkBase = linkBase

// SetSession exposes setSession, and VisitOf visit, to the external tests.
var (
	SetSession = setSession
	VisitOf    = visit
)

// WithSession puts a session in a context, as the middleware does after the
// service authenticated it.
func WithSession(ctx context.Context, s identity.Session) context.Context {
	return context.WithValue(ctx, sessionKey{}, s)
}

// SteppedUp exposes the step-up guard: the deliveries that add a card and
// change the address mount it on their own routes.
func (s Site) SteppedUp(action identity.Action, back string, next http.Handler) http.Handler {
	return s.steppedUp(action, back, next)
}

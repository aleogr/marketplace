package httpx

import (
	"context"

	"github.com/aleogr/marketplace/internal/identity"
)

// LinkBase exposes linkBase to the external tests.
var LinkBase = linkBase

// WithSession puts a session in a context, as the middleware does after the
// service authenticated it.
func WithSession(ctx context.Context, s identity.Session) context.Context {
	return context.WithValue(ctx, sessionKey{}, s)
}

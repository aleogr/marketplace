package httpx

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/api/idtoken"
)

// GoogleCaller validates the OIDC tokens Cloud Tasks and Cloud Scheduler sign
// their callbacks with, against Google's own keys.
type GoogleCaller struct{}

// Validate returns the verified e-mail of the account the token belongs to.
//
// The library checks the signature, the issuer and the audience; what is left
// to check here is that Google is actually vouching for the address, since an
// unverified e-mail in a token is a claim and not a fact.
func (GoogleCaller) Validate(ctx context.Context, token, audience string) (string, error) {
	payload, err := idtoken.Validate(ctx, token, audience)
	if err != nil {
		return "", fmt.Errorf("the token is not valid for this service: %w", err)
	}

	email, ok := payload.Claims["email"].(string)
	if !ok || email == "" {
		return "", errors.New("the token names no account")
	}
	if verified, ok := payload.Claims["email_verified"].(bool); !ok || !verified {
		return "", errors.New("the token's account is not verified")
	}
	return email, nil
}

package httpx_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/httpx"
	"github.com/aleogr/marketplace/internal/platform/jobs"
	"github.com/aleogr/marketplace/internal/platform/outbox"
)

// caller stands in for Google: it accepts one token, minted for one audience,
// belonging to one account.
type caller struct {
	token    string
	audience string
	email    string
}

func (c caller) Validate(_ context.Context, token, audience string) (string, error) {
	if token != c.token {
		return "", errors.New("the signature does not verify")
	}
	if audience != c.audience {
		return "", errors.New("the token was minted for another audience")
	}
	return c.email, nil
}

// noTransactions stands in for the database. Nothing under test here reaches
// it: what is under test is who may call, not what the call does.
type noTransactions struct{}

func (noTransactions) InTx(context.Context, func(pgx.Tx) error) error {
	panic("these tests do not reach the database")
}

// runner records what it was asked to run, and answers as the real one would.
type runner struct {
	known string
	ran   bool
}

func (r *runner) Run(_ context.Context, name string) error {
	if name != r.known {
		return jobs.ErrUnknownJob
	}
	r.ran = true
	return nil
}

const (
	theToken    = "a-token-google-signed"
	theAudience = "https://marketplace.lab.example"
	theInvoker  = "marketplace-invoker@example.iam.gserviceaccount.com"
)

// endpoint returns the callback endpoint, and whether a job it knows has run.
func endpoint(t *testing.T) (http.Handler, *bool) {
	t.Helper()

	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))

	jobRunner := &runner{known: "a-job"}

	tasks := httpx.NewTasks(
		caller{token: theToken, audience: theAudience, email: theInvoker},
		theAudience, theInvoker, outbox.NewRegistry(), jobRunner, noTransactions{}, quiet)

	return http.HandlerFunc(tasks.Handle), &jobRunner.ran
}

// call makes a callback with the authorization given.
func call(t *testing.T, authorization, body string) int {
	t.Helper()

	handler, _ := endpoint(t)
	return callWith(t, handler, authorization, body)
}

func callWith(t *testing.T, handler http.Handler, authorization, body string) int {
	t.Helper()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"https://marketplace.lab.example"+httpx.TasksPath, strings.NewReader(body))
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder.Code
}

// TestAnUnsignedCallIsRefused is the one that matters: the endpoint is on the
// public internet, and the path is guessable.
func TestAnUnsignedCallIsRefused(t *testing.T) {
	for name, authorization := range map[string]string{
		"nothing at all":   "",
		"an empty bearer":  "Bearer ",
		"a made-up token":  "Bearer not-a-token",
		"the wrong scheme": "Basic " + theToken,
	} {
		if status := call(t, authorization, `{"job":"a-job"}`); status != http.StatusUnauthorized {
			t.Errorf("%s was answered with %d, want 401", name, status)
		}
	}
}

// TestATokenForAnotherAudienceIsRefused covers the token that is real and not
// for us: any service this same account also calls could replay one here.
func TestATokenForAnotherAudienceIsRefused(t *testing.T) {
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))

	// The endpoint expects one audience; the caller only validates tokens
	// minted for another.
	tasks := httpx.NewTasks(
		caller{token: theToken, audience: "https://somewhere.else.example", email: theInvoker},
		theAudience, theInvoker, outbox.NewRegistry(), &runner{known: "a-job"}, noTransactions{}, quiet)

	if status := callWith(t, http.HandlerFunc(tasks.Handle), "Bearer "+theToken, `{"job":"a-job"}`); status != http.StatusUnauthorized {
		t.Errorf("a token for another audience was answered with %d, want 401", status)
	}
}

// TestATokenOfAnotherAccountIsRefused: a valid Google token proves who is
// calling, not that they may. Every Google account can mint one.
func TestATokenOfAnotherAccountIsRefused(t *testing.T) {
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))

	tasks := httpx.NewTasks(
		caller{token: theToken, audience: theAudience, email: "somebody-else@example.iam.gserviceaccount.com"},
		theAudience, theInvoker, outbox.NewRegistry(), &runner{known: "a-job"}, noTransactions{}, quiet)

	if status := callWith(t, http.HandlerFunc(tasks.Handle), "Bearer "+theToken, `{"job":"a-job"}`); status != http.StatusUnauthorized {
		t.Errorf("another account's token was answered with %d, want 401", status)
	}
}

func TestAProperlySignedCallRunsTheJob(t *testing.T) {
	handler, ran := endpoint(t)

	if status := callWith(t, handler, "Bearer "+theToken, `{"job":"a-job"}`); status != http.StatusNoContent {
		t.Fatalf("a signed call was answered with %d, want 204", status)
	}
	if !*ran {
		t.Error("the job did not run")
	}
}

// TestWorkThatCanNeverSucceedIsNotRetried: Cloud Tasks retries a 5xx and gives
// up on a 4xx, so the code is the difference between a queue that drains and
// one that grinds on work nobody will ever consume.
func TestWorkThatCanNeverSucceedIsNotRetried(t *testing.T) {
	handler, _ := endpoint(t)

	for name, body := range map[string]string{
		"a job nobody registered":   `{"job":"no-such-job"}`,
		"an event nothing consumes": `{"event":{"ID":"1","Kind":"nobody.cares"}}`,
		"neither":                   `{}`,
		"not even JSON":             `not json`,
	} {
		if status := callWith(t, handler, "Bearer "+theToken, body); status != http.StatusBadRequest {
			t.Errorf("%s was answered with %d, want 400 so that the queue stops", name, status)
		}
	}
}

package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/aleogr/marketplace/internal/platform/jobs"
	"github.com/aleogr/marketplace/internal/platform/outbox"
)

// TasksPath is where Cloud Tasks and Cloud Scheduler call back into this
// service.
//
// It is an address on the same service the visitors reach, because Cloud Run
// gives one service one address. What keeps it from being an open door is the
// token, not the path: anyone can guess a path.
const TasksPath = "/internal/tasks"

// Caller is what proves who is calling.
//
// Cloud Tasks and Cloud Scheduler sign their callbacks with an OIDC token of
// the service account they were told to use. Validating it means three things,
// and all three matter: the token is really Google's, it was minted for this
// audience rather than for some other service the same account also calls, and
// the account is the one this deployment expects.
type Caller interface {
	// Validate returns the e-mail of the account the token belongs to.
	Validate(ctx context.Context, token, audience string) (string, error)
}

// Runner is the part of the job runner this endpoint uses: run the job with
// this name, or say why not.
//
// An interface rather than the runner itself, so that what guards this
// endpoint can be tested without a database — who may call is a question about
// tokens, not about jobs.
type Runner interface {
	Run(ctx context.Context, name string) error
}

// Tasks answers the callbacks.
type Tasks struct {
	caller   Caller
	audience string
	// invoker is the account allowed to call. Anything else is refused, even
	// with a valid Google token: a token is proof of who, not of permission.
	invoker  string
	registry *outbox.Registry
	runner   Runner
	database outbox.Transactor
	log      *slog.Logger
}

// NewTasks returns the callback endpoint.
func NewTasks(caller Caller, audience, invoker string, registry *outbox.Registry, runner Runner, database outbox.Transactor, log *slog.Logger) Tasks {
	return Tasks{
		caller: caller, audience: audience, invoker: invoker,
		registry: registry, runner: runner, database: database, log: log,
	}
}

// call is what Cloud Tasks or Cloud Scheduler sends: one event to deliver, or
// one job to run.
type call struct {
	Event *outbox.Event `json:"event,omitempty"`
	Job   string        `json:"job,omitempty"`
}

// Handle answers a callback.
//
// The status codes are read by Cloud Tasks, which retries what it is told to
// retry: a 5xx comes back, a 4xx does not. So work that failed for a reason
// that might pass — a provider timing out — answers 500, and work that will
// never succeed — an event nobody consumes, a job nobody registered — answers
// 400 and stays refused rather than being retried until the queue gives up.
func (t Tasks) Handle(w http.ResponseWriter, r *http.Request) {
	if !t.allowed(w, r) {
		return
	}

	var body call
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	switch {
	case body.Job != "":
		t.job(w, r, body.Job)
	case body.Event != nil:
		t.event(w, r, *body.Event)
	default:
		http.Error(w, "", http.StatusBadRequest)
	}
}

// allowed reports whether the caller proved it is the one expected, and writes
// the refusal when it did not.
func (t Tasks) allowed(w http.ResponseWriter, r *http.Request) bool {
	token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !found || token == "" {
		t.refuse(r.Context(), w, "no bearer token")
		return false
	}

	email, err := t.caller.Validate(r.Context(), token, t.audience)
	if err != nil {
		t.refuse(r.Context(), w, "the token did not validate: "+err.Error())
		return false
	}
	if !strings.EqualFold(email, t.invoker) {
		// A valid Google token belonging to somebody else. Every Google
		// account can mint one; only this deployment's invoker may be here.
		t.refuse(r.Context(), w, "the token belongs to "+email)
		return false
	}
	return true
}

// refuse answers a caller that did not prove itself, and says in the log what
// was wrong — never to the caller, who does not need to be told which part of
// their forgery to fix.
func (t Tasks) refuse(ctx context.Context, w http.ResponseWriter, reason string) {
	t.log.WarnContext(ctx, "a call to the internal endpoint was refused", "reason", reason)
	http.Error(w, "", http.StatusUnauthorized)
}

func (t Tasks) event(w http.ResponseWriter, r *http.Request, event outbox.Event) {
	err := outbox.Deliver(r.Context(), t.database, t.registry, event, t.log)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, outbox.ErrUnknownKind):
		t.log.ErrorContext(r.Context(), "an event arrived that nothing consumes",
			"event", event.ID, "kind", event.Kind)
		http.Error(w, "", http.StatusBadRequest)
	default:
		t.log.ErrorContext(r.Context(), "an event could not be handled",
			"event", event.ID, "kind", event.Kind, "error", err)
		http.Error(w, "", http.StatusInternalServerError)
	}
}

func (t Tasks) job(w http.ResponseWriter, r *http.Request, name string) {
	err := t.runner.Run(r.Context(), name)
	switch {
	case err == nil, errors.Is(err, jobs.ErrBusy):
		// Busy is not a failure: the other run is doing the work, and a retry
		// would only find it busy again.
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, jobs.ErrUnknownJob):
		http.Error(w, "", http.StatusBadRequest)
	default:
		t.log.ErrorContext(r.Context(), "a job failed", "job", name, "error", err)
		http.Error(w, "", http.StatusInternalServerError)
	}
}

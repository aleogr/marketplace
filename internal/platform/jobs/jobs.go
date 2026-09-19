// Package jobs runs the work Cloud Scheduler asks for, one run at a time.
//
// Cloud Scheduler can fire a job whose previous run is still going — a slow
// reconciliation, an instance that was cold. Two runs doing the same work would
// both write, so a run takes a lease keyed by the job's name and the second one
// stands down. The lease expires, because an instance killed mid-job would
// otherwise hold the job forever (docs/design.md, section 2.5).
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

// lease is how long a run holds a job before another may take it.
//
// Long enough that a slow run keeps its lease, short enough that an instance
// killed mid-job does not block the next hour's run. Cloud Run kills an
// instance ten seconds after SIGTERM, so anything beyond that is about the
// work, not about the platform.
const lease = 10 * time.Minute

// Job is a named piece of periodic work.
type Job interface {
	Name() string
	Run(ctx context.Context) error
}

// JobFunc adapts a function to a Job.
type JobFunc struct {
	Named string
	Do    func(ctx context.Context) error
}

func (j JobFunc) Name() string { return j.Named }

func (j JobFunc) Run(ctx context.Context) error { return j.Do(ctx) }

// Transactor runs work inside a transaction.
type Transactor interface {
	InTx(ctx context.Context, fn func(pgx.Tx) error) error
}

// ErrUnknownJob is what a call naming a job nobody registered returns.
var ErrUnknownJob = errors.New("no such job")

// ErrBusy is what a run whose job is already running returns. It is not a
// failure: it is the second runner doing the right thing.
var ErrBusy = errors.New("the job is already running")

// Runner holds the jobs and the lock they take.
type Runner struct {
	db     Transactor
	holder string
	log    *slog.Logger
	jobs   map[string]Job
	now    func() time.Time
}

// NewRunner returns a runner. holder identifies this instance in the lock, so
// that a stuck job can be traced to the instance that was running it.
func NewRunner(db Transactor, holder string, log *slog.Logger) *Runner {
	return &Runner{db: db, holder: holder, log: log, jobs: map[string]Job{}, now: time.Now}
}

// Register adds a job.
func (r *Runner) Register(job Job) { r.jobs[job.Name()] = job }

// Names returns every registered job, for the endpoint that refuses the rest.
func (r *Runner) Names() []string {
	names := make([]string, 0, len(r.jobs))
	for name := range r.jobs {
		names = append(names, name)
	}
	return names
}

// Run takes the lock and runs the job, or returns ErrBusy.
func (r *Runner) Run(ctx context.Context, name string) error {
	job, known := r.jobs[name]
	if !known {
		return fmt.Errorf("%w: %s", ErrUnknownJob, name)
	}

	taken, err := r.take(ctx, name)
	if err != nil {
		return err
	}
	if !taken {
		r.log.InfoContext(ctx, "a job was asked for while it was already running", "job", name)
		return ErrBusy
	}

	started := r.now()
	runErr := job.Run(ctx)

	// The lease is released whether the run worked or not: a failed run that
	// held its lease to the end of the lease would delay the next attempt by
	// ten minutes for no reason.
	if err := r.release(ctx, name); err != nil {
		r.log.ErrorContext(ctx, "a job finished and its lock could not be released",
			"job", name, "error", err)
	}

	r.log.InfoContext(ctx, "job finished",
		"job", name, "took", r.now().Sub(started).String(), "failed", runErr != nil)
	return runErr
}

// take claims the job for this instance, and reports whether it got it.
//
// One statement: the row is inserted when nobody has ever run the job, and
// updated only when the lease has expired. Two instances racing here both run
// the same statement, and PostgreSQL decides — which is what makes this a lock
// rather than a read followed by a hopeful write.
func (r *Runner) take(ctx context.Context, name string) (bool, error) {
	var taken bool
	err := r.db.InTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			INSERT INTO job_lock (name, holder, leased_until)
			VALUES ($1, $2, now() + $3::interval)
			ON CONFLICT (name) DO UPDATE
			SET holder = EXCLUDED.holder, leased_until = EXCLUDED.leased_until
			WHERE job_lock.leased_until < now()
		`, name, r.holder, lease.String())
		if err != nil {
			return fmt.Errorf("cannot take the lock for %s: %w", name, err)
		}
		taken = tag.RowsAffected() == 1
		return nil
	})
	return taken, err
}

func (r *Runner) release(ctx context.Context, name string) error {
	return r.db.InTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE job_lock
			SET leased_until = now(), finished_at = now()
			WHERE name = $1 AND holder = $2
		`, name, r.holder)
		return err
	})
}

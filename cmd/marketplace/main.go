// Command marketplace is the platform's single binary.
//
// With no arguments it serves. With `migrate` it applies the database
// migrations it carries and exits. There is one binary and therefore one image:
// the schema a deployment applies and the code that expects it are always from
// the same commit (docs/roadmap.md, F4).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/identity/breached"
	"github.com/aleogr/marketplace/internal/platform/audit"
	"github.com/aleogr/marketplace/internal/platform/config"
	"github.com/aleogr/marketplace/internal/platform/db"
	"github.com/aleogr/marketplace/internal/platform/geoip"
	"github.com/aleogr/marketplace/internal/platform/httpx"
	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/internal/platform/jobs"
	"github.com/aleogr/marketplace/internal/platform/keys"
	"github.com/aleogr/marketplace/internal/platform/keys/kms"
	"github.com/aleogr/marketplace/internal/platform/keys/local"
	"github.com/aleogr/marketplace/internal/platform/logging"
	"github.com/aleogr/marketplace/internal/platform/mail"
	"github.com/aleogr/marketplace/internal/platform/mail/brevo"
	"github.com/aleogr/marketplace/internal/platform/mail/mailbox"
	"github.com/aleogr/marketplace/internal/platform/outbox"
	"github.com/aleogr/marketplace/internal/platform/ratelimit"
	"github.com/aleogr/marketplace/internal/platform/seo"
	"github.com/aleogr/marketplace/internal/platform/tasks"
	"github.com/aleogr/marketplace/internal/platform/version"
	"github.com/aleogr/marketplace/internal/tenancy"
)

// shutdownGrace is how long in-flight requests have to finish after SIGTERM.
// Cloud Run allows up to ten seconds before it kills the instance.
const shutdownGrace = 8 * time.Second

// The general limit, per client address and per instance. It is deliberately
// generous: it is there to blunt a flood, not to pace a visitor who is loading
// a page with its images and a few HTMX fragments. The sensitive endpoints are
// bounded far more tightly, and in the database, by the delivery that adds them
// (docs/roadmap.md, F7 and F10). Both become console parameters in F17.
const (
	generalBurst     = 120
	generalPerMinute = 240
)

// ProbeTemplate is the message the delivery probe sends: the one template that
// exists to be sent by hand, after a sending domain is configured, to prove
// that mail leaves the platform and arrives (docs/roadmap.md, F11).
const ProbeTemplate = "probe"

// VerifyAuditJob is the scheduled job that walks every audit chain and
// recomputes it. A chain that does not verify is not a thing to discover during
// an investigation (docs/requirements.md, section 21).
const VerifyAuditJob = "verify-audit-chain"

// DispatchJob is the scheduled job that empties the outbox into the queue.
// Cloud Scheduler calls it; its name is in the Terraform configuration too
// (infra/terraform/tasks.tf).
const DispatchJob = "dispatch-outbox"

func main() {
	if err := run(context.Background(), os.Args[1:], os.LookupEnv, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "marketplace: %v\n", err)
		os.Exit(1)
	}
}

// run reads the configuration and does what the arguments ask for.
func run(ctx context.Context, args []string, lookup config.Lookup, stdout io.Writer) error {
	cfg, err := config.Load(lookup)
	if err != nil {
		return err
	}

	log := logging.New(stdout, cfg.LogLevel)

	if len(args) == 0 {
		return serve(ctx, cfg, log)
	}

	switch args[0] {
	case "migrate":
		return migrate(ctx, cfg, log)
	case "send-probe":
		return sendProbe(ctx, cfg, log, args[1:])
	case "bench-password":
		return benchPassword(ctx, log)
	default:
		return fmt.Errorf(
			"unknown command %q: the binary serves when given no arguments, and applies migrations with `migrate`",
			args[0])
	}
}

// serve runs the HTTP server until the process is asked to stop.
func serve(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The pool is opened before the listener so that a database that was
	// declared and cannot be reached refuses the start rather than answering
	// health checks with a 503 nobody is watching for.
	var database *db.Pool
	if cfg.Database.Configured() {
		pool, err := db.Open(ctx, cfg.Database)
		if err != nil {
			return err
		}
		defer pool.Close()

		// Re-examined on 2026-09-23, when the shared lab instance began to
		// sleep four nights a week: a process started while it sleeps exits
		// here and Cloud Run answers 503 until it wakes. That is kept on
		// purpose — refusing to start is what keeps a revision with a broken
		// database setting from ever taking traffic (docs/infrastructure.md).
		if err := pool.Ping(ctx); err != nil {
			return fmt.Errorf("the database was declared but does not answer: %w", err)
		}
		database = pool
	}

	// What protects the audit log's contents, built and proved before the port
	// is opened: a deployment that can wrap and cannot unwrap looks configured,
	// serves, and loses everything it writes (docs/requirements.md, section
	// 21).
	keeper, closeKeeper, err := protection(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer closeKeeper()
	log.InfoContext(ctx, "audit keys", "keeper", keeper.Name())

	// One audit log for the whole process: the work below and the identity
	// flows append to the same chains. The per-person keys also seal second
	// factors' secrets, so an erasure destroys both (F14 spec, D5).
	personal := audit.NewKeys(keeper)
	trail := audit.NewLog(personal, log)

	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return fmt.Errorf("cannot listen on port %d: %w", cfg.Port, err)
	}

	log.InfoContext(ctx, "server started",
		"version", version.String(),
		"port", cfg.Port,
		"indexable", cfg.Indexable,
		"providers", string(cfg.ProvidersMode),
		// Which of the two modes is in effect, on every start, for the same
		// reason the indexing mode is logged: a process quietly running without
		// a database is a thing somebody must be able to see.
		"database", databaseMode(cfg.Database),
		"platform_host", cfg.PlatformHost,
	)

	// The catalogues are read once, at start-up. A catalogue that cannot be
	// read is a process that cannot say anything to anybody, so it refuses to
	// start rather than serving pages of empty strings.
	catalogue, err := i18n.Load()
	if err != nil {
		return err
	}

	// A nil *db.Pool in an interface is not a nil interface, so the handler is
	// given one only when there is one to give.
	var checker httpx.Database
	if database != nil {
		checker = database
	}

	// The preview image is drawn from the marketplace's name rather than
	// stored, so a new marketplace needs no new file (internal/platform/seo).
	preview, err := seo.NewPreview()
	if err != nil {
		return fmt.Errorf("cannot prepare the link preview: %w", err)
	}

	pages := httpx.NewPages(catalogue)
	site := httpx.NewSite(checker, catalogue, preview, cfg.Indexable)

	// The e-mail port. Which adapter is behind it is the configuration's
	// answer and nothing else's (docs/requirements.md, section 25); what the
	// process knows is that it has one, and that it refuses to start without
	// templates it can render.
	var mailDatabase mail.Database
	if database != nil {
		mailDatabase = database
	}
	mailer, provider, err := mailing(cfg, catalogue, mailDatabase, log)
	if err != nil {
		return err
	}
	log.InfoContext(ctx, "e-mail provider", "provider", provider.Name())

	// The addresses a machine calls rather than a browser. They are outside
	// CSRF and outside the language redirect, for the same reason the task
	// endpoint is: nothing on the other side follows a redirect or holds a
	// token (internal/platform/httpx.Pipeline).
	var callbacks []string
	if database != nil && cfg.Mail.WebhookToken != "" {
		webhook := httpx.NewMailWebhook(provider, cfg.Mail.WebhookToken, database, log)
		site = site.WithMail(webhook)
		callbacks = append(callbacks, webhook.Path())
	}

	// Work that must not happen inside a request. A process without a database
	// has no outbox to move and no lock to take, so it serves pages and
	// nothing else (docs/design.md, section 2.5).
	if database != nil {
		tasks, closeTasks, err := work(ctx, cfg, database, mailer, trail, log)
		if err != nil {
			return err
		}
		defer closeTasks()
		site = site.WithTasks(tasks)
	}

	// Accounts, sign-up and, later, sessions. They need the database, so a
	// process running without one serves no identity pages at all.
	var identityService *identity.Service
	if database != nil {
		var checker breached.Checker = breached.Fake{Known: breached.Common}
		if cfg.ProvidersMode == config.ProvidersReal {
			checker = breached.NewPwned(&http.Client{Timeout: 3 * time.Second}, breached.RangeAPI)
		}
		identityService = identity.NewService(database,
			identity.NewHasher(identity.Current, hashSlots(identity.Current)), checker, trail, log).
			WithSealer(personal)
		site = site.WithIdentity(httpx.IdentityRoutes{
			Service: identityService,
			Pages:   pages,
			Log:     log,
			// Each limiter is named, and its counters are its own
			// (internal/platform/ratelimit).
			Limits: httpx.IdentityLimits{
				SignUp:        ratelimit.NewDatabase(database, "signup", 10, time.Hour),
				Resend:        ratelimit.NewDatabase(database, "resend", 10, time.Hour),
				ResendAddress: ratelimit.NewDatabase(database, "resend-address", 3, time.Hour),
				SignIn:        ratelimit.NewDatabase(database, "signin", 30, 10*time.Minute),
				SignInAddress: ratelimit.NewDatabase(database, "signin-address", 10, 15*time.Minute),
				Password:      ratelimit.NewDatabase(database, "password", 10, time.Hour),
				StepUp:        ratelimit.NewDatabase(database, "step-up", 10, 15*time.Minute),
			},
		})
	}

	handler := site.Handler()

	// The session is read after the marketplace and the language are known,
	// because a session belongs to one marketplace and is looked up in its
	// transaction (docs/design.md, section 2.3). The resolvers below wrap
	// this, so a request meets them first.
	if identityService != nil {
		handler = httpx.Sessions(identityService, log)(handler)
	}

	// Language comes next, and it redirects: the language is part of the URL,
	// so a page is never served at an address that does not say which language
	// it is in (docs/design.md, decision 10). Country detection is a port with
	// no adapter yet, which is a working deployment — everyone gets the
	// official language until they choose otherwise (docs/roadmap.md, F18).
	//
	// Five addresses are outside it, and none of them is a page. The health
	// check and the language switch are machinery. `robots.txt` is read by a
	// crawler before it reads anything else and is defined to live at the
	// root; the link preview is fetched by a scraper that may not follow a
	// redirect at all; and the callback endpoint is called by Cloud Tasks and
	// Cloud Scheduler, which do not follow redirects either — a redirect
	// there is a job that silently never runs.
	handler = i18n.NewResolver(catalogue, geoip.Nowhere{}).
		Resolve(httpx.Speaks(catalogue),
			append([]string{
				httpx.HealthPath, httpx.LanguagePath, httpx.RobotsPath,
				httpx.PreviewPath, httpx.TasksPath,
			}, callbacks...)...)(handler)

	// Host resolution comes before that, because language, session and every
	// query after it are scoped by the answer (docs/design.md, section 2.3).
	// It needs the database to know the hosts, so a process running without
	// one serves every host as before.
	if database != nil {
		resolver := tenancy.NewResolver(tenancy.NewRepository(database), cfg.PlatformHost)
		handler = tenancy.Resolve(resolver, pages, httpx.HealthPath)(handler)
	}

	// And the defences come before that. There is no load balancer and no web
	// application firewall in front of this process, so the binary is what
	// applies them (docs/requirements.md, section 24).
	handler = httpx.Pipeline{
		Indexable: cfg.Indexable,
		ProxyHops: cfg.ProxyHops,
		General:   ratelimit.NewMemory(generalBurst, generalPerMinute),
		Pages:     pages,
		Callbacks: callbacks,
		Log:       log,
	}.Wrap(handler)

	if err := httpx.Serve(ctx, listener, handler, shutdownGrace); err != nil {
		return err
	}

	log.InfoContext(context.WithoutCancel(ctx), "server stopped")
	return nil
}

// hashSlots is how many passwords are hashed at once: as many as fit in a
// quarter of the instance's 512 MiB at the memory one hash takes, and never
// fewer than two (spec, D5). The rest of the instance is the process itself
// and the requests it is serving.
//
// At identity.Current's 64 MiB, that is max(2, 131072/65536) = 2 slots: at
// most two hashes run at once, together holding 2 x 64 MiB = 128 MiB of the
// instance's 512 MiB.
func hashSlots(params identity.Params) int {
	return max(2, 128*1024/int(params.Memory))
}

// protection returns the keeper that wraps every person's key, and the
// function that releases it.
//
// A deployment uses Cloud KMS, where the wrapping key cannot be read by
// anything — which is what makes destroying a person's key final. An
// environment with no key manager wraps with a key of its own, and one with no
// key at all wraps with a key that dies when the process does. The last of
// those is a local run and says so out loud: what it wrote yesterday cannot be
// read today.
func protection(ctx context.Context, cfg config.Config, log *slog.Logger) (keys.Keeper, func(), error) {
	switch {
	case cfg.Audit.Key != "":
		keeper, err := kms.New(ctx, cfg.Audit.Key)
		if err != nil {
			return nil, nil, err
		}
		if err := keys.Check(ctx, keeper); err != nil {
			_ = keeper.Close()
			return nil, nil, err
		}
		return keeper, func() { _ = keeper.Close() }, nil

	case cfg.Audit.LocalKey != "":
		keeper, err := local.Parse(cfg.Audit.LocalKey)
		if err != nil {
			return nil, nil, err
		}
		log.WarnContext(ctx, "the audit keys are wrapped by a key this process holds",
			"keeper", keeper.Name())
		return keeper, func() {}, keys.Check(ctx, keeper)

	default:
		keeper, err := local.Generate()
		if err != nil {
			return nil, nil, err
		}
		log.WarnContext(ctx, "the audit keys are wrapped by a key that dies with this process",
			"keeper", keeper.Name())
		return keeper, func() {}, keys.Check(ctx, keeper)
	}
}

// work assembles the asynchronous half of the process: who consumes which
// event, which jobs exist, and how an event reaches them.
//
// Two ways out of the outbox, and the process picks by what it was told. A
// deployment hands events to Cloud Tasks, which calls back with a signed
// token. Anything else — a local run, the end-to-end suite — delivers them
// inline, which exercises the same registry and the same idempotency with no
// queue and no network (docs/design.md, section 2.2).
func work(ctx context.Context, cfg config.Config, database *db.Pool, mailer *mail.Mailer, trail *audit.Log, log *slog.Logger) (httpx.Tasks, func(), error) {
	registry := outbox.NewRegistry()
	mail.Register(registry, mailer)

	var queuer outbox.Queuer = outbox.NewInline(database, registry, log)
	closeQueue := func() {}

	if cfg.Tasks.Configured() {
		client, err := tasks.New(ctx, tasks.Settings{
			Project:  cfg.Tasks.Project,
			Location: cfg.Tasks.Location,
			URL:      cfg.Tasks.URL,
			Invoker:  cfg.Tasks.Invoker,
			Audience: cfg.Tasks.Audience,
		})
		if err != nil {
			return httpx.Tasks{}, nil, err
		}
		queuer = client
		closeQueue = func() { _ = client.Close() }
	}

	dispatcher := outbox.NewDispatcher(database, queuer, log)

	// No scheduler runs locally, so a process with fake providers dispatches
	// its own outbox (cmd/marketplace/dispatch.go).
	if cfg.ProvidersMode == config.ProvidersFake && !cfg.Tasks.Configured() {
		go dispatchLocally(ctx, dispatcher.Dispatch, localDispatchEvery, log)
		log.InfoContext(ctx, "the outbox is dispatched in this process", "every", localDispatchEvery.String())
	}

	// The dispatcher is itself a job, and Cloud Scheduler is what runs it.
	// There is no always-on process to loop in, and a request that dispatched
	// on its way out would make one visitor pay for everybody's work.
	runner := jobs.NewRunner(database, version.String(), log)
	runner.Register(jobs.JobFunc{
		Named: DispatchJob,
		Do: func(ctx context.Context) error {
			handed, err := dispatcher.Dispatch(ctx)
			if handed > 0 {
				log.InfoContext(ctx, "events handed to the queue", "count", handed)
			}
			return err
		},
	})

	// The audit chains, walked on a schedule. A break is a failed job, which is
	// what makes it something somebody hears about rather than something a
	// report would have shown if anybody had opened it.
	runner.Register(jobs.JobFunc{
		Named: VerifyAuditJob,
		Do: func(ctx context.Context) error {
			report, err := trail.Verify(ctx, database)
			if err != nil {
				return err
			}
			log.InfoContext(ctx, "audit chains verified",
				"chains", report.Chains, "records", report.Records,
				"breaks", len(report.Breaks))
			if len(report.Breaks) > 0 {
				return fmt.Errorf("%d audit records do not verify", len(report.Breaks))
			}
			return nil
		},
	})

	audience := cfg.Tasks.Audience
	if audience == "" {
		audience = cfg.Tasks.URL
	}

	return httpx.NewTasks(httpx.GoogleCaller{}, audience, cfg.Tasks.Invoker,
		registry, runner, database, log), closeQueue, nil
}

// migrate applies the migrations the binary carries and returns.
func migrate(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	if !cfg.Database.Configured() {
		return errors.New(
			"`migrate` needs a database: set DATABASE_URL, or DATABASE_INSTANCE with DATABASE_NAME and DATABASE_USER")
	}

	pool, err := db.Open(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer pool.Close()

	log.InfoContext(ctx, "applying migrations",
		"version", version.String(),
		"database", databaseMode(cfg.Database),
	)

	if err := db.Migrate(ctx, pool, cfg.Database, log); err != nil {
		return err
	}

	// The marketplaces this environment declares, applied after the schema
	// that holds them. They come from the environment rather than from a
	// migration because their hosts are the environment's: a host written into
	// a versioned migration would be created in every environment that runs it
	// (internal/tenancy/seed.go).
	specs, err := tenancy.ParseSpecs(cfg.SeedMarketplaces)
	if err != nil {
		return err
	}
	if len(specs) == 0 {
		log.InfoContext(ctx, "no marketplaces declared for this environment")
		return nil
	}

	// The audit log of the job's own work. A marketplace appearing or changing
	// is a parameter change, and every parameter change is audited
	// (docs/requirements.md, section 21) — including the ones nobody
	// personally made, which is what the `system` actor is for.
	keeper, closeKeeper, err := protection(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer closeKeeper()
	trail := audit.NewLog(audit.NewKeys(keeper), log)

	if err := pool.InTx(ctx, func(tx pgx.Tx) error {
		applied, err := tenancy.Seed(ctx, tx, specs)
		if err != nil {
			return err
		}
		return record(ctx, tx, trail, applied)
	}); err != nil {
		return err
	}

	slugs := make([]string, 0, len(specs))
	for _, spec := range specs {
		slugs = append(slugs, spec.Slug)
	}
	log.InfoContext(ctx, "marketplaces seeded", "slugs", slugs)
	return nil
}

// record writes one audit record per marketplace the job declared.
//
// In the same transaction as the seeding, which is the whole point: a
// deployment that rolled back declared nothing and recorded nothing, and one
// that committed cannot have committed without its record
// (internal/platform/audit).
func record(ctx context.Context, tx pgx.Tx, trail *audit.Log, applied []tenancy.Applied) error {
	for _, one := range applied {
		after, err := json.Marshal(one.Spec)
		if err != nil {
			return fmt.Errorf("cannot describe %s: %w", one.Spec.Slug, err)
		}

		action := "marketplace.updated"
		if one.Created {
			action = "marketplace.created"
		}

		if err := trail.Append(ctx, tx, audit.Entry{
			// The platform's own chain: declaring a marketplace is the
			// platform's act, not the marketplace's.
			Actor:   audit.Actor{Kind: audit.System},
			Action:  action,
			Subject: audit.Subject{Kind: "marketplace", ID: one.ID},
			After:   after,
		}); err != nil {
			return err
		}
	}
	return nil
}

// databaseMode names the route to the database without ever naming a
// credential: the connection string may carry a password, so it is the route
// that is logged and not the setting.
func databaseMode(settings config.Database) string {
	switch {
	case settings.Instance != "":
		return "cloud sql: " + settings.Instance
	case settings.URL != "":
		return "direct"
	default:
		return "none"
	}
}

// provider is an adapter that both sends and reads what the provider posts
// back. Every adapter does both, because sending and being told what became of
// the message are two halves of one integration.
type provider interface {
	mail.Sender
	mail.Reader
}

// mailing assembles the e-mail port: the templates, the adapter behind it, and
// the mailer the rest of the process asks.
//
// Which adapter is a configuration answer. The fake writes each message to a
// directory, which is what the end-to-end suite reads and what an environment
// whose provider account is not ready yet runs with; the real one talks to
// Brevo (docs/requirements.md, section 25).
func mailing(cfg config.Config, catalogue *i18n.Catalogue, database mail.Database, log *slog.Logger) (*mail.Mailer, provider, error) {
	// Read once, at start-up, for the same reason the catalogues are: a
	// template that does not render is a person who never receives what they
	// were promised, and that is a deployment to refuse rather than a message
	// to lose.
	templates, err := mail.LoadTemplates()
	if err != nil {
		return nil, nil, err
	}

	var adapter provider
	switch cfg.ProvidersMode {
	case config.ProvidersReal:
		client, err := brevo.New(brevo.Settings{
			Key: cfg.Mail.Key,
			From: brevo.Address{
				Name:  catalogue.Printer(i18n.Default).Sprintf("page.platform.title"),
				Email: cfg.Mail.From,
			},
		})
		if err != nil {
			return nil, nil, err
		}
		adapter = client
	default:
		box, err := mailbox.New(mailDirectory(cfg))
		if err != nil {
			return nil, nil, err
		}
		adapter = box
	}

	return mail.NewMailer(database, templates, adapter, i18n.Default, log), adapter, nil
}

// mailDirectory is where the fake adapter writes. A default rather than a
// required setting, so that running the binary locally sends its mail
// somewhere instead of refusing to start.
func mailDirectory(cfg config.Config) string {
	if cfg.Mail.Directory != "" {
		return cfg.Mail.Directory
	}
	return filepath.Join(os.TempDir(), "marketplace-mailbox")
}

// sendProbe sends one message to the address given and returns.
//
// It is how a sending domain is proved to work: the records are published, the
// provider verifies them, and then somebody has to receive an actual e-mail
// (docs/roadmap.md, F11). It runs as the migration job does — the same image,
// a different entry point — so proving it in a deployment needs no second
// image and no endpoint that could be called by anybody else.
func sendProbe(ctx context.Context, cfg config.Config, log *slog.Logger, args []string) error {
	if len(args) == 0 || args[0] == "" {
		return errors.New("`send-probe` needs the address to send to")
	}
	address, language := args[0], i18n.Default
	if len(args) > 1 {
		language = args[1]
	}

	catalogue, err := i18n.Load()
	if err != nil {
		return err
	}
	if !catalogue.Speaks(language) {
		return fmt.Errorf("%q is not a language this platform speaks", language)
	}

	// A database is not required. The probe is one message and its own answer
	// — it either arrives or it does not — and an environment that has one
	// gets the suppression check and the record for free.
	var database mail.Database
	if cfg.Database.Configured() {
		pool, err := db.Open(ctx, cfg.Database)
		if err != nil {
			return err
		}
		defer pool.Close()
		database = pool
	}

	mailer, adapter, err := mailing(cfg, catalogue, database, log)
	if err != nil {
		return err
	}

	name := catalogue.Printer(language).Sprintf("page.platform.title")
	log.InfoContext(ctx, "sending the delivery probe",
		"provider", adapter.Name(), "language", language)

	state, err := mailer.Send(ctx, mail.Message{
		Template: ProbeTemplate,
		Language: language,
		To:       address,
		From:     name,
	})
	if err != nil {
		return err
	}

	// What happened, not what was hoped for. A probe that announces success
	// for a message the platform deliberately did not send is a probe that
	// misleads the person reading its last line — which is how this was found
	// (docs/infrastructure.md).
	switch state {
	case mail.StateSkipped:
		log.InfoContext(ctx, "the delivery probe was not sent: the address is suppressed",
			"provider", adapter.Name())
	default:
		log.InfoContext(ctx, "the delivery probe was accepted", "provider", adapter.Name())
	}
	return nil
}

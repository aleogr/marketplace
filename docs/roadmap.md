# Phase 1 (Foundation): Roadmap

> **Status:** implementation roadmap for phase 1 of `docs/design.md`, section 10. It turns the
> foundation phase into an ordered list of deliveries, each small enough for its own pull request.
>
> **Sources of truth:** `docs/requirements.md` decides *what*, `docs/design.md` decides *how*,
> `docs/research/` explains *why*. This document decides *in which order* and *how each step is
> proved done*. It reopens no decision recorded in those documents. Section numbers in the form
> §N refer to `docs/requirements.md`; references such as "design §3" refer to `docs/design.md`.
>
> **Scope of this document:** phase 1 only. Phases 2 to 7 are listed in `docs/design.md`,
> section 10, and get their own roadmap when they start.

---

## 1. Goal

**Goal:** a deployed, observable, tenant-isolated, bilingual Go binary in the lab, in which a
person can create an account, sign in with a second factor, and in which every operation is
audited — with a pipeline that refuses to merge anything that breaks it.

**Architecture:** a modular monolith in Go (design §2.1) behind a request pipeline that resolves
marketplace, language, session and tenant context in that order (design §2.3). Every external
provider sits behind a domain-defined port with one real adapter and one fake (design §2.2).
Asynchronous work uses outbox + Cloud Tasks + Cloud Scheduler, because Cloud Run scales to zero
(design §2.5). Tenant isolation is enforced twice: in the application and by PostgreSQL row-level
security (design §2.6).

**Tech stack:** Go (the language version and the exact toolchain are pinned in `go.mod`), templ + HTMX + Alpine.js,
PostgreSQL 16 on Cloud SQL, Cloud Run, Cloud Storage, Cloud Tasks, Cloud Scheduler, Secret
Manager, Cloud KMS, Terraform, GitHub Actions, pytest + Playwright (Python) for end-to-end tests.

**Spec:** `docs/requirements.md` and `docs/design.md`. Read both before executing any delivery
below; this roadmap argues from them and does not restate them.

## 2. Global constraints

Every delivery's requirements implicitly include this section.

- **Repository language:** everything versioned is written in English — code, comments, commit
  messages, pull request titles and descriptions, documentation. Translation files for other
  languages are the only exception (§3).
- **Definition of done:** every user-facing text exists in **both en-US and pt-BR**, tests pass,
  and there is verification evidence — test output, screenshots or equivalent (§3).
- **No hard-coded user-facing text.** Everything goes through translation keys (§6).
- **The repository is public. No secret may ever be committed** (§3). Secrets live in Secret
  Manager or in the repository's encrypted secrets. Model identifiers appear only in the
  attribution trailer of commit messages and pull request descriptions.
- **Money** is stored in minor units with a currency code; **timestamps** are stored in UTC and
  formatted per locale; **addresses** assume no country's format (§6).
- **Every tenant table** carries `marketplace_id` and is covered by row-level security (design §4).
- **Every external identifier** is stored as the pair `(provider, external_id)` (§25).
- **Tests use fakes of external providers, never real services** (§25). Provider sandboxes are
  exercised only by the manual test plan, which belongs to phase 7.
- **UI principles:** immediate visual feedback when an element is pressed; the operating system's
  reduced-motion preference is respected; animations only when they serve a purpose (§3).
- **Versioning:** SemVer, `0.y.z` during development, `1.0.0` at the first production launch (§27).
- **Branches and pull requests:** one topic, one branch, one pull request opened by Claude Code;
  never a direct push to `main` (§3).
- **Checks that must pass before a push**, kept current in `CLAUDE.md`:

  ```
  go vet ./... && staticcheck ./... && golangci-lint run && gosec ./... && govulncheck ./...
  go test ./...
  pytest e2e/
  ```

## 3. What "done" means for one delivery

A delivery is finished when all of the following are true. The pull request description states
each one with its evidence; a delivery that cannot show the evidence is not finished.

1. The pipeline is green on the pull request: build, `go vet`, `staticcheck`, `golangci-lint`,
   `gosec`, `govulncheck`, `gitleaks`, unit tests with `-race`, integration tests against a real
   PostgreSQL, end-to-end tests, and the container image scanned.
2. Every user-facing string exists in `en-US` and `pt-BR`, and the catalogue-parity check passes.
3. The verification listed in the delivery was executed and its output or screenshots are attached.
4. Any requirement or design decision the delivery changed was updated in the same pull request (§3).
5. Any manual step the owner performed is recorded in `docs/infrastructure.md`, so it can be
   repeated when a second environment is created.

## 4. Working rules for this phase

- **Manual steps are given one at a time.** The owner executes one instruction, reports the
  result, and only then receives the next one (§3).
- **The owner has no local development environment** (§27), and this session has no `gcloud` and
  no Terraform binary (see the appendix). Every manual GCP step is therefore written as a command
  block to paste into **Cloud Shell** in the GCP console, or as a sequence of console clicks.
  Terraform runs in CI, never in a session.
- **Nothing in phase 1 is blocked by the open topics of §29.** The lawyer, the payment gateway
  proposals, the shipping confirmation and the MaxMind account are tracked in section 7 with the
  delivery that first needs them; only F18 depends on an owner-supplied account inside this phase,
  and it is deliberately last.
- **A delivery whose owner step is not ready still merges** behind its fake adapter, and the real
  adapter is enabled by a Terraform variable afterwards. F11 and F18 are written that way.

## 5. Deliveries

Ordered. Each one is a single pull request. `Depends on` lists only hard dependencies; deliveries
not linked by a dependency may be worked in any order, and section 6 shows where that happens.

---

### F1 — Repository skeleton, configuration, build identifier and the pipeline

- [x] **Objective:** every later pull request is checked by a pipeline that runs the full command
  list of `CLAUDE.md`, and the binary starts, announces which build it is, and answers a health
  check.

**Depends on:** nothing.

**What is needed from the owner:**
1. Enable branch protection on `main`: require a pull request and require the CI checks to pass.
2. Enable Dependabot alerts and Dependabot security updates in the repository settings (§27).

**Scope:**
- `go.mod` (module `github.com/aleogr/marketplace`, Go 1.24 toolchain), with the lint and security
  tools pinned as `tool` directives so CI and a session run identical versions.
- `cmd/marketplace/main.go`: HTTP server, `/health`, graceful shutdown on `SIGTERM`, which is what
  Cloud Run sends before stopping an instance.
- `internal/platform/config`: typed loading from the environment. Unknown or unparseable values
  **refuse to start**, naming the variable and quoting the value received (§7.1 generalised).
- `internal/platform/logging`: `log/slog` JSON handler using the Cloud Logging field names
  (`severity`, `message`, `time`), so Cloud Logging parses severity without a parser (design §3).
- `internal/platform/version`: the identifier injected with `-ldflags -X` from `git describe
  --tags --always --dirty`; before the first tag, the short commit hash and the build date. Logged
  on every start (§27).
- `Makefile`: `check`, `test`, `integration`, `e2e`, `build`, `run`.
- `.golangci.yml`, `.gitleaks.toml`, `.github/dependabot.yml` (Go modules and GitHub Actions).
- `.github/workflows/ci.yml`: the command list above, plus `gitleaks` and the image build scanned
  by Trivy.
- `e2e/`: `requirements.txt` pinning `playwright==1.56.0` and `pytest`, `conftest.py` starting the
  binary with `PROVIDERS_MODE=fake` on a free port and stopping it afterwards, `test_smoke.py`.

**Verification:**
- `go vet ./... && staticcheck ./... && golangci-lint run && gosec ./... && govulncheck ./...`
- `go test ./... -race` and `pytest e2e/`, both green, output attached.
- A test asserting that a boolean environment variable with a typo makes the process exit
  non-zero with the variable's name in the message.
- The start-up log line showing the build identifier, attached as an excerpt.
- The pipeline green on this very pull request is itself the evidence that the pipeline runs.

**Not in this delivery:** any GCP resource, any database.

---

### F2 — GCP bootstrap and the Terraform foundation

- [x] **Objective:** infrastructure is described in Terraform, its state lives in a versioned
  bucket, and GitHub authenticates to GCP without any stored key (§27).

**Depends on:** F1.

**What is needed from the owner** (one instruction at a time; steps 2 and 3 are command blocks for
Cloud Shell):
1. Create the GCP project in `us-central1` and link a billing account; report the project id.
2. Run the bootstrap block: enable the APIs (Cloud Run, Cloud SQL Admin, Artifact Registry, Secret
   Manager, Cloud Tasks, Cloud Scheduler, Cloud Storage, IAM, IAM Credentials, STS, Cloud Resource
   Manager, Cloud KMS, Monitoring, Logging); create the Terraform state bucket with object
   versioning and uniform bucket-level access; create the Terraform service account; create the
   Workload Identity Federation pool and provider **restricted to this repository**; grant the
   roles.
3. Add the GitHub Actions repository **variables** (not secrets — federation stores no keys) for
   the project id, the WIF provider resource name and the Terraform service account e-mail.
4. Add `terraform` to the environment setup script described in `docs/claude-code-environment.md`,
   so `terraform fmt` and `terraform validate` can be run in a session before pushing.

**Scope:**
- `infra/terraform/`: `versions.tf` (pinned provider versions), `backend.tf` (the state bucket),
  `providers.tf`, `variables.tf`, and a `lab/` configuration.
- Artifact Registry repository for the container image.
- `.github/workflows/terraform.yml`: `fmt -check`, `validate` and `plan` on pull requests, with
  the plan posted as a comment; `apply` on merge into `main`.
- `docs/infrastructure.md`: the bootstrap block verbatim, so it can be replayed for production.

**Verification:**
- The Terraform plan of this pull request, attached.
- The apply log of the merge showing the Artifact Registry repository created.
- The owner confirms in Cloud Shell that the state bucket has versioning enabled.

**Not in this delivery:** Cloud Run, Cloud SQL, any workload.

---

### F3 — Cloud Run in the lab and continuous deployment on merge

- [x] **Objective:** merging into `main` deploys the binary to the lab automatically, and the lab
  host answers over HTTPS (§27, design §3).

**Depends on:** F1, F2.

**What is needed from the owner:**
1. In Cloudflare, create the CNAME `marketplace.lab.aleogr.dev` pointing to `ghs.googlehosted.com`
   in **DNS only** mode (§7).
2. Report when the certificate has been issued; this can take up to 24 hours and nothing else in
   the phase waits for it.

**Scope:**
- Terraform: the Cloud Run service (minimum instances 0, high concurrency, the smallest useful
  CPU and memory), its own service account with least privilege, **every environment variable
  declared with an explicit value** including `INDEXABLE=false` (§7.1), and the domain mapping.
- `.github/workflows/deploy.yml`: build, push to Artifact Registry tagged with the commit SHA,
  deploy **by digest**.
- `.github/workflows/release.yml`: a `v*` tag creates the GitHub Release with generated notes.
  The promotion of that same digest to production belongs to the phase that creates the production
  project (section 8).

**Verification:**
- `curl -i https://marketplace.lab.aleogr.dev/health` returning 200, output attached.
- The start-up log in Cloud Logging showing the build identifier of the merged commit.
- `pytest e2e/` run against the lab URL, not only against a local process.

**Not in this delivery:** a production environment, a load balancer, custom marketplace domains.

---

### F4 — Cloud SQL, migrations and the database layer

- [x] **Objective:** the application talks to PostgreSQL, schema changes are versioned and applied
  by the pipeline, and integration tests run against a real PostgreSQL with migrations applied
  (design §2.8).

**Depends on:** F3.

**What is needed from the owner:**
1. **Confirm that the billing account is an upgraded, paid Cloud Billing account, not a free
   trial.** A free trial account stops every resource in the project when the trial period ends,
   and the period can be neither paused nor extended, so a Cloud SQL instance created under one
   has an expiry date rather than a lifetime. Upgrading before the trial ends preserves the
   remaining credit.
2. Confirm the Cloud SQL machine tier. This is the **only material recurring cost** of the design
   (design §3) and the decision belongs to the owner.
3. Create a billing budget with an e-mail alert at a threshold the owner chooses.

**Scope:**
- Terraform: Cloud SQL for PostgreSQL 16, smallest instance, automatic backups, point-in-time
  recovery on, a maintenance window, IAM database authentication, and the connection from Cloud
  Run through the connector (design §3). Retention is per environment: the design's 30 days are
  production's, and the lab keeps less because it is rebuilt from this repository
  (`docs/infrastructure.md`).
- `migrations/`: SQL migrations managed by an embedded migration tool, embedded with `go:embed`
  and applied by a `migrate` subcommand of the **same binary**, so there is no second image.
- Terraform: a Cloud Run Job running that subcommand, and a deploy pipeline step that runs the job
  **before** the new revision receives traffic (design §3).
- `internal/platform/db`: a `pgx` pool, a transaction helper that will later set the tenant
  variable, and a readiness check that is part of `/health`.
- `internal/platform/dbtest`: starts a local PostgreSQL 16 cluster for a session run, or uses the
  service container's `DATABASE_URL` in CI; integration tests are behind a build tag so
  `go test -short` stays fast.
- `.github/workflows/ci.yml`: a `postgres:16` service container for the integration job.

**Verification:**
- Integration tests green locally and in CI, output attached.
- The migration job's log from the deploy that applied the first migration.
- The owner runs one `psql` command in Cloud Shell listing the applied migrations.
- A test that proves a migration is idempotent when applied twice.

**Not in this delivery:** any domain table beyond the migration bookkeeping.

---

### F5 — Markets, marketplaces and host resolution

- [x] **Objective:** the host of the incoming request decides which marketplace serves it, and
  Brazil is a row in `market`, not a branch in the code (§5, §7, design §4).

**Depends on:** F4.

**What is needed from the owner:**
1. In Cloudflare, create the CNAME for the first marketplace host,
   `marketplace1.marketplace.lab.aleogr.dev`, to `ghs.googlehosted.com` in **DNS only** mode, and
   report when the certificate is issued.

**Scope:**
- Migrations creating `market` (currency, enabled payment methods, tax and consumer rules as data
  **with effective dates**, address format), `marketplace` (market, languages, revenue model, the
  per-marketplace flags of design decision 2, state `in_preparation` / `active`) and
  `marketplace_host`.
- A seed migration creating the `BR` market, which is environment-neutral: Brazil is Brazil
  everywhere. The **marketplaces come from the environment**, not from a migration, because their
  hosts do: a host written into a versioned migration would be created in every environment that
  ever runs it, so production would hold the lab's addresses. Terraform declares them once and uses
  them twice — a domain mapping per host, and the same list seeded by the migration job. The
  platform host is configuration for the same reason, and resolves to the platform rather than to
  a marketplace.
- `internal/tenancy`: service and repository, with a short-lived in-process cache of the host map
  and explicit invalidation.
- The host-resolution middleware, first in the pipeline (design §2.3), and an explicit page for an
  unknown host — never a silent fallback to a default marketplace.

**Verification:**
- Unit tests over a resolution table: known marketplace host, platform host, unknown host, a host
  with a port, an uppercase host, a marketplace still `in_preparation`.
- An integration test with two marketplaces proving the context carries the right one.
- An end-to-end test against the lab reaching both hosts, with screenshots. It names the hosts in
  `MARKETPLACE_HOSTS`, and the deployment pipeline passes every host the environment declares, so
  each deployment proves them in a browser and keeps the screenshots. The cost is accepted: a host
  mapped for the first time fails that step until Cloud Run has issued its certificate, and the
  deployment is re-run once it exists.
- The page for an address no marketplace claims, checked on every deployment. It is asked of the
  service's own `run.app` address, named in `MARKETPLACE_UNCLAIMED_URL`: that address waits for no
  certificate, and marketplaces are reached through the domains mapped to them, never through it.
  An unknown `Host:` header sent to a mapped address would prove nothing — Google's front end routes
  by that header and refuses one it has no mapping for before the request reaches the binary.

**Not in this delivery:** the console wizard that creates a marketplace (design decision 16); its
infrastructure checklist is written in F19 and the wizard belongs to a later phase.

---

### F6 — Row-level security and tenant isolation

- [x] **Objective:** a query that forgets the marketplace filter returns nothing, instead of
  returning another marketplace's rows (design §2.6).

**Depends on:** F5.

**What is needed from the owner:** nothing.

**Scope:**
- The application database role is **without** `BYPASSRLS`, and is not the owner of the tables; the
  separate role that runs the migrations owns them and is therefore the one that still sees every
  marketplace, which the seed needs. Both roles already exist (F4); this delivery is what makes the
  difference between them matter.
- `ENABLE ROW LEVEL SECURITY` plus policies on every tenant table, keyed on a session variable.
- The transaction helper sets that variable with `SET LOCAL`, so it cannot leak to another
  connection in the pool.
- A schema-wide test that fails when **any** table carrying `marketplace_id` lacks row-level
  security or a policy, so a future table cannot forget it.
- Host resolution reads the routing map through one function that runs as the owning role, because
  it runs before a marketplace is known and would otherwise see nothing. One named exception, so
  the routing tables are covered by the policies rather than left outside them.

**Verification:**
- Integration tests: the same query under two tenants returns disjoint rows; a deliberately
  unfiltered query returns zero rows; a direct connection as the application role cannot read
  another tenant's row.
- The schema-wide test output attached.

---

### F7 — Request pipeline: security headers, CSRF and rate limiting

- [x] **Objective:** the binary defends itself, because there is no load balancer and no web
  application firewall in front of it (§24).

**Depends on:** F5.

**What is needed from the owner:** nothing.

**Scope:**
- `internal/platform/httpx`: security headers mounted once — a Content Security Policy with
  per-request nonces sized for templ, HTMX and Alpine.js, HSTS, `X-Content-Type-Options`,
  `Referrer-Policy`, `Permissions-Policy` and `frame-ancestors`.
- CSRF protection for every state-changing request, with cookies marked `SameSite=Lax`, `Secure`
  and `HttpOnly`, and an HTMX-aware token header.
- Rate limiting in two tiers: an in-memory token bucket per instance for general traffic, and a
  **database-backed counter** for the sensitive endpoints (sign-in, sign-up, recovery, and later
  checkout), because Cloud Run runs several instances and a per-instance counter would multiply
  the effective limit by the instance count.
- The request-origin context — client IP and user agent — that auditing consumes in F12. The client
  address is read from the **right** of `X-Forwarded-For`, not the left: Google's front end appends
  to that header rather than replacing it, so everything to the left of what the infrastructure
  added is written by whoever is being limited. How many entries it appends is in no document that
  could be found, so it is configuration (`TRUSTED_PROXY_HOPS`, default 2) and it is measured
  against the deployment rather than assumed.
- The database-backed limiter is built and proved here; the endpoints it guards arrive with
  sign-in (F10), which mounts it.

**Verification:**
- Unit tests per middleware, including a CSRF token replayed from another session being refused.
- An integration test running two limiter instances against one database, proving the sensitive
  limit holds across instances.
- An end-to-end assertion that every response carries the headers, with `curl -I` output attached.

**Note:** the limits are constants here and become console parameters in F17.

---

### F8 — Internationalization: en-US and pt-BR

- [x] **Objective:** both languages fully working, the language in the URL, and no user-facing
  text outside a translation key (§6).

**Depends on:** F5.

**What is needed from the owner:** nothing.

**Scope:**
- `internal/platform/i18n`: catalogues under `web/locales/`, embedded with `go:embed`; plural and
  formatting rules from `golang.org/x/text`; money formatted from minor units and a currency code;
  dates stored in UTC and formatted per locale.
- Language resolution in the order of design decision 10: **an explicit language in the URL always
  wins**; then the remembered choice in a one-year cookie; then country detection (the `geoip`
  port, satisfied by the fake until F18); then `en-US`.
- Plural forms follow CLDR, with an exact `=0` form available where a language needs one. Portuguese
  needs one: CLDR puts zero in its `one` category, which would have the platform say
  "0 anúncio".
- Canonical BCP 47 prefixes `/en-US/` and `/pt-BR/`; a lowercase variant redirects with **301** to
  the canonical form; `hreflang` and `x-default` links on every page.
- A manual language switch on every page, which writes the cookie.
- Two CI checks: a catalogue-parity check failing when a key exists in one language and not the
  other, and a check failing when a template contains a user-facing literal outside a translation
  call. Both are tests, so they run wherever the tests run, and both were checked against the
  failure they exist to catch.
- A third, which the delivery found it needed: the generated template code is committed, and
  `make check` fails when it is out of date. A template edited without regenerating would
  otherwise serve the old markup with no check saying so.

**Verification:**
- Unit tests over the resolution table, enumerating: explicit URL against a contrary cookie,
  cookie without URL, neither, lowercase URL, an unknown language tag, a language not enabled for
  the marketplace.
- An end-to-end test visiting `/en-us/`, asserting the 301, switching to Portuguese, closing the
  browser context and returning to the root, asserting the remembered choice wins.
- Screenshots of the same page in both languages.

---

### F9 — Indexing control and page descriptions

- [x] **Objective:** the lab cannot be indexed by accident, and a real deployment becomes
  indexable by flipping one variable (§7.1, §7.2).

**Depends on:** F3 (the variable is declared in Terraform), F8 (the page layout exists).

**What is needed from the owner:** nothing.

**Scope:**
- `INDEXABLE` as a strict boolean. An unreadable value **refuses to start**, quoting the value;
  the mode in effect is logged on every start.
- A middleware mounted once writing `X-Robots-Tag: noindex, nofollow` on **every response the
  process writes**, not only HTML.
- `robots.txt` always serving `Allow: /` and never `Disallow: /`; when indexing is off it omits
  only the `Sitemap:` line, with a comment in its place saying why. **With indexing on the line is
  still absent for now**, with a different comment saying so: the sitemap arrives with the first
  indexable pages in phase 2, and inviting a crawler to a 404 is worse than not inviting it. The
  end-to-end test asserts each comment in its own mode, so the day the sitemap exists the test
  fails until the line is back.
- Open Graph and Twitter tags and the preview image served normally in both modes. The image is
  **drawn from the marketplace's name** rather than stored: one file per marketplace would be a
  file to remember on the day somebody creates the fourth one, and a binary blob nobody can review
  in the diff. A designed image replaces the drawn one when the brand exists.
- A single description source feeding `<meta name="description">`, `og:description` and
  `twitter:description`, cut at about 160 **runes** at the end of a sentence, never mid-word and
  never mid-rune.

**Verification:**
- Unit tests, including `INDEXABLE=ture` causing a non-zero exit with the value in the message,
  and a description ending with a multi-byte character being cut without producing invalid UTF-8.
- **End-to-end tests in both modes against the running server** (§7.1): with indexing off, every
  response of the route set carries the header and `robots.txt` has no `Sitemap:` line; with it
  on, no response carries the header and the line is back.
- Both runs' output attached.

**Note:** the sitemap itself, and the §7.2 measurement of description length **across all pages in
the sitemap**, land in phase 2 with the first indexable pages. The helper and its unit tests exist
here so phase 2 only adds the measurement.

---

### F10 — Outbox, Cloud Tasks, Cloud Scheduler and the job runner

- [x] **Objective:** work that must not happen inside a request happens exactly once, without an
  always-on worker (design §2.5).

**Depends on:** F6.

**What is needed from the owner:** nothing; both services stay within the free quota at this
volume (design §3).

**Scope:**
- `outbox_event`, written **in the same transaction** as the data that caused it.
- A dispatcher delivering each event to a Cloud Tasks queue with an idempotency key; one queue per
  class of work (`webhooks`, `notifications`, `jobs`), declared in Terraform with its retry policy.
- An internal endpoint receiving the callbacks, authenticated by the **OIDC token** of the invoker
  service account and validated against the expected audience and service account; anything else
  is refused.
- A consumer registry, idempotent by event id.
- `job_lock` keyed by job name with a lease, so two overlapping runs cannot both proceed.
- Cloud Scheduler jobs declared in Terraform, calling the same endpoint. The first of them is the
  dispatcher itself: emptying the outbox on the way out of a request would make one visitor pay for
  everybody's work.
- The callbacks are addressed to the **platform's own host**, declared as a custom audience on the
  service. The generated `run.app` address cannot be used: it does not exist until the service does,
  so naming it in that service's own environment would be a resource referring to itself.
- A fake dispatcher running consumers inline, used by tests.

**Verification:**
- Integration tests: an event written in a transaction that rolls back is never delivered; a
  failing consumer is retried and then parked; a duplicate delivery is a no-op; of two concurrent
  runs of the same job only one takes the lock; an unsigned or wrongly-audienced call to the
  internal endpoint is refused.
- Evidence from the lab that a scheduled job ran: the Cloud Logging entry attached.

---

### F11 — E-mail port and adapter

- [x] **Objective:** the platform sends transactional e-mail in both languages and stops sending
  to addresses that bounced or complained (§17, §25).

**Depends on:** F10, F8.

**Provider: Brevo** (§25, updated by this delivery). The section left the choice between Amazon SES
and Resend; Brevo was chosen with the owner because its free tier covers the platform until it has
revenue, it needs no AWS account and no sandbox exit, and its transactional API is two endpoints.
The consequence is written into the adapter: Brevo **does not sign its webhooks**, so every event it
posts is re-read from its API before the platform acts on it.

**What is needed from the owner** (one at a time, none of them blocking the merge):
1. Create the API key in the Brevo account (SMTP & API → API Keys).
2. Store it with the single command provided; the credential never enters the repository.
3. Publish the DKIM, SPF and DMARC records of the platform's sending domain in Cloudflare — one
   record at a time — and confirm the sender verifies at the provider.
4. Turn `providers_mode` to `real` in the environment's tfvars, with `mail_from`.
5. Paste the webhook address and its token — both are outputs of the infrastructure — into the
   provider's console, so bounces and complaints reach the platform.

**Scope:**
- The `mail` port: a message with a template, a language, variables and the name it is from.
- The real adapter (Brevo) and a fake writing to a directory the end-to-end suite reads, selected
  by `PROVIDERS_MODE`.
- A contract test suite run against **both** adapters, so a second provider can be added later
  without re-deriving the expectations.
- Templates with a text part and an HTML part, in `en-US` and `pt-BR`. A template missing in a
  language the platform speaks refuses the start-up.
- `email_suppression`, fed by bounce and complaint webhooks arriving through the F10 webhook path,
  checked before every send; `email_send`, the log of what was sent, skipped and failed, under
  row-level security.
- The sending job, consuming outbox events, and `send-probe`, which sends one message by hand to
  prove a sending domain works.

**Verification:**
- Contract tests green for both adapters (`make test`).
- An integration test against a real PostgreSQL: a bounce webhook suppresses the address, and a
  later send to it is skipped and recorded as skipped; an event the provider does not confirm
  suppresses nobody; the sending log of one marketplace is invisible to another.
- End-to-end tests reading the fake mailbox, in both languages (`make e2e`).
- One real e-mail delivered from the lab to the owner's address, on 19 September 2026, read in
  the inbox rather than the spam folder (`docs/infrastructure.md`). The lab has run the real
  adapter since.
- The webhook is configured in the provider's console and the path is proved in the lab: a call
  with no token and a call with the wrong one are refused, a call carrying it is accepted and
  written to the outbox, and an event the provider does not confirm suppresses nobody. That test
  found two defects, both fixed and both now held by tests (`docs/infrastructure.md`).
- A suppression following a **real** hard bounce, observed in the lab on 19 September 2026: one
  message to an unregistered address at a large provider, the bounce reported by the provider a
  second later, the address suppressed within the minute, and the next message to it skipped and
  recorded as skipped. The sequence is in `docs/infrastructure.md`. None of the platform's own
  domains could produce it — the sending domain has no MX, which the provider calls a *soft*
  bounce, and the other two are catch-all.

**If the owner steps are not ready:** the delivery merges with the fake adapter selected in the
lab by the `PROVIDERS_MODE` variable, and the real adapter is switched on by a one-line Terraform
change afterwards. Nothing downstream waits for it.

---

### F12 — Audit log

- [x] **Objective:** every operation is traceable — who, what, when, from where, and the state
  before and after — and a deletion request erases the personal content without breaking the
  chain (§21, §18.3).

**Depends on:** F10, F7, F6.

**What is needed from the owner:** nothing. Cloud KMS adds a negligible monthly cost per key
version, noted in the cost inventory of F19.

**Scope:**
- `audit_log`, **append-only** twice over: `UPDATE` and `DELETE` revoked from the application role,
  and a trigger that raises an exception if they are ever attempted by another role.
- **Tamper-evident:** each record carries the hash of the previous one. The chain is kept **per
  marketplace, plus one for the platform**, because a single global chain would serialise every
  write in the system through one row.
- Records reference people by internal identifier only — never by name, e-mail or document number.
- Fields that inherently carry personal data (the state before and after, the origin IP) are
  encrypted with a **per-user key**, kept in a separate table and wrapped by a Cloud KMS key
  declared in Terraform.
- A deletion request is fulfilled by **destroying that key**: the record and the chain stay intact
  and the content becomes unrecoverable.
- A Cloud Scheduler job verifying the chain and raising an alert on a break.

**Verification:**
- Integration tests against a real PostgreSQL: `UPDATE` and `DELETE` refused by permissions and by
  the trigger; a record altered out of band is detected and named by the verification; destroying a
  key leaves the content unrecoverable while the chain still verifies; a record holds no personal
  data in clear text; each marketplace's chain is its own and a marketplace sees only its records.
- Both belts were mutation-checked, and each one catches what the other does not: with the revoke
  removed the application role is stopped by the trigger, and with the trigger removed the role
  that owns the table is stopped by nothing.
- Delivered in two pull requests, because one was too large to review: the key keeper first, the
  log itself second.

**Note:** the retention periods that delay key destruction are an open §29 topic awaiting the
lawyer. The provisional defaults of design decision 22 are implemented as parameters in F17, so
the lawyer's answer changes a value, not code.

---

### F13 — Identity core: users, credentials and sessions

- [ ] **Objective:** a person creates an account in a marketplace, signs in, signs out, changes
  the password, and every other session of that account ends with the change (§18.1).

**Depends on:** F12, F11, F7, F8.

**What is needed from the owner:** nothing.

**Scope:**
- `account` (per marketplace; staff belong to the platform and carry no marketplace), `credential`,
  `session`.
- Passwords hashed with **argon2id**, with the parameters chosen by a benchmark on the Cloud Run
  CPU and recorded next to the code with the measurement, so a future change is an informed one.
- Sessions stored **server-side and revocable from the first release** (§18.1): the cookie carries
  an opaque token, the database stores only its hash, plus last-seen time, IP and user agent.
- Sign-up with e-mail verification; sign-in; sign-out; password change revoking all other sessions.
- The F7 rate limiter applied to sign-in, sign-up and verification.
- An audit record for each of these actions.
- templ pages for all of it, in both languages.

**Verification:**
- Unit tests on the hashing parameters, on tokens being stored and found only as their hash, and on
  password verification comparing in constant time.
- Integration tests: a revoked session is refused on the next request; a password change ends the
  other sessions; an unverified account cannot sign in; the rate limiter blocks a credential-
  stuffing pattern.
- An end-to-end test in two browser contexts: sign up, read the verification link from the fake
  mailbox, sign in on both, change the password on one, assert the other is signed out.
- Screenshots of every screen in both languages.

**Note:** the screen that lists active sessions and signs them out may arrive after the first
release (§18.1); the revocation it depends on is delivered here.

---

### F14 — Two-factor authentication

- [ ] **Objective:** security keys, authenticator apps and e-mail codes, with the per-user-type
  policy and the recovery codes of §18.2.

**Depends on:** F13, F11.

**What is needed from the owner:** nothing during the delivery. A real security key can be tested
later, at the owner's convenience; the automated verification uses a virtual authenticator.

**Scope:**
- `second_factor` (type and a secret encrypted with the user's key from F12) and `recovery_code`
  (hashed, single use), generated when an authenticator app or a security key is enrolled.
- **TOTP** (RFC 6238), with a ±1 step window and replay protection by storing the last accepted
  step.
- **WebAuthn** security keys, with the relying-party identifier derived from the host.
- **E-mail codes**, short-lived and rate limited.
- The policy as a service, exactly as §18.2 states it: owner and staff **must** use an
  authenticator app or a security key, and an e-mail code is never accepted as their second
  factor, because e-mail is their recovery channel; stores are stepped up on sensitive actions,
  with the e-mail code as the default for a new store; buyers are optional with a step-up by
  e-mail code.
- Enrolment and verification screens in both languages; an audit record for every enrolment,
  removal and step-up.

**Verification:**
- Unit tests using the RFC 6238 test vectors, a replayed code refused, an expired e-mail code
  refused, a recovery code usable exactly once.
- An integration test proving a staff user cannot enrol an e-mail code as their second factor.
- End-to-end tests: the TOTP path computing the code in the test; the WebAuthn path with a
  **virtual authenticator** driven through Chrome DevTools; the e-mail path via the fake mailbox.
- Screenshots of the enrolment flows in both languages.

**Note:** the buyer step-up when adding a card belongs to the checkout phase, which is when cards
exist. The policy hook for it is delivered here.

---

### F15 — Roles, permissions, owner bootstrap and the console shell

- [ ] **Objective:** the owner account exists with no credential in the repository, staff access
  is governed by roles scoped to marketplaces, and the deployed build is visible to staff (§19,
  §27).

**Depends on:** F14.

**What is needed from the owner** (one at a time, against the lab):
1. Read the one-time bootstrap token from Secret Manager with the command provided.
2. Open the first-run URL, set the owner e-mail and password.
3. Enrol the owner's second factor — an authenticator app or a security key — and store the
   recovery codes somewhere safe. Confirm a successful sign-in afterwards.

**Scope:**
- `permission` (a registry declared in Go and seeded by migration, so a permission cannot exist in
  the database without existing in the code), `role`, `role_permission`, `role_scope` (a specific
  marketplace or the whole platform), `user_role`.
- The **owner role cannot be deleted and cannot lose permissions**, enforced in the service and
  again by a database constraint, and it requires a second factor (§19).
- **First run:** while no owner exists, a `/setup` route is served, guarded by a one-time token
  generated at start and written only to the log and to Secret Manager. The token expires on use
  and the route disappears once the owner exists. No credential is ever hard-coded (§19).
- The authorization middleware, and a console shell: templ layout, navigation, language switch and
  the **build identifier in the footer** (§27).
- A `/console/version` endpoint restricted to a staff permission, exposing the build identifier.
- An audit record for every role and permission change (§19).

**Verification:**
- Integration tests: a staff member scoped to marketplace A cannot read marketplace B; the owner
  role cannot be deleted or stripped; the setup route is gone after bootstrap; the bootstrap token
  cannot be used twice; a user without the permission gets 404, not 403, on the version endpoint.
- An end-to-end test running the whole bootstrap against a fresh database, then creating a role,
  assigning it, signing in as that staff member and asserting the menu shows only what the role
  permits.
- Screenshots of the console in both languages, and the audit records produced by the run.

---

### F16 — Account recovery

- [ ] **Objective:** stores and buyers recover through e-mail with a security delay and full
  notification; staff are recovered only by an administrator, with an audit record (§18.2).

**Depends on:** F15.

**What is needed from the owner:** nothing.

**Scope:**
- A recovery request producing a single-use, expiring token, stored hashed.
- A **security delay** before the reset takes effect (default 24 hours, a parameter registered in
  F17), during which a "this was not me" link cancels it.
- Notification on every channel the account has enabled, and revocation of **all** sessions when
  the recovery completes.
- The recovery-code path for accounts with an authenticator app or a security key.
- The staff path: an administrator resets the factor, never self-service, and the reset is audited.
- An injected clock, so the delay can be tested without waiting.

**Verification:**
- Integration tests: the delay is honoured; the cancel link stops the reset; the token works once;
  every session is revoked; a staff account cannot self-recover.
- An end-to-end test using the fake mailbox and the injected clock, covering both the completed
  and the cancelled paths.
- Screenshots in both languages.

---

### F17 — Parameter store and its console screen

- [ ] **Objective:** behaviour that changes with operation rather than with code is a parameter
  with a default, validation and an audited change history (§23, design decision 15).

**Depends on:** F15, F12.

**What is needed from the owner:** nothing.

**Scope:**
- A typed registry declared in Go: key, scope (platform or marketplace), type, default,
  validation, and a description in both languages. A key that is not in the registry cannot be
  stored.
- The `parameter` table with effective dates and the author of each change, read through a cache
  invalidated on write.
- The console screen: list, edit, and the change history of each key.
- An audit record for every change (§23).
- **Only the keys whose behaviour exists in phase 1 are registered here** — the rate limits of F7,
  the session lifetime of F13, the recovery security delay of F16, the trusted-proxy count of F18,
  the default language per marketplace, and the provisional retention defaults of design decision
  22. Every later phase registers its own keys; this roadmap does not invent parameters for
  features that do not exist yet.

**Verification:**
- Integration tests: the default applies when nothing is stored; validation refuses a bad value;
  an entry with a future effective date does not apply yet; the history and the audit record are
  both written; a key outside the registry is refused.
- An end-to-end test lowering a rate limit in the console and observing the next request blocked —
  the parameter proving itself by changing behaviour, not by displaying a value.
- Screenshots in both languages.

---

### F18 — IP geolocation and country detection

- [ ] **Objective:** a visitor with no remembered choice and no language in the URL lands on the
  right language, and the geolocation database stays up to date at no runtime cost (§6, design
  decision 11).

**Depends on:** F8, F10, F2.

**What is needed from the owner** (one at a time):
1. Create the MaxMind account and accept the GeoLite2 licence.
2. Generate a licence key.
3. Add the key as a repository secret for the weekly refresh workflow.
4. Store the same key in Secret Manager with the command provided, if the service is to refresh
   out of band.
5. Confirm the wording and placement of the GeoLite2 attribution in the footer, which the licence
   requires.

**Scope:**
- Terraform: a private bucket for the database file.
- A weekly GitHub Action downloading GeoLite2-Country, verifying its checksum and uploading it to
  the bucket. Nothing is committed to the repository and nothing is embedded in the binary.
- The `geoip` port with the fake used by every test, and the real adapter loading the database
  from the bucket at start into memory, with a periodic reload.
- Client IP extraction: **on Cloud Run the last address in `X-Forwarded-For` is the one appended
  by Google's front end**; the number of trusted proxies is a parameter, ready for the day
  Cloudflare proxies traffic (design decision 11).
- Detection applied **only** when there is no remembered choice and no language in the URL;
  visitors from Brazil are redirected to `pt-BR` (§6).
- The attribution in the footer, in both languages.

**Verification:**
- Unit tests over an IP-extraction table: a direct connection, one proxy, two proxies, a spoofed
  header prepended by the client, IPv6, and a malformed header.
- An integration test against a small fixture database.
- An end-to-end test with a forged `X-Forwarded-For` asserting the redirect to `/pt-BR/`, and a
  second asserting that a remembered `en-US` choice defeats the detection.
- The first successful run of the weekly workflow, attached.

**If the owner steps are not ready:** the delivery merges with the fake adapter active, which
returns no country and therefore falls through to `en-US`. The real adapter is switched on by a
Terraform variable afterwards.

---

### F19 — Operational readiness and phase closure

- [ ] **Objective:** the foundation is operable by one person — alerts arrive, a restore has been
  rehearsed, the runbooks exist, and the phase's evidence is collected in one place (§26, design §3).

**Depends on:** every delivery above.

**What is needed from the owner** (one at a time):
1. Confirm the e-mail address that receives alerts, and click the verification link the GCP
   notification channel sends.
2. Execute the first **Cloud SQL restore test** into a temporary instance, following the runbook
   step by step in Cloud Shell, and confirm the temporary instance was deleted afterwards.
3. Confirm the quarterly cadence for repeating it (design §3).

**Scope:**
- Terraform: the notification channel and the alert policies — 5xx rate, failed Cloud Tasks
  deliveries and failed Cloud Scheduler jobs, Cloud SQL CPU and disk, domain-mapping certificate
  expiry, and a failure of the audit chain verification.
- `docs/runbooks/`: deploy and rollback by image digest; restore from point-in-time recovery;
  rotate a secret; revoke all sessions of an account; respond to an audit chain break; and the
  incident record structure designed in `docs/design.md`, section 7.
- The **marketplace creation infrastructure checklist** of design decision 16, written as an
  ordered list of owner steps: DNS in Cloudflare, domain mapping, e-mail domain records,
  provider production access — a marketplace leaves `in_preparation` only when its host answers
  and its e-mail domain verifies.
- `docs/verification/phase-1.md`: the phase's evidence — test outputs, screenshots in both
  languages, the alert that fired, the restore test log — collected as the definition of done
  requires (§3).
- The recovery targets recorded and measured: recovery point objective 5 minutes through
  point-in-time recovery, recovery time objective 4 hours, backup retention 30 days, with the date
  of the last restore test stored as a parameter (design §3).
- The fixed-cost inventory updated with the actual first month's bill (design §3).

**Verification:**
- An alert deliberately triggered and the e-mail received, screenshot attached.
- The restore test executed end to end, with the measured recovery time compared against the
  4-hour objective.
- A full green pipeline and the complete end-to-end suite run **against the lab**, not only
  locally.
- Every checkbox in section 5 of this document ticked, with the pull request link next to it.

---

## 6. Order and parallelism

Hard dependencies only. Deliveries that share a dependency and do not depend on each other can be
worked in any order, or in parallel branches, once that dependency has merged.

| Delivery | Depends on | Can start as soon as |
|---|---|---|
| F1 | — | now |
| F2 | F1 | F1 merges |
| F3 | F1, F2 | F2 merges |
| F4 | F3 | F3 merges |
| F5 | F4 | F4 merges |
| F6 | F5 | F5 merges |
| F7 | F5 | F5 merges |
| F8 | F5 | F5 merges |
| F9 | F3, F8 | F8 merges |
| F10 | F6 | F6 merges |
| F11 | F8, F10 | F10 merges |
| F12 | F6, F7, F10 | F10 merges |
| F13 | F7, F8, F11, F12 | F11 and F12 merge |
| F14 | F11, F13 | F13 merges |
| F15 | F14 | F14 merges |
| F16 | F15 | F15 merges |
| F17 | F12, F15 | F15 merges |
| F18 | F2, F8, F10 | F10 merges |
| F19 | all of the above | F18 merges |

- **F1 to F5 are strictly sequential.** Each one is the ground the next stands on.
- **F6, F7 and F8 are independent of each other.** If the owner is waiting on a certificate for
  F5's marketplace host, F6 and F7 still proceed.
- **F9 waits only for F8**, since F3 is already behind it.
- **F11 and F18 have owner-supplied accounts.** Both merge behind their fake adapter if the owner
  step is not ready, so neither can stall the phase.
- **F13 to F17 are sequential**, because each builds on the identity model of the one before.

## 7. External dependencies of the phase, with the recommended moment

From `docs/design.md`, section 8, restricted to what this phase touches, plus the moment each one
should be started so that the answer exists when it is needed. Items whose answer is needed by a
later phase are listed because the design says to start them **now** or **during the foundation**.

| Dependency | Needed by | Recommended moment | Blocks phase 1? |
|---|---|---|---|
| GCP project and billing account | F2 | **Immediately** — the first blocking step of the phase | Yes, from F2 onward |
| Branch protection and Dependabot in GitHub | F1 | Immediately | No, but the pipeline is advisory until it is set |
| `terraform` added to the environment setup script | F2 | With F2 | No; CI validates in the meantime |
| Cloudflare CNAME for the platform host | F3 | When F2 merges; the certificate can take 24 h | Only the live check of F3 |
| Cloudflare CNAME for the first marketplace host | F5 | When F3 merges | Only the live check of F5 |
| Billing account upgraded from free trial to paid | F4 | **Before F4 creates the instance.** A free trial stops every resource when it ends, and cannot be paused or extended | Yes, from F4 onward |
| Cloud SQL tier decision and budget alert | F4 | When F3 merges | Yes, from F4 onward |
| E-mail provider account, domain verification, production access | F11 | **Start when F10 merges.** Design §8 places it in the foundation phase because sign-up and password recovery depend on e-mail; production access can take a business day | No — F11 merges with the fake adapter |
| MaxMind account and GeoLite2 licence key | F18 | **Start when F8 merges.** Design §8 places it in the foundation phase | No — F18 merges with the fake adapter |
| Owner first-run bootstrap in the lab | F15 | When F14 merges | Yes for F15's verification |
| Alert e-mail address and the first restore test | F19 | At F19 | Yes for F19 |
| **Lawyer** (terms of use, right of withdrawal for vehicles, retention periods, incident duties) | Phases 5 and 7 | **Now**, in parallel. The answers are needed before checkout reaches the lab with real stores | **No** — the provisional defaults of design decision 22 are parameters, so the answer changes a value, not code |
| **Commercial proposals** from Pagar.me and Asaas, and PagBank on multi-store carts | Phase 5 | **During this phase**, so the answers exist when the payments phase starts (design §8) | No |
| Payment gateway sandbox accounts | Phase 5 | Start of phase 5 | No |
| Melhor Envio written confirmation, OAuth app and sandbox | Phase 6 | Start of phase 6 | No |
| Production-domain spike | Phase 7 | Before the launch phase | No |
| Provider homologations and the manual test plan | Phase 7 | Launch phase | No |

## 8. Explicitly out of phase 1

Listed so that no delivery above quietly grows to include them. Each has its place in
`docs/design.md`, section 10.

**Belongs to a later phase:**
- Categories, attributes, products, variants, offers, media, pre-publication moderation, search,
  filters, comparison, public questions and answers — **phase 2**.
- Store onboarding with the payment gateway, KYC and the verified-identity badge, reputation,
  sanctions and appeals, buyer–seller messaging — **phase 3**.
- Paid listings, dealer plans, visible contact, the vehicle reservation — **phase 4**.
- Cart, checkout, stock reservation, split payments, payout holds, installments, refunds —
  **phase 5**.
- Shipping quotes, labels, tracking, payout release, returns, disputes, chargeback evidence —
  **phase 6**.
- Reviews, the complete notification system with per-channel preferences and an in-app centre,
  LGPD self-service screens and consent management, console metrics, production domains, provider
  homologations, the manual test plan and release `1.0.0` — **phase 7**.

**Deliberately deferred inside topics this phase does touch:**
- **The production environment and the production half of continuous delivery.** §27 defines two
  triggers, but §27 also says the environments are "lab now, production in the future". Phase 1
  ships the lab trigger and the release workflow that creates the GitHub Release; promotion of the
  same image **by digest** to production lands with the production project. This is sequencing,
  not a change to the requirement.
- **The sitemap**, and with it the §7.2 measurement of description length across all pages in the
  sitemap. There are no indexable pages to list yet. The description helper and its rune-safe
  truncation are delivered in F9; phase 2 adds the sitemap and the measurement.
- **The screen listing active sessions and devices.** §18.1 allows it after the first release. The
  server-side revocation it depends on is delivered in F13.
- **The console wizard that creates a marketplace** (design decision 16). Its infrastructure
  checklist is written in F19; the wizard itself belongs to a later phase, because it configures
  root categories and a revenue model, neither of which exists yet.
- **Behaviour events and analytics aggregates** (§22). They are written from the outbox, which F10
  delivers, but the events themselves come from catalog and commerce actions.
- **The buyer step-up when adding a card.** The policy hook is delivered in F14; the action it
  guards exists in phase 5.
- **The translation provider integration** (§25). Listings are displayed in their original
  language in the MVP; automatic translation is a later phase.
- **A dedicated search engine, a load balancer, Kubernetes, a paid APM.** Not planned; the design
  states why (design §2.7, §3, §26).

## 9. Risks specific to this phase

- **Certificate issuance for a domain mapping takes up to 24 hours** (§7). Every delivery that
  needs a host is written so that only its live check waits, never its implementation.
- **Cloud Run domain mapping is a preview feature** (§7). The spike that decides the production
  approach is scheduled before launch; nothing in phase 1 depends on its outcome, because the
  service resolves the marketplace from the host it receives, whatever put it there.
- **Cloud SQL is the phase's only material recurring cost** (design §3). F4 makes the owner
  confirm the tier and set a budget alert before the instance is created.
- **A billing account still in its free trial has an expiry date, and so does everything in it.**
  The trial stops every resource when it ends and cannot be extended, which would take the lab
  database down mid-phase. F4 checks for the upgrade before creating the instance.
- **The pipeline grows faster than the code it protects.** F1 deliberately front-loads it, so that
  no delivery ever merges under a weaker set of checks than the one before it.
- **A wrong `INDEXABLE` value is invisible for months** (§7.1). F9 makes it a start-up failure and
  verifies both modes against the running server, not only in unit tests.
- **One binary and one person** (design §9). F19's runbooks and alerts are the mitigation, which
  is why they are a delivery and not an afterthought.

## Appendix — environment facts verified in this session

Checked directly, because the deliveries above depend on them.

| Fact | Value | Consequence |
|---|---|---|
| Go | `go1.24.7` on the path, but `GOTOOLCHAIN=auto` fetches what `go.mod` asks for | The pinned lint and security tools require a newer Go, so `go.mod` names the exact toolchain; tools are pinned with `tool` directives |
| Go toolchain and `govulncheck` | Every 1.26 patch release below `go1.26.6` carries standard-library vulnerabilities the code reaches through `http.Server.Serve` | The toolchain is pinned to the first release `govulncheck` accepts, and Dependabot keeps it moving |
| `pytest` on the path | A `uv` tool with its own interpreter, which does **not** have Playwright installed | The suite runs as `python3 -m pytest`; the environment setup script installs `pytest` next to Playwright |
| PostgreSQL server | 16.13 installed locally (`/usr/lib/postgresql/16`) | Integration tests run against a real cluster started in the session; CI uses a `postgres:16` service container |
| PostgreSQL as root | `initdb` refuses to run as root, and refuses a data directory the `postgres` user cannot reach | The test helper starts the cluster as the `postgres` user with its data directory outside the session scratchpad |
| Docker | Installed, but **the daemon is not running** | Testcontainers are not an option; hence the helper above |
| Python / Playwright | Python 3.11.15, `playwright` importable, Chromium revision 1194 in `/opt/pw-browsers` | The end-to-end suite pins `playwright==1.56.0`, as `docs/claude-code-environment.md` explains |
| `gcloud` | **Not installed** | Every GCP manual step is written for the owner's Cloud Shell, never run from a session |
| `terraform` | **Not installed in the image**; F2 added `make terraform-deps` to the environment setup script | Terraform applies only in CI, which is where the federation's credentials exist; a session runs `make tf`, which is `fmt -check`, `init -backend=false` and `validate` |

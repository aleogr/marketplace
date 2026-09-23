# F13 — Identity core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A person creates an account in a marketplace, confirms the address, signs in, signs out and changes the password, and a password change ends every other session of that account.

**Architecture:** A new package `internal/identity` holds the rules (password hashing, tokens, the `breached` port, the account and session store, and a `Service` with one method per flow). `internal/platform/httpx` holds everything HTTP: the handlers, a session middleware mounted between language resolution and the mux, and the rate limits per route. Four pull requests, each green on its own: the end-to-end suite gains a database and local mail delivery; accounts; sessions; password change.

**Tech Stack:** Go (the release `go.mod` pins; stdlib `net/http` mux), pgx v5, goose migrations, PostgreSQL 16 with row-level security, templ, `golang.org/x/crypto/argon2`, `golang.org/x/text/unicode/norm`, Python pytest + Playwright for end-to-end, psycopg for the suite's database.

**Spec:** `docs/superpowers/specs/2026-09-23-f13-identity-core-design.md`. Read it first; this plan argues from its decisions D1–D7.

**How to read the steps.** Every code block is preceded by one of these lines, and means exactly that:
- "Create `path`:" — a new file with that content.
- "Replace the contents of `path` with:" — the whole file becomes that content.
- "Append to `path`:" — added at the end of the file, after one blank line (two in a Python file, as PEP 8 asks between top-level definitions).
- "In `path`, replace:" followed by "with:" — the first block occurs once in the file; it becomes the second.
- "Add to `web/locales/en-US.json` and `web/locales/pt-BR.json`" followed by a table — each row is one key, with its English and Portuguese text, added at the end of each file.

The whole plan was transcribed this way into a copy of `main`, and `make check`, `make test`, `make integration` and `make e2e` passed on the result at the end of each pull request.

## Global Constraints

- Everything versioned is written in **English**; the owner is addressed in Portuguese.
- The repository is **public**: no secret is committed. Model identifiers never appear in code, docs, titles or comments.
- The code is spelled in **British English**, which is the locale `misspell` checks (`.golangci.yml`): `NormaliseEmail`, the column `email_normalised`, "normalisation". No `nolint` for our own identifiers.
- Every user-facing string exists in **en-US and pt-BR** (`web/locales/*.json`); no literal text in a template (`web/literals_test.go`), including attributes a screen reader announces; every mail template exists in both languages as `.txt` and `.html` (`internal/platform/mail/templates.go` refuses to load otherwise). A mail template reads its variables flat — `{{.Name}}`, `{{.Link}}`, and `{{.From}}` for the marketplace's name — because `Render` passes one flat map, parsed with `missingkey=error`.
- Tests use **fake providers**, never real services. Integration tests run as the **application role under RLS** (`dbtest.Serving`), never as the owner. The tests of one package share one database (`internal/platform/dbtest`), so every fixture names its marketplaces uniquely.
- Passwords: **12 to 128 characters** counted in Unicode code points after NFKC normalisation; **no composition rules**; refused when found in Pwned Passwords; **accepted and logged when the service cannot be reached**. Pages and messages read both bounds from `identity.MinPasswordLength` and `identity.MaxPasswordLength`, never from a literal.
- argon2id, PHC string stored; `identity.Floor` is OWASP's minimum, **m=19456 KiB, t=2, p=1**, 16-byte salt, 32-byte key; `identity.Current` is the floor until the benchmark on the lab's CPU chooses (Task 14). The derived key is compared with `crypto/subtle.ConstantTimeCompare`.
- **argon2 never runs inside a database transaction.** A flow reads in one transaction, hashes or verifies with none open, and writes in a second one that first checks the password it verified is still the account's. The pool has four connections (`internal/platform/db`).
- Tokens: **32 random bytes**, base64url, stored only as **SHA-256**, and found by that hash: the database never compares the secret itself.
- Sessions: **30 days idle, 90 days absolute**; `last_seen_at` written at most **once an hour**; cookie `__Host-session` over HTTPS and `session` over plain HTTP, `HttpOnly`, `SameSite=Lax`, path `/`, no domain.
- E-mail verification tokens expire after **24 hours** and are single-use.
- The account table is **`account`** (never `user`).
- Every database rate limiter has a **name**, and its counters and its sweep are its own (Task 7).
- In the end-to-end suite, a page's own submit button is `main form button[type=submit]`: the header's language switch and sign-out are submit buttons too, and come first.
- Checks before every push: `make check`, `make test`, `make integration`, `make e2e` (CLAUDE.md).
- Claude Code opens pull requests and never merges. Each pull request is watched until merged.

## Pull requests and branches

| # | branch (from `main`) | tasks | depends on |
|---|---|---|---|
| A | `claude/funny-wright-379asb-f13` | the spec and this plan | — |
| 1 | `claude/funny-wright-379asb-f13e2e` | 1–2 | A |
| 2 | `claude/funny-wright-379asb-f13accounts` | 3–8 | 1 |
| 3 | `claude/funny-wright-379asb-f13sessions` | 9–12 | 2 |
| 4 | `claude/funny-wright-379asb-f13password` | 13–15 | 3, deployed, and the owner's benchmark run (Task 14) |

## Tasks

| task | title | PR |
|---|---|---|
| 1 | Deliver the outbox inside the process when providers are fake | 1 |
| 2 | A migrated, seeded PostgreSQL for the end-to-end suite | 1 |
| 3 | Passwords — the rule, the hash and the concurrency bound | 2 |
| 4 | The `breached` port | 2 |
| 5 | The accounts schema | 2 |
| 6 | The store and the sign-up, verification and resend flows | 2 |
| 7 | Each rate limiter forgets only its own windows | 2 |
| 8 | The sign-up and verification pages, wired into the process | 2 |
| 9 | The session schema and the session flows | 3 |
| 10 | The session middleware, sign-in and sign-out pages | 3 |
| 11 | End-to-end — sign-in, sign-out, refusals and the limit | 3 |
| 12 | The argon2id benchmark command | 3 |
| 12b | Sessions that can never be used again are removed | 3 |
| 13 | Change the password and end the other sessions | 4 |
| 14 | The final argon2id parameters, from the owner's run | 4 |
| 15 | The two-browser end-to-end test, the screenshots, and closing F13 | 4 |

## File Structure

| file | responsibility | PR |
|---|---|---|
| `cmd/marketplace/main.go` (modify) | local outbox dispatch in fake mode; the ping-before-listen note; one audit log; wiring of identity and sessions; `bench-password` | 1, 2, 3 |
| `cmd/marketplace/dispatch.go`, `dispatch_test.go` (create) | the in-process dispatch loop | 1 |
| `cmd/marketplace/bench.go`, `bench_test.go` (create) | the argon2id benchmark | 3 |
| `e2e/conftest.py` (modify) | a PostgreSQL for the suite, migrated and seeded; one audit key; `run_marketplace` | 1 |
| `e2e/database.py` (create) | start or reuse PostgreSQL, create a database and the application role | 1 |
| `e2e/requirements.txt` (modify) | `psycopg[binary]` | 1 |
| `e2e/test_database.py` (create) | two seeded marketplaces answer on their hosts | 1 |
| `.github/workflows/ci.yml` (modify) | a PostgreSQL service for the e2e job | 1 |
| `internal/identity/policy.go`, `password.go`, `token.go` (create) | the password rule, argon2id with a concurrency bound, tokens | 2 |
| `internal/identity/breached/breached.go` (create) | the port, the Pwned Passwords adapter, the fake | 2 |
| `migrations/00009_accounts.sql` (create) | `account`, `credential`, `email_verification` under RLS | 2 |
| `migrations/00010_redact_dispatched_mail_variables.sql` (create) | an `email.send` event's Variables are cleared once it is dispatched or parked, so the confirmation token does not stay in the table | 2 |
| `internal/platform/outbox/outbox_integration_test.go` (modify) | a dispatched or parked `email.send` event's Variables are cleared | 2 |
| `internal/identity/store.go` (create) | SQL for accounts, credentials, verifications; sessions in PR 3 | 2, 3, 4 |
| `internal/identity/identity.go` (create) | `Service`, `Visit`, the flows | 2, 3, 4 |
| `internal/identity/*_test.go` | unit and integration tests | 2, 3, 4 |
| `internal/platform/mail/templates_test.go` (modify) | sample variables for the parity check | 2 |
| `web/mail/verify-email.*`, `account-exists.*`, `password-changed.*` (create) | the three mails, both languages, text and HTML | 2, 4 |
| `internal/platform/ratelimit/database.go`, `database_integration_test.go` (modify) | named limiters; a sweep that forgets only its own windows | 2 |
| `internal/platform/httpx/render.go` (modify) | `renderStatus`: the type before the status | 2 |
| `internal/platform/httpx/identity.go` (create) | the identity handlers and their rate limits | 2, 3, 4 |
| `internal/platform/httpx/site.go` (modify) | the `identity` field, the routes, `page()` carries the signed-in name | 2, 3 |
| `internal/platform/httpx/identity_test.go`, `identity_integration_test.go`, `export_test.go` (create) | handler tests; the credential-stuffing limit against the database | 2, 3 |
| `internal/platform/httpx/session.go`, `session_test.go` (create) | the session middleware and cookie | 3 |
| `internal/platform/httpx/csrf.go` (modify) | `RenewCSRF`, sharing the secret's cookie code | 3 |
| `migrations/00011_sessions.sql` (create) | `session` under RLS | 3 |
| `migrations/00012_session_sweep_index.sql` (create) | index the sweep's `DELETE` needs on `session.marketplace_id` | 3 |
| `web/page.go` (modify) | `Form`; `Account` | 2, 3 |
| `web/identity.templ` (create) | the pages | 2, 3, 4 |
| `web/layout.templ` (modify) | the account navigation in the header | 3, 4 |
| `web/locales/en-US.json`, `pt-BR.json` (modify) | every string | 2, 3, 4 |
| `e2e/test_signup.py`, `e2e/accounts.py`, `e2e/test_signin.py`, `e2e/test_password.py` (create) | the flows and the screenshots | 2, 3, 4 |
| `docs/design.md`, `docs/infrastructure.md`, `docs/roadmap.md`, the spec (modify) | the `breached` port; the benchmark step and its numbers; F13 ticked | 2, 3, 4 |

---

# PR 1 — A database and local mail in the end-to-end suite

## Task 1: Deliver the outbox inside the process when providers are fake

**Files:**
- Create: `cmd/marketplace/dispatch.go`, `cmd/marketplace/dispatch_test.go`
- Modify: `cmd/marketplace/main.go` (`work`, and a comment at the database ping in `serve`)

**Interfaces:**
- Produces: `dispatchLocally(ctx context.Context, dispatch func(context.Context) (int, error), every time.Duration, log *slog.Logger)` — returns when `ctx` is done.

- [ ] **Step 1: Write the failing test**

Create `cmd/marketplace/dispatch_test.go`:

```go
package main

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func TestDispatchLocallyRunsUntilItsContextEnds(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		dispatchLocally(ctx, func(context.Context) (int, error) {
			if calls.Add(1) == 3 {
				cancel()
			}
			return 0, nil
		}, time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("dispatchLocally did not return after its context was cancelled")
	}
	if got := calls.Load(); got < 3 {
		t.Fatalf("dispatch ran %d times, want at least 3", got)
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./cmd/marketplace/ -run TestDispatchLocally -v`
Expected: FAIL, `undefined: dispatchLocally`.

- [ ] **Step 3: Implement**

Create `cmd/marketplace/dispatch.go`:

```go
package main

import (
	"context"
	"log/slog"
	"time"
)

// localDispatchEvery is how often a process with fake providers empties its own
// outbox.
const localDispatchEvery = time.Second

// dispatchLocally empties the outbox on a timer, in this process.
//
// Only with fake providers and no Cloud Tasks. A deployment's outbox is
// emptied by Cloud Scheduler calling the dispatch job, and there is no
// scheduler on a laptop or in the end-to-end suite: without this, a message
// requested there would sit in the outbox forever and the suite could never
// read the mail a flow sends. It calls the same dispatcher a deployment's job
// calls, so what it proves is the same path.
func dispatchLocally(ctx context.Context, dispatch func(context.Context) (int, error), every time.Duration, log *slog.Logger) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := dispatch(ctx); err != nil && ctx.Err() == nil {
				log.WarnContext(ctx, "local dispatch failed", "error", err)
			}
		}
	}
}
```

In `cmd/marketplace/main.go`, replace:

```go
	dispatcher := outbox.NewDispatcher(database, queuer, log)

```

with:

```go
	dispatcher := outbox.NewDispatcher(database, queuer, log)

	// No scheduler runs locally, so a process with fake providers dispatches
	// its own outbox (cmd/marketplace/dispatch.go).
	if cfg.ProvidersMode == config.ProvidersFake && !cfg.Tasks.Configured() {
		go dispatchLocally(ctx, dispatcher.Dispatch, localDispatchEvery, log)
		log.InfoContext(ctx, "the outbox is dispatched in this process", "every", localDispatchEvery.String())
	}

```

In `cmd/marketplace/main.go`, replace:

```go
		defer pool.Close()

		if err := pool.Ping(ctx); err != nil {
```

with:

```go
		defer pool.Close()

		// Re-examined on 2026-09-23, when the shared lab instance began to
		// sleep four nights a week: a process started while it sleeps exits
		// here and Cloud Run answers 503 until it wakes. That is kept on
		// purpose — refusing to start is what keeps a revision with a broken
		// database setting from ever taking traffic (docs/infrastructure.md).
		if err := pool.Ping(ctx); err != nil {
```

- [ ] **Step 4: Run it to see it pass**

Run: `go test ./cmd/marketplace/ -run TestDispatchLocally -race -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/marketplace/dispatch.go cmd/marketplace/dispatch_test.go cmd/marketplace/main.go
git commit -m "Deliver the outbox in the process when the providers are fake"
```

## Task 2: A migrated, seeded PostgreSQL for the end-to-end suite

**Files:**
- Create: `e2e/database.py`, `e2e/test_database.py`
- Modify: `e2e/conftest.py`, `e2e/requirements.txt`, `.github/workflows/ci.yml` (the `e2e` job)

**Interfaces:**
- Produces (pytest fixtures): `database` (session scope) → `Database(name, owner_url, app_url)`; `run_marketplace(**env) -> Marketplace`, where `Marketplace` has `base_url` (`http://127.0.0.1:PORT`), `port`, `url(host, path)` returning `http://{host}:{PORT}{path}`, `mailbox` (`Path`), `process`, `log_lines`. Hosts are `m1.localhost` (slug `m1`, name "Loja Um") and `m2.localhost` (slug `m2`, name "Loja Dois"), both `active`, market `BR`, default `pt-BR`, languages `pt-BR,en-US`. `AUDIT_LOCAL_KEY`, one key for every process of a run.

Three facts of the existing code shape the fixture. `db.Migrate` grants on the database it is told the name of (`internal/platform/db/migrate.go`, `grant`), so `migrate` is given `DATABASE_NAME` next to `DATABASE_URL`. A process with no audit key wraps with one that dies with it (`cmd/marketplace`, `protection`), so every process of a run shares one, or none could open a key another created. And `e2e/requirements.txt` holds no HTTP client: the suite speaks HTTP with `urllib`.

- [ ] **Step 1: Write the failing test**

Create `e2e/test_database.py`:

```python
"""The suite's marketplaces, served from a real database.

Until F13 every end-to-end run had no database, so no host resolved and no
flow that writes could be tested. These are the two marketplaces every
identity test signs up in (docs/superpowers/plans/2026-09-23-f13-identity-core.md).
"""

from __future__ import annotations

import json
import urllib.request

import pytest


def fetch(marketplace, host: str, path: str) -> tuple[int, str]:
    """GET a path as the given host, through the loopback address.

    The request goes to 127.0.0.1 with the host named in the header, because
    Python's resolver, unlike Chromium's, may not resolve ``*.localhost``.
    """
    request = urllib.request.Request(marketplace.base_url + path, headers={"Host": host})
    with urllib.request.urlopen(request, timeout=10) as response:
        return response.status, response.read().decode()


@pytest.mark.local_process
def test_each_seeded_host_serves_its_own_marketplace(run_marketplace):
    marketplace = run_marketplace()

    first_status, first = fetch(marketplace, "m1.localhost", "/pt-BR/")
    second_status, second = fetch(marketplace, "m2.localhost", "/pt-BR/")

    assert first_status == 200 and "Loja Um" in first
    assert second_status == 200 and "Loja Dois" in second


@pytest.mark.local_process
def test_the_health_check_reports_the_database(run_marketplace):
    marketplace = run_marketplace()
    with urllib.request.urlopen(marketplace.base_url + "/health", timeout=10) as response:
        body = json.load(response)
    assert body["database"] == "ok"
```

- [ ] **Step 2: Run it to see it fail**

Run: `make build && MARKETPLACE_BINARY=$PWD/bin/marketplace python3 -m pytest e2e/test_database.py -v`
Expected: ERROR, `fixture 'run_marketplace' not found`.

- [ ] **Step 3: The suite's PostgreSQL**

Create `e2e/database.py`:

```python
"""A PostgreSQL for the end-to-end suite, the way the Go tests get one.

Given ``TEST_DATABASE_URL`` (CI's service container), a fresh database is
created on that server. Without it, a throwaway cluster is started from the
PostgreSQL 16 binaries, exactly as ``internal/platform/dbtest`` does, and
stopped at the end of the session.
"""

from __future__ import annotations

import os
import secrets
import shutil
import socket
import subprocess
import tempfile
import time
from dataclasses import dataclass
from urllib.parse import urlsplit, urlunsplit

import psycopg

BIN = "/usr/lib/postgresql/16/bin"
APP_ROLE = "marketplace_app"


@dataclass
class Database:
    name: str
    owner_url: str
    app_url: str


def _free_port() -> int:
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


def _as_postgres(*command: str) -> None:
    subprocess.run(["su", "postgres", "-c", " ".join(f"'{c}'" for c in command)],
                   check=True, capture_output=True, text=True)


def _start_cluster():
    data = tempfile.mkdtemp(prefix="marketplace-e2e-", dir="/var/tmp")
    shutil.chown(data, "postgres", "postgres")
    os.chmod(data, 0o750)
    _as_postgres(f"{BIN}/initdb", "-D", data, "-U", "postgres", "--auth=trust", "--no-sync")
    port = _free_port()
    _as_postgres(f"{BIN}/pg_ctl", "-D", data, "-o", f"-p {port} -h 127.0.0.1 -k {data} -F",
                 "-l", f"{data}/server.log", "-w", "start")

    def stop() -> None:
        _as_postgres(f"{BIN}/pg_ctl", "-D", data, "-m", "immediate", "-w", "stop")
        shutil.rmtree(data, ignore_errors=True)

    return f"postgres://postgres@127.0.0.1:{port}/postgres?sslmode=disable", stop


def _with_database(server: str, name: str, user: str | None = None, password: str | None = None) -> str:
    parts = urlsplit(server)
    netloc = parts.netloc
    if user is not None:
        host = netloc.split("@")[-1]
        netloc = f"{user}:{password}@{host}"
    return urlunsplit((parts.scheme, netloc, "/" + name, parts.query, ""))


def provision():
    """Return a Database and the function that removes it."""
    server = os.environ.get("TEST_DATABASE_URL", "").strip()
    stop_cluster = None
    if not server:
        server, stop_cluster = _start_cluster()

    name = "e2e_" + secrets.token_hex(6)
    password = secrets.token_hex(16)
    deadline = time.monotonic() + 30
    while True:
        try:
            with psycopg.connect(server, autocommit=True) as conn:
                conn.execute(f'CREATE DATABASE "{name}"')
                conn.execute(
                    f"DO $$ BEGIN IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '{APP_ROLE}') "
                    f"THEN CREATE ROLE {APP_ROLE} LOGIN; END IF; END $$")
                conn.execute(f"ALTER ROLE {APP_ROLE} PASSWORD '{password}'")
            break
        except psycopg.OperationalError:
            if time.monotonic() > deadline:
                raise
            time.sleep(0.5)

    database = Database(
        name=name,
        owner_url=_with_database(server, name),
        app_url=_with_database(server, name, APP_ROLE, password),
    )

    def remove() -> None:
        with psycopg.connect(server, autocommit=True) as conn:
            conn.execute(f'DROP DATABASE IF EXISTS "{name}" WITH (FORCE)')
        if stop_cluster is not None:
            stop_cluster()

    return database, remove
```

In `e2e/requirements.txt`, replace:

```text
# Everything else the suite needs comes from the standard library, so there is
# no third-party HTTP client to keep up to date for four requests.
playwright==1.56.0
pytest==9.1.1
```

with:

```text
# psycopg is how the suite creates, seeds and tidies its own PostgreSQL
# database (e2e/database.py). Everything else it needs comes from the standard
# library, so there is no third-party HTTP client to keep up to date for a few
# requests.
playwright==1.56.0
psycopg[binary]==3.2.10
pytest==9.1.1
```

Then install it where the suite runs: `make e2e-deps` (a session that started before this change has no psycopg).

- [ ] **Step 4: The fixtures**

In `e2e/conftest.py`, replace:

```python
import json
import os
import socket
```

with:

```python
import base64
import json
import os
import secrets
import socket
```

Append to `e2e/conftest.py`:

```python
# The marketplaces every test that needs a database runs against. The hosts
# are *.localhost, which Chromium resolves to the loopback address with no
# configuration (docs/superpowers/specs/2026-09-23-f13-identity-core-design.md).
SEED = json.dumps([
    {"slug": "m1", "name": "Loja Um", "market": "BR", "revenue_model": "commission",
     "state": "active", "default_language": "pt-BR", "languages": ["pt-BR", "en-US"],
     "hosts": ["m1.localhost"]},
    {"slug": "m2", "name": "Loja Dois", "market": "BR", "revenue_model": "commission",
     "state": "active", "default_language": "pt-BR", "languages": ["pt-BR", "en-US"],
     "hosts": ["m2.localhost"]},
])


# The key that wraps the audit log's per-person keys, for every process of this
# run. Without one, each process wraps with a key of its own that dies with it
# (cmd/marketplace, `protection`), and a process could not open a key the
# `migrate` before it, or another test's process, had created. Made fresh for
# each run and never written down.
AUDIT_LOCAL_KEY = base64.b64encode(secrets.token_bytes(32)).decode()


@pytest.fixture(scope="session")
def database(binary: Path):
    """A PostgreSQL, migrated and seeded once for the whole session.

    The module is imported here, not at the top, so that a run against a
    deployment never needs a PostgreSQL driver it would not use. pytest puts
    this directory on the import path, as it does for every conftest.
    """
    from database import APP_ROLE, provision

    db, remove = provision()
    migrate = subprocess.run(
        [str(binary), "migrate"], cwd=REPO_ROOT, capture_output=True, text=True, timeout=120,
        env={"PATH": os.environ.get("PATH", ""), "PROVIDERS_MODE": "fake",
             "DATABASE_URL": db.owner_url, "DATABASE_NAME": db.name,
             "DATABASE_APP_USER": APP_ROLE, "SEED_MARKETPLACES": SEED,
             "AUDIT_LOCAL_KEY": AUDIT_LOCAL_KEY},
    )
    assert migrate.returncode == 0, migrate.stdout + migrate.stderr
    yield db
    remove()


@dataclass
class Marketplace(Server):
    """A local process serving the seeded marketplaces."""

    port: int = 0
    mailbox: Path = Path()

    def url(self, host: str, path: str) -> str:
        return f"http://{host}:{self.port}{path}"


@pytest.fixture
def run_marketplace(run_server, database, tmp_path):
    """Start the binary against the suite's database, as the application role.

    The rate-limit counters are cleared first. Every test reaches the process
    from the same loopback address, and the database lives for the whole
    session, so without this the suite's own sign-ups would add up against the
    per-address limits and a test would fail for what the tests before it did.
    """
    import psycopg

    def factory(**env: str) -> Marketplace:
        with psycopg.connect(database.owner_url, autocommit=True) as conn:
            conn.execute("DELETE FROM rate_limit")
        mailbox = tmp_path / "mailbox"
        server = run_server(DATABASE_URL=database.app_url, MAIL_DIRECTORY=str(mailbox),
                            AUDIT_LOCAL_KEY=AUDIT_LOCAL_KEY, **env)
        port = int(server.base_url.rsplit(":", 1)[1])
        return Marketplace(base_url=server.base_url, process=server.process,
                           log_lines=server.log_lines, port=port, mailbox=mailbox)

    return factory
```

pytest puts the directory of a `conftest.py` on the import path (the default `prepend` import mode, and `e2e/` has no `__init__.py`), which is how `from database import …` and, later, `from accounts import …` resolve.

- [ ] **Step 5: Give the e2e job a PostgreSQL**

In `.github/workflows/ci.yml`, replace:

```yaml
  e2e:
    name: End-to-end tests
    runs-on: ubuntu-latest
    steps:
```

with:

```yaml
  e2e:
    name: End-to-end tests
    runs-on: ubuntu-latest

    # The identity flows write, so the suite needs the same PostgreSQL the
    # integration tests get (e2e/database.py).
    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: marketplace
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
```

In `.github/workflows/ci.yml`, replace:

```yaml
      - name: Run the suite
        run: make e2e
```

with:

```yaml
      - name: Run the suite
        run: make e2e
        env:
          # Given an address, the suite creates its database there instead of
          # starting a cluster of its own (e2e/database.py).
          TEST_DATABASE_URL: postgres://postgres:postgres@127.0.0.1:5432/marketplace?sslmode=disable
```

- [ ] **Step 6: Run the suite**

Run: `make e2e`
Expected: every existing test still passes; `test_database.py` passes (2 passed).

- [ ] **Step 7: Run the repository's checks**

Run: `make check && make test && make integration`
Expected: `0 issues`, all packages `ok`.

- [ ] **Step 8: Commit, push, open PR 1, subscribe**

```bash
git add e2e/ .github/workflows/ci.yml
git commit -m "Give the end-to-end suite a migrated, seeded database"
git push -u origin claude/funny-wright-379asb-f13e2e
```

---

# PR 2 — Accounts

## Task 3: Passwords — the rule, the hash and the concurrency bound

**Files:**
- Create: `internal/identity/policy.go`, `internal/identity/password.go`, `internal/identity/token.go`
- Test: `internal/identity/policy_test.go`, `internal/identity/password_test.go`, `internal/identity/token_test.go`

**Interfaces:**
- Produces:
  - `const MinPasswordLength = 12`, `MaxPasswordLength = 128`
  - `var ErrPasswordShort, ErrPasswordLong, ErrPasswordBreached, ErrEmailInvalid, ErrNameMissing error`
  - `func CheckPassword(password string) error`
  - `func NormaliseEmail(address string) (string, error)`
  - `type Params struct{ Memory, Time uint32; Threads uint8; KeyLen, SaltLen uint32 }`; `var Floor = Params{Memory: 19456, Time: 2, Threads: 1, KeyLen: 32, SaltLen: 16}`; `var Current = Floor` (Task 14 sets it from the benchmark)
  - `func NewHasher(params Params, concurrent int) *Hasher`
  - `func (h *Hasher) Hash(ctx context.Context, password string) (string, error)`
  - `func (h *Hasher) Verify(ctx context.Context, password, encoded string) (ok, stale bool, err error)` — compares with `crypto/subtle.ConstantTimeCompare`
  - `func (h *Hasher) Waste(ctx context.Context, password string)` — the dummy verification for unknown addresses
  - `func NewToken() (token string, hash []byte, err error)`, `func HashToken(token string) []byte`

`decode` bounds the salt and key lengths it reads back before converting them to the `uint32` argon2 takes, so the conversion cannot overflow (gosec G115) without a suppression.

- [ ] **Step 1: Write the failing tests**

Create `internal/identity/policy_test.go`:

```go
package identity

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckPasswordCountsCharactersNotBytes(t *testing.T) {
	cases := []struct {
		password string
		want     error
	}{
		{strings.Repeat("a", MinPasswordLength-1), ErrPasswordShort},
		{strings.Repeat("a", MinPasswordLength), nil},
		{strings.Repeat("ç", MinPasswordLength), nil}, // twice as many bytes, as many characters
		{strings.Repeat("a", MaxPasswordLength), nil},
		{strings.Repeat("a", MaxPasswordLength+1), ErrPasswordLong},
		{"café com leite na padaria", nil}, // spaces allowed, no composition rule
	}
	for _, c := range cases {
		if got := CheckPassword(c.password); !errors.Is(got, c.want) {
			t.Errorf("CheckPassword(%q) = %v, want %v", c.password, got, c.want)
		}
	}
}

func TestNormaliseEmail(t *testing.T) {
	good := []struct{ in, want string }{
		{"\tReader@Example.Test \n", "reader@example.test"}, // trimmed, then lower-cased
		{"a.b+tag@example.test", "a.b+tag@example.test"},
	}
	for _, c := range good {
		got, err := NormaliseEmail(c.in)
		if err != nil || got != c.want {
			t.Errorf("NormaliseEmail(%q) = %q, %v; want %q, nil", c.in, got, err, c.want)
		}
	}
	for _, bad := range []string{"", "no-at-sign", "Name <a@example.test>", "a@", "a b@example.test"} {
		if _, err := NormaliseEmail(bad); !errors.Is(err, ErrEmailInvalid) {
			t.Errorf("NormaliseEmail(%q) err = %v, want ErrEmailInvalid", bad, err)
		}
	}
}
```

Create `internal/identity/password_test.go`:

```go
package identity

import (
	"context"
	"strings"
	"testing"
)

// cheap keeps the unit tests fast; the real parameters are measured by the
// benchmark (cmd/marketplace, `bench-password`).
var cheap = Params{Memory: 64, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16}

func TestHashVerifiesAndRejects(t *testing.T) {
	h := NewHasher(cheap, 2)
	encoded, err := h.Hash(context.Background(), "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=64,t=1,p=1$") {
		t.Fatalf("not a PHC argon2id string: %q", encoded)
	}
	if ok, stale, err := h.Verify(context.Background(), "correct horse battery", encoded); !ok || stale || err != nil {
		t.Fatalf("Verify(right) = %v, %v, %v; want true, false, nil", ok, stale, err)
	}
	if ok, _, err := h.Verify(context.Background(), "wrong horse battery", encoded); ok || err != nil {
		t.Fatalf("Verify(wrong) = %v, %v; want false, nil", ok, err)
	}
}

func TestTwoHashesOfOnePasswordDiffer(t *testing.T) {
	h := NewHasher(cheap, 1)
	a, _ := h.Hash(context.Background(), "same password here")
	b, _ := h.Hash(context.Background(), "same password here")
	if a == b {
		t.Fatal("two hashes of one password are equal; the salt is not random")
	}
}

func TestAHashWithOtherParametersIsStale(t *testing.T) {
	old := NewHasher(cheap, 1)
	encoded, _ := old.Hash(context.Background(), "passphrase long enough")
	current := NewHasher(Params{Memory: 128, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16}, 1)
	ok, stale, err := current.Verify(context.Background(), "passphrase long enough", encoded)
	if !ok || !stale || err != nil {
		t.Fatalf("Verify = %v, %v, %v; want true, true, nil", ok, stale, err)
	}
}

func TestNormalisedFormsVerifyAlike(t *testing.T) {
	h := NewHasher(cheap, 1)
	composed := "senha com cedilha ç ok"    // one code point
	decomposed := "senha com cedilha ç ok" // c and a combining cedilla
	encoded, _ := h.Hash(context.Background(), composed)
	if ok, _, _ := h.Verify(context.Background(), decomposed, encoded); !ok {
		t.Fatal("the same password typed on another keyboard did not verify")
	}
}

func TestAMalformedHashIsAnErrorNotAMatch(t *testing.T) {
	h := NewHasher(cheap, 1)
	for _, encoded := range []string{
		"",
		"$argon2i$v=19$m=64,t=1,p=1$c2FsdHNhbHRzYWx0c2FsdA$a2V5",
		"$argon2id$v=19$m=64,t=1,p=1$c2FsdHNhbHRzYWx0c2FsdA$",                // no key
		"$argon2id$v=19$m=64,t=1,p=1$" + strings.Repeat("A", 2000) + "$a2V5", // salt beyond the bound
	} {
		if ok, _, err := h.Verify(context.Background(), "whatever", encoded); ok || err == nil {
			t.Errorf("Verify(%.40q) = %v, %v; want false and an error", encoded, ok, err)
		}
	}
}

func TestHashingWaitsForASlotAndHonoursTheContext(t *testing.T) {
	h := NewHasher(cheap, 1)
	h.slots <- struct{}{} // occupy the only slot
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := h.Hash(ctx, "any password at all"); err == nil {
		t.Fatal("Hash did not give up when its context was cancelled while waiting")
	}
}

// The deployed parameters may only ever be stronger than OWASP's floor
// (spec, D5), whatever the benchmark measured.
func TestCurrentIsNoWeakerThanTheFloor(t *testing.T) {
	if uint64(Current.Memory)*uint64(Current.Time) < uint64(Floor.Memory)*uint64(Floor.Time) ||
		Current.Threads < Floor.Threads || Current.KeyLen < Floor.KeyLen || Current.SaltLen < Floor.SaltLen {
		t.Fatalf("Current = %+v is weaker than Floor = %+v", Current, Floor)
	}
}
```

Create `internal/identity/token_test.go`:

```go
package identity

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

// A token is looked up by its SHA-256 and never compared itself: the database
// holds only the hash, so there is no secret-dependent comparison to time
// (spec, D6).
func TestTokensAreRandomAndStoredOnlyAsTheirHash(t *testing.T) {
	a, hashA, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _, _ := NewToken()
	if a == b {
		t.Fatal("two tokens are equal")
	}
	raw, err := base64.RawURLEncoding.DecodeString(a)
	if err != nil || len(raw) != 32 {
		t.Fatalf("token is %d bytes (%v), want 32", len(raw), err)
	}
	want := sha256.Sum256([]byte(a))
	if !bytes.Equal(hashA, want[:]) || !bytes.Equal(HashToken(a), hashA) {
		t.Fatal("the stored hash is not the SHA-256 of the token")
	}
}
```

- [ ] **Step 2: Run to see them fail**

Run: `go test ./internal/identity/ -v`
Expected: FAIL to compile (undefined symbols).

- [ ] **Step 3: Implement**

Create `internal/identity/policy.go`:

```go
// Package identity holds accounts, credentials and sessions (docs/design.md,
// section 2.1; docs/superpowers/specs/2026-09-23-f13-identity-core-design.md).
package identity

import (
	"errors"
	"net/mail"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// The password rule is a length and nothing else (spec, D4). NIST SP 800-63B
// says a verifier shall not demand digits, capitals or symbols, because such
// rules produce predictable passwords that guessing tools try first. The
// minimum is the owner's choice of 2026-09-23 and becomes a parameter in F17;
// the pages read both bounds from here, never from a literal of their own.
const (
	MinPasswordLength = 12
	MaxPasswordLength = 128
)

var (
	ErrPasswordShort    = errors.New("identity: password too short")
	ErrPasswordLong     = errors.New("identity: password too long")
	ErrPasswordBreached = errors.New("identity: password found in a breach")
	ErrEmailInvalid     = errors.New("identity: e-mail address invalid")
	ErrNameMissing      = errors.New("identity: name missing")
)

// normalise is what a password is before it is counted or hashed: NFKC, so
// the same password typed on two keyboards is the same password (NIST
// SP 800-63B, section 5.1.1.2).
func normalise(password string) string { return norm.NFKC.String(password) }

// CheckPassword reports whether a password may be set.
func CheckPassword(password string) error {
	n := utf8.RuneCountInString(normalise(password))
	switch {
	case n < MinPasswordLength:
		return ErrPasswordShort
	case n > MaxPasswordLength:
		return ErrPasswordLong
	}
	return nil
}

// NormaliseEmail returns the form an address is compared in, or
// ErrEmailInvalid. Only a bare address is accepted: "Name <a@b>" is valid mail
// syntax and not something a person types into an e-mail field.
func NormaliseEmail(address string) (string, error) {
	address = strings.TrimSpace(address)
	parsed, err := mail.ParseAddress(address)
	if err != nil || parsed.Address != address || parsed.Name != "" {
		return "", ErrEmailInvalid
	}
	return strings.ToLower(address), nil
}
```

Create `internal/identity/password.go`:

```go
package identity

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Params are argon2id's costs. Memory is in KiB.
type Params struct {
	Memory  uint32
	Time    uint32
	Threads uint8
	KeyLen  uint32
	SaltLen uint32
}

// Floor is OWASP's minimum for argon2id (19 MiB, two passes, one lane). It is
// what the service hashes with until the benchmark on its own CPU chooses
// stronger parameters (spec, D5), and what the benchmark falls back to.
var Floor = Params{Memory: 19456, Time: 2, Threads: 1, KeyLen: 32, SaltLen: 16}

// Current is what new hashes are made with. Until the benchmark has run on the
// service's CPU it is the floor; the measurement that replaces it is recorded
// here, beside the value (spec, D5).
var Current = Floor

// maxEncodedLen bounds the salt and the key read back from a stored hash. Ours
// are 16 and 32 bytes; anything past this is not a hash this service wrote.
const maxEncodedLen = 1024

var errNotPHC = errors.New("identity: not an argon2id PHC string")

// Hasher hashes and verifies passwords, a bounded number at a time.
//
// The bound is the instance's memory: 80 concurrent requests on 512 MiB cannot
// each hold tens of megabytes (spec, D5). A request beyond it waits, and gives
// up with its context. No caller holds a database transaction while it waits
// or hashes: a connection of a four-connection pool held for the length of a
// hash is a connection every other request waits for (internal/platform/db).
type Hasher struct {
	params Params
	slots  chan struct{}
	dummy  string
}

// NewHasher returns a hasher with params, allowing concurrent hashes at once.
func NewHasher(params Params, concurrent int) *Hasher {
	h := &Hasher{params: params, slots: make(chan struct{}, concurrent)}
	h.dummy = h.encode([]byte("identity: the dummy that equalises timing"))
	return h
}

func (h *Hasher) acquire(ctx context.Context) error {
	select {
	case h.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Hasher) release() { <-h.slots }

// Hash returns the PHC string for password.
func (h *Hasher) Hash(ctx context.Context, password string) (string, error) {
	if err := h.acquire(ctx); err != nil {
		return "", err
	}
	defer h.release()
	return h.encode([]byte(normalise(password))), nil
}

func (h *Hasher) encode(password []byte) string {
	salt := make([]byte, h.params.SaltLen)
	_, _ = rand.Read(salt)
	key := argon2.IDKey(password, salt, h.params.Time, h.params.Memory, h.params.Threads, h.params.KeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version,
		h.params.Memory, h.params.Time, h.params.Threads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

// Verify reports whether password matches encoded, and whether encoded was made
// with parameters other than this hasher's, so the caller can rehash it.
//
// The derived key is compared with crypto/subtle.ConstantTimeCompare, so how
// long the comparison takes says nothing about how much of it matched
// (spec, D6).
func (h *Hasher) Verify(ctx context.Context, password, encoded string) (ok, stale bool, err error) {
	params, salt, key, err := decode(encoded)
	if err != nil {
		return false, false, err
	}
	if err := h.acquire(ctx); err != nil {
		return false, false, err
	}
	defer h.release()
	got := argon2.IDKey([]byte(normalise(password)), salt, params.Time, params.Memory, params.Threads, params.KeyLen)
	return subtle.ConstantTimeCompare(got, key) == 1, params != h.params, nil
}

// Waste spends what a verification costs, so an unknown address takes as long
// as a wrong password (spec, D7).
func (h *Hasher) Waste(ctx context.Context, password string) {
	_, _, _ = h.Verify(ctx, password, h.dummy)
}

// decode reads a PHC string back into its parameters, salt and key.
func decode(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return Params{}, nil, nil, errNotPHC
	}
	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Time, &p.Threads); err != nil {
		return Params{}, nil, nil, errNotPHC
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return Params{}, nil, nil, errNotPHC
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return Params{}, nil, nil, errNotPHC
	}
	// Bounded before they are converted, so the lengths fit the uint32 that
	// argon2 takes.
	saltLen, keyLen := len(salt), len(key)
	if saltLen < 1 || saltLen > maxEncodedLen || keyLen < 1 || keyLen > maxEncodedLen {
		return Params{}, nil, nil, errNotPHC
	}
	p.SaltLen, p.KeyLen = uint32(saltLen), uint32(keyLen)
	return p, salt, key, nil
}
```

Create `internal/identity/token.go`:

```go
package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// NewToken returns a token to hand to a person once, and the hash that is the
// only thing stored (spec, D6). A token is found by the SHA-256 of 32 random
// bytes, so the database never compares the secret itself, and a lookup's
// timing could teach an attacker only about a hash whose preimage they would
// still need.
func NewToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, HashToken(token), nil
}

// HashToken is how a token is looked up.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
```

Run `go get golang.org/x/crypto@v0.57.0 && go mod tidy`, so the module, already required indirectly at that version, becomes a direct dependency.

- [ ] **Step 4: Run to see them pass**

Run: `go test ./internal/identity/ -race -v && make check`
Expected: PASS; `0 issues`.

- [ ] **Step 5: Commit**

```bash
git add internal/identity/ go.mod go.sum
git commit -m "Hash passwords with argon2id, bounded, and state the password rule"
```

## Task 4: The `breached` port

**Files:**
- Create: `internal/identity/breached/breached.go`
- Test: `internal/identity/breached/breached_test.go`

**Interfaces:**
- Produces: `type Checker interface { Breached(ctx context.Context, password string) (bool, error) }`; `func NewPwned(client *http.Client, base string) *Pwned`; `const RangeAPI = "https://api.pwnedpasswords.com"`; `type Fake struct{ Known map[string]bool; Err error }`; `var Common = map[string]bool{"password1234": true, "123456789012": true, "senhasenha123": true}`.

The address constant is `RangeAPI`: gosec's hard-coded-credential rule (G101) reads a name containing `pw` as a password.

- [ ] **Step 1: Write the failing test**

Create `internal/identity/breached/breached_test.go`:

```go
package breached

import (
	"context"
	"crypto/sha1" // #nosec G505 -- the range API's protocol is SHA-1
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPwnedSendsOnlyThePrefixAndFindsTheSuffix(t *testing.T) {
	sum := sha1.Sum([]byte("password1234")) // #nosec G401 -- see above
	full := strings.ToUpper(hex.EncodeToString(sum[:]))

	var asked, padding string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked, padding = r.URL.Path, r.Header.Get("Add-Padding")
		_, _ = w.Write([]byte("0000000000000000000000000000000000A:0\r\n" + full[5:] + ":12\r\n"))
	}))
	defer server.Close()

	found, err := NewPwned(server.Client(), server.URL).Breached(context.Background(), "password1234")
	if err != nil || !found {
		t.Fatalf("Breached = %v, %v; want true, nil", found, err)
	}
	if asked != "/range/"+full[:5] {
		t.Fatalf("asked %q, want only the five-character prefix", asked)
	}
	if padding != "true" {
		t.Fatal("the request did not ask for padding")
	}
}

func TestAPaddedZeroCountIsNotABreach(t *testing.T) {
	sum := sha1.Sum([]byte("an unusual passphrase")) // #nosec G401 -- see above
	full := strings.ToUpper(hex.EncodeToString(sum[:]))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(full[5:] + ":0\r\n"))
	}))
	defer server.Close()
	if found, _ := NewPwned(server.Client(), server.URL).Breached(context.Background(), "an unusual passphrase"); found {
		t.Fatal("a padding line with count 0 was taken as a breach")
	}
}

func TestTheFakeAnswersFromItsList(t *testing.T) {
	fake := Fake{Known: Common}
	if found, err := fake.Breached(context.Background(), "password1234"); !found || err != nil {
		t.Fatalf("Breached(listed) = %v, %v; want true, nil", found, err)
	}
	if found, _ := fake.Breached(context.Background(), "a passphrase nobody has used"); found {
		t.Fatal("the fake called an unlisted password breached")
	}
}

func TestAnUnreachableServiceIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	if _, err := NewPwned(server.Client(), server.URL).Breached(context.Background(), "whatever"); err == nil {
		t.Fatal("a 503 was not reported as an error")
	}
}
```

- [ ] **Step 2: Run to see it fail** — `go test ./internal/identity/breached/ -v` → FAIL (undefined).

- [ ] **Step 3: Implement**

Create `internal/identity/breached/breached.go`:

```go
// Package breached answers whether a password has appeared in a public breach
// (spec, D4). It is a port with two adapters: Pwned Passwords, and a fake.
package breached

import (
	"bufio"
	"context"
	"crypto/sha1" // #nosec G505 -- SHA-1 is the range API's protocol, not a password hash
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

// Checker is the port.
type Checker interface {
	Breached(ctx context.Context, password string) (bool, error)
}

// RangeAPI is the address of the Pwned Passwords range API.
const RangeAPI = "https://api.pwnedpasswords.com"

// Pwned asks the range API with the first five hexadecimal characters of the
// password's SHA-1 and compares the rest locally (k-anonymity). Padding hides
// even the number of matches from anyone watching the response size.
type Pwned struct {
	client *http.Client
	base   string
}

// NewPwned returns the adapter. client carries the timeout.
func NewPwned(client *http.Client, base string) *Pwned { return &Pwned{client: client, base: base} }

// Breached implements Checker.
func (p *Pwned) Breached(ctx context.Context, password string) (bool, error) {
	sum := sha1.Sum([]byte(password)) // #nosec G401 -- see the import
	full := strings.ToUpper(hex.EncodeToString(sum[:]))
	prefix, suffix := full[:5], full[5:]

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.base+"/range/"+prefix, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Add-Padding", "true")
	req.Header.Set("User-Agent", "aleogr-marketplace")

	resp, err := p.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("breached: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("breached: the range API answered %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		found, count, ok := strings.Cut(line, ":")
		if ok && found == suffix && count != "0" {
			return true, nil
		}
	}
	return false, scanner.Err()
}

// Common are the passwords the fake calls breached, so the suite can prove a
// refusal without reaching the internet.
var Common = map[string]bool{"password1234": true, "123456789012": true, "senhasenha123": true}

// Fake is the adapter tests and a local run use. Known is what it calls
// breached; Err, when set, is what it answers instead, which is how a test
// makes the service unreachable.
type Fake struct {
	Known map[string]bool
	Err   error
}

// Breached implements Checker.
func (f Fake) Breached(_ context.Context, password string) (bool, error) {
	return f.Known[password], f.Err
}
```

- [ ] **Step 4: Run to see it pass** — `go test ./internal/identity/breached/ -race -v` → PASS. Then `make check` (gosec accepts the annotated SHA-1 import and use).

- [ ] **Step 5: Commit** — `git add internal/identity/breached && git commit -m "Check new passwords against Pwned Passwords, behind a port"`

## Task 5: The accounts schema

**Files:**
- Create: `migrations/00009_accounts.sql`
- Test: `internal/identity/store_integration_test.go` (build tag `integration`)

- [ ] **Step 1: Write the migration**

Create `migrations/00009_accounts.sql`:

```sql
-- Accounts, their passwords and the confirmation of their e-mail addresses
-- (docs/superpowers/specs/2026-09-23-f13-identity-core-design.md).
--
-- `account` and not `user`: `user` is a reserved word in PostgreSQL and would
-- need quoting in every query.
--
-- An account belongs to a marketplace (docs/requirements.md, section 4); a
-- staff account belongs to the platform and has no marketplace, which makes it
-- invisible to the application role under the policies below — staff access
-- arrives in F15 by its own path.
--
-- A correction to migrations/00008_audit_log.sql, which calls a person "the
-- platform's, not a marketplace's": that predates this table. `user_key.user_id`
-- holds an `account.id`, unique across the platform, so nothing about the keys
-- changes.

-- +goose Up
CREATE TABLE account (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    marketplace_id   uuid REFERENCES marketplace (id) ON DELETE RESTRICT,
    kind             text        NOT NULL,
    email            text        NOT NULL,
    email_normalised text        NOT NULL,
    name             text        NOT NULL,
    verified_at      timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT account_kind_is_known CHECK (kind IN ('buyer', 'staff')),
    CONSTRAINT account_staff_has_no_marketplace CHECK ((kind = 'staff') = (marketplace_id IS NULL)),
    -- NULLS NOT DISTINCT, so two staff accounts cannot share an address either.
    CONSTRAINT account_email_is_unique UNIQUE NULLS NOT DISTINCT (marketplace_id, email_normalised)
);

CREATE TABLE credential (
    account_id     uuid NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    marketplace_id uuid REFERENCES marketplace (id) ON DELETE RESTRICT,
    kind           text        NOT NULL,
    -- The PHC string: the parameters travel with the hash (spec, D5).
    secret         text        NOT NULL,
    updated_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (account_id, kind),
    CONSTRAINT credential_kind_is_known CHECK (kind IN ('password'))
);

CREATE TABLE email_verification (
    -- SHA-256 of the token; the token itself is only ever in the e-mail.
    token_hash     bytea PRIMARY KEY,
    account_id     uuid NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    marketplace_id uuid REFERENCES marketplace (id) ON DELETE RESTRICT,
    expires_at     timestamptz NOT NULL,
    used_at        timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX email_verification_by_account ON email_verification (account_id);

ALTER TABLE account ENABLE ROW LEVEL SECURITY;
ALTER TABLE credential ENABLE ROW LEVEL SECURITY;
ALTER TABLE email_verification ENABLE ROW LEVEL SECURITY;

CREATE POLICY account_belongs_to_the_marketplace ON account
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());
CREATE POLICY credential_belongs_to_the_marketplace ON credential
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());
CREATE POLICY email_verification_belongs_to_the_marketplace ON email_verification
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());

-- +goose Down
DROP TABLE IF EXISTS email_verification;
DROP TABLE IF EXISTS credential;
DROP TABLE IF EXISTS account;
```

- [ ] **Step 2: Write the isolation test and its fixture** (it fails until Task 6 adds the store; write it now and run it after Step 3 of Task 6)

The tests of a package share one database (`dbtest.Run` makes one per package), so `twoMarketplaces` gives each test two marketplaces of its own, with unique slugs and hosts, and takes their identifiers from what `tenancy.Seed` returns.

Create `internal/identity/store_integration_test.go`:

```go
//go:build integration

package identity

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/platform/config"
	"github.com/aleogr/marketplace/internal/platform/db"
	"github.com/aleogr/marketplace/internal/platform/dbtest"
	"github.com/aleogr/marketplace/internal/tenancy"
)

func TestMain(m *testing.M) { os.Exit(dbtest.Run(m)) }

// serving is a Transactor that runs as the application role, the way the
// deployed service does, so row-level security is what is tested.
type serving struct {
	t    *testing.T
	pool *db.Pool
}

func (s serving) InTxFor(_ context.Context, marketplaceID string, fn func(pgx.Tx) error) error {
	return dbtest.Serving(s.t, s.pool, marketplaceID, fn)
}

// unique returns a slug no other test has used. The tests of this package
// share one database (internal/platform/dbtest), so a fixture with a fixed
// name would find the accounts, mails and sessions an earlier test left.
func unique(t *testing.T, name string) string {
	t.Helper()
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	return name + "-" + hex.EncodeToString(raw)
}

// twoMarketplaces seeds two marketplaces of this test's own and returns their
// identifiers, in the database the package migrated.
func twoMarketplaces(t *testing.T) (serving, string, string) {
	t.Helper()
	pool, err := db.Open(t.Context(), config.Database{URL: dbtest.URL(t)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	dbtest.AsApplication(t, pool)

	one, two := unique(t, "one"), unique(t, "two")
	specs := []tenancy.Spec{
		{Slug: one, Name: "One", Market: "BR", RevenueModel: "commission", State: "active",
			DefaultLanguage: "pt-BR", Languages: []string{"pt-BR"}, Hosts: []string{one + ".test"}},
		{Slug: two, Name: "Two", Market: "BR", RevenueModel: "commission", State: "active",
			DefaultLanguage: "pt-BR", Languages: []string{"pt-BR"}, Hosts: []string{two + ".test"}},
	}
	var applied []tenancy.Applied
	if err := pool.InTx(t.Context(), func(tx pgx.Tx) error {
		var err error
		applied, err = tenancy.Seed(t.Context(), tx, specs)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return serving{t: t, pool: pool}, applied[0].ID, applied[1].ID
}

func TestAnAccountIsInvisibleFromAnotherMarketplace(t *testing.T) {
	db, one, two := twoMarketplaces(t)
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		_, err := insertAccount(t.Context(), tx, one, "Reader@Example.Test", "reader@example.test", "Reader")
		return err
	}); err != nil {
		t.Fatal(err)
	}

	var lookup error
	if err := db.InTxFor(t.Context(), two, func(tx pgx.Tx) error {
		_, lookup = accountByEmail(t.Context(), tx, "reader@example.test")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(lookup, pgx.ErrNoRows) {
		t.Fatalf("looking the account up from the other marketplace = %v, want pgx.ErrNoRows", lookup)
	}

	// And the same address may open an account in the other marketplace.
	if err := db.InTxFor(t.Context(), two, func(tx pgx.Tx) error {
		_, err := insertAccount(t.Context(), tx, two, "reader@example.test", "reader@example.test", "Reader")
		return err
	}); err != nil {
		t.Fatalf("the same address could not open an account in a second marketplace: %v", err)
	}
}
```

- [ ] **Step 3: Commit** — together with Task 6 (the test needs the store).

## Task 6: The store and the sign-up, verification and resend flows

**Files:**
- Create: `internal/identity/store.go`, `internal/identity/identity.go`, `web/mail/verify-email.{en-US,pt-BR}.{txt,html}`, `web/mail/account-exists.{en-US,pt-BR}.{txt,html}`
- Modify: `internal/platform/mail/templates_test.go`
- Test: `internal/identity/identity_integration_test.go` (tag `integration`)

**Interfaces:**
- Consumes: Task 3 (`Hasher`, `CheckPassword`, `NormaliseEmail`, `NewToken`, `HashToken`), Task 4 (`breached.Checker`), `mail.Request(ctx, tx, mail.Message)`, `(*audit.Log).Append(ctx, tx, audit.Entry)`.
- Produces:
  - `type Transactor interface { InTxFor(ctx context.Context, marketplaceID string, fn func(pgx.Tx) error) error }`
  - `type Auditor interface { Append(ctx context.Context, tx pgx.Tx, entry audit.Entry) error }`
  - `type Visit struct { Marketplace, MarketplaceName, Language, IP, UserAgent, BaseURL string }` — `BaseURL` is scheme and host (with its port, if any), no trailing slash, no language
  - `func NewService(db Transactor, hasher *Hasher, checker breached.Checker, auditor Auditor, log *slog.Logger) *Service`
  - `func (s *Service) SignUp(ctx context.Context, v Visit, name, email, password string) error` — returns `ErrNameMissing`, `ErrEmailInvalid`, `ErrPasswordShort`, `ErrPasswordLong`, `ErrPasswordBreached`, or nil whether the address was new or not
  - `func (s *Service) Verify(ctx context.Context, v Visit, token string) error` — `ErrTokenInvalid` for unknown, used or expired
  - `func (s *Service) Resend(ctx context.Context, v Visit, email string) error` — nil unless the database fails
  - `var ErrTokenInvalid error`
  - store (unexported): `insertAccount`, `accountByEmail`, `setPassword`, `insertVerification`, `consumeVerification`; `type Account struct { ID, MarketplaceID, Email, Name string; VerifiedAt *time.Time }`. `accountByID` and `passwordOf` arrive in Task 9, where they are first used (an unused function fails `make check`).
  - mail templates: `verify-email` (variables `Name`, `Link`), `account-exists` (variables `Name`, `SignIn`); both show the marketplace's name as `{{.From}}`

The test double records `audit.Entry` values itself: no alias of `audit.Entry` exists in the package only to serve a test.

- [ ] **Step 1: Write the failing integration tests**

Create `internal/identity/identity_integration_test.go`:

```go
//go:build integration

package identity

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/identity/breached"
	"github.com/aleogr/marketplace/internal/platform/audit"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// recording is an Auditor that remembers the entries it was given.
type recording struct{ entries []audit.Entry }

func (r *recording) Append(_ context.Context, _ pgx.Tx, e audit.Entry) error {
	r.entries = append(r.entries, e)
	return nil
}

// actions are the recorded actions, in order.
func (r *recording) actions() []string {
	var out []string
	for _, e := range r.entries {
		out = append(out, e.Action)
	}
	return out
}

func service(t *testing.T) (*Service, serving, string, string, *recording) {
	db, one, two := twoMarketplaces(t)
	trail := &recording{}
	s := NewService(db, NewHasher(cheap, 2), breached.Fake{Known: breached.Common}, trail, quiet)
	return s, db, one, two, trail
}

func visit(marketplace string) Visit {
	return Visit{Marketplace: marketplace, MarketplaceName: "One", Language: "pt-BR",
		IP: "203.0.113.7", UserAgent: "test", BaseURL: "https://one.test"}
}

// sent is one message requested for delivery.
type sent struct{ template, link string }

// outbox returns the messages requested for delivery in a marketplace, oldest
// first.
func outbox(t *testing.T, db serving, marketplace string) []sent {
	t.Helper()
	var got []sent
	if err := db.InTxFor(t.Context(), marketplace, func(tx pgx.Tx) error {
		rows, err := tx.Query(t.Context(),
			`SELECT payload->>'Template', coalesce(payload->'Variables'->>'Link', '')
			   FROM outbox_event WHERE kind = 'email.send' ORDER BY created_at, id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var one sent
			if err := rows.Scan(&one.template, &one.link); err != nil {
				return err
			}
			got = append(got, one)
		}
		return rows.Err()
	}); err != nil {
		t.Fatalf("cannot read the outbox: %v", err)
	}
	return got
}

// tokenOf is the token a verification link carries.
func tokenOf(t *testing.T, link string) string {
	t.Helper()
	_, token, found := strings.Cut(link, "token=")
	if !found {
		t.Fatalf("%q carries no token", link)
	}
	return token
}

func TestSignUpAndVerify(t *testing.T) {
	s, db, one, _, trail := service(t)
	if err := s.SignUp(t.Context(), visit(one), "Reader", "Reader@Example.Test", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	mails := outbox(t, db, one)
	if len(mails) != 1 || mails[0].template != "verify-email" {
		t.Fatalf("outbox = %v, want one verify-email", mails)
	}
	if !strings.HasPrefix(mails[0].link, "https://one.test/pt-BR/verify?token=") {
		t.Fatalf("link %q is not this marketplace's verification page", mails[0].link)
	}
	token := tokenOf(t, mails[0].link)

	if err := s.Verify(t.Context(), visit(one), token); err != nil {
		t.Fatalf("Verify = %v", err)
	}
	if err := s.Verify(t.Context(), visit(one), token); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("second Verify = %v, want ErrTokenInvalid: a token is single-use", err)
	}
	if got := trail.actions(); !slices.Equal(got, []string{"identity.signup", "identity.email_verified"}) {
		t.Fatalf("audited %v", got)
	}
}

func TestSignUpWithATakenAddressSendsAccountExistsAndSaysNothing(t *testing.T) {
	s, db, one, _, _ := service(t)
	if err := s.SignUp(t.Context(), visit(one), "Reader", "reader@example.test", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	if err := s.SignUp(t.Context(), visit(one), "Someone", "READER@example.test", "another long passphrase"); err != nil {
		t.Fatalf("second SignUp = %v, want nil: the answer must not reveal the account", err)
	}
	mails := outbox(t, db, one)
	if len(mails) != 2 || mails[0].template != "verify-email" || mails[1].template != "account-exists" {
		t.Fatalf("outbox = %v, want verify-email then account-exists", mails)
	}
}

func TestSignUpRefusesABreachedOrShortPassword(t *testing.T) {
	s, db, one, _, _ := service(t)
	if err := s.SignUp(t.Context(), visit(one), "R", "a@example.test", "password1234"); !errors.Is(err, ErrPasswordBreached) {
		t.Fatalf("breached: %v", err)
	}
	if err := s.SignUp(t.Context(), visit(one), "R", "a@example.test", "short"); !errors.Is(err, ErrPasswordShort) {
		t.Fatalf("short: %v", err)
	}
	if mails := outbox(t, db, one); len(mails) != 0 {
		t.Fatalf("a refused sign-up sent %v", mails)
	}
}

func TestAnUnreachableBreachServiceDoesNotBlockSignUp(t *testing.T) {
	db, one, _ := twoMarketplaces(t)
	s := NewService(db, NewHasher(cheap, 1), breached.Fake{Err: errors.New("down")}, &recording{}, quiet)
	if err := s.SignUp(t.Context(), visit(one), "R", "r@example.test", "correct horse battery staple"); err != nil {
		t.Fatalf("SignUp = %v, want nil (fail open, spec D4)", err)
	}
}

func TestAnExpiredTokenIsRefused(t *testing.T) {
	s, db, one, _, _ := service(t)
	if err := s.SignUp(t.Context(), visit(one), "R", "r@example.test", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	token := tokenOf(t, outbox(t, db, one)[0].link)
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(), "UPDATE email_verification SET expires_at = now() - interval '1 second'")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Verify(t.Context(), visit(one), token); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("Verify(expired) = %v, want ErrTokenInvalid", err)
	}
}
```

- [ ] **Step 2: Run to see them fail**

Run: `go test -tags=integration ./internal/identity/ -run 'SignUp|Verify|Expired|Invisible' -v`
Expected: FAIL to compile.

- [ ] **Step 3: Implement the store**

Create `internal/identity/store.go`:

```go
package identity

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Account is what the flows need to know about one.
type Account struct {
	ID            string
	MarketplaceID string
	Email         string
	Name          string
	VerifiedAt    *time.Time
}

var errTaken = errors.New("identity: address taken")

func insertAccount(ctx context.Context, tx pgx.Tx, marketplace, email, normalised, name string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO account (marketplace_id, kind, email, email_normalised, name)
		VALUES ($1, 'buyer', $2, $3, $4)
		ON CONFLICT ON CONSTRAINT account_email_is_unique DO NOTHING
		RETURNING id::text`, marketplace, email, normalised, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errTaken
	}
	return id, err
}

const accountColumns = `id::text, marketplace_id::text, email, name, verified_at`

func scanAccount(row pgx.Row) (Account, error) {
	var a Account
	err := row.Scan(&a.ID, &a.MarketplaceID, &a.Email, &a.Name, &a.VerifiedAt)
	return a, err
}

// accountByEmail finds an account of the marketplace the transaction names;
// row-level security is what scopes it.
func accountByEmail(ctx context.Context, tx pgx.Tx, normalised string) (Account, error) {
	return scanAccount(tx.QueryRow(ctx,
		`SELECT `+accountColumns+` FROM account WHERE email_normalised = $1`, normalised))
}

func setPassword(ctx context.Context, tx pgx.Tx, marketplace, account, secret string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO credential (account_id, marketplace_id, kind, secret)
		VALUES ($1, $2, 'password', $3)
		ON CONFLICT (account_id, kind) DO UPDATE SET secret = EXCLUDED.secret, updated_at = now()`,
		account, marketplace, secret)
	return err
}

func insertVerification(ctx context.Context, tx pgx.Tx, marketplace, account string, hash []byte, expires time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO email_verification (token_hash, account_id, marketplace_id, expires_at)
		VALUES ($1, $2, $3, $4)`, hash, account, marketplace, expires)
	return err
}

// consumeVerification spends a token and confirms its account, or reports
// pgx.ErrNoRows for a token that is unknown, spent or expired.
func consumeVerification(ctx context.Context, tx pgx.Tx, hash []byte, now time.Time) (string, error) {
	var account string
	if err := tx.QueryRow(ctx, `
		UPDATE email_verification SET used_at = $2
		 WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2
		RETURNING account_id::text`, hash, now).Scan(&account); err != nil {
		return "", err
	}
	_, err := tx.Exec(ctx,
		`UPDATE account SET verified_at = coalesce(verified_at, $2) WHERE id = $1`, account, now)
	return account, err
}
```

- [ ] **Step 4: Implement the service**

SignUp hashes before its transaction opens, and hashes for a taken address too, so both answers cost the same (D7) and no connection is held while argon2 runs.

Create `internal/identity/identity.go`:

```go
package identity

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/identity/breached"
	"github.com/aleogr/marketplace/internal/platform/audit"
	"github.com/aleogr/marketplace/internal/platform/mail"
)

// VerificationLifetime is how long a confirmation link works (spec).
const VerificationLifetime = 24 * time.Hour

// ErrTokenInvalid is a token that is unknown, spent or expired. One error for
// all three: which one it was is nobody's business but the log's.
var ErrTokenInvalid = errors.New("identity: token invalid")

// Transactor opens a transaction for one marketplace (db.Pool.InTxFor).
type Transactor interface {
	InTxFor(ctx context.Context, marketplaceID string, fn func(pgx.Tx) error) error
}

// Auditor appends to the audit log ((*audit.Log).Append).
type Auditor interface {
	Append(ctx context.Context, tx pgx.Tx, entry audit.Entry) error
}

// Visit is the request a flow runs for, as the flow needs it.
type Visit struct {
	Marketplace     string // id
	MarketplaceName string
	Language        string
	IP              string
	UserAgent       string
	BaseURL         string // scheme and host, e.g. https://m1.example
}

func (v Visit) link(path string, query url.Values) string {
	u := v.BaseURL + "/" + v.Language + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return u
}

// Service runs the identity flows.
//
// No flow hashes or verifies a password inside a database transaction: a
// flow reads in one transaction, runs argon2 outside any, and writes in a
// second one (spec, D5; internal/platform/db).
type Service struct {
	db       Transactor
	hasher   *Hasher
	breached breached.Checker
	audit    Auditor
	log      *slog.Logger
	now      func() time.Time
}

// NewService returns the flows.
func NewService(db Transactor, hasher *Hasher, checker breached.Checker, auditor Auditor, log *slog.Logger) *Service {
	return &Service{db: db, hasher: hasher, breached: checker, audit: auditor, log: log,
		now: func() time.Time { return time.Now().UTC() }}
}

// checkNew applies D4 to a password about to be set.
func (s *Service) checkNew(ctx context.Context, password string) error {
	if err := CheckPassword(password); err != nil {
		return err
	}
	found, err := s.breached.Breached(ctx, normalise(password))
	if err != nil {
		// Fail open (spec, D4): a third party being down must not stop sign-up.
		s.log.WarnContext(ctx, "the breached-password check could not run", "error", err)
		return nil
	}
	if found {
		return ErrPasswordBreached
	}
	return nil
}

// record audits what an account did itself.
func (s *Service) record(ctx context.Context, tx pgx.Tx, v Visit, account, action string) error {
	after, err := json.Marshal(map[string]string{"user_agent": v.UserAgent})
	if err != nil {
		return err
	}
	return s.audit.Append(ctx, tx, audit.Entry{
		Marketplace: v.Marketplace,
		Actor:       audit.Actor{ID: account, Kind: audit.Buyer},
		Action:      action,
		Subject:     audit.Subject{Kind: "account", ID: account, Person: account},
		From:        v.IP,
		After:       after,
	})
}

// SignUp creates an unconfirmed account and sends the confirmation, or, for an
// address that already has an account, tells its owner so. The caller answers
// the same page either way (spec, D7). The password is hashed before the
// transaction opens, and for a taken address too, so both answers cost the
// same.
func (s *Service) SignUp(ctx context.Context, v Visit, name, email, password string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrNameMissing
	}
	normalised, err := NormaliseEmail(email)
	if err != nil {
		return err
	}
	if err := s.checkNew(ctx, password); err != nil {
		return err
	}
	secret, err := s.hasher.Hash(ctx, password)
	if err != nil {
		return err
	}
	token, hash, err := NewToken()
	if err != nil {
		return err
	}

	return s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		id, err := insertAccount(ctx, tx, v.Marketplace, strings.TrimSpace(email), normalised, name)
		if errors.Is(err, errTaken) {
			existing, err := accountByEmail(ctx, tx, normalised)
			if err != nil {
				return err
			}
			return mail.Request(ctx, tx, mail.Message{
				Template: "account-exists", Language: v.Language, To: existing.Email,
				From: v.MarketplaceName, Marketplace: v.Marketplace,
				Variables: map[string]string{"Name": existing.Name, "SignIn": v.link("/signin", nil)},
			})
		}
		if err != nil {
			return err
		}
		if err := setPassword(ctx, tx, v.Marketplace, id, secret); err != nil {
			return err
		}
		if err := insertVerification(ctx, tx, v.Marketplace, id, hash, s.now().Add(VerificationLifetime)); err != nil {
			return err
		}
		if err := mail.Request(ctx, tx, mail.Message{
			Template: "verify-email", Language: v.Language, To: strings.TrimSpace(email),
			From: v.MarketplaceName, Marketplace: v.Marketplace,
			Variables: map[string]string{"Name": name, "Link": v.link("/verify", url.Values{"token": {token}})},
		}); err != nil {
			return err
		}
		return s.record(ctx, tx, v, id, "identity.signup")
	})
}

// Verify spends a confirmation token.
func (s *Service) Verify(ctx context.Context, v Visit, token string) error {
	return s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		account, err := consumeVerification(ctx, tx, HashToken(token), s.now())
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrTokenInvalid
		}
		if err != nil {
			return err
		}
		return s.record(ctx, tx, v, account, "identity.email_verified")
	})
}

// Resend issues a new confirmation to an unconfirmed address, and does nothing
// otherwise. The caller answers the same page either way (spec, D7).
func (s *Service) Resend(ctx context.Context, v Visit, email string) error {
	normalised, err := NormaliseEmail(email)
	if err != nil {
		return nil
	}
	token, hash, err := NewToken()
	if err != nil {
		return err
	}
	return s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		account, err := accountByEmail(ctx, tx, normalised)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && account.VerifiedAt != nil) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := insertVerification(ctx, tx, v.Marketplace, account.ID, hash, s.now().Add(VerificationLifetime)); err != nil {
			return err
		}
		return mail.Request(ctx, tx, mail.Message{
			Template: "verify-email", Language: v.Language, To: account.Email,
			From: v.MarketplaceName, Marketplace: v.Marketplace,
			Variables: map[string]string{"Name": account.Name, "Link": v.link("/verify", url.Values{"token": {token}})},
		})
	})
}
```

- [ ] **Step 5: The mail templates** — the first line of each `.txt` is `Subject: …`. `Render` passes one flat map: `From` (the marketplace's name, which `Message.From` carries), `Language`, and each variable.

Create `web/mail/verify-email.en-US.txt`:

```text
Subject: Confirm your e-mail address at {{.From}}
Hello, {{.Name}}.

Confirm your e-mail address to finish creating your account at {{.From}}:

{{.Link}}

The link works once and expires in 24 hours. If you did not create an account, ignore this message.
```

Create `web/mail/verify-email.pt-BR.txt`:

```text
Subject: Confirme seu e-mail em {{.From}}
Olá, {{.Name}}.

Confirme seu e-mail para terminar de criar sua conta em {{.From}}:

{{.Link}}

O link funciona uma vez e expira em 24 horas. Se você não criou uma conta, ignore esta mensagem.
```

Create `web/mail/verify-email.en-US.html`:

```html
<p>Hello, {{.Name}}.</p>
<p>Confirm your e-mail address to finish creating your account at {{.From}}:</p>
<p><a href="{{.Link}}">{{.Link}}</a></p>
<p>The link works once and expires in 24 hours. If you did not create an account, ignore this message.</p>
```

Create `web/mail/verify-email.pt-BR.html`:

```html
<p>Olá, {{.Name}}.</p>
<p>Confirme seu e-mail para terminar de criar sua conta em {{.From}}:</p>
<p><a href="{{.Link}}">{{.Link}}</a></p>
<p>O link funciona uma vez e expira em 24 horas. Se você não criou uma conta, ignore esta mensagem.</p>
```

Create `web/mail/account-exists.en-US.txt`:

```text
Subject: You already have an account at {{.From}}
Hello, {{.Name}}.

Someone tried to create an account at {{.From}} with this address, which already has one. To sign in:

{{.SignIn}}

If it was not you, you can ignore this message: nothing was changed.
```

Create `web/mail/account-exists.pt-BR.txt`:

```text
Subject: Você já tem uma conta em {{.From}}
Olá, {{.Name}}.

Alguém tentou criar uma conta em {{.From}} com este e-mail, que já tem uma. Para entrar:

{{.SignIn}}

Se não foi você, pode ignorar esta mensagem: nada foi alterado.
```

Create `web/mail/account-exists.en-US.html`:

```html
<p>Hello, {{.Name}}.</p>
<p>Someone tried to create an account at {{.From}} with this address, which already has one. To sign in:</p>
<p><a href="{{.SignIn}}">{{.SignIn}}</a></p>
<p>If it was not you, you can ignore this message: nothing was changed.</p>
```

Create `web/mail/account-exists.pt-BR.html`:

```html
<p>Olá, {{.Name}}.</p>
<p>Alguém tentou criar uma conta em {{.From}} com este e-mail, que já tem uma. Para entrar:</p>
<p><a href="{{.SignIn}}">{{.SignIn}}</a></p>
<p>Se não foi você, pode ignorar esta mensagem: nada foi alterado.</p>
```

The parity check renders every template in every language, and the templates are parsed with `missingkey=error`, so it must pass the variables they use:

In `internal/platform/mail/templates_test.go`, replace:

```go
// The definition of done: every user-facing text exists in both languages
```

with:

```go
// sampleVariables fills every variable any template asks for. The templates
// are parsed with missingkey=error, so a template that asks for a variable
// missing here fails this test: add the variable here when a template gains
// one.
var sampleVariables = map[string]string{
	"Name":   "Reader",
	"Link":   "https://marketplace1.example/en-US/verify?token=sample",
	"SignIn": "https://marketplace1.example/en-US/signin",
}

// The definition of done: every user-facing text exists in both languages
```

In `internal/platform/mail/templates_test.go`, replace:

```go
				Template: name, Language: language, To: "reader@example.test",
				From: "Marketplace 1",
			}, i18n.Default)
			if err != nil {
				t.Fatalf("the template %q does not render in %s: %v", name, language, err)
```

with:

```go
				Template: name, Language: language, To: "reader@example.test",
				From: "Marketplace 1", Variables: sampleVariables,
			}, i18n.Default)
			if err != nil {
				t.Fatalf("the template %q does not render in %s: %v", name, language, err)
```

- [ ] **Step 6: Run to see them pass**

Run: `go test -tags=integration -count=1 ./internal/identity/ -race -v && go test ./internal/platform/mail/ -v && make check`
Expected: PASS (the mail package's parity test renders the new pairs); `0 issues`.

- [ ] **Step 7: Commit** — `git add internal/identity internal/platform/mail migrations/00009_accounts.sql web/mail && git commit -m "Create accounts, confirm their addresses, and never say which exist"`

## Task 7: Each rate limiter forgets only its own windows

**Files:**
- Modify: `internal/platform/ratelimit/database.go`, `internal/platform/ratelimit/database_integration_test.go`

**Interfaces:**
- Changes: `func NewDatabase(db Execer, name string, limit int, window time.Duration) *Database` — `name` is lower-case letters, digits and hyphens; every row the limiter writes is `name:key`, and its sweep deletes only `name:%` rows older than its own window. A bad name panics at start-up.

Why: today the sweep is `DELETE FROM rate_limit WHERE window_start < start - window`, over every limiter's rows. The identity limits have windows of ten minutes, fifteen minutes and an hour, so a ten-minute limiter opening a window erases the live rows of the hourly ones and hands them a fresh allowance: an hourly limit that resets every ten minutes. Measured in the end-to-end suite before this change: the hourly sign-up counter held 5 hits after 13 sign-ups. Naming the limiters needs no schema change; every existing caller is a test.

- [ ] **Step 1: Write the failing test** — replace the test file; the four existing tests only gain a limiter name, and their keys lose the prefix the name now supplies:

Replace the contents of `internal/platform/ratelimit/database_integration_test.go` with:

```go
//go:build integration

package ratelimit_test

import (
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/aleogr/marketplace/internal/platform/config"
	"github.com/aleogr/marketplace/internal/platform/db"
	"github.com/aleogr/marketplace/internal/platform/dbtest"
	"github.com/aleogr/marketplace/internal/platform/ratelimit"
)

func TestMain(m *testing.M) {
	os.Exit(dbtest.Run(m))
}

// migrated returns a pool whose database carries the schema.
func migrated(t *testing.T) *db.Pool {
	t.Helper()

	settings := config.Database{URL: dbtest.URL(t)}
	pool, err := db.Open(t.Context(), settings)
	if err != nil {
		t.Fatalf("db.Open() = %v, want a pool", err)
	}
	t.Cleanup(pool.Close)

	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := db.Migrate(t.Context(), pool, settings, quiet); err != nil {
		t.Fatalf("db.Migrate() = %v, want nil", err)
	}
	return pool
}

// TestTheLimitHoldsAcrossInstances is why this limiter exists at all.
//
// Cloud Run runs several instances, and each holds its own memory. Two
// limiters here stand for two instances: if the counter lived in the process,
// the pair would allow the limit twice, which is how "five attempts a minute"
// becomes ten, or fifty, depending on how busy the service is
// (docs/roadmap.md, F7).
func TestTheLimitHoldsAcrossInstances(t *testing.T) {
	pool := migrated(t)

	const limit = 5
	// One fixed clock for both, so the run cannot straddle a window boundary
	// and pass for the wrong reason.
	now := time.Now()
	clock := func() time.Time { return now }

	first := ratelimit.NewDatabase(pool, "sign-in", limit, time.Minute)
	second := ratelimit.NewDatabase(pool, "sign-in", limit, time.Minute)
	ratelimit.SetDatabaseClock(first, clock)
	ratelimit.SetDatabaseClock(second, clock)

	instances := []*ratelimit.Database{first, second}
	allowed := 0
	for i := range limit * 2 {
		// Alternating, as a client hitting a service behind a load balancer
		// would be spread across the instances serving it.
		decision, err := instances[i%2].Allow(t.Context(), "203.0.113.7")
		if err != nil {
			t.Fatalf("Allow() = %v", err)
		}
		if decision.Allowed {
			allowed++
		}
	}

	if allowed != limit {
		t.Errorf("two instances allowed %d attempts, want %d: the limit is per client, not per instance",
			allowed, limit)
	}
}

func TestARefusalSaysWhenToComeBack(t *testing.T) {
	pool := migrated(t)

	limiter := ratelimit.NewDatabase(pool, "recovery", 1, time.Minute)
	now := time.Now()
	ratelimit.SetDatabaseClock(limiter, func() time.Time { return now })

	if decision, _ := limiter.Allow(t.Context(), "a"); !decision.Allowed {
		t.Fatal("the first attempt was refused")
	}

	decision, err := limiter.Allow(t.Context(), "a")
	if err != nil {
		t.Fatalf("Allow() = %v", err)
	}
	if decision.Allowed {
		t.Fatal("the second attempt was allowed")
	}
	if decision.RetryAfter <= 0 || decision.RetryAfter > 2*time.Minute {
		t.Errorf("RetryAfter = %v, want something inside the window", decision.RetryAfter)
	}
}

// TestTheNextWindowStartsClean: a limit that never forgets is a ban.
func TestTheNextWindowStartsClean(t *testing.T) {
	pool := migrated(t)

	limiter := ratelimit.NewDatabase(pool, "sign-in", 1, time.Minute)
	now := time.Now().Truncate(time.Minute)
	ratelimit.SetDatabaseClock(limiter, func() time.Time { return now })

	_, _ = limiter.Allow(t.Context(), "b")
	if decision, _ := limiter.Allow(t.Context(), "b"); decision.Allowed {
		t.Fatal("the second attempt in the window was allowed")
	}

	now = now.Add(time.Minute)
	if decision, _ := limiter.Allow(t.Context(), "b"); !decision.Allowed {
		t.Error("the first attempt of the next window was refused")
	}
}

// TestOneClientDoesNotSpendAnothersAllowance, in the database this time.
func TestSubjectsAreCountedApart(t *testing.T) {
	pool := migrated(t)

	limiter := ratelimit.NewDatabase(pool, "sign-in", 1, time.Minute)
	now := time.Now()
	ratelimit.SetDatabaseClock(limiter, func() time.Time { return now })

	_, _ = limiter.Allow(t.Context(), "203.0.113.7")
	if decision, _ := limiter.Allow(t.Context(), "198.51.100.23"); !decision.Allowed {
		t.Error("one address spent another's allowance")
	}
}

// TestALimiterForgetsOnlyItsOwnWindows is the regression test of F13: the
// sweep used to forget every row older than its own window, so a limiter
// with a ten-minute window, opening one, erased the live counters of an
// hourly limiter and handed it a fresh allowance.
func TestALimiterForgetsOnlyItsOwnWindows(t *testing.T) {
	pool := migrated(t)

	hour := time.Now().UTC().Truncate(time.Hour)
	now := hour.Add(5 * time.Minute)
	clock := func() time.Time { return now }

	hourly := ratelimit.NewDatabase(pool, "hourly", 1, time.Hour)
	short := ratelimit.NewDatabase(pool, "short", 1, 10*time.Minute)
	ratelimit.SetDatabaseClock(hourly, clock)
	ratelimit.SetDatabaseClock(short, clock)

	if decision, err := hourly.Allow(t.Context(), "203.0.113.7"); err != nil || !decision.Allowed {
		t.Fatalf("the first hourly attempt = %+v, %v; want allowed", decision, err)
	}

	// Twenty-five minutes on, the short limiter opens a window. Its sweep
	// forgets rows older than ten minutes before that window, and the hourly
	// row, from 5 past, is one of them by age but not by ownership.
	now = hour.Add(25 * time.Minute)
	if decision, err := short.Allow(t.Context(), "203.0.113.7"); err != nil || !decision.Allowed {
		t.Fatalf("the short limiter's attempt = %+v, %v; want allowed", decision, err)
	}

	now = hour.Add(30 * time.Minute)
	decision, err := hourly.Allow(t.Context(), "203.0.113.7")
	if err != nil {
		t.Fatalf("Allow() = %v", err)
	}
	if decision.Allowed {
		t.Fatal("the hourly limit was reset by another limiter's sweep")
	}
}

// TestALimiterStillForgetsItsOwnPastWindows: the sweep still does its job.
func TestALimiterStillForgetsItsOwnPastWindows(t *testing.T) {
	pool := migrated(t)

	now := time.Now().UTC().Truncate(time.Minute)
	limiter := ratelimit.NewDatabase(pool, "sweep", 1, time.Minute)
	ratelimit.SetDatabaseClock(limiter, func() time.Time { return now })

	_, _ = limiter.Allow(t.Context(), "a")
	now = now.Add(3 * time.Minute)
	_, _ = limiter.Allow(t.Context(), "a")

	var left int
	if err := pool.QueryRow(t.Context(),
		"SELECT count(*) FROM rate_limit WHERE subject = 'sweep:a'").Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 1 {
		t.Fatalf("%d rows of this limiter are left, want only the open window's", left)
	}
}
```

- [ ] **Step 2: Run it to see it fail** — `go test -tags=integration ./internal/platform/ratelimit/ -v` → FAIL to compile (`NewDatabase` takes no name). With the name added and the old sweep kept, `TestALimiterForgetsOnlyItsOwnWindows` fails with "the hourly limit was reset by another limiter's sweep".

- [ ] **Step 3: Implement**

Replace the contents of `internal/platform/ratelimit/database.go` with:

```go
package ratelimit

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Execer is the part of the connection pool this limiter uses.
type Execer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Database counts attempts in PostgreSQL, so that the limit is the limit
// however many instances are serving.
//
// It is a fixed window rather than a token bucket: one statement, one row, no
// read-then-write race between instances. The edge of a window lets through
// somewhat more than the limit over a short span, which is the cost, and for
// "five sign-in attempts a minute" it is a cost worth paying to keep the
// counter a single atomic upsert.
//
// Every limiter has a name, and its rows are its own. The table is shared by
// limits with windows of minutes and of hours, and a sweep that forgot every
// row older than its own window would erase the live counters of a longer
// one: an hourly limit reset every ten minutes by its neighbour is not an
// hourly limit.
type Database struct {
	db     Execer
	name   string
	limit  int
	window time.Duration

	now func() time.Time
}

// validName is what a limiter may be called: it is the prefix of every row it
// writes and of the pattern its sweep matches, so it holds nothing a LIKE
// pattern would read as a wildcard.
var validName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// NewDatabase returns a limiter called name, allowing limit attempts per window
// for a key. One name is one limit: two limiters sharing a name must share the
// window too, which is what two instances of the same limit do.
//
// A name that is not lower-case letters, digits and hyphens is a programming
// error, and it panics at start-up rather than miscounting later.
func NewDatabase(db Execer, name string, limit int, window time.Duration) *Database {
	if !validName.MatchString(name) {
		panic(fmt.Sprintf("ratelimit: %q is not a limiter name", name))
	}
	return &Database{db: db, name: name, limit: limit, window: window, now: time.Now}
}

// Allow counts one attempt for key and reports whether it is within the limit.
func (d *Database) Allow(ctx context.Context, key string) (Decision, error) {
	now := d.now().UTC()
	// The window a moment belongs to, so that every instance agrees on which
	// row to write without any of them coordinating.
	start := now.Truncate(d.window)

	var hits int
	err := d.db.QueryRow(ctx, `
		INSERT INTO rate_limit (subject, window_start, hits)
		VALUES ($1, $2, 1)
		ON CONFLICT (subject, window_start)
		DO UPDATE SET hits = rate_limit.hits + 1
		RETURNING hits
	`, d.name+":"+key, start).Scan(&hits)
	if err != nil {
		return Decision{}, fmt.Errorf("cannot count the attempt: %w", err)
	}

	// The windows of this limiter that have passed are forgotten by whoever
	// opens a new one, and only this limiter's: another limiter's rows may
	// belong to a longer window that is still open. There is no scheduler in
	// this phase and a table of expired counters would otherwise grow forever;
	// hanging the sweep on the first hit of a window makes it rare without
	// needing a coin toss to make it rare.
	if hits == 1 {
		if _, err := d.db.Exec(ctx,
			"DELETE FROM rate_limit WHERE subject LIKE $1 AND window_start < $2",
			d.name+":%", start.Add(-d.window)); err != nil {
			return Decision{}, fmt.Errorf("cannot forget the windows that have passed: %w", err)
		}
	}

	if hits > d.limit {
		return Decision{RetryAfter: start.Add(d.window).Sub(now).Round(time.Second) + time.Second}, nil
	}
	return allowed, nil
}
```

- [ ] **Step 4: Run to see it pass** — `go test -tags=integration -count=1 ./internal/platform/ratelimit/ -race -v && make check` → PASS; `0 issues`.

- [ ] **Step 5: Commit** — `git add internal/platform/ratelimit && git commit -m "Let each rate limiter forget only its own windows"`

## Task 8: The sign-up and verification pages, wired into the process

**Files:**
- Create: `internal/platform/httpx/identity.go`, `web/identity.templ` (then `make generate`), `e2e/test_signup.py`
- Modify: `internal/platform/httpx/render.go`, `internal/platform/httpx/site.go` (field, routes), `web/page.go` (`Form`), `cmd/marketplace/main.go` (one audit log; the service), `web/locales/*.json`, `docs/design.md` §2.2
- Test: `internal/platform/httpx/identity_test.go` (package `httpx_test`, like every httpx test)

**Interfaces:**
- Consumes: Task 6 `Service`, `Visit`, errors; Task 7 `ratelimit.NewDatabase(execer, name, limit, window)`; `ratelimit.Limit(limiter, subject, pages.Refused, log)`.
- Produces:
  - `type IdentityRoutes struct { Service *identity.Service; Limits IdentityLimits; Pages Pages; Log *slog.Logger }`
  - `type IdentityLimits struct { SignUp, Resend, ResendAddress, SignIn, SignInAddress, Password ratelimit.Limiter }` (the last three used in PR 3 and 4)
  - `func (s Site) WithIdentity(routes IdentityRoutes) Site`
  - unexported: `visit(r) identity.Visit`, `marketplaceOf(r)`, `byIP`, `byAddress`, `newForm()`, `formError(err) (key string, args []any, shown bool)`, `renderStatus(w, r, status, component)`
  - templ: `SignUp(page Page, form Form)`, `CheckEmail(page Page)`, `Verify(page Page, token string, failed bool)`, `Resend(page Page, sent bool)`; `type Form struct { Name, Email, Error string; ErrorArgs []any; MinLength, MaxLength int }` in `web/page.go`
  - test helpers (package `httpx_test`): `silent()`, `cheap`, `identityHandler(t, service, limits)`, `identitySite(t)`, `inMarketplace(r)`, `formRequest(t, path, form)`, `postForm(t, path, form)`

`Layout` already holds the page's `<main>` (`web/layout.templ`), so the identity pages are its content and carry no `<main>` of their own. The password's bounds reach the page through `Form` (`newForm()` fills them from `identity`) and the messages through `%[1]d`.

- [ ] **Step 1: Locale keys**

Add to `web/locales/en-US.json` and `web/locales/pt-BR.json` (the same keys in both, at the end of each file):

| key | en-US | pt-BR |
|---|---|---|
| `identity.signup.title` | Create your account | Crie sua conta |
| `identity.signup.submit` | Create account | Criar conta |
| `identity.field.name` | Name | Nome |
| `identity.field.email` | E-mail | E-mail |
| `identity.field.password` | Password | Senha |
| `identity.field.password_hint` | At least %[1]d characters. A phrase is easier to remember than symbols. | No mínimo %[1]d caracteres. Uma frase é mais fácil de lembrar que símbolos. |
| `identity.error.name_missing` | Tell us your name. | Informe seu nome. |
| `identity.error.email_invalid` | This does not look like an e-mail address. | Isto não parece um endereço de e-mail. |
| `identity.error.password_short` | The password needs at least %[1]d characters. | A senha precisa de pelo menos %[1]d caracteres. |
| `identity.error.password_long` | The password can have at most %[1]d characters. | A senha pode ter no máximo %[1]d caracteres. |
| `identity.error.password_breached` | This password has appeared in a data breach elsewhere. Choose another. | Esta senha já apareceu em um vazamento de dados em outro lugar. Escolha outra. |
| `identity.check_email.title` | Check your e-mail | Confira seu e-mail |
| `identity.check_email.body` | We sent a message to the address you gave. Follow it to continue. | Enviamos uma mensagem para o endereço informado. Siga-a para continuar. |
| `identity.verify.title` | Confirm your e-mail | Confirme seu e-mail |
| `identity.verify.submit` | Confirm | Confirmar |
| `identity.verify.failed` | This link is invalid, already used or expired. | Este link é inválido, já foi usado ou expirou. |
| `identity.verify.done` | Your e-mail is confirmed. You can sign in. | Seu e-mail foi confirmado. Você já pode entrar. |
| `identity.resend.title` | Send the confirmation again | Reenviar a confirmação |
| `identity.resend.submit` | Send again | Reenviar |
| `identity.resend.sent` | If this address has an account waiting for confirmation, a new link is on its way. | Se este endereço tiver uma conta aguardando confirmação, um novo link está a caminho. |
| `identity.link.signin` | Sign in | Entrar |
| `identity.link.signup` | Create account | Criar conta |
| `identity.link.resend` | Did not receive it? | Não recebeu? |

- [ ] **Step 2: Write the failing handler tests**

Create `internal/platform/httpx/identity_test.go`:

```go
package httpx_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/identity/breached"
	"github.com/aleogr/marketplace/internal/platform/httpx"
	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/internal/platform/ratelimit"
	"github.com/aleogr/marketplace/internal/platform/seo"
	"github.com/aleogr/marketplace/internal/tenancy"
)

// silent is a logger for tests that do not read the log.
func silent() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// cheap keeps the hashing in these tests fast.
var cheap = identity.Params{Memory: 64, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16}

// identityHandler returns the routes with the identity pages mounted over
// service, limited by limits.
func identityHandler(t *testing.T, service *identity.Service, limits httpx.IdentityLimits) http.Handler {
	t.Helper()

	catalogue, err := i18n.Load()
	if err != nil {
		t.Fatalf("i18n.Load() = %v", err)
	}
	preview, err := seo.NewPreview()
	if err != nil {
		t.Fatalf("seo.NewPreview() = %v", err)
	}
	return httpx.NewSite(nil, catalogue, preview, false).WithIdentity(httpx.IdentityRoutes{
		Service: service,
		Limits:  limits,
		Pages:   httpx.NewPages(catalogue),
		Log:     silent(),
	}).Handler()
}

// identitySite is identityHandler over a service with no database, and no
// limits. The tests that use it stop before the database: what they prove is
// the part of each page that needs none.
func identitySite(t *testing.T) http.Handler {
	t.Helper()
	never := ratelimit.Never{}
	return identityHandler(t,
		identity.NewService(nil, identity.NewHasher(cheap, 1), breached.Fake{}, nil, silent()),
		httpx.IdentityLimits{
			SignUp: never, Resend: never, ResendAddress: never,
			SignIn: never, SignInAddress: never, Password: never,
		})
}

// inMarketplace is a request as it reaches the routes: its host resolved to a
// marketplace and its language to pt-BR.
func inMarketplace(r *http.Request) *http.Request {
	ctx := tenancy.WithResolution(r.Context(), tenancy.Resolution{
		Kind: tenancy.MarketplaceHost,
		Marketplace: &tenancy.Marketplace{
			ID: "00000000-0000-0000-0000-000000000001", Name: "Loja Um",
			State: tenancy.Active, DefaultLanguage: "pt-BR", Languages: []string{"pt-BR", "en-US"},
		},
	})
	return r.WithContext(i18n.WithLanguage(ctx, "pt-BR"))
}

// formRequest is a form posted to path in the marketplace.
func formRequest(t *testing.T, path string, form url.Values) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return inMarketplace(request)
}

// postForm sends a form to the identity routes and returns the response.
func postForm(t *testing.T, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	identitySite(t).ServeHTTP(recorder, formRequest(t, path, form))
	return recorder
}

// The validation that needs no database answers first, in the visitor's
// language, as an HTML page with the status that says the form was refused.
func TestSignUpShowsTheFieldErrorBeforeTouchingTheDatabase(t *testing.T) {
	recorder := postForm(t, "/signup",
		url.Values{"name": {""}, "email": {"a@example.test"}, "password": {"correct horse battery"}})

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want the page's", got)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "Informe seu nome.") {
		t.Errorf("the page does not say what is missing: %s", body)
	}
}

// The minimum a visitor reads is the minimum the rule applies: both come from
// identity.MinPasswordLength.
func TestTheShortPasswordMessageNamesTheRulesMinimum(t *testing.T) {
	recorder := postForm(t, "/signup",
		url.Values{"name": {"Leitora"}, "email": {"a@example.test"}, "password": {"curta"}})

	want := "A senha precisa de pelo menos 12 caracteres."
	if recorder.Code != http.StatusUnprocessableEntity || !strings.Contains(recorder.Body.String(), want) {
		t.Fatalf("status %d, body without %q: %s", recorder.Code, want, recorder.Body.String())
	}
}

func TestTheSignUpFieldCarriesTheRulesBounds(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/signup", nil)
	recorder := httptest.NewRecorder()
	identitySite(t).ServeHTTP(recorder, inMarketplace(request))

	body := recorder.Body.String()
	for _, want := range []string{`minlength="12"`, `maxlength="128"`, "No mínimo 12 caracteres."} {
		if !strings.Contains(body, want) {
			t.Errorf("the sign-up page does not carry %s", want)
		}
	}
}
```

- [ ] **Step 3: Run to see them fail** — `go test ./internal/platform/httpx/ -run 'SignUp' -v` → FAIL to compile.

- [ ] **Step 4: Implement the handlers**

A status other than 200 is written through `renderStatus`, which sets the type before the status (a header set after `WriteHeader` is dropped):

Replace the contents of `internal/platform/httpx/render.go` with:

```go
package httpx

import (
	"net/http"

	"github.com/a-h/templ"
)

// render writes a component as the response.
//
// A template that fails halfway has already written part of a page, so there is
// no status left to change and nothing useful to say to the visitor; what there
// is to do is not pretend it succeeded, which is why the error is dropped here
// and the failure shows up as a truncated response and in the access log rather
// than as a second set of headers Go would refuse to write anyway.
func render(w http.ResponseWriter, r *http.Request, component templ.Component) {
	renderStatus(w, r, http.StatusOK, component)
}

// renderStatus is render with a status other than 200: a form shown again
// with what was wrong with it. The type is set before the status is written,
// because a header set after WriteHeader is silently dropped.
func renderStatus(w http.ResponseWriter, r *http.Request, status int, component templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = component.Render(r.Context(), w)
}
```

Create `internal/platform/httpx/identity.go`:

```go
package httpx

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/platform/i18n"
	"github.com/aleogr/marketplace/internal/platform/ratelimit"
	"github.com/aleogr/marketplace/internal/tenancy"
	"github.com/aleogr/marketplace/web"
)

// IdentityLimits are the per-route limits of the identity flows, in the
// database so they hold across instances (docs/roadmap.md, F13). Each limiter
// is named, and namespaces its own counters (internal/platform/ratelimit), so
// the subjects below are just the client or the address.
type IdentityLimits struct {
	SignUp, Resend, ResendAddress, SignIn, SignInAddress, Password ratelimit.Limiter
}

// IdentityRoutes is what the identity pages need.
type IdentityRoutes struct {
	Service *identity.Service
	Limits  IdentityLimits
	Pages   Pages
	Log     *slog.Logger
}

// WithIdentity returns the site serving the identity pages too.
func (s Site) WithIdentity(routes IdentityRoutes) Site {
	s.identity = &routes
	return s
}

// byIP limits by the client's address.
func byIP(r *http.Request) string {
	origin, _ := OriginFrom(r.Context())
	return origin.IP
}

// byAddress limits by the e-mail address a form names, in this marketplace.
func byAddress(r *http.Request) string {
	normalised, _ := identity.NormaliseEmail(r.PostFormValue("email"))
	return marketplaceOf(r) + ":" + normalised
}

func (s Site) identityRoutes(mux *http.ServeMux) {
	if s.identity == nil {
		return
	}
	id := s.identity
	limit := func(l ratelimit.Limiter, subject ratelimit.Subject, h http.Handler) http.Handler {
		return ratelimit.Limit(l, subject, id.Pages.Refused, id.Log)(h)
	}

	mux.HandleFunc("GET /signup", s.signUpForm)
	mux.Handle("POST /signup", limit(id.Limits.SignUp, byIP, http.HandlerFunc(s.signUp)))
	mux.HandleFunc("GET /verify", s.verifyForm)
	mux.HandleFunc("POST /verify", s.verify)
	mux.HandleFunc("GET /verify/resend", s.resendForm)
	mux.Handle("POST /verify/resend", limit(id.Limits.Resend, byIP,
		limit(id.Limits.ResendAddress, byAddress, http.HandlerFunc(s.resend))))
}

func marketplaceOf(r *http.Request) string {
	if resolution, ok := tenancy.FromContext(r.Context()); ok && resolution.Marketplace != nil {
		return resolution.Marketplace.ID
	}
	return ""
}

// visit is the request, as the identity flows need it.
func visit(r *http.Request) identity.Visit {
	v := identity.Visit{Language: i18n.FromContext(r.Context()), UserAgent: r.UserAgent(), BaseURL: origin(r)}
	if o, ok := OriginFrom(r.Context()); ok {
		v.IP = o.IP
	}
	if resolution, ok := tenancy.FromContext(r.Context()); ok && resolution.Marketplace != nil {
		v.Marketplace, v.MarketplaceName = resolution.Marketplace.ID, resolution.Marketplace.Name
	}
	return v
}

// newForm is where every form page starts: the password bounds come from the
// rule itself, so the page, its hint and the rule cannot disagree (spec, D4).
func newForm() web.Form {
	return web.Form{MinLength: identity.MinPasswordLength, MaxLength: identity.MaxPasswordLength}
}

// formError names the message a flow's error is shown as, with its
// arguments. An error it does not name is not the visitor's to see.
func formError(err error) (key string, args []any, shown bool) {
	switch {
	case errors.Is(err, identity.ErrNameMissing):
		return "identity.error.name_missing", nil, true
	case errors.Is(err, identity.ErrEmailInvalid):
		return "identity.error.email_invalid", nil, true
	case errors.Is(err, identity.ErrPasswordShort):
		return "identity.error.password_short", []any{identity.MinPasswordLength}, true
	case errors.Is(err, identity.ErrPasswordLong):
		return "identity.error.password_long", []any{identity.MaxPasswordLength}, true
	case errors.Is(err, identity.ErrPasswordBreached):
		return "identity.error.password_breached", nil, true
	}
	return "", nil, false
}

func (s Site) signUpForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, web.SignUp(s.page(r, "/signup"), newForm()))
}

func (s Site) signUp(w http.ResponseWriter, r *http.Request) {
	form := newForm()
	form.Name, form.Email = r.PostFormValue("name"), r.PostFormValue("email")
	err := s.identity.Service.SignUp(r.Context(), visit(r), form.Name, form.Email, r.PostFormValue("password"))
	if key, args, shown := formError(err); shown {
		form.Error, form.ErrorArgs = key, args
		renderStatus(w, r, http.StatusUnprocessableEntity, web.SignUp(s.page(r, "/signup"), form))
		return
	}
	if err != nil {
		s.identity.Log.ErrorContext(r.Context(), "sign-up failed", "error", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	render(w, r, web.CheckEmail(s.page(r, "/signup")))
}

func (s Site) verifyForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, web.Verify(s.page(r, "/verify"), r.URL.Query().Get("token"), false))
}

func (s Site) verify(w http.ResponseWriter, r *http.Request) {
	err := s.identity.Service.Verify(r.Context(), visit(r), r.PostFormValue("token"))
	if errors.Is(err, identity.ErrTokenInvalid) {
		renderStatus(w, r, http.StatusGone, web.Verify(s.page(r, "/verify"), "", true))
		return
	}
	if err != nil {
		s.identity.Log.ErrorContext(r.Context(), "verification failed", "error", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+"/signin?verified=1", http.StatusSeeOther)
}

func (s Site) resendForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, web.Resend(s.page(r, "/verify/resend"), false))
}

func (s Site) resend(w http.ResponseWriter, r *http.Request) {
	if err := s.identity.Service.Resend(r.Context(), visit(r), r.PostFormValue("email")); err != nil {
		s.identity.Log.ErrorContext(r.Context(), "resend failed", "error", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	render(w, r, web.Resend(s.page(r, "/verify/resend"), true))
}
```

In `internal/platform/httpx/site.go`, replace:

```go
	indexable bool
}
```

with:

```go
	indexable bool
	// identity serves sign-up, sign-in and the account's pages. It is nil in
	// a process with no database, which has no accounts to serve.
	identity *IdentityRoutes
}
```

In `internal/platform/httpx/site.go`, replace:

```go
	mux.HandleFunc("GET /{$}", s.home)
```

with:

```go
	s.identityRoutes(mux)
	mux.HandleFunc("GET /{$}", s.home)
```

Append to `web/page.go`:

```go
// Form is what a form page shows back: the fields a visitor typed, except the
// password; the key of the message explaining what went wrong, with its
// arguments; and the password's bounds, which come from internal/identity.
type Form struct {
	Name      string
	Email     string
	Error     string
	ErrorArgs []any
	MinLength int
	MaxLength int
}
```

- [ ] **Step 5: The pages** — then `make generate`.

Create `web/identity.templ`:

```templ
package web

import "strconv"

// The identity pages. Layout already holds the page's <main>, so each of these
// is that main's content. Every visible string goes through a key, and the
// password's bounds come from the form, which takes them from the rule
// (internal/identity), never from a number written here.
templ SignUp(page Page, form Form) {
	@Layout(page, page.T("identity.signup.title")) {
		<h1>{ page.T("identity.signup.title") }</h1>
		if form.Error != "" {
			<p role="alert">{ page.T(form.Error, form.ErrorArgs...) }</p>
		}
		<form method="post" action={ templ.SafeURL("/" + page.Language + "/signup") }>
			<input type="hidden" name="csrf_token" value={ page.CSRFToken }/>
			<label>
				{ page.T("identity.field.name") }
				<input name="name" autocomplete="name" required value={ form.Name }/>
			</label>
			<label>
				{ page.T("identity.field.email") }
				<input name="email" type="email" autocomplete="email" required value={ form.Email }/>
			</label>
			<label>
				{ page.T("identity.field.password") }
				<input
					name="password"
					type="password"
					autocomplete="new-password"
					required
					minlength={ strconv.Itoa(form.MinLength) }
					maxlength={ strconv.Itoa(form.MaxLength) }
				/>
			</label>
			<p>{ page.T("identity.field.password_hint", form.MinLength) }</p>
			<button type="submit">{ page.T("identity.signup.submit") }</button>
		</form>
		<p><a href={ templ.SafeURL("/" + page.Language + "/signin") }>{ page.T("identity.link.signin") }</a></p>
	}
}

templ CheckEmail(page Page) {
	@Layout(page, page.T("identity.check_email.title")) {
		<h1>{ page.T("identity.check_email.title") }</h1>
		<p>{ page.T("identity.check_email.body") }</p>
		<p><a href={ templ.SafeURL("/" + page.Language + "/verify/resend") }>{ page.T("identity.link.resend") }</a></p>
	}
}

templ Verify(page Page, token string, failed bool) {
	@Layout(page, page.T("identity.verify.title")) {
		<h1>{ page.T("identity.verify.title") }</h1>
		if failed {
			<p role="alert">{ page.T("identity.verify.failed") }</p>
			<p><a href={ templ.SafeURL("/" + page.Language + "/verify/resend") }>{ page.T("identity.resend.title") }</a></p>
		} else {
			<form method="post" action={ templ.SafeURL("/" + page.Language + "/verify") }>
				<input type="hidden" name="csrf_token" value={ page.CSRFToken }/>
				<input type="hidden" name="token" value={ token }/>
				<button type="submit">{ page.T("identity.verify.submit") }</button>
			</form>
		}
	}
}

templ Resend(page Page, sent bool) {
	@Layout(page, page.T("identity.resend.title")) {
		<h1>{ page.T("identity.resend.title") }</h1>
		if sent {
			<p role="status">{ page.T("identity.resend.sent") }</p>
		} else {
			<form method="post" action={ templ.SafeURL("/" + page.Language + "/verify/resend") }>
				<input type="hidden" name="csrf_token" value={ page.CSRFToken }/>
				<label>
					{ page.T("identity.field.email") }
					<input name="email" type="email" autocomplete="email" required/>
				</label>
				<button type="submit">{ page.T("identity.resend.submit") }</button>
			</form>
		}
	}
}
```

- [ ] **Step 6: Wire the process** — one audit log for the process, built once and handed to the work and to the flows; the identity service with named limiters.

In `cmd/marketplace/main.go`, replace:

```go
	log.InfoContext(ctx, "audit keys", "keeper", keeper.Name())
```

with:

```go
	log.InfoContext(ctx, "audit keys", "keeper", keeper.Name())

	// One audit log for the whole process: the work below and the identity
	// flows append to the same chains.
	trail := audit.NewLog(audit.NewKeys(keeper), log)
```

In `cmd/marketplace/main.go`, replace:

```go
		tasks, closeTasks, err := work(ctx, cfg, database, mailer,
			audit.NewLog(audit.NewKeys(keeper), log), log)
```

with:

```go
		tasks, closeTasks, err := work(ctx, cfg, database, mailer, trail, log)
```

In `cmd/marketplace/main.go`, replace:

```go
	handler := site.Handler()
```

with:

```go
	// Accounts, sign-up and, later, sessions. They need the database, so a
	// process running without one serves no identity pages at all.
	var identityService *identity.Service
	if database != nil {
		var checker breached.Checker = breached.Fake{Known: breached.Common}
		if cfg.ProvidersMode == config.ProvidersReal {
			checker = breached.NewPwned(&http.Client{Timeout: 3 * time.Second}, breached.RangeAPI)
		}
		identityService = identity.NewService(database,
			identity.NewHasher(identity.Current, hashSlots(identity.Current)), checker, trail, log)
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
			},
		})
	}

	handler := site.Handler()
```

In `cmd/marketplace/main.go`, replace:

```go
// protection returns the keeper that wraps every person's key, and the
```

with:

```go
// hashSlots is how many passwords are hashed at once: as many as fit in a
// quarter of the instance's 512 MiB at the memory one hash takes, and never
// fewer than two (spec, D5). The rest of the instance is the process itself
// and the requests it is serving.
func hashSlots(params identity.Params) int {
	return max(2, 128*1024/int(params.Memory))
}

// protection returns the keeper that wraps every person's key, and the
```

In `cmd/marketplace/main.go`, replace:

```go
	"log/slog"
	"net"
```

with:

```go
	"log/slog"
	"net"
	"net/http"
```

In `cmd/marketplace/main.go`, replace:

```go
	"github.com/aleogr/marketplace/internal/platform/audit"
```

with:

```go
	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/identity/breached"
	"github.com/aleogr/marketplace/internal/platform/audit"
```

In `docs/design.md` §2.2, add `breached` to the list of ports: "`breached` (whether a password appears in a public breach; Pwned Passwords, failing open)".

- [ ] **Step 7: The end-to-end test** — sign-up, confirmation, a spent link and the resend form, with a screenshot of each screen in both languages.

Create `e2e/test_signup.py`:

```python
"""Sign-up and confirmation, in a browser, with the mail read from the fake mailbox."""

from __future__ import annotations

import json
import time

import pytest
from playwright.sync_api import expect, sync_playwright

PASSWORD = "correct horse battery staple"

# The page's own submit button. The header carries the language switch, whose
# buttons are submit buttons too and come first in the document.
SUBMIT = "main form button[type=submit]"


def wait_for_mail(mailbox, template, to, timeout=10.0):
    """The first message of template sent to an address. The mailbox writes
    addresses lower-cased, the form in which they are compared."""
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        for path in sorted(mailbox.glob("*.json")):
            message = json.loads(path.read_text())
            if message["template"] == template and message["to"] == to.lower():
                return message
        time.sleep(0.2)
    raise AssertionError(f"no {template} mail to {to} in {mailbox}")


@pytest.mark.local_process
@pytest.mark.parametrize("language", ["pt-BR", "en-US"])
def test_sign_up_confirm_and_resend(run_marketplace, screenshots, language):
    marketplace = run_marketplace()
    email = f"leitora-{language}@example.test"
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        page.goto(marketplace.url("m1.localhost", f"/{language}/signup"))
        page.screenshot(path=screenshots / f"f13-signup-{language}.png")

        page.fill("input[name=name]", "Leitora")
        page.fill("input[name=email]", email)
        page.fill("input[name=password]", PASSWORD)
        page.click(SUBMIT)
        expect(page.locator("main h1")).to_be_visible()
        page.screenshot(path=screenshots / f"f13-check-email-{language}.png")

        # The link carries this process's own host and port: it is built from
        # the address the request reached.
        link = wait_for_mail(marketplace.mailbox, "verify-email", email)["variables"]["Link"]
        page.goto(link)
        page.screenshot(path=screenshots / f"f13-verify-{language}.png")
        page.click(SUBMIT)
        assert "/signin" in page.url

        # A spent link says so, and offers a new one.
        page.goto(link)
        page.click(SUBMIT)
        expect(page.locator("[role=alert]")).to_be_visible()
        page.screenshot(path=screenshots / f"f13-verify-failed-{language}.png")

        page.click(f"main a[href='/{language}/verify/resend']")
        expect(page.locator("input[name=email]")).to_be_visible()
        page.screenshot(path=screenshots / f"f13-resend-{language}.png")
        page.fill("input[name=email]", email)
        page.click(SUBMIT)
        expect(page.locator("[role=status]")).to_be_visible()
        page.screenshot(path=screenshots / f"f13-resend-sent-{language}.png")
        browser.close()


@pytest.mark.local_process
def test_a_breached_password_is_refused_in_the_page(run_marketplace):
    marketplace = run_marketplace()
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        page.goto(marketplace.url("m1.localhost", "/pt-BR/signup"))
        page.fill("input[name=name]", "Leitora")
        page.fill("input[name=email]", "vazada@example.test")
        page.fill("input[name=password]", "password1234")
        page.click(SUBMIT)
        expect(page.locator("[role=alert]")).to_have_text(
            "Esta senha já apareceu em um vazamento de dados em outro lugar. Escolha outra.")
        browser.close()


@pytest.mark.local_process
def test_signing_up_twice_answers_the_same_and_mails_account_exists(run_marketplace):
    marketplace = run_marketplace()
    with sync_playwright() as p:
        browser = p.chromium.launch()
        answers = []
        for _ in range(2):
            page = browser.new_page()
            page.goto(marketplace.url("m1.localhost", "/pt-BR/signup"))
            page.fill("input[name=name]", "Leitora")
            page.fill("input[name=email]", "duas-vezes@example.test")
            page.fill("input[name=password]", PASSWORD)
            page.click(SUBMIT)
            expect(page.locator("main h1")).to_have_text("Confira seu e-mail")
            answers.append(page.locator("main").inner_text())
        assert answers[0] == answers[1]
        wait_for_mail(marketplace.mailbox, "account-exists", "duas-vezes@example.test")
        browser.close()
```

- [ ] **Step 8: Run everything** — `make generate && make check && make test && make integration && make e2e`. Expected: all green; new screenshots `f13-signup-*`, `f13-check-email-*`, `f13-verify-*`, `f13-verify-failed-*`, `f13-resend-*`, `f13-resend-sent-*`, in both languages.

- [ ] **Step 9: Commit, push, open PR 2, subscribe**

```bash
git add -A internal web cmd docs e2e
git commit -m "Let a visitor create an account and confirm the address"
git push -u origin claude/funny-wright-379asb-f13accounts
```

---

# PR 3 — Sessions

## Task 9: The session schema and the session flows

**Files:**
- Create: `migrations/00011_sessions.sql`
- Modify: `internal/identity/store.go`, `internal/identity/identity.go`
- Test: `internal/identity/session_integration_test.go`

**Interfaces:**
- Produces:
  - `const SessionIdle = 30 * 24 * time.Hour`, `SessionLifetime = 90 * 24 * time.Hour`, `touchEvery = time.Hour`
  - `var ErrCredentials, ErrUnverified, ErrSessionInvalid error`
  - `func (s *Service) SignIn(ctx context.Context, v Visit, email, password string) (token string, err error)`
  - `func (s *Service) SignOut(ctx context.Context, v Visit, token string) error`
  - `type Session struct { ID string; Account Account }`
  - `func (s *Service) Authenticate(ctx context.Context, marketplace, token string) (Session, error)` — `ErrSessionInvalid` for unknown, revoked, idle or expired
  - store: `accountByID`, `passwordOf`, `passwordStill` (locks the credential and reports whether it is the one verified), `insertSession`, `liveSession`, `touchSession`, `revokeSession`

SignIn runs in three steps, and argon2 runs in no transaction: the account and its hash are read in one; the password is verified (or, for an unknown address, the dummy is verified, so both take as long, D7) with none open; the session is written in a second, which first locks the credential and confirms it is still the hash that was verified, then replaces a stale hash (D5).

`identity.signin_failed` is recorded with the platform as the actor and the account as the subject but not as the person the record is about: the refused attempt's address is then sealed under the platform's key, so the account owner's erasure request cannot erase evidence about somebody else (`audit.Entry.keyFor`/`actorKey`, `internal/platform/audit`).

- [ ] **Step 1: The migration**

Create `migrations/00011_sessions.sql`:

```sql
-- Sessions stored server-side and revocable from the first release
-- (docs/requirements.md, section 18.1). Only the token's SHA-256 is stored.

-- +goose Up
CREATE TABLE session (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id     uuid NOT NULL REFERENCES account (id) ON DELETE CASCADE,
    marketplace_id uuid REFERENCES marketplace (id) ON DELETE RESTRICT,
    token_hash     bytea       NOT NULL UNIQUE,
    created_at     timestamptz NOT NULL DEFAULT now(),
    last_seen_at   timestamptz NOT NULL DEFAULT now(),
    ip             text        NOT NULL,
    user_agent     text        NOT NULL,
    revoked_at     timestamptz,
    revoked_reason text,
    CONSTRAINT session_revocation_has_a_reason CHECK ((revoked_at IS NULL) = (revoked_reason IS NULL))
);
CREATE INDEX session_by_account ON session (account_id) WHERE revoked_at IS NULL;

ALTER TABLE session ENABLE ROW LEVEL SECURITY;
CREATE POLICY session_belongs_to_the_marketplace ON session
    FOR ALL USING (marketplace_id = current_marketplace_id())
    WITH CHECK (marketplace_id = current_marketplace_id());

-- +goose Down
DROP TABLE IF EXISTS session;
```

The `ip` and `user_agent` columns hold personal data in clear, like `email`; the audit log's copy is encrypted per person. This is the same stance the `account.email` column takes and is recorded in the pull request.

- [ ] **Step 2: Write the failing tests**

Create `internal/identity/session_integration_test.go`:

```go
//go:build integration

package identity

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/identity/breached"
	"github.com/aleogr/marketplace/internal/platform/audit"
)

// breachedNone calls no password breached.
var breachedNone = breached.Fake{}

// entry returns the first entry recorded for action.
func (r *recording) entry(t *testing.T, action string) audit.Entry {
	t.Helper()
	for _, e := range r.entries {
		if e.Action == action {
			return e
		}
	}
	t.Fatalf("no %s among the audited actions %v", action, r.actions())
	return audit.Entry{}
}

// confirmed signs up and confirms an account with the given address.
func confirmed(t *testing.T, s *Service, db serving, marketplace, email string) {
	t.Helper()
	if err := s.SignUp(t.Context(), visit(marketplace), "Reader", email, "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	mails := outbox(t, db, marketplace)
	last := mails[len(mails)-1]
	if last.template != "verify-email" {
		t.Fatalf("the last mail is %v, want the verification of %s", last, email)
	}
	if err := s.Verify(t.Context(), visit(marketplace), tokenOf(t, last.link)); err != nil {
		t.Fatal(err)
	}
}

// signIn signs in and fails the test if that is refused.
func signIn(t *testing.T, s *Service, marketplace, email, password string) string {
	t.Helper()
	token, err := s.SignIn(t.Context(), visit(marketplace), email, password)
	if err != nil {
		t.Fatalf("SignIn(%s) = %v", email, err)
	}
	return token
}

func TestSignInOutAndRevocation(t *testing.T) {
	s, db, one, _, _ := service(t)
	confirmed(t, s, db, one, "r@example.test")

	token := signIn(t, s, one, "R@Example.Test", "correct horse battery staple")
	session, err := s.Authenticate(t.Context(), one, token)
	if err != nil || session.Account.Email != "r@example.test" {
		t.Fatalf("Authenticate = %+v, %v", session, err)
	}
	if err := s.SignOut(t.Context(), visit(one), token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(t.Context(), one, token); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("Authenticate after sign-out = %v, want ErrSessionInvalid", err)
	}
}

func TestWrongPasswordAndUnknownAddressLookAlike(t *testing.T) {
	s, db, one, _, trail := service(t)
	confirmed(t, s, db, one, "r@example.test")
	_, wrong := s.SignIn(t.Context(), visit(one), "r@example.test", "not the password at all")
	_, unknown := s.SignIn(t.Context(), visit(one), "nobody@example.test", "not the password at all")
	if !errors.Is(wrong, ErrCredentials) || !errors.Is(unknown, ErrCredentials) {
		t.Fatalf("wrong = %v, unknown = %v; want ErrCredentials for both", wrong, unknown)
	}

	// The failure against an existing account is audited, with the platform as
	// the actor and nobody as the person it is about: the address it came
	// from is not sealed under the account owner's key.
	failed := trail.entry(t, "identity.signin_failed")
	if failed.Actor.Kind != audit.System || failed.Actor.ID != "" || failed.Subject.Person != "" ||
		failed.Subject.Kind != "account" || failed.Subject.ID == "" || failed.From != "203.0.113.7" {
		t.Fatalf("signin_failed recorded as %+v", failed)
	}
}

func TestAnUnconfirmedAccountCannotSignIn(t *testing.T) {
	s, _, one, _, _ := service(t)
	if err := s.SignUp(t.Context(), visit(one), "R", "r@example.test", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SignIn(t.Context(), visit(one), "r@example.test", "correct horse battery staple"); !errors.Is(err, ErrUnverified) {
		t.Fatalf("SignIn(unconfirmed) = %v, want ErrUnverified", err)
	}
}

func TestIdleAndExpiredSessionsAreRefused(t *testing.T) {
	s, db, one, _, _ := service(t)
	confirmed(t, s, db, one, "r@example.test")
	idle := signIn(t, s, one, "r@example.test", "correct horse battery staple")
	old := signIn(t, s, one, "r@example.test", "correct horse battery staple")
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		if _, err := tx.Exec(t.Context(),
			`UPDATE session SET last_seen_at = now() - interval '31 days' WHERE token_hash = $1`, HashToken(idle)); err != nil {
			return err
		}
		_, err := tx.Exec(t.Context(),
			`UPDATE session SET created_at = now() - interval '91 days' WHERE token_hash = $1`, HashToken(old))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	for name, token := range map[string]string{"idle": idle, "expired": old} {
		if _, err := s.Authenticate(t.Context(), one, token); !errors.Is(err, ErrSessionInvalid) {
			t.Errorf("%s session: %v, want ErrSessionInvalid", name, err)
		}
	}
}

func TestASessionIsInvisibleFromAnotherMarketplace(t *testing.T) {
	s, db, one, two, _ := service(t)
	confirmed(t, s, db, one, "r@example.test")
	token := signIn(t, s, one, "r@example.test", "correct horse battery staple")
	if _, err := s.Authenticate(t.Context(), one, token); err != nil {
		t.Fatalf("the session does not work in its own marketplace: %v", err)
	}
	if _, err := s.Authenticate(t.Context(), two, token); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("a session of one marketplace authenticated in the other: %v", err)
	}
}

func TestAStaleHashIsReplacedAtSignIn(t *testing.T) {
	db, one, _ := twoMarketplaces(t)
	old := NewService(db, NewHasher(cheap, 1), breachedNone, &recording{}, quiet)
	confirmed(t, old, db, one, "r@example.test")
	newer := NewService(db, NewHasher(Params{Memory: 128, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16}, 1),
		breachedNone, &recording{}, quiet)
	signIn(t, newer, one, "r@example.test", "correct horse battery staple")

	var secret string
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		return tx.QueryRow(t.Context(), "SELECT secret FROM credential").Scan(&secret)
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(secret, "m=128,") {
		t.Fatalf("the hash was not refreshed: %s", secret)
	}
	// And the refreshed hash still opens the account.
	signIn(t, newer, one, "r@example.test", "correct horse battery staple")
}
```

- [ ] **Step 3: Run to see them fail** — `go test -tags=integration ./internal/identity/ -run 'Sign|Idle|Session|Stale' -v` → FAIL to compile.

- [ ] **Step 4: Implement**

Append to `internal/identity/store.go`:

```go
func accountByID(ctx context.Context, tx pgx.Tx, id string) (Account, error) {
	return scanAccount(tx.QueryRow(ctx, `SELECT `+accountColumns+` FROM account WHERE id = $1`, id))
}

func passwordOf(ctx context.Context, tx pgx.Tx, account string) (string, error) {
	var secret string
	err := tx.QueryRow(ctx,
		`SELECT secret FROM credential WHERE account_id = $1 AND kind = 'password'`, account).Scan(&secret)
	return secret, err
}

// passwordStill locks an account's password and reports whether it is still
// the one a flow verified outside the transaction. A flow that hashed with no
// transaction open writes only if nobody changed the password meanwhile.
func passwordStill(ctx context.Context, tx pgx.Tx, account, verified string) (bool, error) {
	var secret string
	err := tx.QueryRow(ctx,
		`SELECT secret FROM credential WHERE account_id = $1 AND kind = 'password' FOR UPDATE`,
		account).Scan(&secret)
	if err != nil {
		return false, err
	}
	return secret == verified, nil
}

func insertSession(ctx context.Context, tx pgx.Tx, marketplace, account string, hash []byte, ip, agent string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO session (account_id, marketplace_id, token_hash, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5)`, account, marketplace, hash, ip, agent)
	return err
}

type sessionRow struct {
	id, account   string
	created, seen time.Time
}

func liveSession(ctx context.Context, tx pgx.Tx, hash []byte) (sessionRow, error) {
	var r sessionRow
	err := tx.QueryRow(ctx, `
		SELECT id::text, account_id::text, created_at, last_seen_at FROM session
		 WHERE token_hash = $1 AND revoked_at IS NULL`, hash).Scan(&r.id, &r.account, &r.created, &r.seen)
	return r, err
}

func touchSession(ctx context.Context, tx pgx.Tx, id string, now time.Time) error {
	_, err := tx.Exec(ctx, `UPDATE session SET last_seen_at = $2 WHERE id = $1`, id, now)
	return err
}

func revokeSession(ctx context.Context, tx pgx.Tx, hash []byte, reason string, now time.Time) (string, error) {
	var account string
	err := tx.QueryRow(ctx, `
		UPDATE session SET revoked_at = $3, revoked_reason = $2
		 WHERE token_hash = $1 AND revoked_at IS NULL RETURNING account_id::text`, hash, reason, now).Scan(&account)
	return account, err
}
```

Append to `internal/identity/identity.go`:

```go
// Session lengths (spec, D3): the owner's choice of 2026-09-23. Constants now,
// parameters in F17.
const (
	SessionIdle     = 30 * 24 * time.Hour
	SessionLifetime = 90 * 24 * time.Hour
	touchEvery      = time.Hour
)

var (
	// ErrCredentials is a wrong password and an unknown address alike (D7).
	ErrCredentials = errors.New("identity: credentials refused")
	// ErrUnverified is a right password on an unconfirmed account.
	ErrUnverified = errors.New("identity: address not confirmed")
	// ErrSessionInvalid is a session that is unknown, revoked, idle or expired.
	ErrSessionInvalid = errors.New("identity: session invalid")
)

// Session is a signed-in request's account.
type Session struct {
	ID      string
	Account Account
}

// refusal audits an attempt against an account by somebody not known to be
// its owner. The platform is the actor and the account only the subject, not
// the person the record is about: the address the attempt came from is then
// sealed under the platform's key, so the owner's erasure request cannot
// erase the evidence about somebody else (internal/platform/audit).
func (s *Service) refusal(ctx context.Context, tx pgx.Tx, v Visit, account, action string) error {
	after, err := json.Marshal(map[string]string{"user_agent": v.UserAgent})
	if err != nil {
		return err
	}
	return s.audit.Append(ctx, tx, audit.Entry{
		Marketplace: v.Marketplace,
		Actor:       audit.Actor{Kind: audit.System},
		Action:      action,
		Subject:     audit.Subject{Kind: "account", ID: account},
		From:        v.IP,
		After:       after,
	})
}

// credentials reads an account and its password hash, or reports found false
// for an address with no account in this marketplace.
func (s *Service) credentials(ctx context.Context, marketplace, normalised string) (account Account, secret string, found bool, err error) {
	err = s.db.InTxFor(ctx, marketplace, func(tx pgx.Tx) error {
		a, err := accountByEmail(ctx, tx, normalised)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		account, found = a, true
		secret, err = passwordOf(ctx, tx, a.ID)
		return err
	})
	return account, secret, found, err
}

// SignIn checks a password and opens a new session, returning its token.
//
// Three steps, and argon2 runs in none of the transactions: the account is
// read in one, the password verified with none open, and the session written
// in a second, which first checks that the password it verified is still the
// account's.
func (s *Service) SignIn(ctx context.Context, v Visit, email, password string) (string, error) {
	normalised, err := NormaliseEmail(email)
	if err != nil {
		s.hasher.Waste(ctx, password)
		return "", ErrCredentials
	}
	account, secret, found, err := s.credentials(ctx, v.Marketplace, normalised)
	if err != nil {
		return "", err
	}
	if !found {
		// The same work as a wrong password, so the two take as long (D7).
		s.hasher.Waste(ctx, password)
		s.log.InfoContext(ctx, "sign-in for an unknown address")
		return "", ErrCredentials
	}

	ok, stale, err := s.hasher.Verify(ctx, password, secret)
	if err != nil {
		return "", err
	}
	if !ok {
		if err := s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
			return s.refusal(ctx, tx, v, account.ID, "identity.signin_failed")
		}); err != nil {
			return "", err
		}
		return "", ErrCredentials
	}
	if account.VerifiedAt == nil {
		return "", ErrUnverified
	}

	// A hash made with other parameters is made again now, while the password
	// is at hand (spec, D5).
	var fresh string
	if stale {
		if fresh, err = s.hasher.Hash(ctx, password); err != nil {
			return "", err
		}
	}
	token, hash, err := NewToken()
	if err != nil {
		return "", err
	}
	err = s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		still, err := passwordStill(ctx, tx, account.ID, secret)
		if err != nil {
			return err
		}
		if !still {
			// Changed between the read and now: what was verified is no
			// longer the password.
			return ErrCredentials
		}
		if fresh != "" {
			if err := setPassword(ctx, tx, v.Marketplace, account.ID, fresh); err != nil {
				return err
			}
		}
		if err := insertSession(ctx, tx, v.Marketplace, account.ID, hash, v.IP, v.UserAgent); err != nil {
			return err
		}
		return s.record(ctx, tx, v, account.ID, "identity.signin")
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// SignOut revokes the session a token opened.
func (s *Service) SignOut(ctx context.Context, v Visit, token string) error {
	return s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		account, err := revokeSession(ctx, tx, HashToken(token), "signout", s.now())
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		return s.record(ctx, tx, v, account, "identity.signout")
	})
}

// Authenticate returns the session a token belongs to in marketplace.
func (s *Service) Authenticate(ctx context.Context, marketplace, token string) (Session, error) {
	var session Session
	err := s.db.InTxFor(ctx, marketplace, func(tx pgx.Tx) error {
		now := s.now()
		row, err := liveSession(ctx, tx, HashToken(token))
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSessionInvalid
		}
		if err != nil {
			return err
		}
		if now.Sub(row.seen) > SessionIdle || now.Sub(row.created) > SessionLifetime {
			return ErrSessionInvalid
		}
		if now.Sub(row.seen) > touchEvery {
			if err := touchSession(ctx, tx, row.id, now); err != nil {
				return err
			}
		}
		account, err := accountByID(ctx, tx, row.account)
		if err != nil {
			return err
		}
		session = Session{ID: row.id, Account: account}
		return nil
	})
	return session, err
}
```

- [ ] **Step 5: Run to see them pass** — `go test -tags=integration -count=1 ./internal/identity/ -race -v && make check` → PASS; `0 issues`.

- [ ] **Step 6: Commit** — `git add internal/identity migrations/00011_sessions.sql && git commit -m "Open, check and revoke sessions stored on the server"`

## Task 10: The session middleware, sign-in and sign-out pages

**Files:**
- Create: `internal/platform/httpx/session.go`
- Modify: `internal/platform/httpx/csrf.go` (`RenewCSRF`, sharing the secret's cookie code), `internal/platform/httpx/identity.go` (routes, handlers), `internal/platform/httpx/site.go` (`page()` carries the account), `web/page.go` (`Account`), `web/layout.templ` (the account navigation), `web/identity.templ` (`SignIn`), `web/locales/*.json`, `cmd/marketplace/main.go` (mount)
- Test: `internal/platform/httpx/session_test.go`, `internal/platform/httpx/identity_integration_test.go` (tag `integration`)

**Interfaces:**
- Produces: `func Sessions(service *identity.Service, log *slog.Logger) func(http.Handler) http.Handler`; `func SessionFrom(ctx context.Context) (identity.Session, bool)`; `func RenewCSRF(w http.ResponseWriter, r *http.Request)`; templ `SignIn(page Page, form Form, verified bool)`, `AccountNav(page Page)`; the header's `<nav class="account">`, labelled through `identity.header.account`.

- [ ] **Step 1: Locale keys**

Add to `web/locales/en-US.json` and `web/locales/pt-BR.json` (the same keys in both, at the end of each file):

| key | en-US | pt-BR |
|---|---|---|
| `identity.signin.title` | Sign in | Entrar |
| `identity.signin.submit` | Sign in | Entrar |
| `identity.signin.failed` | The e-mail or the password is not right. | O e-mail ou a senha não conferem. |
| `identity.signin.unverified` | Confirm your e-mail address first. We can send the link again. | Confirme seu e-mail primeiro. Podemos reenviar o link. |
| `identity.signout.submit` | Sign out | Sair |
| `identity.header.account` | Account | Conta |
| `identity.header.signed_in_as` | Signed in as %[1]s | Conectado como %[1]s |

- [ ] **Step 2: Write the failing tests**

Create `internal/platform/httpx/session_test.go`:

```go
package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aleogr/marketplace/internal/platform/httpx"
)

// A request with no session cookie carries no session and never reaches the
// service: the middleware is given none here, and would panic if it did.
func TestNoCookieMeansNoSessionAndNoDatabaseCall(t *testing.T) {
	var seen, reached bool
	handler := httpx.Sessions(nil, silent())(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		reached = true
		_, seen = httpx.SessionFrom(r.Context())
	}))
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), inMarketplace(request))
	if !reached {
		t.Fatal("the request did not reach the handler")
	}
	if seen {
		t.Fatal("a request with no cookie carried a session")
	}
}

// Sign-in replaces the CSRF secret the browser held (sign-in CSRF, session
// fixation).
func TestRenewCSRFReplacesTheSecret(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/signin", nil)
	request.AddCookie(&http.Cookie{Name: "csrf", Value: "planted"})
	recorder := httptest.NewRecorder()

	httpx.RenewCSRF(recorder, request)

	var renewed *http.Cookie
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "csrf" {
			renewed = cookie
		}
	}
	if renewed == nil || renewed.Value == "" || renewed.Value == "planted" {
		t.Fatalf("the CSRF cookie was not renewed: %+v", renewed)
	}
	if !renewed.HttpOnly || renewed.SameSite != http.SameSiteLaxMode || renewed.Path != "/" {
		t.Errorf("the renewed cookie lost its flags: %+v", renewed)
	}
}
```

The spec's integration test for the limiter: a credential-stuffing run against one address, from a different client each time, through the real handler and the real database.

Create `internal/platform/httpx/identity_integration_test.go`:

```go
//go:build integration

package httpx_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/aleogr/marketplace/internal/identity"
	"github.com/aleogr/marketplace/internal/identity/breached"
	"github.com/aleogr/marketplace/internal/platform/config"
	"github.com/aleogr/marketplace/internal/platform/db"
	"github.com/aleogr/marketplace/internal/platform/dbtest"
	"github.com/aleogr/marketplace/internal/platform/httpx"
	"github.com/aleogr/marketplace/internal/platform/ratelimit"
)

func TestMain(m *testing.M) { os.Exit(dbtest.Run(m)) }

// A credential-stuffing run tries many passwords against one address, from
// many addresses of its own. The per-address limit is what stops it: the
// eleventh attempt within the window is refused whichever client sends it,
// while another address is still served (docs/roadmap.md, F13).
func TestCredentialStuffingAgainstOneAddressIsStopped(t *testing.T) {
	settings := config.Database{URL: dbtest.URL(t)}
	pool, err := db.Open(t.Context(), settings)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(t.Context(), pool, settings, silent()); err != nil {
		t.Fatal(err)
	}

	service := identity.NewService(pool, identity.NewHasher(cheap, 2), breached.Fake{}, nil, silent())
	handler := identityHandler(t, service, httpx.IdentityLimits{
		SignIn:        ratelimit.NewDatabase(pool, "signin", 1000, 10*time.Minute),
		SignInAddress: ratelimit.NewDatabase(pool, "signin-address", 10, 15*time.Minute),
	})

	attempt := func(email string, n int) int {
		request := formRequest(t, "/signin",
			url.Values{"email": {email}, "password": {fmt.Sprintf("guess number %04d", n)}})
		// A different client each time, as a botnet would be.
		request = request.WithContext(httpx.WithOrigin(request.Context(),
			httpx.Origin{IP: fmt.Sprintf("198.51.100.%d", n+1)}))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder.Code
	}

	for n := range 10 {
		if code := attempt("alvo@example.test", n); code != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d, want %d: refused as a wrong password", n+1, code, http.StatusUnauthorized)
		}
	}
	if code := attempt("alvo@example.test", 10); code != http.StatusTooManyRequests {
		t.Fatalf("the eleventh attempt = %d, want %d", code, http.StatusTooManyRequests)
	}
	if code := attempt("outra@example.test", 11); code != http.StatusUnauthorized {
		t.Fatalf("another address = %d, want %d: the limit is per address", code, http.StatusUnauthorized)
	}
}
```

- [ ] **Step 3: Implement**

Create `internal/platform/httpx/session.go`:

```go
package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/aleogr/marketplace/internal/identity"
)

// The session cookie follows the CSRF cookie's rules (csrf.go): `__Host-` over
// HTTPS, bound to this marketplace's host, never readable by script.
const (
	sessionCookie         = "__Host-session"
	sessionCookieInsecure = "session"
)

type sessionKey struct{}

// SessionFrom returns the signed-in session of a request, if there is one.
func SessionFrom(ctx context.Context) (identity.Session, bool) {
	s, ok := ctx.Value(sessionKey{}).(identity.Session)
	return s, ok
}

func sessionCookieName(r *http.Request) string {
	if isHTTPS(r) {
		return sessionCookie
	}
	return sessionCookieInsecure
}

// Sessions puts the signed-in account in the request's context. A cookie that
// no longer opens a session is cleared, so the browser stops sending it. A
// request with no cookie, or on a host that is no marketplace, never reaches
// the database.
func Sessions(service *identity.Service, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName(r))
			marketplace := marketplaceOf(r)
			if err != nil || cookie.Value == "" || marketplace == "" {
				next.ServeHTTP(w, r)
				return
			}
			session, err := service.Authenticate(r.Context(), marketplace, cookie.Value)
			switch {
			case errors.Is(err, identity.ErrSessionInvalid):
				clearSession(w, r)
			case err != nil:
				log.ErrorContext(r.Context(), "a session could not be checked", "error", err)
			default:
				r = r.WithContext(context.WithValue(r.Context(), sessionKey{}, session))
			}
			next.ServeHTTP(w, r)
		})
	}
}

func setSession(w http.ResponseWriter, r *http.Request, token string) {
	// #nosec G124 -- Secure follows the scheme, as the CSRF cookie's does (csrf.go).
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName(r), Value: token, Path: "/", HttpOnly: true,
		Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode,
		MaxAge: int(identity.SessionLifetime.Seconds()),
	})
}

func clearSession(w http.ResponseWriter, r *http.Request) {
	// #nosec G124 -- see setSession.
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName(r), Value: "", Path: "/", HttpOnly: true,
		Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}
```

`RenewCSRF` sets a new secret with the same code the middleware sets one with:

In `internal/platform/httpx/csrf.go`, replace:

```go
// csrfSecret returns this browser's secret, setting a new one when it has none
// or when what it sent is not one.
func csrfSecret(w http.ResponseWriter, r *http.Request) []byte {
	name := csrfCookie
	if !isHTTPS(r) {
		name = csrfCookieInsecure
	}

	if cookie, err := r.Cookie(name); err == nil {
		if secret, err := base64.RawURLEncoding.DecodeString(cookie.Value); err == nil && len(secret) == secretBytes {
			return secret
		}
	}

	secret := make([]byte, secretBytes)
```

with:

```go
// csrfSecret returns this browser's secret, setting a new one when it has none
// or when what it sent is not one.
func csrfSecret(w http.ResponseWriter, r *http.Request) []byte {
	if cookie, err := r.Cookie(csrfCookieName(r)); err == nil {
		if secret, err := base64.RawURLEncoding.DecodeString(cookie.Value); err == nil && len(secret) == secretBytes {
			return secret
		}
	}
	return newCSRFSecret(w, r)
}

// RenewCSRF gives the browser a new secret whatever it held. Sign-in calls it,
// so a token minted before the session existed — possibly by somebody who
// planted the secret — does not carry over into it (sign-in CSRF, session
// fixation).
func RenewCSRF(w http.ResponseWriter, r *http.Request) { newCSRFSecret(w, r) }

// csrfCookieName is the cookie's name for the scheme the request came in on.
func csrfCookieName(r *http.Request) string {
	if isHTTPS(r) {
		return csrfCookie
	}
	return csrfCookieInsecure
}

// newCSRFSecret sets a new secret in the browser and returns it.
func newCSRFSecret(w http.ResponseWriter, r *http.Request) []byte {
	secret := make([]byte, secretBytes)
```

In `internal/platform/httpx/csrf.go`, replace:

```go
		Name:     name,
```

with:

```go
		Name:     csrfCookieName(r),
```

Routes and handlers:

In `internal/platform/httpx/identity.go`, replace:

```go
		limit(id.Limits.ResendAddress, byAddress, http.HandlerFunc(s.resend))))
}
```

with:

```go
		limit(id.Limits.ResendAddress, byAddress, http.HandlerFunc(s.resend))))
	mux.HandleFunc("GET /signin", s.signInForm)
	mux.Handle("POST /signin", limit(id.Limits.SignIn, byIP,
		limit(id.Limits.SignInAddress, byAddress, http.HandlerFunc(s.signIn))))
	mux.HandleFunc("POST /signout", s.signOut)
}
```

Append to `internal/platform/httpx/identity.go`:

```go
func (s Site) signInForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, web.SignIn(s.page(r, "/signin"), newForm(), r.URL.Query().Get("verified") == "1"))
}

func (s Site) signIn(w http.ResponseWriter, r *http.Request) {
	form := newForm()
	form.Email = r.PostFormValue("email")
	token, err := s.identity.Service.SignIn(r.Context(), visit(r), form.Email, r.PostFormValue("password"))
	switch {
	case errors.Is(err, identity.ErrCredentials):
		form.Error = "identity.signin.failed"
	case errors.Is(err, identity.ErrUnverified):
		form.Error = "identity.signin.unverified"
	case err != nil:
		s.identity.Log.ErrorContext(r.Context(), "sign-in failed", "error", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	default:
		setSession(w, r, token)
		RenewCSRF(w, r)
		http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+"/", http.StatusSeeOther)
		return
	}
	renderStatus(w, r, http.StatusUnauthorized, web.SignIn(s.page(r, "/signin"), form, false))
}

func (s Site) signOut(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName(r)); err == nil {
		if err := s.identity.Service.SignOut(r.Context(), visit(r), cookie.Value); err != nil {
			s.identity.Log.ErrorContext(r.Context(), "sign-out failed", "error", err)
		}
	}
	clearSession(w, r)
	http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+"/", http.StatusSeeOther)
}
```

In `internal/platform/httpx/site.go`, replace:

```go
		page.Marketplace = resolution.Marketplace.Name
	}
```

with:

```go
		page.Marketplace = resolution.Marketplace.Name
	}
	if session, ok := SessionFrom(r.Context()); ok {
		page.Account = session.Account.Name
	}
```

In `web/page.go`, replace:

```go
	CSRFToken string
```

with:

```go
	CSRFToken string
	// Account is the signed-in account's name, empty when signed out.
	Account string
```

The pages:

Append to `web/identity.templ`:

```templ
templ SignIn(page Page, form Form, verified bool) {
	@Layout(page, page.T("identity.signin.title")) {
		<h1>{ page.T("identity.signin.title") }</h1>
		if verified {
			<p role="status">{ page.T("identity.verify.done") }</p>
		}
		if form.Error != "" {
			<p role="alert">{ page.T(form.Error, form.ErrorArgs...) }</p>
		}
		<form method="post" action={ templ.SafeURL("/" + page.Language + "/signin") }>
			<input type="hidden" name="csrf_token" value={ page.CSRFToken }/>
			<label>
				{ page.T("identity.field.email") }
				<input name="email" type="email" autocomplete="username" required value={ form.Email }/>
			</label>
			<label>
				{ page.T("identity.field.password") }
				<input name="password" type="password" autocomplete="current-password" required/>
			</label>
			<button type="submit">{ page.T("identity.signin.submit") }</button>
		</form>
		<p>
			<a href={ templ.SafeURL("/" + page.Language + "/signup") }>{ page.T("identity.link.signup") }</a>
			<a href={ templ.SafeURL("/" + page.Language + "/verify/resend") }>{ page.T("identity.link.resend") }</a>
		</p>
	}
}
```

The account navigation sits in the header, beside the language switch, on a marketplace's pages only (the platform's pages and `InPreparation` have no accounts):

In `web/layout.templ`, replace:

```templ
				@LanguageSwitch(page)
			</header>
```

with:

```templ
				@LanguageSwitch(page)
				if page.Marketplace != "" {
					@AccountNav(page)
				}
			</header>
```

Append to `web/layout.templ`:

```templ
// AccountNav is who is signed in, on a marketplace's pages: the name and the
// way out, or the two ways in. The platform's own pages belong to no
// marketplace and have no accounts, so they carry none.
templ AccountNav(page Page) {
	<nav class="account" aria-label={ page.T("identity.header.account") }>
		if page.Account != "" {
			<span>{ page.T("identity.header.signed_in_as", page.Account) }</span>
			<form method="post" action={ templ.SafeURL("/" + page.Language + "/signout") }>
				<input type="hidden" name="csrf_token" value={ page.CSRFToken }/>
				<button type="submit">{ page.T("identity.signout.submit") }</button>
			</form>
		} else {
			<a href={ templ.SafeURL("/" + page.Language + "/signin") }>{ page.T("identity.link.signin") }</a>
			<a href={ templ.SafeURL("/" + page.Language + "/signup") }>{ page.T("identity.link.signup") }</a>
		}
	</nav>
}
```

Mount the middleware in `cmd/marketplace/main.go`. It wraps the routes and the language and tenancy resolvers wrap it, so a request meets tenancy, then language, then the session, then the mux:

In `cmd/marketplace/main.go`, replace:

```go
	handler := site.Handler()

```

with:

```go
	handler := site.Handler()

	// The session is read after the marketplace and the language are known,
	// because a session belongs to one marketplace and is looked up in its
	// transaction (docs/design.md, section 2.3). The resolvers below wrap
	// this, so a request meets them first.
	if identityService != nil {
		handler = httpx.Sessions(identityService, log)(handler)
	}

```

- [ ] **Step 4: Run** — `make generate && go test ./internal/platform/httpx/ ./web/ -race -v && go test -tags=integration -count=1 ./internal/platform/httpx/ -v && make check` → PASS (including the literal and parity checks); `0 issues`.

- [ ] **Step 5: Commit** — `git add -A internal web cmd && git commit -m "Sign in and out, with the session read on every request"`

## Task 11: End-to-end — sign-in, sign-out, refusals and the limit

**Files:**
- Create: `e2e/accounts.py` (the helpers that sign up and confirm through the mailbox, reused by Task 15), `e2e/test_signin.py`
- Modify: `e2e/test_signup.py` (takes its helpers from `accounts.py`)

- [ ] **Step 1: The helpers**

Create `e2e/accounts.py`:

```python
"""Make a confirmed account the way a person would, and hand it to a test."""

from __future__ import annotations

import json
import time

PASSWORD = "correct horse battery staple"

# The page's own submit button. The header carries the language switch and,
# signed in, the sign-out button, which are submit buttons too and come first
# in the document.
SUBMIT = "main form button[type=submit]"

# Who is signed in, in the header.
SIGNED_IN = "nav.account span"
SIGN_IN_LINK = "nav.account a[href$='/signin']"


def wait_for_mail(mailbox, template, to, timeout=10.0):
    """The first message of template sent to an address. The mailbox writes
    addresses lower-cased, the form in which they are compared."""
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        for path in sorted(mailbox.glob("*.json")):
            message = json.loads(path.read_text())
            if message["template"] == template and message["to"] == to.lower():
                return message
        time.sleep(0.2)
    raise AssertionError(f"no {template} mail to {to} in {mailbox}")


def confirmed_account(page, marketplace, email, host="m1.localhost", language="pt-BR"):
    """Sign up and follow the confirmation link, as a person would. The link
    carries the process's own host and port."""
    page.goto(marketplace.url(host, f"/{language}/signup"))
    page.fill("input[name=name]", "Leitora")
    page.fill("input[name=email]", email)
    page.fill("input[name=password]", PASSWORD)
    page.click(SUBMIT)
    page.goto(wait_for_mail(marketplace.mailbox, "verify-email", email)["variables"]["Link"])
    page.click(SUBMIT)


def sign_in(page, marketplace, email, password=PASSWORD, host="m1.localhost", language="pt-BR"):
    page.goto(marketplace.url(host, f"/{language}/signin"))
    page.fill("input[name=email]", email)
    page.fill("input[name=password]", password)
    page.click(SUBMIT)
```

In `e2e/test_signup.py`, replace:

```python
import json
import time

import pytest
from playwright.sync_api import expect, sync_playwright

PASSWORD = "correct horse battery staple"

# The page's own submit button. The header carries the language switch, whose
# buttons are submit buttons too and come first in the document.
SUBMIT = "main form button[type=submit]"


def wait_for_mail(mailbox, template, to, timeout=10.0):
    """The first message of template sent to an address. The mailbox writes
    addresses lower-cased, the form in which they are compared."""
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        for path in sorted(mailbox.glob("*.json")):
            message = json.loads(path.read_text())
            if message["template"] == template and message["to"] == to.lower():
                return message
        time.sleep(0.2)
    raise AssertionError(f"no {template} mail to {to} in {mailbox}")
```

with:

```python
import pytest
from playwright.sync_api import expect, sync_playwright

from accounts import PASSWORD, SUBMIT, wait_for_mail
```

- [ ] **Step 2: The tests** — every negative check comes with the positive one that proves the page loaded.

Create `e2e/test_signin.py`:

```python
"""Sign-in and sign-out, and what a stranger can learn from them: nothing."""

from __future__ import annotations

import pytest
from playwright.sync_api import expect, sync_playwright

from accounts import PASSWORD, SIGN_IN_LINK, SIGNED_IN, SUBMIT, confirmed_account, sign_in


@pytest.mark.local_process
@pytest.mark.parametrize("language", ["pt-BR", "en-US"])
def test_sign_in_and_out(run_marketplace, screenshots, language):
    marketplace = run_marketplace()
    email = f"entra-{language}@example.test"
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        confirmed_account(page, marketplace, email, language=language)
        page.screenshot(path=screenshots / f"f13-signin-{language}.png")
        sign_in(page, marketplace, email, language=language)
        expect(page.locator(SIGNED_IN)).to_be_visible()
        page.screenshot(path=screenshots / f"f13-signed-in-{language}.png")
        page.click("nav.account button[type=submit]")
        expect(page.locator(SIGN_IN_LINK)).to_be_visible()
        expect(page.locator(SIGNED_IN)).to_have_count(0)
        browser.close()


@pytest.mark.local_process
def test_wrong_password_and_unknown_address_read_the_same(run_marketplace):
    marketplace = run_marketplace()
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        confirmed_account(page, marketplace, "existe@example.test")
        sign_in(page, marketplace, "existe@example.test", password="not the right password")
        expect(page.locator("[role=alert]")).to_be_visible()
        wrong = page.locator("[role=alert]").inner_text()
        sign_in(page, marketplace, "nao-existe@example.test", password="not the right password")
        expect(page.locator("[role=alert]")).to_be_visible()
        unknown = page.locator("[role=alert]").inner_text()
        assert wrong == unknown == "O e-mail ou a senha não conferem."
        browser.close()


@pytest.mark.local_process
def test_an_unconfirmed_account_is_told_to_confirm(run_marketplace):
    marketplace = run_marketplace()
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        page.goto(marketplace.url("m1.localhost", "/pt-BR/signup"))
        page.fill("input[name=name]", "Leitora")
        page.fill("input[name=email]", "pendente@example.test")
        page.fill("input[name=password]", PASSWORD)
        page.click(SUBMIT)
        sign_in(page, marketplace, "pendente@example.test")
        expect(page.locator("[role=alert]")).to_have_text(
            "Confirme seu e-mail primeiro. Podemos reenviar o link.")
        browser.close()


@pytest.mark.local_process
def test_credential_stuffing_against_one_address_is_stopped(run_marketplace):
    """Ten attempts per address in fifteen minutes; the eleventh is refused."""
    marketplace = run_marketplace()
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        confirmed_account(page, marketplace, "alvo@example.test")
        statuses = []
        for attempt in range(11):
            page.goto(marketplace.url("m1.localhost", "/pt-BR/signin"))
            page.fill("input[name=email]", "alvo@example.test")
            page.fill("input[name=password]", f"guess number {attempt:04d}")
            with page.expect_response(lambda r: r.request.method == "POST") as info:
                page.click(SUBMIT)
            statuses.append(info.value.status)
        assert statuses[:10] == [401] * 10 and statuses[10] == 429, statuses
        browser.close()


@pytest.mark.local_process
def test_a_session_does_not_cross_marketplaces(run_marketplace):
    marketplace = run_marketplace()
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        confirmed_account(page, marketplace, "so-no-um@example.test")
        sign_in(page, marketplace, "so-no-um@example.test")
        expect(page.locator(SIGNED_IN)).to_be_visible()
        page.goto(marketplace.url("m2.localhost", "/pt-BR/"))
        expect(page.locator(SIGN_IN_LINK)).to_be_visible()
        expect(page.locator(SIGNED_IN)).to_have_count(0)
        browser.close()
```

- [ ] **Step 3: Run everything** — `make generate && make check && make test && make integration && make e2e` → all green; screenshots `f13-signin-*`, `f13-signed-in-*`.

- [ ] **Step 4: Commit** — `git add e2e && git commit -m "Prove sign-in, sign-out and their refusals in a browser"`

## Task 12: The argon2id benchmark command

It lands in PR 3 so that it is deployed when PR 3 merges: only `main` deploys (`.github/workflows/deploy.yml`), and the owner runs it on the lab's CPU before PR 4 sets the parameters (Task 14).

**Files:**
- Create: `cmd/marketplace/bench.go`, `cmd/marketplace/bench_test.go`
- Modify: `cmd/marketplace/main.go` (the `run` switch), `docs/infrastructure.md`

**Interfaces:**
- Produces: `func benchPassword(ctx context.Context, log *slog.Logger) error` — logs one line per candidate `{"message":"argon2id","memory_kib":…,"time":…,"median_ms":…}` and one `{"message":"argon2id chosen","memory_kib":…,"time":…,"budget_ms":250}`; `func choose(results []benchResult, budget time.Duration) identity.Params` — the most expensive candidate (by memory × time) whose median is within budget, or `identity.Floor`.

- [ ] **Step 1: Write the failing test**

Create `cmd/marketplace/bench_test.go`:

```go
package main

import (
	"testing"
	"time"

	"github.com/aleogr/marketplace/internal/identity"
)

func TestChooseTakesTheMostExpensiveWithinBudget(t *testing.T) {
	results := []benchResult{
		{Params: identity.Params{Memory: 19456, Time: 2}, Median: 60 * time.Millisecond},
		{Params: identity.Params{Memory: 47104, Time: 2}, Median: 150 * time.Millisecond},
		{Params: identity.Params{Memory: 65536, Time: 3}, Median: 320 * time.Millisecond},
	}
	got := choose(results, 250*time.Millisecond)
	if got.Memory != 47104 || got.Time != 2 {
		t.Fatalf("choose = %+v, want m=47104 t=2", got)
	}
}

func TestChooseFallsBackToTheFloor(t *testing.T) {
	got := choose([]benchResult{{Params: identity.Params{Memory: 65536, Time: 3}, Median: time.Second}}, 250*time.Millisecond)
	if got.Memory != identity.Floor.Memory || got.Time != identity.Floor.Time {
		t.Fatalf("choose = %+v, want the OWASP floor", got)
	}
}
```

- [ ] **Step 2: Run to see it fail** — `go test ./cmd/marketplace/ -run Choose -v` → FAIL (undefined).

- [ ] **Step 3: Implement**

Create `cmd/marketplace/bench.go`:

```go
package main

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/aleogr/marketplace/internal/identity"
)

// benchBudget is how long one password verification may take on the service's
// CPU. A sign-in costs one; 250 ms is felt by nobody and costs an attacker
// with the database 250 ms per guess per core.
const benchBudget = 250 * time.Millisecond

type benchResult struct {
	Params identity.Params
	Median time.Duration
}

// benchPassword measures argon2id on this machine, for the candidates OWASP
// lists as equivalent, and logs the choice (spec, D5). It is run once, as the
// lab's migration job with this argument, because that job runs on the same
// CPU as the service (docs/infrastructure.md). The chosen line becomes
// identity.Current.
func benchPassword(ctx context.Context, log *slog.Logger) error {
	candidates := []identity.Params{
		{Memory: 19456, Time: 2, Threads: 1, KeyLen: 32, SaltLen: 16},
		{Memory: 47104, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16},
		{Memory: 47104, Time: 2, Threads: 1, KeyLen: 32, SaltLen: 16},
		{Memory: 65536, Time: 2, Threads: 1, KeyLen: 32, SaltLen: 16},
		{Memory: 65536, Time: 3, Threads: 1, KeyLen: 32, SaltLen: 16},
	}
	var results []benchResult
	for _, p := range candidates {
		h := identity.NewHasher(p, 1)
		var runs []time.Duration
		for range 9 {
			start := time.Now()
			if _, err := h.Hash(ctx, "benchmark passphrase of ordinary length"); err != nil {
				return err
			}
			runs = append(runs, time.Since(start))
		}
		sort.Slice(runs, func(i, j int) bool { return runs[i] < runs[j] })
		r := benchResult{Params: p, Median: runs[len(runs)/2]}
		results = append(results, r)
		log.InfoContext(ctx, "argon2id", "memory_kib", p.Memory, "time", p.Time, "median_ms", r.Median.Milliseconds())
	}
	chosen := choose(results, benchBudget)
	log.InfoContext(ctx, "argon2id chosen", "memory_kib", chosen.Memory, "time", chosen.Time,
		"budget_ms", benchBudget.Milliseconds())
	return nil
}

func choose(results []benchResult, budget time.Duration) identity.Params {
	best := identity.Floor
	var bestCost uint64
	for _, r := range results {
		cost := uint64(r.Params.Memory) * uint64(r.Params.Time)
		if r.Median <= budget && cost > bestCost {
			best, bestCost = r.Params, cost
		}
	}
	return best
}
```

In `cmd/marketplace/main.go`, replace:

```go
	case "send-probe":
		return sendProbe(ctx, cfg, log, args[1:])
```

with:

```go
	case "send-probe":
		return sendProbe(ctx, cfg, log, args[1:])
	case "bench-password":
		return benchPassword(ctx, log)
```

In `docs/infrastructure.md`, under the owner's manual steps, add "Measure argon2id on the service's CPU": the migration job runs on the service's CPU (1 vCPU, 512 MiB, `infra/terraform/migrate_job.tf`), so the benchmark is that job executed once with the argument `bench-password`; the arguments are overridden only for that execution and the job's Terraform definition is untouched. Give the two commands of Task 14, Step 1.

- [ ] **Step 4: Run** — `go test ./cmd/marketplace/ -race -v && make build && ./bin/marketplace bench-password` → PASS, and six log lines (five candidates, one choice; the numbers are this machine's, not the service's).

- [ ] **Step 5: Run everything, commit, push, open PR 3, subscribe**

```bash
make generate && make check && make test && make integration && make e2e
git add -A cmd docs
git commit -m "Add the argon2id benchmark, to run on the service's own CPU"
git push -u origin claude/funny-wright-379asb-f13sessions
```

## Task 12b: Sessions that can never be used again are removed

Added to the plan on 2026-09-23 by the owner's decision, in PR 3: `session` keeps `ip` and
`user_agent` in clear, so a row that can no longer authenticate is a needless copy of that data
(LGPD's necessity principle, docs/requirements.md §18.3). The audit log already keeps the sealed
`identity.signin`/`identity.signout` records, which are the access record; the session row is only
needed while it can still be used. A row is deleted once it was created more than `SessionLifetime`
ago, last seen more than `SessionIdle` ago, or revoked more than `RevokedKept` (a new constant, 7
days — long enough for a sessions screen or a support question about a sign-out that just happened).
`SignIn` triggers the sweep after the new session is written, at most once an hour per process (an
in-memory timestamp on `Service`, guarded for concurrent sign-ins), in its own transaction as the
application role for the signing-in marketplace, so a sweep failure never rolls the sign-in back and
row-level security scopes it to that marketplace. `migrations/00012_session_sweep_index.sql` adds an
index on `session.marketplace_id`, which the table had none of: without it the sweep's `DELETE`
would scan every marketplace's rows, not just the signing-in one's.

**Files:**
- Create: `migrations/00012_session_sweep_index.sql`
- Modify: `internal/identity/store.go` (`sweepSessions`), `internal/identity/identity.go`
  (`RevokedKept`, the once-an-hour trigger in `SignIn`)
- Test: `internal/identity/session_integration_test.go`

---

# PR 4 — Password change

PR 4 starts after PR 3 has merged and deployed, and after the owner's benchmark run (Task 14, Step 1); Tasks 13 and 15 do not need the run and may be written before it.

## Task 13: Change the password and end the other sessions

**Files:**
- Modify: `internal/identity/store.go`, `internal/identity/identity.go`, `internal/platform/httpx/identity.go`, `web/identity.templ`, `web/layout.templ` (the password link), `web/locales/*.json`
- Create: `web/mail/password-changed.{en-US,pt-BR}.{txt,html}`
- Test: `internal/identity/password_integration_test.go`

**Interfaces:**
- Produces: `func (s *Service) ChangePassword(ctx context.Context, v Visit, session Session, current, next string) error` — `ErrCredentials` for a wrong current password (or a password changed meanwhile), and the D4 errors for the new one; templ `Password(page Page, form Form, done bool)`; route `GET/POST /account/password` (redirects to `/signin` when signed out), limited per account.

ChangePassword checks the new password (D4, including the breach check, a network call) with no transaction open, reads the current hash in one transaction, verifies the current password and hashes the new one with none open, and writes in a second that first confirms the hash is still the one verified. The audit record is found by its action, not by its position.

- [ ] **Step 1: Write the failing test**

Create `internal/identity/password_integration_test.go`:

```go
//go:build integration

package identity

import (
	"errors"
	"slices"
	"testing"
)

func TestChangingThePasswordEndsEveryOtherSession(t *testing.T) {
	s, db, one, _, trail := service(t)
	confirmed(t, s, db, one, "r@example.test")
	here := signIn(t, s, one, "r@example.test", "correct horse battery staple")
	there := signIn(t, s, one, "r@example.test", "correct horse battery staple")
	session, err := s.Authenticate(t.Context(), one, here)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.ChangePassword(t.Context(), visit(one), session, "wrong current password", "a brand new passphrase"); !errors.Is(err, ErrCredentials) {
		t.Fatalf("wrong current: %v, want ErrCredentials", err)
	}
	if err := s.ChangePassword(t.Context(), visit(one), session, "correct horse battery staple", "password1234"); !errors.Is(err, ErrPasswordBreached) {
		t.Fatalf("breached new: %v, want ErrPasswordBreached", err)
	}
	if slices.Contains(trail.actions(), "identity.password_changed") {
		t.Fatalf("a refused change was audited as done: %v", trail.actions())
	}

	if err := s.ChangePassword(t.Context(), visit(one), session, "correct horse battery staple", "a brand new passphrase"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(t.Context(), one, here); err != nil {
		t.Fatalf("the session that changed the password ended: %v", err)
	}
	if _, err := s.Authenticate(t.Context(), one, there); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("the other session survived: %v", err)
	}
	if _, err := s.SignIn(t.Context(), visit(one), "r@example.test", "correct horse battery staple"); !errors.Is(err, ErrCredentials) {
		t.Fatalf("the old password still signs in: %v", err)
	}
	signIn(t, s, one, "r@example.test", "a brand new passphrase")

	mails := outbox(t, db, one)
	if mails[len(mails)-1].template != "password-changed" {
		t.Fatalf("no password-changed mail: %v", mails)
	}
	if !slices.Contains(trail.actions(), "identity.password_changed") {
		t.Fatalf("the change was not audited: %v", trail.actions())
	}
}
```

- [ ] **Step 2: Run to see it fail** — `go test -tags=integration ./internal/identity/ -run Changing -v` → FAIL (undefined `ChangePassword`).

- [ ] **Step 3: Implement**

Append to `internal/identity/store.go`:

```go
func revokeOtherSessions(ctx context.Context, tx pgx.Tx, account, keep string, now time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE session SET revoked_at = $3, revoked_reason = 'password_changed'
		 WHERE account_id = $1 AND id <> $2 AND revoked_at IS NULL`, account, keep, now)
	return err
}
```

Append to `internal/identity/identity.go`:

```go
// ChangePassword replaces the password and ends every other session of the
// account (docs/requirements.md, section 18.1), keeping the one that asked.
//
// As in SignIn, argon2 runs in no transaction: the current hash is read in
// one, both passwords are verified and hashed with none open, and the change
// is written in a second, which first checks that nobody changed the password
// in between.
func (s *Service) ChangePassword(ctx context.Context, v Visit, session Session, current, next string) error {
	if err := s.checkNew(ctx, next); err != nil {
		return err
	}
	var secret string
	if err := s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		var err error
		secret, err = passwordOf(ctx, tx, session.Account.ID)
		return err
	}); err != nil {
		return err
	}
	ok, _, err := s.hasher.Verify(ctx, current, secret)
	if err != nil {
		return err
	}
	if !ok {
		return ErrCredentials
	}
	fresh, err := s.hasher.Hash(ctx, next)
	if err != nil {
		return err
	}
	return s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		still, err := passwordStill(ctx, tx, session.Account.ID, secret)
		if err != nil {
			return err
		}
		if !still {
			return ErrCredentials
		}
		if err := setPassword(ctx, tx, v.Marketplace, session.Account.ID, fresh); err != nil {
			return err
		}
		if err := revokeOtherSessions(ctx, tx, session.Account.ID, session.ID, s.now()); err != nil {
			return err
		}
		if err := s.record(ctx, tx, v, session.Account.ID, "identity.password_changed"); err != nil {
			return err
		}
		return mail.Request(ctx, tx, mail.Message{
			Template: "password-changed", Language: v.Language, To: session.Account.Email,
			From: v.MarketplaceName, Marketplace: v.Marketplace,
			Variables: map[string]string{"Name": session.Account.Name},
		})
	})
}
```

The mail — `password-changed`, variable `Name`, the marketplace as `{{.From}}`:

Create `web/mail/password-changed.en-US.txt`:

```text
Subject: Your password at {{.From}} was changed
Hello, {{.Name}}.

The password of your account at {{.From}} was just changed, and every other session was signed out.

If it was not you, recover your account now and contact us.
```

Create `web/mail/password-changed.pt-BR.txt`:

```text
Subject: Sua senha em {{.From}} foi alterada
Olá, {{.Name}}.

A senha da sua conta em {{.From}} acabou de ser alterada, e todas as outras sessões foram encerradas.

Se não foi você, recupere sua conta agora e fale conosco.
```

Create `web/mail/password-changed.en-US.html`:

```html
<p>Hello, {{.Name}}.</p>
<p>The password of your account at {{.From}} was just changed, and every other session was signed out.</p>
<p>If it was not you, recover your account now and contact us.</p>
```

Create `web/mail/password-changed.pt-BR.html`:

```html
<p>Olá, {{.Name}}.</p>
<p>A senha da sua conta em {{.From}} acabou de ser alterada, e todas as outras sessões foram encerradas.</p>
<p>Se não foi você, recupere sua conta agora e fale conosco.</p>
```

Add to `web/locales/en-US.json` and `web/locales/pt-BR.json` (the same keys in both, at the end of each file):

| key | en-US | pt-BR |
|---|---|---|
| `identity.password.title` | Change password | Alterar senha |
| `identity.field.current_password` | Current password | Senha atual |
| `identity.field.new_password` | New password | Nova senha |
| `identity.password.submit` | Change password | Alterar senha |
| `identity.password.wrong_current` | The current password is not right. | A senha atual não confere. |
| `identity.password.done` | Your password was changed. Every other session was signed out. | Sua senha foi alterada. Todas as outras sessões foram encerradas. |
| `identity.header.password` | Password | Senha |

Routes and handlers:

In `internal/platform/httpx/identity.go`, replace:

```go
	mux.HandleFunc("POST /signout", s.signOut)
}
```

with:

```go
	mux.HandleFunc("POST /signout", s.signOut)
	mux.HandleFunc("GET /account/password", s.passwordForm)
	mux.Handle("POST /account/password", limit(id.Limits.Password, byAccount, http.HandlerFunc(s.changePassword)))
}

// byAccount limits by the signed-in account. A request with no session is
// sent to sign in by the handler, and counts under one shared subject.
func byAccount(r *http.Request) string {
	if session, ok := SessionFrom(r.Context()); ok {
		return session.Account.ID
	}
	return "anonymous"
}
```

Append to `internal/platform/httpx/identity.go`:

```go
func (s Site) passwordForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := SessionFrom(r.Context()); !ok {
		http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+"/signin", http.StatusSeeOther)
		return
	}
	render(w, r, web.Password(s.page(r, "/account/password"), newForm(), false))
}

func (s Site) changePassword(w http.ResponseWriter, r *http.Request) {
	session, ok := SessionFrom(r.Context())
	if !ok {
		http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+"/signin", http.StatusSeeOther)
		return
	}
	form := newForm()
	err := s.identity.Service.ChangePassword(r.Context(), visit(r), session,
		r.PostFormValue("current_password"), r.PostFormValue("new_password"))
	switch key, args, shown := formError(err); {
	case errors.Is(err, identity.ErrCredentials):
		form.Error = "identity.password.wrong_current"
	case shown:
		form.Error, form.ErrorArgs = key, args
	case err != nil:
		s.identity.Log.ErrorContext(r.Context(), "password change failed", "error", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	default:
		render(w, r, web.Password(s.page(r, "/account/password"), newForm(), true))
		return
	}
	renderStatus(w, r, http.StatusUnprocessableEntity, web.Password(s.page(r, "/account/password"), form, false))
}
```

The page:

Append to `web/identity.templ`:

```templ
templ Password(page Page, form Form, done bool) {
	@Layout(page, page.T("identity.password.title")) {
		<h1>{ page.T("identity.password.title") }</h1>
		if done {
			<p role="status">{ page.T("identity.password.done") }</p>
		}
		if form.Error != "" {
			<p role="alert">{ page.T(form.Error, form.ErrorArgs...) }</p>
		}
		<form method="post" action={ templ.SafeURL("/" + page.Language + "/account/password") }>
			<input type="hidden" name="csrf_token" value={ page.CSRFToken }/>
			<label>
				{ page.T("identity.field.current_password") }
				<input name="current_password" type="password" autocomplete="current-password" required/>
			</label>
			<label>
				{ page.T("identity.field.new_password") }
				<input
					name="new_password"
					type="password"
					autocomplete="new-password"
					required
					minlength={ strconv.Itoa(form.MinLength) }
					maxlength={ strconv.Itoa(form.MaxLength) }
				/>
			</label>
			<p>{ page.T("identity.field.password_hint", form.MinLength) }</p>
			<button type="submit">{ page.T("identity.password.submit") }</button>
		</form>
	}
}
```

And the link in the signed-in header:

In `web/layout.templ`, replace:

```templ
				<button type="submit">{ page.T("identity.signout.submit") }</button>
			</form>
		} else {
```

with:

```templ
				<button type="submit">{ page.T("identity.signout.submit") }</button>
			</form>
			<a href={ templ.SafeURL("/" + page.Language + "/account/password") }>{ page.T("identity.header.password") }</a>
		} else {
```

- [ ] **Step 4: Run** — `make generate && go test -tags=integration -count=1 ./internal/identity/ -race -v && go test ./internal/platform/... ./web/ -race && make check` → PASS; `0 issues`.

- [ ] **Step 5: Commit** — `git add -A internal web && git commit -m "Change the password and sign every other session out"`

## Task 14: The final argon2id parameters, from the owner's run

**Files:**
- Modify: `internal/identity/password.go` (`Current`), `docs/infrastructure.md` (the run's numbers)

- [ ] **Step 1: The owner's run** — after PR 3 is merged and deployed, the owner runs in Cloud Shell (the job's name and region are in `infra/terraform/migrate_job.tf`):

```sh
gcloud run jobs execute marketplace-migrate --region=us-central1 \
  --project=aleogr-marketplace-lab-a4j5 --args=bench-password --wait
gcloud logging read 'resource.type="cloud_run_job" AND resource.labels.job_name="marketplace-migrate" AND jsonPayload.message=~"^argon2id"' \
  --project=aleogr-marketplace-lab-a4j5 --limit=10 --freshness=1h --format='value(jsonPayload)'
```

and reports the six lines. The one whose message is `argon2id chosen` carries `memory_kib` and `time`; its candidate's line carries `median_ms`.

- [ ] **Step 2: Set `Current`** — with those three numbers and the date of the run (the values are the measurement; everything else is as written):

In `internal/identity/password.go`, replace:

```go
// Current is what new hashes are made with. Until the benchmark has run on the
// service's CPU it is the floor; the measurement that replaces it is recorded
// here, beside the value (spec, D5).
var Current = Floor
```

with:

```go
// Current is what new hashes are made with: what `bench-password` chose on the
// lab's Cloud Run CPU (1 vCPU, 512 MiB) on RUN_DATE, MEDIAN_MS ms per hash
// against a 250 ms budget (cmd/marketplace/bench.go, docs/infrastructure.md).
// A hash made with other parameters is remade at its owner's next sign-in.
var Current = Params{Memory: MEMORY_KIB, Time: TIME, Threads: 1, KeyLen: 32, SaltLen: 16}
```

`TestCurrentIsNoWeakerThanTheFloor` (Task 3) refuses a value weaker than the floor, and `hashSlots` (Task 8) derives the number of concurrent hashes from `Current.Memory`, so nothing else changes. In `docs/infrastructure.md`, under the step Task 12 added, record the six lines, the date and the chosen parameters.

- [ ] **Step 3: Run** — `go test ./internal/identity/ ./cmd/marketplace/ -race -v && make check` → PASS; `0 issues`.

- [ ] **Step 4: Commit** — `git add internal/identity/password.go docs/infrastructure.md && git commit -m "Hash with the argon2id parameters measured on the service's own CPU"`

## Task 15: The two-browser end-to-end test, the screenshots, and closing F13

**Files:**
- Create: `e2e/test_password.py`
- Modify: `docs/roadmap.md` (tick F13's objective), `docs/superpowers/specs/2026-09-23-f13-identity-core-design.md` (status: implemented)

- [ ] **Step 1: The test the roadmap names**

Create `e2e/test_password.py`:

```python
"""The roadmap's F13 verification: two browsers, one password change."""

from __future__ import annotations

import pytest
from playwright.sync_api import expect, sync_playwright

from accounts import PASSWORD, SIGN_IN_LINK, SIGNED_IN, SUBMIT, confirmed_account, sign_in, wait_for_mail


@pytest.mark.local_process
@pytest.mark.parametrize("language", ["pt-BR", "en-US"])
def test_changing_the_password_signs_the_other_browser_out(run_marketplace, screenshots, language):
    marketplace = run_marketplace()
    email = f"duas-abas-{language}@example.test"
    with sync_playwright() as p:
        browser = p.chromium.launch()
        first = browser.new_context().new_page()
        second = browser.new_context().new_page()

        confirmed_account(first, marketplace, email, language=language)
        sign_in(first, marketplace, email, language=language)
        sign_in(second, marketplace, email, language=language)
        expect(first.locator(SIGNED_IN)).to_be_visible()
        expect(second.locator(SIGNED_IN)).to_be_visible()

        first.goto(marketplace.url("m1.localhost", f"/{language}/account/password"))
        first.screenshot(path=screenshots / f"f13-password-{language}.png")
        first.fill("input[name=current_password]", PASSWORD)
        first.fill("input[name=new_password]", "a brand new passphrase")
        first.click(SUBMIT)
        expect(first.locator("[role=status]")).to_be_visible()
        first.screenshot(path=screenshots / f"f13-password-done-{language}.png")

        second.reload()
        expect(second.locator(SIGN_IN_LINK)).to_be_visible()
        expect(second.locator(SIGNED_IN)).to_have_count(0)
        expect(first.locator(SIGNED_IN)).to_be_visible()

        wait_for_mail(marketplace.mailbox, "password-changed", email)
        browser.close()
```

- [ ] **Step 2: Run everything** — `make generate && make check && make test && make integration && make e2e` → all green. The screenshots for F13 are now signup, check-email, verify, verify-failed, resend, resend-sent, signin, signed-in, password and password-done, each in pt-BR and en-US: every screen of the delivery.

- [ ] **Step 3: Close the delivery in the documents** — tick `- [x] **Objective:**` of F13 in `docs/roadmap.md`; set the spec's status to `implemented on <date>` with the pull request numbers.

- [ ] **Step 4: Commit, push, open PR 4, subscribe**

```bash
git add -A e2e docs
git commit -m "Prove a password change signs every other browser out, and close F13"
git push -u origin claude/funny-wright-379asb-f13password
```

The pull request description lists the roadmap's F13 verification items one by one, each with its evidence (test names, screenshots artifact), per `docs/roadmap.md` section 3.

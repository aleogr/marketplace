# F13 — Identity core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A person creates an account in a marketplace, confirms the address, signs in, signs out and changes the password, and a password change ends every other session of that account.

**Architecture:** A new package `internal/identity` holds the rules (password hashing, tokens, the `breached` port, the account and session store, and a `Service` with one method per flow). `internal/platform/httpx` holds everything HTTP: the handlers, a session middleware mounted between language resolution and the mux, and the rate limits per route. Four pull requests, each green on its own: the end-to-end suite gains a database and local mail delivery; accounts; sessions; password change.

**Tech Stack:** Go 1.25 (stdlib `net/http` mux), pgx v5, goose migrations, PostgreSQL 16 with row-level security, templ, `golang.org/x/crypto/argon2`, `golang.org/x/text/unicode/norm`, Python pytest + Playwright for end-to-end.

**Spec:** `docs/superpowers/specs/2026-09-23-f13-identity-core-design.md`. Read it first; this plan argues from its decisions D1–D7.

## Global Constraints

- Everything versioned is written in **English**; the owner is addressed in Portuguese.
- The repository is **public**: no secret is committed. Model identifiers never appear in code, docs, titles or comments.
- Every user-facing string exists in **en-US and pt-BR** (`web/locales/*.json`); no literal text in a template (`web/literals_test.go`); every mail template exists in both languages as `.txt` and `.html` (`internal/platform/mail/templates.go` refuses to load otherwise).
- Tests use **fake providers**, never real services. Integration tests run as the **application role under RLS** (`dbtest.Serving`), never as the owner.
- Passwords: **12 to 128 characters** counted in Unicode code points after NFKC normalisation; **no composition rules**; refused when found in Pwned Passwords; **accepted and logged when the service cannot be reached**.
- argon2id, PHC string stored; provisional parameters **m=19456 KiB, t=2, p=1**, 16-byte salt, 32-byte key; final parameters from the Task 12 benchmark.
- Tokens: **32 random bytes**, base64url, stored only as **SHA-256**.
- Sessions: **30 days idle, 90 days absolute**; `last_seen_at` written at most **once an hour**; cookie `__Host-session` over HTTPS and `session` over plain HTTP, `HttpOnly`, `SameSite=Lax`, path `/`, no domain.
- E-mail verification tokens expire after **24 hours** and are single-use.
- The account table is **`account`** (never `user`).
- Checks before every push: `make check`, `make test`, `make integration`, `make e2e` (CLAUDE.md).
- Claude Code opens pull requests and never merges. Each pull request is watched until merged.

## Pull requests and branches

| # | branch (from `main`) | tasks | depends on |
|---|---|---|---|
| A | `claude/funny-wright-379asb-f13` | the spec and this plan | — |
| 1 | `claude/funny-wright-379asb-f13e2e` | 1–2 | A |
| 2 | `claude/funny-wright-379asb-f13accounts` | 3–7 | 1 |
| 3 | `claude/funny-wright-379asb-f13sessions` | 8–10 | 2 |
| 4 | `claude/funny-wright-379asb-f13password` | 11–13 | 3 |

## File Structure

| file | responsibility | PR |
|---|---|---|
| `cmd/marketplace/main.go` (modify) | local outbox dispatch in fake mode; the ping-before-listen note; wiring of identity, sessions, `bench-password` | 1, 2, 3, 4 |
| `cmd/marketplace/dispatch.go` (create) | the in-process dispatch loop | 1 |
| `cmd/marketplace/dispatch_test.go` (create) | the loop runs, and stops with its context | 1 |
| `e2e/conftest.py` (modify) | a PostgreSQL for the suite, migrated and seeded; `run_marketplace` fixture | 1 |
| `e2e/database.py` (create) | start or reuse PostgreSQL, create a database and the application role | 1 |
| `e2e/test_database.py` (create) | two seeded marketplaces answer on their hosts | 1 |
| `.github/workflows/ci.yml` (modify) | a PostgreSQL service for the e2e job | 1 |
| `internal/identity/password.go` (create) | argon2id hashing with a concurrency bound | 2 |
| `internal/identity/policy.go` (create) | password length rule, e-mail normalisation, errors | 2 |
| `internal/identity/token.go` (create) | random tokens and their hashes | 2 |
| `internal/identity/breached/breached.go` (create) | the port, the Pwned Passwords adapter, the fake | 2 |
| `internal/identity/store.go` (create) | SQL for accounts, credentials, verifications (sessions in PR 3) | 2, 3 |
| `internal/identity/identity.go` (create) | `Service`, `Visit`, sign-up, verification, resend (sign-in etc. in 3, 4) | 2, 3, 4 |
| `internal/identity/*_test.go` | unit and integration tests | 2, 3, 4 |
| `migrations/00009_accounts.sql` (create) | `account`, `credential`, `email_verification` under RLS | 2 |
| `migrations/00010_sessions.sql` (create) | `session` under RLS | 3 |
| `internal/platform/httpx/identity.go` (create) | the identity handlers and their rate limits | 2, 3, 4 |
| `internal/platform/httpx/session.go` (create) | the session middleware and cookie | 3 |
| `internal/platform/httpx/csrf.go` (modify) | `RenewCSRF` for sign-in | 3 |
| `internal/platform/httpx/site.go` (modify) | `WithIdentity`, routes, `page()` carries the signed-in name | 2, 3 |
| `web/identity.templ` (create) | the pages | 2, 3, 4 |
| `web/layout.templ`, `web/page.go` (modify) | the header: sign in / sign up, or name and sign out | 3 |
| `web/locales/en-US.json`, `pt-BR.json` (modify) | every string | 2, 3, 4 |
| `web/mail/verify-email.*`, `account-exists.*`, `password-changed.*` (create) | the three mails, both languages, text and HTML | 2, 4 |
| `e2e/test_signup.py`, `e2e/test_signin.py`, `e2e/test_password.py` (create) | the flows and the screenshots | 2, 3, 4 |
| `docs/design.md`, `docs/infrastructure.md`, `docs/roadmap.md` (modify) | the `breached` port; the benchmark step; F13 ticked | 2, 4 |

---

# PR 1 — A database and local mail in the end-to-end suite

## Task 1: Deliver the outbox inside the process when providers are fake

**Files:**
- Create: `cmd/marketplace/dispatch.go`
- Create: `cmd/marketplace/dispatch_test.go`
- Modify: `cmd/marketplace/main.go` (`work`, and the comment at the database ping in `serve`)

**Interfaces:**
- Produces: `dispatchLocally(ctx context.Context, dispatch func(context.Context) (int, error), every time.Duration, log *slog.Logger)` — returns when `ctx` is done.

- [ ] **Step 1: Write the failing test**

```go
// cmd/marketplace/dispatch_test.go
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

```go
// cmd/marketplace/dispatch.go
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

In `work` (`cmd/marketplace/main.go`), after `dispatcher := outbox.NewDispatcher(database, queuer, log)`:

```go
	// No scheduler runs locally, so a process with fake providers dispatches
	// its own outbox (cmd/marketplace/dispatch.go).
	if cfg.ProvidersMode == config.ProvidersFake && !cfg.Tasks.Configured() {
		go dispatchLocally(ctx, dispatcher.Dispatch, localDispatchEvery, log)
		log.InfoContext(ctx, "the outbox is dispatched in this process", "every", localDispatchEvery.String())
	}
```

In `serve`, extend the comment above `if err := pool.Ping(ctx); err != nil {` with one paragraph:

```go
		// Re-examined on 2026-09-23, when the shared lab instance began to
		// sleep four nights a week: a process started while it sleeps exits
		// here and Cloud Run answers 503 until it wakes. That is kept on
		// purpose — refusing to start is what keeps a revision with a broken
		// database setting from ever taking traffic (docs/infrastructure.md).
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
- Create: `e2e/database.py`
- Modify: `e2e/conftest.py`
- Create: `e2e/test_database.py`
- Modify: `.github/workflows/ci.yml` (the `e2e` job)

**Interfaces:**
- Produces (pytest fixtures): `database` (session scope) → `Database(owner_url: str, app_url: str)`; `run_marketplace(**env) -> Marketplace` where `Marketplace` has `base_url` (`http://127.0.0.1:PORT`), `url(host, path)` returning `http://{host}:{PORT}{path}`, `mailbox` (`Path`), `process`, `log_lines`. Hosts are `m1.localhost` (slug `m1`, name "Loja Um") and `m2.localhost` (slug `m2`, name "Loja Dois"), both `active`, market `BR`, default `pt-BR`, languages `pt-BR,en-US`.

- [ ] **Step 1: Write the failing test**

```python
# e2e/test_database.py
"""The suite's marketplaces, served from a real database.

Until F13 every end-to-end run had no database, so no host resolved and no
flow that writes could be tested. These are the two marketplaces every
identity test signs up in (docs/superpowers/plans/2026-09-23-f13-identity-core.md).
"""

from __future__ import annotations

import pytest
import requests


@pytest.mark.local_process
def test_each_seeded_host_serves_its_own_marketplace(run_marketplace):
    marketplace = run_marketplace()

    first = requests.get(marketplace.url("m1.localhost", "/pt-BR/"), timeout=10)
    second = requests.get(marketplace.url("m2.localhost", "/pt-BR/"), timeout=10)

    assert first.status_code == 200 and "Loja Um" in first.text
    assert second.status_code == 200 and "Loja Dois" in second.text


@pytest.mark.local_process
def test_the_health_check_reports_the_database(run_marketplace):
    marketplace = run_marketplace()
    body = requests.get(marketplace.base_url + "/health", timeout=10).json()
    assert body["database"] == "ok"
```

`requests` resolves `*.localhost` through the system resolver, which may not; the fixture's `url()` therefore returns the loopback address and the tests send the host explicitly. Replace the two `requests.get` lines with:

```python
    first = requests.get(marketplace.base_url + "/pt-BR/", headers={"Host": "m1.localhost"}, timeout=10)
    second = requests.get(marketplace.base_url + "/pt-BR/", headers={"Host": "m2.localhost"}, timeout=10)
```

(Playwright's Chromium resolves `*.localhost` to the loopback itself, so browser tests use `marketplace.url(...)`.)

- [ ] **Step 2: Run it to see it fail**

Run: `make build && MARKETPLACE_BINARY=$PWD/bin/marketplace python3 -m pytest e2e/test_database.py -v`
Expected: ERROR, `fixture 'run_marketplace' not found`.

- [ ] **Step 3: Implement `e2e/database.py`**

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

Add `psycopg[binary]==3.2.10` to `e2e/requirements.txt` (pinned, like the rest of the file).

- [ ] **Step 4: Implement the fixtures in `e2e/conftest.py`** (append)

```python
SEED = json.dumps([
    {"slug": "m1", "name": "Loja Um", "market": "BR", "revenue_model": "commission",
     "state": "active", "default_language": "pt-BR", "languages": ["pt-BR", "en-US"],
     "hosts": ["m1.localhost"]},
    {"slug": "m2", "name": "Loja Dois", "market": "BR", "revenue_model": "commission",
     "state": "active", "default_language": "pt-BR", "languages": ["pt-BR", "en-US"],
     "hosts": ["m2.localhost"]},
])


@pytest.fixture(scope="session")
def database(binary: Path):
    """A PostgreSQL, migrated and seeded once for the whole session."""
    from database import APP_ROLE, provision

    db, remove = provision()
    migrate = subprocess.run(
        [str(binary), "migrate"], cwd=REPO_ROOT, capture_output=True, text=True, timeout=120,
        env={"PATH": os.environ.get("PATH", ""), "PROVIDERS_MODE": "fake",
             "DATABASE_URL": db.owner_url, "DATABASE_APP_USER": APP_ROLE,
             "SEED_MARKETPLACES": SEED},
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
    """Start the binary against the suite's database, as the application role."""

    def factory(**env: str) -> Marketplace:
        mailbox = tmp_path / "mailbox"
        server = run_server(DATABASE_URL=database.app_url, MAIL_DIRECTORY=str(mailbox), **env)
        port = int(server.base_url.rsplit(":", 1)[1])
        return Marketplace(base_url=server.base_url, process=server.process,
                           log_lines=server.log_lines, port=port, mailbox=mailbox)

    return factory
```

Make the fixture module importable: add `import sys; sys.path.insert(0, str(Path(__file__).parent))` at the top of `conftest.py`, after the existing imports.

- [ ] **Step 5: Give the e2e job a PostgreSQL** — in `.github/workflows/ci.yml`, job `e2e`, add the same `services.postgres` block the `integration` job has, and to the `Run the suite` step:

```yaml
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
  - `func NormalizeEmail(address string) (string, error)`
  - `type Params struct{ Memory, Time uint32; Threads uint8; KeyLen, SaltLen uint32 }`, `var Provisional = Params{Memory: 19456, Time: 2, Threads: 1, KeyLen: 32, SaltLen: 16}`
  - `func NewHasher(params Params, concurrent int) *Hasher`
  - `func (h *Hasher) Hash(ctx context.Context, password string) (string, error)`
  - `func (h *Hasher) Verify(ctx context.Context, password, encoded string) (ok, stale bool, err error)`
  - `func (h *Hasher) Waste(ctx context.Context, password string)` — the dummy verification for unknown addresses
  - `func NewToken() (token string, hash []byte, err error)`, `func HashToken(token string) []byte`

- [ ] **Step 1: Write the failing tests**

```go
// internal/identity/policy_test.go
package identity

import (
	"errors"
	"strings"
	"testing"
)

func TestCheckPasswordCountsCharactersNotBytes(t *testing.T) {
	cases := map[string]error{
		strings.Repeat("a", 11):  ErrPasswordShort,
		strings.Repeat("a", 12):  nil,
		strings.Repeat("ç", 12):  nil, // 24 bytes, 12 characters
		strings.Repeat("a", 128): nil,
		strings.Repeat("a", 129): ErrPasswordLong,
		"café com leite na padaria": nil, // spaces allowed, no composition rule
	}
	for password, want := range cases {
		if got := CheckPassword(password); !errors.Is(got, want) {
			t.Errorf("CheckPassword(%q) = %v, want %v", password, got, want)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	good := map[string]string{
		" Reader@Example.Test ": "reader@example.test",
		"a.b+tag@example.test":  "a.b+tag@example.test",
	}
	for in, want := range good {
		got, err := NormalizeEmail(in)
		if err != nil || got != want {
			t.Errorf("NormalizeEmail(%q) = %q, %v; want %q, nil", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "no-at-sign", "Name <a@example.test>", "a@", "a b@example.test"} {
		if _, err := NormalizeEmail(bad); !errors.Is(err, ErrEmailInvalid) {
			t.Errorf("NormalizeEmail(%q) err = %v, want ErrEmailInvalid", bad, err)
		}
	}
}
```

```go
// internal/identity/password_test.go
package identity

import (
	"context"
	"strings"
	"testing"
)

// cheap keeps the unit tests fast; the real parameters are exercised by the
// benchmark (Task 12).
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
	if ok, _, _ := h.Verify(context.Background(), "wrong horse battery", encoded); ok {
		t.Fatal("Verify accepted a wrong password")
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
	composed := "senha com cedilha ç ok" // U+00E7
	decomposed := "senha com cedilha ç ok"
	encoded, _ := h.Hash(context.Background(), composed)
	if ok, _, _ := h.Verify(context.Background(), decomposed, encoded); !ok {
		t.Fatal("the same password typed on another keyboard did not verify")
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
```

```go
// internal/identity/token_test.go
package identity

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestTokensAreRandomAndHashedOneWay(t *testing.T) {
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
	if !bytes.Equal(hashA, HashToken(a)) || len(hashA) != 32 {
		t.Fatal("HashToken is not the SHA-256 NewToken returned")
	}
}
```

- [ ] **Step 2: Run to see them fail**

Run: `go test ./internal/identity/ -v`
Expected: FAIL to compile (undefined symbols).

- [ ] **Step 3: Implement**

```go
// internal/identity/policy.go
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
// minimum is the owner's choice of 2026-09-23 and becomes a parameter in F17.
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

// NormalizeEmail returns the form an address is compared in, or
// ErrEmailInvalid. Only a bare address is accepted: "Name <a@b>" is valid mail
// syntax and not something a person types into an e-mail field.
func NormalizeEmail(address string) (string, error) {
	address = strings.TrimSpace(address)
	parsed, err := mail.ParseAddress(address)
	if err != nil || parsed.Address != address || parsed.Name != "" {
		return "", ErrEmailInvalid
	}
	return strings.ToLower(address), nil
}
```

```go
// internal/identity/password.go
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

// Provisional is OWASP's floor for argon2id (19 MiB, two passes, one lane),
// used until the benchmark on the service's own CPU replaces it (spec, D5).
var Provisional = Params{Memory: 19456, Time: 2, Threads: 1, KeyLen: 32, SaltLen: 16}

var errNotPHC = errors.New("identity: not an argon2id PHC string")

// Hasher hashes and verifies passwords, a bounded number at a time.
//
// The bound is the instance's memory: 80 concurrent requests on 512 MiB cannot
// each hold tens of megabytes (spec, D5). A request beyond it waits, and gives
// up with its context.
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
func (h *Hasher) Verify(ctx context.Context, password, encoded string) (ok, stale bool, err error) {
	params, salt, key, err := decode(encoded)
	if err != nil {
		return false, false, err
	}
	if err := h.acquire(ctx); err != nil {
		return false, false, err
	}
	defer h.release()
	got := argon2.IDKey([]byte(normalise(password)), salt, params.Time, params.Memory, params.Threads, uint32(len(key)))
	params.KeyLen, params.SaltLen = uint32(len(key)), uint32(len(salt))
	return subtle.ConstantTimeCompare(got, key) == 1, params != h.params, nil
}

// Waste spends what a verification costs, so an unknown address takes as long
// as a wrong password (spec, D7).
func (h *Hasher) Waste(ctx context.Context, password string) {
	_, _, _ = h.Verify(ctx, password, h.dummy)
}

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
	if err != nil || len(key) == 0 {
		return Params{}, nil, nil, errNotPHC
	}
	return p, salt, key, nil
}
```

```go
// internal/identity/token.go
package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// NewToken returns a token to hand to a person once, and the hash that is the
// only thing stored (spec, D6). A lookup by SHA-256 of 32 random bytes gives a
// timing attack nothing to learn: the attacker would need the preimage.
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

Run `go get golang.org/x/crypto@v0.57.0` so it becomes a direct dependency (`go mod tidy`).

- [ ] **Step 4: Run to see them pass**

Run: `go test ./internal/identity/ -race -v`
Expected: PASS (all tests).

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
- Produces: `type Checker interface { Breached(ctx context.Context, password string) (bool, error) }`; `func NewPwned(client *http.Client, base string) *Pwned`; `const PwnedBase = "https://api.pwnedpasswords.com"`; `type Fake struct{ Known map[string]bool; Err error }`; `var Common = map[string]bool{"password1234": true, "123456789012": true, "senhasenha123": true}`.

- [ ] **Step 1: Write the failing test**

```go
// internal/identity/breached/breached_test.go
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

```go
// internal/identity/breached/breached.go
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

// PwnedBase is the service's address.
const PwnedBase = "https://api.pwnedpasswords.com"

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

// Fake is the adapter tests and a local run use.
type Fake struct {
	Known map[string]bool
	Err   error
}

// Breached implements Checker.
func (f Fake) Breached(_ context.Context, password string) (bool, error) {
	return f.Known[password], f.Err
}
```

- [ ] **Step 4: Run to see it pass** — `go test ./internal/identity/breached/ -race -v` → PASS. Then `make check` (gosec must accept the annotated SHA-1).

- [ ] **Step 5: Commit** — `git commit -m "Check new passwords against Pwned Passwords, behind a port"`

## Task 5: The accounts schema

**Files:**
- Create: `migrations/00009_accounts.sql`
- Test: `internal/identity/store_integration_test.go` (build tag `integration`)

- [ ] **Step 1: Write the migration**

```sql
-- migrations/00009_accounts.sql
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
    email_normalized text        NOT NULL,
    name             text        NOT NULL,
    verified_at      timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT account_kind_is_known CHECK (kind IN ('buyer', 'staff')),
    CONSTRAINT account_staff_has_no_marketplace CHECK ((kind = 'staff') = (marketplace_id IS NULL)),
    -- NULLS NOT DISTINCT, so two staff accounts cannot share an address either.
    CONSTRAINT account_email_is_unique UNIQUE NULLS NOT DISTINCT (marketplace_id, email_normalized)
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

- [ ] **Step 2: Write the isolation test** (it fails until Task 6 adds the store; write it now and run it after Step 3 of Task 6)

```go
//go:build integration

// internal/identity/store_integration_test.go
package identity

import (
	"context"
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

// twoMarketplaces migrates a fresh database and seeds two marketplaces.
func twoMarketplaces(t *testing.T) (serving, string, string) {
	t.Helper()
	pool, err := db.Open(t.Context(), config.Database{URL: dbtest.URL(t)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	dbtest.AsApplication(t, pool)

	specs := []tenancy.Spec{
		{Slug: "one", Name: "One", Market: "BR", RevenueModel: "commission", State: "active",
			DefaultLanguage: "pt-BR", Languages: []string{"pt-BR"}, Hosts: []string{"one.test"}},
		{Slug: "two", Name: "Two", Market: "BR", RevenueModel: "commission", State: "active",
			DefaultLanguage: "pt-BR", Languages: []string{"pt-BR"}, Hosts: []string{"two.test"}},
	}
	var ids []string
	if err := pool.InTx(t.Context(), func(tx pgx.Tx) error {
		if _, err := tenancy.Seed(t.Context(), tx, specs); err != nil {
			return err
		}
		return tx.QueryRow(t.Context(),
			"SELECT array_agg(id::text ORDER BY slug) FROM marketplace").Scan(&ids)
	}); err != nil {
		t.Fatal(err)
	}
	return serving{t: t, pool: pool}, ids[0], ids[1]
}

func TestAnAccountIsInvisibleFromAnotherMarketplace(t *testing.T) {
	db, one, two := twoMarketplaces(t)
	var id string
	if err := db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		var err error
		id, err = insertAccount(t.Context(), tx, one, "Reader@Example.Test", "reader@example.test", "Reader")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	_ = db.InTxFor(t.Context(), two, func(tx pgx.Tx) error {
		if _, err := accountByEmail(t.Context(), tx, "reader@example.test"); err == nil {
			t.Fatalf("account %s of one marketplace was found from the other", id)
		}
		return nil
	})
	// And the same address may open an account in the other marketplace.
	if err := db.InTxFor(t.Context(), two, func(tx pgx.Tx) error {
		_, err := insertAccount(t.Context(), tx, two, "reader@example.test", "reader@example.test", "Reader")
		return err
	}); err != nil {
		t.Fatalf("the same address could not open an account in a second marketplace: %v", err)
	}
}
```

Check the exact field names of `tenancy.Spec` and the return of `tenancy.Seed` against `internal/tenancy/seed.go` before running, and the `dbtest.AsApplication`/`dbtest.Serving` signatures against `internal/platform/dbtest/application.go`; adjust the helper, not the assertion.

- [ ] **Step 3: Commit** — together with Task 6 (the test needs the store).

## Task 6: The store and the sign-up, verification and resend flows

**Files:**
- Create: `internal/identity/store.go`, `internal/identity/identity.go`
- Test: `internal/identity/identity_integration_test.go` (tag `integration`)

**Interfaces:**
- Consumes: Task 3 (`Hasher`, `CheckPassword`, `NormalizeEmail`, `NewToken`, `HashToken`), Task 4 (`breached.Checker`), `mail.Request(ctx, tx, mail.Message)`, `(*audit.Log).Append(ctx, tx, audit.Entry)`.
- Produces:
  - `type Transactor interface { InTxFor(ctx context.Context, marketplaceID string, fn func(pgx.Tx) error) error }`
  - `type Auditor interface { Append(ctx context.Context, tx pgx.Tx, entry audit.Entry) error }`
  - `type Visit struct { Marketplace, MarketplaceName, Language, IP, UserAgent, BaseURL string }` — `BaseURL` is scheme and host, no trailing slash, no language
  - `func NewService(db Transactor, hasher *Hasher, checker breached.Checker, auditor Auditor, log *slog.Logger) *Service`
  - `func (s *Service) SignUp(ctx context.Context, v Visit, name, email, password string) error` — returns `ErrNameMissing`, `ErrEmailInvalid`, `ErrPasswordShort`, `ErrPasswordLong`, `ErrPasswordBreached`, or nil whether the address was new or not
  - `func (s *Service) Verify(ctx context.Context, v Visit, token string) error` — `ErrTokenInvalid` for unknown, used or expired
  - `func (s *Service) Resend(ctx context.Context, v Visit, email string) error` — nil unless the database fails
  - `var ErrTokenInvalid error`
  - store (unexported, used by PR 3): `insertAccount`, `accountByEmail`, `accountByID`, `setPassword`, `passwordOf`, `insertVerification`, `consumeVerification`; `type Account struct { ID, MarketplaceID, Email, Name string; VerifiedAt *time.Time }`
  - mail templates: `verify-email` (variables `Name`, `Link`, `Marketplace`), `account-exists` (variables `Name`, `SignIn`, `Marketplace`)

- [ ] **Step 1: Write the failing integration tests**

```go
//go:build integration

// internal/identity/identity_integration_test.go
package identity

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/aleogr/marketplace/internal/identity/breached"
)

// recording is an Auditor that remembers the actions it was given.
type recording struct{ actions []string }

func (r *recording) Append(_ context.Context, _ pgx.Tx, e auditEntry) error {
	r.actions = append(r.actions, e.Action)
	return nil
}

func service(t *testing.T) (*Service, serving, string, string, *recording) {
	db, one, two := twoMarketplaces(t)
	trail := &recording{}
	s := NewService(db, NewHasher(cheap, 2), breached.Fake{Known: breached.Common}, trail,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	return s, db, one, two, trail
}

func visit(marketplace string) Visit {
	return Visit{Marketplace: marketplace, MarketplaceName: "One", Language: "pt-BR",
		IP: "203.0.113.7", UserAgent: "test", BaseURL: "https://one.test"}
}

// outbox returns the templates and links requested for delivery.
func outbox(t *testing.T, db serving, marketplace string) []map[string]string {
	var got []map[string]string
	_ = db.InTxFor(t.Context(), marketplace, func(tx pgx.Tx) error {
		rows, err := tx.Query(t.Context(),
			`SELECT payload->>'Template', coalesce(payload->'Variables'->>'Link', '')
			   FROM outbox_event WHERE kind = 'email.send' ORDER BY created_at`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var template, link string
			_ = rows.Scan(&template, &link)
			got = append(got, map[string]string{"template": template, "link": link})
		}
		return rows.Err()
	})
	return got
}

func TestSignUpAndVerify(t *testing.T) {
	s, db, one, _, trail := service(t)
	if err := s.SignUp(t.Context(), visit(one), "Reader", "Reader@Example.Test", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	mails := outbox(t, db, one)
	if len(mails) != 1 || mails[0]["template"] != "verify-email" {
		t.Fatalf("outbox = %v, want one verify-email", mails)
	}
	token := mails[0]["link"][strings.Index(mails[0]["link"], "token=")+len("token="):]
	if !strings.HasPrefix(mails[0]["link"], "https://one.test/pt-BR/verify?token=") {
		t.Fatalf("link %q is not this marketplace's verification page", mails[0]["link"])
	}

	if err := s.Verify(t.Context(), visit(one), token); err != nil {
		t.Fatalf("Verify = %v", err)
	}
	if err := s.Verify(t.Context(), visit(one), token); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("second Verify = %v, want ErrTokenInvalid: a token is single-use", err)
	}
	if strings.Join(trail.actions, ",") != "identity.signup,identity.email_verified" {
		t.Fatalf("audited %v", trail.actions)
	}
}

func TestSignUpWithATakenAddressSendsAccountExistsAndSaysNothing(t *testing.T) {
	s, db, one, _, _ := service(t)
	_ = s.SignUp(t.Context(), visit(one), "Reader", "reader@example.test", "correct horse battery staple")
	if err := s.SignUp(t.Context(), visit(one), "Someone", "READER@example.test", "another long passphrase"); err != nil {
		t.Fatalf("second SignUp = %v, want nil: the answer must not reveal the account", err)
	}
	mails := outbox(t, db, one)
	if len(mails) != 2 || mails[1]["template"] != "account-exists" {
		t.Fatalf("outbox = %v, want verify-email then account-exists", mails)
	}
}

func TestSignUpRefusesABreachedOrShortPassword(t *testing.T) {
	s, _, one, _, _ := service(t)
	if err := s.SignUp(t.Context(), visit(one), "R", "a@example.test", "password1234"); !errors.Is(err, ErrPasswordBreached) {
		t.Fatalf("breached: %v", err)
	}
	if err := s.SignUp(t.Context(), visit(one), "R", "a@example.test", "short"); !errors.Is(err, ErrPasswordShort) {
		t.Fatalf("short: %v", err)
	}
}

func TestAnUnreachableBreachServiceDoesNotBlockSignUp(t *testing.T) {
	db, one, _ := twoMarketplaces(t)
	s := NewService(db, NewHasher(cheap, 1), breached.Fake{Err: errors.New("down")}, &recording{},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := s.SignUp(t.Context(), visit(one), "R", "r@example.test", "correct horse battery staple"); err != nil {
		t.Fatalf("SignUp = %v, want nil (fail open, spec D4)", err)
	}
}

func TestAnExpiredTokenIsRefused(t *testing.T) {
	s, db, one, _, _ := service(t)
	_ = s.SignUp(t.Context(), visit(one), "R", "r@example.test", "correct horse battery staple")
	link := outbox(t, db, one)[0]["link"]
	token := link[strings.Index(link, "token=")+len("token="):]
	_ = db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		_, err := tx.Exec(t.Context(), "UPDATE email_verification SET expires_at = now() - interval '1 second'")
		return err
	})
	if err := s.Verify(t.Context(), visit(one), token); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("Verify(expired) = %v, want ErrTokenInvalid", err)
	}
}
```

In `identity.go` define `type auditEntry = audit.Entry` so the recording double above compiles against the real `Auditor` signature. Verify the outbox payload's JSON field names (`Template`, `Variables`) against `internal/platform/mail/mailer.go` (`Request`) and `internal/platform/outbox/outbox.go` before running; adjust the query, not the assertion.

- [ ] **Step 2: Run to see them fail**

Run: `go test -tags=integration ./internal/identity/ -run 'SignUp|Verify|Expired|Invisible' -v`
Expected: FAIL to compile.

- [ ] **Step 3: Implement the store**

```go
// internal/identity/store.go
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

func insertAccount(ctx context.Context, tx pgx.Tx, marketplace, email, normalized, name string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO account (marketplace_id, kind, email, email_normalized, name)
		VALUES ($1, 'buyer', $2, $3, $4)
		ON CONFLICT ON CONSTRAINT account_email_is_unique DO NOTHING
		RETURNING id::text`, marketplace, email, normalized, name).Scan(&id)
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
func accountByEmail(ctx context.Context, tx pgx.Tx, normalized string) (Account, error) {
	return scanAccount(tx.QueryRow(ctx,
		`SELECT `+accountColumns+` FROM account WHERE email_normalized = $1`, normalized))
}

func accountByID(ctx context.Context, tx pgx.Tx, id string) (Account, error) {
	return scanAccount(tx.QueryRow(ctx, `SELECT `+accountColumns+` FROM account WHERE id = $1`, id))
}

func setPassword(ctx context.Context, tx pgx.Tx, marketplace, account, secret string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO credential (account_id, marketplace_id, kind, secret)
		VALUES ($1, $2, 'password', $3)
		ON CONFLICT (account_id, kind) DO UPDATE SET secret = EXCLUDED.secret, updated_at = now()`,
		account, marketplace, secret)
	return err
}

func passwordOf(ctx context.Context, tx pgx.Tx, account string) (string, error) {
	var secret string
	err := tx.QueryRow(ctx,
		`SELECT secret FROM credential WHERE account_id = $1 AND kind = 'password'`, account).Scan(&secret)
	return secret, err
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

```go
// internal/identity/identity.go
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

type auditEntry = audit.Entry

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

func (s *Service) record(ctx context.Context, tx pgx.Tx, v Visit, account, action string) error {
	after, _ := json.Marshal(map[string]string{"user_agent": v.UserAgent})
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
// the same page either way (spec, D7).
func (s *Service) SignUp(ctx context.Context, v Visit, name, email, password string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrNameMissing
	}
	normalized, err := NormalizeEmail(email)
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
		id, err := insertAccount(ctx, tx, v.Marketplace, strings.TrimSpace(email), normalized, name)
		if errors.Is(err, errTaken) {
			existing, err := accountByEmail(ctx, tx, normalized)
			if err != nil {
				return err
			}
			return mail.Request(ctx, tx, mail.Message{
				Template: "account-exists", Language: v.Language, To: existing.Email,
				From: v.MarketplaceName, Marketplace: v.Marketplace,
				Variables: map[string]string{"Name": existing.Name, "SignIn": v.link("/signin", nil),
					"Marketplace": v.MarketplaceName},
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
			Variables: map[string]string{"Name": name, "Marketplace": v.MarketplaceName,
				"Link": v.link("/verify", url.Values{"token": {token}})},
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
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return nil
	}
	token, hash, err := NewToken()
	if err != nil {
		return err
	}
	return s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		account, err := accountByEmail(ctx, tx, normalized)
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
			Variables: map[string]string{"Name": account.Name, "Marketplace": v.MarketplaceName,
				"Link": v.link("/verify", url.Values{"token": {token}})},
		})
	})
}
```

- [ ] **Step 5: The mail templates** — create, each with its first line `Subject: …`, using `{{.Variables.Name}}` etc. exactly as `web/mail/probe.*` does (read it first for the data shape):

`web/mail/verify-email.en-US.txt`
```
Subject: Confirm your e-mail address at {{.Variables.Marketplace}}
Hello, {{.Variables.Name}}.

Confirm your e-mail address to finish creating your account at {{.Variables.Marketplace}}:

{{.Variables.Link}}

The link works once and expires in 24 hours. If you did not create an account, ignore this message.
```
`web/mail/verify-email.pt-BR.txt`
```
Subject: Confirme seu e-mail em {{.Variables.Marketplace}}
Olá, {{.Variables.Name}}.

Confirme seu e-mail para terminar de criar sua conta em {{.Variables.Marketplace}}:

{{.Variables.Link}}

O link funciona uma vez e expira em 24 horas. Se você não criou uma conta, ignore esta mensagem.
```
`web/mail/account-exists.en-US.txt`
```
Subject: You already have an account at {{.Variables.Marketplace}}
Hello, {{.Variables.Name}}.

Someone tried to create an account at {{.Variables.Marketplace}} with this address, which already has one. To sign in:

{{.Variables.SignIn}}

If it was not you, you can ignore this message: nothing was changed.
```
`web/mail/account-exists.pt-BR.txt`
```
Subject: Você já tem uma conta em {{.Variables.Marketplace}}
Olá, {{.Variables.Name}}.

Alguém tentou criar uma conta em {{.Variables.Marketplace}} com este e-mail, que já tem uma. Para entrar:

{{.Variables.SignIn}}

Se não foi você, pode ignorar esta mensagem: nada foi alterado.
```
The four `.html` files carry the same text, each paragraph in `<p>`, the link as `<a href="{{.Variables.Link}}">{{.Variables.Link}}</a>` (and `SignIn` likewise), following `web/mail/probe.*.html`.

- [ ] **Step 6: Run to see them pass**

Run: `go test -tags=integration ./internal/identity/ -race -v && go test ./internal/platform/mail/ -v`
Expected: PASS (the mail package's template loading test covers the new pairs).

- [ ] **Step 7: Commit** — `git add internal/identity migrations/00009_accounts.sql web/mail && git commit -m "Create accounts, confirm their addresses, and never say which exist"`

## Task 7: The sign-up and verification pages, wired into the process

**Files:**
- Create: `internal/platform/httpx/identity.go`, `web/identity.templ` (then `make generate`)
- Modify: `internal/platform/httpx/site.go` (field, `WithIdentity`, routes), `cmd/marketplace/main.go` (build the service), `web/locales/*.json`, `docs/design.md` §2.2
- Test: `internal/platform/httpx/identity_test.go`, `e2e/test_signup.py`

**Interfaces:**
- Consumes: Task 6 `Service`, `Visit`, errors; `ratelimit.NewDatabase(execer, limit, window)`; `ratelimit.Limit(limiter, subject, pages.Refused, log)`.
- Produces:
  - `type IdentityRoutes struct { Service *identity.Service; Limits IdentityLimits; Pages Pages; Log *slog.Logger }`
  - `type IdentityLimits struct { SignUp, Resend, ResendAddress, SignIn, SignInAddress, Password ratelimit.Limiter }` (the last three used in PR 3 and 4)
  - `func (s Site) WithIdentity(routes IdentityRoutes) Site`
  - `func visit(r *http.Request) identity.Visit` — in `httpx/identity.go`
  - templ: `SignUp(page Page, form Form)`, `CheckEmail(page Page)`, `Verify(page Page, token string, failed bool)`, `Resend(page Page, sent bool)`; `type Form struct { Name, Email, Error string }` in `web/page.go`

- [ ] **Step 1: Locale keys** — add to `web/locales/en-US.json` and `pt-BR.json` (same keys, both files):

| key | en-US | pt-BR |
|---|---|---|
| `identity.signup.title` | Create your account | Crie sua conta |
| `identity.signup.submit` | Create account | Criar conta |
| `identity.field.name` | Name | Nome |
| `identity.field.email` | E-mail | E-mail |
| `identity.field.password` | Password | Senha |
| `identity.field.password_hint` | At least 12 characters. A phrase is easier to remember than symbols. | No mínimo 12 caracteres. Uma frase é mais fácil de lembrar que símbolos. |
| `identity.error.name_missing` | Tell us your name. | Informe seu nome. |
| `identity.error.email_invalid` | This does not look like an e-mail address. | Isto não parece um endereço de e-mail. |
| `identity.error.password_short` | The password needs at least 12 characters. | A senha precisa de pelo menos 12 caracteres. |
| `identity.error.password_long` | The password can have at most 128 characters. | A senha pode ter no máximo 128 caracteres. |
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

- [ ] **Step 2: Write the failing handler test**

```go
// internal/platform/httpx/identity_test.go
package httpx

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// A form post without a database behind it must not reach the service: the
// validation that needs no database answers first, in the visitor's language.
func TestSignUpShowsTheFieldErrorBeforeTouchingTheDatabase(t *testing.T) {
	site := testSite(t).WithIdentity(testIdentityRoutes(t))
	form := url.Values{"name": {""}, "email": {"a@example.test"}, "password": {"correct horse battery"}}
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = withMarketplace(req) // puts a tenancy resolution and pt-BR in the context
	rec := httptest.NewRecorder()

	site.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Informe seu nome.") {
		t.Fatalf("status %d, body %q", rec.Code, rec.Body.String())
	}
}
```

`testSite`, `testIdentityRoutes` and `withMarketplace` are small helpers in the same file: `testSite` mirrors how `site_test.go` builds a `Site` (read it and reuse its helpers where they exist); `testIdentityRoutes` builds an `IdentityRoutes` whose `Service` is `identity.NewService(nil, identity.NewHasher(identity.Params{Memory: 64, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16}, 1), breached.Fake{}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))` and whose limiters are `ratelimit.Never{}`; `withMarketplace` sets `tenancy.WithResolution` (or the package's existing test helper — read `internal/tenancy/middleware.go`) and `i18n.WithLanguage(ctx, "pt-BR")`. The service is never reached in this test, so its nil database is never used.

- [ ] **Step 3: Run to see it fail** — `go test ./internal/platform/httpx/ -run SignUp -v` → FAIL to compile.

- [ ] **Step 4: Implement the handlers**

```go
// internal/platform/httpx/identity.go
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
// database so they hold across instances (docs/roadmap.md, F13).
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

func (s Site) identityRoutes(mux *http.ServeMux) {
	if s.identity == nil {
		return
	}
	id := s.identity
	byIP := func(prefix string) ratelimit.Subject {
		return func(r *http.Request) string {
			origin, _ := OriginFrom(r.Context())
			return prefix + ":" + origin.IP
		}
	}
	byAddress := func(prefix string) ratelimit.Subject {
		return func(r *http.Request) string {
			normalized, _ := identity.NormalizeEmail(r.PostFormValue("email"))
			return prefix + ":" + marketplaceOf(r) + ":" + normalized
		}
	}
	limit := func(l ratelimit.Limiter, subject ratelimit.Subject, h http.HandlerFunc) http.Handler {
		return ratelimit.Limit(l, subject, id.Pages.Refused, id.Log)(h)
	}

	mux.HandleFunc("GET /signup", s.signUpForm)
	mux.Handle("POST /signup", limit(id.Limits.SignUp, byIP("signup"), s.signUp))
	mux.HandleFunc("GET /verify", s.verifyForm)
	mux.HandleFunc("POST /verify", s.verify)
	mux.HandleFunc("GET /verify/resend", s.resendForm)
	mux.Handle("POST /verify/resend", limit(id.Limits.Resend, byIP("resend"),
		limit(id.Limits.ResendAddress, byAddress("resend"), s.resend).ServeHTTP))
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

// errorKey names the message a flow's error is shown as. An error it does not
// name is not the visitor's to see.
func errorKey(err error) (string, bool) {
	switch {
	case errors.Is(err, identity.ErrNameMissing):
		return "identity.error.name_missing", true
	case errors.Is(err, identity.ErrEmailInvalid):
		return "identity.error.email_invalid", true
	case errors.Is(err, identity.ErrPasswordShort):
		return "identity.error.password_short", true
	case errors.Is(err, identity.ErrPasswordLong):
		return "identity.error.password_long", true
	case errors.Is(err, identity.ErrPasswordBreached):
		return "identity.error.password_breached", true
	}
	return "", false
}

func (s Site) signUpForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, web.SignUp(s.page(r, "/signup"), web.Form{}))
}

func (s Site) signUp(w http.ResponseWriter, r *http.Request) {
	form := web.Form{Name: r.PostFormValue("name"), Email: r.PostFormValue("email")}
	err := s.identity.Service.SignUp(r.Context(), visit(r), form.Name, form.Email, r.PostFormValue("password"))
	if key, shown := errorKey(err); shown {
		form.Error = key
		w.WriteHeader(http.StatusUnprocessableEntity)
		render(w, r, web.SignUp(s.page(r, "/signup"), form))
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
		w.WriteHeader(http.StatusGone)
		render(w, r, web.Verify(s.page(r, "/verify"), "", true))
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

In `site.go`: add the field `identity *IdentityRoutes` to `Site`, and call `s.identityRoutes(mux)` in `Handler()` before the `GET /{$}` line.

In `web/page.go` add:

```go
// Form is what a form page shows back: the fields a visitor typed, except the
// password, and the key of the message explaining what went wrong.
type Form struct {
	Name  string
	Email string
	Error string
}
```

- [ ] **Step 5: The pages** — `web/identity.templ` (then `make generate`):

```templ
package web

templ SignUp(page Page, form Form) {
	@Layout(page, page.T("identity.signup.title")) {
		<main>
			<h1>{ page.T("identity.signup.title") }</h1>
			if form.Error != "" {
				<p role="alert">{ page.T(form.Error) }</p>
			}
			<form method="post" action={ templ.SafeURL("/" + page.Language + "/signup") }>
				<input type="hidden" name="csrf_token" value={ page.CSRFToken }/>
				<label>{ page.T("identity.field.name") }
					<input name="name" autocomplete="name" required value={ form.Name }/></label>
				<label>{ page.T("identity.field.email") }
					<input name="email" type="email" autocomplete="email" required value={ form.Email }/></label>
				<label>{ page.T("identity.field.password") }
					<input name="password" type="password" autocomplete="new-password" required minlength="12" maxlength="128"/></label>
				<p>{ page.T("identity.field.password_hint") }</p>
				<button type="submit">{ page.T("identity.signup.submit") }</button>
			</form>
			<p><a href={ templ.SafeURL("/" + page.Language + "/signin") }>{ page.T("identity.link.signin") }</a></p>
		</main>
	}
}

templ CheckEmail(page Page) {
	@Layout(page, page.T("identity.check_email.title")) {
		<main>
			<h1>{ page.T("identity.check_email.title") }</h1>
			<p>{ page.T("identity.check_email.body") }</p>
			<p><a href={ templ.SafeURL("/" + page.Language + "/verify/resend") }>{ page.T("identity.link.resend") }</a></p>
		</main>
	}
}

templ Verify(page Page, token string, failed bool) {
	@Layout(page, page.T("identity.verify.title")) {
		<main>
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
		</main>
	}
}

templ Resend(page Page, sent bool) {
	@Layout(page, page.T("identity.resend.title")) {
		<main>
			<h1>{ page.T("identity.resend.title") }</h1>
			if sent {
				<p role="status">{ page.T("identity.resend.sent") }</p>
			} else {
				<form method="post" action={ templ.SafeURL("/" + page.Language + "/verify/resend") }>
					<input type="hidden" name="csrf_token" value={ page.CSRFToken }/>
					<label>{ page.T("identity.field.email") }
						<input name="email" type="email" autocomplete="email" required/></label>
					<button type="submit">{ page.T("identity.resend.submit") }</button>
				</form>
			}
		</main>
	}
}
```

Check how `web/layout.templ` marks up forms today (the language switch) and follow its style and CSS classes; the literal check (`web/literals_test.go`) must pass — every visible string above goes through `page.T`.

- [ ] **Step 6: Wire the process** — in `cmd/marketplace/main.go`, in `serve`, where `database != nil` (after the audit keeper is built), build and attach:

```go
	if database != nil {
		var checker breached.Checker = breached.Fake{Known: breached.Common}
		if cfg.ProvidersMode == config.ProvidersReal {
			checker = breached.NewPwned(&http.Client{Timeout: 3 * time.Second}, breached.PwnedBase)
		}
		// Four hashes at once at the provisional 19 MiB is 76 MiB of the
		// instance's 512 (spec, D5); Task 12 revisits both numbers together.
		hasher := identity.NewHasher(identity.Provisional, 4)
		service := identity.NewService(database, hasher, checker,
			audit.NewLog(audit.NewKeys(keeper), log), log)
		site = site.WithIdentity(httpx.IdentityRoutes{
			Service: service,
			Pages:   pages,
			Log:     log,
			Limits: httpx.IdentityLimits{
				SignUp:        ratelimit.NewDatabase(database, 10, time.Hour),
				Resend:        ratelimit.NewDatabase(database, 10, time.Hour),
				ResendAddress: ratelimit.NewDatabase(database, 3, time.Hour),
				SignIn:        ratelimit.NewDatabase(database, 30, 10*time.Minute),
				SignInAddress: ratelimit.NewDatabase(database, 10, 15*time.Minute),
				Password:      ratelimit.NewDatabase(database, 10, time.Hour),
			},
		})
	}
```

Check `ratelimit.NewDatabase`'s first parameter type (`Execer`) accepts `*db.Pool`; if not, pass what `internal/platform/ratelimit/database_integration_test.go` passes.

In `docs/design.md` §2.2, add `breached` to the list of ports: "`breached` (whether a password appears in a public breach; Pwned Passwords, failing open)".

- [ ] **Step 7: The end-to-end test**

```python
# e2e/test_signup.py
"""Sign-up and confirmation, in a browser, with the mail read from the fake mailbox."""

from __future__ import annotations

import json
import time

import pytest
from playwright.sync_api import sync_playwright

PASSWORD = "correct horse battery staple"


def wait_for_mail(mailbox, template, timeout=10.0):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        for path in sorted(mailbox.glob("*.json")):
            message = json.loads(path.read_text())
            if message["template"] == template:
                return message
        time.sleep(0.2)
    raise AssertionError(f"no {template} mail in {mailbox}")


@pytest.mark.local_process
@pytest.mark.parametrize("language", ["pt-BR", "en-US"])
def test_sign_up_and_confirm(run_marketplace, screenshots, language):
    marketplace = run_marketplace()
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        page.goto(marketplace.url("m1.localhost", f"/{language}/signup"))
        page.screenshot(path=screenshots / f"f13-signup-{language}.png")

        page.fill("input[name=name]", "Leitora")
        page.fill("input[name=email]", f"leitora-{language}@example.test")
        page.fill("input[name=password]", PASSWORD)
        page.click("button[type=submit]")
        page.screenshot(path=screenshots / f"f13-check-email-{language}.png")

        message = wait_for_mail(marketplace.mailbox, "verify-email")
        link = message["variables"]["Link"].replace("http://m1.localhost", f"http://m1.localhost:{marketplace.port}")
        page.goto(link)
        page.screenshot(path=screenshots / f"f13-verify-{language}.png")
        page.click("button[type=submit]")
        assert "/signin" in page.url
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
        page.click("button[type=submit]")
        assert page.locator("[role=alert]").inner_text().startswith("Esta senha já apareceu")
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
            page.click("button[type=submit]")
            answers.append(page.locator("main").inner_text())
        assert answers[0] == answers[1]
        wait_for_mail(marketplace.mailbox, "account-exists")
        browser.close()
```

The link in the mail is built from the request's host, which in the suite is `m1.localhost:PORT`; if `origin(r)` keeps the port, drop the `.replace(...)` — check the first run's `message["variables"]["Link"]` and keep whichever is true.

- [ ] **Step 8: Run everything** — `make generate && make check && make test && make integration && make e2e`. Expected: all green; new screenshots `f13-signup-*`, `f13-check-email-*`, `f13-verify-*` in both languages.

- [ ] **Step 9: Commit, push, open PR 2, subscribe**

```bash
git add -A internal web cmd docs e2e
git commit -m "Let a visitor create an account and confirm the address"
git push -u origin claude/funny-wright-379asb-f13accounts
```

---

# PR 3 — Sessions

## Task 8: The session schema and the session flows

**Files:**
- Create: `migrations/00010_sessions.sql`
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

- [ ] **Step 1: The migration**

```sql
-- migrations/00010_sessions.sql
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

```go
//go:build integration

// internal/identity/session_integration_test.go
package identity

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// confirmed signs up and confirms an account, returning its address.
func confirmed(t *testing.T, s *Service, db serving, marketplace, email string) {
	t.Helper()
	if err := s.SignUp(t.Context(), visit(marketplace), "Reader", email, "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	mails := outbox(t, db, marketplace)
	link := mails[len(mails)-1]["link"]
	if err := s.Verify(t.Context(), visit(marketplace), link[strings.Index(link, "token=")+6:]); err != nil {
		t.Fatal(err)
	}
}

func TestSignInOutAndRevocation(t *testing.T) {
	s, db, one, _, _ := service(t)
	confirmed(t, s, db, one, "r@example.test")

	token, err := s.SignIn(t.Context(), visit(one), "R@Example.Test", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
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
	s, db, one, _, _ := service(t)
	confirmed(t, s, db, one, "r@example.test")
	_, wrong := s.SignIn(t.Context(), visit(one), "r@example.test", "not the password at all")
	_, unknown := s.SignIn(t.Context(), visit(one), "nobody@example.test", "not the password at all")
	if !errors.Is(wrong, ErrCredentials) || !errors.Is(unknown, ErrCredentials) {
		t.Fatalf("wrong = %v, unknown = %v; want ErrCredentials for both", wrong, unknown)
	}
}

func TestAnUnconfirmedAccountCannotSignIn(t *testing.T) {
	s, _, one, _, _ := service(t)
	_ = s.SignUp(t.Context(), visit(one), "R", "r@example.test", "correct horse battery staple")
	if _, err := s.SignIn(t.Context(), visit(one), "r@example.test", "correct horse battery staple"); !errors.Is(err, ErrUnverified) {
		t.Fatalf("SignIn(unconfirmed) = %v, want ErrUnverified", err)
	}
}

func TestIdleAndExpiredSessionsAreRefused(t *testing.T) {
	s, db, one, _, _ := service(t)
	confirmed(t, s, db, one, "r@example.test")
	idle, _ := s.SignIn(t.Context(), visit(one), "r@example.test", "correct horse battery staple")
	old, _ := s.SignIn(t.Context(), visit(one), "r@example.test", "correct horse battery staple")
	_ = db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		_, _ = tx.Exec(t.Context(), `UPDATE session SET last_seen_at = now() - interval '31 days' WHERE token_hash = $1`, HashToken(idle))
		_, err := tx.Exec(t.Context(), `UPDATE session SET created_at = now() - interval '91 days' WHERE token_hash = $1`, HashToken(old))
		return err
	})
	for name, token := range map[string]string{"idle": idle, "expired": old} {
		if _, err := s.Authenticate(t.Context(), one, token); !errors.Is(err, ErrSessionInvalid) {
			t.Errorf("%s session: %v, want ErrSessionInvalid", name, err)
		}
	}
}

func TestASessionIsInvisibleFromAnotherMarketplace(t *testing.T) {
	s, db, one, two, _ := service(t)
	confirmed(t, s, db, one, "r@example.test")
	token, _ := s.SignIn(t.Context(), visit(one), "r@example.test", "correct horse battery staple")
	if _, err := s.Authenticate(t.Context(), two, token); !errors.Is(err, ErrSessionInvalid) {
		t.Fatalf("a session of one marketplace authenticated in the other: %v", err)
	}
}

func TestAStaleHashIsReplacedAtSignIn(t *testing.T) {
	db, one, _ := twoMarketplaces(t)
	old := NewService(db, NewHasher(cheap, 1), breachedNone, &recording{}, quiet)
	confirmed(t, old, db, one, "r@example.test")
	newer := NewService(db, NewHasher(Params{Memory: 128, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16}, 1), breachedNone, &recording{}, quiet)
	if _, err := newer.SignIn(t.Context(), visit(one), "r@example.test", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	_ = db.InTxFor(t.Context(), one, func(tx pgx.Tx) error {
		var secret string
		_ = tx.QueryRow(t.Context(), "SELECT secret FROM credential").Scan(&secret)
		if !strings.Contains(secret, "m=128,") {
			t.Fatalf("the hash was not refreshed: %s", secret)
		}
		return nil
	})
	_ = time.Second
}
```

Add to `identity_integration_test.go`: `var breachedNone = breached.Fake{}` and `var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))`.

- [ ] **Step 3: Run to see them fail** — `go test -tags=integration ./internal/identity/ -run 'Sign|Idle|Session|Stale' -v` → FAIL to compile.

- [ ] **Step 4: Implement** — append to `store.go`:

```go
func insertSession(ctx context.Context, tx pgx.Tx, marketplace, account string, hash []byte, ip, agent string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO session (account_id, marketplace_id, token_hash, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5)`, account, marketplace, hash, ip, agent)
	return err
}

type sessionRow struct {
	id, account string
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

Append to `identity.go`:

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

// SignIn checks a password and opens a new session, returning its token.
func (s *Service) SignIn(ctx context.Context, v Visit, email, password string) (string, error) {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		s.hasher.Waste(ctx, password)
		return "", ErrCredentials
	}
	var token string
	err = s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		account, err := accountByEmail(ctx, tx, normalized)
		if errors.Is(err, pgx.ErrNoRows) {
			s.hasher.Waste(ctx, password)
			s.log.InfoContext(ctx, "sign-in for an unknown address")
			return ErrCredentials
		}
		if err != nil {
			return err
		}
		secret, err := passwordOf(ctx, tx, account.ID)
		if err != nil {
			return err
		}
		ok, stale, err := s.hasher.Verify(ctx, password, secret)
		if err != nil {
			return err
		}
		if !ok {
			// Recorded, and the transaction still commits: the refusal is
			// returned after it (see below).
			if err := s.record(ctx, tx, v, account.ID, "identity.signin_failed"); err != nil {
				return err
			}
			return nil
		}
		if account.VerifiedAt == nil {
			return ErrUnverified
		}
		if stale {
			fresh, err := s.hasher.Hash(ctx, password)
			if err != nil {
				return err
			}
			if err := setPassword(ctx, tx, v.Marketplace, account.ID, fresh); err != nil {
				return err
			}
		}
		t, hash, err := NewToken()
		if err != nil {
			return err
		}
		if err := insertSession(ctx, tx, v.Marketplace, account.ID, hash, v.IP, v.UserAgent); err != nil {
			return err
		}
		token = t
		return s.record(ctx, tx, v, account.ID, "identity.signin")
	})
	if err != nil {
		return "", err
	}
	if token == "" {
		return "", ErrCredentials
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

- [ ] **Step 5: Run to see them pass** — `go test -tags=integration ./internal/identity/ -race -v` → PASS.

- [ ] **Step 6: Commit** — `git commit -m "Open, check and revoke sessions stored on the server"`

## Task 9: The session middleware, sign-in and sign-out pages

**Files:**
- Create: `internal/platform/httpx/session.go`
- Modify: `internal/platform/httpx/csrf.go` (`RenewCSRF`), `internal/platform/httpx/identity.go` (routes), `internal/platform/httpx/site.go` (`page()` carries the account), `web/page.go` (`Account string`), `web/layout.templ` (header), `web/identity.templ` (`SignIn`), locales, `cmd/marketplace/main.go` (mount)
- Test: `internal/platform/httpx/session_test.go`

**Interfaces:**
- Produces: `func Sessions(service *identity.Service, log *slog.Logger) func(http.Handler) http.Handler`; `func SessionFrom(ctx context.Context) (identity.Session, bool)`; `func RenewCSRF(w http.ResponseWriter, r *http.Request)`; templ `SignIn(page Page, form Form, verified bool)`.

- [ ] **Step 1: Locale keys** (both files)

| key | en-US | pt-BR |
|---|---|---|
| `identity.signin.title` | Sign in | Entrar |
| `identity.signin.submit` | Sign in | Entrar |
| `identity.signin.failed` | The e-mail or the password is not right. | O e-mail ou a senha não conferem. |
| `identity.signin.unverified` | Confirm your e-mail address first. We can send the link again. | Confirme seu e-mail primeiro. Podemos reenviar o link. |
| `identity.signout.submit` | Sign out | Sair |
| `identity.header.signed_in_as` | Signed in as %[1]s | Conectado como %[1]s |

- [ ] **Step 2: Write the failing middleware test**

```go
// internal/platform/httpx/session_test.go
package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNoCookieMeansNoSessionAndNoDatabaseCall(t *testing.T) {
	var seen bool
	handler := Sessions(nil, discardLog())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, seen = SessionFrom(r.Context())
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if seen {
		t.Fatal("a request with no cookie carried a session")
	}
}
```

(`Sessions(nil, …)` proves the service is not reached without a cookie; the database paths are covered by Task 8's integration tests and Task 10's end-to-end tests. `discardLog()` is `slog.New(slog.NewTextHandler(io.Discard, nil))`.)

- [ ] **Step 3: Implement**

```go
// internal/platform/httpx/session.go
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
// no longer opens a session is cleared, so the browser stops sending it.
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

In `csrf.go`, add:

```go
// RenewCSRF gives the browser a new CSRF secret. Sign-in calls it, so a token
// minted before the session existed — possibly by someone who planted the
// secret — does not carry over into it (sign-in CSRF, session fixation).
func RenewCSRF(w http.ResponseWriter, r *http.Request) {
	name := csrfCookie
	if !isHTTPS(r) {
		name = csrfCookieInsecure
	}
	secret := make([]byte, secretBytes)
	_, _ = rand.Read(secret)
	// #nosec G124 -- the flags follow the scheme of the request, as in csrfSecret.
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: base64.RawURLEncoding.EncodeToString(secret), Path: "/",
		HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode,
	})
}
```

Routes, in `identityRoutes`:

```go
	mux.HandleFunc("GET /signin", s.signInForm)
	mux.Handle("POST /signin", limit(id.Limits.SignIn, byIP("signin"),
		limit(id.Limits.SignInAddress, byAddress("signin"), s.signIn).ServeHTTP))
	mux.HandleFunc("POST /signout", s.signOut)
```

Handlers, appended to `identity.go` (httpx):

```go
func (s Site) signInForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, web.SignIn(s.page(r, "/signin"), web.Form{}, r.URL.Query().Get("verified") == "1"))
}

func (s Site) signIn(w http.ResponseWriter, r *http.Request) {
	form := web.Form{Email: r.PostFormValue("email")}
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
	w.WriteHeader(http.StatusUnauthorized)
	render(w, r, web.SignIn(s.page(r, "/signin"), form, false))
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

`web/page.go`: add `Account string // the signed-in account's name, empty when signed out`. `site.go` `page()`: after the marketplace block, `if session, ok := SessionFrom(r.Context()); ok { page.Account = session.Account.Name }`.

`web/identity.templ`, add:

```templ
templ SignIn(page Page, form Form, verified bool) {
	@Layout(page, page.T("identity.signin.title")) {
		<main>
			<h1>{ page.T("identity.signin.title") }</h1>
			if verified {
				<p role="status">{ page.T("identity.verify.done") }</p>
			}
			if form.Error != "" {
				<p role="alert">{ page.T(form.Error) }</p>
			}
			<form method="post" action={ templ.SafeURL("/" + page.Language + "/signin") }>
				<input type="hidden" name="csrf_token" value={ page.CSRFToken }/>
				<label>{ page.T("identity.field.email") }
					<input name="email" type="email" autocomplete="username" required value={ form.Email }/></label>
				<label>{ page.T("identity.field.password") }
					<input name="password" type="password" autocomplete="current-password" required/></label>
				<button type="submit">{ page.T("identity.signin.submit") }</button>
			</form>
			<p><a href={ templ.SafeURL("/" + page.Language + "/signup") }>{ page.T("identity.link.signup") }</a>
				· <a href={ templ.SafeURL("/" + page.Language + "/verify/resend") }>{ page.T("identity.link.resend") }</a></p>
		</main>
	}
}
```

`web/layout.templ`: inside `<body>` before the page's children, a header shown only on a marketplace's pages (`page.Marketplace != ""`):

```templ
			if page.Marketplace != "" {
				<nav aria-label="account">
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

Pages rendered without a marketplace (`InPreparation`, the platform page) keep no header.

`cmd/marketplace/main.go`: right after `handler := site.Handler()`, when the identity service exists:

```go
	// The session is read after the marketplace and the language are known,
	// because a session belongs to one marketplace and is looked up in its
	// transaction (docs/design.md, section 2.3).
	if identityService != nil {
		handler = httpx.Sessions(identityService, log)(handler)
	}
```

(declare `var identityService *identity.Service` before the `database != nil` block of Task 7 and assign it there). Because `handler` is wrapped *inside* the i18n resolver and tenancy (they are applied afterwards, outside it), the order in the request is: … tenancy → i18n → sessions → mux, as the spec requires.

- [ ] **Step 4: Run** — `make generate && go test ./internal/platform/httpx/ ./web/ -race -v` → PASS (including the literal and parity checks).

- [ ] **Step 5: Commit** — `git commit -m "Sign in and out, with the session read on every request"`

## Task 10: End-to-end — sign-in, sign-out, refusals and the limit

**Files:**
- Create: `e2e/test_signin.py`
- Create: `e2e/accounts.py` (the helper that signs up and confirms through the mailbox, reused by Task 13)

- [ ] **Step 1: The helper**

```python
# e2e/accounts.py
"""Make a confirmed account the way a person would, and hand it to a test."""

from __future__ import annotations

import json
import time

PASSWORD = "correct horse battery staple"


def wait_for_mail(mailbox, template, to, timeout=10.0):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        for path in sorted(mailbox.glob("*.json")):
            message = json.loads(path.read_text())
            if message["template"] == template and message["to"] == to:
                return message
        time.sleep(0.2)
    raise AssertionError(f"no {template} mail to {to} in {mailbox}")


def confirmed_account(page, marketplace, email, host="m1.localhost", language="pt-BR"):
    page.goto(marketplace.url(host, f"/{language}/signup"))
    page.fill("input[name=name]", "Leitora")
    page.fill("input[name=email]", email)
    page.fill("input[name=password]", PASSWORD)
    page.click("button[type=submit]")
    link = wait_for_mail(marketplace.mailbox, "verify-email", email)["variables"]["Link"]
    page.goto(link if f":{marketplace.port}" in link
              else link.replace(f"http://{host}", f"http://{host}:{marketplace.port}"))
    page.click("button[type=submit]")


def sign_in(page, marketplace, email, password=PASSWORD, host="m1.localhost", language="pt-BR"):
    page.goto(marketplace.url(host, f"/{language}/signin"))
    page.fill("input[name=email]", email)
    page.fill("input[name=password]", password)
    page.click("button[type=submit]")
```

Replace `wait_for_mail` in `e2e/test_signup.py` with `from accounts import wait_for_mail` (adding the `to` argument there).

- [ ] **Step 2: The tests**

```python
# e2e/test_signin.py
from __future__ import annotations

import pytest
from playwright.sync_api import sync_playwright

from accounts import PASSWORD, confirmed_account, sign_in


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
        assert page.locator("nav[aria-label=account] span").is_visible()
        page.screenshot(path=screenshots / f"f13-signed-in-{language}.png")
        page.click("nav[aria-label=account] button[type=submit]")
        assert not page.locator("nav[aria-label=account] span").is_visible()
        browser.close()


@pytest.mark.local_process
def test_wrong_password_and_unknown_address_read_the_same(run_marketplace):
    marketplace = run_marketplace()
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        confirmed_account(page, marketplace, "existe@example.test")
        sign_in(page, marketplace, "existe@example.test", password="not the right password")
        wrong = page.locator("[role=alert]").inner_text()
        sign_in(page, marketplace, "nao-existe@example.test", password="not the right password")
        unknown = page.locator("[role=alert]").inner_text()
        assert wrong == unknown
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
        page.click("button[type=submit]")
        sign_in(page, marketplace, "pendente@example.test")
        assert page.locator("[role=alert]").inner_text().startswith("Confirme seu e-mail")
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
            response = None
            page.goto(marketplace.url("m1.localhost", "/pt-BR/signin"))
            page.fill("input[name=email]", "alvo@example.test")
            page.fill("input[name=password]", f"guess number {attempt:04d}")
            with page.expect_response(lambda r: r.request.method == "POST") as info:
                page.click("button[type=submit]")
            response = info.value
            statuses.append(response.status)
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
        page.goto(marketplace.url("m2.localhost", "/pt-BR/"))
        assert not page.locator("nav[aria-label=account] span").is_visible()
        browser.close()
```

- [ ] **Step 3: Run everything** — `make generate && make check && make test && make integration && make e2e` → all green; screenshots `f13-signin-*`, `f13-signed-in-*`.

- [ ] **Step 4: Commit, push, open PR 3, subscribe**

```bash
git add -A internal web cmd e2e migrations
git commit -m "Prove sign-in, sign-out and their refusals in a browser"
git push -u origin claude/funny-wright-379asb-f13sessions
```

---

# PR 4 — Password change

## Task 11: Change the password and end the other sessions

**Files:**
- Modify: `internal/identity/store.go`, `internal/identity/identity.go`, `internal/platform/httpx/identity.go`, `web/identity.templ`, locales
- Create: `web/mail/password-changed.{en-US,pt-BR}.{txt,html}`
- Test: `internal/identity/password_integration_test.go`

**Interfaces:**
- Produces: `func (s *Service) ChangePassword(ctx context.Context, v Visit, session Session, current, next string) error` — `ErrCredentials` for a wrong current password, and the D4 errors for the new one; templ `Password(page Page, form Form, done bool)`; route `GET/POST /account/password` (redirects to `/signin` when signed out).

- [ ] **Step 1: Write the failing test**

```go
//go:build integration

// internal/identity/password_integration_test.go
package identity

import (
	"errors"
	"testing"
)

func TestChangingThePasswordEndsEveryOtherSession(t *testing.T) {
	s, db, one, _, trail := service(t)
	confirmed(t, s, db, one, "r@example.test")
	here, _ := s.SignIn(t.Context(), visit(one), "r@example.test", "correct horse battery staple")
	there, _ := s.SignIn(t.Context(), visit(one), "r@example.test", "correct horse battery staple")
	session, _ := s.Authenticate(t.Context(), one, here)

	if err := s.ChangePassword(t.Context(), visit(one), session, "wrong current password", "a brand new passphrase"); !errors.Is(err, ErrCredentials) {
		t.Fatalf("wrong current: %v, want ErrCredentials", err)
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
	if _, err := s.SignIn(t.Context(), visit(one), "r@example.test", "a brand new passphrase"); err != nil {
		t.Fatalf("the new password does not sign in: %v", err)
	}
	mails := outbox(t, db, one)
	if mails[len(mails)-1]["template"] != "password-changed" {
		t.Fatalf("no password-changed mail: %v", mails)
	}
	if trail.actions[len(trail.actions)-2] != "identity.password_changed" {
		t.Fatalf("audited %v", trail.actions)
	}
}
```

- [ ] **Step 2: Run to see it fail** — `go test -tags=integration ./internal/identity/ -run Changing -v` → FAIL.

- [ ] **Step 3: Implement** — `store.go`:

```go
func revokeOtherSessions(ctx context.Context, tx pgx.Tx, account, keep string, now time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE session SET revoked_at = $3, revoked_reason = 'password_changed'
		 WHERE account_id = $1 AND id <> $2 AND revoked_at IS NULL`, account, keep, now)
	return err
}
```

`identity.go`:

```go
// ChangePassword replaces the password and ends every other session of the
// account (docs/requirements.md, section 18.1), keeping the one that asked.
func (s *Service) ChangePassword(ctx context.Context, v Visit, session Session, current, next string) error {
	if err := s.checkNew(ctx, next); err != nil {
		return err
	}
	fresh, err := s.hasher.Hash(ctx, next)
	if err != nil {
		return err
	}
	return s.db.InTxFor(ctx, v.Marketplace, func(tx pgx.Tx) error {
		secret, err := passwordOf(ctx, tx, session.Account.ID)
		if err != nil {
			return err
		}
		ok, _, err := s.hasher.Verify(ctx, current, secret)
		if err != nil {
			return err
		}
		if !ok {
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
			Variables: map[string]string{"Name": session.Account.Name, "Marketplace": v.MarketplaceName},
		})
	})
}
```

(The audit record precedes the mail request in the transaction, which is why the test reads the second-to-last action — the last one is whatever follows; if the recording double only sees audit calls, read the last action instead. Keep the assertion honest to the order the code writes.)

Mail templates `web/mail/password-changed.*`:

en-US txt:
```
Subject: Your password at {{.Variables.Marketplace}} was changed
Hello, {{.Variables.Name}}.

The password of your account at {{.Variables.Marketplace}} was just changed, and every other session was signed out.

If it was not you, recover your account now and contact us.
```
pt-BR txt:
```
Subject: Sua senha em {{.Variables.Marketplace}} foi alterada
Olá, {{.Variables.Name}}.

A senha da sua conta em {{.Variables.Marketplace}} acabou de ser alterada, e todas as outras sessões foram encerradas.

Se não foi você, recupere sua conta agora e fale conosco.
```
(and the two `.html`, paragraphs in `<p>`).

Locale keys (both files):

| key | en-US | pt-BR |
|---|---|---|
| `identity.password.title` | Change password | Alterar senha |
| `identity.field.current_password` | Current password | Senha atual |
| `identity.field.new_password` | New password | Nova senha |
| `identity.password.submit` | Change password | Alterar senha |
| `identity.password.wrong_current` | The current password is not right. | A senha atual não confere. |
| `identity.password.done` | Your password was changed. Every other session was signed out. | Sua senha foi alterada. Todas as outras sessões foram encerradas. |
| `identity.header.password` | Password | Senha |

Handlers (httpx `identity.go`), route with the `Password` limit keyed by account:

```go
	byAccount := func(prefix string) ratelimit.Subject {
		return func(r *http.Request) string {
			session, _ := SessionFrom(r.Context())
			return prefix + ":" + session.Account.ID
		}
	}
	mux.HandleFunc("GET /account/password", s.passwordForm)
	mux.Handle("POST /account/password", limit(id.Limits.Password, byAccount("password"), s.changePassword))
```

```go
func (s Site) passwordForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := SessionFrom(r.Context()); !ok {
		http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+"/signin", http.StatusSeeOther)
		return
	}
	render(w, r, web.Password(s.page(r, "/account/password"), web.Form{}, false))
}

func (s Site) changePassword(w http.ResponseWriter, r *http.Request) {
	session, ok := SessionFrom(r.Context())
	if !ok {
		http.Redirect(w, r, "/"+i18n.FromContext(r.Context())+"/signin", http.StatusSeeOther)
		return
	}
	var form web.Form
	err := s.identity.Service.ChangePassword(r.Context(), visit(r), session,
		r.PostFormValue("current_password"), r.PostFormValue("new_password"))
	if errors.Is(err, identity.ErrCredentials) {
		form.Error = "identity.password.wrong_current"
	} else if key, shown := errorKey(err); shown {
		form.Error = key
	} else if err != nil {
		s.identity.Log.ErrorContext(r.Context(), "password change failed", "error", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if form.Error != "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		render(w, r, web.Password(s.page(r, "/account/password"), form, false))
		return
	}
	render(w, r, web.Password(s.page(r, "/account/password"), web.Form{}, true))
}
```

templ:

```templ
templ Password(page Page, form Form, done bool) {
	@Layout(page, page.T("identity.password.title")) {
		<main>
			<h1>{ page.T("identity.password.title") }</h1>
			if done {
				<p role="status">{ page.T("identity.password.done") }</p>
			}
			if form.Error != "" {
				<p role="alert">{ page.T(form.Error) }</p>
			}
			<form method="post" action={ templ.SafeURL("/" + page.Language + "/account/password") }>
				<input type="hidden" name="csrf_token" value={ page.CSRFToken }/>
				<label>{ page.T("identity.field.current_password") }
					<input name="current_password" type="password" autocomplete="current-password" required/></label>
				<label>{ page.T("identity.field.new_password") }
					<input name="new_password" type="password" autocomplete="new-password" required minlength="12" maxlength="128"/></label>
				<p>{ page.T("identity.field.password_hint") }</p>
				<button type="submit">{ page.T("identity.password.submit") }</button>
			</form>
		</main>
	}
}
```

In the layout's signed-in branch, add a link `<a href={ templ.SafeURL("/" + page.Language + "/account/password") }>{ page.T("identity.header.password") }</a>`.

- [ ] **Step 4: Run** — `make generate && go test -tags=integration ./internal/identity/ -race -v && go test ./internal/platform/... ./web/ -race` → PASS.

- [ ] **Step 5: Commit** — `git commit -m "Change the password and sign every other session out"`

## Task 12: The argon2id benchmark on the service's own CPU, and the final parameters

**Files:**
- Create: `cmd/marketplace/bench.go`, `cmd/marketplace/bench_test.go`
- Modify: `cmd/marketplace/main.go` (`run` switch: `case "bench-password": return benchPassword(ctx, log)`), `internal/identity/password.go` (replace `Provisional` with `Current` and the measurement), `docs/infrastructure.md`

**Interfaces:**
- Produces: `func benchPassword(ctx context.Context, log *slog.Logger) error` — logs one line per candidate `{"message":"argon2id","memory_kib":…,"time":…,"median_ms":…}` and one `{"message":"argon2id chosen",…}`.
- `func choose(results []benchResult, budget time.Duration) identity.Params` — the most expensive candidate (by memory × time) whose median is within budget.

- [ ] **Step 1: Write the failing test**

```go
// cmd/marketplace/bench_test.go
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

- [ ] **Step 2: Run to see it fail** — `go test ./cmd/marketplace/ -run Choose -v` → FAIL.

- [ ] **Step 3: Implement**

```go
// cmd/marketplace/bench.go
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
// CPU as the service (docs/infrastructure.md).
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
		for i := 0; i < 9; i++ {
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

In `internal/identity/password.go`, rename `Provisional` to `Floor` (keeping its value and comment) and add `Current`, set from the owner's run in Step 5; update the call site in `main.go` to `identity.NewHasher(identity.Current, slots)` with `slots = 256 MiB / Current.Memory` (at least 2), and its comment.

- [ ] **Step 4: Run** — `go test ./cmd/marketplace/ -race -v` → PASS. Locally, `./bin/marketplace bench-password` prints the five lines (the numbers are this machine's, not the service's).

- [ ] **Step 5: The owner's run** — after this task's commit is merged and deployed, the owner runs, in Cloud Shell:

```sh
gcloud run jobs execute marketplace-migrate --region=us-central1 \
  --project=aleogr-marketplace-lab-a4j5 --args=bench-password --wait
gcloud logging read 'resource.type="cloud_run_job" AND resource.labels.job_name="marketplace-migrate" AND jsonPayload.message=~"^argon2id"' \
  --project=aleogr-marketplace-lab-a4j5 --limit=10 --freshness=1h --format='value(jsonPayload)'
```

The chosen line's `memory_kib` and `time` become `identity.Current`, with the measurement and date in its comment, in a follow-up commit on this branch. Record the step in `docs/infrastructure.md` under the owner's manual steps: the command that re-runs it, and the numbers and date of the run Step 5 recorded.

The migration job's arguments are overridden only for that execution; its definition in Terraform is untouched, so the next deployment's migration runs as before. Check `infra/terraform/migrate_job.tf` for the job's name and region before handing the command over.

- [ ] **Step 6: Commit** — `git commit -m "Measure argon2id on the service's own CPU and choose its parameters"`

## Task 13: The two-browser end-to-end test, the screenshots, and closing F13

**Files:**
- Create: `e2e/test_password.py`
- Modify: `docs/roadmap.md` (tick F13's objective), `docs/superpowers/specs/2026-09-23-f13-identity-core-design.md` (status: implemented)

- [ ] **Step 1: The test the roadmap names**

```python
# e2e/test_password.py
"""The roadmap's F13 verification: two browsers, one password change."""

from __future__ import annotations

import pytest
from playwright.sync_api import sync_playwright

from accounts import PASSWORD, confirmed_account, sign_in, wait_for_mail


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
        assert second.locator("nav[aria-label=account] span").is_visible()

        first.goto(marketplace.url("m1.localhost", f"/{language}/account/password"))
        first.screenshot(path=screenshots / f"f13-password-{language}.png")
        first.fill("input[name=current_password]", PASSWORD)
        first.fill("input[name=new_password]", "a brand new passphrase")
        first.click("main button[type=submit]")
        assert first.locator("[role=status]").is_visible()
        first.screenshot(path=screenshots / f"f13-password-done-{language}.png")

        second.reload()
        assert not second.locator("nav[aria-label=account] span").is_visible(), \
            "the other browser is still signed in after the password changed"
        assert first.locator("nav[aria-label=account] span").is_visible()

        wait_for_mail(marketplace.mailbox, "password-changed", email)
        browser.close()
```

- [ ] **Step 2: Run everything** — `make generate && make check && make test && make integration && make e2e` → all green. The screenshots for F13 are now: signup, check-email, verify, signin, signed-in, password, password-done — each in pt-BR and en-US.

- [ ] **Step 3: Close the delivery in the documents** — tick `- [x] **Objective:**` of F13 in `docs/roadmap.md`; set the spec's status to `implemented on <date>` with the pull request numbers.

- [ ] **Step 4: Commit, push, open PR 4, subscribe**

```bash
git add -A e2e docs
git commit -m "Prove a password change signs every other browser out, and close F13"
git push -u origin claude/funny-wright-379asb-f13password
```

The pull request description lists the roadmap's F13 verification items one by one, each with its evidence (test names, screenshots artifact), per `docs/roadmap.md` section 3.

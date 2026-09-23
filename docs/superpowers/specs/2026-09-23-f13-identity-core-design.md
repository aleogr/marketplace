# F13 — Identity core: design

**Date:** 2026-09-23
**Status:** implemented on 2026-09-23, in aleogr/marketplace #70, #72, #73, #74 and #75
**Delivers:** roadmap F13 (`docs/roadmap.md`), against `docs/requirements.md` §4, §18.1 and §19,
and `docs/design.md` §2 and §4.

A person creates an account in a marketplace, confirms the e-mail address, signs in, signs out
and changes the password; a password change ends every other session of that account. Two-factor
authentication (F14), roles and the console (F15), account recovery (F16) and the parameter store
(F17) build on what this delivery puts in place, and none of them is in it.

## Decisions

**D1. Identity lives in the binary, in a new package `internal/identity`.** It follows the code
that exists: hand-written pgx queries, row-level security per marketplace, the audit log of F12,
the mail outbox of F11, the rate limiter of F7, templ pages. A separate identity service (Ory
Kratos, for instance) was rejected: a second service to run and pay for, which knows neither the
tenant isolation nor the audit log. A signed-cookie session was rejected because §18.1 requires
sessions stored server-side and revocable from the first release.

**D2. Accounts belong to a marketplace.** `account.marketplace_id` is set for buyers and stores and
null for platform staff (§4, design §4). The table is `account`, not the design's `user`, because
`user` is a reserved word in PostgreSQL and would need quoting in every query; the design's
section 4 is updated to say so. The e-mail address is unique per marketplace,
compared case-insensitively after trimming, so the same person holds separate accounts in two
marketplaces. `account.kind` is `buyer` for every account this delivery creates; the column exists
because F14's second-factor policy depends on it. A store is not an account: it arrives later as
`store` and `store_member` (design §4).

The comment in `migrations/00008_audit_log.sql` that calls a person "the platform's, not a
marketplace's" predates this decision. Nothing about `user_key` changes: the key is indexed by the
account's identifier, which is unique across the platform, and a staff account, which has no
marketplace, still has exactly one key. The correction is recorded in the new migration rather
than by editing an applied one.

**D3. Sessions last 30 days without use and 90 days at most.** The owner chose this on
2026-09-23 over a browser-session-with-"remember me" and over a 12-hour idle limit, because a buyer
signed out every day loses the cart and the order tracking. Sensitive actions will ask for a second
factor in F14. Staff sessions will be shorter, decided in F15. Both durations are constants in this
delivery and become parameters in F17.

**D4. Passwords have a length floor and no composition rules, and a breached password is
refused.** Twelve to 128 characters; spaces and any Unicode character allowed, so a long
passphrase is welcome. No rule demands a digit, a capital or a symbol: NIST SP 800-63B says a
verifier shall not impose them and OWASP ASVS agrees, because such rules produce predictable
passwords (`Senha@123`) that guessing tools try first. The owner chose the minimum of 12 on
2026-09-23: above the 8 that ASVS accepts, below the 15 that NIST's revision 4 requires for a
password used as the only factor, which is the case of a buyer without a second factor. The
minimum becomes a parameter in F17 and can be raised to 15 there; a raise applies to passwords set
from then on and signs nobody out. Composition rules are not a parameter, because the only setting
they offer makes passwords weaker.

A password that appears in the Pwned Passwords corpus is refused at sign-up and at a change; the
owner chose this on 2026-09-23 over a bundled list of common passwords and over no check. Only the
first five hexadecimal characters of the password's SHA-1 leave the server (the range API's
k-anonymity), with the `Add-Padding` header so the response size does not narrow it further. The
check sits behind a new port, `breached`, with a fake adapter for tests (§25). **When the service
cannot be reached, the password is accepted and the event is logged**: sign-up does not depend on
a third party being up.

**D5. Passwords are hashed with argon2id, and the parameters travel with the hash.** The stored
value is the PHC string (`$argon2id$v=19$m=…,t=…,p=…$salt$hash`), so a change of parameters does
not invalidate existing hashes: a hash whose parameters differ from the current ones is recomputed
at the next successful sign-in. The provisional parameters are OWASP's floor (19 MiB, two passes,
one lane). The final ones come from a benchmark run on the Cloud Run CPU the service uses (one
vCPU, 512 MiB), recorded next to the code with the measurement. A semaphore bounds how many hashes
run at once, because 80 concurrent requests on 512 MiB cannot each hold tens of megabytes. No hash
or verification runs inside a database transaction: a flow reads, hashes with no transaction open,
and then writes, so a hash never holds one of the pool's four connections.

**D6. Every token is random, sent once, and stored only as a hash.** Session tokens and e-mail
verification tokens are 32 random bytes; the database stores their SHA-256. A stolen database
dump holds nothing that signs anyone in. Comparisons are constant-time. A token is found by its
SHA-256, so the database never compares the secret itself, and a password's derived key is compared
with `crypto/subtle.ConstantTimeCompare`. The verification link, which carries the raw token, exists
in the database only while its e-mail is pending delivery: the outbox clears it from the event's
payload once the message is dispatched or, failing every attempt, parked.

**D7. Nothing tells a stranger whether an address has an account.** Sign-up always answers "we
sent you an e-mail"; an address that already has an account receives "you already have an
account, sign in or recover your password" instead of a verification link. Sign-in answers the
same sentence, after the same work, for an unknown address and for a wrong password: an unknown
address is hashed against a fixed dummy so the timing matches. The resend-verification form
answers generically. The one exception is a correct password on an unconfirmed account, which is
told to confirm the address, because whoever sees it already knows the password.

## Data model

Four tables, under row-level security like every tenant table (`current_marketplace_id()`,
migration 00004). Three arrive in the second pull request and `session` in the third, each in its
own migration:

| table | columns that matter |
|---|---|
| `account` | `id` uuid, `marketplace_id` (null for staff), `kind`, `email` (as entered), `email_normalised`, `name`, `verified_at`, `created_at`; unique `(marketplace_id, email_normalised)` `NULLS NOT DISTINCT`, so two staff accounts cannot share an address either |
| `credential` | `account_id`, `kind` (`password`), `secret` (PHC string), `updated_at` |
| `email_verification` | `account_id`, `token_hash`, `expires_at` (24 hours), `used_at` |
| `session` | `id`, `account_id`, `token_hash`, `created_at`, `last_seen_at`, `ip`, `user_agent`, `revoked_at`, `revoked_reason` |

Every row carries `marketplace_id` for the policy. A staff account (null marketplace) is invisible
to the application role; staff access arrives in F15 by its own path. `session` is created in the
third pull request and the other three in the second. `user_key.user_id` in the audit log holds an
`account.id`.

## Flows

**Sign-up** (`GET/POST /signup`): name, e-mail, password. Validation (D4), then in one transaction:
a new address creates the account unconfirmed, the credential and a verification token, and requests
the `verify-email` mail through the outbox; an existing address requests `account-exists` instead.
The answer is the same page either way (D7). Rate-limited per IP. Audited as `identity.signup`.

**Verification** (`GET /verify?token=…`, `POST /verify`): the link opens a page with a confirm
button, because corporate mail filters open links by themselves and would spend a single-use token
on a GET. A valid, unexpired, unused token confirms the address, marks the token used and sends the
person to sign-in. Audited as `identity.email_verified`. `/verify/resend` issues a new token,
answers generically, and is limited per IP and per address.

**Sign-in** (`GET/POST /signin`): limited per IP and per account address, which is what stops a
credential-stuffing run. On success: a new session (never a reused one), a new CSRF secret, which
closes session fixation and sign-in CSRF, and the cookie `__Host-session` (HttpOnly, Secure,
SameSite=Lax, path `/`), which is bound to that marketplace's host by the prefix. The hash is
recomputed if its parameters are outdated (D5). Audited as `identity.signin`; a failure against an
existing account is audited as `identity.signin_failed`, with the platform as the actor, so the
address the attempt came from is not sealed under the account owner's key; one against an unknown
address is only logged.

**The session middleware** runs after marketplace and language resolution. It reads the cookie,
looks up the hash in a transaction for that marketplace, and refuses a session that is revoked, has
been unused for 30 days, or is older than 90. `last_seen_at` is written at most once an hour, so a
click does not cost a write. The signed-in account is in the request context.

**The retention sweep** (owner's decision, 2026-09-23) removes a session row once it can never
authenticate again: created more than 90 days ago, unseen for more than 30, or revoked more than 7
days ago (`RevokedKept`, long enough for a sessions screen or a support question about a sign-out
that just happened). The row is deleted rather than only marked, because it keeps `ip` and
`user_agent` in clear (LGPD's necessity principle) and the audit log already keeps the sealed
`identity.signin`/`identity.signout` records as the access record. `SignIn` triggers it after the
new session is written, at most once an hour per process (an in-memory timestamp on `Service`), in
its own transaction so a sweep failure never rolls the sign-in back; it runs as the application role
for the signing-in marketplace, so row-level security scopes it there.

**Sign-out** (`POST /signout`): revokes the session in the database and clears the cookie. Audited
as `identity.signout`.

**Password change** (`GET/POST /account/password`): the current password is required and the new
one follows D4. The credential is replaced, every session of the account is revoked, and a
`password-changed` mail tells the person it happened. The browser that changed the password stays
signed in, on a new session token with a renewed CSRF secret (OWASP Session Management: renew the
session identifier after a password change), so a stolen copy of its old cookie ends too; the answer
redirects to `/account/password?changed=1`, which shows that the change is done. Audited as
`identity.password_changed`.

Every page, message and e-mail exists in en-US and pt-BR; the parity check and the literal check
already in the repository enforce it.

## Testing

**Two gaps close first**, because today the end-to-end suite runs with no database and nothing
delivers mail locally:

- The e2e fixture starts a local PostgreSQL 16, the same way `make integration` does when
  `TEST_DATABASE_URL` is unset, runs `migrate` with test marketplaces on `m1.localhost` and
  `m2.localhost` (which Chromium resolves to the loopback address without configuration), and
  starts the binary against it.
- With `PROVIDERS_MODE=fake` and no Cloud Tasks queue configured, and only then, the outbox
  dispatcher runs inside the process every second, so the verification e-mail reaches the fake
  mailbox the way a real one reaches Brevo.

**Unit:** the argon2id parameters and PHC round trip; the token hash round trip (a token is stored
and found only as its SHA-256) and password verification, which compares with
`crypto/subtle.ConstantTimeCompare`; the password rules; e-mail normalisation; the `breached` fake
and the range-response parser.

**Integration** (real PostgreSQL, as the application role under RLS): a revoked session is refused
on the next request; a password change ends every session and keeps the browser that made it
signed in on a new one; an unconfirmed account cannot sign in; the limiter blocks a
credential-stuffing pattern; an account and a session of one marketplace are invisible from the
other.

**End-to-end:** in two browser contexts, sign up, read the link from the fake mailbox, confirm,
sign in on both, change the password on one, and assert the other is signed out. Screenshots of
every screen in both languages.

## Pull requests

Each is green and useful on its own.

1. **A database and local mail in the end-to-end suite.** No visible change. It also carries the
   comment in `cmd/marketplace/main.go` recording that the database ping before `Listen` was
   re-examined under the sleep schedule on 2026-09-23 and kept.
2. **Accounts.** The `account`, `credential` and `email_verification` tables; sign-up, verification
   and resend; the `breached` port with the Pwned Passwords adapter and its fake; argon2id with the
   provisional parameters; named database rate limiters, each forgetting only its own windows, so
   an hourly limit is not reset by a ten-minute one; `docs/design.md` §2.2 names the new port.
3. **Sessions.** The `session` table, the middleware, sign-in and sign-out, the rate limits and the
   audit records, and the `bench-password` command, so that it is deployed when this pull request
   merges.
4. **Password change.** Revocation of the other sessions, the notification mail, the two-browser
   end-to-end test, and the final argon2id parameters from the owner's benchmark run.

**One step for the owner, between the third's merge and the fourth:** running the argon2id
benchmark once on the lab's Cloud Run CPU, by executing the existing migration job with the argument
`bench-password`. Only `main` deploys, so the command reaches the lab when the third pull request
merges; the fourth sets the parameters from the run's result. The command is given when it is
needed.

## Out of this delivery

The sessions screen (§18.1 allows it after the first release; the revocation it needs is here),
e-mail change (it needs F14's step-up), second factors (F14), staff sign-in, roles and the owner
bootstrap (F15), password recovery (F16), and durations as parameters (F17).

## Risks

**The Pwned Passwords service changes or disappears.** The port isolates it, and the fail-open
rule means an outage costs a weaker check, not a broken sign-up.

**Argon2id memory under load.** Bounded by the semaphore; the benchmark measures time on the real
CPU, and the semaphore's size is chosen with the instance's memory in view.

**A future parameter raise unbalances the dummy's timing.** `Hasher.Waste` hashes against a dummy
made with the hasher's own current parameters (D7), so a hash made with older, cheaper parameters
costs less to verify than that dummy does: after a future parameter raise, a wrong password on a
dormant account (one still holding a hash from before the raise) answers faster than an unknown
address. No account exists in production before this delivery, so every real account starts on
`identity.Current` as raised here; the gap is a consequence of the next raise, handled by whoever
next raises `identity.Current`.

**A marketplace host that is not under `__Host-`'s rules.** The prefix requires HTTPS and no
`Domain` attribute; every marketplace host is served over HTTPS, and the local suite uses the
non-prefixed name the same way the CSRF cookie already does.

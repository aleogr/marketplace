# F14 — Two-factor authentication: design

**Date:** 2026-09-25
**Status:** implemented on 2026-09-25, in aleogr/marketplace #81 (design and plan), #82 and #83
(the app), #84 (the two-step sign-in, the step-up and e-mail codes), #85 and #86 (keys) and #PR4
(the notices, the failure counter and the closing)
**Delivers:** roadmap F14 (`docs/roadmap.md`), against `docs/requirements.md` §18.2 and
`docs/design.md` §4 and item 5 of its decisions list.

A person who has an account adds a second factor: an authenticator app, a security key or the
device's own authenticator, or codes sent by e-mail. From then on, signing in asks for it after the
password, and the actions that could take the account over ask for it again. Recovery codes are
generated when the first app or key is enrolled. The per-user-type policy of §18.2 is a service that
every one of these paths consults. Account recovery by e-mail (F16), staff resetting a colleague's
factor (F15) and the step-ups of checkout and e-mail change belong to later deliveries; the hooks
they need are delivered here.

## What exists and what that means for this delivery

F13 created buyer accounts only. Staff sign in from F15, and stores arrive in a later phase. The
person-visible part of F14 is therefore a buyer enrolling a second factor and being asked for it.
The staff and store rules of §18.2 ("staff only with an app or a key, never an e-mail code", "stores
stepped up on sensitive actions") are implemented in the policy service now and tested with
accounts of each kind; they take effect when those users can sign in.

## Decisions

**D1. WebAuthn accepts any authenticator, not only physical keys.** A YubiKey, a phone's or a
laptop's fingerprint, face or PIN, and passkeys kept on a phone are all accepted as a second factor.
They are the same class of assurance, and few buyers own a physical key. The owner chose this on
2026-09-25; `docs/requirements.md` §18.2 now says "security keys and the device's own
authenticators". Attestation is not required (`attestation: none`), as the large services do, so any
authenticator works; user verification is `preferred`. Passwordless sign-in with a passkey is not in
this delivery: WebAuthn is a second factor here.

**D2. Whoever has a second factor is asked for it again before changing the password or changing
their factors.** The owner chose this on 2026-09-25. It closes the path where a stolen session
changes the password and takes the account even though the account has 2FA. Adding a card and
changing the e-mail address keep their §18.2 step-up by e-mail code; neither action exists yet, and
the step-up they will call is delivered here. `docs/requirements.md` §18.2 records the rule.

**D3. Sign-in in two steps uses a challenge separate from the session.** The owner chose this over
a half-authenticated session and over a signed cookie on 2026-09-25. A correct password on an
account with a second factor creates a `sign_in_challenge`: a random token in its own cookie, stored
as its SHA-256, valid for 5 minutes, at most 5 attempts, used once. The session is created only when
the second factor is accepted, and a password change ends the account's open sign-in challenges,
since the second step does not check the password again. The `session` table keeps holding complete
sessions only, so the F13 middleware and everything that reads a session stay as they are; there is
no half-session to forget to check. A signed stateless cookie was rejected because it can neither
count attempts nor be revoked.

**D4. A step-up lasts ten minutes.** The session gains `stepped_up_at`, set when a second factor is
accepted at sign-in or at a step-up. A sensitive action requires it to be less than ten minutes old,
as GitHub's "sudo mode" does, so that one change does not ask twice. A sensitive action on a session
without a recent step-up goes to the same challenge page and comes back. An e-mail code that is not one
of the account's enrolled factors proves the address, for adding a card or changing the address, and
never a step-up before changing the password or the factors: it sets `email_confirmed_at`, not
`stepped_up_at`.

**D5. Secrets are sealed with the person's own key.** The TOTP secret is encrypted with the per-person
key of the audit log (F12, `internal/platform/audit`), which is wrapped by Cloud KMS. Destroying the
key for an erasure request makes the secret unreadable along with the audit records, with nothing
else to find. The audit package exposes seal and open for this; it does not change how the log uses
the key. WebAuthn stores a public key and a counter, which are not secrets. E-mail codes and challenge
tokens are stored only as SHA-256. Recovery codes are stored as a SHA-256 bound to the account (of
the account id and the code), so the stored value is not the plain hash of the code (owner's
decision, 2026-09-25).

**D6. Standard, widely supported parameters.**
- TOTP (RFC 6238): HMAC-SHA-1, 6 digits, 30-second steps, 160-bit secrets, the only parameters every
  authenticator app accepts. A code is accepted for the previous, current or next step (±30 s of
  clock drift) and never for a step at or before the last one accepted, which is stored.
- WebAuthn: the relying-party identifier is the marketplace's host, so a credential enrolled on one
  marketplace does not work on another, which matches accounts belonging to a marketplace (F13 D2).
  The sign counter is checked; a counter that does not move forward, other than both being zero (an
  authenticator that keeps no counter), refuses the assertion and is audited as a possible clone
  (WebAuthn §7.2).
- E-mail codes: 6 digits, 10 minutes, at most 5 attempts, one purpose each (sign-in, step-up,
  enrolment), at most 1 sent per minute and 5 per hour per account.
- Recovery codes: 10 codes of the form `xxxx-xxxx-xxxx` (about 60 bits each), shown once, used once;
  generating new ones invalidates the old.

**D7. Libraries.** WebAuthn uses `github.com/go-webauthn/webauthn`, the reference Go implementation:
writing the protocol (CBOR, COSE keys, client-data checks) by hand is not good practice. TOTP is
implemented in the package, about forty lines over `crypto/hmac`, and tested against the RFC 6238
vectors; a dependency would add more than it saves. The enrolment QR code uses `rsc.io/qr`, rendered
as a PNG data URI (the CSP already allows `img-src data:`).

**D8. Consecutive failed second factors are counted, announced and, at a hundred, stop the codes.**
The owner chose this on 2026-09-25, and confirmed on 2026-09-26 that the lock applies at the
step-up as well as at sign-in (a stolen session could otherwise keep guessing there). Without it,
someone who has the password is bounded only by the shared per-address sign-in limit (10 attempts
per 15 minutes): about 770 guesses a day at a six-digit code, roughly 0.2% a day and even odds
within a year, and the person never knows. NIST SP 800-63B §5.2.2 caps consecutive failed attempts
on one account at 100. So, per account:
- every failed answer to the second step or to a step-up with a second factor counts — a wrong app
  or e-mail-factor code, a wrong recovery code, a refused key; a challenge that is unknown, expired
  or used does not, because no answer was checked, and neither does a code sent to the account's
  address when e-mail is not one of its factors, which proves the address and not a second factor
  (D4);
- any accepted second factor, at sign-in or at a step-up, ends the run, and so does a password
  change;
- at the 10th failure in a row the person is told by e-mail (`second-factor-failures`) that someone
  who knows the password, or is signed in to the account, keeps failing its second factor, and to
  change the password if it was not them; audited as `identity.second_factor_failures_notified`;
- at the 100th consecutive failure, codes from an authenticator app or by e-mail are refused at
  sign-in and at the step-up until the password is changed or a key or a recovery code is accepted
  (either lifts it): the challenge no longer offers them, an answer with one is refused without
  being checked, and the page says so while still offering a key and a recovery code. The person is
  told by e-mail (`second-factor-locked`); audited as `identity.second_factor_locked`.

Each notice is sent once per run, when the count becomes exactly 10 or exactly 100. The count lives
in its own table, `second_factor_failure`, rather than on `account`: an answer locks its challenge
and then the factor rows, while removing a factor locks the account and then the factor rows, so
writing the account after the factor rows could close a lock cycle. Every path takes the count's row
last.

## Data model

Under row-level security like every tenant table (`current_marketplace_id()`); every row carries
`marketplace_id`.

| table | columns that matter |
|---|---|
| `second_factor` | `id`, `account_id`, `kind` (`totp`, `webauthn`, `email`), `label` (given by the person), `secret` (TOTP, sealed per D5), `totp_last_step`, `credential_id` (unique), `public_key`, `sign_count`, `credential_flags` (the authenticator's backup flags at registration, which go-webauthn checks at every assertion), `created_at`, `last_used_at` |
| `recovery_code` | `account_id`, `code_hash`, `used_at` |
| `email_code` | `account_id`, `purpose`, `code_hash`, `expires_at`, `attempts`, `used_at` |
| `sign_in_challenge` | `token_hash`, `account_id`, `session_id` and `action` (set when the challenge is a step-up, so only that session can answer it), `expires_at`, `attempts`, `used_at`, `webauthn_session` (the WebAuthn challenge) |
| `factor_enrolment` | the pending enrolment between showing the page and the proving answer: an app's sealed secret or a key's registration ceremony, so the server, not the browser, chooses the secret; short-lived, one per session |
| `second_factor_failure` | `account_id`, `failures` (the run of failed second factors, D8), `updated_at`; no row is a run of zero |
| `session` (F13) | gains `stepped_up_at`, and `email_confirmed_at` (D4) |

An account "has 2FA" when it has at least one `second_factor`. Removing the last one turns 2FA off
and deletes the recovery codes.

**Known limit, stated:** a 6-digit code stored as its SHA-256 can be brute-forced by someone holding
the database; the exposure is bounded by the code's ten-minute life, and online guessing by the five
attempts. This is the market's standard for short one-time codes. Recovery codes are long and random,
but a plain SHA-256 of sixty random bits is still a multi-target search across a stolen database:
with N accounts of ten codes each, finding some account's code takes about 2^60/(10·N) hashes, which
falls as the marketplace grows. Binding the stored hash to the account (of the account id and the
code) forces a search per account instead, about 2^60/10 hashes each whatever N is; decided with the
owner on 2026-09-25.

**Known limit, stated (D8):** an account whose codes are locked and whose only second factors are an
app or e-mail, with no recovery code left, cannot confirm at sign-in or at a step-up, so it can
neither sign in nor change its password: it waits for account recovery (F16). Someone who has the
password can also lock an account's codes on purpose, and so can someone holding a signed-in session
of the account, through failed step-ups, without knowing the password; the person is told at the
10th and the 100th failure, a key or a recovery code still works and lifts the lock, and so does a
new password. The count is read without a lock when a challenge is checked, so an answer already
being checked when another reaches the 100th is still checked; each needs a challenge of its own,
which needs the password and the sign-in limit, or a signed-in session and the step-up limit.

## Flows

**Security page** (`/{lang}/account/security`, signed in): lists the methods with their labels and
last use, adds and removes them, and regenerates recovery codes. Every change there needs a recent
step-up when the account already has 2FA (D2, D4).

**Enrolment.**
- App: the page shows the QR code and the key in text; the method becomes active only when the
  person types a current code, which proves the app was set up.
- Key or device: the browser asks for a touch or a fingerprint; the person names the method.
- E-mail: a code is sent to the account's address; typing it activates the method.
- The first app or key enrolled shows the ten recovery codes once, with the instruction to keep them.

**Sign-in with 2FA.** The password step answers as in F13 (D7 of F13 still holds: an unknown address
and a wrong password look alike). A correct password on an account with 2FA creates the challenge
and shows the second step, strongest method first (key, then app, then e-mail), with "use another
method" and "use a recovery code". A correct factor creates the session exactly as F13's sign-in
does (a new token and a new CSRF secret) and audits which method was used. A wrong one counts an attempt; the fifth
destroys the challenge and sends the person back to the password. The F13 per-address limit covers
the password and the second step together.

**Step-up.** Changing the password (D2), adding or removing a method and regenerating recovery codes
go through the same challenge page when the session has no recent step-up, then return to where the
person was. The step-up's challenge is bound to the session that asked for it, and a named rate
limiter (`step-up`, 10 answers per account per 15 minutes) stops a stolen session from opening
challenge after challenge.

**Removal and recovery codes.** Removing a method sends an e-mail saying so. A recovery code used at
sign-in or at a step-up sends an e-mail saying which, and, at sign-in, shows on the next page how
many remain.

**Policy** (`identity.Policy`, one service every path consults):
- Buyers: 2FA optional, any method.
- Staff: required, app or key only; an e-mail code is refused at enrolment and at verification,
  because e-mail is their recovery channel. Takes effect when staff sign in (F15).
- Stores: the step-up hook for their sensitive actions, with the e-mail code as the default for a new
  store. Takes effect when stores exist.
- The step-up entry points for adding a card (checkout) and changing the e-mail address exist and are
  tested; the actions that call them do not exist yet.

**JavaScript.** WebAuthn needs `navigator.credentials`, so the site gets its first script: one small
file served by the site, loaded with the page's nonce under the existing CSP (`script-src 'self'
'nonce-…'`), no inline script. Without JavaScript, the app and the e-mail code still work.

**Mail** (en-US and pt-BR, text and HTML): the code (saying what it is for, and kept out of the
subject), method added, method removed, recovery code used (saying whether it signed in or
confirmed a change), repeated failures of the second step and codes locked (D8).
The code travels through the outbox, whose variables are cleared once dispatched (migration 00010),
so it does not stay in the database after delivery.

**Audit:** enrolment, removal, sign-in with a second factor (naming the method), step-up, an address
proved by a code sent to it that is not a second factor (`identity.email_confirmed`, apart from the
step-up's `identity.stepped_up`, D4), recovery code used, recovery codes regenerated, a failed second
factor, a WebAuthn counter that did not move forward, the failures notice and the codes locked (D8).

Every page, message and e-mail exists in en-US and pt-BR.

## Testing

**Unit:** TOTP against the RFC 6238 vectors; a replayed code refused and ±1 step accepted; an expired
e-mail code refused and the attempt limit; a recovery code usable exactly once; every combination of
the policy (user kind × method × action).

**Integration** (real PostgreSQL, as the application role under RLS): a staff account cannot enrol an
e-mail code; an expired or exhausted challenge is refused; second factors are invisible from another
marketplace; a step-up older than ten minutes is refused; removing the last method turns 2FA off and
deletes the recovery codes; the TOTP secret is unreadable once the person's key is destroyed.

**End-to-end:** the TOTP path with the test computing the code; the WebAuthn path with a **virtual
authenticator** driven through the Chrome DevTools Protocol; the e-mail path through the fake
mailbox; sign-in with a recovery code; a password change asking for the second factor. Screenshots
of every flow in both languages.

## Pull requests

Each is green and useful on its own.

1. **Foundation and the authenticator app.** The tables, sealing with the person's key, the policy,
   TOTP, recovery codes, and the security page with enrolment and removal of an app.
2. **Two-step sign-in and step-up.** The challenge, the step-up (including the password change of
   D2), and e-mail codes.
3. **WebAuthn.** Keys and device authenticators, the script, the virtual authenticator in the tests.
4. **Closing.** The notification mails, screenshots, the roadmap's F14 ticked.

**Nothing is needed from the owner.** Trying a physical key or a phone's fingerprint on the lab is
optional; the automated tests cover the path with a virtual authenticator.

## Out of this delivery

Account recovery by e-mail with a 24-hour delay (F16); staff resetting a colleague's factor (F15);
the step-ups of adding a card (checkout) and changing the e-mail address, whose hooks are here;
passwordless sign-in with a passkey; "remember this device" (sessions already last 30 days idle, so
the second factor is asked rarely); a screen listing sessions.

## Risks

**A person loses every factor and every recovery code before F16.** They cannot sign in until account
recovery arrives. In the lab this is a test account; F16 is the next deliveries' work, and the
security page tells the person to keep the recovery codes.

**WebAuthn across browsers.** The virtual authenticator covers the protocol; a real key or phone can
behave differently. The library is the reference implementation, and the owner can try a real device
on the lab when convenient.

**The first script on the site.** It widens what the CSP allows only by one self-hosted file under a
nonce; no inline script and no third-party origin.

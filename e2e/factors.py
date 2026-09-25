"""Second factors, as a person sets them up and uses them, for the tests."""

from __future__ import annotations

import base64
import hashlib
import hmac
import json
import struct
import time


def totp(key: str, at: float | None = None) -> str:
    """The code an authenticator app set up with key shows at a moment: RFC
    6238 with SHA-1, six digits and thirty-second steps, computed here and not
    asked of the service."""
    compact = key.replace(" ", "")
    secret = base64.b32decode(compact + "=" * (-len(compact) % 8))
    counter = int((time.time() if at is None else at) // 30)
    mac = hmac.new(secret, struct.pack(">Q", counter), hashlib.sha1).digest()
    offset = mac[-1] & 0x0F
    value = struct.unpack(">I", mac[offset:offset + 4])[0] & 0x7FFFFFFF
    return f"{value % 1_000_000:06d}"


def next_totp(key: str) -> str:
    """The code of the next step. The service never accepts a step twice
    (spec, D6), so a test that has just used this step's code answers with
    the next one, which the one-step drift accepts."""
    return totp(key, time.time() + 30)


# The submit button of the form that takes a code. The challenge page also
# carries the form that sends an e-mail code, whose button comes first.
CODE_SUBMIT = "main form:has(input[name=code]) button[type=submit]"

# The header's sign-out button.
SIGN_OUT = "nav.account button[type=submit]"


def enrol_app(page, marketplace, language="pt-BR", label="Celular", host="m1.localhost"):
    """Add an authenticator app from the security page, typing the code the
    test computes. Returns the key and the recovery codes shown."""
    page.goto(marketplace.url(host, f"/{language}/account/security/app"))
    key = page.locator("#totp-key").inner_text()
    page.fill("main form input[name=label]", label)
    page.fill("main form input[name=code]", totp(key))
    page.click(CODE_SUBMIT)
    codes = page.locator("ol.codes code").all_inner_texts()
    return key, codes


def mails(mailbox, template, to):
    """Every message of template sent to an address, oldest first."""
    found = []
    for path in mailbox.glob("*.json"):
        message = json.loads(path.read_text())
        if message["template"] == template and message["to"] == to.lower():
            found.append(message)
    return sorted(found, key=lambda m: m["written"])


def wait_for_new_mail(mailbox, template, to, seen, timeout=10.0):
    """The first message of template to an address after the seen ones."""
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        found = mails(mailbox, template, to)
        if len(found) > seen:
            return found[seen]
        time.sleep(0.2)
    raise AssertionError(f"no new {template} mail to {to} in {mailbox}")


def a_minute_passes(database, email):
    """Move the codes already sent to an address a minute into the past. The
    service sends an account one code a minute (spec, D6); a test that needs
    a second moves the first back rather than waiting for it."""
    import psycopg

    with psycopg.connect(database.owner_url, autocommit=True) as conn:
        conn.execute(
            "UPDATE email_code SET created_at = created_at - interval '2 minutes'"
            " WHERE account_id IN (SELECT id FROM account WHERE email_normalised = %s)",
            (email.lower(),))


def virtual_authenticator(page):
    """A security key inside Chromium, driven through the DevTools Protocol:
    it answers every WebAuthn ceremony of this page as a real key would,
    touch and fingerprint included, with nobody there to touch it."""
    cdp = page.context.new_cdp_session(page)
    cdp.send("WebAuthn.enable")
    added = cdp.send("WebAuthn.addVirtualAuthenticator", {"options": {
        "protocol": "ctap2", "transport": "usb", "hasResidentKey": False,
        "hasUserVerification": True, "isUserVerified": True,
        "automaticPresenceSimulation": True,
    }})
    return cdp, added["authenticatorId"]

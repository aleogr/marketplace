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

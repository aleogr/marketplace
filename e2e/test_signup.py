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

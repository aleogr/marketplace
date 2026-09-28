"""The console's first run, against a fresh database (F15).

The owner is created once, through /setup on the console's own host, with the
token Terraform would have generated; the console then asks for an app or a
key before anything else, shows the recovery codes, and only then opens, with
the build identifier in its footer and on /console/version. Buyers never sign
in there, and the owner never on a store
(docs/superpowers/specs/2026-09-26-f15-console-design.md).
"""

from __future__ import annotations

import json
import re
import urllib.request

import pytest
from playwright.sync_api import expect, sync_playwright

from accounts import SIGNED_IN, confirmed_account, sign_in
from console import CONSOLE_HOST, OWNER, OWNER_NAME, first_run
from factors import CODE_SUBMIT, SIGN_OUT, next_totp, totp

# console.setup.wrong_token, identity.signin.failed, console.enrol.title and
# console.home.title, as web/locales words them.
WRONG_TOKEN = {
    "pt-BR": "Este não é o token de inicialização.",
    "en-US": "This is not the bootstrap token.",
}
REFUSED = {
    "pt-BR": "O e-mail ou a senha não conferem.",
    "en-US": "The e-mail or the password is not right.",
}
ENROL = {
    "pt-BR": "Adicione um segundo fator",
    "en-US": "Add a second factor",
}
BUILD = {
    "pt-BR": "Versão: ",
    "en-US": "Build: ",
}


def health_version(console) -> str:
    """The build the process says it is, from its health check."""
    with urllib.request.urlopen(console.base_url + "/health", timeout=10) as response:
        return json.load(response)["version"]


@pytest.mark.local_process
@pytest.mark.parametrize("language", ["pt-BR", "en-US"])
def test_the_first_run_creates_the_owner_and_opens_the_console(run_console, screenshots, language):
    console = run_console()
    version = health_version(console)
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()

        # The first run: a wrong token is refused on its field.
        page.goto(console.url(CONSOLE_HOST, f"/{language}/setup"))
        page.screenshot(path=screenshots / f"f15-setup-{language}.png", full_page=True)
        first_run(page, console, language, token="not-the-token")
        expect(page.locator("[role=alert]")).to_have_text(WRONG_TOKEN[language])
        expect(page.locator("input[name=token]")).to_be_focused()
        page.screenshot(path=screenshots / f"f15-setup-wrong-token-{language}.png", full_page=True)

        # The right one creates the owner and asks for a second factor
        # before anything else.
        first_run(page, console, language)
        expect(page).to_have_url(re.compile(rf"/{language}/enrol$"))
        expect(page.locator("main h1")).to_have_text(ENROL[language])
        expect(page.locator("nav.menu")).to_have_count(0)
        page.screenshot(path=screenshots / f"f15-enrol-{language}.png", full_page=True)
        page.goto(console.url(CONSOLE_HOST, f"/{language}/"))
        expect(page).to_have_url(re.compile(rf"/{language}/enrol$"))

        page.click("main a[href$='/account/security/app']")
        key = page.locator("#totp-key").inner_text()
        page.fill("main form input[name=label]", "Celular")
        page.fill("main form input[name=code]", totp(key))
        page.click(CODE_SUBMIT)
        expect(page.locator("ol.codes code")).to_have_count(10)
        page.screenshot(path=screenshots / f"f15-recovery-codes-{language}.png", full_page=True)

        # The console, with the owner's name, the menu and the build.
        page.goto(console.url(CONSOLE_HOST, f"/{language}/"))
        expect(page.locator(SIGNED_IN)).to_contain_text(OWNER_NAME)
        expect(page.locator("nav.menu a[aria-current=page]")).to_have_count(1)
        expect(page.locator("footer")).to_have_text(BUILD[language] + version)
        page.screenshot(path=screenshots / f"f15-console-{language}.png", full_page=True)
        page.goto(console.url(CONSOLE_HOST, "/console/version"))
        assert json.loads(page.locator("body").inner_text()) == {"version": version}

        # The first run is gone.
        answer = page.goto(console.url(CONSOLE_HOST, f"/{language}/setup"))
        assert answer.status == 404, answer.status

        # Signing in again asks for the app.
        page.goto(console.url(CONSOLE_HOST, f"/{language}/"))
        page.click(SIGN_OUT)
        sign_in(page, console, OWNER, host=CONSOLE_HOST, language=language)
        expect(page).to_have_url(re.compile(r"/signin/verify$"))
        page.screenshot(path=screenshots / f"f15-second-step-{language}.png", full_page=True)
        page.fill("main form input[name=code]", next_totp(key))
        page.click(CODE_SUBMIT)
        expect(page).to_have_url(re.compile(rf"/{language}/$"))
        expect(page.locator(SIGNED_IN)).to_contain_text(OWNER_NAME)
        browser.close()


@pytest.mark.local_process
def test_buyers_and_staff_each_sign_in_only_on_their_own_host(run_console):
    console = run_console()
    buyer = "leitora-console@example.test"
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        first_run(page, console)
        expect(page).to_have_url(re.compile(r"/pt-BR/enrol$"))

        other = browser.new_page()
        # The owner's address and password open nothing on a store.
        sign_in(other, console, OWNER)
        expect(other.locator("[role=alert]")).to_have_text(REFUSED["pt-BR"])
        # A buyer's open nothing on the console.
        confirmed_account(other, console, buyer)
        sign_in(other, console, buyer, host=CONSOLE_HOST)
        expect(other.locator("[role=alert]")).to_have_text(REFUSED["pt-BR"])
        browser.close()

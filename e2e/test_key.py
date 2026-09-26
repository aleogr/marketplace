"""A security key, added and used to sign in, with a virtual authenticator (F14)."""

from __future__ import annotations

import re

import pytest
from playwright.sync_api import expect, sync_playwright

from accounts import SIGNED_IN, confirmed_account, sign_in, wait_for_mail
from factors import SIGN_OUT, virtual_authenticator


@pytest.mark.local_process
@pytest.mark.parametrize("language", ["pt-BR", "en-US"])
def test_adding_a_key_and_signing_in_with_it(run_marketplace, screenshots, language):
    marketplace = run_marketplace()
    email = f"chave-{language}@example.test"
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        # The script runs under the page's Content Security Policy: nothing
        # it does may be refused, and the console says so when it is.
        errors = []
        page.on("console", lambda message: errors.append(message.text) if message.type == "error" else None)
        cdp, authenticator = virtual_authenticator(page)
        confirmed_account(page, marketplace, email, language=language)
        sign_in(page, marketplace, email, language=language)

        page.goto(marketplace.url("m1.localhost", f"/{language}/account/security"))
        page.click("main a[href$='/account/security/key']")
        page.screenshot(path=screenshots / f"f14-key-{language}.png", full_page=True)
        page.fill("main form input[name=label]", "YubiKey")
        page.click("main form[data-webauthn] button[type=submit]")
        # The first key shows the recovery codes, as the first app does.
        expect(page.locator("ol.codes code")).to_have_count(10)
        page.click("main a[href$='/account/security']")
        expect(page.locator("ul.factors li strong")).to_have_text("YubiKey")
        added = wait_for_mail(marketplace.mailbox, "second-factor-added", email)
        assert added["variables"]["Method"] == "webauthn" and added["language"] == language, added
        page.screenshot(path=screenshots / f"f14-security-key-{language}.png", full_page=True)

        page.click(SIGN_OUT)
        sign_in(page, marketplace, email, language=language)
        expect(page).to_have_url(re.compile(r"/signin/verify$"))
        expect(page.locator("main form[data-webauthn=get]")).to_be_visible()
        page.screenshot(path=screenshots / f"f14-second-step-key-{language}.png", full_page=True)
        page.click("main form[data-webauthn] button[type=submit]")
        expect(page.locator(SIGNED_IN)).to_be_visible()

        # The authenticator signed twice: once to register, once to sign in.
        credentials = cdp.send("WebAuthn.getCredentials", {"authenticatorId": authenticator})["credentials"]
        assert len(credentials) == 1 and credentials[0]["signCount"] >= 1, credentials
        assert not errors, errors
        browser.close()

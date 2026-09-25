"""An authenticator app, added and removed from the security page (F14)."""

from __future__ import annotations

import re

import pytest
from playwright.sync_api import expect, sync_playwright

from accounts import SUBMIT, confirmed_account, sign_in
from factors import next_totp, totp

# identity.app.wrong_code and identity.security.done.removed, as
# web/locales words them.
WRONG_CODE = {
    "pt-BR": "Esse código não confere.",
    "en-US": "That code is not right.",
}
REMOVED = {
    "pt-BR": "O segundo fator foi removido.",
    "en-US": "The second factor was removed.",
}


@pytest.mark.local_process
@pytest.mark.parametrize("language", ["pt-BR", "en-US"])
def test_adding_and_removing_an_app(run_marketplace, screenshots, language):
    marketplace = run_marketplace()
    email = f"app-{language}@example.test"
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        confirmed_account(page, marketplace, email, language=language)
        sign_in(page, marketplace, email, language=language)

        page.goto(marketplace.url("m1.localhost", f"/{language}/account/security"))
        page.screenshot(path=screenshots / f"f14-security-empty-{language}.png", full_page=True)
        page.click("main a[href$='/account/security/app']")
        expect(page.locator("img.qr")).to_be_visible()
        key = page.locator("#totp-key").inner_text()
        page.screenshot(path=screenshots / f"f14-app-{language}.png", full_page=True)

        # A wrong code is refused, the cursor goes back to the code, and the
        # same key is still the one to type.
        page.fill("main form input[name=label]", "Celular")
        page.fill("main form input[name=code]", "000000")
        page.click(SUBMIT)
        expect(page.locator("[role=alert]")).to_contain_text(WRONG_CODE[language])
        expect(page.locator("main form input[name=code]")).to_be_focused()
        assert page.locator("#totp-key").inner_text() == key
        page.screenshot(path=screenshots / f"f14-app-wrong-{language}.png", full_page=True)

        page.fill("main form input[name=code]", totp(key))
        page.click(SUBMIT)
        codes = page.locator("ol.codes code").all_inner_texts()
        assert len(codes) == 10 and all(re.fullmatch(r"[0-9a-z]{4}-[0-9a-z]{4}-[0-9a-z]{4}", c) for c in codes), codes
        page.screenshot(path=screenshots / f"f14-recovery-codes-{language}.png", full_page=True)

        page.click("main a[href$='/account/security']")
        expect(page.locator("ul.factors li")).to_have_count(1)
        expect(page.locator("ul.factors li strong")).to_have_text("Celular")
        page.screenshot(path=screenshots / f"f14-security-app-{language}.png", full_page=True)

        # The account has 2FA now: removing the app asks for it again first,
        # and comes back (spec, D2).
        page.click("ul.factors button[type=submit]")
        expect(page).to_have_url(re.compile(r"/account/verify\?for=factors&next=%2Faccount%2Fsecurity$"))
        page.screenshot(path=screenshots / f"f14-step-up-factors-{language}.png", full_page=True)
        page.fill("main form input[name=code]", next_totp(key))
        page.click(SUBMIT)
        expect(page).to_have_url(re.compile(r"/account/security$"))

        page.click("ul.factors button[type=submit]")
        expect(page.locator("[role=status]")).to_have_text(REMOVED[language])
        expect(page.locator("ul.factors")).to_have_count(0)
        browser.close()

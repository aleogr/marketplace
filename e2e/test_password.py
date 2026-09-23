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

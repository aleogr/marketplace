"""Signing in with a second factor, and being asked for it again (F14)."""

from __future__ import annotations

import re

import pytest
from playwright.sync_api import expect, sync_playwright

from accounts import PASSWORD, SIGNED_IN, SUBMIT, confirmed_account, sign_in
from factors import (CODE_SUBMIT, SIGN_OUT, a_minute_passes, enrol_app, mails, next_totp,
                     wait_for_new_mail)

# identity.challenge.wrong and identity.security.done.recovery_used (nine
# left), as web/locales words them.
WRONG = {
    "pt-BR": "Esse código não confere. Tente de novo.",
    "en-US": "That code is not right. Try again.",
}
NINE_LEFT = {
    "pt-BR": "Você entrou com um código de recuperação. Restam 9",
    "en-US": "You signed in with a recovery code. 9 are left",
}


def second_step(page, marketplace, email, language):
    """Sign out and sign in again with the password, which stops at the
    second step."""
    page.click(SIGN_OUT)
    sign_in(page, marketplace, email, language=language)
    expect(page).to_have_url(re.compile(r"/signin/verify$"))
    expect(page.locator(SIGNED_IN)).to_have_count(0)


@pytest.mark.local_process
@pytest.mark.parametrize("language", ["pt-BR", "en-US"])
def test_signing_in_with_an_app(run_marketplace, screenshots, language):
    marketplace = run_marketplace()
    email = f"duas-etapas-app-{language}@example.test"
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        confirmed_account(page, marketplace, email, language=language)
        sign_in(page, marketplace, email, language=language)
        key, _ = enrol_app(page, marketplace, language=language)

        second_step(page, marketplace, email, language)
        page.screenshot(path=screenshots / f"f14-second-step-app-{language}.png", full_page=True)
        page.fill("main form input[name=code]", "000000")
        page.click(CODE_SUBMIT)
        expect(page.locator("[role=alert]")).to_have_text(WRONG[language])
        expect(page.locator("main form input[name=code]")).to_be_focused()
        page.screenshot(path=screenshots / f"f14-second-step-wrong-{language}.png", full_page=True)

        page.fill("main form input[name=code]", next_totp(key))
        page.click(CODE_SUBMIT)
        expect(page.locator(SIGNED_IN)).to_be_visible()
        browser.close()


@pytest.mark.local_process
@pytest.mark.parametrize("language", ["pt-BR", "en-US"])
def test_signing_in_with_a_recovery_code(run_marketplace, screenshots, language):
    marketplace = run_marketplace()
    email = f"duas-etapas-recuperacao-{language}@example.test"
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        confirmed_account(page, marketplace, email, language=language)
        sign_in(page, marketplace, email, language=language)
        _, codes = enrol_app(page, marketplace, language=language)

        second_step(page, marketplace, email, language)
        page.click("main a[href$='method=recovery']")
        page.screenshot(path=screenshots / f"f14-second-step-recovery-{language}.png", full_page=True)
        page.fill("main form input[name=code]", codes[0].upper())
        page.click(CODE_SUBMIT)
        expect(page.locator(SIGNED_IN)).to_be_visible()
        expect(page.locator("[role=status]")).to_contain_text(NINE_LEFT[language])
        page.screenshot(path=screenshots / f"f14-recovery-used-{language}.png", full_page=True)

        # The same code does not work twice.
        second_step(page, marketplace, email, language)
        page.click("main a[href$='method=recovery']")
        page.fill("main form input[name=code]", codes[0])
        page.click(CODE_SUBMIT)
        expect(page.locator("[role=alert]")).to_have_text(WRONG[language])
        browser.close()


@pytest.mark.local_process
@pytest.mark.parametrize("language", ["pt-BR", "en-US"])
def test_the_password_change_asks_for_the_second_factor(run_marketplace, screenshots, language):
    marketplace = run_marketplace()
    email = f"duas-etapas-senha-{language}@example.test"
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        confirmed_account(page, marketplace, email, language=language)
        sign_in(page, marketplace, email, language=language)
        key, _ = enrol_app(page, marketplace, language=language)

        page.goto(marketplace.url("m1.localhost", f"/{language}/account/password"))
        expect(page).to_have_url(re.compile(r"/account/verify\?for=password&next=%2Faccount%2Fpassword$"))
        page.screenshot(path=screenshots / f"f14-step-up-password-{language}.png", full_page=True)
        page.fill("main form input[name=code]", next_totp(key))
        page.click(CODE_SUBMIT)
        expect(page).to_have_url(re.compile(r"/account/password$"))

        page.fill("input[name=current_password]", PASSWORD)
        page.fill("input[name=new_password]", "a brand new passphrase")
        page.click(SUBMIT)
        expect(page).to_have_url(re.compile(r"/account/password\?changed=1$"))
        expect(page.locator("[role=status]")).to_be_visible()
        browser.close()


@pytest.mark.local_process
@pytest.mark.parametrize("language", ["pt-BR", "en-US"])
def test_signing_in_with_an_email_code(run_marketplace, database, screenshots, language):
    marketplace = run_marketplace()
    email = f"duas-etapas-email-{language}@example.test"
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        confirmed_account(page, marketplace, email, language=language)
        sign_in(page, marketplace, email, language=language)

        page.goto(marketplace.url("m1.localhost", f"/{language}/account/security"))
        page.click("main a[href$='/account/security/email']")
        page.screenshot(path=screenshots / f"f14-email-{language}.png", full_page=True)
        page.click("main form button.secondary")
        code = wait_for_new_mail(marketplace.mailbox, "second-factor-code", email, 0)["variables"]["Code"]
        page.screenshot(path=screenshots / f"f14-email-sent-{language}.png", full_page=True)
        page.fill("main form input[name=code]", code)
        page.click(CODE_SUBMIT)
        expect(page.locator("ul.factors li")).to_have_count(1)

        a_minute_passes(database, email)
        second_step(page, marketplace, email, language)
        page.screenshot(path=screenshots / f"f14-second-step-email-{language}.png", full_page=True)
        seen = len(mails(marketplace.mailbox, "second-factor-code", email))
        page.click("main form button.secondary")
        expect(page.locator("[role=status]")).to_be_visible()
        code = wait_for_new_mail(marketplace.mailbox, "second-factor-code", email, seen)["variables"]["Code"]
        page.fill("main form input[name=code]", code)
        page.click(CODE_SUBMIT)
        expect(page.locator(SIGNED_IN)).to_be_visible()
        browser.close()

"""Sign-in and sign-out, and what a stranger can learn from them: nothing."""

from __future__ import annotations

import pytest
from playwright.sync_api import expect, sync_playwright

from accounts import PASSWORD, SIGN_IN_LINK, SIGNED_IN, SUBMIT, confirmed_account, sign_in


@pytest.mark.local_process
@pytest.mark.parametrize("language", ["pt-BR", "en-US"])
def test_sign_in_and_out(run_marketplace, screenshots, language):
    marketplace = run_marketplace()
    email = f"entra-{language}@example.test"
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        confirmed_account(page, marketplace, email, language=language)
        page.screenshot(path=screenshots / f"f13-signin-{language}.png")
        sign_in(page, marketplace, email, language=language)
        expect(page.locator(SIGNED_IN)).to_be_visible()
        page.screenshot(path=screenshots / f"f13-signed-in-{language}.png")
        page.click("nav.account button[type=submit]")
        expect(page.locator(SIGN_IN_LINK)).to_be_visible()
        expect(page.locator(SIGNED_IN)).to_have_count(0)
        browser.close()


@pytest.mark.local_process
def test_wrong_password_and_unknown_address_read_the_same(run_marketplace):
    marketplace = run_marketplace()
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        confirmed_account(page, marketplace, "existe@example.test")
        sign_in(page, marketplace, "existe@example.test", password="not the right password")
        expect(page.locator("[role=alert]")).to_be_visible()
        wrong = page.locator("[role=alert]").inner_text()
        expect(page.locator("input[name=password]")).to_be_focused()
        sign_in(page, marketplace, "nao-existe@example.test", password="not the right password")
        expect(page.locator("[role=alert]")).to_be_visible()
        unknown = page.locator("[role=alert]").inner_text()
        assert wrong == unknown == "O e-mail ou a senha não conferem."
        browser.close()


@pytest.mark.local_process
def test_an_unconfirmed_account_is_told_to_confirm(run_marketplace):
    marketplace = run_marketplace()
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        page.goto(marketplace.url("m1.localhost", "/pt-BR/signup"))
        page.fill("input[name=name]", "Leitora")
        page.fill("input[name=email]", "pendente@example.test")
        page.fill("input[name=password]", PASSWORD)
        page.click(SUBMIT)
        sign_in(page, marketplace, "pendente@example.test")
        expect(page.locator("[role=alert]")).to_have_text(
            "Confirme seu e-mail primeiro. Podemos reenviar o link.")
        browser.close()


@pytest.mark.local_process
def test_credential_stuffing_against_one_address_is_stopped(run_marketplace):
    """Ten attempts per address in fifteen minutes; the eleventh is refused."""
    marketplace = run_marketplace()
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        confirmed_account(page, marketplace, "alvo@example.test")
        statuses = []
        for attempt in range(11):
            page.goto(marketplace.url("m1.localhost", "/pt-BR/signin"))
            page.fill("input[name=email]", "alvo@example.test")
            page.fill("input[name=password]", f"guess number {attempt:04d}")
            with page.expect_response(lambda r: r.request.method == "POST") as info:
                page.click(SUBMIT)
            statuses.append(info.value.status)
        assert statuses[:10] == [401] * 10 and statuses[10] == 429, statuses
        browser.close()


@pytest.mark.local_process
def test_a_session_does_not_cross_marketplaces(run_marketplace):
    marketplace = run_marketplace()
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        confirmed_account(page, marketplace, "so-no-um@example.test")
        sign_in(page, marketplace, "so-no-um@example.test")
        expect(page.locator(SIGNED_IN)).to_be_visible()
        page.goto(marketplace.url("m2.localhost", "/pt-BR/"))
        expect(page.locator(SIGN_IN_LINK)).to_be_visible()
        expect(page.locator(SIGNED_IN)).to_have_count(0)
        browser.close()

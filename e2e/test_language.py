"""Both languages, and the address says which one.

The language is part of the URL so that each language is its own page to a
search engine, and so that a link a person sends carries the language they were
reading (docs/requirements.md, section 6). These check the rules that only
exist once the whole pipeline is running: the redirects, the remembered choice
and the switch itself.
"""

from __future__ import annotations

import urllib.error
import urllib.request

from playwright.sync_api import sync_playwright


def fetch(url: str, follow: bool = True):
    """Fetch url, optionally stopping at the first redirect."""

    class Stop(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, *args, **kwargs):  # noqa: D102
            return None

    opener = urllib.request.build_opener() if follow else urllib.request.build_opener(Stop)
    try:
        return opener.open(url, timeout=15)
    except urllib.error.HTTPError as error:
        return error


def test_the_root_sends_a_visitor_into_a_language(server):
    """No page is served at an address that does not say its language."""
    response = fetch(f"{server.base_url}/", follow=False)

    assert response.status == 302, response.status
    assert response.headers["Location"] == "/en-US/"


def test_a_lowercase_address_is_corrected_permanently(server):
    """Two spellings of one address are two pages to a search engine, so one
    of them answers permanently with the other (docs/design.md, decision 10)."""
    response = fetch(f"{server.base_url}/en-us/", follow=False)

    assert response.status == 301, response.status
    assert response.headers["Location"] == "/en-US/"


def test_each_language_is_its_own_page(server):
    """The same page, in both languages, each at its own address, each saying
    which language it is in and where the other one is."""
    pages = {}
    for tag in ("en-US", "pt-BR"):
        response = fetch(f"{server.base_url}/{tag}/")
        assert response.status == 200, tag
        pages[tag] = response.read().decode()

        assert f'lang="{tag}"' in pages[tag], f"{tag} does not declare its language"

    # hreflang is what tells a search engine these are translations of each
    # other rather than duplicates competing with each other.
    assert 'hreflang="pt-BR"' in pages["en-US"]
    assert 'hreflang="en-US"' in pages["pt-BR"]
    assert 'hreflang="x-default"' in pages["en-US"]

    assert pages["en-US"] != pages["pt-BR"], "both addresses served the same text"


def test_the_switch_is_remembered_across_sessions(server, screenshots):
    """A visitor's own choice outranks every guess, on this visit and the next.

    The browser is closed between the two halves, keeping only the cookies, as
    a visitor closing their browser and coming back tomorrow would.
    """
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch()

        first = browser.new_context()
        page = first.new_page()
        page.goto(f"{server.base_url}/en-US/")
        page.screenshot(path=str(screenshots / "home-en-US.png"))

        # The switch is a form, because it changes something that is
        # remembered.
        page.get_by_role("button", name="Português").click()
        page.wait_for_url("**/pt-BR/**")
        page.screenshot(path=str(screenshots / "home-pt-BR.png"))

        cookies = first.cookies()
        first.close()

        assert any(cookie["name"] == "language" and cookie["value"] == "pt-BR" for cookie in cookies), (
            f"the choice was not remembered: {cookies}"
        )

        # A new context is a new browsing session; the cookies are what a
        # returning visitor brings with them.
        second = browser.new_context()
        second.add_cookies(cookies)
        returning = second.new_page()
        returning.goto(f"{server.base_url}/")

        assert returning.url.endswith("/pt-BR/"), (
            f"a returning visitor landed on {returning.url}, not on what they chose"
        )

        second.close()
        browser.close()

"""The process answers, says which build it is, and refuses a bad configuration.

These are end-to-end checks against the real binary: the unit tests prove the
same rules in isolation, and these prove the wiring that only exists once the
process is actually started.
"""

from __future__ import annotations

import json
import subprocess
import urllib.error
import urllib.request

from playwright.sync_api import sync_playwright


def get(url: str):
    """Fetch url, returning the response even when the status is an error."""
    try:
        return urllib.request.urlopen(url, timeout=10)
    except urllib.error.HTTPError as error:
        return error


def test_healthz_answers_with_the_running_build(server):
    response = get(f"{server.base_url}/healthz")

    assert response.status == 200
    assert response.headers["content-type"] == "application/json"

    body = json.load(response)
    assert body["status"] == "ok"
    assert body["version"], "the health check carries no build identifier"


def test_the_start_up_log_records_the_build_and_the_indexing_mode(server):
    entry = server.entry("server started")

    assert entry["version"], "the start-up entry carries no build identifier"
    assert entry["indexable"] is False, "the lab must not be indexable by default"
    assert entry["severity"] == "INFO"


def test_an_unreadable_variable_stops_the_process(binary):
    """A typo must be a start-up failure, not a silently wrong default.

    docs/requirements.md, section 7.1: a deployment left non-indexable by a
    misread value is noticed in months, not in days.
    """
    result = subprocess.run(
        [str(binary)],
        env={"INDEXABLE": "ture"},
        capture_output=True,
        text=True,
        timeout=30,
    )

    assert result.returncode != 0, "the process started with INDEXABLE=ture"
    assert "INDEXABLE" in result.stderr, result.stderr
    assert '"ture"' in result.stderr, result.stderr
    assert result.stdout == "", "the process logged before refusing to start"


def test_unknown_paths_are_not_found(server):
    response = get(f"{server.base_url}/no-such-page")

    assert response.status == 404


def test_a_browser_can_reach_the_service(server, screenshots):
    """Prove the Playwright harness itself works, before there are pages.

    The deliveries that add screens rely on this plumbing; finding out then
    that the browser cannot start would cost a delivery, not a test.
    """
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch()
        page = browser.new_page()
        page.goto(f"{server.base_url}/healthz")

        body = json.loads(page.inner_text("body"))
        assert body["status"] == "ok"

        page.screenshot(path=str(screenshots / "healthz.png"))
        browser.close()

"""The host of the request decides which marketplace answers.

These run against the lab, where two hosts exist and a database says which
marketplace each one is. They are skipped anywhere else, and that is not a gap:
a local process runs without a database, so it mounts no host resolution and
would serve every host the same — a test of it there would pass while proving
nothing (docs/roadmap.md, F5).

They are not part of the deployment pipeline either. A newly mapped host waits
for a certificate, which Cloud Run issues in about fifteen minutes and may take
a day, and a deployment must not fail for a wait that was expected. The hosts
to check are named in MARKETPLACE_HOSTS, and the run that names them is the one
that produces the delivery's evidence.

The address no marketplace claims is named separately, in
MARKETPLACE_UNCLAIMED_URL, and that check does run in the pipeline: the
deployment's own `run.app` address is exactly such an address, and it is the
one thing here that needs no certificate to wait for.
"""

from __future__ import annotations

import json
import os
import urllib.request

import pytest
from playwright.sync_api import sync_playwright


def hosts() -> list[str]:
    """The hosts this run was asked to check."""
    declared = os.environ.get("MARKETPLACE_HOSTS", "").strip()
    return [host.strip() for host in declared.split(",") if host.strip()]


needs_the_lab = pytest.mark.skipif(
    not hosts(),
    reason="set MARKETPLACE_HOSTS to the hosts to check, comma separated",
)


@needs_the_lab
def test_every_host_answers_as_the_same_build():
    """One service, many hosts.

    Which marketplace serves a request is decided inside the process, from the
    host it receives; every host reaches the same revision.
    """
    versions = {}
    for host in hosts():
        with urllib.request.urlopen(f"https://{host}/health", timeout=30) as response:
            assert response.status == 200, host
            versions[host] = json.load(response)["version"]

    assert len(set(versions.values())) == 1, (
        f"the hosts answer as different builds: {versions}"
    )


unclaimed = os.environ.get("MARKETPLACE_UNCLAIMED_URL", "").strip()

needs_an_unclaimed_address = pytest.mark.skipif(
    not unclaimed,
    reason="set MARKETPLACE_UNCLAIMED_URL to an address of the service that no "
    "marketplace claims, such as its own run.app address",
)


@needs_an_unclaimed_address
def test_an_address_no_marketplace_claims_is_told_so():
    """The page for a host that belongs to nobody, proved on the deployment.

    It is asked of an address the edge does route to the service but no
    marketplace claims — the deployment's own `run.app` address. Sending an
    unknown `Host:` header to a mapped address proves nothing instead: Google's
    front end routes by that header, and one it has no mapping for is refused
    with Google's own 404 page before the request reaches the binary.
    """
    request = urllib.request.Request(unclaimed.rstrip("/") + "/")
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            raise AssertionError(
                f"an unclaimed address was served with status {response.status}"
            )
    except urllib.error.HTTPError as error:
        assert error.code == 404, error.code
        # Never indexable, whatever the deployment's setting.
        assert error.headers.get("X-Robots-Tag") == "noindex, nofollow", (
            dict(error.headers)
        )
        body = error.read().decode()
        # Both languages, because a request with no marketplace has no language
        # to pick from (docs/requirements.md, section 6).
        assert "does not belong to any marketplace" in body, body
        assert "não pertence a nenhum marketplace" in body, body


@needs_the_lab
def test_a_browser_reaches_every_host(screenshots):
    """The evidence the delivery asks for: each host, in a real browser."""
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch()
        page = browser.new_page()

        for host in hosts():
            page.goto(f"https://{host}/health")
            body = json.loads(page.inner_text("body"))
            assert body["status"] == "ok", host
            page.screenshot(path=str(screenshots / f"{host}.png"))

        browser.close()

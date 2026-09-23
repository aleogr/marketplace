"""The suite's marketplaces, served from a real database.

Until F13 every end-to-end run had no database, so no host resolved and no
flow that writes could be tested. These are the two marketplaces every
identity test signs up in (docs/superpowers/plans/2026-09-23-f13-identity-core.md).
"""

from __future__ import annotations

import json
import urllib.request

import pytest


def fetch(marketplace, host: str, path: str) -> tuple[int, str]:
    """GET a path as the given host, through the loopback address.

    The request goes to 127.0.0.1 with the host named in the header, because
    Python's resolver, unlike Chromium's, may not resolve ``*.localhost``.
    """
    request = urllib.request.Request(marketplace.base_url + path, headers={"Host": host})
    with urllib.request.urlopen(request, timeout=10) as response:
        return response.status, response.read().decode()


@pytest.mark.local_process
def test_each_seeded_host_serves_its_own_marketplace(run_marketplace):
    marketplace = run_marketplace()

    first_status, first = fetch(marketplace, "m1.localhost", "/pt-BR/")
    second_status, second = fetch(marketplace, "m2.localhost", "/pt-BR/")

    assert first_status == 200 and "Loja Um" in first
    assert second_status == 200 and "Loja Dois" in second


@pytest.mark.local_process
def test_the_health_check_reports_the_database(run_marketplace):
    marketplace = run_marketplace()
    with urllib.request.urlopen(marketplace.base_url + "/health", timeout=10) as response:
        body = json.load(response)
    assert body["database"] == "ok"

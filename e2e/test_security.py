"""The binary defends itself, because nothing in front of it does.

There is no load balancer and no web application firewall between the internet
and this process (docs/requirements.md, section 24), so the headers a reverse
proxy would normally add have to be on the responses the binary itself writes.
The unit tests prove each middleware; these prove they are mounted.
"""

from __future__ import annotations

import urllib.error
import urllib.request

import pytest


def head(url: str):
    """Ask for the headers alone, which is what `curl -I` does."""
    request = urllib.request.Request(url, method="HEAD")
    try:
        return urllib.request.urlopen(request, timeout=10)
    except urllib.error.HTTPError as error:
        return error


def test_every_response_carries_the_security_headers(server):
    response = head(f"{server.base_url}/health")

    policy = response.headers["Content-Security-Policy"]
    assert "default-src 'self'" in policy
    assert "frame-ancestors 'none'" in policy
    assert "object-src 'none'" in policy
    # A nonce per response is what lets the policy refuse inline script
    # without refusing this application's own.
    assert "'nonce-" in policy
    assert "unsafe-inline" not in policy and "unsafe-eval" not in policy

    assert response.headers["X-Content-Type-Options"] == "nosniff"
    assert response.headers["Referrer-Policy"] == "strict-origin-when-cross-origin"
    assert "camera=()" in response.headers["Permissions-Policy"]


def test_a_deployment_that_is_not_indexable_says_so(server):
    """The lab must never appear in a search engine.

    Its address is public and a crawler pointed at it directly obeys nothing
    but this header (docs/requirements.md, section 7.1).
    """
    response = head(f"{server.base_url}/health")
    assert response.headers["X-Robots-Tag"] == "noindex, nofollow"


def test_the_nonce_is_new_on_every_response(server):
    """A nonce reused across responses can be read from one page and spent in
    another, which is the whole attack it exists to prevent."""
    nonces = {
        head(f"{server.base_url}/health").headers["Content-Security-Policy"]
        for _ in range(5)
    }
    assert len(nonces) == 5, "the policy, and therefore the nonce, repeats"


def test_a_deployment_reached_over_https_asks_to_be_remembered(server):
    """Strict-Transport-Security is a claim about an address, so the local
    process — reached over plain HTTP — makes none, and a deployment does."""
    if not server.base_url.startswith("https://"):
        pytest.skip("the process under test is not reached over HTTPS")

    policy = head(f"{server.base_url}/health").headers["Strict-Transport-Security"]
    assert "max-age=" in policy and "includeSubDomains" in policy

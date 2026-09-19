"""The lab cannot be indexed by accident, and one variable changes that.

The rule is checked against the running server in both modes, not only in unit
tests, because what is being claimed is how other people's crawlers will treat
this deployment (docs/requirements.md, section 7.1).
"""

from __future__ import annotations

import urllib.error
import urllib.request

import pytest

# Every kind of response the process writes, not only HTML: a preview image or
# a plain-text file listed in a search result is still a listing.
#
# They are asked for at a host a marketplace claims, because that is where
# pages exist; the health check is the exception, and belongs to the service
# rather than to any marketplace (e2e/conftest.py, `site_url`).
ROUTES = ["/robots.txt", "/preview.png", "/en-US/", "/"]


class Stop(urllib.request.HTTPRedirectHandler):
    """A redirect is an answer here, not a step on the way to one."""

    def redirect_request(self, *args, **kwargs):  # noqa: D102
        return None


def head(base_url: str, path: str, follow: bool = True):
    request = urllib.request.Request(base_url + path, method="HEAD")
    opener = urllib.request.build_opener() if follow else urllib.request.build_opener(Stop)
    try:
        return opener.open(request, timeout=15)
    except urllib.error.HTTPError as error:
        return error


def body(base_url: str, path: str) -> str:
    try:
        return urllib.request.urlopen(base_url + path, timeout=15).read().decode()
    except urllib.error.HTTPError as error:
        return error.read().decode()


def test_every_response_refuses_indexing_by_default(server, site_url):
    """Not indexable is the default, and it applies to every response."""
    for path in ROUTES + ["/"]:
        response = head(site_url, path)
        assert response.headers["X-Robots-Tag"] == "noindex, nofollow", (
            f"{path} does not refuse indexing"
        )

    # The health check too: it is a response this process writes, and a probe
    # is reached at the service's own address rather than at a marketplace's.
    assert head(server.base_url, "/health").headers["X-Robots-Tag"] == "noindex, nofollow"


def test_the_files_read_by_machines_have_no_language(site_url):
    """A crawler asks for /robots.txt and a scraper asks for the preview.

    Neither is a page, so neither is served in a language: robots.txt is
    defined to live at the root, and a scraper that does not follow redirects
    would find no image at all (docs/design.md, decision 10).
    """
    for path in ("/robots.txt", "/preview.png"):
        response = head(site_url, path, follow=False)
        assert response.status == 200, (
            f"{path} answered {response.status}, "
            f"sending a machine to {response.headers.get('Location')}"
        )


def test_crawling_stays_allowed(site_url):
    """To refuse indexing, crawling has to be allowed: a crawler that cannot
    fetch the page never reads the header refusing to list it."""
    robots = body(site_url, "/robots.txt")

    assert "Allow: /" in robots
    assert "Disallow: /" not in robots
    assert "Sitemap:" not in robots
    assert "# No sitemap" in robots, "the missing line carries no explanation"


def test_the_link_preview_works_in_both_modes(site_url):
    """That is exactly why crawling stays allowed."""
    response = head(site_url, "/preview.png")

    assert response.status == 200
    assert response.headers["Content-Type"] == "image/png"


@pytest.mark.local_process
def test_one_variable_makes_a_deployment_indexable(run_server):
    """The other mode, against a server actually running in it.

    Enabling indexing on launch day is one variable, and this is the proof
    that it is the only thing that has to change.
    """
    indexable = run_server(INDEXABLE="true")

    for path in ROUTES:
        response = head(indexable.base_url, path)
        assert "X-Robots-Tag" not in response.headers, (
            f"{path} still refuses indexing with INDEXABLE=true"
        )

    robots = body(indexable.base_url, "/robots.txt")
    assert "Allow: /" in robots
    assert "Disallow: /" not in robots
    # The sitemap arrives with the first indexable pages, in phase 2; until
    # then robots.txt says so rather than pointing at a 404.
    assert "# No sitemap yet" in robots


@pytest.mark.local_process
def test_the_start_up_log_says_which_mode_is_in_effect(run_server):
    """A deployment left in the wrong mode is invisible, so the mode is in the
    log of every start (docs/requirements.md, section 7.1)."""
    indexable = run_server(INDEXABLE="true")

    assert indexable.entry("server started")["indexable"] is True

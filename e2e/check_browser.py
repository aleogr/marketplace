"""Fail early when the pinned Playwright cannot use the browser that is here.

Playwright ties each release to one exact Chromium revision. The cloud
environment ships a single revision under ``PLAYWRIGHT_BROWSERS_PATH`` and its
documentation says not to run ``playwright install`` (see
``docs/claude-code-environment.md``), so a Playwright release that wants a
different revision has no browser at all in a session. Without this check the
symptom is an "executable doesn't exist" thrown from inside a test, which reads
like a broken test rather than a version mismatch.

CI is deliberately not covered: the runner downloads whichever revision its
Playwright asks for, so the pipeline cannot fail on this and cannot protect
against it either. That is the whole reason the pin is held in
``.github/dependabot.yml`` rather than left to the bot.
"""

from __future__ import annotations

import importlib
import json
import os
import pathlib
import sys


def required_revision(playwright_module) -> str:
    """The Chromium revision the installed Playwright is built against."""
    manifest = next(
        pathlib.Path(playwright_module.__file__).parent.rglob("browsers.json")
    )
    browsers = json.loads(manifest.read_text())["browsers"]
    return next(b["revision"] for b in browsers if b["name"] == "chromium")


def missing_modules() -> list[str]:
    """The suite's imports that this interpreter does not have."""
    missing = []
    for name in ("playwright", "pytest"):
        try:
            importlib.import_module(name)
        except ImportError:
            missing.append(name)
    return missing


def main() -> int:
    # A container whose setup ran before the repository was cloned has none of
    # this installed, and the bare import error further down reads like a broken
    # suite rather than a missing install.
    missing = missing_modules()
    if missing:
        print(
            f"{', '.join(missing)} not installed for {sys.executable}.\n"
            "Run: make e2e-deps",
            file=sys.stderr,
        )
        return 1

    import playwright

    browsers_path = os.environ.get("PLAYWRIGHT_BROWSERS_PATH")
    if not browsers_path or not pathlib.Path(browsers_path).is_dir():
        # No preinstalled browsers: Playwright manages its own, as on CI.
        return 0

    revision = required_revision(playwright)
    present = sorted(
        p.name.split("-", 1)[1]
        for p in pathlib.Path(browsers_path).iterdir()
        if p.name.startswith("chromium-")
    )

    if revision in present:
        return 0

    print(
        f"Playwright needs Chromium revision {revision}, but {browsers_path} "
        f"only has {', '.join(present) or 'none'}.\n"
        "This environment does not download browsers, so the pinned Playwright "
        "in e2e/requirements.txt must be one built against a revision that is "
        "present. See docs/claude-code-environment.md.",
        file=sys.stderr,
    )
    return 1


if __name__ == "__main__":
    sys.exit(main())

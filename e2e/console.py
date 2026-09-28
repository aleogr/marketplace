"""The console's host, and its first run, for the tests (F15)."""

from __future__ import annotations

from accounts import PASSWORD, SUBMIT

# The hosts the harness serves beside the marketplaces': the platform's own,
# and the console's (docs/superpowers/specs/2026-09-26-f15-console-design.md,
# D1). Chromium resolves any *.localhost to the loopback address, as it does
# the marketplaces'.
PLATFORM_HOST = "platform.localhost"
CONSOLE_HOST = "console.localhost"

# The owner every first run creates.
OWNER = "dona@example.test"
OWNER_NAME = "Dona"


def first_run(page, console, language="pt-BR", token=None):
    """Fill the first run's form on the console and send it, with the
    process's own token unless another is given."""
    page.goto(console.url(CONSOLE_HOST, f"/{language}/setup"))
    page.fill("input[name=token]", console.token if token is None else token)
    page.fill("input[name=name]", OWNER_NAME)
    page.fill("input[name=email]", OWNER)
    page.fill("input[name=password]", PASSWORD)
    page.click(SUBMIT)

# Claude Code Environment

Reference for the configuration used by the **marketplace** cloud environment
in [claude.ai/code](https://claude.ai/code).

## Environment setup script

It lives in the environment settings on claude.ai/code, **not in the
repository**. It runs at the start of every cloud session:

```bash
#!/bin/bash
LOG=/var/log/setup-environment.log
REPO=/home/user/marketplace

# The end-to-end suite's versions live in e2e/requirements.txt, under
# Dependabot. This script names no version of its own: a number repeated here
# would drift, because nothing in CI can see this file.
if [ -d "$REPO" ]; then
  make -C "$REPO" e2e-deps >> "$LOG" 2>&1 \
    || echo "FAILURE: e2e dependencies not installed" >> "$LOG"
else
  echo "NOTE: $REPO absent at setup time; run 'make e2e-deps' in the session" >> "$LOG"
fi

claude plugin marketplace add anthropics/claude-plugins-official >> "$LOG" 2>&1 \
  || echo "FAILURE: official marketplace not added" >> "$LOG"

claude plugin install superpowers@claude-plugins-official >> "$LOG" 2>&1 \
  || echo "FAILURE: superpowers not installed" >> "$LOG"

exit 0
```

All command output goes to `/var/log/setup-environment.log`. Installation
failures are recorded in that file as lines starting with `FAILURE`.

### Why the script installs no version of its own

It used to name `playwright==1.56.0` and `pytest==9.1.1` directly. Both numbers
also live in `e2e/requirements.txt`, where Dependabot maintains them — and this
file is not in the repository, so nothing could keep the two copies together.
They drifted within a day of being written.

Calling `make e2e-deps` leaves the versions in exactly one place. The guard
exists because the order between the repository being cloned and this script
running is not something the session can observe; when the repository is not
there yet, the script says so in the log instead of installing a stale guess,
and `e2e/check_browser.py` tells whoever runs `make e2e` which command fixes it.

### Why `pytest` is installed next to playwright

The environment ships a `pytest` on the path, but it is a `uv` tool with its own
isolated interpreter, which does not have Playwright. `python3` has Playwright.
The end-to-end suite needs both in the same interpreter, which is why
`make e2e-deps` installs with `python3 -m pip` and `make e2e` runs the suite as
`python3 -m pytest`, rather than using whichever `pytest` is first on the path.

### Why Superpowers is installed by the script

Plugins enabled only through `enabledPlugins` in the repository's
`.claude/settings.json` are not installed in cloud sessions. The script
installs the plugin directly, which is why the repository no longer keeps a
`settings.json`.

The script does not pin a version of the plugin. The installed version is
the latest available in the marketplace when the session starts (6.3.0 as
of 2026-09-16).

### Why playwright is pinned to 1.56.0

It is the version of the Python package compatible with the Chromium
pre-installed in the environment (revision 1194, Chromium 141.0.7390.37, in
`/opt/pw-browsers`). Other versions would try to download a different
Chromium.

The Python package does not publish patch releases: the 1.56 series on PyPI
has only 1.56.0, which corresponds to the Node package `playwright@1.56.1`
and uses the same Chromium revision. Pinning to `1.56.1` makes `pip install`
fail with "No matching distribution found"; with the current script, this
shows up in `/var/log/setup-environment.log` as the line
`FAILURE: playwright not installed`.

The pin is **held against Dependabot** by an `ignore` entry in
`.github/dependabot.yml`, and checked by `e2e/check_browser.py`, which `make
e2e` runs before the suite. Both exist because CI cannot protect this: the
GitHub runner downloads whichever revision its Playwright asks for, so a bump
passes the pipeline and breaks only the session. It happened once, in
[#22](https://github.com/aleogr/marketplace/pull/22), which moved the pin to
1.62.0 — a release built against revision 1234.

**When the environment image updates its Chromium**, remove the `ignore` entry
and let Playwright move with it. `make e2e` prints the revision it found when
the two disagree.

## Skills in `.claude/skills/`

| Skill             | Origin                                                                |
| ----------------- | --------------------------------------------------------------------- |
| `frontend-design` | Copied from [anthropics/skills](https://github.com/anthropics/skills), commit `34040c9` |
| `webapp-testing`  | Copied from [anthropics/skills](https://github.com/anthropics/skills), commit `34040c9` |

The `skill-creator` skill is **not** kept in the repository: it is synced from
the claude.ai account and appears in sessions as
`anthropic-skills:skill-creator`.

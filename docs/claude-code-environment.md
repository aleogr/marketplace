# Claude Code Environment

Reference for the configuration used by the **marketplace** cloud environment
in [claude.ai/code](https://claude.ai/code).

## Environment setup script

It lives in the environment settings on claude.ai/code, **not in the
repository**. It runs at the start of every cloud session:

```bash
#!/bin/bash
LOG=/var/log/setup-environment.log

pip install --retries 5 --timeout 60 "playwright==1.56.0" >> "$LOG" 2>&1 \
  || pip install --break-system-packages --retries 5 --timeout 60 "playwright==1.56.0" >> "$LOG" 2>&1 \
  || echo "FAILURE: playwright not installed" >> "$LOG"

claude plugin marketplace add anthropics/claude-plugins-official >> "$LOG" 2>&1 \
  || echo "FAILURE: official marketplace not added" >> "$LOG"

claude plugin install superpowers@claude-plugins-official >> "$LOG" 2>&1 \
  || echo "FAILURE: superpowers not installed" >> "$LOG"

exit 0
```

All command output goes to `/var/log/setup-environment.log`. Installation
failures are recorded in that file as lines starting with `FAILURE`.

### Why Superpowers is installed by the script

Plugins enabled only through `enabledPlugins` in the repository's
`.claude/settings.json` are not installed in cloud sessions. The script
installs the plugin directly, which is why the repository no longer keeps a
`settings.json`.

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

## Skills in `.claude/skills/`

| Skill             | Origin                                                                |
| ----------------- | --------------------------------------------------------------------- |
| `frontend-design` | Copied from [anthropics/skills](https://github.com/anthropics/skills), commit `34040c9` |
| `webapp-testing`  | Copied from [anthropics/skills](https://github.com/anthropics/skills), commit `34040c9` |

The `skill-creator` skill is **not** kept in the repository: it is synced from
the claude.ai account and appears in sessions as
`anthropic-skills:skill-creator`.

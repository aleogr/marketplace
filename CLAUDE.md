# Working agreements for Claude Code

These rules restate section 3 of `docs/requirements.md` and the agreements made in design sessions. They apply to every session in this repository.

## Language

- Talk to the owner in **Portuguese**.
- Everything versioned is written in **English**: code, comments, commit messages, pull request titles and descriptions, documentation. The only exception is translation files for other languages.

## Sources of truth

- `docs/requirements.md` decides what the product is. Do not reopen a decision recorded there; if you find a real contradiction, ask the owner.
- `docs/design.md` decides how the product is built. `docs/research/` explains why; consult it, do not redo research.
- When a session changes a requirement, update `docs/requirements.md` in the same pull request.

## Branches and pull requests

- Each topic is developed in its own branch and merged through a pull request, always opened by Claude Code, never directly on `main`.
- Never push to a branch other than the one designated for the session without explicit permission.

## Working with the owner

- Manual steps outside a session (GCP console, GitHub, Cloudflare, providers) are given **one at a time**: the owner executes, reports the result, then receives the next instruction.
- When the owner asks to see something before an action (commit, push, merge, configuration change), show it and wait for explicit confirmation.
- External dependencies the owner must obtain (commercial proposals, provider confirmations, sandbox accounts) are recorded as dependencies of the phase that needs them, with the recommended moment; they do not block design work.

## Quality

- Follow **market best practices** for the stack and the domain; when a choice departs from them, say so and why.
- Definition of done: every user-facing text exists in **both en-US and pt-BR**, tests pass, and there is verification evidence (test output, screenshots or equivalent).
- No hard-coded user-facing text: everything goes through translation keys, and Claude Code maintains all translations.
- UI principles: immediate visual feedback when an element is pressed; respect the operating system's reduced-motion preference; animations only when they serve a purpose.
- Tests use fake implementations of external providers, never real services.

## Security

- The repository is public. **No secret may ever be committed.** Secrets live in Secret Manager or in the repository's encrypted secrets.
- Model identifiers appear only in the attribution trailer of commit messages and pull request descriptions; never in code, documentation, titles or comments.

## Commands

Once the code exists, the checks that CI runs are the ones to run before pushing; keep this list current:

```
make check   # go vet, staticcheck, golangci-lint, gosec, govulncheck
make test    # go test ./... -race -cover
make e2e     # builds the binary and runs the end-to-end suite against it
```

The tools are pinned as `tool` directives in `go.mod` and run through `go tool`,
so a session and CI run the same versions. CI additionally runs `gitleaks` and
scans the container image with Trivy.

The end-to-end suite must run under the interpreter that has Playwright
installed, which is `python3`, not the `pytest` on the path (see
`docs/claude-code-environment.md`). `make e2e` does that; a bare `pytest e2e/`
fails to import Playwright.

# marketplace

A platform that hosts several marketplaces, each serving one or more niches, as
a single Go binary. The first two marketplaces are electronics and vehicles, and
Brazil is the first market to operate.

- **What the product is:** [`docs/requirements.md`](docs/requirements.md)
- **How it is built:** [`docs/design.md`](docs/design.md)
- **Why the decisions were taken:** [`docs/research/`](docs/research/)
- **What is being built now:** [`docs/roadmap.md`](docs/roadmap.md)

## Running it

```sh
make run
```

The process reads its configuration from the environment and refuses to start
when a variable is set to something it cannot read:

| Variable | Default | Meaning |
|---|---|---|
| `PORT` | `8080` | the TCP port the server listens on |
| `INDEXABLE` | `false` | whether search engines may list this deployment |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error` |
| `PROVIDERS_MODE` | `fake` | `fake` or `real` external providers |

## Checks

These are the checks CI runs. A green run here is a green run there.

```sh
make check   # go vet, staticcheck, golangci-lint, gosec, govulncheck
make test    # go test ./... -race -cover
make e2e     # builds the binary and runs the Playwright suite against it
```

## Layout

```
cmd/marketplace/     the binary
internal/platform/   concerns shared by every module: config, logging, HTTP, version
e2e/                 end-to-end tests, Playwright driven from pytest
docs/                requirements, design, roadmap and research
```

# Contributing to phomv

## Prerequisites

- Go 1.24+
- `make`

## Build & test

```sh
make build    # produces ./bin/phomv
make test     # go test ./...
make vet      # go vet ./...
make fmt      # gofmt -s -w .
```

All four must pass before submitting a PR.

## Making changes

1. Fork the repo and create a branch off `main`.
2. Write or update tests for your change.
3. Run `make test && make vet && make fmt`.
4. Open a pull request against `main`.

Keep PRs focused — one logical change per PR makes review faster.

## Code style

- Standard `gofmt` formatting (enforced by `make fmt`).
- No new external dependencies without discussion in an issue first.
- Prefer table-driven tests in `_test.go` files alongside the package under test.

## Good first issues

Look for issues labelled
[`good first issue`](https://github.com/phomv/phomv/labels/good%20first%20issue).

## Reporting security issues

See [SECURITY.md](SECURITY.md).

## Code of Conduct

See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

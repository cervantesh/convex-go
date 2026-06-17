# Contributing

This project is pre-v1 and is being developed from GitHub issues.

## Development

The minimum supported version is Go 1.25, matching `go.mod`.

```sh
go test ./... -count=1
```

Run the full local quality gates before opening a PR:

```sh
go test ./... -count=1
go test ./... -race -count=1
go test ./... -shuffle=on -count=1
go vet ./...
golangci-lint run --timeout=5m
govulncheck ./...
go mod verify
go mod tidy -diff
git diff --check
```

Coverage must stay at or above 90%. Sonar report generation is documented in
[docs/maintainers/QUALITY.md](docs/maintainers/QUALITY.md).

Keep changes scoped to the issue being addressed. For protocol work, prefer fixtures and deterministic tests over live Convex deployments.
Compatibility claims must cite an upstream source and be backed by an offline fixture; see [docs/CONFORMANCE.md](docs/CONFORMANCE.md).

See [docs/maintainers/DEVELOPMENT.md](docs/maintainers/DEVELOPMENT.md) for the required spec-driven and TDD workflow.
See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) before moving exported APIs or changing package boundaries.
Release preparation is tracked in [docs/maintainers/RELEASE.md](docs/maintainers/RELEASE.md).
Public snapshot export is documented in [docs/maintainers/PUBLICATION.md](docs/maintainers/PUBLICATION.md).

## Compatibility Priorities

1. Match Convex wire formats exactly.
2. Keep Go APIs idiomatic.
3. Keep HTTP and WebSocket clients clearly separated.
4. Preserve testability with fake transports and protocol fixtures.
5. Keep dependency additions rare and justified in the issue or PR.

## Issue Workflow

Issues should include acceptance criteria. A good issue states:

- which upstream client behavior it mirrors (`convex-rs`, `convex-js`, or `convex-py`)
- exact public API expected
- exact wire format expected, if relevant
- tests required before closing

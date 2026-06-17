# Contributing

This project is pre-v1 and is being developed from GitHub issues.

## Development

The minimum supported version is Go 1.25, matching `go.mod`.

```sh
go test ./... -count=1
```

GitHub Actions in `.github/workflows/ci.yml` is the authoritative quality gate
for pull requests and releases. Push your branch and use CI as the source of
truth for:

- `go test ./... -count=1`
- `go test ./... -race -count=1`
- `go test ./... -shuffle=on -count=1`
- `go vet ./...`
- `golangci-lint run --timeout=5m --output.json.path=golangci-report.json`
- `govulncheck ./...`
- `go mod verify`
- `go mod tidy -diff`
- `git diff --check`
- coverage `>=90%`
- Sonar report generation and artifact upload

Sonar is published from GitHub Actions, not from a workstation. Configure
repository secret `SONAR_TOKEN`, and set repository variable `SONAR_HOST_URL`
when using self-hosted SonarQube Server or Community Build.

Local runs are optional for fast feedback while iterating:

```sh
go test ./... -count=1
```

Coverage must stay at or above 90%. The GitHub-first quality contract and Sonar
automation are documented in [docs/maintainers/QUALITY.md](docs/maintainers/QUALITY.md).

Keep changes scoped to the issue being addressed. For protocol work, prefer fixtures and deterministic tests over live Convex deployments.
Compatibility claims must cite an upstream source and be backed by an offline fixture; see [docs/CONFORMANCE.md](docs/CONFORMANCE.md).

See [docs/maintainers/DEVELOPMENT.md](docs/maintainers/DEVELOPMENT.md) for the required spec-driven and TDD workflow.
See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) before moving exported APIs or changing package boundaries.
See [docs/maintainers/LIVE_INTEGRATION.md](docs/maintainers/LIVE_INTEGRATION.md) for the manual live deployment workflow.
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

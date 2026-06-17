# Quality Gates

This repository uses repeatable local and CI gates before public API or sync
changes are considered done.

## Local Gates

Run these from the repository root in any shell:

```text
go run ./cmd/convex-go-maint fmt-check
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

`govulncheck` should be run with the latest patch release for the supported Go
line. Standard library vulnerabilities are fixed by updating the Go toolchain,
not by changing this module's dependencies.

Coverage must stay at coverage >=90%:

```text
go test "-coverprofile=coverage.out" ./...
go run ./cmd/convex-go-maint coverage-check -coverprofile=coverage.out -min=90
```

Generate the reports consumed by Sonar:

```text
go test -json ./... > test-report.out
go test "-coverprofile=coverage.out" ./...
golangci-lint run --timeout=5m --output.json.path=golangci-report.json
```

Run the scanner after reports exist and a Sonar token is available:

```text
sonar-scanner -Dsonar.host.url=<your-sonarqube-host> -Dsonar.token=<your-sonar-token>
```

Sonar report paths are configured in `sonar-project.properties`:

```properties
sonar.go.coverage.reportPaths=coverage.out
sonar.go.tests.reportPaths=test-report.out
sonar.go.golangci-lint.reportPaths=golangci-report.json
```

Do not configure Golint, GoMetaLinter, or go vet report paths unless this repo
actually generates those files. Nonexistent report paths create false Sonar
warnings and hide real signal.

## CI Policy

CI should run a smoke matrix on Linux, macOS, and Windows, plus the heavy
quality gates on Ubuntu. The minimum supported version is Go 1.25, matching
`go.mod`.

The smoke matrix should prove that `fmt-check`, module verification, and
`go test ./... -count=1` work on Linux, macOS, and Windows.

The Ubuntu quality job should enforce `fmt-check`, `go mod tidy -diff`,
shuffled tests, `govulncheck`, race tests, vet, coverage >=90%, Sonar report
generation, and golangci-lint on Go 1.25 and one current stable Go version.

Dependency additions should be justified in the issue or PR that introduces
them. Public API changes require an issue with compatibility notes while the
module is pre-v1.

## Optional Live Integration Workflow

Live infrastructure is optional and must not replace the offline gates in CI.

The separate `Live Integration` workflow:

- runs only on manual `workflow_dispatch`
- requires maintainer-provided secrets for the deployment URL
- runs a preflight environment check before the live test
- exercises a build-tagged live test without changing the default CI contract

See [LIVE_INTEGRATION.md](LIVE_INTEGRATION.md) for the deployment contract,
secrets, and sample Convex app used by the live workflow.

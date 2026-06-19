# Quality Gates

GitHub Actions is the authoritative quality gate for this repository.
Maintainers may still run focused commands locally for debugging, but merge,
release, and publication decisions follow `.github/workflows/ci.yml`, not
workstation-only runs.

## GitHub Actions Contract

The `CI` workflow runs on `pull_request`, pushes to `main`, and manual
`workflow_dispatch`.

The smoke matrix must prove that these gates work on Linux, macOS, and Windows:

- `go run ./cmd/convex-go-maint fmt-check`
- `go mod verify`
- `git diff --check`
- `go test ./... -count=1`
- `go mod tidy -diff` in `examples/realtime_chat`
- `go test ./... -count=1` in `examples/realtime_chat`

The root smoke path compiles the public examples in `examples_test.go` and
`auth_examples_test.go`. The separate demo smoke path compiles the nested
`examples/realtime_chat` module so public application-facing examples stay in
the default CI contract.

The Ubuntu quality job must enforce the heavier gates on Go 1.25 and one
current stable Go version:

- `go run ./cmd/convex-go-maint fmt-check`
- `go mod verify`
- `go mod tidy -diff`
- `git diff --check`
- `go test ./... -count=1`
- `go test ./... -race -count=1`
- `go test ./... -shuffle=on -count=1`
- `go vet ./...`
- `govulncheck ./...`
- `golangci-lint run --timeout=5m --output.json.path=golangci-report.json`
- `go test "-coverprofile=coverage.out" ./...`
- `go run ./cmd/convex-go-maint coverage-check -coverprofile=coverage.out -min=90`
- `go test -json ./... > test-report.out`

Coverage must stay at coverage >=90%.

## Sonar In GitHub Actions

Sonar is published from GitHub Actions, not from a workstation.

The same `CI` workflow generates the reports consumed by
`sonar-project.properties`:

```properties
sonar.go.coverage.reportPaths=coverage.out
sonar.go.tests.reportPaths=test-report.out
sonar.go.golangci-lint.reportPaths=golangci-report.json
```

On Go `stable`, the workflow uploads those reports as artifacts and then runs:

- `SonarSource/sonarqube-scan-action@v8.2.0`
- `SonarSource/sonarqube-quality-gate-action@v1.2.0`

The quality gate step must use `pollingTimeoutSec: 600`.

Configure repository secret `SONAR_TOKEN` for all Sonar installations. When
using self-hosted SonarQube Server or Community Build, also set repository
variable `SONAR_HOST_URL`. SonarQube Cloud does not need `SONAR_HOST_URL`.

To keep the default pull request contract predictable, the Sonar scan runs on
pushes to `main` and manual `workflow_dispatch`, not on `pull_request` events.

Do not configure Golint, GoMetaLinter, or go vet report paths unless this repo
actually generates those files. Nonexistent report paths create false Sonar
warnings and hide real signal.

## Optional Local Debugging

Local commands are for fast feedback only. They are not the release contract.
Typical focused commands:

```text
go test ./... -count=1
go test ./... -run TestName -count=1
go test ./... -race -count=1
```

`govulncheck` should be run with the latest patch release for the supported Go
line. Standard library vulnerabilities are fixed by updating the Go toolchain,
not by changing this module's dependencies.

## Optional Live Integration Workflow

Live infrastructure is optional and must not replace the offline gates in CI.

The separate `Live Integration` workflow:

- runs only on manual `workflow_dispatch`
- requires maintainer-provided secrets for the deployment URL
- runs a preflight environment check before the live test
- exercises a build-tagged live test without changing the default CI contract

See [LIVE_INTEGRATION.md](LIVE_INTEGRATION.md) for the deployment contract,
secrets, and sample Convex app used by the live workflow.

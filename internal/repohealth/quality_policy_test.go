package repohealth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQualityPolicyDocumentsRepeatableGates(t *testing.T) {
	body := readTextFile(t, "docs/maintainers/QUALITY.md")
	for _, want := range []string{
		"go test ./...",
		"go test ./... -race -count=1",
		"go vet ./...",
		"golangci-lint run --timeout=5m",
		"go run ./cmd/convex-go-maint fmt-check",
		"go test -json ./... > test-report.out",
		`go test "-coverprofile=coverage.out" ./...`,
		"go run ./cmd/convex-go-maint coverage-check -coverprofile=coverage.out -min=90",
		"sonar-scanner -Dsonar.host.url=<your-sonarqube-host>",
		"coverage >=90%",
		"sonar.go.coverage.reportPaths=coverage.out",
		"sonar.go.tests.reportPaths=test-report.out",
		"sonar.go.golangci-lint.reportPaths=golangci-report.json",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/maintainers/QUALITY.md must document %q", want)
		}
	}
}

func TestSonarProjectUsesGeneratedReportPaths(t *testing.T) {
	body := readTextFile(t, "sonar-project.properties")
	for _, want := range []string{
		"sonar.projectKey=cervantesh_convex-go",
		"sonar.go.coverage.reportPaths=coverage.out",
		"sonar.go.tests.reportPaths=test-report.out",
		"sonar.go.golangci-lint.reportPaths=golangci-report.json",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("sonar-project.properties must contain %q", want)
		}
	}
	for _, blocked := range []string{
		"sonar.go.golint.reportPaths",
		"sonar.go.gometalinter.reportPaths",
		"sonar.go.govet.reportPaths",
	} {
		if strings.Contains(body, blocked) {
			t.Fatalf("sonar-project.properties must not configure nonexistent report path %q", blocked)
		}
	}
}

func TestSonarReportPathsAreGeneratedAndUploaded(t *testing.T) {
	sonar := readTextFile(t, "sonar-project.properties")
	quality := readTextFile(t, "docs/maintainers/QUALITY.md")
	workflow := readTextFile(t, ".github/workflows/ci.yml")
	for report, command := range map[string]string{
		"coverage.out":         "go test -coverprofile=coverage.out ./...",
		"test-report.out":      "go test -json ./... > test-report.out",
		"golangci-report.json": "golangci-lint run --timeout=5m --output.json.path=golangci-report.json",
	} {
		if !strings.Contains(sonar, report) {
			t.Fatalf("sonar-project.properties must consume generated report %s", report)
		}
		if !strings.Contains(workflow, report) {
			t.Fatalf(".github/workflows/ci.yml must generate or upload report %s", report)
		}
		if !strings.Contains(quality, report) {
			t.Fatalf("docs/maintainers/QUALITY.md must document report %s", report)
		}
		if !strings.Contains(workflow, command) {
			t.Fatalf(".github/workflows/ci.yml must generate report %s with %q", report, command)
		}
		if !strings.Contains(workflow, "if-no-files-found: error") {
			t.Fatal(".github/workflows/ci.yml must fail artifact upload when quality reports are missing")
		}
	}
}

func TestConformanceDocsCiteOfficialUpstreamSources(t *testing.T) {
	body := readTextFile(t, "docs/CONFORMANCE.md")
	for _, want := range []string{
		"official `get-convex/convex-js`",
		"official `get-convex/convex-py`",
		"official `get-convex/convex-rs`",
		"convex-js/src/values/value.ts",
		"convex-py` README",
		"convex-rs/src/base_client/mod.rs",
		"no network access and no dependency on a live Convex deployment",
		"source comment with upstream repo path",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/CONFORMANCE.md must cite upstream fixture source marker %q", want)
		}
	}
	fixtures := map[string][]string{
		"internal/core/value_test.go": {
			"convex-js/src/values/value.ts",
			"convex-rs/src/value/json/mod.rs",
			"get-convex/convex-py README",
		},
		"client_test.go": {
			"get-convex/convex-py README basic usage",
			"get-convex/convex-py README \"Pagination\"",
		},
		"internal/syncprotocol/sync_protocol_test.go": {
			"convex-rs/sync_types/src/types/json.rs",
			"convex-js/src/browser/sync/protocol.ts",
		},
		"baseclient/state_machine_test.go": {
			"convex-rs/src/base_client/mod.rs",
			"convex-js/src/browser/sync/request_manager.ts",
			"convex-js/src/browser/sync/udf_path_utils.ts",
		},
	}
	for path, markers := range fixtures {
		fixture := readTextFile(t, path)
		for _, marker := range markers {
			if !strings.Contains(fixture, marker) {
				t.Fatalf("%s must cite upstream fixture source marker %q", path, marker)
			}
		}
	}
}

func TestGoVersionPolicyMatchesModule(t *testing.T) {
	mod := readTextFile(t, "go.mod")
	if !strings.Contains(mod, "go 1.25") {
		t.Fatalf("go.mod changed; update this policy test and docs after deciding a new supported version")
	}
	for _, path := range []string{"README.md", "CONTRIBUTING.md"} {
		if body := readTextFile(t, path); !strings.Contains(body, "Go 1.25") {
			t.Fatalf("%s must document Go 1.25 as the minimum supported version", path)
		}
	}
	workflow := readTextFile(t, ".github/workflows/ci.yml")
	for _, want := range []string{`"1.25"`, "stable", "go tool cover -func=coverage.out"} {
		if want == "go tool cover -func=coverage.out" {
			continue
		}
		if !strings.Contains(workflow, want) {
			t.Fatalf(".github/workflows/ci.yml must include Go version %q", want)
		}
	}
	if !strings.Contains(workflow, "go run ./cmd/convex-go-maint coverage-check -coverprofile=coverage.out -min=90") {
		t.Fatal(".github/workflows/ci.yml must run the Go coverage check helper")
	}
	for _, want := range []string{"ubuntu-latest", "windows-latest", "macos-latest"} {
		if !strings.Contains(workflow, want) {
			t.Fatalf(".github/workflows/ci.yml must include CI platform %q", want)
		}
	}
}

func TestRepositoryDeclaresTextNormalization(t *testing.T) {
	body := readTextFile(t, ".gitattributes")
	for _, want := range []string{
		"* text=auto eol=lf",
		"*.go text eol=lf",
		"go.mod text eol=lf",
		"go.sum text eol=lf",
		"*.yml text eol=lf",
		"*.md text eol=lf",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf(".gitattributes must contain %q", want)
		}
	}
}

func TestCIIncludesRepositoryHygieneGates(t *testing.T) {
	workflow := readTextFile(t, ".github/workflows/ci.yml")
	for _, want := range []string{
		"permissions:",
		"contents: read",
		"check-latest: true",
		"needs: smoke",
		"go test ./... -count=1",
		"go test ./... -race -count=1",
		"go test ./... -shuffle=on -count=1",
		"go vet ./...",
		"go mod verify",
		"go mod tidy -diff",
		"govulncheck ./...",
		"go run ./cmd/convex-go-maint fmt-check",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf(".github/workflows/ci.yml must contain %q", want)
		}
	}
	for _, blocked := range []string{"shell: bash", "awk ", "files=$("} {
		if strings.Contains(workflow, blocked) {
			t.Fatalf(".github/workflows/ci.yml must not contain shell-specific CI logic %q", blocked)
		}
	}
}

func TestQualityDocsIncludeRepositoryHygieneGates(t *testing.T) {
	body := readTextFile(t, "docs/maintainers/QUALITY.md")
	for _, want := range []string{
		"go run ./cmd/convex-go-maint fmt-check",
		"go test ./... -count=1",
		"go test ./... -race -count=1",
		"go test ./... -shuffle=on -count=1",
		"go vet ./...",
		"golangci-lint run --timeout=5m",
		"govulncheck ./...",
		"go mod verify",
		"go mod tidy -diff",
		"git diff --check",
		"go run ./cmd/convex-go-maint coverage-check -coverprofile=coverage.out -min=90",
		"Linux, macOS, and Windows",
		"Ubuntu quality job",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/maintainers/QUALITY.md must document %q", want)
		}
	}
}

func TestMutationTestingDocsDefineCampaignGovernance(t *testing.T) {
	body := readTextFile(t, "docs/maintainers/MUTATION_TESTING.md")
	for _, want := range []string{
		"CervoMutant",
		"ci-fast",
		"--max-mutants 100",
		"--sample deterministic",
		"campaign",
		"ci-fast >= 80%",
		"reviewed-skip",
		"mutant pattern | scope | disposition | reason | evidence",
		"Do not make full campaign a required CI gate yet",
		"local CervoMutant executable available in your environment",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/maintainers/MUTATION_TESTING.md must document %q", want)
		}
	}
	for _, blocked := range []string{
		`C:\`,
		"baseclient/client.go",
		"`client.go`",
		"| issue/PR |",
	} {
		if strings.Contains(body, blocked) {
			t.Fatalf("docs/maintainers/MUTATION_TESTING.md must not document %q", blocked)
		}
	}
}

func TestCIUsesPinnedGolangCILintV2(t *testing.T) {
	workflow := readTextFile(t, ".github/workflows/ci.yml")
	for _, want := range []string{
		"github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4",
		"golangci-lint run --timeout=5m --output.json.path=golangci-report.json",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf(".github/workflows/ci.yml must contain %q", want)
		}
	}
	if strings.Contains(workflow, "golangci/golangci-lint-action") {
		t.Fatalf(".github/workflows/ci.yml must not use golangci-lint-action because it installed a v1 binary without --output.json.path support")
	}
}

func TestCIUsesNode24CompatibleActions(t *testing.T) {
	workflow := readTextFile(t, ".github/workflows/ci.yml")
	for _, want := range []string{
		"actions/checkout@v6.0.3",
		"actions/setup-go@v6.4.0",
		"actions/upload-artifact@v7.0.1",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf(".github/workflows/ci.yml must contain %q", want)
		}
	}
	for _, blocked := range []string{
		"actions/checkout@v4",
		"actions/setup-go@v5",
		"actions/upload-artifact@v4",
	} {
		if strings.Contains(workflow, blocked) {
			t.Fatalf(".github/workflows/ci.yml must not use Node 20 action %q", blocked)
		}
	}
}

func TestCommunityReadinessFilesExist(t *testing.T) {
	required := map[string][]string{
		"SECURITY.md": {
			"Security report requested",
			"Do not include exploit details",
			"Only the latest pre-v1 release line",
		},
		"SUPPORT.md": {
			"reproducible SDK bugs",
			"Do not include deployment secrets",
		},
		"CODE_OF_CONDUCT.md": {
			"project-specific conduct policy",
			"Maintainers may moderate",
		},
		"CHANGELOG.md": {
			"Unreleased",
			"Community Go client",
		},
		"docs/MAINTAINERS.md": {
			"Maintainer Guides",
			"Cross-platform automation belongs in `cmd/convex-go-maint`",
		},
		"docs/maintainers/PUBLICATION.md": {
			"Publication Workflow",
			"export-snapshot",
			"Initial public snapshot",
		},
		"docs/maintainers/RELEASE.md": {
			"Release Checklist",
			"Version",
			"CHANGELOG.md",
			"Automated Release Workflow",
			"pre-v1 prerelease",
		},
		".github/release.yml": {
			"Breaking Changes",
			"Quality And Tooling",
		},
		".github/workflows/release.yml": {
			"workflow_dispatch",
			"go run ./cmd/convex-go-maint release-check",
			"gh release create",
			"git tag -a",
			"release-notes.md",
		},
		".github/ISSUE_TEMPLATE/bug_report.md": {
			"Go version",
			"convex-go version or commit",
			"Do not include deployment secrets",
		},
		".github/ISSUE_TEMPLATE/docs.md": {
			"Documentation issue",
			"Reader Context",
		},
		".github/ISSUE_TEMPLATE/compatibility_fixture.md": {
			"Upstream Reference",
			"Focused RED command",
			"Fixture does not require a live Convex deployment",
		},
	}
	root := repoRoot(t)
	for path, phrases := range required {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Fatalf("missing community readiness file %s: %v", path, err)
		}
		body := readTextFile(t, path)
		for _, want := range phrases {
			if !strings.Contains(body, want) {
				t.Fatalf("%s must contain %q", path, want)
			}
		}
	}
}

func readTextFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), path))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

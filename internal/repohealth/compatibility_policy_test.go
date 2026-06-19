package repohealth

import (
	"strings"
	"testing"
)

func TestCompatibilityMatrixIsLinkedFromPublicDocs(t *testing.T) {
	for _, path := range []string{"README.md", "docs/MAINTAINERS.md"} {
		body := readTextFile(t, path)
		if !strings.Contains(body, "COMPATIBILITY.md") {
			t.Fatalf("%s must link docs/COMPATIBILITY.md", path)
		}
	}
}

func TestCompatibilityMatrixDocumentsToolchainAndBackendEvidence(t *testing.T) {
	body := readTextFile(t, "docs/COMPATIBILITY.md")
	for _, want := range []string{
		"Compatibility Matrix",
		"Go Toolchain Matrix",
		"Go 1.25",
		"`stable`",
		"`ubuntu-latest`",
		"`windows-latest`",
		"`macos-latest`",
		".github/workflows/ci.yml",
		".github/workflows/live-integration.yml",
		"maintainers/QUALITY.md",
		"maintainers/LIVE_INTEGRATION.md",
		"Backend Evidence Matrix",
		"Supported in default CI",
		"Supported in manual workflow",
		"Not yet a default public gate",
		"Not claimed",
		"`live:listMessages`",
		"`live:sendMessage`",
		"`live:ping`",
		"CONVEX_AUTH_TOKEN",
		"Update Policy",
		"`go.mod`",
		"`testdata/live-integration/convex/`",
		"`live_integration_test.go`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/COMPATIBILITY.md must document %q", want)
		}
	}
	for _, blocked := range []string{"```powershell", "$env:", `.ps1`, `C:\`} {
		if strings.Contains(body, blocked) {
			t.Fatalf("docs/COMPATIBILITY.md must stay shell-neutral and workstation-neutral; found %q", blocked)
		}
	}
}

func TestRoadmapMarksCompatibilityMatrixIssueComplete(t *testing.T) {
	body := readTextFile(t, "docs/ROADMAP.md")
	for _, want := range []string{
		"## Milestone 2 - Runtime Reliability",
		"Completed in this repository:",
		"#35 Document and maintain a Go and backend compatibility matrix",
		"Remaining:",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/ROADMAP.md must document %q", want)
		}
	}
	if strings.Contains(body, "Remaining:\n\n- #35 Document and maintain a Go and backend compatibility matrix") {
		t.Fatal("docs/ROADMAP.md must not list issue #35 as still remaining")
	}
}

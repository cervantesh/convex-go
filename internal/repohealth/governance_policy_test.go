package repohealth

import (
	"strings"
	"testing"
)

func TestGovernanceDocsDefineMaintainerAndReleasePolicy(t *testing.T) {
	body := readTextFile(t, "docs/maintainers/GOVERNANCE.md")
	for _, want := range []string{
		"Governance",
		"`@cervantesh`",
		".github/CODEOWNERS",
		"Do not push directly to\n  `main`",
		"GitHub Actions green before merge",
		"Use squash merges",
		"`Closes #...`",
		"RELEASE.md",
		"QUALITY.md",
		"`CHANGELOG.md`",
		"`client_options.go`",
		"Only the latest pre-v1 release line is expected to receive fixes",
		"linked issue",
		"updated API surface tests",
		"updated public docs",
		"migration notes",
		"`baseclient` is advanced, but it is still public",
		"SUPPORT.md",
		"SECURITY.md",
		"COMMUNITY.md",
		"project remains community-first",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/maintainers/GOVERNANCE.md must document %q", want)
		}
	}
}

func TestGovernanceFilesAreLinkedAndTracked(t *testing.T) {
	maintainers := readTextFile(t, "docs/MAINTAINERS.md")
	if !strings.Contains(maintainers, "maintainers/GOVERNANCE.md") {
		t.Fatal("docs/MAINTAINERS.md must link maintainers/GOVERNANCE.md")
	}

	contributing := readTextFile(t, "CONTRIBUTING.md")
	if !strings.Contains(contributing, "docs/maintainers/GOVERNANCE.md") {
		t.Fatal("CONTRIBUTING.md must link docs/maintainers/GOVERNANCE.md")
	}

	codeowners := readTextFile(t, ".github/CODEOWNERS")
	if !strings.Contains(codeowners, "@cervantesh") {
		t.Fatal(".github/CODEOWNERS must declare the current default owner")
	}
}

func TestRoadmapMarksGovernanceIssueComplete(t *testing.T) {
	body := readTextFile(t, "docs/ROADMAP.md")
	for _, want := range []string{
		"## Milestone 4 - Convex Adoption Readiness",
		"Completed in this repository:",
		"#44 Define governance and maintainer policy for a Convex-adoptable Go client",
		"Remaining:",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/ROADMAP.md must document %q", want)
		}
	}
	if strings.Contains(body, "Remaining:\n\n- #44 Define governance and maintainer policy for a Convex-adoptable Go client") {
		t.Fatal("docs/ROADMAP.md must not list issue #44 as still remaining")
	}
}

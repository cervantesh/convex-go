package repohealth

import (
	"strings"
	"testing"
)

func TestCommunityOperationsDocsDefineTemplatesAndCadence(t *testing.T) {
	body := readTextFile(t, "docs/maintainers/COMMUNITY.md")
	for _, want := range []string{
		"Community Operations",
		".github/ISSUE_TEMPLATE/bug_report.md",
		".github/ISSUE_TEMPLATE/docs.md",
		".github/ISSUE_TEMPLATE/feature_request.md",
		".github/ISSUE_TEMPLATE/compatibility_fixture.md",
		".github/PULL_REQUEST_TEMPLATE.md",
		"Blank issues should stay disabled",
		"weekly triage cadence",
		"Review new issues and PRs at least once per week",
		"SECURITY.md",
		"SUPPORT.md",
		"`Closes #...`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/maintainers/COMMUNITY.md must document %q", want)
		}
	}
}

func TestIssueTemplateConfigRoutesCommunityAndSecurityTraffic(t *testing.T) {
	body := readTextFile(t, ".github/ISSUE_TEMPLATE/config.yml")
	for _, want := range []string{
		"blank_issues_enabled: false",
		"Convex product docs and community",
		"https://www.convex.dev/community",
		"Private security reporting",
		"SECURITY.md",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf(".github/ISSUE_TEMPLATE/config.yml must contain %q", want)
		}
	}
}

func TestSupportAndRoadmapReflectCommunityOpsPolicy(t *testing.T) {
	support := readTextFile(t, "SUPPORT.md")
	for _, want := range []string{
		".github/ISSUE_TEMPLATE/",
		"official Convex documentation",
		"community channels",
		"Do not include deployment secrets",
	} {
		if !strings.Contains(support, want) {
			t.Fatalf("SUPPORT.md must document %q", want)
		}
	}

	maintainers := readTextFile(t, "docs/MAINTAINERS.md")
	if !strings.Contains(maintainers, "maintainers/COMMUNITY.md") {
		t.Fatal("docs/MAINTAINERS.md must link maintainers/COMMUNITY.md")
	}

	roadmap := readTextFile(t, "docs/ROADMAP.md")
	for _, want := range []string{
		"## Milestone 3 - Adoption",
		"Completed in this repository:",
		"#41 Define community operations, templates, and triage cadence for pre-v1",
		"Remaining:",
	} {
		if !strings.Contains(roadmap, want) {
			t.Fatalf("docs/ROADMAP.md must document %q", want)
		}
	}
	if strings.Contains(roadmap, "Remaining:\n\n- #41 Define community operations, templates, and triage cadence for pre-v1") {
		t.Fatal("docs/ROADMAP.md must not list issue #41 as still remaining")
	}
}

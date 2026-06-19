package repohealth

import (
	"strings"
	"testing"
)

func TestNamespaceTransitionGuideDocumentsPrerequisitesAndSequence(t *testing.T) {
	body := readTextFile(t, "docs/maintainers/NAMESPACE_TRANSITION.md")
	for _, want := range []string{
		"Namespace Transition Readiness",
		"`github.com/cervantesh/convex-go`",
		"`github.com/get-convex/convex-go`",
		"readiness plan only",
		"community-first",
		"Convex explicitly adopts the client",
		"Do not rewrite public history in place.",
		"`go.mod`",
		"`README.md`",
		"`CHANGELOG.md`",
		"`pkg.go.dev`",
		"migration guide",
		"legacy repository",
		"official release",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/maintainers/NAMESPACE_TRANSITION.md must document %q", want)
		}
	}
}

func TestNamespaceTransitionGuideIsLinkedAndRoadmapMarksIssueComplete(t *testing.T) {
	maintainers := readTextFile(t, "docs/MAINTAINERS.md")
	if !strings.Contains(maintainers, "NAMESPACE_TRANSITION.md") {
		t.Fatal("docs/MAINTAINERS.md must link NAMESPACE_TRANSITION.md")
	}

	roadmap := readTextFile(t, "docs/ROADMAP.md")
	for _, want := range []string{
		"## Milestone 4 - Convex Adoption Readiness",
		"Completed in this repository:",
		"#45 Draft a namespace transition readiness plan for",
		"`github.com/get-convex/convex-go`",
	} {
		if !strings.Contains(roadmap, want) {
			t.Fatalf("docs/ROADMAP.md must document %q", want)
		}
	}
	if strings.Contains(roadmap, "Remaining:\n\n- #45 Draft a namespace transition readiness plan for") {
		t.Fatal("docs/ROADMAP.md must not list issue #45 as still remaining")
	}
}

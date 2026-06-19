package repohealth

import (
	"strings"
	"testing"
)

func TestAdoptionProposalDocumentsOptionsAndRecommendation(t *testing.T) {
	body := readTextFile(t, "docs/maintainers/ADOPTION_PROPOSAL.md")
	for _, want := range []string{
		"Convex Adoption Proposal",
		"`github.com/cervantesh/convex-go`",
		"`github.com/get-convex/convex-go`",
		"Option 1: Official Link Only",
		"Option 2: Co-Maintained Official Repository",
		"Option 3: New Official Repository From Snapshot",
		"Option 4: Full Transfer Of The Existing Repository",
		"recommended phased path",
		"`go.mod`",
		"`README.md`",
		"`pkg.go.dev`",
		"release/tag continuity",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/maintainers/ADOPTION_PROPOSAL.md must document %q", want)
		}
	}
}

func TestAdoptionProposalIsLinkedAndRoadmapMarksIssueComplete(t *testing.T) {
	maintainers := readTextFile(t, "docs/MAINTAINERS.md")
	if !strings.Contains(maintainers, "ADOPTION_PROPOSAL.md") {
		t.Fatal("docs/MAINTAINERS.md must link ADOPTION_PROPOSAL.md")
	}

	roadmap := readTextFile(t, "docs/ROADMAP.md")
	for _, want := range []string{
		"## Milestone 4 - Convex Adoption Readiness",
		"Completed in this repository:",
		"#47 Draft the official adoption proposal and ownership options for Convex",
	} {
		if !strings.Contains(roadmap, want) {
			t.Fatalf("docs/ROADMAP.md must document %q", want)
		}
	}
	if strings.Contains(roadmap, "Remaining:\n\n- #47 Draft the official adoption proposal and ownership options for Convex") {
		t.Fatal("docs/ROADMAP.md must not list issue #47 as still remaining")
	}
}

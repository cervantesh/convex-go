package repohealth

import (
	"strings"
	"testing"
)

func TestOfficialLinkingRequestDocumentsScopeAndEvidence(t *testing.T) {
	body := readTextFile(t, "docs/maintainers/OFFICIAL_LINKING_REQUEST.md")
	for _, want := range []string{
		"Official Linking Request",
		"recommended community Go client",
		"community-maintained",
		"`github.com/cervantesh/convex-go`",
		"`github.com/get-convex/convex-go`",
		"`docs/ADOPTION_PACKET.md`",
		"`docs/PARITY.md`",
		"`docs/COMPATIBILITY.md`",
		"`docs/CONFORMANCE.md`",
		"`SUPPORT.md`",
		"`SECURITY.md`",
		"not yet an official first-party client",
		"Do not change `go.mod` or the import path as part of this request.",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/maintainers/OFFICIAL_LINKING_REQUEST.md must document %q", want)
		}
	}
}

func TestOfficialLinkingRequestIsLinkedAndRoadmapMarksIssueComplete(t *testing.T) {
	maintainers := readTextFile(t, "docs/MAINTAINERS.md")
	if !strings.Contains(maintainers, "OFFICIAL_LINKING_REQUEST.md") {
		t.Fatal("docs/MAINTAINERS.md must link OFFICIAL_LINKING_REQUEST.md")
	}

	roadmap := readTextFile(t, "docs/ROADMAP.md")
	for _, want := range []string{
		"## Milestone 4 - Convex Adoption Readiness",
		"Completed in this repository:",
		"#46 Request official linking as the recommended community Go client",
	} {
		if !strings.Contains(roadmap, want) {
			t.Fatalf("docs/ROADMAP.md must document %q", want)
		}
	}
	if strings.Contains(roadmap, "Remaining:\n\n- #46 Request official linking as the recommended community Go client") {
		t.Fatal("docs/ROADMAP.md must not list issue #46 as still remaining")
	}
}

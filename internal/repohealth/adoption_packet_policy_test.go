package repohealth

import (
	"strings"
	"testing"
)

func TestAdoptionPacketSummarizesCurrentStateAndEvidence(t *testing.T) {
	body := readTextFile(t, "docs/ADOPTION_PACKET.md")
	for _, want := range []string{
		"Convex Go Client Adoption Packet",
		"community-maintained",
		"pre-v1",
		"not an official first-party Convex client yet",
		"`github.com/cervantesh/convex-go`",
		"`NewClient`",
		"`NewHTTPClient`",
		"`NewWebSocketClient`",
		"`baseclient`",
		"Supported And Not Yet Supported",
		"public root-package logging hooks",
		"Quality Signals",
		"Coverage gate at `>= 90%`",
		"`govulncheck`",
		"`golangci-lint`",
		"CodeQL",
		"Dependabot",
		"Compatibility Evidence",
		"`convex-js`",
		"`convex-py`",
		"`convex-rs`",
		"Maintainer And Support Model",
		"SUPPORT.md",
		"SECURITY.md",
		"maintainers/COMMUNITY.md",
		"maintainers/GOVERNANCE.md",
		"Gaps Before Official Adoption",
		"external adopter validation",
		"`github.com/get-convex/convex-go`",
		"Current Ask To Convex",
		"official linking",
		"co-maintenance",
		"full adoption",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/ADOPTION_PACKET.md must document %q", want)
		}
	}
}

func TestAdoptionPacketIsLinkedAndRoadmapMarksIssueComplete(t *testing.T) {
	maintainers := readTextFile(t, "docs/MAINTAINERS.md")
	if !strings.Contains(maintainers, "ADOPTION_PACKET.md") {
		t.Fatal("docs/MAINTAINERS.md must link ADOPTION_PACKET.md")
	}

	roadmap := readTextFile(t, "docs/ROADMAP.md")
	for _, want := range []string{
		"## Milestone 4 - Convex Adoption Readiness",
		"Completed in this repository:",
		"#43 Create a Convex adoption packet for the Go client",
	} {
		if !strings.Contains(roadmap, want) {
			t.Fatalf("docs/ROADMAP.md must document %q", want)
		}
	}
	if strings.Contains(roadmap, "Remaining:\n\n- #43 Create a Convex adoption packet for the Go client") {
		t.Fatal("docs/ROADMAP.md must not list issue #43 as still remaining")
	}
}

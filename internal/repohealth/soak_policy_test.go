package repohealth

import (
	"strings"
	"testing"
)

func TestCompatibilityMatrixMentionsDeterministicSoakEvidence(t *testing.T) {
	body := readTextFile(t, "docs/COMPATIBILITY.md")
	for _, want := range []string{
		"`subscription_soak_test.go`",
		"`internal/syncclient/websocket_manager_soak_test.go`",
		"auth callback refresh",
		"unsubscribe retry",
		"deterministic reconnect behavior",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/COMPATIBILITY.md must document %q", want)
		}
	}
}

func TestRoadmapMarksDeterministicSoakIssueComplete(t *testing.T) {
	roadmap := readTextFile(t, "docs/ROADMAP.md")
	start := strings.Index(roadmap, "## Milestone 2 - Runtime Reliability")
	end := strings.Index(roadmap, "## Milestone 3 - Adoption")
	if start == -1 || end == -1 || end <= start {
		t.Fatal("docs/ROADMAP.md must contain Milestone 2 and Milestone 3 sections")
	}
	section := strings.Join(strings.Fields(roadmap[start:end]), " ")
	issue := "#32 Add deterministic soak coverage for reconnect, auth refresh, and cancellation"
	completedIndex := strings.Index(section, "Completed in this repository:")
	remainingIndex := strings.Index(section, "Remaining:")
	issueIndex := strings.Index(section, issue)
	if completedIndex == -1 || remainingIndex == -1 || issueIndex == -1 {
		t.Fatal("docs/ROADMAP.md must show issue #32 inside Milestone 2")
	}
	if issueIndex > remainingIndex {
		t.Fatal("docs/ROADMAP.md must list issue #32 as completed, not remaining")
	}
}

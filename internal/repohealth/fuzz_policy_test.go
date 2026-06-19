package repohealth

import (
	"strings"
	"testing"
)

func TestQualityDocsMentionFuzzCommands(t *testing.T) {
	quality := readTextFile(t, "docs/maintainers/QUALITY.md")
	for _, want := range []string{
		"go test ./internal/core -run=^$ -fuzz=Fuzz -fuzztime=10s",
		"go test ./internal/syncprotocol -run=^$ -fuzz=Fuzz -fuzztime=10s",
	} {
		if !strings.Contains(quality, want) {
			t.Fatalf("docs/maintainers/QUALITY.md must document %q", want)
		}
	}
}

func TestRoadmapMarksFuzzIssueComplete(t *testing.T) {
	roadmap := readTextFile(t, "docs/ROADMAP.md")
	start := strings.Index(roadmap, "## Milestone 2 - Runtime Reliability")
	end := strings.Index(roadmap, "## Milestone 3 - Adoption")
	if start == -1 || end == -1 || end <= start {
		t.Fatal("docs/ROADMAP.md must contain Milestone 2 and Milestone 3 sections")
	}
	section := roadmap[start:end]
	issue := "#31 Add fuzz targets for values, wire protocol, and critical conversions"
	completedIndex := strings.Index(section, "Completed in this repository:")
	remainingIndex := strings.Index(section, "Remaining:")
	issueIndex := strings.Index(section, issue)
	if completedIndex == -1 || remainingIndex == -1 || issueIndex == -1 {
		t.Fatal("docs/ROADMAP.md must show issue #31 inside Milestone 2")
	}
	if issueIndex > remainingIndex {
		t.Fatal("docs/ROADMAP.md must list issue #31 as completed, not remaining")
	}
}

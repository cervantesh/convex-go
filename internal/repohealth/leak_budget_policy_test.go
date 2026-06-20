package repohealth

import (
	"strings"
	"testing"
)

func TestQualityDocsMentionLeakAndRetentionBudgetCoverage(t *testing.T) {
	body := readTextFile(t, "docs/maintainers/QUALITY.md")
	for _, want := range []string{
		"Runtime Leak And Retention Budgets",
		"goroutine lifecycle",
		"watcher cleanup",
		"result history",
		"baseclient/retention_test.go",
		"subscription_close_cleanup_test.go",
		"subscription_query_cleanup_test.go",
		"subscription_leak_budget_test.go",
		"internal/syncclient/websocket_manager_leak_budget_test.go",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/maintainers/QUALITY.md must document %q", want)
		}
	}
}

func TestRoadmapMarksLeakBudgetIssueComplete(t *testing.T) {
	roadmap := readTextFile(t, "docs/ROADMAP.md")
	start := strings.Index(roadmap, "## Milestone 2 - Runtime Reliability")
	end := strings.Index(roadmap, "## Milestone 3 - Adoption")
	if start == -1 || end == -1 || end <= start {
		t.Fatal("docs/ROADMAP.md must contain Milestone 2 and Milestone 3 sections")
	}
	section := strings.Join(strings.Fields(roadmap[start:end]), " ")
	issue := "#33 Add leak and retention budget tests for goroutines, watchers, and result history"
	completedIndex := strings.Index(section, "Completed in this repository:")
	remainingIndex := strings.Index(section, "Remaining:")
	issueIndex := strings.Index(section, issue)
	if completedIndex == -1 || remainingIndex == -1 || issueIndex == -1 {
		t.Fatal("docs/ROADMAP.md must show issue #33 inside Milestone 2")
	}
	if issueIndex > remainingIndex {
		t.Fatal("docs/ROADMAP.md must list issue #33 as completed, not remaining")
	}
}

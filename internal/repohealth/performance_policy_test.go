package repohealth

import (
	"strings"
	"testing"
)

func TestMaintainersIndexLinksPerformanceGuide(t *testing.T) {
	body := readTextFile(t, "docs/MAINTAINERS.md")
	if !strings.Contains(body, "maintainers/PERFORMANCE.md") {
		t.Fatal("docs/MAINTAINERS.md must link maintainers/PERFORMANCE.md")
	}
}

func TestPerformanceGuideDocumentsBenchmarksAndBudgetPolicy(t *testing.T) {
	body := readTextFile(t, "docs/maintainers/PERFORMANCE.md")
	for _, want := range []string{
		"Performance Benchmarks",
		"BenchmarkValueJSONRoundTrip",
		"BenchmarkWebSocketClientSubscriptionThroughput",
		"BenchmarkReplayOngoingRequests",
		"bench-compare",
		"max-regression=25",
		"ns/op",
		"B/op",
		"allocs/op",
		"same Go version",
		"shell's normal output redirection",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/maintainers/PERFORMANCE.md must document %q", want)
		}
	}
	for _, blocked := range []string{"```powershell", "$env:", `.ps1`, `C:\`} {
		if strings.Contains(body, blocked) {
			t.Fatalf("docs/maintainers/PERFORMANCE.md must stay shell-neutral and workstation-neutral; found %q", blocked)
		}
	}
}

func TestRoadmapMarksPerformanceIssueComplete(t *testing.T) {
	roadmap := readTextFile(t, "docs/ROADMAP.md")
	start := strings.Index(roadmap, "## Milestone 2 - Runtime Reliability")
	end := strings.Index(roadmap, "## Milestone 3 - Adoption")
	if start == -1 || end == -1 || end <= start {
		t.Fatal("docs/ROADMAP.md must contain Milestone 2 and Milestone 3 sections")
	}
	section := strings.Join(strings.Fields(roadmap[start:end]), " ")
	issue := "#34 Add benchmarks and performance budgets for values, subscribe throughput, and reconnect"
	completedIndex := strings.Index(section, "Completed in this repository:")
	remainingIndex := strings.Index(section, "Remaining:")
	issueIndex := strings.Index(section, issue)
	if completedIndex == -1 || remainingIndex == -1 || issueIndex == -1 {
		t.Fatal("docs/ROADMAP.md must show issue #34 inside Milestone 2")
	}
	if issueIndex > remainingIndex {
		t.Fatal("docs/ROADMAP.md must list issue #34 as completed, not remaining")
	}
}

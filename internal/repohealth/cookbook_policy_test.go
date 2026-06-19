package repohealth

import (
	"strings"
	"testing"
)

func TestCookbookCoversOperationalPatterns(t *testing.T) {
	body := readTextFile(t, "docs/RECIPES.md")
	for _, want := range []string{
		"Community Cookbook",
		"HTTP-Only Server Handler",
		"Bearer Auth With Rotation",
		"Refreshable User Auth Callback",
		"Admin Auth With Acting-As Identity",
		"Connection State Monitoring",
		"Subscription Lifecycle",
		"Error Classification At Boundaries",
		"Pagination Loop",
		"Typed References",
		"Consistent Query And Function Calls",
		"Realtime-Only Client With Optimistic Update",
		"`convex.NewClient`",
		"`NewHTTPClient`",
		"`NewWebSocketClient`",
		"`SetAuthCallback`",
		"`ConnectionState`",
		"`WithOptimisticUpdate`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/RECIPES.md must document %q", want)
		}
	}
}

func TestCookbookIsLinkedAndRoadmapMarksIssueComplete(t *testing.T) {
	readme := readTextFile(t, "README.md")
	if !strings.Contains(readme, "docs/RECIPES.md") {
		t.Fatal("README.md must link docs/RECIPES.md")
	}

	roadmap := readTextFile(t, "docs/ROADMAP.md")
	for _, want := range []string{
		"## Milestone 3 - Adoption",
		"Completed in this repository:",
		"#37 Expand public recipes into a complete Go cookbook",
	} {
		if !strings.Contains(roadmap, want) {
			t.Fatalf("docs/ROADMAP.md must document %q", want)
		}
	}
	if strings.Contains(roadmap, "Remaining:\n\n- #37 Expand public recipes into a complete Go cookbook") {
		t.Fatal("docs/ROADMAP.md must not list issue #37 as still remaining")
	}
}

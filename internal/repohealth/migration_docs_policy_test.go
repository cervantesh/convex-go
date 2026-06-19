package repohealth

import (
	"strings"
	"testing"
)

func TestMigrationGuideCoversOfficialClientMappings(t *testing.T) {
	body := readTextFile(t, "docs/MIGRATION.md")
	for _, want := range []string{
		"Migration Guides",
		"convex-js",
		"convex-rs",
		"convex-py",
		"`convex.NewClient`",
		"`Client.Query`",
		"`Client.Mutation`",
		"`Client.Action`",
		"`Client.Subscribe`",
		"`Client.Close`",
		"`SetAuthCallback`",
		"`SetAuth`",
		"`ClearAuth`",
		"`SetAdminAuth`",
		"`ConnectionState()`",
		"`SubscribeToConnectionState(...)`",
		"`context.Context`",
		"`Watch`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/MIGRATION.md must document %q", want)
		}
	}
}

func TestMigrationGuideIsLinkedAndRoadmapMarksIssueComplete(t *testing.T) {
	readme := readTextFile(t, "README.md")
	if !strings.Contains(readme, "docs/MIGRATION.md") {
		t.Fatal("README.md must link docs/MIGRATION.md")
	}

	usage := readTextFile(t, "docs/USAGE.md")
	if !strings.Contains(usage, "MIGRATION.md") {
		t.Fatal("docs/USAGE.md must link MIGRATION.md")
	}

	roadmap := readTextFile(t, "docs/ROADMAP.md")
	for _, want := range []string{
		"## Milestone 3 - Adoption",
		"Completed in this repository:",
		"#38 Publish migration guides from convex-js, convex-rs, and convex-py",
	} {
		if !strings.Contains(roadmap, want) {
			t.Fatalf("docs/ROADMAP.md must document %q", want)
		}
	}
	if strings.Contains(roadmap, "Remaining:\n\n- #38 Publish migration guides from convex-js, convex-rs, and convex-py") {
		t.Fatal("docs/ROADMAP.md must not list issue #38 as still remaining")
	}
}

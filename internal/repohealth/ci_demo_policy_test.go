package repohealth

import (
	"strings"
	"testing"
)

func TestCIRunsPublicDemoSmoke(t *testing.T) {
	ci := readTextFile(t, ".github/workflows/ci.yml")
	for _, want := range []string{
		"name: Demo module tidy",
		"working-directory: examples/realtime_chat",
		"name: Demo smoke",
	} {
		if !strings.Contains(ci, want) {
			t.Fatalf(".github/workflows/ci.yml must contain %q", want)
		}
	}
	if strings.Count(ci, "working-directory: examples/realtime_chat") < 2 {
		t.Fatal(".github/workflows/ci.yml must run more than one smoke step in examples/realtime_chat")
	}
}

func TestQualityDocsMentionDemoSmoke(t *testing.T) {
	quality := readTextFile(t, "docs/maintainers/QUALITY.md")
	for _, want := range []string{
		"`go mod tidy -diff` in `examples/realtime_chat`",
		"`go test ./... -count=1` in `examples/realtime_chat`",
	} {
		if !strings.Contains(quality, want) {
			t.Fatalf("docs/maintainers/QUALITY.md must document %q", want)
		}
	}
}

func TestRoadmapMarksSmokeCoverageIssueComplete(t *testing.T) {
	roadmap := readTextFile(t, "docs/ROADMAP.md")
	for _, want := range []string{
		"## Milestone 3 - Adoption",
		"#40 Add CI smoke coverage for demos and public examples",
	} {
		if !strings.Contains(roadmap, want) {
			t.Fatalf("docs/ROADMAP.md must document %q", want)
		}
	}
	if strings.Contains(roadmap, "Remaining:\n\n- #40 Add CI smoke coverage for demos and public examples") {
		t.Fatal("docs/ROADMAP.md must not list issue #40 as still remaining")
	}
}

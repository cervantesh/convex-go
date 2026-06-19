package repohealth

import (
	"strings"
	"testing"
)

func TestHandoffGateDocumentsBlockingConditionsAndEvidence(t *testing.T) {
	body := readTextFile(t, "docs/maintainers/HANDOFF_GATE.md")
	for _, want := range []string{
		"Official Handoff Gate",
		"Convex explicitly agrees to evaluate real adoption work.",
		"`github.com/cervantesh/convex-go`",
		"`#30` live integration harness coverage",
		"`#31` fuzz targets",
		"`#32` deterministic soak coverage",
		"`#33` leak and retention budgets",
		"`#34` benchmarks and performance budgets",
		"`#36` feed live outcomes back into offline fixtures",
		"`#37` cookbook",
		"`#38` migration guides",
		"`#39` public demo app",
		"`#40` demo/example CI smoke coverage",
		"`#42` external adopter validation",
		"`#43` adoption packet",
		"`#44` governance policy",
		"`#45` namespace transition readiness",
		"`#46` official linking request",
		"`#47` adoption proposal",
		"`github.com/get-convex/convex-go`",
		"Do not open `#49` through `#53` as active implementation work before this",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/maintainers/HANDOFF_GATE.md must document %q", want)
		}
	}
}

func TestHandoffGateIsLinkedAndRoadmapMarksIssueComplete(t *testing.T) {
	maintainers := readTextFile(t, "docs/MAINTAINERS.md")
	if !strings.Contains(maintainers, "HANDOFF_GATE.md") {
		t.Fatal("docs/MAINTAINERS.md must link HANDOFF_GATE.md")
	}

	roadmap := readTextFile(t, "docs/ROADMAP.md")
	for _, want := range []string{
		"## Milestone 4 - Convex Adoption Readiness",
		"Completed in this repository:",
		"#48 Define the handoff gate that must be met before official adoption work",
	} {
		if !strings.Contains(roadmap, want) {
			t.Fatalf("docs/ROADMAP.md must document %q", want)
		}
	}
	if strings.Contains(roadmap, "Remaining:\n\n- #48 Define the handoff gate that must be met before official adoption work") {
		t.Fatal("docs/ROADMAP.md must not list issue #48 as still remaining")
	}
}

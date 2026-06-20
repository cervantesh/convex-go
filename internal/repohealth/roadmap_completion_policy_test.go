package repohealth

import (
	"strings"
	"testing"
)

func TestRoadmapDocumentsClosedRepositoryBacklog(t *testing.T) {
	body := readTextFile(t, "docs/ROADMAP.md")
	for _, want := range []string{
		"As of 2026-06-20, repository-owned milestones 0-4 are complete.",
		"The original umbrella tracker is closed; this file is the durable roadmap",
		"- None. The repository-owned portion of Milestone 3 is complete.",
		"External dependency, not kept open as a standing repository issue:",
		"## Milestone 5 - External Handoff Contingencies",
		"This milestone is intentionally not tracked as open repository issues today.",
		"No repository-owned implementation issues remain open after Milestones 0-4.",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/ROADMAP.md must document %q", want)
		}
	}
	for _, blocked := range []string{
		"- #42 Run an external adopter validation program for the Go client",
		"- #49 Choose the handoff form after Convex accepts adoption",
		"- #50 Execute the module path migration to `github.com/get-convex/convex-go`",
		"- #51 Publish transition releases under the legacy and official namespaces",
		"- #52 Update docs, demos, and official links after handoff",
		"- #53 Publish the post-handoff v1 roadmap for the official Go client",
	} {
		if strings.Contains(body, blocked) {
			t.Fatalf("docs/ROADMAP.md must not keep external-only items as open issue bullets: %q", blocked)
		}
	}
}

func TestAdoptionPacketDoesNotClaimCompletedAdoptionWorkIsStillMissing(t *testing.T) {
	body := readTextFile(t, "docs/ADOPTION_PACKET.md")
	for _, want := range []string{
		"These are future adoption dependencies, not open implementation issues in this",
		"external adopter validation with real users outside maintainer-controlled",
		"execution of the prepared `github.com/get-convex/convex-go` transition only",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/ADOPTION_PACKET.md must document %q", want)
		}
	}
	for _, blocked := range []string{
		"public cookbook and migration guides",
		"demo application and demo CI smoke coverage",
		"namespace transition readiness under `github.com/get-convex/convex-go`",
	} {
		if strings.Contains(body, blocked) {
			t.Fatalf("docs/ADOPTION_PACKET.md must not describe completed repository work as a remaining gap: %q", blocked)
		}
	}
}

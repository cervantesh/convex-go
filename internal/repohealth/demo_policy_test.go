package repohealth

import (
	"strings"
	"testing"
)

func TestPublicDemoIsDocumented(t *testing.T) {
	readme := readTextFile(t, "README.md")
	if !strings.Contains(readme, "examples/realtime_chat/README.md") {
		t.Fatal("README.md must link the public realtime chat demo")
	}

	demoReadme := readTextFile(t, "examples/realtime_chat/README.md")
	for _, want := range []string{
		"# Realtime Chat Demo",
		"CONVEX_URL",
		"CONVEX_AUTH_TOKEN",
		"go run . -room general",
		"live:listMessages",
		"live:sendMessage",
		"testdata/live-integration/convex/",
	} {
		if !strings.Contains(demoReadme, want) {
			t.Fatalf("examples/realtime_chat/README.md must document %q", want)
		}
	}

	goMod := readTextFile(t, "examples/realtime_chat/go.mod")
	for _, want := range []string{
		"module github.com/cervantesh/convex-go/examples/realtime_chat",
		"require github.com/cervantesh/convex-go v0.1.0",
		"replace github.com/cervantesh/convex-go => ../..",
	} {
		if !strings.Contains(goMod, want) {
			t.Fatalf("examples/realtime_chat/go.mod must document %q", want)
		}
	}
}

func TestRoadmapMarksDemoIssueComplete(t *testing.T) {
	roadmap := readTextFile(t, "docs/ROADMAP.md")
	for _, want := range []string{
		"## Milestone 3 - Adoption",
		"Completed in this repository:",
		"#39 Publish a public demo app that uses the SDK as an application dependency",
	} {
		if !strings.Contains(roadmap, want) {
			t.Fatalf("docs/ROADMAP.md must document %q", want)
		}
	}
	if strings.Contains(roadmap, "Remaining:\n\n- #39 Publish a public demo app that uses the SDK as an application dependency") {
		t.Fatal("docs/ROADMAP.md must not list issue #39 as still remaining")
	}
}

package repohealth

import (
	"strings"
	"testing"
)

func TestRoadmapMarksLiveHarnessIssuesComplete(t *testing.T) {
	body := readTextFile(t, "docs/ROADMAP.md")
	for _, want := range []string{
		"#30 Expand the live integration harness to full request, auth, and reconnect",
		"#36 Feed live harness outcomes back into offline conformance fixtures",
		"- None. Milestone 2 is complete.",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/ROADMAP.md must document %q", want)
		}
	}
	for _, blocked := range []string{
		"Remaining:\n\n- #30 Expand the live integration harness to full request, auth, and reconnect",
		"Remaining:\n\n- #36 Feed live harness outcomes back into offline conformance fixtures",
	} {
		if strings.Contains(body, blocked) {
			t.Fatalf("docs/ROADMAP.md must not keep completed live harness work under Remaining: %q", blocked)
		}
	}
}

func TestConformanceDocsDescribeLiveHarnessFeedbackLoop(t *testing.T) {
	body := readTextFile(t, "docs/CONFORMANCE.md")
	for _, want := range []string{
		"## Live Harness Feedback Loop",
		"`live_integration_test.go`",
		"`TestLiveIntegrationHTTPAndSync`",
		"`TestLiveIntegrationAuthCallbackAndReconnect`",
		"`client_auth_callback_test.go`",
		"`baseclient/reconnect_test.go`",
		"`internal/syncclient/websocket_manager_test.go`",
		"[COMPATIBILITY.md](COMPATIBILITY.md)",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/CONFORMANCE.md must document %q", want)
		}
		}
}

package repohealth

import (
	"strings"
	"testing"
)

func TestGitHubNativeAutomationFilesExist(t *testing.T) {
	required := map[string][]string{
		".github/dependabot.yml": {
			"package-ecosystem: gomod",
			"package-ecosystem: github-actions",
			"interval: weekly",
			"groups:",
		},
		".github/workflows/codeql.yml": {
			"name: CodeQL",
			"workflow_dispatch:",
			"github/codeql-action/init@v4",
			"github/codeql-action/analyze@v4",
			"queries: security-and-quality",
			"go build ./...",
		},
		"SECURITY.md": {
			"GitHub private vulnerability reporting is enabled",
			"Security report requested.",
		},
		"docs/MAINTAINERS.md": {
			".github/",
			"Dependabot",
			"CodeQL",
		},
	}

	for path, phrases := range required {
		body := readTextFile(t, path)
		for _, want := range phrases {
			if !strings.Contains(body, want) {
				t.Fatalf("%s must contain %q", path, want)
			}
		}
	}
}

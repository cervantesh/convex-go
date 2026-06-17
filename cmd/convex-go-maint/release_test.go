package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWithIOReleaseCheckWritesNotes(t *testing.T) {
	root := releaseTestRepo(t, "0.1.0", "# Changelog\n\n## Unreleased\n\n- No unreleased changes yet.\n\n## 0.1.0 - 2026-06-17\n\n- Initial public release.\n- Portable release workflow.\n")
	notesPath := filepath.Join(root, "release-notes.md")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := runWithIO([]string{
		"release-check",
		"-repo", root,
		"-version", "v0.1.0",
		"-notes-out", notesPath,
	}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "validated release v0.1.0") {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	notes, err := os.ReadFile(notesPath)
	if err != nil {
		t.Fatal(err)
	}
	body := string(notes)
	for _, want := range []string{"Initial public release.", "Portable release workflow."} {
		if !strings.Contains(body, want) {
			t.Fatalf("release notes missing %q in %q", want, body)
		}
	}
}

func TestRunWithIOReleaseCheckRejectsVersionMismatch(t *testing.T) {
	root := releaseTestRepo(t, "0.1.0", "# Changelog\n\n## 0.1.1 - 2026-06-17\n\n- Notes.\n")
	err := runWithIO([]string{"release-check", "-repo", root, "-version", "0.1.1"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "version mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithIOReleaseCheckRejectsMissingChangelogSection(t *testing.T) {
	root := releaseTestRepo(t, "0.1.0", "# Changelog\n\n## Unreleased\n\n- Notes.\n")
	err := runWithIO([]string{"release-check", "-repo", root, "-version", "0.1.0"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "does not contain a section") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithIOReleaseCheckRejectsExistingTag(t *testing.T) {
	root := releaseTestRepo(t, "0.2.0-rc.1", "# Changelog\n\n## 0.2.0-rc.1 - 2026-06-17\n\n- Release candidate.\n")
	runGit(t, root, "tag", "v0.2.0-rc.1")
	err := runWithIO([]string{"release-check", "-repo", root, "-version", "0.2.0-rc.1"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func releaseTestRepo(t *testing.T, version, changelog string) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.name", "Test User")
	runGit(t, root, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(root, "client_options.go"), []byte("package convex\n\nconst Version = \""+version+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(changelog), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "init")
	return root
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}

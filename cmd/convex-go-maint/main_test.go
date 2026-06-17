package main

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCoverageProfileComputesTotal(t *testing.T) {
	total, err := parseCoverageProfile(strings.TrimSpace(`
mode: set
client.go:10.1,12.2 2 1
client.go:14.1,16.2 3 0
subscription.go:20.1,24.2 5 7
`))
	if err != nil {
		t.Fatal(err)
	}
	if total != 70 {
		t.Fatalf("coverage total = %v, want 70", total)
	}
}

func TestParseCoverageProfileRejectsInvalidInput(t *testing.T) {
	if _, err := parseCoverageProfile("client.go:10.1,12.2 2 1"); err == nil {
		t.Fatal("expected invalid profile error")
	}
	if _, err := parseCoverageProfile("mode: set\nbad-line"); err == nil {
		t.Fatal("expected invalid line error")
	}
}

func TestListUnformattedFilesFindsOnlyBadFiles(t *testing.T) {
	root := t.TempDir()
	good := filepath.Join(root, "good.go")
	bad := filepath.Join(root, "bad.go")
	if err := os.WriteFile(good, []byte("package main\n\nfunc ok() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("package main\nfunc bad( ) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := listUnformattedFiles(root, []string{"good.go", "bad.go"})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "bad.go" {
		t.Fatalf("unformatted files = %#v, want [bad.go]", files)
	}
}

func TestRunWithIOCoverageCheck(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "coverage.out")
	if err := os.WriteFile(profile, []byte("mode: set\nclient.go:10.1,12.2 2 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := runWithIO([]string{"coverage-check", "-coverprofile", profile, "-min", "90"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "meets minimum 90.0%") {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
}

func TestRunWithIOCoverageCheckFailure(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "coverage.out")
	if err := os.WriteFile(profile, []byte("mode: set\nclient.go:10.1,12.2 2 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runWithIO([]string{"coverage-check", "-coverprofile", profile, "-min", "90"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "below 90.0%") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithIOHelp(t *testing.T) {
	var stderr bytes.Buffer
	err := runWithIO([]string{"help"}, &bytes.Buffer{}, &stderr)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
	if !strings.Contains(stderr.String(), "convex-go-maint") {
		t.Fatalf("unexpected help: %q", stderr.String())
	}
}

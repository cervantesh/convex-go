package repohealth

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootDoesNotOwnRepoHealthTests(t *testing.T) {
	root := repoRoot(t)
	rootPolicyTest := filepath.Join(root, "quality_policy_test.go")
	if _, err := os.Stat(rootPolicyTest); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("repo-health tests must not live in package root; move %s to internal/repohealth", rootPolicyTest)
	}

	repoHealthPolicyTest := filepath.Join(root, "internal", "repohealth", "quality_policy_test.go")
	if _, err := os.Stat(repoHealthPolicyTest); err != nil {
		t.Fatalf("repo-health package must own quality policy tests at %s: %v", repoHealthPolicyTest, err)
	}
}

func TestRootDoesNotOwnDocumentationPolicyTests(t *testing.T) {
	root := repoRoot(t)
	rootTests, err := filepath.Glob(filepath.Join(root, "*_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range rootTests {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{
			"TestPackageDocumentationDeclaresAPITiers",
			"TestArchitectureDocsExplainRootPackageLayout",
		} {
			if strings.Contains(string(body), forbidden) {
				t.Fatalf("documentation policy test %s must live in internal/repohealth, not %s", forbidden, path)
			}
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found while resolving repository root")
		}
		dir = parent
	}
}

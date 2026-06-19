package repohealth

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestPackageDocumentationDeclaresAPITiers(t *testing.T) {
	body := readTextFile(t, "doc.go")
	for _, want := range []string{
		"Package convex",
		"Community Go client",
		"Primary APIs",
		"Typed and generated APIs",
		"Realtime APIs",
		"Advanced protocol APIs",
		"github.com/cervantesh/convex-go/baseclient",
		"not an official first-party Convex client",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("doc.go must document %q", want)
		}
	}
}

func TestReadmeIntroducesBaseClientAfterMainClientPath(t *testing.T) {
	body := readTextFile(t, "README.md")
	assertTextOrder(t, body, "README.md", []string{
		"## Installation",
		"## Example",
		"## Documentation",
		"## Supported Go Version",
		"## Community / Support",
	})
	for _, want := range []string{
		"convex.NewClient",
		"docs/USAGE.md",
		"docs/PARITY.md",
		"docs/CONFORMANCE.md",
		"docs/ARCHITECTURE.md",
		"docs/MAINTAINERS.md",
		"baseclient",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("README.md must document %q", want)
		}
	}
	for _, blocked := range []string{
		"## API Map",
		"## Explicit HTTP Client",
		"## Application Errors",
		"## Value Mapping",
		"## Pagination",
		"## Advanced Base Client",
	} {
		if strings.Contains(body, blocked) {
			t.Fatalf("README.md must stay concise and must not keep legacy section %q", blocked)
		}
	}
}

func TestReadmeSeparatesPublicAndMaintainerDocs(t *testing.T) {
	body := readTextFile(t, "README.md")
	for _, want := range []string{
		"docs/USAGE.md",
		"docs/PARITY.md",
		"docs/CONFORMANCE.md",
		"docs/ARCHITECTURE.md",
		"docs/MAINTAINERS.md",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("README.md must document %q", want)
		}
	}
	for _, blocked := range []string{
		"docs/maintainers/",
		"docs/AGENTS.md",
		"roadmap and GitHub issue plan",
	} {
		if strings.Contains(body, blocked) {
			t.Fatalf("README.md must not document %q", blocked)
		}
	}
}

func TestUsageDocsCoverPublicHowTo(t *testing.T) {
	body := readTextFile(t, "docs/USAGE.md")
	for _, want := range []string{
		"convex.NewClient",
		"SetAuth",
		"SetAuthCallback",
		"HTTPError",
		"ConvexError",
		"Number",
		"QueryInto",
		"NewQueryReference",
		"cmd/convex-go-codegen",
		"NewHTTPClient",
		"NewWebSocketClient",
		"WithOptimisticUpdate",
		"baseclient",
		"RECIPES.md",
		"examples_test.go",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/USAGE.md must document %q", want)
		}
	}
}

func TestTypedReferenceDocsDefineStableCodegenContract(t *testing.T) {
	usage := readTextFile(t, "docs/USAGE.md")
	for _, want := range []string{
		"Typed references are a supported root-package API",
		"cmd/convex-go-codegen",
		"argument types default to `map[string]any`",
		"result types use lightweight offline inference",
		"generic Go shapes such as `any`, `[]any`, or `map[string]any`",
		"TypeScript validator inference",
	} {
		if !strings.Contains(usage, want) {
			t.Fatalf("docs/USAGE.md must document %q", want)
		}
	}

	parity := readTextFile(t, "docs/PARITY.md")
	for _, want := range []string{
		"| Typed references and offline codegen | Supported |",
		"deterministic generic Go shapes",
	} {
		if !strings.Contains(parity, want) {
			t.Fatalf("docs/PARITY.md must document %q", want)
		}
	}
}

func TestReadmeExampleHasCompiledMirror(t *testing.T) {
	body := readTextFile(t, "examples_test.go")
	for _, want := range []string{
		"func ExampleClient_overview()",
		"convex.NewClient",
		`client.Query(ctx, "messages:list"`,
		`client.Mutation(ctx, "messages:send"`,
		`client.Subscribe(ctx, "messages:list", nil)`,
		"client.Close()",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("examples_test.go must keep README example mirror for %q", want)
		}
	}
}

func TestVersionedDocsDoNotMentionLocalGitHubWrapper(t *testing.T) {
	root := repoRoot(t)
	paths := []string{
		"README.md",
		"CONTRIBUTING.md",
		"doc.go",
		filepath.ToSlash(filepath.Join("baseclient", "doc.go")),
	}
	if err := filepath.WalkDir(filepath.Join(root, "docs"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		body := readTextFile(t, path)
		for _, blocked := range []string{"gh-cervantesh", "tools/gh-cervantesh", "tools\\gh-cervantesh"} {
			if strings.Contains(body, blocked) {
				t.Fatalf("%s must not document local-only GitHub wrapper %q", path, blocked)
			}
		}
	}
}

func TestInternalPlanningDocsAreNotTracked(t *testing.T) {
	for _, rel := range []string{
		"docs/AGENTS.md",
		"docs/superpowers",
	} {
		if tracked := gitTrackedPaths(t, rel); len(tracked) > 0 {
			t.Fatalf("%s must not be tracked in the public repository", rel)
		}
	}
}

func TestVersionedDocsAvoidLaptopPaths(t *testing.T) {
	root := repoRoot(t)
	paths := []string{
		filepath.Join(root, "README.md"),
		filepath.Join(root, "CONTRIBUTING.md"),
	}
	if err := filepath.WalkDir(filepath.Join(root, "docs"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	drivePathPattern := regexp.MustCompile(`\b[A-Za-z]:\\`)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if drivePathPattern.Match(data) {
			t.Fatalf("%s must not contain workstation-specific absolute Windows paths", path)
		}
	}
}

func TestVersionedDocsAvoidPowerShellSpecificExamples(t *testing.T) {
	root := repoRoot(t)
	paths := []string{
		"README.md",
		"CONTRIBUTING.md",
	}
	if err := filepath.WalkDir(filepath.Join(root, "docs"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		body := readTextFile(t, path)
		for _, blocked := range []string{"```powershell", "$env:", ".ps1", "pwsh"} {
			if strings.Contains(body, blocked) {
				t.Fatalf("%s must not contain PowerShell-specific example %q", path, blocked)
			}
		}
	}
}

func assertTextOrder(t *testing.T, body, path string, wants []string) {
	t.Helper()
	last := -1
	for _, want := range wants {
		index := strings.Index(body, want)
		if index < 0 {
			t.Fatalf("%s must document %q", path, want)
		}
		if index < last {
			t.Fatalf("%s must document %q after the prior required section", path, want)
		}
		last = index
	}
}

func TestParityDocsStayStatusFocused(t *testing.T) {
	body := readTextFile(t, "docs/PARITY.md")
	for _, want := range []string{
		"Compatibility Status",
		"Supported Surface",
		"Public Contract",
		"Backed by Tests",
		"Known Limits",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/PARITY.md must document %q", want)
		}
	}
	for _, blocked := range []string{
		"GitHub Backlog",
		"Issue [#",
	} {
		if strings.Contains(body, blocked) {
			t.Fatalf("docs/PARITY.md must not document %q", blocked)
		}
	}
}

func TestContributingDocsDefinePreV1APIFreeze(t *testing.T) {
	body := readTextFile(t, "CONTRIBUTING.md")
	for _, want := range []string{
		"## Public API Freeze",
		"`NewClient`",
		"`Subscribe`",
		"`NewHTTPClient`",
		"`NewWebSocketClient`",
		"`baseclient`",
		"`WatchAll`",
		"`Watch`",
		"API surface test update",
		"migration notes",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("CONTRIBUTING.md must document %q", want)
		}
	}
}

func TestMaintainerDevelopmentDocsDefineAPIFreezeWorkflow(t *testing.T) {
	body := readTextFile(t, "docs/maintainers/DEVELOPMENT.md")
	for _, want := range []string{
		"## Public API Freeze Workflow",
		"`NewClient`",
		"`Subscribe`",
		"`NewHTTPClient`",
		"`NewWebSocketClient`",
		"`baseclient`",
		"`WatchAll`",
		"`Watch`",
		"`api_surface_test.go`",
		"`CHANGELOG.md`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/maintainers/DEVELOPMENT.md must document %q", want)
		}
	}
}

func TestRoadmapTracksPublicRepoAsSourceOfTruth(t *testing.T) {
	body := readTextFile(t, "docs/ROADMAP.md")
	for _, want := range []string{
		"Milestone 0 is complete",
		"source of truth",
		"Completed in this repository",
		"Remaining:",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/ROADMAP.md must document %q", want)
		}
	}
	for _, blocked := range []string{
		"The remaining Milestone 0 gap",
		"must stop drifting",
		"convex-go-incubation",
	} {
		if strings.Contains(body, blocked) {
			t.Fatalf("docs/ROADMAP.md must not document stale source-of-truth text %q", blocked)
		}
	}
}

func TestVersionedDocsDoNotNameOldIncubationRepo(t *testing.T) {
	root := repoRoot(t)
	paths := []string{
		"README.md",
		"CONTRIBUTING.md",
	}
	if err := filepath.WalkDir(filepath.Join(root, "docs"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		body := readTextFile(t, path)
		if strings.Contains(body, "convex-go-incubation") {
			t.Fatalf("%s must not name the old incubation repository", path)
		}
	}
}

func TestReleaseChecklistReferencesCurrentVersionFile(t *testing.T) {
	body := readTextFile(t, "docs/maintainers/RELEASE.md")
	if !strings.Contains(body, "client_options.go") {
		t.Fatal("docs/maintainers/RELEASE.md must point at client_options.go for the version constant")
	}
	if strings.Contains(body, "`client.go`") {
		t.Fatal("docs/maintainers/RELEASE.md must not reference the removed client.go file")
	}
}

func TestArchitectureDocsExplainRootPackageLayout(t *testing.T) {
	body := readTextFile(t, "docs/ARCHITECTURE.md")
	for _, want := range []string{
		"Package root",
		"Primary user APIs",
		"Value and error model",
		"Realtime and sync APIs",
		"Advanced protocol APIs",
		"github.com/cervantesh/convex-go/baseclient",
		"Do not move public exports",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docs/ARCHITECTURE.md must document %q", want)
		}
	}
}

func gitTrackedPaths(t *testing.T, rel string) []string {
	t.Helper()
	root := repoRoot(t)
	cmd := exec.Command("git", "ls-files", "--", filepath.ToSlash(rel))
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files %s: %v", rel, err)
	}
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return nil
	}
	return strings.Fields(trimmed)
}

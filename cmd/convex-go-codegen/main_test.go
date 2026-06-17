package main

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWritesGeneratedFile(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "convex")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "messages.ts"), []byte(`export const list = query({});`), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(root, "convexapi", "api.go")

	if err := run([]string{
		"-convex-dir", sourceDir,
		"-package", "convexapi",
		"-out", outPath,
	}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `var MessagesList = convex.NewQueryReference[map[string]any, any]("messages:list")`) {
		t.Fatalf("unexpected generated body:\n%s", body)
	}
}

func TestRunDryRunAndCheck(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "convex")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "messages.ts"), []byte(`export const list = query({});`), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(root, "convexapi", "api.go")

	var stdout bytes.Buffer
	if err := runWithIO([]string{
		"-src", sourceDir,
		"-package", "convexapi",
		"-dry-run",
	}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "MessagesList") {
		t.Fatalf("dry-run did not print generated refs:\n%s", stdout.String())
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not write output, stat err=%v", err)
	}

	if err := run([]string{"-src", sourceDir, "-out", outPath}); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-src", sourceDir, "-out", outPath, "-check"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outPath, []byte("package stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-src", sourceDir, "-out", outPath, "-check"}); err == nil {
		t.Fatal("expected check mode to reject stale generated file")
	}
}

func TestRunRequiresOutputPath(t *testing.T) {
	if err := run([]string{"-convex-dir", t.TempDir()}); err == nil {
		t.Fatal("expected missing -out error")
	}
}

func TestRunReportsGenerateAndCheckErrors(t *testing.T) {
	var stderr bytes.Buffer
	if err := runWithIO([]string{"-src", filepath.Join(t.TempDir(), "missing"), "-dry-run"}, &bytes.Buffer{}, &stderr); err == nil {
		t.Fatal("expected missing source dir error")
	}
	if !strings.Contains(stderr.String(), "source dir") {
		t.Fatalf("expected source dir error on stderr, got %q", stderr.String())
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "messages.ts"), []byte(`export const list = query({});`), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if err := runWithIO([]string{"-src", dir, "-out", filepath.Join(dir, "missing.go"), "-check"}, &bytes.Buffer{}, &stderr); err == nil {
		t.Fatal("expected check mode missing file error")
	}
	if stderr.Len() == 0 {
		t.Fatal("expected check error on stderr")
	}
}

func TestRunHelpIncludesFlagsAndDefaults(t *testing.T) {
	var stderr bytes.Buffer
	err := runWithIO([]string{"-h"}, &bytes.Buffer{}, &stderr)
	if !errors.Is(err, flagErrHelp()) {
		t.Fatalf("expected flag help error, got %v", err)
	}
	help := stderr.String()
	for _, want := range []string{
		"convex-go-codegen",
		"-convex-dir",
		"path to the Convex functions directory",
		"-src",
		"-package",
		"generated Go package name",
		"-import",
		"github.com/cervantesh/convex-go",
		"-convex-import",
		"-out",
		"output Go file path",
		"-dry-run",
		"write generated Go to stdout instead of a file",
		"-check",
		"fail if the output file differs from generated content",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help output missing %q:\n%s", want, help)
		}
	}
	if strings.Count(help, `(default "convex")`) != 2 {
		t.Fatalf("help output should show both source-dir aliases default to convex:\n%s", help)
	}
	if strings.Count(help, "path to the Convex functions directory") != 2 {
		t.Fatalf("help output should describe both source-dir aliases:\n%s", help)
	}
	if strings.Count(help, "convex-go module import path") != 2 {
		t.Fatalf("help output should describe both import path aliases:\n%s", help)
	}
	for _, want := range []string{
		"\n  -convex-import string\n",
		"\n  -import string\n",
		`(default "github.com/cervantesh/convex-go")`,
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help output missing exact flag snippet %q:\n%s", want, help)
		}
	}
}

func TestRunWithIOReportsWriteErrors(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "convex")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "messages.ts"), []byte(`export const list = query({});`), 0o644); err != nil {
		t.Fatal(err)
	}

	wantErr := errors.New("stdout failed")
	err := runWithIO([]string{"-src", sourceDir, "-dry-run"}, errWriter{err: wantErr}, &bytes.Buffer{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected stdout write error, got %v", err)
	}

	blockingParent := filepath.Join(root, "file-parent")
	if err := os.WriteFile(blockingParent, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	err = runWithIO([]string{"-src", sourceDir, "-out", filepath.Join(blockingParent, "api.go")}, &bytes.Buffer{}, &stderr)
	if err == nil {
		t.Fatal("expected mkdir/write error")
	}
	if stderr.Len() == 0 {
		t.Fatal("expected filesystem error on stderr")
	}

	outPathIsDir := filepath.Join(root, "out-is-dir")
	if err := os.Mkdir(outPathIsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	err = runWithIO([]string{"-src", sourceDir, "-out", outPathIsDir}, &bytes.Buffer{}, &stderr)
	if err == nil {
		t.Fatal("expected write error when output path is an existing directory")
	}
	if stderr.Len() == 0 {
		t.Fatal("expected write error on stderr")
	}
}

func TestRunWithIOAliasesAndExactMissingOutError(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "convex")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "jobs.js"), []byte(`export default action({});`), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	if err := runWithIO([]string{
		"-convex-dir", sourceDir,
		"-convex-import", "example.com/convex-go",
		"-package", "customapi",
		"-dry-run",
	}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	body := stdout.String()
	for _, want := range []string{
		"package customapi",
		`import convex "example.com/convex-go"`,
		`var JobsDefault = convex.NewActionReference[map[string]any, any]("jobs:default")`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("generated dry-run body missing %q:\n%s", want, body)
		}
	}

	var stderr bytes.Buffer
	err := runWithIO([]string{"-src", sourceDir}, &bytes.Buffer{}, &stderr)
	if err == nil || err.Error() != "convex-go-codegen: -out is required" {
		t.Fatalf("unexpected missing out error: %v", err)
	}
	if !strings.Contains(stderr.String(), "convex-go-codegen: -out is required") {
		t.Fatalf("expected missing out error on stderr, got %q", stderr.String())
	}
}

func TestRunWithIODefaultsToConvexDirectory(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "convex")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "jobs.js"), []byte(`export default action({});`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	var stdout bytes.Buffer
	if err := runWithIO([]string{"-dry-run"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `var JobsDefault = convex.NewActionReference[map[string]any, any]("jobs:default")`) {
		t.Fatalf("default convex dir was not used:\n%s", stdout.String())
	}
}

func TestMainUsesCLIArgsAndExitStatus(t *testing.T) {
	if os.Getenv("CONVEX_GO_CODEGEN_MAIN_HELPER") == "1" {
		helperMain(t)
		return
	}

	success := exec.Command(os.Args[0], "-test.run=TestMainUsesCLIArgsAndExitStatus")
	success.Env = append(os.Environ(), "CONVEX_GO_CODEGEN_MAIN_HELPER=1", "CONVEX_GO_CODEGEN_MAIN_MODE=success")
	var successStdout bytes.Buffer
	var successStderr bytes.Buffer
	success.Stdout = &successStdout
	success.Stderr = &successStderr
	if err := success.Run(); err != nil {
		t.Fatalf("main should exit successfully on valid dry-run args: %v\nstdout:\n%s\nstderr:\n%s", err, successStdout.String(), successStderr.String())
	}
	if !strings.Contains(successStdout.String(), "JobsDefault") {
		t.Fatalf("main did not pass CLI args through to runWithIO:\n%s", successStdout.String())
	}

	failure := exec.Command(os.Args[0], "-test.run=TestMainUsesCLIArgsAndExitStatus")
	failure.Env = append(os.Environ(), "CONVEX_GO_CODEGEN_MAIN_HELPER=1", "CONVEX_GO_CODEGEN_MAIN_MODE=failure")
	err := failure.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("main should exit 1 on invalid args, got %v", err)
	}
}

func helperMain(t *testing.T) {
	t.Helper()
	switch os.Getenv("CONVEX_GO_CODEGEN_MAIN_MODE") {
	case "success":
		wd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.Chdir(wd); err != nil {
				t.Fatal(err)
			}
		}()
		root := t.TempDir()
		sourceDir := filepath.Join(root, "convex")
		if err := os.MkdirAll(sourceDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sourceDir, "jobs.js"), []byte(`export default action({});`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(root); err != nil {
			t.Fatal(err)
		}
		os.Args = []string{"convex-go-codegen", "-dry-run"}
	case "failure":
		os.Args = []string{"convex-go-codegen", "-src", filepath.Join(t.TempDir(), "missing"), "-dry-run"}
	default:
		t.Fatalf("unknown helper mode %q", os.Getenv("CONVEX_GO_CODEGEN_MAIN_MODE"))
	}
	main()
}

type errWriter struct {
	err error
}

func (w errWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func flagErrHelp() error {
	return flag.ErrHelp
}

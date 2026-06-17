package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if err := runWithIO(args, stdout, stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runWithIO(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		writeUsage(stderr)
		return flag.ErrHelp
	}
	switch args[0] {
	case "fmt-check":
		return runFmtCheck(args[1:], stdout, stderr)
	case "coverage-check":
		return runCoverageCheck(args[1:], stdout, stderr)
	case "export-snapshot":
		return runExportSnapshot(args[1:], stdout, stderr)
	case "integration-env-check":
		return runIntegrationEnvCheck(args[1:], stdout, stderr)
	case "release-check":
		return runReleaseCheck(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		writeUsage(stderr)
		return flag.ErrHelp
	default:
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func runFmtCheck(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("fmt-check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repo := flags.String("repo", ".", "repository root")
	if err := flags.Parse(args); err != nil {
		return err
	}
	files, err := trackedGoFiles(*repo)
	if err != nil {
		return err
	}
	unformatted, err := listUnformattedFiles(*repo, files)
	if err != nil {
		return err
	}
	if len(unformatted) == 0 {
		_, err := fmt.Fprintln(stdout, "gofmt check passed")
		return err
	}
	for _, path := range unformatted {
		if _, err := fmt.Fprintln(stdout, path); err != nil {
			return err
		}
	}
	return fmt.Errorf("gofmt check failed for %d file(s)", len(unformatted))
}

func runCoverageCheck(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("coverage-check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	coverprofile := flags.String("coverprofile", "coverage.out", "path to coverage profile")
	minimum := flags.Float64("min", 90, "minimum total coverage percentage")
	if err := flags.Parse(args); err != nil {
		return err
	}
	total, err := coverageTotal(*coverprofile)
	if err != nil {
		return err
	}
	if total < *minimum {
		return fmt.Errorf("coverage %.1f%% is below %.1f%%", total, *minimum)
	}
	_, err = fmt.Fprintf(stdout, "coverage %.1f%% meets minimum %.1f%%\n", total, *minimum)
	return err
}

func trackedGoFiles(repo string) ([]string, error) {
	return gitTrackedFiles(repo, "*.go")
}

func listUnformattedFiles(repo string, files []string) ([]string, error) {
	if len(files) == 0 {
		return nil, nil
	}
	args := append([]string{"-l"}, files...)
	cmd := exec.Command("gofmt", args...)
	cmd.Dir = repo
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("gofmt -l: %s", strings.TrimSpace(stderr.String()))
		}
		return nil, fmt.Errorf("gofmt -l: %w", err)
	}
	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return nil, nil
	}
	return strings.Fields(output), nil
}

func coverageTotal(path string) (float64, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read coverprofile %s: %w", path, err)
	}
	return parseCoverageProfile(string(body))
}

func parseCoverageProfile(body string) (float64, error) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "mode: ") {
		return 0, errors.New("invalid coverage profile: missing mode line")
	}
	var covered float64
	var total float64
	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return 0, fmt.Errorf("invalid coverage line %q", line)
		}
		numStmts, err := strconv.Atoi(fields[1])
		if err != nil {
			return 0, fmt.Errorf("parse statements in %q: %w", line, err)
		}
		count, err := strconv.Atoi(fields[2])
		if err != nil {
			return 0, fmt.Errorf("parse count in %q: %w", line, err)
		}
		total += float64(numStmts)
		if count > 0 {
			covered += float64(numStmts)
		}
	}
	if total == 0 {
		return 0, errors.New("invalid coverage profile: no statements")
	}
	return covered * 100 / total, nil
}

func writeUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "convex-go-maint provides cross-platform maintainer checks.")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Usage:")
	_, _ = fmt.Fprintln(w, "  go run ./cmd/convex-go-maint fmt-check")
	_, _ = fmt.Fprintln(w, "  go run ./cmd/convex-go-maint coverage-check -coverprofile=coverage.out -min=90")
	_, _ = fmt.Fprintln(w, "  go run ./cmd/convex-go-maint export-snapshot -out ../convex-go-public -git-init")
	_, _ = fmt.Fprintln(w, "  go run ./cmd/convex-go-maint integration-env-check")
	_, _ = fmt.Fprintln(w, "  go run ./cmd/convex-go-maint release-check -version=0.1.0 -notes-out=release-notes.md")
}

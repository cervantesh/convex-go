package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runExportSnapshot(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("export-snapshot", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repo := flags.String("repo", ".", "repository root")
	out := flags.String("out", "", "destination directory for the exported snapshot")
	force := flags.Bool("force", false, "replace the destination directory if it already exists")
	gitInit := flags.Bool("git-init", false, "initialize a fresh git repository in the exported snapshot")
	initialBranch := flags.String("initial-branch", "main", "initial branch name when -git-init is set")
	commitMessage := flags.String("commit-message", "Initial public snapshot", "initial commit message when -git-init is set")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*out) == "" {
		return errors.New("export-snapshot requires -out")
	}

	repoRoot, err := filepath.Abs(*repo)
	if err != nil {
		return fmt.Errorf("resolve repo path: %w", err)
	}
	destRoot, err := filepath.Abs(*out)
	if err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}
	if sameOrNestedPath(destRoot, repoRoot) {
		return fmt.Errorf("destination %s must be outside repository root %s", destRoot, repoRoot)
	}
	if err := prepareExportDestination(destRoot, *force); err != nil {
		return err
	}

	files, err := gitTrackedFiles(repoRoot)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("export-snapshot found no tracked files")
	}
	if err := exportTrackedFiles(repoRoot, destRoot, files); err != nil {
		return err
	}
	if *gitInit {
		if err := initializeSnapshotRepo(destRoot, *initialBranch, *commitMessage); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(stdout, "exported %d tracked file(s) to %s\n", len(files), destRoot); err != nil {
		return err
	}
	if *gitInit {
		_, err := fmt.Fprintln(stdout, "initialized fresh git repository")
		return err
	}
	return nil
}

func prepareExportDestination(destRoot string, force bool) error {
	info, err := os.Stat(destRoot)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return os.MkdirAll(destRoot, 0o755)
	case err != nil:
		return fmt.Errorf("stat destination %s: %w", destRoot, err)
	case !info.IsDir():
		return fmt.Errorf("destination %s is not a directory", destRoot)
	}
	entries, err := os.ReadDir(destRoot)
	if err != nil {
		return fmt.Errorf("read destination %s: %w", destRoot, err)
	}
	if len(entries) == 0 {
		return nil
	}
	if !force {
		return fmt.Errorf("destination %s already exists and is not empty; use -force to replace it", destRoot)
	}
	if err := os.RemoveAll(destRoot); err != nil {
		return fmt.Errorf("clear destination %s: %w", destRoot, err)
	}
	return os.MkdirAll(destRoot, 0o755)
}

func exportTrackedFiles(repoRoot, destRoot string, files []string) error {
	for _, rel := range files {
		source := filepath.Join(repoRoot, filepath.FromSlash(rel))
		target := filepath.Join(destRoot, filepath.FromSlash(rel))
		if err := copyFile(source, target); err != nil {
			return fmt.Errorf("export %s: %w", rel, err)
		}
	}
	return nil
}

func copyFile(source, target string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, data, info.Mode().Perm())
}

func initializeSnapshotRepo(destRoot, initialBranch, commitMessage string) error {
	if err := runGitCommand(destRoot, "init", "-b", initialBranch); err != nil {
		return err
	}
	if err := runGitCommand(destRoot, "add", "."); err != nil {
		return err
	}
	return runGitCommand(
		destRoot,
		"-c", "user.name=convex-go-maint",
		"-c", "user.email=convex-go-maint@local.invalid",
		"commit", "--no-gpg-sign", "-m", commitMessage,
	)
}

func runGitCommand(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func sameOrNestedPath(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

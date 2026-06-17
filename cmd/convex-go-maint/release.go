package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var releaseVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$`)
var versionConstPattern = regexp.MustCompile(`const Version = "([^"]+)"`)

func runReleaseCheck(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("release-check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repo := flags.String("repo", ".", "repository root")
	version := flags.String("version", "", "release version, with or without a leading v")
	versionFile := flags.String("version-file", "client_options.go", "path to the Go version constant file")
	changelog := flags.String("changelog", "CHANGELOG.md", "path to the changelog file")
	notesOut := flags.String("notes-out", "", "optional path for extracted release notes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*version) == "" {
		return errors.New("release-check requires -version")
	}
	normalizedVersion, tagName, err := normalizeReleaseVersion(*version)
	if err != nil {
		return err
	}
	versionValue, err := readVersionConstant(filepath.Join(*repo, *versionFile))
	if err != nil {
		return err
	}
	if versionValue != normalizedVersion {
		return fmt.Errorf("version mismatch: client_options.go declares %q but release version is %q", versionValue, normalizedVersion)
	}
	changelogBody, err := os.ReadFile(filepath.Join(*repo, *changelog))
	if err != nil {
		return fmt.Errorf("read changelog %s: %w", *changelog, err)
	}
	notes, err := extractChangelogNotes(string(changelogBody), normalizedVersion)
	if err != nil {
		return err
	}
	if err := ensureTagDoesNotExist(*repo, tagName); err != nil {
		return err
	}
	if *notesOut != "" {
		if err := os.WriteFile(*notesOut, []byte(notes+"\n"), 0o644); err != nil {
			return fmt.Errorf("write notes file %s: %w", *notesOut, err)
		}
	}
	_, err = fmt.Fprintf(stdout, "validated release %s using %s and %s\n", tagName, *versionFile, *changelog)
	return err
}

func normalizeReleaseVersion(version string) (normalized string, tag string, err error) {
	normalized = strings.TrimSpace(version)
	normalized = strings.TrimPrefix(normalized, "v")
	if !releaseVersionPattern.MatchString(normalized) {
		return "", "", fmt.Errorf("invalid release version %q", version)
	}
	return normalized, "v" + normalized, nil
}

func readVersionConstant(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read version file %s: %w", path, err)
	}
	matches := versionConstPattern.FindStringSubmatch(string(body))
	if len(matches) != 2 {
		return "", fmt.Errorf("could not find const Version in %s", path)
	}
	return matches[1], nil
}

func extractChangelogNotes(body, version string) (string, error) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	headerPrefix := "## " + version
	inSection := false
	var collected []string
	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		if strings.HasPrefix(line, "## ") {
			if inSection {
				break
			}
			rest := strings.TrimPrefix(line, "## ")
			if rest == version || strings.HasPrefix(rest, version+" ") || strings.HasPrefix(rest, version+" -") {
				inSection = true
				continue
			}
		}
		if inSection {
			collected = append(collected, line)
		}
	}
	if !inSection {
		return "", fmt.Errorf("CHANGELOG.md does not contain a section for %s", headerPrefix)
	}
	notes := strings.TrimSpace(strings.Join(collected, "\n"))
	if notes == "" {
		return "", fmt.Errorf("CHANGELOG.md section for %s has no release notes", headerPrefix)
	}
	return notes, nil
}

func ensureTagDoesNotExist(repo, tag string) error {
	cmd := exec.Command("git", "tag", "-l", tag)
	cmd.Dir = repo
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git tag -l %s: %w", tag, err)
	}
	if strings.TrimSpace(string(output)) != "" {
		return fmt.Errorf("tag %s already exists", tag)
	}
	return nil
}

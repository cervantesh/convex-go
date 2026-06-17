package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func gitTrackedFiles(repo string, patterns ...string) ([]string, error) {
	args := []string{"ls-files", "-z", "--"}
	args = append(args, patterns...)
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	return parseNULSeparated(output), nil
}

func parseNULSeparated(body []byte) []string {
	if len(body) == 0 {
		return nil
	}
	parts := strings.Split(string(body), "\x00")
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		paths = append(paths, part)
	}
	return paths
}

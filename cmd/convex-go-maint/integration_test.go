package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunWithIOIntegrationEnvCheckSuccess(t *testing.T) {
	t.Setenv("CONVEX_URL", "https://happy-animal-123.convex.cloud")
	t.Setenv("CONVEX_AUTH_TOKEN", "token")

	var stdout bytes.Buffer
	if err := runWithIO([]string{"integration-env-check"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "validated CONVEX_URL") {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
}

func TestRunWithIOIntegrationEnvCheckRejectsMissingURL(t *testing.T) {
	t.Setenv("CONVEX_URL", "")

	err := runWithIO([]string{"integration-env-check"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "missing required environment variable CONVEX_URL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithIOIntegrationEnvCheckRejectsInvalidURL(t *testing.T) {
	t.Setenv("CONVEX_URL", "://bad-url")

	err := runWithIO([]string{"integration-env-check"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "CONVEX_URL is not a valid URL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithIOIntegrationEnvCheckRejectsNonHTTPURL(t *testing.T) {
	t.Setenv("CONVEX_URL", "ftp://example.com")

	err := runWithIO([]string{"integration-env-check"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "CONVEX_URL must start with http:// or https://") {
		t.Fatalf("unexpected error: %v", err)
	}
}

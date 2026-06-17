package main

import (
	"bytes"
	"errors"
	"flag"
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

func TestRunWithIOIntegrationEnvCheckRejectsMissingHost(t *testing.T) {
	t.Setenv("CONVEX_URL", "https:///missing-host")

	err := runWithIO([]string{"integration-env-check"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "CONVEX_URL must include a host") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithIOIntegrationEnvCheckSupportsCustomEnvNames(t *testing.T) {
	t.Setenv("ALT_CONVEX_URL", "https://happy-animal-123.convex.cloud")

	var stdout bytes.Buffer
	if err := runWithIO([]string{
		"integration-env-check",
		"-url-env", "ALT_CONVEX_URL",
		"-auth-env", "ALT_CONVEX_TOKEN",
	}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "validated ALT_CONVEX_URL") {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "ALT_CONVEX_TOKEN") {
		t.Fatalf("did not expect optional auth env in stdout: %q", stdout.String())
	}
}

func TestRunWithIOIntegrationEnvCheckCustomAuthEnvIsReportedWhenPresent(t *testing.T) {
	t.Setenv("ALT_CONVEX_URL", "https://happy-animal-123.convex.cloud")
	t.Setenv("ALT_CONVEX_TOKEN", "token")

	var stdout bytes.Buffer
	if err := runWithIO([]string{
		"integration-env-check",
		"-url-env", "ALT_CONVEX_URL",
		"-auth-env", "ALT_CONVEX_TOKEN",
	}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "ALT_CONVEX_TOKEN") {
		t.Fatalf("expected optional auth env in stdout: %q", stdout.String())
	}
}

func TestRunWithIOIntegrationEnvCheckHelp(t *testing.T) {
	var stderr bytes.Buffer
	err := runWithIO([]string{"integration-env-check", "-h"}, &bytes.Buffer{}, &stderr)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
	if !strings.Contains(stderr.String(), "url-env") {
		t.Fatalf("unexpected help output: %q", stderr.String())
	}
}

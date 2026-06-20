package main

import (
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
)

func runIntegrationEnvCheck(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("integration-env-check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	urlEnv := flags.String("url-env", "CONVEX_URL", "environment variable containing the live Convex deployment URL")
	authEnv := flags.String("auth-env", "CONVEX_AUTH_TOKEN", "optional environment variable containing a bearer auth token")
	refreshAuthEnv := flags.String("refresh-auth-env", "CONVEX_AUTH_REFRESH_TOKEN", "optional environment variable containing the reconnect refresh auth token")
	expectedSubjectEnv := flags.String("expected-subject-env", "CONVEX_AUTH_EXPECTED_SUBJECT", "optional environment variable containing the expected authenticated subject")
	expectedIssuerEnv := flags.String("expected-issuer-env", "CONVEX_AUTH_EXPECTED_ISSUER", "optional environment variable containing the expected authenticated issuer")
	expectedTokenIdentifierEnv := flags.String("expected-token-identifier-env", "CONVEX_AUTH_EXPECTED_TOKEN_IDENTIFIER", "optional environment variable containing the expected authenticated token identifier")
	if err := flags.Parse(args); err != nil {
		return err
	}

	deploymentURL := strings.TrimSpace(os.Getenv(*urlEnv))
	if deploymentURL == "" {
		return fmt.Errorf("missing required environment variable %s", *urlEnv)
	}
	parsed, err := url.Parse(deploymentURL)
	if err != nil {
		return fmt.Errorf("%s is not a valid URL: %w", *urlEnv, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s must start with http:// or https://", *urlEnv)
	}
	if parsed.Host == "" {
		return fmt.Errorf("%s must include a host", *urlEnv)
	}

	authToken := strings.TrimSpace(os.Getenv(*authEnv))
	refreshAuthToken := strings.TrimSpace(os.Getenv(*refreshAuthEnv))
	expectedSubject := strings.TrimSpace(os.Getenv(*expectedSubjectEnv))
	expectedIssuer := strings.TrimSpace(os.Getenv(*expectedIssuerEnv))
	expectedTokenIdentifier := strings.TrimSpace(os.Getenv(*expectedTokenIdentifierEnv))

	if refreshAuthToken != "" && authToken == "" {
		return fmt.Errorf("%s requires %s", *refreshAuthEnv, *authEnv)
	}
	if authToken == "" && (expectedSubject != "" || expectedIssuer != "" || expectedTokenIdentifier != "") {
		return fmt.Errorf("auth identity expectations require %s", *authEnv)
	}

	if _, err := fmt.Fprintf(stdout, "validated %s", *urlEnv); err != nil {
		return err
	}
	if authToken != "" {
		if _, err := fmt.Fprintf(stdout, " with optional %s", *authEnv); err != nil {
			return err
		}
	}
	if refreshAuthToken != "" {
		if _, err := fmt.Fprintf(stdout, " and optional %s", *refreshAuthEnv); err != nil {
			return err
		}
	}
	if expectedSubject != "" || expectedIssuer != "" || expectedTokenIdentifier != "" {
		if _, err := fmt.Fprint(stdout, " plus auth identity expectations"); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(stdout)
	return err
}

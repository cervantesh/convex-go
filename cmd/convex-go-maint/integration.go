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

	if _, err := fmt.Fprintf(stdout, "validated %s", *urlEnv); err != nil {
		return err
	}
	if strings.TrimSpace(os.Getenv(*authEnv)) != "" {
		if _, err := fmt.Fprintf(stdout, " with optional %s", *authEnv); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(stdout)
	return err
}

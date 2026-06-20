package convex

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

var errLiveIntegrationURLNotSet = errors.New("CONVEX_URL not set")

type liveIntegrationConfig struct {
	deploymentURL           string
	authToken               string
	refreshAuthToken        string
	expectedSubject         string
	expectedIssuer          string
	expectedTokenIdentifier string
}

func loadLiveIntegrationConfigFromEnv() (liveIntegrationConfig, error) {
	deploymentURL := strings.TrimSpace(os.Getenv("CONVEX_URL"))
	if deploymentURL == "" {
		return liveIntegrationConfig{}, errLiveIntegrationURLNotSet
	}

	authToken := strings.TrimSpace(os.Getenv("CONVEX_AUTH_TOKEN"))
	refreshAuthToken := strings.TrimSpace(os.Getenv("CONVEX_AUTH_REFRESH_TOKEN"))
	if refreshAuthToken == "" {
		refreshAuthToken = authToken
	}

	cfg := liveIntegrationConfig{
		deploymentURL:           deploymentURL,
		authToken:               authToken,
		refreshAuthToken:        refreshAuthToken,
		expectedSubject:         strings.TrimSpace(os.Getenv("CONVEX_AUTH_EXPECTED_SUBJECT")),
		expectedIssuer:          strings.TrimSpace(os.Getenv("CONVEX_AUTH_EXPECTED_ISSUER")),
		expectedTokenIdentifier: strings.TrimSpace(os.Getenv("CONVEX_AUTH_EXPECTED_TOKEN_IDENTIFIER")),
	}

	if cfg.authToken == "" && strings.TrimSpace(os.Getenv("CONVEX_AUTH_REFRESH_TOKEN")) != "" {
		return liveIntegrationConfig{}, fmt.Errorf("CONVEX_AUTH_REFRESH_TOKEN requires CONVEX_AUTH_TOKEN")
	}
	if cfg.authToken == "" && (cfg.expectedSubject != "" || cfg.expectedIssuer != "" || cfg.expectedTokenIdentifier != "") {
		return liveIntegrationConfig{}, fmt.Errorf("auth identity expectations require CONVEX_AUTH_TOKEN")
	}

	return cfg, nil
}

func loadLiveIntegrationConfig(t *testing.T) liveIntegrationConfig {
	t.Helper()
	cfg, err := loadLiveIntegrationConfigFromEnv()
	if errors.Is(err, errLiveIntegrationURLNotSet) {
		t.Skip(err.Error())
	}
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestLoadLiveIntegrationConfigFromEnvRejectsRefreshTokenWithoutAuth(t *testing.T) {
	t.Setenv("CONVEX_URL", "https://happy-animal-123.convex.cloud")
	t.Setenv("CONVEX_AUTH_TOKEN", "")
	t.Setenv("CONVEX_AUTH_REFRESH_TOKEN", "refresh-token")

	_, err := loadLiveIntegrationConfigFromEnv()
	if err == nil || !strings.Contains(err.Error(), "CONVEX_AUTH_REFRESH_TOKEN requires CONVEX_AUTH_TOKEN") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadLiveIntegrationConfigFromEnvRejectsIdentityExpectationsWithoutAuth(t *testing.T) {
	t.Setenv("CONVEX_URL", "https://happy-animal-123.convex.cloud")
	t.Setenv("CONVEX_AUTH_EXPECTED_SUBJECT", "subject-123")

	_, err := loadLiveIntegrationConfigFromEnv()
	if err == nil || !strings.Contains(err.Error(), "auth identity expectations require CONVEX_AUTH_TOKEN") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadLiveIntegrationConfigFromEnvDefaultsRefreshTokenToPrimaryAuth(t *testing.T) {
	t.Setenv("CONVEX_URL", "https://happy-animal-123.convex.cloud")
	t.Setenv("CONVEX_AUTH_TOKEN", "primary-token")

	cfg, err := loadLiveIntegrationConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.refreshAuthToken != "primary-token" {
		t.Fatalf("refresh auth token = %q, want primary token", cfg.refreshAuthToken)
	}
}

func TestLoadLiveIntegrationConfigFromEnvPreservesExplicitIdentityExpectations(t *testing.T) {
	t.Setenv("CONVEX_URL", "https://happy-animal-123.convex.cloud")
	t.Setenv("CONVEX_AUTH_TOKEN", "primary-token")
	t.Setenv("CONVEX_AUTH_REFRESH_TOKEN", "refresh-token")
	t.Setenv("CONVEX_AUTH_EXPECTED_SUBJECT", "subject-123")
	t.Setenv("CONVEX_AUTH_EXPECTED_ISSUER", "issuer-123")
	t.Setenv("CONVEX_AUTH_EXPECTED_TOKEN_IDENTIFIER", "issuer-123|subject-123")

	cfg, err := loadLiveIntegrationConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.authToken != "primary-token" || cfg.refreshAuthToken != "refresh-token" {
		t.Fatalf("unexpected auth config: %#v", cfg)
	}
	if cfg.expectedSubject != "subject-123" || cfg.expectedIssuer != "issuer-123" || cfg.expectedTokenIdentifier != "issuer-123|subject-123" {
		t.Fatalf("unexpected identity expectations: %#v", cfg)
	}
}

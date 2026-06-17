package syncprotocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSyncProtocolConformanceUserIdentityAttributesFromConvexRS(t *testing.T) {
	// Sources:
	// - convex-rs/sync_types/src/types/json.rs:
	//   user_identity_attributes_deserialize_token_identifier_given
	//   user_identity_attributes_deserialize_token_identifier_deriver
	//   user_identity_attributes_deserialize_token_identifier_cannot_derive
	t.Run("preserves explicit token identifier", func(t *testing.T) {
		var identity SyncUserIdentityAttributes
		if err := json.Unmarshal([]byte(`{"tokenIdentifier":"explicit","issuer":"issuer","subject":"subject","numeric":1}`), &identity); err != nil {
			t.Fatal(err)
		}
		if identity.TokenIdentifier != "explicit" || identity.Issuer != "issuer" || identity.Subject != "subject" {
			t.Fatalf("unexpected identity claims: %#v", identity)
		}
		if value, ok := claimString(identity.Claims, "numeric"); ok || value != "" {
			t.Fatalf("non-string claim should report absent, got %q %v", value, ok)
		}
	})

	t.Run("derives token identifier from issuer and subject", func(t *testing.T) {
		var identity SyncUserIdentityAttributes
		if err := json.Unmarshal([]byte(`{"issuer":"fake_issuer","subject":"fake_subject"}`), &identity); err != nil {
			t.Fatal(err)
		}
		if identity.TokenIdentifier != "fake_issuer|fake_subject" {
			t.Fatalf("expected derived token identifier, got %#v", identity)
		}
	})

	t.Run("rejects incomplete identity attributes", func(t *testing.T) {
		for _, raw := range []string{
			`{"issuer":"fake_issuer"}`,
			`{"subject":"fake_subject"}`,
			`{"numeric":1}`,
			`{}`,
		} {
			t.Run(raw, func(t *testing.T) {
				var identity SyncUserIdentityAttributes
				err := json.Unmarshal([]byte(raw), &identity)
				if err == nil {
					t.Fatalf("expected %s to be rejected", raw)
				}
				if !strings.Contains(err.Error(), `Either "tokenIdentifier" or "issuer" and "subject" must be set`) {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		}
	})
}

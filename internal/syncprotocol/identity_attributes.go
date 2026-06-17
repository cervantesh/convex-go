package syncprotocol

import (
	"encoding/json"
	"fmt"
)

// SyncUserIdentityAttributes is the sync-protocol identity shape used by admin
// authentication. Extra claims are preserved in Claims.
type SyncUserIdentityAttributes struct {
	TokenIdentifier string
	Issuer          string
	Subject         string
	Claims          map[string]any
}

func (i SyncUserIdentityAttributes) MarshalJSON() ([]byte, error) {
	out := make(map[string]any, len(i.Claims)+3)
	for k, v := range i.Claims {
		out[k] = v
	}
	if i.Issuer != "" {
		out["issuer"] = i.Issuer
	}
	if i.Subject != "" {
		out["subject"] = i.Subject
	}
	tokenIdentifier := i.TokenIdentifier
	if tokenIdentifier == "" && i.Issuer != "" && i.Subject != "" {
		tokenIdentifier = i.Issuer + "|" + i.Subject
	}
	if tokenIdentifier != "" {
		out["tokenIdentifier"] = tokenIdentifier
	}
	return json.Marshal(out)
}

func (i *SyncUserIdentityAttributes) UnmarshalJSON(data []byte) error {
	claims, err := decodeRawMap(data)
	if err != nil {
		return err
	}
	i.Claims = claims
	i.TokenIdentifier, _ = claimString(claims, "tokenIdentifier")
	i.Issuer, _ = claimString(claims, "issuer")
	i.Subject, _ = claimString(claims, "subject")
	if i.TokenIdentifier == "" {
		if i.Issuer != "" && i.Subject != "" {
			i.TokenIdentifier = i.Issuer + "|" + i.Subject
		} else {
			return fmt.Errorf(`convex: Either "tokenIdentifier" or "issuer" and "subject" must be set`)
		}
	}
	return nil
}

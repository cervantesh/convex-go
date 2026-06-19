package convex

import (
	"errors"
	"testing"

	"github.com/cervantesh/convex-go/baseclient"
)

func TestAdaptUserTokenFetcherHandlesNilErrorEmptyAndToken(t *testing.T) {
	if adaptUserTokenFetcher(nil) != nil {
		t.Fatal("nil user token fetcher must stay nil")
	}

	wantErr := errors.New("fetch failed")
	errFetcher := adaptUserTokenFetcher(func(forceRefresh bool) (string, error) {
		if !forceRefresh {
			t.Fatal("expected forwarded forceRefresh flag")
		}
		return "", wantErr
	})
	if _, err := errFetcher(true); !errors.Is(err, wantErr) {
		t.Fatalf("error fetcher error = %v, want %v", err, wantErr)
	}

	noAuthFetcher := adaptUserTokenFetcher(func(forceRefresh bool) (string, error) {
		return "", nil
	})
	noAuthToken, err := noAuthFetcher(false)
	if err != nil {
		t.Fatal(err)
	}
	if noAuthToken != baseclient.NoAuthToken() {
		t.Fatalf("empty token result = %#v, want %#v", noAuthToken, baseclient.NoAuthToken())
	}

	userTokenFetcher := adaptUserTokenFetcher(func(forceRefresh bool) (string, error) {
		return "jwt-123", nil
	})
	userToken, err := userTokenFetcher(false)
	if err != nil {
		t.Fatal(err)
	}
	if userToken != baseclient.UserAuthToken("jwt-123") {
		t.Fatalf("user token result = %#v, want %#v", userToken, baseclient.UserAuthToken("jwt-123"))
	}
}

package convex

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/cervantesh/convex-go/baseclient"
)

func TestClientAuthCallbackRetriesUnauthorizedOnceWithForceRefresh(t *testing.T) {
	var authHeaders []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeaders = append(authHeaders, r.Header.Get("Authorization"))
		if len(authHeaders) == 1 {
			http.Error(w, "expired", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","value":"ok"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	var calls []bool
	err = client.SetAuthCallback(func(forceRefresh bool) (string, error) {
		calls = append(calls, forceRefresh)
		if forceRefresh {
			return "fresh-token", nil
		}
		return "stale-token", nil
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := client.Query(context.Background(), "messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "ok" {
		t.Fatalf("unexpected query result: %#v", got)
	}
	if !reflect.DeepEqual(calls, []bool{false, false, true}) {
		t.Fatalf("unexpected auth callback calls: %#v", calls)
	}
	if !reflect.DeepEqual(authHeaders, []string{"Bearer stale-token", "Bearer fresh-token"}) {
		t.Fatalf("unexpected auth headers: %#v", authHeaders)
	}
}

func TestClientAuthCallbackErrorDoesNotSendHTTPRequest(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		t.Fatalf("unexpected HTTP request with auth callback error: %s", r.URL.Path)
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("token unavailable")
	err = client.SetAuthCallback(func(forceRefresh bool) (string, error) {
		return "", wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected set callback error, got %v", err)
	}
	if requests != 0 {
		t.Fatalf("unexpected HTTP requests during callback install: %d", requests)
	}
}

func TestClientSetAuthCallbackFailurePreservesPreviousHTTPAuth(t *testing.T) {
	var authHeaders []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeaders = append(authHeaders, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","value":"ok"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, WithAuth("user-token"))
	if err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("bad fetcher")
	err = client.SetAuthCallback(func(forceRefresh bool) (string, error) {
		return "", wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected callback error, got %v", err)
	}
	if _, err := client.Query(context.Background(), "messages:list", nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(authHeaders, []string{"Bearer user-token"}) {
		t.Fatalf("expected previous auth to remain active, got %#v", authHeaders)
	}
}

func TestClientSetAuthCallbackNilClearsHTTPAndRealtimeAuth(t *testing.T) {
	httpAuth := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpAuth <- r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","value":"ok"}`))
	}))
	defer server.Close()

	dialer := newFakeSyncDialer()
	client, err := NewClient(server.URL,
		WithSkipDeploymentURLCheck(),
		WithAuth("user-token"),
		withWebSocketDialer(dialer),
		WithWebSocketReconnectBackoff(0),
		WithWebSocketInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	subscription, err := client.Subscribe(ctx, "messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = subscription.Close() }()
	attempt := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
	auth := decodeSentClientMessage[AuthenticateMessage](t, attempt.conn.waitSent(t))
	if auth.TokenType != "User" || auth.Value != "user-token" {
		t.Fatalf("unexpected initial auth: %#v", auth)
	}
	_ = decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t))

	if err := client.SetAuthCallback(nil); err != nil {
		t.Fatal(err)
	}
	none := decodeSentClientMessage[AuthenticateMessage](t, attempt.conn.waitSent(t))
	if none.TokenType != "None" || none.Value != "" {
		t.Fatalf("unexpected clear auth message: %#v", none)
	}

	if _, err := client.Query(context.Background(), "messages:list", nil); err != nil {
		t.Fatal(err)
	}
	if got := <-httpAuth; got != "" {
		t.Fatalf("expected cleared HTTP auth header, got %q", got)
	}
}

func TestClientAuthCallbackAppliesToLazyRealtime(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, err := NewClient("https://happy-animal-123.convex.cloud",
		WithSkipDeploymentURLCheck(),
		withWebSocketDialer(dialer),
		WithWebSocketReconnectBackoff(0),
		WithWebSocketInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	var calls []bool
	if err := client.SetAuthCallback(func(forceRefresh bool) (string, error) {
		calls = append(calls, forceRefresh)
		return "lazy-token", nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	subscription, err := client.Subscribe(ctx, "messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = subscription.Close() }()
	attempt := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
	auth := decodeSentClientMessage[AuthenticateMessage](t, attempt.conn.waitSent(t))
	if auth.TokenType != "User" || auth.Value != "lazy-token" {
		t.Fatalf("unexpected realtime auth from callback: %#v", auth)
	}
	if !reflect.DeepEqual(calls, []bool{false, false}) {
		t.Fatalf("unexpected lazy callback calls: %#v", calls)
	}
}

func TestWebSocketClientSetAuthCallbackFlushesAuthenticateMessage(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)

	var calls []bool
	if err := client.SetAuthCallback(func(forceRefresh bool) (string, error) {
		calls = append(calls, forceRefresh)
		return "socket-token", nil
	}); err != nil {
		t.Fatal(err)
	}
	auth := decodeSentClientMessage[AuthenticateMessage](t, attempt.conn.waitSent(t))
	if auth.TokenType != "User" || auth.Value != "socket-token" {
		t.Fatalf("unexpected websocket auth callback message: %#v", auth)
	}
	if !reflect.DeepEqual(calls, []bool{false}) {
		t.Fatalf("unexpected websocket callback calls: %#v", calls)
	}
}

func TestAdaptUserTokenFetcherNilAndEmptyToken(t *testing.T) {
	if adaptUserTokenFetcher(nil) != nil {
		t.Fatal("expected nil adapter for nil fetcher")
	}

	adapted := adaptUserTokenFetcher(func(forceRefresh bool) (string, error) {
		if forceRefresh {
			t.Fatal("unexpected force refresh")
		}
		return "", nil
	})
	token, err := adapted(false)
	if err != nil {
		t.Fatal(err)
	}
	if token != baseclient.NoAuthToken() {
		t.Fatalf("unexpected adapted token: %#v", token)
	}
}

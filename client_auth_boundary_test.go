package convex

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestClientSetAuthContextFlushErrorStillUpdatesHTTPState(t *testing.T) {
	server, headers := newAuthHeaderServer()
	defer server.Close()

	client, attempt := newStartedAuthBoundaryClient(t, server.URL)
	defer closeTestClient(t, client)

	blocked, _ := attempt.conn.blockNextWrite()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := client.SetAuthContext(ctx, "rotated-user-token")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded auth refresh, got %v", err)
	}
	waitBlockedWrite(t, blocked)

	if _, err := client.Query(context.Background(), "messages:list", nil); err != nil {
		t.Fatal(err)
	}
	if got := headers.last(t); got != "Bearer rotated-user-token" {
		t.Fatalf("unexpected auth header after flush error: %q", got)
	}
}

func TestClientSetAdminAuthContextFlushErrorStillUpdatesHTTPState(t *testing.T) {
	server, headers := newAuthHeaderServer()
	defer server.Close()

	client, attempt := newStartedAuthBoundaryClient(t, server.URL)
	defer closeTestClient(t, client)

	blocked, _ := attempt.conn.blockNextWrite()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := client.SetAdminAuthContext(ctx, "admin-token", UserIdentityAttributes{
		"issuer":  "issuer",
		"subject": "subject",
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded admin auth refresh, got %v", err)
	}
	waitBlockedWrite(t, blocked)

	if _, err := client.Query(context.Background(), "messages:list", nil); err != nil {
		t.Fatal(err)
	}
	if got := headers.last(t); !strings.HasPrefix(got, "Convex admin-token:") {
		t.Fatalf("unexpected admin auth header after flush error: %q", got)
	}
}

func TestClientClearAuthContextFlushErrorStillUpdatesHTTPState(t *testing.T) {
	server, headers := newAuthHeaderServer()
	defer server.Close()

	client, attempt := newStartedAuthBoundaryClient(t, server.URL, WithAuth("stale-user-token"))
	defer closeTestClient(t, client)

	blocked, _ := attempt.conn.blockNextWrite()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := client.ClearAuthContext(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded auth clear, got %v", err)
	}
	waitBlockedWrite(t, blocked)

	if _, err := client.Query(context.Background(), "messages:list", nil); err != nil {
		t.Fatal(err)
	}
	if got := headers.last(t); got != "" {
		t.Fatalf("expected cleared auth header after flush error, got %q", got)
	}
}

func newStartedAuthBoundaryClient(t *testing.T, url string, opts ...Option) (*Client, fakeDialAttempt) {
	t.Helper()

	dialer := newFakeSyncDialer()
	clientOpts := []Option{
		WithSkipDeploymentURLCheck(),
		withWebSocketDialer(dialer),
		WithWebSocketReconnectBackoff(0),
		WithWebSocketInactivityTimeout(time.Hour),
	}
	clientOpts = append(clientOpts, opts...)

	client, err := NewClient(url, clientOpts...)
	if err != nil {
		t.Fatal(err)
	}
	subscription, err := client.Subscribe(context.Background(), "messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = subscription.Close() })

	attempt := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
	if len(opts) > 0 {
		_ = decodeSentClientMessage[AuthenticateMessage](t, attempt.conn.waitSent(t))
	}
	_ = decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t))
	return client, attempt
}

func closeTestClient(t *testing.T, client *Client) {
	t.Helper()
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}

func waitBlockedWrite(t *testing.T, blocked <-chan struct{}) {
	t.Helper()
	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for blocked realtime flush")
	}
}

type authHeaders struct {
	mu     sync.Mutex
	values []string
}

func (h *authHeaders) add(value string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.values = append(h.values, value)
}

func (h *authHeaders) last(t *testing.T) string {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.values) == 0 {
		t.Fatal("expected at least one HTTP request")
	}
	return h.values[len(h.values)-1]
}

func newAuthHeaderServer() (*httptest.Server, *authHeaders) {
	headers := &authHeaders{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers.add(r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","value":"ok","logLines":[]}`))
	}))
	return server, headers
}

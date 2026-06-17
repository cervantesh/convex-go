package convex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClientQueryMutationActionUseHTTP(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","value":"ok","logLines":[]}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, WithSkipDeploymentURLCheck())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if value, err := client.Query(ctx, "messages:list", nil); err != nil || value != "ok" {
		t.Fatalf("unexpected query result: %#v err=%v", value, err)
	}
	if value, err := client.Mutation(ctx, "messages:send", nil); err != nil || value != "ok" {
		t.Fatalf("unexpected mutation result: %#v err=%v", value, err)
	}
	if value, err := client.Action(ctx, "jobs:run", nil); err != nil || value != "ok" {
		t.Fatalf("unexpected action result: %#v err=%v", value, err)
	}
	want := []string{"/api/query", "/api/mutation", "/api/action"}
	if len(paths) != len(want) {
		t.Fatalf("unexpected HTTP calls: %#v", paths)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("unexpected HTTP path %d: got %q want %q", i, paths[i], want[i])
		}
	}
}

func TestNewClientValueMethodsUseHTTP(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","value":{"ok":true},"logLines":[]}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, WithSkipDeploymentURLCheck())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	calls := []struct {
		name string
		call func() (Value, error)
	}{
		{name: "query", call: func() (Value, error) { return client.QueryValue(ctx, "messages:list", nil) }},
		{name: "mutation", call: func() (Value, error) { return client.MutationValue(ctx, "messages:send", nil) }},
		{name: "action", call: func() (Value, error) { return client.ActionValue(ctx, "jobs:run", nil) }},
	}
	for _, tt := range calls {
		value, err := tt.call()
		if err != nil {
			t.Fatalf("%s failed: %v", tt.name, err)
		}
		if got := value.GoValue().(map[string]any)["ok"]; got != true {
			t.Fatalf("unexpected %s value: %#v", tt.name, value.GoValue())
		}
	}
	want := []string{"/api/query", "/api/mutation", "/api/action"}
	if len(paths) != len(want) {
		t.Fatalf("unexpected HTTP calls: %#v", paths)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("unexpected HTTP path %d: got %q want %q", i, paths[i], want[i])
		}
	}
}

func TestNewClientSubscribeInitializesRealtimeLazily(t *testing.T) {
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
	select {
	case attempt := <-dialer.attempts:
		t.Fatalf("unexpected realtime dial before Subscribe: %#v", attempt)
	default:
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
	_ = decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t))
}

func TestNewClientSubscribeReusesRealtime(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	first, err := client.Subscribe(ctx, "messages:first", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = first.Close() }()
	attempt := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
	_ = decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t))

	second, err := client.Subscribe(ctx, "messages:second", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = second.Close() }()
	_ = decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t))
	select {
	case attempt := <-dialer.attempts:
		t.Fatalf("unexpected second realtime dial: %#v", attempt)
	default:
	}
}

func TestNewClientSubscribeReturnsRealtimeInitError(t *testing.T) {
	client, err := NewClient("https://happy-animal-123.convex.cloud",
		WithSkipDeploymentURLCheck(),
		withWebSocketDialer(nil),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Subscribe(context.Background(), "messages:list", nil); err == nil || !strings.Contains(err.Error(), "nil websocket dialer") {
		t.Fatalf("expected realtime init error, got %v", err)
	}
	if _, err := client.WatchAll(context.Background()); err == nil || !strings.Contains(err.Error(), "nil websocket dialer") {
		t.Fatalf("expected watch realtime init error, got %v", err)
	}
}

func TestNewClientWatchAllInitializesRealtimeLazily(t *testing.T) {
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
	select {
	case attempt := <-dialer.attempts:
		t.Fatalf("unexpected realtime dial before WatchAll: %#v", attempt)
	default:
	}

	watcher, err := client.WatchAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = watcher.Close() }()
	attempt := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
}

func TestNewClientCloseIsIdempotent(t *testing.T) {
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
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewClientAuthAppliesToHTTPAndRealtime(t *testing.T) {
	httpAuth := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpAuth <- r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","value":"ok","logLines":[]}`))
	}))
	defer server.Close()

	dialer := newFakeSyncDialer()
	client, err := NewClient(server.URL,
		WithSkipDeploymentURLCheck(),
		withWebSocketDialer(dialer),
		WithWebSocketReconnectBackoff(0),
		WithWebSocketInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	client.SetAuth("user-token")
	if _, err := client.Query(context.Background(), "messages:list", nil); err != nil {
		t.Fatal(err)
	}
	if got := <-httpAuth; got != "Bearer user-token" {
		t.Fatalf("unexpected HTTP auth header: %q", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := client.Subscribe(ctx, "messages:list", nil); err != nil {
		t.Fatal(err)
	}
	attempt := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
	auth := decodeSentClientMessage[AuthenticateMessage](t, attempt.conn.waitSent(t))
	if auth.Value != "user-token" {
		t.Fatalf("unexpected realtime auth: %#v", auth)
	}
}

func TestNewClientAuthOptionsApplyToHTTPAndRealtime(t *testing.T) {
	httpAuth := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpAuth <- r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","value":"ok","logLines":[]}`))
	}))
	defer server.Close()

	dialer := newFakeSyncDialer()
	client, err := NewClient(server.URL,
		WithSkipDeploymentURLCheck(),
		WithAuth("option-token"),
		withWebSocketDialer(dialer),
		WithWebSocketReconnectBackoff(0),
		WithWebSocketInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	if _, err := client.Query(context.Background(), "messages:list", nil); err != nil {
		t.Fatal(err)
	}
	if got := <-httpAuth; got != "Bearer option-token" {
		t.Fatalf("unexpected HTTP auth header: %q", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := client.Subscribe(ctx, "messages:list", nil); err != nil {
		t.Fatal(err)
	}
	attempt := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
	auth := decodeSentClientMessage[AuthenticateMessage](t, attempt.conn.waitSent(t))
	if auth.TokenType != "User" || auth.Value != "option-token" {
		t.Fatalf("unexpected realtime auth: %#v", auth)
	}
}

func TestNewClientAdminAuthOptionAppliesToHTTPAndRealtime(t *testing.T) {
	httpAuth := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpAuth <- r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","value":"ok","logLines":[]}`))
	}))
	defer server.Close()

	dialer := newFakeSyncDialer()
	client, err := NewClient(server.URL,
		WithSkipDeploymentURLCheck(),
		WithAdminAuth("admin-token", UserIdentityAttributes{
			"issuer":  "issuer",
			"subject": "subject",
		}),
		withWebSocketDialer(dialer),
		WithWebSocketReconnectBackoff(0),
		WithWebSocketInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	if _, err := client.Query(context.Background(), "messages:list", nil); err != nil {
		t.Fatal(err)
	}
	if got := <-httpAuth; !strings.HasPrefix(got, "Convex admin-token:") {
		t.Fatalf("unexpected HTTP auth header: %q", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := client.Subscribe(ctx, "messages:list", nil); err != nil {
		t.Fatal(err)
	}
	attempt := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
	auth := decodeSentClientMessage[AuthenticateMessage](t, attempt.conn.waitSent(t))
	if auth.TokenType != "Admin" || auth.Value != "admin-token" || auth.ActingAs == nil {
		t.Fatalf("unexpected realtime admin auth: %#v", auth)
	}
}

func TestNewClientAuthMutatorsApplyToStartedRealtime(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	subscription, err := client.Subscribe(ctx, "messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = subscription.Close() }()
	attempt := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
	_ = decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t))

	client.SetAuth("user-token")
	user := decodeSentClientMessage[AuthenticateMessage](t, attempt.conn.waitSent(t))
	if user.TokenType != "User" || user.Value != "user-token" {
		t.Fatalf("unexpected user auth: %#v", user)
	}

	if err := client.SetAdminAuth("admin-token", UserIdentityAttributes{
		"issuer":  "issuer",
		"subject": "subject",
	}); err != nil {
		t.Fatal(err)
	}
	admin := decodeSentClientMessage[AuthenticateMessage](t, attempt.conn.waitSent(t))
	if admin.TokenType != "Admin" || admin.Value != "admin-token" || admin.ActingAs == nil {
		t.Fatalf("unexpected admin auth: %#v", admin)
	}
	if admin.ActingAs.TokenIdentifier != "issuer|subject" {
		t.Fatalf("unexpected admin acting-as identity: %#v", admin.ActingAs)
	}

	client.ClearAuth()
	none := decodeSentClientMessage[AuthenticateMessage](t, attempt.conn.waitSent(t))
	if none.TokenType != "None" || none.Value != "" {
		t.Fatalf("unexpected clear auth: %#v", none)
	}
}

package convex

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRootMutationHardeningClientOptionErrorsAreExact(t *testing.T) {
	if _, err := NewHTTPClient("https://happy-animal-123.convex.cloud", WithHTTPClient(nil)); err == nil || err.Error() != "convex: nil http client" {
		t.Fatalf("unexpected nil http client error: %v", err)
	}
	if _, err := NewHTTPClient("https://happy-animal-123.convex.cloud", WithClientID(" ")); err == nil || err.Error() != "convex: client ID cannot be empty" {
		t.Fatalf("unexpected empty client id error: %v", err)
	}
	if client, err := NewClient("", WithSkipDeploymentURLCheck()); err == nil || client != nil {
		t.Fatalf("invalid NewClient should return nil client and error, client=%#v err=%v", client, err)
	}
	if _, err := NewClient("https://happy-animal-123.convex.cloud", WithAdminAuth("admin", UserIdentityAttributes{}, UserIdentityAttributes{})); err == nil || err.Error() != "convex: expected at most one acting-as identity" {
		t.Fatalf("unexpected acting-as option error: %v", err)
	}
}

func TestRootMutationHardeningNewHTTPClientDefaults(t *testing.T) {
	client, err := NewHTTPClient("https://happy-animal-123.convex.cloud/api/?q=1#frag")
	if err != nil {
		t.Fatal(err)
	}
	if client.DeploymentURL() != "https://happy-animal-123.convex.cloud/api" {
		t.Fatalf("unexpected normalized deployment URL: %q", client.DeploymentURL())
	}
	if client.clientID != "go-0.1.0" {
		t.Fatalf("unexpected default client id: %q", client.clientID)
	}
	if cap(client.mutationQueue) != 1 {
		t.Fatalf("mutation queue capacity = %d", cap(client.mutationQueue))
	}
	select {
	case <-client.mutationQueue:
	default:
		t.Fatal("mutation queue should start ready")
	}
	select {
	case client.mutationQueue <- struct{}{}:
	default:
		t.Fatal("mutation queue should accept the returned turn")
	}

	queue := newReadyMutationQueue()
	if cap(queue) != 1 {
		t.Fatalf("ready queue capacity = %d", cap(queue))
	}
	select {
	case <-queue:
	default:
		t.Fatal("ready queue should contain one turn")
	}
}

func TestRootMutationHardeningNormalizeDeploymentURLErrors(t *testing.T) {
	tests := map[string]string{
		"":                        "convex: deployment URL cannot be empty",
		"ftp://example.com":       "convex: deployment URL must start with http:// or https://",
		"https:///missing-host":   "convex: deployment URL must include a host",
		"https://app.convex.site": `convex: deployment URL "https://app.convex.site" ends with .convex.site, which is for HTTP Actions; use the .convex.cloud deployment URL or WithSkipDeploymentURLCheck`,
		"://bad-url":              `convex: invalid deployment URL "://bad-url": parse "://bad-url": missing protocol scheme`,
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			if _, err := normalizeDeploymentURL(input, false); err == nil || err.Error() != want {
				t.Fatalf("unexpected normalize error for %q: %v", input, err)
			}
		})
	}
	if got, err := normalizeDeploymentURL("https://app.convex.site/path/?x=1#frag", true); err != nil || got != "https://app.convex.site/path" {
		t.Fatalf("skip check should allow .convex.site and strip query/fragment, got %q err=%v", got, err)
	}
}

func TestRootMutationHardeningHTTPFunctionEndpointsAndBodies(t *testing.T) {
	var requests []struct {
		path string
		body map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Convex-Client") != "go-hardening" {
			t.Fatalf("unexpected headers: %#v", r.Header)
		}
		var body map[string]any
		if r.Body != nil && r.URL.Path != "/api/query_ts" {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
		}
		requests = append(requests, struct {
			path string
			body map[string]any
		}{path: r.URL.Path, body: body})
		switch r.URL.Path {
		case "/api/query_ts":
			_, _ = w.Write([]byte(`{"ts":"ts-123"}`))
		default:
			_, _ = w.Write([]byte(`{"status":"success","value":"ok","logLines":[]}`))
		}
	}))
	defer server.Close()

	client, err := NewHTTPClient(server.URL, WithClientID("go-hardening"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := client.QueryValue(ctx, "messages:list", map[string]any{"room": "general"}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.MutationValue(ctx, "messages:send", nil, WithSkipMutationQueue()); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ActionValue(ctx, "jobs:run", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := client.FunctionValue(ctx, "generic:call", map[string]any{"x": true}, "component/path"); err != nil {
		t.Fatal(err)
	}
	if ts, err := client.GetTimestamp(ctx); err != nil || ts != "ts-123" {
		t.Fatalf("unexpected timestamp: %q err=%v", ts, err)
	}
	if _, err := client.QueryAtTimestamp(ctx, "messages:list", nil, "ts-123"); err != nil {
		t.Fatal(err)
	}

	wantPaths := []string{"/api/query", "/api/mutation", "/api/action", "/api/function", "/api/query_ts", "/api/query_at_ts"}
	if len(requests) != len(wantPaths) {
		t.Fatalf("unexpected request count: %#v", requests)
	}
	for i, want := range wantPaths {
		if requests[i].path != want {
			t.Fatalf("request %d path = %q, want %q", i, requests[i].path, want)
		}
	}
	if requests[0].body["format"] != "convex_encoded_json" || requests[0].body["path"] != "messages:list" {
		t.Fatalf("unexpected query body: %#v", requests[0].body)
	}
	if requests[3].body["componentPath"] != "component/path" || requests[3].body["path"] != "generic:call" {
		t.Fatalf("unexpected function body: %#v", requests[3].body)
	}
	if requests[5].body["ts"] != "ts-123" || requests[5].body["path"] != "messages:list" {
		t.Fatalf("unexpected query_at_ts body: %#v", requests[5].body)
	}
}

func TestRootMutationHardeningTimestampAndDecodeErrors(t *testing.T) {
	t.Run("timestamp http status lower bound", func(t *testing.T) {
		client, err := NewHTTPClient("https://happy-animal-123.convex.cloud", WithHTTPClient(&http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 199,
					Body:       io.NopCloser(strings.NewReader("not ready")),
					Header:     http.Header{},
					Request:    req,
				}, nil
			}),
		}))
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.GetTimestamp(context.Background())
		var httpErr *HTTPError
		if !errors.As(err, &httpErr) || httpErr.StatusCode != 199 || httpErr.Body != "not ready" {
			t.Fatalf("unexpected timestamp HTTP error: %#v err=%v", httpErr, err)
		}
	})

	t.Run("timestamp decode", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"no_ts":"missing"}`))
		}))
		defer server.Close()
		client, err := NewHTTPClient(server.URL)
		if err != nil {
			t.Fatal(err)
		}
		if ts, err := client.GetTimestamp(context.Background()); err != nil || ts != "" {
			t.Fatalf("missing ts field should decode as empty string, got %q err=%v", ts, err)
		}
	})

	var out struct{ Name string }
	if err := decodeInto(nil, &out); err != nil {
		t.Fatalf("decodeInto should decode nil JSON into zero target, got %v", err)
	}
	if out.Name != "" {
		t.Fatalf("nil decode should leave zero value, got %#v", out)
	}
	if err := decodeInto(map[string]any{"name": "Ada"}, nil); err == nil || err.Error() != "convex: nil output target" {
		t.Fatalf("unexpected nil output error: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRootMutationHardeningFunctionResponseErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("case") {
		case "bad-json":
			_, _ = w.Write([]byte(`{`))
		case "bad-status":
			_, _ = w.Write([]byte(`{"status":"mystery","value":null}`))
		case "bad-error-data":
			w.WriteHeader(statusCodeUDFFailed)
			_, _ = w.Write([]byte(`{"status":"error","errorMessage":"boom","errorData":{"$integer":1}}`))
		default:
			_, _ = w.Write([]byte(`{"status":"success","value":null}`))
		}
	}))
	defer server.Close()

	client, err := NewHTTPClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := client.doFunctionRequest(ctx, QueryKind, "messages:list", server.URL+"/api/query?case=bad-json", nil); err == nil || !strings.Contains(err.Error(), "convex: failed to decode response") {
		t.Fatalf("expected decode error, got %v", err)
	}
	if _, err := client.doFunctionRequest(ctx, QueryKind, "messages:list", server.URL+"/api/query?case=bad-status", nil); err == nil || err.Error() != `convex: invalid response status "mystery"` {
		t.Fatalf("expected invalid status error, got %v", err)
	}
	if _, err := client.doFunctionRequest(ctx, QueryKind, "messages:list", server.URL+"/api/query?case=bad-error-data", nil); err == nil || !strings.Contains(err.Error(), "convex: failed to decode error data") {
		t.Fatalf("expected error data decode error, got %v", err)
	}
}

func TestRootMutationHardeningAdminAuthEncoding(t *testing.T) {
	got, err := encodeAdminAuth("admin", UserIdentityAttributes{"sub": "123", "iss": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "admin:") {
		t.Fatalf("admin auth should include token prefix, got %q", got)
	}
	encoded := strings.TrimPrefix(got, "admin:")
	if encoded == "" {
		t.Fatal("admin acting-as payload should not be empty")
	}
	if _, err := encodeAdminAuth("admin", UserIdentityAttributes{}, UserIdentityAttributes{}); err == nil || err.Error() != "convex: expected at most one acting-as identity" {
		t.Fatalf("unexpected multiple acting-as error: %v", err)
	}
	if got, err := encodeAdminAuth("admin", nil); err != nil || got != "admin" {
		t.Fatalf("nil acting-as should return raw token, got %q err=%v", got, err)
	}
}

func TestRootMutationHardeningMutationContextErrorsReturnPromptly(t *testing.T) {
	client, err := NewHTTPClient("https://happy-animal-123.convex.cloud")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() {
		_, err := client.MutationValue(ctx, "messages:send", nil)
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected canceled mutation, got %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("canceled mutation did not return promptly")
	}

	unready := &HTTPClient{mutationQueue: make(chan struct{})}
	done = make(chan error, 1)
	go func() {
		_, err := unready.MutationValue(context.Background(), "messages:send", nil)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || err.Error() != "convex: unready mutation queue" {
			t.Fatalf("expected unready queue error, got %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("unready mutation queue should not block forever")
	}
}

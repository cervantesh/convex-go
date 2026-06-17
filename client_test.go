package convex

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientQuery(t *testing.T) {
	var seenPath string
	var seenAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenAuth = r.Header.Get("Authorization")
		if r.Header.Get("Convex-Client") != "go-test" {
			t.Fatalf("unexpected Convex-Client header: %q", r.Header.Get("Convex-Client"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["path"] != "messages:list" {
			t.Fatalf("unexpected path in body: %#v", body["path"])
		}
		if body["format"] != "convex_encoded_json" {
			t.Fatalf("unexpected format: %#v", body["format"])
		}
		args := body["args"].([]any)
		arg0 := args[0].(map[string]any)
		limit := arg0["limit"].(map[string]any)
		if limit["$integer"] != "CgAAAAAAAAA=" {
			t.Fatalf("unexpected encoded limit: %#v", limit)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","value":{"ok":true,"items":[1]}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL+"/", WithClientID("go-test"), WithAuth("token"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.Query(context.Background(), "messages:list", map[string]any{"limit": 10})
	if err != nil {
		t.Fatal(err)
	}
	if seenPath != "/api/query" {
		t.Fatalf("unexpected request path: %q", seenPath)
	}
	if seenAuth != "Bearer token" {
		t.Fatalf("unexpected auth header: %q", seenAuth)
	}
	obj := got.(map[string]any)
	if obj["ok"] != true {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestClientAdminAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Convex admin-key" {
			t.Fatalf("unexpected auth header: %q", got)
		}
		_, _ = w.Write([]byte(`{"status":"success","value":null}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, WithAdminAuth("admin-key"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Mutation(context.Background(), "_system/foo:bar", nil); err != nil {
		t.Fatal(err)
	}
}

func TestClientAdminImpersonation(t *testing.T) {
	var seenAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"status":"success","value":null}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, WithAuth("user-token"))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SetAdminAuth("admin-key", UserIdentityAttributes{
		"email":         "ada@example.com",
		"name":          "Ada Lovelace",
		"emailVerified": true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Query(context.Background(), "messages:list", nil); err != nil {
		t.Fatal(err)
	}

	const encodedIdentity = "eyJlbWFpbCI6ImFkYUBleGFtcGxlLmNvbSIsImVtYWlsVmVyaWZpZWQiOnRydWUsIm5hbWUiOiJBZGEgTG92ZWxhY2UifQ=="
	want := "Convex admin-key:" + encodedIdentity
	if seenAuth != want {
		t.Fatalf("unexpected auth header: got %q want %q", seenAuth, want)
	}
}

func TestMutationQueueSerializesByDefault(t *testing.T) {
	var active int32
	var overlapped atomic.Bool
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := atomic.AddInt32(&active, 1); got > 1 {
			overlapped.Store(true)
		}
		requests.Add(1)
		time.Sleep(100 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		_, _ = w.Write([]byte(`{"status":"success","value":null}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	for range 2 {
		go func() {
			defer wg.Done()
			if _, err := client.Mutation(context.Background(), "messages:send", nil); err != nil {
				t.Errorf("mutation failed: %v", err)
			}
		}()
	}
	wg.Wait()

	if requests.Load() != 2 {
		t.Fatalf("expected 2 requests, got %d", requests.Load())
	}
	if overlapped.Load() {
		t.Fatal("mutations overlapped; expected default queue to serialize them")
	}
}

func TestMutationQueueCanBeSkipped(t *testing.T) {
	var active int32
	var overlapped atomic.Bool
	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := atomic.AddInt32(&active, 1); got > 1 {
			overlapped.Store(true)
		}
		requests.Add(1)
		time.Sleep(100 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		_, _ = w.Write([]byte(`{"status":"success","value":null}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	for range 2 {
		go func() {
			defer wg.Done()
			if _, err := client.Mutation(context.Background(), "messages:send", nil, WithSkipMutationQueue()); err != nil {
				t.Errorf("mutation failed: %v", err)
			}
		}()
	}
	wg.Wait()

	if requests.Load() != 2 {
		t.Fatalf("expected 2 requests, got %d", requests.Load())
	}
	if !overlapped.Load() {
		t.Fatal("mutations did not overlap; expected skip queue option to run immediately")
	}
}

func TestMutationQueueHonorsContextWhileWaiting(t *testing.T) {
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var firstOnce sync.Once

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/mutation" {
			t.Fatalf("unexpected endpoint: %s", r.URL.Path)
		}
		firstOnce.Do(func() {
			close(firstStarted)
			<-releaseFirst
		})
		_, _ = w.Write([]byte(`{"status":"success","value":null}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	firstDone := make(chan error, 1)
	go func() {
		_, err := client.Mutation(context.Background(), "messages:send", nil)
		firstDone <- err
	}()

	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first mutation did not reach server")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	secondDone := make(chan error, 1)
	go func() {
		_, err := client.Mutation(ctx, "messages:send", nil)
		secondDone <- err
	}()

	select {
	case err := <-secondDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled while waiting for mutation queue, got %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		close(releaseFirst)
		if err := <-firstDone; err != nil {
			t.Fatalf("first mutation failed during cleanup: %v", err)
		}
		err := <-secondDone
		t.Fatalf("mutation waited for queue despite canceled context; eventual error: %v", err)
	}

	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatalf("first mutation failed: %v", err)
	}
}

func TestMutationQueueRejectsUninitializedClient(t *testing.T) {
	client := &HTTPClient{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := client.Mutation(ctx, "messages:send", nil)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "uninitialized HTTP client") {
			t.Fatalf("expected uninitialized client error, got %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		cancel()
		err := <-done
		t.Fatalf("uninitialized client waited on nil mutation queue; eventual error: %v", err)
	}
}

func TestClientFunctionError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCodeUDFFailed)
		_, _ = w.Write([]byte(`{"status":"error","errorMessage":"bad input","errorData":{"code":{"$integer":"AQAAAAAAAAA="}},"logLines":["line"]}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Action(context.Background(), "doThing", nil)
	var functionErr *FunctionError
	if !errors.As(err, &functionErr) {
		t.Fatalf("expected FunctionError, got %T: %v", err, err)
	}
	var convexErr *ConvexError
	if !errors.As(err, &convexErr) {
		t.Fatalf("expected ConvexError unwrap, got %T: %v", err, err)
	}
	if functionErr.Message != "bad input" || !functionErr.HasData {
		t.Fatalf("unexpected function error: %#v", functionErr)
	}
	data := functionErr.Data.(map[string]any)
	if data["code"] != Int64(1) {
		t.Fatalf("unexpected error data: %#v", data)
	}
	if convexErr.Message != "bad input" {
		t.Fatalf("unexpected convex error: %#v", convexErr)
	}
}

func TestClientFunctionErrorDataVariants(t *testing.T) {
	tests := []struct {
		name     string
		response string
		wantKind ValueKind
		wantGo   any
	}{
		{
			name:     "string",
			response: `{"status":"error","errorMessage":"bad input","errorData":"bad_code"}`,
			wantKind: StringKind,
			wantGo:   "bad_code",
		},
		{
			name:     "object",
			response: `{"status":"error","errorMessage":"bad input","errorData":{"code":"bad_code"}}`,
			wantKind: ObjectKind,
			wantGo:   map[string]any{"code": "bad_code"},
		},
		{
			name:     "numeric",
			response: `{"status":"error","errorMessage":"bad input","errorData":7}`,
			wantKind: Float64Kind,
			wantGo:   float64(7),
		},
		{
			name:     "null",
			response: `{"status":"error","errorMessage":"bad input","errorData":null}`,
			wantKind: NullKind,
			wantGo:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newFunctionErrorServer(tt.response)
			defer server.Close()

			functionErr, convexErr := queryFunctionError(t, server.URL)
			assertConvexErrorData(t, functionErr, convexErr, tt.wantKind, tt.wantGo)
		})
	}
}

func newFunctionErrorServer(response string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCodeUDFFailed)
		_, _ = w.Write([]byte(response))
	}))
}

func queryFunctionError(t *testing.T, serverURL string) (*FunctionError, *ConvexError) {
	t.Helper()
	client, err := NewClient(serverURL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Query(context.Background(), "bad:query", nil)
	var functionErr *FunctionError
	if !errors.As(err, &functionErr) {
		t.Fatalf("expected FunctionError, got %T: %v", err, err)
	}
	var convexErr *ConvexError
	if !errors.As(err, &convexErr) {
		t.Fatalf("expected ConvexError unwrap, got %T: %v", err, err)
	}
	return functionErr, convexErr
}

func assertConvexErrorData(t *testing.T, functionErr *FunctionError, convexErr *ConvexError, wantKind ValueKind, wantGo any) {
	t.Helper()
	if convexErr.Data.Kind() != wantKind {
		t.Fatalf("unexpected ConvexError data kind: got %q want %q", convexErr.Data.Kind(), wantKind)
	}
	if !reflect.DeepEqual(convexErr.Data.GoValue(), wantGo) {
		t.Fatalf("unexpected ConvexError data: got %#v want %#v", convexErr.Data.GoValue(), wantGo)
	}
	if !functionErr.HasData {
		t.Fatal("expected FunctionError.HasData")
	}
}

func TestClientFunctionErrorWithoutErrorDataIsNotConvexError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCodeUDFFailed)
		_, _ = w.Write([]byte(`{"status":"error","errorMessage":"plain failure"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Query(context.Background(), "bad:query", nil)
	var functionErr *FunctionError
	if !errors.As(err, &functionErr) {
		t.Fatalf("expected FunctionError, got %T: %v", err, err)
	}
	var convexErr *ConvexError
	if errors.As(err, &convexErr) {
		t.Fatalf("did not expect ConvexError unwrap: %#v", convexErr)
	}
	if functionErr.HasData {
		t.Fatal("did not expect FunctionError.HasData")
	}
}

func TestClientConvexPyReadmeHTTPShape(t *testing.T) {
	// Source: get-convex/convex-py README basic usage: query without args,
	// mutation with a dict payload, and action calls over the same client.
	var requests []functionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body functionRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, body)
		switch r.URL.Path {
		case "/api/query":
			_, _ = w.Write([]byte(`{"status":"success","value":[{"author":"Tom","body":"Have you tried Convex?"}]}`))
		case "/api/mutation":
			_, _ = w.Write([]byte(`{"status":"success","value":null}`))
		case "/api/action":
			_, _ = w.Write([]byte(`{"status":"success","value":{"ok":true}}`))
		default:
			t.Fatalf("unexpected endpoint: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	messages, err := client.Query(context.Background(), "messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := messages.([]any)[0].(map[string]any)["author"]; got != "Tom" {
		t.Fatalf("unexpected query result: %#v", messages)
	}
	if _, err := client.Mutation(context.Background(), "messages:send", map[string]any{
		"author": "Me",
		"body":   "Hello!",
	}); err != nil {
		t.Fatal(err)
	}
	action, err := client.Action(context.Background(), "jobs:run", map[string]any{"attempt": Number(1)})
	if err != nil {
		t.Fatal(err)
	}
	if action.(map[string]any)["ok"] != true {
		t.Fatalf("unexpected action result: %#v", action)
	}

	if len(requests) != 3 {
		t.Fatalf("expected 3 HTTP function requests, got %#v", requests)
	}
	if requests[0].Path != "messages:list" || len(requests[0].Args) != 1 {
		t.Fatalf("unexpected query request: %#v", requests[0])
	}
	if queryArgs := requests[0].Args[0].(map[string]any); len(queryArgs) != 0 {
		t.Fatalf("expected nil query args to encode as empty object, got %#v", queryArgs)
	}
	mutationArgs := requests[1].Args[0].(map[string]any)
	if mutationArgs["author"] != "Me" || mutationArgs["body"] != "Hello!" {
		t.Fatalf("unexpected mutation args: %#v", mutationArgs)
	}
	actionArgs := requests[2].Args[0].(map[string]any)
	if actionArgs["attempt"] != float64(1) {
		t.Fatalf("expected Number wrapper to encode as JS number, got %#v", actionArgs)
	}
}

func TestClientConvexPyPaginationShape(t *testing.T) {
	// Source: get-convex/convex-py README "Pagination" example.
	var seen functionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/query" {
			t.Fatalf("unexpected endpoint: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&seen); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"status":"success","value":{"page":[],"continueCursor":"next","isDone":false}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Query(context.Background(), "listMessages", map[string]any{
		"paginationOpts": map[string]any{
			"numItems": Number(5),
			"cursor":   nil,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen.Path != "listMessages" || seen.Format != "convex_encoded_json" {
		t.Fatalf("unexpected pagination request: %#v", seen)
	}
	args := seen.Args[0].(map[string]any)
	opts := args["paginationOpts"].(map[string]any)
	if opts["numItems"] != float64(5) || opts["cursor"] != nil {
		t.Fatalf("unexpected pagination opts: %#v", opts)
	}
}

func TestClientHTTPError(t *testing.T) {
	tests := []struct {
		name string
		call func(*Client) error
	}{
		{
			name: "query",
			call: func(client *Client) error {
				_, err := client.Query(context.Background(), "x", nil)
				return err
			},
		},
		{
			name: "timestamp",
			call: func(client *Client) error {
				_, err := client.GetTimestamp(context.Background())
				return err
			},
		},
		{
			name: "generic function",
			call: func(client *Client) error {
				_, err := client.Function(context.Background(), "x", nil, "")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "nope", http.StatusUnauthorized)
			}))
			defer server.Close()

			client, err := NewClient(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			err = tt.call(client)
			var httpErr *HTTPError
			if !errors.As(err, &httpErr) {
				t.Fatalf("expected HTTPError, got %T: %v", err, err)
			}
			if httpErr.StatusCode != http.StatusUnauthorized || !strings.Contains(httpErr.Body, "nope") {
				t.Fatalf("unexpected HTTPError: %#v", httpErr)
			}
		})
	}
}

func TestClientHTTPStatus300IsTransportError(t *testing.T) {
	tests := []struct {
		name string
		call func(*Client) error
	}{
		{
			name: "query",
			call: func(client *Client) error {
				_, err := client.Query(context.Background(), "messages:list", nil)
				return err
			},
		},
		{
			name: "timestamp",
			call: func(client *Client) error {
				_, err := client.GetTimestamp(context.Background())
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusMultipleChoices)
				_, _ = w.Write([]byte("redirect-like response"))
			}))
			defer server.Close()

			client, err := NewClient(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			err = tt.call(client)
			var httpErr *HTTPError
			if !errors.As(err, &httpErr) {
				t.Fatalf("expected HTTPError for status 300, got %T: %v", err, err)
			}
			if httpErr.StatusCode != http.StatusMultipleChoices {
				t.Fatalf("unexpected status code: %#v", httpErr)
			}
		})
	}
}

func TestHTTPClientSetAuthReplacesExistingAdminAuth(t *testing.T) {
	var seenAuth []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = append(seenAuth, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","value":null}`))
	}))
	defer server.Close()

	client, err := NewHTTPClient(server.URL, WithAdminAuth("admin-token"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.QueryValue(context.Background(), "messages:list", nil); err != nil {
		t.Fatal(err)
	}
	client.SetAuth("user-token")
	if _, err := client.QueryValue(context.Background(), "messages:list", nil); err != nil {
		t.Fatal(err)
	}

	if len(seenAuth) != 2 {
		t.Fatalf("expected two requests, got %#v", seenAuth)
	}
	if seenAuth[0] != "Convex admin-token" {
		t.Fatalf("unexpected initial admin auth: %#v", seenAuth)
	}
	if seenAuth[1] != "Bearer user-token" {
		t.Fatalf("expected user auth to replace admin auth, got %#v", seenAuth)
	}
}

func TestHTTPClientSetAdminAuthReplacesExistingUserAuth(t *testing.T) {
	var seenAuth []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = append(seenAuth, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","value":null}`))
	}))
	defer server.Close()

	client, err := NewHTTPClient(server.URL, WithAuth("user-token"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.QueryValue(context.Background(), "messages:list", nil); err != nil {
		t.Fatal(err)
	}
	if err := client.SetAdminAuth("admin-token"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.QueryValue(context.Background(), "messages:list", nil); err != nil {
		t.Fatal(err)
	}

	if len(seenAuth) != 2 {
		t.Fatalf("expected two requests, got %#v", seenAuth)
	}
	if seenAuth[0] != "Bearer user-token" {
		t.Fatalf("unexpected initial user auth: %#v", seenAuth)
	}
	if seenAuth[1] != "Convex admin-token" {
		t.Fatalf("expected admin auth to replace user auth, got %#v", seenAuth)
	}
}

func TestNormalizeDeploymentURLRejectsConvexSite(t *testing.T) {
	_, err := NewClient("https://example.convex.site")
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = NewClient("https://example.convex.site", WithSkipDeploymentURLCheck())
	if err != nil {
		t.Fatal(err)
	}
}

func TestClientIntoMethodsDecodeResponses(t *testing.T) {
	server := newFunctionResponseServer(t, map[string]string{
		"/api/query":    `{"status":"success","value":{"name":"Ada","count":{"$integer":"AwAAAAAAAAA="}}}`,
		"/api/mutation": `{"status":"success","value":{"name":"Grace","count":{"$integer":"BAAAAAAAAAA="}}}`,
		"/api/action":   `{"status":"success","value":{"name":"Katherine","count":{"$integer":"BQAAAAAAAAA="}}}`,
	})
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	assertIntoResult(t, func(out any) error {
		return client.QueryInto(context.Background(), "users:get", nil, out)
	}, "Ada", 3)
	assertIntoResult(t, func(out any) error {
		return client.MutationInto(context.Background(), "users:update", nil, out, WithSkipMutationQueue())
	}, "Grace", 4)
	assertIntoResult(t, func(out any) error {
		return client.ActionInto(context.Background(), "users:sync", nil, out)
	}, "Katherine", 5)
}

func TestClientFunctionUsesGenericEndpoint(t *testing.T) {
	var seen anyFunctionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/function" {
			t.Fatalf("unexpected endpoint: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&seen); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"status":"success","value":{"ok":true}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.Function(context.Background(), "components/foo:run", map[string]any{"n": 1}, "component/path")
	if err != nil {
		t.Fatal(err)
	}
	if seen.Path != "components/foo:run" || seen.ComponentPath != "component/path" {
		t.Fatalf("unexpected function payload: %#v", seen)
	}
	if obj := got.(map[string]any); obj["ok"] != true {
		t.Fatalf("unexpected function result: %#v", got)
	}

	value, err := client.FunctionValue(context.Background(), "components/foo:run", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if obj, _ := value.Object(); obj["ok"].GoValue() != true {
		t.Fatalf("unexpected function value: %#v", value)
	}
}

func TestClientTimestampQueries(t *testing.T) {
	var queryAtTimestampBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/query_ts":
			_, _ = w.Write([]byte(`{"ts":"timestamp-token"}`))
		case "/api/query_at_ts":
			if err := json.NewDecoder(r.Body).Decode(&queryAtTimestampBody); err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte(`{"status":"success","value":"at-ts"}`))
		default:
			t.Fatalf("unexpected endpoint: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	ts, err := client.GetTimestamp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ts != "timestamp-token" {
		t.Fatalf("unexpected timestamp: %q", ts)
	}
	value, err := client.QueryAtTimestamp(context.Background(), "messages:list", nil, ts)
	if err != nil {
		t.Fatal(err)
	}
	if value.GoValue() != "at-ts" || queryAtTimestampBody["ts"] != ts {
		t.Fatalf("unexpected query_at_ts value/body: %#v %#v", value, queryAtTimestampBody)
	}
	if _, err := client.QueryAtTimestamp(context.Background(), "messages:list", nil, " "); err == nil {
		t.Fatal("expected empty timestamp error")
	}
	consistent, err := client.ConsistentQuery(context.Background(), "messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	if consistent.GoValue() != "at-ts" {
		t.Fatalf("unexpected consistent query value: %#v", consistent)
	}
}

func TestClientAuthMutatorsAndDeploymentURL(t *testing.T) {
	var authHeaders []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeaders = append(authHeaders, r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"status":"success","value":null}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL + "/extra/?ignored=true")
	if err != nil {
		t.Fatal(err)
	}
	if client.DeploymentURL() != server.URL+"/extra" {
		t.Fatalf("unexpected normalized deployment URL: %q", client.DeploymentURL())
	}
	client.SetAuth("user-token")
	if _, err := client.Query(context.Background(), "messages:list", nil); err != nil {
		t.Fatal(err)
	}
	client.ClearAuth()
	if _, err := client.Query(context.Background(), "messages:list", nil); err != nil {
		t.Fatal(err)
	}
	if err := client.SetAdminAuth("admin-key"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Query(context.Background(), "messages:list", nil); err != nil {
		t.Fatal(err)
	}
	client.ClearAuth()
	if _, err := client.Query(context.Background(), "messages:list", nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(authHeaders, []string{"Bearer user-token", "", "Convex admin-key", ""}) {
		t.Fatalf("unexpected auth headers: %#v", authHeaders)
	}
}

func TestClientOptionsAndDecodeIntoErrors(t *testing.T) {
	if _, err := NewClient("https://example.convex.cloud", WithHTTPClient(nil)); err == nil {
		t.Fatal("expected nil http client error")
	}
	if _, err := NewClient("https://example.convex.cloud", WithClientID(" ")); err == nil {
		t.Fatal("expected empty client id error")
	}
	if _, err := NewClient(""); err == nil {
		t.Fatal("expected empty deployment URL error")
	}
	if _, err := NewClient("ftp://example.convex.cloud"); err == nil {
		t.Fatal("expected invalid scheme error")
	}
	if err := decodeInto(map[string]any{"ok": true}, nil); err == nil {
		t.Fatal("expected nil output target error")
	}
}

func TestClientWithHTTPClientSuccess(t *testing.T) {
	server := newFunctionResponseServer(t, map[string]string{
		"/api/query": `{"status":"success","value":"ok"}`,
	})
	defer server.Close()
	client, err := NewClient(server.URL, WithHTTPClient(server.Client()))
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
}

func TestClientGetTimestampErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no timestamp", http.StatusBadGateway)
	}))
	defer server.Close()
	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetTimestamp(context.Background()); err == nil {
		t.Fatal("expected timestamp HTTP error")
	}

	badJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer badJSON.Close()
	client, err = NewClient(badJSON.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetTimestamp(context.Background()); err == nil {
		t.Fatal("expected timestamp decode error")
	}
}

func newFunctionResponseServer(t *testing.T, responses map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response, ok := responses[r.URL.Path]
		if !ok {
			t.Fatalf("unexpected endpoint: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(response))
	}))
}

func assertIntoResult(t *testing.T, call func(any) error, wantName string, wantCount int) {
	t.Helper()
	var out struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	if err := call(&out); err != nil {
		t.Fatal(err)
	}
	if out.Name != wantName || out.Count != wantCount {
		t.Fatalf("unexpected decoded result: %#v", out)
	}
}

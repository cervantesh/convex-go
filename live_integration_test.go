//go:build integration

package convex

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	coderwebsocket "github.com/coder/websocket"

	"github.com/cervantesh/convex-go/internal/syncclient"
)

const (
	liveListMessagesPath = "live:listMessages"
	liveSendMessagePath  = "live:sendMessage"
	livePingPath         = "live:ping"
	liveViewerPath       = "live:viewer"
)

type liveMessage struct {
	ID           string  `json:"_id"`
	CreationTime float64 `json:"_creationTime"`
	Room         string  `json:"room"`
	Body         string  `json:"body"`
	RequestID    string  `json:"requestId"`
}

type livePingResponse struct {
	OK    bool   `json:"ok"`
	Value string `json:"value"`
}

type liveViewerIdentity struct {
	Authenticated   bool    `json:"authenticated"`
	TokenIdentifier *string `json:"tokenIdentifier"`
	Subject         *string `json:"subject"`
	Issuer          *string `json:"issuer"`
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

func TestLiveIntegrationHTTPAndSync(t *testing.T) {
	cfg := loadLiveIntegrationConfig(t)

	opts := []Option{}
	if cfg.authToken != "" {
		opts = append(opts, WithAuth(cfg.authToken))
	}

	client, err := NewClient(cfg.deploymentURL, opts...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close client: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	timestamp, err := client.GetTimestamp(ctx)
	if err != nil {
		t.Fatalf("get timestamp: %v", err)
	}
	if strings.TrimSpace(timestamp) == "" {
		t.Fatal("expected non-empty timestamp")
	}

	room := fmt.Sprintf("convex-go-live-%d", time.Now().UnixNano())
	requestID := fmt.Sprintf("request-%d", time.Now().UnixNano())
	body := "hello from convex-go live integration"

	subscription, err := client.Subscribe(ctx, liveListMessagesPath, map[string]any{"room": room})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := subscription.Unsubscribe(cleanupCtx); err != nil && err != ErrSubscriptionClosed {
			t.Fatalf("unsubscribe: %v", err)
		}
	}()

	initialMessages := nextLiveMessages(t, ctx, subscription)
	if len(initialMessages) != 0 {
		t.Fatalf("expected empty initial room, got %#v", initialMessages)
	}

	var mutationResult liveMessage
	if err := client.MutationInto(ctx, liveSendMessagePath, map[string]any{
		"room":      room,
		"body":      body,
		"requestId": requestID,
	}, &mutationResult); err != nil {
		t.Fatalf("mutation: %v", err)
	}
	if mutationResult.Room != room || mutationResult.Body != body || mutationResult.RequestID != requestID {
		t.Fatalf("unexpected mutation result: %#v", mutationResult)
	}

	var actionResult livePingResponse
	if err := client.ActionInto(ctx, livePingPath, map[string]any{"value": requestID}, &actionResult); err != nil {
		t.Fatalf("action: %v", err)
	}
	if !actionResult.OK || actionResult.Value != requestID {
		t.Fatalf("unexpected action result: %#v", actionResult)
	}

	var viewer liveViewerIdentity
	if err := client.QueryInto(ctx, liveViewerPath, nil, &viewer); err != nil {
		t.Fatalf("viewer query: %v", err)
	}
	assertLiveViewerIdentity(t, cfg, viewer)

	updatedMessages := waitForRequestID(t, ctx, subscription, requestID)
	if !containsRequestID(updatedMessages, requestID) {
		t.Fatalf("subscription update did not include request %q: %#v", requestID, updatedMessages)
	}

	var queriedMessages []liveMessage
	if err := client.QueryInto(ctx, liveListMessagesPath, map[string]any{"room": room}, &queriedMessages); err != nil {
		t.Fatalf("query: %v", err)
	}
	if !containsRequestID(queriedMessages, requestID) {
		t.Fatalf("query result did not include request %q: %#v", requestID, queriedMessages)
	}
}

func TestLiveIntegrationAuthCallbackAndReconnect(t *testing.T) {
	cfg := loadLiveIntegrationConfig(t)
	if cfg.authToken == "" {
		t.Skip("CONVEX_AUTH_TOKEN not set")
	}

	dialer := newLiveReconnectDialer()
	client, err := NewClient(
		cfg.deploymentURL,
		withWebSocketDialer(dialer),
		WithWebSocketReconnectBackoff(50*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Fatalf("close client: %v", err)
		}
	}()

	var callsMu sync.Mutex
	var calls []bool
	if err := client.SetAuthCallback(func(forceRefresh bool) (string, error) {
		callsMu.Lock()
		calls = append(calls, forceRefresh)
		callsMu.Unlock()
		if forceRefresh {
			return cfg.refreshAuthToken, nil
		}
		return cfg.authToken, nil
	}); err != nil {
		t.Fatalf("set auth callback: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var viewer liveViewerIdentity
	if err := client.QueryInto(ctx, liveViewerPath, nil, &viewer); err != nil {
		t.Fatalf("viewer query via auth callback: %v", err)
	}
	assertLiveViewerIdentity(t, cfg, viewer)

	room := fmt.Sprintf("convex-go-live-reconnect-%d", time.Now().UnixNano())
	requestID := fmt.Sprintf("reconnect-%d", time.Now().UnixNano())

	subscription, err := client.Subscribe(ctx, liveListMessagesPath, map[string]any{"room": room})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if err := subscription.Unsubscribe(cleanupCtx); err != nil && !errors.Is(err, ErrSubscriptionClosed) {
			t.Fatalf("unsubscribe: %v", err)
		}
	}()

	initialMessages := nextLiveMessages(t, ctx, subscription)
	if len(initialMessages) != 0 {
		t.Fatalf("expected empty initial room, got %#v", initialMessages)
	}

	if err := dialer.ForceDisconnect(); err != nil {
		t.Fatalf("force disconnect: %v", err)
	}
	if err := dialer.WaitForReconnect(ctx); err != nil {
		t.Fatalf("wait for reconnect: %v", err)
	}

	sender, err := NewHTTPClient(cfg.deploymentURL)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.authToken != "" {
		sender.SetAuth(cfg.authToken)
	}

	var mutationResult liveMessage
	if err := sender.MutationInto(ctx, liveSendMessagePath, map[string]any{
		"room":      room,
		"body":      "hello after reconnect",
		"requestId": requestID,
	}, &mutationResult); err != nil {
		t.Fatalf("mutation after reconnect: %v", err)
	}

	updatedMessages := waitForRequestID(t, ctx, subscription, requestID)
	if !containsRequestID(updatedMessages, requestID) {
		t.Fatalf("subscription replay did not include request %q: %#v", requestID, updatedMessages)
	}

	callsMu.Lock()
	defer callsMu.Unlock()
	assertAuthCallbackSawRefresh(t, calls)
}

func nextLiveMessages(t *testing.T, ctx context.Context, subscription *QuerySubscription) []liveMessage {
	t.Helper()
	result, err := subscription.Next(ctx)
	if err != nil {
		t.Fatalf("subscription next: %v", err)
	}
	if err := result.Err(); err != nil {
		t.Fatalf("subscription result error: %v", err)
	}
	value, ok := result.Value()
	if !ok {
		t.Fatal("subscription result had no value")
	}
	var messages []liveMessage
	if err := decodeInto(value.GoValue(), &messages); err != nil {
		t.Fatalf("decode subscription value: %v", err)
	}
	return messages
}

func waitForRequestID(t *testing.T, ctx context.Context, subscription *QuerySubscription, requestID string) []liveMessage {
	t.Helper()
	for {
		messages := nextLiveMessages(t, ctx, subscription)
		if containsRequestID(messages, requestID) {
			return messages
		}
	}
}

func containsRequestID(messages []liveMessage, requestID string) bool {
	for _, message := range messages {
		if message.RequestID == requestID {
			return true
		}
	}
	return false
}

func assertLiveViewerIdentity(t *testing.T, cfg liveIntegrationConfig, viewer liveViewerIdentity) {
	t.Helper()
	if cfg.authToken == "" {
		if viewer.Authenticated {
			t.Fatalf("expected unauthenticated viewer without auth token, got %#v", viewer)
		}
		return
	}
	if !viewer.Authenticated {
		t.Fatalf("expected authenticated viewer with auth token, got %#v", viewer)
	}
	if cfg.expectedSubject != "" && derefString(viewer.Subject) != cfg.expectedSubject {
		t.Fatalf("viewer subject = %q, want %q", derefString(viewer.Subject), cfg.expectedSubject)
	}
	if cfg.expectedIssuer != "" && derefString(viewer.Issuer) != cfg.expectedIssuer {
		t.Fatalf("viewer issuer = %q, want %q", derefString(viewer.Issuer), cfg.expectedIssuer)
	}
	if cfg.expectedTokenIdentifier != "" && derefString(viewer.TokenIdentifier) != cfg.expectedTokenIdentifier {
		t.Fatalf("viewer token identifier = %q, want %q", derefString(viewer.TokenIdentifier), cfg.expectedTokenIdentifier)
	}
}

func assertAuthCallbackSawRefresh(t *testing.T, calls []bool) {
	t.Helper()
	var sawInitial bool
	var sawRefresh bool
	for _, forceRefresh := range calls {
		if forceRefresh {
			sawRefresh = true
		} else {
			sawInitial = true
		}
	}
	if !sawInitial || !sawRefresh {
		t.Fatalf("expected auth callback to observe initial and refresh calls, got %#v", calls)
	}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

type liveReconnectDialer struct {
	mu          sync.Mutex
	first       *liveReconnectConn
	dialCount   int
	reconnected chan struct{}
}

func newLiveReconnectDialer() *liveReconnectDialer {
	return &liveReconnectDialer{
		reconnected: make(chan struct{}),
	}
}

func (d *liveReconnectDialer) Dial(ctx context.Context, url string, header http.Header) (syncclient.Conn, error) {
	conn, _, err := coderwebsocket.Dial(ctx, url, &coderwebsocket.DialOptions{HTTPHeader: header})
	if err != nil {
		return nil, err
	}
	wrapped := &liveReconnectConn{conn: conn}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.dialCount++
	if d.dialCount == 1 {
		d.first = wrapped
	}
	if d.dialCount == 2 {
		select {
		case <-d.reconnected:
		default:
			close(d.reconnected)
		}
	}
	return wrapped, nil
}

func (d *liveReconnectDialer) ForceDisconnect() error {
	d.mu.Lock()
	first := d.first
	d.mu.Unlock()
	if first == nil {
		return fmt.Errorf("first live websocket connection not established")
	}
	return first.forceClose(coderwebsocket.StatusGoingAway, "forced reconnect")
}

func (d *liveReconnectDialer) WaitForReconnect(ctx context.Context) error {
	select {
	case <-d.reconnected:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type liveReconnectConn struct {
	conn     *coderwebsocket.Conn
	closeMu  sync.Mutex
	closed   bool
	closeErr error
}

func (c *liveReconnectConn) Read(ctx context.Context) ([]byte, error) {
	_, data, err := c.conn.Read(ctx)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (c *liveReconnectConn) Write(ctx context.Context, data []byte) error {
	return c.conn.Write(ctx, coderwebsocket.MessageText, data)
}

func (c *liveReconnectConn) Close(error) error {
	return c.forceClose(coderwebsocket.StatusNormalClosure, "")
}

func (c *liveReconnectConn) forceClose(code coderwebsocket.StatusCode, reason string) error {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	if c.closed {
		return c.closeErr
	}
	c.closed = true
	c.closeErr = c.conn.Close(code, reason)
	return c.closeErr
}

//go:build integration

package convex

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	liveListMessagesPath = "live:listMessages"
	liveSendMessagePath  = "live:sendMessage"
	livePingPath         = "live:ping"
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

type liveIntegrationConfig struct {
	deploymentURL string
	authToken     string
}

func loadLiveIntegrationConfig(t *testing.T) liveIntegrationConfig {
	t.Helper()
	deploymentURL := strings.TrimSpace(os.Getenv("CONVEX_URL"))
	if deploymentURL == "" {
		t.Skip("CONVEX_URL not set")
	}
	return liveIntegrationConfig{
		deploymentURL: deploymentURL,
		authToken:     strings.TrimSpace(os.Getenv("CONVEX_AUTH_TOKEN")),
	}
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

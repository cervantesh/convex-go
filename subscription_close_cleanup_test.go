package convex

import (
	"context"
	"testing"
	"time"
)

func TestWebSocketClientCloseClearsTrackedSubscriptionsAndWatchers(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)

	subscription, err := client.Subscribe(context.Background(), "messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	add := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
	watcher, err := client.WatchAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	attempt.conn.receive(t, TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{QueryID: add.QueryID, Value: StringValue("cached"), LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	})
	if _, err := subscription.Next(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := watcher.Next(context.Background()); err != nil {
		t.Fatal(err)
	}

	client.mu.Lock()
	if got := len(client.subscriptions); got != 1 {
		client.mu.Unlock()
		t.Fatalf("subscription registry length before close = %d, want 1", got)
	}
	if got := len(client.watchers); got != 1 {
		client.mu.Unlock()
		t.Fatalf("watcher registry length before close = %d, want 1", got)
	}
	client.mu.Unlock()

	if err := client.Close(); err != nil {
		t.Fatal(err)
	}

	client.mu.Lock()
	defer client.mu.Unlock()
	if client.subscriptions != nil {
		t.Fatalf("subscriptions registry not cleared: %#v", client.subscriptions)
	}
	if client.watchers != nil {
		t.Fatalf("watchers registry not cleared: %#v", client.watchers)
	}
	if client.hasLatest {
		t.Fatal("client should not retain latest snapshot after close")
	}
	if !client.latest.IsEmpty() {
		t.Fatalf("latest snapshot should be empty after close, got %#v", client.latest)
	}
}

func TestWebSocketClientParentContextCancelAlsoClearsRegistries(t *testing.T) {
	parentCtx, cancel := context.WithCancel(context.Background())
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClientWithContext(t, parentCtx, dialer)

	if _, err := client.Subscribe(context.Background(), "messages:list", nil); err != nil {
		t.Fatal(err)
	}
	_ = decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t))
	if _, err := client.WatchAll(context.Background()); err != nil {
		t.Fatal(err)
	}

	cancel()

	select {
	case <-client.done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for client shutdown after parent context cancel")
	}

	client.mu.Lock()
	defer client.mu.Unlock()
	if client.subscriptions != nil {
		t.Fatalf("subscriptions registry not cleared after parent cancel: %#v", client.subscriptions)
	}
	if client.watchers != nil {
		t.Fatalf("watchers registry not cleared after parent cancel: %#v", client.watchers)
	}
}

func newStartedTestWebSocketClientWithContext(t *testing.T, ctx context.Context, dialer *fakeSyncDialer) (*WebSocketClient, fakeDialAttempt) {
	t.Helper()
	client, err := NewWebSocketClient(ctx, "https://happy-animal-123.convex.cloud",
		withWebSocketDialer(dialer),
		WithWebSocketReconnectBackoff(0),
		WithWebSocketInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	attempt := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
	return client, attempt
}

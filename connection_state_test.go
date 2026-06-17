package convex

import (
	"context"
	"errors"
	"testing"
	"time"
)

var errConnectionStateSocketClosed = errors.New("fake socket closed")

func TestClientConnectionStateStartsDisconnectedBeforeRealtimeInit(t *testing.T) {
	client, err := NewClient("https://happy-animal-123.convex.cloud", WithSkipDeploymentURLCheck())
	if err != nil {
		t.Fatal(err)
	}
	if got := client.ConnectionState(); got != (ConnectionState{Phase: ConnectionPhaseDisconnected}) {
		t.Fatalf("unexpected initial connection state: %#v", got)
	}

	updates := make(chan ConnectionState, 8)
	unsubscribe := client.SubscribeToConnectionState(func(state ConnectionState) {
		updates <- state
	})
	defer unsubscribe()

	assertPublicConnectionState(t, waitPublicConnectionState(t, updates), ConnectionState{
		Phase: ConnectionPhaseDisconnected,
	})
}

func TestClientConnectionStateSubscriberTracksLazyRealtimeLifecycle(t *testing.T) {
	dialer := newFakeSyncDialer()
	var blocked <-chan struct{}
	var unblock chan<- struct{}
	dialer.configs <- func(conn *fakeSyncConn) {
		_, blocked, unblock = conn.blockNextWriteNumbered()
	}

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

	updates := make(chan ConnectionState, 8)
	unsubscribe := client.SubscribeToConnectionState(func(state ConnectionState) {
		updates <- state
	})
	defer unsubscribe()
	assertPublicConnectionState(t, waitPublicConnectionState(t, updates), ConnectionState{
		Phase: ConnectionPhaseDisconnected,
	})

	subscribeDone := make(chan error, 1)
	go func() {
		_, err := client.Subscribe(context.Background(), "messages:list", nil)
		subscribeDone <- err
	}()

	attempt := dialer.waitAttempt(t)
	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for blocked connect write")
	}
	connecting := waitPublicConnectionState(t, updates)
	if connecting.Phase != ConnectionPhaseConnecting {
		t.Fatalf("expected connecting state, got %#v", connecting)
	}

	close(unblock)
	_ = decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
	connected := waitPublicConnectionState(t, updates)
	assertPublicConnectionState(t, connected, ConnectionState{
		Phase:            ConnectionPhaseConnected,
		HasEverConnected: true,
		ConnectionCount:  1,
	})
	assertPublicConnectionState(t, client.ConnectionState(), ConnectionState{
		Phase:            ConnectionPhaseConnected,
		HasEverConnected: true,
		ConnectionCount:  1,
	})

	select {
	case err := <-subscribeDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscribe")
	}
}

func TestWebSocketClientConnectionStateTracksReconnect(t *testing.T) {
	dialer := newFakeSyncDialer()
	var blocked <-chan struct{}
	var unblock chan<- struct{}
	dialer.configs <- func(conn *fakeSyncConn) {
		_, blocked, unblock = conn.blockNextWriteNumbered()
	}
	client, err := NewWebSocketClient(context.Background(), "https://happy-animal-123.convex.cloud",
		withWebSocketDialer(dialer),
		WithWebSocketReconnectBackoff(0),
		WithWebSocketInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer closeTestWebSocketClient(t, client)

	updates := make(chan ConnectionState, 16)
	unsubscribe := client.SubscribeToConnectionState(func(state ConnectionState) {
		updates <- state
	})
	defer unsubscribe()

	first := dialer.waitAttempt(t)
	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for blocked connect write")
	}
	waitForConnectionPhase(t, updates, ConnectionPhaseConnecting)
	close(unblock)
	_ = decodeSentClientMessage[ConnectMessage](t, first.conn.waitSent(t))
	assertPublicConnectionState(t, waitPublicConnectionState(t, updates), ConnectionState{
		Phase:            ConnectionPhaseConnected,
		HasEverConnected: true,
		ConnectionCount:  1,
	})
	assertPublicConnectionState(t, client.ConnectionState(), ConnectionState{
		Phase:            ConnectionPhaseConnected,
		HasEverConnected: true,
		ConnectionCount:  1,
	})

	first.conn.closeWithError(errConnectionStateSocketClosed)
	assertPublicConnectionState(t, waitPublicConnectionState(t, updates), ConnectionState{
		Phase:             ConnectionPhaseReconnecting,
		HasEverConnected:  true,
		ConnectionCount:   1,
		ConnectionRetries: 1,
	})
	assertReconnectSnapshot(t, client.ConnectionState())

	second := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, second.conn.waitSent(t))
	assertPublicConnectionState(t, waitPublicConnectionState(t, updates), ConnectionState{
		Phase:             ConnectionPhaseConnected,
		HasEverConnected:  true,
		ConnectionCount:   2,
		ConnectionRetries: 1,
	})
	assertPublicConnectionState(t, client.ConnectionState(), ConnectionState{
		Phase:             ConnectionPhaseConnected,
		HasEverConnected:  true,
		ConnectionCount:   2,
		ConnectionRetries: 1,
	})
}

func TestClientConnectionStateObserverReattachesAfterClose(t *testing.T) {
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

	updates := make(chan ConnectionState, 16)
	unsubscribe := client.SubscribeToConnectionState(func(state ConnectionState) {
		updates <- state
	})
	defer unsubscribe()
	assertPublicConnectionState(t, waitPublicConnectionState(t, updates), ConnectionState{
		Phase: ConnectionPhaseDisconnected,
	})

	first, err := client.Subscribe(context.Background(), "messages:first", nil)
	if err != nil {
		t.Fatal(err)
	}
	firstAttempt := dialer.waitAttempt(t)
	waitForConnectionPhase(t, updates, ConnectionPhaseConnecting)
	_ = decodeSentClientMessage[ConnectMessage](t, firstAttempt.conn.waitSent(t))
	waitForConnectionPhase(t, updates, ConnectionPhaseConnected)

	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	assertPublicConnectionState(t, client.ConnectionState(), ConnectionState{
		Phase: ConnectionPhaseDisconnected,
	})

	second, err := client.Subscribe(context.Background(), "messages:second", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = second.Close() }()
	secondAttempt := dialer.waitAttempt(t)
	waitForConnectionPhase(t, updates, ConnectionPhaseConnecting)
	_ = decodeSentClientMessage[ConnectMessage](t, secondAttempt.conn.waitSent(t))
	waitForConnectionPhase(t, updates, ConnectionPhaseConnected)
	_ = first
}

func waitPublicConnectionState(t *testing.T, updates <-chan ConnectionState) ConnectionState {
	t.Helper()
	select {
	case state := <-updates:
		return state
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for public connection state")
		return ConnectionState{}
	}
}

func assertPublicConnectionState(t *testing.T, got ConnectionState, want ConnectionState) {
	t.Helper()
	if got.Phase != want.Phase ||
		got.HasEverConnected != want.HasEverConnected ||
		got.ConnectionCount != want.ConnectionCount ||
		got.ConnectionRetries != want.ConnectionRetries {
		t.Fatalf("connection state = %#v, want %#v", got, want)
	}
}

func waitForConnectionPhase(t *testing.T, updates <-chan ConnectionState, want ConnectionPhase) ConnectionState {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case state := <-updates:
			if state.Phase == want {
				return state
			}
		case <-deadline:
			t.Fatalf("timed out waiting for connection phase %q", want)
			return ConnectionState{}
		}
	}
}

func assertReconnectSnapshot(t *testing.T, got ConnectionState) {
	t.Helper()
	if !got.HasEverConnected || got.ConnectionRetries != 1 {
		t.Fatalf("unexpected reconnect snapshot: %#v", got)
	}
	if got.ConnectionCount < 1 || got.ConnectionCount > 2 {
		t.Fatalf("unexpected reconnect connection count: %#v", got)
	}
	if got.Phase != ConnectionPhaseReconnecting && got.Phase != ConnectionPhaseConnected {
		t.Fatalf("unexpected reconnect phase: %#v", got)
	}
}

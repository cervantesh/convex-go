package syncclient

import (
	"context"
	"testing"
	"time"

	. "github.com/cervantesh/convex-go/internal/syncprotocol"
)

func TestConnectionStateSubscriberReceivesInitialSnapshotAndSuppressesDuplicates(t *testing.T) {
	manager, err := New("https://happy-animal-123.convex.cloud")
	if err != nil {
		t.Fatal(err)
	}

	updates := make(chan ConnectionState, 8)
	unsubscribe := manager.SubscribeToConnectionState(func(state ConnectionState) {
		updates <- state
	})
	defer unsubscribe()

	assertConnectionState(t, waitConnectionState(t, updates), ConnectionState{
		Phase: ConnectionPhaseDisconnected,
	})
	manager.markConnecting()
	connecting := waitConnectionState(t, updates)
	if connecting.Phase != ConnectionPhaseConnecting || connecting.LastChange.IsZero() {
		t.Fatalf("unexpected connecting state: %#v", connecting)
	}
	manager.markConnecting()
	assertNoConnectionStateUpdate(t, updates)
}

func TestConnectionStateUnsubscribeStopsUpdates(t *testing.T) {
	manager, err := New("https://happy-animal-123.convex.cloud")
	if err != nil {
		t.Fatal(err)
	}

	updates := make(chan ConnectionState, 8)
	unsubscribe := manager.SubscribeToConnectionState(func(state ConnectionState) {
		updates <- state
	})
	_ = waitConnectionState(t, updates)
	unsubscribe()
	manager.markConnecting()
	assertNoConnectionStateUpdate(t, updates)
}

func TestManagerConnectionStateTransitionsAcrossReconnect(t *testing.T) {
	dialer := newFakeSyncDialer()
	manager, err := New("https://happy-animal-123.convex.cloud",
		WithDialer(dialer),
		WithReconnectBackoff(0),
		WithInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}

	updates := make(chan ConnectionState, 16)
	unsubscribe := manager.SubscribeToConnectionState(func(state ConnectionState) {
		updates <- state
	})
	defer unsubscribe()

	assertConnectionState(t, waitConnectionState(t, updates), ConnectionState{
		Phase: ConnectionPhaseDisconnected,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := runManager(t, ctx, manager)
	defer cancel()

	first := waitConnectionState(t, updates)
	if first.Phase != ConnectionPhaseConnecting {
		t.Fatalf("expected connecting state, got %#v", first)
	}

	attempt := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
	connected := waitConnectionState(t, updates)
	assertConnectionState(t, connected, ConnectionState{
		Phase:            ConnectionPhaseConnected,
		HasEverConnected: true,
		ConnectionCount:  1,
	})

	attempt.conn.closeWithError(errFakeSocketClosed)
	reconnecting := waitConnectionState(t, updates)
	assertConnectionState(t, reconnecting, ConnectionState{
		Phase:             ConnectionPhaseReconnecting,
		HasEverConnected:  true,
		ConnectionCount:   1,
		ConnectionRetries: 1,
	})

	retry := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, retry.conn.waitSent(t))
	reconnected := waitConnectionState(t, updates)
	assertConnectionState(t, reconnected, ConnectionState{
		Phase:             ConnectionPhaseConnected,
		HasEverConnected:  true,
		ConnectionCount:   2,
		ConnectionRetries: 1,
	})

	cancel()
	waitManagerDone(t, done)
	disconnected := waitConnectionState(t, updates)
	assertConnectionState(t, disconnected, ConnectionState{
		Phase:             ConnectionPhaseDisconnected,
		HasEverConnected:  true,
		ConnectionCount:   2,
		ConnectionRetries: 1,
	})
}

func assertNoConnectionStateUpdate(t *testing.T, updates <-chan ConnectionState) {
	t.Helper()
	select {
	case state := <-updates:
		t.Fatalf("unexpected connection state update: %#v", state)
	case <-time.After(20 * time.Millisecond):
	}
}

func waitConnectionState(t *testing.T, updates <-chan ConnectionState) ConnectionState {
	t.Helper()
	select {
	case state := <-updates:
		return state
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for connection state")
		return ConnectionState{}
	}
}

func assertConnectionState(t *testing.T, got ConnectionState, want ConnectionState) {
	t.Helper()
	if got.Phase != want.Phase ||
		got.HasEverConnected != want.HasEverConnected ||
		got.ConnectionCount != want.ConnectionCount ||
		got.ConnectionRetries != want.ConnectionRetries {
		t.Fatalf("connection state = %#v, want %#v", got, want)
	}
}

package syncclient

import (
	"context"
	"testing"
	"time"

	. "github.com/cervantesh/convex-go/internal/syncprotocol"
)

func TestManagerSoakReconnectsAndReplaysAcrossMultipleFailures(t *testing.T) {
	dialer := newFakeSyncDialer()
	manager, err := New("wss://happy-animal-123.convex.cloud",
		WithDialer(dialer),
		WithReconnectBackoff(0),
		WithInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.SetAuth("steady-token"); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runManager(t, ctx, manager)

	attempt := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
	auth := decodeSentClientMessage[AuthenticateMessage](t, attempt.conn.waitSent(t))
	if auth.Value != "steady-token" || auth.TokenType != AuthTokenUser {
		t.Fatalf("unexpected initial auth: %#v", auth)
	}

	if _, err := manager.Subscribe("messages:list", nil); err != nil {
		t.Fatal(err)
	}
	if err := manager.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	add := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))

	for reconnectNumber := 1; reconnectNumber <= 3; reconnectNumber++ {
		attempt.conn.closeWithError(errFakeSocketClosed)

		attempt = dialer.waitAttempt(t)
		connect := decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
		if connect.ConnectionCount != uint32(reconnectNumber) {
			t.Fatalf("unexpected reconnect count after cycle %d: %#v", reconnectNumber, connect)
		}
		auth = decodeSentClientMessage[AuthenticateMessage](t, attempt.conn.waitSent(t))
		if auth.Value != "steady-token" || auth.TokenType != AuthTokenUser {
			t.Fatalf("unexpected replay auth after cycle %d: %#v", reconnectNumber, auth)
		}
		replayedAdd := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
		if replayedAdd.QueryID != add.QueryID {
			t.Fatalf("unexpected replay query after cycle %d: %#v", reconnectNumber, replayedAdd)
		}
	}

	cancel()
	waitManagerDone(t, done)
}

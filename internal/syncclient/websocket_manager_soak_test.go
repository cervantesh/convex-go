package syncclient

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cervantesh/convex-go/baseclient"
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

func TestManagerSoakReconnectRefreshesAuthAcrossMultipleFailures(t *testing.T) {
	dialer := newFakeSyncDialer()
	manager, err := New("wss://happy-animal-123.convex.cloud",
		WithDialer(dialer),
		WithReconnectBackoff(0),
		WithInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	refreshCalls := 0
	if err := manager.SetAuthCallback(func(forceRefresh bool) (baseclient.AuthToken, error) {
		if forceRefresh {
			refreshCalls++
			return baseclient.UserAuthToken(fmt.Sprintf("fresh-token-%d", refreshCalls)), nil
		}
		return baseclient.UserAuthToken("initial-token"), nil
	}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runManager(t, ctx, manager)

	attempt := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
	auth := decodeSentClientMessage[AuthenticateMessage](t, attempt.conn.waitSent(t))
	if auth.Value != "initial-token" || auth.TokenType != AuthTokenUser {
		t.Fatalf("unexpected initial callback auth: %#v", auth)
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
			t.Fatalf("unexpected reconnect count after auth-refresh cycle %d: %#v", reconnectNumber, connect)
		}
		auth = decodeSentClientMessage[AuthenticateMessage](t, attempt.conn.waitSent(t))
		wantToken := fmt.Sprintf("fresh-token-%d", reconnectNumber)
		if auth.Value != wantToken || auth.TokenType != AuthTokenUser {
			t.Fatalf("unexpected refreshed auth after cycle %d: %#v", reconnectNumber, auth)
		}
		replayedAdd := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
		if replayedAdd.QueryID != add.QueryID {
			t.Fatalf("unexpected replay query after auth-refresh cycle %d: %#v", reconnectNumber, replayedAdd)
		}
	}
	if refreshCalls != 3 {
		t.Fatalf("expected 3 forced refresh calls, got %d", refreshCalls)
	}

	cancel()
	waitManagerDone(t, done)
}

package convex

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/cervantesh/convex-go/internal/syncprotocol"
)

func TestSubscriptionSoakCoalescesBurstUpdates(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	subscription, err := client.Subscribe(ctx, "messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = subscription.Close() }()
	add := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))

	var previous syncprotocol.SyncTimestamp
	for burst := 1; burst <= 3; burst++ {
		want := sendBurstUpdates(t, attempt.conn, add.QueryID, previous, burst, 16)
		previous += 16
		waitClientLatestValue(t, client, subscription.ID(), want)
		result, err := subscription.Next(ctx)
		if err != nil {
			t.Fatal(err)
		}
		value, ok := result.Value()
		if !ok || value.GoValue() != want {
			t.Fatalf("expected burst %d latest value %q, got %#v", burst, want, result)
		}
	}
}

func TestWatchAllSoakCoalescesBurstSnapshots(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	watcher, err := client.WatchAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = watcher.Close() }()

	subscription, err := client.Subscribe(ctx, "messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = subscription.Close() }()
	add := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))

	var previous syncprotocol.SyncTimestamp
	for burst := 1; burst <= 3; burst++ {
		want := sendBurstUpdates(t, attempt.conn, add.QueryID, previous, burst, 16)
		previous += 16
		waitClientLatestValue(t, client, subscription.ID(), want)
		results, err := watcher.Next(ctx)
		if err != nil {
			t.Fatal(err)
		}
		result, ok := results.Get(subscription.ID())
		if !ok {
			t.Fatalf("expected watcher result for burst %d", burst)
		}
		value, ok := result.Value()
		if !ok || value.GoValue() != want {
			t.Fatalf("expected watcher burst %d latest value %q, got %#v", burst, want, result)
		}
	}
}

func TestSubscriptionSoakCanceledUnsubscribeCanRetryAcrossIterations(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)

	for iteration := 1; iteration <= 3; iteration++ {
		subscription, err := client.Subscribe(context.Background(), "messages:list", nil)
		if err != nil {
			t.Fatalf("iteration %d subscribe: %v", iteration, err)
		}
		add := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))

		blocked, unblock := attempt.conn.blockNextWrite()
		cancelDuringFlush, cancel := context.WithCancel(context.Background())
		unsubscribeDone := make(chan error, 1)
		go func() {
			unsubscribeDone <- subscription.Unsubscribe(cancelDuringFlush)
		}()
		select {
		case <-blocked:
		case <-time.After(time.Second):
			t.Fatalf("iteration %d timed out waiting for blocked unsubscribe flush", iteration)
		}
		cancel()
		select {
		case err := <-unsubscribeDone:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("iteration %d expected canceled unsubscribe failure, got %v", iteration, err)
			}
		case <-time.After(time.Second):
			t.Fatalf("iteration %d timed out waiting for canceled unsubscribe", iteration)
		}

		stillActive, cancelStillActive := context.WithTimeout(context.Background(), time.Nanosecond)
		waitContextDone(t, stillActive)
		if _, err := subscription.Next(stillActive); !errors.Is(err, context.DeadlineExceeded) {
			cancelStillActive()
			t.Fatalf("iteration %d expected subscription to remain active after failed close, got %v", iteration, err)
		}
		cancelStillActive()

		close(unblock)
		remove := onlyQuerySetRemove(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
		if remove.QueryID != add.QueryID {
			t.Fatalf("iteration %d unexpected remove while retrying unsubscribe: %#v", iteration, remove)
		}

		if err := subscription.Unsubscribe(context.Background()); err != nil {
			t.Fatalf("iteration %d retry unsubscribe: %v", iteration, err)
		}
		if _, err := subscription.Next(context.Background()); !errors.Is(err, ErrSubscriptionClosed) {
			t.Fatalf("iteration %d expected subscription closed after retry, got %v", iteration, err)
		}
	}
}

func waitContextDone(t *testing.T, ctx context.Context) {
	t.Helper()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for context deadline")
	}
}

func yieldTestLoop() {
	runtime.Gosched()
}

func sendBurstUpdates(t *testing.T, conn *fakeSyncConn, queryID syncprotocol.QueryID, previous syncprotocol.SyncTimestamp, burst, count int) string {
	t.Helper()
	var want string
	for step := 1; step <= count; step++ {
		ts := previous + syncprotocol.SyncTimestamp(step)
		want = fmt.Sprintf("burst-%d-%02d", burst, step)
		conn.receive(t, TransitionMessage{
			StartVersion: StateVersion{TS: ts - 1},
			EndVersion:   StateVersion{TS: ts},
			Modifications: []StateModification{
				QueryUpdated{
					QueryID:  queryID,
					Value:    StringValue(want),
					LogLines: []string{},
					Journal:  OptionalString{Present: true},
				},
			},
		})
	}
	return want
}

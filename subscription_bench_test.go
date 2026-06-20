package convex

import (
	"context"
	"testing"
	"time"

	"github.com/cervantesh/convex-go/internal/syncprotocol"
)

func BenchmarkWebSocketClientSubscriptionThroughput(b *testing.B) {
	dialer := newFakeSyncDialer()
	client, attempt := benchmarkStartedWebSocketClient(b, dialer)
	defer closeBenchmarkWebSocketClient(b, client)

	subscription, err := client.Subscribe(context.Background(), "messages:list", nil)
	if err != nil {
		b.Fatal(err)
	}
	add := benchmarkOnlyQuerySetAdd(b, benchmarkDecodeSentClientMessage[ModifyQuerySetMessage](b, benchmarkWaitSent(b, attempt.conn)))
	ctx := context.Background()
	var previous syncprotocol.SyncTimestamp

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ts := previous + 1
		benchmarkQueueServerMessage(b, attempt.conn, TransitionMessage{
			StartVersion: StateVersion{TS: previous},
			EndVersion:   StateVersion{TS: ts},
			Modifications: []StateModification{
				QueryUpdated{QueryID: add.QueryID, Value: StringValue("bench"), LogLines: []string{}, Journal: OptionalString{Present: true}},
			},
		})
		result, err := subscription.Next(ctx)
		if err != nil {
			b.Fatal(err)
		}
		value, ok := result.Value()
		if !ok || value.GoValue() != "bench" {
			b.Fatalf("unexpected benchmark subscription result: %#v", result)
		}
		previous = ts
	}
}

func benchmarkStartedWebSocketClient(tb testing.TB, dialer *fakeSyncDialer) (*WebSocketClient, fakeDialAttempt) {
	tb.Helper()
	client, err := NewWebSocketClient(context.Background(), "https://happy-animal-123.convex.cloud",
		withWebSocketDialer(dialer),
		WithWebSocketReconnectBackoff(0),
		WithWebSocketInactivityTimeout(time.Hour),
	)
	if err != nil {
		tb.Fatal(err)
	}
	attempt := benchmarkWaitAttempt(tb, dialer)
	_ = benchmarkDecodeSentClientMessage[ConnectMessage](tb, benchmarkWaitSent(tb, attempt.conn))
	return client, attempt
}

func closeBenchmarkWebSocketClient(tb testing.TB, client *WebSocketClient) {
	tb.Helper()
	if err := client.Close(); err != nil {
		tb.Fatal(err)
	}
}

func benchmarkWaitAttempt(tb testing.TB, dialer *fakeSyncDialer) fakeDialAttempt {
	tb.Helper()
	select {
	case attempt := <-dialer.attempts:
		return attempt
	case <-time.After(time.Second):
		tb.Fatal("timed out waiting for dial attempt")
		return fakeDialAttempt{}
	}
}

func benchmarkWaitSent(tb testing.TB, conn *fakeSyncConn) []byte {
	tb.Helper()
	select {
	case data := <-conn.sent:
		return data
	case <-time.After(time.Second):
		tb.Fatal("timed out waiting for sent message")
		return nil
	}
}

func benchmarkQueueServerMessage(tb testing.TB, conn *fakeSyncConn, msg ServerMessage) {
	tb.Helper()
	data, err := syncprotocol.EncodeServerMessage(msg)
	if err != nil {
		tb.Fatal(err)
	}
	select {
	case conn.incoming <- data:
	case <-time.After(time.Second):
		tb.Fatal("timed out queueing benchmark server message")
	}
}

func benchmarkDecodeSentClientMessage[T ClientMessage](tb testing.TB, data []byte) T {
	tb.Helper()
	msg, err := syncprotocol.DecodeClientMessage(data)
	if err != nil {
		tb.Fatal(err)
	}
	typed, ok := msg.(T)
	if !ok {
		tb.Fatalf("unexpected client message type %T: %s", msg, data)
	}
	return typed
}

func benchmarkOnlyQuerySetAdd(tb testing.TB, msg ModifyQuerySetMessage) QuerySetAdd {
	tb.Helper()
	if len(msg.Modifications) != 1 {
		tb.Fatalf("expected one query set modification, got %#v", msg.Modifications)
	}
	add, ok := msg.Modifications[0].(QuerySetAdd)
	if !ok {
		tb.Fatalf("expected QuerySetAdd, got %T %#v", msg.Modifications[0], msg.Modifications[0])
	}
	return add
}

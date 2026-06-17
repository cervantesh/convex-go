package convex

import (
	"context"
	"testing"
	"time"
)

func TestWebSocketClientQueryValueCleanupIsBoundedWhenUnsubscribeFlushBlocks(t *testing.T) {
	oldTimeout := defaultQueryCleanupTimeout
	defaultQueryCleanupTimeout = 20 * time.Millisecond
	defer func() {
		defaultQueryCleanupTimeout = oldTimeout
	}()

	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)

	done := make(chan struct {
		value Value
		err   error
	}, 1)
	go func() {
		value, err := client.QueryValue(context.Background(), "messages:list", nil)
		done <- struct {
			value Value
			err   error
		}{value: value, err: err}
	}()

	add := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
	blocked, _ := attempt.conn.blockNextWrite()
	attempt.conn.receive(t, TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{QueryID: add.QueryID, Value: StringValue("first"), LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	})

	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for blocked query cleanup flush")
	}

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatal(result.err)
		}
		if got, ok := result.value.String(); !ok || got != "first" {
			t.Fatalf("unexpected query value result: %#v", result.value)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for bounded query cleanup")
	}
}

package convex

import (
	"context"
	"testing"
	"time"
)

func TestWebSocketClientLifecycleBudgetClosesDoneAcrossIterations(t *testing.T) {
	for iteration := 1; iteration <= 3; iteration++ {
		dialer := newFakeSyncDialer()
		client, attempt := newStartedTestWebSocketClient(t, dialer)

		subscription, err := client.Subscribe(context.Background(), "messages:list", nil)
		if err != nil {
			t.Fatalf("iteration %d subscribe: %v", iteration, err)
		}
		add := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
		watcher, err := client.WatchAll(context.Background())
		if err != nil {
			t.Fatalf("iteration %d watch all: %v", iteration, err)
		}
		attempt.conn.receive(t, TransitionMessage{
			StartVersion: StateVersion{},
			EndVersion:   StateVersion{TS: 1},
			Modifications: []StateModification{
				QueryUpdated{QueryID: add.QueryID, Value: StringValue("active"), LogLines: []string{}, Journal: OptionalString{Present: true}},
			},
		})
		if _, err := subscription.Next(context.Background()); err != nil {
			t.Fatalf("iteration %d subscription next: %v", iteration, err)
		}
		if _, err := watcher.Next(context.Background()); err != nil {
			t.Fatalf("iteration %d watcher next: %v", iteration, err)
		}

		if err := client.Close(); err != nil {
			t.Fatalf("iteration %d close: %v", iteration, err)
		}
		select {
		case <-client.done:
		case <-time.After(time.Second):
			t.Fatalf("iteration %d timed out waiting for client shutdown", iteration)
		}
	}
}

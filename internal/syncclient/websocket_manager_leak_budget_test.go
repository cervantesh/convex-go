package syncclient

import (
	"context"
	"testing"
	"time"

	. "github.com/cervantesh/convex-go/internal/syncprotocol"
)

func TestManagerLifecycleBudgetStopsAcrossIterations(t *testing.T) {
	for iteration := 1; iteration <= 3; iteration++ {
		dialer := newFakeSyncDialer()
		manager, err := New("wss://happy-animal-123.convex.cloud",
			WithDialer(dialer),
			WithReconnectBackoff(0),
			WithInactivityTimeout(time.Hour),
		)
		if err != nil {
			t.Fatalf("iteration %d new manager: %v", iteration, err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		done := runManager(t, ctx, manager)
		attempt := dialer.waitAttempt(t)
		_ = decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
		if _, err := manager.Subscribe("messages:list", nil); err != nil {
			cancel()
			waitManagerDone(t, done)
			t.Fatalf("iteration %d subscribe: %v", iteration, err)
		}
		if err := manager.Flush(ctx); err != nil {
			cancel()
			waitManagerDone(t, done)
			t.Fatalf("iteration %d flush: %v", iteration, err)
		}
		_ = onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))

		cancel()
		waitManagerDone(t, done)
	}
}

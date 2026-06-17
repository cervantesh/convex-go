package convex

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSubscriptionNextAndClose(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, err := NewWebSocketClient(context.Background(), "https://happy-animal-123.convex.cloud",
		withWebSocketDialer(dialer),
		WithWebSocketReconnectBackoff(0),
		WithWebSocketInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	attempt := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	subscription, err := client.Subscribe(ctx, "messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	if subscription.ID() == (SubscriberID{}) {
		t.Fatal("expected non-zero subscription id")
	}
	add := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
	queryID := add.QueryID

	attempt.conn.receive(t, TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{QueryID: queryID, Value: StringValue("hello"), LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	})
	result, err := subscription.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	value, ok := result.Value()
	if !ok || value.GoValue() != "hello" {
		t.Fatalf("unexpected subscription value: %#v", result)
	}

	if err := subscription.Close(); err != nil {
		t.Fatal(err)
	}
	remove := onlyQuerySetRemove(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
	if remove.QueryID != queryID {
		t.Fatalf("unexpected query remove: %#v", remove)
	}
	if err := subscription.Close(); err != nil {
		t.Fatal(err)
	}
	assertNoWriteDuring(t, attempt.conn, subscription.Close)
}

func TestWebSocketClientQueryUsesFirstSubscriptionResult(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, err := NewWebSocketClient(context.Background(), "https://happy-animal-123.convex.cloud",
		withWebSocketDialer(dialer),
		WithWebSocketReconnectBackoff(0),
		WithWebSocketInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	attempt := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	queryDone := make(chan queryResult, 1)
	go func() {
		value, err := client.Query(ctx, "messages:list", nil)
		queryDone <- queryResult{value: value, err: err}
	}()

	add := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
	attempt.conn.receive(t, TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{QueryID: add.QueryID, Value: StringValue("first"), LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	})
	select {
	case result := <-queryDone:
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.value != "first" {
			t.Fatalf("unexpected query result: %#v", result.value)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for query")
	}
	remove := onlyQuerySetRemove(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
	if remove.QueryID != add.QueryID {
		t.Fatalf("unexpected query cleanup remove: %#v", remove)
	}
}

func TestWebSocketClientMutationWithOptimisticUpdatePublishesToSubscription(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	subscription, err := client.Subscribe(ctx, "messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	add := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
	attempt.conn.receive(t, TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{QueryID: add.QueryID, Value: StringValue("server"), LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	})
	assertNextValue(t, ctx, subscription, "server")

	err = client.Mutation(ctx, "messages:send", nil,
		WithOptimisticUpdate(func(store *OptimisticLocalStore) error {
			return store.SetQuery("messages:list", nil, StringValue("optimistic"))
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	mutation := decodeSentClientMessage[MutationMessage](t, attempt.conn.waitSent(t))
	if mutation.RequestID != 0 {
		t.Fatalf("unexpected mutation request id: got %d want 0", mutation.RequestID)
	}
	assertNextValue(t, ctx, subscription, "optimistic")
}

func TestSubscriptionsShareQueryAndCloseIndependently(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	first, err := client.Subscribe(ctx, "messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	add := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
	var second *QuerySubscription
	assertNoWriteDuring(t, attempt.conn, func() error {
		var err error
		second, err = client.Subscribe(ctx, "messages:list", nil)
		return err
	})

	attempt.conn.receive(t, TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{QueryID: add.QueryID, Value: StringValue("shared"), LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	})
	assertNextValue(t, ctx, first, "shared")
	assertNextValue(t, ctx, second, "shared")

	assertNoWriteDuring(t, attempt.conn, first.Close)
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	remove := onlyQuerySetRemove(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
	if remove.QueryID != add.QueryID {
		t.Fatalf("unexpected final remove: %#v", remove)
	}
}

func TestSubscriptionReceivesCachedResult(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	first, err := client.Subscribe(ctx, "messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	add := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
	attempt.conn.receive(t, TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{QueryID: add.QueryID, Value: StringValue("cached"), LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	})
	assertNextValue(t, ctx, first, "cached")

	var second *QuerySubscription
	assertNoWriteDuring(t, attempt.conn, func() error {
		var err error
		second, err = client.Subscribe(ctx, "messages:list", nil)
		return err
	})
	assertNextValue(t, ctx, second, "cached")
}

func TestSubscriptionSkipsLaggedUpdates(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	subscription, err := client.Subscribe(ctx, "messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	add := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
	attempt.conn.receive(t, TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{QueryID: add.QueryID, Value: StringValue("old"), LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	})
	attempt.conn.receive(t, TransitionMessage{
		StartVersion: StateVersion{TS: 1},
		EndVersion:   StateVersion{TS: 2},
		Modifications: []StateModification{
			QueryUpdated{QueryID: add.QueryID, Value: StringValue("new"), LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	})
	waitClientLatestValue(t, client, subscription.ID(), "new")
	assertNextValue(t, ctx, subscription, "new")
}

func TestSubscriptionNextContextAndClose(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)
	subscription, err := client.Subscribe(context.Background(), "messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t))

	expired, cancelExpired := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancelExpired()
	waitContextDone(t, expired)
	if _, err := subscription.Next(expired); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline, got %v", err)
	}

	nextDone := make(chan error, 1)
	go func() {
		_, err := subscription.Next(context.Background())
		nextDone <- err
	}()
	if err := subscription.Close(); err != nil {
		t.Fatal(err)
	}
	_ = decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t))
	select {
	case err := <-nextDone:
		if !errors.Is(err, ErrSubscriptionClosed) {
			t.Fatalf("expected closed error, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for blocked Next to unblock")
	}
}

func TestSubscriptionUnsubscribeCanRetryAfterFlushContextDeadline(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)
	subscription, err := client.Subscribe(context.Background(), "messages:list", nil)
	if err != nil {
		t.Fatal(err)
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
		t.Fatal("timed out waiting for blocked unsubscribe flush")
	}
	cancel()
	select {
	case err := <-unsubscribeDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected canceled unsubscribe failure, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for canceled unsubscribe")
	}
	stillActive, cancelStillActive := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancelStillActive()
	waitContextDone(t, stillActive)
	if _, err := subscription.Next(stillActive); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected subscription to remain active after failed close, got %v", err)
	}
	close(unblock)
	remove := onlyQuerySetRemove(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
	if remove.QueryID != add.QueryID {
		t.Fatalf("unexpected remove while retrying unsubscribe: %#v", remove)
	}

	if err := subscription.Unsubscribe(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := subscription.Next(context.Background()); !errors.Is(err, ErrSubscriptionClosed) {
		t.Fatalf("expected subscription closed after retry, got %v", err)
	}
}

func TestWatchAllReflectsLocalUnsubscribe(t *testing.T) {
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
	add := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
	attempt.conn.receive(t, TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{QueryID: add.QueryID, Value: StringValue("active"), LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	})
	waitClientLatestValue(t, client, subscription.ID(), "active")
	if err := subscription.Close(); err != nil {
		t.Fatal(err)
	}
	_ = decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t))
	results, err := watcher.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !results.IsEmpty() || results.Len() != 0 {
		t.Fatalf("expected watcher to receive empty snapshot after local unsubscribe, got len=%d", results.Len())
	}

	newWatcher, err := client.WatchAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = newWatcher.Close() }()
	results, err = newWatcher.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !results.IsEmpty() || results.Len() != 0 {
		t.Fatalf("expected new watcher to receive empty latest snapshot, got len=%d", results.Len())
	}
}

func TestWatchAllSkipsLaggedSnapshots(t *testing.T) {
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
	add := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))

	attempt.conn.receive(t, TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{QueryID: add.QueryID, Value: StringValue("old"), LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	})
	attempt.conn.receive(t, TransitionMessage{
		StartVersion: StateVersion{TS: 1},
		EndVersion:   StateVersion{TS: 2},
		Modifications: []StateModification{
			QueryUpdated{QueryID: add.QueryID, Value: StringValue("new"), LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	})
	waitClientLatestValue(t, client, subscription.ID(), "new")
	results, err := watcher.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	result, ok := results.Get(subscription.ID())
	if !ok {
		t.Fatal("expected latest snapshot result")
	}
	value, ok := result.Value()
	if !ok || value.GoValue() != "new" {
		t.Fatalf("expected coalesced latest value, got %#v", result)
	}
}

func TestWebSocketClientRejectsCanceledContexts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &WebSocketClient{done: make(chan struct{})}
	if _, err := client.Subscribe(ctx, "messages:list", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled Subscribe, got %v", err)
	}
	if _, err := client.WatchAll(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled WatchAll, got %v", err)
	}
	subscription := &QuerySubscription{}
	if _, err := subscription.Next(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled subscription Next, got %v", err)
	}
	if err := subscription.Unsubscribe(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled Unsubscribe, got %v", err)
	}
	watcher := &QuerySetSubscription{}
	if _, err := watcher.Next(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled watcher Next, got %v", err)
	}
}

func TestWebSocketClientClosedStatePropagatesRunError(t *testing.T) {
	runErr := errors.New("run failed")
	client := &WebSocketClient{
		done: make(chan struct{}),
		ctx:  context.Background(),
	}
	client.runErr = runErr
	close(client.done)
	if err := client.doneErr(); !errors.Is(err, runErr) {
		t.Fatalf("expected run error, got %v", err)
	}
	subscription := &QuerySubscription{
		client:  client,
		updates: make(chan FunctionResult),
		closed:  make(chan struct{}),
	}
	if _, err := subscription.Next(context.Background()); !errors.Is(err, runErr) {
		t.Fatalf("expected subscription Next run error, got %v", err)
	}
	watcher := &QuerySetSubscription{
		client:  client,
		updates: make(chan QueryResults),
		closed:  make(chan struct{}),
	}
	if _, err := watcher.Next(context.Background()); !errors.Is(err, runErr) {
		t.Fatalf("expected watcher Next run error, got %v", err)
	}
}

func TestWebSocketClientQueryValueReturnsFunctionResultError(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	queryDone := make(chan error, 1)
	go func() {
		_, err := client.QueryValue(ctx, "messages:list", nil)
		queryDone <- err
	}()

	add := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
	attempt.conn.receive(t, TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryFailed{QueryID: add.QueryID, ErrorMessage: "bad query", LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	})
	select {
	case err := <-queryDone:
		var functionErr *FunctionError
		if !errors.As(err, &functionErr) || functionErr.Message != "bad query" {
			t.Fatalf("expected FunctionError, got %T: %v", err, err)
		}
		if functionErr.Kind != QueryKind || functionErr.Path != "messages:list" {
			t.Fatalf("expected query path context, got %#v", functionErr)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for query error")
	}
	_ = decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t))
}

func TestQuerySetSubscriptionNextAfterClose(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, _ := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)
	watcher, err := client.WatchAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := watcher.Next(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := watcher.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := watcher.Next(context.Background()); !errors.Is(err, ErrSubscriptionClosed) {
		t.Fatalf("expected closed watcher error, got %v", err)
	}
	if err := watcher.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestWebSocketClientSubscribeFailsWhenClientCanceledDuringFlush(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)

	blocked, _ := attempt.conn.blockNextWrite()
	done := make(chan subscribeResult, 1)
	go func() {
		subscription, err := client.Subscribe(context.Background(), "messages:list", nil)
		done <- subscribeResult{subscription: subscription, err: err}
	}()

	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for blocked subscribe flush")
	}
	client.cancel()

	select {
	case result := <-done:
		if !errors.Is(result.err, context.Canceled) {
			t.Fatalf("expected canceled subscribe, got subscription=%#v err=%v", result.subscription, result.err)
		}
		if result.subscription != nil {
			t.Fatalf("subscribe returned a subscription after client cancellation: %#v", result.subscription)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscribe cancellation")
	}
}

func TestSubscriptionUnsubscribeFailsWhenClientCanceledDuringFlush(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)
	subscription, err := client.Subscribe(context.Background(), "messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t))

	blocked, _ := attempt.conn.blockNextWrite()
	done := make(chan error, 1)
	go func() {
		done <- subscription.Unsubscribe(context.Background())
	}()

	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for blocked unsubscribe flush")
	}
	client.cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected canceled unsubscribe, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for unsubscribe cancellation")
	}

	client.mu.Lock()
	_, active := client.subscriptions[subscription.ID()]
	client.mu.Unlock()
	if active {
		t.Fatal("expected client-canceled unsubscribe to remove local subscription")
	}
	if _, err := subscription.Next(context.Background()); !errors.Is(err, ErrSubscriptionClosed) {
		t.Fatalf("expected locally closed subscription after client cancel, got %v", err)
	}
}

func TestWebSocketClientCloseIsConcurrentSafe(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, _ := newStartedTestWebSocketClient(t, dialer)

	done := make(chan error, 8)
	for range 8 {
		go func() {
			done <- client.Close()
		}()
	}
	for range 8 {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for concurrent Close")
		}
	}
}

func TestWebSocketClientCloseUnblocksSubscriptionAndWatcherConsumers(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)
	subscription, err := client.Subscribe(context.Background(), "messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t))
	watcher, err := client.WatchAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := watcher.Next(context.Background()); err != nil {
		t.Fatal(err)
	}

	subscriptionDone := make(chan error, 1)
	go func() {
		_, err := subscription.Next(context.Background())
		subscriptionDone <- err
	}()
	watcherDone := make(chan error, 1)
	go func() {
		_, err := watcher.Next(context.Background())
		watcherDone <- err
	}()

	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	for name, done := range map[string]chan error{
		"subscription": subscriptionDone,
		"watcher":      watcherDone,
	} {
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("expected %s consumer to unblock with context.Canceled, got %v", name, err)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for %s consumer to unblock", name)
		}
	}
}

func TestWebSocketClientWatchAllFailsAfterClientContextCanceled(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, _ := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)

	client.cancel()
	watcher, err := client.WatchAll(context.Background())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled WatchAll, got watcher=%#v err=%v", watcher, err)
	}
	if watcher != nil {
		t.Fatalf("WatchAll returned a watcher after client cancellation: %#v", watcher)
	}
}

func TestQuerySubscriptionNextPrefersClosedOverBufferedUpdate(t *testing.T) {
	for range 100 {
		subscription := &QuerySubscription{
			client:  &WebSocketClient{done: make(chan struct{})},
			updates: make(chan FunctionResult, 1),
			closed:  make(chan struct{}),
		}
		subscription.updates <- ValueResult(StringValue("stale"))
		close(subscription.closed)
		if _, err := subscription.Next(context.Background()); !errors.Is(err, ErrSubscriptionClosed) {
			t.Fatalf("expected closed subscription to win over buffered update, got %v", err)
		}
	}
}

func TestQuerySetSubscriptionNextPrefersClosedOverBufferedUpdate(t *testing.T) {
	for range 100 {
		watcher := &QuerySetSubscription{
			client:  &WebSocketClient{done: make(chan struct{})},
			updates: make(chan QueryResults, 1),
			closed:  make(chan struct{}),
		}
		watcher.updates <- QueryResults{}
		close(watcher.closed)
		if _, err := watcher.Next(context.Background()); !errors.Is(err, ErrSubscriptionClosed) {
			t.Fatalf("expected closed watcher to win over buffered update, got %v", err)
		}
	}
}

func TestQuerySetSubscriptionCloseIsConcurrentSafe(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, _ := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)
	watcher, err := client.WatchAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 8)
	for range 8 {
		go func() {
			done <- watcher.Close()
		}()
	}
	for range 8 {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for concurrent watcher Close")
		}
	}
}

func TestNewWebSocketClientNilContextAndInvalidURL(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, err := NewWebSocketClient(context.Background(), "https://happy-animal-123.convex.cloud",
		withWebSocketDialer(dialer),
		WithWebSocketReconnectBackoff(0),
		WithWebSocketInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = dialer.waitAttempt(t)
	closeTestWebSocketClient(t, client)
	if _, err := NewWebSocketClient(context.Background(), ""); err == nil {
		t.Fatal("expected invalid deployment URL error")
	}
}

type queryResult struct {
	value any
	err   error
}

type subscribeResult struct {
	subscription *QuerySubscription
	err          error
}

func waitClientLatestValue(t *testing.T, client *WebSocketClient, id SubscriberID, want string) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for latest value %q", want)
		default:
		}
		client.mu.Lock()
		result, ok := client.latest.Get(id)
		client.mu.Unlock()
		if ok {
			if value, ok := result.Value(); ok && value.GoValue() == want {
				return
			}
		}
		yieldTestLoop()
	}
}

func newStartedTestWebSocketClient(t *testing.T, dialer *fakeSyncDialer) (*WebSocketClient, fakeDialAttempt) {
	t.Helper()
	client, err := NewWebSocketClient(context.Background(), "https://happy-animal-123.convex.cloud",
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

func closeTestWebSocketClient(t *testing.T, client *WebSocketClient) {
	t.Helper()
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}

func assertNextValue(t *testing.T, ctx context.Context, subscription *QuerySubscription, want string) {
	t.Helper()
	result, err := subscription.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	value, ok := result.Value()
	if !ok || value.GoValue() != want {
		t.Fatalf("expected subscription value %q, got %#v", want, result)
	}
}

func assertNoWriteDuring(t *testing.T, conn *fakeSyncConn, operation func() error) {
	t.Helper()
	writeNumber, blocked, unblock := conn.blockNextWriteNumbered()
	done := make(chan error, 1)
	go func() {
		done <- operation()
	}()
	select {
	case err := <-done:
		conn.removeWriteBlock(writeNumber)
		if err != nil {
			t.Fatal(err)
		}
	case <-blocked:
		close(unblock)
		err := <-done
		t.Fatalf("unexpected write during operation: %v", err)
	case <-time.After(time.Second):
		conn.removeWriteBlock(writeNumber)
		t.Fatal("operation did not complete")
	}
}

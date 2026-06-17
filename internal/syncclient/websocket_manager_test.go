package syncclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	coderwebsocket "github.com/coder/websocket"

	"github.com/cervantesh/convex-go/baseclient"
	"github.com/cervantesh/convex-go/internal/core"
	. "github.com/cervantesh/convex-go/internal/syncprotocol"
)

func TestA_ManagerDefaultConfiguration(t *testing.T) {
	manager, err := New("https://happy-animal-123.convex.cloud")
	if err != nil {
		t.Fatal(err)
	}
	if manager.clientID != "go-0.1.0" {
		t.Fatalf("default client ID = %q", manager.clientID)
	}
	if manager.reconnectBackoff != 100*time.Millisecond {
		t.Fatalf("default reconnect backoff = %s", manager.reconnectBackoff)
	}
	if manager.inactivityTimeout != 30*time.Second {
		t.Fatalf("default inactivity timeout = %s", manager.inactivityTimeout)
	}
	if cap(manager.results) != 16 {
		t.Fatalf("results channel capacity = %d", cap(manager.results))
	}
	if cap(manager.flushCh) != 16 {
		t.Fatalf("flush channel capacity = %d", cap(manager.flushCh))
	}
	if errInactiveServer.Error() != "convex: inactive server" {
		t.Fatalf("inactive server error changed: %v", errInactiveServer)
	}
}

func TestA_RunConnectionDoesNotExitAfterInitialConnect(t *testing.T) {
	manager, err := New("https://happy-animal-123.convex.cloud", WithInactivityTimeout(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	conn := newFakeSyncConn()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- manager.runConnection(ctx, conn, "session", 0, "InitialConnect", nil)
	}()
	_ = decodeSentClientMessage[ConnectMessage](t, conn.waitSent(t))
	select {
	case err := <-done:
		t.Fatalf("runConnection exited after initial connect: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation after stopping connection, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for runConnection cancellation")
	}
}

func TestA_WriteLoopAcknowledgesFlushRequest(t *testing.T) {
	manager, err := New("https://happy-animal-123.convex.cloud")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	requests := make(chan flushRequest, 1)
	errCh := make(chan writeResult, 1)
	go manager.writeLoop(ctx, newFakeSyncConn(), requests, errCh)
	ack := make(chan error, 1)
	requests <- flushRequest{ack: ack, replay: true}
	select {
	case err := <-ack:
		if err != nil {
			t.Fatalf("unexpected flush ack error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for writeLoop flush ack")
	}
	select {
	case result := <-errCh:
		t.Fatalf("writeLoop reported unexpected error: %#v", result)
	default:
	}
}

func TestA_PrepareReconnectReturnsCancellationWhenRestartFails(t *testing.T) {
	manager, err := New("https://happy-animal-123.convex.cloud", WithReconnectBackoff(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.SetAuthCallback(func(forceRefresh bool) (baseclient.AuthToken, error) {
		if forceRefresh {
			return baseclient.AuthToken{}, errFakeDialFailed
		}
		return baseclient.UserAuthToken("initial-token"), nil
	}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() {
		done <- manager.prepareReconnect(ctx)
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected canceled reconnect preparation, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for prepareReconnect cancellation")
	}
}

func TestA_RunConnectionInitialFlushWriteErrorRequestsReplay(t *testing.T) {
	manager, err := New("https://happy-animal-123.convex.cloud", WithInactivityTimeout(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Subscribe("messages:list", nil); err != nil {
		t.Fatal(err)
	}
	conn := newFakeSyncConn()
	conn.failWrite(2, errFakeSocketClosed)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- manager.runConnection(ctx, conn, "session", 0, "InitialConnect", nil)
	}()
	_ = decodeSentClientMessage[ConnectMessage](t, conn.waitSent(t))

	reconnect := waitReconnectError(t, done)
	if !errors.Is(reconnect, errFakeSocketClosed) {
		t.Fatalf("expected socket error, got %v", reconnect)
	}
	if !reconnect.replay {
		t.Fatal("initial flush write failure must request replay")
	}
}

func TestA_RunConnectionManualFlushWriteErrorRequestsReplay(t *testing.T) {
	manager, err := New("https://happy-animal-123.convex.cloud", WithInactivityTimeout(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	conn := newFakeSyncConn()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- manager.runConnection(ctx, conn, "session", 0, "InitialConnect", nil)
	}()
	_ = decodeSentClientMessage[ConnectMessage](t, conn.waitSent(t))

	conn.failWrite(2, errFakeSocketClosed)
	if _, err := manager.Mutation("messages:send", nil); err != nil {
		t.Fatal(err)
	}
	if err := manager.Flush(ctx); !errors.Is(err, errFakeSocketClosed) {
		t.Fatalf("expected flush write error, got %v", err)
	}
	reconnect := waitReconnectError(t, done)
	if !reconnect.replay {
		t.Fatal("manual flush write failure must request replay")
	}
}

func TestA_RunConnectionInactivityRequestsReplay(t *testing.T) {
	manager, err := New("https://happy-animal-123.convex.cloud", WithInactivityTimeout(10*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	conn := newFakeSyncConn()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- manager.runConnection(ctx, conn, "session", 0, "InitialConnect", nil)
	}()
	_ = decodeSentClientMessage[ConnectMessage](t, conn.waitSent(t))

	reconnect := waitReconnectError(t, done)
	if !errors.Is(reconnect, errInactiveServer) {
		t.Fatalf("expected inactive server reconnect, got %v", reconnect)
	}
	if !reconnect.replay {
		t.Fatal("inactivity reconnect must request replay")
	}
}

func TestManagerConnectsAndSendsConnect(t *testing.T) {
	dialer := newFakeSyncDialer()
	manager, err := New("https://happy-animal-123.convex.cloud",
		WithClientID("go-test"),
		WithDialer(dialer),
		WithReconnectBackoff(0),
		WithInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runManager(t, ctx, manager)

	attempt := dialer.waitAttempt(t)
	if attempt.url != "wss://happy-animal-123.convex.cloud/api/sync" {
		t.Fatalf("unexpected sync url: %s", attempt.url)
	}
	if got := attempt.header.Get("Convex-Client"); got != "go-test" {
		t.Fatalf("unexpected Convex-Client header: %q", got)
	}
	connect := decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
	if connect.SessionID == "" {
		t.Fatal("expected connect session id")
	}
	if connect.ConnectionCount != 0 || connect.LastCloseReason != "InitialConnect" || connect.ClientTS == nil {
		t.Fatalf("unexpected connect message: %#v", connect)
	}

	cancel()
	waitManagerDone(t, done)
}

func TestManagerFlushesOutgoingAndAppliesIncoming(t *testing.T) {
	dialer := newFakeSyncDialer()
	manager, err := New("http://happy-animal-123.convex.cloud",
		WithDialer(dialer),
		WithReconnectBackoff(0),
		WithInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runManager(t, ctx, manager)
	attempt := dialer.waitAttempt(t)
	_ = attempt.conn.waitSent(t)

	subscriber, err := manager.Subscribe("messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	add := decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t))
	queryID := onlyQuerySetAdd(t, add).QueryID

	attempt.conn.receive(t, TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{QueryID: queryID, Value: core.StringValue("hello"), LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	})
	results := waitQueryResults(t, manager)
	result, ok := results.Get(subscriber)
	if !ok {
		t.Fatal("expected subscriber result")
	}
	value, ok := result.Value()
	if !ok || value.GoValue() != "hello" {
		t.Fatalf("unexpected result: %#v", result)
	}

	cancel()
	waitManagerDone(t, done)
}

func TestManagerReconnectsAndReplaysState(t *testing.T) {
	dialer := newFakeSyncDialer()
	manager, err := New("wss://happy-animal-123.convex.cloud",
		WithDialer(dialer),
		WithReconnectBackoff(0),
		WithInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.SetAuthCallback(func(forceRefresh bool) (baseclient.AuthToken, error) {
		if forceRefresh {
			return baseclient.UserAuthToken("fresh"), nil
		}
		return baseclient.UserAuthToken("initial"), nil
	}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runManager(t, ctx, manager)
	first := dialer.waitAttempt(t)
	_ = first.conn.waitSent(t)
	_ = first.conn.waitSent(t) // initial auth
	if _, err := manager.Subscribe("messages:list", nil); err != nil {
		t.Fatal(err)
	}
	mutationID, err := manager.Mutation("messages:send", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	add := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, first.conn.waitSent(t)))
	queryID := add.QueryID
	_ = first.conn.waitSent(t) // mutation
	first.conn.closeWithError(errFakeSocketClosed)

	second := dialer.waitAttempt(t)
	reconnect := decodeSentClientMessage[ConnectMessage](t, second.conn.waitSent(t))
	if reconnect.ConnectionCount != 1 || reconnect.LastCloseReason == "InitialConnect" {
		t.Fatalf("unexpected reconnect message: %#v", reconnect)
	}
	auth := decodeSentClientMessage[AuthenticateMessage](t, second.conn.waitSent(t))
	if auth.Value != "fresh" || auth.BaseVersion != 0 {
		t.Fatalf("unexpected replay auth: %#v", auth)
	}
	replayedAdd := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, second.conn.waitSent(t)))
	if replayedAdd.QueryID != queryID {
		t.Fatalf("unexpected replay query: %#v", replayedAdd)
	}
	mutation := decodeSentClientMessage[MutationMessage](t, second.conn.waitSent(t))
	if mutation.RequestID != mutationID {
		t.Fatalf("unexpected replay mutation: %#v", mutation)
	}

	cancel()
	waitManagerDone(t, done)
}

func TestManagerPingAndInactivity(t *testing.T) {
	dialer := newFakeSyncDialer()
	manager, err := New("https://happy-animal-123.convex.cloud",
		WithDialer(dialer),
		WithReconnectBackoff(0),
		WithInactivityTimeout(20*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runManager(t, ctx, manager)
	first := dialer.waitAttempt(t)
	_ = first.conn.waitSent(t)
	first.conn.receive(t, PingMessage{})
	second := dialer.waitAttempt(t)
	reconnect := decodeSentClientMessage[ConnectMessage](t, second.conn.waitSent(t))
	if reconnect.ConnectionCount != 1 {
		t.Fatalf("expected reconnect after inactivity, got %#v", reconnect)
	}

	cancel()
	waitManagerDone(t, done)
}

func TestManagerInitialDialFailureDoesNotCancelQueuedAction(t *testing.T) {
	dialer := newFakeSyncDialer()
	dialer.failNext(errFakeDialFailed)
	manager, err := New("https://happy-animal-123.convex.cloud",
		WithDialer(dialer),
		WithReconnectBackoff(0),
		WithInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	actionID, err := manager.Action("jobs:run", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runManager(t, ctx, manager)

	attempt := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
	action := decodeSentClientMessage[ActionMessage](t, attempt.conn.waitSent(t))
	if action.RequestID != actionID {
		t.Fatalf("expected initially queued action to send after retry, got %#v", action)
	}

	cancel()
	waitManagerDone(t, done)
}

func TestManagerProcessesIncomingWhileFlushWriteBlocks(t *testing.T) {
	dialer := newFakeSyncDialer()
	manager, err := New("https://happy-animal-123.convex.cloud",
		WithDialer(dialer),
		WithReconnectBackoff(0),
		WithInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runManager(t, ctx, manager)
	attempt := dialer.waitAttempt(t)
	_ = attempt.conn.waitSent(t)

	subscriber, err := manager.Subscribe("messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	add := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
	queryID := add.QueryID
	if _, err := manager.Mutation("messages:send", nil); err != nil {
		t.Fatal(err)
	}
	blocked, unblock := attempt.conn.blockNextWrite()
	flushDone := make(chan error, 1)
	go func() {
		flushDone <- manager.Flush(ctx)
	}()
	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for blocked write")
	}
	attempt.conn.receive(t, TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{QueryID: queryID, Value: core.StringValue("while-write-blocked"), LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	})
	results := waitQueryResults(t, manager)
	result, ok := results.Get(subscriber)
	if !ok {
		t.Fatal("expected result while write is blocked")
	}
	value, ok := result.Value()
	if !ok || value.GoValue() != "while-write-blocked" {
		t.Fatalf("unexpected result while write is blocked: %#v", result)
	}

	close(unblock)
	if err := <-flushDone; err != nil {
		t.Fatal(err)
	}
	_ = decodeSentClientMessage[MutationMessage](t, attempt.conn.waitSent(t))
	cancel()
	waitManagerDone(t, done)
}

func TestManagerProcessesIncomingWhileInitialFlushWriteBlocks(t *testing.T) {
	dialer := newFakeSyncDialer()
	manager, err := New("https://happy-animal-123.convex.cloud",
		WithDialer(dialer),
		WithReconnectBackoff(0),
		WithInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	subscriber, err := manager.Subscribe("messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Mutation("messages:send", nil); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var blocked <-chan struct{}
	var unblock chan<- struct{}
	dialer.configureNextConn(func(conn *fakeSyncConn) {
		blocked, unblock = conn.blockWrite(3)
	})
	done := runManager(t, ctx, manager)
	attempt := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
	add := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
	queryID := add.QueryID
	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for blocked initial mutation write")
	}

	attempt.conn.receive(t, TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{QueryID: queryID, Value: core.StringValue("initial-write-blocked"), LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	})
	results := waitQueryResults(t, manager)
	result, ok := results.Get(subscriber)
	if !ok {
		t.Fatal("expected result while initial write is blocked")
	}
	value, ok := result.Value()
	if !ok || value.GoValue() != "initial-write-blocked" {
		t.Fatalf("unexpected result while initial write is blocked: %#v", result)
	}

	close(unblock)
	_ = decodeSentClientMessage[MutationMessage](t, attempt.conn.waitSent(t))
	cancel()
	waitManagerDone(t, done)
}

func TestManagerInitialFlushWriteFailureReplaysState(t *testing.T) {
	dialer := newFakeSyncDialer()
	manager, err := New("https://happy-animal-123.convex.cloud",
		WithDialer(dialer),
		WithReconnectBackoff(0),
		WithInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Subscribe("messages:list", nil); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var blocked <-chan struct{}
	dialer.configureNextConn(func(conn *fakeSyncConn) {
		blocked, _ = conn.blockWrite(2)
	})
	done := runManager(t, ctx, manager)
	first := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, first.conn.waitSent(t))
	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for blocked initial flush write")
	}
	first.conn.closeWithError(errFakeSocketClosed)

	second := dialer.waitAttempt(t)
	reconnect := decodeSentClientMessage[ConnectMessage](t, second.conn.waitSent(t))
	if reconnect.ConnectionCount != 1 {
		t.Fatalf("expected reconnect after initial flush failure, got %#v", reconnect)
	}
	add := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, second.conn.waitSent(t)))
	if add.QueryID != 0 || add.UDFPath != "messages:list" {
		t.Fatalf("unexpected replayed query add: %#v", add)
	}

	cancel()
	waitManagerDone(t, done)
}

func TestManagerFatalErrorStopsRun(t *testing.T) {
	dialer := newFakeSyncDialer()
	manager, err := New("https://happy-animal-123.convex.cloud",
		WithDialer(dialer),
		WithReconnectBackoff(0),
		WithInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runManager(t, ctx, manager)
	attempt := dialer.waitAttempt(t)
	_ = attempt.conn.waitSent(t)
	attempt.conn.receive(t, FatalErrorMessage{Error: "fatal"})

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "fatal") {
			t.Fatalf("expected fatal error, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for fatal error")
	}
}

func TestOptionsAndAuthMutators(t *testing.T) {
	base := baseclient.New()
	manager, err := New("https://happy-animal-123.convex.cloud",
		WithBaseClient(base),
		WithDialer(newFakeSyncDialer()),
	)
	if err != nil {
		t.Fatal(err)
	}
	if manager.client != base {
		t.Fatal("expected custom base client")
	}

	if err := manager.SetAuth("user-token"); err != nil {
		t.Fatal(err)
	}
	auth := popAuthenticateMessage(t, base)
	if auth.TokenType != baseclient.AuthTokenUser || auth.Value != "user-token" {
		t.Fatalf("unexpected user auth message: %#v", auth)
	}
	if err := manager.SetAdminAuth("admin-token", SyncUserIdentityAttributes{Issuer: "issuer", Subject: "subject"}); err != nil {
		t.Fatal(err)
	}
	admin := popAuthenticateMessage(t, base)
	if admin.TokenType != baseclient.AuthTokenAdmin || admin.ActingAs.TokenIdentifier != "issuer|subject" {
		t.Fatalf("unexpected admin auth message: %#v", admin)
	}
	if err := manager.ClearAuth(); err != nil {
		t.Fatal(err)
	}
	clear := popAuthenticateMessage(t, base)
	if clear.TokenType != baseclient.AuthTokenNone {
		t.Fatalf("unexpected clear auth message: %#v", clear)
	}
}

func TestManagerMutationPublishesOptimisticResults(t *testing.T) {
	base := baseclient.New()
	manager, err := New("https://happy-animal-123.convex.cloud",
		WithBaseClient(base),
		WithDialer(newFakeSyncDialer()),
	)
	if err != nil {
		t.Fatal(err)
	}
	subscriber, err := manager.Subscribe("messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	add := onlyQuerySetAdd(t, base.PopNextMessage().(ModifyQuerySetMessage))
	if _, err := base.ReceiveMessage(TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{QueryID: add.QueryID, Value: core.StringValue("server"), LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := manager.Mutation("messages:send", nil, baseclient.WithOptimisticUpdate(func(store *baseclient.OptimisticLocalStore) error {
		return store.SetQuery("messages:list", nil, core.StringValue("optimistic"))
	})); err != nil {
		t.Fatal(err)
	}

	results := waitQueryResults(t, manager)
	result, ok := results.Get(subscriber)
	if !ok {
		t.Fatal("expected optimistic result")
	}
	value, ok := result.Value()
	if !ok || value.GoValue() != "optimistic" {
		t.Fatalf("unexpected optimistic result: %#v", result)
	}
}

func TestManagerLatestResultsAndUnsubscribe(t *testing.T) {
	base := baseclient.New()
	manager, err := New("https://happy-animal-123.convex.cloud",
		WithBaseClient(base),
		WithDialer(newFakeSyncDialer()),
	)
	if err != nil {
		t.Fatal(err)
	}
	subscriber, err := manager.Subscribe("messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	add := onlyQuerySetAdd(t, base.PopNextMessage().(ModifyQuerySetMessage))
	if _, err := base.ReceiveMessage(TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{QueryID: add.QueryID, Value: core.StringValue("server"), LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if result, ok := manager.LatestResults().Get(subscriber); !ok || !result.IsValue() {
		t.Fatalf("expected latest result, got %#v ok=%v", result, ok)
	}
	if err := manager.Unsubscribe(subscriber); err != nil {
		t.Fatal(err)
	}
	if !manager.LatestResults().IsEmpty() {
		t.Fatalf("expected empty latest results after unsubscribe: %#v", manager.LatestResults().Iter())
	}
}

func TestManagerRejectsInvalidOptions(t *testing.T) {
	invalidOptions := []struct {
		name    string
		opt     Option
		wantErr string
	}{
		{name: "nil base client", opt: WithBaseClient(nil), wantErr: "convex: nil base client"},
		{name: "empty client ID", opt: WithClientID(" "), wantErr: "convex: client ID cannot be empty"},
		{name: "nil dialer", opt: WithDialer(nil), wantErr: "convex: nil websocket dialer"},
		{name: "negative reconnect backoff", opt: WithReconnectBackoff(-time.Nanosecond), wantErr: "convex: reconnect backoff cannot be negative"},
		{name: "zero inactivity timeout", opt: WithInactivityTimeout(0), wantErr: "convex: inactivity timeout must be positive"},
	}
	for _, tt := range invalidOptions {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New("https://happy-animal-123.convex.cloud", tt.opt); err == nil {
				t.Fatal("expected option to fail")
			} else if err.Error() != tt.wantErr {
				t.Fatalf("unexpected option error: %v", err)
			}
		})
	}
	if _, err := New("https://happy-animal-123.convex.cloud", WithInactivityTimeout(time.Nanosecond)); err != nil {
		t.Fatalf("positive inactivity timeout should be accepted: %v", err)
	}
	if _, err := New(""); err == nil {
		t.Fatal("expected empty deployment URL error")
	} else if err.Error() != "convex: deployment URL cannot be empty" {
		t.Fatalf("unexpected empty deployment URL error: %v", err)
	}
	if _, err := WebSocketURL("https:///missing-host"); err == nil {
		t.Fatal("expected missing host error")
	} else if err.Error() != "convex: deployment URL must include a host" {
		t.Fatalf("unexpected missing host error: %v", err)
	}
}

func TestManagerRejectsConcurrentRun(t *testing.T) {
	dialer := newFakeSyncDialer()
	manager, err := New("https://happy-animal-123.convex.cloud",
		WithDialer(dialer),
		WithReconnectBackoff(0),
		WithInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runManager(t, ctx, manager)
	_ = dialer.waitAttempt(t)

	secondDone := make(chan error, 1)
	go func() {
		secondDone <- manager.Run(context.Background())
	}()
	select {
	case err := <-secondDone:
		if err == nil || !strings.Contains(err.Error(), "already running") {
			t.Fatalf("expected concurrent Run rejection, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for concurrent Run rejection")
	}

	cancel()
	waitManagerDone(t, done)
}

func TestManagerRunCancelDuringDialBackoff(t *testing.T) {
	dialer := newFakeSyncDialer()
	dialer.failNext(errFakeDialFailed)
	manager, err := New("https://happy-animal-123.convex.cloud",
		WithDialer(dialer),
		WithReconnectBackoff(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := runManager(t, ctx, manager)
	cancel()
	waitManagerDone(t, done)
}

func TestManagerBackoffDelaysRedialAfterDialFailure(t *testing.T) {
	dialer := newFakeSyncDialer()
	dialer.failNext(errFakeDialFailed)
	manager, err := New("https://happy-animal-123.convex.cloud",
		WithDialer(dialer),
		WithReconnectBackoff(75*time.Millisecond),
		WithInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runManager(t, ctx, manager)

	select {
	case attempt := <-dialer.attempts:
		t.Fatalf("redial happened before backoff elapsed: %#v", attempt)
	case <-time.After(20 * time.Millisecond):
	}

	attempt := dialer.waitAttempt(t)
	connect := decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))
	if connect.ConnectionCount != 1 || connect.LastCloseReason != errFakeDialFailed.Error() {
		t.Fatalf("unexpected post-backoff connect: %#v", connect)
	}

	cancel()
	waitManagerDone(t, done)
}

func TestManagerFlushReturnsWhenRunContextCanceled(t *testing.T) {
	dialer := newFakeSyncDialer()
	manager, err := New("https://happy-animal-123.convex.cloud",
		WithDialer(dialer),
		WithReconnectBackoff(0),
		WithInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := runManager(t, ctx, manager)
	_ = dialer.waitAttempt(t)
	cancel()
	waitManagerDone(t, done)

	flushDone := make(chan error, 1)
	go func() {
		flushDone <- manager.Flush(context.Background())
	}()
	select {
	case err := <-flushDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected canceled flush after Run exit, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Flush after Run exit")
	}
}

func TestManagerInternalLifecycleHelpers(t *testing.T) {
	manager, err := New("https://happy-animal-123.convex.cloud", WithReconnectBackoff(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(manager.doneErr(), context.Canceled) {
		t.Fatalf("doneErr without run error should be context canceled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := manager.prepareReconnect(ctx); err != nil {
		t.Fatalf("prepareReconnect should return after successful restart: %v", err)
	}
}

func TestManagerAcksAllPendingFlushesOnWriteError(t *testing.T) {
	dialer := newFakeSyncDialer()
	manager, err := New("https://happy-animal-123.convex.cloud",
		WithDialer(dialer),
		WithReconnectBackoff(0),
		WithInactivityTimeout(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runManager(t, ctx, manager)
	attempt := dialer.waitAttempt(t)
	_ = decodeSentClientMessage[ConnectMessage](t, attempt.conn.waitSent(t))

	if _, err := manager.Subscribe("messages:list", nil); err != nil {
		t.Fatal(err)
	}
	blocked, _ := attempt.conn.blockNextWrite()
	flushCtx, cancelFlush := context.WithTimeout(context.Background(), time.Second)
	defer cancelFlush()
	flushDone := make(chan error, 3)
	for range 3 {
		go func() {
			flushDone <- manager.Flush(flushCtx)
		}()
	}
	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for blocked flush write")
	}
	time.Sleep(20 * time.Millisecond)
	attempt.conn.closeWithError(errFakeSocketClosed)

	for range 3 {
		select {
		case err := <-flushDone:
			if err == nil {
				t.Fatal("expected flush error after write failure")
			}
			if errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("flush waited for caller deadline instead of write failure: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for pending flush ACK")
		}
	}

	cancel()
	waitManagerDone(t, done)
}

func TestWebSocketHelpers(t *testing.T) {
	if err := sleepContext(context.Background(), 0); err != nil {
		t.Fatalf("zero sleep should return nil without canceled context: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleepContext(ctx, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("zero sleep should respect canceled context, got %v", err)
	}
	if err := sleepContext(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled sleep, got %v", err)
	}
	wrapped := reconnectableWebSocketError{err: errFakeSocketClosed}
	if !errors.Is(wrapped, errFakeSocketClosed) {
		t.Fatalf("expected reconnectable error to unwrap: %v", wrapped)
	}
	if (reconnectableWebSocketError{}).Error() == "" {
		t.Fatal("expected fallback reconnectable error string")
	}
}

func TestReadLoopContinuesAfterSuccessfulRead(t *testing.T) {
	conn := newFakeSyncConn()
	out := make(chan readResult, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go readLoop(ctx, conn, out)

	conn.incoming <- []byte("first")
	conn.incoming <- []byte("second")

	first := waitReadResult(t, out)
	if string(first.data) != "first" || first.err != nil {
		t.Fatalf("unexpected first read: %#v", first)
	}
	second := waitReadResult(t, out)
	if string(second.data) != "second" || second.err != nil {
		t.Fatalf("readLoop exited after successful read: %#v", second)
	}
}

func TestFormatSyncSessionIDSetsVersionAndVariant(t *testing.T) {
	tests := []struct {
		name  string
		bytes [16]byte
		want  string
	}{
		{
			name: "zero bytes",
			want: "00000000-0000-4000-8000-000000000000",
		},
		{
			name: "one bits",
			bytes: [16]byte{
				0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff,
			},
			want: "ffffffff-ffff-4fff-bfff-ffffffffffff",
		},
		{
			name: "mixed bytes",
			bytes: [16]byte{
				0x00, 0x11, 0x22, 0x33,
				0x44, 0x55, 0x66, 0x77,
				0x88, 0x99, 0xaa, 0xbb,
				0xcc, 0xdd, 0xee, 0xff,
			},
			want: "00112233-4455-4677-8899-aabbccddeeff",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSyncSessionID(tt.bytes); got != tt.want {
				t.Fatalf("formatSyncSessionID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewSyncSessionIDFormatAndVersion(t *testing.T) {
	for range 20 {
		sessionID, err := newSyncSessionID()
		if err != nil {
			t.Fatal(err)
		}
		if len(sessionID) != 36 {
			t.Fatalf("session ID length = %d for %q", len(sessionID), sessionID)
		}
		for _, index := range []int{8, 13, 18, 23} {
			if sessionID[index] != '-' {
				t.Fatalf("session ID missing hyphen at %d: %q", index, sessionID)
			}
		}
		if sessionID[14] != '4' {
			t.Fatalf("session ID version nibble = %q in %q", sessionID[14], sessionID)
		}
		if !strings.ContainsRune("89ab", rune(sessionID[19])) {
			t.Fatalf("session ID variant nibble = %q in %q", sessionID[19], sessionID)
		}
		for i, r := range sessionID {
			if r == '-' {
				continue
			}
			if !strings.ContainsRune("0123456789abcdef", r) {
				t.Fatalf("session ID has non-hex rune at %d: %q in %q", i, r, sessionID)
			}
		}
	}
}

func TestCoderSyncDialerAndConn(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderwebsocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept websocket: %v", err)
			return
		}
		defer func() { _ = conn.Close(coderwebsocket.StatusNormalClosure, "") }()
		_, data, err := conn.Read(r.Context())
		if err != nil {
			t.Errorf("read websocket: %v", err)
			return
		}
		if string(data) != "hello" {
			t.Errorf("unexpected websocket payload: %q", data)
			return
		}
		if err := conn.Write(r.Context(), coderwebsocket.MessageText, []byte("world")); err != nil {
			t.Errorf("write websocket: %v", err)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := coderSyncDialer{}.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), http.Header{"X-Test": []string{"yes"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Write(ctx, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	data, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "world" {
		t.Fatalf("unexpected websocket response: %q", data)
	}
	if err := conn.Close(nil); err != nil {
		t.Fatal(err)
	}
}

var errFakeSocketClosed = errors.New("fake socket closed")
var errFakeDialFailed = errors.New("fake dial failed")

type fakeSyncDialer struct {
	attempts chan fakeDialAttempt
	failures chan error
	configs  chan func(*fakeSyncConn)
}

type fakeDialAttempt struct {
	url    string
	header http.Header
	conn   *fakeSyncConn
}

func newFakeSyncDialer() *fakeSyncDialer {
	return &fakeSyncDialer{
		attempts: make(chan fakeDialAttempt, 16),
		failures: make(chan error, 16),
		configs:  make(chan func(*fakeSyncConn), 16),
	}
}

func (d *fakeSyncDialer) failNext(err error) {
	d.failures <- err
}

func (d *fakeSyncDialer) configureNextConn(configure func(*fakeSyncConn)) {
	d.configs <- configure
}

func (d *fakeSyncDialer) Dial(ctx context.Context, url string, header http.Header) (Conn, error) {
	select {
	case err := <-d.failures:
		return nil, err
	default:
	}
	conn := newFakeSyncConn()
	select {
	case configure := <-d.configs:
		configure(conn)
	default:
	}
	copied := header.Clone()
	select {
	case d.attempts <- fakeDialAttempt{url: url, header: copied, conn: conn}:
		return conn, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (d *fakeSyncDialer) waitAttempt(t *testing.T) fakeDialAttempt {
	t.Helper()
	select {
	case attempt := <-d.attempts:
		return attempt
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for dial attempt")
		return fakeDialAttempt{}
	}
}

type fakeSyncConn struct {
	incoming    chan []byte
	sent        chan []byte
	closed      chan error
	mu          sync.Mutex
	writeCount  int
	writeBlocks map[int]writeBlock
	writeErrors map[int]error
}

type writeBlock struct {
	blocked chan struct{}
	unblock chan struct{}
}

func newFakeSyncConn() *fakeSyncConn {
	return &fakeSyncConn{
		incoming:    make(chan []byte, 16),
		sent:        make(chan []byte, 16),
		closed:      make(chan error, 1),
		writeBlocks: make(map[int]writeBlock),
		writeErrors: make(map[int]error),
	}
}

func (c *fakeSyncConn) Read(ctx context.Context) ([]byte, error) {
	select {
	case data := <-c.incoming:
		return data, nil
	case err := <-c.closed:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *fakeSyncConn) Write(ctx context.Context, data []byte) error {
	c.mu.Lock()
	c.writeCount++
	block, shouldBlock := c.writeBlocks[c.writeCount]
	if shouldBlock {
		delete(c.writeBlocks, c.writeCount)
	}
	writeErr := c.writeErrors[c.writeCount]
	if writeErr != nil {
		delete(c.writeErrors, c.writeCount)
	}
	c.mu.Unlock()
	if shouldBlock {
		close(block.blocked)
		select {
		case <-block.unblock:
		case err := <-c.closed:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if writeErr != nil {
		return writeErr
	}
	copied := append([]byte(nil), data...)
	select {
	case c.sent <- copied:
		return nil
	case err := <-c.closed:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *fakeSyncConn) Close(error) error {
	select {
	case c.closed <- context.Canceled:
	default:
	}
	return nil
}

func (c *fakeSyncConn) waitSent(t *testing.T) []byte {
	t.Helper()
	select {
	case data := <-c.sent:
		return data
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for sent message")
		return nil
	}
}

func (c *fakeSyncConn) blockNextWrite() (<-chan struct{}, chan<- struct{}) {
	_, blocked, unblock := c.blockNextWriteNumbered()
	return blocked, unblock
}

func (c *fakeSyncConn) blockNextWriteNumbered() (int, <-chan struct{}, chan<- struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	writeNumber := c.writeCount + 1
	blocked, unblock := c.blockWriteLocked(writeNumber)
	return writeNumber, blocked, unblock
}

func (c *fakeSyncConn) blockWrite(writeNumber int) (<-chan struct{}, chan<- struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.blockWriteLocked(writeNumber)
}

func (c *fakeSyncConn) failWrite(writeNumber int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writeErrors[writeNumber] = err
}

func (c *fakeSyncConn) blockWriteLocked(writeNumber int) (<-chan struct{}, chan<- struct{}) {
	block := writeBlock{
		blocked: make(chan struct{}),
		unblock: make(chan struct{}),
	}
	c.writeBlocks[writeNumber] = block
	return block.blocked, block.unblock
}

func (c *fakeSyncConn) receive(t *testing.T, msg ServerMessage) {
	t.Helper()
	data, err := EncodeServerMessage(msg)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case c.incoming <- data:
	case <-time.After(time.Second):
		t.Fatal("timed out queueing server message")
	}
}

func (c *fakeSyncConn) closeWithError(err error) {
	select {
	case c.closed <- err:
	default:
	}
}

func runManager(t *testing.T, ctx context.Context, manager *Manager) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- manager.Run(ctx)
	}()
	return done
}

func waitManagerDone(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("manager returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for manager shutdown")
	}
}

func waitReadResult(t *testing.T, out <-chan readResult) readResult {
	t.Helper()
	select {
	case result := <-out:
		return result
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for read result")
		return readResult{}
	}
}

func waitReconnectError(t *testing.T, done <-chan error) reconnectableWebSocketError {
	t.Helper()
	select {
	case err := <-done:
		var reconnect reconnectableWebSocketError
		if !errors.As(err, &reconnect) {
			t.Fatalf("expected reconnectable websocket error, got %v", err)
		}
		return reconnect
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for reconnectable websocket error")
		return reconnectableWebSocketError{}
	}
}

func decodeSentClientMessage[T ClientMessage](t *testing.T, data []byte) T {
	t.Helper()
	msg, err := DecodeClientMessage(data)
	if err != nil {
		t.Fatal(err)
	}
	typed, ok := msg.(T)
	if !ok {
		t.Fatalf("unexpected client message type %T: %s", msg, data)
	}
	return typed
}

func onlyQuerySetAdd(t *testing.T, msg ModifyQuerySetMessage) QuerySetAdd {
	t.Helper()
	if len(msg.Modifications) != 1 {
		t.Fatalf("expected one query set modification, got %#v", msg.Modifications)
	}
	add, ok := msg.Modifications[0].(QuerySetAdd)
	if !ok {
		t.Fatalf("expected QuerySetAdd, got %T %#v", msg.Modifications[0], msg.Modifications[0])
	}
	return add
}

func popAuthenticateMessage(t *testing.T, client *baseclient.Client) AuthenticateMessage {
	t.Helper()
	msg := client.PopNextMessage()
	auth, ok := msg.(AuthenticateMessage)
	if !ok {
		t.Fatalf("expected AuthenticateMessage, got %T %#v", msg, msg)
	}
	return auth
}

func waitQueryResults(t *testing.T, manager *Manager) baseclient.QueryResults {
	t.Helper()
	select {
	case results := <-manager.Results():
		return results
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for query results")
		return baseclient.QueryResults{}
	}
}

func TestWebSocketURL(t *testing.T) {
	tests := map[string]string{
		"http://flying-shark-123.convex.cloud":     "ws://flying-shark-123.convex.cloud/api/sync",
		"https://flying-shark-123.convex.cloud":    "wss://flying-shark-123.convex.cloud/api/sync",
		"ws://flying-shark-123.convex.cloud":       "ws://flying-shark-123.convex.cloud/api/sync",
		"wss://flying-shark-123.convex.cloud":      "wss://flying-shark-123.convex.cloud/api/sync",
		"https://flying-shark-123.convex.cloud///": "wss://flying-shark-123.convex.cloud/api/sync",
	}
	for input, want := range tests {
		got, err := WebSocketURL(input)
		if err != nil {
			t.Fatalf("WebSocketURL(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("WebSocketURL(%q) = %q, want %q", input, got, want)
		}
	}
	if _, err := WebSocketURL("ftp://flying-shark-123.convex.cloud"); err == nil {
		t.Fatal("expected unsupported scheme error")
	}
}

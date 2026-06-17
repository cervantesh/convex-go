package baseclient

import (
	"errors"
	"testing"
)

var errTestReconnectAuth = errors.New("refresh failed")

func TestReconnectReplayClearsStaleMessages(t *testing.T) {
	client := New()
	if _, err := client.Subscribe("messages:list", nil); err != nil {
		t.Fatal(err)
	}
	_ = popModifyQuerySet(t, client)

	if err := client.RestartForReconnect(); err != nil {
		t.Fatal(err)
	}
	firstReplay := popModifyQuerySet(t, client)
	if firstReplay.BaseVersion != 0 || firstReplay.NewVersion != 1 {
		t.Fatalf("unexpected first replay versions: %#v", firstReplay)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("expected first replay queue drained, got %#v", msg)
	}

	if err := client.RestartForReconnect(); err != nil {
		t.Fatal(err)
	}
	// Simulate a failed reconnect that sends only part of the rebuilt queue.
	_ = client.PopNextMessage()
	if err := client.RestartForReconnect(); err != nil {
		t.Fatal(err)
	}
	finalReplay := popModifyQuerySet(t, client)
	if finalReplay.BaseVersion != 0 || finalReplay.NewVersion != 1 {
		t.Fatalf("stale versions leaked into replay: %#v", finalReplay)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("expected stale queue cleared, got %#v", msg)
	}
}

func TestReconnectReplayOrdersAuthQueriesAndMutations(t *testing.T) {
	client := New()
	calls := setRecordingAuthCallback(t, client)
	_ = popAuthenticateMessage(t, client)
	_, queryID, mutationID, actionID := queueReconnectFixture(t, client)

	if err := client.RestartForReconnect(); err != nil {
		t.Fatal(err)
	}
	assertAuthRefreshCalls(t, *calls)
	auth := popAuthenticateMessage(t, client)
	if auth.BaseVersion != 0 || auth.TokenType != AuthTokenUser || auth.Value != "fresh" {
		t.Fatalf("unexpected reconnect auth: %#v", auth)
	}
	replayQueries := popModifyQuerySet(t, client)
	if replayQueries.BaseVersion != 0 || replayQueries.NewVersion != 1 {
		t.Fatalf("unexpected query replay versions: %#v", replayQueries)
	}
	add := onlyQuerySetAdd(t, replayQueries)
	if add.QueryID != queryID || add.UDFPath != "messages:list" {
		t.Fatalf("unexpected query replay add: %#v", add)
	}
	mutation := popMutationMessage(t, client)
	if mutation.RequestID != mutationID {
		t.Fatalf("unexpected mutation replay: %#v", mutation)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("pending action %d should not replay, got %#v", actionID, msg)
	}
}

func setRecordingAuthCallback(t *testing.T, client *Client) *[]bool {
	t.Helper()
	calls := []bool{}
	if err := client.SetAuthCallback(func(forceRefresh bool) (AuthToken, error) {
		calls = append(calls, forceRefresh)
		if forceRefresh {
			return UserAuthToken("fresh"), nil
		}
		return UserAuthToken("initial"), nil
	}); err != nil {
		t.Fatal(err)
	}
	return &calls
}

func queueReconnectFixture(t *testing.T, client *Client) (SubscriberID, QueryID, RequestID, RequestID) {
	t.Helper()
	subscriber, queryID := subscribeAndDrainAdd(t, client, "messages:list", nil)
	mutationID, err := client.Mutation("messages:send", nil)
	if err != nil {
		t.Fatal(err)
	}
	actionID, err := client.Action("jobs:run", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, client)
	_ = popActionMessage(t, client)
	return subscriber, queryID, mutationID, actionID
}

func assertAuthRefreshCalls(t *testing.T, calls []bool) {
	t.Helper()
	if len(calls) != 2 || calls[0] != false || calls[1] != true {
		t.Fatalf("unexpected auth calls: %#v", calls)
	}
}

func TestReconnectReplayWithoutAuthReplaysQueries(t *testing.T) {
	client := New()
	_, firstQuery := subscribeAndDrainAdd(t, client, "messages:list", nil)
	_, secondQuery := subscribeAndDrainAdd(t, client, "users:get", map[string]any{"id": Int64(1)})

	if err := client.RestartForReconnect(); err != nil {
		t.Fatal(err)
	}
	replay := popModifyQuerySet(t, client)
	if replay.BaseVersion != 0 || replay.NewVersion != 1 {
		t.Fatalf("unexpected replay versions: %#v", replay)
	}
	if len(replay.Modifications) != 2 {
		t.Fatalf("expected two replayed queries, got %#v", replay.Modifications)
	}
	firstAdd, ok := replay.Modifications[0].(QuerySetAdd)
	if !ok {
		t.Fatalf("expected first add, got %#v", replay.Modifications[0])
	}
	secondAdd, ok := replay.Modifications[1].(QuerySetAdd)
	if !ok {
		t.Fatalf("expected second add, got %#v", replay.Modifications[1])
	}
	if firstAdd.QueryID != firstQuery || secondAdd.QueryID != secondQuery {
		t.Fatalf("unexpected replay query ids: %#v %#v", firstAdd, secondAdd)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("expected no auth or extra replay messages, got %#v", msg)
	}
}

func TestReconnectReplayAuthRefreshErrorClearsStaleQueue(t *testing.T) {
	client := New()
	if err := client.SetAuthCallback(func(forceRefresh bool) (AuthToken, error) {
		if forceRefresh {
			return AuthToken{}, errTestReconnectAuth
		}
		return UserAuthToken("initial"), nil
	}); err != nil {
		t.Fatal(err)
	}
	_ = popAuthenticateMessage(t, client)
	client.outgoing = append(client.outgoing, ModifyQuerySetMessage{BaseVersion: 99, NewVersion: 100})

	err := client.RestartForReconnect()
	if !errors.Is(err, errTestReconnectAuth) {
		t.Fatalf("expected reconnect auth refresh error, got %v", err)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("expected failed reconnect to clear stale queue, got %#v", msg)
	}
}

func TestReconnectReplayCancelsPendingActions(t *testing.T) {
	client := New()
	mutationID, err := client.Mutation("messages:send", nil)
	if err != nil {
		t.Fatal(err)
	}
	actionID, err := client.Action("jobs:run", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, client)
	_ = popActionMessage(t, client)

	if err := client.RestartForReconnect(); err != nil {
		t.Fatal(err)
	}
	result, ok := client.ActionResult(actionID)
	if !ok {
		t.Fatal("expected reconnect to complete pending action with an error")
	}
	message, ok := result.ErrorMessage()
	if !ok || message != "Connection lost while action was in flight" {
		t.Fatalf("unexpected action reconnect result: %#v", result)
	}
	replayMutation := popMutationMessage(t, client)
	if replayMutation.RequestID != mutationID {
		t.Fatalf("unexpected mutation replay after action cancellation: %#v", replayMutation)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("expected pending action not to replay, got %#v", msg)
	}
}

func TestReconnectReplayDoesNotReplayUnsubscribedQueries(t *testing.T) {
	client := New()
	subscriber, queryID := subscribeAndDrainAdd(t, client, "messages:list", nil)
	if err := client.Unsubscribe(subscriber); err != nil {
		t.Fatal(err)
	}
	_ = popModifyQuerySet(t, client)

	if err := client.RestartForReconnect(); err != nil {
		t.Fatal(err)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("expected no query replay after final unsubscribe of %d, got %#v", queryID, msg)
	}
}

package baseclient

import "testing"

func TestBaseClientSubscribeDedupesQueries(t *testing.T) {
	client := New()

	first, err := client.Subscribe("api.messages.list", map[string]any{"room": "general"})
	if err != nil {
		t.Fatal(err)
	}
	firstMsg := popModifyQuerySet(t, client)
	firstAdd := onlyQuerySetAdd(t, firstMsg)

	second, err := client.Subscribe("messages.ts:list", map[string]any{"room": "general"})
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("subscribers must be distinct")
	}
	if firstAdd.UDFPath != "messages:list" {
		t.Fatalf("expected canonical path, got %q", firstAdd.UDFPath)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("expected no second server query, got %#v", msg)
	}
}

func TestBaseClientUnsubscribeRemovesOnlyAfterFinalSubscriber(t *testing.T) {
	client := New()

	first, err := client.Subscribe("messages:list", map[string]any{"room": "general"})
	if err != nil {
		t.Fatal(err)
	}
	addMsg := popModifyQuerySet(t, client)
	queryID := onlyQuerySetAdd(t, addMsg).QueryID

	second, err := client.Subscribe("messages:list", map[string]any{"room": "general"})
	if err != nil {
		t.Fatal(err)
	}

	if err := client.Unsubscribe(first); err != nil {
		t.Fatal(err)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("expected no remove while another subscriber exists, got %#v", msg)
	}

	if err := client.Unsubscribe(second); err != nil {
		t.Fatal(err)
	}
	removeMsg := popModifyQuerySet(t, client)
	if removeMsg.BaseVersion != 1 || removeMsg.NewVersion != 2 {
		t.Fatalf("unexpected remove versions: %#v", removeMsg)
	}
	remove := onlyQuerySetRemove(t, removeMsg)
	if remove.QueryID != queryID {
		t.Fatalf("unexpected remove query id: got %v want %v", remove.QueryID, queryID)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("expected empty queue, got %#v", msg)
	}
}

func TestBaseClientPopNextMessageIsFIFO(t *testing.T) {
	client := New()

	if _, err := client.Subscribe("messages:list", map[string]any{"room": "general"}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Subscribe("users:get", map[string]any{"id": Int64(1)}); err != nil {
		t.Fatal(err)
	}

	first := popModifyQuerySet(t, client)
	second := popModifyQuerySet(t, client)
	if first.BaseVersion != 0 || first.NewVersion != 1 {
		t.Fatalf("unexpected first versions: %#v", first)
	}
	if second.BaseVersion != 1 || second.NewVersion != 2 {
		t.Fatalf("unexpected second versions: %#v", second)
	}
	if firstAdd, secondAdd := onlyQuerySetAdd(t, first), onlyQuerySetAdd(t, second); firstAdd.QueryID == secondAdd.QueryID {
		t.Fatalf("distinct queries should have distinct ids: %v", firstAdd.QueryID)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("expected empty queue, got %#v", msg)
	}
}

func TestBaseClientSubscribeDedupesArgsByCanonicalJSON(t *testing.T) {
	client := New()

	if _, err := client.Subscribe("messages:list", map[string]any{
		"room": "general",
		"page": Int64(1),
	}); err != nil {
		t.Fatal(err)
	}
	add := onlyQuerySetAdd(t, popModifyQuerySet(t, client))
	if len(add.Args) != 1 {
		t.Fatalf("expected one sync arg object, got %#v", add.Args)
	}
	arg, ok := add.Args[0].Object()
	if !ok {
		t.Fatalf("expected object arg, got %#v", add.Args[0])
	}
	if room, ok := arg["room"].String(); !ok || room != "general" {
		t.Fatalf("unexpected room arg: %#v", arg["room"])
	}
	if page, ok := arg["page"].Int64(); !ok || page != 1 {
		t.Fatalf("unexpected page arg: %#v", arg["page"])
	}

	if _, err := client.Subscribe("messages:list", map[string]any{
		"page": Int64(1),
		"room": "general",
	}); err != nil {
		t.Fatal(err)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("expected no second message, got %#v", msg)
	}
}

func TestBaseClientReceiveTransitionUpdatesLatestResults(t *testing.T) {
	client := New()
	subscriber, queryID := subscribeAndDrainAdd(t, client, "messages:list", map[string]any{"room": "general"})

	ts := SyncTimestamp(1)
	results, err := client.ReceiveMessage(TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: ts},
		Modifications: []StateModification{
			QueryUpdated{
				QueryID:  queryID,
				Value:    StringValue("hello"),
				LogLines: []string{"log"},
				Journal:  OptionalString{Present: true},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if results == nil {
		t.Fatal("expected query results")
		return
	}
	result, ok := (*results).Get(subscriber)
	if !ok {
		t.Fatal("expected subscriber result")
	}
	value, ok := result.Value()
	if !ok {
		t.Fatalf("expected value result, got %#v", result)
	}
	text, ok := value.String()
	if !ok || text != "hello" {
		t.Fatalf("unexpected value: %#v", value)
	}
	if got, ok := client.MaxObservedTimestamp(); !ok || got != ts {
		t.Fatalf("unexpected max observed timestamp: %v %v", got, ok)
	}
}

func TestBaseClientReceiveTransitionMapsFailuresAndRemovals(t *testing.T) {
	client := New()
	subscriber, queryID := subscribeAndDrainAdd(t, client, "messages:list", nil)

	results, err := client.ReceiveMessage(TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryFailed{
				QueryID:      queryID,
				ErrorMessage: "boom",
				LogLines:     []string{},
				Journal:      OptionalString{Present: true},
				ErrorData:    OptionalValue{Present: true, Value: StringValue("bad")},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, ok := results.Get(subscriber)
	if !ok {
		t.Fatal("expected failed result")
	}
	convexErr, ok := result.ConvexError()
	if !ok || convexErr.Message != "boom" {
		t.Fatalf("expected ConvexError result, got %#v", result)
	}

	results, err = client.ReceiveMessage(TransitionMessage{
		StartVersion: StateVersion{TS: 1},
		EndVersion:   StateVersion{TS: 2},
		Modifications: []StateModification{
			QueryRemoved{QueryID: queryID},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := results.Get(subscriber); ok {
		t.Fatal("expected removed query result to disappear")
	}
}

func TestBaseClientReceiveTransitionMapsPlainFailures(t *testing.T) {
	client := New()
	subscriber, queryID := subscribeAndDrainAdd(t, client, "messages:list", nil)

	results, err := client.ReceiveMessage(TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryFailed{
				QueryID:      queryID,
				ErrorMessage: "plain boom",
				LogLines:     []string{},
				Journal:      OptionalString{Present: true},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, ok := results.Get(subscriber)
	if !ok {
		t.Fatal("expected failed result")
	}
	message, ok := result.ErrorMessage()
	if !ok || message != "plain boom" {
		t.Fatalf("expected plain error message, got %#v", result)
	}
}

func TestBaseClientIgnoresTransitionForUnsubscribedQuery(t *testing.T) {
	client := New()
	subscriber, queryID := subscribeAndDrainAdd(t, client, "messages:list", nil)

	if err := client.Unsubscribe(subscriber); err != nil {
		t.Fatal(err)
	}
	_ = popModifyQuerySet(t, client)

	results, err := client.ReceiveMessage(TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{
				QueryID:  queryID,
				Value:    StringValue("late"),
				LogLines: []string{},
				Journal:  OptionalString{Present: true},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := results.Get(subscriber); ok {
		t.Fatal("expected old subscriber result to remain absent")
	}
	if _, ok := client.GetQuery(queryID); ok {
		t.Fatal("expected stale server result to be ignored")
	}
}

func TestBaseClientMutationSuccessCompletesAfterMatchingTransition(t *testing.T) {
	client := New()
	ts := SyncTimestamp(10)
	mutationID, err := client.Mutation("messages:send", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, client)

	results, err := client.ReceiveMessage(MutationResponseMessage{
		RequestID: mutationID,
		Success:   true,
		Value:     StringValue("mutated"),
		TS:        &ts,
		LogLines:  []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if results != nil {
		t.Fatalf("expected no query results for mutation response, got %#v", results)
	}
	if _, ok := client.MutationResult(mutationID); ok {
		t.Fatal("mutation should wait for matching transition")
	}
	if _, err := client.ReceiveMessage(TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: ts},
	}); err != nil {
		t.Fatal(err)
	}
	mutationResult, ok := client.MutationResult(mutationID)
	if !ok {
		t.Fatal("expected mutation result")
	}
	mutationValue, ok := mutationResult.Value()
	if !ok {
		t.Fatalf("expected mutation value, got %#v", mutationResult)
	}
	if text, ok := mutationValue.String(); !ok || text != "mutated" {
		t.Fatalf("unexpected mutation value: %#v", mutationValue)
	}
	if got, ok := client.MaxObservedTimestamp(); !ok || got != ts {
		t.Fatalf("unexpected max observed timestamp: %v %v", got, ok)
	}
}

func TestBaseClientActionFailureCompletesImmediately(t *testing.T) {
	client := New()
	actionID, err := client.Action("jobs:run", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = popActionMessage(t, client)
	_, err = client.ReceiveMessage(ActionResponseMessage{
		RequestID:    actionID,
		Success:      false,
		ErrorMessage: "action failed",
		LogLines:     []string{},
		ErrorData:    OptionalValue{Present: true, Value: StringValue("bad")},
	})
	if err != nil {
		t.Fatal(err)
	}
	actionResult, ok := client.ActionResult(actionID)
	if !ok {
		t.Fatal("expected action result")
	}
	if convexErr, ok := actionResult.ConvexError(); !ok || convexErr.Message != "action failed" {
		t.Fatalf("unexpected action result: %#v", actionResult)
	}
}

func TestBaseClientReceiveControlMessages(t *testing.T) {
	client := New()
	if results, err := client.ReceiveMessage(PingMessage{}); err != nil || results != nil {
		t.Fatalf("expected ping no-op, got results=%#v err=%v", results, err)
	}
	if _, err := client.ReceiveMessage(TransitionChunkMessage{}); err == nil {
		t.Fatal("expected transition chunk error")
	}
}

func TestBaseClientReceiveErrorMessages(t *testing.T) {
	client := New()
	if _, err := client.ReceiveMessage(AuthErrorMessage{Error: "bad auth"}); err == nil {
		t.Fatal("expected auth error")
	}
	if _, err := client.ReceiveMessage(FatalErrorMessage{Error: "fatal"}); err == nil {
		t.Fatal("expected fatal error")
	}
}

func TestBaseClientReceiveRejectsTransitionVersionMismatch(t *testing.T) {
	client := New()

	_, err := client.ReceiveMessage(TransitionMessage{
		StartVersion: StateVersion{TS: 99},
		EndVersion:   StateVersion{TS: 100},
	})
	if err == nil {
		t.Fatal("expected version mismatch error")
	}
}

// Sources for the following conformance tests:
//   - convex-rs/src/base_client/mod.rs: subscription dedupe, cached result
//     retention, separate query result snapshots, auth refresh, and reconnect.
//   - convex-js/src/browser/sync/request_manager.ts: mutation/action completion
//     and action cancellation on reconnect.
func TestBaseClientConformanceDuplicateSubscriptionsRetainCachedResult(t *testing.T) {
	client := New()
	first, queryID := subscribeAndDrainAdd(t, client, "getValue1", nil)
	second, err := client.Subscribe("getValue1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("expected no second add for identical query, got %#v", msg)
	}

	results := receiveInt64Update(t, client, queryID, 10)
	assertResultValue(t, results, first, Int64(10))
	if err := client.Unsubscribe(first); err != nil {
		t.Fatal(err)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("expected no remove while one subscriber remains, got %#v", msg)
	}
	assertResultValue(t, client.LatestResults(), second, Int64(10))
	if err := client.Unsubscribe(second); err != nil {
		t.Fatal(err)
	}
	remove := onlyQuerySetRemove(t, popModifyQuerySet(t, client))
	if remove.QueryID != queryID {
		t.Fatalf("unexpected final remove query id: got %d want %d", remove.QueryID, queryID)
	}
	if _, ok := client.GetQuery(queryID); ok {
		t.Fatal("expected cached result removed after final unsubscribe")
	}
}

func TestBaseClientConformanceSeparateQuerySnapshotsIncludeDuplicateSubscribers(t *testing.T) {
	client := New()
	sub1, query1 := subscribeAndDrainAdd(t, client, "getValue1", nil)
	sub2a, query2 := subscribeAndDrainAdd(t, client, "getValue2", nil)
	sub2b, err := client.Subscribe("getValue2", nil)
	if err != nil {
		t.Fatal(err)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("expected no second add for duplicate getValue2 subscriber, got %#v", msg)
	}
	results := receiveTwoInt64Updates(t, client, query1, query2)
	assertResultValue(t, results, sub1, Int64(10))
	assertResultValue(t, results, sub2a, Int64(20))
	assertResultValue(t, results, sub2b, Int64(20))
}

func TestBaseClientConformanceMutationAndActionErrorsCompleteImmediately(t *testing.T) {
	client := New()
	mutationID, actionID := queueMutationAndAction(t, client)
	if _, err := client.ReceiveMessage(MutationResponseMessage{
		RequestID:    mutationID,
		Success:      false,
		ErrorMessage: "mutation failed",
		LogLines:     []string{},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ReceiveMessage(ActionResponseMessage{
		RequestID:    actionID,
		Success:      false,
		ErrorMessage: "action failed",
		ErrorData:    OptionalValue{Present: true, Value: StringValue("bad")},
		LogLines:     []string{},
	}); err != nil {
		t.Fatal(err)
	}
	assertErrorMessageResult(t, client, mutationID, "mutation failed")
	assertActionConvexError(t, client, actionID, "action failed")
}

func TestBaseClientConformanceReconnectRefreshesAuthReplaysQueriesAndMutations(t *testing.T) {
	client := New()
	if err := client.SetAuthCallback(refetchingAuthCallback); err != nil {
		t.Fatal(err)
	}
	_ = popAuthenticateMessage(t, client)
	_, queryID := subscribeAndDrainAdd(t, client, "some:query", nil)
	mutationID, actionID := queueMutationAndAction(t, client)

	if err := client.RestartForReconnect(); err != nil {
		t.Fatal(err)
	}
	auth := popAuthenticateMessage(t, client)
	if auth.BaseVersion != 0 || auth.Value != "refetched-token" {
		t.Fatalf("unexpected refreshed auth: %#v", auth)
	}
	add := onlyQuerySetAdd(t, popModifyQuerySet(t, client))
	if add.QueryID != queryID {
		t.Fatalf("unexpected replayed query id: got %d want %d", add.QueryID, queryID)
	}
	mutation := popMutationMessage(t, client)
	if mutation.RequestID != mutationID {
		t.Fatalf("unexpected replayed mutation id: got %d want %d", mutation.RequestID, mutationID)
	}
	if _, ok := client.ActionResult(actionID); !ok {
		t.Fatal("expected reconnect to cancel pending action")
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("expected reconnect queue drained, got %#v", msg)
	}
}

func TestBaseClientConformanceSyncWireUDFPathsStripJSExtension(t *testing.T) {
	// Sources:
	// - convex-js/src/browser/sync/udf_path_utils.ts canonicalizeUdfPath.
	// - convex-rs/sync_types/src/udf_path.rs CanonicalizedUdfPath::strip.
	client := New()

	if _, err := client.Subscribe("messages.js:list", map[string]any{"room": "general"}); err != nil {
		t.Fatal(err)
	}
	add := onlyQuerySetAdd(t, popModifyQuerySet(t, client))
	if add.UDFPath != "messages:list" {
		t.Fatalf("sync query path should strip .js, got %q", add.UDFPath)
	}

	mutationID, err := client.Mutation("messages.js:send", nil)
	if err != nil {
		t.Fatal(err)
	}
	mutation := popMutationMessage(t, client)
	if mutation.RequestID != mutationID || mutation.UDFPath != "messages:send" {
		t.Fatalf("sync mutation path should strip .js, got %#v", mutation)
	}

	actionID, err := client.Action("jobs.js:run", nil)
	if err != nil {
		t.Fatal(err)
	}
	action := popActionMessage(t, client)
	if action.RequestID != actionID || action.UDFPath != "jobs:run" {
		t.Fatalf("sync action path should strip .js, got %#v", action)
	}
}

func TestBaseClientCanonicalHelpersRejectInvalidInputs(t *testing.T) {
	if _, _, _, err := canonicalQuery("", nil); err == nil {
		t.Fatal("expected empty canonical query path to fail")
	}
	if _, _, err := canonicalRequest("messages:list", make(chan int)); err == nil {
		t.Fatal("expected unsupported request args to fail")
	}
	if _, err := stableJSON(map[string]any{"bad": int64(1)}); err == nil {
		t.Fatal("expected unsupported stable JSON value to fail")
	}
	if _, err := stableJSON([]any{map[string]any{"ok": true}}); err != nil {
		t.Fatal(err)
	}
}

func TestBaseClientGetQueryMissAndHit(t *testing.T) {
	client := New()
	if _, ok := client.GetQuery(QueryID(99)); ok {
		t.Fatal("expected inactive query miss")
	}
	subscriber, queryID := subscribeAndDrainAdd(t, client, "messages:list", nil)
	results := receiveInt64Update(t, client, queryID, 11)
	if _, ok := results.Get(subscriber); !ok {
		t.Fatal("expected subscriber result")
	}
	if result, ok := client.GetQuery(queryID); !ok {
		t.Fatal("expected active query hit")
	} else if value, ok := result.Value(); !ok || value.GoValue() != Int64(11) {
		t.Fatalf("unexpected query result: %#v", result)
	}
}

func receiveInt64Update(t *testing.T, client *Client, queryID QueryID, value int64) QueryResults {
	t.Helper()
	results, err := client.ReceiveMessage(TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{
				QueryID:  queryID,
				Value:    Int64Value(value),
				LogLines: []string{},
				Journal:  OptionalString{Present: true},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return *results
}

func receiveTwoInt64Updates(t *testing.T, client *Client, first QueryID, second QueryID) QueryResults {
	t.Helper()
	results, err := client.ReceiveMessage(TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{QueryID: first, Value: Int64Value(10), LogLines: []string{}, Journal: OptionalString{Present: true}},
			QueryUpdated{QueryID: second, Value: Int64Value(20), LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return *results
}

func assertResultValue(t *testing.T, results QueryResults, subscriber SubscriberID, want any) {
	t.Helper()
	got, ok := results.Get(subscriber)
	if !ok {
		t.Fatalf("missing result for %#v: %#v %v", subscriber, got, ok)
	}
	value, ok := got.Value()
	if !ok || value.GoValue() != want {
		t.Fatalf("unexpected result for %#v: %#v", subscriber, got)
	}
}

func queueMutationAndAction(t *testing.T, client *Client) (RequestID, RequestID) {
	t.Helper()
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
	return mutationID, actionID
}

func assertErrorMessageResult(t *testing.T, client *Client, id RequestID, want string) {
	t.Helper()
	got, ok := client.MutationResult(id)
	if !ok {
		t.Fatalf("missing mutation error result: %#v %v", got, ok)
	}
	if message, ok := got.ErrorMessage(); !ok || message != want {
		t.Fatalf("unexpected mutation error result: %#v", got)
	}
}

func assertActionConvexError(t *testing.T, client *Client, id RequestID, want string) {
	t.Helper()
	got, ok := client.ActionResult(id)
	if !ok {
		t.Fatal("expected action result")
	}
	if convexErr, ok := got.ConvexError(); !ok || convexErr.Message != want {
		t.Fatalf("unexpected action convex error result: %#v", got)
	}
}

func refetchingAuthCallback(forceRefresh bool) (AuthToken, error) {
	if forceRefresh {
		return UserAuthToken("refetched-token"), nil
	}
	return UserAuthToken("original-token"), nil
}

func popModifyQuerySet(t *testing.T, client *Client) ModifyQuerySetMessage {
	t.Helper()
	msg := client.PopNextMessage()
	modify, ok := msg.(ModifyQuerySetMessage)
	if !ok {
		t.Fatalf("expected ModifyQuerySetMessage, got %T %#v", msg, msg)
	}
	return modify
}

func subscribeAndDrainAdd(t *testing.T, client *Client, path string, args any) (SubscriberID, QueryID) {
	t.Helper()
	subscriber, err := client.Subscribe(path, args)
	if err != nil {
		t.Fatal(err)
	}
	add := onlyQuerySetAdd(t, popModifyQuerySet(t, client))
	return subscriber, add.QueryID
}

func onlyQuerySetRemove(t *testing.T, msg ModifyQuerySetMessage) QuerySetRemove {
	t.Helper()
	if len(msg.Modifications) != 1 {
		t.Fatalf("expected one modification, got %#v", msg.Modifications)
	}
	remove, ok := msg.Modifications[0].(QuerySetRemove)
	if !ok {
		t.Fatalf("expected QuerySetRemove, got %T %#v", msg.Modifications[0], msg.Modifications[0])
	}
	return remove
}

func onlyQuerySetAdd(t *testing.T, msg ModifyQuerySetMessage) QuerySetAdd {
	t.Helper()
	if len(msg.Modifications) != 1 {
		t.Fatalf("expected one modification, got %#v", msg.Modifications)
	}
	add, ok := msg.Modifications[0].(QuerySetAdd)
	if !ok {
		t.Fatalf("expected QuerySetAdd, got %T %#v", msg.Modifications[0], msg.Modifications[0])
	}
	return add
}

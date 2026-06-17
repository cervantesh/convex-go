package baseclient

import (
	"errors"
	"strings"
	"testing"
)

func TestMutationHardeningOptimisticStoreMissesLoadingAndErrors(t *testing.T) {
	client := New()
	_, queryID := subscribeAndDrainAdd(t, client, "messages:list", nil)

	if _, err := client.Mutation("messages:pending", nil, WithOptimisticUpdate(func(store *OptimisticLocalStore) error {
		if _, ok, err := store.GetQuery("messages:list", nil); err != nil || ok {
			t.Fatalf("pending query should be a clean miss, ok=%v err=%v", ok, err)
		}
		if _, ok, err := store.GetQuery("messages:missing", nil); err != nil || ok {
			t.Fatalf("inactive query should be a clean miss, ok=%v err=%v", ok, err)
		}
		if _, ok, err := store.GetQuery("", nil); err == nil || ok {
			t.Fatalf("invalid optimistic query path should fail without ok=true, ok=%v err=%v", ok, err)
		}
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, client)

	failedClient := New()
	_, failedQueryID := subscribeAndDrainAdd(t, failedClient, "messages:list", nil)
	if _, err := failedClient.ReceiveMessage(TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryFailed{QueryID: failedQueryID, ErrorMessage: "boom", LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := failedClient.Mutation("messages:failed", nil, WithOptimisticUpdate(func(store *OptimisticLocalStore) error {
		if _, ok, err := store.GetQuery("messages:list", nil); err != nil || ok {
			t.Fatalf("failed query should be a clean miss, ok=%v err=%v", ok, err)
		}
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, failedClient)

	if queryID != 0 {
		t.Fatalf("expected first query id to stay stable, got %d", queryID)
	}
}

func TestMutationHardeningGetAllQueriesFiltersAndCarriesValueState(t *testing.T) {
	client := New()
	_, messagesID := subscribeAndDrainAdd(t, client, "messages:list", map[string]any{"room": "general"})
	if _, err := client.Subscribe("tasks:list", nil); err != nil {
		t.Fatal(err)
	}
	_ = popModifyQuerySet(t, client)
	receiveStringUpdate(t, client, StateVersion{}, StateVersion{TS: 1}, messagesID, "server")

	if _, err := client.Mutation("messages:send", nil, WithOptimisticUpdate(func(store *OptimisticLocalStore) error {
		queries, err := store.GetAllQueries("messages:list")
		if err != nil {
			return err
		}
		if len(queries) != 1 {
			t.Fatalf("expected only messages:list query, got %#v", queries)
		}
		query := queries[0]
		if query.Path != "messages:list" || !query.HasValue {
			t.Fatalf("unexpected optimistic query metadata: %#v", query)
		}
		if value, ok := query.Value.String(); !ok || value != "server" {
			t.Fatalf("unexpected optimistic query value: %#v", query.Value)
		}
		if _, err := store.GetAllQueries(""); err == nil {
			t.Fatal("invalid GetAllQueries path should fail")
		}
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, client)
}

func TestMutationHardeningOptimisticStoreHandlesQueriesWithoutArgs(t *testing.T) {
	store := &OptimisticLocalStore{
		queries: map[string]*localBaseQuery{
			"manual": {id: 1, canonicalPath: "messages:list"},
		},
		results: map[QueryID]FunctionResult{},
	}
	queries, err := store.GetAllQueries("messages:list")
	if err != nil {
		t.Fatal(err)
	}
	if len(queries) != 1 {
		t.Fatalf("expected manual query without args, got %#v", queries)
	}
	if queries[0].HasValue {
		t.Fatalf("query without result should not have value: %#v", queries[0])
	}

	sorted := sortedBaseQueries(map[string]*localBaseQuery{
		"two": {id: 2, canonicalPath: "messages:list"},
		"one": {id: 1, canonicalPath: "messages:list"},
	})
	if len(sorted) != 2 || sorted[0].id != 1 || sorted[1].id != 2 {
		t.Fatalf("base queries should sort by query id, got %#v", sorted)
	}
}

func TestMutationHardeningQueryResultsIterManualOrdering(t *testing.T) {
	first := SubscriberID{queryID: 2, index: 2}
	second := SubscriberID{queryID: 1, index: 9}
	third := SubscriberID{queryID: 2, index: 1}
	results := QueryResults{
		subscribers: map[SubscriberID]struct{}{
			first:  {},
			second: {},
			third:  {},
		},
		results: map[QueryID]FunctionResult{
			2: ValueResult(StringValue("two")),
		},
	}

	entries := results.Iter()
	want := []SubscriberID{second, third, first}
	if len(entries) != len(want) {
		t.Fatalf("expected %d entries, got %#v", len(want), entries)
	}
	for i, wantID := range want {
		if entries[i].SubscriberID != wantID {
			t.Fatalf("entry %d ordered incorrectly: got %#v want %#v", i, entries[i].SubscriberID, wantID)
		}
	}
	if entries[0].HasResult {
		t.Fatalf("query without result should stay pending: %#v", entries[0])
	}
	assertQueryResultEntriesHaveStringValue(t, entries[1:], "two")
}

func TestMutationHardeningMutationAndActionRejectInvalidInputsWithoutAllocatingIDs(t *testing.T) {
	client := New()
	if id, snapshot, err := client.mutation("", nil); err == nil || id != 0 || snapshot != nil {
		t.Fatalf("invalid mutation should fail with zero id and nil snapshot, id=%d snapshot=%#v err=%v", id, snapshot, err)
	}
	if id, err := client.Action("", nil); err == nil || id != 0 {
		t.Fatalf("invalid action should fail with zero id, id=%d err=%v", id, err)
	}

	wantErr := errors.New("optimistic failed")
	if id, snapshot, err := client.mutation("messages:send", nil, WithOptimisticUpdate(func(*OptimisticLocalStore) error {
		return wantErr
	})); !errors.Is(err, wantErr) || id != 0 || snapshot != nil {
		t.Fatalf("failed optimistic mutation should not allocate, id=%d snapshot=%#v err=%v", id, snapshot, err)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("failed mutation should not queue message, got %#v", msg)
	}

	firstAction, err := client.Action("jobs:first", nil)
	if err != nil {
		t.Fatal(err)
	}
	secondAction, err := client.Action("jobs:second", nil)
	if err != nil {
		t.Fatal(err)
	}
	if firstAction != 0 || secondAction != 1 {
		t.Fatalf("action ids should increment from zero, got %d then %d", firstAction, secondAction)
	}
	if first := popActionMessage(t, client); first.RequestID != firstAction || first.UDFPath != "jobs:first" {
		t.Fatalf("unexpected first action message: %#v", first)
	}
	if second := popActionMessage(t, client); second.RequestID != secondAction || second.UDFPath != "jobs:second" {
		t.Fatalf("unexpected second action message: %#v", second)
	}
}

func TestMutationHardeningMaxObservedTimestampOnlyIncreases(t *testing.T) {
	client := New()
	if got, ok := client.MaxObservedTimestamp(); ok || got != 0 {
		t.Fatalf("new client should have no max timestamp, got %d ok=%v", got, ok)
	}
	client.observeTimestamp(10)
	client.observeTimestamp(5)
	client.observeTimestamp(10)
	if got, ok := client.MaxObservedTimestamp(); !ok || got != 10 {
		t.Fatalf("max timestamp should not decrease, got %d ok=%v", got, ok)
	}
}

func TestMutationHardeningOptimisticRequestsCandidateFilteringAndOrdering(t *testing.T) {
	update := func(*OptimisticLocalStore) error { return nil }
	client := New()
	client.requests = map[RequestID]*trackedSyncRequest{
		2: {id: 2, kind: syncRequestMutation, optimisticUpdate: update},
		1: {id: 1, kind: syncRequestMutation, optimisticUpdate: update},
		3: {id: 3, kind: syncRequestAction, optimisticUpdate: update},
		4: {id: 4, kind: syncRequestMutation},
	}
	candidate := &trackedSyncRequest{id: 0, kind: syncRequestMutation, optimisticUpdate: update}
	requests := client.optimisticRequests(candidate)
	want := []RequestID{0, 1, 2}
	if len(requests) != len(want) {
		t.Fatalf("expected %d optimistic requests, got %#v", len(want), requests)
	}
	for i, wantID := range want {
		if requests[i].id != wantID {
			t.Fatalf("unexpected optimistic request at %d: got %d want %d", i, requests[i].id, wantID)
		}
	}
	if got := client.optimisticRequests(&trackedSyncRequest{id: 5, kind: syncRequestAction, optimisticUpdate: update}); len(got) != 2 {
		t.Fatalf("action candidate should be ignored, got %#v", got)
	}
}

func TestMutationHardeningReplayOngoingRequestsTieBreaks(t *testing.T) {
	client := New()
	firstID, err := client.Mutation("messages:first", nil)
	if err != nil {
		t.Fatal(err)
	}
	secondID, err := client.Mutation("messages:second", nil)
	if err != nil {
		t.Fatal(err)
	}
	thirdID, err := client.Mutation("messages:third", nil)
	if err != nil {
		t.Fatal(err)
	}
	actionID, err := client.Action("jobs:run", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, client)
	_ = popMutationMessage(t, client)
	_ = popMutationMessage(t, client)
	_ = popActionMessage(t, client)

	ts := SyncTimestamp(10)
	for _, id := range []RequestID{secondID, firstID} {
		if _, err := client.ReceiveMessage(MutationResponseMessage{
			RequestID: id,
			Success:   true,
			Value:     StringValue("ok"),
			TS:        &ts,
			LogLines:  []string{},
		}); err != nil {
			t.Fatal(err)
		}
	}

	replay := client.ReplayOngoingRequests()
	want := []RequestID{firstID, secondID, thirdID}
	if len(replay) != len(want) {
		t.Fatalf("expected %d replay messages, got %#v", len(want), replay)
	}
	for i, wantID := range want {
		mutation, ok := replay[i].(MutationMessage)
		if !ok || mutation.RequestID != wantID {
			t.Fatalf("unexpected replay at %d: got %#v want request %d", i, replay[i], wantID)
		}
	}
	if _, ok := client.ActionResult(actionID); ok {
		t.Fatal("plain replay should not complete pending actions")
	}
}

func TestMutationHardeningFailedMutationWithoutOptimisticReturnsNoSnapshot(t *testing.T) {
	client := New()
	mutationID, err := client.Mutation("messages:send", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, client)
	results, err := client.ReceiveMessage(MutationResponseMessage{
		RequestID:    mutationID,
		Success:      false,
		ErrorMessage: "boom",
		LogLines:     []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if results != nil {
		t.Fatalf("non-optimistic failed mutation should not publish query results, got %#v", results)
	}
}

func TestMutationHardeningFailedMutationPropagatesOptimisticRecomputeError(t *testing.T) {
	client := New()
	_, queryID := subscribeAndDrainAdd(t, client, "messages:list", nil)
	receiveStringUpdate(t, client, StateVersion{}, StateVersion{TS: 1}, queryID, "server")
	firstID, err := client.Mutation("messages:first", nil, WithOptimisticUpdate(func(store *OptimisticLocalStore) error {
		return store.SetQuery("messages:list", nil, StringValue("first"))
	}))
	if err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, client)
	replayErr := errors.New("replay failed")
	failOnReplay := false
	if _, err := client.Mutation("messages:second", nil, WithOptimisticUpdate(func(*OptimisticLocalStore) error {
		if failOnReplay {
			return replayErr
		}
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, client)

	failOnReplay = true
	_, err = client.ReceiveMessage(MutationResponseMessage{
		RequestID:    firstID,
		Success:      false,
		ErrorMessage: "nope",
		LogLines:     []string{},
	})
	if !errors.Is(err, replayErr) {
		t.Fatalf("expected optimistic recompute error, got %v", err)
	}
}

func TestMutationHardeningMutationCompletionBoundaries(t *testing.T) {
	client := New()
	firstID, err := client.Mutation("messages:first", nil)
	if err != nil {
		t.Fatal(err)
	}
	secondID, err := client.Mutation("messages:second", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, client)
	_ = popMutationMessage(t, client)

	firstTS := SyncTimestamp(10)
	secondTS := SyncTimestamp(11)
	for _, response := range []MutationResponseMessage{
		{RequestID: firstID, Success: true, Value: StringValue("first"), TS: &firstTS, LogLines: []string{}},
		{RequestID: secondID, Success: true, Value: StringValue("second"), TS: &secondTS, LogLines: []string{}},
	} {
		if _, err := client.ReceiveMessage(response); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := client.ReceiveMessage(TransitionMessage{StartVersion: StateVersion{}, EndVersion: StateVersion{TS: firstTS}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := client.MutationResult(firstID); !ok {
		t.Fatal("first mutation should complete at matching timestamp")
	}
	if _, ok := client.MutationResult(secondID); ok {
		t.Fatal("second mutation should not complete before its timestamp")
	}
	if _, err := client.ReceiveMessage(TransitionMessage{StartVersion: StateVersion{TS: firstTS}, EndVersion: StateVersion{TS: secondTS}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := client.MutationResult(secondID); !ok {
		t.Fatal("second mutation should complete at matching timestamp")
	}
}

func TestMutationHardeningCompleteMutationRequestReturnValue(t *testing.T) {
	client := New()
	if client.completeMutationRequest(99) {
		t.Fatal("unknown mutation should not report optimistic completion")
	}
	client.requests[1] = &trackedSyncRequest{id: 1, kind: syncRequestMutation}
	if client.completeMutationRequest(1) {
		t.Fatal("mutation without result should not complete")
	}
	client.requests[1].hasResult = true
	client.requests[1].result = ValueResult(StringValue("plain"))
	if client.completeMutationRequest(1) {
		t.Fatal("non-optimistic mutation should return false")
	}
	if result, ok := client.MutationResult(1); !ok {
		t.Fatal("completed mutation result missing")
	} else if value, ok := result.Value(); !ok || value.GoValue() != "plain" {
		t.Fatalf("unexpected completed mutation result: %#v", result)
	}

	client.requests[2] = &trackedSyncRequest{
		id:               2,
		kind:             syncRequestMutation,
		hasResult:        true,
		result:           ValueResult(StringValue("optimistic")),
		optimisticUpdate: func(*OptimisticLocalStore) error { return nil },
	}
	if !client.completeMutationRequest(2) {
		t.Fatal("optimistic mutation should return true")
	}
}

func TestMutationHardeningReplayQueryAddsAndCancelActionsOrdering(t *testing.T) {
	client := New()
	client.queries = map[string]*localBaseQuery{
		"two": {id: 2, canonicalPath: "messages:two", args: []Value{StringValue("two")}},
		"one": {id: 1, canonicalPath: "messages:one", args: []Value{StringValue("one")}},
	}
	adds := client.replayQueryAdds()
	if len(adds) != 2 {
		t.Fatalf("expected two replayed query adds, got %#v", adds)
	}
	first, ok := adds[0].(QuerySetAdd)
	if !ok || first.QueryID != 1 || first.UDFPath != "messages:one" {
		t.Fatalf("unexpected first replay add: %#v", adds[0])
	}
	second, ok := adds[1].(QuerySetAdd)
	if !ok || second.QueryID != 2 || second.UDFPath != "messages:two" {
		t.Fatalf("unexpected second replay add: %#v", adds[1])
	}

	client.requests = map[RequestID]*trackedSyncRequest{
		2: {id: 2, kind: syncRequestAction},
		1: {id: 1, kind: syncRequestAction},
		3: {id: 3, kind: syncRequestMutation},
	}
	client.cancelPendingActionsForReconnect()
	for _, id := range []RequestID{1, 2} {
		result, ok := client.ActionResult(id)
		if !ok {
			t.Fatalf("missing canceled action result for %d", id)
		}
		if message, ok := result.ErrorMessage(); !ok || message != actionReconnectErrorMessage {
			t.Fatalf("unexpected canceled action result for %d: %#v", id, result)
		}
	}
	if _, ok := client.requests[3]; !ok || len(client.requests) != 1 {
		t.Fatalf("cancelPendingActions should leave only mutation request, got %#v", client.requests)
	}
}

func TestMutationHardeningAuthDefaultsAndExactErrors(t *testing.T) {
	client := New()
	if err := client.RefreshAuthForReconnect(); err == nil || err.Error() != "convex: no auth callback configured" {
		t.Fatalf("unexpected no-callback refresh error: %v", err)
	}
	if err := client.queueAuth(AuthToken{Value: "unused"}); err != nil {
		t.Fatal(err)
	}
	none := popAuthenticateMessage(t, client)
	if none.TokenType != AuthTokenNone || none.Value != "unused" || none.BaseVersion != 0 {
		t.Fatalf("empty auth token type should default to none: %#v", none)
	}

	identity := &SyncUserIdentityAttributes{Issuer: "issuer", Subject: "subject"}
	if err := client.queueAuth(AuthToken{TokenType: AuthTokenAdmin, Value: "admin-token", ActingAs: identity}); err != nil {
		t.Fatal(err)
	}
	admin := popAuthenticateMessage(t, client)
	if admin.ActingAs == nil || admin.ActingAs.TokenIdentifier != "issuer|subject" || identity.TokenIdentifier != "" {
		t.Fatalf("queueAuth should derive actingAs without mutating caller identity: auth=%#v identity=%#v", admin, identity)
	}

	authErr := (&SyncAuthError{Message: "bad auth"}).Error()
	if authErr != "convex: auth error: bad auth" {
		t.Fatalf("unexpected auth error string: %q", authErr)
	}
	if _, err := client.ReceiveMessage(TransitionChunkMessage{}); err == nil || err.Error() != "convex: unexpected transition chunk" {
		t.Fatalf("unexpected transition chunk error: %v", err)
	}
	if _, err := client.ReceiveMessage(FatalErrorMessage{Error: "fatal"}); err == nil || err.Error() != "convex: fatal error: fatal" {
		t.Fatalf("unexpected fatal error: %v", err)
	}
}

func TestMutationHardeningCanonicalQueryTokenUsesWireKeys(t *testing.T) {
	path, args, token, err := canonicalQuery("api.messages.list", map[string]any{"room": "general"})
	if err != nil {
		t.Fatal(err)
	}
	if path != "messages:list" || len(args) != 1 {
		t.Fatalf("unexpected canonical query output: path=%q args=%#v", path, args)
	}
	for _, fragment := range []string{`"args"`, `"udfPath"`, `"messages:list"`, `"room"`, `"general"`} {
		if !strings.Contains(token, fragment) {
			t.Fatalf("canonical query token %q missing %s", token, fragment)
		}
	}
}

func TestMutationHardeningSuccessfulMutationWithoutTimestampExactError(t *testing.T) {
	client := New()
	mutationID, err := client.Mutation("messages:send", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, client)
	_, err = client.ReceiveMessage(MutationResponseMessage{
		RequestID: mutationID,
		Success:   true,
		Value:     StringValue("ok"),
		LogLines:  []string{},
	})
	if err == nil || err.Error() != "convex: successful mutation response missing timestamp" {
		t.Fatalf("unexpected missing timestamp error: %v", err)
	}
}

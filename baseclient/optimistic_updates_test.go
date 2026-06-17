package baseclient

import "testing"

func TestOptimisticUpdateStoreReadsAndWritesActiveQueries(t *testing.T) {
	client := New()
	generalSub, generalID := subscribeAndDrainAdd(t, client, "messages:list", map[string]any{"room": "general"})
	randomSub, randomID := subscribeAndDrainAdd(t, client, "messages:list", map[string]any{"room": "random"})
	receiveTwoStringUpdates(t, client, generalID, "old", randomID, "random")

	mutationID, err := client.Mutation("messages:send", map[string]any{"body": "optimistic"},
		WithOptimisticUpdate(func(store *OptimisticLocalStore) error {
			value, ok, err := store.GetQuery("messages:list", map[string]any{"room": "general"})
			if err != nil {
				return err
			}
			if !ok || value.GoValue() != "old" {
				t.Fatalf("unexpected general query value in optimistic update: %#v ok=%v", value, ok)
			}
			queries, err := store.GetAllQueries("messages:list")
			if err != nil {
				return err
			}
			if len(queries) != 2 {
				t.Fatalf("expected two active messages:list queries, got %#v", queries)
			}
			if err := store.SetQuery("messages:list", map[string]any{"room": "general"}, StringValue("old+optimistic")); err != nil {
				return err
			}
			return store.SetQueryLoading("messages:list", map[string]any{"room": "random"})
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if mutationID != RequestID(0) {
		t.Fatalf("unexpected mutation id: %d", mutationID)
	}
	_ = popMutationMessage(t, client)

	latest := client.LatestResults()
	assertResultValue(t, latest, generalSub, "old+optimistic")
	if _, ok := latest.Get(randomSub); ok {
		t.Fatal("expected optimistic loading state to hide random query result")
	}
}

func TestOptimisticUpdateReplaysAfterServerTransitionWhilePending(t *testing.T) {
	client := New()
	subscriber, queryID := subscribeAndDrainAdd(t, client, "messages:list", map[string]any{"room": "general"})
	receiveStringUpdate(t, client, StateVersion{}, StateVersion{TS: 1}, queryID, "server-1")

	replayCount := 0
	mutationID, err := client.Mutation("messages:send", nil,
		WithOptimisticUpdate(func(store *OptimisticLocalStore) error {
			replayCount++
			value, ok, err := store.GetQuery("messages:list", map[string]any{"room": "general"})
			if err != nil {
				return err
			}
			if !ok {
				return store.SetQuery("messages:list", map[string]any{"room": "general"}, StringValue("optimistic-only"))
			}
			text, _ := value.String()
			return store.SetQuery("messages:list", map[string]any{"room": "general"}, StringValue(text+"+optimistic"))
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, client)
	assertResultValue(t, client.LatestResults(), subscriber, "server-1+optimistic")

	results := receiveStringUpdate(t, client, StateVersion{TS: 1}, StateVersion{TS: 2}, queryID, "server-2")
	assertResultValue(t, results, subscriber, "server-2+optimistic")
	if replayCount != 2 {
		t.Fatalf("expected optimistic update to replay after server transition, got %d calls", replayCount)
	}
	if _, ok := client.MutationResult(mutationID); ok {
		t.Fatal("mutation should remain pending until its response and matching transition")
	}
}

func TestOptimisticUpdateReplaysPendingMutationsInRequestOrder(t *testing.T) {
	client := New()
	subscriber, queryID := subscribeAndDrainAdd(t, client, "messages:list", nil)
	receiveStringUpdate(t, client, StateVersion{}, StateVersion{TS: 1}, queryID, "server-1")

	appendText := func(suffix string) SyncMutationOption {
		return WithOptimisticUpdate(func(store *OptimisticLocalStore) error {
			value, ok, err := store.GetQuery("messages:list", nil)
			if err != nil {
				return err
			}
			if !ok {
				return store.SetQuery("messages:list", nil, StringValue(suffix))
			}
			text, _ := value.String()
			return store.SetQuery("messages:list", nil, StringValue(text+suffix))
		})
	}
	if _, err := client.Mutation("messages:addA", nil, appendText("+a")); err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, client)
	if _, err := client.Mutation("messages:addB", nil, appendText("+b")); err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, client)
	assertResultValue(t, client.LatestResults(), subscriber, "server-1+a+b")

	results := receiveStringUpdate(t, client, StateVersion{TS: 1}, StateVersion{TS: 2}, queryID, "server-2")
	assertResultValue(t, results, subscriber, "server-2+a+b")
}

func TestOptimisticStoreGetAllQueriesOrdersByQueryIDAndCarriesFirstArg(t *testing.T) {
	client := New()
	_, firstQueryID := subscribeAndDrainAdd(t, client, "messages:list", map[string]any{"room": "general"})
	_, secondQueryID := subscribeAndDrainAdd(t, client, "messages:list", map[string]any{"room": "random"})

	if _, err := client.Mutation("messages:send", nil,
		WithOptimisticUpdate(func(store *OptimisticLocalStore) error {
			queries, err := store.GetAllQueries("messages:list")
			if err != nil {
				return err
			}
			if len(queries) != 2 {
				t.Fatalf("expected two active queries, got %#v", queries)
			}
			if roomFromOptimisticQuery(t, queries[0]) != "general" || roomFromOptimisticQuery(t, queries[1]) != "random" {
				t.Fatalf("queries not ordered by query id with args intact: %#v", queries)
			}
			return nil
		}),
	); err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, client)
	if firstQueryID >= secondQueryID {
		t.Fatalf("test setup expected increasing query ids, got %d then %d", firstQueryID, secondQueryID)
	}
}

func TestOptimisticUpdateRollsBackWhenMutationCompletes(t *testing.T) {
	client := New()
	subscriber, queryID := subscribeAndDrainAdd(t, client, "messages:list", nil)
	receiveStringUpdate(t, client, StateVersion{}, StateVersion{TS: 1}, queryID, "server-1")

	mutationID, err := client.Mutation("messages:send", nil,
		WithOptimisticUpdate(func(store *OptimisticLocalStore) error {
			return store.SetQuery("messages:list", nil, StringValue("optimistic"))
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, client)
	assertResultValue(t, client.LatestResults(), subscriber, "optimistic")

	ts := SyncTimestamp(2)
	if results, err := client.ReceiveMessage(MutationResponseMessage{
		RequestID: mutationID,
		Success:   true,
		Value:     StringValue("ok"),
		TS:        &ts,
		LogLines:  []string{},
	}); err != nil {
		t.Fatal(err)
	} else if results != nil {
		t.Fatalf("successful mutation response should wait for transition rollback, got %#v", results)
	}
	assertResultValue(t, client.LatestResults(), subscriber, "optimistic")

	results := receiveStringUpdate(t, client, StateVersion{TS: 1}, StateVersion{TS: ts}, queryID, "server-final")
	assertResultValue(t, results, subscriber, "server-final")
	if _, ok := client.MutationResult(mutationID); !ok {
		t.Fatal("expected completed mutation result")
	}
}

func TestOptimisticUpdateRollsBackImmediatelyOnMutationFailure(t *testing.T) {
	client := New()
	subscriber, queryID := subscribeAndDrainAdd(t, client, "messages:list", nil)
	receiveStringUpdate(t, client, StateVersion{}, StateVersion{TS: 1}, queryID, "server-1")

	mutationID, err := client.Mutation("messages:send", nil,
		WithOptimisticUpdate(func(store *OptimisticLocalStore) error {
			return store.SetQuery("messages:list", nil, StringValue("optimistic"))
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, client)
	assertResultValue(t, client.LatestResults(), subscriber, "optimistic")

	results, err := client.ReceiveMessage(MutationResponseMessage{
		RequestID:    mutationID,
		Success:      false,
		ErrorMessage: "nope",
		LogLines:     []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if results == nil {
		t.Fatal("expected rollback query results for failed optimistic mutation")
		return
	}
	assertResultValue(t, *results, subscriber, "server-1")
	assertErrorMessageResult(t, client, mutationID, "nope")
}

func roomFromOptimisticQuery(t *testing.T, query OptimisticQueryResult) string {
	t.Helper()
	args, ok := query.Args.GoValue().(map[string]any)
	if !ok {
		t.Fatalf("expected object args, got %#v", query.Args.GoValue())
	}
	room, ok := args["room"].(string)
	if !ok {
		t.Fatalf("expected room string arg, got %#v", args)
	}
	return room
}

func receiveStringUpdate(t *testing.T, client *Client, start StateVersion, end StateVersion, queryID QueryID, value string) QueryResults {
	t.Helper()
	results, err := client.ReceiveMessage(TransitionMessage{
		StartVersion: start,
		EndVersion:   end,
		Modifications: []StateModification{
			QueryUpdated{QueryID: queryID, Value: StringValue(value), LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return *results
}

func receiveTwoStringUpdates(t *testing.T, client *Client, first QueryID, firstValue string, second QueryID, secondValue string) QueryResults {
	t.Helper()
	results, err := client.ReceiveMessage(TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{QueryID: first, Value: StringValue(firstValue), LogLines: []string{}, Journal: OptionalString{Present: true}},
			QueryUpdated{QueryID: second, Value: StringValue(secondValue), LogLines: []string{}, Journal: OptionalString{Present: true}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return *results
}

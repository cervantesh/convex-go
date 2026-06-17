package baseclient

import "testing"

func TestQueryResultsEmptySnapshot(t *testing.T) {
	empty := New().LatestResults()
	if !empty.IsEmpty() || empty.Len() != 0 {
		t.Fatalf("unexpected empty snapshot: len=%d empty=%v", empty.Len(), empty.IsEmpty())
	}
	if entries := empty.Iter(); len(entries) != 0 {
		t.Fatalf("expected no empty entries, got %#v", entries)
	}
}

func TestQueryResultsIterOrdersDuplicateSubscribers(t *testing.T) {
	client := New()
	first, queryID := subscribeAndDrainAdd(t, client, "messages:list", nil)
	second, err := client.Subscribe("messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("expected no second add for duplicate query, got %#v", msg)
	}

	results, err := client.ReceiveMessage(TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{
				QueryID:  queryID,
				Value:    StringValue("shared"),
				LogLines: []string{},
				Journal:  OptionalString{Present: true},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if results.Len() != 2 || results.IsEmpty() {
		t.Fatalf("unexpected result size: len=%d empty=%v", results.Len(), results.IsEmpty())
	}

	entries := results.Iter()
	if len(entries) != 2 {
		t.Fatalf("expected two entries, got %#v", entries)
	}
	if entries[0].SubscriberID != first || entries[1].SubscriberID != second {
		t.Fatalf("entries not in subscriber order: %#v", entries)
	}
	assertQueryResultEntriesHaveStringValue(t, entries, "shared")
}

func TestQueryResultsIterOrdersByQueryThenSubscriber(t *testing.T) {
	client := New()
	first, _ := subscribeAndDrainAdd(t, client, "messages:list", nil)
	second, err := client.Subscribe("messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}
	third, _ := subscribeAndDrainAdd(t, client, "tasks:list", nil)

	entries := client.LatestResults().Iter()
	want := []SubscriberID{first, second, third}
	if len(entries) != len(want) {
		t.Fatalf("expected %d entries, got %#v", len(want), entries)
	}
	for i, wantID := range want {
		if entries[i].SubscriberID != wantID {
			t.Fatalf("entry %d ordered incorrectly: got %#v want %#v in %#v", i, entries[i].SubscriberID, wantID, entries)
		}
	}
}

func assertQueryResultEntriesHaveStringValue(t *testing.T, entries []QueryResultEntry, want string) {
	t.Helper()
	for _, entry := range entries {
		if !entry.HasResult {
			t.Fatalf("expected result for entry %#v", entry)
		}
		value, ok := entry.Result.Value()
		if !ok {
			t.Fatalf("expected value result, got %#v", entry.Result)
		}
		text, ok := value.String()
		if !ok || text != want {
			t.Fatalf("unexpected shared value: %#v", value)
		}
	}
}

func TestQueryResultsZeroSubscriberIDIsNeverValid(t *testing.T) {
	client := New()
	subscriber, _ := subscribeAndDrainAdd(t, client, "messages:list", nil)

	if subscriber == (SubscriberID{}) {
		t.Fatal("first subscriber should not be the zero value")
	}
	if _, ok := client.LatestResults().Get(SubscriberID{}); ok {
		t.Fatal("zero subscriber should not resolve")
	}
	if err := client.Unsubscribe(SubscriberID{}); err == nil {
		t.Fatal("zero subscriber unsubscribe should be rejected")
	}
}

func TestQueryResultsIterIncludesPendingSubscribers(t *testing.T) {
	client := New()
	subscriber, _ := subscribeAndDrainAdd(t, client, "messages:list", nil)

	results := client.LatestResults()
	if results.Len() != 1 || results.IsEmpty() {
		t.Fatalf("unexpected pending snapshot: len=%d empty=%v", results.Len(), results.IsEmpty())
	}
	entries := results.Iter()
	if len(entries) != 1 {
		t.Fatalf("expected one pending entry, got %#v", entries)
	}
	if entries[0].SubscriberID != subscriber || entries[0].HasResult {
		t.Fatalf("unexpected pending entry: %#v", entries[0])
	}
	if _, ok := results.Get(subscriber); ok {
		t.Fatal("pending subscriber should not have a result yet")
	}
}

func TestQueryResultsUnsubscribeCacheSemantics(t *testing.T) {
	client := New()
	first, queryID := subscribeAndDrainAdd(t, client, "messages:list", nil)
	second, err := client.Subscribe("messages:list", nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.ReceiveMessage(TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{
				QueryID:  queryID,
				Value:    StringValue("cached"),
				LogLines: []string{},
				Journal:  OptionalString{Present: true},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	if err := client.Unsubscribe(first); err != nil {
		t.Fatal(err)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("expected no remove for non-final unsubscribe, got %#v", msg)
	}
	remaining := client.LatestResults()
	if remaining.Len() != 1 {
		t.Fatalf("expected one remaining subscriber, got %d", remaining.Len())
	}
	if result, ok := remaining.Get(second); !ok {
		t.Fatal("expected cached result for remaining subscriber")
	} else if value, ok := result.Value(); !ok {
		t.Fatalf("expected value result, got %#v", result)
	} else if text, ok := value.String(); !ok || text != "cached" {
		t.Fatalf("unexpected cached value: %#v", value)
	}

	if err := client.Unsubscribe(second); err != nil {
		t.Fatal(err)
	}
	_ = popModifyQuerySet(t, client)
	final := client.LatestResults()
	if !final.IsEmpty() || final.Len() != 0 {
		t.Fatalf("expected final snapshot empty, got len=%d empty=%v", final.Len(), final.IsEmpty())
	}
	if _, ok := client.GetQuery(queryID); ok {
		t.Fatal("expected final unsubscribe to clear cached query result")
	}
}

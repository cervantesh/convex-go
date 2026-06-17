package baseclient

import "testing"

func TestCompletedRequestRetentionEvictsOldMutationResults(t *testing.T) {
	oldLimit := completedRequestRetentionLimit
	completedRequestRetentionLimit = 2
	defer func() {
		completedRequestRetentionLimit = oldLimit
	}()

	client := New()
	requests := make([]RequestID, 0, 3)
	var previousTS SyncTimestamp
	for i, value := range []string{"first", "second", "third"} {
		requestID, err := client.Mutation("messages:send", map[string]any{"body": value})
		if err != nil {
			t.Fatal(err)
		}
		requests = append(requests, requestID)
		_ = popMutationMessage(t, client)

		ts := SyncTimestamp(i + 1)
		if _, err := client.ReceiveMessage(MutationResponseMessage{
			RequestID: requestID,
			Success:   true,
			Value:     StringValue(value),
			TS:        &ts,
			LogLines:  []string{},
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := client.ReceiveMessage(TransitionMessage{
			StartVersion: StateVersion{TS: previousTS},
			EndVersion:   StateVersion{TS: ts},
		}); err != nil {
			t.Fatal(err)
		}
		previousTS = ts
	}

	if _, ok := client.MutationResult(requests[0]); ok {
		t.Fatalf("expected oldest mutation result to be evicted, request=%d", requests[0])
	}
	for _, requestID := range requests[1:] {
		if _, ok := client.MutationResult(requestID); !ok {
			t.Fatalf("expected retained mutation result for request=%d", requestID)
		}
	}
	if got := len(client.completedMutationOrder); got != 2 {
		t.Fatalf("completed mutation order length = %d, want 2", got)
	}
}

func TestCompletedRequestRetentionEvictsOldActionResults(t *testing.T) {
	oldLimit := completedRequestRetentionLimit
	completedRequestRetentionLimit = 1
	defer func() {
		completedRequestRetentionLimit = oldLimit
	}()

	client := New()
	first, err := client.Action("jobs:first", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = popActionMessage(t, client)
	if _, err := client.ReceiveMessage(ActionResponseMessage{
		RequestID: first,
		Success:   true,
		Value:     StringValue("first"),
		LogLines:  []string{},
	}); err != nil {
		t.Fatal(err)
	}

	second, err := client.Action("jobs:second", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = popActionMessage(t, client)
	if _, err := client.ReceiveMessage(ActionResponseMessage{
		RequestID: second,
		Success:   true,
		Value:     StringValue("second"),
		LogLines:  []string{},
	}); err != nil {
		t.Fatal(err)
	}

	if _, ok := client.ActionResult(first); ok {
		t.Fatalf("expected oldest action result to be evicted, request=%d", first)
	}
	if _, ok := client.ActionResult(second); !ok {
		t.Fatalf("expected latest action result to be retained, request=%d", second)
	}
	if got := len(client.actionResultOrder); got != 1 {
		t.Fatalf("action result order length = %d, want 1", got)
	}
}

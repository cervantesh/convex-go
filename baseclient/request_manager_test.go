package baseclient

import "testing"

func TestRequestManagerMutationActionIDsAndQueuedMessages(t *testing.T) {
	client := New()

	mutationID, err := client.Mutation("api.messages.send", map[string]any{"body": "hi"})
	if err != nil {
		t.Fatal(err)
	}
	actionID, err := client.Action("jobs:run", map[string]any{"id": Int64(1)})
	if err != nil {
		t.Fatal(err)
	}
	if mutationID != RequestID(0) || actionID != RequestID(1) {
		t.Fatalf("unexpected request ids: mutation=%d action=%d", mutationID, actionID)
	}

	mutation := popMutationMessage(t, client)
	if mutation.RequestID != mutationID || mutation.UDFPath != "messages:send" {
		t.Fatalf("unexpected mutation message: %#v", mutation)
	}
	action := popActionMessage(t, client)
	if action.RequestID != actionID || action.UDFPath != "jobs:run" {
		t.Fatalf("unexpected action message: %#v", action)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("expected empty queue, got %#v", msg)
	}
}

func TestRequestManagerActionResponseCompletesImmediately(t *testing.T) {
	client := New()
	requestID, err := client.Action("jobs:run", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = popActionMessage(t, client)

	results, err := client.ReceiveMessage(ActionResponseMessage{
		RequestID: requestID,
		Success:   true,
		Value:     StringValue("done"),
		LogLines:  []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if results != nil {
		t.Fatalf("expected no query results, got %#v", results)
	}
	result, ok := client.ActionResult(requestID)
	if !ok {
		t.Fatal("expected completed action result")
	}
	value, ok := result.Value()
	if !ok {
		t.Fatalf("expected action value, got %#v", result)
	}
	if text, ok := value.String(); !ok || text != "done" {
		t.Fatalf("unexpected action value: %#v", value)
	}
}

func TestRequestManagerSuccessfulMutationCompletesAfterTransitionTimestamp(t *testing.T) {
	client := New()
	requestID, err := client.Mutation("messages:send", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, client)

	ts := SyncTimestamp(10)
	if _, err := client.ReceiveMessage(MutationResponseMessage{
		RequestID: requestID,
		Success:   true,
		Value:     StringValue("ok"),
		TS:        &ts,
		LogLines:  []string{},
	}); err != nil {
		t.Fatal(err)
	}
	if _, ok := client.MutationResult(requestID); ok {
		t.Fatal("successful mutation should wait for transition timestamp")
	}

	if _, err := client.ReceiveMessage(TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: ts - 1},
	}); err != nil {
		t.Fatal(err)
	}
	if _, ok := client.MutationResult(requestID); ok {
		t.Fatal("mutation completed before matching timestamp")
	}

	if _, err := client.ReceiveMessage(TransitionMessage{
		StartVersion: StateVersion{TS: ts - 1},
		EndVersion:   StateVersion{TS: ts},
	}); err != nil {
		t.Fatal(err)
	}
	result, ok := client.MutationResult(requestID)
	if !ok {
		t.Fatal("expected completed mutation result")
	}
	value, ok := result.Value()
	if !ok {
		t.Fatalf("expected mutation value, got %#v", result)
	}
	if text, ok := value.String(); !ok || text != "ok" {
		t.Fatalf("unexpected mutation value: %#v", value)
	}
}

func TestRequestManagerPlainMutationErrorCompletesImmediately(t *testing.T) {
	client := New()
	requestID, err := client.Mutation("messages:send", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, client)

	if _, err := client.ReceiveMessage(MutationResponseMessage{
		RequestID:    requestID,
		Success:      false,
		ErrorMessage: "boom",
		LogLines:     []string{},
	}); err != nil {
		t.Fatal(err)
	}
	result, ok := client.MutationResult(requestID)
	if !ok {
		t.Fatal("expected completed mutation error")
	}
	message, ok := result.ErrorMessage()
	if !ok || message != "boom" {
		t.Fatalf("unexpected mutation error result: %#v", result)
	}
}

func TestRequestManagerRejectsInvalidAndMismatchedResponses(t *testing.T) {
	t.Run("invalid request id", func(t *testing.T) {
		client := New()
		if _, err := client.ReceiveMessage(MutationResponseMessage{RequestID: 99, Success: true, Value: NullValue()}); err == nil {
			t.Fatal("expected invalid request id error")
		}
	})

	t.Run("mutation response for action", func(t *testing.T) {
		client := New()
		requestID, err := client.Action("jobs:run", nil)
		if err != nil {
			t.Fatal(err)
		}
		_ = popActionMessage(t, client)
		ts := SyncTimestamp(1)
		if _, err := client.ReceiveMessage(MutationResponseMessage{RequestID: requestID, Success: true, Value: NullValue(), TS: &ts}); err == nil {
			t.Fatal("expected mismatched mutation response error")
		}
	})

	t.Run("action response for mutation", func(t *testing.T) {
		client := New()
		requestID, err := client.Mutation("messages:send", nil)
		if err != nil {
			t.Fatal(err)
		}
		_ = popMutationMessage(t, client)
		if _, err := client.ReceiveMessage(ActionResponseMessage{RequestID: requestID, Success: true, Value: NullValue()}); err == nil {
			t.Fatal("expected mismatched action response error")
		}
	})
}

func TestRequestManagerRejectsSuccessfulMutationWithoutTimestamp(t *testing.T) {
	client := New()
	requestID, err := client.Mutation("messages:send", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, client)

	if _, err := client.ReceiveMessage(MutationResponseMessage{
		RequestID: requestID,
		Success:   true,
		Value:     StringValue("ok"),
		LogLines:  []string{},
	}); err == nil {
		t.Fatal("expected successful mutation without timestamp to be rejected")
	}
	if _, ok := client.MutationResult(requestID); ok {
		t.Fatal("invalid response should not complete mutation")
	}
}

func TestRequestManagerReplayOngoingRequests(t *testing.T) {
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

	if _, err := client.ReceiveMessage(ActionResponseMessage{RequestID: actionID, Success: true, Value: StringValue("done")}); err != nil {
		t.Fatal(err)
	}
	ts := SyncTimestamp(10)
	if _, err := client.ReceiveMessage(MutationResponseMessage{RequestID: mutationID, Success: true, Value: StringValue("ok"), TS: &ts}); err != nil {
		t.Fatal(err)
	}

	replay := client.ReplayOngoingRequests()
	if len(replay) != 1 {
		t.Fatalf("expected one replayable request, got %#v", replay)
	}
	mutation, ok := replay[0].(MutationMessage)
	if !ok || mutation.RequestID != mutationID {
		t.Fatalf("unexpected replay message: %#v", replay[0])
	}

	if _, err := client.ReceiveMessage(TransitionMessage{StartVersion: StateVersion{}, EndVersion: StateVersion{TS: ts}}); err != nil {
		t.Fatal(err)
	}
	if replay := client.ReplayOngoingRequests(); len(replay) != 0 {
		t.Fatalf("expected no replay after completion, got %#v", replay)
	}
}

func TestRequestManagerReplayOngoingRequestsOrdersByTimestampThenID(t *testing.T) {
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
	_ = popMutationMessage(t, client)
	_ = popMutationMessage(t, client)
	_ = popMutationMessage(t, client)

	firstTS := SyncTimestamp(10)
	secondTS := SyncTimestamp(20)
	if _, err := client.ReceiveMessage(MutationResponseMessage{
		RequestID: firstID,
		Success:   true,
		Value:     StringValue("first"),
		TS:        &firstTS,
		LogLines:  []string{},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ReceiveMessage(MutationResponseMessage{
		RequestID: secondID,
		Success:   true,
		Value:     StringValue("second"),
		TS:        &secondTS,
		LogLines:  []string{},
	}); err != nil {
		t.Fatal(err)
	}

	replay := client.ReplayOngoingRequests()
	want := []RequestID{secondID, firstID, thirdID}
	if len(replay) != len(want) {
		t.Fatalf("expected %d replay messages, got %#v", len(want), replay)
	}
	for i, wantID := range want {
		mutation, ok := replay[i].(MutationMessage)
		if !ok || mutation.RequestID != wantID {
			t.Fatalf("unexpected replay order at %d: got %#v want request %d", i, replay[i], wantID)
		}
	}
}

func TestRequestManagerRestartReplaysQueryAddsInOriginalIDOrder(t *testing.T) {
	client := New()
	_, firstQueryID := subscribeAndDrainAdd(t, client, "messages:first", nil)
	_, secondQueryID := subscribeAndDrainAdd(t, client, "messages:second", nil)
	_, thirdQueryID := subscribeAndDrainAdd(t, client, "messages:third", nil)

	if err := client.RestartForReconnect(); err != nil {
		t.Fatal(err)
	}
	adds := popModifyQuerySet(t, client).Modifications
	want := []QueryID{firstQueryID, secondQueryID, thirdQueryID}
	if len(adds) != len(want) {
		t.Fatalf("expected %d replayed query adds, got %#v", len(want), adds)
	}
	for i, wantID := range want {
		add, ok := adds[i].(QuerySetAdd)
		if !ok || add.QueryID != wantID {
			t.Fatalf("unexpected replayed query at %d: got %#v want query %d", i, adds[i], wantID)
		}
	}
}

func TestRequestManagerDoesNotReplayPendingActions(t *testing.T) {
	client := New()
	if _, err := client.Action("jobs:run", nil); err != nil {
		t.Fatal(err)
	}
	_ = popActionMessage(t, client)

	if replay := client.ReplayOngoingRequests(); len(replay) != 0 {
		t.Fatalf("expected pending actions not to be replayed, got %#v", replay)
	}
}

func TestRequestManagerIgnoresDuplicateMutationResponseAfterCompletion(t *testing.T) {
	client := New()
	requestID, err := client.Mutation("messages:send", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = popMutationMessage(t, client)

	ts := SyncTimestamp(10)
	response := MutationResponseMessage{
		RequestID: requestID,
		Success:   true,
		Value:     StringValue("ok"),
		TS:        &ts,
		LogLines:  []string{},
	}
	if _, err := client.ReceiveMessage(response); err != nil {
		t.Fatal(err)
	}
	if replay := client.ReplayOngoingRequests(); len(replay) != 1 {
		t.Fatalf("expected mutation waiting for transition to replay, got %#v", replay)
	}
	if _, err := client.ReceiveMessage(TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: ts},
	}); err != nil {
		t.Fatal(err)
	}
	if _, ok := client.MutationResult(requestID); !ok {
		t.Fatal("expected completed mutation result")
	}
	if _, err := client.ReceiveMessage(response); err != nil {
		t.Fatalf("duplicate mutation response after completion should be ignored, got %v", err)
	}
}

func popMutationMessage(t *testing.T, client *Client) MutationMessage {
	t.Helper()
	msg := client.PopNextMessage()
	mutation, ok := msg.(MutationMessage)
	if !ok {
		t.Fatalf("expected MutationMessage, got %T %#v", msg, msg)
	}
	return mutation
}

func popActionMessage(t *testing.T, client *Client) ActionMessage {
	t.Helper()
	msg := client.PopNextMessage()
	action, ok := msg.(ActionMessage)
	if !ok {
		t.Fatalf("expected ActionMessage, got %T %#v", msg, msg)
	}
	return action
}

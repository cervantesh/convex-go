package baseclient_test

import (
	"testing"

	"github.com/cervantesh/convex-go/baseclient"
)

func TestClientSubscribeDedupesAndPublishesSharedResults(t *testing.T) {
	client := baseclient.New()

	first, err := client.Subscribe("messages:list", map[string]any{"room": "general"})
	if err != nil {
		t.Fatal(err)
	}
	add := onlyQuerySetAdd(t, popModifyQuerySet(t, client))

	second, err := client.Subscribe("api.messages.list", map[string]any{"room": "general"})
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("duplicate query subscribers must still have distinct local ids")
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("expected duplicate subscription not to queue another add, got %#v", msg)
	}

	results, err := client.ReceiveMessage(baseclient.TransitionMessage{
		StartVersion: baseclient.StateVersion{},
		EndVersion:   baseclient.StateVersion{TS: 1},
		Modifications: []baseclient.StateModification{
			baseclient.QueryUpdated{
				QueryID:  add.QueryID,
				Value:    baseclient.StringValue("shared"),
				LogLines: []string{},
				Journal:  baseclient.OptionalString{Present: true},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if results == nil {
		t.Fatal("expected query results")
	}
	assertStringResult(t, *results, first, "shared")
	assertStringResult(t, *results, second, "shared")
}

func TestClientRestartForReconnectReplaysAuthQueriesAndMutations(t *testing.T) {
	client := baseclient.New()
	if err := client.SetAuthCallback(func(forceRefresh bool) (baseclient.AuthToken, error) {
		if forceRefresh {
			return baseclient.UserAuthToken("fresh"), nil
		}
		return baseclient.UserAuthToken("initial"), nil
	}); err != nil {
		t.Fatal(err)
	}
	_ = popAuthenticateMessage(t, client)

	if _, err := client.Subscribe("messages:list", nil); err != nil {
		t.Fatal(err)
	}
	queryID := onlyQuerySetAdd(t, popModifyQuerySet(t, client)).QueryID
	mutationID, err := client.Mutation("messages:send", nil)
	if err != nil {
		t.Fatal(err)
	}
	actionID, err := client.Action("tasks:run", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := popMutationMessage(t, client); got.RequestID != mutationID {
		t.Fatalf("unexpected mutation request: %#v", got)
	}
	if got := popActionMessage(t, client); got.RequestID != actionID {
		t.Fatalf("unexpected action request: %#v", got)
	}

	if err := client.RestartForReconnect(); err != nil {
		t.Fatal(err)
	}
	auth := popAuthenticateMessage(t, client)
	if auth.Value != "fresh" || auth.TokenType != baseclient.AuthTokenUser {
		t.Fatalf("unexpected reconnect auth: %#v", auth)
	}
	replayedAdd := onlyQuerySetAdd(t, popModifyQuerySet(t, client))
	if replayedAdd.QueryID != queryID || replayedAdd.UDFPath != "messages:list" {
		t.Fatalf("unexpected replayed query: %#v", replayedAdd)
	}
	replayedMutation := popMutationMessage(t, client)
	if replayedMutation.RequestID != mutationID {
		t.Fatalf("unexpected replayed mutation: %#v", replayedMutation)
	}
	if msg := client.PopNextMessage(); msg != nil {
		t.Fatalf("pending action should not replay, got %#v", msg)
	}
	if result, ok := client.ActionResult(actionID); !ok {
		t.Fatal("expected pending action to complete locally on reconnect")
	} else if message, ok := result.ErrorMessage(); !ok || message == "" {
		t.Fatalf("unexpected action reconnect result: %#v", result)
	}
}

func assertStringResult(t *testing.T, results baseclient.QueryResults, subscriber baseclient.SubscriberID, want string) {
	t.Helper()
	result, ok := results.Get(subscriber)
	if !ok {
		t.Fatalf("missing result for subscriber %#v", subscriber)
	}
	value, ok := result.Value()
	if !ok {
		t.Fatalf("expected value result, got %#v", result)
	}
	if got, ok := value.GoValue().(string); !ok || got != want {
		t.Fatalf("unexpected result value: %#v", value.GoValue())
	}
}

func popModifyQuerySet(t *testing.T, client *baseclient.Client) baseclient.ModifyQuerySetMessage {
	t.Helper()
	msg := client.PopNextMessage()
	modify, ok := msg.(baseclient.ModifyQuerySetMessage)
	if !ok {
		t.Fatalf("expected ModifyQuerySetMessage, got %T %#v", msg, msg)
	}
	return modify
}

func onlyQuerySetAdd(t *testing.T, msg baseclient.ModifyQuerySetMessage) baseclient.QuerySetAdd {
	t.Helper()
	if len(msg.Modifications) != 1 {
		t.Fatalf("expected one modification, got %#v", msg.Modifications)
	}
	add, ok := msg.Modifications[0].(baseclient.QuerySetAdd)
	if !ok {
		t.Fatalf("expected QuerySetAdd, got %T %#v", msg.Modifications[0], msg.Modifications[0])
	}
	return add
}

func popAuthenticateMessage(t *testing.T, client *baseclient.Client) baseclient.AuthenticateMessage {
	t.Helper()
	msg := client.PopNextMessage()
	auth, ok := msg.(baseclient.AuthenticateMessage)
	if !ok {
		t.Fatalf("expected AuthenticateMessage, got %T %#v", msg, msg)
	}
	return auth
}

func popMutationMessage(t *testing.T, client *baseclient.Client) baseclient.MutationMessage {
	t.Helper()
	msg := client.PopNextMessage()
	mutation, ok := msg.(baseclient.MutationMessage)
	if !ok {
		t.Fatalf("expected MutationMessage, got %T %#v", msg, msg)
	}
	return mutation
}

func popActionMessage(t *testing.T, client *baseclient.Client) baseclient.ActionMessage {
	t.Helper()
	msg := client.PopNextMessage()
	action, ok := msg.(baseclient.ActionMessage)
	if !ok {
		t.Fatalf("expected ActionMessage, got %T %#v", msg, msg)
	}
	return action
}

package convex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type typedListArgs struct {
	Limit Number `json:"limit"`
}

type typedMessage struct {
	Author string `json:"author"`
	Body   string `json:"body"`
}

type typedSendArgs struct {
	Body string `json:"body"`
}

type typedSendResult struct {
	ID string `json:"id"`
}

func TestTypedFunctionReferencesHTTPCallsDecodeResults(t *testing.T) {
	var endpoints []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		endpoints = append(endpoints, r.URL.Path)
		switch r.URL.Path {
		case "/api/query":
			_, _ = w.Write([]byte(`{"status":"success","value":[{"author":"Ada","body":"hello"}]}`))
		case "/api/mutation":
			_, _ = w.Write([]byte(`{"status":"success","value":{"id":"msg_123"}}`))
		case "/api/action":
			_, _ = w.Write([]byte(`{"status":"success","value":{"id":"job_456"}}`))
		default:
			t.Fatalf("unexpected endpoint: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewHTTPClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	messagesList := NewQueryReference[typedListArgs, []typedMessage]("messages:list")
	if messagesList.Kind() != QueryKind || messagesList.Path() != "messages:list" {
		t.Fatalf("unexpected query reference metadata: kind=%q path=%q", messagesList.Kind(), messagesList.Path())
	}
	messages, err := messagesList.Query(context.Background(), client, typedListArgs{Limit: Number(2)})
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].Author != "Ada" || messages[0].Body != "hello" {
		t.Fatalf("unexpected typed query result: %#v", messages)
	}

	sendMessage := NewMutationReference[typedSendArgs, typedSendResult]("messages:send")
	if sendMessage.Kind() != MutationKind || sendMessage.Path() != "messages:send" {
		t.Fatalf("unexpected mutation reference metadata: kind=%q path=%q", sendMessage.Kind(), sendMessage.Path())
	}
	mutationResult, err := sendMessage.Mutation(context.Background(), client, typedSendArgs{Body: "hi"}, WithSkipMutationQueue())
	if err != nil {
		t.Fatal(err)
	}
	if mutationResult.ID != "msg_123" {
		t.Fatalf("unexpected typed mutation result: %#v", mutationResult)
	}

	runJob := NewActionReference[typedSendArgs, typedSendResult]("jobs:run")
	if runJob.Kind() != ActionKind || runJob.Path() != "jobs:run" {
		t.Fatalf("unexpected action reference metadata: kind=%q path=%q", runJob.Kind(), runJob.Path())
	}
	actionResult, err := runJob.Action(context.Background(), client, typedSendArgs{Body: "digest"})
	if err != nil {
		t.Fatal(err)
	}
	if actionResult.ID != "job_456" {
		t.Fatalf("unexpected typed action result: %#v", actionResult)
	}

	if strings.Join(endpoints, ",") != "/api/query,/api/mutation,/api/action" {
		t.Fatalf("unexpected endpoints: %#v", endpoints)
	}
}

func TestTypedQueryReferenceSubscribeDecodesResults(t *testing.T) {
	dialer := newFakeSyncDialer()
	client, attempt := newStartedTestWebSocketClient(t, dialer)
	defer closeTestWebSocketClient(t, client)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	messagesGet := NewQueryReference[map[string]any, typedMessage]("messages:get")
	subscription, err := messagesGet.Subscribe(ctx, client, map[string]any{"id": "msg_123"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = subscription.Close() }()

	add := onlyQuerySetAdd(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
	attempt.conn.receive(t, TransitionMessage{
		StartVersion: StateVersion{},
		EndVersion:   StateVersion{TS: 1},
		Modifications: []StateModification{
			QueryUpdated{
				QueryID: add.QueryID,
				Value: MustObjectValue(map[string]Value{
					"author": StringValue("Grace"),
					"body":   StringValue("typed subscription"),
				}),
				LogLines: []string{},
				Journal:  OptionalString{Present: true},
			},
		},
	})

	message, err := subscription.Next(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if message.Author != "Grace" || message.Body != "typed subscription" {
		t.Fatalf("unexpected typed subscription result: %#v", message)
	}
	if subscription.ID() == (SubscriberID{}) {
		t.Fatal("expected typed subscription to expose raw subscription id")
	}
	if err := subscription.Unsubscribe(ctx); err != nil {
		t.Fatal(err)
	}
	remove := onlyQuerySetRemove(t, decodeSentClientMessage[ModifyQuerySetMessage](t, attempt.conn.waitSent(t)))
	if remove.QueryID != add.QueryID {
		t.Fatalf("unexpected removed query id: got %v want %v", remove.QueryID, add.QueryID)
	}
}

func TestTypedFunctionReferenceDecodeErrorsAreReturned(t *testing.T) {
	server := newFunctionResponseServer(t, map[string]string{
		"/api/query": `{"status":"success","value":"not-an-object"}`,
	})
	defer server.Close()
	client, err := NewHTTPClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	messagesGet := NewQueryReference[struct{}, typedMessage]("messages:get")
	if _, err := messagesGet.Query(context.Background(), client, struct{}{}); err == nil {
		t.Fatal("expected decode error for mismatched typed result")
	}
}

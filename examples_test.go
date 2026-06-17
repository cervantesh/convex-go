package convex_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os"

	convex "github.com/cervantesh/convex-go"
)

func ExampleClient_queryMutationAction() {
	deploymentURL := os.Getenv("CONVEX_URL")
	if deploymentURL == "" {
		return
	}
	ctx := context.Background()
	client, err := convex.NewClient(deploymentURL)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	messages, err := client.Query(ctx, "messages:list", map[string]any{"limit": convex.Number(10)})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%#v\n", messages)

	if _, err := client.Mutation(ctx, "messages:send", map[string]any{"body": "Hello from Go"}); err != nil {
		log.Fatal(err)
	}
	if _, err := client.Action(ctx, "jobs:run", map[string]any{"kind": "digest"}); err != nil {
		log.Fatal(err)
	}
}

func ExampleNewHTTPClient() {
	deploymentURL := os.Getenv("CONVEX_URL")
	if deploymentURL == "" {
		return
	}
	client, err := convex.NewHTTPClient(deploymentURL)
	if err != nil {
		log.Fatal(err)
	}
	value, err := client.Query(context.Background(), "messages:list", nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%#v\n", value)
}

func ExampleClient_auth() {
	client, err := convex.NewClient("https://happy-animal-123.convex.cloud")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	client.SetAuth("jwt-from-your-auth-provider")
	client.ClearAuth()
	if err := client.SetAdminAuth("deploy-or-admin-key", convex.UserIdentityAttributes{
		"email": "ada@example.com",
		"name":  "Ada Lovelace",
	}); err != nil {
		log.Fatal(err)
	}
}

func ExampleClient_pagination() {
	deploymentURL := os.Getenv("CONVEX_URL")
	if deploymentURL == "" {
		return
	}
	ctx := context.Background()
	client, err := convex.NewClient(deploymentURL)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	type pageResponse struct {
		Page           []map[string]any `json:"page"`
		ContinueCursor string           `json:"continueCursor"`
		IsDone         bool             `json:"isDone"`
	}
	var all []map[string]any
	cursor := ""
	for {
		var page pageResponse
		err := client.QueryInto(ctx, "messages:listPaginated", map[string]any{
			"paginationOpts": map[string]any{
				"numItems": convex.Number(5),
				"cursor":   cursor,
			},
		}, &page)
		if err != nil {
			log.Fatal(err)
		}
		all = append(all, page.Page...)
		if page.IsDone {
			break
		}
		cursor = page.ContinueCursor
	}
	fmt.Println("collected", len(all), "results")
}

func ExampleClient_consistentQuery() {
	deploymentURL := os.Getenv("CONVEX_URL")
	if deploymentURL == "" {
		return
	}
	client, err := convex.NewClient(deploymentURL)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	value, err := client.ConsistentQuery(context.Background(), "messages:list", nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%#v\n", value.GoValue())
}

func ExampleNewQueryReference() {
	type listMessagesArgs struct {
		Limit convex.Number `json:"limit"`
	}
	type message struct {
		Author string `json:"author"`
		Body   string `json:"body"`
	}

	listMessages := convex.NewQueryReference[listMessagesArgs, []message]("messages:list")
	_ = listMessages
}

func ExampleClient_function() {
	deploymentURL := os.Getenv("CONVEX_URL")
	if deploymentURL == "" {
		return
	}
	client, err := convex.NewClient(deploymentURL)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	result, err := client.Function(context.Background(), "components/search:run", map[string]any{
		"query": "convex",
	}, "components/search")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%#v\n", result)

	value, err := client.FunctionValue(context.Background(), "components/search:run", nil, "components/search")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%#v\n", value.GoValue())
}

func ExampleClient_intoMethods() {
	deploymentURL := os.Getenv("CONVEX_URL")
	if deploymentURL == "" {
		return
	}
	client, err := convex.NewClient(deploymentURL)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	var out struct {
		OK bool `json:"ok"`
	}
	if err := client.MutationInto(context.Background(), "messages:send", map[string]any{"body": "hi"}, &out); err != nil {
		log.Fatal(err)
	}
	if err := client.ActionInto(context.Background(), "jobs:run", nil, &out); err != nil {
		log.Fatal(err)
	}
}

func ExampleClient_applicationErrors() {
	deploymentURL := os.Getenv("CONVEX_URL")
	if deploymentURL == "" {
		return
	}
	client, err := convex.NewClient(deploymentURL)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	_, err = client.Mutation(context.Background(), "messages:send", map[string]any{"body": ""})
	if err != nil {
		var convexErr *convex.ConvexError
		if errors.As(err, &convexErr) {
			fmt.Printf("application error: %s data=%#v\n", convexErr.Message, convexErr.Data.GoValue())
			return
		}
		log.Fatal(err)
	}
}

func ExampleClient_subscribe() {
	deploymentURL := os.Getenv("CONVEX_URL")
	if deploymentURL == "" {
		return
	}
	ctx := context.Background()
	client, err := convex.NewClient(deploymentURL)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	subscription, err := client.Subscribe(ctx, "messages:list", nil)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = subscription.Close() }()

	result, err := subscription.Next(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if err := result.Err(); err != nil {
		log.Fatal(err)
	}
	value, _ := result.Value()
	fmt.Printf("%#v\n", value.GoValue())
}

func ExampleWebSocketClient_mutationWithOptimisticUpdate() {
	deploymentURL := os.Getenv("CONVEX_URL")
	if deploymentURL == "" {
		return
	}
	ctx := context.Background()
	client, err := convex.NewWebSocketClient(ctx, deploymentURL)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	err = client.Mutation(ctx, "messages:send", map[string]any{"body": "hello"},
		convex.WithOptimisticUpdate(func(store *convex.OptimisticLocalStore) error {
			current, ok, err := store.GetQuery("messages:list", nil)
			if err != nil {
				return err
			}
			var messages []any
			if ok {
				messages, _ = current.GoValue().([]any)
			}
			next := append(append([]any(nil), messages...), map[string]any{"body": "hello"})
			return store.SetQuery("messages:list", nil, next)
		}),
	)
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleClient_watchAll() {
	deploymentURL := os.Getenv("CONVEX_URL")
	if deploymentURL == "" {
		return
	}
	ctx := context.Background()
	client, err := convex.NewClient(deploymentURL)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	watcher, err := client.WatchAll(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = watcher.Close() }()

	snapshot, err := watcher.Next(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(snapshot.Len())
}

func Example_valueMapping() {
	args := map[string]any{
		"countAsInt64":  int64(10),
		"countAsNumber": convex.Number(10),
		"explicitInt64": convex.Int64(10),
		"payload":       convex.Bytes([]byte("abc")),
		"notANumber":    math.NaN(),
		"negativeZero":  math.Copysign(0, -1),
	}

	encoded, err := convex.EncodeJSON(args)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(encoded) > 0)
	// Output: true
}

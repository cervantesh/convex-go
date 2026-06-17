# Community Recipes

These recipes keep the normal Go path centered on `convex.NewClient`. Reach for
`NewHTTPClient` or `NewWebSocketClient` only when you need those narrower
surfaces explicitly.

## Bearer Auth With Rotation

Use `WithAuth` for startup state, then `SetAuthContext` when a long-running app
rotates a token and needs to observe realtime refresh errors directly.

```go
client, err := convex.NewClient(
	os.Getenv("CONVEX_URL"),
	convex.WithAuth(initialJWT),
)
if err != nil {
	return err
}
defer client.Close()

refreshCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

if err := client.SetAuthContext(refreshCtx, rotatedJWT); err != nil {
	if errors.Is(err, context.DeadlineExceeded) {
		// HTTP state already uses rotatedJWT. The running realtime transport
		// did not flush the auth update before refreshCtx expired.
	}
	return err
}
```

Use `ClearAuthContext` for logout flows that should remove auth from both HTTP
and any active realtime connection:

```go
logoutCtx, cancel := context.WithTimeout(context.Background(), time.Second)
defer cancel()

if err := client.ClearAuthContext(logoutCtx); err != nil {
	return err
}
```

## Admin Auth With Acting-As Identity

Use admin auth when a server-side tool must call internal functions. Pass
`UserIdentityAttributes` only when you intentionally want to act as a user.

```go
client, err := convex.NewClient(os.Getenv("CONVEX_URL"))
if err != nil {
	return err
}
defer client.Close()

if err := client.SetAdminAuthContext(context.Background(), os.Getenv("CONVEX_ADMIN_KEY"), convex.UserIdentityAttributes{
	"issuer":  "https://issuer.example",
	"subject": "user_123",
	"email":   "ada@example.com",
}); err != nil {
	return err
}

_, err = client.Mutation(context.Background(), "internal/messages:send", map[string]any{
	"body": "server side message",
})
if err != nil {
	var httpErr *convex.HTTPError
	var functionErr *convex.FunctionError
	switch {
	case errors.As(err, &httpErr):
		return fmt.Errorf("transport failed: %w", err)
	case errors.As(err, &functionErr):
		return fmt.Errorf("convex function failed: %w", err)
	default:
		return err
	}
}
```

## Subscription Lifecycle

`Client.Subscribe` is the application path. Keep one context for the consumer
loop and unsubscribe with a bounded context when shutting down.

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
defer stop()

client, err := convex.NewClient(os.Getenv("CONVEX_URL"))
if err != nil {
	return err
}
defer client.Close()

subscription, err := client.Subscribe(ctx, "messages:list", map[string]any{
	"room": "general",
})
if err != nil {
	return err
}
defer func() {
	closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = subscription.Unsubscribe(closeCtx)
}()

for {
	result, err := subscription.Next(ctx)
	switch {
	case errors.Is(err, context.Canceled):
		return nil
	case errors.Is(err, convex.ErrSubscriptionClosed):
		return nil
	case err != nil:
		return err
	}

	if err := result.Err(); err != nil {
		var convexErr *convex.ConvexError
		if errors.As(err, &convexErr) {
			return fmt.Errorf("query returned application error: %w", err)
		}
		return err
	}

	value, _ := result.Value()
	fmt.Printf("latest snapshot: %#v\n", value.GoValue())
}
```

If `Unsubscribe` returns a context error, the subscription may still be active.
Retry `Unsubscribe` or close the client.

## Pagination Loop

Use `QueryInto` with a small response struct. Treat `continueCursor` as opaque.

```go
type MessagePage struct {
	Page           []Message `json:"page"`
	ContinueCursor string    `json:"continueCursor"`
	IsDone         bool      `json:"isDone"`
}

func ListAllMessages(ctx context.Context, client *convex.Client) ([]Message, error) {
	cursor := ""
	var all []Message

	for {
		var page MessagePage
		err := client.QueryInto(ctx, "messages:listPaginated", map[string]any{
			"paginationOpts": map[string]any{
				"numItems": convex.Number(50),
				"cursor":   cursor,
			},
		}, &page)
		if err != nil {
			return nil, err
		}

		all = append(all, page.Page...)
		if page.IsDone {
			return all, nil
		}
		cursor = page.ContinueCursor
	}
}
```

## Typed References

Typed references work well with the root `Client` for HTTP calls and with an
explicit realtime client for subscriptions.

```go
type ListMessagesArgs struct {
	Limit convex.Number `json:"limit"`
}

type Message struct {
	Author string `json:"author"`
	Body   string `json:"body"`
}

var listMessages = convex.NewQueryReference[ListMessagesArgs, []Message]("messages:list")

func LoadMessages(ctx context.Context, client *convex.Client) ([]Message, error) {
	return listMessages.Query(ctx, client, ListMessagesArgs{Limit: convex.Number(10)})
}

func WatchMessages(ctx context.Context, url string) error {
	realtime, err := convex.NewWebSocketClient(ctx, url)
	if err != nil {
		return err
	}
	defer realtime.Close()

	subscription, err := listMessages.Subscribe(ctx, realtime, ListMessagesArgs{
		Limit: convex.Number(10),
	})
	if err != nil {
		return err
	}
	defer subscription.Close()

	result, err := subscription.Next(ctx)
	if err != nil {
		return err
	}
	if err := result.Err(); err != nil {
		return err
	}
	_, _ = result.Value()
	return nil
}
```

If generated references are still too broad for a call site, keep the generated
path constant and add a narrower handwritten reference next to the calling
package.

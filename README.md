# Convex Go Client

Community Go client for [Convex](https://convex.dev/), designed for idiomatic
Go applications that need queries, mutations, actions, and realtime
subscriptions.

This repository is intentionally honest about scope:

- `Client` is the main entry point for queries, mutations, actions, and realtime subscriptions.
- `HTTPClient` remains available for explicit HTTP-only use.
- `WebSocketClient` remains available for explicit realtime use, one-shot WebSocket queries, and `WatchAll` snapshots on top of Convex sync.
- The typed `Value`, `ConvexError`, `FunctionResult`, and UDF path helpers are shared by both clients.

## API Map

| Need | Start here | Notes |
| --- | --- | --- |
| Normal app client | `NewClient` | Query, Mutation, Action, Subscribe, Close |
| HTTP-only client | `NewHTTPClient` | Explicit server-side/simple path |
| Realtime-only client | `NewWebSocketClient` + `Subscribe` | Uses Convex sync over WebSocket |
| Typed call sites | `NewQueryReference`, `NewMutationReference`, `NewActionReference` | Can be handwritten or generated |
| Code generation | `cmd/convex-go-codegen` | Offline scanner for local Convex source |
| Protocol integrations | Advanced base client section | For frameworks and bindings, not the normal app path |

## Install

Requires Go 1.25 or newer.

```sh
go get github.com/cervantesh/convex-go
```

## NewClient

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	convex "github.com/cervantesh/convex-go"
)

func main() {
	client, err := convex.NewClient(os.Getenv("CONVEX_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()
	messages, err := client.Query(ctx, "messages:list", map[string]any{
		"limit": convex.Number(10),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%#v\n", messages)

	_, err = client.Mutation(ctx, "messages:send", map[string]any{
		"author": "Me",
		"body":   "Hello from Go!",
	})
	if err != nil {
		log.Fatal(err)
	}

	job, err := client.Action(ctx, "jobs:run", map[string]any{"kind": "digest"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%#v\n", job)
}
```

`NewClient` is the normal entry point. `Query`, `Mutation`, and `Action` use
Convex's HTTP API. `Subscribe` initializes realtime resources lazily, and
`Close` releases them when they were started.

HTTP mutations are serialized per client by default. To run a mutation
immediately:

```go
_, err := client.Mutation(
	context.Background(),
	"messages:send",
	map[string]any{"body": "hello"},
	convex.WithSkipMutationQueue(),
)
```

For typed values and consistent reads:

```go
value, err := client.QueryValue(context.Background(), "counter:get", nil)
if err != nil {
	log.Fatal(err)
}
fmt.Println(value.Kind(), value.GoValue())

value, err := client.ConsistentQuery(context.Background(), "messages:list", nil)
```

## Explicit HTTP Client

Use `NewHTTPClient` when you want an HTTP-only client with no realtime state:

```go
httpClient, err := convex.NewHTTPClient(os.Getenv("CONVEX_URL"))
if err != nil {
	log.Fatal(err)
}

messages, err := httpClient.Query(context.Background(), "messages:list", nil)
```

## Realtime Subscriptions

Realtime subscriptions are available from `Client.Subscribe`, which initializes
the WebSocket sync client lazily. Subscription streams are coalesced: if a
consumer is slow, intermediate updates may be skipped, but `Next` continues
returning newer consistent values.

```go
client, err := convex.NewClient(os.Getenv("CONVEX_URL"))
if err != nil {
	log.Fatal(err)
}
defer client.Close()

subscription, err := client.Subscribe(context.Background(), "messages:list", nil)
if err != nil {
	log.Fatal(err)
}
defer subscription.Close()

for {
	result, err := subscription.Next(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	if err := result.Err(); err != nil {
		log.Fatal(err)
	}
	value, _ := result.Value()
	fmt.Printf("%#v\n", value.GoValue())
}
```

`WatchAll` is an advanced Go helper that streams coalesced snapshots for every
active subscription:

```go
watcher, err := client.WatchAll(context.Background())
if err != nil {
	log.Fatal(err)
}
defer watcher.Close()

snapshot, err := watcher.Next(context.Background())
```

## Auth

```go
client, err := convex.NewClient(
	os.Getenv("CONVEX_URL"),
	convex.WithAuth("jwt-from-your-auth-provider"),
)
if err != nil {
	log.Fatal(err)
}

if err := client.SetAuthContext(context.Background(), "rotated-jwt"); err != nil {
	log.Fatal(err)
}
if err := client.ClearAuthContext(context.Background()); err != nil {
	log.Fatal(err)
}

if err := client.SetAdminAuthContext(context.Background(), "deploy-or-admin-key", convex.UserIdentityAttributes{
	"email": "ada@example.com",
	"name":  "Ada Lovelace",
}); err != nil {
	log.Fatal(err)
}
```

`WithAuth` and `WithAdminAuth` set the initial auth state for HTTP and for any
realtime client started later. `SetAuth` sends a bearer token for public
functions. `SetAdminAuth` sends a Convex admin token and can impersonate an
identity for internal/system functions.

`SetAuthContext`, `ClearAuthContext`, and `SetAdminAuthContext` are the strict
forms for long-running apps: they let you control the timeout and observe a
realtime flush error directly. The convenience helpers without `Context`
(`SetAuth`, `ClearAuth`, `SetAdminAuth`) use a bounded background timeout
internally.

If one of the `*Context` auth calls returns an error, the local client state is
already updated. Future HTTP calls use the new auth immediately. The error only
means the currently running realtime transport did not flush that change within
the provided context.

## Application Errors

Convex functions can throw application errors with `ConvexError`. The HTTP
client preserves both the function context and the decoded application data:

```go
value, err := client.Mutation(context.Background(), "messages:send", map[string]any{
	"body": "",
})
if err != nil {
	var convexErr *convex.ConvexError
	if errors.As(err, &convexErr) {
		fmt.Printf("application error: %s data=%#v\n", convexErr.Message, convexErr.Data.GoValue())
		return
	}
	log.Fatal(err)
}
fmt.Println(value)
```

Plain execution failures without `errorData` remain `*convex.FunctionError`
values but do not unwrap to `*convex.ConvexError`.

## Value Mapping

Convex's HTTP API expects `format: "convex_encoded_json"`. This package handles
that encoding.

| Go value | Convex value |
| --- | --- |
| `nil` | `null` |
| `bool` | boolean |
| `string` | string |
| signed and unsigned integer types | Int64 |
| `convex.Int64` | Int64 |
| `float32`, `float64`, `convex.Number` | Float64 / JS `number` |
| `[]byte`, `convex.Bytes` | Bytes |
| slices and arrays | Array |
| maps with string keys, structs with `json` tags | Object |

Ordinary Go integer types intentionally encode as Convex Int64. If your Convex
function validates with `v.number()`, pass a Go float or `convex.Number`:

```go
map[string]any{
	"countAsInt64": int64(10),
	"countAsNumber": convex.Number(10),
	"price": float64(19.99),
	"explicitInt64": convex.Int64(10),
	"payload": convex.Bytes([]byte("abc")),
	"notANumber": math.NaN(),
	"infinity": math.Inf(1),
}
```

Special Float64 values (`NaN`, infinities, and negative zero) use Convex
encoded JSON, so they round-trip through Convex's wire format instead of
ordinary JSON numbers.

## Pagination

Convex paginated queries usually accept `paginationOpts` and return `page`,
`continueCursor`, and `isDone`. `QueryInto` is useful for decoding each page
into a Go struct:

```go
type Page struct {
	Page           []map[string]any `json:"page"`
	ContinueCursor string           `json:"continueCursor"`
	IsDone         bool             `json:"isDone"`
}

var all []map[string]any
cursor := ""
for {
	var page Page
	err := client.QueryInto(context.Background(), "messages:listPaginated", map[string]any{
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
```

## Typed References And Code Generation

For compile-time typed call sites, define function references directly or
generate them from your Convex source:

```go
type ListMessagesArgs struct {
	Limit convex.Number `json:"limit"`
}

type Message struct {
	Author string `json:"author"`
	Body   string `json:"body"`
}

var listMessages = convex.NewQueryReference[ListMessagesArgs, []Message]("messages:list")

messages, err := listMessages.Query(context.Background(), client, ListMessagesArgs{
	Limit: convex.Number(10),
})
```

Generate Go refs from a local Convex functions directory:

```sh
go run github.com/cervantesh/convex-go/cmd/convex-go-codegen@latest \
  -src ./convex \
  -package convexapi \
  -out ./convexapi/api.go
```

The v0 generator discovers exported `query`, `mutation`, `action`,
`internalQuery`, `internalMutation`, and `internalAction` declarations in
`.ts`, `.tsx`, `.js`, and `.jsx` files. It writes deterministic Go variables
using `NewQueryReference`, `NewMutationReference`, and `NewActionReference`.
Generated refs default to `map[string]any` args and `any` results so users can
add narrower handwritten refs where they need stronger Go types.

Use `-dry-run` to print generated code without writing a file, or `-check` in
CI to fail when the checked-in generated file is stale.

Typed query references can create typed subscriptions with an explicit
`WebSocketClient`:

```go
realtime, err := convex.NewWebSocketClient(context.Background(), os.Getenv("CONVEX_URL"))
if err != nil {
	log.Fatal(err)
}
defer realtime.Close()

subscription, err := listMessages.Subscribe(context.Background(), realtime, ListMessagesArgs{
	Limit: convex.Number(10),
})
messagePage, err := subscription.Next(context.Background())
```

## Optimistic Updates

WebSocket mutations can update active local query results while the mutation is
pending. The update is replayed if new server results arrive before completion,
then rolled back when the mutation completes or fails.

```go
realtime, err := convex.NewWebSocketClient(context.Background(), os.Getenv("CONVEX_URL"))
if err != nil {
	log.Fatal(err)
}
defer realtime.Close()

err = realtime.Mutation(context.Background(), "messages:send", map[string]any{
	"body": "hello",
}, convex.WithOptimisticUpdate(func(store *convex.OptimisticLocalStore) error {
	current, ok, err := store.GetQuery("messages:list", nil)
	if err != nil {
		return err
	}
	var messages []any
	if ok {
		messages, _ = current.GoValue().([]any)
	}
	next := append(append([]any(nil), messages...), map[string]any{
		"body": "hello",
	})
	return store.SetQuery("messages:list", nil, next)
}))
```

`OptimisticLocalStore` supports `GetQuery`, `GetAllQueries`, `SetQuery`, and
`SetQueryLoading`. Treat values read from the store as immutable; copy slices,
maps, and structs before changing them. Optimistic updates only affect active
WebSocket query results and do not change HTTP mutations or the HTTP mutation
queue. Keep the callback synchronous and in-memory; do not call client methods
from inside it.

## Advanced Base Client

Frameworks, bindings, and alternate runtimes can use
`github.com/cervantesh/convex-go/baseclient` for the deterministic sync state
machine without owning a WebSocket connection:

```go
package main

import (
	"log"

	baseclient "github.com/cervantesh/convex-go/baseclient"
)

func main() {
	base := baseclient.New()
	subscriber, err := base.Subscribe("messages:list", nil)
	if err != nil {
		log.Fatal(err)
	}

	msg := base.PopNextMessage()
	_ = msg
	_ = subscriber
}
```

Most applications should stay with `NewClient`. `NewHTTPClient` and
`NewWebSocketClient` are explicit narrower clients. The base client is for
integrations that need to drain protocol messages and feed server messages
manually.

## Current API

Implemented:

- `NewClient`
- `Client.Query`, `Client.Mutation`, `Client.Action`, `Client.Subscribe`, and
  `Client.Close`
- `NewHTTPClient`
- `Query`, `Mutation`, `Action`
- `QueryValue`, `MutationValue`, `ActionValue`
- `Function`, `FunctionValue` for `/api/function`
- `GetTimestamp`, `QueryAtTimestamp`, `ConsistentQuery`
- `QueryInto`, `MutationInto`, `ActionInto`
- typed function references with `NewQueryReference`, `NewMutationReference`,
  and `NewActionReference`
- `cmd/convex-go-codegen` for offline Go ref generation from Convex source
- WebSocket sync mutations with optimistic updates
- bearer auth and admin auth
- Convex encoded JSON for Int64, Bytes, special Float64 values, arrays, objects, maps, and structs
- Convex function errors with decoded `errorData`
- typed `Value`, `ConvexError`, `FunctionResult`
- UDF path parsing and canonicalization helpers
- `NewWebSocketClient`, `Subscribe`, `Query`, `Mutation`, and `WatchAll`

See [docs/PARITY.md](docs/PARITY.md) for current compatibility status and known
limits. The offline compatibility fixture process and source inventory are
documented in [docs/CONFORMANCE.md](docs/CONFORMANCE.md). Package layout and
advanced API boundaries are documented in
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

Maintainer workflow docs:

- [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)
- [docs/QUALITY.md](docs/QUALITY.md)
- [docs/MUTATION_TESTING.md](docs/MUTATION_TESTING.md)
- [RELEASE.md](RELEASE.md)

See [CONTRIBUTING.md](CONTRIBUTING.md) for maintainer workflow details.
Security reporting and support boundaries are documented in [SECURITY.md](SECURITY.md)
and [SUPPORT.md](SUPPORT.md).

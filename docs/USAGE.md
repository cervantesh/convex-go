# Usage

Use `convex.NewClient` as the main entry point for most applications. It
combines HTTP calls for point-in-time operations with lazy realtime startup for
subscriptions.

For longer operational patterns, see [RECIPES.md](RECIPES.md). Compilable API
examples live in [examples_test.go](../examples_test.go).

## Root client

The normal root API is:

- `convex.NewClient`
- `Client.Query`
- `Client.Mutation`
- `Client.Action`
- `Client.Subscribe`
- `Client.Close`

Use the root client unless you explicitly need an HTTP-only or realtime-only
surface.

## Auth

For fixed bearer tokens, use:

- `Client.SetAuth`
- `Client.ClearAuth`
- `Client.SetAdminAuth`

For refreshable user tokens, use `Client.SetAuthCallback`. The callback gets a
`forceRefresh` boolean so the client can retry once after an auth rejection.

If you only want the explicit realtime client, the same user-token callback
shape exists on `WebSocketClient.SetAuthCallback`.

## Realtime

Use `Client.Subscribe` for reactive query results. Each subscription yields
results with `Next(ctx)`.

If you want connection observability, use
`Client.SubscribeToConnectionState` or
`WebSocketClient.SubscribeToConnectionState`. The public snapshot type is
`ConnectionState`.

`WatchAll` exists as an advanced helper for coalesced query snapshots. It is
not the primary realtime verb.

## Observability and logging

The public observability story is intentionally small:

- `ConnectionState`
- `Client.ConnectionState`
- `WebSocketClient.ConnectionState`
- `Client.SubscribeToConnectionState`
- `WebSocketClient.SubscribeToConnectionState`

These APIs expose stable connection snapshots without leaking raw sync
transport types into normal application code.

There is no public SDK logging API yet. The client does not currently expose
`WithLogger`, log levels, or protocol event hooks in the root package. If you
need extra visibility today, use connection-state callbacks for realtime health
and wrap your own `http.Client` or transport boundary outside the SDK.

## Errors

For transport and function boundaries, the public error surface is
intentionally small:

- `HTTPError`
- `FunctionError`
- `ConvexError`
- `SyncAuthError`
- `ErrSubscriptionClosed`

Use `errors.As` and `errors.Is` in normal Go style. Timeouts and cancellation
should continue to use `context.DeadlineExceeded` and `context.Canceled`.

## Values

The package keeps the Convex value model explicit where Go needs it. Common
helpers include:

- `Number`
- `Int64`
- `Bytes`
- `EncodeJSON`
- `Value.GoValue`

Use these helpers when plain JSON would lose fidelity for Convex-specific value
shapes.

## Pagination

Pagination uses ordinary query calls. Pass Convex pagination arguments to your
function and decode into a Go struct with `QueryInto`.

See `ExampleClient_pagination` in [examples_test.go](../examples_test.go) for a
compilable pagination loop.

## Typed references and code generation

Typed references are a supported root-package API:

- `NewQueryReference`
- `NewMutationReference`
- `NewActionReference`

For larger projects, `cmd/convex-go-codegen` can generate deterministic Go
reference declarations from your Convex source tree.

The generated surface intentionally stays Go-first and predictable:

- generated refs still call the same root client APIs
- argument types default to `map[string]any`
- result types use lightweight offline inference for obvious literal returns
- when precise inference is not available, generated results fall back to
  generic Go shapes such as `any`, `[]any`, or `map[string]any`

This keeps code generation deterministic and offline without promising full
TypeScript validator inference.

## Explicit clients

Use `NewHTTPClient` when you want a point-in-time server-side client without
realtime startup.

Use `NewWebSocketClient` when you want direct control over a long-lived
realtime connection.

These explicit clients are public and supported, but the main onboarding path
remains `convex.NewClient`.

## Optimistic updates

Optimistic local query updates are available on sync-backed mutations through
`WithOptimisticUpdate`.

This is scoped to active realtime queries. If your program never starts
realtime, optimistic updates do not apply.

## Advanced baseclient

`github.com/cervantesh/convex-go/baseclient` exists for advanced integrators,
framework authors, and protocol-aware tooling. It is not the primary user
path.

Start there only if you need direct access to the deterministic sync state
machine, low-level query snapshots, or protocol-oriented integrations. For
package boundaries and tradeoffs, see [ARCHITECTURE.md](ARCHITECTURE.md).

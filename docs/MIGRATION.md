# Migration Guides

This guide helps users coming from the official JavaScript, Rust, and Python
clients adopt the Go SDK without guessing at naming or behavior.

The short version is: this package keeps the Convex concepts, but presents them
with Go-first APIs and `context.Context` on I/O boundaries.

## Common Mapping

No matter which client you are coming from, the normal Go path is:

- `convex.NewClient`
- `Client.Query`
- `Client.Mutation`
- `Client.Action`
- `Client.Subscribe`
- `Client.Close`

The explicit clients `NewHTTPClient` and `NewWebSocketClient` remain available,
but `NewClient` is the main onboarding path.

## If You Come From convex-js

Closest mental mapping:

- JS query/mutation/action calls map to `Query`, `Mutation`, and `Action`
- JS realtime subscriptions map to `Subscribe`
- JS `setAuth(...)` maps most closely to `SetAuthCallback` for refreshable user
  auth or `SetAuth` for a fixed token
- JS connection state helpers map to `ConnectionState()` and
  `SubscribeToConnectionState(...)`

Go-specific differences:

- operations take `context.Context`
- there is no React layer in this repository
- `Close()` is explicit on long-lived clients and subscriptions

## If You Come From convex-rs

This is the closest conceptual cousin.

- `ConvexClient` maps to `convex.NewClient`
- Rust `set_auth_callback(...)` maps to `SetAuthCallback`
- Rust connection-state hooks map to `SubscribeToConnectionState(...)`
- Rust examples that use a long-lived realtime client usually translate to
  `Subscribe` plus explicit `Close`

Go-specific differences:

- the public root API stays smaller and flatter
- advanced sync work lives behind `baseclient`
- normal application code should start in the root package, not protocol types

## If You Come From convex-py

Python users should expect the closest match on point-in-time calls and auth
shape.

- Python `query`, `mutation`, and `action` map directly to the Go methods
- `set_auth`, `clear_auth`, and `set_admin_auth` map to `SetAuth`, `ClearAuth`,
  and `SetAdminAuth`
- realtime still uses `Subscribe`, not a separate Python-specific pattern

Go-specific differences:

- Go uses `context.Context` instead of implicit cancellation behavior
- Go makes `Close()` explicit for long-lived realtime resources
- refreshable auth is exposed publicly through `SetAuthCallback`

## Intentional Differences In Go

These are deliberate, not missing ports:

- `Subscribe` is the canonical realtime verb; the root API does not rename it
  to `Watch`
- `context.Context` is used on I/O operations
- `Close()` is part of the normal lifecycle for realtime clients and
  subscriptions
- value helpers such as `Number`, `Int64`, and `Bytes` stay explicit where Go
  would otherwise lose Convex value fidelity
- typed references and offline code generation stay deterministic and Go-first

## Suggested First Move

If you are porting code, start by replacing the client setup and one query:

1. create `client, err := convex.NewClient(...)`
2. translate one `query`/`mutation`/`action` call
3. add `context.Context`
4. translate subscriptions last, with explicit `Close()` calls

For deeper usage patterns after the first port, continue with
[USAGE.md](USAGE.md) and [RECIPES.md](RECIPES.md).

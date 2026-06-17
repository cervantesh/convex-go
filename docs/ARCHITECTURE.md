# Architecture

## Package root

The root package is `github.com/cervantesh/convex-go`. Go packages map to
directories, so files that share the public `convex` API live together in the
repository root.

The root keeps the public client facade, explicit HTTP and realtime clients,
value model, error model, and a small pre-v1 compatibility wrapper set for
advanced sync auth concepts under one import path. Protocol and transport
implementation details live below `internal`.

## Primary user APIs

Most applications should start with `NewClient`, then call `Query`, `Mutation`,
`Action`, `Subscribe`, and `Close`.

These are the APIs that should appear first in README examples and package
documentation.

`NewHTTPClient` and `NewWebSocketClient` remain public for users who want an
explicit HTTP-only or realtime-only client, but they should be documented after
the main `Client` path.

## Value and error model

`Value`, `FunctionResult`, `ConvexError`, `FunctionError`, `HTTPError`, and UDF
path helpers model Convex values and function results across HTTP and sync.

These types are implemented in `internal/core` and re-exported from the package
root because they are shared by HTTP calls, WebSocket subscriptions, typed
references, `baseclient`, and conformance tests.

## Realtime and sync APIs

`Client.Subscribe` is the main realtime API for applications. `WebSocketClient`
is the explicit realtime client for direct sync use. Framework integrations
that need the deterministic sync state machine should use
`github.com/cervantesh/convex-go/baseclient`.

Root realtime auth should stay on `UserIdentityAttributes`. Refreshable user
JWT fetchers belong on `SetAuthCallback`; the sync-protocol identity type
`SyncUserIdentityAttributes` remains an advanced/baseclient concern.

The low-level WebSocket manager, reconnect loop, dialer abstraction, and flush
orchestration live in `internal/syncclient`. The root `WebSocketClient` wraps
that package and exposes only subscriptions, one-shot realtime queries,
WebSocket mutations, `SetAuthCallback`, `ConnectionState`,
`SubscribeToConnectionState`, `WatchAll`, and `Close`.

## Advanced protocol APIs

`baseclient` is the public advanced package for framework authors and bindings.
It exposes the deterministic sync state machine, subscriber identities, request
IDs, query results, auth token fetching, optimistic update support, and a
selected wire/codecs surface for integrators that need to own protocol I/O.

Wire message structs, protocol IDs, query set versions, and JSON codecs are
implemented in `internal/syncprotocol`. `baseclient` reexports the pieces that
advanced integrations and offline tests need, while the normal root API keeps
those details out of the main user path.

Most users should not need these types directly. Public docs should route users
to `Client`, typed references, explicit clients, and code generation first.

## Do not move public exports casually

Moving exported symbols to subpackages changes import paths and can break users.
Any package split must have a GitHub issue, API surface test update, migration
notes, and conformance coverage.

Prefer documenting the root layout before splitting packages for aesthetics.
If the root keeps growing after v0.1, consider moving unexported implementation
details behind root-package wrappers before moving public exports.

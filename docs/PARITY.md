# Compatibility Status

This module is a community-maintained Go client for Convex. It is pre-v1 and
is not an official first-party Convex client.

Compatibility targets are defined by the observable Convex behavior backed by
this repository's tests and offline fixtures. The public API is Go-first:
`NewClient`, `Query`, `Mutation`, `Action`, `Subscribe`, and `Close` are the
main path for normal applications.

## Supported Surface

| Surface | Status | Notes |
| --- | --- | --- |
| HTTP queries, mutations, actions | Supported | Available through `Client` and `NewHTTPClient`. |
| Generic `/api/function` calls | Supported | Exposed through `Function` and `FunctionValue`. |
| Consistent reads and pagination helpers | Supported | Includes timestamp-based query helpers. |
| Convex value encoding and decoding | Supported | Covers Convex JSON markers such as integers, floats, and bytes. |
| Application and transport errors | Supported | Includes `ConvexError`, `FunctionError`, and `HTTPError`. |
| Realtime subscriptions and sync-backed mutations | Supported | `Client.Subscribe` is the main path; `NewWebSocketClient` is the explicit realtime client. |
| User auth callback refresh | Supported | Root `SetAuthCallback` mirrors the refreshable user-token flow exposed by the official JS and Rust clients, while keeping admin auth explicit. |
| Public realtime connection state | Supported | `ConnectionState` and `SubscribeToConnectionState` expose stable connection snapshots without leaking raw protocol transport types. |
| Optimistic local query updates | Supported | Scoped to active realtime queries through `WithOptimisticUpdate`. |
| Advanced sync state machine | Advanced | `baseclient` is public for integrators, bindings, and alternate transports. |
| Typed references and offline codegen | Partial | Typed refs are stable, but generated argument and result types still default to generic Go shapes. |

## Backed by Tests

Compatibility claims in this repository should be backed by offline fixtures
and deterministic tests, not by memory or ad hoc live deployments.

See [CONFORMANCE.md](CONFORMANCE.md) for the upstream source inventory and the
rules for adding or changing compatibility claims.

## Known Limits

- The module is pre-v1, so public APIs may still change when a better Go shape
  is justified.
- This repository validates compatibility primarily through offline fixtures
  and local test harnesses, not through a full live Convex deployment matrix in
  CI.
- `cmd/convex-go-codegen` does not infer arbitrary TypeScript validator types
  yet; generated refs default to generic Go argument and result types.
- `WatchAll` exists as a Go-specific advanced helper. `Subscribe` remains the
  canonical Convex verb in the main API.
- Some advanced auth and sync types remain re-exported at the root for pre-v1
  compatibility, but new framework-style integrations should prefer
  `github.com/cervantesh/convex-go/baseclient`.

## Compatibility Priorities

1. Match Convex wire and value behavior where the official clients define it.
2. Keep the normal Go path centered on `NewClient` and `Subscribe`.
3. Keep raw protocol details out of the main user path.
4. Prefer additive changes and clear migration notes while the module is
   pre-v1.

# Conformance Fixtures

This project uses offline conformance fixtures to keep the Go client aligned
with the official Convex clients without requiring a live Convex deployment in
unit tests.

## Rule

Do not claim compatibility from memory. Every compatibility fixture must point
to an upstream source and must pin one observable behavior.

Accepted sources:

- official `get-convex/convex-js` source, tests, or documentation
- official `get-convex/convex-py` source, tests, README, or documentation
- official `get-convex/convex-rs` source, tests, or documentation
- Convex product documentation when no official client fixture exists

Use GitHub CLI when fetching upstream sources:

```text
gh api repos/get-convex/convex-js/contents/src/values/value.ts
gh api repos/get-convex/convex-py/readme
gh api repos/get-convex/convex-rs/contents/src/lib.rs
```

## Fixture Shape

A good fixture has:

- a test name containing `Conformance`, `Fixture`, or the upstream client name
- a short source comment with upstream repo path and function, test, or README section
- an upstream commit or tag when the fixture copies a subtle or unstable behavior
- exact wire JSON, exact encoded value, or exact public behavior
- no network access and no dependency on a live Convex deployment
- one behavior per subtest where practical

When Go intentionally differs from another client, document the reason in the
test or nearby docs. For example, Go ordinary integers encode as Convex Int64
for Rust-style value parity; use `Number` when the intended Convex type is a
JavaScript/Python-style number.

## Workflow

1. Open or reuse a GitHub issue with acceptance criteria.
2. Fetch the upstream source with GitHub CLI.
3. Write the fixture first.
4. Run the focused RED command and verify it fails for the expected reason.
5. Implement only the minimal behavior needed to pass.
6. Run the focused GREEN command and then the full validation suite.
7. Record source paths plus RED/GREEN evidence in the issue or PR.

Focused conformance command:

```text
go test ./... -run 'Test.*Conformance|Test.*Fixture|Test.*ConvexPy|Test.*ConvexRS' -count=1
```

Full local validation:

```text
go run ./cmd/convex-go-maint fmt-check
go test ./...
go test ./... -race -count=1
go vet ./...
golangci-lint run --timeout=5m
```

## Current Inventory

| Area | Local tests | Upstream basis | Behavior pinned |
| --- | --- | --- | --- |
| Values | `internal/core/value_test.go` `TestValueConformanceFixturesFromOfficialClients` | `convex-js/src/values/value.ts`, `convex-js/src/values/base64.ts`, `convex-rs/src/value/json/mod.rs` | Convex encoded JSON, URL-safe base64 decode, unsupported `$set`/`$map` sentinels |
| Value edge cases | `internal/core/value_test.go` `TestValueConformanceAdditionalOfficialFixtures` | `convex-js/src/values/value.ts`, `convex-rs/src/value/json/mod.rs` | Minimum Int64 encoding and maximum object field length |
| Special float wire rules | `internal/core/value_conformance_test.go` `TestValueConformanceSpecialFloatFixturesFromConvexJS` | `convex-js/src/values/value.ts` | Negative zero and infinities stay in `$float`; ordinary floats reject `$float` wrappers |
| Python value mapping | `internal/core/value_test.go` `TestValueConvexPyDocumentedMapping` | `convex-py` README "Convex types" | README-style `None`, bool, string, number, Int64, bytes, list, dict mapping |
| HTTP API shape | `client_test.go` `TestClientConvexPyReadmeHTTPShape` | `convex-py` README basic usage | Query, mutation, and action body shape with empty-object args for omitted args |
| Python pagination shape | `client_test.go` `TestClientConvexPyPaginationShape` | `convex-py` README "Pagination" | `paginationOpts` request payload shape |
| HTTP mutation queue | `client_test.go` `TestMutationQueueSerializesByDefault`, `TestMutationQueueCanBeSkipped` | `convex-js` HTTP client mutation queue behavior | Mutations serialize per client by default and can be bypassed explicitly |
| Sync protocol | `internal/syncprotocol/sync_protocol_test.go` `TestSyncProtocolConvexRSClientFixtures`, `TestSyncProtocolConvexRSServerFixtures` | `convex-rs/sync_types/src/types/json.rs` | Backward-compatible auth messages, empty args, null `errorData` round trips |
| Sync transition edge cases | `internal/syncprotocol/sync_protocol_test.go` `TestSyncProtocolConformanceTransitionWithEmptyModificationsRoundTrips` | `convex-rs/sync_types/src/types/json.rs` | Empty transition modification arrays round-trip |
| Sync admin auth | `internal/syncprotocol/sync_protocol_test.go` `TestSyncProtocolConformanceAdminAuthUsesImpersonatingField` | `convex-js/src/browser/sync/protocol.ts` | Admin auth encodes `impersonating` on the wire |
| Sync identity attributes | `internal/syncprotocol/identity_conformance_test.go` `TestSyncProtocolConformanceUserIdentityAttributesFromConvexRS` | `convex-rs/sync_types/src/types/json.rs` | Explicit token identifiers are preserved, issuer plus subject derives one, and incomplete identities are rejected |
| Base client state | `baseclient/client_test.go`, `baseclient/state_machine_test.go` `TestBaseClientConformance*` | `convex-rs/src/base_client/mod.rs`, `convex-js/src/browser/sync/request_manager.ts` | Subscription dedupe, cached results, request completion, auth refresh, reconnect replay |
| WebSocket lifecycle | `internal/syncclient/websocket_manager_test.go`, `subscription_test.go` WebSocket and subscription lifecycle tests | `convex-rs` subscription API model, `convex-js` browser sync manager behavior | Reconnect, flush, close, cancellation, and coalesced subscription behavior through fake transports |
| UDF paths | `baseclient/state_machine_test.go` `TestBaseClientConformanceSyncWireUDFPathsStripJSExtension` | `convex-js/src/browser/sync/udf_path_utils.ts`, `convex-rs/sync_types/src/udf_path.rs` | Sync query, mutation, and action paths strip `.js` |
| Typed references | `function_reference_test.go` `TestTypedFunctionReferencesHTTPCallsDecodeResults`, `TestTypedQueryReferenceSubscribeDecodesResults` | `convex-js/src/cli/codegen_templates/api.ts`, `convex-rs/src/client/mod.rs`, `convex-py/python/convex/http_client.py` | Go references preserve typed call sites while using existing string paths and clients |
| Go API codegen | `internal/codegen/codegen_test.go`, `cmd/convex-go-codegen/main_test.go` | `convex-js/src/cli/codegen_templates/api.ts` | Offline source scan generates deterministic Go refs from Convex query/mutation/action declarations |
| Optimistic updates | `baseclient/optimistic_updates_test.go`, `subscription_test.go` `TestWebSocketClientMutationWithOptimisticUpdatePublishesToSubscription` | `convex-js/src/browser/sync/optimistic_updates.ts`, `convex-js/src/browser/sync/optimistic_updates_impl.ts` | Active query local store get/all/set, replay over server transitions, rollback on mutation completion/failure |

## Live Harness Feedback Loop

`live_integration_test.go` is not an offline fixture, but it is the versioned
source of truth for which real-deployment behaviors must be mirrored back into
default CI coverage.

- `TestLiveIntegrationHTTPAndSync` validates request flow plus authenticated
  `live:viewer` queries against a real deployment. The same user-visible
  contracts stay pinned offline in `client_test.go`,
  `client_auth_callback_test.go`, and
  `internal/syncprotocol/identity_conformance_test.go`.
- `TestLiveIntegrationAuthCallbackAndReconnect` validates root auth callback
  refresh, forced reconnect, and subscription replay against a real
deployment. The same invariants stay pinned offline in
  `client_auth_callback_test.go`, `baseclient/reconnect_test.go`, and
  `internal/syncclient/websocket_manager_test.go`.
- The live workflow expands confidence but does not replace upstream-backed
  offline fixtures. When a live run reveals a new stable invariant, add or
  tighten the corresponding offline test first, then update this document and
  [COMPATIBILITY.md](COMPATIBILITY.md).

## Public API Drift Guard

The community-facing API should stay close to the official clients while still
being idiomatic Go:

- canonical verbs: `Query`, `Mutation`, `Action`, `Subscribe`
- `WatchAll` is a Go helper for query-set snapshots, not the primary
  subscription verb
- all I/O or blocking operations take `context.Context`
- protocol-level types belong in `baseclient` or `internal/syncprotocol`, and
  low-level transport belongs in `internal/syncclient`; none should be the first
  path shown in community examples

Before changing a public signature, compare against `convex-js`, `convex-py`,
and `convex-rs`. Prefer additive wrappers over renames while the package is
pre-v1.

## Error Model Guard

Keep the public error taxonomy small:

- `HTTPError` for non-function HTTP transport responses
- `FunctionError` for Convex query, mutation, action, and generic function failures
- `ConvexError` for application errors with Convex `errorData`
- `SyncAuthError` for sync protocol auth rejection
- `ErrSubscriptionClosed` for closed subscription and watcher handles
- `context.Canceled` and `context.DeadlineExceeded` for cancellation and timeout

Do not add public `ReconnectError` or `TimeoutError` types. Reconnect reasons
remain transport internals, and Go timeouts should use the standard context
sentinel errors.

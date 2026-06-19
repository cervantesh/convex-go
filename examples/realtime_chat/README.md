# Realtime Chat Demo

This example is a small CLI application that consumes
`github.com/cervantesh/convex-go` as a normal application dependency from its
own Go module.

## Backend Contract

The demo expects a Convex deployment exposing the same sample functions used by
this repository's live integration harness:

- `live:listMessages`
- `live:sendMessage`

The easiest starting point is the sample backend in
[`testdata/live-integration/convex/`](../../testdata/live-integration/convex/).
Deploy that sample or provide equivalent functions in your own Convex app.

## Environment

Set these variables in your shell or process environment before running the
demo:

- `CONVEX_URL` (required)
- `CONVEX_AUTH_TOKEN` (optional bearer token)

## Run

From this directory:

```sh
go run . -room general
```

To send one message and keep streaming updates:

```sh
go run . -room general -send "hello from go"
```

To query once, optionally send once, and exit without starting the realtime
loop:

```sh
go run . -room general -send "hello from go" -once
```

## What It Demonstrates

- `convex.NewClient` as the primary entry point
- optional bearer auth with `SetAuth`
- HTTP query with `QueryInto`
- realtime subscribe with `Subscribe`
- connection health with `SubscribeToConnectionState`
- clean shutdown with bounded unsubscribe on exit

## Notes

- The live integration test suite covers the same backend contract in
  [`live_integration_test.go`](../../live_integration_test.go).
- This demo stays intentionally small. It is not a framework or UI layer; it
  is a public application-level usage example for the SDK.

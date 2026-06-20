# Live Integration Workflow

This repository includes an opt-in live integration workflow for maintainers
who want to exercise the Go client against a real Convex deployment.

It is intentionally separate from normal CI:

- it runs only through manual `workflow_dispatch`
- it does not run on `push`
- it does not run on `pull_request`
- it does not replace offline unit, fixture, race, lint, or coverage gates

## Required Secret

Create this repository secret before running the workflow:

- `CONVEX_LIVE_DEPLOYMENT_URL`: the real Convex deployment URL, typically the
  `.convex.cloud` deployment URL for the test app

Optional secret:

- `CONVEX_LIVE_AUTH_TOKEN`: bearer token if your test deployment protects the
  sample functions
- `CONVEX_LIVE_AUTH_REFRESH_TOKEN`: bearer token returned by the reconnect
  auth callback; if omitted, the workflow reuses `CONVEX_LIVE_AUTH_TOKEN`
- `CONVEX_LIVE_AUTH_EXPECTED_SUBJECT`: expected authenticated subject for
  `live:viewer`
- `CONVEX_LIVE_AUTH_EXPECTED_ISSUER`: expected authenticated issuer for
  `live:viewer`
- `CONVEX_LIVE_AUTH_EXPECTED_TOKEN_IDENTIFIER`: expected authenticated token
  identifier for `live:viewer`

The workflow maps those secrets to the test environment as `CONVEX_URL` and
`CONVEX_AUTH_TOKEN`, plus the matching reconnect and identity expectation
variables when they are present.

## Expected Deployment Contract

The live workflow expects a deployment exposing these public functions:

- `live:listMessages`
- `live:sendMessage`
- `live:ping`
- `live:viewer`

A minimal sample app lives under `testdata/live-integration/convex/`.

Expected behavior:

- `live:listMessages({ room })` returns an array of messages for that room
- `live:sendMessage({ room, body, requestId })` inserts one message and returns
  the inserted document
- `live:ping({ value })` returns `{ ok: true, value }`
- `live:viewer()` returns the current auth identity summary as
  `{ authenticated, tokenIdentifier, subject, issuer }`

## Running The Workflow

1. Deploy the sample app or an equivalent app with the same function contract.
2. Store the deployment URL as `CONVEX_LIVE_DEPLOYMENT_URL`.
3. Open the `Live Integration` workflow in GitHub Actions.
4. Run it manually against the ref you want to validate.

The workflow first runs:

```text
go run ./cmd/convex-go-maint integration-env-check
```

That preflight rejects mismatched auth inputs such as a refresh token without a
primary auth token, and it reports when auth identity expectations are present.

Then it runs the live-tagged test:

```text
go test . -tags=integration -run TestLiveIntegration -count=1
```

The live suite covers:

- request flow through `live:listMessages`, `live:sendMessage`, and `live:ping`
- auth identity smoke through `live:viewer`
- root `SetAuthCallback` against HTTP plus realtime
- forced websocket reconnect and subscription replay through a wrapped live
  dialer

Use this workflow as extra release or integration evidence, not as a
replacement for the normal offline quality gates in [QUALITY.md](QUALITY.md).

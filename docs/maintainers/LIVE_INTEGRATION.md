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

The workflow maps those secrets to the test environment as `CONVEX_URL` and
`CONVEX_AUTH_TOKEN`.

## Expected Deployment Contract

The live workflow expects a deployment exposing these public functions:

- `live:listMessages`
- `live:sendMessage`
- `live:ping`

A minimal sample app lives under `testdata/live-integration/convex/`.

Expected behavior:

- `live:listMessages({ room })` returns an array of messages for that room
- `live:sendMessage({ room, body, requestId })` inserts one message and returns
  the inserted document
- `live:ping({ value })` returns `{ ok: true, value }`

## Running The Workflow

1. Deploy the sample app or an equivalent app with the same function contract.
2. Store the deployment URL as `CONVEX_LIVE_DEPLOYMENT_URL`.
3. Open the `Live Integration` workflow in GitHub Actions.
4. Run it manually against the ref you want to validate.

The workflow first runs:

```text
go run ./cmd/convex-go-maint integration-env-check
```

Then it runs the live-tagged test:

```text
go test . -tags=integration -run TestLiveIntegration -count=1
```

Use this workflow as extra release or integration evidence, not as a
replacement for the normal offline quality gates in [QUALITY.md](QUALITY.md).

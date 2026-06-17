# Development Process

This project uses spec-driven development plus TDD.

## Spec-Driven Issues

Every implementation issue must define:

- upstream reference: `convex-rs`, `convex-js`, or `convex-py`
- public API expected
- wire format expected, if relevant
- acceptance criteria
- tests required before implementation

If an issue is too broad, split it before coding.

## TDD Rule

No production behavior change should start with implementation code.

For every feature or bug fix:

1. Write the failing test.
2. Run the focused test and verify it fails for the expected reason.
3. Implement the smallest code change that makes it pass.
4. Run the focused test and the full suite.
5. Refactor only while tests stay green.

Record the RED and GREEN commands in the issue or PR body.

## Preferred Test Shapes

- Value codec changes: deterministic table tests with exact Convex encoded JSON.
- HTTP client changes: `httptest.Server` tests asserting method, path, headers, and body.
- Sync protocol changes: JSON fixtures and fake transports, not live Convex deployments.
- Subscription state machine changes: deterministic unit tests with fake server messages.

## Realtime Lifecycle Rules

- If `Unsubscribe(ctx)` returns an error, the caller must retry `Unsubscribe`
  or close the owning client. A failed unsubscribe is not documented as fully
  cleaned up until one of those paths succeeds.
- If a realtime auth update can block on transport flush, prefer the
  context-aware auth methods so callers can bound that wait explicitly.

## Conformance Fixtures

Compatibility claims must be backed by offline fixtures, not memory. A fixture
must cite an upstream `convex-js`, `convex-py`, `convex-rs`, or Convex docs
source and must test one observable behavior without contacting a live Convex
deployment.

See [CONFORMANCE.md](CONFORMANCE.md) for the fixture inventory, source rules,
and focused RED/GREEN commands.

## Quality Gates

Run the quality gates in [QUALITY.md](QUALITY.md) before closing hardening
issues. Sync and WebSocket changes must include `go test ./... -race -count=1`.

## Definition of Done

- Acceptance criteria are checked off.
- A test failed first for each behavior change.
- `go test ./...` passes.
- Docs or examples changed when API behavior changes.
- No live Convex deployment is required for unit tests.

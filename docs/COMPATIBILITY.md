# Compatibility Matrix

This document answers two narrower questions than [PARITY.md](PARITY.md):

- which Go toolchains are supported
- what Convex backend behavior is actually verified

Compatibility claims must stay backed by versioned repository evidence, not by
memory or by ad hoc workstation runs.

## Go Toolchain Matrix

| Scope | Version or platform | Evidence | Notes |
| --- | --- | --- | --- |
| Minimum supported toolchain | Go 1.25 | `go.mod`, [README.md](../README.md), `.github/workflows/ci.yml` | Go versions older than 1.25 are outside the support policy. |
| Release-quality validation | Go 1.25 and `stable` on `ubuntu-latest` | `.github/workflows/ci.yml`, [maintainers/QUALITY.md](maintainers/QUALITY.md) | This job runs race, shuffle, vet, vulncheck, lint, coverage, and Sonar report generation. |
| Cross-platform smoke validation | `stable` on `ubuntu-latest`, `windows-latest`, and `macos-latest` | `.github/workflows/ci.yml` | This proves the default module path, format checks, module verification, the compiled public examples in the root module, and the `examples/realtime_chat` demo module across the supported operating systems. |
| Live integration runner | `stable` on `ubuntu-latest` | `.github/workflows/live-integration.yml`, [maintainers/LIVE_INTEGRATION.md](maintainers/LIVE_INTEGRATION.md) | This workflow is opt-in and manual. It is extra release evidence, not the default merge gate. |

## Backend Evidence Matrix

| Backend surface | Evidence status | Evidence | Notes |
| --- | --- | --- | --- |
| Offline HTTP request semantics | Supported in default CI | `client_test.go`, `unified_client_test.go`, [CONFORMANCE.md](CONFORMANCE.md) | Query, mutation, action, function, timestamp, pagination, and auth edge cases are covered without a live deployment. |
| Offline realtime and sync semantics | Supported in default CI | `subscription_test.go`, `connection_state_test.go`, `subscription_soak_test.go`, `internal/syncclient/websocket_manager_soak_test.go`, [CONFORMANCE.md](CONFORMANCE.md) | Subscribe, auth callback refresh, connection state, unsubscribe retry after cancellation, and deterministic reconnect behavior are covered offline. |
| Live request and auth smoke against a real Convex deployment | Supported in manual workflow | `live_integration_test.go`, `.github/workflows/live-integration.yml`, [maintainers/LIVE_INTEGRATION.md](maintainers/LIVE_INTEGRATION.md) | The live workflow validates `live:listMessages`, `live:sendMessage`, and `live:ping`, with optional bearer auth through `CONVEX_AUTH_TOKEN`. |
| Full live reconnect and replay coverage | Not yet a default public gate | [ROADMAP.md](ROADMAP.md), `.github/workflows/live-integration.yml` | Deeper live reconnect, replay, and auth-refresh coverage remains roadmap work before stronger backend-wide guarantees are claimed. |
| Older, custom, or unofficial Convex backend variants | Not claimed | [PARITY.md](PARITY.md), [CONFORMANCE.md](CONFORMANCE.md) | This repository does not promise compatibility with historical backend builds or unofficial protocol variants without direct evidence. |

## Reading The Matrix

- [PARITY.md](PARITY.md) answers which public SDK surfaces are supported.
- This file answers which Go toolchains and backend evidence support those
  claims.
- [CONFORMANCE.md](CONFORMANCE.md) lists the upstream fixture sources that back
  offline compatibility behavior.
- [maintainers/LIVE_INTEGRATION.md](maintainers/LIVE_INTEGRATION.md) defines
  the sample deployment contract for manual live validation.

## Update Policy

When compatibility policy changes, update these files together:

- `go.mod`
- [README.md](../README.md)
- `.github/workflows/ci.yml`
- `.github/workflows/live-integration.yml`
- [maintainers/QUALITY.md](maintainers/QUALITY.md)
- [maintainers/LIVE_INTEGRATION.md](maintainers/LIVE_INTEGRATION.md)
- this file

When the live backend contract changes, also update
`testdata/live-integration/convex/` and `live_integration_test.go` so the
manual workflow and documented sample deployment stay aligned.

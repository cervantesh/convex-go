# Changelog

All notable changes to this project will be documented here.

## Unreleased

- No unreleased changes yet.

## 0.1.0 - 2026-06-16

- Initial public release of the Community Go client for Convex.
- Main `Client` facade with HTTP queries, mutations, actions, auth, and lazy
  realtime subscriptions.
- Explicit `HTTPClient` and `WebSocketClient` APIs for narrower use cases.
- Convex value encoding, typed function references, offline code generation,
  and offline conformance fixtures.
- Cross-platform CI smoke coverage plus Ubuntu quality gates for race tests,
  coverage, lint, and report generation.

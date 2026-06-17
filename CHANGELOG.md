# Changelog

All notable changes to this project will be documented here.

## Unreleased

- Move Sonar scanning and quality report publication into GitHub Actions so
  the public maintainer workflow no longer depends on workstation-local Sonar
  runs.
- Enable GitHub-native repository hardening: Dependabot version updates,
  Dependabot security updates, vulnerability alerts, private vulnerability
  reporting, immutable releases, and CodeQL scanning.

## 0.1.0 - 2026-06-16

- Initial public release of the Community Go client for Convex.
- Main `Client` facade with HTTP queries, mutations, actions, auth, and lazy
  realtime subscriptions.
- Explicit `HTTPClient` and `WebSocketClient` APIs for narrower use cases.
- Convex value encoding, typed function references, offline code generation,
  and offline conformance fixtures.
- Cross-platform CI smoke coverage plus Ubuntu quality gates for race tests,
  coverage, lint, and report generation.

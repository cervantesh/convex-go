# Convex Go Client Adoption Packet

This packet summarizes the current public state of
`github.com/cervantesh/convex-go` for maintainers, adopters, and the Convex
team.

Status as of 2026-06-19:

- community-maintained
- pre-v1
- not an official first-party Convex client yet

## What Exists Today

- Main Go-first client path: `NewClient`, `Query`, `Mutation`, `Action`,
  `Subscribe`, `Close`
- Explicit narrower clients: `NewHTTPClient`, `NewWebSocketClient`
- Realtime subscriptions, auth callback refresh, connection state snapshots,
  optimistic updates, typed references, and offline code generation
- Advanced public `baseclient` package for integrators and alternate runtimes

See [PARITY.md](PARITY.md), [ARCHITECTURE.md](ARCHITECTURE.md), and
[USAGE.md](USAGE.md) for the current public shape.

## Supported And Not Yet Supported

Supported today:

- HTTP queries, mutations, actions, and generic function calls
- Offline Convex value conformance backed by upstream fixtures
- Realtime subscriptions and sync-backed mutations
- Public auth callback refresh
- Public connection state observability
- Typed references and offline code generation

Not yet supported or intentionally deferred:

- public root-package logging hooks
- full live reconnect and replay coverage as a default CI gate
- official Convex ownership or `get-convex` module path
- public claim of broad compatibility with historical or unofficial backend
  variants

See [PARITY.md](PARITY.md) and [COMPATIBILITY.md](COMPATIBILITY.md) for the
full supported surface and current limits.

## Quality Signals

- Cross-platform GitHub Actions smoke coverage on Linux, macOS, and Windows
- Ubuntu quality gates on Go 1.25 and `stable`
- Coverage gate at `>= 90%`
- Race tests, shuffle tests, `go vet`, `govulncheck`, and `golangci-lint`
- GitHub-hosted Sonar report generation
- CodeQL and Dependabot enabled in the public repository

Quality contract references:

- [maintainers/QUALITY.md](maintainers/QUALITY.md)
- [.github/workflows/ci.yml](../.github/workflows/ci.yml)
- [COMPATIBILITY.md](COMPATIBILITY.md)

## Compatibility Evidence

This repository does not treat compatibility as a marketing claim. Public
compatibility statements should stay backed by:

- offline conformance fixtures against `convex-js`, `convex-py`, and
  `convex-rs`
- deterministic unit and integration-style tests
- documented limits where live backend coverage is still partial

Primary evidence references:

- [CONFORMANCE.md](CONFORMANCE.md)
- [PARITY.md](PARITY.md)
- [COMPATIBILITY.md](COMPATIBILITY.md)

## Maintainer And Support Model

- Support routing is public and issue-driven.
- Security reporting is private-first.
- Governance, merge policy, release authority, and pre-v1 breaking-change
  policy are documented.
- Community intake uses tracked issue templates and weekly triage guidance.

Operational references:

- [SUPPORT.md](../SUPPORT.md)
- [SECURITY.md](../SECURITY.md)
- [maintainers/COMMUNITY.md](maintainers/COMMUNITY.md)
- [maintainers/GOVERNANCE.md](maintainers/GOVERNANCE.md)
- [maintainers/RELEASE.md](maintainers/RELEASE.md)

## Gaps Before Official Adoption

The client is usable today as the public community Go client, but these items
remain open before official adoption should be proposed as complete:

- keep strengthening runtime hardening beyond the current manual live evidence
- fuzzing and deeper runtime hardening
- public cookbook and migration guides
- demo application and demo CI smoke coverage
- external adopter validation
- namespace transition readiness under `github.com/get-convex/convex-go`

These items are already tracked in [ROADMAP.md](ROADMAP.md).

## Current Ask To Convex

The current community-first ask is modest:

1. Evaluate this repository as the public community Go client.
2. Review the parity, compatibility, and governance docs.
3. Decide later whether the next step should be official linking,
   co-maintenance, or full adoption.

This packet is intentionally narrower than an official transfer proposal. It is
meant to make the current state legible without pretending the handoff work is
already done.

# Roadmap

This roadmap tracks the community-first path from the current public Go client
to a Convex-adoptable official client.

## Current status

As of 2026-06-19, Milestone 0 is complete. This public repository is now the
source of truth for the module path, roadmap, issues, CI, tags, and releases.

- `github.com/cervantesh/convex-go` is the public module path.
- `v0.1.0` already exists as a public tag.
- GitHub Actions already covers cross-platform CI, release automation, live
  integration scaffolding, Sonar report generation, Dependabot, and CodeQL.
- The public issue tracker and versioned docs now live in this repository.

Historical incubation copies may still exist as archives, but they should not
receive forward-looking roadmap updates, release tags, or active maintenance.

Umbrella tracker: #19

## Frozen public contract

The normal user path stays centered on:

- `NewClient`
- `Query`
- `Mutation`
- `Action`
- `Subscribe`
- `Close`

The explicit clients remain public:

- `NewHTTPClient`
- `NewWebSocketClient`

`baseclient` remains the advanced path. `Subscribe` remains the canonical
realtime verb. New root-surface changes after the freeze milestone must be
additive or carry explicit pre-v1 migration notes.

## Milestone 0 - Public Source of Truth

Goal: keep the public repo as the only active source of truth for roadmap,
release, and automation work.

Completed in this repository:

- #20 Promote the public repo as the source of truth and retire incubation
  drift.

## Milestone 1 - SDK Freeze

Goal: close remaining parity gaps and freeze the public Go contract.

Completed in this repository:

- #21 Public auth callback parity on root clients
- #22 Public connection state observability on root and realtime clients
- #23 Rewrite public onboarding around a shorter README and stable doc split
- #24 Freeze the public SDK surface and deprecation policy pre-v1
- #25 Harden root and baseclient boundaries for advanced auth and sync exports
- #26 Polish pkg.go.dev and compiled examples for the frozen public API
- #27 Close the public parity gap in errors and auth contracts
- #28 Typed references and offline codegen v1
- #29 Define a public logging and observability story for the SDK

Remaining:

- None. Milestone 1 is complete.

## Milestone 2 - Runtime Reliability

Goal: prove runtime safety with live integration, fuzzing, soak coverage, leak
budgets, and performance baselines.

Completed in this repository:

- #31 Add fuzz targets for values, wire protocol, and critical conversions
- #35 Document and maintain a Go and backend compatibility matrix

Remaining:

- #30 Expand the live integration harness to full request, auth, and reconnect
  coverage
- #32 Add deterministic soak coverage for reconnect, auth refresh, and
  cancellation
- #33 Add leak and retention budget tests for goroutines, watchers, and result
  history
- #34 Add benchmarks and performance budgets for values, subscribe throughput,
  and reconnect
- #36 Feed live harness outcomes back into offline conformance fixtures

## Milestone 3 - Adoption

Goal: reduce friction for new Go adopters and gather external proof.

Completed in this repository:

- #37 Expand public recipes into a complete Go cookbook
- #38 Publish migration guides from convex-js, convex-rs, and convex-py
- #39 Publish a public demo app that uses the SDK as an application dependency
- #40 Add CI smoke coverage for demos and public examples
- #41 Define community operations, templates, and triage cadence for pre-v1
  support

Remaining:

- #42 Run an external adopter validation program for the Go client

## Milestone 4 - Convex Adoption Readiness

Goal: prepare a clean, shareable case for Convex adoption without changing the
module path yet.

Completed in this repository:

- #43 Create a Convex adoption packet for the Go client
- #44 Define governance and maintainer policy for a Convex-adoptable Go client
- #45 Draft a namespace transition readiness plan for
  `github.com/get-convex/convex-go`
- #46 Request official linking as the recommended community Go client
- #47 Draft the official adoption proposal and ownership options for Convex
- #48 Define the handoff gate that must be met before official adoption work
  begins

Remaining:

- None. Milestone 4 is complete.

## Milestone 5 - Official Handoff

Goal: execute the official handoff only after Convex explicitly agrees to
adopt the client.

- #49 Choose the handoff form after Convex accepts adoption
- #50 Execute the module path migration to `github.com/get-convex/convex-go`
  when approved
- #51 Publish transition releases under the legacy and official namespaces
- #52 Update docs, demos, and official links after handoff
- #53 Publish the post-handoff v1 roadmap for the official Go client

## Working rules

- The strategy remains community-first until Convex explicitly accepts adoption
  work.
- Milestone 5 stays blocked on Convex agreement plus the Milestone 4 handoff
  gate.
- Work lands through issues and PRs; avoid direct pushes to `main`.
- Public claims about parity or readiness must stay backed by versioned docs,
  tests, and public tracker evidence.

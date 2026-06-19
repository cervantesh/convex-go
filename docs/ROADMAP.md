# Roadmap To An Official Convex Go Client

Date baseline: June 17, 2026.

Convex still documents Go through the OpenAPI / other-languages path rather
than as a first-party client. This roadmap keeps the strategy community-first:
build the strongest public Go client first, then make official adoption easy if
Convex chooses it.

## Public API Freeze

The main public path is:

- `convex.NewClient`
- `Client.Query`
- `Client.Mutation`
- `Client.Action`
- `Client.Subscribe`
- `Client.Close`

Explicit clients remain public and supported:

- `convex.NewHTTPClient`
- `convex.NewWebSocketClient`

The advanced package remains public but secondary:

- `github.com/cervantesh/convex-go/baseclient`

## Milestone 0 - Public Truth

Goal: remove credibility blockers in the public repository surface.

Completed in this repository:

- module path, README, and repository naming now align on
  `github.com/cervantesh/convex-go`
- `v0.1.0` release metadata, release workflow, and annotated tag flow exist in
  the repository
- Sonar, CodeQL, Dependabot, dependency review, and secret scanning now live in
  GitHub-managed workflows and settings

## Milestone 1 - API Stability

Goal: make the SDK adoptable without constant drift.

Completed in this repository:

- API surface snapshot and repohealth guards for the root package and
  `baseclient`
- connection-state observability in the root client and explicit realtime client
- public auth callback for root and realtime clients
- README, `pkg.go.dev`, and public docs split into concise onboarding plus
  deeper usage docs

## Milestone 2 - Runtime Confidence

Goal: prove the runtime behaves like a product, not just a package.

Completed in this repository:

- coverage, race, shuffle, vet, vulncheck, lint, and Sonar report generation in
  CI
- deterministic subscription soak coverage and connection-state tests
- live integration workflow for a sample Convex deployment

Remaining:

- #30 Expand the live integration harness to full request, auth, and reconnect
  coverage
- #31 Add fuzz targets for values, wire protocol, and critical conversions
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
- #41 Define community operations, templates, and triage cadence for pre-v1
  support

Remaining:

- #40 Add CI smoke coverage for demos and public examples
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

## Milestone 5 - Official Handoff

Goal: execute an official transfer only after Convex accepts it.

These items stay open until an external agreement exists:

- #49 Choose the handoff form after Convex accepts adoption
- #50 Execute the module path migration to
  `github.com/get-convex/convex-go` when approved
- #51 Publish transition releases under the legacy and official namespaces
- #52 Update docs, demos, and official links after handoff
- #53 Publish the post-handoff v1 roadmap for the official Go client

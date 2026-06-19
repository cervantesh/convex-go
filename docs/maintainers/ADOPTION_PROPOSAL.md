# Convex Adoption Proposal

This guide frames the formal adoption conversation for Convex after the
community-first stage has proven out.

It is not a handoff trigger. The current repository remains the canonical
community home at `github.com/cervantesh/convex-go` until Convex explicitly
chooses an ownership path.

## What Convex Would Receive Today

If Convex evaluates the project now, the package already includes:

- a frozen public root API centered on `NewClient`, `Query`, `Mutation`,
  `Action`, `Subscribe`, and `Close`
- public docs for parity, compatibility, conformance, architecture, support,
  governance, and publication workflow
- GitHub Actions, CodeQL, Dependabot, release automation, and tracked issues
- a clear line between community status now and official adoption later

## Adoption Options

### Option 1: Official Link Only

Convex links `github.com/cervantesh/convex-go` as the recommended community Go
client but does not take ownership.

- Lowest change risk
- No `go.mod` or import-path migration
- Good fit while runtime hardening and adopter validation are still open
- Not a first-party support commitment

### Option 2: Co-Maintained Official Repository

Convex creates or owns `github.com/get-convex/convex-go`, but keeps the current
maintainers involved and shares review or release responsibility.

- Creates a clear official namespace
- Lets Convex add CODEOWNERS and release controls gradually
- Requires a module-path migration plan and public migration guide
- Works best after the handoff gate is satisfied

### Option 3: New Official Repository From Snapshot

Convex seeds a fresh official repository from a reviewed snapshot rather than
transferring the current public repository.

- Keeps the official history clean and purpose-built
- Makes governance, issue templates, and release ownership easy to reset
- Requires careful carry-over of docs, tags, and release notes
- Still requires a module-path change to `github.com/get-convex/convex-go`

### Option 4: Full Transfer Of The Existing Repository

Convex takes ownership of the existing repository and its ongoing history.

- Preserves current issue and PR history in one place
- Simplifies discoverability for existing watchers
- Pulls private incubation-era noise and old workflow assumptions forward
- Still requires clear governance reset and possibly a module-path migration

## Recommendation

The recommended phased path is:

1. Start with official linking only.
2. Complete the handoff gate and external validation work.
3. Revisit either co-maintenance or a new official repository under
   `github.com/get-convex/convex-go`.

That keeps the current ask proportional to the current evidence.

## Decision Request To Convex

When this proposal is sent, Convex should answer:

- whether the near-term step is link-only or real ownership work
- whether the preferred official end state is co-maintained, new repo, or full
  transfer
- who owns release authority, support routing, and security intake
- whether `docs.convex.dev` should point to the community repo now or wait for
  a later handoff

## Technical Impacts To Acknowledge

Any adoption path beyond official linking should explicitly plan for:

- `go.mod` module-path migration
- import updates in docs, examples, and generated references
- release/tag continuity between the legacy and official namespaces
- updated links in `README.md`, `pkg.go.dev`, and Convex docs

## Non-Goals

- Do not promise an adoption date from this document.
- Do not imply that official adoption is required for community use today.
- Do not treat ownership transfer as a substitute for runtime reliability work.

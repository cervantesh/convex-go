# Official Handoff Gate

This gate defines what must be true before any official-adoption execution work
begins.

Passing this gate does not itself transfer the repository or change the module
path. It only means the project is ready to have that conversation without
hand-waving.

## Hard Preconditions

Nothing in Milestone 5 should begin until both of these are true:

1. Convex explicitly agrees to evaluate real adoption work.
2. Every checklist item below has public evidence in this repository.

If either condition is missing, the official-handoff issues stay blocked.

## Gate Checklist

### 1. Public Source Of Truth

- `github.com/cervantesh/convex-go` remains the canonical public repository.
- Tags, releases, roadmap, and issue tracking are current.
- GitHub Actions, CodeQL, Dependabot, and release automation are already live.

Primary evidence:

- `README.md`
- `CHANGELOG.md`
- `docs/ROADMAP.md`
- `.github/workflows/`

### 2. Public SDK Contract And Docs

- The root API and advanced boundaries are documented and stable enough for
  outside adopters.
- Parity, compatibility, conformance, and architecture docs are current.
- Pre-v1 break policy is documented for any remaining justified changes.

Primary evidence:

- `docs/PARITY.md`
- `docs/COMPATIBILITY.md`
- `docs/CONFORMANCE.md`
- `docs/ARCHITECTURE.md`
- `docs/maintainers/GOVERNANCE.md`

### 3. Runtime Reliability Evidence

These roadmap items should be closed before the handoff gate is called passed:

- `#30` live integration harness coverage
- `#31` fuzz targets
- `#32` deterministic soak coverage
- `#33` leak and retention budgets
- `#34` benchmarks and performance budgets
- `#36` feed live outcomes back into offline fixtures

The point is simple: adoption should not outrun runtime proof.

### 4. Adoption Readiness Evidence

These items should also be closed before the gate passes:

- `#37` cookbook
- `#38` migration guides
- `#39` public demo app
- `#40` demo/example CI smoke coverage
- `#42` external adopter validation
- `#43` adoption packet
- `#44` governance policy
- `#45` namespace transition readiness
- `#46` official linking request
- `#47` adoption proposal

### 5. Namespace And Release Readiness

Before handoff work starts, maintainers must be able to show:

- a reviewed plan for `github.com/get-convex/convex-go`
- a clear `go.mod` migration story
- release and changelog handling for both legacy and official namespaces
- updated support and security contacts for the target ownership model

Primary evidence:

- `docs/maintainers/NAMESPACE_TRANSITION.md`
- `docs/maintainers/RELEASE.md`
- `docs/maintainers/ADOPTION_PROPOSAL.md`
- `docs/maintainers/OFFICIAL_LINKING_REQUEST.md`

## Pass Rule

The gate passes only when:

- Convex explicitly says to proceed
- Milestone 2 reliability issues are closed
- Milestone 3 adoption issues are closed
- Milestone 4 readiness issues are closed
- the evidence above is public, current, and green in GitHub-hosted checks

## Non-Goals

- Do not use this gate to skip unresolved runtime work.
- Do not treat private notes or local runs as sufficient evidence.
- Do not open `#49` through `#53` as active implementation work before this
  gate is satisfied.

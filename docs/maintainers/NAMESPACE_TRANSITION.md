# Namespace Transition Readiness

This guide prepares a future move from `github.com/cervantesh/convex-go` to
`github.com/get-convex/convex-go` if Convex explicitly adopts the client.

It is a readiness plan only. Do not execute this migration while the project
remains community-first and community-owned.

## Current Position

- The authoritative module path today is `github.com/cervantesh/convex-go`.
- The public roadmap, issues, releases, and quality gates still live here.
- The current repository should remain the only maintained source until there
  is explicit Convex agreement on ownership and namespace.
- Because the module is still pre-v1, one path break is acceptable only if it
  is deliberate, documented, and paired with a migration guide.

## Preconditions

Do not start the namespace move until all of these are true:

1. Convex explicitly agrees to adopt or co-maintain the client.
2. The handoff gate from `ROADMAP.md` issue `#48` is satisfied.
3. The target repository, maintainers, and security contacts are decided.
4. The first official release notes and migration guide are drafted.
5. GitHub Actions, release automation, and support routing are ready in the
   target repository.

## Transition Invariants

These rules should stay fixed during the move:

- Do not rewrite public history in place.
- Do not force-push a hidden path change onto existing users.
- Keep existing tags under `github.com/cervantesh/convex-go` immutable.
- Leave the legacy repository readable with a clear migration notice.
- Publish one canonical official module path, not two competing active paths.
- Ship the first official release with explicit import-path migration steps.

## Decision Inputs From Convex

Before execution, maintainers need concrete answers from Convex on:

- whether the end state is a repo transfer, a new official repo, or
  co-maintenance under `get-convex`
- who owns release authority and CODEOWNERS after handoff
- which repository handles security intake and support routing
- what version should become the first official release
- where `docs.convex.dev` and `pkg.go.dev` should point on announcement day

## Staged Execution Plan

### 1. Freeze The Source Commit

- Pick the exact source commit to migrate.
- Confirm GitHub Actions is green on that commit.
- Confirm `CHANGELOG.md` and release notes match the intended transition.
- Confirm no unrelated open PR is required for the migration release.

### 2. Prepare The Target Repository

- Create or receive the target repository under `github.com/get-convex`.
- Mirror required GitHub settings: CI, CodeQL, Dependabot, issue templates,
  release workflow, and support docs.
- Decide whether historical issues stay here or are recreated in the official
  repository.

### 3. Change The Module Path

- Update `go.mod` to `github.com/get-convex/convex-go` on a dedicated branch.
- Update import paths in examples, docs, codegen output, and tests.
- Re-run the full release gate in the target repository before tagging.

### 4. Publish Transition Releases

- Publish a final legacy-path release that points users to the migration guide.
- Publish the first official release under the new namespace.
- Keep the release notes synchronized so users can compare old and new imports.

### 5. Update Public Entry Points

- Update `README.md`, `USAGE.md`, `ARCHITECTURE.md`, and `ADOPTION_PACKET.md`.
- Update links in support docs, demo repos, and any public release notes.
- Update `pkg.go.dev` references and announcement copy on the same day.

### 6. Monitor The Cutover

- Watch issues for import-path confusion and module-cache problems.
- Triage migration bugs separately from normal feature work.
- Keep a prominent notice in the legacy repository until adoption stabilizes.

## User Impact To Plan For

- `go get` does not transparently redirect module paths for users.
- Every import of `github.com/cervantesh/convex-go` must change.
- Generated references and copied examples must be refreshed.
- Any stale docs that still mention the legacy path will create support churn.

## Verification Checklist

Before announcing the move, confirm:

- the target repo is green in GitHub Actions
- the new module path builds and tests cleanly
- `README.md`, `CHANGELOG.md`, and the migration guide are published
- the legacy repo points to the official path and migration instructions
- support and security contacts are correct in both locations
- no workstation-specific scripts or paths are part of the handoff

## Non-Goals

- Do not execute the module-path change from this guide alone.
- Do not publish an official timeline before Convex confirms ownership.
- Do not treat a namespace move as proof that the runtime hardening work is
  complete.

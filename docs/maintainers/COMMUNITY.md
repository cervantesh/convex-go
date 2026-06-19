# Community Operations

This repository is pre-v1, community-maintained, and issue-driven. Community
operations should stay lightweight, explicit, and easy to follow without
private maintainer context.

## Intake Paths

Use the tracked GitHub templates for every normal intake path:

- `.github/ISSUE_TEMPLATE/bug_report.md` for reproducible SDK bugs
- `.github/ISSUE_TEMPLATE/docs.md` for onboarding, usage, and docs fixes
- `.github/ISSUE_TEMPLATE/feature_request.md` for parity and API work
- `.github/ISSUE_TEMPLATE/compatibility_fixture.md` for upstream-backed fixture
  additions
- `.github/PULL_REQUEST_TEMPLATE.md` for implementation PRs

Blank issues should stay disabled so new reports land in one of those shapes by
default.

## Routing Rules

- SDK bugs, parity gaps, docs fixes, and release blockers belong in GitHub
  issues in this repository.
- Security reports must follow [SECURITY.md](../../SECURITY.md), not public
  issues.
- General Convex product questions belong in the official Convex docs and
  community channels, not in this repository's bug tracker.

## Pre-v1 Triage Cadence

Use a simple weekly triage cadence:

1. Review new issues and PRs at least once per week.
2. Confirm that each new issue matches the right template and has enough
   reproduction or acceptance detail to act on.
3. Route product/support questions away from the SDK tracker when they are not
   Go client issues.
4. Re-check open release blockers before every prerelease or release cut.

This is a target cadence, not a hard SLA. The important part is that the public
tracker stays curated and current.

## Closing Rules

- Close fixed issues by merging a PR that links them with `Closes #...`.
- Close support-only or out-of-scope issues with a short redirect to
  [SUPPORT.md](../../SUPPORT.md) or the Convex community.
- Do not close compatibility or parity issues without public docs, tests, or
  fixture evidence that supports the claim.

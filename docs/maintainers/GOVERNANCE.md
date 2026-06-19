# Governance

This repository is a community-maintained, pre-v1 Go client for Convex. The
governance goal is to keep releases, support, and public API changes
predictable enough that the project could later move under a broader Convex
ownership model without hidden local process.

## Current Maintainer Model

- The current project maintainer is the repository owner, `@cervantesh`.
- `.github/CODEOWNERS` defines the default review owner for tracked files.
- Changes land through GitHub issues and pull requests. Do not push directly to
  `main` for normal feature or docs work.

## Merge Policy

- Every non-trivial change should link back to an issue.
- Pull requests must keep GitHub Actions green before merge.
- Use squash merges so the public history stays readable at pre-v1 velocity.
- If a pull request fixes an issue, close it with `Closes #...` in the PR body.

## Release Authority

- Only maintainers should dispatch releases.
- Releases must follow [RELEASE.md](RELEASE.md) and the GitHub-hosted quality
  gates in [QUALITY.md](QUALITY.md).
- `CHANGELOG.md` and `Version` in `client_options.go` are the public release
  contract.
- Only the latest pre-v1 release line is expected to receive fixes.

## Pre-v1 Breaking Change Policy

- Prefer additive changes in the root package.
- A pre-v1 breaking change is still allowed when it produces a clearly better
  Go API, but it must include:
  - a linked issue
  - updated API surface tests
  - updated public docs
  - migration notes in `CHANGELOG.md` or the PR body
- `baseclient` is advanced, but it is still public. Breaking it casually is not
  acceptable just because it is not the default path.

## Security And Support

- Public support routing lives in [SUPPORT.md](../../SUPPORT.md).
- Security handling lives in [SECURITY.md](../../SECURITY.md).
- Community intake and weekly triage expectations live in
  [COMMUNITY.md](COMMUNITY.md).

## Governance Changes

- Governance changes should be proposed in a GitHub issue and merged through a
  normal PR.
- Until Convex explicitly agrees to adopt or co-maintain the client, the
  project remains community-first under the current module path.

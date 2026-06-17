# Publication Workflow

This guide is for cutting a clean public snapshot that keeps the public history
small, the docs readable, and the repository free of workstation-specific
artifacts.

## Preconditions

Before exporting:

1. Merge or defer the issues intended for the public baseline.
2. Confirm GitHub Actions from [QUALITY.md](QUALITY.md) is green on the source
   commit. If `SONAR_TOKEN` is configured, confirm the GitHub-hosted Sonar step
   is green too.
3. Confirm the public docs are accurate:
   - [../ARCHITECTURE.md](../ARCHITECTURE.md)
   - [../CONFORMANCE.md](../CONFORMANCE.md)
   - [../PARITY.md](../PARITY.md)
   - [../RECIPES.md](../RECIPES.md)
4. Confirm local-only paths stay untracked:
   - `tools/`
   - `tmp/`
   - `.cervomut/`
   - `cervomut.yaml`

## Export A Clean Snapshot

Use the Go maintainer tool so the export path stays cross-platform:

```sh
go run ./cmd/convex-go-maint export-snapshot \
  -out ../convex-go-public \
  -git-init \
  -initial-branch main \
  -commit-message "Initial public snapshot"
```

This command copies only Git-tracked files into the destination, initializes a
fresh Git repository, and creates one initial commit.

If you have moved or renamed tracked files locally, stage or commit those
changes before exporting so the Git-tracked snapshot matches the working tree.

## Verify The Exported Tree

Push the exported tree to a staging repository or branch and let GitHub Actions
validate it there. Treat the GitHub-hosted `CI` workflow and Sonar job as the
publication gate, not local-only verification.

## Bootstrap The Public Repository

After review, add the new remote and push:

```sh
git remote add origin <new-repository-url>
git push -u origin main
```

Then seed the repository with:

1. the issue templates already tracked under `.github/ISSUE_TEMPLATE/`
2. a small curated backlog for the next public milestone
3. the first annotated tag and GitHub release notes

Prefer creating a clean public snapshot over rewriting history in place.

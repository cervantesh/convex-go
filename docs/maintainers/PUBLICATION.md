# Publication Workflow

This guide is for cutting a clean public snapshot that keeps the public history
small, the docs readable, and the repository free of workstation-specific
artifacts.

## Preconditions

Before exporting:

1. Merge or defer the issues intended for the public baseline.
2. Run the quality gates from [QUALITY.md](QUALITY.md).
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

Run the public baseline checks inside the exported directory:

```sh
cd ../convex-go-public
go test ./... -count=1
go test ./... -race -count=1
go vet ./...
```

If the release requires lint, coverage, or Sonar reports, run the full gates
from [QUALITY.md](QUALITY.md) before publishing.

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

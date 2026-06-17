# Release Checklist

1. Confirm open issues and pull requests intended for the release are closed or
   explicitly deferred.
2. Confirm the target commit is green in GitHub Actions from
   [QUALITY.md](QUALITY.md). If `SONAR_TOKEN` is configured, confirm the
   GitHub-hosted Sonar step is green too.
3. Confirm `Version` in `client_options.go` matches the planned tag.
4. Confirm `CHANGELOG.md` has release notes for the tag.
5. Confirm README and package docs describe known limits.
6. Run the `Release` GitHub Actions workflow with the version you intend to
   publish. The workflow validates `Version`, `CHANGELOG.md`, tag absence, and
   extracts release notes from the matching changelog section.
7. If the version contains a hyphen such as `0.2.0-rc.1`, the workflow
   publishes a pre-v1 prerelease. Otherwise it publishes a normal release.
8. Confirm the created GitHub release matches the changelog notes and links to
   the intended target ref.

## Automated Release Workflow

The repository includes a manual `Release` workflow in
`.github/workflows/release.yml`.

Inputs:

- `version`: release version without the leading `v`
- `target`: branch or commit to tag and release; defaults to `main`

Expected changelog shape:

```text
## 0.1.1 - 2026-06-17

- Release note line one.
- Release note line two.
```

The workflow is intentionally built on top of the checklist instead of
replacing it with hidden automation. Maintainers should still review the
release notes, known limits, and linked issues before dispatching it.

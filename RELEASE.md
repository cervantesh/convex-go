# Release Checklist

1. Confirm open issues and pull requests intended for the release are closed or
   explicitly deferred.
2. Run the full quality gates from `docs/QUALITY.md`.
3. Confirm `Version` in `client_options.go` matches the planned tag.
4. Confirm `CHANGELOG.md` has release notes for the tag.
5. Confirm README and package docs describe known limits.
6. Create an annotated tag after maintainer approval.
7. Publish GitHub release notes with compatibility notes and known limits.

# Maintainer Guides

This index is for maintainers preparing releases, hardening quality gates, or
cutting a clean public snapshot from the repository without carrying local-only
artifacts or private incubation history.

This public repository is the active source of truth for roadmap updates,
tracked issues, releases, and GitHub automation.

Normal SDK users should start with:

- [README.md](../README.md)
- [ADOPTION_PACKET.md](ADOPTION_PACKET.md)
- [USAGE.md](USAGE.md)
- [COMPATIBILITY.md](COMPATIBILITY.md)
- [ARCHITECTURE.md](ARCHITECTURE.md)
- [CONFORMANCE.md](CONFORMANCE.md)
- [PARITY.md](PARITY.md)
- [RECIPES.md](RECIPES.md)

Maintainer workflow docs:

- [ROADMAP.md](ROADMAP.md)
- [maintainers/ADOPTION_PROPOSAL.md](maintainers/ADOPTION_PROPOSAL.md)
- [maintainers/COMMUNITY.md](maintainers/COMMUNITY.md)
- [maintainers/GOVERNANCE.md](maintainers/GOVERNANCE.md)
- [maintainers/DEVELOPMENT.md](maintainers/DEVELOPMENT.md)
- [maintainers/HANDOFF_GATE.md](maintainers/HANDOFF_GATE.md)
- [maintainers/OFFICIAL_LINKING_REQUEST.md](maintainers/OFFICIAL_LINKING_REQUEST.md)
- [maintainers/PERFORMANCE.md](maintainers/PERFORMANCE.md)
- [maintainers/QUALITY.md](maintainers/QUALITY.md)
- [maintainers/LIVE_INTEGRATION.md](maintainers/LIVE_INTEGRATION.md)
- [maintainers/MUTATION_TESTING.md](maintainers/MUTATION_TESTING.md)
- [maintainers/NAMESPACE_TRANSITION.md](maintainers/NAMESPACE_TRANSITION.md)
- [maintainers/PUBLICATION.md](maintainers/PUBLICATION.md)
- [maintainers/RELEASE.md](maintainers/RELEASE.md)

These guides should stay shell-neutral and workstation-neutral.
Cross-platform automation belongs in `cmd/convex-go-maint`, not in local-only
scripts.

GitHub-native repository automation should stay in tracked config under
`.github/`, including Dependabot and CodeQL.

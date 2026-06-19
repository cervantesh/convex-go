# Official Linking Request

This guide packages the modest ask for Convex to link this repository as the
recommended community Go client without changing ownership, namespace, or
support boundaries.

The request is intentionally narrower than official adoption. The client stays
community-maintained at `github.com/cervantesh/convex-go` unless Convex later
chooses a stronger handoff path.

## Request Goal

Ask Convex to publicly point Go users to this repository as the community Go
client for Convex.

That request should not imply:

- first-party ownership
- a promise to move to `github.com/get-convex/convex-go` immediately
- a commitment from Convex to support every issue directly

## When To Send It

Send the request only when these are already true:

1. The latest release and GitHub Actions checks are green.
2. The public docs are current: `README.md`, `PARITY.md`, `COMPATIBILITY.md`,
   and `ADOPTION_PACKET.md`.
3. Governance, support routing, and security contacts are public.
4. The namespace transition plan exists, but remains clearly deferred.
5. The request can point to a stable tag or release instead of an arbitrary
   draft branch.

## Evidence Bundle

Include links to:

- `docs/ADOPTION_PACKET.md`
- `docs/PARITY.md`
- `docs/COMPATIBILITY.md`
- `docs/CONFORMANCE.md`
- `docs/maintainers/GOVERNANCE.md`
- `SUPPORT.md`
- `SECURITY.md`

The point is to show that the repository already has a clear API story,
quality gates, support routing, and documented limits.

## Exact Ask To Convex

The recommended ask is:

1. Link `github.com/cervantesh/convex-go` from the Go or other-languages path
   in public Convex docs.
2. Describe it as the community Go client for Convex.
3. Keep the wording clear that it is not yet an official first-party client.
4. Revisit stronger adoption options only after more runtime hardening and user
   validation are complete.

## Suggested Outreach Note

Use a short note that stays factual:

> We now maintain a public, community Go client for Convex at
> `github.com/cervantesh/convex-go`. The repo has public CI, compatibility and
> conformance docs, governance, support routing, and release automation. We
> are not asking for a transfer or official ownership yet. The current ask is
> simply to link it as the recommended community Go client for Go users.

## Success Criteria

Treat the request as successful only if:

- Convex links the repository publicly
- the linked wording does not overstate ownership or support
- the docs still point to the current canonical module path
- a follow-up issue records the exact linked location and wording

## Non-Goals

- Do not treat official linking as official adoption.
- Do not change `go.mod` or the import path as part of this request.
- Do not promise timeline or ownership outcomes on Convex's behalf.

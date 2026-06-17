# Mutation Testing

This is a maintainer-facing document. CervoMutant is used as an extra quality
signal, not as a replacement for coverage, race tests, vet, lint, or offline
conformance fixtures.

Examples below use `cervomut` as a placeholder command for the local CervoMutant executable available in your environment.

## Commands

Use the fast deterministic sample for PR feedback:

```text
cervomut run ./... --policy ci-fast --max-mutants 100 --sample deterministic --workers 2 --isolation overlay --coverage-prefilter --max-process-memory-mb 6144 --report json,summary,junit --out .cervomut/reports-ci-fast
```

Use the full campaign for periodic audit work:

```text
cervomut run ./... --policy campaign --workers 2 --isolation overlay --coverage-prefilter --max-process-memory-mb 6144 --report json,summary,junit --out .cervomut/reports-campaign
```

Do not make full campaign a required CI gate yet.

## Policy

- Keep `ci-fast >= 80%` unless the baseline is intentionally reset with
  evidence in the PR.
- Use the full `campaign` for periodic audits, risky package refactors, and
  pre-release quality review.
- Do not change production behavior only to improve the mutation score.
- Keep package-specific score targets and transient survivor notes in the active
  issue or PR, not in this document.

## Review Order

Review findings in this order:

1. Timed out mutants.
2. Not covered mutants.
3. Low and medium-risk survivors such as arithmetic, conditionals, logical
   operators, and collection boundaries.
4. Literal and string survivors when they express real public behavior.
5. High-risk mutators such as `nil-checks`, `returns`, and `loop-control` only
   when a test can describe stable SDK semantics.

## reviewed-skip Rules

Use `reviewed-skip` only when the mutant is equivalent, tool-limited,
platform-sensitive, or timeout-specific under the mutation harness.

Every `reviewed-skip` entry should record:

- the mutated location or pattern
- the scope or package
- the reason the mutant is not worth forcing into a brittle test
- the evidence that the real public behavior is already covered

Template:

| mutant pattern | scope | disposition | reason | evidence |
| --- | --- | --- | --- | --- |
| `<fill in>` | `package/path` | `reviewed-skip` | Explain why a stronger assertion would be artificial, equivalent, or unsafe. | Link to the test, report, or PR discussion that justifies the skip. |

## Reporting

Mutation-testing PRs should include the scoped command that was run, a short
before/after summary, any accepted `reviewed-skip` entries, and the regular
verification commands from [QUALITY.md](QUALITY.md).

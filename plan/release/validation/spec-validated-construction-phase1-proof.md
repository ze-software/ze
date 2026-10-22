# Spec: validated-construction-phase1-proof

| Field | Value |
|-------|-------|
| Status | design |
| Scope | validation |
| Priority | P2 |
| Release reason | Delivered Phase 1 validated-construction code still owes functional and long-gate evidence. |
| Depends | - (Phase 1 implementation and prerequisites are recorded landed) |
| Phase | Phase 1 proof remainder only |
| Updated | 2026-10-22 |

## Task

Complete the still-owed proof recorded in the 2026-10-11 Phase 1 closure of
`plan/post-release/engineering/spec-validated-construction-and-state-types.md`.
That parent retains the complete census, owner decisions D-1..D-7, compiler
probes, unit results, closure and review history. Their delivered-phase
outstanding gate entries are non-owning references to this child.

## Acceptance criteria and proof obligations

| Item | Required evidence |
|------|-------------------|
| Scoped lint | `./le go lint run scope` over the 197 package directories touched by Phase 1, including outstanding C-T2c/4c, 4d and 4e scopes. The recorded attempts were refused for changing lint inputs, including the six-package 4c attempt; run against a quiet input set and record the result, not a historical refusal as success. |
| Unit coverage | `./le test unit all`, including the unfinished batch-1 web, `cmd/ze/hub` and plugin/server runs. Preserve the distinction between these owed runs and the 13 named scoped tests already passing at closure. |
| Functional entry point | The `ui` functional suite, including `test/ui/cli-argument-refused-plugin-route.ci`, proving R9 through `Dispatcher.routeToProcess`. The fixture existed but was not re-run at closure; compiler or unit evidence does not substitute for this run. |
| Integrated gates | `./le --update`, then `./le verify worktree`; no Phase 1 batch had this complete run recorded. Record actual results, skips and any blockers rather than treating the split as proof. |

## Evidence preserved and exclusions

At HEAD `7689a86b74`, all 13 named scoped tests passed through `./le job run`,
and `./le repo compiles check` passed every shipped flavor. P-1..P-6 compiler
probes and per-route red/green evidence stay in the parent. The independent
review ended clean in round 3 and I5 was resolved by the owner on 2026-10-11.
This split neither reopens that recorded implementation closure nor claims
that it discharged the explicitly outstanding runs.

The closure's `./le rfc check` found no issue in its three scoped stems but
exited 2 on six unrelated findings; its exact list remains in the parent.
The commit-audit approval-reader limitation remains in the named journal,
not a newly invented defect or a requirement to alter RFC artifacts here.
Phase 1 changes no wire format and owes no interop scenario under its recorded
internal-refactor exemption.

The parent's later full census, IKE migrations, remaining later-phase ACs,
`command.Node`/SDK exclusions and review-only T2 residual bypasses remain
unauthorized engineering work. Upstream-dependent goyang patch retirement
also stays there. The proof child does not commission any of those phases,
lift owner pauses, change compiler/unit evidence or promote release readiness.

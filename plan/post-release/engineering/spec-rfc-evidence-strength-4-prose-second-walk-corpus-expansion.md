# Spec: rfc-evidence-strength-4-prose-second-walk-corpus-expansion

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | `spec-rfc-evidence-strength-4-prose-second-walk.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |
| Priority | P3 |
| Release reason | Retained later capability or corpus expansion; shipped correctness and required release proof stay with the companion. |

## Task

Own the retained wider corpus/enrolment/evidence programme outside the required release slice. The release population is requirement-level: behavior of a shipped capability, accepted configuration, a public support claim, and evidence used to justify any of them. A partially implemented RFC does not exempt its shipped subset. Missing optional capabilities and systematic expansion beyond that population belong to this post-release spec, not its release-validation companion. Preserve every permanent requirement id, existing tag, disposition, evidence record and historical count; reconcile the dated populations before execution. No defect in a shipped producer, known unsound proof, or missing required proof may be assigned to the optional slice merely because it was found during a corpus walk. This planning split changes no RFC verdict or permission to weaken a claim. The release owner implements quote validation, blind-walk/comparison tooling and truthful aggregate reporting once. This post-release spec owns additional prose-corpus second walks only. D-6’s rfc2119-register expansion remains outside this child, as originally decided; D-8’s independent/human calibration is retained for every executed tranche.

## Acceptance Criteria

Original criterion ids from `plan/release/validation/spec-rfc-evidence-strength-4-prose-second-walk.md` are retained for this slice only; read the full inherited design and dated evidence in its Historical source record. Where a criterion spans both slices, the ownership table below is the exact boundary.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-5 | a gated row neither walk can tie to a sentence | reported as "no source sentence"; handled under D-5 in the same commit as the second-walk record |
| AC-6 | each of the 73 prose stems | its artifact carries a second-walk record: walker, date, agreement counts, and one adjudication line for each disagreement; every unsourced row carries a verified quote |

## Implementation Steps

Reconcile every original row not assigned to the release slice, including later and pending-decision work, and execute the inherited method only with its original approvals and prerequisites. Consume shared tooling from the release owner; do not reimplement it. Full-programme totals are the union of both owners, never a declaration that the release subset completes the corpus.

## Work Not Done

The companion retains all other obligations and the complete historical source record. This split is neither closure nor a claim of missing implementation; original approvals, dependencies, artifact requirements and proof standards remain in force.

| Original obligation | Implementation owner | Boundary |
|---------------------|----------------------|----------|
| Listed criteria: shipped-behavior/support-claim proof and shared gate/tooling work | `plan/release/validation/spec-rfc-evidence-strength-4-prose-second-walk.md` | Required release population; existing code-defect owners remain responsible for fixes |
| Listed criteria: absent-capability or wider-corpus expansion only | `plan/post-release/engineering/spec-rfc-evidence-strength-4-prose-second-walk-corpus-expansion.md` | Every inherited remainder outside the release population; no automatic feature implementation |
| Historical counts, full-programme milestones and pending owner decisions | `plan/release/validation/spec-rfc-evidence-strength-4-prose-second-walk.md and plan/post-release/engineering/spec-rfc-evidence-strength-4-prose-second-walk-corpus-expansion.md` | Account for the union without duplicate implementation; neither split alone closes the original full programme |

## Required Reading

- `plan/release/validation/spec-rfc-evidence-strength-4-prose-second-walk.md`: full original source record, design, tests, decisions and evidence; apply them to this file’s owned slice only.

# Spec: followup-rfc-enrollment-corpus-expansion

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | `spec-followup-rfc-enrollment.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |
| Priority | P3 |
| Release reason | Retained later capability or corpus expansion; shipped correctness and required release proof stay with the companion. |

## Task

Own the retained wider corpus/enrolment/evidence programme outside the required release slice. The release population is requirement-level: behavior of a shipped capability, accepted configuration, a public support claim, and evidence used to justify any of them. A partially implemented RFC does not exempt its shipped subset. Missing optional capabilities and systematic expansion beyond that population belong to this post-release spec, not its release-validation companion. Preserve every permanent requirement id, existing tag, disposition, evidence record and historical count; reconcile the dated populations before execution. No defect in a shipped producer, known unsound proof, or missing required proof may be assigned to the optional slice merely because it was found during a corpus walk. This planning split changes no RFC verdict or permission to weaken a claim. MCP non-IETF enrolment remains an owner decision, not authorization to enrol or decline it. AC-6 stays with the existing public-row child. The SHOULD-level prerequisite remains the full RFC-only MUST milestone across both owners, not just release-slice completion.

## Acceptance Criteria

Original criterion ids from `plan/release/validation/spec-followup-rfc-enrollment.md` are retained for this slice only; read the full inherited design and dated evidence in its Historical source record. Where a criterion spans both slices, the ownership table below is the exact boundary.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The reconciled RFC-only enrolment population | Every eligible summary is enrolled through Meta, every source MUST-level obligation is captured, and `./le rfc check` accepts its proof or owner-authorised disposition; every excluded stem has an explicit owner decision. This is the MUST-enrolment prerequisite for the SHOULD-level child |
| AC-2 | A source obligation is absent from a summary, including the historical zero-capture set | The source walk captures it before enrolment is credited; no historical missing-summary item disappears through a changed count |
| AC-3 | The inherited extraction backlog, including the wider set outside the Supported child | Every obligation is accounted for by an accepted current source walk; the Supported child meets its own contract, and the inherited source-inventory stability assumption has a recorded closure decision |
| AC-4 | Tested requirements in the inherited audit-freshness backlog | Every required audit verdict is enforced and fresh against the final tagged tests; absence of an audit artifact is outstanding work, even if a gate skips it |
| AC-5 | The inherited `{gap}` / `{not-applicable}` annotation population | Every annotation is re-derived from the source and producing code under current compliance rules; invalid dispositions are replaced by implementation and proof or an explicit owner-authorised decision |
| AC-7 | MCP's non-IETF enrolment question | Thomas's decision is recorded, and every resulting obligation has completed proof or an approved live owner before this umbrella closes |

## Implementation Steps

Reconcile every original row not assigned to the release slice, including later and pending-decision work, and execute the inherited method only with its original approvals and prerequisites. Consume shared tooling from the release owner; do not reimplement it. Full-programme totals are the union of both owners, never a declaration that the release subset completes the corpus.

## Work Not Done

The companion retains all other obligations and the complete historical source record. This split is neither closure nor a claim of missing implementation; original approvals, dependencies, artifact requirements and proof standards remain in force.

| Original obligation | Implementation owner | Boundary |
|---------------------|----------------------|----------|
| Listed criteria: shipped-behavior/support-claim proof and shared gate/tooling work | `plan/release/validation/spec-followup-rfc-enrollment.md` | Required release population; existing code-defect owners remain responsible for fixes |
| Listed criteria: absent-capability or wider-corpus expansion only | `plan/post-release/engineering/spec-followup-rfc-enrollment-corpus-expansion.md` | Every inherited remainder outside the release population; no automatic feature implementation |
| Historical counts, full-programme milestones and pending owner decisions | `plan/release/validation/spec-followup-rfc-enrollment.md and plan/post-release/engineering/spec-followup-rfc-enrollment-corpus-expansion.md` | Account for the union without duplicate implementation; neither split alone closes the original full programme |

## Required Reading

- `plan/release/validation/spec-followup-rfc-enrollment.md`: full original source record, design, tests, decisions and evidence; apply them to this file’s owned slice only.

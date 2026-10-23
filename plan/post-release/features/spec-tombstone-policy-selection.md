# Spec: tombstone-policy-selection

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | Per-peer and group strip or propagate selection is absent; restoring the inherited forwarding default remains a release repair. |
| Depends | `plan/release/fixes/spec-tombstone-forwarding-policy.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Implement per-peer and per-group inherit/strip/propagate selection with neighbor precedence, physical marker removal for strip, and the specified Transitive/Partial handling for propagate. Reuse the corrected default forwarding behavior, preserve shared received-wire ownership and the zero-copy no-marker fast path. The original design decisions and constraints remain binding.

## Source and Evidence

Split from `plan/release/fixes/spec-tombstone-forwarding-policy.md` under the release-scope and spec-split rules in `ai/META-DECISIONS.md`. The complete original design, decisions, evidence and implementation history are preserved under Historical Combined Specification in `plan/release/fixes/spec-tombstone-forwarding-policy.md`. Read that history before design; no historical claim is freshly verified by this migration. The source lifecycle is retained on the original identity; this new work item begins as a skeleton, not an inferred readiness promotion.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-2 | Policy `strip`, marker-bearing UPDATE forwarded | No marker in the forwarded UPDATE at all — rebuilt, not masked (draft Section 5.3: "If complete removal is required, the implementation MUST rebuild the path attributes") |
| AC-3 | Policy `propagate`, non-transitive marker | Transitive bit SET before forwarding ("the implementation MUST set the Transitive bit before forwarding") |
| AC-4 | Policy `propagate`, marker with Partial set by an upstream non-recognizing speaker | Partial CLEARED ("MUST clear the Partial bit (setting it to 0), even if it was set by an intermediate non-recognizing speaker") |
| AC-5 | Policy configured per peer-group; a neighbor overrides it | The neighbor value wins ("The policy SHOULD be configurable per peer-group or per neighbor") |
| AC-8 | UPDATE carrying no marker, any policy | Zero-copy fast path unchanged; no rebuild, no added allocation |

## Work Not Done and Split Ownership

The 2026-10-22 portfolio split changes planning ownership only. Status, existing dependencies and recorded evidence are retained; no implementation, verification or closure is claimed. Original criteria below the historical boundary are provenance, not a second active owner. Historical implementation steps apply only to the owner assigned in this table. Every test, documentation and review obligation follows its behavior owner.

| Work | Implementation owner |
|------|----------------------|
| Original AC-1, AC-6, AC-7 and AC-8 with no policy configured; default repair on both live rails | `plan/release/fixes/spec-tombstone-forwarding-policy.md` |
| Original AC-2 through AC-5 and AC-8 under explicit policy; configurable inherit and override surface | `plan/post-release/features/spec-tombstone-policy-selection.md` |

Work assigned to `plan/release/fixes/spec-tombstone-forwarding-policy.md` is not performed or closed by this spec. Shared mechanisms are reused, not independently reimplemented. No feature is declared complete by the release repair or validation disposition.

## Implementation Ownership

Design and implement only the behavior assigned above. Original wiring, functional, interop, documentation and review obligations for that behavior transfer here with the original criteria. Resolve the recorded open design decisions before readiness; the portfolio move itself selects no new product behavior.

# Spec: rsvp-te-one-to-one-config-fidelity

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P1 |
| Release reason | Already accepted one-to-one backup configuration must not silently promise protection the engine does not provide. |
| Depends | mpls-4-rsvp-te-fast-reroute (facility backup), mpls-3-rsvp-te (closed), mpls-1-kernel (closed) (inherited from the combined spec) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Resolve the existing accepted-but-ineffective `backup one-to-one` contract independently of implementing a new FRR mode. Establish the live acceptance and protection report path, then obtain the behavior disposition needed to make configuration and reporting honest. Explicit refusal/removal versus implementation is a design decision, not a change authorized by this migration. Existing facility backup and link-protection fallback remain separate.

## Source and Evidence

Split from `plan/post-release/features/spec-mpls-9-rsvp-te-one-to-one-backup.md` under the release-scope and spec-split rules in `ai/META-DECISIONS.md`. The complete original design, decisions, evidence and implementation history are preserved under Historical Combined Specification in `plan/post-release/features/spec-mpls-9-rsvp-te-one-to-one-backup.md`. Read that history before design; no historical claim is freshly verified by this migration. The source lifecycle is retained on the original identity; this new work item begins as a skeleton, not an inferred readiness promotion.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-R1 | Configure one-to-one on a build without detour support | Configuration cannot silently succeed while presenting protection as effective; the approved disposition is explicit and tested |
| AC-R2 | Existing facility-backup configuration | Its behavior and diagnostics are preserved |
| AC-R3 | Scope disposition | The absent detour feature retains all original ACs and is not reported complete by a refusal |

## Work Not Done and Split Ownership

The 2026-10-22 portfolio split changes planning ownership only. Status, existing dependencies and recorded evidence are retained; no implementation, verification or closure is claimed. Original criteria below the historical boundary are provenance, not a second active owner. Historical implementation steps apply only to the owner assigned in this table. Every test, documentation and review obligation follows its behavior owner.

| Work | Implementation owner |
|------|----------------------|
| Original AC-1 through AC-5, DETOUR codec, signaling, merging, failover and reoptimization | `plan/post-release/features/spec-mpls-9-rsvp-te-one-to-one-backup.md` |
| Current one-to-one acceptance and protection-report fidelity; no silent successful unsupported setting | `plan/release/fixes/spec-rsvp-te-one-to-one-config-fidelity.md` |

Work assigned to `plan/post-release/features/spec-mpls-9-rsvp-te-one-to-one-backup.md` is not performed or closed by this spec. Shared mechanisms are reused, not independently reimplemented. No feature is declared complete by the release repair or validation disposition.

## Files to Modify

- `internal/plugins/rsvpte/register.go` - existing one-to-one acceptance and configured protection request, under the design disposition.
- `internal/plugins/rsvpte/yang/ze-rsvp-te-conf.yang` - fidelity of the existing backup setting and its help, not new detour configuration.
- `internal/plugins/rsvpte/engine.go` - existing arming and protection-state fidelity, without implementing detour signaling.
- `internal/plugins/rsvpte/frr.go` - existing protection-request semantics only; DETOUR codec, selection and merging remain feature-owned.

## Implementation Ownership

Design and implement only the behavior assigned above. Original wiring, functional, interop, documentation and review obligations for that behavior transfer here with the original criteria. Resolve the recorded open design decisions before readiness; the portfolio move itself selects no new product behavior.

# Spec: inherited-functional-proof

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | validation |
| Priority | P2 |
| Release reason | Inherited forwarding, SDK process-boundary and IPsec lifecycle, monitoring and recovery proof concerns shipped behavior. |
| Depends | - |
| Phase | - |
| Updated | 2026-10-22 |

## Task and acceptance ownership

Own the following inherited obligations from
`plan/post-release/engineering/spec-ci-coverage-remaining-surfaces.md`.
They are proof debts, not assertions of current product defects.

| Original obligation | Required evidence and retained prerequisite |
|---------------------|--------------------------------------------|
| `fixit-ddos-test-infra`, 2026-07-16, AC-10 transit proof | Prove packets are FORWARDED, not merely admitted: a receiver at the far veth end counts arrivals. The existing shape can prove drop but not forwarding. |
| `fixit-plugin-event-subscription`, 2026-07-19, functional-proof | Author the functional `.ci` for the SDK-fork end-to-end path. The historical struck-through "unrunnable here" does not establish delivered proof. Exercise the process boundary, not an in-process substitute. |
| `fixit-ipsec-clear-reestablish`, 2026-07-19 | Execute strongSwan clear-reestablish and responder-accepts-reinit interop recovery evidence, retaining the recorded Docker plus charon prerequisite. |
| `test-coverage-gaps`, 2026-07-10, AC-3 event assertions | Restore the retained `vpn-ipsec/sa-up` assertions across the IPsec functional cases; establish current producers and prerequisites rather than treating the historical engine-gap diagnosis as a new finding. |
| `test-coverage-gaps`, 2026-07-10, AC-3 DPD-timeout proof | Preserve the deleted `ipsec-dpd-timeout.ci` proof obligation. Reconcile the current runner's ability to stop a background daemon; a demonstrated missing primitive becomes an explicit dependency, not a reason to drop the proof. |
| `test-coverage-gaps`, 2026-07-10, AC-3 monitor-streaming proof | Prove `monitor vpn ipsec` emits `sa-up` and `child-up` as they occur. Re-establish the current subscription path and event ordering; the historical startup-subscription blocker is an investigation lead, not a verified current defect. |

Design the concrete carriers and discriminating assertions when this skeleton
is taken up. No existing acceptance condition is weakened and no broader
coverage expansion is commissioned by the split.

## Reciprocal boundary and full history

The parent retains the complete original four-group description, twelve-row
inventory, provenance and deferral bodies. References to the rows above
there are non-owning historical records; this child is their sole proof owner.
The remaining tooling rows stay with the parent: alloc bench-log run,
bounded fuzz run, corrected agent-tooling proof, stop-background live run,
optional doc-drift hook, and PPPoE Partial support status. A runner enhancement
needed for this proof becomes an explicit dependency only after its absence
is demonstrated against the current runner.
The PPPoE record describes feature completeness rather than test coverage;
it stays Partial and is not upgraded by these proofs.

## Scheduling and qualifications

The source umbrella closed only its op-1 phase on 2026-08-17, not these debts.
The eight July 19 rows were one deferred live-server/QEMU-to-CI scheduling
batch, not eight independent commissioning decisions. This child preserves
that qualification, the undesigned/deferred state and the explicit record
that Thomas has not commissioned the work. Classification supplies a release
owner without lifting an owner pause, claiming a passing run, or reviving
outdated infrastructure assumptions as current findings.

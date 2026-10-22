# Spec: firewall-dynamic-set-config-fidelity

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P0 |
| Release reason | Accepted firewall dynamic, timeout and constant settings must not be silently ignored at apply. |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Resolve the existing accepted dynamic, set-timeout and constant knobs that the combined spec records as dead on apply. Own `flags-dynamic`, `flags-timeout`, `flags-constant` and their `Dynamic`/`HasTimeout`/`Constant` lowering or an explicitly approved honest refusal/removal. Do not add a packet-triggered insertion action merely to discharge configuration fidelity; that extension retains its own owner.

## Source and Evidence

Split from `plan/post-release/features/spec-firewall-dynamic-address-group.md` under owner decisions M30-M33. The complete original design, decisions, evidence and implementation history are preserved under Historical Combined Specification in `plan/post-release/features/spec-firewall-dynamic-address-group.md`. Read that history before design; no historical claim is freshly verified by this migration. The source lifecycle is retained on the original identity; this new work item begins as a skeleton, not an inferred readiness promotion.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-R1 | An existing dynamic, timeout or constant setting is accepted | The backend receives its declared `Dynamic`, `HasTimeout`, `Constant` flags and timeout semantics, or the unsupported setting is explicitly refused under the approved design |
| AC-R2 | Static named sets without the optional flags | Existing match and apply behavior remains unchanged |
| AC-R3 | Runtime insertion feature is not implemented | This repair neither exposes nor claims the new packet action |

## Work Not Done and Split Ownership

The 2026-10-22 portfolio split changes planning ownership only. Status, existing dependencies and recorded evidence are retained; no implementation, verification or closure is claimed. Original criteria below the historical boundary are provenance, not a second active owner. Historical implementation steps apply only to the owner assigned in this table. Every test, documentation and review obligation follows its behavior owner.

| Work | Implementation owner |
|------|----------------------|
| Original AC-1 through AC-7, packet-triggered insertion, action validation and action-inserted entry expiry | `plan/post-release/features/spec-firewall-dynamic-address-group.md` |
| Existing dynamic/timeout/constant flag parsing-to-apply fidelity, including `Dynamic`, `HasTimeout` and `Constant` lowering | `plan/release/fixes/spec-firewall-dynamic-set-config-fidelity.md` |

Work assigned to `plan/post-release/features/spec-firewall-dynamic-address-group.md` is not performed or closed by this spec. Shared mechanisms are reused, not independently reimplemented. No feature is declared complete by the release repair or validation disposition.

## Files to Modify

- `internal/plugins/firewall/nft/backend_linux.go` - existing set flag and timeout lowering in `applySet`.
- `internal/component/firewall/config.go` - existing set flag and timeout parsing, if required by the approved fidelity disposition.
- `internal/component/firewall/model.go` - existing set flag representation only.
- `internal/component/firewall/validate.go` - existing set-setting validation or approved explicit refusal.
- `internal/component/firewall/yang/ze-firewall-conf.yang` - existing set settings and help if the approved disposition changes acceptance.

The packet-insertion action and its `lowerAction` implementation remain feature-owned.

## Implementation Ownership

Design and implement only the behavior assigned above. Original wiring, functional, interop, documentation and review obligations for that behavior transfer here with the original criteria. Resolve the recorded open design decisions before readiness; the portfolio move itself selects no new product behavior.

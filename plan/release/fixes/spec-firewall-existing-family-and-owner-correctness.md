# Spec: firewall existing family and owner correctness

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P0 |
| Release reason | Existing ARP IP-header misreads and cross-producer table collisions can defeat accepted firewall and VRRP protection. |
| Scope | config |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Correct accepted family-arp matches that read IP offsets and prevent operator/producer table-name collisions. These obligations exist without the new ARP/ND leaves. AC-16 must be demonstrated using the current operator match surface; adding the six new leaves is not a prerequisite. Preserve the source decisions D-3 and its journal evidence, without assuming that dated observations are a fresh reproduction.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|---|---|---|
| AC-12 | In a table of family arp, any of source-address, destination-address, source-port, destination-port, protocol, connection-state, connection-mark, tcp-flags, a set match, an IRR or domain-group match | Refused at verify naming the leaf and "family arp carries no IP header"; icmp-type, icmpv6-type and dscp keep their existing refusals |
| AC-15 | An operator table whose kernel name equals a name another producer reserved (`vrrp_owner_arp`, `vrrp_owner_nd`, `vrrp` among them), in any family, whether or not that producer is active | Refused at verify naming the table, its kernel name and the owner that holds it; nothing is registered, so the VRRP owner table in the kernel keeps its terms |
| AC-16 | An operator arp table with an accept term for ARP replies from a VRRP-owned address, beside the VRRP owner table | The VRRP owner drop still removes the parent's physical-MAC reply (drop in any base chain is final) |
| AC-22 | Any owner calls `RegisterTables` with a table name another owner reserved | Refused with an error naming both owners; the registry keeps the reserving owner's tables unchanged. IRR and domain registering sets into an operator table's name stay accepted |

## Implementation Ownership

| Scope | Current owner |
|---|---|
| New leaves: AC-1..11, AC-13..14, AC-17..21; all feature design outside the release correction. | `plan/post-release/features/spec-firewall-arp-nd-matches.md` |
| Existing match-family correctness and reserved-table ownership: AC-12, AC-15, AC-16, AC-22. | `plan/release/fixes/spec-firewall-existing-family-and-owner-correctness.md` |

## Files to Modify

- `internal/component/firewall/validate.go` - existing ARP-family refusals and reserved table-name validation (AC-12, AC-15).
- `internal/component/firewall/registry.go` - owner reservations and cross-owner `RegisterTables` refusal (AC-22).
- `internal/plugins/vrrp/`, `internal/plugins/flowspec-firewall/`, `internal/plugins/copp/`, `internal/plugins/policyroute/`, `internal/plugins/ddos/local/`, `internal/component/gtsm/`, `internal/component/firewall/plugins/irr/` - the existing table producers reserve their own names; IRR reserves only `ze_irr_iface`, preserving set contributions.

This is planned ownership for the existing-family and owner repair, including
the current-surface AC-16 proof. New match leaves, model types and lowering
remain with the feature sibling.

## Work Not Done

| Scope | Current owner |
|---|---|
| New leaves: AC-1..11, AC-13..14, AC-17..21; all feature design outside the release correction. | `plan/post-release/features/spec-firewall-arp-nd-matches.md` |

## Provenance and retained evidence

Split from `plan/post-release/features/spec-firewall-arp-nd-matches.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/post-release/features/spec-firewall-arp-nd-matches.md`. This child claims no new test result or completion.

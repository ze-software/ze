# Spec: ipsec spd name selection

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P1 |
| Release reason | The Name-selector MUST applies to IPsec already shipped; it cannot be waived merely because multi-SPD and application override features are absent. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Resolve and implement/prove RFC4301-4.4.1.1-5 for the existing IKE/SPD selection role. The source records peer-selected PROTECT entries and PAD identity authorization already present; it does not prove those protections broken. Determine the precise missing named-selection obligation, including the recorded BYPASS/DISCARD gap, without making multiple SPDs or a local application API prerequisites. Any non-applicability decision remains the owner responsibility.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

- AC-1: RFC4301-4.4.1.1-5 has discriminating positive and negative proof for the shipped role, or an explicit owner applicability ruling with its grounds.
- AC-2: existing PROTECT/PAD identity binding remains intact and unauthorized identities do not acquire policy access.
- AC-3: any required named selection in existing policy actions is implemented and proven without waiting for optional multi-SPD work.

## Implementation Ownership

| Scope | Current owner |
|---|---|
| RFC4301-4.4.1-5 multi-SPD and RFC4301-4.4.1-8 absent application-principal surface; original AC-1 for those rows. | `plan/post-release/features/spec-ipsec-spd-selection.md` |
| RFC4301-4.4.1.1-5 and the corresponding original AC-1 evidence; mandatory named selection only. | `plan/release/fixes/spec-ipsec-spd-name-selection.md` |

## Files to Modify

- `internal/component/ike/ipsec/spd_policy.go` - the existing policy model and selection path, limited to the mandatory Name-selector obligation and any required named BYPASS/DISCARD selection.

The applicability decision and proof remain open. This planned producer
ownership does not assert that existing PROTECT/PAD authorization is broken,
and excludes optional multi-SPD and application-override leaves.

## Work Not Done

| Scope | Current owner |
|---|---|
| RFC4301-4.4.1-5 multi-SPD and RFC4301-4.4.1-8 absent application-principal surface; original AC-1 for those rows. | `plan/post-release/features/spec-ipsec-spd-selection.md` |

## Provenance and retained evidence

Split from `plan/post-release/features/spec-ipsec-spd-selection.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/post-release/features/spec-ipsec-spd-selection.md`. This child claims no new test result or completion.

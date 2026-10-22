# Spec: rfc-scheduled-debt-marker

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |
| Priority | P2 |
| Release reason | Required truthful accounting for scheduled absent features; origination remains separately owned. |

## Task

Own the scheduled-debt marker and truthful ledger reporting (original Phase 1), independently of implementing BGP-LS origination. The marker must resolve the live post-release origination spec and must never claim its requirements are covered.

## Acceptance Criteria

Original criterion ids from `plan/post-release/features/spec-bgp-ls-origination-and-the-scheduled-marker.md` are retained for this slice only; read the full inherited design and dated evidence in its Historical source record. Where a criterion spans both slices, the ownership table below is the exact boundary.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a gated requirement carries `{scheduled: plan/spec-<name>.md; why}` and `plan/spec-<name>.md` exists | `./le rfc check` reports no violation for that row |
| AC-2 | the same marker, and `plan/spec-<name>.md` does NOT exist | the check errors, naming the row and the missing spec |
| AC-3 | a `{scheduled}` marker with no reason after the `;` | the check errors, as `_parse_annotation` already does for every bare annotation |
| AC-4 | a row carrying `{scheduled}` AND a tagged test | the check errors that the marker is stale, as it does for `{gap}` |
| AC-5 | `./le rfc index-update` over a scheduled row | `ai/RFC-REQUIREMENTS.md` shows it as DEBT with the owning spec named, never as covered |
| AC-6 | a row that had a tagged test at HEAD now carries `{scheduled}` | `check_coverage_ratchet` fires; the marker is not an escape |
| AC-7 | a commit removes a spec while a `{scheduled}` row still names it | the check errors on the removal, before the row goes silently stale |
| AC-8 | `docs/features/rfc-status.md` Remaining prose | its spelled count agrees with the real scheduled count as well as the gap count |
| AC-9 | the three RFC 9552 rows | each carries `{scheduled}` naming this spec, and `./le rfc check` exits 0 on rfc9552 |

## Implementation Steps

Implement original Phase 1 and its existing test plan. AC-9 names the origination owner at its current path; the feature remains open until its own proof discharges the rows.

## Work Not Done

The companion retains all other obligations and the complete historical source record. This split is neither closure nor a claim of missing implementation; original approvals, dependencies, artifact requirements and proof standards remain in force.

| Original obligation | Implementation owner | Boundary |
|---------------------|----------------------|----------|
| AC-10–AC-15; original Phase 2 | `plan/post-release/features/spec-bgp-ls-origination-and-the-scheduled-marker.md` | BGP-LS origination, Instance-ID and producer controls |
| AC-1–AC-9; original Phase 1 | `plan/release/validation/spec-rfc-scheduled-debt-marker.md` | Scheduled marker validation, ratchets and debt publication |

## Required Reading

- `plan/post-release/features/spec-bgp-ls-origination-and-the-scheduled-marker.md`: full original source record, design, tests, decisions and evidence; apply them to this file’s owned slice only.

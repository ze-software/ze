# 287 — Documentation Accuracy Audit

## Objective

Review all 56 documentation files under `docs/` (excluding `plan/`) and fix inaccuracies against actual source code — wrong struct field names, stale spec references, removed types, and TODO items that were already implemented.

## Decisions

- Mechanical audit task; no design decisions made.

## Patterns

- Completed specs consistently referenced as `plan/spec-*.md` in docs — must be updated to `plan/learned/NNN-*.md`.
- Struct field names in example docs diverge from code over time; cross-check against actual `internal/` types.
- "TODO" or "planned" language in docs needs updating when code exists.
- Wire format and RFC reference docs (nlri-evpn, nlri-flowspec, nlri-bgpls) required no changes — they describe RFCs, not code structure.

## Gotchas

- Non-existent types/pools documented in `pool-architecture.md` (blob pools) — removed.
- `CheckedBufWriter` vs `CheckedWireWriter` naming discrepancy in `buffer-writer.md` — the interface is `CheckedBufWriter`.
- `Attribute` does not embed `WireWriter` in the actual code — documentation claimed it did.

## Files

- `docs/architecture/core-design.md`, `buffer-architecture.md`, `encoding-context.md`, `pool-architecture.md`, `update-building.md` — field names and spec refs corrected
- `docs/architecture/wire/attributes.md`, `capabilities.md`, `buffer-writer.md`, `qualifiers.md` — type names and interface definitions corrected
- `docs/architecture/api/architecture.md`, `capability-contract.md`, `update-syntax.md` — code paths and spec refs corrected
- `docs/contributing/rfc-implementation-guide.md` — code paths corrected

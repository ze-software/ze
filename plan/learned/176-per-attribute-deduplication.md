# 176 — Per-Attribute Deduplication

## Objective

Replace whole-blob attribute storage in the RIB with per-attribute-type pools so routes sharing ORIGIN/AS_PATH/LOCAL_PREF but differing in MED share pool entries instead of duplicating full blobs.

## Decisions

- New `FamilyRIBPerAttr` type created alongside existing `FamilyRIB` — preserves existing code for migration rather than breaking it.
- Pool indices 2–12 used for per-attribute pools (0=Attributes, 1=NLRI already taken).
- `OtherAttrs` blob pools known-but-not-individually-pooled attrs (MP_REACH, AGGREGATOR, etc.) together with unknown attrs.
- Partial flag (0x20) not preserved for pooled optional-transitive attributes — only value bytes stored, flags hardcoded at 0xC0 on reconstruction. Impact is low as modern implementations understand these attrs.
- No-op detection by handle equality: same handles = same data, skip redundant release/intern cycle.

## Patterns

- Implicit withdraw must save slot values before calling `Release()` — Release invalidates the handle, so save first.
- `AttrIterator` reused from `internal/component/bgp/attribute/` — do not duplicate parsing logic.
- Exhaustive switch on 22 attribute codes required (lint enforces it).
- Extended length attributes (>255 bytes) need flag 0x10 set in wire reconstruction.

## Gotchas

- `AttrIterator` is at `internal/component/bgp/attribute/` not `internal/bgp/attribute/` — wrong path causes compile failure.
- `ToWireBytes()` placed on `RouteEntry` not a standalone function — encapsulates pool access, avoids leaking handle details.

## Files

- `internal/pool/attributes.go` — 11 per-attribute pools added (idx=2–12)
- `internal/component/plugin/rib/storage/routeentry.go` — RouteEntry with per-attribute handles, Has*, Release, AddRef, Clone
- `internal/component/plugin/rib/storage/attrparse.go` — ParseAttributes() using AttrIterator
- `internal/component/plugin/rib/storage/familyrib_perattr.go` — FamilyRIBPerAttr with per-attr storage

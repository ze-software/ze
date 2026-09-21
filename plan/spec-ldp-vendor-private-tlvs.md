# Spec: LDP vendor-private TLVs and messages

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-21 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Each sentence is conditional on ze supporting the feature ("Implementations that support vendor-private TLVs MUST ...", "Implementations that support Vendor-Private messages MUST ..."). Ze encodes no vendor-private TLV (types 0x3E00-0x3EFF) and no Vendor-Private message (types 0x3E00-0x3EFF) anywhere in `wire.go`.

Ze does not offer this feature today. The owner can decline it in one word,
which turns every row below from `{gap}` into `{feature-declined}`; until then
each row is a scheduled requirement (owner ruling, 2026-09-21: our goal is RFC
compliance).

Requirements this spec covers, each with the RFC sentence and the producer or
absence in `internal/plugins/ldp`:

- `RFC5036-3.6.1.1-1` [MUST] "Implementations that support vendor-private TLVs MUST support a user-accessible configuration interface that causes the U-bit to be set on all transmitted vendor-private TLVs; this requirement MAY be satisfied by an interface that prevents transmission of all vendor-private TLVs for which the U-bit is clear (§3.6.1.1)" -- no vendor-private TLV or message is encoded in `wire.go`, so no U-bit configuration interface exists
- `RFC5036-3.6.1.2-1` [MUST] "Implementations that support Vendor-Private messages MUST support a user-accessible configuration interface that causes the U-bit to be set on all transmitted Vendor-Private messages; this requirement MAY be satisfied by an interface that prevents transmission of all Vendor-Private messages for which the U-bit is clear (§3.6.1.2)" -- no vendor-private TLV or message is encoded in `wire.go`, so no U-bit configuration interface exists

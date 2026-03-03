# 087 — Update Wire Input (hex/b64)

## Objective

Implement `peer X update hex|b64 attr set <bytes> nhop set <addr> nlri <family> add <bytes>...` handlers, allowing API callers to send wire-encoded BGP attributes and NLRIs without Ze parsing them.

## Decisions

- `WireNLRI` wraps raw bytes implementing `nlri.NLRI` interface: reactor can process wire NLRIs through the same `AnnounceNLRIBatch` path as parsed NLRIs without a separate code path.
- `PathAttributes.Wire *attribute.AttributesWire` field added: when set, reactor uses raw bytes and ignores semantic fields — single flag determines which mode is active.
- `APIContextID` registered at init with ASN4=true: API-originated wire data is assumed to use modern encoding; context re-encoding (ASN4→ASN2) is deferred.
- `GetNLRISizeFunc` exported from `chunk_mp_nlri.go`: wire parser needs it to split concatenated NLRI bytes into individual NLRIs without fully parsing them.
- ADD-PATH mismatch handled in `WireNLRI.Pack()`: strip 4 bytes if source has path-id and target doesn't; prepend NOPATH (0x00000000) if source lacks path-id and target expects it.
- `nhop` keyword replaces `next-hop`/`next-hop-self` inside attr sections: old syntax returns deprecation error with migration hint.
- `path-information` is text-mode only; wire mode embeds path-id in NLRI bytes directly via the `addpath` flag.
- Whitespace stripped before hex/b64 decode: allows users to space out bytes for readability.

## Gotchas

- Context re-encoding (ASN4→ASN2) deliberately deferred: users must ensure their wire bytes match the target peer's context. This is documented as `spec-wire-recode.md` skeleton.
- `attr set` is required for announce in wire mode (reactor validates): withdrawal-only commands need no attributes, but announce without attributes is an error.
- ADD-PATH test (TestReactor_WireMode_AddPathMismatch) deferred: requires peer capability checking not yet available in test setup.

## Files

- `internal/bgp/context/api.go` — APIContextID
- `internal/bgp/nlri/wire.go` — WireNLRI type
- `internal/plugin/update_wire.go` — ParseUpdateWire, hex/b64 handlers
- `internal/plugin/types.go` — PathAttributes.Wire field
- `internal/reactor/announce.go` — wire mode in AnnounceNLRIBatch/WithdrawNLRIBatch

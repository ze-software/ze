# 086 — NLRI Struct Embedding

## Objective

Reduce duplication in NLRI types by embedding `PrefixNLRI` (family, prefix, pathID) into INET and LabeledUnicast, and `RDNLRIBase` (rd, data, cached) into MVPN and MUP.

## Decisions

- IPVPN excluded from PrefixNLRI embedding: IPVPN field order is `family, rd, labels, prefix, pathID` — RD comes before prefix, making PrefixNLRI embedding incompatible without changing wire format.
- `buildData()` in RDNLRIBase used only in `Bytes()`, never in `WriteTo()`: `buildData()` allocates via `append()`; `WriteTo()` must remain zero-alloc by writing directly to the buffer.
- Deleted `encodeLabel()` in `labeled.go` in favour of existing `EncodeLabelStack()` from `ipvpn.go`: avoids maintaining two identical implementations.
- Added `sync.Once` to `RDNLRIBase` for thread-safe lazy `Bytes()` initialization: discovered during review that MVPN/MUP `Bytes()` had a race condition.
- `buildData()` returns a copy when no RD (no aliasing): prevents caller mutation of the `data` field through the returned slice.

## Gotchas

- `(bits + 7) / 8` prefix byte calculation was duplicated in 3 places across inet.go, labeled.go, ipvpn.go — extracted to `PrefixBytes(bits int) int` helper.
- Removing `make()+copy()` in `ParseMVPN()`/`ParseMUP()`: both `cached` and `data` fields became zero-copy slices of the original wire buffer. Verify callers do not mutate.

## Files

- `internal/bgp/nlri/helpers.go` — PrefixBytes(), WriteLabelStack()
- `internal/bgp/nlri/base.go` — PrefixNLRI, RDNLRIBase
- `internal/bgp/nlri/inet.go`, `labeled.go` — embed PrefixNLRI
- `internal/bgp/nlri/other.go` — embed RDNLRIBase in MVPN/MUP

# 093 — WriteTo Bounds Safety (Wire Path)

## Objective

Prevent buffer overflow when forwarding wire UPDATEs from Extended Message peers (65535-byte limit) to standard peers (4096-byte limit). Adds a check-after-write splitting algorithm to the wire-path split function.

## Decisions

- **Scope wire path only**: `SplitUpdateWithAddPath` in `chunk_mp_nlri.go` — not the API/UpdateBuilder path (separate spec).
- **Check-after-write over pre-calculation**: simpler and avoids the complexity of look-ahead size tables.
- **Subslices in hot path**: `SplitMPNLRI` returns subslices of the original buffer — no `append()`, no allocation in the split loop.
- **BGP-LS is the only unsplittable family**: 2-byte length field means a single NLRI can exceed 4096 bytes; return error rather than panic.
- **Per-family AddPath via caller pre-split**: `SplitUpdateWithAddPath` keeps a single `addPath bool` argument; callers split UPDATE by family first if MP_REACH and MP_UNREACH have different AddPath settings.
- **`ChunkMPNLRI` vs `SplitMPNLRI`**: `ChunkMPNLRI` copies (used when caller needs independent chunks), `SplitMPNLRI` subslices (wire forwarding hot path). This spec applies to the latter.

## Patterns

- Size of each NLRI family is calculable from its length byte in wire format — no max-size lookup table needed.
- FlowSpec individual NLRIs are capped at 4095 bytes (RFC 5575), so splitting always works.
- RFC constraint comments (`// RFC XXXX: requirement quoted`) added to each family size function.

## Gotchas

- Implementation already existed correctly; the work was adding tests, RFC reference comments, and documenting the BGP-LS unsplittable constraint.
- `SplitMPNLRI`'s `maxSize` parameter is the space available for NLRIs only — caller subtracts header and attribute bytes before passing it.

## Files

- `internal/bgp/message/chunk_mp_nlri.go` — RFC reference and constraint comments added
- `internal/bgp/message/chunk_mp_nlri_test.go` — subslice, no-alloc, boundary tests added
- `internal/bgp/message/update_split_test.go` — check-after-write, IPv4 field, FlowSpec, BGP-LS error tests added

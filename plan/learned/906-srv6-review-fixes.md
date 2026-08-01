# 906: SRv6 Prefix-SID RFC Compliance Fixes

## Context

External review found 8 RFC compliance bugs in the SRv6 Prefix-SID implementation.
5 were wire-format encoding errors, 3 were state-machine gaps.

## Findings

### Wire format: always cross-check encoder output against RFC wire diagrams

The SID Info Sub-TLV encoder emitted 20 bytes instead of 21, parsed Endpoint
Behavior as uint8 instead of uint16, and used the wrong Label-Index TLV field
layout. All three were caused by coding from memory instead of reading the RFC
wire diagram.

Pattern: RFC wire diagrams are byte-level truth. When writing or modifying any
encoder/decoder, read the RFC summary in `rfc/short/` and cross-reference each
field's offset, size, and type against the code.

### Validator and extractor must agree on minimum lengths

The validator checked `subLen < 21` (correct), but the extractor accepted
`subLen >= 17` (wrong). The mismatch meant valid-per-validator data could still
be mis-parsed by the extractor. When both sides reference the same wire format,
they must use the same constants.

### Transposition errata: read errata before implementing

RFC 9252 errata 7652 clarifies that transposed bits occupy high-order positions
of the label field. The original implementation read from low-order positions,
which only worked when transposLen == labelWidth. The fix requires knowing the
label field width (20 for VPN, 24 for EVPN) to extract from the correct bit
positions.

### Short-circuit optimizations must cover all fields that affect downstream

The best-change short-circuit compared peer, NH, MED, and eBGP flag but not the
SRv6 SID. A SID-only change was silently swallowed. When adding a field to the
emission path, audit the short-circuit/dedup path for the same field.

### State machine: store derived state only after all eligibility checks pass

sysrib stored `resolvedNH` before checking SID resolvability. When the SID was
unresolvable, the route was correctly suppressed but stale resolvedNH state
remained, preventing the "SID becomes reachable" transition from firing.
Pattern: computed state (resolvedNH) should only be persisted after all gates
(NH resolution + SID resolvability) pass.

## Applied To

- `internal/component/bgp/config/routeattr_prefixsid.go`
- `internal/component/bgp/message/rfc7606.go`
- `internal/component/bgp/plugins/rib/pool/srv6sid.go`
- `internal/component/bgp/plugins/rib/rib_bestchange.go`
- `internal/plugins/sysrib/sysrib.go`

## Files

None recorded.

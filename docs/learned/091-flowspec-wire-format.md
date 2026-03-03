# 091 — FlowSpec Wire Format

## Objective

Implement FlowSpec NLRI wire encoding/decoding (RFC 8955 IPv4, RFC 8956 IPv6) including VPN variants (SAFI 133/134).

## Decisions

- Component ordering enforced at encode time via `slices.SortFunc()` before writing: RFC 8955 Section 4.2 requires strict ascending type order.
- FlowSpec NLRI length encoding: <240 = 1 byte, >=240 = 2 bytes with 0xF0 prefix (not 0xFF, not standard 2-byte big-endian).
- Two operator formats implemented as separate types: `FlowOperator` (numeric: lt/gt/eq/and/end bits) and `FlowBitmaskOp` (bitmask: match/not/and/end bits).

## Patterns

- Component interface (`FlowComponent`) with `Len()` and `WriteTo()` enables zero-alloc encoding of the full NLRI.
- Backfill spec: implementation was complete before spec was written. Spec documents what exists.

## Gotchas

- FlowSpec length field: threshold is 240 (0xF0), not 256. The 2-byte format uses the first byte as `0xF0 | high_bits`, second byte as low bits — not standard big-endian uint16.
- IPv6 FlowSpec (RFC 8956) adds Type 13 (flow label) not present in RFC 8955 — requires AFI-aware parsing.
- VPN variant (SAFI 134) prepends an 8-byte Route Distinguisher before the FlowSpec components.

## Files

- `internal/bgp/nlri/flowspec.go` — ~1,200 lines: all types and encoding
- `internal/bgp/nlri/flowspec_test.go` — ~670 lines: comprehensive tests

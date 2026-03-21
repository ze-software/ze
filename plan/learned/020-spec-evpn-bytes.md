# 020 — EVPN Bytes Methods

## Objective

Implement `Bytes()` encoding methods for all 5 EVPN route types (Types 1-5), which previously returned nil and blocked EVPN route re-advertisement.

## Decisions

- Fixed `Len()` for Type 3 and Type 5 (both previously returned 0) as part of this work — encoding correctness requires accurate length.
- Type 5 uses fixed 4/16 byte prefix fields per RFC 9136 §3.1, not variable-length — matched the fix from spec 006 (RFC annotation).
- Wire format documented inline in `evpn.go` — tests use known wire bytes for round-trip verification (parse known bytes → Bytes() → compare with original).

## Patterns

- All 5 types follow the same pattern: build common header (type byte + length byte), encode fields in RFC-specified order using existing helpers (`RouteDistinguisher.Bytes()`, `EncodeLabelStack()`).
- Round-trip tests (parse → encode → compare) are the canonical verification for EVPN encoding.

## Gotchas

- `EVPNType3.Len()` and `EVPNType5.Len()` returned 0, not just Bytes() — both had to be fixed together since length is needed before encoding.
- Type 2 wire format varies by IP length (0/4/16 bytes for no IP/IPv4/IPv6) — encoding must check IP length at runtime.

## Files

- `internal/bgp/nlri/evpn.go` — Bytes() for all 5 types, fixed Len() for Type 3 and 5
- `internal/bgp/nlri/evpn_test.go` — round-trip tests for all 5 types

# 071 — NLRI Wire Format Tests

## Objective

Add comprehensive NLRI wire format tests (hex verification and round-trip) that would have caught the ADD-PATH/EVPN bug in spec-070, preventing similar regressions.

## Decisions

- Added EVPN to existing consistency tests (`TestLenWithContext_MatchesWriteNLRI_AllTypes`) — the original omission enabled the spec-070 bug to exist undetected
- Wire format tests verify actual hex bytes, not just lengths — hex verification catches bit-level encoding errors
- Round-trip tests (encode → decode → encode, compare wire) catch asymmetric encode/decode bugs

## Patterns

- None.

## Gotchas

- **Bug found during implementation:** EVPN Type 5 `e.gateway.As4()` panicked when gateway was empty `netip.Addr{}` (zero value) — fixed by checking `e.gateway.IsValid()` first; per RFC 9136 Section 3.1, gateway IP is 0 when not used
- Root cause of original spec-070 EVPN bug: EVPN wasn't in the consistency tests; adding it to those tests immediately exposed the inconsistency

## Files

- `internal/bgp/nlri/len_test.go` — EVPN cases added to consistency tests
- `internal/bgp/nlri/wire_format_test.go` — new file: TestWireFormat_AddPath, TestWireFormat_IPVPN, TestWireFormat_LabeledUnicast, TestWireFormat_EVPN, TestRoundTrip_{INET,IPVPN,EVPN}
- `internal/bgp/nlri/evpn.go` — EVPN Type 5 gateway panic fix (line 919)

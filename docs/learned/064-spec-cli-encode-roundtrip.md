# 064 — CLI Encode Command and Round-Trip Testing

## Objective

Add `ze bgp encode` CLI command (API command text → BGP wire hex) and round-trip tests (encode → decode → encode) covering all supported address families.

## Decisions

- Mechanical implementation covering all 12 families (ipv4/ipv6 unicast, mpls-vpn, nlri-mpls, flowspec, l2vpn/vpls, l2vpn/evpn, ipv4/ipv6 mup)
- FlowSpec redirect: 2-byte ASN when value ≤65535, 4-byte ASN otherwise (RFC 7674 boundary at 65535/65536)
- ESI parsing: accepts both hex (`0102030405060708090A`) and colon-separated (`01:02:03:...`) formats
- `nlri.ParseRDString()` extracted from reactor.go to shared location — encode command needed it too
- stdin support via `-` convention for piped usage
- `--no-header` flag and `-n` (NLRI-only) flag for scripting integration

## Patterns

- Round-trip tests use encode → `ze bgp decode` → check JSON fields — proves internal consistency without reference to a specific external tool
- `ParseRouteAttributes()` and `ParseL2VPNArgs()` exported from `internal/component/plugin/route.go` to support the CLI

## Gotchas

- EVPN always has PathID=0 when ADD-PATH is enabled (limitation documented)
- MUP SRv6 Prefix SID and extended communities not wired through CLI (documented limitation)
- Phase 4 (forked process tests matching ExaBGP functional test style) not implemented — low priority

## Files

- `cmd/ze/bgp/encode.go` — CLI encode command (~1050 lines)
- `cmd/ze/bgp/encode_test.go` — 40 encode tests
- `cmd/ze/bgp/roundtrip_test.go` — 13 round-trip tests
- `internal/bgp/message/update_build.go` — `EVPNParams`, `BuildEVPN()`
- `internal/bgp/nlri/evpn.go` — `NewEVPNType1-5()` constructors

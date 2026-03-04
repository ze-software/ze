# 354 — NLRI Family Test Coverage

## Objective

Add fuzz, functional decode, and plugin integration tests for all NLRI family plugins — fuzz tests are highest priority since parse functions handle untrusted wire bytes from BGP peers.

## Decisions

- All 8 NLRI family plugins got fuzz tests (VPN, EVPN, FlowSpec, BGP-LS, MUP, MVPN, RTC, VPLS)
- Decode tests used `bgp-` prefix naming convention (`bgp-mup-1.ci` not `mup-1.ci`)
- BGP-LS got extensive decode coverage (11 test files) due to complex NLRI subtypes
- Labeled IPv4 decode, EVPN types 3/5 decode, plugin-ls and plugin-labeled integration tests remain as gaps

## Patterns

- Fuzz test pattern: seed with known-good round-trip bytes + empty + truncated + max-value, then exercise `String()`, `Bytes()`, `Len()`, `WriteTo()` on successful parse
- Decode `.ci` pattern: `stdin=payload:hex=...` + `cmd=...ze-test decode --json --nlri <family> -` + `expect=json:json={...}`
- Plugin `.ci` pattern: `cmd=...ze plugin <name> --nlri <hex>` + `expect=exit:code=0`

## Gotchas

- Plan naming didn't match implementation naming (`mup-1.ci` vs `bgp-mup-1.ci`) — always check existing naming conventions before writing tests
- `bgp-evpn-1.ci` covers EVPN Type 2 (MAC/IP), not Type 1 (Ethernet Auto-Discovery) despite the filename number

## Files

- `internal/component/bgp/plugins/bgp-nlri-*/types_fuzz_test.go` — 8 fuzz test files
- `test/decode/bgp-{mup,mvpn,rtc,vpls,ls}-*.ci` — functional decode tests
- `test/plugin/plugin-{evpn,flowspec,vpn,mup,mvpn,rtc,vpls}-nlri.ci` — plugin integration tests

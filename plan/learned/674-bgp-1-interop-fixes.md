# 674 -- BGP Interop Failure Fixes

## Context

After expanding the interop matrix (learned 433), 7 of 11 new scenarios failed. The 4 passing scenarios (IPv6/GoBGP, multihop with all 3 vendors) proved the test infrastructure was correct, so all failures were Ze bugs or test script bugs. The goal was to fix every failure and get all 11 scenarios (22-32) passing.

## Decisions

- **Fixed AS_PATH at the source** (over adding workarounds per family): `writeASPath`/`buildBatchASPath` used `a.r.config.LocalAS` (global=0) instead of per-peer `settings.LocalAS`. This single fix resolved the most widespread failure: FRR and GoBGP both reject UPDATEs with AS 0 per RFC 7607 (treat-as-withdraw).
- **Added static registry fallback for family validation** (over removing the check): `validatePeerFamilies` only checked the runtime plugin registry, which is empty for in-process plugins at config-validation time. Added fallback to static registry so non-unicast families pass validation without removing the safety check.
- **Fixed VPN next-hop encoding per RFC 4364** (over accepting the short form): `MPReachNLRI.WriteTo()` sent 4-byte next-hop for VPN (SAFI 128), but RFC 4364 Section 4.3.4 requires 12 bytes (8-byte RD prefix of zeros + 4-byte IPv4). FRR was lenient and accepted the short form, masking the bug. GoBGP correctly rejected it.
- **Fixed OPEN parameter encoding** (over one-cap-per-param): Ze was emitting one type-2 optional parameter per capability. RFC 5492 allows bundling multiple capabilities in a single parameter. GoBGP only parsed the last type-2 parameter, so it saw only one capability. Changed to single-parameter encoding.
- **Added multihop for BIRD IPv6** (over changing the Docker network): BIRD enforces RFC 4271 next-hop reachability strictly. On the IPv4-only Docker network, the IPv6 next-hop (2001:db8::2) was unreachable. Adding `multihop;` to BIRD config disabled the directly-connected check.

## Consequences

- All 11 scenarios (22-32) pass with 0 regressions.
- VPN next-hop encoding is now RFC-compliant; the FRR leniency that masked the bug no longer matters.
- AS_PATH encoding is correct for all address families (was only broken for non-unicast routes announced via process plugins).
- The OPEN parameter bundling fix benefits all peers, not just GoBGP.

## Gotchas

- **FRR leniency masks bugs**: FRR accepted a 4-byte VPN next-hop that GoBGP (correctly) rejected. Triangulation across vendors is essential; passing with one vendor does not mean the encoding is correct.
- **Global vs per-peer config fields**: `a.r.config.LocalAS` is the globally-configured AS (often 0 when using per-peer AS). Using it in wire encoding silently produces invalid messages that some peers tolerate and others reject. Always use `settings.LocalAS` for per-peer context.
- **check.py vendor-specific JSON structures**: FRR nests EVPN/VPN routes under an RD key, GoBGP returns dicts instead of lists, and FRR uses `l2VpnEvpn` (not `l2vpnEvpn`). Each vendor needs its own JSON extraction path in test scripts.
- **Text API parser differences**: announce scripts originally used invalid extended community syntax (`[origin:65001:100]` where 100 is not a valid IP) and `time.sleep(0.1)` instead of proper barrier flush (`wait_for_ack(1)`). Both caused silent failures.

## Files

- `internal/component/bgp/reactor/reactor_api_batch.go` (AS_PATH fix)
- `internal/component/bgp/reactor/reactor.go` (static registry fallback)
- `internal/component/bgp/reactor/session_negotiate.go` (OPEN parameter encoding)
- `internal/core/bgp/attribute/mpnlri.go` (VPN next-hop encoding)
- `test/interop/scenarios/22-32/` (check.py and announce script fixes)
- `test/interop/scenarios/25-ipv6-ebgp-bird/bird.conf` (multihop)
- `test/interop/scenarios/29-vpn-gobgp/gobgp.toml` (afi-safi name)

# bgp-mup-dsd-gobgp

Interop scenario for `draft-ietf-bess-mup-safi` Section 3.3.6 against GoBGP v4.11.0.

## What this scenario proves

1. Ze and GoBGP v4.11.0 establish an eBGP session negotiating `ipv4/unicast` and `ipv4/mup`, with `accept-srv6-prefix-sid true` on Ze's peer so RFC 8669 Section 4 retains attribute 40 across the eBGP boundary.
2. GoBGP originates an `ipv4/mup` Direct Segment Discovery (Route Type 2) route (`192.0.2.1`, RD `65002:100`, SRv6 Prefix-SID `2001:db8::/48`, `END_DT4`, IPv6 next hop `2001:db8:5::1`) alongside an `ipv4/unicast` control prefix (`10.20.0.0/24`).
3. Ze parses GoBGP's live wire DSD UPDATE, stores the DSD NLRI (`0100020c0000fdea00000064c0000201`) in `bgp-adj-rib-in`, and — because no IGP locator-origin snapshot is active — keeps the DSD route unverified while logging and raising `doctor-bgp-mup-locator-unresolved` (fix `ze show isis database`) on `show warnings`.
4. The BGP session remains Established with zero drops (`connections-established: 1`, `connections-dropped: 0`).

## Scope boundary (what GoBGP cannot synthesize)

GoBGP's CLI (`gobgp global rib add -a ipv4-mup dsd ...`) always requires an SRv6 Prefix-SID (`prefix`, `locator-node-length`, `function-length`, `behavior`) and cannot synthesize a DSD UPDATE with an absent Prefix-SID, nor does GoBGP run an IGP speaker to originate a disagreeing SRv6 locator for a proven-mismatch check. Those negative paths are proven by the `ze-peer` functional tests:

- `test/plugin/mup-dsd-without-prefix-sid-withdraw.ci` (DSD without Prefix-SID -> treat-as-withdraw)
- `test/plugin/mup-dsd-ebgp-prefix-sid-discarded-withdraw.ci` (eBGP Prefix-SID discarded under RFC 8669 Section 4 -> DSD treat-as-withdraw)
- `test/plugin/mup-dsd-nexthop-locator-mismatch-withdraw.ci` (DSD next hop disagreeing with IS-IS locator originator -> treat-as-withdraw)
- `test/plugin/mup-dsd-locator-unresolved-kept.ci` (unresolved DSD keep-and-warn on internal rail)
- `test/plugin/mup-isd-without-prefix-sid-withdraw.ci` and `test/plugin/mup-isd-originator-mismatch-withdraw.ci` (ISD Section 3.3.3 counterparts)

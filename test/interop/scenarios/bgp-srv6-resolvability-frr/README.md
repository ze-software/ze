# bgp-srv6-resolvability-frr

Proves RFC 9252 Section 5 (`RFC9252-5-2`) SRv6 Service SID pre-selection resolvability against FRR 10.7.1:

1. `frr` (`172.30.0.3`, iBGP AS 65001) exports IPv6 VPN prefix `2001:db8:9252:1::/64` (RD `65001:1`) carrying an SRv6 L3 Service SID allocated from `LOC1` (`fc00:0:1::/48`), alongside an independent non-SRv6 IPv4 unicast control `10.92.52.0/24`.
2. `ze` (`172.30.0.2`, AS 65001) has a static route resolving FRR's native IPv6 BGP `NEXT_HOP` (`fd00:1e:0::/64`), so only the SRv6 Service SID locator `fc00:0:1::/48` is initially unreachable. Ze must:
   - Propagate the control `10.92.52.0/24` to `frr-sink` (`172.30.0.15`, eBGP AS 65002).
   - Retain `2001:db8:9252:1::/64` in Adj-RIB-In marked `"srv6-sid-resolvable": false` and `"ineligible-reason": "srv6-sid-unresolvable"`.
   - Report `doctor-bgp-srv6-sid-unresolvable` in `show doctor`.
   - Exclude the VPN route from `show bgp rib best` and NOT advertise it to `frr-sink`.
3. `frr` then advertises `fc00:0:1::/48` over `ipv6 unicast`. With no new VPN UPDATE, Ze resolves the retained Service SID, elects `2001:db8:9252:1::/64` as best-path, clears the ineligibility and doctor warning, and advertises `2001:db8:9252:1::/64` to `frr-sink`.

Checker: `internal/le/interoplab/bgp/check_srv6_resolvability.go` (`checkSRv6ResolvabilityFRR`).

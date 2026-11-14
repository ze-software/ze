# An interop check expects what the peer cannot do

The check was written from the RFC or from Ze's side, not from the peer's
source, and asks the peer for a feature it lacks or an output it never prints.
The scenario can never pass, and its red reads as a Ze defect.

Captured: 2026-11-02 rule interop-and-goal-validation/directives/read-the-peer-s-source-before-an-interop-expectation

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-02 | - | `ospfv3-ri-frr` checker | It asked FRR 10.7.1 ospf6d for Router Information support (`router-info`, `database router-information`), which ospf6d does not have: `% Unknown command` | fixed: e206a51fbb reads Ze's RI LSAs in FRR's database as unknown types 0xa00c and 0xc00c |
| 2026-11-02 | - | `ospfv3-debug-inject-frr` checker | It injected a 4-octet Intra-Area-Prefix body that `ospf6_lsa_examin` drops as undersized, and waited for `0x2009`, which FRR never prints for a type it knows | fixed: e206a51fbb injects a valid one-prefix body and waits for the prefix, then for its withdrawal |
| 2026-11-03 | spec-filter-wire-0-umbrella | `bgp-srv6-invalid-sid-ineligible-frr` FRR-side wait for `show bgp ipv6 vpn` | FRR 10.7.1 never exports its default-instance SRv6 VPN route: export runs before zebra sends the locator (`ensure_vrf_tovpn_sid_per_af` returns at `!locator_bgp`), and `vpn_leak_postchange_all` re-leaks VRF instances only | fixed: b8e7f293ae waits for the locator, re-runs `export vpn`, waits for the SID, re-runs; no assertion removed |
| 2026-11-14 | spec-rfc-verdict-fix-ospf (found, not caused) | `ospfv3-sr-frr` fixture and checker | Not run. FRR 10.7.1 ospf6d SR support is unverified, and the scenario very likely has two gaps `ospf-sr-frr` had before 4581355915: no `ze-sysctls` file for the MPLS label space, and no loopback address for the Prefix-SID prefix | open: read ospf6d's SR source at 10.7.1 before any expectation, fix the fixture, then observe red and green |

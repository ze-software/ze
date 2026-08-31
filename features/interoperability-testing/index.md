# Interoperability Testing

<!-- source: internal/le/interoplab/bgp/run.go -- native interop runner -->
<!-- source: internal/le/interoplab/lab.go -- scenario lifecycle -->

Ze ships a Docker-based interoperability test suite that verifies protocol correctness
against real third-party BGP implementations. Tests are not mocks -- they launch actual
daemon instances in containers and exchange real BGP messages.

| Feature | Description |
|---------|-------------|
| Target daemons | FRR, BIRD, GoBGP (tested), rustbgpd, RustyBGP, freeRtr (Dockerfiles ready) |
| Scenario count | 100+ scenarios covering core BGP protocol and extensions plus adjacent protocols (RPKI, BMP, IS-IS, OSPF/OSPFv3) |
| Runner | `./le integration interop` (all) or `INTEROP_SCENARIO=<name> ./le integration interop` (single) |
| Container images | Customizable via env vars (e.g., `FRR_IMAGE=quay.io/frrouting/frr:10.3`) |

## Scenarios

The table below is a representative subset covering the core BGP protocol and
extensions. The full suite has over 100 scenarios; the remainder exercise RPKI, BMP,
IS-IS, and OSPF/OSPFv3 interoperability against FRR.

| # | Scenario | Target |
|---|----------|--------|
| 01 | eBGP IPv4 | FRR |
| 02 | eBGP IPv4 | BIRD |
| 03 | iBGP | FRR |
| 04 | 4-byte ASN | FRR |
| 05 | Routes from | FRR |
| 06 | Routes from | BIRD |
| 07 | Routes to | FRR |
| 08 | Triangle topology | multi |
| 09 | Route withdrawal | FRR |
| 10 | IPv6 eBGP | FRR |
| 11 | Add-Path | FRR |
| 12 | Route Refresh | FRR |
| 13 | Graceful Restart | FRR |
| 14 | Route Server | FRR |
| 15 | Standard communities | FRR |
| 16 | Extended communities | FRR |
| 17 | MD5 authentication | FRR |
| 18 | eBGP | GoBGP |
| 19 | Routes | GoBGP |
| 20 | BGP Roles | FRR |
| 21 | BGP Roles | GoBGP |
| 22 | EVPN | FRR |
| 23 | VPN | FRR |
| 24 | FlowSpec | FRR |
| 25 | IPv6 eBGP | BIRD |
| 26 | IPv6 eBGP | GoBGP |
| 27 | Multihop eBGP | FRR |
| 28 | EVPN | GoBGP |
| 29 | VPN | GoBGP |
| 30 | FlowSpec | GoBGP |
| 31 | Multihop eBGP | BIRD |
| 32 | Multihop eBGP | GoBGP |
| 33 | BFD failover | FRR |
| 34 | ECMP | FRR + GoBGP |
| 35 | SRv6 VPNv6 | FRR |
| 36 | Remove private AS | FRR |
| 37 | Remove private AS via AS4_PATH | FRR + BIRD |

Only Ze and rustbgpd ship cross-implementation interop test suites among open-source
BGP daemons. Ze's suite has more scenarios and tests against more target implementations.

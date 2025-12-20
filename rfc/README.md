# BGP RFC Reference

Text versions of RFCs relevant to ZeBGP implementation.

## Core Protocol

| RFC | Title | Status |
|-----|-------|--------|
| [4271](rfc4271.txt) | A Border Gateway Protocol 4 (BGP-4) | **Core** |
| [4760](rfc4760.txt) | Multiprotocol Extensions for BGP-4 | Core |
| [6793](rfc6793.txt) | BGP Support for Four-Octet AS Numbers | Core |

## Capabilities

| RFC | Title | Status |
|-----|-------|--------|
| [5492](rfc5492.txt) | Capabilities Advertisement with BGP-4 | Core |
| [9072](rfc9072.txt) | Extended Optional Parameters Length | Planned |
| [2918](rfc2918.txt) | Route Refresh Capability | Implemented |
| [7313](rfc7313.txt) | Enhanced Route Refresh | Planned |
| [7911](rfc7911.txt) | Advertisement of Multiple Paths (Add-Path) | Implemented |
| [8654](rfc8654.txt) | Extended Message Support | Planned |

## Path Attributes

| RFC | Title | Status |
|-----|-------|--------|
| [1997](rfc1997.txt) | BGP Communities Attribute | Implemented |
| [4360](rfc4360.txt) | BGP Extended Communities Attribute | Implemented |
| [5701](rfc5701.txt) | IPv6 Address Specific Extended Community | Planned |
| [8092](rfc8092.txt) | BGP Large Communities Attribute | Implemented |
| [8195](rfc8195.txt) | Use of BGP Large Communities | Reference |

## NLRI Types

| RFC | Title | Status |
|-----|-------|--------|
| [7432](rfc7432.txt) | BGP MPLS-Based Ethernet VPN (EVPN) | Partial |
| [8955](rfc8955.txt) | Dissemination of Flow Specification Rules | Implemented |
| [4364](rfc4364.txt) | BGP/MPLS IP Virtual Private Networks (VPNs) | Implemented |
| [4659](rfc4659.txt) | BGP-MPLS IP VPN Extension for IPv6 VPN | Implemented |
| [4761](rfc4761.txt) | Virtual Private LAN Service (VPLS) | Planned |
| [4684](rfc4684.txt) | Constrained Route Distribution (RTC) | Planned |

## Notifications

| RFC | Title | Status |
|-----|-------|--------|
| [8203](rfc8203.txt) | BGP Administrative Shutdown Communication | Planned |
| [9003](rfc9003.txt) | Extended BGP Administrative Shutdown Communication | Planned |

---

**Note:** When implementing RFC functionality, code MUST reference the RFC number and section.

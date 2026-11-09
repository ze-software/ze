# DHCP Server Shared Segment

## Meta

| Field | Value |
|-------|-------|
| Name | DHCP Server Shared Segment |
| Kind | protocol |
| Scope | partial |
| Scope gaps | option 82 relay agent information, its link-selection sub-option (RFC 3527) and the server identifier override (RFC 5107) are not read, so a relayed request selects its segment by giaddr alone |
| Level | experimental |
| Components | internal/plugins/dhcpserver |
| Real-path tests | test/dhcp/dhcpserver-shared-segment-reservation.ci, test/dhcp/dhcpserver-request-states.ci, test/dhcp/dhcpserver-server-identifier.ci, test/dhcp/dhcpserver-relay-only-subnet.ci, test/dhcp/dhcpserver-no-segment-address.ci |
| Interop | bgp/dhcpv4-server-shared-segment-dhcpcd, topology/lan-services |
| RFCs | rfc2131 |
| Docs | docs/guide/configuration.md |
| Doc review | 2026-11-09: the changes since to docs/guide/configuration.md (the subnet lease-time and dns-source rows, resolv-conf-path, the l2tp pool dns-source rows, the hostname and pppoe-client paragraphs) leave "Several subnets on one interface" as checked; resolveSegment is still in internal/plugins/dhcpserver/segment.go and the server-identifier leaf in ze-dhcp-server-conf.yang. Earlier, 2026-11-08: "Several subnets on one interface" in docs/guide/configuration.md checked against resolveSegment and the server identifier choice in internal/plugins/dhcpserver/segment.go, and the server-identifier leaf in internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang |
| Defect review | 2026-11-08: no open journal row against internal/plugins/dhcpserver segment selection |

## Description

Several IPv4 subnets served on one LAN segment. The subnets of one `shared-network` share a segment; a request is served from the segment its listen interface is addressed in, or the one holding the relay's `giaddr`. A static reservation for the client's MAC in any subnet of the segment wins over every pool, then the home subnet's pool is used, then the others in configuration order. Option 54 is the interface's address in the chosen subnet, or the subnet's optional `server-identifier` while the interface holds it, and a SELECTING request naming any of the server's addresses on the segment is accepted (RFC 2131 Section 4.1). <!-- source: internal/plugins/dhcpserver/segment.go -- resolveSegment -->

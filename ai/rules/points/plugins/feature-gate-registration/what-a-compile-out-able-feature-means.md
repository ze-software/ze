---
kind: note
level:
stage:
---
How to add or change a **compile-out-able feature**: a subsystem that can be
dropped from the `ze` binary at build time via a `//go:build ze_<feature>` tag,
for a smaller binary and a smaller attack surface (looking-glass `ze_lg`, ssh
`ze_ssh`, web `ze_web`, gNMI `ze_gnmi`, MCP `ze_mcp`, REST API `ze_rest`, gRPC
API `ze_grpc`, Prometheus exporter `ze_telemetry`, routing protocols `ze_isis` /
`ze_ldp` / `ze_ospf` / `ze_rsvpte`, first-hop redundancy `ze_vrrp`, BGP `ze_bgp`,
BMP `ze_bmp`, MRT `ze_mrt`, and since feature-gate-12 the remaining service and
protocol surface: BFD `ze_bfd`, IKEv2/IPsec `ze_ike`, the L2TP/PPPoE BNG
`ze_l2tp`, RADIUS `ze_radius`, TACACS+ `ze_tacacs`, the VPP dataplane `ze_vpp`,
flow export `ze_flowexport`, DDoS `ze_ddos`, anomaly `ze_anomaly`, AS112
`ze_as112`, GeoDNS `ze_geodns`, DHCP server `ze_dhcpserver`, netboot `ze_pxe`
(tftpserver + imageserver), NTP `ze_ntp`, traffic accounting `ze_trafficusage`,
policy routing `ze_policyroute`, CoS `ze_cos`, CoPP `ze_copp`, MPLS operational
surface `ze_mpls`, ExaBGP bridge `ze_exabgp`). The manifest is the inventory;
this list is illustrative.

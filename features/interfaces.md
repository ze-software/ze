# Interfaces

## Meta

| Field | Value |
|-------|-------|
| Name | Interfaces |
| Page | docs/features/interfaces.md |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/iface, internal/plugins/iface, internal/plugins/cos, internal/core/rtproto/rtproto.go |
| Real-path tests | test/plugin/iface-tunnel-kinds.ci, test/plugin/iface-vlan-qos.ci, test/plugin/iface-cli-ops.ci, test/plugin/iface-verbs.ci, test/plugin/iface-rate-json.ci, test/plugin/iface-ensure-rollback.ci, test/plugin/iface-learned-route-metric.ci, test/plugin/iface-route-protocol-name.ci, test/parse/iface-rpf-check.ci, test/parse/iface-netlink-accepts-wireguard.ci, test/parse/pppoe-client-keepalive.ci, test/reload/tx-iface-apply.ci |
| Interop | bgp/dhcpv6-client-renew-kea, bgp/dhcpv6-client-confirm-kea |
| Docs | docs/features/interfaces.md |
| Doc review | 2026-11-07: the PPPoE client keepalive and reconnect paragraph matches the YANG leaves keepalive interval and failures and reconnect delay-min and delay-max (defaults 10, 3, 1 and 60) in internal/component/iface/yang/ze-iface-conf.yang, keepaliveLoop (ends the session when echoFails reaches the configured failures), pppoeClientConfigChanged (timing excluded from the restart comparison) and setTiming (stored for the next session and the next wait). Earlier, 2026-10-20: the DHCPv6 move paragraph of docs/features/interfaces.md matches confirmAfterMove, dropAddresses and v6ConfirmCollector (Confirm after the CNF_MAX_DELAY wait, Success or silence keeps the lease, NotOnLink alone removes every address and solicits); both Interop scenarios exist under test/interop/scenarios; the row itself says target-runner evidence is still required; anchors (rtproto Iface 253, route-priority default 254, tunnel ttl defaults) exist at the named files |
| Defect review | 2026-10-07: open: plan/release/fixes/spec-bridge-member-removal-never-detaches-port.md, spec-iface-delete-parent-before-vlan-rolls-back.md, spec-interface-monitor-container-names-no-behavior.md |
| Extra criteria | supported: privileged netlink evidence on a target runner per link type = none yet |

## Description

The PPPoE client answers access-network LCP renegotiation on the existing session.
Standalone tests cover repeated authentication, IPCP and complete result
notifications. The interface consumer reapplies changed MTU, local/peer addresses
and the default route; consumer tests cover reapplication and failure handling.
Target-kernel proof and wholesale interoperability proof remain outstanding.
<!-- source: internal/component/l2tp/pppoeclient/connected_session.go -- restart -->
<!-- source: internal/component/l2tp/pppoeclient/connected_changes_test.go -- TestConnectedChangesEveryCompletion, TestConnectedChangesNewestWins -->
<!-- source: internal/component/iface/pppoe_client.go -- runSession -->
<!-- source: internal/component/iface/pppoe_changes_test.go -->
<!-- source: internal/component/iface/pppoe_change_lifecycle_test.go -->

The PPPoE client's dead-peer detection and redial hold-off are configurable on
each `pppoe-client` entry: `keepalive { interval; failures }` sets how often an
LCP Echo-Request is sent (1 to 3600 seconds, default 10) and how many unanswered
requests in a row end the session (1 to 255, default 3), and
`reconnect { delay-min; delay-max }` bounds the doubling wait before the next
discovery (1 to 3600 seconds, defaults 1 and 60). `delay-max` below `delay-min`
is refused naming both leaves. A reload that changes only these leaves keeps the
running session up: the new values apply from the next session and the next
redial wait.
<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- container keepalive, container reconnect -->
<!-- source: internal/component/l2tp/pppoeclient/network_phase.go -- keepaliveLoop -->
<!-- source: internal/component/iface/pppoe_client.go -- pppoeClientConfigChanged, setTiming -->

Linux interface management via netlink: ethernet, dummy, veth, bridge, loopback, VLAN (with 802.1p ingress/egress QoS maps translating PCP to internal priority and back, and named class-of-service profiles via the cos plugin with interface-level inheritance and per-unit override/opt-out; dynamic per-subscriber CoS via RADIUS Filter-Id "cos:\<name\>" or vendor-specific attributes (Cisco-AVPair, Juniper ERX, Nokia, Huawei) applied on session-up with mid-session CoA changes and session-down revert; MikroTik Mikrotik-Rate-Limit VSA for shaper rate extraction <!-- source: internal/plugins/iface/netlink/manage_linux.go -- CreateVLAN QoS maps, UpdateVLANQoSMap --> <!-- source: internal/plugins/cos/config.go -- parseAndRegisterProfiles --> <!-- source: internal/plugins/cos/handler.go -- dynamic CoS event handler --> <!-- source: internal/component/l2tp/plugins/authradius/extract_vsa.go -- vendor VSA CoS/rate extraction -->), 9 tunnel kinds (GRE, GRETAP, IP6GRE, IP6GRETAP, IPIP, SIT, IP6TNL, IPIP6, VXLAN with a 24-bit VNI over UDP 4789 per RFC 7348), each carrying an outer-header TTL of 64 when the config names none, so an encapsulated packet survives a multi-hop underlay; `ttl 0` still asks the kernel to inherit the inner packet's TTL, and the default reaches netlink-backed tunnels only, because the VPP tunnel calls carry no TTL field, so a commit that names a `ttl` on a vpp-backed tunnel is refused by path rather than accepted and dropped <!-- source: internal/component/iface/yang/ze-iface-conf.yang -- container encapsulation choice kind, ttl and hoplimit defaults --> <!-- source: internal/component/iface/tunnel.go -- tunnelSchema, applyDefaults --> <!-- source: internal/plugins/iface/netlink/tunnel_linux.go -- vxlan link build -->, and WireGuard (declarative peers with `$9$`-encoded keys); DHCP (config-driven, routes, DNS, NTP discovery), NTP client (clock sync, RTC, time persistence, max-step safety cap), monitoring, migration, mirroring, per-interface offload and steering (GRO, GSO, SG, TSO, LRO, hw-tc-offload via kernel ethtool ioctl; RPS, RFS via sysfs; boolean three-state: true/false/absent), per-family reverse path filtering (`rpf-check strict\|loose\|disable` in ipv4/ipv6 containers, backed by `rp_filter` sysctl on Linux; IPv6 enforcement requires VPP), per-interface rate tracking (1s sampler with `show interface rate`, `monitor interface rate`, 12 `ze_interface_*` Prometheus gauges, web rate columns). Local privileged integration covers first-apply non-adoption, reload deletion scoped to previously managed links, and rollback of created kernel links after partial apply failure; target-runner evidence is still required before production deployment claims. Every kernel route the interface layer installs (DHCP, RA, PPPoE, PPP) carries `rtm_protocol` 253, the `ze-iface` producer id, and a delete matches on that protocol, so one producer's expiry cannot remove an operator static route. A default route learned on a unit takes metric 254 by default (`route-priority`, range 0..4294966271), on an interface unit and on a `pppoe-client`. <!-- source: internal/core/rtproto/rtproto.go -- Iface Proto = 253, name ze-iface --> <!-- source: internal/component/iface/yang/ze-iface-conf.yang -- leaf route-priority default 254 --> The NTP client is compile-out-able with `ze_ntp` (the `show system` NTP block reads it through the nil-safe registry seam) and the class-of-service profiles with `ze_cos`; the dynamic per-subscriber CoS handler additionally requires `ze_l2tp` (a BNG feature: no sessions, no dynamic CoS). <!-- source: feature-gates.txt -- ze_ntp, ze_cos --> <!-- source: internal/plugins/cos/handler_off.go -- ze_l2tp-off stub -->

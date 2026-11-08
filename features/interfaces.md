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
| Real-path tests | test/plugin/iface-tunnel-kinds.ci, test/plugin/iface-vlan-qos.ci, test/plugin/iface-cli-ops.ci, test/plugin/iface-verbs.ci, test/plugin/iface-rate-json.ci, test/plugin/iface-ensure-rollback.ci, test/plugin/iface-learned-route-metric.ci, test/plugin/iface-route-protocol-name.ci, test/parse/iface-rpf-check.ci, test/parse/iface-netlink-accepts-wireguard.ci, test/parse/pppoe-client-keepalive.ci, test/parse/pppoe-client-ipv6.ci, test/parse/pppoe-client-ipv6-requires-enabled.ci, test/parse/iface-delegated-prefix-unknown-source.ci, test/parse/iface-delegated-prefix-subnet-id-invalid.ci, test/reload/tx-iface-apply.ci |
| Interop | bgp/dhcpv6-client-renew-kea, bgp/dhcpv6-client-confirm-kea, pppoe/01-pppoe-chap-ipv4, pppoe/pppoe-ipcp-dns-rejected-ze-client, pppoe/pppoe-ipv6cp-ze-client, pppoe/pppoe-ipv6cp-rejected-ze-client, topology/pppoe-dhcpv6-pd-ze-client, topology/road-warrior |
| Docs | docs/features/interfaces.md, docs/guide/pppoe.md |
| Doc review | 2026-11-08: the PPPoE client IPv6 paragraph matches clientIPv6CP.receiveNak (adopts a valid suggested Interface-Identifier, a fresh one when it equals the last Nak Ze sent), receiveReject (a rejected Interface-Identifier stops IPv6CP with one WARN, IPCP stays), protocolRejected (an LCP Protocol-Reject naming IPv6CP stops it), connectedSession.applyIPv6CP (passes IPv6 on Opened, discards it on Closed, ze_pppoe_client_ipv6cp_opened), applyPPPoELinkLocal and applyPPPoEClientSysctls (link-local and net.ipv6.conf.<name>.* on the session device), reconcileDHCP (DHCPv6 on the pppoe-client device, DUID from the source interface) and verifyDelegatedPrefixes, requestsDelegation and deriveDelegatedAddress (the delegated-prefix use and its refusals); the three added interop scenarios exist under test/interop-pppoe/scenarios and test/interop-topology/scenarios, and the four added parse tests under test/parse. Earlier, 2026-11-08: the PPPoE client DNS paragraph matches ipcpInitialRequest (IP-Address, Primary and Secondary DNS at 0.0.0.0), adoptNakDNS (only a unicast Nak value is adopted), the Configure-Reject branch of negotiateIPCP (a Rejected DNS option is dropped, a Rejected IP-Address fails) and PPPoEClient.endSessionLeases; both named interop scenarios exist under test/interop-pppoe/scenarios; the docs/features/interfaces.md DNS row and the RA paragraph match Resolvers and wanResolver.holdsDefault (one WARN when a WAN starts failing, one INFO when it resolves). Earlier, 2026-11-07: the PPPoE client keepalive and reconnect paragraph matches the YANG leaves keepalive interval and failures and reconnect delay-min and delay-max (defaults 10, 3, 1 and 60) in internal/component/iface/yang/ze-iface-conf.yang, keepaliveLoop (ends the session when echoFails reaches the configured failures), pppoeClientConfigChanged (timing excluded from the restart comparison) and setTiming (stored for the next session and the next wait). Earlier, 2026-10-20: the DHCPv6 move paragraph of docs/features/interfaces.md matches confirmAfterMove, dropAddresses and v6ConfirmCollector (Confirm after the CNF_MAX_DELAY wait, Success or silence keeps the lease, NotOnLink alone removes every address and solicits); both Interop scenarios exist under test/interop/scenarios; the row itself says target-runner evidence is still required; anchors (rtproto Iface 253, route-priority default 254, tunnel ttl defaults) exist at the named files |
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

The PPPoE client asks IPCP for the access network's DNS servers (RFC 1877
Primary and Secondary DNS), adopts the unicast values the access concentrator
Naks back, and keeps the session up when it Rejects them. The learned servers
become the session's resolver record, replaced when a renegotiation changes
them and withdrawn when the session ends, and `show interface pppoe-client`
reports them as `dns-servers`. The accel-ppp interop scenarios
`01-pppoe-chap-ipv4` and `pppoe-ipcp-dns-rejected-ze-client` prove both halves.
<!-- source: internal/component/l2tp/pppoeclient/session.go -- ipcpInitialRequest, adoptNakDNS -->
<!-- source: internal/component/iface/pppoe_client.go -- PPPoEClient.endSessionLeases -->
<!-- source: internal/le/interoplab/pppoe/check_client.go -- checkZeClient -->
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

The PPPoE client carries IPv6 when its `ipv6 enabled` is true: it negotiates
IPv6CP beside IPCP (RFC 5072), each Configure-Request carrying one
Interface-Identifier. A valid identifier the access concentrator Naks back is
adopted for the next request, and a fresh one is chosen when the suggestion
equals the one Ze last Naked (RFC 5072 Section 4.1). Once IPv6CP opens, the
session device passes IPv6 and holds the link-local address formed from the
negotiated identifier. An access concentrator that sends an LCP Protocol-Reject
for IPv6CP, Configure-Rejects the identifier, or never answers stops IPv6CP
only: IPCP and IPv4 stay up, and a rejected identifier is logged at WARN. The
`accept-ra`, `autoconf` and `forwarding` leaves become the
`net.ipv6.conf.<name>.*` sysctls of the session device each time a session
creates it, so the client can take its default route and a SLAAC address from
the access concentrator's Router Advertisements. A `dhcpv6` block runs the
DHCPv6 client on the session device, with its DUID-LL and IAIDs built from the
source interface unless `duid` is set, and `pd { length }` asks for a delegated
prefix. A LAN unit numbers from that prefix with `ipv6 delegated-prefix
<pppoe-client> { subnet-id; interface-identifier }`: it holds the /64 at
`subnet-id` with the remaining lifetimes of the delegation, and its Router
Advertisements follow the CE rules of the IPv6 Router Advertisements feature.
Config verify refuses a `subnet-id` the delegated length cannot hold, two units
taking one id from one source, and a source that is not a `pppoe-client` whose
`dhcpv6` asks for a prefix; the editor completes the source from that same set.
`ze_pppoe_client_ipv6cp_opened{name}` and `ze_iface_delegated_prefixes_held{name}`
report the state. The accel-ppp scenarios `pppoe-ipv6cp-ze-client` and
`pppoe-ipv6cp-rejected-ze-client` prove the negotiated link-local, the RA default
route and SLAAC, and an IPv4 session that survives a rejected IPv6CP; the
`pppoe-dhcpv6-pd-ze-client` topology scenario proves the delegation reaching a
LAN host. `docs/guide/pppoe.md`, "PPPoE client IPv6", is the operator guide.
<!-- source: internal/component/l2tp/pppoeclient/ipv6cp_client.go -- receiveNak, receiveReject, protocolRejected, applyIPv6CP -->
<!-- source: internal/component/iface/pppoe_client_ipv6.go -- parsePPPoEClientIPv6, applyPPPoEClientSysctls, applyPPPoELinkLocal -->
<!-- source: internal/component/iface/register.go -- reconcileDHCP -->
<!-- source: internal/component/iface/delegated_prefix.go -- verifyDelegatedPrefixes, requestsDelegation, deriveDelegatedAddress, delegationSourceReference -->
<!-- source: internal/component/iface/delegated_consumer.go -- installDelegatedAddress -->

Linux interface management via netlink: ethernet, dummy, veth, bridge, loopback, VLAN (with 802.1p ingress/egress QoS maps translating PCP to internal priority and back, and named class-of-service profiles via the cos plugin with interface-level inheritance and per-unit override/opt-out; dynamic per-subscriber CoS via RADIUS Filter-Id "cos:\<name\>" or vendor-specific attributes (Cisco-AVPair, Juniper ERX, Nokia, Huawei) applied on session-up with mid-session CoA changes and session-down revert; MikroTik Mikrotik-Rate-Limit VSA for shaper rate extraction <!-- source: internal/plugins/iface/netlink/manage_linux.go -- CreateVLAN QoS maps, UpdateVLANQoSMap --> <!-- source: internal/plugins/cos/config.go -- parseAndRegisterProfiles --> <!-- source: internal/plugins/cos/handler.go -- dynamic CoS event handler --> <!-- source: internal/component/l2tp/plugins/authradius/extract_vsa.go -- vendor VSA CoS/rate extraction -->), 9 tunnel kinds (GRE, GRETAP, IP6GRE, IP6GRETAP, IPIP, SIT, IP6TNL, IPIP6, VXLAN with a 24-bit VNI over UDP 4789 per RFC 7348), each carrying an outer-header TTL of 64 when the config names none, so an encapsulated packet survives a multi-hop underlay; `ttl 0` still asks the kernel to inherit the inner packet's TTL, and the default reaches netlink-backed tunnels only, because the VPP tunnel calls carry no TTL field, so a commit that names a `ttl` on a vpp-backed tunnel is refused by path rather than accepted and dropped <!-- source: internal/component/iface/yang/ze-iface-conf.yang -- container encapsulation choice kind, ttl and hoplimit defaults --> <!-- source: internal/component/iface/tunnel.go -- tunnelSchema, applyDefaults --> <!-- source: internal/plugins/iface/netlink/tunnel_linux.go -- vxlan link build -->, and WireGuard (declarative peers with `$9$`-encoded keys); DHCP (config-driven, routes, DNS, NTP discovery), NTP client (clock sync, RTC, time persistence, max-step safety cap), monitoring, migration, mirroring, per-interface offload and steering (GRO, GSO, SG, TSO, LRO, hw-tc-offload via kernel ethtool ioctl; RPS, RFS via sysfs; boolean three-state: true/false/absent), per-family reverse path filtering (`rpf-check strict\|loose\|disable` in ipv4/ipv6 containers, backed by `rp_filter` sysctl on Linux; IPv6 enforcement requires VPP), per-interface rate tracking (1s sampler with `show interface rate`, `monitor interface rate`, 12 `ze_interface_*` Prometheus gauges, web rate columns). Local privileged integration covers first-apply non-adoption, reload deletion scoped to previously managed links, and rollback of created kernel links after partial apply failure; target-runner evidence is still required before production deployment claims. Every kernel route the interface layer installs (DHCP, RA, PPPoE, PPP) carries `rtm_protocol` 253, the `ze-iface` producer id, and a delete matches on that protocol, so one producer's expiry cannot remove an operator static route. A default route learned on a unit takes metric 254 by default (`route-priority`, range 0..4294966271), on an interface unit and on a `pppoe-client`. <!-- source: internal/core/rtproto/rtproto.go -- Iface Proto = 253, name ze-iface --> <!-- source: internal/component/iface/yang/ze-iface-conf.yang -- leaf route-priority default 254 --> The NTP client is compile-out-able with `ze_ntp` (the `show system` NTP block reads it through the nil-safe registry seam) and the class-of-service profiles with `ze_cos`; the dynamic per-subscriber CoS handler additionally requires `ze_l2tp` (a BNG feature: no sessions, no dynamic CoS). <!-- source: feature-gates.txt -- ze_ntp, ze_cos --> <!-- source: internal/plugins/cos/handler_off.go -- ze_l2tp-off stub -->

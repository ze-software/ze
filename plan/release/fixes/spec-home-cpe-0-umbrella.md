# Spec: home-cpe-0-umbrella

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | - |
| Release reason | Ze as the router of a home FTTP line, replacing a VyOS CPE. Children that fix shipped defects belong in `release/fixes/` on their own merit. |
| Scope | product |
| Depends | - |
| Phase | 0/4 |
| Handoff | - |
| Updated | 2026-10-31 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

`plan/home-cpe/` sits outside the plan folders that inventory tooling reads,
by the owner's decision. The owner's own configuration and his provider's
settings are kept out of the repository; nothing here is drawn from them
beyond the generic capability a home CPE needs. Every value in this spec is
invented.

## Task

Run Ze as the CPE of a residential FTTP line delivered as PPPoE, where the
wholesale access network carries the session from a LAC to the retail ISP's LNS
over L2TP, and migrate an existing VyOS configuration to it.

No converter tool is written. A model (Claude Opus 5.5) reads the VyOS
configuration and writes the Ze configuration. What this umbrella owns is
everything except a translator program:

1. Ze implements every capability a home CPE of this kind uses, correctly.
2. A written VyOS-to-Ze mapping guide makes the conversion reproducible and
   reviewable.
3. A lab that re-creates the access chain proves the converted router works.

## Capabilities a home FTTP CPE needs

| Area | Capability | Ze today |
|------|-----------|----------|
| WAN | PPPoE client, CHAP, MTU 1492, no default route from PPP | supported |
| WAN | Static default route via the PPPoE interface by its configured name | broken: the name never reaches the kernel `pppN` device (child 1) |
| WAN | LCP renegotiation started by the LNS after the LAC completed LCP (common in wholesale L2TP access) | broken: the request is dropped (child 12) |
| WAN | DNS learned from IPCP (RFC 1877) | missing (child 13) |
| WAN | IPv6: IPv6CP, DHCPv6-PD, /64 and RAs on the LAN | missing (child 8) |
| WAN | MSS clamp on SYN | clamps on the stack: Ze writes the value unconditionally, but the kernel's nft exthdr set never raises an MSS; unproven by test and misdescribed in YANG (child 21); no clamp-to-PMTU (child 9) |
| LAN | Bridge with several members and addresses; ethernet MAC match and offloads | supported |
| LAN | DHCPv4 server, several subnets on one bridge, static reservations | broken: a reservation in a later subnet can get a lease from the first (child 2) |
| Firewall | Input filtering v4/v6, state, sets, global options | supported |
| Firewall | Masquerade out the PPPoE interface | works only with the kernel name (child 1) |
| Firewall | Conntrack helpers (ftp, h323, pptp, sip, tftp) | not built into Ze's kernel, never assigned (child 3, owner decision) |
| VPN | WireGuard road-warrior server with PSK peers | supported, no runtime proof yet |
| VPN | IKEv2 initiator, X.509, FQDN remote, dynamic WAN address | partial: local address fixed at apply, no NAT detection with an FQDN (child 4) |
| VPN | X.509 with EdDSA keys (RFC 8420) | missing (child 5) |
| VPN | Route-based tunnel carrying IPv6 inside an IPv4 SA, selectors narrowed by the responder | selectors supported; datapath unproven, suspected defect (children 6, 20) |
| System | SSH, users with keys, NTP, name servers, rollback | supported |
| System | sha512-crypt passwords | bcrypt only: passwords are set again |
| System | Kernel hostname on the appliance | missing (child 7) |
| System | Remote REST API over TLS with named keys | missing; gRPC over TLS is the route |
| System | Monitoring container | none; the Prometheus exporter with Netdata-named metrics replaces it |

## The spec set

| Child | Delivers | Kind |
|-------|----------|------|
| 1 | A logical interface name (a `pppoe-client` name, any non-ethernet name) resolves to its kernel device in static routes, firewall and NAT, and follows a PPPoE unit renumber | release fix |
| 2 | DHCPv4 server picks the subnet a request belongs to, static reservations first across every subnet on the interface | release fix |
| 3 | Conntrack helpers: kernel modules with their NAT companions, nft `ct helper` objects assigned by port in ip and ip6, names no kernel module provides (`nfs`, `sqlnet`) refused or served by a user-space helper | release fix, after the owner's decision |
| 4 | IKE follows a dynamic WAN address (boot before the WAN is up, PPPoE reconnect); NAT detection with an FQDN remote; keeps retrying while the FQDN does not resolve | release fix |
| 5 | RFC 8420 Ed25519 and Ed448 in IKEv2 RFC 7427 signature authentication, sign and verify | RFC owed work |
| 6 | Route-based tunnel carrying IPv6 inside an IPv4 SA: propose both families, accept a responder narrowing to one, install policy on the VTI's if_id; interop proof against strongSwan | release fix and validation |
| 7 | SSH accepts `0.0.0.0` and `::` on one port then fails one bind (validate refuses it); kernel hostname on the appliance | release fix |
| 8 | IPv6 CPE chain: IPv6CP in the PPPoE client, DHCPv6-PD on the PPPoE interface, /64 on the LAN, RAs from the delegated prefix | feature |
| 9 | Clamp-to-PMTU action | feature |
| 10 | VyOS-to-Ze mapping guide the model follows | documentation |
| 11 | The lab (below) | validation |
| 12 | PPPoE client renegotiates LCP on a Configure-Request received in Opened (RFC 1661 Section 4.1), then re-authenticates and runs IPCP | release fix, blocks the migration |
| 13 | PPPoE client requests RFC 1877 DNS in IPCP and uses the learned servers, ranked after static name servers; handles a Configure-Reject of the DNS options (today any Reject ends the session, RFC 1661 Section 5.4) | feature, P3 |
| 14 | LAC runs LCP and CHAP with the subscriber, selects the LNS by the realm of the username, and sends the proxy LCP AVPs; reuses `post-release/features/spec-l2tp-proxy-authentication.md` for the proxy authentication AVPs | feature |
| 15 | LNS option `proxy-lcp accept\|renegotiate` (default `accept`) to renegotiate LCP instead of accepting proxy LCP (RFC 2661 Section 4.4.5; proxy authentication is Section 9.5). The L2TP guide, five Go comments and `rfc/short/rfc1661.md` cite a nonexistent "Section 18" and the guide says the LNS can already renegotiate; both are corrected | feature |
| 16 | LNS MRU leaf (an L2TP session falls back to 1500 today) and an authentication allow-list so CHAP-only refuses a fallback to PAP | release fix |
| 17 | PPPoE client echo interval, failure count and reconnect hold-off configurable (fixed at 10 s by 3, backoff 1 to 60 s today) | feature, optional |
| 18 | Interop lab harness: one scenario declares several named links and takes members from any suite's images (Ze nodes, FreeRADIUS, strongSwan, wg peer, dhcpcd, Knot or Unbound). Today each suite runs alone with one network per container, multiple links exist only in the BGP suite, and no scenario runs two Ze nodes | lab tooling |
| 19 | Access-chain scenario: Ze CPE, Ze LAC (`relay ""`), Ze LNS, FreeRADIUS; the first end-to-end proof of the PPPoE-to-L2TP relay data path. Depends on 18 and `release/delivery/spec-docker-hosts-run-the-ze-kernel.md` | lab scenario |
| 20 | xfrm tunnel state for IPv6 inside IPv4: Ze installs tunnel states without `XFRM_STATE_AF_UNSPEC` and with no selector; the kernel's `copy_from_user_state` then gives the selector the outer family and state lookup skips the state for an IPv6 flow (read in the Ze kernel source). Needs a hunk in the vendored netlink patch. Proven red first, then fixed | release fix, P1 |
| 21 | MSS clamp proof: `tcp-mss` and `tcp-mss-set` write the value without comparing, and the kernel skips a write that would raise it (`nft_exthdr_tcp_set_eval`: "increase can cause connection to stall"). Owed: a netns test on Ze's kernel proving a smaller MSS survives, and YANG and comment text that say clamp | validation, P2 |
| 22 | IKE initiator refuses a responder-started reauthentication (a new IKE_SA_INIT reaches only `respond` peers) | release fix, low |

Child spec files, all in `plan/home-cpe/`:

| Child | File |
|-------|------|
| 1 | `spec-home-cpe-1-logical-interface-name.md` |
| 2 | `spec-home-cpe-2-dhcp-shared-interface-subnets.md` |
| 3 | `spec-home-cpe-3-conntrack-helpers.md` |
| 4 | `spec-home-cpe-4-ike-dynamic-wan.md` |
| 5 | `spec-home-cpe-5-ike-eddsa.md` |
| 6 | `spec-home-cpe-6-ipsec-ipv6-in-ipv4-vti.md` |
| 7 | `spec-home-cpe-7-system-small-defects.md` |
| 8 | `spec-home-cpe-8-ipv6-cpe-chain.md` |
| 9 | `spec-home-cpe-9-mss-clamp-to-pmtu.md` |
| 10 | `spec-home-cpe-10-vyos-mapping-guide.md` |
| 11 | `spec-home-cpe-11-lab.md` |
| 12 | `spec-home-cpe-12-pppoe-client-lcp-renegotiation.md` |
| 13 | `spec-home-cpe-13-pppoe-client-ipcp-dns.md` |
| 14 | `spec-home-cpe-14-lac-realm-relay.md` |
| 15 | `spec-home-cpe-15-lns-lcp-renegotiation.md` |
| 16 | `spec-home-cpe-16-lns-mru-and-auth-allow-list.md` |
| 17 | `spec-home-cpe-17-pppoe-client-keepalive-config.md` |
| 18 | `spec-home-cpe-18-interoplab-multi-link-topology.md` |
| 19 | `spec-home-cpe-19-access-chain-scenario.md` |
| 20 | `spec-home-cpe-20-xfrm-cross-family-tunnel-state.md` |
| 21 | `spec-home-cpe-21-mss-clamp-not-raise.md` |
| 22 | `spec-home-cpe-22-ike-responder-reauth.md` |

Existing specs the children reuse rather than duplicate:
`release/validation/spec-wireguard-runtime-proof.md` (the road-warrior
scenario is its acceptance criteria), `release/validation/spec-ipsec-0-umbrella.md`
(parent of the IPsec scenarios), `release/fixes/spec-ipsec-11-mobike.md`
(check whether child 4 fits there),
`release/delivery/spec-ze-loads-kernel-modules-on-vendor-kernels.md` (owns the
helper kernel row; child 3), `release/fixes/spec-network-services-one-off-defects.md`
(a home for children 2 and 21 as journal rows).

Order: 12, 1, 2, 21 and 7 first (each makes an accepted configuration do the
wrong thing); then 5, 4, 20 and 6 (the management tunnel); then 18, 19 and the
remaining scenarios, with 14 and 15 for the wholesale-shaped variant; 3 after
the owner's decision; 8, 9, 13, 16, 17 and 22 last.

Doc defects found during the survey, owed with the child that touches each page:
`docs/guide/ipsec.md` (PKI example uses leaf names that do not exist; implies
Ed25519 works), `docs/guide/static-routes.md` (`interface pppoe0` example does
not resolve), `docs/guide/configuration.md` (conntrack module list omits two
accepted names), `docs/guide/l2tp.md` (says `l2tp-ppp` exercises the LAC
bridge, and that the LNS can renegotiate proxy LCP).

## Conversion by the model

| Step | What happens | Check |
|------|--------------|-------|
| Input | The VyOS configuration with secrets redacted | The model never receives passwords, keys or PSKs |
| Mapping | The model follows the mapping guide (child 10): every VyOS construct, its Ze form, and the constructs with no equivalent | Every source line is mapped or listed as dropped with a reason |
| Output | A Ze configuration with placeholders for secrets | `ze config validate` is clean |
| Secrets | The owner fills the placeholders on the router; passwords are re-entered as bcrypt | No secret passes through the model |
| Proof | The converted configuration, with lab values substituted, runs in the lab | Child 11 scenarios green |

The mapping guide records the rewrites the survey found: `ppp*` for the PPPoE
device until child 1 lands, `icmpv6` for `ipv6-icmp`, no `protocol` for `all`,
`aes128gcm` for `aes128gcm128`, address-groups as nft sets, default-log as a
final term, SSH on `::` only, an IKE peer with no `local-address`, explicit
`pfs disable` where VyOS left the ESP group unattached, REST automation moved
to gRPC with TLS, containers replaced by the Prometheus exporter, and unused BGP
policy dropped.

## Lab

Ze plays the CPE, the LAC and the LNS. FreeRADIUS, strongSwan, a WireGuard
peer, DHCP clients and DNS complete it. Everything runs on Ze's kernel. All
values are invented: documentation prefixes, ULA, `example.net` names.

### Topology

```
 LAN segment (bridge)            access segment (L2)          core segment (IP)
+------------+                  +---------+                  +---------+----------+-----------+
| lan-client |--+            +--| Ze LAC  |--L2TP over UDP---| Ze LNS  | RADIUS   | DNS       |
| (dhcp)     |  |            |  +---------+                  +---------+----------+-----------+
+------------+  |  +------+  |                                    |
                +--|Ze CPE|--+ PPPoE                       internet segment (IP, routed by LNS)
+------------+  |  +------+                               +-------------+-----------+-----------+
| lan-res-a  |--+    br0 + eth members, wg0, vti0         | strongSwan  | wg-client | web/ftp   |
| lan-res-b  |--+                                         | responder   |           | server    |
+------------+                                            +-------------+-----------+-----------+
```

| Segment | Prefix | Members |
|---------|--------|---------|
| LAN | 192.168.50.0/24 and 10.99.0.0/24 on one bridge | CPE br0, a range client, one reserved client per subnet |
| access | none (Ethernet, PPPoE only) | CPE WAN port, LAC subscriber port |
| core | 192.0.2.0/24 | LAC, LNS, FreeRADIUS, DNS |
| subscriber | RADIUS hands 203.0.113.1/32; pool fallback 100.64.0.0/24 | CPE ppp interface |
| internet | 198.51.100.0/24, routed by the LNS | strongSwan responder `vpn.example.net`, WireGuard peer, a web and FTP server |
| management | fd00:db8:1::/112 inside the tunnel | CPE vti0, responder |
| road warriors | 10.88.0.0/24 | CPE wg0, wg-client |

### Node configuration

| Node | Configured as |
|------|---------------|
| Ze CPE | A configuration converted by the model from a VyOS CPE configuration, with lab values |
| Ze LAC | PPPoE server on the access segment; `relay ""` to the LNS remote with a tunnel secret. Realm variant once child 14 exists |
| Ze LNS | L2TP with a tunnel secret, CHAP only, RADIUS with NAS-Identifier and source address, interim accounting, MRU 1492, pool fallback, DNS in IPCP, routes the internet segment. Renegotiating variant once child 15 exists |
| FreeRADIUS | User `cpe@lab.example.net` Accepted with Framed-IP-Address, a second user Accepted without one; Acct-Interim-Interval 60 s, the floor (RFC 2869 Section 5.16: "The value MUST NOT be smaller than 60.") |
| DNS | Answers `vpn.example.net`; can be started late |
| strongSwan | IKEv2 responder, CA with path length 0, P-256 server certificate; CPE certificate in Ed25519 and a P-256 variant; narrows selectors to `::/0`; ESP AES-GCM-128, no PFS; MOBIKE on |
| wg-client | Peer with PSK, dials the CPE's WAN address |
| LAN clients | dhcpcd in the range, and one with a reserved MAC in each subnet |

### Scenarios

Each scenario asserts state, not timing, and each is broken once on purpose to
show it goes red (`ai/rules/interop-and-goal-validation.md`).

| Scenario | Asserts | Needs |
|----------|---------|-------|
| access-up | PADO with an empty Service-Name, session through LAC to LNS, CHAP checked by RADIUS, Framed-IP on the CPE, DNS learned from IPCP, CPE pings the LNS | 18, 19 (13 for the DNS assertion) |
| access-pool | User without Framed-IP gets a pool address | 18, 19 |
| access-accounting | Accounting Start, interim, Stop at FreeRADIUS with the session's address and octets | 18, 19 |
| access-wholesale | LAC runs LCP and CHAP and picks the LNS by realm; LNS renegotiates LCP and re-challenges; the CPE comes up | 12, 14, 15 |
| access-reconnect | LNS drops the session; CPE reconnects with a new address; tunnel and NAT follow | 4, 18 |
| mgmt-tunnel | Ed25519 authentication, selectors narrowed to `::/0`, IPv6 ping across vti0, MOBIKE float to 4500; a CPE booted while DNS is down establishes once DNS answers | 4, 5, 6, 20 |
| lan-services | Range lease, both reservations honoured, masquerade to the web server, MSS 1400 written on SYN and a smaller MSS left alone, WAN input dropped, FTP through NAT via the helper | 2, 3, 21 |
| road-warrior | wg-client reaches a LAN host and the web server, masqueraded | 18 |
| conversion | The converted configuration passes `ze config validate` and is the CPE configuration of every scenario above | 10 |
| access-wholesale-accel-lns | Ze CPE behind the Ze realm LAC, with accel-ppp as LNS: accel-ppp ignores proxy LCP and renegotiates, the wholesale shape against third-party software. No accel-ppp setting makes it renegotiate on a direct PPPoE session, so this replaces a direct accel-ppp variant | 12, 14 |

### What exists to build on

The LAC relay is proven against xl2tpd as LNS for the control connection only
(`test/interop-l2tp/scenarios/03-ze-lac-xl2tpd-lns`); the relay data path has no
end-to-end proof. The Ze PPPoE client runs against accel-ppp
(`test/interop-pppoe`, IPv4 only). Ze with FreeRADIUS runs in
`test/interop-radius-subscriber`. The IPsec suite runs strongSwan with Ze as
X.509 initiator and VTI (IPv4 inner, PSK), MOBIKE and a real NAT box, but no
IPv6 inner, no FQDN remote and no EdDSA. The Ze DHCP server runs against
dhcpcd with one subnet. No test passes real traffic through masquerade, MSS
or WireGuard with Ze in the path. Ze's kernel carries PPPoE, L2TP, PPPoL2TP,
xfrm interfaces, XFRM_MIGRATE, WireGuard and the nft NAT pieces; every
conntrack helper is off.

## Open decisions for the owner

| Decision | Options |
|----------|---------|
| Conntrack helpers (child 3) | Ze's kernel leaves them out against NAT slipstreaming. Build them in and assign by port, or leave them out and let FTP, SIP and PPTP through NAT break |
| Where the children live | here under `plan/home-cpe/`, or in `release/` and `post-release/` where inventory tooling sees them |
| RFC 8420 (child 5) | It is in no release profile, so no summary is written and it counts nowhere until it is listed for one |
| Delegated prefix use (child 8) | `rfc/short/rfc9915.md` declines RFC9915-6.3-1/-2/-3; the IPv6 CPE chain needs that decline lifted |
| Learned DNS for LAN clients (child 13) | The LAN `dns-resolver` forwards only to configured servers; whether it should also use IPCP-learned servers |

Edits outside `plan/home-cpe/` that the children owe once they move to `ready`
(held back by the owner's no-change instruction): list
`vti-ipv6-inner-ipv4-outer` in `release/validation/spec-ipsec-0-umbrella.md`;
add a child 22 pointer to `post-release/features/spec-ike-reauth.md`; narrow
`post-release/features/spec-l2tp-proxy-authentication.md` to its LNS half
(child 14 takes the LAC half); move the hostname half of the
`plan/journal/unwired-feature.md` 2026-09-04 row to child 4; journal rows for
the client dropping Configure-Request, the pppoe-client name never reaching the
kernel, and static routes not reinstalled after a reconnect; fetch
`rfc/full/rfc8420.txt`.

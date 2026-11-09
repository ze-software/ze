# Spec: home-cpe-0-umbrella

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | - |
| Release reason | Ze as the router of a home FTTP line, replacing a VyOS CPE. Children that fix shipped defects belong in `release/fixes/` on their own merit. |
| Scope | product |
| Depends | - |
| Phase | 6/6 |
| Handoff | - |
| Updated | 2026-11-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

The home-CPE specs live in `plan/release/fixes/`, inside the plan folders
that inventory tooling reads. The owner's own configuration and his provider's
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
| WAN | Static default route via the PPPoE interface by its configured name | supported: the configured name is the kernel device and follows a reconnect (child 1, closed) |
| WAN | LCP renegotiation started by the LNS after the LAC completed LCP (common in wholesale L2TP access) | supported in every phase (child 12, closed) |
| WAN | DNS learned from IPCP (RFC 1877) | supported: requested in IPCP, a Reject of the DNS options handled (child 13, closed); its use beside static and other learned servers is the `dns-source` work (child 23 closed for the router's own resolver; child 23b, closed, for the LAN, DHCPv4, RA and subscriber consumers) |
| WAN | IPv6: IPv6CP, DHCPv6-PD, /64 and RAs on the LAN | supported (child 8, closed `87c7b998ad`) |
| WAN | MSS clamp on SYN | supported: the value is an upper bound and a smaller MSS is kept, proven on Ze's kernel and described as a clamp in YANG (child 21, closed); clamp-to-PMTU is the `tcp-mss-clamp-to-pmtu` leaf (child 9, closed) |
| LAN | Bridge with several members and addresses; ethernet MAC match and offloads | supported |
| LAN | DHCPv4 server, several subnets on one bridge, static reservations | supported: reservations first across every subnet on the interface (child 2, closed) |
| Firewall | Input filtering v4/v6, state, sets, global options | supported |
| Firewall | Masquerade out the PPPoE interface | supported by the configured name (child 1, closed) |
| Firewall | Conntrack helpers (ftp, h323, pptp, sip, tftp) | supported: built into Ze's kernel, assigned only when listed (child 3, closed; Option A, owner decision 2026-11-02) |
| VPN | WireGuard road-warrior server with PSK peers | supported; the runtime proof is child 11's `road-warrior` scenario |
| VPN | IKEv2 initiator, X.509, FQDN remote, dynamic WAN address | supported: follows the WAN address, NAT detection with an FQDN, retries while the name does not resolve (child 4, closed); IKE over IPv6 is the open 2026-11-07 row of `plan/journal/unwired-feature.md` |
| VPN | X.509 with EdDSA keys (RFC 8420) | Ed25519 signs and verifies (child 5, closed); the RFC 8420 summary is `plan/post-release/features/spec-ike-rfc8420-eddsa-conformance.md` |
| VPN | Route-based tunnel carrying IPv6 inside an IPv4 SA, selectors narrowed by the responder | supported: scenario `vti-ipv6-inner-ipv4-outer` (child 6, closed) with the cross-family state fix (child 20, closed) |
| System | SSH, users with keys, NTP, name servers, rollback | supported |
| System | sha512-crypt passwords | bcrypt only: passwords are set again |
| System | Kernel hostname on the appliance | supported, with the dual-wildcard SSH listener refused at validate (child 7, closed) |
| System | Remote REST API over TLS with named keys | missing; gRPC over TLS is the route |
| System | Monitoring container | none; the Prometheus exporter with Netdata-named metrics replaces it |

## The spec set

| Child | Delivers | Kind |
|-------|----------|------|
| 1 | A logical interface name (a `pppoe-client` name, any non-ethernet name) resolves to its kernel device in static routes, firewall and NAT, and follows a PPPoE unit renumber | release fix |
| 2 | DHCPv4 server picks the subnet a request belongs to, static reservations first across every subnet on the interface | release fix |
| 3 | Conntrack helpers: kernel modules with their NAT companions, nft `ct helper` objects assigned by port in ip and ip6, names no kernel module provides (`nfs`, `sqlnet`) refused or served by a user-space helper. Owner decision settled 2026-11-02: Option A (helpers built in, assigned only when listed, empty default list) | release fix |
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
| 14 | LAC runs LCP and CHAP with the subscriber, selects the LNS by the realm of the username, and sends the proxy LCP AVPs and the proxy authentication AVPs (AVP 33 hidden for PAP); it writes every LAC-side AVP itself, and `post-release/features/spec-l2tp-proxy-authentication.md` keeps the LNS half | feature |
| 15 | LNS option `proxy-lcp accept\|renegotiate` (default `accept`) to renegotiate LCP instead of accepting proxy LCP (RFC 2661 Section 4.4.5; proxy authentication is Section 9.5). The L2TP guide, comments in five Go files and `rfc/short/rfc1661.md` cite a nonexistent "Section 18" and the guide says the LNS can already renegotiate; both are corrected | feature |
| 16 | LNS MRU leaf (an L2TP session falls back to 1500 today) and an authentication allow-list so CHAP-only refuses a fallback to PAP | release fix |
| 17 | PPPoE client echo interval, failure count and reconnect hold-off configurable (fixed at 10 s by 3, backoff 1 to 60 s today) | feature, optional |
| 18 | Interop lab harness: one scenario declares several named links and takes members from any suite's images (Ze nodes, FreeRADIUS, strongSwan, wg peer, dhcpcd, Knot or Unbound). Today each suite runs alone with its own images, links exist only in the BGP suite, and several Ze nodes exist only inside one suite (BGP `ze-peer`, RSVP-TE roles); the shared declaration reuses that shape and replaces the per-suite link code | lab tooling |
| 19 | Access-chain scenario: Ze CPE, Ze LAC (`relay ""`), Ze LNS, FreeRADIUS; the first end-to-end proof of the PPPoE-to-L2TP relay data path. Depends on 18 and `release/delivery/spec-docker-hosts-run-the-ze-kernel.md` | lab scenario |
| 20 | xfrm tunnel state for IPv6 inside IPv4: Ze installs tunnel states without `XFRM_STATE_AF_UNSPEC` and with no selector; the kernel's `copy_from_user_state` then gives the selector the outer family and state lookup skips the state for an IPv6 flow (read in the Ze kernel source). Needs a hunk in the vendored netlink patch. Proven red first, then fixed | release fix, P1 |
| 21 | MSS clamp proof: `tcp-mss` and `tcp-mss-set` write the value without comparing, and the kernel skips a write that would raise it (`nft_exthdr_tcp_set_eval`: "increase can cause connection to stall"). Owed: a netns test on Ze's kernel proving a smaller MSS survives, and YANG and comment text that say clamp | validation, P2 |
| 22 | IKE initiator refuses a responder-started reauthentication (a new IKE_SA_INIT reaches only `respond` peers) | release fix, low |
| 23 | DNS source fall-through, part a: the resolver record registry, the shared selection rule and `dns-source` list grammar, the `system-dns` event, the router's own resolver as its first consumer, RFC 5625 | feature; answers the LAN-learned-DNS question child 13 left open |
| 23b | DNS source consumers: the LAN resolver, DHCPv4, RA and subscriber consumers, RADIUS DNS, `show system dns` | feature, depends on 23 |

Child spec files, all in `plan/release/fixes/`:

Every child is closed (checked 2026-11-09 with `ls
plan/release/fixes/spec-home-cpe-*`, which lists only this umbrella), and its
record is the spec text at the parent of its closure commit
(`git show <commit>^:plan/release/fixes/<file>`).

| Child | State | File, or closure commit |
|-------|-------|-------------------------|
| 1 | closed | `c5794805be` (`spec-home-cpe-1-logical-interface-name.md`) |
| 2 | closed | `2bce073236` (`spec-home-cpe-2-dhcp-shared-interface-subnets.md`) |
| 3 | closed | `d8584db704` (`spec-home-cpe-3-conntrack-helpers.md`) |
| 4 | closed | `4b3eee5598` (`spec-home-cpe-4-ike-dynamic-wan.md`) |
| 5 | closed | `209709901c` (`spec-home-cpe-5-ike-eddsa.md`) |
| 6 | closed | `9dabfc7a2a` (`spec-home-cpe-6-ipsec-ipv6-in-ipv4-vti.md`) |
| 7 | closed | `cede669879` (`spec-home-cpe-7-system-small-defects.md`) |
| 8 | closed | `87c7b998ad` (`spec-home-cpe-8-ipv6-cpe-chain`), after the nine RFC 7084 and RFC 9915 records were retaken in `8d83d8bde0` |
| 9 | closed | `403e3256f9` (`spec-home-cpe-9-mss-clamp-to-pmtu.md`) |
| 10 | closed | `3d1d4ecba7` (`spec-home-cpe-10-vyos-mapping-guide.md`) |
| 11 | closed | `dcc20ffb68` (`spec-home-cpe-11-lab.md`) |
| 12 | closed | `9bad2dd6bd` (`spec-home-cpe-12-pppoe-client-lcp-renegotiation.md`); PPPoE client LCP renegotiation in every phase documented in `docs/architecture/l2tp/cpe-1-pppoe-client.md` and `docs/guide/configuration.md` ("PPPoE Client") |
| 13 | closed | `ac001804c0` (`spec-home-cpe-13-pppoe-client-ipcp-dns.md`); IPCP-learned DNS documented in `docs/architecture/l2tp/cpe-1-pppoe-client.md` and `docs/features/dns-resolver.md` |
| 14 | closed | `4bef6da011` (`spec-home-cpe-14-lac-realm-relay`); `TestReloadAppliesRelayRealms` passed (`rwinv/out-gb7/c14-reload.log`) and the RFC2661-6.1-1 record was re-observed red; relay by realm documented in `docs/guide/l2tp.md` ("Relay by realm") |
| 15 | closed | `5ba9503304` (`spec-home-cpe-15-lns-lcp-renegotiation`); the `l2tp proxy-lcp` leaf, its session outcome and its counter label documented in `docs/guide/l2tp.md` ("PPP negotiation") |
| 16 | closed | `5b587ad9d2` (`spec-home-cpe-16-lns-mru-and-auth-allow-list`), its review follow-up `dc3514eaec` (`spec-home-cpe-16-mru-enforcement-and-review-fixes`); policy and MRU controls documented in `docs/guide/l2tp.md` and `docs/guide/pppoe.md` |
| 17 | closed | `1587411097` (`spec-home-cpe-17-pppoe-client-keepalive-config`); the `keepalive` and `reconnect` leaves documented in `docs/guide/pppoe.md` ("PPPoE client keepalive and redial") and `docs/architecture/l2tp/cpe-1-pppoe-client.md` |
| 18 | closed | `e063c582f9` (`spec-home-cpe-18-interoplab-multi-link-topology.md`) |
| 19 | closed | `bf94d63407` (`spec-home-cpe-19-access-chain-scenario.md`) |
| 20 | closed | `12fb5bbfe9` (`spec-home-cpe-20-xfrm-cross-family-tunnel-state.md`) |
| 21 | closed | `0a7cfc0ae1` (`spec-home-cpe-21-mss-clamp-not-raise.md`) |
| 22 | closed | `5d9241eaa6` (`spec-home-cpe-22-ike-responder-reauth.md`) |
| 23 | closed | `62c14b96ca` (`spec-home-cpe-23-dns-source-fall-through`, 23a: the resolver registry, the router consumer, RFC 5625); its design is in `docs/features/dns-resolver.md` |
| 23b | closed 2026-11-09 | `spec-home-cpe-23b-dns-source-consumers` (the LAN resolver, DHCPv4, RA and subscriber consumers, RADIUS DNS, `show system dns`; depends on 23) |

Review correction and completion (2026-11-06): the owner found eight defects
in the earlier child16 closure. The follow-up
`spec-home-cpe-16-mru-enforcement-and-review-fixes` split them into 16a through 16h;
all eight now have implementation, independent review, scoped central proof and
formal closure. Omitted and valid proxied MRUs preserve useful authenticated
service while Ze installs the session transmit cap. The dated owner requirement and its derived design live in
`docs/architecture/l2tp/subscriber-session-model.md`, Subscriber transmit limit.
The durable operator contracts and reproduction procedures are `docs/guide/l2tp.md`, `docs/guide/pppoe.md`,
`docs/labs/l2tp-interop.md` and `docs/labs/pppoe-interop.md`. The final coordination
record is retained in its closure commit history; these surviving pages carry
the operator behavior. Full subscriber IPv6 remains the previously owner-approved
post-release feature in `plan/post-release/features/spec-l2tp-ipv6-subscriber.md`.

Existing specs the children reuse rather than duplicate:
`release/validation/spec-wireguard-runtime-proof.md` (the road-warrior
scenario is its acceptance criteria), `release/validation/spec-ipsec-0-umbrella.md`
(parent of the IPsec scenarios), `release/fixes/spec-ipsec-11-mobike.md`
(check whether child 4 fits there),
`release/delivery/spec-ze-loads-kernel-modules-on-vendor-kernels.md` (owns the
helper kernel row; child 3), `release/fixes/spec-network-services-one-off-defects.md`
(a home for children 2 and 21 as journal rows).

### Dependencies, scenario ownership and order

Each child separates two kinds of dependency in its `Depends` row: an
implementation prerequisite (code or a harness the child builds on, needed
before its first commit) and a closure dependency (another child that must
have landed before the child's own scenarios can go green and the spec can
close). A child's implementation lands as soon as its prerequisites have
landed; only its closure waits.

Implementation prerequisites:

| Child | Needs implemented first |
|-------|-------------------------|
| 12, 16, 18, 1, 2, 3, 7, 10, 20, 21, 22, 5, 4 | none within the set |
| 6 | 20's fix committed (not closed); both specs carry a "Dependency and closure order" section |
| 13 | 12 (the change record and the re-enterable `negotiateIPCP`) |
| 17 | 12 (`keepaliveLoop` and the scripted access concentrator) |
| 14 | 16 (the PPPoE allow-list the LAC's own authentication follows) |
| 15 | none |
| 19 | 18, 16 (LNS `mru` and allow-list leaves), `release/delivery/spec-docker-hosts-run-the-ze-kernel.md` |
| 8 | 1 (its topology proof needs 18 for closure) |
| 9 | 21 (its netns test harness and the proof that the MSS write never raises) |
| 11 | every child whose scenario it composes |
| 23 | 13 (the IPCP-learned `ppp` lease) |
| 23b | 23 (the record registry, the selection rule, the list grammar, the `system-dns` event) |

Scenario ownership: the owner creates the scenario directory and checker,
records its red walk, and cites it as closure evidence. Another child's
assertion inside a scenario is owned by that child and holds only that child's
closure.

| Scenario | Owner | Closure needs implemented |
|----------|-------|---------------------------|
| `two-ze-pppoe-link`, `ze-static-link-chain` | 18 | - |
| `access-up`, `access-pool`, `access-accounting` | 19 | 18, 16 |
| `access-up` DNS clause (child 19 AC-6) | 13 | 19 |
| `access-wholesale` | 15 | 12, 14, 16, 18, 19 |
| `access-wholesale-accel-lns` | 12 | 14, 18 |
| `l2tp-lac-realm-relay-accel-ppp`, `l2tp-lac-realm-relay-xl2tpd` | 14 | 18 |
| `l2tp-lns-lcp-renegotiate-pppd` | 15 | 14, 18 |
| `l2tp-lns-chap-only-xl2tpd`, `pppoe-chap-only-pap-client` | 16 | - |
| `01-pppoe-chap-ipv4` DNS extension, `pppoe-ipcp-dns-rejected-ze-client` | 13 | - |
| `vti-ipv6-inner-ipv4-outer` | 6 alone; its red walk reverts 20's fix, and 20 closes citing 6's run | 20 |
| child 8's topology proof | 8 | 18 |
| `pppoe-mss-pmtu` (not a home-CPE lab scenario: it does not run the converted CPE file) | 9 | 18 |
| `access-reconnect`, `mgmt-tunnel`, `lan-services`, `road-warrior`, `conversion` | 11 | the children in each row's Needs column of the Lab table |

Executable order. Implementation, landing each piece as it is complete:
12, 16, 1, 2, 21, 7, 3, 18, 17, 13, 14, 15, 19, 5, 4, 20, 6, 8, 9, 22, 10, 11,
23, 23b.
Closure, in an order where every closure dependency above is already
implemented: 16, 1, 2, 21, 7, 3, 17, 18, 19, 13, 14, 12, 15, 5, 4, 6, 20,
8, 9, 22, 10, 11, 23, 23b. Child 6 closes before 20 because 20's closure cites 6's
scenario run. Children 12 and 16 come first because each makes an
accepted configuration do the wrong thing; 12 lands its code first and closes
once `access-wholesale-accel-lns` can run. Child 23b closes after 23, whose
registry and grammar it consumes. Children 8 (`87c7b998ad`), 14
(`4bef6da011`), 23 (`62c14b96ca`) and 23b (`f0072e4a2e`) have closed, and the
umbrella closes after them and the lab rerun (Goal Validation).

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
| access-up | PADO with an empty Service-Name, session through LAC to LNS, CHAP checked by RADIUS, LNS session MRU 1492 and method CHAP-MD5, Framed-IP on the CPE, DNS learned from IPCP, CPE pings the LNS | 16, 18, 19 (13 for the DNS assertion) |
| access-pool | User without Framed-IP gets a pool address | 16, 18, 19 |
| access-accounting | Accounting Start, interim, Stop at FreeRADIUS with the session's address and octets | 16, 18, 19 |
| access-wholesale | LAC runs LCP and CHAP and picks the LNS by realm; LNS renegotiates LCP and re-challenges; the CPE comes up | 12, 14, 15, 16, 18, 19 |
| access-reconnect | LNS drops the session; CPE reconnects with a new address; tunnel and NAT follow | 4, 18 |
| mgmt-tunnel | Ed25519 authentication, selectors narrowed to `::/0`, IPv6 ping across vti0, MOBIKE float to 4500; a CPE booted while DNS is down establishes once DNS answers | 4, 5, 6, 20 |
| lan-services | Range lease, both reservations honoured, masquerade to the web server, MSS 1400 written on SYN and a smaller MSS left alone, WAN input dropped, FTP through NAT via the helper | 2, 3, 21 |
| road-warrior | wg-client reaches a LAN host and the web server, masqueraded | 18 |
| conversion | The converted configuration passes `ze config validate` and is the CPE configuration of every home-CPE lab scenario in this table (child 9's `pppoe-mss-pmtu` is not one) | 10 |
| access-wholesale-accel-lns | Ze CPE behind the Ze realm LAC, with accel-ppp as LNS: accel-ppp ignores proxy LCP and renegotiates, the wholesale shape against third-party software. No accel-ppp setting makes it renegotiate on a direct PPPoE session, so this replaces a direct accel-ppp variant | 12, 14, 18 |

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

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/interop.md` - suites, scenario discovery, vacuity traps
  → Constraint: scenarios are named, never numbered; each lab scenario is broken once at its producer and the red recorded (children 11, 18, 19)
- [ ] `docs/architecture/iface/logical-name-resolution.md` - how a configured interface name becomes a kernel device
  → Constraint: interface names go through the shared resolver, never straight to the kernel (`ai/doors/forwarding.md`); child 1 extends it to the `pppoe-client` name
- [ ] `docs/architecture/l2tp/cpe-1-pppoe-client.md`, `docs/architecture/l2tp/subscriber-session-model.md` - the PPPoE client and the LAC/LNS session model
  → Constraint: subscriber auth, pool and shaper reach a session through the `l2tp/subscriber` handler registries (`ai/doors/access-security.md`); children 12 to 17 extend those paths
- [ ] `ai/doors/config-yang.md` Traps - `ze config validate` never calls a live `OnConfigVerify`
  → Decision: the conversion check validates offline, then starts Ze with the file (children 10, 11)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc1661.md`, `rfc/short/rfc2661.md`, `rfc/short/rfc9915.md` - PPP, L2TP and DHCPv6 rows the children change
  → Constraint: each child names its own RFC rows; the umbrella adds none

**Key insights:**
- Every child carries its own Current Behavior, Wiring Test and TDD plan; this umbrella holds the split, the order, the lab and the owner decisions.
- All values in this spec set are invented (RFC 5737, RFC 3849, ULA, `example.net`).

## Current Behavior (MANDATORY)

**Source files read:** each child's Current Behavior section lists the producers it read; the umbrella's capability table is drawn from them. The entry points the set touches:
- [ ] `internal/component/l2tp/pppoeclient/` - the PPPoE client (children 1, 12, 13, 17)
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `pppoe-client`, `bridge`, `ethernet`, `wireguard` (children 1, 8)
- [ ] `internal/plugins/dhcpserver/` - the DHCPv4 server (child 2)
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` - `masquerade`, `tcp-mss-set` (children 3, 9, 21)
- [ ] `internal/component/ike/engine/`, `internal/component/ike/dataplane/` - IKEv2 and XFRM install (children 4, 5, 6, 20, 22)
- [ ] `internal/le/interoplab/` - the interop lab (children 11, 18, 19)

**Behavior to preserve:** every accepted configuration that works today keeps working; listed per child.

**Behavior to change:** listed per child; the capability table above names each change.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Interop scenario discovery | `internal/le/interoplab/discover.go` `Discover` | lists named scenario directories and refuses one with no checker | extends: children 11, 18 and 19 add scenarios and a topology suite through it |
| LAC relay against xl2tpd | `test/interop-l2tp/scenarios/03-ze-lac-xl2tpd-lns` | proves the LAC relay control connection only | extends: child 19 adds the first end-to-end data-path proof |
| PPPoE client interop | `test/interop-pppoe/scenarios/` | Ze client against accel-ppp, IPv4 only | extends: children 11, 12 and 13 add scenarios there |
| ExaBGP migration | `internal/exabgp/migration/migrate.go` `MigrateFromExaBGP` | converts an ExaBGP configuration in code | does not apply: the VyOS conversion is done by a model following the guide (child 10), with no converter program |
| WireGuard runtime proof spec | `plan/release/validation/spec-wireguard-runtime-proof.md` | owns the WireGuard road-warrior proof | extends: the `road-warrior` scenario is its acceptance evidence |
| IPsec umbrella | `plan/release/validation/spec-ipsec-0-umbrella.md` | parent of the IPsec scenarios | extends: `mgmt-tunnel` is a scenario under it |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A VyOS configuration, redacted, read by the model with the mapping guide (child 10).
- The Ze configuration it produces, loaded by `ze` on the CPE.
- Wire: PPPoE on the WAN port, L2TP between LAC and LNS, IKEv2 and WireGuard over the PPP session, DHCPv4 on the LAN bridge.

### Transformation Path
1. The model converts the VyOS configuration to Ze configuration with placeholders and a conversion record (child 10).
2. `ze config validate`, then a Ze start with lab values substituted (child 11 `conversion`).
3. Each child's path from config to kernel or wire, as its own Data Flow states.
4. The lab scenarios assert the result on both ends (child 11, 19).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| owner ↔ model | redacted text only | No |
| config ↔ components and plugins | the config transaction, per child | No |
| Ze ↔ kernel | netlink, nft, XFRM, PPP devices, per child | No |
| Ze ↔ peers | PPPoE, L2TP, RADIUS, IKEv2, WireGuard, DHCPv4 in the lab | No |

### Integration Points
- Each child names its own; the lab (child 11) composes them on one converted CPE configuration.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | per child; interface names go through the shared resolver (child 1) |
| No unintended coupling (components stay isolated) | Yes | per child |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing Mechanisms table; reused specs named in the Task |
| Zero-copy preserved where applicable (refs, not copies) | N-A | umbrella; per child |
| Registration over hardcoding, outbound | Yes | per child; new scenarios register with the lab |
| Registration over hardcoding, inbound | Yes | per child |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The capability table covers every construct a home FTTP CPE of this kind uses | survey of a VyOS home-CPE configuration (umbrella Task) | a construct maps to nothing | child 10's conversion record lists every source line as mapped or dropped | confirmed (closure, 2026-11-09): Pre-Commit Verification, Assumptions Resolved |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A lab scenario is red because its child has not landed | red on a known clause | scenarios land with their child, in the order above |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | per child; the umbrella itself changes no code |
| How is it reverted? | each child is its own revert |
| Who else touches this path? | the reused specs named in the Task |

## Wiring Test (MANDATORY -- NOT deferrable)

Each child owns its wiring tests; the rows below name one per child from its spec file.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `pppoe-client pppoe0` config (`spec-home-cpe-1-logical-interface-name.md`) | → | named unit creation | `TestPPPoEClientUsesConfiguredDeviceName` |
| two subnets, reservation in the second (`spec-home-cpe-2-dhcp-shared-interface-subnets.md`) | → | segment resolution | `test/dhcp/dhcpserver-shared-segment-reservation.ci` |
| `system conntrack module ftp` (`spec-home-cpe-3-conntrack-helpers.md`) | → | helper table installed | `test/firewall/conntrack-helper-ftp-assigned.ci` |
| interface address added (`spec-home-cpe-4-ike-dynamic-wan.md`) | → | IKE listener rebind, peer start | `TestInterfaceAddressArrivalStartsPeers` |
| Ed25519 `pki certificate` on an IKE peer (`spec-home-cpe-5-ike-eddsa.md`) | → | EdDSA signer | `TestComputeX509AuthSignsWithEd25519UnderIdentity` |
| two selectors and `vti bind` (`spec-home-cpe-6-ipsec-ipv6-in-ipv4-vti.md`) | → | VTI policy install | `TestDualFamilyProposalNarrowedToIPv6InstallsVTIPolicy` |
| SSH on `0.0.0.0` and `::` (`spec-home-cpe-7-system-small-defects.md`) | → | listener conflict check | `test/parse/listener-dual-wildcard-refused.ci` |
| `pppoe-client ... ipv6` (`spec-home-cpe-8-ipv6-cpe-chain.md`) | → | IPv6CP | `test/parse/pppoe-client-ipv6.ci` |
| clamp-to-PMTU term (`spec-home-cpe-9-mss-clamp-to-pmtu.md`) | → | firewall apply | `test/firewall/firewall-tcp-mss-clamp-to-pmtu.ci` |
| guide worked example (`spec-home-cpe-10-vyos-mapping-guide.md`) | → | `ze config validate` | `test/parse/vyos-guide-example.ci` |
| converted CPE configuration (`spec-home-cpe-11-lab.md`) | → | CPE load and commit verifiers | scenario `conversion` |
| LCP Configure-Request during CHAP (`spec-home-cpe-12-pppoe-client-lcp-renegotiation`, closed) | → | renegotiation | `TestRFC1661ClientRenegotiatesLCPDuringAuth` |
| IPCP Configure-Nak with DNS (`spec-home-cpe-13-pppoe-client-ipcp-dns`, closed; `docs/architecture/l2tp/cpe-1-pppoe-client.md`) | → | resolver lease | `TestPPPoEClientSetsResolverLease` |
| realm relay config and subscriber (`spec-home-cpe-14-lac-realm-relay`) | → | realm match, proxy LCP | `TestRelaySinkRealmMatchPlacesCallWithProxyLCP` |
| `proxy-lcp renegotiate` (`spec-home-cpe-15-lns-lcp-renegotiation`, closed; `docs/guide/l2tp.md`) | → | non-proxy session branch | `TestL2TPProxyLCPRenegotiateReachesDriver` |
| `l2tp { mru 1492 }` (`docs/guide/l2tp.md`, "PPP negotiation") | → | session MRU | `TestReactorPPPPolicyReloadIsolation`, `TestConfiguredMRUNakInformationFloor`; interop `l2tp-lns-mru-xl2tpd` |
| `keepalive interval 2; failures 2` (`spec-home-cpe-17-pppoe-client-keepalive-config`, closed; `docs/guide/pppoe.md`) | → | keepalive loop | `TestPPPoEClientKeepaliveFromConfig`, `TestPPPoEClientKeepaliveConfigEndToEndDetection` |
| `./le test integration interop-topology scenario ze-static-link-chain` (`spec-home-cpe-18-interoplab-multi-link-topology.md`) | → | topology runner | scenario `ze-static-link-chain` |
| `./le test integration interop-topology scenario access-up` (`spec-home-cpe-19-access-chain-scenario.md`) | → | LAC relay to LNS and RADIUS | scenario `access-up` |
| tunnel-mode SA install (`spec-home-cpe-20-xfrm-cross-family-tunnel-state.md`) | → | XFRM state builder | `TestXFRMTunnelStateIsInstalledAFUnspec` |
| `tcp-mss-set 1400` (`spec-home-cpe-21-mss-clamp-not-raise.md`) | → | MSS lowering | `TestTCPMSSSetClampsNeverRaises` |
| IKE_SA_INIT from an established peer (`spec-home-cpe-22-ike-responder-reauth.md`) | → | responder supersede | `TestInitiatePeerAdoptsPeerStartedReauth` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | every child closed | each child's ACs demonstrated in its own spec |
| AC-2 | the lab (child 11) | every scenario of the Lab table green on the converted CPE configuration, each with a recorded red walk |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| per child | children 1 to 23b, each spec's TDD Test Plan | each child's ACs | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| per child | each child's boundary table | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| per child | `test/parse/`, `test/dhcp/`, `test/firewall/`, `test/l2tp/`, `test/policy/` | config acceptance, refusal and behavior per child | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| the Lab table scenarios | `test/interop-topology/scenarios/`, `test/interop-pppoe/scenarios/` (children 11, 19) | FreeRADIUS, strongSwan, wireguard-tools, dhcpcd, accel-ppp | the converted CPE works end to end | |

## Files to Modify
- per child: each child's Files to Modify names the feature code and the pages its change makes wrong
- the doc defects listed in the Task, each with the child that touches the page

## Files to Create
- the child specs listed in the Task table

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | The umbrella ships no code. Each child that adds a config leaf (15 `proxy-lcp`, 16 `mru` and the allow-list, 17 the keepalive leaves, among others) or a command (13 `show interface pppoe-client`) answers this row |
| YANG validation constraints | N-A | Per child, beside each leaf it adds |
| YANG custom validators | N-A | Per child |
| CLI commands/flags | N-A | Per child (13 adds a show node; 14 and 15 add keys to `show l2tp session id`) |
| CLI grammar (keyword before value) | N-A | Per child |
| Editor autocomplete | N-A | Per child |
| Functional test for new RPC/API | N-A | Per child |
| Pipe completeness | N-A | Per child |
| Env var registration | N-A | No child adds an env var through the umbrella |
| Doctor check for runtime dependencies | N-A | Per child (child 3's conntrack helper modules answer it) |
| Prometheus counters/metrics | N-A | Per child (child 12's renegotiation counter) |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP change in the set |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | The umbrella changes no behavior; each child answers for its own `features/` entry |
| 2 | Config syntax changed? | No | Per child |
| 3 | CLI command added/changed? | No | Per child |
| 4 | API/RPC added/changed? | No | Per child |
| 5 | Plugin added/changed? | No | Per child |
| 6 | Has a user guide page? | No | The doc defects found during the survey ("Dependencies, scenario ownership and order", last paragraph) are each owed by the child that touches the page |
| 7 | Wire format changed? | No | Per child |
| 8 | Plugin SDK/protocol changed? | No | No child changes `pkg/` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Per child, in its `rfc/short/` rows |
| 10 | Test infrastructure changed? | No | Child 18 answers it for the topology suite |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | Per child |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | Per child |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Per child |
| 16 | Any changed source file referenced by existing doc source anchors? | No | The umbrella changes no source file; each child derives its own with `./le spec citation anchors` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | Per child; child 10's mapping guide is the umbrella's user-facing page |

## Implementation Steps

The phases follow the executable order in "Dependencies, scenario ownership
and order"; that section is the one order, and each child is wiring-first in
its own spec.

1. **Phase: wrong behaviour on accepted configurations** -- children 12, 16, 1, 2, 21, 7 and 3
2. **Phase: lab harness and PPP features** -- children 18, 17, 13, 14, 15 and 19
3. **Phase: management tunnel** -- children 5, 4, 20 and 6
4. **Phase: remaining** -- children 8, 9, 22, 10, then 11
5. **Phase: DNS sources** -- child 23, then 23b
6. **Phase: umbrella closure** -- the whole-CPE lab rerun once 8, 14, 23 and 23b have landed (Goal Validation)

## Open decisions for the owner

| Decision | Options |
|----------|---------|
| Where the children live | settled: every child is in `plan/release/fixes/`, inside the folders inventory tooling reads |
| Conntrack helpers (child 3) | settled 2026-11-02: Option A, helpers built into Ze's kernel and assigned only when listed. The `lan-services` FTP-through-NAT assertion is therefore unconditional |
| RFC 8420 (child 5) | settled 2026-11-08 (child 5's O-1, closure `209709901c`): RFC 8420 gets its own spec, `plan/post-release/features/spec-ike-rfc8420-eddsa-conformance.md`, which lists it as optional for vpn-concentrator and writes and enrols the summary |
| Delegated prefix use (child 8) | settled 2026-11-07: Thomas ruled `RFC9915-6.3-1`, `-2` and `-3` are requirements Ze owes, so the decline is lifted; child 8 carries them as its RFC gaps and implements them (its "Owner ruling") |
| Learned DNS for LAN clients (child 13) | answered by child 23 and 23b: the `dns-source` list decides which learned sources each consumer uses, the LAN resolver among them (23b); child 13 closed with the LAN resolver forwarding only to configured servers (`docs/architecture/dns/lan-resolver.md`, "Forwarding") |

Edits outside the home-CPE specs that the children owed, as checked on
2026-11-09:

| Edit | State |
|------|-------|
| list `vti-ipv6-inner-ipv4-outer` in `release/validation/spec-ipsec-0-umbrella.md` | done in `ff924ca486` (its "Home-CPE scenarios" section) |
| child 22 pointer in `post-release/features/spec-ike-reauth.md` | done in `ff924ca486`; its IKE SA rekey row, which named a `rekeyIKESA` that does not exist, corrected at closure (Review Gate) |
| narrow `post-release/features/spec-l2tp-proxy-authentication.md` to its LNS half | already done: its Task says it is the LNS half only and names child 14 for the LAC half |
| move the hostname half of the `plan/journal/unwired-feature.md` 2026-09-04 `remote-address` row to child 4 | already done: the row names `spec-home-cpe-4-ike-dynamic-wan (hostname half; IPv6 half: 2026-11-07 row)` |
| journal rows for the client dropping Configure-Request, the pppoe-client name never reaching the kernel, static routes not reinstalled after a reconnect | not owed: each defect is fixed, the first by child 12, the other two by child 1 (its Current Behavior covers reconnects), so no open row is written for them |
| fetch `rfc/full/rfc8420.txt` | already done: the file is in the tree |

## Deliverables Checklist

The umbrella ships no code: its deliverables are its children, the guide and
the lab, each verified by its own closure.

| Deliverable | Verification method | Evidence |
|-------------|---------------------|----------|
| Every child spec closed | `git log --diff-filter=D -- 'plan/*spec-home-cpe-*'` names the closing commit of each child file; `ls plan/release/fixes/spec-home-cpe-*` lists the open ones | 24 of 24 closed (Implementation Summary); 23b last, `f0072e4a2e` |
| The VyOS-to-Ze mapping guide | `ls docs/guide/migrating-from-vyos.md`; child 10's AC table | page present; child 10 closed `3d1d4ecba7` with AC-1 to AC-7 Done |
| The home-CPE lab on the converted configuration | `ls -d test/interop-topology/scenarios/<name>` for the eleven scenarios of the rerun list; child 11's closure | all eleven directories present; child 11 closed `dcc20ffb68`; the rerun after 23b ran green on 2026-11-09 (Goal Validation) |
| The edits outside the home-CPE specs the children owed | the "Edits outside the home-CPE specs" table above | every row done or not owed |

## Security Review Checklist

| Check | What to look for | Result |
|-------|------------------|--------|
| Secrets in the conversion | the model never receives a password, key or PSK; placeholders are never valid in a leaf | child 10's guide (secret path list, placeholder form) and child 11's lab substitution, each reviewed in its own closure |
| Code-level security | injection, bounds, untrusted input, DoS, authorization | per child: the umbrella changes no code, and each child's Security Review Checklist covered its own diff |
| Owner data in the repository | nothing drawn from the owner's configuration or provider | every value in the spec set and the lab is invented (RFC 5737, RFC 3849, ULA, `example.net`) |

## Goal Validation notes

Child 11's Goal Validation ("Goal Validation
(umbrella goals)" in `spec-home-cpe-11-lab`, closed `dcc20ffb68`, readable in
that commit's parent) maps each goal to its lab evidence. That lab ran before
children 8, 14, 23 and 23b landed, so the whole-CPE lab is owed one more run
over the converted configuration once they have: 8, 14, 23 and 23b have closed
(`87c7b998ad`, `4bef6da011`, `62c14b96ca`, `f0072e4a2e`). The command
list is `tmp/session/2026-11-07-104251/scratch/rwinv/umbrella-lab.sh` (the ten
Lab table scenarios plus `mgmt-tunnel-ed25519`, one log each under
`tmp/session/2026-11-07-104251/scratch/rwinv/umbrella-lab/`), run by the main
thread, and its logs are the evidence of the Goal Validation table at
the end of this spec.

The rerun pulls the FreeRADIUS image as it is pinned since `a59d1ee848`
(23b): `interoplab.FreeRADIUSImage` now names the 3.2.10 manifest list
(amd64, arm64, arm/v7) rather than its amd64 manifest, which had made the
container exit 255 with an exec format error on the arm64 docker-lab guest.
The topology suite shares that pin: its `ze-topology-freeradius` image is
built from `interoplab.FreeRADIUSImage`, passed in as `FREERADIUS_IMAGE`
(`test/interop-topology/access-chain/Dockerfile.freeradius`), so the access
scenarios in this rerun are the first topology runs on the new pin.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-2 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, per child: each child's scoped package tests and functional suites, and the lab scenarios through `./le test integration interop-topology scenario <name>`. That scoped evidence is the whole proof: no agent runs any `./le verify` command, and GitHub CI verifies the pushed commits (`ai/META-DECISIONS.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (per child)
- [ ] Functional `.ci` tests for end-to-end behavior (per child)
- [ ] Interop tests for protocol features (the lab, children 11 and 19)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A** and **Commit B** per `ai/rules/planning.md`

## Implementation Summary

### What Was Implemented

The umbrella ships no code. Its work is its children, each closed by its own
spec with its own review gate; the closing commit of each (the commit that
removed the child file, `git log --diff-filter=D -- 'plan/*spec-home-cpe-*'`):

| Child | Delivered | Closing commit |
|-------|-----------|----------------|
| 1 | logical interface names (`pppoe-client` and any non-ethernet name) resolve to the kernel device in static routes, firewall and NAT, and follow a reconnect | `c5794805be` |
| 2 | DHCPv4 subnet selection on a shared interface, reservations first across every subnet | `2bce073236` |
| 3 | conntrack helpers built into Ze's kernel, assigned only when listed (Option A) | `d8584db704` |
| 4 | IKE follows a dynamic WAN address, NAT detection with an FQDN remote, retries while the name does not resolve | `4b3eee5598` |
| 5 | Ed25519 sign and verify in IKEv2 signature authentication | `209709901c` |
| 6 | route-based tunnel carrying IPv6 inside an IPv4 SA, `vti-ipv6-inner-ipv4-outer` against strongSwan | `9dabfc7a2a` |
| 7 | dual-wildcard SSH listener refused at validate, kernel hostname on the appliance | `cede669879` |
| 8 | IPv6 CPE chain: IPv6CP, DHCPv6-PD, /64 and RAs on the LAN | `87c7b998ad` (records retaken in `8d83d8bde0`) |
| 9 | `tcp-mss-clamp-to-pmtu` | `403e3256f9` |
| 10 | the VyOS-to-Ze mapping guide, `docs/guide/migrating-from-vyos.md` | `3d1d4ecba7` |
| 11 | the home-CPE lab on the converted configuration | `dcc20ffb68` |
| 12 | PPPoE client LCP renegotiation in every phase | `9bad2dd6bd` |
| 13 | RFC 1877 DNS in IPCP, a Reject of the DNS options handled | `ac001804c0` |
| 14 | LAC realm relay with proxy LCP and proxy authentication AVPs | `4bef6da011` |
| 15 | LNS `proxy-lcp accept\|renegotiate` | `5ba9503304` |
| 16 | LNS MRU and authentication allow-list; review follow-up 16a to 16h | `5b587ad9d2`; follow-up `dc3514eaec` (16a `b21c1ebcc2`, 16b `7ffaf575d6`, 16c `389f10fb16`, 16d `1875e80c5c`, 16e `c3a1f04941`, 16f `433c56dad8`, 16g `b64e0b88e2`, 16h `095c3f43d2`) |
| 17 | PPPoE client keepalive and reconnect leaves | `1587411097` |
| 18 | interop lab multi-link topology suite | `e063c582f9` |
| 19 | access-chain scenarios (`access-up`, `access-pool`, `access-accounting`) | `bf94d63407` |
| 20 | xfrm tunnel state for IPv6 inside IPv4 (`XFRM_STATE_AF_UNSPEC`) | `12fb5bbfe9` |
| 21 | MSS clamp never raises, proven on Ze's kernel | `0a7cfc0ae1` |
| 22 | an `initiate` peer adopts a responder-started reauthentication | `5d9241eaa6` |
| 23 | `dns-source` part a: resolver registry, selection rule, router consumer, RFC 5625 | `62c14b96ca` |
| 23b | `dns-source` consumers: LAN resolver, DHCPv4, RA, subscriber, RADIUS DNS, `show system dns` | `f0072e4a2e` |

The umbrella's own edits outside its file landed in `ff924ca486`:
`plan/release/validation/spec-ipsec-0-umbrella.md` (the "Home-CPE scenarios"
section, the `internal/plugins/iface/netlink/xfrm_linux.go` path) and
`plan/post-release/features/spec-ike-reauth.md` (the child 22 pointer and its
Existing Mechanisms section, whose rekey row this closure corrected).

### Bugs Found/Fixed
- `plan/post-release/features/spec-ike-reauth.md` named a `rekeyIKESA` function that does not exist, and the umbrella's held-back edit copied it into the Existing Mechanisms row. The IKE SA rekey is `startIKERekey` (`internal/component/ike/engine/established.go`), `initiateIKERekey`, `applyIKERekeyResponse` and `respondIKERekey` (`internal/component/ike/engine/rekey.go`), and old keys are cleared by `forgetKeys` (`internal/component/ike/engine/sa.go`). Every occurrence in that spec corrected (Review Gate).

### Documentation Updates
- None by the umbrella: it changes no behavior. Each child carried its own page edits (the child table above names the pages for 12 to 17), and the four survey doc defects were owed and handled by the child that touched each page (10 for `static-routes.md`, 3 for `configuration.md`, 5 and 6 for `ipsec.md`, 14 and 15 for `l2tp.md`).

### Deviations from Plan

Owner decisions taken in this goal that the children carry:

| Decision | Where it is recorded | What it does |
|----------|----------------------|--------------|
| Child 1, review coverage (owner-approved 2026-11-08) | `Review-owed:` trailer of `c5794805be` | 139 commits carry `Spec-work: home-cpe-1-logical-interface-name` because this goal's session committed IKE, interop and DNS work while child 1 was claimed; child 1's own files have a CLEAN artifact, the rest were reviewed under their own work; the debt is owed at push |
| SPD override, "Override, owe review at push" (2026-11-09) | `Review-owed:` trailer of `52c4757cea` (`spec-ipsec-spd-selection` closure) | six commits under the SPD-selection trailer are other sessions' work, among them `8d13a9dceb`, which is child 23's RA RDNSS work; reviewed under their own specs, the debt owed at push |
| "Override all home-cpe" for misattributed `Spec-work` trailers (2026-11-09) | `Review-owed:` trailer of `62c14b96ca` (child 23) | the closure review coverage is overridden for every home-cpe closure of this goal whose trailer carries other specs' work (`f9ae2b9b88`, `a5adb4ca2c`, `bf2e2d753c` under 23a); each is reviewed under its own spec, the debt owed at push |
| Conntrack helpers, Option A (2026-11-02) | child 3; "Open decisions for the owner" | helpers built into Ze's kernel, assigned only when listed |
| RFC 8420 in its own spec (2026-11-08) | child 5's O-1 | `plan/post-release/features/spec-ike-rfc8420-eddsa-conformance.md` |
| RFC9915-6.3-1 to -3 owed (2026-11-07) | child 8's "Owner ruling" | the decline lifted; child 8 implemented them |
| IPv6 subscriber DNS after this umbrella (2026-11-08) | Work Not Done | its own post-release spec, not blocking the umbrella |

The umbrella's status went from `skeleton` straight to `in-progress` at
closure (through `ready` and the claim): it was never implemented as a unit,
its children were.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | the held-back `spec-ike-reauth` edit (`ff924ca486`) copied that spec's `rekeyIKESA` into a new Existing Mechanisms row without resolving the symbol | no such function; the rekey is `startIKERekey`, `initiateIKERekey`, `applyIKERekeyResponse`, `respondIKERekey` | closure review, `grep -rn 'func .*[Rr]ekey' internal/component/ike/engine/` | every occurrence corrected in this closure |
| approach | the umbrella body kept statements its children had made stale: "four children remain open", child 8 "blocked on one lint log", child 5 "back in design", children 23 and 23b both open, "done in this commit" for edits `ff924ca486` already landed | one child open (23b) at the time; 8, 5 and 23 closed, 23b since (`f0072e4a2e`) | closure review against `git log` | corrected in this closure |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| 1. Ze implements every capability a home CPE of this kind uses, correctly | Done | children 1 to 23b closed (Implementation Summary); the whole-CPE lab rerun green (Goal Validation row 1) | |
| 2. A written VyOS-to-Ze mapping guide | Done | `docs/guide/migrating-from-vyos.md`, child 10 `3d1d4ecba7` | the construct-coverage check is Work Not Done |
| 3. A lab that re-creates the access chain proves the converted router works | Done | child 11 `dcc20ffb68`, `test/interop-topology/scenarios/`; the rerun after 23b, eleven scenarios `exit=0` (Goal Validation row 3) | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | 24 of 24 children closed, each demonstrating its own ACs in its closure commit's parent copy; 23b last, `f0072e4a2e` | |
| AC-2 | Done | child 11's closure: every Lab table scenario green on the converted configuration, each with a recorded red walk (`rwinv/c11-*`); the rerun over today's tree (`umbrella-lab.sh`, le build `74a20ff92c`, 2026-11-09): all eleven scenarios `exit=0`, `interop: 1 passed, 0 failed` each, and the new `access-up` option 6 pass-through check red under its break | logs in `tmp/session/2026-11-07-104251/scratch/rwinv/umbrella-lab/` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| per child unit, boundary and functional tests | Done | each child's closure (Wiring Test rows re-checked in Pre-Commit Verification) | 23b's in its own spec |
| Lab table interop scenarios | Done | `test/interop-topology/scenarios/` | rerun green 2026-11-09, eleven logs `exit=0` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| the child specs (Files to Create) | Done | all written; each removed at its closure, 23b last (`f0072e4a2e`) |
| the survey doc defects (Files to Modify) | Done | handled by the child that touched each page |

### Audit Summary
- **Total items:** 9
- **Done:** 9
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| 1. Ze implements every capability a home CPE of this kind uses, correctly | interop | child 11's committed table ("Goal Validation (umbrella goals)" in `spec-home-cpe-11-lab` at `dcc20ffb68^`): WAN (`access-up`, `access-reconnect`, `access-wholesale`, `access-wholesale-accel-lns`), LAN and firewall (`lan-services`), VPN (`road-warrior`, `mgmt-tunnel`, `mgmt-tunnel-ed25519`), system (`conversion`); IPv6 by child 8's tests and `pppoe-dhcpv6-pd-ze-client`; clamp-to-PMTU by `TestTCPMSSClampToPMTU` (`internal/plugins/firewall/nft/integration_linux_test.go`) and `pppoe-mss-pmtu`. Final evidence: the whole-CPE rerun after 23b, `tmp/session/2026-11-07-104251/scratch/rwinv/umbrella-lab.sh` (le build `74a20ff92c`, 2026-11-09), logs in `tmp/session/2026-11-07-104251/scratch/rwinv/umbrella-lab/`: `access-up`, `access-pool`, `access-accounting`, `access-wholesale`, `access-wholesale-accel-lns`, `access-reconnect`, `mgmt-tunnel`, `mgmt-tunnel-ed25519`, `lan-services`, `road-warrior` and `conversion` each `exit=0`, `interop: 1 passed, 0 failed`. The new `access-up` option 6 pass-through check (`internal/le/interoplab/topology/check_access_option6.go`, 23b story 1b) passes clean and goes red under 23b's pass-through break (`access-up-passthrough-broken.log`, `exit=1`: "lan-res-c was never handed ze-cpe's learned resolvers in option 6: the lease carried no option 6, want [192.0.2.53 192.0.2.54]") |
| 2. A written VyOS-to-Ze mapping guide makes the conversion reproducible and reviewable | functional + interop | child 10's guide (`3d1d4ecba7`) with `test/parse/vyos-guide-example.ci`; child 11's `conversion` scenario (the record covers every line of the synthetic source, and the guide's output is the configuration every home-CPE scenario runs). Final evidence: `conversion.log` of the rerun, `exit=0`, `interop: 1 passed, 0 failed` |
| 3. A lab that re-creates the access chain proves the converted router works | interop | child 11: every home-CPE lab scenario on the converted configuration, each with a recorded red walk (`rwinv/c11-*` breaks). Final evidence: all eleven logs of the rerun end `exit=0` (summary table at the end of `tmp/session/2026-11-07-104251/scratch/rwinv/umbrella-lab.log`), and the one break the rerun carries went red at its own assertion |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| A live reorder of an interface's addresses moves its primary | found by child 11's lab; a separate reconcile change | `plan/release/fixes/spec-iface-address-reorder-moves-primary.md` |
| The synthetic VyOS source covers every construct the guide maps or drops, with a check (child 11 A-2 broken: address and port groups, `ip-src-route`, `clamp-mss-to-pmtu`, `ipv6 adjust-mss`, router advertisements, NTP, the commit archive and three dropped rows are absent) | found at child 11's closure | `plan/release/validation/spec-home-cpe-lab-guide-construct-coverage.md` |
| An IPv6 subscriber of the LNS or PPPoE server is given no resolver (no RDNSS, no DHCPv6 option 23) | owner decision 2026-11-08: a spec of its own after this umbrella, which it does not block | `plan/post-release/features/spec-l2tp-ipv6-subscriber-dns.md` |
| Ze accepts no IPv6 address request from an L2TP or PPPoE subscriber | previously owner-approved post-release feature | `plan/post-release/features/spec-l2tp-ipv6-subscriber.md` |
| The problem classes the home-CPE lab found that `ze doctor` does not report | owner request: one doctor check per class | `plan/release/validation/spec-doctor-coverage-2-home-cpe-batch.md` |
| RFC 8420 summary and enrolment | child 5's O-1, owner decision 2026-11-08 | `plan/post-release/features/spec-ike-rfc8420-eddsa-conformance.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | recorded by `./le spec review record spec plan/release/fixes/spec-home-cpe-0-umbrella.md` over the three files below; the path is in the commit body |
| `./le spec review check` | re-recorded 2026-11-09 after the 23b closure and the lab rerun, over this spec and the six citers repointed to the bare stem; clean |
| Rounds | 3 |
| Reviewer lenses used | spec claims against source (every symbol, path and commit the umbrella and its two held-back edits name, resolved by grep and `git log`), stale statements, owner-decision record |

Files reviewed: `plan/release/fixes/spec-home-cpe-0-umbrella.md`,
`plan/release/validation/spec-ipsec-0-umbrella.md` (its `ff924ca486` hunks:
the scenario directory and the `CreateXFRM` path, package `ifacenetlink`,
both resolve), `plan/post-release/features/spec-ike-reauth.md`.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Existing Mechanisms row names `rekeyIKESA`, which does not exist; the same name in Recovery, Current Behavior, Behavior to preserve and Security Review | `plan/post-release/features/spec-ike-reauth.md` | named `startIKERekey`, `initiateIKERekey`, `applyIKERekeyResponse`, `respondIKERekey`, `forgetKeys`; the quoted established.go comment replaced by the one in the source |

Round 1 NOTEs (record defects, fixed in one edit): the stale umbrella
statements in the Mistake Log; the Work Not Done row that understated child
11's A-2 gap; the missing Deliverables and Security Review Checklists, added.
Round 2: 0 BLOCKER, 0 ISSUE.

Round 3 (2026-11-09, after 5c324f96db): over the 23b and lab-rerun edits.
Every claim checked at its source: `f0072e4a2e` deletes the 23b spec; the
rerun's summary table (`umbrella-lab.log`) shows eleven scenarios exit 0 and
the pass-through break exit 1 at "the lease carried no option 6"; the 33
deletions of the AC-1 log command. One NOTE fixed in the same edit: the AC-1
row said the log lists 24 commits, it lists 33 (the nine child 16 follow-ups
beside the 24 children). Citers repointed to the bare stem:
`plan/handover/2026-11-07-home-cpe-implementation.md`,
`plan/post-release/features/spec-l2tp-ipv6-subscriber-dns.md`,
`plan/post-release/features/spec-oauth-login-0-umbrella.md`,
`plan/post-release/features/spec-vpn-concentrator-0-umbrella.md` (its AC-4,
which the spec hook refused as an absence-only criterion, restated with the
same meaning in positive form),
`plan/release/validation/spec-home-cpe-lab-guide-construct-coverage.md`,
`plan/release/validation/spec-ipsec-0-umbrella.md`. 0 BLOCKER, 0 ISSUE.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/dhcp/dhcpserver-shared-segment-reservation.ci`, `test/firewall/conntrack-helper-ftp-assigned.ci`, `test/parse/listener-dual-wildcard-refused.ci`, `test/parse/pppoe-client-ipv6.ci`, `test/firewall/firewall-tcp-mss-clamp-to-pmtu.ci`, `test/parse/vyos-guide-example.ci` | yes | `ls`, 2026-11-09 |
| the eleven rerun scenario directories under `test/interop-topology/scenarios/`, plus `ze-static-link-chain`, `pppoe-dhcpv6-pd-ze-client`, `pppoe-mss-pmtu` and `test/interop-l2tp/scenarios/l2tp-lns-mru-xl2tpd` | yes | `ls -d`, 2026-11-09 |
| `docs/guide/migrating-from-vyos.md` | yes | `ls`, 2026-11-09 |
| every Work Not Done destination spec | yes | `git ls-files --error-unmatch`, 2026-11-09 (status skeleton or design) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | 24 of 24 children closed | `git log --diff-filter=D --since=2026-10-15 -- 'plan/*spec-home-cpe-*'` lists 33 deletions: the 24 child closing commits of the Implementation Summary, 23b's `f0072e4a2e` last, and nine child 16 follow-up specs (16 review fixes, 16a to 16h); `ls plan/release/fixes/spec-home-cpe-*` lists the umbrella only |
| AC-2 | the lab ran green with red walks at child 11's closure, and again after 23b | `spec-home-cpe-11-lab` at `dcc20ffb68^`, "Goal Validation (umbrella goals)"; the rerun's summary table at the end of `tmp/session/2026-11-07-104251/scratch/rwinv/umbrella-lab.log`: eleven rows exit `0`, `access-up-passthrough-broken` exit `1` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| every Wiring Test row | the six `.ci` files above, the scenarios above | each test function resolves: `grep -rl 'func <Test>('` finds all fifteen named Go tests (for example `TestPPPoEClientUsesConfiguredDeviceName` in `internal/component/iface/pppoe_client_test.go`, `TestXFRMTunnelStateIsInstalledAFUnspec` in `internal/component/ike/dataplane/xfrm_integration_linux_test.go`); what each exercises was judged in its child's closure |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | child 10's guide maps or drops every construct of the survey, and child 11's conversion record lists every line of the synthetic source as mapped or dropped (`dcc20ffb68^`, Goal Validation row 2). The synthetic source covers part of the guide's inventory (child 11 A-2 broken), homed in `plan/release/validation/spec-home-cpe-lab-guide-construct-coverage.md` |

### Remaining before commit A
| Item | Owner |
|------|-------|
| child 23b's closure | done: `spec-home-cpe-23b-dns-source-consumers` closed `f0072e4a2e` |
| the whole-CPE lab rerun, eleven logs ending `exit=0` | done 2026-11-09: eleven logs `exit=0`; Goal Validation filled from them |
| the access-up option 6 pass-through lab run 23b handed this umbrella (23b's Work Not Done row) | done in the same rerun: `access-up` green, and red under the pass-through break (`access-up-passthrough-broken.log`, `exit=1`) |

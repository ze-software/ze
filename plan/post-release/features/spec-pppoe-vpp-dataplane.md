# Spec: pppoe-vpp-dataplane -- terminate PPPoE subscriber sessions in the VPP data plane

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P2 |
| Release reason | Owner decision: Ze terminates PPPoE subscriber sessions in VPP, after the first release (OD-5, decided 2026-10-28). The shipped-behaviour fix that was Phase 1 (validate accepts PPPoE under the VPP backend and every session then fails) moved to `plan/release/fixes/spec-pppoe-refuse-vpp-backend.md` |
| RFC gaps | - |
| Depends | - |
| Phase | 0/7 |
| Handoff | - |
| Updated | 2026-10-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Doors: `ai/doors/system.md`, `ai/doors/forwarding.md` (vpp, iface, traffic), `ai/doors/access-security.md` (l2tp, PPPoE, PPP).

## Task

Ze's PPPoE access concentrator terminates every subscriber session in the Linux
kernel: an AF_PPPOX socket per session, a `/dev/ppp` channel and unit, and a
`pppN` interface that netlink addresses and tc shapes. The owner decided that Ze
will also terminate PPPoE subscriber sessions in the VPP data plane, so that
subscriber traffic is encapsulated, decapsulated, routed and policed by VPP at
DPDK speed while Ze keeps the whole control plane (discovery, LCP, auth, NCP,
DHCPv6, RA, RADIUS, accounting).

The goal has five parts:

| # | Goal |
|---|------|
| G-1 | A PPPoE client reaching an access interface owned by VPP completes discovery, LCP, authentication and IPCP/IPv6CP against Ze, and its IP traffic is forwarded by VPP, never by the Linux kernel |
| G-2 | The kernel and VPP session data planes are two registered backends behind one seam. The configuration selects exactly one; neither falls back to the other (`ai/rules/no-layering.md`) |
| G-3 | Every per-session data-plane operation Ze performs today on `pppN` (MTU, admin state, peer address, framed and delegated-prefix routes, VRF, rate limit, teardown) has a VPP equivalent or is refused at verify |
| G-4 | Subscribers get IPv6 (RA, DHCPv6 IA_NA and IA_PD) on the VPP backend without kernel sockets bound to `pppN` |
| G-5 | A VPP restart or a lost GoVPP connection leaves no stale session state in Ze or in VPP, and service resumes without operator action |

### Reference design: accel-ppp-ng (VyOS)

`/home/thomas/Code/github.com/accel-ppp/accel-ppp-ng` already does this for VyOS.
Read at the source, not from its README:

| Seam | What ng does | Proof |
|------|--------------|-------|
| Hook table | A session-hooks interface registered by name; the PPPoE server looks up the `"vpp"` hooks when an interface is configured `vpp-cp=true` | `accel-pppd/include/ap_session_hooks.h`, `ap_session_hooks.c`; `ctrl/pppoe/pppoe.c` (`parse_server`, `ap_session_hooks_find("vpp")`); `session_hooks/vyos_vpp/vpphooks.c` (`vpp_hooks`, `DEFINE_INIT`) |
| Control-plane interface | PPPoE runs on the Linux side of the access interface's LCP TAP. VyOS maps the VPP access interface (dp) to its LCP TAP (cp) with `pppoe_add_del_cp(dp_sw_if_index, cp_sw_if_index)` | `vyos-1x python/vyos/vpp/control_vpp.py` (`map_pppoe_interface` reads the LCP pair: `vpp_index_hw` is dp, `vpp_index_kernel` is cp); `src/conf_mode/service_pppoe-server.py` (`apply`) |
| Session without `/dev/ppp` | The kernel AF_PPPOX socket is connected on the cp TAP but never attached to a PPP channel or unit; it is read and written directly as the PPP frame channel ("non-dev-ppp"). No `pppN` is created | `accel-pppd/ppp/ppp.c` (`establish_ppp`: `chan_fd = fd` when `is_non_dev_ppp`; `connect_ppp_channel`: `unit_fd = fd`, `ifindex = -1`); `ctrl/pppoe/pppoe.c` (`connect_channel`) |
| Session interface | `pppoe_add_del_session(client_mac, session_id, disable_fib=1)` creates the VPP `pppoe_sessionN` interface; ng then disables the `ip4-not-enabled`/`ip6-not-enabled` features on it and names the session after the VPP interface | `session_hooks/vyos_vpp/vpppoe.c` (`vpppoe_sync_add_pppoe_interface`, `vpppoe_set_feature`); `vpphooks.c` (`vpphook_pppoe_create_vpp_session_interface`) |
| Addresses and routes | Every address operation becomes a VPP route to the session interface: peer /32, framed routes, IPv6 /128 and delegated prefixes, all with `ip_route_add_del` | `vpphooks.c` (`vpphook_ipaddr_add_peer`, `vpphook_iproute_add`, `vpphook_ip6route_add`); `vppiputils.c` |
| Rate limit | Two VPP policers per session (`policer_add_del`), bound with `policer_input` and `policer_output`, the input one enabled as a feature on the `ip4-unicast`, `ip6-unicast`, `ip4-multicast`, `ip6-multicast` arcs | `session_hooks/vyos_vpp/vpppolicer.c` (`vpppolicer_install_limiter`) |
| IPv6 control | VPP punts RS, NS and DHCPv6 inside PPP frames to the cp TAP; ng parses IPv6 out of the PPP channel and answers in PPP frames ("non-socket DHCPv6/ND") | `accel-pppd/ipv6/ppp_ipv6layer.c`, `ipv6/dhcpv6.c`, `ipv6/nd.c` (`is_non_socket_dhcpv6_nd`); commit `07f158d` |
| Teardown | Flush routes, remove policers, delete the VPP session | `vpphooks.c` (`vpphook_pppoe_terminate`) |
| VPP loss | On a lost VPP connection the VPP PPPoE services stop; they restart when netlink reports the cp TAP again | commit `90c6d35` (`libnetlink/new_link_event.c`, `ctrl/pppoe/pppoe.c` `pppoe_on_add_link`) |

ng drives the **upstream VPP `pppoe` plugin through VAPI**, not a custom plugin,
but the API it calls is the **VyOS-patched** one: upstream VPP 25.10 has no
`disable_fib` field and no per-interface cp map.

### VPP patches ng requires

VyOS builds VPP `stable/2510` with every patch in `vyos/vyos-vpp-patches`
(`vyos-build scripts/package-build/vpp/package.toml`). The PPPoE path depends on:

| Patch | What it changes | Why ng needs it |
|-------|-----------------|-----------------|
| `0013-pppoe.-Automated-session-management.-12.patch` | `pppoe_add_del_cp` takes `dp_sw_if_index` + `cp_sw_if_index` (a map per access interface instead of one global cp); `pppoe_add_del_session` gains `disable_fib`; LCP, IPCP and PADT sniffing for optional auto sessions | Several access interfaces; a session created before IPCP assigns an address |
| `0014-pppoe-Added-option-enable-pass-nd-and-dhcpv6.patch` | startup.conf `pppoe { enable-pass-nd-and-dhcpv6 }` punts ICMPv6 RS/NS and UDP DHCPv6 inside the session to the cp interface | IPv6 RA/DHCPv6 served from the PPP channel |
| `0018-policer-Added-features-for-unicast-multicast-arc.-29.patch` | Registers `policer-input` on the `ip4/ip6-unicast` and `ip4/ip6-multicast` arcs | Upstream `policer-input` exists only on `device-input`, which a PPPoE session interface never traverses |
| `0025-T8231-Removed-PPPoE-input-feature-setup-41.patch` | Stops enabling `pppoe-input` as a `device-input` feature on the cp interface | Non-PPPoE traffic was being steered into PPPoE nodes |
| `0031-PPPoE-Added-cp_binding_dump-API-call-49.patch` | `pppoe_cp_binding_dump` | Reconciling the dp/cp map after a restart or reload |

`0024-T7860-IPv6-ICMP-RA-punt-shortcut-36.patch` touches `pppoe_cp.c` but serves
DHCPv6 on LCP TAPs, not subscribers.

### What upstream VPP 25.10 does without patches

Read in the upstream `v25.10` source (`src/plugins/pppoe/`, `src/vnet/policer/node_funcs.c`,
`src/vnet/classify/policer_classify.c`, fetched for this spec):

| Fact | Where | Consequence for Ze |
|------|-------|--------------------|
| Non-IP PPP protocols (LCP, PAP, CHAP, IPCP, IPv6CP) and discovery go to the CP node before any session lookup | `pppoe_decap.c` (`PPPOE_INPUT_NEXT_CP_INPUT`) | Control frames reach Ze before the VPP session exists |
| One global `cp_if_index`, set by `pppoe_add_del_cp(sw_if_index)` | `pppoe_cp.c`, `pppoe.api` | All access interfaces share one cp TAP; Ze cannot tell the ingress access interface |
| The CP node learns `client MAC -> ingress interface` in `link_table`, keyed by MAC alone; frames Ze sends out the cp are switched by destination MAC and get the access interface's MAC as source | `pppoe_cp_node.c` | The same client MAC on two access interfaces collides |
| `pppoe_add_del_session` takes `client_ip` and always installs `client_ip/32` (or `/128`) to the session in `decap_vrf_id`; it fails with `INVALID_SW_IF_INDEX` until `link_table` knows the MAC | `pppoe.c` (`pppoe_add_del_session`, `fib_table_entry_path_add`) | The session can be created only once an address is known; an IPv6-only session has no unique address at that time |
| IPv6 inside the session is decapsulated to `ip6-input` on the session interface; nothing reaches the cp | `pppoe_decap.c` | RS and DHCPv6 need VPP's own RA (`ip6_nd` API) and a punt socket for UDP 547, or patch 0014 |
| `policer-input` is a `device-input` feature; `policer-output` is on `ip4-output` and `ip6-output`; `ip4/ip6-policer-classify` run on `ip4/ip6-unicast` | `node_funcs.c`, `policer_classify.c` | Download policing works upstream; upload policing needs a classify table per session, or patch 0018 |

### What Ze vendors

`go.fd.io/govpp v0.13.0` (`go.mod`) ships `binapi/pppoe` generated from VPP
`25.10-release`, in the upstream shape (no `DisableFib`, single `SwIfIndex` cp).
It is not in `vendor/` because nothing imports it; neither are `punt`, `ip6_nd`
or `feature`. A patched VPP changes the message CRCs, so the govpp module's
binapi would no longer match and Ze would generate its own `pppoe` binapi from
the patched `.api`.

## Required Reading

### Architecture Docs
- [ ] `ai/digests/subscriber.md` - PPPoE discovery to PPP handoff, shared PPP driver, subscriber registry
  → Constraint: PADS is sent only after the session data plane exists (`handlePADR`); the VPP backend keeps that order
  → Constraint: one reader per channel (`pppSession.run` starts exactly one `readFrames`)
- [ ] `ai/digests/vpp-dataplane.md` - VPP supervision, connector, readiness events, iface-vpp and fib-vpp
  → Constraint: readiness is only the `vpp/connected` and `vpp/reconnected` EventBus events (`internal/core/vpp/events/events.go`); `vpp/disconnected` exists
  → Constraint: `startup.conf` is written once; a plugin load or section change needs a VPP restart
- [ ] `docs/guide/vpp.md` - operator view, LCP TAP namespace limitation
  → Constraint: LCP TAPs land in `vpp.lcp.netns` (default `dataplane`); Ze's discovery socket must open in the namespace the cp TAP lives in
- [ ] `docs/architecture/core-design.md` sections 14, 14a, 14b, 19 - backend split, `ze:backend` commit gate, allowed imports
  → Decision: a backend that cannot implement config exactly refuses it at verify
- [ ] `docs/architecture/traffic/fw-7-traffic-vpp.md` - "Exact or reject"
  → Constraint: a VPP policer drops; a tc TBF/HTB shapes. Mapping one to the other silently is refused
- [ ] `plan/post-release/engineering/spec-dataplane-seams-4-control-packet-rx.md` - shared control-packet receive path (decision pending)
  → Constraint: this spec does not build a shared receive path; it adds one PPPoE-local receive path over the cp TAP
- [ ] `plan/post-release/engineering/spec-dataplane-seams-3-route-egress-interface.md`
  → Constraint: `BestChangeEntry` carries no egress interface, so a route "to session interface" cannot travel through sysrib to fib-vpp today
- [ ] `docs/architecture/l2tp/bng-5-pppoe.md` - PPPoE AC design the changed files declare
  → Constraint: its kernel-session sections gain the backend split and the VPP path in Phase 7
- [ ] `docs/research/l2tpv2-ze-integration.md` - PPP driver and transport boundary the PPP files declare
  → Constraint: the transport/PPP contract it describes (fds in `StartSession`) is replaced in Phase 2, and the page is edited in that phase
- [ ] `docs/research/vpp-deployment-reference.md` - startup.conf reference the VPP component declares
  → Constraint: the `plugins` section and any `pppoe` startup section are documented there
- [ ] `docs/labs/pppoe-interop.md` - PPPoE Docker lab, pppd + rp-pppoe client image
- [ ] `third_party/vpp-linux-cp/README.md` - LCP pair semantics, v25.10

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2516.md` - PPPoE; the session-stage obligations that move into VPP (table below)
- [ ] `rfc/short/rfc1661.md` - LCP stays in Ze; MRU vs VPP session MTU
- [ ] `rfc/short/rfc1332.md`, `rfc/short/rfc5072.md` - IPCP and IPv6CP stay in Ze
- [ ] `rfc/short/rfc4861.md`, `rfc/short/rfc4862.md` - RA/RS for subscribers
- [ ] `rfc/short/rfc4638.md` - PPP-Max-Payload; the VPP session MTU must follow it (`plan/release/fixes/spec-pppoe-rfc4638-max-payload.md`)
- [ ] `rfc/short/rfc2866.md`, `rfc/short/rfc5176.md` - accounting octets and CoA rate change must read VPP counters and re-program VPP policers

#### RFC 2516 obligations whose performer changes

| Id | Requirement (short) | Kernel backend performer | VPP backend performer | Test owed (asserts what Ze installs) |
|----|---------------------|--------------------------|----------------------|--------------------------------------|
| RFC2516-3-1 | AC allocates a PPP virtual interface once the session is established | `ppp.DevPPPSetup` creates `pppN` | `pppoe_add_del_session` creates `pppoe_sessionN` | `pppoe_session_dump` after PADS shows the session; absent before PADI |
| RFC2516-6-1 | Session-stage CODE is 0x00 | Linux `pppoe` module encap | VPP `pppoe` encap (`pppoe.c`, `pppoe_fixup`) | Captured downstream data frame from VPP carries CODE 0 |
| RFC2516-7-1 | SESSION_ID does not change and is the discovery value | AF_PPPOX connect address | `session_id` in `pppoe_add_del_session` | Session dump SID equals the PADS SID |
| RFC2516-x-6 | Session traffic DESTINATION_ADDR is the peer's unicast MAC | AF_PPPOX remote | `client_mac` in the session | Captured frame destination equals PADR source |
| RFC2516-x-7 | SOURCE_ADDR is the sender's MAC | kernel device MAC | encap interface hardware address (`link_table`) | Captured frame source equals the access interface MAC |
| RFC2516-x-1, x-2 | VER and TYPE are 1 | kernel encap | VPP encap | Same capture |
| RFC2516-5.5-4 | No PPP frames after PADT | Ze closes the PPPoX fd | Ze deletes the VPP session before sending PADT; control path stops in Ze | After PADT, VPP session absent and no session frame leaves for that SID |
| RFC2516-7-3 | Stop using the session when LCP terminates | Ze closes fds | Ze deletes the VPP session | Same as above, LCP-initiated |
| RFC2516-x-9 | MRU not negotiated above 1492 | Ze LCP | Ze LCP; VPP session MTU set to the negotiated MRU | Session interface MTU readback |

Each row keeps its `lower-layer:` annotation and names both performers, one per backend (`ai/rules/rfc-compliance.md`, whole-stack directive).

**Key insights:**
- ng does not bypass the kernel for control: control frames travel VPP -> LCP TAP -> Linux -> Ze. Only data never touches Linux.
- The upstream VPP pppoe plugin is usable for one access interface and dual-stack sessions; multi-interface, IPv6-only, upload policing and IPv6 control need either the VyOS patches or the upstream workarounds named above.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/pppoe/server.go` - `handlePADR`: cookie check, SID alloc, `pppoeCreate` then `ppp.DevPPPSetup`, then PADS, then `ppp.StartSession` on `SessionsIn()`
- [ ] `internal/component/l2tp/pppoe/subsystem.go` - `Start`: one raw discovery socket for all interfaces, servers keyed by kernel ifindex; PPP driver built with `iface.GetBackend()` as its `ppp.IfaceBackend`; `onSessionUp` calls `subscriber.GetShaperHandler()` with `sess.PppInterface`
- [ ] `internal/component/l2tp/pppoe/kernel_linux.go` - discovery socket, `pppoeCreate` (AF_PPPOX)
- [ ] `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` - `pppoe` container: globals, `list interface` keyed by kernel interface name; no `ze:backend`
- [ ] `internal/component/l2tp/ppp/start_session.go` - `StartSession` carries `ChanFD`, `UnitFD`, `UnitNum` (iface name is `"ppp"+UnitNum`)
- [ ] `internal/component/l2tp/ppp/session.go` - `IfaceBackend`: `SetMTU`, `SetAdminUp`, `AddAddressP2P`, `AddRoute`, `RemoveAddress`, `RemoveRoute`
- [ ] `internal/component/l2tp/ppp/session_run.go` - `PPPIOCCONNECT` of chan to unit, `pppN` MRU/MTU/admin-up
- [ ] `internal/component/l2tp/ppp/devppp_linux.go` - `DevPPPSetup` ioctl sequence
- [ ] `internal/component/l2tp/ppp/ra_linux.go`, `dhcpv6_linux.go` - RA and DHCPv6 sockets bound with `SO_BINDTODEVICE` to `pppN`
- [ ] `internal/component/l2tp/subscriber/handler_registry.go` - `ShaperHandler(iface, down, up)` singleton
- [ ] `internal/component/l2tp/plugins/shaper/shaper.go`, `config.go` - shaping through `traffic.GetBackend()`, qdisc TBF or HTB only
- [ ] `internal/plugins/iface/vpp/ifacevpp.go` - `AddAddressP2P` and `AddRoute` answer `errNotSupported`
- [ ] `internal/plugins/iface/vpp/lcp.go` - `SetupLCPPair`, `RemoveLCPPair`
- [ ] `internal/component/vpp/startupconf.go` - `plugin default disable`; dpdk, urpf, linux_cp, linux_nl, wireguard enabled explicitly
- [ ] `internal/core/vpp/events/events.go` - `connected`, `disconnected`, `reconnected`
- [ ] `internal/le/test/deployment/vppiface.go` - real-VPP harness image `ligato/vpp-base:26.06-release`

**Behavior to preserve:**
- The kernel backend's behaviour, wire output, `show pppoe` answers and every existing PPPoE `.ci` and interop scenario, unchanged.
- L2TP keeps the kernel data plane; it consumes the same PPP package and must not regress.
- PADS only after the session data plane exists.

**Behavior to change:**
- PPPoE under `interface backend vpp` validates and then fails every session at IPCP Opened (journal row below). It becomes either a working VPP session data plane or a verify-time refusal.
- `ppp.StartSession` stops carrying kernel file descriptors as its data-plane contract (replaced, not layered: see Key Design Decisions).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- PPPoE discovery and session frames from a subscriber arrive on a VPP-owned access interface (DPDK or af_packet).
- Config: `interface { backend vpp; }`, `vpp { lcp ...; plugins ... }`, `pppoe { interface <name> ...; }`.

### Transformation Path
1. VPP `pppoe-input` (registered for both PPPoE ethertypes) sends discovery and non-IP session frames to `pppoe-cp-dispatch`, which learns the client MAC and transmits on the cp interface (the access interface's LCP TAP).
2. Ze's PPPoE subsystem reads discovery and session control frames on the Linux side of that TAP, in the TAP's namespace.
3. On PADR, the VPP session backend creates the VPP session interface (`pppoe_add_del_session`), then Ze sends PADS and starts the PPP session with a frame channel that is not `/dev/ppp`.
4. LCP, auth and NCP run in Ze exactly as today. On NCP Opened, the session's data-plane handle installs the peer route(s) to the session interface in the subscriber's VRF, enables IP on it, sets MTU, and applies the rate.
5. Subscriber IP traffic is decapsulated by VPP into the session interface and routed by the VPP FIB; downstream traffic routed to the session interface is encapsulated with the session ID and client MAC.
6. Teardown (PADT either way, LCP terminate, CoA Disconnect, operator clear) removes routes, policers and the VPP session before PADT is sent.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| VPP -> Linux (control frames) | LCP TAP, cp map | No (Phase 0 lab) |
| Ze -> VPP (session, routes, policers) | GoVPP binary API through `vppcomp.GetActiveConnector()` | No |
| VPP lifecycle -> PPPoE | EventBus `vpp/connected`, `vpp/disconnected`, `vpp/reconnected` | Events exist; consumer No |
| PPPoE -> subscriber plugins | unchanged: `subscriber` handler registries and `subscriber` event namespace | Yes (digest) |

### Integration Points
- `pppoe.Subsystem` selects the registered session data plane by the configured interface backend name.
- `ppp.Driver` receives a per-session data-plane handle through `StartSession` instead of fds.
- `iface` VPP backend creates the LCP pair for each PPPoE access interface (existing `SetupLCPPair`).
- `vpp` component loads `pppoe_plugin.so` and, if patched, writes the `pppoe` startup section.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes (design) | Control plane stays in Ze; only the per-session data-plane calls change |
| No unintended coupling | Yes (design) | VPP backend lives in its own `ze_vpp`+`ze_l2tp` gated package and registers; `pppoe` never imports `vpp` |
| No duplicated functionality | Yes (design) | Reuses PPP driver, discovery codec, subscriber registries, GoVPP connector |
| Zero-copy preserved | Yes (design) | Control frames only; data never reaches Ze |
| Registration over hardcoding, outbound | Yes (design) | Session data planes register by name from `init()` |
| Registration over hardcoding, inbound | No, to verify in Phase 1 | Lists to search: `startupconf.go` plugin section (wireguard is a toggle there today), `feature-gates.txt` dependent-gate notes, doctor codes |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | A Linux AF_PPPOX socket connected on the cp TAP but never attached to a PPP channel delivers and sends PPP frames as a datagram socket | ng `establish_ppp` non-dev-ppp path; Linux `pppoe_rcv_core` queues to the socket when not bound | Use an AF_PACKET 0x8864 socket on the TAP with Ze-side demux by (MAC, SID) | Phase 0 lab capture | unvalidated |
| A-2 | The ligato `vpp-base` image ships `pppoe_plugin.so` | upstream builds every core plugin | Lab builds VPP itself (same pipeline as OD-1 option B) | `vppctl show plugins` in Phase 0 | unvalidated |
| A-3 | VLAN-tagged access (subinterfaces) reaches the cp TAP with the tag intact | `pppoe_cp_node.c` resets the buffer to the Ethernet header | VLAN access needs an LCP pair per subinterface and per-subinterface cp map | Phase 0 capture | unvalidated |
| A-4 | VPP's pppoe encap honours the session interface MTU | upstream `pppoe.c` | MRU/MTU enforcement must be a VPP feature or a refusal | Phase 0 oversize ping | unvalidated |
| A-5 | Discovery and session sockets opened in the LCP netns work while Ze runs in another namespace | `vpp.lcp.netns` default `dataplane`; Ze has netns helpers for other features | Restrict to empty netns and say so in docs | Phase 0 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Control-frame path through LCP TAP is slow under PADI storm | PADI rate test drops LCP | Existing PADI limiter; VPP-side policer on cp (copp work, seams-4) |
| R-2 | Patched VPP CRCs diverge from govpp binapi | `CheckCompatiblity` fails at connect | Ze-generated binapi from the same patched `.api` (OD-1 B) |
| R-3 | VPP restart loses every session silently | Sessions in Ze, none in `pppoe_session_dump` | Reconcile on `vpp/reconnected` (OD-2) |
| R-4 | Policer semantics differ from tc shaping | Operator configured TBF on VPP | Refuse at verify; separate `policer` rate mode |
| R-5 | Scale: one VPP interface per session | VPP interface table growth, stats segment size | Measure on Thomas's Mac (perf rule); size limits documented |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Kernel-backend PPPoE and L2TP subscribers if the PPP channel seam regresses; VPP subscribers lose service |
| How is it reverted? | Per-phase commits; Phase 2 (PPP channel seam) is the only one touching L2TP |
| Who else touches this path? | `spec-pppoe-rfc4638-max-payload`, `spec-pppoe-subscribers-produce-no-accounting-or-telemetry`, `spec-pppoe-relay-agent`, `spec-pppoe-virtual-interface-proof`, `spec-bsd-l2tp-pppoe` (a third session data plane: netgraph/pipex, must fit the same seam), `spec-dataplane-seams-4-control-packet-rx`, `spec-l2tp-ipv6-subscriber` |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config `interface backend vpp` + `pppoe interface X` | → | PPPoE verify selects the `vpp` session data plane or refuses | `test/parse/pppoe-vpp-dataplane-refused-without-backend.ci`, `test/parse/pppoe-vpp-dataplane-accepted.ci` |
| PADR on a VPP access interface | → | VPP session backend `CreateSession` -> `pppoe_add_del_session` | `TestPPPoEVPPSessionCreatedBeforePADS` (fake GoVPP channel) and `test/vpp/pppoe-vpp-session.ci` |
| IPCP Opened | → | session handle installs peer /32 route in VPP | `TestPPPoEVPPPeerRouteOnIPCPOpened`, interop `pppoe-vpp-ze-ac-pppd-client` |
| `vpp/disconnected` then `vpp/reconnected` | → | session reconciliation per OD-2 | `TestPPPoEVPPReconcileOnReconnect`, interop `pppoe-vpp-restart` |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `interface backend vpp`, a `pppoe` block, and no VPP session data plane in the build (`ze_vpp` absent) | `ze config validate` refuses, naming the `pppoe` path, the backend and the build tag that provides it |
| AC-2 | Same tree on a build with the VPP session data plane, `vpp lcp` disabled | Validate refuses: the PPPoE access interface needs an LCP pair, with the leaf that enables it |
| AC-3 | pppd client sends PADI on a VPP access interface | PADO comes back with Ze's AC-Name; the PADO source MAC is the access interface MAC |
| AC-4 | Client sends PADR | A VPP session with the PADS SESSION_ID and the client MAC exists before PADS leaves; no `pppN` exists on the host |
| AC-5 | LCP, CHAP, IPCP complete | The peer address is reachable from a host behind VPP; VPP's session interface counters move; the host kernel sees no 0x8864 data frame for that session |
| AC-6 | Session in a configured VRF | Peer route sits in that VRF's VPP table only |
| AC-7 | RADIUS Framed-Route or DHCPv6-PD prefix for the session | Route to the session interface installed in VPP; removed on teardown |
| AC-8 | IPv6CP opens | Client receives an RA with the configured prefix and obtains IA_NA/IA_PD by DHCPv6, all without a kernel socket bound to a `pppN` |
| AC-9 | Rate attributes (RADIUS Filter-Id rate or config) on a VPP session | Download and upload are policed at the rate on the session interface; a TBF/HTB shaper config under the VPP data plane is refused at verify |
| AC-10 | CoA rate change | Policers replaced on the session; no traffic gap beyond one policer swap |
| AC-11 | PADT from client, LCP Terminate, CoA Disconnect, operator clear | VPP session, routes and policers are gone; PADT sent after the VPP session is deleted; pool address released; accounting Stop sent |
| AC-12 | VPP process killed while sessions are up | Ze reports the loss once, frees every session's state per OD-2, and serves new PADI once `vpp/reconnected` fires and the LCP pair is back |
| AC-13 | Kernel backend (default netlink) | Every existing PPPoE and L2TP test and interop scenario passes unchanged |
| AC-14 | `show pppoe session` on VPP | Names the VPP session interface and its sw_if_index; `ze doctor` reports the VPP pppoe plugin state |
| AC-15 | A policed session ends (PADT, terminate, CoA Disconnect, idle) or a CoA swaps its rate, with many such sessions at once under traffic | The `policer-input` features leave the ip4/ip6 unicast and multicast arcs before the policer is unbound and deleted; VPP keeps its PID; no policer remains for a session that is gone. Source: accel-ppp-ng PR #46 (https://github.com/accel-ppp/accel-ppp-ng/pull/46), where the reverse order let `policer_input_node_fn` run on a deleted policer and VPP died with SIGBUS (VyOS T9502) |
| AC-16 | A VPP session ends while others are up (PADT, operator clear, idle or session timeout) | The session's PPPoE socket, which is its PPP frame channel with no `/dev/ppp` unit, is closed exactly once by its one owner, after it is deregistered from any poller; the daemon and every other session keep running. Source: accel-ppp-ng PR #44 (https://github.com/accel-ppp/accel-ppp-ng/pull/44), where the shared fd was closed early and the later `EPOLL_CTL_DEL` on it failed with EBADF and aborted the daemon, dropping every session (VyOS T9499) |
| AC-17 | A VPP session carries a RADIUS Idle-Timeout or Session-Timeout, and sends traffic | Its byte counters and idle detection come from the session's data-plane handle (the VPP session interface's counters, by the sw_if_index known only after the session interface is created); accounting octets move; a counter read that fails is reported and never read as zero or as idle, and never ends the session as a NAS error. Source: accel-ppp-ng PR #42 (https://github.com/accel-ppp/accel-ppp-ng/pull/42), where the ifindex was resolved before the hook created the interface, stats read nothing, and every session ended with NAS-Error every 60 s (VyOS T9211) |

## End-to-End User Stories
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Subscriber dials PPPoE through a VPP access port and browses | client -> VPP pppoe-cp -> TAP -> Ze discovery/PPP -> VPP session + route -> VPP FIB | interop `pppoe-vpp-ze-ac-pppd-client` |
| 2 | Operator restarts VPP | vpp component -> `vpp/disconnected` -> PPPoE backend reconcile -> `vpp/reconnected` | interop `pppoe-vpp-restart` |
| 3 | RADIUS lowers a subscriber's rate by CoA | authradius CoA -> `SessionRateChange` -> VPP policer replace | `test/vpp/pppoe-vpp-coa-rate.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSessionDataplaneRegistry` | `internal/component/l2tp/pppoe/dataplane_test.go` | Register, lookup by name, duplicate refused, unknown name error lists registered names | |
| `TestKernelDataplaneParity` | `internal/plugins/pppoe/kernel/` | Kernel backend reproduces today's AF_PPPOX + `/dev/ppp` sequence | |
| `TestPPPChannelSeam` | `internal/component/l2tp/ppp/` | PPP session runs over a non-fd frame channel; one reader | |
| `TestPPPoEVPPSessionCreatedBeforePADS` | `internal/plugins/pppoe/vpp/` | PADS withheld when session add fails | |
| `TestPPPoEVPPPeerRouteOnIPCPOpened` | `internal/plugins/pppoe/vpp/` | Route add carries VRF table and session sw_if_index | |
| `TestPPPoEVPPTeardownOrder` | `internal/plugins/pppoe/vpp/` | Route flush, policer delete, session delete, then PADT | |
| `TestPPPoEVPPReconcileOnReconnect` | `internal/plugins/pppoe/vpp/` | OD-2 policy on disconnect and reconnect | |
| `TestPPPoEVPPPolicerFeaturesRemovedBeforePolicer` | `internal/plugins/pppoe/vpp/` | AC-15: on teardown and on a CoA swap the fake GoVPP channel records the four feature disables before the policer unbind and delete | |
| `TestSessionFrameChannelClosedOnceByOwner` | `internal/component/l2tp/ppp/` | AC-16: every session end path closes the frame channel once, after poller deregistration; other sessions keep running | |
| `TestPPPoEVPPCountersFromSessionHandle` | `internal/plugins/pppoe/vpp/` | AC-17: counters read by the sw_if_index the handle learned at create; a failed read is an error, never zero, and never idle | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Session ID | 1-65534 | 65534 | 0 | 65535 (RFC2516-4-1) |
| Session MTU on VPP | 576-1492 (1500+ only with RFC 4638) | 1492 | 575 | 1493 without PPP-Max-Payload |
| Policer rate | existing `traffic.ValidateRate` bounds | per validator | per validator | per validator |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pppoe-vpp-dataplane-refused-without-backend` | `test/parse/` | AC-1, AC-2 | |
| `pppoe-vpp-dataplane-accepted` | `test/parse/` | Valid VPP PPPoE tree | |
| `pppoe-vpp-session` | `test/vpp/` | Session add/route/teardown against the VPP stub harness | |
| `pppoe-vpp-coa-rate` | `test/vpp/` | AC-10 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `pppoe-vpp-ze-ac-pppd-client` | `test/interop-pppoe/scenarios/` | pppd + rp-pppoe client (existing `Dockerfile.client`), real VPP container, Ze container | AC-3..AC-8, AC-11: discovery, CHAP, IPCP/IPv6CP, data forwarded by VPP (VPP session counters, no 0x8864 data on the TAP), clean teardown | |
| `pppoe-vpp-restart` | `test/interop-pppoe/scenarios/` | same | AC-12 | |

Goal validation: G-1 by the first scenario's VPP-counter and TAP-capture assertions; G-2 by AC-1/AC-13; G-3 by AC-6, AC-7, AC-9; G-4 by AC-8; G-5 by the restart scenario. Each scenario is reverted-and-rebuilt once to record its red (`ai/rules/interop-and-goal-validation.md`). The lab Docker host runs Ze's kernel (`ai/META-DECISIONS.md`).

## Files to Modify
- `internal/component/l2tp/pppoe/server.go` - `handlePADR` calls the selected session data plane instead of `pppoeCreate`/`DevPPPSetup`
- `internal/component/l2tp/pppoe/subsystem.go` - select the data plane by backend name; discovery socket opened per data plane (cp TAP, netns); VPP readiness events
- `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` - no leaf added (OD-3); the refusal `spec-pppoe-refuse-vpp-backend` lands is replaced by the data-plane selection
- `internal/component/l2tp/ppp/start_session.go`, `manager.go`, `session.go`, `session_run.go`, `ncp.go`, `ipv6_service*.go`, `ra*.go`, `dhcpv6*.go` - frame channel and per-session data-plane handle replace fds and `IfaceBackend`
- `internal/component/l2tp/` L2TP reactor kernel path - supplies the kernel handle (no behaviour change)
- `internal/component/l2tp/plugins/shaper/` - rate application through the session handle; VPP policer mode
- `internal/component/vpp/startupconf.go`, `config.go`, `yang/ze-vpp-conf.yang` - load `pppoe_plugin.so`; the patched `pppoe` startup section (OD-1 B)
- `rfc/short/rfc2516.md` - performer per backend on the rows above
- `feature-gates.txt` - dependent gate entry for the VPP session data plane

## Files to Create
- `internal/component/l2tp/pppoe/dataplane.go` - the seam: interface and name registry
- `internal/plugins/pppoe/kernel/` - kernel session data plane (moved, not copied)
- `internal/plugins/pppoe/vpp/` - VPP session data plane (`ze_vpp` + `ze_l2tp`)
- `test/interop-pppoe/scenarios/pppoe-vpp-ze-ac-pppd-client/`, `pppoe-vpp-restart/`
- `test/parse/pppoe-vpp-*.ci`, `test/vpp/pppoe-vpp-*.ci`
- OD-1 B: `third_party/vpp-patches/` (diffs against the VPP release) and the build step that applies them

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | No | OD-3: no leaf added |
| YANG validation constraints | Yes | Backend verifier for PPPoE on VPP |
| YANG custom validators | Yes | LCP pair present for each PPPoE access interface |
| CLI commands/flags | Yes | `show pppoe session` gains the data-plane interface |
| CLI grammar | No | No new verb |
| Editor autocomplete | No | |
| Functional test for new RPC/API | Yes | `test/vpp/pppoe-vpp-*.ci` |
| Pipe completeness | No | |
| Env var registration | No | |
| Doctor check for runtime dependencies | Yes | `doctor-vpp-pppoe-plugin` (running VPP lists `pppoe_plugin.so`), following `doctor-vpp-lcp-plugin` |
| Prometheus counters/metrics | Yes | Per-session octets from VPP interface stats feed accounting (shared with `spec-pppoe-subscribers-produce-no-accounting-or-telemetry`) |
| BGP family surface | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature | Yes | `features/` PPPoE entry, VPP data plane level |
| 2 | Config syntax changed | No (OD-3) | `docs/guide/configuration.md` states PPPoE follows `interface backend` |
| 6 | User guide page | Yes | `docs/guide/vpp.md` (status table, PPPoE section), PPPoE guide |
| 9 | RFC behavior proven | Yes | `rfc/short/rfc2516.md` performer annotations |
| 10 | Test infrastructure | Yes | `docs/labs/pppoe-interop.md` (VPP scenarios) |
| 12 | Internal architecture | Yes | `ai/digests/subscriber.md`, `ai/digests/vpp-dataplane.md`, `ai/doors/access-security.md` |
| 16 | Source anchors | Yes | Run `./le spec citation anchors spec plan/post-release/features/spec-pppoe-vpp-dataplane.md` at implementation |

## Implementation Steps

1. **Phase 0: Lab proof of mechanism (no product code).** Real VPP container (af_packet access interface, LCP pair), pppd client, Ze unmodified except a scratch harness. Prove A-1..A-5 and the upstream facts above by capture and `vppctl show pppoe session`. Output: the evidence recorded in this spec, run against the VPP Ze builds with the patches (OD-1 B).
2. **Phase 1: moved out.** The verify-time refusal of PPPoE under `interface backend vpp` is `plan/release/fixes/spec-pppoe-refuse-vpp-backend.md` (OD-5), which owns the journal row and lands before this spec. This spec's AC-1 holds while no VPP session data plane is registered, and Phase 3's registration replaces that refusal with AC-2.
3. **Phase 2: The seam, kernel backend moved behind it.** `pppoe` declares the session data-plane interface and registry; the kernel implementation moves to its own package and registers; `ppp.StartSession` carries a frame channel and a per-session data-plane handle in place of `ChanFD`/`UnitFD`/`UnitNum` and `IfaceBackend`; L2TP supplies the kernel handle. The handle's contract names one owner for each descriptor it holds, which closes it exactly once (AC-16). No behaviour change. Tests: AC-13, AC-16, `TestKernelDataplaneParity`, `TestPPPChannelSeam`.
4. **Phase 3: VPP backend, IPv4.** cp map, discovery and session control on the cp TAP, session create before PADS, MTU, peer route in VRF, framed routes, teardown order, session counters and idle detection read through the session handle, `show pppoe` and doctor. Tests: AC-2..AC-7, AC-11, AC-14, AC-16, AC-17, interop `pppoe-vpp-ze-ac-pppd-client` IPv4.
5. **Phase 4: VPP backend, IPv6.** IPv6CP, RA, DHCPv6 IA_NA/IA_PD, delegated-prefix routes, through patch 0014's punt of RS and DHCPv6 to the cp (OD-1 B). Tests: AC-8.
6. **Phase 5: Rate limiting.** VPP policers per session, CoA replace, verify refuses tc qdisc modes under VPP. Removal order on teardown and on a CoA swap: the `policer-input` features leave the four arcs before the policer is unbound and deleted (AC-15). Tests: AC-9, AC-10, AC-15.
7. **Phase 6: VPP loss and restart.** OD-2 policy on `vpp/disconnected`/`vpp/reconnected`, cp map and LCP pair re-established, discovery resumes. Tests: AC-12, interop `pppoe-vpp-restart`.
8. **Phase 7: Docs, RFC annotations, discrimination records.** `rfc/short/rfc2516.md` performers, `./le rfc record` for each tagged unit, guides and digests.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | PADS never precedes a successful VPP session add; PADT never precedes VPP session delete |
| Data flow | No subscriber IP packet traverses the Linux kernel on the VPP backend (capture) |
| Rule: no-layering | No path tries kernel then VPP; fds removed from `StartSession`, not kept beside the handle |
| Rule: exact-or-reject | tc shaping config refused under VPP, never mapped to a policer silently |
| Rule: process boundary | The VPP backend reaches the connector the same way `fib-vpp` does; nothing calls into `pppoe` from another component |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Seam with two registered backends | `rg RegisterSessionDataplane internal/` shows kernel and vpp registrations |
| VPP session data plane | interop `pppoe-vpp-ze-ac-pppd-client` passes with recorded red |
| Restart handling | interop `pppoe-vpp-restart` passes with recorded red |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Frames read off the cp TAP are parsed with the existing bounded discovery and PPP codecs; a frame whose (MAC, SID) has no session is dropped and counted |
| Spoofing | Session add only after cookie-verified PADR; a client MAC already bound on another access interface is refused (upstream `link_table` collision) |
| Resource exhaustion | `max-sessions` bounds VPP interfaces; PADI limiter applies on the TAP |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The seam ng calls "session hooks" is a per-session data-plane handle, not a global function table. In Ze it is a registered backend that returns, per session, an object owning that session's data-plane state; the PPP package programs addresses, routes and rate through that object and never names a backend.
- Subscriber routes cannot go through sysrib and fib-vpp until `BestChangeEntry` carries an egress interface (seams-3). The VPP session backend installs them directly with its own route source, exactly as `internal/plugins/static/vpp` does for interface routes, and owns their removal.
- The cp TAP is a Linux interface in the LCP namespace, so Ze's existing raw-socket and PPP codecs carry over; only their binding target changes. That is why this is a backend and not a rewrite.

## Owned journal rows

None. The row this spec found (`validate-accepts-what-start-refuses.md`, 2026-10-28) is owned by `plan/release/fixes/spec-pppoe-refuse-vpp-backend.md`.

## Work Not Done

| Item | Where it went | Why |
|------|---------------|-----|
| Phase 1: refuse PPPoE under `interface backend vpp` at verify | `plan/release/fixes/spec-pppoe-refuse-vpp-backend.md` | OD-5: a shipped-behaviour fix belongs to the release; the rest of this spec is post-release |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D-1 Session data plane is a backend registered by name in the `pppoe` component, with kernel and vpp implementations in their own packages | A hook table like ng; a switch on backend name in `subsystem.go` | Registration pattern (`ai/doors/system.md`); a third backend (BSD netgraph/pipex, `spec-bsd-l2tp-pppoe`) plugs in the same way |
| D-2 One backend per configuration, no fallback; selection follows the interface backend | Kernel fallback when VPP is down | `ai/rules/no-layering.md`; a VPP-owned access NIC has no kernel data path to fall back to |
| D-3 `ppp.StartSession` carries a frame channel and a data-plane handle; fds and `IfaceBackend` are removed | Keep fds and add an optional handle | No-layering: two contracts for one job is the hybrid the rule bans |
| D-4 Ze keeps all control: discovery, PPP, auth, NCP, DHCPv6, RA, RADIUS | VPP auto session management (patch 0013 sniffing) | Ze is the RFC 2516 AC; VPP sniffing would make a second owner of session state |
| D-5 PADS after VPP session add; VPP session delete before PADT | Optimistic PADS | Preserves today's invariant (`handlePADR`) and RFC2516-5.5-4 |
| D-6 Subscriber routes installed by the VPP session backend directly | Through sysrib and fib-vpp | `BestChangeEntry` has no egress interface (seams-3) |
| D-7 Rate on VPP is policing, declared as such; tc shaping modes refused under VPP | Map TBF to a policer | Exact-or-reject (`fw-7-traffic-vpp.md`) |

## Owner decisions (decided 2026-10-28)

Thomas approved the recommendation for each of the five decisions. None is open.

| ID | Decision | Chosen |
|----|----------|--------|
| OD-1 | Which VPP | **B now, C in parallel.** Ze builds VPP itself with the VyOS patches (0013, 0014, 0018, 0025, 0031) kept as diffs against the VPP release and applied at build time, and generates its own `pppoe` binapi from the patched `.api`; in parallel the needed changes are offered upstream to FD.io, and a patch is dropped once its change lands in a VPP release Ze builds |
| OD-2 | VPP loss policy | **A.** On `vpp/disconnected` tear down every VPP session in Ze (no PADT is possible), Stop accounting with a lost-service cause, release pool addresses; clients redial when LCP echo fails |
| OD-3 | Config surface | **A.** No new leaf: PPPoE's data plane follows `interface backend`; `pppoe_plugin.so` loads whenever `pppoe` is configured with the VPP backend |
| OD-4 | Subscriber rate on VPP | **A.** Police both directions; TBF and HTB shaping under VPP are refused at verify until a VPP shaping mechanism exists |
| OD-5 | Release bucket | **Post-release**, except Phase 1 (refuse PPPoE on the VPP backend at verify), which moved to `plan/release/fixes/spec-pppoe-refuse-vpp-backend.md` |

The options as they were put to the owner:

| ID | Decision | Option A | Option B | Option C | Evidence |
|----|----------|----------|----------|----------|----------|
| OD-1 | Which VPP | Upstream VPP only. Limits: one access interface per VPP (global cp, MAC-keyed `link_table`); VPP session created at first NCP Opened with the peer address (IPv6-only sessions need a workaround: unique peer link-local per session); upload policing via a classify table per session; IPv6 via VPP-native RA (`ip6_nd` API) and a punt socket for UDP 547 (new DHCPv6 transport in Ze) | Ze builds VPP itself with patches 0013, 0014, 0018, 0025, 0031 kept as diffs against the VPP release and applied at build time (the rule Ze already applies to Go modules); Ze generates the `pppoe` binapi from the patched `.api` | Upstream the needed changes to FD.io first and use A until they land | Upstream facts table above (`pppoe_cp.c`, `pppoe.c`, `pppoe_decap.c`, `node_funcs.c`); patches in `vyos/vyos-vpp-patches`; Ze today execs the host's `/usr/bin/vpp` (`internal/component/vpp/register.go`) and builds no VPP; VyOS ships B |
| OD-2 | VPP loss policy | Tear down every VPP session in Ze on `vpp/disconnected` (no PADT is possible: the control path is VPP), Stop accounting with a lost-service cause, release pools; clients redial when LCP echo fails | Keep Ze's sessions and replay VPP state on `vpp/reconnected`; a session whose client MAC VPP has not relearned (`INVALID_SW_IF_INDEX`) waits for the client's next control frame | - | ng does A (commit `90c6d35`); B depends on `link_table` relearning (`pppoe.c`) and on clients surviving the outage within their LCP echo budget |
| OD-3 | Config surface | No new leaf: PPPoE's data plane follows `interface backend`; `pppoe_plugin.so` loads whenever `pppoe` is configured with the VPP backend | Explicit `pppoe dataplane kernel\|vpp` leaf plus a `vpp plugins pppoe` toggle like `vpp plugins wireguard` | - | "every fact declared once" (`ai/rules/principles.md`); wireguard precedent in `startupconf.go`; ng uses a per-interface `vpp-cp=true` option |
| OD-4 | Subscriber rate on VPP | Police both directions (ng behaviour) and refuse TBF/HTB under VPP | Also implement shaping by another VPP mechanism | - | `vpppolicer.c`; `fw-7-traffic-vpp.md` exact-or-reject |
| OD-5 | Release bucket | Post-release feature (this file's bucket) | Release scope | - | `plan/README.md` buckets; Phase 1 is a shipped-behaviour fix and may move to `plan/release/fixes/` on its own |

## Known Limitations
- No VPP-side PADI or LCP policing of the cp path in this spec; that is copp work (`spec-dataplane-seams-0-umbrella` F-5).
- L2TP LNS sessions stay on the kernel data plane; VPP has an L2TPv3 plugin, not an L2TPv2 LNS.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
The VPP backend's session add, PADS ordering, teardown ordering and MTU setting
each carry the RFC 2516 quote of the row they discharge; callers carry the
section reference (`ai/rules/rfc-compliance.md`, functions added from 2026-09-24).

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, and `./le verify changed` over the closing commits
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/features/spec-pppoe-vpp-dataplane.md` only, in the same `./le commit create` script

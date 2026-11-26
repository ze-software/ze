# Spec: rfc4861-ze-owned-must-gaps

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Ze ships Router Advertisement origination, Router Solicitation answering and VRRP unsolicited Neighbor Advertisements, so shipped code must meet the RFC 4861 MUSTs of those capabilities. The nine rows below are `{gap}` in `rfc/short/rfc4861.md`; ND Proxy and AH/ESP-secured ND were split into `plan/post-release/features/spec-rfc4861-nd-proxy-and-ipsec-nd.md` per the 2026-11-24 owner decision |
| RFC gaps | RFC4861-6.1.1-2, RFC4861-4.6-1, RFC4861-4.4-3, RFC4861-6.2.1-1, RFC4861-6.2.1-8, RFC4861-6.2.4-1, RFC4861-6.2.5-2, RFC4861-7.2.6-1, RFC4861-9-7 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

| Topic | Decision |
|-------|----------|
| ND Proxy (`RFC4861-7.2.8-1`, `RFC4861-7.2.8-2`) and AH/ESP-secured ND (`RFC4861-11.2-1`, `RFC4861-11.2-2`) | Move both unstarted features to `plan/post-release/features/` as honest post-release gaps (not declined). Executed: created `plan/post-release/features/spec-rfc4861-nd-proxy-and-ipsec-nd.md`. |
| RA design decision D-6 (`RFC4861-6.2.4-1`, `RFC4861-6.2.5-2`) | Reverse D-6: suppress non-zero Router Lifetime RAs when IPv6 forwarding is off, send a final RA with Router Lifetime 0 when forwarding turns off, keep the `ze doctor` warning, and update the RA design document. |

## Task

Close the nine shipped-capability RFC 4861 MUST gaps that remain in Ze's Router Advertisement sender, Router Solicitation receivers, and VRRP unsolicited Neighbor Advertisement path.

| Row | Producer (HEAD) | Defect and design fix |
|-----|-----------------|----------------------|
| RFC4861-6.1.1-2, RFC4861-4.6-1 | `readSolicitations` (`internal/plugins/iface/ra/sender_linux.go`); shared validator in `internal/component/l2tp/ppp/router_solicitation.go` (already wired in `internal/component/l2tp/ppp/kernel/ipv6.go::(*ipv6Carrier).ReadRS` and `internal/component/l2tp/pppoe/dataplane/vpp/ipv6_wire.go::(*controlSocket).dispatchIPv6`) | `openRASocket` does not enable `ipv6.FlagHopLimit`, `rsReadBufferSize` truncates at 256 octets, and `readSolicitations` signals every packet the ICMPv6 type filter passes with zero validity checks. Promote `ValidRouterSolicitation` to `internal/core/ndp/rs.go` with typed discard reasons, delegate `ppp.ValidRouterSolicitation` to it, enable `ipv6.FlagHopLimit` and 1500-octet reads in `sender_linux.go`, validate every RS in `readSolicitations` before signaling, count discards in `ze_iface_ra_rs_discarded_total{interface, reason}`, and add RFC-tagged tests with discrimination records on both LAN and PPP subscriber paths. |
| RFC4861-4.4-3 | `validateGroup` (`internal/plugins/vrrp/groups.go`), `frameBuilder` (`internal/plugins/vrrp/transport/transport.go`), `BuildNA` (`internal/plugins/vrrp/transport/na.go`) | `validateGroup` requires only `VIPs[0]` to be link-local unicast and accepts multicast IPv6 addresses in `VIPs[1:]`, which `BuildNA` copies unchecked into the NA Target Address. Refuse any multicast IPv6 `virtual-address` in `validateGroup` with an operator error naming RFC 4861 Section 4.4 and the fix, and guard `BuildNA` (return 0) and `frameBuilder` (return `0, false`) against multicast targets as defense in depth. |
| RFC4861-7.2.6-1 | `(*announcer).runBurst` (`internal/plugins/vrrp/transport/announce.go`), `OpenInstance` (`internal/plugins/vrrp/transport/transport.go`) | `announcer` shares one 100 ms `announceRepeatInterval` across IPv4 GARP and IPv6 unsolicited NA, violating RFC 4861 Section 7.2.6 (`>= RetransTimer`, default 1,000 ms). Give `announcer` a family-specific repeat interval: keep 100 ms for IPv4 GARP, use `max(interface retrans_time_ms, 1000 ms)` for IPv6 unsolicited NA, send round-by-round so every VIP announces immediately on promotion while repeats stay separated by at least `RetransTimer`, and interrupt inter-repeat waits immediately when `close()` closes `stop`. |
| RFC4861-6.2.1-1 | `ze-iface-conf.yang`, `parseRAConfig`/`raValidate` (`internal/component/iface/config_ra.go`), `raSpecFor`/`raAdvertisementEqual` (`internal/component/iface/reconcile_ra.go`), `RALen`/`BuildRA` (`internal/core/ndp/ra.go`) | `AdvLinkMTU` cannot be configured, so no MTU option (type 5, length 1, reserved 0, 32-bit MTU) is sent. Add `link-mtu` (`uint32`, range `0 \| 1280..65535`, default `0`, units `octets`) under `container router-advertisement`, parse and validate it, propagate it in `RASenderSpec`, compare it in `raAdvertisementEqual`, and encode the RFC 4861 Section 4.6.4 MTU option in `ndp.BuildRA` when non-zero. |
| RFC4861-6.2.1-8 | `applySysctl` (`internal/component/iface/config_sysctl.go`), `applyConfigPass` (`internal/component/iface/config_apply.go`), `internal/core/sysctl/known_linux.go` | Advertised `hop-limit`, `reachable-time`, and `retransmit-timer` are never applied to the advertising interface's own Linux ND variables. Register the three per-interface sysctl templates in `internal/core/sysctl/known_linux.go` (`net.ipv6.conf.<iface>.hop_limit`, `net.ipv6.neigh.<iface>.base_reachable_time_ms`, `net.ipv6.neigh.<iface>.retrans_time_ms`), bound `retransmit-timer` to Linux's `0..2147483647` ms range under exact-or-reject, and emit them via EventBus under dedicated source `interface-ra` for every enabled RA unit whose respective value is non-zero (clearing stale defaults when disabled or returned to 0). |
| RFC4861-6.2.4-1, RFC4861-6.2.5-2 | `reconcileRA` (`internal/component/iface/reconcile_ra.go`), `runEngine` (`internal/component/iface/register.go`), `(*Sender).run` (`internal/plugins/iface/ra/sender_linux.go`), `checkRAForwarding` (`internal/plugins/iface/ra/doctor.go`) | Reversing D-6 per Thomas's 2026-11-24 ruling: when IPv6 forwarding is off on an interface (per-device `forwarding == 0` or kernel forwarding off: `all.forwarding == 0` and `force_forwarding != 1`), suppress non-zero Router Lifetime RAs in both `reconcileRA` (do not start a sender; stop any running sender so `Stop` emits one final Router Lifetime 0 RA and leaves `ff02::2`) and `Sender.run` (defense in depth for external `/proc/sys` flips: send one final Router Lifetime 0 RA on ON->OFF transition, then suppress). Subscribe `iface` to `(sysctl, applied)` for forwarding keys to trigger `reconcileRA`. Preserve explicit `router-lifetime 0` (prefix/RDNSS-only, non-default-router mode). Keep the `doctor-iface-ra-forwarding` warning and update `docs/features/interfaces.md`. |
| RFC4861-9-7 | `ndp.SplitRA` (`internal/core/ndp/ra.go`), `(*Sender).run` (`internal/plugins/iface/ra/sender_linux.go`) | `list prefix` has no link-MTU bound and `Sender.run` puts every option in one RA, exceeding a 1500-octet link MTU past ~44 prefixes. Implement `ndp.SplitRA(cfg RAConfig, linkMTU int) []RAConfig` guaranteeing `40 + RALen(chunk) <= effectiveMTU` (with `effectiveMTU >= 1280` and capped by `cfg.LinkMTU` when non-zero) across Prefixes, Routes, RDNSS, and RDNSSWithdrawn, and have `Sender.run` transmit every chunk using the resolved device MTU. |

### Reference Daemons Checked (FRR and BIRD)

FRR (`FRRouting/frr`, `zebra/rtadv.c`) and BIRD (`labs/bird`, `proto/radv/packets.c`) were checked for the deciding functions behind these gaps:

| Topic | FRR (`zebra/rtadv.c`) | BIRD (`proto/radv/packets.c`) | How Ze aligns |
|-------|-----------------------|-------------------------------|---------------|
| RS receive validation | `rtadv_read` / `rtadv_recv_packet` / `rtadv_process_solicit` fetch ancillary hop limit and source address before counting or answering a solicitation | `radv_rx_hook` validates ICMPv6 option lengths against remaining packet bounds and drops zero-length or truncated options | `ndp.ValidateRouterSolicitation` checks Hop Limit 255, non-multicast IPv6 source, type 133, code 0, length `>= 8`, all option lengths `> 0` and in bounds, and no SLLA from `::` before counting or answering |
| AdvLinkMTU | `rtadv_send_packet` appends `struct nd_opt_mtu` (`ND_OPT_MTU`, len 1, reserved 0, `htonl(AdvLinkMTU)`) only when `zif->rtadv.AdvLinkMTU != 0` | `link mtu` defaults to 0 (omitted) and emits `OPT_MTU` in `radv_prepare_ra` when configured non-zero | `link-mtu` leaf defaults to 0 (omitted); `ndp.BuildRA` emits type 5, len 1, reserved 0, big-endian `LinkMTU` when non-zero |
| Link MTU bound | `rtadv_send_packet` tracks message size inside `RTADV_MSG_SIZE` against interface MTU | `radv_prepare_ra` packs options inside the link MTU | `ndp.SplitRA` splits variable options into multiple RAs so every packet satisfies `40 + RALen(chunk) <= effectiveMTU` per RFC 4861 Sections 6.2.3 and 9 |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, process boundary, registration, and shared machinery
  → Constraint: plugins can run in another process; kernel tunables go through `sysctl` over the EventBus; buffer-first encoding uses caller buffers
- [ ] `ai/doors/forwarding.md` - `iface`, `sysctl`, kernel backend ownership, and `ze:backend "kernel"` rules
  → Constraint: `iface` emits sysctl defaults on the EventBus (`internal/component/sysctl/events`), never writes `/proc` directly; interface names go through the shared resolver
- [ ] `ai/doors/access-security.md` - L2TP/PPP subscriber session stack and `ppp` leaf package boundary
  → Constraint: `internal/component/l2tp/ppp` is a leaf package with no component imports, and may import `internal/core/ndp`
- [ ] `ai/doors/core.md` - `internal/core/` leaf tier import direction
  → Constraint: `internal/core/ndp` and `internal/core/sysctl` import neither `internal/component/` nor `internal/plugins/`
- [ ] `ai/doors/system-plugins.md` - edge plugin placement and boundary rules for `internal/plugins/vrrp`
  → Constraint: cross-leaf VRRP config rules stay pure in `groups.go` (`InProcessConfigVerifier` and `OnConfigVerify`) so `ze config validate` works offline
- [ ] `docs/features/interfaces.md` - Router Advertisement configuration, send loop, counters, and D-6 forwarding behavior
  → Decision: D-6 ("Ze reports the state instead") is reversed by the 2026-11-24 owner decision; update `Send and accept are separate`, `Container leaves`, `Send loop`, and `Counters`
- [ ] `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` - VRRP verification, transport, and unsolicited NA announcer
  → Constraint: VRRP IPv4 GARP keeps 100 ms spacing; IPv6 unsolicited NA must obey RFC 4861 Section 7.2.6 and Section 4.4
- [ ] `docs/research/l2tpv2-ze-integration.md` - PPP IPv6 carrier contract and RS admission
  → Decision: `internal/component/l2tp/ppp/router_solicitation.go::ValidRouterSolicitation` is already wired into both kernel and VPP carriers and delegates to shared `internal/core/ndp`

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4861.md` and `rfc/full/rfc4861.txt`, Sections 4.4, 4.6, 4.6.4, 6.1.1, 6.2.1 to 6.2.5, 7.2.6, 9, and 10
  → Constraint: every MUST/MUST NOT row listed in `RFC gaps` receives both positive and negative tagged tests and recorded discrimination verdicts via `./le rfc record`

**Key insights:**
- On HEAD, `internal/component/l2tp/ppp/ra_linux.go` no longer exists: PPP subscriber RS validation already landed in `internal/component/l2tp/ppp/router_solicitation.go::ValidRouterSolicitation`, invoked by `internal/component/l2tp/ppp/kernel/ipv6.go::(*ipv6Carrier).ReadRS` (with `FlagHopLimit|FlagInterface` enabled in `ipv6_linux.go::configureRASocket`) and `internal/component/l2tp/pppoe/dataplane/vpp/ipv6_wire.go::(*controlSocket).dispatchIPv6`. What remains on the PPP side is sharing that validator from `internal/core/ndp` and adding RFC-tagged tests with discrimination records.
- `internal/plugins/iface/ra/advertiser.go` already owns the CE Router Lifetime matrix (`routerLifetime`, `urgent`) and `sender_linux.go` already owns `sendFinal` (one Router Lifetime 0 RA respecting `ndp.CeaseWait`). Reversing D-6 extends those existing lifecycle paths instead of adding a parallel shutdown path.
- Existing `TestRASolicitedResponse` (`internal/plugins/iface/ra/rfc4861_ra_integration_linux_test.go`) requires IPv6 forwarding off to prove the socket's own `ff02::2` join (`RFC4861-6.2.2-2`); setting `RouterLifetime: 0` in that test preserves its exact proof under Thomas's D-6 reversal ("suppress non-zero Router Lifetime RAs when IPv6 forwarding is off") while enabling forwarding in `withRAVeth` for router-mode integration tests.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/iface/ra/sender_linux.go` - `openRASocket` sets multicast/unicast hop limit 255, joins `ff02::2`, and sets `ICMP6_FILTER` for type 133, but does not call `SetControlMessage(ipv6.FlagHopLimit, true)`. `readSolicitations` reads into a 256-octet buffer, ignores `n`, `cm`, and `src`, and signals `ch` for every datagram. `Sender.run` sends one `BuildRA` call regardless of `binding.MTU` and regardless of interface IPv6 forwarding state. `sendFinal` sends one Router Lifetime 0 RA respecting `ndp.CeaseWait`.
- [ ] `internal/plugins/iface/ra/advertiser.go` - `advertiser.build` assembles configured prefixes, delegated prefixes, retiring withdrawals, RDNSS, and CE `routerLifetime`; does not check interface forwarding state or link MTU.
- [ ] `internal/plugins/iface/ra/ifacera.go` - registers only `ze_iface_ra_sent_total` and `ze_iface_ra_solicited_total`; no RS discard counter.
- [ ] `internal/plugins/iface/ra/doctor.go` and `doctor_linux.go` - `checkRAForwarding` reads `net.ipv6.conf.<dev>.forwarding` and `all.forwarding`/`force_forwarding` via `readIPv6Forwarding` / `readIPv6KernelForwarding` and emits `doctor-iface-ra-forwarding` warning (implementing D-6).
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `container router-advertisement` has `enabled`, intervals, `router-lifetime`, `hop-limit`, `managed`, `other-config`, `reachable-time`, `retransmit-timer` (unbounded `uint32`), `prefix` list (no `max-elements`), and `rdnss`; no `link-mtu` leaf.
- [ ] `internal/component/iface/config_ra.go` - `parseRAConfig` and `raValidate` parse and bound Section 6.2.1 leaves; no `LinkMTU` field and no upper bound check on `retransmit-timer`.
- [ ] `internal/component/iface/reconcile_ra.go` - `reconcileRA` starts a sender for every enabled unit with `router-advertisement enabled true`, ignoring forwarding state; `RASenderSpec` and `raAdvertisementEqual` carry no `LinkMTU`.
- [ ] `internal/component/iface/config_sysctl.go` and `config_apply.go` - `applySysctl` emits `autoconf`, `accept_ra`, and `forwarding` per unit and `applyIPv6ForwardingSwitch` manages `net.ipv6.conf.all.forwarding` under source `interface-ipv6-forwarding`; never emits `hop_limit`, `base_reachable_time_ms`, or `retrans_time_ms`.
- [ ] `internal/component/iface/register.go` - calls `reconcileRA` on startup, config apply, rollback, delegation updates, and RDNSS updates; does not subscribe to `(sysctl, applied)` for forwarding changes.
- [ ] `internal/core/sysctl/known_linux.go` - registers `net.ipv6.conf.<iface>.{autoconf,accept_ra,forwarding}` and `all.forwarding`; does not register `net.ipv6.conf.<iface>.hop_limit`, `net.ipv6.neigh.<iface>.base_reachable_time_ms`, or `net.ipv6.neigh.<iface>.retrans_time_ms`.
- [ ] `internal/core/ndp/ra.go` - encodes RA header, Source Link-layer Address, Prefix Information, Route Information, and RDNSS options; has no `OptMTU`, no `LinkMTU` field, and no `SplitRA`.
- [ ] `internal/component/l2tp/ppp/ra_carrier.go`, `router_solicitation.go`, `ra_send.go`, `kernel/ipv6.go`, `kernel/ipv6_linux.go`, and `internal/component/l2tp/pppoe/dataplane/vpp/ipv6_wire.go` - `ra_linux.go` is gone. `ValidRouterSolicitation` already checks Hop Limit 255, non-multicast IPv6 source, length `>= 8`, type 133, code 0, non-zero in-bounds option lengths, and no SLLA from `::`, and both the Linux kernel carrier (`kernel/ipv6.go::ReadRS` with `FlagHopLimit|FlagInterface`) and the VPP carrier (`pppoe/dataplane/vpp/ipv6_wire.go::dispatchIPv6`) already call it. PPP RAs emit only the 16-octet header (`56` octets with IPv6 header, always inside link MTU). The existing PPP solicitation tests are not yet tagged for `RFC4861-6.1.1-2` / `RFC4861-4.6-1`.
- [ ] `internal/plugins/vrrp/groups.go` - `validateGroup` checks `g.VIPs[0].IsLinkLocalUnicast()` for IPv6 but checks only `!vip.Is4()` for `VIPs[1:]`, admitting multicast IPv6 virtual addresses.
- [ ] `internal/plugins/vrrp/transport/na.go` - `BuildNA` writes `vip` straight into Target Address with no multicast check.
- [ ] `internal/plugins/vrrp/transport/announce.go` and `transport.go` - `announcer` sends `announceRepeatCount` (3) frames per VIP separated by `announceRepeatInterval` (100 ms) for both IPv4 GARP and IPv6 NA, and sleeps sequentially per VIP via `time.Sleep`.

**Behavior to preserve:**
- Every existing RA wire field, option order (with MTU placed before Prefixes when non-zero), schedule timing, CE Router Lifetime matrix (RFC 7084 G-4/G-5/L-4), delegated-prefix withdrawal queue, and RDNSS selection when `link-mtu` is omitted (`0`) and forwarding is on.
- Explicit `router-lifetime 0` prefix/RDNSS-only advertising even when forwarding is off (non-default-router mode).
- `ze doctor` warning `doctor-iface-ra-forwarding` when RA is configured with forwarding off.
- `sysctl` layer precedence (`config > transient > default`): an explicit operator `sysctl { setting ... }` overrides `interface-ra` defaults.
- IPv4 VRRP GARP burst behavior: 3 frames per VIP separated by 100 ms.
- Existing PPP `IPv6Carrier` RS validation and `ppp.ValidRouterSolicitation` call signature.
- Every existing RFC 4861 tagged test and discrimination record (updating `TestRASolicitedResponse` to `RouterLifetime: 0` so its `RFC4861-6.2.2-2` all-routers join proof remains valid with forwarding off, and re-recording its verdict).

**Behavior to change:**
- Validate every received Router Solicitation in `internal/plugins/iface/ra/sender_linux.go::readSolicitations` via shared `internal/core/ndp/rs.go::ValidateRouterSolicitation`, enable `ipv6.FlagHopLimit` and 1500-octet reads, and count discards by reason in `ze_iface_ra_rs_discarded_total{interface, reason}`.
- Refuse multicast IPv6 `virtual-address` entries in VRRP `validateGroup`, `frameBuilder`, and `BuildNA`.
- Space repeated VRRP IPv6 unsolicited Neighbor Advertisements by at least `RetransTimer` (`max(interface retrans_time_ms, 1000 ms)`), transmitting round-by-round across VIPs and interrupting wait on `close()`.
- Add `link-mtu` (`0 | 1280..65535`, default `0`) to `router-advertisement` YANG, config parse, `RASenderSpec`, and `ndp.BuildRA` MTU option encoding.
- Emit per-interface RA ND sysctl defaults (`hop_limit`, `base_reachable_time_ms`, `retrans_time_ms`) through the `sysctl` EventBus under source `interface-ra` for enabled RA units with non-zero values, clearing them on disable or zero.
- Reverse D-6: suppress non-zero Router Lifetime RAs when IPv6 forwarding is off, send one final Router Lifetime 0 RA when forwarding turns off, subscribe `iface` to `(sysctl, applied)` forwarding changes, and update `docs/features/interfaces.md`.
- Split Router Advertisements in `ndp.SplitRA` and `Sender.run` so no transmitted ND packet exceeds the link MTU (`40 + RALen(chunk) <= effectiveMTU`).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| PPP Router Solicitation validator | `internal/component/l2tp/ppp/router_solicitation.go` `ValidRouterSolicitation`; callers `internal/component/l2tp/ppp/kernel/ipv6.go` `(*ipv6Carrier).ReadRS` and `internal/component/l2tp/pppoe/dataplane/vpp/ipv6_wire.go` `(*controlSocket).dispatchIPv6` | Validates RFC 4861 §6.1.1 / §4.6 RS packets (Hop Limit 255, non-multicast IPv6 source, type 133, code 0, length `>= 8`, option lengths `> 0` in bounds, no SLLA from `::`) for both kernel and VPP PPPoE/L2TP subscriber carriers | extends: promote the validator into `internal/core/ndp/rs.go` with typed discard reasons so `internal/plugins/iface/ra` and `internal/component/l2tp/ppp` share one implementation; keep `ppp.ValidRouterSolicitation` as a wrapper |
| Core ND encoder | `internal/core/ndp/ra.go` `RAConfig`, `RALen`, `BuildRA` | Buffer-first zero-alloc RA encoder shared by `iface/ra` and `l2tp/ppp` | extends: add `OptMTU`, `RAConfig.LinkMTU`, MTU option encoding, `IPv6HeaderLen`, `MinIPv6LinkMTU`, `RetransTimer`, and `SplitRA` |
| Interface RA reconciler and sender | `internal/component/iface/reconcile_ra.go` `reconcileRA`, `RASenderSpec`; `internal/plugins/iface/ra/sender_linux.go` `Sender.run`, `sendFinal`, `readSolicitations` | Manages one RA sender goroutine per enabled unit, handles in-place updates, and sends one Router Lifetime 0 final RA on `Stop` | extends: carry `LinkMTU`, enforce forwarding suppression and ON->OFF final RA in both `reconcileRA` and `Sender.run`, validate RS in `readSolicitations`, and split packets with `ndp.SplitRA` |
| Interface sysctl EventBus integration and registry | `internal/component/iface/config_sysctl.go` `applySysctl`, `applyIPv6ForwardingSwitch`; `internal/core/sysctl/known_linux.go` `MustRegister`; `internal/component/sysctl/events/events.go` `EventDefault`, `EventClearSourceDefaults`, `EventApplied` | Emits per-interface sysctl defaults with source attribution, clears stale defaults by source, and broadcasts `EventApplied` when kernel tunables change | extends: register the three IPv6 ND sysctl templates, emit them under source `interface-ra` for enabled RA units, and subscribe `iface` to `EventApplied` for forwarding keys |
| RA forwarding doctor check and procfs probes | `internal/plugins/iface/ra/doctor.go` `checkRAForwarding`; `doctor_linux.go` `readIPv6Forwarding`, `readIPv6KernelForwarding` | Reads `net.ipv6.conf.<dev>.forwarding`, `all.forwarding`, and `force_forwarding`, and warns under `doctor-iface-ra-forwarding` | extends: reuse the same Linux forwarding probe logic for `Sender` and `reconcileRA` while keeping the `ze doctor` warning per Thomas's 2026-11-24 ruling |
| VRRP group verifier and announcer | `internal/plugins/vrrp/groups.go` `validateGroup`; `internal/plugins/vrrp/transport/announce.go` `announcer`; `na.go` `BuildNA` | Verifies cross-leaf VRRP config rules offline, builds unsolicited NAs, and runs one long-lived announcer goroutine per instance | extends: add multicast IPv6 VIP refusal in `validateGroup`/`frameBuilder`/`BuildNA` and per-family repeat spacing (`>= RetransTimer` for IPv6 NA, round-by-round) in `announcer` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "Every plugin, ours included, runs either internal (a goroutine in the daemon) or external (a forked subprocess that talks only over TLS and RPC)." | agrees | Every new cross-unit interaction stays process-safe: `iface` reaches `sysctl` exclusively over EventBus messages (`EventDefault`, `EventClearSourceDefaults`, `EventApplied`), and VRRP multicast validation and unsolicited NA spacing stay inside `internal/plugins/vrrp`. |
| `ai/doors/forwarding.md` | Boundaries | the door covering `internal/plugins/iface/ra/*`, `internal/component/iface/*` | "Interface events reach consumers on the EventBus. BGP and `iface` do not import each other (`core-design.md`, section 14)." | agrees | Advertised `hop-limit`, `reachable-time`, and `retransmit-timer` are applied to the router's own interface through `sysctl` EventBus defaults under source `interface-ra`, and forwarding transitions arrive via `(sysctl, applied)` on the EventBus; no `/proc` writes or new cross-component imports are added. |
| `ai/doors/access-security.md` | Boundaries | the door covering `internal/component/l2tp/ppp/router_solicitation.go`, `internal/component/l2tp/ppp/kernel/rfc4861_solicitation_reader_test.go` | "PPP, LCP and NCP state machines, no component import (core-design section 19)." | agrees | `internal/component/l2tp/ppp/router_solicitation.go` imports only `internal/core/ndp` (core leaf tier), never any component package. |
| `ai/doors/core.md` | Boundaries | the door covering `internal/core/ndp/*`, `internal/core/sysctl/known_linux.go` | "Where core needs something a component owns, the component registers into core. Do not add a row to the core import baseline." | agrees | `internal/core/ndp` and `internal/core/sysctl` use only standard library and core leaf packages and import neither `internal/component/` nor `internal/plugins/`. |
| `ai/doors/system-plugins.md` | Boundaries | the door covering `internal/plugins/vrrp/*` | "Every plugin here can run in another process." | agrees | VRRP multicast VIP validation stays pure in `groups.go`, NA spacing lives inside `vrrp/transport`, and no new in-process cross-component Go calls are introduced. |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `test/plugin/iface-ra-link-mtu-and-forwarding.ci` | "A `.ci` that needs a Linux kernel feature carries `option=needs-linux`" | agrees | `test/plugin/iface-ra-link-mtu-and-forwarding.ci` carries `option=needs-linux:caps=net-admin` so Linux RA socket and forwarding checks execute inside the QEMU VM. |
| `docs/features/interfaces.md` | Send and accept are separate | the `// Design:` page of `internal/plugins/iface/ra/*`, `internal/component/iface/*`; a page this spec modifies | "Config verify cannot catch this, because forwarding can arrive from a sysctl profile after verify runs. Ze reports the state instead." | departs | Reversed by Thomas's 2026-11-24 owner ruling on D-6: Ze now suppresses non-zero Router Lifetime RAs while IPv6 forwarding is off, sends one final Router Lifetime 0 RA when forwarding turns off, keeps the `doctor-iface-ra-forwarding` warning, and updates this section of `docs/features/interfaces.md`. |
| `docs/features/interfaces.md` | Design decisions | decision of 2026-11-22 (`docs/features/interfaces.md`) | "`ze init` claims every discovered device by default, including the ones another program owns. `ze init --confirm` asks about each discovered device and writes only the ones the operator confirms" | agrees | This spec does not change `ze init` device discovery or confirmation behavior; all RA and sysctl changes apply only to explicitly configured `router-advertisement` units. |
| `docs/features/interfaces.md` | Design decisions | decision of 2026-11-22 (`docs/features/interfaces.md`) | "`ze init --confirm` takes only an explicit answer: `yes`, `y`, `no` or `n`. Anything else, an empty line included, is not accepted, and the same device is asked again" | agrees | This spec does not touch `ze init --confirm` prompt handling; its input acceptance rules remain intact. |
| `docs/architecture/iface/management.md` | The resolver fan-out discards the oldest, never the newest | anchored to `internal/plugins/iface/ra/sender_linux.go`, `internal/component/iface/yang/ze-iface-conf.yang`, `config_sysctl.go`, `register.go` | "The router-advertisement sender does not: `Sender.onLinkEvent` records a down in `state.linkDown`, and its timer branch returns without rearming while that flag is set, so the next `up` is the only thing that can restart advertisements." | agrees | Link-down pausing and link-up initial-burst restart in `Sender.onLinkEvent` remain unchanged; forwarding suppression is tracked orthogonally and recovers on forwarding re-enable or config reconcile. |
| `docs/architecture/config/yang-config-design.md` | 1. Key Principle | anchored to `internal/component/iface/yang/ze-iface-conf.yang` | "YANG defines format. Extensions declare behavior. Implementation executes behavior." | agrees | The new `link-mtu` leaf declares native YANG type and range (`uint32`, `0 \| 1280..65535`, default `0`) inside the kernel-only `router-advertisement` container, and `config_ra.go` re-checks the bounds. |
| `docs/architecture/iface/offload.md` | Absence means "keep the OS default" | anchored to `internal/component/iface/yang/ze-iface-conf.yang`, `config_apply.go` | "" | silent | This spec does not touch ethtool offloads or RPS/RFS steering; it only adds `link-mtu` inside `router-advertisement` and emits RA ND sysctls when explicitly non-zero. |
| `docs/architecture/iface/vlan-qos-map.md` | Decision: two maps, because the kernel has two | anchored to `internal/component/iface/yang/ze-iface-conf.yang` | "" | silent | This spec does not modify VLAN 802.1p QoS maps or netlink VLAN QoS attributes. |
| `docs/architecture/appliance/kernel-profiles.md` | Go owns the guarantee | anchored to `internal/component/iface/config_apply.go` | "" | silent | This spec does not alter appliance kernel fragments, requirement manifests, or kernel builder behavior; standard Linux IPv6 ND sysctls and ICMPv6 raw sockets require no new kernel config options. |
| `docs/architecture/iface/logical-name-resolution.md` | The config apply path translates, and does not go through the resolver | anchored to `internal/plugins/vrrp/groups.go`, `internal/component/iface/config_apply.go`, `register.go` | "The per-interface sysctl keys and the ethtool offload ioctls take a device string and make no backend call, so a translation that lives in the dispatch layer never touches them." | agrees | `applyRASysctl` runs inside `applyConfigPass` using the already-resolved `osName` (including VLAN subinterface names), and VRRP validation in `groups.go` stays pure with no resolver call. |
| `docs/architecture/plugin/plugin-system.md` | Registration fields | anchored to `internal/component/iface/config_apply.go`, `register.go` | "Each plugin registers exactly one `registry.Registration` from its `init()`." | agrees | `iface` keeps its single `registry.Registration` and adds only a runtime EventBus subscription to `(sysctl, applied)` during startup without altering registration sealing or config roots. |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 (`docs/architecture/plugin/plugin-system.md`) | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | All new config validation (`link-mtu`, `retransmit-timer`, multicast IPv6 VRRP VIP) stays pure inside offline `InProcessConfigVerifier` / `OnConfigVerify` handlers with no daemon dependency. |
| `docs/architecture/traffic/cos-plugin.md` | The interface resolves against the candidate's profiles | anchored to `internal/component/iface/config_apply.go`, `register.go` | "" | silent | This spec does not change `class-of-service` profile resolution, `ConfigReads`, or VLAN QoS map programming in `config_apply.go`. |
| `docs/architecture/config/apply-ordering.md` | The decisions | anchored to `internal/component/iface/register.go` | "Decomposition and constraint rules live in the owning component, never in a central switch." | agrees | `link-mtu` and RA ND sysctls ride the existing `configure-interfaces` residual operation and `applyConfigPass` without altering address-disturbance decomposition or phase ordering. |
| `docs/architecture/config/transaction-protocol.md` | Phase 1: Verify | anchored to `internal/component/iface/register.go` | "Every plugin that owns affected config roots validates the candidate config against its constraints. Verification is non-destructive: no state changes, no side effects." | agrees | `link-mtu` and `retransmit-timer` checks in `iface` and multicast IPv6 VIP checks in `vrrp` run side-effect-free during verify before any apply or sysctl emission. |
| `docs/architecture/l2tp/cpe-1-pppoe-client.md` | IPv6CP | anchored to `internal/component/iface/register.go` | "" | silent | This spec does not touch the PPPoE client dialer, IPv6CP automaton, or PPPoE client reconciliation in `iface`. |
| `docs/architecture/plugin/plugin-relationships.md` | 7. Config Transaction Participation | anchored to `internal/component/iface/register.go` | "A plugin is a participant in a transaction when a root it wants changed." | agrees | `iface` keeps its existing `ConfigRoots`/`WantsConfig` (`interface`) and `ConfigReads` without adding cross-plugin config-root ownership; RA ND sysctls flow over the EventBus to `sysctl`. |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` | Decisions | anchored to `internal/core/sysctl/known_linux.go` | "" | silent | This spec does not touch IPsec/IKEv2 Child SAs, XFRM dataplanes, or IPsec sysctls; it only registers the three IPv6 ND sysctl templates in `known_linux.go`. |
| `docs/architecture/mpls/mpls-kernel.md` | Constraint: the pushed label carries the IP TTL | anchored to `internal/core/sysctl/known_linux.go` | "" | silent | This spec does not touch MPLS forwarding or `net.mpls.ip_ttl_propagate` validation in `known_linux.go`. |
| `docs/architecture/core-design.md` | 19. Component Boundaries | the `// Design:` page of `internal/core/ndp/ra.go`, `internal/core/sysctl/known_linux.go`; anchored to `config_sysctl.go`, `config_apply.go`, `register.go` | "Each component under `internal/component/` is independently removable. Cross-component coupling follows a strict hierarchy:" | agrees | Shared RS validation and RA splitting live in leaf package `internal/core/ndp` and sysctl templates live in `internal/core/sysctl`, preserving dependency direction across `iface`, `iface/ra`, `l2tp/ppp`, and `vrrp`. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (`docs/architecture/core-design.md`) | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | The new `link-mtu` leaf sits inside `container router-advertisement`, which carries `ze:backend "kernel"`, so commits pairing it with `interface backend vpp` remain refused; the registered ND sysctl templates remain standard Linux `sysctl` keys. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (`docs/architecture/core-design.md`) | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | `link-mtu` inherits `router-advertisement`'s `ze:backend "kernel"` annotation and is judged by the existing single backend-gate verdict (as refined at start by the 2026-11-26 decision below). |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (`docs/architecture/core-design.md`) | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This spec does not touch VPP firewall verification or flowtable annotations; all RA and VRRP nodes stay under their existing kernel-only containers. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (`docs/architecture/core-design.md`) | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This spec uses only `kernel` and `vpp` data-plane names and adds no retired selector names. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (`docs/architecture/core-design.md`) | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | This spec does not touch BGP next-hop rewriting or export filter chains; BGP policy behavior is untouched. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (`docs/architecture/core-design.md`) | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This spec does not touch `policychain.Sets` or BGP export chains. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (`docs/architecture/core-design.md`) | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This spec does not touch BGP `policychain.Chain` compilation or `pkg/plugin/rpc.FilterInsertion`. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (`docs/architecture/core-design.md`) | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | This spec does not touch BGP UPDATE forwarding or `wireOverride` next-hop provenance. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (`docs/architecture/core-design.md`) | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This spec does not touch BGP import/export policy chains or `NextHopProfile`. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (`docs/architecture/core-design.md`) | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | This spec does not touch BGP route origination or forwarding chains. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 (`docs/architecture/core-design.md`) | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This spec does not touch BGP filter chains or the implied-filter display seam. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 (`docs/architecture/core-design.md`) | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | An RA unit whose interface has IPv6 forwarding off never prevents `ze start` from running: `reconcileRA` suppresses only that unit's non-zero Router Lifetime sender, logs a WARN, and `ze doctor` reports `doctor-iface-ra-forwarding` with its fix while all other interfaces and services run. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 (`docs/architecture/core-design.md`) | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | `link-mtu` sits inside `router-advertisement` (`ze:backend "kernel"`), so at start under VPP it is excluded with the container and reported under `doctor-dataplane-mismatch`, while commit, reload, and `ze config validate` still refuse it. |
| `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` | Consequences | the `// Design:` page of `internal/plugins/vrrp/*`; a page this spec modifies | "Cross-leaf config rules need sibling context, so they live in the plugin's `InProcessConfigVerifier` and `OnConfigVerify`, not in per-leaf `ze:validate`." | agrees | Refusing multicast IPv6 `virtual-address` entries belongs in `validateGroup` (executed by both `InProcessConfigVerifier` and `OnConfigVerify`), keeping `ze config validate` offline and matching the existing first-VIP link-local rule. |
| `docs/architecture/doctor-and-health-checks.md` | Every finding names the command that fixes it | the `// Design:` page of `internal/plugins/iface/ra/register_doctor_codes.go` | "A doctor diagnostic carries `fix`: the command or config statement the operator runs to clear the problem, alone and with no label, as an operator error holds its fix (`internal/core/opererr`)." | agrees | `doctor-iface-ra-forwarding` stays registered at `DoctorPhasePostConfig` and keeps its explicit `Fix` command (`set ipv6 forwarding true on the unit`). |
| `ai/rules/repo-maintenance.md` | Directives | the `// Design:` page of `internal/plugins/iface/ra/doctor.go` | "A change that adds or changes a surface future agents use, verify, document or avoid MUST update its discovery path in the same work" | agrees | This spec updates `docs/features/interfaces.md`, `docs/architecture/vrrp/vrrp-first-hop-redundancy.md`, `docs/guide/configuration.md`, `docs/config-reference.md`, `docs/plugin-development/metrics.md`, and `rfc/short/rfc4861.md` alongside the code changes. |
| `docs/research/l2tpv2-ze-integration.md` | 9.5 IPv6 Routes | the `// Design:` page of `internal/component/l2tp/ppp/router_solicitation.go` | "Solicitation admission checks the received interface index and Hop Limit before it signals the RA scheduler. The shared body validator checks the type, code, minimum length and bounded option lengths, and rejects a source link-layer option with an unspecified source." | agrees | Delegating `ppp.ValidRouterSolicitation` to `ndp.ValidRouterSolicitation` preserves the exact validation contract while sharing one validator with `internal/plugins/iface/ra`. |
| `docs/config-reference.md` | IPv6 Router Advertisements | a page this spec modifies | "Sending is separate from accepting. Set `forwarding true` on an advertising interface, and leave `accept-ra` at 0 unless the same interface also learns from another router." | agrees | Keeping `forwarding` and `accept-ra` separate remains unchanged; this spec adds `link-mtu` and `ze_iface_ra_rs_discarded_total` to this section and notes forwarding suppression of non-zero Router Lifetime RAs alongside `doctor-iface-ra-forwarding`. |
| `docs/plugin-development/metrics.md` | Full Inventory | a page this spec modifies | "`ze_iface_ra_sent_total` counts every Router Advertisement put on the wire." | agrees | `ze_iface_ra_rs_discarded_total` follows the `ze_iface_ra_<event>_total` naming policy with `{interface, reason}` labels and is registered in the same `iface-ra` `SetMetricsRegistry` hook. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator YANG config: `interface <kind> <name> unit <u> ipv6 router-advertisement` (including new `link-mtu` leaf, `hop-limit`, `reachable-time`, `retransmit-timer`, `router-lifetime`, `prefix`, `rdnss`) and `interface <kind> <name> unit <u> ipv6 vrrp group <name> virtual-address`.
- Wire ingress: ICMPv6 Router Solicitations (type 133) received on an advertising interface raw ICMPv6 socket (`internal/plugins/iface/ra/sender_linux.go`) or a PPP subscriber IPv6 carrier (`internal/component/l2tp/ppp/kernel/ipv6.go`, `internal/component/l2tp/pppoe/dataplane/vpp/ipv6_wire.go`).
- Runtime state transitions: `(sysctl, applied)` EventBus notifications for IPv6 forwarding keys and live Linux procfs forwarding reads.

### Transformation Path
1. **RA config & own ND sysctls**: `parseRAConfig` and `raValidate` parse and bound `link-mtu` (`0 | 1280..65535`) and `retransmit-timer` (`0..2147483647`). During `applyConfigPass`, `applyRASysctl` emits non-zero `hop-limit`, `reachable-time`, and `retransmit-timer` to `sysctl` over the EventBus under source `interface-ra` keyed by the resolved OS device `osName` (clearing stale `interface-ra` defaults when changed or disabled).
2. **RA reconciliation & forwarding gate**: `reconcileRA` builds `RASenderSpec` (including `LinkMTU`). For units with configured `RouterLifetime > 0`, `reconcileRA` checks unit/kernel IPv6 forwarding state: if off, it suppresses starting a sender (logging WARN once) and stops any running sender (whose `Stop()` emits one final Router Lifetime 0 RA and closes the socket, departing `ff02::2`). `(sysctl, applied)` events on forwarding keys wake the RA reconcile worker. Explicit `router-lifetime 0` units remain allowed regardless of forwarding state.
3. **RA send & MTU splitting**: `Sender.run` checks forwarding state before sending any non-zero Router Lifetime RA (defense in depth for out-of-band `/proc/sys` flips: ON->OFF sends one final Router Lifetime 0 RA and suppresses subsequent sends). For each outgoing `ndp.RAConfig`, `Sender.run` computes `effectiveMTU` (resolved link MTU floored at 1280, capped by `cfg.LinkMTU` when non-zero), splits options via `ndp.SplitRA(cfg, effectiveMTU)`, encodes each chunk via `ndp.BuildRA` (including the type 5 MTU option when `LinkMTU > 0`), and writes each chunk with Hop Limit 255.
4. **RS receive & validation**: `openRASocket` enables `ipv6.FlagHopLimit`. `readSolicitations` reads up to 1500 octets with control message and source address, validates via `ndp.ValidateRouterSolicitation`, counts discards by reason in `ze_iface_ra_rs_discarded_total{interface, reason}`, and signals `solicitations` only on valid packets. PPP carriers call the same `ndp.ValidRouterSolicitation` through `ppp.ValidRouterSolicitation`.
5. **VRRP VIP verification & unsolicited NA burst**: `validateGroup` refuses any multicast IPv6 `virtual-address`. `OpenInstance` configures the IPv6 `announcer` with `repeatInterval = max(retrans_time_ms(parent), 1000 ms)` (while IPv4 GARP keeps 100 ms). On Master promotion, `(*announcer).runBurst` sends round-by-round across all VIPs (skipping multicast defensively in `frameBuilder` and `BuildNA`), sleeping `repeatInterval` between rounds with immediate wakeup on `close()`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to `iface` component | config transaction `verifyIfaceConfig` / `OnConfigVerify` / `OnConfigApply` | No |
| `iface` component to `sysctl` component | EventBus `(sysctl, default)`, `(sysctl, clear-source-defaults)`, `(sysctl, applied)` | No |
| `iface` component to `iface-ra` plugin | `RASenderFactory` / `RASender` seam (`RASenderSpec` value type) | No |
| `iface-ra` and `l2tp/ppp` to Linux kernel | raw ICMPv6 sockets with `IPV6_RECVHOPLIMIT` / `IPV6_HOPLIMIT=255`, `/proc/sys/net/ipv6/conf/*` read-only probes | No |
| `vrrp` plugin to `vrrp/transport` | `OpenInstance` / `AnnounceMaster` value types | No |

### Changed-Wire Census

**Search evidence:** Searched `internal/` with `git grep` for `BuildRA`, `RALen`, `OptRDNSS`, `ICMPv6TypeRouterSolicitation`, `ICMPv6TypeRouterAdvertisement`, `readSolicitations`, `ValidRouterSolicitation`, `ReadRS`, `BuildNA`, `newAnnouncer`, and `runBurst`. Positive controls confirmed: `internal/core/ndp/ra.go::BuildRA` (2 callers: `internal/plugins/iface/ra/sender_linux.go` and `internal/component/l2tp/ppp/ra.go` -> `ra_send.go`), `internal/component/l2tp/ppp/router_solicitation.go::ValidRouterSolicitation` (2 carrier callers: `internal/component/l2tp/ppp/kernel/ipv6.go::ReadRS` and `internal/component/l2tp/pppoe/dataplane/vpp/ipv6_wire.go::dispatchIPv6`), `internal/plugins/iface/ra/sender_linux.go::readSolicitations` (LAN RS reader), and `internal/plugins/vrrp/transport/na.go::BuildNA` (1 production caller: `internal/plugins/vrrp/transport/transport.go::frameBuilder` -> `announce.go::runBurst`).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| ICMPv6 Router Advertisement (type 134) | writer | `internal/core/ndp/ra.go::BuildRA`, `SplitRA`; `internal/plugins/iface/ra/sender_linux.go::(*Sender).run`, `sendFinal`; `internal/component/l2tp/ppp/ra_send.go::(*raSender).send` | `BuildRA` encodes all Ze RAs. `iface/ra` sends configured/delegated options; `l2tp/ppp` sends a fixed 16-octet header (`56` octets with IPv6 header, always below 1280). | change: add RFC 4861 §4.6.4 MTU option (type 5) when `LinkMTU > 0`, split options across RAs in `SplitRA` so `40 + RALen <= effectiveMTU`, and emit Router Lifetime 0 on forwarding loss; preserve byte-for-byte output when `LinkMTU == 0`, options fit MTU, and forwarding is on. |
| ICMPv6 Router Advertisement (type 134) | reader | IPv6 hosts and routers on the link (outside repository, including Linux kernel `net/ipv6/ndisc.c::ndisc_router_discovery`); Ze test capture `internal/plugins/iface/ra/ra_integration_linux_test.go::parseRA` | RFC 4861 §6.3.4 hosts accept the union of split RAs and consume type 5 MTU option (`>= 1280`); `parseRA` in tests parses RA options. | change test reader `parseRA` to decode MTU option (type 5) for wire proof; standard RFC 4861 wire format preserved for all outside hosts. |
| ICMPv6 Router Solicitation (type 133) | writer | IPv6 hosts on LAN and PPP links (outside repository, Linux `net/ipv6/ndisc.c::ndisc_send_rs`); test writers `internal/plugins/iface/ra/ra_integration_linux_test.go::sendRouterSolicitation` and `rfc4861_solicitation_reader_linux_test.go` | Hosts and test helpers transmit ICMPv6 type 133 solicitations with Hop Limit 255. | preserve standard RFC 4861 §4.1 format; update unprivileged loopback test sender in `internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go` to set Hop Limit 255 so it passes strict §6.1.1 validation. |
| ICMPv6 Router Solicitation (type 133) | reader | `internal/plugins/iface/ra/sender_linux.go::readSolicitations`; `internal/component/l2tp/ppp/kernel/ipv6.go::(*ipv6Carrier).ReadRS`; `internal/component/l2tp/pppoe/dataplane/vpp/ipv6_wire.go::(*controlSocket).dispatchIPv6` | `kernel/ipv6.go` and `vpp/ipv6_wire.go` already validate via `ValidRouterSolicitation`; `sender_linux.go::readSolicitations` currently skips all checks. | change `sender_linux.go` to enable `FlagHopLimit`, read up to 1500 octets, validate via shared `ndp.ValidateRouterSolicitation`, and count discards; preserve valid solicitation admission on all three readers. |
| ICMPv6 Unsolicited Neighbor Advertisement (type 136, VRRP) | writer | `internal/plugins/vrrp/transport/na.go::BuildNA`; `internal/plugins/vrrp/transport/transport.go::(*instance).frameBuilder`; `internal/plugins/vrrp/transport/announce.go::(*announcer).runBurst` | `frameBuilder` calls `BuildNA` for each IPv6 VIP; `runBurst` repeats 3 times at 100 ms. | change: refuse multicast Target Address in `BuildNA`/`frameBuilder` and space IPv6 NA repeats by `>= RetransTimer` (at least 1,000 ms), round-by-round; preserve 32-octet RFC 4861/9568 wire layout for valid unicast VIPs. |
| ICMPv6 Unsolicited Neighbor Advertisement (type 136, VRRP) | reader | IPv6 neighbors on the VRRP link (outside repository, Linux `net/ipv6/ndisc.c::ndisc_recv_na`); Ze tests `internal/plugins/vrrp/transport/rfc_announce_verdict_test.go` and `rfc4861_unsolicited_na_test.go` | Neighbors discard multicast-target NAs per RFC 4861 §7.1.2 and process valid unicast unsolicited NAs with R=1, S=0, O=1. | preserve standard RFC 4861 §4.4 / §7.2.6 wire contract for all external neighbors and existing verdict tests. |

### Integration Points
- `internal/core/ndp/ra.go` and `rs.go` - shared ND encoder, splitter, and RS validator imported by `internal/plugins/iface/ra`, `internal/component/iface`, and `internal/component/l2tp/ppp`.
- `internal/component/iface/config_ra.go`, `config_sysctl.go`, `config_apply.go`, `reconcile_ra.go`, `register.go` - config parsing, ND sysctl emission, forwarding-aware RA reconciliation, and `(sysctl, applied)` subscription.
- `internal/core/sysctl/known_linux.go` - per-interface IPv6 ND sysctl template registration.
- `internal/plugins/iface/ra/sender_linux.go`, `ifacera.go`, `doctor.go`, `register_doctor_codes.go` - RS validation, discard metrics, forwarding defense-in-depth, MTU-bounded RA transmission, and doctor warning text.
- `internal/plugins/vrrp/groups.go`, `transport/na.go`, `transport/announce.go`, `transport/transport.go` - multicast VIP rejection and `RetransTimer`-spaced IPv6 unsolicited NA bursts.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Config -> YANG -> `config_ra.go` -> `config_sysctl.go` (EventBus to `sysctl`) and `reconcile_ra.go` (`RASenderSpec` to `iface-ra`) -> `ndp.SplitRA`/`BuildRA`; VRRP config -> `groups.go::validateGroup` -> `transport.go`/`announce.go`/`na.go`. |
| No unintended coupling (components stay isolated) | Yes | Shared ND logic lives in leaf `internal/core/ndp`; `iface` still talks to `iface-ra` only through `RASender`/`RASenderSpec` and to `sysctl` only through `internal/component/sysctl/events`; `vrrp` adds no new cross-component imports. |
| No duplicated functionality (extends existing, does not recreate) | Yes | Promotes existing `ppp.ValidRouterSolicitation` to `internal/core/ndp` for all three RS readers; reuses existing `Sender.Stop()`/`sendFinal` zero-lifetime RA path and `doctor_linux.go` forwarding probe semantics. |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `ndp.BuildRA` and `vrrp/transport.BuildNA` write directly into caller-owned buffers; `readSolicitations` reuses one fixed 1500-octet buffer per goroutine. |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | New sysctl templates register via `sysctl.MustRegister` in `internal/core/sysctl/known_linux.go`; new RS discard counter registers in `ifacera.SetMetricsRegistry`; no central switch or factory edited. |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `internal/component/iface/` and `internal/core/sysctl/`: `link-mtu` completion/help derives automatically from `ze-iface-conf.yang`; sysctl completion/describe derives automatically from `sysctl.MustRegister`; no external table enumerates RA leaves or ND sysctl templates. |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Linux raw `IPPROTO_ICMPV6` sockets verify the ICMPv6 checksum in the kernel (`rawv6_init_sk` / `rawv6_rcv_skb` in `net/ipv6/raw.c`) before delivering packets to `readSolicitations` | `internal/component/l2tp/ppp/kernel/ipv6_linux.go` lines 5-8 and `docs/research/l2tpv2-ze-integration.md` lines 946-949 | A bad-checksum RS could reach `readSolicitations` if kernel behavior differed | QEMU Linux integration test sending a bad-checksum RS (or raw packet capture verification) | unvalidated |
| A-2 | Linux `net.ipv6.neigh.<dev>.base_reachable_time_ms` drives the kernel's periodic `ReachableTime` randomization (`neigh_rand_reach_time`) as RFC 4861 §6.2.1 / §6.3.2 require | Linux kernel `net/core/neighbour.c` (`neigh_rand_reach_time`) | Router's reachable time would stay fixed instead of randomized | QEMU Linux integration test reading `/proc/sys/net/ipv6/neigh/<dev>/base_reachable_time_ms`, `retrans_time_ms`, and `hop_limit` after RA config apply | unvalidated |
| A-3 | Existing `TestRASolicitedResponse` (`RFC4861-6.2.2-2`) still proves the socket's own `ff02::2` multicast join when configured with explicit `RouterLifetime: 0` while IPv6 forwarding is off | `internal/plugins/iface/ra/rfc4861_ra_integration_linux_test.go` lines 129-181 and Thomas's 2026-11-24 ruling ("suppress non-zero Router Lifetime RAs when IPv6 forwarding is off") | `RFC4861-6.2.2-2` test would fail under forwarding-off suppression | Re-running `TestRASolicitedResponse` and `./le rfc record` after the change | unvalidated |
| A-4 | Changing VRRP IPv6 `announcer.runBurst` to round-by-round with `>= 1000 ms` repeat spacing keeps `TestAnnounceMasterFramesPerAddress` (`rfc_announce_verdict_test.go`, 3 s deadline for 2 IPv6 VIPs) green (~2 s total) | `internal/plugins/vrrp/transport/rfc_announce_verdict_test.go` lines 24-33 | Test would time out if 2 VIPs ran sequentially (4 s > 3 s) | Running `go test -race ./internal/plugins/vrrp/...` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Startup/reload ordering race: `reconcileRA` runs before `sysctl` finishes applying `net.ipv6.conf.<dev>.forwarding = 1` (e.g., when `sysctl` runs as an external plugin or applies its own `sysctl` block after `iface`) | RA sender initially suppressed at boot even though forwarding is configured | Subscribe `iface` to `(sysctl, applied)` for `net.ipv6.conf.*.forwarding` and `force_forwarding` keys and wake the RA reconcile worker so the sender starts immediately once `sysctl` applies the key |
| R-2 | Existing `internal/component/iface` unit tests (`reconcile_ra_test.go`, `rfc4861_router_configuration_test.go`) test `reconcileRA` on synthetic interface names (`eth0`) without procfs entries or explicit `ipv6 forwarding true` | Unit test regressions in `internal/component/iface` | When `u.IPv6.Forwarding` is explicitly `false` or `raForwardingReader` returns `(false, true)`, `reconcileRA` suppresses immediately; when `u.IPv6.Forwarding == nil` and `raForwardingReader` returns `known == false`, `reconcileRA` defers to `Sender`, which fails closed on live Linux devices if procfs is unreadable |
| R-3 | Closing a VRRP IPv6 instance during a 1 s inter-repeat wait stalls shutdown for up to 1 s per instance | Slow VRRP teardown or test timeout in `CloseInstance` | Use a `time.NewTimer` selected against `<-a.stop` inside `announcer` so `close()` wakes immediately |
| R-4 | A huge prefix list produces many split RA packets in one burst | High packet count on pathological config | `SplitRA` guarantees every chunk stays inside `effectiveMTU` and preserves every option with no silent drop; single-packet fast path is untouched when `RALen(cfg) <= effectiveMTU - 40` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | IPv6 hosts on LAN links fail to receive RAs (if forwarding detection false-negatives), keep stale default routes (if final Router Lifetime 0 RA fails on forwarding loss), receive oversized RAs (if `SplitRA` miscomputes header size), or VRRP IPv6 failover announcements stall/break |
| How is it reverted? | Single commit revert; `link-mtu` defaults to 0 so existing configs remain valid either way |
| Who else touches this path? | `plan/post-release/features/spec-ra-vrrp-virtual-router.md` (skeleton, touches `sender_linux.go` and `reconcile_ra.go`); `plan/post-release/features/spec-rfc4861-nd-proxy-and-ipsec-nd.md` (split sibling, touches `ze-iface-conf.yang` and `rfc/short/rfc4861.md`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Router Solicitation on a Linux veth advertising interface | → | `openRASocket` (`FlagHopLimit`), `readSolicitations`, `ndp.ValidateRouterSolicitation` | `TestRFC4861SolicitationReaderDiscardsInvalid` (`internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go`) |
| Router Solicitation on a PPP subscriber carrier | → | `(*ipv6Carrier).ReadRS`, `ppp.ValidRouterSolicitation`, `ndp.ValidateRouterSolicitation` | `TestRFC4861SubscriberSolicitationReaderDiscardsInvalid` (`internal/component/l2tp/ppp/kernel/rfc4861_solicitation_reader_test.go`) |
| `router-advertisement { link-mtu 1400; }` config through `parseRAConfig` and `reconcileRA` to wire RA | → | `parseRAConfig`, `raSpecFor`, `ndp.BuildRA` MTU option | `TestRFC4861AdvLinkMTUConfigured` (`internal/component/iface/rfc4861_router_configuration_test.go`) and `TestRFC4861WireAdvLinkMTUOption` (`internal/plugins/iface/ra/rfc4861_ra_integration_linux_test.go`) |
| `router-advertisement { hop-limit; reachable-time; retransmit-timer; }` through `applyConfigPass` | → | `applyRASysctl` EventBus emissions to `sysctl` | `TestRFC4861RouterUsesItsAdvertisedVariables` (`internal/component/iface/rfc4861_router_configuration_test.go`) |
| `router-advertisement` with IPv6 forwarding off, then on, then off | → | `reconcileRA`, `(sysctl, applied)` trigger, `Sender.run`, `Sender.sendFinal` | `TestRFC4861AdvertisingFollowsForwarding` (`internal/component/iface/rfc4861_router_configuration_test.go`) and `TestRFC4861ForwardingLossSendsFinalZeroLifetimeRA` (`internal/plugins/iface/ra/rfc4861_ra_integration_linux_test.go`) |
| `router-advertisement` with 50 prefixes on a 1500-octet link | → | `ndp.SplitRA`, `Sender.run` multi-chunk send | `TestRFC4861AdvertisementInsideLinkMTU` (`internal/core/ndp/rfc4861_router_advertisement_test.go`) and `TestRFC4861WireRASplitsToFitLinkMTU` (`internal/plugins/iface/ra/rfc4861_ra_integration_linux_test.go`) |
| VRRP IPv6 group with multicast vs unicast `virtual-address` | → | `validateGroup`, `frameBuilder`, `BuildNA` | `TestRFC4861VRRPRefusesMulticastTarget` (`internal/plugins/vrrp/rfc4861_virtual_address_test.go`) |
| VRRP IPv6 Master promotion via `Transport.AnnounceMaster` | → | `OpenInstance`, `(*announcer).runBurst` | `TestRFC4861UnsolicitedNASpacedByRetransTimer` (`internal/plugins/vrrp/transport/rfc4861_unsolicited_na_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Valid Router Solicitation (Hop Limit 255, code 0, length `>= 8`, all option lengths `> 0`, no SLLA from `::`) vs each invalid solicitation (Hop Limit 254, code 1, length 7, option length 0, truncated option, SLLA from `::`) on both LAN `readSolicitations` and PPP `(*ipv6Carrier).ReadRS` | Valid solicitations are reported and trigger an RA (including non-zero Reserved and unrecognized options); every invalid solicitation is silently discarded without triggering an RA and increments `ze_iface_ra_rs_discarded_total{interface, reason}` on the LAN reader (`RFC4861-6.1.1-2`, `RFC4861-4.6-1`) | none |
| AC-2 | VRRP IPv6 group configured with a multicast `virtual-address` (e.g., `ff02::1` or `ff0e::1` after the link-local first address) vs all-unicast IPv6 virtual addresses | Config verification (`validateGroup`) refuses the multicast virtual address with an operator error naming RFC 4861 Section 4.4 and the fix; `BuildNA` returns 0 and `frameBuilder` emits nothing for a multicast target; all-unicast IPv6 groups verify and emit valid 32-octet NAs (`RFC4861-4.4-3`) | none |
| AC-3 | VRRP IPv6 Master promotion with 1 or more IPv6 VIPs, both with default link `RetransTimer` (1,000 ms) and with a higher interface `retrans_time_ms` (e.g., 2,000 ms), plus IPv4 VRRP Master promotion | Every IPv6 VIP receives its first unsolicited NA immediately in round 0, and each of the 3 repeat rounds is separated by at least `max(interface retrans_time_ms, 1000 ms)`; `close()` interrupts any active sleep immediately; IPv4 GARP repeats remain 3 frames 100 ms apart (`RFC4861-7.2.6-1`) | IPv4 GARP retains 100 ms spacing because RFC 4861 Section 7.2.6 governs IPv6 Neighbor Discovery only |
| AC-4 | `router-advertisement` configured with `link-mtu 1400`, with `link-mtu 0` (or omitted), and with out-of-range values `1279` or `65536` | `link-mtu 1400` commits, updates a running sender in place, and puts an RFC 4861 §4.6.4 MTU option (type 5, length 1, reserved 0, MTU 1400) on the wire; `0` or omitted emits no MTU option; `1279` and `65536` are refused at schema and `raValidate` (`RFC4861-6.2.1-1`) | none |
| AC-5 | Enabled `router-advertisement` unit with `hop-limit 128`, `reachable-time 45000`, `retransmit-timer 2000`, then updated to `reachable-time 0` or disabled | `applyRASysctl` emits `net.ipv6.conf.<dev>.hop_limit=128`, `net.ipv6.neigh.<dev>.base_reachable_time_ms=45000`, and `net.ipv6.neigh.<dev>.retrans_time_ms=2000` over EventBus under source `interface-ra` (overridable by explicit `sysctl` config); setting a leaf to `0` or disabling RA clears stale `interface-ra` defaults so `sysctl` restores the original kernel value (`RFC4861-6.2.1-8`) | Explicit operator `sysctl { setting ... }` overrides `interface-ra` defaults because Ze's `sysctl` precedence is `config > transient > default`; leaves set to `0` ("unspecified") do not overwrite the kernel default |
| AC-6 | Unit with `router-advertisement enabled true` and non-zero `router-lifetime` (default 1800) when IPv6 forwarding is off, when forwarding turns on, and when forwarding turns off again (via config reload, `sysctl` EventBus, or direct procfs change), plus explicit `router-lifetime 0` with forwarding off | With forwarding off and `router-lifetime > 0`, no non-zero Router Lifetime RA is sent, `reconcileRA` suppresses the sender and logs WARN, and `ze doctor` reports `doctor-iface-ra-forwarding`; when forwarding turns on, advertising starts with the configured Router Lifetime; when forwarding turns off, one final RA with Router Lifetime 0 leaves immediately (within `ndp.CeaseWait`) and further non-zero Router Lifetime RAs are suppressed; explicit `router-lifetime 0` still advertises prefixes/RDNSS with Router Lifetime 0 (`RFC4861-6.2.4-1`, `RFC4861-6.2.5-2`, Thomas's 2026-11-24 D-6 reversal) | Explicit `router-lifetime 0` continues to advertise with Router Lifetime 0 while forwarding is off, as explicitly preserved by Thomas's ruling ("suppress non-zero Router Lifetime RAs when IPv6 forwarding is off") and RFC 4861 Section 6.2.5 |
| AC-7 | `router-advertisement` unit carrying 50 prefixes (plus SLLA, MTU, routes, and RDNSS) on a 1500-octet link, and on a 1280-octet link, vs a 1-prefix unit | `ndp.SplitRA` and `Sender.run` transmit multiple valid RAs whose union contains every prefix, route, and RDNSS address in order and where every individual packet satisfies `40 + len(icmpv6Payload) <= effectiveMTU`; a 1-prefix advertisement produces a single unchanged RA (`RFC4861-9-7`) | none |
| AC-8 | All nine RFC 4861 MUST gaps in `RFC gaps` | Each gap row in `rfc/short/rfc4861.md` has its `{gap}` removed, carries positive and negative RFC-tagged tests with discrimination records produced by `./le rfc record`, and passes `./le rfc check` | The four unstarted ND Proxy and AH/ESP-secured ND gaps (`RFC4861-7.2.8-1`, `7.2.8-2`, `11.2-1`, `11.2-2`) remain `{gap}` and are owned by `plan/post-release/features/spec-rfc4861-nd-proxy-and-ipsec-nd.md` per Thomas's 2026-11-24 ruling |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `link-mtu 1400`, `hop-limit 128`, `reachable-time 45000`, `retransmit-timer 2000` on an advertising interface | YANG -> `parseRAConfig` -> `applyRASysctl` (EventBus -> `sysctl`) + `reconcileRA` -> `Sender.run` -> `ndp.BuildRA` -> wire RA with MTU option and own interface sysctls updated | `TestRFC4861WireAdvLinkMTUOption`, `TestRFC4861RouterUsesItsAdvertisedVariables`, `iface-ra-link-mtu-and-forwarding.ci` |
| 2 | Enables `router-advertisement` without `ipv6 forwarding true`, then enables forwarding, then disables forwarding | `reconcileRA` suppresses sender + `ze doctor` warns -> forwarding on starts sender -> forwarding off stops sender and emits final Router Lifetime 0 RA | `TestRFC4861AdvertisingFollowsForwarding`, `TestRFC4861ForwardingLossSendsFinalZeroLifetimeRA`, `iface-ra-link-mtu-and-forwarding.ci` |
| 3 | Advertises 50 prefixes on a 1500-octet LAN interface | `parseRAConfig` -> `reconcileRA` -> `Sender.run` -> `ndp.SplitRA` -> multiple RAs all `<= 1500` octets received on host | `TestRFC4861AdvertisementInsideLinkMTU`, `TestRFC4861WireRASplitsToFitLinkMTU` |
| 4 | Sends malformed and valid Router Solicitations to a Ze router (LAN or PPP subscriber) | Raw ICMPv6 socket (`FlagHopLimit`) -> `readSolicitations` / `(*ipv6Carrier).ReadRS` -> `ndp.ValidateRouterSolicitation` -> invalid silently discarded and counted, valid answered | `TestRFC4861SolicitationReaderDiscardsInvalid`, `TestRFC4861SubscriberSolicitationReaderDiscardsInvalid` |
| 5 | Configures a VRRP IPv6 group (tries multicast VIP, then valid unicast VIPs and promotes to Master) | `validateGroup` refuses multicast VIP; valid unicast group promotes -> `AnnounceMaster` -> `(*announcer).runBurst` sends 3 NA rounds spaced by `>= RetransTimer` | `TestRFC4861VRRPRefusesMulticastTarget`, `TestRFC4861UnsolicitedNASpacedByRetransTimer` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC4861ValidateRouterSolicitation` | `internal/core/ndp/rfc4861_router_solicitation_test.go` | `RFC4861-6.1.1-2`, `RFC4861-4.6-1`: rejects Hop Limit != 255, non-IPv6/multicast source, length < 8, type != 133, code != 0, option length 0, truncated option, SLLA from `::`; accepts valid RS with Reserved/unknown options | |
| `TestRFC4861SolicitationReaderDiscardsInvalid` | `internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go` | `RFC4861-6.1.1-2`, `RFC4861-4.6-1`: `readSolicitations` silently discards all invalid RS cases (counting `ze_iface_ra_rs_discarded_total{interface, reason}`) and reports the following valid RS | |
| `TestRFC4861SubscriberSolicitationReaderDiscardsInvalid` | `internal/component/l2tp/ppp/kernel/rfc4861_solicitation_reader_test.go` | `RFC4861-6.1.1-2`, `RFC4861-4.6-1`: `(*ipv6Carrier).ReadRS` and `rsReaderLoop` discard all five invalid RS cases and admit the following valid RS | |
| `TestRFC4861VRRPRefusesMulticastTarget` | `internal/plugins/vrrp/rfc4861_virtual_address_test.go` | `RFC4861-4.4-3`: `validateGroup` refuses multicast IPv6 `virtual-address` in any position and accepts unicast IPv6 virtual addresses; `BuildNA` returns 0 for multicast and 32 for unicast | |
| `TestRFC4861UnsolicitedNASpacedByRetransTimer` | `internal/plugins/vrrp/transport/rfc4861_unsolicited_na_test.go` | `RFC4861-7.2.6-1`: IPv6 NA repeats are spaced `>= 1000 ms` (and follow higher `retrans_time_ms`), round-by-round across VIPs, interruptible on `close()`; IPv4 GARP remains 100 ms | |
| `TestRFC4861AdvLinkMTUConfigured` | `internal/component/iface/rfc4861_router_configuration_test.go` | `RFC4861-6.2.1-1`: `link-mtu` bounds (`0`, `1280`, `65535` accepted; `1`, `1279`, `65536` refused), reaches `RASenderSpec`, and updates running sender | |
| `TestRFC4861MTUOptionEncoded` | `internal/core/ndp/rfc4861_router_advertisement_test.go` | `RFC4861-6.2.1-1`: `BuildRA` emits type 5, len 1, reserved 0 (over 0xff dirty buffer), big-endian MTU when `LinkMTU > 0`, and omits it when `LinkMTU == 0` | |
| `TestRFC4861RouterUsesItsAdvertisedVariables` | `internal/component/iface/rfc4861_router_configuration_test.go` | `RFC4861-6.2.1-8`: `applyRASysctl` emits `hop_limit`, `base_reachable_time_ms`, and `retrans_time_ms` under `interface-ra` for non-zero values, skips 0, and clears on disable | |
| `TestRFC4861AdvertisingFollowsForwarding` | `internal/component/iface/rfc4861_router_configuration_test.go` | `RFC4861-6.2.4-1`, `RFC4861-6.2.5-2`: `reconcileRA` suppresses non-zero Router Lifetime sender when forwarding is off, starts when on, stops (triggering final RA) when turned off, and keeps `router-lifetime 0` active | |
| `TestRFC4861AdvertisementInsideLinkMTU` | `internal/core/ndp/rfc4861_router_advertisement_test.go` | `RFC4861-9-7`: `SplitRA` splits 50 prefixes + routes + RDNSS for 1500-octet and 1280-octet MTUs so every chunk obeys `40 + RALen(chunk) <= effectiveMTU` with full option union preserved; small configs stay 1 chunk | |
| `TestRFC4861WireAdvLinkMTUOption`, `TestRFC4861ForwardingLossSendsFinalZeroLifetimeRA`, `TestRFC4861WireRASplitsToFitLinkMTU` | `internal/plugins/iface/ra/rfc4861_ra_integration_linux_test.go` | Linux userns/veth wire proofs: MTU option on wire, forwarding off suppression + ON->OFF final Router Lifetime 0 RA on wire, and 50-prefix split RAs all `<= 1500` octets on wire | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `router-advertisement link-mtu` | `0 \| 1280..65535` octets | `0`, `1280`, `65535` | `1`..`1279` (`1279`) | `65536` |
| `router-advertisement retransmit-timer` | `0..2147483647` ms | `0`, `1`, `2147483647` | N/A (`uint32`) | `2147483648` |
| RS ICMPv6 length | `>= 8` octets | `8` | `7` | N/A |
| RS Hop Limit | `255` | `255` | `254` | `256` (impossible in uint8) |
| RS option length octet | `>= 1` (8-octet units) within packet | `1` (8 octets) | `0` | exceeds remaining bytes |
| `ndp.SplitRA` `linkMTU` | `>= 1280` (clamped if `< 1280`) | `1280`, `1500` | `0`, `1279` (clamped to `1280`) | N/A |
| VRRP IPv6 NA repeat spacing | `>= 1000` ms | `1000` ms (`ndp.RetransTimer`) | `999` ms (refused by floor) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-ra-link-mtu-and-forwarding` | `test/plugin/iface-ra-link-mtu-and-forwarding.ci` | Operator configures `link-mtu 1400` (accepted), `link-mtu 1279` (refused), `retransmit-timer 2147483648` (refused), and RA with `ipv6 forwarding false` (shows `doctor-iface-ra-forwarding` warning while daemon runs cleanly) | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A (Linux kernel host peer in QEMU userns integration suite `internal/plugins/iface/ra/rfc4861_ra_integration_linux_test.go` and existing `vrrp-keepalived` QEMU lab) | `internal/plugins/iface/ra/` | Linux kernel IPv6 ND stack (`net/ipv6/ndisc.c`) | The peer for RA origination, RS answering, MTU option adoption, Router Lifetime 0 withdrawal, split RAs, and VRRP unsolicited NAs is an IPv6 host/neighbor kernel stack, directly exercised over real veth pairs in Linux namespaces; no separate routing daemon peer is needed beyond the kernel host and existing `vrrp-keepalived` lab | |

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `counter-counts-the-wrong-packets.md` | 2026-10-26 | `readSolicitations` (`iface/ra/sender_linux.go`), `rsReaderLoop` (`l2tp/ppp/ra_linux.go`) |
| `unwired-feature.md` | 2026-10-26 | iface RA sender (`iface/config_ra.go`, `reconcile_ra.go`, `iface/ra/sender_linux.go`) |
| `unwired-feature.md` | 2026-10-26 | VRRP unsolicited NA (`internal/plugins/vrrp/transport/announce.go` `runBurst`, `internal/plugins/vrrp/groups.go`) |

Note: the 2026-10-26 `rfc-extension-not-implemented.md` row (`IPv6 Neighbor Discovery (iface RA, kernel ND)`) moved to `plan/post-release/features/spec-rfc4861-nd-proxy-and-ipsec-nd.md` per Thomas's 2026-11-24 ruling.

## Files to Modify

- `internal/core/ndp/ra.go` - add `OptMTU`, `IPv6HeaderLen`, `MinIPv6LinkMTU`, `RetransTimer`, `RAConfig.LinkMTU`, MTU option in `RALen`/`BuildRA`, and `SplitRA`
- `internal/plugins/iface/ra/sender_linux.go` - enable `FlagHopLimit` in `openRASocket`, raise read buffer to 1500, validate RS via `ndp.ValidateRouterSolicitation` in `readSolicitations`, enforce forwarding suppression and ON->OFF final Router Lifetime 0 RA in `Sender.run`, and transmit `ndp.SplitRA` chunks within link MTU
- `internal/plugins/iface/ra/ifacera.go` - add `ze_iface_ra_rs_discarded_total{interface, reason}` counter
- `internal/plugins/iface/ra/doctor.go` - keep `doctor-iface-ra-forwarding` warning, update comment and message wording for D-6 reversal
- `internal/plugins/iface/ra/register_doctor_codes.go` - update `doctor-iface-ra-forwarding` description for D-6 reversal
- `internal/plugins/iface/ra/rfc4861_solicitation_reader_linux_test.go` - set Hop Limit 255 on loopback test sender and add `TestRFC4861SolicitationReaderDiscardsInvalid`
- `internal/plugins/iface/ra/rfc4861_ra_integration_linux_test.go` - enable forwarding in router-mode tests, set `RouterLifetime: 0` in `TestRASolicitedResponse`, add wire tests for MTU option, forwarding suppression/loss final RA, and MTU splitting
- `internal/component/iface/yang/ze-iface-conf.yang` - add `link-mtu` leaf (`0 | 1280..65535`, default `0`) and bound `retransmit-timer` (`0..2147483647`) under `container router-advertisement`
- `internal/component/iface/config_ra.go` - parse and validate `link-mtu` and `retransmit-timer` ceiling
- `internal/component/iface/config_sysctl.go` - add `applyRASysctl` emitting `hop_limit`, `base_reachable_time_ms`, and `retrans_time_ms` under source `interface-ra` and clearing stale defaults
- `internal/component/iface/config_apply.go` - call `applyRASysctl` in Phase 2/2b sysctl apply
- `internal/component/iface/reconcile_ra.go` - propagate/compare `LinkMTU`, enforce forwarding gate on non-zero Router Lifetime units
- `internal/component/iface/register.go` - subscribe to `(sysctl, applied)` for IPv6 forwarding keys to wake RA reconciliation
- `internal/component/iface/rfc4861_router_configuration_test.go` - add `TestRFC4861AdvLinkMTUConfigured`, `TestRFC4861RouterUsesItsAdvertisedVariables`, `TestRFC4861AdvertisingFollowsForwarding`
- `internal/core/sysctl/known_linux.go` - register `net.ipv6.conf.<iface>.hop_limit`, `net.ipv6.neigh.<iface>.base_reachable_time_ms`, and `net.ipv6.neigh.<iface>.retrans_time_ms`
- `internal/core/ndp/rfc4861_router_advertisement_test.go` - add `TestRFC4861MTUOptionEncoded` and `TestRFC4861AdvertisementInsideLinkMTU`
- `internal/component/l2tp/ppp/router_solicitation.go` - delegate `ValidRouterSolicitation` to `ndp.ValidRouterSolicitation`
- `internal/plugins/vrrp/groups.go` - refuse multicast IPv6 `virtual-address` in `validateGroup`
- `internal/plugins/vrrp/transport/na.go` - return 0 for multicast Target Address in `BuildNA`
- `internal/plugins/vrrp/transport/announce.go` - family-specific repeat interval, round-by-round burst across VIPs, stop-interruptible sleep
- `internal/plugins/vrrp/transport/transport.go` - pass `RetransTimer`-floored interval for IPv6 announcer and reject multicast VIP in `frameBuilder`
- `internal/plugins/vrrp/transport/rfc4861_unsolicited_na_test.go` - add `TestRFC4861UnsolicitedNASpacedByRetransTimer`
- `rfc/short/rfc4861.md` - remove `{gap}` on the nine closed rows and update stale `ra_linux.go` pointers on those rows
- `docs/features/interfaces.md` - document D-6 reversal, `link-mtu`, router own ND sysctls, link-MTU RA splitting, and `ze_iface_ra_rs_discarded_total`
- `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` - document multicast IPv6 VIP refusal and `>= RetransTimer` unsolicited NA repeat spacing
- `docs/guide/configuration.md` - document `link-mtu` under `router-advertisement`
- `docs/config-reference.md` - document `link-mtu` leaf and `ze_iface_ra_rs_discarded_total` counter under `router-advertisement`
- `docs/plugin-development/metrics.md` - add `ze_iface_ra_rs_discarded_total` to the `iface-ra` metrics table and description

## Files to Create

- `internal/core/ndp/rs.go` - shared `ValidateRouterSolicitation` and `ValidRouterSolicitation`
- `internal/core/ndp/rfc4861_router_solicitation_test.go` - RFC-tagged RS validity tests
- `internal/component/iface/forwarding_linux.go` and `internal/component/iface/forwarding_other.go` - per-device IPv6 forwarding probe for `reconcileRA`
- `internal/component/l2tp/ppp/kernel/rfc4861_solicitation_reader_test.go` - RFC-tagged subscriber RS validity tests
- `internal/plugins/vrrp/rfc4861_virtual_address_test.go` - RFC-tagged VRRP multicast VIP refusal tests
- `internal/plugins/vrrp/transport/retrans_linux.go` and `internal/plugins/vrrp/transport/retrans_other.go` - interface `retrans_time_ms` reader for VRRP IPv6 NA spacing
- `test/plugin/iface-ra-link-mtu-and-forwarding.ci` - functional `.ci` test for `link-mtu` validation and forwarding-off RA doctor warning

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/iface/yang/ze-iface-conf.yang`: add `link-mtu` leaf under `router-advertisement` |
| YANG validation constraints | Yes | `link-mtu` carries native `uint32` range `0 \| 1280..65535`; `retransmit-timer` carries `0..2147483647` |
| YANG custom validators | No | Native YANG range constraints and existing `parseRAConfig` / `validateGroup` in-process verifiers cover all rules |
| CLI commands/flags | No | No new CLI commands or flags added |
| CLI grammar (keyword before value) | N-A | No new CLI commands |
| Editor autocomplete | Yes | Automatic from `ze-iface-conf.yang` for `link-mtu` and from `internal/core/sysctl/known_linux.go` for the three ND sysctl templates |
| Functional test for new RPC/API | Yes | `test/plugin/iface-ra-link-mtu-and-forwarding.ci` covers the new `link-mtu` config leaf and forwarding doctor warning |
| Pipe completeness | N-A | No new show or operational command output |
| Env var registration | No | No new `environment/` leaves or env vars |
| Doctor check for runtime dependencies | Yes | Existing `doctor-iface-ra-forwarding` in `internal/plugins/iface/ra/doctor.go` and `register_doctor_codes.go` kept and updated for D-6 reversal |
| Doctor code per problem class | Yes | `doctor-iface-ra-forwarding` covers RA configured with forwarding off at `DoctorPhasePostConfig` with fix `set ipv6 forwarding true on the unit` |
| Prometheus counters/metrics | Yes | `ze_iface_ra_rs_discarded_total{interface, reason}` registered in `internal/plugins/iface/ra/ifacera.go` |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not a BGP family change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Existing RA and VRRP features keep their scope; RFC 4861 conformance rows update in row 9 |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` (add `link-mtu` leaf under `router-advertisement`) |
| 3 | CLI command added/changed? | No | No CLI command added or changed |
| 4 | API/RPC added/changed? | No | No RPC or API endpoint added or changed |
| 5 | Plugin added/changed? | No | No new plugin; `iface-ra` and `vrrp` behavior documented on their owning pages |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` (`router-advertisement` section) |
| 7 | Wire format changed? | Yes | `docs/features/interfaces.md` (MTU option and split RAs) and `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` (unsolicited NA spacing and target check); no `docs/architecture/wire/` page covers NDP |
| 8 | Plugin SDK/protocol changed? | No | No `pkg/plugin/` change |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4861.md` (remove `{gap}` on the nine closed rows with tagged test evidence) |
| 10 | Test infrastructure changed? | No | Uses existing Go unit/userns/QEMU and `.ci` test runners |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` makes no per-row RFC 4861 claim altered by these fixes |
| 12 | Internal architecture changed? | Yes | `docs/features/interfaces.md` (D-6 reversal, own ND sysctls, RA splitting) and `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata change |
| 14 | Prometheus counters added/changed? | Yes | `docs/features/interfaces.md`, `docs/config-reference.md`, and `docs/plugin-development/metrics.md` (`ze_iface_ra_rs_discarded_total{interface, reason}`) |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Reuses existing `(sysctl, applied)` and `(sysctl, default)` event types |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived via `./le spec citation anchors`: updated: `docs/features/interfaces.md`, `docs/architecture/vrrp/vrrp-first-hop-redundancy.md`, `docs/config-reference.md`, `docs/plugin-development/metrics.md`; unaffected (unchanged subsystem contracts): `docs/architecture/iface/management.md` (link-event fan-out), `docs/architecture/iface/logical-name-resolution.md` (resolver path), `docs/architecture/core-design.md` (core leaf tier), `docs/architecture/doctor-and-health-checks.md` (doctor framework), `docs/research/l2tpv2-ze-integration.md` (PPP RS admission contract), `docs/architecture/appliance/kernel-profiles.md` (kernel build profiles), `docs/architecture/config/apply-ordering.md` (operation graph), `docs/architecture/config/transaction-protocol.md` (transaction phases), `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` (IPsec XFRM), `docs/architecture/l2tp/cpe-1-pppoe-client.md` (PPPoE client), `docs/architecture/mpls/mpls-kernel.md` (MPLS FIB), `docs/architecture/plugin/plugin-relationships.md` (plugin roots), `docs/architecture/plugin/plugin-system.md` (plugin registration), `docs/architecture/traffic/cos-plugin.md` (CoS QoS profiles), `docs/features/dns-resolver.md`, `docs/features/plugins.md`, `docs/guide/appliance.md`, `docs/guide/health-checks.md`, `docs/guide/ipsec.md`, `docs/guide/migrating-from-vyos.md`, `docs/guide/mpls.md`, `docs/guide/netlab.md`, `docs/guide/plugins.md`, `docs/guide/pppoe.md`, `docs/guide/redistribution.md`, `docs/guide/vpp.md`, `docs/guide/vrrp.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/features/interfaces.md` and `docs/guide/configuration.md` RA tables/examples checked and updated with `link-mtu` |

## Design presentation

### What changes

This fix closes all nine shipped-capability RFC 4861 MUST gaps across Ze's IPv6 Router Advertisement sender, Router Solicitation receivers, and VRRP unsolicited Neighbor Advertisements, while moving the two unstarted features (ND Proxy and AH/ESP-secured ND) to `plan/post-release/features/spec-rfc4861-nd-proxy-and-ipsec-nd.md` per Thomas's 2026-11-24 ruling. Concretely: (1) `iface/ra` validates every received Router Solicitation (Hop Limit 255, code 0, length `>= 8`, non-zero in-bounds option lengths, no SLLA from `::`) using one shared `internal/core/ndp` validator also used by `l2tp/ppp`, counting discards by reason; (2) VRRP refuses multicast IPv6 virtual addresses at config verify and in `BuildNA`, and spaces repeated IPv6 unsolicited NAs by at least `RetransTimer` (1,000 ms floor, round-by-round across VIPs); (3) operators can configure `link-mtu` (`0 | 1280..65535`) under `router-advertisement` to emit the RFC 4861 MTU option; (4) enabled RA units push their non-zero `hop-limit`, `reachable-time`, and `retransmit-timer` onto the router's own interface via `sysctl` defaults; (5) reversing D-6, Ze suppresses non-zero Router Lifetime RAs while IPv6 forwarding is off, sends one final Router Lifetime 0 RA when forwarding turns off, and keeps the `ze doctor` warning; and (6) `ndp.SplitRA` splits large option sets across multiple RAs so no advertisement exceeds the link MTU.

### Before and after

1. **Invalid Router Solicitation on an advertising interface**
   - Before: `readSolicitations` ignored Hop Limit, ICMP code, packet length, and option lengths, signaling an RA answer for every ICMPv6 type 133 packet (even Hop Limit 254 or a zero-length option).
   - After: `openRASocket` enables `IPV6_RECVHOPLIMIT`, `readSolicitations` validates via `ndp.ValidateRouterSolicitation`, silently discards invalid packets, increments `ze_iface_ra_rs_discarded_total{interface="eth0",reason="hop-limit"}`, and answers only valid solicitations.
2. **Router Advertisement when IPv6 forwarding is off or turns off (D-6 reversal)**
   - Before: A unit with `router-advertisement { enabled true; }` (default `router-lifetime 1800`) advertised itself as a default router even with `ipv6 forwarding false`, and turning forwarding off left hosts pointing at a non-forwarding router until timeout.
   - After: With forwarding off, Ze suppresses non-zero Router Lifetime RAs, logs a WARN, and `ze doctor` still reports `doctor-iface-ra-forwarding` with `fix: set ipv6 forwarding true on the unit`. When forwarding turns off on a live advertising interface, Ze immediately transmits one final RA with `Router Lifetime = 0` and stops advertising until forwarding returns. Explicit `router-lifetime 0` continues to advertise prefixes/RDNSS with `Router Lifetime = 0`.
3. **AdvLinkMTU and router's own ND variables**
   - Before: `router-advertisement` had no `link-mtu` leaf, and configuring `hop-limit 128; reachable-time 45000; retransmit-timer 2000;` advertised those numbers to hosts while the router's own interface stayed at Linux kernel defaults.
   - After: `set interface ethernet eth0 unit 0 ipv6 router-advertisement link-mtu 1400` emits an ICMPv6 MTU option (type 5, length 1, MTU 1400) on the wire, and `applyRASysctl` applies `net.ipv6.conf.eth0.hop_limit=128`, `net.ipv6.neigh.eth0.base_reachable_time_ms=45000`, and `net.ipv6.neigh.eth0.retrans_time_ms=2000` through `sysctl` under source `interface-ra`.
4. **Large prefix list exceeding link MTU**
   - Before: Configuring 50 prefixes on a 1500-octet Ethernet interface produced a single 1664-octet IPv6 RA exceeding the link MTU.
   - After: `ndp.SplitRA` splits the advertisement into two valid RAs (each `<= 1500` octets including the 40-octet IPv6 header) whose union carries every prefix, route, and resolver.
5. **VRRP IPv6 multicast VIP and unsolicited NA spacing**
   - Before: `virtual-address [ fe80::1 ff02::1 ]` passed verification and emitted an NA targeting `ff02::1`; unsolicited NAs repeated 3 times 100 ms apart.
   - After: `validateGroup` refuses `ff02::1` at verify/commit naming RFC 4861 Section 4.4 and the fix; valid IPv6 VRRP promotions announce all VIPs immediately in round 0 and space the 3 rounds by at least `RetransTimer` (`>= 1000 ms`), while IPv4 GARP remains 100 ms.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Cap `router-advertisement prefix` with a static YANG `max-elements 40` instead of splitting RAs | Link MTU varies (1280 vs 1500 vs 9000) and delegated prefixes, retiring withdrawals, and RDNSS options share the same packet budget; RFC 4861 Sections 6.2.3 and 9 explicitly specify splitting options across multiple RAs |
| Keep D-6 (doctor warning only) or refuse `router-advertisement` at config verify when `ipv6 forwarding` is absent | Thomas explicitly reversed D-6 on 2026-11-24 to suppress non-zero Router Lifetime RAs at runtime, send a final Router Lifetime 0 RA on forwarding loss, and keep the doctor warning; verify-time refusal would reject valid configs where forwarding arrives from a `sysctl` block or profile |
| Raise `announceRepeatInterval` to 1000 ms for both IPv4 GARP and IPv6 NA | RFC 4861 Section 7.2.6 applies only to IPv6 Neighbor Discovery; slowing IPv4 GARP from 100 ms to 1000 ms degrades IPv4 failover convergence and breaks existing IPv4 GARP timing tests for no RFC reason |
| Write `hop_limit`, `base_reachable_time_ms`, and `retrans_time_ms` directly to `/proc/sys` from `iface-ra` | Violates `ai/doors/forwarding.md` ("Kernel tunables go through `sysctl`") and would bypass `sysctl` config-over-default precedence and shutdown/disable restoration |
| Duplicate `ValidRouterSolicitation` inside `internal/plugins/iface/ra` instead of sharing it via `internal/core/ndp` | Duplicates RFC 4861 §6.1.1 option-walking logic across two packages and risks drift between LAN RA and PPP subscriber RS validation |

### Documents departed from

- `docs/features/interfaces.md`, section `Send and accept are separate`: currently states `"Config verify cannot catch this, because forwarding can arrive from a sysctl profile after verify runs. Ze reports the state instead."` (design decision D-6). Per Thomas's 2026-11-24 owner ruling reversing D-6, this spec changes Ze to suppress non-zero Router Lifetime RAs while IPv6 forwarding is off and transmit one final Router Lifetime 0 RA when forwarding turns off, while retaining the `doctor-iface-ra-forwarding` warning. After this work, `docs/features/interfaces.md` will state that Ze suppresses non-zero Router Lifetime RAs whenever IPv6 forwarding is off, sends a final Router Lifetime 0 RA if forwarding turns off on an active advertising interface, preserves explicit `router-lifetime 0` prefix-only advertising, and continues to report the forwarding-off state via `doctor-iface-ra-forwarding`.

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- promote `ndp.ValidateRouterSolicitation` and `ndp.SplitRA` / `LinkMTU` seams in `internal/core/ndp`, add `link-mtu` leaf to `ze-iface-conf.yang`, wire `parseRAConfig` / `RASenderSpec`, register the three ND sysctl templates in `internal/core/sysctl/known_linux.go`, and wire the forwarding probe + `(sysctl, applied)` subscription in `internal/component/iface`.
   - Tests: `TestRFC4861AdvLinkMTUConfigured`, `TestRFC4861AdvertisingFollowsForwarding`, `TestRFC4861RouterUsesItsAdvertisedVariables`
   - Files: `internal/core/ndp/ra.go`, `internal/core/ndp/rs.go`, `internal/core/sysctl/known_linux.go`, `internal/component/iface/yang/ze-iface-conf.yang`, `config_ra.go`, `config_sysctl.go`, `config_apply.go`, `reconcile_ra.go`, `register.go`, `forwarding_linux.go`, `forwarding_other.go`
   - Verify: `link-mtu` reaches `RASenderSpec`, ND sysctl defaults emit under `interface-ra`, and forwarding off/on/off drives `reconcileRA` start/stop.
2. **Phase: Router Solicitation validity** -- wire `FlagHopLimit`, 1500-octet read buffer, `ndp.ValidateRouterSolicitation`, and `ze_iface_ra_rs_discarded_total` in `internal/plugins/iface/ra`; delegate `ppp.ValidRouterSolicitation` to `ndp.ValidRouterSolicitation` and add RFC-tagged tests on both LAN and PPP subscriber paths.
   - Tests: `TestRFC4861ValidateRouterSolicitation`, `TestRFC4861SolicitationReaderDiscardsInvalid`, `TestRFC4861SubscriberSolicitationReaderDiscardsInvalid`
   - Files: `internal/plugins/iface/ra/sender_linux.go`, `ifacera.go`, `rfc4861_solicitation_reader_linux_test.go`, `internal/component/l2tp/ppp/router_solicitation.go`, `internal/component/l2tp/ppp/kernel/rfc4861_solicitation_reader_test.go`
   - Verify: all invalid RS cases are discarded and counted; valid solicitations (including non-zero Reserved and unrecognized options) remain reported.
3. **Phase: VRRP multicast target refusal and unsolicited NA spacing** -- refuse multicast IPv6 VIPs in `validateGroup`, `frameBuilder`, and `BuildNA`; implement family-specific round-by-round `announcer` spacing (`>= RetransTimer` for IPv6, 100 ms for IPv4 GARP) with stop-interruptible wait.
   - Tests: `TestRFC4861VRRPRefusesMulticastTarget`, `TestRFC4861UnsolicitedNASpacedByRetransTimer`
   - Files: `internal/plugins/vrrp/groups.go`, `rfc4861_virtual_address_test.go`, `transport/na.go`, `transport/announce.go`, `transport/transport.go`, `transport/retrans_linux.go`, `transport/retrans_other.go`, `transport/rfc4861_unsolicited_na_test.go`
   - Verify: multicast IPv6 VIP is refused at verify and `BuildNA`; IPv6 unsolicited NA rounds stay `>= 1000 ms` apart while IPv4 GARP tests stay green.
4. **Phase: RA forwarding defense-in-depth, MTU option, and link-MTU splitting** -- complete `Sender.run` forwarding check + ON->OFF final Router Lifetime 0 RA, `ndp.SplitRA` chunk transmission, doctor warning updates, wire integration tests, functional `.ci` test, and docs updates.
   - Tests: `TestRFC4861MTUOptionEncoded`, `TestRFC4861AdvertisementInsideLinkMTU`, `TestRFC4861WireAdvLinkMTUOption`, `TestRFC4861ForwardingLossSendsFinalZeroLifetimeRA`, `TestRFC4861WireRASplitsToFitLinkMTU`, `test/plugin/iface-ra-link-mtu-and-forwarding.ci`
   - Files: `internal/plugins/iface/ra/sender_linux.go`, `doctor.go`, `register_doctor_codes.go`, `rfc4861_ra_integration_linux_test.go`, `internal/core/ndp/rfc4861_router_advertisement_test.go`, `test/plugin/iface-ra-link-mtu-and-forwarding.ci`, `docs/features/interfaces.md`, `docs/architecture/vrrp/vrrp-first-hop-redundancy.md`, `docs/guide/configuration.md`, `docs/config-reference.md`, `docs/plugin-development/metrics.md`
   - Verify: wire captures confirm MTU option, forwarding suppression/final RA, and split RAs all `<= linkMTU`.
5. **Phase: RFC records and summary update** -- remove `{gap}` on the nine closed rows of `rfc/short/rfc4861.md`, record discrimination verdicts via `./le rfc record` (including re-recording `TestRASolicitedResponse` if modified), and confirm `./le rfc check` passes.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All nine `RFC gaps` rows have enforcing code at `file:line`, positive and negative RFC-tagged tests, and discrimination records |
| Feature completeness | Every user story (1-5) passes end to end, including `iface-ra-link-mtu-and-forwarding.ci` |
| Correctness | `ndp.SplitRA` accounts for 40-octet IPv6 header plus SLLA and MTU option in every chunk; `readSolicitations` checks `cm != nil` and `cm.HopLimit == 255`; `validateGroup` rejects multicast in any IPv6 VIP slot; `announcer` wakes immediately on `close()` |
| Naming | YANG leaf is `link-mtu`; metric is `ze_iface_ra_rs_discarded_total{interface, reason}`; sysctl source is `interface-ra` |
| Data flow | ND sysctls go exclusively through EventBus `sysctl` defaults; `internal/core/ndp` has zero upward imports |
| Operator errors and doctor codes | Multicast VIP refusal and `link-mtu`/`retransmit-timer` refusals name the offending value, RFC section, and exact fix; `doctor-iface-ra-forwarding` retains its code and `Fix` |
| Rule: `ai/rules/rfc-compliance.md` | Every enforcing function carries `// RFC 4861 Section X.Y: "<quote>"` and preserves explicit `router-lifetime 0` operator control |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Shared RS validator & RA splitter in `internal/core/ndp` | Scoped unit test `go test -race ./internal/core/ndp/...` |
| LAN RA RS validation, forwarding suppression/final RA, MTU option, and split RA | Scoped Linux/QEMU tests on `./internal/plugins/iface/ra/...` and `./internal/component/iface/...` |
| Subscriber RS tagged tests | Scoped unit test `go test -race ./internal/component/l2tp/ppp/...` |
| VRRP multicast VIP refusal and `RetransTimer`-spaced unsolicited NA | Scoped unit test `go test -race ./internal/plugins/vrrp/...` |
| Functional `.ci` test for `link-mtu` and forwarding doctor warning | `./le test functional plugin` (or single `.ci` run of `test/plugin/iface-ra-link-mtu-and-forwarding.ci`) |
| RFC discrimination records and summary update | `./le rfc check` |
| Updated docs and split post-release spec | `./le doc check all` and `./le spec design check` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation (untrusted wire RS) | `ValidateRouterSolicitation` bounds-checks option length (`length == 0` or `length > remaining` stops loop with no infinite loop or out-of-bounds panic); `readSolicitations` drops `cm == nil` fail-closed and uses a fixed 1500-octet buffer |
| Resource exhaustion (RS flood / VRRP burst) | Coalesced depth-1 `solicitations` channel and `ndp.MinDelayBetweenRAs` keep RS floods bounded; `announcer` queue remains bounded at 16 and `close()` interrupts inter-repeat waits immediately |
| Sysctl path safety | `applyRASysctl` and `forwarding_linux.go` reject empty, `..`, `/`, or NUL device names before forming `/proc/sys/...` paths or keys |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood -> RESEARCH |
| Lint failure | Fix inline. If architectural -> DESIGN |
| Functional test fails | Check the AC: wrong AC -> DESIGN, correct AC -> IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- On HEAD, `internal/component/l2tp/ppp/ra_linux.go` was already replaced by the `IPv6Carrier` architecture (`ra_carrier.go`, `router_solicitation.go`, `kernel/ipv6.go`, `kernel/ipv6_linux.go`, and `pppoe/dataplane/vpp/ipv6_wire.go`), which already validates RS packets via `ppp.ValidRouterSolicitation`. Promoting that function to `internal/core/ndp` avoids a duplicate validator in `internal/plugins/iface/ra`.
- `TestRASolicitedResponse` (`RFC4861-6.2.2-2`) deliberately runs with forwarding off so the Linux kernel does not auto-join `ff02::2`; setting `RouterLifetime: 0` in that test harmonizes it with Thomas's 2026-11-24 D-6 reversal ("suppress non-zero Router Lifetime RAs when IPv6 forwarding is off") without weakening the all-routers join proof.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Split oversized RAs via `ndp.SplitRA` instead of capping `list prefix` with a small static `max-elements` | Static `max-elements 40` in YANG | Link MTU varies across interfaces (1280, 1500, jumbo) and delegated/retiring prefixes plus RDNSS share the packet budget; RFC 4861 §§6.2.3 and 9 explicitly specify splitting options across multiple RAs |
| Enforce forwarding suppression in both `reconcileRA` (with `(sysctl, applied)` wake) and `Sender.run` | Enforce only in `reconcileRA`, or only in `Sender.run` | `reconcileRA` avoids opening a raw socket or joining `ff02::2` while forwarding is off and reuses `Stop()`/`sendFinal` on config or `sysctl` transitions; `Sender.run` adds defense in depth if `/proc/sys` is flipped outside Ze |
| Emit router own ND variables via EventBus `sysctl` defaults under source `interface-ra` | Direct `/proc/sys` writes in `iface-ra` | Respects `ai/doors/forwarding.md` (`sysctl` owns all kernel tunables), preserves `config > transient > default` precedence, and cleanly restores original values on disable or reload |
| Use round-by-round VRRP IPv6 NA transmission with `max(retrans_time_ms, 1000 ms)` spacing while keeping IPv4 GARP at 100 ms | Raise both IPv4 and IPv6 to 1000 ms; or send each VIP's 3 repeats sequentially | Round-by-round gives 0 ms failover delay for every VIP and keeps 2-VIP bursts at 2 s (inside existing test deadlines) while guaranteeing `>= RetransTimer` between repeats; IPv4 GARP is outside RFC 4861 |

## Known Limitations

- Neighbor Discovery proxy (`RFC4861-7.2.8-1`, `RFC4861-7.2.8-2`) and AH/ESP-secured Neighbor Discovery (`RFC4861-11.2-1`, `RFC4861-11.2-2`) are out of scope for this release fix and are homed in `plan/post-release/features/spec-rfc4861-nd-proxy-and-ipsec-nd.md` per Thomas's 2026-11-24 owner ruling.

## RFC Documentation (Scope: protocol)

Add `// RFC 4861 Section X.Y: "<quoted requirement>"` above enforcing code.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 4861 §§4.6, 6.1.1 (`RFC4861-6.1.1-2`, `4.6-1`) | Default advertising interface and PPP subscriber carrier silently discard any RS failing Hop Limit 255, code 0, length `>= 8`, option length `> 0`, or SLLA from `::` | Non-zero Reserved field, unrecognized options, and options defined for other ND messages continue to be accepted | `TestRFC4861ValidateRouterSolicitation`, `TestRFC4861SolicitationReaderDiscardsInvalid`, `TestRFC4861SubscriberSolicitationReaderDiscardsInvalid` | `TestRFC4861SolicitationReaderIgnoresReservedAndOptions`, `TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions` |
| RFC 4861 §4.4 (`RFC4861-4.4-3`) | VRRP IPv6 groups refuse any multicast `virtual-address` at verify and never emit a multicast NA Target Address | Multiple unicast IPv6 virtual addresses in configuration order (first link-local) remain preserved | `TestRFC4861VRRPRefusesMulticastTarget` | `TestAnnounceMasterFramesPerAddress` |
| RFC 4861 §7.2.6 (`RFC4861-7.2.6-1`) | VRRP IPv6 unsolicited NA repeats are spaced by at least `RetransTimer` (`>= 1000 ms`, following higher interface `retrans_time_ms`) | IPv4 VRRP GARP retains 3x 100 ms burst spacing | `TestRFC4861UnsolicitedNASpacedByRetransTimer` | `TestAnnounceBurstRepeatsAndSpacing` |
| RFC 4861 §6.2.1 (`RFC4861-6.2.1-1`, `6.2.1-8`) | `link-mtu` defaults to 0 (no MTU option) and is configurable in `1280..65535`; non-zero `hop-limit`, `reachable-time`, and `retransmit-timer` are applied to the router's own interface via `sysctl` defaults | Explicit `sysctl { setting ... }` overrides `interface-ra` defaults; `0` means unspecified and leaves kernel default alone | `TestRFC4861AdvLinkMTUConfigured`, `TestRFC4861MTUOptionEncoded`, `TestRFC4861RouterUsesItsAdvertisedVariables` | `TestRFC4861AdvLinkMTUConfigured` (`0` omitted), `TestRFC4861RouterUsesItsAdvertisedVariables` (`0` skipped) |
| RFC 4861 §§6.2.4, 6.2.5 (`RFC4861-6.2.4-1`, `6.2.5-2`) | Default `router-lifetime 1800` is suppressed when IPv6 forwarding is off, and turning forwarding off sends one final Router Lifetime 0 RA | Explicit `router-lifetime 0` continues to advertise prefixes/RDNSS with Router Lifetime 0 when forwarding is off; `doctor-iface-ra-forwarding` still warns | `TestRFC4861AdvertisingFollowsForwarding`, `TestRFC4861ForwardingLossSendsFinalZeroLifetimeRA` | `TestRFC4861AdvertisingFollowsForwarding` (`router-lifetime 0`), `TestRASolicitedResponse` |
| RFC 4861 §9 (`RFC4861-9-7`) | Outgoing RAs are split across multiple packets via `ndp.SplitRA` so every packet stays within `effectiveMTU` (IPv6 header + ICMPv6 payload) | Single-packet RAs within link MTU remain byte-for-byte unchanged | `TestRFC4861AdvertisementInsideLinkMTU`, `TestRFC4861WireRASplitsToFitLinkMTU` | `TestRASenderWireFormat` |

## Review Gate

To be populated by independent `/ze-review` and `/ze-review-spec` rounds after implementation.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] FRR `zebra/rtadv.c` and BIRD `proto/radv/` checked and deciding functions quoted
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior, Existing Mechanisms, Design documents, Design presentation, and Data Flow sections completed
- [ ] AC-N rows carry positive observable assertions and Permitted Survivors
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist and Documentation Update Checklist answered Yes/No/N-A with evidence

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Every user story (1-5) has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted scoped tests pass in the main checkout (run by the main thread): `./le test unit core`, `./le test unit plugins`, QEMU Linux integration tests for `internal/plugins/iface/ra/`, `test/plugin/iface-ra-link-mtu-and-forwarding.ci`, and `./le rfc check`
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every owned journal row resolved
- [ ] Split post-release items homed in `plan/post-release/features/spec-rfc4861-nd-proxy-and-ipsec-nd.md`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop / Linux kernel wire tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + journal rows
- [ ] **Commit B:** `remove plan/release/fixes/spec-rfc4861-ze-owned-must-gaps.md` only, in the same `./le commit create` script

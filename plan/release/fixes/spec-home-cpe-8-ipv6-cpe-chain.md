# Spec: home-cpe-8-ipv6-cpe-chain

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P3 |
| Release reason | Feature: a dual-stack home CPE on a PPPoE WAN. Owner timing: a missing RFC feature may wait until after the first release. The rows it closes are recorded gaps of enrolled RFCs (RFC 7084 G-4, G-5, WLL-2, WLL-3, L-2, L-3, L-4; RFC 5072 on the client side). |
| RFC gaps | rfc5072, RFC7084-4.1-4, RFC7084-4.1-5, RFC7084-4.2-9, RFC7084-4.2-10, RFC7084-4.3-5, RFC7084-4.3-6, RFC7084-4.3-7, RFC9915-6.3-1, RFC9915-6.3-2, RFC9915-6.3-3 |
| Depends | Implementation: `spec-home-cpe-1-logical-interface-name.md` (the PPPoE client's kernel device carries its configured name, which every IPv6 action, the DHCPv6 lease and the delegated-prefix source name). Proof and closure: `spec-home-cpe-18-interoplab-multi-link-topology.md` (the `topology` suite and its `interop-topology` verb that run `pppoe-dhcpv6-pd-ze-client`); `plan/release/validation/spec-ppp-ipv6-link-local-never-assigned.md` (its live proof validates A-2) |
| Phase | 5/5 |
| Handoff | - |
| Updated | 2026-11-08 |

Parent: `plan/release/fixes/spec-home-cpe-0-umbrella.md` (child 8).

RFC gaps, in full: RFC 5072 (IPv6CP, client side); RFC 7084 `RFC7084-4.1-4` (G-4), `RFC7084-4.1-5` (G-5), `RFC7084-4.2-9` (WLL-2), `RFC7084-4.2-10` (WLL-3), `RFC7084-4.3-5` (L-2), `RFC7084-4.3-6` (L-3), `RFC7084-4.3-7` (L-4); RFC 9915 Section 6.3 rows `RFC9915-6.3-1`, `RFC9915-6.3-2`, `RFC9915-6.3-3`, `{gap}` and owed (owner ruling below).

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner ruling: delegated-prefix use is owed

Thomas ruled on 2026-11-07 that `RFC9915-6.3-1`, `RFC9915-6.3-2` and
`RFC9915-6.3-3` are requirements Ze owes. The line this spec carried saying
delegated-prefix use was declined on 2026-10-19 had no decision of his behind
it, and a feature an earlier agent declined with no owner decision behind it
is a gap, not a declined feature (`ai/META-DECISIONS.md`). The three rows are
`{gap}` in `rfc/short/rfc9915.md` and are owed. Phases 4 and 5 are in scope
and run: they implement and prove the three rows, and RFC 7084 G-4, G-5, L-2,
L-3 and L-4, which act on the LAN unit that takes its prefix from the WAN (the
CE role below).

## Task

Make Ze a dual-stack CPE on a PPPoE WAN:

1. **IPv6CP in the PPPoE client** (RFC 5072). Negotiate the Interface-Identifier
   alongside IPCP, independently of it (RFC 7084 WLL-3), and install the
   link-local address on `<wan>` from the negotiated identifier.
2. **RA and SLAAC on the WAN.** Accept Router Advertisements on `<wan>` with
   forwarding on (accept_ra 2), install the default route from Router
   Discovery (RFC 7084 W-3), autoconfigure a global address where the ISP
   offers one.
3. **DHCPv6 prefix delegation on the PPPoE interface** (RFC 9915), with the
   existing DHCPv6 client, which today runs only on an interface unit.
4. **A /64 from the delegated prefix on a named LAN unit** (RFC 7084 L-2),
   selected with the syntax under "LAN prefix selection syntax", with the
   address on the LAN unit and the prefix advertised in its RAs, the
   advertised and installed lifetimes never longer than what remains of the
   delegated prefix (RFC 9915 Section 6.3), and a Route Information Option for
   the delegated prefix (L-3).
5. **Router Lifetime per the condition matrix** (RFC 7084 G-4, G-5, L-4): the
   configured value only while the WAN has a default router and the unit has a
   prefix, zero otherwise, and an immediate zero-lifetime RA when either is lost.
6. **No Release while delegated** (`RFC9915-6.3-3`).

## Device identity

Child 1 makes the kernel device of a `pppoe-client` carry the client's
configured name (child 1 AC-1: "kernel device `pppoe0` exists; no `pppN`
created by the client"). This spec builds on that contract and never names a
`pppN` device: the link-local address, the IPv6 sysctls, the DHCPv6 client,
the WAN default route and the delegated-prefix source all name the client by
its configured name, written `<wan>` below (`pppoe0` in the examples). The
DHCPv6 lease names the same device (`DHCPPayload.Name`, set from
`v6Worker.payload`), so the LAN consumer matches its source by that name.

## Router Lifetime condition matrix

This matrix is the one definition of the Router Lifetime this spec owes. The
Existing Mechanisms, Task, Data Flow, Acceptance Criteria and tests all cite
it rather than restate it.

It governs a LAN unit in the CE role: a unit whose `ipv6 delegated-prefix`
names a WAN source (syntax below). A unit with no source keeps today's
behavior, the configured `router-advertisement router-lifetime`, which RFC
4861 Section 6.2.3 states: "In the Router Lifetime field: the interface's
configured AdvDefaultLifetime." Ze knows which interface is the WAN only
through that source, so the CE rules below apply only where it is named.

Inputs, evaluated per CE-role unit:

| Input | Holds when |
|-------|------------|
| W (WAN default router) | the main table holds an IPv6 default route (`::/0`) whose output device is `<wan>`, learned by Router Discovery (RFC 7084 W-3) or configured |
| P (prefix configured or delegated) | the unit advertises at least one prefix: an operator `router-advertisement prefix` entry, or its derived /64 while the delegation's valid lifetime remains above zero |

| W | P | Router Lifetime in every RA the unit sends | PIO and RIO | Source |
|---|---|--------------------------------------------|-------------|--------|
| yes | yes | the configured `router-lifetime` | sent | RFC 4861 Section 6.2.3 |
| yes | no | 0 | RIO only if a delegation is still held | RFC 7084 L-4 |
| no | yes | 0 | sent | RFC 7084 G-4; L-3 keeps the RIO |
| no | no | 0 | none | RFC 7084 G-4 and L-4 |

Transitions:

| Change | Ze sends |
|--------|----------|
| W or P goes from true to false while the unit advertised a non-zero Router Lifetime | one unsolicited RA at once with Router Lifetime 0, then the periodic schedule. For W this is G-5; for P it is the configuration change RFC 4861 Section 6.2.4 allows ("the router MAY transmit up to MAX_INITIAL_RTR_ADVERTISEMENTS unsolicited advertisements") |
| W and P both become true | the configured value from the next RA; an immediate RA is sent under the same Section 6.2.4 sentence |
| The session ends | the device disappears with its default route, so W becomes false: the G-5 row above |

The RFC text each row rests on:

- RFC 7084 Section 4.1 G-4: "By default, an IPv6 CE router that has no default
  router(s) on its WAN interface MUST NOT advertise itself as an IPv6 default
  router on its LAN interfaces. That is, the "Router Lifetime" field is set to
  zero in all Router Advertisement messages it originates [RFC4861]."
- RFC 7084 Section 4.1 G-5: "By default, if the IPv6 CE router is an
  advertising router and loses its IPv6 default router(s) and/or detects loss
  of connectivity on the WAN interface, it MUST explicitly invalidate itself as
  an IPv6 default router on each of its advertising interfaces by immediately
  transmitting one or more Router Advertisement messages with the "Router
  Lifetime" field set to zero [RFC4861]."
- RFC 7084 Section 4.3 L-4: "An IPv6 CE router MUST NOT advertise itself as a
  default router with a Router Lifetime [RFC4861] greater than zero if it has
  no prefixes configured or delegated to it."
- RFC 7084 Section 4.3 L-3: "This advertisement is independent of having or not
  having IPv6 connectivity on the WAN interface."
- RFC 4861 Section 6.2.1, AdvDefaultLifetime: "A value of zero indicates that
  the router is not to be used as a default router."

P is judged per unit, which is stricter than L-4's router-wide wording: a
router with no prefix anywhere has none on any unit, so L-4 holds, and a unit
that advertises no prefix gives its hosts no global address to source traffic
from, so withdrawing the default router there loses nothing. The final RA when
an interface stops advertising (RFC 4861 Section 6.2.5, `Sender.sendFinal`)
is unchanged.

Interoperability reference: BIRD 2.18's RA protocol implements the same W
input as an operator option. `radv_rt_notify` (`proto/radv/radv.c`) sets
`p->active` from whether the configured `trigger` route (for a CPE, `::/0`)
exists, and on a change calls `radv_iface_notify_all(p, RA_EV_CHANGE)`, which
resets the interface to `MAX_INITIAL_RTR_ADVERTISEMENTS` fast advertisements;
`radv_prepare_ra` (`proto/radv/packets.c`) writes the configured
`default_lifetime` only while `p->active || !ic->default_lifetime_sensitive`,
and 0 otherwise. Ze derives the trigger from the delegated-prefix source
instead of asking the operator for it, because G-4 is the CE default. FRR's
`zebra` RA sender is still to be read before approval: no FRR checkout was
available to this revision.

## LAN prefix selection syntax

There is no delegated-prefix consumer in Ze's YANG today: `dhcpv6 pd` holds
only `length` (the hint), and `router-advertisement prefix` takes literal
prefixes (`ze-iface-conf.yang`, `grouping interface-unit`). RFC 7084 frames the
selection per LAN interface (L-2, quoted under Required Reading), and Ze
already configures addressing on the unit that holds the address (`ipv6
address`). The selection therefore lives on the LAN unit, inside its `ipv6`
container:

| Node | Type | Description |
|------|------|-------------|
| `delegated-prefix` | list, key `source` | one entry per WAN delegation this unit takes a /64 from |
| `source` | string, validated | the configured name of a `pppoe-client` whose `ipv6 dhcpv6 pd` is set; a custom validator refuses any other name, with the fix in the message |
| `subnet-id` | uint16, mandatory | the /64 within the delegated prefix: `<delegated prefix> + subnet-id`, shifted into bits (delegated length) to 64 |
| `interface-identifier` | IPv6 address, low 64 bits only, default `::1` | the host part of the unit's own address in that /64 |

Example as set lines: `set interface ethernet lan0 unit 0 ipv6 delegated-prefix
pppoe0 subnet-id 1` gives `lan0` the address `<delegated>:1::1/64` and, with
`router-advertisement enabled true`, advertises that /64 in a PIO with the A
and L flags set (RFC 7084 L-6) and the delegated prefix in a RIO.

Range rules:

| Rule | Where it is refused |
|------|---------------------|
| `subnet-id` at or above 2^(64 - `pd length`) when the source sets `pd length` | config verify, naming the largest valid id |
| Two units naming the same `source` and `subnet-id` (L-2: "a separate /64 ... for each of its LAN interfaces") | config verify, naming both units |
| `subnet-id` outside a delegation shorter than the hint (WPD-3 lets the server choose) | at run time: the unit takes no /64, P stays false for it, and Ze logs a system management error (RFC 7084 `RFC7084-4.2-31`: "If the delegated prefix is too small to address all of its interfaces, the IPv6 CE router SHOULD log a system management error.") |

Two choices neither the RFC nor the existing YANG settles, and BIRD, which has
no DHCPv6 client, offers no reference. Both are settled on the syntax above,
one subnet id per LAN unit, the shape the brief for Phases 4 and 5 names:
OD-1 (a) and OD-2 (a).

| Open decision | Options | Evidence |
|---------------|---------|----------|
| OD-1: where the selection is written | (a) on the LAN unit, as above; (b) on the WAN client, listing the LAN interfaces it feeds, as VyOS writes `interfaces ethernet eth0 dhcpv6-options pd 0 interface eth1 sla-id 1` (vyos-1x `interface-definitions/include/interface/dhcpv6-options.xml.i`: `pd` tag node holding `length` and an `interface` tag node with `address` and `sla-id`, read at commit db29bdc89) | (a) matches RFC 7084 L-2's per-LAN-interface framing and Ze's `ipv6 address`, which lives on the unit it configures; it keeps the LAN consumer in iface reading the published lease (Key Design Decisions). (b) eases the VyOS mapping (child 10) to a one-to-one rewrite; with (a) child 10 moves each `interface <lan> sla-id <n>` under that LAN unit, still one line per LAN |
| OD-2: whether `subnet-id` may be omitted | (a) mandatory, as above; (b) optional, Ze assigning the lowest free id per source, as OpenWrt's `ip6assign` does | (a) gives an address that does not move when another LAN unit is added or removed, and needs no persisted assignment state; (b) needs that state in `statestore` (`ai/rules/architecture.md`) so a reboot does not renumber the LAN |

## Required Reading

### Architecture Docs
- [ ] `docs/guide/pppoe.md`, `docs/guide/configuration.md` (interface units, `ipv6 dhcpv6`, `router-advertisement`)
  → Constraint: `list pppoe-client` in `ze-iface-conf.yang` uses `grouping interface-common` only, so it has no `ipv6` container today; the `ipv6` container is written inline inside `grouping interface-unit`.
- [ ] `plan/post-release/features/spec-dhcpv6-client-0-umbrella.md` Q5
  → Decision: Q5 recorded delegated-prefix use as declined with no decision of Thomas's behind it. He ruled on 2026-11-07 that `RFC9915-6.3-1/2/3` are owed ("Owner ruling" above); the umbrella's other rows are not judged here.
- [ ] `plan/post-release/features/spec-router-advertisement-options-out-of-scope.md`
  → Constraint: it holds RFC 4191 route information and router preference as deferred options. This spec implements the RIO only for a delegated prefix, as RFC 7084 L-3 requires; router preference and operator-configured routes stay there.
- [ ] `plan/release/validation/spec-ppp-ipv6-link-local-never-assigned.md`
  → Constraint: whether the kernel derives a link-local on a PPP device is unverified; the client installs the link-local from the negotiated identifier itself, as pppd does, and that spec's live proof covers the server side.
- [ ] `plan/post-release/features/spec-l2tp-ipv6-subscriber.md`
  → Constraint: the subscriber (server) IPv6 side is that spec's; this one is the client.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5072.md`
  → Constraint: RFC 5072 Section 2: "Before any IPv6 packets may be communicated, PPP MUST reach the network-layer protocol phase, and the IPv6 Control Protocol MUST reach the Opened state."
  → Constraint: RFC 5072 Section 4.1: "A Configure-Request MUST contain exactly one instance of the interface-identifier option [1]."
  → Constraint: RFC 5072 Section 5: "The negotiated interface identifier is used by the local end of the PPP link to autoconfigure an IPv6 link-local unicast address for the PPP interface."
- [ ] `rfc/short/rfc7084.md`
  → Constraint: RFC 7084 Section 4.3 L-2: "The IPv6 CE router MUST assign a separate /64 from its delegated prefix(es) (and ULA prefix if configured to provide ULA addressing) for each of its LAN interfaces."
  → Constraint: "Prefix delegation is two halves" (the summary's pitfall): requesting IA_PD is the client, carving and advertising is the CE glue.
  → Constraint: G-4, G-5, L-3 and L-4 are quoted under "Router Lifetime condition matrix"; the rows `RFC7084-4.1-4` and `RFC7084-4.1-5` are gaps no other spec owns, and both act on the Router Lifetime this spec changes, so this spec takes them.
- [ ] `rfc/short/rfc9915.md`
  → Constraint: RFC 9915 Section 6.3: "If the client uses a delegated prefix to configure addresses on interfaces on itself or other nodes behind it, the preferred and valid lifetimes of those addresses MUST be no longer than the remaining preferred and valid lifetimes, respectively, for the delegated prefix at any time."
- [ ] `rfc/short/rfc4861.md`, `rfc/short/rfc4862.md` - RA and SLAAC on the WAN and LAN.
  → Constraint: RFC 4861 Section 6.2.3 fills the Router Lifetime from AdvDefaultLifetime; Section 6.2.1 says zero means "not to be used as a default router"; Section 6.2.4 allows extra unsolicited RAs when the advertised information changes (quoted under the matrix).

**Key insights:**
- The server side already parses and writes IPv6CP options (`internal/component/l2tp/ppp/ipv6cp.go`: `parseIPv6CPOptions`, `writeIPv6CPOptions`, `isValidIPv6CPInterfaceID`, `generateIPv6CPInterfaceID`, `suggestIPv6CPInterfaceID`); the client reuses that codec rather than writing a second one.
- The DHCPv6 client already requests IA_PD and keeps the binding (`v6KindPrefix`, "published, not installed"); the missing half is the consumer.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/pppoeclient/session.go` - `negotiateSession` calls `negotiateLCP`, `runClientAuth`, then `negotiateIPCP`: the client runs LCP, authentication, then IPCP only.
- [ ] `internal/component/l2tp/pppoeclient/network_phase.go` - `keepaliveLoop`: a unit frame whose protocol is not `ppp.ProtoIPCP` is dropped with `continue`; IPv6CP from the AC is never answered.
- [ ] `internal/component/l2tp/ppp/ipv6cp.go` - the server-side IPv6CP option codec and identifier rules (`parseIPv6CPOptions`, `writeIPv6CPOptions`, `isValidIPv6CPInterfaceID`, `generateIPv6CPInterfaceID`, `suggestIPv6CPInterfaceID`), all unexported in package `ppp`.
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `list pppoe-client` uses `grouping interface-common` only; `grouping interface-unit` holds the `ipv6` container (`address`, `autoconf`, `accept-ra`, `forwarding`, `rpf-check`), its `dhcpv6` container (`enabled`, `pd length` with range 48..64, `duid`) and its `router-advertisement` container (`enabled`, intervals, `router-lifetime`, flags, `prefix` list, `rdnss`). No node consumes a delegated prefix.
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_lease_linux.go` - `v6KindPrefix`: "an IA Prefix inside the IA_PD, published, not installed".
- [ ] `internal/plugins/iface/dhcp/dhcp_v6_linux.go` - `v6Worker.effect` installs IA_NA addresses and publishes every installed binding through `publishDHCP` with `v6Worker.payload`; the `iface.DHCPPayload` it builds carries `Name`, `Unit`, `Address`, `PrefixLength` and `LeaseTime` (the valid lifetime in seconds) but no preferred lifetime; no subscriber configures an address from an IA_PD prefix (`rfc/short/rfc9915.md` row 6.3-1 reason). `v6Worker.shutdown` builds and sends a Release from the whole lease, prefixes included.
- [ ] `internal/component/iface/config_ra.go` - `raUnitConfig` (`Prefixes` from the operator's `prefix` list only, `RouterLifetime` from `router-lifetime`, default `raDefaultRouterLifetime` 1800 seconds); `parseRAConfig`, `raValidate`.
- [ ] `internal/plugins/iface/ra/sender_linux.go` - `NewSender` starts one sender per `iface.RASenderSpec`, fixed for the sender's life; `zeroLifetime` is called only from `Sender.sendFinal`, when the interface stops advertising. Nothing lowers the Router Lifetime for a WAN or prefix condition (`rfc/short/rfc7084.md` rows 4.1-4, 4.1-5, 4.3-7 reasons).
- [ ] `internal/core/ndp/ra.go` - `BuildRA` writes the header, SLLA (`writeSourceLinkLayerAddress`), PIO (`writePrefixOptions`) and RDNSS (`writeRDNSS`); no RIO.
- [ ] `internal/core/routewatch/routewatch.go` - `Watcher.Register` delivers `RouteEvent` (`Prefix`, `NextHop`, `Protocol`, `Metric`, `TableID`, `Action`) from one netlink route subscription (`Global`); the event carries no output device.
- [ ] `test/interop-pppoe/scenarios/ipv6cp-missing-option`, `ipv6cp-zero-identifier` - IPv6CP scenarios on the AC side.

**Behavior to preserve:**
- IPv4-only PPPoE sessions against an AC that does not offer IPv6CP: the client's IPv6CP Configure-Request is rejected or times out and IPCP stays Opened (RFC 7084 WLL-3, independent NCPs).
- Operator-configured RA prefixes and the configured Router Lifetime on a unit with no `delegated-prefix` source (matrix scope).
- The final zero-lifetime RA when an interface stops advertising (`Sender.sendFinal`).
- The DHCPv6 client on interface units.

**Behavior to change:**
- The client opens IPv6CP; DHCPv6 and the IPv6 sysctls apply on `<wan>`; a LAN unit can take a /64 from a named delegation; a CE-role unit's Router Lifetime follows the matrix.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec |
|-----------|------------------------------------------|--------------------|-----------|
| Server-side IPv6CP codec | `internal/component/l2tp/ppp/ipv6cp.go` `parseIPv6CPOptions`, `writeIPv6CPOptions`, `generateIPv6CPInterfaceID`, `suggestIPv6CPInterfaceID` | encodes and decodes the Interface-Identifier option and draws identifiers for the LNS | extends: the codec is exported to the client within the component rather than written a second time |
| Client NCP phase | `internal/component/l2tp/pppoeclient/session.go` `negotiateIPCP`; `network_phase.go` `keepaliveLoop` | runs LCP, authentication, then IPCP only; drops any non-IPCP unit frame | extends: IPv6CP negotiation beside IPCP, and its frames answered in the network phase |
| Unit IPv6 config | `internal/component/iface/yang/ze-iface-conf.yang` `grouping interface-unit` (`ipv6`, `dhcpv6`, `router-advertisement`), `list pppoe-client` | IPv6 leaves on a unit; `pppoe-client` uses `interface-common` only | extends: the `ipv6` container becomes a grouping shared with `pppoe-client`, and a unit's `ipv6` gains the `delegated-prefix` list ("LAN prefix selection syntax") |
| Configured device name | `spec-home-cpe-1-logical-interface-name.md` (the client's device carries its configured name) | after child 1, the PPPoE session device is named after the `pppoe-client` key | reuses: every IPv6 action, the DHCPv6 lease and the `delegated-prefix` source name `<wan>`; no `pppN` lookup is written |
| DHCPv6 lease effect | `internal/plugins/iface/dhcp/dhcp_v6_linux.go` `v6Worker.effect`, `v6Worker.payload`, `v6Worker.shutdown`; `dhcp_v6_lease_linux.go` `v6KindPrefix`; `internal/component/iface/iface.go` `DHCPPayload` | installs IA_NA addresses and publishes each IA_PD prefix with its valid lifetime only; nothing consumes it; Release on shutdown covers the prefixes | extends: DHCPv6 runs on `<wan>`; the payload gains the preferred lifetime and the Reply's receive instant so the consumer bounds both lifetimes; a new LAN consumer subscribes to the published prefix; `shutdown` sends no Release for a prefix a LAN unit still uses |
| RA unit config | `internal/component/iface/config_ra.go` `raUnitConfig`, `parseRAConfig`, `raParsePrefixes` | prefixes come from the operator's `prefix` list only; Router Lifetime from `router-lifetime` | extends: prefixes derived from the delegated prefix are added at run time, and a CE-role unit's Router Lifetime follows the matrix (W and P inputs) |
| RA sender | `internal/plugins/iface/ra/sender_linux.go` `NewSender`, `Sender.run`, `zeroLifetime`, `Sender.sendFinal`; `iface.RASenderSpec` | one sender per unit, its spec fixed for its life; zero lifetime only in the final RA | extends: the sender takes run-time updates of derived prefixes and of the matrix result, and sends an immediate RA on a true-to-false transition; `zeroLifetime` is reused for the zero rows |
| RA encoder | `internal/core/ndp/ra.go` `BuildRA`, `RAConfig`, `writePrefixOptions` | writes SLLA, PIO and RDNSS; no RIO | extends: a Route Information Option (RFC 4191 Section 2.3) written into the same buffer |
| Route subscription | `internal/core/routewatch/routewatch.go` `Watcher.Register`, `RouteEvent`, `Global` | one shared netlink route subscription, used by `internal/plugins/kernel` and `internal/plugins/fib/kernel`; the event has no output device | extends: `RouteEvent` gains the output interface index, and the RA side registers a handler that sets W from `::/0` routes through `<wan>`; no second netlink subscription |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `interface pppoe-client <wan> ipv6 { accept-ra; autoconf; forwarding; dhcpv6 { enabled; pd { length } } }` and, on the LAN unit, `ipv6 delegated-prefix <wan> { subnet-id; interface-identifier }` with `router-advertisement enabled` ("LAN prefix selection syntax").
- Wire: IPv6CP (PPP protocol 0x8057) on the PPPoE session; RA on `<wan>`; DHCPv6 Reply with IA_PD.

### Transformation Path
1. LCP and authentication as today; then IPCP and IPv6CP negotiate independently on the session.
2. On IPv6CP Opened, the client installs `fe80::<identifier>/64` on `<wan>` and applies `accept_ra 2`, `autoconf`, `forwarding`.
3. The DHCPv6 client starts on `<wan>`, requests IA_PD with the configured hint, and publishes the binding with its valid and preferred lifetimes and the Reply's receive instant.
4. The LAN unit's delegated-prefix consumer derives the /64 at `subnet-id`, installs `<derived /64> + interface-identifier` with lifetimes bounded by what remains of the delegation, and hands the prefix to the RA sender. P becomes true.
5. The route subscription reports `::/0` through `<wan>` (the kernel installs it from the AC's RA, W-3); W becomes true.
6. The RA sender advertises the /64 in a PIO and the delegated prefix in a RIO, lifetimes bounded the same way, and the Router Lifetime the matrix gives for the current W and P.
7. On a Renew or Rebind with new lifetimes, the address and the advertised lifetimes follow. On loss of the delegation, the PIO is advertised with zero lifetimes, the address removed, P becomes false. On loss of the WAN default route or of the session, W becomes false. Each true-to-false change sends the immediate zero-lifetime RA of the matrix. No Release is sent while the delegation is in use.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| PPPoE client ↔ kernel | link-local and sysctls on `<wan>` via netlink | No |
| DHCPv6 client ↔ iface | `dhcp-acquired` event carrying the IA_PD binding | No |
| kernel ↔ RA side | `routewatch` route events for `::/0` with the output device | No |
| iface ↔ RA sender | the unit's RA configuration with derived prefixes and the matrix inputs | No |

### Integration Points
- `internal/component/l2tp/ppp/ipv6cp.go` codec reused by the client.
- The DHCPv6 client's event bus publication, already present.
- `raUnitConfig.Prefixes`, extended with prefixes derived at run time.
- `routewatch.Global`, already shared by the kernel and fib-kernel plugins.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | the LAN consumer reads the published event; the DHCP plugin configures no other interface |
| No unintended coupling | Yes | the PPPoE client does not call the DHCPv6 client; config wires them |
| No duplicated functionality | Yes | one IPv6CP codec; the `ipv6` container is a grouping shared by unit and pppoe-client |
| Zero-copy preserved where applicable | Yes | IPv6CP frames written into the session buffer, as IPCP |
| Registration over hardcoding, outbound | Yes | consumers subscribe to the event |
| Registration over hardcoding, inbound | Yes | the RA sender receives prefixes, it learns no DHCP name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | accel-ppp serves IPv6CP, RAs and DHCPv6 IA_PD on PPPoE | accel-ppp `ipv6pool` and `ipv6-dhcp` modules | interop peer must change | read accel-ppp's source and configure the existing `Dockerfile.accel` image | unvalidated |
| A-2 | The kernel does not derive a link-local on the client's device by itself | `spec-ppp-ipv6-link-local-never-assigned` premise | the client's install is redundant but harmless | that spec's live proof; the client test reads the address either way | unvalidated |
| A-3 | The ipv6 grouping can be shared by `pppoe-client` without the unit's other leaves | YANG grouping structure | a separate container duplicates leaves | YANG refactor in Phase 1 | unvalidated |
| A-4 | Delegated-prefix use on the LAN is owed work | Thomas's ruling of 2026-11-07 ("Owner ruling" above) | - | owner ruling | validated |
| A-5 | The kernel installs the RA-learned `::/0` on `<wan>` with the AC's link-local as next hop, and removes it when the device goes | Linux `accept_ra 2` behaviour; child 1 AC-1 (device exists only during a session) | W must be derived from the sender's own RA reception instead | `TestRFC7084G4RouterLifetimeFollowsWANDefault` against a route event fixture, and the topology scenario reading the LAN RA after `accel-cmd` terminates the session | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | IPv6CP failure tears down IPv4 | IPCP closes when IPv6CP fails | WLL-3 test: IPv6CP rejected, IPv4 traffic continues |
| R-2 | Lifetimes drift past the delegation's | advertised valid lifetime above remaining | computed from the receive instant at each RA; unit test with a fake clock |
| R-4 | A transient `::/0` flap withdraws the LAN's default router | LAN hosts lose their default route on a WAN blip | G-5 requires it; the matrix restores the configured value with the next RA, and Section 6.2.4's fast RAs shorten the gap |
| R-3 | Journal row in `silent-fall-through.md` (2026-10-31, owned by `spec-ppp-l2tp-aaa-one-off-defects`) changes the same loop (Protocol-Reject for unsupported protocols) | both edit `keepaliveLoop` | coordinate: IPv6CP is no longer "unsupported" once this lands; the Protocol-Reject path stays theirs |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | PPPoE sessions against IPv4-only ACs if NCP independence fails; LAN hosts keep addresses past the delegation |
| How is it reverted? | single commit revert per phase; config leaves are new, so existing configs are unaffected |
| Who else touches this path? | children 12, 13, 17 (PPPoE client); `spec-ppp-l2tp-aaa-one-off-defects`; the DHCPv6 client umbrella; RA options spec |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `pppoe-client ... ipv6` config | → | client IPv6CP negotiation | `test/parse/pppoe-client-ipv6.ci` and interop `pppoe-ipv6cp-ze-client` |
| `pppoe-client ... ipv6 dhcpv6 pd` | → | DHCPv6 client on `<wan>` | interop `pppoe-dhcpv6-pd-ze-client` |
| LAN unit `ipv6 delegated-prefix <wan>` | → | /64 install, RA PIO/RIO and the Router Lifetime matrix | interop `pppoe-dhcpv6-pd-ze-client` (LAN host SLAAC; Router Lifetime read on the LAN before and after the session ends) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | AC offers IPv6CP; client `pppoe0` configured with `ipv6` | the client's Configure-Request carries exactly one Interface-Identifier option, non-zero; IPv6CP reaches Opened; device `pppoe0` holds `fe80::<negotiated id>/64`, and no `pppN` device carries it |
| AC-2 | AC Naks the identifier with a suggestion | the client's next Configure-Request carries the suggested identifier (RFC 5072 Section 4.1) |
| AC-3 | AC rejects IPv6CP (Protocol-Reject) or ignores it | IPCP stays Opened and IPv4 traffic continues; no LCP termination |
| AC-4 | IPv6CP closes while IPCP is Opened | IPv4 traffic continues |
| AC-5 | AC sends RAs on the session | `pppoe0` has accept_ra 2 with forwarding on; a default route via the AC's link-local through `pppoe0`; a SLAAC address when the RA carries an autonomous prefix |
| AC-6 | `dhcpv6 pd length 56` on the PPPoE client | Solicit on `pppoe0` carries IA_PD with a /56 hint; the binding is published with its valid and preferred lifetimes; a different delegated length is accepted (WPD-3) |
| AC-7 | `ipv6 delegated-prefix pppoe0 subnet-id 1` on the LAN unit, no `interface-identifier` | the unit holds `<delegated>:1::1/64`, preferred and valid lifetimes never above the delegation's remaining ones; with `interface-identifier ::2` it holds `<delegated>:1::2/64` instead |
| AC-8 | same | the unit's RAs carry a PIO for the /64 with A and L set and lifetimes bounded the same way, and a RIO for the delegated prefix |
| AC-9 | each of the four rows of the Router Lifetime condition matrix, on a CE-role unit | the unit's RAs carry the Router Lifetime that row names: the configured value for W yes and P yes, 0 for the other three; for W no and P yes the PIO and RIO are still sent (L-3) |
| AC-10 | Renew returns shorter lifetimes | the installed and advertised lifetimes shrink on the next RA |
| AC-11 | the delegation expires, or the session ends | the PIO is advertised with zero lifetimes and the address removed; the unit sends an RA with Router Lifetime 0 at once, not at the next periodic interval (matrix Transitions), and keeps 0 until W and P are both true again |
| AC-14 | the WAN default route is withdrawn while the session stays up and the delegation remains | the unit sends an RA with Router Lifetime 0 at once (G-5) and keeps the PIO and RIO (L-3); when `::/0` through `pppoe0` returns, the configured Router Lifetime comes back |
| AC-15 | a unit with `router-advertisement` and no `delegated-prefix` | its Router Lifetime is the configured `router-lifetime` whatever the WAN state, as today |
| AC-16 | config with `subnet-id 256` beside `pd length 56`, or two units naming `pppoe0` with the same `subnet-id`, or a `delegated-prefix` naming a name with no `pppoe-client` and `dhcpv6 pd` | config verify refuses, naming the largest valid id, both units, or the unknown source, with the fix |
| AC-12 | client shutdown with a delegation in use | no Release for the delegated prefix while LAN addresses derived from it are outstanding |
| AC-13 | a LAN host (dhcpcd) on the LAN unit | it autoconfigures from the /64 and pings an address on the AC side over IPv6 through Ze |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `ipv6` on the PPPoE client | config → pppoe-client → IPv6CP → `<wan>` link-local → RA → default route | `pppoe-ipv6cp-ze-client` |
| 2 | Configures PD and names the delegation on the LAN | DHCPv6 on `<wan>` → event → LAN /64 → RA → LAN host SLAAC → IPv6 traffic | `pppoe-dhcpv6-pd-ze-client` |
| 3 | Loses the WAN session | session end → `::/0` through `<wan>` withdrawn → immediate LAN RA with Router Lifetime 0 → LAN host drops Ze as default router | `pppoe-dhcpv6-pd-ze-client` (terminate step) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC5072ClientConfigureRequestOneIdentifier` | `internal/component/l2tp/pppoeclient/rfc5072_ipv6cp_client_test.go` | AC-1 | |
| `TestRFC5072ClientAdoptsNakSuggestion` | same | AC-2 | |
| `TestRFC7084WLL3NCPIndependence` | `internal/component/l2tp/pppoeclient/rfc7084_ncp_independence_test.go` | AC-3, AC-4 | |
| `TestRFC5072ClientLinkLocalFromIdentifier` | `rfc5072_ipv6cp_client_test.go` | AC-1 address | |
| `TestDelegatedSubnetDerivation` | `internal/component/iface/delegated_prefix_test.go` | AC-7: `<delegated> + subnet-id` and `interface-identifier`; a subnet id outside a shorter-than-hinted delegation yields no /64 and the log line | |
| `TestDelegatedPrefixVerify` | `internal/component/iface/delegated_prefix_test.go` | AC-16: subnet id above the `pd length` range, duplicate source and subnet id, unknown source | |
| `TestRFC9915DelegatedLifetimeBound` | `internal/component/iface/rfc9915_delegated_lifetime_test.go` | AC-7, AC-10 with a fake clock | |
| `TestRFC7084L3RouteInformationOption` | `internal/core/ndp/rfc7084_rio_test.go` | AC-8 RIO encoding | |
| `TestRFC7084RouterLifetimeMatrix` | `internal/plugins/iface/ra/rfc7084_router_lifetime_test.go` | AC-9: one case per matrix row, and AC-15 (no source, configured value kept) | |
| `TestRFC7084G5ImmediateZeroLifetime` | `internal/plugins/iface/ra/rfc7084_router_lifetime_test.go` | AC-11, AC-14: a W or P true-to-false change sends a zero-lifetime RA before the periodic timer fires, PIO and RIO kept while P holds | |
| `TestRFC7084G4RouterLifetimeFollowsWANDefault` | `internal/plugins/iface/ra/rfc7084_router_lifetime_test.go` | A-5, AC-14: `routewatch` add and remove of `::/0` through `<wan>` drive W; a `::/0` through another device does not | |
| `TestRouteEventCarriesOutputDevice` | `internal/core/routewatch/routewatch_test.go` | the added output interface index reaches a registered handler | |
| `TestRFC9915NoReleaseWhileDelegated` | `internal/plugins/iface/dhcp/rfc9915_dhcp_v6_release_linux_test.go` | AC-12 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `subnet-id` (config, beside `pd length 56`) | 0 to 2^(64 - 56) - 1 | 255 | N/A (uint16) | 256 |
| `subnet-id` (config, no `pd length`) | 0 to 65535, the /48 maximum | 65535 | N/A | 65536 (type) |
| `pd length` | 48-64 per existing leaf | 64 | 47 | 65 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pppoe-client-ipv6` | `test/parse/pppoe-client-ipv6.ci` | the new `ipv6` leaves on `pppoe-client` and the `delegated-prefix` list parse and validate | |
| `iface-delegated-prefix-unknown-source` | `test/parse/iface-delegated-prefix-unknown-source.ci` | a LAN unit naming a source with no `pppoe-client` and `dhcpv6 pd` is refused with the fix (AC-16) | |
| `iface-delegated-prefix-subnet-id-invalid` | `test/parse/iface-delegated-prefix-subnet-id-invalid.ci` | a `subnet-id` above the `pd length` range, and two units with the same source and `subnet-id`, are refused naming the limit and both units (AC-16) | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `pppoe-ipv6cp-ze-client` | `test/interop-pppoe/scenarios/pppoe-ipv6cp-ze-client/`, checker `checkZeClientIPv6CP` (`internal/le/interoplab/pppoe/check_ipv6cp_client.go`) | accel-ppp, which Naks every Interface-Identifier but its own `ipv6-peer-intf-id` 0:0:0:5a | AC-1, AC-2, AC-5: Ze adopts the Nak's identifier and opens IPv6CP, `pppoe0` holds fe80::5a, RA default route and SLAAC address, IPv6 ping to accel-ppp. Red walks: `receiveNak` keeps `c.local` instead of adopting the suggestion (red "IPv6CP never opened on the identifier accel-ppp suggested"); and both IPv6CP starts broken, `ipv6.start` in `runIPCP` and `c.start` in `clientIPv6CP.receiveRequest` (red, same message). That second walk proves Ze never opens IPv6CP at all, which is wider than the planned break: breaking `negotiateIPCP`'s start alone was off-path, because accel-ppp with `ipv6=prefer` opens IPv6CP actively and `receiveRequest` starts Ze's side (Deviations) | green 2026-11-08 after fdae5b498e and 36c64b0804 (`tmp/session/2026-11-07-104251/scratch/rwinv/c4c8-ipv6cp-clean.log`); reds `rwinv/c4c8-ipv6cp-nak-not-adopted.log`, `rwinv/c4c8-ipv6cp-never-started.log` |
| `pppoe-ipv6cp-rejected-ze-client` | `test/interop-pppoe/scenarios/pppoe-ipv6cp-rejected-ze-client/`, checker `checkZeClientIPv6CPRejected` | accel-ppp with IPv6 denied, which Protocol-Rejects IPv6CP | NCP independence (AC-3): one Protocol-Reject, no LCP Terminate-Request, one CHAP Success, IPv4 ping after the reject, no link-local address. Red walk: the `ppp.LCPProtocolReject` arm of `runIPCP` (session.go) no longer stops IPv6CP, red "accel-ppp sent 6 Protocol-Rejects for IPv6CP, want 1: Ze MUST stop sending IPv6CP after the first (RFC 1661 Section 5.7)". The planned break in `keepaliveLoop` was off-path, because IPv6CP now opens beside IPCP and the reject arrives during `runIPCP` | green 2026-11-08 (`rwinv/c4c8-ipv6cp-rejected-clean.log`); red `rwinv/c4c8-ipv6cp-protocol-reject.log` |
| `pppoe-dhcpv6-pd-ze-client` | `test/interop-topology/scenarios/` (suite and verb from child 18; this spec owns the scenario) | accel-ppp, dhcpcd | PD on `pppoe0`, LAN /64, RA PIO and RIO, LAN host traffic; after `accel-cmd` terminates the session, the next LAN RA carries Router Lifetime 0 and dhcpcd drops Ze as default router. Red walks: lifetime bound removed (advertised valid lifetime above remaining); W input forced true (Router Lifetime stays non-zero after the session ends) | green 2026-11-08 in the full topology run (`rwinv/topo-full-ac12.log`, 18 passed). The checker (`checkPPPoEDHCPv6PD`, then `checkDelegatedLAN` in `internal/le/interoplab/topology/check_pppoe_dhcpv6_pd_lan.go`) judges AC-1, AC-5, AC-6 on `pppoe0`, then on `lan0` the unit's address (AC-7), RA lifetimes bounded by the delegation (RFC 9915 Section 6.3), dhcpcd's SLAAC address and RIO route and an IPv6 ping to accel-ppp (AC-8, AC-13), and the Router Lifetime 0 RA after `accel-cmd terminate` (G-5, AC-11). Its accel-ppp runs `ipv6=allow`, which holds IPv6CP passive, so the scenario also judges that Ze opens IPv6CP beside IPCP (journal `ncp-started-behind-another-ncp.md`). Reds: lifetime bound removed (`scratch/rw8-lifetime.log`: "advertises a PIO valid lifetime of 120s, 1.9s after accel-ppp's Reply granted 120s, so above the 118.1s that remained"); W forced true (`scratch/rw8-w.log`: "no RA with Router Lifetime 0 since the session was terminated") |

AC-4 (IPv6CP closes while IPCP is Opened) has no interop form: accel-ppp ends the whole session when an IPv6CP it started finishes (`accel-pppd/ppp/ppp_ipv6cp.c`, `ipv6cp_layer_finished` calls `ap_session_terminate` once `ipv6cp->started`), so no accel-ppp run can close IPv6CP alone. `TestRFC7084WLL3NCPIndependence` proves AC-4 by driving the close through the automaton and the session glue.

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - `ipv6` grouping shared by unit and `pppoe-client`; `ipv6 delegated-prefix` list on the unit (`source`, `subnet-id`, `interface-identifier`)
- `internal/component/l2tp/pppoeclient/session.go`, `network_phase.go` - IPv6CP negotiation and Opened handling
- `internal/component/l2tp/ppp/ipv6cp.go` - codec exported from package `ppp` for the client
- `internal/plugins/iface/dhcp/dhcp_v6_linux.go` - DHCPv6 on `<wan>`; preferred lifetime and receive instant in the payload; no Release while delegated
- `internal/component/iface/iface.go` - `DHCPPayload` gains the preferred lifetime and receive instant
- `internal/component/iface/config_ra.go`, `internal/plugins/iface/ra/sender_linux.go`, `internal/core/ndp/ra.go` - derived prefixes, RIO, Router Lifetime per the matrix and its immediate RA
- `internal/core/routewatch/routewatch.go`, `routewatch_linux.go` - `RouteEvent` carries the output interface index
- `docs/architecture/core-design.md` "Shared Route Watcher" (the design page `routewatch.go` declares) - wrong after this change: it says each event carries "the kernel table and priority"; it gains the output device and the RA side as a third registered consumer beside `fib-kernel` and `kernel`
- `rfc/short/rfc5072.md`, `rfc/short/rfc7084.md`, `rfc/short/rfc9915.md` - rows moved from gap or declined to tested
- `docs/guide/pppoe.md`, `docs/guide/configuration.md`
- `docs/architecture/l2tp/cpe-1-pppoe-client.md` - wrong after this change: its opening says the client negotiates "LCP, authentication and IPCP", and its network-phase paragraph describes IPCP as the only NCP the unit reader handles; both gain IPv6CP negotiation and its Opened handling, with source anchors on the new client code
- `docs/features/interfaces.md` - wrong after this change: the Capability Table row "Route Information option (RFC 4191)" reads `missing`, and the DHCPv6 paragraph says nothing of a delegated prefix being used on a LAN unit or of DHCPv6 on the PPPoE client's device; both are updated
- `docs/research/l2tpv2-ze-integration.md` section 9.5 (the design page `ppp/ipv6cp.go` declares) - names the codec functions `parseIPv6CPOptions` and `suggestIPv6CPInterfaceID`; if exporting the codec renames them, the names there follow; its LNS-side behavior is unchanged
- `features/pppoe-access.md` (PPPoE client IPv6), `features/interfaces.md` (DHCPv6 on `<wan>`, delegated-prefix use), `features/ipv6-router-advertisements.md` (RIO and the CE Router Lifetime matrix)

## Files to Create
- `internal/component/iface/delegated_prefix.go` and tests - the LAN consumer
- `internal/component/l2tp/pppoeclient/ipv6cp_client.go` and tests
- the interop scenario directories above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | Yes | `ze-iface-conf.yang` |
| YANG validation constraints | Yes | `subnet-id` uint16, `pd length` reused |
| YANG custom validators | Yes | the `delegated-prefix` source must name a configured `pppoe-client` with `dhcpv6 pd`; `subnet-id` within the source's `pd length`; one unit per source and `subnet-id`; completion of names |
| CLI commands/flags | Yes | `show interface` shows the delegated prefix and derived /64 |
| CLI grammar | Yes | keyword before value |
| Editor autocomplete | Yes | `CompleteFn` for the source name |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | Yes | show output through pipes |
| Env var registration | N-A | none |
| Doctor check | N-A | no new runtime dependency |
| Prometheus counters/metrics | Yes | IPv6CP state per session, delegated prefix present |
| BGP family surface | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `features/pppoe-access.md`, `features/interfaces.md`, `features/ipv6-router-advertisements.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/pppoe.md` |
| 7 | Wire format changed? | Yes | IPv6CP diagram in the client source with RFC 5072 offsets; RIO in `ra.go` |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc5072.md`, `rfc7084.md`, `rfc9915.md`, `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` IPv6 CPE row |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | Yes | `docs/guide/monitoring.md` |
| 15 | Registered inventory changed? | No | |
| 16 | Changed source referenced by doc anchors? | Yes | derived by `./le spec citation anchors` at implementation |
| 17 | Existing docs show examples for this area? | Yes | `pppoe.md` examples |

## Implementation Steps

Prerequisite: child 1 has landed (the client's device carries its configured
name). Phases 1 to 3 need only that. Phases 4 and 5 are in scope by the owner
ruling, and take OD-1 (a) and OD-2 (a). Closure also needs child 18's `topology`
suite, because `pppoe-dhcpv6-pd-ze-client` runs in it.

1. **Phase: Wiring (MANDATORY FIRST)** -- YANG `ipv6` grouping on `pppoe-client`, parse test, interop scenario skeleton failing at IPv6CP
2. **Phase: IPv6CP client** -- AC-1 to AC-4; RFC 5072 quotes above each implementing function
3. **Phase: WAN RA, SLAAC and DHCPv6 on `<wan>`** -- AC-5, AC-6
4. **Phase: LAN /64 and RA** -- AC-7 to AC-11, AC-14 to AC-16, the `delegated-prefix` list and the Router Lifetime matrix
5. **Phase: No Release, rows and docs** -- AC-12, AC-13; RFC rows recorded with `./le rfc record`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | lifetimes computed from the Reply's receive instant, not the configured RA values; every Router Lifetime decision cites the matrix, and no section restates a different condition |
| Data flow | the DHCP plugin configures no LAN address itself; iface consumes the event |
| Rule: no-layering | one IPv6CP codec; one `ipv6` grouping |
| Rule: rfc-compliance | every implementing function quotes its RFC sentence |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| IPv6CP client | unit tests and `pppoe-ipv6cp-ze-client` |
| PD to LAN | `pppoe-dhcpv6-pd-ze-client` green with red walk |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | IPv6CP option lengths; a delegated prefix shorter than /48 or longer than /64 refused for LAN derivation |
| Resource exhaustion | repeated Configure-Naks bounded by the existing max-configure counter |

## Owned journal rows

One owned row, below. Related: `plan/journal/silent-fall-through.md` row 2026-10-31 (PPPoE client drops IPv6CP, CCP and unknown protocols), owned by `spec-ppp-l2tp-aaa-one-off-defects`; this spec removes IPv6CP from that row's set when it lands.

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| ncp-started-behind-another-ncp.md | 2026-11-08 | PPPoE client starts IPv6CP only after IPCP opened, so accel-ppp ipv6=allow activates IPv4-only and never runs ND or DHCPv6 (pppoe-dhcpv6-pd-ze-client red) |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Reuse the server's IPv6CP codec | a client-only codec | one implementation of RFC 5072 options |
| Share the unit's `ipv6` grouping on `pppoe-client` | give `pppoe-client` a unit | the PPPoE interface has no VLAN or unit semantics; a grouping avoids a second copy of the leaves |
| LAN consumer in iface, fed by the published event | the DHCP plugin installs LAN addresses | keeps the DHCP plugin from configuring other interfaces; matches the existing publication |
| RIO only for the delegated prefix | wait for the RA options spec | L-3 is a MUST of the CE role this spec implements |
| W derived from the delegated-prefix source's `::/0`, read through `routewatch` | an operator trigger route as BIRD's `trigger`; a second netlink subscription | G-4 is the CE default, so no option is needed; `routewatch` is the one shared route subscription |
| The device named by its configured name (child 1) | look up the `pppN` unit at run time | child 1's contract; a `pppN` name changes on every reconnect |

## Known Limitations

- ULA generation (RFC 7084 ULA-1 to ULA-5) and a LAN DHCPv6 server (L-10) are not in this spec; they stay gaps in `rfc/short/rfc7084.md` for the RA options and DHCPv6 server specs.
- Delegation to further routers behind the CPE is out of scope.
- A `delegated-prefix` source is a `pppoe-client` only. An Ethernet WAN running `dhcpv6 pd` on a unit is not a source in this spec; `source` is a configured interface name, so widening the validator later changes no syntax.
- L-13 (`RFC7084-4.3-13`, advertising a replaced delegated prefix with a zero Preferred Lifetime) is not in this spec's RFC gaps; it stays a gap in `rfc/short/rfc7084.md`.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

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

### Goal Gates (MUST pass)
- [ ] AC-1..AC-16 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] Interop red walks recorded

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output): the scoped package tests of `internal/component/l2tp/pppoeclient`, `internal/component/iface`, `internal/core/ndp`, `internal/plugins/iface/ra`, `internal/plugins/iface/dhcp` and `internal/core/routewatch` under `./le job run`, `./le test functional parse` for the three parse tests, the interop scenarios through `./le test deployment docker-pppoe-accel` and `./le test integration interop-topology pppoe-dhcpv6-pd-ze-client`, then `./le verify changed` over the closing commits

## Blocker

Closure was attempted on 2026-11-08 and handed back before commit A. The code
is committed (2bebae34b5, ee9d52da6e, 5aa5b17adb, 6e7ba09a0f, 7361ba2192,
a5adb4ca2c, 2652d989f8, a02374d9ad and the interop-topology commits), all under
`Spec-work: home-cpe-1-logical-interface-name`, so no review artifact names
this spec. Five things block commits A and B:

| # | What blocks | Producer | What clears it |
|---|-------------|----------|----------------|
| 1 | The two parse red walks never ran. `iface-delegated-prefix-unknown-source` and `iface-delegated-prefix-subnet-id-invalid` carry `skip-os=darwin`, so on the Mac both the broken and the clean runs SKIP (`rwinv/home-cpe-8-*-parse/run.log`) | `verifyDelegatedEntry` (`internal/component/iface/delegated_prefix.go`), its unknown-source and subnet-id refusals | On Linux (the Ze-kernel QEMU guest): `GOFLAGS=-overlay=<checkout>/tmp/session/2026-11-07-104251/scratch/rwinv/home-cpe-8-unknown-source-parse/overlay.json ./le job run label c8parse command bin/le test bgp parse iface-delegated-prefix-unknown-source` and the same with `home-cpe-8-subnet-id-parse` and `iface-delegated-prefix-subnet-id-invalid`, each wanting FAIL; then both, and `pppoe-client-ipv6`, without the overlay, wanting pass. The overlay files name host paths under `/Users/thomas/...`, so they are rewritten for the guest mount |
| 2 | The Review Gate has not run. No `/ze-review` covers the implementation, about 8,000 changed lines | the commits above | A fresh closure agent runs `/ze-review` over those commits and records `./le spec review record` over the files commit A carries |
| 3 | Two Integration Checklist rows marked Yes are not implemented: Prometheus metrics for the IPv6CP state per session and for a delegated prefix being present (`internal/component/l2tp/pppoeclient/metrics.go` registers only `ze_pppoe_client_renegotiations_total`), and editor completion of the `delegated-prefix source` name (the `source` leaf in `ze-iface-conf.yang` carries no completion) | `metrics.go`; `list delegated-prefix` | Implement both, with `docs/guide/monitoring.md`, or home each in a spec the Work Not Done table names |
| 4 | Pages the checklist names were not updated: `features/ipv6-router-advertisements.md` and `features/interfaces.md` do not mention the Route Information option, the CE Router Lifetime matrix or delegated-prefix use; `docs/comparison.md` has no IPv6 CPE row. `features/pppoe-access.md` declares the access concentrator (Components `internal/component/l2tp/pppoe`), not the client, so the client's IPv6 has no feature declaration; `docs/guide/pppoe.md` documents it | the feature declarations | Edit the declarations and decide the comparison row in the closure run, with `./le doc check all` |
| 5 | `./le rfc check` (2026-11-08, `scratch/c8close-rfccheck.log`) reports, on stems this spec touched, a STALE audit verdict for `RFC5072-3-1` (its test `TestNCPHeldUntilNetworkPhase` changed in 14d3df65a7, `Spec-work: ppp-l2tp-aaa-one-off-defects`) and a producer-changed record for `RFC9915-21.6-3` (`dhcp_v6_linux.go` `effect` changed in 7854ccf79b, `Spec-work: home-cpe-23-dns-source-fall-through`). Neither is this spec's tag, and every RFC 7084, 9915 and 5072 unit this spec tagged verifies | the commits named | The owning specs re-take them; this closure only checks they are not its own |

Evidence already gathered for the closure run: the interop rows above (green
runs and reds, each with its log); the tagged-unit records re-taken 2026-11-08
(`scratch/rw8rec/rec.log`, `rwinv/home-cpe-8-rfc*`); and the owned journal row
in `ncp-started-behind-another-ncp.md`, which is `fixed:`. Deviation to record
at closure: the `ipv6cp-never-started` walk breaks both IPv6CP starts and
proves "Ze never opens IPv6CP", wider than the single `negotiateIPCP` break
planned. The passive-peer case, where Ze must open IPv6CP itself, is the
`ipv6=allow` accel-ppp of `pppoe-dhcpv6-pd-ze-client`. That scenario has no
logged red walk for the old order, where IPv6CP started after IPCP opened:
the journal row states the red, but no log exists.

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A** and **Commit B** per `ai/rules/planning.md`

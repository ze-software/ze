# Spec: home-cpe-4-ike-dynamic-wan

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P1 |
| Release reason | Shipped IKE initiator does the wrong thing on a dynamic WAN: a peer whose local address comes from `interface` never comes up when the address appears after boot and never follows a new address; a peer with neither `interface` nor `local-address` cannot build a Child SA; a DNS-named remote sends no NAT detection. Each configuration is accepted and silently fails. |
| RFC gaps | - |
| Depends | spec-home-cpe-1-logical-interface-name.md (only for `interface` naming a PPPoE logical unit) |
| Phase | 0/4 |
| Handoff | - |
| Updated | 2026-11-02 |

Parent: plan/release/fixes/spec-home-cpe-0-umbrella.md

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The home CPE dials `vpn.example.net` from a PPPoE WAN whose address is assigned
by the LNS, can change on every reconnect, and does not exist at boot. Four
behaviours are owed:

1. **Local address follows the bound interface.** With `vpn ipsec interface`
   set, the IKE listeners and every peer without its own `local-address` follow
   that interface's current IPv4 address: peers start when the address appears
   (including after a boot that found none), re-home when it changes, and stop
   (removing their Child SAs) when it disappears.
2. **Route-chosen local address.** A peer with no `local-address` while
   `vpn ipsec interface` is unset uses the source address the kernel's route to
   the remote selects, resolved at the start of every connect cycle. This is
   VyOS's `local-address any`. Today such a peer is accepted and cannot build a
   Child SA (`newChildSA` refuses the empty address).
3. **NAT detection with a DNS-named remote.** The address the cycle resolved is
   the remote address for every reader: the NAT_DETECTION payloads Ze sends, the
   check of the responder's payloads, the Child SA endpoints, the wildcard
   selector family, transport-mode host pairs, and a `respond` peer's match of an
   unsolicited IKE_SA_INIT.
4. **Keep retrying while the name does not resolve**, with the existing backoff,
   and establish once it does (a CPE booted while DNS is down).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/access-security.md` - IKE door
  → Constraint: "A refusal added for reload also ran at startup, so the IPsec config apply ... skipped every IKE transport when the WAN came up late" (door, known traps). Startup applies a configuration whose interface has no address and warns; this spec keeps that and adds the bring-up.
  → Constraint: a component reaches interface state through the event bus, never by importing the iface plugin.
- [ ] `docs/guide/ipsec.md` "Which edits restart a tunnel" - a reload whose interface has no IPv4 address is refused, startup warns
  → Decision: that reload refusal stays; an address that arrives later is not a reload and goes through the new event path.
- [ ] `docs/architecture/ike/ipsec-7-ikev2-engine.md` - peer sessions, reconnect loop
- [ ] `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md` - NAT-T and NAT detection
- [ ] `plan/release/fixes/spec-ipsec-11-mobike.md` - MOBIKE (RFC 4555), Status design, P1
  → Decision: MOBIKE owns moving an ESTABLISHED SA to a new address (UPDATE_SA_ADDRESSES, `serviceMobike`'s route-source poll). This spec owns peer lifecycle around an address (bring-up, rebind, restart, the route-chosen source of a NEW cycle). With a wildcard listener and MOBIKE negotiated, this spec leaves the established SA to MOBIKE and restarts nothing; with a listener bound to the old address, MOBIKE cannot help because the socket itself must move (`rebindListeners` comment), and the peer restarts.
- [ ] `plan/release/fixes/spec-rfcgate-1b-rfc7296-pilot-deferred-ike-source-address.md` - Status blocked on the owner's question OR-WP8-1
  → Decision: not absorbed, not a dependency. The route-chosen mode binds the wildcard, which is already the listener of every configuration without `interface` (`ikeListenHost`). On a single-address CPE the reply source equals the request's destination. On a multi-homed host it inherits 1b's open gap, and the guide says so. 1b stays the owner of the Section 2.11 address half.
- [ ] `plan/release/fixes/spec-ipsec-close-action-and-dpd-action-select-nothing.md` - DPD and close actions
  → Constraint: a peer stopped because its address vanished is not a DPD or close event; it waits for the address, it does not apply `close-action`.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - present
  → Constraint: RFC 7296 Section 2.23: "Both the IKE initiator and responder MUST include in their IKE_SA_INIT packets Notify payloads of type NAT_DETECTION_SOURCE_IP and NAT_DETECTION_DESTINATION_IP." (inside the section's NAT-T scope: "In this section only, requirements listed as MUST apply only to implementations supporting NAT traversal.") Ze supports NAT-T, so a DNS-named peer that sends none violates it.
  → Constraint: RFC 7296 Section 2.23: "There MAY be multiple NAT_DETECTION_SOURCE_IP payloads in a message if the sender does not know which of several network attachments will be used to send the packet." The route-chosen source is known before the send, so one payload is sent.
  → Constraint: RFC 7296 Section 2.11: "It MUST specify the address and port at which the request was received as the source address and port in the response." Unchanged here; owned by rfcgate-1b.
- [ ] `rfc/short/rfc4555.md` - present
  → Constraint: RFC 4555 Section 3.5: "Typically, triggers include information received from lower layers, such as changes in IP addresses or link-down indications." The interface address event is that trigger; MOBIKE consumes it through its own path.
- [ ] `rfc/short/rfc3948.md` - present (UDP encapsulation once NAT is detected)

**Key insights:**
- The local address is resolved once, in `applyConfig`; nothing re-resolves it.
- The remote name is resolved per cycle in `runOnce`, but the result never reaches the readers that call `net.ParseIP(RemoteAddress)`.
- The interface address bus events exist (`interface/addr-added`, `addr-removed`) and OSPF already consumes them with a long-lived worker.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/apply.go` `applyConfig` - resolves `cfg.Interface` once through `resolveInterfaceAddrFn`; on a reload with no address returns the `unbindablePeers` refusal; at startup warns and continues; fills `LocalAddress` of peers that name none only when an address was found; then `rebindListeners(ikeListenHost(ifIP, peerLocal), ...)`.
- [ ] `internal/component/ike/engine/apply.go` `rebindListeners`, `ikeListeners.open` - ONE socket pair (`listeners.ike`, `listeners.natt`) serves every peer. When the host moves, `rebindListeners` stops EVERY peer (`stopAllPeers`), closes both sockets and opens new ones, because `PeerSession.ike` and `PeerSession.natt` are immutable after `startPeerSession` and a surviving session would hold a closed descriptor (its doc comment). Reached only from `applyConfig`. A literal-`local-address` peer therefore shares, and loses, the interface-bound socket.
- [ ] `internal/component/ike/engine/testport.go` `ikeListenHost` - interface address when known, otherwise `0.0.0.0`. A startup with an address-less interface therefore listens on the wildcard while its peers keep an empty `LocalAddress`.
- [ ] `internal/component/ike/engine/register.go` `resolveInterfaceAddr` - first IPv4 address of the logical interface through the iface resolver; distinguishes lookup failure from "no IPv4 address".
- [ ] `internal/component/ike/engine/fsm.go` `runOnce` - `net.ResolveUDPAddr("udp4", ikeAddr(peer.RemoteAddress))` per cycle; the resolved address is used for the send only.
- [ ] `internal/component/ike/engine/fsm.go` IKE_SA_INIT response handling - the NAT_DETECTION_SOURCE_IP check runs only when `net.ParseIP(sa.PeerCfg.RemoteAddress)` is non-nil, the DESTINATION_IP check only when `LocalAddress` parses.
- [ ] `internal/component/ike/engine/initiator.go` IKE_SA_INIT request build - NAT_DETECTION payloads added only when both configured strings parse as IPs: a DNS name or an empty local sends none.
- [ ] `internal/component/ike/engine/create_child.go` `newChildSA` - endpoints from the configured strings, overridden by the MOBIKE path or `peerTunnelAddress(sa)` (the authenticated peer endpoint) for tunnel mode; refuses a nil local.
- [ ] `internal/component/ike/engine/rekey.go` `anyChildTSPayloads` - wildcard family from `ParseIP(RemoteAddress)`.
- [ ] `internal/component/ike/engine/ts_nat_substitute.go`, `transport_mode.go`, `inner_selectors.go` `configuredHostPair` - read the configured strings; a DNS name yields nil.
- [ ] `internal/component/ike/engine/register.go` `matchResponderPeer` - compares the configured remote string to the datagram source, character for character.
- [ ] `internal/component/ike/engine/fsm.go` `reconnectDelay` - 1 s doubling to 60 s on `connectFailures`; `PeerSession.run` never gives up.
- [ ] `internal/component/ike/engine/mobike_route_linux.go` `routeSourceTo` - asks the routing table with RTM_GETROUTE (`netlink.RouteGet`) and returns the first non-unspecified `Src`; it never connects a socket. Its comment gives the reason: a UDP connect is an output route lookup the kernel runs through the XFRM SPD, the ephemeral port is named by no IKE bypass, so under Ze's catch-all DISCARD (the default, RFC 4301 Section 4.4.1) the connect fails with EPERM. `mobike_route_other.go` `routeSourceTo` (off Linux, no SPD) uses a connected UDP socket. Its one caller today is `mobike.go` `mobikeRouteSource`, which first keeps the SA's current MOBIKE local address while the host still holds it, and runs only for an established MOBIKE SA.
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - leaves `interface`, `local-address` (a string read with `net.ParseIP`, no validation), `remote-address` (IPv4 or DNS name, `udp4`).
- [ ] `internal/component/iface/iface.go` `AddrPayload` and `internal/core/iface/events/events.go` - `interface/addr-added`, `interface/addr-removed` with name, unit, index, address, prefix length, family.
- [ ] `internal/plugins/ospf/transport/transport.go` - subscribes to both events and feeds a bounded queue drained by one worker goroutine.
- [ ] `internal/component/ike/engine/register.go` doctor checks (`ipsec-cookie-threshold`, `ipsec-virtual-ip-pool`, each with a code in `internal/core/diagnostic/codes.go`) and `doctor_virtual_ip.go` `checkIPsecVirtualIPPool` - the pattern a new IPsec doctor check follows: a check function reading engine runtime state, registered in the plugin's `Registration`, reporting nothing with no engine running.

**Behavior to preserve:**
- A reload whose interface has no IPv4 address is refused (`unbindablePeers`); startup warns and continues.
- A peer with its own `local-address` literal is untouched by interface events (AC-5; today this holds only by accident of timing, see `rebindListeners` above, and the listener set below makes it true).
- Port 500 binds the interface address when `interface` is set; 4500 binds the wildcard when live XFRM migration is available (`ikeListeners.open`).
- The backoff schedule.
- Remote IPv6 IKE stays unsupported (`udp4`).

**Behavior to change:**
- Interface address events drive bring-up, re-home and stop.
- A peer with no local address and no `interface` resolves its source per cycle.
- The resolved remote replaces the configured string for every reader listed above.
- `local-address` accepts only an IP literal; anything else is refused with the fix in the message.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Interface address at apply | `internal/component/ike/engine/apply.go` `applyConfig`, `register.go` `resolveInterfaceAddr` | Resolves the interface's first IPv4 address once per apply through the iface resolver; refuses an unbindable reload, warns at startup | extends: the address worker re-runs the same resolution on each interface address event |
| Listener rebind | `apply.go` `rebindListeners`, `ikeListeners`, `testport.go` `ikeListenHost` | One socket pair for every peer; stops every peer, closes and reopens both sockets when the host moves; reached only from `applyConfig` | extends: the single pair becomes a listener set keyed by local origin (see "Listener set"); `rebindListeners` moves only the interface pair and stops only the sessions that hold it; the address worker calls it, no second rebind path |
| Interface address events | `internal/component/iface/iface.go` `AddrPayload`, `internal/core/iface/events/events.go` (`interface/addr-added`, `interface/addr-removed`); `ai/doors/forwarding.md` "Boundaries" | Published on the EventBus with name, unit, index, address, prefix length, family | extends: the engine subscribes, the way OSPF's transport does (`internal/plugins/ospf/transport/transport.go`, one bounded queue drained by one worker) |
| Route-source lookup | `internal/component/ike/engine/mobike_route_linux.go` `routeSourceTo` (RTM_GETROUTE through `netlink.RouteGet`, no socket connect, so it works under the SPD catch-all DISCARD); `mobike_route_other.go` off Linux; caller `mobike.go` `mobikeRouteSource` | Returns the routing table's source toward the peer; called only for an established MOBIKE SA | extends: called as is, with no extraction, at the start of each connect cycle of a peer with no local address and no `interface`; its error text and its "the one caller" comments stop naming MOBIKE alone |
| Authenticated peer endpoint | `internal/component/ike/engine/create_child.go` `peerTunnelAddress`; `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` "Transport-mode NAT substitution changes the selectors, not the outer addresses" | Tunnel mode takes the outer remote from the authenticated endpoint; other readers take the configured strings | extends: the resolved pair replaces the configured strings for every other reader listed in Current Behavior |
| Initiator reconnect backoff | `internal/component/ike/engine/fsm.go` `reconnectDelay`, `PeerSession.run` | 1 s doubling to 60 s, never gives up | does not apply: preserved as is |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `vpn ipsec interface <name>`, or no `interface` and no `local-address`; `remote-address vpn.example.net`.
- Bus: `interface/addr-added`, `interface/addr-removed` from the iface monitor (PPPoE IPCP completing on `ppp0`, or the address leaving).

### Transformation Path
1. The IKE engine subscribes to both events at start; a bounded queue feeds one long-lived worker (OSPF's pattern).
2. On any IPv4 event the worker re-runs `resolveInterfaceAddrFn(cfg.Interface)` (the event is a trigger, the resolver is the truth, so logical names resolve as they do at apply).
3. Address appeared or changed: the engine moves the INTERFACE socket pair to it, sets it as the local address of the peers that name none, and starts or restarts only those peers. Address gone: those peers stop and remove their Child SAs, the interface pair closes. No other pair is touched (Listener set).
4. Each connect cycle resolves the remote name, then (route-chosen mode) the local source toward it through `routeSourceTo`, and stores both on the SA.
5. Every reader takes the SA's resolved pair: NAT_DETECTION build and checks, `newChildSA`, `anyChildTSPayloads`, transport host pairs, NAT substitution.
6. `respond` peers configured by name: `matchResponderPeer` compares the datagram source against the name's cached resolution, never resolving on the packet path (DNS responder cache).

### Listener set

AC-5 promises a literal-`local-address` peer is never restarted by interface
events, while today's single socket pair makes every session hold the
interface-bound socket. The design keeps AC-5 by giving each local origin its own
socket pair, so a rebind moves only the sockets of the peers whose address moved:

| Local origin of a peer | Socket pair it holds | On an interface address event |
|------------------------|----------------------|-------------------------------|
| `interface` set, peer names no `local-address` | the interface pair, bound to the interface's current address (absent while it has none) | moves, closes or opens; only these sessions stop or restart |
| `interface` set, peer names a literal that differs from the interface's current address | a pair bound to that literal, opened when the first such peer starts and closed when the last one stops | untouched |
| `interface` set, literal equal to the interface's current address | shares the interface pair (one address and port cannot be bound twice) | follows the interface rows: that peer's own address left the host, so it stops with the address and starts when it returns |
| no `interface` (literal or route-chosen) | the wildcard pair, as today | no interface events apply |

Port 4500 follows its pair: with live XFRM migration available it stays the
wildcard as today (`ikeListeners.open`) and is never moved by an interface event;
otherwise each pair carries its own 4500 socket bound like its 500 socket. Every
pair feeds the same SA table through its own `dispatchInbound`. With `interface`
set every socket is bound to a specific address, which is already today's choice
for that case (`ikeListenHost`), so this does not take either route of
`spec-rfcgate-1b-rfc7296-pilot-deferred-ike-source-address.md` (OR-WP8-1) for the
wildcard listener, which stays that spec's owner question.

### DNS responder cache

| Event | Effect on a `respond` peer's cached set |
|-------|-----------------------------------------|
| Apply that adds the peer or changes its `remote-address` | the set is emptied and a resolution is queued on the address worker; until it answers, no unsolicited IKE_SA_INIT matches the peer |
| Successful resolution | the set is replaced by every address of the answer (never merged), with the time of the answer |
| Refresh | every 60 s (the reconnect backoff ceiling; Go's resolver exposes no TTL) and after each apply, on the address worker, never on the packet path |
| Failed resolution | the previous set is kept and the failure is logged with the peer, the name, the resolver error and the fix; the address match only selects which configuration a peer must then authenticate against, so a stale address admits no one IKE_AUTH would not |
| Answer with no IPv4 address | the set becomes empty and the peer matches nothing until a later answer |
| Peer removed, or its `remote-address` becomes a literal | the cache entry is deleted with the peer's session |

strongSwan resolves a named remote per packet (`src/libcharon/config/ike_cfg.c`
`match`, `host_create_from_dns`); Ze does not, by design (R-4).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface ↔ IKE engine | event bus, JSON `AddrPayload` | No |
| IKE engine ↔ resolver | `net.Resolver` per cycle, and per refresh for `respond` peers | No |
| Ze ↔ strongSwan | NAT_DETECTION payloads, port float | No |
| IKE engine ↔ kernel | route-source lookup, RTM_GETROUTE through `netlink.RouteGet`, no socket and no packet | No |

### Integration Points
- `applyConfig`, `rebindListeners` and `ikeListeners` (extended, not duplicated); `reconcilePeers` hands each session the pair of its origin; `runOnce`; `matchResponderPeer`; `routeSourceTo` (called as is).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | Bus events, not an iface import |
| No unintended coupling | Yes | Name resolution stays in the iface resolver |
| No duplicated functionality | Yes | Rebind path and route-source lookup reused |
| Zero-copy preserved where applicable | N-A | Control path |
| Registration over hardcoding, outbound | N-A | No new registered feature |
| Registration over hardcoding, inbound | Yes | No list learns a name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The iface monitor emits `addr-added` for an address the PPPoE client installs on `ppp0` | Netlink monitor maps RTM_NEWADDR (`internal/plugins/iface/netlink/monitor_linux.go` `handleAddrUpdate`, `addrUpdateToEventType`) | Subscribe to the PPPoE client's own up event as well | Functional test with a PPPoE session | unvalidated |
| A-2 | `vpn ipsec interface` naming a PPPoE unit that has no kernel device yet is accepted at commit | YANG: "Commit refuses a name no interface on the host carries" | The boot case needs child 1's logical-name resolution, or the CPE uses route-chosen mode | Parse test with a pppoe-client unit | unvalidated |
| A-3 | A resolve failure increments `connectFailures`, so the backoff grows | `reconnectDelay` reads `connectFailures` | Retry at 1 s forever; fix in this spec | `TestUnresolvableRemoteRetriesWithBackoff` | unvalidated |
| A-4 | strongSwan, with no NAT on the path and Ze sending correct NAT_DETECTION, keeps port 500 until MOBIKE floats it | RFC 7296 Section 2.23 and RFC 4555 | Scenario expectation changes | mgmt-tunnel scenario | unvalidated |
| A-5 | The route-source lookup works with no default route through the WAN (the CPE's PPPoE adds none) | Umbrella: "no default route from PPP"; the static default route via the PPPoE interface is child 1 | Route-chosen mode needs that route; document it | Functional test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Address flap restarts every peer repeatedly | Log shows rebinds in a burst | Compare the resolved address before acting; act only on a change |
| R-2 | Events arrive before the engine finished `applyConfig` | Race in the startup test | The worker reads the applied configuration under the engine's lock; a missed event is covered by applying the current resolver answer after each apply |
| R-3 | DNS answers vary per cycle (round robin) and the responder match flaps | `respond` peer misses a packet | Match against every address of the last resolution; expiry, refresh, failure and removal follow the "DNS responder cache" table (AC-19, AC-20) |
| R-4 | Resolving a name on the unauthenticated packet path | CPU on a flood | Never resolve in `matchResponderPeer`; resolve on apply and on the refresh timer of "DNS responder cache", compare to the cached set |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | IKE tunnels restarted needlessly, or never started after a WAN change |
| How is it reverted? | Single commit per phase |
| Who else touches this path? | `spec-ipsec-11-mobike.md` (established-SA moves), `spec-rfcgate-1b-...` (reply source), child 1 (logical names), child 11 `access-reconnect` and `mgmt-tunnel` scenarios |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `interface/addr-added` on the bus for the configured interface | → | engine address worker → `rebindListeners` → peer start | `TestInterfaceAddressArrivalStartsPeers` |
| Peer with no `interface` and no `local-address` | → | per-cycle route-source resolution in `runOnce` | `TestRouteChosenLocalAddressBuildsChildSA` |
| `remote-address` is a DNS name | → | resolved pair on the SA → `encodeSAInitRequest` NAT_DETECTION | `TestDNSRemoteSendsNATDetection` |
| End to end on a late, changing, vanishing WAN address | → | all of the above | functional `ipsec-follows-wan-address`, interop `fqdn-remote-dynamic-local` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Startup, `interface wan0` with no IPv4 address | Warning as today; no IKE_SA_INIT sent; no Child SA attempt logged as failing every cycle |
| AC-2 | AC-1, then 203.0.113.1/32 appears on `wan0` | Listener on 203.0.113.1:500; peers without `local-address` start; IKE_SA_INIT leaves from 203.0.113.1 |
| AC-3 | Established, address changes to 203.0.113.7 | Listener rebinds; affected peers restart; new IKE SA from 203.0.113.7; old Child SAs removed from the kernel |
| AC-4 | Address removed | Affected peers stop and remove their Child SAs; nothing retries until an address returns |
| AC-5 | Peer with its own `local-address` literal | Never restarted by interface events |
| AC-6 | No `interface`, no `local-address`, route to the remote via 203.0.113.1 | IKE_SA_INIT from 203.0.113.1, NAT_DETECTION_SOURCE_IP hashed over it, Child SA endpoints 203.0.113.1 to the resolved remote |
| AC-7 | AC-6, address changes, MOBIKE negotiated | Not restarted by this spec; the MOBIKE path moves it (asserted by spec-ipsec-11's scenario, referenced) |
| AC-8 | AC-6, address changes, no MOBIKE | Peer whose SA's local address vanished restarts and comes up from the new address |
| AC-9 | `local-address any`, or any non-literal | `ze config validate` refuses: says the value is not an address, that omitting the leaf selects the route-chosen source, and that `vpn ipsec interface` binds an interface |
| AC-10 | `remote-address vpn.example.net` | IKE_SA_INIT carries both NAT_DETECTION payloads over the resolved addresses |
| AC-11 | AC-10, responder's NAT_DETECTION_SOURCE_IP does not match the resolved remote | NAT detected, SA floats to 4500, UDP encapsulation used |
| AC-12 | AC-10, no NAT | No float at IKE_SA_INIT; with MOBIKE, IKE_AUTH floats to 4500 as RFC 4555 requires (existing behaviour, now reached with a name) |
| AC-13 | Name does not resolve | Cycle fails with a log naming the peer, the name and the resolver error; retries at 1, 2, 4 ... 60 s and keeps retrying |
| AC-14 | AC-13, DNS starts answering | Next cycle establishes |
| AC-15 | Name resolves to a new address on a later cycle | That cycle dials the new address |
| AC-16 | `respond` peer with `remote-address` a name | Unsolicited IKE_SA_INIT from any address the name last resolved to is accepted; from another source refused; no DNS query is made on receipt |
| AC-17 | Transport-mode peer with a DNS remote | Proposes the resolved host pair (RFC 7296 Section 2.23.1) |
| AC-18 | Reload with an address-less interface | Refused as today |
| AC-19 | AC-16, a refresh fails | Previous set kept; failure logged with peer, name, resolver error and fix |
| AC-20 | AC-16, the name's answer changes, or the peer is removed | Next refresh replaces the set (old address now refused); a removed peer's entry is gone |
| AC-21 | Startup with the default `unmatched` DISCARD catch-all, route-chosen peer | The route-source lookup answers and IKE_SA_INIT leaves from the route's source (no EPERM) |
| AC-22 | A peer waits for its interface address | `ze doctor` check `ipsec-interface-address` warns, naming the peer, the interface and the fix (bring the interface up, or give the peer a `local-address`) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Boots the CPE before the PPPoE session is up | startup warn → addr-added → bring-up | `ipsec-follows-wan-address` |
| 2 | The LNS drops and re-establishes the session with a new address | addr-removed → stop → addr-added → restart | child 11 `access-reconnect` |
| 3 | DNS is down at boot, comes up later | per-cycle resolve, backoff, establish | `fqdn-remote-dynamic-local` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInterfaceAddressArrivalStartsPeers` | `internal/component/ike/engine/dynamic_wan_test.go` | AC-1, AC-2 with `resolveInterfaceAddrFn` swapped and a fake bus | |
| `TestInterfaceAddressChangeRehomesPeers` | same | AC-3, AC-5 | |
| `TestInterfaceAddressRemovalStopsPeers` | same | AC-4 | |
| `TestRouteChosenLocalAddressBuildsChildSA` | same | AC-6 | |
| `TestRouteChosenAddressChangeWithoutMobikeRestarts` | same | AC-8 | |
| `TestLocalAddressMustBeLiteral` | `internal/component/ike/ipsec/config_test.go` | AC-9 | |
| `TestDNSRemoteSendsNATDetection` | `internal/component/ike/engine/nat_dns_test.go` | AC-10 | |
| `TestDNSRemoteNATMismatchFloats` | same | AC-11 | |
| `TestUnresolvableRemoteRetriesWithBackoff` | `internal/component/ike/engine/fsm_test.go` | AC-13, AC-14, A-3 | |
| `TestRespondPeerMatchesResolvedName` | `internal/component/ike/engine/register_test.go` | AC-16 | |
| `TestTransportModeDNSRemoteProposesResolvedPair` | `internal/component/ike/engine/transport_mode_test.go` | AC-17 | |
| `TestLiteralPeerKeepsItsSocketAcrossInterfaceRebind` | `internal/component/ike/engine/dynamic_wan_test.go` | AC-5 with a literal peer established beside interface-bound peers: the interface pair moves, the literal peer's session and socket survive | |
| `TestRespondNameCacheLifecycle` | `internal/component/ike/engine/register_test.go` | AC-19, AC-20: refresh failure keeps the set, a changed answer replaces it, removal deletes it, nothing resolves on receipt | |
| `TestRouteChosenSourceUnderCatchAllDiscard` | `internal/component/ike/engine/dynamic_wan_integration_linux_test.go` (`integration` tag, its own user and network namespace through `userns.Enter`) | AC-21: engine applied with the default DISCARD catch-all, `routeSourceTo` answers and the first IKE_SA_INIT leaves from the route's source | |
| `TestDoctorIPsecInterfaceAddressWaiting` | `internal/component/ike/engine/doctor_interface_address_test.go` | AC-22 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| reconnect delay | 1 s to 60 s | 60 s | N/A | capped at 60 s |
| address event queue depth | fixed | full queue drops oldest, next resolver read corrects | N/A | N/A |
| `respond` name refresh interval | fixed 60 s | 60 s | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-follows-wan-address` | `test/ipsec/ipsec-follows-wan-address.ci`, run by `./le test ipsec --pattern ipsec-follows-wan-address` | address added late, changed, removed on a dummy interface under the default DISCARD catch-all; `show vpn ipsec sa` follows; a literal peer's SA keeps its SPIs throughout | |
| `ipsec-local-address-literal-only` | `test/parse/ipsec-local-address-literal-only.ci`, run by `./le test bgp parse --pattern ipsec-local-address-literal-only` | AC-9 message | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `fqdn-remote-dynamic-local` | `test/interop-ipsec/scenarios/` | strongSwan 6.1.0 behind a name served by a DNS container started late | AC-10, AC-12, AC-13, AC-14, AC-6 | |
| `fqdn-remote-real-nat` | `test/interop-ipsec/scenarios/` | strongSwan with the existing NAT box | AC-11 | |

## Files to Modify
- `internal/component/ike/engine/apply.go` - address worker entry; reuse `rebindListeners`; per-peer local origin (literal, interface, route)
- `internal/component/ike/engine/register.go` - subscribe at engine start, unsubscribe at stop; `matchResponderPeer` compares cached resolutions
- `internal/component/ike/engine/fsm.go` - store the resolved remote and local on the SA; NAT checks read them
- `internal/component/ike/engine/initiator.go`, `responder.go` - NAT_DETECTION from the resolved pair
- `internal/component/ike/engine/create_child.go`, `rekey.go`, `ts_nat_substitute.go`, `transport_mode.go`, `inner_selectors.go` - resolved pair instead of configured strings
- `internal/component/ike/engine/mobike_route_linux.go`, `mobike_route_other.go` - `routeSourceTo` called by the connect cycle too: error text and the "one caller" comments generalised
- `internal/component/ike/engine/doctor_interface_address.go` (create) and `register.go` - doctor check `ipsec-interface-address` registered beside `ipsec-virtual-ip-pool`; `internal/core/diagnostic/codes.go` gains `CodeDoctorIPsecInterfaceAddress`; `docs/architecture/ike/ipsec-10-cli-diag.md` documents it
- `internal/component/ike/engine/dynamic_wan_integration_linux_test.go`, `doctor_interface_address_test.go` (create) - AC-21, AC-22
- `test/ipsec/ipsec-follows-wan-address.ci` (create) - the functional test
- `internal/component/ike/ipsec/config.go` - `local-address` literal validation
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - `interface`, `local-address`, `remote-address` descriptions rewritten to the new behaviour
- `docs/guide/ipsec.md` - dynamic WAN section; multi-homed caveat naming rfcgate-1b
- `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - declared by `create_child.go` and `rekey.go`. "Transport-mode NAT substitution changes the selectors, not the outer addresses" says `createFirstChildSA` normally takes `LocalAddr` and `RemoteAddr` from the operator's `local-address` and `remote-address`; after this change it takes the resolved pair stored on the SA, so that sentence is rewritten
- `docs/architecture/ike/ipsec-3-data-model.md` - declared by `config.go`. Nothing in it becomes wrong: it describes no address leaf. The `local-address` literal check is a new parse refusal, which the page may list beside the parser it names (`parseSiteToSitePeer`) without correcting any statement

## Files to Create
- `internal/component/ike/engine/dynamic_wan.go` - the address worker
- `internal/component/ike/engine/dynamic_wan_test.go`, `nat_dns_test.go`
- `test/interop-ipsec/scenarios/fqdn-remote-dynamic-local/`, `fqdn-remote-real-nat/`
- the functional `.ci` files named above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new leaf |
| YANG validation constraints | Yes | `local-address` becomes an IP literal type |
| YANG custom validators | No | |
| CLI commands/flags | No | |
| CLI grammar | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | No | |
| Pipe completeness | No | |
| Env var registration | No | |
| Doctor check for runtime dependencies | Yes | check `ipsec-interface-address` (`internal/component/ike/engine/doctor_interface_address.go`, registered in `register.go`, code `CodeDoctorIPsecInterfaceAddress`): a peer waiting for its interface address is reported with the peer, the interface name and the fix; proven by `TestDoctorIPsecInterfaceAddressWaiting` (AC-22) |
| Prometheus counters/metrics | No | |
| BGP family surface | N-A | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | Feature scope changed? | Yes | the IPsec feature file under `features/` |
| 2 | Config syntax changed? | Yes | `local-address` literal only; `docs/guide/configuration.md` if it shows the leaf |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/ipsec.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior changed or proven? | Yes | `rfc/short/rfc7296.md` Section 2.23 rows gain the DNS-remote test |
| 10 | Test infrastructure changed? | Yes | a late-starting DNS container in the IPsec suite: `docs/architecture/testing/interop.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ike/ipsec-7-ikev2-engine.md` |
| 13 | Route metadata keys | No | |
| 14 | Prometheus counters | No | |
| 15 | Registered plugin/event changed? | Yes | IKE becomes a consumer of `interface/addr-*`: `docs/plugin-overview.md` event consumers if listed |
| 16 | Changed source referenced by doc anchors? | Yes | derived by `./le spec citation anchors spec plan/release/fixes/spec-home-cpe-4-ike-dynamic-wan.md` |
| 17 | Existing examples for this area? | Yes | `interface eth0` examples in `docs/guide/ipsec.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)**
   - Tests: `TestInterfaceAddressArrivalStartsPeers`, `TestRouteChosenLocalAddressBuildsChildSA`, `TestDNSRemoteSendsNATDetection`
   - Files: `dynamic_wan.go` skeleton with the subscription; resolved-pair fields on the SA
   - Verify: subscription reached; tests red on the missing behaviour
2. **Phase: Interface-bound local address** - AC-1 to AC-5, AC-18
3. **Phase: Route-chosen local and literal validation** - AC-6 to AC-9
4. **Phase: Resolved remote for every reader, retry proof, respond match** - AC-10 to AC-17; interop scenarios, each reverted once and recorded red

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every `ParseIP(...RemoteAddress)` and `ParseIP(...LocalAddress)` reader listed in Current Behavior reads the resolved pair |
| Correctness | No restart when the resolved address did not change |
| Data flow | Bus subscription, no iface import |
| Rule: goroutines | One long-lived worker on a channel, stopped with the engine |
| Rule: user-facing errors | AC-9 and AC-13 messages say what, why, and the fix |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No configured-string reader left | grep `ParseIP(sa.PeerCfg.RemoteAddress)` returns only the resolver |
| Two scenarios | `ls test/interop-ipsec/scenarios/fqdn-remote-*` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Unauthenticated path | No DNS lookup triggered by an inbound packet |
| Spoofed events | Bus is in-process; the worker trusts the resolver's answer, not the event payload |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A-2 broken | Coordinate with child 1; the CPE uses route-chosen mode meanwhile |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

Two different questions shared one leaf: "where do I send from" and "what did I
configure". Configured strings answered both, and a name or an absent value made
every reader silently skip its work. The SA now carries the answer the cycle
measured, and the configuration is only where it came from.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `unwired-feature.md` | 2026-09-04 | IPsec `remote-address` accepts a hostname its consumers cannot use (FIRST finding: NAT detection, `detectResponderNAT`, transport mode, `respond` match) |
| `unwired-feature.md` | 2026-11-07 | IKE over IPv6, every socket udp4 (owner of record) |

The 2026-09-04 row once carried two findings. It is split: the 2026-09-04 row now
holds the hostname half alone, which this spec fixes, and the IPv6 half is the
2026-11-07 row. This spec is that row's owner of record only. Making IKE run over
IPv6 is not this spec's work (Known Limitations), so the row stays open when this
spec closes and moves to the spec that carries a second socket family through the
transport.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Absent `local-address` with no `interface` means route-chosen; `any` is refused with a message | Accept a literal `any` keyword | One spelling per fact; the mapping guide (child 10) writes the omission, as the umbrella already records |
| The address event triggers a resolver read | Act on the event payload | The payload names the kernel device; the resolver already maps logical names (and will follow child 1) |
| Restart peers on an interface-bound address change | MOBIKE update | The bound socket must move, which ends every session holding it (`rebindListeners`); MOBIKE helps only with a wildcard listener, and that case is left to spec-ipsec-11 |
| One socket pair per local origin (interface, each distinct literal, wildcard), "Listener set" | Keep one shared pair and restart every peer on a rebind, as `rebindListeners` does today | `PeerSession.ike` and `natt` are fixed for a session's life, so a shared pair makes a literal-`local-address` peer lose its socket on every interface event; separate pairs keep AC-5 true and move only the sessions whose address moved (`TestLiteralPeerKeepsItsSocketAcrossInterfaceRebind`) |
| rfcgate-1b stays separate | Absorb it | It is blocked on an owner question about listener design; this spec changes nothing it decides |
| Cache `respond`-peer name resolutions | Resolve per packet | No DNS on the unauthenticated path |

## Known Limitations
- IKE over IPv6 remains unsupported.
- On a multi-homed host the route-chosen mode replies from the route's source (rfcgate-1b).

## RFC Documentation (Scope: protocol)

The NAT_DETECTION build and check carry the RFC 7296 Section 2.23 quote above the
statement; the address worker's call into the MOBIKE-eligible path carries
`// RFC 4555 Section 3.5`.

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
- [ ] Integration Checklist marks "Doctor check" for the waiting-for-address state

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] The targeted tests for what changed pass: the scoped package tests for `internal/component/ike/engine` and `internal/component/ike/ipsec` through `./le job run label unit-pkg command go test -tags ...`, `./le test bgp parse --pattern ipsec-local-address-literal-only`, `./le test ipsec --pattern ipsec-follows-wan-address` for `test/ipsec/ipsec-follows-wan-address.ci`, `TestRouteChosenSourceUnderCatchAllDiscard` through the same `./le job run` form with the `integration` tag added, `IPSEC_INTEROP_SCENARIO=fqdn-remote-dynamic-local ./le test integration interop-ipsec` and `IPSEC_INTEROP_SCENARIO=fqdn-remote-real-nat ./le test integration interop-ipsec`, and `./le verify changed` over the closing commits
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A** and **Commit B** per `ai/rules/planning.md`

# Spec: home-cpe-4-ike-dynamic-wan

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Shipped IKE initiator does the wrong thing on a dynamic WAN: a peer whose local address comes from `interface` never comes up when the address appears after boot and never follows a new address; a peer with neither `interface` nor `local-address` cannot build a Child SA; a DNS-named remote sends no NAT detection. Each configuration is accepted and silently fails. |
| RFC gaps | - |
| Depends | spec-home-cpe-1-logical-interface-name.md (only for `interface` naming a PPPoE logical unit) |
| Phase | 0/4 |
| Handoff | - |
| Updated | 2026-10-31 |

Parent: plan/home-cpe/spec-home-cpe-0-umbrella.md

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
- [ ] `docs/guide/ipsec.md` around line 183 - a reload whose interface has no IPv4 address is refused, startup warns
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
- [ ] `internal/component/ike/engine/apply.go` `applyConfig` (line 217) - resolves `cfg.Interface` once through `resolveInterfaceAddrFn`; on a reload with no address returns the `unbindablePeers` refusal; at startup warns and continues; fills `LocalAddress` of peers that name none only when an address was found; then `rebindListeners(ikeListenHost(ifIP, peerLocal), ...)`.
- [ ] `internal/component/ike/engine/apply.go` `rebindListeners` (about line 391) - when the host moves, stops every peer, closes the sockets, opens new ones; reached only from `applyConfig`.
- [ ] `internal/component/ike/engine/testport.go` `ikeListenHost` - interface address when known, otherwise `0.0.0.0`. A startup with an address-less interface therefore listens on the wildcard while its peers keep an empty `LocalAddress`.
- [ ] `internal/component/ike/engine/register.go` `resolveInterfaceAddr` (about line 894) - first IPv4 address of the logical interface through the iface resolver; distinguishes lookup failure from "no IPv4 address".
- [ ] `internal/component/ike/engine/fsm.go` `runOnce` (about line 182) - `net.ResolveUDPAddr("udp4", ikeAddr(peer.RemoteAddress))` per cycle; the resolved address is used for the send only.
- [ ] `internal/component/ike/engine/fsm.go` (about line 615) - NAT_DETECTION_SOURCE_IP check runs only when `net.ParseIP(sa.PeerCfg.RemoteAddress)` is non-nil; DESTINATION_IP check (about line 637) only when `LocalAddress` parses.
- [ ] `internal/component/ike/engine/initiator.go` (about line 93) - NAT_DETECTION payloads added only when both configured strings parse as IPs: a DNS name or an empty local sends none.
- [ ] `internal/component/ike/engine/create_child.go` `newChildSA` (line 51) - endpoints from the configured strings, overridden by the MOBIKE path or `peerTunnelAddress(sa)` (the authenticated peer endpoint) for tunnel mode; refuses a nil local.
- [ ] `internal/component/ike/engine/rekey.go` `anyChildTSPayloads` (line 237) - wildcard family from `ParseIP(RemoteAddress)`.
- [ ] `internal/component/ike/engine/ts_nat_substitute.go`, `transport_mode.go`, `inner_selectors.go` `configuredHostPair` - read the configured strings; a DNS name yields nil.
- [ ] `internal/component/ike/engine/register.go` `matchResponderPeer` (line 722) - compares the configured remote string to the datagram source, character for character.
- [ ] `internal/component/ike/engine/fsm.go` `reconnectDelay` (line 94) - 1 s doubling to 60 s on `connectFailures`; `PeerSession.run` never gives up.
- [ ] `internal/component/ike/engine/mobike.go` `mobikeRouteSource` (line 601), `serviceMobike` - a route-source lookup by connecting a UDP socket toward the peer; runs only for an established MOBIKE SA.
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - `interface` (line 72), `local-address` (line 862, a string read with `net.ParseIP`, no validation), `remote-address` (line 871, IPv4 or DNS name, `udp4`).
- [ ] `internal/component/iface/iface.go` `AddrPayload` (line 50) and `internal/core/iface/events/events.go` - `interface/addr-added`, `interface/addr-removed` with name, unit, index, address, prefix length, family.
- [ ] `internal/plugins/ospf/transport/transport.go` (about line 220) - subscribes to both events and feeds a bounded queue drained by one worker goroutine.

**Behavior to preserve:**
- A reload whose interface has no IPv4 address is refused (`unbindablePeers`); startup warns and continues.
- A peer with its own `local-address` literal is untouched by interface events.
- Port 500 binds the interface address when `interface` is set; 4500 binds the wildcard when live XFRM migration is available (`apply.go` about line 75).
- The backoff schedule.
- Remote IPv6 IKE stays unsupported (`udp4`).

**Behavior to change:**
- Interface address events drive bring-up, re-home and stop.
- A peer with no local address and no `interface` resolves its source per cycle.
- The resolved remote replaces the configured string for every reader listed above.
- `local-address` accepts only an IP literal; anything else is refused with the fix in the message.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `vpn ipsec interface <name>`, or no `interface` and no `local-address`; `remote-address vpn.example.net`.
- Bus: `interface/addr-added`, `interface/addr-removed` from the iface monitor (PPPoE IPCP completing on `ppp0`, or the address leaving).

### Transformation Path
1. The IKE engine subscribes to both events at start; a bounded queue feeds one long-lived worker (OSPF's pattern).
2. On any IPv4 event the worker re-runs `resolveInterfaceAddrFn(cfg.Interface)` (the event is a trigger, the resolver is the truth, so logical names resolve as they do at apply).
3. Address appeared or changed: the engine rebinds listeners to it, sets it as the local address of the peers that name none, and starts or restarts those peers. Address gone: those peers stop and remove their Child SAs, the listener closes.
4. Each connect cycle resolves the remote name, then (route-chosen mode) the local source toward it, and stores both on the SA.
5. Every reader takes the SA's resolved pair: NAT_DETECTION build and checks, `newChildSA`, `anyChildTSPayloads`, transport host pairs, NAT substitution.
6. `respond` peers configured by name: the engine keeps the name's last resolved addresses and `matchResponderPeer` compares against them.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface ↔ IKE engine | event bus, JSON `AddrPayload` | No |
| IKE engine ↔ resolver | `net.Resolver` per cycle | No |
| Ze ↔ strongSwan | NAT_DETECTION payloads, port float | No |
| IKE engine ↔ kernel | route-source lookup (connected UDP socket, no packet sent) | No |

### Integration Points
- `applyConfig` and `rebindListeners` (reused, not duplicated); `runOnce`; `matchResponderPeer`; `mobikeRouteSource` (its route-source lookup is extracted and shared).

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
| A-1 | The iface monitor emits `addr-added` for an address the PPPoE client installs on `ppp0` | Netlink monitor maps RTM_NEWADDR (`monitor_linux.go` about line 421) | Subscribe to the PPPoE client's own up event as well | Functional test with a PPPoE session | unvalidated |
| A-2 | `vpn ipsec interface` naming a PPPoE unit that has no kernel device yet is accepted at commit | YANG: "Commit refuses a name no interface on the host carries" | The boot case needs child 1's logical-name resolution, or the CPE uses route-chosen mode | Parse test with a pppoe-client unit | unvalidated |
| A-3 | A resolve failure increments `connectFailures`, so the backoff grows | `reconnectDelay` reads `connectFailures` | Retry at 1 s forever; fix in this spec | `TestUnresolvableRemoteRetriesWithBackoff` | unvalidated |
| A-4 | strongSwan, with no NAT on the path and Ze sending correct NAT_DETECTION, keeps port 500 until MOBIKE floats it | RFC 7296 Section 2.23 and RFC 4555 | Scenario expectation changes | mgmt-tunnel scenario | unvalidated |
| A-5 | The route-source lookup works with no default route through the WAN (the CPE's PPPoE adds none) | Umbrella: "no default route from PPP"; the static default route via the PPPoE interface is child 1 | Route-chosen mode needs that route; document it | Functional test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Address flap restarts every peer repeatedly | Log shows rebinds in a burst | Compare the resolved address before acting; act only on a change |
| R-2 | Events arrive before the engine finished `applyConfig` | Race in the startup test | The worker reads the applied configuration under the engine's lock; a missed event is covered by applying the current resolver answer after each apply |
| R-3 | DNS answers vary per cycle (round robin) and the responder match flaps | `respond` peer misses a packet | Match against every address of the last resolution |
| R-4 | Resolving a name on the unauthenticated packet path | CPU on a flood | Never resolve in `matchResponderPeer`; resolve on apply and on a timer, compare to the cached set |

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
| End to end on PPPoE | → | all of the above | functional `ike-follows-wan-address`, interop `fqdn-remote-dynamic-local` |

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
| AC-16 | `respond` peer with `remote-address` a name | Unsolicited IKE_SA_INIT from any address the name last resolved to is accepted; from another source refused |
| AC-17 | Transport-mode peer with a DNS remote | Proposes the resolved host pair (RFC 7296 Section 2.23.1) |
| AC-18 | Reload with an address-less interface | Refused as today |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Boots the CPE before the PPPoE session is up | startup warn → addr-added → bring-up | `ike-follows-wan-address` |
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

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| reconnect delay | 1 s to 60 s | 60 s | N/A | capped at 60 s |
| address event queue depth | fixed | full queue drops oldest, next resolver read corrects | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ike-follows-wan-address` | `test/plugin/` or the IKE functional suite | address added late, changed, removed on a dummy interface; `show vpn ipsec sa` follows | |
| `ipsec-local-address-literal-only` | `test/parse/` | AC-9 message | |

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
- `internal/component/ike/engine/mobike.go` - route-source lookup extracted for reuse
- `internal/component/ike/ipsec/config.go` - `local-address` literal validation
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - `interface`, `local-address`, `remote-address` descriptions rewritten to the new behaviour
- `docs/guide/ipsec.md` - dynamic WAN section; multi-homed caveat naming rfcgate-1b

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
| Doctor check for runtime dependencies | Yes | a peer waiting for its interface address is reported by `ze doctor` with the interface name and the fix |
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
| 16 | Changed source referenced by doc anchors? | Yes | derived by `./le spec citation anchors spec plan/home-cpe/spec-home-cpe-4-ike-dynamic-wan.md` |
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

The row is listed today in `plan/release/fixes/spec-yang-single-source-of-truth.md`'s
owned rows. Its SECOND finding (IKE over IPv6, all sockets `udp4`) is not this
spec's work. When this spec moves to `ready`, the main thread splits the row (the
IPv6 half gets its own row and owner) and moves the first half's ownership here.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Absent `local-address` with no `interface` means route-chosen; `any` is refused with a message | Accept a literal `any` keyword | One spelling per fact; the mapping guide (child 10) writes the omission, as the umbrella already records |
| The address event triggers a resolver read | Act on the event payload | The payload names the kernel device; the resolver already maps logical names (and will follow child 1) |
| Restart peers on an interface-bound address change | MOBIKE update | The bound socket must move, which ends every session holding it (`rebindListeners`); MOBIKE helps only with a wildcard listener, and that case is left to spec-ipsec-11 |
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
- [ ] The targeted tests for what changed pass
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

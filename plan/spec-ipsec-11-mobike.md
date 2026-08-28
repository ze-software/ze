# Spec: ipsec-11 -- MOBIKE (RFC 4555)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | ipsec-8 (Child SA/XFRM), ipsec-9 (NAT-T) |
| Phase | - |
| Updated | 2026-05-21 |

Anchor refresh (2026-07-22 plan review, design unchanged, feature not
landed; Depends ipsec-8/ipsec-9 both SATISFIED, learned 742/744): the
planned interop scenario numbers 03/04 collided with existing
`03-eap-mschapv2`/`04-eap-tls`; the tree runs through
`11-responder-accepts-reinit`. RESOLVED in-body 2026-07-22: every
occurrence is renumbered to `12-mobike-responder`/`13-mobike-initiator`
(verified free -- scenario directories present are 01-05 and 07-11).
The `monitor_linux.go` refresh note still holds.

Superseded 2026-08-24: the numbering scheme is gone. Thomas ruled that
interop scenarios are named, never numbered, so the paragraph above
describes a collision that can no longer happen and a resolution that no
longer applies. The two planned directories are now `mobike-responder`
and `mobike-initiator`, and no number has to be reserved for them. The
2026-07-22 observation is left as it was written, because it records what
that reviewer saw on that date.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `rfc/short/rfc4555.md` -- MOBIKE protocol reference
4. `internal/component/ike/engine/sa.go` -- SA struct (mutable address fields)
5. `internal/component/ike/engine/inbound.go` -- INFORMATIONAL handler
6. `internal/component/ike/engine/established.go` -- maintainSA loop
7. `internal/component/ike/wire/payload_notify.go` -- notify type constants
8. `internal/component/ike/dataplane/dataplane.go` -- Dataplane interface

## Task

Implement MOBIKE (RFC 4555) in Ze's native IKE engine. MOBIKE allows IP addresses
associated with IKE SAs and tunnel-mode IPsec SAs to change after establishment.
Both initiator and responder roles are implemented. The initiator detects local
address changes via netlink address monitoring (`AddrSubscribe`) and sends
UPDATE_SA_ADDRESSES. The responder accepts UPDATE_SA_ADDRESSES from the peer,
updates SA addresses, and reinstalls Child SAs with new addresses.

Interop testing against strongSwan: two Docker scenarios exercising both roles
with real address changes and ESP traffic verification.

### Scope

**In scope:**
- MOBIKE capability negotiation (MOBIKE_SUPPORTED in IKE_AUTH)
- UPDATE_SA_ADDRESSES exchange (both initiator and responder)
- Return routability check (COOKIE2)
- NAT detection in MOBIKE INFORMATIONAL exchanges
- Address monitoring via netlink AddrSubscribe (Linux)
- Child SA reinstall on address change (remove + install, no new Dataplane methods)
- ADDITIONAL_IP4_ADDRESS / ADDITIONAL_IP6_ADDRESS (announce additional addresses)
- NO_ADDITIONAL_ADDRESSES notification
- UNACCEPTABLE_ADDRESSES error handling
- Config option: `mobike enable/disable` in ike-group (default: enable)
- Two interop test scenarios against strongSwan

**Out of scope:**
- NO_NATS_ALLOWED / UNEXPECTED_NAT_DETECTED (NAT prohibition for non-NAT-T paths)
  -- Ze always uses NAT-T when MOBIKE is active (port 4500)
- VPP address monitoring (future: consume existing iface/vpp/monitor events)
- Path testing to probe alternative paths when current path still works
- Load balancing across multiple address pairs
- Transport mode IPsec (MOBIKE is tunnel-mode only per RFC)

### Linux vs VPP Differences

| Aspect | Linux (XFRM) | VPP |
|--------|-------------|-----|
| Address monitoring | `netlink.AddrSubscribe` (RTM_NEWADDR/RTM_DELADDR) | `sw_interface_event` via GoVPP WantInterfaceEvents |
| SA update on address change | `RemoveSA` + `InstallSA` via netlink | `RemoveSA` + `InstallSA` via VPP binary API |
| UDP encapsulation update | Reinstall SA with new UDPEncapSPort/DPort | Reinstall SA with new encap params |
| Implementation in this spec | Full | SA reinstall works (same Dataplane interface); address monitoring deferred |

The Dataplane interface (`InstallSA`, `RemoveSA`) is backend-agnostic. MOBIKE address
changes use the existing remove+install sequence. No new Dataplane methods are needed.
The VPP backend already implements `InstallSA`/`RemoveSA` with the correct API calls.
The only VPP-specific gap is address monitoring: the iface/vpp/monitor already receives
address events, but wiring them to the IKE engine is deferred to a future spec.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- component isolation, registration pattern
  -> Constraint: IKE engine is a registered plugin. Address monitoring must integrate via the same event/bus pattern, not direct coupling.
- [ ] `rfc/short/rfc4555.md` -- MOBIKE protocol
  -> Constraint: Initiator decides address pair. Responder MUST NOT update IPsec SAs without UPDATE_SA_ADDRESSES from initiator. Port 4500 mandatory when both support MOBIKE + NAT-T. COOKIE2 is 8-64 bytes, MUST be echoed verbatim.
  -> Decision: Return routability check (COOKIE2) SHOULD be performed before updating IPsec SAs by default.
- [ ] `rfc/short/rfc7296.md` -- IKEv2 base: INFORMATIONAL exchange, NAT detection
  -> Constraint: INFORMATIONAL exchanges use ExchangeType 37, encrypted under SK. NAT detection uses SHA-1 hash of SPIs + IP + port.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4555.md` -- MOBIKE (primary reference)
  -> Constraint: 9 new notify types (40, 41, 16396-16402). UPDATE_SA_ADDRESSES has no data. COOKIE2 data is 8-64 bytes. MOBIKE_SUPPORTED data is empty (zero-length).
- [ ] `rfc/short/rfc7296.md` -- IKEv2 base
  -> Constraint: Section 2.23 NAT detection. INFORMATIONAL must be encrypted. Message ID ordering.

**Key insights:**
- SA addresses are currently immutable (from config). MOBIKE needs mutable `CurrentLocalAddr`/`CurrentRemoteAddr`/`CurrentRemotePort` fields.
- `remoteUDPAddr()` resolves from config; must use current mutable fields instead.
- INFORMATIONAL handler (`inbound.go`) only processes DELETE and DPD. Must add UPDATE_SA_ADDRESSES case.
- `routewatch` exists for route events; address events need a similar `addrwatch` or direct subscription in the engine.
- `netlink.AddrSubscribe` is already vendored and used in `plugins/iface/netlink/monitor_linux.go`.
- Child SA reinstall (remove+install) uses existing Dataplane methods. No interface changes.
- strongSwan supports MOBIKE by default (mobike=yes in swanctl.conf).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/sa.go` -- SA struct with immutable PeerCfg addresses. Fields: PeerName, PeerCfg (SiteToSitePeer with LocalAddress/RemoteAddress strings), InitiatorSPI/ResponderSPI, State, key material, NATDetected/BehindNAT, message IDs. `remoteUDPAddr()` at line 147 resolves from `sa.PeerCfg.RemoteAddress`.
  -> Constraint: `remoteUDPAddr()` resolves from `sa.PeerCfg.RemoteAddress`, not a mutable field. All senders use this.
- [ ] `internal/component/ike/engine/inbound.go` -- INFORMATIONAL handler at line 27: processes DELETE (PayloadDelete) and DPD (empty INFORMATIONAL). Unknown PayloadNotify types are logged via `log.Debug("ike: informational notify")` but ignored.
  -> Constraint: Unknown notify types are logged but ignored. Must add explicit UPDATE_SA_ADDRESSES handling.
- [ ] `internal/component/ike/engine/established.go` -- `runEstablished` at line 17 creates Child SA then calls `maintainSA`. `maintainSA` at line 65 runs 1-second ticker loop for DPD/rekey. Has `routeReannounce` (30s) for bus events.
  -> Constraint: 1-second ticker drives DPD/rekey. MOBIKE address change can be checked in same loop via a channel.
- [ ] `internal/component/ike/engine/dpd.go` -- `sendDPD` at line 70 builds empty INFORMATIONAL without NAT detection payloads. Uses `sa.remoteUDPAddr()` for destination.
  -> Constraint: MOBIKE extends DPD to include NAT detection payloads when behind NAT.
- [ ] `internal/component/ike/engine/auth.go` -- `buildAuthRequest` at line 77 assembles IDi, AUTH, CERT, SAi2, TSi, TSr. No MOBIKE_SUPPORTED notify.
  -> Constraint: Must add N(MOBIKE_SUPPORTED) after SAi2/TSi/TSr payloads.
- [ ] `internal/component/ike/wire/payload_notify.go` -- notify constants lines 8-39: error types 1-44, status types 16384-16395 then 16430-16431.
  -> Constraint: Gap at 16396-16402 is exactly where MOBIKE constants go. Error types 40-41 also need adding (UNACCEPTABLE_ADDRESSES, UNEXPECTED_NAT_DETECTED).
- [ ] `internal/component/ike/dataplane/dataplane.go` -- Dataplane interface at line 73: InstallSA(SAParams), RemoveSA(spi, dst, proto). No UpdateSA method.
  -> Decision: Use remove+install for address change. No new Dataplane interface methods.
- [ ] `internal/component/ike/engine/child.go` -- ChildSA struct at line 46 with LocalAddr/RemoteAddr (net.IP), InboundSPI/OutboundSPI, IfID, Keys.
  -> Constraint: ChildSA addresses must be updated when MOBIKE changes addresses.
- [ ] `internal/component/ike/engine/reconcile.go` -- PeerSession struct at line 16 with peerName, peerCfg, espGroup, sa, childSA, stopCh/done.
  -> Constraint: PeerSession holds SA reference. MOBIKE state can live on SA struct.
- [ ] `internal/core/routewatch/routewatch.go` -- Watcher pattern: Register(Handler) returns unsubscribe func, Start(errCb), Stop().
  -> Decision: Address watcher follows same pattern.
- [ ] `internal/plugins/iface/netlink/monitor_linux.go` -- Uses `netlink.AddrSubscribe(addrCh, m.stopCh)` at ~~line 75~~ line 76 (moved by the 2026-07 wave, see Post-wave corrections), receives `netlink.AddrUpdate` with `NewAddr bool`, `LinkAddress *net.IPNet`, `LinkIndex int`.

**Behavior to preserve:**
- DPD continues to work without MOBIKE (non-MOBIKE peers get no NAT detection in DPD)
- Non-MOBIKE peers: SA addresses remain immutable from config
- Existing interop scenarios (psk-site-to-site, ipsec-bgp-redistribute-frr) unaffected
- `remoteUDPAddr()` still works for non-MOBIKE SAs

**Behavior to change:**
- IKE_AUTH includes N(MOBIKE_SUPPORTED) when MOBIKE is enabled in config
- SA gets mutable current address fields (initialized from config, updated by MOBIKE)
- INFORMATIONAL handler processes UPDATE_SA_ADDRESSES
- DPD includes NAT detection payloads when MOBIKE is active and behind NAT
- Child SA addresses are updated on MOBIKE address change

## Data Flow (MANDATORY)

### Entry Point: Responder (receiving UPDATE_SA_ADDRESSES)
- Inbound IKE packet on port 4500 (NAT-T) from peer's new IP address
- Packet source IP is the new address; IKE payload contains N(UPDATE_SA_ADDRESSES)

### Transformation Path (Responder)
1. `dispatchNATTInbound` -> strip non-ESP marker -> `handleInbound` -> `handleEstablishedInbound`
2. `handleInformational` detects `UPDATE_SA_ADDRESSES` notify -> `handleMobikeUpdate`
3. Validate source address against policy
4. Update `SA.CurrentRemoteAddr` / `CurrentRemotePort` from packet source
5. Send INFORMATIONAL response with NAT detection + COOKIE2 echo
6. Perform return routability check if configured
7. `reinstallChildSA`: remove old Child SA from dataplane, install with new addresses

### Entry Point: Initiator (detecting address change)
- Netlink `AddrUpdate` event (RTM_NEWADDR on configured interface, or RTM_DELADDR on current interface)
- Or: config reconciliation changes peer's local-address

### Transformation Path (Initiator)
1. Address monitor detects change on the IKE interface -> notify PeerSession via channel
2. PeerSession updates `SA.CurrentLocalAddr` from new address
3. Build INFORMATIONAL request with N(UPDATE_SA_ADDRESSES) + NAT detection + COOKIE2
4. Send from new address to current remote address
5. On response: verify COOKIE2, process NAT detection
6. `reinstallChildSA`: remove old Child SA, install with new local address

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire -> Engine | Notify type dispatch in `handleInformational` | [ ] |
| Engine -> Dataplane | `RemoveSA` + `InstallSA` calls | [ ] |
| Netlink -> Engine | AddrSubscribe channel in engine goroutine | [ ] |
| Engine -> Transport | Send INFORMATIONAL via `tr.Send` from new address | [ ] |

### Integration Points
- `handleInformational` in `inbound.go` -- add UPDATE_SA_ADDRESSES case
- `maintainSA` in `established.go` -- check for address change events
- `buildAuthRequest` in `auth.go` -- add MOBIKE_SUPPORTED notify
- `sendDPD` in `dpd.go` -- add NAT detection payloads for MOBIKE SAs
- `createFirstChildSA` in `child.go` -- use current addresses, not config addresses

### Architectural Verification
- [ ] No bypassed layers (MOBIKE flows through INFORMATIONAL handler, same as DPD)
- [ ] No unintended coupling (address monitoring is a goroutine in PeerSession, not a global)
- [ ] No duplicated functionality (reuses NAT detection hash, SK encryption, Child SA install)
- [ ] Zero-copy preserved (MOBIKE payloads use same wire encoding as existing notifies)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| IKE_AUTH exchange | -> | MOBIKE_SUPPORTED included | `TestMobikeSupported` |
| Inbound INFORMATIONAL with UPDATE_SA_ADDRESSES | -> | `handleMobikeUpdate` | `TestHandleMobikeUpdate` |
| Address change on interface | -> | Initiator sends UPDATE_SA_ADDRESSES | `TestInitiatorAddressChange` |
| COOKIE2 in request | -> | Echoed in response | `TestCookie2Echo` |
| Interop: strongSwan initiator moves | -> | Ze responder updates SA | `test/interop-ipsec/scenarios/mobike-responder/check.py` |  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `design` and the work is not implemented) -->
| Interop: Ze initiator moves | -> | strongSwan responder updates SA | `test/interop-ipsec/scenarios/mobike-initiator/check.py` |  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `design` and the work is not implemented) -->

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | MOBIKE enabled in ike-group config, IKE_AUTH exchange | Both peers include N(MOBIKE_SUPPORTED) in IKE_AUTH. SA.MobikeSupported is true. |
| AC-2 | strongSwan initiator changes IP, sends UPDATE_SA_ADDRESSES to Ze responder | Ze updates SA.CurrentRemoteAddr from packet source, sends INFORMATIONAL response with NAT detection, reinstalls Child SA with new remote address. |
| AC-3 | Ze initiator detects local address change, sends UPDATE_SA_ADDRESSES to strongSwan responder | strongSwan accepts, Ze reinstalls Child SA with new local address. SA.CurrentLocalAddr updated. |
| AC-4 | After address change (responder role), ESP traffic sent to new peer address | XFRM SA byte counters increase. Traffic flows through tunnel using new addresses. |
| AC-5 | After address change (initiator role), ESP traffic sent from new local address | XFRM SA byte counters increase. Traffic flows through tunnel using new source address. |
| AC-6 | COOKIE2 included in UPDATE_SA_ADDRESSES request | Responder echoes COOKIE2 verbatim. Initiator verifies match. |
| AC-7 | COOKIE2 mismatch in response | SA is closed (StateDead). Logged as security event. |
| AC-8 | Responder rejects address (policy violation) | UNACCEPTABLE_ADDRESSES sent. Initiator logs rejection. |
| AC-9 | MOBIKE disabled in config | No MOBIKE_SUPPORTED in IKE_AUTH. UPDATE_SA_ADDRESSES from peer is logged and ignored. |
| AC-10 | DPD with MOBIKE active and behind NAT | DPD INFORMATIONAL includes NAT_DETECTION_SOURCE_IP and NAT_DETECTION_DESTINATION_IP. NAT mapping change detected triggers UPDATE_SA_ADDRESSES. |
| AC-11 | Interop: Ze responder, strongSwan initiator changes IP mid-session, traffic verified | End-to-end Docker interop test passes. Scenario mobike-responder. |
| AC-12 | Interop: Ze initiator, address change mid-session, traffic verified | End-to-end Docker interop test passes. Scenario mobike-initiator. |
| AC-13 | ADDITIONAL_IP4_ADDRESS in IKE_AUTH | Peers can announce additional addresses. Stored on SA for future use. |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMobikeNotifyConstants` | `wire/payload_notify_test.go` | All 9 MOBIKE notify type constants have correct values | |
| `TestMobikeSupported` | `engine/mobike_test.go` | MOBIKE_SUPPORTED included in IKE_AUTH payloads | |
| `TestHandleMobikeUpdate` | `engine/mobike_test.go` | UPDATE_SA_ADDRESSES updates SA addresses and triggers reinstall | |
| `TestCookie2Echo` | `engine/mobike_test.go` | COOKIE2 in request is copied to response | |
| `TestCookie2Mismatch` | `engine/mobike_test.go` | Mismatched COOKIE2 closes SA | |
| `TestCookie2Length` | `engine/mobike_test.go` | COOKIE2 between 8 and 64 bytes accepted; outside range rejected | |
| `TestUnacceptableAddresses` | `engine/mobike_test.go` | UNACCEPTABLE_ADDRESSES response handled correctly | |
| `TestMobikeDisabled` | `engine/mobike_test.go` | UPDATE_SA_ADDRESSES ignored when MOBIKE not negotiated | |
| `TestDPDWithMobike` | `engine/dpd_test.go` | DPD includes NAT detection when MOBIKE + behind NAT | |
| `TestAdditionalAddresses` | `engine/mobike_test.go` | ADDITIONAL_IP4/IP6_ADDRESS parsed and stored | |
| `TestNoAdditionalAddresses` | `engine/mobike_test.go` | NO_ADDITIONAL_ADDRESSES clears stored addresses | |
| `TestInitiatorAddressChange` | `engine/mobike_test.go` | Local address change triggers UPDATE_SA_ADDRESSES send | |
| `TestReinstallChildSA` | `engine/mobike_test.go` | Remove old + install new ChildSA with updated addresses | |
| `TestAddrWatch` | `engine/addrwatch_test.go` | Address watcher detects add/remove on interface | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| COOKIE2 length | 8-64 bytes | 64 | 7 | 65 |
| Notify type MOBIKE_SUPPORTED | 16396 | N/A | N/A | N/A |
| UPDATE_SA_ADDRESSES data | 0 bytes (empty) | 0 | N/A | non-zero (ignored per RFC) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mobike-responder` | `test/interop-ipsec/scenarios/mobike-responder/check.py` | strongSwan initiator moves IP, Ze responder updates tunnel, traffic flows | |  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `design` and the work is not implemented) -->
| `mobike-initiator` | `test/interop-ipsec/scenarios/mobike-initiator/check.py` | Ze initiator moves IP, strongSwan responder updates tunnel, traffic flows | |  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `design` and the work is not implemented) -->
| `ipsec-mobike-config` (added 2026-07-10) | `test/parse/ipsec-mobike-config.ci` | `mobike enable/disable` ike-group leaf parsed and accepted | |

### Interop Test Network Design

The existing lab uses a single Docker bridge network (`172.28.0.0/24`). MOBIKE tests
need a second network so a container can acquire a new IP and send UPDATE_SA_ADDRESSES.

#### Network Layout

```
Network 1 (primary):  ze-ipsec-<pid>     172.28.0.0/24
  Ze:          172.28.0.2
  strongSwan:  172.28.0.3

Network 2 (mobility): ze-ipsec-mob-<pid>  172.29.0.0/24
  (attached mid-test to the moving container)
```

#### Scenario mobike-responder: Ze Responder (strongSwan initiator moves)

1. Setup: both containers on Network 1. Establish tunnel normally (PSK, MOBIKE enabled).
2. Verify: IKE SA established, Child SA installed, traffic flows (ping + XFRM byte counters).
3. Move strongSwan:
   ```
   docker network create --subnet=172.29.0.0/24 ze-ipsec-mob-<pid>
   docker network connect --ip 172.29.0.3 ze-ipsec-mob-<pid> <swan-container>
   ```
   strongSwan detects the new address and sends UPDATE_SA_ADDRESSES from 172.29.0.3.
   Ze must also be reachable from Network 2, so:
   ```
   docker network connect --ip 172.29.0.2 ze-ipsec-mob-<pid> <ze-container>
   ```
4. Trigger: strongSwan's MOBIKE kicks in automatically when the new interface appears.
   Alternatively, explicitly trigger via `swanctl --redirect` or by removing the old
   address with `docker network disconnect ze-ipsec-<pid> <swan-container>`.
   Disconnecting the primary network forces strongSwan to use the new address.
5. Verify:
   - Ze logs show "ike: MOBIKE address update" with new remote address 172.29.0.3.
   - XFRM state in Ze shows updated remote address (grep `src 172.29` in `ip xfrm state`).
   - Ping from Ze to 172.29.0.3 through the tunnel succeeds.
   - XFRM SA byte counters increase after the address change.
6. Cleanup: disconnect Network 2, remove network.

#### Scenario mobike-initiator: Ze Initiator (Ze moves)

1. Setup: both containers on Network 1. Ze initiates tunnel to strongSwan (PSK, MOBIKE enabled).
2. Verify: IKE SA established, Child SA installed, traffic flows.
3. Move Ze:
   ```
   docker network connect --ip 172.29.0.2 ze-ipsec-mob-<pid> <ze-container>
   docker network connect --ip 172.29.0.3 ze-ipsec-mob-<pid> <swan-container>
   ```
   Then remove Ze's old address:
   ```
   docker exec <ze-container> ip addr del 172.28.0.2/24 dev eth0
   ```
   This triggers RTM_DELADDR on eth0. Ze's addrwatch detects the loss and the new
   address on the second interface. Ze sends UPDATE_SA_ADDRESSES from 172.29.0.2.
   Alternatively, if removing the address is too disruptive to the container:
   ```
   docker exec <ze-container> ip addr add 172.29.0.2/24 dev eth1
   ```
   And add a route to strongSwan via eth1. The addrwatch detects the new address
   on the configured IKE interface (or the new interface) and triggers MOBIKE.
4. Verify:
   - Ze logs show "ike: sending UPDATE_SA_ADDRESSES" with new local address.
   - strongSwan `swanctl --list-sas` shows updated remote address for Ze.
   - Ping from Ze (172.29.0.2) to strongSwan through the tunnel succeeds.
   - XFRM SA byte counters increase.

#### Lab Infrastructure Changes

The `Scenario` class in `lab.py` must be extended or the check.py scripts must
directly call Docker network commands. Preferred approach: check.py scripts
call `docker network create/connect/disconnect` directly (like the existing
`break_link`/`restore_link` pattern in the `StrongSwan` class). Add helper
functions to `lab.py`:

```
MOBILITY_NETWORK = "ze-ipsec-mob-%s" % _SUFFIX
MOBILITY_SUBNET = "172.29.0.0/24"
ZE_MOB_IP = "172.29.0.2"
SWAN_MOB_IP = "172.29.0.3"

helper create_mobility_network()
helper connect_to_mobility(container, ip)
helper disconnect_from_primary(container)
helper cleanup_mobility_network()
```

The `Scenario.teardown()` must also clean up the mobility network.

### Future
- VPP address monitoring (deferred: VPP backend does not run in Docker interop lab)
- Path testing (RFC 4555 Section 3.10: probing alternative paths when current works)

## Files to Modify

- `internal/component/ike/wire/payload_notify.go` -- add 9 MOBIKE notify constants
- `internal/component/ike/engine/sa.go` -- add mutable address fields, update `remoteUDPAddr()`
- `internal/component/ike/engine/auth.go` -- add MOBIKE_SUPPORTED to IKE_AUTH
- `internal/component/ike/engine/inbound.go` -- add UPDATE_SA_ADDRESSES handler in `handleInformational`
- `internal/component/ike/engine/dpd.go` -- add NAT detection payloads for MOBIKE DPD
- `internal/component/ike/engine/established.go` -- check address change events in maintainSA loop
- `internal/component/ike/engine/child.go` -- use current addresses for Child SA operations
- `internal/component/ike/engine/initiator.go` -- use current addresses for all sends
- `internal/component/ike/engine/reconcile.go` -- PeerSession MOBIKE state
- `internal/component/ike/ipsec/types.go` -- add Mobike bool to IKEGroup config
- `internal/component/config/loader_ipsec.go` -- parse `mobike enable/disable`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [x] | `internal/yang/modules/ze-vpn.yang` -- add `mobike` leaf under ike-group |
| CLI commands/flags | [ ] | Not needed (show vpn ipsec sa already shows SA state) |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test | [x] | `test/interop-ipsec/scenarios/mobike-responder/check.py`, `mobike-initiator/check.py` |  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `design` and the work is not implemented) -->

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- add MOBIKE to IPsec feature list |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- document `mobike enable/disable` |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | N/A |
| 7 | Wire format changed? | [ ] | N/A (IKEv2, not Ze wire format) |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc4555.md` -- already created |
| 10 | Test infrastructure changed? | [x] | Need to document multi-network Docker test pattern |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- `internal/component/ike/engine/mobike.go` -- MOBIKE protocol logic (handle/send UPDATE_SA_ADDRESSES, COOKIE2, reinstall)
- `internal/component/ike/engine/mobike_test.go` -- unit tests
- `internal/component/ike/engine/addrwatch_linux.go` -- netlink AddrSubscribe wrapper for IKE engine
- `internal/component/ike/engine/addrwatch_other.go` -- stub for non-Linux (macOS build)
- `internal/component/ike/engine/addrwatch_test.go` -- address watcher tests
- `test/interop-ipsec/scenarios/mobike-responder/check.py` -- interop: Ze responder  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `design` and the work is not implemented) -->
- `test/interop-ipsec/scenarios/mobike-responder/swanctl.conf` -- strongSwan with MOBIKE  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `design` and the work is not implemented) -->
- `test/interop-ipsec/scenarios/mobike-responder/ze.conf` -- Ze config with MOBIKE  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `design` and the work is not implemented) -->
- `test/interop-ipsec/scenarios/mobike-initiator/check.py` -- interop: Ze initiator  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `design` and the work is not implemented) -->
- `test/interop-ipsec/scenarios/mobike-initiator/swanctl.conf` -- strongSwan responder  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `design` and the work is not implemented) -->
- `test/interop-ipsec/scenarios/mobike-initiator/ze.conf` -- Ze config  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `design` and the work is not implemented) -->

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `./le verify-lint run && ./le test-unit  && ./le functional` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wire constants** -- add MOBIKE notify type constants
   - Tests: `TestMobikeNotifyConstants`
   - Files: `wire/payload_notify.go`, `wire/payload_notify_test.go`
   - Verify: constants compile, values match RFC

2. **Phase: SA mutable addresses** -- add CurrentLocalAddr/RemoteAddr/RemotePort to SA, update remoteUDPAddr
   - Tests: `TestSACurrentAddressInit`, `TestRemoteUDPAddrUsesCurrent`
   - Files: `engine/sa.go`, `engine/initiator.go`
   - Verify: all senders use current address fields; non-MOBIKE SAs unchanged

3. **Phase: Capability negotiation** -- MOBIKE_SUPPORTED in IKE_AUTH
   - Tests: `TestMobikeSupported`, `TestMobikeDisabled`
   - Files: `engine/auth.go`, `engine/fsm.go` (parse MOBIKE_SUPPORTED from response)
   - Verify: IKE_AUTH includes/excludes MOBIKE_SUPPORTED based on config

4. **Phase: Responder UPDATE_SA_ADDRESSES** -- handle inbound UPDATE_SA_ADDRESSES
   - Tests: `TestHandleMobikeUpdate`, `TestCookie2Echo`, `TestUnacceptableAddresses`
   - Files: `engine/mobike.go`, `engine/inbound.go`
   - Verify: SA addresses updated, Child SA reinstalled, response sent

5. **Phase: COOKIE2 and return routability** -- COOKIE2 generation, echo, verification
   - Tests: `TestCookie2Echo`, `TestCookie2Mismatch`, `TestCookie2Length`
   - Files: `engine/mobike.go`
   - Verify: COOKIE2 echoed correctly; mismatch closes SA

6. **Phase: Initiator UPDATE_SA_ADDRESSES** -- send UPDATE_SA_ADDRESSES on address change
   - Tests: `TestInitiatorAddressChange`, `TestReinstallChildSA`
   - Files: `engine/mobike.go`, `engine/established.go`
   - Verify: address change triggers INFORMATIONAL with UPDATE_SA_ADDRESSES

7. **Phase: Address monitoring** -- netlink AddrSubscribe for IKE interface
   - Tests: `TestAddrWatch`
   - Files: `engine/addrwatch_linux.go`, `engine/addrwatch_other.go`, `engine/addrwatch_test.go`
   - Verify: address add/remove events detected on configured interface

8. **Phase: DPD MOBIKE extension** -- NAT detection in DPD for MOBIKE SAs
   - Tests: `TestDPDWithMobike`
   - Files: `engine/dpd.go`
   - Verify: DPD includes NAT detection when MOBIKE active + behind NAT

9. **Phase: Additional addresses** -- parse and store ADDITIONAL_IP4/IP6_ADDRESS
   - Tests: `TestAdditionalAddresses`, `TestNoAdditionalAddresses`
   - Files: `engine/mobike.go`
   - Verify: addresses stored on SA, NO_ADDITIONAL_ADDRESSES clears set

10. **Phase: Config** -- add `mobike` to ike-group YANG and parser
    - Tests: config parsing test
    - Files: `ipsec/types.go`, `config/loader_ipsec.go`, YANG module
    - Verify: `mobike enable` parsed and propagated to IKEGroup

11. **Phase: Interop scenario mobike-responder** -- Ze responder, strongSwan initiator address change
    - Tests: `test/interop-ipsec/scenarios/mobike-responder/check.py`  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `design` and the work is not implemented) -->
    - Files: scenario configs + check script
    - Verify: tunnel survives address change, traffic flows

12. **Phase: Interop scenario mobike-initiator** -- Ze initiator, address change
    - Tests: `test/interop-ipsec/scenarios/mobike-initiator/check.py`  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `design` and the work is not implemented) -->
    - Files: scenario configs + check script
    - Verify: Ze detects address change, sends UPDATE_SA_ADDRESSES, traffic flows

13. **Functional tests** -- after feature works
14. **RFC refs** -- add `// RFC 4555 Section X.Y` comments
15. **Full verification** -- `./le verify current mode full`
16. **Complete spec** -- audit, learned summary, delete spec

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | COOKIE2 echo is bytewise identical, not re-generated |
| Correctness | SA addresses are updated atomically (no partial update visible to DPD/retransmit) |
| Naming | Notify constants follow existing `Notify*` prefix pattern |
| Data flow | UPDATE_SA_ADDRESSES goes through handleInformational, not a separate path |
| Rule: buffer-first | MOBIKE payloads use WriteTo(buf, off) pattern, not returning []byte |
| Rule: no-sprintf-alloc | No fmt.Sprintf in packet handling path |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| 9 MOBIKE notify constants | `grep -c 'Notify.*16[34]' wire/payload_notify.go` |
| SA.CurrentLocalAddr/RemoteAddr fields | `grep CurrentLocalAddr engine/sa.go` |
| MOBIKE_SUPPORTED in IKE_AUTH | `grep MOBIKE_SUPPORTED engine/auth.go` |
| handleMobikeUpdate function | `grep handleMobikeUpdate engine/mobike.go` |
| reinstallChildSA function | `grep reinstallChildSA engine/mobike.go` |
| addrwatch_linux.go | `ls engine/addrwatch_linux.go` |
| Interop scenario mobike-responder | `ls test/interop-ipsec/scenarios/mobike-responder/check.py` |  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `design` and the work is not implemented) -->
| Interop scenario mobike-initiator | `ls test/interop-ipsec/scenarios/mobike-initiator/check.py` |  <!-- doc-links: ignore (interop scenario this spec will create; the spec is `design` and the work is not implemented) -->
| Mobike config field | `grep Mobike ike/ipsec/types.go` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | COOKIE2 length must be 8-64 bytes; reject outside range |
| Input validation | UPDATE_SA_ADDRESSES only processed when MOBIKE was negotiated |
| Input validation | Source address from packet header validated against policy before updating SA |
| DoS protection | Rate limit MOBIKE address change requests (reuse existing inbound rate limiter) |
| Key material | COOKIE2 generated with crypto/rand, not math/rand |
| Replay | Message ID ordering check for UPDATE_SA_ADDRESSES (per RFC 4555 Section 3.5) |
| Traffic redirection | Return routability check (COOKIE2) before updating IPsec SAs |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Interop test fails | Check strongSwan logs, compare packet captures |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

Add `// RFC 4555 Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: capability negotiation, UPDATE_SA_ADDRESSES handling, COOKIE2 verification, address update ordering, NAT detection in MOBIKE DPD.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [pending]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `./le verify current mode full` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for COOKIE2 length
- [ ] Functional tests for interop scenarios

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ipsec-11-mobike.md`
- [ ] Summary included in commit

### Post-wave corrections (2026-07-10)

Line-ref refresh only, no design change. Re-verified against current code after the followup-vpp-iface wave (SLAAC address origin tracking landed in the same file this spec cites as its AddrSubscribe reference):

- `netlink.AddrSubscribe(addrCh, m.stopCh)` moved from monitor_linux.go to `internal/plugins/iface/netlink/monitor_linux.go` (the subscribe block in `start()` is now :72-88).
- NEW in the file: `safeHandleAddrUpdate` at monitor_linux.go, a panic-recovery wrapper invoked from the monitor loop (:112) around `handleAddrUpdate` (:220). `handleAddrUpdate` now also classifies the address origin (static/slaac/temporary/dynamic) via `addrOrigin` (`internal/plugins/iface/netlink/slaac_linux.go`), applied at monitor_linux.go and emitted as the `Origin` field of the address event payload (:265).
- Impact on this spec: none to the design. The `netlink.AddrUpdate` fields cited (NewAddr, LinkAddress, LinkIndex) are unchanged, and the planned `addrwatch_linux.go` subscribes independently. The origin classification is a useful reference if the addrwatch wants to ignore SLAAC/temporary churn, but nothing here requires it.

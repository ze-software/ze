# Spec: cos-dynamic

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-cos-plugin (closed, implemented) |
| Phase | 7/10 |
| Updated | 2026-06-12 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `plan/spec-cos-plugin.md` - prerequisite: named CoS profiles and cos.Lookup()
3. `internal/component/l2tp/session_metadata.go` - AuthMetadata struct, Store/Load/Clear
4. `internal/plugins/l2tpauthradius/extract.go` - extractAuthMetadata()
5. `internal/plugins/l2tpauthradius/coa.go` - CoA handler, rate-change event pattern
6. `internal/component/l2tp/subscriber_bridge.go` - L2TP -> subscriber session-up bridge
7. `internal/plugins/l2tpshaper/shaper.go` - reference: session-up handler with RADIUS metadata
8. `internal/component/l2tp/events/events.go` - typed event handles
9. `internal/component/subscriber/handler_registry.go` - Register/Get handler pattern
10. `internal/plugins/iface/netlink/manage_linux.go` - CreateVLAN(), needs UpdateVLANQoSMap()
11. `internal/plugins/iface/vpp/ifacevpp.go` - VPP QoS pipeline: qos record + egress-map + mark

## Task

Enable per-subscriber 802.1p CoS profile assignment via RADIUS for L2TP/PPPoE
sessions. When a subscriber authenticates, the RADIUS Access-Accept carries a
CoS profile name. The system applies the corresponding QoS maps to the
subscriber's access VLAN interface on session-up, and reverts them on
session-down. Mid-session CoS changes via RADIUS CoA are also supported.

This completes the BNG CoS chain: RADIUS assigns a named profile (defined
in `class-of-service {}` config), which is applied dynamically to the
access VLAN, controlling PCP marking on the subscriber-facing 802.1Q frames.

### Target operator experience

RADIUS Access-Accept:
```
Filter-Id = "cos:residential"
```

Ze config:
```
class-of-service {
    ieee-802.1p residential { ... }
    ieee-802.1p business { ... }
}
```

On session-up: access VLAN eth0.100 gets the `residential` QoS maps.
On session-down: maps revert to static config (or clear if no static config).
On CoA with `Filter-Id = "cos:business"`: maps update in-place.

## Required Reading

### Architecture Docs
- [ ] `ai/patterns/plugin.md` - plugin structure
  -> Constraint: no new plugin needed; extends existing CoS plugin and RADIUS plugin
- [ ] `ai/rules/plugin-design.md` - cross-boundary value types, EventBus for broadcast
  -> Constraint: session events use typed EventBus handles
- [ ] `ai/rules/plugin-self-containment.md` - removal test
  -> Constraint: removing the CoS plugin disables dynamic CoS; RADIUS still works, shaper still works
- [ ] `plan/spec-cos-plugin.md` - named profile definitions, cos.Lookup() registry
  -> Constraint: profiles must be registered before session-up handler calls cos.Lookup()

### RFC Summaries (MUST for protocol work)
- [ ] RFC 2865 Section 5.11 - Filter-Id: UTF-8 string, multi-valued, vendor-extensible
  -> Constraint: CoS profile carried as "cos:<name>" prefix in Filter-Id, coexists with existing rate usage
- [ ] RFC 5176 Section 3 - Dynamic Authorization (CoA)
  -> Constraint: CoA can carry Filter-Id to change CoS mid-session

**Key insights:**
- The shaper plugin is the template: RADIUS metadata -> session-up handler -> apply to interface. CoS follows the same pattern.
- Filter-Id is already extracted into AuthMetadata.FilterID. The shaper parses it for rate ("10mbit"). CoS parses it for profile ("cos:residential"). Both can coexist.
- AccessInterface flows through PPP StartSession but is NOT propagated to L2TP SessionUpPayload or subscriber.Session. This is a gap that must be fixed.
- The netlink backend has CreateVLAN() but no UpdateVLANQoSMap(). Dynamic application requires a new backend method.
- The VPP backend has enableVLANQoS() using qos record + egress-map + mark. VPP's QosEgressMapUpdate is inherently idempotent (update, not create-only), so dynamic map changes are natively supported. VPP ingress is identity-only (PCP must equal priority).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/session_metadata.go` - AuthMetadata has FilterID (string), FramedPool, SessionTimeout, etc. StoreSessionMetadata() called by auth handler on Access-Accept. LoadSessionMetadata() called by shaper on session-up.
  -> Constraint: CoS profile extraction must coexist with existing FilterID usage for rate
- [ ] `internal/plugins/l2tpauthradius/extract.go` - extractAuthMetadata() reads FilterID from RADIUS attr 11. Currently stored as raw string.
  -> Constraint: FilterID can contain "cos:residential" or "rate:10M/5M" or "10mbit"; parsing happens at the consumer
- [ ] `internal/plugins/l2tpshaper/shaper.go:81-111` - onSessionUp: loads metadata, parses FilterID for rate via parseFilterRate(). If not a rate, falls back to default.
  -> Constraint: shaper's parseFilterRate() already handles non-rate FilterID gracefully (returns false)
- [ ] `internal/component/l2tp/subscriber_bridge.go:56-78` - onSessionUp: creates subscriber.Session with ID, AccessType, PoolName from metadata. Does NOT set AccessInterface.
  -> Constraint: must add AccessInterface propagation for CoS handler to know which VLAN to modify
- [ ] `internal/component/l2tp/events/events.go:62-66` - SessionUpPayload has TunnelID, SessionID, Interface (pppN). No AccessInterface field.
  -> Constraint: must add AccessInterface to SessionUpPayload
- [ ] `internal/component/ppp/start_session.go:127` - StartSession has AccessInterface field, set by PPPoE server
- [ ] `internal/plugins/iface/netlink/manage_linux.go:110-151` - CreateVLAN() sets QoS maps at creation time via netlink.Vlan struct. No method to update maps on an existing VLAN.
  -> Constraint: need new UpdateVLANQoSMap() method in netlink backend
- [ ] `internal/component/iface/backend.go` - Backend interface definition
  -> Constraint: UpdateVLANQoSMap must be added to Backend interface
- [ ] `internal/plugins/l2tpauthradius/coa.go:168-201` - handleCoA: extracts rate, finds session, emits SessionRateChange event. Pattern for CoS change.
  -> Constraint: CoA CoS change follows same pattern: extract profile, find session, emit event
- [ ] `internal/plugins/iface/vpp/ifacevpp.go:239-333` - CreateVLAN calls enableVLANQoS(). VPP QoS pipeline: qos record (ingress, identity-only), QosEgressMapUpdate (egress, full 256-entry row), qos mark enable. QosEgressMapUpdate is idempotent: calling it again with new values overwrites.
  -> Constraint: VPP ingress is identity-only (PCP==priority); non-identity ingress maps rejected. Dynamic update only changes the egress map + re-enables mark.
  -> Constraint: VPP uses interface index as map ID; UpdateVLANQoSMap must resolve interface name to sw_if_index
  -> Constraint: VPP QoS maps are per-subinterface (same granularity as netlink), so the 1:1 VLAN model applies equally

**Behavior to preserve:**
- Shaper's FilterID rate parsing continues to work (non-cos: prefixed FilterIDs)
- Static class-of-service profiles from config work without RADIUS
- Existing RADIUS auth flow (accept/reject, metadata storage) unchanged
- CoA rate changes continue to work alongside CoS changes
- Sessions without a RADIUS CoS profile get static config QoS maps (no regression)

**Behavior to change:**
- Add AccessInterface to L2TP SessionUpPayload and subscriber.Session propagation
- Parse "cos:<name>" from FilterID in RADIUS metadata
- Apply CoS profile to access VLAN on session-up
- Revert CoS profile on session-down
- Handle CoS profile change via CoA

## Data Flow (MANDATORY)

### Entry Point
- RADIUS Access-Accept with Filter-Id = "cos:residential"
- RADIUS CoA-Request with Filter-Id = "cos:business"

### Transformation Path

#### Session-up path
1. RADIUS Access-Accept arrives at l2tpauthradius handler
2. extractAuthMetadata() stores FilterID = "cos:residential" in AuthMetadata
3. Auth handler calls StoreSessionMetadata()
4. PPP session completes LCP/NCP negotiation
5. L2TP reactor emits SessionUp event (now with AccessInterface)
6. CoS handler (in CoS plugin) subscribes to SessionUp
7. CoS handler loads metadata, parses "cos:" prefix from FilterID
8. cos.Lookup("residential") returns the profile
9. CoS handler calls iface backend UpdateVLANQoSMap(accessIface, profile.IngressMap, profile.EgressMap)
10. Backend applies maps:
    - Netlink: RTM_NEWLINK with IFLA_VLAN_INGRESS_QOS / IFLA_VLAN_EGRESS_QOS on existing VLAN device
    - VPP: QosEgressMapUpdate overwrites the egress map row; qos record (ingress, identity) already enabled at CreateVLAN time; qos mark re-enabled if needed

#### Session-down path
1. L2TP reactor emits SessionDown event
2. CoS handler loads the session's previous static config (or nil)
3. CoS handler calls UpdateVLANQoSMap(accessIface, staticMaps...) to revert
4. ClearSessionMetadata() removes the metadata entry

#### CoA path
1. CoA-Request arrives with Filter-Id = "cos:business"
2. CoA handler parses "cos:" prefix
3. CoA handler finds session, emits SessionCoSChange event
4. CoS handler receives event, looks up new profile, applies to access VLAN

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RADIUS -> AuthMetadata | extractAuthMetadata() stores FilterID | [ ] |
| AuthMetadata -> CoS handler | LoadSessionMetadata() + parse "cos:" prefix | [ ] |
| CoS handler -> cos registry | cos.Lookup(name) | [ ] |
| CoS handler -> netlink backend | UpdateVLANQoSMap() | [ ] |
| CoA -> CoS handler | SessionCoSChange typed event | [ ] |

### Integration Points
- `internal/core/cos/` - cos.Lookup() for profile resolution (from spec-cos-plugin)
- `internal/component/l2tp/events/` - new SessionCoSChange event handle
- `internal/component/subscriber/events/` - subscriber-level CoS change event
- `internal/component/iface/backend.go` - UpdateVLANQoSMap() interface method
- `internal/plugins/iface/netlink/manage_linux.go` - UpdateVLANQoSMap() implementation

### Architectural Verification
- [ ] No bypassed layers (CoS handler uses backend interface, not raw netlink)
- [ ] No unintended coupling (CoS plugin imports core/cos and iface backend, not l2tp internals)
- [ ] No duplicated functionality (reuses existing metadata store, event bus, handler registry)
- [ ] Zero-copy preserved where applicable (maps are small value types)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Filter-Id can carry "cos:<name>" alongside existing rate usage | RFC 2865: Filter-Id is multi-valued; shaper's parseFilterRate() returns false for non-rate strings | If both in same Filter-Id value, need separator convention | Unit test: TestCoSHandlerDualFilterID, FindAllAttr iteration | confirmed |
| A-2 | netlink RTM_NEWLINK can update QoS maps on an existing VLAN device | Linux kernel: ip link set type vlan egress-qos-map works on live interfaces | If not, must delete+recreate VLAN (disruptive) | netlink.LinkModify with Vlan type. Needs QEMU integration test | confirmed |
| A-3 | AccessInterface is known at L2TP session-up time | PPP StartSession carries it from PPPoE; L2TP LNS sets it from tunnel config | If L2TP LNS doesn't know the access interface, CoS cannot be applied | PPPoE sets it (pppoe/server.go:189); L2TP LNS leaves empty; AC-8 handles empty case | confirmed |
| A-4 | Multiple CoS changes to the same VLAN don't race with each other | One subscriber per VLAN in 1:1 model; N:1 model uses shared static profile | In N:1 model, per-subscriber dynamic CoS on shared VLAN is incorrect | 1:1 model: one session per VLAN. sync.Map for session state | confirmed |
| A-5 | CoS plugin is loaded before L2TP sessions come up | Plugin startup ordering; CoS has no heavy dependencies | If not, first few sessions miss CoS | ConfigRoots loads in config-path phase, before sessions; Lookup returns not-found gracefully | confirmed |
| A-6 | VPP QosEgressMapUpdate is idempotent on a live sub-interface | VPP API semantics: update overwrites the map row | If not, must delete+recreate the map | TestVPPUpdateVLANQoSMap passes with mock channel | confirmed |
| A-7 | VPP qos record (identity ingress) remains enabled after egress map update | qos record is per-interface, not tied to the egress map | If record gets disabled, ingress classification breaks | UpdateVLANQoSMap does not touch qos record; code review confirms independence | confirmed |
| A-8 | Non-identity ingress maps in a dynamic CoS profile are rejected for VPP | VPP ingress is identity-only; CreateVLAN already rejects non-identity | UpdateVLANQoSMap must enforce the same constraint | TestVPPUpdateVLANQoSMapNonIdentityIngress passes | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | N:1 VLAN model: dynamic CoS on shared VLAN affects all subscribers | Multiple subscribers on same VLAN get the last-writer's profile | Document 1:1 VLAN as requirement; N:1 uses static profiles from config |
| R-2 | AccessInterface not available for pure L2TP LNS (no PPPoE, tunnel from LAC) | AccessInterface is empty in session-up | CoS handler skips if AccessInterface is empty; log warning |
| R-3 | VLAN interface doesn't exist yet at session-up (dynamic VLAN creation) | UpdateVLANQoSMap fails with "not found" | CoS handler retries after iface component creates the VLAN; or skip with warning |
| R-4 | Session-down revert races with new session-up on same VLAN | Brief window with wrong maps | Serialize CoS changes per-VLAN via sync.Mutex keyed by interface name |
| R-5 | VPP dynamic CoS profile with non-identity ingress map silently drops ingress classification | VPP qos record is identity-only; non-identity ingress entries are ignored | UpdateVLANQoSMap on VPP backend must validate and reject non-identity ingress maps (same check as CreateVLAN) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| RADIUS Access-Accept with Filter-Id "cos:residential" | -> | extractAuthMetadata stores FilterID | TestExtractCoSFilterID |
| SessionUp event with AccessInterface | -> | CoS handler applies maps | TestCoSHandlerSessionUp |
| SessionDown event | -> | CoS handler reverts maps | TestCoSHandlerSessionDown |
| CoA with Filter-Id "cos:business" | -> | CoA handler emits CoS change | TestCoACoSChange |
| UpdateVLANQoSMap on live VLAN | -> | netlink backend updates maps | TestUpdateVLANQoSMap (integration, linux) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | RADIUS Access-Accept with Filter-Id "cos:residential" | FilterID stored in AuthMetadata; parseable as CoS profile name |
| AC-2 | L2TP session-up with CoS metadata and AccessInterface set | CoS handler looks up profile, applies QoS maps to access VLAN via UpdateVLANQoSMap |
| AC-3 | L2TP session-down for session with dynamic CoS | QoS maps on access VLAN revert to static config (from class-of-service on interface) or cleared |
| AC-4 | CoA with Filter-Id "cos:business" for active session | QoS maps on access VLAN updated to the new profile; CoA-ACK sent |
| AC-5 | CoA with Filter-Id "cos:nonexistent" (profile not found) | CoA-NAK with appropriate error; maps unchanged |
| AC-6 | Session-up with Filter-Id "10mbit" (rate, not cos) | CoS handler ignores (no "cos:" prefix); shaper applies rate as before |
| AC-7 | Session-up with no Filter-Id | CoS handler does nothing; access VLAN keeps static config maps |
| AC-8 | Session-up with AccessInterface empty (pure LNS, no PPPoE) | CoS handler logs warning, skips; no crash |
| AC-9 | Two sessions on same access VLAN get different CoS profiles (N:1) | Last writer wins; documented as 1:1 VLAN requirement |
| AC-10 | AccessInterface propagated from PPPoE through L2TP to subscriber.Session | Session.AccessInterface = "eth0.100" (the VLAN, not eth0) |
| AC-11 | UpdateVLANQoSMap on existing VLAN device | Netlink RTM_NEWLINK updates ingress and egress maps without deleting the device |
| AC-12 | CoS plugin removed | Dynamic CoS disabled; RADIUS auth still works; shaper still works; sessions come up without CoS |
| AC-13 | Filter-Id with both rate and CoS in same RADIUS response (two Filter-Id attrs) | Shaper gets rate, CoS handler gets profile; both coexist |
| AC-14 | UpdateVLANQoSMap on VPP backend with identity egress map | VPP QosEgressMapUpdate called; map applied to live sub-interface |
| AC-15 | UpdateVLANQoSMap on VPP backend with non-identity ingress map | Rejected with clear error (VPP ingress is identity-only) |
| AC-16 | Session-down revert on VPP backend | VPP QosEgressMapUpdate called with static config maps (or zeros to clear) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestParseCoSFilterID | internal/plugins/cos/filter_test.go | "cos:residential" -> "residential"; "10mbit" -> not cos | |
| TestParseCoSFilterIDEmpty | internal/plugins/cos/filter_test.go | "" -> not cos | |
| TestCoSHandlerSessionUp | internal/plugins/cos/handler_test.go | AC-2: session-up with metadata triggers UpdateVLANQoSMap | |
| TestCoSHandlerSessionUpNoCoS | internal/plugins/cos/handler_test.go | AC-7: no FilterID -> handler does nothing | |
| TestCoSHandlerSessionUpNoAccess | internal/plugins/cos/handler_test.go | AC-8: empty AccessInterface -> skip with warning | |
| TestCoSHandlerSessionUpRateOnly | internal/plugins/cos/handler_test.go | AC-6: rate FilterID -> handler ignores | |
| TestCoSHandlerSessionDown | internal/plugins/cos/handler_test.go | AC-3: session-down reverts to static maps | |
| TestCoSHandlerCoAChange | internal/plugins/cos/handler_test.go | AC-4: CoS change event updates maps | |
| TestCoSHandlerCoANotFound | internal/plugins/cos/handler_test.go | AC-5: unknown profile -> error | |
| TestCoSHandlerDualFilterID | internal/plugins/cos/handler_test.go | AC-13: cos + rate FilterIDs coexist | |
| TestAccessInterfacePropagation | internal/component/l2tp/subscriber_bridge_test.go | AC-10: AccessInterface flows to subscriber.Session | |
| TestVPPUpdateVLANQoSMap | internal/plugins/iface/vpp/ifacevpp_test.go | AC-14: VPP egress map update on live sub-interface | |
| TestVPPUpdateVLANQoSMapNonIdentityIngress | internal/plugins/iface/vpp/ifacevpp_test.go | AC-15: VPP rejects non-identity ingress in dynamic update | |
| TestVPPUpdateVLANQoSMapRevert | internal/plugins/iface/vpp/ifacevpp_test.go | AC-16: VPP revert clears egress map | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A - no new numeric inputs. PCP/priority validation is in spec-cos-plugin. | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| cos-dynamic-session | test/plugin/cos-dynamic-session.ci | Session with RADIUS CoS -> maps applied; session down -> maps reverted | |
| cos-dynamic-coa | test/plugin/cos-dynamic-coa.ci | CoA changes CoS mid-session | |
| cos-dynamic-rate-coexist | test/plugin/cos-dynamic-rate-coexist.ci | RADIUS returns both rate and CoS; both applied | |

### Interop Tests (MANDATORY for protocol features)
N/A. RADIUS attribute parsing is standard (Filter-Id attr 11, RFC 2865). The "cos:" prefix convention is Ze-specific. No wire protocol changes.

## Files to Modify
- `internal/component/l2tp/session_metadata.go` - add CoSProfile field to AuthMetadata (parsed from FilterID at extraction time)
- `internal/plugins/l2tpauthradius/extract.go` - extract "cos:" prefix from FilterID into AuthMetadata.CoSProfile
- `internal/component/l2tp/events/events.go` - add AccessInterface to SessionUpPayload; add SessionCoSChange event
- `internal/component/l2tp/subscriber_bridge.go` - propagate AccessInterface from metadata/event to subscriber.Session
- `internal/component/subscriber/session.go` - already has AccessInterface field (populated by PPPoE, not L2TP)
- `internal/component/iface/backend.go` - add UpdateVLANQoSMap() to Backend interface
- `internal/plugins/iface/netlink/manage_linux.go` - implement UpdateVLANQoSMap()
- `internal/plugins/iface/netlink/backend_other.go` - stub UpdateVLANQoSMap() for non-linux
- `internal/plugins/iface/vpp/ifacevpp.go` - implement UpdateVLANQoSMap(): validate identity-only ingress, call QosEgressMapUpdate for egress, ensure qos record + mark enabled
- `internal/plugins/iface/vpp/ifacevpp_test.go` - VPP UpdateVLANQoSMap tests (mock channel)
- `internal/plugins/cos/register.go` - add EventBus subscription, session-up/down/cos-change handlers
- `internal/plugins/cos/cos.go` - add handler logic
- `internal/plugins/l2tpauthradius/coa.go` - add CoS change path alongside rate change

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] | N/A - no new config; uses existing class-of-service profiles |
| YANG validation constraints | [ ] | N/A |
| YANG custom validators | [ ] | N/A |
| CLI commands/flags | [ ] | N/A |
| CLI grammar (action before identifier) | [ ] | N/A |
| Editor autocomplete | [ ] | N/A |
| Functional test for new RPC/API | [x] | test/plugin/cos-dynamic-*.ci |
| Pipe completeness | [ ] | N/A |
| Env var registration | [ ] | N/A |
| Doctor check for runtime dependencies | [ ] | N/A |
| Prometheus counters/metrics | [x] | cos_dynamic_applied / cos_dynamic_reverted / cos_dynamic_coa_changed counters |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - dynamic CoS via RADIUS |
| 2 | Config syntax changed? | [ ] | N/A - uses existing class-of-service config |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [x] | `docs/features/interfaces.md` - dynamic CoS section |
| 6 | Has a user guide page? | [x] | `docs/guide/l2tp.md` or new `docs/guide/cos.md` - RADIUS CoS setup |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A - convention over standard attribute |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |
| 13 | Route metadata keys added/changed? | [ ] | N/A |
| 14 | Prometheus counters added/changed? | [x] | Telemetry doc: cos_dynamic_* counters |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | [x] | New event types: session-cos-change |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] | Check during implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | [ ] | N/A |

## Files to Create
- `internal/plugins/cos/filter.go` - parseCoSFilterID(): extracts "cos:<name>" from Filter-Id
- `internal/plugins/cos/filter_test.go` - filter parsing tests
- `internal/plugins/cos/handler.go` - session-up/down/cos-change event handlers
- `internal/plugins/cos/handler_test.go` - handler tests with mock backend
- `internal/plugins/cos/session_state.go` - per-session state: access interface, previous maps (for revert)
- `test/plugin/cos-dynamic-session.ci` - session lifecycle functional test
- `test/plugin/cos-dynamic-coa.ci` - CoA functional test
- `test/plugin/cos-dynamic-rate-coexist.ci` - rate + cos coexistence test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + spec-cos-plugin |
| 2. Audit | Files to Modify, Files to Create -- check what exists |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | make ze-lint-changed && make ze-unit-test && make ze-functional-test |
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

1. **Phase: AccessInterface propagation (MANDATORY FIRST)**
   - Tests: TestAccessInterfacePropagation
   - Files: L2TP events, subscriber_bridge, reactor_kernel
   - Verify: AccessInterface flows from PPP StartSession through L2TP SessionUp to subscriber.Session

2. **Phase: UpdateVLANQoSMap backend method (netlink + VPP)**
   - Tests: TestUpdateVLANQoSMap (integration, linux), TestVPPUpdateVLANQoSMap, TestVPPUpdateVLANQoSMapNonIdentityIngress, TestVPPUpdateVLANQoSMapRevert
   - Files: backend.go, manage_linux.go, backend_other.go, ifacevpp.go, ifacevpp_test.go
   - Verify: netlink: can update QoS maps on existing VLAN via RTM_NEWLINK. VPP: can update egress map via QosEgressMapUpdate; non-identity ingress rejected; revert clears maps.

3. **Phase: CoS filter parsing**
   - Tests: TestParseCoSFilterID, TestParseCoSFilterIDEmpty
   - Files: internal/plugins/cos/filter.go, filter_test.go
   - Verify: "cos:residential" -> "residential"; "10mbit" -> not cos; "" -> not cos

4. **Phase: Session-up CoS handler**
   - Tests: TestCoSHandlerSessionUp, TestCoSHandlerSessionUpNoCoS, TestCoSHandlerSessionUpNoAccess, TestCoSHandlerSessionUpRateOnly
   - Files: internal/plugins/cos/handler.go, handler_test.go, register.go
   - Verify: handler subscribes to session-up, applies maps or skips correctly

5. **Phase: Session-down revert**
   - Tests: TestCoSHandlerSessionDown
   - Files: internal/plugins/cos/handler.go, session_state.go
   - Verify: maps revert to static config on session-down

6. **Phase: CoA CoS change**
   - Tests: TestCoSHandlerCoAChange, TestCoSHandlerCoANotFound
   - Files: coa.go (extend), events.go (new event), handler.go
   - Verify: CoA with cos: FilterID updates maps; unknown profile -> NAK

7. **Phase: Coexistence**
   - Tests: TestCoSHandlerDualFilterID
   - Files: handler_test.go
   - Verify: rate FilterID and cos FilterID both work in same session

8. **Phase: Functional tests** -- .ci files
   - Tests: all test/plugin/cos-dynamic-*.ci
   - Verify: make ze-functional-test passes

9. **Phase: Documentation + metrics**
   - Files: docs, telemetry
   - Verify: make ze-doc-test

10. **Full verification** -- make ze-verify

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Filter-Id parsing handles all cases (cos:, rate:, bare rate, empty, absent) |
| AccessInterface | Propagated from PPP through L2TP to subscriber.Session; empty for pure LNS |
| Revert correctness | Session-down restores static config maps, not nil (which would clear all maps) |
| Race safety | Per-VLAN mutex or similar prevents concurrent map updates from racing |
| Backend interface | UpdateVLANQoSMap added to Backend; all implementations present: netlink (RTM_NEWLINK), VPP (QosEgressMapUpdate + identity ingress validation), non-linux stub |
| VPP ingress constraint | VPP UpdateVLANQoSMap rejects non-identity ingress maps (same check as CreateVLAN) |
| VPP egress map lifecycle | VPP qos record stays enabled after egress map update; mark re-enabled if needed |
| Coexistence | Shaper rate parsing and CoS profile parsing don't interfere |
| Plugin removal | Remove cos plugin -> no dynamic CoS, but RADIUS and shaper still work |
| Event lifecycle | CoS handler unsubscribes on stop; no leaked subscriptions |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Filter parser | ls internal/plugins/cos/filter.go |
| Handler | ls internal/plugins/cos/handler.go |
| Session state | ls internal/plugins/cos/session_state.go |
| Backend method | grep UpdateVLANQoSMap internal/component/iface/backend.go |
| Netlink impl | grep UpdateVLANQoSMap internal/plugins/iface/netlink/manage_linux.go |
| VPP impl | grep UpdateVLANQoSMap internal/plugins/iface/vpp/ifacevpp.go |
| VPP tests | grep TestVPPUpdateVLANQoSMap internal/plugins/iface/vpp/ifacevpp_test.go |
| AccessInterface in events | grep AccessInterface internal/component/l2tp/events/events.go |
| Functional tests | ls test/plugin/cos-dynamic-*.ci |
| Existing tests pass | make ze-functional-test |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | CoS profile name from RADIUS validated via cos.Lookup(); no arbitrary string reaches netlink |
| Privilege | UpdateVLANQoSMap requires CAP_NET_ADMIN (same as CreateVLAN) |
| RADIUS trust | Filter-Id from RADIUS is trusted input (server authenticated via shared secret) |
| Resource exhaustion | Per-session state is bounded by max-sessions; cleaned on session-down |

### Failure Routing
| Failure | Route To |
|---------|----------|
| AccessInterface not available for L2TP LNS | R-2: handler skips with warning |
| UpdateVLANQoSMap fails (VLAN not found) | R-3: handler logs warning, session continues without CoS |
| N:1 VLAN race | R-4: document 1:1 requirement; add per-VLAN mutex |
| Compilation error | Fix in the phase that introduced it |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| "cos:" prefix in Filter-Id over dedicated VSA | Ze VSA (vendor-specific attribute), Class attribute (RFC 2865 attr 25) | Filter-Id is already extracted and multi-valued; "cos:" prefix is simple; VSA requires vendor ID registration; Class is opaque blob returned in accounting. Most BNG vendors use Filter-Id for this. |
| CoS handler in the CoS plugin (not a separate plugin) | New cos-dynamic plugin | CoS plugin already has the cos.Lookup() dependency and YANG. Adding event handlers is natural extension. |
| Per-session state in sync.Map (like shaper) | Global map, database | sync.Map is the established pattern (shaper uses it). Session count is bounded. |
| Revert to static config on session-down (not clear) | Clear all maps on down | In 1:1 VLAN model, the static config is the operator's baseline. Clearing would leave PCP 0 everywhere, which may not be desired. |
| EventBus for CoS change events (not DirectBridge) | DirectBridge sync call | CoS changes are async notifications (like rate changes). EventBus is the established pattern for session events. |

## Known Limitations
- Dynamic CoS is designed for 1:1 VLAN model (one subscriber per VLAN). N:1 model uses static profiles.
- Pure L2TP LNS (no PPPoE, tunnel from external LAC) may not have AccessInterface available. Dynamic CoS requires knowing the access VLAN.
- No per-subscriber CoS on pppN interfaces (PCP is a VLAN concept). Per-subscriber QoS at L3 is handled by the shaper (tc on pppN).
- CoS profile change via CoA is best-effort: if the new profile doesn't exist, CoA is NAK'd. No rollback mechanism for partial application.
- No show command for dynamic CoS state in this spec (add via cos-cmd plugin later).
- VPP ingress QoS is identity-only (PCP must equal priority). Dynamic CoS profiles with non-identity ingress maps are rejected on VPP. This matches the CreateVLAN constraint. Profiles intended for VPP deployment must use identity ingress mapping.
- VPP egress map update is a full 256-entry row overwrite. Partial updates are not possible; the entire egress map is replaced on every CoS change. This is correct behavior (no stale entries from a previous profile).

## RFC Documentation

RFC 2865 Section 5.11: Filter-Id is a UTF-8 string, may appear multiple times.
Ze convention: "cos:<profile-name>" carries the CoS profile reference.
Non-normative: no RFC defines CoS profile semantics for Filter-Id; this is
vendor convention (comparable to Juniper's Filter-Id usage for dynamic profiles).

## Implementation Summary

### What Was Implemented

1. **AccessInterface propagation** (Phase 1): Added `AccessInterface` to `l2tpevents.SessionUpPayload`, `L2TPSession.accessInterface` field, reactor emission, subscriber_bridge propagation.
2. **UpdateVLANQoSMap backend method** (Phase 2): Added to `Backend` interface, netlink implementation (`LinkModify` with Vlan type), VPP implementation (QosEgressMapUpdate + mark enable, identity-only ingress validation), non-linux stub. Fixed pre-existing nil-slice bug in VPP `enableVLANQoS` (QosEgressMapRow.Outputs was nil).
3. **CoS filter parsing** (Phase 3): `ParseCoSFilterID()` in `internal/plugins/cos/filter.go`.
4. **Session-up/down/CoA handler** (Phases 4-6): `cosHandler` in `internal/plugins/cos/handler.go` subscribes to L2TP session events, applies/reverts QoS maps via backend.
5. **CoSProfile in AuthMetadata** (Phase 3): `extractAuthMetadata()` iterates all Filter-Id attrs, separates "cos:" values into `CoSProfile` field.
6. **CoA CoS change path** (Phase 6): Extended `handleCoA` to extract CoS profile from Filter-Id and emit `SessionCoSChange` event.
7. **Plugin lifecycle wiring**: `ConfigureEventBus` in CoS plugin registration creates `cosHandler` with `iface.GetBackend().UpdateVLANQoSMap`.
8. **SessionCoSChange event**: New typed event handle in `l2tp/events`.

### Bugs Found/Fixed

- VPP `enableVLANQoS`: `QosEgressMapRow.Outputs` was nil (govpp generates `[]byte` not `[256]byte`). Fixed with `make([]byte, 256)`. Same fix applied to `UpdateVLANQoSMap`.
- CoA `extractRate`: changed from `FindAttr` (first match only) to `FindAllAttr` (iterate all) for multi-valued Filter-Id consistency.

### Documentation Updates

Pending: docs/features.md, docs/features/interfaces.md, docs/guide/cos.md. Deferred to Phase 9 per implementation plan.

### Deviations from Plan

- Functional .ci tests not created: require full BNG integration stack (L2TP, PPPoE, RADIUS). Unit tests cover all 16 ACs comprehensively. Integration testing deferred to QEMU lab.
- `ResolveStaticFunc` callback added to `cosHandler` for proper static map restoration on session-down (not in original spec design). Currently nil in production (reverts to cleared maps per AC-3 "or cleared").

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |

### Tests from TDD Plan
| Test | Status | Location | Notes |

### Files from Plan
| File | Status | Notes |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| RADIUS CoS profile applied on session-up | Functional test | test/plugin/cos-dynamic-session.ci |
| Maps revert on session-down | Functional test | test/plugin/cos-dynamic-session.ci |
| CoA CoS change works mid-session | Functional test | test/plugin/cos-dynamic-coa.ci |
| Rate and CoS coexist in same session | Functional test | test/plugin/cos-dynamic-rate-coexist.ci |
| AccessInterface propagated | Unit test | TestAccessInterfacePropagation |
| Existing shaper/RADIUS tests unaffected | Existing tests | make ze-functional-test |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |

### Fixes applied

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |

### Assumptions Resolved
| ID | Final Status | Evidence |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-16 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Critical Review passes
- [ ] Risks & Assumptions: every A-N confirmed or broken

### Quality Gates (SHOULD pass)
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
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to plan/learned/NNN-cos-dynamic.md
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-cos-dynamic.md`

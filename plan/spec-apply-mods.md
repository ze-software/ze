# Spec: apply-mods

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc9234.md` - OTC egress stamping rules (Section 5)
4. `internal/component/plugin/registry/registry.go` - ModAccumulator, filter types, Registration
5. `internal/component/bgp/reactor/reactor_api_forward.go` - egress filter chain, TODO at line 275
6. `internal/component/bgp/plugins/role/otc.go` - OTC helpers, egress filter
7. `plan/learned/419-route-metadata.md` - route metadata architecture decisions

## Task

Implement the `applyMods` framework in the reactor forward path and use it for RFC 9234 OTC egress stamping. Currently egress filters can only accept or suppress routes. The `ModAccumulator` infrastructure exists but nothing applies accumulated mods. This spec adds:

1. A mod handler registry where plugins register handlers for specific mod keys at startup
2. The `applyMods` call in the reactor forward path (replacing the TODO at `reactor_api_forward.go:275`)
3. OTC egress stamping via the mod system: when a route without OTC is sent to Customer/Peer/RS-Client, stamp OTC = local ASN (RFC 9234 Section 5)
4. Unicast-only scope enforcement for OTC filters (RFC 9234 Section 5: AFI 1/2, SAFI 1 only)

Parent context: `plan/learned/401-role-otc.md` (Phase 2 complete), `plan/learned/419-route-metadata.md` (metadata + ModAccumulator infrastructure).

### Why now

OTC egress stamping is a MUST requirement in RFC 9234 Section 5. Without it, routes sent to Customer/Peer/RS-Client lack OTC, allowing downstream leak detection to fail. The mod framework also unblocks `spec-llgr-4-readvertisement.md` which needs `mods["add:attr:community"]`, `mods["set:attr:local-preference"]`, and `mods["withdraw:nlri:*"]`.

### Scope

**In scope:**

| Area | Description |
|------|-------------|
| Mod handler registry | Register/lookup `ModHandlerFunc` by mod key in `registry.go`. A mod handler is a post-accept transformation: it runs ONLY after all egress filters have accepted the route for a given peer. It is not a filter — it cannot reject. It takes the payload + mod value and returns a modified payload. |
| `applyMods` in forward path | After the egress filter chain accepts a route for a peer, iterate accumulated mods and call each registered handler. Handlers compose sequentially: each receives the output of the previous. Produces a modified WireUpdate for this peer's forwarding. |
| OTC egress stamping | Role plugin writes `mods.Set("set:attr:otc", localASN)` when route has no OTC and destination is Customer/Peer/RS-Client |
| OTC mod handler | Registered by role plugin; called by reactor after acceptance; calls `insertOTCInPayload` to produce modified payload |
| Unicast-only scope | OTC ingress and egress filters skip non-unicast families (IPv4/IPv6 unicast only per RFC) |
| LocalASN capture | Role plugin captures reactor's LocalAS during OnConfigure for use in egress stamping |

**Out of scope:**

| Area | Reason |
|------|--------|
| LLGR mod handlers | Separate spec (`spec-llgr-4-readvertisement.md`) |
| `resolveExport` optimization | Performance, not correctness; separate concern |
| Private AS removal filter | Future filter, not RFC 9234 |
| AS Confederation OTC rules | No confederation support yet |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - forward path, egress filter chain, WireUpdate
  -> Constraint: forward path is per-peer in reactor_api_forward.go, wire selection happens after filter chain
  -> Constraint: WireUpdate created from payload via `wireu.NewWireUpdate(payload, ctxID)`
- [ ] `docs/architecture/meta/README.md` - route metadata key registry, meta conventions
  -> Constraint: mod keys follow `<action>:<target>:<name>` convention

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9234.md` - OTC egress stamping rules
  -> Constraint: "If a route is to be advertised to a Customer, a Peer, or an RS-Client [...] and the OTC Attribute is not present, then [...] an OTC Attribute MUST be added with a value equal to the AS number of the local AS." (Section 5)
  -> Constraint: "Once the OTC Attribute has been set, it MUST be preserved unchanged." (Section 5)
  -> Constraint: OTC procedures "MUST NOT be applied to other address families by default" (AFI 1/2, SAFI 1 only)

**Key insights:**
- `ModAccumulator` already exists with `Set/Get/Range/Len/Reset`, lazily allocated, per-peer fresh instance
- `EgressFilterFunc` already receives `*ModAccumulator` but no consumer applies mods yet
- TODO at `reactor_api_forward.go:275` marks the exact insertion point for `applyMods`
- `ModHandlerFunc` is a post-accept transformation: `func(payload []byte, val any) []byte`. Runs only after all egress filters accepted the route for a peer. Cannot reject — only transform. Handlers compose sequentially per peer.
- `insertOTCInPayload` already exists in `otc.go:180-217` and works correctly
- Role plugin captures per-peer config via package-level maps set during OnConfigure; localASN can use the same pattern
- Family can be determined from payload: no MP_REACH/MP_UNREACH attributes means IPv4 unicast; otherwise parse AFI/SAFI from MP_REACH
- `spec-llgr-4` depends on this framework for community addition, local-pref modification, and withdrawal conversion

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/registry/registry.go` - ModAccumulator (lines 54-94), EgressFilterFunc (line 52), Registration struct (lines 98-152), IngressFilters/EgressFilters (lines 506-534). No mod handler registry exists yet.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - ForwardUpdate method. Lines 256-274: egress filter chain with fresh `ModAccumulator` per peer. Line 275: `TODO(spec-llgr-4): applyMods(mods)`. Lines 277-288: wire selection for EBGP/IBGP. Line 291: fwdItem built with meta.
- [ ] `internal/component/bgp/plugins/role/otc.go` - `OTCEgressFilter` (lines 278-312): ignores `mods` parameter (underscore). `checkOTCEgress` (lines 152-157): checks if route has OTC and dest is Provider/Peer/RS (suppression only). `insertOTCInPayload` (lines 180-217): creates new payload with OTC appended. `isUnicastFamily` (lines 92-96): defined but unused. `extractAttrsFromPayload` (lines 161-176): parses UPDATE payload for attributes.
- [ ] `internal/component/bgp/plugins/role/role.go` - Package-level filter state: `filterPeerConfigs`, `filterRemoteRoles`, `filterNameToIP` maps. `setFilterState` called during OnConfigure. No `localASN` captured currently.
- [ ] `internal/component/bgp/plugins/role/register.go` - Registers `IngressFilter: OTCIngressFilter`, `EgressFilter: OTCEgressFilter`. Registers OTC attribute name.
- [ ] `internal/component/bgp/reactor/reactor.go` - `Config.LocalAS` (line 106): reactor's local ASN, available at configure time.

**Behavior to preserve:**
- OTC ingress stamping and leak rejection (already working)
- OTC egress suppression: routes with OTC not sent to Provider/Peer/RS (already working)
- Export role filtering (already working)
- `meta["src-role"]` set at ingress, read at egress (already working)
- Lazy ModAccumulator allocation (zero cost when no mods written)
- Fail-closed panic recovery wrappers (safeIngressFilter, safeEgressFilter)
- Forward path zero-copy when no mods and ContextID matches

**Behavior to change:**
- `OTCEgressFilter` stops ignoring `mods` parameter; writes `"set:attr:otc"` mod when stamping needed
- New `applyMods` call in forward path applies accumulated mods, producing modified WireUpdate
- OTC ingress and egress filters check family; skip non-unicast
- Role plugin captures `localASN` during OnConfigure for egress stamping value

## Data Flow (MANDATORY)

### Entry Point -- Egress Stamping
- ForwardUpdate dispatches a route to matching destination peers
- Per-peer egress filter chain runs with fresh ModAccumulator

### Transformation Path

Per-peer forward loop in `reactor_api_forward.go`:

1. **Filter phase** (lines 258-274): egress filter chain runs. Each filter can suppress (return false) or accept (return true) and optionally write mods. If any filter suppresses, skip this peer entirely. OTC egress filter checks: route has no OTC AND dest is Customer/Peer/RS-Client -> writes `mods.Set("set:attr:otc", localASN)`, returns true.
2. **All filters accepted** — route is going to this peer.
3. **Apply mods** (line 275, currently TODO): if `mods.Len() > 0`, iterate mods via `mods.Range()`. For each mod key, look up registered `ModHandlerFunc`. Call handler with current payload + mod value. Handler returns modified payload. Chain: next handler receives the previous handler's output.
4. **Wrap result**: if payload was modified, create new `WireUpdate` via `wireu.NewWireUpdate(modifiedPayload, ctxID)`.
5. **Wire selection** (lines 277-288): select EBGP/IBGP wire version. Uses modified WireUpdate if mods were applied.
6. **Build fwdItem** (line 291): forward item built with final wire + meta.

### Entry Point -- Family Scope
- OTC ingress filter receives payload with path attributes
- OTC egress filter receives payload with path attributes
- Both need to determine if family is IPv4/IPv6 unicast before applying OTC rules

### Transformation Path -- Family Check
1. Parse UPDATE payload for MP_REACH_NLRI (type 14) or MP_UNREACH_NLRI (type 15) attributes
2. If neither present: family is IPv4 unicast (RFC 4271 implicit)
3. If MP_REACH present: read AFI (2 bytes) + SAFI (1 byte) from attribute value
4. Check AFI in {1, 2} and SAFI == 1; if not, skip OTC processing

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin registry -> Reactor | Mod handlers registered at init, retrieved at reactor startup | [ ] |
| Egress filter -> ModAccumulator | Filter writes mod key+value; reactor reads after chain | [ ] |
| ModAccumulator -> Handler | Reactor iterates mods, calls registered handler per key | [ ] |
| Handler -> WireUpdate | Handler returns modified payload; reactor wraps in WireUpdate | [ ] |

### Integration Points
- `registry.go` - New `ModHandlerFunc` type + registration/lookup functions
- `reactor_api_forward.go:275` - Replace TODO with `applyMods` call
- `role/otc.go:OTCEgressFilter` - Write `"set:attr:otc"` mod instead of ignoring `mods`
- `role/otc.go:OTCIngressFilter` - Add family check before OTC processing
- `role/role.go` - Capture `localASN` during `setFilterState`
- `role/register.go` - Register `"set:attr:otc"` mod handler

### Architectural Verification
- [ ] No bypassed layers (mods flow through registry-based handlers, reactor never imports plugins)
- [ ] No unintended coupling (reactor calls handlers by key, doesn't know about OTC/role)
- [ ] No duplicated functionality (reuses `insertOTCInPayload` already in `otc.go`)
- [ ] Zero-copy preserved when no mods (common case: `mods.Len() == 0` skips applyMods entirely)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `import provider` + `export default` + route without OTC forwarded to Customer peer | -> | OTC egress stamping via applyMods | `test/plugin/role-otc-egress-stamp.ci` |
| Config with `import provider` + multicast family route | -> | OTC not applied to non-unicast family | `test/plugin/role-otc-unicast-scope.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Route without OTC forwarded to Customer peer (local role = provider) | OTC attribute added with value = local ASN |
| AC-2 | Route without OTC forwarded to Peer (local role = provider) | OTC attribute added with value = local ASN |
| AC-3 | Route without OTC forwarded to RS-Client (local role = RS) | OTC attribute added with value = local ASN |
| AC-4 | Route without OTC forwarded to Provider (local role = customer) | No OTC stamped (Provider is not Customer/Peer/RS-Client from sender's perspective) |
| AC-5 | Route with OTC already present forwarded to Customer peer | OTC preserved unchanged (MUST NOT modify existing OTC) |
| AC-6 | Non-unicast family route (e.g., ipv4/multicast) from Provider peer | OTC ingress rules not applied |
| AC-7 | Non-unicast family route forwarded to Customer peer | OTC egress rules not applied (no stamping, no suppression) |
| AC-8 | No role configured on either peer | No OTC processing, no mods written (backward compatible) |
| AC-9 | Mod handler registered for `"set:attr:otc"` | Handler callable from reactor without importing role plugin |
| AC-10 | `mods.Len() == 0` after filter chain | `applyMods` is a no-op, zero allocation |
| AC-11 | `insertOTCInPayload` returns nil (overflow) | Mod handler returns original payload unchanged, route still forwarded |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestModHandlerRegistration` | `registry/registry_test.go` | Register and retrieve mod handler by key | |
| `TestModHandlerNotFound` | `registry/registry_test.go` | Unknown mod key returns nil handler | |
| `TestApplyModsNoMods` | `reactor/reactor_api_forward_test.go` | `mods.Len() == 0` is a no-op, returns original payload | |
| `TestApplyModsOTCStamp` | `reactor/reactor_api_forward_test.go` | `"set:attr:otc"` mod produces payload with OTC appended | |
| `TestApplyModsUnknownKey` | `reactor/reactor_api_forward_test.go` | Unknown mod key logged, original payload returned | |
| `TestOTCEgressStampMod` | `role/otc_test.go` | Egress filter writes `"set:attr:otc"` mod when route has no OTC and dest is Customer | |
| `TestOTCEgressNoStampProvider` | `role/otc_test.go` | Egress filter does NOT write mod when dest is Provider | |
| `TestOTCEgressPreserveExisting` | `role/otc_test.go` | Egress filter does NOT write mod when route already has OTC | |
| `TestOTCEgressStampLocalASN` | `role/otc_test.go` | Mod value is local ASN, not source or dest peer ASN | |
| `TestOTCIngressUnicastOnly` | `role/otc_test.go` | Ingress filter skips OTC processing for non-unicast family | |
| `TestOTCEgressUnicastOnly` | `role/otc_test.go` | Egress filter skips OTC processing for non-unicast family | |
| `TestExtractFamilyFromPayload` | `role/otc_test.go` | Correctly identifies IPv4 unicast (no MP_REACH), IPv6 unicast (MP_REACH AFI=2 SAFI=1), multicast (SAFI=2) | |
| `TestOTCStampOverflow` | `role/otc_test.go` | `insertOTCInPayload` returns nil on overflow; mod handler returns original payload | |
| `TestLocalASNCaptured` | `role/role_test.go` | `setFilterState` stores localASN, egress filter uses it for stamping | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Local ASN for OTC stamp | 1-4294967295 | 4294967295 | 0 (should not stamp with ASN 0) | N/A (uint32) |
| MP_REACH AFI | 1-2 for unicast scope | 2 (IPv6) | 0 (reserved) | 3+ (not unicast scope) |
| MP_REACH SAFI | 1 for unicast scope | 1 | 0 (reserved) | 2+ (not unicast) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `role-otc-egress-stamp` | `test/plugin/role-otc-egress-stamp.ci` | Provider sends route without OTC; Customer peer receives route WITH OTC = local ASN | |
| `role-otc-unicast-scope` | `test/plugin/role-otc-unicast-scope.ci` | Multicast route forwarded without OTC processing regardless of role config | |

### Future (if deferring any tests)
- Property-based testing for mod handler round-trip (mod applied -> verify attribute present in wire)
- Benchmark for applyMods overhead per peer (should be negligible when mods.Len() == 0)

## Files to Modify
- `internal/component/plugin/registry/registry.go` - Add `ModHandlerFunc` type, mod handler registration and lookup functions
- `internal/component/bgp/reactor/reactor_api_forward.go` - Replace TODO at line 275 with `applyMods` implementation
- `internal/component/bgp/reactor/reactor.go` - Load mod handlers at startup (alongside ingress/egress filters)
- `internal/component/bgp/plugins/role/otc.go` - Add family extraction, unicast scope check in ingress and egress filters, write `"set:attr:otc"` mod in egress
- `internal/component/bgp/plugins/role/role.go` - Capture `localASN` in `setFilterState`
- `internal/component/bgp/plugins/role/register.go` - Register `"set:attr:otc"` mod handler

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new behavior | Yes | `test/plugin/role-otc-egress-stamp.ci`, `test/plugin/role-otc-unicast-scope.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | OTC stamping is automatic per RFC, not user-configured |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | Same role plugin, extended behavior |
| 6 | Has a user guide page? | Yes | `docs/guide/bgp-role.md` - note OTC is now stamped on egress per RFC |
| 7 | Wire format changed? | No | OTC attribute format unchanged |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc9234.md` - mark egress stamping as implemented |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | Already listed in comparison |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` - document applyMods in forward path, `docs/architecture/meta/README.md` - document mod handler pattern |

## Files to Create
- `test/plugin/role-otc-egress-stamp.ci` - OTC egress stamping functional test
- `test/plugin/role-otc-unicast-scope.ci` - unicast-only scope functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Phases 1-5 below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Mod handler registry** -- Add `ModHandlerFunc` type and registration/lookup in `registry.go`
   - Tests: `TestModHandlerRegistration`, `TestModHandlerNotFound`
   - Files: `registry/registry.go`, `registry/registry_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: applyMods in forward path** -- Replace TODO at `reactor_api_forward.go:275` with mod application logic
   - Tests: `TestApplyModsNoMods`, `TestApplyModsUnknownKey`
   - Files: `reactor/reactor_api_forward.go`, `reactor/reactor.go`
   - Verify: no-op when `mods.Len() == 0`; unknown keys logged and skipped

3. **Phase: Family extraction + unicast scope** -- Add `extractFamilyFromPayload` in `otc.go`, gate ingress and egress filters on unicast family
   - Tests: `TestExtractFamilyFromPayload`, `TestOTCIngressUnicastOnly`, `TestOTCEgressUnicastOnly`
   - Files: `role/otc.go`, `role/otc_test.go`
   - Verify: non-unicast payloads bypass OTC processing

4. **Phase: OTC egress stamping via mods** -- Capture localASN in role plugin, write `"set:attr:otc"` mod in egress filter, register OTC mod handler
   - Tests: `TestLocalASNCaptured`, `TestOTCEgressStampMod`, `TestOTCEgressNoStampProvider`, `TestOTCEgressPreserveExisting`, `TestOTCEgressStampLocalASN`, `TestOTCStampOverflow`, `TestApplyModsOTCStamp`
   - Files: `role/role.go`, `role/otc.go`, `role/register.go`, `reactor/reactor_api_forward.go`
   - Verify: egress filter writes mod; applyMods calls handler; handler produces payload with OTC

5. **Phase: Functional tests** -- .ci tests for end-to-end egress stamping and unicast scope
   - Tests: `role-otc-egress-stamp`, `role-otc-unicast-scope`
   - Files: `test/plugin/role-otc-egress-stamp.ci`, `test/plugin/role-otc-unicast-scope.ci`
   - Verify: all .ci tests pass

6. **RFC refs** -- Add `// RFC 9234 Section 5` comments on egress stamping code
7. **Full verification** -- `make ze-verify`
8. **Complete spec** -- audit, learned summary, delete spec

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | OTC stamped with local ASN (not source or dest peer ASN) per RFC 9234 Section 5 |
| RFC compliance | Existing OTC preserved unchanged; stamping only when OTC absent |
| Scope | OTC processing gated on unicast family (AFI 1/2, SAFI 1) |
| Backward compat | No role config = no OTC processing, no mods written |
| Performance | `applyMods` is no-op when `mods.Len() == 0` (common case); no allocation |
| Coupling | Reactor calls mod handlers by key, never imports role plugin |
| Data flow | Egress filter writes mod -> applyMods applies -> modified WireUpdate used for forwarding |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `ModHandlerFunc` type in registry | grep "ModHandlerFunc" in registry.go |
| Mod handler registration/lookup | grep "RegisterModHandler\|ModHandler(" in registry.go |
| `applyMods` replaces TODO | grep "applyMods\|TODO.*spec-llgr" in reactor_api_forward.go (TODO gone) |
| OTC egress stamping writes mod | grep `"set:attr:otc"` in otc.go |
| OTC mod handler registered | grep `"set:attr:otc"` in register.go |
| localASN captured | grep "localASN\|filterLocalASN" in role.go |
| Unicast scope check | grep "isUnicast\|extractFamily" in otc.go has production callers |
| Functional tests exist | ls test/plugin/role-otc-egress-stamp.ci test/plugin/role-otc-unicast-scope.ci |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Mod handler validates value type (uint32) before casting; malformed mod skipped |
| Overflow | `insertOTCInPayload` returns nil on uint16 overflow; handler returns original payload |
| Panic safety | Mod handler called inside existing safeEgressFilter scope or separate recovery |
| No injection | Mod keys are string constants, not user input |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
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

Add `// RFC 9234 Section 5: "<quoted requirement>"` above enforcing code.
MUST document: OTC egress stamp rule (add OTC = local ASN to Customer/Peer/RS-Client), OTC preservation (existing OTC unchanged), unicast-only scope.

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

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
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
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
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-apply-mods.md`
- [ ] Summary included in commit

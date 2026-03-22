# Spec: role-otc

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/5 |
| Updated | 2026-03-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc9234.md` - OTC attribute and ingress/egress rules
4. `internal/component/bgp/plugins/role/` - existing role plugin
5. `internal/component/bgp/reactor/reactor_api_forward.go` - forward path

## Task

Implement RFC 9234 Phase 2: OTC (Only to Customer) attribute processing with `import`/`export` filtering.

Phase 1 (already done, learned/239): Role capability negotiation and OPEN validation.
Phase 2 (this spec): OTC attribute parsing, ingress stamping/rejection (`import`), egress filtering (`export`).

### `import` / `export` Config Keywords

Two keywords under `role {}` control role declaration and OTC filtering, using Junos-style terminology:

- `import` -- declares the local role AND enables RFC 9234 ingress rules (non-overridable). Replaces the Phase 1 `name` keyword.
- `export` -- outbound: which destination peer roles may receive routes learned from this peer

```
bgp {
  peer 10.0.0.1 {
    role {
      import customer              # role = customer, OTC ingress rules enabled
      export default               # RFC 9234 egress rules (suppress + OTC stamping)
      strict true
    }
  }
  peer 10.0.0.2 {
    role {
      import peer
      export default unknown       # RFC rules + send to untagged peers
    }
  }
  peer 10.0.0.3 {
    role {
      import provider
      export customer peer         # explicit override: only to customers and peers
    }
  }
}
```

**`import` token:**

| Token | Meaning |
|-------|---------|
| `provider` | Local role = provider, OTC ingress rules for provider |
| `customer` | Local role = customer, OTC ingress rules for customer |
| `peer` | Local role = peer, OTC ingress rules for peer |
| `rs` | Local role = RS, OTC ingress rules for RS |
| `rs-client` | Local role = RS-client, OTC ingress rules for RS-client |

`import` declares the local role (sent in OPEN capability code 9) AND enables RFC 9234 Section 5 ingress rules. One token only (the role name). The ingress rules are non-overridable per RFC. Without `import`, no role is negotiated and no OTC processing occurs.

**`export` tokens:**

| Token | Meaning |
|-------|---------|
| `default` | Expands to RFC 9234 Section 5 egress rules for the declared role |
| `unknown` | Also send to peers with no role configured (no OTC processing) |
| `provider` | Destination peers with role = provider |
| `customer` | Destination peers with role = customer |
| `peer` | Destination peers with role = peer |
| `rs` | Destination peers with role = rs |
| `rs-client` | Destination peers with role = rs-client |

`default` is additive -- `export default unknown` means "RFC rules plus also send to peers without role." Without `export`, no OTC egress filtering is applied (routes sent to all peers as before).

### RFC 9234 Ingress Rules (applied by `import`)

Same rules for all local roles (determined by remote peer's role):

| Condition | Action |
|-----------|--------|
| Route has OTC, received from Customer or RS-Client | Reject (route leak) |
| Route has OTC from Peer, OTC != peer's ASN | Reject (route leak) |
| Route has no OTC from Provider, Peer, or RS | Stamp OTC = remote ASN |

### RFC 9234 `export default` Expansion

| Local Role (from `import`) | `default` Allows Sending To |
|----------------------------|----------------------------|
| Provider | customer, rs-client |
| Customer | provider, rs, peer |
| RS | rs-client |
| RS-Client | rs, provider |
| Peer | customer, rs-client |

### Filter Chain Architecture

A generic **peer filter chain** in the reactor handles both ingress and egress filtering. Plugins inject filter functions during startup; the reactor calls all registered filters without importing any plugin.

| Aspect | Design |
|--------|--------|
| Registration | Reactor exposes a filter registration function; plugins register during startup |
| Directions | Ingress (per source peer, before bus) and egress (per destination peer, during forward) |
| Filter signature | Takes source peer info + destination peer info + route attributes, returns accept/reject |
| Chaining | All registered filters called in order; any reject skips the route (ingress) or peer (egress) |
| Coupling | Reactor knows about filter functions, not about role/OTC/private-AS |
| Hot path | O(1) per filter per peer (role lookup, not per-attribute parsing) |

The role plugin injects both an ingress filter (OTC stamp/reject) and an egress filter (OTC suppress/stamp) into the same chain.

**Current scope:** Role OTC ingress + egress filters (this spec).
**Future filters (TODO):** Private AS removal, plugin-defined external filters.

### OTC Attribute (type 35)

4-byte ASN, Optional Transitive (flags 0xC0). Stamped on routes to indicate "only send downstream."

**Ingress rules (RFC 9234 Section 5):**

| Condition | Action |
|-----------|--------|
| Route has OTC, received from Customer or RS-Client | Route leak -- mark ineligible |
| Route has OTC, received from Peer, OTC != peer's ASN | Route leak -- mark ineligible |
| Route has no OTC, received from Provider, Peer, or RS | Add OTC = remote ASN |

**Egress rules (RFC 9234 Section 5):**

| Condition | Action |
|-----------|--------|
| Route has OTC, destination is Provider, Peer, or RS | Suppress (do not send) |
| Route has no OTC, destination is Customer, Peer, or RS-Client | Add OTC = local ASN |

`import <role>` declares the local role and enables RFC-mandated ingress rules (non-overridable). `export` controls egress suppression via the reactor's peer filter chain, and can be extended with explicit role tokens. Without `import`, no role is negotiated and no OTC processing occurs. Without `export`, no egress filtering is applied.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - forward path, WireUpdate, PackContext
  -> Constraint: forward path is in reactor_api_forward.go, per-peer wire selection
- [ ] `docs/architecture/plugin/rib-storage-design.md` - how RIB stores attributes
  -> Constraint: attributes stored as pool handles, not raw wire
- [ ] `docs/architecture/wire/attributes.md` - attribute type registry
  -> Constraint: new attributes need type code registration

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9234.md` - OTC attribute wire format, ingress/egress rules, MUST requirements
  -> Constraint: OTC is type 35, Optional Transitive (0xC0), 4 bytes (ASN)
  -> Constraint: Scope is AFI 1/2 (IPv4/IPv6), SAFI 1 (Unicast) only
  -> Constraint: "The operator MUST NOT have the ability to modify the procedures" (Section 5) -- `import <role>` always enables non-overridable ingress rules
  -> Constraint: Malformed OTC (length != 4) uses treat-as-withdraw

**Key insights:**
- `import <role>` replaces Phase 1 `name` keyword -- declares role AND enables non-overridable ingress rules
- `export` controls egress filtering via reactor peer filter chain -- can be extended with explicit role tokens
- `import`/`export` use Junos-style terminology on top of RFC 9234
- Without `import`, no role is negotiated and no OTC processing occurs
- Egress filtering uses a generic peer filter chain (role is the first filter; private AS removal, plugin-defined filters come later)
- OTC scope: IPv4/IPv6 unicast only (SAFI 1)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/role/role.go` - 5-stage startup, OnConfigure, OnValidateOpen
- [ ] `internal/component/bgp/plugins/role/config.go` - extracts per-peer role configs (currently name + strict; will become import + strict + export)
- [ ] `internal/component/bgp/plugins/role/validate.go` - OPEN pair validation
- [ ] `internal/component/bgp/plugins/role/schema/` - YANG schema for role config
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - ForwardUpdate, per-peer wire selection
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` - handleReceived stores routes, handleState for peer lifecycle
- [ ] `internal/component/bgp/capability/capability.go` - capability code 9 = Role
- [ ] `internal/component/bgp/message/` - attribute type constants

**Behavior to preserve:**
- Role capability negotiation (code 9) and OPEN validation (Phase 1)
- Strict mode enforcement
- Per-peer and group-level role config with per-peer override
- Forward path zero-copy when ContextID matches
- Existing attribute parsing for all other attributes

**Behavior to change:**
- Rename `name` to `import` in role YANG schema and config parsing (declares role + enables ingress)
- Add `export` keyword to role YANG schema and config parsing
- Add OTC attribute (type 35) parsing in attribute type registry
- Add OTC ingress processing (stamp/reject) in role plugin event handler
- Add generic peer filter chain in reactor forward path
- Register OTC egress filter from role plugin into peer filter chain

## Data Flow (MANDATORY)

### Entry Point -- Ingress
- UPDATE received from peer, parsed into WireUpdate
- Reactor runs ingress peer filter chain (filters injected by plugins at startup)

### Transformation Path -- Ingress
1. Reactor receives UPDATE from peer, runs ingress filter chain for source peer
2. OTC ingress filter (injected by role plugin): check if source peer has a negotiated role
3. Parse OTC attribute (type 35) from raw attributes if present
4. Apply ingress rules: reject leak or stamp OTC = remote ASN
5. If rejected: filter returns reject, route never reaches the bus
6. If accepted/stamped: route dispatched to bus (plugins receive it, RIB stores it)

### Entry Point -- Egress
- RIB plugin calls ForwardUpdate(selector, updateID)
- Forward path iterates destination peers

### Transformation Path -- Egress
1. ForwardUpdate matches destination peers by selector
2. For each destination peer: run peer filter chain
3. OTC filter (registered by role plugin): check source peer's `export` config against destination peer's role
4. If any filter rejects: skip peer
5. If all filters accept: apply OTC egress stamping if needed, forward

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire -> Reactor | UPDATE parsed into WireUpdate | [ ] |
| Reactor ingress filter chain | Injected filters called per source peer before bus | [ ] |
| Reactor egress filter chain | Injected filters called per destination peer during forward | [ ] |

### Replay Selector Change (prerequisite)

The adj-rib-in plugin's `buildReplayCommands` currently sends each route to a specific `targetPeer` via `updateRoute(targetPeer, cmd)`. This bypasses any filter chain because it names a single destination. Change to `updateRoute("!"+sourcePeer, cmd)` so the route goes through the standard dispatcher, which resolves all matching peers and applies the filter chain per-destination.

This also solves the "new peer gets routes" problem: on peer-up, replay all source peers' routes to `!<source>`. The new peer is in the `!<source>` set. Existing peers receive idempotent re-announcements. At startup the RIB is empty so replay sends nothing.

The bgp-rs plugin already builds explicit comma-separated peer lists in `selectForwardTargets`, which also goes through the dispatcher. The filter chain applies to both paths.

### Integration Points
- `role/config.go` - rename `name` to `import`, add `export` keyword parsing
- `role/schema/` - YANG schema: rename `name` to `import`, add `export` leaf-list
- Attribute type registry - register type 35 (OTC)
- Reactor - add generic peer filter chain (registration + call sites for ingress and egress)
- Role plugin startup - inject OTC ingress filter and OTC egress filter into peer filter chain
- `adj_rib_in/rib.go` - change `buildReplayCommands` to use `!<sourcePeer>` selector instead of `targetPeer`
- `adj_rib_in/rib.go` - trigger replay on peer-up in `handleState`

### Architectural Verification
- [ ] No bypassed layers (ingress and egress both via reactor filter chain, filters injected by plugin)
- [ ] No unintended coupling (role plugin communicates via events/commands, not direct import)
- [ ] No duplicated functionality (extends existing role plugin)
- [ ] Zero-copy preserved where applicable (egress filtering skips peers, doesn't modify wire)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `import default` + peer sends route with OTC from Customer | -> | OTC ingress rejection (leak) | `test/plugin/role-otc-ingress-reject.ci` |
| Config with `export default` + peer sends route | -> | OTC egress filter by destination role | `test/plugin/role-otc-egress-filter.ci` |
| Config with `export default unknown` | -> | Routes sent to untagged peers | `test/plugin/role-otc-export-unknown.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `import provider` + route from Provider with no OTC | OTC stamped with provider's ASN |
| AC-2 | `import customer` + route from Customer with OTC present | Route rejected (leak detection) |
| AC-3 | `import peer` + route from Peer with OTC != peer's ASN | Route rejected (leak detection) |
| AC-4 | `export default` + route with OTC, destination is Provider | Route suppressed (not sent) |
| AC-5 | `export default` + route with OTC, destination is Customer | Route sent (allowed by default for all source roles) |
| AC-6 | `export default unknown`, destination has no role | Route sent to untagged peer |
| AC-7 | `export customer peer` explicit override | Routes only sent to customer and peer roles |
| AC-8 | No `import`/`export` keywords configured | No role negotiated, no OTC filtering (backward compatible) |
| AC-9 | Malformed OTC (length != 4) | Treat-as-withdraw per RFC |
| AC-10 | OTC scope limited to IPv4/IPv6 unicast | OTC not applied to other families |
| AC-11 | `import customer` replaces Phase 1 `name customer` | Role capability sent in OPEN, OPEN validation works |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestImportDeclaresRole` | `role/config_test.go` | `import customer` sets role = customer | |
| `TestImportReplacesName` | `role/config_test.go` | `import` accepted, `name` rejected | |
| `TestParseExportConfig` | `role/config_test.go` | Parsing export tokens from config | |
| `TestExportDefault_Provider` | `role/config_test.go` | export default expands correctly for provider role | |
| `TestExportDefault_Customer` | `role/config_test.go` | export default expands correctly for customer role | |
| `TestExportDefaultUnknown` | `role/config_test.go` | export default + unknown combined | |
| `TestExportExplicitRoles` | `role/config_test.go` | explicit export role list without default | |
| `TestOTCIngressStamp` | `role/otc_test.go` | OTC stamped on routes from Provider/Peer/RS | |
| `TestOTCIngressRejectLeak` | `role/otc_test.go` | Route with OTC from Customer rejected | |
| `TestOTCIngressRejectPeerWrongASN` | `role/otc_test.go` | Route with OTC from Peer, wrong ASN rejected | |
| `TestOTCEgressFilter` | `role/otc_test.go` | Route with OTC suppressed to Provider/Peer/RS | |
| `TestOTCEgressStamp` | `role/otc_test.go` | OTC added on egress to Customer/Peer/RS-Client | |
| `TestOTCMalformed` | `role/otc_test.go` | Length != 4 triggers treat-as-withdraw | |
| `TestOTCScopeUnicastOnly` | `role/otc_test.go` | OTC not processed for non-unicast families | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| OTC ASN | 1-4294967295 | 4294967295 | 0 (reserved) | N/A (uint32) |
| OTC length | 4 | 4 | 3 (malformed) | 5 (malformed) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `role-otc-ingress-reject` | `test/plugin/role-otc-ingress-reject.ci` | `import default`, customer sends route with OTC, route rejected as leak | |
| `role-otc-egress-filter` | `test/plugin/role-otc-egress-filter.ci` | `export default`, provider sends route, customer peer receives it, provider peer does not | |
| `role-otc-export-unknown` | `test/plugin/role-otc-export-unknown.ci` | `export default unknown`, untagged peer receives routes | |

### Future (if deferring any tests)
- AS Confederation OTC handling (Section 5 confederation rules) - no confederation support yet
- Property-based testing for OTC wire encoding round-trip

## Files to Modify
- `internal/component/bgp/plugins/role/config.go` - rename `name` to `import`, add `export` parsing
- `internal/component/bgp/plugins/role/role.go` - subscribe to UPDATE events, ingress OTC processing, register egress filter
- `internal/component/bgp/plugins/role/schema/` - YANG schema: rename `name` to `import`, add `export` leaf-list
- `internal/component/bgp/plugins/role/validate.go` - potentially extend for OTC validation
- `internal/component/bgp/reactor/reactor_api_forward.go` - add generic peer filter chain + call site in ForwardUpdate
- Attribute type registry (location TBD during research) - register type 35
- Existing Phase 1 tests - update `name` to `import` in config fixtures
- `internal/component/bgp/plugins/adj_rib_in/rib.go` - change replay selector to `!<source>`, add auto-replay on peer-up

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | Yes | `internal/component/bgp/plugins/role/schema/` |
| CLI commands/flags | No | N/A |
| Editor autocomplete | Yes | YANG-driven (automatic if YANG updated) |
| Functional test for new behavior | Yes | `test/plugin/role-otc-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - add OTC route leak prevention |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` - add `import`/`export` keywords under role |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` - role plugin now handles OTC |
| 6 | Has a user guide page? | Yes | `docs/guide/bgp-role.md` - add OTC section and import/export config |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/attributes.md` - OTC attribute type 35 |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc9234.md` - mark OTC sections as implemented |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - route leak prevention via OTC |
| 12 | Internal architecture changed? | No | |

## Files to Create
- `internal/component/bgp/plugins/role/otc.go` - OTC attribute processing (ingress/egress)
- `internal/component/bgp/plugins/role/otc_test.go` - OTC unit tests
- `test/plugin/role-otc-ingress-reject.ci` - ingress rejection functional test
- `test/plugin/role-otc-egress-filter.ci` - egress filtering functional test
- `test/plugin/role-otc-export-unknown.ci` - export unknown functional test

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

1. **Phase: Rename `name` to `import` + add `export`** -- YANG schema, config parsing, update Phase 1 tests
   - Tests: `TestImportDeclaresRole`, `TestImportReplacesName`, `TestParseExportConfig`, `TestExportDefault_*`, `TestExportExplicitRoles`
   - Files: `role/config.go`, `role/schema/`, `role/config_test.go`, existing Phase 1 test configs
   - Verify: tests fail -> implement -> tests pass; existing role tests still pass with `import`

2. **Phase: OTC attribute type** -- register type 35 in attribute registry
   - Tests: `TestOTCMalformed`
   - Files: attribute type registry
   - Verify: OTC parseable from wire bytes

3. **Phase: Peer filter chain infrastructure** -- generic filter chain in reactor (ingress + egress)
   - Tests: filter registration and call-site tests
   - Files: `reactor/reactor_api_forward.go` (or new `reactor_filter.go`)
   - Verify: filters can be registered and are called at ingress (peer, before bus) and egress points

4. **Phase: OTC ingress + egress filters** -- role plugin injects both filters into chain
   - Tests: `TestOTCIngressStamp`, `TestOTCIngressRejectLeak`, `TestOTCIngressRejectPeerWrongASN`, `TestOTCScopeUnicastOnly`, `TestOTCEgressFilter`, `TestOTCEgressStamp`
   - Files: `role/otc.go`, `role/role.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Functional tests** -- .ci tests for end-to-end behavior
   - Tests: `role-otc-ingress-reject`, `role-otc-egress-filter`, `role-otc-export-unknown`
   - Files: `test/plugin/role-otc-*.ci`
   - Verify: all .ci tests pass

6. **RFC refs** -- add `// RFC 9234 Section 5` comments
7. **Full verification** -- `make ze-verify`
8. **Complete spec** -- audit, learned summary, delete spec

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Ingress rules match RFC 9234 Section 5 exactly |
| RFC compliance | `import <role>` always enables non-overridable ingress; `export` relaxes egress |
| Scope | OTC applied to IPv4/IPv6 unicast only |
| Backward compat | No `import`/`export` keywords = no OTC filtering (existing configs unaffected) |
| Data flow | Ingress in role plugin, egress in forward path |
| Performance | Egress check is O(1) per peer (role lookup), no per-attribute parsing in hot path |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `import`/`export` config parses, `name` removed | grep "import" and "export" in YANG, grep confirms no "name" |
| OTC attribute type 35 registered | grep "35" or "OTC" in attribute registry |
| Ingress stamping/rejection works | `TestOTCIngressStamp` + `TestOTCIngressRejectLeak` pass |
| Egress filtering works | `TestOTCEgressFilter` passes |
| Functional tests exist | ls test/plugin/role-otc-*.ci |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | OTC length must be exactly 4; malformed = treat-as-withdraw |
| Route leak | `import <role>` always enables ingress rules, no way to disable (RFC Section 5 MUST) |
| ASN validation | OTC ASN is uint32, no overflow possible |

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
MUST document: OTC ingress stamp rule, OTC ingress reject rule, OTC egress suppress rule, OTC egress stamp rule, treat-as-withdraw for malformed, unicast-only scope.

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
- [ ] Write learned summary to `plan/learned/NNN-role-otc.md`
- [ ] Summary included in commit

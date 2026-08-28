# Spec: bgp-evpn-route-type-match

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/bgp/plugins/filter_family/` - the closest existing filter (family-granularity match)
4. `internal/component/bgp/plugins/nlri/evpn/types.go` - EVPN route types
5. `ai/rules/plugins.md` - filters are self-contained plugins

## Task

Ze supports the EVPN address family and models all five EVPN route types, but
policy filtering can only match at address-family granularity: an operator can
filter "all l2vpn/evpn" but cannot match a specific EVPN route type (Type-2 MAC-IP
vs Type-5 IP-prefix, etc.). This prevents common EVPN policies such as "permit only
Type-5 prefixes from this peer" or "drop Type-3 multicast".

Add an EVPN route-type match filter: a new filter plugin that matches routes by
their EVPN route type (EAD/Type-1, MAC-IP/Type-2, IMET-multicast/Type-3,
Ethernet-Segment/Type-4, IP-prefix/Type-5), accepting both symbolic names and the
numeric 1-5, following the existing one-plugin-per-match-type pattern.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugins.md` - each filter is its own plugin; remove it and the feature vanishes.
  → Constraint: register a new `filter_evpn_route_type` plugin; do not add an EVPN case to a generic filter.
- [ ] `ai/patterns/config-option.md` - config leaf/validator pattern for the match value.
  → Constraint: use native YANG enumeration + numeric where possible.

**Key insights:**
- The EVPN NLRI already exposes route type; the filter reads it from the parsed NLRI, it does not re-parse the wire.
- `filter_family` is the structural template: it matches a property of the UPDATE and permits/denies accordingly.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/nlri/evpn/types.go` - `EVPNRouteType1..5` = EAD / MAC-IP / IMET(inclusive-multicast) / Ethernet-Segment / IP-prefix (types.go); each has a struct, parser, and `WriteTo`; `EVPNRouteType.String()` yields names like `ethernet-segment`.
- [ ] `internal/component/bgp/plugins/filter_family/match.go` - matches only at address-family granularity (extracts the family from the UPDATE body); no route-type awareness. This is the closest existing filter.
- [ ] `internal/component/bgp/plugins/` filter set - `filter_aspath`, `filter_aspath_length`, `filter_community`, `filter_community_match`, `filter_family`, `filter_irr`, `filter_modify`, `filter_prefix`, `filter_remove_private_as`; none reference EVPN route type (grep for `RouteType` in filters returns nothing).
- [ ] `internal/component/bgp/filterapi/filterapi.go` - the filter API (`IngressFilterFunc`, `PeerFilterInfo`) that plugins implement and register.

**Behavior to preserve:**
- Existing filters and `filter_family` behaviour unchanged.
- EVPN NLRI encode/decode unchanged; the filter only reads the parsed route type.
- Non-EVPN families are unaffected by the new filter (it is a no-op for them).

**Behavior to change:**
- Add the ability to permit/deny by EVPN route type in policy.

## Data Flow (MANDATORY)

### Entry Point
- Config: a new filter binding (e.g. under a peer/group filter chain) selecting `evpn-route-type <name|1-5>` with permit/deny, defined in the new plugin's YANG.
- Runtime: an UPDATE carrying EVPN NLRI passes through the ingress/egress filter chain.

### Transformation Path
1. Config parsed into the plugin's per-peer match set (allowed/denied route types).
2. Plugin registers an ingress (and/or egress) filter via the filter API.
3. For each route, the plugin inspects the parsed EVPN NLRI route type.
4. If the family is EVPN and the type matches the rule, permit/deny per config; non-EVPN routes pass through unaffected.
5. Result feeds the existing filter-chain decision.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ filter plugin | YANG match value → per-peer route-type set | [ ] |
| Filter API ↔ plugin | plugin registers `IngressFilterFunc` | [ ] |
| NLRI ↔ filter | filter reads parsed EVPN route type, not raw wire | [ ] |

### Integration Points
- `internal/component/bgp/filterapi/` - implement/register the filter (same as other filter_* plugins).
- `internal/component/bgp/plugins/nlri/evpn/types.go` - source of the route-type value.
- filter chain (ingress/egress) - the new plugin participates like any other filter.

### Architectural Verification
- [ ] No bypassed layers (filter uses the filter API, not a reactor hook)
- [ ] No unintended coupling (reads EVPN route type via the NLRI plugin's exported type)
- [ ] No duplicated functionality (new match type, reuses the filter framework)
- [ ] Registration over hardcoding — a self-contained `filter_evpn_route_type` plugin registers itself; no EVPN case added to a generic/central filter (`ai/rules/plugins.md`).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The parsed EVPN route type is reachable from the filter API context | filter_family reads family from the UPDATE | filter needs richer route context | trace what the filter API exposes about the NLRI during audit | unvalidated |
| A-2 | One-plugin-per-match is the right structure here | existing filter_* plugins | might belong in an existing filter | confirm with plugin-self-containment rule | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Filter API does not surface route-type without re-parsing | plugin has to decode NLRI itself | expose the parsed route type from the evpn plugin as a small accessor |
| R-2 | Numeric vs symbolic input inconsistency | operators confused by 2 vs macip | accept both, canonicalize to one form in the model |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| filter `evpn-route-type prefix deny` on a peer | → | `filter_evpn_route_type` denies Type-5 EVPN routes | `test/plugin/bgp-evpn-route-type-match.ci` |
| Type-2 route with a Type-5 deny rule | → | route permitted (type mismatch) | `test/plugin/bgp-evpn-route-type-match.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | rule `deny prefix` (Type-5), incoming Type-5 EVPN route | route denied |
| AC-2 | rule `deny prefix`, incoming Type-2 (MAC-IP) route | route permitted |
| AC-3 | rule uses numeric `5` | equivalent to `prefix` |
| AC-4 | rule `permit ethernet-segment` (Type-4) | Type-4 permitted, others per default |
| AC-5 | non-EVPN route (e.g. ipv4/unicast) | filter is a no-op, route unaffected |
| AC-6 | invalid route-type value | config verify rejects (enum/range) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | denies Type-5 EVPN prefixes from a peer | config → filter plugin → ingress deny on Type-5 | `test/plugin/bgp-evpn-route-type-match.ci` |
| 2 | permits only Ethernet-Segment routes | filter permits Type-4, denies others | `test/plugin/bgp-evpn-route-type-match.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEVPNRouteTypeMatchDeny` | `internal/component/bgp/plugins/filter_evpn_route_type/filter_test.go` | Type-5 deny, Type-2 permit | |
| `TestEVPNRouteTypeNumericAlias` | `internal/component/bgp/plugins/filter_evpn_route_type/filter_test.go` | numeric 1-5 equal symbolic | |
| `TestEVPNRouteTypeNonEVPNNoop` | `internal/component/bgp/plugins/filter_evpn_route_type/filter_test.go` | non-EVPN route unaffected | |
| `TestEVPNRouteTypeConfigParse` | `internal/component/bgp/plugins/filter_evpn_route_type/config_test.go` | YANG value → match set | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| evpn-route-type (numeric) | 1-5 | 5 | 0 | 6 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-evpn-route-type-match` | `test/plugin/bgp-evpn-route-type-match.ci` | policy denies/permits by EVPN route type | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-evpn-route-type-peer` | `test/interop/scenarios/` | GoBGP/BIRD EVPN | Ze filters real EVPN route types received from a peer | |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/bgp/plugins/nlri/evpn/` - expose the parsed route type to the filter (small accessor, if not already reachable)

### BGP Family Checklist (if new SAFI / capability / attribute)
This spec adds no new SAFI/capability/attribute; it is a policy filter over the
existing EVPN family. The BGP Family Checklist does not apply.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `internal/component/bgp/plugins/filter_evpn_route_type/yang/`; `ai/rules/config.md` |
| YANG validation constraints | [ ] yes | `enumeration` of names + numeric 1-5 |
| CLI grammar | [ ] yes | `ai/rules/cli.md` |
| Functional test for new behaviour | [ ] yes | `test/plugin/bgp-evpn-route-type-match.ci` |
| Registered plugin/inventory changed | [ ] yes | plugin snapshot/all_test; `docs/plugin-overview.md` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 5 | Plugin added/changed? | [ ] yes | `docs/guide/plugins.md` |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` |

## Files to Create
- `internal/component/bgp/plugins/filter_evpn_route_type/` - new filter plugin (filter.go, config.go, register.go, yang/)
- `internal/component/bgp/plugins/filter_evpn_route_type/filter_test.go` - unit tests
- `test/plugin/bgp-evpn-route-type-match.ci` - functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — scaffold `filter_evpn_route_type` plugin (register.go, YANG, no-op filter) and register it; failing `test/plugin/bgp-evpn-route-type-match.ci`.
2. **Phase: Match logic** — read EVPN route type from the parsed NLRI; permit/deny per config; no-op for non-EVPN.
   - Tests: `TestEVPNRouteTypeMatchDeny`, `TestEVPNRouteTypeNonEVPNNoop`
3. **Phase: Value parsing** — symbolic + numeric 1-5, validation.
   - Tests: `TestEVPNRouteTypeNumericAlias`, `TestEVPNRouteTypeConfigParse`
4. **Functional + interop tests**
5. **Full verification** → `./le verify current mode full`
6. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Feature completeness | all five route types matchable; numeric + symbolic |
| Correctness | non-EVPN routes pass untouched |
| Naming | YANG kebab-case; route-type names match the NLRI plugin's `String()` |
| Registration over hardcoding | new plugin self-registers; no central EVPN case |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| filter plugin | `go test ./internal/component/bgp/plugins/filter_evpn_route_type/...` |
| plugin registered | plugin snapshot test lists it |
| interop | `NN-evpn-route-type-peer` scenario passes |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | route-type value bounded by enum/range |
| Resource | filter is O(1) per route; no per-route allocation |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights
<!-- LIVE -->

## Implementation Summary
### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `./le verify current mode full` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features

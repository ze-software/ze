# Spec: Interop and Config Parse Coverage

| Field | Value |
|-------|-------|
| Status | supported-subset evidence added |
| Depends | - |
| Phase | testing |
| Updated | 2026-05-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `test/interop/interop.py` - interop framework (Scenario, FRR, BIRD, GoBGP, Ze classes)
4. `test/interop/run.py` - interop runner
5. `internal/test/runner/parsing.go` - parse test framework (.ci format)

## Task

Add interop and config parse coverage scenarios for protocol features and config patterns that Ze does not yet cover. This spec covers the missing scenarios, organized by priority.

## Required Reading

### Architecture Docs
- [ ] `test/interop/interop.py` - Scenario lifecycle, FRR/BIRD/GoBGP/Ze helper classes
- [ ] `test/interop/run.py` - Runner, image builds, scenario discovery
- [ ] `internal/test/runner/parsing.go` - .ci parse test format and runner
- [ ] `docs/architecture/testing/ci-format.md` - .ci file format spec

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4456.md` - Route Reflection (new scenario)
- [ ] `rfc/short/rfc8950.md` - Extended Next-Hop (new scenario)
- [ ] `rfc/short/rfc4724.md` - Graceful Restart timer mechanics (deepen existing)
- [ ] `rfc/short/rfc9494.md` - Long-Lived GR (new scenario)
- [ ] `rfc/short/rfc5765.md` - Confederation (new parse test)
- [ ] `rfc/short/rfc6811.md` - RPKI Origin Validation (new interop)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `test/interop/scenarios/` - 37 existing scenarios (01-37)
- [ ] `test/parse/*.ci` - ~160 parse tests
- [ ] `test/exabgp-compat/etc/` - ~40 ExaBGP migration configs
- [ ] `etc/ze/bgp/` - ~90 config fixtures

**Behavior to preserve:**
- Existing interop framework: Python Scenario class, docker-based, PID-suffixed containers
- Existing parse test format: .ci files with stdin=, cmd=, expect=, reject= directives
- Existing scenario numbering (01-37 allocated)
- Flat lab topology with default source prefix `172.30.0.0/24`; runner renders to an available `/24` for concurrent runs

**Behavior to change:**
- None -- this spec adds new scenarios only

## Data Flow (MANDATORY)

### Entry Point
- Interop: `python3 test/interop/run.py` discovers scenarios, builds images, runs check.py
- Parse: `ze-test bgp parse` discovers .ci files, runs `ze config validate`

### Transformation Path
1. Scenario directory detected (has check.py or .ci file)
2. Containers started / config piped to ze
3. Assertions run against daemon state (vtysh/birdc/gobgp/ze show)
4. Pass/fail reported

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze <-> FRR | TCP BGP session on rendered lab subnet | [x] |
| Ze <-> Config | `ze config validate` on stdin | [ ] |

### Integration Points
- `test/interop/interop.py` Scenario class - all new interop scenarios use this
- `test/parse/*.ci` - all new config parse coverage uses this format

### Architectural Verification
- [ ] No bypassed layers (new scenarios use existing framework)
- [ ] No unintended coupling (scenarios are independent directories)
- [ ] No duplicated functionality (new scenarios, not rewrites)
- [ ] Zero-copy preserved where applicable (N/A -- test infrastructure)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `python3 test/interop/run.py 38-route-reflection-frr` | -> | `test/interop/scenarios/38-route-reflection-frr/check.py` | scenario runs and passes |
| `ze-test bgp parse` | -> | `test/parse/coverage-ixp-peering.ci` | parse test runs and passes |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Route Reflection interop scenario | RR reflects routes between clients, ORIGINATOR_ID and CLUSTER_LIST present |
| AC-2 | Policy engine interop scenario | Import sets local-pref, export denies/permits with MED, community-add works |
| AC-3 | Extended Next-Hop interop scenario | IPv4 prefix received with IPv6 next-hop via FRR |
| AC-4 | GR timer expiry interop scenario | Stale routes swept after restart timer expires without reconnection |
| AC-5 | LLGR interop scenario | GR-to-LLGR promotion, routes preserved during LLGR phase |
| AC-6 | RPKI interop scenario | RTR session established, valid/invalid/not-found affect route selection |
| AC-7 | BMP interop scenario | BMP Initiation + PeerUp + RouteMonitoring messages received by collector |
| AC-8 | Max-prefix Cease scenario | NOTIFICATION sent when prefix limit exceeded, session recovers |
| AC-9 | GTSM interop scenario | TTL security negotiated alongside MD5, session established |
| AC-10 | IXP config parse coverage | Dual-stack, 4+ peer-groups, bogon as-path-list + prefix-list + route-map chain validates |
| AC-11 | EVPN L3VPN config parse coverage | Multi-VRF + per-VRF BGP + VXLAN VNIs validates |
| AC-12 | Confederation config parse coverage | Confed ID + sub-AS members validates |
| AC-13 | Large-scale config parse coverage | 20+ neighbors, multiple route-maps, communities validates |
| AC-14 | RPKI config parse coverage | RTR cache + route-map match rpki valid/invalid/notfound validates |
| AC-15 | EGP/IGP redistribution config parse coverage | BGP + OSPF redistribution with route-maps validates |

## TDD Test Plan

### Unit Tests
N/A -- this spec adds integration/functional tests only.

### Boundary Tests
N/A -- no numeric inputs.

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `coverage-ixp-peering` | `test/parse/coverage-ixp-peering.ci` | IXP config with peer-groups, bogon filtering, route-maps validates | |
| `coverage-evpn-l3vpn` | `test/parse/coverage-evpn-l3vpn.ci` | Multi-VRF EVPN PE config validates | |
| `coverage-confederation` | `test/parse/coverage-confederation.ci` | Confederation config validates | |
| `coverage-large-scale` | `test/parse/coverage-large-scale.ci` | 20+ neighbor config validates | |
| `coverage-rpki` | `test/parse/coverage-rpki.ci` | RPKI + route-map config validates | |
| `coverage-redistribution` | `test/parse/coverage-redistribution.ci` | BGP + OSPF redistribution validates | |

### Interop Tests (MANDATORY for protocol features)

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `38-route-reflection-frr` | `test/interop/scenarios/` | FRR | RR reflects to clients, ORIGINATOR_ID/CLUSTER_LIST correct | |
| `39-policy-import-export-frr` | `test/interop/scenarios/` | FRR | Import local-pref set, export deny/MED/community-add | |
| `40-extended-nexthop-frr` | `test/interop/scenarios/` | FRR | IPv4 prefix with IPv6 NH (RFC 8950) | |
| `41-gr-timer-expiry-frr` | `test/interop/scenarios/` | FRR | Stale route sweep after restart timer expires | |
| `42-llgr-frr` | `test/interop/scenarios/` | FRR | Long-Lived GR two-phase timer (RFC 9494) | |
| `43-rpki-frr` | `test/interop/scenarios/` | FRR | RTR session + origin validation states | |
| `44-bmp-frr` | `test/interop/scenarios/` | FRR | BMP message types to collector | |
| `45-max-prefix-cease-frr` | `test/interop/scenarios/` | FRR | NOTIFICATION on limit, session recovery | |
| `46-gtsm-frr` | `test/interop/scenarios/` | FRR | TTL security (RFC 5082) | |

### Future (if deferring any tests)
- Dynamic peer add/remove at runtime -- requires runtime peer management API
- Add-Path multi-sender forwarding -- requires 3+ peer topology, defer to separate spec
- Route Server mode with multiple observers -- requires 4+ containers, defer to separate spec
- BGP + IPsec combined config coverage -- defer until IPsec config schema stabilizes (spec-ipsec-0-umbrella)
- DMVPN config coverage -- Ze does not support NHRP/DMVPN
- BGP-LS config coverage -- defer until BGP-LS config schema lands

## Files to Modify

- `test/interop/interop.py` - may need RTR/BMP helper classes for scenarios 43-44

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | N/A -- tests only |
| CLI commands/flags | [ ] | N/A |
| Functional test for new RPC/API | [x] | `test/parse/coverage-*.ci` |
| Pipe completeness | [ ] | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 10 | Test infrastructure changed? | [x] | `docs/functional-tests.md` -- add parse coverage section |

## Files to Create

### Interop scenarios (9 new)
- `test/interop/scenarios/38-route-reflection-frr/ze.conf`
- `test/interop/scenarios/38-route-reflection-frr/frr.conf`
- `test/interop/scenarios/38-route-reflection-frr/check.py`
- `test/interop/scenarios/39-policy-import-export-frr/ze.conf`
- `test/interop/scenarios/39-policy-import-export-frr/frr.conf`
- `test/interop/scenarios/39-policy-import-export-frr/check.py`
- `test/interop/scenarios/40-extended-nexthop-frr/ze.conf`
- `test/interop/scenarios/40-extended-nexthop-frr/frr.conf`
- `test/interop/scenarios/40-extended-nexthop-frr/check.py`
- `test/interop/scenarios/41-gr-timer-expiry-frr/ze.conf`
- `test/interop/scenarios/41-gr-timer-expiry-frr/frr.conf`
- `test/interop/scenarios/41-gr-timer-expiry-frr/announce-gr.py`
- `test/interop/scenarios/41-gr-timer-expiry-frr/check.py`
- `test/interop/scenarios/42-llgr-frr/ze.conf`
- `test/interop/scenarios/42-llgr-frr/frr.conf`
- `test/interop/scenarios/42-llgr-frr/check.py`
- `test/interop/scenarios/43-rpki-frr/ze.conf`
- `test/interop/scenarios/43-rpki-frr/frr.conf`
- `test/interop/scenarios/43-rpki-frr/check.py`
- `test/interop/scenarios/44-bmp-frr/ze.conf`
- `test/interop/scenarios/44-bmp-frr/frr.conf`
- `test/interop/scenarios/44-bmp-frr/bmp-collector.py`
- `test/interop/scenarios/44-bmp-frr/check.py`
- `test/interop/scenarios/45-max-prefix-cease-frr/ze.conf`
- `test/interop/scenarios/45-max-prefix-cease-frr/frr.conf`
- `test/interop/scenarios/45-max-prefix-cease-frr/announce-many.py`
- `test/interop/scenarios/45-max-prefix-cease-frr/check.py`
- `test/interop/scenarios/46-gtsm-frr/ze.conf`
- `test/interop/scenarios/46-gtsm-frr/frr.conf`
- `test/interop/scenarios/46-gtsm-frr/check.py`

### Config parse coverage (6 new)
- `test/parse/coverage-ixp-peering.ci`
- `test/parse/coverage-evpn-l3vpn.ci`
- `test/parse/coverage-confederation.ci`
- `test/parse/coverage-large-scale.ci`
- `test/parse/coverage-rpki.ci`
- `test/parse/coverage-redistribution.ci`

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Create -- check Ze's config schema supports each feature |
| 3. Wiring phase | First scenario (38) + first parse coverage test -- verify framework works |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `python3 test/interop/run.py` + `ze-test bgp parse` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Config parse coverage** -- Write 6 .ci files with realistic multi-feature configs
   - Tests: coverage-ixp-peering.ci through coverage-redistribution.ci
   - Files: `test/parse/coverage-*.ci`
   - Verify: `ze-test bgp parse` -- all parse coverage files pass `ze config validate`
   - Note: configs must use Ze's YANG syntax

2. **Phase: Route Reflection interop (38)** -- RR with 2 clients
   - Tests: `38-route-reflection-frr/check.py`
   - Files: ze.conf (RR config), frr.conf (2 client configs)
   - Verify: routes reflected, ORIGINATOR_ID + CLUSTER_LIST present in FRR

3. **Phase: Policy interop (39)** -- Import/export attribute manipulation
   - Tests: `39-policy-import-export-frr/check.py`
   - Files: ze.conf with policy config, frr.conf
   - Verify: local-pref set on import, community added, MED set on export

4. **Phase: Extended Next-Hop interop (40)** -- RFC 8950
   - Tests: `40-extended-nexthop-frr/check.py`
   - Files: ze.conf with ext-NH capability, frr.conf
   - Verify: IPv4 prefix received with IPv6 next-hop

5. **Phase: GR timer expiry (41)** -- Deepen existing GR coverage
   - Tests: `41-gr-timer-expiry-frr/check.py`
   - Files: ze.conf with GR + restart-time, frr.conf, announce-gr.py
   - Verify: routes preserved during GR, swept after timer expiry

6. **Phase: LLGR interop (42)** -- RFC 9494
   - Tests: `42-llgr-frr/check.py`
   - Files: ze.conf with LLGR config, frr.conf
   - Verify: GR-to-LLGR promotion, routes survive LLGR phase

7. **Phase: RPKI interop (43)** -- Origin validation
   - Tests: `43-rpki-frr/check.py`
   - Files: ze.conf with RPKI cache, frr.conf, mock RTR server or stayrtr container
   - Verify: valid/invalid/not-found states affect route selection

8. **Phase: BMP interop (44)** -- BMP collector
   - Tests: `44-bmp-frr/check.py`
   - Files: ze.conf with BMP config, frr.conf, bmp-collector.py (Python TCP listener)
   - Verify: Initiation, PeerUp, RouteMonitoring messages received

9. **Phase: Max-prefix Cease (45)** -- NOTIFICATION + recovery
   - Tests: `45-max-prefix-cease-frr/check.py`
   - Files: ze.conf with prefix maximum, frr.conf, announce-many.py
   - Verify: session tears down on limit, recovers after timeout

10. **Phase: GTSM interop (46)** -- TTL Security
    - Tests: `46-gtsm-frr/check.py`
    - Files: ze.conf with GTSM config, frr.conf with ttl-security
    - Verify: session established with TTL security, rejected without

11. **Functional tests** -- Verify all scenarios run via `python3 test/interop/run.py`
12. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has a passing scenario or .ci test |
| Correctness | Configs use valid Ze YANG syntax |
| Naming | Scenario directories follow NN-feature-peer pattern |
| Data flow | check.py uses existing FRR/BIRD/GoBGP/Ze helper classes |
| Framework | No custom docker orchestration -- all via interop.py Scenario class |
| Independence | Each scenario works standalone (`run.py <name>`) |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| 9 new interop scenarios | `ls test/interop/scenarios/3[89]-* test/interop/scenarios/4[0-6]-*` |
| 6 new config parse coverage files | `ls test/parse/coverage-*.ci` |
| All scenarios have check.py | `find test/interop/scenarios/3[89]-* test/interop/scenarios/4[0-6]-* -name check.py` |
| All .ci tests pass | `ze-test bgp parse` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Mock RTR server (scenario 43) must not expose real network access |
| Container isolation | BMP collector (scenario 44) listens only on test network |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Ze config syntax not supported | Feature gap -- document, skip parse coverage case or mark expected-fail |
| Interop session won't establish | Check Ze config, FRR config, IP addressing |
| Scenario timeout | Increase SESSION_TIMEOUT or fix Ze daemon startup |
| 3 fix attempts fail | STOP. Report. Ask user. |

## Gap Coverage Matrix

### Tier 1: Protocol interop + config validation

| Gap | Ze deliverable |
|-----|----------------|
| RPKI interop | scenario 43 + coverage-rpki.ci |
| Route Reflection | scenario 38 |
| Import/export policy | scenario 39 |
| Extended Next-Hop | scenario 40 |
| Confederation | coverage-confederation.ci |

### Tier 2: Protocol behavior

| Gap | Ze deliverable |
|-----|----------------|
| GR timer expiry sweep | scenario 41 |
| Long-Lived GR | scenario 42 |
| BMP collector | scenario 44 |
| Max-prefix Cease | scenario 45 |
| GTSM/TTL Security | scenario 46 |

### Tier 3: Config corpus

| Gap | Ze deliverable |
|-----|----------------|
| IXP peering holistic config | coverage-ixp-peering.ci |
| Large-scale config | coverage-large-scale.ci |
| EVPN L3VPN + VRF | coverage-evpn-l3vpn.ci |
| EGP/IGP redistribution | coverage-redistribution.ci |

### Deferred (with justification)

| Gap | Why deferred |
|-----|-------------|
| Dynamic peer add/remove | Ze lacks runtime peer management API |
| Add-Path multi-sender | Requires 3+ peer topology, separate spec |
| Route Server multi-observer | Requires 4+ containers, separate spec |
| BGP + IPsec combo config | IPsec schema not stable (spec-ipsec-0-umbrella) |
| DMVPN config | Ze does not support NHRP |
| VRF + PPPoE combo | PPPoE underlay not yet in Ze |
| BGP-LS config | BGP-LS config schema not landed |
| Private AS removal modes | Ze has 36/37 scenarios; full mode coverage is incremental |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Use existing interop.py framework, not Containerlab | Adopt Containerlab | Ze's framework is more self-contained, richer assertion library, no extra dependency |
| Config parse coverage as .ci files, not standalone configs | Directory with assert files | .ci format already exists, integrates with ze-test bgp parse, richer assertions |
| FRR as primary peer for all new scenarios | Mix FRR/BIRD/GoBGP | FRR is the most feature-complete peer, reduces implementation scope |
| Start with Tier 1 gaps (both projects test) | Start with easiest | Tier 1 has highest confidence of being real gaps |

## Known Limitations

- Config parse coverage only tests parse + validate, not runtime behavior
- RPKI scenario (43) needs a mock RTR server or stayrtr container in the test infrastructure
- BMP scenario (44) needs a Python BMP message parser as collector
- LLGR (42) requires FRR 10.x which supports LLGR; older FRR versions do not

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| `plugin { external ... use ... }` process plugin could be a reliable BMP collector | Ze's BMP sender can connect before a Ze process plugin is listening, then back off | Scenario 44 first run received only Initiation and timed out | Added framework sidecar collector support in `test/interop/interop.py` |
| BIRD as the source RR client would reflect to FRR in scenario 38 | FRR source to BIRD destination is the reliable covered interop path | Scenario 38 first run had BIRD exporting 1 route but FRR receiving 0 | Swapped source/destination while still proving RR attributes on a real peer |
| Ze runtime RIB CLI was available inside the interop container | The interop image only exposes static CLI commands; runtime `show rib` commands are not reachable that way | Scenario 39 first run and manual `docker exec ze show rib status` | Verified policy through BIRD receiving iBGP attributes instead |
| New interop scenarios could run in parallel with fixed subnets | Docker rejects concurrent scenario networks when every scenario requests `172.30.0.0/24` | Parallel runs of scenarios 39/44/45 made 44 and 45 fail at network creation | Added dynamic subnet allocation and rendered scenario copies |
| Scenario 45 should first wait for a stable Established state | FRR can advertise both routes and receive Cease before a 2s poll observes Established | Fresh scenario 45 run timed out in Active with Ze already enforcing the limit | Changed check to wait for enforcement log and dropped session, then verify recovery |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Scenario 39 using process-originated Ze routes for export policy | Export filters did not apply to locally originated process routes in the expected way | FRR source to Ze to BIRD forwarding path with import and export filters |
| Scenario 44 using a Ze process plugin as collector | Collector start raced with BMP sender reconnect backoff | Pre-started BMP sidecar container on the run's `.6` address |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| RPKI retry-polls adj-rib-in instead of storing decisions | Once (scenario 43), but architectural | Store-and-reconcile between parallel subscribers | Learned 797: replace retry loop with buffered validation decisions |

## Design Insights

- The supported subset is test-only and uses existing BGP/filter/BMP behavior. Unsupported protocol or config surfaces were not approximated.
- Ze-owned plugins in the new scenarios use `use bgp-*`, which runs them in-process through the internal plugin runner. External Python is only used for route injection or sidecar collection.
- Runtime checks should prefer peer-daemon evidence for interop. The `ze-interop` image is not a management CLI client for live RIB commands.
- Interop scenarios keep readable `172.30.0.x` source fixtures, then render to `tmp/interop-rendered/` with an allocated `/24` before mounting files and importing `check.py`.

## Implementation Summary

### What Was Implemented
- Parse coverage files:
  - `test/parse/coverage-ixp-peering.ci`
  - `test/parse/coverage-large-scale.ci`
  - `test/parse/coverage-rpki.ci`
  - `test/parse/coverage-redistribution.ci`
- Interop scenarios:
  - `test/interop/scenarios/38-route-reflection-frr/`
  - `test/interop/scenarios/39-policy-import-export-frr/`
  - `test/interop/scenarios/43-rpki-frr/`
  - `test/interop/scenarios/44-bmp-frr/`
  - `test/interop/scenarios/45-max-prefix-cease-frr/`
- Test framework support:
  - `test/interop/interop.py` starts a `bmp-collector.py` sidecar container on the run's `.6` address before Ze.
  - `test/interop/interop.py` starts a `ze-test rpki` sidecar container on the run's `.7` address before Ze.
  - `test/interop/interop.py` allocates a non-overlapping Docker subnet and renders scenario files before container startup.

### Bugs Found/Fixed
- Test bug: parse coverage files initially used missing semicolons in inline one-line config. Fixed.
- Harness gap: BMP sender collector as Ze process plugin was racy. Fixed with sidecar collector support.
- Test race: scenario 45 could miss the brief pre-Cease Established state. Fixed by waiting for the prefix-limit event and session drop before recovery.
- Harness gap: parallel Docker interop runs overlapped on `172.30.0.0/24`. Fixed with subnet retry and rendered scenario copies.
- Product bug: structured adj-rib-in IPv4 UPDATE handling parsed NLRI without preserving legacy NEXT_HOP, so received direct-bridge routes were dropped as unreplayable. Fixed and covered by `TestHandleReceivedStructuredIPv4NextHop`.
- Test race: scenario 44 forced an FRR session clear after the initial BMP stream, and re-establishment was nondeterministic. Fixed by waiting for Initiation, PeerUp, and RouteMonitoring from the initial session.
- Architecture fix: RPKI validation used retry-polling (20x50ms) to handle the race with adj-rib-in. Replaced with store-and-reconcile: adj-rib-in buffers early RPKI decisions, applies them when the route arrives. RPKI dispatches once with no retry. See learned 797.

### Documentation Updates
- `docs/functional-tests.md` documents parse coverage, BGP interop runner, and BMP/RPKI sidecar behavior.

### Deviations from Plan
- Implemented the user-approved supported subset only.
- Not implemented: scenarios 40, 41, 42, 46 and parse coverage for EVPN L3VPN, confederation, route-map RPKI matching, and OSPF redistribution.
- Reason: audited Ze currently lacks the required config/runtime surfaces for those exact ACs.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Add interop gap coverage for supported behavior | Done | `test/interop/scenarios/38-*`, `39-*`, `43-*`, `44-*`, `45-*` | Verified with targeted runs |
| Add config parse coverage for supported behavior | Done | `test/parse/coverage-*.ci` | Verified with `bin/ze-test bgp parse ...` |
| Avoid unsupported approximation | Done | This audit | Unsupported ACs remain explicitly open |
| Update test docs | Done | `docs/functional-tests.md` | Documents parse coverage and BMP/RPKI sidecars |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `ZE_INTEROP_SUBNET_INDEX=1 NO_BUILD=1 python3 test/interop/run.py 38-route-reflection-frr` | Fresh run passed on rendered `172.30.1.0/24`; FRR source, BIRD client receives reflected route with ORIGINATOR_ID and CLUSTER_LIST |
| AC-2 | Done | `NO_BUILD=1 python3 test/interop/run.py 39-policy-import-export-frr` | Fresh run passed after renderer change; FRR source to Ze to BIRD proves import local-pref, export MED/community, and export deny |
| AC-3 | Not done | Blocked | Needs exact RFC 8950 interop path; no scenario added in supported subset |
| AC-4 | Not done | Blocked | Existing GR capability coverage remains; timer-expiry interop not added in supported subset |
| AC-5 | Not done | Blocked | Existing LLGR plugin coverage remains; FRR LLGR interop not added in supported subset |
| AC-6 | Done | `NO_BUILD=1 python3 test/interop/run.py 43-rpki-frr` | RTR sidecar established, valid and not-found routes are accepted with validation states, invalid route is rejected |
| AC-7 | Done | `NO_BUILD=1 python3 test/interop/run.py 44-bmp-frr` | Fresh serial run passed; Ze BMP sender to sidecar collector receives Initiation, PeerUp, RouteMonitoring |
| AC-8 | Done | `NO_BUILD=1 python3 test/interop/run.py 45-max-prefix-cease-frr` | Fresh parallel run passed on `172.30.0.0/24`; prefix maximum triggers teardown and recovers after route removal |
| AC-9 | Not done | Unsupported | No GTSM outbound TTL/security support found |
| AC-10 | Done | `bin/ze-test bgp parse coverage-ixp-peering coverage-large-scale coverage-rpki coverage-redistribution` | Fresh parse subset run passed; supported IXP-style config validates |
| AC-11 | Not done | Unsupported | No VXLAN/VRF per-VRF BGP config surface found |
| AC-12 | Not done | Unsupported | No confederation schema/runtime support found |
| AC-13 | Done | `bin/ze-test bgp parse coverage-ixp-peering coverage-large-scale coverage-rpki coverage-redistribution` | Fresh parse subset run passed; 20+ neighbors and policy chains validate |
| AC-14 | Partial | `bin/ze-test bgp parse coverage-ixp-peering coverage-large-scale coverage-rpki coverage-redistribution` | Fresh parse subset run passed for supported global policy subset; route-map `match rpki` unsupported |
| AC-15 | Partial | `bin/ze-test bgp parse coverage-ixp-peering coverage-large-scale coverage-rpki coverage-redistribution` | Fresh parse subset run passed for BGP/kernel subset; OSPF redistribution unsupported |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `coverage-ixp-peering` | Done | `test/parse/coverage-ixp-peering.ci` | Passing |
| `coverage-evpn-l3vpn` | Not done | N/A | Unsupported config surface |
| `coverage-confederation` | Not done | N/A | Unsupported config surface |
| `coverage-large-scale` | Done | `test/parse/coverage-large-scale.ci` | Passing |
| `coverage-rpki` | Partial | `test/parse/coverage-rpki.ci` | Supported global RPKI subset passing |
| `coverage-redistribution` | Partial | `test/parse/coverage-redistribution.ci` | Supported BGP/kernel subset passing |
| `38-route-reflection-frr` | Done | `test/interop/scenarios/38-route-reflection-frr/` | Passing |
| `39-policy-import-export-frr` | Done | `test/interop/scenarios/39-policy-import-export-frr/` | Passing |
| `43-rpki-frr` | Done | `test/interop/scenarios/43-rpki-frr/` | Passing |
| `44-bmp-frr` | Done | `test/interop/scenarios/44-bmp-frr/` | Passing |
| `45-max-prefix-cease-frr` | Done | `test/interop/scenarios/45-max-prefix-cease-frr/` | Passing after race fix |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `test/interop/interop.py` | Changed | Adds BMP sidecar container support, dynamic subnet allocation, and rendered scenario copies |
| `test/interop/scenarios/38-route-reflection-frr/*` | Added | RR interop |
| `test/interop/scenarios/39-policy-import-export-frr/*` | Added | Policy interop |
| `test/interop/scenarios/43-rpki-frr/*` | Added | RPKI origin validation interop |
| `test/interop/scenarios/44-bmp-frr/*` | Added | BMP sender interop |
| `test/interop/scenarios/45-max-prefix-cease-frr/*` | Added | Max-prefix interop |
| `test/parse/coverage-ixp-peering.ci` | Added | Supported IXP parse coverage |
| `test/parse/coverage-large-scale.ci` | Added | Supported large-scale parse coverage |
| `test/parse/coverage-rpki.ci` | Added | Supported RPKI parse coverage subset |
| `test/parse/coverage-redistribution.ci` | Added | Supported redistribution parse coverage subset |
| `docs/functional-tests.md` | Changed | Documents parse coverage and interop sidecars |

### Audit Summary
- **Total items:** 15 ACs
- **Done:** 8 ACs
- **Partial:** 2 ACs
- **Skipped:** 5 ACs, all unsupported or blocked in the audited surface
- **Changed:** Supported subset only, by user choice

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Route Reflection tested | interop test | `ZE_INTEROP_SUBNET_INDEX=1 NO_BUILD=1 python3 test/interop/run.py 38-route-reflection-frr` passed |
| Policy engine tested | interop test | `NO_BUILD=1 python3 test/interop/run.py 39-policy-import-export-frr` passed after renderer change |
| Extended Next-Hop tested | unsupported | Not implemented in supported subset |
| GR timer mechanics tested | blocked | Not implemented in supported subset |
| LLGR tested | blocked | Not implemented in supported subset |
| RPKI interop tested | interop test | `NO_BUILD=1 python3 test/interop/run.py 43-rpki-frr` passed |
| BMP tested | interop test | `NO_BUILD=1 python3 test/interop/run.py 44-bmp-frr` passed |
| Max-prefix Cease tested | interop test | `NO_BUILD=1 python3 test/interop/run.py 45-max-prefix-cease-frr` passed |
| GTSM tested | unsupported | No GTSM support found |
| IXP config validates | functional test | `bin/ze-test bgp parse coverage-ixp-peering coverage-large-scale coverage-rpki coverage-redistribution` passed |
| EVPN L3VPN config validates | unsupported | No VXLAN/VRF per-VRF BGP config surface found |
| Confederation config validates | unsupported | No confederation schema/runtime support found |
| Large-scale config validates | functional test | `bin/ze-test bgp parse coverage-ixp-peering coverage-large-scale coverage-rpki coverage-redistribution` passed |
| RPKI config validates | functional test | `bin/ze-test bgp parse coverage-ixp-peering coverage-large-scale coverage-rpki coverage-redistribution` passed for supported global policy subset |
| Redistribution config validates | functional test | `bin/ze-test bgp parse coverage-ixp-peering coverage-large-scale coverage-rpki coverage-redistribution` passed for BGP/kernel subset |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

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
| `test/interop/scenarios/38-route-reflection-frr/*` | Yes | Glob returned `ze.conf`, `frr.conf`, `bird.conf`, `check.py` |
| `test/interop/scenarios/39-policy-import-export-frr/*` | Yes | Glob returned `ze.conf`, `frr.conf`, `bird.conf`, `check.py` |
| `test/interop/scenarios/43-rpki-frr/*` | Yes | Glob returned `ze.conf`, `frr.conf`, `rpki-server`, `rpki-check.py`, `check.py` |
| `test/interop/scenarios/44-bmp-frr/*` | Yes | Glob returned `ze.conf`, `frr.conf`, `bmp-collector.py`, `check.py` |
| `test/interop/scenarios/45-max-prefix-cease-frr/*` | Yes | Glob returned `ze.conf`, `frr.conf`, `check.py` |
| `test/parse/coverage-*.ci` | Yes | Glob returned the 4 supported-subset parse coverage files |
| `plan/learned/788-interop-gap-coverage.md` | Yes | Glob returned the learned summary |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | RR interop supported subset passes | `python3 test/interop/run.py 38-route-reflection-frr` passed with image build; `ZE_INTEROP_SUBNET_INDEX=1 NO_BUILD=1 python3 test/interop/run.py 38-route-reflection-frr` passed and waited for `172.30.1.2` |
| AC-2 | Policy interop supported subset passes | `NO_BUILD=1 python3 test/interop/run.py 39-policy-import-export-frr` passed after renderer change |
| AC-6 | RPKI interop supported subset passes | `python3 test/interop/run.py 43-rpki-frr` passed with image build; `NO_BUILD=1 python3 test/interop/run.py 43-rpki-frr` passed after diagnostics were removed |
| AC-7 | BMP interop supported subset passes | `NO_BUILD=1 python3 test/interop/run.py 44-bmp-frr` passed |
| AC-8 | Max-prefix interop supported subset passes | `NO_BUILD=1 python3 test/interop/run.py 45-max-prefix-cease-frr` passed |
| Harness | Parallel Docker subnets do not overlap | Parallel runs of scenarios 44 and 45 passed; scenario 44 used `172.30.1.2`, scenario 45 used `172.30.0.2` |
| AC-10 | IXP config supported subset validates | `bin/ze-test bgp parse coverage-ixp-peering coverage-large-scale coverage-rpki coverage-redistribution` passed |
| AC-13 | Large-scale config supported subset validates | `bin/ze-test bgp parse coverage-ixp-peering coverage-large-scale coverage-rpki coverage-redistribution` passed |
| AC-14 | RPKI supported subset validates | `bin/ze-test bgp parse coverage-ixp-peering coverage-large-scale coverage-rpki coverage-redistribution` passed; route-map `match rpki` unsupported |
| AC-15 | Redistribution supported subset validates | `bin/ze-test bgp parse coverage-ixp-peering coverage-large-scale coverage-rpki coverage-redistribution` passed; OSPF redistribution unsupported |
| AC-3/4/5/9/11/12 | Not claimed | Unsupported or blocked as documented above |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `python3 test/interop/run.py 38-route-reflection-frr` | N/A | Scenario discovered and `check.py` passed |
| `NO_BUILD=1 python3 test/interop/run.py 39-policy-import-export-frr` | N/A | Scenario discovered and `check.py` passed |
| `NO_BUILD=1 python3 test/interop/run.py 43-rpki-frr` | N/A | Scenario discovered, RPKI sidecar started, and `check.py` passed |
| `NO_BUILD=1 python3 test/interop/run.py 44-bmp-frr` | N/A | Scenario discovered, BMP sidecar started, and `check.py` passed |
| `NO_BUILD=1 python3 test/interop/run.py 45-max-prefix-cease-frr` | N/A | Scenario discovered and `check.py` passed |
| `bin/ze-test bgp parse coverage-ixp-peering coverage-large-scale coverage-rpki coverage-redistribution` | `test/parse/coverage-*.ci` | All 4 parse coverage files passed |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [x] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [x] Implementation Audit complete
- [x] Mistake Log escalation reviewed

### TDD
- [x] Tests written
- [ ] Tests FAIL (paste output)
- [x] Tests PASS (paste output)
- [x] Functional tests for end-to-end behavior
- [x] Interop tests for protocol features
- [x] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [ ] Critical Review passes
- [x] Implementation Summary filled
- [x] Implementation Audit filled
- [x] Write learned summary to `plan/learned/NNN-interop-gap-coverage.md`
- [ ] **Commit A:** code + tests + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-interop-gap-coverage.md`

# Spec: bgp-chaos-session (Phase 1 of 5)

**Master design:** `docs/plan/spec-bgp-chaos.md`
**Previous spec:** None (first phase)
**Next spec:** `spec-bgp-chaos-validation.md`

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/plan/spec-bgp-chaos.md` - master design (architecture, CLI, component diagram)
3. `.claude/rules/planning.md` - workflow rules
4. `internal/plugins/bgp/message/open.go` - OPEN message packing
5. `internal/plugins/bgp/message/update_build.go` - UpdateBuilder API
6. `internal/plugins/bgp/message/keepalive.go` - KEEPALIVE packing
7. `internal/plugins/bgp/capability/encoding.go` - EncodingCaps, Family types
8. `internal/test/peer/peer.go` - reference for BGP handshake logic

## Task

Build the foundation of `ze-bgp-chaos`: CLI skeleton, seed-based scenario generation, Ze config output, and a working single-peer BGP session that can announce ipv4/unicast routes.

**Scope of this phase:**
- CLI with all flags defined (but not all functional yet)
- Seed-based PeerProfile generation (deterministic)
- Ze config generation from scenario
- Single BGP peer: TCP connect, OPEN exchange, KEEPALIVE, send ipv4/unicast UPDATEs
- Basic stdout output showing session state and routes sent
- ipv4/unicast only (other families in Phase 4)

**NOT in scope (later phases):**
- Multi-peer orchestration (Phase 2)
- Route validation / convergence tracking (Phase 2)
- Chaos event injection (Phase 3)
- Non-unicast-v4 families (Phase 4)
- Live dashboard / JSON log / Prometheus (Phase 5)

## Required Reading

### Architecture Docs
- [ ] `docs/plan/spec-bgp-chaos.md` - master design document
  → Decision: Tool is black-box external peer; imports wire-encoding packages only
  → Decision: Seed determines all "random" decisions for reproducibility
  → Constraint: CLI interface defined in master spec (implement flags even if some are no-ops initially)
- [ ] `docs/architecture/core-design.md` - UPDATE structure, zero-copy forwarding
  → Constraint: IPv4/unicast uses inline NLRI, not MP_REACH_NLRI

### Source Code
- [ ] `internal/plugins/bgp/message/open.go` - Open struct, PackTo()
  → Constraint: Open.MyAS is uint16; for ASN4, set ASN4 field and MyAS=23456 (AS_TRANS)
- [ ] `internal/plugins/bgp/message/update_build.go` - NewUpdateBuilder, BuildUnicast, UnicastParams
  → Constraint: BuildUnicast takes UnicastParams{Prefix, NextHop, Origin, LocalPreference, MED, ...}
- [ ] `internal/plugins/bgp/message/keepalive.go` - KEEPALIVE packing
- [ ] `internal/plugins/bgp/message/notification.go` - NOTIFICATION building (for clean shutdown)
- [ ] `internal/plugins/bgp/capability/` - Capability types, Family, encoding
- [ ] `internal/test/peer/peer.go` - reference for OPEN exchange handshake pattern

**Key insights:**
- `message.PackTo(open, capBytes)` builds a complete OPEN message with header
- `message.NewUpdateBuilder(localAS, isIBGP, asn4, addPath)` creates builder for UPDATEs
- `builder.BuildUnicast(params)` returns `*Update` with wire-ready PathAttributes and NLRI
- `Update.Pack()` or `Update.WriteTo()` produces final wire bytes with BGP header
- Capabilities are encoded as optional parameters in OPEN

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] No existing `cmd/ze-bgp-chaos/` — this is entirely new code
- [ ] `internal/test/peer/peer.go` — existing test peer for reference

**Behavior to preserve:**
- No existing behavior to preserve (new tool)

**Behavior to change:**
- None

## Data Flow (MANDATORY)

### Entry Point
- User runs `ze-bgp-chaos --seed 42 --peers 4 --config-out chaos.conf`
- Seed + flags → ScenarioGenerator

### Transformation Path
1. **Seed parsing** → RNG initialized with seed
2. **Profile generation** → N PeerProfiles with ASN, families, route counts, connection mode
3. **Config generation** → Ze config file written to `--config-out`
4. **User starts Ze** → `ze chaos.conf` (manual step, outside tool)
5. **TCP connection** → Tool connects to Ze (or listens for Ze's connection)
6. **OPEN exchange** → Tool sends OPEN with capabilities, receives Ze's OPEN
7. **KEEPALIVE exchange** → Session established
8. **Route sending** → Build ipv4/unicast UPDATEs via UpdateBuilder, send on wire
9. **EOR** → Send End-of-RIB marker
10. **Steady state** → KEEPALIVE loop + optional churn (churn in Phase 2+)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Tool ↔ Ze Engine | TCP BGP wire bytes | [ ] |

### Integration Points
- `message.PackTo()` — OPEN message construction
- `message.NewUpdateBuilder()` → `BuildUnicast()` — UPDATE construction
- `capability.*` — Capability types for OPEN optional parameters
- `nlri.Family*` constants — Address family identifiers

### Architectural Verification
- [ ] No bypassed layers (pure TCP BGP peer)
- [ ] No unintended coupling (only wire-format package imports)
- [ ] No duplicated functionality (reuses Ze's encoding)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze-bgp-chaos --seed 42 --peers 4 --config-out /dev/stdout` | Prints valid Ze config with 4 peers, matching ASNs and families |
| AC-2 | Same seed twice | Identical PeerProfiles (ASNs, families, route counts, connection mode) |
| AC-3 | `--peers 1`, run against Ze with matching config | TCP connects, OPEN exchange succeeds, session established |
| AC-4 | Session established | Tool sends N ipv4/unicast UPDATE messages with unique prefixes |
| AC-5 | Session established | Tool sends End-of-RIB marker after initial routes |
| AC-6 | Session established | Tool maintains KEEPALIVE at negotiated interval |
| AC-7 | `--peers 50` | 50 valid peer profiles generated, config has 50 peer blocks |
| AC-8 | `--peers 0` or `--peers 51` | Error message, exit code 1 |
| AC-9 | `Ctrl-C` during run | Sends NOTIFICATION cease to Ze, clean shutdown |
| AC-10 | `--duration 5s` | Runs for 5s then exits cleanly |
| AC-11 | Stdout output during run | Shows seed, peer state (connecting/established), routes sent count |
| AC-12 | Generated config | Passes `ze validate <config>` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSeedDeterminism` | `scenario/generator_test.go` | Same seed + same flags → identical PeerProfile slice | |
| `TestSeedDifferent` | `scenario/generator_test.go` | Different seeds → different profiles | |
| `TestPeerCount` | `scenario/generator_test.go` | --peers N produces exactly N profiles | |
| `TestPeerCountBounds` | `scenario/generator_test.go` | 0 and 51 rejected, 1 and 50 accepted | |
| `TestIBGPRatio` | `scenario/generator_test.go` | ibgp-ratio 0.3 with 10 peers → ~3 iBGP | |
| `TestIBGPRatioExtreme` | `scenario/generator_test.go` | ratio 0.0 → all eBGP; ratio 1.0 → all iBGP | |
| `TestProfileASN` | `scenario/profile_test.go` | iBGP peers share local-as; eBGP peers have unique ASNs | |
| `TestProfileRouterID` | `scenario/profile_test.go` | Each peer has unique router ID | |
| `TestProfileConnectionMode` | `scenario/profile_test.go` | Mix of passive and active peers | |
| `TestRouteGenIPv4Unique` | `scenario/routes_test.go` | N routes from same seed → N distinct /24 prefixes | |
| `TestRouteGenIPv4Deterministic` | `scenario/routes_test.go` | Same seed+peer → same route set | |
| `TestRouteGenIPv4NoPeerOverlap` | `scenario/routes_test.go` | Routes from peer 0 and peer 1 don't overlap | |
| `TestRouteGenIPv4Count` | `scenario/routes_test.go` | --routes 100 → 100 routes; --heavy-routes 2000 → 2000 | |
| `TestConfigGenStructure` | `scenario/config_test.go` | Config has router-id, local-as, N peer blocks, RR process | |
| `TestConfigGenPeerBlock` | `scenario/config_test.go` | Peer block has correct ASN, families, passive flag | |
| `TestConfigGenDeterministic` | `scenario/config_test.go` | Same seed → identical config output | |
| `TestOpenMessageBuild` | `peer/session_test.go` | Builds valid OPEN with ASN4, multiprotocol caps | |
| `TestKeepaliveLoop` | `peer/session_test.go` | Sends KEEPALIVE at correct interval | |
| `TestUpdateBuild` | `peer/sender_test.go` | Builds valid ipv4/unicast UPDATE from generated route | |
| `TestEORBuild` | `peer/sender_test.go` | Builds valid End-of-RIB marker (empty UPDATE) | |
| `TestGracefulShutdown` | `peer/session_test.go` | Sends NOTIFICATION cease on context cancel | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| peers | 1-50 | 50 | 0 | 51 |
| routes | 1-100000 | 100000 | 0 | N/A (warn only) |
| heavy-routes | 1-100000 | 100000 | 0 | N/A |
| ibgp-ratio | 0.0-1.0 | 1.0 | N/A (clamp) | N/A (clamp) |
| port | 1024-65535 | 65535 | 1023 | 65536 |
| duration | 0 (unlimited) or >0 | any | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `chaos-session-basic` | `test/chaos/session-basic.ci` | 1 peer, seed 42, connects to Ze, sends 10 routes, receives them back (sink mode on Ze) | |

## Files to Create

- `cmd/ze-bgp-chaos/main.go` - CLI entry point, flag parsing, seed handling
- `cmd/ze-bgp-chaos/scenario/generator.go` - Seed → PeerProfile[] generation
- `cmd/ze-bgp-chaos/scenario/generator_test.go` - Determinism, bounds, ratio tests
- `cmd/ze-bgp-chaos/scenario/profile.go` - PeerProfile type definition
- `cmd/ze-bgp-chaos/scenario/profile_test.go` - ASN, router-id, connection mode tests
- `cmd/ze-bgp-chaos/scenario/routes.go` - IPv4 route generation (other families later)
- `cmd/ze-bgp-chaos/scenario/routes_test.go` - Uniqueness, determinism, count tests
- `cmd/ze-bgp-chaos/scenario/config.go` - Ze config file generation
- `cmd/ze-bgp-chaos/scenario/config_test.go` - Structure, content, determinism tests
- `cmd/ze-bgp-chaos/peer/simulator.go` - Per-peer goroutine skeleton (single peer for now)
- `cmd/ze-bgp-chaos/peer/session.go` - TCP + OPEN + KEEPALIVE exchange
- `cmd/ze-bgp-chaos/peer/session_test.go` - OPEN build, KEEPALIVE, shutdown tests
- `cmd/ze-bgp-chaos/peer/sender.go` - Route UPDATE building and sending
- `cmd/ze-bgp-chaos/peer/sender_test.go` - UPDATE build, EOR tests

## Files to Modify

- `Makefile` - Add `ze-bgp-chaos` build target

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | N/A |
| CLI commands/flags | No | Standalone binary |
| Makefile build target | Yes | `Makefile` |

## Implementation Steps

1. **Create package structure** - directories, go.mod awareness
   → Review: Does it compile with `go build ./cmd/ze-bgp-chaos/...`?

2. **Write CLI skeleton** (`main.go`) - all flags defined, seed printed
   → Review: `ze-bgp-chaos --help` shows all flags?

3. **Write PeerProfile type** (`profile.go`) - struct with all fields
   → Review: Covers ASN, families, routes, connection mode, capabilities?

4. **Write scenario generator tests** - determinism, bounds, ratio
   → Run: Tests FAIL (no implementation yet)

5. **Implement scenario generator** (`generator.go`) - seed → profiles
   → Run: Tests PASS

6. **Write route generation tests** - uniqueness, determinism, no overlap
   → Run: Tests FAIL

7. **Implement IPv4 route generator** (`routes.go`)
   → Run: Tests PASS

8. **Write config generation tests** - structure, content
   → Run: Tests FAIL

9. **Implement config generator** (`config.go`)
   → Run: Tests PASS
   → Verify: Generated config passes `ze validate`

10. **Write session tests** - OPEN build, KEEPALIVE
    → Run: Tests FAIL

11. **Implement BGP session** (`session.go`) - TCP, OPEN exchange, KEEPALIVE loop
    → Run: Tests PASS

12. **Write sender tests** - UPDATE build, EOR
    → Run: Tests FAIL

13. **Implement route sender** (`sender.go`) - build UPDATEs from generated routes
    → Run: Tests PASS

14. **Wire it together** - main.go creates scenario, generates config, runs single peer
    → Manual test against running Ze instance

15. **Verify** - `make lint && make test`
    → Review: Clean build, all tests pass?

16. **Update follow-on specs** - Record learnings in Phase 2-5 skeletons
    → What APIs were easier/harder than expected?
    → What structures need to be shared across phases?
    → Any design changes needed for validation/chaos/families?

## Spec Propagation Task

**MANDATORY at end of this phase:**

Before marking this spec complete, update the following specs with learnings:

1. **`spec-bgp-chaos-validation.md`** — Update with:
   - Actual PeerProfile struct fields (may differ from planned)
   - Actual route data structures (what does a "generated route" look like?)
   - How sessions report state changes (channels? callbacks?)
   - Any shared types that need to move to a common package

2. **`spec-bgp-chaos-chaos.md`** — Update with:
   - How to interrupt a running session (context cancellation? method call?)
   - Session reconnection mechanics (what state needs reset?)
   - KEEPALIVE loop implementation (how to stop/resume for hold-timer chaos)

3. **`spec-bgp-chaos-families.md`** — Update with:
   - How route generation is structured (can it be extended per family?)
   - How UpdateBuilder is used (parameters needed per family)
   - Whether NLRI builders need wrapping or can be called directly

4. **`spec-bgp-chaos-reporting.md`** — Update with:
   - What data is available from session/sender for dashboard display
   - Event types emitted (for JSON log)
   - Channel/interface patterns used (for reporter integration)

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

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| CLI with all flags | | | |
| Seed-based profile generation | | | |
| Ze config output | | | |
| Single-peer BGP session | | | |
| OPEN/KEEPALIVE exchange | | | |
| ipv4/unicast UPDATE sending | | | |
| End-of-RIB marker | | | |
| Graceful shutdown | | | |
| Duration limit | | | |
| Basic stdout output | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | | | |
| AC-2 | | | |
| AC-3 | | | |
| AC-4 | | | |
| AC-5 | | | |
| AC-6 | | | |
| AC-7 | | | |
| AC-8 | | | |
| AC-9 | | | |
| AC-10 | | | |
| AC-11 | | | |
| AC-12 | | | |

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 demonstrated
- [ ] Tests pass (`make test`)
- [ ] No regressions (`make functional`)
- [ ] `ze-bgp-chaos` binary builds

### Quality Gates (SHOULD pass)
- [ ] `make lint` passes
- [ ] Follow-on specs updated (Spec Propagation Task)
- [ ] Implementation Audit completed

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
- [ ] Boundary tests for numeric inputs

### Completion
- [ ] Spec Propagation Task completed
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-bgp-chaos-session.md`

# Spec: fib-depth-1-nhresolver -- NH Resolution Cascade Wiring

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-fib-depth |
| Phase | 1/4 |
| Updated | 2026-05-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `internal/plugins/sysrib/nhresolver.go` -- existing Track/Untrack/Dependents/Resolve
3. `internal/plugins/sysrib/sysrib.go` -- recomputeBest, resolveNextHop, getNHResolver
4. `internal/core/rib/locrib/change.go` -- ChangeHandler, Change struct
5. `internal/core/rib/locrib/manager.go` -- RIB.OnChange subscription

## Task

Wire the nhResolver's dependency tracking into sysrib so that when a next-hop's
covering route changes (reachability lost, metric changed, new path), all prefixes
using that NH are automatically re-evaluated and FIB updates emitted.

The nhResolver already provides Track, Untrack, and Dependents methods. What is
missing:
1. Calling Track(nh, prefix) when sysrib installs a route with a recursive NH
2. Subscribing to Loc-RIB OnChange and detecting when a changed prefix covers a tracked NH
3. Triggering recomputeBest for all dependent prefixes when their NH changes

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- Loc-RIB change notification design

### RFC Summaries
- [ ] `rfc/short/rfc4271.md` -- Section 9.1.2.2 step 6 (IGP cost triggers re-evaluation)

**Key insights:**
- Loc-RIB `OnChange` delivers Change{Family, Prefix, Kind, Best} synchronously under shard lock
- Handler MUST NOT call RIB mutators; must offload to goroutine
- sysRIB.mu and Loc-RIB shard locks have documented ordering constraint (see resolveNextHop comment)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/sysrib/nhresolver.go` -- Track/Untrack/Dependents exist, never called from production
- [ ] `internal/plugins/sysrib/sysrib.go` -- resolveNextHop called in recomputeBest; no tracking
- [ ] `internal/core/rib/locrib/change.go` -- ChangeHandler type, Change struct with Kind/Prefix/Best

**Behavior to preserve:**
- resolveNextHop returns the directly-connected NH (already works)
- recomputeBest emits outgoing changes to FIB (already works)
- Lock ordering: sysRIB.mu -> Loc-RIB shard read locks (never reverse)

**Behavior to change:**
- sysrib subscribes to Loc-RIB OnChange during run()
- On ChangeUpdate/ChangeRemove for a prefix that covers a tracked NH, cascade re-evaluation
- Track/Untrack called during route install/withdraw in recomputeBest

## Data Flow (MANDATORY)

### Entry Point
- Loc-RIB fires ChangeHandler when a covering prefix best-path changes

### Transformation Path
1. OnChange handler receives Change{prefix, kind}
2. Handler checks if any tracked NH falls within the changed prefix
3. For each affected NH: gather all dependent prefixes via Dependents(nh)
4. Queue dependent prefixes for re-evaluation (offload from handler to avoid lock inversion)
5. Worker goroutine calls recomputeBest for each queued prefix under sysRIB.mu

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Loc-RIB -> sysrib | OnChange subscription | [ ] |
| sysrib cascade -> FIB | existing EventBus emission | [ ] |

### Integration Points
- `locrib.RIB.OnChange` -- subscribe during sysrib run()
- `nhResolver.Dependents` -- look up affected prefixes
- `sysRIB.recomputeBest` -- re-evaluate and emit

### Architectural Verification
- [ ] No bypassed layers
- [ ] No lock inversion (handler offloads to channel/goroutine)
- [ ] No duplicated functionality

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Loc-RIB prefix withdrawal | -> | cascade withdraws dependent routes | `TestNHCascadeWithdraw` |
| Loc-RIB metric change | -> | cascade re-evaluates best-path | `TestNHCascadeCostChange` |
| ECMP member NH withdrawn | -> | ECMP group updated, not full withdraw | `TestECMPMemberFail` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | NH becomes unreachable (covering prefix withdrawn) | All routes using that NH withdrawn from FIB |
| AC-2 | NH cost changes (covering prefix metric changes) | Best-path re-evaluated for all prefixes using that NH |
| AC-3 | One ECMP member's NH unreachable | ECMP group updated (member removed), not full withdrawal |
| AC-4 | NH restored after withdrawal | Dependent routes re-installed |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNHCascadeWithdraw` | `internal/plugins/sysrib/sysrib_test.go` | AC-1: NH down cascades withdrawal | PASS |
| `TestNHCascadeCostChange` | `internal/plugins/sysrib/sysrib_test.go` | AC-2: metric change triggers re-eval | PASS |
| `TestECMPMemberFail` | `internal/plugins/sysrib/sysrib_test.go` | AC-3: partial ECMP removal | PASS |
| `TestNHCascadeRestore` | `internal/plugins/sysrib/sysrib_test.go` | AC-4: NH restored re-installs | PASS |
| `TestNHResolver_CoveredNHs` | `internal/plugins/sysrib/nhresolver_test.go` | CoveredNHs helper | PASS |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A | N/A | Internal mechanism, not protocol-visible. Unit tests exercise the full Loc-RIB -> sysrib -> EventBus pipeline. | N/A |

### Interop Tests
- N/A (internal mechanism, not protocol-visible)

## Files Modified
- `internal/plugins/sysrib/sysrib.go` -- Track/Untrack wiring, cascade worker, async OnChange, resolvedNH tracking
- `internal/plugins/sysrib/nhresolver.go` -- CoveredNHs, familyForPrefix helpers
- `internal/plugins/sysrib/sysrib_test.go` -- 4 cascade tests + helpers
- `internal/plugins/sysrib/nhresolver_test.go` -- CoveredNHs test
- `internal/core/rib/locrib/manager.go` -- LPM race fix (g.best() inside read lock)

## Implementation Steps

### Implementation Phases

1. **Phase: Track wiring** -- call Track(nh, prefix) in recomputeBest when a route has recursive NH
   - Tests: verify tracking map populated after route install
   - Files: sysrib.go
   - Verify: Track called for recursive routes, Untrack on withdrawal

2. **Phase: OnChange subscription** -- subscribe to Loc-RIB in sysrib run(), offload to channel
   - Tests: verify handler fires for covering prefix changes
   - Files: sysrib.go
   - Verify: no lock inversion, handler is non-blocking

3. **Phase: Cascade worker** -- goroutine reads channel, calls recomputeBest for dependents
   - Tests: TestNHCascadeWithdraw, TestNHCascadeCostChange
   - Files: sysrib.go
   - Verify: dependent routes withdrawn/updated after NH change

4. **Phase: ECMP member removal** -- partial cascade updates ECMP group
   - Tests: TestECMPMemberFail
   - Files: sysrib.go, ecmp.go
   - Verify: ECMP group shrinks, route not fully withdrawn

### Critical Review Checklist
| Check | What to verify |
|-------|---------------|
| Lock ordering | OnChange handler never acquires sysRIB.mu directly |
| Completeness | All 4 ACs have tests |
| No stale tracking | Untrack called on every withdrawal path |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | Cascade channel bounded; drop or coalesce if overloaded |
| Infinite cascade | NH pointing to self already handled by maxRecursionDepth |

## Implementation Summary

### What Was Implemented
- Track/Untrack wiring in `recomputeBest`: calls `Track(nh, prefix)` when a route becomes best, `Untrack` when it's replaced or withdrawn
- `CoveredNHs(prefix)` method on nhResolver: returns all tracked NHs within a given prefix
- `cascadeRecompute(key)` method on sysRIB: re-resolves NH, handles ECMP member removal, emits FIB changes
- `processCascade(nhs)` method: multi-level cascade with seen-set to prevent infinite loops
- `processLocRIBChange(c)` method: processes a single Loc-RIB change with cascade check
- Async change processing in `run()`: OnChange handler queues to channel, worker goroutine processes outside shard lock
- `resolvedNH` map on sysRIB: tracks last emitted resolved NH per prefix for cascade comparison
- LPM race fix in locrib: moved `g.best()` inside shard read lock to prevent PathGroup backing-array race

### Deviations from Plan
- Moved ALL Loc-RIB change processing to async channel worker (not just cascade) to fix a pre-existing deadlock: OnChange handler ran under shard write lock, processEvent -> resolveNextHop -> LPM tried to read-lock the same shard
- Functional .ci test dropped: cascade is an internal mechanism with no protocol-visible behavior. Unit tests exercise the full pipeline (real Loc-RIB, real nhResolver, real EventBus, real run())
- `ecmpCollectResolved` added as a new helper for cascade-aware ECMP collection (resolves each member's NH)
- `familyForPrefix` added to nhresolver.go for cascade use

## Implementation Audit

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | PASS | `TestNHCascadeWithdraw` | Covering route withdrawal cascades FIB withdraw |
| AC-2 | PASS | `TestNHCascadeCostChange` | Metric change triggers re-evaluation |
| AC-3 | PASS | `TestECMPMemberFail` | Partial ECMP removal, not full withdrawal |
| AC-4 | PASS | `TestNHCascadeRestore` | NH restoration re-installs dependent routes |

## Review Gate

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Functional tests for end-to-end behavior

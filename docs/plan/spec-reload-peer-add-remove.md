# Spec: reload-peer-add-remove

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `test/reload/reload-add-route.ci` - existing reload test pattern
4. `internal/test/peer/peer.go` - SIGHUP delivery mechanism
5. `internal/plugin/bgp/reactor/reactor.go` - reconcilePeers()

## Task

Add functional tests for the three reload scenarios deferred from spec-config-reload-3-sighup:

1. **Remove peer** — peer block removed from config, SIGHUP sent, session torn down
2. **Add peer** — new peer block added to config, SIGHUP sent, new session established
3. **No-change (standalone)** — SIGHUP with identical config, no session disruption

**Context:** The daemon code already handles all three cases via `reconcilePeers()` in `reactor.go`. Only the functional tests were deferred because SIGHUP testing needed daemon orchestration — which spec 225 later built (`action=sighup`, `action=rewrite` in .ci format). These tests are now feasible.

**Note:** `reload-rapid-sighup.ci` already tests the no-change case as its second SIGHUP. A standalone test adds clarity but is lower priority.

## Required Reading

### Architecture Docs
- [ ] `docs/functional-tests.md` - .ci format reference
- [ ] `docs/architecture/testing/ci-format.md` - formal .ci specification

### Source Files (MUST read)
- [ ] `test/reload/reload-add-route.ci` - existing pattern for SIGHUP reload tests
- [ ] `test/reload/reload-restart-peer.ci` - existing pattern for peer restart
- [ ] `internal/test/peer/peer.go` - NextSighupAction(), SIGHUP delivery via daemon.pid
- [ ] `internal/plugin/bgp/reactor/reactor.go:1054-1110` - reconcilePeers() add/remove logic

**Key insights:**
- Test infrastructure already supports `action=sighup` and `action=rewrite` in .ci files
- ze-peer reads `daemon.pid` from working directory and sends SIGHUP via `syscall.Kill`
- reconcilePeers diffs current peers vs new config: missing peers get `peer.Stop()`, new peers get `AddPeer()`
- Existing tests only cover peer setting/route changes — not peer addition or removal

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `test/reload/reload-add-route.ci` - pattern: initial config → SIGHUP with rewritten config → verify reconnection
- [ ] `internal/test/peer/peer.go` - SIGHUP action (line 464-480): read daemon.pid, syscall.Kill(pid, SIGHUP), sleep 500ms
- [ ] `internal/plugin/bgp/reactor/reactor.go` - reconcilePeers (line 1054-1110) already handles add/remove

**Behavior to preserve:**
- All existing 4 reload tests must continue passing
- reconcilePeers logic unchanged — tests validate existing behavior

**Behavior to change:**
- None — this spec adds tests only, no daemon code changes

## Data Flow (MANDATORY)

### Entry Point
- SIGHUP signal → reactor signal handler → Reload() → reloadFunc → reconcilePeers()

### Transformation Path
1. SIGHUP received by signal handler in reactor
2. Signal handler calls adapter.Reload()
3. Reload calls reloadFunc(configPath) to parse config file
4. Reload builds new peer list, calls reconcilePeers(newPeers)
5. reconcilePeers diffs current vs new: Stop removed, AddPeer for new

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Test peer → Daemon | SIGHUP via daemon.pid + syscall.Kill | [x] Existing tests use this |
| Config file → Reactor | action=rewrite changes file, SIGHUP triggers re-read | [x] Existing tests use this |

### Integration Points
- `reconcilePeers()` — already implemented, handles add/remove/change
- `peer.Stop()` — closes session, sends NOTIFICATION on remove
- `AddPeer()` — creates new peer, initiates connection on add

### Architectural Verification
- [x] No bypassed layers (tests use standard SIGHUP → reload path)
- [x] No unintended coupling (tests are pure .ci files)
- [x] No duplicated functionality (new test scenarios, not duplicating existing)
- [x] Zero-copy preserved where applicable (N/A — test-only)

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Existing reload unit tests | `internal/plugin/bgp/reactor/reload_test.go` | reconcilePeers add/remove/change logic | Already passing |

No new unit tests — daemon code is already tested. This spec adds functional tests only.

### Boundary Tests (MANDATORY for numeric inputs)
N/A — no new numeric inputs.

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `reload-remove-peer` | `test/reload/reload-remove-peer.ci` | Peer removed from config, SIGHUP → session torn down, daemon survives | |
| `reload-add-peer` | `test/reload/reload-add-peer.ci` | New peer added to config, SIGHUP → new BGP session established | |
| `reload-no-change` | `test/reload/reload-no-change.ci` | SIGHUP with same config → no session disruption | |

## Files to Modify
- None — test-only spec

## Files to Create
- `test/reload/reload-remove-peer.ci` — peer removal via SIGHUP
- `test/reload/reload-add-peer.ci` — peer addition via SIGHUP
- `test/reload/reload-no-change.ci` — no-op SIGHUP (standalone)

## Implementation Steps

### Step 1: Read existing reload tests and ci-format docs
Understand the exact format, action triggers, and expectations. Confirm ze-peer SIGHUP delivery works.

### Step 2: Write reload-remove-peer.ci
- Initial config: 1 peer at 127.0.0.1 (standard setup)
- Rewritten config: bgp section with no peer blocks
- After SIGHUP: daemon calls reconcilePeers, finds peer missing, calls peer.Stop()
- Test verification: conn=1 receives UPDATE + EOR, then action=rewrite + action=sighup. No conn=2 expected — ze-peer completes when conn=1 expectations are met and connection closes.
- tcp_connections=1 (only one connection expected)

### Step 3: Write reload-add-peer.ci
**Investigation needed:** adding a peer means a NEW connection must be established after SIGHUP. Options:
- **Option A:** Start with peer A at 127.0.0.1, after SIGHUP add peer B at 127.0.0.2. Both connect to same ze-peer if it binds 0.0.0.0. Verify conn=2 appears from new peer.
- **Option B:** Start with no peers configured. Use an initial timer/delay to trigger rewrite+sighup without needing conn=1. Requires .ci format enhancement.
- **Option C:** Start with peer A, after SIGHUP keep peer A + add peer B at same address (different port?). Not valid BGP.
- Evaluate options during implementation. Option A is preferred if ze-peer can bind all interfaces.

### Step 4: Write reload-no-change.ci
- Initial config: 1 peer at 127.0.0.1
- Rewritten config: identical copy
- After SIGHUP: reconcilePeers finds no diff, no peers added/removed
- Test verification: conn=1 receives UPDATE + EOR, action=sighup (no rewrite — same config). tcp_connections=1, no conn=2. Daemon continues normally.

### Step 5: Run all reload tests
`make functional` — all reload tests pass, including existing 4.

## RFC Documentation
N/A — test-only spec.

## Implementation Summary

<!-- Fill this section AFTER implementation, before moving to done -->

### What Was Implemented
- (TBD)

### Bugs Found/Fixed
- (TBD)

### Design Insights
- (TBD)

### Documentation Updates
- (TBD)

### Deviations from Plan
- (TBD)

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| reload-remove-peer.ci | | | |
| reload-add-peer.ci | | | |
| reload-no-change.ci | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| reload-remove-peer | | | |
| reload-add-peer | | | |
| reload-no-change | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| test/reload/reload-remove-peer.ci | | |
| test/reload/reload-add-peer.ci | | |
| test/reload/reload-no-change.ci | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### 🏗️ Design (see `rules/design-principles.md`)
- [ ] No premature abstraction (3+ concrete use cases exist?)
- [ ] No speculative features (is this needed NOW?)
- [ ] Single responsibility (each component does ONE thing?)
- [ ] Explicit behavior (no hidden magic or conventions?)
- [ ] Minimal coupling (components isolated, dependencies minimal?)
- [ ] Next-developer test (would they understand this quickly?)

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs
- [ ] Feature code integrated into codebase
- [ ] Functional tests verify end-user behavior

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation (during implementation)
- [ ] Required docs read
- [ ] RFC summaries read (all referenced RFCs)
- [ ] RFC references added to code
- [ ] RFC constraint comments added

### Completion (after tests pass)
- [ ] Architecture docs updated with learnings
- [ ] Implementation Audit completed
- [ ] All Partial/Skipped items have user approval
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together

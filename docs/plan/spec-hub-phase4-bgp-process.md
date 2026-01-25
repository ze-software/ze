# Spec: hub-phase4-bgp-process

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/hub-separation-phases.md` - phase overview
4. `internal/reactor/reactor.go` - current BGP code location

## Task

Move BGP code under `internal/plugin/bgp/` and make `ze bgp` work as a forked child process of the hub.

**Scope:** Package moves, make ze bgp accept config via stdin, send events via stdout.

**Depends on:** Phase 3 complete

## Required Reading

### Source Files
- [ ] `internal/bgp/*` - BGP protocol code (to move)
- [ ] `internal/rib/` - peer-to-peer RIB (to move with BGP)
- [ ] `internal/reactor/` - reactor code (to move)
- [ ] `cmd/ze/bgp/` - existing ze bgp command

**Key insights:**
- `internal/bgp/*` contains message, attribute, nlri, capability, fsm
- `internal/rib/` is for peer-to-peer routing (stays with BGP)
- `internal/plugin/rib/` is for adj-rib tracking (separate process)
- `internal/reactor/` mixes hub + BGP concerns, needs splitting

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBGPProcessStage1` | `internal/plugin/bgp/process_test.go` | BGP declares schema + handlers | |
| `TestBGPProcessStage2` | `internal/plugin/bgp/process_test.go` | BGP accepts config JSON | |
| `TestBGPProcessReady` | `internal/plugin/bgp/process_test.go` | BGP completes 5-stage | |
| `TestBGPProcessCommand` | `internal/plugin/bgp/process_test.go` | BGP handles commands | |
| `TestBGPProcessEvent` | `internal/plugin/bgp/process_test.go` | BGP emits events | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A - existing BGP code, no new numeric inputs | | | | |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `hub-bgp-startup` | `test/data/hub/bgp-startup.ci` | Hub forks BGP, BGP completes 5-stage | |
| `hub-bgp-standalone` | `test/data/hub/bgp-standalone.ci` | `ze bgp` works standalone (no hub) | |

**Smoke test:** Verify BGP works both as hub child and standalone. Critical for regression testing.

## Files to Move

| From | To | Purpose |
|------|-----|---------|
| `internal/bgp/*` | `internal/plugin/bgp/*` | BGP protocol code |
| `internal/rib/` | `internal/plugin/bgp/rib/` | Peer-to-peer zero-copy routing (BGP engine internal) |
| `internal/reactor/` | `internal/plugin/bgp/reactor/` | BGP event loop |

**Note:** `internal/plugin/rib/` is the **adj-RIB tracking plugin** (separate process) - it stays where it is. `internal/rib/` is the **BGP engine's internal peer-to-peer routing** for zero-copy route passing - it moves with BGP.

**⚠️ Impact Assessment (as of 2026-01-25):**
- **126 files** contain `internal/bgp` imports
- **239 total import references** need updating
- Estimate: 2-4 hours for import updates + testing
- Run `grep -r "internal/bgp" --include="*.go" | wc -l` before starting to get current count

## Files to Modify

- `cmd/ze/bgp/main.go` - Accept config via stdin, work as child
- All files that import moved packages - Update imports

## Files to Create

- `internal/plugin/bgp/process.go` - BGP process entry point for child mode

## Implementation Steps

1. **Write unit tests** - Test BGP as child process

   → **Review:** Can test without full hub running?

2. **Run tests** - Verify FAIL (paste output)

3. **Move packages** - Use git mv to preserve history
   ```bash
   git mv internal/bgp internal/plugin/bgp
   git mv internal/rib internal/plugin/bgp/rib
   git mv internal/reactor internal/plugin/bgp/reactor
   ```

   → **Review:** All imports updated?

4. **Update imports** - Fix all broken imports

5. **Create process.go** - BGP child mode entry point

   **RunAsChild behavior:**
   1. Stage 1: declare ze-bgp schema and handlers
   2. Stage 2: receive config JSON from hub
   3. Stage 3-5: complete 5-stage protocol
   4. Run BGP reactor with received config

6. **Modify cmd/ze/bgp** - Detect child vs standalone mode

7. **Run tests** - Verify PASS (paste output)

8. **Run existing tests** - Ensure nothing broken
   ```bash
   make test && make lint && make functional
   ```

   → **Review:** All 80+ functional tests still pass?

## Design Decisions

### What stays in reactor vs moves to hub?

| Component | Location | Why |
|-----------|----------|-----|
| Process forking | Hub | Orchestrator concern |
| Pipe management | Hub | Orchestrator concern |
| Signal handling | Hub | Global signals |
| TCP listeners | BGP reactor | BGP-specific |
| Sessions/FSM | BGP reactor | BGP-specific |
| Peer management | BGP reactor | BGP-specific |

### Child detection

`ze bgp` detects child mode using this precedence:

| Check | Result |
|-------|--------|
| `--child` flag present | Child mode |
| `ZE_CHILD_MODE=1` env var | Child mode |
| stdin is pipe AND no config file arg | Child mode |
| Otherwise | Standalone mode |

**Why explicit flag:** User might pipe config to standalone mode (`cat config.conf | ze bgp`). Flag removes ambiguity.

## Implementation Summary

<!-- Fill after implementation -->

### What Was Implemented
- [List actual changes]

### Bugs Found/Fixed
- [Any bugs discovered]

### Deviations from Plan
- [Any differences and why]

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation (during implementation)
- [ ] Required docs read

### Completion (after tests pass)
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together

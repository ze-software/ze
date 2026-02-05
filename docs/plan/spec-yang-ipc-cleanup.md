# Spec: yang-ipc-cleanup

## ⚠️ REDUCED SCOPE — Verify + Benchmark Only

**This spec was reduced from a full cleanup spec.** Since Specs 1-3 follow the no-layering rule (replace, don't layer), there should be minimal legacy code remaining. This spec verifies completeness and captures performance benchmarks.

If Specs 1-3 left orphaned code, this spec identifies and removes it. If not, this spec is just verification.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. Completed specs 1-3 in `docs/plan/done/`
4. `internal/plugin/` - all plugin files

## Task

Verify that Specs 1-3 fully replaced all legacy protocol code. Capture post-implementation performance benchmarks and compare with baseline from Spec 1. Update architecture documentation to reflect the YANG IPC protocol.

**Depends on:** spec-yang-ipc-schema (Spec 1), spec-yang-ipc-dispatch (Spec 2), spec-yang-ipc-plugin (Spec 3), all plugin update specs

**BLOCKING:** Do not start this spec until Specs 1-3 and all plugin updates are complete and all tests pass.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - current API design

### Completed Specs
- [ ] `docs/plan/done/XXX-yang-ipc-schema.md` - what was implemented
- [ ] `docs/plan/done/XXX-yang-ipc-dispatch.md` - what was implemented
- [ ] `docs/plan/done/XXX-yang-ipc-plugin.md` - what was implemented

### Source Files to Review
- [ ] `internal/plugin/server.go` - verify no legacy code paths
- [ ] `internal/plugin/process.go` - verify no legacy code paths
- [ ] `internal/plugin/registration.go` - verify ParseLine deleted

## Current Behavior (MANDATORY)

**Source files read:** (read after Specs 1-3 complete)
- [ ] `internal/plugin/server.go` - should have only JSON protocol
- [ ] `internal/plugin/process.go` - should have only socket pairs

**Behavior to preserve:**
- All YANG IPC functionality from Specs 1-3
- All BGP functionality
- All test coverage

**Behavior to change:**
- Remove: Any orphaned legacy code found during verification
- Update: Documentation to YANG IPC only

## Data Flow (MANDATORY)

### Entry Point
- This is a verification spec; no new data flow

### Transformation Path
- N/A (verification, not addition)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| N/A (verification) | Code review only | [ ] |

### Integration Points
- Verify all integration points from Specs 1-3 work correctly

### Architectural Verification
- [ ] No orphaned code after Specs 1-3
- [ ] All tests still pass
- [ ] Documentation accurate

## Verification Checklist

### Code Verification

Search for orphaned legacy code:

| Pattern to Search | Should Find | Action |
|-------------------|-------------|--------|
| `ReadString('\n')` in plugin/ | Zero matches | If found, delete |
| `parseSerial` | Zero matches | If found, delete |
| `isComment` | Zero matches | If found, delete |
| `encodeAlphaSerial` | Zero matches | If found, delete |
| `RegisterBuiltin` | Zero matches | If found, delete |
| `ParseLine` in registration.go | Zero matches | If found, delete |
| `stdin` in process.go | Zero matches | If found, delete |
| `config done` text string | Zero matches | If found, delete |
| `declare family` text string | Zero matches | If found, delete |
| `capability hex` text string | Zero matches | If found, delete |
| `WriteQueueHighWater` | Zero matches | If found, delete |

### Performance Comparison

Compare with baseline from Spec 1 (`internal/plugin/benchmark_test.go`):

| Metric | Baseline (Spec 1) | Post-YANG IPC | Target |
|--------|-------------------|---------------|--------|
| Connection setup time | ______ | ______ | <= Baseline |
| Event throughput (events/sec) | ______ | ______ | >= Baseline |
| Memory per connection | ______ | ______ | <= Baseline |
| Plugin startup time | ______ | ______ | <= Baseline |
| Command dispatch | ______ | ______ | <= Baseline |

## Documentation Updates

| Document | Updates Needed |
|----------|---------------|
| `docs/architecture/api/architecture.md` | YANG IPC protocol description |
| `.claude/rules/plugin-design.md` | Update plugin patterns for RPC |
| `.claude/rules/json-format.md` | YANG-defined message format |
| `CLAUDE.md` | Update API section |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNoLegacyCode` | `internal/plugin/verify_test.go` | No orphaned legacy patterns | |

### Boundary Tests
- N/A (verification, not new functionality)

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| All existing tests | `test/` | Must still pass | |

## Files to Modify
- Documentation files (see table above)
- Any orphaned files found during verification (if any)

## Files to Create
- `internal/plugin/verify_test.go` - tests verifying legacy removal (if needed)

## Implementation Steps

1. **Run all tests** - Verify everything passes after Specs 1-3

2. **Search for orphaned code** - Use verification checklist
   → **Review:** Any unexpected leftovers?

3. **Remove orphans** - If any found
   → **Review:** Tests still pass?

4. **Run performance benchmarks** - Compare with baseline

5. **Update documentation** - All architecture docs

6. **Full functional test suite** - All .ci files

7. **Verify all** - `make lint && make test && make functional`

8. **Final self-review**

## Implementation Summary

<!-- Fill AFTER implementation -->

### What Was Implemented
-

### Orphaned Code Found
-

### Performance Results
| Metric | Baseline | Post-YANG IPC | Change |
|--------|----------|---------------|--------|
| | | | |

### Documentation Updated
-

### Deviations from Plan
-

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| No orphaned legacy code | | | |
| Performance meets baseline | | | |
| Documentation updated | | | |
| All tests passing | | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Verification
- [ ] No orphaned code
- [ ] Performance meets baseline
- [ ] Documentation accurate
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Completion
- [ ] Architecture docs updated
- [ ] Implementation Audit completed
- [ ] All specs moved to done
- [ ] All files committed together

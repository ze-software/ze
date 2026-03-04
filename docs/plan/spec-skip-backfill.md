# Spec: skip-backfill

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/buffer-first.md` - current encoding rules
4. `.claude/hooks/block-encoding-alloc.sh` - current hook scope

## Task

Audit all `Len()`-then-`WriteTo()` call sites, enforce skip-and-backfill as the canonical encoding pattern, and expand the existing hook to prevent regressions in hot paths.

**Origin:** Comparison with freeRtr's `packHolder` revealed Ze already uses skip-and-backfill in the hot path (`reactor_wire.go`) but has no enforcement preventing future contributors from introducing `Len()`-then-`WriteTo()` double traversals. The existing `block-encoding-alloc.sh` hook checks for `append()`/`make()`/`.Bytes()`/`.Pack()` but not for the Len-then-WriteTo anti-pattern.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/pool-architecture.md` - pool design, buffer lifecycle
  → Constraint: pool buffers are bounded to RFC max sizes
- [ ] `.claude/rules/buffer-first.md` - current banned patterns
  → Decision: WriteTo(buf, off) is the canonical write interface

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - UPDATE message format, attribute header format
  → Constraint: attribute header is 3 or 4 bytes depending on Extended Length flag

**Key insights:**
- BGP has exactly 3 variable-length fields with fixed-position length placeholders (message length, withdrawn length, path attributes length) — skip-and-backfill works perfectly
- Attribute headers have a 3-vs-4-byte ambiguity (Extended Length flag), but each `write*Attr()` helper in `reactor_wire.go` already handles this inline

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/reactor_wire.go` - hot path UPDATE building, already uses skip-and-backfill exclusively, zero Len() calls
- [ ] `internal/component/bgp/message/update.go` - Update.WriteTo() uses lengthPos backfill
- [ ] `internal/component/bgp/message/update_split.go` - UPDATE splitter, calls chunk.Len() externally then calls WriteAttrTo() which calls Len() internally (double traversal)
- [ ] `internal/component/bgp/attribute/attribute.go` - AttributesSize() sums Len(), WriteAttrTo() calls Len() for header size decision
- [ ] `internal/component/bgp/rib/store.go` - Hash()/Equal()/Key() use Len()-then-WriteTo() for dedup serialization
- [ ] `internal/component/bgp/rib/route.go` - Index() uses Len()-then-WriteTo(), but caches result in indexCache
- [ ] `internal/component/bgp/rib/outgoing.go` - buildNLRIIndex() uses Len()-then-WriteTo()
- [ ] `internal/component/bgp/reactor/session_negotiate.go` - OPEN building sums capability Len() then writes
- [ ] `internal/component/bgp/wire/writer.go` - BufWriter (no Len) and CheckedBufWriter (with Len) interfaces
- [ ] `internal/component/bgp/context/context.go` - WireWriter interface (Len + WriteTo)
- [ ] `.claude/hooks/block-encoding-alloc.sh` - only scopes to update_build* and message/pack*

**Behavior to preserve:**
- All current skip-and-backfill in `reactor_wire.go` — this is the gold standard
- `CheckedWriteTo()` pattern: `Len()` for capacity guard then `WriteTo()` — this is safe, not a double traversal
- RIB dedup in `rib/store.go` Hash/Equal/Key — unavoidable, need serialized bytes for comparison
- `indexCache` in `rib/route.go` — amortizes the cost correctly
- Cold-path `Len()-then-WriteTo()` in `format/`, `reactor_api_routes.go`, `commit/` — not worth optimizing

**Behavior to change:**
- `update_split.go` double `Len()`: external `chunk.Len()` + internal `WriteAttrTo()` → `Len()` again
- Hook scope: expand `block-encoding-alloc.sh` to also detect `Len()-then-WriteTo()` in hot paths
- Rule: update `buffer-first.md` to name skip-and-backfill explicitly and ban `Len()-then-WriteTo()` in hot paths
- `reactor_wire.go`: add comment documenting the skip-and-backfill pattern for future contributors

## Data Flow (MANDATORY)

### Entry Point
- Wire encoding starts at `reactor_wire.go` `WriteAnnounceUpdate()` / `WriteWithdrawUpdate()`
- Also `message/update.go` `Update.WriteTo()` for pre-built updates
- Also `message/update_split.go` `SplitUpdate()` for oversized updates

### Transformation Path
1. Caller gets pool buffer (`getBuildBuf()`)
2. Writes marker + header placeholders (skip)
3. Writes payload attributes/NLRI forward (write)
4. Backfills length fields at saved positions (backfill)
5. Returns buffer to pool after send

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Pool ↔ Encoder | Get/Put lifecycle | [ ] |
| Encoder ↔ Session | writeUpdate(buf[:n]) | [ ] |

### Integration Points
- `block-encoding-alloc.sh` hook — extend with new check
- `.claude/rules/buffer-first.md` — add skip-and-backfill section

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Edit to reactor_wire.go with Len()-then-WriteTo() | → | block-encoding-alloc.sh rejects | `TestHookBlocksLenThenWriteTo` |
| Edit to update_split.go with double Len() | → | Refactored SplitUpdate avoids double traversal | `TestSplitUpdateNoDoubleLenCall` (existing split tests still pass) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `block-encoding-alloc.sh` receives code with `x.Len()` followed by `x.WriteTo()` in hot-path file | Hook exits 2 (blocked) |
| AC-2 | `block-encoding-alloc.sh` receives code with `CheckedWriteTo()` calling internal `Len()` | Hook exits 0 (allowed — capacity guard pattern) |
| AC-3 | `update_split.go` SplitUpdate path | No double `Len()` traversal on same attribute chunk |
| AC-4 | `.claude/rules/buffer-first.md` | Documents skip-and-backfill pattern with before/after examples |
| AC-5 | `reactor_wire.go` | Has comment block documenting skip-and-backfill pattern for future contributors |
| AC-6 | All existing tests | `make test-all` passes — zero functional regressions |

## Audit: Len()-then-WriteTo() Call Sites

### Classification

| File | Call Site | Path | Pattern | Action |
|------|----------|------|---------|--------|
| `reactor_wire.go` | (none) | HOT | Already skip-and-backfill | None — document pattern |
| `message/update.go` | (none — uses pre-set slices) | HOT | Already backfill | None |
| `message/update_split.go:224,265` | `chunk.Len()` then `WriteAttrTo()` which calls `Len()` again | COLD | Double Len() | Fix: pass pre-computed length to `WriteAttrTo` |
| `attribute/attribute.go:232` | `WriteAttrTo()` calls `attr.Len()` for header decision | WARM | Len for flag decision | Keep — 1 byte overhead avoided cheaply |
| `attribute/attribute.go:201` | `AttributesSize()` sums `Len()` | WARM | Pre-alloc sizing | Keep — used in paths needing buffer pre-alloc |
| `rib/store.go:46,60,90` | `Hash()`, `Equal()`, `Key()` | WARM | Unavoidable dedup serialization | Keep — no alternative for byte equality |
| `rib/route.go:255` | `Index()` | WARM | Cached after first call | Keep — indexCache amortizes |
| `rib/outgoing.go:149` | `buildNLRIIndex()` | WARM | Per-withdrawal | Keep — cold-ish, per event |
| `rib/grouping.go:134` | Grouping key | COLD | RR grouping | Keep |
| `reactor/session_negotiate.go:153` | OPEN capability sizing | COLD | Once per session | Keep |
| `reactor/reactor_api_routes.go:254,530` | ExtCommunity extraction | COLD | API path | Keep |
| `format/text.go:632,825` | Unknown attr hex dump | COLD | Display | Keep |
| `format/decode.go:152` | Unknown capability hex | COLD | Display | Keep |
| `commit/commit_manager.go:89` | NLRI index | COLD | Config commit | Keep |
| `message/common_attrs.go:64` | ExtCommunity extraction | COLD | Attr collection | Keep |

### Summary
- **Fix:** 1 site (`update_split.go` double Len)
- **Document:** 1 site (`reactor_wire.go` — add pattern comment)
- **Keep:** 13 sites — all warm/cold paths where Len()-then-WriteTo() is acceptable or unavoidable

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSplitUpdatePreservesOutput` | `message/update_split_test.go` | Split produces identical wire bytes after refactor | |
| `TestWriteAttrToWithKnownLen` | `attribute/attribute_test.go` | New WriteAttrToLen() variant works correctly | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A — no new numeric inputs. This is a refactor + enforcement spec.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing split tests | `message/update_split_test.go` | Large UPDATEs split correctly | |
| `make test-all` | All | No regressions | |

### Future
- Property test: `WriteTo` output identical regardless of whether `Len()` was called first (fuzz)

## Files to Modify

- `.claude/rules/buffer-first.md` - add skip-and-backfill section + ban Len-then-WriteTo in hot paths
- `.claude/hooks/block-encoding-alloc.sh` - add Len-then-WriteTo detection for hot-path files
- `internal/component/bgp/reactor/reactor_wire.go` - add documentation comment for the pattern
- `internal/component/bgp/message/update_split.go` - eliminate double Len() traversal
- `internal/component/bgp/attribute/attribute.go` - add `WriteAttrToLen()` variant or pass length param

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | |
| RPC count in arch docs | No | |
| CLI commands/flags | No | |
| Plugin SDK docs | No | |
| Functional test for new RPC/API | No | |

## Files to Create
None.

## Implementation Steps

1. **Write test for update_split.go** — verify current split output, then ensure refactored version produces identical bytes → Review: captures the wire output before/after?
2. **Run tests** → Verify PASS on existing behavior (baseline)
3. **Refactor update_split.go** → Pass pre-computed attr length to `WriteAttrTo` (or add `WriteAttrToLen` accepting length), eliminating internal `Len()` re-call → Review: no functional change? Simplest approach?
4. **Run tests** → Verify PASS, identical output
5. **Update `buffer-first.md`** → Add "Skip-and-Backfill" section documenting the pattern, add Len-then-WriteTo to banned table for hot paths
6. **Update `block-encoding-alloc.sh`** → Add check for `.Len()` followed by `.WriteTo()` on same variable in hot-path files. Expand scope to include `reactor_wire*` files
7. **Add pattern comment to `reactor_wire.go`** → Document skip-and-backfill at top of WriteAnnounceUpdate
8. **Verify all** → `make test-all`
9. **Critical Review** → All 6 quality checks
10. **Complete spec** → Audit, learned summary, commit

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3 (fix refactor) |
| Split output differs | Step 3 (refactor changed semantics — investigate) |
| Hook false positive on CheckedWriteTo | Step 6 (refine regex to exclude capacity-guard pattern) |
| Hook misses real anti-pattern | Step 6 (add test case, tighten regex) |

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
N/A — this spec is about internal encoding patterns, not protocol compliance.

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
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make test-all` passes (lint + all ze tests)
- [ ] Feature code integrated
- [ ] Architecture docs updated (`buffer-first.md`)
- [ ] Critical Review passes

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
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `docs/learned/NNN-<name>.md`
- [ ] Summary included in commit

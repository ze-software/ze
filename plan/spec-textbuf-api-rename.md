# Spec: textbuf API rename for allocation-first consistency

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-06-09 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/no-sprintf-alloc.md` - textbuf usage rules and decision tree
4. `internal/core/textbuf/textbuf.go` - the library being renamed

## Task

Rename textbuf standalone functions so the bare name means zero-alloc append (matching Buffer chain methods) and the string-returning allocating version gets a marked name.

Currently `textbuf.Uint(v)` returns `string` (allocates) while `textbuf.AppendUint(dst, v)` appends zero-alloc into a caller buffer. This is backwards: in a zero-alloc-first library, `b.Uint(v)` on Buffer already means "append." The standalone API should match.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/no-sprintf-alloc.md` - textbuf usage rules, decision tree, banned patterns
  -> Constraint: the decision tree, replacement tables, and self-check all reference the current names; every instance must be updated
  -> Constraint: the hook `_TEXTBUF_REF` in pretool-writeedit.py also references current names
- [ ] `ai/rules/memory-architecture.md` - data lifecycle, caller-owned buffers, pool strategy
  -> Decision: caller-owned buffers are the preferred pattern; bare name should match this
- [ ] `ai/rules/buffer-first.md` - WriteTo pattern, zero-copy encoding
  -> Constraint: AppendTo(buf) []byte is the canonical wire encoding pattern; standalone textbuf append functions should feel like the same family
- [ ] `plan/learned/713-textbuf-alloc-sweep.md` - prior textbuf migration
  -> Constraint: 689 Sprintf + 254 Itoa were already converted; the new names must not regress any of those callsites

### RFC Summaries (MUST for protocol work)
N/A - internal refactor, no protocol impact.

**Key insights:**
- textbuf exists to eliminate fmt.Sprintf/strconv allocations
- Buffer chain methods (.Uint, .Addr, etc.) all mean "append into buffer" -- already correct
- Standalone functions have inconsistent naming: bare name allocates, Append prefix is zero-alloc
- Combo helpers (StrInt, StrUint, IntStr, UintStr, StrIntStr, StrUintStr) already encode "Str" in their name and return strings
- 86 callsites use string-returning standalone functions, 4 use Append functions
- ~55 production files + ~30 test files need updating

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/textbuf/textbuf.go` - 569 lines, standalone + Buffer + Append functions
  -> Constraint: Buffer chain methods (.Uint, .Int, .Addr, etc.) are NOT renamed -- already mean "append"
  -> Constraint: combo helpers (StrInt, StrUint, etc.) are NOT renamed -- already encode "Str" in name
  -> Constraint: Join, HostPort are NOT renamed -- string-returning, names are clear
  -> Decision: only the 10 string-returning standalone functions and 6 Append functions are renamed

**Behavior to preserve:**
- All textbuf.Buffer chain methods unchanged (already correct: .Uint = append)
- All combo helpers unchanged (StrInt, StrUint, IntStr, UintStr, StrIntStr, StrUintStr)
- textbuf.Join, textbuf.HostPort unchanged (string-returning, names are clear)
- All Append-into-buffer semantics preserved (functions just get shorter names)
- All string-returning semantics preserved (functions just get prefixed names)
- Zero runtime behavior change for any caller

**Behavior to change:**
- Standalone function names only (no semantic change)

## Data Flow (MANDATORY)

### Entry Point
N/A: pure rename refactor, no data flow change.

### Transformation Path
N/A

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| N/A | Pure rename, no boundary changes | [ ] |

### Integration Points
- Every file calling `textbuf.Uint`, `textbuf.Int`, `textbuf.Addr`, `textbuf.Prefix`, `textbuf.Hex`, `textbuf.HexUpper`, `textbuf.MAC`, `textbuf.Uint8`, `textbuf.Uint16`, `textbuf.Uint32`
- Every file calling `textbuf.AppendUint`, `textbuf.AppendInt`, `textbuf.AppendAddr`, `textbuf.AppendPrefix`, `textbuf.AppendHex`, `textbuf.AppendMAC`
- Hook file `.claude/hooks/pretool-writeedit.py` (`_TEXTBUF_REF` constant)
- Rule file `ai/rules/no-sprintf-alloc.md`

### Architectural Verification
- [ ] No bypassed layers (N/A - rename only)
- [ ] No unintended coupling (N/A - rename only)
- [ ] No duplicated functionality (N/A - rename only)
- [ ] Zero-copy preserved where applicable (N/A - rename only)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Any caller using renamed append function | -> | textbuf.Uint(dst, v) | `TestUint` (renamed from TestAppendUint) |
| Any caller using renamed string function | -> | textbuf.StringUint(v) | `TestStringUint` (renamed from TestUint) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `textbuf.Uint(dst, 42)` called | Appends "42" to dst, returns extended slice (zero-alloc) |
| AC-2 | `textbuf.StringUint(42)` called | Returns "42" as string (one alloc, same behavior as old textbuf.Uint) |
| AC-3 | `textbuf.Int(dst, -7)` called | Appends "-7" to dst, returns extended slice |
| AC-4 | `textbuf.StringInt(-7)` called | Returns "-7" as string |
| AC-5 | `textbuf.Addr(dst, addr)` called | Appends IP string to dst |
| AC-6 | `textbuf.StringAddr(addr)` called | Returns IP as string |
| AC-7 | Same for Prefix, Hex, HexUpper, MAC | Both append and string variants renamed and working |
| AC-8 | `textbuf.StringUint8`, `StringUint16`, `StringUint32` | Typed variants renamed |
| AC-9 | All callers compile after rename | `go build ./...` succeeds with zero errors |
| AC-10 | All tests pass after rename | `make ze-unit-test` passes |
| AC-11 | Hook `_TEXTBUF_REF` updated | Reflects new names |
| AC-12 | Rule `no-sprintf-alloc.md` updated | All examples and tables use new names |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUint` | `textbuf_test.go` | Bare Uint appends to []byte (was TestAppendUint) | |
| `TestInt` | `textbuf_test.go` | Bare Int appends to []byte (was TestAppendInt) | |
| `TestAddr` | `textbuf_test.go` | Bare Addr appends to []byte (was TestAppendAddr) | |
| `TestHex` | `textbuf_test.go` | Bare Hex appends to []byte (was TestAppendHex) | |
| `TestPrefix` | `textbuf_test.go` | Bare Prefix appends to []byte (was TestAppendPrefix) | |
| `TestMAC` | `textbuf_test.go` | Bare MAC appends to []byte (was TestAppendMAC) | |
| `TestStringUint` | `textbuf_test.go` | StringUint returns string (was TestUint) | |
| `TestStringInt` | `textbuf_test.go` | StringInt returns string (was TestInt) | |
| `TestStringAddr` | `textbuf_test.go` | StringAddr returns string (was TestAddr) | |
| `TestStringHex` | `textbuf_test.go` | StringHex returns string (was TestHex) | |
| `TestStringHexUpper` | `textbuf_test.go` | StringHexUpper returns string (was TestHexUpper) | |
| `TestStringPrefix` | `textbuf_test.go` | StringPrefix returns string (was TestPrefix) | |
| `TestStringMAC` | `textbuf_test.go` | StringMAC returns string (was TestMAC) | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - no new numeric logic, pure rename.

### Functional Tests
N/A - internal library, no end-user-facing behavior change.

### Interop Tests
N/A - no protocol change.

### Future
- None deferred.

## Files to Modify

- `internal/core/textbuf/textbuf.go` - rename functions in library
- `internal/core/textbuf/textbuf_test.go` - rename test functions
- ~55 caller files across `internal/` (mechanical: old name -> new name)
- `.claude/hooks/pretool-writeedit.py` - update `_TEXTBUF_REF` constant
- `ai/rules/no-sprintf-alloc.md` - update all examples, tables, decision tree

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | N/A |
| CLI commands/flags | No | N/A |
| Functional test | No | N/A |
| Pipe completeness | No | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `ai/rules/no-sprintf-alloc.md` -- all examples and tables |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, etc.? | No | |
| 16 | Source anchors referencing changed files? | Yes | Grep docs/ for textbuf references |
| 17 | Existing docs show examples for this area? | Yes | `ai/rules/no-sprintf-alloc.md` examples |

## Files to Create
- None (pure rename)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify -- verify all callers found via grep |
| 3. Wiring phase | N/A (no new wiring) |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Library rename** -- rename functions in textbuf.go, tests in textbuf_test.go
   - Tests: all renamed tests in textbuf_test.go
   - Files: `internal/core/textbuf/textbuf.go`, `internal/core/textbuf/textbuf_test.go`
   - Verify: `go test ./internal/core/textbuf/` passes
   - Order within textbuf.go: rename Append* to bare names first (no conflict since old bare names still exist), then rename old bare names to String* (frees the bare names), then update internal cross-references

2. **Phase: Caller sweep** -- update all callers across internal/
   - Tests: existing tests must still pass after mechanical rename
   - Files: ~55 production files + their test files
   - Verify: `go build ./...` compiles, `make ze-unit-test` passes
   - Method: for each rename pair, grep + sed across internal/ (excluding textbuf.go itself)

3. **Phase: Rule and hook update** -- update documentation and hook
   - Files: `ai/rules/no-sprintf-alloc.md`, `.claude/hooks/pretool-writeedit.py`
   - Verify: hook syntax check passes (`python3 -c "import py_compile; ..."`), rule doc consistent

4. **Full verification** -- `make ze-verify`

### Rename Table

| Current Name | New Name | Type | Callsites |
|-------------|----------|------|-----------|
| `textbuf.Uint(v) string` | `textbuf.StringUint(v) string` | string-returning | 19 |
| `textbuf.Uint8(v) string` | `textbuf.StringUint8(v) string` | string-returning | 5 |
| `textbuf.Uint16(v) string` | `textbuf.StringUint16(v) string` | string-returning | 5 |
| `textbuf.Uint32(v) string` | `textbuf.StringUint32(v) string` | string-returning | 27 |
| `textbuf.Int(v) string` | `textbuf.StringInt(v) string` | string-returning | 29 |
| `textbuf.Addr(a) string` | `textbuf.StringAddr(a) string` | string-returning | 11 |
| `textbuf.Prefix(p) string` | `textbuf.StringPrefix(p) string` | string-returning | 0 |
| `textbuf.Hex(d) string` | `textbuf.StringHex(d) string` | string-returning | 11 |
| `textbuf.HexUpper(d) string` | `textbuf.StringHexUpper(d) string` | string-returning | 16 |
| `textbuf.MAC(m) string` | `textbuf.StringMAC(m) string` | string-returning | 0 |
| `textbuf.AppendUint(dst, v)` | `textbuf.Uint(dst, v)` | append | 1 |
| `textbuf.AppendInt(dst, v)` | `textbuf.Int(dst, v)` | append | 1 |
| `textbuf.AppendAddr(dst, a)` | `textbuf.Addr(dst, a)` | append | 1 |
| `textbuf.AppendPrefix(dst, p)` | `textbuf.Prefix(dst, p)` | append | 0 |
| `textbuf.AppendHex(dst, d)` | `textbuf.Hex(dst, d)` | append | 1 |
| `textbuf.AppendMAC(dst, m)` | `textbuf.MAC(dst, m)` | append | 0 |

**Total: 127 callsite renames across ~57 files**

### Not Renamed

| Function | Why |
|----------|-----|
| `b.Uint(v)`, `b.Int(v)`, etc. | Buffer chain methods already mean "append" -- correct |
| `textbuf.StrInt(prefix, v)` | Already encodes "Str" in name -- returns string, clear |
| `textbuf.StrUint`, `IntStr`, `UintStr`, `StrIntStr`, `StrUintStr` | Same: name says "string" |
| `textbuf.Join(items, sep)` | Returns string, name is clear (matches strings.Join convention) |
| `textbuf.HostPort(host, port)` | Returns string, name is clear |
| `textbuf.New()`, `textbuf.Get()` | Buffer constructors, no rename needed |

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every function in Rename Table has been renamed in library + all callers |
| Correctness | No caller uses old name (grep finds zero hits for old names) |
| Naming | New names follow the convention: bare = append, String prefix = allocates |
| Data flow | No semantic change -- every renamed function does exactly what it did before |
| Rule: no-sprintf-alloc | Rule doc updated: all examples, tables, decision tree use new names |
| Hook | `_TEXTBUF_REF` in pretool-writeedit.py uses new names |
| Buffer methods | No Buffer chain method was accidentally renamed |
| Combo helpers | No combo helper (StrInt, etc.) was accidentally renamed |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| No old Append names in code | `grep -rn 'textbuf\.AppendUint\|textbuf\.AppendInt\|textbuf\.AppendAddr\|textbuf\.AppendPrefix\|textbuf\.AppendHex\|textbuf\.AppendMAC' internal/ --include='*.go'` returns nothing |
| String-returning functions use String prefix | `grep -rn 'textbuf\.StringUint\|textbuf\.StringInt\|textbuf\.StringAddr' internal/ --include='*.go'` finds expected callsites |
| Bare names are append functions only | In textbuf.go: `func Uint(dst []byte, v uint64) []byte` signature |
| All tests pass | `make ze-unit-test` exit 0 |
| Clean build | `go build ./...` exit 0 |
| Rule doc updated | grep `StringUint` in `ai/rules/no-sprintf-alloc.md` finds hits |
| Hook updated | grep `StringUint` in `.claude/hooks/pretool-writeedit.py` finds hits |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | N/A - pure rename, no new inputs |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix the caller that still uses old name |
| Test fails wrong reason | Check test was renamed correctly |
| Name collision during rename | Rename in correct order: Append* -> bare first, then old bare -> String* |
| Lint failure | Fix inline |
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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `String` prefix for allocating versions | `Convert` prefix, `Format` prefix | `String` says exactly what you get (a string) and why it costs (string allocation). Parallels `.String()` on Buffer. `Format` would collide conceptually with banned `strconv.FormatUint`. `Convert` is vague about what it converts to. |
| Bare name for append version | Keep `Append` prefix | Matches Buffer chain method semantics (.Uint = append). In a zero-alloc-first library, the default operation should be the zero-alloc path. The `Append` prefix was inherited from Go stdlib convention where allocation is the default; textbuf inverts that priority. |
| Full rename in one pass | Two-phase with aliases | Clean break avoids a period where both names work and callers drift. The rename is mechanical and grep-verifiable. |
| Combo helpers (StrInt, etc.) keep their names | Rename to StringInt, etc. | The `Str` prefix is established across 113 callsites. Renaming would be a much larger sweep for marginal clarity. The prefix is unambiguous: `Str` = returns string. |

## Known Limitations
- Combo helpers (StrInt, etc.) use `Str` prefix while the new convention is `String`. This inconsistency is accepted because renaming 113 additional callsites provides minimal clarity gain and `Str` is already unambiguous in context.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered]

### Documentation Updates
- [Docs updated]

### Deviations from Plan
- [Differences from original plan]

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

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| API consistency: bare name = zero-alloc | unit test | TestUint appends to []byte |
| API consistency: String prefix = alloc | unit test | TestStringUint returns string |
| All callers updated | build | `go build ./...` succeeds |
| No behavioral change | test suite | `make ze-unit-test` passes |

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

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Documentation Update Checklist answered
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

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-textbuf-api-rename.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-textbuf-api-rename.md`

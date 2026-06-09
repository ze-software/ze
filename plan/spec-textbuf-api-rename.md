# Spec: textbuf API rename for allocation-first consistency

| Field | Value |
|-------|-------|
| Status | implemented |
| Depends | - |
| Phase | 4/4 |
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
  -> Constraint: the decision tree, replacement tables, examples, and self-check all reference the current names; every instance must be updated
  -> Constraint: `.claude/hooks/pretool-writeedit.py` references current names in `_TEXTBUF_REF` and in replacement examples; update both
- [ ] `ai/rules/memory-architecture.md` - data lifecycle, caller-owned buffers, pool strategy
  -> Constraint: examples and mistake tables reference current names; update every textbuf example so future agents do not copy stale APIs
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
- Current source has string-returning standalone callsites and existing `Append*` callsites across Go, rules, hooks, and tests; implementation must rediscover the exact set with LSP/search before editing
- Current source has `StringHexUpper` behavior under the old bare `HexUpper` name but no append-style `HexUpper(dst, data)` counterpart; add that append function while retaining the string-returning behavior under `StringHexUpper`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/textbuf/textbuf.go` - standalone + Buffer + Append functions
  -> Constraint: Buffer chain methods (.Uint, .Int, .Addr, etc.) are NOT renamed; they already mean "append"
  -> Constraint: combo helpers (StrInt, StrUint, etc.) are NOT renamed; they already encode "Str" in name
  -> Constraint: Join, HostPort are NOT renamed; their string-returning behavior is retained as an explicit exception because the names match established conventions
  -> Constraint: existing string-returning `HexUpper(data)` behavior must be retained as `StringHexUpper(data)`
  -> Decision: rename 10 existing string-returning standalone functions, rename 6 existing Append functions to bare append names, and add the missing append-style `HexUpper(dst, data)` counterpart

**Behavior and features to preserve:**
- No formatting capability is removed. Every old behavior remains reachable under a new or retained name.
- All textbuf.Buffer chain methods unchanged (already correct: .Uint = append)
- All combo helpers unchanged (StrInt, StrUint, IntStr, UintStr, StrIntStr, StrUintStr)
- textbuf.Join, textbuf.HostPort unchanged (string-returning, explicit exception)
- All existing Append-into-buffer semantics preserved under bare standalone names
- All existing string-returning semantics preserved under `String*` names
- Old identifiers are intentionally not kept as aliases; this is a clean API cutover, not feature removal
- Zero runtime behavior change for existing callers after callsites are updated

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
- Every Go caller of `textbuf.Uint`, `textbuf.Int`, `textbuf.Addr`, `textbuf.Prefix`, `textbuf.Hex`, `textbuf.HexUpper`, `textbuf.MAC`, `textbuf.Uint8`, `textbuf.Uint16`, `textbuf.Uint32`
- Every Go caller of `textbuf.AppendUint`, `textbuf.AppendInt`, `textbuf.AppendAddr`, `textbuf.AppendPrefix`, `textbuf.AppendHex`, `textbuf.AppendMAC`
- New tests and any future callers for `textbuf.HexUpper(dst, data)` append behavior
- Hook file `.claude/hooks/pretool-writeedit.py` (`_TEXTBUF_REF` constant and replacement examples)
- Rule files `ai/rules/no-sprintf-alloc.md`, `ai/rules/memory-architecture.md`

### Architectural Verification
- [ ] No bypassed layers (N/A - rename only)
- [ ] No unintended coupling (N/A - rename only)
- [ ] No duplicated functionality (N/A - rename only)
- [ ] Zero-copy preserved where applicable (N/A - rename only)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Any caller using renamed append function | -> | `textbuf.Uint(dst, v)` | `TestUint` (renamed from TestAppendUint) |
| Any caller using renamed string function | -> | `textbuf.StringUint(v)` | `TestStringUint` (renamed from TestUint) |
| Caller needing uppercase hex append | -> | `textbuf.HexUpper(dst, data)` | `TestHexUpper` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `textbuf.Uint(dst, 42)` called | Appends "42" to dst, returns extended slice, zero allocations when dst has capacity |
| AC-2 | `textbuf.StringUint(42)` called | Returns "42" as string, same behavior as old textbuf.Uint |
| AC-3 | `textbuf.Int(dst, -7)` called | Appends "-7" to dst, returns extended slice |
| AC-4 | `textbuf.StringInt(-7)` called | Returns "-7" as string |
| AC-5 | `textbuf.Addr(dst, addr)` called | Appends IP string to dst |
| AC-6 | `textbuf.StringAddr(addr)` called | Returns IP as string |
| AC-7 | Prefix, Hex, HexUpper, MAC append variants called | Bare functions append into dst; `HexUpper(dst, data)` is added because no append counterpart exists today |
| AC-8 | Prefix, Hex, HexUpper, MAC string variants called | `StringPrefix`, `StringHex`, `StringHexUpper`, `StringMAC` return the same strings as the old bare functions |
| AC-9 | `textbuf.StringUint8`, `StringUint16`, `StringUint32` | Typed variants renamed and preserve current output |
| AC-10 | All Go callers compile after rename | Project build targets for `ze`, `chaos`, `test`, `analyze`, and `perf` succeed |
| AC-11 | All tests pass after rename | `make ze-unit-test` passes |
| AC-12 | Hook updated | `_TEXTBUF_REF` and replacement examples reflect new names |
| AC-13 | Rule docs updated | `ai/rules/no-sprintf-alloc.md` and `ai/rules/memory-architecture.md` use new names in all examples and tables |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUint` | `textbuf_test.go` | Bare Uint appends to []byte (was TestAppendUint) | |
| `TestInt` | `textbuf_test.go` | Bare Int appends to []byte (was TestAppendInt) | |
| `TestAddr` | `textbuf_test.go` | Bare Addr appends to []byte (was TestAppendAddr) | |
| `TestHex` | `textbuf_test.go` | Bare Hex appends to []byte (was TestAppendHex) | |
| `TestHexUpper` | `textbuf_test.go` | Bare HexUpper appends uppercase hex to []byte (new append counterpart) | |
| `TestPrefix` | `textbuf_test.go` | Bare Prefix appends to []byte (was TestAppendPrefix) | |
| `TestMAC` | `textbuf_test.go` | Bare MAC appends to []byte (was TestAppendMAC) | |
| `TestBareAppendEmptyDst` | `textbuf_test.go` | Bare append functions work with nil dst | |
| `TestBareAppendNoAllocWithCapacity` | `textbuf_test.go` | Bare append functions allocate zero when dst has enough capacity | |
| `TestStringUint` | `textbuf_test.go` | StringUint returns string (was TestUint) | |
| `TestStringUint8` | `textbuf_test.go` | StringUint8 returns string (was TestUint8) | |
| `TestStringUint16` | `textbuf_test.go` | StringUint16 returns string (was TestUint16) | |
| `TestStringUint32` | `textbuf_test.go` | StringUint32 returns string (was TestUint32) | |
| `TestStringInt` | `textbuf_test.go` | StringInt returns string (was TestInt) | |
| `TestStringAddr` | `textbuf_test.go` | StringAddr returns string (was TestAddr) | |
| `TestStringHex` | `textbuf_test.go` | StringHex returns string (was TestHex) | |
| `TestStringHexLargeData` | `textbuf_test.go` | StringHex large-data behavior preserved (was TestHexLargeData) | |
| `TestStringHexUpper` | `textbuf_test.go` | StringHexUpper returns string (was TestHexUpper) | |
| `TestStringHexUpperLarge` | `textbuf_test.go` | StringHexUpper large-data behavior preserved (was TestHexUpperLarge) | |
| `TestStringHexBoundary` | `textbuf_test.go` | StringHex stack/heap boundary preserved (was TestHexBoundary) | |
| `TestStringHexUpperBoundary` | `textbuf_test.go` | StringHexUpper stack/heap boundary preserved (was TestHexUpperBoundary) | |
| `TestStringHexAllocBoundary` | `textbuf_test.go` | StringHex allocation boundary preserved (was TestHexAllocBoundary) | |
| `TestStringHexUpperAllocBoundary` | `textbuf_test.go` | StringHexUpper allocation boundary preserved (was TestHexUpperAllocBoundary) | |
| `TestStringPrefix` | `textbuf_test.go` | StringPrefix returns string (was TestPrefix) | |
| `TestStringMAC` | `textbuf_test.go` | StringMAC returns string (was TestMAC) | |

### Boundary Tests (MANDATORY for numeric and length-sensitive inputs)
Existing boundary tests are renamed, not deleted. Add `TestBareAppendNoAllocWithCapacity` for append capacity behavior and keep the Hex/HexUpper stack-vs-heap boundary tests under their new `String*` names.

### Functional Tests
N/A - internal library, no end-user-facing behavior change.

### Interop Tests
N/A - no protocol change.

### Future
- None deferred.

## Files to Modify

- `internal/core/textbuf/textbuf.go` - rename functions in library
- `internal/core/textbuf/textbuf_test.go` - rename test functions
- All Go caller files discovered by LSP references/search (mechanical: old string names -> `String*`, old Append names -> bare append)
- `internal/component/gnmi/set.go` - special case: `string(textbuf.AppendInt(nil, v.IntVal))` becomes `textbuf.StringInt(v.IntVal)`, not `string(textbuf.Int(nil, ...))`
- `.claude/hooks/pretool-writeedit.py` - update `_TEXTBUF_REF` constant and replacement examples
- `ai/rules/no-sprintf-alloc.md` - update all examples, tables, decision tree
- `ai/rules/memory-architecture.md` - update AppendTo examples and mistake table

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
| 12 | Internal architecture changed? | Yes | `ai/rules/no-sprintf-alloc.md`, `ai/rules/memory-architecture.md` -- all examples and tables |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, etc.? | No | |
| 16 | Source anchors referencing changed files? | Yes | Search `docs/`, `ai/`, and hooks for textbuf references |
| 17 | Existing docs show examples for this area? | Yes | textbuf examples in rule docs and hook messages |

## Files to Create
- None committed. Temporary codemod/verifier helpers under `tmp/` are allowed during implementation and must not be shipped unless a later decision promotes them to reusable tooling. `tmp/go.mod` is a sentinel that keeps temporary Go helpers out of the main module during `go list ./...`; never delete, rename, or overwrite it.

## Implementation Steps
### Programmatic Implementation Design

This work SHOULD be done programmatically. Manual editing is only for reviewing generated diffs and fixing cases the verifier rejects.

| Step | Tooling | Purpose | Safety property |
|------|---------|---------|-----------------|
| Discovery manifest | LSP references plus `search` | List Go callsites, rule docs, and hook text before edits | Prevents relying on stale call counts |
| Go AST codemod | One-off temporary Go or Python AST tool under `tmp/` | Rewrite selector calls only when the import resolves to `internal/core/textbuf` | Avoids changing unrelated identifiers or Buffer methods |
| Library edit | AST codemod plus manual review | Rename declarations, update typed wrappers, add append-style HexUpper | Avoids same-package name collisions by doing string declarations first |
| Special-case rewrite | AST codemod | Replace string casts around nil-dst append calls with `String*` calls | Preserves string-returning intent and avoids hiding allocation behind append names |
| Doc/hook rewrite | Ordered literal replacements with diff preview | Update rule examples and hook guidance | Human-reviewed because prose has context |
| AST verifier | Temporary verifier under `tmp/` | Reject stale or ambiguous Go call patterns | Proves bare textbuf calls in Go are append arity only |
| Temporary Go module guard | Existing `tmp/go.mod` | Keep throwaway Go helpers from polluting root `go list ./...` | Preserves verification gates while allowing scripts under `tmp/` |

#### Discovery manifest

Before editing, generate a manifest with:

| Category | Include | Reject if |
|----------|---------|-----------|
| Go files importing textbuf | Any `.go` file importing `codeberg.org/thomas-mangin/ze/internal/core/textbuf` | Dot-import or renamed import not handled by the codemod |
| String-returning calls | One-argument calls to old bare names | Any call shape cannot be classified |
| Append calls | Calls to old `Append*` names | Any nil-dst append is not classified as string intent or append intent |
| Documentation and hook hits | `ai/`, `.claude/hooks/`, and `docs/` textbuf references | Old names appear outside an explicitly allowed historical context |

The manifest is implementation evidence. It belongs in the spec audit or learned summary, not in committed source unless useful beyond this migration.

#### Go AST codemod rules

| Match | Rewrite |
|-------|---------|
| textbuf import under its package name or explicit alias | Track the local import name for selector matching |
| One-argument old string selector | Rename selector to `String*` |
| Old `Append*` selector | Rename selector to the corresponding bare append name |
| String cast around nil-dst old append selector | Replace with corresponding `String*` selector |
| Buffer method selectors | Leave unchanged |
| `Join`, `HostPort`, constructors, combo helpers | Leave unchanged |
| Any unclassified selector on the textbuf import | Fail the codemod, do not guess |

The codemod must run with preview output first. Apply only after the preview shows the expected categories.

#### Temporary Go helper guard

When creating Go helpers under `tmp/`, keep `tmp/go.mod` in place. It is not clutter. It prevents root-module `go list ./...` and `make ze-unit-test` from traversing unrelated temporary Go files and downloaded module caches under `tmp/`. If a helper complains about modules, create a subdirectory under `tmp/` or run the helper by path from the repo root. Do not remove `tmp/go.mod`.

#### Library transformation order

| Order | Change | Reason |
|-------|--------|--------|
| 1 | Rename string-returning declarations to `String*` | Frees bare names without collision |
| 2 | Update typed string wrappers to call `StringUint` | Prevents wrappers from accidentally targeting append functions |
| 3 | Rename existing `Append*` declarations to bare names | Establishes append-first API |
| 4 | Add append-style `HexUpper(dst, data)` | Completes the family without removing `StringHexUpper` |
| 5 | Run gofmt on modified Go files | Mechanical formatting only |

#### Programmatic verifier

After edits, run a verifier that parses Go files and checks:

| Invariant | Verification |
|-----------|--------------|
| No old `Append*` selectors | Import-resolved selector scan finds none |
| No old typed bare string selectors | `Uint8`, `Uint16`, `Uint32` selectors on textbuf import find none |
| Bare renamed family is append-only | `Uint`, `Int`, `Addr`, `Prefix`, `Hex`, `HexUpper`, `MAC` calls have append arity |
| String family is string-only | `String*` calls have the old string arity |
| Buffer methods unchanged | Method selectors on non-package receivers are ignored by the package-selector scan |
| Exceptions preserved | `Join`, `HostPort`, combo helpers, `New`, and `Get` remain allowed |

The verifier should fail closed and print file:line plus the selector and argument count for each violation.


### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify -- verify all callers found via LSP references plus search for docs/hooks |
| 3. Wiring phase | N/A (no new user entry point; new `HexUpper(dst, data)` is covered by unit test) |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | project build targets, `make ze-unit-test`, then `make ze-verify` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Library rename** -- rename functions in textbuf.go, tests in textbuf_test.go
   - Tests: all renamed tests in textbuf_test.go, including HexUpper append and no-alloc append capacity coverage
   - Files: `internal/core/textbuf/textbuf.go`, `internal/core/textbuf/textbuf_test.go`
   - Verify: `go test ./internal/core/textbuf/` passes
   - Order within textbuf.go: rename existing string-returning functions to `String*` first, update typed wrappers and tests, then rename existing `Append*` functions to bare append names, then add the new append-style `HexUpper(dst, data)` function. This avoids same-package name collisions.

2. **Phase: Caller sweep** -- update all Go callers
   - Tests: existing tests must still pass after mechanical rename
   - Files: all production and test files found by LSP references/search
   - Verify: project build targets compile, `make ze-unit-test` passes
   - Method: use LSP references/rename for exported Go symbols where available. Use `search` to find any remaining old names. Do not use blind text replacement for Go symbols because old and new names intentionally swap meanings.
   - Special case: replace `string(textbuf.AppendInt(nil, v))` with `textbuf.StringInt(v)`.

3. **Phase: Rule and hook update** -- update documentation and hook
   - Files: `ai/rules/no-sprintf-alloc.md`, `ai/rules/memory-architecture.md`, `.claude/hooks/pretool-writeedit.py`
   - Verify: hook syntax check passes (`python3 -m py_compile .claude/hooks/pretool-writeedit.py`), rule docs have no stale examples

4. **Full verification** -- `make ze-verify`

### Rename Table

| Current Name | New Name | Type | Notes |
|-------------|----------|------|-------|
| `textbuf.Uint(v) string` | `textbuf.StringUint(v) string` | string-returning | existing behavior retained |
| `textbuf.Uint8(v) string` | `textbuf.StringUint8(v) string` | string-returning | existing behavior retained |
| `textbuf.Uint16(v) string` | `textbuf.StringUint16(v) string` | string-returning | existing behavior retained |
| `textbuf.Uint32(v) string` | `textbuf.StringUint32(v) string` | string-returning | existing behavior retained |
| `textbuf.Int(v) string` | `textbuf.StringInt(v) string` | string-returning | existing behavior retained |
| `textbuf.Addr(a) string` | `textbuf.StringAddr(a) string` | string-returning | existing behavior retained |
| `textbuf.Prefix(p) string` | `textbuf.StringPrefix(p) string` | string-returning | existing behavior retained |
| `textbuf.Hex(d) string` | `textbuf.StringHex(d) string` | string-returning | existing behavior retained |
| `textbuf.HexUpper(d) string` | `textbuf.StringHexUpper(d) string` | string-returning | existing behavior retained |
| `textbuf.MAC(m) string` | `textbuf.StringMAC(m) string` | string-returning | existing behavior retained |
| `textbuf.AppendUint(dst, v)` | `textbuf.Uint(dst, v)` | append | existing behavior retained |
| `textbuf.AppendInt(dst, v)` | `textbuf.Int(dst, v)` | append | existing behavior retained |
| `textbuf.AppendAddr(dst, a)` | `textbuf.Addr(dst, a)` | append | existing behavior retained |
| `textbuf.AppendPrefix(dst, p)` | `textbuf.Prefix(dst, p)` | append | existing behavior retained |
| `textbuf.AppendHex(dst, d)` | `textbuf.Hex(dst, d)` | append | existing behavior retained |
| `textbuf.AppendMAC(dst, m)` | `textbuf.MAC(dst, m)` | append | existing behavior retained |
| N/A | `textbuf.HexUpper(dst, d)` | append | new counterpart so uppercase hex has both append and string variants |

Implementation must rediscover exact callsites with LSP/search before editing. Counts in this plan are intentionally not treated as authoritative.

### Not Renamed

| Function | Why |
|----------|-----|
| `b.Uint(v)`, `b.Int(v)`, etc. | Buffer chain methods already mean "append" -- correct |
| `textbuf.StrInt(prefix, v)` | Already encodes "Str" in name -- returns string, clear |
| `textbuf.StrUint`, `IntStr`, `UintStr`, `StrIntStr`, `StrUintStr` | Same: name says "string" |
| `textbuf.Join(items, sep)` | Returns string and intentionally mirrors `strings.Join`; append use stays available as `b.Join(items, sep)` |
| `textbuf.HostPort(host, port)` | Returns string and intentionally mirrors `net.JoinHostPort`; append use stays available as `b.HostPort(host, port)` |
| `textbuf.New()`, `textbuf.Get()` | Buffer constructors, no rename needed |

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every function in Rename Table has been renamed or added in library + all callers |
| No feature removal | Every old string behavior has a `String*` replacement; every old Append behavior has a bare replacement; Join, HostPort, Buffer methods, and combo helpers remain |
| Correctness | No caller uses old name (search finds zero stale hits outside historical logs) |
| Naming | New names follow the convention for this family: bare = append, String prefix = allocates; Join/HostPort are documented exceptions |
| Data flow | No semantic change for existing behavior |
| Rule: no-sprintf-alloc | Rule doc updated: all examples, tables, decision tree use new names |
| Rule: memory-architecture | AppendTo examples and mistake table use new names |
| Hook | `_TEXTBUF_REF` and replacement examples in pretool-writeedit.py use new names |
| Buffer methods | No Buffer chain method was accidentally renamed |
| Combo helpers | No combo helper (StrInt, etc.) was accidentally renamed |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| No old Append names in Go/rules/hooks | `search` for `textbuf\.Append(Uint|Int|Addr|Prefix|Hex|MAC)\(` returns no active source/doc/hook hits |
| String-returning functions use String prefix | LSP/search verifies old bare string callsites now call `textbuf.String*` |
| Bare names are append functions only for renamed family | In textbuf.go: `func Uint(dst []byte, v uint64) []byte` and peers have append signatures |
| Uppercase hex has both variants | `func HexUpper(dst []byte, data []byte) []byte` and `func StringHexUpper(data []byte) string` both exist and are tested |
| All tests pass | `make ze-unit-test` exit 0 |
| Project build targets pass | `make ze chaos test analyze perf` exit 0 |
| Full verification passes | `make ze-verify` exit 0 |
| Rule docs updated | `search` for new names in `ai/rules/no-sprintf-alloc.md` and `ai/rules/memory-architecture.md` finds updated examples |
| Hook updated | `search` for new names in `.claude/hooks/pretool-writeedit.py` finds `_TEXTBUF_REF` and replacement examples |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | N/A - pure rename, no new inputs |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix the caller that still uses old name |
| Test fails wrong reason | Check test was renamed correctly |
| Name collision during rename | Rename in correct order: old string functions -> `String*` first, then existing `Append*` -> bare append names |
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
| Full rename in one pass | Two-phase with aliases | Clean break avoids a period where both names work and callers drift. The rename is mechanical and search-verifiable. |
| Combo helpers (StrInt, etc.) keep their names | Rename to StringInt, etc. | The `Str` prefix is established across 113 callsites. Renaming would be a much larger sweep for marginal clarity. The prefix is unambiguous: `Str` = returns string. |
| Add append-style `HexUpper(dst, data)` | Rename only existing functions and leave uppercase hex string-only | AC-7 promises both append and string variants for HexUpper. Adding the append counterpart preserves the existing string feature and completes the zero-alloc family without removing behavior. |

## Known Limitations
- Combo helpers (StrInt, etc.) use `Str` prefix while the new convention is `String`. This inconsistency is accepted because renaming 113 additional callsites provides minimal clarity gain and `Str` is already unambiguous in context.
- `textbuf.Join` and `textbuf.HostPort` remain string-returning bare names. They are explicit exceptions because the names mirror established Go conventions; append variants remain available as Buffer methods.

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
| API consistency: bare name = zero-alloc append | unit test | TestUint/TestHexUpper append to []byte with no allocation when capacity is sufficient |
| API consistency: String prefix = alloc | unit test | TestStringUint returns string |
| All callers updated | build | project build targets for `ze`, `chaos`, `test`, `analyze`, and `perf` succeed |
| No behavioral feature removed | unit tests + source check | String* tests preserve old outputs; Join/HostPort/Buffer/combo helpers remain |
| No behavioral regression | test suite | `make ze-unit-test` and `make ze-verify` pass |

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

### Files Exist (find/read)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (search/test)
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
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-verify` passes
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

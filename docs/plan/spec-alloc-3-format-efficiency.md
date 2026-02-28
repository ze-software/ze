# Spec: Allocation Reduction — Format Efficiency

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/plan/spec-alloc-0-umbrella.md` — umbrella tracker
3. `internal/plugins/bgp/format/text.go` — `formatFilterResultText`, `formatAttributeText`, `writeNLRIList`
4. `internal/plugins/bgp/nlri/inet.go` — `INET.String()`, `INET.Key()`

## Task

Replace `fmt.Fprintf`, `netip.Prefix.String()`, and `netip.Addr.String()` with zero-alloc alternatives (`strconv.AppendInt`, `netip.Prefix.AppendTo`, `netip.Addr.AppendTo`) in the text formatting hot path. The `netip.Addr.string4` function is the #1 object allocator in the profile (30.2M objects, 461 MB).

Parent: `spec-alloc-0-umbrella.md` (child 3).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/text-format.md` — current event text format
  → Constraint: text format must remain byte-identical — these changes are internal to the formatter
  → Decision: short-form keywords (`next`, `path`, `pref`, `s-com`, `l-com`, `x-com`) are package-level constants
- [ ] `.claude/rules/buffer-first.md` — buffer-first encoding patterns
  → Constraint: encoding code should avoid `append(buf, ...)` where pre-computed size + offset writes are possible
  → Decision: `AppendTo` returns a new slice if input is too small — safe to use with `scratch[:0]`

**Key insights:**
- `netip.Prefix.AppendTo(b []byte) []byte` writes prefix text into caller-provided buffer — zero-alloc with stack-local scratch
- `netip.Addr.AppendTo(b []byte) []byte` same pattern for IP addresses
- `strconv.AppendInt(dst []byte, i int64, base int) []byte` writes integer text into caller-provided buffer
- `strings.Builder` supports `Write([]byte)` in addition to `WriteString(string)` — can write `AppendTo` results directly
- `fmt.Fprintf` boxes arguments to `any` interface, which causes heap escapes for small values

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/bgp/format/text.go` — line 629: `formatFilterResultText` builds text via `strings.Builder`:
  - Line 633: `fmt.Fprintf(&sb, "peer %s asn %d %s update %d", peer.Address, peer.PeerAS, direction, msgID)` — boxes `peer.PeerAS` (uint32) and `msgID` (uint64) to `any`, heap alloc per call
  - Line 650: `fam.NextHop.String()` — calls `netip.Addr.String()` → `string4` → heap alloc (30.2M objects)
  - Line 681: `nlris[0].String()` — for INET: `"prefix " + i.prefix.String()` → two allocs (string concat + prefix.String)
  - Line 685: `k.Key()` → `i.prefix.String()` → heap alloc per NLRI
  → Constraint: output must be byte-identical to current format — tests validate exact text
- [ ] `internal/plugins/bgp/format/text.go` — line 711: `formatAttributeText` switch on attribute code:
  - Line 732: `fmt.Fprintf(sb, "%d", asn)` per ASN in AS_PATH — boxes uint32 to `any`
  - Line 742: `nh.Addr.String()` for NEXT_HOP — heap alloc
  - Line 749: `fmt.Fprintf(sb, "%d", med.Value())` for MED — boxes uint32
  - Line 757: `fmt.Fprintf(sb, "%d", lp.Value())` for LOCAL_PREF — boxes uint32
  → Constraint: `strings.Builder` accepts `Write([]byte)` — can write scratch buffer directly
- [ ] `internal/plugins/bgp/nlri/inet.go` — line 177: `Key() string` returns `i.prefix.String()`. Line 183: `String() string` returns `"prefix " + i.prefix.String()`. Both call `netip.Prefix.String()` which calls `netip.Addr.String()` internally.
  → Decision: add `AppendKey(b []byte) []byte` and `AppendString(b []byte) []byte` methods as zero-alloc alternatives
  → Constraint: `String()` and `Key()` must remain for non-hot-path callers (map keys, logging, etc.)

**Behavior to preserve:**
- Exact text output format (byte-identical to current)
- `INET.String()` and `INET.Key()` remain available (used outside hot path)
- `formatAttributeText` switch structure unchanged
- `writeNLRIList` comma vs space logic unchanged

**Behavior to change:**
- `fmt.Fprintf` in header and numeric attributes → `strconv.AppendInt` + `sb.Write`
- `netip.Addr.String()` → `netip.Addr.AppendTo(scratch[:0])` + `sb.Write`
- `netip.Prefix.String()` → `netip.Prefix.AppendTo(scratch[:0])` + `sb.Write`
- `INET.String()` / `INET.Key()` in hot path → `AppendString` / `AppendKey` variants

## Data Flow (MANDATORY)

### Entry Point
- `formatFilterResultText` called from `formatFromFilterResult` (line 156) for text-encoded UPDATE events
- Called once per UPDATE per distinct format key in subscription matching

### Transformation Path
1. `formatFilterResultText` creates `strings.Builder`
2. Header: `fmt.Fprintf` for peer info → **change to** `sb.WriteString` + `strconv.AppendInt` into `[20]byte` scratch
3. Attributes: `formatAttributesText` iterates `result.Attributes`, calls `formatAttributeText` per code
4. AS_PATH: `fmt.Fprintf(sb, "%d", asn)` per ASN → **change to** `strconv.AppendInt` into scratch + `sb.Write`
5. NEXT_HOP: `nh.Addr.String()` → **change to** `nh.Addr.AppendTo(scratch[:0])` + `sb.Write`
6. Announced NLRIs: `fam.NextHop.String()` → **change to** `fam.NextHop.AppendTo(scratch[:0])` + `sb.Write`
7. `writeNLRIList`: `n.String()` / `k.Key()` per NLRI → **change to** `AppendString` / `AppendKey` with scratch
8. `sb.String()` returns final text (this allocation is unavoidable — result must be a string)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| NLRI interface → text | `NLRI.String()` or new `AppendString` method | [ ] |
| Attribute → text | `formatAttributeText` writes into Builder | [ ] |

### Integration Points
- `writeNLRIList` uses `NLRI` interface — adding `AppendString`/`AppendKey` requires interface extension or type assertion
- `formatAttributeText` receives `attribute.Attribute` interface — switch on concrete types already present
- INET NLRI implements `keyer` interface (checked via type assertion at line 679)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `FormatMessage` with INET UPDATE | → | `writeNLRIList` uses `AppendKey` | `TestFormatTextINETNoStringAlloc` |
| `FormatMessage` with AS_PATH | → | `formatAttributeText` uses `strconv.AppendInt` | `TestFormatTextASPathNoFmtSprintf` |
| `FormatMessage` with next-hop | → | `formatFilterResultText` uses `AppendTo` for NH | `TestFormatTextNextHopNoStringAlloc` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `formatFilterResultText` with INET UPDATE | Output byte-identical to current format |
| AC-2 | `formatFilterResultText` with INET UPDATE | No `netip.Addr.String()` calls — verified via `testing.AllocsPerRun` |
| AC-3 | `formatAttributeText` with AS_PATH attribute | No `fmt.Fprintf` calls — uses `strconv.AppendInt` |
| AC-4 | `INET.AppendKey(scratch[:0])` | Returns same bytes as `INET.Key()` |
| AC-5 | `INET.AppendString(scratch[:0])` | Returns same bytes as `INET.String()` |
| AC-6 | Header formatting | No `fmt.Fprintf` — uses `WriteString` + `strconv.AppendInt` |
| AC-7 | `make ze-verify` | Passes with zero regressions |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestINETAppendKey` | `internal/plugins/bgp/nlri/inet_test.go` | AC-4: `AppendKey` matches `Key()` for IPv4 and IPv6 prefixes | |
| `TestINETAppendString` | `internal/plugins/bgp/nlri/inet_test.go` | AC-5: `AppendString` matches `String()` | |
| `TestFormatTextINETNoStringAlloc` | `internal/plugins/bgp/format/text_test.go` | AC-2: `testing.AllocsPerRun` shows reduced allocs vs baseline | |
| `TestFormatTextASPathNoFmtSprintf` | `internal/plugins/bgp/format/text_test.go` | AC-3: AS_PATH formatting uses no `fmt` calls | |
| `TestFormatTextOutputIdentical` | `internal/plugins/bgp/format/text_test.go` | AC-1: output matches golden strings from existing tests | |
| `TestFormatTextHeaderNoFprintf` | `internal/plugins/bgp/format/text_test.go` | AC-6: header line matches expected format | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| AS number | 0-4294967295 | 4294967295 | N/A | N/A |
| Prefix bits IPv4 | 0-32 | 32 | N/A | 33 |
| Prefix bits IPv6 | 0-128 | 128 | N/A | 129 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing `test/plugin/*.ci` | `test/plugin/` | UPDATE text format unchanged — regression check | |
| Existing `test/encode/*.ci` | `test/encode/` | Encoding output unchanged — regression check | |

## Files to Modify
- `internal/plugins/bgp/format/text.go` — replace `fmt.Fprintf` with `strconv.AppendInt` + `sb.Write`; replace `String()` calls with `AppendTo` + `sb.Write`
- `internal/plugins/bgp/nlri/inet.go` — add `AppendKey(b []byte) []byte` and `AppendString(b []byte) []byte` methods

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | |
| CLI commands/flags | No | |
| API commands doc | No | |
| Plugin SDK docs | No | |
| Functional test for new RPC/API | No — existing tests cover regression | |

## Files to Create
- No new files — changes are in existing files

## Implementation Steps

1. **Write `TestINETAppendKey` and `TestINETAppendString`** — verify output matches `Key()` and `String()` for IPv4/IPv6 prefixes with various prefix lengths
2. **Run tests** → Verify FAIL (methods don't exist yet)
3. **Implement `AppendKey` and `AppendString`** on `INET`:
   - `AppendKey(b []byte) []byte`: `return i.prefix.AppendTo(b)`
   - `AppendString(b []byte) []byte`: `b = append(b, "prefix "...); return i.prefix.AppendTo(b)`
4. **Run tests** → Verify PASS
5. **Write `TestFormatTextOutputIdentical`** — format a known UPDATE, compare output string to golden value
6. **Run test** → Verify PASS (baseline — current code produces correct output)
7. **Modify `formatFilterResultText`**:
   - Replace `fmt.Fprintf(&sb, "peer %s asn %d %s update %d", ...)` with `sb.WriteString("peer ")` + `sb.WriteString(peer.Address.String())` + `sb.WriteString(" asn ")` + scratch `strconv.AppendUint` + `sb.Write` + etc.
   - Replace `fam.NextHop.String()` with `fam.NextHop.AppendTo(scratch[:0])` + `sb.Write`
8. **Modify `writeNLRIList`**:
   - Type-assert INET NLRIs to use `AppendKey`/`AppendString` with scratch buffer
   - Non-INET NLRIs: keep `n.String()` (not hot path)
9. **Modify `formatAttributeText`**:
   - Replace `fmt.Fprintf(sb, "%d", asn)` with `strconv.AppendUint(scratch[:0], uint64(asn), 10)` + `sb.Write`
   - Replace `nh.Addr.String()` with `nh.Addr.AppendTo(scratch[:0])` + `sb.Write`
   - Replace MED/LocalPref `fmt.Fprintf` with `strconv.AppendUint` + `sb.Write`
10. **Run `TestFormatTextOutputIdentical`** → Verify PASS (output unchanged)
11. **Write allocation benchmark** — `testing.AllocsPerRun` comparing before/after
12. **Run `make ze-verify`** → Verify zero regressions
13. **Critical Review** → All 6 checks from `rules/quality.md`

### Failure Routing
| Failure | Route To |
|---------|----------|
| Output differs from golden | Step 7-9 — check spacing, comma, keyword differences |
| `AppendTo` returns different format than `String()` | Step 3 — verify `netip.Prefix.AppendTo` behavior matches `String()` |
| Alloc count unchanged | Step 7-9 — check if scratch buffer escapes to heap (use `go build -gcflags='-m'`) |

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

## Implementation Summary

### What Was Implemented
- (to be filled)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

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

### Goal Gates
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates
- [ ] `make ze-lint` passes
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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Spec moved to `docs/plan/done/NNN-alloc-3-format-efficiency.md`
- [ ] Spec included in commit

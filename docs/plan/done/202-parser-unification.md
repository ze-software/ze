# Spec: Parser Unification

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/config/routeattr.go` - config value parsers
4. `internal/plugin/route.go` - API value parsers

## Task

Reduce code duplication between API and config parsing for BGP attribute values by extracting shared value parsers to `internal/parse/`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - overall architecture
- [ ] `docs/architecture/wire/attributes.md` - attribute encoding

### RFC Summaries
- [ ] `rfc/short/rfc4271.md` - BGP-4 (ORIGIN, AS_PATH)
- [ ] `rfc/short/rfc1997.md` - Communities
- [ ] `rfc/short/rfc4360.md` - Extended Communities

**Key insights:**
- ORIGIN values: 0=IGP, 1=EGP, 2=INCOMPLETE (RFC 4271 Section 5.1.1)
- Communities are 4-byte values: high 16 bits = ASN, low 16 bits = value
- Extended communities are 8-byte values with type encoding

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/config/routeattr.go` - Config parser implementations
- [x] `internal/plugin/route.go` - API parser implementations

**Behavior to preserve:**

| Parser | Config (`routeattr.go`) | API (`route.go`) | Difference |
|--------|-------------------------|------------------|------------|
| Origin | `ParseOrigin()` line 25: accepts `""` as IGP | `parseOrigin()` line 199: accepts `"?"` for incomplete | Empty string handling differs |
| AS-Path | `ParseASPath()` line 757: brackets + commas | `parseASPath()` line 390: brackets + commas | Similar |
| Community | `ParseCommunity()` line 60: well-known names | `parseCommunity()` line 525: well-known names | Similar |
| Extended Community | `parseOneExtCommunity()` line 228: 30+ formats | `parseExtendedCommunity()` line 708: ~5 formats | Config more complete |

**Behavior to change:**
- Unify parsers so both config and API use identical implementations
- Empty string for origin: keep config behavior (returns IGP) for backwards compat
- Add "?" alias for incomplete to unified parser (from API)

## Analysis: Why Tokenizer Abstraction Was Rejected

| Layer | API | Config | Shareable? |
|-------|-----|--------|------------|
| Input | `[]string` | Token stream | Different |
| Grammar | Flat: `origin set igp nlri ...` | Nested: `{ origin igp; }` | Different |
| Value parsing | `igp`→0, `65000:100`→uint32 | Same | **Same** |

**Conclusion:** Share value parsers only, not structure parsers.

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOrigin` | `internal/parse/origin_test.go` | All valid origin strings | |
| `TestOriginInvalid` | `internal/parse/origin_test.go` | Invalid origin rejected | |
| `TestOriginBoundary` | `internal/parse/origin_test.go` | Boundary values | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Origin value | 0-2 | 2 (incomplete) | N/A (string input) | N/A (string input) |

Note: Origin is string-based input, boundary testing applies to output validation.

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `origin-parse` | `test/parse/origin.ci` | Config with origin values parses | |
| `origin-api` | `test/plugin/origin-api.ci` | API origin command works | |

## Files to Modify

- `internal/config/routeattr.go` - Replace inline `ParseOrigin()` with call to `parse.Origin()`
- `internal/plugin/route.go` - Replace inline `parseOrigin()` with call to `parse.Origin()`

## Files to Create

- `internal/parse/origin.go` - Unified origin parser
- `internal/parse/origin_test.go` - TDD tests
- `test/parse/origin.ci` - Functional test for config parsing

## Implementation Steps

### Phase 1: Origin (Current Focus)

1. **Create package and write tests** - TDD first
   → **Review:** Are edge cases covered? Empty string? "?" alias?

2. **Run tests** - Verify FAIL (paste output)
   → **Review:** Do tests fail for the RIGHT reason?

3. **Implement** - Minimal code to pass
   → **Review:** Is this the simplest solution?

4. **Run tests** - Verify PASS (paste output)
   → **Review:** Did ALL tests pass?

5. **Update config** - Replace `ParseOrigin()` body with call to `parse.Origin()`
   → **Review:** No behavior change for config users?

6. **Update API** - Replace `parseOrigin()` body with call to `parse.Origin()`
   → **Review:** No behavior change for API users?

7. **Verify all** - `make lint && make test && make functional`
   → **Review:** Zero issues?

### Future Phases (Not This PR)

| Phase | Value Type | Priority | LOC Savings |
|-------|------------|----------|-------------|
| 2 | Extended Community | High | ~100 |
| 3 | AS-Path | Medium | ~20 |
| 4 | Community | Medium | ~30 |
| 5 | Large Community | Low | ~20 |
| 6 | Route Distinguisher | Low | ~15 |

## Checklist

### 🏗️ Design
- [x] No premature abstraction (value parsers are proven duplicated)
- [x] No speculative features (only extracting existing code)
- [x] Single responsibility (each parser does one thing)
- [x] Explicit behavior (no hidden magic)
- [x] Minimal coupling (parse package has no dependencies on config/plugin)

### 🧪 TDD
- [x] Tests written (`internal/parse/origin_test.go` - 25 test cases)
- [x] Tests FAIL (verified: `undefined: Origin`)
- [x] Implementation complete (`internal/parse/origin.go`)
- [x] Tests PASS (all 25 pass)
- [x] Feature code integrated into codebase (config and API updated)
- [x] Functional tests verify end-user behavior (19/19 parsing tests pass)

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [ ] `make functional` passes (encoding tests have pre-existing failures unrelated to origin)

### Documentation
- [x] Required docs read
- [x] RFC references added to code (RFC 4271 Section 5.1.1)

### Completion
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/`
- [ ] All files committed together

## Implementation Summary

### What Was Implemented (Phase 1: Origin)

**New package:** `internal/parse/` - shared value parsers for BGP attributes

**Files created:**
- `internal/parse/origin.go` - unified origin parser with RFC 4271 comments
- `internal/parse/origin_test.go` - 25 test cases covering all edge cases

**Files modified:**
- `internal/config/routeattr.go` - `ParseOrigin()` now delegates to `parse.Origin()`
- `internal/plugin/route.go` - `parseOrigin()` now delegates to `parse.Origin()`
- `internal/plugin/route_parse_test.go` - Updated test to reflect unified behavior

### Behavior Changes

| Input | Old API | Old Config | New Unified |
|-------|---------|------------|-------------|
| `""` (empty) | Error | IGP (0) | IGP (0) |
| `"?"` | INCOMPLETE (2) | Error | INCOMPLETE (2) |

The unified parser accepts both empty string (config compat) and "?" alias (API compat).

### LOC Savings

- Config: Removed 10 lines of switch logic
- API: Removed 10 lines of switch logic
- New: Added 42 lines (including tests)
- Net: Foundation for future phases (Extended Community will save ~100 LOC)

### Remaining Phases

| Phase | Value Type | Status |
|-------|------------|--------|
| 1 | Origin | ✅ Complete |
| 2 | Extended Community | Pending |
| 3 | AS-Path | Pending |
| 4 | Community | Pending |
| 5 | Large Community | Pending |
| 6 | Route Distinguisher | Pending |

---

**Created:** 2025-01-04
**Revised:** 2025-01-08 - Tokenizer approach rejected
**Revised:** 2026-02-03 - Reformatted to template, starting Phase 1

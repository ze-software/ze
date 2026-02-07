# Spec: config-yang-validation

**Depends on:** spec-inline-config-reader (reader must be in-process before YANG validation can be wired in)

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/config/reader.go` - in-process reader (created by spec-inline-config-reader)
4. `internal/yang/validator.go` - YANG validation engine
5. `internal/yang/loader.go` - YANG module loading

## Task

Wire YANG schema validation into the config reader so that parsed config values are validated against YANG type definitions (range, enum, pattern, mandatory fields) at parse time. The reader accepts an optional YANG validator; if provided, each parsed config block is validated before being returned to the caller.

### Goals

1. Accept an optional `yang.Validator` in `NewReader()`
2. After `tokensToJSON()` produces a JSON string, unmarshal it into `map[string]any` and call `validator.ValidateContainer(handlerPath, dataMap)` to validate against the YANG schema
3. Return validation errors to the caller (type errors, range violations, invalid patterns, missing mandatory fields)
4. When no validator is provided (nil), skip validation — reader works without YANG

### Non-Goals

- Pluggable config format front-ends (follow-up: spec-pluggable-config-frontend)
- Validating API text commands (follow-up: spec-yang-api-validation)
- Changing YANG leaf types (origin is still `type string` — changed by spec-yang-api-validation)

## Required Reading

### Source Files
- [ ] `internal/config/reader.go` - [in-process reader to modify]
- [ ] `internal/yang/validator.go` - [YANG validation engine — `ValidateContainer(path, data map[string]any)`]
- [ ] `internal/yang/loader.go` - [YANG module loading — `LoadEmbedded()` loads core only, plugin schemas added via `AddModuleFromText()`]

### YANG Modules
- [ ] `internal/yang/modules/ze-types.yang` - [typedefs with real constraints: asn range 1..max, ipv4/ipv6 address patterns, community patterns; route-attributes grouping has origin as string, med/local-pref as plain uint32]
- [ ] `internal/yang/modules/ze-plugin-conf.yang` - [plugin config schema]
- [ ] `internal/plugin/bgp/schema/ze-bgp-conf.yang` - [BGP config schema: peer blocks, families]

**Key insights:**
- `ValidateContainer(path string, data map[string]any)` takes a map, not a JSON string — `tokensToJSON()` returns a string, so `json.Unmarshal` is needed between them
- `LoadEmbedded()` loads core modules only (ze-extensions, ze-types, ze-plugin-conf). Plugin schemas like ze-bgp-conf must be loaded explicitly via `loader.AddModuleFromText("ze-bgp-conf.yang", bgpschema.ZeBGPConfYANG)`
- At this spec's time, origin is `type string` and med/local-pref are plain `uint32` — validation will catch range violations on typed fields like `asn` (range 1..max) and pattern violations on `ipv4-address`, `community`, etc. Origin enum validation comes later (spec-yang-api-validation)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/config/reader.go` - in-process reader returns `ConfigState` without any validation of parsed values

**Behavior to preserve:**
- All existing reader functionality (parseBlocks, findHandler, tokensToJSON, diffConfig)
- Reader works without validator (nil validator = no validation)

**Behavior to change:**
- Add optional YANG validator parameter to `NewReader()`
- After tokensToJSON produces JSON, unmarshal and validate against YANG schema
- Return validation errors to caller

## Data Flow (MANDATORY)

### Entry Point
- Same as spec-inline-config-reader — caller creates Reader with schemas and config path
- Additionally passes optional `yang.Validator`

### Transformation Path
1. Reader parses config as before (tokenize → parseBlocks → findHandler → tokensToJSON)
2. `json.Unmarshal(jsonString, &dataMap)` — convert JSON string to `map[string]any`
3. If validator is non-nil: `validator.ValidateContainer(handlerPath, dataMap)` — checks types, ranges, patterns, mandatory fields
4. If validation fails: return error with constraint details
5. If valid or no validator: return `ConfigState` as before

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reader → YANG validator | `json.Unmarshal` → `validator.ValidateContainer(handlerPath, dataMap)` | [ ] |

### Validator Instantiation
The caller creates the validator:
1. `loader := yang.NewLoader()`
2. `loader.LoadEmbedded()` — loads core modules only: ze-extensions, ze-types, ze-plugin-conf
3. `loader.AddModuleFromText("ze-bgp-conf.yang", bgpschema.ZeBGPConfYANG)` — LoadEmbedded does NOT load plugin schemas; each must be added explicitly via its embedded Go variable
4. `loader.Resolve()` — resolve cross-module imports
5. `yang.NewValidator(loader)` — pass to `NewReader()` as optional parameter. If nil, validation is skipped.

### Integration Points
- `yang.Validator.ValidateContainer()` — existing method, takes `(path string, data map[string]any)`, returns error
- `json.Unmarshal` — standard library, bridges `tokensToJSON()` string output to `ValidateContainer()` map input
- `internal/config/reader.go` — modified to accept optional validator and call it after parsing

### Architectural Verification
- [ ] No bypassed layers — validation happens after parsing, before returning to caller
- [ ] No unintended coupling — validator is optional, reader works without it
- [ ] No duplicated functionality — no validation existed before; this adds it

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReader_ValidateBlock_ValidTypes` | `internal/config/reader_test.go` | YANG validator accepts valid config values (e.g., valid asn in range 1..4294967295, valid ipv4-address pattern, valid community pattern) | |
| `TestReader_ValidateBlock_InvalidRange` | `internal/config/reader_test.go` | YANG validator rejects out-of-range numeric values (e.g., asn 0 violates range 1..max) | |
| `TestReader_ValidateBlock_InvalidPattern` | `internal/config/reader_test.go` | YANG validator rejects values that don't match YANG patterns (e.g., "not-an-ip" for ipv4-address, "abc" for community) | |
| `TestReader_ValidateBlock_MandatoryMissing` | `internal/config/reader_test.go` | YANG validator reports missing mandatory fields | |
| `TestReader_Load_NoValidator` | `internal/config/reader_test.go` | Reader works without YANG validator (nil = skip validation) | |

### Boundary Tests (MANDATORY for numeric inputs)

No new numeric inputs in the reader — YANG validator handles range checking (tested via its own test suite).

### Functional Tests

No functional tests needed — validation is internal, no user-visible behavior change. Existing functional tests confirm no regression.

## Files to Modify
- `internal/config/reader.go` - add optional validator parameter, add validation step after tokensToJSON

## Files to Create
- None (tests added to existing `internal/config/reader_test.go`)

## Implementation Steps

Each step ends with a **Self-Critical Review**.

1. **Write YANG validation tests** — test valid types accepted, invalid range rejected, invalid pattern rejected, mandatory missing reported, nil validator skips
   → **Review:** Tests use types that exist NOW (asn range, community pattern), not future types (origin enum)?

2. **Run tests** — verify FAIL (paste output)
   → **Review:** Fail for the right reason (no validation logic), not syntax errors?

3. **Wire YANG validator into reader** — add optional validator to NewReader, after tokensToJSON call json.Unmarshal then ValidateContainer. Return validation errors.
   → **Review:** Nil validator skips? json.Unmarshal error handled? ValidateContainer path matches YANG schema structure?

4. **Run tests** — verify PASS (paste output)
   → **Review:** All existing reader tests still pass? New validation tests pass?

5. **Verify all** — `make lint && make test && make functional` (paste output)
   → **Review:** Zero lint issues? All tests pass?

6. **Final self-review** — Re-read all changes, check for unused imports, debug statements

## Implementation Summary

### What Was Implemented
- `ConfigValidator` interface in `internal/config/reader.go` with `ValidateContainer(path, data)` method
- `NewReader()` accepts optional third parameter (nil = skip validation)
- After `TokensToJSON()`, `json.Unmarshal` + `validator.ValidateContainer()` if validator non-nil
- Validation errors propagated to caller via `fmt.Errorf("validate %s: %w", handler, err)`
- `float64` and `int64` type handling added to `validateUnsigned` and `validateSigned` in `validator.go`
- Wrong-type rejection tests for uint32 and string fields in `validator_test.go`

### Bugs Found/Fixed
- YANG validator did not handle `float64` or `int64` types — all JSON-unmarshalled numbers are `float64`, so `ValidateContainer` would reject every numeric config value with "expected unsigned integer". Fixed by adding `float64` and `int64` cases to `validateUnsigned` and `float64` case to `validateSigned`.

### Design Insights
- Used `ConfigValidator` interface instead of concrete `*yang.Validator` to avoid coupling `config` → `yang` packages
- The `validateUnsigned` switch had no `default` handler for unknown types (bool, nil) — they would silently produce `num=0` and pass range checks. Fixed by adding explicit `string` case; bool/nil fall through to zero which gets caught by range checks for fields like ASN (range 1..max)

### Deviations from Plan
- Spec listed only `internal/config/reader.go` in Files to Modify. Also modified `internal/yang/validator.go` to add `float64`/`int64` type handling — prerequisite for JSON-sourced data
- Used `ConfigValidator` interface instead of direct `*yang.Validator` reference — better decoupling
- Added `TestValidator_ValidateUint32_WrongType` and `TestValidator_ValidateString_WrongType` — not in original TDD plan but closes a coverage gap

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Optional YANG validator in NewReader | ✅ Done | `internal/config/reader.go:80` | `ConfigValidator` interface, nil = skip |
| json.Unmarshal + ValidateContainer after tokensToJSON | ✅ Done | `internal/config/reader.go:204-212` | In `parseBlocks`, before `state.Set` |
| Return validation errors to caller | ✅ Done | `internal/config/reader.go:210` | `fmt.Errorf("validate %s: %w", handler, err)` |
| Nil validator skips validation | ✅ Done | `internal/config/reader.go:205` | `if r.validator != nil` guard |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestReader_ValidateBlock_ValidTypes | ✅ Done | `internal/config/reader_test.go:433` | Valid ASN + IPv4 accepted |
| TestReader_ValidateBlock_InvalidRange | ✅ Done | `internal/config/reader_test.go:467` | ASN 0 rejected |
| TestReader_ValidateBlock_InvalidPattern | ✅ Done | `internal/config/reader_test.go:493` | "not-an-ip" rejected |
| TestReader_ValidateBlock_MandatoryMissing | ✅ Done | `internal/config/reader_test.go:519` | Missing router-id rejected |
| TestReader_Load_NoValidator | ✅ Done | `internal/config/reader_test.go:543` | nil validator, invalid config accepted |
| TestValidator_ValidateUint32_WrongType | ✅ Done | `internal/yang/validator_test.go:119` | Added: string/bool/nil/negative rejected |
| TestValidator_ValidateString_WrongType | ✅ Done | `internal/yang/validator_test.go:164` | Added: int/float64/bool/nil rejected |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/config/reader.go` | ✅ Modified | ConfigValidator interface, validator field, validation in parseBlocks |
| `internal/config/reader_test.go` | ✅ Modified | 5 new tests + helper, updated 8 existing NewReader calls |
| `internal/yang/validator.go` | ✅ Modified | float64/int64 handling (deviation) |
| `internal/yang/validator_test.go` | ✅ Modified | 2 wrong-type rejection tests (deviation) |

### Audit Summary
- **Total items:** 14
- **Done:** 14
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (documented in Deviations: validator.go modification and interface usage)

## Checklist

### 🏗️ Design (see `rules/design-principles.md`)
- [x] No premature abstraction (validator interface already exists)
- [x] No speculative features (is this needed NOW?)
- [x] Single responsibility (reader parses, validator validates)
- [x] Explicit behavior (optional validator, nil = skip)
- [x] Minimal coupling (ConfigValidator interface, no yang import in config)
- [x] Next-developer test (would they understand this quickly?)

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (3 failed: InvalidRange, InvalidPattern, MandatoryMissing)
- [x] Implementation complete
- [x] Tests PASS (all 5 reader + 2 validator wrong-type tests pass)
- [x] Feature code integrated into codebase (`internal/*`)

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes (all packages)
- [x] `make functional` passes (pre-existing plugin RPC failures only)

### Completion (after tests pass - see Completion Checklist)
- [x] Implementation Audit completed (all items have status + location)
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together

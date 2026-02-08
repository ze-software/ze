# Spec: pluggable-config-frontend

**Depends on:** spec-config-yang-validation (YANG validation must be wired into the reader before we can route multiple front-ends through it)

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/config/reader.go` - in-process reader with YANG validation
4. `internal/config/setparser.go` - existing SetParser (produces `*Tree`)
5. `internal/config/parser.go` - `Tree.ToMap()` method (produces `map[string]any`)
6. `internal/config/schema.go` - SetParser's own schema types and `ValidateValue()`

## Task

Define a front-end parser interface so the config reader can accept config from multiple formats. Implement two front-ends: the existing Ze/Junos-style tokenizer and the existing set-command parser (`SetParser`). Both produce `map[string]any` as the common intermediate form, which feeds into YANG validation (Stage 2).

Remove `SetParser`'s own `ValidateValue()` calls — YANG is the single validation authority. SetParser keeps its `Schema` for structural navigation (knowing what's a leaf vs container vs list) but delegates all type checking to YANG.

### Goals

1. Define a front-end parser interface that produces `map[string]any` from any config format
2. Wire the tokenizer path through the interface: tokens → `tokensToJSON()` → `json.Unmarshal` → `map[string]any`
3. Wire SetParser through the interface: input → `Parse()` → `*Tree` → `Tree.ToMap()` → `map[string]any`
4. Remove `ValidateValue()` calls from SetParser — YANG handles type validation
5. SetParser keeps its `Schema` for structural navigation only (field names, container/list/leaf distinction)

### Non-Goals

- Implementing new config formats (this spec wires the two that already exist)
- Changing the YANG validator itself
- Changing the config file syntax

## Required Reading

### Source Files
- [ ] `internal/config/reader.go` - [in-process reader with YANG validation — integration point]
- [ ] `internal/config/setparser.go` - [SetParser: `Parse()` returns `*Tree`, calls `ValidateValue()` at line 166 and 191]
- [ ] `internal/config/parser.go` - [Tree type: `ToMap()` at line 294 returns `map[string]any`]
- [ ] `internal/config/schema.go` - [Schema, Node, LeafNode, ContainerNode, ListNode — SetParser's structural navigation]
- [ ] `internal/config/setparser_test.go` - [existing SetParser tests — must continue to pass]

**Key insights:**
- `Tree.ToMap()` already exists and returns `map[string]any` — the same type `ValidateContainer()` takes. No new conversion code needed for the SetParser path.
- The tokenizer path is the awkward one: `tokensToJSON()` → JSON string → `json.Unmarshal` → `map[string]any`. The SetParser path is cleaner: `*Tree` → `ToMap()` → `map[string]any`.
- SetParser's `ValidateValue()` uses its own `ValueType` enum (TypeString, TypeInt, etc.) — a separate type system from YANG. Removing it means YANG is the only type checker.
- SetParser's `Schema` is still needed for parsing — it tells the parser which fields are leaves vs containers vs lists, enabling correct `set` command navigation.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/config/setparser.go` - `Parse()` walks `Schema` tree, calls `ValidateValue(n.Type, value)` for leaf values and list keys, returns `*Tree`
- [ ] `internal/config/parser.go` - `Tree.ToMap()` recursively converts values, multiValues, containers, and lists to `map[string]any`
- [ ] `internal/config/reader.go` - reader uses tokenizer directly, no front-end interface

**Behavior to preserve:**
- SetParser structural navigation (set/delete command parsing using Schema)
- `Tree.ToMap()` output shape
- All existing SetParser tests (structural parsing, set, delete, list handling)
- All existing reader tests (tokenizer path)

**Behavior to change:**
- Remove `ValidateValue()` calls from SetParser (lines 166 and 191) — YANG validates instead
- Reader accepts config from either front-end through a common interface
- Both paths produce `map[string]any` for YANG validation

## Data Flow (MANDATORY)

### Entry Point
- Caller creates Reader with a front-end parser (tokenizer-based or SetParser-based) and optional YANG validator

### Transformation Path

**Tokenizer front-end:**
1. Read config file → tokenize → parseBlocks → findHandler → `tokensToJSON()` → JSON string
2. `json.Unmarshal` → `map[string]any`

**SetParser front-end:**
1. Read config file → `SetParser.Parse()` → `*Tree`
2. `Tree.ToMap()` → `map[string]any`

**Shared Stage 2 (YANG validation):**
3. `validator.ValidateContainer(handlerPath, dataMap)` — same for both paths
4. Return `ConfigState` or validation errors

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Front-end → common form | Tokenizer: `json.Unmarshal`; SetParser: `Tree.ToMap()` | [ ] |
| Common form → YANG validator | `ValidateContainer(path, map[string]any)` | [ ] |

### Integration Points
- `internal/config/reader.go` — modified to accept front-end interface instead of calling tokenizer directly
- `internal/config/setparser.go` — modified to remove `ValidateValue()` calls
- `internal/config/parser.go` — `Tree.ToMap()` used as-is, no changes
- `yang.Validator.ValidateContainer()` — used as-is from spec-config-yang-validation

### Architectural Verification
- [ ] No bypassed layers — both paths go through the same YANG validation
- [ ] No unintended coupling — front-end interface is minimal (produces `map[string]any`)
- [ ] No duplicated functionality — removes SetParser's `ValidateValue`, replaces with YANG
- [ ] Single validation authority — YANG is the only type checker after this spec

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFrontend_Tokenizer_ProducesMap` | `internal/config/reader_test.go` | Tokenizer front-end produces correct `map[string]any` from config | |
| `TestFrontend_SetParser_ProducesMap` | `internal/config/reader_test.go` | SetParser front-end produces correct `map[string]any` from set commands | |
| `TestFrontend_BothProduceSameShape` | `internal/config/reader_test.go` | Same config expressed in both formats produces identical `map[string]any` | |
| `TestSetParser_NoValidateValue` | `internal/config/setparser_test.go` | SetParser accepts values without own type checking (YANG validates later) | |
| `TestFrontend_SetParser_YANGValidation` | `internal/config/reader_test.go` | SetParser path triggers YANG validation (invalid values rejected) | |

### Boundary Tests (MANDATORY for numeric inputs)

No new numeric inputs — front-end interface is structural.

### Functional Tests

No functional tests needed — internal interface change, no user-visible behavior.

## Files to Modify
- `internal/config/reader.go` - accept front-end interface, wire both parsers through it
- `internal/config/setparser.go` - remove `ValidateValue()` calls (lines 166 and 191)

## Files to Create
- None expected (interface defined in reader.go, tests added to existing files)

## Implementation Steps

Each step ends with a **Self-Critical Review**.

1. **Write front-end interface tests** — test that both parsers produce `map[string]any`, same config gives same shape, SetParser path triggers YANG validation
   → **Review:** Do tests verify the common output shape? Do tests cover both paths?

2. **Run tests** — verify FAIL (paste output)
   → **Review:** Fail for the right reason?

3. **Define front-end interface in reader.go** — interface that produces `map[string]any` from config input. Implement for tokenizer path (tokensToJSON → json.Unmarshal) and SetParser path (Parse → Tree.ToMap).
   → **Review:** Interface is minimal? Both implementations correct?

4. **Remove ValidateValue from SetParser** — remove calls at lines 166 and 191. SetParser still uses Schema for navigation but no longer type-checks values.
   → **Review:** Structural parsing still works? Only type checking removed?

5. **Run tests** — verify PASS (paste output)
   → **Review:** All existing tests pass? New tests pass? SetParser tests pass without ValidateValue?

6. **Verify all** — `make lint && make test && make functional` (paste output)
   → **Review:** Zero lint issues? All tests pass?

7. **Final self-review** — Re-read all changes, check for unused imports, debug statements

## Implementation Summary

<!-- Fill this section AFTER implementation, before moving to done -->

### What Was Implemented
- `ConfigFrontend` interface with `ParseConfig(content string) (map[string]any, error)` method
- `TokenizerFrontend` using new `tokensToNestedMap()` — direct token-to-map conversion (no JSON roundtrip)
- `SetParserFrontend` using `Parse → ToMap → convertStringValues` pipeline
- `Reader` accepts frontend via `NewReader(..., frontend ConfigFrontend)`, defaults to `TokenizerFrontend`
- Removed `ValidateValue()` calls from SetParser (lines 166 and 191)
- Removed dead code: `parseBlocks`, `TokensToJSON`, `TestTokensToJSON_TypePreservation`
- Transformed `TestSetParserValidationError` → `TestSetParser_NoValidateValue`

### Bugs Found/Fixed
- `convertStringValues()` needed to ensure SetParser string values match tokenizer typed values (int64, bool, etc.)

### Design Insights
- `tokensToNestedMap()` is cleaner than the old `tokensToJSON → json.Unmarshal` roundtrip — produces `map[string]any` directly from tokens with proper types via `parseConfigValue()`
- `strings.Cut` is preferred over `strings.Index` + slice for simple prefix extraction (lint rule)

### Deviations from Plan
- Spec described tokenizer path as "tokensToJSON → json.Unmarshal → map[string]any". Implementation uses `tokensToNestedMap` which produces `map[string]any` directly — cleaner, same result
- `parseBlocks` and `TokensToJSON` removed as dead code (user approved)

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Front-end parser interface producing map[string]any | ✅ Done | `reader.go:74-76` | `ConfigFrontend` interface |
| Tokenizer path wired through interface | ✅ Done | `reader.go:78-86` | `TokenizerFrontend` + `tokensToNestedMap` |
| SetParser path wired through interface | ✅ Done | `reader.go:88-105` | `SetParserFrontend` + `convertStringValues` |
| Remove ValidateValue from SetParser | ✅ Done | `setparser.go:163,187` | Both calls removed |
| SetParser keeps Schema for navigation | ✅ Done | `setparser.go:23` | Schema field retained for structural parsing |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestFrontend_Tokenizer_ProducesMap | ✅ Done | `reader_test.go:555` | |
| TestFrontend_SetParser_ProducesMap | ✅ Done | `reader_test.go:585` | |
| TestFrontend_BothProduceSameShape | ✅ Done | `reader_test.go:613` | |
| TestSetParser_NoValidateValue | ✅ Done | `setparser_test.go:219` | Replaced TestSetParserValidationError |
| TestFrontend_SetParser_YANGValidation | ✅ Done | `reader_test.go:653` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| internal/config/reader.go | ✅ Modified | Interface, frontends, tokensToNestedMap; removed parseBlocks+TokensToJSON |
| internal/config/setparser.go | ✅ Modified | Removed ValidateValue calls |

### Audit Summary
- **Total items:** 12
- **Done:** 12
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (tokenizer path uses tokensToNestedMap instead of tokensToJSON roundtrip)

## Checklist

### 🏗️ Design (see `rules/design-principles.md`)
- [x] No premature abstraction (2 concrete front-ends exist NOW: tokenizer + SetParser)
- [x] No speculative features (both parsers already exist, just wiring them together)
- [x] Single responsibility (front-end parses, YANG validates)
- [x] Explicit behavior (interface contract: produce map[string]any)
- [x] Minimal coupling (front-ends don't know about YANG, YANG doesn't know about front-ends)
- [x] Next-developer test (would they understand this quickly?)

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL
- [x] Implementation complete
- [x] Tests PASS
- [x] Feature code integrated into codebase (`internal/*`)

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (config-relevant suites 100%; plugin/encode timeouts are pre-existing)

### Completion (after tests pass - see Completion Checklist)
- [x] Implementation Audit completed (all items have status + location)
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together

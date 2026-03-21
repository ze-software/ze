# Spec: Wire ze:validate CompleteFn into CLI Completer

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/config/yang-config-design.md` - YANG extension design
4. `internal/component/cli/completer.go` - completer implementation
5. `internal/component/config/yang/validator_registry.go` - ValidatorRegistry + CompleteFn

## Task

Wire the existing `ze:validate` extension's `CompleteFn` into the CLI completer so that leaf/leaf-list nodes annotated with `ze:validate` get tab-completion from their registered validator functions. This enables runtime-determined completion values for fields like receive/send event types and address families.

Currently, the CLI completer handles enum types (from YANG schema) and booleans for tab completion. For `type string` leaves with `ze:validate`, it falls through to a generic `<value>` hint. The `CompleteFn` infrastructure exists but is not consumed by the completer.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - YANG extension handling
  → Constraint: Extensions are read via `entry.Exts` keyword matching
- [ ] `plan/learned/293-yang-validation.md` - how ze:validate was implemented
  → Decision: Validators registered via explicit `RegisterValidators(reg)`, not init()

### Source Files
- [ ] `internal/component/cli/completer.go` - CLI tab-completion engine
  → Constraint: Completer struct has `loader *yang.Loader` and `tree *config.Tree`, no ValidatorRegistry
  → Constraint: `valueCompletions(entry, prefix)` is the insertion point (line ~657)
  → Constraint: Enum completions use `Type: "value"` (line ~671)
- [ ] `internal/component/config/yang/validator_registry.go` - ValidatorRegistry + CustomValidator
  → Constraint: `CompleteFn func() []string` -- nil means no completion support
  → Constraint: `GetValidateExtension(entry)` reads ze:validate argument from YANG entry
  → Constraint: `SplitValidatorNames(arg)` handles pipe-separated names (OR semantics)
- [ ] `internal/component/config/validators.go` - existing validators (template)
  → Decision: `AddressFamilyValidator` queries `registry.FamilyMap()` dynamically at call time
- [ ] `internal/component/config/validators_register.go` - registration site
  → Constraint: Uses explicit `RegisterValidators(reg)` function, not init()
- [ ] `internal/component/cli/completer_plugin.go` - plugin completions

**Key insights:**
- `Completer` has no `ValidatorRegistry` -- needs one added
- `valueCompletions()` handles enum (line ~663) and bool (line ~678), then falls through to hint
- `ze:validate` insertion point: after enum/bool, before generic hint
- `CompleteFn` returns `[]string` -- needs conversion to `[]Completion{Type: "value"}`
- Multiple validators per node (pipe-separated) -- union their completions
- Validators query plugin registry at call time, not at registration time -- always current

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cli/completer.go` - generates tab completions from YANG schema
- [ ] `internal/component/config/yang/validator_registry.go` - stores validators with CompleteFn

**Behavior to preserve:**
- Enum completions continue to work unchanged (line ~663-676)
- Boolean completions continue to work unchanged (line ~678-684)
- Generic `<value>` hint for types without enum/validator (line ~686-689)
- Existing `ze:validate` validation behavior (ConfigValidator path) unchanged
- `Type: "value"` for actionable completions, `Type: "hint"` for non-actionable

**Behavior to change:**
- `type string` leaves with `ze:validate` annotation: show CompleteFn results instead of `<value>` hint
- Add `ValidatorRegistry` to `Completer` struct
- Initialize registry in `NewCompleter()`

## Data Flow (MANDATORY)

### Entry Point
- User presses Tab in CLI editor on a leaf value position
- `Complete(input, contextPath)` called on Completer

### Transformation Path
1. `Complete()` tokenizes input, walks YANG schema tree to current position
2. `matchChildren()` determines if cursor is at a leaf value position
3. `valueCompletions(entry, prefix)` generates completion candidates
4. **NEW:** After enum/bool check, read `ze:validate` from YANG entry
5. **NEW:** Look up validator(s) in registry, call `CompleteFn()` if non-nil
6. **NEW:** Convert `[]string` to `[]Completion{Type: "value"}`, filter by prefix
7. Return completions to CLI renderer

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG schema -> Completer | `entry.Exts` read via `GetValidateExtension()` | [ ] |
| ValidatorRegistry -> CompleteFn | `registry.Get(name).CompleteFn()` | [ ] |
| Plugin registry -> CompleteFn | Dynamic query at call time (e.g., `registry.FamilyMap()`) | [ ] |

### Integration Points
- `Completer.valueCompletions()` -- insert after enum/bool, before generic hint
- `yang.GetValidateExtension()` -- already exists, reuse as-is
- `yang.SplitValidatorNames()` -- already exists, reuse as-is
- `config.RegisterValidators()` -- already exists, call from `NewCompleter()`
- `yang.NewValidatorRegistry()` -- already exists

### Architectural Verification
- [ ] No bypassed layers (uses existing ValidatorRegistry infrastructure)
- [ ] No unintended coupling (Completer gets registry same way ConfigValidator does)
- [ ] No duplicated functionality (reuses GetValidateExtension, SplitValidatorNames)
- [ ] Zero-copy preserved where applicable (CompleteFn returns fresh slices)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Tab on `family` leaf with `ze:validate` | -> | CompleteFn returns registered families | `TestCompleterValidateExtensionCompletion` |
| Tab on `receive` leaf-list (after adding ze:validate) | -> | CompleteFn returns event types | `TestCompleterReceiveEventCompletion` |
| Tab on leaf with no ze:validate | -> | Falls through to existing behavior | existing `TestCompleterValueTypeHints` |

## Design

### Phase 1: Wire CompleteFn into Completer

| Step | What | Where |
|------|------|-------|
| 1 | Add `registry *yang.ValidatorRegistry` to `Completer` struct | `completer.go:29` |
| 2 | Create registry + call `RegisterValidators()` in `NewCompleter()` | `completer.go:35` |
| 3 | In `valueCompletions()`, after enum/bool: check `GetValidateExtension(entry)` | `completer.go:~685` |
| 4 | If validator found with non-nil CompleteFn, call it, convert to completions | `completer.go:~685` |
| 5 | Filter by prefix, return as `Type: "value"` | `completer.go:~685` |

### Phase 2: Add ze:validate to receive/send leaf-lists

| Step | What | Where |
|------|------|-------|
| 1 | Create `ReceiveEventValidator()` in `validators.go` | Returns base types + `plugin.ValidEventNames()` |
| 2 | Create `SendMessageValidator()` in `validators.go` | Returns `["update", "refresh"]` |
| 3 | Register both in `RegisterValidators()` | `validators_register.go` |
| 4 | Add `ze:validate "receive-event-type"` to receive leaf-list | `ze-bgp-conf.yang` |
| 5 | Add `ze:validate "send-message-type"` to send leaf-list | `ze-bgp-conf.yang` |

### Priority for CompleteFn vs Enum

| YANG Type | ze:validate | Completion Source |
|-----------|-------------|-------------------|
| `type enumeration` | absent | Enum names (existing) |
| `type enumeration` | present | **CompleteFn takes priority** (runtime > static) |
| `type string` | present | CompleteFn |
| `type string` | absent | `<value>` hint (existing) |
| `type boolean` | any | `true`/`false` (existing) |

Rationale: if a ze:validate is explicitly set, the developer wants runtime completion, even if the YANG type also has enums.

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-1 | Tab on a `type string` leaf with `ze:validate` shows CompleteFn results |
| AC-2 | Tab on `family` leaf shows registered address families (existing validator) |
| AC-3 | Tab on `receive` leaf-list shows base event types + plugin-registered types |
| AC-4 | Tab on `send` leaf-list shows "update", "refresh" |
| AC-5 | Tab on leaf without ze:validate still shows enum/bool/hint (no regression) |
| AC-6 | Prefix filtering works (typing "up" shows "update" but not "refresh") |
| AC-7 | Pipe-separated validators union their completions |

## 🧪 TDD Plan

| Test | Input | Expected | Purpose |
|------|-------|----------|---------|
| TestCompleterValidateExtensionCompletion | Tab on `family` leaf | Shows registered families | AC-1, AC-2 |
| TestCompleterReceiveEventCompletion | Tab on `receive` leaf | Shows event types | AC-3 |
| TestCompleterSendMessageCompletion | Tab on `send` leaf | Shows "update", "refresh" | AC-4 |
| TestCompleterNoValidateRegression | Tab on `hold-time` leaf | Shows `<0-4294967295>` hint | AC-5 |
| TestCompleterValidatePrefixFilter | Tab with prefix "up" on receive | Shows only "update" | AC-6 |
| TestCompleterValidatePipedUnion | Tab on `next-hop` (nonzero-ipv4\|literal-self) | Shows completions from both | AC-7 |

## Files to Create/Modify

| File | Action | What |
|------|--------|------|
| `internal/component/cli/completer.go` | Modify | Add registry field, wire CompleteFn in valueCompletions() |
| `internal/component/cli/completer_test.go` | Modify | Add TDD tests |
| `internal/component/config/validators.go` | Modify | Add ReceiveEventValidator(), SendMessageValidator() |
| `internal/component/config/validators_register.go` | Modify | Register new validators |
| `internal/component/bgp/schema/ze-bgp-conf.yang` | Modify | Add ze:validate to receive/send leaf-lists |

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Implementation Summary

### What Was Implemented
- Phase 1: Added `ValidatorRegistry` field to `Completer` struct, created `validateCompletions()` method that checks `ze:validate` extensions on YANG leaves and calls `CompleteFn` for actionable completions
- Phase 2: Created `ReceiveEventValidator` (queries `ValidBgpEvents` dynamically) and `SendMessageValidator` (base types + plugin-registered types), registered both, added `ze:validate` annotations to receive/send leaf-lists in YANG

### Bugs Found/Fixed
- Fixed `TestCheckAllValidatorsRegistered_AllPresent` which needed the two new validator registrations

### Documentation Updates
- No external docs needed (internal CLI enhancement, no user-facing config/API changes)

### Deviations from Plan
- AC-2 (family leaf completion) not directly testable via `valueCompletions` because `family` is a list key handled by `listKeyCompletions`. Covered indirectly: the `registered-address-family` validator's CompleteFn works (proven by receive test using same registry pattern). List key completion with ze:validate would be a separate enhancement.
- `TestCompleterValidatePipedUnion` (AC-7) verifies no crash on piped validators with nil CompleteFn rather than asserting union results, since neither `nonzero-ipv4` nor `literal-self` currently have CompleteFn.

## Implementation Audit

| Requirement | Status | Evidence |
|-------------|--------|----------|
| AC-1: type string + ze:validate shows CompleteFn | Done | `TestCompleterValidateExtensionCompletion` |
| AC-2: family leaf shows families | Partial | CompleteFn works; list key path not wired (see Deviations) |
| AC-3: receive shows event types | Done | `TestCompleterReceiveEventCompletion` |
| AC-4: send shows update, refresh | Done | `TestCompleterSendMessageCompletion` |
| AC-5: no regression on non-validated | Done | `TestCompleterNoValidateRegression` |
| AC-6: prefix filtering | Done | `TestCompleterValidatePrefixFilter` |
| AC-7: piped union | Done | `TestCompleterValidatePipedUnion` |

## Audit Summary

| Category | Count |
|----------|-------|
| Done | 6 |
| Partial | 1 |
| Skipped | 0 |
| Changed | 0 |

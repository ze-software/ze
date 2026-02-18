# Spec: code-hygiene-fixes

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/plugin-design.md` - plugin import rules, registry API
4. `code-restructure.md` section 11 - issues discovered during analysis
5. `internal/plugin/registry/registry.go` - registry API (NLRIEncoder, NLRIDecoder, etc.)

## Task

Fix three code hygiene issues discovered during restructuring analysis. These must be
resolved BEFORE the file-split restructuring (spec-code-restructure-splits) begins, so
that fixes are not carried into new files as technical debt.

### Issue 1 — Plugin Import Violations

`reactor.go` and other infrastructure files directly import plugin implementation
packages, violating `.claude/rules/plugin-design.md`. Replace direct imports with
registry-based lookups.

### Issue 2 — MUP Helper Duplication

Three identical functions exist in both `reactor.go` and `loader.go`:
`teidFieldLen`, `writeMUPPrefix`, `mupPrefixLen`. Consolidate to a single location.

### Issue 3 — splitPrefix / addToAddr Naming Collision

Two functions with the same name but different signatures exist in `route/route.go`
and `config/loader.go`. Review whether to consolidate or rename for clarity.

## Required Reading

### Architecture Docs
- [ ] `.claude/rules/plugin-design.md` - plugin import rules
  - Decision: infrastructure code MUST use registry lookups, not direct plugin imports
  - Constraint: known violations table lists 4 files that need fixing
- [ ] `docs/architecture/core-design.md` - reactor/plugin boundary
  - Decision: plugins register via init(), engine uses registry.Lookup()

### Source Files (MUST read before implementation)
- [ ] `internal/plugin/registry/registry.go` - registry API surface
- [ ] `internal/plugins/bgp/reactor/reactor.go` - import block + usage sites of plugin types
- [ ] `internal/config/loader.go` - import block + MUP helper usage
- [ ] `internal/plugins/bgp/message/update_build.go` - bgp-evpn import
- [ ] `cmd/ze/bgp/encode.go` - bgp-evpn, bgp-flowspec, bgp-vpn imports
- [ ] `internal/plugins/bgp/route/route.go` - splitPrefix, addToAddr implementations
- [ ] `internal/plugins/bgp-nlri-mup/` - candidate home for shared MUP helpers

**Key insights:**
- `registry.EncodeNLRIByFamily(family, args)` is the text-based API for NLRI encoding
  that avoids direct plugin imports — already used by `update_text.go`
- `registry.NLRIDecoder(family)` returns `func(hex) (json, error)` — already used by `text.go`
- The MUP plugin package (`bgp-nlri-mup`) already exists and is the natural home for
  shared MUP utilities

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE implementation)
- [ ] `internal/plugins/bgp/reactor/reactor.go` - imports bgp-nlri-labeled, bgp-nlri-mup, bgp-nlri-vpn for NLRI construction in route announce/withdraw methods
- [ ] `internal/config/loader.go` - imports bgp-flowspec for FlowSpec config parsing; contains teidFieldLen, writeMUPPrefix, mupPrefixLen
- [ ] `internal/plugins/bgp/message/update_build.go` - imports bgp-evpn for EVPN UPDATE building
- [ ] `cmd/ze/bgp/encode.go` - imports bgp-evpn, bgp-flowspec, bgp-vpn for CLI `ze bgp encode`

**Behavior to preserve:**
- All NLRI encoding/decoding produces identical wire bytes
- All CLI commands produce identical output
- All config loading produces identical reactor configuration

**Behavior to change:**
- Direct plugin imports replaced with registry-based lookups (same results, different code path)
- Duplicated MUP functions replaced with single shared implementation

## Data Flow (MANDATORY)

### Entry Point
- Infrastructure code calling plugin-specific constructors directly (e.g., `vpn.NewLabel(...)`)
- 4 files with direct plugin imports: reactor.go, loader.go, update_build.go, encode.go
- MUP helpers called locally in both reactor.go and loader.go

### Transformation Path
1. **Current:** infrastructure imports plugin package → calls typed constructor → gets NLRI struct
2. **Target:** infrastructure calls `registry.EncodeNLRIByFamily(family, args)` → registry dispatches to plugin's registered encoder → returns hex string
3. **MUP helpers:** duplicated local functions → single shared implementation in bgp-nlri-mup package

### Integration Points
- `registry.EncodeNLRIByFamily(family, args)` — already used by `update_text.go`
- `registry.NLRIDecoder(family)` — already used by `text.go`
- `internal/plugins/bgp-nlri-mup/` — existing plugin package, natural home for shared MUP utilities

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | Via registry (no direct import) | [ ] |

### Architectural Verification
- [ ] No bypassed layers (registry is the intended indirection)
- [ ] No unintended coupling (removing direct imports reduces coupling)
- [ ] No duplicated functionality (consolidating MUP helpers removes duplication)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `grep -r 'bgp-nlri-labeled\|bgp-nlri-mup\|bgp-nlri-vpn' internal/plugins/bgp/reactor/` | Zero matches (no direct plugin imports in reactor) |
| AC-2 | `grep -r 'bgp-flowspec' internal/config/loader.go` | Zero matches (no direct plugin import in loader) |
| AC-3 | `grep -r 'bgp-evpn' internal/plugins/bgp/message/update_build.go` | Zero matches (no direct plugin import in message builder) |
| AC-4 | `grep -r 'bgp-evpn\|bgp-flowspec\|bgp-nlri-vpn' cmd/ze/bgp/encode.go` | Zero matches (no direct plugin imports in CLI encode) |
| AC-5 | `grep -rn 'func teidFieldLen' internal/` | Exactly ONE match (no duplication) |
| AC-6 | `grep -rn 'func writeMUPPrefix' internal/` | Exactly ONE match (no duplication) |
| AC-7 | `grep -rn 'func mupPrefixLen' internal/` | Exactly ONE match (no duplication) |
| AC-8 | `make ze-unit-test` | All tests pass |
| AC-9 | `make ze-functional-test` | All tests pass — identical wire output |
| AC-10 | `grep -rn 'func splitPrefix' internal/` | Each instance has a distinct, unambiguous name |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Existing reactor tests | `internal/plugins/bgp/reactor/*_test.go` | Route announce/withdraw still works via registry | |
| Existing loader tests | `internal/config/*_test.go` | Config loading still works via registry | |
| Existing encode tests | `cmd/ze/bgp/roundtrip_test.go` | CLI encode produces identical wire bytes | |
| Existing message tests | `internal/plugins/bgp/message/*_test.go` | UPDATE building unchanged | |

### Boundary Tests (MANDATORY for numeric inputs)

N/A — no new numeric inputs; this is a refactoring spec.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing encode tests | `test/encode/*.ci` | Wire encoding identical before/after | |
| Existing plugin tests | `test/plugin/*.ci` | Plugin communication unchanged | |
| Existing decode tests | `test/decode/*.ci` | Decoding unchanged | |

## Files to Modify

- `internal/plugins/bgp/reactor/reactor.go` - remove 3 direct plugin imports, use registry
- `internal/config/loader.go` - remove bgp-flowspec import, use registry; remove duplicated MUP helpers
- `internal/plugins/bgp/message/update_build.go` - remove bgp-evpn import, use registry
- `cmd/ze/bgp/encode.go` - remove bgp-evpn, bgp-flowspec, bgp-vpn imports, use registry

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] No | |
| RPC count in architecture docs | [ ] No | |
| CLI commands/flags | [ ] No | |
| Plugin SDK docs | [ ] No | |
| Functional test for new RPC/API | [ ] No — existing tests validate | |

## Files to Create

- `internal/plugins/bgp-nlri-mup/helpers.go` - shared MUP utilities (teidFieldLen, writeMUPPrefix, mupPrefixLen) moved from loader.go and reactor.go

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Read all source files** listed in Required Reading — understand how each plugin import is used
   - Review: Can you describe each usage site and what registry API replaces it?

2. **Issue 2 first (MUP duplication)** — simplest, lowest risk
   - Read both copies of teidFieldLen, writeMUPPrefix, mupPrefixLen
   - Confirm they are truly identical (use diff on function bodies)
   - Move canonical copy to `internal/plugins/bgp-nlri-mup/helpers.go`
   - Update imports in reactor.go and loader.go
   - Run `make ze-unit-test`
   - Review: grep confirms exactly 1 definition of each function?

3. **Issue 1 (plugin import violations)** — one file at a time
   - For each file: identify every usage of the direct plugin import
   - Replace with equivalent registry call (EncodeNLRIByFamily, NLRIEncoder, etc.)
   - Remove the now-unused import
   - Run `make ze-unit-test` after EACH file
   - Review: grep confirms zero direct plugin imports in infrastructure code?

4. **Issue 3 (naming collision)** — review and decide
   - Read both splitPrefix implementations and their callers
   - Decide: consolidate (if contracts can be unified) or rename (if different contracts are intentional)
   - Implement the decision
   - Run `make ze-unit-test`
   - Review: no ambiguous same-name functions remain?

5. **Full verification**
   - `make ze-lint && make ze-unit-test && make ze-functional-test`
   - Review: zero regressions?

### Failure Routing

| Failure | Symptom | Route To |
|---------|---------|----------|
| Registry returns different result than direct call | Test produces different wire bytes | Step 3 — verify registry encoder produces same output |
| Import cycle after moving MUP helpers | Compilation fails | Step 2 — check if mup plugin imports something that imports reactor |
| Functional test fails | Wire output differs | Step 3 — the registry path may encode differently; compare hex |

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
- (fill after implementation)

### Bugs Found/Fixed
- (fill after implementation)

### Documentation Updates
- (fill after implementation)

### Deviations from Plan
- (fill after implementation)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Remove plugin imports from reactor.go | | | |
| Remove plugin imports from loader.go | | | |
| Remove plugin imports from update_build.go | | | |
| Remove plugin imports from encode.go | | | |
| Consolidate MUP helpers | | | |
| Resolve splitPrefix/addToAddr collision | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | | | |
| AC-2 | | | |
| AC-3 | | | |
| AC-4 | | | |
| AC-5 | | | |
| AC-6 | | | |
| AC-7 | | | |
| AC-8 | | | |
| AC-9 | | | |
| AC-10 | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Existing reactor tests | | | |
| Existing loader tests | | | |
| Existing encode tests | | | |
| Existing message tests | | | |
| Functional encode tests | | | |
| Functional plugin tests | | | |
| Functional decode tests | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `reactor.go` | | |
| `loader.go` | | |
| `update_build.go` | | |
| `encode.go` | | |
| `bgp-nlri-mup/helpers.go` | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Goal Gates (MUST pass)
- [ ] Acceptance criteria AC-1..AC-10 all demonstrated
- [ ] Tests pass (`make ze-unit-test`)
- [ ] No regressions (`make ze-functional-test`)
- [ ] Zero direct plugin imports in infrastructure code
- [ ] Zero duplicated MUP helpers
- [ ] No ambiguous same-name functions across packages

### Quality Gates (SHOULD pass)
- [ ] `make ze-lint` passes
- [ ] Implementation Audit fully completed
- [ ] Mistake Log escalation candidates reviewed

### 🏗️ Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit behavior
- [ ] Minimal coupling

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Functional tests verify end-to-end behavior unchanged

### Documentation
- [ ] Required docs read
- [ ] plugin-design.md known violations table updated after fixes

### Completion
- [ ] All Partial/Skipped items have user approval
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-code-hygiene-fixes.md`
- [ ] All files committed together

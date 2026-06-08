# Spec: yang-rename-ownership

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/2 |
| Updated | 2026-06-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/command-ownership.md` - three-directory model, folder test, YANG as data
4. `ai/rules/plugin-self-containment.md` - self-containment invariant
5. `scripts/codegen/yang_glue.go` - existing codegen for yang/ dirs
6. `scripts/codegen/plugin_imports.go` - all.go generation, already handles both schema/ and yang/

## Task

Complete the plugin self-containment refactoring started in the session documented by `handover/11-plugin-self-containment.md`. Three items were requested for follow-up:

1. **Rename `schema/` to `yang/`** across all plugins and components. YANG is declarative data, not code; the folder name should reflect this. ~90 dirs, ~388 files (148 .yang + 240 .go), all Go imports referencing `*/schema` paths must be updated.

2. **Write codegen** to generate `embed.go` + `register.go` from `.yang` files found in `yang/` directories. **Already done:** `scripts/codegen/yang_glue.go` exists and generates for `yang/` dirs. After the rename, it covers everything. The remaining work is to delete the hand-written `embed.go` + `register.go` in renamed dirs and regenerate them.

3. **Move command YANG + RPC handlers from `component/` to `plugins/`** for subsystems whose command surfaces should be removable. The handover identified these as "consider" items: iface, firewall, ping, traceroute, resolve, ike. Additional candidates exist: bfd, aaa, subscriber, storage, gnmi, l2tp, ldp, mpls, pki, pppoe, rsvpte, traffic, flowexport, config/archive.

Origin: `handover/11-plugin-self-containment.md` "Remaining" section. The handover can be deleted after this spec is written.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/command-ownership.md` - three-directory model, folder test, YANG as data, codegen, container merge
  → Decision: YANG files go in `yang/` not `schema/`. Config YANG stays in component/. Command YANG goes in plugins/.
  → Constraint: folder test must hold: copy plugin in + codegen = works, delete + codegen = gone.
- [ ] `ai/rules/plugin-self-containment.md` - self-containment invariant, central guard + owner presence tests
  → Constraint: central verb anchors (show, monitor, clear, delete, set, update) stay in component/cmd/
- [ ] `ai/rules/plugin-design.md` - component vs plugin placement rule
  → Decision: command surfaces belong in plugins/, implementations in component/

**Key insights:**
- `scripts/codegen/yang_glue.go` already generates embed.go + register.go for yang/ dirs (item 2 is done)
- `scripts/codegen/plugin_imports.go` already handles both schema/ and yang/ dirs (line 201-203)
- 15 plugins already have yang/ dirs with cmd YANG (migrated in prior session)
- 16 plugins still have schema/ dirs with only conf YANG (rename only, no move needed)
- ~90 schema/ dirs across component/, plugins/, core/, test/ need renaming
- 38 test files in schema/ dirs must move with the rename
- Central verb anchors (cmd/show, cmd/clear, etc.) stay in component/cmd/ but rename schema/ to yang/
- `configyang.RegisterModule()` at `internal/component/config/yang/register.go:16` is the YANG registration entry point

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/codegen/yang_glue.go` - scans `yang/` dirs for .yang files, generates embed.go + register.go
- [ ] `scripts/codegen/plugin_imports.go` - scans both schema/ and yang/ for register.go with yang imports
- [ ] `internal/component/plugin/all/all.go` - generated blank imports for plugins, schemas, RPCs, namespaces
- [ ] `docs/architecture/command-ownership.md` - documents the target architecture

**Behavior to preserve:**
- All YANG schemas load and register identically after rename
- All CLI commands work unchanged
- All self-containment tests pass (central guards + owner presence)
- Codegen (`make generate`) produces identical all.go imports (modulo schema->yang path change)
- `yang_glue.go --check` passes after regeneration

**Behavior to change:**
- All `schema/` directories containing .yang files renamed to `yang/`
- All Go import paths updated from `*/schema` to `*/yang`
- Hand-written embed.go + register.go in renamed dirs replaced by codegen output
- Command YANG for Phase 2 subsystems moved from component/ to plugins/

## Data Flow (MANDATORY)

### Entry Point
- YANG files loaded at startup via `init()` in `register.go` -> `configyang.RegisterModule()`
- Codegen scans filesystem for `yang/` dirs -> generates Go glue -> `make generate` rewires all.go

### Transformation Path
1. `.yang` file exists in `yang/` directory
2. `yang_glue.go` generates `embed.go` (//go:embed vars) and `register.go` (RegisterModule calls)
3. `plugin_imports.go` discovers `register.go` files importing `config/yang` -> adds to all.go
4. `all.go` blank-imports trigger `init()` -> YANG modules registered at startup

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Filesystem -> Codegen | yang_glue.go scans for yang/*.yang | [ ] |
| Codegen -> Compilation | Generated embed.go + register.go compiled | [ ] |
| Compilation -> Runtime | init() calls RegisterModule() | [ ] |

### Integration Points
- `configyang.RegisterModule()` - YANG module registration
- `pluginserver.RegisterRPCs()` - RPC handler registration
- `internal/component/plugin/all/all.go` - generated import hub

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `make generate` | -> | yang_glue.go + plugin_imports.go | `go run scripts/codegen/yang_glue.go --check` + `go run scripts/codegen/plugin_imports.go --check` |
| `make ze` | -> | all.go blank imports | compilation succeeds |
| Self-containment tests | -> | YANG module presence | `go test ./internal/.../yang/ -run Containment` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | No `schema/` directory contains `.yang` files | All YANG files live in `yang/` directories |
| AC-2 | `make generate` followed by `make ze` | Clean compilation, no import errors |
| AC-3 | `go run scripts/codegen/yang_glue.go --check` | All yang/ dirs have current generated glue |
| AC-4 | `go run scripts/codegen/plugin_imports.go --check` | all.go is current |
| AC-5 | All self-containment tests pass | `go test ./internal/... -run Containment` |
| AC-6 | `make ze-unit-test` | No regressions |
| AC-7 | `grep -r '"/schema"' --include='*.go' internal/` | Zero hits (no stale import paths) |
| AC-8 | Command YANG for Phase 2 subsystems lives in plugins/, not component/ | `find ./internal/component/<name> -name '*-cmd.yang'` returns empty for each moved subsystem |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Self-containment tests (existing, relocated) | `internal/.../yang/*_test.go` | Owner YANG declares expected commands | |
| Central guard tests (existing, relocated) | `internal/component/cmd/*/yang/*_test.go` | Banned tokens absent from central YANG | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A -- pure refactoring, no numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing functional tests | `test/` | All CLI commands work unchanged after rename | |

### Interop Tests
N/A -- no protocol changes.

### Future
None. Pure refactoring must not defer tests.

## Files to Modify

~90 directories renamed, ~388 files moved, all Go files with `*/schema` imports updated.

### Phase 1: Rename schema/ to yang/

**Directories to rename** (every `schema/` dir containing .yang files):
- All `internal/component/*/schema/` -> `yang/`
- All `internal/component/bgp/plugins/*/schema/` -> `yang/`
- All `internal/component/cmd/*/schema/` -> `yang/`
- All `internal/plugins/*/schema/` -> `yang/`
- All `internal/core/*/schema/` -> `yang/`
- All `internal/test/plugins/*/schema/` -> `yang/`
- `internal/component/config/*/schema/` -> `yang/`

**Go imports to update:**
- Every `.go` file importing a `*/schema` path from a renamed dir

**Codegen to re-run:**
- `yang_glue.go` regenerates embed.go + register.go in all yang/ dirs
- `plugin_imports.go` regenerates all.go with updated import paths

### Phase 2: Move command YANG to plugins/

Subsystems to evaluate (command YANG currently in component/, should move to plugins/):

| Subsystem | Current cmd YANG location | Proposed plugin | Decision needed |
|-----------|--------------------------|-----------------|-----------------|
| iface | component/iface/yang/ | plugins/iface-cmd/ | Intrinsic to component? |
| firewall | component/firewall/yang/ | plugins/firewall-cmd/ | Intrinsic to component? |
| ping | component/ping/yang/ | plugins/ping/ | Standalone tool |
| traceroute | component/traceroute/yang/ | plugins/traceroute/ | Standalone tool |
| resolve | component/resolve/yang/ | plugins/resolve/ | Standalone tool |
| ike | component/ike/yang/ | plugins/ike-cmd/ | Paired with ipsec component |
| bfd | component/bfd/yang/ | Already has plugins/bfd/ | Merge cmd YANG into existing plugin |
| aaa | component/aaa/yang/ | plugins/aaa-cmd/ | Auth subsystem |
| subscriber | component/subscriber/yang/ | plugins/subscriber-cmd/ | PPPoE subsystem |
| storage | component/storage/yang/ | plugins/storage-cmd/ | System utility |
| doctor | component/doctor/yang/ | Keep in component/ | Engine infrastructure (per prior decision) |
| gnmi | component/gnmi/yang/ | plugins/gnmi-cmd/ | Management interface |
| l2tp | component/l2tp/yang/ | plugins/l2tp-cmd/ | Protocol subsystem |
| ldp | component/ldp/yang/ | plugins/ldp-cmd/ | Protocol subsystem |
| mpls | component/mpls/yang/ | plugins/mpls-cmd/ | Protocol subsystem |
| pki | component/pki/yang/ | plugins/pki-cmd/ | Certificate management |
| pppoe | component/pppoe/yang/ | plugins/pppoe-cmd/ | Protocol subsystem |
| rsvpte | component/rsvpte/yang/ | plugins/rsvpte-cmd/ | Protocol subsystem |
| traffic | component/traffic/yang/ | plugins/traffic-cmd/ | QoS subsystem |
| flowexport | component/flowexport/yang/ | plugins/flowexport-cmd/ | Telemetry subsystem |
| config/archive | component/config/archive/yang/ | plugins/config-archive/ | Config subsystem |
| BGP plugins (cmd/*) | component/bgp/plugins/cmd/*/yang/ | Already in bgp plugin tree | Rename only |

**Central verb anchors (KEEP in component/cmd/):**
- cmd/show, cmd/clear, cmd/delete, cmd/log, cmd/meta, cmd/metrics, cmd/monitor, cmd/set, cmd/subscribe, cmd/update
- These hold generic-only commands and banned-token guards. Rename schema/ to yang/ but do not move.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | No new YANG, only moving existing |
| CLI commands/flags | No | No new commands |
| Functional test for new RPC/API | No | No new RPCs |
| Pipe completeness | No | No new output |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/command-ownership.md` - update Summary table if schema/ references remain |
| 16 | Source anchors reference changed files? | Yes | Grep docs for `schema/` references, update to `yang/` |

## Files to Create

- Migration script: `scripts/dev/rename-schema-to-yang.sh` (runs the mechanical rename + import fixup)
- New plugin directories for Phase 2 moves (e.g., `plugins/ping/`, `plugins/traceroute/`, etc.)

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, current schema/ inventory |
| 3. Wiring phase | Write migration script, verify codegen handles yang/ |
| 4. Implement (TDD) | Phase 1: rename, Phase 2: move cmd YANG |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Rename schema/ to yang/ (mechanical)**
   - Write a migration script that: renames dirs, updates Go imports, updates Go package declarations
   - Run `yang_glue.go` to regenerate embed.go + register.go in all yang/ dirs
   - Run `plugin_imports.go` to regenerate all.go
   - Delete any hand-written embed.go + register.go that the codegen now produces
   - Tests: `make ze` compiles, `make ze-unit-test` passes, codegen --check passes
   - Files: ~90 dirs, ~388 files, all .go files with schema imports
   - Verify: AC-1, AC-2, AC-3, AC-4, AC-5, AC-6, AC-7

2. **Phase: Move command YANG to plugins/ (design + execute)**
   - For each subsystem in the Phase 2 table: decide keep vs move (user input needed for "intrinsic" items)
   - For each "move" decision: create plugin dir, move cmd YANG + handlers, update self-containment tests
   - Run codegen, verify compilation + tests
   - Tests: self-containment tests pass, functional tests pass
   - Files: new plugin dirs, updated component dirs, updated all.go
   - Verify: AC-8

3. **Functional tests** -> Existing functional tests cover all CLI commands. No new tests needed for pure refactoring.

4. **Full verification** -> `make ze-verify`

5. **Complete spec** -> Fill audit tables, write learned summary.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every schema/ dir with .yang files renamed to yang/ |
| Correctness | No stale `*/schema` import paths remain in Go files |
| Naming | All yang/ package declarations say `package yang` |
| Data flow | RegisterModule() calls work identically after rename |
| Rule: no-layering | No duplicate schema/ + yang/ dirs for the same package |
| Rule: self-containment | All central guard + owner presence tests pass |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| No schema/ dirs with .yang files | `find ./internal -path '*/schema/*.yang' \| wc -l` = 0 |
| Clean compilation | `make ze` exits 0 |
| Codegen current | `yang_glue.go --check` + `plugin_imports.go --check` exit 0 |
| No stale imports | `grep -r '"/schema"' --include='*.go' internal/ \| wc -l` = 0 |
| Self-containment tests pass | `go test ./internal/... -run Containment` |
| All unit tests pass | `make ze-unit-test` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | N/A -- pure refactoring, no new inputs |
| File operations | Migration script must not delete files, only rename/move |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error after rename | Fix stale import path |
| Self-containment test fails | Fix YANG path in test assertion |
| Codegen --check fails | Re-run codegen |
| Circular import after move | Restructure plugin/component split |
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
| Rename all schema/ to yang/ uniformly | Rename only dirs with cmd YANG, leave conf YANG in schema/ | Consistency: YANG is data regardless of type. The doc says "YANG files go in yang/ not schema/". |
| Write migration script rather than manual rename | Manual rename per dir | ~90 dirs is error-prone manually. Script ensures completeness and is repeatable. |
| Phase 2 decisions deferred to user | Decide all moves unilaterally | Whether iface/firewall command surfaces are "intrinsic" is a design call the user should make |

## Known Limitations
- Phase 2 moves require design decisions for "intrinsic" subsystems (user input)
- BGP plugins already have a deep directory structure; their schema/ dirs rename in place without moving to a separate plugins/ tree

## Implementation Summary

### What Was Implemented
- Phase 1: Renamed 99 schema/ dirs to yang/ (96 simple renames, 3 merges into existing yang/)
- Updated all Go import paths, aliases, package declarations, identifier usages
- Expanded yang_glue.go acronyms map (added MRT, TFTP, AS, RR, PoolStats, AsPath, FlowExport, AAA, IPsec, PPPoE)
- Codegen regenerated for 109 yang/ directories
- Updated string literal path references in tests, docs, scripts
- Fixed 3 variable name mismatches (adj_rib_in API, vpp, fib/vpp)
- Fixed 8 external test package declarations (package schema_test -> yang_test)
- Updated docs/architecture/command-ownership.md Summary table

### Bugs Found/Fixed
- rename script missed package schema_test (external test packages) -- fixed
- rename script missed relative path references (../schema/) -- fixed
- rename script missed scripts/docvalid/ Go imports -- fixed
- rename script missed filepath.Join("schema") calls in scripts/checks/ -- fixed
- yang_glue.go missing acronyms caused codegen/hand-written variable name mismatches -- fixed

### Documentation Updates
- docs/architecture/command-ownership.md: updated Summary table and self-containment test path
- docs/config-reference.md, docs/features.md, docs/research/*: updated schema/ references to yang/

### Deviations from Plan
- config/yang/cli/ (ze schema CLI command) intentionally kept: no .yang files, separate CLI, would need user-facing command rename
- Phase 2 (command YANG moves) not started: requires design decisions on intrinsic subsystems

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
| All YANG files in yang/ dirs | deliverable check | `find ./internal -path '*/schema/*.yang'` returns empty |
| Clean build after rename | build | `make ze` exits 0 |
| No stale imports | grep | `grep -r '"/schema"' --include='*.go' internal/` returns empty |
| Self-containment invariant holds | test | `go test ./internal/... -run Containment` passes |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [pending]

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
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
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
- [ ] Write learned summary to `plan/learned/NNN-yang-rename-ownership.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-yang-rename-ownership.md`

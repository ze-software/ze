# Spec: policy-3-filter-chain

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-policy-1-framework |
| Phase | - |
| Updated | 2026-04-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-policy-0-umbrella.md` - parent umbrella spec, design decisions
4. `internal/component/bgp/config/redistribution.go` - current filter chain assembly
5. `internal/component/bgp/reactor/filter_chain.go` - PolicyFilterChain colon split
6. `internal/component/config/parser.go` - inactive: prefix parsing
7. `internal/component/cli/model.go` - CLI editor commands

## Task

Per-peer filter chain infrastructure for the route policy framework. Implements the `filter { import/export }` container at peer, group, and bgp global levels (replacing `redistribution`), with `ordered-by user` leaf-lists preserving user-specified execution order. Extends the `inactive:` mechanism to individual leaf-list values for deactivating built-in defaults. Adds CLI `insert` and context-aware `delete` commands for ordered leaf-list manipulation. Migrates away from the `redistribution` YANG container and associated Go code (no-layering: full removal).

Child spec of `spec-policy-0-umbrella.md`. See umbrella Design Decisions table for resolved questions.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/architecture/core-design.md` - filter pipeline architecture
- [ ] `docs/architecture/config/syntax.md` - config parsing, YANG schema, inactive: mechanism
- [ ] `plan/spec-policy-0-umbrella.md` - umbrella design decisions
  -> Decision: filter order = user config order, no sorting (umbrella D1, D3)
  -> Decision: delete on built-in = deactivate with inactive: prefix (umbrella D4)
  -> Decision: delete on user-defined = remove (umbrella D4)
  -> Decision: inactive: extended to leaf-list values (umbrella D6)
  -> Decision: no colon split, name is just a name (umbrella D7)
  -> Constraint: no-layering -- redistribution format fully removed (umbrella Migration table)

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - AS loop detection (Section 9) -- relevant to default filter behavior

**Key insights:** TBD

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE moving to design)
<!-- Same rule: never tick [ ] to [x]. Write -> Constraint: annotations instead. -->
- [ ] `internal/component/bgp/config/redistribution.go` - validateFilterRefs, DefaultImportFilters, DefaultExportFilters, applyOverrides, concatFilters
- [ ] `internal/component/bgp/config/redistribution_test.go` - existing unit tests for filter chain
- [ ] `internal/component/bgp/config/peers.go` - filter chain assembly in PeersFromConfigTree
- [ ] `internal/component/bgp/reactor/filter_chain.go` - PolicyFilterChain with colon-split name parsing
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - ingress policy chain wiring
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - egress policy chain wiring
- [ ] `internal/component/bgp/schema/ze-bgp-conf.yang` - redistribution YANG container at bgp/group/peer levels
- [ ] `internal/component/config/parser.go` - inactive: prefix parsing (containers/lists only)
- [ ] `internal/component/config/serialize.go` - inactive: serialization
- [ ] `internal/component/config/tree.go` - tree operations for config editor
- [ ] `internal/component/cli/model.go` - CLI editor commands
- [ ] `test/plugin/redistribution-*.ci` - 6 existing .ci tests to migrate

**Behavior to preserve:**
- TBD (document after reading source files)

**Behavior to change:**
- `redistribution { import/export }` replaced by `filter { import/export }` at bgp/group/peer levels
- `<plugin>:<filter>` colon-split format replaced by plain name lookup
- `DefaultImportFilters`/`DefaultExportFilters` package vars replaced by default auto-population in chain assembly
- `applyOverrides` replaced by user-controlled chain with delete = deactivate for built-ins
- `inactive:` mechanism extended from containers/lists to leaf-list values
- CLI gains `insert` command for ordered leaf-lists
- CLI `delete` on leaf-list values becomes context-aware (built-in vs user-defined)

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Config file: `peer { filter { import [ no-self-as reject-bogons ]; } }` parsed by config parser
- CLI editor: `insert filter import reject-bogons before no-self-as` command
- CLI editor: `delete filter import no-self-as` command (deactivates built-in)

### Transformation Path
1. YANG validation -- `filter { import/export }` containers with `ordered-by user` leaf-lists
2. Config parser -- preserves order, handles `inactive:` prefix on individual leaf-list values
3. Config tree -- stores ordered list with inactive markers
4. `PeersFromConfigTree` -- resolves names against filter registry, auto-populates defaults, builds validated chain
5. Reactor startup -- receives validated filter chain, no colon split needed

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config parser -> Config tree | Ordered leaf-list values with inactive: prefix | [ ] |
| Config tree -> PeersFromConfigTree | Name resolution against filter registry | [ ] |
| PeersFromConfigTree -> Reactor | Validated chain in PeerSettings | [ ] |
| CLI editor -> Config tree | insert/delete commands modify ordered leaf-list | [ ] |

### Integration Points
- TBD (document after reading source files)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `filter { import [ name ] }` | -> | YANG parse + ordered leaf-list | TBD |
| Config with `filter { import [ inactive:name ] }` | -> | inactive: on leaf-list value | TBD |
| CLI `insert filter import name before existing` | -> | ordered insert in config tree | TBD |
| CLI `delete filter import built-in-name` | -> | deactivate (inactive: prefix) | TBD |
| CLI `delete filter import user-name` | -> | remove from leaf-list | TBD |
| Config with defaults auto-populated | -> | default chain visible in show config | TBD |
| Migrated redistribution .ci tests | -> | new filter format | TBD |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `filter { import [ a b c ]; }` in peer config | Filters parsed and stored in user-specified order a, b, c |
| AC-2 | `filter { export [ x y ]; }` in peer config | Export chain parsed separately from import chain |
| AC-3 | `filter { import/export }` at bgp global level | Inherited by peers without explicit filter config |
| AC-4 | `filter { import/export }` at group level | Inherited by peers in group without explicit filter config |
| AC-5 | `ordered-by user` YANG annotation on import/export leaf-lists | Order preserved through parse, serialize, and edit round-trip |
| AC-6 | `filter { import [ inactive:no-self-as ]; }` in config | inactive: prefix parsed on leaf-list value, filter skipped at execution |
| AC-7 | Serialize config with inactive leaf-list value | `inactive:` prefix preserved in output |
| AC-8 | CLI `insert filter import reject-bogons first` | Entry added at beginning of import list |
| AC-9 | CLI `insert filter import reject-bogons last` | Entry added at end of import list |
| AC-10 | CLI `insert filter import new-filter before existing-filter` | Entry inserted before the named existing entry |
| AC-11 | CLI `insert filter import new-filter after existing-filter` | Entry inserted after the named existing entry |
| AC-12 | CLI `delete` on a built-in default filter | Sets `inactive:` prefix, filter remains in list but skipped |
| AC-13 | CLI `delete` on a user-defined filter | Removes entry from the leaf-list |
| AC-14 | CLI `activate` on an `inactive:` prefixed entry | Removes `inactive:` prefix, filter runs again |
| AC-15 | Peer with no explicit filter config | Built-in defaults (e.g., `no-self-as`) auto-populated in filter chain |
| AC-16 | `show config` on peer with defaults | Defaults visible in output |
| AC-17 | `redistribution` YANG container | Fully removed from bgp, group, and peer levels |
| AC-18 | `DefaultImportFilters`, `DefaultExportFilters` package vars | Removed from redistribution.go |
| AC-19 | `applyOverrides` function | Removed from redistribution.go |
| AC-20 | Colon-split in `PolicyFilterChain` | Removed, names used directly |
| AC-21 | 6 existing redistribution .ci tests | Migrated to use new `filter { import/export }` format, all pass |
| AC-22 | `insert` on a non-ordered leaf-list | Command rejected with error |
| AC-23 | `insert` with `before`/`after` referencing nonexistent entry | Command rejected with error |
| AC-24 | `insert` with duplicate entry name | Command rejected with error |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFilterLeafListOrderPreserved` | TBD | AC-1, AC-5: order preserved through parse/serialize | |
| `TestFilterInactiveLeafListValue` | TBD | AC-6, AC-7: inactive: prefix on leaf-list values | |
| `TestFilterDefaultAutoPopulation` | TBD | AC-15, AC-16: built-in defaults auto-populated | |
| `TestFilterInsertFirst` | TBD | AC-8: insert at beginning | |
| `TestFilterInsertLast` | TBD | AC-9: insert at end | |
| `TestFilterInsertBefore` | TBD | AC-10: insert before existing | |
| `TestFilterInsertAfter` | TBD | AC-11: insert after existing | |
| `TestFilterDeleteBuiltIn` | TBD | AC-12: delete on built-in sets inactive: | |
| `TestFilterDeleteUserDefined` | TBD | AC-13: delete on user-defined removes | |
| `TestFilterActivate` | TBD | AC-14: activate removes inactive: prefix | |
| `TestFilterInsertNonOrdered` | TBD | AC-22: insert rejected on non-ordered leaf-list | |
| `TestFilterInsertBeforeNonexistent` | TBD | AC-23: insert before/after nonexistent rejected | |
| `TestFilterInsertDuplicate` | TBD | AC-24: duplicate entry rejected | |
| `TestFilterChainNoColonSplit` | TBD | AC-20: name used directly, no colon parsing | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A -- no numeric inputs in this spec | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `filter-ordered-list` | `test/editor/filter-ordered-list.et` | CLI insert/delete on ordered filter leaf-list | |
| `filter-inactive-leaflist` | `test/parse/filter-inactive-leaflist.ci` | Config with inactive: on leaf-list value parses correctly | |
| Migrated redistribution tests (6) | `test/plugin/redistribution-*.ci` | Existing tests updated to filter format | |

### Future (if deferring any tests)
- TBD

## Files to Modify

- `internal/component/bgp/schema/ze-bgp-conf.yang` - replace `redistribution` container with `filter` container, add `ordered-by user` annotation on import/export leaf-lists
- `internal/component/bgp/config/redistribution.go` - remove `validateFilterRefs`, `DefaultImportFilters`, `DefaultExportFilters`, `applyOverrides`, `concatFilters`
- `internal/component/bgp/config/redistribution_test.go` - update or replace tests for new filter chain
- `internal/component/bgp/config/peers.go` - new filter chain assembly with default auto-population and name resolution
- `internal/component/bgp/reactor/filter_chain.go` - remove colon-split, use name-based lookup directly
- `internal/component/bgp/reactor/reactor_notify.go` - update ingress policy chain wiring if needed
- `internal/component/bgp/reactor/reactor_api_forward.go` - update egress policy chain wiring if needed
- `internal/component/config/parser.go` - extend inactive: prefix parsing to leaf-list values
- `internal/component/config/serialize.go` - extend inactive: serialization to leaf-list values
- `internal/component/config/tree.go` - ordered leaf-list operations (insert first/last/before/after)
- `internal/component/cli/model.go` - add `insert` command, update `delete` for context-aware behavior on leaf-list values, add `activate` command

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | `internal/component/bgp/schema/ze-bgp-conf.yang` |
| CLI commands/flags | [ ] | `internal/component/cli/model.go` |
| Editor autocomplete | [ ] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [ ] | `test/editor/filter-ordered-list.et`, `test/parse/filter-inactive-leaflist.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `.claude/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/rfcNNNN.md` |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` or subsystem doc |

## Files to Create

- `test/editor/filter-ordered-list.et` - CLI insert/delete test for ordered filter leaf-lists
- `test/parse/filter-inactive-leaflist.ci` - inactive: on leaf-list value parse test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: YANG schema** -- replace `redistribution` container with `filter { import/export }` at bgp/group/peer levels, add `ordered-by user` annotation
   - Tests: `TestFilterLeafListOrderPreserved`
   - Files: `ze-bgp-conf.yang`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: inactive: leaf-list extension** -- extend inactive: prefix parsing and serialization to individual leaf-list values
   - Tests: `TestFilterInactiveLeafListValue`
   - Files: `parser.go`, `serialize.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: ordered leaf-list tree operations** -- implement insert first/last/before/after and context-aware delete in config tree
   - Tests: `TestFilterInsertFirst`, `TestFilterInsertLast`, `TestFilterInsertBefore`, `TestFilterInsertAfter`, `TestFilterInsertNonOrdered`, `TestFilterInsertBeforeNonexistent`, `TestFilterInsertDuplicate`
   - Files: `tree.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: CLI commands** -- add `insert`, update `delete` for leaf-list context-awareness, add `activate`
   - Tests: `TestFilterDeleteBuiltIn`, `TestFilterDeleteUserDefined`, `TestFilterActivate`
   - Files: `model.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: filter chain assembly** -- replace redistribution code with new filter chain assembly, default auto-population, name resolution
   - Tests: `TestFilterDefaultAutoPopulation`, `TestFilterChainNoColonSplit`
   - Files: `redistribution.go`, `peers.go`, `filter_chain.go`
   - Verify: tests fail -> implement -> tests pass

6. **Phase: migration** -- remove redistribution YANG/Go code, migrate 6 .ci tests
   - Tests: migrated redistribution .ci tests
   - Files: `ze-bgp-conf.yang`, `redistribution.go`, `redistribution_test.go`, `filter_chain.go`
   - Verify: all 6 migrated tests pass

7. **Functional tests** -- create editor and parse functional tests
   - Files: `test/editor/filter-ordered-list.et`, `test/parse/filter-inactive-leaflist.ci`
   - Verify: functional tests pass

8. **Full verification** -- `make ze-verify`

9. **Complete spec** -- fill audit tables, write learned summary to `plan/learned/NNN-policy-3-filter-chain.md`, delete spec from `plan/`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-24 has implementation with file:line |
| Correctness | Insert before/after positions correct, inactive: prefix round-trips, built-in vs user-defined delete distinction works |
| Naming | YANG uses kebab-case for containers/leaf-lists, CLI commands match YANG names |
| Data flow | Filter names resolved in PeersFromConfigTree only, reactor receives validated chain |
| Rule: no-layering | redistribution YANG container fully removed, DefaultImportFilters/DefaultExportFilters/applyOverrides deleted, colon-split deleted |
| Rule: ordered-by user | Leaf-list order preserved through parse -> tree -> serialize -> edit round-trip |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `filter` YANG container at bgp/group/peer levels | grep `ze-bgp-conf.yang` for `filter` container |
| `ordered-by user` on import/export leaf-lists | grep `ze-bgp-conf.yang` for `ordered-by user` |
| `inactive:` on leaf-list values in parser | grep `parser.go` for inactive leaf-list handling |
| `inactive:` on leaf-list values in serializer | grep `serialize.go` for inactive leaf-list handling |
| CLI `insert` command | grep `model.go` for insert command |
| CLI `activate` command | grep `model.go` for activate command |
| `redistribution` YANG container removed | grep `ze-bgp-conf.yang` confirms no `redistribution` container |
| `DefaultImportFilters`/`DefaultExportFilters` removed | grep `redistribution.go` confirms removal |
| `applyOverrides` removed | grep `redistribution.go` confirms removal |
| Colon-split removed from PolicyFilterChain | grep `filter_chain.go` confirms no colon split |
| 6 .ci tests migrated | ls `test/plugin/redistribution-*.ci` + all pass |
| `test/editor/filter-ordered-list.et` exists | ls the file |
| `test/parse/filter-inactive-leaflist.ci` exists | ls the file |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Filter names in leaf-list validated against registry; reject unknown names at parse time |
| Insert position | `before`/`after` target validated to exist; reject nonexistent references |
| Duplicate prevention | Duplicate filter names in same chain rejected |
| inactive: prefix injection | Ensure `inactive:` prefix cannot be used to bypass mandatory filters (OTC stays in-process, not in named system) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
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
<!-- LIVE -- write IMMEDIATELY when you learn something -->
<!-- Route at completion: subsystem -> arch doc, process -> rules, knowledge -> memory.md -->

## RFC Documentation

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer constraints, message ordering, any MUST/MUST NOT.

## Implementation Summary

### What Was Implemented
- TBD

### Bugs Found/Fixed
- TBD

### Documentation Updates
- TBD

### Deviations from Plan
- TBD

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->
<!-- For each item: run a command (grep, ls, go test -run) and paste the evidence. -->
<!-- Hook pre-commit-spec-audit.sh (exit 2) checks this section exists and is filled. -->

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1 through AC-24 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-policy-3-filter-chain.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.

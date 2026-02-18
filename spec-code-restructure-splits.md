# Spec: code-restructure-splits

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `code-restructure.md` - full restructuring report with per-file split plans
4. `code-restructure.md` sections 0a and 0b - MANDATORY tool verification and safe-move protocol

## Task

Split 6 large Go files into smaller single-responsibility files within the same
package. Each split is a pure mechanical refactor — no code changes, no new exports,
no renamed symbols. This spec MUST be executed AFTER spec-code-hygiene-fixes is
complete (plugin import violations and MUP duplication fixed first).

The full split plans, declaration inventories, and verification protocols are in
`code-restructure.md` (repository root). This spec provides the tracking structure;
the report provides the execution details.

### Files to Split

| File | Current lines | Target files | Report section |
|------|--------------|--------------|----------------|
| `internal/plugins/bgp/reactor/reactor.go` | 5,390 | 7 new files | Section 1 |
| `cmd/ze/bgp/decode.go` | 1,928 | 3 new files | Section 2 |
| `internal/plugin/server.go` | 1,425 | 4 new files | Section 3 |
| `internal/config/loader.go` | 1,689 | 3 new files | Section 4 |
| `internal/plugins/bgp/reactor/peer.go` | 2,679 | 2 new files (optional) | Section 5 |
| `cmd/ze-chaos/main.go` | 1,296 | 2 new files | Section 6 |

### Additional Work

- Investigate 5 large files listed in report section 7 (read, assess, split if warranted)
- Add `doc.go` index files to 5 directories per report section 8

## Required Reading

### Architecture Docs
- [ ] `code-restructure.md` - the restructuring report this spec tracks
  - Decision: all splits are within-package, same directory, same package line
  - Constraint: sections 0a (tool verification) and 0b (safe-move protocol) are BLOCKING
- [ ] `.claude/rules/plugin-design.md` - plugin import rules (relevant for reactor.go split)
  - Constraint: do not add new direct plugin imports when splitting

**Key insights:**
- Go compiles all .go files in a directory as one unit — within-package splits have zero API risk
- The report's declaration inventories may be stale — section 0a requires tool-based re-extraction
- `goimports` handles import block adjustment automatically

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE splitting each file)
- [ ] `internal/plugins/bgp/reactor/reactor.go` - BGP reactor orchestration + API adapter + wire encoding + routes + config parsing
- [ ] `cmd/ze/bgp/decode.go` - CLI decode dispatch + plugin invocation + wire parsing + JSON envelope
- [ ] `internal/plugin/server.go` - plugin lifecycle + 5-stage protocol + RPC dispatch + subscriptions + capabilities
- [ ] `internal/config/loader.go` - config entry points + route conversions + FlowSpec parsing + MUP encoding
- [ ] `internal/plugins/bgp/reactor/peer.go` - peer session + route converters + next-hop resolution
- [ ] `cmd/ze-chaos/main.go` - CLI dispatch + simulation orchestration + post-analysis sub-commands

**Behavior to preserve:**
- Every exported symbol name, type, and signature unchanged
- Every test passes with identical results
- No code modifications — only file moves and import adjustments

**Behavior to change:**
- None. This is a pure structural refactor.

## Data Flow (MANDATORY)

### Entry Point
- N/A — no data flow changes. This is a file-level structural refactor within existing packages.

### Transformation Path
1. Read source file declarations using grep (section 0a)
2. Group declarations by concern
3. Move declaration blocks to new files verbatim (section 0b)
4. Adjust import blocks via goimports
5. Verify zero mutation (compile, test, export diff)

### Integration Points
- No new integration points. All code remains in the same package with the same API.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None | No boundaries crossed — same-package refactor | [ ] |

### Architectural Verification
- [ ] No bypassed layers (no behavioral change)
- [ ] No unintended coupling (no new imports between packages)
- [ ] No duplicated functionality (code moves, not copies)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `wc -l internal/plugins/bgp/reactor/reactor.go` | Below 600 lines |
| AC-2 | `wc -l cmd/ze/bgp/decode.go` | Below 600 lines |
| AC-3 | `wc -l internal/plugin/server.go` | Below 600 lines |
| AC-4 | `wc -l internal/config/loader.go` | Below 600 lines |
| AC-5 | `make ze-unit-test` | All tests pass |
| AC-6 | `make ze-functional-test` | All tests pass |
| AC-7 | `make chaos-unit-test` | All tests pass |
| AC-8 | Exported symbol count before vs after (per section 0b) | Identical |
| AC-9 | `ls internal/plugin/doc.go internal/plugins/bgp/doc.go internal/plugins/bgp/reactor/doc.go internal/config/doc.go cmd/ze-chaos/doc.go` | All 5 exist |
| AC-10 | Each new file has a one-line role comment before the package line | Verified by reading first line |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| All existing reactor tests | `internal/plugins/bgp/reactor/*_test.go` | Reactor behavior unchanged | |
| All existing plugin tests | `internal/plugin/*_test.go` | Plugin server behavior unchanged | |
| All existing config tests | `internal/config/*_test.go` | Config loading unchanged | |
| All existing decode tests | `cmd/ze/bgp/*_test.go` | CLI decode unchanged | |
| All existing chaos tests | `cmd/ze-chaos/**/*_test.go` | Chaos tool unchanged | |

### Boundary Tests (MANDATORY for numeric inputs)

N/A — no new numeric inputs; this is a structural refactor.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing encode tests | `test/encode/*.ci` | Wire encoding identical | |
| Existing plugin tests | `test/plugin/*.ci` | Plugin communication identical | |
| Existing decode tests | `test/decode/*.ci` | Decoding identical | |

## Files to Modify

- `internal/plugins/bgp/reactor/reactor.go` - shrink to lifecycle only
- `cmd/ze/bgp/decode.go` - shrink to CLI dispatch only
- `internal/plugin/server.go` - shrink to lifecycle only
- `internal/config/loader.go` - shrink to entry points only
- `internal/plugins/bgp/reactor/peer.go` - optional: shrink to core session only
- `cmd/ze-chaos/main.go` - shrink to CLI dispatch only

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] No | |
| CLI commands/flags | [ ] No | |
| Plugin SDK docs | [ ] No | |
| Functional test for new RPC/API | [ ] No — existing tests validate | |

## Files to Create

**reactor.go split (section 1):**
- `internal/plugins/bgp/reactor/reactor_wire.go` - wire attribute encoding
- `internal/plugins/bgp/reactor/reactor_routes.go` - route type converters + RIB builders
- `internal/plugins/bgp/reactor/reactor_api.go` - API adapter core dispatch
- `internal/plugins/bgp/reactor/reactor_api_families.go` - per-family announce/withdraw
- `internal/plugins/bgp/reactor/reactor_api_transactions.go` - transactions, watchdog, messages
- `internal/plugins/bgp/reactor/reactor_config.go` - config tree to peer settings

**decode.go split (section 2):**
- `cmd/ze/bgp/decode_plugin.go` - plugin invocation strategies
- `cmd/ze/bgp/decode_wire.go` - wire format parsing
- `cmd/ze/bgp/decode_envelope.go` - JSON envelope and family helpers

**server.go split (section 3):**
- `internal/plugin/server_protocol.go` - 5-stage registration protocol
- `internal/plugin/server_rpc.go` - RPC dispatch
- `internal/plugin/server_subscriptions.go` - subscription management
- `internal/plugin/server_capabilities.go` - capability aggregation

**loader.go split (section 4):**
- `internal/config/loader_routes.go` - route conversions + extended community
- `internal/config/loader_flowspec.go` - FlowSpec text parsing
- `internal/config/loader_mup.go` - MUP NLRI encoding

**peer.go split (section 5, optional):**
- `internal/plugins/bgp/reactor/peer_routes.go` - route type converters
- `internal/plugins/bgp/reactor/peer_nexthop.go` - next-hop resolution

**ze-chaos main.go split (section 6):**
- `cmd/ze-chaos/main_orchestrate.go` - simulation orchestration
- `cmd/ze-chaos/main_analysis.go` - post-simulation analysis sub-commands

**doc.go index files (section 8):**
- `internal/plugin/doc.go`
- `internal/plugins/bgp/doc.go`
- `internal/plugins/bgp/reactor/doc.go`
- `internal/config/doc.go`
- `cmd/ze-chaos/doc.go`

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

**BLOCKING: Follow sections 0a and 0b of `code-restructure.md` for EVERY split.**

**BLOCKING: Do NOT commit during the split work.** All splits must remain uncommitted so
that a full critical review can be performed with another model after the work is done.
Commit only after the review is complete and approved.

1. **Verify spec-code-hygiene-fixes is complete** — plugin imports and MUP duplication must be resolved first
   - Review: Are AC-1 through AC-10 of that spec all demonstrated?

2. **Split reactor.go** (report section 1) — largest file, highest impact
   - Follow section 0a: grep all declarations, group by concern, compare with report
   - Follow section 0b: capture baseline, move verbatim, verify zero mutation
   - Run `make ze-unit-test` after completion
   - Review: reactor.go below 600 lines? All 6 new files have role comments?

3. **Split decode.go** (report section 2)
   - Follow 0a + 0b protocol
   - Run `make ze-unit-test`
   - Review: decode.go below 600 lines?

4. **Split server.go** (report section 3)
   - Follow 0a + 0b protocol
   - Run `make ze-unit-test`
   - Review: server.go below 600 lines?

5. **Split loader.go** (report section 4)
   - Follow 0a + 0b protocol
   - Run `make ze-unit-test && make ze-functional-test`
   - Review: loader.go below 600 lines?

6. **Assess and optionally split peer.go** (report section 5)
   - Read file, decide if split warranted
   - If yes: follow 0a + 0b protocol
   - Run `make ze-unit-test`

7. **Split ze-chaos main.go** (report section 6)
   - Follow 0a + 0b protocol
   - Run `make chaos-unit-test`

8. **Investigate additional large files** (report section 7)
   - Read each, list declarations, decide split or not
   - Document decision for each

9. **Add doc.go files** (report section 8)
   - Use templates from report section 8
   - Verify compile: `go build ./...`

10. **Full verification**
    - `make ze-lint && make ze-unit-test && make ze-functional-test && make chaos-unit-test`
    - Review: zero regressions? All ACs met?

### Failure Routing

| Failure | Symptom | Route To |
|---------|---------|----------|
| Compilation error after split | `go build` fails | Missing import in new file — run `goimports -w` on both files |
| Test failure after split | Test references moved function | Check if test file needs updated import (shouldn't for same-package) |
| Export diff mismatch | Before/after symbol lists differ | A declaration was accidentally modified — discard and redo |
| File still too large after split | Over 600 lines | Review declaration groupings — further split needed |

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
| Split reactor.go | | | |
| Split decode.go | | | |
| Split server.go | | | |
| Split loader.go | | | |
| Assess peer.go | | | |
| Split ze-chaos main.go | | | |
| Investigate 5 additional large files | | | |
| Add 5 doc.go files | | | |

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
| Existing plugin tests | | | |
| Existing config tests | | | |
| Existing decode tests | | | |
| Existing chaos tests | | | |
| Functional encode tests | | | |
| Functional plugin tests | | | |
| Functional decode tests | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `reactor_wire.go` | | |
| `reactor_routes.go` | | |
| `reactor_api.go` | | |
| `reactor_api_families.go` | | |
| `reactor_api_transactions.go` | | |
| `reactor_config.go` | | |
| `decode_plugin.go` | | |
| `decode_wire.go` | | |
| `decode_envelope.go` | | |
| `server_protocol.go` | | |
| `server_rpc.go` | | |
| `server_subscriptions.go` | | |
| `server_capabilities.go` | | |
| `loader_routes.go` | | |
| `loader_flowspec.go` | | |
| `loader_mup.go` | | |
| `main_orchestrate.go` | | |
| `main_analysis.go` | | |
| 5 `doc.go` files | | |

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
- [ ] Feature code integrated into codebase (`internal/*`, `cmd/*`)

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
- [ ] Existing tests still pass (pure structural refactor — no new tests, no code changes)
- [ ] Functional tests verify end-to-end behavior unchanged

### Documentation
- [ ] Required docs read
- [ ] code-restructure.md sections 0a and 0b followed for every split

### Completion
- [ ] All Partial/Skipped items have user approval
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-code-restructure-splits.md`
- [ ] All files committed together

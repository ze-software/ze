# Spec: cli-grammar

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-iface-rate (for interface rate commands only) |
| Phase | 1/11 |
| Updated | 2026-05-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/cli-grammar.md` - the rule being enforced (already created)
4. `internal/component/cmd/show/show.go:627` - handleShowInterface dispatch
5. `internal/component/iface/cmd/clear.go` - handleClearInterfaceCounters dispatch
6. `internal/component/bgp/plugins/cmd/cache/cache.go:36` - handleBgpCache dispatch
7. `internal/component/bgp/plugins/cmd/commit/commit.go:78` - handleCommit dispatch

## Task

Audit and fix ALL CLI commands to enforce the grammar rule: action keyword before
identifier. The first token after the noun must always be a keyword (action), never
a user-supplied identifier (name, address, ID). This eliminates the entire class of
ambiguity where an identifier could collide with a keyword.

Additionally, identifiers that are conventionally numeric (cache IDs, etc.) must be
string-typed at the CLI layer.

The grammar rule is documented in `.claude/rules/cli-grammar.md` (already created).
This spec covers the code changes to enforce it across all existing handlers.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/cli-patterns.md` - CLI dispatch patterns
- [ ] `ai/patterns/cli-command.md` - CLI command registration pattern
- [ ] `ai/rules/derive-not-hardcode.md` - keyword lists must be derived
- [ ] `.claude/rules/cli-grammar.md` - the grammar rule (already created)

### Existing Code
- [ ] `internal/component/cmd/show/show.go:627` - handleShowInterface dispatch logic
- [ ] `internal/component/iface/cmd/clear.go` - handleClearInterfaceCounters dispatch logic
- [ ] `internal/component/bgp/plugins/cmd/cache/cache.go:36` - handleBgpCache dispatch logic
- [ ] `internal/component/bgp/plugins/cmd/commit/commit.go:78` - handleCommit dispatch logic
- [ ] `internal/component/iface/schema/ze-iface-cmd.yang` - YANG command tree for interface

**Key insights:**
- handleShowInterface dispatches on args[0]: brief, type, errors are keywords, anything else is treated as interface name
- handleClearInterfaceCounters documents the keyword/name ambiguity explicitly (lines 37-46)
- handleBgpCache: args[0] is "list" (keyword) or cache ID (numeric), then args[1] is action
- handleCommit: args[0] is "list" (keyword) or commit name (string), then args[1] is action
- Cache IDs are uint64 (cache.go:69) but should be string at the CLI layer
- Grammar rule: `<verb> <noun> <action> [<identifier>]` eliminates the ambiguity class universally

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cmd/show/show.go:627` - show interface dispatches: brief, type, errors, <name>, <name> counters
- [ ] `internal/component/iface/cmd/clear.go` - clear interface dispatches: counters, <name>, <name> counters
- [ ] `internal/component/bgp/plugins/cmd/cache/cache.go:36` - cache dispatches: list, <id> retain/release/expire/forward
- [ ] `internal/component/bgp/plugins/cmd/commit/commit.go:78` - commit dispatches: list, <name> start/end/eor/rollback/show/withdraw

**Behavior to preserve:**
- All existing subcommands continue to function (with deprecation warnings for old grammar)
- YANG-driven autocomplete continues to work
- Keyword-first commands (already correct) are unchanged

**Behavior to change:**

### interface commands
- `show interface <name>` becomes `show interface detail <name>` (old form accepted with deprecation warning)
- `show interface <name> counters` becomes `show interface counters <name>` (old form accepted with deprecation warning)
- `clear interface <name> counters` becomes `clear interface counters <name>` (old form accepted with deprecation warning)
- `clear interface <name>` (bare, no action keyword) requires explicit action

### cache commands
- `cache <id> retain` becomes `cache retain <id>` (old form accepted with deprecation warning)
- `cache <id> release` becomes `cache release <id>` (old form accepted with deprecation warning)
- `cache <id> expire` becomes `cache expire <id>` (old form accepted with deprecation warning)
- `cache <id> forward <sel>` becomes `cache forward <id> <sel>` (old form accepted with deprecation warning)
- Cache ID accepted as string, parsed to uint64 internally only when needed

### commit commands
- `commit <name> start` becomes `commit start <name>` (old form accepted with deprecation warning)
- `commit <name> end` becomes `commit end <name>` (old form accepted with deprecation warning)
- `commit <name> eor` becomes `commit eor <name>` (old form accepted with deprecation warning)
- `commit <name> rollback` becomes `commit rollback <name>` (old form accepted with deprecation warning)
- `commit <name> show` becomes `commit show <name>` (old form accepted with deprecation warning)
- `commit <name> withdraw route <pfx>` becomes `commit withdraw <name> route <pfx>` (old form accepted with deprecation warning)

## Data Flow (MANDATORY)

### Entry Point
- CLI command input string parsed into verb + tokens by the CLI framework
- Tokens dispatched to handler via YANG RPC wire method

### Transformation Path
1. User types command in CLI (SSH or TUI)
2. CLI framework parses command, matches YANG RPC wire method
3. Handler receives args[] after the wire method prefix
4. Handler dispatches on args[0] as action keyword
5. Handler uses args[1] (if present) as interface name

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI input -> handler | YANG RPC dispatch | [ ] |
| Handler -> iface package | GetInterface, ListInterfaces, ResetCounters | [ ] |

### Integration Points
- YANG command tree defines valid completions
- Handler dispatch logic in show.go and clear.go

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Known Violations

### show interface

| Current grammar | Problem | New grammar |
|-----------------|---------|-------------|
| `show interface <name>` | First arg is interface name, collides with keywords | `show interface detail <name>` |
| `show interface <name> counters` | Interface name before keyword | `show interface counters <name>` |
| `show interface brief` | Already correct (keyword first) | No change |
| `show interface errors` | Already correct (keyword first) | No change |
| `show interface type <type>` | Already correct (keyword first) | No change |
| `show interface rate [<name>]` | Correct (added by spec-iface-rate) | No change |

### clear interface

| Current grammar | Problem | New grammar |
|-----------------|---------|-------------|
| `clear interface <name> counters` | Interface name before keyword | `clear interface counters <name>` |
| `clear interface counters` | Already correct (keyword first) | No change |
| `clear interface <name>` | Ambiguous: is `<name>` a keyword or interface? | `clear interface counters <name>` (explicit action required) |

### monitor interface

| Current grammar | Problem | New grammar |
|-----------------|---------|-------------|
| `monitor interface rate [<name>]` | Correct (added by spec-iface-rate) | No change |

### cache (BGP)

| Current grammar | Problem | New grammar |
|-----------------|---------|-------------|
| `cache <id> retain` | ID before action, `args[0]` ambiguous with `list` | `cache retain <id>` |
| `cache <id> release` | ID before action | `cache release <id>` |
| `cache <id> expire` | ID before action | `cache expire <id>` |
| `cache <id> forward <sel>` | ID before action | `cache forward <id> <sel>` |
| `cache list` | Already correct (keyword first) | No change |
| Cache ID type | `uint64` at CLI layer | String at CLI, parse internally |

### commit (BGP)

| Current grammar | Problem | New grammar |
|-----------------|---------|-------------|
| `commit <name> start` | Name before action, `args[0]` ambiguous with `list` | `commit start <name>` |
| `commit <name> end` | Name before action | `commit end <name>` |
| `commit <name> eor` | Name before action | `commit eor <name>` |
| `commit <name> rollback` | Name before action | `commit rollback <name>` |
| `commit <name> show` | Name before action | `commit show <name>` |
| `commit <name> withdraw route <pfx>` | Name before action | `commit withdraw <name> route <pfx>` |
| `commit list` | Already correct (keyword first) | No change |

### Full audit (to be completed during implementation)

All handlers in `internal/component/cmd/` and `internal/component/*/cmd/` must be
audited for `args[0]` dispatch where the value could be either a keyword or identifier.
The explorer agent audit results will populate this section.

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `show interface detail eth0` | -> | handleShowInterface new dispatch | TestShowInterface_DetailByName |
| `show interface counters eth0` | -> | handleShowInterface new dispatch | TestShowInterface_CountersByName |
| `show interface eth0` (deprecated) | -> | handleShowInterface backward compat | TestShowInterface_DeprecatedNameFirst |
| `clear interface counters eth0` | -> | handleClearInterfaceCounters new dispatch | TestClearInterface_CountersByName |
| `cache retain <id>` | -> | handleBgpCache new dispatch | TestBgpCache_ActionFirst |
| `cache <id> retain` (deprecated) | -> | handleBgpCache backward compat | TestBgpCache_DeprecatedIdFirst |
| `commit start <name>` | -> | handleCommit new dispatch | TestCommit_ActionFirst |
| `commit <name> start` (deprecated) | -> | handleCommit backward compat | TestCommit_DeprecatedNameFirst |
| Grammar rule file | -> | .claude/rules/cli-grammar.md exists | (file already created) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show interface detail eth0` | Shows full detail for eth0 |
| AC-2 | `show interface counters eth0` | Shows counters for eth0 |
| AC-3 | `show interface eth0` (old grammar) | Works with deprecation warning |
| AC-4 | `show interface eth0 counters` (old grammar) | Works with deprecation warning |
| AC-5 | `show interface detail` (no name) | Shows all interfaces in detail |
| AC-6 | `clear interface counters eth0` | Clears counters for eth0 |
| AC-7 | `clear interface counters` | Clears all counters |
| AC-8 | Interface named "brief" | `show interface detail brief` shows interface named "brief" |
| AC-9 | `cache retain <id>` | Retains cache entry (action-first grammar) |
| AC-10 | `cache <id> retain` (old grammar) | Works with deprecation warning |
| AC-11 | `cache forward <id> <sel>` | Forwards cache entry (action-first grammar) |
| AC-12 | `commit start <name>` | Starts named commit (action-first grammar) |
| AC-13 | `commit <name> start` (old grammar) | Works with deprecation warning |
| AC-14 | `commit show <name>` | Shows named commit (action-first grammar) |
| AC-15 | Cache ID accepted as string | `cache retain abc123` accepted, parsed internally |
| AC-16 | Grammar rule documented | `.claude/rules/cli-grammar.md` exists with the rule |
| AC-17 | INSTRUCTIONS.md pointer | "Before You" table has cli-grammar entry |
| AC-18 | Full audit complete | All handlers checked, violations listed and fixed |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestShowInterface_DetailByName | `internal/component/cmd/show/show_test.go` | New grammar: detail <name> | |
| TestShowInterface_CountersByName | `internal/component/cmd/show/show_test.go` | New grammar: counters <name> | |
| TestShowInterface_DeprecatedNameFirst | `internal/component/cmd/show/show_test.go` | Old grammar still works with warning | |
| TestClearInterface_CountersByName | `internal/component/iface/cmd/clear_test.go` | New grammar: counters <name> | |
| TestClearInterface_DeprecatedBare | `internal/component/iface/cmd/clear_test.go` | Old bare <name> still works with warning | |
| TestBgpCache_ActionFirst | `internal/component/bgp/plugins/cmd/cache/cache_test.go` | New grammar: retain/release/expire/forward <id> | |
| TestBgpCache_DeprecatedIdFirst | `internal/component/bgp/plugins/cmd/cache/cache_test.go` | Old grammar: <id> retain still works with warning | |
| TestBgpCache_StringId | `internal/component/bgp/plugins/cmd/cache/cache_test.go` | String ID accepted at CLI layer | |
| TestCommit_ActionFirst | `internal/component/bgp/plugins/cmd/commit/commit_test.go` | New grammar: start/end/show <name> | |
| TestCommit_DeprecatedNameFirst | `internal/component/bgp/plugins/cmd/commit/commit_test.go` | Old grammar: <name> start still works with warning | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| test-show-interface-grammar | `test/plugin/show-interface-grammar.ci` | New and old grammar both work | |

### Future (if deferring any tests)
- None

## Files to Modify
- `internal/component/cmd/show/show.go` - refactor handleShowInterface dispatch
- `internal/component/iface/cmd/clear.go` - refactor handleClearInterfaceCounters
- `internal/component/bgp/plugins/cmd/cache/cache.go` - refactor handleBgpCache to action-first, string ID
- `internal/component/bgp/plugins/cmd/commit/commit.go` - refactor handleCommit to action-first
- `internal/component/iface/schema/ze-iface-cmd.yang` - update YANG grammar
- `internal/component/bgp/plugins/cmd/cache/schema/ze-bgp-cmd-cache-api.yang` - update YANG grammar
- `internal/component/bgp/plugins/cmd/commit/schema/ze-bgp-cmd-commit-api.yang` - update YANG grammar
- `docs/guide/command-reference.md` - update documentation
- `ai/patterns/cli-command.md` - add grammar rule reference to Command Grammar section

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | Yes | `internal/component/iface/schema/ze-iface-cmd.yang` |
| CLI commands/flags | Yes | show.go, clear.go |
| Editor autocomplete | Yes | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | Yes | `test/plugin/show-interface-grammar.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` - grammar change |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Files to Create
- `.claude/rules/cli-grammar.md` - grammar rule: `<verb> interface <action> [<interface>]`
- `test/plugin/show-interface-grammar.ci` - functional test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring** - write failing tests for new grammar (all four handlers)
   - Tests: TestShowInterface_DetailByName, TestClearInterface_CountersByName,
     TestBgpCache_ActionFirst, TestCommit_ActionFirst (all fail)
   - Files: show_test.go, clear_test.go, cache_test.go, commit_test.go
   - Verify: tests fail (old dispatch doesn't recognize new grammar)

2. **Phase: Show dispatch refactor** - action-first dispatch with backward compat
   - Refactor handleShowInterface: keyword dispatch first, then deprecated name-first fallback with warning
   - Tests: all show tests pass
   - Files: show.go, show_test.go
   - Verify: new grammar works, old grammar works with deprecation warning

3. **Phase: Clear dispatch refactor** - action-first dispatch with backward compat
   - Refactor handleClearInterfaceCounters: require explicit action keyword
   - Tests: all clear tests pass
   - Files: clear.go, clear_test.go
   - Verify: new grammar works, old grammar works with deprecation warning

4. **Phase: Cache dispatch refactor** - action-first, string IDs
   - Refactor handleBgpCache: action keyword first (`retain <id>`, `forward <id> <sel>`),
     accept ID as string, parse to uint64 internally
   - Backward compat: detect `<id> <action>` pattern (numeric first arg followed by keyword),
     accept with deprecation warning
   - Tests: all cache tests pass
   - Files: cache.go, cache_test.go
   - Verify: new grammar works, string IDs accepted, old grammar with deprecation warning

5. **Phase: Commit dispatch refactor** - action-first
   - Refactor handleCommit: action keyword first (`start <name>`, `show <name>`)
   - Backward compat: detect `<name> <action>` pattern (non-keyword first arg followed by keyword),
     accept with deprecation warning
   - Tests: all commit tests pass
   - Files: commit.go, commit_test.go
   - Verify: new grammar works, old grammar with deprecation warning

6. **Phase: YANG update** - update all command trees
   - Update ze-iface-cmd.yang, ze-bgp-cmd-cache-api.yang, ze-bgp-cmd-commit-api.yang
   - Files: all three YANG files
   - Verify: YANG compiles, autocomplete shows new grammar

7. **Phase: Documentation** - update patterns doc
   - Add grammar rule reference to `ai/patterns/cli-command.md`
   - Update `docs/guide/command-reference.md`
   - Files: cli-command.md, command-reference.md
   - Verify: docs reflect new grammar

8. **Phase: Full audit** - scan all remaining commands
   - Grep all handlers for args[0] keyword/name ambiguity
   - Verify firewall, neighbor, policy, DNS, resolve handlers are clean (explorer confirmed)
   - Document any additional violations found
   - Files: varies
   - Verify: audit complete, all violations addressed

9. **Functional tests** - create .ci test
10. **Full verification** - `make ze-verify`
11. **Complete spec** - fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Old grammar still works (backward compat) with deprecation warning |
| Naming | `detail` keyword chosen over `info`/`show`/`view` |
| Data flow | Dispatch order: keywords checked before name fallback |
| YANG | Command tree reflects new grammar, autocomplete correct |
| Audit | All handlers scanned, no violations missed |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| Grammar rule file | `ls .claude/rules/cli-grammar.md` |
| Show dispatch refactored | `grep "detail" internal/component/cmd/show/show.go` |
| Clear dispatch refactored | `grep dispatch changes in clear.go` |
| Functional test | `ls test/plugin/show-interface-grammar.ci` |
| Deprecation warning | `grep -i "deprecat" internal/component/cmd/show/show.go` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Interface names validated before lookup (existing behavior preserved) |
| Backward compat | Old grammar doesn't silently break scripts |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design Insights

The `clear.go` handler already documents the ambiguity (lines 37-46) and explains
the workaround (`clear interface counters counters` for an interface named "counters").
The action-first grammar eliminates this class of ambiguity entirely.

## RFC Documentation

N/A - no RFC protocol work.

## Implementation Summary

### What Was Implemented
- [To be filled during implementation]

### Bugs Found/Fixed
- [To be filled during implementation]

### Documentation Updates
- [To be filled during implementation]

### Deviations from Plan
- [To be filled during implementation]

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
- [ ] AC-1..AC-18 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
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

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit

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

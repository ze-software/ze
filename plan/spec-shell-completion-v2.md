# Spec: Shell Completion v2 -- Dynamic Value Completions

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 3/3 |
| Updated | 2026-04-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/command/node.go` - Node struct (add ValueHints here)
4. `internal/component/command/completer.go` - TreeCompleter (matchChildren adds ValueHints)
5. `cmd/ze/completion/words.go` - shell completion walker (delegate to TreeCompleter)
6. `cmd/ze/cli/main.go` - BuildCommandTree (wire ValueHints after build)
7. `cmd/ze/completion/bash.go`, `zsh.go`, `fish.go`, `nushell.go` - shell scripts

## Task

Ze has shell completion for bash, zsh, fish, and nushell (subcommand dispatch + YANG-driven
dynamic words via `ze completion words`, peer selectors via `ze completion peers`). The
current system completes **command names** and **peer selectors** well but does not complete
**argument values** -- family names, log levels, flag values, or config keys at tab time.

This spec covers two things: unifying the completion walker so CLI and shell share
one code path, and extending completion to provide contextual value suggestions.

### Already implemented (prior work, not part of this spec)

- Peer addresses from running daemon via `ze completion peers` (peers.go)
- Schema module names via `ze schema list` (dynamic in all 4 shells)
- Plugin names via `ze --plugins --json` (dynamic in all 4 shells)
- Nushell support (nushell.go -- full extern definitions + dynamic completers)
- Dynamic show/run multi-level completion via `ze completion words show|run`
- Graceful degradation when daemon unreachable (returns 0, stderr suppressed)
- File path completion for config/exabgp (bash: `_ze_filedir conf`, zsh: `_files -g '*.conf'`, fish: `-F`)

### Problem: two walkers for one tree

`words.go` reimplements tree walking that `command.TreeCompleter` already does.
Both consume the same `command.Node` tree from `cli.BuildCommandTree()`.
Adding value completions to both paths would mean duplicating the logic.

### Design: unify walker + add ValueHints

1. Add `ValueHints func() []Suggestion` to `command.Node` -- terminal argument values (families, log levels, config sections)
2. `TreeCompleter.matchChildren()` includes ValueHints alongside Children and DynamicChildren
3. `words.go` delegates to `TreeCompleter` instead of reimplementing the walk
4. `cli/main.go:BuildCommandTree()` wires ValueHints to known nodes after building the tree
5. Shell scripts get value completions through the existing `ze completion words` interface -- no new subcommand needed

| Node field | Returns | After selection |
|------------|---------|-----------------|
| `Children` | Static subcommands | Navigate deeper |
| `DynamicChildren` | Runtime subcommands (peer selectors) | Navigate deeper |
| `ValueHints` | Terminal argument values (families, levels) | No further navigation |

### Remaining work (this spec)

- Add `ValueHints func() []Suggestion` field to `command.Node`
- `TreeCompleter.matchChildren()` includes ValueHints in output
- `words.go` delegates to `TreeCompleter` instead of manual tree walk
- Wire ValueHints in `BuildCommandTree()` for: families, log levels, config sections

The goal: typing `ze show rib <TAB>` sees family names alongside subcommands; typing
`ze show log set <TAB>` sees log levels; both CLI interactive and shell completion
get the same suggestions from the same code. No shell script changes needed.

### Inspiration

rustbgpd pre-generates completions for bash/zsh/fish. Ze's dynamic approach via
`ze completion words` is already superior for command names. This spec extends that
advantage to argument values, which rustbgpd does not complete dynamically either.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - API command structure
  -> Decision:
  -> Constraint:
- [ ] `docs/architecture/api/wire-format.md` - how CLI communicates with daemon
  -> Decision:
  -> Constraint:

### Source files (MUST read before implementing)
- [ ] `internal/component/command/node.go` - Node struct, BuildTree
  -> Decision: ValueHints field added here
  -> Constraint: Node used by both CLI and shell completion -- changes affect both
- [ ] `internal/component/command/completer.go` - TreeCompleter, matchChildren, Complete
  -> Decision: matchChildren includes ValueHints
  -> Constraint: Complete() handles pipe operators -- shell path must skip Type=="pipe"
- [ ] `cmd/ze/completion/words.go` - current manual walk
  -> Decision: replace walk with TreeCompleter delegation
  -> Constraint: preserve show/run mode dispatch and tab-separated output format
- [ ] `cmd/ze/cli/main.go` - BuildCommandTree, mergeDescriptions
  -> Decision: wire ValueHints after tree build
  -> Constraint: families available via plugin init(), log levels are static, config sections from YANG loader
- [ ] `internal/component/bgp/message/family.go` - family constant/registry
  -> Decision:
  -> Constraint:

### RFC Summaries (MUST for protocol work)
- N/A (not protocol work)

**Key insights:**
- `TreeCompleter.Complete()` with trailing space in input returns all children (no prefix filter) -- this is what shell completion needs
- `TreeCompleter` skips unknown words when `DynamicChildren` is set (line 129 completer.go) -- ValueHints needs different behavior (terminal, no further navigation)
- `DynamicChildren` for peer selectors stays separate: CLI wires it with daemon SSH cache, shell uses `ze completion peers` (different runtime context)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/command/node.go` - Node struct, BuildTree, RPCInfo
- [ ] `internal/component/command/completer.go` - TreeCompleter, matchChildren, Complete, GhostText
- [ ] `cmd/ze/completion/main.go` - dispatch: bash/zsh/fish/nushell/words/peers subcommands
- [ ] `cmd/ze/completion/words.go` - manual tree walk (to be replaced with TreeCompleter delegation)
- [ ] `cmd/ze/completion/peers.go` - dynamic peer selector completion from running daemon via SSH
- [ ] `cmd/ze/completion/bash.go` - bash script with static + dynamic completions + peer hooks
- [ ] `cmd/ze/completion/zsh.go` - zsh script with `_arguments` patterns + peer hooks
- [ ] `cmd/ze/completion/fish.go` - fish script with `complete` commands + peer hooks
- [ ] `cmd/ze/completion/nushell.go` - nushell extern definitions + dynamic completers + peer hooks
- [ ] `cmd/ze/cli/main.go` - BuildCommandTree, mergeDescriptions, ValueHints wiring point
- [ ] `cmd/ze/internal/cmdutil/cmdutil.go` - DescribeCommand, used by words.go for descriptions

**Behavior to preserve:**
- All existing subcommand completions (bash, zsh, fish, nushell)
- `ze completion words show|run [path...]` interface (same CLI, different internal path)
- `ze completion peers` interface (daemon peer selectors -- separate runtime)
- Dynamic YANG-driven command discovery
- Dynamic schema module completion (`ze schema list`)
- Dynamic plugin name completion (`ze --plugins --json`)
- Peer selector hooks in show/run paths (all 4 shells)
- Installation methods (`eval "$(ze completion bash)"`, file redirect)
- Graceful degradation on daemon unreachable
- CLI interactive completion behavior (TreeCompleter API unchanged)
- Existing test assertions (tab-separated output, tree walk behavior)

**Behavior to change:**
- `words.go` reimplements tree walk -- replace with `TreeCompleter` delegation
- `command.Node` has no ValueHints -- add field for terminal argument values
- `TreeCompleter.matchChildren` only returns Children + DynamicChildren -- add ValueHints
- `BuildCommandTree` does not wire value hints -- attach families, log levels, config sections
- Shell scripts have no hooks for flag-value completion (e.g., `--family <TAB>`)
- `ze completion words` output includes only commands -- will include value hints too

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Shell tab key triggers completion function
- Completion function calls `ze completion words <mode> <path...>` for commands + values
- Completion function calls `ze completion peers` for peer selectors (already implemented)
- CLI interactive `<TAB>` calls `TreeCompleter.Complete()` directly (already implemented)

### Transformation Path (unified)
1. `cli.BuildCommandTree()` builds `command.Node` tree from RPCs + YANG descriptions
2. `BuildCommandTree()` wires `ValueHints` callbacks to nodes that accept typed arguments
3. **Shell path:** `ze completion words show <path>` -> `TreeCompleter.Complete(input)` -> tab-separated stdout
4. **CLI path:** `CommandCompleter.Complete(input)` -> `TreeCompleter.Complete(input)` -> `[]Suggestion` (unchanged)
5. Both paths call the same `matchChildren()` which merges Children + DynamicChildren + ValueHints

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Shell -> ze binary | exec `ze completion words` with context args | [x] (existing) |
| ze binary -> daemon | SSH query for peer data via `ze completion peers` | [x] (existing) |
| Family registry -> Node | `ValueHints` callback queries registered families at call time | [ ] |
| YANG loader -> Node | Config section names from YANG top-level containers | [ ] |

### Integration Points
- `internal/component/command/node.go` - add `ValueHints` field to `Node`
- `internal/component/command/completer.go` - `matchChildren` includes ValueHints
- `cmd/ze/completion/words.go` - delegate to `TreeCompleter` instead of manual walk
- `cmd/ze/cli/main.go` - wire ValueHints in `BuildCommandTree()`

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze completion words show <path>` | -> | `TreeCompleter.Complete()` via `words.go` | TBD |
| `ze completion words show log set` | -> | ValueHints on `log set` node returns log levels | TBD |
| `ze completion words show rib` | -> | ValueHints on `rib` node returns families | TBD |
| CLI interactive `rib <TAB>` | -> | Same TreeCompleter + ValueHints | TBD (existing tests cover TreeCompleter) |

## Acceptance Criteria

### Already satisfied (prior work -- preserved, not re-tested by this spec)

| AC ID | Input / Condition | Expected Behavior | Evidence |
|-------|-------------------|-------------------|----------|
| ~~AC-1~~ | `ze show peer <TAB>` (daemon running) | Lists peer selectors from running session | `peers.go` + shell hooks in all 4 scripts |
| ~~AC-5~~ | `ze schema show <TAB>` | Lists YANG module names | Dynamic `ze schema list` in all 4 shells |
| ~~AC-8~~ | Completions degrade gracefully when daemon unreachable | No error output, just no live suggestions | `peers.go` returns 0 on SSH failure |

### Remaining (this spec)

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-2 | `ze completion words show rib` | Output includes registered address families (from ValueHints) |
| AC-3 | `ze completion words show log set` | Output includes log levels: debug, info, warn, err, disabled |
| ~~AC-4~~ | ~~`ze completion words show config edit`~~ | ~~config sections~~ Not applicable: config is a standalone binary path (`ze config edit`), not in the show/run RPC tree. Requires shell script changes (separate concern). |
| AC-6 | `ze completion words show` | Same output as before (walker unification is transparent) |
| AC-7 | Completions work offline (no daemon) for static values | Family names, log levels always available (no SSH needed) |
| AC-9 | CLI interactive `rib <TAB>` | Same families appear as in shell completion (shared TreeCompleter) |
| AC-10 | `words.go` has no manual tree walk | All tree walking delegated to `TreeCompleter.Complete()` |

## TDD Test Plan

### Existing tests (prior work -- 49 tests across 3 files)
- `cmd/ze/completion/main_test.go` - 36 tests: script structure, commands, dynamic callbacks, depth guards (all 4 shells)
- `cmd/ze/completion/words_test.go` - 6 tests: tab output format, show vs run, deep path, edge cases
- `cmd/ze/completion/peers_test.go` - 7 tests: format, ASN dedup, no-name, empty/invalid JSON, dispatch

### New unit tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestValueHintsIncludedInMatchChildren` | `internal/component/command/completer_test.go` | ValueHints returned by matchChildren alongside static children | |
| `TestValueHintsNotNavigable` | `internal/component/command/completer_test.go` | ValueHint selections are terminal (no further children) | |
| `TestWordsShowDelegatesToTreeCompleter` | `cmd/ze/completion/words_test.go` | AC-10: words output matches TreeCompleter output (no manual walk) | |
| `TestWordsShowRibIncludesFamilies` | `cmd/ze/completion/words_test.go` | AC-2: `words show rib` output includes family names from ValueHints | |
| `TestWordsShowLogSetIncludesLevels` | `cmd/ze/completion/words_test.go` | AC-3: `words show log set` output includes log levels | |
| `TestWordsShowPreservesFormat` | `cmd/ze/completion/words_test.go` | AC-6: tab-separated output unchanged after refactor | |
| `TestTreeCompleterFamilyHints` | `cmd/ze/cli/main_test.go` | AC-9: BuildCommandTree wires family ValueHints to rib node | |

### Boundary Tests (MANDATORY for numeric inputs)
- N/A (no numeric inputs in completion)

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-completion-values` | `test/ui/completion-values.ci` | `ze completion words show rib` output includes family names | |

## Files to Modify
- `internal/component/command/node.go` - add `ValueHints func() []Suggestion` field to `Node`
- `internal/component/command/completer.go` - `matchChildren` includes ValueHints in output
- `cmd/ze/completion/words.go` - replace manual tree walk with `TreeCompleter` delegation
- `cmd/ze/cli/main.go` - wire ValueHints to known nodes in `BuildCommandTree()` (families, log levels, config sections)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | N/A |
| RPC count in architecture docs | [ ] | N/A |
| CLI commands/flags | [ ] | N/A (no new CLI commands) |
| CLI usage/help text | [ ] | N/A |
| API commands doc | [ ] | N/A |
| Plugin SDK docs | [ ] | N/A |
| Editor autocomplete | [x] | CLI interactive gets ValueHints automatically via shared TreeCompleter |
| Functional test for new RPC/API | [x] | `test/ui/completion-values.ci` |

## Files to Create
- `test/ui/completion-values.ci` - functional test for value completions via `ze completion words`

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
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

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
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

1. **Phase: ValueHints on Node + TreeCompleter** -- add field to Node, include in matchChildren
   - Tests: `TestValueHintsIncludedInMatchChildren`, `TestValueHintsNotNavigable`
   - Files: `internal/component/command/node.go`, `internal/component/command/completer.go`
   - Verify: tests fail -> implement -> tests pass
   - Existing tests must still pass (TreeCompleter behavior unchanged for nodes without ValueHints)
2. **Phase: words.go delegates to TreeCompleter** -- replace manual walk with Complete() call
   - Tests: `TestWordsShowDelegatesToTreeCompleter`, `TestWordsShowPreservesFormat`
   - Files: `cmd/ze/completion/words.go`
   - Verify: all existing words_test.go tests still pass with new implementation
   - Key: output format unchanged (tab-separated), skip Type=="pipe" suggestions
3. **Phase: Wire ValueHints in BuildCommandTree** -- attach families, log levels, config sections
   - Tests: `TestTreeCompleterFamilyHints`, `TestWordsShowRibIncludesFamilies`, `TestWordsShowLogSetIncludesLevels`
   - Files: `cmd/ze/cli/main.go`
   - Verify: tests fail -> implement -> tests pass
   - Source for values: family registry (plugins register at init), static log levels, YANG top-level containers
4. **Functional tests** -> Create after feature works. Cover user-visible behavior.
5. **Full verification** -> `make ze-verify` (lint + all ze tests except fuzz)
6. **Complete spec** -> Fill audit tables, write learned summary to `plan/learned/NNN-<name>.md`, delete spec from `plan/`. BLOCKING: summary is part of the commit, not a follow-up.

Note: shell script changes (bash.go, zsh.go, fish.go, nushell.go) are NOT needed for this spec.
Value hints flow through the existing `ze completion words show|run <path>` interface -- the shell
scripts already display whatever that command returns. Flag-value completion (e.g., `--family <TAB>`)
is a separate concern that can be a follow-up spec if needed.

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Family names match `make ze-inventory` output exactly |
| Naming | Completion output uses kebab-case family format (afi/safi) |
| No duplication | `words.go` has zero tree-walking code -- all delegated to TreeCompleter |
| Shared path | CLI interactive and shell completion use the same matchChildren + ValueHints |
| Data flow | Static values never require daemon connection |
| Rule: no-layering | ValueHints extends Node, not a parallel mechanism |
| Backward compat | All existing words_test.go and completer_test.go tests pass unchanged |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `command.Node` has `ValueHints` field | grep for ValueHints in node.go |
| `TreeCompleter.matchChildren` includes ValueHints | grep + test |
| `words.go` delegates to TreeCompleter | no manual tree walk in words.go (grep for `current.Children`) |
| `ze completion words show rib` includes families | Run command, check output |
| `ze completion words show log set` includes levels | Run command, check output |
| CLI interactive gets same ValueHints | TestTreeCompleterFamilyHints passes |
| All existing tests pass | `make ze-verify` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Completion context args sanitized before passing to daemon query |
| No command injection | Shell scripts escape user input in completion functions |
| Timeout | Daemon queries have short timeout (1-2s) to avoid hanging tab completion |

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

### Why unify walkers before adding value completions
Adding value completions to the existing split architecture (manual walk in words.go + TreeCompleter
in completer.go) would mean implementing the same ValueHints logic twice. Unifying first means
value completions are added once in TreeCompleter.matchChildren and both CLI interactive and shell
completion benefit immediately.

### ValueHints vs DynamicChildren
Both are callbacks on Node that return []Suggestion. The semantic difference:
- DynamicChildren = navigation targets (peer names become part of the path, you navigate deeper)
- ValueHints = terminal argument values (family name completes the argument, no further navigation)
The completer treats them differently: after a DynamicChildren match, Continue walks try that
child's subtree. After a ValueHints match, no further navigation is attempted.

### Peer completion stays separate
`ze completion peers` (peers.go) stays as a standalone command because shell completion runs as a
one-shot process without a persistent daemon connection. The interactive CLI wires DynamicChildren
with a cached SSH query (cli/main.go:391-397). Same data, different runtime -- legitimate
separation, not code duplication.

### Shell scripts need no changes
Value hints flow through the existing `ze completion words show|run <path>` interface. The shell
scripts already display whatever that command returns. The output just includes more suggestions
now (value hints mixed with subcommands). The shell does its own prefix filtering.

## RFC Documentation

N/A -- not protocol work.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

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
- [ ] AC-1..AC-N all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.

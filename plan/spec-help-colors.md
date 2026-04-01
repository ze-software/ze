# Spec: Structured colored help output

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-colored-slog (UseColor) |
| Phase | - |
| Updated | 2026-04-01 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/internal/helpfmt/` - the new package (once created)
4. `cmd/ze/main.go` (lines 921-971) - main usage() function
5. `cmd/ze/bgp/main.go` (lines 51-74) - typical subcommand usage()
6. `internal/component/command/help.go` - existing WriteHelp for YANG tree

## Task

Ze has 40+ files with inline `fmt.Fprintf(os.Stderr, ...)` help text. Each subcommand hand-formats its own output with no shared structure and no color. Help text is hard to scan in a terminal.

**Goal:** Create a shared `helpfmt` package that gives help output semantic structure (like HTML elements with CSS) and applies color via `slogutil.UseColor()`. Migrate all subcommand help to use it.

## Required Reading

### Architecture Docs
- [ ] `.claude/rules/cli-patterns.md` - flag conventions, dispatch pattern
  -> Constraint: each domain has `cmd/ze/<domain>/main.go` with `func Run(args []string) int`
  -> Constraint: errors to stderr, return exit codes
- [ ] `internal/core/slogutil/color.go` - UseColor(), ANSI constants
  -> Constraint: UseColor(w) checks NO_COLOR, TERM=dumb, ze.log.color, TTY
- [ ] `internal/component/command/help.go` - existing WriteHelp for YANG tree
  -> Decision: WriteHelp renders dynamic verb list, must integrate with new system

**Key insights:**
- Help goes to stderr (consistent across all subcommands)
- `UseColor(os.Stderr)` is the color gate -- already available
- `command.WriteHelp()` renders YANG command tree entries -- needs to gain color awareness
- `cmd/ze/internal/` already exists with `cmdutil/`, `ssh/`, `suggest/` -- good home for `helpfmt/`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/main.go` (lines 921-971) - main usage(): multiline backtick string with sections (Usage, Verbs, Tools, Options, Examples), calls command.WriteHelp for dynamic verbs
- [ ] `cmd/ze/bgp/main.go` (lines 51-74) - typical subcommand: single backtick string, sections (Usage, Commands, See also, Examples)
- [ ] `cmd/ze/config/main.go` - config usage(): grouped sections (Editing, Storage, Inspection, History, Migration, Options, Examples)
- [ ] `internal/component/command/help.go` (lines 50-97) - WriteHelp: renders sorted children as `"  %-16s %s\n"` lines
- [ ] `internal/test/runner/color.go` - test runner Colors struct with per-color methods (Red, Green, etc.)

**Behavior to preserve:**
- All help output goes to stderr
- Section layout: Usage, Commands/Verbs, Options, Examples, See also
- 2-column format for entries: `"  %-16s %s\n"` (name + description)
- Dynamic YANG verb list in main usage
- `fs.Usage` callback pattern for flag-based subcommands
- Exit codes unchanged (help returns 0)

**Behavior to change:**
- Raw `fmt.Fprintf` calls replaced with structured `helpfmt` API
- Section headers, command names, flags, args, examples get semantic color
- `command.WriteHelp()` gains color-aware entry formatting

## Data Flow (MANDATORY)

### Entry Point
- User runs `ze --help` or `ze <cmd> --help` or invalid command
- Subcommand's `usage()` function is called

### Transformation Path
1. Subcommand builds `helpfmt.Page` struct with sections and entries
2. `helpfmt.Page.Write(os.Stderr)` calls `slogutil.UseColor(os.Stderr)` once
3. If color enabled: each role wraps text in ANSI codes before writing
4. If color disabled: text written as-is (current behavior)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Subcommand -> helpfmt | Struct construction, no io | [ ] |
| helpfmt -> slogutil | UseColor(w) call for color decision | [ ] |
| helpfmt -> stderr | fmt.Fprintf with colored/plain text | [ ] |

### Integration Points
- `slogutil.UseColor(w)` - color gate (from spec-colored-slog)
- `command.WriteHelp()` - must use helpfmt formatting for entries
- Every `usage()` function in cmd/ze/ - migration targets

### Architectural Verification
- [ ] No bypassed layers (all help goes through helpfmt)
- [ ] No unintended coupling (helpfmt depends only on slogutil for color gate)
- [ ] No duplicated functionality (replaces inline fmt.Fprintf, not alongside)
- [ ] Zero-copy preserved where applicable (N/A)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze --help` | -> | `usage()` -> `helpfmt.Page.Write(os.Stderr)` | `test/parse/help-color.ci` |
| `ze bgp --help` | -> | bgp `usage()` -> `helpfmt.Page.Write(os.Stderr)` | `test/parse/help-bgp.ci` |
| `NO_COLOR=1 ze --help` | -> | helpfmt respects UseColor | `test/parse/help-no-color.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze --help` on TTY | Section headers in bold/bright, command names in cyan, flags in yellow, args dim, examples dim |
| AC-2 | `ze --help` with `NO_COLOR=1` | Plain text, no ANSI codes (identical to current output layout) |
| AC-3 | `ze bgp --help` on TTY | Same color roles applied to subcommand help |
| AC-4 | `ze config --help` on TTY | Grouped sections (Editing, Storage, etc.) each get header color |
| AC-5 | `helpfmt.Page` struct built by subcommand | Sections, entries, examples all populated without raw format strings |
| AC-6 | `command.WriteHelp()` renders colored entries | YANG verb names in subcommand color, descriptions in default |
| AC-7 | All existing help content preserved | No text changes, only color wrapping added |
| AC-8 | Main help migrated | `cmd/ze/main.go` usage() uses helpfmt |
| AC-9 | BGP help migrated | `cmd/ze/bgp/main.go` usage() uses helpfmt |
| AC-10 | Config help migrated | `cmd/ze/config/main.go` usage() uses helpfmt |

## Design

### Semantic Roles

| Role | What it styles | Color (when enabled) | ANSI |
|------|---------------|---------------------|------|
| Header | Section titles ("Usage:", "Commands:") | Bold bright white | `\033[1;37m` |
| Command | Top-level command path ("ze bgp") | Cyan | `\033[36m` |
| Subcommand | Entry names in lists ("decode", "encode") | Green | `\033[32m` |
| Flag | Flag names ("--debug", "-f") | Yellow | `\033[33m` |
| Arg | Placeholder args ("<file>", "[options]") | Dim | `\033[2m` |
| Desc | Description text | Default (no ANSI) | - |
| Example | Example command lines | Dim | `\033[2m` |
| Error | Error messages | Bold red | `\033[1;31m` |

### API

The helpfmt package exposes a struct-based API. Subcommands build a `Page`, then call `Write`.

| Type | Fields | Purpose |
|------|--------|---------|
| `Page` | Command, Summary, Usage, Sections, Examples, SeeAlso | Full help page |
| `Section` | Title string, Entries []Entry | Named group of entries |
| `Entry` | Name string, Desc string | Single command/flag + description |

| Function | Signature | Purpose |
|----------|-----------|---------|
| `Page.Write` | `(w io.Writer)` | Render full help page with color |
| `Entry.Write` | `(w io.Writer, color bool)` | Render single entry line |
| `WriteError` | `(w io.Writer, format string, a ...any)` | Colored error message |
| `WriteHint` | `(w io.Writer, format string, a ...any)` | Colored hint/suggestion |

### Flag detection

Entries whose Name starts with `-` are auto-detected as flags and colored with the Flag role instead of Subcommand. Angle brackets in names (`<file>`) get the Arg role.

### command.WriteHelp integration

`command.WriteHelp()` currently writes plain `"  %-16s %s\n"` lines. It should accept a color bool parameter (or use UseColor internally) and apply Subcommand color to names and Desc color to descriptions.

### Migration pattern

Each subcommand replaces its `usage()` body from a backtick string to a `helpfmt.Page` struct literal. The struct literal is equally readable and contains the same text -- just structured.

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPageWriteColored` | `cmd/ze/internal/helpfmt/helpfmt_test.go` | AC-1: colored output contains ANSI codes for each role | |
| `TestPageWritePlain` | `cmd/ze/internal/helpfmt/helpfmt_test.go` | AC-2: plain output has no ANSI codes | |
| `TestFlagDetection` | `cmd/ze/internal/helpfmt/helpfmt_test.go` | Entries starting with `-` get flag color, not subcommand | |
| `TestArgHighlighting` | `cmd/ze/internal/helpfmt/helpfmt_test.go` | Angle brackets in usage lines get dim styling | |
| `TestEmptySection` | `cmd/ze/internal/helpfmt/helpfmt_test.go` | Section with no entries is omitted | |
| `TestWriteError` | `cmd/ze/internal/helpfmt/helpfmt_test.go` | Error output in bold red when color enabled | |
| `TestWriteHint` | `cmd/ze/internal/helpfmt/helpfmt_test.go` | Hint output in yellow when color enabled | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `help-no-color` | `test/parse/help-no-color.ci` | `NO_COLOR=1 ze --help` produces no ANSI codes | |
| `help-bgp` | `test/parse/help-bgp.ci` | `ze bgp --help` exits 0, contains expected sections | |

### Future (if deferring any tests)
- Migration of all 40+ subcommands beyond the initial 3 (main, bgp, config)

## Files to Create
- `cmd/ze/internal/helpfmt/helpfmt.go` - Page, Section, Entry types, Write methods, role coloring
- `cmd/ze/internal/helpfmt/helpfmt_test.go` - unit tests
- `test/parse/help-no-color.ci` - functional test
- `test/parse/help-bgp.ci` - functional test

## Files to Modify
- `cmd/ze/main.go` - migrate usage() to helpfmt.Page
- `cmd/ze/bgp/main.go` - migrate usage() to helpfmt.Page
- `cmd/ze/config/main.go` - migrate usage() to helpfmt.Page
- `internal/component/command/help.go` - add color support to WriteHelp/writeHelpEntry

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | No (no new flags) | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No (visual enhancement, no new capability) | N/A |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | No | N/A |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | No | N/A |

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

1. **Phase: helpfmt package** -- create Page/Section/Entry types with Write methods and role-based ANSI coloring
   - Tests: `TestPageWriteColored`, `TestPageWritePlain`, `TestFlagDetection`, `TestArgHighlighting`, `TestEmptySection`, `TestWriteError`, `TestWriteHint`
   - Files: `cmd/ze/internal/helpfmt/helpfmt.go`, `cmd/ze/internal/helpfmt/helpfmt_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: migrate main help** -- convert `cmd/ze/main.go` usage() to helpfmt.Page
   - Tests: visual verification + `test/parse/help-no-color.ci`
   - Files: `cmd/ze/main.go`
   - Verify: `ze --help` output matches current layout with colors added

3. **Phase: migrate bgp and config help** -- convert bgp and config usage() functions
   - Tests: `test/parse/help-bgp.ci`
   - Files: `cmd/ze/bgp/main.go`, `cmd/ze/config/main.go`
   - Verify: output matches current layout

4. **Phase: command.WriteHelp integration** -- add color to YANG tree help entries
   - Files: `internal/component/command/help.go`
   - Verify: `ze --help` shows colored verb list

5. **Full verification** -- `make ze-verify`
6. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Output text identical to current (only ANSI wrapping differs) |
| Naming | Package name `helpfmt`, types `Page`/`Section`/`Entry` follow Go conventions |
| Data flow | UseColor called once per Write, not per line |
| Rule: no-layering | Old inline fmt.Fprintf help replaced, not kept alongside |
| Rule: design-principles | No premature abstraction -- each type has 3+ concrete uses across migrated files |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| helpfmt package exists | `ls cmd/ze/internal/helpfmt/helpfmt.go` |
| Main help uses helpfmt | `grep 'helpfmt' cmd/ze/main.go` |
| BGP help uses helpfmt | `grep 'helpfmt' cmd/ze/bgp/main.go` |
| Config help uses helpfmt | `grep 'helpfmt' cmd/ze/config/main.go` |
| command.WriteHelp uses color | `grep 'UseColor\|ansi\|color' internal/component/command/help.go` |
| Functional test exists | `ls test/parse/help-no-color.ci` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| No injection | ANSI codes are constants, help text is hardcoded strings, no user input in formatting |
| No resource exhaustion | Page.Write is bounded by number of sections/entries, all static |

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
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
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.

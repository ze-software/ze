# Spec: tui-launcher

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 4/4 |
| Updated | 2026-06-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/ze_core_dispatch.go` - dispatch logic, no-arg branch at line 293
4. `cmd/ze/ze_core_usage.go` - current help output
5. `internal/component/command/registry/registry.go` - section constants, Meta struct

## Task

When `ze` is invoked with no arguments and stdin is a TTY, show an interactive
BubbleTea menu instead of the static help text. The menu lists Ze's top-level
commands grouped by section (Operations, Configuration, System), each with its
description. The user navigates with arrow keys, selects with Enter, and the
chosen command is dispatched as if typed on the command line.

When stdin is not a TTY (piped, cron, scripts), fall back to the current static
help text and exit 1, preserving backward compatibility.

Inspiration: Ollama's no-arg TUI, which shows a navigable menu of actions using
BubbleTea + Lipgloss.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/system-architecture.md` - overall dispatch and CLI entry
  -> Constraint: dispatch order in zeDispatch must be preserved; TUI is a new branch at `len(args) < 1`
- [ ] `docs/architecture/core-design.md` - registration pattern
  -> Decision: commands self-register via registry; menu items come from registry, not a hardcoded list

### RFC Summaries (MUST for protocol work)
N/A -- not protocol work.

**Key insights:**
- Ze already depends on bubbletea v2 and lipgloss v2 (used by `internal/component/cli/`)
- Commands register into sections (Operations, Configuration, System, Test) via `registry.RegisterRoot()`
- `registry.ListRootBySection()` already returns commands grouped by section with display titles
- `zeUsage()` already builds section-grouped help from the same registry data
- The CLI component (`internal/component/cli/model.go`) is a full BubbleTea app, so the pattern is established

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/ze_core_dispatch.go` - line 293: `if len(args) < 1 { zeUsage(); return 1 }`
  -> Constraint: this is the only branch to modify; everything after it stays unchanged
- [ ] `cmd/ze/ze_core_usage.go` - `zeUsage()` builds sections from `registry.ListRootBySection()` + YANG verb tree + hardcoded options
  -> Constraint: same data sources feed the TUI menu
- [ ] `internal/component/command/registry/registry.go` - `ListRootBySection()`, `Meta`, section constants
  -> Decision: Meta already has Description, Mode, Section, Subs -- enough for menu items

**Behavior to preserve:**
- Non-TTY invocations: static help to stderr, exit 1
- All dispatch paths after the no-arg check: untouched
- `ze --help` / `ze help`: untouched (separate code paths)

**Behavior to change:**
- TTY invocations with no args: show interactive menu instead of static help

## Data Flow (MANDATORY)

### Entry Point
- User runs `ze` with no arguments, stdin is a TTY

### Transformation Path
1. `zeDispatch()` reaches `len(args) < 1` branch
2. TTY check: `os.Stdin.Stat()` or `term.IsTerminal()`
3. If not TTY: `zeUsage()` + return 1 (unchanged)
4. If TTY: build menu items from `registry.ListRootBySection()` + YANG verb tree
5. Run BubbleTea program with the menu model
6. User selects a command -> return the command name
7. Re-enter dispatch with the selected command as args

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Registry -> TUI | `ListRootBySection()` provides items | [ ] |
| TUI -> Dispatch | Selected command string fed back to `zeDispatch()` | [ ] |

### Integration Points
- `registry.ListRootBySection()` - existing, provides grouped commands
- `cli.BuildCommandTree()` / `cli.YANGCommandTree()` - existing, provides YANG verbs
- `helpfmt.Page` - existing help formatter, used as fallback

### Architectural Verification
- [ ] No bypassed layers (menu uses registry, dispatch handles execution)
- [ ] No unintended coupling (TUI model is self-contained in its own file)
- [ ] No duplicated functionality (reuses registry data, does not rebuild command lists)
- [ ] Zero-copy preserved where applicable (N/A -- UI strings)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | bubbletea v2 works for a simple menu without pulling new deps | go.mod shows charm.land/bubbletea/v2 v2.0.2 already imported | Would need dep evaluation | `go build` compiles | confirmed (go.mod: bubbletea v2.0.2, lipgloss v2.0.2) |
| A-2 | `registry.ListRootBySection()` provides enough info (name + description) for menu items | Registry source shows Meta has Description, Mode, Section, Subs | Would need to enrich Meta | grep registry usage | confirmed (Meta.Description sufficient; ListRootBySection returns []SectionEntry with Commands []RootCommand) |
| A-3 | YANG verbs (show, set, clear, ...) should appear in the menu alongside registered root commands | Current `zeUsage()` mixes them in the Operations section | User may want them separate or excluded | user confirmation | confirmed (matches existing zeUsage() behavior: YANG verbs appended to Operations section) |
| A-4 | `os.Stdin` TTY detection works on gokrazy appliance (serial console) | gokrazy uses serial TTY for console | Menu on serial console might be broken | test on gokrazy or confirm with user | confirmed (serial consoles are TTYs; term.IsTerminal pattern used in 6+ places including login.go, appliance) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Menu on slow serial console (gokrazy appliance) may render poorly | Visual artifacts on 9600 baud | Detect baud/term type, fall back to static help |
| R-2 | Some commands need additional args (e.g., `ze show bgp peer list`), menu only selects the root | User expects to type the rest after selection | After selection, either enter sub-menu or pass to dispatch and let normal arg parsing take over |
| R-3 | Test section commands (ze-test, ze-chaos) showing in menu may confuse end users | User reports on issue tracker | Filter by build tag or section; only show Test section in test builds |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze` with no args, TTY | -> | `runTUIMenu()` in dispatch | `TestTUIMenuDispatch` |
| `ze` with no args, non-TTY | -> | `zeUsage()` (unchanged) | `TestNonTTYFallback` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze` with no args, stdin is TTY | Interactive menu appears with grouped commands |
| AC-2 | `ze` with no args, stdin is not TTY | Static help text to stderr, exit 1 (unchanged) |
| AC-3 | Arrow keys in menu | Cursor moves between items |
| AC-4 | Enter on a menu item | Selected command is dispatched (YANG verbs show their sub-command help) |
| AC-5 | Esc or q in menu | Clean exit, no command run |
| AC-6 | Menu items grouped by section | Operations, Configuration, System sections visible with headers |
| AC-7 | Each item shows name + description | Same info as current help text |
| AC-8 | YANG verbs appear in Operations group | show, set, clear, request, delete, update, validate, monitor |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `ze` in terminal with no args | dispatch -> TTY check -> bubbletea menu -> user picks "doctor" -> dispatch "doctor" | `TestTUISelectDoctor` |
| 2 | Runs `ze` in a pipe (`echo "" \| ze`) | dispatch -> TTY check -> zeUsage() -> exit 1 | `TestNonTTYFallback` |
| 3 | Runs `ze` in terminal, presses Esc | dispatch -> TTY check -> bubbletea menu -> Esc -> clean exit 0 | `TestTUIEscapeExit` |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestMenuModel_Init` | `cmd/ze/tui_menu_test.go` | Model initializes with correct sections and items | |
| `TestMenuModel_Navigation` | `cmd/ze/tui_menu_test.go` | Up/down keys move cursor, wrapping at boundaries | |
| `TestMenuModel_Select` | `cmd/ze/tui_menu_test.go` | Enter key returns selected command name | |
| `TestMenuModel_Quit` | `cmd/ze/tui_menu_test.go` | Esc/q triggers quit without selection | |
| `TestMenuModel_View` | `cmd/ze/tui_menu_test.go` | Rendered output contains section headers and item names | |
| `TestBuildMenuItems` | `cmd/ze/tui_menu_test.go` | Registry data correctly transformed into menu items | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A -- no numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-tui-fallback` | `test/cli/*.ci` | Non-TTY `ze` invocation still shows help text | |

### Interop Tests (MANDATORY for protocol features)
N/A -- not a protocol feature.

### Future (if deferring any tests)
- Sub-menu navigation for YANG verbs (e.g., selecting "show" then drilling into sub-commands) -- deferred to follow-up spec if desired

## Files to Modify
- `cmd/ze/ze_core_dispatch.go` - replace `zeUsage()` call with TTY check + TUI branch
- `cmd/ze/internal/helpfmt/helpfmt.go` - possibly expose section building for reuse (if not already)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | |
| YANG validation constraints | No | |
| YANG custom validators | No | |
| CLI commands/flags | No | |
| CLI grammar (action before identifier) | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | No | |
| Pipe completeness | No | |
| Env var registration | No | |
| Doctor check for runtime dependencies | No | |
| Prometheus counters/metrics | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- mention interactive launcher |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` -- document no-arg behavior |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | |
| 16 | Any changed source file is referenced by existing doc source anchors? | TBD | grep during implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | TBD | grep during implementation |

## Files to Create
- `cmd/ze/tui_menu.go` - BubbleTea model for the interactive launcher menu
- `cmd/ze/tui_menu_test.go` - unit tests for the menu model

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create -- check what exists |
| 3. Wiring phase | Wiring Test table -- TTY check in dispatch, failing wiring test |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7-13 | Standard flow |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- TTY detection + dispatch branch
   - Tests: `TestTUIMenuDispatch`, `TestNonTTYFallback`
   - Files: `cmd/ze/ze_core_dispatch.go`, `cmd/ze/tui_menu.go` (stub)
   - Verify: TTY branch exists, calls stub, non-TTY path unchanged

2. **Phase: Menu model** -- BubbleTea Init/Update/View for the menu
   - Tests: `TestMenuModel_Init`, `TestMenuModel_Navigation`, `TestMenuModel_Select`, `TestMenuModel_Quit`, `TestMenuModel_View`
   - Files: `cmd/ze/tui_menu.go`
   - Verify: model renders sections, handles keys, returns selection

3. **Phase: Registry integration** -- populate menu from live registry data
   - Tests: `TestBuildMenuItems`
   - Files: `cmd/ze/tui_menu.go`
   - Verify: menu shows real commands from `ListRootBySection()` + YANG verbs

4. **Phase: Dispatch loop** -- selected command feeds back into dispatch
   - Tests: `TestTUISelectDoctor`, `TestTUIEscapeExit`
   - Files: `cmd/ze/ze_core_dispatch.go`, `cmd/ze/tui_menu.go`
   - Verify: selecting a command runs it; Esc exits cleanly

5. **Functional tests** -- non-TTY fallback .ci test
6. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | TTY detection works on Linux and macOS; menu renders correctly |
| Naming | Menu file named `tui_menu.go`, build-tagged `ze_core` |
| Data flow | Menu items sourced from registry only, no hardcoded command lists |
| No-regression | Non-TTY path identical to current behavior |
| Build tag | New files use `//go:build ze_core` to match dispatch file |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `cmd/ze/tui_menu.go` exists | `ls cmd/ze/tui_menu.go` |
| `cmd/ze/tui_menu_test.go` exists | `ls cmd/ze/tui_menu_test.go` |
| Non-TTY behavior unchanged | `echo "" \| go run ./cmd/ze/ 2>&1` shows help text |
| Menu renders in terminal | `go run ./cmd/ze/` shows interactive menu |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | BubbleTea handles key input; no user-provided strings executed |
| No command injection | Selected command name comes from registry, not user text |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| BubbleTea rendering issue | Check terminal capability detection |
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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| BubbleTea for the menu | Minimal custom TUI, charmbracelet/huh forms | Already a dependency, proven in Ze's CLI, full control over rendering |
| Registry as sole data source | Hardcoded menu items | Stays in sync as commands are added/removed; zero maintenance |
| TTY gate with static help fallback | Always show menu (require `--help` for text) | Scripts and CI must not hang on interactive input |

## Known Limitations
- No sub-menu drill-down for YANG verbs (selecting "show" dispatches `ze show`, which has its own help)
- No search/filter (menu is small enough that arrow navigation suffices)
- No model/config selection (unlike Ollama, Ze commands don't need pre-selection of a resource)

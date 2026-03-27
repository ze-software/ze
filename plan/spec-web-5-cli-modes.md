# Spec: web-5 -- CLI Modes (Persistent CLI Bar and Terminal Mode)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-web-3-config-edit |
| Phase | 1/4 |
| Updated | 2026-03-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-web-0-umbrella.md` -- umbrella design decisions (D-18 through D-20)
4. `internal/component/cli/completer.go` -- Completer, Complete(), GhostText()
5. `internal/component/cli/editor.go` -- Editor struct, command dispatch, contextPath
6. `internal/component/web/handler.go` -- route registration (from web-1)
7. `internal/component/web/render.go` -- template rendering (from web-1)
8. `internal/component/web/templates/layout.html` -- page frame with CLI bar area (from web-1)

## Task

Implement the persistent CLI input bar and two CLI modes (integrated and terminal) for the Ze web interface.

The CLI bar is a fixed input field at the bottom of every page (except login). It accepts the same command grammar as the SSH CLI (`set`, `delete`, `show`, `edit`, `top`, `up`, `commit`, `discard`, etc.) and provides YANG-driven autocomplete via the same Completer the CLI uses. The prompt displays the current context path, synchronized with the breadcrumb navigation.

In integrated mode (default), CLI bar commands drive the GUI: `edit` changes the breadcrumb and content area, `set` updates the content and notification, `show` renders config output in the content area, and navigation commands (`top`, `up`) update both breadcrumb and content. GUI navigation (clicking breadcrumb links, list keys) reciprocally updates the CLI bar prompt.

In terminal mode, the entire content area becomes a text terminal delivering the same experience as the SSH CLI over HTTPS. Commands produce the same text output as the SSH CLI with full scrollback. Only the notification bar remains from the GUI frame. A toggle button switches between modes.

This is Phase 5 of the web interface. It builds on Phase 2 (config view) for schema walking, template rendering, and breadcrumb navigation, and depends on Phase 3 (config edit) because the CLI bar's `set`, `delete`, `commit`, and `discard` commands need the per-user Editor infrastructure from web-3. It does not depend on Phase 4 (admin commands).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/architecture/core-design.md` -- overall architecture, component model
  -> Constraint: web UI is a component (`internal/component/web/`), not a plugin
- [ ] `plan/spec-web-0-umbrella.md` -- umbrella design decisions
  -> Decision: D-18 persistent CLI bar, D-19 HTMX partial updates, D-20 CLI modes
  -> Constraint: CLI bar prompt synced with breadcrumb. Terminal mode replaces content area only

### RFC Summaries (MUST for protocol work)
- Not applicable (no protocol work in this spec)

### Source Files
- [ ] `internal/component/cli/completer.go` -- Completer with Complete(), GhostText()
  -> Decision: CLI bar autocomplete calls Complete() with URL-derived context path and partial input
- [ ] `internal/component/cli/editor.go` -- Editor struct, command dispatch (SetValue, DeleteValue, show, edit, top, up, CommitSession, discard), contextPath
  -> Decision: web handler uses the same Editor methods for command execution
  -> Constraint: contextPath is `[]string`, maps 1:1 to URL path segments and breadcrumb
- [ ] `internal/component/cli/editor_walk.go` -- walkPath, walkOrCreateIn, schemaGetter
  -> Constraint: list keys consume 2 path segments (list name + key value)
- [ ] `internal/component/cli/editor_session.go` -- EditSession with User, Origin, ID
  -> Constraint: web sessions use Origin "web"
- [ ] `internal/component/web/handler.go` -- route registration from web-1
  -> Constraint: CLI routes registered on the existing mux
- [ ] `internal/component/web/render.go` -- template rendering from web-1
  -> Decision: extend with CLI bar context data in all page renders
- [ ] `internal/component/web/templates/layout.html` -- page frame from web-1 (includes CLI bar area)
  -> Constraint: CLI bar is already part of the layout frame; this spec fills it with functional content

**Key insights:**
- Completer.Complete() takes a context path and partial input, returns candidate completions
- Completer has no internal synchronization. The web component must either (a) create a separate Completer per user session, or (b) ensure the shared Completer's tree is not modified during concurrent Complete() calls. Since Complete() is read-only and SetTree() is called only during config reload, option (b) with a read-write lock on SetTree() is sufficient
- Editor.contextPath is the same `[]string` as breadcrumb path segments and URL path segments
- The URL is the source of truth for context path. CLI bar POST response includes HX-Push-Url header to update browser URL. All state derives from the URL
- CLI commands return results that the web handler must translate into HTMX swap targets
- Terminal mode is a different rendering of the same Editor commands -- text output instead of HTML fragments
- The CLI bar is always visible in the layout frame; this spec wires it to the Editor and Completer

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write -> Constraint: annotations instead. -->
- [ ] `internal/component/cli/completer.go` -- provides Complete() with context-sensitive YANG completions
- [ ] `internal/component/cli/editor.go` -- Editor dispatches commands (SetValue, DeleteValue, show, edit, top, up, CommitSession, discard), maintains contextPath
- [ ] `internal/component/cli/editor_walk.go` -- walkPath for navigating schema + tree in parallel
- [ ] `internal/component/cli/editor_session.go` -- EditSession tracks per-user state
- [ ] `internal/component/web/handler.go` -- (from web-1) route registration, auth middleware
- [ ] `internal/component/web/render.go` -- (from web-1) template rendering with content negotiation
- [ ] `internal/component/web/templates/layout.html` -- (from web-1) page frame with areas for breadcrumb, content, notification, CLI bar

**Behavior to preserve:**
- Completer interface and Complete() logic unchanged (consumed, not modified)
- Editor command dispatch unchanged (consumed, not modified)
- contextPath as `[]string` representation unchanged
- EditSession with Origin field unchanged
- Layout frame structure from web-1 unchanged (CLI bar area already present)
- Breadcrumb navigation from web-2 unchanged

**Behavior to change:**
- None -- all existing behavior preserved. This spec adds CLI bar functionality and terminal mode to the existing layout frame

## Data Flow (MANDATORY -- see `rules/data-flow-tracing.md`)

### Entry Point
- CLI bar command submission: `POST /cli` with form body (command text + current mode)
- CLI bar autocomplete request: `GET /cli/complete?input=<partial>&path=<context-path>`
- Terminal mode toggle: `POST /cli/mode` with form body (target mode)
- Terminal mode command: `POST /cli/terminal` with form body (command text)
- GUI navigation: existing GET routes (from web-2) -- side effect updates CLI bar prompt

### Transformation Path
1. Browser captures Enter key in CLI bar input, sends HTMX POST to `/cli`
2. Auth middleware validates session cookie (from web-1)
3. CLI handler parses command text into verb + arguments
4. Handler looks up or creates the user's Editor (from EditSession)
5. For integrated mode: command dispatched to Editor, result translated to HTMX swap targets (content + breadcrumb OOB + notification OOB as appropriate per command type)
6. For terminal mode: command dispatched to Editor, result formatted as text output, appended to scrollback, returned as terminal content fragment
7. For autocomplete: context path + partial input passed to Completer.Complete(), candidates returned as HTML option list
8. Response delivered as HTMX partial (HTML fragment with swap targets)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Browser <-> Web server | HTTPS POST with form body (CLI command) + session cookie, HTTPS GET (autocomplete) + session cookie | [ ] |
| Web handler <-> Editor | Direct in-process function call (same as CLI) | [ ] |
| Web handler <-> Completer | Direct in-process call for autocomplete candidates | [ ] |
| HTMX <-> DOM | `hx-target`, `hx-swap-oob` for multi-element updates | [ ] |

### Integration Points
- `Editor` -- CLI bar commands dispatch through same Editor methods the CLI uses
- `Completer` -- autocomplete calls Complete() with context path derived from URL/breadcrumb
- `layout.html` -- CLI bar area receives functional template content
- Breadcrumb -- GUI navigation updates CLI bar prompt; CLI navigation updates breadcrumb (bidirectional sync)
- Content area -- integrated mode swaps content on `edit`/`show`; terminal mode replaces content with text terminal

### Architectural Verification
- [ ] No bypassed layers (CLI bar commands go through Editor, not direct tree manipulation)
- [ ] No unintended coupling (CLI bar uses Completer interface, not internal completion logic)
- [ ] No duplicated functionality (reuses Completer and Editor, does not reimplement command parsing)
- [ ] Zero-copy preserved where applicable (Editor returns results, web handler wraps in template)

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: Proves the feature is reachable from its intended entry point. -->
<!-- Without this, the feature exists in isolation -- unit tests pass but nothing calls it. -->
<!-- Every row MUST have a test name. "Deferred" / "TODO" / empty = spec cannot be marked done. -->
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| CLI bar `edit bgp` command via POST /cli | -> | Editor.contextPath updated, breadcrumb + content swapped | `test/plugin/web-cli-integrated.ci` |
| Terminal mode toggle via POST /cli/mode | -> | Content area replaced with text terminal | `test/plugin/web-cli-terminal.ci` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion. -->
<!-- The Implementation Audit cross-references these criteria. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any authenticated page load (except login) | CLI bar visible at bottom of page with input field, prompt, and mode toggle button |
| AC-2 | Page at URL `/config/bgp/peer/192.168.1.1/` | CLI bar prompt shows `ze[bgp peer 192.168.1.1]# ` matching breadcrumb path |
| AC-3 | Type `edit bgp` in CLI bar and press Enter | Breadcrumb updates to `/ > bgp`, content area shows BGP container view |
| AC-4 | Type `set remote-as 65002` in CLI bar while in peer context | Value is set, content area refreshes with updated value, notification shows confirmation |
| AC-5 | Type `show` in CLI bar | Config output rendered as text in content area |
| AC-6 | Type `top` in CLI bar | Breadcrumb clears to root, content shows root config view |
| AC-7 | Type `up` in CLI bar | Breadcrumb removes last segment(s), content shows parent view |
| AC-8 | Press Tab key in CLI bar with partial input | Autocomplete candidates appear (same completions as SSH CLI) |
| AC-9 | Click a breadcrumb link or list key in GUI | CLI bar prompt updates to match the new context path |
| AC-10 | Click mode toggle button | Content area switches to terminal mode (text CLI) |
| AC-11 | Terminal mode active | Content area shows full text CLI with scrollback buffer |
| AC-12 | Type command in terminal mode and press Enter | Text output identical to SSH CLI output appears in terminal area |
| AC-13 | Click mode toggle button while in terminal mode | Content area switches back to integrated mode (normal GUI) |
| AC-14 | Terminal mode active | Notification bar remains visible above or below terminal area |
| AC-15 | Press Enter with command text in CLI bar | HTMX POST sent to `/cli`, input field cleared after response |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCLIBarCommandParsing` | `internal/component/web/cli_test.go` | Input text correctly split into verb + arguments | |
| `TestCLIBarEditUpdatesContext` | `internal/component/web/cli_test.go` | `edit` command returns HTMX response with new breadcrumb + content swap targets | |
| `TestCLIBarSetUpdatesNotification` | `internal/component/web/cli_test.go` | `set` command returns content + notification OOB swap | |
| `TestCLIBarShowRendersOutput` | `internal/component/web/cli_test.go` | `show` command returns config text rendered in content area | |
| `TestCLIBarTopClearsContext` | `internal/component/web/cli_test.go` | `top` command returns root breadcrumb + root content | |
| `TestCLIBarUpNavigates` | `internal/component/web/cli_test.go` | `up` command removes last path segment, returns parent view | |
| `TestCLIBarAutocomplete` | `internal/component/web/cli_test.go` | GET /cli/complete with partial input returns Completer results as HTML | |
| `TestCLIBarPromptSync` | `internal/component/web/cli_test.go` | Context path from URL included in CLI bar template data as formatted prompt | |
| `TestTerminalModeToggle` | `internal/component/web/cli_test.go` | Toggle request returns terminal template for content area swap | |
| `TestTerminalModeCommand` | `internal/component/web/cli_test.go` | Command in terminal mode returns plain text output (not HTMX fragments) | |
| `TestIntegratedModeRestore` | `internal/component/web/cli_test.go` | Toggle back from terminal returns normal GUI content | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Command text length | 1-4096 bytes | 4096 bytes | 0 (empty -- rejected) | 4097 bytes (truncated or rejected) |
| Autocomplete input length | 0-1024 bytes | 1024 bytes | N/A (empty returns all top-level completions) | 1025 bytes (truncated or rejected) |

### Functional Tests
<!-- REQUIRED: Verify feature works from end-user perspective -->
<!-- New RPCs/APIs MUST have functional tests -- unit tests alone are NOT sufficient -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `web-cli-integrated` | `test/plugin/web-cli-integrated.ci` | CLI bar `edit bgp` updates breadcrumb + content, `top` returns to root, `up` navigates parent | |
| `web-cli-terminal` | `test/plugin/web-cli-terminal.ci` | Toggle to terminal mode, execute command, verify text output, toggle back to integrated | |

### Future (if deferring any tests)
- None -- all tests required for this spec

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
<!-- Check // Design: annotations on each file -- if the change affects behavior
     described in the referenced architecture doc, include the doc here too -->
- `internal/component/web/handler.go` -- register `/cli`, `/cli/complete`, `/cli/mode`, `/cli/terminal` routes on the existing mux
- `internal/component/web/render.go` -- include CLI bar context data (current path, formatted prompt, current mode) in all page render calls
- `internal/component/web/templates/layout.html` -- ensure CLI bar area receives template data for prompt, input field, autocomplete, and mode toggle button

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | No | N/A (web-1 handles `ze web` CLI) |
| Editor autocomplete | Yes -- CLI bar autocomplete reuses Completer | `internal/component/cli/completer.go` (consumed, not modified) |
| Functional test for new RPC/API | Yes | `test/plugin/web-cli-integrated.ci`, `test/plugin/web-cli-terminal.ci` |

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- add CLI bar and terminal mode to web interface feature list |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A (web-1 covers `ze web` command) |
| 4 | API/RPC added/changed? | No | N/A (HTTP endpoints, not YANG RPCs) |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | Yes | `docs/guide/web-interface.md` -- add CLI bar usage, autocomplete, integrated vs terminal mode |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- web CLI bar is a differentiator |
| 12 | Internal architecture changed? | No | N/A (extends existing web component) |

## Files to Create
- `internal/component/web/cli.go` -- CLI bar endpoint handlers: POST /cli (command dispatch for integrated mode), GET /cli/complete (autocomplete), POST /cli/mode (mode toggle), POST /cli/terminal (terminal mode command). Command parsing, Editor dispatch, HTMX response construction, prompt formatting
- `internal/component/web/templates/cli_bar.html` -- CLI input bar partial template: context prompt, text input field, mode toggle button, HTMX attributes for Enter submission and Tab autocomplete
- `internal/component/web/templates/terminal.html` -- terminal mode content template: text output area with scrollback, input line, HTMX attributes for command submission
- `internal/component/web/cli_test.go` -- unit tests for CLI bar handlers
- `test/plugin/web-cli-integrated.ci` -- functional test: CLI bar commands drive GUI updates
- `test/plugin/web-cli-terminal.ci` -- functional test: terminal mode text CLI works

## Implementation Steps

<!-- Steps must map to /implement stages. Each step should be a concrete phase of work,
     not a generic process description. The review checklists below are what /implement
     stages 5, 9, and 10 check against -- they MUST be filled with feature-specific items. -->

### /implement Stage Mapping

<!-- This table maps /implement stages to spec sections. Fill during design. -->
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

<!-- List concrete phases of work. Each phase follows TDD: write test -> fail -> implement -> pass.
     Phases should be ordered by dependency (e.g., schema before resolution, resolution before CLI). -->

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: CLI bar command parsing and prompt** -- Implement command text parsing (verb + arguments extraction), prompt formatting from context path, CLI bar template partial with input field and HTMX attributes
   - Tests: `TestCLIBarCommandParsing`, `TestCLIBarPromptSync`
   - Files: `internal/component/web/cli.go`, `internal/component/web/templates/cli_bar.html`, `internal/component/web/render.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Integrated mode command dispatch** -- Wire CLI bar commands to Editor methods. Each verb produces the appropriate HTMX swap response: `edit` swaps breadcrumb + content, `set` swaps content + notification, `show` renders text in content, `top` and `up` navigate with breadcrumb + content swap
   - Tests: `TestCLIBarEditUpdatesContext`, `TestCLIBarSetUpdatesNotification`, `TestCLIBarShowRendersOutput`, `TestCLIBarTopClearsContext`, `TestCLIBarUpNavigates`
   - Files: `internal/component/web/cli.go`, `internal/component/web/handler.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Autocomplete** -- Implement GET /cli/complete endpoint that calls Completer.Complete() with context path and partial input, returns HTML candidate list for the CLI bar dropdown
   - Tests: `TestCLIBarAutocomplete`
   - Files: `internal/component/web/cli.go`, `internal/component/web/handler.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Terminal mode** -- Implement terminal mode toggle (POST /cli/mode), terminal command endpoint (POST /cli/terminal), terminal content template with scrollback area. Terminal commands produce text output (same as SSH CLI), not HTMX fragments
   - Tests: `TestTerminalModeToggle`, `TestTerminalModeCommand`, `TestIntegratedModeRestore`
   - Files: `internal/component/web/cli.go`, `internal/component/web/templates/terminal.html`, `internal/component/web/handler.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Context synchronization** -- Wire GUI navigation (breadcrumb clicks, list key clicks) to update CLI bar prompt. Wire CLI navigation commands to update breadcrumb. Ensure bidirectional sync via HTMX OOB swaps
   - Tests: `TestCLIBarPromptSync` (extended), `TestCLIBarEditUpdatesContext` (verified bidirectional)
   - Files: `internal/component/web/render.go`, `internal/component/web/templates/layout.html`
   - Verify: tests fail -> implement -> tests pass

6. **Functional tests** -> Create after feature works. Cover user-visible behavior.
7. **Full verification** -> `make ze-verify` (lint + all ze tests except fuzz)
8. **Complete spec** -> Fill audit tables, write learned summary to `plan/learned/NNN-<name>.md`, delete spec from `plan/`. BLOCKING: summary is part of the commit, not a follow-up.

### Critical Review Checklist (/implement stage 5)

<!-- MANDATORY: Fill with feature-specific checks. /implement uses this table
     to verify the implementation. Generic checks from rules/quality.md always apply;
     this table adds what's specific to THIS feature. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | CLI bar prompt format matches `ze[<context-path>]# ` exactly. Autocomplete returns same candidates as SSH CLI |
| Naming | Routes use `/cli/` prefix consistently. Template names follow web-1 conventions |
| Data flow | Commands go through Editor (not direct tree manipulation). Autocomplete goes through Completer (not custom logic) |
| Context sync | GUI click updates CLI prompt. CLI `edit`/`top`/`up` updates breadcrumb. Both directions verified |
| Mode toggle | Terminal mode replaces content only. Notification bar remains. Toggle back restores GUI content |
| Rule: no-layering | No duplicate command parsing -- reuses Editor's existing command dispatch |
| Rule: design-principles | No identity wrappers around Completer or Editor. Direct calls |

### Deliverables Checklist (/implement stage 9)

<!-- MANDATORY: Every deliverable with a concrete verification method.
     /implement re-reads the spec and checks each item independently. -->
| Deliverable | Verification method |
|-------------|---------------------|
| `internal/component/web/cli.go` exists | `ls -la internal/component/web/cli.go` |
| `internal/component/web/templates/cli_bar.html` exists | `ls -la internal/component/web/templates/cli_bar.html` |
| `internal/component/web/templates/terminal.html` exists | `ls -la internal/component/web/templates/terminal.html` |
| `internal/component/web/cli_test.go` exists | `ls -la internal/component/web/cli_test.go` |
| `test/plugin/web-cli-integrated.ci` exists | `ls -la test/plugin/web-cli-integrated.ci` |
| `test/plugin/web-cli-terminal.ci` exists | `ls -la test/plugin/web-cli-terminal.ci` |
| Routes registered in handler.go | `grep -n "/cli" internal/component/web/handler.go` |
| CLI bar template included in layout | `grep -n "cli_bar" internal/component/web/templates/layout.html` |
| All 11 unit tests pass | `go test -run "TestCLIBar\|TestTerminal\|TestIntegrated" ./internal/component/web/...` |
| Both functional tests pass | `ze-test bgp plugin web-cli-integrated && ze-test bgp plugin web-cli-terminal` |

### Security Review Checklist (/implement stage 10)

<!-- MANDATORY: Feature-specific security concerns. /implement checks each item.
     Think about: untrusted input, injection, resource exhaustion, error leakage. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | CLI bar command text must be bounded (max length 4096). Reject or truncate oversized input |
| Injection | Command text passed to Editor must not allow template injection. Use html/template escaping for all rendered output |
| Auth on all endpoints | `/cli`, `/cli/complete`, `/cli/mode`, `/cli/terminal` all require valid session cookie (same middleware as config routes) |
| Error leakage | Editor errors (invalid command, unknown path) returned as user-friendly messages, not stack traces or internal paths |
| Resource exhaustion | Terminal mode scrollback buffer bounded (max lines or max bytes). Autocomplete response bounded (max candidates) |
| CSRF | CLI bar uses HTMX POST with session cookie. CSRF protection needed (e.g., SameSite cookie attribute, CSRF token in form) |

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

Not applicable -- no RFC protocol work in this spec.

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
<!-- For EVERY file in "Files to Create": ls -la <path> -- paste output. -->
<!-- For EVERY .ci file in Wiring Test and Functional Tests: ls -la <path> -- paste output. -->
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
<!-- For EVERY AC-N: independently verify. Do NOT copy from audit -- re-check. -->
<!-- Acceptable evidence: test name + pass output, grep showing function call, ls showing file. -->
<!-- NOT acceptable: "already checked", "should work", reference to audit table above. -->
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
<!-- For EVERY wiring test row: does the .ci test exist AND does it exercise the full path? -->
<!-- Read the .ci file content. Does it actually test what the wiring table claims? -->
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
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

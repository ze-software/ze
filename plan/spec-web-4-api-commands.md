# Spec: web-4 -- Admin/Operational Commands

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-web-2-config-view |
| Phase | - |
| Updated | 2026-03-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-web-0-umbrella.md` -- umbrella design decisions (D-3, D-7, D-8, D-11, D-21)
4. `plan/spec-web-2-config-view.md` -- config view patterns (breadcrumb, schema walking, content negotiation)
5. `internal/component/web/handler.go` -- route registration, auth middleware (from web-1)
6. `internal/component/web/render.go` -- template rendering, content negotiation (from web-1)
7. `internal/component/config/schema.go` -- Node types, schemaGetter interface
8. YANG `-api` and `-cmd` modules -- command tree structure (not `-conf`)

## Task

Implement the admin command tree navigation and execution under the `/admin/` URL prefix. The `/admin/` tier is for operational mutations: peer teardown, rib clear, daemon shutdown, and future fleet management. Read-only operational queries (bgp summary display, rib status) use the `/show/` prefix (view tier). Monitor uses the `/monitor/` prefix.

The web UI walks the YANG command tree (from `-api` and `-cmd` modules) using the same breadcrumb and container navigation pattern established in web-2 for config view. Command containers render as navigable links. Leaf commands (those with no sub-commands) render as a form with parameter fields and an "Execute" button. Executing a command via POST returns a titled result card. Multiple results stack with the most recent on top.

This is Phase 4 of the web interface. It builds on the navigation and template patterns from web-2 (config view) and the foundation from web-1 (HTTP server, TLS, auth, layout frame). It reuses the breadcrumb template, container link rendering, and content negotiation established in those phases.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/architecture/core-design.md` -- overall architecture, reactor, plugin model
  -> Constraint: web UI is a component (`internal/component/web/`), not a plugin
- [ ] `plan/spec-web-0-umbrella.md` -- umbrella design decisions
  -> Decision: D-3 both config and admin YANG in scope, D-7 `/admin/...` prefix for mutation commands with POST to execute, D-8 `command.html` template for titled result cards, D-11 content negotiation (Accept header or `?format=json`), D-21 command result rendering (titled card, stacking)
- [ ] `plan/spec-web-2-config-view.md` -- config view navigation patterns
  -> Decision: breadcrumb template, schema walking via schemaGetter, node-kind dispatch, content negotiation integration, HTMX partial responses
- [ ] `docs/architecture/api/commands.md` -- API command structure, RPC dispatch
  -> Constraint: command execution goes through the same RPC dispatch that CLI uses

### RFC Summaries (MUST for protocol work)
- Not applicable (no protocol work in this spec)

### Source Files
- [ ] `internal/component/web/handler.go` -- route registration, auth middleware (from web-1)
  -> Constraint: Admin routes registered on the same mux alongside config routes
- [ ] `internal/component/web/render.go` -- template parsing, execution, content negotiation (from web-1)
  -> Decision: extend with command result template data types
- [ ] `internal/component/config/schema.go` -- Node types, schemaGetter interface, Node.Kind()
  -> Constraint: YANG command modules produce the same Node hierarchy (ContainerNode for sub-trees, LeafNode for terminal commands)
- [ ] YANG `-api` and `-cmd` modules -- command tree structure
  -> Constraint: these are separate from `-conf` modules. The command tree has different semantics (execute, not configure) but same structural types

**Key insights:**
- The admin command tree uses `-api` and `-cmd` YANG modules, not the `-conf` modules used for config navigation
- Command containers are structurally identical to config containers for navigation purposes (breadcrumb, links to children)
- Leaf commands differ from config leaves: instead of a value input, they render a form with parameter fields and an "Execute" button
- Command execution is via POST (not GET) per D-7
- Result cards are a new rendering concept: titled header with command name, body with formatted output, cards stack most-recent-on-top per D-21
- Content negotiation applies to command results too: JSON returns raw command output per D-11
- View-tier commands (read-only operational queries like `bgp summary`) use `/show/` prefix, not `/admin/`. The `/admin/` prefix is for mutations only (teardown, clear, shutdown). Monitor uses `/monitor/` prefix

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/web/handler.go` -- (from web-1) registers `/config/` routes, auth middleware, layout rendering. No `/admin/` routes yet
- [ ] `internal/component/web/render.go` -- (from web-1) template parsing, content negotiation, template data structs for config views
- [ ] `internal/component/web/templates/breadcrumb.html` -- (from web-2) breadcrumb partial template
- [ ] `internal/component/web/templates/container.html` -- (from web-2) container node template (links to children, leaves as fields)
- [ ] `internal/component/config/schema.go` -- Node types, schemaGetter, ValueType enum

**Behavior to preserve:**
- All existing web-1 and web-2 behavior unchanged (auth, layout, config routes, config templates)
- Breadcrumb template reused as-is for admin navigation (same partial, different URL prefix)
- Content negotiation logic reused as-is (same Accept header and `?format=json` handling)
- Schema walking via schemaGetter reused for command tree traversal

**Behavior to change:**
- None -- all existing behavior preserved. This spec adds new handlers and templates for admin command navigation and execution.

## Data Flow (MANDATORY)

### Entry Point
- Browser sends GET request to `/admin/...` URL for admin command tree navigation (e.g., `/admin/peer/`)
- Browser sends POST request to `/admin/...` URL for admin command execution (e.g., `POST /admin/peer/192.168.1.1/teardown`)
- Format: HTTPS URL with path segments mapping to YANG command tree path

### Transformation Path
1. Auth middleware validates session cookie (from web-1)
2. Admin handler extracts URL path, strips `/admin/` prefix, splits into path segments
3. Schema walk: path segments validated against YANG command schema (from `-api` and `-cmd` modules) via schemaGetter
4. Node kind dispatch: if the resolved node is a container (has children), render as navigable links; if a leaf (no sub-commands), render as command form
5. For GET requests (navigation): assemble template data with breadcrumb path, child list, and render container or form template
6. For POST requests (execution): extract form parameters, dispatch command through RPC dispatch (same path CLI uses), collect output
7. Result assembly: command name becomes card header, command output becomes card body
8. Template execution: render result card template with assembled data
9. HTMX partial response: if `HX-Request` header present, return content fragment only (no layout wrapper)

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Browser <-> Web server | HTTPS GET/POST with session cookie, HTMX partial swap headers | [ ] |
| Web handler <-> YANG command schema | Direct in-process via schemaGetter.Get() chain on `-api`/`-cmd` modules | [ ] |
| Web handler <-> RPC dispatch | Direct in-process call to same command dispatch CLI uses | [ ] |

### Integration Points
- `schemaGetter` interface -- web handler walks command schema to determine container vs leaf and available children
- YANG `-api` and `-cmd` modules -- provide the command tree structure (separate from `-conf`)
- RPC dispatch -- command execution goes through the same dispatch mechanism the CLI uses
- Web-1 layout template -- admin views rendered inside the layout frame's content area
- Web-1 auth middleware -- every admin request authenticated via session cookie before reaching the handler
- Web-1 content negotiation -- JSON response returned when `Accept: application/json` or `?format=json`
- Web-2 breadcrumb template -- reused for `/admin/` path rendering with clickable segments

### Architectural Verification
- [ ] No bypassed layers (commands dispatched through standard RPC path, not direct function calls)
- [ ] No unintended coupling (admin handler uses same schemaGetter interface, does not import specific command packages)
- [ ] No duplicated functionality (reuses breadcrumb, container link rendering, content negotiation from web-1/web-2)
- [ ] Zero-copy preserved where applicable (command output passed through, not re-serialized)

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: Proves the feature is reachable from its intended entry point. -->
<!-- Without this, the feature exists in isolation -- unit tests pass but nothing calls it. -->
<!-- Every row MUST have a test name. "Deferred" / "TODO" / empty = spec cannot be marked done. -->
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| POST `/admin/peer/192.168.1.1/teardown` with session cookie | -> | Admin handler dispatches mutation command, renders result card | `test/plugin/web-admin-command.ci` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion. -->
<!-- The Implementation Audit cross-references these criteria. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | GET `/admin/` | Renders root admin view with top-level mutation command modules as navigable links |
| AC-2 | GET `/admin/peer/` | Renders peer admin tree with sub-commands as links |
| AC-3 | Leaf command (no sub-commands) at its URL | Renders with parameter form fields and "Execute" button |
| AC-4 | POST `/admin/peer/192.168.1.1/teardown` | Executes the mutation command and returns result card |
| AC-5 | Result card after command execution | Has titled header with command name ("peer 192.168.1.1 teardown") and output in body |
| AC-6 | Execute a second command after the first | New card appears above the previous one (most recent on top) |
| AC-7 | GET `/admin/peer/192.168.1.1/teardown?format=json` | Returns JSON command output (not HTML) |
| AC-8 | Breadcrumb at `/admin/peer/` | Shows `[back] / > admin > peer` with clickable segments |
| AC-9 | Back button at `/admin/peer/` | Navigates to `/admin/` |
| AC-10 | Command with parameters (e.g., `peer 192.168.1.1 teardown`) | Renders form with pre-filled path and parameter fields |
| AC-11 | Command execution error | Renders error in result card (error indicator in card header or body styling) |
| AC-12 | `bgp summary` (read-only query) | Served under `/show/bgp/summary` (view tier), not `/admin/` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAdminRouteDispatch` | `internal/component/web/handler_admin_test.go` | GET `/admin/peer/` returns command tree with child links | |
| `TestAdminCommandExecution` | `internal/component/web/handler_admin_test.go` | POST `/admin/peer/192.168.1.1/teardown` dispatches command and returns result card data | |
| `TestCommandResultCard` | `internal/component/web/handler_admin_test.go` | Result template data includes command name as header and formatted output as body | |
| `TestCommandResultCardStack` | `internal/component/web/handler_admin_test.go` | Multiple results render in reverse chronological order (most recent first) | |
| `TestCommandErrorCard` | `internal/component/web/handler_admin_test.go` | Command execution error produces error-styled card (error flag set in template data) | |
| `TestAdminContentNegotiation` | `internal/component/web/handler_admin_test.go` | `?format=json` on command result returns JSON instead of HTML | |
| `TestAdminBreadcrumb` | `internal/component/web/handler_admin_test.go` | `/admin/peer/` produces breadcrumb with segments `[admin, peer]` and correct link URLs | |
| `TestCommandFormRendering` | `internal/component/web/handler_admin_test.go` | Leaf command produces form data with parameter fields and execute action URL | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| No numeric inputs in this spec -- command parameters are validated by the RPC dispatch layer, not the web handler | | | | |

### Functional Tests
<!-- REQUIRED: Verify feature works from end-user perspective -->
<!-- New RPCs/APIs MUST have functional tests -- unit tests alone are NOT sufficient -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `web-admin-command` | `test/plugin/web-admin-command.ci` | POST `/admin/peer/192.168.1.1/teardown` with session cookie -> mutation command dispatched -> result card rendered | |

### Future (if deferring any tests)
- None -- no tests deferred

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
<!-- Check // Design: annotations on each file -- if the change affects behavior
     described in the referenced architecture doc, include the doc here too -->
- `internal/component/web/handler.go` -- register `/admin/` routes on the existing mux
- `internal/component/web/render.go` -- add command result template data types (CommandResultData, CommandFormData)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No -- consumes existing `-api`/`-cmd` YANG modules, does not add new RPCs | |
| CLI commands/flags | No -- no new CLI commands (uses existing `ze web` from web-1) | |
| Editor autocomplete | No -- not applicable to command forms | |
| Functional test for new RPC/API | Yes | `test/plugin/web-admin-command.ci` |

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- add web admin command execution |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No -- consumes existing RPCs via web, does not add new ones | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/web-interface.md` -- add admin commands section (navigation, execution, result cards) |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |

## Files to Create
- `internal/component/web/handler_admin.go` -- admin command tree navigation handler, mutation command execution dispatch, result card assembly
- `internal/component/web/templates/command.html` -- command result card template (titled header with command name, body with formatted output)
- `internal/component/web/templates/command_form.html` -- command parameter form template (parameter fields, execute button, action URL)
- `internal/component/web/handler_admin_test.go` -- unit tests for admin handler
- `test/plugin/web-admin-command.ci` -- functional test for admin command execution

## Implementation Steps

<!-- Steps must map to /implement stages. Each step should be a concrete phase of work,
     not a generic process description. The review checklists below are what /implement
     stages 5, 9, and 10 check against -- they MUST be filled with feature-specific items. -->

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

1. **Phase: Admin route registration and command tree navigation** -- Register `/admin/` routes on the mux. Implement GET handler that walks the YANG command schema (from `-api` and `-cmd` modules) and renders container nodes as navigable links with breadcrumb.
   - Tests: `TestAdminRouteDispatch`, `TestAdminBreadcrumb`
   - Files: `handler.go` (route registration), `handler_admin.go` (new), `render.go` (template data types)
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Command form rendering** -- Implement leaf command detection and form rendering. When a resolved schema node has no children (leaf command), render a form with parameter fields and an "Execute" button instead of navigation links.
   - Tests: `TestCommandFormRendering`
   - Files: `handler_admin.go`, `templates/command_form.html` (new)
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Command execution and result cards** -- Implement POST handler that extracts form parameters, dispatches the mutation command through RPC dispatch, and renders the result as a titled card. Handle execution errors with error-styled cards.
   - Tests: `TestAdminCommandExecution`, `TestCommandResultCard`, `TestCommandErrorCard`
   - Files: `handler_admin.go`, `templates/command.html` (new)
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Result card stacking and content negotiation** -- Implement card stacking (most recent on top) for multiple command executions. Implement JSON response for `?format=json` on command results.
   - Tests: `TestCommandResultCardStack`, `TestAdminContentNegotiation`
   - Files: `handler_admin.go`, `render.go`
   - Verify: tests fail -> implement -> tests pass

5. **Functional tests** -- Create `.ci` test that exercises command execution end-to-end.
   - Tests: `test/plugin/web-admin-command.ci`
   - Files: `test/plugin/web-admin-command.ci` (new)
   - Verify: functional test passes

6. **Full verification** -- `make ze-verify` (lint + all ze tests except fuzz)

7. **Complete spec** -- Fill audit tables, write learned summary to `plan/learned/NNN-web-4-api-commands.md`, delete spec from `plan/`. BLOCKING: summary is part of the commit, not a follow-up.

### Critical Review Checklist (/implement stage 5)

<!-- MANDATORY: Fill with feature-specific checks. /implement uses this table
     to verify the implementation. Generic checks from rules/quality.md always apply;
     this table adds what's specific to THIS feature. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Command tree uses `-api`/`-cmd` YANG modules, not `-conf`. Result card header matches the executed command path. Card stacking order is most-recent-first. Admin tier contains only mutations, not read-only queries |
| Naming | URL paths use `/admin/` prefix for mutations, `/show/` for read-only queries. Template data field names are consistent with web-2 patterns. JSON keys use kebab-case |
| Data flow | GET navigates schema tree, POST dispatches through RPC. No direct command function calls bypassing dispatch |
| Rule: no-layering | No duplicate navigation logic -- reuses web-2 breadcrumb and container patterns |
| Rule: integration-completeness | `.ci` test exercises POST to `/admin/peer/192.168.1.1/teardown` and verifies result card output |

### Deliverables Checklist (/implement stage 9)

<!-- MANDATORY: Every deliverable with a concrete verification method.
     /implement re-reads the spec and checks each item independently. -->
| Deliverable | Verification method |
|-------------|---------------------|
| `handler_admin.go` exists with GET and POST handlers | `ls -la internal/component/web/handler_admin.go` |
| `templates/command.html` exists | `ls -la internal/component/web/templates/command.html` |
| `templates/command_form.html` exists | `ls -la internal/component/web/templates/command_form.html` |
| `/admin/` routes registered on mux | grep for `/admin/` in `handler.go` |
| Result card template data type defined | grep for `CommandResult` in `render.go` |
| Functional test exists | `ls -la test/plugin/web-admin-command.ci` |
| All unit tests pass | `go test -run 'TestAdmin\|TestCommand' ./internal/component/web/...` |

### Security Review Checklist (/implement stage 10)

<!-- MANDATORY: Feature-specific security concerns. /implement checks each item.
     Think about: untrusted input, injection, resource exhaustion, error leakage. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | Command path segments validated against YANG schema before dispatch (no arbitrary command injection) |
| Template escaping | All command output rendered through `html/template` auto-escaping (no raw HTML injection from command results) |
| Auth enforcement | Every `/admin/` route goes through auth middleware with session cookie validation (no unauthenticated command execution) |
| Error leakage | Command execution errors do not expose internal stack traces or file paths in result cards |
| Resource exhaustion | Result card stacking is bounded (browser-side concern, not server-side -- each POST returns one card) |
| Parameter injection | Form parameter values sanitized before passing to RPC dispatch |

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
- (to be filled during implementation)

### Bugs Found/Fixed
- (to be filled during implementation)

### Documentation Updates
- (to be filled during implementation)

### Deviations from Plan
- (to be filled during implementation)

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
- [ ] AC-1..AC-12 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-web-4-api-commands.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.

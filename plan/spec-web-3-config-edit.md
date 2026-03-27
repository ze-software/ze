# Spec: web-3 -- Config Editing

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-web-2-config-view |
| Phase | 1/8 |
| Updated | 2026-03-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-web-0-umbrella.md` -- umbrella spec with design decisions D-2, D-7, D-12, D-13, D-15, D-16
4. `internal/component/cli/editor.go` -- Editor struct, draft/commit/discard semantics
5. `internal/component/cli/editor_session.go` -- EditSession, per-user change files
6. `internal/component/web/handler_config.go` -- config GET handlers from web-2
7. `internal/component/web/render.go` -- template rendering from web-2
8. `internal/component/ssh/auth.go` -- AuthenticateUser, username extraction

## Task

Add config editing to the web interface. This covers setting and deleting leaf values via POST, per-user draft management via EditSession with Origin "web", inline diff rendering for modified fields, a commit page showing the full diff with confirmation, and draft discard. All editing operations reuse the CLI editor infrastructure -- the web handler creates EditSession instances keyed by username from the session cookie (D-5 from umbrella; JSON API consumers may use Basic Auth) and delegates to Editor methods.

This is Phase 3 of the web interface (child of `spec-web-0-umbrella.md`). It builds on the read-only config display from `spec-web-2-config-view.md` and adds write operations.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `docs/architecture/core-design.md` -- overall architecture, component model
  -> Constraint: web UI is a component, not a plugin (D-1 from umbrella)
- [ ] `docs/architecture/zefs-format.md` -- zefs storage, change file format
  -> Constraint: change files stored as `config.conf.change.<username>` in zefs
- [ ] `plan/spec-web-0-umbrella.md` -- umbrella spec, design decisions
  -> Decision: D-2 (full read-write editor), D-5 (session cookie for browsers, Basic Auth for JSON API), D-7 (verb-first URL paths: `/config/<verb>/<yang-path>`), D-12 (one draft per user), D-13 (zefs credentials, EditSession origin "web"), D-15 (inline diff with color + hover), D-16 (set/discard auto-navigate back one level, commit is dedicated page)

### RFC Summaries (MUST for protocol work)
- Not applicable -- this is a web UI feature, not protocol work.

**Key insights:**
- Editor already supports SetValue/DeleteValue/DeleteContainer/CommitSession/Discard/Diff; web handler delegates to these methods
- EditSession carries Origin field ("web" for web sessions) and User from session cookie username (D-5)
- Per-user change files use sanitized username in filename, enabling multi-user concurrent editing
- Conflict detection scans all `config.conf.change.*` files (same mechanism as CLI)
- YANG type validation happens inside Editor.SetValue() -- web handler does not need separate validation logic
- Inline diff requires comparing user's draft values against committed tree values (via Editor.Diff())

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write -> Constraint: annotations instead. -->
- [ ] `internal/component/cli/editor.go` -- Editor struct with SetValue(path, key, value), DeleteValue(path, key), DeleteContainer(path, name), CommitSession(), Discard(), Diff() methods
  -> Constraint: Editor.SetValue() validates leaf values against YANG type before applying
  -> Constraint: Editor.Diff() returns diff between draft and committed config
- [ ] `internal/component/cli/editor_session.go` -- EditSession struct with User, Origin, ID fields; per-user change file creation
  -> Constraint: EditSession constructor takes User and Origin parameters
  -> Constraint: change file path derived from sanitized username
- [ ] `internal/component/cli/editor_walk.go` -- walkPath, walkOrCreateIn for navigating schema tree
  -> Constraint: list keys consume 2 path segments (name + key value)
- [ ] `internal/component/web/handler_config.go` -- GET handlers for config view (from web-2)
  -> Constraint: URL path parsing and schema walking already implemented for read-only view
- [ ] `internal/component/web/render.go` -- template rendering infrastructure (from web-2)
  -> Constraint: template data structures already defined for container, list, leaf views
- [ ] `internal/component/web/templates/container.html` -- container view template (from web-2)
- [ ] `internal/component/web/templates/list.html` -- list view template (from web-2)
- [ ] `internal/component/web/templates/notification.html` -- notification partial (from web-2)
- [ ] `internal/component/ssh/auth.go` -- AuthenticateUser(), username extraction
  -> Constraint: auth middleware already provides username in request context via session cookie (from web-1, D-5)

**Behavior to preserve:**
- All read-only config view behavior from web-2 unchanged
- Editor.SetValue() validation semantics unchanged -- YANG type checking happens inside Editor
- Editor.Diff() output format unchanged
- Per-user change file naming convention unchanged
- AuthenticateUser behavior unchanged
- Breadcrumb navigation and template rendering unchanged

**Behavior to change:**
- Container and list templates gain diff indicators (CSS class on modified fields, data attribute for old value)
- Notification partial gains change count display and feedback messages
- Config handler gains POST routes for verb-first paths: `/config/set/<path>`, `/config/delete/<path>`, `/config/commit`, `/config/discard`
- Leaf input template gains diff class and data-old-value attribute

## Data Flow (MANDATORY -- see `rules/data-flow-tracing.md`)

### Entry Point
- Browser sends POST to `/config/set/<yang-path>/` or `/config/delete/<yang-path>/` with form body containing leaf name and value
- Session cookie (D-5) provides authenticated username for draft keying. JSON API consumers may use Basic Auth as fallback
- Browser sends POST to `/config/commit` or `/config/discard` for draft lifecycle

### Transformation Path
1. Auth middleware extracts username from session cookie (or Basic Auth for JSON API consumers). Already exists from web-1 (D-5)
2. Config handler matches URL pattern and extracts verb prefix (set, delete, commit, discard) from the first segment after `/config/`
3. Handler looks up or creates EditSession for this username (keyed by username, Origin "web"). Per-user Editor map (`map[string]*userSession`) protected by a `sync.RWMutex` for map access. Each userSession holds the Editor and a per-user `sync.Mutex` wrapping all Editor method calls. HTTP handlers acquire the per-user lock before calling any Editor method, release after
4. For set: handler reads form body (leaf name + value), calls Editor.SetValue(path, key, value). For TypeBool leaves, web handler transforms HTML checkbox presence to "true" and absence to "false" before calling SetValue
5. For delete: handler reads form body (leaf name), calls Editor.DeleteValue(path, key) or Editor.DeleteContainer(path, name) depending on the schema node type
6. For commit: handler calls Editor.CommitSession(), which writes change file to committed config. If CommitSession() detects a conflict (another user committed overlapping changes), the commit page re-renders with an error message listing conflicting paths; the user's draft is preserved
7. For discard: handler calls Editor.Discard(), which removes user's change file
8. Editor validates leaf value against YANG type (for set) and returns error if invalid
9. Handler formats response: redirect (set/delete/discard) or commit page HTML (commit GET) or redirect to config root (commit POST, discard POST)

### Draft Persistence and Session Management

On server restart, existing change files in zefs are detected. When a user's first authenticated request arrives, the web handler checks for an existing change file and initializes the Editor from it.

Per-user Editor map has a maximum size (configurable, default 50). Inactive sessions (no activity for 1 hour) are evicted from the in-memory map. Change files persist in zefs regardless of eviction -- re-initialized on next request.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| HTTP request -> Handler | URL path parsing + form body extraction | [ ] |
| Handler -> Editor | Direct in-process call to Editor.SetValue/DeleteValue/DeleteContainer/CommitSession/Discard, guarded by per-user mutex | [ ] |
| Editor -> Storage | Change file write via zefs Storage interface | [ ] |
| Handler -> Template | Render data includes diff info (modified fields, old values) | [ ] |

### Integration Points
- `cli.Editor` -- web handler creates/reuses Editor per user, delegates all edit operations via SetValue/DeleteValue/DeleteContainer/CommitSession/Discard/Diff
- `cli.EditSession` -- web handler creates EditSession with Origin "web" and username from session cookie
- `config.Tree` -- diff rendering compares draft tree values against committed tree values (via Editor.Diff())
- Auth middleware (from web-1) -- provides username in request context via session cookie (D-5)
- Template rendering (from web-2) -- extended with diff indicators

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: Proves the feature is reachable from its intended entry point. -->
<!-- Without this, the feature exists in isolation -- unit tests pass but nothing calls it. -->
<!-- Every row MUST have a test name. "Deferred" / "TODO" / empty = spec cannot be marked done. -->
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| POST `/config/set/<path>/` with session cookie | -> | Editor.SetValue() called, change file written | `test/plugin/web-config-edit.ci` |
| POST `/config/commit` with session cookie | -> | Editor.CommitSession() called, config persisted | `test/plugin/web-config-commit.ci` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion. -->
<!-- The Implementation Audit cross-references these criteria. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | POST `/config/set/bgp/peer/192.168.1.1/` with form leaf=remote-as, value=65002, valid session cookie | Value set in user's draft (change file updated) via Editor.SetValue(path, key, value) |
| AC-2 | Successful set operation | Response redirects back one level (to `/config/edit/bgp/peer/192.168.1.1/`) |
| AC-3 | POST `/config/delete/bgp/peer/192.168.1.1/` with form leaf=description | Value deleted from user's draft via Editor.DeleteValue(path, key) |
| AC-4 | Successful delete operation | Response redirects back one level |
| AC-5 | GET container/list view where user has modified a field | Modified field element has CSS class indicating modification (e.g., `modified`) |
| AC-6 | Hovering over modified field in browser | Old value visible via title attribute or data attribute tooltip |
| AC-7 | GET `/config/compare` with uncommitted changes | Page shows diff of all uncommitted changes (via Editor.Diff()), color-coded: adds green, deletes red, changes yellow |
| AC-8 | POST `/config/commit` | Changes committed via Editor.CommitSession(), response redirects to config root (`/config/edit/`) with success notification |
| AC-9 | POST `/config/discard` | Draft discarded via Editor.Discard(), response redirects to config root (`/config/edit/`) |
| AC-10 | Two different authenticated users (user-a, user-b) each POST set operations | Each user has independent EditSession and separate change file |
| AC-11 | User A sets a value; User B views same path before commit | User B does not see User A's uncommitted change |
| AC-12 | POST set with invalid value (e.g., "abc" for a TypeUint32 leaf) | Validation error returned in notification area, value not set in draft |
| AC-13 | Successful set operation | Notification area shows feedback message ("field X updated") |
| AC-14 | Any page view with uncommitted changes in user's draft | Notification area persistently shows change count ("3 uncommitted changes") |
| AC-15 | `who` command via CLI bar or API | Web users listed alongside CLI users, web users show Origin "web" |
| AC-16 | CommitSession() detects a conflict (another user committed overlapping changes) | Commit page re-renders with an error message listing conflicting paths. User's draft is preserved |
| AC-17 | Server restart with existing change files in zefs, then user's first authenticated request arrives | Web handler detects existing change file and initializes the Editor from it |
| AC-18 | Per-user Editor map reaches maximum size (default 50) and a session has been inactive for 1 hour | Inactive session is evicted from in-memory map. Change file persists in zefs. Next request re-initializes from change file |
| AC-19 | POST `/config/discard` when already at config root | Response redirects to config root (`/config/edit/`). Global actions (commit, discard) always redirect to config root |
| AC-20 | POST set for a TypeBool leaf with HTML checkbox checked | Web handler converts checkbox presence to "true" before calling Editor.SetValue() |
| AC-21 | POST set for a TypeBool leaf with HTML checkbox unchecked (absent from form) | Web handler converts checkbox absence to "false" before calling Editor.SetValue() |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSetVerbDispatch` | `internal/component/web/handler_config_test.go` | POST `/config/set/<path>/` with form body dispatches to Editor.SetValue(path, key, value) with correct arguments | |
| `TestDeleteVerbDispatch` | `internal/component/web/handler_config_test.go` | POST `/config/delete/<path>/` with form body dispatches to Editor.DeleteValue(path, key) with correct arguments | |
| `TestSetNavigatesBack` | `internal/component/web/handler_config_test.go` | Response after successful set is a redirect one level up from the current path | |
| `TestDiscardRedirectsToRoot` | `internal/component/web/handler_config_test.go` | Response after discard is a redirect to config root (`/config/edit/`) | |
| `TestDraftPerUser` | `internal/component/web/editor_test.go` | Two different usernames produce independent Editor instances with separate change files | |
| `TestEditSessionOriginWeb` | `internal/component/web/editor_test.go` | EditSession created via web handler has Origin field set to "web" | |
| `TestEditorConcurrentAccess` | `internal/component/web/editor_test.go` | Concurrent goroutines calling SetValue on same user's Editor via per-user mutex do not race (verified with race detector) | |
| `TestInlineDiffModifiedField` | `internal/component/web/render_test.go` | Template data for a modified leaf includes diff CSS class and old value attribute | |
| `TestInlineDiffUnmodifiedField` | `internal/component/web/render_test.go` | Template data for an unmodified leaf has no diff class and no old value attribute | |
| `TestCommitPageDiff` | `internal/component/web/render_test.go` | Commit page template data includes all changes grouped by path with add/delete/change types (using Editor.Diff() output) | |
| `TestValidationError` | `internal/component/web/handler_config_test.go` | Invalid value (wrong type for leaf) returns error response, Editor.SetValue() returns error | |
| `TestChangeCount` | `internal/component/web/editor_test.go` | Change count for user's draft reflects number of uncommitted modifications | |
| `TestBoolCheckboxConversion` | `internal/component/web/handler_config_test.go` | HTML checkbox presence converted to "true", absence converted to "false" before calling Editor.SetValue() for TypeBool leaves | |
| `TestCommitConflict` | `internal/component/web/handler_config_test.go` | When CommitSession() returns a conflict error, commit page re-renders with error message listing conflicting paths; draft preserved | |
| `TestDraftPersistenceOnRestart` | `internal/component/web/editor_test.go` | Existing change file in zefs is detected on first authenticated request; Editor initialized from it | |
| `TestSessionEviction` | `internal/component/web/editor_test.go` | Inactive session (no activity for 1 hour) is evicted from in-memory map; change file persists in zefs | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Leaf value for TypeUint32 | 0 to 4294967295 | 4294967295 | N/A (unsigned) | 4294967296 |
| Leaf value for TypeUint16 | 0 to 65535 | 65535 | N/A (unsigned) | 65536 |

### Functional Tests
<!-- REQUIRED: Verify feature works from end-user perspective -->
<!-- New RPCs/APIs MUST have functional tests -- unit tests alone are NOT sufficient -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `web-config-edit` | `test/plugin/web-config-edit.ci` | POST to `/config/set/<path>/` with session cookie sets value in user's draft, change file written | |
| `web-config-commit` | `test/plugin/web-config-commit.ci` | POST to `/config/commit` applies draft changes via CommitSession(), redirects to config root | |
| `web-config-multiuser` | `test/plugin/web-config-multiuser.ci` | POST set as user-a, GET same path as user-b, verify user-b does not see user-a's uncommitted change | |

### Future (if deferring any tests)
- Concurrent multi-user editing stress test -- requires multiple simultaneous HTTP clients, deferred to chaos testing infrastructure

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
<!-- Check // Design: annotations on each file -- if the change affects behavior
     described in the referenced architecture doc, include the doc here too -->
- `internal/component/web/handler_config.go` -- add POST handlers for verb-first paths `/config/set/<path>`, `/config/delete/<path>`, `/config/commit` (GET and POST), `/config/discard`; dispatch to Editor.SetValue/DeleteValue/DeleteContainer/CommitSession/Discard; form body parsing; HTML checkbox-to-boolean conversion for TypeBool leaves; redirect responses
- `internal/component/web/render.go` -- add diff rendering logic: compare draft tree vs committed tree, annotate template data with modified flag and old value for each leaf
- `internal/component/web/templates/container.html` -- add CSS class for modified fields, data attribute for old value on leaf inputs
- `internal/component/web/templates/list.html` -- add CSS class for modified fields in right panel detail view
- `internal/component/web/templates/notification.html` -- add change count display, feedback message area (field updated, validation errors)
- `internal/component/web/templates/leaf_input.html` -- add conditional diff class and data-old-value attribute when leaf is modified

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A -- web editing uses HTTP routes, not YANG RPCs |
| CLI commands/flags | No | N/A -- no new CLI commands in this phase |
| Editor autocomplete | No | N/A -- CLI bar autocomplete is phase 5 |
| Functional test for new RPC/API | Yes | `test/plugin/web-config-edit.ci`, `test/plugin/web-config-commit.ci`, `test/plugin/web-config-multiuser.ci` |

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- add web config editing (set, delete, commit, discard) |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` -- document POST `/config/set/<path>`, `/config/delete/<path>`, `/config/commit`, `/config/discard` endpoints |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | Yes | `docs/guide/web-interface.md` -- add editing section (set, delete, commit, discard, inline diff, conflict detection) |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- add web-based config editing capability |
| 12 | Internal architecture changed? | No | N/A |

## Files to Create
- `internal/component/web/editor.go` -- per-user Editor management: Editor map (`map[string]*userSession`) protected by `sync.RWMutex` for map access. Each userSession holds the Editor and a per-user `sync.Mutex` wrapping all Editor method calls. EditSession creation with Origin "web", change count, draft lifecycle (create on first edit, remove on discard). Maximum map size (configurable, default 50) with 1-hour inactivity eviction. On startup, detects existing change files in zefs and re-initializes Editors on first authenticated request
- `internal/component/web/templates/commit.html` -- commit diff page: all changes across all paths (via Editor.Diff()), color-coded (green for adds, red for deletes, yellow for changes), confirm button to POST `/config/commit`. Conflict error display area for re-rendering when CommitSession() detects overlapping changes
- `test/plugin/web-config-edit.ci` -- functional test: configure web server, POST to `/config/set/<path>/` with session cookie, verify change file written with correct value
- `test/plugin/web-config-commit.ci` -- functional test: configure web server, POST set to stage a change, POST to `/config/commit`, verify config applied and change file removed
- `test/plugin/web-config-multiuser.ci` -- functional test: POST set as user-a, GET same path as user-b, verify user-b does not see user-a's uncommitted change

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

1. **Phase: Per-user editor management** -- create `editor.go` with username-to-Editor map (protected by `sync.RWMutex`), per-user `sync.Mutex` wrapping Editor method calls, EditSession creation with Origin "web", change count tracking, max map size with inactivity eviction, draft persistence (detect existing change files on first request)
   - Tests: `TestDraftPerUser`, `TestEditSessionOriginWeb`, `TestChangeCount`, `TestEditorConcurrentAccess`, `TestDraftPersistenceOnRestart`, `TestSessionEviction`
   - Files: `internal/component/web/editor.go`, `internal/component/web/editor_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Set and delete handlers** -- add POST handlers for verb-first paths `/config/set/<path>` and `/config/delete/<path>` in handler_config.go, parse form body, convert HTML checkbox to boolean for TypeBool leaves, delegate to Editor.SetValue() and Editor.DeleteValue()/DeleteContainer(), redirect back one level
   - Tests: `TestSetVerbDispatch`, `TestDeleteVerbDispatch`, `TestSetNavigatesBack`, `TestValidationError`, `TestBoolCheckboxConversion`
   - Files: `internal/component/web/handler_config.go`, `internal/component/web/handler_config_test.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Commit and discard handlers** -- add GET `/config/compare` (shows diff page via Editor.Diff()), POST `/config/commit` (executes CommitSession(), handles conflict re-render), POST `/config/discard` (discards draft, redirects to config root)
   - Tests: `TestCommitPageDiff`, `TestDiscardRedirectsToRoot`, `TestCommitConflict`
   - Files: `internal/component/web/handler_config.go`, `internal/component/web/templates/commit.html`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Inline diff rendering** -- modify render.go to compare draft vs committed tree, annotate template data with modified flag and old value; update container.html, list.html, leaf_input.html with conditional CSS class and data-old-value attribute
   - Tests: `TestInlineDiffModifiedField`, `TestInlineDiffUnmodifiedField`
   - Files: `internal/component/web/render.go`, `internal/component/web/render_test.go`, `internal/component/web/templates/container.html`, `internal/component/web/templates/list.html`, `internal/component/web/templates/leaf_input.html`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Notification updates** -- update notification.html to show change count and feedback messages (field updated, validation errors)
   - Tests: `TestChangeCount` (already passing from phase 1), verify notification template renders count
   - Files: `internal/component/web/templates/notification.html`
   - Verify: template renders change count and feedback correctly

6. **Functional tests** -- create .ci tests exercising set, commit, and multi-user isolation from user's perspective
   - Tests: `web-config-edit`, `web-config-commit`, `web-config-multiuser`
   - Files: `test/plugin/web-config-edit.ci`, `test/plugin/web-config-commit.ci`, `test/plugin/web-config-multiuser.ci`
   - Verify: functional tests pass end-to-end

7. **Full verification** -- `make ze-verify` (lint + all ze tests except fuzz)

8. **Complete spec** -- fill audit tables, write learned summary to `plan/learned/NNN-web-3-config-edit.md`, delete spec from `plan/`. BLOCKING: summary is part of the commit, not a follow-up.

### Critical Review Checklist (/implement stage 5)

<!-- MANDATORY: Fill with feature-specific checks. /implement uses this table
     to verify the implementation. Generic checks from rules/quality.md always apply;
     this table adds what's specific to THIS feature. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-21 has implementation with file:line |
| Correctness | Set/delete/commit/discard all delegate to Editor methods (SetValue/DeleteValue/DeleteContainer/CommitSession/Discard), no reimplementation of editor logic |
| Correctness | Redirect after set/delete goes exactly one level up, not to root or same level. Commit/discard redirect to config root (`/config/edit/`) |
| Correctness | Validation errors from Editor.SetValue() are surfaced in notification area, not swallowed |
| Correctness | Commit conflict from CommitSession() re-renders commit page with error listing conflicting paths |
| Correctness | Commit page diff includes all paths with changes, not just current path |
| Naming | URL verb-first paths are lowercase: `/config/set/...`, `/config/delete/...`, `/config/commit`, `/config/discard` |
| Naming | CSS class for modified fields follows existing CSS naming convention |
| Data flow | Handler -> Editor -> Storage path, no direct zefs access from handler |
| Data flow | Draft isolation: user A's changes never visible to user B before commit |
| Rule: no-layering | No duplicate validation logic (Editor handles YANG type validation) |
| Correctness | Per-user mutex prevents data races on concurrent Editor access (verified by `TestEditorConcurrentAccess` with race detector) |
| Correctness | HTML checkbox converted to boolean string for TypeBool leaves before calling SetValue |
| Rule: integration-completeness | All three .ci functional tests exercise full HTTP -> Editor -> Storage path, including multi-user isolation |

### Deliverables Checklist (/implement stage 9)

<!-- MANDATORY: Every deliverable with a concrete verification method.
     /implement re-reads the spec and checks each item independently. -->
| Deliverable | Verification method |
|-------------|---------------------|
| `internal/component/web/editor.go` exists | `ls -la internal/component/web/editor.go` |
| `internal/component/web/templates/commit.html` exists | `ls -la internal/component/web/templates/commit.html` |
| `test/plugin/web-config-edit.ci` exists | `ls -la test/plugin/web-config-edit.ci` |
| `test/plugin/web-config-commit.ci` exists | `ls -la test/plugin/web-config-commit.ci` |
| POST `/config/set/...` dispatches to Editor.SetValue() | `TestSetVerbDispatch` passes |
| POST `/config/delete/...` dispatches to Editor.DeleteValue()/DeleteContainer() | `TestDeleteVerbDispatch` passes |
| Per-user draft isolation | `TestDraftPerUser` passes |
| EditSession has Origin "web" | `TestEditSessionOriginWeb` passes |
| Modified field has diff CSS class | `TestInlineDiffModifiedField` passes |
| Commit page shows grouped diff | `TestCommitPageDiff` passes |
| Validation error surfaced | `TestValidationError` passes |
| Concurrent Editor access is safe | `TestEditorConcurrentAccess` passes with race detector |
| Commit conflict re-renders with error | `TestCommitConflict` passes |
| Boolean checkbox conversion works | `TestBoolCheckboxConversion` passes |
| Draft persistence after restart | `TestDraftPersistenceOnRestart` passes |
| Session eviction on inactivity | `TestSessionEviction` passes |
| `test/plugin/web-config-multiuser.ci` exists | `ls -la test/plugin/web-config-multiuser.ci` |
| End-to-end set works | `web-config-edit.ci` passes |
| End-to-end commit works | `web-config-commit.ci` passes |
| Multi-user isolation works | `web-config-multiuser.ci` passes |
| `make ze-verify` passes | Run and capture output |

### Security Review Checklist (/implement stage 10)

<!-- MANDATORY: Feature-specific security concerns. /implement checks each item.
     Think about: untrusted input, injection, resource exhaustion, error leakage. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | Form body leaf names validated against YANG schema (reject arbitrary keys) |
| Input validation | Form body values validated via Editor.SetValue() YANG type checking (reject malformed values) |
| Input validation | URL path segments sanitized before schema walking (no path traversal) |
| Authentication | Every POST handler requires valid session cookie (or Basic Auth for JSON API). Auth middleware from web-1 (D-5) |
| Authorization | Username from authenticated session used to key drafts -- no user can access another user's draft |
| Injection | Template rendering uses html/template auto-escaping for all user-supplied values |
| Injection | data-old-value attribute content is HTML-escaped to prevent XSS via old config values |
| Resource exhaustion | Per-user Editor map bounded at configurable max (default 50) with 1-hour inactivity eviction |
| Error leakage | Validation errors show leaf name and constraint, not internal paths or stack traces |
| CSRF | Session cookie uses SameSite=Strict (D-5), which prevents cross-origin requests. Consider additional CSRF token if SameSite alone is insufficient for the threat model |

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

Not applicable -- this is a web UI feature, not protocol work.

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
- [ ] AC-1 through AC-21 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-web-3-config-edit.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.

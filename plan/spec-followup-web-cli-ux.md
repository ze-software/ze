# Spec: followup-web-cli-ux

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `git log -p plan/deferrals.md` (pre-2026-07-06) - original deferral rows + evidence

## Task

User-facing surface follow-ups across web, CLI completion, CLI stdio, and looking glass.

This is a consolidation skeleton created from verified deferral survivors (backlog triage 2026-07-06). Each item below was confirmed still-open against the codebase with a producing `file:line`. Split into phases when picked up; the sections after Task are lightweight scaffolding to be filled at design time.

### Work items (migrated from the 2026-07-06 deferral triage; `L#` = row in the pre-triage `plan/deferrals.md`)

- **web phase 2 (L77)** - RBAC (auth.go is basic-auth only), i18n, mobile layout, config upload/download, plugin web extensions.
- **shell-completion v3 (L78)** - flag-value completion (`--family <TAB>`) and config-section completion; today does peers/env-keys only.
- **cli-stdio-hardening (L218)** - shared error-capturing `renderWriter` for the project-wide `fmt.Printf`/`Fprintf`-to-stdout render paths that ignore write errors.
- **looking-glass v2 (L76)** - pagination/offset, large-RIB perf benchmark, Alice-LG e2e `.ci` (TLS already landed).

## Required Reading

### Source files / docs

- [ ] `internal/component/web/` (auth, assets)
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `internal/plugins/completion/`
  -> Constraint: verify current behaviour against this source before designing.
- [ ] `internal/component/lg/server.go`, `internal/component/lg/handler_api.go`
  -> Constraint: verify current behaviour against this source before designing.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; line numbers are pre-triage references)

- [ ] `internal/component/web/`
- [ ] `internal/plugins/completion/`
- [ ] `internal/component/lg/server.go`

**Behavior to preserve:**
- All existing behaviour of the listed files; this backlog work only adds the missing pieces named in the Task work items.

**Behavior to change:**
- Only the specific gaps enumerated in the Task work items.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Web HTTP requests; CLI tab-completion; CLI render paths; looking-glass API

### Transformation Path
1. A user hits a web route / presses TAB / runs a render command / queries the LG
2. The web/completion/cli/lg surface handles it
3. Response reflects RBAC / completion options / captured stdio errors / pagination

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| HTTP -> web handler | mux routing + auth | [ ] |
| CLI -> completion | YANG command tree / CompleteFn | [ ] |

### Integration Points
- `internal/component/web/`
- `internal/plugins/completion/`
- `internal/component/lg/`

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding - new commands/views/families/handlers register and are core-discovered, not hardcoded into a core/shared package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The verified `file:line` evidence in the Task items still holds at design time | 2026-07-06 backlog triage | Re-scope the item | grep/LSP at design time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Scope drift when the umbrella is split into per-item specs | Item needs its own design doc | Split into a dedicated spec and re-point |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Non-admin user hits an admin web route | → | RBAC denies | (fill during design) |
| `--family <TAB>` at the CLI | → | flag-value completion offers families | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | (define per work item when this skeleton moves to `design`) | (define at design time) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (define at design time) | (define at design time) | per Task work item | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| web-rbac, completion-flag, lg-paginate (new) (`.ci`) | test/web, test/ui, test/plugin | web/CLI/LG behaviour end-to-end | |

## Files to Modify

- `internal/component/web/` - see Task work items
- `internal/plugins/completion/` - see Task work items
- `internal/component/lg/server.go` - see Task work items

## Implementation Steps

1. **Phase: split** - if the umbrella covers unrelated items, split into per-item specs first.
2. **Phase: design** - for the chosen item, re-verify the `file:line` evidence and fill the Data Flow / Wiring / AC sections above.
3. **Phase: wiring** - register entry points, write the failing wiring test.
4. **Phase: implement (TDD)** - write test, fail, implement, pass, per work item.
5. **Full verification** - `make ze-verify`.
6. **Complete spec** - fill audit tables, write `plan/learned/NNN-<name>.md`, two-commit closure.

## Checklist

### Goal Gates (MUST pass)
- [ ] Every chosen work item has feature code + test
- [ ] Wiring Test table complete (concrete test names, none deferred)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Registration over hardcoding respected

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Notes
- Skeleton = captured intent, not a designed spec (see `ai/rules/deferral-tracking.md`). Moves to `design` when someone picks it up.

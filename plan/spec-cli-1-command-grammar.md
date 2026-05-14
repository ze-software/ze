# Spec: Unify CLI Command Grammar

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-05-14 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `plan/learned/395-yang-command-tree.md` -- YANG as single source of CLI hierarchy
4. `plan/learned/496-cli-dispatch.md` -- unified verb dispatch
5. `plan/learned/572-cmd-8-policy-show.md` -- show policy list/chain

## Task

Migrate all plugin-handled noun-verb CLI commands into the YANG `show` tree so every read command follows the `show <noun>` grammar. Currently, plugins using `OnExecuteCommand` string matching register commands as noun-verb (`static show`, `policy show`, `bmp sessions`, `rr status`) while the YANG show tree uses verb-noun (`show host`, `show policy list`). The overlap between `show policy list` (BGP filters) and `policy show` (PBR) is confusing. After this spec, all read commands use `show <noun>` exclusively.

## Required Reading

### Architecture Docs
- [ ] `plan/learned/395-yang-command-tree.md` -- YANG command tree
  -> Decision: YANG is the single source of CLI hierarchy. `ze:command "wire-method"` in `-cmd.yang` modules binds tree nodes to handlers.
  -> Constraint: new commands need a YANG module with `ze:command` extension and an `init()` handler registration via `pluginserver.RegisterRPCs`. No static help strings.
- [ ] `plan/learned/496-cli-dispatch.md` -- unified verb dispatch
  -> Decision: `ze show`, `ze set`, `ze del` are top-level verbs dispatched through the YANG command tree.
  -> Constraint: `HasCommandPrefix` checks both builtins and plugin registry.
- [ ] `plan/learned/572-cmd-8-policy-show.md` -- show policy list/chain
  -> Decision: `show policy list` and `show policy chain` registered in `cmd/show/show_policy.go` via `pluginserver.RegisterRPCs`.
  -> Constraint: handlers in `cmd/show/` package, YANG additions in `cmd/show/schema/ze-cli-show-cmd.yang`.
- [ ] `plan/learned/443-command-inventory.md` -- command inventory
  -> Constraint: `ze-validate-commands` cross-checks YANG `ze:command` entries against registered handlers. Must pass after migration.

**Key insights:**
- Two dispatch mechanisms exist: YANG show tree (`pluginserver.RegisterRPCs` + `ze:command` in YANG) and plugin `OnExecuteCommand` (string matching). The fix is to move all read commands to the YANG path.
- Plugin `OnExecuteCommand` is the wrong place for read commands. It was designed for plugin-internal commands dispatched via the engine, not for the CLI grammar.
- `ze-validate-commands` will catch any wiring gaps after migration.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/static/register.go:183-193` -- `OnExecuteCommand` matches `"static show"` string
  -> Constraint: handler calls `rm.showRoutes()` which is a method on the plugin's internal routeManager. Must be exported or called via DispatchCommand.
- [ ] `internal/plugins/policyroute/register.go:148-159` -- `OnExecuteCommand` matches `"policy show"` string
  -> Constraint: handler calls `formatPolicies(currentPolicies)` under a mutex. Data access needs synchronization.
- [ ] `internal/component/bgp/plugins/bmp/bmp.go:174-176,467-478` -- `OnExecuteCommand` matches `"bmp sessions"`, `"bmp peers"`, `"bmp collectors"`, `"bmp rib show"` strings
  -> Constraint: `bmp rib show` dispatches internally via `bp.plugin.DispatchCommand(ctx, "bgp rib show-protocol bmp")` -- it calls back into the engine, not local data.
- [ ] `internal/component/bgp/plugins/rr/register.go` -- `OnExecuteCommand` matches `"rr status"`, `"rr peers"` strings
- [ ] `internal/component/cmd/show/show_policy.go` -- `show policy list` and `show policy chain` via `pluginserver.RegisterRPCs`
  -> Decision: this is the target pattern. Handlers register at init() time, YANG tree provides CLI path.
- [ ] `internal/component/cmd/show/host.go` -- `show host <section>` via `pluginserver.RegisterRPCs`
  -> Decision: programmatic registration from map keys. Static/BMP/RR could follow same pattern.

**Behavior to preserve:**
- JSON output format of every command (callers parse this)
- Command semantics (what each command returns)
- `show policy list` and `show policy chain` keep their current names and behavior
- All existing `.ci` functional tests must pass

**Behavior to change:**
- `static show` becomes `show static` (routes is implied, only one subcommand)
- `policy show` (PBR) becomes `show policy-routes` (distinct from `show policy` which is BGP filters)
- `bmp sessions` becomes `show bmp sessions`
- `bmp peers` becomes `show bmp peers`
- `bmp collectors` becomes `show bmp collectors`
- `bmp rib show` becomes `show bmp rib`
- `rr status` becomes `show rr status`
- `rr peers` becomes `show rr peers`
- Old names remain as aliases during a deprecation period, logged with a warning

## Data Flow (MANDATORY)

### Entry Point
- User types a command at `ze cli` or `ze show <noun>` from the shell
- Dispatcher receives the command string

### Transformation Path
1. CLI parser splits command into verb + path
2. YANG command tree resolves path to `ze:command "wire-method"` node
3. `pluginserver.Dispatch` routes to registered handler by WireMethod
4. Handler executes and returns `(status, data)` JSON

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> Dispatcher | Command string via SSH or direct call | [ ] |
| Dispatcher -> Handler | WireMethod lookup from YANG tree | [ ] |
| Handler -> Plugin data | Handler calls plugin-internal function or DispatchCommand | [ ] |

### Integration Points
- `pluginserver.RegisterRPCs` -- where new handlers register
- `ze-cli-show-cmd.yang` -- where YANG tree entries are added
- `ze-validate-commands` -- build-time validation of YANG <-> handler wiring

### Architectural Verification
- [ ] No bypassed layers (commands go through YANG tree, not string matching)
- [ ] No unintended coupling (handlers in `cmd/show/` call into plugin packages)
- [ ] No duplicated functionality (replaces `OnExecuteCommand`, does not add parallel path)
- [ ] Zero-copy preserved where applicable (JSON marshaling unchanged)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze show static` | -> | `cmd/show/show_static.go` | `test/plugin/show-static.ci` |
| `ze show policy-routes` | -> | `cmd/show/show_policy_routes.go` | `test/plugin/show-policy-routes.ci` |
| `ze show bmp sessions` | -> | `cmd/show/show_bmp.go` | `test/plugin/show-bmp-sessions.ci` |
| `ze show bmp peers` | -> | `cmd/show/show_bmp.go` | `test/plugin/show-bmp-peers.ci` |
| `ze show bmp rib` | -> | `cmd/show/show_bmp.go` | `test/plugin/show-bmp-rib.ci` |
| `ze show rr status` | -> | `cmd/show/show_rr.go` | `test/plugin/show-rr-status.ci` |
| `ze show rr peers` | -> | `cmd/show/show_rr.go` | `test/plugin/show-rr-peers.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | User types `show static` in `ze cli` | Returns JSON array of configured static routes (same output as current `static show`) |
| AC-2 | User types `show policy-routes` in `ze cli` | Returns JSON with PBR policy routes (same output as current `policy show`) |
| AC-3 | User types `show bmp sessions` in `ze cli` | Returns JSON array of BMP receiver sessions (same output as current `bmp sessions`) |
| AC-4 | User types `show bmp peers` in `ze cli` | Returns JSON array of BMP monitored peers (same output as current `bmp peers`) |
| AC-5 | User types `show bmp rib` in `ze cli` | Returns JSON with BMP-monitored routes (same output as current `bmp rib show`) |
| AC-6 | User types `show rr status` in `ze cli` | Returns JSON with RR running state (same output as current `rr status`) |
| AC-7 | User types `show rr peers` in `ze cli` | Returns JSON array of RR peers (same output as current `rr peers`) |
| AC-8 | User types old command (`static show`, `policy show`, `bmp sessions`, etc.) | Command still works but logs a deprecation warning |
| AC-9 | `ze-validate-commands` build target | Passes with zero errors (all YANG <-> handler wiring valid) |
| AC-10 | `show policy list` | Unchanged: returns BGP filter types (no regression) |
| AC-11 | Tab completion at `show ` | Includes `static`, `bmp`, `rr`, `policy-routes` alongside existing `host`, `policy` |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestShowStatic` | `internal/component/cmd/show/show_static_test.go` | Handler returns expected JSON | |
| `TestShowPolicyRoutes` | `internal/component/cmd/show/show_policy_routes_test.go` | Handler returns expected JSON | |
| `TestShowBMPSessions` | `internal/component/cmd/show/show_bmp_test.go` | Handler returns expected JSON | |
| `TestShowRRStatus` | `internal/component/cmd/show/show_rr_test.go` | Handler returns expected JSON | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `show-static` | `test/plugin/show-static.ci` | CLI returns static route JSON | |
| `show-policy-routes` | `test/plugin/show-policy-routes.ci` | CLI returns PBR policy JSON | |
| `show-bmp-sessions` | `test/plugin/show-bmp-sessions.ci` | CLI returns BMP session list | |
| `show-bmp-rib` | `test/plugin/show-bmp-rib.ci` | CLI returns BMP monitored routes | |
| `show-rr-status` | `test/plugin/show-rr-status.ci` | CLI returns RR running state | |

## Files to Modify

- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` -- YANG additions for new show tree entries
- `internal/plugins/static/register.go` -- deprecation warning on `OnExecuteCommand`, export data access
- `internal/plugins/policyroute/register.go` -- deprecation warning on `OnExecuteCommand`, export data access
- `internal/component/bgp/plugins/bmp/bmp.go` -- deprecation warning on old command names
- `internal/component/bgp/plugins/rr/register.go` -- deprecation warning on old command names
- `docs/guide/command-reference.md` -- update command names
- `docs/guide/bmp.md` -- update CLI section
- `docs/guide/static-routes.md` -- update CLI section
- `docs/guide/policy-routing.md` -- update CLI section

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | Yes | `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` |
| CLI commands/flags | Yes | YANG-driven (automatic) |
| Editor autocomplete | Yes | YANG-driven (automatic) |
| Functional test for new RPC/API | Yes | `test/plugin/show-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/bmp.md`, `docs/guide/static-routes.md`, `docs/guide/policy-routing.md` |

## Files to Create
- `internal/component/cmd/show/show_static.go`
- `internal/component/cmd/show/show_policy_routes.go`
- `internal/component/cmd/show/show_bmp.go`
- `internal/component/cmd/show/show_rr.go`
- `test/plugin/show-static.ci`
- `test/plugin/show-policy-routes.ci`
- `test/plugin/show-bmp-sessions.ci`
- `test/plugin/show-bmp-rib.ci`
- `test/plugin/show-rr-status.ci`

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Phases 1-5 below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | lint + unit + functional tests |
| 6-9. Critical review | Critical Review Checklist |
| 10. Deliverables | Deliverables Checklist |
| 11. Security review | Security Review Checklist |

### Implementation Phases

1. **Phase: YANG tree entries** -- Add `show static`, `show policy-routes`, `show bmp`, `show rr` containers to `ze-cli-show-cmd.yang` with `ze:command` extensions pointing to new WireMethod strings.
   - Files: `ze-cli-show-cmd.yang`
   - Verify: `ze-validate-commands` reports unregistered handlers (expected)

2. **Phase: Show handlers** -- Create handler files in `cmd/show/` using the `pluginserver.RegisterRPCs` pattern. Each handler retrieves data from the plugin package.
   - Tests: unit tests for each handler
   - Files: `show_static.go`, `show_policy_routes.go`, `show_bmp.go`, `show_rr.go`
   - Verify: unit tests pass, `ze-validate-commands` passes

3. **Phase: Plugin data access** -- Export data-retrieval functions from each plugin so `cmd/show/` handlers can call them. For BMP `rib show`, use DispatchCommand to the engine (it already does this internally).
   - Files: `static/register.go`, `policyroute/register.go`, `bmp/bmp.go`, `rr/register.go`

4. **Phase: Deprecation aliases** -- Keep old `OnExecuteCommand` handlers but log a deprecation warning. Backwards compatibility during transition.
   - Files: all plugin `register.go` files
   - Verify: old commands still work, warning logged

5. **Phase: Functional tests + documentation** -- Create `.ci` tests, update docs and wiki.
   - Files: `test/plugin/show-*.ci`, docs, wiki

### Critical Review Checklist
| Check | What to verify |
|-------|---------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | JSON output identical to old command for each migrated command |
| Naming | New YANG paths follow `show/<noun>` consistently |
| Data flow | Handlers call plugin data functions, not engine dispatch (except BMP rib) |
| Rule: no-layering | Old `OnExecuteCommand` bodies deprecated, not duplicated |
| Rule: derive-not-hardcode | YANG tree drives help text and completion |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `show static` works | `ze show static` returns JSON |
| `show policy-routes` works | `ze show policy-routes` returns JSON |
| `show bmp sessions/peers/collectors/rib` works | `ze show bmp sessions` returns JSON |
| `show rr status/peers` works | `ze show rr status` returns JSON |
| Old commands deprecated with warning | `static show` logs deprecation |
| `ze-validate-commands` passes | Zero errors |
| Tab completion includes new entries | `ze show <tab>` shows new entries |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Show commands are read-only with no user parameters (BMP rib peer key validation exists). |
| Authorization | Show commands read-only, allowed by default authz profile. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Plugin data function not accessible | Phase 3: export or use DispatchCommand |
| `ze-validate-commands` fails | Phase 1: fix YANG <-> handler wiring |
| 3 fix attempts fail | STOP. Report approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights

## Implementation Summary

### What Was Implemented
### Bugs Found/Fixed
### Documentation Updates
### Deviations from Plan

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

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above

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
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] Tests pass
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Summary included in commit

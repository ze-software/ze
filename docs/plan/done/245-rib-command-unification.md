# Spec: rib-command-unification

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/rib_handler.go` - builtin RIB command handlers
4. `internal/plugin/command.go` - dispatcher and command routing
5. `internal/plugins/bgp-rib/rib.go` - RIB plugin implementation
6. `internal/plugins/bgp/types/reactor.go` - BGPReactor interface (RIB methods)

## Task

Unify the two parallel RIB command sets into a single consistent API. Currently there are overlapping commands with different names and different data sources:

| Builtin (rib_handler.go) | RIB Plugin (bgp-rib/rib.go) | Data Source |
|---|---|---|
| `rib show in` → reactor.RIBInRoutes() | `rib adjacent inbound show` → plugin's ribInPool | Different |
| `rib clear in` → reactor.ClearRIBIn() | `rib adjacent inbound empty` → plugin's ribInPool | Different |
| `rib show out` → STUB (not implemented) | `rib adjacent outbound show` → plugin's ribOut | Plugin only |
| `rib clear out` → STUB (not implemented) | `rib adjacent outbound resend` → plugin's ribOut | Plugin only |
| `rib help/command-list/command-complete` → Dispatcher | (none) | Engine only |
| `rib event list` → static list | (none) | Engine only |
| (none) | `rib adjacent status` → plugin stats | Plugin only |

**Problems:**
1. Two different command namespaces for the same concept (`rib show in` vs `rib adjacent inbound show`)
2. Two different data sources for inbound RIB (reactor's IncomingRIB vs plugin's ribInPool)
3. Builtin outbound commands are stubs that return errors
4. Users see both command sets in `rib command list` output, which is confusing
5. Meta-commands (help, command-list, command-complete) require Dispatcher access, which plugins don't have

**Goals:**
1. Single command namespace for all RIB operations
2. Single data source per operation (no parallel stores serving the same API)
3. Meta-commands stay in engine (they need Dispatcher)
4. Data commands are owned by the plugin (it has the real implementation)
5. Graceful fallback when RIB plugin is not loaded

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - overall system architecture
  → Constraint: Engine and plugin communicate via JSON over pipes; no direct function calls across process boundary
- [ ] `.claude/rules/plugin-design.md` - plugin registration and command dispatch
  → Decision: Commands route via Dispatcher: builtin first, then subsystem, then plugin registry
  → Constraint: Plugin commands registered via SDK `OnExecuteCommand` callback

### Source Files
- [ ] `internal/plugin/command.go` - Dispatcher, command routing, priority order
  → Decision: Longest-match prefix routing; builtins shadow plugin commands
  → Constraint: Builtin commands cannot be overridden by plugins (AddBuiltin marks them)
  → Decision: `RequireBGPReactor()` type-asserts `ReactorLifecycle` → `bgptypes.BGPReactor` for data handlers
- [ ] `internal/plugin/rib_handler.go` - current builtin RIB handlers
  → Constraint: Meta-commands use ctx.Dispatcher() to enumerate plugin commands
  → Constraint: Data handlers (show/clear in/out) use RequireBGPReactor(), not ctx.Reactor() directly
- [ ] `internal/plugins/bgp-rib/rib.go` - RIB plugin implementation
  → Decision: Plugin tracks both inbound (pool storage) and outbound (Route structs)
- [ ] `internal/plugins/bgp/types/reactor.go` - BGPReactor interface
  → Constraint: RIBInRoutes/ClearRIBIn/RIBStats are on BGPReactor (not ReactorLifecycle); removing them is a larger refactor
- [ ] `internal/plugin/mock_reactor_test.go` - mockReactor implements full ReactorInterface
  → Constraint: mockReactor has RIB stubs (RIBInRoutes, ClearRIBIn, RIBStats); may have dead code after removal

**Key insights:**
- Builtin commands have priority over plugin commands (AddBuiltin prevents shadowing)
- The RIB plugin already has richer data (pool storage for in, full outbound tracking)
- Meta-commands need Dispatcher which is engine-only infrastructure
- The reactor's RIB methods live on BGPReactor (not ReactorLifecycle) — accessed via type assertion
- Removing builtin data handlers also removes the only callers of RequireBGPReactor for RIB operations
- mockReactor RIB stubs (RIBInRoutes, ClearRIBIn, etc.) may become dead code but they satisfy the BGPReactor interface, so they stay

## Current Behavior (MANDATORY)

**Source files read (re-read after reactor-interface-split committed):**
- [ ] `internal/plugin/rib_handler.go` - 9 builtin RPC handlers for rib namespace
- [ ] `internal/plugin/command.go` - Dispatcher routes: builtin → subsystem → plugin
- [ ] `internal/plugins/bgp-rib/rib.go` - RIBManager with 5 execute-command handlers
- [ ] `internal/plugins/bgp/types/reactor.go` - BGPReactor has 6 RIB methods (3 deprecated)
- [ ] `internal/plugins/bgp/rib/incoming.go` - IncomingRIB: peer → index → Route
- [ ] `internal/plugins/bgp/rib/outgoing.go` - OutgoingRIB: pending/sent route queues
- [ ] `internal/plugin/mock_reactor_test.go` - mockReactor implements ReactorInterface (includes RIB methods)

**Post reactor-interface-split state:**
- `CommandContext.Reactor()` now returns `ReactorLifecycle` (narrow interface — no RIB methods)
- Builtin data handlers use `RequireBGPReactor(ctx)` which type-asserts to `bgptypes.BGPReactor`
- `RequireBGPReactor()` is at `command.go:152-171` — returns error if reactor doesn't implement BGPReactor
- `mockReactor` in tests implements full `ReactorInterface` (both `ReactorLifecycle` + `BGPReactor`)
- No existing unit tests for the RIB handler functions (grep found zero test references)

**Behavior to preserve:**
- Meta-commands (`rib help`, `rib command list`, `rib command complete`, `rib command help`) enumerate both builtin and plugin commands
- `rib event list` returns static event type list
- Plugin receives peer selector via execute-command RPC's peer parameter
- Plugin's outbound resend replays routes via `update-route` engine RPC
- Functional tests in `test/plugin/rib-reconnect*.ci` and `rib-withdrawal.ci` must keep passing

**Behavior to change:**
- Remove builtin data handlers (`rib show in`, `rib clear in`, `rib show out`, `rib clear out`)
- Plugin registers under short names matching the removed builtins
- Add fallback error messages when RIB plugin is not loaded

## Data Flow (MANDATORY)

### Entry Point — User Command
- User sends `rib show in` via API socket
- Dispatcher receives tokenized input

### Transformation Path (current — builtin)
1. Dispatcher matches builtin `rib show in` (longest prefix)
2. Handler calls `RequireBGPReactor(ctx)` to type-assert `ReactorLifecycle` → `bgptypes.BGPReactor`
3. Calls `r.RIBInRoutes(peerID)` on the BGPReactor
4. Reactor adapter reads from `IncomingRIB` (parsed Route objects)
5. Returns JSON response via socket

### Transformation Path (current — plugin)
1. Dispatcher fails to match builtin (different name: `rib adjacent inbound show`)
2. Falls through to plugin registry
3. `dispatchPlugin()` finds match, calls `routeToProcess()`
4. `SendExecuteCommand()` RPC to plugin over Socket B
5. Plugin's `handleCommand()` reads from `ribInPool` (pool storage)
6. Returns JSON via RPC response

### Transformation Path (proposed)
1. Dispatcher finds no builtin match for `rib show in` (removed)
2. Falls through to plugin registry
3. Plugin registered `rib show in` → `routeToProcess()` → RPC
4. Plugin's `handleCommand()` returns data from pool storage
5. If RIB plugin not loaded → `ErrUnknownCommand` → engine returns error

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | execute-command RPC (JSON over Socket B) | [ ] |
| Dispatcher → Plugin | routeToProcess() via CommandRegistry | [ ] |

### Integration Points
- `Dispatcher.Register()` — remove builtin data handler registrations
- `CommandRegistry.AddBuiltin()` — stop marking data commands as builtin (allows plugin registration)
- `sdk.Registration.Commands` — plugin declares new command names
- `RIBManager.handleCommand()` — plugin handles new command names

### Architectural Verification
- [ ] No bypassed layers (commands flow through Dispatcher → plugin RPC)
- [ ] No unintended coupling (engine no longer calls reactor RIB methods for API)
- [ ] No duplicated functionality (one data source per command)
- [ ] Zero-copy preserved where applicable (pool storage unchanged)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `rib show in` with RIB plugin loaded | Routes returned from plugin's pool storage (same as current `rib adjacent inbound show`) |
| AC-2 | `rib clear in` with RIB plugin loaded | Routes cleared in plugin's pool storage |
| AC-3 | `rib show out` with RIB plugin loaded | Routes returned from plugin's ribOut |
| AC-4 | `rib clear out` with RIB plugin loaded | Routes resent from plugin's ribOut (same as current `rib adjacent outbound resend`) |
| AC-5 | `rib show in` without RIB plugin loaded | Error response: "rib plugin not loaded" or similar |
| AC-6 | `rib help` | Lists all rib subcommands (both meta and data) |
| AC-7 | `rib command list` | Lists all rib commands with sources (builtin for meta, plugin for data) |
| AC-8 | `rib status` with RIB plugin loaded | Returns plugin status (peers, routes_in, routes_out) |
| AC-9 | `rib adjacent inbound show` | Still works (backward compat alias or identical command) |
| AC-10 | Existing functional tests pass | `test/plugin/rib-reconnect*.ci`, `rib-withdrawal.ci` |

## Design Decision: Command Naming

Two options for the unified namespace:

**Option A — Short names (recommended):**
Remove builtins, plugin registers short names matching current builtin style.

| Old Builtin | Old Plugin | New (Plugin) |
|---|---|---|
| `rib show in` | `rib adjacent inbound show` | `rib show in` |
| `rib clear in` | `rib adjacent inbound empty` | `rib clear in` |
| `rib show out` | `rib adjacent outbound show` | `rib show out` |
| `rib clear out` | `rib adjacent outbound resend` | `rib clear out` |
| (none) | `rib adjacent status` | `rib status` |

Pros: Familiar short names, matches existing user-facing API style.
Cons: Loses RFC-standard "adjacent" terminology.

**Option B — RFC-style names:**
Remove builtins, keep plugin's `rib adjacent ...` names as-is.

Pros: RFC 4271 terminology (Adj-RIB-In, Adj-RIB-Out).
Cons: More typing, inconsistent with other command styles.

→ **Recommend Option A** unless user prefers otherwise.

## Design Decision: Reactor RIB Methods

The `BGPReactor` interface (in `internal/plugins/bgp/types/reactor.go`) has 6 RIB methods:
- `RIBInRoutes(peerID)`, `RIBStats()`, `ClearRIBIn()` — used by builtin handlers being removed
- `RIBOutRoutes()`, `ClearRIBOut()`, `FlushRIBOut()` — all deprecated, always return zero/nil

After the reactor-interface-split, these live on `bgptypes.BGPReactor` (not `ReactorLifecycle`). The builtin handlers access them via `RequireBGPReactor()` type assertion. Once the builtin data handlers are removed, the only production callers of these 6 RIB methods are gone.

**Options:**
- **Keep methods on interface** — they satisfy the interface contract; mockReactor stubs stay; future internal use possible
- **Remove methods** — larger refactor, separate spec (also needs removing 3 deprecated methods)

→ **Recommend: Keep methods, remove only the builtin handlers.** The interface cleanup is a separate concern.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRibRPCsMetaOnly` | `internal/plugin/rib_handler_test.go` | ribRPCs() returns only meta-commands (no data handlers) | |
| `TestRibHelpIncludesPlugin` | `internal/plugin/rib_handler_test.go` | rib help lists both meta and plugin data commands | |
| `TestRibCommandListShowsPlugin` | `internal/plugin/rib_handler_test.go` | rib command list includes plugin-registered data commands | |
| `TestRIBPluginHandleCommandShortNames` | `internal/plugins/bgp-rib/rib_test.go` | handleCommand dispatches short names (rib show in, etc.) | |
| `TestRIBPluginHandleCommandLegacyNames` | `internal/plugins/bgp-rib/rib_test.go` | handleCommand still dispatches old names (rib adjacent ...) | |

### Boundary Tests
N/A — no new numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing `rib-reconnect-simple` | `test/plugin/rib-reconnect-simple.ci` | Reconnect replays routes | |
| existing `rib-reconnect` | `test/plugin/rib-reconnect.ci` | Full reconnect scenario | |
| existing `rib-withdrawal` | `test/plugin/rib-withdrawal.ci` | Withdrawal handling | |

## Files to Modify
- `internal/plugin/rib_handler.go` — Remove data handlers (handleRIBShowIn/Out, handleRIBClearIn/Out) and their ribRPCs() entries; keep 5 meta handlers (help, command-list, command-help, command-complete, event-list)
- `internal/plugins/bgp-rib/rib.go` — Add short-name command dispatch in handleCommand(); register short names in sdk.Registration.Commands; keep old names as aliases
- `internal/plugin/rpc_registration_test.go` — Three updates needed:
  1. Total count: 32 → 28 (removing 4 data handlers)
  2. Per-module count: RIB 9 → 5
  3. Expected methods list: remove `ze-rib:show-in` and `ze-rib:clear-in`
  4. Dispatcher test: remove `ze-rib:show-in` assertion

Note: `command.go` does NOT need modification — `RibPluginRPCs()` just delegates to `ribRPCs()` in rib_handler.go.
Note: `mock_reactor_test.go` keeps its RIB stubs — they satisfy the BGPReactor interface even though the builtin callers are gone.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] No — commands unchanged at RPC level | |
| RPC count in architecture docs | [ ] No — net zero (removing builtins, adding plugin) | |
| CLI commands/flags | [ ] No — API commands only | |
| CLI usage/help text | [ ] No | |
| API commands doc | [x] Yes — update command list if documented | `docs/architecture/api/commands.md` |
| Plugin SDK docs | [ ] No — existing SDK mechanism | |
| Editor autocomplete | [ ] No — YANG-driven | |
| Functional test for new RPC/API | [ ] No — existing functional tests cover behavior | |

## Files to Create
None — all changes are modifications to existing files.

## Implementation Steps

Each step ends with a **Self-Critical Review**.

1. **Write unit tests** — Tests for removed builtin handlers and new plugin command names
   → **Review:** Do tests cover both old (rib adjacent) and new (rib show in) names?

2. **Run tests** — Verify FAIL (paste output)
   → **Review:** Tests fail for the RIGHT reason?

3. **Remove builtin data handlers** — Delete handleRIBShowIn/Out, handleRIBClearIn/Out from rib_handler.go; remove from ribRPCs()
   → **Review:** Meta handlers still intact? No orphaned code?

4. **Update plugin command registration** — Add short names to sdk.Registration.Commands; update handleCommand() dispatch
   → **Review:** Both old and new names work? No naming collision with remaining builtins?

5. **Run tests** — Verify PASS (paste output)
   → **Review:** All tests pass? No regressions?

6. **Run full verification** — `make lint && make test && make functional`
   → **Review:** Zero lint issues? All functional tests pass?

7. **Final self-review** — Check all changes, verify no unused code or debug statements

### Failure Routing

| Failure | Symptom | Route To |
|---------|---------|----------|
| Plugin commands shadowed by builtin | Command still hits old builtin handler | Step 3 — verify AddBuiltin removed for data commands |
| Plugin not found for short names | "unknown command" error | Step 4 — check registration |
| Functional test regression | rib-reconnect fails | Check if test uses old command names |

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

## Implementation Summary

### What Was Implemented
- Removed 4 builtin data handlers (handleRIBShowIn, handleRIBClearIn, handleRIBShowOut, handleRIBClearOut) from rib_handler.go
- Reduced ribRPCs() from 9 to 5 entries (meta-commands only)
- Added 5 short-name commands to RIB plugin's handleCommand() switch (rib status, rib show in, rib clear in, rib show out, rib clear out)
- Registered 10 total commands in sdk.Registration.Commands (5 short + 5 long names)
- Updated rpc_registration_test.go counts (32→28 total, 9→5 RIB)
- Updated TestCommandTree in cmd/ze/cli to check meta-commands instead of removed data commands
- Created rib_handler_test.go with TestRibRPCsMetaOnly

### Bugs Found/Fixed
- TestCommandTree in cmd/ze/cli/main_test.go validated the command tree included `rib show in/out` as builtins. Updated to validate meta-commands (help, command, event) instead.

### Design Insights
- The Dispatcher's builtin→subsystem→plugin priority chain naturally handles the unification: removing builtins makes commands fall through to plugin registry without any Dispatcher changes.

### Documentation Updates
- None — no architectural changes

### Deviations from Plan
- Added cmd/ze/cli/main_test.go to modified files (not in original plan — discovered during testing)
- ~~TestRibHelpIncludesPlugin and TestRibCommandListShowsPlugin not written~~ — implemented using lightweight mock (Process + CommandDef registration in CommandRegistry)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Remove builtin data handlers | ✅ Done | `rib_handler.go:8-18` | 4 handlers + ribRPCs entries removed |
| Plugin registers short command names | ✅ Done | `rib.go:138-152` | 5 short + 5 long names in Registration |
| Meta-commands stay in engine | ✅ Done | `rib_handler.go:8-18` | help, command-list/help/complete, event-list stay |
| Graceful fallback when plugin not loaded | ✅ Done | Dispatcher returns ErrUnknownCommand | Natural fallback via dispatchPlugin() |
| Old plugin command names still work | ✅ Done | `rib.go:776-790` | handleCommand switch has both short and long |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `TestRIBPluginHandleCommandShortNames/rib_show_in` | Routes from plugin pool |
| AC-2 | ✅ Done | `TestRIBPluginHandleCommandShortNames/rib_clear_in` | Clears plugin pool |
| AC-3 | ✅ Done | `TestRIBPluginHandleCommandShortNames/rib_show_out` | Routes from ribOut |
| AC-4 | ✅ Done | `TestRIBPluginHandleCommandShortNames/rib_clear_out` | Resends from ribOut |
| AC-5 | ✅ Done | Dispatcher returns ErrUnknownCommand | No builtin, no plugin → error |
| AC-6 | ✅ Done | handleRibHelp hardcodes command/event + discovers plugin subcommands | `TestRibHelpIncludesPlugin` |
| AC-7 | ✅ Done | handleRibCommandList enumerates builtin + plugin | `TestRibCommandListShowsPlugin` |
| AC-8 | ✅ Done | `TestRIBPluginHandleCommandShortNames/rib_status` | Returns running/peers/routes |
| AC-9 | ✅ Done | `TestRIBPluginHandleCommandLegacyNames` | All 5 old names pass |
| AC-10 | ✅ Done | `make functional` 96/96 passed | rib-reconnect*, rib-withdrawal pass |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestRibRPCsMetaOnly | ✅ Done | `rib_handler_test.go:14` | Verifies 5 meta RPCs only |
| TestRibHelpIncludesPlugin | ✅ Done | `rib_handler_test.go:47` | Verifies plugin subcommands appear in help |
| TestRibCommandListShowsPlugin | ✅ Done | `rib_handler_test.go:100` | Verifies builtins + plugin commands listed |
| TestRIBPluginHandleCommandShortNames | ✅ Done | `rib_test.go:807` | All 5 short names pass |
| TestRIBPluginHandleCommandLegacyNames | ✅ Done | `rib_test.go:853` | All 5 old names pass |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| internal/plugin/rib_handler.go | ✅ Modified | Removed 4 data handlers, ribRPCs 9→5 |
| internal/plugins/bgp-rib/rib.go | ✅ Modified | Short names in handleCommand + Registration |
| internal/plugin/rpc_registration_test.go | ✅ Modified | Counts 32→28, 9→5, updated expected methods |
| internal/plugin/rib_handler_test.go | ✅ Created | TestRibRPCsMetaOnly, TestRibHelpIncludesPlugin, TestRibCommandListShowsPlugin |
| cmd/ze/cli/main_test.go | ✅ Modified | TestCommandTree checks meta-commands |

### Audit Summary
- **Total items:** 20
- **Done:** 20
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (added cmd/ze/cli/main_test.go — not in original plan)

## Checklist

### Goal Gates (MUST pass - cannot defer)
- [x] Acceptance criteria AC-1..AC-10 all demonstrated
- [x] Tests pass (`make test`)
- [x] No regressions (`make functional` — 96/96 passed)
- [x] Feature code integrated into codebase (`internal/*`)

### Quality Gates (SHOULD pass - can defer with explicit user approval)
- [x] `make lint` passes
- [x] Architecture docs updated with learnings (none needed)
- [x] Implementation Audit fully completed
- [x] Mistake Log escalation candidates reviewed (none)

### 🏗️ Design
- [x] No premature abstraction
- [x] No speculative features
- [x] Single responsibility
- [x] Explicit behavior
- [x] Minimal coupling
- [x] Next-developer test

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (verified: ribRPCs returned 9, short names returned unknown command)
- [x] Implementation complete
- [x] Tests PASS (verified: all new and existing tests pass)
- [x] Functional tests verify end-to-end behavior (96/96)

### Documentation
- [x] Required docs read
- [x] RFC summaries read (not applicable — no protocol changes)

### Completion
- [ ] All Partial/Skipped items have user approval
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together

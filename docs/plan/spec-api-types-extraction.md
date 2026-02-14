# Spec: api-types-extraction

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/types.go` - where BGP data types currently live
4. `internal/plugins/bgp/handler/register.go` - existing RPCProviders pattern
5. `internal/plugin/command.go` - BgpPluginRPCs() builtin aggregation

## Task

Extract BGP-specific data types (`RawMessage`, `ContentConfig`) from `internal/plugin/types.go` to `internal/plugins/bgp/types/`, and migrate remaining BGP builtin RPCs (update, watchdog) from hardcoded builtins to RPCProviders injection. This removes backward BGP imports from the plugin framework and enables moving BGP implementation files out of `internal/plugin/`.

### Why

`internal/plugin/types.go` imports 4 BGP-specific packages (`attribute`, `filter`, `message`, `wireu`) solely for `RawMessage` and `ContentConfig` field types. This backward dependency (generic framework → BGP specifics) prevents moving BGP implementation files to `internal/plugins/bgp/` without creating import cycles.

Additionally, `update_text.go` (2300 LOC), `update_wire.go` (396 LOC), and `route_watchdog.go` are BGP command implementations stuck in `internal/plugin/` because they're registered as "builtins" via `BgpPluginRPCs()`. The RPCProviders injection pattern (already used by `handler/`) can replace this.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - API command dispatch and RPC registration
  → Constraint: RPCs must be discoverable via YANG wire methods AND CLI text commands
- [ ] `docs/architecture/core-design.md` - Engine/plugin boundary
  → Decision: Plugin framework is protocol-agnostic; BGP-specific code belongs in `internal/plugins/bgp/`

### Source Files (MUST read before implementing)
- [ ] `internal/plugin/types.go` - Current location of RawMessage, ContentConfig, PeerInfo
  → Constraint: types.go imports 5 BGP packages; after extraction should import only bgptypes
- [ ] `internal/plugin/command.go` - BgpPluginRPCs() aggregation
  → Decision: subscribeRPCs() stays (generic); updateRPCs(), routeRPCs() move to injection
- [ ] `internal/plugins/bgp/handler/register.go` - RPCProviders injection pattern
  → Pattern: BgpHandlerRPCs() aggregates all handler RPCs, injected via ServerConfig
- [ ] `internal/plugins/bgp/reactor/reactor.go:4268-4275` - Where RPCProviders are wired
  → Pattern: reactor creates ServerConfig with RPCProviders slice
- [ ] `internal/plugin/update_text.go` - Biggest file to move (2300 LOC)
- [ ] `internal/plugin/update_wire.go` - Wire-format update parsing (396 LOC)
- [ ] `internal/plugin/route_watchdog.go` - Watchdog RPC handlers

**Key insights:**
- RPCProviders injection pattern is proven (handler/ already uses it)
- RawMessage and ContentConfig are the ONLY types creating backward BGP imports in types.go
- PeerInfo has NO BGP imports (just netip, time) — stays in plugin/ (used by ReactorLifecycle interface)
- subscribe.go is generic event infrastructure — stays in plugin/
- server_bgp.go and validate_open.go are Server methods — stay in plugin/ by design

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/types.go` - Defines RawMessage, ContentConfig, PeerInfo, Response, ReactorLifecycle, etc.
  - Imports: attribute, filter, message, bgptypes, wireu (5 BGP packages)
  - After extracting RawMessage + ContentConfig: only bgptypes remains (1 BGP package)
- [ ] `internal/plugin/command.go` - BgpPluginRPCs() calls subscribeRPCs() + routeRPCs() + updateRPCs()
- [ ] `internal/plugin/server.go` - RPCProviders loop at lines 236-243 registers injected RPCs

**Behavior to preserve:**
- All RPC wire methods and CLI commands must remain reachable after migration
- RPC registration test counts must be updated (builtin count decreases, provider count increases)
- Subscribe/unsubscribe RPCs stay as builtins (generic, not BGP-specific)
- System, RIB, session, plugin lifecycle RPCs unchanged

**Behavior to change:**
- `BgpPluginRPCs()` will return only `subscribeRPCs()` (remove updateRPCs, routeRPCs)
- update/watchdog RPCs registered via RPCProviders injection instead of builtins
- RawMessage and ContentConfig accessed via `bgptypes.RawMessage`, `bgptypes.ContentConfig`

## Data Flow (MANDATORY)

### Entry Point
- RPC commands enter via CLI text dispatch or YANG wire method dispatch
- Both paths converge in handler functions with `Handler` signature

### Transformation Path
1. Server receives command (text via plugin protocol, or JSON-RPC via socket)
2. Dispatcher matches command to handler (builtin → subsystem → plugin)
3. Handler executes using CommandContext (accesses reactor, commit manager, etc.)
4. Handler returns Response

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Server → Dispatcher | Text command string or YANG method | [ ] |
| Dispatcher → Handler | Handler function call with CommandContext | [ ] |
| Handler → Reactor | Via ctx.Reactor() or RequireBGPReactor() | [ ] |

### Integration Points
- `ServerConfig.RPCProviders` - where new RPC sources are injected
- `handler.BgpHandlerRPCs()` - existing aggregation function to extend
- `reactor.go:4272` - where RPCProviders slice is constructed

### Architectural Verification
- [ ] No bypassed layers (RPCs still go through dispatcher)
- [ ] No unintended coupling (handler/ imports plugin/ for types — established pattern)
- [ ] No duplicated functionality (migrating, not duplicating)
- [ ] Zero-copy preserved where applicable (RawMessage fields unchanged)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `internal/plugin/types.go` imports | Only `bgptypes` remains as BGP import (4 removed) |
| AC-2 | `make test` after Phase 1 | All tests pass with bgptypes.RawMessage, bgptypes.ContentConfig |
| AC-3 | `bgp peer update text ...` command | Still dispatched and executed correctly after RPC migration |
| AC-4 | `bgp watchdog announce/withdraw` | Still dispatched correctly after RPC migration |
| AC-5 | `make functional` | All 243+ functional tests pass |
| AC-6 | RPC registration test | Builtin count decreases, total count unchanged |
| AC-7 | update_text.go no longer in `internal/plugin/` | File moved to `internal/plugins/bgp/handler/` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestAllBuiltinRPCCount | `rpc_registration_test.go` | Builtin count decreases after migration | |
| TestBgpHandlerRPCCount | `handler/dispatch_test.go` | Handler provider count increases | |
| TestRawMessageInBgpTypes | `types/rawmessage_test.go` | RawMessage accessible from bgptypes | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A — no new numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing update tests | `test/encode/*.ci` | Update commands still work after migration | |
| Existing plugin tests | `test/plugin/*.ci` | Plugin commands still work | |

### Future
- Move text.go and json.go to `internal/plugins/bgp/format/` (separate spec, lower priority)

## Files to Modify

### Phase 1: Move BGP data types
- `internal/plugin/types.go` - Remove RawMessage, ContentConfig; remove 4 BGP imports
- `internal/plugins/bgp/types/rawmessage.go` - New file for RawMessage
- `internal/plugins/bgp/types/contentconfig.go` - New file for ContentConfig
- `internal/plugin/text.go` - Change RawMessage/ContentConfig to bgptypes.*
- `internal/plugin/json.go` - Change RawMessage to bgptypes.RawMessage (if referenced)
- `internal/plugin/server_bgp.go` - Change RawMessage/ContentConfig to bgptypes.*
- `internal/plugins/bgp/reactor/reactor.go` - Change plugin.RawMessage to bgptypes.RawMessage

### Phase 2: Migrate BGP builtin RPCs to injection
- `internal/plugin/command.go` - Remove updateRPCs(), routeRPCs() from BgpPluginRPCs()
- `internal/plugin/update_text.go` → `internal/plugins/bgp/handler/update_text.go`
- `internal/plugin/update_wire.go` → `internal/plugins/bgp/handler/update_wire.go`
- `internal/plugin/route_watchdog.go` → `internal/plugins/bgp/handler/route_watchdog.go`
- `internal/plugins/bgp/handler/register.go` - Add UpdateRPCs(), WatchdogRPCs() to BgpHandlerRPCs()
- `internal/plugin/rpc_registration_test.go` - Update builtin count

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | No new RPCs, just moving existing |
| RPC count in architecture docs | No | Total count unchanged |
| CLI commands/flags | No | Commands unchanged |
| CLI usage/help text | No | No changes |
| API commands doc | No | No changes |
| Plugin SDK docs | No | No changes |
| Editor autocomplete | No | No changes |
| Functional test for new RPC/API | No | Existing tests cover moved RPCs |

## Files to Create
- `internal/plugins/bgp/types/rawmessage.go` - RawMessage type (moved from types.go)
- `internal/plugins/bgp/types/contentconfig.go` - ContentConfig type (moved from types.go)

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

### Phase 1: Move BGP data types

1. **Move RawMessage** to `internal/plugins/bgp/types/rawmessage.go`
   - Move type definition and IsAsyncSafe() method
   - Update all references: plugin.RawMessage → bgptypes.RawMessage
   → **Review:** Did all references update? Any new import cycles?

2. **Move ContentConfig** to `internal/plugins/bgp/types/contentconfig.go`
   - Move type definition and WithDefaults() method
   - Update all references: plugin.ContentConfig → bgptypes.ContentConfig
   → **Review:** Did all references update? Any missing imports?

3. **Clean up types.go imports** - Remove attribute, filter, message, wireu imports
   → **Review:** Only bgptypes import remains? No unused imports?

4. **Run tests** - `make test` (paste output)
   → **Review:** All pass? No regressions?

### Phase 2: Migrate BGP builtin RPCs

5. **Move update_text.go** to `internal/plugins/bgp/handler/update_text.go`
   - Change package to `handler`
   - Update internal references (CommandContext → plugin.CommandContext, etc.)
   - Move updateRPCs() → UpdateRPCs() (exported)
   → **Review:** No import cycles? All references resolved?

6. **Move update_wire.go** to `internal/plugins/bgp/handler/update_wire.go`
   - Same pattern as step 5
   → **Review:** Clean build?

7. **Move route_watchdog.go** to `internal/plugins/bgp/handler/route_watchdog.go`
   - Change routeRPCs() → WatchdogRPCs() (exported)
   → **Review:** Clean build?

8. **Update BgpPluginRPCs()** in command.go
   - Remove updateRPCs() and routeRPCs() calls
   - Only subscribeRPCs() remains
   → **Review:** subscribeRPCs still works? No missing handlers?

9. **Update BgpHandlerRPCs()** in handler/register.go
   - Add UpdateRPCs() and WatchdogRPCs() to aggregation
   → **Review:** All RPCs now discoverable?

10. **Update RPC registration tests** - Adjust builtin/handler counts
    → **Review:** Counts match reality?

11. **Run full verification** - `make lint && make test && make functional` (paste output)
    → **Review:** Zero issues?

### Failure Routing

| Failure | Symptom | Route To |
|---------|---------|----------|
| Import cycle | `import cycle not allowed` | Verify no backward imports; check which type creates the cycle |
| Missing type | `undefined: RawMessage` | Update import to use bgptypes |
| Test count mismatch | RPC count assertion fails | Update expected counts in test |
| Functional test fails | Command not found | Verify RPCProviders registration includes moved RPCs |

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

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Move RawMessage to bgptypes | | | |
| Move ContentConfig to bgptypes | | | |
| Remove 4 backward BGP imports from types.go | | | |
| Migrate updateRPCs to RPCProviders injection | | | |
| Migrate routeRPCs to RPCProviders injection | | | |
| Move update_text.go to handler/ | | | |
| Move update_wire.go to handler/ | | | |
| Move route_watchdog.go to handler/ | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | | | |
| AC-2 | | | |
| AC-3 | | | |
| AC-4 | | | |
| AC-5 | | | |
| AC-6 | | | |
| AC-7 | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestAllBuiltinRPCCount | | | |
| TestBgpHandlerRPCCount | | | |
| TestRawMessageInBgpTypes | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugin/types.go` | | |
| `internal/plugins/bgp/types/rawmessage.go` | | |
| `internal/plugins/bgp/types/contentconfig.go` | | |
| `internal/plugin/command.go` | | |
| `internal/plugins/bgp/handler/register.go` | | |
| `internal/plugins/bgp/handler/update_text.go` | | |
| `internal/plugins/bgp/handler/update_wire.go` | | |
| `internal/plugins/bgp/handler/route_watchdog.go` | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Goal Gates (MUST pass — cannot defer)
- [ ] Acceptance criteria AC-1..AC-7 all demonstrated
- [ ] Tests pass (`make test`)
- [ ] No regressions (`make functional`)
- [ ] Feature code integrated into codebase (`internal/*`)

### Quality Gates (SHOULD pass — can defer with explicit user approval)
- [ ] `make lint` passes
- [ ] Architecture docs updated with learnings
- [ ] Implementation Audit fully completed
- [ ] Mistake Log escalation candidates reviewed

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Functional tests verify end-to-end behavior

### Documentation (during implementation)
- [ ] Required docs read
- [ ] RFC references added to code (N/A — no protocol changes)

### Completion (after tests pass)
- [ ] All Partial/Skipped items have user approval
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together

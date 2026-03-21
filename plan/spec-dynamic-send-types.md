# Spec: dynamic-send-types

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-03-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/plugin/events.go` - dynamic event type registration pattern (model for send types)
4. `internal/component/bgp/reactor/config.go` - parseOneSendFlag with hardcoded enhanced-refresh
5. `internal/component/plugin/server/startup_autoload.go` - auto-load pattern

## Task

Two related concerns:

**1. Dynamic send type registration.** Send types are currently hardcoded in `parseOneSendFlag` (update, refresh, enhanced-refresh). This breaks the plugin boundary -- the engine knows about plugin-specific send capabilities. Apply the same pattern as `EventTypes` for receive: plugins declare `SendTypes` in Registration, engine validates against the dynamic registry at config parse time.

**2. Enhanced Route Refresh plugin.** RFC 7313 Enhanced Route Refresh (BORR/EORR) is a protocol concern separate from the basic RIB, similar to how Graceful Restart has its own `bgp-gr` plugin. The `enhanced-refresh` send type should be registered by a dedicated plugin (e.g., `bgp-enhanced-refresh`) that handles the BORR/EORR lifecycle.

**Current state (stopgap):** `SendEnhancedRefresh bool` is hardcoded on ProcessBinding. `parseOneSendFlag` has a hardcoded `case "enhanced-refresh"`. This works but violates the plugin boundary principle.

**Target state:** Engine has no knowledge of `enhanced-refresh`. A plugin registers it via `Registration.SendTypes: []string{"enhanced-refresh"}`. Config `send [ enhanced-refresh ]` is validated against the dynamic registry. The plugin handles BORR/EORR protocol logic.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - plugin startup, auto-loading
  -> Constraint: Three-phase startup, auto-load for families and event types
  -> Decision: Same pattern extends to send types
- [ ] `docs/architecture/core-design.md` - plugin boundary, dynamic registration
  -> Constraint: Engine stays content-agnostic
- [ ] `.claude/rules/plugin-design.md` - Registration fields, proximity principle
  -> Decision: SendTypes field parallel to EventTypes

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7313.md` - Enhanced Route Refresh
  -> Constraint: BORR/EORR are always paired, per-family

**Key insights:**
- EventTypes pattern (receive side) is the model: Registration field, RegisterPluginSendTypes at startup, ValidSendTypes map, runtime validation in parseOneSendFlag
- Enhanced Route Refresh is protocol-level (BORR/EORR markers on wire), not just a permission flag
- GR plugin (`bgp-gr`) is the precedent for extracting protocol concerns into plugins

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/reactor/config.go` - parseOneSendFlag: hardcoded update, refresh, enhanced-refresh
- [ ] `internal/component/bgp/reactor/peersettings.go` - SendEnhancedRefresh bool on ProcessBinding
- [ ] `internal/component/plugin/types.go` - SendEnhancedRefresh bool on PeerProcessBinding
- [ ] `internal/component/plugin/events.go` - EventTypes dynamic registration pattern (model)
- [ ] `internal/component/plugin/resolve.go` - RegisterPluginEventTypes (model)
- [ ] `internal/component/bgp/plugins/rpki_decorator/register.go` - EventTypes usage example
- [ ] `rfc/short/rfc7313.md` - Enhanced Route Refresh protocol
- [ ] `internal/component/bgp/plugins/route_refresh/` - existing route refresh handler

**Behavior to preserve:**
- `send [ update ]` and `send [ refresh ]` continue to work unchanged
- `send [ enhanced-refresh ]` continues to be accepted in config
- Existing refresh.ci and rib-reconnect.ci tests pass

**Behavior to change:**
- `enhanced-refresh` validated dynamically (not hardcoded case)
- `SendEnhancedRefresh` bool removed from ProcessBinding (use SendCustom map)
- New `SendTypes` field on Registration
- New `RegisterPluginSendTypes` function (parallel to RegisterPluginEventTypes)
- New plugin registers `enhanced-refresh` and handles BORR/EORR lifecycle

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin registers SendTypes: ["enhanced-refresh"] | `enhanced-refresh` accepted in `send [ ]` config |
| AC-2 | No plugin registers "enhanced-refresh" | `send [ enhanced-refresh ]` rejected as unknown |
| AC-3 | Config has `send [ enhanced-refresh ]` but plugin not configured | Plugin auto-loaded (Phase 3 pattern) |
| AC-4 | Engine config.go parseOneSendFlag | No hardcoded `enhanced-refresh` case |
| AC-5 | ProcessBinding / PeerProcessBinding | No `SendEnhancedRefresh` bool -- uses `SendCustom map[string]bool` |

## Design Decisions

### D1: SendTypes parallel to EventTypes
Same registration pattern. Plugins declare what send types they enable. Engine validates dynamically.

### D2: Enhanced Route Refresh is a plugin
Like `bgp-gr` for Graceful Restart. Owns BORR/EORR lifecycle, registers the send type, handles the protocol.

### D3: SendCustom map for plugin-registered send types
Parallel to ReceiveCustom. Base types (update, refresh) keep dedicated bool fields. Plugin types go into the map.

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Dynamic Send Type Registration

#### Entry Point
- Plugin registration via `init()` in `register.go` files
- Engine startup calls `registry.All()` to collect all registrations

#### Transformation Path
1. Plugin registers with `SendTypes: []string{"enhanced-refresh"}` in Registration
2. Engine startup calls `RegisterPluginSendTypes()`
3. For each SendType, engine calls `RegisterSendType(token)` adding to ValidSendTypes
4. Config parsing validates `send [ enhanced-refresh ]` against ValidSendTypes

#### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin registry -> ValidSendTypes | RegisterSendType at startup | [ ] |
| ValidSendTypes -> config validation | Map lookup in parseOneSendFlag | [ ] |

### Integration Points
- `internal/component/plugin/events.go` - ValidSendTypes map (new, parallel to ValidEvents)
- `internal/component/plugin/registry/registry.go` - SendTypes field on Registration (new)
- `internal/component/bgp/reactor/config.go` - parseOneSendFlag validates against dynamic set

### Architectural Verification
- [ ] No bypassed layers -- send types validated through same path as receive
- [ ] No unintended coupling -- engine does not know about enhanced-refresh
- [ ] No duplicated functionality -- reuses EventTypes registration pattern

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Plugin registers SendTypes in Registration | -> | Send type appears in ValidSendTypes | TBD |
| Config has `send [ enhanced-refresh ]` | -> | Accepted when plugin registered | TBD |
| Config has `send [ unknown-type ]` | -> | Rejected as unknown | TBD |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegisterSendType` | `internal/component/plugin/events_test.go` | RegisterSendType adds to ValidSendTypes | |
| `TestParseOneSendFlagDynamic` | `internal/component/bgp/reactor/config_test.go` | Registered send types accepted | |
| `TestParseOneSendFlagRejectsUnregistered` | `internal/component/bgp/reactor/config_test.go` | Unregistered send types rejected | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A -- no numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-enhanced-refresh` | `test/plugin/enhanced-refresh.ci` | Config with enhanced-refresh, plugin auto-loaded | |

### Future (if deferring any tests)
- Full BORR/EORR lifecycle test (deferred until plugin handles protocol logic)

## Files to Modify
- `internal/component/plugin/events.go` - Add ValidSendTypes map, RegisterSendType, ValidSendTypeNames
- `internal/component/plugin/registry/registry.go` - Add SendTypes field to Registration
- `internal/component/plugin/resolve.go` - Add RegisterPluginSendTypes (parallel to RegisterPluginEventTypes)
- `internal/component/bgp/reactor/config.go` - parseOneSendFlag validates against dynamic set
- `internal/component/bgp/reactor/peersettings.go` - Remove SendEnhancedRefresh bool, add SendCustom map
- `internal/component/plugin/types.go` - Same on PeerProcessBinding
- `internal/component/bgp/reactor/reactor_api.go` - Copy SendCustom

## Files to Create
- `internal/component/bgp/plugins/enhanced_refresh/` - New plugin (register.go, handler, schema)
- `test/plugin/enhanced-refresh-autoload.ci` - Functional test

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] Yes | `enhanced_refresh/schema/` |
| Plugin all.go blank import | [ ] Yes | `internal/component/plugin/all/all.go` |
| Plugin count in tests | [ ] Yes | Update TestAllPluginsRegistered |
| Functional tests | [ ] Yes | `test/plugin/enhanced-refresh*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] Yes | `docs/features.md` -- enhanced-refresh plugin |
| 2 | Config syntax changed? | [ ] No | Already accepts enhanced-refresh |
| 3 | CLI command added/changed? | [ ] No | |
| 4 | API/RPC added/changed? | [ ] No | |
| 5 | Plugin added/changed? | [ ] Yes | `docs/guide/plugins.md` -- new plugin |
| 6 | Has a user guide page? | [ ] Yes | `docs/guide/route-refresh.md` or similar |
| 7 | Wire format changed? | [ ] No | |
| 8 | Plugin SDK/protocol changed? | [ ] Yes | `.claude/rules/plugin-design.md` -- SendTypes field |
| 9 | RFC behavior implemented? | [ ] Yes | `rfc/short/rfc7313.md` |
| 10 | Test infrastructure changed? | [ ] No | |
| 11 | Affects daemon comparison? | [ ] Maybe | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] Yes | `docs/architecture/core-design.md` -- dynamic send types |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Critical Review Checklist below |
| 6-12 | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase 1: Dynamic send type registration** -- ValidSendTypes, RegisterSendType, RegisterPluginSendTypes
   - Tests: TestRegisterSendType
   - Files: `events.go`, `resolve.go`, `registry.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase 2: Config parser uses dynamic set** -- parseOneSendFlag validates against ValidSendTypes, SendCustom map
   - Tests: TestParseOneSendFlagDynamic, TestParseOneSendFlagRejectsUnregistered
   - Files: `config.go`, `peersettings.go`, `types.go`, `reactor_api.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase 3: Enhanced refresh plugin** -- registers enhanced-refresh send type, handles BORR/EORR
   - Tests: functional test
   - Files: `enhanced_refresh/`, `all/all.go`
   - Verify: refresh.ci and rib-reconnect.ci still pass

4. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | No hardcoded enhanced-refresh in engine code |
| Naming | SendTypes parallel to EventTypes, SendCustom parallel to ReceiveCustom |
| Data flow | Send types validated through dynamic registry, not switch cases |
| Rule: plugin-design | Plugin registers send type, engine validates generically |
| Rule: no-layering | SendEnhancedRefresh bool fully removed |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| ValidSendTypes map exists | `grep ValidSendTypes events.go` |
| SendTypes field on Registration | `grep SendTypes registry.go` |
| No hardcoded enhanced-refresh in engine | `grep enhanced-refresh config.go` returns 0 matches |
| Enhanced refresh plugin registered | `grep enhanced.refresh all.go` |
| Functional tests exist | `ls test/plugin/enhanced-refresh*.ci` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | RegisterSendType rejects empty strings, whitespace |
| Resource exhaustion | SendTypes registration bounded by plugin count |

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
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

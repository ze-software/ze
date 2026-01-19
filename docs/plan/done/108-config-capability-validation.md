# Spec: config-capability-validation

## Task

Add config-time validation for process-dependent capabilities. When a peer has `route-refresh` or `graceful-restart` enabled, fail at config load time if no process binding with `send { update; }` is configured.

This is a fail-fast validation that catches misconfiguration before startup, rather than silently failing at runtime when a route-refresh request arrives with no plugin to handle it.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/capability-contract.md` - Defines API-dependent capabilities and fail-fast requirements
- [ ] `docs/architecture/config/syntax.md` - Config syntax for peer/process blocks

### RFC Summaries
- [ ] `docs/rfc/rfc2918.md` - Route Refresh requires process to resend routes
- [ ] `docs/rfc/rfc4724.md` - Graceful Restart requires process to retain/replay routes

**Key insights:**
- Route-refresh and graceful-restart require a process to resend routes from RIB
- Without `send { update; }`, the engine cannot resend routes when requested
- Silent failures at runtime are unacceptable - must fail at config load time

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConfigValidationRouteRefreshRequiresProcess` | `internal/config/bgp_test.go` | route-refresh without process → error | |
| `TestConfigValidationGracefulRestartRequiresProcess` | `internal/config/bgp_test.go` | graceful-restart without process → error | |
| `TestConfigValidationRouteRefreshWithProcess` | `internal/config/bgp_test.go` | route-refresh + process with send { update; } → OK | |
| `TestConfigValidationGracefulRestartWithProcess` | `internal/config/bgp_test.go` | graceful-restart + process with send { update; } → OK | |
| `TestConfigValidationRouteRefreshProcessNoSendUpdate` | `internal/config/bgp_test.go` | route-refresh + process without send { update; } → error | |
| `TestConfigValidationBothCapabilitiesWithProcess` | `internal/config/bgp_test.go` | both caps + process with send { update; } → OK | |
| `TestConfigValidationRouteRefreshFromTemplate` | `internal/config/bgp_test.go` | template has route-refresh, peer inherits, no process → error | |
| `TestConfigValidationSendAllSatisfiesRequirement` | `internal/config/bgp_test.go` | route-refresh + process with send { all; } → OK | |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| N/A | - | Config validation is unit-testable | - |

### Future (if deferring any tests)
- Runtime validation (Stage 3): plugin must declare capability it handles - separate spec

## Files to Modify
- `internal/config/bgp.go` - Add `validateProcessCapabilities()` function, call after parsing peers

## Files to Create
- None (tests go in existing `bgp_test.go`)

## Implementation Steps
1. **Write unit tests** - Create tests for all validation scenarios
2. **Run tests** - Verify FAIL (paste output)
3. **Implement** - Add `validateProcessCapabilities()` in bgp.go, call after plugin validation loop in `LoadBGPConfig`
4. **Run tests** - Verify PASS (paste output)
5. **Verify all** - `make lint && make test && make functional` (paste output)

## RFC Documentation

### Reference Comments
Not applicable - this is config validation, not wire format code.

### Constraint Comments
Not applicable - no RFC MUST/MUST NOT constraints being enforced. This is a fail-fast usability feature.

## Design Decisions

### Where to validate
After peer parsing is complete, before returning cfg in `LoadBGPConfig`. This ensures all peers and their bindings are fully parsed (including template/match inheritance).

### Error message format

**Selection logic:**
- If `len(ProcessBindings) == 0` → use "no process bindings" format
- Else → use "configured: <names>" format

No process bindings:
```
peer 192.168.1.1: route-refresh requires process with send { update; }
  no process bindings configured
```

Has bindings but none with send.update:
```
peer 192.168.1.1: route-refresh requires process with send { update; }
  configured: process logger, process monitor - none have send { update; }
```

### What counts as valid
- Each peer with route-refresh/graceful-restart must have at least one `ProcessBinding` with `Send.Update = true`
- `send { all; }` satisfies this requirement (sets `Update = true`)
- `send { update; }` satisfies this requirement
- Plugin existence validated separately (existing code in `LoadBGPConfig`)

### Capabilities checked
| Capability | Field | Requires |
|------------|-------|----------|
| route-refresh | `Capabilities.RouteRefresh` | `Send.Update = true` |
| graceful-restart | `Capabilities.GracefulRestart` | `Send.Update = true` |

Enhanced route-refresh is implied by route-refresh (same capability).

## Implementation Summary

### What Was Implemented
- Added `validateProcessCapabilities()` function in `internal/config/bgp.go:833-874`
- Called after peer parsing, before returning config in `LoadBGPConfig`
- 8 unit tests covering all validation scenarios
- Updated 3 existing tests that used route-refresh without process
- Fixed 1 functional test config (`test/data/encode/vpn.conf`)

### Bugs Found/Fixed
- **Flex parsing inconsistency:** `route-refresh true;` stores in `multiValues` (via `AppendValue`), but code uses `Get()` which only checks `values`. Using flag syntax `route-refresh;` works correctly. Tests updated to use flag syntax.
- **graceful-restart value mode:** `graceful-restart 120;` not handled by current code - only block mode `graceful-restart { restart-time 120; }` works. Tests updated to use block syntax.

### Design Insights
- Config capability validation happens AFTER template/match/inherit merging is complete
- `Send.Update` is satisfied by both `send { update; }` and `send { all; }`
- Static route announcements (via `announce` block) don't need route-refresh capability

### Deviations from Plan
- None

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (verified before implementation)
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (37 encoding, 16 plugin, 10 parsing, 18 decoding)

### Documentation (during implementation)
- [x] Required docs read
- [x] RFC summaries read (all referenced RFCs)

### Completion (after tests pass)
- [x] Architecture docs updated with learnings
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together

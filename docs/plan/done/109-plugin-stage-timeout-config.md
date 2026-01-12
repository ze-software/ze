# Spec: plugin-stage-timeout-config

## Task

Make the plugin stage timeout configurable per-plugin. Currently hardcoded as `5s` in `pkg/plugin/server.go`. Add `timeout` keyword to plugin config block.

## Required Reading

### Architecture Docs
- [x] `docs/architecture/config/syntax.md` - Config syntax for plugin blocks
- [x] `docs/architecture/api/capability-contract.md` - Startup protocol stages

### RFC Summaries (MUST for protocol work)
Not applicable - this is config/infrastructure, not BGP protocol.

**Key insights:**
- Stage timeout controls how long engine waits for plugin to complete each startup stage
- 5 stages: Declaration, Config, Capability, Registry, Running
- Timeout applies per-stage, not total startup time
- Default 5s is reasonable for most plugins; some may need longer (slow init, network calls)

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPluginConfigTimeout` | `pkg/config/bgp_test.go` | `timeout 10s;` parses to 10s | ✅ |
| `TestPluginConfigTimeoutDefault` | `pkg/config/bgp_test.go` | missing timeout → 0 (use default) | ✅ |
| `TestPluginConfigTimeoutInvalid` | `pkg/config/bgp_test.go` | `timeout abc;` → error | ✅ |
| `TestPluginConfigTimeoutNegative` | `pkg/config/bgp_test.go` | `timeout -5s;` → error | ✅ |
| `TestPluginConfigTimeoutVariants` | `pkg/config/bgp_test.go` | Various duration formats | ✅ |
| `TestServerUsesPluginTimeout` | `pkg/plugin/server_test.go` | Server uses configured timeout | Deferred |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| N/A | - | Config parsing is unit-testable | - |

### Future (if deferring any tests)
- `TestServerUsesPluginTimeout` - Integration test for server timeout behavior. Would require spawning real process with mock coordinator. Current unit tests validate config parsing and code inspection confirms correct usage in `stageTransition()`.

## Files to Modify
- `pkg/config/bgp.go` - Add `StageTimeout` to `PluginConfig`, add `timeout` to schema, parse in `TreeToConfig()`
- `pkg/config/loader.go` - Pass `StageTimeout` through to reactor config
- `pkg/reactor/reactor.go` - Add `StageTimeout` to `PluginConfig`, pass to plugin config
- `pkg/plugin/types.go` - Add `StageTimeout` to `PluginConfig`
- `pkg/plugin/server.go` - Use `proc.config.StageTimeout` in `stageTransition()` and "ready" handling

## Files to Create
- None

## Implementation Steps
1. **Write unit tests** - Create unit tests BEFORE implementation (strict TDD)
2. **Run tests** - Verify FAIL (paste output)
3. **Implement** - Minimal code to pass
4. **Run tests** - Verify PASS (paste output)
5. **RFC refs** - N/A (not protocol code)
6. **RFC constraints** - N/A (not protocol code)
7. **Functional tests** - N/A (config parsing is unit-testable)
8. **Verify all** - `make lint && make test && make functional` (paste output)

## Design Decisions

### Config syntax
```
plugin myapp {
    run ./myapp;
    encoder json;
    timeout 10s;    # stage timeout (default: 5s)
}
```

### Duration format
Use Go's `time.ParseDuration()` format: `5s`, `500ms`, `1m`, `1m30s`

### Default behavior
- `timeout 0;` or missing → use `defaultStageTimeout` (5s)
- Explicit `timeout 5s;` → use 5s
- Negative values → rejected with error

### Per-plugin vs global
Each plugin can have its own timeout. No global override needed.

### Where timeout is stored
```go
type PluginConfig struct {
    Name          string
    Run           string
    Encoder       string
    ReceiveUpdate bool
    StageTimeout  time.Duration  // NEW: 0 = use default 5s
}
```

### Where timeout is used
```go
// pkg/plugin/server.go
func (s *Server) stageTransition(proc *Process, ...) bool {
    timeout := proc.config.StageTimeout
    if timeout == 0 {
        timeout = defaultStageTimeout
    }
    stageCtx, cancel := context.WithTimeout(s.ctx, timeout)
    // ...
}
```

## RFC Documentation

N/A - This is config/infrastructure, not BGP protocol code.

## Implementation Summary

### What Was Implemented
- Added `StageTimeout time.Duration` field to `PluginConfig` in three locations:
  - `pkg/config/bgp.go` (config parsing)
  - `pkg/reactor/reactor.go` (reactor config)
  - `pkg/plugin/types.go` (plugin internal config)
- Added `timeout` keyword to BGPSchema for plugin block
- Added timeout parsing in `TreeToConfig()` using `time.ParseDuration()`
- Added negative timeout validation (must be non-negative; 0 = use default)
- Updated `stageTransition()` and "ready" handling to use per-plugin timeout
- Updated config passthrough: `config.Loader` → `reactor.PluginConfig` → `plugin.PluginConfig`
- Documented multi-plugin timeout synchronization semantics in `docs/architecture/config/syntax.md`

### Bugs Found/Fixed
- Negative timeout validation: `time.ParseDuration("-5s")` succeeds but would cause immediate context expiration. Added validation to reject negative values. Added `TestPluginConfigTimeoutNegative` test.

### Investigation → Test Rule
- Discovered negative duration edge case during code review
- Added test to prevent regression: `TestPluginConfigTimeoutNegative`
- Future devs don't need to re-investigate this edge case

### Design Insights
- Three `PluginConfig` types exist across packages (config, reactor, plugin) - each needs updating when adding fields
- Timeout uses `proc.config.StageTimeout` directly - no additional field on Process struct needed
- Multi-plugin timeout semantics: each plugin uses its own timeout when waiting for ALL plugins to sync. With different timeouts, fast plugins may timeout waiting for slow ones. Documented in config/syntax.md.

### Deviations from Plan
- Added `TestPluginConfigTimeoutVariants` test for coverage of duration formats (not in original plan)
- Added `TestPluginConfigTimeoutNegative` test for negative timeout rejection (found during review)
- Deferred `TestServerUsesPluginTimeout` - would require spawning real process with mock coordinator

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (output: `cfg.Plugins[0].StageTimeout undefined`)
- [x] Implementation complete
- [x] Tests PASS (all 5 tests pass)

### Verification
- [x] `make lint` passes (26 linters including `govet`, `staticcheck`, `gosec`, `gocritic`)
- [x] `make test` passes
- [x] `make functional` passes

### Documentation (during implementation)
- [x] Required docs read
- [x] RFC summaries read (N/A - not protocol work)
- [x] RFC references added to code (N/A - not protocol code)
- [x] RFC constraint comments added (N/A - not protocol code)
- [x] `docs/architecture/config/syntax.md` updated with `timeout` keyword

### Completion (after tests pass)
- [x] Architecture docs updated with learnings
- [x] Spec updated with Implementation Summary
- [x] Spec moved to `docs/plan/done/109-plugin-stage-timeout-config.md`
- [ ] All files committed together

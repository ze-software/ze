# Spec: API Command Restructure - Step 3: System Namespace

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/ipc_protocol.md` - target protocol spec
4. `internal/plugin/handler.go` - current system handlers

## Task

Enhance `system` namespace with new commands and update existing ones.

**Changes:**
| Old | New |
|-----|-----|
| `system version` | `system version software` |
| *new* | `system version api` |
| *new* | `system shutdown` |
| *new* | `system subsystem list` |

**Scope change:**
- `system command list` - Returns only system namespace commands
- `system command help` - Help for system commands only
- `system command complete` - Completion for system commands only

**No backward compatibility** - `system version` alone will fail.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/ipc_protocol.md` - target command structure

### Source Files
- [ ] `internal/plugin/handler.go` - current system handlers

## Current State

**handler.go registrations:**
```go
d.Register("system help", handleSystemHelp, ...)
d.Register("system version", handleSystemVersion, ...)
d.Register("system command list", handleSystemCommandList, ...)
d.Register("system command help", handleSystemCommandHelp, ...)
d.Register("system command complete", handleSystemCommandComplete, ...)
```

**system version returns:**
```json
{"status":"done","data":{"version":"0.1.0"}}
```

## Target State

**New registrations:**
```go
d.Register("system help", handleSystemHelp, "List system subcommands")
d.Register("system version software", handleSystemVersionSoftware, "Show ZeBGP version")
d.Register("system version api", handleSystemVersionAPI, "Show IPC protocol version")
d.Register("system shutdown", handleSystemShutdown, "Graceful application shutdown")
d.Register("system subsystem list", handleSystemSubsystemList, "List available subsystems")
d.Register("system command list", handleSystemCommandList, "List system commands")
d.Register("system command help", handleSystemCommandHelp, "Show command details")
d.Register("system command complete", handleSystemCommandComplete, "Complete command/args")
```

**system version software returns:**
```json
{"type":"response","response":{"status":"done","data":{"version":"0.1.0"}}}
```

**system version api returns:**
```json
{"type":"response","response":{"status":"done","data":{"version":"2.0"}}}
```

**system subsystem list returns:**
```json
{"type":"response","response":{"status":"done","data":{"subsystems":["bgp","rib"]}}}
```

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDispatchSystemVersionSoftware` | `internal/plugin/handler_test.go` | `system version software` returns version | |
| `TestDispatchSystemVersionAPI` | `internal/plugin/handler_test.go` | `system version api` returns "2.0" | |
| `TestDispatchSystemShutdown` | `internal/plugin/handler_test.go` | `system shutdown` triggers reactor stop | |
| `TestDispatchSystemSubsystemList` | `internal/plugin/handler_test.go` | `system subsystem list` returns subsystems | |
| `TestOldSystemVersionRemoved` | `internal/plugin/handler_test.go` | `system version` alone returns error | |
| `TestSystemCommandListScope` | `internal/plugin/handler_test.go` | Lists only system commands | |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `system-commands` | `test/data/plugin/system-commands.ci` | System namespace commands | |

## Files to Modify

| File | Changes |
|------|---------|
| `internal/plugin/handler.go` | Update system registrations and handlers |

## Constants to Add

```go
// APIVersion is the IPC protocol version.
const APIVersion = "2.0"
```

## Implementation Steps

1. **Write unit tests** - Create tests for new system commands
2. **Run tests** - Verify FAIL (paste output)
3. **Add APIVersion constant** - In handler.go or types.go
4. **Update handleSystemVersion** - Rename to `handleSystemVersionSoftware`
5. **Add handleSystemVersionAPI** - Return APIVersion
6. **Add handleSystemShutdown** - Call reactor.Stop() (application-level shutdown)
7. **Add handleSystemSubsystemList** - Return configured subsystems
8. **Update handleSystemCommandList** - Filter to system namespace only
9. **Update registrations** - New paths
10. **Run tests** - Verify PASS (paste output)
11. **Verify all** - `make lint && make test && make functional` (paste output)

## Handler Implementations

### handleSystemVersionAPI
```go
func handleSystemVersionAPI(_ *CommandContext, _ []string) (*Response, error) {
    return NewResponse("done", map[string]any{
        "version": APIVersion,
    }), nil
}
```

### handleSystemShutdown
```go
func handleSystemShutdown(ctx *CommandContext, _ []string) (*Response, error) {
    // Application-level shutdown (not just BGP subsystem)
    ctx.Reactor.Stop()
    return NewResponse("done", map[string]any{
        "message": "shutdown initiated",
    }), nil
}
```

### handleSystemSubsystemList
```go
func handleSystemSubsystemList(ctx *CommandContext, _ []string) (*Response, error) {
    // Return configured subsystems
    // For now, hardcode; later query reactor for enabled subsystems
    subsystems := []string{"bgp"}
    if ctx.Reactor.HasRIB() {  // Add method if needed
        subsystems = append(subsystems, "rib")
    }
    return NewResponse("done", map[string]any{
        "subsystems": subsystems,
    }), nil
}
```

**Note:** Handlers return `*Response`. The `WrapResponse()` function (from Step 1) wraps responses at serialization time to produce the final `{"type":"response","response":{...}}` format.

## System vs BGP Shutdown

| Command | Scope | Effect |
|---------|-------|--------|
| `system shutdown` | Application | Stop entire ZeBGP process |
| `bgp daemon shutdown` | BGP subsystem | Stop BGP but keep API running |

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Completion
- [ ] All files committed together
- [ ] Spec moved to `docs/plan/done/`

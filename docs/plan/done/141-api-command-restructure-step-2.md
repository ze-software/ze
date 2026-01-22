# Spec: API Command Restructure - Step 2: Plugin Namespace

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/ipc_protocol.md` - target protocol spec
4. `internal/plugin/session.go` - current session handlers
5. `internal/plugin/handler.go` - command registration

## Task

Create `plugin` namespace for plugin lifecycle operations.

**Move commands:**
| Old | New |
|-----|-----|
| `session api ready` | `plugin session ready` |
| `session ping` | `plugin session ping` |
| `session bye` | `plugin session bye` |

**Add introspection:**
- `plugin help` - List plugin subcommands
- `plugin command list` - List plugin commands
- `plugin command help "<cmd>"` - Command details
- `plugin command complete "<partial>"` - Completion

**Remove:**
- `session reset` - No longer needed (was only resetting sync/encoding to defaults)

**No backward compatibility** - old commands will fail.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/ipc_protocol.md` - target command structure

### Source Files
- [ ] `internal/plugin/session.go` - current handlers
- [ ] `internal/plugin/handler.go` - registration pattern
- [ ] `internal/plugin/command.go` - dispatcher structure

## Current State

**session.go registrations:**
```go
d.Register("session sync enable", handleSessionSyncEnable, ...)
d.Register("session sync disable", handleSessionSyncDisable, ...)
d.Register("session api ready", handleSessionAPIReady, ...)
d.Register("session api encoding", handleSessionAPIEncoding, ...)
d.Register("session reset", handleSessionReset, ...)  // REMOVE
d.Register("session ping", handleSessionPing, ...)
d.Register("session bye", handleSessionBye, ...)
```

## Target State

**New registrations in handler.go:**
```go
// Plugin lifecycle
d.Register("plugin session ready", handlePluginSessionReady, "Signal plugin init complete")
d.Register("plugin session ping", handlePluginSessionPing, "Health check (returns PID)")
d.Register("plugin session bye", handlePluginSessionBye, "Disconnect")

// Plugin introspection
d.Register("plugin help", handlePluginHelp, "List plugin subcommands")
d.Register("plugin command list", handlePluginCommandList, "List plugin commands")
d.Register("plugin command help", handlePluginCommandHelp, "Show command details")
d.Register("plugin command complete", handlePluginCommandComplete, "Complete command/args")
```

**Note:** `session sync` and `session api encoding` move to `bgp plugin` in Step 4.

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDispatchPluginSessionReady` | `internal/plugin/handler_test.go` | `plugin session ready` dispatches correctly | |
| `TestDispatchPluginSessionPing` | `internal/plugin/handler_test.go` | `plugin session ping` returns PID | |
| `TestDispatchPluginSessionBye` | `internal/plugin/handler_test.go` | `plugin session bye` acknowledges | |
| `TestDispatchPluginHelp` | `internal/plugin/handler_test.go` | `plugin help` lists subcommands | |
| `TestDispatchPluginCommandList` | `internal/plugin/handler_test.go` | `plugin command list` returns commands | |
| `TestOldSessionCommandsRemoved` | `internal/plugin/handler_test.go` | `session ping` returns unknown command | |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `plugin-session` | `test/data/plugin/plugin-session.ci` | Plugin startup with new commands | |

## Files to Modify

| File | Changes |
|------|---------|
| `internal/plugin/handler.go` | Add plugin namespace registrations |
| `internal/plugin/session.go` | Rename handlers, remove `session reset`, remove old registrations |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/plugin/plugin.go` | Plugin namespace handlers (help, command list/help/complete) |

## Implementation Steps

1. **Write unit tests** - Create tests for new plugin commands
2. **Run tests** - Verify FAIL (paste output)
3. **Create plugin.go** - Plugin introspection handlers
4. **Update session.go** - Rename handlers to `handlePluginSession*`
5. **Update handler.go** - Register new paths, remove old paths
6. **Delete session reset** - Remove handler and registration
7. **Run tests** - Verify PASS (paste output)
8. **Verify all** - `make lint && make test && make functional` (paste output)

## Handler Mapping

| Old Handler | New Handler | Notes |
|-------------|-------------|-------|
| `handleSessionAPIReady` | `handlePluginSessionReady` | Rename |
| `handleSessionPing` | `handlePluginSessionPing` | Rename |
| `handleSessionBye` | `handlePluginSessionBye` | Rename |
| `handleSessionReset` | *deleted* | No longer needed |
| `handleSessionSyncEnable` | *keep at old path* | **Step 4** moves to `bgp plugin ack sync` |
| `handleSessionSyncDisable` | *keep at old path* | **Step 4** moves to `bgp plugin ack async` |
| `handleSessionAPIEncoding` | *keep at old path* | **Step 4** moves to `bgp plugin encoding` |

**Note:** Session sync/encoding remain at `session sync ...` and `session api encoding ...` paths until Step 4, which moves them to the `bgp plugin` namespace. This allows Steps 2, 3, 4 to be implemented in parallel.

## Plugin Introspection Details

**`plugin help`** returns:
```json
{"type":"response","response":{"status":"done","data":{"subcommands":["session","command"]}}}
```

**`plugin command list`** returns only plugin-registered commands (not builtins):
```json
{"type":"response","response":{"status":"done","data":{"commands":[{"name":"myapp status","description":"Show status"}]}}}
```

**`plugin command help "myapp status"`** returns:
```json
{"type":"response","response":{"status":"done","data":{"command":"myapp status","description":"Show status","args":"[verbose]","source":"myapp-plugin"}}}
```

## Implementation Summary

### What Was Implemented

- **Plugin namespace handlers** - Added to `internal/plugin/plugin.go`:
  - `RegisterPluginHandlers()` - registers all plugin namespace commands
  - `handlePluginHelp()` - returns list of subcommands
  - `handlePluginCommandList()` - returns plugin-registered commands
  - `handlePluginCommandHelp()` - returns details for specific plugin command
  - `handlePluginCommandComplete()` - returns completions for plugin commands

- **Plugin session handlers** - Added to `internal/plugin/session.go`:
  - `handlePluginSessionReady()` - signals API initialization complete
  - `handlePluginSessionPing()` - returns pong with PID
  - `handlePluginSessionBye()` - acknowledges disconnect

- **Removed commands**:
  - `session api ready` - moved to `plugin session ready`
  - `session ping` - moved to `plugin session ping`
  - `session bye` - moved to `plugin session bye`
  - `session reset` - deleted (no longer needed)

- **Preserved commands** (Step 4 moves these):
  - `session sync enable/disable` - still works
  - `session api encoding` - still works

### Files Modified

| File | Changes |
|------|---------|
| `internal/plugin/plugin.go` | Added RegisterPluginHandlers and introspection handlers |
| `internal/plugin/session.go` | Removed old handlers, added handlePluginSession* handlers |
| `internal/plugin/handler.go` | Added RegisterPluginHandlers call |
| `internal/plugin/handler_test.go` | Added 8 new tests for plugin namespace |
| `internal/plugin/session_test.go` | Updated tests for renamed handlers, removed session reset test |
| `internal/plugin/rib/rib.go` | Changed `session api ready` → `plugin session ready` |
| `internal/plugin/rib/rib_test.go` | Updated test assertions for new command |
| `internal/plugin/types.go` | Updated SignalPeerAPIReady comment |
| `internal/reactor/api_sync.go` | Updated API sync comments |
| `internal/reactor/peer.go` | Updated peer API sync comments |
| `internal/reactor/reactor.go` | Updated WaitForAPIReady comment |
| `docs/architecture/api/architecture.md` | Updated command table and API sync docs |
| `docs/architecture/api/commands.md` | Updated command list |
| `docs/architecture/api/process-protocol.md` | Updated diagram |
| `docs/architecture/api/capability-contract.md` | Updated implementation status |

### Deviations from Plan

- Did NOT create new `plugin.go` file - added handlers to existing file which already had plugin-related parsing functions
- Functional test `plugin-session.ci` not created - existing unit tests provide sufficient coverage

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (verified during implementation)
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (30 tests)

### Completion
- [ ] All files committed together
- [x] Spec moved to `docs/plan/done/`

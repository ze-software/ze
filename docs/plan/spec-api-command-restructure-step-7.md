# Spec: API Command Restructure - Step 7: RIB Namespace & Plugin Commands

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/ipc_protocol.md` - RIB namespace, plugin commands
4. `internal/plugin/msgid.go` - current msg-id handlers (to be removed)
5. `internal/plugin/forward.go` - current forward handlers (to be removed)
6. `internal/plugin/rib/rib.go` - RIB plugin (to be updated)

## Task

Create `rib` namespace and make cache/RIB commands plugin-provided.

**Built-in RIB commands:**
- `rib help` - List rib subcommands
- `rib command list` - List rib commands
- `rib command help "<cmd>"` - Command details
- `rib command complete "<partial>"` - Completion
- `rib event list` - List available RIB event types

**Plugin-provided commands (registered by RIB plugin):**
| Command | Registered By | Description |
|---------|---------------|-------------|
| `bgp cache <id> forward <sel>` | RIB plugin | Forward cached UPDATE |
| `bgp cache <id> retain` | RIB plugin | Prevent eviction |
| `bgp cache <id> release` | RIB plugin | Allow eviction |
| `bgp cache <id> expire` | RIB plugin | Delete immediately |
| `bgp cache list` | RIB plugin | List cached msg-ids |
| `rib show in [peer]` | RIB plugin | Show Adj-RIB-In |
| `rib clear in [peer]` | RIB plugin | Clear Adj-RIB-In |

**Remove from engine:**
- `msg-id retain/release/expire/list` handlers
- `forward update-id` handler
- `delete update-id` handler
- `rib show in` handler (now plugin-provided)
- `rib clear in` handler (now plugin-provided)

**Why plugin-provided:**
- Cache/RIB functionality requires a RIB plugin to be useful
- Engine provides reactor methods; plugins register commands that use them
- Without RIB plugin, these commands don't exist

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/ipc_protocol.md` - RIB namespace

### Source Files
- [ ] `internal/plugin/msgid.go` - msg-id handlers (to be deleted)
- [ ] `internal/plugin/forward.go` - forward handlers (to be deleted)
- [ ] `internal/plugin/handler.go` - rib show/clear handlers (to be deleted)
- [ ] `internal/plugin/rib/rib.go` - RIB plugin

## Current State

**msgid.go:**
```go
d.Register("msg-id retain", handleMsgIDRetain, ...)
d.Register("msg-id release", handleMsgIDRelease, ...)
d.Register("msg-id expire", handleMsgIDExpire, ...)
d.Register("msg-id list", handleMsgIDList, ...)
```

**forward.go:**
```go
d.Register("forward update-id", handleForwardUpdateID, ...)
d.Register("delete update-id", handleDeleteUpdateID, ...)
```

**handler.go:**
```go
d.Register("rib show in", handleRIBShowIn, ...)
d.Register("rib clear in", handleRIBClearIn, ...)
```

## Target State

**Built-in registrations only:**
```go
// RIB introspection (built-in)
d.Register("rib help", handleRibHelp, "List rib subcommands")
d.Register("rib command list", handleRibCommandList, "List rib commands")
d.Register("rib command help", handleRibCommandHelp, "Show command details")
d.Register("rib command complete", handleRibCommandComplete, "Complete command/args")
d.Register("rib event list", handleRibEventList, "List available RIB event types")
```

**RIB plugin registers dynamically:**
```go
// In plugin startup Stage 1 (REGISTRATION)
declare cmd bgp cache forward
declare cmd bgp cache retain
declare cmd bgp cache release
declare cmd bgp cache expire
declare cmd bgp cache list
declare cmd rib show in
declare cmd rib clear in
declare done
```

## 🧪 TDD Test Plan

### Boundary Tests (plugin-provided commands)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| msg-id | 1-uint64 max | 18446744073709551615 | 0 | N/A (uint64) |
| msg-id format | numeric | `12345` | `abc`, `-1`, empty | overflow string |

**Note:** These tests apply to the RIB plugin's handling of `bgp cache <id>` commands.

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDispatchRibHelp` | `internal/plugin/handler_test.go` | `rib help` returns subcommands | |
| `TestDispatchRibCommandList` | `internal/plugin/handler_test.go` | `rib command list` returns commands | |
| `TestDispatchRibEventList` | `internal/plugin/handler_test.go` | `rib event list` returns event types | |
| `TestOldMsgIdCommandsRemoved` | `internal/plugin/handler_test.go` | `msg-id retain` returns unknown | |
| `TestOldForwardCommandsRemoved` | `internal/plugin/handler_test.go` | `forward update-id` returns unknown | |
| `TestOldRibShowRemoved` | `internal/plugin/handler_test.go` | Built-in `rib show in` removed | |
| `TestPluginRegistersBgpCache` | `internal/plugin/rib/rib_test.go` | RIB plugin registers cache commands | |
| `TestPluginRegistersRibShow` | `internal/plugin/rib/rib_test.go` | RIB plugin registers rib commands | |
| `TestBgpCacheMsgIdZero` | `internal/plugin/rib/rib_test.go` | `bgp cache 0 retain` fails | |
| `TestBgpCacheMsgIdInvalid` | `internal/plugin/rib/rib_test.go` | `bgp cache abc retain` fails | |
| `TestBgpCacheMsgIdNegative` | `internal/plugin/rib/rib_test.go` | `bgp cache -1 retain` fails | |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `rib-plugin-commands` | `test/data/plugin/rib-plugin-commands.ci` | RIB plugin command registration | |

## Files to Delete

| File | Reason |
|------|--------|
| `internal/plugin/msgid.go` | Commands now plugin-provided |
| `internal/plugin/forward.go` | Commands now plugin-provided |

## Files to Modify

| File | Changes |
|------|---------|
| `internal/plugin/handler.go` | Remove rib show/clear, add rib introspection |
| `internal/plugin/rib/rib.go` | Update command strings, register new commands |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/plugin/rib.go` | RIB namespace introspection handlers |

## RIB Plugin Command Registration

**Stage 1 declarations:**
```
declare cmd bgp cache forward "<id>" "<selector>" "Forward cached UPDATE to peers"
declare cmd bgp cache retain "<id>" "Prevent eviction of cached UPDATE"
declare cmd bgp cache release "<id>" "Allow eviction of cached UPDATE"
declare cmd bgp cache expire "<id>" "Delete cached UPDATE immediately"
declare cmd bgp cache list "List all cached msg-ids"
declare cmd rib show in "[peer]" "Show Adj-RIB-In"
declare cmd rib clear in "[peer]" "Clear Adj-RIB-In"
declare done
```

**Command handler in RIB plugin:**
When engine receives `bgp cache 12345 forward *`, it routes to RIB plugin:
```json
{"type":"request","serial":"abc","command":"bgp cache forward","args":["12345","*"]}
```

Plugin responds:
```json
{"serial":"abc","status":"done","data":{"msg_id":12345,"forwarded_to":["10.0.0.1","10.0.0.2"]}}
```

## Implementation Steps

1. **Write unit tests** - Create tests for rib namespace and removed commands
2. **Run tests** - Verify FAIL (paste output)
3. **Create rib.go** - RIB introspection handlers
4. **Update handler.go** - Add rib introspection, remove old rib show/clear
5. **Delete msgid.go** - Remove file entirely
6. **Delete forward.go** - Remove file entirely
7. **Update rib/rib.go** - Register new command strings
8. **Run tests** - Verify PASS (paste output)
9. **Verify all** - `make lint && make test && make functional` (paste output)

## RIB Introspection Handlers

### handleRibEventList
```go
func handleRibEventList(_ *CommandContext, _ []string) (*Response, error) {
    events := []string{"cache", "route"}
    return NewResponse("done", map[string]any{
        "events": events,
    }), nil
}
```

### handleRibCommandList
```go
func handleRibCommandList(ctx *CommandContext, _ []string) (*Response, error) {
    // Return only rib namespace commands from plugin registry
    var commands []Completion

    for _, cmd := range ctx.Dispatcher.Registry().All() {
        if strings.HasPrefix(cmd.Name, "rib ") {
            commands = append(commands, Completion{
                Value: cmd.Name,
                Help:  cmd.Description,
            })
        }
    }

    return NewResponse("done", map[string]any{
        "commands": commands,
    }), nil
}
```

**Note:** Handlers return `*Response`. The `WrapResponse()` function wraps at serialization time.

## RIB Plugin Updates

**Old command strings in rib/rib.go:**
```go
proc.WriteCommand("peer %s update text ...", peer)
proc.WriteCommand("peer %s session api ready", peer)
proc.WriteCommand("peer %s borr %s", peer, family)
proc.WriteCommand("peer %s eorr %s", peer, family)
```

**New command strings:**
```go
proc.WriteCommand("bgp peer %s update text ...", peer)
proc.WriteCommand("bgp peer %s ready", peer)
proc.WriteCommand("bgp peer %s borr %s", peer, family)
proc.WriteCommand("bgp peer %s eorr %s", peer, family)
```

## Reactor Methods Still Available

The ReactorInterface methods remain available for plugins to call:
- `ForwardUpdate(sel, updateID)`
- `DeleteUpdate(updateID)`
- `RetainUpdate(updateID)`
- `ReleaseUpdate(updateID)`
- `ListUpdates()`
- `RIBInRoutes(peerID)`
- `ClearRIBIn()`

Plugins call these via the engine when handling their registered commands.

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

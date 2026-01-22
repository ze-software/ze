# Spec: API Command Restructure - Step 4: BGP Namespace Foundation

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/ipc_protocol.md` - target protocol spec
4. `internal/plugin/session.go` - session sync/encoding handlers
5. `internal/plugin/handler.go` - command registration

## Task

Create `bgp` namespace foundation with introspection and plugin configuration.

**New commands:**
- `bgp help` - List bgp subcommands
- `bgp command list` - List bgp commands
- `bgp command help "<cmd>"` - Command details
- `bgp command complete "<partial>"` - Completion
- `bgp event list` - List available BGP event types

**Move commands (from Step 2 temporary state):**

| Old (kept in Step 2) | New |
|----------------------|-----|
| `session sync enable` | `bgp plugin ack sync` |
| `session sync disable` | `bgp plugin ack async` |
| `session api encoding` | `bgp plugin encoding json\|text` |
| *new* | `bgp plugin format hex\|base64\|parsed\|full` |

**Note:** Step 2 kept these handlers at their old paths. This step moves them to `bgp plugin` and removes the old paths.

**Remove:**
- `WireEncodingCBOR` - Incompatible with line-delimited protocol

**No backward compatibility** - old commands will fail.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/ipc_protocol.md` - target command structure

### Source Files
- [ ] `internal/plugin/session.go` - sync/encoding handlers
- [ ] `internal/plugin/types.go` - WireEncoding constants
- [ ] `internal/plugin/handler.go` - registration pattern

## Current State

**session.go:**
```go
d.Register("session sync enable", handleSessionSyncEnable, ...)
d.Register("session sync disable", handleSessionSyncDisable, ...)
d.Register("session api encoding", handleSessionAPIEncoding, ...)
```

**types.go WireEncoding:**
```go
const (
    WireEncodingHex  WireEncoding = iota
    WireEncodingB64
    WireEncodingCBOR  // REMOVE
    WireEncodingText
)
```

## Target State

**New registrations:**
```go
// BGP introspection
d.Register("bgp help", handleBgpHelp, "List bgp subcommands")
d.Register("bgp command list", handleBgpCommandList, "List bgp commands")
d.Register("bgp command help", handleBgpCommandHelp, "Show command details")
d.Register("bgp command complete", handleBgpCommandComplete, "Complete command/args")
d.Register("bgp event list", handleBgpEventList, "List available BGP event types")

// BGP plugin configuration
d.Register("bgp plugin encoding", handleBgpPluginEncoding, "Set event encoding (json|text)")
d.Register("bgp plugin format", handleBgpPluginFormat, "Set wire format (hex|base64|parsed|full)")
d.Register("bgp plugin ack", handleBgpPluginAck, "Set ACK timing (sync|async)")
```

**New encoding/format model:**
- `encoding` controls overall structure: `json` (structured) or `text` (human-readable)
- `format` controls wire bytes representation: `hex`, `base64`, `parsed`, `full`
- `format` only applies when `encoding` is `json`

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDispatchBgpHelp` | `internal/plugin/handler_test.go` | `bgp help` returns subcommands | ✅ |
| `TestDispatchBgpCommandList` | `internal/plugin/handler_test.go` | `bgp command list` returns bgp commands | ✅ |
| `TestDispatchBgpEventList` | `internal/plugin/handler_test.go` | `bgp event list` returns event types | ✅ |
| `TestDispatchBgpPluginEncoding` | `internal/plugin/handler_test.go` | `bgp plugin encoding json` sets encoding | ✅ |
| `TestDispatchBgpPluginFormat` | `internal/plugin/handler_test.go` | `bgp plugin format full` sets format | ✅ |
| `TestDispatchBgpPluginAck` | `internal/plugin/handler_test.go` | `bgp plugin ack sync` sets sync mode | ✅ |
| `TestOldSessionSyncRemoved` | `internal/plugin/handler_test.go` | `session sync enable` returns error | ✅ |
| `TestBgpPluginEncodingAllValues` | `internal/plugin/handler_test.go` | All encoding values + Process state | ✅ |
| `TestBgpPluginEncodingInvalid` | `internal/plugin/handler_test.go` | Invalid encoding rejected | ✅ |
| `TestBgpPluginFormatAllValues` | `internal/plugin/handler_test.go` | All format values + Process state | ✅ |
| `TestBgpPluginFormatInvalid` | `internal/plugin/handler_test.go` | Invalid format rejected | ✅ |
| `TestBgpPluginAckAllValues` | `internal/plugin/handler_test.go` | sync/async + Process state | ✅ |
| `TestBgpPluginAckInvalid` | `internal/plugin/handler_test.go` | Invalid ack mode rejected | ✅ |

### Functional Tests

Not required - unit tests provide comprehensive coverage for handler logic.

## Files to Modify

| File | Changes |
|------|---------|
| `internal/plugin/handler.go` | Add bgp namespace registrations |
| `internal/plugin/session.go` | Move handlers, remove old registrations |
| `internal/plugin/types.go` | Remove `WireEncodingCBOR`, add Format constants |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/plugin/bgp.go` | BGP namespace handlers |

## New Constants

```go
// Encoding controls overall message structure.
const (
    EncodingJSON = "json"  // Structured JSON
    EncodingText = "text"  // Human-readable text
)

// Format controls wire bytes representation (JSON only).
const (
    FormatHex    = "hex"     // Hex string
    FormatBase64 = "base64"  // Base64 encoded
    FormatParsed = "parsed"  // Decoded fields only (no wire bytes)
    FormatFull   = "full"    // Both parsed AND wire bytes
)
```

## Implementation Steps

1. **Write unit tests** - Create tests for bgp namespace commands
2. **Run tests** - Verify FAIL (paste output)
3. **Remove WireEncodingCBOR** - Delete from types.go, update ParseWireEncoding
4. **Add encoding/format constants** - In types.go
5. **Create bgp.go** - BGP introspection handlers
6. **Move session handlers** - Rename to `handleBgpPlugin*`
7. **Update registrations** - New paths in handler.go
8. **Remove old session paths** - Delete from session.go
9. **Run tests** - Verify PASS (paste output)
10. **Verify all** - `make lint && make test && make functional` (paste output)

## Handler Implementations

### handleBgpEventList
```go
func handleBgpEventList(_ *CommandContext, _ []string) (*Response, error) {
    events := []string{
        "update", "open", "notification", "keepalive",
        "refresh", "state", "negotiated",
    }
    return NewResponse("done", map[string]any{
        "events": events,
    }), nil
}
```

### handleBgpPluginEncoding
```go
func handleBgpPluginEncoding(ctx *CommandContext, args []string) (*Response, error) {
    if len(args) == 0 {
        return nil, fmt.Errorf("missing encoding: bgp plugin encoding <json|text>")
    }

    enc := strings.ToLower(args[0])
    switch enc {
    case EncodingJSON, EncodingText:
        if ctx.Process != nil {
            ctx.Process.SetEncoding(enc)
        }
    default:
        return nil, fmt.Errorf("invalid encoding: %s (valid: json, text)", args[0])
    }

    return NewResponse("done", map[string]any{
        "encoding": enc,
    }), nil
}
```

### handleBgpPluginAck
```go
func handleBgpPluginAck(ctx *CommandContext, args []string) (*Response, error) {
    if len(args) == 0 {
        return nil, fmt.Errorf("missing mode: bgp plugin ack <sync|async>")
    }

    mode := strings.ToLower(args[0])
    switch mode {
    case "sync":
        if ctx.Process != nil {
            ctx.Process.SetSync(true)
        }
    case "async":
        if ctx.Process != nil {
            ctx.Process.SetSync(false)
        }
    default:
        return nil, fmt.Errorf("invalid mode: %s (valid: sync, async)", args[0])
    }

    return NewResponse("done", map[string]any{
        "ack": mode,
    }), nil
}
```

**Note:** Handlers return `*Response`. The `WrapResponse()` function wraps at serialization time.

## Encoding vs Format Relationship

| encoding | format | Wire Bytes | Parsed Fields | Result |
|----------|--------|------------|---------------|--------|
| `text` | *ignored* | N/A | ✅ | Human-readable text output |
| `json` | `hex` | ✅ hex string | ❌ | JSON with wire bytes only |
| `json` | `base64` | ✅ base64 | ❌ | JSON with wire bytes only |
| `json` | `parsed` | ❌ | ✅ | JSON with decoded fields only (default) |
| `json` | `full` | ✅ hex string | ✅ | JSON with both |

**Default:** `encoding json` + `format parsed` (most efficient for typical plugins).

**Timing:** Encoding/format settings apply at event delivery time, not subscription time.

## Implementation Summary

### What Was Implemented

**New file created:**
- `internal/plugin/bgp.go` - BGP namespace handlers

**Files modified:**
- `internal/plugin/handler.go` - Added `RegisterBgpHandlers(d)` call
- `internal/plugin/session.go` - Removed old sync/encoding handlers, kept plugin session handlers
- `internal/plugin/session_test.go` - Removed tests for removed handlers
- `internal/plugin/types.go` - Removed `WireEncodingCBOR`, added `FormatHex`, `FormatBase64`
- `internal/plugin/process.go` - Added `encoding`/`format` fields and `Encoding()`/`SetEncoding()`/`Format()`/`SetFormat()` methods
- `internal/plugin/update_wire.go` - Removed CBOR case from switch
- `internal/plugin/route.go` - Updated description (removed cbor mention)
- `internal/plugin/handler_test.go` - Added comprehensive tests for all handlers

**Commands implemented:**
- `bgp help` - List bgp subcommands
- `bgp command list [verbose]` - List bgp commands
- `bgp command help "<cmd>"` - Show command details
- `bgp command complete "<partial>"` - Tab completion
- `bgp event list` - List BGP event types
- `bgp plugin encoding <json|text>` - Set event encoding
- `bgp plugin format <hex|base64|parsed|full>` - Set wire format
- `bgp plugin ack <sync|async>` - Set ACK timing

**Commands removed:**
- `session sync enable` → use `bgp plugin ack sync`
- `session sync disable` → use `bgp plugin ack async`
- `session api encoding` → use `bgp plugin encoding` + `bgp plugin format`

### Design Notes

1. **Encoding vs Format separation:**
   - `encoding` (json|text) - overall message structure
   - `format` (hex|base64|parsed|full) - wire bytes representation in JSON mode
   - `format` ignored when `encoding=text`

2. **Process state fields:**
   - `Process.encoding` and `Process.format` store settings as atomic.Value
   - Not yet consumed by output code - will be wired in later steps

3. **FormatRaw vs FormatHex:**
   - Existing code uses `FormatRaw = "raw"` for wire-bytes-only output
   - New API uses `FormatHex = "hex"` per spec
   - Both constants exist; will unify when output code is updated

### Deviations from Plan

- Added comprehensive tests for error paths (invalid inputs, missing args)
- Added tests verifying Process state actually changes
- Did not create functional test - unit tests provide adequate coverage

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (verified during implementation)
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (96 tests)

### Completion
- [ ] All files committed together
- [ ] Spec moved to `docs/plan/done/`

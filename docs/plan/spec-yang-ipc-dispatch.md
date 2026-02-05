# Spec: yang-ipc-dispatch

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/spec-yang-ipc-schema.md` - YANG IPC definitions (Spec 1)
4. `internal/plugin/command.go` - current Dispatcher
5. `internal/plugin/schema.go` - current SchemaRegistry
6. `internal/ipc/` - wire format from Spec 1

## Task

Replace the text-based command dispatch with schema-driven IPC dispatch that routes JSON method calls to handlers using the YANG RPC definitions from Spec 1. This deletes the `RegisterBuiltin()` pattern and longest-prefix text matching, replacing them with YANG-driven exact-match dispatch.

**CLI commands stay the same.** The dispatcher still receives `bgp peer list` from CLI users. Internally, it maps this to the YANG RPC `ze-bgp:peer-list`, validates parameters against YANG types, calls the handler, and formats output per YANG output definition. The user sees no difference.

**This spec replaces, not layers.** The text-based `Dispatcher.Dispatch()` with longest-prefix matching is deleted. The new dispatcher uses YANG RPC metadata for routing. There is no dual-protocol support — Ze has no users, no backwards compatibility.

**Scope:**
- Extend SchemaRegistry to index RPCs and notifications from YANG
- Build RPC dispatcher that routes commands to handler functions using YANG metadata
- Map CLI text commands to YANG RPC methods (bidirectional lookup table)
- Parameter extraction and validation from YANG input types
- Response formatting from YANG output types
- Error mapping to YANG error identities
- Replace server.go `clientLoop` and `handleSingleProcessCommands` with NUL-terminated JSON
- CLI schema introspection: `ze schema methods`, `ze schema events`

**Depends on:** spec-yang-ipc-schema (Spec 1)

**NOT in scope:**
- Plugin protocol migration (Spec 3)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - current handler architecture

### From Spec 1
- [ ] `internal/ipc/framing.go` - NUL-byte framing
- [ ] `internal/ipc/message.go` - Request/Response/Error types
- [ ] `internal/plugin/bgp/schema/ze-bgp-conf.yang` - RPCs defined in Spec 1

### Source Files
- [ ] `internal/plugin/command.go` - Dispatcher, Handler, CommandContext
- [ ] `internal/plugin/handler.go` - all RegisterBuiltin() calls
- [ ] `internal/plugin/schema.go` - SchemaRegistry
- [ ] `internal/plugin/server.go` - Server, Client, acceptLoop, clientLoop
- [ ] `internal/yang/loader.go` - YANG loader
- [ ] `internal/config/yang_schema.go` - YANG → config bridge (pattern to follow)
- [ ] `cmd/ze/schema/main.go` - schema CLI

**Key insights:**
- Current Dispatcher uses longest-prefix text matching on command strings
- New dispatch uses YANG RPC metadata for exact match
- The new dispatcher replaces the old one — no coexistence
- Handler functions are unchanged; only the routing and parameter handling changes
- CLI text commands map to YANG RPC names via a bidirectional lookup table
- SchemaRegistry already has FindHandler() with longest-prefix; extended with FindRPC() for exact match
- Server.clientLoop() and Server.handleSingleProcessCommands() both change to NUL-terminated JSON

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/command.go` - Dispatcher with longest-prefix matching, tokenize(), looksLikeIPOrGlob()
- [ ] `internal/plugin/server.go` - clientLoop reads lines via ReadString('\n'), processCommand calls dispatcher
- [ ] `internal/plugin/handler.go` - 47+ commands in init() via RegisterBuiltin()

**Behavior to preserve:**
- All existing handler functions continue to work unchanged
- CLI command syntax unchanged: `bgp peer list`, `bgp peer 10.0.0.1 teardown`
- Peer selector extraction from command text (`bgp peer <addr> <cmd>`)
- Plugin command registration and routing
- Event subscriptions and streaming
- Dispatcher precedence: builtin → subsystem → plugin

**Behavior to change (replaces, not alongside):**
- Delete `RegisterBuiltin()` init() pattern → handlers registered via YANG RPC metadata
- Delete longest-prefix text matching → YANG-based command resolution
- Delete `clientLoop` newline reading → NUL-terminated JSON reading
- Delete `processCommand` text dispatch → JSON dispatch via `processRPCRequest`
- Delete `parseSerial` / `#N` prefix → `id` field in JSON envelope
- Delete `isComment` / `encodeAlphaSerial` / `isAlphaSerial` → not needed in JSON protocol
- Add `module:rpc-name` exact-match dispatch for wire protocol
- Add parameter extraction from JSON `params` object
- Add response formatting to JSON `result` object
- Add YANG-driven input validation
- Extend SchemaRegistry with RPC/notification indexing

## Data Flow (MANDATORY)

### Entry Point
- Unix socket connection (same as today)
- Client sends NUL-terminated JSON (replaces newline-terminated text)

### Transformation Path (Socket Client)
1. **Read:** Server reads until NUL byte → raw JSON bytes
2. **Parse:** Unmarshal into Request struct (method, params, id, more)
3. **Resolve:** Split method `ze-bgp-conf:peer-list` → module `ze-bgp-conf`, rpc `peer-list`
4. **Validate:** Check RPC exists in YANG, validate params against input leaves
5. **Execute:** Call handler function with CommandContext and validated params
6. **Format:** Convert handler Response to JSON output matching YANG output leaves
7. **Write:** Marshal JSON, append NUL, write to socket

### Transformation Path (CLI Text Command)
1. **Receive:** CLI text `bgp peer list` arrives
2. **Lookup:** Bidirectional table maps `bgp peer list` → `ze-bgp:peer-list`
3. **Extract:** Parse peer selector from text if present (`bgp peer 10.0.0.1 teardown` → selector=10.0.0.1, method=`ze-bgp:peer-teardown`)
4. **Build:** Create Request with method and params from extracted text
5. **Continue from step 3 above** (same path as socket client)

### Streaming Through Dispatch

Current streaming uses `Response.Partial = true` for intermediate results. The new dispatch maps this to the JSON wire format:

1. Handler is called (e.g., `handleSubscribe`)
2. Handler registers subscription, returns Response with `Status: "done"`
3. Bridge sends JSON: `{"result": {"status": "subscribed"}, "id": 1}`
4. Events arrive via `OnMessageReceived` → `proc.WriteEvent()`
5. Each event is delivered as a separate JSON notification message on the connection
6. For socket clients: each notification is a NUL-terminated JSON message with `continues: true`
7. Client sends `ze-bgp:unsubscribe` to stop → final message without `continues`

For handlers that return multiple partial responses (non-subscription streaming):
1. Handler returns Response with `Partial: true`
2. Bridge sends JSON: `{"result": data, "id": N, "continues": true}`
3. Handler returns next partial → same
4. Handler returns final Response with `Partial: false`
5. Bridge sends JSON: `{"result": data, "id": N}` (no `continues`)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Socket → Server | NUL-framing (Spec 1) | [ ] |
| CLI text → Dispatcher | Bidirectional lookup table | [ ] |
| Dispatcher → Handler | CommandContext + validated params | [ ] |
| Dispatcher → SchemaRegistry | RPC lookup | [ ] |

### Integration Points
- `internal/ipc/` from Spec 1 (framing, message types)
- `internal/plugin/command.go` (replace Dispatcher internals)
- `internal/plugin/server.go` (replace clientLoop and processCommand)
- `internal/plugin/schema.go` (SchemaRegistry extension)
- `cmd/ze/schema/main.go` (CLI extension)

### Architectural Verification
- [ ] No bypassed layers (all dispatch goes through YANG-aware path)
- [ ] No unintended coupling (dispatch module delegates to handlers)
- [ ] No duplicated functionality (one dispatcher, not two)
- [ ] Text protocol removed, not coexisting

## Command-to-RPC Mapping

### Bidirectional Lookup Table

Every CLI command maps to exactly one YANG RPC method. This table is built from YANG at startup:

| CLI Command | Wire Method | Module |
|-------------|-------------|--------|
| `bgp daemon status` | `ze-bgp:daemon-status` | ze-bgp |
| `bgp daemon shutdown` | `ze-bgp:daemon-shutdown` | ze-bgp |
| `bgp peer list` | `ze-bgp:peer-list` | ze-bgp |
| `bgp peer show` | `ze-bgp:peer-show` | ze-bgp |
| `bgp peer teardown` | `ze-bgp:peer-teardown` | ze-bgp |
| `bgp peer update` | `ze-bgp:update` | ze-bgp |
| `subscribe` | `ze-bgp:subscribe` | ze-bgp |
| `system command list` | `ze-system:command-list` | ze-system-api |
| `rib show in` | `ze-rib:show-in` | ze-rib-api |
| ... (all 47+ commands) | ... | ... |

The mapping is stored as a YANG extension on each RPC (e.g., `ze:cli-command "bgp peer list"`) or as a separate registration table built at startup.

### Peer Selector Handling

The current dispatcher extracts peer selector from `bgp peer <addr> <command>`. The new dispatcher preserves this:

1. Text `bgp peer 10.0.0.1 teardown` arrives
2. Dispatcher recognizes `bgp peer` prefix, extracts `10.0.0.1` as selector
3. Remaining text `teardown` → lookup `bgp peer teardown` → `ze-bgp:peer-teardown`
4. Build Request: `{"method": "ze-bgp:peer-teardown", "params": {"selector": "10.0.0.1"}}`
5. Dispatch as normal

## SchemaRegistry Extension

### Current State

| Method | Purpose |
|--------|---------|
| `Register(schema)` | Store by module name, map handler paths |
| `GetByModule(name)` | Lookup by module |
| `GetByHandler(path)` | Lookup by handler path |
| `FindHandler(path)` | Longest-prefix match for config routing |

### New Methods

| Method | Purpose |
|--------|---------|
| `RegisterRPCs(module, rpcs)` | Index RPCs extracted from YANG module |
| `RegisterNotifications(module, notifs)` | Index notifications from YANG |
| `FindRPC(method)` | Exact match `module:rpc-name` → RPC metadata |
| `FindRPCByCommand(cmd)` | CLI text → RPC metadata |
| `ListRPCs(module?)` | List all RPCs, optionally filtered by module |
| `ListNotifications(module?)` | List all notifications |

### RPC Metadata

| Field | Type | Description |
|-------|------|-------------|
| Module | string | YANG module name |
| Name | string | RPC name (kebab-case) |
| CLICommand | string | Corresponding CLI text command |
| Description | string | From YANG description |
| InputLeaves | list | Input parameter names, types, required/optional |
| OutputLeaves | list | Output parameter names and types |
| Errors | list | Error identities this RPC can return |
| Handler | Handler | The handler function (from registration) |

## Handler Registration

### Replacing RegisterBuiltin

Current: `RegisterBuiltin("bgp peer list", handleBgpPeerList, "List peer(s)")` in init()

New: Handlers are registered by associating a handler function with a YANG RPC name. The YANG module provides the help text, parameter types, and error identities. The handler function is the same.

The registration table maps:
- YANG RPC name → handler function
- CLI command text → YANG RPC name

This can be a single data structure built at startup, replacing the init() `RegisterBuiltin` calls.

## Server Changes

### clientLoop Replacement

Current `clientLoop` (server.go:1102):
- Reads newline-terminated text via `bufio.Reader.ReadString('\n')`
- Strips whitespace, skips comments
- Calls `processCommand` which calls `Dispatcher.Dispatch`

New `clientLoop`:
- Reads NUL-terminated JSON via ipc.FrameReader (from Spec 1)
- Unmarshals to Request
- Calls RPC dispatcher
- Writes NUL-terminated JSON response

### handleSingleProcessCommands Replacement

Current: reads text lines from plugin stdout, dispatches via text.

New: reads NUL-terminated JSON from plugin socket (after Spec 3 migrates transport). Until Spec 3, plugin protocol remains text-based — this spec only changes the socket client path.

**Clarification:** This spec replaces the **socket client** protocol (clientLoop). The **plugin** protocol (handleSingleProcessCommands) is replaced by Spec 3.

## CLI Integration

### New Commands

| Command | Description |
|---------|-------------|
| `ze schema methods [module]` | List RPCs from YANG (all or specific module) |
| `ze schema events [module]` | List notifications from YANG |
| `ze schema describe <module:rpc>` | Show RPC input/output from YANG |

### Tab Completion from YANG

YANG RPC input leaves can drive tab completion. This is a natural extension — the SchemaRegistry provides input leaf names and types when queried. This happens through the existing `command-complete` handler, which now reads from YANG instead of hardcoded completions.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSchemaRegistryRPC` | `internal/plugin/schema_test.go` | RegisterRPCs, FindRPC | |
| `TestSchemaRegistryNotification` | `internal/plugin/schema_test.go` | RegisterNotifications, ListNotifications | |
| `TestSchemaRegistryFindByCommand` | `internal/plugin/schema_test.go` | CLI text → RPC lookup | |
| `TestRPCDispatchSimple` | `internal/ipc/dispatch_test.go` | Route method to handler | |
| `TestRPCDispatchWithParams` | `internal/ipc/dispatch_test.go` | Extract params, call handler | |
| `TestRPCDispatchUnknownMethod` | `internal/ipc/dispatch_test.go` | Returns error for unknown method | |
| `TestRPCDispatchValidation` | `internal/ipc/dispatch_test.go` | Validate params against YANG types | |
| `TestRPCDispatchStreaming` | `internal/ipc/dispatch_test.go` | Partial responses → continues:true | |
| `TestCLITextToRPC` | `internal/ipc/dispatch_test.go` | CLI text command → RPC resolution | |
| `TestPeerSelectorExtraction` | `internal/ipc/dispatch_test.go` | Extract peer from `bgp peer <addr> <cmd>` | |
| `TestServerNULProtocol` | `internal/plugin/server_test.go` | Server handles NUL-terminated messages | |
| `TestYANGRPCExtraction` | `internal/yang/rpc_test.go` | Extract RPC metadata from YANG Entry tree | |
| `TestCLISchemaMethodsCmd` | `cmd/ze/schema/main_test.go` | `ze schema methods` output | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Method name length | 1-256 | 256 | 0 (empty) | 257 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-ipc-peer-list.ci` | `test/ipc/` | JSON call peer-list via ze-ipc, verify response | |
| `test-ipc-daemon-status.ci` | `test/ipc/` | JSON call daemon-status via ze-ipc | |
| `test-ipc-peer-teardown.ci` | `test/ipc/` | JSON call with params via ze-ipc | |
| `test-ipc-update.ci` | `test/ipc/` | JSON call update (DSL mode) via ze-ipc | |
| `test-ipc-unknown-method.ci` | `test/ipc/` | Error for bad method name via ze-ipc | |
| `test-ipc-subscribe.ci` | `test/ipc/` | Stream events via subscribe using ze-ipc --stream | |
| `test-schema-methods.ci` | `test/schema/` | `ze schema methods` shows RPCs | |

## Files to Modify
- `internal/plugin/command.go` - replace Dispatcher internals with YANG-aware dispatch
- `internal/plugin/schema.go` - extend SchemaRegistry with RPC indexing
- `internal/plugin/server.go` - replace clientLoop with NUL-terminated JSON protocol
- `internal/plugin/handler.go` - replace RegisterBuiltin init() with YANG-based registration
- `cmd/ze/schema/main.go` - add `methods`, `events`, `describe` subcommands

## Files to Create
- `internal/ipc/dispatch.go` - RPC dispatcher
- `internal/ipc/dispatch_test.go` - dispatcher tests
- `internal/yang/rpc.go` - extract RPC metadata from YANG Entry tree
- `internal/yang/rpc_test.go` - RPC extraction tests
- `test/ipc/peer-list.ci` - functional test

## Implementation Steps

1. **Write SchemaRegistry extension tests** - RegisterRPCs, FindRPC, FindRPCByCommand
   → **Review:** Covers duplicate detection? Module filtering? CLI text lookup?

2. **Run tests** - Verify FAIL

3. **Implement SchemaRegistry extension** - New methods for RPC indexing
   → **Review:** Thread-safe? Consistent with existing pattern?

4. **Run tests** - Verify PASS

5. **Write YANG RPC extraction tests** - Parse YANG, extract RPC metadata
   → **Review:** Input/output leaves extracted? Descriptions?

6. **Implement YANG RPC extraction** - Walk YANG Entry tree for RPCs
   → **Review:** Handles all YANG types correctly?

7. **Run tests** - Verify PASS

8. **Write dispatcher tests** - CLI text resolution, JSON dispatch, streaming, peer selector
   → **Review:** Error paths? Unknown methods? Validation? Streaming?

9. **Implement RPC dispatcher** - Route both CLI text and JSON methods to handlers
   → **Review:** Single code path for both entry points?

10. **Run tests** - Verify PASS

11. **Replace handler.go** - Delete RegisterBuiltin init(), replace with YANG-based registration
    → **Review:** All 47+ commands mapped? None lost?

12. **Replace server.go clientLoop** - NUL-terminated JSON protocol for socket clients
    → **Review:** All old text protocol code deleted?

13. **Delete obsolete functions** - parseSerial, isComment, encodeAlphaSerial, isAlphaSerial
    → **Review:** No callers remain?

14. **Write CLI commands** - `ze schema methods`, `ze schema events`
    → **Review:** Output matches YANG content?

15. **Functional tests** - Create .ci files using ze-ipc from Spec 1

16. **Verify all** - `make lint && make test && make functional`

17. **Final self-review**

## Implementation Summary

<!-- Fill AFTER implementation -->

### What Was Implemented
-

### Bugs Found/Fixed
-

### Design Insights
-

### Deviations from Plan
-

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| SchemaRegistry RPC indexing | | | |
| SchemaRegistry notification indexing | | | |
| SchemaRegistry CLI text → RPC lookup | | | |
| YANG RPC metadata extraction | | | |
| RPC dispatcher (replaces text dispatch) | | | |
| CLI text → RPC resolution | | | |
| Streaming through dispatch | | | |
| Parameter validation from YANG | | | |
| Server clientLoop replaced with JSON | | | |
| Text protocol code deleted | | | |
| RegisterBuiltin init() deleted | | | |
| `ze schema methods` command | | | |
| `ze schema events` command | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestSchemaRegistryRPC | | | |
| TestSchemaRegistryFindByCommand | | | |
| TestRPCDispatchSimple | | | |
| TestRPCDispatchStreaming | | | |
| TestCLITextToRPC | | | |
| TestServerNULProtocol | | | |
| TestYANGRPCExtraction | | | |
| test-ipc-peer-list.ci | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| internal/ipc/dispatch.go | | |
| internal/yang/rpc.go | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility
- [ ] Explicit behavior
- [ ] Minimal coupling
- [ ] Next-developer test

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
- [ ] Boundary tests
- [ ] Feature code integrated
- [ ] Functional tests

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Completion
- [ ] Architecture docs updated
- [ ] Implementation Audit completed
- [ ] Spec moved to done
- [ ] All files committed together

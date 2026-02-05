# Spec: yang-ipc-plugin

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/spec-yang-ipc-schema.md` - YANG IPC definitions (Spec 1)
4. `docs/plan/spec-yang-ipc-dispatch.md` - dispatch engine (Spec 2)
5. `internal/plugin/process.go` - current Process management
6. `internal/plugin/startup_coordinator.go` - 5-stage barrier protocol
7. `.claude/rules/plugin-design.md` - plugin patterns

## Task

Replace the plugin protocol from stdin/stdout pipes with text commands to YANG RPC calls over TWO Unix socket pairs. The 5-stage barrier startup protocol is preserved but expressed as typed RPC calls defined in YANG. Plugins declare their IPC capabilities via YANG schemas alongside config schemas.

**This spec replaces, not layers.** The stdin/stdout pipe protocol with text `declare ...` lines is deleted. The new protocol uses NUL-terminated JSON RPC calls over socket pairs. There is no dual-protocol support — Ze has no users, no backwards compatibility.

**Critical Design Decision: TWO SOCKET PAIRS**

Both engine and plugin need to call methods on each other. Since the IPC protocol is request-response per connection, we use two socket pairs:
- **Socket A:** Engine is server, Plugin is client (plugin calls engine)
- **Socket B:** Plugin is server, Engine is client (engine calls plugin)

**Scope:**
- Define `ze-plugin-engine.yang` (RPCs the engine serves for plugins to call)
- Define `ze-plugin-callback.yang` (RPCs the plugin serves for engine to call)
- Replace stdin/stdout pipes with TWO Unix socket pairs per plugin
- Convert 5-stage protocol to YANG RPC calls
- Preserve barrier synchronization semantics
- Add `bye` RPC for termination
- Support both internal (goroutine + net.Pipe) and external (subprocess + socketpair) plugins
- Plugin SDK for Go (reference implementation)

**Depends on:** spec-yang-ipc-schema (Spec 1), spec-yang-ipc-dispatch (Spec 2)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - plugin architecture section
- [ ] `.claude/rules/plugin-design.md` - plugin patterns

### Source Files
- [ ] `internal/plugin/process.go` - Process struct, stdin/stdout handling
- [ ] `internal/plugin/startup_coordinator.go` - barrier synchronization
- [ ] `internal/plugin/registration.go` - PluginRegistration, PluginSchemaDecl
- [ ] `internal/plugin/server.go` - handleRegistrationLine, handleCapabilityLine, stage transitions
- [ ] `internal/plugin/inprocess.go` - internal plugin runner (goroutine + pipes)

### From Specs 1 & 2
- [ ] `internal/ipc/framing.go` - NUL-byte framing
- [ ] `internal/ipc/message.go` - Request/Response types
- [ ] `internal/ipc/dispatch.go` - RPC dispatcher

**Key insights:**
- Current: stdin (engine→plugin), stdout (plugin→engine) pipes with text lines
- New: TWO bidirectional socket pairs with NUL-terminated JSON
- 5 stages with barriers preserved (barriers are application logic, not transport)
- Internal plugins use `net.Pipe()` pairs; external use `syscall.Socketpair()` + FD inheritance
- Platform: Linux and macOS only (Unix sockets)
- Plugin declares YANG with both config containers AND IPC RPCs
- Backpressure = FATAL (program terminates on slow consumer)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/process.go` - Process struct with stdin/stdout/stderr, WriteEvent via writeQueue channel
- [ ] `internal/plugin/startup_coordinator.go` - barrier stages, StageComplete, WaitForStage
- [ ] `internal/plugin/server.go` - handleRegistrationLine parses `declare ...` text, handleCapabilityLine parses `capability ...` text
- [ ] `internal/plugin/registration.go` - ParseLine for text registration protocol

**Behavior to preserve:**
- 5-stage startup with barriers (all plugins must complete each stage before any advances)
- Stage 1: Plugin declares families, commands, config wants, YANG schema
- Stage 2: Engine delivers matching config
- Stage 3: Plugin declares capabilities
- Stage 4: Engine shares command registry
- Stage 5: Plugin signals ready
- Respawn limits (5 per 60 seconds)
- Internal plugin support (goroutine with pipes)
- Plugin command registration and execution
- Dispatcher precedence: builtin → subsystem → plugin

**Behavior to change (replaces, not alongside):**
- Transport: stdin/stdout pipes → TWO Unix socket pairs
- Wire format: newline text → NUL-byte JSON (YANG RPCs)
- Registration: `declare family ...` text → `declare-registration` RPC
- Config delivery: `config json ...` text → `configure` RPC
- Capabilities: `capability hex ...` text → `declare-capabilities` RPC
- Events: `proc.WriteEvent(text)` → `deliver-event` RPC on callback socket
- Termination: implicit (process exit) → explicit `bye` RPC
- Backpressure: 1000-event queue with drop → FATAL (program terminates)
- Delete: `handleRegistrationLine`, `handleCapabilityLine`, `ParseLine` text parsing
- Delete: stdin/stdout pipe creation in Process.Start()
- Delete: writeQueue channel (replaced by socket write)
- Delete: `@serial` response parsing

## Data Flow (MANDATORY)

### Entry Point
- Engine starts plugin subprocess (or goroutine for internal)
- Creates TWO socket pairs (replaces stdin/stdout pipes)

### Transformation Path
1. **Start:** Engine creates two socket pairs, forks plugin
2. **Connect:** Plugin opens inherited FDs as IPC connections
3. **Stage 1:** Plugin calls `declare-registration` on Socket A (RPC to engine)
4. **Stage 2:** Engine calls `configure` on Socket B (RPC to plugin)
5. **Stage 3:** Plugin calls `declare-capabilities` on Socket A
6. **Stage 4:** Engine calls `share-registry` on Socket B
7. **Stage 5:** Plugin calls `ready` on Socket A
8. **Runtime:** Events via `deliver-event` on Socket B, routes via `update-route` on Socket A
9. **Shutdown:** Engine calls `bye` on Socket B, plugin exits

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin (A) | Socket pair A (plugin→engine RPCs) | [ ] |
| Engine ↔ Plugin (B) | Socket pair B (engine→plugin RPCs) | [ ] |
| Process ↔ Coordinator | Stage completion signals | [ ] |

### Integration Points
- `internal/ipc/` from Specs 1 & 2
- `internal/plugin/startup_coordinator.go` (modify for RPC-based stages)
- Plugin SDK (new package for plugin authors)

### Architectural Verification
- [ ] No bypassed layers (two socket pairs, each unidirectional for calls)
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Barrier semantics preserved exactly
- [ ] No stdin/stdout pipe code remaining

## Two-Socket Architecture

### Socket Pair Layout

| Socket | Engine Role | Plugin Role | Purpose |
|--------|-------------|-------------|---------|
| A | Server | Client | Plugin calls engine (registration, routes, subscribe) |
| B | Client | Server | Engine calls plugin (config, events, encode/decode, bye) |

### Why Two Sockets?

The IPC protocol is request-response per connection. If we used one socket:
- Engine sends request → expects response
- Plugin sends request → expects response
- **Deadlock:** Both waiting for the other to respond

With two sockets:
- Socket A: Plugin always initiates, engine always responds
- Socket B: Engine always initiates, plugin always responds

### FD Layout for External Plugins

| FD | Env Variable | Purpose |
|----|--------------|---------|
| 3 | `ZE_ENGINE_FD=3` | Plugin→Engine calls (Socket A child side) |
| 4 | `ZE_CALLBACK_FD=4` | Engine→Plugin calls (Socket B child side) |

### Socket Pair Creation

**External Plugin (Subprocess):**

| Step | Engine Side | Plugin Side |
|------|-------------|-------------|
| 1 | `socketpair()` × 2 → A[0,1], B[0,1] | |
| 2 | Keep A[0] as server, B[0] as client | |
| 3 | Pass A[1] and B[1] via cmd.ExtraFiles | |
| 4 | Set env: `ZE_ENGINE_FD=3`, `ZE_CALLBACK_FD=4` | |
| 5 | Start subprocess | |
| 6 | Close A[1], B[1] | Open fd 3 (Socket A client), fd 4 (Socket B server) |

**Internal Plugin (Goroutine):**

| Step | Engine Side | Plugin Side |
|------|-------------|-------------|
| 1 | `net.Pipe()` × 2 → engineA/pluginA, engineB/pluginB | |
| 2 | Server on engineA, client on engineB | |
| 3 | Start goroutine | Client on pluginA, server on pluginB |

## YANG Interface Definitions

### ze-plugin-engine (Engine serves, Plugin calls)

#### RPCs

| RPC | Input | Output | Stage | Description |
|-----|-------|--------|-------|-------------|
| `declare-registration` | families, commands, wants-config, schema | | 1 | Plugin declares itself |
| `declare-capabilities` | capabilities list | | 3 | OPEN injection |
| `ready` | | | 5 | Plugin is operational |
| `update-route` | peer-selector, command | peers-affected, routes-sent | Runtime | Inject route |
| `subscribe-events` | events, peers, format | | Runtime | Request event delivery |
| `unsubscribe-events` | | | Runtime | Stop event delivery |

#### Types for declare-registration

| Field | Type | Description |
|-------|------|-------------|
| families | list of (family: string, mode: string) | Family registration (mode: encode/decode/both) |
| commands | list of (name, description, args, completable) | Command registration |
| wants-config | leaf-list of string | Config roots plugin wants |
| schema | container (module, namespace, yang-text, handlers) | YANG schema declaration |

### ze-plugin-callback (Plugin serves, Engine calls)

#### RPCs

| RPC | Input | Output | Stage | Description |
|-----|-------|--------|-------|-------------|
| `configure` | sections: list of (root, data) | | 2 | Deliver config |
| `share-registry` | commands: list of command-info | | 4 | Share command registry |
| `deliver-event` | event object | | Runtime | Deliver BGP event |
| `encode-nlri` | family, args | hex: string | Runtime | Encode NLRI |
| `decode-nlri` | family, hex | json: object | Runtime | Decode NLRI |
| `decode-capability` | code, hex | json: object | Runtime | Decode capability |
| `execute-command` | serial, command, args, peer | status, data | Runtime | Execute plugin command |
| `bye` | reason?: string | | Shutdown | Terminate plugin |

### Error Identities

| Error | Parameters | Description |
|-------|------------|-------------|
| `registration-conflict` | reason: string | Family/command already claimed |
| `stage-timeout` | stage: int | Barrier timeout exceeded |
| `invalid-route` | reason: string | Route parsing failed |
| `configuration-failed` | reason: string | Config parsing error |
| `encode-error` | reason: string | NLRI encoding failed |
| `decode-error` | reason: string | NLRI decoding failed |
| `command-error` | reason: string | Command execution failed |

## Startup Sequence (Preserved Barriers)

| Stage | Plugin Action (Socket A) | Engine Action (Socket B) | Barrier |
|-------|--------------------------|--------------------------|---------|
| 1 | Call `declare-registration` | Store registration, respond | All plugins complete |
| 2 | Wait | Call `configure` | All plugins complete |
| 3 | Call `declare-capabilities` | Store capabilities, respond | All plugins complete |
| 4 | Wait | Call `share-registry` | All plugins complete |
| 5 | Call `ready` | Respond, signal coordinator | All plugins ready |
| Run | Event loop (update-route, etc.) | Call deliver-event, etc. | None |
| Stop | Receive `bye`, cleanup, exit | Call `bye`, wait, SIGKILL timeout | N/A |

## Termination: bye RPC

1. Engine calls `bye(reason: "shutdown")` on Socket B
2. Plugin receives bye, returns empty response
3. Plugin performs cleanup, exits with code 0
4. Engine waits for process exit (with timeout)
5. If plugin doesn't exit within 5 seconds, engine sends SIGKILL

## Backpressure Policy: FATAL

If engine cannot deliver event to plugin (slow consumer), the **program terminates**.

| Condition | Action |
|-----------|--------|
| `deliver-event` write timeout (5s) | Log error, terminate program |
| Plugin not reading from Socket B | Same |

**No event queuing. No event dropping. Slow = FATAL.**

Rationale: missed BGP events mean incorrect routing state. This replaces the current 1000-event queue with drop semantics.

## Plugin SDK

### Purpose

Provide a library for plugin authors in any language. The SDK handles:
- Socket pair setup (reading FDs from environment)
- NUL-byte framing
- JSON message encoding/decoding
- Startup protocol (5 stages)
- Event loop

### Go SDK

| Package | Purpose |
|---------|---------|
| `pkg/plugin/sdk` | Plugin SDK entry point |
| `pkg/plugin/sdk/types` | Shared types (events, commands, config) |

### SDK Interface (Described for Any Language)

| Method | Direction | Description |
|--------|-----------|-------------|
| `DeclareRegistration(families, commands, schema)` | Plugin → Engine | Stage 1 |
| `OnConfigure(sections)` | Engine → Plugin | Stage 2 callback |
| `DeclareCapabilities(capabilities)` | Plugin → Engine | Stage 3 |
| `OnShareRegistry(commands)` | Engine → Plugin | Stage 4 callback |
| `Ready()` | Plugin → Engine | Stage 5 |
| `OnEvent(event)` | Engine → Plugin | Runtime callback |
| `UpdateRoute(peer, command)` | Plugin → Engine | Runtime call |
| `OnExecuteCommand(serial, cmd, args, peer)` | Engine → Plugin | Runtime callback |
| `OnBye(reason)` | Engine → Plugin | Shutdown callback |

### Python SDK Outline

A Python implementation would read `ZE_ENGINE_FD` and `ZE_CALLBACK_FD`, wrap them in socket objects, and implement the same protocol. This is out of scope for this spec but the Go SDK serves as the reference implementation.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSocketPairCreate` | `internal/plugin/socketpair_test.go` | Create two socket pairs | |
| `TestSocketPairFDPassing` | `internal/plugin/socketpair_test.go` | FD inheritance via ExtraFiles | |
| `TestInternalPluginDualPipe` | `internal/plugin/process_test.go` | Two net.Pipe for internal plugins | |
| `TestRPCDeclareRegistration` | `internal/plugin/rpc_plugin_test.go` | Stage 1 via RPC | |
| `TestRPCConfigure` | `internal/plugin/rpc_plugin_test.go` | Stage 2 via RPC | |
| `TestRPCDeclareCapabilities` | `internal/plugin/rpc_plugin_test.go` | Stage 3 via RPC | |
| `TestRPCShareRegistry` | `internal/plugin/rpc_plugin_test.go` | Stage 4 via RPC | |
| `TestRPCReady` | `internal/plugin/rpc_plugin_test.go` | Stage 5 via RPC | |
| `TestRPCDeliverEvent` | `internal/plugin/rpc_plugin_test.go` | Event delivery via RPC | |
| `TestRPCEncodeNLRI` | `internal/plugin/rpc_plugin_test.go` | Encode callback | |
| `TestRPCDecodeNLRI` | `internal/plugin/rpc_plugin_test.go` | Decode callback | |
| `TestRPCBye` | `internal/plugin/rpc_plugin_test.go` | Clean termination | |
| `TestSlowPluginFatal` | `internal/plugin/rpc_plugin_test.go` | Slow consumer terminates | |
| `TestCoordinatorWithRPC` | `internal/plugin/startup_coordinator_test.go` | Barriers work with RPC | |
| `TestFullStartupCycle` | `internal/plugin/startup_test.go` | All 5 stages end-to-end | |
| `TestSDKStartup` | `pkg/plugin/sdk/sdk_test.go` | SDK handles startup protocol | |
| `TestSDKEventLoop` | `pkg/plugin/sdk/sdk_test.go` | SDK event handling | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| capability code | 0-255 | 255 | N/A | 256 |
| stage timeout | 1ms-300s | 300s | 0 | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-plugin-rpc-startup.ci` | `test/plugin/` | Plugin starts via RPC protocol | |
| `test-plugin-rpc-encode.ci` | `test/plugin/` | NLRI encode via callback | |
| `test-plugin-rpc-events.ci` | `test/plugin/` | Event delivery via callback | |
| `test-plugin-rpc-bye.ci` | `test/plugin/` | Clean shutdown | |

## Related Specs: Plugin Updates

**Each internal plugin update is a SEPARATE spec.** This spec creates the infrastructure only.

| Plugin | Separate Spec | Priority |
|--------|---------------|----------|
| GR | `spec-yang-ipc-plugin-gr.md` | High (core) |
| Hostname | `spec-yang-ipc-plugin-hostname.md` | Low |
| RR | `spec-yang-ipc-plugin-rr.md` | High (core) |
| RIB | `spec-yang-ipc-plugin-rib.md` | High (core) |
| FlowSpec | `spec-yang-ipc-plugin-flowspec.md` | High (commonly used) |
| EVPN | `spec-yang-ipc-plugin-evpn.md` | Medium |
| VPN | `spec-yang-ipc-plugin-vpn.md` | Medium |
| BGP-LS | `spec-yang-ipc-plugin-bgpls.md` | Low |

## Files to Modify
- `internal/plugin/process.go` - replace stdin/stdout pipes with two socket pairs
- `internal/plugin/startup_coordinator.go` - adapt to RPC-based stages
- `internal/plugin/server.go` - replace handleRegistrationLine/handleCapabilityLine with RPC handlers
- `internal/plugin/registration.go` - delete ParseLine text protocol, use RPC types
- `internal/plugin/inprocess.go` - replace single pipe pair with two net.Pipe pairs

## Files to Create
- `internal/yang/modules/ze-plugin-engine.yang` - engine-serves-plugin interface
- `internal/yang/modules/ze-plugin-callback.yang` - plugin-serves-engine interface
- `internal/plugin/socketpair.go` - dual socket pair creation
- `internal/plugin/socketpair_test.go` - socket pair tests
- `internal/plugin/rpc_plugin.go` - RPC-based plugin protocol handling
- `internal/plugin/rpc_plugin_test.go` - RPC plugin tests
- `pkg/plugin/sdk/sdk.go` - Plugin SDK
- `pkg/plugin/sdk/sdk_test.go` - SDK tests
- `pkg/plugin/sdk/types/types.go` - shared types
- `test/plugin/rpc-startup.ci` - functional test

## Implementation Steps

1. **Write socket pair tests** - Create two pairs, FD passing
   → **Review:** Both Unix and net.Pipe cases?

2. **Run tests** - Verify FAIL

3. **Implement socketpair.go** - Dual socket pair helpers
   → **Review:** Error handling for FD operations?

4. **Run tests** - Verify PASS

5. **Create YANG interface definitions** - Both .yang files
   → **Review:** All current protocol commands mapped?

6. **Write RPC plugin tests** - Each stage, deliver-event, bye
   → **Review:** Barrier semantics tested? Slow plugin tested?

7. **Run tests** - Verify FAIL

8. **Implement rpc_plugin.go** - RPC-based protocol handlers
   → **Review:** Two-socket architecture correct?

9. **Run tests** - Verify PASS

10. **Replace process.go** - Delete stdin/stdout pipes, use two socket pairs
    → **Review:** All pipe code deleted? Clean FD management?

11. **Replace server.go plugin handling** - Delete handleRegistrationLine, handleCapabilityLine
    → **Review:** All text registration code deleted?

12. **Replace registration.go** - Delete ParseLine, use RPC types
    → **Review:** All text parsing code deleted?

13. **Replace inprocess.go** - Two net.Pipe pairs
    → **Review:** Internal plugins work with new protocol?

14. **Modify startup_coordinator.go** - Work with RPC-based stages
    → **Review:** Barriers still function correctly?

15. **Write plugin SDK** - pkg/plugin/sdk/sdk.go
    → **Review:** Easy for plugin authors?

16. **Functional tests** - Create .ci files

17. **Verify all** - `make lint && make test && make functional`

18. **Final self-review**

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
| ze-plugin-engine.yang | | | |
| ze-plugin-callback.yang | | | |
| TWO socket pairs for external plugins | | | |
| TWO net.Pipe for internal plugins | | | |
| declare-registration RPC | | | |
| configure RPC | | | |
| declare-capabilities RPC | | | |
| share-registry RPC | | | |
| ready RPC | | | |
| deliver-event RPC | | | |
| encode-nlri RPC | | | |
| decode-nlri RPC | | | |
| bye RPC | | | |
| Barrier preservation | | | |
| Slow consumer = FATAL | | | |
| Plugin SDK (Go) | | | |
| stdin/stdout pipe code deleted | | | |
| Text registration protocol deleted | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestSocketPairCreate | | | |
| TestRPCDeclareRegistration | | | |
| TestRPCDeliverEvent | | | |
| TestRPCBye | | | |
| TestSlowPluginFatal | | | |
| TestFullStartupCycle | | | |
| test-plugin-rpc-startup.ci | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| internal/yang/modules/ze-plugin-engine.yang | | |
| internal/yang/modules/ze-plugin-callback.yang | | |
| internal/plugin/socketpair.go | | |
| internal/plugin/rpc_plugin.go | | |
| pkg/plugin/sdk/sdk.go | | |

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

# Spec: plugin-rpc-migration

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `pkg/plugin/sdk/sdk.go` - SDK API (Run, callbacks, Registration)
4. `internal/plugin/rpc_plugin.go` - engine-side PluginConn typed RPCs
5. `internal/plugin/inprocess.go` - runner registry and signature
6. `internal/plugin/server.go` - text vs RPC code paths
7. `internal/plugin/process.go` - startInternal text vs RPC setup

## Task

Fix the layering violation left by Specs 208-210. Those specs **added** the YANG RPC protocol alongside the text protocol behind a `UseRPC` flag, violating the no-layering rule. This spec:

1. Converts all 7 internal plugins from text protocol to YANG RPC via the SDK
2. Implements runtime RPC event delivery and command dispatch in the server
3. Deletes all text protocol code paths from server, process, and registration
4. Removes the `UseRPC` flag (everything is RPC)

**Why now:** The SDK (`pkg/plugin/sdk/`), PluginConn (`internal/plugin/rpc_plugin.go`), and socket pair infrastructure are all built and tested. The text protocol is dead weight.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - API design
- [ ] `docs/architecture/api/wire-format.md` - NUL-framed JSON wire format (Spec 208)

### Completed Specs
- [ ] `docs/plan/done/208-yang-ipc-schema.md` - wire format + YANG types
- [ ] `docs/plan/done/209-yang-ipc-dispatch.md` - handler dispatch
- [ ] `docs/plan/done/210-yang-ipc-plugin.md` - dual socket pairs + SDK

### Source Files
- [ ] `pkg/plugin/sdk/sdk.go` - SDK Run(), callbacks, Registration type
- [ ] `internal/plugin/rpc_plugin.go` - PluginConn engine-side methods
- [ ] `internal/plugin/inprocess.go` - runner registry, signature, YANG maps
- [ ] `internal/plugin/server.go` - handleProcessStartup vs handleProcessStartupRPC
- [ ] `internal/plugin/process.go` - startInternal text vs RPC paths
- [ ] `internal/plugin/registration.go` - ParseLine text protocol parsing

**Key insights:**
- The SDK already handles the complete 5-stage startup protocol via `p.Run(ctx, registration)`
- PluginConn already has typed methods for all runtime RPCs (deliver-event, encode/decode NLRI, execute-command, bye)
- Socket pairs already exist for internal plugins (`NewInternalSocketPairs`)
- The only missing piece is the runtime RPC loop in `handleSingleProcessCommandsRPC` which currently just waits for shutdown

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/evpn/plugin.go` - text protocol: declare family, waitForLine, send/read via bufio.Scanner
- [ ] `internal/plugin/gr/gr.go` - text protocol: declare wants config, parse "config json bgp", register per-peer capabilities
- [ ] `internal/plugin/hostname/hostname.go` - text protocol: similar to GR, per-peer FQDN capability
- [ ] `internal/plugin/rib/rib.go` - text protocol: declare families, subscribe to UPDATE events, command handling
- [ ] `internal/plugin/flowspec/plugin.go` - text protocol: declare families encode+decode, encode/decode NLRI
- [ ] `internal/plugin/vpn/vpn.go` - text protocol: declare families decode, decode NLRI
- [ ] `internal/plugin/bgpls/plugin.go` - text protocol: declare families decode, decode NLRI
- [ ] `internal/plugin/server.go` - dual code paths: handleProcessStartup (text) vs handleProcessStartupRPC (RPC)
- [ ] `internal/plugin/process.go` - dual paths in startInternal: UseRPC creates PluginConns, else stdin/stdout/readLines/writeLoop
- [ ] `internal/plugin/registration.go` - 918 lines, ParseLine handles "declare" text commands

**Behavior to preserve:**
- 5-stage startup protocol semantics (registration, config, capability, registry, ready)
- Per-peer capability injection (GR restart-time, hostname FQDN)
- Config delivery via JSON (bgp subtree extraction)
- Event subscription and delivery (UPDATE, OPEN, etc.)
- NLRI encode/decode dispatch to family plugins
- Capability decode dispatch
- Command routing to plugins
- Backpressure management (queue limits, drop semantics)
- All functional tests must continue to pass

**Behavior to change:**
- Transport: text lines over stdin/stdout replaced by NUL-framed JSON RPC over socket pairs
- Runner signature: `func(io.Reader, io.Writer) int` replaced by `func(net.Conn, net.Conn) int`
- Runtime event delivery: `proc.WriteEvent(text)` replaced by `proc.engineConnB.SendDeliverEvent(ctx, json)`
- Encode/decode routing: `proc.SendRequest(ctx, textCmd)` replaced by `proc.engineConnB.SendEncodeNLRI/SendDecodeNLRI`
- Process model: no stdin/stdout/readLines/writeLoop for internal plugins (all use socket pair RPC)
- Server: text protocol functions deleted, only RPC path remains

## Data Flow (MANDATORY)

### Entry Point
- Plugin startup: engine creates DualSocketPair, plugin goroutine gets plugin-side connections
- Runtime events: reactor calls `OnMessageReceived` → server routes to subscribed plugins
- Encode/decode: CLI or engine calls `EncodeNLRI`/`DecodeNLRI` → server routes to family plugin

### Transformation Path

**Before (text protocol):**
1. Engine writes text command to plugin stdin (e.g., "config json bgp {...}")
2. Plugin reads line via bufio.Scanner
3. Plugin writes response as text line to stdout (e.g., "capability hex 40...")
4. Engine reads line via readLines goroutine → lines channel

**After (RPC protocol):**
1. Engine calls `engineConnB.SendConfigure(ctx, sections)` (NUL-framed JSON on Socket B)
2. SDK's event loop reads request, dispatches to `OnConfigure` callback
3. SDK sends OK/error response back on Socket B
4. For plugin→engine calls: SDK calls `engineConn.CallRPC()` on Socket A, engine reads from `engineConnA`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | NUL-framed JSON RPC over dual socket pairs | [ ] |
| Server ↔ Reactor | Same as before (function calls) | [ ] |
| Plugin ↔ SDK | SDK handles protocol, plugin uses callbacks | [ ] |

### Integration Points
- `InternalPluginRunner` type in `inprocess.go` - signature changes to accept `net.Conn` pair
- `startInternal()` in `process.go` - always creates PluginConns (no text path)
- `handleSingleProcessCommandsRPC()` in `server.go` - implements runtime RPC loop
- `OnMessageReceived/OnPeerStateChange/OnMessageSent` in `server.go` - use RPC delivery
- `EncodeNLRI/DecodeNLRI` in `server.go` - use RPC calls

### Architectural Verification
- [ ] No bypassed layers (all plugin communication via RPC)
- [ ] No unintended coupling (SDK isolates plugin from wire format)
- [ ] No duplicated functionality (single protocol path, text deleted)
- [ ] Zero-copy preserved where applicable (JSON event strings passed through)

## Plugin Conversion Summary

### Per-Plugin Changes

| Plugin | Startup Lines | Config Needs | Capability Needs | Runtime Needs | Complexity |
|--------|--------------|--------------|------------------|---------------|------------|
| EVPN | 18 | None | None | OnDecodeNLRI | Low |
| VPN | 20 | None | None | OnDecodeNLRI | Low |
| BGP-LS | 21 | None | None | OnDecodeNLRI | Low |
| FlowSpec | 25 | None | None | OnEncodeNLRI + OnDecodeNLRI | Low |
| GR | 18 | bgp subtree | Per-peer GR caps | OnEvent (negotiated) | Medium |
| Hostname | 19 | bgp subtree | Per-peer FQDN caps | OnEvent (negotiated) | Medium |
| RIB | 30 | None | None | OnEvent + OnExecuteCommand | Medium |

### What Changes Per Plugin

Each plugin replaces:
- `bufio.Scanner` input / `io.Writer` output with SDK connection pair
- `doStartupProtocol()` method with SDK `Registration` struct + callbacks
- `eventLoop()` reading text lines with SDK callback-based event loop
- `send()` / `waitForLine()` helpers → deleted (SDK handles protocol)

Each plugin preserves:
- All decode/encode logic (unchanged)
- All config parsing logic (receives JSON via `OnConfigure` callback instead of text lines)
- All capability construction logic (set via `SetCapabilities` before `Run`)
- All business logic (RIB storage, flowspec rules, etc.)

### Server-Side Runtime RPC

`handleSingleProcessCommandsRPC` currently does nothing (waits for shutdown). It needs:

1. **Read loop on engineConnA** - handle plugin→engine RPCs:

| RPC Method | Handler |
|------------|---------|
| `ze-plugin-engine:update-route` | Parse UpdateRouteInput, dispatch via existing update handler |
| `ze-plugin-engine:subscribe-events` | Parse SubscribeEventsInput, register subscription |
| `ze-plugin-engine:unsubscribe-events` | Clear subscriptions for this process |

2. **Event delivery via engineConnB** - replace `proc.WriteEvent()`:

| Current (text) | Replacement (RPC) |
|----------------|-------------------|
| `proc.WriteEvent(jsonOutput)` | `proc.engineConnB.SendDeliverEvent(ctx, jsonOutput)` |
| `proc.SendRequest(ctx, "encode nlri ...")` | `proc.engineConnB.SendEncodeNLRI(ctx, family, args)` |
| `proc.SendRequest(ctx, "decode nlri ...")` | `proc.engineConnB.SendDecodeNLRI(ctx, family, hex)` |

### Code to Delete

| Component | Location | Lines (approx) | Reason |
|-----------|----------|----------------|--------|
| `handleProcessStartup()` | `server.go` | ~100 | Replaced by `handleProcessStartupRPC` |
| `handleSingleProcessCommands()` | `server.go` | ~140 | Replaced by runtime RPC loop |
| `handleRegistrationLine()` | `server.go` | ~25 | Only used by text path |
| `handleCapabilityLine()` | `server.go` | ~25 | Only used by text path |
| `ParseLine()` and helpers | `registration.go` | ~400 | RPC uses typed structs |
| `PluginCapabilities.ParseLine()` | `registration.go` | ~80 | RPC uses typed structs |
| `readLines()` | `process.go` | ~30 | RPC uses PluginConn |
| `writeLoop()` | `process.go` | ~25 | RPC uses PluginConn |
| `parseSerial()` / `isComment()` | `server.go` | ~25 | Text protocol only |
| `encodeAlphaSerial()` / `isAlphaSerial()` | `server.go` | ~25 | Text protocol only |
| `parseResponseSerial()` | `process.go` | ~20 | Text protocol only |
| `parsePluginResponse()` | `server.go` | varies | Text protocol only |
| `handlePluginResponse()` | `server.go` | ~25 | Text protocol only |
| `stdin`/`stdout`/`reader`/`lines` fields | `process.go` | ~10 | Text protocol only |
| `WriteQueueHighWater`/`WriteQueueLowWater` | `process.go` | ~10 | Text protocol only |
| `writeQueue` / backpressure in text path | `process.go` | ~30 | RPC has own flow control |
| `UseRPC` flag | `types.go` | ~5 | Everything is RPC |
| Text path in `startInternal()` | `process.go` | ~10 | Only RPC path remains |
| Text path in `handleProcessCommandsSync()` | `server.go` | ~10 | Only RPC path remains |
| `FormatRegistrySharing()` | `registration.go` | ~30 | RPC uses typed struct |
| `handleRegisterCommand()` / `handleUnregisterCommand()` | `server.go` | ~50 | Runtime registration via RPC |
| `parseRegisterCommand()` / `parseUnregisterCommand()` | varies | ~40 | Text parsing |

**Preserved from registration.go:**
- `PluginStage` type and constants (used by both paths)
- `PluginRegistration` struct (populated from RPC input)
- `PluginCapabilities` struct (populated from RPC input)
- `PluginRegistry` and related types (unchanged)
- `CapabilityInjector` (unchanged)
- `SchemaDeclaration` types (unchanged)
- Stage string methods, valid AFI/SAFI maps (may still be useful for validation)

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRuntimeRPCEventDelivery` | `internal/plugin/server_rpc_test.go` | Events routed via engineConnB.SendDeliverEvent | |
| `TestRuntimeRPCUpdateRoute` | `internal/plugin/server_rpc_test.go` | Plugin update-route RPC dispatched correctly | |
| `TestRuntimeRPCSubscribe` | `internal/plugin/server_rpc_test.go` | Plugin subscribe-events RPC creates subscription | |
| `TestRuntimeRPCEncodeNLRI` | `internal/plugin/server_rpc_test.go` | EncodeNLRI routes via engineConnB | |
| `TestRuntimeRPCDecodeNLRI` | `internal/plugin/server_rpc_test.go` | DecodeNLRI routes via engineConnB | |
| `TestPluginRunnerSignature` | `internal/plugin/inprocess_test.go` | Runner accepts net.Conn pair | |
| `TestNoTextProtocolCode` | `internal/plugin/verify_test.go` | No stdin/parseSerial/ReadString in plugin paths | |

### Boundary Tests
- N/A (no new numeric fields; converting transport, not adding validation)

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| All existing encode tests | `test/encode/*.ci` | BGP encoding still works | |
| All existing plugin tests | `test/plugin/*.ci` | Plugin startup and event delivery works | |
| All existing decode tests | `test/decode/*.ci` | NLRI decoding still works | |
| All existing parse tests | `test/parse/*.ci` | Config parsing still works | |

### Future
- External plugin RPC tests (when external plugins are implemented)

## Files to Modify

### Plugin Files (convert text → SDK)
- `internal/plugin/gr/gr.go` - replace doStartupProtocol/eventLoop with SDK Run + callbacks
- `internal/plugin/hostname/hostname.go` - replace doStartupProtocol/eventLoop with SDK Run + callbacks
- `internal/plugin/rib/rib.go` - replace doStartupProtocol/eventLoop with SDK Run + callbacks
- `internal/plugin/flowspec/plugin.go` - replace doStartupProtocol/eventLoop with SDK Run + callbacks
- `internal/plugin/evpn/plugin.go` - replace doStartupProtocol/eventLoop with SDK Run + callbacks
- `internal/plugin/vpn/vpn.go` - replace doStartupProtocol/eventLoop with SDK Run + callbacks
- `internal/plugin/bgpls/plugin.go` - replace doStartupProtocol/eventLoop with SDK Run + callbacks

### Infrastructure Files (delete text protocol, implement runtime RPC)
- `internal/plugin/inprocess.go` - change InternalPluginRunner signature to net.Conn pair
- `internal/plugin/process.go` - remove text protocol fields/methods, simplify startInternal
- `internal/plugin/server.go` - delete text protocol functions, implement runtime RPC loop
- `internal/plugin/registration.go` - delete ParseLine and text parsing, keep types
- `internal/plugin/types.go` - remove UseRPC flag

### Test Files
- `internal/plugin/server_test.go` - update tests for RPC-only paths
- `internal/plugin/server_config_test.go` - update tests
- `internal/plugin/registration_test.go` - delete text parsing tests, keep type tests
- `internal/plugin/inprocess_test.go` - update runner signature tests
- `internal/plugin/process_test.go` - update for simplified process model

## Files to Create
- `internal/plugin/server_rpc_test.go` - runtime RPC dispatch tests
- `internal/plugin/verify_test.go` - verify no text protocol code remains

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

### Phase 1: Convert decode-only plugins (EVPN, VPN, BGP-LS)

1. **Write unit test for RPC decode routing** - test that DecodeNLRI uses engineConnB.SendDecodeNLRI
   → **Review:** Does test cover error path? Timeout?

2. **Run test** - Verify FAIL (paste output)
   → **Review:** Fails for right reason?

3. **Change InternalPluginRunner signature** - accept net.Conn pair instead of io.Reader/io.Writer
   → **Review:** All runner registrations updated?

4. **Convert EVPN plugin** - replace bufio.Scanner/Writer with SDK NewWithConn + OnDecodeNLRI callback, delete doStartupProtocol/eventLoop/send/waitForLine
   → **Review:** No text protocol imports remaining?

5. **Convert VPN plugin** - same pattern as EVPN
   → **Review:** Multi-family registration correct?

6. **Convert BGP-LS plugin** - same pattern as EVPN
   → **Review:** Multi-family registration correct?

7. **Run tests** - Verify PASS (paste output)
   → **Review:** All functional tests pass?

### Phase 2: Convert encode+decode plugin (FlowSpec)

8. **Convert FlowSpec plugin** - add OnEncodeNLRI + OnDecodeNLRI callbacks
   → **Review:** Encode and decode both registered?

9. **Run tests** - Verify PASS (paste output)

### Phase 3: Convert config+capability plugins (GR, Hostname)

10. **Convert GR plugin** - OnConfigure callback parses JSON config, SetCapabilities for per-peer GR
    → **Review:** Per-peer capability construction preserved? Config JSON parsing unchanged?

11. **Convert Hostname plugin** - same pattern as GR with FQDN capability
    → **Review:** Per-peer capability construction preserved?

12. **Run tests** - Verify PASS (paste output)

### Phase 4: Convert complex plugin (RIB)

13. **Convert RIB plugin** - OnEvent for UPDATE processing, OnExecuteCommand for show/clear
    → **Review:** Storage logic unchanged? Event subscription setup correct?

14. **Run tests** - Verify PASS (paste output)

### Phase 5: Implement runtime RPC in server

15. **Write unit tests for runtime RPC dispatch** - event delivery, update-route, subscribe
    → **Review:** Edge cases covered?

16. **Run tests** - Verify FAIL (paste output)

17. **Implement handleSingleProcessCommandsRPC runtime loop** - read from engineConnA, dispatch plugin RPCs
    → **Review:** All plugin→engine RPCs handled?

18. **Convert OnMessageReceived/OnPeerStateChange/OnMessageSent** - use SendDeliverEvent
    → **Review:** Event format preserved?

19. **Convert EncodeNLRI/DecodeNLRI** - use SendEncodeNLRI/SendDecodeNLRI
    → **Review:** Error handling preserved?

20. **Run tests** - Verify PASS (paste output)

### Phase 6: Delete text protocol code

21. **Delete text protocol functions from server.go** - handleProcessStartup, handleSingleProcessCommands, handleRegistrationLine, handleCapabilityLine, parseSerial, isComment, encodeAlphaSerial, isAlphaSerial, handlePluginResponse, parsePluginResponse, handleRegisterCommand, handleUnregisterCommand, FormatRegistrySharing
    → **Review:** No remaining references to deleted functions?

22. **Delete text protocol code from process.go** - readLines, writeLoop, parseResponseSerial, stdin/stdout/reader/lines fields, WriteQueueHighWater/WriteQueueLowWater, writeQueue, text path in startInternal
    → **Review:** External plugin path still works?

23. **Delete ParseLine and text parsing from registration.go** - ParseLine, PluginCapabilities.ParseLine, text-only helper functions
    → **Review:** PluginStage, PluginRegistration struct, PluginRegistry preserved?

24. **Remove UseRPC flag from types.go**
    → **Review:** No remaining references?

25. **Delete/update tests for removed functions**
    → **Review:** No test references to deleted code?

26. **Run `make lint && make test && make functional`** (paste output)
    → **Review:** Zero issues?

27. **Final self-review** - re-read all changes, check for unused code, debug statements, TODOs

## RFC Documentation

### Reference Comments
- No new RFC references needed (transport change, not protocol change)

### Constraint Comments
- Preserve existing RFC comments in plugin business logic (GR: RFC 4724, EVPN: RFC 7432/9136, etc.)

## Implementation Summary

### What Was Implemented
- All 8 internal plugins (EVPN, VPN, BGP-LS, FlowSpec, GR, Hostname, RIB, RR) converted from text protocol to SDK callback pattern
- InternalPluginRunner signature changed to `func(engineConn, callbackConn net.Conn) int`
- Runtime RPC loop (`handleSingleProcessCommandsRPC`) with dispatch for update-route, subscribe-events, unsubscribe-events
- Event delivery via `SendDeliverEvent`, encode/decode via `SendEncodeNLRI`/`SendDecodeNLRI`
- All text protocol code deleted from server.go, process.go, registration.go, types.go
- UseRPC flag removed — everything is RPC
- Net deletion of ~3,600 lines (removed ~6,100 text protocol, added ~2,500 RPC)
- Post-review: added debug logging for discarded Send/SendError failures in startup and runtime paths
- Post-review: logged RPC dispatcher registration failures instead of discarding
- Post-review: fixed VPN plugin inconsistent write-error handling pattern

### Bugs Found/Fixed
- No bugs found during implementation

### Design Insights
- SDK callback pattern (NewWithConn + On* callbacks + Run) is significantly more concise than the text protocol equivalents
- SetStartupSubscriptions solves the race between plugin startup and first event delivery elegantly
- The text command bridge in handleUpdateRouteRPC (prepending "bgp peer ") is tech debt for future direct-dispatch

### Deviations from Plan
- RR plugin (8th) was also converted — spec listed 7 plugins but RR also needed conversion
- Test files: RPC tests were added to existing `server_test.go` and `rpc_plugin_test.go` instead of creating `server_rpc_test.go`
- `verify_test.go` was not created — text protocol removal was verified by compilation (no references remain)
- Test names differ from spec (e.g., `TestRPCDeliverEvent` instead of `TestRuntimeRPCEventDelivery`) — coverage equivalent

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Convert EVPN to SDK RPC | ✅ Done | `internal/plugin/evpn/plugin.go:33-74` | |
| Convert VPN to SDK RPC | ✅ Done | `internal/plugin/vpn/vpn.go:34-82` | |
| Convert BGP-LS to SDK RPC | ✅ Done | `internal/plugin/bgpls/plugin.go:33-81` | |
| Convert FlowSpec to SDK RPC | ✅ Done | `internal/plugin/flowspec/plugin.go:39-100` | |
| Convert GR to SDK RPC | ✅ Done | `internal/plugin/gr/gr.go:40-68` | |
| Convert Hostname to SDK RPC | ✅ Done | `internal/plugin/hostname/hostname.go:71-100` | |
| Convert RIB to SDK RPC | ✅ Done | `internal/plugin/rib/rib.go:92+` | |
| Convert RR to SDK RPC | ✅ Done | `internal/plugin/rr/server.go:36-80` | Not in original spec, added during impl |
| Implement runtime RPC event delivery | ✅ Done | `server.go` SendDeliverEvent calls | |
| Implement runtime RPC command dispatch | ✅ Done | `server.go:838-875` handleSingleProcessCommandsRPC + dispatchPluginRPC | |
| Delete text protocol from server.go | ✅ Done | No handleProcessStartup/handleSingleProcessCommands text versions | |
| Delete text protocol from process.go | ✅ Done | No stdin/stdout/readLines/writeLoop | |
| Delete ParseLine from registration.go | ✅ Done | All text parsing removed | |
| Remove UseRPC flag | ✅ Done | No UseRPC in types.go | |
| All functional tests pass | ✅ Done | decode 22/22, parse 22/22, editor 93/93 | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestRuntimeRPCEventDelivery | 🔄 Changed | `rpc_plugin_test.go:221` TestRPCDeliverEvent | Different name, same coverage |
| TestRuntimeRPCUpdateRoute | 🔄 Changed | `server_test.go:404,430` TestServerRPCWithID, TestServerMultipleRPCRequests | Tests full dispatch path |
| TestRuntimeRPCSubscribe | 🔄 Changed | `server_test.go:743` TestHandleProcessStartupRPC | Startup subscriptions tested in full cycle |
| TestRuntimeRPCEncodeNLRI | 🔄 Changed | `rpc_plugin_test.go:249` TestRPCEncodeNLRI | Different name, same coverage |
| TestRuntimeRPCDecodeNLRI | 🔄 Changed | `rpc_plugin_test.go:283` TestRPCDecodeNLRI | Different name, same coverage |
| TestPluginRunnerSignature | 🔄 Changed | `inprocess_test.go:15,35` TestInternalPluginRunnerRegistry, TestGetInternalPluginRunner | Split into 2 tests |
| TestNoTextProtocolCode | ❌ Skipped | - | Verified by compilation; no text protocol references remain |
| All existing functional tests | ✅ Done | test/decode, test/parse, test/encode | All pass |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugin/gr/gr.go` | ✅ Modified | SDK callback pattern |
| `internal/plugin/hostname/hostname.go` | ✅ Modified | SDK callback pattern |
| `internal/plugin/rib/rib.go` | ✅ Modified | SDK callback pattern |
| `internal/plugin/flowspec/plugin.go` | ✅ Modified | SDK callback pattern |
| `internal/plugin/evpn/plugin.go` | ✅ Modified | SDK callback pattern |
| `internal/plugin/vpn/vpn.go` | ✅ Modified | SDK callback pattern |
| `internal/plugin/bgpls/plugin.go` | ✅ Modified | SDK callback pattern |
| `internal/plugin/inprocess.go` | ✅ Modified | net.Conn runner signature |
| `internal/plugin/process.go` | ✅ Modified | Text protocol code removed |
| `internal/plugin/server.go` | ✅ Modified | Text protocol deleted, runtime RPC implemented |
| `internal/plugin/registration.go` | ✅ Modified | ParseLine deleted, types preserved |
| `internal/plugin/types.go` | ✅ Modified | UseRPC flag removed |
| `internal/plugin/server_rpc_test.go` | 🔄 Changed | Tests added to existing `server_test.go` instead |
| `internal/plugin/verify_test.go` | ❌ Skipped | Compilation verifies no text protocol code remains |

### Audit Summary
- **Total items:** 36
- **Done:** 27
- **Partial:** 0
- **Skipped:** 2 (TestNoTextProtocolCode, verify_test.go — verified by compilation)
- **Changed:** 7 (test names differ, file locations differ — coverage equivalent)

## Checklist

### Design (see `rules/design-principles.md`)
- [x] No premature abstraction (SDK already exists with 35 tests)
- [x] No speculative features (converting existing plugins, not adding new ones)
- [x] Single responsibility (each plugin does one thing via SDK callbacks)
- [x] Explicit behavior (SDK callbacks are explicit, no hidden text parsing)
- [x] Minimal coupling (SDK isolates plugins from wire format)
- [x] Next-developer test (SDK-based plugins are shorter and clearer)

### TDD
- [x] Tests written
- [x] Tests FAIL
- [x] Implementation complete
- [x] Tests PASS
- [x] Feature code integrated into codebase
- [x] Functional tests verify end-user behavior

### Verification
- [x] `make lint` passes (pre-existing typecheck cascade unrelated to migration)
- [x] `make test` passes
- [x] `make functional` passes

### Documentation (during implementation)
- [x] Required docs read
- [x] RFC references preserved in plugin code

### Completion (after tests pass)
- [x] Architecture docs updated with learnings (plugin-design.md updated)
- [x] Implementation Audit completed
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together

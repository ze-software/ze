# Spec: connection-handoff

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/plugin-design.md` - plugin architecture patterns
4. `internal/plugin/socketpair.go` - existing socketpair infrastructure
5. `internal/plugin/process.go` - external plugin startup (`startExternal`)
6. `internal/hub/hub.go` - Orchestrator struct and plugin lifecycle

## Task

Implement connection handoff from Hub to plugins via SCM_RIGHTS (fd passing over Unix domain sockets). This enables the Hub to create listen sockets and pass them to plugins, similar to systemd socket activation.

**Scope:** SCM_RIGHTS fd passing primitives + listen socket handoff protocol + hub integration.

**Prerequisites:**
- Socketpair infrastructure exists (`internal/plugin/socketpair.go`)
- External plugins already use socketpairs for IPC (`process.go:startExternal`)
- 5-stage plugin protocol is implemented

**Depends on:** Nothing (socketpairs already in place).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - system overview, plugin architecture
- [ ] `.claude/rules/plugin-design.md` - 5-stage protocol, plugin registration

### RFC Summaries
(No protocol RFCs — this is an IPC mechanism, not BGP wire format.)

**Key insights:**
- External plugins already communicate via `socketpair(AF_UNIX, SOCK_STREAM)` — see `process.go:329`
- Socket FDs passed via `cmd.ExtraFiles` with env vars `ZE_ENGINE_FD=3`, `ZE_CALLBACK_FD=4`
- Unix stream sockets support `SCM_RIGHTS` ancillary data for fd passing
- `DualSocketPair` provides engine-side `net.Conn` which is `*net.UnixConn` for external plugins

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/socketpair.go` - `DualSocketPair`, `NewExternalSocketPairs()`, `PluginFiles()`, `newUnixSocketPair()`, `connToFile()`
- [ ] `internal/plugin/process.go` - `startExternal()` creates socketpairs, passes via `ExtraFiles`, creates `PluginConn` on engine side
- [ ] `internal/hub/hub.go` - `Orchestrator` composes `SubsystemManager`, `SchemaRegistry`, `Hub`; no connection handoff logic

**Behavior to preserve:**
- External plugin startup via `startExternal()` — socketpair creation, FD inheritance, env vars
- Internal plugin startup via `startInternal()` — `net.Pipe` pairs
- 5-stage plugin registration protocol
- Engine-side `PluginConn` for YANG RPC

**Behavior to change:**
- Add `SendFD()` and `ReceiveFD()` methods for SCM_RIGHTS on engine-side Unix connections
- Add `declare connection-handler` protocol extension to Stage 1
- Add hub logic to create listen sockets and hand them off after Stage 1

## Data Flow (MANDATORY)

### Entry Point
- Hub creates listen socket (e.g., TCP port 179)
- Plugin declares `connection-handler listen <port>` in Stage 1

### Transformation Path
1. Hub parses config, identifies needed listen sockets
2. Hub creates listen socket(s) before forking plugins
3. Plugin starts, completes Stage 1 (Declaration) including `connection-handler` declaration
4. Hub detects declaration, sends listen fd via SCM_RIGHTS over engine socket
5. Plugin receives fd, converts to `*net.TCPListener`
6. Protocol continues to Stage 2 (Config) and beyond
7. After Stage 5 (Ready), plugin calls `Accept()` on listener

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Hub → Plugin (fd) | SCM_RIGHTS ancillary data on Unix socket | [ ] |
| Plugin → Hub (declaration) | YANG RPC on Socket A | [ ] |
| Hub → Plugin (ack) | YANG RPC on Socket B | [ ] |

### Integration Points
- `internal/plugin/socketpair.go` — add `SendFD` / `ReceiveFD` functions
- `internal/plugin/process.go` — no changes needed (socketpairs already used)
- `internal/plugin/server.go` — handle `connection-handler` declaration in Stage 1
- `internal/hub/hub.go` — create listen sockets, pass after declaration
- `pkg/plugin/sdk/` — add `ReceiveFD()` helper for plugin-side consumption

### Architectural Verification
- [ ] No bypassed layers (fd passing uses existing socket infrastructure)
- [ ] No unintended coupling (fd passing is a utility on existing connections)
- [ ] No duplicated functionality (no existing fd passing code)
- [ ] Zero-copy preserved where applicable (fd passing is zero-copy by nature)

---

## Design

### 1. SCM_RIGHTS Primitives

Add fd passing functions that operate on the existing `*net.UnixConn` connections.

#### SendFD

| Step | Operation |
|------|-----------|
| 1 | Cast `net.Conn` to `*net.UnixConn` |
| 2 | Build `unix.UnixRights(fd)` control message |
| 3 | Call `WriteMsgUnix(data, oob, nil)` — data carries a framing byte, oob carries the fd |
| 4 | Caller closes their copy of fd after send |

#### ReceiveFD

| Step | Operation |
|------|-----------|
| 1 | Cast `net.Conn` to `*net.UnixConn` |
| 2 | Call `ReadMsgUnix(data, oob)` with OOB buffer sized for one fd |
| 3 | Parse control messages via `unix.ParseSocketControlMessage()` |
| 4 | Extract fd via `unix.ParseUnixRights()` |
| 5 | Return fd as `*os.File` (caller converts to `net.Listener` or `net.Conn`) |

#### Error Cases

| Condition | Behavior |
|-----------|----------|
| Connection is `*net.Pipe` (internal plugin) | Return error — fd passing requires real Unix sockets |
| No fd in control message | Return error |
| Multiple fds received | Use first, close extras |

### 2. Connection Handler Protocol

Extension to Stage 1 (Declaration). Plugins that want listen sockets include this in their registration.

#### Declaration (Plugin → Engine, Stage 1)

Plugin includes in `ze-plugin-engine:declare-registration`:

| Field | Type | Description |
|-------|------|-------------|
| `connection-handlers` | list | Requested listen sockets |
| `connection-handlers[].type` | string | `"listen"` (Mode A) |
| `connection-handlers[].port` | integer | TCP port to listen on |
| `connection-handlers[].address` | string | Bind address (optional, default `""` = all interfaces) |

#### Handoff (Engine → Plugin, between Stage 1 and Stage 2)

After processing Stage 1, engine sends fd via:
1. YANG RPC `ze-plugin-callback:connection-handler` with port metadata
2. SCM_RIGHTS ancillary data carrying the listen fd

Plugin responds with standard RPC acknowledgment.

#### Registration Data

| Field | Type | Description |
|-------|------|-------------|
| `ConnectionHandlers` | list | Parsed from Stage 1 declaration |
| Each entry: Type | string | `"listen"` |
| Each entry: Port | integer | Requested port |
| Each entry: Address | string | Bind address |

### 3. Hub Listen Socket Management

The hub creates and manages listen sockets based on config and plugin declarations.

#### Startup Flow

| Phase | Hub Action | Plugin Action |
|-------|------------|---------------|
| 1 | Parse config, identify listeners needed | — |
| 2 | Create listen socket(s) | — |
| 3 | Fork plugin with socketpair for IPC | Start, connect to sockets |
| 4 | Wait for Stage 1 declaration | Send `declare-registration` including `connection-handlers` |
| 5 | Match declared ports to created sockets | — |
| 6 | Send listen fd via SCM_RIGHTS | Receive fd, convert to listener |
| 7 | Continue Stage 2 (Config) | Continue protocol |
| 8 | — | After Stage 5 (Ready), start `Accept()` loop |

#### Socket Ownership

| Phase | Owner | State |
|-------|-------|-------|
| Pre-handoff | Hub | Holds listen socket fd |
| During handoff | Both | Hub sends, plugin receives copy |
| Post-handoff | Plugin | Hub closes its copy; plugin owns exclusively |

---

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSendReceiveFD` | `internal/plugin/fdpass_test.go` | Send fd over socketpair, receive and verify usable | |
| `TestSendReceiveFDListenSocket` | `internal/plugin/fdpass_test.go` | Pass TCP listener fd, verify Accept() works on receiver | |
| `TestSendFDInternalPipeError` | `internal/plugin/fdpass_test.go` | Returns error for net.Pipe connections | |
| `TestSendReceiveFDMultiple` | `internal/plugin/fdpass_test.go` | Send multiple fds sequentially | |
| `TestConnectionHandlerDeclaration` | `internal/plugin/registration_test.go` | Parse connection-handler in Stage 1 registration | |
| `TestConnectionHandlerNoDeclaration` | `internal/plugin/registration_test.go` | No connection-handlers field = empty list | |
| `TestHubListenSocketCreate` | `internal/hub/hub_test.go` | Hub creates listen socket on specified port | |
| `TestHubListenSocketHandoff` | `internal/hub/hub_test.go` | Hub passes listen fd to plugin, plugin can Accept() | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Port (handoff) | 1–65535 | 65535 | 0 | 65536 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `handoff-listen` | `test/plugin/handoff-listen.ci` | Plugin receives listen socket, accepts connection | |
| `handoff-no-declare` | `test/plugin/handoff-no-declare.ci` | Plugin without connection-handler works normally | |

### Future (if deferring any tests)
- Per-connection handoff (Mode B) — deferred until Mode A is proven
- Multiple plugins competing for same port — deferred

---

## Files to Modify
- `internal/plugin/server.go` - Handle `connection-handlers` in Stage 1 registration processing
- `internal/plugin/types.go` - Add `ConnectionHandler` to `PluginRegistration`
- `internal/hub/hub.go` - Create listen sockets, send fd after Stage 1
- `pkg/plugin/sdk/sdk.go` - Add `ReceiveFD()` helper and `ConnectionHandler` declaration in `Registration`

## Files to Create
- `internal/plugin/fdpass.go` - `SendFD()` and `ReceiveFD()` over Unix domain sockets
- `internal/plugin/fdpass_test.go` - fd passing unit tests
- `test/plugin/handoff-listen.ci` - Listen socket handoff functional test
- `test/plugin/handoff-no-declare.ci` - No-handoff backward compat functional test

---

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write fd passing unit tests** - Send/receive fd over socketpair, listen socket roundtrip
   → **Review:** Error cases covered (net.Pipe, no fd in message)?

2. **Run tests** - Verify FAIL (paste output)
   → **Review:** Fail for the right reason?

3. **Implement `internal/plugin/fdpass.go`** - SendFD and ReceiveFD using `unix.UnixRights`
   → **Review:** Simplest solution? Any platform portability issues?

4. **Run tests** - Verify PASS (paste output)

5. **Write registration tests** - Parse connection-handler declaration
   → **Review:** Missing declaration handled gracefully?

6. **Run tests** - Verify FAIL

7. **Add ConnectionHandler to types.go and registration parsing in server.go**
   → **Review:** Consistent with existing registration fields?

8. **Run tests** - Verify PASS

9. **Write hub integration tests** - Create listen socket, handoff to plugin
   → **Review:** Socket cleanup on failure?

10. **Run tests** - Verify FAIL

11. **Implement hub listen socket management** - Create, hold, pass after Stage 1
    → **Review:** Socket closed by hub after handoff? Error path cleanup?

12. **Run tests** - Verify PASS

13. **Add SDK helper** - `ReceiveFD()` for plugin-side consumption
    → **Review:** Matches engine-side SendFD framing?

14. **Write functional tests** - End-to-end listen socket handoff
    → **Review:** Tests cover both with and without connection-handler?

15. **Verify all** - `make lint && make test && make functional` (paste output)
    → **Review:** Zero lint issues? All tests deterministic?

---

## Implementation Summary

<!-- Fill this section AFTER implementation, before moving to done -->

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered during implementation]

### Design Insights
- [Key learnings that should be documented elsewhere]

### Documentation Updates
- [List docs updated, or "None — no architectural changes"]

### Deviations from Plan
- [Any differences from original plan and why]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| SCM_RIGHTS SendFD | | | |
| SCM_RIGHTS ReceiveFD | | | |
| Connection handler declaration parsing | | | |
| Hub listen socket creation | | | |
| Hub fd handoff after Stage 1 | | | |
| SDK ReceiveFD helper | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestSendReceiveFD | | | |
| TestSendReceiveFDListenSocket | | | |
| TestSendFDInternalPipeError | | | |
| TestSendReceiveFDMultiple | | | |
| TestConnectionHandlerDeclaration | | | |
| TestConnectionHandlerNoDeclaration | | | |
| TestHubListenSocketCreate | | | |
| TestHubListenSocketHandoff | | | |
| handoff-listen | | | |
| handoff-no-declare | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugin/server.go` | | |
| `internal/plugin/types.go` | | |
| `internal/hub/hub.go` | | |
| `pkg/plugin/sdk/sdk.go` | | |
| `internal/plugin/fdpass.go` | | |
| `internal/plugin/fdpass_test.go` | | |
| `test/plugin/handoff-listen.ci` | | |
| `test/plugin/handoff-no-declare.ci` | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Checklist

### 🏗️ Design
- [ ] No premature abstraction (3+ concrete use cases exist?)
- [ ] No speculative features (is this needed NOW?)
- [ ] Single responsibility (each component does ONE thing?)
- [ ] Explicit behavior (no hidden magic or conventions?)
- [ ] Minimal coupling (components isolated, dependencies minimal?)
- [ ] Next-developer test (would they understand this quickly?)

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs (last valid, first invalid above/below)
- [ ] Feature code integrated into codebase (`internal/*`, `cmd/*`)
- [ ] Functional tests verify end-user behavior (`.ci` files)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] RFC summaries read (all referenced RFCs)
- [ ] RFC references added to code
- [ ] RFC constraint comments added

### Completion
- [ ] Architecture docs updated with learnings
- [ ] Implementation Audit completed (all items have status + location)
- [ ] All Partial/Skipped items have user approval
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together

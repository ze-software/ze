# Spec: Pipe-Based Subsystem Infrastructure

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugin/subsystem.go` - subsystem handler implementation
4. `cmd/ze-subsystem/main.go` - subsystem binary

## Task

Build infrastructure for internal subsystems that run as **separate forked processes** communicating via **pipes**, using the same 5-stage protocol as external plugins.

### Why Forked Processes?

The previous approach (init() self-registration) was wrong because:
- Internal handlers are compiled into the main binary
- They cannot be run separately or replaced at runtime
- No isolation between subsystems

**User requirement:** "We want to have OTHER programs, forked, registering later on and communicating via the plugin API - for that we can not have internal registration, it must be done via a pipe."

### Goals

1. **Forked processes** - Subsystems run as separate processes (fork or separate binary)
2. **Pipe communication** - stdin/stdout pipes like external plugins
3. **Same protocol** - 5-stage startup (Declaration → Config → Capability → Registry → Ready)
4. **Reuse process.go** - Extend existing Process infrastructure
5. **Command routing** - Dispatcher routes commands to correct subsystem via pipes

## Current Behavior

**Source files read:**
- [x] `internal/plugin/process.go` - existing pipe communication for external plugins
- [x] `internal/plugin/registration.go` - 5-stage protocol parsing
- [x] `internal/plugin/command.go` - Dispatcher routes commands to handlers

**Behavior preserved:**
- External plugin protocol unchanged
- Process infrastructure reused, not duplicated

## Required Reading

### Architecture Docs
- [x] `docs/architecture/api/architecture.md` - [current API design]
- [x] `docs/architecture/api/ipc_protocol.md` - [5-stage protocol details]

### Source Files
- [x] `internal/plugin/process.go` - [pipe communication, Process struct]
- [x] `internal/plugin/registration.go` - [5-stage protocol parsing]
- [x] `internal/plugin/command.go` - [Dispatcher, command routing]
- [x] `internal/plugin/server.go` - [coordinator, startup]

**Key insights:**
- Process already handles pipe communication and 5-stage protocol
- ProcessManager manages multiple processes
- Dispatcher routes commands but originally only for builtin handlers
- Extended Dispatcher to route to processes via pipes

## Design Decisions

### Binary Structure

**Decision:** Single `ze-subsystem` binary with `--mode` flag is simpler for distribution than separate binaries per subsystem.

| Option | Pros | Cons | Chosen |
|--------|------|------|--------|
| Single binary with --mode flag | Simple distribution, shared code | All modes in one binary | Yes |
| Separate binaries per subsystem | Smaller individual binaries | More binaries to manage | No |

### Subsystem Dependencies

**Problem:** Subsystems need access to reactor APIs (cache, peers, events) but run in separate process.

**Solution:** Bidirectional communication - subsystem can call back to engine for reactor operations.

| Communication Direction | Format | Purpose |
|------------------------|--------|---------|
| Engine → Subsystem | `#alpha command` | Engine-initiated request |
| Subsystem → Engine | `#N command` | Subsystem callback (numeric serial) |
| Response | `@serial data` | Response to either direction |

### Message Flow for Reactor Access

1. Engine sends command to subsystem via stdin
2. Subsystem needs reactor data, sends callback to engine via stdout
3. Engine processes callback, sends response to subsystem
4. Subsystem receives callback response, completes original request
5. Subsystem sends final response to engine

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSubsystemBinaryExists` | `internal/plugin/subsystem_test.go` | Binary compiles | Done |
| `TestSubsystemProtocol` | `internal/plugin/subsystem_test.go` | 5-stage protocol completes | Done |
| `TestSubsystemCommand` | `internal/plugin/subsystem_test.go` | Command routed to subprocess | Done |
| `TestSubsystemShutdown` | `internal/plugin/subsystem_test.go` | Graceful shutdown | Done |
| `TestSubsystemHandler` | `internal/plugin/subsystem_test.go` | Handler wrapper works | Done |
| `TestSubsystemManager` | `internal/plugin/subsystem_test.go` | Multi-subsystem coordination | Done |
| `TestDispatcherSubsystemIntegration` | `internal/plugin/subsystem_test.go` | Dispatcher routing | Done |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| Unit tests cover integration | `internal/plugin/subsystem_test.go` | Full protocol and command flow | Done |

## Files to Create

- `cmd/ze-subsystem/main.go` - Subsystem binary entry point
- `internal/plugin/subsystem.go` - SubsystemHandler (forked process wrapper)
- `internal/plugin/subsystem_test.go` - Tests

## Files to Modify

- `internal/plugin/command.go` - Add process-based command routing via SubsystemManager

## Implementation Steps

1. **Write unit tests** - Create tests BEFORE implementation (Done)
2. **Run tests** - Verify FAIL (Done)
3. **Create subsystem binary** - cmd/ze-subsystem with 5-stage protocol (Done)
4. **Create SubsystemHandler** - wrapper that spawns and routes to process (Done)
5. **Integrate with Dispatcher** - route commands to subsystems (Done)
6. **Run tests** - Verify PASS (Done)
7. **Verify all** - make lint, make test, make functional (Done)

## Implementation Summary

### What Was Implemented

- **`cmd/ze-subsystem/main.go`** - Subsystem binary with `--mode=cache|route|session` flag
  - Implements 5-stage protocol (Declaration → Config → Capability → Registry → Ready)
  - Bidirectional communication via callEngine() for reactor callbacks
  - Three subsystem modes: cache, route, session

- **`internal/plugin/subsystem.go`** - Core infrastructure
  - SubsystemConfig - configuration for forked subsystems
  - SubsystemHandler - wraps forked process, completes protocol, routes commands
  - SubsystemManager - manages multiple subsystem handlers
  - YANG schema parsing during declaration phase
  - Command routing via FindHandler()

- **`internal/plugin/subsystem_test.go`** - Comprehensive test coverage (7 tests)

### Design Insights

- Bidirectional communication solved the "reactor access" problem: subsystems can call back to engine for operations requiring reactor state
- Using numeric serials for subsystem→engine requests differentiates them from engine→subsystem requests (alpha serials)
- Single binary with --mode flag is simpler for distribution than separate binaries per subsystem

### Deviations from Plan

- None significant - implementation followed the "Final insight" design from the spec

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

# Spec: Hub Phase 0 - Serial Prefix Consistency

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/ipc_protocol.md` - IPC protocol format
4. `internal/plugin/subsystem.go` - current command handling

## Task

Ensure all commands in the IPC protocol use consistent serial-prefix format. This is prerequisite cleanup before Hub Architecture phases.

### Goals

1. Audit all command formats in existing code
2. Ensure all requests use `#serial command args` format
3. Ensure all responses use `@serial status [data]` format
4. Document any fire-and-forget commands that don't need serial

### Non-Goals

- Adding new commands (that's Phase 1-5)
- Changing command semantics
- Protocol version bump (format stays compatible)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/ipc_protocol.md` - [current protocol specification]
- [ ] `docs/architecture/api/process-protocol.md` - [5-stage protocol]

### Source Files
- [ ] `internal/plugin/subsystem.go` - [SubsystemHandler command handling]
- [ ] `internal/plugin/process.go` - [Process pipe communication]
- [ ] `cmd/ze-subsystem/main.go` - [plugin-side command handling]

## Design

### Command Format Standard

**Requests (expect response):**
```
#serial command args...
```

**Responses:**
```
@serial done
@serial error message
```

**Fire-and-forget (no response expected):**
```
command args...
```

### Commands to Audit

| Command | Current Format | Expected Format | Needs Fix? |
|---------|----------------|-----------------|------------|
| `declare cmd <name>` | No serial | Fire-and-forget | No |
| `declare done` | No serial | Fire-and-forget | No |
| `config <key> <value>` | No serial | Fire-and-forget | No |
| `config done` | No serial | Fire-and-forget | No |
| `ready` | No serial | Fire-and-forget | No |
| Runtime commands | Varies | `#serial cmd` | Check |

### Stage Protocol Commands

Stage 1-5 commands are fire-and-forget (no serial needed):
- `declare ...` - Plugin declares capabilities
- `config ...` - Hub sends config
- `capability ...` - Plugin sends capabilities
- `registry ...` - Hub sends registry
- `ready` - Plugin signals ready

These don't need serial because:
- They're one-way during startup
- Failures abort startup
- No concurrent requests to disambiguate

### Runtime Commands

Runtime commands MUST use serial prefix:
- `#serial bgp peer list`
- `#serial system shutdown`
- `#serial config reload`

Enables:
- Concurrent requests
- Response matching
- Timeout handling

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSerialPrefix_Request` | `internal/plugin/protocol_test.go` | Request format `#serial cmd` | |
| `TestSerialPrefix_Response` | `internal/plugin/protocol_test.go` | Response format `@serial status` | |
| `TestFireAndForget` | `internal/plugin/protocol_test.go` | Commands without serial work | |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `serial-request-response` | `test/data/plugin/serial-request-response.ci` | Serial matching works | |

## Files to Modify

- `internal/plugin/subsystem.go` - Verify serial handling
- `internal/plugin/process.go` - Verify serial handling
- `docs/architecture/api/ipc_protocol.md` - Document format clearly

## Implementation Steps

**Self-Critical Review:** After each step, review for issues and fix before proceeding.

1. **Audit existing code** - Search for command handling patterns
2. **Write tests** - Test serial prefix parsing
3. **Run tests** - Verify current behavior
4. **Fix inconsistencies** - If any found
5. **Update documentation** - Clarify serial requirement
6. **Verify all** - `make lint && make test && make functional`

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

### Documentation
- [ ] Required docs read
- [ ] IPC protocol doc updated

### Completion
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-hub-phase0-serial-prefix.md`

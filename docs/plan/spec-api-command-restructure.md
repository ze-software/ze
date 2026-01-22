# Spec: API Command Restructure

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/ipc_protocol.md` - canonical protocol spec
4. `internal/plugin/command.go` - dispatch implementation

## Task

Restructure API commands to use namespace prefixes:
- BGP-specific commands under `bgp` prefix
- Transaction commands under `transaction` prefix
- Keep `rib`, `session`, `system`, `daemon` at root level

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/ipc_protocol.md` - new command structure
- [ ] `docs/architecture/api/architecture.md` - current implementation
- [ ] `docs/architecture/api/commands.md` - current command syntax

### Source Files
- [ ] `internal/plugin/command.go` - dispatcher, handlers
- [ ] `internal/plugin/route.go` - route handlers
- [ ] `internal/plugin/forward.go` - forward handler
- [ ] `internal/plugin/session.go` - session handlers
- [ ] `internal/plugin/handler.go` - system handlers
- [ ] `internal/plugin/msgid.go` - msg-id handlers
- [ ] `internal/plugin/commit.go` - transaction handlers

## Command Mapping

### Old → New

| Old Command | New Command |
|-------------|-------------|
| `peer list` | `bgp list` |
| `peer show` | `bgp show` |
| `peer <ip> show` | `bgp peer <sel> show` |
| `peer <ip> teardown` | `bgp peer <sel> teardown` |
| `peer <sel> update text ...` | `bgp peer <sel> update text ...` |
| `peer <sel> update hex ...` | `bgp peer <sel> update hex ...` |
| `peer <sel> forward update-id <id>` | `bgp id <id> forward peer <sel>` |
| `msg-id <id> retain` | `bgp id <id> retain` |
| `msg-id <id> release` | `bgp id <id> release` |
| `msg-id <id> expire` | `bgp id <id> expire` |
| `msg-id list` | `bgp id list` |
| `peer <sel> borr <family>` | `bgp peer <sel> borr <family>` |
| `peer <sel> eorr <family>` | `bgp peer <sel> eorr <family>` |
| `watchdog announce <name>` | `bgp watchdog announce <name>` |
| `watchdog withdraw <name>` | `bgp watchdog withdraw <name>` |
| `begin transaction` | `transaction begin` |
| `commit transaction` | `transaction commit` |
| `rollback transaction` | `transaction rollback` |

### Unchanged

| Command | Reason |
|---------|--------|
| `rib show/flush/clear` | Protocol-agnostic |
| `session sync/reset/ping/bye/api` | IPC session, not BGP |
| `system help/version/command` | System-level |
| `daemon shutdown/reload/restart/status` | Daemon-level |
| `register/unregister command` | Plugin registration |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDispatchBgpList` | `internal/plugin/command_test.go` | `bgp list` routes correctly | |
| `TestDispatchBgpPeerShow` | `internal/plugin/command_test.go` | `bgp peer <sel> show` parses selector | |
| `TestDispatchBgpPeerUpdate` | `internal/plugin/command_test.go` | `bgp peer <sel> update text` | |
| `TestDispatchBgpIdForward` | `internal/plugin/command_test.go` | `bgp id <id> forward peer <sel>` | |
| `TestDispatchBgpIdRetain` | `internal/plugin/command_test.go` | `bgp id <id> retain` | |
| `TestDispatchTransaction` | `internal/plugin/command_test.go` | `transaction begin/commit/rollback` | |
| `TestOldCommandsRejected` | `internal/plugin/command_test.go` | Old syntax returns error | |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `bgp-peer-update` | `test/data/plugin/bgp-peer-update.ci` | Route injection with new syntax | |
| `bgp-id-forward` | `test/data/plugin/bgp-id-forward.ci` | Forward cached update | |
| `transaction-batch` | `test/data/plugin/transaction-batch.ci` | Transaction begin/commit | |

## Files to Modify

| File | Changes |
|------|---------|
| `internal/plugin/command.go` | Restructure dispatch tree, add `bgp` and `transaction` nodes |
| `internal/plugin/route.go` | Update handler signatures for new paths |
| `internal/plugin/forward.go` | Move under `bgp id <id> forward` |
| `internal/plugin/msgid.go` | Rename to `id.go`, update command paths |
| `internal/plugin/commit.go` | Update `transaction begin/commit/rollback` paths |
| `internal/plugin/rib/rib.go` | Update command strings sent to engine |
| `internal/plugin/rr/server.go` | Update command strings |
| `internal/plugin/gr/gr.go` | Update command strings (if any) |

## Files to Create

None - restructuring existing code.

## Implementation Steps

### Phase 1: Dispatch Tree Restructure

1. **Add `bgp` dispatch node**
   - `bgp list` → `handleBgpList`
   - `bgp show` → `handleBgpShow`
   - `bgp peer <sel>` → subtree for peer commands
   - `bgp id <id>` → subtree for id commands
   - `bgp watchdog` → subtree for watchdog commands

2. **Add `transaction` dispatch node**
   - `transaction begin` → `handleTransactionBegin`
   - `transaction commit` → `handleTransactionCommit`
   - `transaction rollback` → `handleTransactionRollback`

3. **Update selector parsing**
   - `bgp peer <sel>` extracts selector before dispatching

4. **Update id parsing**
   - `bgp id <id>` extracts ID before dispatching

### Phase 2: Handler Updates

1. **Rename `msgid.go` → `id.go`**
2. **Update handler function names**
3. **Update handler signatures for new argument order**

### Phase 3: Plugin Updates

1. **Update RIB plugin** (`internal/plugin/rib/rib.go`)
   - Change `peer <addr> update text ...` → `bgp peer <addr> update text ...`
   - Change `peer <addr> session api ready` → `bgp peer <addr> session api ready` (or keep session separate?)

2. **Update RR plugin** (`internal/plugin/rr/server.go`)
   - Similar command string updates

### Phase 4: Documentation Updates

1. **Update `commands.md`** with new syntax
2. **Update `architecture.md`** examples
3. **Update `process-protocol.md`** examples

### Phase 5: Deprecation (Optional)

1. Add aliases for old commands with deprecation warning
2. Remove after one release cycle

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| `bgp` prefix for BGP-specific | Namespace for future protocol support |
| `transaction` noun-first | Consistent with `bgp peer`, `bgp id` |
| `bgp id` not `bgp update id` | ID is entity, not action on update |
| Keep `rib` at root | Protocol-agnostic routing table |
| Keep `session` at root | IPC session, not BGP session |

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
- [ ] `ipc_protocol.md` updated (done)
- [ ] `commands.md` updated
- [ ] `architecture.md` updated
- [ ] `process-protocol.md` updated

### Completion
- [ ] All files committed together
- [ ] Spec moved to `docs/plan/done/`

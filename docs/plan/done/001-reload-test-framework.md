# Spec: reload-test-framework

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/testing/ci-format.md` - .ci format reference
4. `docs/functional-tests.md` - test infrastructure
5. `internal/test/runner/record.go` - .ci parsing
6. `internal/reactor/reactor.go` - Reload() stub

## Task

Implement config reload testing framework:
1. Extend .ci format to support mid-test signals and config updates
2. Implement Reload() to actually reload config
3. Create test: empty config → SIGHUP → peer added → connection occurs

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` - .ci format syntax
- [ ] `docs/functional-tests.md` - functional test infrastructure

### Source Files
- [ ] `internal/test/runner/record.go` - parsing extension points
- [ ] `internal/test/runner/runner.go` - test execution
- [ ] `internal/reactor/reactor.go` - Reload() stub at line 922
- [ ] `internal/reactor/signal.go` - SignalHandler, OnReload callback

**Key insights:**
- Reload() is a stub returning nil (does nothing)
- SIGHUP → OnReload callback is wired but handler does nothing
- .ci parseCmd() handles foreground/background processes
- Extension points: parseOption(), parseExpect(), parseCmd(), parseAction()
- Tmpfs files are written to temp dir at runtime

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReload_AddsPeer` | `internal/reactor/reactor_test.go` | Reload adds new peer from config | |
| `TestReload_RemovesPeer` | `internal/reactor/reactor_test.go` | Reload removes peer not in new config | |
| `TestReload_ParseError` | `internal/reactor/reactor_test.go` | Reload returns error on bad config | |
| `TestParseCmdSignal` | `internal/test/runner/record_test.go` | cmd=signal: parses correctly | |
| `TestParseCmdSleep` | `internal/test/runner/record_test.go` | cmd=sleep: parses correctly | |
| `TestParseCmdTmpfsUpdate` | `internal/test/runner/record_test.go` | cmd=tmpfs-update: parses correctly | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| sleep duration | 1ms-60s | 60s | 0ms | 61s |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `reload-add-peer` | `test/data/plugin/reload-add-peer.ci` | Empty config → reload → peer connects | |
| `reload-remove-peer` | `test/data/plugin/reload-remove-peer.ci` | Peer config → reload → peer removed | |

### Future (if deferring any tests)
- ~~Reload with changed peer settings (hold-time, capabilities)~~ - **IMPLEMENTED** in Phase 1

## Files to Modify
- `internal/reactor/reactor.go` - Implement Reload() method
- `internal/test/runner/record.go` - Add signal, sleep, tmpfs-update parsing
- `internal/test/runner/runner.go` - Execute new commands during test

## Files to Create
- `internal/reactor/reactor_test.go` - Reload unit tests (if not exists)
- `test/data/plugin/reload-add-peer.ci` - Functional test

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write unit tests for Reload()** - Create unit tests BEFORE implementation (strict TDD)
   - Test adding new peer via reload
   - Test removing peer via reload
   - Test parse error handling

   Review: Are edge cases covered? Boundary tests for numeric inputs?

2. **Run tests** - Verify FAIL (paste output)

   Review: Do tests fail for the RIGHT reason? Not syntax errors?

3. **Implement Reload()** - Minimal code to pass
   - Store config path in reactor
   - Re-parse config file
   - Diff current peers vs new peers
   - Add new peers, remove missing peers

   Review: Is this the simplest solution? Any code duplication?

4. **Run tests** - Verify PASS (paste output)

   Review: Did ALL tests pass? Any flaky behavior?

5. **Write unit tests for .ci extensions** - Test signal, sleep, tmpfs-update parsing

6. **Implement .ci parsing** - Add new command types to parseCmd()

7. **Implement .ci execution** - Execute signal/sleep/tmpfs-update during test run

8. **Functional tests** - Create reload-add-peer.ci

9. **Verify all** - `make lint && make test && make functional` (paste output)

## Design Decisions

### .ci Format Extensions

New command types in parseCmd():

| Command | Syntax | Behavior |
|---------|--------|----------|
| signal | `cmd=signal:seq=N:name=SIGHUP` | Send signal to foreground process |
| sleep | `cmd=sleep:seq=N:duration=500ms` | Pause test execution |
| tmpfs-update | `cmd=tmpfs-update:seq=N:path=config.conf` | Update tmpfs file mid-test |

### Reload Implementation

Reactor needs:
1. `configPath` field to store original config path
2. `Reload()` re-parses and diffs
3. New peers: call existing AddPeer() logic
4. Missing peers: call Shutdown() on peer

### Peer Diff Algorithm

```
current_peers = set of current peer addresses
new_peers = set of peer addresses from new config

to_add = new_peers - current_peers
to_remove = current_peers - new_peers

for each in to_add: create and start peer
for each in to_remove: shutdown peer
```

## Test Scenario: reload-add-peer.ci

```
# Initial empty config
tmpfs=config.conf:terminator=EOF_EMPTY
ze bgp {
}
EOF_EMPTY

# Start ze-peer to accept connections
cmd=background:seq=1:exec=ze-peer --sink --port $PORT

# Start server with empty config (no peers)
cmd=background:seq=2:exec=ze bgp server config.conf

# Wait for server startup
cmd=sleep:seq=3:duration=1s

# Update config to add peer
cmd=tmpfs-update:seq=4:path=config.conf
ze bgp {
    neighbor 127.0.0.1 {
        local-as 65001;
        peer-as 65002;
        connect $PORT;
    }
}
EOF_UPDATE

# Send SIGHUP to trigger reload
cmd=signal:seq=5:name=SIGHUP:target=ze

# Wait for connection
cmd=sleep:seq=6:duration=2s

# Verify OPEN was sent
expect=bgp:conn=1:seq=1:contains=OPEN
```

## Implementation Summary

### What Was Implemented

**Phase 1: Reload Implementation (COMPLETED)**

1. **reactor.go changes:**
   - Added `ConfigPath` field to `Config` struct
   - Added `ErrNoConfigPath` and `ErrNoReloadFunc` errors
   - Added `ReloadFunc` type returning `[]*PeerSettings` (full settings, not thin struct)
   - Added `SetReloadFunc()` method to set reload callback
   - Added `SetConfigPath()` method to set config path
   - Added `peerSettingsEqual()` to detect settings changes
   - Implemented `Reload()` on `reactorAPIAdapter`:
     - Validates ConfigPath and ReloadFunc are set
     - Calls ReloadFunc to get full PeerSettings from config
     - Diffs current peers vs new peers (add/remove/change)
     - Detects settings changes and re-adds peers with new settings
     - Uses `errors.Join()` to return all add errors
   - Wired up SIGHUP → Reload() in StartWithContext()

2. **config/loader.go changes:**
   - Added `CreateReactorWithPath()` for reload-enabled reactors
   - Added `createReloadFunc()` that calls `configToPeer()` for full conversion
   - Updated `LoadReactorFile()` to use new path-aware creation

3. **trace/trace.go changes:**
   - Added `ConfigReloaded()` trace function
   - Added `ConfigReloadFailed()` trace function

4. **reload_test.go (new file):**
   - `TestReloadAddsPeer` - verifies adding peer via reload
   - `TestReloadRemovesPeer` - verifies removing peer via reload
   - `TestReloadChangedSettings` - verifies settings changes detected and applied
   - `TestReloadParseError` - verifies error on bad config
   - `TestReloadNoConfigPath` - verifies error when path not set
   - `TestReloadNoReloadFunc` - verifies ErrNoReloadFunc returned
   - `TestReloadFileNotFound` - verifies error when file missing
   - `TestPeerSettingsEqual` - verifies comparison function

**Phase 2: .ci Framework Extensions (DEFERRED)**
- `cmd=signal:`, `cmd=sleep:`, `cmd=tmpfs-update:` - not yet implemented
- Functional test `reload-add-peer.ci` - not yet created

### Bugs Found/Fixed
- Wrong error returned when reloadFn is nil (was ErrNoConfigPath, now ErrNoReloadFunc)
- AddPeer errors were silently ignored (now returned via errors.Join)
- Settings changes were ignored (now detected and peer re-added)

### Design Insights
- Config package creates reactor, reactor can't import config → solved with callback pattern
- ReloadFunc returns full `*PeerSettings` to avoid duplicating conversion logic
- `configToPeer()` reused in reload path ensures identical peer configuration
- SIGHUP handler was declared but never wired up in StartWithContext()

### Deviations from Plan
- Split into two phases: Reload implementation (done) and .ci extensions (deferred)
- Used callback pattern instead of direct config import to avoid circular dependency
- Unit tests use simplified regex parser instead of full config parser
- Removed `ReloadPeerConfig` struct in favor of full `*PeerSettings` (simpler, no duplication)
- Implemented settings change detection (was planned as "Future/deferred")

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (ConfigPath field didn't exist)
- [x] Implementation complete
- [x] Tests PASS (8/8 reload tests pass)
- [ ] Boundary tests - N/A for this phase (no numeric inputs)

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes

### Documentation (during implementation)
- [x] Required docs read
- [ ] Architecture docs updated with learnings

### Completion (after tests pass)
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together

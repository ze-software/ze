# Spec: GR Restart Marker

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc4724.md` - GR capability wire format, Restarting Speaker MUST requirements
4. `internal/component/bgp/plugins/gr/gr.go` - current GR capability generation (R=0 hardcoded)
5. `internal/component/bgp/reactor/session_negotiate.go` - OPEN construction, pluginCapGetter
6. `internal/component/bgp/reactor/reactor.go` - shutdown cleanup path
7. `pkg/zefs/store.go` - BlobStore API (ReadFile, WriteFile, Has, Remove)
8. `internal/component/bgp/grmarker/` - marker package (created by this spec)

## Task

Implement RFC 4724 Restarting Speaker detection using a GR marker in zefs. On `ze signal restart` (or `restart` in the interactive CLI), the engine writes a marker with an expiry timestamp to `meta/bgp/gr-marker` in `database.zefs`, then shuts down. On startup, the engine reads the marker and, if valid, stores the expiry deadline. While the deadline has not passed, the engine sets the Restart State bit (R=1) in all GR capabilities advertised in OPEN messages. After the deadline passes, new connections get R=0. This tells peers "I just restarted, please retain my routes and don't wait for my EoR before sending routes."

Currently, ze only implements the Receiving Speaker side of RFC 4724. The Restarting Speaker side is missing because ze has no way to distinguish a restart from a cold start. All GR capabilities are advertised with R=0.

**Restart vs stop:** `ze signal restart` (or `restart` in the interactive CLI) writes the marker then shuts down (GR intent). `ze signal stop` (or `stop` in CLI) shuts down without a marker (no GR intent). The operator decides whether a shutdown is a restart.

**Two entry points, same handler:** The restart/stop commands must be available both via `ze signal` (SSH exec, non-interactive) and from the interactive CLI session. Both call the same underlying handler on the daemon. In the interactive CLI, `exit`/`quit` disconnect the SSH session (daemon keeps running); `stop` shuts down the daemon (no marker); `restart` writes the marker then shuts down the daemon. Confirmation prompt for `stop`/`restart` in interactive CLI since they affect all connected users.

**Design constraint:** The marker logic lives entirely in the engine. Plugins never touch zefs -- they may be remote (TCP) or non-Go (Python). No SDK or protocol changes needed.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - reactor lifecycle, plugin capability injection
  -> Constraint: capabilities flow from plugin (SetCapabilities) through CapabilityInjector to OPEN (pluginCapGetter)
  -> Decision: engine modifies InjectedCapability at retrieval time, not at storage time
- [ ] `docs/architecture/zefs-format.md` - BlobStore format, key conventions
  -> Constraint: hierarchical keys with "/" separator, atomic writes via temp+rename
  -> Constraint: existing keys use `meta/` prefix for engine metadata (meta/ssh/*, meta/instance/name)

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4724.md` - Graceful Restart mechanism
  -> Constraint: "the Restarting Speaker MUST set the 'Restart State' bit in the Graceful Restart Capability of the OPEN message" (Section 4.1)
  -> Constraint: R-bit is bit 0 (MSB) of the first byte of the GR capability value (0x80 mask)
  -> Constraint: restart-time is 12 bits (0-4095 seconds)
  -> Constraint: F-bit (Forwarding State) is per-family -- separate concern from R-bit

**Key insights:**
- R-bit in OPEN tells peers: "I have restarted." Peers MUST NOT wait for EoR from a restarting speaker before sending their own routes (deadlock avoidance).
- Without R=1, ze cannot signal restart state to peers. Every startup looks like a cold start.
- The marker bridges process boundaries: shutdown writes it, startup reads it.
- Engine already holds all GR capabilities via CapabilityInjector -- setting R-bit is a single byte OR on retrieval.
- R=1 must be time-bounded: only valid within the restart window. After the deadline, new connections get R=0.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/gr/gr.go` - GR plugin generates code-64 capability with R=0 hardcoded (line 409: `restartTime&0x0FFF` masks off R-bit)
  -> Constraint: plugin sends hex payload via sdk.CapabilityDecl, engine decodes into InjectedCapability
- [ ] `internal/component/bgp/reactor/session_negotiate.go` - sendOpen() calls pluginCapGetter() to fetch plugin capabilities for OPEN
  -> Constraint: capabilities pass through Peer.getPluginCapabilities() which queries CapabilityInjector
- [ ] `internal/component/bgp/reactor/peer.go` - getPluginCapabilities() queries api.GetPluginCapabilitiesForPeer()
  -> Constraint: returns []capability.Capability, engine already converts InjectedCapability to Capability
- [ ] `internal/component/bgp/reactor/reactor.go` - cleanup() stops components in 3 phases
- [ ] `internal/component/plugin/registration.go` - InjectedCapability struct, CapabilityInjector storage
  -> Constraint: InjectedCapability has Code (uint8) and Value ([]byte) fields
- [ ] `cmd/ze/main.go` - resolveStorage() creates BlobStore at `{configDir}/database.zefs`
  -> Constraint: zefs path derived from config directory
  -> Constraint: cmd/ze/main.go does NOT import reactor -- marker functions must live outside reactor/
- [ ] `pkg/zefs/store.go` - BlobStore: ReadFile, WriteFile, Has, Remove, atomic writes
- [ ] `internal/component/ssh/ssh.go` - SSH server dispatches "stop" via shutdownFunc callback
  -> Constraint: restart handler will follow same callback pattern (restartFunc)
- [ ] `internal/component/cli/model.go` - interactive CLI has exit/quit (disconnect session), no stop/restart commands
  -> Constraint: stop/restart are daemon lifecycle commands, not session commands

**Behavior to preserve:**
- GR plugin generates capability hex unchanged (R=0 in payload)
- Plugin 5-stage protocol unchanged -- no new RPCs
- SDK unchanged -- remote and Python plugins unaffected
- Existing receiving-speaker procedures in GR plugin unchanged
- OPEN message structure and capability ordering unchanged
- `exit`/`quit` in interactive CLI disconnect session only (daemon keeps running)

**Behavior to change:**
- Engine sets R=1 on code-64 capabilities while within restart deadline (new)
- `ze signal restart` writes marker then shuts down (new command)
- `restart` in interactive CLI does the same (new command)
- `stop` in interactive CLI shuts down daemon without marker (new command)
- Engine reads marker on startup, stores expiry deadline (new)
- SSH server handles "restart" exec command alongside existing "stop" (new)

## Data Flow (MANDATORY)

### Entry Point: Restart command (marker write)

- Operator runs `ze signal restart` or types `restart` in interactive CLI
- SSH exec delivers "restart" command to SSH server (or CLI dispatches to daemon handler)
- SSH server dispatches to restartFunc callback (new, alongside existing shutdownFunc)

### Transformation Path: Restart

1. SSH server receives "restart" exec command (or CLI dispatches "restart")
2. restartFunc callback fires -- this callback is a closure wired by daemon startup code, capturing CapabilityInjector and zefs BlobStore
3. Closure: query CapabilityInjector for all code-64 capabilities across all peers
4. Parse restart-time from each capability's Value bytes (bits 4-15 of first 2 bytes)
5. Compute max restart-time across all peers
6. If max > 0: compute expiry = now + max-restart-time, write marker to `meta/bgp/gr-marker` in zefs
7. Call shutdownFunc (same path as `ze signal stop`)
8. Daemon exits

### Entry Point: Startup (marker read)

- `cmd/ze/bgp/` startup path runs before reactor starts
- Opens `database.zefs` via resolveStorage()

### Transformation Path: Startup

1. Read `meta/bgp/gr-marker` key from zefs
2. Parse expiry timestamp from value (8-byte big-endian int64 UNIX seconds)
3. If marker exists and not expired: store expiry as `RestartUntil time.Time` in reactor config
4. If marker exists but expired: discard (R=0, cold start behavior)
5. Remove marker from zefs (consumed on read -- prevents stale restart on next cold start)
6. Start reactor with RestartUntil
7. When building OPEN messages: `Peer.getPluginCapabilities()` checks `time.Now().Before(r.restartUntil)`
8. If within deadline: copy the InjectedCapability.Value slice, OR 0x80 into byte 0 of the copy, return modified copy
9. If past deadline: return unmodified capability (R=0)

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| cmd/ze/bgp -> reactor | RestartUntil time.Time in reactor Config struct | [ ] |
| reactor -> session | pluginCapGetter callback returns modified capability copies when within deadline | [ ] |
| zefs -> cmd/ze/bgp | grmarker.Read(store) returns expiry time | [ ] |
| restartFunc -> zefs | grmarker.Write(store, expiresAt) in restart handler closure | [ ] |

### Integration Points

- `Peer.getPluginCapabilities()` - where R-bit is conditionally applied (time check + copy + modify)
- `reactor.Config.RestartUntil` - carries expiry deadline from startup to reactor
- `ssh.Server.restartFunc` - closure wired by daemon startup, captures CapabilityInjector + zefs
- `resolveStorage()` in cmd/ze startup - where marker is read

### Architectural Verification

- [ ] No bypassed layers (marker goes through zefs BlobStore API)
- [ ] No unintended coupling (engine owns marker, plugin is unaware)
- [ ] No duplicated functionality (zefs already provides atomic writes)
- [ ] Zero-copy preserved (marker is tiny, no performance concern)
- [ ] R-bit expires naturally (time comparison, no timer goroutine needed)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze signal restart` via SSH exec | -> | Marker written to zefs, daemon exits | `test/plugin/gr-signal-restart.ci` |
| `restart` typed in interactive CLI | -> | Same handler as above | `test/plugin/gr-cli-restart.ci` |
| Config with GR + valid marker in zefs | -> | R=1 in OPEN | `test/plugin/gr-marker-restart.ci` |
| Config with GR + no marker in zefs | -> | R=0 in OPEN | `test/plugin/gr-marker-cold-start.ci` |
| Config with GR + expired marker in zefs | -> | R=0 in OPEN | `test/plugin/gr-marker-expired.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze signal restart` with GR-enabled peers | Marker written to zefs with expiry = now + max(restart-time), then daemon shuts down |
| AC-2 | Startup with valid (non-expired) marker in zefs | R=1 (0x80) set in code-64 capability in OPEN for connections within the restart window |
| AC-3 | Startup with no marker in zefs | R=0 in code-64 capability in OPEN (current behavior preserved) |
| AC-4 | Startup with expired marker in zefs | R=0 in code-64 capability, marker removed |
| AC-5 | `ze signal restart` with no GR-enabled peers | No marker written, daemon shuts down normally |
| AC-6 | Marker consumed on startup | Marker removed from zefs after reading |
| AC-7 | Max restart-time computed correctly | Marker expiry uses largest restart-time across all GR-enabled peers |
| AC-8 | `ze signal stop` | No marker written (existing behavior preserved) |
| AC-9 | `ze signal restart` via SSH exec | SSH server dispatches "restart" command, responds "restarting daemon" |
| AC-10 | `restart` in interactive CLI | Confirmation prompt, then restart handler called, CLI session ends |
| AC-11 | `stop` in interactive CLI | Confirmation prompt, then stop handler called (no marker), CLI session ends |
| AC-12 | Peer connects after restart window expires | R=0 in OPEN (RestartUntil deadline has passed) |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWriteGRMarker` | `internal/component/bgp/grmarker/grmarker_test.go` | Marker written to zefs with correct expiry | |
| `TestReadGRMarkerValid` | `internal/component/bgp/grmarker/grmarker_test.go` | Valid marker read, returns expiry deadline | |
| `TestReadGRMarkerExpired` | `internal/component/bgp/grmarker/grmarker_test.go` | Expired marker returns no-restart | |
| `TestReadGRMarkerMissing` | `internal/component/bgp/grmarker/grmarker_test.go` | Missing marker returns no-restart | |
| `TestReadGRMarkerCorrupt` | `internal/component/bgp/grmarker/grmarker_test.go` | Corrupt marker returns no-restart (not crash) | |
| `TestRemoveGRMarker` | `internal/component/bgp/grmarker/grmarker_test.go` | Marker removed after reading | |
| `TestMaxRestartTime` | `internal/component/bgp/grmarker/grmarker_test.go` | Max computed from multiple InjectedCapabilities | |
| `TestSetRBitOnCapability` | `internal/component/bgp/grmarker/grmarker_test.go` | 0x80 OR'd into byte 0 of copied code-64 Value | |
| `TestSetRBitNoGRCap` | `internal/component/bgp/grmarker/grmarker_test.go` | Non-code-64 capabilities unchanged | |
| `TestSetRBitShortValue` | `internal/component/bgp/grmarker/grmarker_test.go` | Code-64 with Value < 2 bytes: no panic, no modification | |
| `TestSetRBitOriginalUnmodified` | `internal/component/bgp/grmarker/grmarker_test.go` | Original InjectedCapability.Value unchanged after R-bit set on copy | |
| `TestRestartDeadlineExpiry` | `internal/component/bgp/reactor/peer_test.go` | R=1 before deadline, R=0 after deadline | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| restart-time | 0-4095 | 4095 | N/A (0 is valid) | N/A (clamped by plugin) |
| expiry timestamp | epoch to far future | now + 4095s | now - 1s (expired) | N/A |
| marker value | 8 bytes (int64 big-endian) | valid timestamp | 0 bytes (empty) | > 8 bytes (truncated) |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `gr-signal-restart` | `test/plugin/gr-signal-restart.ci` | `ze signal restart` writes marker and shuts down | |
| `gr-cli-restart` | `test/plugin/gr-cli-restart.ci` | `restart` in interactive CLI writes marker and shuts down | |
| `gr-marker-restart` | `test/plugin/gr-marker-restart.ci` | GR marker exists, OPEN shows R=1 | |
| `gr-marker-cold-start` | `test/plugin/gr-marker-cold-start.ci` | No marker, OPEN shows R=0 | |
| `gr-marker-expired` | `test/plugin/gr-marker-expired.ci` | Expired marker, OPEN shows R=0 | |

### Future (if deferring any tests)
- F-bit (Forwarding State) per family -- separate spec for forwarding-state signaling
- Selection Deferral Timer -- restarting speaker SHOULD defer route selection (RFC 4724 Section 4.1)
- Supervisor crash recovery -- supervisor writes marker on behalf of crashed child

## Files to Modify

- `cmd/ze/signal/main.go` - add "restart" command to dispatch and usage text
- `internal/component/ssh/ssh.go` - add restartFunc callback alongside shutdownFunc, dispatch "restart" exec command
- `internal/component/cli/model.go` - add `restart` and `stop` as interactive CLI commands (constants + dispatch)
- `internal/component/cli/model_commands.go` - handle restart/stop commands with confirmation prompt (call daemon handler, end session)
- `internal/component/bgp/reactor/peer.go` - modify getPluginCapabilities() to check RestartUntil deadline and conditionally set R-bit on copies
- `internal/component/bgp/reactor/reactor.go` - add RestartUntil time.Time to Config, wire restartFunc to SSH server
- `cmd/ze/bgp/childmode.go` (or bgp startup path) - read marker from zefs at startup, set reactor Config.RestartUntil
- `internal/component/plugin/registration.go` - add method to iterate code-64 capabilities for restart-time extraction

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| RPC count in architecture docs | No | - |
| CLI commands/flags | Yes | `cmd/ze/signal/main.go` - add "restart" to dispatch |
| CLI usage/help text | Yes | `cmd/ze/signal/main.go` - add "restart" to usage |
| Interactive CLI commands | Yes | `internal/component/cli/model.go` - add restart/stop constants |
| API commands doc | No | - |
| Plugin SDK docs | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | No (no new RPCs) | - |

## Files to Create

- `internal/component/bgp/grmarker/grmarker.go` - marker read/write/remove helpers, R-bit copy-and-modify logic, max restart-time extraction. Depends only on `pkg/zefs` and `internal/component/plugin` (for InjectedCapability type). Importable from both cmd/ze/bgp/ (startup) and reactor wiring (restart handler closure).
- `internal/component/bgp/grmarker/grmarker_test.go` - unit tests
- `test/plugin/gr-signal-restart.ci` - functional test: `ze signal restart` writes marker and shuts down
- `test/plugin/gr-cli-restart.ci` - functional test: `restart` in interactive CLI writes marker and shuts down
- `test/plugin/gr-marker-restart.ci` - functional test: marker present, R=1 in OPEN
- `test/plugin/gr-marker-cold-start.ci` - functional test: no marker, R=0 in OPEN
- `test/plugin/gr-marker-expired.ci` - functional test: expired marker, R=0 in OPEN

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Marker read/write** -- zefs marker operations
   - Tests: `TestWriteGRMarker`, `TestReadGRMarkerValid`, `TestReadGRMarkerExpired`, `TestReadGRMarkerMissing`, `TestReadGRMarkerCorrupt`, `TestRemoveGRMarker`
   - Files: `internal/component/bgp/grmarker/grmarker.go`
   - Marker key: `meta/bgp/gr-marker`
   - Marker value: 8-byte big-endian int64 UNIX timestamp (expiry time in seconds)
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Max restart-time extraction** -- compute from stored capabilities
   - Tests: `TestMaxRestartTime`
   - Files: `internal/component/bgp/grmarker/grmarker.go`
   - Filters code 64, parses restart-time from Value[0:2] (bits 4-15), returns max across all peers
   - Verify: tests fail -> implement -> tests pass

3. **Phase: R-bit injection** -- copy-and-modify on capability retrieval
   - Tests: `TestSetRBitOnCapability`, `TestSetRBitNoGRCap`, `TestSetRBitShortValue`, `TestSetRBitOriginalUnmodified`, `TestRestartDeadlineExpiry`
   - Files: `internal/component/bgp/grmarker/grmarker.go`, `internal/component/bgp/reactor/peer.go`
   - R-bit helper in grmarker package takes InjectedCapability slice, returns new slice with R-bit set on copies of code-64 entries (original Values unchanged)
   - In `Peer.getPluginCapabilities()`: if `time.Now().Before(r.restartUntil)`, call grmarker helper on results before converting to capability.Capability
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Startup integration** -- read marker, set deadline
   - Files: `cmd/ze/bgp/childmode.go` (or BGP startup path), reactor Config
   - On startup: open zefs, call grmarker.Read, if valid set reactor Config.RestartUntil, call grmarker.Remove
   - Verify: integration with reactor

5. **Phase: Restart command** -- `ze signal restart` and interactive CLI
   - Tests: `TestSSHRestartCommand` (in ssh_test.go or signal test)
   - Files: `cmd/ze/signal/main.go`, `internal/component/ssh/ssh.go`, `internal/component/cli/model.go`, `internal/component/cli/model_commands.go`, reactor wiring
   - SSH server gets a `restartFunc` callback, wired as a closure by daemon startup. The closure captures CapabilityInjector (for max restart-time) and zefs BlobStore (for marker write). It computes, writes, then calls shutdownFunc.
   - `ze signal restart` sends "restart" via SSH exec (same pattern as "stop")
   - Interactive CLI: `restart` and `stop` commands with confirmation prompt ("This will shut down the daemon. Continue? [y/N]"). On confirm: call restartFunc or shutdownFunc, then quit CLI.
   - `ze signal stop` and `exit`/`quit` in CLI unchanged
   - Verify: `ze signal restart` writes marker and daemon exits

6. **Functional tests** -- create .ci tests for end-to-end behavior
7. **RFC refs** -- add `// RFC 4724 Section 4.1` comments on R-bit logic
8. **Full verification** -- `make ze-verify`
9. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | R-bit is 0x80 (bit 0 MSB), not 0x01. Restart-time is bits 4-15, not bytes 1-2. |
| R-bit on copy | getPluginCapabilities returns modified copies, original InjectedCapability.Value slices unchanged |
| R-bit time-bounded | R=1 only when time.Now().Before(restartUntil). Test with deadline in past -> R=0 |
| Marker cleanup | Marker is removed after reading to avoid stale restart on next cold start |
| Expired marker | Expired marker treated same as missing (R=0, marker removed) |
| No plugin changes | GR plugin code unchanged -- verify with git diff |
| Restart handler closure | restartFunc captures CapabilityInjector + zefs -- verify both are accessible |
| CLI stop vs exit | `stop` shuts down daemon, `exit`/`quit` disconnect session -- verify distinction |
| Confirmation prompt | `stop` and `restart` in interactive CLI prompt before acting |
| Rule: no-layering | No fallback to old behavior -- RestartUntil is zero-value time.Time for cold start |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `grmarker/grmarker.go` exists | `ls internal/component/bgp/grmarker/grmarker.go` |
| `grmarker/grmarker_test.go` exists | `ls internal/component/bgp/grmarker/grmarker_test.go` |
| All unit tests pass | `go test -race ./internal/component/bgp/grmarker/... -v` |
| Deadline expiry test passes | `go test -race ./internal/component/bgp/reactor/... -run TestRestartDeadline -v` |
| R-bit applied in getPluginCapabilities | `grep -n 'restartUntil\|RestartUntil' internal/component/bgp/reactor/peer.go` |
| "restart" in signal dispatch | `grep -n 'restart' cmd/ze/signal/main.go` |
| "restart" in SSH handler | `grep -n 'restart' internal/component/ssh/ssh.go` |
| "restart" and "stop" in interactive CLI | `grep -n 'cmdRestart\|cmdStop' internal/component/cli/model.go` |
| Functional tests exist | `ls test/plugin/gr-marker-*.ci test/plugin/gr-signal-restart.ci test/plugin/gr-cli-restart.ci` |
| GR plugin unchanged | `git diff internal/component/bgp/plugins/gr/` shows no changes |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Marker value must be exactly 8 bytes; reject shorter/longer |
| Timestamp overflow | UNIX timestamp must be positive and not overflow time.Time |
| Corrupt marker | Corrupt/truncated marker must not panic -- return no-restart |
| Marker key path | Key `meta/bgp/gr-marker` is a valid fs.ValidPath -- no injection |
| Denial of service | An attacker with zefs write access could inject a permanent marker -- but zefs access implies local root anyway |
| CLI confirmation | `stop`/`restart` in interactive CLI require confirmation -- prevent accidental daemon shutdown |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| `Restarting bool` flag | R=1 would persist for entire daemon lifetime -- RFC violation for connections after restart window | Critical review found that flag never expires. Replaced with `RestartUntil time.Time` with time comparison on each connection |
| Marker functions in `reactor/` package | `cmd/ze/main.go` does not import reactor -- would force new import cycle | Import check. Moved to standalone `internal/component/bgp/grmarker/` package (depends only on `pkg/zefs`) |
| Marker written in reactor.cleanup() | Restart handler (SSH) writes marker before calling shutdown -- cleanup() should not write marker (stop != restart) | Design review found that cleanup() runs for both stop and restart, so it cannot distinguish intent |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

- **Plugin never touches zefs.** Plugins may be remote (TCP) or non-Go (Python). Any persistence API would need to be exposed via the existing RPC protocol to all languages. Since the marker is engine-internal state (not plugin policy), keeping it in the engine is the correct boundary.
- **R-bit on copy, time-bounded.** The R-bit is applied to a copy of InjectedCapability.Value, checked against `time.Now().Before(restartUntil)`. No timer goroutine needed -- the time comparison naturally expires. After the deadline, new connections get R=0 without any state mutation.
- **restartFunc as closure.** The SSH server does not need to know about CapabilityInjector or zefs. The daemon startup code wires a `restartFunc func()` closure that captures both dependencies. Same pattern as the existing `shutdownFunc func()`. The SSH server just calls the function.
- **stop vs exit/quit.** In the interactive CLI: `exit`/`quit` disconnect the SSH session (daemon keeps running). `stop`/`restart` are daemon lifecycle commands that affect all connected users. Confirmation prompt is mandatory for the latter.
- **Inspired by rustbgpd.** rustbgpd (github.com/lance0/rustbgpd) uses a `gr-restart.toml` file with a versioned timestamp. Ze uses zefs instead of a raw file -- same concept, integrated storage.

## RFC Documentation

Add `// RFC 4724 Section 4.1: "<quoted requirement>"` above enforcing code.
MUST document: R-bit set on restart, restart-time extraction, marker expiry check, time-bounded R-bit.

## Implementation Summary

### What Was Implemented
- [To be filled after implementation]

### Bugs Found/Fixed
- [To be filled after implementation]

### Documentation Updates
- [To be filled after implementation]

### Deviations from Plan
- [To be filled after implementation]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |

### Tests from TDD Plan
| Test | Status | Location | Notes |

### Files from Plan
| File | Status | Notes |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `docs/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.

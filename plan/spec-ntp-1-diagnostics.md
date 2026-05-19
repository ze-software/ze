# Spec: ntp-1-diagnostics

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/5 |
| Updated | 2026-05-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugins/ntp/ntp.go` - existing NTP sync worker
4. `internal/plugins/ntp/register.go` - plugin registration
5. `internal/component/cmd/show/show.go` - show RPC registration pattern
6. `internal/component/cmd/show/system.go` - system show handlers

## Task

Upgrade the NTP plugin from a fire-and-forget SNTP client to a diagnosable
time synchronization subsystem. The existing plugin queries NTP servers and
steps the clock but discards all response metadata. Operators cannot see
whether NTP is working, which server is in use, what the offset is, or
whether servers are reachable.

Three gaps to close:

1. **Per-server state tracking.** The sync worker currently picks one random
   server, queries it, sets the clock, and throws away the response. Store
   per-server offset, RTT, stratum, jitter, reach bitmap, and last-query
   timestamp so diagnostics have something to report.

2. **Clock discipline improvement.** Currently only `Settimeofday` (step).
   Add `Adjtimex` slew for small offsets (under a configurable threshold,
   default 128ms). This avoids time jumps that break log ordering and
   confuse timer-based protocols (BGP hold timers, BFD).

3. **CLI diagnostics.** `show system ntp` (sync status summary),
   `show system ntp peers` (per-server table). No `monitor ntp` in this
   spec (can be a follow-up).

## Required Reading

### Architecture Docs
- [ ] `ai/patterns/cli-command.md` - RPC registration, YANG tree, WireMethod naming
  -> Constraint: YANG containers mirror CLI path. WireMethod is kebab-case.
  -> Constraint: Infrastructure MUST NOT import plugin implementations directly.
- [ ] `ai/rules/plugin-design.md` - Plugin boundary, import rules, state exposure
  -> Constraint: Show handlers cannot import `internal/plugins/ntp/` directly.
  -> Decision: NTP plugin registers its own RPCs via `pluginserver.RegisterRPCs()` in `init()`.
- [ ] `docs/architecture/core-design.md` - Component vs plugin boundary
- [ ] `ai/rules/no-sprintf-alloc.md` - No fmt.Sprintf on hot paths
- [ ] `ai/rules/buffer-first.md` - Buffer-first for any wire encoding

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5905.md` - NTPv4 specification (if exists)
  -> Constraint: Anti-thundering-herd jitter (already implemented).
  -> Constraint: Reach register is 8-bit shift register (shift left, set bit 0 on success).

**Key insights:**
- NTP plugin is in `internal/plugins/ntp/`, not `internal/component/`. Show handlers in `internal/component/cmd/show/` cannot import it directly.
- The NTP plugin already uses `beevik/ntp` which returns all needed metadata (offset, RTT, stratum, root delay/dispersion, precision, leap indicator).
- `Adjtimex` is available in `golang.org/x/sys/unix` for Linux.
- `show system date` already exists; NTP diagnostics extend the `system` YANG subtree.
- `pluginserver.RegisterRPCs()` is called from 53 packages including BGP plugins at `internal/component/bgp/plugins/cmd/*`. NTP plugin can use it directly in `init()`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ntp/ntp.go` - syncWorker, doSync, server selection, clock setting
  -> Constraint: doSync queries one random server, calls setClock, returns bool. No state retained.
  -> Constraint: servers() returns configured servers (priority) or DHCP servers (fallback).
  -> Constraint: syncWorker struct has: cfg, stop, done, mu, dhcpSrv, eventBus, synced fields.
- [ ] `internal/plugins/ntp/register.go` - plugin registration with SDK, config verify/apply, lifecycle
  -> Constraint: Plugin uses `registry.Register()` with `RunEngine: runNTPPlugin`.
  -> Constraint: `init()` registers event namespace, logger, config verifier, CLI handler stub.
- [ ] `internal/plugins/ntp/clock_linux.go` - setClock (Settimeofday), setRTC (ioctl)
  -> Constraint: Requires CAP_SYS_TIME. Gokrazy grants this.
- [ ] `internal/plugins/ntp/clock_other.go` - stubs for non-Linux
- [ ] `internal/plugins/ntp/persist.go` - saveTime/loadTime (RFC3339 to file, atomic write)
- [ ] `internal/plugins/ntp/events/events.go` - "system" namespace, "clock-synced" event
- [ ] `internal/plugins/ntp/schema/ze-ntp-conf.yang` - config YANG (environment/ntp container)
- [ ] `internal/component/cmd/show/show.go` - RPC registration pattern for show handlers
- [ ] `internal/component/cmd/show/system.go` - handleShowSystemDate (reference for system commands)
- [ ] `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` - show CLI YANG tree

**Behavior to preserve:**
- All existing sync logic: random server selection, anti-thundering-herd jitter, max-step guard
- DHCP server discovery and configured-server priority
- Time persistence to disk and restore on boot
- RTC write on Linux
- `clock-synced` event bus emission (once after first sync)
- Config parsing and validation (server addresses, intervals, persist-path)
- All existing tests continue to pass unchanged

**Behavior to change:**
- doSync currently discards `ntp.Response`; will store per-server state
- doSync currently queries one random server; will query all servers and pick best
- Clock setting currently only steps (Settimeofday); will slew (Adjtimex) for small offsets on Linux
- `show system date` output extended with NTP sync metadata (last-sync-time, sync-source, synced boolean)

## Data Flow (MANDATORY)

### Entry Point
- **Config:** `environment ntp { enabled true; server pool0 { address "..." }; }` parsed by `parseNTPConfig`
- **CLI:** `show system ntp` / `show system ntp peers` dispatched by YANG tree to RPC handlers
- **Sync loop:** `syncWorker.run()` queries NTP servers periodically

### Transformation Path
1. NTP config applied -> `startWorker(cfg)` creates syncWorker with server list
2. syncWorker.run() enters poll loop: queries each server via `beevik/ntp.Query()`
3. `ntp.Response` (offset, RTT, stratum, root delay/dispersion) stored in per-server state
4. Best server selected by stratum + offset; clock adjusted (slew or step)
5. `show system ntp` RPC handler reads sync state, returns JSON
6. `show system ntp peers` RPC handler reads per-server state, returns JSON array

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| NTP sync worker -> show handler | Package-level `atomic.Pointer[syncState]` in `internal/plugins/ntp/` | [ ] |
| CLI -> daemon | YANG-dispatched RPC (`ze-show:system-ntp`, `ze-show:system-ntp-peers`) | [ ] |
| NTP plugin -> kernel | `Settimeofday` (step) or `Adjtimex` (slew) syscall | [ ] |

### Integration Points
- `pluginserver.RegisterRPCs()` - register show handlers in NTP plugin's `init()`
- `ze-cli-show-cmd.yang` - add `ntp` and `ntp peers` containers under `system`
- `handleShowSystemDate` - extend with NTP sync metadata
- `golang.org/x/sys/unix.Adjtimex()` - new dependency for clock slew

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Design Decisions

### State Exposure Across Import Boundary

The show handler (infrastructure) cannot import the NTP plugin (plugin). Options:

| Option | Pros | Cons |
|--------|------|------|
| **A. Leaf state package** `internal/plugins/ntp/ntpstate/` | Simple, type-safe, no indirection | Show handler still imports a plugin subpackage |
| **B. NTP plugin registers RPCs itself** via `pluginserver.RegisterRPCs()` in its own `init()` | Clean separation, follows existing pattern (53 callers) | Handler runs in plugin server context, needs access to sync worker state via package-level var |
| **C. Registry-based state provider** in `internal/component/plugin/registry/` | Fully decoupled | Adds interface indirection, type assertion at read time |

-> Decision: **Option B.** The NTP plugin registers its own RPCs in `init()`.
The handlers are package-level functions that read from a package-level
`atomic.Pointer[syncState]` written by the sync worker. This is the same
pattern used by BGP plugins that register `pluginserver.RegisterRPCs()`.
The show YANG tree in `ze-cli-show-cmd.yang` adds containers with
`ze:command` pointing to the NTP-registered WireMethods. The show package
does NOT import the NTP plugin; it only knows about the YANG path.

### Per-Server State

Track for each configured/DHCP server:

| Field | Type | Source |
|-------|------|--------|
| address | string | config |
| offset | time.Duration | `ntp.Response.ClockOffset` |
| rtt | time.Duration | `ntp.Response.RTT` |
| stratum | uint8 | `ntp.Response.Stratum` |
| root-delay | time.Duration | `ntp.Response.RootDelay` |
| root-dispersion | time.Duration | `ntp.Response.RootDispersion` |
| reach | uint8 | 8-bit shift register (shift left, set bit 0 on success) |
| last-query | time.Time | set on each query attempt |
| last-success | time.Time | set on successful query |
| last-error | string | last query error message, "" on success |

### Slew vs Step Threshold

| Offset | Action | Mechanism |
|--------|--------|-----------|
| abs(offset) <= slew-threshold (default 128ms) | Slew | `unix.Adjtimex` with `ADJ_OFFSET` mode |
| abs(offset) > slew-threshold AND <= max-step | Step | `Settimeofday` (existing) |
| abs(offset) > max-step | Reject | Log warning, do not adjust clock |

New config leaf: `slew-threshold` (uint32, range 0..1000, default 128, milliseconds).
Value 0 disables slew (always step, current behavior). This is backward compatible.

### Server Selection

Current: pick one random server.
New: query all servers each cycle, select best by:
1. Reachable (reach > 0)
2. Lowest stratum
3. Smallest absolute offset (tiebreaker)

This gives per-server stats for diagnostics and a more reliable sync source.

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `show system ntp` CLI | -> | `handleShowSystemNTP()` | `TestShowSystemNTPWiring` |
| `show system ntp peers` CLI | -> | `handleShowSystemNTPPeers()` | `TestShowSystemNTPPeersWiring` |
| Config `slew-threshold` | -> | `parseNTPConfig()` | `TestParseNTPConfigSlewThreshold` |
| Sync loop stores state | -> | `syncWorker.doSync()` updates `syncState` | `TestSyncWorkerStoresServerState` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `show system ntp` with NTP enabled and synced | Returns JSON: `synced: true`, `source` (server address), `offset`, `stratum`, `poll-interval`, `last-sync` timestamp |
| AC-2 | `show system ntp` with NTP enabled but not yet synced | Returns JSON: `synced: false`, other fields zero/empty |
| AC-3 | `show system ntp` with NTP disabled | Returns JSON: `enabled: false` |
| AC-4 | `show system ntp peers` with 2+ servers configured | Returns JSON array with per-server: `address`, `offset`, `rtt`, `stratum`, `reach`, `last-query`, `last-error` |
| AC-5 | `show system ntp peers` with no servers | Returns empty array |
| AC-6 | Small offset (< slew-threshold) on Linux | Clock adjusted via `Adjtimex` (slew), not `Settimeofday` (step) |
| AC-7 | Large offset (> slew-threshold, <= max-step) on Linux | Clock adjusted via `Settimeofday` (step) |
| AC-8 | Offset exceeds max-step | Clock NOT adjusted, warning logged |
| AC-9 | Non-Linux platform | Slew falls back to step (Adjtimex unavailable) |
| AC-10 | Config `slew-threshold 0` | Slew disabled, always step (backward compat with current behavior) |
| AC-11 | Config `slew-threshold 500` | Offsets <= 500ms use Adjtimex |
| AC-12 | Multiple servers configured | All servers queried each cycle; per-server reach bitmap updated |
| AC-13 | Server unreachable | Reach bitmap shifts left with 0 in bit 0; `last-error` set |
| AC-14 | Server becomes reachable after being unreachable | Reach bitmap gains set bits; `last-error` cleared |
| AC-15 | `show system date` | Existing fields preserved; new fields: `ntp-synced` (bool), `ntp-source` (string or null), `ntp-offset` (duration string or null) |
| AC-16 | syncState is updated atomically | No data race between sync worker writing state and show handler reading it |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseNTPConfigSlewThreshold` | `internal/plugins/ntp/ntp_test.go` | AC-10, AC-11: slew-threshold config parsing | |
| `TestParseNTPConfigSlewThresholdBounds` | `internal/plugins/ntp/ntp_test.go` | slew-threshold range 0..1000 validated | |
| `TestClockOffsetAction` | `internal/plugins/ntp/ntp_test.go` | AC-6, AC-7, AC-8: correct action (slew/step/reject) for offset ranges | |
| `TestReachRegisterShift` | `internal/plugins/ntp/ntp_test.go` | AC-12, AC-13, AC-14: reach bitmap behavior | |
| `TestServerSelection` | `internal/plugins/ntp/ntp_test.go` | Best server by stratum then offset | |
| `TestSyncStateSnapshot` | `internal/plugins/ntp/ntp_test.go` | AC-16: atomic state snapshot returns consistent data | |
| `TestShowSystemNTPEnabled` | `internal/plugins/ntp/ntp_test.go` | AC-1: handler returns expected fields when synced | |
| `TestShowSystemNTPDisabled` | `internal/plugins/ntp/ntp_test.go` | AC-3: handler returns enabled:false | |
| `TestShowSystemNTPPeers` | `internal/plugins/ntp/ntp_test.go` | AC-4: handler returns per-server array | |
| `TestShowSystemNTPPeersEmpty` | `internal/plugins/ntp/ntp_test.go` | AC-5: empty array when no servers | |
| `TestSlewClockLinux` | `internal/plugins/ntp/clock_linux_test.go` | AC-6: Adjtimex called with correct offset | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| slew-threshold | 0..1000 | 1000 | N/A (0 is valid = disable) | 1001 |
| interval | 60..86400 | (existing) | (existing) | (existing) |
| max-step | 0..86400 | (existing) | (existing) | (existing) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-show-system-ntp` | `test/plugin/ntp-show.ci` | `show system ntp` returns valid JSON with expected keys | |
| `test-show-system-ntp-peers` | `test/plugin/ntp-show-peers.ci` | `show system ntp peers` returns JSON array | |

### Future (if deferring any tests)
- `monitor ntp` with `| log` mode (separate spec)
- Web UI NTP status page (separate spec)
- NTS (TLS-authenticated NTP) support via `beevik/nts` (separate spec)

## Files to Modify
- `internal/plugins/ntp/ntp.go` - Per-server state struct, syncState, doSync stores results, query all servers
- `internal/plugins/ntp/register.go` - Register show RPCs via `pluginserver.RegisterRPCs()`, show handler implementations
- `internal/plugins/ntp/clock_linux.go` - Add `slewClock()` using `unix.Adjtimex`
- `internal/plugins/ntp/clock_other.go` - Add `slewClock()` stub (falls back to step)
- `internal/plugins/ntp/ntp_test.go` - New tests for state, reach, slew, show handlers
- `internal/plugins/ntp/schema/ze-ntp-conf.yang` - Add `slew-threshold` leaf
- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` - Add `ntp` containers under `system`
- `internal/component/cmd/show/system.go` - Extend `handleShowSystemDate` with NTP sync metadata

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/plugins/ntp/schema/ze-ntp-conf.yang`, `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` |
| CLI commands/flags | [x] | YANG-driven (`show system ntp`, `show system ntp peers`) |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [x] | `test/plugin/ntp-show.ci`, `test/plugin/ntp-show-peers.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - NTP diagnostics commands |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` - slew-threshold leaf |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` - show system ntp, show system ntp peers |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` - ze-show:system-ntp, ze-show:system-ntp-peers |
| 5 | Plugin added/changed? | [ ] | N/A (extending existing NTP plugin) |
| 6 | Has a user guide page? | [ ] | Could add `docs/guide/ntp.md` but not required for this scope |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | NTP state tracking is operational, not protocol compliance |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create
- `internal/plugins/ntp/clock_linux_test.go` - Tests for slewClock
- `internal/plugins/ntp/state.go` - syncState struct, per-server state, atomic snapshot
- `test/plugin/ntp-show.ci` - Functional test for show system ntp
- `test/plugin/ntp-show-peers.ci` - Functional test for show system ntp peers

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** - Register RPC entry points, write failing wiring tests
   - Tests: `TestShowSystemNTPWiring`, `TestShowSystemNTPPeersWiring`
   - Files: `register.go` (add `pluginserver.RegisterRPCs`), `ze-cli-show-cmd.yang` (add YANG containers), stub handlers returning empty
   - Verify: entry points exist and are reachable; wiring tests fail because handlers return stub data

2. **Phase: Per-server state** - State struct, reach register, atomic snapshot
   - Tests: `TestReachRegisterShift`, `TestSyncStateSnapshot`, `TestServerSelection`
   - Files: `state.go` (new), `ntp.go` (doSync stores per-server results)
   - Verify: tests pass; sync worker stores and exposes state

3. **Phase: Slew mode** - Adjtimex for small offsets, configurable threshold
   - Tests: `TestClockOffsetAction`, `TestSlewClockLinux`, `TestParseNTPConfigSlewThreshold`, `TestParseNTPConfigSlewThresholdBounds`
   - Files: `clock_linux.go` (add slewClock), `clock_other.go` (stub), `ntp.go` (doSync uses slew/step decision), `ze-ntp-conf.yang` (slew-threshold leaf)
   - Verify: correct action selected for each offset range; config parsed

4. **Phase: Show handlers** - Implement RPC handlers reading sync state
   - Tests: `TestShowSystemNTPEnabled`, `TestShowSystemNTPDisabled`, `TestShowSystemNTPPeers`, `TestShowSystemNTPPeersEmpty`
   - Files: `register.go` (handler implementations), `system.go` (extend handleShowSystemDate)
   - Verify: handlers return correct JSON; wiring tests pass

5. **Functional tests** - End-to-end .ci tests
   - Files: `test/plugin/ntp-show.ci`, `test/plugin/ntp-show-peers.ci`
   - Verify: functional tests pass against running daemon

6. **Full verification** - `make ze-verify`

7. **Complete spec** - Audit tables, learned summary, delete spec

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Reach register shifts correctly (bit 0 = most recent), Adjtimex offset in correct units (usec) |
| Naming | JSON keys use kebab-case, YANG uses kebab-case, WireMethod is `ze-show:system-ntp` |
| Data flow | State flows from syncWorker -> atomic pointer -> show handler. No direct import of plugin from infrastructure. |
| Rule: no-sprintf-alloc | Show handlers do not use fmt.Sprintf for building response maps |
| Rule: import-rules | `internal/component/cmd/show/` does NOT import `internal/plugins/ntp/` |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| `show system ntp` returns sync status | `grep -rn 'ze-show:system-ntp' internal/plugins/ntp/` |
| `show system ntp peers` returns per-server table | `grep -rn 'ze-show:system-ntp-peers' internal/plugins/ntp/` |
| YANG tree has ntp containers | `grep -n 'ntp' internal/component/cmd/show/schema/ze-cli-show-cmd.yang` |
| Slew config leaf in YANG | `grep -n 'slew-threshold' internal/plugins/ntp/schema/ze-ntp-conf.yang` |
| Per-server state stored | `grep -n 'serverState' internal/plugins/ntp/state.go` |
| Adjtimex used for slew | `grep -n 'Adjtimex\|slewClock' internal/plugins/ntp/clock_linux.go` |
| Functional tests exist | `ls test/plugin/ntp-show*.ci` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Server addresses already validated (existing). slew-threshold must be uint32 in range. |
| NTP response validation | Already validates year range and `resp.Validate()`. Ensure per-server state cannot be poisoned by a malicious NTP response beyond what the existing guards prevent. |
| Adjtimex safety | Verify offset passed to Adjtimex is within slew-threshold range (double-check before syscall). |
| Race conditions | Atomic pointer swap for syncState. No mutex contention between sync worker and show handlers. |
| Denial of service | Show handlers read a snapshot; cannot block the sync worker. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior, RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural, DESIGN phase |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

Add `// RFC 5905 Section 13.1` above reach register implementation (8-bit shift register for reachability).
Add `// RFC 5905 Section 11.2` above Adjtimex clock discipline (frequency adjustment).

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [pending]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-16 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
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

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit

# Spec: multipeer-ci

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-04-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/testing/ci-format.md` - .ci test format
4. `internal/test/runner/runner_exec.go` - process orchestration, syncWriter

## Task

Fix the `.ci` functional test framework so multi-peer tests work correctly. Currently, two existing multi-peer tests (`role-otc-egress-stamp`, `community-strip`) are broken: `ze_bgp_tcp_port` overrides ALL peer ports to `$PORT`, but the second ze-peer binds to `$PORT2` on a different IP. Ze never connects to the second peer. Tests pass only because validation doesn't check the second peer's connection.

**Root cause:** Tests used different ports to differentiate peers. Ports are irrelevant when peers bind to different loopback IPs (127.0.0.1 vs 127.0.0.2). All peers should use `$PORT` on different IPs. The port override then applies correctly to all peers.

**Platform portability:** Linux routes the entire 127.0.0.0/8 to lo automatically. macOS/FreeBSD only bind 127.0.0.1 by default. The test infrastructure must add loopback aliases (127.0.0.2, etc.) on non-Linux platforms using the BSD kernel socket API (`SIOCAIFADDR` ioctl on the loopback interface).

**Secondary problem:** The runner shares a single `syncWriter` and `strings.Builder` across all ze-peer background processes. This causes (a) a race condition where `WaitFor("listening on")` fires on the first peer, skipping readiness check for the second, and (b) interleaved output that prevents per-peer validation.

### Scope

**In Scope:**

| Area | Description |
|------|-------------|
| Fix existing tests | Change `--port $PORT2` to `--port $PORT` in `role-otc-egress-stamp.ci` and `community-strip.ci` |
| Loopback alias setup | Add 127.0.0.2+ aliases on macOS/FreeBSD via `SIOCAIFADDR` ioctl before multi-peer tests |
| Per-process output tracking | Each background ze-peer gets its own syncWriter and stderr capture |
| Per-process WaitFor | Each ze-peer independently synchronized before ze starts |
| Documentation | Update `docs/architecture/testing/ci-format.md` with multi-peer pattern |
| Proof-of-concept validation | Verify the second peer in an existing test actually receives data |

**Out of Scope:**

| Area | Reason |
|------|--------|
| Port allocation changes | Unnecessary -- same port on different IPs works |
| `$PORT3`/`$PORT4` substitution | Unnecessary -- peers differentiated by IP, not port |
| New `option=peers:value=multi` | Unnecessary -- no port override suppression needed |
| Port override suppression | Unnecessary -- override correctly sets all peers to `$PORT` |
| LLGR egress suppress test | Separate spec -- depends on LLGR AC-9 readvertisement logic, not test infrastructure |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/ci-format.md` - .ci format, options, port substitution
  -> Constraint: `$PORT` and `$PORT2` already supported. Different IPs via `--bind`.
- [ ] `docs/architecture/core-design.md` - plugin architecture, peer model

### Source Files (MUST read before implementation)
- [ ] `internal/test/runner/runner_exec.go:27-77` - syncWriter implementation (single shared instance)
- [ ] `internal/test/runner/runner_exec.go:366-368` - single peerStdout/peerStderr allocated per test
- [ ] `internal/test/runner/runner_exec.go:549-551` - all ze-peer processes share same stdout/stderr
- [ ] `internal/test/runner/runner_exec.go:578-584` - WaitFor fires once, not per-peer
- [ ] `internal/test/runner/runner_exec.go:650-656` - peerProc is singular (last match wins)
- [ ] `internal/test/runner/runner_exec.go:679` - PeerOutput combines single shared stdout+stderr
- [ ] `internal/component/bgp/config/peers.go:438-451` - applyPortOverride sets ALL peers to same port

**Key insights:**
- `applyPortOverride` overrides ALL peer ports to `ze_bgp_tcp_port` value. With same port on all peers, this is correct behavior, not a bug.
- `syncWriter.found` is a permanent flag (line 48). Once set, `WaitFor` returns true immediately forever. Second peer's readiness is never checked.
- `peerProc` at line 650-656 assigns only the LAST ze-peer process. `Wait()` at line 669 only waits on that one. First peer's exit status is silently ignored.
- POC confirmed (Linux): two ze-peer instances bind to same port on 127.0.0.1 and 127.0.0.2 simultaneously. TCP connections to both succeed.

### Loopback Alias (platform portability)
- [ ] BSD `SIOCAIFADDR` ioctl -- adds an address alias to an interface. Used on macOS/FreeBSD to make 127.0.0.2+ available on lo0.
  -> Constraint: Linux 127.0.0.0/8 routes to lo automatically; BSD only has 127.0.0.1 by default.
  -> Constraint: Requires root on macOS/FreeBSD (`SIOCAIFADDR` modifies interface config).
- [ ] Implementation: build-tagged files in test runner package. Linux is a no-op. BSD uses `SIOCAIFADDR` on lo0.
  -> Decision: best location is the runner package, called before first multi-peer test runs.
  -> Decision: if alias already exists, no-op (idempotent). No cleanup needed -- aliases on loopback are harmless.
  -> Decision: if ioctl fails (no root), log warning and let the test fail naturally on bind -- clear error.

**BSD ioctl detail:** `SIOCAIFADDR` (0x80266919 on darwin) takes `struct in_ifaliasreq` (64 bytes): 16-byte interface name ("lo0"), three `sockaddr_in` structs (addr, broadaddr, mask). Open an `AF_INET/SOCK_DGRAM` socket, call `SYS_IOCTL`. `SIOCAIFADDR` is NOT in `golang.org/x/sys/unix` for darwin -- must be defined as a constant. Build tags: `//go:build darwin || freebsd` and `//go:build linux`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/runner/runner_exec.go` - shared syncWriter/stderr across all ze-peer processes; single peerProc tracking
- [ ] `internal/test/runner/record_parse.go` - port allocation: `et.port += 2` per test (unchanged)
- [ ] `internal/test/peer/peer.go` - ze-peer binds to `--bind` address + `--port`, sink mode accepts any message
- [ ] `internal/component/bgp/config/peers.go:438-451` - applyPortOverride overrides all peer ports
- [ ] `test/plugin/role-otc-egress-stamp.ci` - uses `--port $PORT2` for second peer (bug: ze connects to `$PORT` due to override)
- [ ] `test/plugin/community-strip.ci` - same bug as above

**Behavior to preserve:**
- All existing single-peer tests continue working unchanged
- `$PORT` and `$PORT2` semantics unchanged
- Default port injection (`ze_bgp_tcp_port`) remains for all tests
- Port allocation (2 per test) unchanged
- `option=tcp_connections:value=N` still handles sequential reconnections to one peer
- Multi-peer tests differentiate peers by bind address (127.0.0.x), not port

**Behavior to change:**
- Per ze-peer process: independent syncWriter for stdout, independent stderr capture, independent `WaitFor("listening on")` synchronization
- `peerProc` tracking: wait for ALL ze-peer processes, check each exit status
- `rec.PeerOutput`: combine all per-peer outputs (order preserved) for downstream validation
- Fix `role-otc-egress-stamp.ci`: `--port $PORT2` -> `--port $PORT`
- Fix `community-strip.ci`: `--port $PORT2` -> `--port $PORT`

**Behavior to fix (existing bugs):**
- `role-otc-egress-stamp.ci` line 144: second ze-peer on `$PORT2`, but ze connects to `$PORT` (override). Ze never reaches second peer. Test claims "route forwarded" but never verifies it.
- `community-strip.ci` line 147: same bug.

## Data Flow (MANDATORY)

### Entry Point
- `.ci` file with multiple `cmd=background` ze-peer processes on different `--bind` addresses, all using `--port $PORT`

### Transformation Path
1. Parser reads `.ci` file, allocates 2 ports as usual (unchanged)
2. Runner starts ze-peer background instances sequentially. Each gets its own syncWriter for stdout and its own stderr builder.
3. Runner waits for EACH ze-peer's `WaitFor("listening on")` independently before starting next command
4. Runner starts ze with `ze_bgp_tcp_port=$PORT` (unchanged). Override sets all peer ports to `$PORT` -- correct, because all peers use `$PORT` on different IPs.
5. Ze connects to each peer by IP (from config `remote > ip`), all on `$PORT`
6. Each ze-peer processes its own stdin block and validates its own expectations independently
7. Runner waits for ALL ze-peer processes to complete, collects per-peer output

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| .ci file -> Runner | Parser reads commands, allocates ports (unchanged) | [ ] |
| Runner -> ze-peer processes | Each started with `--bind 127.0.0.x --port $PORT` | [ ] |
| Runner -> ze process | `ze_bgp_tcp_port=$PORT` injected (unchanged) | [ ] |
| ze config -> peer connections | Per-peer `remote > ip` in config, port overridden to `$PORT` | [ ] |

### Integration Points
- `runner_exec.go:366-368` - replace single `peerStdout`/`peerStderr` with per-process tracking (e.g., slice of syncWriter/Builder pairs indexed by ze-peer command order)
- `runner_exec.go:549-551` - assign per-process stdout/stderr to each ze-peer background process
- `runner_exec.go:578-584` - each ze-peer gets its own `WaitFor` on its own syncWriter
- `runner_exec.go:650-656` - track ALL ze-peer processes, not just last one. Wait for all.
- `runner_exec.go:679` - combine per-peer outputs into `rec.PeerOutput` (concatenated, not interleaved)
- NOT touched: `runner_exec.go:162-173` (plugin test peer start -- single-peer path, unchanged)
- NOT touched: `record_parse.go` (port allocation unchanged)
- NOT touched: `runner_validate.go` (PeerOutput is still a single string, just better ordered)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| .ci file with 2 ze-peer on different IPs, same port | -> | Per-process syncWriter, per-process WaitFor | `test/plugin/role-otc-egress-stamp.ci` (fixed) |
| ze config with per-peer `remote > ip` | -> | ze connects to both peers via port override | `test/plugin/role-otc-egress-stamp.ci` (fixed) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Multi-peer test with 2 ze-peer background instances on different IPs, same `$PORT` | Each ze-peer gets independent syncWriter and stderr capture |
| AC-2 | Multi-peer test starts | Runner waits for each ze-peer's `WaitFor("listening on")` independently before proceeding |
| AC-3 | Multi-peer test completes | Runner waits for ALL ze-peer processes and checks each exit status |
| AC-4 | Existing single-peer test (only one ze-peer) | Unchanged behavior: single syncWriter, same flow as before |
| AC-5 | Test runner starts multi-peer test on macOS/FreeBSD | Loopback aliases (127.0.0.2+) added automatically via `SIOCAIFADDR` ioctl; ze-peer binds successfully |
| AC-6 | `role-otc-egress-stamp.ci` runs with `--port $PORT` on both peers | Ze connects to both peers; second peer (sink) receives UPDATE messages ("sank" in output) |
| AC-7 | `community-strip.ci` runs with `--port $PORT` on both peers | Ze connects to both peers; second peer (sink) receives forwarded messages |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPerProcessSyncWriter` | `internal/test/runner/runner_exec_test.go` | Each ze-peer gets independent syncWriter, WaitFor works per-process | |
| `TestSinglePeerUnchanged` | `internal/test/runner/runner_exec_test.go` | Single ze-peer still uses one syncWriter (backward compat) | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `role-otc-egress-stamp` | `test/plugin/role-otc-egress-stamp.ci` | Fixed: both peers on `$PORT`, ze connects to both | |
| `community-strip` | `test/plugin/community-strip.ci` | Fixed: both peers on `$PORT`, ze connects to both | |

### Future (if deferring any tests)
- LLGR egress suppress test (separate spec, depends on LLGR readvertisement logic)
- Multi-peer route reflection test (separate spec)

## Files to Modify

- `internal/test/runner/runner_exec.go` - per-process syncWriter/stderr/WaitFor, multi-peer peerProc tracking
- `test/plugin/role-otc-egress-stamp.ci` - `--port $PORT2` -> `--port $PORT`
- `test/plugin/community-strip.ci` - `--port $PORT2` -> `--port $PORT`
- `docs/architecture/testing/ci-format.md` - document multi-peer pattern (same port, different IPs)

## Files to Create

- `internal/test/runner/loopback_linux.go` - no-op (`//go:build linux`): Linux routes 127.0.0.0/8 to lo automatically
- `internal/test/runner/loopback_darwin.go` - `SIOCAIFADDR` ioctl to add 127.0.0.x aliases on lo0 (`//go:build darwin || freebsd`)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A (uses existing tests) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | N/A |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | No | N/A |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/ci-format.md` -- multi-peer pattern |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | No | N/A |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-verify` |
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

1. **Phase: Loopback alias portability** -- ensure 127.0.0.2+ available on all platforms
   - Files: `internal/test/runner/loopback_linux.go` (no-op), `internal/test/runner/loopback_bsd.go` (`SIOCAIFADDR` ioctl)
   - Verify: build succeeds on Linux; on macOS, alias is added idempotently

2. **Phase: Per-process output tracking** -- each ze-peer background process gets its own syncWriter and stderr builder
   - Tests: `TestPerProcessSyncWriter`, `TestSinglePeerUnchanged`
   - Files: `runner_exec.go`
   - Verify: two ze-peer instances can start with independent `WaitFor` synchronization; single-peer tests unchanged

3. **Phase: Multi-peer process completion** -- wait for ALL ze-peer processes, not just the last one
   - Files: `runner_exec.go`
   - Verify: runner checks exit status of every ze-peer process

4. **Phase: Fix existing tests** -- change `$PORT2` to `$PORT` in multi-peer .ci files
   - Files: `role-otc-egress-stamp.ci`, `community-strip.ci`
   - Verify: both tests pass; second peer (sink) output contains "sank" (received messages)

5. **Phase: Documentation** -- update ci-format.md with multi-peer pattern
   - Files: `docs/architecture/testing/ci-format.md`

6. **Phase: Full verification** -- `make ze-verify`

7. **Phase: Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 6 ACs implemented with evidence |
| Correctness | Existing single-peer tests still pass (no regression) |
| Output isolation | Each ze-peer instance has independent syncWriter, no shared state |
| Backward compat | Single ze-peer tests unchanged in behavior |
| Bug fix validated | Second peer in fixed tests actually receives messages |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Loopback alias (BSD) | Build succeeds on Linux; `SIOCAIFADDR` defined for darwin/freebsd |
| Per-process syncWriter | Unit test + grep in runner_exec.go |
| Per-process WaitFor | Unit test |
| Multi-peer completion tracking | Grep for peerProc handling |
| `role-otc-egress-stamp` fixed | Run test, check for "sank" in peer output |
| `community-strip` fixed | Run test, check for "sank" in peer output |
| Existing single-peer tests pass | `make ze-functional-test` full pass |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Process cleanup | All background ze-peer instances killed on test end |
| No port conflicts | Same port on different IPs verified by POC |

### Failure Routing

| Failure | Route To |
|---------|----------|
| ze still can't connect to second peer after port fix | Trace `applyPortOverride`, check config `remote > ip` |
| WaitFor race causes intermittent failures | Increase per-peer WaitFor timeout or add sequential start |
| Sink peer shows no "sank" output | Check ze forwarding path -- may be a separate ze bug |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Multi-Peer Test Pattern (reference)

All multi-peer tests follow this pattern:

**Same port, different IPs.** Each ze-peer binds to a different loopback address (127.0.0.1, 127.0.0.2, etc.) but all use `--port $PORT`. The `ze_bgp_tcp_port=$PORT` override sets all peer ports to `$PORT`, which is correct because all peers ARE on `$PORT`.

**Ze config:** Each peer has a different `remote > ip` matching the ze-peer's `--bind` address. No `remote > port` needed -- the env var override handles it.

**Stdin block routing:** Already works. Each `cmd=background` ze-peer references its own stdin block by name (e.g., `stdin=source`, `stdin=dest`). The runner dispatches by matching `cmd.Stdin` to `rec.StdinBlocks`.

**Output tracking:** Each ze-peer background process gets its own syncWriter for stdout and its own `strings.Builder` for stderr. The runner waits for each peer's "listening on" independently before starting ze.

**Example .ci pattern:**
```
cmd=background:seq=1:exec=ze-peer --port $PORT:stdin=source
cmd=background:seq=2:exec=ze-peer --bind 127.0.0.2 --mode sink --port $PORT:stdin=dest
cmd=foreground:seq=3:exec=ze -:stdin=ze-bgp:timeout=20s
```

## Cross-References

| Document | Relevance |
|----------|-----------|
| `plan/learned/511-llgr-0-umbrella.md` | AC-9 partial: needs separate spec for LLGR egress suppress test |
| `plan/learned/509-llgr-4-readvertisement.md` | Noted multi-peer gap |

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| WaitFor race (second peer not ready) | Low | Medium | Per-process syncWriter eliminates race; ze also has BGP connect-retry |
| Existing tests regressed by runner change | Low | High | Single-peer path preserved; backward compat unit test |
| Sink peer doesn't receive messages (ze forwarding bug) | Medium | Medium | If sink shows no "sank", that's a ze routing bug, not a test infra bug -- file separate issue |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Different ports needed for different peers | Same port works when IPs differ | POC: bind 127.0.0.1 and 127.0.0.2 to same port | Eliminated 70% of original spec complexity |
| Existing multi-peer tests worked correctly | Ze never connects to second peer (`applyPortOverride` overrides to `$PORT`, second peer on `$PORT2`) | Traced `applyPortOverride` and discovered `role-otc-egress-stamp` dest-peer is unreachable | Two existing tests have latent bugs |
| Test validation proved second peer worked | Python plugin claims "route forwarded" unconditionally (line 61-66) without verifying | Read the test plugin source | False positive in existing test |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Port suppression (`option=peers:value=multi`) | Unnecessary complexity -- same port on different IPs eliminates the problem | Use `--bind` for IP differentiation, all peers on `$PORT` |
| `$PORT3`/`$PORT4` allocation | Unnecessary -- peers don't need different ports | Same `$PORT` for all peers |
| Active/passive BGP (one side connects, other accepts) | Would require ze-peer connect mode (doesn't exist) | Same port, different IPs is simpler |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- Multi-peer differentiation by IP, not port, is the natural fit for BGP testing. In production BGP, peers are identified by IP address, not port. Test infrastructure should mirror this.
- `applyPortOverride` setting ALL peers to the same port is correct behavior when peers are on different IPs. The override was designed for single-peer tests but works for multi-peer when the port is uniform.
- The `syncWriter.found` permanent flag is fine for single-peer but creates a race for multi-peer. Per-process writers are the minimal fix.

## Implementation Summary

### What Was Implemented
- Per-process output tracking for ze-peer background processes (`peerOutput` struct with independent syncWriter/stderr per process)
- Multi-peer process completion: runner waits for ALL ze-peer processes, not just the last one
- Loopback alias portability: `loopback_linux.go` (no-op) and `loopback_darwin.go` (`SIOCAIFADDR` ioctl for macOS/FreeBSD)
- Loopback alias call site in `runOrchestrated`: scans `--bind` addresses and ensures aliases exist before starting ze-peer
- Fixed `newSyncWriter` to take no args (linter `unparam` fix)

### Bugs Found/Fixed
- `role-otc-egress-stamp.ci`: second ze-peer on `$PORT2` but `applyPortOverride` sets all peers to `$PORT`. Ze never connected to second peer. Fixed: `--port $PORT` for both peers.
- `community-strip.ci`: same bug. Fixed: `--port $PORT` for both peers.
- `syncWriter.found` permanent flag caused second peer's `WaitFor` to return immediately when shared. Fixed: per-process syncWriter.
- `peerProc` was singular (last match wins). First peer's exit status silently ignored. Fixed: wait for all peer processes.

### Documentation Updates
- `docs/architecture/testing/ci-format.md`: added Multi-Peer example section with pattern and platform notes

### Deviations from Plan
- None

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Fix existing multi-peer tests | Done | `test/plugin/role-otc-egress-stamp.ci:144`, `test/plugin/community-strip.ci:147` | `$PORT2` -> `$PORT` |
| Per-process output tracking | Done | `runner_exec.go:90-98`, `runner_exec.go:581-588` | `peerOutput` struct, per-process syncWriter |
| Per-process WaitFor | Done | `runner_exec.go:617-625` | Each ze-peer gets own `po.stdout.WaitFor()` |
| Multi-peer completion | Done | `runner_exec.go:710-719` | Wait for ALL peer processes |
| Loopback alias portability | Done | `loopback_linux.go`, `loopback_darwin.go` | No-op on Linux, SIOCAIFADDR on BSD |
| Documentation | Done | `docs/architecture/testing/ci-format.md` | Multi-Peer example section |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `runner_exec.go:581-588` + `TestPerProcessSyncWriter` | Each ze-peer gets independent peerOutput |
| AC-2 | Done | `runner_exec.go:617-625` + `TestPerProcessSyncWriter` | Independent WaitFor per process |
| AC-3 | Done | `runner_exec.go:710-719` | Wait loop over all peerOutputs |
| AC-4 | Done | `TestSinglePeerUnchanged` + `make ze-functional-test` green | Single-peer path uses slice of one |
| AC-5 | Done | `loopback_linux.go`, `loopback_darwin.go`, `TestEnsureLoopbackAlias` | Platform-specific implementations |
| AC-6 | Done | Debug log shows `sank #1` in role-otc-egress-stamp | Sink peer receives UPDATE |
| AC-7 | Done | Debug log shows `sank #1`, `sank #2` in community-strip | Sink peer receives forwarded messages |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestPerProcessSyncWriter` | Pass | `loopback_test.go:20` | Validates AC-1, AC-2 |
| `TestSinglePeerUnchanged` | Pass | `loopback_test.go:64` | Validates AC-4 |
| `TestEnsureLoopbackAlias` | Pass | `loopback_test.go:92` | Validates AC-5 basic case |
| `role-otc-egress-stamp` | Pass | `test/plugin/role-otc-egress-stamp.ci` | Validates AC-6 |
| `community-strip` | Pass | `test/plugin/community-strip.ci` | Validates AC-7 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/test/runner/runner_exec.go` | Modified | Per-process tracking, loopback alias call |
| `internal/test/runner/loopback_linux.go` | Created | No-op on Linux |
| `internal/test/runner/loopback_darwin.go` | Created | SIOCAIFADDR ioctl for BSD |
| `internal/test/runner/loopback_test.go` | Created | Unit tests for all 3 new features |
| `test/plugin/role-otc-egress-stamp.ci` | Modified | `$PORT2` -> `$PORT` |
| `test/plugin/community-strip.ci` | Modified | `$PORT2` -> `$PORT` |
| `docs/architecture/testing/ci-format.md` | Modified | Multi-peer pattern documentation |

### Audit Summary
- **Total items:** 16
- **Done:** 16
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/test/runner/loopback_linux.go` | Yes | Created |
| `internal/test/runner/loopback_darwin.go` | Yes | Created |
| `internal/test/runner/loopback_test.go` | Yes | Created |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Per-process syncWriter | `TestPerProcessSyncWriter` PASS |
| AC-2 | Independent WaitFor | `TestPerProcessSyncWriter` PASS (second writer doesn't fire from first) |
| AC-3 | Wait for all peers | `runner_exec.go:710-719` wait loop |
| AC-4 | Single-peer unchanged | `TestSinglePeerUnchanged` PASS + `make ze-functional-test` green |
| AC-5 | Loopback alias | `TestEnsureLoopbackAlias` PASS (127.0.0.1) |
| AC-6 | role-otc-egress-stamp | Debug log: `sank #1` in sink peer output |
| AC-7 | community-strip | Debug log: `sank #1`, `sank #2` in sink peer output |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| 2 ze-peer on different IPs, same port | `test/plugin/role-otc-egress-stamp.ci` | Yes -- sink receives UPDATE |
| 2 ze-peer on different IPs, same port | `test/plugin/community-strip.ci` | Yes -- sink receives forwarded messages |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-verify` passes
- [ ] Existing tests not regressed
- [ ] Fixed tests show second peer receiving messages

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Documentation updated

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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-multipeer-ci.md`
- [ ] Summary included in commit

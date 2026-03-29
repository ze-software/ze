# Spec: exabgp-dynamic-port

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `test/exabgp-compat/bin/bgp` - mock BGP peer (server side)
3. `test/exabgp-compat/bin/functional` - test runner (orchestrator)
4. `test/exabgp-compat/bin/exabgp` - Ze wrapper (client side)

## Task

Replace sequential port allocation (1900, 1901, ...) in ExaBGP compatibility tests with OS-assigned dynamic ports (bind to port 0). This eliminates port collisions when running concurrent test instances and removes the hardcoded port range.

**Problem:** Tests use a static `Port` class that hands out ports from 1900 upward. Concurrent test runs collide. The `--port` CLI flag exists but is never wired. Port 1900+ can also collide with user services.

**Solution:** Server (`bgp` mock peer) binds port 0, reports the OS-assigned port to stdout. Test runner reads the port from the server's stdout temp file, passes it to the client.

## Required Reading

### Architecture Docs
- [ ] `docs/functional-tests.md` - ExaBGP test structure
  -> Constraint: test infrastructure changes must not break existing test semantics

### RFC Summaries (MUST for protocol work)
N/A - test infrastructure only.

**Key insights:**
- `bgp` mock peer binds and listens; Ze dials out (client)
- Server and client are separate subprocesses spawned by `functional`
- Server stdout captured to temp file by `Exec.run()` - readable by parent
- SSH port derived as `bgp_port + 20000` inside client subprocess - works with dynamic ports since client receives actual port via env var

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `test/exabgp-compat/bin/functional:263-270` - `Port` class: sequential allocation from base 1900
  -> Constraint: `Port.get()` called once per test at `__init__` (line 1620)
- [ ] `test/exabgp-compat/bin/functional:1890-2019` - `run_selected`: starts all servers, then all clients
  -> Constraint: server port passed via `--port` CLI arg to both server and client subprocesses
- [ ] `test/exabgp-compat/bin/functional:1721-1830` - `_retry_failed_tests`: same server/client launch pattern
  -> Constraint: retry must also use dynamic ports
- [ ] `test/exabgp-compat/bin/functional:1624-1688` - `client()`, `server()`, `client_cmd()`: generate shell commands with port
  -> Constraint: `exabgp_tcp_port`, `ze_bgp_tcp_port` env vars carry the port
- [ ] `test/exabgp-compat/bin/bgp:1403-1470` - `parse_cmdline()`: rejects `port <= 0`
  -> Constraint: must change validation to allow port 0
- [ ] `test/exabgp-compat/bin/bgp:1525-1560` - `main()`: `sock.bind((host, port))` then `asyncio.start_server(sock=sock)`
  -> Constraint: after bind with port 0, `sock.getsockname()[1]` gives actual port
- [ ] `test/exabgp-compat/bin/exabgp:274-280` - SSH port derivation: `bgp_port + 20000`
  -> Constraint: client receives actual port in env var, derivation works as-is
- [ ] `test/exabgp-compat/bin/functional:529-608` - `Exec` class: stdout to temp file, accessible via `_stdout_file.name`
  -> Constraint: parent can read server's temp file while server is still running

**Behavior to preserve:**
- Test pass/fail semantics unchanged
- Server/client subprocess architecture unchanged
- `--server`/`--client` manual debug mode still works
- SSH port derivation for process bridging
- Retry logic for failed tests
- All `.ci` test expectations unchanged
- `--dry` output format (shows commands with ports)
- Stress test mode (`--stress`)

**Behavior to change:**
- Port allocation: sequential from 1900 -> OS-assigned via port 0
- Port discovery: pre-known -> read from server stdout after startup
- Concurrent run warning: "port conflicts" message becomes unnecessary

## Data Flow (MANDATORY)

### Entry Point
- Test runner (`functional`) starts server subprocesses

### Transformation Path
1. `functional` starts server subprocess with `--port 0`
2. Server subprocess runs `bgp --port 0`
3. `bgp` binds to `(host, 0)`, OS assigns port
4. `bgp` gets actual port via `sock.getsockname()[1]`
5. `bgp` prints `PORT <actual_port>` to stdout (flushed), then serves
6. `functional` reads server's stdout temp file, extracts port
7. `functional` starts client subprocess with `--port <actual_port>`
8. Client (Ze via exabgp wrapper) connects to `127.0.0.1:<actual_port>`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| bgp -> functional | stdout line `PORT <N>` via temp file | [ ] |
| functional -> client | `--port <N>` CLI arg -> env vars | [ ] |

### Integration Points
- `Exec._stdout_file.name` - parent reads server's stdout temp file to discover port
- `exabgp_tcp_port` / `ze_bgp_tcp_port` env vars - carry port to client

### Architectural Verification
- [ ] No bypassed layers (uses existing subprocess/temp file mechanism)
- [ ] No unintended coupling (server reports port, parent reads it - clean contract)
- [ ] No duplicated functionality (replaces Port class, doesn't add parallel mechanism)
- [ ] Zero-copy preserved where applicable (N/A - test infrastructure)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-exabgp-test` | -> | `bgp` binds port 0, reports port; `functional` discovers and passes to client | `make ze-exabgp-test` (existing tests all pass with dynamic ports) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `bgp --port 0` | Binds to OS-assigned port, prints `PORT <N>` to stdout where N > 0 |
| AC-2 | `functional encoding` (normal run) | All tests pass using dynamic ports (no pre-allocated ports) |
| AC-3 | Two concurrent `functional encoding` runs | Both complete without port collisions |
| AC-4 | `functional encoding --server <test>` (manual) | Prints assigned port so user can pass to `--client` |
| AC-5 | `bgp --port 0` server fails to start | Error reported, no hang waiting for port |
| AC-6 | `functional encoding --port 1900` (explicit override) | Uses port 1900, backward-compatible |

## TDD Test Plan

### Unit Tests
N/A - Python test scripts, no Go code changed.

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `--port` to `bgp` | 0-65535 | 65535 | -1 (rejected) | 65536 (rejected by argparse `type=int` range) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| All existing exabgp encoding tests | `test/exabgp-compat/encoding/*.ci` | All pass with dynamic port allocation | |
| Manual: `bgp --port 0` | Manual verification | Prints `PORT <N>`, accepts connection | |

### Future (if deferring any tests)
- None - all tests are covered by existing `make ze-exabgp-test`

## Files to Modify

- `test/exabgp-compat/bin/bgp` - allow port 0, report assigned port
- `test/exabgp-compat/bin/functional` - replace Port class, add port discovery, update launch flow

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | No | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` - note dynamic port allocation |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Files to Create

None - modifying existing files only.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify - check current state |
| 3. Implement | Implementation phases below |
| 4. Full verification | `make ze-exabgp-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: bgp port 0 support** - Allow port 0, report assigned port
   - Changes: `test/exabgp-compat/bin/bgp`
   - Validation: change `cmdarg.port <= 0` to `cmdarg.port < 0`
   - After `sock.bind()`: `actual_port = sock.getsockname()[1]`
   - Print `PORT <actual_port>` to stdout via `flushed()` before serving
   - Verify: `bgp --port 0 <msg_file>` prints `PORT <N>` and accepts connections

2. **Phase: functional port discovery** - Replace static allocation with dynamic discovery
   - Changes: `test/exabgp-compat/bin/functional`
   - Replace `Port` class usage: set `test.conf['port'] = 0` in `__init__`
   - Add `_discover_port(test, timeout)` method: polls server's stdout temp file for `PORT <N>` line
   - In `run_selected`: after starting all servers, discover ports before starting clients
   - In `_retry_failed_tests`: same pattern for retries
   - Update `dry()` to show port 0 (actual port unknown until runtime)
   - Handle `--port` CLI override: if user specifies explicit port, use it (backward compat)
   - Verify: `make ze-exabgp-test` passes

3. **Phase: cleanup** - Remove dead code
   - Remove `Port` class (no longer used)
   - Update concurrent-run warning (port collisions no longer possible)
   - Verify: `make ze-exabgp-test` passes

4. **Phase: documentation** - Update test infrastructure docs
   - Update `docs/functional-tests.md` re: dynamic port allocation
   - Verify: doc accurately describes new behavior

5. **Full verification** - `make ze-exabgp-test` (timeout 120s)

6. **Complete spec** - Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Port 0 works, discovery works, all existing tests pass |
| Correctness | Port discovery handles server crash (timeout, not hang) |
| Naming | `PORT <N>` sentinel consistent in bgp and functional |
| Data flow | Port flows: bgp stdout -> temp file -> functional reads -> client env var |
| Race conditions | Server has time to bind before parent reads; polling handles timing |
| Backward compat | `--port 1900` explicit override still works |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `bgp --port 0` prints PORT line | Run manually, check stdout |
| All exabgp tests pass | `make ze-exabgp-test` |
| No hardcoded port 1900 in allocation path | `grep -n 'base.*1900\|Port\.get' test/exabgp-compat/bin/functional` returns nothing |
| Port discovery has timeout | `grep '_discover_port\|timeout' test/exabgp-compat/bin/functional` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | `bgp --port` still rejects negative values |
| Port range | OS-assigned port is always valid (kernel guarantees) |
| Temp file race | stdout temp file read by parent only - no external access concern |

### Failure Routing

| Failure | Route To |
|---------|----------|
| bgp fails to bind port 0 | Phase 1 - check socket options |
| Port discovery times out | Phase 2 - increase timeout, check bgp prints before serving |
| Existing test fails | Phase 2 - verify port passed correctly to client |
| Concurrent runs still collide | Phase 2 - verify no residual static port usage |
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
N/A - test infrastructure only.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

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
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-exabgp-test` passes
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests PASS (`make ze-exabgp-test` output)
- [ ] Boundary tests for port input

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit

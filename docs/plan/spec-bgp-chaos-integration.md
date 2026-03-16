# Spec: bgp-chaos-integration (Phase 11 of 11)

| Field | Value |
|-------|-------|
| Status | blocked |
| Depends | - |
| Phase | - |
| Updated | 2026-03-03 |

**Master design:** `docs/plan/spec-bgp-chaos.md`
**Previous spec:** `spec-bgp-chaos-selftest.md` (Phase 10)
**Next spec:** None (final phase)

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/plan/spec-bgp-chaos.md` - master design (CLI flags, exit codes, config generation)
3. Phase 1-9 done specs - all chaos tool capabilities
4. `docs/architecture/testing/ci-format.md` - `.ci` test format
5. `test/plugin/fast.ci` - reference for Ze + peer orchestration pattern
6. `.claude/rules/planning.md` - workflow rules

## Task

Add end-to-end integration tests that prove `ze-chaos` actually tests Ze. Today the chaos tool has extensive unit tests for its components (scenario generation, validation model, event log, properties, shrinking) but nothing that starts a real Ze instance, connects chaos peers to it, and verifies route propagation through the route reflector.

**The gap:** The whole purpose of the chaos tool is to test Ze, but there is no automated test that runs the tool against Ze.

**Scope:**
- `--config-only` flag: write Ze config to stdout and exit (no peer simulation)
- `--managed` flag: start Ze as a subprocess, wait for ready, run chaos, stop Ze on completion
- `make functional-chaos` target in Makefile
- Smoke tests at three levels: propagation-only, basic chaos, multi-family
- Integration with CI (exit code 0 = pass, 1 = validation failure, 2 = runtime error)

**Deferred from Phase 9 (spec-bgp-chaos-inprocess):**
- `.ci` functional tests for in-process mode (`test/chaos/inprocess-basic.ci`, `inprocess-properties.ci`, `inprocess-chaos.ci`) — requires CLI `--in-process` entry point and `test/chaos/` directory, both part of this spec's integration infrastructure

## Required Reading

### Architecture Docs
- [ ] `docs/plan/spec-bgp-chaos.md` - CLI interface, exit codes, config generation
  → Decision: Exit codes: 0=pass, 1=validation failure, 2=runtime error
  → Decision: `--config-out <path>` writes Ze config matching the scenario
- [ ] `docs/architecture/testing/ci-format.md` - `.ci` test format
  → Constraint: `cmd=background` + `cmd=foreground` + `stdin=` blocks
  → Constraint: `$PORT` substituted by test runner
- [ ] `docs/functional-tests.md` - how functional tests are organized
  → Constraint: Tests discovered by `ze-test`, run via `make functional`

### Source Code
- [ ] `cmd/ze-chaos/main.go` - current CLI flags and startup flow
- [ ] `cmd/ze-chaos/scenario/config.go` - config generation
- [ ] `cmd/ze-chaos/orchestrator.go` - peer lifecycle coordination
- [ ] `cmd/ze-test/bgp.go` - test runner: how test suites are registered
- [ ] `internal/test/runner/` - `.ci` parsing, process orchestration, port allocation
- [ ] `test/plugin/fast.ci` - reference: Ze + peer orchestration pattern
- [ ] `Makefile` - existing functional test targets

**Key insights:**
- The `.ci` format excels at testing wire-level BGP behavior (hex patterns, message sequences) but is awkward for chaos integration (needs two-phase config gen + Ze startup)
- `--managed` mode makes the chaos tool self-contained: single command that generates config, starts Ze, runs chaos, reports results — ideal for CI
- `--config-only` enables the two-phase approach for users who want to run Ze separately (e.g., debugging, or testing against a remote Ze instance)
- The existing test runner allocates ports dynamically via `$PORT` — chaos tests must use the same mechanism to avoid conflicts with parallel test execution

## Current Behavior (MANDATORY)

**Source files read:** (to be re-read after Phase 9 completes)
- [ ] `cmd/ze-chaos/main.go` — CLI entry, flag parsing, `--config-out` logic
- [ ] `cmd/ze-chaos/orchestrator.go` — starts peer simulators, wires events
- [ ] `cmd/ze-chaos/scenario/config.go` — generates Ze config from scenario
- [ ] `Makefile` — current functional test targets

**Behavior to preserve:**
- All Phase 1-9 functionality
- Default mode: write config to stdout, run peers (existing behavior)
- `--config-out <path>` writes config to file (existing behavior)

**Behavior to change:**
- Add `--config-only`: write config and exit without starting peers
- Add `--managed`: start Ze subprocess, run chaos, stop Ze
- Add Makefile target and test files

## Data Flow (MANDATORY)

### Entry Point
- `--config-only`: CLI flags + seed → scenario generator → config to stdout → exit
- `--managed`: CLI flags + seed → scenario generator → config to temp file → Ze subprocess → peer simulators → validation → summary → exit
- `.ci` test: test runner invokes `ze-chaos --managed --port $PORT` as foreground process

### Transformation Path

**`--config-only` mode:**
1. Scenario generator creates peer profiles from seed (same as normal mode)
2. Config generator writes Ze config to stdout (or `--config-out` path)
3. Exit 0 — no peer simulators started, no TCP connections

**`--managed` mode:**
1. Generate config → write to `os.CreateTemp("", "ze-chaos-*.conf")`
2. Start Ze subprocess: `ze bgp server <temp-config>`
3. Wait for Ze to be listening (poll TCP port every 100ms, timeout 5s)
4. Run peer simulators (normal chaos flow — same as external mode)
5. On completion (duration elapsed, Ctrl-C, or violation found):
   - Stop peer simulators
   - Send SIGTERM to Ze subprocess
   - Wait for Ze to exit (timeout 5s, then SIGKILL)
   - Remove temp config file
   - Print summary
   - Exit with appropriate code (0/1/2)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Chaos tool → Ze subprocess | `os/exec.Command("ze", "bgp", "server", configPath)` | [ ] |
| Chaos peers ↔ Ze | TCP (same as normal external mode) | [ ] |
| Test runner → chaos tool | `cmd=foreground:exec=ze-chaos --managed ...` | [ ] |

### Integration Points
- Ze binary must be in `$PATH` or buildable via `make` before chaos tests run
- Port allocation: `--port` flag accepts `$PORT` from test runner
- Temp config file: use `os.CreateTemp` in managed mode, clean up on exit

### Architectural Verification
- [ ] `--config-only` produces identical config to normal mode (same seed → same output)
- [ ] `--managed` mode produces identical results to manual two-terminal mode
- [ ] Ze subprocess stderr captured and logged (for debugging failures)
- [ ] Ze subprocess cleaned up even on panic or SIGKILL of chaos tool

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `--config-only --seed 42 --peers 3` | Writes valid Ze config to stdout and exits 0 |
| AC-2 | `--config-only --config-out f.conf` | Writes config to file and exits 0 |
| AC-3 | Config from `--config-only` | Passes `ze bgp validate` |
| AC-4 | `--managed --seed 42 --peers 3 --duration 10s --chaos-rate 0` | Starts Ze, runs peers, routes propagate, exits 0 |
| AC-5 | `--managed` with chaos | Chaos events fire, routes validated, exits 0 |
| AC-6 | `--managed` with Ze crash | Detects Ze exit, reports error, exits 2 |
| AC-7 | `--managed` with Ctrl-C | Clean shutdown: stops peers, stops Ze, prints summary |
| AC-8 | `make functional-chaos` | Runs smoke tests, all pass |
| AC-9 | `make functional-chaos` in parallel with `make functional` | No port conflicts (uses allocated ports) |
| AC-10 | `--managed --port 0` | Auto-allocate available port |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConfigOnly` | `main_test.go` | `--config-only` writes config to stdout and exits | |
| `TestConfigOnlyFile` | `main_test.go` | `--config-only --config-out` writes to file | |
| `TestConfigOnlyValidates` | `main_test.go` | Output passes `ze bgp validate` | |
| `TestConfigOnlyDeterministic` | `main_test.go` | Same seed → byte-identical config | |
| `TestManagedStartsZe` | `managed_test.go` | Ze subprocess starts and listens | |
| `TestManagedStopsZe` | `managed_test.go` | Ze subprocess stopped on completion | |
| `TestManagedZeCrash` | `managed_test.go` | Ze exit detected, error reported | |
| `TestManagedSignal` | `managed_test.go` | SIGTERM → clean shutdown of both | |
| `TestManagedPortAllocation` | `managed_test.go` | `--port 0` finds available port | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Ze startup timeout | 1s-30s | 30s | 0 | N/A (clamped) |
| Ze shutdown timeout | 1s-10s | 10s | 0 | N/A (clamped) |

### Functional Tests

These are the actual integration tests that prove Ze works correctly:

| Test | Location | Scenario | Duration | Status |
|------|----------|----------|----------|--------|
| `chaos-smoke-propagation` | `test/chaos/smoke-propagation.ci` | 3 peers, chaos-rate 0, verify all routes propagate | 10s | |
| `chaos-smoke-disconnect` | `test/chaos/smoke-disconnect.ci` | 3 peers, 1 forced disconnect, verify withdrawal + replay | 15s | |
| `chaos-smoke-chaos` | `test/chaos/smoke-chaos.ci` | 4 peers, chaos-rate 0.3, verify no validation failures | 20s | |

### Functional Test Format

The `.ci` files use `--managed` mode for self-contained execution:

```
# test/chaos/smoke-propagation.ci
#
# Smoke test: 3 peers, no chaos, verify route propagation through RR
# Uses --managed to start Ze automatically

cmd=foreground:seq=1:exec=ze-chaos --managed --seed 42 --peers 3 --duration 10s --chaos-rate 0 --quiet --port $PORT:timeout=20s
expect=exit:code=0
```

```
# test/chaos/smoke-disconnect.ci
#
# Smoke test: 3 peers, one disconnect event, verify withdrawal propagation
# Uses fixed seed that produces a disconnect at ~5s

cmd=foreground:seq=1:exec=ze-chaos --managed --seed 100 --peers 3 --duration 15s --chaos-rate 0.5 --quiet --port $PORT:timeout=25s
expect=exit:code=0
```

```
# test/chaos/smoke-chaos.ci
#
# Smoke test: 4 peers with moderate chaos, verify no validation failures
# Longer run to exercise reconnect and withdrawal paths

cmd=foreground:seq=1:exec=ze-chaos --managed --seed 7777 --peers 4 --duration 20s --chaos-rate 0.3 --quiet --port $PORT:timeout=35s
expect=exit:code=0
```

**Why `--managed`:** The `.ci` test runner provides `$PORT` and manages timeouts. With `--managed`, the chaos tool handles Ze lifecycle internally. The test runner only needs to check the exit code — all validation is done by the chaos tool itself.

**Why fixed seeds:** Each test uses a specific seed that produces a known scenario. If Ze has a regression, the test fails deterministically with that seed.

## Files to Create

- `cmd/ze-chaos/managed.go` — managed mode: Ze subprocess lifecycle
- `cmd/ze-chaos/managed_test.go`
- `test/chaos/smoke-propagation.ci` — propagation-only smoke test
- `test/chaos/smoke-disconnect.ci` — disconnect + withdrawal smoke test
- `test/chaos/smoke-chaos.ci` — moderate chaos smoke test

## Files to Modify

- `cmd/ze-chaos/main.go` — add `--config-only` and `--managed` flags
- `cmd/ze-chaos/orchestrator.go` — skip peer startup in `--config-only` mode
- `cmd/ze-test/bgp.go` — register `chaos` test suite (discovers `test/chaos/*.ci`)
- `Makefile` — add `functional-chaos` target

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | N/A |
| Makefile | Yes | `functional-chaos` target |
| Test runner registration | Yes | `cmd/ze-test/bgp.go` — add `chaos` suite |
| Ze binary dependency | Yes | `make functional-chaos` must build Ze first |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|

## Implementation Steps

1. **Read Phase 1-9 learnings** — understand config generation and orchestrator flow
   → Review: How does config-out work today? What's the startup sequence?

2. **Implement `--config-only`** — write config, exit
   → Simple: skip orchestrator startup, just generate and print

3. **Write `--config-only` tests**
   → Run: Tests PASS (implementation first since it's trivial)

4. **Write managed mode tests** — Ze subprocess lifecycle
   → Run: Tests FAIL

5. **Implement managed mode** — start Ze, wait for ready, run chaos, stop Ze
   → Run: Tests PASS

6. **Create `.ci` smoke test files**

7. **Register chaos test suite** in `cmd/ze-test/bgp.go`

8. **Add Makefile target** — `functional-chaos`

9. **Run smoke tests** — `make functional-chaos`
   → Verify: All three pass against real Ze

10. **Verify** — `make ze-lint && make test && make functional-chaos`

## Managed Mode Design

### Ze Subprocess Management

**Startup:**
1. Write config to `os.CreateTemp("", "ze-chaos-*.conf")`
2. Start `ze bgp server <configPath>` with `exec.CommandContext`
3. Capture Ze's stderr to a buffer (for error reporting)
4. Poll Ze's listening port (TCP connect attempt every 100ms, timeout 5s)
5. Once listening, proceed with chaos scenario

**Ready detection:**
- Try `net.DialTimeout("tcp", addr, 100ms)` in a loop
- Ze listens on `127.0.0.1:<port>` (configured in generated config)
- After successful dial, close the probe connection immediately
- Retry up to 50 times (5s total) before giving up

**Shutdown:**
1. Stop peer simulators (existing graceful shutdown)
2. Send SIGTERM to Ze subprocess via `cmd.Process.Signal(syscall.SIGTERM)`
3. Wait up to 5s for Ze to exit (`cmd.Wait()` with timeout)
4. If still running, send SIGKILL
5. Remove temp config file
6. Report Ze's exit code in summary (non-zero = warning)

**Error handling:**
- Ze crashes during run → detect via `cmd.Wait()` returning early, report error, exit 2
- Ze fails to start → timeout on port polling, report Ze's stderr, exit 2
- Port already in use → Ze fails to bind, captured in stderr, exit 2
- Chaos tool panics → deferred cleanup sends SIGKILL to Ze

### Port Allocation

**`--port 0` (auto-allocate):**
1. Bind a TCP listener on `127.0.0.1:0`
2. Read the allocated port from `listener.Addr()`
3. Close the listener
4. Use that port for Ze config and peer connections
5. Small race window (port freed then Ze binds it) — acceptable for testing

**`--port N` (explicit):**
- Use as-is (existing behavior)
- In `.ci` files, `$PORT` is substituted by test runner

## Spec Propagation Task

**MANDATORY at end of this phase (final phase):**

1. **Update `docs/plan/spec-bgp-chaos.md`** (master design) with:
   - Phase 10 added to phase table
   - Integration test pattern documented
   - `--config-only` and `--managed` in CLI interface section

2. **Update Makefile documentation** if any exists

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

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `--config-only` flag | ❌ Not implemented | — | |
| `--managed` flag | ❌ Not implemented | — | |
| Ze subprocess lifecycle | ❌ Not implemented | — | |
| Ready detection (port polling) | ❌ Not implemented | — | |
| Clean shutdown (SIGTERM → SIGKILL) | ❌ Not implemented | — | |
| Ze crash detection | ❌ Not implemented | — | |
| `make functional-chaos` target | ❌ Not implemented | — | |
| Smoke test: propagation | ❌ Not implemented | — | |
| Smoke test: disconnect | ❌ Not implemented | — | |
| Smoke test: chaos | ❌ Not implemented | — | |
| Test runner registration | ❌ Not implemented | — | |
| Port auto-allocation | ❌ Not implemented | — | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ❌ Not implemented | — | --config-only not built |
| AC-2 | ❌ Not implemented | — | --config-only + --config-out not built |
| AC-3 | ❌ Not implemented | — | Config validation not tested |
| AC-4 | ❌ Not implemented | — | --managed mode not built |
| AC-5 | ❌ Not implemented | — | --managed with chaos not built |
| AC-6 | ❌ Not implemented | — | Ze crash detection not built |
| AC-7 | ❌ Not implemented | — | Ctrl-C clean shutdown not built |
| AC-8 | ❌ Not implemented | — | make functional-chaos not built |
| AC-9 | ❌ Not implemented | — | Port conflict avoidance not built |
| AC-10 | ❌ Not implemented | — | --port 0 auto-allocate not built |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| All unit tests | ❌ Not implemented | — | Entire spec not started |
| All functional tests | ❌ Not implemented | — | No .ci files created |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `managed.go` | ❌ Not created | |
| `managed_test.go` | ❌ Not created | |
| `test/chaos/smoke-propagation.ci` | ❌ Not created | |
| `test/chaos/smoke-disconnect.ci` | ❌ Not created | |
| `test/chaos/smoke-chaos.ci` | ❌ Not created | |
| `main.go` | ❌ Not modified | --config-only/--managed not added |
| `orchestrator.go` | ❌ Not modified | |
| `cmd/ze-test/bgp.go` | ❌ Not modified | chaos suite not registered |
| `Makefile` | ❌ Not modified | functional-chaos target not added |

### Audit Summary
- **Total items:** 33
- **Done:** 0
- **Partial:** 0
- **Not implemented:** 33 (entire spec not started)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 demonstrated
- [ ] Tests pass (`make ze-unit-test`)
- [ ] No regressions (`make ze-functional-test`)
- [ ] `make functional-chaos` passes

### Quality Gates (SHOULD pass)
- [ ] `make ze-lint` passes
- [ ] Master design doc updated (Spec Propagation Task)
- [ ] Implementation Audit completed

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
- [ ] Boundary tests for numeric inputs
- [ ] Functional tests verify Ze route propagation end-to-end

### Completion
- [ ] Spec Propagation Task completed
- [ ] Spec updated with Implementation Summary
- [ ] Write learned summary to `docs/learned/NNN-bgp-chaos-integration.md`

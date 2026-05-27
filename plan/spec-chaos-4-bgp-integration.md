# Spec: bgp-chaos-integration (Phase 11 of 11)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/5 |
| Updated | 2026-05-27 |

**Master design:** `plan/spec-bgp-chaos.md`
**Previous spec:** `spec-bgp-chaos-selftest.md` (Phase 10)
**Next spec:** None (final phase)

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-bgp-chaos.md` - master design (CLI flags, exit codes, config generation)
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
- [ ] `plan/spec-bgp-chaos.md` - CLI interface, exit codes, config generation
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
| `--config-only` flag | → | `cmd/ze-chaos/main.go:509` | `TestConfigOnly`, `TestConfigOnlyFile` |
| Fork mode (default) | → | `cmd/ze-chaos/fork.go:22` | `.ci` smoke tests via `ze-test bgp chaos` |
| `--port 0` auto-alloc | → | `cmd/ze-chaos/subcommand.go:157` | `TestAllocatePort`, `TestAllocatePortUnique` |
| `ze-test bgp chaos` | → | `cmd/ze-test/bgp.go:275` | `make ze-chaos-integration-test` |
| `make ze-chaos-integration-test` | → | `mk/test-chaos.mk:35` | CI target |

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `ai/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/rfcNNNN.md` |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` or subsystem doc |

### Critical Review Checklist

| # | What to verify | How to verify |
|---|----------------|---------------|
| 1 | Ze subprocess cleaned up on all exit paths (normal, error, signal, panic) | Code review: `defer child.Shutdown()` in fork.go |
| 2 | Fork mode pipes config via stdin (no temp file to leak) | Code review: fork.go stdin.Write, no os.CreateTemp |
| 3 | Config generation deterministic (same seed = identical output) | TestConfigOnlyDeterministic |
| 4 | Stdin close → 5s wait → SIGKILL escalation | Code review: fork.go:81-97 + smoke-propagation.ci |
| 5 | Ze crash detected and reported as exit code 2 | Code review: main.go:697-709 zeCrashed atomic |
| 6 | Port auto-allocation works (--port 0) | TestAllocatePort, TestAllocatePortUnique |
| 7 | --config-only does NOT start peers or Ze | TestConfigOnlyNoNetwork |
| 8 | Ze stderr forwarded for diagnostics | Code review: fork.go:33 `cmd.Stderr = os.Stderr` |
| 9 | .ci tests use $PORT for parallel safety | grep $PORT in test/chaos/*.ci |

### Deliverables Checklist

| # | Deliverable | Verification method |
|---|-------------|---------------------|
| 1 | `--config-only` flag | `ze-chaos --config-only --seed 42 --peers 3` exits 0 with valid config |
| 2 | Fork mode (default, = spec's `--managed`) | `ze-chaos --seed 42 --peers 3 --duration 5s` exits 0 |
| 3 | `fork.go` (= spec's `managed.go`) | `ls cmd/ze-chaos/fork.go` |
| 4 | `main_test.go` (= spec's `managed_test.go`) | `go test ./cmd/ze-chaos/ -run TestConfigOnly` |
| 5 | `test/chaos/smoke-propagation.ci` | `ls test/chaos/smoke-propagation.ci` |
| 6 | `test/chaos/smoke-disconnect.ci` | `ls test/chaos/smoke-disconnect.ci` |
| 7 | `test/chaos/smoke-chaos.ci` | `ls test/chaos/smoke-chaos.ci` |
| 8 | `make ze-chaos-integration-test` target | `make -n ze-chaos-integration-test` succeeds |
| 9 | Chaos test suite registered | `grep chaos cmd/ze-test/bgp.go` |

### Security Review Checklist

| # | Concern | What to check |
|---|---------|---------------|
| 1 | Command injection via ze binary path | Binary from --ze flag or exec.LookPath, not user web input |
| 2 | No temp files to leak | Fork mode pipes config via stdin, no os.CreateTemp |
| 3 | Subprocess environment | Ze subprocess inherits parent env (appropriate for test tool) |
| 4 | Port binding scope | localAddr defaults to 127.0.0.1, not 0.0.0.0 |
| 5 | Resource exhaustion | Ze startup timeout via waitForZe; shutdown timeout 5s in fork.go |

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

1. **Update `plan/spec-bgp-chaos.md`** (master design) with:
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
| `--config-only` flag | ✅ Done | `main.go:158,509-523` | Pre-existing |
| `--managed` flag | ✅ Done (as fork mode) | `fork.go:22` | Default mode, no flag needed |
| Ze subprocess lifecycle | ✅ Done | `fork.go:22-97` | forkZe + zeChild.Shutdown |
| Ready detection (port polling) | ✅ Done | `subcommand.go:180` | waitForZe with backoff |
| Clean shutdown (SIGTERM → SIGKILL) | ✅ Done | `fork.go:81-97` | 5s timeout then SIGKILL |
| Ze crash detection | ✅ Done | `main.go:697-709` | zeCrashed atomic, exit 2 |
| `make ze-chaos-integration-test` | ✅ Done | `mk/test-chaos.mk:35` | Was `functional-chaos` in spec |
| Smoke test: propagation | ✅ Done | `test/chaos/smoke-propagation.ci` | Pre-existing |
| Smoke test: disconnect | ✅ Done | `test/chaos/smoke-disconnect.ci` | Pre-existing |
| Smoke test: chaos | ✅ Done | `test/chaos/smoke-chaos.ci` | Pre-existing |
| Test runner registration | ✅ Done | `cmd/ze-test/bgp.go:275` | chaos suite |
| Port auto-allocation | ✅ Done | `subcommand.go:157,main.go:447` | Pre-existing |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `TestConfigOnly` | main_test.go |
| AC-2 | ✅ Done | `TestConfigOnlyFile` | main_test.go |
| AC-3 | ✅ Done | `TestConfigOnlyDeterministic` | main_test.go |
| AC-4 | ✅ Done | `smoke-propagation.ci` | Fork mode = managed mode |
| AC-5 | ✅ Done | `smoke-chaos.ci` | chaos-rate 0.3 |
| AC-6 | ✅ Done | `main.go:697-709` | zeCrashed → exit 2 |
| AC-7 | ✅ Done | `main.go:682-694,fork.go:81` | SIGTERM handler + Shutdown |
| AC-8 | ✅ Done | `mk/test-chaos.mk:35` | `make ze-chaos-integration-test` |
| AC-9 | ✅ Done | `$PORT` in .ci files | Test runner allocates ports |
| AC-10 | ✅ Done | `TestAllocatePort` | main_test.go |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestConfigOnly` | ✅ | `main_test.go` | Stdout output |
| `TestConfigOnlyFile` | ✅ | `main_test.go` | File output |
| `TestConfigOnlyDeterministic` | ✅ | `main_test.go` | Same seed = identical |
| `TestConfigOnlyNoNetwork` | ✅ | `main_test.go` | No TCP side effects |
| `TestConfigOnlyPipeExclusive` | ✅ | `main_test.go` | Flag exclusivity |
| `TestConfigOnlyInProcessExclusive` | ✅ | `main_test.go` | Flag exclusivity |
| `TestAllocatePort` | ✅ | `main_test.go` | Valid port range |
| `TestAllocatePortUnique` | ✅ | `main_test.go` | No duplicates |
| `TestCheckPortFree` | ✅ | `main_test.go` | Occupied port |
| `TestCheckPortFreeAvailable` | ✅ | `main_test.go` | Free port |
| `TestWaitForZeTimeout` | ✅ | `main_test.go` | Timeout on no listener |
| `TestWaitForZeSuccess` | ✅ | `main_test.go` | Success with listener |
| `TestRunInvalidPeers` | ✅ | `main_test.go` | Boundary: 0 and 51 |
| `TestRunInvalidChaosRate` | ✅ | `main_test.go` | Boundary: -0.1 and 1.1 |
| `TestRunInvalidPort` | ✅ | `main_test.go` | Boundary: privileged port |
| Functional: propagation | ✅ | `test/chaos/smoke-propagation.ci` | Pre-existing |
| Functional: disconnect | ✅ | `test/chaos/smoke-disconnect.ci` | Pre-existing |
| Functional: chaos | ✅ | `test/chaos/smoke-chaos.ci` | Pre-existing |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `managed.go` | ✅ (as `fork.go`) | Fork mode is the managed mode |
| `managed_test.go` | ✅ (as `main_test.go`) | Tests cover run(), allocatePort, waitForZe, checkPortFree |
| `test/chaos/smoke-propagation.ci` | ✅ Pre-existing | |
| `test/chaos/smoke-disconnect.ci` | ✅ Pre-existing | |
| `test/chaos/smoke-chaos.ci` | ✅ Pre-existing | |
| `main.go` | ✅ Pre-existing | --config-only already implemented |
| `orchestrator.go` | ✅ Pre-existing | |
| `cmd/ze-test/bgp.go` | ✅ Pre-existing | chaos suite registered |
| `mk/test-chaos.mk` | ✅ Pre-existing | Replaces planned Makefile changes |

### Audit Summary
- **Total items:** 33
- **Done:** 33
- **Partial:** 0
- **Not implemented:** 0

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 demonstrated
- [ ] Tests pass (`make ze-unit-test`)
- [ ] Tests pass (`make ze-test`)
- [ ] No regressions (`make ze-functional-test`)
- [ ] `make ze-chaos-integration-test` passes

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
- [ ] Write learned summary to `plan/learned/NNN-bgp-chaos-integration.md`

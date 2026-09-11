# Spec: failing-socket-proof-needs-a-non-ptrace-injection-point

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | 0/0 |
| Handoff | - |
| Updated | 2026-09-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Ze has no way to hold a real socket in a persistently failing state from
outside the daemon, so no test can assert what an operator's monitoring shows
while one fails: the daemon's own CPU use, read off `/metrics`, over a window.**

`spec-subscriber-reader-loops-retry-a-failing-socket-without-backoff` needed that
assertion for its AC-1 and could not build it. The finding was measured on
2026-09-11 and is kept, with its numbers, at
`test/draft/l2tp/subscriber-reader-failing-socket.ci`:

- No ordinary network condition sustains a repeating, non-blocking read error on
  a UDP or `AF_PACKET` socket. Interface-down delivers silence, and a
  device-bound `AF_PACKET` socket gets `ENETDOWN` delivered once from
  `packet_notifier` before it reverts to blocking.
- ptrace-based syscall fault injection does sustain one, and it destroys the
  measurement. Go's runtime emits a continuous SIGURG stream for goroutine
  preemption, ptrace traps every delivery regardless of `--seccomp-bpf`, and
  ptrace-stopped time counts as wall clock rather than as the tracee's CPU. The
  pre-fix build reached 36,640 read errors and 0.99 CPU-seconds driven by hand,
  and 19 errors with no measurable CPU delta through the `.ci` harness, where it
  passed every assertion.
- `LD_PRELOAD` does not apply: Ze builds `CGO_ENABLED=0` and makes raw syscalls,
  so there is no libc call to interpose on.

The goal: a `.ci` test can fail one named socket for a bounded window without
tracing the daemon, so a process-level CPU or rate assertion discriminates a
paced loop from a spinning one.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/qemu-integration.md` - how a Linux-only test reaches ze's runtime kernel, and what a guest run may install
- [ ] `test/draft/README.md` - why the scenario is tracked where it is, and what moving it to `test/l2tp/` requires
- [ ] `docs/architecture/testing/interop.md` - the four vacuity traps; this spec exists because one of them was met

### RFC Summaries (Scope: protocol)
- N-A. No protocol behavior changes; this is test infrastructure.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `test/draft/l2tp/subscriber-reader-failing-socket.ci` - the scenario, its strace wrapper, and the header carrying the whole finding
- [ ] `internal/test/fixture/tunnel_fixture_l2tp_failing_socket.go` - `tunnelL2TPFailingSocket`, which reads `process_cpu_seconds_total` and `ze_l2tp_listener_read_errors_total` across a 3s window. Sound as written; only its stimulus is broken

**Behavior to preserve:**
- The fixture's measurement. The CPU and counter reads are not what failed.

**Behavior to change:**
- The stimulus. A mechanism that fails a socket read without tracing the process.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A `.ci` file asks for a named socket on the daemon under test to fail its reads.

### Transformation Path
1. The test declares which socket fails and for how long.
2. The mechanism injects the failure below the daemon, in the kernel.
3. The daemon's reader loop meets a real error it cannot classify.
4. The fixture reads the daemon's own `/metrics` across the window.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Test to kernel | Whatever the mechanism uses to arm the failure | No |
| Kernel to daemon | The read error the loop already handles | No |
| Daemon to test | `/metrics`, unchanged | Yes, the fixture already does this |

### Integration Points
- The guest kernel build, if the mechanism needs a config symbol.
- `test/l2tp/`, where the rewritten scenario lands.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No wire path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A kernel fault-injection framework can fail one process's socket reads without tracing it | docs.kernel.org/fault-injection describes `fail_function` and per-task filtering | The mechanism is something else, and this spec's first phase is to find it | Arm it in the QEMU guest and watch a read fail | unvalidated |
| A-2 | Ze's runtime kernel can carry the config symbols it needs | `tmp/kernel/build/config` is Ze's own kernel build | A second kernel flavor is needed for the test, or the mechanism is rejected | Read the config and the kernel build inputs | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The mechanism fails reads for every process, so the harness fails too | The test runner dies before the daemon does | Filter by task, which is what the per-task knobs exist for |
| R-2 | The measurement is still swamped, by the injection rather than by ptrace | The pre-fix and fixed builds agree again | Measure the instrument first, against a deliberately spinning stub, before trusting a verdict |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A test claims a loop is paced when it is not; nothing ships wrong |
| How is it reverted? | Delete the scenario and the mechanism. No product code changes |
| Who else touches this path? | Any future spec that needs a process-level resource assertion under a real failure |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A `.ci` asking for a failing socket | → | the injection mechanism | `subscriber-reader-failing-socket`, rewritten and gated in `test/l2tp/` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A `.ci` arms the mechanism against one socket | Reads on that socket fail for the window, with no ptrace in the path |
| AC-2 | The pre-fix (pacer-reverted) build runs the scenario | It goes RED, and the red is recorded |
| AC-3 | The fixed build runs the scenario | It goes GREEN |
| AC-4 | The daemon is launched the way a `.ci` launches it | The mechanism still reaches it, with no hand-driven wrapper |
| AC-5 | Ze's runtime kernel boots in the guest | It carries whatever the mechanism needs |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes a test that proves a daemon stays responsive while a socket fails | `.ci` → injection → real read error → `/metrics` assertion | `subscriber-reader-failing-socket` in `test/l2tp/` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| the mechanism's own arm and disarm | with the mechanism's code | AC-1 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| failure window | bounded, shorter than the test timeout | to be fixed at design time | N/A | a window that outlives the test leaves the guest broken |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `subscriber-reader-failing-socket` | `test/l2tp/` after the move | A subscriber socket fails persistently and the daemon's CPU stays low | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | No protocol behavior changes | N-A |

## Files to Modify
- `test/draft/l2tp/subscriber-reader-failing-socket.ci` - rewritten onto the mechanism and moved to `test/l2tp/`
- `.gitignore` - the `test/draft/l2tp/` exception is deleted with the move
- `test/draft/README.md` - its tracked-exception table loses that row

## Files to Create
- The mechanism itself, wherever it belongs once it is chosen

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Test infrastructure |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No verb |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No leaf |
| Functional test for new RPC/API | Yes | The rewritten scenario is the test |
| Pipe completeness | N-A | No command output |
| Env var registration | N-A | Decided at design time |
| Doctor check for runtime dependencies | N-A | Nothing the shipped daemon depends on |
| Prometheus counters/metrics | N-A | The counters already exist |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Test infrastructure |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/qemu-integration.md` and `test/draft/README.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | No | DERIVED at design time |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: find the mechanism** -- arm a kernel fault injection against one
   socket read in the QEMU guest, by hand, and watch a read fail
   - Tests: none; the artifact is a recorded manual run
   - Files: read only
   - Verify: a real `read` error reaches a process that is not being traced
2. **Phase: measure the instrument** -- run a deliberately spinning stub and a
   paced one under the mechanism and confirm they differ by orders of magnitude
   - Tests: the comparison itself
   - Files: a throwaway stub
   - Verify: the two are distinguishable, which is exactly what ptrace failed
3. **Phase: wire it into a `.ci`** -- rewrite the scenario and gate it
   - Tests: `subscriber-reader-failing-socket`
   - Files: the scenario, `.gitignore`, `test/draft/README.md`
   - Verify: the pacer-reverted build goes RED and the fixed build GREEN

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | The scenario is gated, not merely runnable by hand |
| Feature completeness | The mechanism is reusable by the next spec that needs it |
| Correctness | The red was observed, not predicted |
| Naming | The mechanism is named for what it does, not for the one test that used it first |
| Data flow | The daemon is not traced, at all, anywhere in the path |
| Rule: `ai/rules/simplicity.md` | No framework. One way to fail one socket |
| Rule: `ai/rules/principles.md` | A window that fails to arm is an error, never a quiet pass |
| Rule: `ai/rules/goroutine-lifecycle.md` | N-A unless the mechanism owns a goroutine |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The mechanism works with no ptrace | The recorded run from phase 1 |
| The instrument does not swamp the signal | The phase 2 comparison |
| The scenario discriminates | The recorded red from the reverted pacer |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Blast radius of the injection | It fails one task's reads, never the whole guest |
| Cleanup | The failure is disarmed when the test ends, including on a failed test |
| Privilege | Whatever it needs stays inside the guest and never reaches a shipped image |

### Failure Routing
| Failure | Route To |
|---------|----------|
| The mechanism cannot be armed | RESEARCH: the candidate was wrong, find the next one |
| The two builds still agree | Phase 2 found a second instrument problem; diagnose before writing the test |
| The scenario passes on the reverted build | The test is vacuous again; do not gate it |

## Design Insights

- The measurement was never the problem. The stimulus was, and the instrument
  that provided it was also the thing that destroyed the reading.
- A test that cannot be made to fail is worth keeping only where its finding is,
  which is why the vacuous scenario is tracked rather than deleted.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Kernel fault injection | ptrace, `LD_PRELOAD` | ptrace was measured and destroys the signal; `LD_PRELOAD` reaches nothing under `CGO_ENABLED=0` |

## Known Limitations
- Until this is built, no spec can assert daemon CPU under a real socket failure,
  and each one has to say so in its own Known Limitations.

## RFC Documentation (Scope: protocol)

N-A. No protocol behavior changes.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

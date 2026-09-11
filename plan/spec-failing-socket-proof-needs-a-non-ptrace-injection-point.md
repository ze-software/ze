# Spec: failing-socket-proof-needs-a-non-ptrace-injection-point

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 3/3 |
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
  - -> Constraint: the guest runs ze's own kernel only when `kernel tmp/kernel/build/vmlinuz` is passed, and `Run.assertRuntimeKernel` then refuses a boot whose `uname -r` disagrees with `internal/appliance/kernel.version`. The phase-1 runs took that path and the guest reported `Linux localhost 7.2.0 ... aarch64`
  - -> Decision: the mechanism needs no new boot path, no `./le qemu` action of its own, no `packages` beyond the base image, and no module load. `./le qemu run ... command "<one shell command>"` carried the whole hand run
- [ ] `test/draft/README.md` - why the scenario is tracked where it is, and what moving it to `test/l2tp/` requires
  - -> Constraint: the move to `test/l2tp/` is earned by the pre-fix build FAILING the scenario, and it carries three edits together: the file move, the `.gitignore` negation, and the tracked-exception row
- [ ] `docs/architecture/testing/interop.md` - the four vacuity traps; this spec exists because one of them was met
- [ ] `docs/architecture/testing/ci-format.md` - the `Design:` document of `internal/test/cli/register.go`, which this spec adds a root to
  - -> Constraint: a root is one `registerRoot` line, and the `.ci` reaches it through the ordinary `cmd=background:exec=` path. Neither the format nor the dispatch changes
- [ ] `docs/research/l2tpv2-ze-integration.md` - the `Design:` document of `internal/test/fixture/tunnel_fixture_l2tp_failing_socket.go`, which this spec re-heads
  - -> Constraint: the fixture's receiver-goroutine measurement is unchanged. Only the header's account of the stimulus is rewritten

### RFC Summaries (Scope: protocol)
- N-A. No protocol behavior changes; this is test infrastructure.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `test/draft/l2tp/subscriber-reader-failing-socket.ci` - the scenario, its strace wrapper, and the header carrying the whole finding
  - -> Decision: the `tmpfs=run-strace.sh` block and the strace line are deleted, not adapted. The `cmd=background:exec=` line names the seccomp launcher, which execs `ze -` with the same stdin config
- [ ] `internal/test/fixture/tunnel_fixture_l2tp_failing_socket.go` - `tunnelL2TPFailingSocket`, which reads `process_cpu_seconds_total` and `ze_l2tp_listener_read_errors_total` across a 3s window. Sound as written; only its stimulus is broken
  - -> Constraint: the measurement is kept as written, and its two bounds already sit between the observed builds. Measured in the guest on 2026-09-11: ze as it stands (paced) counted 14 read errors and 0.14 CPU-seconds across a 3s window, about 0.05 of a core; an unpaced reader loop under the same filter reached 4.4e5 to 2.6e6 errors a second at 0.86 to 0.97 of a core. The fixture bounds are 1000 errors and 0.5 of a core
- [ ] `internal/component/l2tp/listener.go`, `internal/component/l2tp/ppp/ra_linux.go`, `internal/component/l2tp/ppp/dhcpv6_linux.go`, `internal/component/l2tp/pppoe/subsystem.go` - the four loops, and the syscall each blocks in
  - -> Constraint: `readLoop` and `dhcpv6ServerLoop` block in `recvfrom` on a UDP socket (`internal/poll.FD.ReadFromInet4/6` calls `unix.RecvfromInet4/6`), `discoveryReader` blocks in `recvfrom` on a blocking `AF_PACKET/SOCK_RAW` socket (`readDiscoveryFrame`, `unix.Recvfrom`), and `rsReaderLoop` blocks in `recvmsg` (`ipv6.PacketConn.ReadFrom` reaches `socket.Conn.RecvMsg`). A filter naming only `recvfrom` leaves the RA reader untouched, which was observed
  - -> Constraint: the injected errno MUST NOT be `EBADF` or `EINVAL`. `readDiscoveryFrame` maps both onto `errSocketClosed`, which ends the loop instead of counting an error. `ENETDOWN` is classified as unrecoverable-but-unclassified by all four loops

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
| No bypassed layers (data flows through the intended path) | Yes | The daemon is launched by the `.ci` runner's ordinary `cmd=background:exec=` path. The launcher `execve`s ze, so the process the fixture measures IS the daemon, with no supervisor above it and `TracerPid: 0` |
| No unintended coupling (components stay isolated) | Yes | `internal/test/failsyscall` imports `golang.org/x/sys/unix` and `internal/core/crashlog`. No product package imports it, and nothing in `internal/component/l2tp` knows it exists |
| No duplicated functionality (extends existing, does not recreate) | Yes | Errno names are resolved by walking `unix.ErrnoName` rather than by a second table. The 10 MB output cap that bounds the red run is the runner's existing `maxOutputBytes` (`internal/test/runner/runner_exec_util.go`), not a new one |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No wire path |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | `registerRoot("fail-syscall", failsyscall.Run, ...)` in `internal/test/cli/register.go`, the same line every other `ze-test` verb takes. `syscallsByName` is a map inside the launcher's own package, and each entry carries its own probe function, so adding a syscall edits no switch |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A kernel fault-injection framework can fail one process's socket reads without tracing it | docs.kernel.org/fault-injection describes `fail_function` and per-task filtering | The mechanism is something else, and this spec's first phase is to find it | Arm it in the QEMU guest and watch a read fail | BROKEN as written, and the mechanism is found. `CONFIG_FAULT_INJECTION` and `CONFIG_KPROBES` are both `is not set` in `tmp/kernel/build/config`, so `fail_function`, `/proc/<pid>/make-it-fail` and `/sys/kernel/debug/error_injection/list` do not exist on ze's runtime kernel. The mechanism is a classic seccomp filter answering `SECCOMP_RET_ERRNO`, installed by a launcher that then execs the daemon. Observed in the guest on 2026-09-11: ze's own listener logged `l2tp: listener read error ... recvfrom: network is down` on 127.0.0.1:17010 with `TracerPid: 0` and `Seccomp: 2, Seccomp_filters: 1` |
| A-2 | Ze's runtime kernel can carry the config symbols it needs | `tmp/kernel/build/config` is Ze's own kernel build | A second kernel flavor is needed for the test, or the mechanism is rejected | Read the config and the kernel build inputs | VALIDATED, no kernel change. `tmp/kernel/build/config` carries `CONFIG_SECCOMP=y`, `CONFIG_SECCOMP_FILTER=y` and `CONFIG_HAVE_ARCH_SECCOMP_FILTER=y`, and kernel 7.2 ran the filter in the guest. Neither symbol is requested in `gokrazy/kernel/runtime.config` nor pinned in `gokrazy/kernel/runtime.require`, so both arrive from the base defconfig and nothing fails the build if one goes away: design decides whether to pin them PINNED on 2026-09-11: `CONFIG_SECCOMP` and `CONFIG_SECCOMP_FILTER` are now lines in `gokrazy/kernel/runtime.require`, which `enforceKernelRequirements` (`internal/appliance/kernelreq.go`) reads against the RESOLVED config, so a defconfig that stopped setting either fails the kernel build. `TestRuntimeConfigPairing` (`internal/appliance/kernelconfig_pairing_test.go`) passes with the two added |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The mechanism fails reads for every process, so the harness fails too | The test runner dies before the daemon does | Filter by task, which is what the per-task knobs exist for |
| R-2 | The measurement is still swamped, by the injection rather than by ptrace | The pre-fix and fixed builds agree again | Measure the instrument first, against a deliberately spinning stub, before trusting a verdict CLOSED. Measured in the guest on 2026-09-11 against a pacer-reverted ze rather than a stub, under one armed filter over the same 3s window: paced 12 read errors at 0.0133 of one core, unpaced 1,053,776 read errors at 2.1067 of one core |

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
| `TestArmFailsOnlyTheSelectedReadLength` | `internal/test/failsyscall/failsyscall_linux_test.go` | AC-1 | PASS (guest, linux/arm64) |
| `TestArmRefusesWhatItCannotSelect` | `internal/test/failsyscall/failsyscall_linux_test.go` | AC-1, arming is loud | PASS (guest) |
| `TestBuildFilterJumpsLandOnTheAllow` | `internal/test/failsyscall/failsyscall_linux_test.go` | AC-1 | PASS (guest) |
| `TestRunRestoresStderrBeforeExec` | `internal/test/failsyscall/failsyscall_linux_test.go` | AC-4 | PASS (guest) |
| `TestParseOptions*` | `internal/test/failsyscall/options_test.go` | AC-1 | PASS (host and guest) |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| failure window | the launched daemon's own lifetime, which the runner ends with the test | 3s, the fixture's window | N/A | N/A. The filter lives in the daemon's process and dies with it, so no window outlives the test |
| read length selector | 0 or above; `lengthUnset` (-1) means every read | 1500 (rxBufLen) | -1 or below, refused by `options.set` | no ceiling: an unmatched length simply selects nothing, which the launcher's own probe refuses |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `subscriber-reader-failing-socket` | `test/l2tp/subscriber-reader-failing-socket.ci` | A subscriber socket fails persistently and the daemon's CPU stays low | GREEN on this tree, RED on the pacer-reverted build. Both observed in the guest on 2026-09-11 |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | No protocol behavior changes | N-A |

## Files to Modify
- `test/draft/l2tp/subscriber-reader-failing-socket.ci` - REMOVED, rewritten onto the mechanism as `test/l2tp/subscriber-reader-failing-socket.ci`
- `.gitignore` - the `test/draft/l2tp/` exception is deleted with the move
- `test/draft/README.md` - its tracked-exception table loses that row and names where the finding went
- `internal/test/cli/register.go` - registers the `fail-syscall` root
- `internal/test/fixture/tunnel_fixture_l2tp_failing_socket.go` - the header described the ptrace stimulus and claimed no gated caller; both are now false
- `gokrazy/kernel/runtime.require` - pins `CONFIG_SECCOMP` and `CONFIG_SECCOMP_FILTER`
- `docs/architecture/testing/qemu-integration.md` - a new "Failing ONE Socket Under a Daemon" section
- `plan/journal/output-lost-to-an-exit-past-the-flush.md` - one row for the `execve`-past-the-flush defect this work walked into

## Files to Create
- `internal/test/failsyscall/options.go` - the keyword grammar
- `internal/test/failsyscall/failsyscall_linux.go` - `arm`, `buildFilter`, `proveArmed`, `Run`
- `internal/test/failsyscall/failsyscall_other.go` - the refusal off Linux
- `internal/test/failsyscall/options_test.go`, `failsyscall_linux_test.go`
- `test/l2tp/subscriber-reader-failing-socket.ci`

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
- The instrument was the problem TWICE. ptrace destroyed the CPU reading, and
  the replacement destroyed the LOG reading: `crashlog.Init` dup2s a pipe over
  fd 2 in every `cmd/ze` binary, and an `execve` leaves the launched daemon
  writing into a pipe whose reader died with the image. The daemon served
  metrics, counted read errors, and wrote not one log line. The only reason it
  was caught is that the scenario asserts the LOG as well as the counter, which
  is an argument for keeping an assertion that looks redundant.
- A window that fails to arm cannot be detected by the test that depends on it,
  so the launcher detects it: it makes the selected call itself, plus one call
  of a length it must not select, and refuses to `execve` if either answer is
  wrong.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Kernel fault injection | ptrace, `LD_PRELOAD` | ptrace was measured and destroys the signal; `LD_PRELOAD` reaches nothing under `CGO_ENABLED=0` |
| A classic seccomp filter answering `SECCOMP_RET_ERRNO` | `fail_function`, `/proc/<pid>/make-it-fail` | A-1: `CONFIG_FAULT_INJECTION` and `CONFIG_KPROBES` are both off in ze's runtime kernel, and seccomp is already on |
| The launcher is a `ze-test` verb, `ze-test fail-syscall`, in `internal/test/failsyscall` | a separate `cmd/ze-failsyscall` binary reached by a `.ci` `exec=` line | `ai/rules/simplicity.md`: `ze-test` is already built, already cross-compiled and already on the PATH the `.ci` runner gives its children, so the verb adds one `registerRoot` line and no build wiring. A separate binary would need a cross-compile target, a staging step into the guest, and its own place in `runnableInGuest`. The cost the verb DOES carry is that `ze-test` calls `crashlog.Init`, which the launcher has to undo before its `execve`; that is one call and one test |
| The name says what it does, not which test used it first | `failsock`, `l2tp-fail-reader` | The Critical Review Checklist asks for it, and the mechanism is not about sockets: it fails a named syscall for a named process |
| Selection is by read LENGTH only, never by file descriptor | select by `args[0]`, the fd | A `.ci` cannot know a daemon's fd number in advance, so an fd selector would have no caller. The length is a per-loop constant a test CAN name (1500 for the l2tp listener) |
| `length` is refused on `recvmsg` | compare `args[2]` on every syscall alike | `recvfrom(fd, buf, len, flags, ...)` carries the length in `args[2]`; `recvmsg(fd, msghdr, flags)` carries the FLAGS there. Comparing them alike would select sockets by a flag word and report that it armed |
| The launcher proves it armed, and refuses to launch anything if it did not | trust the install and let the test find out | `ai/rules/principles.md`. A filter that installs and selects nothing leaves the daemon healthy while the test reports an armed window, which is the vacuity this whole spec exists to escape. The proof is a positive probe plus one negative control, made in the launcher before the `execve` |

## Known Limitations
- The launcher knows two syscalls, `recvfrom` and `recvmsg`. A third is one map
  entry, and the entry MUST state whether `args[2]` of that call is a read
  length before a caller may select on it.
- On a RED run the daemon's DEBUG flood fills the runner's 10 MB stderr capture
  (`maxOutputBytes`), so the fixture's own numeric diagnostic is evicted from
  the report. The verdict is unaffected: the step trace names the failing
  assertion and the flood itself is the evidence of the spin. No memory or disk
  is at risk, because the capped writer keeps draining the pipe and reports the
  caller's full length.

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

---

## Implementation Summary

### What Was Implemented
- `internal/test/failsyscall`, reached by `ze-test fail-syscall`. `arm` installs a
  classic seccomp BPF filter answering `SECCOMP_RET_ERRNO | <errno>` for one named
  syscall, `proveArmed` makes the selected call plus one negative control before
  anything is launched, and `Run` then `execve`s the command after `--`. No tracer,
  so the refused call is charged to the launched process's own CPU.
- `internal/test/cli/register.go` registers the root.
  `internal/test/failsyscall/failsyscall_other.go` refuses off Linux, so the verb
  list is the same on every host.
- `gokrazy/kernel/runtime.require` pins `CONFIG_SECCOMP` and `CONFIG_SECCOMP_FILTER`,
  read by `enforceKernelRequirements` (`internal/appliance/kernelreq.go`) against the
  resolved config.
- `test/l2tp/subscriber-reader-failing-socket.ci` replaces the ptrace scenario, which
  was removed from `test/draft/l2tp/` along with its `.gitignore` negation and its
  `test/draft/README.md` row. It is now gated: `./le functional select` lists `l2tp`
  among the 27 suites a gating run starts.
- `docs/architecture/testing/qemu-integration.md` gains "Failing ONE Socket Under a
  Daemon", with the keyword table and the three constraints a caller has to know.

### Bugs Found/Fixed
- `crashlog.Init` (`internal/core/crashlog/crashlog.go`, `redirectStderr` in
  `internal/core/crashlog/stderr.go`) dup2s a pipe over fd 2 and drains it from a
  goroutine that an `execve` destroys. The launched daemon writes into a pipe nobody
  reads, loses its whole log, and blocks once 64 KiB accumulate. `Run` now calls
  `crashlog.Flush()` immediately before `unix.Exec`, and `crashlog.Flush` restores
  the saved descriptor with `dupStderr(int(origStderr.Fd()))`. Covered by
  `TestRunRestoresStderrBeforeExec`, which drives the whole launcher with
  `crashlog.Init` armed. Four sibling `execve` sites carry the same defect and are
  NOT fixed here; the row is `plan/journal/output-lost-to-an-exit-past-the-flush.md`.

### Documentation Updates
- `docs/architecture/testing/qemu-integration.md`: new section, carrying
  `<!-- source: internal/test/failsyscall/failsyscall_linux.go -- arm, buildFilter, proveArmed -->`
  and a `<!-- test: ... -->` anchor naming the scenario.
- `test/draft/README.md`: two tracked exceptions become one, and the row says where
  the finding went.
- `./le doc check links`: 42 broken references tree-wide, none of them produced by
  this change. The one `continue.md` reference to the moved `.ci` is repaired here.

### Deviations from Plan
- Selection by file descriptor was dropped at design time: a `.ci` cannot know a
  daemon's fd in advance, so an fd selector would have had no caller. Selection is
  by read LENGTH only, and `length` is refused on `recvmsg`, whose `args[2]` is the
  flags word rather than a length.
- `usageLine` lives in `failsyscall_linux.go` rather than `options.go`, because
  `options.go` carries no build tag and the const has no user off Linux.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1 assumed a kernel fault-injection framework (`fail_function`, `/proc/<pid>/make-it-fail`) could fail one process's socket reads | `CONFIG_FAULT_INJECTION` and `CONFIG_KPROBES` are both `is not set` in `tmp/kernel/build/config`, so none of those interfaces exists on ze's runtime kernel | Read the config before arming anything | A classic seccomp filter, which the same kernel already carries (`CONFIG_SECCOMP=y`) |
| approach | The launcher was expected to hand the daemon a working stderr simply by `execve`-ing it | `crashlog.Init` had already put a pipe on fd 2 whose reader dies with the image | The scenario's `expect=stderr:` assertion went red while both `/metrics` assertions passed | `crashlog.Flush()` before `unix.Exec`, plus the journal row for the four unfixed siblings |
| escalation | The lint gate selects packages from TRACKED files, so a brand-new untracked package is linted by nothing until it is committed | `golangci-lint` run directly over `./internal/test/failsyscall/...` reported 5 findings the gate had not seen | Closure ran the linter over the new package by hand | All five fixed here. A new package is worth linting directly before its first commit |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A `.ci` can fail one named socket for a bounded window without tracing the daemon | Done | `internal/test/failsyscall/failsyscall_linux.go` (`arm`, `Run`) | `TracerPid: 0`, `Seccomp: 2`, `Seccomp_filters: 1` observed in the guest |
| A process-level CPU or rate assertion discriminates a paced loop from a spinning one | Done | `internal/test/fixture/tunnel_fixture_l2tp_failing_socket.go` | 0.0133 of a core / 12 errors against 2.1067 / 1,053,776 over the same 3s window |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestArmFailsOnlyTheSelectedReadLength` | A 1500-byte read answers `ENETDOWN`; a 256-byte read answers `EAGAIN` from the kernel |
| AC-2 | Done | guest discrimination run, 2026-09-11 | pacer-reverted build: `FAIL 27 subscriber-reader-failing-socket`, exit 1, 7507 bytes captured |
| AC-3 | Done | same run, same boot | this tree: `PASS 27 subscriber-reader-failing-socket`, exit 0 |
| AC-4 | Done | `cmd=background:seq=1:exec=ze-test fail-syscall ... -- ze -` | the step trace records the ordinary `.ci` launch, with no wrapper and no hand-driven step |
| AC-5 | Done | `gokrazy/kernel/runtime.require` | the guest reported `Linux localhost 7.2.0 ... aarch64`, and the filter installed |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestArmFailsOnlyTheSelectedReadLength` | Done | `internal/test/failsyscall/failsyscall_linux_test.go` | PASS in the guest |
| `TestArmRefusesWhatItCannotSelect` | Done | same | three sub-cases, all PASS |
| `TestBuildFilterJumpsLandOnTheAllow` | Done | same | both filter shapes; every `jf` lands on the allow |
| `TestRunRestoresStderrBeforeExec` | Done | same | PASS in the guest |
| `TestParseOptions*` | Done | `internal/test/failsyscall/options_test.go` | nine refusal cases plus the two ceiling cases; PASS on host and guest |
| `subscriber-reader-failing-socket` | Done | `test/l2tp/subscriber-reader-failing-socket.ci` | GREEN on this tree, RED on the pacer-reverted build |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/test/failsyscall/*.go` | Done | five files |
| `test/l2tp/subscriber-reader-failing-socket.ci` | Done | tracked; `git check-ignore -v` exits 1 on it |
| `test/draft/l2tp/subscriber-reader-failing-socket.ci` | Done | removed, with the `.gitignore` negation and the README row |
| `internal/test/cli/register.go` | Done | one `registerRoot` line |
| `gokrazy/kernel/runtime.require` | Done | two pinned symbols |
| `docs/architecture/testing/qemu-integration.md` | Done | new section |
| `plan/journal/output-lost-to-an-exit-past-the-flush.md` | Done | one row |

### Audit Summary
- **Total items:** 20
- **Done:** 20
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (fd selection dropped at design time; `usageLine` placement)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A `.ci` test can fail one named socket for a bounded window without tracing the daemon | functional | `test/l2tp/subscriber-reader-failing-socket.ci` passes in the QEMU guest on kernel 7.2.0. The launched process reports `TracerPid: 0`, `Seccomp: 2`, `Seccomp_filters: 1` |
| A process-level CPU or rate assertion discriminates a paced loop from a spinning one | functional, both polarities in ONE guest boot | paced `exit=0` / `PASS`, 12 read errors, 0.0133 of one core; pacer-reverted `exit=1` / `FAIL`, 1,053,776 read errors, 2.1067 of one core. The fixture's bounds (1000 errors, 0.5 of a core) sit between them |
| The mechanism is reusable by the next spec that needs it | wiring | `registerRoot("fail-syscall", failsyscall.Run, ...)` in `internal/test/cli/register.go`; the keyword grammar and its three constraints are documented in `docs/architecture/testing/qemu-integration.md` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| The four sibling `execve` sites that lose stderr to the crashlog pipe: `defaultRestart` (`internal/component/config/system/selfupdate.go`), `loginShell` (`cmd/ze/login.go`), `internal/le/terminaldemo/runtime.go`, `internal/test/fixture/ui_fixture_update_serve.go` | The goal this spec exists to achieve does not depend on them, and `ai/rules/rule-precedence.md` forbids fixing an unrelated defect on the way to closing the work in hand | `plan/journal/output-lost-to-an-exit-past-the-flush.md`. It is one problem class with the 2026-09-06 row, and the source repair both rows name is the same decision, so the class earns one fix rather than two specs |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/failing-socket-proof-needs-a-non-ptrace-injection-point-8a072de8-ce8e-47de-bdd2-015f5b44ca51.md`, hash-pinned over 12 files. The prose of both runs is beside it at `tmp/session/2026-09-08-8a072de8-ce8e-47de-bdd2-015f5b44ca51/scratch/review-failing-socket.md` |
| `./le spec session review check` | `review_gate: OK (8 code files, clean, hashes match ...)` |
| Rounds | 2 |
| Reviewer lenses used | wiring and vacuity (can the gated test pass while proving nothing), guard driven from its entry point, the style pass over every changed Go file, allocation and bounds, the security surface of the injected filter, documentation drift, citation clearing |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `length` had no ceiling. `buildFilter` casts it with `uint32(readLength)`, so a value above 2^32 arms a filter for a length the caller never named, and `proveArmed` allocates `make([]byte, probeLength)` of whatever the `.ci` line says | `internal/test/failsyscall/options.go`, `options.set` | `readLengthMax` (1 MiB) refused in `options.set`, with `TestParseOptionsAcceptsTheCeiling` on the limit and a refusal case one byte above it |
| 2 | ISSUE | Nothing said the `expect=stderr:` line is load-bearing, so it reads as a third copy of the two `/metrics` assertions and would be deleted as noise. Deleting it lets the scenario pass with the daemon writing no log at all, which is what was observed on 2026-09-11 | `test/l2tp/subscriber-reader-failing-socket.ci` | A `DO NOT DELETE THE expect=stderr LINE` paragraph in the header, naming `crashlog.Flush` and the journal row |
| 3 | ISSUE | The journal row named `selfUpdateExec` in `internal/component/config/system/selfupdate.go`. No such symbol: `gopls symbols` gives `defaultRestart`, and that is the function calling `syscall.Exec` | `plan/journal/output-lost-to-an-exit-past-the-flush.md` | Corrected to `defaultRestart` |
| 4 | ISSUE | `plan/journal/green-that-could-not-have-been-red.md` states the scenario "is kept, tracked and gated by nothing, at the path in the Surface cell" and names this spec as owner. Both stop being true at this closure, and the path it gives is deleted | that file, the 2026-09-11 row | The cell now records the settlement, the new path, and that the l2tp suite gates it |
| 5 | ISSUE | Five lint findings in the new package that `./le verify lint run` could not see, because it selects packages from TRACKED files and the package was untracked: errorlint, gosec G103, noctx twice, unused | `internal/test/failsyscall/` | `%v` kept with a reason (a probe the filter never touched answers a nil error, which `%w` cannot render), `//nolint:gosec` on the `sock_fprog` address, `exec.CommandContext(t.Context(), ...)`, `usageLine` moved beside its only user. `golangci-lint` over the package on linux/amd64, linux/arm64 and darwin/arm64: 0 issues |
| 6 | ISSUE | `continue.md` and `plan/learned/017-...` cite the spec path that commit B deletes, and `continue.md` also cites the deleted `test/draft/l2tp/` scenario | both files | Restated with the bare stem, and repointed at `test/l2tp/subscriber-reader-failing-socket.ci` |

### Notes (do not block)
- `option=needs-linux` means the scenario is skipped on a darwin host, so a gating
  run there reports it skipped rather than run. That is the existing shape of
  `test/l2tp/radius-acct-wire.ci` and `test/l2tp/session-stopccn-cascade.ci`, not
  something this change introduced.
- The filter compares the low 32 bits of `args[2]`, so on a little-endian target a
  read whose length shares its low word with the selector would also fail.
  `readLengthMax` makes every length the keyword accepts fit that word exactly.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/test/failsyscall/failsyscall_linux.go` | Yes | `ls -la` -> `13K Sep 11 06:26` |
| `internal/test/failsyscall/failsyscall_other.go` | Yes | `ls -la` -> `632 Sep 11 05:32` |
| `internal/test/failsyscall/options.go` | Yes | `ls -la` -> `4.1K Sep 11 06:26` |
| `internal/test/failsyscall/options_test.go` | Yes | `ls -la` -> `2.9K Sep 11 06:19` |
| `internal/test/failsyscall/failsyscall_linux_test.go` | Yes | `ls -la` -> `5.7K Sep 11 06:25` |
| `test/l2tp/subscriber-reader-failing-socket.ci` | Yes | `ls -la` -> `3.9K Sep 11 06:20`; `git check-ignore -v` exits 1, so no ignore rule swallows it |
| `test/draft/l2tp/` | No, by design | `ls test/draft/l2tp` -> `No such file or directory` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | reads on the selected socket fail, with no ptrace | guest run 2026-09-11: `--- PASS: TestArmFailsOnlyTheSelectedReadLength (0.00s)`, which asserts `ENETDOWN` on 1500 bytes and `EAGAIN` on 256 |
| AC-2 | the pacer-reverted build goes RED | `== case unpaced exit=1 captured=7507 bytes`, `FAIL 27 subscriber-reader-failing-socket` |
| AC-3 | the fixed build goes GREEN | `== case paced exit=0 captured=385 bytes`, `PASS 27 subscriber-reader-failing-socket` |
| AC-4 | the ordinary `.ci` launch reaches it | step trace: `exec /root/zb/ze-test fail-syscall syscall recvfrom errno ENETDOWN length 1500 -- ze -  [stdin=config piped]`, status pass |
| AC-5 | ze's runtime kernel carries what it needs | `Runtime kernel confirmed in the guest: 7.2`; `Seccomp: 2`, `Seccomp_filters: 1` in `/proc/<pid>/status` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| a `.ci` asking for a failing socket | `test/l2tp/subscriber-reader-failing-socket.ci` | Yes. Read the file: `cmd=background:seq=1` launches `ze-test fail-syscall ... -- ze -`, `cmd=foreground:seq=2` runs the fixture against `$PORT2`, and the three `expect=` lines cover CPU, the counter and the log. `registerRoot("fail-syscall", failsyscall.Run, ...)` is the non-test caller of the package's one exported symbol, so `./le repository check` reports nothing against it |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken, and replaced | `CONFIG_FAULT_INJECTION` and `CONFIG_KPROBES` are `is not set` in `tmp/kernel/build/config`. The mechanism is a classic seccomp filter; Mistake Log row 1 |
| A-2 | confirmed | `CONFIG_SECCOMP=y` and `CONFIG_SECCOMP_FILTER=y` resolve, kernel 7.2 ran the filter, and both symbols are now pinned in `gokrazy/kernel/runtime.require` where `enforceKernelRequirements` reads them |
| R-1 | did not occur | The filter is installed in the launcher and inherited only through its own `execve`. The runner and the fixture are separate processes and were unaffected in both cases of the discrimination run |
| R-2 | closed | The two builds separate by 158x on CPU and by four orders of magnitude on the counter, measured under one armed filter over the same 3s window |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Test infrastructure changed (checklist row 10) | `docs/architecture/testing/qemu-integration.md`, "Failing ONE Socket Under a Daemon"; `test/draft/README.md` now names one tracked exception | Yes |
| "the runner's `ze`-only arms do not fire" | `zeReadyFileEnabled` (`internal/test/runner/runner_exec_util.go`) returns false unless `binName == binNameZe`, and `ze.storage.blob=false` is appended under the same guard (`internal/test/runner/runner_exec.go`). The launched binary is `ze-test`, so the `.ci` sets the variable itself | Yes |
| "`length` reads `args[2]`, which is a read length only for some calls" | `syscallsByName` carries `arg2IsReadLength` per entry and `arm` refuses the pair; `TestArmRefusesWhatItCannotSelect/length_on_a_syscall_whose_third_argument_is_not_a_length` PASS | Yes |
| No doctor check owed | Nothing the shipped daemon depends on: `internal/test/cli` is imported only from `cmd/ze/ze_test_register.go`, which is `//go:build ze_test`, so `failsyscall` reaches no shipped image | Yes |
| No RFC row owed | No protocol behavior changes; the diff carries no `// RFC` comment and no `rfc/short/` edit | Yes |

## Core Insight

The instrument that replaces a broken instrument brings its own defect, and the
assertion that catches it is the one that looks redundant. ptrace destroyed the
CPU reading; the seccomp launcher that replaced it destroyed the LOG reading,
because `execve` outlives the goroutine draining the crashlog pipe on fd 2. Both
failures were silent, because the daemon served metrics and counted errors
throughout. Only `expect=stderr:contains=` failed, and only because the scenario
asserts the log beside the counter.

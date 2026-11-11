# Spec: stress-children-die-with-harness

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | A stress harness killed outright leaves ze, BIRD, test peers and their network namespaces running as root, holding ports and addresses that redden later stress and functional runs, the evidence the release depends on |
| RFC gaps | - |
| Depends | - |
| Phase | 2/2 |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

**Cap exception (2026-11-11).** Thomas authorised starting this spec above the
WIP cap ("ignore the cap"); it was claimed at 19/19.

## Task

**Symptom (read at the producer, 2026-11-11).** The stress harness
(`./le test integration stress` and `stress-bird`, `internal/le/test/integration/`)
starts every child through one system implementation, `realStressBirdSystem`
(`stressbird.go`; `stress.go`'s `realStressSystem` embeds it): `Start` for the
long-lived ones (`le test peer --mode sink` and `--mode inject`, ze, BIRD,
tcpdump, the CPU-profile curl), `Run` for the short ones (`ip`, `ethtool`,
`birdc`, `ss`, the DUT `go build`). Both call `exec.CommandContext` with no
`SysProcAttr`: no process group of its own and no parent-death signal. Cancel,
`Terminate` and `Kill` signal the direct pid only.

**Consequence.** A harness that dies without running Go code (SIGKILL of its
pid, the OOM killer, a torn-down session) leaves every child reparented to
init, running as root, with its `ze-stress-ze-<pid>` and `ze-stress-bb-<pid>`
namespaces. The BIRD gate is worse: `runStressBirdGate` drops the signal
context `nativeRunner` hands it and `runStressBirdAction` runs under
`context.Background()`, so a SIGINT or SIGTERM to the harness ends it with
Go's default action and no teardown at all.

**Sibling already fixed.** The `.ci` runner had the same gap, fixed by
`spec-test-children-die-with-runner` (commit c195efcf1b): its children fork
with `plugin.NewSysProcAttr` (own group, SIGKILL on parent death) through
`plugin.StartChild`. `docs/architecture/testing/runner-architecture.md`,
"Three things end a process", states the rule every runner owes.

**Goal.** Every process the stress harness starts dies when the harness dies,
however it dies, and a SIGINT or SIGTERM to the harness runs its teardown on
every gate.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/runner-architecture.md` - "Three things end a process", the rule every runner owes
  → Constraint: a child's death signal is bound to the forking THREAD, so a fork carrying `Pdeathsig` goes through `plugin.StartChild`, whose spawner thread stays locked for the life of the process
  → Decision: the stress children need no namespace-thread fork: `ip netns exec` switches namespace itself, so every stress fork runs from the spawner thread in the host namespace
- [ ] `docs/functional-tests.md` - the stress harness section, its cleanup paragraph (`stressNamespacePath` anchor)
  → Constraint: this page documents the harness lifecycle; the lifetime guarantee is added beside its cleanup anchor in the same diff as the code
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md`, `ai/doors/plugin-infra.md` - the doors this work crosses
  → Constraint: `internal/le` may import `internal/component/plugin`; product code never imports `internal/le`, and this change adds no import in that direction

**Key insights:**
- `ip netns exec <ns> <cmd>` outside batch mode execs its target without forking (iproute2 v6.1.0 `ip/ipnetns.c` `netns_exec` calls `cmd_exec(argv[1], argv + 1, !!batch_mode, ...)`; `lib/exec.c` `cmd_exec` forks only when `do_fork`), so the target IS the forked child and inherits its group and its death signal
- The kernel clears the death signal whenever a process's effective credentials change (`commit_creds`), by a setuid exec or by a setuid call after the exec. The harness execs no setuid binary; tcpdump (which a Debian or Ubuntu build switches to its own user) runs with `-Z root`, and ze starts without `ze.user`, so no child switches user (corrected at closure: the design read covered only the exec)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/test/integration/stressbird.go` - `realStressBirdSystem.Start` and `Run` (plain `exec.CommandContext`, no `SysProcAttr`), `realStressBirdProcess.Terminate`/`Kill` (direct pid only), `runStressBirdAction` (`context.Background()`), `stressBirdRunner.cleanup` (Terminate, bounded Wait, Kill, then `ip netns del`)
- [ ] `internal/le/test/integration/actions.go` - `nativeRunner` (roots each native verb on `signalContext`), `runStressBirdGate` (discards that context), `runStressGate` (keeps it)
- [ ] `internal/le/test/integration/stress.go` - `runZeStressScenario` embeds `stressBirdRunner` and starts receiver, ze, tcpdump, peer and CPU profile through the same system; `realStressSystem` embeds `realStressBirdSystem`
- [ ] `internal/component/plugin/sysproc.go`, `sysproc_linux.go`, `sysproc_other.go` - `KillGroupOnCancel`, `KillProcessGroup`, `NewSysProcAttr`, `StartChild`
- [ ] `internal/test/runner/process_death_linux_test.go` - the re-exec stand-in pattern the new tests follow

**Behavior to preserve:**
- The recorder-fixture call order every `TestStressBird*` test asserts: this change is inside `realStressBirdSystem` and the gate root, and the scenario control flow is untouched
- Cleanup's order and bounds: Terminate, wait `stressBirdStopTimeout`, Kill, wait, then namespace deletion and file removal
- `Run`'s result contract: stdout, stderr, exit code, and the context error when its timeout or cancel ended it
- The checkout root the BIRD gate resolves (`nativeRunner` hands the toolchain root, the same checkout `lepath.Root` resolves)

**Behavior to change:**
- Every stress child forks in a process group of its own with SIGKILL on harness death, from the spawner thread
- Context cancel, `Terminate` and `Kill` signal the child's whole group
- The BIRD gate runs under the signal context `nativeRunner` provides

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Plugin fork attributes | `internal/component/plugin/sysproc_linux.go` `NewSysProcAttr`, `sysproc.go` `KillGroupOnCancel` | Own process group plus `Pdeathsig` SIGKILL, and a cancel that SIGKILLs the group | extends: the stress system's `Start` and `Run` take these attributes through `KillGroupOnCancel`, the pairing that file requires |
| Spawner thread | `internal/component/plugin/sysproc_linux.go` `StartChild` | Forks from one thread locked for the process's life, so the death signal fires only when the process dies | extends: every stress fork goes through it, as the file's contract requires of any fork carrying `NewSysProcAttr` |
| Group kill | `internal/component/plugin/sysproc.go` `KillProcessGroup` | SIGKILLs the group a pid heads | extends: `realStressBirdProcess.Kill` uses it; `Terminate` sends SIGTERM to the same group |
| Native gate signal root | `internal/le/test/integration/actions.go` `nativeRunner`, `signalContext` | Cancels the verb's context on SIGINT or SIGTERM so deferred teardown runs | extends: the BIRD gate stops discarding that context |
| `.ci` runner start | `internal/test/runner/runner_exec_util.go` `startWithETXTBSYRetry` | The sibling fix of the same defect | does not apply: its ETXTBSY retry and netns-thread choice answer `.ci` needs the stress harness does not have; the shared part is the plugin mechanism both use |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test integration stress` or `stress-bird`, a native verb run by `nativeRunner` under `signalContext`

### Transformation Path
1. `nativeRunner` roots the verb on a SIGINT/SIGTERM context and calls the gate with the checkout root
2. The gate runs `runStressBird` or `runStressAction` with the real system
3. The runner calls `system.Run` / `system.Start` with an argv, usually `ip netns exec <ns> <binary> ...`
4. The system forks the child through `plugin.StartChild` with the plugin fork attributes; `ip` execs the target in place
5. Cleanup or cancel signals the child's group; the kernel SIGKILLs the child if the harness dies first

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Harness ↔ child process | fork from the spawner thread, exec through `ip netns exec` | Yes: iproute2 `cmd_exec` execs without forking outside batch mode |
| Operator signal ↔ gate | `signal.NotifyContext` in `signalContext` | Yes: `TestNativeRunnerCancelsOnSIGINT` |

### Changed-Wire Census

**Search evidence:** No wire changes. The change alters how processes are forked and signaled; no message, file format, report field or command-line grammar changes. Searched `internal/le/test/integration/` for `StressBirdReport` writers and readers: the report struct and its JSON tags are untouched.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| stress JSON report | writer | `stressbird.go` `StressBirdReport` | struct unchanged by this spec | preserve: no field changes |
| stress JSON report | reader | `internal/le/test/qemu` stress report copy (reads the file the guest wrote) | reads the report file only | preserve: no field changes |

### Integration Points
- `plugin.KillGroupOnCancel`, `plugin.StartChild`, `plugin.KillProcessGroup` - called from `realStressBirdSystem` and `realStressBirdProcess`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | every stress fork still goes through `stressBirdSystem` |
| No unintended coupling (components stay isolated) | Yes | `internal/le` importing `internal/component/plugin` is the permitted direction (`ai/doors/le-tooling.md`) |
| No duplicated functionality (extends existing, does not recreate) | Yes | fork attributes, spawner and group kill come from the plugin package |
| Zero-copy preserved where applicable (refs, not copies) | N-A | no data path |
| Registration over hardcoding, outbound | N-A | no command, family or handler added |
| Registration over hardcoding, inbound | N-A | no name added to any list |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `ip netns exec` execs its target in place, so the death signal set on the forked `ip` reaches ze, BIRD and the peer | iproute2 v6.1.0 `ip/ipnetns.c` `netns_exec`, `lib/exec.c` `cmd_exec` | the target is a grandchild and outlives a killed harness | source read (done); the host runs iproute2 6.1.0 (`ip -V`) | confirmed |
| A-2 | No stress child relies on sharing the harness's process group | grep of `stressbird.go`, `stress.go` for `Getpgid`/`Setpgid`/group signals: none | a child stops receiving a terminal's group signal | grep (done) | confirmed |
| A-3 | Moving each child to its own group does not let `./le job`'s group stop miss it | `Pdeathsig` kills the child when the harness dies, which the job's group SIGKILL causes | children outlive a stopped job | AC-1 test kills the stand-in harness by SIGKILL | confirmed: `TestStressHarnessKilledOutrightTakesItsChildren` red before the fix (child alive 5 s after the harness's SIGKILL), green after |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A child forked on a goroutine's own thread dies when that thread is destroyed | a stress child ends mid-round with SIGKILL | every fork goes through `plugin.StartChild`; AC-4 test |
| R-2 | `Terminate` to the group hits a group whose leader was already reaped | ESRCH from `Kill(-pid)` | both stops check `done` first, as today; a pid is not reused while its group has members |
| R-3 | Group SIGKILL on `Run` timeout ends a `go build` mid-write | a half-written DUT binary | the next run rebuilds it from the checkout every time (`stressRunner.buildZe`) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Stress runs: a child dying early reddens a stress round. No shipped behavior |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `internal/le/test/qemu` drives the same harness inside a guest; it reads only the report |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A harness process that started a stress child is SIGKILLed | → | `realStressBirdSystem.Start` | `TestStressHarnessKilledOutrightTakesItsChildren/start` |
| A harness process killed during a short stress command | → | `realStressBirdSystem.Run` | `TestStressHarnessKilledOutrightTakesItsChildren/run` |
| `./le test integration stress-bird` receives SIGINT | → | `stressBirdGate` (replaces `runStressBirdGate`) → `runStressBird` | `TestStressBirdGateRunsUnderTheSignalContext` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A process that started a child through `realStressBirdSystem.Start` is SIGKILLed | The child is dead by SIGKILL within 5 s | The namespaces and `/tmp` files the run created: they hold no process, and cleaning another run's leftovers is out of this spec (Known Limitations) |
| AC-2 | A process running a command through `realStressBirdSystem.Run` is SIGKILLed | The command's process is dead by SIGKILL within 5 s | Same as AC-1 |
| AC-3 | `Terminate`, then `Kill` if needed, on a started child that has its own child | The child and its child are both gone once `Wait` returns | None: a stop that leaves part of the child's tree running is the defect |
| AC-4 | A child is started from a goroutine that locks its thread and returns still locked | The child keeps running while the harness lives | None: the harness owns the child's lifetime until its stop |
| AC-5 | `Run` with a timeout on a command that has its own child | When `Run` returns with the deadline error, the command and its child are both gone | None: the timeout is the stop |
| AC-6 | The BIRD gate's context is canceled (SIGINT or SIGTERM to the harness) | The run stops at its next wait, reports the cancel, and runs its cleanup (process stops, namespace deletion, file removal) | None: teardown is owed on every signal |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStressHarnessKilledOutrightTakesItsChildren` (subtests `start`, `run`) | `internal/le/test/integration/stressbird_death_linux_test.go` | AC-1, AC-2: re-exec stand-in harness, SIGKILL it, child must die | |
| `TestStressHarnessDeathStandIn` | same | helper process for the test above; skips in any other run | |
| `TestStressStopReachesTheChildsWholeGroup` | same | AC-3 | |
| `TestStressChildLivesWhileItsHarnessLives` | same | AC-4 | |
| `TestStressRunTimeoutReachesTheWholeGroup` | same | AC-5 | |
| `TestStressBirdGateRunsUnderTheSignalContext` | `internal/le/test/integration/stressbird_test.go` | AC-6, through a recorder whose waits honor the context | |
| `TestStressChildrenKeepTheirCredentials` | `internal/le/test/integration/stress_test.go` | AC-1 for ze and tcpdump: neither is asked to switch user, which clears the death signal (closure review) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | the change takes no numeric input | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | | The harness needs root and a quiet host (`ai/META-DECISIONS.md`, one root run at a time); the unit tests drive the real `realStressBirdSystem` without root, which is the boundary this spec changes | |

## Files to Modify
- `internal/le/test/integration/stressbird.go` - delete `runStressBirdAction`; the real system moves out (below)
- `internal/le/test/integration/actions.go` - `stressBirdGate(system)` replaces `runStressBirdGate` and runs `runStressBird` under the context and root it is handed
- `internal/le/test/integration/gates.go` - the `stress-bird` row uses `stressBirdGate(realStressBirdSystem{})`
- `docs/functional-tests.md` - the stress cleanup paragraph: the children's lifetime
- `internal/le/test/integration/stress.go` - `stressRunner.startZe` (closure review): ze starts without `ze.user`/`ze.group`, tcpdump with `-Z root`, so neither switches user and loses its death signal
- `internal/le/test/integration/stress_test.go` - `TestStressChildrenKeepTheirCredentials` (closure review)

## Files to Create
- `internal/le/test/integration/stressbird_system.go` - `realStressBirdSystem` and `realStressBirdProcess` moved out of `stressbird.go` (which passed 1000 lines with the change): `Start`, `Run` fork through `plugin.KillGroupOnCancel` and `plugin.StartChild`; `Terminate`, `Kill` signal the group
- `internal/le/test/integration/stressbird_death_linux_test.go` - AC-1 to AC-5

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | tooling, no config |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | N-A | no command or flag changes |
| CLI grammar (keyword before value) | N-A | no grammar change |
| Editor autocomplete | N-A | no YANG |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no output change |
| Env var registration | N-A | no env var |
| Doctor check for runtime dependencies | N-A | no new runtime dependency; development tooling |
| Doctor code per problem class | N-A | no new refusal or log line |
| Prometheus counters/metrics | N-A | no observable product state |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | development tooling only |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | the plugin package is called, not changed |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, the stress cleanup paragraph: children die with the harness, every gate tears down on SIGINT/SIGTERM |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | `runner-architecture.md` covers the `.ci` runner; the stress harness page is `functional-tests.md` |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/functional-tests.md` anchors `stressbird.go` (updated, row 10); `stressbird.go`'s `// Design:` names `stress.go`, which is unaffected; `docs/architecture/core-design.md`, declared by a changed file's `// Design:` header, is unaffected: it describes the product's core, and nothing here changes it; `docs/architecture/testing/interop.md`, declared by `gates.go`, is unaffected: the `stress-bird` row's verb, summary and requirements are unchanged, only its runner function |
| 17 | Existing docs show config/CLI/API examples for this area? | No | the stress invocation examples are unchanged |

## Implementation Steps

1. **Phase: Wiring and red tests** -- write the six tests; confirm each fails against the current code for the defect it names
   - Tests: all rows of the TDD Test Plan
   - Files: `stressbird_death_linux_test.go`, `stressbird_test.go`
   - Verify: AC-1, AC-2, AC-3, AC-5, AC-6 red; AC-4 green today (no death signal yet) and must stay green
2. **Phase: Fork and stop through the plugin mechanism** -- `Start` and `Run` take `KillGroupOnCancel` and fork through `StartChild`; `Terminate` and `Kill` signal the group; the BIRD gate keeps its context; the page paragraph
   - Tests: the same six
   - Files: `stressbird.go`, `actions.go`, `docs/functional-tests.md`
   - Verify: all six green; `go test -race ./internal/le/test/integration/` green

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation and its test |
| Correctness | Every fork carrying `NewSysProcAttr` goes through `StartChild`; no `cmd.Start`/`cmd.Run` left in `realStressBirdSystem` |
| Correctness | `Terminate` and `Kill` signal `-pid` only for a child forked with `Setpgid` (all of them now), and only before `done` closes |
| Correctness | `Run` still returns the context error on timeout and the exit code otherwise |
| Data flow | `stressBirdGate` passes its context and root through; nothing roots a stress run on `context.Background()` |
| Tests | each new test was red before the fix for the reason it names (paste the red) |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Stress children fork with the plugin attributes | `grep -n "StartChild\|KillGroupOnCancel" internal/le/test/integration/stressbird.go` |
| No background root left | `grep -n "context.Background" internal/le/test/integration/stressbird.go` answers nothing |
| Six tests green | `go test -race -run 'TestStress(HarnessKilled\|StopReaches\|ChildLives\|RunTimeout\|BirdGateRuns)' ./internal/le/test/integration/` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Signal scope | A group signal never reaches the harness's own group: every target group is one `Setpgid` created for the child |
| Root children | No root child can outlive the harness on the paths the ACs cover |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `guard-added-to-one-half-of-a-pair` | 2026-11-11 | stress harness `realStressBirdSystem.Start`: children with no group and no death signal, BIRD gate with no signal root |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Use the plugin package's attributes, spawner and group kill | A stress-local `SysProcAttr` and fork | The plugin package already owns the thread-bound death-signal contract; a second copy would drift from it |
| Fork every stress child from the spawner thread | Fork on the caller's goroutine | The death signal is bound to the forking thread; a destroyed locked thread would kill the child mid-round (R-1) |
| Cover `Run` as well as `Start` | Only the long-lived children | A harness killed during the DUT `go build` or a `birdc` query leaves that process too; the goal says every process |
| The BIRD gate calls `runStressBird` with the context and root `nativeRunner` hands it | Keep `runStressBirdAction` with a context parameter | `runStressBirdAction` existed only to resolve a root the gate already has; removing it removes the background root |

## Known Limitations
- A harness killed outright still leaves its two namespaces and `/tmp` files: the next run's opening cleanup deletes only names carrying its own suffix. With every process dead those namespaces hold no socket and no host address, and their names carry the dead run's PID, so no later run collides with them. Sweeping them needs a judgment of which suffixes belong to dead runs; that work is `plan/release/validation/spec-stress-sweeps-abandoned-namespaces.md`

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/validation/spec-stress-children-die-with-harness.md` only, in the same `./le commit create` script (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- `stressbird_system.go` (new, 3d82a368bb): `realStressBirdSystem.Start` and `Run` fork through `plugin.KillGroupOnCancel` + `plugin.StartChild` (own group, Pdeathsig SIGKILL, spawner thread); `Run` sets `WaitDelay`; `realStressBirdProcess.Terminate`/`Kill` signal `-pid`, ESRCH read as `os.ErrProcessDone` (`stressGroupSignalResult`)
- `actions.go`/`gates.go`: `stressBirdGate(system)` replaces `runStressBirdGate`, runs `runStressBird` under nativeRunner's signal context and root; `runStressBirdAction` (the `context.Background()` root) deleted
- Closure fix, `stress.go` `stressRunner.startZe`: ze's environment drops `ze.user`/`ze.group` (`env.Without`), tcpdump starts with `-Z root`

### Bugs Found/Fixed
- Closure review: tcpdump (Ubuntu/Debian build, `tcpdump` user created by its postinst) switches user after opening the capture and ze switches user when `ze.user` is inherited; the kernel clears the death signal on an effective uid change, so either outlived a harness killed outright. Covered by `TestStressChildrenKeepTheirCredentials`

### Documentation Updates
- `docs/functional-tests.md`, the stress cleanup paragraph (3d82a368bb), anchors `stressbird_system.go` (Start, Run, Terminate, Kill) and `actions.go` (stressBirdGate, runStressGate); still true after the closure fix
- `./le doc check all`: fails only on `../wiki/command-catalog.md` drift (unrelated, other work); every source anchor resolves (main thread, 3d82a368bb)

### Deviations from Plan
- `realStressBirdSystem` moved to a new `stressbird_system.go` (Files to Create) because `stressbird.go` passed 1000 lines
- `stress.go` and `stress_test.go` added at closure for the credential fix above

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The design read "the kernel clears the death signal on an exec that changes credentials; root execs ordinary binaries" and stopped there | `commit_creds` clears `pdeath_signal` on any effective-uid change, including a `setuid` after exec; Ubuntu/Debian tcpdump drops to its `tcpdump` user, and ze drops to `ze.user` | Closure review, reading the tcpdump argv in `stressRunner.startZe` and `privilege.Drop` | Fixed in `startZe`; the comment above `realStressBirdSystem` and the spec's key insight now state the whole rule |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Every stress child dies when the harness dies | Done | `stressbird_system.go` `Start`, `Run`; `stress.go` `startZe` | death signal kept for tcpdump and ze by the closure fix |
| SIGINT/SIGTERM runs teardown on every gate | Done | `actions.go` `stressBirdGate`, `runStressGate` | cleanup roots on `context.WithoutCancel` (`stressBirdRunner.cleanup`) |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestStressHarnessKilledOutrightTakesItsChildren/start`, `TestStressChildrenKeepTheirCredentials` | |
| AC-2 | Done | `TestStressHarnessKilledOutrightTakesItsChildren/run` | |
| AC-3 | Done | `TestStressStopReachesTheChildsWholeGroup` | |
| AC-4 | Done | `TestStressChildLivesWhileItsHarnessLives` | green before the fix, by design |
| AC-5 | Done | `TestStressRunTimeoutReachesTheWholeGroup` | |
| AC-6 | Done | `TestStressBirdGateRunsUnderTheSignalContext` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestStressHarnessKilledOutrightTakesItsChildren` + `TestStressHarnessDeathStandIn` | Done | `stressbird_death_linux_test.go` | |
| `TestStressStopReachesTheChildsWholeGroup` | Done | same | |
| `TestStressChildLivesWhileItsHarnessLives` | Done | same | |
| `TestStressRunTimeoutReachesTheWholeGroup` | Done | same | |
| `TestStressBirdGateRunsUnderTheSignalContext` | Done | `stressbird_test.go` | |
| `TestStressChildrenKeepTheirCredentials` | Done | `stress_test.go` | green; red under a discrimination break of `startZe` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `stressbird.go`, `actions.go`, `gates.go`, `docs/functional-tests.md` | Done | 3d82a368bb |
| `stressbird_system.go`, `stressbird_death_linux_test.go` | Done | 3d82a368bb |
| `stress.go`, `stress_test.go` | Done | closure fix, commit A |

### Audit Summary
- **Total items:** 16
- **Done:** 16
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Every process the stress harness starts dies when the harness dies, however it dies | unit test over the real `realStressBirdSystem`, real processes | `TestStressHarnessKilledOutrightTakesItsChildren/{start,run}`: red before the fix ("outlived its SIGKILLed harness: still in state S 5s later"), green after; `TestStressStopReachesTheChildsWholeGroup`, `TestStressRunTimeoutReachesTheWholeGroup` red/green on the grandchild; `TestStressChildrenKeepTheirCredentials` for the user-switching children |
| A SIGINT or SIGTERM to the harness runs its teardown on every gate | unit test through the gate entry | `TestStressBirdGateRunsUnderTheSignalContext`: red before ("a canceled gate answered code 0"), green after; cleanup tail asserted |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Sweeping namespaces and `/tmp` files a harness killed outright leaves | needs a judgment of which run suffixes belong to dead runs; outside this spec's ACs | `plan/release/validation/spec-stress-sweeps-abandoned-namespaces.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | recorded by `./le spec review record` (path in the closure report) |
| `./le spec review check` | run after record |
| Rounds | 3 |
| Reviewer lenses used | logic+wiring+removed-behavior (round 1, full diff of 3d82a368bb), security+edge-cases over the fork and signal paths, style pass over every changed Go file; round 2 over the closure fix only |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | tcpdump drops to its own user after opening the capture (Ubuntu/Debian build) and ze drops to an inherited `ze.user`; `commit_creds` then clears the death signal, so either outlives a harness killed outright, and the comment above `realStressBirdSystem` claimed no credential change happens | `stress.go` `stressRunner.startZe`; `stressbird_system.go` type comment | `-Z root` on tcpdump, `env.Without(..., "ze.user", "ze.group")` for ze, comment corrected, `TestStressChildrenKeepTheirCredentials` |

### Fixes applied
- Root cause: `stressRunner.startZe` started tcpdump with no `-Z` and handed ze the harness environment unfiltered; `privilege.Drop` (ze) and tcpdump's `droproot` change the effective uid. [workaround] document the two as exceptions; [source] keep both children's credentials, which keeps the goal true

### Run 2
- Scope: the closure fix. `env.Without` returns a new slice and leaves `r.base.environ` unchanged; `-Z root` is a no-op credential switch on a build with no default user; the test wrapper delegates every method to the recorder. 0 BLOCKER, 0 ISSUE

### Run 3
- Scope: the test-match fix in `TestStressChildrenKeepTheirCredentials` only. The DUT build argv (`go build` with the binary as its `-o` target, `stressRunner.buildZe`) holds no `start` after the binary, the start argv (`ip netns exec`, the binary, optional `--pprof`, then `start` and the config, `stressRunner.startZe`) does, and `sawZe` still requires the start to be seen, so the test cannot pass vacuously. 0 BLOCKER, 0 ISSUE

### NOTEs
- `Terminate`/`Kill` check `done`, then signal `-pid`: in the window after `collect` reaps the child and before `done` closes, the emptied group's pid could in principle be reused by a new group leader. Microsecond window, test tooling, no fix
- `stress.go` is 1009 lines (1003 before this spec); the file-length hook warns

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/test/integration/stressbird_system.go` | Yes | in 3d82a368bb (`git show --stat`) |
| `internal/le/test/integration/stressbird_death_linux_test.go` | Yes | in 3d82a368bb (`git show --stat`) |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-6 | the six tests pass | main thread: `go test -race -count=1 -v -run 'TestStress(HarnessKilled\|StopReaches\|ChildLives\|RunTimeout\|BirdGateRuns)' ./internal/le/test/integration/` all PASS; `./le test unit pkg path ./internal/le/test/integration/` ok (3d82a368bb) |
| AC-1 (credentials) | ze and tcpdump keep their credentials | main thread: `TestStressChildrenKeepTheirCredentials` first went red on a test defect (`ze started with "ze.user=nobody"`: its ze match also caught the DUT `go build -o` argv, which rightly inherits the harness environment); match fixed to the binary followed by `start`. Then `./le test unit pkg path ./internal/le/test/integration/` exit 0 (the credential test and the six death tests included). Discrimination: `startZe` temporarily on `env.Without(r.base.environ, "ze.discrimination-probe")` turned it red (`ze started with "ze.user=nobody"`); restored to `env.Without(r.base.environ, "ze.user", "ze.group")`, green |
| lint | `./le go lint run scope "./internal/le/test/integration/..."` | NOT RUN: aborted four times before planning because other sessions changed its inputs (l2tp, cos, `plugin/process/process.go`); it judged no code |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le test integration stress-bird` | none (N-A: root and a quiet host); `gates.go` `Native: stressBirdGate(realStressBirdSystem{})` | Yes, read at source |
| `./le test integration stress` | none; `realStressSystem` embeds `realStressBirdSystem` (`stress.go`) | Yes, read at source |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | iproute2 `cmd_exec`; AC-1 green with `sleep` forked directly |
| A-2 | confirmed | grep of `stressbird.go`, `stress.go`: no group use |
| A-3 | confirmed | `TestStressHarnessKilledOutrightTakesItsChildren` red then green |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/functional-tests.md`: children get their own group and SIGKILL on harness death; stops reach the group; both gates tear down on SIGINT/SIGTERM | `stressbird_system.go` `Start`, `Run`, `Terminate`, `Kill`; `actions.go` `stressBirdGate`, `runStressGate`; `stressBirdRunner.cleanup` (`context.WithoutCancel`) | Yes |
| No other page | `grep -rln 'source: internal/le/test/integration/stress' docs/`: `functional-tests.md`, plus `perf-round-3.md` (`stressProfileReach`, `buildZe`) and `testing/qemu-integration.md` (`stressConfigDir`, `stageConfig`), whose anchored symbols this spec does not change | Yes |
| `./le test weakened check` | 5 `[DELETED]` findings, all `internal/component/plugin/server/managed_auth_refused_test.go`, another session's file; none in this spec's files | Yes |

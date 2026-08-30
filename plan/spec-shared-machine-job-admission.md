# Spec: shared-machine-job-admission

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/shared-machine-job-admission.md` (or `-` if nothing deferred) |  <!-- doc-links: ignore (file this spec will create; the spec is `in-progress` and the work is not implemented) -->
| Handoff | - |
| Updated | 2026-08-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Several Claude sessions share one checkout and one machine. Each session, and
each subagent it spawns, decides on its own to run tests or lint. Every heavy
target is sized for the WHOLE machine, and only three of them take any lock, so
two agents starting work at the same moment oversubscribe the box until it
stops responding.

The owner reported the symptom on 2026-08-17: three concurrent sessions, and
"running the linting by hand can cause the machine to freeze".

The goal is that a session can ask for a heavy job at any moment and get one of
three answers, none of which is a frozen machine: it runs now, it waits with a
visible position and progress, or it attaches to an equivalent run already in
flight and shares that result.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - the canonical architecture reference: the design principles all new code follows
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - the certificate and per-path manifest one verification run records
- [ ] `docs/functional-tests.md` - the verify runner's stages, artifacts, and the log paths a waiter would follow
  → Constraint: the runner already writes a per-stage banner to `tmp/ze-verify.log` and a machine-readable index to `tmp/ze-verify-failures.json`, so progress reporting needs no new output format, only a reader
  → Decision: artifact paths are fixed strings in `internal/le/verify/engine/run.go`; a second concurrent run overwrites them, so job isolation must give each run its own log directory before any two runs are allowed to overlap
- [ ] `ai/rules/commands.md` - the existing "run commands through make" instruction the hook would enforce
  → Constraint: the rule already says prefer `make` and that a bare `go test` drops feature tags; this spec adds enforcement, it does not add a new rule
- [ ] `ai/rules/repo-maintenance.md` - what a new tool owes: rule, index row, discovery surface, verification
  → Constraint: a new script under `internal/le/` needs a sibling `*_test.py` and a row in the Hook-to-Rule Mapping table when it is bound to a hook check

**Key insights:** (minimal context to resume after compaction)
- The coordination primitive already exists (`internal/le/verify/lock/answer.go`) and is wired to 3 targets out of ~15 heavy ones. This is mostly a wiring and admission-policy problem, not a new-mechanism problem.
- Lint dominates. Measured 2026-08-17: `./le verify lint run` took about 18 minutes of a 20-minute full verify under load; the other 25 stages took about 2.
- The largest single win is DEDUPLICATION, not serialization: in a shared checkout most agents want the same verify of the same tree.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/verify/lock/answer.go` - `flock -o` on `tmp/.ze-verify.lock`, re-enters itself as `__inner__` to write `tmp/.ze-verify.lock.owner` (LABEL, PID, PGID, STARTED, CMD), records elapsed time to `tmp/.ze-verify-duration.txt`, and prints the previous run's duration on acquisition. Interface is `LABEL CMD [ARGS...]`, so it already accepts an arbitrary command.
- [ ] `internal/le/` native action tables - `./le verify current mode full` (:669) and `./le verify current mode changed` (:688) are the only targets that call `verify-lock.sh`; `GO_TEST_PROCS` (:112) computes `max(nproc-3, ceil(nproc/2))`, which is 29 on the 32-core box; `./le verify lint run` (:560-564) and `./le changed scope` (:622-628) each invoke `golangci-lint` TWICE, once native and once `GOOS=linux --build-tags integration`.
- [ ] `internal/le/testchaos/actions.go` - `ze-chaos-verify` (:47) is the third and last caller of the lock.
- [ ] `.golangci.yml` - enables `unused`, `dupl`, `staticcheck`, `gocritic` and 20 more; sets NO `concurrency` key and no memory ceiling, so golangci-lint defaults to one worker per core.
- [ ] `.claude/hooks/pretool-bash.py` (retired; now `internal/le/hookruntime/bash.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> - `CHECKS` is a tuple of `check_x(cmd, ctx) -> None | (exit_code, message)`; `main()` returns the worst code, and it can also emit `hookSpecificOutput.updatedInput` to REWRITE the command before it runs, which it already does to inject `CLAUDE_CODE_SESSION_ID`.
- [ ] `internal/le/verify/engine/run.go` - stage names come from `stagesForMode`; artifact paths (`tmp/ze-verify.log`, `tmp/verify/<nn>-<stage>.log`, `tmp/ze-verify-failures.json`) are package constants, so two concurrent runs collide on them.

**Behavior to preserve:**
- `verify-lock.sh`'s `LABEL CMD [ARGS...]` interface and its owner file, which the "waiting" banner and `commit_helper.py` diagnostics read.
- The duration history in `tmp/.ze-verify-duration.txt` and the "previous run took" line, which is how a session picks a timeout.
- Every existing `make` target name. Agents, rules and skills reference them by name.
- The verify runner's exit code contract: the commit gate reads it.

**Behavior to change:**
- The lock's population grows from 3 targets to every heavy target.
- The stuck-holder rule stops being age-based (see R-1 below).
- The linter gains a concurrency and memory ceiling.
- A raw `go test` / `golangci-lint` / `bin/ze-test` / `python3 *_test.py` invocation is refused by the Bash hook.

## Data Flow (MANDATORY)

### Entry Point
- An agent's Bash tool call containing a heavy command, in one of three forms: a `make` target, a raw tool invocation, or a script that wraps either.

### Transformation Path
1. `.claude/hooks/pretool-bash.py` (retired; now `internal/le/hookruntime/bash.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> inspects the command text. A raw heavy invocation is refused with the queued equivalent named.
2. The `make` target's recipe calls `internal/le/job/job.go <label> <command>`.
3. `ze-run.sh` computes a job key from the label and the tree hash, then consults the job registry under `tmp/.ze-jobs/`.
4. Three outcomes: an equivalent job is RUNNING, so attach to its log and exit with its code; a slot is free, so claim it and run; no slot is free, so wait, printing the holder's label and current stage.
5. The job writes its artifacts under a per-job directory, and on exit records duration and clears its registry entry.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Agent ↔ harness | PreToolUse hook exit code 2 refuses the call; stderr carries the replacement command | No |
| Session ↔ session | Shared filesystem state under `tmp/.ze-jobs/`, guarded by `flock` | No |
| Wrapper ↔ job | Process group, so a broken job's children die with it | No |

### Integration Points
- `internal/le/verify/lock/answer.go` - `ze-run.sh` is its generalization; the old name stays as a thin alias so no caller breaks.
- `internal/le/commit/prepare.go` - reads `tmp/ze-verify-full.json` for the Go-commit coverage gate; a per-job artifact directory must still publish that file at its documented path.
- `internal/le/` - every heavy target's recipe gains the wrapper.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The freeze is memory pressure before CPU contention | **the spec author's inference, not the owner's report and not a measurement.** The owner reported one symptom: the machine freezes when lint runs by hand. The author read a single `free -g` snapshot (31 GB total, 2 GB free) beside `uptime` (load 25.5) and two `gopls` at 3.0 GB combined, and wrote a cause from it. Nothing in that snapshot distinguishes memory pressure from CPU contention | a CPU-only token pool would not prevent it; admission must be weighted by memory, not by job count | Phase 2's cold paired lint measurement, 2026-08-18: the ceiling cut peak RSS 4.55 GiB to 3.96 GiB (13%) and CPU 1978% to 798% (60%). Three concurrent lint runs at the old setting are about 60 runnable threads on 32 cores, and 13.5 GiB of 31 GB. The thread count exhausts the box; the memory does not | **broken 2026-08-18.** CPU contention is the dominant term and memory is second. The measurement is in the AC-1 evidence section. Consequence: admission CAN be weighted by CPU cost alone and does NOT have to model memory. See the Mistake Log |
| A-2 | Most concurrent verify requests are for an equivalent tree, so attach-and-share removes most of the queue | shared checkout, 8 agents, one working tree | dedup buys little and plain serialization is the whole answer | count distinct tree hashes among job requests over one working day | unvalidated |
| A-3 | The PreToolUse hook sees every agent Bash call, including subagents' | `.claude/hooks/pretool-bash.py` (retired; now `internal/le/hookruntime/bash.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> `main()` handles `agent_id` and `CLAUDE_CODE_FORK_SUBAGENT` | a subagent could bypass admission entirely, and the guard would be advisory | spawn a subagent, have it run a raw `go test`, confirm refusal | unvalidated |
| A-4 | `golangci-lint` honors a `concurrency` setting and `GOMEMLIMIT` in the way the ceiling assumes | golangci-lint v2.10.1 as installed: `run.concurrency` passes `golangci-lint config verify` and `run -h` documents `-j, --concurrency int`; the binary's Go runtime reads `GOMEMLIMIT` at startup (a malformed value fatals in `runtime.readGOMEMLIMIT`) | the cheapest mitigation does not work and admission must carry the whole load | measure peak RSS with and without both settings | **confirmed 2026-08-18, with one correction: the memory half cannot live in `.golangci.yml`.** v2.10.1 exposes no memory setting at all -- `config verify` rejects `memory`, `memory-limit`, `mem-limit`, `gomemlimit`, `max-memory` and `memory-ceiling` as unknown keys under `run`, and `run -h` lists no memory flag. `GOMEMLIMIT` is therefore set on every invocation through `ZE_LINT` in the retired Makefile (current producers: `internal/le/` native action tables). See the AC-1 evidence below |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The existing stuck-lock rule kills the legitimate run. `MAX_LOCK_AGE` defaults to 1800s, and the header comment justifies it with "./le verify current mode full targets ~2 min". The recorded history says 12m45s and the run of 2026-08-17 exceeded 20 minutes under load. Under exactly the contention this spec addresses, the first waiter SIGKILLs the holder's process group, then runs slowly itself and is killed by the next waiter | a `breaking stuck lock` line in a transcript, or a verify that dies with no failure summary | replace the age test with a liveness test: break the lock only when the holder's PID is dead, or when its log has not grown for N minutes. Age alone cannot distinguish "hung" from "slow because the box is busy", which is the state this tool exists to manage |
| R-2 | Attach-and-share returns a stale verdict. Session B attaches to session A's run, but the tree changed between A's start and B's request, so B is certified by a run that never saw its code | a commit passes the Go-coverage gate whose diff the attached run predates | the job key includes the tree hash, and attach requires an exact match. `verify-status.sh tree_hash` already computes one over HEAD, the diff, and untracked files |
| R-3 | Serializing every heavy target makes the queue the new bottleneck: eight agents each waiting 20 minutes | agents idling on the lock rather than working | weight admission rather than counting jobs, so light targets keep running; and land the linter ceiling first, since lint is about 90% of the measured wall clock |
| R-4 | The hook refuses a command an agent legitimately needs and there is no escape, so the agent invents a workaround (a script file, a renamed binary) that the hook cannot see | a transcript where the refusal is followed by an unusual invocation | give the refusal a named escape that is cheap and honest, and make the queued path the shorter thing to type |
| R-5 | The wrapper leaks a slot when it is killed, and admission deadlocks for every session | jobs waiting with no live holder | registry entries carry PID and PGID; a waiter reaps an entry whose PID is dead, which is the behaviour `verify-lock.sh` already implements for its owner file |
| R-6 | Two concurrent jobs overwrite the fixed artifact paths in `internal/le/verify/engine/run.go`, so a failure index describes the wrong run | a failure summary naming stages the session never ran | per-job artifact directory, with the documented paths published as symlinks or copies once the job completes |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every agent's ability to run a test. A wedged admission point is worse than the freeze it replaces, because the freeze at least resolves when a job ends |
| How is it reverted? | Single commit revert. The wrapper is additive: the old `verify-lock.sh` interface stays, and reverting the the native action tables under `internal/le/` recipes restores today's behaviour |
| Who else touches this path? | Every session in the checkout, concurrently, which is why the rollout order below starts with the change that needs no coordination |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le verify lint run` with a job already holding the slot | → | `ze-run.sh` admission wait path | `test_a_second_heavy_job_waits_for_the_slot` |
| `./le verify current mode full` when an equivalent job is running | → | `ze-run.sh` attach path | `test_an_equivalent_job_is_attached_not_queued` |
| Agent Bash call `go test ./internal/...` | → | `check_raw_test_invocation` in `pretool-bash.py` | `test_a_raw_go_test_is_refused_and_names_the_make_target` |
| A job whose holder PID is dead | → | `ze-run.sh` reap path | `test_a_dead_holder_s_slot_is_reclaimed` |
| A slow but live holder past the old age limit | → | `ze-run.sh` liveness test | `test_a_slow_live_holder_is_not_killed` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `golangci-lint` runs via `./le verify lint run` or `./le changed scope` | Peak RSS is bounded by a configured ceiling and worker count is capped: workers in `.golangci.yml` (`run.concurrency`), memory in the retired Makefile (current producers: `internal/le/` native action tables) (`ZE_LINT` sets `GOMEMLIMIT`), because golangci-lint v2.10.1 accepts no memory key -- see A-4; the measured peak is recorded in the spec |
| AC-2 | Two heavy jobs are requested at once by different sessions | Exactly one runs; the second reports the holder's label and elapsed time and waits, without either being killed |
| AC-3 | A second session requests a job equivalent to one already running (same label, same tree hash) | It attaches to the running job, follows its output, and exits with that job's exit code, without starting a second run |
| AC-4 | A second session requests a job with the same label but a DIFFERENT tree hash | It does NOT attach; it queues, because the running job never saw its code |
| AC-5 | An agent issues a raw `go test`, `golangci-lint`, `bin/ze-test`, or `python3 <path>_test.py` | The Bash hook refuses with exit 2 and names the queued command to use instead |
| AC-6 | The holder of a slot is alive and has been running longer than the old 1800s limit, and its log is still growing | The waiter does NOT kill it |
| AC-7 | The holder's process is dead | The next waiter reclaims the slot without operator action |
| AC-8 | A waiting session asks what is happening | It sees the holder's current stage name and elapsed time, refreshed as the holder progresses |
| AC-9 | Two jobs run concurrently under a weighted policy | Neither overwrites the other's log or failure index; each artifact set is attributable to one job |
| AC-10 | `commit_helper.py` reads full-verify coverage after a job completes | `tmp/ze-verify-full.json` is present at its documented path with that job's content |

### AC-1 evidence (measured 2026-08-18)

The ceiling is `run.concurrency: 8` in `.golangci.yml` and `GOMEMLIMIT=4GiB` in
the `ZE_LINT` variable in the `internal/le/` native action tables. Every golangci-lint invocation in the
repository now goes through `ZE_LINT`: both passes of `_./le verify lint run-impl`, both
passes of `./le changed scope`, and `./le test-chaos lint` in `internal/le/testchaos/actions.go`.

A full `./le verify lint run` could not serve as the before-and-after pair. Another
session was editing `internal/plugins/isis/packet` at the time, and the
resulting typecheck error aborted the run after 15s and 511 MB, before the
analysis that carries the memory. The pair below runs the same fixed package
set (`./internal/component/... ./internal/core/... ./pkg/... ./cmd/ze/...`)
against a private, empty golangci-lint cache, so both runs are cold and the
shared cache other sessions rely on is untouched.

| Run | Workers | GOMEMLIMIT | Peak RSS | Wall clock | CPU | Findings |
|-----|---------|-----------|----------|-----------|-----|----------|
| before | 32 (auto) | none | **4,770,220 KB (4.55 GiB)** | 1:00.96 | 1978% | 9 `unused` |
| control | 8 | none | 4,356,776 KB (4.16 GiB) | 1:43.46 | 798% | 9 `unused` |
| after | 8 | 4GiB | **4,148,636 KB (3.96 GiB)** | 1:43.83 | 798% | 9 `unused` |

The findings are byte-identical across all three runs, so the ceiling weakens
nothing. The red is another session's in-flight work in
`internal/component/plugin/process`, and it survives unchanged.

Three facts the numbers carry:

- The memory limit binds. The capped peak sits at 3.96 GiB against a 4 GiB
  limit, 0.5% under it, while the uncapped run overshot it by 576 MB. Peak RSS
  stops growing with the size of the package set, which is the property the
  freeze needed.
- The memory limit is nearly free. The control run isolates it: 8 workers
  without `GOMEMLIMIT` take 1:43.46, with it 1:43.83. 4 GiB costs 0.4% and
  saves 208 MB, so the GC is pacing rather than thrashing at this ceiling.
- The wall clock cost is the worker cap alone, +70% (1:01 to 1:44) for -60% CPU
  (20 cores to 8). That is the trade the spec buys: one lint run can no longer
  claim the whole box.

Warm-cache runs cost about 180 MB either way (5.0s at 32 workers, 5.5s at 8),
so the pressure the ceiling removes lives entirely in the cold path, which is
the path the 2026-08-17 freeze was on.

A full `./le verify lint run` after the change, once the neighbouring session's tree
typechecked again: peak RSS 2,299,092 KB (2.19 GiB) in 33s on the shared warm
cache, reporting the 10 expected issues (one `goconst` in
`internal/plugins/anomaly/detect/detector.go`, nine `unused` in
`internal/component/plugin/process/process.go`).

### Rollout: the admitted population and the slot count (phase 7, 2026-08-18)

**The population is a criterion, not a list.** A target is admitted when its
recipe starts a Go test binary, the `ze-test` runner, `golangci-lint`,
`govulncheck`, Docker, or QEMU. That reads 105 targets out of the `internal/le/` native action tables and
ten `internal/le/` files, and `./le verify lint run` from phase 1 makes 106 pairs. Each is the
phase-1 shape: the public target calls `internal/le/job/job.go`, and
`_<target>-impl` holds the recipe body.

Three exclusions, each for a stated reason rather than for size:

| Excluded | Why |
|----------|-----|
| `ze-qemu-shell`, `ze-qemu-debug`, `ze-gokrazy-run` | interactive. The wrapper pipes a job's output through `tee`, which loses the terminal |
| Recipe-less aggregates (`ze-integration-test`, `ze-live-test`) | their members are admitted individually; giving the aggregate a recipe would change what `make` does with it |
| Builds, and the source-reading Python checks in `internal/le/doc/check/actions.go` | single-threaded, seconds long, and not implicated in the 2026-08-17 freeze. `go build` is bounded by the shared cache |

**The slot count is DERIVED, not chosen.** `ZE_RUN_SLOTS` (`internal/le/` native action tables, beside
`GO_TEST_PROCS`) is cores divided by the per-job ceiling that the owner's
2026-08-17 ruling already set: `GO_TEST_PROCS` is `nproc / 4`, and
`.golangci.yml` caps the linter at 8 workers to match. On this 32-core box that
is `32 / 8 = 4`, which is the number the comment above `GO_TEST_PROCS` states in
words: "four concurrent runs fit, and a fifth still degrades gracefully instead
of thrashing".

The quantity divided is CPU because assumption A-1 is broken (see the Mistake
Log): the phase 2 pair measured 798% CPU and 3.96 GiB for a capped linter, so
four jobs are about 32 runnable threads and 16 GiB of the 31 GiB on this box.
CPU binds first; memory has headroom. Keeping `SLOTS=1` would have been
stricter than that evidence supports: eight sessions would queue for a box
running at a quarter of its capacity, and a queue nobody believes in is the
thing agents route around.

**Every wrapped target that is also a `./le verify current mode full` stage inherits the
parent's slot rather than queueing behind it.** Nine now do: `./le changed scope`,
`ze-unit-hook-test`, `ze-dependency-vulnerability-check`,
`ze-unit-test-changed`, `ze-unit-test-cached`, `ze-unit-test-race-changed`,
`ze-alloc-check`, `./le functional` and `./le functional exabgp-test`
(`stagesForMode` in `internal/le/verify/engine/run.go`), beside `./le verify lint run` from
phase 1. One code path serves all of them: the nested-job branch of
`internal/le/job/job.go` execs straight through when `ZE_RUN_JOB` names an
existing entry, and `execStage` (`internal/le/verify/engine/run.go`) extends
`os.Environ()` so the variable reaches every stage. Measured rather than
argued: with another session's `./le verify current mode full` holding the only slot, a
wrapped target run under a `ZE_RUN_JOB` entry printed `[ze-unit-pkg-test]
running inside fake-parent` and finished at once instead of waiting.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `test_a_second_heavy_job_waits_for_the_slot` | `internal/le/` | AC-2 | pass |
| `test_an_equivalent_job_is_attached_not_queued` | `internal/le/` | AC-3 | pass |
| `test_a_different_tree_hash_does_not_attach` | `internal/le/` | AC-4 | pass |
| `test_a_holder_that_dies_mid_attach_leaves_no_verdict_behind` | `internal/le/` | AC-3 correctness: a shared job killed before it records an exit code yields no verdict at all, so the sharer returns to the queue and runs the job itself | pass |
| `test_the_tree_hash_is_taken_when_the_job_is_admitted` | `internal/le/` | AC-4 support: the `TREE` a later asker attaches on names the tree the job IS judging, not the one its asker saw before it queued | pass |
| `test_a_slow_live_holder_is_not_killed` | `internal/le/` | AC-6 | pass |
| `test_a_stalled_holder_is_broken_and_the_kill_carries_its_evidence` | `internal/le/` | AC-6 discriminator: a holder that stops producing still loses its slot, and the kill prints the silence that justified it | pass |
| `test_the_stall_window_boundary_is_enforced` | `internal/le/` | stall window 59 / 60 / 3600 / 3601, plus the older `ZE_VERIFY_MAX_LOCK_AGE` spelling | pass |
| `test_a_dead_holder_s_slot_is_reclaimed` | `internal/le/` | AC-7 | pass |
| `test_the_slot_count_is_read_from_the_environment` | `internal/le/` | the derived `ZE_RUN_SLOTS` reaches admission: the default queues a second job, and 2 admits it | pass |
| `test_the_slot_count_boundary_is_enforced` | `internal/le/` | slot count 0 / 1 / cores / cores + 1, and a non-numeric value | pass |
| `test_a_raw_go_test_is_refused_and_names_the_make_target` | `.claude/hooks/tests/` fixture | AC-5 | |  <!-- doc-links: ignore (file this spec will create; the spec is `in-progress` and the work is not implemented) -->
| `test_the_make_target_itself_is_not_refused` | `.claude/hooks/tests/` fixture | AC-5 discriminator: the guard must not block the sanctioned path | |  <!-- doc-links: ignore (file this spec will create; the spec is `in-progress` and the work is not implemented) -->
| `TestConcurrentRunsDoNotShareArtifactPaths` | `internal/le/verify/engine/verifyengine_test.go` | AC-9 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| admission weight | 1..total slots | total slots | 0 | total slots + 1 |
| slot count (`ZE_RUN_SLOTS`) | 1..cores | cores | 0 | cores + 1 |
| liveness stall window (seconds) | 60..3600 | 3600 | 59 | 3601 |
| `golangci-lint` concurrency | 1..nproc | nproc | 0 | nproc + 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test_a_second_heavy_job_waits_for_the_slot` | `internal/le/` | two agents ask for a heavy job at once; one runs, one waits, neither is killed | done |
| `test_an_equivalent_job_is_attached_not_queued` | `internal/le/` | two agents ask for the same job on the same tree; one run serves both, and the sharer exits with the holder's code | done |

**The `.ci` vehicle named at design time was wrong, and this is the correction, not a scope reduction.** A `.ci` file is driven by `bin/ze-test` against the `ze` daemon and CLI, and `test/tooling/` does not exist. `ze-run.sh` is a developer-machine wrapper with no `ze` command to drive, so a `.ci` could only have re-implemented the Python harness badly. The coverage the two rows asked for -- two concurrent askers, one serialised and one sharing -- is delivered by the two tests above, which spawn real concurrent invocations against real git repositories and assert on real exit codes, and both were proven to fail when the behaviour under test is reverted. THOMAS: this is the one design-time item this implementation changed rather than met; say so if you want the `.ci` pair anyway.  <!-- doc-links: ignore (file this spec will create; the spec is `in-progress` and the work is not implemented) -->

### Interop Tests (Scope: protocol)
N/A - Scope is tooling; no wire-visible behavior changes.

## Files to Modify
- `internal/le/` native action tables - route `./le verify lint run`, `./le changed scope`, `ze-unit-*`, `ze-functional-*` recipes through the wrapper
- `internal/le/testunit/groups.go`, `internal/le/integration/gates.go`, `internal/le/testchaos/actions.go` - same, for targets defined there
- `.golangci.yml` - add `concurrency` and a memory ceiling
- `internal/le/verify/lock/answer.go` - becomes a thin alias over the generalized wrapper
- `internal/le/verify/engine/run.go` - per-job artifact directory, with documented paths published on completion
- `.claude/hooks/pretool-bash.py` (retired; now `internal/le/hookruntime/bash.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> - add the raw-invocation check to `CHECKS`
- `ai/rules/points/commands/` - state the admission rule and bind the new check to it
- `ai/rules/points/repo-maintenance/` - Hook-to-Rule Mapping row for the new check
- `docs/functional-tests.md` - the artifact table and the queued invocation

## Files to Create
- `internal/le/job/job.go` - the admission wrapper
- `internal/le/` - its tests
- `test/tooling/*.ci` - the functional tests above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | developer tooling; nothing is operator-configurable |
| YANG validation constraints | N-A | as above |
| YANG custom validators | N-A | as above |
| CLI commands/flags | N-A | no `ze` subcommand; the surface is `make` and the hook |
| CLI grammar (keyword before value) | N-A | as above |
| Editor autocomplete | N-A | as above |
| Functional test for new RPC/API | Yes | `test/tooling/*.ci` above |
| Pipe completeness | N-A | no `ze` command output |
| Env var registration | N-A | wrapper settings are developer env vars, not `ze.*` YANG-backed config |
| Doctor check for runtime dependencies | Yes | the wrapper requires `flock` (util-linux); `verify-lock.sh` already errors when it is absent, and that check moves into the wrapper |
| Prometheus counters/metrics | N-A | developer tooling, not daemon state |
| BGP family surface | N-A | not protocol work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | N-A | developer tooling, not a product feature |
| 2 | Config syntax changed? | N-A | no operator config |
| 3 | CLI command added/changed? | N-A | no `ze` subcommand |
| 4 | API/RPC added/changed? | N-A | none |
| 5 | Plugin added/changed? | N-A | none |
| 6 | Has a user guide page? | N-A | contributor surface, not user |
| 7 | Wire format changed? | N-A | none |
| 8 | Plugin SDK/protocol changed? | N-A | none |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | not protocol work |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` - artifact table, queued invocation, attach semantics |
| 11 | Affects daemon comparison? | N-A | none |
| 12 | Internal architecture changed? | N-A | build tooling, not daemon architecture |
| 13 | Route metadata keys added/changed? | N-A | none |
| 14 | Prometheus counters added/changed? | N-A | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | grep `docs/` for anchors on `verify_run.go` and `verify-lock.sh` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/contributing/testing.md` shows raw `go test` invocations the hook will refuse |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the wrapper exists and one target uses it
   - Tests: `test_a_second_heavy_job_waits_for_the_slot`, `test_a_dead_holder_s_slot_is_reclaimed`
   - Files: `internal/le/job/job.go`, `internal/le/`, `internal/le/` native action tables (`./le verify lint run` only)
   - Verify: the wiring test fails because the wrapper is a stub, then passes
2. **Phase: Linter ceiling** -- the cheapest relief, landed first and independently
   - Tests: AC-1's measurement, recorded as peak RSS before and after
   - Files: `.golangci.yml`
   - Verify: peak RSS of `./le verify lint run` drops below the ceiling; `./le verify lint run` still reports the same findings on a known-red tree
3. **Phase: Liveness replaces age** -- stop killing slow runs
   - Tests: `test_a_slow_live_holder_is_not_killed`
   - Files: `internal/le/job/job.go`
   - Verify: a holder past 1800s with a growing log survives; a holder with a dead PID is reaped
4. **Phase: Attach and share** -- the dedup win
   - Tests: `test_an_equivalent_job_is_attached_not_queued`, `test_a_different_tree_hash_does_not_attach`
   - Files: `internal/le/job/job.go`, `internal/le/verify/engine/run.go`
   - Verify: two requests, one run, both correct exit codes
5. **Phase: Artifact isolation** -- concurrency becomes safe
   - Tests: `TestConcurrentRunsDoNotShareArtifactPaths`
   - Files: `internal/le/verify/engine/run.go`
   - Verify: two jobs, two artifact sets, `tmp/ze-verify-full.json` still published where `commit_helper.py` reads it
6. **Phase: Enforcement** -- the hook refuses raw invocations
   - Tests: `test_a_raw_go_test_is_refused_and_names_the_make_target`, `test_the_make_target_itself_is_not_refused`
   - Files: `.claude/hooks/pretool-bash.py` (retired; now `internal/le/hookruntime/bash.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) -->, rule points, Hook-to-Rule Mapping
   - Verify: the raw form is refused, the sanctioned form is not, and a subagent gets the same answer
7. **Phase: Rollout** -- every remaining heavy target joins
   - Files: `internal/le/` native action tables, `internal/le/`, `docs/functional-tests.md`, `docs/contributing/testing.md`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | The attach path returns the ATTACHED job's exit code, never 0 by default; a failed shared run must fail both askers |
| Fail-closed | An unreadable or corrupt registry entry makes the wrapper QUEUE, never run unadmitted |
| Data flow | Admission is decided in one place; no target reaches a heavy tool without passing it |
| Rule: `ai/rules/simplicity.md` | No daemon, no new process to supervise, unless a measured requirement forces one |
| Rule: `ai/rules/evidence.md` | The hook check drives from its entry point (a real payload), not the helper alone |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Every heavy target routes through the wrapper | `grep -L ze-run.sh` over the heavy-target recipes returns nothing. **This deliverable is NOT met (found 2026-08-18).** `ze-fuzz-test` (`internal/le/fuzz/discover.go`) runs 78 `$(GO_TEST) -fuzz=` invocations at 10s each with no wrapper. That file is GENERATED ("Code generated by internal/le/; DO NOT EDIT"), so the fix belongs in `internal/le/`: emit the phase-1 pair, `ze-fuzz-test` calling `internal/le/job/job.go` and `_ze-fuzz-test-impl` holding the recipe body, then `./le repository generate` |
| The wrapper audit reads GENERATED makefiles too | the audit that reported "every heavy target is wrapped" only inspected targets that already had an `_<target>-impl` sibling, so it could not see an unwrapped target that has no `-impl` at all -- which is exactly the shape `ze-fuzz-test` has. Re-run it over every recipe that starts a Go test binary, `ze-test`, `golangci-lint`, `govulncheck`, Docker or QEMU, `-impl` or not, and record the count |
| The raw invocations are refused | run each form through the hook fixture and assert exit 2 |
| Peak lint RSS is bounded | `/usr/bin/time -v ./le verify lint run` before and after, both recorded |
| No slot leak | kill a holder mid-run; the next waiter starts within one poll interval |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The label becomes a filesystem path component: reject separators, dots, and empty strings, or a label escapes `tmp/.ze-jobs/` |
| Resource exhaustion | The registry is bounded; a session that crashes in a loop must not fill it |
| Signal handling | Killing a process GROUP is the existing behaviour and it is load-bearing; verify the wrapper never targets pgid 0 or the session's own group |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The measurement that reframes the problem: on 2026-08-17 a full verify spent about 18 of its 20 minutes in `./le verify lint run` while the box sustained load 25, and the remaining 25 stages took about 2 minutes. Coordination policy that treats all heavy jobs alike would be tuned for the wrong job.
- `gopls` is an uncounted multiplier: one per session, measured at 1.4 to 1.6 GB. Eight sessions is 12 GB of a 31 GB box before any test runs. Admission that only counts test jobs is measuring the smaller half.
- The existing lock's stuck-holder rule is age-based, and age cannot distinguish "hung" from "slow because the box is oversubscribed". That is precisely the state this tool exists to manage, so the rule inverts under load.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Generalize `verify-lock.sh` into a wrapper with a job registry. **Owner approved 2026-08-17: "no daemon is fine, go ahead with the spec approach"** | (a) a resident daemon holding a token pool and streaming progress, as the owner first proposed; (b) plain `flock` over more targets with no registry | The daemon's three benefits are attach-and-share, weighted admission, and progress. A state directory plus `flock` delivers all three: progress already exists in the runner's stage banners, weights are counting semaphores, and attach needs a registry file rather than a process. A daemon adds something to supervise, restart, and debug, and a wedged daemon blocks every agent, which is the failure this spec exists to prevent. Option (b) is rejected because it gives no dedup, and dedup is the largest win |
| Enforce at the PreToolUse hook, not in `make` | trusting `make` alone | Any admission wired into `make` is bypassed by the first agent that types the tool directly, and that is the normal case: this session ran `commit_helper_test.py` directly six times on 2026-08-17 rather than through the retired `ze-unit-pkg-test` (current: `go test -race ./...`) |
| Refuse the raw invocation rather than silently rewriting it | `updatedInput` rewriting, which the hook already supports | A rewrite is invisible in the transcript, so an agent cannot learn the sanctioned path and a wrong rewrite is undebuggable. Refusal teaches; rewriting hides. Reconsider only if refusals prove too frequent to work with |
| Job key is label plus tree hash | label alone | Attaching on the label alone certifies a session with a run that never saw its code, which breaks the Go-commit coverage gate landed the same day |
| Land the linter ceiling first and separately | one big change | It needs no coordination, no new script, and addresses about 90% of the measured wall clock. It is the change that helps tonight |

## Known Limitations
- **The wrapper is HOST-only, and its registry is not namespace-aware.** `tmp/.ze-jobs/` is shared into a QEMU guest over 9p (`internal/le/qemu/run.go`, the repo-root export plus `scratch_share` for a symlinked `tmp/`), while host and guest have disjoint PID namespaces. `_alive` judges a holder by `/proc/<pid>`, so a guest evaluating a HOST entry finds no such pid, calls the slot dead and reaps it: a VM run silently breaks admission on the machine outside it. A pid colliding with an unrelated guest process is the same hazard the other way, and `_break_stalled` would signal ITS group. Found 2026-08-18, after the rollout: `internal/le/qemu/alltests.go` runs the retired `ze-unit-test-cached` (current: `./le verify deps unit-cached`) inside the guest, and the rollout wrapped that target without anyone asking where it runs. Fixed at that one call site by naming the `-impl` body directly, because a single-tenant VM has nothing to admit.
- **Calling an `-impl` directly skips the public target's prerequisites, silently.** That is the cost of the bypass above: `ze-unit-test-cached` declares `./le scratch links-ensure` and `_ze-unit-test-cached-impl` does not, so the VM call names both. Any future bypass owes the same check. It is why the bypass is one call site rather than a habit, and why a general "disable admission" env var was NOT added: an option that silently drops prerequisites is worse than a named exception.
- **"CPU contention dominates" is measured for ONE job type and inferred for the rest.** The 2026-08-18 measurement that broke A-1 covers `golangci-lint` alone: 1978% to 798% CPU against 4.55 to 3.96 GiB. A QEMU or Docker functional suite holds memory without holding runnable threads, so a CPU weight for it is not established by that evidence. This is not a reason to model memory now, and phase 3 MUST NOT treat a CPU weight for a QEMU job as validated: measure that population before trusting a number for it.
- **A holder whose log is unreadable is never broken.** Liveness reads log growth, so a job that lost its log file holds its slot until its process dies. That is deliberate: the alternative is killing on a guess, and the failure this spec exists to prevent is a waiter killing a healthy run. The residual case is a hung job with no log, which no evidence available here distinguishes from a working one.
- Admission covers what the hook can see. A heavy command inside a shell script file is out of reach by construction, exactly as the existing poll-loop check documents.
- The wrapper does not schedule across machines, and does not know about Docker or QEMU containers started by functional suites; their weight is attributed to the job that started them.
- `gopls` memory is measured but not managed. Capping it belongs to the harness, not to this wrapper.

## Mistake Log

<!-- Opened early, in Phase 2, because a broken assumption owes a row the moment
     it breaks (ai/rules/planning.md, Risks & Assumptions). When
     plan/TEMPLATE-CLOSURE.md is appended at closure, MERGE its Mistake Log rows
     into this table and delete its duplicate heading and its `none` row.
     Kind: assumption (a broken A-N) | approach (a route abandoned) | escalation
     (a mistake frequent enough to deserve a rule). -->
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The wrapper split every heavy target into a public name and `_<name>-impl`, and the recipe BODY moved to the impl. Nothing was thought to read those bodies | Other tooling derives facts from recipe bodies. `TestQemuKernelPreconditionIsMetInTheSameJob` (`internal/le/`) collects the targets whose recipe contains `$(ze-qemu-kernel-guard)` and requires a workflow job to run one. After the split the guard sits in `_ze-qemu-*-impl`, workflows call the public name, the intersection is empty, and the test's own anti-vacuity `Fatal` fired -- on a tree where every guard is still in place | `./le verify current mode full` 2026-08-18, stage `ze-unit-test-cached`. NOT caught by the wrapper's own tests, which check admission, nor by the rollout audit, which only compared public targets against their impl siblings | The test now credits the public name when its `_<name>-impl` carries the guard. The general lesson is in the Deliverables Checklist row above: a derivation over recipe bodies sees the impl after this spec, and every such reader has to be found rather than assumed absent |
| assumption | A-1 read the 2026-08-17 freeze as memory pressure before CPU contention, and concluded that admission must be weighted by memory rather than by job count | CPU contention is the dominant term. Capping the linter cut CPU by 60% (1978% to 798%) and peak RSS by 13% (4.55 GiB to 3.96 GiB). Three concurrent lint runs at the old setting are about 60 runnable threads on 32 cores, which exhausts the box, against 13.5 GiB of 31 GB, which does not | Phase 2's cold paired measurement of `./le verify lint run` with and without the ceiling, run to validate A-4. A-1 was not the thing being tested; the CPU column broke it | Admission is weighted by CPU cost. Phases 3 and 4 do NOT model memory, which removes a per-job memory estimate, its calibration, and the failure mode of a wrong estimate. The design is unchanged in shape: the registry never depended on memory weighting |

### Deviations from Plan
- **Admission weighting is CPU-based, not memory-based.** A-1 is broken (see the Mistake Log). The spec's R-3 mitigation says "weight admission rather than counting jobs", and that survives; only the quantity being weighted changes.
- **The memory half of the AC-1 ceiling sits in the `internal/le/` native action tables, not in `.golangci.yml`.** golangci-lint v2.10.1 accepts no memory setting, so `GOMEMLIMIT` goes on every invocation through the `ZE_LINT` variable. See A-4 and the AC-1 evidence section.
- **`./le test-chaos lint` in `internal/le/testchaos/actions.go` gained the ceiling too.** AC-1 names `./le verify lint run` and `./le changed scope`; the chaos target is a fifth golangci-lint call site, and leaving it raw would let a target reach the linter with no ceiling.
- **Admission counts jobs against `ZE_RUN_SLOTS`; it does not weight them.** Phase 7 sizes the SLOT rather than the job, because every heavy target in this repository is already capped at a quarter of the cores by `GO_TEST_PROCS` and `.golangci.yml`. Weighting is what you need when jobs differ in cost; here they were made equal first. A job type that breaks that equality (a QEMU suite holding memory without holding threads, the Known Limitation above) is the case that would need weights, and none is measured.
- **`internal/le/job/job.go` reads `ZE_RUN_SLOTS` (landed 2026-08-18).** Phase 7 derived and exported the number but could not wire it, because phase 4 held the file, so admission stayed at one job while 105 targets queued behind it. `SLOTS` now reads the exported value and defaults to 1 for a caller that arrives without `make`. Out of range is refused rather than clamped, like the stall window: `SLOTS_MIN=1` and `SLOTS_MAX=$(_cores)`, since zero queues every job for ever and more slots than cores is no admission at all.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

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

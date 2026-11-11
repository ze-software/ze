# Spec: test-children-die-with-runner

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | A leaked test peer holds its functional-test port for days, so later functional runs fail on "address already in use" and redden the evidence the release depends on |
| RFC gaps | - |
| Depends | - |
| Phase | 1/3 |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Symptom.** On 2026-11-11 the host carried 59 `le test peer` processes, parent
PID 1, idle for 8.7 and 9.8 days, about 83 MB each (about 5 GB), each still
bound to a 25xxx functional-test port. They came from two functional runs of
session 384a3761 (one from `bin/le-s-384a3761/le`, one from a scratch copy
under that session's `wt40/`). 28 exited on SIGTERM; 31 needed SIGKILL, 9 of
them in uninterruptible sleep.

**Cause, read at the producers.**

1. The `.ci` runner starts every long-lived child (the peer, the ze client, each
   multi-process command) through `startWithETXTBSYRetry`, which gives it a
   process group of its own (`boundProcessDeadline`) and no parent-death
   signal. Every stop of those children is Go code in the runner: the deferred
   `ownedProcess.finish` and the context cancel. A runner that dies without
   running Go code (SIGKILL, the OOM killer, a torn-down session) stops none
   of them, and they are reparented to init.
2. le stops an interrupted suite by SIGKILLing the suite's process group
   (`gaterun.StreamContext` Cancel, `functional.Execute`). Its comment assumes
   "every process the suite starts stays in it unless it moves itself out",
   but the runner moves every child out (point 1). So the documented operator
   interrupt path is itself one of the routes that orphans every peer.
3. `le test peer` has no lifetime of its own. `option=timeout` is read by the
   runner only (reproduced: a peer declared `timeout=6s` was still waiting for
   its connection after 20 s), and an `option=linger` peer is designed to run
   until it is killed. An orphan therefore runs forever.

**Page disagrees with code.** `docs/architecture/testing/runner-architecture.md`
states that a suite owes three routes, the third being "`Pdeathsig` on Linux:
the runner dying outright, which no code in the runner can cover". The `.ci`
owner has never set it. `StartServerProcess` in the same package does.

**Goal.** A process the `.ci` runner starts dies when the runner dies, however
the runner dies, on Linux. le's suite interrupt then ends the tree it means to
end.

**Split from.** `plan/post-release/engineering/spec-test-sandbox.md` AC-5 ("a run
ends, killed or not: no process it started survives, and no address it added
stays on loopback"). This spec takes the process half for the `.ci` runner; the
loopback-address half stays there.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md`, `ai/doors/plugin-infra.md` - the doors this work crosses
  → Constraint: `internal/test/runner` may import `internal/component/plugin` (it already does, `server_process.go`); nothing under `internal/le` may be imported from product code, so the shared fork stays in `internal/component/plugin`
  → Constraint: the runner primitives are judged by `./le test functional runner` and `./le test unit pkg path ./internal/test/runner/`
- [ ] `docs/architecture/testing/runner-architecture.md` - child process lifetime, "Three things end a process"
  → Decision: the third route (`Pdeathsig`) is already the documented contract; this spec makes the `.ci` owner meet it rather than adding a new route
  → Constraint: every fork takes its attributes and cancellation from its process owner; the `.ci` owner keeps the group kill on cancel and the before-reap group retirement
- [ ] `internal/component/plugin/sysproc_linux.go` - `NewSysProcAttr`, `StartChild`
  → Constraint: the kernel sends the death signal when the THREAD that forked exits, and the Go runtime destroys a thread whose goroutine returns while locked (ze's capability probes do this on purpose), so a fork with `Pdeathsig` MUST come from a thread that outlives the child; `StartChild` forks from a persistent spawner thread for exactly this (`TestStartChildSurvivesForkingThreadExit`)
- [ ] `internal/test/runner/netns_linux.go` - `enterTestNetns`
  → Constraint: in netns mode ze, the peer and fixtures are forked from the test goroutine's locked thread so they inherit its namespace (A-5 there, `TestNetnsLaunchChildInheritsNamespace`); they MUST NOT be forked from the spawner thread, which sits in the host namespace
  → Constraint: that thread stays locked for the test's whole lifetime, and the restore is deferred before the children's `finish`, so it outlives every child the test started

**Key insights:**
- One function, `startWithETXTBSYRetry`, starts every long-lived `.ci` child, so it is the single place the fix lands
- `StartServerProcess` in the same package already does the right thing for owned servers; the `.ci` path is the half the guard was not added to
- Netns mode needs a different forking thread, not different attributes

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/test/runner/runner_exec_util.go` - `startWithETXTBSYRetry` (calls `boundProcessDeadline`, then `proc.Start()` on the caller's thread; a retry copies `SysProcAttr` for the netns credential drop); `boundProcessDeadline` (sets `Setpgid`, the group-kill `Cancel`, `WaitDelay`)
- [ ] `internal/test/runner/process_owner.go` - `ownedProcess` (sole reaper, before-reap group SIGKILL, `finish`)
- [ ] `internal/test/runner/server_process.go` - `StartServerProcess` (`plugin.NewSysProcAttr` plus `plugin.StartChild`)
- [ ] `internal/test/runner/runner_exec.go` - the three callers: single peer (`le test peer`), single client (`ze`), multi-process commands (one per `cmd=` line, including `le test peer` with a `ze-peer-expect-*.msg` file); netns mode replaces `SysProcAttr` with a `Credential` for ze before the start
- [ ] `internal/le/test/functional/run.go`, `internal/le/gaterun/gaterun.go` - `Execute`, `StreamContext` (suite group SIGKILL on cancel)
- [ ] `internal/test/cli/ci_runner.go`, `internal/test/cli/cmd_peer.go` - runner and peer signal handling (both catch SIGINT/SIGTERM and cancel a context)

**Behavior to preserve:**
- Every child heads its own process group, and cancel or retirement SIGKILLs that group (reaches grandchildren such as ze's plugins)
- Netns-mode children inherit the test's network namespace and ze's credential drop; a retry after ETXTBSY keeps the credential
- A runner stopped by SIGINT or SIGTERM still tears down through its own code (exit statuses, output drain, temp files)
- Non-Linux builds compile and keep today's behaviour (`sysproc_other.go` has no `Pdeathsig`)

**Behavior to change:**
- A `.ci` child receives SIGKILL from the kernel when the runner process dies

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Plugin fork attributes | `internal/component/plugin/sysproc_linux.go` `NewSysProcAttr` | `Setpgid` plus `Pdeathsig: SIGKILL` for every forked plugin | Extends: the `.ci` owner takes the same attributes, merged with the netns `Credential` |
| Persistent spawner thread | `internal/component/plugin/sysproc_linux.go` `StartChild` | Forks from one locked thread that lives as long as the process, so `Pdeathsig` fires only when the process dies | Extends: the default `.ci` start goes through it |
| Owned server start | `internal/test/runner/server_process.go` `StartServerProcess` | Already combines the two above with `boundProcessDeadline` | Pattern followed; not reused as is because it hands back a `ServerProcess`, while the `.ci` callers need the `ownedProcess` and the ETXTBSY retry |
| `.ci` process owner | `internal/test/runner/runner_exec_util.go` `boundProcessDeadline`, `process_owner.go` `ownedProcess` | Own group, group kill on cancel, before-reap retirement | Extends: unchanged; gains the parent-death route |
| Netns locked thread | `internal/test/runner/netns_linux.go` `enterTestNetns` | Locks the test goroutine's thread into a per-test namespace for the test's lifetime | Does not apply as a change: a netns-mode child forks from that thread, which outlives it, so `Pdeathsig` is safe there without the spawner |
| Suite group stop | `internal/le/gaterun/gaterun.go` `StreamContext`, `internal/le/test/functional/run.go` `Execute` | SIGKILLs the suite's group on interrupt | Does not apply as a change: once the runner's children carry `Pdeathsig`, killing the runner kills them, so this stop becomes correct as written |
| Sandbox process reaping | `plan/post-release/engineering/spec-test-sandbox.md` AC-5 | Planned, not built | This spec takes its process half for the `.ci` runner |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le test <suite>` (or `le test functional ...`) starts a suite; the runner reads `.ci` files and starts children per test

### Transformation Path
1. Runner builds an `exec.Cmd` per child (`runner_exec.go`)
2. `startWithETXTBSYRetry` attaches the owner (`boundProcessDeadline`) and forks
3. The kernel records the forking thread as the child's parent for `Pdeathsig`
4. Runner death ends that thread; the kernel SIGKILLs the child

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Runner ↔ child process | fork/exec with `SysProcAttr` | No |
| Runner ↔ kernel | `PR_SET_PDEATHSIG` set by Go's fork path | No |

### Changed-Wire Census

No wire changes. The fix changes how a child is forked, not anything a process
writes or reads: no argv, no environment variable, no file format, no output
line changes.

**Search evidence:** scope `internal/test/runner`, terms `exec.Command`, `.Start()`, `startWithETXTBSYRetry`, `StartServerProcess`, `plugin.StartChild`. Control: `StartServerProcess` found as a known producer using `plugin.StartChild`. Findings: all three long-lived `.ci` starts call `startWithETXTBSYRetry`; the other starts (`decoding.go`, `parsing.go`, `runner_validate.go`, `runner.go`, `stale.go`) are short foreground commands bounded by the test context.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| none | writer | `internal/test/runner/runner_exec_util.go` `startWithETXTBSYRetry` | Fork attributes only; argv, env and pipes are set by the callers and unchanged | Preserve: no bytes change |
| none | reader | `internal/test/cli/cmd_peer.go` peer command | Reads the same argv and expect file as before | Preserve: no bytes change |

### Integration Points
- `plugin.NewSysProcAttr`, `plugin.StartChild` - attributes and forking thread
- `enterTestNetns` - the netns-mode forking thread

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The start stays inside the `.ci` owner |
| No unintended coupling (components stay isolated) | Yes | The runner already imports `internal/component/plugin` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `NewSysProcAttr` and `StartChild` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Not on a data path |
| Registration over hardcoding, outbound | Yes | No feature, command or handler added |
| Registration over hardcoding, inbound | Yes | No list learns a name; searched the runner's start sites above |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every long-lived `.ci` child is started through `startWithETXTBSYRetry` | Start-site census above | A child started elsewhere still leaks | Grep in the review gate; the regression test exercises the function itself | confirmed: the only long-lived starts are `runTest` peer and client and `runOrchestrated` commands, all through `startWithETXTBSYRetry`, which now refuses an unspecified fork thread |
| A-2 | In netns mode the forking thread outlives every child it forks | `enterTestNetns` comment: the thread stays locked for the whole test; restore is deferred before `finish` | Children killed early, mid-test, when the thread exits | `TestNetnsLaunchChildInheritsNamespace` plus a netns run of `test/runner` | confirmed in code: `defer restore()` in `runOrchestrated` is registered before every `defer owned.finish`, in the same function and goroutine, so every finish runs first; the netns runtime run needs a privileged host (see Design Insights) |
| A-3 | Merging `Pdeathsig` into the netns `Credential` attributes keeps the credential drop | Go applies `Credential` and then `Pdeathsig` in the child before exec | ze runs as root in netns mode, its readiness handshake breaks | The existing netns `.ci` suite stays green | confirmed in Go source: `syscall/exec_linux.go` applies setgid/setuid (lines 502, 506) before `PR_SET_PDEATHSIG` (line 555), so the credential drop does not clear the signal; `TestRetriedStartKeepsTheDeathSignal` asserts the caller's Credential survives the merge |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A netns-mode ze binary carries file capabilities (setcap); the kernel clears the death signal on exec of such a binary, so that ze still outlives a dead runner | ze in a netns run survives the regression scenario | Recorded as a known limitation below; the peer and fixtures are still covered |
| R-2 | Forking from the spawner thread serialises every `.ci` start through one thread | Suite wall time rises on a parallel run | The spawner is already used for every plugin ze forks; no timing figure is reported from this host (`ai/META-DECISIONS.md`), so the check is that a parallel `test/runner` run still passes within its budgets |
| R-3 | Pdeathsig fires on the thread, not the process; a start that bypasses the spawner on a non-locked thread kills its child when Go retires that thread | A child dies mid-test with SIGKILL and no runner action | The design routes every non-netns start through `StartChild`; the review checks no other path sets `Pdeathsig` |
| R-4 | ETXTBSY retry rebuilds the `exec.Cmd` and copies `SysProcAttr`; a retry that loses `Pdeathsig` silently reopens the leak | Regression test passes but a retried start lacks the attribute | The retry builds its attributes the same way as the first attempt; a unit test asserts the attribute on a retried start |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Functional tests: children killed early (false red), or netns tests run in the wrong namespace. Nothing shipped |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `spec-test-sandbox` (post-release, AC-5); any session editing `internal/test/runner` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A runner process that started a `.ci` child is SIGKILLed | → | `startWithETXTBSYRetry` fork attributes and thread | `TestRunnerKilledOutrightTakesItsChildren` |
| A netns-mode test forks its children | → | `startWithETXTBSYRetry` netns branch | `TestNetnsLaunchChildInheritsNamespace` (existing, must stay green) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A process running the `.ci` start path starts a long-lived child, then is SIGKILLed | The child has exited, killed by SIGKILL, within 5 s of the parent's death, and the test reads the child's exit to prove it | None: the child has no owner left |
| AC-2 | The same start path with no parent death, run to a normal test end | The child runs until the runner's own teardown stops it, and the test sees it alive while the runner lives | The child itself, for the whole life of its runner, because the test needs it |
| AC-3 | A start that hits ETXTBSY and retries | The started child carries the parent-death signal and its process group, exactly like a first-attempt start | None |
| AC-4 | A netns-mode test | Its children run in the test's namespace, ze runs under the dropped credential, and a non-ze child dies with a SIGKILLed runner | A setcap'd ze, whose exec clears the death signal (R-1, Known Limitations) |
| AC-5 | `docs/architecture/testing/runner-architecture.md` "Three things end a process" | The page names the `.ci` owner as meeting the `Pdeathsig` route and states the netns forking-thread rule and the setcap exception | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRunnerKilledOutrightTakesItsChildren` | `internal/test/runner/process_death_linux_test.go` | The test binary re-executes itself as a helper that starts `sleep` through `startWithETXTBSYRetry` and prints the child pid; the test SIGKILLs the helper and asserts the child is gone within 5 s (AC-1). Must go red with the fix reverted | |
| `TestOwnedChildLivesWhileItsRunnerLives` | `internal/test/runner/process_death_linux_test.go` | The same helper not killed: the child is alive after the goroutine that requested the start has returned from a locked thread (AC-2, guards R-3) | |
| `TestRetriedStartKeepsTheDeathSignal` | `internal/test/runner/runner_exec_util_test.go` | A start forced through the retry branch carries `Pdeathsig` and `Setpgid` (AC-3, R-4) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N-A: no numeric input | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing runner primitives | `test/runner/*.ci` via `./le test functional runner` | Peers and daemons still start, run and stop as before | |
| existing netns suite | the suites `netnsModeActive` gates | Children still see the test namespace and credential (AC-4) | |

### Interop Tests (Scope: protocol)
N-A: tooling, no protocol behaviour changes.

## Files to Modify
- `internal/test/runner/runner_exec_util.go` - `startWithETXTBSYRetry`: merge the plugin fork attributes into the caller's attributes; fork through `plugin.StartChild` outside netns mode and on the test's locked thread in netns mode; keep the attributes on a retry
- `internal/test/runner/runner_exec.go` - the netns `Credential` assignment merges into, rather than replaces, the attributes
- `docs/architecture/testing/runner-architecture.md` - AC-5
- `internal/le/test/functional/run.go` - `Execute` comment: the group-kill premise now holds through the death signal, not because children stay in the group
- `plan/post-release/engineering/spec-test-sandbox.md` - AC-5 names this spec as owner of the `.ci` process half

## Files to Create
- `internal/test/runner/process_death_linux_test.go` - AC-1, AC-2

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config |
| YANG validation constraints | N-A | No config |
| YANG custom validators | N-A | No config |
| CLI commands/flags | N-A | No command or flag changes |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No command |
| Functional test for new RPC/API | N-A | No RPC; runner suite covers regressions |
| Pipe completeness | N-A | No output |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | Test tooling, no runtime dependency of ze |
| Doctor code per problem class | N-A | No new refusal or log line |
| Prometheus counters/metrics | N-A | None |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Test tooling only |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | No | - |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/runner-architecture.md` (the page that owns child lifetime) |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/testing/runner-architecture.md` |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/testing/runner-architecture.md` (affected, AC-5). `docs/architecture/testing/ci-format.md` (declared by a changed file's `// Design:` header; unaffected: no `.ci` key, option or command line changes, only how a child is forked). The rest derived by `./le spec citation anchors spec plan/release/validation/spec-test-children-die-with-runner.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

## Implementation Steps

1. **Phase: Wiring and red test** -- write `TestRunnerKilledOutrightTakesItsChildren` against today's `startWithETXTBSYRetry`
   - Tests: `TestRunnerKilledOutrightTakesItsChildren`
   - Files: `internal/test/runner/process_death_linux_test.go`
   - Verify: red today (the child survives its SIGKILLed parent); paste the output
2. **Phase: Fork through the owner's death route** -- attributes and forking thread in `startWithETXTBSYRetry`, credential merge in `runner_exec.go`
   - Tests: all three unit tests, `TestNetnsLaunchChildInheritsNamespace`
   - Files: `runner_exec_util.go`, `runner_exec.go`
   - Verify: green; then `./le test unit pkg path ./internal/test/runner/` and `./le test functional runner`
3. **Phase: Pages and comments** -- AC-5, the `Execute` comment, the sandbox spec AC-5 pointer
   - Verify: `./le doc check all`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | No `.ci` start sets `Pdeathsig` while forking from an unlocked goroutine thread (R-3) |
| Correctness | The netns branch forks on the locked test thread, never the spawner (A-2) |
| Correctness | The retry path builds the same attributes as the first attempt (R-4) |
| Data flow | Attributes come from `plugin.NewSysProcAttr`, never a second literal |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Death route on every `.ci` start | `TestRunnerKilledOutrightTakesItsChildren` green, red with the fix reverted |
| Netns unchanged | `TestNetnsLaunchChildInheritsNamespace` green |
| Page updated | `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Privilege | The netns credential drop is preserved on first start and retry (A-3) |
| Resource exhaustion | A dead runner leaves no child holding ports or memory |

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

- `TestRetriedStartKeepsTheDeathSignal` lives in `process_death_linux_test.go`, not `runner_exec_util_test.go`: that file builds on every platform and `Pdeathsig` is Linux-only. The retry is driven through `startWithRetry` with a stand-in fork that answers ETXTBSY to attempt 1, because a real ETXTBSY needs a racing fork (a clock, not a fence)
- The fork thread is a required argument (`forkThread`), and the zero value is refused, so a new caller cannot silently fork on a short-lived goroutine thread (R-3)
- Red on the old code, recorded 2026-11-11: `TestRunnerKilledOutrightTakesItsChildren` "the child 1563899 outlived its SIGKILLed runner: still in state S 5s after the runner died"; green after the fix
- Netns-mode runtime evidence (`TestNetnsLaunchChildInheritsNamespace`, a netns `.ci` run) needs a privileged host; the code-level evidence is A-2 and A-3

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Kernel death signal on every `.ci` child | (a) le sets itself as child subreaper and kills reparented descendants after a suite stop; (b) le stops a suite with SIGTERM first, SIGKILL after a grace | The death signal covers every way the runner dies, including the whole session being torn down, where (a) and (b) run no code. (a) also covers setcap'd ze and grandchildren, but only while the outer le survives; it is the follow-up if R-1 proves to matter. (b) only helps a stop le itself sends |
| Netns children fork on the test's locked thread | Fork them from the spawner thread too | The spawner is in the host namespace; forking there breaks the netns contract (`TestNetnsLaunchChildInheritsNamespace`) |
| No lifetime limit inside `le test peer` | Pass the test's timeout to the peer as a hard lifetime | With the death signal, no route leaves an orphaned peer; a second mechanism for the same guarantee is machinery the problem does not need (`ai/rules/simplicity.md`) |

## Known Limitations
- A netns-mode ze that carries file capabilities loses the death signal at exec (kernel rule for capability binaries). Its peer and fixtures are covered. Closing it needs the subreaper alternative above; if a netns run shows a surviving ze after this lands, that becomes a spec of its own in `plan/release/validation/`.
- Non-Linux hosts have no `Pdeathsig`; production and the test hosts are Linux (`ai/META-DECISIONS.md`, "Production is Linux").

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
- [ ] The targeted tests for what changed pass, run in the main checkout. No agent runs any `./le verify` command
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
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)

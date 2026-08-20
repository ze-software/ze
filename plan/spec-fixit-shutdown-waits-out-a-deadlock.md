# Spec: fixit-shutdown-waits-out-a-deadlock

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 6/6 |
| Deferral shard | `-` |
| Handoff | - |
| Updated | 2026-08-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Every `ze` daemon shutdown waits out a deadlock. Two timeouts break it, so
nothing hangs and nothing is red, which is why it has never been chased.

**The cycle, verified link by link.**

1. The hub stops the API server: `apiServer.Stop()` (`cmd/ze/hub/main.go`, seven
   call sites).
2. `(*Server).Stop` (`internal/component/plugin/server/server.go`) calls
   `s.cleanup()`, which calls `pm.Stop()` on the stored `ProcessManager`.
3. `ProcessManager.Stop` (`internal/component/plugin/process/manager.go`) waits
   for every plugin's engine through `Process.WaitEngine`
   (`internal/component/plugin/process/engine_wait.go`), which blocks on
   `p.engineDone`.
4. `engineDone` for the bgp plugin closes when `runBGPEngine`
   (`internal/component/bgp/plugin/register.go`) returns. It is blocked in
   `Reactor.Wait` (`internal/component/bgp/reactor/reactor.go`), which waits on
   `r.wg`.
5. `r.wg` still holds `Reactor.monitor`, whose deferred `cleanup` is running.
6. `Reactor.cleanup` Phase 1 calls `r.api.Stop()` -- **the same
   `*pluginserver.Server`** -- re-entering step 2 and a second `pm.Stop()` on the
   same manager, which waits again for the engine at step 4.

So the engine cannot return, because returning requires a cleanup that waits for
the engine. `pluginStopGrace` (3s, bounding `WaitEngine`) and the 500ms group
wait that follows it are the only exits.

**Measured**, four traced runs of `test/plugin/system-cpu-show.ci` under
`strace -f -ttt` at `-p 1`: 3001ms then 501ms, stable to a few ms, matching both
constants exactly. A `--pprof` goroutine dump during the window shows all three
goroutines in the cycle. Reproduced standalone with no peer ever connecting, so
it is not session-related.

**The comment above the wait states the opposite of what happens**: "An engine
that returns promptly costs nothing here." No engine can return promptly.

**This costs an operator, not only the tests.** A clean `ze` stop takes 3.5s and
logs `plugin engine did not finish its shutdown cleanup in time, resources it
installed may be left behind`. Either that warning is noise on every shutdown,
or the resources really are leaking. Both are defects, and which one is true is
part of AC-5.

**And it is most of a test suite.** 3.50s of a 7.5s `test/plugin` case, the same
in every `reload` case, and 2.0s in every `encode` case where the runner
SIGKILLs at its own grace instead. About 580 of 632 plugin tests start a daemon,
so roughly 2030s of that suite's 4545s of test-time -- **45%** -- is this one
timeout. The BGP handshake those tests exist to exercise takes 5ms.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - the plugin process contract and who owns a plugin's lifecycle
  → Constraint: the Process contract is "call Wait after Stop", so Stop signals and Wait blocks
- [ ] `ai/rules/goroutine-lifecycle.md` - before changing any shutdown path
  → Constraint: every goroutine has an owner that stops it and waits for it

**Key insights:**
- `Server` already has the right shape: `Stop` to signal and `Wait` to block, and `Reactor.cleanup` already uses both correctly -- Phase 1 signals, Phase 2 waits under one 2s deadline. The split is honoured everywhere except inside `Stop` itself, which blocks.
- `Reactor.cleanup`'s Phase 1 comment says "Signal everything to stop (non-blocking)". `r.api.Stop()` is the one call in that phase that blocks, for 3.5s.
- `plan/journal/plugin-startup-barrier-deadlock.md` holds a second, different plugin-barrier deadlock from 2026-08-10, also unfixed. Two rows in one class.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/process/manager.go` - `ProcessManager.Stop`, `pluginStopGrace`, the engine wait and the 500ms group wait
- [ ] `internal/component/plugin/process/engine_wait.go` - `Process.WaitEngine`, blocking on `p.engineDone`
- [ ] `internal/component/plugin/server/server.go` - `(*Server).Stop`, `(*Server).Wait`, `(*Server).cleanup` calling `pm.Stop()`
- [ ] `internal/component/bgp/reactor/reactor.go` - `Reactor.Wait`, `Reactor.monitor`, `Reactor.cleanup` and its two phases
- [ ] `internal/component/bgp/plugin/register.go` - `runBGPEngine`
- [ ] `cmd/ze/hub/main.go` - the `apiServer.Stop()` call sites

**Behavior to preserve:**
- The timeouts stay. They guard a genuinely stuck plugin, and this spec removes the CYCLE, not the guard. Shortening either constant is not a fix and would only make the leak warning arrive sooner.
- Whatever a plugin engine releases on its way out must still be released before the daemon exits. Speed is worthless if it converts a wait into a leak.
- `ze` must still stop cleanly under a signal, and the existing exit codes and log lines that tests assert on stay.

**Behavior to change:**
- The cycle is broken, so a clean shutdown does not wait out a timeout.
- The `resources it installed may be left behind` warning stops firing on a clean stop.

**→ Constraint: the design phase decides WHICH link to cut, and the candidates
are not equal.** Do not pick the smallest diff; pick the one that leaves one
owner per lifecycle.

| Candidate | What it says | Cost |
|---|---|---|
| A. `Reactor.cleanup` stops signalling `r.api` | the reactor does not own the API server; the hub creates it and stops it at seven sites. A component stopping its own host is the layering error underneath the cycle | must prove nothing else relied on the reactor stopping it, including a reactor used standalone in tests |
| B. `(*Server).Stop` becomes non-blocking | honours the Stop/Wait split the type already has, moving `pm.Stop()`'s wait into `Wait` | the wait still has to happen somewhere, and every caller of `Stop` must then call `Wait` |
| C. `ProcessManager.Stop` becomes re-entrant | a nested Stop does not re-wait for engines; the outer call owns that | defensive rather than corrective: it leaves two owners and hides the next instance |

A and B are not exclusive, and B alone does not remove the double ownership.

## Data Flow (MANDATORY)

### Entry Point
- A signal to the daemon, or `ze` stopping at the end of a functional test.

### Transformation Path
1. The hub begins shutdown and calls `apiServer.Stop()`.
2. `Server.Stop` signals, and today also waits through `ProcessManager.Stop`.
3. `ProcessManager.Stop` waits for each plugin engine to return.
4. The bgp engine's return requires `Reactor.cleanup` to finish.
5. `Reactor.cleanup` today re-enters step 2.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| hub ↔ plugin server | `apiServer.Stop()` | Yes -- `cmd/ze/hub/main.go`, `runHub`, the final shutdown pair `apiServer.Stop()` then `eng.Stop(stopCtx)` under a 3s budget |
| plugin server ↔ process manager | `s.procManager.Load().Stop()` | Yes -- `(*Server).Stop` calls `s.cleanup`, whose whole body is that call (`internal/component/plugin/server/server.go`) |
| process manager ↔ engine | `Process.WaitEngine` on `engineDone` | Yes -- `ProcessManager.Stop` waits every process under one `pluginStopGrace` context, then the 500ms group wait (`internal/component/plugin/process/manager.go`) |
| bgp engine ↔ reactor | `runBGPEngine` calling `Reactor.Wait` | Yes -- the tail of `runBGPEngine` calls `bgpReactor.Stop()` then `bgpReactor.Wait(context.Background())`, an UNBOUNDED wait (`internal/component/bgp/plugin/register.go`) |
| reactor ↔ plugin server | `Reactor.cleanup` calling `r.api.Stop()` | Yes -- `cleanup` Phase 1 calls it with no ownership guard, unlike `abortStartup` in the same file, which guards on `!r.externalServer` |

### Integration Points
- `Reactor.cleanup` Phase 1 and Phase 2, which already implement signal-then-wait correctly.
- The hub's shutdown sequence, which already gives `eng.Stop` a 3s budget and warns when it is missed.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `r.api` the reactor stops is the same `*pluginserver.Server` the hub owns and stops | `reactor.go` types the field `*pluginserver.Server`; the hub calls `apiServer.Stop()` | Candidate A is wrong and the reactor owns a second server | Trace construction: who builds the Server the reactor is given | **confirmed, in borrow mode only.** One pointer, four hops: `pluginserver.NewServer` in `cmd/ze/hub/main.go` -> `registry.SetPluginServer(apiServer)` -> `registry.GetPluginServer()` read by `runBGPEngine` (`internal/component/bgp/plugin/register.go`) -> `Reactor.SetPluginServerAny` assigning `r.api`. In STANDALONE mode the reactor builds its own server instead (`startAPIServer`, `!r.externalServer` branch), so candidate A is void for that mode and the fix is guarded rather than unconditional |
| A-2 | Nothing depends on the reactor stopping the API server | the hub stops it at seven sites | Removing the call leaves the API running after a reactor-only shutdown | Find every path where a reactor stops without the hub also stopping, tests included | **broken as stated, and the break is what shapes the fix.** One production path DOES depend on it: the ze-chaos in-process simulation (`internal/chaos/inprocess/runner.go`) builds a standalone reactor through `LoadReactorWithPluginsStandalone` (`internal/component/bgp/config/loader.go`), stops it with `reactorCancel()` + `reactor.Wait`, and nothing else ever stops that server -- so `cleanup` is its only stop. Every `Standalone: true` reactor unit test is the same shape. `Reactor.abortStartup` already guards on `!r.externalServer`. The borrow-mode reload path (`runBGPEngine` returning when bgp is removed) does NOT depend on it and is actively harmed by it. Conclusion: guard the call on ownership, never remove it |
| A-3 | The engine genuinely releases everything it installed, so the warning is noise rather than a real leak | untested; the log says resources "may be" left behind | The warning is TRUE today and speeding shutdown makes a real leak more likely | AC-5: prove the release happens, do not assume the warning is cosmetic | **reframed, still unvalidated.** The warning is ACCURATE, not a false alarm: the bgp engine really has not returned when the grace expires, so `ProcessManager.Stop` returns while `Reactor.cleanup` is still running and the hub prints "Ze stopped." under it. Two facts narrow what is at risk. The Cease NOTIFICATIONs go out in `Reactor.stop` BEFORE the context cancel, so the wire-visible teardown completes inside the grace. What runs after it is `cleanup` Phase 2's component waits and Phase 3 (`r.recentUpdates.Stop()`), which are in-process. **Resolved by phase 5: the release LANDS.** `TestEngineReleasesWhatItInstalledOnStop` (`internal/component/plugin/process/manager_test.go`) drives an engine that installs eight entries in state the manager holds no reference to, re-enters `ProcessManager.Stop` the way the bgp engine does, and releases 700ms after its read loop ends. The stop returns with every entry released, inside the grace, and logs no warning. Zeroing the engine wait leaves all eight behind. So the warning was accurate about the OLD path, and the new path neither leaks nor warns |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The fix converts a 3.5s wait into a leak | a resource the engine installed survives a stop | AC-5 asserts release, not speed. A faster shutdown that leaks is a worse defect than the one being fixed |
| R-2 | Removing a Stop leaves the API server running in some path | a test hangs, or a port stays bound | A-2 enumerates every path before the call is removed |
| R-3 | A genuinely stuck plugin now hangs instead of timing out | a shutdown that never returns | The timeouts stay. This removes the cycle, never the guard |
| R-4 | Shutdown ordering changes break a `.ci` asserting on log lines | a functional test reds on missing output | The existing lines stay; only the delay and the warning go |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The daemon hangs on shutdown, or leaks what a plugin installed. This is the live shutdown path of the shipped binary |
| How is it reverted? | Single commit revert; the timeouts are untouched, so the old behaviour returns |
| Who else touches this path? | Every plugin, every functional test that starts a daemon, and every operator stopping ze |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze` stopping cleanly | → | the broken cycle | `TestCleanShutdownDoesNotWaitOutTheEngineGrace` |
| a plugin engine that genuinely hangs | → | `pluginStopGrace` | `TestAStuckEngineStillHitsItsGrace` |
| a stopped daemon | → | engine release | `TestEngineReleasesWhatItInstalledOnStop` |
| `ze` stopping under a signal | → | the whole path | `test/plugin/shutdown-is-prompt.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A daemon with the bgp plugin loaded is stopped cleanly | It stops well inside `pluginStopGrace`, and the measured 3.0s + 0.5s pair is gone |
| AC-2 | The same stop | No `plugin engine did not finish its shutdown cleanup in time` warning is logged |
| AC-3 | A plugin engine that genuinely does not return | `pluginStopGrace` still bounds the wait and the warning still fires, naming that plugin |
| AC-4 | `ProcessManager.Stop` is entered twice for one manager | The engine wait happens once; the second entry does not re-wait |
| AC-5 | A daemon that installed resources through a plugin is stopped | Everything the engine installed is released before exit, PROVEN rather than assumed. If the warning was telling the truth, that is a second defect and it gets its own row |
| AC-6 | `test/plugin/system-cpu-show.ci` is traced as before | Teardown is a small fraction of the test, not 3.5s of 7.5s |
| AC-7 | The `encode` suite runs | The runner no longer needs to SIGKILL at its 2s grace; the daemon exits on SIGTERM |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCleanShutdownDoesNotWaitOutTheEngineGrace` | `internal/component/plugin/process/manager_test.go` | AC-1, AC-2 | RED (phase 2). Measured 3.502874767s against a 3s grace, and the warning logged twice |
| `TestAStuckEngineStillHitsItsGrace` | `internal/component/plugin/process/manager_test.go` | AC-3: the guard survives | GREEN. Reds when the engine wait is zeroed: Stop returns in 501ms against the 900ms grace it asserts |
| `TestStopIsIdempotentForTheEngineWait` | `internal/component/plugin/process/manager_test.go` | AC-4 | GREEN. Reds when the `stopping.Swap` guard is dropped: the second entry took 2.502s |
| `TestARestartedManagerStillWaitsForItsEngines` | `internal/component/plugin/process/manager_test.go` | AC-4's guard is per generation | GREEN. Reds when `startConfigs` stops clearing the flag: the second stop returns before the release |
| `TestReactorCleanupDoesNotStopWhatItDoesNotOwn` | `internal/component/bgp/reactor/reactor_shutdown_ownership_test.go` | the chosen candidate's contract | GREEN. Reds when the `!r.externalServer` guard is removed. Two siblings share the file: `TestBGPRemovedAtReloadLeavesTheHubPluginServerRunning` (the reload case, which also loses an unrelated plugin) and `TestReactorCleanupStopsTheServerItOwns` (a standalone reactor still stops the server it built) |
| `TestEngineReleasesWhatItInstalledOnStop` | `internal/component/plugin/process/manager_test.go` | AC-5 | GREEN. Reds when the engine wait is zeroed: all eight installed entries survive the stop |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `pluginStopGrace` | unchanged at 3s | 3s | N/A | N/A |
| measured clean-stop time | 0-3000ms | well under the grace | N/A | 3000ms means the cycle is still there |

<!-- The grace is NOT a tuning knob in this spec. A clean stop at or near 3000ms
     is the failure signature, not a slow pass. -->

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `shutdown-is-prompt` | `test/plugin/shutdown-is-prompt.ci` | An operator stops ze and it exits promptly, with no warning that resources may have been left behind | GREEN (3.0s). With both guards reverted the daemon logs the warning twice and then never exits, so the case times out at 30s |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | Shutdown ordering, no wire-visible protocol change. A BGP session is torn down by NOTIFICATION as before, which existing interop scenarios already cover | |

## Files to Modify
- `internal/component/bgp/reactor/reactor.go` - `cleanup`, per the chosen candidate
- `internal/component/plugin/server/server.go` - `Stop` and `cleanup`, per the chosen candidate
- `internal/component/plugin/process/manager.go` - `Stop` re-entrancy, if candidate C is taken
- `cmd/ze/hub/main.go` - the shutdown ordering, if ownership moves
- `docs/architecture/api/process-protocol.md` - who owns a plugin's lifecycle

## Files to Create
- `test/plugin/shutdown-is-prompt.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config surface; a lifecycle fix |
| YANG validation constraints | N-A | as above |
| YANG custom validators | N-A | as above |
| CLI commands/flags | N-A | No command changes |
| CLI grammar (keyword before value) | N-A | as above |
| Editor autocomplete | N-A | as above |
| Functional test for new RPC/API | N-A | No RPC added |
| Pipe completeness | N-A | No new output |
| Env var registration | N-A | No `ze.*` leaf added |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Prometheus counters/metrics | No | Shutdown duration is a candidate, but adding a metric is not this fix |
| BGP family surface (new SAFI / capability / attribute) | N-A | No protocol surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | A defect fix |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | Yes | `docs/architecture/api/process-protocol.md`: who owns a plugin's lifecycle and who may stop the server |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | Yes | the Stop/Wait contract, if candidate B is taken |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | The `.ci` is a new test, not new infrastructure |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | the shutdown sequence |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep for anchors naming `manager.go`, `server.go`, `reactor.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Check the shutdown sequence description in `docs/architecture/` |

## Implementation Steps

1. **Phase: Establish ownership (MANDATORY FIRST)** -- A-1 and A-2 decide the design
   - Trace who CONSTRUCTS the `*pluginserver.Server` the reactor holds, and enumerate every path where a reactor stops without the hub also stopping, tests included
   - Choose between candidates A, B and C with that evidence, and record the choice and the rejected alternatives in Key Design Decisions
   - **If the reactor turns out to own a second, distinct server, candidate A is void and the design changes.** Say so rather than proceeding on the assumption
2. **Phase: Wiring** -- a failing test that pins the current 3.0s + 0.5s signature, so the fix has something to turn green
   - Tests: `TestCleanShutdownDoesNotWaitOutTheEngineGrace`
3. **Phase: Break the cycle** -- apply the chosen candidate
   - Tests: as above plus `TestStopIsIdempotentForTheEngineWait`
4. **Phase: Prove the guard survives** -- a genuinely stuck engine still hits its grace and still warns
   - Tests: `TestAStuckEngineStillHitsItsGrace`
5. **Phase: Prove release** -- AC-5, which is the risk this fix carries
   - Tests: `TestEngineReleasesWhatItInstalledOnStop`, `test/plugin/shutdown-is-prompt.ci`
6. **Phase: Docs** -- the ownership rule, so the next component does not stop its host

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | The cycle is broken at ONE place, with one owner per lifecycle |
| Correctness | The timeouts still bound a genuinely stuck plugin. Prove it with a test that hangs an engine deliberately |
| Naming | One name for the shutdown phase that signals and one for the phase that waits |
| Data flow | No component stops a server it does not own |
| Rule: `ai/rules/goroutine-lifecycle.md` | Every goroutine still has an owner that stops it and waits for it |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| A clean stop is prompt | trace `test/plugin/system-cpu-show.ci` as before and compare the teardown phase |
| The guard survives | `TestAStuckEngineStillHitsItsGrace` |
| Nothing leaks | `TestEngineReleasesWhatItInstalledOnStop` |
| The suite is faster | `make ze-functional-plugin-test` wall clock, before and after, same `-p` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | A shutdown that returns before a plugin released a socket, a netlink handle or an XFRM state leaves it held. AC-5 is the guard and it must not be waved through |

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

- The type already knows the answer: `Server` has `Stop` to signal and `Wait` to block, and `Reactor.cleanup` uses both correctly in two phases under one deadline. The defect is that one implementation of `Stop` blocks, inside a phase whose own comment says it does not.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| **Candidate A, applied as an ownership guard.** `Reactor.cleanup` stops and waits for `r.api` only when the reactor OWNS it, which is `!r.externalServer` -- the exact guard `Reactor.abortStartup` already applies (`internal/component/bgp/reactor/reactor.go`). One owner per lifecycle: whoever CONSTRUCTS the server stops it. The hub constructs it in production (`cmd/ze/hub/main.go`, `runHub`) and the reactor constructs it in standalone mode (`reactor.go`, `startAPIServer`) | B, C | A cuts the cycle at its cause and makes the two teardown paths of one file agree. The file already knows the rule, and `cleanup` is the one place that does not apply it |
| Rejected: **B, `(*Server).Stop` becomes non-blocking** | A, C | B does not break the cycle, it re-times it. Moving `pm.Stop()`'s wait into `(*Server).Wait` leaves `cleanup` Phase 2's `r.api.Wait(waitCtx)` (`reactor.go`, `cleanup`) waiting for the engine that is RUNNING that cleanup, so 3.5s becomes the Phase-2 2s deadline and the double ownership survives untouched |
| Rejected: **C, `ProcessManager.Stop` becomes re-entrant** | A, B | C leaves the reactor stopping a server it does not own, and that is a second live defect rather than a style point. `runBGPEngine` (`internal/component/bgp/plugin/register.go`) returns when bgp is REMOVED AT RELOAD as well as at daemon shutdown, by its own comment, and its tail calls `bgpReactor.Stop()`. Today's unguarded `r.api.Stop()` therefore takes the hub's whole plugin server down on a reload that removed one plugin, while the daemon keeps running. C makes that quiet instead of fixing it |
| AC-4 (`ProcessManager.Stop` entered twice does not re-wait) stays owed, and it is NOT what breaks the cycle | folding AC-4 into the candidate choice | A removes the only known re-entrant caller, so AC-4 becomes a guard rather than a cure. The spec asks for it as an acceptance criterion, so phase 3 implements it as one and names it a guard |

## Known Limitations
- This fixes one cycle. `plan/journal/plugin-startup-barrier-deadlock.md` records a different plugin-barrier deadlock from 2026-08-10, on the STARTUP side, which this spec does not touch.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
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
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

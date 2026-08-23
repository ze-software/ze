# Spec: fixit-shutdown-waits-out-a-deadlock

| Field | Value |
|-------|-------|
| Status | done |
| Scope | plugin |
| Depends | - |
| Phase | 6/6 |
| Deferral shard | `-` |
| Handoff | - |
| Updated | 2026-08-23 |

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

<!-- Answered after the fix landed. Every row read No before it, which is what the
     cycle was. -->

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Shutdown runs hub -> `Server.Stop` -> `ProcessManager.Stop` -> `Process.WaitEngine` in one direction. `Reactor.cleanup` (`internal/component/bgp/reactor/reactor.go`) no longer calls back up into the server that hosts it |
| No unintended coupling (components stay isolated) | Yes | The reactor reads a borrowed server and never stops it. The guard is `!r.externalServer`, set once at construction from `!config.Standalone` (`reactor.go`, `New`), the same predicate `abortStartup` already applied |
| No duplicated functionality (extends existing, does not recreate) | Yes | No new type and no new lifecycle. Two existing conditions gained an existing predicate, and `ProcessManager` gained one `atomic.Bool` field |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Shutdown ordering only. No buffer and no encoding path is touched |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No plugin is named anywhere in the fix. `ProcessManager.Stop` treats every process the same way, and the reactor guard tests ownership rather than a plugin name |

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
| 8 | Plugin SDK/protocol changed? | No | Candidate B was rejected, so `(*Server).Stop` keeps its signature and its blocking behaviour. No SDK surface moved |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | The `.ci` is a new test, not new infrastructure |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | the shutdown sequence |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep for anchors naming `manager.go`, `server.go`, `reactor.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Check the shutdown sequence description in `docs/architecture/` |
| 18 | `docs/architecture/core-design.md`, the design doc `reactor.go` declares? | No | Its only shutdown claim is the Engine bullet, "Engine supervises startup/shutdown order", which stays true: the fix moves who stops the plugin server, not who supervises the order. `grep -in "cleanup\|api.Stop\|plugin server" docs/architecture/core-design.md` returns no claim about which component stops that server |
| 19 | `docs/architecture/hub-architecture.md`, the design doc `cmd/ze/hub/main.go` declares? | No | `cmd/ze/hub/main.go` was NOT modified: ownership stayed where it was, so the conditional entry in Files to Modify never fired. `grep -in "shutdown\|apiServer\|plugin server\|Stop()" docs/architecture/hub-architecture.md` returns two lines, both bare path references in a component list, neither an ordering claim |

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
- [ ] Learned summary written as a row in `plan/journal/<class>.md` (`plan/learned/NNN-*.md` is no longer a destination)
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented

The cycle is cut by candidate A, applied as an ownership guard, in `bde0241cf`
(pushed). `Reactor.cleanup` (`internal/component/bgp/reactor/reactor.go`) stops and
waits for `r.api` only when `!r.externalServer`, the predicate `abortStartup` in the
same file already used. Ownership is fixed at construction: `New` sets
`externalServer: !config.Standalone`, so the guard reads a property of the reactor
rather than of the call site.

`ProcessManager.Stop` (`internal/component/plugin/process/manager.go`) gained the
AC-4 guard: `if pm.stopping.Swap(true) { return }`. It is a guard, not the cure, and
the spec says so. Every spawn site clears the flag, because a manager holding live
processes MUST have a `Stop` that waits for them.

Six unit tests and one `.ci` were added. The docs got the ownership rule in
`docs/architecture/api/process-protocol.md`, "Who stops the plugin server", with four
source anchors.

### Bugs Found/Fixed

Two, both found by this closure's review over the landed diff.

- **The re-entry guard was cleared at one of three spawn sites.** `startConfigs`
  cleared it and its comment claimed "Every spawn goes through here". `Respawn`
  (`manager.go`) and `AddProcess` do not: each stores into `pm.processes` directly.
  `Respawn` is reached in production from `Server.restartPlugin`
  (`internal/component/plugin/server/reload_tx.go`) when a reload rollback reports a
  broken plugin. `Stop` cancels `pm.ctx` before it waits, and
  `Process.startInternal` (`internal/component/plugin/process/process.go`) never
  reads that context: it makes the pipe, sets `running`, and starts the engine
  goroutine whatever the context says. So a respawn landing after a `Stop`'s engine
  wait left a LIVE engine behind a set flag, and the next `Stop` returned before that
  engine released. The guard added to stop a leak could itself leak. Fixed at all
  three sites; covered by `TestARespawnedPluginStillGetsItsEngineWait`, which reds in
  0.70s against the unfixed `Respawn` with the message "the stop after a respawn
  returned before the respawned engine released".
- **`test/plugin/shutdown-is-prompt.ci` carried `option=skip-os:value=darwin`.** The
  one functional proof of a defect no gate could see was invisible on the primary
  development machine, and no reason for the skip was recorded. Nothing in the test
  is platform-specific. Measured with the skip removed: 5 of 5 pass on darwin in 1.1s
  to 2.3s against a 30s budget, and with both ownership guards reverted the same
  darwin run fails on `reject=stderr:pattern=may be left behind`, 3.001s after the
  shutdown request, with the warning logged twice for `plugins=bgp`. The skip is
  removed and the file now states why it must not come back.

### Documentation Updates

- `docs/architecture/api/process-protocol.md`, "Who stops the plugin server": the
  ownership rule, the two measured costs of breaking it, and the standalone
  exception. Four `<!-- source: -->` anchors, one of them repointed later by
  `5a713b067` so it names the function it claims.
- `docs/architecture/core-design.md` and `docs/architecture/hub-architecture.md`
  checked and unaffected: rows 18 and 19 of the Documentation Update Checklist carry
  the greps.
- `make ze-repository-check`: `all checks passed`.

### Deviations from Plan

- `cmd/ze/hub/main.go` is listed under Files to Modify with the condition "if
  ownership moves". Ownership did not move, so the file was not touched. The hub was
  already the owner; the fix stopped a component contradicting it.
- `internal/component/plugin/server/server.go` is listed under Files to Modify "per
  the chosen candidate". Candidate B was rejected, so it was not touched.
- The learned summary is a `plan/journal/` row. `plan/learned/NNN-*.md` is no longer
  a destination.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2 assumed nothing depends on the reactor stopping the API server | The ze-chaos in-process runner (`internal/chaos/inprocess/runner.go`) builds a standalone reactor whose `cleanup` is that server's only stop | Phase 1's ownership trace | The call is GUARDED on ownership, never removed. `TestReactorCleanupStopsTheServerItOwns` pins the standalone half |
| approach | The re-entry guard's own comment asserted a property of the code that was false: "Every spawn goes through here" | Three sites write `pm.processes`, and `Respawn` is reachable in production | Closure review, guard audit item 3 (`ai/rules/evidence.md`: a false safety claim is the shield that stops the next reviewer asking) | All three sites clear the flag; the comment now names all three and says why |
| escalation | A test written for the change was skipped on the machine the work happens on, so its green carried no information there | The test passes on darwin and discriminates on darwin | Closure review asked whether the proof could fail, and ran it | Skip removed. Row added to `plan/journal/green-that-could-not-have-been-red.md` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Break the cycle at one place, one owner per lifecycle | Done | `internal/component/bgp/reactor/reactor.go`, `Reactor.cleanup` Phase 1 and Phase 2 | Guarded on `!r.externalServer`, set at `New` from `!config.Standalone` |
| The timeouts stay | Done | `internal/component/plugin/process/manager.go`, `pluginStopGrace` unchanged at 3s, the 500ms group wait unchanged | `TestAStuckEngineStillHitsItsGrace` asserts the bound is still spent |
| Nothing the engine installed is lost | Done | `ProcessManager.Stop`, the engine wait on `Process.WaitEngine` | `TestEngineReleasesWhatItInstalledOnStop`, and `TestARespawnedPluginStillGetsItsEngineWait` for the respawn site |
| `ze` still stops cleanly, same exit code and log lines | Done | `test/plugin/shutdown-is-prompt.ci` | `expect=exit:code=0`, `expect=stdout:contains=Ze stopped.` |
| The warning stops firing on a clean stop | Done | `ProcessManager.Stop`, the `late` branch | `reject=stderr:pattern=may be left behind` in the `.ci` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestCleanShutdownDoesNotWaitOutTheEngineGrace` (`internal/component/plugin/process/manager_test.go`) | Reds at 3.502s against the 3s grace when the guard is removed |
| AC-2 | Done | Same test's `NotContains "may be left behind"`, and the `.ci` reject | Reds with the warning logged twice |
| AC-3 | Done | `TestAStuckEngineStillHitsItsGrace` | Reds at 500ms against a 900ms grace when the engine wait is zeroed |
| AC-4 | Done | `TestStopIsIdempotentForTheEngineWait`, `TestARestartedManagerStillWaitsForItsEngines`, `TestARespawnedPluginStillGetsItsEngineWait` | The third is new in this closure and covers the site the guard originally missed |
| AC-5 | Done | `TestEngineReleasesWhatItInstalledOnStop` | Eight installed entries, released 700ms after the read loop ends, inside the grace, no warning |
| AC-6 | Done | The `.ci` measured end to end on darwin | The whole case, daemon start included, is 1.1s to 2.3s. Teardown is no longer a 3.5s fixed cost. The Linux `strace` re-trace of `system-cpu-show.ci` was not repeated: that file is darwin-skipped for a real reason, Linux `/proc` CPU stats |
| AC-7 | Done | `ze-test bgp encode --start 1`: 59/59 pass, 14.4s total | No 2.0s plateau anywhere in the per-test timings, so no test is reaching the runner's SIGKILL grace. One test at 2.9s, none at 2.0s |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestCleanShutdownDoesNotWaitOutTheEngineGrace` | Done | `internal/component/plugin/process/manager_test.go` | |
| `TestAStuckEngineStillHitsItsGrace` | Done | same | |
| `TestStopIsIdempotentForTheEngineWait` | Done | same | |
| `TestARestartedManagerStillWaitsForItsEngines` | Done | same | |
| `TestEngineReleasesWhatItInstalledOnStop` | Done | same | |
| `TestARespawnedPluginStillGetsItsEngineWait` | Changed | same | ADDED by this closure. The TDD plan had no row for the respawn spawn site, which is why the guard shipped with a hole |
| `TestReactorCleanupDoesNotStopWhatItDoesNotOwn` | Done | `internal/component/bgp/reactor/reactor_shutdown_ownership_test.go` | |
| `TestBGPRemovedAtReloadLeavesTheHubPluginServerRunning` | Done | same | |
| `TestReactorCleanupStopsTheServerItOwns` | Done | same | |
| `shutdown-is-prompt` | Changed | `test/plugin/shutdown-is-prompt.ci` | Darwin skip removed by this closure |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/reactor/reactor.go` | Done | `cleanup`, both phases guarded |
| `internal/component/plugin/server/server.go` | Skipped | Candidate B rejected, so `Stop` keeps its shape. The spec makes this file conditional on the candidate, so the condition is false rather than the work undone |
| `internal/component/plugin/process/manager.go` | Done | The AC-4 guard, plus the three-site clear this closure added |
| `cmd/ze/hub/main.go` | Skipped | Conditional on ownership moving. It did not |
| `docs/architecture/api/process-protocol.md` | Done | "Who stops the plugin server" |
| `test/plugin/shutdown-is-prompt.ci` | Done | Created, and its darwin skip removed here |

### Audit Summary
- **Total items:** 21
- **Done:** 17
- **Partial:** 0
- **Skipped:** 2, both conditional on candidate B or on ownership moving, and both conditions are false
- **Changed:** 2, one test added and one test's platform gate corrected

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A clean `ze` stop no longer waits out a deadlock | functional | `test/plugin/shutdown-is-prompt.ci`, 5 of 5 on darwin, 1.1s to 2.3s for the WHOLE case including daemon start. With both guards reverted, the same run reds 3.001s after the shutdown request |
| The proof can actually fail | discrimination | Six source mutations, six real reds, a duration reported in every one. A: manager guard removed, 3 tests red at 3.502s, 2.502s and 3.501s. B: reactor guard removed, 2 tests red. C: reactor calls deleted rather than guarded, the standalone test red. D: `startConfigs` clear removed, 1 red. E: engine wait zeroed, 2 red. F: `Respawn` clear removed, 1 red at 0.70s |
| The timeouts still bound a genuinely stuck plugin | unit | `TestAStuckEngineStillHitsItsGrace`: `Stop` returns at or after the 900ms grace and names the plugin. Zeroing the wait reds it at 500ms |
| Nothing the engine installed leaks | unit | `TestEngineReleasesWhatItInstalledOnStop`: 8 entries released inside the grace, no warning. Zeroing the wait leaves all 8 |
| The suite stops paying the timeout | functional | `ze-test bgp encode --start 1`, 59/59 in 14.4s with no 2.0s SIGKILL plateau |
| No wire-visible protocol change | interop N-A | Shutdown ordering only. The Cease NOTIFICATION still goes out in `Reactor.stop` before the context cancel, which existing interop scenarios cover |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| No shard exists for this spec | done | The spec metadata carries `Deferral shard: -`, and `plan/deferrals/` holds no file for this stem |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-shutdown-waits-out-a-deadlock-ebf5d9b3-b158-40df-bba0-32a51591883e.md` |
| `review_gate.py check` | `OK (0 code files, clean, hashes match ...)` |
| Rounds | 2. Round 1 over the landed diff plus the working tree found 2 findings. Round 2 over the fixes found 0 |
| Reviewer lenses used | wiring and ownership, guard audit (`ai/rules/evidence.md` fail-closed and false-safety-claim), discrimination and vacuity (`ai/rules/testing.md`), goroutine lifecycle (`ai/rules/goroutine-lifecycle.md`), blast radius over every `Stop` call site |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The re-entry guard is cleared at one of three spawn sites, and its comment asserts it is the only one. `Respawn` is reachable in production from `Server.restartPlugin`, and `Process.startInternal` starts an engine under a canceled context, so a respawn racing a shutdown leaves a live engine that the next `Stop` never waits for. The guard added to prevent a leak could cause one | `internal/component/plugin/process/manager.go`, `startConfigs` and `Respawn` and `AddProcess` | All three sites clear the flag. The comment now names all three and says why. `TestARespawnedPluginStillGetsItsEngineWait` added, red in 0.70s without the `Respawn` clear |
| 2 | ISSUE | `option=skip-os:value=darwin` with no recorded reason made the spec's only functional proof invisible on the primary development machine, on a spec whose whole premise is that the defect is invisible to every gate | `test/plugin/shutdown-is-prompt.ci` | Skip removed after measuring 5 of 5 green on darwin and a real red under the reverted guards. The file now carries the measurement and says not to skip it here |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/shutdown-is-prompt.ci` | Yes | `ls -la` reports it, and `ze-test bgp plugin --pattern shutdown-is-prompt` resolves it as test id 621 |
| `internal/component/bgp/reactor/reactor_shutdown_ownership_test.go` | Yes | `ls -la` reports 6.8K; `gopls symbols` lists its three test functions |
| `internal/component/plugin/process/manager_test.go` | Yes | `grep -n "^func Test"` lists all six manager tests |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | A clean stop is well inside `pluginStopGrace` | `make ze-unit-pkg-test PKG=./internal/component/plugin/...` green, `process` package 9.152s. Mutation A reds this test at 3.502s |
| AC-2 | No leak warning on a clean stop | Same test's `NotContains`; the `.ci`'s `reject=stderr:pattern=may be left behind` passes 5 of 5 on darwin |
| AC-3 | A stuck engine still hits its grace and is named | `TestAStuckEngineStillHitsItsGrace` green; mutation E reds it at 500ms against 900ms |
| AC-4 | The engine wait happens once, and only while nothing has spawned since | Three tests green; mutations A, D and F each red exactly one of them |
| AC-5 | Everything installed is released before exit | `TestEngineReleasesWhatItInstalledOnStop` green; mutations A and E each leave all 8 entries |
| AC-6 | Teardown is a small fraction of the test | `.ci` whole-case time 1.1s to 2.3s on darwin, daemon start included |
| AC-7 | The encode suite does not need the SIGKILL grace | 59/59 in 14.4s, no per-test time at 2.0s |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze` stopping cleanly | `test/plugin/shutdown-is-prompt.ci` | Yes. Read the file: it brings a daemon up with the bgp plugin and an external plugin, issues `request shutdown` through `ze_api`, and asserts exit 0, `Ze stopped.`, and the absence of the leak warning |
| a plugin engine that genuinely hangs | unit, `TestAStuckEngineStillHitsItsGrace` | Yes. No `.ci` can hang an engine deliberately |
| a stopped daemon releasing what it installed | unit, `TestEngineReleasesWhatItInstalledOnStop` | Yes, plus the `.ci`'s reject of the warning at the daemon |
| `ze` stopping under a signal | `test/plugin/shutdown-is-prompt.ci` | Yes, through `request shutdown`, which is the same `Server.Wait` then `apiServer.Stop` then `eng.Stop` path |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | One pointer, four hops, `pluginserver.NewServer` in `cmd/ze/hub/main.go` to `Reactor.SetPluginServerAny`. `grep SetPluginServer internal/ cmd/` finds exactly one production injector, in `runYANGConfig` (`cmd/ze/hub/main.go`) |
| A-2 | broken | The ze-chaos in-process runner depends on the reactor stopping the server it BUILT. The call is guarded rather than removed, and `TestReactorCleanupStopsTheServerItOwns` pins it |
| A-3 | confirmed | The warning was accurate about the old path and the new path neither leaks nor warns. `TestEngineReleasesWhatItInstalledOnStop` proves the release lands inside the grace |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| "Whoever CONSTRUCTS the plugin server stops it, and nobody else" | `Reactor.cleanup`'s two `!r.externalServer` guards and `New`'s `externalServer: !config.Standalone`; the hub's `apiServer.Stop()` sites in `cmd/ze/hub/main.go` | Yes |
| "A component that CONSTRUCTS its own server still stops it" | `startAPIServer`'s `!r.externalServer` branch, and `internal/chaos/inprocess/runner.go` | Yes |
| Rows 18 and 19: `core-design.md` and `hub-architecture.md` unaffected | The two greps recorded in those rows return no ownership or shutdown-ordering claim | Yes |
| `make ze-repository-check` | `all checks passed`, so no source anchor went stale | Yes |

## Core Insight

The type already carried the answer twice, and the file already carried it once.
`Server` splits `Stop` from `Wait`, and `Reactor.abortStartup` already guarded the
same call on `!r.externalServer`. The defect was one teardown path in a file whose
other teardown path was correct. The general shape is worth more than the fix: when
two paths in one file do the same thing and only one carries a guard, the missing
guard is the bug, and the file has already told you what that guard should be.

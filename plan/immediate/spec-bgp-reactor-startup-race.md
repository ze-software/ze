# Spec: bgp-reactor-startup-race

| Field | Value |
|-------|-------|
| Status | completed |
| Scope | plugin |
| Depends | - |
| Phase | 2/2 |
| Handoff | - |
| Updated | 2026-10-04 |

Bucket: `plan/immediate/`, because the fix is in the reactor's shared
borrowed-server startup boundary. A-1 bounds the production timing evidence.

Scope is `plugin`: the defect sits on the boundary between the plugin server's
startup goroutine and the BGP reactor that borrows that server. No wire format
changes.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

In borrow mode the plugin server is constructed and started before the BGP
reactor. `Server.StartWithContext` launches `runPluginStartup` on its own
goroutine, and that goroutine ends by calling
`SignalPluginStartupComplete` on the reactor, which reads
`startupCompleteOnce` and `startupComplete`. Meanwhile the reactor's
`StartWithContext` reaches its API setup, which calls `SetAPIProcessCount`,
and `SetAPIProcessCount` assigns a new channel to `startupComplete` and a zero
value to `startupCompleteOnce`. The two goroutines touch the same fields with
no ordering between them, so `go test -race` reports a data race.

The defect was recorded on 2026-10-03 in
`plan/journal/new-caller-assumes-the-old-callers-timing.md`, red at HEAD
c39f0b9c38. The journal row names five reactor tests that the race detector
fails:

| Test | File | How it reaches the race |
|------|------|-------------------------|
| `TestRFC7705NoPrependInstalledAndAdvertised` | `internal/component/bgp/reactor/rfc7705_live_behavior_test.go` | `lowLiveRouter`: borrowed server started, then reactor `StartWithContext` |
| `TestRFC7705MigrationWireSemantics` | `internal/component/bgp/reactor/rfc7705_live_behavior_test.go` | `lowLiveRouter` |
| `TestRFC7947AllAttributesReachClient` | `internal/component/bgp/reactor/rfc7947_consumer_test.go` | `lowLiveRouter` |
| `TestRouteServerTransparencyStopsAtOrdinaryPeer` | `internal/component/bgp/reactor/rfc7947_consumer_test.go` | `lowLiveRouter` |
| `TestRFC9687Event29ReleasesTheLivePeersRIB` | `internal/component/bgp/reactor/rfc9687_rib_release_test.go` | borrowed server, then reactor `StartWithContext` |

The original journal abbreviated the two rfc7947 names. The historical red run
confirmed both names above. The closure edit writes all five names into the
journal so the evidence survives removal of this spec.

Goal:

1. No data race between the plugin server's startup goroutine and the
   reactor's start, in borrow mode and in standalone mode. The five tests
   above pass under `-race`.
2. The startup ordering is stated as a contract: which side creates the
   startup-sync state, which side may signal it, and what happens when the
   signal arrives before the reactor has started. The contract lives in the
   owning architecture page and in the function doc comments, and a test
   proves it.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - named by the `// Design:` header of `api_sync.go` as the page for API process synchronization
  → Constraint: the page carries no statement about startup ordering between the server and the reactor today (grep for `startupComplete`, `SetAPIProcessCount` and `borrow` finds nothing), so the contract this spec owes has no existing home to contradict and must be written
  → Decision: the page has no "API process synchronization" section at all, so the header pointer dangles. The contract goes to `docs/architecture/api/architecture.md`, which `ai/CODE-TO-DOCS.md` already lists for `api_sync.go`, and the header is repointed there
- [ ] `docs/architecture/api/process-protocol.md` - the 5-stage startup protocol and "Who stops the plugin server"
  → Decision: whoever constructs the plugin server owns it; the reactor borrows it through `registry.SetPluginServer` and `Reactor.SetPluginServerAny`, and a borrowed server is read, never stopped
  → Constraint: `sendPostStartupToAll` runs after `signalStartupComplete` freezes the command registry; any reordering of the reactor signal must keep that order
- [ ] `docs/architecture/api/architecture.md` - listed for `api_sync.go` in `ai/CODE-TO-DOCS.md`
- [ ] `docs/architecture/plugin/rib-storage-design.md` - the `// Design:` page of `rfc7705_live_behavior_test.go` and `rfc9687_rib_release_test.go`
  → Constraint: the page says nothing about when a test starts peers (grep for `StartPeers`, `borrow` and `lowLiveRouter` finds nothing), so moving those tests' peer start behind server startup leaves it true

### RFC Summaries (Scope: protocol)
- N-A: Scope is `plugin`; no RFC behavior changes. The failing tests are tagged for RFC 7705, 7947 and 9687 only because they drive a live reactor.

**Key insights:** (minimal context to resume after compaction)
- Before the fix, `SetAPIProcessCount` was called on every reactor start, in both modes, after the server already existed in borrow mode. After it, only a standalone start calls it, before its own server starts.
- In borrow mode the reactor never waits on `startupComplete` (`WaitForPluginStartupComplete` runs only under `!r.externalServer`), so the channel it re-creates there serves no reader.
- The race is on plain struct fields (`chan`, `sync.Once`), not on the atomics beside them.

## Current Behavior (MANDATORY)

This section records the pre-fix behavior. The Implementation Summary records
the completed change and its verification.

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/reactor/api_sync.go` - `aPISyncState` holds `processCount` and `readyCount` as atomics and `apiReady`, `apiReadyOnce`, `startupComplete`, `startupCompleteOnce` as plain fields
  → Constraint: `SetAPIProcessCount` stores the count, zeroes `readyCount`, defaults `apiTimeout`, then unconditionally assigns a new `startupComplete` channel and a fresh `startupCompleteOnce`; when the count is non-zero it also re-creates `apiReady` and `apiReadyOnce`. Its comment says it "Must be called before WaitForAPIReady", and nothing states it must also precede any signal
  → Constraint: `SignalPluginStartupComplete` runs `startupCompleteOnce.Do` and closes `startupComplete` when non-nil; it is called from the server's goroutine
  → Constraint: `AddAPIProcessCount` lazily assigns `apiReady` and `apiReadyOnce` with no lock, and `signalAllReady` reads them from `SignalAPIReady`, which the server also drives; the same unordered-write shape sits on the `apiReady` pair
  → Constraint: `WaitForPluginStartupComplete` returns at once when `startupComplete` is nil, otherwise waits on it, a timeout of three API timeouts, or the reactor context
- [ ] `internal/component/bgp/reactor/reactor.go` - the API setup reached from `StartWithContext`, and the startup wait
  → Constraint: borrow mode is fixed at construction (`externalServer` is `!Config.Standalone`); borrow mode without an injected server returns `errBorrowModeNoServer`
  → Constraint: the API setup creates and owns a server only when `!r.externalServer`, wires the BGP handlers in both modes, then calls `SetAPIProcessCount(len(r.config.Plugins))`, then starts the server only when it owns it. In standalone mode the count is therefore set before the server's startup goroutine exists; in borrow mode it is set after
  → Constraint: `StartWithContext` skips `WaitForPluginStartupComplete` under `r.externalServer`, because the reactor is itself a plugin of that server and waiting would wait for itself
- [ ] `internal/component/plugin/server/server.go` - `StartWithContext`
  → Constraint: launches `runPluginStartup` on a goroutine when any plugins, paths, families, custom events or send types are configured, otherwise calls `signalStartupComplete` inline; the comment there relies on `SetAPIProcessCount` having created `startupComplete` already, which holds in standalone mode only
- [ ] `internal/component/plugin/server/startup.go` - end of `runPluginStartup`
  → Constraint: calls `sendPostStartupToAll`, then `s.reactor.SignalPluginStartupComplete()` when a reactor is attached, then closes `startupDone`
- [ ] `internal/component/plugin/server/startup_autoload.go` - config-path auto-load
  → Constraint: also calls `SignalPluginStartupComplete` after a phase settles, relying on the reactor's `sync.Once` to make a reload a no-op; a re-created Once breaks that assumption
- [ ] `internal/component/bgp/reactor/reactor_shutdown_ownership_test.go` - `newBorrowedPluginServer`
  → Constraint: the test helper builds and starts the server, waits for the named plugins to spawn, and only then lets the caller attach and start the reactor, which mirrors the hub (`cmd/ze/hub/main.go`)

**Behavior to preserve:** (unless the user explicitly said to change it)
- Standalone mode: `WaitForPluginStartupComplete` blocks until Phase 1 and Phase 2 finish, then returns; auto-load configs with zero explicit plugins still wait.
- Borrow mode: the reactor never waits on plugin startup, never starts or stops the borrowed server.
- `SignalPluginStartupComplete` stays idempotent across reloads.
- `WaitForAPIReady` semantics and its timeout and context-cancel paths.

**Behavior to change:** (only what the user asked for)
- The startup-sync fields are no longer written by one goroutine while another reads them with no ordering; `go test -race` is clean for the five tests.
- The ordering between server start, reactor start and the startup signal becomes a stated, tested contract.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The hub (or a test) constructs a plugin server, starts it, and hands it to the reactor through `SetPluginServer`; the reactor's `StartWithContext` follows.
- The signal enters as a method call from the server's startup goroutine through `reactorAPIAdapter.SignalPluginStartupComplete`.

### Transformation Path
1. Server `StartWithContext` spawns `runPluginStartup` (or signals inline when nothing is configured).
2. Reactor `StartWithContext` reaches its API setup and calls `SetAPIProcessCount`, re-initializing the sync state.
3. `runPluginStartup` finishes its phases and calls `SignalPluginStartupComplete`, which reads the same state.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin server ↔ BGP reactor | `reactorAPIAdapter` (tests, standalone) or `plugin.Coordinator` (hub) method calls across goroutines | Yes: standalone arms before the server's goroutines exist; borrow mode writes nothing the server reads |

### Integration Points
- `reactorAPIAdapter.SignalPluginStartupComplete` (`internal/component/bgp/reactor/reactor_api.go`) forwards in standalone mode and the live fixtures. The hub uses `plugin.Coordinator`, which forwards after `SetReactor`.
- `SetPluginServer` / `SetPluginServerAny` - how a borrowed server reaches the reactor

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The server still signals through the same adapter methods; only the arming call moved |
| No unintended coupling (components stay isolated) | Yes | No new import; `reactor.go` and `api_sync.go` only |
| No duplicated functionality (extends existing, does not recreate) | Yes | One call moved inside the existing `!r.externalServer` block; no new state |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No buffers involved |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | N-A | No command, family or handler added |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | N-A | The change introduces no name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The production hub path has the same unordered interleaving as the test helper, not only the tests | `newBorrowedPluginServer` originally claimed to mirror the hub | A test reproduction cannot establish production timing | Read `cmd/ze/hub/main.go` `runYANGConfig`, `internal/component/bgp/plugin/register.go` `runBGPEngine`, and `internal/component/plugin/coordinator.go` `SetReactor` and startup forwarding methods | broken as an equivalence assumption: the fixture forwards from server construction, while the coordinator drops reactor-directed signals before attachment and forwards after `SetReactor`. `runBGPEngine` attaches before `StartWithContext`. This proves forwarding availability, not that completion occurs in that interval. No production reproduction is claimed. Removing the borrow-mode write does not depend on that timing |
| A-2 | The two rfc7947 tests in the journal row are `TestRFC7947AllAttributesReachClient` and `TestRouteServerTransparencyStopsAtOrdinaryPeer` | They are the only rfc7947 tests that call `lowLiveRouter` | The test table names the wrong tests | Scoped `go test -race -run` over the reactor package | confirmed: both red under `-race` in the red run, each report naming `SetAPIProcessCount` at `reactor.go` `startAPIServer` |
| A-3 | The `apiReady` / `apiReadyOnce` pair carries the same race shape in borrow mode | `SetAPIProcessCount` and `AddAPIProcessCount` assign them unguarded while `SignalAPIReady` runs from the server | The fix covers only the startup pair and the next race report names `apiReady` | `-race` run with a config that sets explicit plugins under a borrowed server | broken for the reactor-start race: with the fix, borrow mode never calls `SetAPIProcessCount`, so the reactor's goroutine writes neither field. The remaining lazy write in `AddAPIProcessCount` is between server goroutines, and the five live tests, which host explicit plugins under a borrowed server, report no race on it under `-race -count=3` |
| A-4 | A signal that arrives before `SetAPIProcessCount` is lost today, because the re-created channel is never closed | `SetAPIProcessCount` overwrites startup state after an early signal | No functional consequence in borrow mode (nobody waits), but the contract must say so | Read and test at design | confirmed for the old ordering: `SignalPluginStartupComplete` can consume its Once before `SetAPIProcessCount` creates a new open channel and resets the Once. The fix removes that borrow-mode reinitialization. Its `startupComplete` remains nil, and completion signals close nothing |
| A-5 | Nothing in borrow mode reads `processCount`, `readyCount`, `apiReady`, `apiReadyOnce`, `startupComplete` or `apiTimeout` outside the signal methods | grep over `internal/component/bgp/reactor` non-test sources | Design A changes a value something reads | Grep every reader | confirmed: the only readers are `api_sync.go` itself; `WaitForPluginStartupComplete` and `WaitForAPIReady` are called only under `!r.externalServer` in `StartWithContext`; `peer.go`'s `SignalAPIReady` is the per-peer barrier and touches none of these fields; the adapter methods in `reactor_api.go` only forward |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A fix that orders the two goroutines by making the reactor wait reintroduces the self-wait deadlock the borrow-mode skip exists to avoid | Live reactor tests hang at start | The contract states the reactor never waits on its own host in borrow mode; a test proves start returns |
| R-2 | Standalone mode loses its "auto-load configs still wait" guarantee | Standalone startup tests return before Phase 2 | Keep standalone tests that assert the wait in the test plan |
| R-3 | Reload path stops being idempotent | Second `SignalPluginStartupComplete` panics on a closed channel | Test a double signal |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Reactor start hangs or returns before plugins are ready; in the worst case a panic on a double close at reload |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `internal/component/plugin/server` startup phases, the hub start in `cmd/ze/hub/main.go`, every live reactor test that uses a borrowed server |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Borrowed server started, then reactor `StartWithContext` | → | `Reactor.startAPIServer` arming the barrier, `SignalPluginStartupComplete` | `TestRFC7705NoPrependInstalledAndAdvertised` under `-race` (existing, red before the fix); `TestBorrowModeStartArmsNoStartupBarrier` (new) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The five tests in the Task table run under `go test -race -count=3` with the repo feature tags | All pass, and the race detector reports nothing |
| AC-2 | A borrow-mode reactor starts while a goroutine calls `SignalPluginStartupComplete` and `AddAPIProcessCount(0)` in a loop, before, during and after `StartWithContext` | Red under `-race` without the fix, with the race report naming `SetAPIProcessCount` against `SignalPluginStartupComplete`; green with it. Start returns no error and leaves `startupComplete` nil. The loop's repeated signals also prove a repeated `SignalPluginStartupComplete` is a no-op that never panics |
| AC-3 | The standalone startup tests (`reactor_startup_test.go`, `api_sync_test.go`) | Green under `-race`: the standalone barrier, its waits, timeouts and immediate-ready paths are unchanged |
| AC-4 | A reader follows `api_sync.go`'s `// Design:` pointer and asks who arms the barrier, when, and what a borrow-mode signal does | The pointer names `docs/architecture/api/architecture.md`, "The reactor's startup barrier", which answers all three with source anchors to `Reactor.startAPIServer`, `SetAPIProcessCount` and `SignalPluginStartupComplete` |
| AC-5 | The journal row in `plan/journal/new-caller-assumes-the-old-callers-timing.md` | Names the five tests correctly and is marked fixed |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator starts the daemon with BGP configured | hub starts plugin server -> BGP plugin starts reactor -> server signals startup complete | The five live tests use a running plugin server and `reactorAPIAdapter`, then drive peer wire messages through `net.Pipe`. They establish the borrowed-server boundary, not the hub's exact scheduling. Current daemon plus external-plugin smoke also completed startup and exchanged policy-driven UPDATEs (recorded under Goal Validation) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| The five tests in the Task table, under `-race -count=3` | `internal/component/bgp/reactor/` | AC-1 | red before the fix, green after |
| `TestBorrowModeStartArmsNoStartupBarrier` | `internal/component/bgp/reactor/reactor_borrow_startup_barrier_test.go` | AC-2 | red before the fix, green after |
| `TestAPISync*`, `TestSignalPeerAPIReady*`, `TestStartWithContext*`, `TestStopAfterFailedStartupIsSafe`, `TestExternalServerDerivedFromMode`, `TestReactorBorrowModeErrorsWithoutServer`, `TestReactorStandaloneSelfHosts` | `api_sync_test.go`, `reactor_startup_test.go` | AC-3 | existing, green |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| API process count | 0 and above | N/A: the fix changes where the count is set, not its range; zero and non-zero are covered by `TestAPISyncNoProcesses` and `TestAPISyncSingleProcess` | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| No new functional test: the memory-ordering regression needs `-race`. Existing daemon startup and live external-plugin policy smoke cover the user path | Current session's `linklocal-raw-policy-sections` smoke and existing `test/` gating suite | Daemon with BGP starts and exchanges peer UPDATEs | Smoke passed with one scenario and zero failures. Full gating verification is ordered after startup closure, not claimed here |

### Interop Tests (Scope: protocol)
N-A: Scope is `plugin` and no wire-visible behavior changes.

## Files to Modify
- `internal/component/bgp/reactor/api_sync.go` - `// Design:` pointer, the contract on `SetAPIProcessCount` and `SignalPluginStartupComplete`, the stale `AddAPIProcessCount` comment
- `internal/component/bgp/reactor/reactor.go` - `startAPIServer` arms the barrier only under `!r.externalServer`, before the owned server starts; its doc comment and the `externalServer` field comment
- `internal/component/plugin/server/server.go` - the inline-signal comment, which claimed the channel is always created
- `internal/component/bgp/reactor/reactor_shutdown_ownership_test.go` - `newBorrowedPluginServer` comment: it passes a `reactorAPIAdapter`, not the hub's `Coordinator`; new helper `startBorrowedPeers`
- `internal/component/bgp/reactor/rfc7705_live_behavior_test.go` (`lowLiveRouter`) and `rfc9687_rib_release_test.go` - start peers through `startBorrowedPeers`
- `docs/architecture/api/architecture.md` - new section "The reactor's startup barrier"
- `plan/journal/new-caller-assumes-the-old-callers-timing.md` - the row's test names and its fix

## Files to Create
- `internal/component/bgp/reactor/reactor_borrow_startup_barrier_test.go` - `TestBorrowModeStartArmsNoStartupBarrier`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config or RPC change |
| YANG validation constraints | N-A | No config change |
| YANG custom validators | N-A | No config change |
| CLI commands/flags | N-A | No CLI change |
| CLI grammar (keyword before value) | N-A | No CLI change |
| Editor autocomplete | N-A | No CLI change |
| Functional test for new RPC/API | N-A | No new RPC |
| Pipe completeness | N-A | No command output |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Prometheus counters/metrics | N-A | No new metric; `ze_plugin_startup_seconds` and `ze_api_ready_seconds` are observed only on the standalone path, as before |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | `Reactor.startAPIServer` changes internal barrier ownership only |
| 2 | Config syntax changed? | No | No YANG or config parser change in 3c0cfa5fe6 |
| 3 | CLI command added/changed? | No | No command handler or grammar change |
| 4 | API/RPC added/changed? | No | Existing in-process signal methods retain their signatures, with ordering comments clarified |
| 5 | Plugin added/changed? | No | No plugin registration change. `Server.StartWithContext` has a comment-only edit |
| 6 | Has a user guide page? | No | The changed contract belongs to `docs/architecture/api/architecture.md`, not operator syntax |
| 7 | Wire format changed? | No | No message parser, encoder, or wire assertion changed |
| 8 | Plugin SDK/protocol changed? | No | `Server.signalStartupComplete` still freezes registries, sends post-startup callbacks, forwards completion, then closes `startupDone`. `process-protocol.md` remains accurate |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Live RFC tests retain their assertions. Existing owner approvals cover the two tagged-unit edits, separately from their proof refresh |
| 10 | Test infrastructure changed? | No | `startBorrowedPeers` is a local fixture helper, not a runner or test format change |
| 11 | Affects daemon comparison? | No | No capability or support-level change |
| 12 | Internal architecture changed? | Yes | 3c0cfa5fe6 added "The reactor's startup barrier" in `docs/architecture/api/architecture.md`. Closure narrows the no-channel statement to `startupComplete` and bounds coordinator timing |
| 13 | Route metadata keys added/changed? | No | No route metadata declaration or consumer change |
| 14 | Prometheus counters added/changed? | No | `StartWithContext` retains the same standalone-only startup metric observations |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration or inventory file in the implementation diff |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/` anchor search finds `api_sync.go`, `reactor.go`, and `server.go`. The startup contract is corrected here. `process-protocol.md` ownership and post-startup order remain unchanged |
| 17 | Existing docs show config/CLI/API examples for this area? | No | The changed section documents internal startup ownership and has no config, CLI, or RPC example to migrate |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- write `TestBorrowModeStartArmsNoStartupBarrier`, confirm it and the five tests red under `-race`
   - Tests: the five tests in the Task table, the new test
   - Files: the new test file
   - Verify: the race report names `SetAPIProcessCount` and `SignalPluginStartupComplete`
2. **Phase: Contract and fix** -- move `SetAPIProcessCount` into the standalone block of `startAPIServer`, state the contract in the doc comments and the architecture page, correct the journal row
   - Tests: AC-1 to AC-3
   - Files: from Files to Modify
   - Verify: the new test and the five tests clean under `-race -count=3`; the reactor package under `-race`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | No goroutine writes a sync field another may read without ordering; borrow mode never waits on its host |
| Naming | Doc comments on `SetAPIProcessCount` and `SignalPluginStartupComplete` state the contract |
| Data flow | Startup-sync state is created by one owner, at one point, before any signaller can reach it |
| Rule: `ai/rules/goroutine-lifecycle.md` | No new per-event goroutine |

### Deliverables Checklist
| Deliverable | Verification method | Evidence |
|-------------|---------------------|----------|
| Race-free start | `go test -race -run` over the five tests, under `./le job run` | Current Linux `-race -count=3`: all five pass three times, package `ok` in 16.100s. Full reactor `-race -count=1`: `ok` in 151.435s |
| Stated contract | Read architecture page and source anchors | `docs/architecture/api/architecture.md`, "The reactor's startup barrier", matches `Reactor.startAPIServer` and the `api_sync.go` producers |

### Security Review Checklist
| Check | What to look for | Result and source evidence |
|-------|-----------------|----------------------------|
| Resource exhaustion | A start that waits must keep its timeout and context-cancel exits | CLEAN: `WaitForAPIReady` retains `clock.After(apiTimeout)` and `ctx.Done`; `WaitForPluginStartupComplete` retains three API timeouts and `ctx.Done`. Borrow mode adds no wait. The test helper's server wait is bounded by ten seconds |
| Race and repeated signals | Concurrent startup must not rewrite state read by the server | CLEAN: `Reactor.startAPIServer` calls `SetAPIProcessCount` only before an owned server starts. `SignalPluginStartupComplete` retains `sync.Once` and its nil guard. Historical red and current Linux race runs discriminate this change |
| Hostile input and authority | No new unchecked input, allocation bound, injection path, or authorization bypass | CLEAN: the moved call consumes the existing configured plugin count. No parser, shell, path, cryptography, permission, or wire behavior changes. New test channels have fixed size and one owned signaller |

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

- The barrier (`startupComplete`, `apiReady` and their Onces) exists for one reader: a standalone reactor's `StartWithContext`, which waits on it before validating families and starting peers. Borrow mode skips both waits, so arming the barrier there created state nobody read, and it did so on the reactor's goroutine while the borrowed server's goroutines were already reading it.
- The barrier is armed by its only waiter, at the one point where no signaller exists yet: immediately before the owned server's `StartWithContext`. That gives the ordering the standalone path always had, now stated as the contract, and leaves borrow mode with nothing to order.
- In borrow mode `SignalPluginStartupComplete` finds no `startupComplete` channel and closes nothing. `AddAPIProcessCount` can still create `apiReady` and `SignalAPIReady` can close it. Both waits remain skipped. The closure corrects the original blanket claim that every signal was discarded.
- With the barrier fixed, the historical full-package race run exposed peer OPEN validation reading `Process.Registration` while startup wrote it through `Process.SetRegistration`. `startBorrowedPeers` preserves reactor-start overlap, then waits for `Server.WaitForStartupComplete` before `StartPeers`. The normal hub startup registers `StartPeers` through `Coordinator.OnPostStartup` after reactor start. This does not establish synchronization for plugin respawn, which remains an unverified lead in `plan/journal/false-synchronization-claim.md`.
- The research was exposed by 5050b3ec88 and 7e69a83522, which added the live borrow-mode tests; the production hub reaches the reactor through a `Coordinator` set before reactor start (A-1).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A: arm the barrier only where it is awaited, inside the standalone block before the owned server starts | B: guard the barrier fields with a mutex; C: create the barrier once in `New` and never re-create it; D: make borrow mode wait on the barrier | A removes the write that races and the state nobody reads, with one moved call. B adds a lock to protect state with no reader. C still needs `SetAPIProcessCount` to re-arm with the count at start, so the borrow-mode write stays unless it is also moved, which is A. D reintroduces the self-wait deadlock (R-1) |
| Contract home is `docs/architecture/api/architecture.md` | `core-design.md`, `process-protocol.md` | `ai/CODE-TO-DOCS.md` already maps `api_sync.go` there; `core-design.md` had no section to point at; `process-protocol.md` covers the plugin-side stages, not the reactor's waits |

## Known Limitations
- `AddAPIProcessCount` still writes `apiReady` lazily from the server's startup goroutine while `signalAllReady` may read it from a session goroutine. That is a server-internal ordering question, not the reactor-start race; it was not observed under `-race` (A-3) and is not changed here.

## RFC Documentation (Scope: protocol)

Scope is `plugin`, with no protocol code or wire assertion changes.
The live RFC test edits nevertheless require current proof records.

| Record | Closure status | Evidence |
|--------|----------------|----------|
| Owner approvals | Existing approvals retained, no new tagged-test edit in closure | 3c0cfa5fe6 carries `RFC-approved` trailers for `reactor.TestRFC7705MigrationWireSemantics` and `reactor.TestRFC9687Event29ReleasesTheLivePeersRIB`, quoting the owner's 2026-10-03 "Approve both" |
| Startup-owned RFC judgments | Independently rejudged and natively stamped `enforced` | `rfc/audit/rfc7705.json`: RFC7705-3.3-2 and RFC7705-4.2-4. `rfc/audit/rfc9687.json`: RFC9687-4.3-3. The first row also belongs to startup because the frozen diff formats `byte(113 + i)` in its tagged unit |
| Discrimination records | Five exact tuples renewed through native observed-red runs | `rfc/discrimination/rfc7705.json`: positive/negative for RFC7705-3.3-2 and RFC7705-4.2-4. `rfc/discrimination/rfc9687.json`: positive for RFC9687-4.3-3. Parent's `scratch/startup-discrimination-full.log` records every producer break and observed test failure |
| Record strength | Reachability breaks plus independent semantic judgment | The renewed breaks replace producer bodies with a panic. They prove that the tagged units reach those producers, not a particular semantic fault. Independent judgments examine the exact wire and live-state assertions |
| RFC check | 84 residual violations, none on the three owned rows | Parent's integrated `./le rfc check` output is retained in `scratch/startup-rfc-full.log`, down from 92 before the three judgments and five renewals |
| Other rows and public status | Unchanged | RFC7705-3.3-3, RFC9687-4.3-8, and RFC7947 residuals belong to other edits. No startup-owned SHIFTED rows exist. Parent ran `./le rfc index-update`; no tracked public support-level edit is owed |

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
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
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
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/immediate/spec-bgp-reactor-startup-race.md` only, in the same `./le commit create` script (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented

- Commit 3c0cfa5fe6 moves the only production call to `SetAPIProcessCount`
  into the standalone branch of `Reactor.startAPIServer`, before the owned
  server starts. Borrow-mode startup never rewrites `startupComplete`.
- `SetAPIProcessCount` and `SignalPluginStartupComplete` document their
  ordering contract. `api_sync.go` points to the durable API architecture page.
- `startBorrowedPeers` waits for the server's startup completion before it
  calls `StartPeers`. Both live fixtures retain their original peer-start
  error assertion and every RFC behavior assertion.
- `TestBorrowModeStartArmsNoStartupBarrier` drives early and repeated
  completion signals across `StartWithContext`. The historical run fails
  on the original race. Current Linux runs pass.

### Bugs Found/Fixed

| Defect | Producing function | Fix and regression evidence |
|--------|--------------------|-----------------------------|
| Borrow-mode startup rewrote channel and Once fields concurrently with completion signals | `internal/component/bgp/reactor/reactor.go`, `Reactor.startAPIServer`, calling `api_sync.go` `SetAPIProcessCount` | Standalone-only arming. Historical red reports `SetAPIProcessCount` against `SignalPluginStartupComplete`. The dedicated test and all five live tests now pass under `-race` |
| Live fixtures started peers while plugin registration was still being written | `lowLiveRouter` and `TestRFC9687Event29ReleasesTheLivePeersRIB` | `startBorrowedPeers` adds a bounded `Server.WaitForStartupComplete` before the unchanged `StartPeers` assertion. Live wire and RIB assertions remain |
| Architecture prose claimed every borrow-mode signal found no channel | `api_sync.go` `AddAPIProcessCount` and `SignalAPIReady` contradict that sentence | Narrowed the no-channel statement to `SignalPluginStartupComplete`. Readiness bookkeeping can still create and close `apiReady`, which borrow-mode startup never waits on |

### Documentation Updates

- `docs/architecture/api/architecture.md`, "The reactor's startup barrier":
  owner, arming order, timeout/cancel exits, nil startup-completion channel,
  separate readiness bookkeeping, and bounded coordinator evidence.
  Source anchors name `Reactor.startAPIServer`, `Reactor.StartWithContext`,
  the `api_sync.go` producers, `Coordinator.SetReactor`, its completion and
  callback methods, `runBGPEngine`, and both borrowed-server fixture helpers.
- `docs/architecture/api/process-protocol.md` remains unchanged: source
  inspection confirms registry freeze, post-startup notification, reactor
  notification, then `startupDone` closure in `Server.signalStartupComplete`.
  The constructor still owns the server's stop.
- The existing timing journal now retains all five exact test names, the fix
  commit, and the durable contract page. Historical bare spec stems remain.
- Parent ran `./le doc index write`, which regenerated ignored indexes, then
  `./le doc check verify`. The latter exits 1 on the same 29 previously
  recorded catalog drifts, with no startup-owned finding. This is not a
  full documentation-green claim.

### Deviations from Plan

- A-1 is broken as a claim of identical production timing. The fixture has
  an adapter from construction, whereas the coordinator forwards only after
  `SetReactor`. Production completion inside reactor startup was not reproduced.
  The fix remains valid without that timing assumption.
- A-3 is broken as a remaining reactor-start race: removing the arming call
  removes that reactor-side writer for both channel pairs. This does not
  prove all server-internal readiness or respawn synchronization.
- The peer-start helper was added after the historical full-package run
  exposed early OPEN validation. It waits after reactor start, preserving
  the overlap that the original race test must exercise.
- Full native Linux verification follows closure under the handover order.
  No worker reran builds, tests, lint, formatters, or native gates.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-1 treated fixture and hub signal timing as identical | The coordinator drops pre-attach reactor signals and forwards after attachment. This does not prove when completion arrives | Read `Coordinator.SetReactor`, its startup forwarding methods, and `runBGPEngine` | Marked broken and bounded the claim in this spec and the durable page |
| assumption | A-3 treated readiness fields as a remaining reactor-start writer pair | Borrow-mode startup no longer writes either pair through `SetAPIProcessCount` | Read `Reactor.startAPIServer`, `AddAPIProcessCount`, and all startup-state readers | Marked broken for this race, without claiming server-internal or respawn synchronization |
| approach | The new architecture paragraph generalized the startup-completion nil guard to every readiness call | Positive `AddAPIProcessCount` can allocate `apiReady` in either mode | Independent review of `api_sync.go` producers | Corrected the page before closing, with no code or test change |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Remove the reactor-start race and keep all five live tests passing | Done | `reactor.go` `Reactor.startAPIServer` | Current Linux acceptance run passes all five three times with no race report |
| State and test ownership and early-signal ordering | Done | `api_sync.go`, the API architecture page, and `TestBorrowModeStartArmsNoStartupBarrier` | Standalone arms before server start. Borrow-mode completion signals never gain a channel |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | All five named live tests pass three times in the current Linux `-race -count=3` run | No overlay. Package `ok` in 16.100s |
| AC-2 | Done | `TestBorrowModeStartArmsNoStartupBarrier`: historical race red, current Linux green three times | The nil-field assertion supplements the concurrent execution, rather than replacing it |
| AC-3 | Done | Current Linux passes the existing API readiness and startup lifecycle tests three times | Their assertions are unchanged. Source inspection confirms standalone call order and both timeout/cancel paths remain |
| AC-4 | Done | `api_sync.go` Design header and `docs/architecture/api/architecture.md`, "The reactor's startup barrier" | Review corrected the readiness overclaim and qualified the hub timing evidence |
| AC-5 | Done | `plan/journal/new-caller-assumes-the-old-callers-timing.md`, 2026-10-03 row | All five exact names, fixed status, 3c0cfa5fe6, and durable contract citation survive spec removal |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Five live regressions in the Task table | Done | `rfc7705_live_behavior_test.go`, `rfc7947_consumer_test.go`, `rfc9687_rib_release_test.go` | Historical red names all five. Current Linux count-three and full-package race runs pass |
| Borrow-mode concurrent startup regression | Done | `reactor_borrow_startup_barrier_test.go` `TestBorrowModeStartArmsNoStartupBarrier` | Startup returns successfully while the signaller runs. Historical race detector evidence supplies the discrimination |
| Existing readiness and startup lifecycle cases | Done | `api_sync_test.go`, `reactor_startup_test.go` | Count-three run passes readiness counts, timeout, immediate readiness, repeated waits, concurrency, mode, and failure cleanup cases |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/reactor/api_sync.go` | Done | Contract and durable Design header |
| `internal/component/bgp/reactor/reactor.go` | Done | Standalone-only barrier arming |
| `internal/component/plugin/server/server.go` | Done | Inline-completion comment corrected |
| `internal/component/bgp/reactor/reactor_shutdown_ownership_test.go` | Done | Adapter distinction and `startBorrowedPeers` |
| `internal/component/bgp/reactor/rfc7705_live_behavior_test.go` | Done | Helper call plus formatting, assertions preserved |
| `internal/component/bgp/reactor/rfc9687_rib_release_test.go` | Done | Helper call plus formatting, assertions preserved |
| `docs/architecture/api/architecture.md` | Done | Original contract plus closure's source-backed correction |
| `plan/journal/new-caller-assumes-the-old-callers-timing.md` | Done | Fixed row with all five test names |
| `internal/component/bgp/reactor/reactor_borrow_startup_barrier_test.go` | Done | New regression |

### Audit Summary

- Total items: 19 (2 task requirements, 5 acceptance criteria, 3 TDD groups,
  and 9 planned files).
- Done: 19.
- Partial: 0.
- Skipped: 0.
- Changed: 0. The bounded assumptions and added peer-start ordering are
  recorded under Deviations from Plan.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Race-free reactor start in borrow and standalone modes, with the five live tests passing | Race-instrumented live regression | Historical `job-race-red-a3ddac78.log` reports all five plus the dedicated borrow test failing, naming `SetAPIProcessCount` and `SignalPluginStartupComplete`. Current Linux `go test -race -v -count=3 -timeout 20m` with repository feature tags and the named startup/readiness selection returns `ok github.com/ze-software/ze/internal/component/bgp/reactor 16.100s`, 69 top-level PASS results plus nine migration subcases. Full-package `-race -count=1` returns `ok` in 151.435s |
| Explicit ownership and early-signal contract with a discriminating test | Producer review and concurrent lifecycle regression | `Reactor.startAPIServer` arms only the standalone server before `StartWithContext`. `SignalPluginStartupComplete` uses the unchanged Once/nil guard. `TestBorrowModeStartArmsNoStartupBarrier` fails on the original fields and passes three times on current Linux. Durable page source anchors describe the same contract |

The current daemon/external-plugin smoke `linklocal-raw-policy-sections`
passed one scenario with zero failures in 1.1s. It observed rewritten legacy
NLRI `18CB0071` and MP_UNREACH `800F0C0002014020010DB800070000` after live
policy callbacks, so the hub startup path reached peer traffic. This is
user-path evidence, not reproduction of A-1's proposed schedule. The D6 FRR
launch scenario also passed (one scenario, zero failures, 120.63s).

### Observed evidence provenance

| Evidence | Source and observed output | Limits |
|----------|----------------------------|--------|
| Historical red | Original session `ff3776cb`, `scratch/job-race-red-a3ddac78.log`: `WARNING: DATA RACE`, `SetAPIProcessCount` versus `SignalPluginStartupComplete`, six named failures, package `FAIL` in 5.692s | Historical pre-fix run, read during independent review |
| Historical green | Same session, `scratch/job-race-green5-d505bba9.log`: `PASS`, package `ok` in 21.950s | Used the implementation session's overlay for concurrent RIB changes. Current Linux results below supersede that verification limit |
| Current acceptance | Session `01a10694-3795-71f2-8250-25e3a877f4cf`, `scratch/job-startup-linux-race-67daf03c.log`: `--- PASS: TestBorrowModeStartArmsNoStartupBarrier` three times, each named live test three times, `PASS`, package `ok` in 16.100s | Parent ran with `CGO_ENABLED=1`, repository feature tags, `-race -v -count=3 -timeout 20m`, selected startup/readiness tests, current checkout on Colima Linux, no overlay |
| Current full reactor | Same session, `scratch/job-linklocal-linux-reactor-cgo-26ecb2d1.log`: dedicated startup test and all five live tests PASS, final `PASS`, package `ok` in 151.435s | Full reactor package with `-race -count=1`, current Linux checkout, no overlay. This is not full repository verification |
| Runtime smoke | Same session, `scratch/linklocal-runtime-observations.json`, `raw-policy-smoke.post-fix` and `postfix-frr` | Startup/user-path evidence from D6, not a race-detector run |
| Historical early-peer race | Original session `ff3776cb`, `scratch/job-reactor-pkg-c15e1e99.log`: `TestRFC7705NoPrependInstalledAndAdvertised` fails under `-race`, naming `Process.Registration` against `Process.SetRegistration` through startup handshake | This is the reproduced fixture startup race. It does not reproduce the separate respawn lead |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None within the startup-race acceptance criteria | AC-1 through AC-5 are satisfied. Full native verification follows closure under the handover order, with existing verification debt retained | N-A |

The separate `Process.Registration` respawn lead remains explicitly
unverified in `plan/journal/false-synchronization-claim.md`. No restart feature
or test weakening is included in this closure.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bgp-reactor-startup-race-01a10694-3795-71f2-8250-25e3a877f4cf.md`, natively recorded over 14 files, verdict CLEAN |
| `./le spec review check` | OK: 7 code files, clean, hashes match the native artifact |
| Rounds | 2: full frozen 3c0cfa5fe6 diff, then the documentation-only correction independently reviewed by Main |
| Reviewer lenses used | Logic and wiring, security and edge cases, test strength and documentation completeness. Style pass covered all seven changed Go files |
| Final source-review verdict | CLEAN: 0 BLOCKER, 0 ISSUE after the source-backed documentation correction |
| Automated pre-check scope | Repo check retains the two previously recorded numberparse findings; startup closure changes no Go. Documentation check retains 29 catalog drifts. Integrated RFC check retains 84 out-of-scope violations and clears all three startup-owned rows. Full native Linux verification follows closure |

### Run 1 scope and results

The frozen diff contains 13 files, 301 insertions, and 96 deletions. Most
changes are the regression, contract, history, and formatting of existing
tests. The production fix moves one call into an existing ownership branch.
The full review found no unwired production symbol, removed guard, lost
assertion, new input boundary, hot-path allocation, or peer-reachable panic.

`lowLiveRouter` and the RFC9687 test still start the reactor before waiting
for the server. The added wait therefore preserves the original startup
overlap. The old `StartPeers` error assertion moves into `startBorrowedPeers`
and still runs. All wire and live-RIB assertions remain.

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The borrow-mode paragraph claimed all readiness calls found no channel and were discarded, contrary to positive `AddAPIProcessCount` and `SignalAPIReady` | `docs/architecture/api/architecture.md`, reactor startup barrier | Narrowed the statement to `startupComplete`, documented separate readiness bookkeeping, and preserved bounded wait behavior. Main independently verified the correction against producers |

### Diagnosis and correction

The symptom was a false architecture contract. The producing functions are
`AddAPIProcessCount`, which allocates `apiReady` on a positive count, and
`SignalAPIReady`, which can close it through `signalAllReady`. Documentation
owns the mismatch. A [workaround] would suppress these calls in borrow mode
to make the sentence true, adding an unrequested behavior change. The [source]
fix corrects the sentence and adds producer anchors. No new test is warranted
for that prose correction because the runtime behavior did not change.

### Run 2 scope and result

Main independently reviewed only the corrected startup paragraph against
`api_sync.go`, `Reactor.startAPIServer`, `Coordinator`, and `runBGPEngine`.
Result: CLEAN. Closure-record clarifications do not open another code round.

### Non-blocking observations

- The dedicated test's `startupComplete == nil` assertion pins an implementation
  choice. Its useful regression is the concurrent signal loop, successful
  startup return, and historical race red. The test is not solely a field/default
  assertion or a source-text test. No incidental assertion or new test was added.
- `TestExternalServerDerivedFromMode` is an existing internal-field assertion.
  The neighboring behavioral cases still exercise missing-server rejection and
  standalone server construction. Those tests were not changed.
- The style pass found no added panic, unbounded production loop, hot-path
  copy/allocation, widened return type, or new production lifecycle. The test's
  signaller has a stop channel and is joined after startup.

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/reactor/reactor_borrow_startup_barrier_test.go` | Yes | Read `TestBorrowModeStartArmsNoStartupBarrier` from source. Current Linux count-three output names it three times as PASS |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Five live tests are race-clean, three runs each | Current Linux acceptance output retains all five named PASS lines for each iteration, final package `ok` in 16.100s |
| AC-2 | Borrow startup tolerates early/repeated concurrent completion signals | Dedicated test PASS three times. Historical red contains the exact writer/reader race and the non-nil channel failure |
| AC-3 | Existing standalone/readiness behavior remains | Existing selected API-sync and lifecycle tests PASS three times. `Reactor.StartWithContext` still calls both waits only for standalone, after its server starts |
| AC-4 | Durable contract names owner, order, and early-signal behavior | Read `api_sync.go` Design header and source-backed architecture section. Main independently reviewed the corrected prose |
| AC-5 | Journal survives closure with exact names and fix | Existing row contains all five Task names, fixed commit 3c0cfa5fe6, and the durable architecture page |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Borrowed server -> reactor startup -> live peer protocol traffic | N-A for the race-detector regression. Existing live Go fixtures are `lowLiveRouter` and `TestRFC9687Event29ReleasesTheLivePeersRIB` | The source starts the server, injects it, starts the reactor, waits for server startup, then sends and observes peer messages and RIB state. Current Linux runs pass |
| Hub daemon -> BGP plugin -> external policy callback -> peer UPDATE | Session's `linklocal-raw-policy-sections` runtime smoke | One passed scenario, zero failures, observed rewritten legacy NLRI and MP_UNREACH. Full gating suite follows closure |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | Fixture adapter forwards from construction. `Coordinator` forwards after `SetReactor`, which `runBGPEngine` calls before reactor start. Completion inside that interval is unproven |
| A-2 | confirmed | Historical red names both exact rfc7947 tests, each failing with the startup race |
| A-3 | broken | Borrow-mode `startAPIServer` no longer calls `SetAPIProcessCount`, so it writes neither startup channel pair. This does not certify server-internal lazy-readiness ordering |
| A-4 | confirmed | Old `SetAPIProcessCount` reset the early completion state. Current borrow start leaves `startupComplete` nil and repeated completion signals harmless |
| A-5 | confirmed | Startup-state reads remain in `api_sync.go`. The only production waits are inside the standalone guard in `Reactor.StartWithContext`. Adapter and coordinator methods forward without waiting |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Reactor startup barrier and readiness distinction | `Reactor.startAPIServer`, `SetAPIProcessCount`, `AddAPIProcessCount`, `SignalAPIReady`, `SignalPluginStartupComplete` | Yes, source reviewed and corrected prose independently reviewed |
| Hub forwarding and deferred peer startup | `Coordinator.SetReactor`, startup forwarding methods, `runBGPEngine` | Yes, bounded to attachment and callback ordering without claiming the test's schedule occurs in production |
| Process protocol and ownership stay unchanged | `Server.signalStartupComplete`, `Server.WaitForStartupComplete`, `Reactor.startAPIServer`, and the unchanged borrow-mode cleanup guard | Yes, no protocol-stage or server-ownership change |
| No config, CLI, wire, feature-list, comparison, inventory, metadata, or metrics update | Full implementation diff and `docs/` source-anchor search for every changed Go file | Yes, no corresponding surface changed |
| Documentation gate | Parent's `./le doc check verify`, recorded in `scratch/startup-doc-check.log` | Exit 1: 29 known catalog drifts, no startup-owned finding. No claim of repository-wide green |

## Core Insight

The reactor must create a barrier only when it owns the waiter and can order
its creation before the signaller starts. That contract is retained in
`docs/architecture/api/architecture.md`, "The reactor's startup barrier".

### Citation disposition and closure manifest

| Citer | Disposition |
|-------|-------------|
| `api_sync.go` and `reactor_borrow_startup_barrier_test.go` Design headers | Already point to `docs/architecture/api/architecture.md`, which retains the source-backed contract |
| Dedicated test's VALIDATES comment | Bare `spec-bgp-reactor-startup-race` provenance, no resolvable file promise |
| Timing and false-synchronization journal rows | Bare spec stems retained. Timing row now names all five tests and points to the durable page |
| Historical handover | Bare spec stem and historical session-state provenance retained without claiming a live spec path |
| Full-path startup-spec references | Source scan found only this spec's own Commit B instruction, removed with the file. No sibling path or Design header remains to repoint |
| Citation baseline | Unchanged. No citation was hidden in `plan/.citation-baseline` |

Commit A contains exactly these seven canonical files. The implementation
already exists in 3c0cfa5fe6:

| File | Owner and purpose |
|------|-------------------|
| `docs/architecture/api/architecture.md` | StartupReviewClosure: corrected durable contract |
| `plan/immediate/spec-bgp-reactor-startup-race.md` | StartupReviewClosure: completed spec and evidence, retained in history before removal |
| `plan/journal/new-caller-assumes-the-old-callers-timing.md` | StartupReviewClosure: all five test names and durable contract citation |
| `rfc/audit/rfc7705.json` | Parent native stamp after independent RFC review: two startup-owned judgments |
| `rfc/audit/rfc9687.json` | Parent native stamp after independent RFC review: one startup-owned judgment |
| `rfc/discrimination/rfc7705.json` | Parent native renewal: four exact tuples |
| `rfc/discrimination/rfc9687.json` | Parent native renewal: one exact tuple |

Commit B removes only this spec. The parent owns the native review artifact,
claim release, generated two-commit script, and execution. Ignored generated
indexes, scratch logs, `go.mod`, `go.sum`, and the unrelated enum spec are
excluded. Full native Linux verification follows both closure commits.

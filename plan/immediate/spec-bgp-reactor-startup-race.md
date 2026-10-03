# Spec: bgp-reactor-startup-race

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 2/2 |
| Handoff | - |
| Updated | 2026-10-03 |

Bucket: `plan/immediate/`, because an operator running the BGP reactor under a
borrowed plugin server, which is the production shape, can meet it.

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

The journal row names the two rfc7947 tests only as "two rfc7947 ones". The
two above are the only rfc7947 tests that drive `lowLiveRouter`; that mapping
was made by reading the tests, not by a race run, and the design phase confirms
it with a scoped `go test -race -run` before relying on it.

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
- `reactorAPIAdapter.SignalPluginStartupComplete` (`internal/component/bgp/reactor/reactor_api.go`) - the server's only route into the reactor's startup state
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
| A-1 | The production hub path has the same unordered interleaving as the test helper, not only the tests | `newBorrowedPluginServer` comment says it mirrors `cmd/ze/hub/main.go`; the BGP engine runs as a plugin of the already-started server | The defect is test-only and the bucket moves to `plan/` | Read the hub start path and the stage at which the BGP plugin starts its reactor | partly confirmed: the hub passes a `plugin.Coordinator` (`cmd/ze/hub/main.go`, `NewServer(serverConfig, coordinator)`), and the bgp plugin calls `Coordinator.SetReactor` before `StartWithContext` (`internal/component/bgp/plugin/register.go`), so from that point every server signal reaches the reactor while it starts. Whether a signal can arrive in that window in production depends on the stage barrier and is not shown; the fix removes the write either way |
| A-2 | The two rfc7947 tests in the journal row are `TestRFC7947AllAttributesReachClient` and `TestRouteServerTransparencyStopsAtOrdinaryPeer` | They are the only rfc7947 tests that call `lowLiveRouter` | The test table names the wrong tests | Scoped `go test -race -run` over the reactor package | confirmed: both red under `-race` in the red run, each report naming `SetAPIProcessCount` at `reactor.go` `startAPIServer` |
| A-3 | The `apiReady` / `apiReadyOnce` pair carries the same race shape in borrow mode | `SetAPIProcessCount` and `AddAPIProcessCount` assign them unguarded while `SignalAPIReady` runs from the server | The fix covers only the startup pair and the next race report names `apiReady` | `-race` run with a config that sets explicit plugins under a borrowed server | broken for the reactor-start race: with the fix, borrow mode never calls `SetAPIProcessCount`, so the reactor's goroutine writes neither field. The remaining lazy write in `AddAPIProcessCount` is between server goroutines, and the five live tests, which host explicit plugins under a borrowed server, report no race on it under `-race -count=3` |
| A-4 | A signal that arrives before `SetAPIProcessCount` is lost today, because the re-created channel is never closed | `SetAPIProcessCount` overwrites a channel the server may already have closed | No functional consequence in borrow mode (nobody waits), but the contract must say so | Read and test at design | confirmed by reading: the overwrite replaced a closed channel with an open one and a fresh Once. After the fix no borrow-mode overwrite exists, and the page states that borrow-mode signals close nothing |
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
| 1 | Operator starts the daemon with BGP configured | hub starts plugin server -> BGP plugin starts reactor -> server signals startup complete | The five live reactor tests: each starts a real plugin server hosting real internal plugins, injects it into a borrow-mode reactor through `SetPluginServer`, and drives peers over TCP. They reach the reactor through a `reactorAPIAdapter`, not the hub's `Coordinator`, which forwards the same three calls once `SetReactor` has run; the existing gating functional suite covers the hub binary itself |

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
| N-A: no new functional test. The defect is a memory-ordering fault visible only to the race detector, and the shipped binary is not race-enabled; the gating functional suite already starts the daemon with BGP and reaches peer start, and it is owed to the main thread | `test/` | Daemon with BGP starts and reaches peer start | owed |

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
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | The startup stage order is unchanged; `process-protocol.md` needs no edit |
| 9 | RFC behavior implemented, changed, or newly proven? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/architecture.md`, "The reactor's startup barrier" (core-design.md had no section to amend; the dangling `// Design:` pointer now names this page) |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | To be derived at design with `./le spec citation anchors` | - |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

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
| Deliverable | Verification method |
|-------------|---------------------|
| Race-free start | `go test -race -run` over the five tests, under `./le job run` |
| Stated contract | grep the architecture page for the source anchor |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | A start that waits must keep its timeout and context-cancel exits |

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
- In borrow mode the server's signals find no channel and close nothing; `SignalPluginStartupComplete`'s nil check already handles it, so no new code path exists.
- With the barrier fixed, a full `-race` run of the package showed a second race in `TestRFC7705NoPrependInstalledAndAdvertised`: a peer's OPEN validation (`Server.PluginsWithPerPeerOpenPolicy` reading `Process.Registration`) against the startup handshake (`Process.SetRegistration`). The live tests called `StartPeers` straight after reactor start, while the borrowed server was still handshaking. The daemon never does that: the bgp plugin registers `StartPeers` as the coordinator's post-startup callback (`internal/component/bgp/plugin/register.go`), which runs from `SignalPluginStartupComplete`. The tests now start peers through `startBorrowedPeers`, which waits on `Server.WaitForStartupComplete` first, so they keep the reactor-start overlap the barrier fix needs and drop the peer-start ordering production cannot produce.
- The research was exposed by 5050b3ec88 and 7e69a83522, which added the live borrow-mode tests; the production hub reaches the reactor through a `Coordinator` set before reactor start (A-1).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A: arm the barrier only where it is awaited, inside the standalone block before the owned server starts | B: guard the barrier fields with a mutex; C: create the barrier once in `New` and never re-create it; D: make borrow mode wait on the barrier | A removes the write that races and the state nobody reads, with one moved call. B adds a lock to protect state with no reader. C still needs `SetAPIProcessCount` to re-arm with the count at start, so the borrow-mode write stays unless it is also moved, which is A. D reintroduces the self-wait deadlock (R-1) |
| Contract home is `docs/architecture/api/architecture.md` | `core-design.md`, `process-protocol.md` | `ai/CODE-TO-DOCS.md` already maps `api_sync.go` there; `core-design.md` had no section to point at; `process-protocol.md` covers the plugin-side stages, not the reactor's waits |

## Known Limitations
- `AddAPIProcessCount` still writes `apiReady` lazily from the server's startup goroutine while `signalAllReady` may read it from a session goroutine. That is a server-internal ordering question, not the reactor-start race; it was not observed under `-race` (A-3) and is not changed here.

## RFC Documentation (Scope: protocol)

N-A: Scope is `plugin`; no protocol code changes.

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

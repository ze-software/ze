# Spec: bgp-reactor-startup-race

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | - |
| Phase | - |
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
- [ ] `docs/architecture/api/process-protocol.md` - the 5-stage startup protocol and "Who stops the plugin server"
  → Decision: whoever constructs the plugin server owns it; the reactor borrows it through `registry.SetPluginServer` and `Reactor.SetPluginServerAny`, and a borrowed server is read, never stopped
  → Constraint: `sendPostStartupToAll` runs after `signalStartupComplete` freezes the command registry; any reordering of the reactor signal must keep that order
- [ ] `docs/architecture/api/architecture.md` - listed for `api_sync.go` in `ai/CODE-TO-DOCS.md`

### RFC Summaries (Scope: protocol)
- N-A: Scope is `plugin`; no RFC behavior changes. The failing tests are tagged for RFC 7705, 7947 and 9687 only because they drive a live reactor.

**Key insights:** (minimal context to resume after compaction)
- `SetAPIProcessCount` is called on every reactor start, in both modes, after the server already exists in borrow mode.
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
| Plugin server ↔ BGP reactor | `reactorAPIAdapter` method calls across goroutines | No |

### Integration Points
- `reactorAPIAdapter.SignalPluginStartupComplete` (`internal/component/bgp/reactor/reactor_api.go`) - the server's only route into the reactor's startup state
- `SetPluginServer` / `SetPluginServerAny` - how a borrowed server reaches the reactor

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to be answered at design |
| No unintended coupling (components stay isolated) | No | to be answered at design |
| No duplicated functionality (extends existing, does not recreate) | No | to be answered at design |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A expected: no buffers involved; confirm at design |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | to be answered at design |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | to be answered at design |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The production hub path has the same unordered interleaving as the test helper, not only the tests | `newBorrowedPluginServer` comment says it mirrors `cmd/ze/hub/main.go`; the BGP engine runs as a plugin of the already-started server | The defect is test-only and the bucket moves to `plan/` | Read the hub start path and the stage at which the BGP plugin starts its reactor | unvalidated |
| A-2 | The two rfc7947 tests in the journal row are `TestRFC7947AllAttributesReachClient` and `TestRouteServerTransparencyStopsAtOrdinaryPeer` | They are the only rfc7947 tests that call `lowLiveRouter` | The test table names the wrong tests | Scoped `go test -race -run` over the reactor package | unvalidated |
| A-3 | The `apiReady` / `apiReadyOnce` pair carries the same race shape in borrow mode | `SetAPIProcessCount` and `AddAPIProcessCount` assign them unguarded while `SignalAPIReady` runs from the server | The fix covers only the startup pair and the next race report names `apiReady` | `-race` run with a config that sets explicit plugins under a borrowed server | unvalidated |
| A-4 | A signal that arrives before `SetAPIProcessCount` is lost today, because the re-created channel is never closed | `SetAPIProcessCount` overwrites a channel the server may already have closed | No functional consequence in borrow mode (nobody waits), but the contract must say so | Read and test at design | unvalidated |

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
| Borrowed server started, then reactor `StartWithContext` | → | `SetAPIProcessCount` and `SignalPluginStartupComplete` | `TestRFC7705NoPrependInstalledAndAdvertised` under `-race` (existing, red today); a dedicated startup-ordering test is named at design |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The five tests in the Task table run under `go test -race` | All pass, and the race detector reports nothing |
| AC-2 | Borrowed server finishes plugin startup before the reactor starts | Reactor start returns, no race, no panic |
| AC-3 | Reactor starts before the borrowed server signals startup complete | Reactor start returns, the later signal does not race or panic |
| AC-4 | Standalone reactor with auto-load only config | Start still waits for Phase 1 and Phase 2 before validating |
| AC-5 | `SignalPluginStartupComplete` called twice (reload) | Second call is a no-op |
| AC-6 | A reader of the architecture page asks who creates the startup-sync state and when a signal may arrive | The page answers, with a source anchor to the producing function |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator starts the daemon with BGP configured | hub starts plugin server -> BGP plugin starts reactor -> server signals startup complete | To be named at design: a functional `.ci` start under the race-enabled binary, or the live reactor tests if the design shows they cover the hub path |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| The five tests in the Task table, under `-race` | `internal/component/bgp/reactor/` | AC-1 | red today |
| To be named at design: signal before start, signal after start, double signal | `internal/component/bgp/reactor/api_sync_test.go` or a new file | AC-2, AC-3, AC-5 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| API process count | 0 and above | to be stated at design | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| To be named at design | `test/plugin/` | Daemon with BGP starts and reaches peer start | |

### Interop Tests (Scope: protocol)
N-A: Scope is `plugin` and no wire-visible behavior changes.

## Files to Modify
- `internal/component/bgp/reactor/api_sync.go` - the startup-sync state and its initialization
- `internal/component/bgp/reactor/reactor.go` - the API setup call order in `StartWithContext`
- `internal/component/plugin/server/server.go` - the inline signal and its comment, if the contract moves creation
- `docs/architecture/core-design.md` - the startup ordering contract

## Files to Create
- To be decided at design: a startup-ordering test file in `internal/component/bgp/reactor/`

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
| Prometheus counters/metrics | N-A | No new metric expected; confirm at design |
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
| 8 | Plugin SDK/protocol changed? | To be answered at design | `docs/architecture/api/process-protocol.md` if the startup stage order is restated |
| 9 | RFC behavior implemented, changed, or newly proven? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | To be derived at design with `./le spec citation anchors` | - |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- confirm the five tests red under `-race` and name the ordering tests
   - Tests: the five tests in the Task table
   - Files: none changed
   - Verify: the race report names `SetAPIProcessCount` and `SignalPluginStartupComplete`
2. **Phase: Contract and fix** -- to be designed: state the ordering, then make the sync state safe under it
   - Tests: AC-2 to AC-5 tests
   - Files: from Files to Modify
   - Verify: tests fail, implement, tests pass, five tests clean under `-race`

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

- Skeleton: no design yet. The research that fills this section starts from A-1 to A-4.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| To be decided at design | - | - |

## Known Limitations
- Skeleton: none decided yet.

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

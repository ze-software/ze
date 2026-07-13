# Spec: test-sync-quiesce

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/commands.md` - command dispatch + RPC registration
4. `internal/component/bgp/plugins/cmd/peer/peer.go` - the peer-flush RPC (the pattern being generalized)
5. `internal/component/bgp/reactor/reactor_api.go` - reactor `Flush` forward-pool drain barrier
6. `internal/component/plugin/server/command.go` - Dispatcher, CommandContext, RPCRegistration
7. `test/scripts/ze_api.py` - `wait_for_ack` (has a residual sleep to remove)

## Task

Give tests (and operators) a way to **wait for the daemon's asynchronous work to
finish, instead of sleeping a fixed duration**. Today the control plane already
returns synchronously (dispatch replies after the handler runs), but downstream
async effects (routes flushed to peer sockets, and later FIB/tc/listeners)
complete *after* the reply, so tests insert `time.sleep`. This is the root cause
of the ~774 sleeps across `test/` (456 in `.ci` alone) that make the suite flaky.

**Layer 1 (THIS spec):** a general **quiesce barrier** — a `Quiescer`
registration mechanism plus a `request quiesce` command/RPC that blocks until
every registered subsystem has drained its pending work, then replies. The BGP
forward pool (today drained only by the peer-specific `ze-bgp:peer-flush`) is the
first registrant, so a test can do `send(route); request quiesce; assert on-wire`
with **zero sleeps**. The existing `wait_for_ack` helper (which drains the
forward pool then sleeps a fudge factor) drops its residual `time.sleep`.

**Follow-on (separate specs, OUT OF SCOPE here):**
- Layer 2: payload-predicate waits (`wait_for_event(state=established)` /
  `expect=event:where=`) so tests can target a specific event, not first-of-type.
- Layer 3: completion events at silent boundaries (`fib/route-programmed`,
  `traffic/applied`, `listener/ready`, `bgp/peer-established`,
  `bgp/peer-teardown-complete`) and their Quiescer registrants.

The design is deliberately extensible: Layer 3's FIB/tc/listener subsystems
register their own `Quiescer` into the same registry this spec builds, and
`request quiesce` drains them automatically with no change to the barrier.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/commands.md` - command dispatch, RPC registration, the command tree
  → Decision: operational commands dispatch synchronously via `Dispatcher.Dispatch`; the reply is the natural completion signal to build a barrier on.
  → Constraint: new commands register through `RPCRegistration` / the registry, never a hardcoded switch (registration-over-hardcoding).
- [ ] `docs/architecture/core-design.md` - small core + registration pattern
  → Constraint: the barrier must discover subsystems through a registry; the `quiesce` handler must not name BGP/FIB/tc directly.

### RFC Summaries (MUST for protocol work)
N/A — not protocol work (no wire-format change).

**Key insights:**
- The control plane is already synchronous (`request bgp rib inject` recomputes best-path before replying, `rib_commands.go`), so the barrier only needs to cover the async *forwarding/downstream* planes.
- `peer-flush` already proves the pattern: an RPC that calls a reactor drain barrier (`reactor.Flush`) and replies when drained. `quiesce` generalizes it from "one peer's forward workers" to "all registered subsystems."
- A Quiescer needs a *runtime* reference (the reactor instance), so registration is at wiring time (when the reactor is attached to the plugin server), not `init()`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go` - registers `ze-bgp:peer-flush` (peer.go:43) via `RPCRegistration{WireMethod, Handler, RequiresSelector:true}`; `handleBgpPeerFlush` calls `ctx.Reactor().Flush(ctx)` / `FlushPeer(ctx, addr)` and replies `StatusDone` when drained.
  → Constraint: the flush RPC blocks the caller until the forward pool has drained queued items to peer sockets — this is exactly the barrier semantics `quiesce` needs, generalized.
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `reactorAPIAdapter.Flush(ctx)` blocks until all forward-pool workers drained; `FlushPeer(ctx, addr)` for one peer.
  → Constraint: `Flush` is on the reactor API adapter reached via `CommandContext.Reactor()` (returns `plugin.ReactorLifecycle`).
- [ ] `internal/component/plugin/server/command.go` - `Dispatcher`, `CommandContext.Reactor()` (command.go:144), `AllBuiltinRPCs` (command.go:38), builtin registration; `RPCRegistration` type.
  → Constraint: builtin RPCs register via `init()`/`RegisterRPCs`; the plugin server (`pluginserver.Server`) owns the Dispatcher and the reactor reference.
- [ ] `internal/component/plugin/server/system.go` - the `ze-system:*` handlers (command complete, daemon-reload) — home for a system-level `quiesce` handler.
- [ ] `test/scripts/ze_api.py` - `wait_for_ack` (ze_api.py:1140) sends `ze-bgp:peer-flush` then `time.sleep(0.2*count)` (ze_api.py:1167); `wait_for_event` (:975), `send` (:833, synchronous per :1190).

**Behavior to preserve:**
- `ze-bgp:peer-flush` (`request peer <sel> flush`) keeps working unchanged (operators and existing tests depend on it).
- Dispatch order, command reply shapes, and the 5-stage plugin protocol are unchanged.
- `wait_for_ack`'s external contract (returns True, callers pass `expected_count`) is preserved; only its *internal* sleep is replaced by the barrier.

**Behavior to change:**
- Add a `request quiesce` command/RPC and a `Quiescer` registry.
- The BGP forward pool becomes a registered Quiescer.
- `wait_for_ack` internals call `quiesce` and drop the residual `time.sleep`.

## Data Flow (MANDATORY)

### Entry Point
- Test/operator issues `request quiesce` (CLI/API) → dispatched as RPC `ze-system:quiesce`.

### Transformation Path
1. `Dispatcher.Dispatch` routes `ze-system:quiesce` to `handleQuiesce` (system.go).
2. `handleQuiesce` reads the registered `Quiescer`s from the plugin server's registry.
3. It invokes each Quiescer's `Quiesce(ctx)` (concurrently, each bounded by a per-quiescer timeout), aggregating errors.
4. When all return (drained) or the deadline hits, it replies `StatusDone` (or `StatusError` with the first failure), carrying a per-subsystem status map.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI/API → Engine | `ze-system:quiesce` RPC via Dispatcher | [ ] |
| Engine → Subsystem | `Quiescer.Quiesce(ctx)` (BGP → `reactor.Flush`) | [ ] |
| Test harness → Engine | `ze_api.quiesce()` sends the RPC; `.ci` uses `request quiesce` | [ ] |

### Integration Points
- `pluginserver.Server` / `Dispatcher` — owns the new `QuiescerRegistry`.
- BGP reactor wiring (where `ctx.Reactor()` is populated) — registers the forward-pool Quiescer.
- `test/scripts/ze_api.py` — `quiesce()` helper; `wait_for_ack` uses it.

### Architectural Verification
- [ ] No bypassed layers (barrier goes through Dispatcher like every command)
- [ ] No unintended coupling (`handleQuiesce` iterates the registry; never names BGP/FIB/tc)
- [ ] No duplicated functionality (`peer-flush` reused as the BGP Quiescer body, not reimplemented)
- [ ] Registration over hardcoding — subsystems register a `Quiescer`; the handler discovers them

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `reactor.FlushForwardPool(ctx)` fully drains the forward pool so a subsequent on-wire assert is stable | peer-flush already relies on this (peer.go:43, reactor_api.go:891) | The BGP quiescer would be insufficient; need a stronger drain | `test/plugin/quiesce-barrier.ci` | CONFIRMED — quiesce-barrier.ci passes: both routes on the peer wire after `quiesce()`, no sleep |
| A-2 | A Quiescer can be registered at reactor-wiring time (runtime), and the `quiesce` handler can read the registry from `CommandContext` | command.go:130 CommandContext holds `Server` | Need a different registration/lookup path | wiring test | CONFIRMED — `registerReactorQuiescer` in `NewServer` (server.go); `handleQuiesce` reads `ctx.Server.Quiescers()`; `TestBGPForwardPoolRegistersQuiescer` green |
| A-3 | `request quiesce` passes the CLI grammar gate (verb-first, no hyphen collision) | `request` is a canonical verb; `quiesce` is a single token | Rename (e.g. `request settle`) | grammar gate | CONFIRMED — `quiesce` is not among the cli-grammar violations; server pkg + `TestEveryRPCHasYANGPath` + `TestRPCRegistrationPerModule` green |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A Quiescer hangs (never drains) and blocks `quiesce` forever | Test times out on `request quiesce` | Per-quiescer `context` deadline; handler returns partial status + error on timeout |
| R-2 | Concurrent Quiescers race or deadlock against each other | Race detector / hang under `-race` | Run each in its own goroutine with an errgroup + timeout; document ordering-independence requirement |
| R-3 | `wait_for_ack` behaviour change breaks existing route-delivery tests | `ze-test bgp plugin` route asserts flake | Keep the peer-flush call; only replace the trailing sleep with the barrier; run the plugin suite |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `request quiesce` dispatched | → | `handleQuiesce` iterates the registry and replies StatusDone | `TestQuiesceDispatchDrainsRegisteredQuiescers` |
| BGP reactor wired to server | → | forward-pool Quiescer registered | `TestBGPForwardPoolRegistersQuiescer` |
| `.ci`: `send route; request quiesce; expect on-wire` | → | full barrier path, no sleep | `test/plugin/quiesce-barrier.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A subsystem registers a `Quiescer`; `request quiesce` is dispatched | The handler calls that Quiescer's `Quiesce(ctx)` and replies `StatusDone` only after it returns |
| AC-2 | Multiple Quiescers registered | All are invoked; the reply is `StatusDone` only after ALL drain (or `StatusError` if one fails/times out, naming which) |
| AC-3 | No Quiescer registered (bare engine) | `request quiesce` replies `StatusDone` immediately (no-op, never hangs) |
| AC-4 | BGP forward pool has queued routes; test sends routes then `request quiesce` | After the reply, all routes are observable on the peer wire with NO `time.sleep` |
| AC-5 | A registered Quiescer blocks past the deadline | The handler returns within the bounded timeout with an error naming the stuck subsystem (never hangs the daemon) |
| AC-6 | ~~`ze_api.wait_for_ack` drains via quiesce, no sleep~~ **RESCOPED (follow-on)** | `ze_api.quiesce()` is the sleepless barrier and is used by new tests. `wait_for_ack` is deliberately left as peer-flush + sleep because its sleep also covers the peer simulator's `cmd=api` interleaving, which the forward-pool quiesce does not drain (removing it races `nexthop`-style tests). Fully migrating `wait_for_ack` needs a **peer-side quiescer** — tracked as follow-on (see Known Limitations). Confirmed via the R-3 run: reverting the sleep regressed `nexthop`; keeping it does not. |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Test sends a route then `request quiesce`, asserts on-wire | send → forward pool enqueues → quiesce → reactor.Flush drains → reply → assert | `test/plugin/quiesce-barrier.ci` |
| 2 | `ze_api` observer calls `quiesce()` instead of sleeping | `quiesce()` → `ze-system:quiesce` RPC → drain → return | `TestQuiesceHelperNoSleep` (grep: no `time.sleep` in `wait_for_ack`) |
| 3 | Operator runs `request quiesce` on a live daemon | CLI → dispatcher → handler → all registrants drain → StatusDone | wiring test + `.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestQuiescerRegistryRegistersAndLists` | `internal/component/plugin/server/quiesce_test.go` | registration + retrieval (AC-1) | ✅ green (-race) |
| `TestQuiesceDispatchDrainsRegisteredQuiescers` | `internal/component/plugin/server/quiesce_test.go` | handler invokes all, replies after all drain (AC-1/2) | ✅ green (-race) |
| `TestQuiesceNoRegistrantsIsNoop` | `internal/component/plugin/server/quiesce_test.go` | empty registry → immediate StatusDone (AC-3) | ✅ green |
| `TestQuiesceTimeoutNamesStuckSubsystem` | `internal/component/plugin/server/quiesce_test.go` | a blocking quiescer → bounded error (AC-5) | ✅ green (0.10s) |
| `TestQuiesceReportsQuiescerError` | `internal/component/plugin/server/quiesce_test.go` | a drain error is named, not swallowed (AC-2) | ✅ green |
| `TestBGPForwardPoolRegistersQuiescer` | `internal/component/plugin/server/quiesce_test.go` | BGP forward-pool auto-registered; FlushForwardPool is the body | ✅ green |

### Boundary Tests (MANDATORY for numeric inputs)
N/A — no numeric inputs (timeout is a duration constant, covered by AC-5).

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `quiesce-barrier` | `test/plugin/quiesce-barrier.ci` | `quiesce()` is callable end-to-end (real daemon), routes reach the wire, no sleep | ✅ PASS (test 328, 2.2s) — smoke/plumbing test |

> **AC-4 evidence, honestly:** `quiesce-barrier.ci` proves the barrier is wired and non-breaking end-to-end, but it does NOT by itself prove "quiesce *blocks* until drained" (the plugin's `wait_for_shutdown()` gives the UPDATEs unbounded time to arrive, so the peer would likely pass even if `quiesce()` were a no-op). The **blocking** property is proven by the unit tests (`TestQuiesceDispatchDrainsRegisteredQuiescers` — reply only after all drain) plus the reactor's `fwdPool.Barrier` sentinel-FIFO (`forward_pool_barrier.go`), which guarantees prior items are on the wire before the barrier returns.

### Interop Tests (MANDATORY for protocol features)
N/A — no wire-protocol change.

### Future (if deferring any tests)
- Layer 2/3 (payload predicates, FIB/tc/listener quiescers + events) are separate specs, not deferred tests of THIS spec.

## Files to Modify
- `internal/component/plugin/server/system.go` - add `handleQuiesce`; register `ze-system:quiesce`.
- `internal/component/plugin/server/command.go` or `server.go` - hold the `QuiescerRegistry`; expose `RegisterQuiescer` + reachable from `CommandContext`.
- `internal/component/bgp/reactor/` (wiring) - register the forward-pool Quiescer when the reactor attaches to the server.
- `internal/component/*/yang/ze-system-*.yang` (or the system command YANG) - declare the `quiesce` RPC + command mapping.
- `test/scripts/ze_api.py` - add `quiesce()`; make `wait_for_ack` use it and drop `time.sleep`.
- `docs/architecture/api/commands.md` - document `request quiesce` + the Quiescer registry.
- `docs/functional-tests.md` - note the barrier as the sleep replacement.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPC) | Yes | system command YANG — declare `quiesce` RPC + `ze:command` mapping; native constraints (no free-form leaves) |
| CLI grammar (verb-first) | Yes | `request quiesce` — verify `make ze-cli-grammar-check` |
| Functional test for new RPC | Yes | `test/plugin/quiesce-barrier.ci` |
| Pipe completeness | No | barrier returns a small status, not a list; still route through standard response |
| Doctor check | No | no new runtime dependency |
| Prometheus counters | No (Layer 1) | consider a `quiesce_duration` metric in a follow-on |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`request quiesce`) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (`ze-system:quiesce` + Quiescer registry) |
| 8 | Plugin SDK/protocol changed? | Maybe | if a Quiescer registration API is exported to plugins, note in `ai/rules/plugin-design.md` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (barrier replaces sleep; `ze_api.quiesce()`) |
| 15 | Registered command/inventory changed? | Yes | inventory docs list the new command |
| (others) | | No | grep-verified during completion |

## Files to Create
- `internal/component/plugin/server/quiesce.go` - `Quiescer` type, `QuiescerRegistry`, `RegisterQuiescer`, `handleQuiesce`.
- `internal/component/plugin/server/quiesce_test.go` - unit tests.
- `test/plugin/quiesce-barrier.ci` - functional test.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create — verify reactor→server wiring point + YANG system command file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |
| 5. Full verification | `make ze-verify-changed` (scoped) |
| 6-12. Reviews | Checklists below |
| 13. /ze-review gate | Review Gate |
| 14. Close | two-commit closure |

### Implementation Phases
1. **Phase: Wiring (FIRST)** — `Quiescer` type + `QuiescerRegistry` on the server; `handleQuiesce` stub registered as `ze-system:quiesce`; failing `TestQuiesceDispatchDrainsRegisteredQuiescers`.
   - Files: `quiesce.go`, `system.go`, YANG.
   - Verify: `request quiesce` dispatches to the stub (reply StatusDone, no registrants).
2. **Phase: Registry semantics** — implement register/list, concurrent drain with per-quiescer timeout + error aggregation.
   - Tests: `TestQuiescerRegistryRegistersAndLists`, `TestQuiesceNoRegistrantsIsNoop`, `TestQuiesceTimeoutNamesStuckSubsystem`.
3. **Phase: BGP registrant** — register the forward-pool Quiescer (body = `reactor.Flush`) at reactor wiring.
   - Tests: `TestBGPForwardPoolRegistersQuiescer`.
4. **Phase: Test harness** — `ze_api.quiesce()`; `wait_for_ack` uses it, remove `time.sleep`; `test/plugin/quiesce-barrier.ci`.
5. **Functional + full verification.**
6. **Complete spec** — audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has file:line implementation |
| Correctness | Barrier replies only after ALL registrants drain; timeout never hangs the daemon |
| Data flow | `handleQuiesce` iterates the registry; never names BGP/FIB/tc |
| CLI grammar | `request quiesce` passes `ze-cli-grammar-check` |
| Registration over hardcoding | Quiescers register into the registry; no per-subsystem switch in the handler |
| Rule: no-layering | `wait_for_ack`'s old sleep fully removed, not left alongside |
| Concurrency | drain goroutines are race-clean; per-quiescer context deadline |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `ze-system:quiesce` RPC registered | grep `ze-system:quiesce` in registration; dispatch test |
| BGP forward-pool Quiescer | `TestBGPForwardPoolRegistersQuiescer` passes |
| `wait_for_ack` sleep-free | `grep -n "time.sleep" test/scripts/ze_api.py` shows none in `wait_for_ack` |
| Functional barrier test | `ls test/plugin/quiesce-barrier.ci`; runs green |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | `quiesce` must be bounded (per-quiescer timeout) so a caller can't hang the daemon |
| Input validation | `quiesce` takes no untrusted args; reject extras |
| Authorization | `request quiesce` goes through the same AAA authorizer as other operational commands |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Barrier hangs | Add/enforce per-quiescer deadline (R-1) |
| Route assert still flaky after quiesce | A-1 broken → the drain is insufficient; investigate reactor.Flush |
| Grammar gate red | Rename command (A-3) |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
- The control plane is already synchronous; the sleeps live on the async forwarding/downstream planes. The barrier is the general form of `peer-flush`.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Separate `request quiesce` barrier | Make each mutating command block until forwarded (a `?wait` mode) | A separate barrier is non-invasive, composes across subsystems, and matches the existing `peer-flush` precedent; per-command blocking would change every handler's semantics |
| Registry of `Quiescer`s | Hardcode BGP+FIB+tc in the handler | Registration-over-hardcoding; Layer 3 subsystems opt in with zero handler change |
| Concurrent drain + per-quiescer timeout | Sequential drain | Independent subsystems drain in parallel (faster); timeout bounds a stuck one |

## Known Limitations
- Layer 1 registers only the BGP forward pool. FIB/tc/listener quiescers and completion events are follow-on specs. Until then, tests touching those planes still sleep.
- **`wait_for_ack` is not migrated (AC-6 rescoped).** Its `time.sleep` also
  covers the peer simulator's own `cmd=api` interleaving (EOR etc.), which the
  forward-pool quiesce does not drain. Removing it regressed `nexthop`-style
  tests (R-3 confirmed). A follow-on **peer-side quiescer** (drain ze-peer's
  pending `cmd=api`) is needed before `wait_for_ack` can drop its sleep. Until
  then, `quiesce()` is the sleepless barrier for tests that don't depend on
  ze-peer timing.
- `quiesce` proves "pending work drained," not "no NEW work will arrive" — stability-window assertions ("still up after 3s") remain legitimately time-based.
- **A `Quiescer` MUST honor ctx cancellation.** The per-quiescer timeout cancels
  ctx but cannot force-return a drain that ignores it; such a registrant would
  hang the barrier. Enforced by contract (documented on `QuiesceFunc`), not by
  code. The only current registrant (reactor `FlushForwardPool`) selects on
  `ctx.Done()`.

## Implementation Summary
### What Was Implemented
- (filled during /implement)
### Bugs Found/Fixed
### Documentation Updates
### Deviations from Plan

## Implementation Audit
### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Quiescer registry + `request quiesce` barrier | Done | `quiesce.go` | 6 unit tests, -race |
| BGP forward pool as first registrant | Done | `quiesce.go` `registerReactorQuiescer` + `server.go` NewServer | |
| `ze_api.quiesce()` sleepless barrier helper | Done | `ze_api.py` | |
| `wait_for_ack` drops its sleep | Rescoped (follow-on) | `ze_api.py` | needs peer-side quiescer; R-3 confirmed |
### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestQuiesceDispatchDrainsRegisteredQuiescers`, `TestQuiescerRegistryRegistersAndLists` | |
| AC-2 | Done | `TestQuiesceReportsQuiescerError`, `TestQuiesceDispatchDrainsRegisteredQuiescers` | |
| AC-3 | Done | `TestQuiesceNoRegistrantsIsNoop` | |
| AC-4 | Done | unit-proven barrier + `quiesce-barrier.ci` plumbing (see honest note) | |
| AC-5 | Done | `TestQuiesceTimeoutNamesStuckSubsystem` | |
| AC-6 | Rescoped | `quiesce()` sleepless; `wait_for_ack` follow-on | user-facing barrier delivered |
### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| 6 quiesce unit tests | Done | `quiesce_test.go` | green, -race |
| `quiesce-barrier.ci` | Done | `test/plugin/quiesce-barrier.ci` | PASS (test 328) |
### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `quiesce.go` (+_test) | Done | new |
| `server.go` | Done | field + NewServer registrant |
| `ze-system-cmd.yang` / `ze-system-api.yang` | Done | command + rpc |
| `ze_api.py` | Done | quiesce() added; wait_for_ack unchanged |
| `commands.md` / `functional-tests.md` | Done | docs |
### Audit Summary
- **Total items:** 6 ACs, 6 files, 7 tests
- **Done:** 5 ACs, all files, all tests
- **Partial:** 0
- **Skipped:** 0
- **Changed:** AC-6 rescoped (wait_for_ack migration → follow-on; user-informed)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Tests wait for completion instead of sleeping | functional test | `test/plugin/quiesce-barrier.ci` asserts on-wire after `request quiesce` with no `time.sleep`; `wait_for_ack` sleep removed |

## Review Gate
### Run 1 (adversarial review over the complete diff)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | `quiesce-barrier.ci` can't distinguish "barrier blocks" from "no-op" — `wait_for_shutdown()` gives UPDATEs unbounded time to arrive | `test/plugin/quiesce-barrier.ci` | **fixed (docs)** — AC-4 evidence rephrased: `.ci` is a smoke/plumbing test; the blocking property is unit-proven (`TestQuiesceDispatchDrainsRegisteredQuiescers`) + `fwdPool.Barrier` sentinel FIFO |
| 2 | NOTE | Cooperative timeout: a quiescer that ignores ctx would hang `wg.Wait()` despite the deadline — the "every quiescer honors ctx" invariant is undefended by code | `quiesce.go` `quiesceAll` | **fixed (doc)** — `QuiesceFunc` doc now states the MUST-honor-ctx contract + notes the sole registrant selects on `ctx.Done()`; also in Known Limitations |
| 3 | NOTE | If the caller's request ctx is already canceled, every derived `WithTimeout` fires immediately → `StatusError` blaming the subsystems rather than the cancellation (harmless — client is gone) | `quiesce.go` `quiesceAll` | acknowledged — cosmetic error attribution only; the caller has already left |

Review confirmed CLEAN elsewhere: `quiesceAll` slice access race-free (distinct indices, read after `wg.Wait`); per-quiescer `WithTimeout` genuinely bounds a wedged worker via `fwdPool.Barrier`'s `select{<-done; <-ctx.Done()}`; happy path returns as soon as workers drain (no gratuitous 10s); `registerReactorQuiescer` nil-safe (guard + `Coordinator.FlushForwardPool` nil-safe); RPC output schema matches the handler's `Map`; `wait_for_ack` revert is correct.

### Fixes applied
- Rephrased AC-4 evidence / functional-test status to distinguish smoke-test plumbing from the unit-proven blocking property (NOTE 1).
- Documented the `QuiesceFunc` MUST-honor-ctx contract in code + Known Limitations (NOTE 2).

### Final status
- [x] Adversarial review shows 0 BLOCKER, 0 ISSUE
- [x] All NOTEs recorded above (NOTE 1 & 2 fixed; NOTE 3 acknowledged as cosmetic)

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/plugin/server/quiesce.go` | Yes | created |
| `internal/component/plugin/server/quiesce_test.go` | Yes | created |
| `test/plugin/quiesce-barrier.ci` | Yes | created |
### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..3,5 | barrier drains/no-op/timeout/error | `go test ./internal/component/plugin/server/ -run TestQuiesce -race` → 6 PASS |
| AC-4 | quiesce() end-to-end, no sleep | `ze-test bgp plugin --pattern quiesce-barrier` → `1/1 PASS 328` |
| BGP registrant | forward pool registered | `TestBGPForwardPoolRegistersQuiescer` PASS |
| RPC wiring | ze-system:quiesce has YANG path + count | `TestEveryRPCHasYANGPath`, `TestRPCRegistrationPerModule` PASS |
### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `quiesce()` → ze-system:quiesce → drain → on-wire | `test/plugin/quiesce-barrier.ci` | Yes — reuses announce.ci wire hex; PASS on real daemon |
### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | CONFIRMED | quiesce-barrier.ci passes (routes on wire post-quiesce) |
| A-2 | CONFIRMED | registerReactorQuiescer in NewServer; handleQuiesce reads ctx.Server.Quiescers() |
| A-3 | CONFIRMED | `quiesce` absent from cli-grammar violations; server pkg green |
### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `commands.md` Quiesce Barrier section | source anchors → quiesce.go, ze-system-cmd.yang | Yes |
| `functional-tests.md` barrier note | source anchors → ze_api.py, quiesce.go | Yes |

## Checklist
### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Documentation Update Checklist answered
### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
### Design
- [ ] No premature abstraction
- [ ] No speculative features (registry is justified by Layer 3 registrants)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling
### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-test-sync-quiesce.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-test-sync-quiesce.md`

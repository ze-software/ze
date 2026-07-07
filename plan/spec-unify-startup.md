# Spec: unify-startup

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/process-protocol.md` - the 5-stage plugin startup protocol (canonical wire contract)
4. `internal/component/plugin/server/startup.go`, `internal/component/plugin/server/subsystem.go`, `internal/component/plugin/server/adhoc.go`, `internal/component/plugin/startup_coordinator.go`

## Task

DESIGN-REVIEW.md finding 2 (row "Process orchestration") flags that the plugin startup 5-stage
handshake is driven by TWO independent implementations that share no code, coupled only by the
same magic method strings (`ze-plugin-engine:declare-registration`, `:declare-capabilities`,
`:ready` and the callbacks `ze-plugin-callback:configure`, `:share-registry`).

1. The engine path: `Server.handleProcessStartupRPC` (`internal/component/plugin/server/startup.go:508`),
   a barrier-coordinated driver using `StartupCoordinator` so concurrently started plugins in a
   dependency tier synchronize stage-by-stage.
2. The hub path: `SubsystemHandler.completeProtocol` (`internal/component/plugin/server/subsystem.go:129`),
   which re-implements the identical five stages inline and synchronously on the raw connection,
   with no barrier.

The two hard-code the same wire choreography in two places. A change to the protocol (a new method
name, a reordered stage, an added validation) must be applied to both by hand, and nothing forces
them to stay in step. This spec unifies the wire choreography behind a single shared stage-driver
that both paths call, closing the duplication while preserving all externally observable behavior
(this is an internal refactor, not a protocol change).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `docs/architecture/api/process-protocol.md` - canonical description of the 5-stage handshake and its method names
  → Constraint: the wire sequence is fixed (registration -> configure -> capabilities -> share-registry -> ready); the refactor must not reorder or rename any stage message.
  → Decision: the magic method strings are the only contract between engine and plugin, so they must live in exactly one place after unification (a shared driver), not be duplicated per orchestrator.
- [ ] `ai/rules/data-flow-tracing.md` - required data-flow section discipline
  → Constraint: every boundary crossed by the handshake bytes must be enumerated and marked verified.
- [ ] `ai/rules/plugin-self-containment.md` - registration over hardcoding
  → Constraint: the shared driver must not grow a per-caller switch; caller-specific behavior is injected through an interface (a startup sink), discovered by the driver, not branched on inside it.
- [ ] `ai/rules/no-partial-completion.md` - completion discipline for the migration
  → Constraint: the loser (inline `completeProtocol` stage logic) must be fully deleted, not left beside the winner.

**Key insights:**
- The engine path is a strict superset of the hub path in engine-side effects (it registers families, capabilities, commands, subscriptions, doctor checks, enrichers, bridge dispatch, cache-consumer state and signals the reactor). The hub path deliberately does almost none of that: it only harvests the plugin's declared commands and YANG schema and delivers nil config / nil registry.
- The engine path ALREADY has a "single-conn synchronous" mode: `HandleAdHocPluginSession` (`adhoc.go:24`) runs `handleProcessStartupRPC` with `s.coordinator == nil`, and `stageTransition` (`startup.go:46-48`) returns immediately when the coordinator is nil. So the barrier is already optional. The remaining gap is that the engine driver is a `*Server` method and unconditionally uses `*Server` fields (`registry`, `capInjector`, `dispatcher`, `reactor`), which the hub process does not have.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `internal/component/plugin/server/startup.go` - engine 5-stage driver `handleProcessStartupRPC` (:508), barrier helpers `stageTransition` (:42), `progressThroughStages` (:87), phase orchestration `runPluginStartup` (:130) / `runPluginPhase` (:317), callback delivery `deliverConfigRPC` (:791) / `deliverRegistryRPC` (:829). Reads stages at :542 (declare-registration), :648 (declare-capabilities), :699 (ready).
  → Constraint: the engine path does full engine registration (registry, families, capabilities, commands, subscriptions, doctor checks, enrichers, bridge, cache-consumer) and reactor signaling between stages; these effects must be preserved bit-for-bit.
- [ ] `internal/component/plugin/server/subsystem.go` - hub 5-stage driver `SubsystemHandler.completeProtocol` (:129), inline and synchronous on the raw conn: :145 (declare-registration), :178 (SendConfigure nil), :187 (declare-capabilities), :196 (SendShareRegistry nil), :205 (ready). Orchestration chain `SubsystemManager.StartAll` (:317) -> `SubsystemHandler.Start` (:86) -> `completeProtocol`. Harvests `h.commands` (:157) and `h.schema` (:162).
  → Constraint: the hub path must keep harvesting commands and schema into the handler-local fields; `FindHandler` (:410) and `RegisterSchemas` (:479) depend on them.
- [ ] `internal/component/plugin/server/adhoc.go` - `HandleAdHocPluginSession` (:24) runs the engine driver with a nil coordinator (all barriers skipped). Proof that the engine driver already supports a single-conn synchronous mode.
  → Constraint: this path must keep working unchanged after the refactor; it is the reference for "engine driver without a barrier".
- [ ] `internal/component/plugin/startup_coordinator.go` - `StartupCoordinator` barrier: `NewStartupCoordinator` (:46), `StageComplete` (:76), `WaitForStage` (:105), `advanceStage` (:229). Synchronizes all plugins in a tier before any advances.
  → Constraint: the barrier semantics (deadline measured from stage start, first-failure-wins abort) must be preserved for the engine path.
- [ ] `internal/component/hub/hub.go` - `Orchestrator.Start` (:86) calls `subsystems.StartAll` (:93), then `RegisterSchemas` (:98) and `Freeze` (:105-106). The hub process has no reactor, no capInjector, no dispatcher command registry.
  → Constraint: the hub caller signature (`StartAll`, `Start`) must not change; only the internals of `completeProtocol` are redirected.
- [ ] `internal/component/hub/reload.go` - reload path pre-starts a replacement handler via `NewSubsystemHandler(...).Start(ctx)` (:157-166) and `startPlugin` (:146). Second caller of `SubsystemHandler.Start`.
  → Constraint: reload must continue to start a fresh handler through the same protocol path.

**Feature inventory (feature x implementation):**

| Feature | Engine `handleProcessStartupRPC` | Hub `completeProtocol` |
|---------|----------------------------------|------------------------|
| Reads declare-registration, validates method string | Yes (:542) | Yes (:145) |
| Sends configure callback | Yes, real config sections from config tree via `deliverConfigRPC` / `WantsConfigRoots` (:801-812) | Yes, always nil (:178) |
| Reads declare-capabilities, validates method string | Yes (:648) | Yes (:187) |
| Sends share-registry callback | Yes, real command list from `deliverRegistryRPC` (:830-852) | Yes, always nil (:196) |
| Reads ready, validates method string | Yes (:699) | Yes (:205) |
| Barrier sync across concurrent plugins in a tier | Yes, via `StartupCoordinator` (:377, :767) | No, one conn at a time, synchronous |
| Nil-barrier single-conn mode | Yes, used by ad-hoc sessions (:46-48, adhoc.go) | Inherently single-conn (no barrier concept) |
| Per-stage timeout measured from stage start | Yes (`stageTransition` deadline, :57-67) | No, one 30s wrap over the whole protocol (subsystem.go:117) |
| Registers plugin into `PluginRegistry` (`s.registry`) | Yes (:601) | No |
| Registers declared families | Yes (`registerPluginFamilies`, :609) with rollback | No |
| Registers capabilities into cap injector | Yes (:670) | No |
| Registers commands into dispatcher CommandRegistry (+ deprecated aliases) | Yes (:744-760) | No |
| Harvests declared commands into a local slice | No (uses dispatcher) | Yes (`h.commands`, :157) |
| Harvests declared YANG schema into a local field | No (schema flows via registration/schema registry elsewhere) | Yes (`h.schema`, :162) |
| Validates doctor-check declarations | Yes (:558) | No |
| Validates + proxy-registers enrichers | Yes (:566, :624) | No |
| Validates declared dependencies against configured set | Yes (:586-595) | No |
| Registers startup subscriptions from ready params | Yes (:716) | No |
| Wires direct bridge dispatch / bridge transport switch | Yes (:726, :783) | No |
| Registers cache-consumer state | Yes (:579) | No |
| Signals reactor API-ready | Yes (:773) | No |
| Sets `proc` stage and rolls back partial registration on failure | Yes (`rollbackStartupProcess`, :463) | No, just `proc.Stop()` on error (Start :121) |
| Requires a full `*Server` (reactor/capInjector/dispatcher) | Yes | No, only needs a `process.Process` and a conn |

**Why two exist (root cause):** the two run in different processes with different amounts of
infrastructure. `handleProcessStartupRPC` runs inside the main engine `Server`, which owns the
reactor, capability injector, dispatcher command registry, subscription table and bridge, so it
performs the full set of engine-side registrations between stages. `completeProtocol` runs inside
the lighter hub orchestrator process (`internal/component/hub`), which forks `ze bgp` / `ze rib` /
`ze gr` as subsystems and routes by schema-handler path; it has none of that machinery, so it only
needs the plugin's declared commands and schema and intentionally discards config and registry
(sends nil). They are NOT different protocol layers: they speak the identical five-message wire
sequence with the identical method strings. The divergence is in the engine-side EFFECTS between
stages, not in the wire choreography. That makes the wire choreography (read method, validate name,
respond, send callback) the genuinely duplicated, unifiable part; the per-caller effects are the
legitimately different part and belong behind an injected interface, not inside the shared driver.

**Behavior to preserve:** (unless user explicitly said to change)
- The exact five-message wire sequence and every method string: `ze-plugin-engine:declare-registration`, `ze-plugin-callback:configure`, `ze-plugin-engine:declare-capabilities`, `ze-plugin-callback:share-registry`, `ze-plugin-engine:ready`.
- Engine path: every engine-side effect listed in the inventory (registry, families with rollback, capabilities, commands + deprecated aliases, subscriptions, doctor checks, enrichers, bridge dispatch, cache-consumer, reactor signaling), plus barrier synchronization and per-stage-start deadline semantics.
- Hub path: harvesting declared commands into `h.commands` and schema into `h.schema`; delivering nil config and nil registry; the 30-second whole-protocol timeout envelope in `Start`.
- Ad-hoc path: nil-coordinator single-conn behavior of `HandleAdHocPluginSession` unchanged.
- All existing error messages that callers/tests assert on (for example "expected declare-registration, got ...", "expected declare-capabilities, got ...", "expected ready, got ...").

**Behavior to change:**
- None - internal refactor, behavior preserved. The only structural change is that both drivers call one shared stage-driving helper set instead of each hard-coding the wire sequence.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Engine path: `Server.Start` spawns `runPluginStartup` (`server.go:522`), which runs `runPluginPhase` per dependency tier; each tier launches one `handleProcessStartupRPC` goroutine per plugin over that plugin's `MuxConn`.
- Hub path: `Orchestrator.Start` (`hub.go:86`) calls `SubsystemManager.StartAll` (`subsystem.go:317`), which calls `SubsystemHandler.Start` (`subsystem.go:86`) -> `completeProtocol` over the forked subsystem's conn.
- Ad-hoc path: `HandleAdHocPluginSession` (`adhoc.go:24`) over an arbitrary reader/writer (for example an SSH channel), coordinator set to nil.
- Format at entry: JSON-RPC requests/responses framed by `rpc.MuxConn` (plugin-initiated requests read via `conn.ReadRequest`, engine-initiated callbacks sent via `conn.SendConfigure` / `conn.SendShareRegistry`).

### Transformation Path
1. Driver reads the plugin-initiated `declare-registration` request and validates the method string; on mismatch it sends a JSON-RPC error and aborts.
2. Driver hands the decoded `DeclareRegistrationInput` to the caller-specific sink: the engine sink validates and registers (registry, families, doctor checks, enrichers, dependencies, cache-consumer); the hub sink harvests commands and schema. Driver sends the OK response.
3. Driver advances to Config: engine path optionally waits at the barrier, then delivers real config sections; hub path delivers nil. Delivery is a sink call (`ConfigSections()` returns the payload).
4. Driver reads `declare-capabilities`, validates the method string, hands the input to the sink (engine registers into cap injector; hub ignores), sends OK.
5. Driver advances to Registry: delivers the share-registry payload from the sink (engine builds the real command list; hub sends nil).
6. Driver reads `ready`, validates the method string, hands the ready input to the sink (engine registers subscriptions, wires bridge dispatch, registers dispatcher commands, signals the reactor; hub does nothing), advances the final barrier and sends OK.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine/hub process ↔ plugin process | JSON-RPC over `rpc.MuxConn` (`ReadRequest`, `SendResult`, `SendError`, `SendConfigure`, `SendShareRegistry`) | [ ] |
| Shared stage-driver ↔ caller effects | injected startup-sink interface (per-stage hooks), no per-caller switch in the driver | [ ] |
| Driver ↔ barrier | `StartupCoordinator` when non-nil; no-op when nil (ad-hoc and hub) | [ ] |
| Hub handler ↔ hub routing | harvested `h.commands` feed `FindHandler`; harvested `h.schema` feeds `RegisterSchemas` | [ ] |

### Integration Points
- `StartupCoordinator` (`internal/component/plugin/startup_coordinator.go`) - remains the barrier for the engine path; the shared driver calls it only through the sink/barrier seam so a nil barrier is a clean no-op.
- `rpc` package (`pkg/plugin/rpc`) - the method strings and callback senders; after unification the strings are referenced in exactly one driver.
- `process.Process` (`internal/component/plugin/process`) - both paths already build a `process.Process` and obtain a `PluginConn`; the shared driver operates on that conn.
- `SubsystemManager.RegisterSchemas` / `FindHandler` (`subsystem.go`) - unchanged consumers of the hub sink's harvested data.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — the shared driver dispatches caller-specific effects through an injected startup-sink interface that the driver discovers and calls; it does NOT add a per-caller field, switch case, or factory branch inside the driver or any core/shared struct (small-core/registration; `ai/rules/plugin-self-containment.md`).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The two drivers speak byte-identical wire sequences (same five methods, same order, same callbacks) | Direct read of `startup.go:542-704` vs `subsystem.go:145-211`; both assert the same method strings | If they differ, a single shared driver would change one path's wire behavior | grep both files for the method strings; diff the read/send order; existing `TestSubsystemRPCProtocol` and `TestAdHocProcessHandshake` must still pass | unvalidated |
| A-2 | The engine driver's `*Server` field usage can be factored behind a sink interface without needing the hub to build a `*Server` | `adhoc.go` already runs the engine driver with a nil coordinator; only `s.registry`/`s.capInjector`/`s.dispatcher`/`s.reactor` derefs remain caller-specific | If some effect is entangled with `*Server` internals that cannot be expressed through a sink, the hub cannot adopt the driver | Extract the sink interface; confirm the engine sink is a thin `*Server` adapter and the hub sink needs only a `process.Process` + conn | unvalidated |
| A-3 | The hub path's nil config and nil registry are intentional, not a latent bug | `subsystem.go:178,196` pass nil unconditionally; hub has no config tree or command registry to deliver | If they were meant to deliver real data, preserving nil would freeze a bug | User/design confirmation plus `docs/architecture/api/process-protocol.md`; hub sink returns nil for both payloads by design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Timeout-model mismatch: the engine uses per-stage-start deadlines, the hub a single 30s envelope; a naive merge could change hub timeout behavior | Hub reload/start tests hang or fail faster/slower than before | Keep the 30s envelope in `SubsystemHandler.Start` around the shared driver call; the shared driver's barrier deadline logic stays inert when the barrier is nil |
| R-2 | Losing an engine-side effect during extraction (for example bridge dispatch or subscription registration) | Plugin startup functional/interop test regressions (bgp-rs replay, rpki gate) | Move effects one stage at a time behind the sink; characterization test in Phase 1 captures the full engine effect set before refactor |
| R-3 | The sink interface leaks `*Server` types back into the hub package, recreating coupling | Hub package imports grow; tier-check flags a bad dependency direction | Define the sink interface in the driver's package with value/`rpc` types only; engine and hub each implement it locally |

## Wiring Test (MANDATORY — NOT deferrable)

<!-- For a refactor, the wiring test is the EXISTING test that proves the migrated path still works. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Hub forks `ze bgp` and completes startup | → | `SubsystemHandler.Start` -> shared stage-driver (was `completeProtocol`) | `TestHubStartupWithBGP` (`test/hub/startup_test.go`) |
| Hub subsystem completes the 5-stage handshake | → | shared stage-driver via `SubsystemManager` | `TestSubsystemRPCProtocol` (`internal/component/plugin/server/subsystem_test.go`) |
| Engine ad-hoc session runs the driver with a nil barrier | → | `Server.handleProcessStartupRPC` (nil coordinator) via shared driver | `TestAdHocProcessHandshake` (`internal/component/plugin/server/adhoc_test.go`) |
| Engine tier startup registers and rolls back on failure | → | `Server.runPluginPhase` -> shared stage-driver | `TestPluginStartupRollsBackPartialRegistration`, `TestRunPluginPhaseReturnsStageFailure` (`internal/component/plugin/server/startup_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The five wire method strings exist in the codebase | They appear in exactly one shared stage-driver location; neither `handleProcessStartupRPC` nor `completeProtocol` re-declares the read/validate/respond sequence inline |
| AC-2 | Hub forks a subsystem and runs startup | `SubsystemHandler` still harvests declared commands into `h.commands` and schema into `h.schema`, delivers nil config and nil registry, all via the shared driver; `TestSubsystemRPCProtocol` passes |
| AC-3 | Engine starts a tier of two concurrent plugins | Barrier synchronization is unchanged: both reach each stage before either advances; per-stage-start deadline preserved; `TestRunPluginPhaseReturnsStageFailure` passes |
| AC-4 | Engine startup, a plugin fails mid-handshake | Partial registration is rolled back exactly as before (`rollbackStartupProcess`); `TestPluginStartupRollsBackPartialRegistration` and `TestPluginStartupRollsBackFamiliesAfterLaterStageFailure` pass |
| AC-5 | Ad-hoc plugin session over a non-`net.Conn` stream | Nil-coordinator single-conn handshake completes; `TestAdHocProcessHandshake` and `TestAdHocProcessRuntime` pass |
| AC-6 | A plugin sends a wrong method at any stage | The shared driver returns the same error message strings as today ("expected declare-registration, got ...", etc.) for both engine and hub callers |
| AC-7 | The inline `completeProtocol` stage sequence | Is deleted; `completeProtocol` (or its replacement) contains no re-implemented read/validate/respond/send sequence, only a call into the shared driver plus hub-specific sink hooks |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSubsystemRPCProtocol` | `internal/component/plugin/server/subsystem_test.go` | Hub 5-stage handshake through the shared driver; commands/schema harvested; nil payloads delivered | |
| `TestSubsystemHandler` / `TestSubsystemManager` | `internal/component/plugin/server/subsystem_test.go` | Handler lifecycle and manager registration unchanged | |
| `TestAdHocProcessHandshake` / `TestAdHocProcessRuntime` | `internal/component/plugin/server/adhoc_test.go` | Nil-coordinator engine driver path unchanged | |
| `TestPluginStartupRollsBackPartialRegistration` | `internal/component/plugin/server/startup_test.go` | Engine rollback on mid-handshake failure preserved | |
| `TestPluginStartupRollsBackFamiliesAfterLaterStageFailure` | `internal/component/plugin/server/startup_test.go` | Family rollback across later stage failure preserved | |
| `TestRunPluginPhaseReturnsStageFailure` | `internal/component/plugin/server/startup_test.go` | Barrier stage-failure propagation preserved | |
| `TestRPCRegistrationToRegistry` / `TestRPCCapabilityToInjector` | `internal/component/plugin/server/rpc_registration_test.go` | Engine-side registration and capability injection effects preserved through the sink | |
| `TestSharedStartupDriverSinkDispatch` (new) | `internal/component/plugin/server/startup_test.go` | The shared driver invokes each sink hook in the correct stage order for both a full engine sink and a minimal hub sink (characterization of the injected effects) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A - no new numeric inputs (refactor reuses existing stage enum and timeouts) | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestHubStartupWithBGP` | `test/hub/startup_test.go` | Operator starts the hub; it forks `ze bgp` and completes the 5-stage protocol; no user-facing behavior change; existing test suite passes with no regressions | |
| Existing engine startup + managed-hub suites | `internal/component/plugin/server/*_test.go`, `test/managed/*.ci` | Plugins load and register through the engine path; no user-facing behavior change; existing test suite passes with no regressions | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - no wire protocol change to any external peer; the plugin startup handshake is an internal engine/hub-to-plugin contract, unchanged on the wire. Justification per `ai/rules/interop-and-goal-validation.md`: refactor preserves the exact byte sequence, validated by existing engine and hub startup tests. | N/A | N/A | N/A | |

### Future (if deferring any tests)
- None deferred.

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
- `internal/component/plugin/server/startup.go` - extract the wire choreography (read method, validate name, respond, deliver callback) into shared stage-driving helpers plus a startup-sink interface; make `handleProcessStartupRPC` a thin engine sink over the shared driver.
- `internal/component/plugin/server/subsystem.go` - replace the inline `completeProtocol` stage logic with a call into the shared driver, providing a minimal hub sink that harvests commands/schema and returns nil payloads; delete the duplicated inline sequence.
- `internal/component/plugin/server/adhoc.go` - keep the nil-coordinator ad-hoc path working; adjust only if the extraction changes the driver signature.
- `internal/component/plugin/startup_coordinator.go` - no behavior change expected; confirm the barrier seam is reachable through the sink so a nil barrier is a clean no-op.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] No - no new RPC or config surface | - |
| CLI commands/flags | [ ] No | - |
| Functional test for new RPC/API | [ ] No new RPC; existing functional tests must still pass | `test/hub/startup_test.go` |
| Doctor check for runtime dependencies | [ ] No new runtime dependency | - |
| Prometheus counters/metrics | [ ] No new observable state | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 8 | Plugin SDK/protocol changed? | [ ] No - wire protocol byte-identical; only internal driver structure changes | `docs/architecture/api/process-protocol.md` (verify no stale claim about two drivers) |
| 12 | Internal architecture changed? | [ ] Yes - one shared startup driver replaces two | `docs/architecture/api/process-protocol.md` and/or `docs/architecture/core-design.md`: note the single driver + injected sink |
| 16 | Any changed source file referenced by existing doc source anchors? | [ ] Check - `startup.go` and `subsystem.go` carry `// Design:` anchors to `process-protocol.md` | grep `docs/` for those anchors and update any claim describing two separate drivers |

## Files to Create
- None expected. The shared driver and sink interface live in `internal/component/plugin/server/startup.go` (or a new sibling file in the same package if size warrants, decided during implementation). A new characterization test may be added to `internal/component/plugin/server/startup_test.go`.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan - check what exists |
| 3. Wiring phase | Wiring Test table - characterization test capturing current behavior |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7-10. Critical review + fix + re-verify | Critical Review Checklist |
| 11-14. Deliverables, security, re-verify, summary | Checklists below |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring / characterization (MANDATORY FIRST)** — capture current behavior before touching structure.
   - Tests: add `TestSharedStartupDriverSinkDispatch` capturing the exact stage-by-stage hook order and payloads for a full engine sink and a minimal hub sink; confirm `TestSubsystemRPCProtocol`, `TestAdHocProcessHandshake`, `TestPluginStartupRollsBackPartialRegistration`, `TestRunPluginPhaseReturnsStageFailure` all pass against the current code.
   - Files: `internal/component/plugin/server/startup_test.go`.
   - Verify: characterization test compiles and passes against the unmodified drivers; it fails loudly if any stage effect is dropped later.
2. **Phase: Extract the startup-sink interface** — define an interface in the server package with per-stage hooks (on-registration, config-sections, on-capabilities, registry-commands, on-ready) using only value/`rpc` types, no `*Server` in the signature.
   - Tests: characterization test still passes; a unit test asserts the engine sink and hub sink both satisfy the interface.
   - Files: `internal/component/plugin/server/startup.go`.
   - Verify: interface has exactly the hooks the two callers need, no per-caller branching.
3. **Phase: Route the engine driver through the shared choreography** — rewrite `handleProcessStartupRPC` so the read/validate/respond/deliver sequence lives in shared helpers, and the engine-side effects move into an engine-sink adapter over `*Server`. Preserve barrier calls and rollback exactly.
   - Tests: all `startup_test.go`, `rpc_registration_test.go`, `adhoc_test.go` tests pass.
   - Files: `internal/component/plugin/server/startup.go`, `internal/component/plugin/server/adhoc.go`.
   - Verify: engine effects and barrier semantics unchanged; ad-hoc nil-coordinator path unchanged.
4. **Phase: Route the hub driver through the shared choreography and delete the loser** — replace `completeProtocol`'s inline stage logic with a call into the shared helpers plus a minimal hub sink (harvest commands/schema, nil payloads); delete the duplicated inline sequence. Keep the 30s envelope in `Start`.
   - Tests: `TestSubsystemRPCProtocol`, `TestSubsystemHandler`, `TestSubsystemManager`, `test/hub/startup_test.go` pass.
   - Files: `internal/component/plugin/server/subsystem.go`.
   - Verify: no read/validate/respond/send sequence remains inline in `subsystem.go`; grep for the method strings shows a single driver location.
5. **Functional tests** → confirm `test/hub/startup_test.go` and managed-hub suites pass unchanged.
6. **RFC refs** → none (no protocol MUST/MUST NOT changes; wire sequence preserved).
7. **Full verification** → `make ze-verify`.
8. **Complete spec** → fill audit tables, write learned summary; two commits (code+spec+learned, then `git rm` spec).

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line; the inline `completeProtocol` sequence is gone |
| Feature completeness | Engine keeps every effect in the inventory; hub keeps command/schema harvest and nil delivery; ad-hoc keeps nil-barrier behavior |
| Correctness | Method strings unchanged; error messages unchanged; barrier deadline still measured from stage start; family/registration rollback still fires |
| Naming | Sink interface and hooks use kebab/Go-idiomatic names consistent with the package; no leakage of `*Server` into the hub package |
| Data flow | Wire choreography lives in one place; caller effects live behind the injected sink, not in the driver |
| Registration over hardcoding | The driver dispatches caller effects through the injected sink interface it discovers; NO per-caller field, switch case, or factory added to the driver or any core/shared struct. See `ai/rules/plugin-self-containment.md` |
| Rule: no-layering | Old inline stage logic in `subsystem.go` fully deleted, not left beside the shared driver |
| Rule: tier-check | Sink interface uses value/`rpc` types only; hub package dependency direction unchanged (`make ze-tier-check`) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Single driver location for the five method strings | grep the method strings across `internal/`; expect one driver file, not two inline sequences |
| Inline `completeProtocol` sequence deleted | read `subsystem.go`; confirm it delegates and no read/validate/respond/send remains |
| Engine effects preserved | `go test ./internal/component/plugin/server/... -run 'Startup|RPCRegistration|AdHoc|Subsystem'` all pass |
| Hub startup preserved | `go test ./test/hub/...` and `go test ./internal/component/hub/...` pass |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | The shared driver must keep validating the method string at every stage before acting on params; no path may skip the `req.Method` check |
| Error leakage | Error messages returned to plugins stay identical; no new internal detail leaked in the shared driver's error responses |
| Resource exhaustion | Timeouts preserved: engine per-stage deadline and hub 30s envelope both still bound the handshake; a stalled plugin cannot hang startup indefinitely |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior; a dropped effect means back to the extraction phase |
| Lint failure | Fix inline; if architectural (sink leaks `*Server`) → DESIGN phase |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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
- The engine driver's existing nil-coordinator mode (used by ad-hoc sessions) is the proof that the barrier is already an optional, injectable concern; unification does not invent a new abstraction, it generalizes one the engine already relies on.

## Core Insight
- Two "orchestrators" is a misnomer: there is one wire protocol and two sets of engine-side effects. Unify the protocol, inject the effects.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Winner: the engine `handleProcessStartupRPC` / coordinator path, generalized into a shared stage-driver that both callers use | (a) Keep both, document the boundary; (b) make the hub path the winner and teach it barriers | The engine path is a strict superset of behavior, already has the harder feature (barrier sync) and already supports a barrier-less single-conn mode (ad-hoc). The hub path is a proper subset of effects. Growing the subset to cover the superset would be more work and would move complex engine registration into the lighter hub. Keeping both leaves the magic-string coupling that finding 2 calls out. |
| Inject caller effects through a startup-sink interface rather than branching inside the driver | An `if hub { ... } else { ... }` switch in one merged function | A switch violates registration-over-hardcoding and would re-import `*Server` concerns into the hub context. An interface keeps the driver caller-agnostic and satisfies `ai/rules/plugin-self-containment.md`. |
| Preserve the hub's 30s whole-protocol envelope and the engine's per-stage-start deadlines separately | Force one timeout model on both | The two timeout models are legitimately different (barrier-synchronized tier vs single forked child). The barrier logic is inert when nil, so keeping the envelope in `SubsystemHandler.Start` around the shared driver preserves both without conflict. |
| This IS a genuine redundancy to merge (not two layers to keep) | Declare them separate layers and stop | They speak the byte-identical five-message sequence with identical method strings; only the between-stage effects differ. The wire choreography is truly duplicated and belongs in one place. |

## Known Limitations
- The refactor unifies only the startup handshake choreography. The broader "process orchestration" question (whether the hub `Orchestrator` and the engine `Server` should themselves be one process model) is out of scope; this spec keeps both processes and both orchestration callers, changing only the shared handshake driver they both invoke.
- The hub sink deliberately keeps delivering nil config and nil registry (preserving current behavior); if the hub later needs to deliver real config, that is a separate feature, not part of this unification.

## RFC Documentation

No RFC-governed wire behavior changes; the plugin startup handshake is an internal engine/hub-to-plugin protocol. No `// RFC` comments required.

## Implementation Summary

### What Was Implemented
- [pending implementation]

### Bugs Found/Fixed
- [pending implementation]

### Documentation Updates
- [pending implementation]

### Deviations from Plan
- [pending implementation]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| One shared driver replaces two inline handshake implementations | functional test + grep | [pending] |
| All externally observable behavior preserved | existing test suite | [pending] |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed / deferred / acknowledged |

### Fixes applied
- [pending]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/component/plugin/server/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs updated where the two-driver claim is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added (N/A - no protocol change)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (2 concrete callers today, ad-hoc is a third consumer)
- [ ] No speculative features (needed NOW - finding 2 duplication)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N/A - no new numeric inputs)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N/A with justification above)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-unify-startup.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-unify-startup.md` only

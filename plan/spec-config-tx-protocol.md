# Spec: Config Transaction Protocol

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/8 |
| Updated | 2026-04-07 |

## Transport Revision (2026-04-07)

This spec was originally designed bus-based (publish/subscribe via `pkg/ze/bus.go`).
After review, the bus was found to be premature abstraction: the existing stream
system in `internal/component/plugin/server/dispatch.go` (`subscribe-events`,
`emit-event`, `deliver-event`) already provides pub/sub fan-out, schema validation,
DirectBridge zero-copy, and external plugin participation over TLS. Adding a parallel
bus duplicates infrastructure with no concrete consumer that the stream system cannot
serve.

**Decision:** drop the bus, use the stream system. Config transactions become
events in a new `config` namespace. Topic-style names (`config/verify/<plugin>`)
become `(namespace=config, event-type=verify-<plugin>)` pairs. The protocol
semantics (verify -> apply -> rollback, journal, failure codes, plugin participation
declarations) are unchanged. Only the wire layer changes.

This rolls back part of `spec-arch-0` (the Bus component is removed; the
5-component model becomes 4). See `plan/deferrals.md` 2026-04-07 entry for
the open-namespace future work that handles plugin-to-plugin opaque messaging.

The bus-language sections below are kept where the semantics are still correct;
references to specific bus types/topics are updated inline. Strikethrough
indicates superseded content with the stream-based replacement following.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now) -- pay attention to "Transport Revision (2026-04-07)" at the top
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/config/transaction-protocol.md` - full protocol design (NOTE: bus-based, NEEDS REWRITE)
4. `docs/architecture/plugin/plugin-relationships.md` - plugin dependency map
5. `internal/component/plugin/server/reload.go` - current sequential RPC verify/apply (being replaced)
6. `internal/component/plugin/server/dispatch.go` - stream system (the new transport)
7. `pkg/plugin/sdk/sdk_callbacks.go` - current SDK callbacks
8. `internal/component/config/transaction/orchestrator.go` - existing TxCoordinator (bus-based, pub/sub layer being rewritten)

## Task

Replace ze's sequential RPC-based config reload with a transaction protocol
delivered over the existing stream event system.

Today config reload (`SIGHUP`, CLI commit, API) uses direct RPC calls per plugin
for verify and apply (`SendConfigVerify`/`SendConfigApply` in `reload.go`). There
is no rollback -- if a plugin fails during apply, previously applied plugins keep
their changes. There is no transaction exclusion -- concurrent commits are not
prevented. Plugins cannot declare interest in config roots they don't own.

The goal: a transaction protocol where verify, apply, rollback, and finalization
are events delivered through the stream system (`subscribe-events`/`emit-event`/
`deliver-event` RPCs in `dispatch.go`). Events live in a new `config` namespace
with registered event types (`verify`, `apply`, `rollback`, `committed`, `applied`,
`rolled-back`, `verify-ok`, `verify-failed`, `apply-ok`, `apply-failed`,
`rollback-ok`). Plugins opt in via `ConfigRoots` (ownership) and `WantsConfig`
(read-only interest). The engine orchestrates deadlines from plugin estimates,
enforces transaction exclusion, and provides an SDK journal for rollback support.

**Replaces:** `spec-config-apply-rollback.md` (Go callback approach, never implemented).

**Does NOT include:** per-plugin wiring (reconcilePeers, iface apply, etc.) -- those
are separate follow-up specs that consume this protocol.

**Why stream, not bus:** the stream system already provides pub/sub fan-out
(`dispatch.go:347 deliverEvent` matches subscribers and delivers to each),
schema validation via the event-type registry, DirectBridge zero-copy for
internal plugins, and external plugin participation over TLS. The bus duplicates
all of this. See "Transport Revision" above.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/transaction-protocol.md` - full protocol design (NEEDS REWRITE: currently bus-based, must be re-stated in stream-system terms)
  -> Decision: stream events in `config` namespace, not bus topics, not Go callbacks, not direct RPC
  -> Decision: per-plugin event types (`verify-<name>`/`apply-<name>` for engine->plugin) so each plugin sees only its own diffs
  -> Decision: broadcast event types (`verify-ok`/`verify-failed`/...) for plugin->engine acks, engine subscribes
  -> Decision: config file written after all apply/ok, failure is warning not rollback
  -> Decision: plugin-estimated timeouts, updated after each phase
  -> Constraint: one transaction at a time, SIGHUP queued
- [ ] `docs/architecture/plugin/plugin-relationships.md` - plugin dependency map
  -> Constraint: only 5 plugins own config roots today (bgp, interface, sysrib, fib-kernel, fib-p4)
  -> Constraint: BGP sub-plugins mediated by reactor, no direct config participation
  -> Constraint: no plugin currently uses WantsConfig
- [ ] ~~`docs/architecture/core-design.md` - bus architecture~~ (superseded: bus removed)
  -> ~~Constraint: bus is content-agnostic, `[]byte` payloads, hierarchical topics with `/` separator~~
  -> ~~Constraint: bus metadata for subscription filtering~~
- [ ] `internal/component/plugin/server/dispatch.go` - stream system implementation
  -> Constraint: `subscribeEvents`/`emitEvent`/`deliverEvent` provide pub/sub fan-out
  -> Constraint: `plugin.IsValidEvent(namespace, event-type)` validates known events at registration
  -> Constraint: emitter is excluded from delivery to prevent self-delivery loops
  -> Constraint: DirectBridge zero-copy for internal plugin event delivery
- [ ] `docs/architecture/config/yang-config-design.md` - YANG config system
  -> Constraint: validators are init-time write, read-only after startup
- [ ] `docs/architecture/plugin-manager-wiring.md` - 5-stage startup protocol
  -> Constraint: Stage 1 declares ConfigRoots, WantsConfig. Stage 2 delivers config.

### RFC Summaries (MUST for protocol work)
N/A - internal architecture, not protocol work.

**Key insights:**
- Current reload in `reload.go` is sequential verify then concurrent apply via RPC
- Stream system already handles cross-component events (BGP, RPKI) via `(namespace, event-type)` pairs with DirectBridge zero-copy hot path
- 5 system plugins own config today; BGP sub-plugins are mediated by reactor
- `WantsConfig` is a new declaration; no plugin uses it yet
- Config file write is decoupled from transaction success
- Stream system schema validation (`plugin.IsValidEvent`) catches typos at registration -- a feature, not friction

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/reload.go` - config reload orchestration
  -> Constraint: ReloadFromDisk loads tree, diffs, auto-stops removed plugins, auto-loads new
  -> Constraint: verify is sequential per plugin via RPC
  -> Constraint: apply is concurrent via RPC, errors collected but no rollback
  -> Constraint: reactor apply (reconcilePeers) runs after all plugin applies
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` - SDK callback registration
  -> Constraint: OnConfigVerify(func([]ConfigSection) error) and OnConfigApply(func([]ConfigDiffSection) error)
  -> Constraint: no OnConfigRollback exists
- [ ] `pkg/plugin/rpc/types.go` - RPC type definitions
  -> Constraint: ConfigSection has Root + Data. ConfigDiffSection has Root + Added/Removed/Changed.
- [ ] ~~`pkg/ze/bus.go` - Bus interface~~ (superseded: bus removed, use stream system)
  -> ~~Constraint: Publish(topic, payload, metadata), Subscribe(prefix, filter, consumer)~~
  -> ~~Constraint: Consumer.Deliver(events []Event) error~~
- [ ] `pkg/plugin/rpc/types.go` (legacy event RPCs) - stream system wire types
  -> Constraint: `SubscribeEventsInput` has Events, Peers, Format, Encoding fields
  -> Constraint: `EmitEventInput` has Namespace, EventType, Direction, PeerAddress, Event
  -> Constraint: Engine validates `(namespace, event-type)` against registry, rejects unknown
- [ ] `internal/component/plugin/server/dispatch.go` - stream dispatch and routing
  -> Constraint: `deliverEvent` finds matching subscribers and delivers; emitter excluded
  -> Constraint: `handleSubscribeEventsRPC` and `handleEmitEventRPC` handle plugin-initiated calls
  -> Constraint: `handleSubscribeEventsDirect` / `handleEmitEventDirect` for in-process via DirectBridge
- [ ] `internal/component/plugin/server/startup.go` - 5-stage startup with tiers
  -> Constraint: DeclareRegistrationInput has Families, Commands, WantsConfig, Schema
  -> Constraint: tier-ordered handshake, barrier per tier
- [ ] `cmd/ze/hub/main.go` - SIGHUP handling
  -> Constraint: handleSIGHUPReload calls s.ReloadFromDisk with 30s timeout

**Behavior to preserve:**
- Auto-stop plugins whose config roots are removed during reload
- Auto-load plugins for newly added config roots
- Plugin startup tier ordering (dependencies)
- Config diff calculation (added/removed/changed per root)
- All existing stream events (BGP, RPKI) and DirectBridge zero-copy hot path
- 5-stage startup protocol (extended, not replaced)

**Behavior to change:**
- Config verify: sequential RPC per plugin replaced with stream event per plugin (concurrent fan-out via `deliverEvent`)
- Config apply: sequential RPC per plugin replaced with stream event per plugin
- Add rollback phase (does not exist today)
- Add transaction exclusion (does not exist today)
- Add SDK journal for rollback support
- Add `WantsConfig` declaration to Stage 1 registration
- Add `VerifyBudget` and `ApplyBudget` to Stage 1 registration
- Config file written after apply success, not before
- Disk write failure is warning, not transaction failure
- Register `config` namespace in the event-type registry (`plugin.RegisterEventTypes` or equivalent) with all transaction event types

## Data Flow (MANDATORY)

### Entry Point
- Config change enters via: CLI editor commit, API ConfigCommit, SIGHUP reload
- Format: candidate config tree (map of config roots to subtrees)

### Transformation Path
1. Engine diffs candidate against running config
2. Engine auto-stops/auto-loads plugins for removed/added roots
3. Engine emits stream event `(namespace=config, event-type=verify-<plugin>)` per plugin with filtered diffs
4. Plugins validate, emit `(config, verify-ok)` + apply budget estimate (with peer/key field carrying plugin name for engine routing)
5. Engine computes apply deadline from max plugin estimate
6. Engine emits `(config, apply-<plugin>)` per plugin with filtered diffs
7. Plugins apply, may wait for side-effect events, emit `(config, apply-ok)`
8. Engine emits `(config, committed)` (journals discarded)
9. Engine writes config file (best effort, warning on failure)
10. Engine emits `(config, applied)` (observers notified)

Note: per-plugin event types (`verify-<plugin>`, `apply-<plugin>`) are
registered dynamically as plugins start. Alternative: a single `verify` event
type with peer-address field carrying plugin name, subscribed by all and
filtered locally. The orchestrator chooses; the protocol semantics are the same.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine -> Plugin (verify/apply) | Per-plugin stream event with JSON payload, dispatched via `deliverEvent` | [ ] |
| Plugin -> Engine (ack/fail) | Stream event in `config` namespace, engine subscribes | [ ] |
| Plugin -> Plugin (side effects) | Existing stream events (interface/created, bgp/state, etc.) | [ ] |
| Engine -> Observers (notification) | Stream event (`config`, `applied`/`rolled-back`), observers subscribe | [ ] |

### Integration Points
- Stream system (`internal/component/plugin/server/dispatch.go`) - all transaction events flow through existing pub/sub
- `plugin/server/reload.go` - replaced by stream-based orchestrator
- `plugin/sdk/sdk_callbacks.go` - extended with OnConfigRollback, journal, budgets
- `plugin/rpc/types.go` - extended with new message types
- `plugin/server/startup.go` - Stage 1 extended with WantsConfig, budgets
- `cmd/ze/hub/main.go` - SIGHUP handler updated to use new orchestrator
- Event type registry - `config` namespace registered with all transaction event types

### Architectural Verification
- [ ] No bypassed layers (all config changes go through transaction protocol)
- [ ] No unintended coupling (plugins communicate via stream events, not direct calls)
- [ ] No duplicated functionality (replaces reload.go RPC, does not add alongside)
- [ ] DirectBridge zero-copy preserved (internal plugins use the same hot path as BGP events)
- [ ] No bus dependency (`pkg/ze/bus.go` not imported by transaction code)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| SIGHUP with config change | -> | txLock acquired, TxCoordinator emits stream events | `test/reload/test-tx-protocol-sighup.ci` |
| Plugin verify-ok with apply budget | -> | Engine computes deadline, emits apply event | `TestOrchestratorVerifyToApply` |
| Plugin apply-failed | -> | Engine emits rollback event, all plugins undo | `TestOrchestratorRollbackOnFailure` |
| Transaction in progress + second commit | -> | Second commit rejected, SIGHUP queued | `TestTxLockSIGHUPQueuing` (reload_test.go) |
| Journal Record + Rollback | -> | Undo functions called in reverse order | `TestJournalRollback` |
| External Python plugin in transaction | -> | Plugin receives verify event over TLS via stream system, acks via emit-event | `test/reload/test-tx-protocol-external-plugin.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config change triggers transaction | Engine emits stream event `(config, verify-<plugin>)` per plugin with only that plugin's relevant diffs |
| AC-2 | All plugins verify-ok | Engine emits `(config, apply-<plugin>)` per plugin with deadline from max apply budget |
| AC-3 | Any plugin verify-failed | Engine emits `(config, verify-abort)`, no apply sent |
| AC-4 | All plugins apply-ok | Engine emits `(config, committed)`, writes config file, emits `(config, applied)` |
| AC-5 | Any plugin apply-failed | Engine emits `(config, rollback)`, collects `(config, rollback-ok)` from all |
| AC-6 | rollback-ok with code `broken` | Engine restarts plugin via 5-stage protocol. Second `broken` stops plugin. |
| AC-7 | Config file write fails after apply | `(config, applied)` emitted with `saved: false`. Runtime is live. No rollback. |
| AC-8 | Concurrent commit while transaction active | Second commit rejected with error. SIGHUP queued. |
| AC-9 | Plugin declares `WantsConfig: ["bgp"]` | Plugin receives BGP diffs during verify/apply without owning bgp root |
| AC-10 | Plugin estimates apply budget in verify-ok | Engine uses max across all plugins as apply deadline |
| AC-11 | Plugin updates budgets in apply-ok | Engine stores updated budgets for next transaction |
| AC-12 | Journal.Record(apply, undo) called 3 times, then Rollback() | Undo functions called in reverse order (3, 2, 1) |
| AC-13 | Journal.Discard() after `(config, committed)` | Journal cleared, no undo functions callable |
| AC-14 | Plugin with neither ConfigRoots nor WantsConfig | Does not receive any transaction events |
| AC-15 | Verify timeout (plugin does not respond) | Engine emits `(config, verify-abort)` after verify deadline |
| AC-16 | Apply timeout (plugin does not respond) | Engine emits `(config, rollback)` after apply deadline |
| AC-17 | External plugin (Python via TLS) | Receives transaction events via existing stream `subscribe-events`/`deliver-event` RPCs over TLS, same as BGP events. No bus-over-wire infrastructure needed. |
| AC-18 | Internal Go plugin | Receives transaction events via DirectBridge zero-copy hot path, same as BGP events |
| AC-19 | Event type registration | At engine startup, `config` namespace and all transaction event types are registered. Unknown event types in `config` namespace are rejected by `plugin.IsValidEvent`. |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOrchestratorVerifyAllOk` | `config/transaction/orchestrator_test.go` | All verify/ok triggers apply phase | |
| `TestOrchestratorVerifyFailed` | `config/transaction/orchestrator_test.go` | Any verify/failed triggers abort | |
| `TestOrchestratorVerifyTimeout` | `config/transaction/orchestrator_test.go` | Missing verify ack triggers abort | |
| `TestOrchestratorVerifyToApply` | `config/transaction/orchestrator_test.go` | Apply deadline computed from max budget | |
| `TestOrchestratorApplyAllOk` | `config/transaction/orchestrator_test.go` | All apply/ok triggers committed + file write | |
| `TestOrchestratorRollbackOnFailure` | `config/transaction/orchestrator_test.go` | Apply/failed triggers rollback, collects acks | |
| `TestOrchestratorRollbackOnTimeout` | `config/transaction/orchestrator_test.go` | Apply deadline exceeded triggers rollback | |
| `TestOrchestratorBrokenRecovery` | `config/transaction/orchestrator_test.go` | Broken code triggers plugin restart | |
| `TestOrchestratorFileWriteFailure` | `config/transaction/orchestrator_test.go` | Write failure produces applied with saved=false | |
| ~~`TestTransactionExclusion`~~ `TestTxLockSIGHUPQueuing` | `plugin/server/reload_test.go` | Concurrent commit rejected, SIGHUP queued + drained | |
| ~~`TestTransactionExclusionSIGHUPFires`~~ | (merged into TestTxLockSIGHUPQueuing) | Queued SIGHUP fires after transaction completes | |
| `TestPerPluginDiffFiltering` | `config/transaction/orchestrator_test.go` | Plugin receives only declared roots | |
| `TestWantsConfigDiffDelivery` | `config/transaction/orchestrator_test.go` | WantsConfig plugin receives other plugin's diffs | |
| `TestBudgetUpdatesStored` | `config/transaction/orchestrator_test.go` | Updated budgets from apply/ok used for next tx | |
| `TestJournalRecord` | `plugin/sdk/journal_test.go` | Record calls apply, stores undo | |
| `TestJournalRollback` | `plugin/sdk/journal_test.go` | Rollback calls undos in reverse order | |
| `TestJournalRollbackContinuesOnError` | `plugin/sdk/journal_test.go` | One undo fails, remaining still called | |
| `TestJournalDiscard` | `plugin/sdk/journal_test.go` | Discard clears journal, Rollback is no-op after | |
| `TestJournalRecordApplyFails` | `plugin/sdk/journal_test.go` | Failed apply does not store undo | |
| `TestConfigTopicConstants` | `config/transaction/topics_test.go` | All topic strings match expected hierarchy | |
| `TestRegistrationWantsConfig` | `plugin/sdk/sdk_test.go` | WantsConfig declared in Stage 1 | |
| `TestRegistrationBudgets` | `plugin/sdk/sdk_test.go` | VerifyBudget and ApplyBudget in Stage 1 | |
| `TestOnConfigRollbackCallback` | `plugin/sdk/sdk_test.go` | SDK dispatches rollback to registered handler | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| verify-budget | 0s - 600s | 600s | N/A (0 = trivial) | 601s (capped) |
| apply-budget | 0s - 600s | 600s | N/A (0 = trivial) | 601s (capped) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-tx-protocol-sighup` | `test/reload/test-tx-protocol-sighup.ci` | SIGHUP triggers verify/apply cycle, config applied | |
| `test-tx-protocol-rollback` | `test/reload/test-tx-protocol-rollback.ci` | Plugin apply fails, all plugins rolled back | |
| `test-tx-protocol-exclusion` | `test/reload/test-tx-protocol-exclusion.ci` | Concurrent reload during transaction, second queued | |

### Future (if deferring any tests)
- Property test: random sequences of verify/apply/rollback with injected failures
- Per-plugin wiring tests (separate specs for BGP, iface, sysrib, fib consumers)

## Files to Modify
- `internal/component/plugin/server/reload.go` - replace sequential RPC verify/apply with stream-based orchestrator call
- `internal/component/plugin/server/startup.go` - Stage 1: add WantsConfig, VerifyBudget, ApplyBudget
- `internal/component/plugin/server/dispatch.go` - register `config` namespace and transaction event types at engine startup
- `internal/component/config/transaction/orchestrator.go` (committed) - rewrite pub/sub layer to use stream emit/subscribe instead of bus.Publish/Subscribe; orchestration logic preserved
- `internal/component/config/transaction/topics.go` (committed) - replace topic constants with namespace + event-type constants
- `internal/component/config/transaction/types.go` (committed) - event payload types stay (still JSON), references to bus-specific concepts removed
- ~~`pkg/plugin/sdk/sdk.go`~~ Not needed: Registration is a type alias for rpc.DeclareRegistrationInput, budget fields added to rpc/types.go propagate automatically
- `pkg/plugin/sdk/sdk_callbacks.go` - add OnConfigRollback callback, extend OnConfigVerify/OnConfigApply to return budget updates
- `pkg/plugin/rpc/types.go` - new message types for transaction events
- `cmd/ze/hub/main.go` - SIGHUP handler uses new orchestrator
- `docs/architecture/config/transaction-protocol.md` - REWRITE: bus-based design must be re-stated in stream-system terms
- `pkg/ze/bus.go` - eventually deleted once no consumers remain (separate cleanup)

## Files to Create
~~The following were created in earlier (bus-based) implementation. They exist
and need their pub/sub layer rewritten, not recreation:~~
- ~~`internal/component/config/transaction/orchestrator.go`~~ (exists, rewrite pub/sub layer)
- ~~`internal/component/config/transaction/orchestrator_test.go`~~ (exists, rewrite test setup)
- ~~`internal/component/config/transaction/topics.go`~~ (exists, becomes event-type constants)
- ~~`internal/component/config/transaction/topics_test.go`~~ (exists, update assertions)
- ~~`internal/component/config/transaction/types.go`~~ (exists, mostly unchanged)
- ~~`pkg/plugin/sdk/journal.go`~~ (exists, add tx ID parameter)
- ~~`pkg/plugin/sdk/journal_test.go`~~ (exists, update assertions)

Still to create:
- `test/reload/test-tx-protocol-sighup.ci` - functional test: SIGHUP transaction
- `test/reload/test-tx-protocol-rollback.ci` - functional test: rollback
- `test/reload/test-tx-protocol-exclusion.ci` - functional test: exclusion
- `test/reload/test-tx-protocol-external-plugin.ci` - functional test: external Python plugin participates via stream over TLS

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | |
| CLI commands/flags | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | Yes | `test/reload/test-tx-protocol-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/config/transaction-protocol.md` (already written), `docs/architecture/plugin-manager-wiring.md` (Stage 1 changes) |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (config transaction section) |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + transaction-protocol.md |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

**Note (2026-04-07):** Phases 1-3, 5, 6, 8 were completed in earlier commits
(559b3e9b, 9b0a7ea9, 7c5f8d89) against the bus-based design. With the stream
pivot, phases 1, 4, and 7 need rework. The journal, SDK callbacks, and Stage 1
changes are largely intact.

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Event-type constants and registration** (REWORK from bus topics)
   - Replace `topics.go` topic constants with namespace + event-type constants
   - Register `config` namespace in event-type registry at engine startup
   - Tests: `TestConfigEventTypeConstants`, `TestConfigNamespaceRegistered`
   - Files: `transaction/topics.go` (rename or repurpose), `dispatch.go` (register namespace)
   - Verify: tests fail -> implement -> tests pass

2. **Phase: SDK journal** (DONE in earlier commit, may need tx ID parameter)
   - Tests: `TestJournalRecord`, `TestJournalRollback`, `TestJournalRollbackContinuesOnError`, `TestJournalDiscard`, `TestJournalRecordApplyFails`
   - Files: `sdk/journal.go`
   - Status: existing, verify tx ID handling

3. **Phase: SDK registration and callbacks** (PARTIAL: WantsConfig and budgets done; need to extend OnConfigVerify/Apply to return budget updates)
   - Tests: `TestRegistrationWantsConfig`, `TestRegistrationBudgets`, `TestOnConfigRollbackCallback`, `TestOnConfigVerifyReturnsBudget`, `TestOnConfigApplyReturnsBudget`
   - Files: `sdk/sdk.go`, `sdk/sdk_callbacks.go`, `rpc/types.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Transaction orchestrator** (REWORK: pub/sub layer changes from bus to stream)
   - Replace `bus.Publish`/`bus.Subscribe` calls with stream `emitEvent`/`subscribeEvents` calls
   - Orchestration logic (state machine, deadline computation, ack collection) preserved
   - Add reverse-tier rollback ordering using `registry.TopologicalTiers`
   - Add dependency-graph-aware deadline (sum chains, max independent)
   - Tests: existing orchestrator tests, updated for stream system; new tests for tier ordering and dependency-graph deadline
   - Files: `transaction/orchestrator.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Transaction exclusion** (DONE in earlier commit)
   - Status: txLock + SIGHUP queuing already in `reload.go`

6. **Phase: Broken plugin recovery** (DONE in earlier commit, restartFn needs wiring to plugin manager)
   - Files: `transaction/orchestrator.go`, `plugin/server/startup.go`
   - Status: framework exists, restartFn currently nil-callable

7. **Phase: Wire into reload and SIGHUP** (NOT STARTED)
   - Replace sequential RPC verify/apply loop in `reload.go` with `TxCoordinator.Execute()`
   - Build participant list from process manager registrations
   - Convert `configDiff` to transaction package's diff format
   - Tests: functional tests (`test-tx-protocol-*.ci`)
   - Files: `plugin/server/reload.go`, `cmd/ze/hub/main.go`
   - Verify: tests fail -> implement -> tests pass

8. **Phase: Stage 1 registration changes** (DONE in earlier commit)
   - Status: WantsConfig, VerifyBudget, ApplyBudget already in `DeclareRegistrationInput`

9. **Phase: Doc rewrite** (NEW)
   - `docs/architecture/config/transaction-protocol.md` rewritten in stream-system terms
   - Old bus-based content struck through with reason
   - References to `pkg/ze/bus.go` removed; references to `dispatch.go` added

10. **Phase: Bus interface deletion** (NEW)
    - After all transaction code uses stream system, delete `pkg/ze/bus.go`
    - Update `arch-0` learned summary to reflect 4-component model
    - Verify no remaining imports of `pkg/ze.Bus`

11. **Full verification** -> `make ze-verify`
12. **Complete spec** -> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Rollback order is reverse-tier (from `registry.TopologicalTiers`). File write only after all apply/ok. |
| Naming | Event types follow `(config, <verb>)` pattern. Types follow ze naming conventions. |
| Data flow | Engine -> per-plugin event type (filtered diffs). Plugin -> broadcast ack event. Never full config to all. |
| Rule: no-layering | Old sequential RPC verify/apply in reload.go fully replaced, not kept alongside. Bus interface fully removed, not kept alongside stream. |
| Rule: init-time budgets | Initial budgets set at registration (Stage 1), updated after each phase |
| Rule: stream over bus | No imports of `pkg/ze.Bus` in transaction code. All pub/sub via `subscribeEvents`/`emitEvent`/`deliverEvent`. |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| Transaction orchestrator | `grep "type TxCoordinator" internal/component/config/transaction/orchestrator.go` |
| Config namespace registered | `grep "NamespaceConfig\|\"config\"" internal/component/plugin/server/dispatch.go` |
| Config event types defined | `grep "EventVerify\|EventApply\|EventRollback" internal/component/config/transaction/topics.go` |
| Event payload types | `grep "type VerifyEvent\|type ApplyEvent" internal/component/config/transaction/types.go` |
| SDK journal | `grep "type Journal" pkg/plugin/sdk/journal.go` |
| OnConfigRollback callback | `grep "OnConfigRollback" pkg/plugin/sdk/sdk_callbacks.go` |
| WantsConfig in registration | `grep "WantsConfig" pkg/plugin/rpc/types.go` |
| Budget fields in registration | `grep "VerifyBudget\|ApplyBudget" pkg/plugin/rpc/types.go` |
| reload.go uses TxCoordinator | `grep "TxCoordinator\|Execute" internal/component/plugin/server/reload.go` |
| No bus imports in transaction | `grep -c "pkg/ze.Bus\|ze.Subscribe\|ze.Publish" internal/component/config/transaction/` returns 0 |
| Reverse-tier rollback | `grep "TopologicalTiers\|tier" internal/component/config/transaction/orchestrator.go` |
| Functional tests exist | `ls test/reload/test-tx-protocol-*.ci` |
| Bus interface deleted | `ls pkg/ze/bus.go` returns "No such file" (after Phase 10) |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Plugin-provided budgets capped at 600s. Transaction ID uniqueness. |
| Resource exhaustion | Rollback loop bounded by number of participants. Journal entries bounded by diff size. |
| Transaction lock | Lock released on all exit paths (success, rollback, engine error). No deadlock on panic. |
| Config scoping | Plugin never receives diffs for roots it did not declare. Verify in orchestrator. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design Decisions

### ~~Bus events~~ Stream events replace RPC for config verify/apply/rollback (REVISED 2026-04-07)

~~Direct RPC calls in reload.go are replaced with bus events. This unifies config
transactions with the same pub/sub mechanism used for all other cross-component
coordination. Plugins participate by subscribing to bus topics.~~

Direct RPC calls in reload.go are replaced with stream events in a new `config`
namespace. This unifies config transactions with the same pub/sub mechanism
used for BGP and RPKI events. Plugins participate by subscribing to event types
in the `config` namespace via the existing `subscribe-events` RPC. The bus
(`pkg/ze/bus.go`) was found to be premature abstraction and is removed; the
stream system already provides everything needed (fan-out, schema validation,
DirectBridge zero-copy, external plugin participation).

Design doc: `docs/architecture/config/transaction-protocol.md` (NEEDS REWRITE)

### ~~Per-plugin topics~~ Per-plugin event types for engine-to-plugin, broadcast event types for plugin-to-engine

~~Engine publishes to `config/verify/<plugin>` with only that plugin's relevant
diffs. Config scoping is enforced by the routing layer. Plugin responses
(`config/verify/ok`, etc.) are broadcast -- engine subscribes and collects.~~

Engine emits per-plugin event types `(config, verify-<plugin>)` and `(config,
apply-<plugin>)`. Each plugin subscribes to its own event types and receives
only its filtered diffs. Plugin responses use broadcast event types `(config,
verify-ok)`, `(config, apply-ok)`, etc. with the plugin name carried in the
peer-address field for engine routing. Engine subscribes to all `(config, *-ok)`
and `(config, *-failed)` events and demultiplexes by transaction ID and plugin
name.

Alternative considered: a single `(config, verify)` event type subscribed by
all participating plugins, with plugin filtering done locally by each plugin.
This trades more events on the wire for simpler registry. The orchestrator
implementation chooses; the protocol semantics are identical.

### Config file written after all apply/ok, disk failure is warning

Runtime is authoritative. The config file is a persistence artifact written
after `config/committed`. If the file write fails, the runtime is still
correct. The caller gets a warning (`saved: false` in `config/applied`).

### Plugin-estimated timeouts, updated after each phase

Plugins provide initial verify and apply budgets at registration. After each
verify, the plugin updates its apply budget. After each apply, the plugin
updates both budgets. The engine uses the max across all participants.
Self-correcting: underestimate -> timeout -> rollback -> retry with better estimate.

### ConfigRoots and WantsConfig are separate declarations

`ConfigRoots` = schema authority + receives diffs. `WantsConfig` = read-only
access to another plugin's config diffs. A DHCP plugin reads interface config
without owning the interface root.

### Transaction exclusion with SIGHUP queuing

One transaction at a time. CLI/API commits rejected during active transaction.
SIGHUP queued (not rejected) because the operator expects reload to happen.

### SDK journal for rollback support

Plugins use `journal.Record(apply, undo)` during apply. On rollback, the
journal replays undos in reverse. On `(config, committed)`, the journal is
discarded. Plugins that don't need rollback ignore the journal.

### Broken plugin recovery via 5-stage restart

A plugin that reports `broken` (can't undo cleanly) is restarted once via
the 5-stage startup protocol. It receives full config in Stage 2 and rebuilds
from scratch. Second `broken` stops the plugin.

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Go callbacks (ConfigHandler with ApplyFn/RollbackFn) | Only works for internal components, not external plugins over RPC | Stream-based protocol works for all plugins |
| Config file write before apply | Failed apply leaves file/runtime diverged | Write after apply, failure is warning |
| Mid-transaction extension mechanism | Over-complex, unnecessary | Timeout + rollback + retry with better estimate |
| Bus-based protocol (`pkg/ze/bus.go`, content-agnostic pub/sub) | Premature abstraction. Stream system already provides all required capabilities (fan-out, schema validation, DirectBridge zero-copy, external plugin support over TLS). No concrete consumer requires the bus that the stream system cannot serve. Adding the bus alongside the stream creates parallel infrastructure with no benefit. | Stream system in `dispatch.go`, with config events in a new `config` namespace |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation
N/A - internal architecture, not protocol work.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

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
- **Partial:**
- **Skipped:**
- **Changed:**

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

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-16 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-config-tx-protocol.md`
- [ ] Summary included in commit

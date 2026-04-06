# Spec: Config Transaction Protocol

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 8/8 |
| Updated | 2026-04-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/config/transaction-protocol.md` - full protocol design
4. `docs/architecture/plugin/plugin-relationships.md` - plugin dependency map
5. `internal/component/plugin/server/reload.go` - current verify/apply (being replaced)
6. `pkg/plugin/sdk/sdk_callbacks.go` - current SDK callbacks

## Task

Replace ze's RPC-based config reload with a bus-based transaction protocol.

Today config reload (`SIGHUP`, CLI commit, API) uses direct RPC calls per plugin
for verify and apply. There is no rollback -- if a plugin fails during apply,
previously applied plugins keep their changes. There is no transaction exclusion --
concurrent commits are not prevented. Plugins cannot declare interest in config
roots they don't own.

The goal: a bus-based protocol where verify, apply, rollback, and finalization are
events. Plugins opt in via `ConfigRoots` (ownership) and `WantsConfig` (read-only
interest). The engine orchestrates deadlines from plugin estimates, enforces
transaction exclusion, and provides an SDK journal for rollback support.

**Replaces:** `spec-config-apply-rollback.md` (Go callback approach, never implemented).

**Does NOT include:** per-plugin wiring (reconcilePeers, iface apply, etc.) -- those
are separate follow-up specs that consume this protocol.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/transaction-protocol.md` - full protocol design (written during this design session)
  -> Decision: bus events, not Go callbacks or RPC
  -> Decision: per-plugin topics (engine->plugin), broadcast (plugin->engine)
  -> Decision: config file written after all apply/ok, failure is warning not rollback
  -> Decision: plugin-estimated timeouts, updated after each phase
  -> Constraint: one transaction at a time, SIGHUP queued
- [ ] `docs/architecture/plugin/plugin-relationships.md` - plugin dependency map
  -> Constraint: only 5 plugins own config roots today (bgp, interface, sysrib, fib-kernel, fib-p4)
  -> Constraint: BGP sub-plugins mediated by reactor, no direct config participation
  -> Constraint: no plugin currently uses WantsConfig
- [ ] `docs/architecture/core-design.md` - bus architecture
  -> Constraint: bus is content-agnostic, `[]byte` payloads, hierarchical topics with `/` separator
  -> Constraint: bus metadata for subscription filtering
- [ ] `docs/architecture/config/yang-config-design.md` - YANG config system
  -> Constraint: validators are init-time write, read-only after startup
- [ ] `docs/architecture/plugin-manager-wiring.md` - 5-stage startup protocol
  -> Constraint: Stage 1 declares ConfigRoots, WantsConfig. Stage 2 delivers config.

### RFC Summaries (MUST for protocol work)
N/A - internal architecture, not protocol work.

**Key insights:**
- Current reload in `reload.go` is sequential verify then concurrent apply via RPC
- Bus already handles cross-component events (interface -> BGP via `interface/addr/added`)
- 5 system plugins own config today; BGP sub-plugins are mediated by reactor
- `WantsConfig` is a new declaration; no plugin uses it yet
- Config file write is decoupled from transaction success

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
- [ ] `pkg/ze/bus.go` - Bus interface
  -> Constraint: Publish(topic, payload, metadata), Subscribe(prefix, filter, consumer)
  -> Constraint: Consumer.Deliver(events []Event) error
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
- All existing bus topics and subscriptions (interface, bgp, rib, sysrib, fib)
- 5-stage startup protocol (extended, not replaced)

**Behavior to change:**
- Config verify: RPC per plugin replaced with bus event per plugin
- Config apply: RPC per plugin replaced with bus event per plugin
- Add rollback phase (does not exist today)
- Add transaction exclusion (does not exist today)
- Add SDK journal for rollback support
- Add `WantsConfig` declaration to Stage 1 registration
- Add `VerifyBudget` and `ApplyBudget` to Stage 1 registration
- Config file written after apply success, not before
- Disk write failure is warning, not transaction failure

## Data Flow (MANDATORY)

### Entry Point
- Config change enters via: CLI editor commit, API ConfigCommit, SIGHUP reload
- Format: candidate config tree (map of config roots to subtrees)

### Transformation Path
1. Engine diffs candidate against running config
2. Engine auto-stops/auto-loads plugins for removed/added roots
3. Engine publishes per-plugin `config/verify/<plugin>` with filtered diffs
4. Plugins validate, respond with `config/verify/ok` + apply budget estimate
5. Engine computes apply deadline from max plugin estimate
6. Engine publishes per-plugin `config/apply/<plugin>` with filtered diffs
7. Plugins apply, may wait for side-effect events, respond with `config/apply/ok`
8. Engine publishes `config/committed` (journals discarded)
9. Engine writes config file (best effort, warning on failure)
10. Engine publishes `config/applied` (observers notified)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine -> Plugin (verify/apply) | Per-plugin bus topic with JSON payload | [ ] |
| Plugin -> Engine (ack/fail) | Broadcast bus topic with JSON payload | [ ] |
| Plugin -> Plugin (side effects) | Existing bus topics (interface/created, etc.) | [ ] |
| Engine -> Observers (notification) | Broadcast bus topic | [ ] |

### Integration Points
- `Bus` (pkg/ze/bus.go) - all transaction events flow through existing bus
- `plugin/server/reload.go` - replaced by bus-based orchestrator
- `plugin/sdk/sdk_callbacks.go` - extended with OnConfigRollback, journal, budgets
- `plugin/rpc/types.go` - extended with new message types
- `plugin/server/startup.go` - Stage 1 extended with WantsConfig, budgets
- `cmd/ze/hub/main.go` - SIGHUP handler updated to use new orchestrator

### Architectural Verification
- [ ] No bypassed layers (all config changes go through transaction protocol)
- [ ] No unintended coupling (plugins communicate via bus, not direct calls)
- [ ] No duplicated functionality (replaces reload.go RPC, does not add alongside)
- [ ] Zero-copy preserved where applicable (bus payloads are JSON, no wire data)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| SIGHUP with config change | -> | txLock acquired, config reload via RPC path | `test/reload/test-tx-protocol-sighup.ci` |
| Plugin verify/ok with apply budget | -> | Engine computes deadline, publishes apply | `TestOrchestratorVerifyToApply` |
| Plugin apply/failed | -> | Engine publishes rollback, all plugins undo | `TestOrchestratorRollbackOnFailure` |
| Transaction in progress + second commit | -> | Second commit rejected, SIGHUP queued | `TestTxLockSIGHUPQueuing` (reload_test.go) |
| Journal Record + Rollback | -> | Undo functions called in reverse order | `TestJournalRollback` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config change triggers transaction | Engine publishes per-plugin `config/verify/<plugin>` with only that plugin's relevant diffs |
| AC-2 | All plugins verify/ok | Engine publishes per-plugin `config/apply/<plugin>` with deadline from max apply budget |
| AC-3 | Any plugin verify/failed | Engine publishes `config/verify/abort`, no apply sent |
| AC-4 | All plugins apply/ok | Engine publishes `config/committed`, writes config file, publishes `config/applied` |
| AC-5 | Any plugin apply/failed | Engine publishes `config/rollback`, collects rollback/ok from all |
| AC-6 | Rollback/ok with code `broken` | Engine restarts plugin via 5-stage protocol. Second `broken` stops plugin. |
| AC-7 | Config file write fails after apply | `config/applied` published with `saved: false`. Runtime is live. No rollback. |
| AC-8 | Concurrent commit while transaction active | Second commit rejected with error. SIGHUP queued. |
| AC-9 | Plugin declares `WantsConfig: ["bgp"]` | Plugin receives BGP diffs during verify/apply without owning bgp root |
| AC-10 | Plugin estimates apply budget in verify/ok | Engine uses max across all plugins as apply deadline |
| AC-11 | Plugin updates budgets in apply/ok | Engine stores updated budgets for next transaction |
| AC-12 | Journal.Record(apply, undo) called 3 times, then Rollback() | Undo functions called in reverse order (3, 2, 1) |
| AC-13 | Journal.Discard() after config/committed | Journal cleared, no undo functions callable |
| AC-14 | Plugin with neither ConfigRoots nor WantsConfig | Does not receive any transaction events |
| AC-15 | Verify timeout (plugin does not respond) | Engine publishes verify/abort after verify deadline |
| AC-16 | Apply timeout (plugin does not respond) | Engine publishes config/rollback after apply deadline |

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
- `internal/component/plugin/server/reload.go` - replace RPC verify/apply with bus orchestrator call
- `internal/component/plugin/server/startup.go` - Stage 1: add WantsConfig, VerifyBudget, ApplyBudget
- ~~`pkg/plugin/sdk/sdk.go`~~ Not needed: Registration is a type alias for rpc.DeclareRegistrationInput, budget fields added to rpc/types.go propagate automatically
- `pkg/plugin/sdk/sdk_callbacks.go` - add OnConfigRollback callback
- `pkg/plugin/rpc/types.go` - new message types for transaction events
- `cmd/ze/hub/main.go` - SIGHUP handler uses new orchestrator
- `docs/architecture/config/transaction-protocol.md` - update if design changes during implementation

## Files to Create
- `internal/component/config/transaction/orchestrator.go` - transaction state machine and bus orchestration
- `internal/component/config/transaction/orchestrator_test.go` - orchestrator unit tests
- `internal/component/config/transaction/topics.go` - config/ topic constants
- `internal/component/config/transaction/topics_test.go` - topic constant tests
- `internal/component/config/transaction/types.go` - transaction event payload types
- `pkg/plugin/sdk/journal.go` - apply journal library
- `pkg/plugin/sdk/journal_test.go` - journal unit tests
- `test/reload/test-tx-protocol-sighup.ci` - functional test: SIGHUP transaction
- `test/reload/test-tx-protocol-rollback.ci` - functional test: rollback
- `test/reload/test-tx-protocol-exclusion.ci` - functional test: exclusion

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

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Topic constants and event types** -- define config/ topic hierarchy and payload types
   - Tests: `TestConfigTopicConstants`
   - Files: `transaction/topics.go`, `transaction/types.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: SDK journal** -- apply journal library for plugins
   - Tests: `TestJournalRecord`, `TestJournalRollback`, `TestJournalRollbackContinuesOnError`, `TestJournalDiscard`, `TestJournalRecordApplyFails`
   - Files: `sdk/journal.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: SDK registration and callbacks** -- WantsConfig, budgets, OnConfigRollback
   - Tests: `TestRegistrationWantsConfig`, `TestRegistrationBudgets`, `TestOnConfigRollbackCallback`
   - Files: `sdk/sdk.go`, `sdk/sdk_callbacks.go`, `rpc/types.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Transaction orchestrator** -- state machine, bus publish/subscribe, deadline management
   - Tests: `TestOrchestratorVerifyAllOk`, `TestOrchestratorVerifyFailed`, `TestOrchestratorVerifyTimeout`, `TestOrchestratorVerifyToApply`, `TestOrchestratorApplyAllOk`, `TestOrchestratorRollbackOnFailure`, `TestOrchestratorRollbackOnTimeout`, `TestOrchestratorFileWriteFailure`, `TestPerPluginDiffFiltering`, `TestWantsConfigDiffDelivery`, `TestBudgetUpdatesStored`
   - Files: `transaction/orchestrator.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Transaction exclusion** -- lock, SIGHUP queuing
   - Tests: `TestTransactionExclusion`, `TestTransactionExclusionSIGHUPFires`
   - Files: `transaction/orchestrator.go`
   - Verify: tests fail -> implement -> tests pass

6. **Phase: Broken plugin recovery** -- restart via 5-stage on broken code
   - Tests: `TestOrchestratorBrokenRecovery`
   - Files: `transaction/orchestrator.go`, `plugin/server/startup.go`
   - Verify: tests fail -> implement -> tests pass

7. **Phase: Wire into reload and SIGHUP** -- replace reload.go RPC path, update hub
   - Tests: functional tests (`test-tx-protocol-*.ci`)
   - Files: `plugin/server/reload.go`, `cmd/ze/hub/main.go`
   - Verify: tests fail -> implement -> tests pass

8. **Phase: Stage 1 registration changes** -- WantsConfig, budgets in startup protocol
   - Tests: existing startup tests must pass, new registration fields verified
   - Files: `plugin/server/startup.go`
   - Verify: tests fail -> implement -> tests pass

9. **Full verification** -> `make ze-verify`
10. **Complete spec** -> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Rollback order is strictly reverse. File write only after all apply/ok. |
| Naming | Topics follow `config/` hierarchy. Types follow ze naming conventions. |
| Data flow | Engine -> per-plugin topic (filtered diffs). Plugin -> broadcast ack. Never full config to all. |
| Rule: no-layering | Old RPC verify/apply in reload.go fully replaced, not kept alongside |
| Rule: init-time budgets | Initial budgets set at registration (Stage 1), updated after each phase |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| Transaction orchestrator | `grep "type Orchestrator" internal/component/config/transaction/orchestrator.go` |
| Config topic constants | `grep "TopicVerify\|TopicApply\|TopicRollback" internal/component/config/transaction/topics.go` |
| Event payload types | `grep "type VerifyEvent\|type ApplyEvent" internal/component/config/transaction/types.go` |
| SDK journal | `grep "type Journal" pkg/plugin/sdk/journal.go` |
| OnConfigRollback callback | `grep "OnConfigRollback" pkg/plugin/sdk/sdk_callbacks.go` |
| WantsConfig in registration | `grep "WantsConfig" pkg/plugin/sdk/sdk.go` |
| Budget fields in registration | `grep "VerifyBudget\|ApplyBudget" pkg/plugin/sdk/sdk.go` |
| reload.go uses orchestrator | `grep "Orchestrator\|orchestrator" internal/component/plugin/server/reload.go` |
| Functional tests exist | `ls test/reload/test-tx-protocol-*.ci` |

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

### Bus events replace RPC for config verify/apply/rollback

Direct RPC calls in reload.go are replaced with bus events. This unifies config
transactions with the same pub/sub mechanism used for all other cross-component
coordination. Plugins participate by subscribing to bus topics.

Design doc: `docs/architecture/config/transaction-protocol.md`

### Per-plugin topics for engine-to-plugin, broadcast for plugin-to-engine

Engine publishes to `config/verify/<plugin>` with only that plugin's relevant
diffs. Config scoping is enforced by the routing layer. Plugin responses
(`config/verify/ok`, etc.) are broadcast -- engine subscribes and collects.

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
journal replays undos in reverse. On `config/committed`, the journal is
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
| Go callbacks (ConfigHandler with ApplyFn/RollbackFn) | Only works for internal components, not external plugins over RPC | Bus-based protocol works for all plugins |
| Config file write before apply | Failed apply leaves file/runtime diverged | Write after apply, failure is warning |
| Mid-transaction extension mechanism | Over-complex, unnecessary | Timeout + rollback + retry with better estimate |

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

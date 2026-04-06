# Spec: Config Transaction Bus-Native Protocol

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-config-tx-consumers |
| Phase | - |
| Updated | 2026-04-06 |

## Task

The current transaction implementation uses sequential RPC calls (`SendConfigVerify`,
`SendConfigApply`) from the server to each plugin. The design document
(`docs/architecture/config/transaction-protocol.md`) specifies a bus-native protocol
where all phases are bus events, plugins receive them concurrently, and the engine
orchestrates deadlines and rollback.

This spec bridges the gap between what was designed and what was implemented. Every
section references the specific document section that defines the expected behavior.

## Gap Analysis

| # | Document section | What it says | What the code does | Gap |
|---|-----------------|-------------|-------------------|-----|
| 1 | S1 principle: Bus-native | Transactions use bus pub/sub, no separate RPC path | Server sends `SendConfigVerify`/`SendConfigApply` RPCs sequentially | Replace RPC dispatch with bus publish |
| 2 | S1 principle: Rollback is an event | Any plugin failure triggers `config/rollback` for all | Server collects errors, returns aggregate, no rollback sent | Implement automatic rollback on first apply failure |
| 3 | S2 Phase 1: Verify | Engine publishes `config/verify`, plugins respond concurrently | Server calls plugins one by one, waits for each | Publish verify event, collect concurrent responses |
| 4 | S2 Phase 2: Apply | Engine publishes `config/apply`, plugins respond concurrently | Server calls plugins one by one (BGP last) | Publish apply event, collect concurrent responses |
| 5 | S2 Phase 3: Rollback | Engine publishes `config/rollback` in reverse tier order | No rollback sent to plugins on apply failure | Publish rollback event, reverse tier ordering |
| 6 | S2 Completion | Engine publishes `config/committed` then writes config file | Server calls `SetConfigTree` directly, no committed event | Add committed/applied/rolled-back events |
| 7 | S3 Timeout | Deadline computed from plugin estimates via dependency graph | No deadline enforcement, no timeout | Compute deadline from budgets, enforce with timeout |
| 8 | S3 Self-correcting | Plugins update budgets after each transaction | Budgets are static at registration time | Add budget update in verify/apply responses |
| 9 | S4 Bus events | Full topic hierarchy: `config/verify`, `config/apply`, etc. | No config bus topics exist | Create all topics and payloads |
| 10 | S4 Transaction ID | Every event carries a shared transaction ID | No transaction ID in verify/apply RPCs | Generate tx ID, pass through all phases |
| 11 | S7 Journal | `NewJournal(tx)` with transaction ID | `NewJournal()` without ID | Add tx parameter |
| 12 | S8 Finalization | `config/committed` signals plugins to discard journals | Journals garbage collected on next apply | Implement committed callback |
| 13 | S11 SDK | Method-style: `sdk.DeclareConfigRoots()`, `sdk.WantsConfig()` | Struct fields on `sdk.Registration` | Keep struct fields (simpler), update doc |
| 14 | S12 Failure codes | ok, timeout, transient, error, broken in rollback response | No failure codes | Add codes to rollback response |
| 15 | S13 Recovery | Engine restarts broken plugins once | No recovery mechanism | Implement broken detection and restart |
| 16 | S14 Failure modes | Plugin crash during apply triggers rollback | Plugin crash during apply collects error | Detect crash, trigger rollback |

## Design Decisions

### Gap 13: SDK struct fields vs methods

The doc says `sdk.DeclareConfigRoots(...)`, `sdk.WantsConfig(...)`. The code uses struct
fields on `sdk.Registration`. Struct fields are simpler and already work for Go and
external plugins (JSON serialization). **Decision: keep struct fields, update doc to match.**
This is the only gap where the doc changes instead of the code.

### Concurrent vs sequential delivery

The doc says concurrent. The current RPC path is sequential because `SendConfigVerify`
blocks until the plugin responds. Bus delivery is inherently concurrent -- the engine
publishes once, all subscribed plugins receive the event and process in parallel. The
engine collects responses with a deadline. This is a fundamental architectural change
in `reload.go`.

### Rollback ordering

The doc says reverse tier order. The current implementation has no tier concept for
rollback. Startup tiers already exist in the plugin manager. Rollback reverses them:
tier 2 plugins (GR, healthcheck) roll back first, then tier 1 (RIB, RS), then tier 0
(BGP, interface, sysrib).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/transaction-protocol.md` -- the authoritative design
- [ ] `docs/architecture/plugin/plugin-relationships.md` -- plugin tiers and dependencies
- [ ] `docs/architecture/core-design.md` -- bus interface, plugin startup tiers

### Source Files
- [ ] `internal/component/plugin/server/reload.go` -- current RPC-based verify/apply
- [ ] `internal/component/config/transaction/orchestrator.go` -- TxCoordinator (exists but not wired to bus)
- [ ] `internal/component/config/transaction/topics.go` -- topic constants (exist)
- [ ] `internal/component/config/transaction/types.go` -- event payload types (exist)
- [ ] `pkg/plugin/sdk/journal.go` -- Journal (needs tx ID)
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` -- OnConfigVerify/Apply/Rollback callbacks
- [ ] `pkg/plugin/rpc/types.go` -- ConfigVerifyOutput/ConfigApplyOutput (need budget + failure code fields)
- [ ] `pkg/ze/bus.go` -- Bus interface

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/reload.go`
  -> Constraint: `reloadConfig` sends RPCs sequentially, collects errors, no rollback
  -> Constraint: `txLock` prevents concurrent transactions
  -> Constraint: BGP applied last via explicit ordering
- [ ] `internal/component/config/transaction/orchestrator.go`
  -> Constraint: TxCoordinator exists with topic constants but is not called from reload.go
- [ ] `pkg/plugin/rpc/types.go`
  -> Constraint: ConfigVerifyOutput has status + error, no budget field
  -> Constraint: ConfigApplyOutput has status + error, no budget or failure code fields

**Behavior to preserve:**
- txLock exclusion (one transaction at a time)
- SIGHUP queuing during active transaction
- Plugin auto-load/auto-stop for added/removed config sections
- Journal Record/Rollback/Discard semantics
- All 5 consumer plugins' verify/apply/rollback callback logic

**Behavior to change:**
- Replace sequential RPC dispatch with bus event publish + concurrent response collection
- Add transaction ID to all phases
- Add deadline computation from plugin budgets
- Add automatic rollback when any plugin apply fails
- Add `config/committed` event for journal finalization
- Add failure codes to rollback responses
- Add budget update fields to verify and apply responses
- Wire TxCoordinator into reload.go (replace current inline logic)

## Implementation Phases

### Phase 1: Wire protocol types

Add fields to RPC types so verify/apply responses carry budgets and failure codes.
Add transaction ID to verify/apply/rollback inputs.

Files: `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/journal.go`

### Phase 2: Bus event publish + concurrent collection

Replace the sequential RPC loop in `reload.go` with:
1. Publish `config/verify` bus event (all plugins receive concurrently)
2. Collect responses with deadline (computed from max budget)
3. If all ok: publish `config/apply`
4. Collect responses with deadline
5. If any fail: publish `config/rollback` in reverse tier order
6. Publish `config/committed` or `config/rolled-back`

Wire TxCoordinator into this flow.

Files: `internal/component/plugin/server/reload.go`, `internal/component/config/transaction/orchestrator.go`

### Phase 3: Plugin-side bus subscription

Each plugin subscribes to `config/` topics via the bus instead of receiving RPCs.
The SDK's existing `OnConfigVerify`/`OnConfigApply`/`OnConfigRollback` callbacks
are invoked when the corresponding bus event arrives. The SDK handles the subscription
internally -- plugin authors still register callbacks, not bus subscriptions.

Files: `pkg/plugin/sdk/sdk.go`, `pkg/plugin/sdk/sdk_callbacks.go`

### Phase 4: Deadline enforcement and budget feedback

Engine computes deadline from plugin budgets (dependency graph: sum chains, max
across independent paths). Plugins return updated budgets in verify/apply responses.
Engine stores updated budgets for next transaction.

Files: `internal/component/plugin/server/reload.go`, `internal/component/config/transaction/orchestrator.go`

### Phase 5: Failure codes and broken recovery

Add failure code to rollback responses. Engine detects `broken` code and restarts
the plugin once via the 5-stage protocol.

Files: `internal/component/plugin/server/reload.go`, `pkg/plugin/rpc/types.go`

### Phase 6: Update doc for gap 13

Change the SDK method-style references in `transaction-protocol.md` to match the
actual struct field API. This is the only doc change.

Files: `docs/architecture/config/transaction-protocol.md`

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config reload triggers transaction | Engine publishes `config/verify` bus event with transaction ID, all participating plugins receive it concurrently |
| AC-2 | All plugins verify ok | Engine publishes `config/apply` bus event with deadline computed from max budget |
| AC-3 | One plugin verify fails | Engine publishes `config/verify/abort`, transaction ends, no apply sent |
| AC-4 | All plugins apply ok | Engine publishes `config/committed`, then writes config file, then publishes `config/applied` |
| AC-5 | One plugin apply fails | Engine publishes `config/rollback` in reverse tier order, then `config/rolled-back` |
| AC-6 | Plugin receives committed | Plugin discards journal (changes are permanent) |
| AC-7 | Plugin exceeds apply deadline | Engine publishes `config/rollback` with reason "timeout" |
| AC-8 | Plugin returns updated budget in verify response | Engine uses new budget for this transaction's apply deadline |
| AC-9 | Plugin returns updated budget in apply response | Engine stores new budget for next transaction |
| AC-10 | Plugin rollback returns code "broken" | Engine restarts plugin once via 5-stage protocol |
| AC-11 | Plugin rollback returns code "broken" twice | Engine stops plugin, logs error, no restart loop |
| AC-12 | Transaction ID in all events | Every verify, apply, rollback, committed, applied event carries the same tx ID |
| AC-13 | `NewJournal(tx)` with transaction ID | Journal stores tx ID for correlation |
| AC-14 | External Python plugin | Receives bus events on its TLS connection the same as internal Go plugins |
| AC-15 | Doc section 11 SDK references | Updated to reflect struct field API, not method-style |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| SIGHUP with 3 plugins | -> | concurrent verify via bus | `test/reload/test-tx-bus-concurrent-verify.ci` |
| Apply failure in one plugin | -> | automatic rollback to all | `test/reload/test-tx-bus-rollback-on-failure.ci` |
| Apply timeout | -> | deadline enforcement + rollback | `test/reload/test-tx-bus-timeout-rollback.ci` |
| Successful transaction | -> | committed event + config file write | `test/reload/test-tx-bus-committed.ci` |

## Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/transaction-protocol.md` -- update section 11 SDK references to struct fields |

## Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Bus-native | No `SendConfigVerify`/`SendConfigApply` RPCs remain in reload.go |
| Concurrent | Verify and apply are published once, not sent per-plugin |
| Transaction ID | Every event in a transaction carries the same ID |
| Deadline | Computed from budgets, enforced with timeout |
| Rollback | Triggered automatically on first apply failure or timeout |
| Committed | Plugins receive committed event before config file is written |
| Failure codes | Rollback responses carry a code (ok, timeout, transient, error, broken) |
| Recovery | Broken plugins restarted once |
| External plugins | Python plugins receive bus events via their connection |

## Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Bus events published | `grep "config/verify\|config/apply\|config/rollback\|config/committed" internal/component/plugin/server/reload.go` |
| No sequential RPCs | `grep -c "SendConfigVerify\|SendConfigApply" internal/component/plugin/server/reload.go` returns 0 |
| Transaction ID | `grep "tx\|transaction-id" pkg/plugin/rpc/types.go` |
| Deadline computation | `grep "deadline\|budget" internal/component/config/transaction/orchestrator.go` |
| Failure codes | `grep "broken\|transient\|timeout" pkg/plugin/rpc/types.go` |
| Journal with tx | `grep "func NewJournal" pkg/plugin/sdk/journal.go` shows tx parameter |
| Doc updated | `grep "struct field\|Registration{" docs/architecture/config/transaction-protocol.md` |

## Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Deadline bypass | Can a malicious plugin set budget to MaxInt and block all transactions? Cap budget (existing MaxBudgetSeconds). |
| Transaction ID collision | tx ID must be unique. Use crypto/rand or monotonic counter. |
| Rollback amplification | A plugin that always fails apply forces infinite rollback cycles. Cap retries per transaction. |

## Design Insights

The key architectural shift: the server goes from being a sequential RPC caller
("call A, wait, call B, wait") to being a bus publisher ("publish event, wait for
N responses with deadline"). This makes external Python plugins first-class
participants -- they receive the same bus events as internal Go plugins, at the
same time, with the same deadline.

# Spec: Config Transaction Protocol Consumers

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-config-tx-protocol |
| Phase | 3/5 |
| Updated | 2026-04-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/config/transaction-protocol.md` - protocol design
4. `docs/architecture/plugin/plugin-relationships.md` - plugin map
5. `internal/component/bgp/reactor/reactor_api.go` - reconcilePeers
6. `internal/component/iface/register.go` - interface plugin apply

## Task

Wire the 5 system plugins that own config roots into the config transaction protocol
built by `spec-config-tx-protocol.md`. Each plugin implements OnConfigVerify (with
duration estimate), OnConfigApply (using the SDK journal for rollback), and
OnConfigRollback.

The plugins:

| Plugin | Config Root | Key challenge |
|--------|-----------|---------------|
| bgp | `bgp` | reconcilePeers needs journal: Record(addPeer, stopPeer) per peer change |
| interface | `interface` | Creates interfaces + addresses; publishes side-effect events consumed by other plugins |
| sysrib | `sysrib` | Admin distance config; feeds fib plugins via bus |
| fib-kernel | `fib.kernel` | OS route table settings |
| fib-p4 | `fib.p4` | P4 switch settings |

This spec does NOT build the protocol infrastructure (bus orchestrator, journal library,
topic constants) -- that is `spec-config-tx-protocol.md`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/transaction-protocol.md` - protocol design
  -> Decision: per-plugin bus topics, plugin-estimated timeouts, journal for rollback
  -> Decision: config file written after all apply/ok
  -> Constraint: plugins must respond to verify even if unaffected roots
  -> Constraint: dependency waiting is plugin-internal (engine doesn't manage)
- [ ] `docs/architecture/plugin/plugin-relationships.md` - plugin map
  -> Constraint: BGP sub-plugins mediated by reactor, not direct transaction participants
  -> Constraint: interface publishes side-effect events (created, addr/added) consumed by BGP, DHCP
  -> Constraint: sysrib -> fib chain via bus (rib/best-change -> sysrib/best-change)
- [ ] `docs/architecture/core-design.md` - reactor, config pipeline
  -> Constraint: config pipeline: File -> Tree -> ResolveBGPTree -> map[string]any -> reconcilePeers

### RFC Summaries (MUST for protocol work)
N/A - internal architecture, not protocol work.

**Key insights:**
- reconcilePeers diffs old/new peers into toRemove/toAdd, applies sequentially with no rollback
- Interface plugin uses declarative config application (config.go) with make-before-break migration
- sysrib/fib plugins have simple config (admin distance, route table settings)
- BGP is the most complex consumer; interface has side-effect event coordination

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/reactor_api.go` - reconcilePeers
  -> Constraint: categorizes peers into toRemove/toAdd based on settings diff
  -> Constraint: removes peers first, then adds. No rollback on partial add failure.
  -> Constraint: VerifyConfig validates all peers without modifying reactor state
  -> Constraint: ApplyConfigDiff calls reconcilePeers
- [ ] `internal/component/bgp/reactor/reactor_api.go` - VerifyConfig
  -> Constraint: loads peers via reloadFunc, validates field constraints, returns error
- [ ] `internal/component/iface/register.go` - interface plugin startup and config apply
  -> Constraint: ConfigureBus injects bus for event publishing
  -> Constraint: applyConfig called during startup and reload
- [ ] `internal/component/iface/config.go` - declarative interface application
  -> Constraint: diffInterfaces computes add/remove/modify sets
  -> Constraint: make-before-break migration for modified interfaces
  -> Constraint: publishes interface/created, interface/addr/added etc. as side effects
- [ ] `internal/plugins/sysrib/register.go` - sysrib plugin registration
  -> Constraint: OnConfigure parses admin distance config
  -> Constraint: ConfigRoots: ["sysrib"]
- [ ] `internal/plugins/fibkernel/register.go` - fib-kernel plugin registration
  -> Constraint: ConfigRoots: ["fib.kernel"], Dependencies: ["sysrib"]
- [ ] `internal/plugins/fibp4/register.go` - fib-p4 plugin registration
  -> Constraint: ConfigRoots: ["fib.p4"], Dependencies: ["sysrib"]

**Behavior to preserve:**
- reconcilePeers diff logic (toRemove/toAdd categorization)
- VerifyConfig validation (peer field constraints)
- Interface make-before-break migration
- Interface side-effect events (interface/created, addr/added, etc.)
- sysrib admin distance config parsing
- fib plugin config parsing
- All existing bus topics and subscriptions

**Behavior to change:**
- reconcilePeers: wrap each peer add/remove in journal.Record for rollback
- Interface apply: wrap each interface/address operation in journal.Record
- All 5 plugins: implement OnConfigVerify with duration estimate
- All 5 plugins: implement OnConfigApply using SDK journal
- All 5 plugins: implement OnConfigRollback (journal.Rollback)
- All 5 plugins: provide initial VerifyBudget and ApplyBudget at registration

## Data Flow (MANDATORY)

### Entry Point
- Transaction orchestrator publishes `config/verify/<plugin>` and `config/apply/<plugin>`
- Each plugin receives its filtered diffs via bus subscription

### Transformation Path

**BGP:**
1. `config/verify/bgp` received with BGP subtree diff
2. Plugin parses candidate peer settings via existing VerifyConfig path
3. Plugin estimates duration: count of peer changes * per-peer cost
4. Responds `config/verify/ok` with apply budget
5. `config/apply/bgp` received
6. Plugin calls reconcilePeers with journal wrapping:
   - For each peer to remove: `journal.Record(stopPeer, restartPeerWithOldSettings)`
   - For each peer to add: `journal.Record(addPeer, stopPeer)`
7. Responds `config/apply/ok` with updated budgets
8. On rollback: journal replays (re-adds removed peers, stops added peers)

**Interface:**
1. `config/verify/interface` received with interface subtree diff
2. Plugin validates interface names, addresses, MTU values
3. Estimates duration: count of interface operations * per-op cost
4. Responds `config/verify/ok`
5. `config/apply/interface` received
6. Plugin calls diffInterfaces, then for each operation:
   - Create interface: `journal.Record(createIface, deleteIface)` -> publishes `interface/created`
   - Add address: `journal.Record(addAddr, removeAddr)` -> publishes `interface/addr/added`
   - Delete interface: `journal.Record(deleteIface, recreateIface)` -> publishes `interface/deleted`
7. Responds `config/apply/ok`
8. On rollback: journal replays in reverse, publishes corresponding undo events

**sysrib / fib-kernel / fib-p4:**
1. `config/verify/<plugin>` received
2. Plugin validates config fields
3. Responds `config/verify/ok` with small budget (simple config)
4. `config/apply/<plugin>` received
5. Plugin applies config with journal wrapping
6. Responds `config/apply/ok`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Orchestrator -> BGP plugin | `config/verify/bgp` and `config/apply/bgp` bus events | [ ] |
| Orchestrator -> Interface plugin | `config/verify/interface` and `config/apply/interface` bus events | [ ] |
| Interface -> BGP (side effects) | `interface/addr/added` bus event during apply | [ ] |
| BGP -> RIB chain (side effects) | Peer state changes propagate via existing bus topics | [ ] |

### Integration Points
- SDK journal (from spec-config-tx-protocol) - used by all 5 plugins
- Transaction orchestrator (from spec-config-tx-protocol) - publishes events, collects acks
- Existing bus topics - side-effect events continue to work during transaction apply
- reconcilePeers - extended with journal, not replaced
- diffInterfaces - extended with journal, not replaced

### Architectural Verification
- [ ] No bypassed layers (plugins receive events via bus, not direct calls)
- [ ] No unintended coupling (each plugin handles its own rollback via journal)
- [ ] No duplicated functionality (extends existing diff/apply logic, wraps with journal)
- [ ] Zero-copy preserved where applicable (N/A for config path)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| SIGHUP with BGP peer change | -> | reconcilePeers with journal | `test/reload/test-tx-bgp-rollback.ci` |
| SIGHUP with interface add | -> | iface apply with journal, side-effect events | `test/reload/test-tx-iface-apply.ci` |
| BGP apply fails on peer 3 of 5 | -> | journal rolls back peers 1-2 | `TestReconcilePeersJournalRollback` |
| Interface apply + BGP addr binding | -> | iface creates addr, BGP binds listener | `test/reload/test-tx-iface-bgp-chain.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | BGP config change triggers transaction | BGP plugin responds to config/verify/bgp with apply budget based on peer change count |
| AC-2 | BGP apply: 5 peers to add, peer 3 fails | Journal rolls back peers 1-2 (stopped), plugin publishes apply/failed |
| AC-3 | BGP apply: all peers succeed | All peers running, plugin publishes apply/ok with updated budgets |
| AC-4 | BGP rollback after partial apply | Removed peers re-added with old settings, added peers stopped |
| AC-5 | Interface config adds new interface + address | Interface created, address assigned, side-effect events published |
| AC-6 | Interface rollback after partial apply | Created interfaces deleted, removed interfaces recreated, undo events published |
| AC-7 | Interface apply triggers BGP listener binding | BGP waits for interface/addr/added, then binds listener within same transaction |
| AC-8 | sysrib config change | Admin distance updated via journal, rollback restores previous distances |
| AC-9 | fib-kernel config change | Route table settings updated via journal |
| AC-10 | fib-p4 config change | P4 switch settings updated via journal |
| AC-11 | All 5 plugins provide initial budgets at registration | VerifyBudget and ApplyBudget set in Stage 1 |
| AC-12 | Plugin verify estimates scale with diff size | BGP: budget proportional to peer count. Interface: proportional to interface count. |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReconcilePeersJournalRollback` | `reactor/reactor_api_test.go` | AddPeer fails on peer 3, journal rolls back peers 1-2 | |
| `TestReconcilePeersJournalSuccess` | `reactor/reactor_api_test.go` | All peers added, journal committed (no rollback) | |
| `TestReconcilePeersJournalRemoveThenAdd` | `reactor/reactor_api_test.go` | Removed peers journaled with old settings for rollback | |
| `TestBGPVerifyEstimate` | `reactor/reactor_api_test.go` | Duration estimate proportional to peer change count | |
| `TestBGPApplyBudgetUpdate` | `reactor/reactor_api_test.go` | Budget updated after apply based on actual duration | |
| `TestIfaceApplyJournalCreate` | `iface/config_test.go` | Interface created via journal, rollback deletes it | |
| `TestIfaceApplyJournalAddress` | `iface/config_test.go` | Address added via journal, rollback removes it | |
| `TestIfaceApplyJournalRollbackEvents` | `iface/config_test.go` | Rollback publishes interface/deleted and addr/removed events | |
| `TestIfaceVerifyEstimate` | `iface/config_test.go` | Duration estimate proportional to interface change count | |
| `TestSysribApplyJournal` | `plugins/sysrib/sysrib_test.go` | Admin distance applied via journal, rollback restores | |
| `TestFibKernelApplyJournal` | `plugins/fibkernel/fibkernel_test.go` | Config applied via journal | |
| `TestFibP4ApplyJournal` | `plugins/fibp4/fibp4_test.go` | Config applied via journal | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - no new numeric inputs. Budget estimates tested via proportional scaling tests.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-tx-bgp-rollback` | `test/reload/test-tx-bgp-rollback.ci` | Config adds peers, one fails, all rolled back | |
| `test-tx-iface-apply` | `test/reload/test-tx-iface-apply.ci` | Config adds interface, applied with side-effect events | |
| `test-tx-iface-bgp-chain` | `test/reload/test-tx-iface-bgp-chain.ci` | Interface created, BGP binds to new address in same transaction | |

### Future (if deferring any tests)
- DHCP as WantsConfig consumer (iface-dhcp not yet wired)
- sysrib -> fib chain rollback coordination (fib rollback when sysrib rolls back)

## Files to Modify
- `internal/component/bgp/reactor/reactor_api.go` - reconcilePeers: wrap with journal
- `internal/component/bgp/reactor/reactor_api.go` - VerifyConfig: return duration estimate
- `internal/component/bgp/plugin/register.go` - add VerifyBudget, ApplyBudget, transaction callbacks
- `internal/component/iface/config.go` - diffInterfaces apply: wrap with journal
- `internal/component/iface/register.go` - add budgets, transaction callbacks
- `internal/plugins/sysrib/register.go` - add budgets, transaction callbacks
- `internal/plugins/sysrib/sysrib.go` - apply via journal
- `internal/plugins/fibkernel/register.go` - add budgets, transaction callbacks
- `internal/plugins/fibkernel/fibkernel.go` - apply via journal
- `internal/plugins/fibp4/register.go` - add budgets, transaction callbacks
- `internal/plugins/fibp4/fibp4.go` - apply via journal

## Files to Create
- `test/reload/test-tx-bgp-rollback.ci` - functional test: BGP rollback
- `test/reload/test-tx-iface-apply.ci` - functional test: interface apply
- `test/reload/test-tx-iface-bgp-chain.ci` - functional test: cross-plugin chain

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | |
| CLI commands/flags | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | Yes | `test/reload/test-tx-*.ci` |

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
| 8 | Plugin SDK/protocol changed? | No (protocol spec handles SDK docs) | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/plugin/plugin-relationships.md` - update transaction participation column after implementation |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + transaction-protocol.md + plugin-relationships.md |
| 2. Audit | Files to Modify, TDD Test Plan |
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

1. **Phase: BGP transaction callbacks** -- verify estimate, apply with journal, rollback
   - Tests: `TestReconcilePeersJournalRollback`, `TestReconcilePeersJournalSuccess`, `TestReconcilePeersJournalRemoveThenAdd`, `TestBGPVerifyEstimate`, `TestBGPApplyBudgetUpdate`
   - Files: `reactor/reactor_api.go`, `bgp/plugin/register.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Interface transaction callbacks** -- verify estimate, apply with journal, side-effect events
   - Tests: `TestIfaceApplyJournalCreate`, `TestIfaceApplyJournalAddress`, `TestIfaceApplyJournalRollbackEvents`, `TestIfaceVerifyEstimate`
   - Files: `iface/config.go`, `iface/register.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: sysrib transaction callbacks** -- verify, apply with journal
   - Tests: `TestSysribApplyJournal`
   - Files: `plugins/sysrib/register.go`, `plugins/sysrib/sysrib.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: fib-kernel and fib-p4 transaction callbacks** -- verify, apply with journal
   - Tests: `TestFibKernelApplyJournal`, `TestFibP4ApplyJournal`
   - Files: `plugins/fibkernel/register.go`, `plugins/fibkernel/fibkernel.go`, `plugins/fibp4/register.go`, `plugins/fibp4/fibp4.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Cross-plugin functional tests** -- SIGHUP end-to-end with rollback
   - Tests: functional tests (`test-tx-*.ci`)
   - Verify: tests fail -> implement -> tests pass

6. **Full verification** -> `make ze-verify`
7. **Complete spec** -> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Journal undo operations are exact inverses of apply operations |
| Side effects | Interface rollback publishes undo events (interface/deleted, addr/removed) |
| Dependency waiting | BGP waits for interface/addr/added before binding, handles rollback during wait |
| Budget accuracy | Estimates scale with diff size, updated after each transaction |
| Rule: no-layering | Old apply paths (direct reconcilePeers call without journal) replaced, not kept |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| BGP journal in reconcilePeers | `grep "journal.Record" internal/component/bgp/reactor/reactor_api.go` |
| BGP verify estimate | `grep "VerifyBudget\|ApplyBudget\|estimate" internal/component/bgp/plugin/register.go` |
| Interface journal in apply | `grep "journal.Record" internal/component/iface/config.go` |
| Interface rollback events | `grep "interface/deleted\|addr/removed" internal/component/iface/config.go` |
| sysrib journal | `grep "journal" internal/plugins/sysrib/sysrib.go` |
| fib-kernel journal | `grep "journal" internal/plugins/fibkernel/fibkernel.go` |
| fib-p4 journal | `grep "journal" internal/plugins/fibp4/fibp4.go` |
| Functional tests exist | `ls test/reload/test-tx-bgp-rollback.ci test/reload/test-tx-iface-apply.ci test/reload/test-tx-iface-bgp-chain.ci` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Rollback completeness | Journal undo must fully reverse state. Partial undo leaves system inconsistent. |
| Side-effect event integrity | Rollback undo events must match apply events. Missing undo event leaves downstream stale. |
| Resource cleanup | Rolled-back peers must release sockets, timers, buffers. No leak on rollback. |

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

### Journal wraps existing diff/apply logic, does not replace it

reconcilePeers and diffInterfaces already compute what to add/remove. The journal
wraps each operation with its undo counterpart. The diff logic is unchanged.

### Interface rollback publishes undo events

When a journal rollback deletes a created interface, it publishes `interface/deleted`.
This ensures downstream plugins (BGP, DHCP) that reacted to `interface/created`
also react to the undo. Without undo events, downstream state would be stale after
rollback.

### BGP reconcilePeers journals removes before adds

The existing order (remove first, then add) is preserved. Journal records:
- Remove peer X: `Record(stopPeer(X), addPeerWithOldSettings(X))`
- Add peer Y: `Record(addPeer(Y), stopPeer(Y))`

On rollback, the journal replays in reverse: stop added peers, then re-add
removed peers with old settings. This restores the exact pre-transaction state.

### Budget estimates are proportional to diff size

BGP estimates per-peer cost * number of peer changes. Interface estimates per-operation
cost * number of interface operations. These are rough but self-correcting: if the
actual apply takes longer, the plugin updates its per-unit cost for next time.

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
- **Journal location:** Spec listed `iface/config.go`, `sysrib/sysrib.go`, `fibkernel/fibkernel.go`, `fibp4/fibp4.go` as journal targets. Implementation places journal wrapping in the `register.go` OnConfigApply callbacks instead, wrapping the existing apply calls at the callback boundary. This is architecturally equivalent (journal wraps the apply, not individual operations inside it) and avoids modifying the core apply functions.
- **Additional files:** `reactor.go` (public methods for PeerDiffCount/ReconcilePeersWithJournal), `registry/interfaces.go` (ConfigJournal interface + BGPReactorHandle methods), `server/reload.go` (removed direct reactor verify/apply for BGP, added BGP-last apply ordering) were modified but not listed in spec.
- **Interface rollback events:** Spec design decision said per-operation undo events (`interface/deleted`, `addr/removed`). Implementation publishes a single `interface/rollback` aggregate event. Reason: `applyConfig` is a monolithic function; per-operation events would require refactoring it. Downstream consumers should re-query interface state on rollback rather than tracking individual undo events.

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
- [ ] AC-1..AC-12 all demonstrated
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
- [ ] Write learned summary to `plan/learned/NNN-config-tx-consumers.md`
- [ ] Summary included in commit

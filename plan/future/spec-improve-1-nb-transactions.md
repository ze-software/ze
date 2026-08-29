# Spec: improve-1 -- Northbound Config Transaction Contract

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-improve-0-umbrella.md` -- set context and comparison-honesty scope
4. `internal/component/config/transaction/orchestrator.go` -- transaction coordinator
5. `internal/component/cli/editor.go` -- backup/revision storage

## Task

Ze already runs every config commit through a transaction coordinator with a unique
transaction ID and verify/apply/commit/rollback phases, but no operator-facing surface
exposes transactions as addressable objects. The pieces are split: gNMI Set commits a
session and returns no transaction ID; CLI rollback restores editor backup files by
revision number; the coordinator's txID never reaches the operator.

Build one operator-facing transaction contract on the existing machinery: persist a
transaction record per successful commit (ID, timestamp, user, comment, config
revision reference), and expose one coherent surface: commit with optional comment,
confirmed commit with a timeout that auto-rolls-back unless confirmed, confirm, list
transactions, get one transaction, and rollback-to-transaction (which itself creates a
new transaction). Surface through the existing CLI (`ze config ...`,
`show config transactions`) and return the transaction ID from gNMI Set via the
SetResponse extension mechanism where it fits. No new custom proto.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config CLI conventions
  → Constraint: (fill during design)
- [ ] `ai/rules/config.md` - config content must go through approved methods
  → Constraint: rollback-to-transaction must reuse the commit path, not write files directly
- [ ] `ai/rules/cli.md` - keywords before values for new CLI verbs
  → Constraint: (fill during design)
- [ ] `plan/future/spec-fleet-3-audit-trail.md` - hub-side audit trail spec; overlap check
  → Decision: (fill during design -- transaction records are the device-local half of an audit story)

### RFC Summaries (MUST for protocol work)
- Not protocol work. RFC 6241 (NETCONF confirmed commit) is prior art only; check
  `rfc/short/` during design if terminology is borrowed.

**Key insights:**
- The coordinator already generates `tx-<nanos>` IDs; the work is persistence + surface,
  not new commit machinery.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/config/transaction/orchestrator.go` - `TxCoordinator.Execute` runs verify -> apply -> commit or rollback (:198-234); txID generated at :150, exposed via `TransactionID()` (:163) but never persisted or shown to operators
- [ ] `internal/component/gnmi/set.go` - `Set` commits the session (:97-105) and returns results + timestamp only (:112-115); no transaction ID, no confirmed-timeout shape
- [ ] `internal/component/config/cli/cmd_rollback.go` - rollback restores editor backup revision N (:62-86)
- [ ] `internal/component/cli/editor.go` - `ListBackups` (:1019), `Rollback(backupPath)` (:1075) are file-based revision storage
- [ ] `internal/component/config/transaction/executor.go` - owner ACK waiting (verify during design)
- [ ] `internal/component/config/archive/` - config archive destinations (verify overlap during design)

**Behavior to preserve:** (unless user explicitly said to change)
- `ze config rollback <N> <file>` file-revision rollback keeps working (operators may
  rely on it for offline files); transaction rollback is additive.
- gNMI Set semantics for existing clients: response stays valid gNMI; the transaction
  ID rides an extension, not a breaking shape change.
- TxCoordinator phase semantics and plugin ACK protocol unchanged.

**Behavior to change:** (only if user explicitly requested)
- Successful commits gain a persisted transaction record and a visible transaction ID.

## Data Flow (MANDATORY)

### Entry Point
- Operator commit: CLI commit verb or gNMI SetRequest; both land in the config session
  commit path which drives `TxCoordinator.Execute`.
- Confirmed commit: same entry with a timeout argument; confirm is a follow-up command
  referencing the pending transaction.

### Transformation Path
1. Commit request (CLI or gNMI) reaches the session commit path with optional comment + confirmed-timeout.
2. `TxCoordinator.Execute` runs verify/apply/commit as today (`orchestrator.go`).
3. On success, a transaction record (ID, timestamp, user, comment, revision reference) is appended to a persistent transaction store.
4. Confirmed commit arms a rollback timer; expiry replays the previous revision through the SAME commit path, creating a new transaction; confirm before expiry disarms it.
5. `show config transactions` / get / rollback-to-id read the store and reuse the commit path.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| gNMI ↔ config session | existing `sessions.Commit(ConfigCommitRequest)` call gains comment/confirmed fields | [ ] |
| CLI ↔ transaction store | new show/commit verbs read/write the store | [ ] |
| Store ↔ revision storage | record references the editor backup / archive revision, does not copy config bodies | [ ] |

### Integration Points
- `TxCoordinator.Execute` - the single producer of committed transactions.
- `Editor.ListBackups`/`Rollback` - existing revision storage the records reference.
- gNMI `Set` - returns the transaction ID on success.

### Architectural Verification
- [ ] No bypassed layers (rollback-to-transaction goes through verify/apply, never raw file writes)
- [ ] No unintended coupling (store is owned by the config component)
- [ ] No duplicated functionality (reuses editor backups/archive for config bodies)
- [ ] Registration over hardcoding -- new CLI verbs and RPCs register via existing dispatch registries (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every committed config already has a durable revision (editor backup or archive) a record can reference | `editor.go` ListBackups; archive package exists | Store must persist full config bodies itself | Trace commit path during design: does every commit write a backup? | unvalidated |
| A-2 | gNMI SetResponse can carry a transaction ID without breaking clients | gNMI proto has Extension fields | Need a separate RPC or omit gNMI exposure | Read gnmi proto + one client (gnmic) during design | unvalidated |
| A-3 | A daemon restart during a pending confirmed commit must roll back on boot | NETCONF confirmed-commit semantics | Timer state must be persisted, not in-memory only | Design decision + test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Confirmed-commit auto-rollback races a second operator commit | Design review of store locking | Serialize: reject new commits while a rollback is pending, or make confirm implicit on next commit |
| R-2 | Transaction store grows unbounded on appliances | Store size in long-run soak | Cap records (ring or max-N) with YANG-configurable retention |
| R-3 | Overlap with fleet audit trail (spec-fleet-3) creates two record formats | Design-phase cross-check | Share the record schema; fleet trail aggregates device-local records |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| CLI commit with comment | → | transaction record appended after `TxCoordinator.Execute` success | test/config/transaction-record.ci |
| CLI confirmed commit + no confirm | → | rollback timer replays previous revision via commit path | test/config/transaction-confirm-timeout.ci |
| gNMI Set success | → | SetResponse carries transaction ID | TestGNMISetReturnsTransactionID |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Successful commit (CLI or gNMI) | Persisted record with ID, timestamp, user, comment, revision reference |
| AC-2 | `show config transactions` | Lists records newest first with ID, time, user, comment |
| AC-3 | Confirmed commit, timeout expires unconfirmed | Previous config restored through verify/apply; restoration is itself a new transaction |
| AC-4 | Confirmed commit, confirm before timeout | Config kept; timer disarmed; no rollback |
| AC-5 | Rollback to transaction ID | That transaction's config is committed as a new transaction; failure leaves running config untouched |
| AC-6 | Commit fails in verify/apply | No transaction record created |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Commits risky BGP change with `confirmed 5m`, loses SSH | timer fires -> previous revision -> verify/apply -> connectivity restored | test/config/transaction-confirm-timeout.ci |
| 2 | Reviews who changed what | show config transactions -> get one -> diff against running | test/config/transaction-record.ci |
| 3 | Reverts to last week's config | rollback-to-id -> commit path -> new record | test/config/transaction-rollback.ci |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestTransactionStoreAppendList | `internal/component/config/transaction/store_test.go` | record persistence, ordering, retention cap | |
| TestConfirmedCommitTimerRollback | `internal/component/config/transaction/confirm_test.go` | timeout -> rollback, confirm -> disarm | |
| TestGNMISetReturnsTransactionID | `internal/component/gnmi/set_test.go` | Set response carries tx ID | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| confirmed timeout | (fill during design) | (fill) | 0/negative rejected | (fill) |
| retention count | (fill during design) | (fill) | (fill) | (fill) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| transaction-record | `test/config/transaction-record.ci` | commit -> show transactions lists it | |
| transaction-confirm-timeout | `test/config/transaction-confirm-timeout.ci` | unconfirmed commit rolls back | |
| transaction-rollback | `test/config/transaction-rollback.ci` | rollback-to-id restores config | |

### Interop Tests (MANDATORY for protocol features)
- Not a wire-protocol feature; N/A. gNMI client compatibility covered by functional tests.

## Files to Modify
- `internal/component/config/transaction/orchestrator.go` - emit record on committed result
- `internal/component/gnmi/set.go` - commit request gains comment/confirmed fields; response carries tx ID
- `internal/component/config/cli/` - new commit/confirm/rollback/show verbs
- YANG schema for retention config (owning module per `ai/rules/config.md`)

## Files to Create
- `internal/component/config/transaction/store.go` - persistent transaction records
- `internal/component/config/transaction/confirm.go` - confirmed-commit timer
- `test/config/transaction-*.ci` - functional tests above

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - register new CLI verbs + store skeleton, failing wiring tests from the table above
2. **Phase: transaction store** - record append/list/get + retention
3. **Phase: confirmed commit** - timer, confirm verb, restart behavior (per A-3 decision)
4. **Phase: rollback-to-id + gNMI ID exposure**
5. Functional tests, full verification (`./le verify current mode full`), learned summary, two-commit closure

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-6 implemented with file:line |
| Correctness | rollback path reuses verify/apply; no raw file writes |
| Registration over hardcoding | verbs/RPCs registered, core discovers them (`ai/rules/plugins.md`) |
| Data flow | records reference revisions; single producer (coordinator) |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | timeout bounds, transaction ID parsing, comment length |
| AuthZ | who may rollback/confirm; align with RBAC guides |
| Resource exhaustion | retention cap enforced; store writes bounded |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Design Insights
- (fill during design)

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Front existing coordinator, no new proto | a dedicated northbound transaction proto | Ze already has gNMI + CLI + plugin RPC; concepts transfer, a new API surface does not pay for itself |

## Known Limitations
- (fill during design)

## Implementation Summary

### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

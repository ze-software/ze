# Spec: fw-2-firewall-nft — nftables Firewall Backend

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-fw-1-data-model |
| Phase | - |
| Updated | 2026-04-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-fw-0-umbrella.md` — design decisions
3. `plan/spec-fw-1-data-model.md` — data model types
4. `internal/component/firewall/backend.go` — Backend interface (from fw-1)

## Task

Implement the firewallnft plugin using `github.com/google/nftables`. The plugin receives
`[]Table` via `Apply`, reconciles against the kernel (create/replace ze_* tables, delete
orphans), and **lowers** abstract Match/Action types to google/nftables expr.* register operations.

Also implements `ListTables` and `GetCounters` read methods for CLI.

New dependency: `github.com/google/nftables` (uses already-vendored `github.com/mdlayher/netlink`).

## Required Reading

### Architecture Docs
- [ ] `plan/spec-fw-0-umbrella.md` — design decisions 3, 8, 9
  → Decision: Apply + ListTables + GetCounters. Plugin owns reconciliation.
  → Constraint: only touch ze_* tables, delete orphan ze_* tables
  → Decision: abstract Match/Action types lowered to nftables register operations
- [ ] `internal/plugins/ifacenetlink/register.go` — plugin registration pattern
  → Constraint: init() calls RegisterBackend(name, factory)
- [ ] `internal/plugins/ifacenetlink/manage_linux.go` — linux-only build tags
  → Constraint: firewallnft files use `_linux.go` suffix, `_other.go` stubs

**Key insights:**
- Plugin registers as backend "nft" via init() in register.go
- Apply receives full desired state, reconciles: list current ze_* tables, diff, create/replace/delete
- Translation: ze Expression types map 1:1 to google/nftables expr.* types
- All kernel operations within a single nftables.Conn.Flush() for atomicity

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ifacenetlink/manage_linux.go` — linux backend implementation pattern
  → Constraint: build tags, error wrapping, logging with slogutil
- [ ] `internal/plugins/ifacenetlink/register.go` — RegisterBackend("netlink", factory) in init()
  → Constraint: same pattern for firewallnft: RegisterBackend("nft", factory)

**Behavior to preserve:**
- No existing nftables code. Greenfield.
- Non-ze_* tables in kernel must never be touched.

**Behavior to change:**
- Add firewallnft plugin that programs nftables via google/nftables

## Data Flow (MANDATORY)

### Entry Point
- Component calls `backend.Apply(desired []Table)` on startup and reload

### Transformation Path
1. Backend receives `[]Table` (ze data model)
2. Backend lists current kernel tables, filters to `ze_*` prefix
3. Backend computes diff: tables to create, replace, delete
4. For each table: translate ze Table/Chain/Set/Flowtable to google/nftables types
5. For each Term: lower abstract Match types to nftables register operations (Payload+Bitwise+Cmp chains), lower Action types to nftables expressions (Verdict, NAT, Log, etc.)
6. All operations queued on `nftables.Conn`, committed atomically via `Flush()`
7. ListTables: query kernel via Conn.ListTables, filter ze_*, read chains/rules/sets, reverse-map nftables expressions back to abstract types
8. GetCounters: query kernel via Conn.GetRule, extract Counter expression values

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Component → Plugin | Apply([]Table) method call | [ ] |
| Plugin → Kernel | google/nftables Conn.Flush() via NFNETLINK socket | [ ] |

### Integration Points
- `internal/component/firewall/model.go` (fw-1) — Table, Chain, Term, Match, Action types
- `internal/component/firewall/backend.go` (fw-1) — Backend interface, RegisterBackend
- `github.com/google/nftables` — external library for kernel communication

### Architectural Verification
- [ ] No bypassed layers (component → backend → kernel)
- [ ] No unintended coupling (plugin only depends on firewall model + google/nftables)
- [ ] No duplicated functionality
- [ ] Zero-copy not applicable (config structs, not wire data)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Firewall config in YANG | → | firewallnft Apply creates ze_* tables | `test/firewall/001-boot-apply.ci` |
| Config reload removes a table | → | firewallnft Apply deletes orphan ze_* table | `test/firewall/002-reload.ci` |
| Non-ze_* table exists | → | firewallnft Apply ignores it | `test/firewall/003-coexistence.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Apply with one table containing base chain | ze_* table and chain exist in kernel with hook/priority/policy |
| AC-2 | Apply with terms containing all match/action types | All abstract types lowered to correct nftables expressions |
| AC-3 | Apply with named set and elements | Named set created with correct type, flags, elements |
| AC-4 | Apply with flowtable | Flowtable created with devices and priority |
| AC-5 | Apply with verdict map | Verdict map created with correct key/data types |
| AC-6 | Apply called twice with different tables | First call's orphan ze_* tables deleted |
| AC-7 | Non-ze_* table in kernel | Not touched by Apply |
| AC-8 | Apply with empty desired state | All ze_* tables deleted from kernel |
| AC-9 | Apply fails mid-operation | Atomic: either all changes applied or none (Flush semantics) |
| AC-10 | Backend registered as "nft" | LoadBackend("nft") succeeds |
| AC-11 | ListTables called | Returns ze_* tables with chains, terms, sets from kernel |
| AC-12 | GetCounters called | Returns per-term packet/byte counter values |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLowerTable` | `internal/plugins/firewallnft/lower_test.go` | ze Table → nftables.Table |
| `TestLowerChain` | `internal/plugins/firewallnft/lower_test.go` | ze Chain → nftables.Chain (base + regular) |
| `TestLowerMatchSourceAddress` | `internal/plugins/firewallnft/lower_test.go` | MatchSourceAddress → Payload+Bitwise+Cmp chain |
| `TestLowerMatchDestPort` | `internal/plugins/firewallnft/lower_test.go` | MatchDestinationPort → Meta(L4Proto)+Payload+Cmp |
| `TestLowerMatchConnState` | `internal/plugins/firewallnft/lower_test.go` | MatchConnState → Ct+Bitwise+Cmp |
| `TestLowerAllMatches` | `internal/plugins/firewallnft/lower_test.go` | Every Match type produces valid nftables expressions |
| `TestLowerAllActions` | `internal/plugins/firewallnft/lower_test.go` | Every Action type produces valid nftables expressions |
| `TestLowerSet` | `internal/plugins/firewallnft/lower_test.go` | ze Set → nftables.Set with elements |
| `TestLowerFlowtable` | `internal/plugins/firewallnft/lower_test.go` | ze Flowtable → nftables.Flowtable |
| `TestListTables` | `internal/plugins/firewallnft/read_test.go` | ListTables returns ze_* tables from kernel |
| `TestGetCounters` | `internal/plugins/firewallnft/read_test.go` | GetCounters returns per-term packet/byte counts |
| `TestReconcileCreate` | `internal/plugins/firewallnft/reconcile_test.go` | New table created |
| `TestReconcileDelete` | `internal/plugins/firewallnft/reconcile_test.go` | Orphan ze_* table deleted |
| `TestReconcileReplace` | `internal/plugins/firewallnft/reconcile_test.go` | Changed table replaced |
| `TestReconcileIgnoreNonZe` | `internal/plugins/firewallnft/reconcile_test.go` | Non-ze_* tables untouched |
| `TestReconcileEmpty` | `internal/plugins/firewallnft/reconcile_test.go` | Empty desired → all ze_* deleted |
| `TestBackendRegistration` | `internal/plugins/firewallnft/register_test.go` | RegisterBackend("nft") works |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Chain priority | int32 | max int32 | N/A | N/A |
| Set element count | 0-unlimited | 1M elements | N/A | memory limit |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Boot apply | `test/firewall/001-boot-apply.ci` | ze starts with firewall config, kernel has ze_* tables | |
| Reload | `test/firewall/002-reload.ci` | firewall config changed, reload, kernel state matches new config | |
| Coexistence | `test/firewall/003-coexistence.ci` | non-ze table exists, ze reload does not touch it | |
| All expressions | `test/firewall/005-all-expressions.ci` | config with every expression type, all programmed correctly | |
| Named sets | `test/firewall/006-named-sets.ci` | config with named sets and lookups, set elements in kernel | |

### Future (if deferring any tests)
- None

## Files to Modify

- `go.mod` — add `github.com/google/nftables` dependency
- `go.sum` — updated
- `vendor/` — vendor new dependency

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | Spec-fw-4 |
| CLI commands | No | Spec-fw-5 |
| Functional test | Yes | `test/firewall/*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` — nftables firewall backend |
| 2-12 | Other categories | Handled by umbrella and fw-4/fw-5 specs | |

## Files to Create

- `internal/plugins/firewallnft/firewallnft.go` — package doc, logger
- `internal/plugins/firewallnft/backend_linux.go` — Apply implementation
- `internal/plugins/firewallnft/backend_other.go` — stub for non-linux
- `internal/plugins/firewallnft/lower_linux.go` — abstract Match/Action → nftables register expressions
- `internal/plugins/firewallnft/lower_test.go` — lowering unit tests
- `internal/plugins/firewallnft/read_linux.go` — ListTables, GetCounters implementations
- `internal/plugins/firewallnft/read_test.go` — read method tests
- `internal/plugins/firewallnft/reconcile_linux.go` — diff and apply logic
- `internal/plugins/firewallnft/reconcile_test.go` — reconciliation tests
- `internal/plugins/firewallnft/register.go` — init() RegisterBackend("nft", factory)
- `test/firewall/001-boot-apply.ci` — functional test
- `test/firewall/002-reload.ci` — functional test
- `test/firewall/003-coexistence.ci` — functional test
- `test/firewall/005-all-expressions.ci` — functional test
- `test/firewall/006-named-sets.ci` — functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + fw-0 + fw-1 |
| 2. Audit | Files to Create |
| 3. Implement (TDD) | Phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5-12 | Standard flow |

### Implementation Phases

1. **Phase: Vendor google/nftables** — add dependency
   - `go get github.com/google/nftables`, `go mod vendor`
   - Verify: `go build ./...`

2. **Phase: Lowering layer** — abstract Match/Action → nftables register expressions
   - Tests: TestLowerTable, TestLowerChain, TestLowerMatchSourceAddress, TestLowerAllMatches, TestLowerAllActions
   - Files: lower_linux.go
   - Verify: tests fail → implement → tests pass

3. **Phase: Read methods** — ListTables, GetCounters
   - Tests: TestListTables, TestGetCounters
   - Files: read_linux.go
   - Verify: tests fail → implement → tests pass

4. **Phase: Reconciler** — diff desired vs kernel, apply delta
   - Tests: TestReconcileCreate, TestReconcileDelete, TestReconcileReplace, TestReconcileIgnoreNonZe
   - Files: reconcile_linux.go, backend_linux.go
   - Verify: tests fail → implement → tests pass

5. **Phase: Registration** — RegisterBackend("nft"), stubs
   - Tests: TestBackendRegistration
   - Files: register.go, backend_other.go
   - Verify: tests fail → implement → tests pass

6. **Phase: Functional tests** — .ci tests
   - Files: test/firewall/*.ci
   - Verify: all pass

7. **Full verification** → `make ze-verify`

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented |
| Correctness | ze_* prefix enforced on all table operations |
| Naming | Backend name is "nft" |
| Data flow | Component → Apply → translate → Flush, no shortcuts |
| Rule: no-layering | No wrapper types around google/nftables. Lowering is direct. |
| Lowering correctness | Each abstract type produces the correct nftables expression chain (register allocation, offsets, masks) |
| Atomicity | All changes in single Flush() call |
| Read methods | ListTables filters to ze_*, GetCounters reads correct rule handles |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| firewallnft plugin compiles | `go build ./internal/plugins/firewallnft/...` |
| google/nftables vendored | `ls vendor/github.com/google/nftables/` |
| Lowering covers all abstract types | `grep "lower" internal/plugins/firewallnft/lower_linux.go` covers 42 types |
| Read methods work | TestListTables, TestGetCounters pass |
| Reconciler handles create/delete/replace | unit test output |
| Functional tests pass | `test/firewall/*.ci` all pass |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Table ownership | Apply MUST filter ListTables to ze_* prefix before any delete |
| Privilege | nftables requires CAP_NET_ADMIN |
| Atomic rollback | Flush failure leaves kernel state unchanged |
| Set element validation | Elements validated before Flush |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Translation mismatch | Re-read google/nftables API docs |
| Reconciler incorrect | Re-check diff logic |
| 3 fix attempts fail | STOP. Report. Ask user. |

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

## Implementation Summary

### What Was Implemented
### Bugs Found/Fixed
### Documentation Updates
### Deviations from Plan

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
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-fw-2-firewall-nft.md`
- [ ] Summary included in commit

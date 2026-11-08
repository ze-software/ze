# Spec: iface-address-reorder-moves-primary

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P2 |
| Release reason | The first configured address is documented as the interface's primary (DHCP segment selection, source-address selection), and on a live interface reordering the leaf-list does not make it so: the box keeps answering from the old primary until a restart |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`desiredState` (`internal/component/iface/config_apply.go`) returns each
device's addresses in configured order, and every install path walks that
order, so on a fresh install the first configured address is installed first
and is the kernel's primary. `docs/features/interfaces.md`, "The first
configured address is the primary", states the rule and its present limit: a
reconcile adds only the addresses missing from the device, so reordering the
`address` leaf-list of a live interface does not move an address already
installed. The old primary stays first, and the DHCPv4 server's segment
selection and kernel source selection keep following it.

Read at the producer: `reconcileOnReadyWithJournal` skips every desired
address the device already holds (`slices.Contains(current, addr)`) and then
removes stale ones; `decomposeIfaceOperations` skips every candidate address
the active config already names. Neither pass compares the kernel's order with
the configured order.

The goal: after a commit that changes which address comes first on a live
interface, the kernel lists the new first address first, with no restart, and
an address the reorder does not move stays installed throughout where the
kernel allows it (make-before-break, as the address migration already does).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Ordered desired state | `internal/component/iface/config_apply.go` `desiredState` | each device's addresses as a list in configured order, plugin addresses after | extends: the reorder pass reads the order from it |
| Reconcile add and remove passes | `internal/component/iface/config_apply.go` `reconcileOnReadyWithJournal` | adds missing addresses in configured order, removes stale ones in backend order, each step journaled for rollback | extends: a pass between them re-adds, in configured order, the addresses from the first one out of place |
| Transaction address operations | `internal/component/iface/operation.go` `decomposeIfaceOperations` | add-address and remove-address operations in configured order, for addresses entering or leaving only | extends: the same reorder expressed as operations, so the transaction path and the reconcile agree |
| Make-before-break migration | `docs/features/interfaces.md`, "Make-before-break" | adds the new address, waits for readiness, then removes the old | constrains: the reorder must not drop a BGP session bound to an address it moves |

## Required Reading

### Architecture Docs
- [ ] `docs/features/interfaces.md`, "The first configured address is the primary" and "Make-before-break"
- [ ] `ai/digests/iface.md`
- [ ] `docs/architecture/iface/logical-name-resolution.md`

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a live unit with addresses A then B; a commit reorders them to B then A | the kernel lists B first; A is still installed |
| AC-2 | the same reorder through the transaction path | the add and remove operations produce the same kernel order as the reconcile |
| AC-3 | the reorder fails part way | rollback restores A then B |

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/config_apply.go` - `desiredState`, `reconcileOnReadyWithJournal` (add pass, stale-removal pass), `addDesiredAddresses`
- [ ] `internal/component/iface/operation.go` - `decomposeIfaceOperations`, address additions and removals

**Behavior to preserve:** fresh-install order (`config_apply_order_test.go`); learned addresses (DHCP, SLAAC, link-local) never removed (`learnedAddrSet`); journaled rollback of each backend step.

**Behavior to change:** a reorder of a live unit's addresses reaches the kernel order.

## Data Flow (MANDATORY)

### Entry Point
- A commit that changes the order of a unit's `address` leaf-list.

### Transformation Path
1. `desiredState` yields the new configured order.
2. The reconcile (or the transaction's decomposition) compares it with the device's current order and re-adds from the first address out of place.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| iface ↔ kernel | netlink address add and delete through the backend | No |

### Integration Points
- The address migration's make-before-break, and the binders told of a disturbed address (`DisturbedAddresses`).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Deleting and re-adding the addresses after the new first one makes the kernel list it first | the kernel appends a new address after the existing ones of its family | another primitive is needed | an integration test against the kernel | unvalidated |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| commit reordering a live unit's addresses | → | reconcile reorder pass | to be named at design |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| reorder through the reconcile | `internal/component/iface/config_apply_order_test.go` | AC-1, AC-3 | |
| reorder through the transaction | `internal/component/iface/config_apply_order_test.go` | AC-2 | |

## Files to Modify
- `internal/component/iface/config_apply.go`, `internal/component/iface/operation.go`, `docs/features/interfaces.md` (drop the limit sentence once fixed)

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | Feature behavior changed? | Yes | `docs/features/interfaces.md` |

## Implementation Steps

1. Design the reorder pass against make-before-break, then tests, then code.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 demonstrated
- [ ] The targeted tests pass: `./le job run label unit-pkg command go test -count=1 ./internal/component/iface/` and `./le test integration iface`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Owned journal rows

Found by `spec-home-cpe-11-lab` (closed), whose lab exposed the address-order
defect that `3745936efa` fixed for a fresh install.

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `output-not-byte-stable.md` | 2026-11-08 | open: reordering a live interface's addresses does not move its primary |

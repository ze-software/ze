# Spec: vpp-nat44-pool-ownership

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P1 |
| Release reason | Under `firewall backend vpp`, an snat pool config is accepted that VPP cannot apply, a failed apply leaves addresses pooled with no owner, a second apply of an unchanged snat term fails, and a removed term's addresses are never released: configuration and lifecycle correctness of a shipped backend. |
| RFC gaps | - |
| Depends | `plan/release/validation/spec-finish-vpp-stub.md` (its stub answers no `nat44_*` message yet; the functional proof below needs it) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make the VPP NAT44 snat pool exact or refused. Overlapping snat ranges across
terms are refused at verify with the two term names; each apply reconciles
VPP's address pool against the set Ze applied, so an unchanged term is a no-op,
a removed term's addresses are deleted, and an undo deletes only the addresses
the failing apply added.

Found by comparing recent VyOS fixes against Ze (2026-11-08). VyOS fixed the
overlap half of this class for CGNAT, T9473: `9365e7cdc` ("reject external
addresses used in more than one range": an address listed in two ranges was
handed out twice), `036bed6e0` (overlapping internal ranges reported at verify)
and `a95094379` (external ranges overlapping internal ranges rejected).

VPP semantics, read in VPP master `src/plugins/nat/nat44-ed/` on 2026-11-08
(Ze targets the v26.06 API, `docs/guide/vpp.md`; confirm against that tag at
design): `vl_api_nat44_add_del_address_range_t_handler` loops over the range
calling `nat44_ed_add_address` or `nat44_ed_del_address` per address and
`goto send_reply` on the first non-zero return, so a range add is not atomic.
`nat44_ed_add_address` returns `VNET_API_ERROR_VALUE_EXIST` when the address is
already pooled. `nat44_ed_del_address` returns `VNET_API_ERROR_NO_SUCH_ENTRY`
for an unknown address and otherwise deletes the address together with every
dynamic session translated to it. The pool keeps no owner or reference count
per address. `nat44_ed_add_static_mapping` also answers `VALUE_EXIST` for a
mapping that already exists.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/firewall/fw-6-firewall-vpp.md` - NAT44-ED integration of the VPP firewall backend
  → Constraint: a backend that cannot apply a config exactly refuses it at verify (`ai/rationale/exact-or-reject.md`)
- [ ] `ai/doors/forwarding.md` - VPP semantics differ from Linux; Ze owns only what it marked
  → Constraint: pool addresses carry no tag, so ownership is the set Ze applied, never "every pooled address"

**Key insights:**
- The nft backend lowers overlapping snat ranges in separate rules without conflict; the overlap refusal belongs to the VPP verifier only.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/firewall/vpp/verify.go` - `verifyNATTerm` checks each term alone (address family, matches, actions). No check compares the snat ranges of different terms, chains or tables.
- [ ] `internal/plugins/firewall/vpp/nat_linux.go` - `applyNATChains` returns at once when no NAT chain is desired, so removing the last NAT chain removes nothing. Otherwise it enables NAT44, runs `cleanupNATOrphans` (static mappings by `ze/` tag only), then `applyNATChain` per chain. `applySNAT` calls `nat44AddDelAddressRange(first, last, true)` and returns any error unfiltered, then enables the inside feature on every interface. The per-chain undo deletes each earlier term's whole range and disables the inside feature on every interface, whoever else uses it; the failing term registers no undo, so the addresses VPP added before its `VALUE_EXIST` stay pooled.
- [ ] `internal/plugins/firewall/vpp/backend_linux.go` - `reconcileWithOps` calls `applyNATChains` on every `Apply`, so every firewall commit (any owner's `firewall.ApplyAll`) re-adds every snat range. `govppOps.nat44AddDelAddressRange` wraps `RetvalToVPPApiError`, whose text for `-81` is "Value already exists".
- [ ] `internal/plugins/firewall/vpp/ops.go` - the `vppOps` seam has `nat44StaticMappingDump` but no pool dump, so nothing reads VPP's pool back.

**Behavior to preserve:**
- `TestApplyNATSNAT` and the masquerade and dnat apply tests in `internal/plugins/firewall/vpp/apply_test.go`.
- The verify refusals already in `verify_test.go`.
- `cleanupNATOrphans` keeps static mappings Ze does not own (no `ze/` tag).

**Behavior to change:**
- Overlapping snat ranges are refused at verify.
- The snat pool is reconciled: add what is missing, delete what Ze applied and no longer wants, leave the rest.
- Removing every NAT chain withdraws Ze's pool addresses, static mappings and interface features.
- To confirm at design: whether an unchanged dnat term also fails a second apply on `VALUE_EXIST` from `nat44_ed_add_static_mapping`; if it does, this spec owns it under the same reconcile.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| VPP firewall verifier | `internal/plugins/firewall/vpp/verify.go` `Verify`, `verifyNATChain` | Per-term NAT checks, errors joined | Extends: a cross-term pass over every snat range in the desired tables |
| NAT static-mapping orphan sweep | `internal/plugins/firewall/vpp/nat_linux.go` `cleanupNATOrphans` | Dumps static mappings, deletes `ze/`-tagged ones no desired term names | Extends to the pool: dump with `nat44_address_dump` and diff against the addresses Ze applied |
| Applied-state record | `internal/plugins/firewall/vpp/backend_linux.go` `backend.lastApplied` | The tables of the last successful apply | Extends: the source of Ze's pool ownership, since pool addresses carry no tag |
| ACL discover and reconcile | `internal/plugins/firewall/vpp/backend_linux.go` `discoverACLs`, `reconcileRemovals` | Refreshes ownership from VPP each apply, deletes only owned ACLs | Pattern to follow for the pool |
| VPP operation seam | `internal/plugins/firewall/vpp/ops.go` `vppOps` | Narrow interface the fake substitutes in unit tests | Extends: one pool dump operation |
| VPP stub | `plan/release/validation/spec-finish-vpp-stub.md` | Lists `nat44_add_del_address_range` handling as missing | Depends: the functional proof needs the stub to model the pool, `VALUE_EXIST` included |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A config commit carrying `firewall` with `backend vpp` and a NAT chain with an `snat` term; any other owner's `firewall.ApplyAll` re-enters the same apply.

### Transformation Path
1. The firewall component builds `[]firewall.Table` and calls the backend verifier, then `Apply`.
2. `reconcileWithOps` applies ACLs, then `applyNATChains`, then classify actions.
3. `applySNAT` sends `nat44_add_del_address_range` to VPP over GoVPP.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to VPP | GoVPP binary API, `nat44_ed` messages | No |

### Changed-Wire Census

**Search evidence:** no wire format changes; the same `nat44_ed` messages are sent, plus a `nat44_address_dump` read. To be confirmed at design.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [none expected] | | | | |

### Integration Points
- `reconcileWithOps` - the NAT step becomes a reconcile, ordered like the ACL step.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Commit with two overlapping snat terms under `backend vpp` | → | `Verify` cross-term check | `test/vpp/vpp-nat44-snat-overlap-refused.ci` |
| Two commits with an unchanged snat term, then one removing it | → | pool reconcile in `applyNATChains` | `test/vpp/vpp-nat44-snat-pool-reconcile.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Test that proves it through the real entry point |
|-------|-------------------|-------------------|--------------------------------------------------|
| AC-1 | Two snat terms (same or different chain or table) whose ranges share an address | The commit is refused at verify, naming both terms and the shared address; VPP receives no NAT message | `TestVerifyRejectsOverlappingSNATRanges` and `test/vpp/vpp-nat44-snat-overlap-refused.ci` |
| AC-2 | Disjoint ranges, including adjacent ones | Accepted | `TestVerifyAcceptsAdjacentSNATRanges` |
| AC-3 | An unchanged snat term and a second commit (or another owner's `ApplyAll`) | The apply succeeds and sends no add for an address already pooled | Fake ops returning `VALUE_EXIST` for a pooled address; `test/vpp/vpp-nat44-snat-pool-reconcile.ci` |
| AC-4 | An snat term removed, or its range shrunk | Exactly the departed addresses are deleted; addresses Ze never applied stay | Same `.ci`, reading the pool back from the stub; unit test over the diff |
| AC-5 | The last NAT chain removed | Ze's pool addresses, `ze/` static mappings and the NAT features it enabled are withdrawn | Same `.ci`, final phase |
| AC-6 | An apply fails partway through a range | The addresses this apply added are deleted, addresses an earlier apply owns stay, and the commit fails naming VPP's answer | Fake ops failing mid-range |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVerifyRejectsOverlappingSNATRanges` | `internal/plugins/firewall/vpp/verify_test.go` | AC-1 | |
| `TestVerifyAcceptsAdjacentSNATRanges` | `internal/plugins/firewall/vpp/verify_test.go` | AC-2 | |
| `TestApplySNATPoolReconcile` | `internal/plugins/firewall/vpp/apply_test.go` | AC-3, AC-4, AC-5 | |
| `TestApplySNATUndoKeepsOwnedAddresses` | `internal/plugins/firewall/vpp/apply_test.go` | AC-6 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vpp-nat44-snat-overlap-refused` | `test/vpp/vpp-nat44-snat-overlap-refused.ci` | Operator commits overlapping pools and is told which terms collide | |
| `vpp-nat44-snat-pool-reconcile` | `test/vpp/vpp-nat44-snat-pool-reconcile.ci` | Recommit, shrink and remove a pool | |

### Interop Tests (Scope: protocol)
N-A: no protocol peer; VPP data-plane state only. A real-VPP run of the reconcile `.ci` is preferred where a VPP host is available.

## Files to Modify

- `internal/plugins/firewall/vpp/verify.go` - cross-term snat range overlap refusal
- `internal/plugins/firewall/vpp/nat_linux.go` - pool reconcile, removal when no NAT chain remains, undo scoped to what was added
- `internal/plugins/firewall/vpp/ops.go` - a pool dump operation
- `internal/plugins/firewall/vpp/backend_linux.go` - the govpp adapter for the dump
- `docs/architecture/firewall/fw-6-firewall-vpp.md` - pool ownership and the overlap refusal

## Files to Create

- `test/vpp/vpp-nat44-snat-overlap-refused.ci`
- `test/vpp/vpp-nat44-snat-pool-reconcile.ci`

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the two `.ci` files, red today (the second needs the stub's `nat44_*` handling from the dependency)
2. **Phase: Verify** -- cross-term overlap refusal
   - Tests: `TestVerifyRejectsOverlappingSNATRanges`, `TestVerifyAcceptsAdjacentSNATRanges`
3. **Phase: Pool reconcile** -- dump, diff against the applied set, add missing, delete departed, scoped undo
   - Tests: `TestApplySNATPoolReconcile`, `TestApplySNATUndoKeepsOwnedAddresses`
4. **Phase: Page** -- `docs/architecture/firewall/fw-6-firewall-vpp.md` in the same change

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete
- [ ] Scoped evidence: `./le test unit plugins` for `internal/plugins/firewall/vpp`, and `./le test vpp` for the two new `.ci` files

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `rollback-forgets-partial-apply.md` | 2026-11-08 | overlapping snat ranges pass verify; partial range add leaks, undo deletes whole ranges |
| `apply-re-adds-what-the-data-plane-already-holds.md` | 2026-11-08 | `applySNAT` re-adds pooled addresses (VALUE_EXIST); removed term's addresses never deleted |

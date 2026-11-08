# Spec: policyroute-reload-make-before-break

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P1 |
| Release reason | Every policy-route edit drops all policy steering for the length of the apply, and renumbers the marks, tables and rule priorities of policies the operator did not touch: reload correctness of a shipped feature. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A config commit that changes any `policy route` must leave the forwarding of
unchanged policies undisturbed, and must never leave a window in which a marked
packet has no `ip rule` and falls through to the main table. Today the reload is
break-before-make and the allocation is positional. The fix is make-before-break
over a stable allocation: install what is new, then delete only what is stale.

Found by comparing recent VyOS fixes against Ze (2026-11-08). VyOS fixed the
same class in WAN load balancing, T9472: `794320749` ("reload the health daemon
instead of restarting": its rules, routes and ip rules were gone until the
daemon came back; a commit now adds or removes only the routes and ip rules of
changed interfaces) and `2b97c4fec` ("keep marks of interfaces when others
change": table and mark followed the position in a sorted list, so adding one
interface renumbered all of them; now each keeps its table while configured and
a new one gets the lowest free table).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/policyroute/policy-routing.md` - the mark, table and `ip rule` model
  → Constraint: marks, auto tables and priorities stay inside the reserved ranges declared once beside the other producers' ranges
- [ ] `ai/doors/forwarding.md` - Ze owns only the kernel state it marked (`rtproto.PolicyRoute` routes, the `ze_` nft table)
  → Constraint: the stale-rule delete removes only rules and routes this plugin installed

**Key insights:**
- The marking is per packet at prerouting (`meta mark`), so the visible harm is the apply window and the renumbering, not stale conntrack marks.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/policyroute/register.go` - `applyPolicies` runs, in order: `alloc.reset()`; `alloc.translate(policies)`; `rm.removeAll(oldResult)`, which deletes every old `ip rule` and every old auto-table default route; `firewall.RegisterTables("policy-routes", ...)` and `firewall.ApplyAll()`, which replace the nft marking table; `rm.applyAll(result)`, which adds the new auto routes, then the new ip rules. Between the remove and the add, a packet the nft table marks finds no rule and is routed by the main table. On an `applyAll` failure it removes the new result and withdraws the nft table, so until the journal undo re-runs `applyPolicies` with the old set no policy is installed at all.
- [ ] `internal/plugins/policyroute/marks.go` - `allocator.reset` empties `markByKey` and `tableByNH` and restarts `nextMark` at `0x50000` and `nextTable` at the first auto table. A mark and an auto table are therefore a function of the policy's position in the candidate, not of its identity, and a removed key's value is never reused other than by the reset.
- [ ] `internal/plugins/policyroute/translate.go` - `allocator.translate` starts `basePriority` at 100 and `buildActions` increments it per marking rule, so the `ip rule` priority is positional too. The nft action is `firewall.SetMark` (a per-packet mark at prerouting), so no conntrack state carries the mark: the harm is the window and the renumbering seen by packets in flight.
- [ ] `internal/plugins/policyroute/rules_linux.go` - `applyIPRules` adds with `RuleAdd`; `removeIPRules` deletes with `RuleDel` and drops the error; `applyAutoRoutes` adds with `RouteAdd` (protocol `rtproto.PolicyRoute`); `removeAutoRoutes` drops errors. Nothing reads the kernel back.
- [ ] `internal/component/firewall/registry.go` - `RegisterTables` stores the owner's tables, `ApplyAll` hands the merged set to the backend in one apply.

**Behavior to preserve:**
- `test/policy/policy-reload.ci`: the old rule's effect is gone and the new one applies after a reload.
- `applyStagedPolicies`: an empty staged set is a removal and is applied; an apply without a verify is refused (`TestApplyStagedPoliciesRemovesWhenEveryPolicyIsGone`, `TestApplyStagedPoliciesRefusesAnApplyWithoutAVerify`).
- Marks, auto tables and priorities stay inside their declared reserved ranges.
- `cleanupOnShutdown` still removes every rule, route and the nft table.

**Behavior to change:**
- The reload becomes make-before-break: no instant at which a policy present in both the old and new config has no `ip rule`.
- A policy's mark, auto table and rule priority survive an edit to another policy.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Policy-route allocator | `internal/plugins/policyroute/marks.go` `allocator` | Dedups marks by `markKey` and tables by next hop within one apply; `reset` clears it on every apply | Extends: keep the maps across applies, give a new key the lowest free value, release a key that left the config |
| Verify-to-apply journal | `internal/plugins/policyroute/register.go` `applyStagedPolicies` | Install and undo arms, both `applyPolicies` | Extends: the undo arm runs the same diff from the new result back to the old one |
| Firewall table registry | `internal/component/firewall/registry.go` `RegisterTables`, `ApplyAll` | Replaces the merged nft table set in one backend apply | Kept as the way the marking table changes. The order around it changes: new routes and rules before the table swap, stale ones after it |
| Make-before-break address change | `plan/known-failures/RESOLVED.md` (iface `config_apply.go` adds the new address before removing the old) | Same ordering problem solved for interface addresses | Pattern to follow; no shared code |
| Policy-routing page | `docs/architecture/policyroute/policy-routing.md`, "Reserved ranges by construction" | States "Allocation is sequential with dedup" | Extends: the page states stable allocation and the add-then-delete order in the same change |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A config commit or reload carrying the `policy route` root, delivered to the plugin's `OnConfigVerify` and `OnConfigApply`.

### Transformation Path
1. `parsePolicyConfig` stages the candidate (`OnConfigVerify`).
2. `applyStagedPolicies` runs the install arm, `applyPolicies`.
3. `allocator.translate` builds the nft table, the `ip rule` list and the auto routes.
4. The rule manager and `firewall.ApplyAll` write the kernel.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin to kernel | netlink `RuleAdd`/`RuleDel`, `RouteAdd`/`RouteDel` | No |
| Plugin to firewall component | `firewall.RegisterTables`, `firewall.ApplyAll` (process-local registry) | No |

### Changed-Wire Census

**Search evidence:** no wire format changes; the netlink messages keep their shape, only their order and the allocated values change. To be confirmed at design.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [none expected] | | | | |

### Integration Points
- `applyStagedPolicies` - the install and undo arms both call the new diffing apply.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config reload editing one of two policies | → | `applyPolicies` diff path, stable `allocator` | `test/policy/policy-reload-make-before-break.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Test that proves it through the real entry point |
|-------|-------------------|-------------------|--------------------------------------------------|
| AC-1 | Policies A and B applied; the operator edits only B's match and commits | A's `ip rule` (mark, table, priority) and A's auto route are present at every sample taken while the commit runs, and identical before and after | New `test/policy/policy-reload-make-before-break.ci` (needs-linux, net-admin) with a fixture that samples `ip rule` across the commit; red under today's remove-first order |
| AC-2 | Policies A and B applied; a policy that sorts before both is added | A's and B's marks, auto tables and rule priorities are unchanged; the new policy gets values no live policy holds | Same `.ci`, second phase, comparing the dumped rules before and after; plus an `allocator` unit test across two `translate` calls |
| AC-3 | A policy is removed | Its rule, auto route and nft term are deleted after the new set is installed; no other policy's state changes; its mark and table become free for reuse | Same `.ci`, third phase; plus an `allocator` unit test |
| AC-4 | The new set fails to install (a rule add refused) | The old set is still installed and still steers traffic; the commit fails naming the cause | Unit test with a failing rule-manager seam, through `applyStagedPolicies` |
| AC-5 | An `ip rule` identical in the old and new results | It is neither deleted nor re-added (no EEXIST, no gap) | Unit test over the diff of two results |
| AC-6 | Shutdown | Unchanged: every rule, route and the table are removed | Existing shutdown coverage, rerun |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAllocatorKeepsValuesAcrossApplies` | `internal/plugins/policyroute/marks_test.go` | AC-2, AC-3 | |
| `TestApplyDiffKeepsUnchangedRules` | `internal/plugins/policyroute/reload_test.go` | AC-5 | |
| `TestApplyFailureKeepsOldSet` | `internal/plugins/policyroute/reload_test.go` | AC-4 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `policy-reload-make-before-break` | `test/policy/policy-reload-make-before-break.ci` | AC-1 to AC-3: an edit to one policy never removes another's rule | |

### Interop Tests (Scope: protocol)
N-A: no protocol peer; kernel state only.

## Files to Modify

- `internal/plugins/policyroute/register.go` - `applyPolicies` order: add new, swap table, delete stale
- `internal/plugins/policyroute/marks.go` - stable allocation, release of departed keys
- `internal/plugins/policyroute/translate.go` - priority taken from the allocator, not a running counter
- `internal/plugins/policyroute/rules_linux.go` - diff-based add and delete
- `docs/architecture/policyroute/policy-routing.md` - allocation and order

## Files to Create

- `test/policy/policy-reload-make-before-break.ci`
- A fixture under `internal/test/fixture/` that samples `ip rule` across the commit

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the `.ci` and its sampling fixture, red under today's remove-first order
   - Tests: `test/policy/policy-reload-make-before-break.ci`
2. **Phase: Stable allocation** -- the allocator keeps marks, tables and priorities by key across applies
   - Tests: `TestAllocatorKeepsValuesAcrossApplies`
3. **Phase: Make-before-break apply** -- add new routes and rules, swap the nft table, delete stale ones; failure keeps the old set
   - Tests: `TestApplyDiffKeepsUnchangedRules`, `TestApplyFailureKeepsOldSet`
4. **Phase: Page** -- `docs/architecture/policyroute/policy-routing.md` in the same change

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete
- [ ] Scoped evidence: `./le test unit plugins` for `internal/plugins/policyroute`, and `./le test policy` in the QEMU guest (needs-linux)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `component-rebuilt-during-reload.md` | 2026-11-08 | `applyPolicies` break-before-make; positional marks, tables and priorities |

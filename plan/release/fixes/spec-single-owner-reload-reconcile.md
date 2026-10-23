# Spec: single-owner-reload-reconcile

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | A reload or rollback that leaves the dataplane, the routing table or a running plugin out of step with the committed config is a shipped-daemon defect: the operator's commit says one thing and the box does another. |
| Depends | `plan/release/fixes/spec-single-owner-lifecycle.md` (the ownership check) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Family S3 of the journal backlog analysis (2026-10-23), part 2 of 3.

## Task

**Root cause.** A component's applied state has no single owner that computes
it from the committed config. Instead, several paths write it on their own
terms:

1. rollback is a separate restore path that re-derives state from somewhere
   other than the tree it was handed;
2. a reconcile repairs against a snapshot (an `activeCfg` the dataplane never
   received, a 1 Hz rate sample, two reads with an undelivered event between
   them) instead of against the owner's record of what it applied;
3. ownership of kernel or VPP objects is inferred from a name prefix or from
   this process's memory, so objects a previous process left, or another
   producer named differently, are never reclaimed or are claimed wrongly;
4. a policy or config change is stored and never re-applied to what the old
   policy already admitted or started;
5. a multi-field edit applies field by field with no rollback when one field
   is refused.

**Goal.** Each config-apply participant has one apply function that takes the
desired tree and the owner's record of what is applied, and produces the
difference. Rollback is that function run with the previous tree. Owned
kernel and VPP objects carry an ownership marker the owner writes, so a fresh
process reclaims what a previous one left. A registered conformance test
runs every participant through apply, change, rollback and restart, so a new
participant is tested without anyone listing it.

**Why it matters for shipping.** An operator who removes a redistribute rule
keeps the routes it admitted; a rejected BGP reload re-applies the peers it
rejected; an iface reconcile removes addresses the operator did not ask to
remove; a firewall table survives a restart with no owner; a reload never
starts the external plugin the new config declares.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/transaction-protocol.md` (config apply, verify, rollback)
  -> Constraint: a participant answers apply and rollback inside one transaction.
- [ ] `docs/architecture/core-design.md` (reload path, plugin config delivery)
- [ ] `ai/doors/system.md` and the config, iface and firewall doors in `ai/doors/README.md`
- [ ] `docs/architecture/isis/isis-5-adjacency.md` (circuit build from config)
- [ ] `docs/architecture/web-interface.md` (form save)
- [ ] `docs/features/interfaces.md` (learned addresses, reconcile)
- [ ] `docs/guide/flowspec-protected-router.md` (flowspec firewall table)
  -> Decision: the page names the table; renaming or marking it changes this page in the same work.

**Key insights:**
- The compensation path for a BGP reload already receives the tree it must
  restore, and the file route ignores it.
- iface already knows when an apply is owed; the registry reconcile does not ask.

## Current Behavior (MANDATORY)

Verified at the producer on 2026-10-23.

**Source files read:**
- [ ] `internal/component/plugin/server/reload_compensation.go` - `restoreReload`
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `loadPeersFullOrTree`
- [ ] `internal/component/iface/config_apply.go` - `reconcileOnRegistryChange`, `applyIsOwed`
- [ ] `internal/plugins/firewall/nft/backend_linux.go` - `shouldDeleteTable`
- [ ] `internal/component/web/editor.go` - `ApplyFormEdits`
- [ ] `internal/component/config/redistribute/evaluator.go` - `SetGlobal`
- [ ] `internal/component/plugin/server/startup_autoload.go` - `autoLoadForNewConfigPaths`

| Producer | What it does today | Rows it explains |
|----------|--------------------|------------------|
| `(*reactorAPIAdapter).loadPeersFullOrTree` (`internal/component/bgp/reactor/reactor_api.go`) | When a reload function and a config path exist, it calls `reloadFn(configPath)` and never reads its `bgpTree` argument. `restoreReload` hands it `undo.before`, so the compensation re-reads the staged candidate | rollback-forgets-partial-apply 2026-10-11 |
| `reconcileOnRegistryChange` (`internal/component/iface/config_apply.go`) | Loads `activeCfg` and reconciles addresses from it. It never calls `applyIsOwed`, so while vpp refused the last apply it reconciles against a config the dataplane never received | reconciler-repairs-against-a-stale-snapshot 2026-10-17 |
| `(*backend).shouldDeleteTable` (`internal/plugins/firewall/nft/backend_linux.go`) | Returns false for any table without the `ze_` prefix, and for a `ze_` table outside the desired set unless this backend instance's `applied` map holds it. A fresh process's map is empty, so a table a previous process left is never reclaimed, and a ze producer that names its table without the prefix is never replaced | gate-excludes-part-of-its-population 2026-09-09, 2026-08-18, 2026-08-22 |
| `(*EditorManager).ApplyFormEdits` (`internal/component/web/editor.go`) | Holds the user lock across the save (fixed 2026-10-17) and applies each field with `SetValue` or `DeleteValue`. On the first refused field it returns, leaving the fields before it applied | concurrent-session-corruption 2026-10-17 (rollback row) |
| `SetGlobal` (`internal/component/config/redistribute/evaluator.go`) | Stores the new evaluator in an atomic pointer. Nothing re-judges routes the old evaluator admitted | policy-change-never-re-evaluates-what-it-admitted 2026-10-08 |
| `(*Server).autoLoadForNewConfigPaths` (`internal/component/plugin/server/startup_autoload.go`) | Reads `registry.ConfigRootsMap()`, the internal plugins only. An `external` program the new config declares is never started | unwired-feature 2026-10-11 |

**Behavior to preserve:** transaction semantics (verify before apply, all or
nothing across participants); operator config that names tables without the
`ze_` prefix keeps its kernel name.

**Behavior to change:** every row in "Owned journal rows".

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `commit`, `rollback`, SIGHUP reload, web form save, daemon restart.

### Transformation Path
1. The transaction hands each participant the desired subtree.
2. The participant's single apply function reads its owner record (what it
   applied, with the ownership marker it wrote to the kernel or VPP object)
   and the observed state, and computes the difference.
3. It applies the difference and updates the owner record only for what
   succeeded.
4. Rollback calls the same function with the previous subtree.
5. A reconcile triggered by a registry, carrier or address event calls the same
   function with the last tree the dataplane accepted, never with a tree still
   owed.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config transaction <-> participant plugin | candidate subtree, apply and rollback | No |
| Participant <-> kernel and VPP | ownership marker on the object (nft table user data, route protocol, interface tag) | No |

### Integration Points
- The config transaction participant registry: the conformance test iterates it.
- The ownership check from `spec-single-owner-lifecycle.md`: the owner record
  fields are annotated so only the apply function writes them.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | |
| No unintended coupling | No | |
| No duplicated functionality | No | restore paths fold into apply |
| Zero-copy preserved where applicable | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | the conformance test derives its population from the participant registry |

## The fix

| Component | Single owner | Replaces |
|-----------|--------------|----------|
| BGP reload compensation | `loadPeersFullOrTree` parses the tree it is given, with defaults and inheritance (the full parse over a given tree) | re-reading the staged file |
| iface reconcile | every reconcile reads the last accepted config and refuses to run while an apply is owed; carrier and address judgements use event-ordered evidence (a link change counter read with each address read) | `activeCfg` and rate-tracker snapshots |
| iface learned addresses | addresses another owner installed (DHCP, SLAAC, VPP lifetimes) are registered with the owner and never removed by the config reconcile | the remaining VPP lifetime gap |
| nft tables | every ze producer writes an ownership marker into table user data; the backend reclaims any marked table outside the desired set, whatever its name | name prefix plus in-memory map |
| firewall merge | tables from different owners never merge by name; a collision is refused with what and why | `mergeSameNameTables` merging the VRRP owner table |
| web form save | the save applies to a copy of the working tree and swaps it in only when every field applied | field-by-field apply |
| redistribute | a rule change re-evaluates the held routes (`heldRoutes`) and withdraws those the new rule refuses | store-only `SetGlobal` |
| plugin set on reload | the reload diff covers external programs as well as registry plugins | registry-only auto-load |
| isis | a committed `net` or `system-id` change rebuilds the affected circuits | reading config only at circuit build |
| bmp | an `environment` change re-applies the receiver listeners | Stage-2-only `startReceiver` |

**The check that stops it coming back.** A conformance test registered over
the config transaction participant registry. For every participant that
declares a test fixture it runs: apply A, apply B, rollback to A, restart the
process, and asserts the observed state after each step equals the state a
fresh apply of that tree produces. A participant that declares no fixture fails
the test by name, so a new participant cannot skip it. The owner record fields
carry the ownership annotation, so a write outside the apply function is
refused at commit.

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `commit` then `rollback` over every participant | -> | each participant's apply | `TestEveryParticipantRoundTripsApplyRollbackRestart` (new) |
| BGP reload rejected by a plugin | -> | `loadPeersFullOrTree` on the given tree | `TestReloadCompensationRestoresPreviousPeers` (new, reactor) |
| daemon restart with a stale ze nft table | -> | marked-table reclaim | `TestRestartReclaimsMarkedTable` (new, Linux integration) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A participant registers with no conformance fixture | the conformance test fails naming it |
| AC-2 | Every registered participant: apply A, apply B, rollback, restart | observed state equals a fresh apply of the expected tree at every step |
| AC-3 | A BGP reload is rejected by another participant | the running peers equal the pre-reload peers, read from the running tree |
| AC-4 | vpp refuses an apply, then an interface registry event fires | no address is added or removed until the owed apply succeeds |
| AC-5 | DHCP, SLAAC and VPP-learned addresses exist on an enabled unit and a commit runs | none of them is removed |
| AC-6 | ze restarts after a previous process left a ze-owned nft table the new config does not request | the table is removed at startup; an operator table without the marker is untouched |
| AC-7 | An operator table whose name collides with a ze owner table | the commit is refused with what, why and the rename command |
| AC-8 | A web form save where the third field is refused | the working tree is unchanged |
| AC-9 | A reload narrows a redistribute rule | routes the old rule admitted and the new rule refuses are withdrawn at the destination |
| AC-10 | A reload adds an `external` plugin program | the program starts and completes its stages |
| AC-11 | A commit changes isis `net` or `system-id`, or bmp `environment` listeners | the running circuits or listeners reflect the change without a restart |
| AC-12 | iface carrier or address flap between two reads | the reconcile acts on event-ordered state; the counter its functional test reads counts each repair once |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEveryParticipantRoundTripsApplyRollbackRestart` | `internal/component/config/transaction/conformance_test.go` | AC-1, AC-2 | |
| `TestReloadCompensationRestoresPreviousPeers` | `internal/component/bgp/reactor/reactor_api_test.go` | AC-3 | |
| `TestRegistryReconcileWaitsForOwedApply` | `internal/component/iface/config_apply_test.go` | AC-4 | |
| `TestFormSaveRefusedFieldLeavesTreeUnchanged` | `internal/component/web/editor_test.go` | AC-8 | |
| `TestRuleNarrowingWithdrawsHeldRoutes` | `internal/component/config/redistribute/evaluator_test.go` | AC-9 | |
| `TestReloadStartsNewExternalPlugin` | `internal/component/plugin/server/startup_autoload_test.go` | AC-10 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `reload-reject-restores-peers` | `test/reload/` | a rejected reload leaves the previous peers running (AC-3) | |
| `redistribute-rule-narrowed` | `test/reload/` | narrowing a rule withdraws what it no longer admits (AC-9) | |
| `TestIntegrationReconcileKeepsLearnedAddresses` extended | `internal/component/iface/` QEMU integration | AC-5 with VPP lifetimes | |
| `TestRestartReclaimsMarkedTable` | `internal/plugins/firewall/nft/` Linux integration | AC-6 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | config apply only; the redistribute withdrawal is proven on the wire by the `.ci` above | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the conformance test over the participant registry, red for every participant
   - Tests: `TestEveryParticipantRoundTripsApplyRollbackRestart`
2. **Phase: BGP and plugin set** -- full parse over a given tree; external programs in the reload diff
   - Tests: AC-3, AC-10
3. **Phase: iface** -- owed-apply guard, learned-address owners, event-ordered evidence
   - Tests: AC-4, AC-5, AC-12
4. **Phase: firewall** -- ownership marker, reclaim, collision refusal
   - Tests: AC-6, AC-7
5. **Phase: web, redistribute, isis, bmp** -- AC-8, AC-9, AC-11
6. **Phase: fixtures** -- every participant declares its conformance fixture; AC-1 and AC-2 green

## Files to Modify
- `internal/component/plugin/server/reload_compensation.go`, `startup_autoload.go`
- `internal/component/bgp/reactor/reactor_api.go`
- `internal/component/iface/config_apply.go`, `link_queue.go`
- `internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go`
- `internal/plugins/firewall/nft/backend_linux.go`, `internal/component/firewall/registry.go`
- `internal/plugins/flowspec-firewall/state.go`
- `internal/component/web/editor.go`
- `internal/component/config/redistribute/evaluator.go`
- `internal/plugins/isis/circuits.go`, `server.go`
- `internal/component/bgp/plugins/bmp/bmp.go`

## Files to Create
- `internal/component/config/transaction/conformance_test.go`

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| rollback-forgets-partial-apply | 2026-10-11 | `restoreReload` -> `loadPeersFullOrTree` ignores `undo.before` |
| policy-change-never-re-evaluates-what-it-admitted | 2026-10-08 | redistribute `SetGlobal` stores, never re-judges held routes |
| reconciler-claims-state-another-owner-installed | 2026-10-18 | `reconcileOnReadyWithJournal` removes learned addresses (VPP lifetimes left) |
| reconciler-repairs-against-a-stale-snapshot | 2026-08-23 | `resyncCarrierState` judges carrier from a 1 Hz sample |
| reconciler-repairs-against-a-stale-snapshot | 2026-10-17 | `reconcileOnRegistryChange` ignores `applyIsOwed` |
| reconciler-repairs-against-a-stale-snapshot | 2026-10-19 | `goneWhileUp` misses an undelivered down and up |
| concurrent-session-corruption | 2026-10-17 | `ApplyFormEdits` refused field leaves earlier fields applied |
| gate-excludes-part-of-its-population | 2026-09-09 | `shouldDeleteTable` never reclaims a previous process's table |
| gate-excludes-part-of-its-population | 2026-08-18 | nft table ownership by `ze_` name prefix |
| gate-excludes-part-of-its-population | 2026-08-22 | flowspec-firewall table `flowspec` outside the prefix |
| two-owners-share-one-name | 2026-09-28 | firewall `mergeSameNameTables` merges operator table into VRRP owner table |
| unwired-feature | 2026-10-11 | `autoLoadForNewConfigPaths` never starts an external program |
| unwired-feature | 2026-09-20 | isis `buildCircuit` never sees a committed `net` or `system-id` change |
| unwired-feature | 2026-08-31 | bmp `environment` reload never re-applies the receiver |
| unwired-feature | 2026-09-06 | reload of a running `ze` daemon from the ExaBGP test runner |

15 rows.

## Rows found already fixed

| Class file | Date | Surface (short) | Evidence |
|------------|------|-----------------|----------|
| rollback-forgets-partial-apply | 2026-09-23 | VRRP shared-sysctl restore | `internal/plugins/vrrp/dataplane_linux.go` retains failed restorations independently of group references (comments on the restore and compaction helpers), and `dataplane_failure_linux_test.go` carries the rollback-failure and teardown-retry regressions the row names. Tests not run by this agent |

## Rows considered and not owned

| Class file | Date | Surface (short) | Why not this root cause |
|------------|------|-----------------|-------------------------|
| policy-change-never-re-evaluates-what-it-admitted | 2026-10-22 | GoBGP `UpdateConfig` export policy | third-party checkout, not Ze code |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Producer evidence re-read for every row before implementation starts
- [ ] No code snippets

### Goal Gates (MUST pass)
- [ ] AC-1 to AC-12 each proven by a named test
- [ ] Wiring Test table complete
- [ ] Targeted tests pass: `./le test unit` over the packages in Files to Modify, `./le test functional reload`, and `./le verify current fast`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: config apply)

### Closure
- [ ] Owned rows re-pointed to this spec's closure

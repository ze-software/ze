# Spec: single-owner-reload-reconcile

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | A reload or rollback that leaves the dataplane, the routing table or a running plugin out of step with the committed config is a shipped-daemon defect: the operator's commit says one thing and the box does another. |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-19 |

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

## Existing Mechanisms (MANDATORY)

Inventory read at the producers on 2026-11-14. It records what exists. Where
the "This spec" column needs a choice the design pass has not made, it says
undecided.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Config transaction coordinator | `internal/component/config/transaction/orchestrator.go` `TxCoordinator`, `Participant`; `docs/architecture/config/transaction-protocol.md`, sections 2 and 6 | Runs verify, apply and rollback over stream events for the participants of one transaction, one transaction at a time. A `Participant` carries the plugin name, its `ConfigRoots`, `WantsConfig`, `ConfigOperations` and budgets | Extends: the single apply function answers the existing apply and rollback events. The coordinator holds its participants per transaction, not in a process-wide registry, so which population the conformance test iterates (these participants, or the plugin registrations in the next row) is undecided and waits for the design pass |
| Plugin registration as reload participant | `internal/component/plugin/registry/registry.go` `Registration` (`ConfigRoots`, `ConfigReads`); `ai/doors/config-yang.md`, "Boundaries" | A plugin declares the roots it owns and the roots it reads, and the live plugin's `WantsConfig` makes it a reload participant | Extends: it is the registered population a conformance test can derive from without a hand-kept list |
| SDK apply journal | `docs/architecture/config/transaction-protocol.md`, section 7; `pkg/plugin/sdk/config_tx_gate.go` `configTxGate` | A plugin records each apply step with its undo, and rollback replays the undo steps in reverse. The SDK runs a rollback handler only for an apply it accepted that `config-committed` has not closed | Undecided: the Goal makes rollback the apply function run with the previous tree, which replaces the journal for a participant that moves to it. Whether the journal stays for the undo of a partial apply inside one apply waits for the design pass |
| Rejected reload compensation | `internal/component/plugin/server/reload_compensation.go` `recordReloadCompensation`, `restoreReload` | Keeps the trees before and after a reload. On rejection it runs a restore transaction over `undo.before`, then calls `RestoreCreatedPeers`, `ApplyConfigDiff(undo.before)` and `SetConfigTree(undo.before)` on the reactor | Extends: the compensation already hands the reactor the tree to restore; the BGP row is that the reactor reads the file instead |
| BGP full-pipeline reload parse | `internal/component/bgp/reactor/reactor_api.go` `reactorAPIAdapter.loadPeersFullOrTree`; `plan/learned/DESIGN-HISTORY.md`, "Reload re-parses peers through the FULL config pipeline, never the partial tree parser" | With a reload function and a config path, it re-reads and re-parses the file through the full pipeline and ignores its `bgpTree` argument. Without them, it parses the given tree with `GlobalsFromTree` and `PeersFromTree` alone | Extends: the fix keeps the DESIGN-HISTORY constraint (the full pipeline, never the partial parser) and runs it over the given tree |
| iface owed-apply record | `internal/component/iface/config_apply.go` `applyIsOwed`, `applyOwedConfig` | Records an apply that a not-ready backend deferred, and performs it when vpp connects. The startup, reload and vpp-connect paths in `internal/component/iface/register.go` read it; `reconcileOnRegistryChange` does not | Extends: the registry reconcile reads the same record |
| iface address owner registry | `internal/component/iface/address_owner.go` `RegisterOwnedAddresses`, `UnregisterOwnedAddresses` | Another owner declares the addresses it owns on an interface, and each change triggers `reconcileOnRegistryChange` | Extends: the learned-address row registers the DHCP, SLAAC and VPP-lifetime addresses here |
| Kernel route ownership by protocol number | `ai/doors/forwarding.md`, "Who depends on it from outside" | Ze owns kernel routes by `rtm_protocol` (`FIBKernel` 250, `Static` 251) | Does not apply to the rows here, which are about nft tables, addresses and VPP objects. Whether the new markers follow the same shape is undecided |
| nft table ownership prefix and one-time legacy sweep | `internal/plugins/firewall/nft/backend_linux.go` `backend.Apply`, `shouldDeleteTable`, `isLegacyTable`; `docs/architecture/firewall/table-ownership-and-shutdown-flush.md`, Rules 2 and 2a | The backend deletes a `ze_` table that is desired or held in this instance's `applied` map. On the first reconcile of a process only, it deletes the tables an earlier build wrote without the prefix, judged by name, family and chains | Replaces, per "The fix": the user-data marker replaces the prefix plus the in-memory map as the reclaim test. How the one-time legacy sweep relates to the marker is undecided |
| Firewall table registry | `internal/component/firewall/registry.go` `RegisterTables`, `ApplyAll`, `mergeSameNameTables` | `RegisterTables` refuses a table name without the `ze_` prefix. `ApplyAll` merges same-name tables of different owners and applies the merged set | Extends: every table this registry hands the backend carries the marker |
| Firewall table name reservation | `internal/component/firewall/reservation.go` `MustReserveTableNames`, `refuseForeignReservations`, `refuseReservedTableNames`; `docs/architecture/firewall/table-ownership-and-shutdown-flush.md`, Rule 2b | A producer reserves its table names from `init()`. A registration by another owner, and an operator table whose kernel name is reserved, are refused | Extends: it is the collision refusal the firewall merge row asks for, for reserved names |
| Redistribute import chain and held-route reconcile | `internal/plugins/redistribute/config.go` `compileRuleChains`, `ruleState.apply`, `reconcileConsumers`; `internal/plugins/redistribute/held.go` `withdrawRejectedHeld`; `docs/guide/redistribution.md`, "Route Redistribution"; `docs/guide/route-policy.md`, "Where you attach a chain" | Each import rule's `filters` and `final` compile against the `redistribute import` attachment of the route policy library. A reload's apply installs the new evaluator and reconciles: it withdraws the held routes the new rules reject, then replays every producer. Rollback does the same with the previous evaluator | Extends: AC-9 is judged against this path. The design pass re-reads it first, because the apply already reconciles the held routes |
| isis node-identity reconcile | `internal/plugins/isis/server.go` `engine.reconcile`, `circuitNeedsRebuild`; `internal/plugins/isis/circuits.go` `buildCircuit` | Reconcile detects a node-identity change (System ID, level or NETs) and closes and reopens every desired circuit when one occurs. `buildCircuit` reads the System ID and the areas from the active config | Extends: the design pass re-reads it first, because it bears on the isis half of AC-11 |
| bmp receiver reload rail | `internal/component/bgp/plugins/bmp/bmp.go` `OnConfigVerify`, `OnConfigApply`, `applyReceiverConfig` | Verify parses the `environment` receiver config, and apply re-applies the listeners, with a rollback to the replaced config | Extends: the design pass re-reads it first, because it bears on the bmp half of AC-11 |
| Compiler-enforced single writer | `plan/release/fixes/spec-single-owner-lifecycle.md`, "Owner decisions" D1; `docs/contributing/ze-go-style.md`, "One type per lifecycle state" | No annotation gate and no typed analyzer is built. A state that must have one writer lives in a type with unexported fields in a package of its own | Extends: the owner record takes that form, so the apply function is its only writer |

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
- The single-writer rule of `spec-single-owner-lifecycle.md`, decision D1: the
  owner record's fields live in a type with unexported fields in a package of
  its own, so the apply function is the only writer and the compiler enforces
  it. No annotation check is built.

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
the test by name, so a new participant cannot skip it. The owner record's
fields live in a type with unexported fields in a package of its own, so a
write outside the apply function does not compile
(`spec-single-owner-lifecycle.md`, decision D1).

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
| AC-9 | A reload narrows a redistribute import's route-policy chain (`redistribute { destination <d> { import <s> { filters [ ... ]; final ...; } } }`: a chain of the route policy library at the `redistribute import` attachment, `docs/guide/route-policy.md` "Where you attach a chain" and `docs/guide/redistribution.md` "Route Redistribution", compiled by `compileRuleChains` in `internal/plugins/redistribute/config.go`; an import rule carries no family or tag field, and family and tag are `match` conditions of the `policy` root) | routes the old chain admitted and the new chain refuses are withdrawn at the destination |
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
| `TestRuleNarrowingWithdrawsHeldRoutes` | `internal/plugins/redistribute/config_test.go` (the orchestrator's apply re-judges every held route against the newly compiled route-policy chain) | AC-9 | |
| `TestReloadStartsNewExternalPlugin` | `internal/component/plugin/server/startup_autoload_test.go` | AC-10 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `reload-reject-restores-peers` | `test/reload/reload-reject-restores-peers.ci` | a rejected reload leaves the previous peers running (AC-3) | |
| `redistribute-rule-narrowed` | `test/reload/redistribute-rule-narrowed.ci` | narrowing a redistribute import's route-policy chain (a `filters` entry added, or `final accept` turned to `final reject`) withdraws what it no longer admits (AC-9) | |
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
- `internal/plugins/redistribute/config.go`, `held.go` (AC-9: the apply re-judges held routes against the route-policy chain that `compileRuleChains` compiles from the rule's `filters` and `final`, `docs/guide/route-policy.md`), and their page `docs/guide/redistribution.md` (what a reload withdraws)
- `internal/plugins/isis/circuits.go`, `server.go`
- `internal/component/bgp/plugins/bmp/bmp.go`

## Files to Create
- `internal/component/config/transaction/conformance_test.go`

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| rollback-forgets-partial-apply | 2026-10-11 | `restoreReload` -> `loadPeersFullOrTree` ignores `undo.before` |
| rollback-forgets-partial-apply | 2026-11-19 | `updateWebCertificate` and `updateLGCertificate` answer no restore call, so a rolled-back reload keeps the new certificate (read, not run) |
| policy-change-never-re-evaluates-what-it-admitted | 2026-10-08 | redistribute `SetGlobal` stores, never re-judges held routes |
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
| announced-state-never-replayed | 2026-09-06 | `api-reload.1.conf` two static routes, one advertised |
| gate-excludes-part-of-its-population | 2026-08-25 | `Server.ReloadConfig` verifies no plugin for a root the reload adds |
| unwired-feature | 2026-09-06 | reload of a running `ze` daemon from the ExaBGP test runner |

16 rows.

## Rows found already fixed

| Class file | Date | Surface (short) | Evidence |
|------------|------|-----------------|----------|
| rollback-forgets-partial-apply | 2026-09-23 | VRRP shared-sysctl restore | `internal/plugins/vrrp/dataplane_linux.go` retains failed restorations independently of group references (comments on the restore and compaction helpers), and `dataplane_failure_linux_test.go` carries the rollback-failure and teardown-retry regressions the row names. Tests not run by this agent |
| check-mode-mutates-the-tree | 2026-10-11 | bgp `createReloadFunc` applies dynamic groups during verify | AC-23 of the closed spec-bgp-live-receive-binding-and-bfd-keep-session (`a7b42bdc54`): `createReloadFunc` (`internal/component/bgp/config/loader_create.go`) now returns the groups and applies nothing; `TestTreeRouteLeavesDynamicGroupsUnchanged` (`reactor_dynamic_swap_test.go`). Row closed in its journal file |

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

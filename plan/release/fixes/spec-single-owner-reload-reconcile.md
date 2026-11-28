# Spec: single-owner-reload-reconcile

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | Umbrella of five release fixes for reload and reconcile defects that leave the dataplane, the routing table, a web candidate or a peer LSDB out of step with the committed config. It holds the owner decision record, the source-verified retirement of ten rows already landed or superseded at HEAD, and the routing of every remaining open row to five narrow child specs; it edits no code itself. |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Set: this file is the umbrella of `spec-single-owner-reload-reconcile-<N>-<name>.md`, children 1 to 5, all in `plan/release/fixes/`. The children carry every remaining acceptance criterion, test, and code edit, each scoped to its own subsystem door (`ai/META-DECISIONS.md`, "How agents work"). This umbrella carries the 2026-11-26 producer audit, the 2026-11-24 owner decision, and the routing of all 17 original journal rows.

Family S3 of the journal backlog analysis (2026-10-23), part 2 of 3. Part 1 is `plan/release/fixes/spec-single-owner-lifecycle.md`.

## Owner decisions (2026-11-24)

| Topic | Decision |
|-------|----------|
| AC-10 (reload starting newly declared `external` plugins) | Dropped and superseded by `RefuseChangedDeclarations` (`ai/doors/plugin-infra.md`). External plugin declarations stay fixed while the daemon runs; `spec-reload-starts-declared-external-programs` is likewise superseded and removed. |

## Task

**The original diagnosis (2026-10-23).** Seventeen journal rows across nine subsystem doors (config transactions, plugin server, BGP reactor, BMP, redistribute, iface, firewall, web editor, IS-IS, and the ExaBGP compat runner) were gathered under one umbrella: reload and reconcile paths lacked a single owner over applied state.

**What producer re-verification at HEAD showed (2026-11-26).** Every original acceptance criterion (AC-1 to AC-12) and all 17 rows (the original prose counted "16 rows" over 17 table rows) were re-read at the producing code:

- **Ten of the 17 rows are already fixed or superseded at HEAD:** BGP reload compensation parses the tree it is handed (`loadPeersFromConfig`, `internal/component/bgp/reactor/reload_tree.go`), hub reload restores web and LG TLS certificates on failure (`restoreCertificates`, `cmd/ze/hub/main_reload.go`), redistribute re-judges held routes and withdraws rejected ones on reload (`withdrawRejectedHeld`, `internal/plugins/redistribute/held.go`, `test/plugin/redistribute-chain-narrow-reload.ci`), firewall reserves producer table names against collisions (`MustReserveTableNames`, `internal/component/firewall/reservation.go`, `test/parse/firewall-reserved-table-name-rejected.ci`), `flowspec-firewall` uses `ze_flowspec` with one-time legacy sweep (`internal/plugins/flowspec-firewall/state.go`, `legacy_tables.go`), BMP reloads `environment` listeners with rollback (`applyReceiverConfig`, `internal/component/bgp/plugins/bmp/bmp.go`), the ExaBGP compat runner parses `signal:` and drives SIGHUP reloads over multi-config cases (`d064818a9b`, `ae49f963d0`), and AC-10 (starting newly declared external plugins on reload) was dropped by owner decision on 2026-11-24 (`RefuseChangedDeclarations`).
- **Seven rows remain genuinely open across five subsystem areas:** pre-autoload verification of newly added config roots plus the registered reload-participant conformance suite (Child 1), three iface stale-snapshot defects plus VPP-shaped learned-address unit coverage (Child 2), durable kernel `NFTA_TABLE_USERDATA` ownership for firewall nftables reclaim across restarts (Child 3), atomic web form save rollback across `web` and `cli.Editor` (Child 4), and IS-IS old-SystemID LSP purge on `SystemID` reload (Child 5; circuit rebuild already landed at HEAD).

**Goal.** Split the remaining work into five single-door child specs (`spec-single-owner-reload-reconcile-1` through `-5`), retire every already-landed item with exact file/symbol evidence, keep zero code edits in this umbrella, and ensure every child respects the 2026-11-26 "Ze always starts" ruling (`docs/architecture/core-design.md`, `## Design decisions`), the process boundary, and D1 (compiler-enforced single-writer types, no annotation gate).

## Children

| # | Spec | What it fixes | Doors it names | Status |
|---|------|---------------|----------------|--------|
| 1 | `plan/release/fixes/spec-single-owner-reload-reconcile-1-tx-conformance.md` | Runs `config.VerifyPluginConfigMapTransition(running, newTree)` in `Server.reloadConfig` before `autoLoadForReload` (without affecting `LoadConfigForStart`); registers reload conformance fixtures on `registry.Registration` and tests apply/change/rollback/restart across `registry.All()` in `plugin/all` | config-yang, plugin-infra | design |
| 2 | `plan/release/fixes/spec-single-owner-reload-reconcile-2-iface-reconcile.md` | Guards `reconcileOnRegistryChange` with `applyIsOwed()`; orders `resyncCarrierState` snapshots by monotonic carrier sequence; surfaces Linux `IFLA_CARRIER_CHANGES` on `InterfaceInfo`/`AddressState` so DHCPv6 `goneWhileUp` never Declines an undelivered down+up; adds unit coverage for VPP-shaped zero-lifetime registered leases | forwarding | design |
| 3 | `plan/release/fixes/spec-single-owner-reload-reconcile-3-firewall-nft-ownership.md` | Stamps every `firewall/nft` table with Linux `NFTA_TABLE_USERDATA` (`ze:firewall:v1`) via `mdlayher/netlink`, reclaims marked orphans across restarts even with empty `b.applied`, preserves atomic in-place rule replacement, and leaves unmarked foreign tables and `ze_iface_rpf` untouched | forwarding | design |
| 4 | `plan/release/fixes/spec-single-owner-reload-reconcile-4-web-form-atomic-save.md` | Adds `ApplyFieldEdits` to `contract.Editor` and `cli.Editor` using cloned working-tree and sparse-change-file staging under one `draftLock`, so `EditorManager.ApplyFormEdits` leaves working tree and change file untouched on any refused field | api-surfaces, commands-cli | design |
| 5 | `plan/release/fixes/spec-single-owner-reload-reconcile-5-isis-identity-purge.md` | Purges and floods all own ordinary and DIS pseudo-node LSPs of `oldCfg.SystemID` (ISO/IEC 10589 clause 7.3.16) before tearing down and rebuilding circuits under `newCfg.SystemID`, proven by a named FRR interop scenario | routing-protocols | design |

Every child also names `ai/doors/system.md` and `ai/doors/le-tooling.md`. No child edits a file another child edits; they can be implemented in any order.

## Row routing

All 17 original rows of `## Owned journal rows` re-verified against HEAD on 2026-11-26:

| # | Class file | Date | Surface (short) | Outcome | Journal marker today |
|---|------------|------|-----------------|---------|----------------------|
| 1 | `rollback-forgets-partial-apply` | 2026-10-11 | `restoreReload` -> `ApplyConfigDiff(undo.before)` | Already fixed at HEAD (`reload_tree.go` `loadPeersFromConfig`, `reactor_api.go:1022` `ApplyConfigDiff`); kept in umbrella owned table until journal row flips to `fixed:` | open |
| 2 | `rollback-forgets-partial-apply` | 2026-11-19 | `updateWebCertificate` and `updateLGCertificate` rollback | Already fixed at HEAD (`cmd/ze/hub/main_reload.go:941-996, 1217-1266` `runningListenerCertificates`, `restoreOn`, `restoreCertificates`; `TestRollbackReloadRestoresInstalledListenerCertificate`); kept in umbrella owned table until journal row flips to `fixed:` | open |
| 3 | `policy-change-never-re-evaluates-what-it-admitted` | 2026-10-08 | redistribute `SetGlobal` never re-judges held routes | Already fixed at HEAD (`internal/plugins/redistribute/config.go:146-193` `reconcileConsumers`, `held.go:597-616` `withdrawRejectedHeld`; `TestRuleNarrowingWithdrawsHeldRoutes`, `test/plugin/redistribute-chain-narrow-reload.ci`); kept in umbrella owned table until journal row flips to `fixed:` | open |
| 4 | `reconciler-repairs-against-a-stale-snapshot` | 2026-08-23 | `resyncCarrierState` judges carrier from a 1 Hz sample | Child 2 | open |
| 5 | `reconciler-repairs-against-a-stale-snapshot` | 2026-10-17 | `reconcileOnRegistryChange` ignores `applyIsOwed` | Child 2 | open |
| 6 | `reconciler-repairs-against-a-stale-snapshot` | 2026-10-19 | `goneWhileUp` misses an undelivered down and up | Child 2 | open |
| 7 | `concurrent-session-corruption` | 2026-10-17 | `ApplyFormEdits` refused field leaves earlier fields applied | Child 4 | open |
| 8 | `gate-excludes-part-of-its-population` | 2026-09-09 | `shouldDeleteTable` never reclaims a previous process's table | Child 3 | open |
| 9 | `gate-excludes-part-of-its-population` | 2026-08-18 | nft table ownership by `ze_` name prefix | Already fixed at HEAD (`spec-fixit-flowspec-table-never-swept`, `RegisterTables` prefix enforcement + `legacy_tables.go`; prose inside journal cell notes `FIXED 2026-08-23`); kept in umbrella owned table until marker flips from `open:` to `fixed:` | open |
| 10 | `gate-excludes-part-of-its-population` | 2026-08-22 | flowspec-firewall table `flowspec` outside the prefix | Already fixed at HEAD (`internal/plugins/flowspec-firewall/state.go:13` `ze_flowspec`, `legacy_tables.go`; prose inside journal cell notes `FIXED 2026-08-23`); kept in umbrella owned table until marker flips from `open:` to `fixed:` | open |
| 11 | `two-owners-share-one-name` | 2026-09-28 | firewall `mergeSameNameTables` merges operator table into VRRP owner table | Already fixed at HEAD (`internal/component/firewall/reservation.go` `MustReserveTableNames`; `test/parse/firewall-reserved-table-name-rejected.ci`) | fixed |
| 12 | `unwired-feature` | 2026-10-11 | `autoLoadForNewConfigPaths` never starts an external program | Dropped and superseded by owner decision 2026-11-24 (`RefuseChangedDeclarations`, `cmd/ze/hub/main_reload.go:83`, `test/reload/external-yang-reload-refused.ci`) | fixed |
| 13 | `unwired-feature` | 2026-09-20 | isis `buildCircuit` / `reconcile` on `net` or `system-id` change | Circuit rebuild landed (`server.go:721`, `TestRFC3786NormalHelloIdentityReload`); residual old-SystemID LSP purge routed to Child 5 | open |
| 14 | `unwired-feature` | 2026-08-31 | bmp `environment` reload never re-applies receiver | Already fixed at HEAD (`internal/component/bgp/plugins/bmp/bmp.go:577-679` `applyReceiverConfig`; `test/reload/bmp-receiver-bind-conflict.ci`; prose inside journal cell notes `FIXED 2026-09-20`); kept in umbrella owned table until marker flips from `open:` to `fixed:` | open |
| 15 | `announced-state-never-replayed` | 2026-09-06 | `api-reload.1.conf` two static routes, one advertised | Already fixed at HEAD (commit `ae49f963d0` confirms `3.0.0.0/24` announce and `2.0.0.0/24` withdraw both arrive on `api-reload`; `test/exabgp-compat/native/api-reload.1.conf` declares both prefixes); kept in umbrella owned table until journal row flips to `fixed:` | open |
| 16 | `gate-excludes-part-of-its-population` | 2026-08-25 | `Server.ReloadConfig` verifies no plugin for a root the reload adds | Child 1 | open |
| 17 | `unwired-feature` | 2026-09-06 | reload of a running `ze` daemon from the ExaBGP test runner | Already fixed at HEAD (`d064818a9b`: `exabgp_case.go` parses `signal:`, `cmd_exabgp.go:1058-1101` migrates all configs into `reloads`, `cmd_exabgp_process.go:143-225` writes next config and sends `SIGHUP`); kept in umbrella owned table until journal row flips to `fixed:` | open |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, process boundary, registration over enumeration
- [ ] The subsystem door each child names in `## Children` (`ai/META-DECISIONS.md`, "How agents work")
- [ ] `docs/architecture/core-design.md` - Section 18 and `## Design decisions` (2026-11-26 "Ze always starts")
- [ ] `docs/contributing/ze-go-style.md` - "One type per lifecycle state" (D1 from `spec-single-owner-lifecycle.md`)
- [ ] `plan/journal/README.md` - "Owned rows"

## Current Behavior (MANDATORY)

Verified at the producers on 2026-11-26. The umbrella edits no code itself; each child carries the full producer reading for its own files. This table records the 10 retired items verified landed or superseded at HEAD:

**Source files read:**
- [ ] `internal/component/bgp/reactor/reload_tree.go` & `reactor_api.go` - `ApplyConfigDiff` parses via `loadPeersFromConfig(tree)` (never re-reads the staged file).
- [ ] `cmd/ze/hub/main_reload.go` & `main_reload_pki_test.go` - `runningListenerCertificates` / `restoreCertificates` restores web and LG TLS certificates on reload rollback (`TestRollbackReloadRestoresInstalledListenerCertificate`).
- [ ] `internal/plugins/redistribute/config.go`, `held.go`, `config_test.go` - `ruleState.apply` / `rollback` calls `reconcileConsumers` -> `withdrawRejectedHeld` + `replayConsumers` (`TestRuleNarrowingWithdrawsHeldRoutes`, `test/plugin/redistribute-chain-narrow-reload.ci`).
- [ ] `internal/component/firewall/reservation.go` & `internal/plugins/flowspec-firewall/state.go` - `MustReserveTableNames` + `refuseReservedTableNames` refuse collisions (`test/parse/firewall-reserved-table-name-rejected.ci`); FlowSpec uses `ze_flowspec` with one-time `legacy_tables.go` sweep.
- [ ] `internal/component/bgp/plugins/bmp/bmp.go` - `OnConfigVerify` / `OnConfigApply` / `OnConfigRollback` call `applyReceiverConfig` for `environment` (`test/reload/bmp-receiver-bind-conflict.ci`).
- [ ] `internal/component/plugin/server/reload_declarations.go` & `cmd/ze/hub/main_reload.go` - `RefuseChangedDeclarations` supersedes AC-10 (owner decision 2026-11-24).
- [ ] `internal/le/interoplab/bgp/exabgp_case.go`, `exabgp_server.go`, `internal/test/cli/cmd_exabgp.go`, `cmd_exabgp_process.go` - ExaBGP compat runner parses `signal:`, migrates every `option=file:`, writes next config and sends `SIGHUP` (`d064818a9b`, `ae49f963d0`).

**Behavior to preserve:**
- Every child's "Behavior to preserve" section, plus all 10 retired mechanisms above.

**Behavior to change:**
- Only what Children 1 to 5 specify.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| BGP full-pipeline tree reload | `internal/component/bgp/reactor/reload_tree.go` `TreeReloadFunc`, `SetTreeReloadFunc`, `loadPeersFromConfig`; `reactor_api.go` `ApplyConfigDiff` | Parses the given tree through the full pipeline (defaults, inheritance, dynamic groups) without reading the staged candidate file; `TestTreeRouteReadsTheBGPBlockOfTheConfigTree` | Retired (already fixed at HEAD): original AC-3 and row `rollback-forgets-partial-apply` 2026-10-11 are satisfied |
| Hub TLS certificate reload rollback | `cmd/ze/hub/main_reload.go` `runningListenerCertificates`, `listenerCertificateNames.restoreOn`, `restoreCertificates`; `main_reload_pki_test.go` `TestRollbackReloadRestoresInstalledListenerCertificate` | Captures prior web and LG certificate/transport config and restores them on deferred reload rollback | Retired (already fixed at HEAD): row `rollback-forgets-partial-apply` 2026-11-19 is satisfied |
| Redistribute held-route reconcile on reload | `internal/plugins/redistribute/config.go` `ruleState.apply`, `rollback`, `reconcileConsumers`; `held.go` `withdrawRejectedHeld` | Re-judges held routes against newly compiled route-policy chains and withdraws rejected ones on apply and rollback (`TestRuleNarrowingWithdrawsHeldRoutes`, `test/plugin/redistribute-chain-narrow-reload.ci`) | Retired (already fixed at HEAD): original AC-9 and row `policy-change-never-re-evaluates-what-it-admitted` 2026-10-08 are satisfied |
| Firewall table name reservation and `ze_` prefix + legacy sweep | `internal/component/firewall/reservation.go` `MustReserveTableNames`, `refuseReservedTableNames`; `registry.go` `RegisterTables`; `internal/plugins/flowspec-firewall/state.go` `ze_flowspec`; `legacy_tables.go` | Refuses colliding reserved table names (`test/parse/firewall-reserved-table-name-rejected.ci`) and unprefixed registrations; sweeps legacy names once | Retired for AC-7 and rows 2026-08-18 / 2026-08-22 / 2026-09-28; Child 3 extends the nft backend with `NFTA_TABLE_USERDATA` for cross-restart reclaim (row 2026-09-09) |
| External plugin declaration refusal on reload | `cmd/ze/hub/main_reload.go` `diskConfigLoaders`; `internal/component/plugin/server/reload_declarations.go` `RefuseChangedDeclarations` | Refuses reloads changing declared external plugins (`test/reload/external-yang-reload-refused.ci`) | Superseded by Owner decision 2026-11-24: AC-10 dropped |
| BMP receiver reload rail | `internal/component/bgp/plugins/bmp/bmp.go` `OnConfigVerify`, `OnConfigApply`, `OnConfigRollback`, `applyReceiverConfig` | Reconciles `environment` BMP receiver listeners with rollback (`test/reload/bmp-receiver-bind-conflict.ci`) | Retired (already fixed at HEAD): BMP half of original AC-11 and row `unwired-feature` 2026-08-31 are satisfied |
| ExaBGP compat reload driver | `internal/le/interoplab/bgp/exabgp_case.go`, `exabgp_server.go`; `internal/test/cli/cmd_exabgp.go`, `cmd_exabgp_process.go` | Parses `signal:` directives, migrates all `option=file:` configs, writes next config and sends `SIGHUP` (`d064818a9b`, `ae49f963d0`) | Retired (already fixed at HEAD): rows `announced-state-never-replayed` 2026-09-06 and `unwired-feature` 2026-09-06 are satisfied |
| Child 1-5 mechanisms | Each child's `## Existing Mechanisms` table | `VerifyPluginConfigMapTransition`, `registry.All()`, `applyIsOwed`, `linkEventQueue`, `counterSource`, `NFTA_TABLE_USERDATA` via `mdlayher/netlink`, `cli.Editor` write-through draft, `isis/lsdb.Originator` | Extended in Children 1 to 5 |
| Compiler-enforced single writer (D1) | `plan/release/fixes/spec-single-owner-lifecycle.md` D1; `docs/contributing/ze-go-style.md` "One type per lifecycle state" | Unexported fields in a dedicated type/package enforce single writer at compile time; no annotation gate | Applies across all five children |

## Data Flow (MANDATORY)

### Entry Point
- None of its own. Each child names the entry point of its fix: `Server.reloadConfig` and `registry.All()` (Child 1), `reconcileOnRegistryChange` / `linkEventQueue` / `v6Worker` (Child 2), `firewall/nft` `Apply` (Child 3), `EditorManager.ApplyFormEdits` (Child 4), and IS-IS `engine.reconcile` (Child 5).

### Transformation Path
1. Each remaining open defect and journal row from the 2026-10-23 analysis is routed to the single child spec behind its subsystem door.
2. Each child designs, tests, implements, and closes its own fix independently.
3. Once all five children close and the eight already-fixed-at-HEAD journal rows flip from `open:` to `fixed:`, this umbrella closes.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| none | Umbrella edits no code; each child holds its own table | Yes, by Files sections below |

### Changed-Wire Census

This umbrella edits no code and changes no wire format. Children 1 to 4 change no network wire format; Child 5 emits standard ISO/IEC 10589 clause 7.3.16 zero-lifetime IS-IS LSP purge PDUs on `SystemID` change (documented in Child 5's own census).

**Search evidence:** The umbrella lists zero files to modify or create; each child carries its own source-verified census.

### Integration Points
- Each child names its own.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | Umbrella adds no path; each child closes a bypass at its choke point |
| No unintended coupling | Yes | Zero file overlap across Children 1 to 5 |
| No duplicated functionality | Yes | Ten already-landed items retired rather than rebuilt; each child extends existing mechanisms |
| Zero-copy preserved where applicable | Yes | Control-plane reload/reconcile fixes only |
| Registration over hardcoding, outbound | Yes | Child 1 registers conformance fixtures per plugin; Child 3 marks kernel tables directly |
| Registration over hardcoding, inbound | Yes | Child 1 iterates `registry.All()` |

## Wiring Test (MANDATORY -- NOT deferrable)

The umbrella reaches no code itself. Each child's primary wiring test is listed here for one-place visibility:

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `Server.ReloadConfig` adding a root whose plugin is not running yet | -> | Child 1: `config.VerifyPluginConfigMapTransition` before `autoLoadForReload` | `TestReloadConfigVerifiesNewlyAddedRootBeforeAutoLoad` |
| `registry.All()` walk over `ConfigRoots` participants | -> | Child 1: registered conformance fixture runner | `TestEveryParticipantRoundTripsApplyRollbackRestart` |
| `RegisterOwnedAddresses` while `applyIsOwed()` is true | -> | Child 2: `reconcileOnRegistryChange` guard | `TestRegistryReconcileWaitsForOwedApply` |
| Fresh `firewallnft.newBackend()` `Apply` after previous process left a marked table | -> | Child 3: `NFTA_TABLE_USERDATA` reclaim | `TestRestartReclaimsMarkedTable` |
| `EditorManager.ApplyFormEdits` with refused 3rd field | -> | Child 4: `cli.Editor.ApplyFieldEdits` clone-and-swap | `TestFormSaveRefusedFieldLeavesTreeUnchanged` |
| IS-IS `engine.reconcile` changing `SystemID` | -> | Child 5: `purgeIdentity` -> `Originator.PurgeSystem` | `TestSystemIDReloadPurgesOldLSPsBeforeCircuitRebuild` |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Children 1 to 5 (`spec-single-owner-reload-reconcile-1` through `-5`) | Each child is implemented, verified by its named tests, and closed | None for the five child specs; the ten already-landed or superseded items listed in `## Row routing` remain retired at HEAD with their existing code and tests intact |
| AC-2 | All 17 rows in `## Row routing` | Each of the 7 open rows routed to Children 1 to 5 reads `fixed:` in `plan/journal/` at child closure, and each of the 8 already-fixed-at-HEAD rows whose journal entry still starts with `open:` reads `fixed:` with the HEAD evidence recorded above | The single third-party row (`policy-change-never-re-evaluates-what-it-admitted` 2026-10-22, GoBGP) remains unowned under `## Rows considered and not owned` because it is outside Ze; no open row from the 17 routed rows remains open at umbrella closure |
| AC-3 | Owner decision 2026-11-24 (AC-10) | `RefuseChangedDeclarations` refuses any reload that alters declared external plugins and reports the restart requirement (`test/reload/external-yang-reload-refused.ci`), and no code reintroduces hot-loading external plugins on reload | Reloads whose declared external plugin set is unchanged remain permitted and proceed through normal config transaction reload because external plugin declarations stay fixed while the daemon runs |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| per child (Children 1 to 5) | per child | AC-1 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| per child | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| per child (Children 1, 2, 3, 4, 5) | per child | AC-1 | |

### Interop Tests (Scope: config)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `isis-system-id-change-frr` (Child 5) | `test/interop/scenarios/isis-system-id-change-frr/` | FRR | Wire-level zero-lifetime IS-IS purge on `SystemID` reload | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- each child's own Phase 1 wiring test listed in `## Wiring Test`.
2. **Phase: Children 1 to 5** -- implement and close each child independently (no overlapping files across children).
3. **Phase: Journal cleanup and umbrella closure** -- flip the 8 already-fixed-at-HEAD journal rows from `open:` to `fixed:` with their HEAD evidence, verify AC-1 to AC-3, and close the umbrella.

## Files to Modify
- None. Children 1 to 5 carry every code and documentation edit.

## Files to Create
- None. Children 1 to 5 exist on disk under `plan/release/fixes/`.

## Owned journal rows

The 7 genuinely open rows moved to Children 1 to 5 (one owner each). The 2 rows that already read `fixed:` in `plan/journal/` (`two-owners-share-one-name` 2026-09-28 and `unwired-feature` 2026-10-11) need no owner. The 8 rows below are ALREADY FIXED AT HEAD (see `## Row routing` and `## Rows found already fixed`), and are retained here solely because their lines in `plan/journal/*.md` still begin with `open:` until flipped to `fixed:`:

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| rollback-forgets-partial-apply | 2026-10-11 | already fixed at HEAD (`reload_tree.go` `loadPeersFromConfig`); awaiting journal flip to `fixed:` |
| rollback-forgets-partial-apply | 2026-11-19 | already fixed at HEAD (`main_reload.go` `restoreCertificates`); awaiting journal flip to `fixed:` |
| policy-change-never-re-evaluates-what-it-admitted | 2026-10-08 | already fixed at HEAD (`redistribute/held.go` `withdrawRejectedHeld`); awaiting journal flip to `fixed:` |
| gate-excludes-part-of-its-population | 2026-08-18 | already fixed at HEAD (`RegisterTables` `ze_` prefix + `legacy_tables.go`); awaiting journal marker flip to `fixed:` |
| gate-excludes-part-of-its-population | 2026-08-22 | already fixed at HEAD (`flowspec-firewall/state.go` `ze_flowspec` + `legacy_tables.go`); awaiting journal marker flip to `fixed:` |
| unwired-feature | 2026-08-31 | already fixed at HEAD (`bmp/bmp.go` `applyReceiverConfig` on reload rail); awaiting journal marker flip to `fixed:` |
| announced-state-never-replayed | 2026-09-06 | already fixed at HEAD (`ae49f963d0` confirms `api-reload` advertises and withdraws `2.0.0.0/24`); awaiting journal flip to `fixed:` |
| unwired-feature | 2026-09-06 | already fixed at HEAD (`d064818a9b` `exabgp_case.go` + `cmd_exabgp_process.go` SIGHUP reload); awaiting journal flip to `fixed:` |

## Rows found already fixed

| Class file | Date | Surface (short) | Evidence |
|------------|------|-----------------|----------|
| rollback-forgets-partial-apply | 2026-09-23 | VRRP shared-sysctl restore | `internal/plugins/vrrp/dataplane_linux.go` retains failed restorations independently of group references, with regressions in `dataplane_failure_linux_test.go` |
| check-mode-mutates-the-tree | 2026-10-11 | bgp `createReloadFunc` applies dynamic groups during verify | AC-23 of `spec-bgp-live-receive-binding-and-bfd-keep-session` (`a7b42bdc54`): `createReloadFunc` (`internal/component/bgp/config/loader_create.go`) applies nothing; `TestTreeRouteLeavesDynamicGroupsUnchanged` |
| rollback-forgets-partial-apply | 2026-10-11 | `restoreReload` -> `ApplyConfigDiff(undo.before)` | `internal/component/bgp/reactor/reload_tree.go` (`loadPeersFromConfig`, `SetTreeReloadFunc`) and `reactor_api.go:1022` (`ApplyConfigDiff` parses the handed tree, never the staged file); `TestTreeRouteReadsTheBGPBlockOfTheConfigTree` |
| rollback-forgets-partial-apply | 2026-11-19 | `updateWebCertificate` and `updateLGCertificate` rollback | `cmd/ze/hub/main_reload.go:941-996, 1217-1266` (`runningListenerCertificates`, `restoreOn`, `restoreCertificates`); `TestRollbackReloadRestoresInstalledListenerCertificate` (`main_reload_pki_test.go:599`) |
| policy-change-never-re-evaluates-what-it-admitted | 2026-10-08 | redistribute `SetGlobal` never re-judges held routes | `internal/plugins/redistribute/config.go:146-193` (`reconcileConsumers`) and `held.go:597-616` (`withdrawRejectedHeld`); `TestRuleNarrowingWithdrawsHeldRoutes` (`config_test.go:240`), `test/plugin/redistribute-chain-narrow-reload.ci` |
| gate-excludes-part-of-its-population | 2026-08-18 | nft table ownership by `ze_` name prefix | `spec-fixit-flowspec-table-never-swept`: `RegisterTables` refuses bare names; `legacy_tables.go` sweeps legacy names once |
| gate-excludes-part-of-its-population | 2026-08-22 | flowspec-firewall table `flowspec` outside the prefix | `internal/plugins/flowspec-firewall/state.go:13` (`const tableName = "ze_flowspec"`) + `legacy_tables.go` |
| two-owners-share-one-name | 2026-09-28 | firewall `mergeSameNameTables` merges operator table into VRRP owner table | `internal/component/firewall/reservation.go` (`MustReserveTableNames`, `refuseReservedTableNames`); `TestOperatorTableRefusedOnAReservedName`, `test/parse/firewall-reserved-table-name-rejected.ci` |
| unwired-feature | 2026-10-11 | `autoLoadForNewConfigPaths` never starts an external program | Dropped and superseded 2026-11-24 by `RefuseChangedDeclarations` (`cmd/ze/hub/main_reload.go:83`, `test/reload/external-yang-reload-refused.ci`) |
| unwired-feature | 2026-08-31 | bmp `environment` reload never re-applies receiver | `internal/component/bgp/plugins/bmp/bmp.go:577-679` (`OnConfigVerify`/`OnConfigApply`/`OnConfigRollback` -> `applyReceiverConfig`); `test/reload/bmp-receiver-bind-conflict.ci` |
| announced-state-never-replayed | 2026-09-06 | `api-reload.1.conf` two static routes, one advertised | `ae49f963d0` confirms `api-reload` advertises and withdraws `2.0.0.0/24` alongside `3.0.0.0/24` |
| unwired-feature | 2026-09-06 | reload of a running `ze` daemon from the ExaBGP test runner | `d064818a9b`: `exabgp_case.go` parses `signal:`, `cmd_exabgp.go:1058-1101` migrates all configs into `reloads`, `cmd_exabgp_process.go:143-225` writes next config and signals `SIGHUP` |

## Rows considered and not owned

| Class file | Date | Surface (short) | Why not this root cause |
|------------|------|-----------------|-------------------------|
| policy-change-never-re-evaluates-what-it-admitted | 2026-10-22 | GoBGP `UpdateConfig` export policy | third-party checkout, not Ze code |

## Work Not Done

All remaining implementation work is carried by the five child specs:
- `plan/release/fixes/spec-single-owner-reload-reconcile-1-tx-conformance.md`
- `plan/release/fixes/spec-single-owner-reload-reconcile-2-iface-reconcile.md`
- `plan/release/fixes/spec-single-owner-reload-reconcile-3-firewall-nft-ownership.md`
- `plan/release/fixes/spec-single-owner-reload-reconcile-4-web-form-atomic-save.md`
- `plan/release/fixes/spec-single-owner-reload-reconcile-5-isis-identity-purge.md`

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A config change is a transaction of stream events (verify, apply, rollback) that each plugin answers for its own roots (`docs/architecture/config/transaction-protocol.md`)." | agrees | Umbrella edits no code and splits the remaining reload/reconcile fixes into five single-door child specs, each enforcing exact-or-reject verification and single-owner state transitions. |

## Design presentation

### What changes
- This umbrella retires 10 of its 17 original journal rows (already landed at HEAD or superseded by the 2026-11-24 owner decision dropping AC-10) and splits the remaining 7 open rows into 5 single-door child specs (`spec-single-owner-reload-reconcile-1` through `-5`), editing no code itself.

### Before and after
- **Before:** One monolithic spec spanned nine subsystem doors, mixed already-landed fixes (BGP compensation, hub TLS cert rollback, redistribute held-route reconcile, firewall table reservations, BMP receiver reload, ExaBGP reload runner, dropped AC-10) with genuinely open defects, and planned an annotation/single-spec rollout.
- **After:** The umbrella holds the owner decision, producer-verified retirements, and row routing; five narrow child specs each own one subsystem area with zero file overlap and exact producer-verified designs.

### Alternatives set aside
- **Keeping all remaining fixes in one multi-door spec:** Rejected under `ai/META-DECISIONS.md` ("a spec whose edits need many doors is split into narrower child specs") and the precedent of sibling `plan/release/fixes/spec-single-owner-lifecycle.md`.
- **Re-implementing already-landed fixes or keeping dropped AC-10:** Rejected after producer verification at HEAD and the 2026-11-24 owner decision (`RefuseChangedDeclarations`).

### Documents departed from
None.

### Choices for Thomas
none

## Review Gate

- [ ] Children 1 to 5 each passed their own Review Gate and closed
- [ ] All 17 routed journal rows read `fixed:` in `plan/journal/`

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Producer evidence re-read for every row
- [ ] No code snippets

### Goal Gates (MUST pass)
- [ ] AC-1 to AC-3 satisfied by closure of Children 1 to 5 and journal cleanup
- [ ] Wiring Test table complete
- [ ] Each child's targeted tests pass in the main checkout (umbrella runs no tests of its own; GitHub CI verifies pushed commits)

### TDD
- [ ] Per child (Children 1 to 5)

### Closure
- [ ] Owned rows re-pointed to child closures or flipped to `fixed:` with HEAD evidence

# Spec: plugin-reload-applies-or-refuses

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Depends | `plan/immediate/spec-bgp-open-session-rfc-defects.md` (D2, Q1 = b) for Phase 4 only: the capability-only plugins' explicit declaration lands after its capability extractor |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A configuration reload reaches about twenty plugins that answer `config-apply`
with OK and change nothing. The plugin SDK binds `callbackConfigApply` to a
default that accepts the apply when the plugin registered no `OnConfigApply`
handler (`pkg/plugin/sdk/sdk_callbacks.go`, `initCallbackDefaults`,
`configApplyCallback(nil)`). The plugin's `OnConfigure` runs at Stage 2 only, so
the operator's commit reports success while the plugin keeps its startup
configuration. Example: an edit of a prefix-list's entries reaches
`bgp-filter-prefix`, which answers OK and keeps the old lists.

Owner decision, 2026-10-14 ("yes"), two steps in this order:

| Step | What | Rule |
|------|------|------|
| (c) | Every reload-relevant plugin gets its own config verify, apply and rollback handlers, so a reload either takes effect or is refused at verify with a "needs restart" error naming the leaf. Template: `filter_irr` (`irrConfigTx`, `internal/component/bgp/plugins/filter_irr/config_tx.go`) | per-plugin semantics |
| (a) | Then the SDK refuses config for a participant that registered no apply handler, so a new plugin cannot silently ignore a reload again | `ai/rules/no-layering.md`: the silent default is deleted, not kept beside the refusal |

Source investigation (read-only, 2026-10-14):
`tmp/session/2026-10-12-0c3648c8-b9d7-45d8-b3f0-2473bceefb25/scratch/investigate-sdk-config-apply-noop.md`.
Its claims were re-read at the producers on 2026-10-14; the corrections are in
Current Behavior.

### Boundary with the BGP OPEN spec

`plan/immediate/spec-bgp-open-session-rfc-defects.md`, D2, owner Q1 = b
(2026-10-13): a reload that changes, adds or removes a plugin-declared
CAPABILITY is refused at config-verify by the SDK, through the capability
extractor each capability plugin registers (`Plugin.DeclareCapabilitiesFrom`,
final name set there). Its callers: `softver`, `hostname`, `role`, `llnh`, `gr`,
the exabgp bridge, and the external `internal/plugins/exabgp/main_sdk.go`.

| Concern | Owner |
|---------|-------|
| Any leaf whose value reaches a capability declaration (softver encoding, hostname FQDN, role value, llnh capability, GR/LLGR timers and families, bridge route-refresh and add-path) | BGP OPEN spec. This spec adds no verify check for these leaves and never re-declares a capability |
| Every other leaf those plugins read (role `strict`, role `export`, the role name-to-address map, the bridge's non-capability leaves) | This spec |
| The apply handler of a plugin whose only config is capability declarations (`gr`, `hostname`, `llnh`, `softver`, external exabgp) | This spec, Phase 4, and only after the OPEN spec's extractor has landed: before it, a no-op apply would name as handled a change nobody refuses |

The two verify refusals compose: the SDK runs the capability comparison first
(OPEN spec, "the SDK runs first"), then the plugin's own verify handler from
this spec. Neither spec weakens the other's refusal.

### Other specs and precedents folded in

| Source | Relation |
|--------|----------|
| `cdc0bba8a2` fix(bgp-rs): `queueSizeConfigTx` verify/apply/rollback for `worker-queue-size` | Done; rs reads no other config leaf (`server.go` OnConfigure registers `queueSizeTx.configure` only), so rs is outside the population |
| `filter_irr` (`config_tx.go`) | Done; the template |
| `plan/immediate/spec-bgp-update-propagation-rfc-defects.md` D5 (status design) | Plans a filter_community `OnConfigVerify`/`OnConfigApply` that rebuilds its blackhole-agreement cache. This spec owns filter_community's whole transaction (definitions and per-peer filter configs); D5's cache becomes one more field of the same snapshot. Whichever lands second extends the first, never adds a second apply handler (`OnConfigApply` overwrites: one handler per plugin) |
| No other spec | `grep` of `plan/*.md`, `plan/immediate/`, `plan/pre-release/` for `OnConfigApply`, "ignores reload" and the plugin names found no spec covering this population |

### Journal rows this spec closes

| Row | Closed by |
|-----|-----------|
| `plan/journal/unwired-feature.md`, 2026-10-14, `runFilterCommunity` registers only `OnConfigure` | Phase 2 (filter_community) |
| `plan/journal/unwired-feature.md`, 2026-08-24, fixit-ipsec-peer-reload-ignored, residual "nothing yet detects the next plugin that declares `WantsConfig` and registers no apply handler" | Phase 5 (SDK refusal) |
| `plan/journal/comment-describes-superseded-behaviour.md`, 2026-10-14, `plugin-relationships.md` section 7 participation rows wrong | Phase 6 doc edit: this spec changes every row's reload answer, so the table is re-derived from the registrations in the same change (`ai/rules/documentation.md`) |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/transaction-protocol.md` - participation, verify/apply/rollback, rollback gate
  → Constraint: a participant is any process whose `WantsConfigRoots` (SDK `Registration.WantsConfig` joined with registry `ConfigReads`, `server/startup.go` `joinConfigReads`) names a changed root; there is no subtree granularity, so every plugin wanting `bgp` is a participant in EVERY bgp edit and every plugin wanting `service` in every service edit
  → Constraint: verify receives the FULL candidate content of each changed root; apply receives only diff sections (`rpc.ConfigApplyInput`); the plugin must therefore hold its parsed candidate from verify to apply, as `irrConfigTx.pending` does
  → Constraint: rollback reaches a plugin only after it accepted an apply that is not committed (`config_tx_gate.go`); an apply that returns an error must undo its own partial work first (`OnConfigApply` doc comment)
  → Decision: refusing at verify is preferred over refusing at apply: verify runs for the whole transaction before any apply, so nothing is applied and nothing rolls back
- [ ] `docs/plugin-development/handlers.md` (OnConfigApply section, the response table) - documents the no-op default as intended
  → Constraint: the row "No handler registered: `{"status":"ok"}` (graceful no-op)" and the OnConfigApply prose become false at Phase 5 and are rewritten in the same phase
- [ ] `docs/guide/config-reload.md` - operator reload page
  → Constraint: the row "Plugin config changed | Plugin reloaded" is false today; "What Requires Restart" gains each leaf this spec refuses at verify; restart command is `ze signal restart`
- [ ] `docs/architecture/plugin/plugin-relationships.md` section 7 - participation table, wrong per journal
  → Decision: re-derived from the registrations in Phase 6
- [ ] `ai/patterns/plugin.md` - plugin pattern
  → Constraint: says nothing about reload today; gains the rule that a plugin with `WantsConfig` registers an apply handler or the SDK refuses its reloads
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: every refusal states the problem in operator terms (plugin, leaf path), why it matters (the running plugin cannot take the change), and the next step on its own line (`ze signal restart`)
- [ ] `plan/immediate/spec-bgp-open-session-rfc-defects.md` D2 refusal design
  → Constraint: the capability extractor runs at config-verify composed BEFORE the plugin's own verify; this spec's verify handlers must not re-check capability leaves

**Key insights:**
- The SDK default apply is `configApplyCallback(nil)`: it opens the rollback gate and answers OK
- 28 SDK plugins register `OnConfigure` and no `OnConfigApply`; 4 more declare `WantsConfig` with neither
- Verify gets full sections, apply gets diffs: every handler parses in verify, installs in apply, restores in rollback
- Shared roots (`bgp`, `service`) make most participants receive edits that do not touch them: apply must be cheap and side-effect free when the plugin's own parsed config is unchanged

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` - `initCallbackDefaults` binds verify to `marshalStatusOK` and apply to `configApplyCallback(nil)`; `OnConfigApply` replaces it; `configApplyCallback` calls `fn` only when non-nil, then `p.configTx.applied()` and OK
  → Constraint: Phase 5 deletes the nil branch; `configApplyCallback` takes a non-nil handler only
- [ ] `pkg/plugin/sdk/config_tx_gate.go` and `config_tx_gate_test.go` - rollback gate (`applied`, `rollbackOwed`, `committed`)
  → Constraint: preserved unchanged
- [ ] `internal/component/plugin/server/reload.go` (participant selection, `reloadConfigSections`) - unchanged by this spec
- [ ] `internal/component/bgp/plugins/filter_irr/config_tx.go` - `configure` (Stage 2), `verify` (parse to `pending`, set `verified`), `apply` (refuse when not verified, swap, keep `previous`), `rollback` (restore `previous`)
  → Constraint: an apply with no verified candidate is refused, never answered OK; an accepted apply with no change drops the previous undo set
- [ ] Each population plugin's `OnConfigure` body (table below)

**Population, re-derived 2026-10-14** (`OnConfigure` without `OnConfigApply`, plus `WantsConfig` without either):

| # | Plugin | Config it reads at Stage 2 | How it stores it | Participant? |
|---|--------|----------------------------|------------------|--------------|
| 1 | `bgp/plugins/filter_aspath` | as-path-list definitions | `listsByName.Store` (atomic pointer) | yes, `bgp` |
| 2 | `bgp/plugins/filter_aspath_length` | as-path-length filter definitions | `defsByName.Store` | yes |
| 3 | `bgp/plugins/filter_community` | community definitions; bgp/group/peer filter configs (tag, strip, scrub, blackhole-propagation) | `configureCommunityFilter`: parse + validate, then `definitions`, `peerConfigs` under `mu` | yes |
| 4 | `bgp/plugins/filter_community_match` | community-list definitions | `listsByName.Store` | yes |
| 5 | `bgp/plugins/filter_family` | family filter instances | `instancesByName.Store`; has an `OnConfigVerify` | yes |
| 6 | `bgp/plugins/filter_modify` | attribute defaults (absent base MED, local-pref) and modifier definitions | `absentBase.Store` then `defsByName.Store`: two separate stores | yes |
| 7 | `bgp/plugins/filter_path_asn` | reject-asn lists and attachment counts | `listsByName.Store` then `attachmentsByList.Store` | yes |
| 8 | `bgp/plugins/filter_prefix` | prefix-list definitions | `listsByName.Store` | yes |
| 9 | `bgp/plugins/filter_remove_private_as` | remove-private-as definitions | `defsByName.Store` | yes |
| 10 | `bgp/plugins/healthcheck` | probe definitions | `mgr.validateConfig` then `mgr.applyConfig`, which stops removed or changed probes and starts new ones | yes |
| 11 | `bgp/plugins/watchdog` | per-peer route pools | `srv.peerPools = peerPools` under `srv.mu`; each `poolSet` also holds which entries are announced | yes |
| 12 | `bgp/plugins/rib` | `maximum-paths`, `relax-as-path`, RFC 7999 blackhole honoring rules, AS notation | `maximumPaths.Store` ONLY when > 0, `relaxASPath.Store`, `blackholeCfg.Store`, `asn.ConfigureFromBGP` (process global) | yes |
| 13 | `bgp/plugins/role` | per-peer role, `strict`, `export`; name-to-address map; role capability | `setFilterState`; `OnValidateOpen` closure reads the captured `peerConfigs`, `nameToIP` variables directly; `SetCapabilities` | yes |
| 14 | `bgp/plugins/gr` | GR (64) and LLGR (71) capability leaves only | `SetCapabilities`; `setEgressState` shares runtime-learned `peerLLGRCaps`, not config | yes |
| 15 | `bgp/plugins/hostname` | hostname capability only | `SetCapabilities` | yes |
| 16 | `bgp/plugins/llnh` | link-local next-hop capability only; own verify refusal | `SetCapabilities` | yes |
| 17 | `bgp/plugins/softver` | software-version capability only | `SetCapabilities` | yes |
| 18 | `bgp/plugins/route_refresh` | nothing (`OnConfigure` returns nil; the engine advertises route-refresh from `reactor/config.go`) | none | yes, `bgp` |
| 19 | `bgp/plugins/adj_rib_in` | nothing (applies startup claims) | none | no: no `WantsConfig` |
| 20 | `plugins/cos` | `class-of-service ieee-802.1p` profiles | `coreCos.Register` per profile, never unregisters; consumed by `iface/config.go` through `cos.Resolve` at iface apply time | yes, own root |
| 21 | `plugins/routingtable` | routing tables | `SetRegistry(New(tables))`; read by `static` verify through `routingtable.GetRegistry()` | yes, own root |
| 22 | `plugins/as112` | `service as112` | parse, `applyAddressRegistration` (register or unregister loopback owned addresses), build and store state, `reloadTotal` metric | yes, `service` |
| 23 | `plugins/geodns` | `service geodns` | parse, `buildState`, `storeState`; metric `reloadTotal` named as if reload re-ran it | yes, `service` |
| 24 | `plugins/dhcpserver` | `service dhcp...` | `startServer`: `stopListeners` (closes listeners, `h.leases.stop()` drops lease state) then new handlers and listeners | yes, `service` |
| 25 | `plugins/tftpserver` | `service tftp` | `startServer` (stop then start) | yes, `service` |
| 26 | `plugins/imageserver` | `service image-server` | `startServer` (stop then start) | yes, `service` |
| 27 | `plugins/exabgp/bridgeplugin` | `exabgp` root: scripts, families, route-refresh, add-path | `applyConfig` returns nil once `r.started`, so a re-run would itself be a no-op | yes, `exabgp` |
| 28 | `plugins/flowspec-firewall` | nothing from config | `initialized` guard | no: `sdk.Registration{}` |
| 29 | `plugins/connected` | empty `connected` container | none, no `OnConfigure` | yes, own root |
| 30 | `plugins/kernel` | empty `kernel` container | none, no `OnConfigure` | yes, own root |
| 31 | `plugins/exabgp/main_sdk.go` (external) | capability declarations (OPEN spec caller) | none, no `OnConfigure` | yes, `bgp` |
| 32 | `internal/test/plugins/fakeddos` | test fixture, own root | `OnConfigure` | yes |

Corrections to the investigation: it counted 29 and omitted rows 29 to 31
(they declare `WantsConfig` and register no `OnConfigure`, so the SDK default
answers their apply). `internal/component/config/schema/cli/main.go` carries a
`WantsConfig` field on a `pluginserver.Schema` registration, not an SDK plugin,
and is outside the population. The investigation's note that `rs` declared no
`WantsConfig` is fixed by `cdc0bba8a2`.

**Behavior to preserve:**
- Stage 2 `OnConfigure` delivery and its failed-removal compensation (`server/startup_removal.go`)
- Every existing `test/reload/*.ci` outcome other than the ones this spec turns from silent acceptance into effect or refusal
- `filter_irr` and `rs` transactions (unless Q5 = shared helper, where they migrate with identical behavior)
- The rollback gate semantics in `config_tx_gate.go`
- The OPEN spec's capability refusal and its wording

**Behavior to change:**
- Each population plugin's reload: takes effect, or is refused at verify naming the leaf
- The SDK's answer for a participant with no apply handler: refusal instead of OK

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator edits the config file and sends SIGHUP (`cmd/ze/hub/main_reload.go`), or commits from the CLI editor
- Format at entry: config tree, diffed against the running tree

### Transformation Path
1. `server/reload.go` diffs roots, selects participants by `WantsConfigRoots`
2. Transaction orchestrator (`internal/component/config/transaction/orchestrator.go`) sends `config-verify` with full changed-root sections to every participant
3. SDK verify callback: OPEN spec capability comparison first, then the plugin's verify handler parses its subtree into a pending candidate, or refuses with a needs-restart error
4. If every participant verified, `config-apply` with diff sections; SDK apply callback calls the plugin's apply handler, which installs the pending candidate (atomic swap or reconcile) and keeps the previous for rollback
5. `config-committed` closes the gate; on failure `config-rollback` reaches plugins that applied, which restore the previous

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | `config-verify` / `config-apply` / `config-rollback` / `config-committed` RPCs, JSON sections (`rpc.ConfigVerifyInput`, `rpc.ConfigApplyInput`) | Yes, `sdk_callbacks.go` read 2026-10-14 |
| Plugin ↔ consumers of its state | atomic pointers read by filter RPC handlers; `coreCos` registry read by iface; `routingtable` registry read by static | Yes, call sites grepped 2026-10-14 |

### Integration Points
- `Plugin.OnConfigVerify`, `OnConfigApply`, `OnConfigRollback` - each plugin registers all three
- `rib.reselectAIGPRoutes` (`rib_aigp.go`) - the model for the reselect walk rib owes after a selection-rule change
- OPEN spec `DeclareCapabilitiesFrom` - composed before the plugin verify

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | every change goes through the existing transaction RPCs |
| No unintended coupling | Yes | no plugin imports another; cos/iface coupling is Q2 and goes through config roots |
| No duplicated functionality | Yes, under Q5 = helper | one transaction shape instead of ~25 copies of `irrConfigTx` |
| Zero-copy preserved where applicable | N-A | config path, not wire path |
| Registration over hardcoding, outbound | Yes | each plugin registers its handlers in its own `run*` function |
| Registration over hardcoding, inbound | Yes | the SDK refusal reads only "did this plugin register an apply handler"; no list of plugin names is added anywhere. Searched: `sdk_callbacks.go` defaults, `reload.go` participant selection |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | Verify always precedes apply in the same transaction for the same participant | `transaction-protocol.md`; `irrConfigTx.apply` refuses an unverified apply and `filter_irr` reload tests pass | handlers install stale pending state | `TestConfigTxApplyWithoutVerifyIsRefused` per plugin (or once for the helper) | unvalidated |
| A-2 | A verify that carries no section for the plugin's root means "unchanged" | `irrConfigTx.verify` comment | a removal could read as no change | unit test: verify with the root absent, then apply, leaves state; a `.ci` that deletes the whole subtree | unvalidated |
| A-3 | Filter RPC handlers read their state through one atomic load per call, so an apply swap is seen whole | `filter_*` use `atomic.Pointer` `Store`/`Load` | a filter call mid-swap sees mixed state | `filter_modify` and `filter_path_asn` move to one snapshot (R-3); grep each handler's loads | unvalidated |
| A-4 | `route_refresh`, `connected`, `kernel` declare `WantsConfig` only so they load when their root exists, and read no leaf | their `OnConfigure` is empty or absent; YANG containers are empty | dropping `WantsConfig` would change autoload | read `autoLoadForNewConfigPaths` before choosing between removing `WantsConfig` and the explicit declaration | unvalidated |
| A-5 | The OPEN spec's extractor lands before Phase 4 | its Status is design, Q1 answered | Phase 4 waits; Phases 1 to 3 and 5 do not depend on it except rows 14 to 17 and 31 | check `git log` for `DeclareCapabilitiesFrom` before Phase 4 | unvalidated |
| A-6 | `ze signal restart` is the restart command an operator uses | six guide pages name it | the refusal's next step is wrong | grep `cmd/ze/signal` for the verb at implementation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Phase 5 refuses every bgp edit on a config loading any plugin not yet migrated, turning many `test/reload/*.ci` red | `./le test functional reload` red after Phase 5 | Phase 5 runs last; a pre-Phase-5 check lists every SDK plugin that declares `WantsConfig` and registers no apply handler, and it must be empty |
| R-2 | Shared roots: a `service` edit restarts dhcp/tftp/image listeners, or a `bgp` peer edit rebuilds every filter, when the plugin's own subtree did not change | log line "applied" on an unrelated edit; dropped DHCP leases | apply compares the parsed candidate with the current one and does nothing when equal (and drops the previous undo set, per `OnConfigApply` contract) |
| R-3 | `filter_modify` (two stores) and `filter_path_asn` (lists then counts) tear during a swap | a filter call reading new defs with old base | each moves its fields into one snapshot struct behind one atomic pointer |
| R-4 | role's `OnValidateOpen` closure reads reassigned captured variables: re-running would race | `-race` in role reload unit test | role state moves into one atomic snapshot read by both the filters and `OnValidateOpen` |
| R-5 | rib changes selection rules without reselecting stored routes, so old prefixes keep the old multipath set or blackhole stamping | `show` of an untouched prefix after a `maximum-paths` edit | apply runs a reselect walk over stored routes when `maximum-paths`, `relax-as-path` or blackhole rules changed, modeled on `reselectAIGPRoutes`; the walk re-publishes when the route type changes even if the winner did not |
| R-6 | rib `maximum-paths` removal keeps the old value (stored only when > 0) | removal `.ci` | absent leaf stores the YANG default 1 (`ze-bgp-conf.yang`) |
| R-7 | watchdog swap loses announce state: an entry removed from a pool stays announced, or a kept entry is announced twice | `.ci` that removes an announced entry | apply merges: unchanged entries keep state, removed announced entries are withdrawn through the existing withdraw path |
| R-8 | healthcheck: a probe stopped by reconcile leaves its route announced | `.ci` that removes a probe whose route is up | verify that `runProbe` cancel withdraws; if not, apply withdraws before deleting |
| R-9 | routingtable: `static` verifies against `routingtable.GetRegistry()` at its own verify, before routingtable applies, so one transaction that adds a table and a route in it is refused | `.ci` adding both in one commit | record and test the behavior; if refused, static's verify must read the candidate tables (new journal row if out of reach) |
| R-10 | cos: a profile edit changes the registry but interfaces already programmed keep old maps | `cos-*` `.ci` after reload | Q2 |
| R-11 | dhcpserver restart drops every lease on any dhcp change | lease table empty after reload | Q3 |
| R-12 | The SDK refusal fires for test fixtures (`fakeddos`) and external Go plugins built on the SDK | red plugin tests | fakeddos gets a handler; the SDK change is noted in `docs/plugin-development/handlers.md` as a breaking change for external authors |
| R-13 | Apply failure midway leaves partial state (healthcheck, dhcp) | apply returns error | handler undoes its own partial work before returning, per the `OnConfigApply` contract |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A filter, RIB rule or service runs config the operator replaced; or a reload is refused that should apply; or (Phase 5) every bgp reload is refused |
| How is it reverted? | Per-plugin commits revert independently; Phase 5 is one commit |
| Who else touches this path? | BGP OPEN spec (capability plugins, SDK verify), update-propagation spec D5 (filter_community), anyone editing `pkg/plugin/sdk/sdk_callbacks.go` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SIGHUP after editing a prefix-list's entries | → | filter_prefix apply swaps `listsByName` | `test/reload/bgp-filter-prefix-definition-reload.ci` |
| SIGHUP after editing a community definition and a peer's tag | → | filter_community apply | `test/reload/bgp-filter-community-reload.ci` |
| SIGHUP after `maximum-paths` 1 to 2 | → | rib apply + reselect walk | `test/reload/bgp-rib-maximum-paths-reload.ci` |
| SIGHUP after removing a watchdog pool entry that is announced | → | watchdog apply merge + withdraw | `test/reload/bgp-watchdog-pool-reload.ci` |
| SIGHUP after a dhcp subnet range edit | → | dhcpserver apply restart | `test/reload/dhcpserver-range-reload.ci` |
| SIGHUP after an exabgp bridge script edit | → | bridge verify refusal | `test/reload/exabgp-bridge-run-reload-refused.ci` |
| SIGHUP to a plugin with `WantsConfig` and no apply handler | → | SDK verify refusal | `test/reload/plugin-without-apply-refused.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | For each live-apply plugin (rows 1 to 13, 20 to 26): a reload that changes one of its leaves | The commit succeeds and the plugin's behavior after the reload reflects the new value (route accepted/rejected, attribute set, probe running, listener range) |
| AC-2 | Same plugins: a reload that removes the plugin's whole subtree or a leaf | The plugin returns to the "unconfigured" or YANG-default behavior (rib `maximum-paths` back to 1) |
| AC-3 | Same plugins: a reload whose candidate the plugin's parser refuses | The commit is refused at verify with the plugin's parse error, and the plugin keeps its running config |
| AC-4 | Same plugins: a transaction another participant fails after this plugin applied | Rollback restores this plugin's previous behavior |
| AC-5 | Shared root: a reload that changes `bgp` or `service` outside the plugin's subtree | The plugin's state is unchanged and it performs no restart (no listener restart, no lease loss, no probe restart) |
| AC-6 | rib: `maximum-paths` or `relax-as-path` or a blackhole honoring rule changed | Already-stored prefixes are reselected under the new rule without waiting for a new UPDATE |
| AC-7 | role: `strict` or `export` changed | Egress filtering uses the new export set at once; `strict` applies to the next OPEN; a role value change is refused by the OPEN spec, not here |
| AC-8 | exabgp bridge: any change under `exabgp` while the bridge runs, other than a capability leaf | Verify refuses with a message naming the leaf path, saying the bridge cannot change it while running, and the next step `ze signal restart` on its own line |
| AC-9 | Each leaf decided "refuse" in the decision table | The refusal follows AC-8's format and the daemon keeps its running config |
| AC-10 | Phase 5: a participant SDK plugin that registered no apply handler and no explicit "applies nothing" declaration | The SDK refuses at config-verify, naming the plugin and the changed root, with the next step `ze signal restart`; no participant applies |
| AC-11 | Phase 5: plugins that read no config (rows 18, 29, 30) or only capabilities (rows 14 to 17, 31) | Reloads that touch their root are accepted; their explicit declaration is the reason, and a debug log line names it |
| AC-12 | Phase 5: the SDK default apply | No code path answers `config-apply` OK without calling a registered handler |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Edits a prefix-list and reloads; the peer's route is now rejected | SIGHUP → reload diff → verify filter_prefix → apply → filter RPC reads new lists | `test/reload/bgp-filter-prefix-definition-reload.ci` |
| 2 | Changes a community tag on a peer and reloads | same path, filter_community | `test/reload/bgp-filter-community-reload.ci` |
| 3 | Raises `maximum-paths` and reloads | rib apply + reselect | `test/reload/bgp-rib-maximum-paths-reload.ci` |
| 4 | Edits an exabgp bridge script and reloads | verify refusal | `test/reload/exabgp-bridge-run-reload-refused.ci` |
| 5 | Edits an unrelated peer while dhcp runs | dhcp verify sees equal config, apply no-op | `test/reload/dhcpserver-unrelated-service-edit-keeps-leases.ci` |

## Per-plugin decision table

Every leaf of rows 1 to 13 and 20 to 26 is applied live; refusals are listed explicitly.

| # | Plugin | Decision | How |
|---|--------|----------|-----|
| 1, 2, 4, 8, 9 | filter_aspath, filter_aspath_length, filter_community_match, filter_prefix, filter_remove_private_as | apply live, every leaf | verify parses into pending; apply stores the pointer and keeps the previous; rollback stores the previous back |
| 3 | filter_community | apply live, every leaf | split `configureCommunityFilter` into parse+validate (verify) and install under `mu` (apply); D5 cache joins the same snapshot |
| 5 | filter_family | apply live | its existing verify handler becomes the parsing verify |
| 6 | filter_modify | apply live | one snapshot holding absent base and defs (R-3) |
| 7 | filter_path_asn | apply live | one snapshot holding lists and attachment counts (R-3) |
| 10 | healthcheck | apply live | verify runs `parseConfig` + `validateConfig`; apply calls the existing reconcile `applyConfig`; rollback reconciles to the previous probe set (R-8) |
| 11 | watchdog | apply live | merge preserving announce state; removed announced entries withdrawn (R-7) |
| 12 | rib | apply live: `maximum-paths`, `relax-as-path`, blackhole rules, AS notation | absent `maximum-paths` stores 1 (R-6); reselect walk on a selection-rule change (R-5); AS notation through `asn.ConfigureFromBGP` with the previous kept for rollback |
| 13 | role | apply live: `strict`, `export`, name map. Role value: OPEN spec | one atomic snapshot for filters and `OnValidateOpen` (R-4); `strict` takes effect at the next OPEN |
| 14 to 17 | gr, hostname, llnh, softver | capability-only: explicit "applies nothing beyond its capability declaration" | Phase 4, after the OPEN spec (A-5); llnh keeps its own verify refusal |
| 18 | route_refresh | reads no config | explicit declaration, or drop `WantsConfig` per A-4 |
| 19, 28 | adj_rib_in, flowspec-firewall | not participants | no change; AC-10 test asserts they are never refused |
| 20 | cos | apply live per Q2 | registry gains a replace-all so deleted profiles disappear |
| 21 | routingtable | apply live | `SetRegistry` with the previous kept for rollback; R-9 tested |
| 22, 23 | as112, geodns | apply live | the existing configure path, run from apply only when the parsed candidate differs (R-2); `reloadTotal` now counts real reloads |
| 24 | dhcpserver | apply live per Q3 | restart only when the dhcp subtree changed (R-2) |
| 25, 26 | tftpserver, imageserver | apply live | restart only when their subtree changed; an in-flight transfer is cut on change, documented |
| 27 | exabgp bridgeplugin | refuse at verify every non-capability change under `exabgp` while running (Q4) | needs-restart error naming the leaf path |
| 29, 30 | connected, kernel | read no config | explicit declaration, or drop `WantsConfig` per A-4 |
| 31 | exabgp external `main_sdk.go` | capability-only | Phase 4 |
| 32 | fakeddos (test fixture) | apply live | small handler so plugin tests keep working after Phase 5 |

## Open owner questions (each "which way")

| Q | Which way | Recommendation |
|---|-----------|----------------|
| Q1 | Where does the SDK refuse a participant with no apply handler: (a1) at config-verify, (a2) at config-apply as the decision words it, (a3) at `Run` startup when `WantsConfig` is set and no handler is registered? | (a1): nothing applies, nothing rolls back; (a3) would also be sound but refuses a plugin that is never a participant in practice |
| Q2 | cos profile edit with interfaces already using it: (b1) iface reads `class-of-service` (`ConfigReads`) and re-resolves on a cos change, cos ordered before iface; (b2) cos verify refuses a change to a profile an interface references ("needs restart"); (b3) apply only, interfaces pick it up at their next edit? | (b1): the edit takes effect everywhere; (b3) is silently wrong and rejected |
| Q3 | dhcpserver lease state on a dhcp change: (d1) keep the lease table for subnets whose definition is unchanged, restart the rest; (d2) restart all and drop leases, documented; (d3) refuse at verify? | (d1) |
| Q4 | exabgp bridge non-capability edits: (e1) refuse at verify ("needs restart"); (e2) restart the script fleet on apply? | (e1): the bridge has no live-reconfigure path today and the OPEN spec already refuses its capability half |
| Q5 | Transaction shape: (f1) one SDK helper (parse function, install function, previous kept by the SDK) used by every plugin, with `filter_irr` and `rs` migrated to it; (f2) a hand copy of `irrConfigTx` in each of ~25 plugins? | (f1): the fact "parse in verify, swap in apply, restore in rollback" is declared once (`ai/rules/principles.md`); each plugin still owns its parse and install |
| Q6 | Plugins that read no config (route_refresh, connected, kernel): (g1) drop `WantsConfig` if autoload does not need it; (g2) keep it and register the explicit "applies nothing" declaration? | (g1) where A-4 holds, else (g2) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConfigApplyWithoutHandlerIsRefused` | `pkg/plugin/sdk/sdk_callbacks_test.go` | AC-10, AC-12: a plugin with no apply handler is refused (at the phase Q1 picks) and the refusal names the plugin and root | |
| `TestConfigApplyNothingDeclarationAccepts` | `pkg/plugin/sdk/sdk_callbacks_test.go` | AC-11 | |
| `TestConfigTxApplyWithoutVerifyIsRefused` | helper test (Q5 f1) or each plugin's `config_tx_test.go` | A-1 | |
| `TestConfigTx{Verify,Apply,Rollback}` per plugin | `<plugin>/config_tx_test.go` | AC-1 to AC-4 at the plugin's state (the stored pointer, probe set, pools) | |
| `TestConfigTxUnchangedSubtreeIsNoop` per shared-root plugin | same | AC-5 | |
| `TestRIBMaximumPathsRemovalRestoresDefault`, `TestRIBReselectOnRuleChange` | `bgp/plugins/rib/config_tx_test.go` | R-5, R-6, AC-6 | |
| `TestRoleSnapshotSwapRace` (`-race`) | `bgp/plugins/role/config_tx_test.go` | R-4 | |
| `TestWatchdogApplyKeepsAnnounceState`, `TestWatchdogApplyWithdrawsRemoved` | `bgp/plugins/watchdog/config_tx_test.go` | R-7 | |
| `TestBridgeVerifyRefusesRunningChange` | `plugins/exabgp/bridgeplugin/config_tx_test.go` | AC-8 message text | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| rib `maximum-paths` on reload | 1-256 | 1 and 256 | 0 (refused at verify) | 257 (refused at verify) |

### Functional Tests
Each `.ci` rewrites the config, sends SIGHUP, and asserts the effect (or the refusal text) on the wire or in `show` output; each records a red run with the handler removed (`ai/rules/interop-and-goal-validation.md` discrimination).

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-filter-prefix-definition-reload` | `test/reload/` | prefix-list entry edit flips accept to reject | |
| `bgp-filter-aspath-reload`, `bgp-filter-aspath-length-reload`, `bgp-filter-community-match-reload`, `bgp-filter-remove-private-as-reload`, `bgp-filter-path-asn-reload`, `bgp-filter-family-reload`, `bgp-filter-modify-reload` | `test/reload/` | definition edit takes effect on the next UPDATE | |
| `bgp-filter-community-reload` | `test/reload/` | tag edit reaches the egress UPDATE | |
| `bgp-healthcheck-probe-reload` | `test/reload/` | probe removed: its route withdrawn | |
| `bgp-watchdog-pool-reload` | `test/reload/` | pool entry removed: withdrawn | |
| `bgp-rib-maximum-paths-reload` | `test/reload/` | multipath set changes for a stored prefix | |
| `bgp-role-export-reload` | `test/reload/` | export edit changes egress | |
| `cos-profile-reload`, `routing-table-reload`, `as112-reload`, `geodns-reload`, `dhcpserver-range-reload`, `dhcpserver-unrelated-service-edit-keeps-leases`, `tftpserver-reload`, `imageserver-reload` | `test/reload/` | edit takes effect; unrelated edit does nothing | |
| `exabgp-bridge-run-reload-refused` | `test/reload/` | AC-8 refusal text | |
| `plugin-without-apply-refused` | `test/reload/` | AC-10 refusal, via a fixture plugin with no handler | |
| `reload-plugin-only-no-change` (existing) | `test/reload/` | stays green after Phase 5 | |

### Interop Tests (Scope: protocol)
N-A: no wire format or protocol behavior changes; the effects are exercised by the `.ci` peers above.

## Files to Modify
- `pkg/plugin/sdk/sdk_callbacks.go` - Phase 5: delete the nil-handler apply; refusal per Q1; explicit "applies nothing" declaration; Q5 helper if chosen
- `internal/component/bgp/plugins/{filter_aspath,filter_aspath_length,filter_community,filter_community_match,filter_family,filter_modify,filter_path_asn,filter_prefix,filter_remove_private_as,healthcheck,watchdog,rib,role,gr,hostname,llnh,softver,route_refresh}/*.go` - handler registration and parse/install split
- `internal/component/bgp/plugins/filter_irr/config_tx.go`, `internal/component/bgp/plugins/rs/server.go` - only under Q5 f1
- `internal/plugins/{cos,routingtable,as112,geodns,dhcpserver,tftpserver,imageserver,exabgp/bridgeplugin,connected,kernel}/*.go`, `internal/plugins/exabgp/main_sdk.go`
- `internal/core/cos/cos.go` - replace-all for profiles
- `internal/component/iface/` - only under Q2 b1 (`ConfigReads` of `class-of-service`)
- `internal/test/plugins/fakeddos/register.go`
- Docs listed in the checklist below

## Files to Create
- `<plugin>/config_tx.go` and `config_tx_test.go` per plugin (or one SDK helper file and test under Q5 f1)
- `test/reload/*.ci` listed above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new leaf |
| YANG validation constraints | No | existing leaves unchanged |
| YANG custom validators | No | refusals live in plugin verify |
| CLI commands/flags | No | none added |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | N-A | no new leaf |
| Functional test for new RPC/API | N-A | no new RPC; reload `.ci` listed |
| Pipe completeness | N-A | no output |
| Env var registration | No | none |
| Doctor check for runtime dependencies | N-A | no new runtime dependency |
| Prometheus counters/metrics | No | as112/geodns `reloadTotal` start counting real reloads; no new metric |
| BGP family surface | N-A | no family, capability or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature or scope change? | No | reload fidelity is a defect fix |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | RPCs unchanged |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` reload behavior per plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/config-reload.md`: correct "Plugin config changed" row, add each refused leaf to "What Requires Restart" |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/plugin-development/handlers.md` (OnConfigApply, response table), `docs/architecture/config/transaction-protocol.md` (no-handler answer), `ai/patterns/plugin.md`, `ai/rules/plugins.md` |
| 9 | RFC behavior changed? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/plugin/plugin-relationships.md` section 7 re-derived |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | No | |
| 15 | Registered plugin inventory changed? | No | |
| 16 | Changed source files referenced by doc anchors? | Yes | run `./le spec citation anchors spec plan/immediate/spec-plugin-reload-applies-or-refuses.md` per phase |
| 17 | Existing docs show examples for this area? | Yes | `docs/plugin-development/handlers.md` OnConfigApply example |

Design documents declared by the `// Design:` headers of files this spec changes:

| Document | Affected? | Edit |
|----------|-----------|------|
| `docs/architecture/api/process-protocol.md` | Yes, Phase 5 | the `config-apply` callback row (line ~757) and the config-apply section gain the answer for a plugin with no handler (refused, per Q1) |
| `docs/architecture/bgp/filter-irr.md` | Only under Q5 f1 | names the SDK helper as the home of the transaction shape; behavior unchanged |
| `docs/architecture/traffic/cos-plugin.md` | Yes, Phase 3 | states reload behavior of profiles per the Q2 answer; it says nothing about reload today |

Documents that only `<!-- source: -->` mention a changed file (advisory, from `./le spec citation anchors`, 2026-10-14):

| Document | Affected? | Reason |
|----------|-----------|--------|
| `docs/plugin-development/protocol.md` | Yes, Phase 5 | "Config reload errors" says a missing handler answers `{"status":"ok"}` (graceful no-op); rewritten to the refusal |
| `docs/plugin-development/README.md` | Yes, Phase 5 | the callback table row for `OnConfigApply` gains "required for a plugin with `WantsConfig`" |
| `docs/plugin-development/testing.md` | Yes, Phase 5 | the reload-testing section names the required handler and the refusal test |
| `docs/architecture/plugin/plugin-system.md` | Checked at Phase 5 | mentions the callbacks; edited only if it states the default |
| `docs/architecture/api/ipc_protocol.md`, `docs/architecture/api/commands.md`, `docs/plugin-development/commands.md`, `docs/plugin-development/schema.md` | No | cite `sdk_callbacks.go` for other callbacks (commands, schema, events) |
| `docs/architecture/decisions/001-pull-model-metrics.md`, `docs/architecture/firewall/firewall-domain-group.md`, `docs/architecture/ike/ipsec-7-ikev2-engine.md` | No | cite `sdk_callbacks.go` for metrics, firewall and IKE handlers this spec does not change |
| `docs/architecture/traffic/cos-dynamic.md` | Checked at Phase 3 | cites `internal/core/cos/cos.go`; edited if the replace-all changes what it states about registration |
| `docs/guide/irr-filtering.md` | Only under Q5 f1 | cites `filter_irr/config_tx.go`; reload behavior unchanged |

## Implementation Steps

1. **Phase 1: Wiring** - the SDK helper (Q5) or the first hand copy, plus `bgp-filter-prefix-definition-reload.ci` red
2. **Phase 2: replace-style bgp plugins** - rows 1 to 9 (filter_community with the D5 coordination), then 10 and 11
3. **Phase 3: rib, role, system plugins** - rows 12, 13, 20 to 27, 32, with Q2, Q3, Q4 answers
4. **Phase 4: capability-only and no-config plugins** - rows 14 to 18, 29 to 31; capability rows after A-5 holds
5. **Phase 5: SDK refusal** - delete the nil default; refusal per Q1; pre-check that no participant SDK plugin lacks a handler (R-1)
6. **Phase 6: docs** - each phase edits its pages as it lands; Phase 6 is only the plugin-relationships table re-derivation and the journal row closures

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every population row has a handler or a named reason it is not a participant |
| Correctness | apply is a no-op when the parsed candidate equals the current one; rollback restores exactly the previous |
| Boundary | no verify handler re-checks a capability leaf; no apply re-declares a capability |
| Errors | every refusal names the leaf path and ends with `ze signal restart` on its own line |
| Rule: no-layering | after Phase 5, `configApplyCallback` has no nil branch and nothing answers apply OK without a handler |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No SDK plugin with `WantsConfig` lacks an apply handler or declaration | grep loop over `OnConfigure`/`WantsConfig`/`OnConfigApply` per package, empty |
| Nil-handler default gone | `configApplyCallback` read |
| Reload `.ci` set green, each with a recorded red | `./le test functional reload` (main thread) |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-open | an apply with no verified candidate is refused, never OK |
| Error leakage | refusal text names config paths only |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The no-op default hid every defect behind it: a guard nobody runs gives no wrong answer to notice (journal, IKE row 2026-08-24)

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Per-plugin handlers first, SDK refusal last | SDK default re-runs `OnConfigure` (investigation option b) | b is silently wrong for capability plugins, rib removal, role (race) and the bridge; owner chose c then a |
| Refuse at verify | refuse at apply | nothing applied, no rollback; Q1 confirms |
| No-op apply when own subtree unchanged | apply always | shared roots would restart services on unrelated edits |

## Known Limitations
- Capability leaves are the BGP OPEN spec's; this spec does not make them reloadable

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A with reason above)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/immediate/spec-plugin-reload-applies-or-refuses.md` only, in the same `./le commit create` script

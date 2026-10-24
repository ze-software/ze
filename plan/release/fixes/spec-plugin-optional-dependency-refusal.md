# Spec: plugin-optional-dependency-refusal

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P0 |
| Release reason | Accepted configuration can silently omit required plugin dependencies or start with missing hard edges; refusal must cover all existing lifecycle entries. |
| Scope | plugin |
| Depends | - |
| Phase | 8/8 |
| Handoff | - |
| Updated | 2026-10-24 |

## Release placement

Schema nodes that wrongly survive builds without BFD/LDP are owned by plan/release/fixes/spec-yang-feature-opt-in-missing-build-refusal.md. This spec retains runtime dependency refusal when configuration is otherwise accepted; the two scopes are complementary, not aliases.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner ruling 1 of `plan/handover/08-plugin-dependencies-and-subsystems.md`: a
plugin dependency may be soft. A build without the provider still runs the
owner, and refuses only the config that needs the provider, with an error
naming the build tag that would carry it. Example: a build without `ze_bgp`
runs flow export and refuses `enrichment bgp`.

Agreed scope (2026-10-21, SCOPE gate):

| Part | What |
|------|------|
| a | Extend the existing optional dependency edge (no new edge kind) with a declaration of which config needs the provider; a config that needs an absent provider is refused at verify/commit, naming the build tag |
| b | A generated, always-compiled table of gated plugin name to build tag, because a binary today cannot name a plugin it was built without |
| c | Flow export (`enrichment bgp`, which today declares no edge and silently receives nothing) and traffic (optional `vpp`) as the first users |
| d | A check that finds undeclared cross-gate couplings like flow export's |
| e | (owner, 2026-10-21, after WRITE) The hard-edge sibling: a HARD dependency absent from the build is refused on every load path, naming the plugin, the dependency and its build tag. Today two boot auto-load paths in `plugin/server/startup_autoload.go` log `ErrMissingDependency` at WARN and start the plugin without it (`resolved = needed`); the reload path and an explicit `plugin {}` block already fail closed, but their message names no tag |

Scope widened at the RESEARCH gate (2026-10-21, owner): every coupling the
part-d check finds is declared in this spec, so the check lands green. Found
so far: interface and firewall `backend vpp` (same shape as traffic), DDoS
FlowSpec mitigation without BGP, BGP-LS export without IS-IS/OSPF/RSVP-TE,
DDoS and anomaly detection without a flow source (flow export or traffic
usage). An optional edge's config condition decides both whether the provider
is pulled and whether the config is refused.

Scope finding at the gate: of the four declared optional edges, only traffic
to `vpp` crosses a build-tag boundary. Route server to adj-RIB-in and graceful
restart to route server share `ze_bgp`; static to interface targets an ungated
component. `ResolveDependencies` skips an unregistered optional dependency with
no log line and no error (`internal/component/plugin/registry/registry.go`).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/plugin-system.md` - registration fields, startup phases
  → Constraint: a plugin's registration is its only declaration; the core never enumerates plugins, so the config-to-provider fact lives on the owner's `Registration`
- [ ] `docs/architecture/plugin/feature-gates.md` - how a feature is compiled out
  → Constraint: `feature-gates.txt` is the only declaration of a gate; `internal/le/repo/featuretags` and `feature-tags` are its only readers. The name-to-tag table is generated from them, never hand-written
  → Constraint: the generator gates `<pkg>` and `<pkg>/yang` together, so a config root owned by a compiled-out plugin already fails at parse ("unknown top-level keyword"). The defect this spec fixes is only a leaf inside a PRESENT plugin's YANG that needs an ABSENT provider
  → Constraint: the page's only invariant is import-level; it is silent on optional edges and on couplings over the event bus or a backend registry. This spec adds that section
- [ ] `docs/architecture/config/yang-config-design.md` "Layer 3" - plugin verifiers
  → Constraint: plugin verifiers do not run at daemon boot (`LoadConfig` never calls `VerifyPluginConfig`), so a refusal placed only in a verifier misses boot
- [ ] `docs/architecture/flowexport/flow-export-2-flow-records.md` - BGP enrichment
  → Constraint: enrichment subscribes to `ribevents.BestChange` through the typed handle; the producer is `bgp-rib` (ze_bgp). The page's "one-second ticker" was wrong (code: `enrichRebuildInterval`, five seconds); corrected 2026-10-21 in this spec's research
- [ ] `ai/rules/plugins.md`, `ai/patterns/registration.md`, `ai/rules/config.md`, `ai/rules/user-facing-errors.md`
  → Constraint: the refusal names what is wrong, why, and the fix: the build tag and the build command that adds it
  → Constraint: `ai/rules/plugins.md` has no "Feature-Gate Registration" section although `feature-gates.md` points to it, and `feature-gates.txt` names `ai/rules/feature-gate-registration.md`, which does not exist. Both pointers are repaired in this spec's doc phase

**Key insights:**
- An optional edge LOADS its provider whenever the provider is compiled in (`ResolveDependencies`). A bare edge from flow export to `bgp-rib` would start the RIB in every BGP build that runs flow export, even with enrichment off. The config condition must govern the pull, not only the refusal.
- Both first users condition on a leaf VALUE, not on a path's presence: `flow-export enrichment bgp` is a boolean with default false; `traffic control backend` is a free string whose value `vpp` needs the `vpp` component.
- The one door that covers boot, SIGHUP reload and editor-commit reload is `config.LoadConfig`, beside `refuseInvalidCustomSections`. Editor pre-commit, the API commit hook, `ze config validate` and doctor go through `VerifyPluginConfigMapTransition`. One check, called from both.
- No binary can name a plugin it was built without. le is built with every tag and sees every registration (`internal/le/repo/inventory` `Plugins`), so a generator joins registration name, package directory and `featuretags.Gates` into an always-compiled table. Nested gates yield compound tags (bgp-bmp: `ze_bgp && ze_bmp`).
- The process protocol's Stage 1 declaration carries `Dependencies` only, so an external plugin cannot declare an optional edge today.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/registry/registry.go` - `Registration.OptionalDependencies`; `Register` refuses empty and self edges over Dependencies, OptionalDependencies and StartAfter; `ResolveDependencies` pulls a registered optional dependency and skips an unregistered one silently; `detectCycles` and `TopologicalTiers` treat it as an ordering edge
  → Constraint: changing the field's type breaks every consumer, including the published website JSON (`internal/le/site`, `internal/le/repo/inventory`); a sibling field keyed by dependency name touches only `Register` validation, the new check, and optionally inventory/site
- [ ] `internal/component/plugin/server/startup.go` `selectedPluginGraph`, `startup_autoload.go` `collectOrphanCandidates`, `pluginDependsOn` - startup graph and orphan stop treat the optional edge like a hard one
- [ ] `internal/component/config/loader.go` `LoadConfig` - order: `parseTreeWithYANG`, `refuseInvalidCustomSections`, `ExpandDependencies` (calls `ResolveDependencies`)
- [ ] `internal/component/config/plugin_verify.go` `VerifyPluginConfigMapTransition` - loops over compiled-in registrations with a verifier and ConfigRoots
- [ ] `internal/component/config/backend_gate.go` `ValidateBackendFeatures` - the `ze:backend` YANG extension refuses a node the active backend does not support; keyed on backend value, never on whether a plugin is present
- [ ] `internal/plugins/flowexport/register.go`, `enrichbgp.go`, `yang/ze-flowexport-conf.yang` - `Dependencies: interface` only; `enrichment bgp` (boolean, default false) starts `bgpEnrichBuilder`, which subscribes to `ribevents.BestChange`. Without ze_bgp the subscription succeeds and nothing is ever published; the only trace is the Info line `bgp-enrichment=true`
- [ ] `internal/component/traffic/register.go`, `backend.go` - `OptionalDependencies: vpp`; `traffic control backend` is a free string, default `tc`. Without ze_vpp, verify passes (`RunVerifier` returns nil for an unregistered backend) and configure/apply fails with `traffic: unknown backend "vpp" (registered: [tc])`, tag not named
- [ ] `internal/plugins/static/register.go`, `bgp/plugins/rs/register.go`, `bgp/plugins/gr/register.go` - the other three optional edges; provider and owner never sit in different builds, and no owner config needs the provider
  → Constraint: static's comment says a config with no `interface` stanza "leaves static unconstrained", but `ResolveDependencies` pulls `interface` whenever static loads. Comment or behavior is wrong; settled in DESIGN (A-3)

**Behavior to preserve:**
- Startup ordering of every existing optional edge when both ends are selected
- An optional provider that is compiled in and needed still loads automatically
- Route server, graceful restart and static keep working unchanged in every build
- Existing parse-level refusal of a compiled-out plugin's own config root

**Behavior to change:**
- A config that needs a compiled-out optional provider is refused before it is applied, at boot, reload, editor commit, API commit and `ze config validate`, with an error naming the build tag
- Flow export declares its soft edge to `bgp-rib`; traffic's existing edge gains its config condition
- A check reports a cross-gate coupling that carries no declared edge

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config text (file at boot, editor commit, API commit, SIGHUP reload, `ze config validate`)

### Transformation Path
1. Parse with YANG (`parseTreeWithYANG`); a compiled-out plugin's root fails here already
2. Custom section validation (`refuseInvalidCustomSections`); the new refusal sits here for the LoadConfig doors
3. Plugin verifiers (`VerifyPluginConfigMapTransition`); the new refusal is called here for the editor, API and validate doors
4. Dependency expansion (`ExpandDependencies`); an optional provider is pulled only when its config condition holds (design decision pending)
5. Stage 2 configure in each plugin

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ plugin registry | `registry.All()` read by the config component | Yes (`VerifyPluginConfigMapTransition`) |
| Build manifest ↔ binary | generated table, compiled in every build | No (new) |
| Engine ↔ external plugin | Stage 1 declaration carries `Dependencies` only | Yes (`pkg/plugin/rpc` `DeclareRegistrationInput`) |

### Integration Points
- `registry.Registration` - new sibling declaration beside `OptionalDependencies`
- `config.LoadConfig`, `config.VerifyPluginConfigMapTransition` - the two call sites of the one refusal
- `registry.ResolveDependencies` - conditional pull
- `./le repo generate` (`internal/le/repo/generate.go`) - generator for the name-to-tag table
- `internal/le/plugin/boundary` - nearest existing check for the coupling scan (per-file import alias resolution, plugin search roots)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | design pending |
| No unintended coupling (components stay isolated) | No | design pending |
| No duplicated functionality (extends existing, does not recreate) | No | design pending: `ze:backend` and `RegisteredProtocolValidator` are adjacent mechanisms |
| Zero-copy preserved where applicable (refs, not copies) | No | config path, not hot path |
| Registration over hardcoding, outbound | No | design pending |
| Registration over hardcoding, inbound | No | design pending |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `bgp-rib` is the only producer of `ribevents.BestChange` | research report; `bgp/plugins/rib/register.go` registers the namespace | flow export's edge names the wrong provider | grep every `BestChange.Emit` | confirmed (2026-10-24, phase 1): the only non-test `ribevents.BestChange.Emit` calls are `internal/component/bgp/plugins/rib/rib_bestchange.go` (two sites), registered as `bgp-rib`; `sysrib.go` emits `sysribevents.BestChange`, a different handle (`internal/component/sysrib/events/events.go`) |
| A-2 | le sees every registration because it is built with every gate tag | `le` launcher builds with `ze_le` plus every feature tag | generated table misses plugins | generator test compares table rows against `featuretags.Gates` packages | confirmed (phase 2): the `le` launcher builds with `ze_le` plus every `feature_tags` tag; `TestGatedPluginTableMatchesManifest` (`internal/le/plugin/gatedtable`) walks every manifest package whose source calls `registry.Register` and finds a table row from each; the generated table holds 77 gated plugins |
| A-3 | static's `interface` edge can be pulled unconditionally without harm, because interface is loaded in every real config | `static/register.go` comment contradicts `ResolveDependencies` | conditional pull changes static's startup | read interface autoload; functional test with static and no interface stanza | half confirmed (2026-10-24, phase 1): the comment is wrong. `iface/register.go` auto-loads `interface` only from its `interface` ConfigRoot, while `registry.ResolveDependencies` pulls a registered optional dependency whenever its owner loads, so static pulls `interface` with no `interface` stanza. The basis "interface is loaded in every real config" is not established by any always-load path. Closure (2026-10-24): broken in its basis, harmless in effect. Static declares no ConfigNeed, so its pull of `interface` is unchanged from HEAD (`TestOptionalEdgesWithoutNeedsPullAsBefore`, `internal/component/plugin/all/optional_edges_without_needs_test.go`); the wrong comment is corrected in `internal/plugins/static/register.go`. Mistake Log row |
| A-4 | No external plugin needs an optional edge in this spec | Stage 1 carries `Dependencies` only; no external plugin is a first user | the refusal misses externals | owner scope statement (SCOPE gate: a+b+c+d) | confirmed (closure): every ConfigNeed and optional edge in this spec is declared on an internal registration (`grep -rn 'ConfigNeeds:' internal`), and `pkg/plugin/rpc` `DeclareRegistrationInput` still carries `Dependencies` only, which Known Limitations records |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A bare optional edge from flow export to `bgp-rib` starts the RIB in every BGP build running flow export | `bgp-rib` in the process list with enrichment off | pull conditioned on the declared config value |
| R-2 | Refusal placed only in a verifier misses boot | boot accepts the config, apply fails | one check called from `LoadConfig` and from `VerifyPluginConfigMapTransition` |
| R-3 | Generated table drifts from `feature-gates.txt` | a new gate without a row | `./le repo generate` check refuses a stale table |
| R-4 | Coupling check floods with benign couplings (sysrib fan-in, static's vpp fallback, flow export's ddos signal) | many rows on first run | any declared edge (Dependencies, OptionalDependencies, StartAfter) satisfies the check; a benign consumer declares `StartAfter`, which orders without loading |
| R-5 | A caller of dependency resolution has no config in hand and silently resolves as if no need held | a provider missing after a reload that turned a leaf on | every caller passes the config it resolves for; no caller may pass "no config" (`ai/rules/principles.md`, silently wrong values) |
| R-6 | A need's config path drifts from the YANG (leaf renamed) and the need never matches | refusal test goes green on a build that should refuse | `Register` cannot see YANG, so a unit test per owner walks each declared path against the loaded schema and fails on a path that does not exist |
| R-7 | Refusing at boot a config that booted yesterday (stripped build, enrichment on) | operator report after upgrade | intended: the config never worked (silently empty, or failed at apply). The message names the leaf to turn off; recorded as a behavior change in the commit body |
| R-8 | An any-of need (DDoS/anomaly flow source) refused when one provider is compiled in but not configured | detection refused although trafficusage is in the build | the refusal is about the BUILD only; whether the provider is configured stays the existing doctor warning (`ddos-detect-flow-source`, `anomaly-detect-feature-source`) |
| R-10 | Refusing boot on a missing hard dependency stops a daemon that started yesterday with a half-loaded plugin | boot fails after upgrade on a stripped build | intended (owner, part e): the plugin never worked without its dependency. The message names the plugin to remove from config or the tag to build with; recorded as a behavior change in the commit body |
| R-11 | No functional binary can reach the flow-export rows (Wiring rows 1-2, AC-1..AC-4) or AC-10/AC-12: `ze-stripped` is built `ze_core ze_ssh` (`internal/le/test/functional/binaries.go`, `daemonBuildCommands`), so `flow-export` and `ddos` fail at parse there, and the full `ze` carries `ze_bgp`. AC-10 also needs ddos WITHOUT flowexport, so one stripped binary cannot hold every row | phase 1 wired only the vpp rows (traffic is ungated, vpp is gated) | Owner decision (2026-10-24): one extra functional binary built `ze_core ze_ssh ze_ddos ze_anomaly ze_flowexport` (no `ze_bgp`) proves AC-1..AC-4 and AC-12 end to end; AC-10 (no flow source at all) is proven by unit test on the same refusal function. `ze-stripped` and its docs stay unchanged |
| R-9 | Coupling list from research is incomplete (BGP-LS export, DDoS FlowSpec runtime paths were not traced) | check reports a coupling not in the AC table | the check is the authority: every row it reports gets a need or a StartAfter in this spec before it lands (owner scope, RESEARCH gate) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Config rejected that should be accepted (a need whose condition matches too widely), or a provider not started (a pull condition that never matches). Full builds are affected by the pull change; stripped builds by the refusal |
| How is it reverted? | Single commit revert; no config migration, no wire change |
| Who else touches this path? | `plugin/server/startup_autoload.go` (R3 spec will edit the data-plane read in the same file), `config/loader.go`, every plugin `register.go` listed below |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` on a build without `ze_bgp`, config with `flow-export enrichment bgp true` | → | config-need refusal via `VerifyPluginConfigMapTransition` | `test/ui/ze-stripped-optional-provider.ci` |
| daemon boot with the same config on the same build | → | config-need refusal via `LoadConfig` | `test/ui/ze-stripped-optional-provider.ci` (boot step) |
| `ze config validate` on a build without `ze_vpp`, `traffic control backend vpp` | → | config-need refusal | `test/ui/ze-stripped-optional-provider.ci` (vpp step) |
| full build, flow export configured with enrichment off and no `bgp {}` | → | conditional pull in `ResolveDependencies` | `TestFlowExportEnrichmentOffDoesNotPullRIB` (`internal/component/plugin/all/`) |
| `./le plugin coupling check` | → | coupling scan | `TestCouplingCheckFlagsUndeclaredSubscriber` (`internal/le/plugin/coupling/`) |
| `./le repo generate` | → | gated-plugin table generator | `TestGatedPluginTableMatchesManifest` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Build without `ze_bgp`; `flow-export { enrichment { bgp true; } }`; `ze config validate` | Non-zero exit. Message names the leaf `flow-export enrichment bgp`, says the BGP RIB is not in this build, and gives the fix on its own line: rebuild with `ze_bgp`, or set the leaf to false |
| AC-2 | Same build; enrichment absent or false | Accepted; flow export starts and exports records |
| AC-3 | Same build; AC-1 config at daemon boot | Daemon refuses before any plugin starts, with the AC-1 message |
| AC-4 | Same build, running with enrichment off; editor commit (and API commit, and SIGHUP reload) turning it on | Commit refused with the AC-1 message; running config unchanged. Evidence (phase 6b): on ze-stripped-flow, a SIGHUP reload to `ddos flowspec response-level enforce` is refused naming the setting and `ze_bgp`, and `show config running \| json` over SSH then answers `alert` (`reloadStrippedFlowSpecEnforce`, `strippedFlowRunningLevel`, `test/ui/ze-stripped-optional-provider.ci` seq=2); red observed with the step asserting `enforce`. The editor and API commit routes are proven at the verify door only (`TestVerifyPluginConfigRefusesAbsentProvider`), with no running-config read after them |
| AC-5 | Full build; flow export configured, enrichment off, no `bgp {}` | `bgp-rib` is not started on flow export's account |
| AC-6 | Full build; enrichment on | `bgp-rib` is pulled and ordered before flow export |
| AC-7 | Build without `ze_vpp`; `traffic control backend vpp` | Refused at validate/commit/boot naming `ze_vpp` and the leaf; no longer reaches apply |
| AC-8 | Build without `ze_vpp`; `interface backend vpp` | Same as AC-7 for the interface leaf |
| AC-9 | Build without `ze_vpp`; `firewall backend vpp` | Same as AC-7 for the firewall leaf |
| AC-10 | Build with neither `ze_flowexport` nor `ze_trafficusage`; DDoS detection or anomaly detection configured | Refused, naming both tags as alternatives |
| AC-11 | Build with one of the two flow sources | AC-10 config accepted (the existing doctor warning about an unconfigured source is unchanged) |
| AC-12 | Build without `ze_bgp`; DDoS FlowSpec mitigation configured | Refused naming `ze_bgp` |
| AC-13 | BGP-LS export of a protocol whose plugin is compiled out (IS-IS, OSPF, RSVP-TE) | Refused naming that protocol's tag. RSVP-TE publishes no BGP-LS source in any build (the only `linkstateevents.RegisterSource` calls are `internal/plugins/isis/bgpls_export.go`, `internal/plugins/ospf/bgpls_export.go`, `internal/component/bgp/plugins/epe/epe_source.go`), so it has no tag to name: `source rsvp-te` is refused as a source no plugin in this build publishes, naming the sources the build does publish (`verifyDomainSources`, phase 6) |
| AC-14 | Every further coupling the coupling check reports (R-9) | Declared as a need (refused when absent) or a `StartAfter` (benign); the check is green on landing |
| AC-15 | `Register` with a need whose provider is not in `OptionalDependencies`, or an empty path | `Register` refuses, naming the plugin and the bad need |
| AC-16 | A need path that does not exist in the loaded YANG | Per-owner schema test fails, naming the path |
| AC-17 | Route server, graceful restart, static optional edges (no needs) | Pulled and ordered exactly as before |
| AC-18 | `feature-gates.txt` gains a gate, table not regenerated | `./le repo generate` check reports the table stale |
| AC-19 | Nested gate (bgp-bmp) absent | The refusal names the compound requirement `ze_bgp` and `ze_bmp` |
| AC-20 | Coupling check with flow export's need removed | Check reports flow export to `bgp-rib` over `ribevents.BestChange` (red), and is green with it restored |
| AC-21 | Boot, config-path or token auto-load of a plugin whose HARD dependency is not in the build | Startup refused before any plugin starts; the plugin is never started without its dependency. Message names the plugin, the dependency and the build tag, with the fix on its own line |
| AC-22 | Reload auto-load and an explicit `plugin {}` block with the same missing hard dependency | Still refused (unchanged), and the message now names the build tag as in AC-21 |
| AC-23 | A hard dependency that is not a gated plugin and not registered (an external name, a typo) | Refused naming the dependency as unknown to this build, never a guessed tag |

### AC-14 couplings (phase 7, from `./le plugin coupling check` on the tree)

| Consumer | Item (kind) | Producer | Edge | Why |
|----------|-------------|----------|------|-----|
| `rib` (sysrib) | `ribevents.BestChange` (event) | `bgp-rib` | `StartAfter` | merges BGP best changes with every other protocol's; selects without them |
| `static`, `connected`, `as112` | `redistevents.ReplayRequestEvent` (event) | `redistribute-orchestrator` | `StartAfter` | answer BGP's late-join replay; without BGP nobody asks |
| `cos` | `l2tpevents.SessionCoSChange` (event) | `l2tp-auth-radius` | `StartAfter` | RADIUS CoS change; static profiles apply without it |
| `l2tp-shaper` | `l2tpevents.SessionRateChange` (event) | `l2tp-auth-radius` | `StartAfter` | RADIUS rate change; configured rate applies without it |
| `firewall` | `firewall.RegisterTables` x3 (registration) | `vrrp` | `StartAfter` on vrrp | vrrp pushes filter tables; `firewall.ApplyAll` loads the OS default backend on demand |
| `cos` | `l2tpevents.SessionUp`, `SessionDown` (event) | L2TP subsystem (no plugin) | none possible | listed as a note: no edge can name a subsystem |
| `mrt` | `registry.GetRIBDumpCallback` (seam) | `bgp-rib` | none: same gate | `feature-gates.txt` gates `internal/plugins/mrt` under `ze_bgp` with no tag of its own (owner); bgp-rib is in every such build and auto-loads from the `bgp` root that starts BGP, so `mrt routes` always finds it |
| `exabgp-bridge` | text dispatch `send bgp ...` (not visible to the check) | `bgp` | `Dependencies` | hard: no bridge config works without BGP, so no value leaf can carry a need. Run: a `ze_core ze_exabgp` build refuses to boot the bridge naming ze_bgp. A full build with no `bgp {}` block now fails boot with "missing required bgp { } block" instead of starting a bridge whose every command fails |
| `flow-export` | `ddosevent.Detected` (event) | `ddos-detect` | satisfied by ddos-detect's optional edge to flow-export | either-direction rule; a reverse StartAfter would be a cycle |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator on a BGP-free build turns on BGP flow enrichment | config text → parse → config-need refusal → error with the tag | `test/ui/ze-stripped-optional-provider.ci` |
| 2 | Operator on a full build runs flow export without enrichment | config → resolver (need false) → flow export alone | `TestFlowExportEnrichmentOffDoesNotPullRIB` |
| 3 | Operator on a VPP-free build selects the VPP traffic backend | config → refusal at validate, not at apply | `test/ui/ze-stripped-optional-provider.ci` (vpp step) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegisterRefusesNeedOutsideOptionalDependencies` | `internal/component/plugin/registry/registry_test.go` | AC-15 | |
| `TestResolveDependenciesPullsNeededProviderOnlyWhenNeedHolds` | `internal/component/plugin/registry/registry_test.go` | AC-5, AC-6, AC-17 | |
| `TestRefuseAbsentProviderNamesTagAndLeaf` | `internal/component/config/` (new file test) | AC-1 message: problem, why, fix line | |
| `TestRefuseAbsentProviderAnyOf` | same | AC-10, AC-11 | |
| `TestRefuseAbsentProviderCompoundTag` | same | AC-19 | |
| `TestRefuseAbsentProviderUnknownNamesNoTag` | same | a provider neither registered nor gated is named unknown, no guessed tag | |
| `TestRefuseAbsentProviderLeafListMember` | same | a need on a leaf-list holds on membership | |
| `TestRefuseAbsentProviderLowersOnlyForDeclaredNeeds` | same | no lowering without a declared need; no config refuses (fail closed) | |
| `TestLoadConfigRefusesAbsentProvider` | `internal/component/config/loader_test.go` | AC-3 at the LoadConfig door | |
| `TestVerifyPluginConfigRefusesAbsentProvider` | `internal/component/config/plugin_verify_test.go` | AC-4 at the verify door | |
| `TestConfigNeedPathsExistInSchema` | one per owner package (flowexport, traffic, iface, firewall, ddos, anomaly, ls_export/epe) | AC-16 | |
| `TestPathFaultRefusesEveryPathANeedCannotHoldOn` | `internal/component/config/configneedtest/` (the shared helper every owner test calls: `RequirePathsInSchemaForTest`) | AC-16: absent element, list on the path, path not ending at a leaf | |
| `TestVPPBackendNeedPullsVPPOnlyWhenSelected` | `internal/component/plugin/all/` (`ze_vpp`) | AC-7..AC-9 full-build side: vpp pulled only with `backend vpp`; owner still ordered after vpp when both run (firewall's former `StartAfter`) | |
| `TestGatedPluginTableMatchesManifest` | generator package test | AC-18 | |
| `TestCouplingCheckFlagsUndeclaredSubscriber` | `internal/le/plugin/coupling/` | AC-20 | |
| `TestCouplingCheckAcceptsStartAfter` | same | R-4 | |
| `TestBootAutoLoadRefusesMissingHardDependency` | `internal/component/plugin/server/startup_autoload_test.go` | AC-21, for both boot auto-load paths (config-path and token) | |
| `TestMissingDependencyErrorNamesTag` | `internal/component/plugin/registry/registry_test.go` | AC-22, AC-23: `ErrMissingDependency` carries the tag from the generated table, or "not in this build" with no tag for an ungated name | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ze-stripped-optional-provider` | `test/ui/ze-stripped-optional-provider.ci` (fixture beside `ui/ze-stripped-no-bgp`) | Stripped builds refuse each needed-but-absent provider by name at validate and at boot, and accept the same config with the leaf off | |
| `ze-stripped-no-bgp` (existing) | `test/ui/ze-stripped-no-bgp.ci` | Still green: the BGP-free surface parses | |
| `show-config-running` | `test/ui/show-config-running.ci` | AC-4 read: `show config running` over SSH answers the applied value and `/* SECRET-DATA */` for the password; `show config dump` with no file names it. Red observed with masking disabled in `runningConfigValue` (both the placeholder expect and the `$2a$04$` reject) | |
| `api-peer-create-delete-rib` (existing, assertion corrected) | `test/plugin/api-peer-create-delete-rib.ci` | `lifecycleFileUntouched` now asserts the running config NAMES the created peer (`reactorAPIAdapter.SetConfigTree` design); the old "names no peer" passed only on the refused REST route and went red once the route answered | |

### Interop Tests (Scope: protocol)
N-A: no wire-visible change; config and startup only.

## Files to Modify
- `internal/component/plugin/registry/registry.go` - config-need declaration on `Registration`, `Register` validation, conditional pull in `ResolveDependencies` (receives evaluated needs; registry stays a leaf with no config import)
- `internal/component/config/loader.go` - call the refusal beside `refuseInvalidCustomSections`; pass the tree's evaluated needs to `ExpandDependencies`
- `internal/component/config/plugin_verify.go` - call the same refusal in `VerifyPluginConfigMapTransition`
- `internal/component/plugin/server/startup_autoload.go` - the three resolver callers pass the config they resolve for; the two boot auto-load paths stop falling back to `resolved = needed` and refuse (AC-21)
- `internal/plugins/flowexport/register.go` - optional edge to `bgp-rib` with its need
- `internal/component/traffic/register.go` - need on the existing `vpp` edge
- `internal/component/iface/register.go`, `internal/component/firewall/register.go` - optional edge to the VPP provider with the `backend vpp` need
- `internal/plugins/ddos/detect/register.go`, `internal/plugins/anomaly/detect/register.go` - any-of need on flow export / traffic usage
- `internal/plugins/ddos/flowspec/register.go` - need on the BGP engine
- `internal/component/bgp/plugins/ls_export/register.go`, `internal/component/bgp/plugins/epe/register.go` - per-protocol needs
- `internal/plugins/static/register.go` - correct the comment that claims a conditional pull (A-3)
- `internal/le/repo/generate.go` - add the table generator action
- `feature-gates.txt` - repair the pointer to a rule file that does not exist
- `docs/architecture/plugin/feature-gates.md` - new section: optional providers, config needs, the generated table, the coupling check
- `docs/architecture/plugin/plugin-system.md` - registration fields: config needs
- `ai/rules/plugins.md` - repair the "Feature-Gate Registration" pointer; state the soft-dependency directive
- `docs/architecture/flowexport/flow-export-2-flow-records.md`, traffic and interface/firewall backend pages - what happens when the provider is absent

## Files to Create
- `internal/component/plugin/registry/gated_plugins_gen.go` - generated, always compiled: plugin name to build tag requirement
- generator under `internal/le/plugin/` (the table writer, reading `repoinventory.Plugins` and `featuretags.Gates`)
- `internal/component/config/optional_provider.go` - the one refusal function and its test
- `internal/le/plugin/coupling/` - the coupling check, `./le plugin coupling check`, wired into native verification
- `internal/test/fixture/register_ze_stripped_optional_provider.go` (the edit hook refuses a registering `init()` outside a `register*.go` file), `test/ui/ze-stripped-optional-provider.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No leaf added; needs reference existing leaves |
| YANG validation constraints | No | unchanged leaves |
| YANG custom validators | No | the refusal is a registry-driven check, not a `ze:validate` |
| CLI commands/flags | No | daemon CLI unchanged; `./le plugin coupling check` is tooling (`ai/rules/cli.md` read in its phase) |
| CLI grammar (keyword before value) | Yes | `./le plugin coupling check` follows the le grammar |
| Editor autocomplete | No | no new leaf |
| Functional test for new RPC/API | Yes | `show config running` (phase 6b): `test/ui/show-config-running.ci`, plus `test/ui/ze-stripped-optional-provider.ci` seq=2 after the refused reload |
| Pipe completeness | Yes | `show config running` answers data, so `\| json` renders it (both `.ci` above read `\| json`) |
| Env var registration | No | none |
| Doctor check for runtime dependencies | No | no runtime dependency added; the refusal precedes runtime. Existing flow-source doctor checks unchanged (R-8) |
| Prometheus counters/metrics | No | config-time refusal, nothing observable at run time |
| BGP family surface (new SAFI / capability / attribute) | No | no family change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | refusal of an already-broken config; no feature level changes |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | daemon CLI unchanged |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`: soft dependencies and the build-tag refusal |
| 6 | Has a user guide page? | Yes | `docs/guide/traffic-control.md`, flow export guide page: what an absent provider produces |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugins.md`; process protocol unchanged (A-4), stated in `docs/architecture/plugin/plugin-system.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`: the stripped-build fixture |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | Yes | `docs/architecture/plugin/feature-gates.md`, `docs/architecture/plugin/plugin-system.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | inventory and website catalog publish the needs if `internal/le/repo/inventory` carries them; `docs/plugin-overview.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | see the declared-design table below; re-run `./le spec citation anchors spec plan/release/fixes/spec-plugin-optional-dependency-refusal.md` in each phase |

Design documents declared by the files this spec changes:

| Document | Affected? | Reason |
|----------|-----------|--------|
| `docs/architecture/api/architecture.md` | Yes | describes dependency resolution at plugin startup; gains the conditional pull |
| `docs/architecture/api/process-protocol.md` | No | Stage 1 declaration unchanged (A-4); tier ordering semantics unchanged |
| `docs/architecture/config/syntax.md` | No | no syntax change; loader gains a refusal, described in `feature-gates.md` |
| `docs/architecture/core-design.md` | Yes | one sentence where it describes soft dependencies between features, if present; verify in phase 4 |
| `docs/architecture/static-routes.md` | Yes | if it repeats the "no interface stanza leaves static unconstrained" claim, correct it (A-3) |
| `docs/architecture/wire/nlri-bgpls.md` | Yes | BGP-LS export states what happens when the source protocol is compiled out |
| `docs/features/interfaces.md` | Yes | `backend vpp` without `ze_vpp` is refused at commit, not at apply |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `feature-gates.txt` ze_vpp comment ("rejected" is true only at apply today) |

## Implementation Steps

1. **Phase: Wiring** -- the declaration on `Registration`, `Register` validation, the empty refusal function called from `LoadConfig` and `VerifyPluginConfigMapTransition`, the stripped-build fixture
   - Tests: `TestRegisterRefusesNeedOutsideOptionalDependencies`, `ze-stripped-optional-provider` (fails: nothing refused yet)
   - Files: `registry.go`, `loader.go`, `plugin_verify.go`, new fixture
   - Verify: the fixture reaches both doors and fails on the missing refusal
2. **Phase: Generated table** -- generator, always-compiled table, `./le repo generate` action and staleness check
   - Tests: `TestGatedPluginTableMatchesManifest`, AC-19
3. **Phase: Refusal** -- evaluate needs over the tree, refuse with problem/why/fix and the tag(s)
   - Tests: `TestRefuseAbsentProvider*`, `TestLoadConfigRefusesAbsentProvider`, `TestVerifyPluginConfigRefusesAbsentProvider`
4. **Phase: Hard-edge refusal** -- `ErrMissingDependency` names the tag; both boot auto-load paths refuse instead of starting without the dependency
   - Tests: `TestMissingDependencyErrorNamesTag`, `TestBootAutoLoadRefusesMissingHardDependency`
5. **Phase: Conditional pull** -- resolver receives evaluated needs; the three autoload callers and `ExpandDependencies` pass their config
   - Tests: `TestResolveDependenciesPullsNeededProviderOnlyWhenNeedHolds`, `TestFlowExportEnrichmentOffDoesNotPullRIB`, AC-17
6. **Phase: First users** -- flow export, traffic; then interface, firewall, DDoS, anomaly, DDoS FlowSpec, BGP-LS/EPE
   - Flow export's optional edge to `bgp-rib` and its `enrichment bgp` need landed in phase 5 (`TestFlowExportEnrichmentOffDoesNotPullRIB` runs against the real registration); phase 6 adds its schema test and functional steps, not the declaration
   - Tests: `TestConfigNeedPathsExistInSchema` per owner; functional fixture steps for AC-1..AC-13
7. **Phase: Coupling check** -- `./le plugin coupling check`, wired into native verification; declare every further coupling it reports (AC-14)
   - Tests: `TestCouplingCheckFlagsUndeclaredSubscriber`, `TestCouplingCheckAcceptsStartAfter`
8. **Phase: Docs** -- each page in Files to Modify, edited in the phase that changes its behavior, never batched here

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Each user story's path runs through both doors |
| Correctness | The refusal fires on the build only, never on whether a provider is configured (R-8); a need with no match never pulls |
| Naming | The message names the leaf as the operator typed it, and the tag exactly as `feature-gates.txt` spells it |
| Data flow | Registry stays a leaf: no config import; the tree predicate lives in config |
| Rule: principles (one declaration) | The provider is named once, on the owner's registration; the tag only in `feature-gates.txt` |
| Rule: no-layering | No second edge kind; the late apply-time `unknown backend` errors stay as defence but are no longer the operator's first refusal |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Config-need declaration | `go doc` on `registry.Registration` shows it |
| Generated table | `./le repo generate` check green; table file present |
| Refusal at both doors | `TestLoadConfigRefusesAbsentProvider`, `TestVerifyPluginConfigRefusesAbsentProvider` |
| Coupling check | `./le plugin coupling check` green on the tree, red with AC-20's mutation |
| Stripped-build functional test | `./le test ui ze-stripped-optional-provider` (exact runner command per `ai/rules/commands.md`) |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The need evaluation reads an already-parsed tree; a missing or wrongly typed node is "need does not hold", never a panic |
| Error leakage | The message names leaves and tags only, no internal paths |
| Fail-open | A need evaluation error refuses the config; it never skips the check |

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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Config needs declared on the owner's `Registration`, beside `OptionalDependencies` (owner, DESIGN gate 2026-10-21) | YANG extension on the leaf, like `ze:backend` | One declaration drives pull, order and refusal; the YANG route names the provider twice |
| Need = path + trigger value + any-of providers | path presence only | both first users trigger on a value (boolean true, backend name); flow sources are alternatives |
| Refusal is about the build, not about configuration | refuse when a provider is not configured | an unconfigured provider is a doctor warning today; refusing it would break configs that work |
| One refusal function, called from `LoadConfig` and `VerifyPluginConfigMapTransition` | verifier only | verifiers do not run at boot |
| Benign couplings declare `StartAfter` | allowlist in the check | an allowlist is a second list; `StartAfter` orders without loading |
| A need path may cross a YANG list: the list keyword is followed by `registry.AnyEntry` (`"*"`), and the need holds when ANY entry carries the value at the rest of the path; still path + value, never presence (main thread, 2026-10-24, AC-13) | refuse in ls_export's own verifier; declare `StartAfter` only | BGP-LS export's protocol leaf `source` sits inside `list domain` (`ze-bgp-ls-export-conf.yang`). A verifier-only refusal misses boot (R-2), and `StartAfter` alone drops AC-13. The marker is explicit because the lowered tree cannot tell a list map from a container map without the schema; `PathFault` checks each list is followed by it. The refusal names the concrete entry (`bgp-ls-export domain core source isis`) |
| AC-4 "running config unchanged" is proven through a new daemon-side read of the in-memory running config, reachable over SSH in the stripped build, asserted after the refused reload (owner, 2026-10-24; scope widened) | in-process unit test on reload only; keep the indirect functional evidence | no entry point answers the running config today: REST `config/running` and gRPC `GetRunningConfig` dispatch `show config dump` with no file, which refuses (`plan/journal/unwired-feature.md`, 2026-10-24, owned here), and after a refused reload the file already holds the new config |
| mrt is gated under `ze_bgp` as a nested BGP plugin (owner, 2026-10-24, coupling check) | hard `Dependencies: {bgp-rib}`; a presence trigger on `mrt routes` | every mrt stream is BGP data and its RIB dump is filled only by `bgp-rib` (`registry.SetRIBDumpCallback`); a hard edge pulls the RIB for updates-only configs, and a presence trigger is a second trigger kind. Inside a BGP build, `routes` must still find `bgp-rib` loaded |
| A need naming more than one provider is a set of alternatives: it refuses when the build carries none of them and pulls none of them when it holds; a single-provider need keeps the pull. One predicate, `registry.ConfigNeed.PullsProvider`, decides it for boot resolution (`optionalProviderPulled`) and reload (`appendHeldNeedProviders`) (main thread, 2026-10-24, logic bug found by `anomaly-show` and `anomaly-observe-show` on darwin) | pull every compiled-in alternative when the need holds (phase 5 behaviour) | pulling every alternative started flow-export for a config that never configured it, and flow-export hard-depends on interface, so detection enabled refused boot on a host with no interface backend. The operator's chosen source loads from its own config root; an unconfigured source stays the doctor warning (R-8) |
| A plugin extraction failure is reported once: `verifyTree` returns it as a `*config.PluginExtractionError` and does not ask the dependency refusal, which cannot answer without the extracted set. A registered extractor's failure carries its section (`RegisterPluginExtractor(section, fn)`), and a door that checks that section itself drops the relayed copy: `ze config validate` always (its bgp check reports the same `ResolveBGPTree` failure), the editor only when its own pass or the engine's peer resolution already refused the config. `ze config validate`'s separate `ExtractPluginsFromTree` call is folded into the typed error and keeps the `config-plugin-extract` code (main thread, 2026-10-24, `errors-pipe-match.et` red) | skip the extraction failure in the dependency check and rely on each door's own producer | doctor and the editor have no producer for `plugin { }` block extraction failures, so skipping would call valid at those doors a config boot refuses |

## Journal Rows This Spec Closes

The 2026-10-20 plugin dependency census in `plan/journal/silent-fall-through.md`
already recorded couplings this spec declares. The coupling check (phase 6)
must report each, and AC-14 covers the ones not named in AC-1..AC-13:

| Row (silent-fall-through.md) | Covered by |
|------------------------------|-----------|
| ddos flowspec responder, exabgp bridge, flowexport `enrichment bgp` reach BGP undeclared | AC-1, AC-12; the exabgp bridge under AC-14 |
| mrt table dump and `show system` NTP block cannot tell "not built" from "not configured" | AC-14 (mrt `dump` needs `bgp-rib`); the NTP show block is an answer, not a config, and stays outside unless the check reports it |
| redistribute `destination`, bgp ls_export `source` are free strings naming another plugin | AC-13 for ls_export: fixed in phase 6 (compiled-out IGP by its need with the tag; any other unpublished value by the `verifyDomainSources` tree validator). redistribute `destination` under AC-14: a different shape, because its consumers register at plugin start (`ReregisterConsumer`, some from an external process such as `fakeredist`), so no parse-time set of valid names exists; the row stays open for it |
| boot auto-load starts a plugin without its HARD dependency after a WARN | AC-21..AC-23 (scope part e, owner 2026-10-21) |

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `unwired-feature.md` | 2026-10-24 | REST `/config/running` and gRPC `GetRunningConfig` return no running config (fixed in phase 6b: `show config running`) |
| `gate-excludes-part-of-its-population.md` | 2026-10-24 | `ze config validate` accepts a config whose hard dependency is compiled out, which boot refuses (phase 7 run, exabgp bridge) |
| `gate-excludes-part-of-its-population.md` | 2026-10-24 | a compiled-out plugin's config root is refused as `unknown top-level keyword` with no build tag (phase 7 run, mrt and bgp) |

## Known Limitations
- External plugins cannot declare an optional edge or a need: the Stage 1 declaration carries `Dependencies` only (A-4). Not in the owner's scope for this spec.
- Rulings 3 and 4 of the handover are their own specs.

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
- [ ] AC-1..AC-23 all demonstrated
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
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-plugin-optional-dependency-refusal.md` only, in the same `./le commit create` script

---

## Implementation Summary

### What Was Implemented
- `registry.ConfigNeed` on `Registration.ConfigNeeds` (path + value + any-of providers, `AnyEntry` across a list), validated by `validateConfigNeeds`; conditional pull through `optionalProviderPulled` and `ConfigNeed.PullsProvider` (single-provider needs pull, any-of needs pull nothing).
- One refusal, `config.refuseAbsentOptionalProviders`, called from `LoadConfig` and `VerifyPluginConfigMapTransition`; `config.ConfigNeedHolds` is the one evaluator for refusal and pull.
- Generated `registry/gated_plugins_gen.go` (name and config root to build tags) from `internal/le/plugin/gatedtable`, checked by `./le plugin gated-table` and a verify stage; compound tags for nested gates.
- Hard-edge refusal: `*registry.MissingDependencyError` names plugin, dependency and tag; both boot auto-load paths call `refuseBootAutoLoad` instead of `resolved = needed`; `refuseUnresolvableDependencies` brings validate, editor, API and doctor in line with boot.
- `config.TopLevelKeywordHint`: a compiled-out root names its build tag.
- `./le plugin coupling check` (`internal/le/plugin/coupling`), wired into native verification; every coupling it reported is declared (AC-14 table).
- Needs on flow export, traffic, interface, firewall, DDoS detect/FlowSpec, anomaly detect, BGP-LS export; mrt regated under `ze_bgp`; exabgp bridge hard-depends on `bgp`.
- `show config running` (`config/cli/running_rpc.go`), answered by REST `/config/running` and gRPC `GetRunningConfig`.
- Functional binary `ze-stripped-flow` and `test/ui/ze-stripped-optional-provider.ci`, `test/ui/show-config-running.ci`.

### Bugs Found/Fixed
- Closure review: `ze doctor` reported a bgp tree that does not resolve twice, once from `config-semantics` (relayed `*config.PluginExtractionError`, Section bgp) and once from `doctor-config-bgp-peer`. Fixed in `config.ValidateSemantics`; `TestDoctorReportsBGPResolveFailureOnce` (red observed with the skip disabled, green restored).
- Implementation phase: any-of needs pulled every alternative (anomaly-show red on darwin), fixed by `ConfigNeed.PullsProvider`; extraction failure reported twice by the editor (`errors-pipe-match.et`), fixed by `PluginExtractionError`.

### Documentation Updates
- Pages listed in Files to Modify, plus `docs/architecture/plugin/plugin-system.md` (doctor drop, anchor `validate_semantic.go -- ValidateSemantics`, closure), `docs/guide/api.md`, `docs/guide/lifecycle.md`, `docs/architecture/api/commands.md`, `docs/guide/command-catalogue.md`.
- `../wiki/command-catalog.md` regenerated with `./le cli catalog update file ../wiki/command-catalog.md` (488 commands, adds `show config running`); diff holds only this spec's command.
- `./le doc check verify`: Documentation tests PASSED (closure run).

### Deviations from Plan
- Scope widened by owner decisions recorded in Key Design Decisions and R-11: `ze-stripped-flow` binary, `show config running` for AC-4, mrt under `ze_bgp`, any-of needs pull nothing, both validate-door gaps fixed.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-3 assumed interface loads in every real config | `interface` auto-loads only from its own root; static pulls it as before | phase 1 reading `iface/register.go` | static comment corrected; behaviour unchanged (`TestOptionalEdgesWithoutNeedsPullAsBefore`) |
| approach | Phase 5 pulled every provider of an any-of need | pulling alternatives started an unconfigured flow-export and its hard deps | `anomaly-show` red on darwin | `ConfigNeed.PullsProvider` |
| approach | verifyTree relayed the bgp extraction failure to doors that report it themselves | editor and doctor listed one fault twice | `errors-pipe-match.et`; doctor at closure review | section on `PluginExtractionError`, dropped per door |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| a: config need on the optional edge, refused at verify/commit naming the tag | Done | `registry/config_need.go`, `config/optional_provider.go` | |
| b: generated always-compiled name-to-tag table | Done | `registry/gated_plugins_gen.go`, `internal/le/plugin/gatedtable` | |
| c: flow export and traffic first users | Done | `plugins/flowexport/register.go`, `component/traffic/register.go` | |
| d: coupling check | Done | `internal/le/plugin/coupling` | exit 0 on the tree (phase 7) |
| e: hard-edge refusal on every load path | Done | `plugin/server/startup_autoload.go` `refuseBootAutoLoad`, `registry/missing_dependency.go` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1..AC-4 | Done | `test/ui/ze-stripped-optional-provider.ci`, `TestVerifyPluginConfigRefusesAbsentProvider` | AC-4 editor/API commit at verify door only |
| AC-5, AC-6 | Done | `TestFlowExportEnrichmentOffDoesNotPullRIB` | |
| AC-7..AC-9 | Done | `.ci` vpp step, `TestVPPBackendNeedPullsVPPOnlyWhenSelected`, per-owner `TestConfigNeedPathsExistInSchema` | |
| AC-10, AC-11 | Done | `TestRefuseAbsentProviderAnyOf`, `TestDetectionEnabledPullsNoFlowSource` | |
| AC-12 | Done | `.ci` seq=2 (FlowSpec enforce refused naming ze_bgp) | |
| AC-13 | Done | `ls_export` `config_need_test.go`, `export_source_test.go` | |
| AC-14, AC-20 | Done | AC-14 table; `TestCouplingCheckFlagsUndeclaredSubscriber` | |
| AC-15, AC-16 | Done | `TestRegisterRefusesNeedOutsideOptionalDependencies`, `TestPathFaultRefusesEveryPathANeedCannotHoldOn` | |
| AC-17 | Done | `TestOptionalEdgesWithoutNeedsPullAsBefore` | |
| AC-18, AC-19 | Done | `TestGatedPluginTableMatchesManifest`, `TestRefuseAbsentProviderCompoundTag` | |
| AC-21..AC-23 | Done | `TestBootAutoLoadRefusesMissingHardDependency`, `TestMissingDependencyErrorNamesTag`, `TestVerifyPluginConfigRefusesMissingHardDependency` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| every row of the TDD Unit Tests table | Done | each named file | `grep -rln "func <name>" internal`: each found (TestConfigNeedPathsExistInSchema in 8 owner packages) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Files to Modify and Files to Create | Done | plus the owner-widened files named in the session state |

### Audit Summary
- **Total items:** 5 requirements, 23 ACs
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** scope widenings, recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A build without the provider runs the owner and refuses only the config that needs it, naming the tag | functional | `test/ui/ze-stripped-optional-provider.ci` on `ze-stripped-flow` (no ze_bgp): `enrichment bgp true` refused at validate and boot naming ze_bgp, accepted with the leaf off; green in the targeted verify run |
| A refused reload leaves the running config unchanged | functional | same `.ci` seq=2 via `show config running \| json` after a refused SIGHUP; red observed with the probe asserting `enforce` |
| A missing hard dependency never starts half-loaded | unit + hand run | `TestBootAutoLoadRefusesMissingHardDependency`; `ze_core ze_exabgp` build refuses boot and validate with the same three lines naming ze_bgp |
| Undeclared cross-gate couplings are found | tooling test | `TestCouplingCheckFlagsUndeclaredSubscriber` red with flow export's edge removed |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | every AC is implemented; Linux-only functional runs are owed evidence, listed under Pre-Commit Verification | `plan/release/fixes/spec-plugin-optional-dependency-refusal.md` (this spec, closing) |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/plugin-optional-dependency-refusal-d3d51937-c4e7-4d97-be6d-290838333fb9.md` (163 files) |
| `./le spec review check` | clean: "OK (123 code files, clean, hashes match)" |
| Rounds | 3 |
| Reviewer lenses used | round 3: the `ForTest` rename only (finding 3: grep for the old name, gofmt, the 9 package tests); round 1: pre-checks (`./le repo check`, `./le commit audit`), wiring and logic of the core producers (`config_need.go`, `optional_provider.go`, `plugin_verify.go`, `loader.go`, `startup_autoload.go` refusal, `running_rpc.go` masking), doctor door (open item 4), security of the need walk and the running-config view; round 2: the doctor fix only (test red/green, gofmt, refuse-by-default). Not re-read line by line at closure: coupling scanner, gatedtable generator, fixture, ls_export source validator, REST/gRPC handlers (each had phase-level review and tests) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `ze doctor` reports one bgp resolve failure twice: `config-semantics` relays the bgp extractor's `*PluginExtractionError` and `doctor-config-bgp-peer` reports the same `ResolveBGPTree` failure (`validatePeersFromTree` to `peersAndDynamicGroups`) | `internal/component/config/validate_semantic.go` `ValidateSemantics` | skip Section bgp, as `cmd_validate.runValidation` does; `TestDoctorReportsBGPResolveFailureOnce`; page `plugin-system.md` |
| 2 | ISSUE | `./le commit audit`: `TestBuildTag_Gate11_SymbolMatrix` 6 to 5 rows, unexplained | `cmd/ze/hub/build_tag_gate11_absent_test.go` | ledger row `test/weakened/bf9d5372.md` (rows folded by the owner's mrt regating) |
| 3 | ISSUE | `./le doc wiring` (refusing at `./le commit create`): exported `configneedtest.RequirePathsInSchema` has no non-test reference. Round 1 had read it as a NOTE (a test-helper package by design); the check refuses it, and its `ForTest` exemption is the declared form for a test-only helper | `internal/component/config/configneedtest/configneedtest.go` and its 8 `config_need_test.go` callers | renamed `RequirePathsInSchemaForTest` (exempt by `internal/le/doc/wiring/wiring.go`); the 9 packages' tests rerun green |

NOTE: `runningConfigValue` clones a `[]string` unmasked; no `ze:sensitive` leaf-list exists and `config.SecretKeys` collects leaves only.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `optional_provider.go`, `gated_plugins_gen.go`, `coupling/coupling.go`, fixture, both new `.ci` | yes | `ls` at closure: all six listed with sizes |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| all | each TDD test exists | `grep -rln "func <name>" internal` for all 19 names: each 1 (schema test 8) |
| item 4 (doctor) | reported once | `go test -race` bgp/config + config ok; forced red: `config-semantics relays the bgp resolve failure (config-plugin-verify)` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| validate, boot, SIGHUP on ze-stripped-flow | `test/ui/ze-stripped-optional-provider.ci` | green in targeted verify run (session state) |
| `show config running` over SSH | `test/ui/show-config-running.ci` | green; red observed with masking disabled |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | only `rib_bestchange.go` emits `ribevents.BestChange` |
| A-2 | confirmed | `TestGatedPluginTableMatchesManifest` |
| A-3 | broken basis, harmless | `TestOptionalEdgesWithoutNeedsPullAsBefore`; Mistake Log |
| A-4 | confirmed | needs declared on internal registrations only; Stage 1 unchanged |

### Owed, not run on this host (darwin)
| Command | Why |
|---------|-----|
| `./le test bgp plugin ddos-detect-characterize ddos-detect-external-warns ddos-detect-mitigate ddos-show` | `needs-linux`, skipped on darwin |
| `./le test functional` flow-export, traffic, vpp and static suites | red identically on the pre-change binary here (darwin environment) |
| `./le verify current` | full gate after the closing commits; Thomas runs it |

Foreign reds checked against this diff: rfc7606/rfc9774 (`docs/architecture/wire/messages.md`, `nlri.md` hunks are another session's), watchdog wire-methods, `check_ratchets.go` compound guard, rules-points quality/testing, `rfc check` findings (no RFC stem touched here); `ze_ssh` red is the missing vendor ssh package.

# Spec: plugin-optional-dependency-refusal

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-21 |

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
| A-1 | `bgp-rib` is the only producer of `ribevents.BestChange` | research report; `bgp/plugins/rib/register.go` registers the namespace | flow export's edge names the wrong provider | grep every `BestChange.Emit` | unvalidated |
| A-2 | le sees every registration because it is built with every gate tag | `le` launcher builds with `ze_le` plus every feature tag | generated table misses plugins | generator test compares table rows against `featuretags.Gates` packages | unvalidated |
| A-3 | static's `interface` edge can be pulled unconditionally without harm, because interface is loaded in every real config | `static/register.go` comment contradicts `ResolveDependencies` | conditional pull changes static's startup | read interface autoload; functional test with static and no interface stanza | unvalidated |
| A-4 | No external plugin needs an optional edge in this spec | Stage 1 carries `Dependencies` only; no external plugin is a first user | the refusal misses externals | owner scope statement (SCOPE gate: a+b+c+d) | unvalidated |

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
| AC-4 | Same build, running with enrichment off; editor commit (and API commit, and SIGHUP reload) turning it on | Commit refused with the AC-1 message; running config unchanged |
| AC-5 | Full build; flow export configured, enrichment off, no `bgp {}` | `bgp-rib` is not started on flow export's account |
| AC-6 | Full build; enrichment on | `bgp-rib` is pulled and ordered before flow export |
| AC-7 | Build without `ze_vpp`; `traffic control backend vpp` | Refused at validate/commit/boot naming `ze_vpp` and the leaf; no longer reaches apply |
| AC-8 | Build without `ze_vpp`; `interface backend vpp` | Same as AC-7 for the interface leaf |
| AC-9 | Build without `ze_vpp`; `firewall backend vpp` | Same as AC-7 for the firewall leaf |
| AC-10 | Build with neither `ze_flowexport` nor `ze_trafficusage`; DDoS detection or anomaly detection configured | Refused, naming both tags as alternatives |
| AC-11 | Build with one of the two flow sources | AC-10 config accepted (the existing doctor warning about an unconfigured source is unchanged) |
| AC-12 | Build without `ze_bgp`; DDoS FlowSpec mitigation configured | Refused naming `ze_bgp` |
| AC-13 | BGP-LS export of a protocol whose plugin is compiled out (IS-IS, OSPF, RSVP-TE) | Refused naming that protocol's tag |
| AC-14 | Every further coupling the coupling check reports (R-9) | Declared as a need (refused when absent) or a `StartAfter` (benign); the check is green on landing |
| AC-15 | `Register` with a need whose provider is not in `OptionalDependencies`, or an empty path | `Register` refuses, naming the plugin and the bad need |
| AC-16 | A need path that does not exist in the loaded YANG | Per-owner schema test fails, naming the path |
| AC-17 | Route server, graceful restart, static optional edges (no needs) | Pulled and ordered exactly as before |
| AC-18 | `feature-gates.txt` gains a gate, table not regenerated | `./le repo generate` check reports the table stale |
| AC-19 | Nested gate (bgp-bmp) absent | The refusal names the compound requirement `ze_bgp` and `ze_bmp` |
| AC-20 | Coupling check with flow export's need removed | Check reports flow export to `bgp-rib` over `ribevents.BestChange` (red), and is green with it restored |

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
| `TestLoadConfigRefusesAbsentProvider` | `internal/component/config/loader_test.go` | AC-3 at the LoadConfig door | |
| `TestVerifyPluginConfigRefusesAbsentProvider` | `internal/component/config/plugin_verify_test.go` | AC-4 at the verify door | |
| `TestConfigNeedPathsExistInSchema` | one per owner package (flowexport, traffic, iface, firewall, ddos, anomaly, ls_export/epe) | AC-16 | |
| `TestGatedPluginTableMatchesManifest` | generator package test | AC-18 | |
| `TestCouplingCheckFlagsUndeclaredSubscriber` | `internal/le/plugin/coupling/` | AC-20 | |
| `TestCouplingCheckAcceptsStartAfter` | same | R-4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ze-stripped-optional-provider` | `test/ui/ze-stripped-optional-provider.ci` (fixture beside `ui/ze-stripped-no-bgp`) | Stripped builds refuse each needed-but-absent provider by name at validate and at boot, and accept the same config with the leaf off | |
| `ze-stripped-no-bgp` (existing) | `test/ui/ze-stripped-no-bgp.ci` | Still green: the BGP-free surface parses | |

### Interop Tests (Scope: protocol)
N-A: no wire-visible change; config and startup only.

## Files to Modify
- `internal/component/plugin/registry/registry.go` - config-need declaration on `Registration`, `Register` validation, conditional pull in `ResolveDependencies` (receives evaluated needs; registry stays a leaf with no config import)
- `internal/component/config/loader.go` - call the refusal beside `refuseInvalidCustomSections`; pass the tree's evaluated needs to `ExpandDependencies`
- `internal/component/config/plugin_verify.go` - call the same refusal in `VerifyPluginConfigMapTransition`
- `internal/component/plugin/server/startup_autoload.go` - the three resolver callers pass the config they resolve for
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
- `internal/test/fixture/ui_fixture_ze_stripped_optional_provider.go`, `test/ui/ze-stripped-optional-provider.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No leaf added; needs reference existing leaves |
| YANG validation constraints | No | unchanged leaves |
| YANG custom validators | No | the refusal is a registry-driven check, not a `ze:validate` |
| CLI commands/flags | No | daemon CLI unchanged; `./le plugin coupling check` is tooling (`ai/rules/cli.md` read in its phase) |
| CLI grammar (keyword before value) | Yes | `./le plugin coupling check` follows the le grammar |
| Editor autocomplete | No | no new leaf |
| Functional test for new RPC/API | No | no RPC; functional test is `test/ui/ze-stripped-optional-provider.ci` |
| Pipe completeness | No | no new show command |
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
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | see the declared-design table below; re-run `./le spec citation anchors spec plan/spec-plugin-optional-dependency-refusal.md` in each phase |

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
4. **Phase: Conditional pull** -- resolver receives evaluated needs; the three autoload callers and `ExpandDependencies` pass their config
   - Tests: `TestResolveDependenciesPullsNeededProviderOnlyWhenNeedHolds`, `TestFlowExportEnrichmentOffDoesNotPullRIB`, AC-17
5. **Phase: First users** -- flow export, traffic; then interface, firewall, DDoS, anomaly, DDoS FlowSpec, BGP-LS/EPE
   - Tests: `TestConfigNeedPathsExistInSchema` per owner; functional fixture steps for AC-1..AC-13
6. **Phase: Coupling check** -- `./le plugin coupling check`, wired into native verification; declare every further coupling it reports (AC-14)
   - Tests: `TestCouplingCheckFlagsUndeclaredSubscriber`, `TestCouplingCheckAcceptsStartAfter`
7. **Phase: Docs** -- each page in Files to Modify, edited in the phase that changes its behavior, never batched here

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

## Journal Rows This Spec Closes

The 2026-10-20 plugin dependency census in `plan/journal/silent-fall-through.md`
already recorded couplings this spec declares. The coupling check (phase 6)
must report each, and AC-14 covers the ones not named in AC-1..AC-13:

| Row (silent-fall-through.md) | Covered by |
|------------------------------|-----------|
| ddos flowspec responder, exabgp bridge, flowexport `enrichment bgp` reach BGP undeclared | AC-1, AC-12; the exabgp bridge under AC-14 |
| mrt table dump and `show system` NTP block cannot tell "not built" from "not configured" | AC-14 (mrt `dump` needs `bgp-rib`); the NTP show block is an answer, not a config, and stays outside unless the check reports it |
| redistribute `destination`, bgp ls_export `source` are free strings naming another plugin | AC-13 for ls_export; redistribute `destination` under AC-14 |

Not covered, owner decision outstanding: the census row "boot auto-load starts
a plugin without its HARD dependency after a WARN". It is the hard-edge sibling
of this spec and uses the same name-to-tag table.

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
- [ ] AC-1..AC-20 all demonstrated
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
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/spec-plugin-optional-dependency-refusal.md` only, in the same `./le commit create` script

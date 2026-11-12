# Spec: route-policy-1-gate-proof-and-forked-sysrib -- the Loc-RIB import gate cannot be bypassed by a forked system RIB, and a daemon test proves each import chain

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | A configuration Ze accepts today (`plugin { external <name> { run "ze plugin rib"; } }`) programs BGP routes the central `rib filter import` chain rejected, installs no static, connected, IS-IS or OSPF route, and resolves no next hop. That is accepted and ineffective configuration with a fail-open import filter, in code the first release ships. The daemon tests and the two documentation rows prove and declare the same shipped gate, so they are owed with it. |
| RFC gaps | - |
| Depends | `plan/post-release/features/spec-route-policy-0-umbrella.md` (the gate, the chains and the fixture drivers this spec proves; its closure review round 4 produced the three findings this spec owns) |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The closure review of `spec-route-policy-0-umbrella` (round 4, 2026-11-12)
handed back three findings. The owner moved them here on 2026-11-12:
"route-policy's three review findings get in a new spec". This spec owns
all three, and the umbrella no longer does.

| # | Finding (umbrella id) | What is wrong today |
|---|-----------------------|---------------------|
| 1 | A forked system RIB goes around the gate (B-2, R4-2, broken assumption A-2) | The system RIB has two inputs. In the daemon it follows the Loc-RIB, whose every insert passed the import gate. Forked, it holds no Loc-RIB and follows the BGP RIB's own best-change event, which is published whatever the gate decided. Research for this spec found the forked system RIB is wrong in five more ways (Current Behavior) |
| 2 | No daemon test proves an import chain outside IS-IS and OSPF (B-1, R4-1) | `test/plugin/rib-import-central-bogons.ci` and `test/plugin/static-import-filter-distance.ci` were listed by the umbrella and never committed. Their fixture drivers were. No test configures `filter import` under `rib`, `static` or `connected` and reads the result from a running daemon |
| 3 | Route policy is undeclared and uncompared (B-3, R4-3) | No `features/` declaration covers route policy, and `docs/comparison.md` has no row for per-protocol import and export policy |

Goals:

| Goal | Proof it is met |
|------|-----------------|
| An operator cannot run Ze with a system RIB that ignores the import gate | The config that forks the system RIB is refused at commit with the fix, by `ze config validate` and by start alike, and the ungated input no longer exists |
| "Never install a bogon from any protocol" (the umbrella's goal) is proven on a running daemon | One daemon test per Loc-RIB import attachment that had none: central, static, connected |
| What Ze claims about route policy is declared and compared | A feature declaration with its evidence, and a comparison row with source anchors |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the process boundary every plugin is held to
  → Constraint: "Every plugin, ours included, runs either internal (a goroutine in the daemon) or external (a forked subprocess that talks only over TLS and RPC). The SDK hides which. Code that only works internal is broken." The fix in this spec makes the system RIB a declared exception to that sentence. The exception mechanism already exists (`Registration.RunsInProcessOnly`); the door and `docs/plugin-overview.md` are corrected in the same work to name the first shipped plugins that use it.
- [ ] `ai/doors/forwarding.md` - the route path and who writes the FIB
  → Constraint: protocols insert `locrib.Path` values; a forked producer ships them over the `route-install` RPC because `locrib.Default()` is nil in a subprocess. The gate therefore already covers every forked producer. Only a forked consumer of the Loc-RIB is uncovered.
  → Constraint: "Exact or refuse, at verify." A backend that cannot apply a config exactly refuses it; it never approximates.
- [ ] `docs/architecture/forked-route-install.md` - how a forked producer's route reaches the system RIB
  → Decision: "A sink per installer, not a remote Loc-RIB. ... Making `locrib.Default()` return a remote proxy was refused, because it would change a core contract and cross the module tiers." A design that streams the Loc-RIB to a forked system RIB meets the same refusal from the other side.
  → Constraint: the distance declaration seam is process-global and the system RIB is its only publisher. A forked system RIB publishes into its own process, so the daemon's Loc-RIB never sees `rib { distance }`.
- [ ] `docs/architecture/rib/unified-locrib.md` - the Loc-RIB the gate sits on
  → Constraint: `OnChange` subscribers run synchronously under the Loc-RIB write lock. A cross-process stream fed from there would put an RPC under that lock.
- [ ] `docs/plugin-overview.md`, "Plugin modes" and `docs/architecture/doctor-and-health-checks.md`, "External plugin and managed fleet checks"
  → Constraint: `RunsInProcessOnly` is read by the `plugin-external-in-process` doctor check, "by reading the field, never a list of plugin names". The page also says "No shipped plugin sets the field yet", which this spec makes false and corrects.
  → Decision: a `plugin external` name of the wrong form is refused at commit, by `ze config validate` and by start, because validating what start refuses was a recorded defect (`plan/journal/validate-accepts-what-start-refuses.md`). The same route is taken here.
- [ ] `ai/doors/config-yang.md` - where a commit-time rule must live
  → Constraint: "`ze config validate` never calls a live `OnConfigVerify`." A refusal placed only in a plugin's verify callback passes offline and fails at start. The refusal must sit where both paths run.
- [ ] `docs/guide/route-policy.md` - the operator's syntax for chains and `show rib candidates`
  → Constraint: a filter's `accept` means continue; only `final` grants acceptance. The tests' configs must state `final`.
- [ ] `docs/contributing/feature-maturity.md` - the declaration format
  → Constraint: Name, Kind and Scope are required; Level when the scope is complete or partial; Components when implemented. `supported` needs recorded green runs, a doc review and a defect review. This declaration claims `experimental`.
- [ ] `.claude/skills/ze-test/SKILL.md` - how a `.ci` is written
  → Constraint: a new `.ci` is drafted under `test/draft/`, proven under load, then promoted with a plain move. `test/draft/` is ignored by git, which is how the umbrella lost these two tests: a draft that is never promoted never reaches a commit.

### RFC Summaries (Scope: protocol)
- [ ] No RFC applies. An import policy on the local RIB is local policy (umbrella A-3, confirmed against RFC 2328 Section 16 and RFC 1195 Section 3.10). Which process hosts the system RIB is an implementation matter.

**Key insights:**
- The gate is correct and complete for producers. The defect is one consumer with a second, ungated input.
- A forked system RIB is not a degraded mode that this spec could repair by gating one event. It sees BGP only, has no next-hop resolver, publishes distances nobody reads, and already answers `show rib candidates` with "the system RIB plugin must run inside the daemon". The honest state of the code is that it must run in the daemon; the config does not say so.
- AS112's own engine comment already names the gap this spec closes for all three plugins: an operator may write the external block because "nothing in config validation forbids it".
- The two missing tests are half written: the fixture drivers and their registrations are committed, and they inject through BGP and static, not through OSPF as the umbrella's criteria say.

## Current Behavior (MANDATORY)

**Source files read:** by the session that wrote this spec, on 2026-11-12. Two facts below are marked (research): they come from its read-only research agent and are confirmed by the implementer before the first edit.
- [ ] `internal/component/sysrib/sysrib.go` `run` - with a Loc-RIB it subscribes to `OnChange`, walks the existing bests and starts one worker; without one it subscribes to `ribevents.BestChange` and emits a broadcast `ribevents.ReplayRequest`. Its comment: "Forked setups ... leave Loc-RIB unwired because processes cannot share a struct; sysrib falls back to the BGP EventBus stream."
  → Constraint: the Loc-RIB branch stays as it is. Only the other branch goes.
- [ ] `internal/component/sysrib/sysrib.go` `processEvent`, `effectivePriority`, `reapplyAdminDistances`, the `rankedByLocRIB` field - `processEvent` serves both inputs. A batch marked `FromLocRIB` replaces the per-prefix entry and keeps the distance the Loc-RIB resolved; an unmarked batch is upserted per protocol and ranked by `effectivePriority`. `reapplyAdminDistances` skips every route the Loc-RIB ranked.
  → Constraint: `processEvent` stays, as the Loc-RIB feed's path. What goes is the unmarked branch and the arbitration that exists only for it.
  → Constraint (research): about 62 direct `processEvent` calls across 13 test files build a batch without `FromLocRIB`. Each is re-pointed at the Loc-RIB path or judged through `ze-test-judge` (Risks, R-2).
- [ ] `internal/component/sysrib/register.go` `init`, `runSysRIBPlugin`, `declareDistances`, `reselectLocRIB` - the registration sets no `RunsInProcessOnly`; `runSysRIBPlugin` calls `SetLocRIB(locrib.Default())` and comments that nil leaves "the EventBus fallback path active"; `reselectLocRIB` does nothing without a Loc-RIB.
- [ ] `internal/core/rib/locrib/default.go` `Default` - returns nil when `ze.plugin.hub.token` is set, that is, in a forked plugin process.
- [ ] `internal/component/sysrib/candidates.go` `errNoLocRIB`, `showRIBCandidates`, `candidatePath` - without a Loc-RIB the command refuses: "the system RIB plugin must run inside the daemon: remove any `plugin { external ... }` entry that forks it, then restart with `ze signal restart`". Each path answers `source`, `distance`, `tag`, `selected`, `hidden`, `hidden-by`, `chain`.
- [ ] `internal/core/bgp/ribevents/ribevents.go` `BestChange`, `ReplayRequest`, `BestChangeBatch.FromLocRIB` - both events are registered on namespace `bgp-rib`; `FromLocRIB` is `json:"-"`, an in-process marker that never crosses a process boundary.
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` `publishBestChanges` - emits `ribevents.BestChange` for every best the BGP RIB elected; nothing in it reads the gate's verdict.
- [ ] `internal/component/bgp/plugins/rib/rib_remote.go` `insertLocRIB`, `removeLocRIB` - the BGP RIB mirrors its best into the Loc-RIB with `InsertForward` in process, or through the route-install sink when forked. Either way the path meets the gate. (research) The mirror happens before the event is published.
- [ ] `internal/component/plugin/registry/registry.go` `Registration.RunsInProcessOnly` - "A config that names it in a `plugin external` block starts a process that silently does nothing, so the plugin-external-in-process doctor check reports that block".
- [ ] `internal/component/plugin/doctor/check_external.go` `checkExternalPluginNames`, `checkExternalInProcess`, `inProcessOnlyRegistration`, `externalInProcessRow` - the name check reads the parsed tree, "on the tree that ReadConfig's refusal does not hide"; the in-process check reads `ctx.Plugins`, resolves each external block through `zeplugin.RegistryNames` and `registry.Lookup`, and reports `doctor-plugin-external-unsupported` with the fix "move it to an internal block".
  → Constraint: once the commit refuses the block, the in-process check must still be handed it. Whether `ctx.Plugins` is filled for a refused config decides if the check moves to the parsed tree (Assumptions, A-4).
- [ ] `internal/component/config/validators.go` `ExternalPluginNameValidator`, `InternalPluginNameValidator` - commit-time validators on leaves of the plugin block; the second already reads the plugin registry from this package.
- [ ] `internal/component/plugin/yang/ze-plugin-conf.yang` `list external` - `ze:validate "external-plugin-name"` sits on the `name` leaf; the `run` leaf carries no validator. Its description: "A value of the form 'ze.<plugin>' names a built-in plugin, which the engine then runs in this process rather than as a fork."
  → Constraint: a validator on `name` sees only the name. The in-process-only refusal judges the `run` value, so it attaches to the `run` leaf, and it must accept the `ze.<plugin>` form, which runs in the daemon.
- [ ] `internal/component/config/loader.go` external block extraction, `MarkInternalPlugin` - a `use` value and a `run` that resolves to an internal transport set `Internal`; "`run ze plugin bgp-rib` names bgp-rib and still forks".
- [ ] `internal/plugins/vrrp/register.go` `runVRRPEngine`, `internal/plugins/as112/register.go` `runAS112Plugin` - both refuse at run time on `!p.IsInternal()`; neither registration declares `RunsInProcessOnly`. AS112's comment: an operator may write the external block because "nothing in config validation forbids it".
- [ ] `internal/test/fixture/rib_import_policy_fixture.go` `ribImportCentralBogons`, `staticImportFilterDistance` and helpers - registered as `plugin/rib-import-central-bogons` and `plugin/static-import-filter-distance`. Both inject BGP routes with `request bgp rib inject`, rely on static blackhole routes from the config, require the control prefix selected before judging, and read `hidden`, `hidden-by` and `chain`. `candidateItem` decodes no `distance` and no `tag`, and neither driver reloads config.
- [ ] `internal/le/interoplab/bgp/check_import_policy.go` `checkOSPFImportPolicy`, `waitRIBCandidates` - the BIRD scenario proves that `ospf filter import` sets a distance and so changes selection. It hides nothing and configures no central chain.
- [ ] `test/plugin/doctor-plugin-external-unsupported.ci`, `test/parse/plugin-external-bad-name.ci` - the doctor test runs `ze doctor --json config <file>` over the test-only `fake-in-process-only` plugin; the parse test asserts `ze config validate` and `ze start` refuse alike.

**What a forked system RIB does today** (research, each traced to its producer; the first three rows confirmed by the reads above):

| Behavior | In the daemon | Forked |
|----------|---------------|--------|
| Sources it programs | every Loc-RIB source | BGP only |
| `rib filter import` and each source's chain | applied: a rejected path is hidden | ignored: a rejected BGP best is programmed |
| `show rib candidates` | lists paths | refuses with `errNoLocRIB` |
| `set distance`, `set tag` from a chain | applied | ignored |
| `rib { distance }` | published to the Loc-RIB | published in the forked process, read by nobody |
| Next-hop resolution and its cascade | resolved | none: an unresolved next hop is programmed raw |
| ECMP siblings | carried | absent |
| A filter reload | re-admits or hides through `Reselect` | reaches no consumer |

No test runs the system RIB forked, no page offers it, and internal is the default on every autoload path. It is reached only by an operator writing the external block by hand. `docs/architecture/system-architecture.md` section 2 shows `external rib { run "ze rib"; }` as an example.

**Behavior to preserve:**
- The system RIB in the daemon: selection, FIB events, `fib-withhold`, `show rib`, `show rib candidates`, the next-hop cascade.
- `ribevents.BestChange` itself and every other reader of it (the flow exporter, BMP, any third-party plugin subscribed to `bgp-rib`).
- `ze doctor` reporting `doctor-plugin-external-unsupported` for an external block that runs an in-process-only plugin, with the same message and fix, including when the commit now refuses the same config.
- Forked producers (BGP RIB, OSPF, IS-IS, static): they keep the `route-install` path and are not affected.
- An external block whose `run` is `ze.<plugin>`, or which sets `use`: it runs in the daemon and stays accepted.
- The two existing interop scenarios' checks and the umbrella's unit tests.

**Behavior to change:**
- A `plugin external` block whose `run` forks a plugin declaring `RunsInProcessOnly` is refused at commit.
- The system RIB, VRRP and AS112 declare `RunsInProcessOnly`.
- The system RIB's best-change input from the BGP RIB, and the arbitration that exists only for it, are deleted.

## Existing Mechanisms (MANDATORY)

Verified at HEAD on 2026-11-12.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| In-process-only declaration | `internal/component/plugin/registry/registry.go` `Registration.RunsInProcessOnly` (`docs/plugin-overview.md`, "Plugin modes") | A plugin states it works only inside the daemon; only the test plugin `fake-in-process-only` sets it | Extends: the system RIB, VRRP and AS112 set it. No new field and no list of names |
| Doctor report of an in-process-only plugin run externally | `internal/component/plugin/doctor/check_external.go` `checkExternalInProcess` (`docs/architecture/doctor-and-health-checks.md`) | Reports the block with the fix, by reading the field | Extends: stays the diagnostic, and must still answer on a config the commit now refuses, the way `checkExternalPluginNames` reads the parsed tree |
| Commit-time validators on the plugin block | `internal/component/config/validators.go` `ExternalPluginNameValidator`, `InternalPluginNameValidator`; `ze:validate` in `ze-plugin-conf.yang`; proof `test/parse/plugin-external-bad-name.ci` | Refuse a bad external name and an unregistered internal plugin, in `ze config validate` and at start | Extends: one more validator, on the external `run` leaf, reading `RunsInProcessOnly` of the registration the run line forks |
| Run-line resolution | `internal/component/plugin/resolve.go` `RegistryNames`, `ResolvePlugin`; `internal/component/config/loader.go` `MarkInternalPlugin` | Say which registry row a run line names and whether it forks | Extends: the validator and the doctor check ask the same two questions through the same functions |
| Run-time guard against a forked run | `internal/plugins/vrrp/register.go` `runVRRPEngine`, `internal/plugins/as112/register.go` `runAS112Plugin` (`!p.IsInternal()`) | The engine logs an operator error and exits when started forked | Extends: the system RIB gains the same guard as defence in depth; the commit refusal is the gate (forwarding door: a run-time refusal "is defence in depth, not the gate") |
| Loc-RIB import gate | `internal/core/rib/locrib/policy.go` `admit`, called from `manager.go` `(*RIB).insert` | Judges every source's insert, forked producers included | Does not apply: the gate is untouched. This spec removes the one consumer input that did not come from it |
| Forked route install | `internal/core/rib/routeinstall` sink, `internal/component/plugin/server/dispatch_route.go` `applyRouteInstall` (`docs/architecture/forked-route-install.md`) | Carries a forked producer's paths into the daemon's Loc-RIB | Does not apply: producers already pass the gate. The page's refusal of a remote Loc-RIB is the precedent for refusing a forked consumer |
| System RIB's Loc-RIB feed | `internal/component/sysrib/locrib_feed.go`, `sysrib.go` `processLocRIBChange`, `changeToBatch` | Turns each Loc-RIB change into one authoritative best per prefix | Extends: becomes the only input |
| System RIB's own distance arbitration | `sysrib.go` `effectivePriority`, `reapplyAdminDistances`, `rankedByLocRIB`, the unmarked branch of `processEvent` | Ranks per-protocol entries that arrived outside the Loc-RIB | Replaces: deleted with the input it serves; the Loc-RIB is the one arbiter (no layering) |
| Fixture drivers for the import tests | `internal/test/fixture/rib_import_policy_fixture.go`, `register_rib_import_policy.go` | Drive BGP injection, read `show rib` and `show rib candidates`, require hidden paths beside a selected control | Extends: the two `.ci` files call them; the drivers gain the distance and tag observations and the reload step, and a third driver covers connected |
| Import-policy interop checks | `internal/le/interoplab/bgp/check_import_policy.go` `checkOSPFImportPolicy`, `waitRIBCandidates`; scenario `test/interop/scenarios/ospf-import-policy-bird` | Prove against BIRD that OSPF's own import chain sets a distance and changes selection | Extends: one more assertion on that scenario proves the central chain hides an OSPF route |
| Feature declarations | `features/<id>.md`, `internal/le/feature/declaration.go` (`docs/contributing/feature-maturity.md`) | One file per feature renders `docs/features.md` | Extends: one new declaration; `features/static-routes.md` is the model |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `plugin { external <name> { run "ze plugin <plugin>"; } }` reaches the commit validators; `policy { ... }` and `filter { import { ... } }` under `rib`, `static` and `connected` reach the policy registry.
- Routes: a producer's insert into the daemon's Loc-RIB.

### Transformation Path
1. Commit: the external block's `run` value is resolved to the registration it forks. A registration that declares `RunsInProcessOnly` refuses the commit with the block, the plugin and the fix.
2. Start: the system RIB runs in the daemon and holds the Loc-RIB. A system RIB started without one refuses to run.
3. Insert: `(*RIB).insert` judges the path through `admit`; a rejected path is stored hidden.
4. Selection: the Loc-RIB dispatches the change; the system RIB's feed turns it into one best per prefix and publishes the system best-change the FIB plugins read.
5. Observation: `show rib` lists selected prefixes; `show rib candidates prefix <p>` lists every path with `hidden`, `hidden-by`, `chain` and `distance`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config to plugin registry | the commit validator reads `Registration.RunsInProcessOnly` of the registration an external `run` forks | No |
| Loc-RIB to system RIB | in-process `OnChange` only | No |
| BGP RIB to system RIB | none after this spec: the BGP RIB reaches the system RIB only through the Loc-RIB | No |

### Changed-Wire Census

No wire format changes. One reader of one event is removed, and one writer of its replay request with it. The census lists that event's whole population so the removal is judged against every other party.

**Search evidence:** scope `internal/` non-test Go, 2026-11-12. Terms `ribevents.BestChange.Subscribe`, `ribevents.ReplayRequest.Emit`, `ribevents.BestChange.Emit`, `FromLocRIB`. Positive control: the search finds the system RIB's own subscription in `sysrib.go` `run`, which was read at its range. `FromLocRIB` is `json:"-"` (`ribevents.go`, read), so it never crosses a process boundary and removing it changes no payload. Findings: subscribers are the system RIB, the flow exporter and BMP; replay-request writers are the system RIB, BMP and the FlowSpec firewall; emitters are two sites in `rib_bestchange.go`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `(bgp-rib, best-change)` event | writer | `internal/component/bgp/plugins/rib/rib_bestchange.go` `publishBestChanges` and the replay emit in the same file | `ribevents.BestChange.Emit` at both sites; `publishBestChanges` read | Preserve: unchanged |
| `(bgp-rib, best-change)` event | reader | `internal/component/sysrib/sysrib.go` `run`, the branch taken without a Loc-RIB | `ribevents.BestChange.Subscribe` in `run`, read | Change: deleted. The system RIB reads BGP's best from the Loc-RIB only |
| `(bgp-rib, best-change)` event | reader | `internal/plugins/flowexport/enrichbgp.go` (`applyBatch`) | `ribevents.BestChange.Subscribe` | Preserve: unchanged |
| `(bgp-rib, best-change)` event | reader | `internal/component/bgp/plugins/bmp/bmp_locrib.go` (`handleBestChange`) | `ribevents.BestChange.Subscribe` | Preserve: unchanged |
| `(bgp-rib, best-change)` event | reader | a third-party plugin subscribed to namespace `bgp-rib` (outside the repository) | `docs/architecture/api/process-protocol.md`, subscription namespace | Preserve: the JSON payload is untouched |
| `(bgp-rib, replay-request)` event | writer | `internal/component/sysrib/sysrib.go` `run`, same branch | `ribevents.ReplayRequest.Emit` in `run`, read | Change: deleted with the subscription |
| `(bgp-rib, replay-request)` event | writer | `internal/component/bgp/plugins/bmp/bmp_locrib.go`, `internal/plugins/flowspec-firewall/engine.go` | `ribevents.ReplayRequest.Emit` | Preserve: unchanged |
| `(bgp-rib, replay-request)` event | reader | the BGP RIB's replay handler in `internal/component/bgp/plugins/rib/` | the replay emit site in `rib_bestchange.go` answers it | Preserve: unchanged |

### Integration Points
- `internal/component/config` commit validators - gain the in-process-only refusal beside the two plugin-name validators.
- `internal/component/sysrib/register.go` registration - declares `RunsInProcessOnly`; `runSysRIBPlugin` refuses a run without a Loc-RIB.
- `internal/component/plugin/doctor` - the in-process check must answer on a config whose commit is refused.
- `internal/test/fixture` - the two registered drivers, plus one for the connected chain.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | after the deletion the system RIB has one input, the Loc-RIB, whose every insert passed `admit` |
| No unintended coupling (components stay isolated) | Yes | the validator reads the plugin registry, which `validators.go` already does for `InternalPluginNameValidator`; it names no plugin |
| No duplicated functionality (extends existing, does not recreate) | Yes | the declaration, the doctor check and the commit validators all exist; the system RIB's second arbiter is removed, not joined by a third |
| Zero-copy preserved where applicable (refs, not copies) | Yes | no hot path is touched; the Loc-RIB feed is unchanged |
| Registration over hardcoding, outbound | Yes | three plugins set a registration field; the refusal and the report read the field |
| Registration over hardcoding, inbound | Yes | searched for a list of in-process-only plugin names: `check_external.go` reads the field, `docs/plugin-overview.md` states the rule, no list exists and none is added. The run-time guards in VRRP and AS112 are per-plugin and stay in their own packages |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No shipped configuration, test or interop scenario runs the system RIB forked | research: no `.ci` runs it external, every `s.run` unit calls `SetLocRIB` first, autoload is internal | a test or lab config is refused after the change | grep `test/`, `docs/` and the interop scenarios for `plugin rib` in an `external` block before the first edit | unvalidated |
| A-2 | The only subscriber the deletion removes is the system RIB; no other consumer depends on the system RIB reading `ribevents.BestChange` | Changed-Wire Census | a consumer loses its feed | the census greps, rerun at implementation | unvalidated |
| A-3 | A validator on the external `run` leaf is walked at commit, as the one on `name` is | `test/parse/plugin-external-bad-name.ci`; config door trap "A `ze:validate` can be declared and never run" | the refusal is declared and never runs | the parse test asserts both `ze config validate` and start refuse | unvalidated |
| A-4 | `ze doctor --json config <file>` can still report the block when the commit refuses it | `checkExternalPluginNames` already reads the parsed tree for that reason | doctor goes silent on the exact config it exists to explain | `test/plugin/doctor-plugin-external-unsupported.ci` stays green unchanged; if it goes red the in-process check moves to the parsed tree | unvalidated |
| A-5 | VRRP and AS112 are in-process only by their own code, so declaring it refuses no working configuration | both engines exit on `!p.IsInternal()` (read) | a working forked VRRP or AS112 is refused | no test runs either forked; grep before the edit | unvalidated |
| A-6 | A static route and a connected route can be offered for a test prefix in one Linux daemon without a neighbour | `test/static/static-distance-beats-ebgp.ci`, `test/plugin/connected-distance-arbitration.ci` | the connected test needs a different fixture | the draft run | unvalidated |
| A-7 | A daemon test can change the config and reload between two observations | the runner's reload suite (`test/reload/`) | the re-admit criterion needs a second command step in the `.ci` | the draft run | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The refusal lands where `ze config validate` never looks, so validate accepts what start refuses | the parse test passes on start and fails on validate | the test asserts both commands, as `plugin-external-bad-name.ci` does |
| R-2 | Deleting the arbitration turns about 62 `processEvent` unit tests into tests of nothing, or tempts their deletion | tests fail to compile or pass vacuously | each test is re-pointed at the Loc-RIB path with the same assertion; a test whose only subject was the deleted arbitration is retired through `ze-test-judge`, with its `Weakening-approved:` line |
| R-3 | A daemon test passes because the route never arrived, not because the gate hid it | green with `admit`'s reject ignored | each test first requires the path to exist in `show rib candidates` and the control selected, then requires `hidden`; each is run green, red with the reject ignored, green |
| R-4 | The connected or static test is green on a host where the install is refused | no capability marker | `option=needs-linux:caps=net-admin`, as the neighbouring tests carry (forwarding door, Traps) |
| R-5 | The tests are written as drafts and never promoted, as in the umbrella | the `.ci` exists only under `test/draft/` | the Deliverables check is `git ls-files` for each promoted path; closure refuses without it |
| R-6 | The validator refuses the `ze.<plugin>` form or a `use` block, which run in the daemon | an internal-by-run config stops loading | the unit test accepts both forms for an in-process-only plugin |
| R-7 | Another session edits `sysrib.go` or its tests during the deletion | `git status` shows foreign hunks | commit own hunks alone with `hunks <path> <patch-file>`; never take foreign hunks out of the tree |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A config that forks the system RIB, VRRP or AS112 no longer commits. A wrong deletion in the system RIB would stop FIB programming for every route, which the existing system RIB units and `test/plugin/fib-sysrib.ci` catch |
| How is it reverted? | Each phase is its own commit and reverts alone. No config migration: the refused configuration never worked |
| Who else touches this path? | `spec-route-policy-0-umbrella` (closing; it only cites this spec). `plan/release/fixes/spec-single-owner-reload-reconcile.md` touches reload ordering, not the system RIB's input |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| config `plugin { external rib-out { run "ze plugin rib"; } }`, `ze config validate` and start | → | commit validator reading `RunsInProcessOnly` | `test/parse/plugin-external-in-process-only-refused.ci` |
| the same shape of config, `ze doctor --json config` | → | `checkExternalInProcess` | `test/plugin/doctor-plugin-external-unsupported.ci` (existing, unchanged) |
| config `rib { filter { import { filters [ NO-BOGONS ]; final accept; } } }` | → | `admit` central chain, `showRIBCandidates` | `test/plugin/rib-import-central-bogons.ci` |
| config `static { filter { import { ... } } }` | → | `admit` source chain for static | `test/plugin/static-import-filter-distance.ci` |
| config `connected { filter { import { ... } } }` | → | `admit` source chain for connected | `test/plugin/connected-import-filter-hides.ci` |
| the system RIB's engine started with no Loc-RIB | → | `runSysRIBPlugin` refusal | `TestSysRIBRefusesToRunWithoutLocRIB` in `internal/component/sysrib/` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A config with `plugin { external <name> { run "ze plugin rib"; } }` | `ze config validate` and start both refuse it, with one message that names the block, says the system RIB works only inside the daemon and why, and gives the internal block to write instead | A third-party binary, a plugin that does not declare the field, and the `ze.<plugin>` and `use` forms still load: the refusal reads the declaration and the transport only |
| AC-2 | The same config given to `ze doctor --json config` | doctor reports `doctor-plugin-external-unsupported` with the block, the plugin and the fix, exactly as it does today for the test plugin | None: doctor must explain every config the commit refuses |
| AC-3 | An external block whose `run` forks VRRP or AS112 | refused at commit with the same message shape, and reported by doctor | Their run-time guards stay as defence in depth |
| AC-4 | The system RIB in the daemon, a BGP best offered for a prefix the central chain rejects, and a second BGP best the chain accepts | the accepted prefix reaches `show rib` and the system best-change the FIB plugins read; the rejected prefix's path is listed hidden by `show rib candidates`, and a `ribevents.BestChange` for it leaves the system RIB's table as it was | The BGP RIB still publishes its own best-change for the rejected prefix: the flow exporter and BMP report what BGP selected, which may differ from what the box installed |
| AC-5 | The system RIB's engine started in a process that holds no Loc-RIB | it exits with an operator error that names the cause and the internal block to write, and the plugin start fails | None: there is no input left for it to follow |
| AC-6 | `rib filter import` rejects 192.0.2.0/24; BGP and static each offer it; a control prefix is offered the same way | for the control prefix the expected source is selected and `show rib` lists it; for 192.0.2.0/24 each path shows `hidden: true`, `hidden-by: central`, `chain: rib import` | The control prefix is installed: it proves the routes arrived and the chain is selective |
| AC-7 | The central chain rejects one prefix BIRD advertises over OSPF in `ospf-import-policy-bird` | Ze's OSPF path for it shows `hidden: true`, `hidden-by: central`, `chain: rib import`, while the scenario's two existing prefixes keep the distances and winners they have today | The prefix stays in the OSPF database Ze holds and floods: an import filter acts after SPF and leaves the LSDB alone |
| AC-8 | `static filter import` rejects a static route for a prefix BGP also offers | the BGP path is selected; the static path shows `hidden: true`, `hidden-by: source`, `chain: static import` | The BGP path is installed |
| AC-9 | AC-8, then the reject is removed from the chain and the config reloaded | `show rib` lists the prefix with static as its source, and `show rib candidates` shows the static path selected and unhidden at static's distance | The BGP path remains as an unselected candidate. The static route's own config and the BGP session stay as they were: the re-admit comes from the Loc-RIB's stored path |
| AC-10 | A `modify` filter on `static filter import` sets a distance | `show rib candidates` shows the static path at that distance, and the winner against BGP follows it | Static routes the filter does not match keep the declared distance |
| AC-11 | A static route with `tag 4242`, static running in the daemon, and `rib filter import` with a `match` on tag 4242 that rejects | the tagged route's path shows `tag: 4242`, `hidden: true`, `hidden-by: central`; an untagged static route for another prefix is selected | The forked-static leg stays proven by `TestRouteInstallTagReachesGate`, which drives the daemon's real `route-install` handler with the compiled chain; no daemon test forks static today |
| AC-12 | `connected filter import` rejects one connected prefix of an interface that carries two | the other prefix is selected from connected and listed by `show rib`; the rejected one's path shows `hidden: true`, `hidden-by: source`, `chain: connected import` | The kernel's own connected route for the rejected prefix remains: Ze does not own it |
| AC-13 | Each of the three daemon tests with `admit`'s reject ignored | the test fails on its `hidden` assertion, and passes again once restored | None: this is the red proof |
| AC-14 | `./le feature report` | lists a route policy feature at `experimental` with its components, its real-path tests and its docs | The level stays below `supported` until recorded green runs and reviews exist |
| AC-15 | `docs/comparison.md` | carries a row for per-protocol import and export policy with a paragraph and source anchors, and its redistribution example shows `filters` and `final` | Cells for daemons whose source was not read say `Unclear`, and stay so until the source is read |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes a config that forks the system RIB | commit → `run` validator → registry field → refusal with the fix | `test/parse/plugin-external-in-process-only-refused.ci` |
| 2 | Blocks a bogon from every protocol | config → central chain → gate → hidden paths → `show rib candidates` | `test/plugin/rib-import-central-bogons.ci` |
| 3 | Keeps a static route out of the RIB, then lets it back in | config → static chain → hidden → reload → `Reselect` → selected | `test/plugin/static-import-filter-distance.ci` |
| 4 | Keeps one connected prefix out of the RIB | config → connected chain → hidden | `test/plugin/connected-import-filter-hides.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExternalRunOfInProcessOnlyPluginRefused` | `internal/component/config/` (beside the plugin-name validators' tests) | AC-1, AC-3, R-6: refuses a forked run by the field; accepts a plugin without it, a third-party binary, the `ze.<plugin>` form | |
| `TestSysRIBRegistrationDeclaresInProcessOnly` | `internal/component/sysrib/` | the registration sets the field | |
| `TestSysRIBRefusesToRunWithoutLocRIB` | `internal/component/sysrib/` | AC-5 | |
| `TestSysRIBIgnoresBGPBestChangeEvent` | `internal/component/sysrib/` | AC-4: a `ribevents.BestChange` emitted on the bus leaves the system RIB unchanged, while a Loc-RIB change updates it | |
| `TestEveryRuntimeInProcessGuardDeclaresTheField` | `internal/component/plugin/all/` | AC-3: VRRP and AS112 declare the field | |
| existing `processEvent` units | `internal/component/sysrib/*_test.go` | re-pointed at the Loc-RIB path with unchanged assertions (R-2) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N/A: the spec adds no numeric input | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `plugin-external-in-process-only-refused` | `test/parse/plugin-external-in-process-only-refused.ci` | AC-1: validate and start both refuse the forked system RIB | |
| `doctor-plugin-external-unsupported` | `test/plugin/doctor-plugin-external-unsupported.ci` (existing) | AC-2 | |
| `rib-import-central-bogons` | `test/plugin/rib-import-central-bogons.ci` | AC-6, AC-11 | |
| `static-import-filter-distance` | `test/plugin/static-import-filter-distance.ci` | AC-8, AC-9, AC-10 | |
| `connected-import-filter-hides` | `test/plugin/connected-import-filter-hides.ci` | AC-12 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-import-policy-bird` | `test/interop/scenarios/` | BIRD 3 | AC-7: the central chain hides an OSPF route; red with the central reject removed and the image rebuilt | |

## Files to Modify
- `internal/component/sysrib/register.go` - declare `RunsInProcessOnly`; refuse to run without a Loc-RIB; correct the comment in `runSysRIBPlugin`
- `internal/component/sysrib/sysrib.go` - delete the best-change subscription, its replay request, the unmarked branch of `processEvent` and the arbitration that serves only it; correct the comments on `run` and `processEvent`
- `internal/core/bgp/ribevents/ribevents.go` - remove `FromLocRIB` once no reader is left, and correct the `ReplayRequest` comment that names the system RIB as an emitter
- `internal/core/rib/locrib/default.go` - the comment that names the system RIB's subprocess transport
- `internal/component/config/validators.go`, `validators_register.go` - the in-process-only validator
- `internal/component/plugin/yang/ze-plugin-conf.yang` - `ze:validate` on the external `run` leaf, and its description
- `internal/component/plugin/registry/registry.go` - the `RunsInProcessOnly` comment: the commit refuses, doctor reports
- `internal/component/plugin/doctor/check_external.go` - only if A-4 fails: read the parsed tree
- `internal/plugins/vrrp/register.go`, `internal/plugins/as112/register.go` - declare the field; AS112's "nothing in config validation forbids it" comment
- `internal/test/fixture/rib_import_policy_fixture.go`, `register_rib_import_policy.go` - distance and tag observations, the reload step, the connected driver
- `internal/le/interoplab/bgp/check_import_policy.go` - the central-chain assertion for the OSPF scenario
- `test/interop/scenarios/ospf-import-policy-bird/ze.conf`, `bird.conf`, `README.md` - the third prefix, the central chain, and what it proves
- `ai/doors/system.md`, `ai/doors/forwarding.md` - name the declared exception and the system RIB as a user of it
- `docs/plugin-overview.md` - "No shipped plugin sets the field yet" is replaced by the rule and the commit refusal
- `docs/architecture/doctor-and-health-checks.md` - the commit refusal beside the doctor check
- `docs/architecture/system-architecture.md` - the `external rib` example
- `docs/architecture/forked-route-install.md` - producers fork, the system RIB does not
- `docs/architecture/rib/unified-locrib.md` - the system RIB has one input
- `docs/architecture/core-design.md` - section 16's FIB pipeline diagram draws `bgp-rib/best-change/bgp` feeding the system RIB; the system RIB section gains the one-input rule (declared by `internal/component/sysrib/sysrib.go`)
- `docs/architecture/config/yang-config-design.md` - the custom validators it lists gain the in-process-only one (declared by `internal/component/config/validators.go`)
- `docs/architecture/testing/interop.md` - the `ospf-import-policy-bird` paragraph and its scenario table row gain the central-chain assertion (declared by `internal/le/interoplab/bgp/check_import_policy.go`)
- `docs/architecture/vrrp/vrrp-first-hop-redundancy.md` - states that VRRP runs only in the daemon and that the commit refuses a forked run; the page says neither today (declared by `internal/plugins/vrrp/register.go`)
- `docs/architecture/dns/as112.md` - the same statement for AS112 (declared by `internal/plugins/as112/register.go`)
- `docs/guide/plugins.md`, `docs/guide/route-policy.md` - the refusal an operator meets; the tests as evidence
- `docs/guide/static-routes.md` - a named-table route is outside `static filter import` (round 4 note)
- `docs/comparison.md` - the new row and paragraph; the stale redistribution example
- `docs/functional-tests.md` - the new tests

## Files to Create
- `test/parse/plugin-external-in-process-only-refused.ci`
- `test/plugin/rib-import-central-bogons.ci`
- `test/plugin/static-import-filter-distance.ci`
- `test/plugin/connected-import-filter-hides.ci`
- `features/route-policy.md`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/plugin/yang/ze-plugin-conf.yang`: one `ze:validate`, no new node |
| YANG validation constraints | N-A | no new leaf |
| YANG custom validators | Yes | the in-process-only validator, registered beside `external-plugin-name` |
| CLI commands/flags | N-A | no command is added |
| CLI grammar (keyword before value) | N-A | no command is added |
| Editor autocomplete | N-A | no new value |
| Functional test for new RPC/API | Yes | the four `.ci` files above; `show rib candidates` is exercised by three of them |
| Pipe completeness | N-A | no new output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | existing `plugin-external-in-process`; no new dependency |
| Doctor code per problem class | Yes | the commit refusal and the run-time refusal both name `doctor-plugin-external-unsupported` |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/route-policy.md` (new) |
| 2 | Config syntax changed? | No | the grammar is unchanged; one accepted config is now refused, documented under rows 5 and 6 |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, `docs/plugin-overview.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/route-policy.md`, `docs/guide/static-routes.md` |
| 7 | Wire format changed? | No | Changed-Wire Census: no payload changes |
| 8 | Plugin SDK/protocol changed? | No | no `pkg/` change |
| 9 | RFC behavior implemented, changed, or newly proven? | No | local policy, no RFC |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` |
| 12 | Internal architecture changed? | Yes | `docs/architecture/forked-route-install.md`, `docs/architecture/rib/unified-locrib.md`, `docs/architecture/system-architecture.md`, `docs/architecture/doctor-and-health-checks.md`, `ai/doors/system.md`, `ai/doors/forwarding.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | the system RIB no longer subscribes to `bgp-rib` events: `docs/plugin-overview.md`, `docs/guide/status.md` where either lists the subscription |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | The pages the changed files declare, each named in Files to Modify: `docs/architecture/core-design.md`, `docs/architecture/config/yang-config-design.md`, `docs/architecture/testing/interop.md`, `docs/architecture/vrrp/vrrp-first-hop-redundancy.md`, `docs/architecture/dns/as112.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/api/process-protocol.md` (unaffected: the `bgp-rib` event payload does not change). `docs/architecture/api/architecture.md`, declared by `registry.go`, is unaffected: only the comment on `RunsInProcessOnly` changes and the page does not describe the field (grep 2026-11-12). The advisory anchors are derived at implementation with `./le spec citation anchors spec plan/release/fixes/spec-route-policy-1-gate-proof-and-forked-sysrib.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/system-architecture.md` (`external rib`), `docs/comparison.md` (redistribution example) |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the refusal an operator meets
   - Tests: `TestExternalRunOfInProcessOnlyPluginRefused`, `test/parse/plugin-external-in-process-only-refused.ci`, `TestSysRIBRegistrationDeclaresInProcessOnly`
   - Files: the validator, its registration, the YANG `ze:validate`, the system RIB's registration
   - Verify: the `.ci` fails because the commit accepts the config, then passes; `test/plugin/doctor-plugin-external-unsupported.ci` stays green (A-4)
2. **Phase: sibling declarations** -- VRRP and AS112 declare the field
   - Tests: `TestEveryRuntimeInProcessGuardDeclaresTheField`
   - Files: the two registrations
   - Verify: red before, green after; their own package tests stay green
3. **Phase: delete the ungated input** -- the system RIB follows the Loc-RIB only
   - Tests: `TestSysRIBRefusesToRunWithoutLocRIB`, `TestSysRIBIgnoresBGPBestChangeEvent`, the re-pointed `processEvent` units
   - Files: `sysrib.go`, `register.go`, `ribevents.go`, `default.go`
   - Verify: the two new tests red then green; the system RIB package, `test/plugin/fib-sysrib.ci` and `test/plugin/forked-route-install.ci` green
4. **Phase: the three daemon tests** -- through `/ze-test`, drafted, proven under load, promoted
   - Tests: the three `test/plugin/` files; AC-13's red proof for each
   - Files: the fixture drivers and registration, the `.ci` files
   - Verify: each green on Ze's kernel, red with `admit`'s reject ignored, green; each path present in `git ls-files`
5. **Phase: the OSPF leg** -- the central chain on the interop scenario
   - Tests: `ospf-import-policy-bird`
   - Files: the scenario's configs and README, the checker
   - Verify: green, red with the central reject removed and the image rebuilt, green
6. **Phase: documentation** -- declaration, comparison, pages and doors
   - Tests: `./le feature report`, `./le doc check all`
   - Files: every page in Files to Modify, `features/route-policy.md`
   - Verify: the report lists the feature; every new claim carries a source anchor

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The refusal reads the registration field and names no plugin; `ze config validate` and start give the same answer; the system RIB has exactly one route input |
| Naming | The refusal names the block as the operator wrote it and the plugin as the registry holds it, as the doctor row does |
| Data flow | No reader of `ribevents.BestChange` other than the system RIB changed |
| Operator errors and doctor codes | The commit refusal and the run-time refusal are operator errors with the fix on the last line and the code `doctor-plugin-external-unsupported` |
| Rule: no layering | `effectivePriority`, `reapplyAdminDistances`, `rankedByLocRIB` and `FromLocRIB` are gone, not kept beside the Loc-RIB path |
| Rule: testing | No `processEvent` test is deleted or weakened without a `ze-test-judge` verdict |
| Rule: vacuity | Each daemon test requires the path to exist and the control selected before it requires the judged path hidden |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The forked system RIB is refused at commit | `./le test functional parse` on `plugin-external-in-process-only-refused` |
| The ungated input is gone | `grep -n 'ribevents.BestChange.Subscribe' internal/component/sysrib/` returns nothing |
| Three plugins declare the field | `grep -rn 'RunsInProcessOnly: *true' internal/component/sysrib internal/plugins/vrrp internal/plugins/as112` |
| The three daemon tests are tracked | `git ls-files test/plugin/rib-import-central-bogons.ci test/plugin/static-import-filter-distance.ci test/plugin/connected-import-filter-hides.ci` lists all three |
| The red proof | the pasted red and green runs of each test with `admit`'s reject ignored |
| The OSPF leg | the interop run record, green, red, green |
| The declaration | `./le feature report` lists route policy |
| The comparison row | `grep -n 'import and export policy' docs/comparison.md` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Fail-open | No path remains by which a route the central chain rejected reaches the FIB plugins; the system RIB without a Loc-RIB stops, it does not run permissive |
| Refusal bypass | An external block that reaches the system RIB by another spelling of `run` (a path to the `ze` binary, extra arguments) resolves to the same registration and is refused; a spelling the resolver cannot map is covered by the run-time guard |
| Error leakage | The refusal names the block and the plugin, nothing from the environment |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| A-1 fails: something shipped runs the system RIB forked | STOP. Report what runs it; the design choice returns to the owner |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- A consumer with a fallback input is a second gate nobody wrote. The fallback was added so a forked system RIB would do something rather than nothing, and "something" became every BGP best with none of the policy.
- `errNoLocRIB` already said the system RIB must run in the daemon. The code knew; the commit did not refuse.
- A test drafted under an ignored directory is not a deliverable until it is promoted. The Deliverables check for a `.ci` is `git ls-files`, not the test's name in a table.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions

Status of this section: proposed. The owner approves the design before the spec moves to `ready`.

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The system RIB runs only in the daemon: it declares `RunsInProcessOnly`, the commit refuses a config that forks it, and its ungated input is deleted | (b) Keep a forked system RIB and feed it the Loc-RIB's post-gate changes over a new engine-to-plugin stream. (c) Keep the event input and judge it in the forked process with a copy of the chains | (b) needs a new published contract under `pkg/plugin/rpc`, an RPC fed from under the Loc-RIB write lock, and still leaves next-hop resolution, `rib { distance }`, ECMP and `show rib candidates` broken in the forked process; `forked-route-install.md` already refused a remote Loc-RIB. (c) is a second gate, against the owner's decision of one gate at the Loc-RIB insert, and it cannot hide and re-admit paths it never stored. The chosen design uses mechanisms that exist and deletes one that should not |
| The refusal is at commit, read from the registration field, for every plugin that declares it | A run-time refusal only, as VRRP and AS112 do today; a refusal that names the system RIB | A run-time refusal stops the startup tier after the config was accepted, which is the validate-accepts-what-start-refuses defect. Naming a plugin in config code is a central enumeration. The plugin-name validators are the existing pattern |
| VRRP and AS112 declare the field in the same work | Leave them with their run-time guards | The refusal and the doctor report both read the field. Two plugins that are in-process only by their own code and do not say so would be accepted at commit and unreported by doctor, which is the defect this spec fixes for the system RIB |
| The daemon tests use BGP and static as sources; OSPF's leg of the central chain goes on the existing BIRD scenario | Put OSPF in the daemon test with a netns neighbour; drop the OSPF leg and rely on the unit `TestLocRIBCentralChainAllSources` | The umbrella's AC-7 names OSPF, so the leg is owed. A neighbour in a `.ci` adds a second daemon and its timing to a test whose question is the central chain. The scenario already brings up BIRD and already reads `show rib candidates` |
| A third daemon test covers `connected filter import` | Count a connected source under the central chain as coverage | The review named connected among the unproven chains. A source passing the central chain does not prove its own attachment is wired |
| AC-11's forked-static leg stays on the unit test | Add a daemon test that forks static | `TestRouteInstallTagReachesGate` drives the daemon's real `route-install` handler with the compiled chain, which is the forked path's entry point on the daemon side. The review asked for the in-process case in a daemon test, and no daemon test forks static today |
| One spec for the three findings, in `plan/release/fixes/` | Three specs across fixes, validation and post-release | The owner asked for one spec. The fix is a release obligation, and the tests and rows prove and declare the same shipped gate |

## Known Limitations

- A third-party route consumer cannot follow the Loc-RIB's post-gate stream from another process. Nothing in the repository needs it. If one is wanted, it is a feature with a published contract and its own spec, not a repair of this one.
- Forked graceful-restart retention stays unsupported, as `docs/architecture/forked-route-install.md` records. This spec does not touch it.
- The feature declaration claims `experimental`. Reaching `supported` needs recorded green runs and the reviews `docs/contributing/feature-maturity.md` lists, which are not part of this spec.

## RFC Documentation (Scope: protocol)

None: no RFC requirement is enforced or changed by this spec.

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
- [ ] AC-1..AC-15 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
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
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)

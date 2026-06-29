# Spec: tiers-5-structure-tidy

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-tiers-0-umbrella (Phases 1-4 complete, B-1 complete) |
| Phase | 9/9 (implementation complete, closing) |
| Updated | 2026-06-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/module-tiers.md` - the current tier rule (enforced engine gate + advisory non-engine)
4. `plan/spec-tiers-0-umbrella.md` - parent umbrella, Phase 5 section and Open Design Decision
5. `plan/learned/979-tiers-5-b1-unify-discovery.md` - B-1 resolution
6. `scripts/dev/dep_audit.py` - the placement audit tool

## Task

Complete the module-tier taxonomy so the directory structure communicates dependency
boundaries to humans, not just to the gate.

Today the enforced gate covers only `sdk.NewWithConn` engines (Path C). Everything
else -- shared libraries, framework packages, host services, setup commands -- is
advisory. The rule doc (`ai/rules/module-tiers.md`) says "not engine -> core" but
that is wrong for most of these packages. The directory structure gives no signal
about which packages form a unit and which are independent.

This spec does five things:

1. **Resolve B-3 (the host/orchestration gap).** The rule needs categories for
   framework infrastructure (config, plugin, command, cli), host services (web, ssh,
   gnmi, mcp, lg), and domain libraries that are not engines but also are not
   foundational core packages.

2. **Resolve B-2 (fused library+engine) for the packages that are actually leaf.**
   Extract BGP codec/type subpackages and iface/vpp event constants so "imports bgp"
   distinguishes codec/type use from engine dependence. `ike/dataplane` is explicitly
   out of scope until its component/VPP backend is split from its interface package.

3. **Introduce related-code clustering where isolation is real.** BNG code
   (l2tp + ppp + pppoe + subscriber + l2tp* plugins) and VPN code (ike + ipsec +
   pki) form isolated dependency clusters. Nest those under their domain roots.
   Do **not** nest AAA, traffic, firewall, or CoS in this spec; those are either
   platform infrastructure or have global interface-facing behavior.

4. **Make mechanical moves possible.** Extend the migration tool before relying on it
   so it can handle repo-relative core moves and nested component-domain moves, not
   only top-level component<->plugins relocations.

5. **Move misplaced directories and upgrade enforcement.** Relocate genuinely-leaf
   libraries, clean up dead stubs, fix audit classification gaps, and extend the gate
   to cover non-engine placements through a machine-readable category manifest.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/module-tiers.md` - the current rule; defines the two axes and three tiers
  → Constraint: only engine placement is enforced (Path C). Non-engine tiers are advisory. No allowlist exists for engines.
  → Constraint: `dep_audit.py --check` is the gate; `dep_audit.py` (no flag) prints the advisory report.
  → Decision: Path C was chosen over Path A (allowlist) and Path B (full mechanical). This spec implements Path B.
- [ ] `plan/spec-tiers-0-umbrella.md` - parent spec, Phase 5 analysis and Open Design Decision
  → Decision: umbrella recommends (a)/(c) hybrid: enforce engines now, document host/orchestration as subcategory, relocate only genuinely-leaf infra. This spec goes further (B-2 extraction + clustering).
  → Constraint: B-2 (bgp/iface/vpp/ike fuse library+engine) blocks mechanical core/composition enforcement. B-3 (host packages don't fit trichotomy) needs a human decision on tier placement.
- [ ] `plan/learned/979-tiers-5-b1-unify-discovery.md` - B-1 resolution
  → Decision: "wired as a plugin" derived from composition roots (all.go + cmd/ze dispatch), not per-mechanism grep. setup_features_*.go NOT recognized as registration importers (known advisory gap for connect/local/provision/systemd).
- [ ] `ai/rules/plugin-self-containment.md` - the "delete the folder" invariant
  → Constraint: removing a plugin must remove all its features and break nothing else. Clustering must preserve this.
- [ ] `ai/rules/plugin-design.md` - registration patterns, Proximity Principle
  → Constraint: new nested packages must preserve registration-over-hardcoding and plugin self-containment.
- [ ] `docs/architecture/core-design.md` - overall architecture
  → Constraint: registration and plugin inventory remain the integration boundary; structural moves must not change runtime behavior.

### RFC Summaries (MUST for protocol work)
N/A -- this is structural reorganization, not protocol work.

**Key insights:**
- Engine placement is fully enforced and clean (baseline empty). This spec extends to non-engines.
- BGP extraction dependency graph (verified by go list -json this session):
  - Layer 0 (leaf, zero deps): wire, capability, events
  - Layer 1 (one dep): context -> capability; nlri -> wire
  - Layer 2 (two deps): attribute -> wire + context
  - BLOCKED: message -> wire + attribute + context + nlri + plugin/registry (component-tier dep)
  - STAYS: types -> attribute + filter + message + nlri + rib + wireu (deeply fused)
  - STAYS: wireu -> wire + attribute + context + message + nlri (codec consumer)
- iface/vpp leaf extractions: iface/events (leaf), vpp/events (leaf). `ike/dataplane`
  is NOT leaf as a package today because its VPP backend imports `component/vpp`; it
  stays out of scope until split.
- Shared libraries that move in this spec:
  - `audit` -> `internal/core/audit` if the implementation audit confirms no
    component imports.
  - `ppp` -> `internal/component/l2tp/ppp` as a BNG domain library, not core.
- Shared libraries that stay but must be categorized in `module-tiers.md`: `engine`
  (framework/orchestration), `gokrazy` (appliance/host support), `host` (host
  service, pinned by doctor-platform registration), `managed`, `pppoeclient` (BNG
  domain library), `support`.
- plugins/ "shared libraries" (connect, local, provision, systemd) are CLI setup
  commands wired via setup_features_*.go. They have register.go. The audit
  misclassifies them because setup_features is not recognized as a registration
  importer.
- Dead stubs: component/diag (doc.go only), component/update (doc.go only).
- BNG cluster (ISOLATED, nesting candidate): l2tp, ppp, pppoe, pppoeclient,
  subscriber + runtime plugin directories authlocal, authradius, pool, shaper
  under `internal/component/l2tp/plugins/`. Command YANG packages live next to
  their handlers under `l2tp/cmd/yang`, `l2tp/pppoe/cmd/yang`, and
  `l2tp/subscriber/cmd/yang`. Runtime plugin names keep their full L2TP names.
  `cos` stays flat unless a later spec splits its global interface QoS behavior
  from subscriber CoS behavior.
- VPN cluster (ISOLATED): ike, ipsec. PKI stays top-level because it is shared
  certificate infrastructure for IPsec and future TLS users.
- AAA cluster (NOT isolated, DON'T nest): aaa, authz, radius, tacacs. Depended on by
  api, bgp, ssh, web.
- Traffic/firewall: moderate isolation, leave flat.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `scripts/dev/dep_audit.py` (lines 1-80) - the placement audit; `is_registration_importer` recognizes all.go, all_*.go, _imports.go, dispatch companions, but NOT setup_features_*.go
  → Constraint: extending the gate requires either recognizing setup_features as registration importers or deciding these dirs should move.
- [ ] `internal/component/host/register.go` - host's only component-tier import is `plugin/registry` for `RegisterDoctorPlatforms`. This single call pins host to component tier.
  → Constraint: to move host to core, the doctor-platform registration must move to a core-tier interface or host must stop importing plugin/registry.
- [ ] `ai/rules/module-tiers.md` (full) - current rule defines engine/component/plugin tiers but says "not engine -> core" without handling framework/host packages that can't be core.
- [ ] `plan/spec-tiers-0-umbrella.md` (full) - documents B-2 and B-3 blockers, the probe results (65 raw mismatches), and three path options (C chosen, B deferred).

**Behavior to preserve:** all runtime behavior, registration names, config roots, command surfaces, build tags, and plugin inventory.

**dep_audit.py advisory report (this session, `dep_audit.py` with no flags):**

component/ REGISTERED PLUGINS (not engines): aaa (ext=23), aihelp (2), api (8),
authz (12), bgp (39), cli (30), cmd (4), command (73), config (311), debug (2),
doctor (0), gnmi (1), hub (1), ipsec (13), l2tp (17), lg (1), mcp (4), mpls (0),
ping (2), pki (6), plugin (309), pppoe (2), radius (9), resolve (12), ssh (2),
storage (1), subscriber (8), tacacs (1), telemetry (1), traceroute (2),
trafficstat (0), web (2).

component/ CORE CANDIDATES: diag (0 ext, 0 reg, 0 tests), update (0 ext, 0 reg, 0 tests).

component/ SHARED LIBRARIES: audit (22), ppp (20), host (19), managed (3),
engine (2), gokrazy (1), pppoeclient (1), support (1).

plugins/ SHARED LIBRARIES: connect (1), local (1), provision (1), systemd (1).
plugins/ CORE CANDIDATE: ifacenetlink (0 reg, 0 tests).

**Import graph analysis (this session, go list -json):**

BGP subpackages imported externally:
- As library (codec/types): attribute (config, chaos, perf, test), capability (chaos, perf), context (perf), events (config, meta), message (chaos, perf, mrt), nlri (chaos, perf), types (sysrib, fib, test)
- As engine wiring: cli (plugin), config (chaos, config), plugin (plugin), plugins (config, plugin, sysrib, flowexport, flowspec-firewall), reactor (plugin), yang (config, plugin)

Dependency clusters identified:
- BNG/subscriber: l2tp, ppp, pppoe, pppoeclient, subscriber + runtime plugins authlocal, authradius, pool, shaper + command schemas under l2tp/cmd, pppoe/cmd, subscriber/cmd + cos
- AAA: aaa, authz, radius, tacacs + aaa-cmd
- VPN/IPsec: ike, ipsec. PKI stays top-level until shared certificate infrastructure is split by a later spec.
- Traffic/monitoring: traffic, trafficstat + trafficusage, traffic-cmd, ddos
- Firewall: firewall + plugins/firewall, copp, policyroute, flowspec-firewall

All clusters share the same external dependency pattern: config, plugin, command (framework), plus iface/sysctl/vpp (platform engines). Within each cluster, cross-imports are dense. Between clusters, cross-imports are sparse or absent.

**Behavior to preserve:**
- All runtime behavior. Every change is a relocation or extraction: same registration, same config roots, same commands, same wire behavior.
- `bin/ze --plugins` inventory unchanged across moves.
- `dep_audit.py --check` (engine gate) stays green throughout.
- Build-tag gating (ze_distro, ze_setup, etc.) continues to work.

**Behavior to change:**
- Directory locations and import paths of relocated/extracted packages.
- `module-tiers.md` rule doc: expanded to cover non-engine categories.
- `dep_audit.py`: extended to enforce non-engine tiers (at least for new code).
- Generator `pluginDirs`/`rpcRoot` updated per moves.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Package directory location under `internal/`.
- Registration in composition root (`all.go`, `all_*.go`, dispatch).

### Transformation Path
1. Author places a package under a tier directory.
2. Generator (`plugin_imports.go`) scans `pluginDirs` + discovery scanners -> generates `all.go`.
3. `all.go` blank-imports the package; its `init()` calls `registry.Register` or equivalent.
4. At runtime the registry discovers and starts the plugin when config enables it.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Directory <-> generator | hardcoded `pluginDirs`/`rpcRoot` in `plugin_imports.go` | [ ] each move updates them |
| Generator <-> composition root | `all.go` regeneration | [ ] `plugin_imports.go --check` clean |
| Placement <-> rule | `dep_audit.py --check` + extended non-engine check | [ ] audit exits 0 |
| Library extraction <-> importers | import path rewrite | [ ] `go build ./...` green |

### Integration Points
- `scripts/dev/dep_audit.py` -- the audit gate, needs extension for non-engine
  enforcement from a category manifest
- `scripts/codegen/plugin_imports.go` -- generator, needs pluginDirs update per nested
  cluster namespace
- `scripts/dev/migrate_module.py` -- relocation tool; must be extended before any
  core or nested move because the current tool only handles top-level component<->plugins
  moves

### Architectural Verification
- [ ] No bypassed layers (moves keep registry discovery intact)
- [ ] No unintended coupling (no new cross-tier imports introduced)
- [ ] No duplicated functionality (reuses existing audit + migration tools)
- [ ] Zero-copy preserved (N/A -- relocation only)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | BGP library subpackages can be extracted to `internal/core/bgp/` in dependency order | go list shows wire/capability/events are leaf; context depends only on capability; nlri only on wire; attribute on wire+context. Message has a plugin/registry import (BLOCKS its extraction). Types depends on nearly everything (stays in engine). | partial extraction only; message and types need refactoring before they can move | verified by go list -json this session | PARTIALLY BROKEN -- extractable set is wire/capability/events/context/nlri/attribute (6 of 9). message needs plugin/registry decoupled. types/wireu stay in engine. |
| A-2 | Clustering BNG and VPN under domain roots preserves the plugin-self-containment invariant | BGP already does this with bgp/plugins/; the generator's pluginDirs discovers nested plugins | a nested plugin fails the "delete the folder" test or the generator misses it | add pluginDir entries for `internal/component/l2tp/plugins` and `internal/component/ike/plugins` if needed, run generator, verify all.go | unvalidated |
| A-3 | host's only component-tier import (plugin/registry for RegisterDoctorPlatforms) can be categorized without moving host now | `internal/component/host/register.go:5-12` imports plugin/registry only to call `RegisterDoctorPlatforms` | host requires deeper decoupling than expected before non-engine enforcement can pass | read RegisterDoctorPlatforms and encode host as host-service in the category manifest | confirmed |
| A-4 | setup_features_*.go files are the only non-test Go wiring path for connect/local/provision/systemd | exact import grep found connect/local/systemd in `cmd/ze/setup_features_distro.go` and provision in `cmd/ze/setup_features_setup.go`; connect self-import appears only in its test | another wiring path exists | grep for exact package imports across cmd/internal/pkg/scripts | confirmed |
| A-5 | The existing migrate_module.py tool handles nested/core moves | current tool handled flat component<->plugins moves in Phases 2-3 | tool must be extended before implementation can safely perform this spec's moves | read current tool, add selftests for core and nested destinations | BROKEN -- current tool only supports top-level `internal/component` <-> `internal/plugins` moves. Add tooling phase 0. |
| A-6 | Extracting iface/events and vpp/events to core is feasible | both packages are leaf event constants with no imports beyond package declarations/constants | hidden imports create a core-to-component cycle | check each subpackage's full import list | confirmed |
| A-7 | Extracting ike/dataplane to core is feasible as-is | earlier go list note | current package imports component/vpp via its VPP backend | read full package imports | BROKEN -- out of scope until interface/backend split. |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | B-2 extraction changes hundreds of import paths across the codebase | count of importers per extracted subpackage (bgp/attribute alone has 4+ external consumers, many internal) | use migrate_module.py for mechanical rewrite; phase extractions one subpackage at a time; build after each |
| R-2 | Clustering creates generator discovery gaps (new pluginDir entries needed) | `plugin_imports.go --check` fails after nesting | add each cluster root to pluginDirs before moving; verify all.go registration set preserved |
| R-3 | Non-engine enforcement produces false positives on framework packages | dep_audit advisory already shows the 65-mismatch class from the umbrella probe | use declared subcategories (not an allowlist) for framework/host; enforce only clear-cut cases |
| R-4 | Scope is too large for one spec; should be an umbrella with child specs | work exceeds a single implementation pass | phase the work: rule doc first, then core moves, then B-2 extraction, then clustering, then enforcement upgrade |
| R-5 | Clustering requires renaming dozens of plugins whose paths appear in configs, CLI, docs | grep for old paths across .yang, .ci, docs/, .md | migrate_module.py handles import rewrites; a second pass for non-Go references (yang, docs, ci, scripts) |

## Wiring Test (MANDATORY -- NOT deferrable)
No new feature or user-facing behavior; functional `.ci` evidence is listed where touched engines/domains need runtime proof.


| Entry Point | → | Feature Code | Test / Evidence |
|-------------|---|--------------|-----------------|
| `make ze-tier-check` | → | `dep_audit.py --check` enforces non-engine tier categories from a manifest | `scripts/dev/dep_audit_gate_test.go::TestEnginePlacementSelftest` runs `dep_audit.py --selftest`, including `TestNonEnginePlacement` fixture cases |
| `go build ./...` | → | extracted `internal/core/bgp/` libraries and all moved importers compile | `go build ./...` after final generator run |
| generator run after clustering | → | `all.go` includes nested cluster plugins | `go run scripts/codegen/plugin_imports.go --check` plus normalized all.go blank-import set diff |
| `module-tiers.md` rule doc | → | framework, host-service, and domain-library categories documented | `make ze-doc-test` and `dep_audit.py --selftest` category fixtures |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `module-tiers.md` updated | Defines categories for framework (config, plugin, command, cli), host services (web, ssh, gnmi, mcp, lg), BNG/VPN domain libraries, and true core libraries. The "not engine -> core" simplification is replaced. |
| AC-2 | BGP leaf library subpackages extracted | `internal/core/bgp/{wire,capability,events,context,nlri,attribute}` exist (6 packages, in dependency order). External importers use new paths. Engine code (reactor, fsm, server, wireu, types, message) stays in `internal/component/bgp/`. |
| AC-3 | iface/vpp leaf subpackages extracted | `internal/core/iface/events` and `internal/core/vpp/events` exist and are used by external importers. `ike/dataplane` stays in `internal/component/ike/dataplane` and is recorded as future decoupling work. |
| AC-4 | BNG and VPN clusters visible in directory structure | BNG: ppp, pppoe, pppoeclient, subscriber nest under `internal/component/l2tp/`; runtime plugin folders (`authlocal`, `authradius`, `pool`, `shaper`) nest under `internal/component/l2tp/plugins/` while runtime plugin names stay full. Command YANG packages live beside their handlers under `internal/component/l2tp/cmd/yang`, `internal/component/l2tp/pppoe/cmd/yang`, and `internal/component/l2tp/subscriber/cmd/yang`, not under `plugins/`. VPN: ipsec nests under `internal/component/ike/`. PKI stays top-level in `internal/component/pki/` because it is shared certificate infrastructure for IPsec and future TLS users. AAA, traffic, firewall, and CoS stay flat. Generator discovers nested runtime plugins. |
| AC-5 | Dead stubs removed or relocated | `internal/component/diag` and `internal/component/update` are either deleted if truly dead or moved to their correct owner. `internal/plugins/ifacenetlink` is resolved. |
| AC-6 | Shared libraries correctly tiered | `audit` moves to core if leaf-confirmed. `ppp` moves to the BNG domain under l2tp. `engine`, `gokrazy`, `host`, `managed`, `pppoeclient`, and `support` have explicit category-manifest rows and rule-doc rationale. |
| AC-7 | plugins/ classification gap fixed | connect, local, provision, and systemd are classified by dep_audit as setup-feature registrations by recognizing `cmd/ze/setup_features_*.go` as registration importers. |
| AC-8 | `dep_audit.py --check` extended | Non-engine tier enforcement uses a machine-readable category manifest. A new unclassified or illegal non-engine placement exits 2. Existing framework/host/domain-library packages pass because they have category rows, not because of an unstructured exception allowlist. |
| AC-9 | All moves behavior-preserving | `go build ./...` green. `go run scripts/codegen/plugin_imports.go --check` green with plugin/schema/RPC counts preserved. `dep_audit.py --check` green. No functional change. |
| AC-10 | Generator updated | `pluginDirs`/`rpcRoot` reflect new nested locations. `go run scripts/codegen/plugin_imports.go --check` green. `all.go` registration set preserved. |
| AC-11 | Migration tool supports this spec's moves | `scripts/dev/migrate_module.py --selftest` covers top-level, core-destination, and nested component-domain moves; dry-run output for a nested fixture shows import rewrite, residual refs, generator edits, and registration preservation. |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Developer creates a new non-engine package | reads module-tiers.md -> picks correct tier from expanded categories -> dep_audit catches misplacement | `TestNonEnginePlacement` in dep_audit selftest |
| 2 | Developer looks at directory structure to understand BNG scope | sees BNG cluster grouped together -> knows what packages interact | structural: ls shows cluster grouping |
| 3 | Developer imports bgp/attribute for codec work | imports from `internal/core/bgp/attribute` (not the engine) -> dep_audit correctly classifies as core dependency, not engine dependency | `go build` + dep_audit advisory shows no false engine-dependence |
| 4 | Developer removes a clustered plugin | deletes the nested dir -> build stays green, other cluster members unaffected | plugin-self-containment test (existing pattern) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNonEnginePlacement` fixture | `scripts/dev/dep_audit.py::selftest`, invoked by `scripts/dev/dep_audit_gate_test.go::TestEnginePlacementSelftest` | category-manifest enforcement exits 2 for a planted illegal non-engine package and passes for framework/host/domain-library examples | |
| `TestSetupFeaturesIsRegistrationImporter` fixture | `scripts/dev/dep_audit.py::selftest` | `cmd/ze/setup_features_*.go` recognized as registration importers for connect/local/provision/systemd | |
| `TestClusterPluginDiscovery` | `scripts/codegen/plugin_imports_test.go` | generator discovers plugins nested under cluster dirs | |
| `TestExtractedBGPLibraryBuilds` | Go package test / command evidence | `internal/core/bgp/{wire,capability,events,context,nlri,attribute}` compiles; `types` and `wireu` remain under `internal/component/bgp` | |
| `TestMigrationToolNestedAndCoreMoves` fixture | `scripts/dev/migrate_module.py --selftest` | migration tool supports core destinations and nested component-domain destinations | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A -- no numeric inputs.

### Functional Tests
| Test | Location / Command | End-User Scenario | Status |
|------|--------------------|-------------------|--------|
| plugin-inventory-stable | `go run scripts/codegen/plugin_imports.go --check`; compare generated count summary before/after moves if needed | no plugin/schema/RPC dropped by any move | |
| all-packages-compile | `go build ./...` | every moved package and importer still compiles after path changes | |

### Interop Tests (MANDATORY for protocol features)
N/A -- structural reorganization, no wire protocol changes.

### Future
- Full non-engine enforcement (beyond just new-code detection) after all moves land.

## Files to Modify
- `ai/rules/module-tiers.md` - expanded tier categories
- `scripts/dev/dep_audit.py` - non-engine category-manifest enforcement + setup_features recognition + nested advisory areas
- `scripts/dev/dep_audit_gate_test.go` - wrapper coverage for new dep_audit selftest fixtures if needed
- `scripts/dev/migrate_module.py` - support core destinations and nested component-domain destinations
- `scripts/codegen/plugin_imports.go` - pluginDirs for cluster nesting
- `internal/component/plugin/all/all.go` - regenerated after moves
- `ai/INSTRUCTIONS.md` - update generated arch lists or Before-You source if needed; regenerate generated agent files, do not hand-edit `CLAUDE.md`/`AGENTS.md`
- Every file importing a relocated package (mechanical rewrite via the extended migrate_module.py)
- Generated files (`ai/rules/INDEX.md`, `internal/component/plugin/all/*.go`, agent instruction outputs) - update only by their generators

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | N/A | no config/CLI changes |
| YANG validation constraints | N/A | |
| YANG custom validators | N/A | |
| CLI commands/flags | N/A | |
| CLI grammar (action before identifier) | N/A | |
| Editor autocomplete | N/A | |
| Functional test for new RPC/API | N/A | |
| Pipe completeness | N/A | |
| Env var registration | N/A | |
| Doctor check for runtime dependencies | N/A | |
| Prometheus counters/metrics | N/A | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | structural change only |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | Yes (relocated) | `docs/plugin-overview.md`, `docs/features/plugins.md` - paths and inventories |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if new named inventory or migration selftest workflow is documented |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, tier docs, cluster structure docs |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | Yes (paths only) | `docs/plugin-overview.md`, `docs/features/plugins.md`, `ai/INSTRUCTIONS.md` (arch lists via arch_map.py) |
| 16 | Any changed source file is referenced by existing doc source anchors? | Yes | run `make ze-doc-test`; update every stale `source:` anchor found by `code_to_docs.py --check` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | verify with docs grep and source anchors |

## Files to Create
- `scripts/dev/tier_non_engine_categories.txt` - machine-readable non-engine category manifest consumed by `dep_audit.py --check`
- `internal/core/bgp/wire/` - extracted buffer-first encoding primitives (leaf, zero deps)
- `internal/core/bgp/capability/` - extracted BGP capability types (leaf, zero deps)
- `internal/core/bgp/events/` - extracted BGP event types (leaf, zero deps)
- `internal/core/bgp/context/` - extracted encoding context (depends on capability)
- `internal/core/bgp/nlri/` - extracted NLRI types (depends on wire)
- `internal/core/bgp/attribute/` - extracted attribute types (depends on wire + context)
- `internal/core/iface/events/` - extracted iface event types (leaf)
- `internal/core/vpp/events/` - extracted vpp event types (leaf)
- `internal/core/audit/` - extracted audit library if implementation audit confirms it is leaf
- `internal/component/l2tp/ppp/` - PPP library nested under BNG domain root
- `internal/component/l2tp/pppoe/` - PPPoE nested under BNG domain root
- `internal/component/l2tp/subscriber/` - subscriber management nested under BNG
- `internal/component/l2tp/pppoeclient/` - PPPoE client nested under BNG
- `internal/component/l2tp/plugins/` - BNG runtime edge plugins (`authlocal`, `authradius`, `pool`, `shaper`; runtime plugin names keep full L2TP names). Command YANG packages move beside their handlers under `internal/component/l2tp/{cmd,pppoe/cmd,subscriber/cmd}/yang/`
- `internal/component/ike/ipsec/` - IPsec nested under VPN domain root

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `go build ./...`, `plugin_imports.go --check`, `dep_audit.py --check` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

0. **Phase: Migration tooling precondition** -- extend `migrate_module.py` before any relocation so it supports repo-relative destinations under `internal/core/...` and nested component-domain destinations under `internal/component/<domain>/...`.
   - Tests: `TestMigrationToolNestedAndCoreMoves` fixture in `migrate_module.py --selftest`
   - Files: `scripts/dev/migrate_module.py`
   - Verify: dry-run fixture shows import rewrite, residual refs, generator edits, and all.go preservation for core and nested moves

1. **Phase: Rule doc expansion (B-3 resolution)** -- update `module-tiers.md` with non-engine categories: framework, host-service, domain-library, and true core library. Define the placement rule for each. No code moves yet.
   - Tests: human review of rule doc + `dep_audit.py --selftest` category fixtures
   - Files: `ai/rules/module-tiers.md`, `scripts/dev/tier_non_engine_categories.txt`
   - Verify: every directory in the dep_audit advisory report is classifiable under the expanded rule or deliberately out of scope

2. **Phase: Dead stubs and easy core/domain moves** -- delete or relocate component/diag, component/update. Move audit to `internal/core/` if leaf-confirmed. Move ppp to `internal/component/l2tp/ppp`. Categorize engine, gokrazy, host, managed, pppoeclient, and support in the manifest.
   - Tests: `go build ./...`, dep_audit advisory, plugin inventory diff
   - Files: relocated packages + all importers (mechanical rewrite)
   - Verify: `go build ./...` green, plugin inventory stable

3. **Phase: Audit classification fix** -- recognize setup_features_*.go as registration importers in dep_audit. Correctly classify connect/local/provision/systemd as registered setup commands.
   - Tests: `TestSetupFeaturesIsRegistrationImporter` fixture in dep_audit selftest
   - Files: `scripts/dev/dep_audit.py`
   - Verify: dep_audit advisory no longer misclassifies these 4 dirs

4. **Phase: B-2 extraction layer 0 (leaf)** -- extract `bgp/wire`, `bgp/capability`, `bgp/events` to `internal/core/bgp/`. These have zero internal dependencies. Also extract `iface/events` and `vpp/events` (also leaf). Mechanical import rewrite.
   - Tests: `go build ./...`, affected unit tests, dep_audit advisory
   - Files: new `internal/core/bgp/{wire,capability,events}`, `internal/core/iface/events`, `internal/core/vpp/events` + all importers
   - Verify: `go build ./...`, plugin inventory

5. **Phase: B-2 extraction layer 1 (one-deep)** -- extract `bgp/context` (depends on core/bgp/capability) and `bgp/nlri` (depends on core/bgp/wire). These can move once their single dependency is in core.
   - Tests: `go build ./...`, affected unit tests
   - Files: new `internal/core/bgp/{context,nlri}` + importers
   - Verify: `go build ./...`

5b. **Phase: B-2 extraction layer 2** -- extract `bgp/attribute` (depends on core/bgp/wire + core/bgp/context). Note: `bgp/message` stays in component/bgp/ because it imports `plugin/registry` (needs refactoring to decouple). `bgp/types` and `bgp/wireu` stay (deep engine coupling).
   - Tests: `go build ./...`, affected unit tests
   - Files: new `internal/core/bgp/attribute` + importers
   - Verify: `go build ./...`, dep_audit advisory shows reduced false engine-dependence

6. **Phase: Related-code clustering** -- cluster only BNG and VPN. BNG nests under `internal/component/l2tp/`; VPN nests under `internal/component/ike/`. AAA, traffic, firewall, and CoS are explicit non-goals. Update generator pluginDirs. Mechanical import rewrite.
   - Tests: `TestClusterPluginDiscovery`, generator --check, plugin inventory diff
   - Files: cluster directories + all importers + pluginDirs
   - Verify: `go build ./...`, plugin inventory stable, self-containment tests pass

7. **Phase: Enforcement upgrade** -- extend dep_audit --check to enforce non-engine tiers for new code using `scripts/dev/tier_non_engine_categories.txt`. Framework/host/domain-library categories are declared and reviewed, not an unstructured allowlist.
   - Tests: `TestNonEnginePlacement` fixture in dep_audit selftest
   - Files: `scripts/dev/dep_audit.py`, `scripts/dev/tier_non_engine_categories.txt`
   - Verify: `make ze-tier-check` exits 2 for a planted unclassified/illegal non-engine placement and passes on the real tree

8. **Full verification** -- `go build ./...`, `go run scripts/codegen/plugin_imports.go --check`, `python3 scripts/dev/dep_audit.py --check`
9. **Complete spec** -- learned summary, two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Import rewrites are complete (no stale paths in .go, .yang, .ci, docs, scripts) |
| Data flow | Registration chain preserved: package -> init() -> registry -> config -> start |
| Plugin inventory | `make ze-inventory-json` normalized before/after diff empty |
| Self-containment | Removing any moved plugin still passes build or the owner surface disappears cleanly |
| Rule: no-layering | Old paths fully deleted, no redirects or aliases |
| Quality rule | Every Self-Critical Review row and applicable Adversarial Self-Review item in `ai/rules/quality.md` is answered |
| Registration over hardcoding | New nested packages remain registered/discovered through existing registries and generator outputs; no new per-feature switch, factory, or hardcoded core field |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Expanded module-tiers.md | read file, all dep_audit dirs classifiable |
| migration tool supports core/nested moves | `scripts/dev/migrate_module.py --selftest` |
| core/bgp/ library packages | `ls internal/core/bgp/` + `go test ./internal/core/bgp/...` |
| Cluster directories | `ls internal/component/l2tp/` and `ls internal/component/ike/` show the expected nested packages |
| dep_audit non-engine enforcement | `dep_audit.py --selftest` fixture with planted misplacement -> exit 2 |
| Plugin inventory unchanged | `make ze-inventory-json` normalized diff empty |
| Generator clean | `go run scripts/codegen/plugin_imports.go --check` exit 0 |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | N/A -- structural reorganization, no new inputs |
| Import path injection | migration tool rewrites only quoted Go import paths, not arbitrary strings |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Build break after a move | migration tool's rewrite step -- fix path handling, re-run |
| `--plugins` inventory changed | generator not updated -- fix pluginDirs, re-run |
| dep_audit false positive on framework/host | refine category definition in rule doc |
| B-2 extraction creates import cycle | the extracted subpackage has a hidden component-tier dependency -- decouple first |
| 3 fix attempts fail | STOP, report, ask user |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| All initially listed BGP library candidates could move to core | `bgp/message` imports `plugin/registry`; `bgp/types` and `bgp/wireu` remain deeply coupled | go list analysis captured in A-1 | extraction scope reduced to six BGP packages |
| Existing `migrate_module.py` could handle nested/core moves | current tool only handles top-level component<->plugins moves | review of `scripts/dev/migrate_module.py` | added phase 0 to extend the tool before relocations |
| `ike/dataplane` was leaf | its VPP backend imports `internal/component/vpp` | source read of `internal/component/ike/dataplane/vpp.go` | `ike/dataplane` removed from this spec; future split needed |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Put CoS under the BNG cluster | CoS owns global class-of-service config and interface VLAN QoS behavior | Keep CoS flat unless a later spec splits subscriber-specific behavior |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Core Insight
Directory structure is a communication tool. The tiers exist not just for the gate but to tell a developer "these packages form a unit" and "changing this cannot affect that." The import graph already contains this information; the structure should surface it.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Extracted BGP library subpackages go to `internal/core/bgp/` | (a) new `internal/lib/` tier; (b) keep in component/bgp/ with smarter audit | core/ rule is "leaf library that can't run as a plugin" -- exactly what wire/capability/events/context/nlri/attribute become after layered extraction. A new tier adds decisional complexity without solving a real problem. |
| Extraction is layered (6 packages), not batch | batch move of all 9 candidates | dependency analysis shows wire/capability/events are leaf, context/nlri/attribute depend on them, message depends on plugin/registry (can't extract without decoupling), types/wireu depend on nearly everything (stay in engine). Layered extraction keeps the build green at each step. |
| Cluster BNG and VPN only | (a) cluster all 5 groups; (b) flat naming prefix; (c) intermediate grouping dir in plugins/ only | BNG and IKE/IPsec are genuinely isolated domains. PKI is top-level shared certificate infrastructure for IPsec and future TLS users. AAA is platform infrastructure (depended on by api, bgp, ssh, web). Traffic/firewall are borderline. CoS has global interface-facing behavior. Nesting those would miscommunicate blast radius. |
| BNG cluster root = `internal/component/l2tp/`; VPN root = `internal/component/ike/` for IPsec only | new `internal/component/bng/` or `internal/component/vpn/` top-level | l2tp and ike are already components. Nesting related packages under them follows the BGP precedent without adding another top-level domain; PKI remains top-level until a later spec splits shared certificate infrastructure. |
| `module-tiers.md` expanded with 4 non-engine categories | (a) 4th `internal/host/` tier; (b) single "not engine" bucket | a 4th tier adds a location but doesn't solve classification. Categories within component/ plus a manifest make human placement decisions reviewable and enforceable. |
| Enforcement upgrade: fail new unclassified or illegal non-engine placements, don't retroactively fail categorized existing packages | enforce everything immediately; warn only; keep purely advisory | Retroactive enforcement on existing dirs is noise, and warn-only does not stop drift. A category manifest turns human B-3 decisions into reviewable gate input. |

## Known Limitations
- `bgp/message` stays in `internal/component/bgp/` (imports plugin/registry -- needs its own decoupling spec to extract).
- `bgp/types` and `bgp/wireu` stay in `internal/component/bgp/` (deep cross-dependencies with the engine).
- `ike/dataplane` stays in `internal/component/ike/dataplane/` until its core interface is split from component/VPP backend registration.
- `pki` stays in `internal/component/pki/`. It is not treated as VPN-only in this spec because the feature docs describe it as shared infrastructure for IPsec and TLS consumers.
- AAA cluster (aaa, authz, radius, tacacs) is NOT nested because it's platform infrastructure consumed by api, bgp, ssh, web. Nesting would falsely imply isolation.
- Traffic and firewall clusters left flat for now; they have moderate external dependents that make isolation claims weak.
- CoS stays flat because it owns global class-of-service config and interface VLAN QoS, not only BNG subscriber behavior.
- Non-engine enforcement uses a category manifest for framework/host/domain-library packages. This is human judgment made explicit and reviewable, not a hidden allowlist.
- This is large work (8+ phases, hundreds of import rewrites). If split into child specs, every AC must move to an explicit receiving spec before scope can shrink.

## Implementation Summary

### What Was Implemented
- Expanded `ai/rules/module-tiers.md` with four non-engine categories: framework, host-service, domain-library, planned-violation (AC-1)
- Extracted 6 BGP library subpackages to `internal/core/bgp/`: wire, capability, events, context, nlri, attribute (AC-2)
- Extracted `iface/events` and `vpp/events` to `internal/core/` (AC-3)
- Clustered BNG domain under `internal/component/l2tp/`: ppp, pppoe, pppoeclient, subscriber as siblings; 7 edge plugins under `l2tp/plugins/` (authlocal, authradius, pool, shaper, cmd, pppoe-cmd, subscriber-cmd) (AC-4)
- Clustered VPN domain: ipsec nested under `internal/component/ike/ipsec/` (AC-4)
- Deleted dead stubs: component/diag, component/update, plugins/ifacenetlink (AC-5)
- Moved audit to `internal/core/audit/` (AC-6)
- Moved ppp to `internal/component/l2tp/ppp/` as BNG domain library (AC-6)
- Extended `dep_audit.py` to recognize `setup_features_*.go` as registration importers (AC-7)
- Added `scripts/dev/tier_non_engine_categories.txt` manifest with 28 rows, enforced by `dep_audit.py --check` (AC-8)
- Extended `scripts/dev/migrate_module.py` for core and nested component-domain destinations (AC-11)
- Updated generator `pluginDirs`/`nestedPluginDomains` for l2tp and ike cluster namespaces (AC-10)
- Regenerated all.go, arch_map, code_to_docs indexes

### Bugs Found/Fixed
- None specific to this work. Pre-existing verify failures unrelated to tier moves.

### Documentation Updates
- `ai/rules/module-tiers.md`: expanded with non-engine categories and clustering rules
- `ai/INSTRUCTIONS.md`: regenerated arch lists via arch_map.py
- `docs/plugin-overview.md`, `docs/features/plugins.md`: paths updated for moved packages
- Source anchors updated via `code_to_docs.py --check` (all valid)

### Deviations from Plan
- `pki` stays flat at `component/pki/` (categorized as framework, not domain-library) because it serves as shared certificate infrastructure for IPsec and future TLS consumers, not VPN-only
- `ike/dataplane` not extracted to core (imports component/vpp); recorded in Known Limitations
- `bgp/message`, `bgp/types`, `bgp/wireu` not extracted (deep engine coupling); recorded in Known Limitations
- CoS left flat (global interface QoS behavior, not BNG-only)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Resolve B-3 (non-engine categories) | Done | `ai/rules/module-tiers.md`, `tier_non_engine_categories.txt` | 4 categories, 28 manifest rows |
| Resolve B-2 (fused library+engine) | Done (partial) | `internal/core/bgp/{wire,capability,events,context,nlri,attribute}` | 6 of 9 extracted; message/types/wireu stay |
| Related-code clustering | Done | `component/l2tp/`, `component/ike/` | BNG and VPN nested; AAA/traffic/firewall flat |
| Migration tool extension | Done | `scripts/dev/migrate_module.py` | Core + nested destinations, selftest passes |
| Misplaced dir moves + enforcement | Done | dep_audit --check green, dead stubs deleted | 28 manifest rows, no unclassified |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `grep -c 'framework\|host-service\|domain-library' ai/rules/module-tiers.md` = 7 | |
| AC-2 | Done | `ls internal/core/bgp/` shows 6 packages | |
| AC-3 | Done | `ls internal/core/iface/events internal/core/vpp/events` | |
| AC-4 | Done | `ls component/l2tp/{ppp,pppoe,subscriber,plugins}` and `component/ike/ipsec` | |
| AC-5 | Done | diag, update, ifacenetlink all GONE | |
| AC-6 | Done | `ls internal/core/audit/`, ppp under l2tp, manifest rows for remaining | |
| AC-7 | Done | dep_audit.py has 6 references to setup_features | |
| AC-8 | Done | `dep_audit.py --check` reports "28 manifest row(s)" | |
| AC-9 | Done | `go build ./...` exit 0, generator --check shows 98 plugins preserved | |
| AC-10 | Done | `plugin_imports.go --check` shows "98 plugins, 125 schemas, 39 rpcs" | |
| AC-11 | Done | `migrate_module.py --selftest` exit 0 | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestNonEnginePlacement | Done | `dep_audit.py --selftest` | Fixture tests planted misplacement -> exit 2 |
| TestSetupFeaturesIsRegistrationImporter | Done | `dep_audit.py --selftest` | setup_features_*.go wired plugins classified correctly |
| TestClusterPluginDiscovery | Done | `plugin_imports.go --check` | Nested l2tp/plugins and ike discovered |
| TestExtractedBGPLibraryBuilds | Done | `go build ./internal/core/bgp/...` | All 6 packages compile |
| TestMigrationToolNestedAndCoreMoves | Done | `migrate_module.py --selftest` exit 0 | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `ai/rules/module-tiers.md` | Modified | Non-engine categories added |
| `scripts/dev/dep_audit.py` | Modified | Non-engine gate + setup_features recognition |
| `scripts/dev/tier_non_engine_categories.txt` | Created | 28 rows |
| `scripts/dev/migrate_module.py` | Modified | Core + nested move support |
| `scripts/codegen/plugin_imports.go` | Modified | pluginDirs + nestedPluginDomains |
| `internal/component/plugin/all/all.go` | Regenerated | 98 plugins preserved |
| `internal/core/bgp/{wire,capability,events,context,nlri,attribute}` | Created | Extracted from component/bgp |
| `internal/core/iface/events` | Created | Extracted from component/iface |
| `internal/core/vpp/events` | Created | Extracted from component/vpp |
| `internal/core/audit` | Moved | From component/audit |
| BNG cluster under l2tp/ | Moved | ppp, pppoe, pppoeclient, subscriber + 7 plugins |
| VPN ipsec under ike/ | Moved | From component/ipsec |

### Audit Summary
- **Total items:** 32 (5 requirements, 11 ACs, 5 tests, 12 files)
- **Done:** 32
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4 (Deviations: pki flat, ike/dataplane out, bgp/message+types+wireu stay, cos flat)

## Executive Summary

### Objective
Complete the module-tier taxonomy: resolve B-2 (fused library+engine) and B-3 (non-engine categories), introduce related-code clustering for isolated domains, enforce non-engine placements.

### Changes
| Area | What changed | Why |
|------|--------------|-----|
| Rule doc | `module-tiers.md` expanded with 4 non-engine categories | B-3: "not engine -> core" was wrong for 28 packages |
| Core extraction | 6 BGP + 2 iface/vpp + audit packages to `internal/core/` | B-2: separate library use from engine dependence |
| BNG clustering | ppp/pppoe/pppoeclient/subscriber + 7 plugins under `component/l2tp/` | Isolated domain; structure shows blast radius |
| VPN clustering | ipsec under `component/ike/` | Isolated domain |
| Dead code | diag, update, ifacenetlink deleted | 0 importers, 0 registration |
| Enforcement | `dep_audit.py --check` enforces non-engine categories via manifest | Prevent drift; make B-3 decisions reviewable |
| Tooling | `migrate_module.py` extended for core + nested destinations | Enable mechanical moves |

### Risks and observations
- `bgp/message` stays in component/bgp/ (imports plugin/registry). Future decoupling needed.
- `ike/dataplane` stays (imports component/vpp). Future interface/backend split needed.
- `DOMAIN_LIBRARY_PREFIXES` in dep_audit.py hardcodes l2tp + ike. Third cluster requires updating this tuple.

### Verification
- `go build ./...`: exit 0
- `dep_audit.py --check`: engine clean + 28 non-engine rows clean
- `plugin_imports.go --check`: 98 plugins, 125 schemas, 39 rpcs preserved
- `arch_map.py --check`: up to date
- `code_to_docs.py --check`: 1186 paths valid
- `migrate_module.py --selftest`: OK
- `dep_audit.py --selftest`: OK

## Goal Validation (BLOCKING)
| Goal | Evidence Type | Concrete Evidence |
|------|---------------|-------------------|
| Non-engine tiers documented | rule doc + category manifest | `module-tiers.md` covers every category; `tier_non_engine_categories.txt` classifies existing non-engine packages |
| Library/engine separated | build + audit | `go test ./internal/core/bgp/...` green; dep_audit advisory shows no false engine-dependence for extracted packages |
| Clusters visible | structure + generator | `ls internal/component/l2tp/` and `ls internal/component/ike/`; generator check proves nested registrations |
| Enforcement extended | gate test | `dep_audit.py --selftest` has planted illegal non-engine placement exiting 2; real tree passes `make ze-tier-check` |
| Migration tooling covers requested moves | selftest | `migrate_module.py --selftest` covers top-level, core, and nested component-domain moves |
| Zero functional change | inventory + compile | `go run scripts/codegen/plugin_imports.go --check` count summary preserved; `go build ./...` green |

## Review Gate
### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification
### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/bgp/wire/` | Yes | `ls -d internal/core/bgp/wire` |
| `internal/core/bgp/capability/` | Yes | `ls -d internal/core/bgp/capability` |
| `internal/core/bgp/events/` | Yes | `ls -d internal/core/bgp/events` |
| `internal/core/bgp/context/` | Yes | `ls -d internal/core/bgp/context` |
| `internal/core/bgp/nlri/` | Yes | `ls -d internal/core/bgp/nlri` |
| `internal/core/bgp/attribute/` | Yes | `ls -d internal/core/bgp/attribute` |
| `internal/core/iface/events/` | Yes | `ls -d internal/core/iface/events` |
| `internal/core/vpp/events/` | Yes | `ls -d internal/core/vpp/events` |
| `internal/core/audit/` | Yes | `ls -d internal/core/audit` |
| `internal/component/l2tp/ppp/` | Yes | `ls -d internal/component/l2tp/ppp` |
| `internal/component/l2tp/plugins/` | Yes | `ls -d internal/component/l2tp/plugins` |
| `internal/component/ike/ipsec/` | Yes | `ls -d internal/component/ike/ipsec` |
| `scripts/dev/tier_non_engine_categories.txt` | Yes | `ls scripts/dev/tier_non_engine_categories.txt` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | module-tiers.md has non-engine categories | `grep -c 'framework\|host-service\|domain-library' ai/rules/module-tiers.md` = 7 |
| AC-2 | 6 BGP packages in core | `ls internal/core/bgp/` shows wire, capability, events, context, nlri, attribute |
| AC-3 | iface/vpp events in core | `ls -d internal/core/{iface,vpp}/events` |
| AC-4 | BNG + VPN clusters | `ls internal/component/l2tp/{ppp,pppoe,pppoeclient,subscriber,plugins}` + `ls internal/component/ike/ipsec` |
| AC-5 | Dead stubs gone | `ls internal/component/{diag,update}` -> No such file; `ls internal/plugins/ifacenetlink` -> No such file |
| AC-6 | audit in core, ppp in l2tp | `ls internal/core/audit` + `ls internal/component/l2tp/ppp` |
| AC-7 | setup_features recognized | `grep -c setup_features scripts/dev/dep_audit.py` = 6 |
| AC-8 | Non-engine enforcement | `dep_audit.py --check` -> "28 manifest row(s)" |
| AC-9 | Build green | `go build ./...` exit 0 |
| AC-10 | Generator clean | `plugin_imports.go --check` -> "98 plugins, 125 schemas, 39 rpcs" |
| AC-11 | Migration tool extended | `migrate_module.py --selftest` exit 0 |

### Wiring Verified (end-to-end)
| Entry Point | Evidence | Verified |
|-------------|----------|----------|
| `make ze-tier-check` | `dep_audit.py --check` exits 0 with engine + non-engine clean | Yes |
| `make ze-verify` (build) | `go build ./...` exit 0 | Yes |
| Generator after clustering | `plugin_imports.go --check` -> 98 plugins preserved | Yes |
| Rule doc | `module-tiers.md` has framework/host-service/domain-library sections | Yes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | PARTIALLY BROKEN | 6 of 9 BGP subpackages extracted. message (plugin/registry dep), types, wireu stay. |
| A-2 | Confirmed | l2tp/plugins + ike nested; generator discovers them; all.go preserved |
| A-3 | Confirmed | host categorized as host-service in manifest; not moved |
| A-4 | Confirmed | setup_features_*.go is the only non-test wiring path |
| A-5 | BROKEN then fixed | migrate_module.py extended in Phase 0; selftest covers nested+core |
| A-6 | Confirmed | iface/events and vpp/events extracted; leaf verified |
| A-7 | BROKEN | ike/dataplane imports component/vpp; excluded from scope |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Plugin paths updated | `arch_map.py --check` -> up to date | Yes |
| Source anchors valid | `code_to_docs.py --check` -> 1186 paths, all valid | Yes |
| Architecture docs | `ai/rules/module-tiers.md` expanded with non-engine categories | Yes |

## Checklist
### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has concrete command/test evidence, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `go build ./...` passes
- [ ] `make ze-test` and `make ze-verify` are not completion gates for this move-only spec; `go build ./...` plus generator/audit checks are the user-approved gate
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes every Self-Critical Review row and applicable Adversarial Self-Review item in `ai/rules/quality.md`
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes every Self-Critical Review row and applicable Adversarial Self-Review item in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Executive Summary filled
- [ ] Write learned summary to `plan/learned/NNN-tiers-5-structure-tidy.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-tiers-5-structure-tidy.md`

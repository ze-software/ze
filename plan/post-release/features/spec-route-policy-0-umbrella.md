# Spec: route-policy-0-umbrella -- one route policy library for every protocol, into the RIB and out

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P3 |
| Release reason | New capability: a protocol-neutral route policy and a gate on every source entering the Loc-RIB. No shipped behavior is defective without it; the two defects it owns (journal rows below) are post-release quality, not release blockers. |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-fixit-redistribution-chain-drops-silently.md` (phase 3 needs a working redistribution chain to prove anything); `plan/release/fixes/spec-single-owner-reload-reconcile.md` (its AC-9 tests the redistribute evaluator phase 3 replaces); phase 4 sequences with `plan/post-release/engineering/spec-filter-wire-0-umbrella.md` |
| Phase | 1/4 |
| Handoff | - |
| Updated | 2026-10-29 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze's route policy is BGP-only. Filter types live under `bgp policy`, read BGP
UPDATE text, and attach only to BGP sessions. Every other source (IS-IS, OSPF,
static, connected, forked plugins) inserts into the unified Loc-RIB with no
gate, and redistribution carries its own small family-and-tag matcher. An
operator cannot say "only install these IS-IS prefixes", "never install a
bogon from any protocol", or "advertise only loopbacks into IS-IS".

The goal is one route policy library and one filter syntax that every protocol
and route-moving plugin uses, on the way into the RIB and on the way out:

| Direction | Where it attaches | What it decides |
|-----------|-------------------|-----------------|
| Import, per protocol | `isis`, `ospf`, `static`, `connected`: `filter { import { filters [ ... ]; final accept\|reject; } }` | whether that protocol's route is installed in the Loc-RIB, and with which attributes |
| Import, central | `rib { filter { import { ... } } }` | runs on every source's route after that source's own chain, BGP's best included |
| Import, BGP | `bgp`/group/peer `filter import` (unchanged shape) | per-peer ingress admission, as today |
| Export, per protocol | `isis`, `ospf`: `filter { export { ... } }`; `bgp` export as today | what the protocol advertises |
| Redistribution | `redistribute { destination X { import Y { filters [ ... ]; final ...; } } }` | what moves from one protocol into another |

Owner decisions (2026-10-29), all settled:

| Decision | Outcome |
|----------|---------|
| Attachment | Both per protocol and central `rib filter import` |
| Policy root | `bgp policy` moves to the top-level `policy` root, shared with policy-based routing's existing `policy route`. No alias for the old path (no layering). Filter types are discovered from the `ze:filter` marker, so `policy route` is never taken for a filter type |
| Language | Keep Ze's chain of named filter instances plus a required `final`; add a generic `match` filter type rather than adopting Junos `term from/then` |
| Spec shape | One umbrella, four phases |
| Gate location | One gate at the Loc-RIB insert, which every source passes |
| Rejected routes | Kept in the Loc-RIB as hidden paths (Junos hidden routes): never selected, never programmed, re-evaluated when a filter changes, so relaxing a filter re-admits without producer replay |
| BGP import | Stays the existing per-peer ingress chain; `rib filter import` also sees BGP's best |
| Route fields | Route tag, IS-IS level and OSPF route type are added to the Loc-RIB path so filters can match them |
| OSPF redistribute settings | OSPF's per-source `redistribute { source; metric; metric-type; tag }` is folded into the OSPF export chain's set actions and deleted |

Junos reference (owner request): `policy-options` holds one policy language;
each protocol applies it with `import` and `export`. IS-IS import (Junos 17.1
and later) rejects by prefix or tag and sets install priority; OSPF import
covers external routes; neither touches the link-state database. Ze matches
that: an IGP import filter decides local installation only, never flooding.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/rib/unified-locrib.md` - the Loc-RIB the gate sits on
  → Decision: the Loc-RIB owns AdminDistance; every insert overwrites it in `PathGroup.upsert` via `resolvedDistance`. A filter that sets distance writes the existing `DistanceOverride`/`HasDistanceOverride` pair, never AdminDistance.
  → Decision: the two triggers do not collapse. BGP's ingress filter pipeline stays on the receive path; the Loc-RIB gate is a second, later stage. `bgp filter import` keeps its meaning.
  → Constraint: `(*RIB).Reselect` re-resolves stored paths and dispatches ChangeUpdate only where best or ECMP moved. Filter reload reuses that walk.
- [ ] `docs/guide/bgp-policy.md` - the policy model being generalised
  → Constraint: chains are cumulative across levels (automatic, global, group, peer); the most-specific configured direction supplies `final`; a direction must state its own `final`; inactive references stay in the chain and are skipped. The new attachments keep every one of these rules.
  → Constraint: a filter's `accept` means continue; only `final` grants acceptance; an earlier reject or fail-closed error cannot be overridden by `final accept`.
- [ ] `docs/guide/redistribution.md` - filters and redistribution roots
  → Decision: the `redistribute` root's `family` and `tag` leaves are replaced by chains in phase 3 (no layering). Withdrawal always reaches a destination that holds a route, whatever the rule now says; that rule stays.
- [ ] `docs/architecture/core-design.md` - Ingress Filter Pipeline, ModAccumulator
  → Constraint: in-process BGP filters take wire bytes and write mod-ops; the external chain is text-based (`spec-filter-wire-0-umbrella` removes the text). The library must not add a third representation for BGP.
- [ ] `internal/component/sysrib/yang/ze-rib-conf.yang` - the `rib` root that gains `filter`
  → Constraint: `fib-withhold` sits downstream of selection in sysrib and never touches the Loc-RIB. Import filtering is a different stage: a hidden path is not selected at all, a withheld winner is selected and not programmed. Both stay.
- [ ] `docs/architecture/isis/isis-9-spf-rib.md` - IS-IS install is a Loc-RIB insert
  → Constraint: IS-IS resolves level and up/down preference inside SPF and publishes one path per prefix with no level field. The new level field on Path records the level of that already-selected route; filtering by level never changes which level SPF prefers.
  → Decision: per-level admin distance stays unimplemented; a `set distance` on `match level` in `isis filter import` gives an operator that effect per route without new distance leaves.
- [ ] `docs/architecture/isis/isis-11-redistribution.md` - IS-IS as producer and consumer
  → Constraint: one `isis` protocol identity carries both levels and self-import is rejected by `WouldLoop`; the export chain must not split the identity per level.
  → Constraint: the consumer writes Extended IP Reachability with a fixed default metric through the injector seam; the IS-IS export chain runs before that write and supplies metric and level from its set actions.
- [ ] `docs/architecture/api/ipc_protocol.md` - plugin RPC framing
  → Constraint: the route-install RPC is a JSON RPC over the plugin transport; the tag, level and route-type fields are added as optional JSON members so an older forked plugin that omits them is read as "not carried", which the field declaration then refuses for a filter that needs them (R-8).
- [ ] `docs/guide/policy-routing.md` - the existing `policy route` root
  → Constraint: policy routing keeps `policy { route <name> { rule ... } }` and its `show policy routes` command unchanged; the shared root must not change its config, tests or output.

### RFC Summaries (Scope: protocol)
- [ ] No RFC defines a local RIB import policy for IS-IS or OSPF; it is local policy. The constraint that applies is that the LSDB is not altered: flooding and SPF input stay untouched (A-3).
  → Constraint: an IGP import filter acts after SPF, on the routes the IGP offers the Loc-RIB; it never edits LSPs/LSAs received or suppresses flooding.

**Key insights:** (minimal context to resume after compaction)
- Every Loc-RIB write ends in `(*RIB).insert` / `(*RIB).Remove` (`internal/core/rib/locrib/manager.go`), forked plugins included (`applyRouteInstall` in `plugin/server/dispatch_route.go` calls `InsertForward`). That is the one gate.
- Insert is the BGP full-table hot path and runs under the shard lock with OnChange subscribers dispatched under it: the gate is allocation-free and reads a snapshot published through an atomic pointer, like the distance seam (`internal/core/rib/distance`).
- `locrib.Path` carries no tag, IS-IS level or OSPF route type; prefix and family are arguments to insert, not fields. Static's tag reaches only redistevents.
- A `policy` root exists (policy routing). `BuildFilterRegistry` takes every list child of the policy container as a filter type and never reads `ze:filter`; no Go code reads that marker.
- Three filter registries exist: plugin-registry `FilterTypes` (type to plugin), stage-1 `Registration.Filters` (`<plugin>:<filter>` refs, with an `Attributes` field already declaring what a filter reads, in BGP names), and in-process `filterapi.Register`.
- Redistribution's `ImportRule.Accept` is an OR over rules with no `final`; `import ibgp`/`import ebgp` never match (journal row).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/rib/locrib/manager.go` - `insert` validates, locks the prefix shard, upserts in `Modify`, computes best and ECMP siblings, dispatches Add/Update, records metrics. `Remove` keyed on (Source, Instance).
  → Constraint: ChangeRemove fires only when the PathGroup has no valid best; sysrib's `changeToBatch` relies on that. Hidden paths must keep "no selectable best" mapped to ChangeRemove.
- [ ] `internal/core/rib/locrib/candidate.go` - `Path` fields: Source, Instance, NextHop, Interface, OnLink, Weight, AdminDistance, DistanceOverride pair, Metric, Labels, SRv6SID, IsEBGP, IsBGP, AIGP, MetricRecursive, Backup*, ECMP, RouteType. `Equal` and `key` contracts documented per field.
  → Constraint: a new field states whether it is in `key()` and `Equal`. Tag, level and route type are excluded from `key()` (same source re-advertising is an update) and compared by `Equal` (a change must re-run the filter and reach consumers).
- [ ] `internal/core/rib/locrib/entry.go` - `PathGroup{Paths, Best}`, `upsert` writes AdminDistance and reselects; `remove`.
- [ ] `internal/component/bgp/config/filter_registry.go` - `BuildFilterRegistry` scans list children of the policy schema; refuses one instance name under two types.
- [ ] `internal/component/bgp/filterapi/policy.go`, `filterapi.go` - `Policy{refs, final, requiresIngress}`, `FilterRef{Name, Inactive}`, `AcceptsAtEnd` carrying the RFC 8212 EBGP default, `Register` for in-process filters.
- [ ] `internal/component/bgp/config/redistribution.go` - `extractFilterPolicy`, `concatPolicies`, `canonicalizeFilterRefs` (plugin:name, type:name, bare name).
- [ ] `internal/component/bgp/config/peers.go` - `prependDefaultFilters` hardcodes `loop-detection`; reads `GetContainer("policy")` under `bgp`.
- [ ] `internal/component/bgp/reactor/filter_chain.go` - `PolicyFilterChain` runs the external chain on text, skips inactive and in-process refs, applies `final`.
- [ ] `internal/component/config/redistribute/route.go`, `evaluator.go` - `RedistRoute{Origin, Family, Source, Tag}`, `ImportRule.Accept`, `Evaluator` singleton set at startup only.
- [ ] `internal/component/bgp/plugins/redistribute_egress/redistribute.go`, `held.go`, `replay.go` - `handleBatch` builds RedistRoute from the batch protocol name, decides per consumer, `removeReplacedRoute` withdraws a held route a rejected replacement displaces.
- [ ] `internal/plugins/isis/redistribute/consumer.go`, `internal/plugins/ospf/redistribute/consumer.go` - `InjectRoute`: IS-IS writes TLV 135/236 with `DefaultRedistMetric`; OSPF calls `InjectExternal(prefix, source, routeTag)` with per-source `metric`, `metric-type`, `tag` from `ospf redistribute`.
- [ ] `internal/plugins/isis/spf/install.go`, `internal/plugins/ospf/spf/install.go`, `internal/plugins/static/locrib.go`, `internal/plugins/connected/locrib.go`, `internal/component/bgp/plugins/rib/rib_remote.go` - the five producers; each keeps its own installed set.
- [ ] `internal/plugins/policyroute/yang/ze-policyroute-conf.yang` - top-level `container policy { list route { list rule } }`.

**Behavior to preserve:**
- BGP ingress and egress chains: semantics, level inheritance, `final`, RFC 8212 default, automatic loop-detection, inactive members, reload of `final` with replay.
- Policy routing config, `show policy routes`, and every policy-routing test.
- `fib-withhold` behavior and `show rib` output for routes no filter touches.
- Redistribution: loop guard (`WouldLoop`), replay to late consumers and late peers, withdrawal reaching a holding destination regardless of the current rule.
- Loc-RIB: Add/Update/Remove dispatch contract, ECMP sibling computation, Reselect on distance change, zero allocations on the insert path for a route no filter rejects.

**Behavior to change:**
- `bgp policy { <type> <name> ... }` moves to `policy { <type> <name> ... }`; the old path is refused at load with a message naming the new one, and `ze config migrate` rewrites it.
- New import attachments on `isis`, `ospf`, `static`, `connected`, `rib`; new export attachments on `isis`, `ospf`.
- `redistribute` `family` and `tag` leaves are replaced by `filters`/`final`; `ospf redistribute` metric/metric-type/tag become set actions of the OSPF export chain.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `policy { <filter-type> <name> { ... } }` defines instances; `filter { import|export { filters [ ... ]; final ...; } }` under a protocol, `rib`, or a redistribute import attaches them.
- Routes: a producer's insert into the Loc-RIB (in-process call, or the route-install RPC for a forked plugin); a redistribute batch; an IGP origination.

### Transformation Path
1. Config load: the policy registry discovers filter types from the `ze:filter` marker across all modules augmenting `policy`, builds the instance namespace, resolves every attachment's references, and checks each referenced type's declared fields against the fields the attachment's protocol declares for its routes (refuse on mismatch).
2. Compilation: each attachment becomes an immutable compiled chain; the per-source chain table and the central chain are published as one snapshot through an atomic pointer read by the Loc-RIB.
3. Import: `insert` reads the snapshot before taking the shard lock, evaluates the source's chain then the central chain against a route view over the Path plus (family, prefix). Outcome: accept (possibly with set actions applied to the Path value), or reject. A rejected path is upserted as hidden.
4. Selection: `selectBest` and `siblingNextHops` skip hidden paths; a group whose only paths are hidden has no best and dispatches ChangeRemove.
5. Reload: publishing a new snapshot calls Reselect, which re-evaluates every stored path (hidden ones included) against the new chains, re-applies set actions from the producer's original values, and dispatches where best or ECMP moved.
6. Export: an IGP consumer runs its export chain in `InjectRoute` before writing the LSP/LSA, applying set actions (metric, metric-type, tag, level). BGP export is unchanged. Redistribute runs the import rule's chain per (consumer, route) in `handleBatch` and replay.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Forked plugin to engine (route install) | `rpc.RouteInstallInput` gains tag, level and route-type fields; the engine rebuilds the Path and inserts through the same gate | No |
| Config component to Loc-RIB (core) | compiled chain snapshot published through an atomic-pointer seam in a core leaf package, like `internal/core/rib/distance` | No |
| Policy library to BGP | BGP's chains compile from the same registry and attachment model; BGP evaluation keeps its wire-native path behind a lazy route view | No |

### Integration Points
- `internal/core/rib/locrib` `insert`, `selectBest`, `siblingNextHops`, `Reselect` - the gate, the hidden flag and re-evaluation.
- `internal/component/bgp/config/filter_registry.go` - becomes the protocol-neutral policy registry (moved out of BGP).
- `internal/component/config/redistribute` - `ImportRule` carries a compiled chain instead of family/tag.
- `internal/component/config/redistribute/registry.go` `RouteSource` - gains the declaration of the fields its routes carry.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | every producer reaches the Loc-RIB through `insert`; the gate is there, so no producer can skip it |
| No unintended coupling (components stay isolated) | Yes | the Loc-RIB reads a compiled-chain seam in a core leaf package; it imports neither config nor any protocol |
| No duplicated functionality (extends existing, does not recreate) | Yes | the BGP policy model (Policy, FilterRef, final, concatenation, canonicalisation) is lifted, not rewritten; redistribute's matcher and OSPF's per-source settings are deleted |
| Zero-copy preserved where applicable (refs, not copies) | Yes | BGP keeps its wire-native evaluation; the Loc-RIB gate evaluates on the Path value already in hand |
| Registration over hardcoding, outbound | Yes | filter types register through YANG `ze:filter` plus a Go registration declaring fields read; protocols register their route fields on `RouteSource` |
| Registration over hardcoding, inbound | Yes | searched lists: `BuildFilterRegistry` (list children: replaced by the marker), `prependDefaultFilters` (hardcoded `loop-detection`: becomes a registered automatic filter), `canonicalizeOne`, redistribute `ExtractRedistributeRules` family/tag parsing, OSPF `externalParams`. Each derives from the policy registry after this spec |

## Route Field Vocabulary

The field contract. A filter type registers the fields it READS and the fields
it SETS; an attachment point registers the fields its routes CARRY and the
fields it lets a filter SET. At load, a chain is refused when any filter in it
reads a field the attachment does not carry, or sets one it does not allow; the
error names the filter, the field and the attachment. The vocabulary is
declared once, in the policy component, as a registered enumeration: a protocol
or filter plugin that needs a new field registers it there, never edits a
switch (`ai/rules/principles.md`).

### Fields carried, per attachment point

`C` = carried, `-` = not carried (a filter reading it is refused).

| Field | Type | Loc-RIB: isis | Loc-RIB: ospf | Loc-RIB: static | Loc-RIB: connected | Loc-RIB: bgp best (central chain only) | redistribute import | IGP export (isis/ospf) | BGP ingress/egress |
|-------|------|---|---|---|---|---|---|---|---|
| `prefix` | prefix | C | C | C | C | C | C | C | C (per NLRI) |
| `family` | AFI/SAFI | C | C | C | C | C | C | C | C |
| `protocol` | registered source name | C | C | C | C | C (`ebgp` or `ibgp`, from `IsEBGP`) | C (`ebgp`/`ibgp` from the route, journal row) | C (the origin protocol) | C (`bgp`) |
| `next-hop` | address | C | C | C | - (none: connected names no next-hop) | C | C | C | C |
| `interface` | name | C | C | C (when configured) | C | - | - | - | - |
| `metric` | uint32 | C (SPF metric) | C (SPF cost) | C (when configured) | - | C (MED as published) | C | C | C (MED) |
| `tag` | uint32 | - (A-8: Ze's IS-IS has no RFC 5130 admin tag) | C (external route tag) | C (`tag` leaf) | - | - | C | C (OSPF only) | - |
| `level` | 1 or 2 | C | - | - | - | - | - | C (IS-IS consumer only) | - |
| `route-type` | intra-area, inter-area, external-1, external-2, nssa-1, nssa-2 | - | C | - | - | - | - | C (OSPF consumer only) | - |
| `origin-as` | uint32 | - | - | - | - | - | C (redistevents entry) | C | C (from AS_PATH) |
| `community`, `large-community`, `extended-community` | lists | - | - | - | - | - | C (batch community) | - | C |
| `as-path` | AS path | - | - | - | - | - | - | - | C |
| `local-preference`, `origin`, `aigp`, `med` | BGP attributes | - | - | - | - | - | - | - | C |

The BGP best reaching the Loc-RIB carries no communities or AS path: those
are judged by the BGP ingress chain, before best-path selection (Key Design
Decisions, "BGP import stays ingress").

### Fields a filter may set, per attachment point

| Set action | Loc-RIB import (any source, central) | redistribute import | IS-IS export | OSPF export | BGP |
|------------|---|---|---|---|---|
| `distance` (0-255) | allowed: writes `DistanceOverride` | - | - | - | - |
| `tag` | allowed: replaces the path's tag | allowed | - (no RFC 5130 admin tag, A-8) | allowed (external route tag) | - |
| `metric` | - (metric ranks within a protocol; the producer owns it) | allowed | allowed (wide metric range) | allowed (0-16777215) | `med` as today |
| `metric-type` | - | - | - | allowed (1 or 2) | - |
| `level` | - | - | allowed (1 or 2) | - | - |
| BGP attribute sets of `modify` (`local-preference`, `community-add`, ...) | - | - | - | - | allowed, unchanged |

### Fields each filter type reads and sets

| Filter type | Reads | Sets |
|-------------|-------|------|
| `prefix-list` | `prefix`, `family` | - |
| `family-filter` | `family` | - |
| `match` (new) | any of `prefix` (by prefix-list reference), `family`, `protocol`, `next-hop`, `interface`, `metric` range, `tag`, `level`, `route-type`, `origin-as`; reads only what its instance names, so one type serves every attachment | - |
| `modify` | its `match` fields (communities today; `tag`, `protocol`, `level`, `route-type` added) | its `set` fields; the new non-BGP sets are `distance`, `tag`, `metric`, `metric-type`, `level` |
| `as-path-list`, `as-path-length`, `reject-asn`, `remove-private-as`, `loop-detection` | `as-path` | `as-path` (`remove-private-as` only) |
| `community-match` | `community`, `large-community`, `extended-community` | - |
| external `<plugin>:<filter>` | its stage-1 `Attributes` declaration mapped onto this vocabulary (A-5) | as declared |

A `match` or `modify` instance's own fields are what the check reads: an
instance naming only `prefix-list` and `tag` attaches to static, IS-IS and OSPF
import, and is refused on connected import because connected carries no tag.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every Loc-RIB write passes through `(*RIB).insert`/`Remove`, forked plugins included | research; `dispatch_route.go` `applyRouteInstall` calls `InsertForward` | a producer bypasses the gate | grep for every caller of the locrib write methods in phase 2's wiring test, plus a test per producer | unvalidated |
| A-2 | sysrib's second input (`ribevents.BestChange` when it has no Loc-RIB) is not reachable in any shipped build | research: sysrib.go subscribes to BestChange only without a Loc-RIB | routes reach the FIB around the gate | read sysrib startup; if reachable, delete that input or gate it (phase 2) | unvalidated |
| A-3 | Filtering IGP routes from the local RIB is local policy and violates no requirement of RFC 2328, RFC 5340 or the IS-IS RFCs Ze implements, provided LSDB and flooding are untouched | Junos IS-IS/OSPF `import`; BIRD and FRR source read 2026-10-29 (Key Design Decisions, "Interop evidence") | an RFC MUST forbids it | implementation: read RFC 2328 Section 16 and RFC 1195 Section 3.10 text and quote; implementations: BIRD and FRR confirmed below | partly validated: BIRD and FRR confirmed, RFC text read owed in phase 2 |
| A-4 | Adding three fields to `locrib.Path` keeps the insert path allocation-free | Path is value-typed; fields are scalars | Path grows past a cache line and the BGP full-table benchmark regresses | `./le perf` on Thomas's Mac before and after phase 2 (harness prepared, owner runs it) | unvalidated |
| A-5 | `FilterRegistration.Attributes` can be generalised into the "fields this filter reads" declaration | research: stage-1 registration already declares attribute names | external plugins need a new registration field | phase 1 test over an external filter's declaration | unvalidated |
| A-6 | No shipped filter type is named `route`, so sharing `policy` with policy routing cannot collide | the ten augmenting modules list their types | a name clash | phase 1 load test with both present | unvalidated |
| A-8 | IS-IS routes carry no tag, because Ze implements no RFC 5130 administrative tag | grep 2026-10-29: no `5130` under `internal/plugins/isis`, no `rfc/short/rfc5130.md` | if Ze later implements RFC 5130, `tag` becomes carried for IS-IS import and settable on IS-IS export by a registration, not a redesign | the field declaration test refuses a tag filter on IS-IS (`TestPolicyFieldMismatchRefused`) | validated: not carried; RFC 5130 is outside every release profile, so adding it is the owner's scope call |
| A-7 | Static routes in named tables stay outside the Loc-RIB (they never enter it), so `static filter import` covers MAIN-table routes only | `unified-locrib.md` | operators expect named tables filtered | stated in the static guide; refused nothing | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The gate adds latency or allocation to every BGP best change | BGP full-table benchmark; alloc test on insert | snapshot read before the lock; a source with no chain and no central chain takes one pointer load and a nil check; alloc-free test enforced |
| R-2 | A hidden path changes ChangeRemove semantics and sysrib keeps a FIB entry for a fully filtered prefix | sysrib test with only hidden paths | "no selectable best" maps to ChangeRemove; test asserts the FIB entry is removed |
| R-3 | Re-evaluation on reload applies set actions on top of already-modified values (distance set twice, tag rewritten) | reload test applying a set twice | the gate keeps the producer's original values; set actions apply to a copy each evaluation |
| R-4 | Filtering an IGP route locally creates a blackhole or loop for transit traffic (downstream routers still route through us) | operator report | documented in the IS-IS and OSPF guides as Junos documents it; `ze doctor` warns when an IGP import chain rejects a prefix that is the next-hop of a selected route |
| R-5 | Moving `bgp policy` breaks 115 `.ci` files, 19 test configs, web pages and docs at once | build and functional red after phase 1 | phase 1 lands the move in one commit with `ze config migrate` rewriting old configs; the old path is refused with the exact migrate command |
| R-6 | Phase 4 collides with `spec-filter-wire-0-umbrella` on the text-to-ModOps seam | both specs in progress | phase 4 starts only after that spec lands or is folded in; the library never adds a text representation |
| R-7 | Concurrent filter reload and insert see a torn chain table | race test | one immutable snapshot swapped atomically; Reselect runs after publication |
| R-8 | Forked producers send routes without the new fields and filters on tag silently see 0 | wire test over the route-install RPC | the RPC fields are added in the same phase; a filter on tag against a source that does not declare tag is refused at load |
| R-9 | Redistribution semantics change: OR-over-rules becomes one chain per import rule | redistribute functional tests | each `import <source>` block owns one chain; multiple sources under a destination stay independent, as today |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | routes missing from the RIB and FIB (a filter rejects too much, or a hidden path is never re-admitted); routes kept that should be removed; existing BGP configs refused after the root move |
| How is it reverted? | per phase: each phase is its own commit series; phase 1's root move needs `ze config migrate` run backwards by hand |
| Who else touches this path? | `spec-filter-wire-0-umbrella` (BGP chain), `spec-review-redistribute-orchestrator` (`handleBatch`), `spec-single-owner-reload-reconcile` (redistribute reload), `spec-route-source-attribute` (source identity), `spec-bgp-filtered-route-storage` (consumes the import reject verdict) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| config `policy { prefix-list X }` + `bgp filter import { filters [ X ] }` | → | policy registry, BGP chain compile | `test/plugin/policy-root-bgp-import.ci` |
| config with `bgp policy { ... }` | → | load refusal naming the new path | `test/parse/policy-root-old-path-refused.ci` |
| config `isis { filter { import { filters [ X ]; final accept; } } }` | → | Loc-RIB gate for IS-IS source | `test/isis/isis-import-filter-hides-prefix.ci` |
| config `rib { filter { import ... } }` | → | central chain on every source | `TestLocRIBCentralChainAllSources` in `internal/core/rib/locrib/policy_test.go` |
| forked static plugin route install with tag | → | `applyRouteInstall` → gate | `TestRouteInstallTagReachesGate` in `internal/component/plugin/server/dispatch_route_test.go` |
| config `isis { filter { export ... } }` | → | IS-IS `InjectRoute` export chain | interop `isis-export-policy-frr` |
| config `redistribute { destination ospf { import static { filters [ X ]; final reject; } } }` | → | `handleBatch` chain | `test/plugin/redistribute-chain-static-ospf.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config defines `policy { prefix-list P { ... } }` and a BGP peer's import references `P` | BGP import behaves exactly as with the same list under `bgp policy` before the move (same accepted and rejected prefixes) |
| AC-2 | Config still uses `bgp policy { ... }` | load fails; the message names the new `policy` path and the `ze config migrate` command; `ze config migrate` rewrites the file and the result loads |
| AC-3 | `policy { route R { ... } }` (policy routing) beside `policy { prefix-list P { ... } }` | both load; policy routing output is unchanged; `R` is not a filter instance, and a filter chain referencing `R` is refused naming it as a policy-routing rule, not a filter |
| AC-4 | IS-IS learns 10.0.0.0/24 and 10.1.0.0/24; `isis filter import` references a prefix-list accepting only 10.0.0.0/24, `final reject` | 10.0.0.0/24 is installed in the RIB and FIB; 10.1.0.0/24 is absent from `show rib` and the FIB, appears as hidden in `show rib candidates`, and is still in the IS-IS LSDB and still flooded to neighbours |
| AC-5 | AC-4, then the filter is relaxed to accept both and the config is committed | 10.1.0.0/24 is installed without any IS-IS event (no SPF run needed) |
| AC-6 | A prefix is learned by OSPF (accepted) and static (rejected by `static filter import`) | the OSPF path is selected; the static path is hidden; removing the reject makes static win on distance |
| AC-7 | `rib filter import` rejects 192.0.2.0/24; BGP, OSPF and static each offer it | no source's path is selected; `show rib` does not list the prefix; every path is hidden |
| AC-8 | Every path for a prefix becomes hidden after a reload | the FIB entry is removed (ChangeRemove reaches sysrib) |
| AC-9 | A `modify` instance with `match { tag [ 100 ]; } set { distance 90; }` on `ospf filter import` | OSPF external routes tagged 100 rank at distance 90; others at the declared OSPF distance; removing the filter returns them to the declared distance. The same instance on `isis filter import` is refused at load (IS-IS carries no tag, A-8) |
| AC-10 | A community-match filter referenced from `isis filter import` | load fails naming the filter, the field it reads (community), and that IS-IS routes do not carry it |
| AC-11 | A static route with `tag 4242`, `rib filter import` with a `match { tag [ 4242 ]; }` reject | the static route is hidden, whether static runs in-process or forked |
| AC-12 | `isis filter export` accepts only /32 loopbacks; IS-IS redistributes connected | the IS-IS LSP carries only the loopback /32s; an FRR neighbour installs those and nothing else |
| AC-13 | `ospf filter export` sets metric 50 and metric-type 1 for static routes matching a tag | the OSPF AS-external LSA carries metric 50, E1; `ospf redistribute { metric ... }` no longer exists and is refused with the migrate command |
| AC-14 | `redistribute { destination ospf { import bgp { filters [ P ]; final reject; } } }` | only prefixes P accepts move into OSPF; a route that was held and is now rejected after a reload is withdrawn |
| AC-15 | `redistribute { destination ospf { import ebgp } }` | only eBGP-learned best paths move; iBGP-learned ones do not (fixes the journal row) |
| AC-16 | A source with no chain and no central chain configured | insert performs zero allocations and the gate is one pointer load (alloc test) |
| AC-18 | A config with only a `policy {}` section; a config with only `isis {}` (no policy section); a build without the `policy` plugin and an `isis filter import` | the first auto-loads `policy`; the second loads `policy` as a dependency of `isis`; the third is refused at load and by `ze config validate` with the missing-dependency message naming `policy` and the rebuild fix. A registry-derived test asserts every plugin with a filter attachment lists `policy` in `Dependencies` |
| AC-17 | Phase 4: BGP import and export chains compile from the shared registry | every existing BGP policy `.ci` and interop scenario passes unchanged in expectation |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Installs only some IS-IS prefixes | config → registry → snapshot → IS-IS SPF install → gate → Loc-RIB hidden/selected → sysrib → FIB | `test/isis/isis-import-filter-hides-prefix.ci` |
| 2 | Blocks bogons from every protocol | config → central chain → gate for every source | `test/plugin/rib-import-central-bogons.ci` |
| 3 | Advertises only loopbacks into IS-IS | config → IS-IS export chain → LSP | interop `isis-export-policy-frr` |
| 4 | Sets OSPF external metric per route | config → OSPF export chain set actions → AS-external LSA | interop `ospf-export-policy-frr` |
| 5 | Upgrades a config with `bgp policy` | `ze config migrate` → new root → load | `test/parse/policy-root-migrate.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPolicyRegistryDiscoversByMarker` | `internal/component/policy/registry_test.go` | only `ze:filter` lists are filter types; `route` is not | |
| `TestPolicyRegistryDuplicateName` | `internal/component/policy/registry_test.go` | one name under two types refused | |
| `TestPolicyFieldMismatchRefused` | `internal/component/policy/fields_test.go` | AC-10 | |
| `TestLocRIBHiddenPathNotSelected` | `internal/core/rib/locrib/policy_test.go` | hidden skipped by `selectBest` and `siblingNextHops` | |
| `TestLocRIBAllHiddenDispatchesRemove` | `internal/core/rib/locrib/policy_test.go` | AC-8 | |
| `TestLocRIBReselectReadmitsHidden` | `internal/core/rib/locrib/policy_test.go` | AC-5 | |
| `TestLocRIBSetActionsFromOriginal` | `internal/core/rib/locrib/policy_test.go` | R-3: reload twice gives the same distance and tag | |
| `TestLocRIBCentralChainAllSources` | `internal/core/rib/locrib/policy_test.go` | AC-7 | |
| `TestLocRIBSetDistanceByTag` | `internal/core/rib/locrib/policy_test.go` | AC-9: tagged OSPF path ranks at the set distance, untagged at the declared one, and back after the filter is removed | |
| `TestLocRIBInsertNoChainZeroAlloc` | `internal/core/rib/locrib/policy_test.go` | AC-16 | |
| `TestLocRIBSnapshotRace` | `internal/core/rib/locrib/policy_test.go` | R-7 under `-race` | |
| `TestRouteInstallTagReachesGate` | `internal/component/plugin/server/dispatch_route_test.go` | AC-11 forked | |
| `TestRedistributeChainAccept` | `internal/component/config/redistribute/route_test.go` | AC-14 | |
| `TestRedistributeEBGPClass` | `internal/component/bgp/plugins/redistribute_egress/redistribute_test.go` | AC-15 | |
| `TestOSPFExportSetMetric` | `internal/plugins/ospf/redistribute/consumer_test.go` | AC-13 | |
| `TestISISExportChain` | `internal/plugins/isis/redistribute/consumer_test.go` | AC-12 unit | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `match tag` | 0-4294967295 | 4294967295 | N/A | 4294967296 |
| `set distance` | 0-255 | 255 | N/A | 256 |
| `set metric` (OSPF export) | 0-16777215 | 16777215 | N/A | 16777216 |
| `set metric` (IS-IS export, wide) | 0-4261412864 | 4261412864 | N/A | 4261412865 |
| `set metric-type` (OSPF) | 1-2 | 2 | 0 | 3 |
| `set level` (IS-IS) | 1-2 | 2 | 0 | 3 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `policy-root-bgp-import` | `test/plugin/policy-root-bgp-import.ci` | AC-1 | |
| `policy-root-old-path-refused` | `test/parse/policy-root-old-path-refused.ci` | AC-2 refusal | |
| `policy-root-migrate` | `test/parse/policy-root-migrate.ci` | AC-2 migrate | |
| `policy-root-shared-with-pbr` | `test/parse/policy-root-shared-with-pbr.ci` | AC-3 | |
| `isis-import-filter-hides-prefix` | `test/isis/isis-import-filter-hides-prefix.ci` | AC-4, AC-5 | |
| `rib-import-central-bogons` | `test/plugin/rib-import-central-bogons.ci` | AC-7 | |
| `static-import-filter-distance` | `test/plugin/static-import-filter-distance.ci` | AC-6 | |
| `redistribute-chain-static-ospf` | `test/plugin/redistribute-chain-static-ospf.ci` | AC-14 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `isis-import-policy-frr` | `test/interop/scenarios/` | FRR | AC-4: FRR's prefix is in Ze's LSDB, flooded on to a second FRR, and absent from Ze's FIB | |
| `isis-export-policy-frr` | `test/interop/scenarios/` | FRR | AC-12 | |
| `ospf-export-policy-frr` | `test/interop/scenarios/` | FRR | AC-13 | |
| `ospf-import-policy-bird` | `test/interop/scenarios/` | BIRD 3 | AC-9 with tagged OSPF externals learned from BIRD: the tagged prefix is installed at the set distance and loses to a static route it would otherwise beat | |

## Files to Modify
- `internal/core/rib/locrib/candidate.go`, `entry.go`, `manager.go`, `distance.go` - hidden flag, tag/level/route-type fields, gate, re-evaluation in Reselect
- `internal/component/bgp/config/filter_registry.go` - moves to `internal/component/policy/` and discovers by `ze:filter`
- `internal/component/bgp/config/redistribution.go`, `peers.go` - policy model and attachment extraction move to the library; `prependDefaultFilters` derives automatic filters from registration
- `internal/component/bgp/filterapi/policy.go` - protocol-neutral parts move; the RFC 8212 default stays a BGP attachment property
- `internal/component/bgp/yang/ze-bgp-conf.yang` - `policy` container removed
- the ten modules augmenting `/bgp:bgp/bgp:policy` - augment `/policy` instead; each plugin's `config.go` reads the top-level root
- `internal/component/config/yang/modules/ze-extensions.yang` - `ze:filter` description matches behavior
- `internal/plugins/policyroute/yang/ze-policyroute-conf.yang` - `policy` container shared (owned by the policy component)
- `internal/component/sysrib/yang/ze-rib-conf.yang` - `rib filter import`
- `internal/plugins/isis/yang/`, `internal/plugins/ospf/yang/`, `internal/plugins/static/yang/`, `internal/plugins/connected/yang/` - `filter` containers
- `internal/plugins/isis/spf/install.go`, `internal/plugins/ospf/spf/install.go`, `internal/plugins/static/locrib.go`, `internal/plugins/connected/locrib.go` - populate tag/level/route-type on the Path
- `pkg/plugin/rpc/types.go` (`RouteInstallInput`), `internal/component/plugin/server/dispatch_route.go` - carry the new fields
- `internal/component/config/redistribute/route.go`, `evaluator.go`, `registry.go`, `internal/component/config/loader_redistribute.go` - chains replace family/tag; `RouteSource` declares fields
- `internal/component/config/redistribute/yang/ze-redistribute-conf.yang` - `filters`/`final` replace `family`/`tag`
- `internal/component/bgp/plugins/redistribute_egress/redistribute.go`, `replay.go` - evaluate chains; BGP class from the route
- `internal/component/bgp/redistribute/producer.go` - carries the eBGP/iBGP class per entry
- `internal/plugins/isis/redistribute/consumer.go`, `internal/plugins/ospf/redistribute/consumer.go`, `redist_wiring.go` - export chains; OSPF per-source settings deleted
- `internal/component/sysrib/sysrib.go` - `show rib candidates` (hidden paths); A-2 second input
- `internal/component/bgp/plugins/filter_modify/` - `set distance`, `set tag`, `match tag`, `match protocol`, `match prefix-list` for non-BGP routes
- `internal/component/web/` BGP policy pages - read the new root
- ExaBGP migration (`ze exabgp migrate`) - no change (it emits no `policy`); verified, not edited
- `cmd/ze/config/migrate` - rewrite `bgp policy`, `redistribute family/tag`, `ospf redistribute` settings

## Files to Create
- `internal/component/policy/` - registry, field declarations, chain compile, route view interface (`register.go`, `registry.go`, `fields.go`, `chain.go`, `view.go`, `yang/ze-policy-conf.yang`)
- `internal/core/rib/policy/` - the leaf seam the Loc-RIB reads (compiled chain snapshot, atomic pointer)
- `internal/component/policy/plugins/match/` - the generic `match` filter type (prefix-list, tag, protocol, family, metric range, next-hop)
- `test/...` - functional tests listed above; interop scenario directories listed above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/policy/yang/ze-policy-conf.yang`; `filter` containers in isis/ospf/static/connected/rib YANG; redistribute YANG |
| YANG validation constraints | Yes | ranges in the Boundary table; filter references validated against the registry |
| YANG custom validators | Yes | `ze:validate "policy-filter-ref"` with `CompleteFn` listing instances whose fields fit the attachment's protocol |
| CLI commands/flags | Yes | `show rib candidates [prefix]` showing hidden paths and the chain that hid them |
| CLI grammar (keyword before value) | Yes | `show rib candidates prefix <p>` |
| Editor autocomplete | Yes | filter names by attachment, via `CompleteFn` |
| Functional test for new RPC/API | Yes | `test/plugin/rib-import-central-bogons.ci` exercises `show rib candidates` |
| Pipe completeness | Yes | `show rib candidates` through `ApplyPipes` |
| Env var registration | No | no environment leaf added |
| Doctor check for runtime dependencies | Yes | no runtime dependency; a diagnostic for R-4 (`doctor-policy-igp-import-hides-nexthop`) in `internal/core/diagnostic/codes.go`, with unit and functional test |
| Prometheus counters/metrics | Yes | `ze_rib_policy_hidden_paths{source}` gauge, `ze_rib_policy_rejected_total{source,chain}` counter, `ze_redistribute_chain_rejected_total{destination,source}` replacing `ze_bgp_redistribute_filtered_rule_total` |
| BGP family surface (new SAFI / capability / attribute) | N-A | no family, capability or attribute added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/` entry for route policy (new) and the BGP policy entry |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`show rib candidates`) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (route-install fields) |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` (filter types move; `match` type) |
| 6 | Has a user guide page? | Yes | new `docs/guide/route-policy.md`; `docs/guide/bgp-policy.md`, `redistribution.md`, `isis.md`, `ospf.md`, `static-routes.md`, `policy-routing.md` |
| 7 | Wire format changed? | No | LSP/LSA formats unchanged; only which prefixes and metrics are written |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md`, `ai/rules/plugins.md` (route-install fields, filter field declaration) |
| 9 | RFC behavior implemented, changed, or newly proven? | No | local policy (A-3) |
| 10 | Test infrastructure changed? | No | existing runners |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (per-protocol import/export policy vs Junos, FRR, BIRD) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/rib/unified-locrib.md` (gate, hidden paths), `docs/architecture/core-design.md` |
| 13 | Route metadata keys added/changed? | No | Loc-RIB path fields, not route metadata keys |
| 14 | Prometheus counters added/changed? | Yes | subsystem telemetry doc for the counters above |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived per phase by `./le spec citation anchors spec plan/post-release/features/spec-route-policy-0-umbrella.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | every `bgp { policy { ... } }` example in docs and website moves to `policy { ... }` |

## Implementation Steps

1. **Phase 1: Library and policy root move**
   - Tests: `TestPolicyRegistryDiscoversByMarker`, `TestPolicyRegistryDuplicateName`, `TestPolicyFieldMismatchRefused`, `policy-root-*` functional tests, AC-1..AC-3, AC-10
   - Files: `internal/component/policy/`, the ten filter-type YANG modules and their `config.go`, BGP config extraction, policy routing YANG, web pages, migrate, docs and `.ci`/test config rewrite
   - Verify: every BGP policy test passes with the new root; the old path is refused; policy routing unchanged
2. **Phase 2: RIB import for IGP, static, connected and the central chain**
   - Tests: Loc-RIB unit tests, `TestRouteInstallTagReachesGate`, `isis-import-filter-hides-prefix`, `rib-import-central-bogons`, `static-import-filter-distance`, interop `isis-import-policy-frr`, `ospf-import-policy-bird`; AC-4..AC-11, AC-16
   - Files: locrib, `internal/core/rib/policy/`, producers, route-install RPC, sysrib `show rib candidates`, `match` filter type, `modify` set distance/tag, YANG `filter` containers, doctor check
   - Verify: alloc test green; perf harness prepared for Thomas (A-4)
3. **Phase 3: Export and redistribution on the library**
   - Tests: `TestRedistributeChainAccept`, `TestRedistributeEBGPClass`, `TestOSPFExportSetMetric`, `TestISISExportChain`, `redistribute-chain-static-ospf`, interop `isis-export-policy-frr`, `ospf-export-policy-frr`; AC-12..AC-15
   - Files: redistribute config/evaluator/YANG/loader, orchestrator, BGP producer class, IS-IS and OSPF consumers, OSPF redistribute YANG deletion, migrate
   - Verify: depends on `spec-fixit-redistribution-chain-drops-silently` closed and green chain scenarios
4. **Phase 4: BGP on the library**
   - Tests: AC-17, every existing BGP policy `.ci` and `bgp-policy-*` interop scenario
   - Files: BGP filter chain compile through the library's registry; lazy route view over the wire UPDATE; sequenced with `spec-filter-wire-0-umbrella`
   - Verify: BGP throughput benchmark prepared for Thomas; no new text representation

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | hidden paths never reach `Best`, ECMP, sysrib or the FIB; set actions apply to the producer's original values on every evaluation; an IGP import filter never edits the LSDB |
| Naming | `filter import`/`export`, `filters`, `final` identical on every attachment; filter type names unchanged by the move |
| Data flow | the Loc-RIB reads only the compiled-chain seam; config never imported by locrib |
| Rule: no-layering | no `bgp policy` alias, no redistribute family/tag beside chains, no OSPF per-source settings beside the export chain |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| top-level policy registry | `grep -rn 'augment "/bgp:bgp/bgp:policy"' internal` returns nothing |
| Loc-RIB gate | `TestLocRIBCentralChainAllSources` passes |
| hidden paths visible | `show rib candidates` in `rib-import-central-bogons.ci` |
| redistribute chains | `grep -n MatchTag internal/component/config/redistribute` returns nothing |
| IGP export | interop `isis-export-policy-frr` and `ospf-export-policy-frr` green with recorded red |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | every filter reference resolves at load; a chain referencing an unknown or field-incompatible filter is refused, never skipped |
| Fail-closed | a filter error on import hides the route (fail closed) unless the filter type declares fail-open; documented per type |
| Resource exhaustion | hidden paths count toward the Loc-RIB; the hidden-paths gauge exposes them; a filter rejecting a full table keeps memory bounded by what producers offer |

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

- The registry's "every list child is a filter type" scan is what made the shared root look impossible; reading the `ze:filter` marker, which the extension already documents, removes the collision.
- Hidden paths turn filter reload into a local Reselect. Without them, relaxing a filter needs a replay call from five producers that do not share one.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/silent-fall-through.md` | 2026-10-29 | redistribute `import ibgp` / `import ebgp` never match |
| `plan/journal/comment-describes-superseded-behaviour.md` | 2026-10-29 | `ze:filter` marker documented as discovery, never read |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One gate at Loc-RIB insert | a filter call in each of five producers plus the RPC | one choke point every source already passes; producers cannot forget it |
| Hidden paths | drop rejected paths and replay from producers on reload | no common replay exists; hidden paths make reload local and match Junos |
| Share `policy` root with policy routing | move PBR to its own root; name the new root `policy-options` | keeps the approved syntax and policy routing unchanged; the marker removes the collision |
| Keep chain plus `final` | Junos `term from/then` | the model ships, is documented and tested; a generic `match` type gives term-like conditions |
| BGP import stays ingress | move BGP import to the Loc-RIB gate | BGP import must see communities and AS path and decide Adj-RIB-In admission; the Loc-RIB sees one best per prefix |
| The policy library is an in-process plugin `policy`: `ConfigRoots ["policy"]` auto-loads it from a `policy {}` section, and every protocol with a filter attachment (bgp, isis, ospf, static, connected, rib, the redistribute orchestrator) and every filter-type plugin lists it in hard `Dependencies`, so loading any protocol loads it (owner, 2026-10-30) | optional dependency; a plain compiled-in component | a hard edge makes a build without it refuse a `filter` attachment with the registry's missing-dependency message instead of ignoring it; it runs in the engine because the Loc-RIB gate is there; a test derives the protocols with attachments from the registry and asserts each edge |
| IGP import never touches the LSDB | filter LSPs/LSAs on receipt | the LSDB must stay identical across the area; Junos, FRR and BIRD filter after SPF |
| Interop evidence: BIRD | - | BIRD `nest/rt-table.c` `rte_update2` runs the channel's `in_filter` on every route any protocol (OSPF via `proto/ospf/rt.c` `rte_update`) offers the table; on reject it drops the route, or with `in_keep_filtered` keeps it flagged `REF_FILTERED`. That is the gate-at-RIB-entry plus hidden-path design here |
| Interop evidence: FRR | - | FRR `zebra/zebra_routemap.c` `zebra_route_map_check` applies the route-map named for the route's protocol (`ip protocol <proto> route-map`), falling back to the `any` route-map only when no per-protocol one exists; called from `zebra_nhg.c` nexthop resolution, so a denied route stays in the RIB inactive. Ze differs deliberately: the source chain and the central chain both run, in that order, because the owner chose both attachments |

## Known Limitations
- Static routes in named tables never enter the Loc-RIB, so `static filter import` covers the main table only (A-7). Named tables reaching the Loc-RIB is not in scope here.
- IS-IS route tags need RFC 5130, which Ze does not implement and no release profile lists (A-8). The vocabulary marks `tag` not carried for IS-IS, so a tag filter on IS-IS is refused at load rather than silently matching 0.
- Storage of BGP routes rejected by import (BIRD `import keep filtered`) stays in `plan/post-release/features/spec-bgp-filtered-route-storage.md`.
- Source identity as a registered enum stays in `plan/post-release/features/spec-route-source-attribute.md`; phase 3 derives the BGP class from the route and coordinates with it.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT. The IS-IS and OSPF
export chains write into LSP TLVs and AS-external LSAs: the metric and
metric-type set actions quote RFC 5305 Section 4 and RFC 2328 Section A.4.5 at
the encoding sites they change.

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
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits (`/ze-close` step 6f). The full verification is Thomas's release check: no agent runs it, and one that is owed is handed to him as its exact command. A red there gets a fix commit
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

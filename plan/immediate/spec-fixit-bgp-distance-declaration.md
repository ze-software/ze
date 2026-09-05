# Spec: bgp distance declaration

| Field | Value |
|-------|-------|
| Status | done |
| Scope | config |
| Depends | - |
| Phase | 7/7 |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Administrative distance is declared in three places and they do not arbitrate.
`bgp { admin-distance { ebgp 20; ibgp 200; } }` and `rib { admin-distance {
connected 0; static 10; ebgp 20; ospf 110; isis 115; ibgp 200; } }` each carry
the eBGP and iBGP numbers as YANG defaults, and `extractAdminDistanceConfig`
(`internal/component/bgp/plugins/rib/rib_admin_distance_config.go`) returns
`(0, 0)` for an absent block so that its caller keeps a third copy.

`(*sysRIB).effectivePriority` (`internal/component/sysrib/sysrib.go`) is where
they meet. When its map holds a key for the protocol it returns the configured
value and DISCARDS the distance BGP stamped. When the map is empty it returns
the stamped value. `parseAdminDistanceConfig` returns an empty map when the
config carries no `rib` section, and YANG defaults never fill it, because
`config.ApplyDefaults` (`internal/component/config/schema_defaults.go`) is
called from two sites only, both on a peer entry.

So an operator who writes no `rib` block gets BGP's knob, and an operator who
wrote one for OSPF silently loses it. No log line marks either case. That is a
value that is silently wrong rather than a tidiness problem
(`ai/rules/principles.md`).

Goal: one declaration of every protocol's administrative distance, always
populated, with nothing discarded in silence; and one spelling, `distance`, on
every surface an operator or a plugin author can see, so the `bgp { defaults {
... } }` block that follows this spec has no second copy inside it and no
second name beside it.

### Phase 1 audit: the rename boundary (2026-09-04, complete)

The name appears on three surfaces. The first two are renamed and the third is
not.

| # | Surface | What it is | Files | Renamed |
|---|---------|-----------|-------|---------|
| 1 | Config leaf | the container an operator writes: `ze-rib-conf.yang` and `ze-bgp-conf.yang`, the two Go parsers reading the quoted key (`sysrib.go`, `rib_admin_distance_config.go`), `test/isis/isis-redist-arbitration.ci`, `test/ospf/ospf-redist-arbitration.ci`, and the doc pages showing the block | 2 YANG, 2 Go, 2 `.ci`, ~8 docs | YES |
| 2 | Operator and plugin facing | `AdminDistance uint8 \`json:"admin-distance"\`` (`pkg/plugin/rpc/types.go:849`), `OperationSetAdminDistance ConfigOperationType = "set-admin-distance"` (`:1084`), its re-export (`pkg/plugin/sdk/sdk_types.go:108`), the `show` JSON key (`rib_commands.go:502`), the CLI column header (`internal/component/bgp/plugins/cmd/rib/rib.go:229`) | 4 | YES (owner decision, 2026-09-04) |
| 3 | RIB concept | the Go identifier `AdminDistance` as the protocol-independent selection key across `internal/core/rib/locrib/`, `internal/core/rib/routeinstall/`, `internal/component/sysrib/`, `internal/plugins/isis/`, `internal/plugins/ospf/` | 35 | NO |

Surface 2 is why the rename is not internal. `json:"admin-distance"` is a field
an out-of-tree plugin decodes and `"set-admin-distance"` is a transaction
operation string, so this change breaks the plugin SDK. Pre-release makes that
free (`CLAUDE.md`: no users of `main`), and the owner took the decision with
that on the table.

Surface 3 stays because `locrib/candidate.go` documents `AdminDistance` as "the
protocol's trustworthiness rank": it is the quantity, not the config leaf, and
renaming it buys nothing and buries the real change in 35 files.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - how a YANG default reaches a
      running config, which is the mechanism this spec repairs
  → Decision: the page describes the parse path and is SILENT on which sections
    get `ApplyDefaults` applied, so it neither states nor contradicts the hole.
    It gains the rule this spec settles.
  → Constraint: a leaf's `default` is schema metadata, not a runtime value. A
    consumer reading a config section directly sees the leaf missing.
- [ ] `docs/architecture/plugin/rib-storage-design.md` - declared by
      `rib_admin_distance_config.go` as its design page
  → Constraint: it documents the BGP-side extraction this spec deletes, so the
    paragraph is rewritten rather than dropped.
- [ ] `docs/architecture/core-design.md` - declared as the design page by
      `internal/component/sysrib/sysrib.go`, whose `effectivePriority` this spec
      changes
  → Decision: the page describes the cross-protocol RIB and is SILENT on where a
    protocol's distance is declared, so it neither states nor contradicts the
    duplication. It gains one sentence naming the single declaration, and a
    pointer rather than a second copy of the six numbers
    (`ai/rules/principles.md`).
  → Constraint: distance-then-metric is the selection order the page already
    describes, and this spec does not change it.
- [ ] `docs/architecture/isis/isis-9-spf-rib.md` - declared by
      `internal/plugins/isis/spf/install.go`, whose comments cite
      `rib.admin-distance.isis` and `effectivePriority`
  → Decision: ISIS is the second consumer of the declaration this spec renames,
    so the page's distance references move with it.
  → Constraint: ISIS reaches its distance through the same `effectivePriority`
    that BGP does, which is the evidence that one declaration serves every
    protocol and BGP's private container was the exception.
- [ ] `ai/CODE-TO-DOCS.md` for `internal/component/sysrib/` and
      `internal/component/bgp/plugins/rib/`
  → Decision: DERIVED. Run `./le spec citation anchors spec
    plan/immediate/spec-fixit-bgp-distance-declaration.md` and answer every page named.

### RFC Summaries (Scope: config)
- [ ] N-A. Administrative distance is not an RFC concept, and
      `ze-bgp-conf.yang` already says so: "The defaults follow the Cisco and
      Juniper convention. RFC 4271 mandates no values."
  → Constraint: that sentence is why the rename is safe. No RFC names the
    quantity, so no RFC text is contradicted by calling it `distance`.

**Key insights:**
- The rename and the deduplication are one change. Deleting the BGP container
  is what leaves a single name to rename.
- `effectivePriority` already has the right shape. What it lacks is a map that
  is always populated.
- `AdminDistance` names two different things in this tree, and only one of them
  is a config leaf.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/sysrib/sysrib.go` - `parseAdminDistanceConfig` returns
      an empty map for an absent `rib` or `admin-distance` key.
      `(*sysRIB).effectivePriority` returns the incoming priority when the map
      is empty, the configured value when the key is present, and the incoming
      priority when the map is non-empty but lacks the key.
- [ ] `internal/component/bgp/plugins/rib/rib_admin_distance_config.go` -
      `extractAdminDistanceConfig` returns `(0, 0)` for an absent block, and its
      doc comment states the caller then keeps 20 and 200. Read `rib.go` for
      where that third copy lives.
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - stamps
      `locrib.Path.AdminDistance` from `r.adminDistanceEBGP` or
      `r.adminDistanceIBGP` per the eBGP/iBGP class.
- [ ] `internal/component/sysrib/register.go` - the verify, configure and
      rollback sites that load `s.adminDist`, and `reapplyAdminDistances`.
- [ ] `internal/core/bgp/routeaction/routeaction.go` - `ProtocolType.String()`
      returns "ebgp" and "ibgp", the same strings the RIB leaves use, which is
      why the override reaches BGP routes at all.
- [ ] `internal/component/config/schema_defaults.go` - `ApplyDefaults`, and its
      two callers in `internal/component/bgp/config/peers.go`.
- [ ] `internal/component/sysrib/yang/ze-rib-conf.yang` and
      `internal/component/bgp/yang/ze-bgp-conf.yang` - the two containers.
- [ ] `internal/core/rib/locrib/entry.go`, `candidate.go` - `AdminDistance` as
      the protocol-independent selection key, which this spec does NOT rename.
- [ ] `internal/plugins/isis/spf/install.go` - comments citing
      `rib.admin-distance.isis` and `effectivePriority`, showing how a second
      protocol expects this to work.

**Behavior to preserve:**
- The six numbers: connected 0, static 10, ebgp 20, ospf 110, isis 115,
  ibgp 200.
- Cross-protocol selection order, which stays distance first and metric second.
- `reapplyAdminDistances` recomputing every stored route on a reload, and the
  rollback path restoring the previous map.
- The `AdminDistance` field on `locrib.Path` and every RIB, ISIS and OSPF
  identifier spelling the protocol-independent concept.

**Behavior to change:**
- `bgp { admin-distance { } }` no longer exists, and BGP no longer stamps a
  distance from its own config.
- `rib { admin-distance { } }` becomes `rib { distance { } }`.
- The distance map is populated whether or not the operator wrote the block.
- A protocol whose distance cannot be resolved is logged rather than silently
  taking whatever the producer stamped.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator writes, or omits, `rib { distance { ... } }`, and a BGP best path
  is later mirrored into the shared Loc-RIB.
- Format at entry: the config tree as `map[string]any`, and a
  `ribevents.BestChangeEntry` carrying a protocol type and a priority.

### Transformation Path
1. The config section reaches sysrib's verify and configure callbacks.
2. The distance map is built, and after this spec it carries all six protocols
   whether the block was written or not.
3. BGP inserts a `locrib.Path` whose `AdminDistance` is no longer set from BGP
   config.
4. `effectivePriority` resolves the protocol's distance from the one map.
5. `locrib` selects the best path on distance, then metric.
6. The FIB plugins install what sysrib publishes.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config file to sysrib | a config section's JSON, read by the verify and configure callbacks | No |
| BGP rib plugin to Loc-RIB | `locrib.Path.AdminDistance` on insert | No |
| sysrib to FIB plugins | `sysribevents.BestChangeBatch` after arbitration | No |
| Operator to daemon | a config reload changing a distance, and its rollback | No |

### Integration Points
- `effectivePriority`, which already resolves a protocol to a distance and
  needs only a map that is always complete.
- `reapplyAdminDistances`, which already recomputes stored routes on a change.
- `ProtocolType.String()`, which already produces the leaf names.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| Registration over hardcoding | No | |
| One declaration of one fact | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `effectivePriority` is the only place a protocol's distance is resolved for cross-protocol selection | `gopls references` 2026-09-04: two non-test call sites, `sysrib.go:411` and `:492`, plus three in its own test. No other reader of `s.adminDist` | A second resolver would keep the duplication this spec exists to remove | done | confirmed |
| A-2 | No in-tree config or test depends on the empty-map path, where the BGP-stamped value survives | BROKEN. 131 configs under `test/`, `demos/` and `contrib/` write a `rib {` block and only 2 name a distance, so 129 were taking the empty-map branch and now take the declared map | Closing the hole changes what those configs install, and a `.ci` expectation moves | done | broken |
| A-3 | The name `admin-distance` appears on THREE surfaces, not two, and only the internal Go identifier stays | Phase 1 audit, 2026-09-04, recorded below | A rename crossing into the RIB concept touches 35 files for no gain and buries the real change | The phase 1 audit, complete | confirmed |
| A-4 | Two YANG modules can be renamed with no migration path | `CLAUDE.md`: "Ze is PRE-RELEASE... There is NO release, NO version, NO tag, and NO user of `main`" | A shipped config would break on upgrade | The owner's standing pre-release directive | confirmed |
| A-5 | The config validator refuses an unknown container, so the deleted `bgp { admin-distance }` spelling is an error rather than a silence | BROKEN. `walkTree` (`internal/component/config/yang/validator.go:620`) iterates `entry.Dir`, the SCHEMA's children, and checks each against the data; it never iterates the data. Its own comment says "unknown fields from other modules are silently skipped", and `validators.go:731` states "nothing in the config walk emits ErrTypeUnknown". The parsers DO error on an unknown field (`parser_freeform.go:132`, `parser_list.go:106`) but `loader.go:275` says parse "records each unknown field as a warning and PRUNES it from the tree", and which path this block takes is unestablished | An operator's old config loads with the block ignored and their distance silently not applied | `TestAdminDistanceIsRetiredNotSilent`, written before the deletion | broken |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The rename crosses into the RIB concept and becomes a 67-identifier diff | The diff touches `internal/core/rib/locrib/` or `internal/plugins/isis/` | A-3's boundary is drawn BEFORE the first edit, and the audit names which files are in scope |
| R-2 | Closing the empty-map hole changes what an existing config installs | A `.ci` or interop expectation about an installed route goes red | A-2's grep runs first. A red here is a behavior change to state in the commit body, not a test to weaken |
| R-3 | BGP loses its distance entirely if the deletion lands before the RIB side is populated | Every BGP route installs at distance 0 | The phases are ordered so the RIB declaration is complete and populated BEFORE the BGP container is deleted |
| R-4 | A config carrying the old spelling loads with the leaf ignored | An operator's distance stops applying with no error | A-5 is validated by a test written before the deletion, and the refusal is added if it is missing |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Which route the kernel installs when two protocols hold the same prefix. A wrong distance silently prefers a BGP route over a connected one, or the reverse, on every prefix both carry |
| How is it reverted? | A single commit revert. Config files naming `distance` would need the old spelling back, which is why the rename lands in one commit with the code |
| Who else touches this path? | ISIS already reads `rib.admin-distance.isis` through `effectivePriority` (`internal/plugins/isis/spf/install.go`). `spec-bgp-attribute-defaults` adds the sibling `attribute` container under `bgp { defaults { } }` and depends on this spec landing first. That spec closed on 2026-09-04, so read `internal/component/bgp/yang/ze-bgp-conf.yang` and `docs/config-reference.md` rather than the spec |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An operator starts ze with a config carrying NO `rib { distance { } }` block, and a BGP eBGP route competes with a static route for one prefix | → | `runSysRIBPlugin` seeds the map and the seam from the declaration before any section arrives, so all six protocols resolve and `effectivePriority` returns 20 against 10 | `TestRunSysRIBPluginSeedsTheDeclarationAtStart` for the daemon start path, `TestDistanceMapIsPopulatedWithoutAConfigBlock` for the resolved map |
| An operator writes `rib { distance { ebgp 250; } }` and an eBGP route competes with an OSPF route | → | the declaration reaches the BGP stamp through `internal/core/rib/distance`, and `locrib.selectBest` then ranks OSPF's 110 above it | `TestBgpStampsTheDeclaredDistanceNotItsOwn` at the stamp and `TestRaisedEbgpDistanceLetsOspfWin` at the selection. `test/plugin/distance-ebgp-override.ci` covers the config surface only and its header says so |
| An operator writes the old `bgp { admin-distance { ebgp 30; } }` spelling | → | the config validator refuses the unknown container | `TestAdminDistanceIsRetiredNotSilent` |
| A config reload changes `rib { distance { ebgp } }` | → | `reapplyAdminDistances` recomputes every stored route and the FIB is republished | `TestDistanceReloadRecomputesStoredRoutes` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A config carrying no `rib` section at all | The distance map carries all six protocols at their declared values, and `effectivePriority` resolves each without falling back to an incoming value |
| AC-2 | A config carrying `rib { distance { ospf 90; } }` and nothing else | ospf resolves to 90 and the other five resolve to their declared values |
| AC-3 | A BGP eBGP best path reaches the Loc-RIB | Its distance comes from the single declaration, and no BGP-side config supplies one |
| AC-4 | `bgp { admin-distance { ebgp 20; } }` appears in a config | The operator is told the keyword is retired and that `distance` replaces it. It MUST NOT load with the block silently pruned. The YANG validator cannot deliver this (A-5 broken), so the retired-keyword mechanism in `internal/component/config/retired.go` is what does |
| AC-4b | `rib { admin-distance { ebgp 20; } }` appears in a config | The same retired-keyword answer, naming `rib { distance { } }` |
| AC-5 | An eBGP route and a static route hold the same prefix, on default settings | The static route wins, because 10 is lower than 20, and this holds whether or not a `rib` block was written |
| AC-6 | A reload changes `distance { ebgp }` from 20 to 250 | Every stored route is recomputed and the selection changes for prefixes where it now matters |
| AC-7 | A reload with a distance change is rolled back | The previous map is restored and the selection returns to what it was |
| AC-8 | A protocol reports a route whose type names no declared distance | The route is not installed at a silent 0, and the condition is logged with the protocol name |
| AC-9 | The six numbers are read anywhere outside their declaration | They are derived from it, AT THE PRODUCER. `locrib.selectBest` ranks on the stamped value and `(*sysRIB).run` consumes one already-arbitrated best, so deriving them in sysrib alone leaves cross-protocol selection on the producer constants. The constants survive as a bootstrap value reachable only before the first configure |
| AC-10 | An operator sets `rib { distance { ebgp 250 } }` and an eBGP route meets an OSPF route for one prefix | PROVEN at both layers. `TestBgpStampsTheDeclaredDistanceNotItsOwn` shows the declaration reaches the stamp (RED at 0x14 where 0xfa was required), and `TestRaisedEbgpDistanceLetsOspfWin` (`internal/core/rib/locrib/distance_arbitration_test.go`) holds one prefix from two protocols and shows `selectBest` choosing OSPF at 110 over eBGP at 250, where it chooses eBGP at the default 20. Observed RED with distance ranking disabled: "best is 1, want the OSPF path at 110" |
| AC-11 | A route is stamped before sysrib's first configure has run | The producer's own bootstrap constant is used, never 0. Zero is the best possible distance and the value `connected` holds, so a zero stamped by accident beats every protocol |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs ze with a minimal config and expects the classical distances to apply | config to sysrib to `effectivePriority` to `locrib` selection to FIB | `TestDistanceMapIsPopulatedWithoutAConfigBlock` |
| 2 | Raises the eBGP distance so an OSPF route wins | `rib { distance { ebgp 250; } }` to the map to selection to the installed route | `test/plugin/distance-ebgp-override.ci` |
| 3 | Loads a config written against the old schema | the old spelling to the validator to a refusal naming it | `TestAdminDistanceIsRetiredNotSilent` |
| 4 | Changes a distance at runtime and reloads | reload to `reapplyAdminDistances` to republished best changes | `TestDistanceReloadRecomputesStoredRoutes` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDistanceMapIsPopulatedWithoutAConfigBlock` | `internal/component/sysrib/sysrib_test.go` | AC-1: the empty-map hole is closed | |
| `TestDistancePartialBlockKeepsTheOtherDefaults` | `internal/component/sysrib/sysrib_test.go` | AC-2 | |
| `TestEffectivePriorityResolvesEveryProtocolType` | `internal/component/sysrib/sysrib_test.go` | AC-1 and AC-8: every `ProtocolType.String()` value has a distance | |
| `TestUnknownProtocolTypeIsLoggedNotZeroed` | `internal/component/sysrib/sysrib_test.go` | AC-8 | |
| `TestAdminDistanceIsRetiredNotSilent` | `internal/component/config/yang/validator_test.go` | AC-4, and A-5 before the deletion | |
| `TestDistanceReloadRecomputesStoredRoutes` | `internal/component/sysrib/sysrib_test.go` | AC-6 | |
| `TestDistanceRollbackRestoresThePreviousMap` | `internal/component/sysrib/sysrib_test.go` | AC-7 | |
| `TestBgpStampsTheDeclaredDistanceNotItsOwn` | `internal/component/bgp/plugins/rib/rib_bestchange_test.go` | AC-3, AC-10. Observed RED under a bypassed seam: `0x14` where `0xfa` was required | |
| `TestBgpFallsBackToItsBootstrapBeforeConfigure` | `internal/component/bgp/plugins/rib/rib_bestchange_test.go` | AC-11 | |
| `TestUnsetSeamDoesNotAnswerZero` | `internal/core/rib/distance/distance_test.go` | AC-11 at the seam, and the one divergence from `igpcost` | |
| `TestDeclaredValueReachesTheProducer` | `internal/core/rib/distance/distance_test.go` | AC-10 at the seam, including a declared `connected 0` reading as an answer rather than an absence | |
| `TestSetReplacesRatherThanMerges` | `internal/core/rib/distance/distance_test.go` | AC-6: a leaf an operator removed reverts rather than lingering | |
| `TestAdminDistanceIsRetiredNotSilent` | `internal/component/config/retired_test.go` | AC-4, AC-4b. The YANG validator cannot deliver this, so the retired-keyword table is the only guard | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| a distance leaf other than `connected` | 1 to 255 | 255 | 0, refused by the restored YANG range | 256, refused by the YANG range |
| `connected` | 0 to 255 | 255 | N-A, 0 is its default and its correct value | 256, refused by `uint8` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `distance-config-surface` | `test/plugin/distance-config-surface.ci` | The new container validates, the block is optional, BOTH retired spellings are refused by name, and `ebgp 0` is refused | |
| NOT DONE, and homed in Work Not Done rather than left as a bare deferral: an end-to-end case installing a route | `test/plugin/` | An operator raises the eBGP distance above OSPF's and the OSPF route is the one INSTALLED. Two protocols must hold one prefix on a running daemon, which is a QEMU scenario rather than a `.ci` case. Each half is proven: `TestBgpStampsTheDeclaredDistanceNotItsOwn` at the stamp, `TestRaisedEbgpDistanceLetsOspfWin` at `locrib.selectBest`, `TestRunSysRIBPluginSeedsTheDeclarationAtStart` at the daemon's start | |

### Interop Tests (Scope: config)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | Administrative distance is local selection policy and reaches no wire, so no peer can observe it and no scenario can discriminate the change | |

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| One declaration of every protocol's distance | unit | `TestBgpStampsTheDeclaredDistanceNotItsOwn`, plus a grep recorded in the audit showing no second literal 20 or 200 in Go or YANG |
| The declaration is always populated AND reaches the producers | unit | Three tests, one per layer. `TestDeclaredDistancesApplyWithNoRibBlock`: the declaration resolves completely with no config. `TestPublishDistancesReachesTheSeam`: the resolved table reaches the seam a producer reads, RED under a broken publish. `TestRunSysRIBPluginSeedsTheDeclarationAtStart`: the daemon's own start path publishes it, RED with the seeding block removed ("the daemon start path published no declaration; every producer would keep its own constant"), which closes the call-site gap the previous closure rounds left open |
| Nothing is discarded in silence | unit | `TestUnknownProtocolTypeIsLoggedNotZeroed` |
| The rename does not cross into the RIB concept | audit | The recorded count on each side of the A-3 boundary, and a diff touching no file under `internal/core/rib/locrib/` |

## Files to Modify
- `internal/component/sysrib/yang/ze-rib-conf.yang` - the container is renamed
  to `distance`, and its description distinguishes administrative distance from
  the IGP distance OSPF's help text uses the word for
- `internal/core/rib/distance/` - NEW leaf package, the seam carrying the
  declaration to the producers. `locrib.selectBest` ranks on the value the
  producer stamped and `(*sysRIB).run` consumes one already-arbitrated best per
  prefix, so a distance resolved in sysrib alone changes no cross-protocol
  selection. Mirrors `internal/core/rib/igpcost`, except that an unset seam
  reports that it did not answer instead of returning 0, because 0 is the BEST
  distance here rather than a no-op
- `internal/component/sysrib/register.go` - `publishDistances` installs the
  resolved table on the seam at every site that assigns `s.adminDist`, the
  rollback included
- `internal/component/bgp/plugins/rib/rib_bestchange.go` - the stamp reads the
  seam, with the plugin's constants demoted to a bootstrap value
- `internal/plugins/isis/spf/install.go`, `internal/plugins/ospf/spf/install.go` -
  the same, read AT the stamp rather than at construction so a reload takes effect
- `internal/component/sysrib/sysrib.go` - `parseAdminDistanceConfig` returns a
  complete map, and `effectivePriority` stops treating an empty map as a reason
  to trust an incoming value
- `internal/component/sysrib/register.go` - the three load sites
- `internal/component/bgp/yang/ze-bgp-conf.yang` - the `admin-distance`
  container is DELETED
- `internal/component/bgp/plugins/rib/rib_admin_distance_config.go` - deleted
- `internal/component/config/retired.go` - `admin-distance` is registered as a
  RETIRED keyword naming `distance` as its replacement, in both containers.
  A-5 is broken, so without this an operator's existing config loses its
  distances to a pruned warning rather than an error (`loader.go:275`)
- `internal/component/bgp/plugins/rib/rib.go` - the two atomics, their
  configure-stage population, and the third copy of 20 and 200
- `internal/component/bgp/plugins/rib/rib_bestchange.go` - the stamp site and
  the comment above it
- `internal/plugins/isis/spf/install.go` - the two comments naming
  `rib.admin-distance.isis`
- `pkg/plugin/rpc/types.go` - the `json:"admin-distance"` tag and the
  `OperationSetAdminDistance` operation string. This is the plugin SDK's wire
  contract, so the change is BREAKING for an out-of-tree plugin
- `pkg/plugin/sdk/sdk_types.go` - the re-export of that operation constant
- `internal/component/bgp/plugins/rib/rib_commands.go` - the `show` JSON key
- `internal/component/bgp/plugins/cmd/rib/rib.go` - the CLI column header
- `internal/component/config/transaction/operation.go` - whichever side of the
  operation string it holds
- `docs/architecture/config/syntax.md` - the rule about which sections receive
  YANG defaults
- `docs/architecture/core-design.md` - the sentence naming the single
  declaration of a protocol's distance
- `docs/architecture/isis/isis-9-spf-rib.md` - the distance references, which
  move with the rename
- `docs/architecture/plugin/rib-storage-design.md` - the BGP-side extraction
  paragraph
- `docs/config-reference.md`, `docs/guide/configuration.md` - the container name
  and its leaves
- every `.conf` and `.ci` under `test/`, `demos/` and `contrib/` naming
  `admin-distance`, listed by the audit rather than guessed

## Files to Create
- `test/plugin/distance-ebgp-override.ci`
- `test/plugin/distance-default-without-block.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-rib-conf.yang` renames a container; `ze-bgp-conf.yang` deletes one |
| YANG validation constraints | Yes | `static`, `ebgp`, `ospf`, `isis` and `ibgp` carry `range "1..255"`; `connected` stays a bare `uint8` because 0 is its declared default and its legitimate value. The rib leaves were bare `uint8` BEFORE this spec, and deleting `bgp { admin-distance }` (whose leaves did carry the range) removed the only guard, so `ebgp 0` was briefly accepted and would beat a connected route |
| YANG custom validators | No | a range is sufficient; no cross-leaf rule exists |
| CLI commands/flags | No | no command is added |
| CLI grammar (keyword before value) | N-A | no command added |
| Editor autocomplete | Yes | DERIVED from the schema; confirm the renamed container completes and the deleted one does not |
| Functional test for new RPC/API | Yes | the two `.ci` tests above |
| Pipe completeness | N-A | no command output changes |
| Env var registration | No | none added |
| Doctor check for runtime dependencies | N-A | no file, socket, port, module or binary is added |
| Prometheus counters/metrics | No | a distance is config, not a rate |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | A knob moves and two duplicates are removed |
| 2 | Config syntax changed? | Yes | `docs/config-reference.md`, `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | Yes | `docs/architecture/plugin/rib-storage-design.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` |
| 7 | Wire format changed? | No | distance reaches no wire |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | no RFC names this quantity |
| 10 | Test infrastructure changed? | No | none |
| 11 | Affects daemon comparison? | No | no feature gained or lost |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/syntax.md` for the YANG-default rule, `docs/architecture/core-design.md` for the single declaration, `docs/architecture/isis/isis-9-spf-rib.md` for the second consumer |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/immediate/spec-fixit-bgp-distance-declaration.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | every page showing an `admin-distance` block |

## Implementation Steps

1. **Phase: the boundary** - draw and record the rename line before editing
   - Tests: none. This phase produces the audit record A-3 needs
   - Files: none edited
   - Verify: every file naming `admin-distance` is classified as config-leaf or
     RIB concept, with the count on each side written into the spec
2. **Phase: Wiring (MANDATORY FIRST)** - the two `.ci` tests, the empty-map unit
   test, and the validator test A-5 needs
   - Tests: `TestDistanceMapIsPopulatedWithoutAConfigBlock`,
     `TestAdminDistanceIsRetiredNotSilent`,
     `test/plugin/distance-default-without-block.ci`
   - Files: `internal/component/sysrib/sysrib_test.go`,
     `internal/component/config/yang/validator_test.go`, the two new `.ci` files
   - Verify: red, and the red names the empty map rather than a missing fixture
3. **Phase: populate the declaration** - the map carries all six protocols
   whether or not the block was written
   - Tests: phase 2's, plus `TestDistancePartialBlockKeepsTheOtherDefaults` and
     `TestEffectivePriorityResolvesEveryProtocolType`
   - Files: `internal/component/sysrib/sysrib.go`, `register.go`, and whichever
     surface holds the six numbers once
   - Verify: green, and A-2's grep is answered before this phase closes
4. **Phase: speak** - an unresolvable protocol is logged rather than zeroed
   - Tests: `TestUnknownProtocolTypeIsLoggedNotZeroed`
   - Files: `internal/component/sysrib/sysrib.go`
   - Verify: the log line names the protocol
5. **Phase: delete the duplicates** - the BGP container, its reader, and the Go
   copy of 20 and 200
   - Tests: `TestBgpStampsTheDeclaredDistanceNotItsOwn`
   - Files: `ze-bgp-conf.yang`, `rib_admin_distance_config.go`, `rib.go`,
     `rib_bestchange.go`
   - Verify: no second literal 20 or 200 remains, and the old spelling is refused
6. **Phase: rename** - `admin-distance` becomes `distance` on the config-leaf
   side of the phase 1 boundary and nowhere else
   - Tests: the suites above, plus every `.ci` naming the old spelling
   - Files: `ze-rib-conf.yang` and the config files the audit listed
   - Verify: the diff touches no file under `internal/core/rib/locrib/`
7. **Phase: the pages** - repair every page the change made wrong
   - Tests: none. `./le spec citation anchors` names the pages
   - Files: the Documentation Update Checklist's rows
   - Verify: no page still shows an `admin-distance` block

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Correctness | The six numbers exist in exactly one place, checked by reading it rather than by grep alone |
| Correctness | `effectivePriority` has no path returning an incoming value for a protocol the declaration names |
| Naming | The rename touched only the config-leaf side of the A-3 boundary |
| Data flow | A BGP route's distance enters selection from one source, traced from the config file to the installed route |
| Rule: `ai/rules/principles.md` | One fact declared once, and no zero reachable as a valid-looking answer |
| Rule: `ai/rules/no-layering.md` | The BGP container is deleted, not deprecated, and no compatibility path reads the old spelling |
| Rule: `ai/rules/config.md` | The renamed container keeps its range constraints, and its help text distinguishes it from IGP distance |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| One declaration of the six numbers | Read the declaring surface, then grep for a second literal |
| `bgp { admin-distance }` is gone from the schema | `grep admin-distance internal/component/bgp/yang/` returns nothing |
| The map is populated with no config block | `TestDistanceMapIsPopulatedWithoutAConfigBlock` |
| The rename boundary held | The diff's file list against the phase 1 record |
| No page shows the old spelling | Grep `docs/`, `test/`, `demos/`, `contrib/` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Guard that could fail open | Route selection decides what the kernel installs. A distance resolving to 0 makes a route beat every other protocol, which is what AC-8 exists to prevent |
| Input validation | Every leaf but `connected` carries `range "1..255"`, so 0 cannot be configured for a protocol that must not beat a directly connected route. This was RESTORED rather than kept: the rib leaves never had it, and the deleted BGP container was where it lived |
| Guard that could fail open | The validator must refuse the deleted container rather than ignoring it, or an operator's intended distance silently does not apply |
| Resource exhaustion | N-A. The map holds one entry per protocol |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood, back to RESEARCH |
| Lint failure | Fix inline. If architectural, back to DESIGN |
| Functional test fails | Check the AC: wrong AC to DESIGN, correct AC to IMPLEMENT |
| An existing test goes red and is right about the product | It is a defect found. Root cause it under `ai/rules/completion.md` |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The duplication was not the defect. The defect was that the declarations met
  in a function whose empty-map branch silently changed which one decided, and
  no log line marked the switch.
- A YANG `default` reads like a value and is metadata. Every consumer parsing a
  config section directly, rather than through the peer path that calls
  `ApplyDefaults`, sees a leaf that is simply absent. That is a general trap and
  the reason the `syntax.md` edit is in scope.
- The rename is safe only because no RFC names this quantity, which the BGP help
  text already says.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Keep the RIB declaration and delete the BGP one | Keep BGP's and have sysrib defer to the stamped value | The RIB table already names all six protocols and is where cross-protocol arbitration belongs. BGP's names two and describes the same quantity |
| Rename to `distance` | Keep `admin-distance` | Owner decision, 2026-09-04. No `distance` leaf or container exists in the schema, so there is no identifier collision; the word appears in OSPF help prose for the IGP quantity, which the new container's help text distinguishes |
| Rename the operator-facing and plugin-facing names too, not the config leaf alone | Rename the config container only, leaving `show` output and the plugin RPC on `admin-distance` | Owner decision, 2026-09-04, taken with the SDK break on the table. Renaming the container alone relocates the synonym rather than removing it: an operator would configure `distance` and read `admin-distance` back from `show`. One quantity, one spelling, everywhere a person or a plugin author looks |
| Populate the map rather than keep the empty-map fallback | Leave the fallback and let BGP keep a constant | The fallback is what makes the answer depend on whether an unrelated block was written. Removing the duplicate without removing the fallback leaves BGP with no configurable distance at all |
| Log an unresolvable protocol rather than defaulting it | Return a safe high distance silently | A silent default is the failure class this spec exists to remove |

## Known Limitations

- The mechanism for populating the six numbers is chosen in phase 3 and is not
  fixed here. Applying YANG defaults to the `rib` section, and declaring them in
  Go with the schema deriving from them, are both single-declaration answers;
  the phase records which it took and why.
- Whether a distance change republishes to the FIB promptly under load is
  unchanged by this spec and unmeasured by it.

## RFC Documentation (Scope: config)

N-A. Administrative distance is a vendor convention with no RFC behind it, which
`ze-bgp-conf.yang` states today: "The defaults follow the Cisco and Juniper
convention. RFC 4271 mandates no values." No `// RFC NNNN Section X.Y` comment is
owed by this work, and none is removed by it.

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
- [ ] AC-1..AC-9 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- `rib { distance { } }` (`internal/component/sysrib/yang/ze-rib-conf.yang`) is the
  ONE declaration of every protocol's administrative distance. The container is
  renamed from `admin-distance`, `bgp { admin-distance { } }` is deleted, and
  `rib_admin_distance_config.go` with it.
- `parseAdminDistanceConfig` (`internal/component/sysrib/sysrib.go`) returns every
  protocol the schema declares, defaults included, and returns an ERROR rather
  than an empty map. `effectivePriority` lost its empty-map branch; the two states
  that still reach the fallback log once per protocol through `distanceSpoken`.
- `internal/core/rib/distance` is a new leaf package, the seam that carries the
  resolved table to the PRODUCERS. `Of` reports whether the declaration answered;
  it never answers 0, because 0 is the best distance and the one `connected`
  holds. `publishDistances` (`internal/component/sysrib/register.go`) installs the
  table at every site that assigns `s.adminDist`, the rollback included.
- The three producers that stamp `locrib.Path.AdminDistance` read the seam at the
  stamp rather than at construction, so a reload takes effect:
  `rib_bestchange.go:962`, `internal/plugins/isis/spf/install.go:264`,
  `internal/plugins/ospf/spf/install.go:232`.
- `runSysRIBPlugin` seeds map and seam from `parseAdminDistanceConfig("{}")` at
  plugin start and refuses to start when the schema will not resolve, because a
  config with no `rib` block delivers no section at all.
- `admin-distance` is a retired keyword (`internal/component/config/retired.go`)
  naming `distance` as its replacement, in both containers. The YANG validator
  cannot deliver that message: `walkTree` iterates the schema's children, never
  the data.
- `range "1..255"` restored on the five non-connected leaves. Deleting the BGP
  container removed the only leaves that carried it, so `ebgp 0` was briefly
  accepted and would have beaten a directly connected route.
- BREAKING for an out-of-tree plugin: `json:"admin-distance"` became
  `json:"distance"` (`pkg/plugin/rpc/types.go`) and `OperationSetAdminDistance`
  became `OperationSetDistance`.

### Bugs Found/Fixed
- The feature shipped INERT in `de739c8b2`. `ExtractConfigSubtree` returns nil for
  an absent path and `BuildPluginConfigSections`
  (`internal/component/config/plugin_verify.go`) skips that root, so `OnConfigure`
  never ran for a config with no `rib` block. One config in this repository
  carries one. Fixed in `e0879e4f6`; covered by
  `TestRunSysRIBPluginSeedsTheDeclarationAtStart`.
- The rollback re-opened the empty map through a second door: `previousDist` was
  nil until `OnConfigure` fired, so an operator ADDING a block whose transaction
  rolled back got an empty map into both `s.adminDist` and `publishDistances`.
  Fixed in `95f07a8f7` by seeding `previousDist` with the declaration.
- `ebgp 0` was accepted between the container delete and the range restore, and
  would have beaten `connected`. Pinned by `test/plugin/distance-config-surface.ci`.
- Three web goldens went red on the deleted BGP container and were regenerated
  under the canonical tag set in `95f07a8f7`; thirteen unrelated goldens that
  `-update-golden` rewrote were restored, twice.
- Closure round 1 (this phase): `internal/component/iface/yang/ze-iface-conf.yang`
  told an operator that `rib admin-distance` ranks connected 0, static 10, ebgp
  20. The leaf is retired. The hit hid behind a line break inside the word.
- Closure round 1: `RIBManager.adminDistanceEBGP` and `adminDistanceIBGP` became
  write-once when the configure block was deleted, so two `atomic.Uint32` fields,
  two `//nolint:gosec` conversions and two log fields carried a constant. The
  stamp reads `DefaultAdminDistanceEBGP` and `DefaultAdminDistanceIBGP` directly.

### Documentation Updates
- `docs/config-reference.md`, `docs/guide/configuration.md` -- the container name,
  its leaves, and the retirement of both old spellings (`de739c8b2`).
- `docs/architecture/core-design.md` -- the single declaration, the seam, and why
  the value has to reach the producer.
- `docs/architecture/config/syntax.md` -- "YANG Defaults Do Not Reach a Config On
  Their Own", and NEW in this closure, "An Absent Config Root Delivers No Section
  At All", with source anchors on `BuildPluginConfigSections` and
  `runSysRIBPlugin`. That is the lesson routed to the surface that governs it
  (`ai/rules/planning.md`).
- `docs/architecture/isis/isis-9-spf-rib.md` -- `rib.distance.isis` (`57ecac2d4`).
- `docs/architecture/config/transaction-protocol.md` -- `set-distance`.
- `docs/architecture/rib/unified-locrib.md`, `docs/architecture/ospf/ospf-ext-6-ti-lfa.md`,
  `ai/digests/fib-programming.md` -- the renamed leaf.
- `docs/architecture/plugin/rib-storage-design.md`: NOT updated, and none was
  owed. `git show de739c8b2~1:docs/architecture/plugin/rib-storage-design.md |
  grep -i admin.distance` returns nothing, so the page never documented the
  BGP-side extraction this spec deleted. The checklist row predicted wrongly.
- `./le doc check verify` result: recorded in Pre-Commit Verification below.

### Deviations from Plan
- The six numbers stay declared in YANG alone, and Go derives them. The Known
  Limitations left the mechanism open for phase 3 to choose; it chose the schema,
  because YANG carries the numbers anyway for validation, completion and the
  generated config reference.
- `internal/core/rib/distance` was NOT in the original Files to Modify. It was
  added when the audit showed `locrib.selectBest` ranks on the value the PRODUCER
  stamped, so a distance resolved in sysrib alone changes no selection at all.
- `internal/core/rib/locrib/distance_arbitration_test.go` was added, which the
  A-3 boundary said the diff would not touch. The boundary was about the RENAME,
  which did not cross; a test that inserts two protocols into one `locrib.RIB` is
  the only place the winner can flip, so AC-10 could not be proven anywhere else.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2 assumed no in-tree config depends on the empty-map path | 131 configs write a `rib {` block and 2 name a distance, so 129 took the empty-map branch | the grep A-2 required, run before phase 3 | Recorded broken; the behavior change is stated in `de739c8b2` |
| assumption | A-5 assumed the config validator refuses an unknown container | `walkTree` (`internal/component/config/yang/validator.go`) iterates the SCHEMA's children and never the data, so it emits nothing for a key it does not know | `TestAdminDistanceIsRetiredNotSilent`, written before the deletion | The retired-keyword table carries the message instead |
| approach | The declaration was resolved in `OnConfigure` alone | A config with no `rib` block delivers no section, so the callback never fires and the feature is inert on all but one config in the tree | an independent closure gate, after the first commit | Seeded at plugin start; the rule is now in `docs/architecture/config/syntax.md` |
| escalation | Three closure rounds in a row faulted the same shape: the proof stopping one layer above the behavior. Every test called `parseAdminDistanceConfig` or assigned `s.adminDist` by hand | Deleting `publishDistances`, and later the seeding block, left the whole suite green | this closure asked what a deletion would leave green | `TestPublishDistancesReachesTheSeam` and `TestRunSysRIBPluginSeedsTheDeclarationAtStart`, both observed RED |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| One declaration of every protocol's distance | Done | `internal/component/sysrib/yang/ze-rib-conf.yang`, container `distance` | `grep admin-distance internal/component/bgp/yang/` returns nothing |
| Always populated | Done | `parseAdminDistanceConfig`, `runSysRIBPlugin` seeding block | `TestRunSysRIBPluginSeedsTheDeclarationAtStart` |
| Nothing discarded in silence | Done | `(*sysRIB).effectivePriority`, `distanceSpoken` | `TestUnknownProtocolTypeIsLoggedNotZeroed` |
| One spelling, `distance`, on every operator and plugin surface | Done | `pkg/plugin/rpc/types.go`, `rib_commands.go`, `cmd/rib/rib.go`, `transaction/operation.go` | `git grep set-admin-distance` returns only spec prose |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRunSysRIBPluginSeedsTheDeclarationAtStart`, `TestDeclaredDistancesApplyWithNoRibBlock` | RED observed with the seeding block removed |
| AC-2 | Done | `TestDistancePartialBlockKeepsTheOtherDefaults` | |
| AC-3 | Done | `TestBgpStampsTheDeclaredDistanceNotItsOwn` | RED at 0x14 where 0xfa was required |
| AC-4 | Done | `TestAdminDistanceIsRetiredNotSilent`, `test/plugin/distance-config-surface.ci` seq 3 | |
| AC-4b | Done | `test/plugin/distance-config-surface.ci` seq 4 | |
| AC-5 | Done | `TestSysRIBStaticWinsOverBGP` with the seeded map | |
| AC-6 | Done | `TestDistanceReloadRecomputesStoredRoutes` | |
| AC-7 | Done | `TestDistanceRollbackRestoresThePreviousMap` | `previousDist` seeded with the declaration |
| AC-8 | Done | `TestUnknownProtocolTypeIsLoggedNotZeroed` | |
| AC-9 | Done | `git grep -n "AdminDistance:" --include=*.go internal/` names three stamps, all reading the seam | The constants survive as the documented bootstrap |
| AC-10 | Done | `TestRaisedEbgpDistanceLetsOspfWin` at `selectBest`, `TestBgpStampsTheDeclaredDistanceNotItsOwn` at the stamp | RED: "best is 1, want the OSPF path at 110" |
| AC-11 | Done | `TestBgpFallsBackToItsBootstrapBeforeConfigure`, `TestUnsetSeamDoesNotAnswerZero` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestDistanceMapIsPopulatedWithoutAConfigBlock` | Done | `internal/component/sysrib/sysrib_test.go:498` | |
| `TestDistancePartialBlockKeepsTheOtherDefaults` | Done | `sysrib_test.go:518` | |
| `TestEffectivePriorityResolvesEveryProtocolType` | Done | `sysrib_test.go:1324` | |
| `TestUnknownProtocolTypeIsLoggedNotZeroed` | Done | `sysrib_test.go:467` | |
| `TestAdminDistanceIsRetiredNotSilent` | Changed | `internal/component/config/retired_test.go:124` | Landed in `retired_test.go`, not `yang/validator_test.go`: A-5 broke, and the validator is not what refuses |
| `TestDistanceReloadRecomputesStoredRoutes` | Done | `sysrib_test.go:1351` | |
| `TestDistanceRollbackRestoresThePreviousMap` | Done | `sysrib_test.go:1374` | |
| `TestBgpStampsTheDeclaredDistanceNotItsOwn` | Done | `rib_bestchange_test.go:1828` | |
| `TestBgpFallsBackToItsBootstrapBeforeConfigure` | Done | `rib_bestchange_test.go:1870` | |
| `TestUnsetSeamDoesNotAnswerZero` | Done | `internal/core/rib/distance/distance_test.go:11` | |
| `TestDeclaredValueReachesTheProducer` | Done | `distance_test.go:25` | |
| `TestSetReplacesRatherThanMerges` | Done | `distance_test.go:54` | |
| `TestDeclaredDistancesApplyWithNoRibBlock` | Changed | `sysrib_test.go:1412` | Added after the plan, by the gate that found the inert feature |
| `TestPublishDistancesReachesTheSeam` | Changed | `sysrib_test.go:1447` | Added after the plan; proves the table reaches the seam |
| `TestRaisedEbgpDistanceLetsOspfWin` | Changed | `internal/core/rib/locrib/distance_arbitration_test.go:26` | Added after the plan; the only layer where the winner can flip |
| `TestRunSysRIBPluginSeedsTheDeclarationAtStart` | Changed | `sysrib_test.go:1494` | Added in this closure; the call site nothing asserted |
| `test/plugin/distance-config-surface.ci` | Done | `test/plugin/` | |
| `test/plugin/distance-ebgp-override.ci` | Done | `test/plugin/` | Config surface only; its header says so |
| `test/plugin/distance-default-without-block.ci` | Done | `test/plugin/` | Discriminates nothing on its own; its header says so |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/sysrib/yang/ze-rib-conf.yang` | Done | Renamed, ranges restored |
| `internal/core/rib/distance/` | Done | New leaf package |
| `internal/component/sysrib/register.go` | Done | `publishDistances`, the seeding block, the rollback seed |
| `internal/component/sysrib/sysrib.go` | Done | Complete map, no empty-map branch |
| `internal/component/bgp/plugins/rib/rib_bestchange.go` | Done | Stamp reads the seam |
| `internal/plugins/isis/spf/install.go`, `internal/plugins/ospf/spf/install.go` | Done | Same, read at the stamp |
| `internal/component/bgp/yang/ze-bgp-conf.yang` | Done | Container deleted |
| `internal/component/bgp/plugins/rib/rib_admin_distance_config.go` | Done | Deleted, with its test file |
| `internal/component/config/retired.go` | Done | `admin-distance` registered |
| `internal/component/bgp/plugins/rib/rib.go` | Done | Third copy gone; the write-once atomics removed in this closure |
| `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk_types.go`, `internal/component/config/transaction/operation.go` | Done | `distance`, `set-distance` |
| `internal/component/bgp/plugins/rib/rib_commands.go`, `internal/component/bgp/plugins/cmd/rib/rib.go` | Done | JSON key and column header |
| `docs/architecture/plugin/rib-storage-design.md` | Changed | No edit owed: the page never mentioned admin distance |
| every `.conf` and `.ci` naming `admin-distance` | Done | `git grep admin-distance` outside `plan/`, `rfc/` and the frozen `internal/le/site/testdata/` published-site snapshots returns only the retirement text and its tests |

### Audit Summary
- **Total items:** 47 (4 requirements, 12 AC, 19 tests, 14 files, minus overlap)
- **Done:** 41
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 6 (recorded in Deviations and in the tables above)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| One declaration of every protocol's distance | functional + audit | `test/plugin/distance-config-surface.ci` drives the real binary: the new container validates, both retired spellings are refused by name, `ebgp 0` is refused. `git grep -n "AdminDistance:" --include=*.go internal/` names three stamp sites and all three read the seam |
| The declaration is always populated AND reaches the producers | unit, RED observed at each layer | `TestDeclaredDistancesApplyWithNoRibBlock` (resolution), `TestPublishDistancesReachesTheSeam` (RED: "publishDistances did not reach the seam"), `TestRunSysRIBPluginSeedsTheDeclarationAtStart` (RED: "the daemon start path published no declaration") |
| Nothing is discarded in silence | unit | `TestUnknownProtocolTypeIsLoggedNotZeroed`; `effectivePriority` has no branch that returns an incoming value for a protocol the declaration names |
| The rename does not cross into the RIB concept | audit | No file under `internal/core/rib/locrib/` was RENAMED; the one file added there is a test that inserts two protocols into one RIB, which is the only place AC-10 can be observed |
| An operator raising eBGP above OSPF gets the OSPF route | unit at two layers | `TestBgpStampsTheDeclaredDistanceNotItsOwn` (RED at 0x14 where 0xfa was required) and `TestRaisedEbgpDistanceLetsOspfWin` (RED: "best is 1, want the OSPF path at 110"). NOT proven on a running daemon: see Work Not Done |
| Interop | N-A | Administrative distance is local selection policy and reaches no wire, so no peer can observe it and no scenario can discriminate the change |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| An end-to-end case on a running daemon where an operator raises the eBGP distance above OSPF's and the OSPF route is the one INSTALLED | Two protocols must hold one prefix on a live daemon, which is a QEMU scenario rather than a `.ci` config case. Every layer is proven separately: the daemon seeds, the declaration reaches the stamp, `selectBest` flips the winner | `plan/immediate/spec-connected-static-reach-the-locrib.md`, which brings the second protocol into the same Loc-RIB and is the first spec able to hold one prefix from two producers |
| `rib { distance { connected } }` and `{ static }` are settable and inert | Neither the static plugin nor the interface layer inserts into the Loc-RIB, so neither reads the seam. Pre-existing, not introduced here, and recorded in `plan/journal/guard-added-to-one-half-of-a-pair.md` | `plan/immediate/spec-connected-static-reach-the-locrib.md` |
| A distance declared in one process does not reach a producer in another | The seam is process-global and sysrib is its only publisher, so `plugin { external rib }` or any forked producer stamps its own bootstrap. Stated at the seam (`internal/core/rib/distance/distance.go`, `Set`) rather than left for the next reader | Unowned. No spec claims it yet; the limitation is documented where somebody would go to change it |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | ``tmp/review/fixit-bgp-distance-declaration-zeclose-distance.md` (24 files, verdict=clean)` |
| `./le spec session review check` | `OK (clean, hashes match). NOTE: recorded on an unknown model, so the review-model boundary is UNCHECKED by the tool; the phase boundary holds, since this context did not write the implementation` |
| Rounds | 2. Round 1 found the seven rows below. Round 2 re-read every fix, re-ran `go vet` and `gofmt` over the changed Go, and ran `go test -race -count=2 ./internal/component/sysrib/` (ok, 2.168s) because the new test starts a plugin goroutine and writes two process-global seams; it found 0 BLOCKER and 0 ISSUE |
| Reviewer lenses used | deliverables against source; security (route selection as a guard that must not fail open); documentation against the producing function; Go style (`docs/contributing/ze-go-style.md`); dead machinery and stale comments; test discrimination (what would a deletion leave green) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The seeding CALL SITE was unasserted. Deleting `s.adminDist = declared; publishDistances(declared)` left the whole suite green, so the one path an ordinary operator takes had no test. `ai/rules/principles.md`: the feature is the path from what the user types to what the product answers | `runSysRIBPlugin` (`internal/component/sysrib/register.go:121-131`) | `TestRunSysRIBPluginSeedsTheDeclarationAtStart` (`internal/component/sysrib/sysrib_test.go`), RED observed with the publish removed |
| 2 | ISSUE | An operator reading `interface { ... unit { route-priority } }` help was told the ranking follows `rib admin-distance`, a leaf this spec retired. The grep for the old spelling missed it because a line break splits the word | `internal/component/iface/yang/ze-iface-conf.yang:251` | The help text names `rib distance` |
| 3 | ISSUE | `RIBManager.adminDistanceEBGP` and `adminDistanceIBGP` became write-once when the configure block was deleted: two `atomic.Uint32` fields, two `//nolint:gosec` conversions and two log fields all carrying a constant, under a comment whose stated reason ("the stamp site reads them on the forwarding path") is not a reason to hold a constant in an atomic | `internal/component/bgp/plugins/rib/rib.go`, `rib_bestchange.go:958` | Fields, stores, `nolint`s and log fields removed; the stamp reads `DefaultAdminDistanceEBGP` / `DefaultAdminDistanceIBGP` |
| 4 | ISSUE | The lesson of the whole spec governed no surface: no page said an absent config root delivers no section, so the next component to initialize state in `OnConfigure` alone repeats it | `docs/architecture/config/syntax.md` | New section "An Absent Config Root Delivers No Section At All", anchored on `BuildPluginConfigSections` and `runSysRIBPlugin` |
| 5 | NOTE | The Wiring Test and Functional Tests tables named `.ci` files as proof of behavior those files state in their own headers they do not prove | this spec | Both tables now name the test that proves each row |
| 6 | NOTE | `publishDistances` refuses a value outside 0..255 by answering `(0, false)` while `effectivePriority` would return that same value from its map, so the comment "the seam and the map cannot disagree" has one branch where they can. Unreachable today: YANG declares `uint8` with `range "1..255"` | `internal/component/sysrib/register.go:75` | Left; recorded here |
| 7 | NOTE | `distanceSpoken` grows one entry per distinct protocol name seen and declares no bound. Not peer-reachable: the name comes from an in-tree producer's `BestChangeBatch`, never from the wire | `internal/component/sysrib/sysrib.go:209` | Left; recorded here |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/rib/distance/distance.go` | Yes | `ls -l` 4117 bytes, 2026-09-05 |
| `internal/core/rib/distance/distance_test.go` | Yes | `ls -l` 2470 bytes |
| `internal/component/bgp/plugins/rib/rib_distance.go` | Yes | `ls -l` 1471 bytes |
| `internal/core/rib/locrib/distance_arbitration_test.go` | Yes | `ls -l` 2842 bytes |
| `test/plugin/distance-config-surface.ci` | Yes | `ls -l` 2523 bytes |
| `test/plugin/distance-default-without-block.ci` | Yes | `ls -l` 1641 bytes |
| `test/plugin/distance-ebgp-override.ci` | Yes | `ls -l` 1575 bytes |
| `internal/component/bgp/plugins/rib/rib_admin_distance_config.go` | No, deleted | `ls` returns "No such file or directory" |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | The map is complete with no `rib` section, and the daemon installs it | `go test ./internal/component/sysrib/ -run TestRunSysRIBPluginSeedsTheDeclarationAtStart`: FAIL with the publish removed ("the daemon start path published no declaration"), ok 0.199s with it restored |
| AC-3, AC-10, AC-11 | The declaration reaches the stamp; an unset seam gives 20, never 0 | `go test ./internal/component/bgp/plugins/rib/` ok 3.356s |
| AC-4, AC-4b | Both retired spellings are refused by name | `go test ./internal/component/config/` ok 21.692s, carrying `TestAdminDistanceIsRetiredNotSilent` |
| AC-10 | `selectBest` flips the winner to OSPF at a configured eBGP 250 | `go test ./internal/core/rib/locrib/` ok 0.023s |
| AC-11 | The seam does not answer zero | `go test ./internal/core/rib/distance/` ok 0.005s |
| AC-9 | The six numbers are declared once | `grep admin-distance internal/component/bgp/yang/` returns nothing; `git grep -n "AdminDistance:" --include=*.go internal/` names three stamps, each `ribdistance.OrDefault(...)` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| An operator starts ze with no `rib` block | `test/plugin/distance-default-without-block.ci` | Read in full. It runs `ze config validate` only and its header says it discriminates nothing. The daemon start path is verified by `TestRunSysRIBPluginSeedsTheDeclarationAtStart` instead, with its RED recorded |
| An operator raises the eBGP distance | `test/plugin/distance-ebgp-override.ci` | Read in full. Config surface only, and its header says so. The behavior is verified by `TestRaisedEbgpDistanceLetsOspfWin` |
| An operator writes either retired spelling | `test/plugin/distance-config-surface.ci` | Read in full. seq 3 and seq 4 each run the real binary and require exit 1 with `admin-distance` on stdout; seq 5 refuses `ebgp 0` |
| A reload changes a distance | none | `TestDistanceReloadRecomputesStoredRoutes` and `TestDistanceRollbackRestoresThePreviousMap` drive `reapplyAdminDistances` and the journal rollback |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `git grep -n effectivePriority internal/component/sysrib/*.go` outside tests: two call sites, `sysrib.go:411` and `:492`, and both hold `s.mu` for writing |
| A-2 | broken | 131 configs write a `rib {` block, 2 name a distance; the behavior change is stated in `de739c8b2` |
| A-3 | confirmed | The rename touched no identifier under `internal/core/rib/locrib/`; the only file added there is a test |
| A-4 | confirmed | Pre-release: no release, no tag, no user of `main` |
| A-5 | broken | `walkTree` (`internal/component/config/yang/validator.go`) iterates `entry.Dir`, the schema's children, never the data. `internal/component/config/retired.go` carries the message instead |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Config syntax (row 2) | `docs/config-reference.md:78`, `docs/guide/configuration.md:2060` name `distance` and the retirement of both old spellings; checked against `ze-rib-conf.yang` container `distance` and `retiredKeywords` | Yes |
| Plugin design page (row 5) | `git show de739c8b2~1:docs/architecture/plugin/rib-storage-design.md \| grep -i admin.distance` returns nothing: the page never documented the deleted extraction | No edit owed |
| Internal architecture (row 12) | `docs/architecture/core-design.md:1329-1348` states the single declaration and the seam, checked against `parseAdminDistanceConfig` and `distance.Of`; `docs/architecture/config/syntax.md` carries both the YANG-default rule and the new absent-section rule; `docs/architecture/isis/isis-9-spf-rib.md` names `rib.distance.isis` | Yes |
| Existing examples (row 17) | `git grep admin-distance` outside `plan/` and `rfc/`: the only hits are the retirement text, its tests, the YANG revision log, and the frozen published-site snapshots under `internal/le/site/testdata/` which are inputs to `TestTheConfigurationMirrorReadsAsThePublishedMirror`, not renderings of the current schema | Yes |
| Operator help text | `internal/component/iface/yang/ze-iface-conf.yang:251` said `rib admin-distance`; repaired in this closure to `rib distance` | Yes, fixed |
| Doctor checks | No runtime dependency is added: no file, socket, port, module or binary | N-A |
| RFC status | No RFC names administrative distance. `ze-rib-conf.yang` states it: "RFC 4271 mandates no values" | N-A |
| `./le doc check verify` | exit 1 with 3482 issues, none of them this spec's. Every issue names `../gh-pages/` or `../wiki/`, the published-site checkouts left behind by another session's thirteenth CLI verb (`send`). `grep -iE 'iface-conf\|ze-rib-conf\|config/syntax\|sysrib\|rib/distance'` over the full log returns nothing. Digest anchors: 3026 checked across 23 digests, all resolve | |

## Core Insight

A knob is not configurable because a config leaf exists and a resolver reads it.
It is configurable only where the value ARRIVES before the decision is taken.

`locrib.selectBest` ranks paths on the distance the PRODUCER stamped, and
`(*sysRIB).run` consumes one already-arbitrated best per prefix. So sysrib, which
owned the config and the resolver, sat downstream of the only place the number
could change an outcome. A perfectly resolved table there changed nothing, and
every test that stopped at the resolver passed.

Three closure rounds each found the same shape one layer lower: the resolver, the
publish, then the call site. The question that finds it is not "does the test
pass" but "what would this test do if I deleted the code it names".


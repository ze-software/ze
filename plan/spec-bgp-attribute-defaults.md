# Spec: bgp attribute defaults

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | spec-fixit-filter-subject-drops-five-attributes, spec-fixit-bgp-distance-declaration (ordering only) |
| Phase | CLOSURE, 2026-09-04. Implementation complete and the closure sections are appended |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-09-04 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The value an `increment` or a `decrement` starts from when the route carries no
such attribute is hard-coded. `absentBase`
(`internal/component/bgp/plugins/filter_modify/modify.go`) declares MED 0 and
LOCAL_PREF 100, and an operator cannot change either.

MED 0 is the RFC's own number, so a fixed value is defensible there. LOCAL_PREF
100 is not: RFC 4271 Section 9.1.1 says of the degree of preference that "The
exact nature of this policy information, and the computation involved, is a
local matter", and 100 is a convention Ze inherited from FRR
(`BGP_DEFAULT_LOCAL_PREF`, `bgpd/bgpd.h`) and BIRD (`default_local_pref`,
`proto/bgp/config.Y`). A constant encodes one operator's policy as though the
standard named it, and both reference daemons expose the value as a knob.

Goal: `bgp { defaults { attribute { med 0; local-preference 100; } } }`, with
`absentBase` deriving from config rather than declaring the numbers itself.

The `defaults` container was designed to hold `admin-distance` alongside
`attribute`. It will not: `plan/spec-fixit-bgp-distance-declaration.md` moved
administrative distance to `rib { distance { } }` and deleted the BGP container,
so `defaults` holds `attribute` alone and this spec creates both.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/plugins.md` - the Route Attribute Modifier section, which
      carries the absent-attribute table the depended-on filter spec added
  → Decision: the page already states the three answers (med starts from 0,
    local-preference from 100, aigp writes nothing) and their RFC reasons. It
    gains only the sentence saying the first two are configurable, so the table
    is not restated (`ai/rules/principles.md`).
  → Constraint: the page also states the missing-as-worst consequence of writing
    `med 0` onto a route that carried none. Making the base configurable does not
    remove that consequence, it lets an operator move it.
- [ ] `docs/architecture/core-design.md` - declared as the design page by
      `filter_modify/modify.go`, the route attribute modifier
  → Decision: the page describes the modifier and is SILENT on where the absent
    base comes from, so it neither states nor contradicts the constant. It gains
    a pointer to the config leaves.
  → Constraint: the chain runs per UPDATE, so a config change must reach the
    plugin through its configure callback rather than being read per route.
- [ ] `docs/architecture/config/syntax.md` - how a YANG default reaches a
      running config
  → Constraint: the depended-on spec settles the rule for a non-peer section. A
    leaf's `default` is schema metadata, so whatever mechanism that spec chose
    for the `rib` section is the mechanism this one uses for `bgp defaults`.
- [ ] `docs/config-reference.md`, `docs/guide/configuration.md` - where the
      `bgp` block's containers are listed for an operator
  → Decision: both list the containers, so both gain the new one.

### RFC Summaries (Scope: config)
- [ ] `rfc/short/rfc4271.md` - Sections 9.1.1 and 9.1.2.2, which decide why one
      of these two values is a standard number and the other is local policy
  → Constraint: Section 9.1.2.2 names 0 for an absent MULTI_EXIT_DISC, and it
    names it as the value of a comparison function inside the Decision Process,
    not as a statement that the attribute exists. A configurable base changes
    what policy arithmetic starts from and MUST NOT silently change what phase 2
    compares.
- [ ] `rfc/short/rfc7311.md` - Sections 3.4.1 and 4.1, which are why AIGP gets
      no leaf
  → Decision: a route with no AIGP TLV is eliminated rather than scored, so
    there is no number to configure. Adding a leaf would invite an operator to
    set one and would make Ze create an attribute Section 3.4.1 forbids.

**Key insights:**
- The two values are not symmetric, and the schema must not pretend they are:
  one is an RFC number Ze is choosing to let an operator override, the other is
  a policy value Ze had no business fixing.
- The `defaults` block does NOT already exist. It was designed to arrive with
  `admin-distance` inside it, that container moved to `rib` instead, and this
  spec now creates both `defaults` and `attribute`.
- LOCAL_PREF has a second job in FRR that Ze does not yet perform, and this spec
  deliberately does not take it on.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/filter_modify/modify.go` - `absentBase` is
      a package-level map declaring `medAttr: 0` and `localPreferenceAttr: 100`,
      with the RFC reasoning in its doc comment. `currentForArithmetic` reads it
      on the `attrAbsent` branch and refuses the operation when no base is
      declared. `readUint32Attr` returns the three-state reading that separates
      absent from a present zero.
- [ ] `internal/component/bgp/plugins/filter_modify/filter_modify.go` - the SDK
      entry point, `logger`, and the `localPreferenceAttr`, `medAttr` and
      `aigpAttr` constants. The configure callback is where a config value would
      arrive.
- [ ] `internal/component/bgp/plugins/filter_modify/config.go` - how the plugin
      already parses its own config section into `modifyDef` values, which is the
      pattern a new leaf follows.
- [ ] `internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang` -
      the `increment` and `decrement` containers, each carrying
      `local-preference`, `med` and `aigp` with `range "1..4294967295"`.
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - the `bgp` container's
      existing children. There is no `defaults` container: the distance spec
      deleted `bgp { admin-distance }` and added nothing, so this spec creates
      `defaults` as well as `attribute`.
- [ ] `internal/component/config/schema_defaults.go` - `ApplyDefaults` and its
      two peer-entry callers, which is why a YANG default does not reach a
      non-peer section on its own.

**Behavior to preserve:**
- The three answers themselves on default settings: med starts from 0,
  local-preference from 100, aigp writes nothing.
- `currentForArithmetic` refusing the operation for an attribute with no
  declared base, and for a value that did not parse.
- The three-state reading that separates an absent attribute from a present
  zero.
- Every `increment` and `decrement` result on a route that DOES carry the
  attribute, which no default can affect.

**Behavior to change:**
- `absentBase` stops declaring the two numbers and derives them from config.
- An operator can set either value, and a reload changes what subsequent routes
  compute from.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator writes `bgp { defaults { attribute { med N; local-preference M; } } }`,
  and a peer later announces a route that carries neither attribute.
- Format at entry: the config tree as `map[string]any`, delivered to the plugin
  as a config section.

### Transformation Path
1. The config section reaches the `bgp-filter-modify` plugin's configure
   callback.
2. The two values are parsed and stored, replacing the package-level constants.
3. A route arrives, and the reactor renders the filter subject.
4. `currentForArithmetic` finds the attribute absent and reads the stored base.
5. `buildDynamicDelta` computes the new value and emits the delta.
6. The reactor turns the delta into wire modification ops.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config file to plugin | a config section delivered to the plugin's configure callback | Yes: `test/plugin/attribute-default-localpref-configured.ci` writes the block in a config file |
| Reactor to plugin | `rpc.FilterUpdateInput.Update`, the text subject | Yes: the same `.ci`, and `TestAttributeDefaultsReloadTakesEffect` at `handleFilterUpdate` |
| Plugin to reactor | `rpc.FilterUpdateOutput.Update`, the delta | Yes: the same two |
| Operator to peer | the re-announced route's MULTI_EXIT_DISC or LOCAL_PREF | Yes: `bgp-attribute-default-localpref-gobgp`, where GoBGP decodes LocalPref 130 |

### Integration Points
- `absentBase` and `currentForArithmetic`, which already have the shape a
  configured value slots into.
- The plugin's existing configure callback, which already parses a config
  section.
- The `defaults` container the depended-on spec creates under `bgp`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The value travels config file, config section, `applyBGPConfig`, `absentBase`, `currentForArithmetic`, delta, wire. The interop scenario reads the far end of that path at GoBGP |
| Registration over hardcoding | Yes | `parseAttributeDefaults` builds its map from the schema container's own children, so a leaf added to `bgp/defaults/attribute` needs no edit in the plugin, and a key the container does not declare is refused |
| One declaration of one fact | Yes | The two numbers are `default` statements in `ze-bgp-conf.yang`. `TestAbsentBaseComesFromConfig` compares the plugin's base against `config.SchemaDefaultInt` and then proves an empty base leaves no compiled-in fallback |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `absentBase` has exactly one reader, `currentForArithmetic` | Both are declared in `modify.go` and the map is package-private | A second reader would keep a path on the old constants after the config lands | `gopls references` on the symbol, recorded in the audit | confirmed, 2026-09-04. `gopls references` on the declaration answered three sites: `currentForArithmetic` (`modify.go`) and two in `modify_test.go`, which is `TestAbsentValueTableCoversEveryArithmeticAttribute` reading the map twice. One non-test reader |
| A-2 | The plugin's configure callback runs before any filter update is handled, so a route never computes from an unset base | `filter_modify.go` already warns "filter-update before configure" for the definitions map, which means the ordering is not guaranteed and is already handled | A route arriving first would compute from a zero value rather than the declared default | A test that calls the update path before configure and asserts the declared defaults apply | broken as an ordering claim, and the design does not rest on it. `handleFilterUpdate` still rejects a filter update that arrives before configure, and `absentBaseFor` (`modify.go`) answers that window from the schema's own defaults rather than from a zero value. `TestDefaultsApplyBeforeConfigure` drives it with the store held nil |
| A-3 | `bgp { defaults { } }` exists by the time this spec lands | BROKEN, 2026-09-04. The distance spec was expected to create it holding `admin-distance`, but its investigation moved administrative distance to `rib { distance { } }` and DELETED the BGP container outright, so nothing creates `bgp { defaults { } }`. Confirmed: no `container defaults` in `internal/component/bgp/yang/ze-bgp-conf.yang` | This spec creates the container itself, which is now the design rather than a duplication | done | broken |
| A-4 | Nothing outside the modifier reads a default MED or LOCAL_PREF today | The two numbers were introduced by `spec-fixit-filter-subject-drops-five-attributes` and live in one map | A second consumer would need the same values and would be a second declaration | A grep for the literals 100 and 0 near LOCAL_PREF and MED handling | broken, 2026-09-04. Six producers write 100 as the LOCAL_PREF ze emits toward an internal peer when the operator configured none, and none reads a declaration. That is the OTHER half of FRR's `default_local_pref`, which Known Limitations already places outside this spec, so the design holds. The find is one row in `plan/journal/helper-bypassed-by-an-open-coded-copy.md`. Nothing outside the modifier reads a default MED |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The configured MED base is applied to the Decision Process comparison as well as to policy arithmetic | A best-path selection changes on a config that only meant to change a modifier | The leaf's description states the scope, and a test asserts phase 2 comparison is unaffected. RFC 4271 Section 9.1.2.2 names 0 there, and moving it is a deviation this spec does not make |
| R-2 | An operator sets a local-preference default and expects it to apply to eBGP-learned routes generally, as FRR's does | A support question, or a route reflected into iBGP carrying no LOCAL_PREF | The leaf's description says what it governs and what it does not. The wider job is named in Known Limitations rather than half-implemented |
| R-3 | The plugin computes from an unset base if a route arrives before configure | A route's arithmetic answers from 0 rather than 100 | A-2's test. The stored value is initialized to the declared defaults rather than to the zero value |
| R-4 | Adding an `aigp` leaf later looks like a natural symmetry and violates RFC 7311 | A review comment asking why aigp is missing | The YANG description states the RFC 7311 reason at the container, so the absence is documented where somebody would go to add it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | What `increment` and `decrement` compute for every route that arrives without the attribute, on every chain using them. A wrong base changes the metric or the preference Ze re-announces |
| How is it reverted? | A single commit revert. The values return to the constants, and a config naming the leaves fails to load |
| Who else touches this path? | `spec-fixit-filter-subject-drops-five-attributes` created `absentBase` and its three answers in `internal/component/bgp/plugins/filter_modify/modify.go`, and it closed on 2026-09-04, so read the code rather than the spec. `plan/spec-fixit-bgp-distance-declaration.md` creates the `defaults` block this container sits in, and also lands first |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An operator writes `bgp { defaults { attribute { local-preference 80; } } }` and a peer announces a route with no LOCAL_PREF through a chain carrying `increment { local-preference 50; }` | → | `currentForArithmetic` reads 80 and the delta is `local-preference 130` | `test/plugin/attribute-default-localpref-configured.ci` |
| An operator writes no `defaults` block at all and the same route arrives | → | the declared defaults apply and the delta is `local-preference 150` | `test/plugin/modify-increment-localpref.ci`, the existing test |
| An operator writes `bgp { defaults { attribute { med 50; } } }` and a route with no MED meets `decrement { med 30; }` | → | the delta is `med 20` | `TestConfiguredMedBaseFeedsTheArithmetic` |
| An operator writes `bgp { defaults { attribute { aigp 10; } } }` | → | the config is refused; no such leaf exists | `TestAigpDefaultLeafDoesNotExist` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | No `defaults { attribute { } }` block in the config | med starts from 0 and local-preference from 100, unchanged from the constants they replace |
| AC-2 | `local-preference 80` configured, route carries no LOCAL_PREF, chain increments by 50 | The delta is `local-preference 130` |
| AC-3 | `med 50` configured, route carries no MED, chain decrements by 30 | The delta is `med 20` |
| AC-4 | `med 50` configured, route carries no MED, chain decrements by 80 | The delta is `med 0`, because the decrement still floors |
| AC-5 | Either value configured, and the route DOES carry the attribute | The configured base is not used, and the arithmetic runs on the route's own value |
| AC-6 | `aigp` named inside `defaults { attribute { } }` | The config is refused. No leaf exists, and the container's description says why |
| AC-7 | A reload changes `local-preference` from 100 to 80 | Routes arriving after the reload compute from 80 |
| AC-8 | A filter update is handled before the plugin's configure callback has run | The declared defaults apply, and no route computes from a zero the operator did not choose |
| AC-9 | `med 50` is configured | Decision Process phase 2 still treats an absent MULTI_EXIT_DISC as 0 per RFC 4271 Section 9.1.2.2, and the configured base reaches policy arithmetic alone |
| AC-10 | The two numbers are read anywhere | They come from one declaration. `absentBase` no longer states 0 or 100 as literals |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Sets a site-wide default local preference of 80 and increments by 50 on one peer | config to the plugin's configure callback to `currentForArithmetic` to the delta to the re-announced route | `test/plugin/attribute-default-localpref-configured.ci` |
| 2 | Leaves the block out and expects the documented behavior | the declared defaults to the same path | `test/plugin/modify-increment-localpref.ci` |
| 3 | Tries to configure an AIGP default and is told there is none | config to the validator to a refusal | `TestAigpDefaultLeafDoesNotExist` |
| 4 | Changes the default and reloads | reload to the configure callback to subsequent routes | `TestAttributeDefaultsReloadTakesEffect` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAbsentBaseComesFromConfig` | `internal/component/bgp/plugins/filter_modify/modify_test.go` | AC-10: no literal 0 or 100 remains in the map's declaration | PASSES |
| `TestConfiguredLocalPrefBaseFeedsTheArithmetic` | `internal/component/bgp/plugins/filter_modify/modify_test.go` | AC-2 | PASSES |
| `TestConfiguredMedBaseFeedsTheArithmetic` | `internal/component/bgp/plugins/filter_modify/modify_test.go` | AC-3 and AC-4 | PASSES |
| `TestConfiguredBaseIsUnusedWhenTheRouteCarriesTheAttribute` | `internal/component/bgp/plugins/filter_modify/modify_test.go` | AC-5 | PASSES |
| `TestConfiguredBaseArithmeticHoldsAtTheBoundaries` | `internal/component/bgp/plugins/filter_modify/modify_test.go` | the arithmetic row of the boundary table: saturation at the top, floor at 0 | PASSES |
| `TestDefaultsApplyBeforeConfigure` | `internal/component/bgp/plugins/filter_modify/modify_test.go` | AC-8 and A-2 | PASSES |
| `TestAttributeDefaultsReloadTakesEffect` | `internal/component/bgp/plugins/filter_modify/config_test.go` | AC-7 | PASSES |
| `TestAttributeDefaultsRefusedConfigInstallsNothing` | `internal/component/bgp/plugins/filter_modify/config_test.go` | a delivery that does not parse installs neither the base nor the modifiers | PASSES |
| `TestAttributeDefaultLeafRangeHoldsAtTheBoundaries` | `internal/component/bgp/plugins/filter_modify/config_test.go` | the two leaf rows of the boundary table, through `config.ValidateLeafValue` | PASSES |
| `TestAigpDefaultLeafDoesNotExist` | `internal/component/bgp/plugins/filter_modify/config_test.go` | AC-6. NOT the file this table first named: `internal/component/config/yang` cannot host it, because the lookup needs the bgp module and `internal/component/bgp/yang` imports that package, so an in-package test there is an import cycle | PASSES |
| `TestAbsentMedStillComparesAsZeroInPhaseTwo` | `internal/component/bgp/plugins/rib/` best-path test | AC-9: the configured base does not reach the Decision Process | not this agent's phase |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `defaults attribute med` | 0 to 4294967295 | 4294967295 | N-A, 0 is valid and is the declared default | 4294967296, refused by the YANG range |
| `defaults attribute local-preference` | 0 to 4294967295 | 4294967295 | N-A, 0 is valid | 4294967296, refused by the YANG range |
| the arithmetic result after a configured base | 0 to 4294967295 | saturates at the top, floors at 0 | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `attribute-default-localpref-configured` | `test/plugin/attribute-default-localpref-configured.ci` | An operator sets the default to 80, a peer announces a route with no LOCAL_PREF, and the delta is 130 | written; the `./le functional plugin` result is in the phase report |
| `modify-increment-localpref` | `test/plugin/modify-increment-localpref.ci` | The existing test, re-run to prove the declared defaults still apply with no block written | unchanged; same run |

### Interop Tests (Scope: config)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-attribute-default-localpref-gobgp` | `test/interop/scenarios/bgp-attribute-default-localpref-gobgp` | GoBGP | A raw injector announces a route carrying no LOCAL_PREF, ze's import chain increments by 50 from a configured base of 80, and GoBGP reads 130. Reverting the config plumbing makes it read 150 | PASSES. RED recorded 2026-09-04: with `absentBaseFor` reverted to ignore the stored value and the `ze-interop` image rebuilt, the run failed `assertion 5: GoBGP decoded LocalPref 150 for 10.64.0.0/24, want 130`. Restored, green again |

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An operator can set the two values | interop | `test/interop/scenarios/bgp-attribute-default-localpref-gobgp`, where a peer daemon reads 130 rather than 150. A unit test over the map is not evidence here, for the reason the depended-on spec recorded: a unit test over a hand-written subject is the shape that hid this area's last defect for its whole life |
| The default behavior is unchanged | functional | `test/plugin/modify-increment-localpref.ci`, unchanged and still asserting 150 with no block written |
| The numbers are declared once | unit | `TestAbsentBaseComesFromConfig`, plus a grep recorded in the audit |
| Policy arithmetic and the Decision Process stay separate | unit | `TestAbsentMedStillComparesAsZeroInPhaseTwo` |

## Files to Modify
- `internal/component/bgp/yang/ze-bgp-conf.yang` - the `defaults` container
  AND the `attribute` container inside it, two leaves with
  `range "0..4294967295"`, and a description at `attribute` saying why there is
  no `aigp` leaf. A-3 is broken, so this spec creates `defaults` rather than
  finding it. The distance spec edits the same file (it deletes
  `bgp { admin-distance }`), so that one lands first to keep the two edits from
  colliding; nothing else in this spec depends on it
- `internal/component/bgp/plugins/filter_modify/modify.go` - `absentBase` stops
  declaring the numbers and reads the configured values;
  `currentForArithmetic`'s doc comment moves the RFC reasoning to the config
  declaration rather than restating it
- `internal/component/bgp/plugins/filter_modify/config.go` - parse the new
  section
- `internal/component/bgp/plugins/filter_modify/filter_modify.go` - store the
  values from the configure callback, initialized to the declared defaults
- `docs/guide/plugins.md` - one sentence saying the first two rows of the
  absent-attribute table are configurable, and naming the leaves
- `docs/architecture/core-design.md` - a pointer from the modifier section to
  the config leaves
- `docs/config-reference.md`, `docs/guide/configuration.md` - the new container

## Files to Create
- `test/plugin/attribute-default-localpref-configured.ci`
- `test/interop/scenarios/bgp-attribute-default-localpref-gobgp/` with its
  `ze.conf`, `gobgp.toml`, `inject.msg` and `inject-args`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-bgp-conf.yang` gains `defaults { attribute { } }` |
| YANG validation constraints | Yes | both leaves carry `range "0..4294967295"`; 0 is valid for both and is MED's declared default |
| YANG custom validators | No | a range is sufficient; no cross-leaf rule exists |
| CLI commands/flags | No | no command is added |
| CLI grammar (keyword before value) | N-A | no command added |
| Editor autocomplete | Yes | DERIVED from the schema; confirm the new leaves complete |
| Functional test for new RPC/API | Yes | `test/plugin/attribute-default-localpref-configured.ci` |
| Pipe completeness | N-A | no command output changes |
| Env var registration | No | none added |
| Doctor check for runtime dependencies | N-A | no file, socket, port, module or binary is added |
| Prometheus counters/metrics | No | a default is config, not a rate |
| BGP family surface (new SAFI / capability / attribute) | N-A | no family, capability or attribute code is added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/plugins.md`, `docs/guide/configuration.md` |
| 2 | Config syntax changed? | Yes | `docs/config-reference.md`, `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | the filter text protocol is unchanged |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, the Route Attribute Modifier section |
| 6 | Has a user guide page? | Yes | `docs/guide/redistribution.md` carries filter chain configuration and is checked for the same claim |
| 7 | Wire format changed? | No | no wire change |
| 8 | Plugin SDK/protocol changed? | No | the subject and the delta are unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4271.md` Section 9.1.1, where the local-matter clause becomes a configurable value rather than a constant |
| 10 | Test infrastructure changed? | No | the runners are unchanged |
| 11 | Affects daemon comparison? | Yes | Ze gains a knob FRR and BIRD both have. `docs/features/` comparison rows, if one names it |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, the modifier section |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | no registration changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/spec-bgp-attribute-defaults.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/plugins.md` shows `increment { local-preference 50; }` and `decrement { med 30; }`, and each is checked against what the code now computes |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - prove the config path does not exist yet
   - Tests: `test/plugin/attribute-default-localpref-configured.ci`,
     `TestConfiguredLocalPrefBaseFeedsTheArithmetic`
   - Files: the new `.ci`, `modify_test.go`
   - Verify: red, and the red is a refused config rather than a wrong number
2. **Phase: the schema** - the container, its two leaves, and the description
   that says why `aigp` is absent
   - Tests: `TestAigpDefaultLeafDoesNotExist`
   - Files: `ze-bgp-conf.yang`
   - Verify: the config loads, and an `aigp` leaf is refused
3. **Phase: the plumbing** - the plugin reads the values and `absentBase`
   derives from them
   - Tests: phase 1's, plus `TestAbsentBaseComesFromConfig`,
     `TestConfiguredMedBaseFeedsTheArithmetic`,
     `TestConfiguredBaseIsUnusedWhenTheRouteCarriesTheAttribute`,
     `TestDefaultsApplyBeforeConfigure`
   - Files: `modify.go`, `config.go`, `filter_modify.go`
   - Verify: green, and no literal 0 or 100 remains in the map's declaration
4. **Phase: the separation** - prove the configured MED base does not reach the
   Decision Process
   - Tests: `TestAbsentMedStillComparesAsZeroInPhaseTwo`
   - Files: none expected. A file change here means the two jobs were already
     entangled, which is a defect to root cause
   - Verify: phase 2 comparison is unchanged with a non-zero base configured
5. **Phase: the reload** - a changed default takes effect
   - Tests: `TestAttributeDefaultsReloadTakesEffect`
   - Files: `filter_modify.go`
   - Verify: routes after the reload compute from the new value
6. **Phase: interop** - prove it against a peer daemon
   - Tests: `test/interop/scenarios/bgp-attribute-default-localpref-gobgp`
   - Files: the new scenario directory
   - Verify: red with the plumbing reverted and the image rebuilt, recorded per
     `ai/rules/interop-and-goal-validation.md`
7. **Phase: the pages** - repair every page the change made wrong
   - Tests: none. `./le spec citation anchors` names the pages
   - Files: the Documentation Update Checklist's rows
   - Verify: no page still calls either value fixed

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Correctness | `absentBase` has no path returning a compiled-in 0 or 100 once config is wired |
| Correctness | The configured MED base reaches policy arithmetic and NOT the phase 2 comparison, checked by reading the comparison rather than by test alone |
| Naming | The container is `attribute`, singular, matching `admin-distance`'s sibling shape in the same `defaults` block |
| Data flow | A configured value is traced from the config file to a re-announced route |
| Rule: `ai/rules/principles.md` | One fact declared once: the two numbers exist in the schema and nowhere else |
| Rule: `ai/rules/rfc-compliance.md` | The AIGP absence is documented at the container with its RFC 7311 citation, so a later reader cannot add the leaf without meeting the reason |
| Rule: `ai/rules/config.md` | Both leaves carry range constraints and help text stating what they govern and what they do not |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The two leaves exist and validate | Load a config using them, and one using an out-of-range value |
| `absentBase` declares no literal | Read the declaration |
| No `aigp` leaf exists | `grep aigp internal/component/bgp/yang/ze-bgp-conf.yang` inside the defaults container returns only the description |
| The interop scenario exists and is named, with no numeric prefix | `ls test/interop/scenarios/bgp-attribute-default-localpref-gobgp` |
| The Decision Process is untouched | Read the phase 2 comparison and confirm it reads no configured base |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Guard that could fail open | An import filter is a guard. A changed base must not make any existing filter more permissive, which story 2 checks by leaving the default behavior asserted |
| Input validation | Both leaves are range-constrained in YANG, so the plugin receives a bounded uint32 |
| Guard that could fail open | A route handled before configure must use the declared defaults, not a zero value. AC-8 is that check |
| Resource exhaustion | N-A. Two scalars |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood, back to RESEARCH |
| Lint failure | Fix inline. If architectural, back to DESIGN |
| Functional test fails | Check the AC: wrong AC to DESIGN, correct AC to IMPLEMENT |
| Phase 4 needs a code change | The two jobs were entangled before this spec. Root cause it under `ai/rules/completion.md` rather than routing the base around it |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The asymmetry is the design. Making both values configurable is right, but for
  different reasons, and a schema that presents them as one kind of thing would
  invite the AIGP leaf that RFC 7311 forbids.
- FRR's `default_local_pref` does two jobs, and Ze is taking on one of them. The
  other, supplying the degree of preference an eBGP-learned route carries into
  iBGP readvertisement, is what RFC 4271 Section 9.1.1 actually describes, and
  it is a larger feature than a modifier default.
- The MED knob is the one that could quietly become a conformance deviation. The
  phase that proves it does not is the phase that expects to change no code.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Two leaves, and no `aigp` | Three leaves for symmetry | RFC 7311 Section 4.1 eliminates a route with no AIGP TLV rather than scoring it, so no number stands in for the absence; Section 3.4.1 forbids creating the attribute outside the AIGP administrative domain. A leaf would invite exactly that |
| The base governs policy arithmetic only | One value serving both the arithmetic and the phase 2 comparison | RFC 4271 Section 9.1.2.2 names 0 for the comparison. Moving that is a deliberate deviation, and FRR ships it separately as `bgp bestpath med missing-as-worst`. Folding them means an operator tuning a modifier silently changes best-path selection |
| Global, with no per-peer override | Per-peer, inheritable through `group` | Owner decision, 2026-09-04. A per-peer override needs an inheritance rule and a resolution order, and no requirement asks for one yet |
| `attribute`, singular, inside `defaults`, kept even though `defaults` now holds nothing else | The leaves directly in `defaults`, which is three levels rather than four to reach a value | Owner decision, 2026-09-04, taken AFTER the sibling disappeared. The original reason for the nesting was to separate two kinds of default from each other, and administrative distance leaving for `rib { distance { } }` removed the sibling it was being separated from. The nesting is kept anyway: `attribute` names what these values are defaults FOR, which a bare `defaults { med 0; }` does not, and it leaves the block able to take a second kind of default without moving an operator's existing leaves |

## Known Limitations

- FRR's `default_local_pref` also supplies the LOCAL_PREF an eBGP-learned route
  carries into iBGP readvertisement, which RFC 4271 Section 9.1.1 requires of a
  speaker. Ze does not do that here, and this leaf does not start doing it. That
  is a separate feature and wants its own spec.
- The MED comparison in Decision Process phase 2 stays at the RFC's 0. Ze offers
  no equivalent of FRR's `bgp bestpath med missing-as-worst`, and adding one is
  a deliberate RFC deviation that needs the owner's decision.
- A route handled between daemon start and the plugin's configure callback uses
  the declared defaults. That is correct rather than a limitation, but it means
  a config value is never retroactive for routes already processed.

## RFC Documentation (Scope: config)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.

Two citations are owed and both already exist on `absentBase` in
`internal/component/bgp/plugins/filter_modify/modify.go`: RFC 4271 Section
9.1.2.2 for the MED value, and RFC 7311 Sections 4.1 and 3.4.1 for the AIGP
absence. When the numbers move to the schema, the quotations move with them to
the YANG descriptions, and the Go comment points at the declaration rather than
repeating it. RFC 4271 Section 9.1.1's local-matter clause is quoted at the
`local-preference` leaf, because it is the sentence that makes the value
configurable rather than fixed.

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

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
- `bgp { defaults { attribute { med, local-preference } } }` in
  `internal/component/bgp/yang/ze-bgp-conf.yang`. Both leaves carry
  `range "0..4294967295"` and a default, each leaf's `ze:help` carries the RFC
  sentence behind its own number, and the container's description carries RFC
  7311's reason for having no `aigp` leaf.
- `parseAttributeDefaults` (`internal/component/bgp/plugins/filter_modify/config.go`)
  reads that container, refuses a key the container does not declare, and fills
  a leaf the operator left out from the schema through `config.ApplyDefaults`.
  `attributeDefaultsContainer` resolves the container once per process, because
  `config.YANGSchema` rebuilds and re-resolves every module on each call.
- `applyBGPConfig` (`filter_modify.go`) installs one delivery: the base first,
  then the modifiers, and neither when the delivery does not parse.
- `absentBaseFor` and `declaredAbsentBase` (`modify.go`) replace the
  package-level `absentBase` map. No literal 0 or 100 remains in the plugin.
- AC-9's proof in `internal/component/bgp/plugins/rib/rfc4271_test.go`, with
  `RFC4271-9.1.2.2-4` declared in `rfc/short/rfc4271.md` and its discrimination
  record in `rfc/discrimination/rfc4271.json`. No product file under
  `plugins/rib/` changed, which is what step 4 predicted.
- `test/plugin/attribute-default-localpref-configured.ci`, and the interop
  scenario `test/interop/scenarios/bgp-attribute-default-localpref-gobgp` with
  its checker `checkAttributeDefaultLocalPref`
  (`internal/le/interoplab/bgp/check_special.go`).

### Bugs Found/Fixed
- The `absentBase` doc comment quoted a sentence RFC 4271 does not contain. The
  verified Section 9.1.2.2 (c) sentence now sits in the `med` leaf's `ze:help`.
  Row: `plan/journal/reference-checked-claim-unchecked.md`.
- Six producers write 100 as the LOCAL_PREF ze emits toward an internal peer,
  with no declaration behind any of them (A-4, broken). Out of this spec's
  scope by Known Limitations. Row:
  `plan/journal/helper-bypassed-by-an-open-coded-copy.md`.
- The redistribution section of `docs/guide/configuration.md` claimed an
  intra-BGP `IngressFilter` reads the `redistribute` block and anchored on
  `redistribute_ingress/filter.go`, a package commit `1ec5b741f8` deleted.
  Fixed here because closure was already carrying that page. Row:
  `plan/journal/claim-outlives-the-evidence-it-cites.md`.

### Documentation Updates
- `docs/architecture/core-design.md`, the route attribute modifier section, with
  anchors on `parseAttributeDefaults` and on `absentBaseFor, currentForArithmetic`.
- `docs/config-reference.md`, a new `bgp { defaults { attribute { } } }` section
  with the two leaves, their defaults, their range, and what they do NOT govern.
  Anchor: `ze-bgp-conf.yang -- container defaults`.
- `docs/guide/configuration.md`, the block and its example beside the modifier
  table. Same anchor.
- `docs/guide/plugins.md`, one paragraph saying the first two rows of the
  absent-attribute table are configurable and naming the leaves.
- `docs/architecture/route-selection.md`, the `lost-med` row, which now states
  that Section 9.1.2.2 (c) gives a route with no MULTI_EXIT_DISC the lowest
  possible value.
- `./le doc check verify` FAILS on this tree, and on nothing this spec owns: 8
  stale `../gh-pages/reference/command-equivalents/` pages, four command
  descriptions belonging to other sessions, and a stale `ai/DOCS-TO-CODE.md`
  warning. Its source-anchor stage is now clean, which it was not before the
  `docs/guide/configuration.md` repair above.

### Deviations from Plan
- A-3 was broken before implementation started: nothing created
  `bgp { defaults { } }`, so this spec created the container as well as
  `attribute`.
- AC-6's test could not live in `internal/component/config/yang`: the lookup
  needs the bgp module, and `internal/component/bgp/yang` imports that package,
  so an in-package test there is an import cycle. It lives in
  `filter_modify/config_test.go`.
- `filter_modify/schema_modules_test.go` was added and is not in Files to
  Create. The package's own test binary links only what it imports, and the
  YANG loader resolves the module CLOSURE, so `ze-hub-conf` has to be registered
  for a lookup in `ze-bgp-conf` to resolve.
- `internal/le/interoplab/bgp/check_special.go` and its branch in `bgp_test.go`
  were needed for the interop scenario and are not in Files to Modify. A
  scenario with no registered checker asserts nothing.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-3 assumed `bgp { defaults { } }` would exist by the time this spec landed, created by the distance spec | That spec moved administrative distance to `rib { distance { } }` and deleted the BGP container outright, so nothing created it | `grep "container defaults" internal/component/bgp/yang/ze-bgp-conf.yang` answered nothing | This spec creates the container. Files to Modify records it |
| assumption | A-4 assumed nothing outside the modifier reads a default MED or LOCAL_PREF | Six producers write 100 as the LOCAL_PREF toward an internal peer, none of them reading a declaration | The grep the assumption itself named | Journal row in `plan/journal/helper-bypassed-by-an-open-coded-copy.md`. Out of scope by Known Limitations, which already named the second job as wanting its own spec |
| assumption | A-2 assumed the configure callback always runs before the first filter update | `handleFilterUpdate` rejects an update that arrives first, so the ordering is not guaranteed and the plugin already handles it | Reading `handleFilterUpdate` rather than trusting the assumption | The design does not rest on the ordering: `absentBaseFor` answers the window from the schema, and `TestDefaultsApplyBeforeConfigure` drives it |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| An operator can set the two values | Done | `container defaults`, `internal/component/bgp/yang/ze-bgp-conf.yang` | Both leaves range-constrained, both with a default |
| `absentBase` derives from config rather than declaring the numbers | Done | `absentBaseFor`, `internal/component/bgp/plugins/filter_modify/modify.go` | The map literal is gone; the plugin holds no 0 and no 100 |
| MED keeps the RFC's number as its default, LOCAL_PREF is declared as local policy | Done | the two leaves' `ze:help` in `ze-bgp-conf.yang` | Section 9.1.2.2 quoted at `med`, Section 9.1.1 at `local-preference` |
| The `defaults` container is created by this spec | Done | `ze-bgp-conf.yang` | A-3 broken; recorded in the Mistake Log |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestAbsentBaseComesFromConfig`, `TestDefaultsApplyBeforeConfigure`, `test/plugin/modify-increment-localpref.ci` | The unchanged `.ci` still asserts 150 with no block written |
| AC-2 | Done | `TestConfiguredLocalPrefBaseFeedsTheArithmetic`, `test/plugin/attribute-default-localpref-configured.ci`, `bgp-attribute-default-localpref-gobgp` | GoBGP decodes 130 |
| AC-3 | Done | `TestConfiguredMedBaseFeedsTheArithmetic` | 50 less 30 is 20 |
| AC-4 | Done | `TestConfiguredMedBaseFeedsTheArithmetic` | 50 less 80 floors at 0 |
| AC-5 | Done | `TestConfiguredBaseIsUnusedWhenTheRouteCarriesTheAttribute` | 100 and 200 from the route, not 50 and 80 from the config |
| AC-6 | Done | `TestAigpDefaultLeafDoesNotExist` | The lookup fails, the container declares two children, and `parseAttributeDefaults` refuses an `aigp` key |
| AC-7 | Done | `TestAttributeDefaultsReloadTakesEffect` | 100, then 80, then 100 again, so a latched base fails |
| AC-8 | Done | `TestDefaultsApplyBeforeConfigure` | The store held nil, and the arithmetic answers 150 and 50 |
| AC-9 | Done | `TestAbsentMedStillComparesAsZeroInPhaseTwo`, `TestAbsentMedTiesAnExplicitMedOfZero` | No file under `plugins/rib/` needed a change |
| AC-10 | Done | `TestAbsentBaseComesFromConfig`, and `grep -n "100" internal/component/bgp/plugins/filter_modify/*.go` outside tests answers only a community example | The base is compared against `config.SchemaDefaultInt`, then emptied to prove no compiled-in fallback |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestAbsentBaseComesFromConfig` | Done | `filter_modify/modify_test.go` | |
| `TestConfiguredLocalPrefBaseFeedsTheArithmetic` | Done | `filter_modify/modify_test.go` | |
| `TestConfiguredMedBaseFeedsTheArithmetic` | Done | `filter_modify/modify_test.go` | |
| `TestConfiguredBaseIsUnusedWhenTheRouteCarriesTheAttribute` | Done | `filter_modify/modify_test.go` | |
| `TestConfiguredBaseArithmeticHoldsAtTheBoundaries` | Done | `filter_modify/modify_test.go` | |
| `TestDefaultsApplyBeforeConfigure` | Done | `filter_modify/modify_test.go` | |
| `TestAttributeDefaultsReloadTakesEffect` | Done | `filter_modify/config_test.go` | Driven through `handleFilterUpdate`, not through `buildDynamicDelta` |
| `TestAttributeDefaultsRefusedConfigInstallsNothing` | Done | `filter_modify/config_test.go` | |
| `TestAttributeDefaultLeafRangeHoldsAtTheBoundaries` | Done | `filter_modify/config_test.go` | Through `config.ValidateLeafValue` |
| `TestAigpDefaultLeafDoesNotExist` | Changed | `filter_modify/config_test.go` | Moved out of `internal/component/config/yang`: import cycle. Recorded in Deviations |
| `TestAbsentMedStillComparesAsZeroInPhaseTwo` | Done | `plugins/rib/rfc4271_test.go` | Plus `TestAbsentMedTiesAnExplicitMedOfZero`, the negative polarity |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/yang/ze-bgp-conf.yang` | Done | `defaults` and `attribute` both created |
| `internal/component/bgp/plugins/filter_modify/modify.go` | Done | |
| `internal/component/bgp/plugins/filter_modify/config.go` | Done | |
| `internal/component/bgp/plugins/filter_modify/filter_modify.go` | Done | |
| `docs/guide/plugins.md` | Done | |
| `docs/architecture/core-design.md` | Done | |
| `docs/config-reference.md`, `docs/guide/configuration.md` | Done | |
| `test/plugin/attribute-default-localpref-configured.ci` | Done | |
| `test/interop/scenarios/bgp-attribute-default-localpref-gobgp/` | Done | Four files |
| `internal/component/bgp/plugins/filter_modify/config_test.go` | Changed | Not in the plan; AC-6 and AC-7 live here |
| `internal/component/bgp/plugins/filter_modify/schema_modules_test.go` | Changed | Not in the plan; the test binary's module closure |
| `internal/le/interoplab/bgp/check_special.go`, `bgp_test.go` | Changed | Not in the plan; the scenario's checker |
| `internal/component/bgp/plugins/rib/rfc4271_test.go`, `rfc/short/rfc4271.md`, `rfc/requirements/rfc4271.md`, `rfc/discrimination/rfc4271.json`, `docs/architecture/route-selection.md` | Changed | AC-9's proof and its ledger row |

### Audit Summary
- **Total items:** 10 AC, 11 tests, 13 file rows
- **Done:** 10 AC, 10 tests, 9 file rows
- **Partial:** none
- **Skipped:** none
- **Changed:** 1 test (AC-6's file), 4 file rows added beyond the plan, each in Deviations

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none: the spec's metadata declares `Deferral shard: -`, and `ls plan/deferrals/spec-bgp-attribute-defaults.md` reports no such file | done | Nothing was deferred. The two out-of-scope finds are journal rows, not deferrals: `plan/journal/helper-bypassed-by-an-open-coded-copy.md` and `plan/journal/reference-checked-claim-unchecked.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/bgp-attribute-defaults-78c6feb8-8c75-4b7f-b997-a31a03785caf.md`, 11 code files |
| `./le spec session review check` | `review_gate: OK (11 code files, clean, hashes match)` |
| Rounds | 1. Round 1 found 0 BLOCKER and 0 ISSUE, so no code changed and a second pass would read the same bytes |
| Reviewer lenses used | wiring and top-down feature walk; functional and interop coverage; documentation drift and source anchors; removed-behavior and test-rewrite audit; logic, guard and zero-value audit; allocation and hot-path performance; security over the config input; the `ze-go-style` pass over every changed Go file; RFC conformance over RFC 4271 Sections 9.1.1 and 9.1.2.2 and RFC 7311 Sections 3.4.1 and 4.1 |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | - | No BLOCKER and no ISSUE. The five NOTEs are in the artifact's findings block and none of them blocks | - | - |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/attribute-default-localpref-configured.ci` | Yes | `ls` lists it; its `expect=stderr:pattern=local-preference 130` is the assertion |
| `test/interop/scenarios/bgp-attribute-default-localpref-gobgp/` | Yes | `ls` lists `gobgp.toml`, `inject-args`, `inject.msg`, `ze.conf` |
| `test/plugin/modify-increment-localpref.ci` | Yes | `ls` lists it, unchanged by this spec |
| `rfc/discrimination/rfc4271.json` | Yes | `ls` lists it; it holds both polarities of `RFC4271-9.1.2.2-4` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | The declared defaults apply with no block written | `./le functional plugin` run of 2026-09-04 22:51: `427/735 PASS modify-increment-localpref`, which asserts 150 |
| AC-2 | A configured 80 makes the delta 130 | Same run: `72/735 PASS attribute-default-localpref-configured`. GoBGP read 130 in the interop run, and 150 with the plumbing reverted |
| AC-6 | No `aigp` leaf exists | `grep -n aigp internal/component/bgp/yang/ze-bgp-conf.yang` answers one line, the container description that says why there is none |
| AC-9 | Phase 2 still compares an absent MULTI_EXIT_DISC as 0 | `rfc/requirements/rfc4271.md` carries `RFC4271-9.1.2.2-4` with both polarities, and `./le rfc check` names no rfc4271 violation |
| AC-10 | No literal 0 or 100 is left in the plugin | `grep -n "100" internal/component/bgp/plugins/filter_modify/*.go` outside tests answers one line, the `65000:100` community example in a comment |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| An operator writes `local-preference 80` and a route with no LOCAL_PREF meets `increment { local-preference 50; }` | `test/plugin/attribute-default-localpref-configured.ci` | Yes: the file writes the block in the daemon's config and expects `local-preference 130` on the modify apply line. It passed in the 2026-09-04 22:51 suite run |
| An operator writes no block at all | `test/plugin/modify-increment-localpref.ci` | Yes: unchanged, still expects 150, passed in the same run |
| An operator's number reaches a foreign peer | `test/interop/scenarios/bgp-attribute-default-localpref-gobgp` | Yes: `checkAttributeDefaultLocalPref` reads GoBGP's RIB for LocalPref 130 on the tagged prefix and for no LocalPref at all on the control prefix |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `gopls references` on the declaration answered `currentForArithmetic` and two test sites. One non-test reader |
| A-2 | broken as an ordering claim | `handleFilterUpdate` rejects a filter update that arrives before configure, so the ordering was never guaranteed. The design does not rest on it: `absentBaseFor` answers that window from the schema, driven by `TestDefaultsApplyBeforeConfigure` |
| A-3 | broken | No `container defaults` existed in `ze-bgp-conf.yang`. This spec creates it |
| A-4 | broken | Six producers write 100 as the LOCAL_PREF toward an internal peer. Journal row written; out of scope by Known Limitations |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| 1, 2, 5, 6: the new block and the modifier's absent-attribute table | `docs/config-reference.md`, `docs/guide/configuration.md` and `docs/guide/plugins.md` each state the two leaves, their defaults and their range, checked against `container defaults` in `ze-bgp-conf.yang` | Yes |
| 9: the RFC row | `rfc/short/rfc4271.md` declares `RFC4271-9.1.2.2-4` in the RFC's own words, and `docs/features/rfc-status.md` regenerated to `101 gated: 76 proven` | Yes |
| 11: the comparison table | `grep -n "default-local-pref\|local_pref\|BGP_DEFAULT_LOCAL_PREF" docs/comparison.md docs/features.md` answers nothing: no row names this knob, so no row needs a change | Yes, No update needed |
| 12: internal architecture | `docs/architecture/core-design.md` gained the paragraph and two anchors on the producing functions | Yes |
| 16: existing anchors on changed files | The source-anchor stage of `./le doc check verify` reports every anchor on this spec's files resolving. The one stale anchor it found named a package another commit deleted, and this closure repaired it | Yes |
| 3, 4, 7, 8, 10, 13, 14, 15: CLI, API, wire format, SDK, test infrastructure, route metadata, metrics, registrations | No command, RPC, event, send type, capability, metric or plugin registration changed. This spec's file set holds no `register.go`, no `pkg/plugin/rpc` file and no `testdata/*.snapshot` | Yes, No update needed |

### Gate state at closure

`./le verify current mode changed` reports 52 lint issues over this checkout and
NONE of them names a file this spec touches: they are `goconst`, `gocritic`,
`godot`, `gofmt`, `gosec`, `misspell`, `modernize`, `nilnil`, `noctx`,
`unconvert`, `unparam` and `unused` findings in `internal/test/fixture`,
`internal/component/iface`, `internal/component/ike`, `internal/test/mock/radius`
and `redistribute_egress`, each belonging to another session's uncommitted work
(`ai/rules/principles.md`).

`./le verify worktree` judges HEAD rather than this checkout: it runs
`git worktree add --detach <path> <sha>` at the current HEAD SHA
(`internal/le/verify/lifecycle.go`) and verifies that tree. Its lint stages
report 67 host issues, a `typecheck` failure where
`internal/component/doctor/checks_redistribute_test.go` and
`checks_mpls_linux_test.go` both declare `codes`, and 2 `nilnil` findings in the
capability flavor. All are HEAD's, and one of them, a `godot` finding on
`filter_modify/modify.go`, names the very comment this spec DELETED.

`./le repository check` is down to one issue, `AllRPCDocs` in
`internal/component/config/yang/cli/tree.go`, which belongs to another session.
`./le rfc check` names no rfc4271 violation. `./le doc check verify` fails on 8
stale `../gh-pages/` pages, four foreign command descriptions and a stale
`ai/DOCS-TO-CODE.md`; its source-anchor stage is clean.

## Core Insight

A YANG `default` is a declaration, not a mechanism. Nothing applies it to a
container that is not a peer entry until a consumer asks
`config.ApplyDefaults` for that container by name, so the value an operator
never wrote is simply absent from the tree the plugin is handed. A reader that
indexes the map instead gets 0, which for both of these leaves is a value an
operator could legitimately have chosen. That is why `parseAttributeDefaults`
resolves the schema container rather than defaulting in Go, and why the class
is a journal row: three consumers now do this by hand
(`plan/journal/declared-default-applied-by-each-consumer.md`).

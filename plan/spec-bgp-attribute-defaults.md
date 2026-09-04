# Spec: bgp attribute defaults

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Depends | spec-fixit-filter-subject-drops-five-attributes, spec-fixit-bgp-distance-declaration (ordering only) |
| Phase | DESIGN complete. Awaiting owner approval |
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
| Config file to plugin | a config section delivered to the plugin's configure callback | No |
| Reactor to plugin | `rpc.FilterUpdateInput.Update`, the text subject | No |
| Plugin to reactor | `rpc.FilterUpdateOutput.Update`, the delta | No |
| Operator to peer | the re-announced route's MULTI_EXIT_DISC or LOCAL_PREF | No |

### Integration Points
- `absentBase` and `currentForArithmetic`, which already have the shape a
  configured value slots into.
- The plugin's existing configure callback, which already parses a config
  section.
- The `defaults` container the depended-on spec creates under `bgp`.

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
| A-1 | `absentBase` has exactly one reader, `currentForArithmetic` | Both are declared in `modify.go` and the map is package-private | A second reader would keep a path on the old constants after the config lands | `gopls references` on the symbol, recorded in the audit | unvalidated |
| A-2 | The plugin's configure callback runs before any filter update is handled, so a route never computes from an unset base | `filter_modify.go` already warns "filter-update before configure" for the definitions map, which means the ordering is not guaranteed and is already handled | A route arriving first would compute from a zero value rather than the declared default | A test that calls the update path before configure and asserts the declared defaults apply | unvalidated |
| A-3 | `bgp { defaults { } }` exists by the time this spec lands | BROKEN, 2026-09-04. The distance spec was expected to create it holding `admin-distance`, but its investigation moved administrative distance to `rib { distance { } }` and DELETED the BGP container outright, so nothing creates `bgp { defaults { } }`. Confirmed: no `container defaults` in `internal/component/bgp/yang/ze-bgp-conf.yang` | This spec creates the container itself, which is now the design rather than a duplication | done | broken |
| A-4 | Nothing outside the modifier reads a default MED or LOCAL_PREF today | The two numbers were introduced by `spec-fixit-filter-subject-drops-five-attributes` and live in one map | A second consumer would need the same values and would be a second declaration | A grep for the literals 100 and 0 near LOCAL_PREF and MED handling | unvalidated |

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
| `TestAbsentBaseComesFromConfig` | `internal/component/bgp/plugins/filter_modify/modify_test.go` | AC-10: no literal 0 or 100 remains in the map's declaration | |
| `TestConfiguredLocalPrefBaseFeedsTheArithmetic` | `internal/component/bgp/plugins/filter_modify/modify_test.go` | AC-2 | |
| `TestConfiguredMedBaseFeedsTheArithmetic` | `internal/component/bgp/plugins/filter_modify/modify_test.go` | AC-3 and AC-4 | |
| `TestConfiguredBaseIsUnusedWhenTheRouteCarriesTheAttribute` | `internal/component/bgp/plugins/filter_modify/modify_test.go` | AC-5 | |
| `TestDefaultsApplyBeforeConfigure` | `internal/component/bgp/plugins/filter_modify/modify_test.go` | AC-8 and A-2 | |
| `TestAttributeDefaultsReloadTakesEffect` | `internal/component/bgp/plugins/filter_modify/config_test.go` | AC-7 | |
| `TestAigpDefaultLeafDoesNotExist` | `internal/component/config/yang/validator_test.go` | AC-6 | |
| `TestAbsentMedStillComparesAsZeroInPhaseTwo` | `internal/component/bgp/plugins/rib/` best-path test | AC-9: the configured base does not reach the Decision Process | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `defaults attribute med` | 0 to 4294967295 | 4294967295 | N-A, 0 is valid and is the declared default | 4294967296, refused by the YANG range |
| `defaults attribute local-preference` | 0 to 4294967295 | 4294967295 | N-A, 0 is valid | 4294967296, refused by the YANG range |
| the arithmetic result after a configured base | 0 to 4294967295 | saturates at the top, floors at 0 | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `attribute-default-localpref-configured` | `test/plugin/attribute-default-localpref-configured.ci` | An operator sets the default to 80, a peer announces a route with no LOCAL_PREF, and an observing peer reads 130 | |
| `modify-increment-localpref` | `test/plugin/modify-increment-localpref.ci` | The existing test, re-run to prove the declared defaults still apply with no block written | |

### Interop Tests (Scope: config)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-attribute-default-localpref-gobgp` | `test/interop/scenarios/bgp-attribute-default-localpref-gobgp` | GoBGP | GoBGP announces a route carrying no LOCAL_PREF, ze's import chain increments by 50 from a configured base of 80, and a second GoBGP reads 130. Reverting the config plumbing makes it read 150 | |

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

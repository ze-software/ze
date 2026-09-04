# Spec: filter subject drops five attributes

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 6/7 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-09-04 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Five of the thirteen attribute names Ze advertises in its policy filter text
reach no filter plugin, on any session. `appendSingleAttr`
(`internal/component/bgp/reactor/filter_format.go`) type-switches on pointer
forms for `Origin`, `MED`, `LocalPref`, `AtomicAggregate` and `ClusterList`,
while the parsers registered in `knownAttrParsers`
(`internal/core/bgp/attribute/wire.go`) box the value forms. A Go type switch
arm that names a type no producer creates can never match, so `origin`, `med`,
`local-preference`, `atomic-aggregate` and `cluster-list` are absent from the
subject text every text-mode filter receives.

The switch has no default arm, so the miss is silent. Nothing logs, nothing
counts it, and the filter chain runs to a verdict taken on a subject that is
missing the attribute the operator asked the filter to judge.

Goal: every attribute name `attrNameToCode` advertises appears in the subject
when the UPDATE carries that attribute, the miss stops being silent, and the
`med` half is proven against a peer daemon rather than against a hand-written
string.

Symptoms already recorded. `plan/journal/silent-fall-through.md` carries two
rows. The 2026-08-15 row (spec `rfc4271-med-across-as`) found the MED case
alone, moved the configured-removal gate to the wire rather than fixing the
renderer, and concluded the renderer fix wants its own spec. The 2026-09-03 row
extends the same mismatch to all five. This spec is the one the first row asked
for.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - the page that defines the
      filter text protocol, sections "The AS Path in the Filter Text Protocol"
      and "Non-CIDR Families in the Filter Text Protocol"
  → Decision: the page documents two shapes of the subject (the merged AS path,
    the non-CIDR NLRI marker) and is SILENT on which attribute names the subject
    carries. A plugin author cannot learn from it that five never appear, so the
    page gains an attribute-name table in this work.
  → Constraint: the subject is built once per UPDATE for the whole chain, so an
    attribute name added to it is added for every filter on that chain at once.
- [ ] `docs/guide/plugins.md` - "Route Filters" and "Route Attribute Modifier"
  → Decision: the page states "Supported attributes: local-preference, med,
    aigp" for increment and decrement. That is WRONG for `med` today, because
    `extractUint32Attr` reads 0 from a subject that never names it, so
    `increment med` sets rather than increments.
  → Constraint: the page also states "the engine sends only those attributes as
    text for each UPDATE". That is WRONG: every runtime caller of
    `AppendUpdateForFilter` passes a nil declared list, and `FilterInfo`'s
    declared list is consumed by `validateModifyDelta` alone.
- [ ] `docs/architecture/bgp/egress-attribute-rules.md` - the MED chapter
  → Constraint: the page states the defect as a live fact and names
    `medRemoveHasWork` as the workaround that reads the wire instead of the
    text. Landing this fix makes that paragraph wrong, so the page edit lands in
    the same work, before the next code edit.
- [ ] `docs/architecture/core-design.md` - the Policy Filter Chain section. Five
      files this spec modifies declare it as their design page:
      `filter_format.go`, `filter_chain.go`, `filter_delta.go`,
      `policy_dryrun.go` and `filter_modify/modify.go`
  → Decision: the page describes how the chain runs and is SILENT on which
    attribute names the subject carries, so it neither states nor contradicts
    the defect. It gains a pointer to the attribute-name table rather than a
    second copy of it (`ai/rules/principles.md`, every fact declared once).
  → Constraint: its "Two Input Modes" example is the plugin ANNOUNCE format, not
    the filter subject. The two formats look alike and are read in opposite
    directions, so an implementer must not take that example as evidence about
    what the engine emits to a filter.
- [ ] `docs/architecture/api/architecture.md` - the unified BGP route filter
      pipeline, declared by `filter_ordered.go`
  → Decision: silent on the subject's attribute names for the same reason, and
    it takes the same pointer rather than a copy.
  → Constraint: its attribute-storage table is about what a replay queue holds,
    not about what the subject renders. The two are different sets, and this
    spec changes only the second.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - ORIGIN, MULTI_EXIT_DISC, LOCAL_PREF and
      ATOMIC_AGGREGATE are the attributes Sections 5.1.1, 5.1.4, 5.1.5 and 5.1.6
      define, and the filter subject is where an operator policy reads them
  → Constraint: Section 5.1.4 requires a configured mechanism that takes
    MULTI_EXIT_DISC off a route before Decision Process phases 1 and 2. The
    import chain is where that mechanism runs, and the gate that decides whether
    it has work is part of this spec's blast radius.
- [ ] `rfc/short/rfc4456.md` - CLUSTER_LIST is Section 8, and a route reflector
      policy that reasons about the cluster the route already traversed reads it
      from the subject
  → Constraint: the subject renders CLUSTER_LIST as space-separated dotted
    decimal identifiers with no brackets, which is the legacy shape
    `parseFilterAttrs` already reads back.

**Key insights:**
- The switch arm type must equal the parser's return type. Eight of the thirteen
  arms already obey that rule; the five broken arms are the exception.
- The subject is built once per chain with a nil declared list, so the fix
  reaches every filter through one arm of `AppendAttrsForFilter`.
- The delta merge starts from the full subject, so the five new names appear on
  BOTH sides of the before and after comparison and no unmodified attribute
  diffs as changed.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/filter_format.go` - builds the subject.
      `AppendUpdateForFilter` calls `AppendAttrsForFilter`, which routes both a
      declared list and an empty list through `attrForFilter` and then through
      `appendSingleAttr`. `appendSingleAttr` is a thirteen-arm type switch with
      no default. Five arms name a pointer type.
- [ ] `internal/core/bgp/attribute/wire.go` - `knownAttrParsers` registers the
      parser for each code. `ParseOrigin`, `ParseMED`, `ParseLocalPref`,
      `parseAtomicAggregate` and `ParseClusterList` each return a value form.
- [ ] `internal/core/bgp/attribute/simple.go` and `origin.go` - the five types
      carry value receivers throughout: `Code`, `Flags`, `Len`, `WriteTo`,
      `WriteToWithContext`, `CheckedWriteTo`. `MED` and `LocalPref` are named
      `uint32` types, `AtomicAggregate` is an empty struct, `ClusterList` and
      `Origin` are a slice and a byte.
- [ ] `internal/core/bgp/attribute/text_append.go` - `AppendText` for those five
      also takes a value receiver, and each emits the attribute name followed by
      its value.
- [ ] `internal/component/bgp/reactor/filter_chain.go` - `parseFilterAttrs`
      reads a subject back into `filterAttrs`, `formatFilterAttrs` renders it
      out, `applyFilterDelta` merges a delta into the current subject, and
      `policyFilterFunc` builds the RPC input and validates a modify delta
      against the filter's declared list.
- [ ] `internal/component/bgp/reactor/filter_delta.go` - `textDeltaToModOps`
      diffs the before and after subjects into wire modification ops,
      `encodeAttrValue` turns a text value into wire value bytes,
      `medRemoveHasWork` decides whether a configured MED removal has work, and
      `ExtractMEDRemoveOps` records the suppression.
- [ ] `internal/component/bgp/reactor/filter_ordered.go` - the two runtime
      callers of `AppendUpdateForFilter`, both passing a nil declared list, and
      the import site that calls `medRemoveHasWork`.
- [ ] `internal/component/bgp/reactor/policy_dryrun.go` - the `ze policy test`
      caller, also passing nil, whose `TextBefore` and `TextAfter` are
      user-visible output.
- [ ] `internal/component/bgp/plugins/filter_modify/modify.go` -
      `buildDynamicDelta` computes an increment or a decrement from
      `extractUint32Attr(updateText, name)`, which reads the subject.
- [ ] `internal/component/bgp/filtertext/community.go` and `aspath.go` - the one
      reader two plugins share, cutting on a keyword and stopping at the next
      attribute name.
- [ ] `internal/component/plugin/server/server.go` - `FilterInfo` returns the
      declared attribute list and the raw flag for a named filter.

**Behavior to preserve:**
- The subject's attribute order, which `appendAllAttrs` fixes and
  `formatFilterAttrsOrder` mirrors, so a render, parse and re-render round trip
  produces the same string.
- The text shape of every attribute value, which `AppendText` owns and
  `encodeAttrValue` reads back.
- The `nlri` block staying last, which `filter_prefix` and `filter_irr` both
  depend on when they cut the subject on the `nlri` keyword.
- The invalid-aggregator drop: an `Aggregator` whose address does not parse
  writes nothing and leaves no dangling keyword.
- Every functional expectation in `test/plugin/` that asserts a filter verdict
  or a wire payload.

**Behavior to change:**
- `origin`, `med`, `local-preference`, `atomic-aggregate` and `cluster-list`
  appear in the subject whenever the UPDATE carries the attribute.
- A type the switch does not name stops being dropped in silence.
- `increment med` and `decrement med` compute from the route's metric rather
  than from zero.
- `medRemoveHasWork` stops reading the wire for a fact the subject now carries.

## Data Flow (MANDATORY)

### Entry Point
- A BGP UPDATE arrives on a peer session as wire bytes, or an operator runs
  `ze policy test` with a hex UPDATE body.
- Format at entry: the UPDATE body's attribute section, held lazily by
  `attribute.AttributesWire`.

### Transformation Path
1. `wireu.WireUpdate.Attrs` exposes the attribute section without copying it.
2. `attrForFilter` calls `AttributesWire.Get(code)`, which dispatches through
   `knownAttrParsers` to the parser for that code and returns the boxed result.
3. `appendSingleAttr` type-switches on that boxed value and calls `AppendText`
   for the matching arm. Five arms never match, so nothing is appended and the
   attribute is absent from the subject.
4. `AppendUpdateForFilter` appends the NLRI block after the attributes.
5. `PolicyFilterChain` hands the subject to each filter over
   `rpc.FilterUpdateInput.Update`, piping each filter's output into the next.
6. A modify response is merged by `applyFilterDelta`, which parses the current
   subject and the delta, merges, and re-renders.
7. `textDeltaToModOps` diffs the before and after subjects into wire ops, which
   `buildModifiedPayload` applies to the UPDATE body.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor to plugin | `rpc.FilterUpdateInput.Update`, a space-separated text subject | No |
| Plugin to reactor | `rpc.FilterUpdateOutput.Update`, a delta in the same text format | No |
| Reactor to wire | `filterapi.ModAccumulator` ops applied by `buildModifiedPayload` | No |
| Reactor to operator | `plugin.PolicyDryRunResult.TextBefore` and `TextAfter` in `ze policy test` output and its JSON | No |

### Integration Points
- `attribute.AppendText` on the five value types, already written and already
  covered in `internal/core/bgp/attribute/`.
- `encodeAttrValue` in `filter_delta.go`, which already carries an arm for each
  of the five names.
- `filterAttrNameToID` and `formatFilterAttrsOrder` in `filter_chain.go`, which
  already carry all five names in the emit order.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every runtime caller of `AppendUpdateForFilter` passes a nil declared list, so the render-everything arm always builds the chain subject | `filter_ordered.go` at the import and export sites, and `policy_dryrun.go`, are the only three non-test callers and each passes nil | The declared arm could narrow a subject and hide `med` again, which is the case the wire arm of `medRemoveHasWork` was written for, so dropping that arm would reintroduce the 2026-08-15 defect | A test that asserts the three call sites pass nil, plus a grep recorded in the audit | confirmed -- `AppendAttrsForFilter` (`filter_format.go`) takes the render-everything arm when the declared list is empty, and all three non-test callers pass nil: `tracePolicyFilterChain` (`policy_dryrun.go`) and both call sites in `filter_ordered.go`. `TestFilterSubjectCallersPassNoDeclaredList` (`filter_format_attrs_test.go`) is what fails if that stops being true |
| A-2 | `applyFilterDelta` merges a delta INTO the full current subject, so the modified attribute map is always a superset of the original one | `applyFilterDelta` parses `current`, parses `delta`, calls `filterAttrs.merge`, and re-renders | The removal arm of `textDeltaToModOps` would become reachable for the five names and could emit a zero-length set that strips an attribute nobody asked to strip | A unit test that runs an accept-only and a modify chain over a subject carrying all five and asserts no removal op | confirmed -- `applyFilterDelta` (`filter_chain.go`) parses the full current subject, merges the delta into it, and re-renders. `policyFilterFunc` seeds `current` with the whole subject and rewrites it only through that function, so the modified map is a superset by construction. `TestTextDeltaRecordsNothingForAnUnchangedSubject` (`filter_delta_test.go`) asserts the zero removal ops |
| A-3 | The render, parse and re-render round trip is byte-stable for the five names, because `appendAllAttrs` and `formatFilterAttrsOrder` list them in the same positions, and `parseFilterAttrs` already treats `atomic-aggregate` as valueless and `cluster-list` as multi-token | `appendAllAttrs`, `formatFilterAttrsOrder`, `parseFilterAttrs` and `isPolicyAttrName` | A chain that modifies one attribute would silently reshape the other four, and the text comparison in `filter_ordered.go` would fire on a route no filter changed | A round-trip unit test over a subject carrying all thirteen names | confirmed -- `TestFilterSubjectRoundTripsThroughParseAndFormat` (`filter_chain_test.go`) renders the subject through `AppendUpdateForFilter`, `parseFilterAttrs` reads back every one of the thirteen ids plus the NLRI block, and `formatFilterAttrs` returns the same bytes. Discriminated on 2026-09-04: swapping `faMED` and `faLocalPreference` in `formatFilterAttrsOrder` turns that test and five of the six delta subtests red, and the restored file runs green |
| A-4 | No in-tree filter plugin reads the subject positionally | `filtertext.CommunityValues` and `filtertext.ASPath` cut on a keyword; `filter_prefix` and `filter_irr` cut on the `nlri` keyword; `filter_remove_private_as` reads the AS path only | A filter would change verdict on a route it judged correctly before | A functional test per affected plugin, run before and after the fix | confirmed -- the whole `./le functional plugin` suite ran on 2026-09-04 with the wider subject in place. Every `filter-*` and `med-*` test passed, `med-removal-configured` (400/727) included. No in-tree plugin changed verdict |
| A-5 | An out-of-tree plugin that looks up a keyword is unaffected, and one that anchors on the first token is not | `rpc.FilterUpdateInput.Update` is the whole contract, and nothing in it promises a first token | An external plugin silently changes verdict on upgrade, which no test in this repository can see | A stated contract note in `docs/architecture/api/process-protocol.md` naming the five new names | confirmed -- the page carries the thirteen-row attribute table and, at `:873`, the note that Ze added the five names on 2026-09-04 and that a plugin reading a position is not unaffected. The risk to an out-of-tree plugin is stated, not removed: no test in this repository can see one |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Dropping the wire arm of `medRemoveHasWork` breaks the configured MED removal when a future change narrows the subject | `test/interop/scenarios/bgp-med-remove-configured-gobgp` goes red | Keep the function's doc comment naming A-1 as the invariant it rests on, and add a unit test that fails if a call site passes a non-nil declared list |
| R-2 | `increment med` and `decrement med` change operator-visible results on an existing deployment | An operator's MED arithmetic changes without a config change | The change is the fix, not a regression. State it in the page edit and in the commit body; `docs/guide/plugins.md` already documents the intended semantics |
| R-3 | A filter that sets one of the five to the value the route already carries stops emitting a modification op, so `buildModifiedPayload` is skipped where it used to run | A functional test asserting a modify log line for a no-op set goes red | Verify against the existing `test/plugin/` set. A skipped rebuild that changes no byte is the correct outcome |
| R-4 | The subject grows by up to five pairs, so an UPDATE near the 65536-byte scratch array spills to the heap on the filter path | An allocation regression in the reactor benchmark | The spill costs one allocation for that one call, which the scratch discipline in `docs/architecture/api/process-protocol.md` already names as correct |
| R-5 | The 59 existing test functions that hand-write a subject are green about a text the product never emitted, so some assert a shape the fix changes | The reactor and `filter_modify` package suites go red | Each red is read against the producer: a test wrong about what it asserts is corrected, and a test right about the product is a defect found |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every text-mode filter's verdict on every route on every session. An import filter is a guard, so a wrong subject means routes accepted that policy exists to reject. `increment med` and `decrement med` change what they compute for every operator using them |
| How is it reverted? | A single commit revert. Nothing is persisted and no config migration is involved. A peer that already received a re-announced route with a changed MED keeps it until the next announcement |
| Who else touches this path? | `plan/journal/silent-fall-through.md` records that `rfc4271-med-across-as` moved the MED gate to the wire rather than fixing this. An RFC 6793 session is changing `attribute.MergeAS4Path` (`internal/core/bgp/attribute/as4.go`), whose second non-test caller is `asPathForFilter` (`filter_format.go`, the same file this spec edits, a different function). A change to the confederation prepend clause moves the `as-path` token in the subject, so `TestFilterSubjectNamesEveryAdvertisedAttribute` and `TestFilterSubjectRoundTripsThroughParseAndFormat` assert an AS path that session owns. No open spec claims `appendSingleAttr` |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A BGP UPDATE carrying ORIGIN, MULTI_EXIT_DISC, LOCAL_PREF, ATOMIC_AGGREGATE and CLUSTER_LIST arrives on a peer with an import filter | → | `appendSingleAttr` renders all five into the subject `PolicyFilterChain` hands the plugin | `test/plugin/filter-subject-names-every-attribute.ci` |
| An operator configures `modify { increment { med 50 } }` on an import chain and a peer announces a route carrying MED 100 | → | `extractUint32Attr` reads 100 and `buildDynamicDelta` writes `med 150` | `test/plugin/modify-increment-med-from-route-value.ci` |
| An operator runs `ze policy test` with an UPDATE body carrying all five | → | `policy_dryrun.go` renders `TextBefore` through the same builder | `TestPolicyDryRunSubjectNamesEveryAttribute` |
| A filter plugin rejects on `origin incomplete` | → | the subject names `origin`, so the match container of `bgp-filter-modify` and any keyword reader can find it | `test/plugin/filter-match-on-origin.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An `AttributesWire` carrying ORIGIN, AS_PATH, NEXT_HOP, MULTI_EXIT_DISC, LOCAL_PREF, ATOMIC_AGGREGATE, AGGREGATOR, COMMUNITIES, ORIGINATOR_ID, CLUSTER_LIST, extended communities, AIGP and large communities, rendered with an empty declared list | The subject names all thirteen attributes, in the order `appendAllAttrs` fixes, with the value shape each `AppendText` produces |
| AC-2 | The same wire, rendered with a declared list naming only `origin`, `med`, `local-preference`, `atomic-aggregate` and `cluster-list` | The subject names exactly those five, so both arms of `AppendAttrsForFilter` share one renderer |
| AC-3 | A boxed attribute whose concrete type no switch arm names reaches `appendSingleAttr` | Nothing is appended, and one rate-limited warning names the type. The chain continues on the subject it has, which is the same degrade-and-speak shape `asPathForFilter` already uses in this file |
| AC-4 | An import chain runs `bgp-filter-modify` with `increment { med 50 }` on a route carrying MULTI_EXIT_DISC 100 | The delta is `med 150`, and the wire modification sets MULTI_EXIT_DISC to 150 |
| AC-5 | The same chain with `decrement { med 30 }` on a route carrying MULTI_EXIT_DISC 100 | The delta is `med 70`, and the wire modification sets MULTI_EXIT_DISC to 70 |
| AC-6 | A chain in which every filter answers accept, over a subject carrying all five | The chain output text equals the input text, `textDeltaToModOps` records no operation, and no payload rebuild runs |
| AC-7 | A chain whose single filter sets `local-preference` to the value the route already carries | `textDeltaToModOps` records no operation for LOCAL_PREF, because the before and after values are equal |
| AC-8 | A subject carrying all thirteen names is parsed by `parseFilterAttrs` and re-rendered by `formatFilterAttrs` | The output equals the input byte for byte |
| AC-9 | An import chain carries `modify { del { med } }`, the route arrived with MULTI_EXIT_DISC, and the peer is GoBGP | The re-announced route carries no MULTI_EXIT_DISC, and `medRemoveHasWork` reaches that verdict from the subject alone, with no `AttributesWire` parameter |
| AC-10 | An import chain carries `modify { del { med } }`, the route arrived with no MULTI_EXIT_DISC, and no earlier filter set one | No suppression operation is recorded and no payload rebuild runs |
| AC-11 | A filter matches on `origin incomplete` and the route carries ORIGIN incomplete | The verdict is reached on a subject that names `origin`, and the route is rejected. The same filter accepts a route carrying ORIGIN igp |
| AC-12 | A route reflector policy filter reads `cluster-list` from the subject on a route carrying CLUSTER_LIST | The subject names `cluster-list` followed by the dotted decimal identifiers in wire order |
| AC-13 | `ze policy test` runs over an UPDATE body carrying all five | `TextBefore` names all five, and its JSON rendering carries the same string |
| AC-14 | An import chain carries `modify { decrement { med 30 } }` and the route arrived with no MULTI_EXIT_DISC | The absent-value table supplies 0, the decrement floors at 0, the delta is `med 0`, and the re-announced route carries MULTI_EXIT_DISC 0. The 0 carries its RFC 4271 Section 9.1.2.2 quote at the site |
| AC-15 | An import chain carries `modify { increment { local-preference 50 } }` and the route arrived with no LOCAL_PREF | The table supplies 100, the delta is `local-preference 150`, and the re-announced route carries LOCAL_PREF 150. This CHANGES today's behavior, which yields 50 |
| AC-16 | An import chain carries `modify { increment { aigp 50 } }` or `decrement { aigp 30 }`, and the route arrived with no AIGP | NO delta is emitted for `aigp`, no wire operation is recorded, and the re-announced route carries no AIGP attribute. Absence is not a value for this attribute and Ze MUST NOT create one |
| AC-17 | `extractUint32Attr` is asked for an attribute the subject does not name | It reports the absence to its caller distinctly from a present zero. `buildDynamicDelta` consults the absent-value table rather than computing on a returned zero. No caller infers absence from a value |
| AC-18 | The absent-value table is asked for an attribute it does not name | The arithmetic is refused for that attribute rather than defaulting to 0, so a fourth attribute added to the `increment` or `decrement` container cannot silently inherit a zero nobody chose |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes an import policy that rejects routes whose ORIGIN is incomplete | wire to `AppendUpdateForFilter` to `PolicyFilterChain` to the plugin's match container to reject | `test/plugin/filter-match-on-origin.ci` |
| 2 | Writes `increment { med 50 }` on an import chain and expects the peer's metric to grow by 50 | wire to subject to `extractUint32Attr` to `buildDynamicDelta` to `textDeltaToModOps` to `buildModifiedPayload` to the re-announced UPDATE | `test/interop/scenarios/bgp-med-increment-gobgp` |
| 3 | Runs `ze policy test` to see what a chain does to a route before configuring it | hex body to `AppendUpdateForFilter` to `tracePolicyFilterChain` to `PolicyDryRunResult` to CLI and JSON output | `TestPolicyDryRunSubjectNamesEveryAttribute` |
| 4 | Writes a route reflector policy that reasons about the clusters a route already traversed | wire to subject to the plugin reading `cluster-list` | `TestFilterSubjectNamesClusterList` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFilterSubjectNamesEveryAdvertisedAttribute` | `internal/component/bgp/reactor/filter_format_attrs_test.go` | AC-1: one wire carrying all thirteen renders all thirteen names, so a fourteenth name cannot be added without the test seeing it | |
| `TestFilterSubjectDeclaredArmNamesTheSameFive` | `internal/component/bgp/reactor/filter_format_attrs_test.go` | AC-2: the declared arm and the render-everything arm share one renderer | |
| `TestFilterSubjectNamesClusterList` | `internal/component/bgp/reactor/filter_format_attrs_test.go` | AC-12: the dotted decimal shape and the wire order | |
| `TestAppendSingleAttrWarnsOnUnnamedType` | `internal/component/bgp/reactor/filter_format_attrs_test.go` | AC-3: an attribute type no arm names produces one warning and no output | |
| `TestFilterSubjectCallersPassNoDeclaredList` | `internal/component/bgp/reactor/filter_format_attrs_test.go` | A-1: the invariant the `medRemoveHasWork` simplification rests on | |
| `TestFilterSubjectRoundTripsThroughParseAndFormat` | `internal/component/bgp/reactor/filter_chain_test.go` | AC-8: render, parse, re-render is byte-stable over all thirteen names | green |
| `TestTextDeltaRecordsNothingForAnUnchangedSubject` | `internal/component/bgp/reactor/filter_delta_test.go` | AC-6 and AC-7: an accept-only chain and a no-op set each record zero operations | green |
| `TestMEDRemoveNeedsAMetricToRemove` | `internal/component/bgp/reactor/filter_delta_test.go` | AC-9 and AC-10: the gate answers from the subject, with no wire parameter | |
| `TestIncrementMedComputesFromTheRouteValue` | `internal/component/bgp/plugins/filter_modify/modify_test.go` | AC-4: 100 plus 50 is 150, driven from a subject the product would emit | |
| `TestDecrementMedComputesFromTheRouteValue` | `internal/component/bgp/plugins/filter_modify/modify_test.go` | AC-5: 100 less 30 is 70 | |
| `TestPolicyDryRunSubjectNamesEveryAttribute` | `internal/component/bgp/reactor/policy_dryrun_test.go` | AC-13: the operator-visible before text names all five | |
| `TestDecrementMedOnAbsentAttributeMaterialisesZero` | `internal/component/bgp/plugins/filter_modify/modify_test.go` | AC-14: the RFC 4271 default of 0 is applied, floored, and written |
| `TestIncrementLocalPrefOnAbsentAttributeStartsFromTheDefault` | `internal/component/bgp/plugins/filter_modify/modify_test.go` | AC-15: 100 plus 50 is 150 |
| `TestAigpArithmeticOnAbsentAttributeCreatesNothing` | `internal/component/bgp/plugins/filter_modify/modify_test.go` | AC-16: the RFC 7311 Section 3.4.1 MUST NOT, held for both increment and decrement |
| `TestReadUint32AttrReportsAbsenceDistinctly` | `internal/component/bgp/plugins/filter_modify/modify_test.go` | AC-17: an absent attribute and a present zero are told apart at the reader |
| `TestAbsentValueTableCoversEveryArithmeticAttribute` | `internal/component/bgp/plugins/filter_modify/modify_test.go` | AC-18: a leaf in the YANG `increment` or `decrement` container with no table entry fails the test rather than defaulting to 0 |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MULTI_EXIT_DISC after increment | 0 to 4294967295 | 4294967295, which saturates | N/A | N/A |
| MULTI_EXIT_DISC after decrement | 0 to 4294967295 | 0, which floors | N/A | N/A |
| ORIGIN code rendered | 0 to 2 | 2, the incomplete token | N/A | 3 and above render as the incomplete token |
| CLUSTER_LIST identifier count | 0 to the attribute length divided by 4 | the last whole identifier | N/A | a trailing partial identifier is a parse error the wire layer refuses |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `filter-subject-names-every-attribute` | `test/plugin/filter-subject-names-every-attribute.ci` | A peer announces a route carrying all five attributes and a fixture filter plugin logs the subject it received. The test asserts each of the five names is in it | green -- `./le functional plugin`, 2026-09-04, `260/727 PASS filter-subject-names-every-attribute` |
| `modify-increment-med-from-route-value` | `test/plugin/modify-increment-med-from-route-value.ci` | An operator configures `increment { med 50 }`, a peer announces MED 100, and an observing peer reads MED 150 on the wire | |
| `filter-match-on-origin` | `test/plugin/filter-match-on-origin.ci` | An operator writes a policy that rejects ORIGIN incomplete and accepts ORIGIN igp, and each route gets the verdict the operator wrote | |
| `med-removal-configured` | `test/plugin/med-removal-configured.ci` | The existing test, re-run after `medRemoveHasWork` loses its wire parameter, proving the gate still finds its work | green -- `./le functional plugin`, 2026-09-04, `400/727 PASS med-removal-configured`, with `med-removal-before-decision`, `med-removal-export-refused`, `med-not-propagated-across-as` and `med-locally-set-reaches-peer` green beside it |
| `modify-increment-localpref` | `test/plugin/modify-increment-localpref.ci` | The existing test, which announces no LOCAL_PREF on an EBGP session and asserts `local-preference 50`. It stays green through the fix, which is why it cannot be goal validation | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-med-increment-gobgp` | `test/interop/scenarios/bgp-med-increment-gobgp` | GoBGP | GoBGP announces MED 100 to ze, ze's import chain increments by 50, and a second GoBGP reads MED 150. Reverting the renderer fix makes the second GoBGP read 50 | green -- 2026-09-04, ze image `sha256:e65cda6e10ac`, Med 150 on `10.63.0.0/24` and Med 100 on the control. RED forced on image `sha256:c181f4f23fdb`: `assertion 5: GoBGP decoded Med 50 for 10.63.0.0/24, want 150`. Restore re-run green on image `sha256:8133bdad1786` |
| `bgp-med-remove-configured-gobgp` | `test/interop/scenarios/bgp-med-remove-configured-gobgp` | GoBGP | The existing scenario, re-run after `medRemoveHasWork` loses its wire parameter. It is the only thing that saw the original defect, so it is the only thing that can prove the simplification is safe | green -- 2026-09-04, ze image `sha256:d2578086a2e1`, `passed: 1, failed: 0` |
| `bgp-policy-import-export-frr` | `test/interop/scenarios/bgp-policy-import-export-frr` | FRR | The existing scenario, re-run to prove the wider subject changes no verdict on a chain that reads communities and prefixes | green -- 2026-09-04, ze image `sha256:2d544778c064`, `passed: 1, failed: 0` |

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Every advertised attribute name appears in the subject when the UPDATE carries it | functional | `test/plugin/filter-subject-names-every-attribute.ci`, which drives the real daemon and a real filter plugin and asserts the five names in the subject the plugin logged. It goes red on the current code because all five are absent |
| A filter can reason about an attribute it was asked to judge | functional | `test/plugin/filter-match-on-origin.ci`, in which one policy gives opposite verdicts to ORIGIN igp and ORIGIN incomplete. On the current code both are accepted |
| `increment med` computes from the route's metric | interop | `INTEROP_SCENARIO=bgp-med-increment-gobgp ./le integration interop`, run 2026-09-04. PASSED against GoBGP 3.31 on ze image `sha256:e65cda6e10ac84c0095de9f24de00b9470352d5d1977fb77b3a9c20388b266a4`: GoBGP decoded Med 150 for `10.63.0.0/24` and Med 100 for the untagged control `10.63.1.0/24`. RED phase forced by reverting the five arms of `appendSingleAttr` to their pointer form and rebuilding: image `sha256:c181f4f23fdbe015bf486d302fab1b87dd75ef16354a349885da4ee8b1e1e66a` FAILED with `scenario bgp-med-increment-gobgp assertion 5: GoBGP decoded Med 50 for 10.63.0.0/24, want 150`, and that run's ze log carried `filter text: attribute dropped, no renderer names its type ... type=attribute.MED`. Restored byte-identical (sha256 `1ee918345445f9654199e4b57687472d9d74748c0562d219643fdb16280b94b5`) and re-run GREEN on image `sha256:8133bdad178623acacca7b6e1d32b989863c37b99f3761f357fc0a1ad4f1a5f3`. A unit test over the renderer is not evidence here, because a unit test over a hand-written subject is the shape that stayed green through this defect for the whole of its life |
| The configured MED removal still works after the wire arm goes | interop | `INTEROP_SCENARIO=bgp-med-remove-configured-gobgp ./le integration interop`, run 2026-09-04 on ze image `sha256:d2578086a2e1db49e15c082a86818f51367e903db79449c1db432b2c40ed31d5`: PASSED, so `medRemoveHasWork` reading the subject rather than the wire keeps the RFC 4271 Section 5.1.4 configured removal working. `bgp-policy-import-export-frr` passed on image `sha256:2d544778c064d23ca4a5cbc20af4d6f603ec49b0a798c5d39f7284140bae77d7`, so the wider subject changes no verdict on a chain reading communities and prefixes |
| The miss stops being silent | unit | `TestAppendSingleAttrWarnsOnUnnamedType`, asserting one warning naming the type |

## Files to Modify
- `internal/component/bgp/reactor/filter_format.go` - the five switch arms and
  the default arm in `appendSingleAttr`
- `internal/component/bgp/reactor/filter_delta.go` - `medRemoveHasWork` loses
  its `AttributesWire` parameter and its fail-open arm, and its doc comment
  stops stating the defect as a live fact
- `internal/component/bgp/reactor/filter_ordered.go` - the import call site of
  `medRemoveHasWork`
- `internal/component/bgp/reactor/policy_dryrun.go` - the dry-run call site of
  `medRemoveHasWork`
- `internal/component/bgp/reactor/filter_chain.go` - the comment at the med and
  med-remove chain order, which cites the defect
- `internal/component/bgp/reactor/filter_format_as4_test.go` - the
  `as4FilterNextHop` comment, which explains the NEXT_HOP choice by the defect
- `internal/component/bgp/reactor/filter_delta_test.go` - the subjects it hands
  the diff, re-read against what the product now emits
- `internal/component/bgp/reactor/policy_dryrun_test.go` - the same
- `internal/component/bgp/plugins/filter_modify/modify_test.go` - the same
- `internal/component/bgp/plugins/filter_modify/modify.go` - `extractUint32Attr`
  reports absence distinctly from a present zero, and `buildDynamicDelta` applies
  the RFC 4271 Section 9.1.2.2 default in a named branch carrying the citation.
  Behavior is unchanged; the guard stops being anonymous
- `docs/architecture/api/process-protocol.md` - a new attribute-name table for
  the filter text protocol, and the note on what an out-of-tree plugin sees
  change
- `docs/guide/plugins.md` - the increment and decrement semantics for `med`, and
  the sentence claiming the engine sends only the declared attributes
- `docs/architecture/bgp/egress-attribute-rules.md` - the paragraph that states
  the defect as a live fact and names the wire gate as the answer
- `plan/journal/silent-fall-through.md` - the Fix cell of both rows, which cite
  `medPresentOnWire`. DECIDED: correct them. That symbol exists in no Go file in
  the tree; the gate the `rfc4271-med-across-as` spec actually left behind is
  `medRemoveHasWork` (`internal/component/bgp/reactor/filter_delta.go`)

## Files to Create
- `internal/component/bgp/reactor/filter_format_attrs_test.go` - the
  attribute-coverage tests, held over the whole advertised set rather than over
  one attribute
- `test/plugin/filter-subject-names-every-attribute.ci` - the functional proof
  that the subject reaches a real plugin with the five names in it
- `test/plugin/modify-increment-med-from-route-value.ci` - the functional proof
  that the increment reads the route's metric
- `test/plugin/filter-match-on-origin.ci` - the functional proof that a policy
  can decide on ORIGIN
- `test/interop/scenarios/bgp-med-increment-gobgp/` - the interop proof, with
  its `ze.conf`, `gobgp.toml`, `inject.msg` and `inject-args`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config leaf is added. `increment` and `decrement` already exist in `internal/component/bgp/plugins/filter_modify/yang/ze-filter-modify.yang` |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | No leaf added |
| CLI commands/flags | N-A | No command added. `ze policy test` output changes, which row 3 of the documentation checklist covers |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | No leaf added |
| Functional test for new RPC/API | Yes | `test/plugin/filter-subject-names-every-attribute.ci`, `test/plugin/modify-increment-med-from-route-value.ci`, `test/plugin/filter-match-on-origin.ci` |
| Pipe completeness | N-A | `ze policy test` already routes through the pipe machinery and this work adds no output field |
| Env var registration | N-A | No env var added. The warning in AC-3 goes through `fwdLogger`, whose `ze.log.bgp.reactor.forward` knob already exists |
| Doctor check for runtime dependencies | N-A | No file path, socket, port, module, binary or certificate is added |
| Prometheus counters/metrics | No | The unnamed-type warning is a Ze defect signal rather than an operational rate, and `fwdLogger` already damps it |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute code is added. The five attributes already exist and already parse |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The five names were advertised already. This makes the advertisement true rather than adding a feature |
| 2 | Config syntax changed? | No | No syntax change |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, for the `ze policy test` before and after text |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/process-protocol.md`, the filter text protocol section |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, the Route Filters and Route Attribute Modifier sections |
| 6 | Has a user guide page? | Yes | `docs/guide/redistribution.md` carries the filter chain configuration and is checked for the same two claims |
| 7 | Wire format changed? | No | The wire is unchanged. The text subject is not a wire format |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` and `ai/rules/plugins.md`, because `FilterUpdateInput.Update` is an external contract |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4271.md` for the Section 5.1.4 removal gate, now proven from the subject, and the generated `docs/features/rfc-status.md` row that follows it |
| 10 | Test infrastructure changed? | No | The functional and interop runners are unchanged |
| 11 | Affects daemon comparison? | No | No feature is gained or lost against another daemon |
| 12 | Internal architecture changed? | Yes | `docs/architecture/bgp/egress-attribute-rules.md`, whose MED chapter states the defect as a live fact |
| 13 | Route metadata keys added/changed? | No | No metadata key |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/spec-fixit-filter-subject-drops-five-attributes.md` and answer every page it names |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/plugins.md` shows `increment { local-preference 50 }` and `decrement { med 30 }`, and each is checked against what the code now computes |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - prove the subject is reachable and
   currently wrong
   - Tests: `TestFilterSubjectNamesEveryAdvertisedAttribute`,
     `test/plugin/filter-subject-names-every-attribute.ci`
   - Files: `internal/component/bgp/reactor/filter_format_attrs_test.go`,
     `test/plugin/filter-subject-names-every-attribute.ci`, and the fixture
     plugin that functional test drives
   - Verify: both are RED on the current tree, and the red names all five
     missing attributes rather than one
2. **Phase: the renderer** - make each switch arm type equal the parser's return
   type, and give the switch a default arm that speaks
   - Tests: `TestFilterSubjectNamesEveryAdvertisedAttribute`,
     `TestFilterSubjectDeclaredArmNamesTheSameFive`,
     `TestFilterSubjectNamesClusterList`,
     `TestAppendSingleAttrWarnsOnUnnamedType`
   - Files: `internal/component/bgp/reactor/filter_format.go`
   - Verify: phase 1's tests go green, and the whole reactor package is re-read
     for what the wider subject changed
3. **Phase: the round trip** - prove parse and re-render are stable, and that an
   unmodified attribute records no operation
   - Tests: `TestFilterSubjectRoundTripsThroughParseAndFormat`,
     `TestTextDeltaRecordsNothingForAnUnchangedSubject`
   - Files: `internal/component/bgp/reactor/filter_chain_test.go`,
     `internal/component/bgp/reactor/filter_delta_test.go`
   - Verify: no new modification op appears for a chain that changed nothing
4. **Phase: the MED gate** - the wire arm goes now that the subject carries the
   fact, and the scenario that measured the original defect is re-run
   - Tests: `TestMEDRemoveNeedsAMetricToRemove`,
     `TestFilterSubjectCallersPassNoDeclaredList`,
     `test/plugin/med-removal-configured.ci`,
     `test/interop/scenarios/bgp-med-remove-configured-gobgp`
   - Files: `internal/component/bgp/reactor/filter_delta.go`,
     `filter_ordered.go`, `policy_dryrun.go`, `filter_chain.go`
   - Verify: the interop scenario is green, and reverting the renderer fix turns
     it red again
5. **Phase: the modifier** - prove the increment and the decrement compute from
   the route's metric, on the wire, against GoBGP
   - Tests: `TestIncrementMedComputesFromTheRouteValue`,
     `TestDecrementMedComputesFromTheRouteValue`,
     `test/plugin/modify-increment-med-from-route-value.ci`,
     `test/interop/scenarios/bgp-med-increment-gobgp`
   - Files: `internal/component/bgp/plugins/filter_modify/modify_test.go`, the
     new functional test, the new scenario directory
   - Verify: the scenario is red with the renderer fix reverted and the image
     rebuilt, recorded per `ai/rules/interop-and-goal-validation.md`
6. **Phase: the pages** - repair every page the change made wrong, and the
   journal rows
   - Tests: none. `./le spec citation anchors` names the pages
   - Files: `docs/architecture/api/process-protocol.md`,
     `docs/guide/plugins.md`, `docs/architecture/bgp/egress-attribute-rules.md`,
     `docs/guide/command-reference.md`, `docs/guide/redistribution.md`,
     `rfc/short/rfc4271.md`, `plan/journal/silent-fall-through.md`
   - Verify: no page still states the defect as a live fact, and the increment
     and decrement claims match what the code computes
7. **Phase: the existing suite** - read every red against the producer
   - Tests: the package suites for `reactor`, `filter_modify`, `filtertext` and
     the `test/plugin` set
   - Files: whichever tests are wrong about what they assert
   - Verify: each change is justified by what the product emits, and a test that
     was right about the product is treated as a defect found rather than
     corrected

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol, and AC-3's warning has a caller that can reach it |
| Feature completeness | Story 1 through 4 each have a test that goes red with the renderer fix reverted |
| Correctness | The switch arm type equals the parser's return type for all thirteen arms, checked one by one against `knownAttrParsers` |
| Correctness | The `medRemoveHasWork` simplification is safe only under A-1, and A-1 has its own test |
| Naming | The subject's attribute names match `attrNameToCode` exactly, and no synonym is introduced |
| Data flow | The five names appear on both sides of `textDeltaToModOps`, so no unmodified attribute diffs as changed |
| Rule: `ai/rules/principles.md` | One fact declared once: after the fix, the route's metric is read from the subject alone and not from the wire as well |
| Rule: `ai/rules/protocol.md` | The external contract change is stated on the page an out-of-tree plugin author reads |
| Rule: `ai/rules/interop-and-goal-validation.md` | Every new scenario has a recorded red phase produced by reverting the fix and rebuilding the artifact |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No pointer arm in `appendSingleAttr` names a type `knownAttrParsers` does not build | Read the thirteen arms against the parser table and record the pairing in the audit |
| The switch has a default arm | `gopls symbols internal/component/bgp/reactor/filter_format.go` and a read of the function |
| `medRemoveHasWork` takes one parameter | `gopls references` on the symbol, showing both call sites updated |
| The new interop scenario exists and is named, with no numeric prefix | `ls test/interop/scenarios/bgp-med-increment-gobgp` |
| No page still states the defect as a live fact | Grep `docs/` for the phrase that names the missing token, and read the three pages named |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The five values come from parsers that already validate length and shape, so the subject cannot carry an unvalidated value. `encodeAttrValue` re-validates on the way back to the wire |
| Guard that could fail open | An import filter is a guard. A wider subject must not make any existing filter more permissive, which is what story 1 and the FRR policy scenario check |
| Guard that could fail open | `medRemoveHasWork` currently answers TRUE on an unreadable attribute section, which is the safe direction. Losing that arm loses the fail-safe as well as the redundancy, so A-1 carries the whole weight and needs its test |
| Resource exhaustion | The subject grows, so an UPDATE near the scratch array bound spills once to the heap. R-4 covers it |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood, back to RESEARCH |
| Lint failure | Fix inline. If architectural, back to DESIGN |
| Functional test fails | Check the AC: wrong AC to DESIGN, correct AC to IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| An existing test goes red and is right about the product | It is a defect found. Root cause it under `ai/rules/completion.md`, do not correct the test |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The switch has no default arm, which is what let a five-arm mismatch survive
  from the day it was written. The five-character fix removes today's instance;
  the default arm removes the class.
- `asPathForFilter`, in the same file, already documents the opposite habit: it
  logs each of its guard misses and states in its comment that silence would put
  the wrong subject back on the branch nobody looks at. The same reasoning
  applies to `appendSingleAttr` and had not been carried across.
- The tests that hand-write a subject are not wrong to exist. They are wrong to
  be the only thing that reads it, and this is the second time in this
  repository that a subject nobody produced stayed green for months.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Change the five switch arms to the value forms | Change `Origin`, `MED`, `LocalPref`, `AtomicAggregate` and `ClusterList` to pointer receivers so the pointer arms match | The switch is the odd one out, not the receivers. Eight arms already name the parser's return type: four name pointer types whose parsers return pointers, four name value types whose parsers return values. The five broken arms are the only ones that disagree with their parser. Moving the types to pointers would change five parser signatures, every value receiver on each type including the encode-path `WriteTo`, and every construction site, would put a 4-byte `uint32` behind a pointer on a path that allocates nothing today, and would leave the four value-typed arms as the new exception. Five characters fewer leaves one rule with no exception |
| Add a default arm that logs through `fwdLogger` | Leave the switch exhaustive by inspection, or add a compile-time exhaustiveness check | A compile-time check cannot exist here: the switch is over an interface, and any package can implement `attribute.Attribute`. `fwdLogger` is the same rate-limited logger `asPathForFilter` uses for the same class of miss in the same file, so the file gains no new mechanism |
| Drop the `AttributesWire` parameter from `medRemoveHasWork` | Keep the wire arm as a second reading | Two declarations of one fact is what `ai/rules/principles.md` forbids, and a reader cannot tell which side is authoritative once they disagree. The wire arm exists only because the subject was missing `med`, and its own doc comment says so. It rests on A-1, which gets its own test |
| Name the interop scenario `bgp-med-increment-gobgp` | Reuse `bgp-med-across-as-gobgp` by adding an increment to it | A scenario proves one thing, and `ai/rules/interop-and-goal-validation.md` requires a name rather than a number so the scenario's identity survives. Folding a second assertion into an existing scenario makes a red ambiguous |
| One declared absent-value table, with a different answer per attribute and its authority beside it (owner decision, 2026-09-04) | BIRD's semantics, where arithmetic on an absent attribute is a filter runtime error; or an asymmetric rule where increment materializes and decrement skips | RFC 4271 Section 9.1.2.2 (c) supplies the substituted value itself: "If route n has no MULTI_EXIT_DISC attribute, the function returns the lowest possible MULTI_EXIT_DISC value (i.e., 0)." FRR reaches the same number deliberately, guarding on the presence flag and defaulting to 0 (`bgpd/bgp_routemap.c:route_set_metric`), then materializing the attribute through `bgp_attr_set_med` (`bgpd/bgp_attr.h`), which sets `ATTR_FLAG_BIT(BGP_ATTR_MULTI_EXIT_DISC)` alongside the value. BIRD instead yields `T_VOID` for an unset attribute (`filter/f-inst.c:FI_EA_GET` through `filter/data.h:val_empty`) and `FI_SUBTRACT`'s `ARG(1,T_INT)` turns that into a `runtime()` error returning `F_ERROR`, which drops the route. BIRD's rule is symmetric, so adopting it would also break `increment local-preference 50` on a route with no LOCAL_PREF, which Ze ships as a feature and `test/plugin/modify-increment-localpref.ci` asserts, and it would need a `defined()` equivalent Ze's filter text does not have. Ze keeps FRR's answer FOR MED. The three attributes the `increment` and `decrement` containers accept do NOT share one answer, so a single default would be wrong for two of them. MULTI_EXIT_DISC: 0, and the RFC names the number. LOCAL_PREF: RFC 4271 Section 9.1.1 declines to name one ("The exact nature of this policy information, and the computation involved, is a local matter"), FRR uses `BGP_DEFAULT_LOCAL_PREF` 100 (`bgpd/bgpd.h:2330`, seeded in `bgpd/bgp_routemap.c:route_set_local_pref`) and BIRD `default_local_pref` 100 (`proto/bgp/config.Y:78`), so Ze takes 100 and 100 plus 50 is 150. AIGP: there is NO default and absence is not a value. RFC 7311 Section 4.1 eliminates a route with no AIGP TLV from consideration rather than scoring it, so a substituted 0 would make it BEST where the RFC makes it lose, and Section 3.4.1 says "A BGP speaker MUST NOT add the AIGP attribute to any route whose path leads outside the AIGP administrative domain to which the BGP speaker belongs" with AIGP_ORIGINATE defaulting to disabled. So AIGP arithmetic on an absent attribute emits nothing. The table is the one declaration of these three facts, each row carrying its citation |
| Prove the fix with a functional and an interop test rather than a renderer unit test | A unit test over `AppendAttrsForFilter` | A unit test over a hand-written subject is precisely the shape that stayed green through this defect for its whole life. The unit tests are still written, for the round trip and the boundaries, but they are not the goal validation |

## Known Limitations

- The declared-attribute arm of `AppendAttrsForFilter` has no non-test caller
  passing a non-nil list, so the claim in `docs/guide/plugins.md` that the
  engine sends only the declared attributes is repaired on the page rather than
  in code. Wiring the narrowing is separate work, and it would reopen the
  question the wire arm of `medRemoveHasWork` answered.
- `decrement med` on a route carrying no MULTI_EXIT_DISC writes `med 0`, which
  puts an attribute on a route that did not have one. That behavior is KEPT
  (owner decision, 2026-09-04), and this spec brings it in scope to pin it rather
  than to change it. See the Key Design Decisions row "Absent attribute reads as
  zero" for the evidence.
- `extractUint32Attr` cuts the subject on `"<name> "` as a substring rather than
  as a token. The wider subject this fix produces gives that cut more text to
  match against. No advertised attribute name ends in another one, so no
  collision exists today; the fragility is recorded rather than fixed.
- The subject still flattens AS_PATH segment types, which is stated in
  `(*ASPath).AppendText` and unchanged here.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

The requirement this work touches is the configured MULTI_EXIT_DISC removal of
RFC 4271 Section 5.1.4, which `ExtractMEDRemoveOps` already quotes. The change
is to where its gate reads the metric's presence, so that comment and the one on
`medRemoveHasWork` are both re-stated against the subject.

## Checklist

### Pre-Spec Verification
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
- [ ] AC-1..AC-13 all demonstrated
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
- [ ] Functional tests under `test/plugin/` for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** the spec file leaves `plan/` and nothing else, per the two-commit closure rule in `ai/rules/planning.md` (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- `appendSingleAttr` (`internal/component/bgp/reactor/filter_format.go`): the
  `Origin`, `MED`, `LocalPref`, `AtomicAggregate` and `ClusterList` arms name the
  value form their parser in `knownAttrParsers` returns, so all thirteen arms now
  obey one rule. A `default:` arm warns through `fwdLogger()` under
  `filterUnnamedAttrPhrase` and names the type, so the class cannot recur in
  silence.
- `medRemoveHasWork` (`filter_delta.go`) takes the subject alone. Its
  `*attribute.AttributesWire` parameter and its fail-open arm are gone, because
  the subject now carries the fact the wire was read for. Both call sites,
  `runIngressPolicyChain` (`filter_ordered.go`) and `computeWireChanges`
  (`policy_dryrun.go`), pass one argument.
- `internal/component/bgp/plugins/filter_modify/modify.go`: `extractUint32Attr`
  became `readUint32Attr`, returning a three-state `attrReading` that separates a
  present zero from an absence from an unreadable value.
  `currentForArithmetic` answers what the arithmetic starts from and whether it
  runs at all, reading the declared `absentBase` table (med 0, local-preference
  100, aigp deliberately absent). Each row carries its RFC citation at the site.
- `validateNoConflict` (`filter_modify/config.go`) dropped the value read beside
  `containsAttrName`. That helper anchors on a token boundary, so it already
  answered for a set of any value including 0, and the unanchored read added
  nothing it could not see.
- `Docker.Build` (`internal/le/interoplab/docker.go`): the hardcoded 10-minute
  `dockerBuildTimeout` became `dockerBuildTimeoutDefault` (90 minutes) with a
  `BUILD_TIMEOUT` override read once by `NewDocker` through `machineBuildTimeout`.
  This was a blocking defect, not a convenience: the interop image could not
  build at all on this workstation, so the goal validation this spec owes was
  unreachable without it.

### Bugs Found/Fixed
- The renderer defect itself. Covered by
  `TestFilterSubjectNamesEveryAdvertisedAttribute` and, end to end, by
  `test/plugin/filter-subject-names-every-attribute.ci` and the
  `bgp-med-increment-gobgp` interop scenario.
- `medAttrsWire` (`forward_med_test.go`) built its `AttributesWire` on
  `bgpctx.ContextID(0)`. `parseKnownAttribute` refuses a nil encoding context, so
  every attribute answered an error and the subject rendered empty: the fixture
  was green about a text the product never emitted. Row in
  `plan/journal/green-that-could-not-have-been-red.md`.
- The interop build deadline above. Row in
  `plan/journal/gate-verdict-depends-on-the-machine.md` (already committed).

### Documentation Updates
- `docs/architecture/bgp/egress-attribute-rules.md`: the MED chapter stated the
  defect as a live fact and named the wire gate as the answer. It now states that
  the gate reads the subject alone, and keeps the history in one paragraph.
- `docs/architecture/api/process-protocol.md`: the thirteen-row attribute-name
  table for the filter text protocol, and the note that Ze added the five names
  on 2026-09-04 and that a plugin reading a POSITION is not unaffected. Landed in
  `8788a29f8b`, a peer session's commit over this shared checkout.
- `docs/guide/plugins.md`: the Route Attribute Modifier section carries the
  absent-attribute table (med 0, local-preference 100, aigp nothing) with the RFC
  behind each row, and the sentence about `decrement { med 30; }` on a route that
  carried no metric. Landed in a peer session's commit.
- `docs/architecture/core-design.md`: the Policy Filter Chain section dropped
  the false claim that "the reactor parses only the union across the chain" and
  points at the attribute-name table rather than copying it.
- `docs/architecture/api/architecture.md`: the same pointer, beside its
  attribute-storage table, saying that the two sets are different.
- `docs/architecture/testing/interop.md`: `BUILD_TIMEOUT`, the measured 40m39s,
  and why the bound is generous. `.dockerignore`'s comment names
  `dockerBuildTimeoutDefault`.
- `./le verify lint run scope ./internal/component/bgp/reactor/...`: exit 0,
  0 issues on both flavors, after the review-gate fix.

### Deviations from Plan
- `plan/journal/silent-fall-through.md` was named in Files to Modify. Its two
  `medPresentOnWire` cells were corrected and the file reached git inside
  `4a1521d1b3`, a peer session's commit, before this closure ran. Nothing is
  lost; the edit is simply not in commit A.
- `docs/guide/command-reference.md` and `rfc/short/rfc4271.md` were listed as
  documentation candidates and needed no edit. The command-reference rows
  describe what `show policy test` DOES and quote no subject text, and the RFC
  summary's Section 5.1.4 coverage cell names `ExtractMEDRemoveOps`
  (`filter_delta.go`), which is still the producer. The gate moved inside that
  producer's caller; the requirement's implementation and its support level did
  not change.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Phase 5 read four interop scenarios failing on the same symptom and prepared to attribute the relay break to this diff | The host was the cause: the same tree relayed every one of those routes when the tests did not compete for it, and the machine had a full root filesystem and load average 110 | Fifteen of 143 functional failures re-run one at a time, all fifteen green on the same binaries | The interop rows were re-measured on a healthy host and each recorded with its image digest. Row in `plan/journal/gate-verdict-depends-on-the-machine.md` |
| assumption | A-1's test asserted the invariant over the reactor package directory alone | `AppendUpdateForFilter` is exported, so a caller in ANY package can narrow the subject, and `medRemoveHasWork` now holds no second reading of the wire | Review gate round 1 | The scan is repository-wide, sourced from `git ls-files`, and discriminated by a cross-package probe |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Every name `attrNameToCode` advertises appears in the subject when the UPDATE carries it | Done | `appendSingleAttr`, `internal/component/bgp/reactor/filter_format.go` | thirteen arms, each naming its parser's return type |
| The miss stops being silent | Done | the `default:` arm of `appendSingleAttr`, warning under `filterUnnamedAttrPhrase` | |
| The `med` half is proven against a peer daemon | Done | `checkMEDIncrementFromRouteValue`, `internal/le/interoplab/bgp/check_special.go` | GoBGP decoded Med 150; the reverted build decoded 50 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestFilterSubjectNamesEveryAdvertisedAttribute` | one wire carrying all thirteen |
| AC-2 | Done | `TestFilterSubjectDeclaredArmNamesTheSameFive` | both arms share one renderer |
| AC-3 | Done | `TestAppendSingleAttrWarnsOnUnnamedType` | nothing appended, one warning, separator intact |
| AC-4 | Done | `TestIncrementMedComputesFromTheRouteValue`, `test/plugin/modify-increment-med-from-route-value.ci`, `bgp-med-increment-gobgp` | |
| AC-5 | Done | `TestDecrementMedComputesFromTheRouteValue`, `TestDecrementMedOnAPresentAttributeSubtracts` | |
| AC-6 | Done | `TestTextDeltaRecordsNothingForAnUnchangedSubject/every_filter_accepts` | |
| AC-7 | Done | `TestTextDeltaRecordsNothingForAnUnchangedSubject/a_filter_sets_local-preference_150` | |
| AC-8 | Done | `TestFilterSubjectRoundTripsThroughParseAndFormat` | |
| AC-9 | Done | `TestMEDRemoveNeedsAMetricToRemove`, `bgp-med-remove-configured-gobgp` | the gate takes one parameter |
| AC-10 | Done | `TestMEDRemoveNeedsAMetricToRemove` | |
| AC-11 | Done | `test/plugin/filter-match-on-origin.ci` | opposite verdicts for igp and incomplete |
| AC-12 | Done | `TestFilterSubjectNamesClusterList` | dotted decimal, wire order |
| AC-13 | Done | `TestPolicyDryRunSubjectNamesEveryAttribute` | |
| AC-14 | Done | `TestDecrementMedOnAbsentAttributeMaterialisesZero` | RFC 4271 Section 9.1.2.2 (c) quoted at `absentBase` |
| AC-15 | Done | `TestIncrementLocalPrefOnAbsentAttributeStartsFromTheDefault`, `test/plugin/modify-increment-localpref.ci` | the `.ci` now expects 150 |
| AC-16 | Done | `TestAigpArithmeticOnAbsentAttributeCreatesNothing` | RFC 7311 Sections 4.1 and 3.4.1 quoted at `absentBase` |
| AC-17 | Done | `TestReadUint32AttrReportsAbsenceDistinctly` | three readings, six cases |
| AC-18 | Done | `TestAbsentValueTableCoversEveryArithmeticAttribute` | a leaf with no row refuses rather than defaulting |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestFilterSubjectNamesEveryAdvertisedAttribute` | Done | `filter_format_attrs_test.go` | |
| `TestFilterSubjectDeclaredArmNamesTheSameFive` | Done | `filter_format_attrs_test.go` | |
| `TestFilterSubjectNamesClusterList` | Done | `filter_format_attrs_test.go` | |
| `TestAppendSingleAttrWarnsOnUnnamedType` | Done | `filter_format_attrs_test.go` | |
| `TestFilterSubjectCallersPassNoDeclaredList` | Changed | `filter_format_attrs_test.go` | scans the whole repository, not the package |
| `TestFilterSubjectRoundTripsThroughParseAndFormat` | Done | `filter_chain_test.go` | |
| `TestTextDeltaRecordsNothingForAnUnchangedSubject` | Done | `filter_delta_test.go` | |
| `TestMEDRemoveNeedsAMetricToRemove` | Done | `filter_delta_test.go` | |
| `TestIncrementMedComputesFromTheRouteValue` | Done | `modify_test.go` | |
| `TestDecrementMedComputesFromTheRouteValue` | Done | `modify_test.go` | |
| `TestPolicyDryRunSubjectNamesEveryAttribute` | Done | `policy_dryrun_test.go` | |
| `TestDecrementMedOnAbsentAttributeMaterialisesZero` | Done | `modify_test.go` | |
| `TestIncrementLocalPrefOnAbsentAttributeStartsFromTheDefault` | Done | `modify_test.go` | |
| `TestAigpArithmeticOnAbsentAttributeCreatesNothing` | Done | `modify_test.go` | |
| `TestReadUint32AttrReportsAbsenceDistinctly` | Done | `modify_test.go` | replaces `TestExtractUint32Attr`, keeping all five of its cases |
| `TestAbsentValueTableCoversEveryArithmeticAttribute` | Done | `modify_test.go` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/reactor/filter_format.go` | Done | five arms plus the default arm |
| `internal/component/bgp/reactor/filter_delta.go` | Done | `medRemoveHasWork` takes one parameter |
| `internal/component/bgp/reactor/filter_ordered.go` | Done | call site |
| `internal/component/bgp/reactor/policy_dryrun.go` | Done | call site |
| `internal/component/bgp/reactor/filter_chain.go` | Changed | the comment it was listed for did not cite the defect; nothing to repair |
| `internal/component/bgp/reactor/filter_format_as4_test.go` | Done | the `as4FilterNextHop` comment |
| `internal/component/bgp/reactor/filter_delta_test.go` | Done | |
| `internal/component/bgp/reactor/policy_dryrun_test.go` | Done | |
| `internal/component/bgp/plugins/filter_modify/modify_test.go` | Done | |
| `internal/component/bgp/plugins/filter_modify/modify.go` | Done | `readUint32Attr`, `currentForArithmetic`, `absentBase` |
| `docs/architecture/api/process-protocol.md` | Done | landed in `8788a29f8b` |
| `docs/guide/plugins.md` | Done | landed in a peer session's commit |
| `docs/architecture/bgp/egress-attribute-rules.md` | Done | in commit A |
| `plan/journal/silent-fall-through.md` | Done | landed in `4a1521d1b3` |
| `internal/component/bgp/reactor/filter_format_attrs_test.go` | Done | created |
| `test/plugin/filter-subject-names-every-attribute.ci` | Done | created |
| `test/plugin/modify-increment-med-from-route-value.ci` | Done | created |
| `test/plugin/filter-match-on-origin.ci` | Done | created |
| `test/interop/scenarios/bgp-med-increment-gobgp/` | Done | created, four files |

### Audit Summary
- **Total items:** 53
- **Done:** 51
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (`filter_chain.go`, which needed no edit; `TestFilterSubjectCallersPassNoDeclaredList`, widened by the review gate)

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none -- the spec metadata declares no deferral shard | done | `ls plan/deferrals/` holds no shard for this spec, so there is no shard to remove and no row to home |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-filter-subject-drops-five-attributes-3450cbba-f237-4466-ad72-294aa02bc1df.md`, 34 files, verdict=clean |
| `./le spec session review check` | `review_gate: OK (clean, hashes match)`, exit 0 |
| Rounds | 2. Round 1 read the whole diff and found one ISSUE. Round 2 read the fix that ISSUE produced and found nothing in its scope |
| Reviewer lenses used | wiring and functional-test coverage; guard audit and removed-behavior audit; RFC quote verification against `rfc/full/`; the `docs/contributing/ze-go-style.md` style pass over every changed Go file |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The test that carries A-1 scanned only the reactor package directory, while its VALIDATES line claims the invariant for every runtime caller. `AppendUpdateForFilter` is exported and `medRemoveHasWork` now holds no second reading of the wire, so a caller in another package could narrow the subject and take that gate's answer away with nothing red | `TestFilterSubjectCallersPassNoDeclaredList`, `internal/component/bgp/reactor/filter_format_attrs_test.go` | the scan reads every Go file `git ls-files --cached --others --exclude-standard` names, parsing only those whose bytes hold the symbol. `repositorySources` refuses an empty listing rather than passing over zero files. Discriminated: a probe function in the `filter_modify` package calling `reactor.AppendUpdateForFilter` with a non-nil declared list turned the test RED and named the probe's file and position, and the probe was removed |

Round 1 also recorded one NOTE, which does not block and was not changed:
`BUILD_TIMEOUT` is unnamespaced beside its `ZE_INTEROP_*` siblings.
`SESSION_TIMEOUT`, read by `sessionTimeout` in
`internal/le/interoplab/bgp/helper.go`, is the same shape and is the precedent
`machineBuildTimeout`'s own comment names.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/reactor/filter_format_attrs_test.go` | Yes | `ls -1`: `-rw-r--r-- 14K Sep 4 15:28` |
| `test/plugin/filter-subject-names-every-attribute.ci` | Yes | `ls -1`: `-rw-r--r-- 3.5K Sep 4 00:20` |
| `test/plugin/modify-increment-med-from-route-value.ci` | Yes | `ls -1`: `-rw-r--r-- 5.6K Sep 4 09:45` |
| `test/plugin/filter-match-on-origin.ci` | Yes | `ls -1`: `-rw-r--r-- 3.4K Sep 4 09:45` |
| `test/plugin/med-removal-configured.ci` | Yes | `ls -1`: `-rw-r--r-- 5.9K Aug 28 01:30` |
| `test/plugin/modify-increment-localpref.ci` | Yes | `ls -1`: `-rw-r--r-- 2.0K Sep 4 08:55` |
| `test/interop/scenarios/bgp-med-increment-gobgp/` | Yes | `ls -1`: `gobgp.toml` 257 bytes, `inject-args` 12, `inject.msg` 5.1K, `ze.conf` 4.4K |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2, AC-3, AC-8, AC-12 | the renderer names every advertised attribute and speaks on a miss | `go test -count=1 -run 'TestFilterSubject...' ./internal/component/bgp/reactor/`: `ok ... 0.980s`, 2026-09-04 |
| AC-6, AC-7, AC-9, AC-10, AC-13 | the delta records nothing for an unchanged subject, and the MED gate answers from the subject | same run |
| AC-4, AC-5, AC-14 to AC-18 | the arithmetic computes from the route's value, and absence is told from a present zero | `go test -count=1 ./internal/component/bgp/plugins/filter_modify/`: `ok ... 0.474s`, 2026-09-04 |
| AC-4 on the wire | GoBGP decodes 150 | `INTEROP_SCENARIO=bgp-med-increment-gobgp`, ze image `sha256:e65cda6e10ac`, PASS. The reverted image `sha256:c181f4f23fdb` FAILED with `assertion 5: GoBGP decoded Med 50 for 10.63.0.0/24, want 150` |
| AC-11 | a policy decides on ORIGIN | `test/plugin/filter-match-on-origin.ci` PASS 1/1 in 4.0s, and RED with the arms reverted: `the subject named origin "<unnamed>", want "incomplete"` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| An UPDATE carrying all five arrives on a peer with an import filter | `test/plugin/filter-subject-names-every-attribute.ci` | Yes. It configures `external filter-subject-test` running `ze-test fixture plugin/filter-subject-names-every-attribute` on the peer's `import` chain, and the fixture (`internal/test/fixture/plugin_fixture_filter_subject.go`) reads `input.Update` and asserts each of the five names |
| `increment { med 50 }` on a route carrying MED 100 | `test/plugin/modify-increment-med-from-route-value.ci` | Yes. It reads `80040400000096` off the observing peer's wire, which is MULTI_EXIT_DISC 150 |
| A filter rejects on `origin incomplete` | `test/plugin/filter-match-on-origin.ci` | Yes. One policy, two prefixes, opposite verdicts, and each verdict is re-read from `show bgp adj-rib-in` so it is proven to have reached the route rather than only the plugin |
| `ze policy test` over a body carrying all five | `TestPolicyDryRunSubjectNamesEveryAttribute` | Yes. It drives `(&reactorAPIAdapter{r}).PolicyDryRun`, which is the RPC the command calls |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestFilterSubjectCallersPassNoDeclaredList` now reads every non-test Go file in the repository and finds exactly three call sites, each passing `nil`: two in `filter_ordered.go` and one in `policy_dryrun.go`. Grepping `AppendUpdateForFilter` outside the reactor package names only test callers, in `filter_modify/modify_test.go` and `filter_path_asn/rfc6793_subject_test.go`, and both pass nil |
| A-2 | confirmed | `applyFilterDelta` (`filter_chain.go`) merges the delta INTO the full current subject; `TestTextDeltaRecordsNothingForAnUnchangedSubject` asserts zero removal ops over six chains |
| A-3 | confirmed | `TestFilterSubjectRoundTripsThroughParseAndFormat`, discriminated by swapping `faMED` and `faLocalPreference` in `formatFilterAttrsOrder` |
| A-4 | confirmed | the whole `./le functional plugin` suite ran with the wider subject in place; every `filter-*` and `med-*` test passed, `med-removal-configured` at 400/727 included |
| A-5 | confirmed | `docs/architecture/api/process-protocol.md` states the change to an out-of-tree plugin. The risk is stated rather than removed: no test in this repository can see such a plugin |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| The filter text protocol names thirteen attributes (rows 4 and 8) | `docs/architecture/api/process-protocol.md` table, read against the thirteen arms of `appendSingleAttr` | Yes |
| `increment` and `decrement` semantics for an absent attribute (rows 5 and 17) | `docs/guide/plugins.md` absent-attribute table, read against `absentBase` and `currentForArithmetic` | Yes |
| The MED chapter no longer states the defect as a live fact (row 12) | `docs/architecture/bgp/egress-attribute-rules.md`, read against `medRemoveHasWork` | Yes |
| The chain does not narrow the subject to a declared union | `docs/architecture/core-design.md` Policy Filter Chain, read against `AppendAttrsForFilter` | Yes |
| `ze policy test` output (row 3) | No edit needed. `docs/guide/command-reference.md` describes what `show policy test` does and quotes no subject text | Yes |
| RFC 4271 Section 5.1.4 support level (row 9) | No edit needed. `rfc/short/rfc4271.md` names `ExtractMEDRemoveOps` (`internal/component/bgp/reactor/filter_delta.go`) as the producer, which is unchanged. The gate moved inside its caller and the support level did not change | Yes |
| Source anchors over changed files | `./le repository check`: no stale anchor names any file of this spec. Its one anchor issue points at `internal/component/bgp/plugins/redistribute_ingress/filter.go`, a file another session deleted | Yes |

## Core Insight

A type switch over an interface has no compile-time exhaustiveness check, so an
arm naming a type no producer builds is invisible to the compiler, to the linter,
and to every unit test that hand-writes the switch's output. Five such arms
survived from the day the function was written. What found them was not a better
reading of the switch: it was a peer daemon decoding a number that could only
come from arithmetic on an attribute the subject never carried. The default arm
is the part of this fix that outlives it, because it turns the next instance of
the class from silence into a log line.

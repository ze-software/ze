# Spec: YANG reference types leafref, identityref and instance-identifier

| Field | Value |
|-------|-------|
| Status | deferred |
| Priority | P3 |
| Release reason | Full YANG coverage runs after the first release; existing accepted-config correctness and landed-fix review are split into release ownership. |
| Scope | config |
| Depends | - |
| Phase | 1 |
| Handoff | - |
| Updated | 2026-10-21 |

## Task

Full YANG coverage and expansion still run after the initial release. The historical implementation of reference types and grouped actions is preserved, not restarted. Own further unsupported YANG forms and full-RFC enrollment. Existing accepted-config validation and applicability of the two remaining goyang observations are owned by the release child. The three older type skeletons remain historical lineage, not duplicate implementation commissions.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

- AC-1: a module declaring leafref, identityref or instance-identifier loads; `ErrUncheckedType` and `uncheckedKindError` for those kinds no longer exist.
- AC-2: a leafref path that does not parse, names no node, or ends at a node that is not a leaf or leaf-list is refused at load (RFC7950-9.9.2-1).
- AC-3: a config leafref with require-instance true whose target is not config is refused at load; a leafref chain that loops is refused at load (RFC7950-9.9-1).
- AC-4: a leafref value is checked against the referred leaf's type (9.9.4), and with require-instance true it must equal a referred instance in the tree, or a default in use (RFC7950-9.9-1, 9.9.2-1, 9.9.3-1); require-instance false skips the instance check.
- AC-5: an identityref value must name an identity derived from the base, unprefixed or with the defining module's prefix or name; the base itself, an unknown name and a wrong prefix are refused (RFC7950-9.10.2-1, 9.10.3-1, 9.1-2).
- AC-6: an instance-identifier value must match the Section 14 rule with every node name prefixed, name data nodes, carry one predicate per key of each list it selects; with require-instance true the instance must exist (RFC7950-9.13-1, 9.1-2).
- AC-7: a union member of one of the three types answers by its own check, in member order.
- AC-9: `rfc/short/rfc7950.md` rows lose the gap annotations the code now meets; RFC7950-9.1-1 is re-judged by an independent agent; `./le rfc check` clean.

These criteria govern full coverage and unsupported-form expansion only. For already accepted forms, the release child owns correctness and remaining review; a broad RFC7950-9.1-1 claim must not substitute for its specific evidence.

## Implementation Ownership

| Scope | Current owner |
|---|---|
| Full YANG coverage, new unsupported forms, whole-RFC judgment/enrollment; historical AC-1..7,9 beyond the accepted release surface. | `plan/post-release/features/spec-config-yang-reference-types.md` |
| Accepted reference/action correctness, AC-8, remaining landed-fix review and applicability/repair of the two goyang journal rows. | `plan/release/fixes/spec-config-yang-accepted-input-correctness.md` |

## Work Not Done

| Scope | Current owner |
|---|---|
| Accepted reference/action correctness, AC-8, remaining landed-fix review and applicability/repair of the two goyang journal rows. | `plan/release/fixes/spec-config-yang-accepted-input-correctness.md` |

## Historical planning record

The following is the complete pre-migration planning body. Its old Task, criteria, implementation steps and bucket statements are retained as history, not a second current implementation contract. The current sections above assign every obligation; original owner decisions, constraints, evidence and pauses remain authoritative. Heading levels alone are demoted for that distinction.

Deferred: full YANG coverage runs after the initial release. Remaining: review round 10 over a639bb22e9 (the grouped-action fix), and the two goyang journal rows of 2026-10-21 (action without input refuses an augment of its input, `parser-refuses-a-form-its-producer-emits.md`; a uses augment is never applied, `silent-fall-through.md`).

Recovery after compaction: `.claude/rules/post-compaction.md`.

#### Task

Owner decision, 2026-10-18, relayed by the main thread: IMPLEMENT the three
YANG built-in types Ze refuses today, leafref (RFC 7950 Section 9.9),
identityref (9.10) and instance-identifier (9.13), so RFC7950-9.1-1 ("MUST
support all lexical representations") can be proven whole. This spec replaces
the three skeletons `plan/post-release/features/spec-config-yang-type-leafref.md`,
`plan/post-release/features/spec-config-yang-type-identityref.md` and
`plan/post-release/features/spec-config-yang-type-instance-identifier.md`, which are removed in the
same commit; their Task tables are carried into the AC list below.

Context: commit fd9d7ed55c made the loader refuse a module declaring one of the
three types (`ErrUncheckedType`) and the validator refuse their values
(`uncheckedKindError`). That refusal is deleted first (`ai/rules/no-layering.md`)
and replaced by the checks below.

#### Required Reading

##### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` -- the loader checks table and the value check table
  -> Constraint: every loader refusal and every value check has a row there.
- [ ] `docs/plugin-development/schema.md` -- "References (no leafref)" section, made false by this spec
- [ ] `docs/architecture/hub-architecture.md`, `docs/architecture/hub-api-commands.md`, `docs/architecture/system-architecture.md` -- each says no Ze module can declare a leafref

##### RFC Summaries
- [ ] `rfc/full/rfc7950.txt` Sections 9.1, 9.9 to 9.9.4, 9.10 to 9.10.3, 9.13 to 9.13.2, 14 (path-arg, instance-identifier)
- [ ] `rfc/short/rfc7950.md` rows RFC7950-9.1-1, 9.1-2, 9.9-1, 9.9.2-1, 9.9.3-1, 9.10.2-1, 9.10.3-1, 9.13-1

#### Current Behavior (MANDATORY)

- [ ] `internal/component/config/yang/loader_unchecked.go` -> `moduleUncheckedTypeErrors` refuses the three types at `Loader.Resolve` through `checkStructure`.
- [ ] `internal/component/config/yang/validator.go` -> `validateYangType` sends `Yleafref`, `Yidentityref`, `YinstanceIdentifier` to `uncheckedKindError`.
- [ ] `internal/component/config/yang/loader_grammar.go` -> `checkPathArg` / `pathArg` checks a leafref path-arg's syntax and keeps nothing of it.
- [ ] `internal/component/config/validate_sections.go` -> `ValidateCustomSections` walks each section alone, so no check sees two sections at once.
- [ ] goyang resolves an identityref base (`YangType.IdentityBase`, whose `Values` holds every identity derived from it, transitively) and refuses an identityref with no base; it keeps the leafref path as text (`YangType.Path`) and `require-instance false` as `OptionalInstance`.

#### Data Flow (MANDATORY - see `ai/rules/architecture.md`)

##### Entry Point
A module reaches `Loader.Resolve`; a config value reaches `Validator.ValidateTree` / `ValidateTreeAllModules` (per section) and the new whole-tree `Validator.ValidateReferences`, both called by `config.ValidateCustomSections`, which `ze config validate`, the daemon's load and the editor's commit share.

##### Transformation Path
1. Load: `Loader.Resolve` -> `checkReferences` over every resolved entry: a leafref path parses (`parsePathArg`), resolves to a leaf or leaf-list schema node, a config leafref with require-instance points at config, and no leafref chain loops.
2. Value (lexical): `validateYangType(path, leaf, type, value)`: identityref -> a derived identity of the base; instance-identifier -> the Section 14 syntax, prefixed names, each step a data node, key predicates complete; leafref -> the value checked against the referred leaf's type.
3. Value (instance): `ValidateReferences(root)` walks the whole data tree; for each leafref or instance-identifier value with require-instance true, the referred instance exists in the tree, or is a leaf whose default is in use.

##### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| config -> config/yang | `ValidateCustomSections` calls `Validator.ValidateReferences(tree.ToMap())` | [ ] |

##### Integration Points
`validateYangType` switch; `Loader.Resolve`; `ValidateCustomSections`.

##### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No new import between components | yes | all code in `internal/component/config` |
| One parser for path-arg | yes | `checkPathArg` becomes a wrapper of `parsePathArg` |

#### Risks & Assumptions

##### Assumptions
- Config text carries no XML namespace. An identityref value may name the identity unprefixed when exactly one derived identity has that name, or with the defining module's prefix or name; an instance-identifier value MUST be prefixed (9.13.2) with the module prefix or name.
- Values compare by their string as the tree stores it, except integer kinds, which compare by number.

##### Risks
- The traffic-control `default-class` leaf becomes a leafref (the shape of the RFC's own 9.9.6 example). A config naming an absent class, accepted before and silently ignored by the netlink backend, is now refused at validate and commit.
- goyang reads one `base` per identityref; a second base is refused by goyang at parse. Recorded on RFC7950-9.10.2-1 as partial.

#### Blast Radius
| Question | Answer |
|----------|--------|
| Who loads modules | every Ze binary, at schema build |
| Who validates values | `ze config validate`, daemon load, editor commit, web commit |
| Existing modules using the types | none before this spec; traffic-control `default-class` after |

#### Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point -> Feature Code -> Test |
|-------------------------------------|
| `ze config validate` -> `ValidateCustomSections` -> `ValidateReferences` -> `test/parse/yang-leafref-default-class.ci` |

#### Acceptance Criteria
- AC-1: a module declaring leafref, identityref or instance-identifier loads; `ErrUncheckedType` and `uncheckedKindError` for those kinds no longer exist.
- AC-2: a leafref path that does not parse, names no node, or ends at a node that is not a leaf or leaf-list is refused at load (RFC7950-9.9.2-1).
- AC-3: a config leafref with require-instance true whose target is not config is refused at load; a leafref chain that loops is refused at load (RFC7950-9.9-1).
- AC-4: a leafref value is checked against the referred leaf's type (9.9.4), and with require-instance true it must equal a referred instance in the tree, or a default in use (RFC7950-9.9-1, 9.9.2-1, 9.9.3-1); require-instance false skips the instance check.
- AC-5: an identityref value must name an identity derived from the base, unprefixed or with the defining module's prefix or name; the base itself, an unknown name and a wrong prefix are refused (RFC7950-9.10.2-1, 9.10.3-1, 9.1-2).
- AC-6: an instance-identifier value must match the Section 14 rule with every node name prefixed, name data nodes, carry one predicate per key of each list it selects; with require-instance true the instance must exist (RFC7950-9.13-1, 9.1-2).
- AC-7: a union member of one of the three types answers by its own check, in member order.
- AC-8: traffic-control `default-class` is a leafref to `../class/name`; `ze config validate` refuses a config naming an absent class with the path and the fix, and accepts one naming a present class (functional test).
- AC-9: `rfc/short/rfc7950.md` rows lose the gap annotations the code now meets; RFC7950-9.1-1 is re-judged by an independent agent; `./le rfc check` clean.

#### End-to-End User Stories
- An operator writes `default-class bulk;` under a qdisc with no class `bulk`; `ze config validate` names the leaf, the missing class, and the classes that exist.

#### 🧪 TDD Test Plan

##### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestRFC7950LeafrefPath* | `internal/component/config/yang/rfc7950_reference_test.go` | AC-2, AC-3 | [ ] |
| TestRFC7950LeafrefValue*, TestRFC7950LeafrefInstance* | same | AC-4 | [ ] |
| TestRFC7950Identityref* | same | AC-5 | [ ] |
| TestRFC7950InstanceIdentifier* | same | AC-6 | [ ] |
| TestRFC7950ReferenceUnionMember | same | AC-7 | [ ] |

##### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| instance-identifier pos | 1..count | count | 0 | count+1 |

##### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| yang-leafref-default-class | `test/parse/yang-leafref-default-class.ci` | AC-8 | [ ] |

##### Interop Tests (Scope: protocol)
Not applicable: config-only feature with no protocol peer.

#### Files to Modify
- `internal/component/config/yang/loader_grammar.go`, `loader_structure.go`, `loader.go`, `validator.go`, `validator_lexical.go`, `validator_lexical_test.go`
- `internal/component/config/validate_sections.go`, `yang_schema.go`, `yang/module_default.go`
- `internal/component/traffic/yang/ze-traffic-control-conf.yang`
- docs listed under Required Reading; `rfc/short/rfc7950.md`

#### Files to Create
- `internal/component/config/yang/reference_path.go`, `validator_reference.go`, `loader_reference.go`, `rfc7950_reference_test.go`
- `test/parse/yang-leafref-default-class.ci`

##### Documentation Update Checklist (BLOCKING)
- [ ] `docs/architecture/config/yang-config-design.md` loader and value tables
- [ ] `docs/plugin-development/schema.md` references section
- [ ] hub-architecture, hub-api-commands, system-architecture leafref sentences

#### Implementation Phases
1. Delete the refusal; path-arg parser records its steps; loader reference checks; value checks; whole-tree instance pass; traffic leaf; unit and functional tests; docs; RFC summary and discrimination records.
2. Independent review (`/ze-review`, an agent that did not write the code) and independent re-judgement of RFC7950-9.1-1, then `/ze-close`.

#### Implementation Steps
Phase 1 in the order listed; tests first for each check.

##### Phase 1 result (2026-10-18)
- Done: refusal deleted (`loader_unchecked.go` removed by the commit, `uncheckedKindError` kept for `Ynone` only); `parsePathArg` records steps; `checkReferences` at `Loader.Resolve`; `validateLeafref`, `validateIdentityref`, `validateInstanceIdentifier`; `Validator.ValidateReferences` wired through `referenceFailures` in `ValidateCustomSections`; traffic `default-class` is a leafref; 17 tagged unit tests in `rfc7950_reference_test.go`; `test/parse/yang-leafref-default-class.ci` (red observed with the leaf reverted to `type string`); five doc pages; `rfc/short/rfc7950.md` annotations.
-> Decision: `ValidateType(path, type, value)` keeps its signature (many RFC-tagged tests call `validateYangType`); the leaf-aware path is `ValidateLeafType` / `validateLeafValue`. A leafref checked with no leaf is refused (`TestValidateLeafrefNeedsLeaf`), so a leafref member of a union default is refused at schema build: no Ze module has one.
-> Decision: reference failures use `ErrTypeType`; no new `ErrorType`, which would need a new diagnostic code.
- Owed by phase 2: independent `/ze-review`; independent re-judgement (`./le rfc audit-stamp ... mode rejudge`) of the verdicts `./le rfc check` reports STALE: RFC7950-9.1-1, 9.1-2 (its audit record says unimplemented), 7.6.5-1, 8.3.3-2, 7.2.2-1; then `./le rfc index-update`; gates `./le go lint run` and `./le verify worktree` in a fresh agent.

#### Review Gate

| Round | Reviewer | Scope | BLOCKER | ISSUE | Disposition |
|-------|----------|-------|---------|-------|-------------|
| 1-3 | independent agents, 2026-10-18 | bccd1e7cf5, then 2e4be94ae3, then 8a404499d5 | see commits | see commits | fixed by 2e4be94ae3, 8a404499d5, 40a03eb762 and 32d2ed5b51 |
| 4 | independent agent (closure, 2026-10-21), author of none of the five commits | whole change, bccd1e7cf5^..HEAD over the spec's files | 0 | 3 | all fixed in the round-4 commit; a fresh reader owes round 5 |
| 5 | independent agent, 2026-10-21 | whole change after round 4 | 0 | 3 | all fixed by 495eb83095 |
| 6 | independent agent, 2026-10-21, author of none of the commits | whole change after 495eb83095, with a probe matrix of 34 module sets | 0 | 2 | both fixed in the round-6 fix commit; a fresh reader owes round 7 |
| 7 | independent agent, 2026-10-21, author of none of the commits | whole change after the round-6 fix commit, with probes R1-R7 and N1-N14 (augments, nested groupings, choices, submodule deviations, leaf-list deviations, `-conf` sections) | 0 | 1 | ISSUE and NOTE 1-3 fixed in the round-7 fix commit; a fresh reader owes round 8 |
| 8 | independent agent, 2026-10-21, author of none of the commits | whole change after the round-7 fix commit, with probes Q1-Q3, L1-L11, P1-P5, N13-N13h (rpc, action and notification paths, submodules, typedefs, augments of augments, `-conf` sections) | 0 | 2 | ISSUE 1-2 and NOTE 1-3 fixed in the round-8 fix commit, with the accessible-tree sweep below; a fresh reader owes round 9 |
| 9 | independent agent, 2026-10-21, author of none of the commits (owner-authorised round, Thomas 2026-10-21) | 3997109ab6 and what it touched: every row of `TestLeafrefAccessibleTreeMatrix` judged against Section 6.4.1 and its examples (6.4.1.1), then probes P1-P14 (notification and action in a grouping used twice, rpc to another rpc, choice and container in input, augments, `uses` augments and deviations into one use of a grouped action's input, `current()` into an operation, a nested notification augmented into a list) | 0 | 2 | ISSUE 2 fixed in the round-9 fix commit; ISSUE 1 fixed by the vendor patch `goyang-entry-dup-rpc.patch` (below); round 10 owed over that fix |

Round 9 findings:
- Matrix: all 81 rows agree with Section 6.4.1 and the 6.4.1.1 examples (the action "reset" and notification "down" instances sit under `/a/b`, so `../../bx` from an action parameter is the action's container and `/nn/x` is no path).
- ISSUE 1 (fixed 2026-10-21, as a vendor patch, not a fork): `internal/le/vendorpatch/patches/goyang-entry-dup-rpc.patch` makes `Entry.dup` copy `RPC.Input` and `RPC.Output` with `dup()`, parented to the copy, and makes `Entry.Find` parent an input or output it creates for an augment to its operation (deleting `reparentOperationParams` showed that second case: an augment adding the input of an action that declares only an output panicked the loader). `reparentOperationParams` and `copyEntry` are deleted. Tests `TestAugmentAndDeviationReachOneUseOfGroupedAction` (P10, P11, P12 and the deviation reaching box1), red observed without the patch (P10 loaded), and `TestAugmentCreatesInputOfAction`, red observed without the `Find` hunk (panic). The finding as recorded: an augment or deviation aimed at the input or output of one use of a grouped action reaches every use. goyang's `Entry.dup` copies `RPC` by pointer, and `Modules.Process` applies augments and deviations before `Loader.Resolve` runs `reparentOperationParams`, which then copies the already-changed shared input. Probe P10 (`augment "/a:box1/a:act/a:input" { leaf z }`, then a leafref `../b:z` augmented into box2's input) loads though box2's input holds no z (AC-2 fail-open, Section 6.4.1); probe P12 (`deviate not-supported` on box1's `x`) refuses a valid `../a:x` in box2 and names `/a/box1/act/input/r`, a node the module never defined. Section 7.17: "The argument is a string that identifies a node in the schema tree.  This node is called the augment's target node. ... It is augmented with the nodes defined in the substatements that follow the "augment" statement." The target is box1's input, not box2's. Fix: `Entry.dup` copies `RPC.Input` and `RPC.Output` with `dup()` and parents them to the copy (planned then as a fork change, superseded by the vendor patch); then delete `reparentOperationParams` and `copyEntry` (no-layering), and turn P10-P12 into a regression test. Run against the patched goyang through an alternate modfile, P10, P11 and P12 answer correctly and `./internal/component/config/...` keeps the failure set of HEAD (config/cli, config/infra, config/yang/cli fail on HEAD without it).
- ISSUE 2 (fixed): a step naming an rpc, action or notification outside the accessible tree was refused with "an unprefixed name names a node of module a ..., whose top level holds none called "op2"", false when module a's top level holds the rpc op2 (P5; also `/note/x` from config). `outsideOperation` (`reference_path.go`) names what the step reaches: `"op2" is an rpc, not a data node, and of the rpcs, actions and notifications the accessible tree holds only the operation the referring leaf is defined in (RFC 7950 Section 6.4.1)`. Test `TestLeafrefOperationOutsideTreeNamed`, red observed (5 cases).
- NOTE: `schemaChild` now skips an rpc, action or notification for `walkTree` as well, so a config key named after an action is left to the config reader as unknown; config data never holds one.

Round 8 findings:
- ISSUE 1 (fixed): rpc and action input and output paths resolved one level off: `dataParent` treated goyang's input and output entries as data nodes, and `topLevelParents` never saw the operation's parameters. Section 6.4.1: "Additionally, for an RPC, the root node also has the node representing the RPC operation being defined as a child.  The node representing the operation being defined has the operation's input parameters as children." Probe Q2 (`../../../x` from an action input) loaded; Q1, L5, P1, P2 and P3 were refused though valid. Fix (`reference_path.go`): `dataParent` passes through input and output (`isPathTransparent`); `schemaChild` never answers an rpc, action or notification (`isOperation`); `pathContext.child` gives the accessible tree the operation being defined alone, at the root for an rpc or a top-level notification and under its data node for an action or a nested notification (Section 7.15.2), with the parameters of the referring leaf's own input or output as children. The sweep showed the same defect's other face: a config or state leafref into a notification (`/note/x`, `/box/nn/x`) loaded, because goyang keeps the notification's nodes in its Dir; now refused.
- ISSUE 2 (fixed): `rootStepsFix` named only the first missing import, so following the fix met a second refusal (N13c, then P5). It collects every missing import. Test `TestLeafrefRootFixNamesEveryImport`, red observed.
- NOTE 1 (fixed): the L11 refusal said an unprefixed name named the referring module and that the same module held the path. When the holder is the referring module it now says so plainly. Test `TestLeafrefRootFixHolderIsReferringModule`, red observed.
- NOTE 2 (fixed): `topLevel`'s doc comment named three sources; it reads two.
- NOTE 3 (fixed): this row and the round-9 scope row.
- Sweep defect (fixed): goyang copies an rpc or action for a grouping use or a submodule include without copying its input and output, which stay parented to the grouping or submodule copy. A leafref in an action input defined in a grouping panicked the loader (`InstantiatingModule` on a grouping parent), and two uses of one grouping shared one input. `reparentOperationParams` (`loader_reference.go`), called from `Loader.Resolve` before any path is resolved, gave each copy its own input and output; since round 9 ISSUE 1 the goyang vendor patch does it inside `Entry.dup` and the function is deleted. Test `TestLeafrefOtherInstanceOfOperationGrouping`, red observed (the panic).
- Sweep wording (fixed): an unprefixed first step refused at the top level with no fix (a step with a predicate) now names the namespace it was looked up in.
- Decision: an action or a nested notification is reachable as a child of the data node it is defined in, by absolute or relative path. Section 6.4.1 puts only an rpc or a top-level notification at the root; Section 7.15.2 says the action instance "MUST contain all containers and list nodes in the direct path from the top level down to the list or container containing the action", and Section 7.16.2 says the same of a nested notification.

Round 7 findings:
- ISSUE 1 (fixed): an unprefixed first step at the root, from a leaf another module augments in, was looked up in the augment target's module (`moduleRoot(leaf)`), so `/top2/z`, `../../top2/z`, a `current()/../../top2/z` predicate and the same path in a grouping used in the augment were refused as naming no node. Section 6.4.1: "Names without a namespace prefix belong to the same namespace as the identifier of the current node."; Section 4.2.8: "If a module augments another module, the XML elements that are added to the encoding are in the namespace of the augmenting module." Fix: `topLevelParents` (`reference_path.go`) looks the step up in the root of `ctx.namespace` (`namespaceRoot`); the reference walk's `childEntries` (`validator_instance.go`) takes its own-module root from the leaf's namespace too (`leafNamespaceRoot`). Test `TestLeafrefUnprefixedRootInAugmentingModule`, red observed against the round-6 code.
- NOTE 1 (fixed, main-thread decision: an implemented RFC gets full, correct coverage): the merged `-conf` section exception (`mergedSection`) departed from Section 6.4.1 with no recorded deviation and was wider than its comment. Removed: an unprefixed name resolves strictly in the referring node's namespace, and a path into another module's section writes that module's prefix. Every Ze YANG reference path still loads (the only one, `ze-traffic-control-conf.yang` `default-class`, is relative within its module). Test `TestLeafrefUnprefixedNoMergedSectionException`, red observed; the `merged conf section` case of `TestLeafrefUnprefixedStepNamespace` moved from accepted to refused; `TestLeafrefPrefixedIntoOtherConfSection` proves the walk still reads a prefixed path into another module's merged section.
- NOTE 2 (fixed): `/sec/y`, where only another `-conf` module's `sec` holds `y`, was refused with no fix. `rootPathFix` names the prefixed path (`/zb:sec/zb:y`) and the import to add.
- NOTE 3 (fixed): no test covered the submodule branch of `deviationStatement` or a `deviate replace` on a leaf-list. Tests `TestLeafrefFromSubmoduleDeviation` and `TestLeafrefDeviateReplaceLeafList` (probes N10-N11c); both pass against the round-6 code, so they guard behaviour already correct.

Round 5 findings (495eb83095):
- ISSUE 1 (fixed): a prefixed first step into another module's top level was refused; Section 6.4.1, "The root node has all top-level configuration data nodes in all modules as children." `topLevelParents` looks the step up in the module its prefix names.
- ISSUE 2 (fixed): a typedef's path prefixes resolved in the module using the typedef; Section 6.4.1 names "the module where the XPath expression is specified". `pathStatement` finds the statement that writes the path.
- ISSUE 3 (fixed): prefix refusals did not name the fix, and two quoted a leaf name as a module name. `prefixFix`.

Round 6 findings:
- ISSUE 1 (fixed, regression from 495eb83095): a leafref a `deviate replace` gives a leaf was refused as "a Ze defect", because `pathStatement` searched only the leaf's own type statement. Section 7.20.3.2: "The argument "replace" replaces properties of the target node." Fix: `deviationStatement` (`reference_path.go`) searches the deviate type statements of every loaded module and submodule, so the prefixes resolve in the deviating module; the nil-statement refusal (`noPathStatement`) no longer calls valid YANG a Ze defect. Test `TestLeafrefFromDeviateReplace` (`reference_namespace_test.go`), red observed against HEAD.
- ISSUE 2 (fixed, predates round 5, AC-2 fail-open): an unprefixed path name was matched by local name in any namespace, so `../y` in pe loaded naming pd's y. Section 6.4.1: "Names without a namespace prefix belong to the same namespace as the identifier of the current node." Fix: `checkStepNamespace` at every step, key predicate and `current()` name, keeping the merged `-conf` top-level section exception; the `checkPathPrefix` comment claiming the local-name match followed 6.4.1 corrected. Test `TestLeafrefUnprefixedStepNamespace`, red observed against HEAD; the traffic `default-class` leafref still loads.
- NOTE (fixed): an undeclared prefix, or one naming a module that lacks the node, on a first step was refused as "names no node" (probes T3, U3), not with the prefix refusal the `topLevelParents` comment promised. The prefixed lookup now falls back to every loaded module's top level (`loadedTopLevelParents`). Test `TestLeafrefPrefixRefusedAtTopLevel`.
- NOTE: goyang applies a `deviate add` type as it does `replace`; Section 7.20.3.2 forbids adding a property that can appear once and a leaf always has a type. Journal row, not fixed here.

Round 4 findings:
- ISSUE 1 (fixed): a leafref path step, key predicate or `current()` name was matched by its local name alone, so `../bogus:l/k` and a prefix naming an imported module that does not hold the node loaded (AC-2 fail-open; RFC 7950 Section 6.4, "all prefixes used MUST be present in the XPath context"). Fix: `checkPathPrefix` (`reference_path.go`), test `TestLeafrefPathPrefixChecked` (`reference_prefix_test.go`), red observed with the check disabled.
- ISSUE 2 (fixed): `checkLeafrefSchema` refused a leafref under an rpc or action input, or a notification, that refers to state, because goyang answers `ReadOnly` false there; Section 9.9 binds only a referring node that "represents configuration data". Fix: `representsConfig` (`loader_reference.go`), test `TestLeafrefNonConfigNodeReferringToState`, red observed (both nodes refused) with the check disabled.
- ISSUE 3 (fixed): two claims made false by AC-1, `internal/component/l2tp/config.go` `validateRelays` ("No leafref exists in ze's YANG engine") and the `ze-l2tp-conf.yang` remote `name` description, corrected; `yang-config-design.md` loader table carries the two fixes.
- NOTE: `test/parse/yang-leafref-default-class.ci` claimed the refusal names the classes that exist and did not assert it; `expect=stdout:contains=name one of "data voice"` added, suite run green.
- NOTE: an instance-identifier step at a leaf-list with no `[.='v']` is accepted and answers existence of any entry (Section 9.13 states no MUST for it).
- NOTE: `canonicalValue` compares a union- or leafref-typed referred leaf as written text.
- Pre-existing, not from this change: `go test ./internal/component/config/yang/cli/` fails at HEAD without the round-4 edits (`TestDocCommand`, "unknown command: show bgp peer list").

##### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | A union picks the first member that matches; require-instance false never consults the tree |
| Naming | Errors name the path, the value, and the fix |

#### Checklist

- [ ] Tests written
- [ ] Tests FAIL before the change
- [ ] Tests PASS after the change
- [ ] `./le verify worktree`

##### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Tagged unit tests | `./le rfc check` |
| Functional test | `./le test parse` selector |

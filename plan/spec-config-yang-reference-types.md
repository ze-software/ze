# Spec: YANG reference types leafref, identityref and instance-identifier

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | - |
| Phase | 1 |
| Handoff | - |
| Updated | 2026-10-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner decision, 2026-10-18, relayed by the main thread: IMPLEMENT the three
YANG built-in types Ze refuses today, leafref (RFC 7950 Section 9.9),
identityref (9.10) and instance-identifier (9.13), so RFC7950-9.1-1 ("MUST
support all lexical representations") can be proven whole. This spec replaces
the three skeletons `plan/spec-config-yang-type-leafref.md`,
`plan/spec-config-yang-type-identityref.md` and
`plan/spec-config-yang-type-instance-identifier.md`, which are removed in the
same commit; their Task tables are carried into the AC list below.

Context: commit fd9d7ed55c made the loader refuse a module declaring one of the
three types (`ErrUncheckedType`) and the validator refuse their values
(`uncheckedKindError`). That refusal is deleted first (`ai/rules/no-layering.md`)
and replaced by the checks below.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` -- the loader checks table and the value check table
  -> Constraint: every loader refusal and every value check has a row there.
- [ ] `docs/plugin-development/schema.md` -- "References (no leafref)" section, made false by this spec
- [ ] `docs/architecture/hub-architecture.md`, `docs/architecture/hub-api-commands.md`, `docs/architecture/system-architecture.md` -- each says no Ze module can declare a leafref

### RFC Summaries
- [ ] `rfc/full/rfc7950.txt` Sections 9.1, 9.9 to 9.9.4, 9.10 to 9.10.3, 9.13 to 9.13.2, 14 (path-arg, instance-identifier)
- [ ] `rfc/short/rfc7950.md` rows RFC7950-9.1-1, 9.1-2, 9.9-1, 9.9.2-1, 9.9.3-1, 9.10.2-1, 9.10.3-1, 9.13-1

## Current Behavior (MANDATORY)

- [ ] `internal/component/config/yang/loader_unchecked.go` -> `moduleUncheckedTypeErrors` refuses the three types at `Loader.Resolve` through `checkStructure`.
- [ ] `internal/component/config/yang/validator.go` -> `validateYangType` sends `Yleafref`, `Yidentityref`, `YinstanceIdentifier` to `uncheckedKindError`.
- [ ] `internal/component/config/yang/loader_grammar.go` -> `checkPathArg` / `pathArg` checks a leafref path-arg's syntax and keeps nothing of it.
- [ ] `internal/component/config/validate_sections.go` -> `ValidateCustomSections` walks each section alone, so no check sees two sections at once.
- [ ] goyang resolves an identityref base (`YangType.IdentityBase`, whose `Values` holds every identity derived from it, transitively) and refuses an identityref with no base; it keeps the leafref path as text (`YangType.Path`) and `require-instance false` as `OptionalInstance`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
A module reaches `Loader.Resolve`; a config value reaches `Validator.ValidateTree` / `ValidateTreeAllModules` (per section) and the new whole-tree `Validator.ValidateReferences`, both called by `config.ValidateCustomSections`, which `ze config validate`, the daemon's load and the editor's commit share.

### Transformation Path
1. Load: `Loader.Resolve` -> `checkReferences` over every resolved entry: a leafref path parses (`parsePathArg`), resolves to a leaf or leaf-list schema node, a config leafref with require-instance points at config, and no leafref chain loops.
2. Value (lexical): `validateYangType(path, leaf, type, value)`: identityref -> a derived identity of the base; instance-identifier -> the Section 14 syntax, prefixed names, each step a data node, key predicates complete; leafref -> the value checked against the referred leaf's type.
3. Value (instance): `ValidateReferences(root)` walks the whole data tree; for each leafref or instance-identifier value with require-instance true, the referred instance exists in the tree, or is a leaf whose default is in use.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| config -> config/yang | `ValidateCustomSections` calls `Validator.ValidateReferences(tree.ToMap())` | [ ] |

### Integration Points
`validateYangType` switch; `Loader.Resolve`; `ValidateCustomSections`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No new import between components | yes | all code in `internal/component/config` |
| One parser for path-arg | yes | `checkPathArg` becomes a wrapper of `parsePathArg` |

## Risks & Assumptions

### Assumptions
- Config text carries no XML namespace. An identityref value may name the identity unprefixed when exactly one derived identity has that name, or with the defining module's prefix or name; an instance-identifier value MUST be prefixed (9.13.2) with the module prefix or name.
- Values compare by their string as the tree stores it, except integer kinds, which compare by number.

### Risks
- The traffic-control `default-class` leaf becomes a leafref (the shape of the RFC's own 9.9.6 example). A config naming an absent class, accepted before and silently ignored by the netlink backend, is now refused at validate and commit.
- goyang reads one `base` per identityref; a second base is refused by goyang at parse. Recorded on RFC7950-9.10.2-1 as partial.

## Blast Radius
| Question | Answer |
|----------|--------|
| Who loads modules | every Ze binary, at schema build |
| Who validates values | `ze config validate`, daemon load, editor commit, web commit |
| Existing modules using the types | none before this spec; traffic-control `default-class` after |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point -> Feature Code -> Test |
|-------------------------------------|
| `ze config validate` -> `ValidateCustomSections` -> `ValidateReferences` -> `test/parse/yang-leafref-default-class.ci` |

## Acceptance Criteria
- AC-1: a module declaring leafref, identityref or instance-identifier loads; `ErrUncheckedType` and `uncheckedKindError` for those kinds no longer exist.
- AC-2: a leafref path that does not parse, names no node, or ends at a node that is not a leaf or leaf-list is refused at load (RFC7950-9.9.2-1).
- AC-3: a config leafref with require-instance true whose target is not config is refused at load; a leafref chain that loops is refused at load (RFC7950-9.9-1).
- AC-4: a leafref value is checked against the referred leaf's type (9.9.4), and with require-instance true it must equal a referred instance in the tree, or a default in use (RFC7950-9.9-1, 9.9.2-1, 9.9.3-1); require-instance false skips the instance check.
- AC-5: an identityref value must name an identity derived from the base, unprefixed or with the defining module's prefix or name; the base itself, an unknown name and a wrong prefix are refused (RFC7950-9.10.2-1, 9.10.3-1, 9.1-2).
- AC-6: an instance-identifier value must match the Section 14 rule with every node name prefixed, name data nodes, carry one predicate per key of each list it selects; with require-instance true the instance must exist (RFC7950-9.13-1, 9.1-2).
- AC-7: a union member of one of the three types answers by its own check, in member order.
- AC-8: traffic-control `default-class` is a leafref to `../class/name`; `ze config validate` refuses a config naming an absent class with the path and the fix, and accepts one naming a present class (functional test).
- AC-9: `rfc/short/rfc7950.md` rows lose the gap annotations the code now meets; RFC7950-9.1-1 is re-judged by an independent agent; `./le rfc check` clean.

## End-to-End User Stories
- An operator writes `default-class bulk;` under a qdisc with no class `bulk`; `ze config validate` names the leaf, the missing class, and the classes that exist.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestRFC7950LeafrefPath* | `internal/component/config/yang/rfc7950_reference_test.go` | AC-2, AC-3 | [ ] |
| TestRFC7950LeafrefValue*, TestRFC7950LeafrefInstance* | same | AC-4 | [ ] |
| TestRFC7950Identityref* | same | AC-5 | [ ] |
| TestRFC7950InstanceIdentifier* | same | AC-6 | [ ] |
| TestRFC7950ReferenceUnionMember | same | AC-7 | [ ] |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| instance-identifier pos | 1..count | count | 0 | count+1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| yang-leafref-default-class | `test/parse/yang-leafref-default-class.ci` | AC-8 | [ ] |

### Interop Tests (Scope: protocol)
Not applicable: config-only feature with no protocol peer.

## Files to Modify
- `internal/component/config/yang/loader_grammar.go`, `loader_structure.go`, `loader.go`, `validator.go`, `validator_lexical.go`, `validator_lexical_test.go`
- `internal/component/config/validate_sections.go`, `yang_schema.go`, `yang/module_default.go`
- `internal/component/traffic/yang/ze-traffic-control-conf.yang`
- docs listed under Required Reading; `rfc/short/rfc7950.md`

## Files to Create
- `internal/component/config/yang/reference_path.go`, `validator_reference.go`, `loader_reference.go`, `rfc7950_reference_test.go`
- `test/parse/yang-leafref-default-class.ci`

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/architecture/config/yang-config-design.md` loader and value tables
- [ ] `docs/plugin-development/schema.md` references section
- [ ] hub-architecture, hub-api-commands, system-architecture leafref sentences

## Implementation Phases
1. Delete the refusal; path-arg parser records its steps; loader reference checks; value checks; whole-tree instance pass; traffic leaf; unit and functional tests; docs; RFC summary and discrimination records.
2. Independent review (`/ze-review`, an agent that did not write the code) and independent re-judgement of RFC7950-9.1-1, then `/ze-close`.

## Implementation Steps
Phase 1 in the order listed; tests first for each check.

### Phase 1 result (2026-10-18)
- Done: refusal deleted (`loader_unchecked.go` removed by the commit, `uncheckedKindError` kept for `Ynone` only); `parsePathArg` records steps; `checkReferences` at `Loader.Resolve`; `validateLeafref`, `validateIdentityref`, `validateInstanceIdentifier`; `Validator.ValidateReferences` wired through `referenceFailures` in `ValidateCustomSections`; traffic `default-class` is a leafref; 17 tagged unit tests in `rfc7950_reference_test.go`; `test/parse/yang-leafref-default-class.ci` (red observed with the leaf reverted to `type string`); five doc pages; `rfc/short/rfc7950.md` annotations.
-> Decision: `ValidateType(path, type, value)` keeps its signature (many RFC-tagged tests call `validateYangType`); the leaf-aware path is `ValidateLeafType` / `validateLeafValue`. A leafref checked with no leaf is refused (`TestValidateLeafrefNeedsLeaf`), so a leafref member of a union default is refused at schema build: no Ze module has one.
-> Decision: reference failures use `ErrTypeType`; no new `ErrorType`, which would need a new diagnostic code.
- Owed by phase 2: independent `/ze-review`; independent re-judgement (`./le rfc audit-stamp ... mode rejudge`) of the verdicts `./le rfc check` reports STALE: RFC7950-9.1-1, 9.1-2 (its audit record says unimplemented), 7.6.5-1, 8.3.3-2, 7.2.2-1; then `./le rfc index-update`; gates `./le go lint run` and `./le verify worktree` in a fresh agent.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | A union picks the first member that matches; require-instance false never consults the tree |
| Naming | Errors name the path, the value, and the fix |

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the change
- [ ] Tests PASS after the change
- [ ] `./le verify worktree`

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Tagged unit tests | `./le rfc check` |
| Functional test | `./le test parse` selector |

# Spec: YANG loader refuses the module structures RFC 7950 forbids

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

→ Decision (2026-10-09, status audit): the AC-2 code landed (12f339513b, 6c06cb43af, 037cc25e29, a812b18c41, 7070e5861a, 66e87707f5, 180df07922) while this spec still read `skeleton`. It never passed `design` or `ready`, and no design for it was ever presented to the owner: the work ran under the owner orders quoted in the progress notes below ("fix the three tests", 2026-10-08; "deal with it now", 2026-10-09). The status moves to `in-progress` because implementation has started. The sections below record the design the landed code took, read from source and tests rather than from the progress notes. The design of the eighteen AC-1 rows still open was never written; it is owed before their implementation and needs the owner's approval like any design.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - how Ze loads and resolves its YANG modules
  → Constraint: every module passes `Loader.Resolve`, which joins goyang's own resolution with Ze's checks; only a successful `Resolve` yields a `*yang.Resolved` (180df07922).

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7950.md`, and `rfc/full/rfc7950.txt` at each section the Task table cites
  → Constraint: each id in the Task table keeps `{gap}` naming this spec until it holds a positive and a negative tagged test with a discrimination record.

**Key insights:**
- goyang accepts modules RFC 7950 forbids; Ze adds `checkStructure`, `checkExtensions` and `checkPatterns`, each joined by `Resolve`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/yang/loader.go` - `Resolve` runs goyang's `Modules.Process`, then joins `checkExtensions`, `checkPatterns` and `checkStructure`; `DefaultLoader` returns the `*Resolved` of the embedded modules
- [ ] `internal/component/config/yang/loader_structure.go` - `checkStructure`: length parts disjoint and ascending, min/max read as the restricted type's bounds (9.4.4); enum restriction a subset that keeps base values (9.6.4); enum values within int32 and unique (9.6.4.2); extension substatements are YANG statements (7.19)
- [ ] `internal/component/config/yang/loader_abnf.go`, `loader_grammar.go`, `rfc7950.abnf` - the Section 14 grammar read from the embedded ABNF and applied to statements under an extension
- [ ] `internal/component/config/yang/loader_source.go` - binds each module to its source text; `statementHasBlock` answers what goyang does not record
- [ ] `internal/component/config/yang/enum_assignment.go` - `assignEnumValues` assigns enum values per Section 9.6.4.2
- [ ] `internal/component/config/yang/loader_rfc7950_test.go` - pins, untagged, what goyang refuses

**Behavior to preserve:**
- Every embedded Ze module loads through `DefaultLoader` (`TestPublishedExtensionUsagesLoad` and the package tests).

**Behavior to change:**
- A module carrying one of the Task table's faults is refused with an error naming the fault, rather than loading or crashing goyang.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Module text reaches `Loader.AddModuleFromText`, then `Loader.Resolve`.

### Transformation Path
1. Module text -> goyang AST (`yang.Statement`), with the source text Ze keeps beside it (`loader_source.go`).
2. `Resolve`: goyang `Modules.Process`, then `checkExtensions`, `checkPatterns` and `checkStructure` over every loaded module and submodule; the errors are joined.
3. `*yang.Resolved` -> Ze schema nodes through `yangToNode`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None | every step is inside `internal/component/config` | Yes |

### Integration Points
- `Loader.Resolve` and `DefaultLoader` - the only producers of `*yang.Resolved`, so no caller builds a schema from a module that failed a check.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | `loader.go` `Resolve` joins `checkStructure` beside goyang's own processing |
| No unintended coupling | Yes | the checks live in `internal/component/config/yang` |
| No duplicated functionality | Yes | each check covers a rule goyang accepts; what goyang refuses stays pinned in `loader_rfc7950_test.go` |
| Zero-copy preserved where applicable | Yes | load time only, no wire path |
| Registration over hardcoding, outbound | Yes | no command, family or handler added |
| Registration over hardcoding, inbound | Yes | the Section 14 rules are read from the embedded `rfc7950.abnf` (`loader_abnf.go::parseYANGGrammar`), not listed by hand |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every embedded Ze module passes the new checks | `DefaultLoader` runs at every daemon start | the daemon refuses to start | `TestPublishedExtensionUsagesLoad` and the config package tests | validated for AC-2 |
| A-2 | The goyang fork's enum numbering fix stays until upstream ships it | `go.mod` replace comment (66e87707f5) | goyang refuses a valid module | `TestGoyangNumbersAnEnumAfterANegativeValue` | validated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A check refuses a valid module | a Ze module stops loading | each check carries a positive tagged test |
| R-2 | Three open AC-1 faults crash goyang (self-using grouping, self-based identity, augment of a leaf) | stack overflow or nil-map panic in `ToEntry` | the Ze check for those rows runs before goyang's `Process`; settled at their design |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | the daemon refuses its own embedded modules at start |
| How is it reverted? | single commit revert |
| Who else touches this path? | `plan/spec-validated-construction-and-state-types.md` Phase 1 builds on `Resolve`, `DefaultLoader` and `checkStructure` (its A-5) |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `Loader.AddModuleFromText` then `Loader.Resolve` | → | `loader_structure.go::checkStructure` | `TestRFC7950LengthPartsDisjointAndAscending`, `TestRFC7950EnumRestrictionKeepsTheBaseValue`, `TestRFC7950ExtensionUsageSubstatementsAreYANGStatements` |
| `Loader.Resolve` | → | `loader_grammar.go`, the Section 14 rules under an extension | `TestRFC7950ExtensionSubstatementContextForms` |
| `DefaultLoader` (daemon start) | → | `Resolve` over every embedded module | `TestPublishedExtensionUsagesLoad` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a module carrying the fault of any requirement id in the Task table | `Resolve` refuses it naming the fault; each id carries a positive and a negative tagged test with a discrimination record, and its `{gap}` leaves `rfc/short/rfc7950.md` |
| AC-2 | a module whose length parts overlap or descend (9.4.4), whose enum values leave int32 or repeat (9.6.4.2), or whose extension substatements are not Section 14 statements (7.19) | `Resolve` refuses it; an independent `ze-rfc-audit` judges RFC7950-7.19-1, RFC7950-9.4.4-1 and RFC7950-9.6.4.2-1 `enforced` |

### AC audit, 2026-10-09

Read from source, tests, `rfc/audit/rfc7950.json` and `rfc/short/rfc7950.md`, not from the progress notes.

| AC | Part | Verdict | Evidence |
|----|------|---------|----------|
| AC-2 | RFC7950-7.19-1 | met | audit verdict `enforced`, re-judged after 180df07922; tagged pairs in `rfc7950_extension_grammar_test.go` and `rfc7950_loader_structural_test.go` |
| AC-2 | RFC7950-9.4.4-1 | met | audit verdict `enforced`; `TestRFC7950LengthPartsDisjointAndAscending`, `TestRFC7950LengthMinMaxAreTheRestrictedTypeBounds` |
| AC-2 | RFC7950-9.6.4.2-1 | NOT met | audit verdict `weak`: the goyang numbering fix was proven only by the untagged `TestGoyangNumbersAnEnumAfterANegativeValue`, so dropping the fork reddens no tagged unit. Tagging it was tried on 2026-10-09 and withdrawn: `./le rfc discriminate-record` cannot prove a vendored producer (`vendor/github.com/openconfig/goyang/pkg/yang/types_builtin.go::Set` reads "never executes", because the coverage profile names the import path; journal row in `plan/journal/check-cannot-see-the-change-it-looks-for.md`), and a tag without its record fails `./le rfc check`. Owed: that tool fix, then the tag and record, then an independent re-judgement |
| AC-2 | red file replaced | met | `loader_rfc7950_structural_red_test.go` is gone; `rfc7950_loader_structural_test.go` holds the tagged proofs |
| AC-1 | RFC7950-9.6.4-2 | met in code and tests; no audit verdict yet | its `{gap}` left the summary; `TestRFC7950EnumRestrictionKeepsTheBaseValue` tagged positive and negative |
| AC-1 | the other eighteen ids | NOT met, undesigned | each still carries `{gap}` naming this spec: 5.1-1, 5.5-1, 5.6.5-1, 6.2.1-1, 6.3.1-1, 7.1.4-1, 7.3-1, 7.9.2-1, 7.9.3-1, 7.12-1, 7.15-1, 7.16-1, 7.17-1, 7.18.2-1, 7.21.2-1, 9.2.4-2, 9.6.4-1, 9.12-2 |
| AC-1 | RFC7950-7.2.2-1 | NOT met | carries `{gap}` naming this spec; the Task table folds its sentence under 5.1-1 |

The original wording of both ACs:

- AC-1: every requirement id in the Task table carries a positive and a negative tagged test with a discrimination record, and its `{gap}` annotation leaves `rfc/short/rfc7950.md`.
- AC-2: the weak verdicts RFC7950-7.19-1 (extension substatement syntax), RFC7950-9.4.4-1 (length values non-negative, disjoint, ascending) and RFC7950-9.6.4.2-1 (enum value range and uniqueness; the last sentence of §9.6.4.2) are re-judged `enforced` once the loader performs these structural checks. The untracked red `internal/component/config/yang/loader_rfc7950_structural_red_test.go` is replaced by tagged proofs. Moved here from spec-rfc-verdict-fix-services under P-3 (ruling R4), 2026-09-30.

Progress 2026-10-08 (owner order "fix the three tests"):
- AC-2, code and proofs: met. `loader_structure.go::checkStructure`, joined by `Resolve` and `DefaultLoader`, refuses length parts that overlap or descend (9.4.4), an enum restriction that adds a name or changes a value (9.6.4, 9.6.4.2), and a non-YANG statement or a missing argument under an extension statement (7.19). The red file is now `rfc7950_loader_structural_test.go`, tagged RFC7950-9.4.4-1, RFC7950-9.6.4.2-1, RFC7950-9.6.4-2 and RFC7950-7.19-1, positive and negative, each with a revert discrimination record.
- AC-2, verdicts: NOT met. The audit verdicts of 7.19-1, 9.4.4-1 and 9.6.4.2-1 are stale and owe an independent `ze-rfc-audit` re-judgement; 9.6.4-2 has no verdict yet.
- AC-1: met for RFC7950-9.6.4-2 only (its `{gap}` left the summary). Every other Task row stays open.

Progress 2026-10-09 (owner order "deal with it now", RFC7950-7.19-1 Section 14 completion):
- Statements under an extension are now checked against the whole Section 14 rule, read from the embedded ABNF by `loader_abnf.go::parseYANGGrammar`: `uri-str` (RFC 3986) and `path-arg-str` have checkers and `uncheckedArgumentRules` is gone; substatement counts follow the ABNF repetition; required substatements and required blocks are refused when missing (`refine x;` through `loader_source.go::statementHasBlock`, since goyang records no block); and the production of each statement is chosen within its parent's block, so the four deviate forms, the type-body-stmts alternatives and the two augment argument forms are context-specific.
- Proofs: `rfc7950_extension_grammar_test.go`, four tests tagged RFC7950-7.19-1 positive and negative, each refused case red before the fix.
- What the ABNF does not decide: which type-body-stmts alternative a base type takes (`type int8 { length "1"; }` is grammatical; Section 9 binds restrictions to base types), and substatement order.
- The 7.19-1 audit verdict owes an independent re-judgement.

## End-to-End User Stories
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | a Ze developer adds a module whose length parts overlap | `AddModuleFromText` -> `Resolve` -> `checkStructure` refuses, naming the fault | `TestRFC7950LengthPartsDisjointAndAscending` |
| 2 | the daemon starts | `DefaultLoader` -> `Resolve` over every embedded module, which loads | `TestPublishedExtensionUsagesLoad` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC7950LengthPartsDisjointAndAscending` | `internal/component/config/yang/rfc7950_loader_structural_test.go` | 9.4.4 parts disjoint and ascending | landed |
| `TestRFC7950LengthMinMaxAreTheRestrictedTypeBounds` | same | 9.4.4 min and max read as the restricted type's bounds | landed |
| `TestRFC7950EnumRestrictionKeepsTheBaseValue` | same | 9.6.4 subset of names, base values kept | landed |
| `TestRFC7950ExtensionUsageSubstatementsAreYANGStatements` | same | 7.19 statements under an extension | landed |
| the four `TestRFC7950ExtensionSubstatement*` tests | `internal/component/config/yang/rfc7950_extension_grammar_test.go` | 7.19 Section 14 grammar under an extension | landed |
| `TestRFC7950EnumValueRangeAndUniqueness`, `TestRFC7950EnumImplicitValueFollowsTheHighest` | `internal/component/config/yang/rfc7950_enum_value_test.go` | 9.6.4.2 range, uniqueness, implicit value | landed |
| `TestGoyangNumbersAnEnumAfterANegativeValue` | `internal/component/config/yang/goyang_enum_numbering_test.go` | 9.6.4.2 positive through goyang's own numbering | landed untagged; tag and record owed (blocked on the vendored-producer tool defect) |
| one positive and one negative test per open AC-1 id | `internal/component/config/yang/loader_rfc7950_test.go` | each open Task row | owed after design |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| enum `value` | -2147483648 to 2147483647 | -2147483648 and 2147483647 | -2147483649 (`TestRFC7950EnumValueWithinInt32`) | 2147483648 |
| length part | 0 upward | 0 | a negative value (9.4.4) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| none | - | the checks run at load time over Ze's own embedded modules; every daemon start runs them through `DefaultLoader`, and no operator input reaches them | N/A |

## Files to Modify
- `internal/component/config/yang/loader.go` - `Resolve` joins `checkStructure` (landed)
- `internal/component/config/yang/enum_assignment.go` - enum values per 9.6.4.2 (landed)
- `internal/component/config/yang/loader_rfc7950_test.go` - one tagged pair per open AC-1 id (owed)

## Files to Create
- `internal/component/config/yang/loader_structure.go`, `loader_abnf.go`, `loader_grammar.go`, `loader_source.go`, `rfc7950.abnf` (landed)
- `internal/component/config/yang/rfc7950_loader_structural_test.go`, `rfc7950_extension_grammar_test.go`, `rfc7950_enum_value_test.go` (landed)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no module changes |
| YANG validation constraints | No | the loader checks module structure, not config values |
| CLI commands/flags | No | none |
| Env var registration | No | none |
| Doctor check for runtime dependencies | No | no runtime dependency |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7950.md` (`{gap}` removed for 9.6.4-2); `docs/features/rfc-status.md` is derived |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md`, to be checked against the loader checks at close |
| 16 | Changed source referenced by doc anchors? | Yes | derived at close by `./le spec citation anchors spec plan/pre-release/spec-config-yang-loader-structural-checks.md` |

## Implementation Steps

1. **Phase: AC-2 (landed)** - length, enum and extension checks in `checkStructure`, joined by `Resolve`.
   - Tests: the landed rows of the TDD table.
   - Verify: each refused case was red before its fix (records in `rfc/discrimination/rfc7950.json`).
2. **Phase: AC-2 remainder** - teach `internal/le/rfc/discriminate_observe.go::coverPackages` (and the coverage lookup) to map a `vendor/` producer to its import path, tag `TestGoyangNumbersAnEnumAfterANegativeValue` positive, record the discrimination of `TestGoyangNumbersAnEnumAfterANegativeValue` (`./le rfc discriminate-record id RFC7950-9.6.4.2-1 polarity positive unit internal/component/config/yang/goyang_enum_numbering_test.go::TestGoyangNumbersAnEnumAfterANegativeValue route revert producer vendor/github.com/openconfig/goyang/pkg/yang/types_builtin.go::Set`), then an independent `ze-rfc-audit` re-judges RFC7950-9.6.4.2-1 and judges RFC7950-9.6.4-2.
3. **Phase: AC-1 design** - one design row per open id: where the check runs (before goyang's `Process` for the three rows that crash it), the error it names, its two tests. Owner approval before implementation.
4. **Phase: AC-1 implementation** - per approved row: a failing tagged pair, the check, a discrimination record, the `{gap}` removed from the summary.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every Task id has a check at file:line and two tagged tests |
| Correctness | each error names the module, the statement and the RFC section |
| Rule: rfc-compliance | a function added from 2026-09-24 quotes the RFC 7950 sentence it implements |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| no `{gap}` naming this spec in `rfc/short/rfc7950.md` | `grep -c loader-structural-checks rfc/short/rfc7950.md` returns 0 |
| AC-2 verdicts `enforced` | `rfc/audit/rfc7950.json` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | module text is Ze-authored and embedded; a check still must not panic or recurse without bound on a cyclic module (R-2) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| A Ze module is refused | the check, unless the module breaks the RFC |
| 3 fix attempts failed | STOP, report all three |

## Design Insights
- goyang records no statement block, so a required block is read from the source text (`loader_source.go::statementHasBlock`).

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Read Section 14 from the embedded ABNF | a hand-written statement table | the ABNF is the one declaration; a table would be a second copy |
| Check every enumeration, used or not | only types goyang resolved | a grouping no node uses is still a statement the RFC binds |

## Known Limitations
- The ABNF does not decide which type-body-stmts alternative a base type takes, nor substatement order.

## RFC Documentation (Scope: protocol)

Every check carries `// RFC 7950 Section X.Y: "<quoted requirement>"` above the code that enforces it.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the change
- [ ] Tests PASS after the change
- [ ] `./le verify worktree`

## Task

Ze loads its YANG modules through goyang (`internal/component/config/yang/loader.go::AddModuleFromText`, `Resolve`). `internal/component/config/yang/loader_rfc7950_test.go` fed the loader one violating module per rule on 2026-09-21: goyang refuses the rules tagged there and ACCEPTS the ones below, and two of them CRASH it (a grouping that uses itself and an identity whose base is itself each overflow the stack in `ToEntry`; an augment whose target is a leaf panics on a nil map). This spec adds the checks to Ze's loader, before `Resolve` hands the modules to goyang, so a Ze module carrying one of these faults is refused with a message naming the fault instead of loading or crashing. Every check gets a positive and a negative tagged test in `loader_rfc7950_test.go`, where the untagged cases already pin the part of each row goyang does enforce. Scope: the embedded modules are authored by Ze, so an operator never meets this at runtime, and the release cannot go out claiming RFC 7950 conformance without it.

Each row below is a MUST of RFC 7950 the summary `rfc/short/rfc7950.md` carries as `{gap}` naming this spec. The ruling of 2026-09-21 (Thomas: "our goal is RFC compliance") makes every one a requirement until the owner declines it in one word; none is declined today.

| Id | RFC text | Producer or absence |
|----|----------|---------------------|
| RFC7950-5.1-1 | Within a server, all module names MUST be unique. (§5.1) A submodule MUST NOT include different revisions of other submodules than the revisions that its module includes. (§5.1) A module or submodule MUST NOT include submodules from other modules, and a submodule MUST NOT import its own module. (§5.1) o For a module or submodule to reference definitions in an external module, the external module MUST be imported. (§5.1) o A module MUST include all its submodules. (§5.1) There MUST NOT be any circular chains of imports. (§5.1) When a definition in an external module is referenced, a locally defined prefix MUST be used, followed by a colon (":") and then the external identifier. (§5.1) Multiple revisions of the same submodule MUST NOT be included. (§7.1.6) A submodule MUST only be included by either the module to which it belongs or another submodule that belongs to that module. (§7.2.2) | goyang (vendor/github.com/openconfig/goyang/pkg/yang, Modules.Process); Ze producer internal/component/config/yang/loader.go::(*Loader).Resolve |
| RFC7950-5.5-1 | Scoped definitions MUST NOT shadow definitions at a higher scope. (§5.5) | absent in Ze; goyang "duplicate %s %s at %s and %s" may cover it (not read), S |
| RFC7950-5.6.5-1 | A server MUST NOT implement more than one revision of a module. (§5.6.5) If a server implements a module A that imports a module B, and A uses any node from B in an "augment" or "path" statement that the server supports, then the server MUST implement a revision of module B that has these nodes defined. (§5.6.5) | internal/component/config/yang_schema.go::loadYANGModules, S |
| RFC7950-6.2.1-1 | All identifiers defined in a namespace MUST be unique. (§6.2.1) This means that any descendant node may use that typedef, and it MUST NOT define a typedef with the same name. o All grouping names defined within a parent node or at the top level of the module or its submodules share the same grouping identifier namespace. (§6.2.1) This means that any descendant node may use that grouping, and it MUST NOT define a grouping with the same name. (§6.2.1) | goyang (vendor/github.com/openconfig/goyang/pkg/yang, Modules.Process) "duplicate %s %s at %s and %s"; Ze producer internal/component/config/yang/loader.go::(*Loader).Resolve |
| RFC7950-6.3.1-1 | When an imported extension is used, the extension's keyword MUST be qualified using the prefix with which the extension's module was imported. (§6.3.1) If an extension is used in the module where it is defined, the extension's keyword MUST be qualified with the prefix of this module. (§6.3.1) Any supported extension MUST be processed in accordance with the specification governing that extension. (§6.3.1) | goyang "matchingExtensions: module prefix %q not found"; Ze processes its ze: extensions in internal/component/config/yang_schema.go::getSyntaxExtension and siblings |
| RFC7950-7.1.4-1 | If there is a conflict, i.e., two different modules that both have defined the same prefix are imported, at least one of them MUST be imported with a different prefix. (§7.1.4) All prefixes, including the prefix for the module itself, MUST be unique within the module or submodule. (§7.1.4) | goyang (vendor/github.com/openconfig/goyang/pkg/yang, Modules.Process); Ze producer internal/component/config/yang/loader.go::(*Loader).Resolve |
| RFC7950-7.3-1 | The "typedef" statement's argument is an identifier that is the name of the type to be defined and MUST be followed by a block of substatements that holds detailed typedef information. (§7.3) The name of the type MUST NOT be one of the YANG built-in types. (§7.3) If the typedef is defined at the top level of a YANG module or submodule, the name of the type to be defined MUST be unique within the module. (§7.3) The "type" statement, which MUST be present, defines the base type from which this type is derived. (§7.3.2) | goyang (vendor/github.com/openconfig/goyang/pkg/yang, Modules.Process) "missing required %s field", "duplicate %s"; Ze producer internal/component/config/yang/loader.go::(*Loader).Resolve |
| RFC7950-7.9.2-1 | The identifiers of all these child nodes MUST be unique within all cases in a choice. (§7.9.2) Schema node identifiers (Section 6.5) MUST always explicitly include case node identifiers. (§7.9.2) The case identifier MUST be unique within a choice. (§7.9.2) | goyang (vendor/github.com/openconfig/goyang/pkg/yang, Modules.Process) "duplicate %s"; Ze producer internal/component/config/yang/loader.go::(*Loader).Resolve |
| RFC7950-7.9.3-1 | The "default" statement MUST NOT be present on choices where "mandatory" is "true". (§7.9.3) There MUST NOT be any mandatory nodes (Section 3) directly under the default case. (§7.9.3) | absent; yang_schema.go::flattenChoiceCases drops the choice/case structure, S |
| RFC7950-7.12-1 | A grouping MUST NOT reference itself, neither directly nor indirectly through a chain of other groupings. (§7.12) If the grouping is defined at the top level of a YANG module or submodule, the grouping's identifier MUST be unique within the module. (§7.12) | goyang (vendor/github.com/openconfig/goyang/pkg/yang, Modules.Process); Ze producer internal/component/config/yang/loader.go::(*Loader).Resolve |
| RFC7950-7.15-1 | An action MUST NOT be defined within an rpc, another action, or a notification, i.e., an action node MUST NOT have an rpc, action, or a notification node as one of its ancestors in the schema tree. (§7.15) An action MUST NOT have any ancestor node that is a list node without a "key" statement. (§7.15) | goyang (vendor/github.com/openconfig/goyang/pkg/yang, Modules.Process); Ze producer internal/component/config/yang/loader.go::(*Loader).Resolve |
| RFC7950-7.16-1 | A notification MUST NOT be defined within an rpc, action, or another notification, i.e., a notification node MUST NOT have an rpc, action, or a notification node as one of its ancestors in the schema tree. (§7.16) A notification MUST NOT have any ancestor node that is a list node without a "key" statement. (§7.16) If a leaf in the notification tree has a "mandatory" statement with the value "true", the leaf MUST be present in a notification instance. (§7.16) If a leaf in the notification tree has a default value, the client MUST use this value in the same cases as those described in Section 7.6.1. (§7.16) In these cases, the client MUST operationally behave as if the leaf was present in the notification instance with the default value as its value. (§7.16) If a leaf-list in the notification tree has one or more default values, the client MUST use these values in the same cases as those described in Section 7.7.2. (§7.16) In these cases, the client MUST operationally behave as if the leaf-list was present in the notification instance with the default values as its values. (§7.16) | goyang (vendor/github.com/openconfig/goyang/pkg/yang, Modules.Process); Ze producer internal/component/config/yang/loader.go::(*Loader).Resolve |
| RFC7950-7.17-1 | The target node MUST be either a container, list, choice, case, input, output, or notification node. (§7.17) If the "augment" statement is on the top level in a module or submodule, the absolute form (defined by the rule "absolute-schema-nodeid" in Section 14) of a schema node identifier MUST be used. (§7.17) If the "augment" statement is a substatement to the "uses" statement, the descendant form (defined by the rule "descendant-schema-nodeid" in Section 14) MUST be used. (§7.17) The "augment" statement MUST NOT add multiple nodes with the same name from the same module to the target node. (§7.17) If the augmentation adds mandatory nodes (see Section 3) that represent configuration to a target node in another module, the augmentation MUST be made conditional with a "when" statement. (§7.17) | goyang (vendor/github.com/openconfig/goyang/pkg/yang, Modules.Process) "augment %s not found", "duplicate %s"; Ze producer internal/component/config/yang/loader.go::(*Loader).Resolve |
| RFC7950-7.18.2-1 | Otherwise, an identity with the matching name MUST be defined in the current module or an included submodule. (§7.18.2) An identity MUST NOT reference itself, neither directly nor indirectly through a chain of other identities. (§7.18.2) | goyang identity.go; Ze producer internal/component/config/yang/loader.go::(*Loader).Resolve |
| RFC7950-7.21.2-1 | If a definition is "current", it MUST NOT reference a "deprecated" or "obsolete" definition within the same module. (§7.21.2) If a definition is "deprecated", it MUST NOT reference an "obsolete" definition within the same module. (§7.21.2) | absent in Ze; goyang status-reference check unverified, S |
| RFC7950-9.2.4-2 | If multiple values or ranges are given, they all MUST be disjoint and MUST be in ascending order. (§9.2.4) Each explicit value and range boundary value given in the range expression MUST match the type being restricted or be one of the special values "min" or "max". (§9.2.4) | goyang types.go range parsing; Ze producer internal/component/config/yang_schema.go::numericRangesFromType reads the parsed ranges |
| RFC7950-9.6.4-1 | The "enum" statement, which is a substatement to the "type" statement, MUST be present if the type is "enumeration". (§9.6.4) The string MUST NOT be zero-length and MUST NOT have any leading or trailing whitespace characters (any Unicode character with the "White_Space" property). (§9.6.4) All assigned names in an enumeration MUST be unique. (§9.6.4) | goyang (vendor/github.com/openconfig/goyang/pkg/yang, Modules.Process) "duplicate %s", "missing required %s field"; Ze producer internal/component/config/yang/loader.go::(*Loader).Resolve |
| RFC7950-9.6.4-2 | When an existing enumeration type is restricted, the set of assigned names in the new type MUST be a subset of the base type's set of assigned names. (§9.6.4) The value of such an assigned name MUST NOT be changed. (§9.6.4) | goyang (vendor/github.com/openconfig/goyang/pkg/yang, Modules.Process); Ze producer internal/component/config/yang/loader.go::(*Loader).Resolve |
| RFC7950-9.12-2 | When the type is "union", the "type" statement (Section 7.4) MUST be present. (§9.12) | goyang (vendor/github.com/openconfig/goyang/pkg/yang, Modules.Process); Ze producer internal/component/config/yang/loader.go::(*Loader).Resolve; Ze consumes the member types in internal/component/config/yang/validator.go::(*Validator).validateUnion |

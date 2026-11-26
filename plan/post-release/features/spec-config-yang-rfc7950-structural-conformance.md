# Spec: Post-release RFC 7950 YANG module structural conformance

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P3 |
| Release reason | Full YANG (RFC 7950) conformance runs after the first release (`ai/META-DECISIONS-SITUATIONAL.md`, "Protocol and RFC scope"); v1.0 release fixes cover only the three crashing goyang bugs in `plan/release/fixes/spec-config-yang-loader-structural-checks.md`. |
| RFC gaps | RFC7950-5.1-1, RFC7950-5.5-1, RFC7950-5.6.5-1, RFC7950-6.2.1-1, RFC7950-6.3.1-1, RFC7950-7.1.4-1, RFC7950-7.3-1, RFC7950-7.9.2-1, RFC7950-7.9.3-1, RFC7950-7.15-1, RFC7950-7.16-1, RFC7950-7.17-1, RFC7950-7.21.2-1, RFC7950-9.2.4-2, RFC7950-9.6.4-1, RFC7950-9.12-2 |
| Depends | `plan/release/fixes/spec-config-yang-loader-structural-checks.md`, `plan/post-release/engineering/spec-gates-share-one-semantic-population.md` (for `coverPackages` vendor-producer coverage on `RFC7950-9.6.4.2-1`) |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Split out of `plan/release/fixes/spec-config-yang-loader-structural-checks.md` on 2026-11-26 per `ai/META-DECISIONS-SITUATIONAL.md` ("Full YANG (RFC 7950) coverage runs after the first release. Before it, YANG work is limited to what blocks something Ze needs.").

Ze loads YANG modules through goyang (`internal/component/config/yang/loader.go::AddModuleFromText`, `Resolve`). The three faults that crash goyang (cyclic `grouping`, cyclic `identity`, and `augment` targeting a non-augmentable node) stay in `plan/release/fixes/spec-config-yang-loader-structural-checks.md`. The five earlier structural checks that already landed at HEAD (`RFC7950-7.19-1`, `RFC7950-9.4.4-1`, `RFC7950-9.6.4-2`, `RFC7950-7.2.2-1`, and the `RFC7950-9.6.4.2-1` enum assignment plus `goyang-enum-negative.patch`) are retired into `Existing Mechanisms` there.

This post-release spec owns the fifteen non-crashing RFC 7950 structural rows that goyang still accepts without crashing, the four remaining non-crashing clauses of `RFC7950-7.17-1` (whose target-kind crash guard lands in `spec-config-yang-loader-structural-checks.md`), any non-crashing uniqueness clauses of `RFC7950-7.12-1` and `RFC7950-7.18.2-1` needed to remove their `{gap}` rows, and the remaining RFC 7950 audit proof work on landed checks (`RFC7950-9.6.4.2-1` and `RFC7950-9.6.4-2`).

Note on `rfc/short/rfc7950.md`: the `{gap}` annotations for the rows below currently still cite `plan/release/fixes/spec-config-yang-loader-structural-checks.md` and are repointed here by the main thread or when `spec-config-yang-loader-structural-checks.md` closes.

### Non-crashing RFC 7950 structural rows

| Id | RFC text | Producer or absence |
|----|----------|---------------------|
| RFC7950-5.1-1 | Within a server, all module names MUST be unique. (§5.1) A submodule MUST NOT include different revisions of other submodules than the revisions that its module includes. (§5.1) A module or submodule MUST NOT include submodules from other modules, and a submodule MUST NOT import its own module. (§5.1) For a module or submodule to reference definitions in an external module, the external module MUST be imported. (§5.1) A module MUST include all its submodules. (§5.1) There MUST NOT be any circular chains of imports. (§5.1) When a definition in an external module is referenced, a locally defined prefix MUST be used, followed by a colon (":") and then the external identifier. (§5.1) Multiple revisions of the same submodule MUST NOT be included. (§7.1.6) Note: the §7.2.2 foreign-submodule include clause (`RFC7950-7.2.2-1`) is already landed and `enforced` in `loader_structure.go::moduleIncludeErrors`. | goyang (`vendor/github.com/openconfig/goyang/pkg/yang`, `Modules.Process`); Ze producer `internal/component/config/yang/loader.go::(*Loader).Resolve` |
| RFC7950-5.5-1 | Scoped definitions MUST NOT shadow definitions at a higher scope. (§5.5) | absent in Ze; goyang duplicate checks unverified for nested scopes |
| RFC7950-5.6.5-1 | A server MUST NOT implement more than one revision of a module. (§5.6.5) If a server implements a module A that imports a module B, and A uses any node from B in an "augment" or "path" statement that the server supports, then the server MUST implement a revision of module B that has these nodes defined. (§5.6.5) | `internal/component/config/yang_schema.go::loadYANGModules` |
| RFC7950-6.2.1-1 | All identifiers defined in a namespace MUST be unique. (§6.2.1) Any descendant node may use a typedef and MUST NOT define a typedef with the same name. (§6.2.1) All grouping names defined within a parent node or at the top level of the module or its submodules share the same grouping identifier namespace, and any descendant node MUST NOT define a grouping with the same name. (§6.2.1) | goyang (`Modules.Process`); Ze producer `internal/component/config/yang/loader.go::(*Loader).Resolve` |
| RFC7950-6.3.1-1 | When an imported extension is used, the extension's keyword MUST be qualified using the prefix with which the extension's module was imported. (§6.3.1) If an extension is used in the module where it is defined, the extension's keyword MUST be qualified with the prefix of this module. (§6.3.1) Any supported extension MUST be processed in accordance with the specification governing that extension. (§6.3.1) | `loader.go::checkExtensions`; Ze processes `ze:` extensions in `internal/component/config/yang_schema.go::getSyntaxExtension` and siblings |
| RFC7950-7.1.4-1 | If two different modules that both defined the same prefix are imported, at least one MUST be imported with a different prefix. (§7.1.4) All prefixes, including the prefix for the module itself, MUST be unique within the module or submodule. (§7.1.4) | goyang (`Modules.Process`); Ze producer `internal/component/config/yang/loader.go::(*Loader).Resolve` |
| RFC7950-7.3-1 | The "typedef" statement's argument is an identifier that is the name of the type to be defined and MUST be followed by a block of substatements. (§7.3) The name of the type MUST NOT be one of the YANG built-in types. (§7.3) If the typedef is defined at the top level of a YANG module or submodule, its name MUST be unique within the module. (§7.3) The "type" statement MUST be present. (§7.3.2) | goyang (`Modules.Process`); Ze producer `internal/component/config/yang/loader.go::(*Loader).Resolve` |
| RFC7950-7.9.2-1 | The identifiers of all child nodes MUST be unique within all cases in a choice. (§7.9.2) Schema node identifiers (Section 6.5) MUST always explicitly include case node identifiers. (§7.9.2) The case identifier MUST be unique within a choice. (§7.9.2) | goyang (`Modules.Process`); Ze producer `internal/component/config/yang/loader.go::(*Loader).Resolve` |
| RFC7950-7.9.3-1 | The "default" statement MUST NOT be present on choices where "mandatory" is "true". (§7.9.3) There MUST NOT be any mandatory nodes (Section 3) directly under the default case. (§7.9.3) | absent; `yang_schema.go::flattenChoiceCases` drops choice/case structure |
| RFC7950-7.15-1 | An action MUST NOT be defined within an rpc, another action, or a notification. (§7.15) An action MUST NOT have any ancestor node that is a list node without a "key" statement. (§7.15) | goyang (`Modules.Process`); Ze producer `internal/component/config/yang/loader.go::(*Loader).Resolve` |
| RFC7950-7.16-1 | A notification MUST NOT be defined within an rpc, action, or another notification. (§7.16) A notification MUST NOT have any ancestor node that is a list node without a "key" statement. (§7.16) Mandatory and default leaf/leaf-list semantics in notification instances. (§7.16) | goyang (`Modules.Process`); Ze producer `internal/component/config/yang/loader.go::(*Loader).Resolve` |
| RFC7950-7.17-1 (non-crashing clauses) | Top-level "augment" MUST use `absolute-schema-nodeid`; "augment" under "uses" MUST use `descendant-schema-nodeid`; "augment" MUST NOT add multiple nodes with the same name from the same module to the target node; cross-module augmentation adding mandatory config nodes MUST be conditional with a "when" statement. (§7.17) Note: the crashing target-kind clause (container, list, choice, case, input, output, or notification) is owned by `plan/release/fixes/spec-config-yang-loader-structural-checks.md`. | goyang (`Modules.Process`); Ze producer `internal/component/config/yang/loader.go::(*Loader).Resolve` |
| RFC7950-7.21.2-1 | A "current" definition MUST NOT reference a "deprecated" or "obsolete" definition within the same module. (§7.21.2) A "deprecated" definition MUST NOT reference an "obsolete" definition within the same module. (§7.21.2) | absent in Ze; goyang status-reference check unverified |
| RFC7950-9.2.4-2 | Multiple range values or ranges MUST be disjoint and in ascending order. (§9.2.4) Each explicit value and range boundary value MUST match the type being restricted or be "min" or "max". (§9.2.4) | goyang `types.go` range parsing; Ze producer `internal/component/config/yang_schema.go::numericRangesFromType` |
| RFC7950-9.6.4-1 | The "enum" statement MUST be present if the type is "enumeration". (§9.6.4) The assigned name string MUST NOT be zero-length and MUST NOT have leading or trailing Unicode `White_Space` characters. (§9.6.4) All assigned names in an enumeration MUST be unique. (§9.6.4) | goyang (`Modules.Process`); Ze producer `internal/component/config/yang/loader.go::(*Loader).Resolve` |
| RFC7950-9.12-2 | When the type is "union", the "type" statement (Section 7.4) MUST be present. (§9.12) | goyang (`Modules.Process`); Ze consumes member types in `internal/component/config/yang/validator.go::(*Validator).validateUnion` |

### Remaining RFC 7950 audit proof work on landed checks

| Item | Current state at HEAD | What remains |
|------|-----------------------|--------------|
| `RFC7950-9.6.4.2-1` audit verdict (`weak` -> `enforced`) | Code and goyang vendor patch (`internal/le/vendorpatch/patches/goyang-enum-negative.patch`) and tests (`rfc7950_enum_value_test.go`, `goyang_enum_numbering_test.go::TestGoyangNumbersAnEnumAfterANegativeValue`) are all landed. | Once `plan/post-release/engineering/spec-gates-share-one-semantic-population.md` fixes `internal/le/rfc/discriminate_observe.go::coverPackages` to map `vendor/` producers to their import paths, tag `TestGoyangNumbersAnEnumAfterANegativeValue` positive for `RFC7950-9.6.4.2-1`, record its revert discrimination against `vendor/github.com/openconfig/goyang/pkg/yang/types_builtin.go::Set`, and have an independent `ze-rfc-audit` re-judge `RFC7950-9.6.4.2-1`. |
| `RFC7950-9.6.4-2` audit verdict | Code (`loader_structure.go::enumRestrictionErrors`), tagged pair (`rfc7950_loader_structural_test.go::TestRFC7950EnumRestrictionKeepsTheBaseValue`), and discrimination record (`rfc/discrimination/rfc7950/RFC7950-9.6.4-2.json`) are landed; `{gap}` already left `rfc/short/rfc7950.md`. | Run an independent `ze-rfc-audit` judgment to record the verdict in `rfc/audit/rfc7950/`. |
| `RFC7950-7.12-1` and `RFC7950-7.18.2-1` full RFC tagging and `{gap}` removal | The crashing cycle guards are owned by `plan/release/fixes/spec-config-yang-loader-structural-checks.md`. | Verify top-level grouping uniqueness (§7.12) and undefined local base identity (§7.18.2), add positive/negative RFC tags and discrimination records, obtain `ze-rfc-audit` verdicts, and clear their `{gap}` rows in `rfc/short/rfc7950.md`. |

## Required Reading

- [ ] `ai/doors/system.md` and `ai/doors/config-yang.md` - config/YANG boundaries and gates
- [ ] `docs/architecture/config/yang-config-design.md` - how `Loader.Resolve`, `checkStructure`, and goyang vendor patches compose
- [ ] `rfc/short/rfc7950.md` and `rfc/full/rfc7950.txt` - exact MUST sentences for each row above
- [ ] `internal/component/config/yang/rfc7950_loader_test.go` - existing untagged pins for what goyang already refuses

## Open Design Questions (to resolve when moving to `design`)

1. For each of the fifteen non-crashing rows, separate what goyang already refuses (pinned untagged in `rfc7950_loader_test.go`) from what Ze must check in `loader_structure.go` (or a goyang vendor patch where post-expansion state is required, such as `7.9.3-1` or `7.17-1` cross-module mandatory `when`).
2. Confirm whether `RFC7950-7.16-1` notification instance semantics belong in `validator.go` rather than `loader_structure.go` (or are already covered elsewhere).

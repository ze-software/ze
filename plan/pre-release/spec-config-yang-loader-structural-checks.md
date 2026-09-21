# Spec: YANG loader refuses the module structures RFC 7950 forbids

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-21 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Required Reading

- `docs/architecture/config/yang-config-design.md`
- `rfc/short/rfc7950.md`, and `rfc/full/rfc7950.txt` at each section the Task table cites

## Current Behavior (MANDATORY)

- [ ] `internal/component/config/yang/loader.go` -> goyang parses and resolves every module; Ze adds no structural check of its own
- [ ] `internal/component/config/yang/validator.go` -> walkTree enforces type, range, length, pattern, enum, mandatory and min/max-elements
- [ ] `internal/component/config/yang_schema.go` -> yangToNode converts the goyang entry tree to Ze schema nodes and validates defaults
- [ ] `internal/component/config/yang/loader_rfc7950_test.go` -> pins what goyang refuses today

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
Module text reaches `Loader.AddModuleFromText`, then `Loader.Resolve`; config data reaches `Validator.ValidateTree`.

### Transformation Path
Module text -> goyang AST -> `yang.Entry` tree -> Ze schema nodes through `yangToNode` -> validator walk over the config data.

### Boundaries Crossed
None: every step is inside `internal/component/config`.

### Integration Points
The schema build error accumulator `recordSchemaBuildError` and the `ValidationError` type.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point -> Feature Code -> Test |
|-------------------------------------|
| To be designed -> to be designed -> to be designed |

## Acceptance Criteria

- AC-1: every requirement id in the Task table carries a positive and a negative tagged test with a discrimination record, and its `{gap}` annotation leaves `rfc/short/rfc7950.md`.

## Risks & Assumptions

To be written at design.

## 🧪 TDD Test Plan

### Unit Tests

| Test | Asserts |
|------|---------|
| To be designed | To be designed |

## Files to Modify

To be decided at design.

## Implementation Steps

To be decided at design.

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

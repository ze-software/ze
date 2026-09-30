# Spec: Config validator evaluates when, unique and mandatory choice

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

- [ ] `internal/component/config/yang/validator.go` -> walkTree enforces type, range, length, pattern, enum, mandatory and min/max-elements, and evaluates no when, unique or choice
- [ ] `internal/component/config/yang_schema.go` -> flattenChoiceCases drops the choice and case structure; ListNode.Unique is filled and read only by the web fragment for display

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
Config data reaches `Validator.ValidateTree` from the config load and the CLI commit.

### Transformation Path
Config tree -> walkTree over the goyang `yang.Entry` tree -> `ValidationError` list.

### Boundaries Crossed
None: every step is inside `internal/component/config`.

### Integration Points
The `ValidationError` type and the CLI commit path that reports it.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point -> Feature Code -> Test |
|-------------------------------------|
| To be designed -> to be designed -> to be designed |

## Acceptance Criteria

- AC-1: every requirement id in the Task table carries a positive and a negative tagged test with a discrimination record, and its `{gap}` annotation leaves `rfc/short/rfc7950.md`.
- AC-2: moved here from the services child under OWNER RULING 4, 2026-09-30. Five rows hold tags at HEAD, so the coverage ratchet refuses a `{gap}` on them and each sentence is one clause that cannot be split; each is `weak` in `rfc/audit/rfc7950.json` and is judged `enforced` only when a tagged test proves every clause in both polarities:
  - RFC7950-7.6.5-1 ("If no such ancestor exists in the schema tree, the leaf MUST exist."): a mandatory leaf whose ancestors are all non-presence containers absent from the data (ze-bgp-conf `bgp/router-id`, `bgp/session/asn/local`) is not required, `bgp` is not in LoadConfig's validatedSections, and a missing mandatory leaf is graded a warning (`SectionValidationError.Blocking`). The untracked red `internal/component/config/validator_mandatory_rfc7950_red_test.go` belongs here. Enforcing it conflicts with the `ze:required` group/peer inheritance and with over 120 `.ci` configs that carry `bgp` without `router-id`: the design decision comes first (journal row of 2026-09-30 in `plan/journal/declared-format-contradicts-payload.md`).
  - RFC7950-8.3.3-1 ("When datastore processing is complete, the final contents MUST obey all validation constraints."): the tagged units prove a ze:validate refusal through LoadConfig, but a missing mandatory leaf is only a warning there (`SectionValidationError.Blocking`) and when, unique and choice are evaluated nowhere, so "all" does not hold.
  - RFC7950-8.1-2 ("The running configuration datastore MUST always be valid."): no tagged test drives the daemon reload (`cmd/ze/hub/main_reload.go`) or the editor commit (`internal/component/cli/editor_commit.go`).
  - RFC7950-8.3-1 (three enforcement windows): the `<edit-config>` window, the editor set and commit path, is untested.
  - RFC7950-9.1-1 ("Implementations MUST support all lexical representations specified in this document."): hexadecimal and octal integer forms in a module default (§9.2.1) are refused, and bits, binary, empty and identityref have no lexical check.

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

Ze's config validator (`internal/component/config/yang/validator.go::(*Validator).walkTree`) enforces type, range, length, pattern, enum, mandatory and min/max-elements, and evaluates NO `when` statement, NO `unique` statement and NO choice: `yang_schema.go::flattenChoiceCases` drops the choice and case structure, so data for two cases of one choice, a mandatory choice with no case, a `when` whose condition is false, and a `unique` violated by two list entries are each accepted. The modules carry 30 `when` and 9 `unique` statements that nobody reads. RFC 7950 Section 6.4 does not require an XPath interpreter, only that the requirements the model encodes are enforced; the design decides whether Ze evaluates the XPath subset its own modules use or replaces each `when` with a Go validator. This spec is the one place for these rows.

Each row below is a MUST of RFC 7950 the summary `rfc/short/rfc7950.md` carries as `{gap}` naming this spec. The ruling of 2026-09-21 (Thomas: "our goal is RFC compliance") makes every one a requirement until the owner declines it in one word; none is declined today.

| Id | RFC text | Producer or absence |
|----|----------|---------------------|
| RFC7950-6.4-1 | An implementation is not required to implement an XPath interpreter but MUST ensure that the requirements encoded in the data model are enforced. (§6.4) | internal/component/config/yang/validator.go::(*Validator).walkTree, L |
| RFC7950-6.4-2 | The XPath expressions MUST be syntactically correct, and all prefixes used MUST be present in the XPath context (see Section 6.4.1). (§6.4) | absent; no XPath parser in Ze or goyang, M |
| RFC7950-7.8.3-1 | It takes as an argument a string that contains a space- separated list of schema node identifiers, which MUST be given in the descendant form (see the rule "descendant-schema-nodeid" in Section 14). (§7.8.3) Each such schema node identifier MUST refer to a leaf. (§7.8.3) If one of the referenced leafs represents configuration data, then all of the referenced leafs MUST represent configuration data. (§7.8.3) The "unique" constraint specifies that the combined values of all the leaf instances specified in the argument string, including leafs with default values, MUST be unique within all list entry instances in which all referenced leafs exist or have default values. (§7.8.3) | internal/component/config/yang/validator.go::(*Validator).walkTree; ListNode.Unique is filled by yang_schema.go:840 and read only by internal/component/web/fragment.go:746 for display, M |
| RFC7950-7.9.4-1 | If "mandatory" is "true", at least one node from exactly one of the choice's case branches MUST exist. (§7.9.4) | internal/component/config/yang/validator.go::(*Validator).walkTree reads Mandatory on leafs only; choice cases are flattened, M |
| RFC7950-7.21.5-1 | A leaf that is a list key MUST NOT have a "when" statement. (§7.21.5) If a key leaf is defined in a grouping that is used in a list, the "uses" statement MUST NOT have a "when" statement. (§7.21.5) If the XPath expression references any node that also has associated "when" statements, those "when" expressions MUST be evaluated first. (§7.21.5) There MUST NOT be any circular dependencies among "when" expressions. (§7.21.5) | internal/component/config/yang/validator.go::(*Validator).walkTree evaluates no when, M |
| RFC7950-8.1-1 | o If the constraint is defined on configuration data, it MUST be true in a valid configuration data tree. (§8.1) o If the constraint is defined on state data, it MUST be true in a valid state data tree. (§8.1) o If the constraint is defined on notification content, it MUST be true in any notification data tree. (§8.1) o If the constraint is defined on RPC or action input parameters, it MUST be true in an invocation of the RPC or action operation. (§8.1) o If the constraint is defined on RPC or action output parameters, it MUST be true in the RPC or action reply. (§8.1) | composite: the constraints of §8 on configuration data (when, unique, choice) are enforced by nobody (see 8.1-4); Ze keeps no state, notification or RPC data tree, L |
| RFC7950-7.6.1-2 | Otherwise, if this ancestor is a case node, the default value MUST be used if any node from the case exists in the data tree or the case node is the choice's default case, and if no nodes from any other case exist in the data tree. (§7.6.1) | internal/component/config/yang_schema.go::flattenChoiceCases drops the case node; internal/component/config/schema_defaults.go::ApplyDefaults then fills every case's defaults, M |
| RFC7950-7.6.1-4 | In these cases, the default value is said to be in use. Note that if the leaf or any of its ancestors has a "when" condition or "if-feature" expression that evaluates to "false", then the default value is not in use. When the default value is in use, the server MUST operationally behave as if the leaf was present in the data tree with the default value as its value. (§7.6.1) | internal/component/config/schema_defaults.go::ApplyDefaults evaluates no when, M |
| RFC7950-8.1-4 | All key leafs MUST be present for all list entries. (§8.1) o Nodes MUST be present for at most one case branch in all choices. (§8.1) o There MUST be no nodes tagged with "if-feature" present if the "if-feature" expression evaluates to "false" in the server. (§8.1) o There MUST be no nodes tagged with "when" present if the "when" condition evaluates to "false" in the data tree. (§8.1) o All referential integrity constraints defined via the "path" statement MUST be satisfied. (§8.1) o All "unique" constraints on lists MUST be satisfied. o The "mandatory" constraint is enforced for leafs and choices, unless the node or any of its ancestors has a "when" condition or "if-feature" expression that evaluates to "false". (§8.1) | composite: mandatory and min/max-elements are met and tagged (7.6.5-1, 7.7.5-1; the type-constraint bullet is its own row, 8.1-3, met and tagged); key presence is the parser; choice, if-feature, when, path and unique are enforced by nobody (see 6.4-1, 7.8.3-1, 7.9.4-1), L |
| RFC7950-8.3.1-2 | o If data for more than one case branch of a choice is present, the server MUST reply with a "bad-element" <error-tag> in the <rpc-error>. (§8.3.1) o If data for a node tagged with "if-feature" is present and the "if-feature" expression evaluates to "false" in the server, the server MUST reply with an "unknown-element" <error-tag> in the <rpc-error>. (§8.3.1) o If data for a node tagged with "when" is present and the "when" condition evaluates to "false", the server MUST reply with an "unknown-element" <error-tag> in the <rpc-error>. (§8.3.1) | internal/component/config/yang/validator.go::(*Validator).walkTree, M |
| RFC7950-8.3.2-1 | During this processing, the following errors MUST be detected: (§8.3.2) In this case, the server MUST reply with an "unknown-element" <error-tag> in the <rpc-error>. (§8.3.2) | internal/component/config/yang/validator.go::(*Validator).walkTree, M |
| RFC7950-7.6.5-2 | Otherwise, if this ancestor is a case node, the leaf MUST exist if any node from the case exists in the data tree. (§7.6.5) | internal/component/config/yang/validator.go::(*Validator).walkTree looks each child up only in the container's own Dir, where goyang keeps no case leaf, so a mandatory leaf inside a case is never required; moved here from the services child under OWNER RULING 4, 2026-09-30, M |
| RFC7950-7.6.5-3 | Otherwise, the leaf MUST exist if the ancestor node exists in the data tree. (§7.6.5) | internal/component/config/yang/validator.go::(*Validator).walkTree checks a mandatory leaf only inside containers present in the data, so one below an absent non-presence container is never required; moved here from the services child under OWNER RULING 4, 2026-09-30, M |
| RFC7950-8.3.3-2 | If the datastore is "running" or "startup", these constraints MUST be enforced at the end of the <edit-config> or <copy-config> operation. (§8.3.3) | internal/component/config/loader.go::LoadConfig (<copy-config>) and internal/component/cli/editor_commit.go::(*Editor).validateStagedTree (<edit-config>, the web editor through config/cli.ValidateContent) refuse type and ze:validate violations, but grade a missing mandatory leaf a warning and evaluate no when, unique or choice; moved here from the services child under OWNER RULING 4, 2026-09-30, M |

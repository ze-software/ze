# Spec: config yang accepted input correctness

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P1 |
| Release reason | Deferring full YANG coverage does not permit silently wrong accepted configuration; separate shipped reference/action correctness from full YANG expansion. |
| Scope | config |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-13 |

## Task

Complete review round 10 of a639bb22e9 for already accepted grouped actions and validate the traffic-control default-class reference path. Determine whether the 2026-10-21 action-without-input augment refusal and silently ignored uses augment are reachable through accepted release schema/config. Repair and prove reachable accepted-config failures; explicitly retain unsupported-form expansion with the post-release full-YANG parent. The journal reports are evidence leads, not proof every shipped module uses them.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

Also repair the already accepted operational transaction declarations entering
editable configuration through `schemaFromResolved` and `yangToNode`.
Commit `857da9e798c` added `config false` request containers to
`internal/plugins/cos/yang/ze-cos-conf.yang` and
`internal/component/l2tp/plugins/shaper/yang/ze-l2tp-shaper-conf.yang`.
Their registered modules reach `config.YANGSchema`, whose configuration
projection imports those containers; the web root navigation then includes
`/show/request/`. Main observed the resulting
`TestWebHandlerGoldenOutput/post-cli-mode` and
`TestWebHandlerGoldenOutput/get-show-finder` fixture failures. Configuration
mutation acceptance has not been executed and is not claimed by this finding.
RFC 7950 section 7.21.1 states, at `rfc/full/rfc7950.txt`: "Data nodes
representing state data are not part of configuration datastores."

This repair is confined to the accepted operational/configuration boundary,
not new YANG forms or full-YANG expansion. Preserve the hidden CoS and shaper
transaction routes registered by `commandDecls` and the AS112 synthetic
listener anchors consumed by `DiscoverListenerServices`. Blanket removal of
every read-only entry from `yangToNode` would remove those listener anchors;
refreshing the two web fixtures would approve the incorrect navigation.

Also distinguish literal numeric `#N` endings in accepted list keys from
generated duplicate-entry suffixes before serialization. Source inspection
found that `serialize_set.go` `serializeSetList` calls `list_key.go`
`setFormEntry` and `ListKeyTokens`, which remove a numeric suffix through
`serialize.go` `StripListKeySuffix`. Thus set output shortens a literal
single key `edge#3` or a final composite-key value `beta#3`, although
`Parser.addParsedListEntry` accepts those string values and `Tree.toMap`
retains their identities. `WritePositionalListEntry`, `WriteInlineListEntry`
and set inline-list writers also call that normalizer. Ordinary named
single-key brace output already retains the key. This finding is
source-traced, not runtime reproduced; the enum-refusal required-path tests
exercise required Path/SetHint and raw dispatch, not serializer round trips.

## Acceptance Criteria

- AC-8: traffic-control `default-class` is a leafref to `../class/name`; `ze config validate` refuses a config naming an absent class with the path and the fix, and accepts one naming a present class (functional test).

- AC-10: the remaining review of the landed grouped-action fix records current producers and evidence for accepted forms.
- AC-11: each remaining goyang observation is classified against a concrete release input; a reachable accepted construct is enforced rather than silently ignored.
- AC-12: the existing accepted reference forms retain the checks described by historical AC-1..7; no full-YANG support or RFC enrollment is claimed from this limited repair.
- AC-13: the accepted CoS and shaper operational transaction declarations do not become editable configuration nodes or `/show/request/` navigation. Prove rejection of their configuration mutation paths and retention of ordinary configuration roots, hidden transaction dispatch and AS112 listener-conflict detection. Resolve the two observed root-navigation handler golden failures at the producing boundary without refreshing fixtures to accept the operational root; retain their rendering assertions.
- AC-14: serialization and replay preserve accepted literal single-key `edge#3` and composite-key final value `beta#3` identities through set and set-meta output, and preserve literal suffixes in affected brace/inline writers. Distinguish literal key data from generated duplicate identity before writing; retain anonymous entries and existing generated `#N` duplicate controls. Prove each affected writer with native round-trip tests and unchanged ordinary named brace output; blanket removal of suffix stripping does not satisfy this criterion. The finding is currently source-traced, with no serializer runtime proof.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `operational-schema-enters-configuration.md` | 2026-11-12 | CoS/shaper operational request roots enter editable schema |
| `format-cannot-hold-a-state-the-tree-holds.md` | 2026-11-13 | literal numeric suffix in serialized list keys |

## Implementation Ownership

| Scope | Current owner |
|---|---|
| Full YANG coverage, new unsupported forms, whole-RFC judgment/enrollment; historical AC-1..7,9 beyond the accepted release surface. | `plan/post-release/features/spec-config-yang-reference-types.md` |
| Accepted reference/action correctness under AC-8, operational/configuration projection under AC-13, literal-key serialization under AC-14, remaining landed-fix review and applicability/repair of the two goyang journal rows. | `plan/release/fixes/spec-config-yang-accepted-input-correctness.md` |

## Files to Modify

- `internal/component/config/yang/loader_grammar.go`, `internal/component/config/yang/loader_structure.go`, `internal/component/config/yang/loader.go`, `internal/component/config/yang/loader_reference.go` - accepted schema/action loading and reachable augment handling only.
- `internal/component/config/yang/reference_path.go`, `internal/component/config/yang/validator_reference.go`, `internal/component/config/yang/validator.go`, `internal/component/config/yang/validator_lexical.go` - preserve enforcement of already accepted reference forms.
- `internal/component/config/validate_sections.go`, `internal/component/config/yang_schema.go`, `internal/component/config/yang/module_default.go` - accepted-config validation and schema integration where the applicability review identifies a repair.
- `internal/component/traffic/yang/ze-traffic-control-conf.yang` - AC-8 default-class reference.
- `internal/component/config/list_key.go`, `internal/component/config/serialize.go`, `internal/component/config/serialize_set.go` - distinguish literal numeric key suffixes from generated duplicate identities in affected writers under AC-14; preserve existing anonymous and duplicate forms.

These paths bound the accepted-input repair and review, not a claim that every
file needs an edit. Unsupported-form expansion and full-YANG coverage stay
with the feature sibling.

## Work Not Done

| Scope | Current owner |
|---|---|
| Full YANG coverage, new unsupported forms, whole-RFC judgment/enrollment; historical AC-1..7,9 beyond the accepted release surface. | `plan/post-release/features/spec-config-yang-reference-types.md` |

## Provenance and retained evidence

Split from `plan/post-release/features/spec-config-yang-reference-types.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/post-release/features/spec-config-yang-reference-types.md`. This child claims no new test result or completion.

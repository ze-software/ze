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
| Updated | 2026-10-22 |

## Task

Complete review round 10 of a639bb22e9 for already accepted grouped actions and validate the traffic-control default-class reference path. Determine whether the 2026-10-21 action-without-input augment refusal and silently ignored uses augment are reachable through accepted release schema/config. Repair and prove reachable accepted-config failures; explicitly retain unsupported-form expansion with the post-release full-YANG parent. The journal reports are evidence leads, not proof every shipped module uses them.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

- AC-8: traffic-control `default-class` is a leafref to `../class/name`; `ze config validate` refuses a config naming an absent class with the path and the fix, and accepts one naming a present class (functional test).

- AC-10: the remaining review of the landed grouped-action fix records current producers and evidence for accepted forms.
- AC-11: each remaining goyang observation is classified against a concrete release input; a reachable accepted construct is enforced rather than silently ignored.
- AC-12: the existing accepted reference forms retain the checks described by historical AC-1..7; no full-YANG support or RFC enrollment is claimed from this limited repair.

## Implementation Ownership

| Scope | Current owner |
|---|---|
| Full YANG coverage, new unsupported forms, whole-RFC judgment/enrollment; historical AC-1..7,9 beyond the accepted release surface. | `plan/post-release/features/spec-config-yang-reference-types.md` |
| Accepted reference/action correctness, AC-8, remaining landed-fix review and applicability/repair of the two goyang journal rows. | `plan/release/fixes/spec-config-yang-accepted-input-correctness.md` |

## Files to Modify

- `internal/component/config/yang/loader_grammar.go`, `internal/component/config/yang/loader_structure.go`, `internal/component/config/yang/loader.go`, `internal/component/config/yang/loader_reference.go` - accepted schema/action loading and reachable augment handling only.
- `internal/component/config/yang/reference_path.go`, `internal/component/config/yang/validator_reference.go`, `internal/component/config/yang/validator.go`, `internal/component/config/yang/validator_lexical.go` - preserve enforcement of already accepted reference forms.
- `internal/component/config/validate_sections.go`, `internal/component/config/yang_schema.go`, `internal/component/config/yang/module_default.go` - accepted-config validation and schema integration where the applicability review identifies a repair.
- `internal/component/traffic/yang/ze-traffic-control-conf.yang` - AC-8 default-class reference.

These paths bound the accepted-input repair and review, not a claim that every
file needs an edit. Unsupported-form expansion and full-YANG coverage stay
with the feature sibling.

## Work Not Done

| Scope | Current owner |
|---|---|
| Full YANG coverage, new unsupported forms, whole-RFC judgment/enrollment; historical AC-1..7,9 beyond the accepted release surface. | `plan/post-release/features/spec-config-yang-reference-types.md` |

## Provenance and retained evidence

Split from `plan/post-release/features/spec-config-yang-reference-types.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/post-release/features/spec-config-yang-reference-types.md`. This child claims no new test result or completion.

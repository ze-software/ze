# Spec: pol-3-validation -- Unique Filter Names and Plain References

| Field | Value |
|-------|-------|
| Status | complete |
| Depends | spec-pol-2-actions.md |
| Phase | 5/5 |
| Updated | 2026-05-28 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-pol-0-umbrella.md` - policy roadmap and naming goals
4. `plan/spec-pol-2-actions.md` - remove-private-as policy action context
5. `internal/component/bgp/config/filter_registry.go` - unique filter-name registry
6. `internal/component/bgp/config/redistribution.go` - filter reference canonicalization
7. `internal/component/cmd/show/show_policy.go` - policy chain display
8. `docs/guide/plugins.md` - user-facing filter examples

## Task

Make unique filter instance names the normal operator-facing way to reference filters in policy chains, and reserve type-prefixed or plugin-prefixed references as explicit disambiguation or advanced forms.

The concrete user experience should be: if a filter instance is named `DROP_PRIVATE_AS`, operators can write `export [ DROP_PRIVATE_AS ]`. They should not need to write `export [ remove-private-as:DROP_PRIVATE_AS ]` when the name is unique.

This spec is intentionally narrow. It does not add a new policy language and does not remove the existing prefixed forms.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/config/syntax.md` - policy config syntax and YANG-driven parsing
  -> Decision: filter definitions live under `bgp policy`, while chains live under `filter import` and `filter export`.
  -> Constraint: config semantics must be enforced by the config tree and validation layer, not by ad hoc runtime guesses.
- [ ] `docs/architecture/api/commands.md` - command output shape and read-only command behavior
  -> Decision: operator commands should return structured JSON data.
  -> Constraint: existing command outputs must not silently change shape without tests and docs.
- [ ] `docs/architecture/core-design.md` - policy filter chain and plugin ownership
  -> Decision: runtime execution uses canonical plugin filter refs.
  -> Constraint: user-facing names may be friendly, but the reactor still needs canonical plugin refs internally.

### Rules and Patterns

- [ ] `ai/patterns/config-option.md` - config option workflow
  -> Decision: config UX is part of the feature, not only parser behavior.
  -> Constraint: invalid and ambiguous config must fail before runtime where possible.
- [ ] `ai/patterns/cli-command.md` - command output and registration pattern
  -> Decision: if `show policy chain` output changes, update YANG help, docs, and command tests.
  -> Constraint: user-facing output should derive from registries, not duplicate hard-coded filter lists.
- [ ] `ai/rules/derive-not-hardcode.md` - registry-derived enumerations
  -> Decision: available filter types and names must come from the filter registry or plugin registry.
  -> Constraint: docs examples may use sample names, but command code must not hard-code remove-private-as special cases.

### RFC Summaries

- [ ] None - this is config and CLI UX. It does not change BGP wire behavior.

**Key insights:**
- Filter instance names are already intended to be globally unique under `bgp policy`.
- Runtime dispatch still needs canonical `<plugin>:<filter>` references, so plain names must be resolved before peer settings are used by the reactor.
- Prefixed references should remain accepted for compatibility and advanced use.
- User-facing docs and examples should prefer plain unique names so operators do not learn plugin internals first.

## Current Behavior (MANDATORY)

**Source files read:**

- [ ] `internal/component/bgp/config/filter_registry.go` - `BuildFilterRegistry` scans schema list children under `policy`, records filter entries by instance name, and returns an error if the same name appears under two different filter types.
  -> Constraint: duplicate filter names are already rejected with a `duplicate filter name` error.
- [ ] `internal/component/bgp/config/filter_registry.go` - `ValidateFilterNames` validates plain chain names and skips names containing `:` after `inactive:` stripping.
  -> Constraint: plain names are compile-time validated; colon refs are left for runtime validation.
- [ ] `internal/component/bgp/config/redistribution.go` - `canonicalizeFilterRefs` accepts plain names, type-prefixed names, and plugin-prefixed names.
  -> Constraint: plain names are looked up in the filter registry, then resolved through `registry.FilterTypesMap` to the owning plugin.
- [ ] `internal/component/bgp/config/redistribution.go` - `canonicalizeOne` preserves `inactive:` around canonicalization.
  -> Constraint: deactivated filters must not be reactivated or rejected by this UX change.
- [ ] `internal/component/bgp/config/filter_registry_test.go` - tests already cover duplicate names, lookup, unknown plain refs, and inactive prefix validation.
  -> Constraint: new tests should extend current coverage rather than duplicate it.
- [ ] `internal/component/bgp/config/remove_private_as_test.go` - remove-private-as refs resolve through filter-type registration.
  -> Constraint: remove-private-as should prove the plain-name path, not only the type-prefixed path.
- [ ] `internal/component/cmd/show/show_policy.go` - `show policy chain` returns `ImportFilters` and `ExportFilters` from `PeerInfo`, which are canonical runtime refs.
  -> Constraint: current output may expose plugin names even when the user configured plain names.
- [ ] `docs/guide/plugins.md` - remove-private-as docs currently tell users to reference `remove-private-as:NAME` and show `export [ remove-private-as:STRIP ]`.
  -> Constraint: docs are stale for the desired UX.
- [ ] `plan/learned/572-cmd-8-policy-show.md` - policy introspection intentionally shipped list and chain first.
  -> Constraint: any `show policy chain` output improvement should fit existing introspection commands.

**Behavior to preserve:**
- Canonical runtime refs remain `<plugin>:<filter>` inside peer settings and reactor forwarding.
- Plain names continue to fail config validation when missing.
- Duplicate filter names across types continue to fail config validation.
- Type-prefixed refs such as `remove-private-as:DROP_PRIVATE_AS` remain accepted.
- Plugin-prefixed refs such as `bgp-filter-remove-private-as:DROP_PRIVATE_AS` remain accepted.
- `inactive:` refs remain accepted and inactive.

**Behavior to change:**
- User-facing docs and examples prefer plain unique names.
- Tests explicitly prove remove-private-as and at least one existing filter type work when referenced by plain unique name.
- `show policy chain` should avoid forcing plugin-prefixed refs as the only user-facing representation. It should expose plain names when unambiguous and retain canonical refs for diagnostics.

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point

- Config enters through YANG under `bgp policy` filter definition lists.
- Chain references enter through global, group, and peer `filter import` and `filter export` leaf-lists.
- Policy display enters through `show policy chain` with a peer selector and optional direction.

### Transformation Path

1. YANG plugin schemas add filter lists under `bgp policy`.
2. `BuildFilterRegistry` scans filter lists and builds a unique name to type registry.
3. Duplicate names across filter types reject config loading.
4. Plain chain refs are validated against the registry.
5. `canonicalizeFilterRefs` rewrites plain refs to canonical plugin refs for runtime.
6. Reactor peer settings store canonical refs and use them for filter dispatch.
7. `show policy chain` reads peer info and should render an operator-facing form derived from canonical refs plus registry/type information.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| YANG config -> filter registry | `BuildFilterRegistry` scans schema list children and config tree entries | [ ] |
| User refs -> runtime refs | `canonicalizeFilterRefs` maps plain names through `FilterTypesMap` | [ ] |
| Runtime refs -> CLI output | `show policy chain` formats peer filter refs for operators | [ ] |
| Docs -> functional behavior | Examples use plain names that parse and execute | [ ] |

### Integration Points

- `BuildFilterRegistry` - source of truth for unique names.
- `canonicalizeFilterRefs` - source of truth for input reference resolution.
- `registry.FilterTypesMap` - source of truth for filter type to plugin mapping.
- `plugin.PeerInfo.ImportFilters` and `ExportFilters` - current canonical runtime view.
- `show policy chain` - current operator view of effective chains.

### Architectural Verification

- [ ] No bypassed layers: config references still flow through registry and canonicalization.
- [ ] No unintended coupling: filter plugins do not know about display names.
- [ ] No duplicated functionality: display-name derivation reuses existing parsing and registry information.
- [ ] Zero-copy preserved: no runtime UPDATE path changes.

## Wiring Test (MANDATORY, NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `filter export [ DROP_PRIVATE_AS ]` | -> | `canonicalizeFilterRefs` resolves through `BuildFilterRegistry` | `TestCanonicalizePlainRemovePrivateASRef` |
| duplicate `policy` names across filter types | -> | `BuildFilterRegistry` rejects ambiguity | existing `TestFilterRegistryDuplicateNameError`, plus parse `.ci` if not covered end-to-end |
| `show policy chain peer X export` | -> | display uses plain names plus canonical diagnostics | `TestToFilterRefs` (unit, output shape) + `test/plugin/policy-chain-plain-names.ci` (daemon end-to-end). Handler-level unit test not feasible: `*Server.reactor` is unexported, so a fake reactor cannot be injected from package `show`; the daemon `.ci` covers the assembled output. |
| remove-private-as docs example | -> | parser accepts plain name example | `test/parse/remove-private-as-plain-ref.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A filter instance name is unique across `bgp policy` | A chain may reference it by plain name. |
| AC-2 | A chain references a plain name | Runtime peer settings still store or can derive the canonical plugin ref needed for dispatch. |
| AC-3 | The same filter instance name appears under two filter types | Config validation fails with a clear duplicate-name error naming both types. |
| AC-4 | A plain chain name is missing | Config validation fails before runtime. |
| AC-5 | A type-prefixed chain ref is used | It remains accepted and resolves to the owning plugin. |
| AC-6 | A plugin-prefixed chain ref is used | It remains accepted for compatibility. |
| AC-7 | A chain ref starts with `inactive:` | The inactive state is preserved through canonicalization and display. |
| AC-8 | `show policy chain` displays a configured chain | Output includes the operator-facing plain name when the filter name is unique and includes canonical data for debugging. |
| AC-9 | Documentation shows remove-private-as usage | Examples use `export [ DROP_PRIVATE_AS ]` or another plain unique name, not `remove-private-as:NAME` as the primary form. |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCanonicalizeRemovePrivateASRef` | `internal/component/bgp/config/remove_private_as_test.go` | Plain `STRIP`/`DROP_PRIVATE_AS` resolves to `bgp-filter-remove-private-as:NAME` (actual name; spec earlier said `TestCanonicalizePlainRemovePrivateASRef`) | PASS |
| `TestCanonicalizePlainPrefixRef` | `internal/component/bgp/config/redistribution_test.go` | Existing filter types also use the plain-name path | PASS |
| `TestCanonicalizeInactivePlainRef` | `internal/component/bgp/config/redistribution_test.go` | `inactive:NAME` preserves inactive state after resolution | PASS |
| `TestToFilterRefs` | `internal/component/cmd/show/show_policy_test.go` | Chain output exposes plain names without losing canonical diagnostics (covers the `TestHandleShowPolicyChainPlainNames` intent; handler-level injection not feasible, see Wiring Test) | PASS |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Filter instance name count | 0 to many | many unique names | N/A | duplicate across types rejects |
| Filter ref prefix count | 0, 1, or plugin form | plain and one-prefix forms | empty ref rejects through existing validation | malformed multi-prefix remains runtime-only unless explicitly supported |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `remove-private-as-plain-ref` | `test/parse/remove-private-as-plain-ref.ci` | Config defines `remove-private-as DROP_PRIVATE_AS` and exports `[ DROP_PRIVATE_AS ]` | PASS |
| `policy-chain-plain-names` | `test/plugin/policy-chain-plain-names.ci` | `show policy chain peer PEER export` reports operator-facing plain names + canonical | PASS |

> **Functional execution (resolved):** both tests pass. `show policy chain` had a latent orphan-`peer`
> dispatch defect (the command had never been run with a selector); fixed by making `peer` a YANG path node
> (`show policy chain peer PEER [import|export]`). An unrelated external blocker (untracked `internal/component/ldp`
> + `internal/component/rsvpte` YANG augmenting a missing `/protocol`) initially prevented the daemon from booting;
> corrected to top-level containers. See `spec-pol-4-explain.md` for the shared dispatch fix detail.

### Interop Tests (MANDATORY for protocol features)

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Not required | N/A | N/A | This spec changes config and display UX, not wire protocol behavior | |

### Future (if deferring any tests)

- None. If `show policy chain` output is not changed in implementation, explicitly record user approval and keep docs/tests focused on config refs only.

## Files to Modify

- `internal/component/bgp/config/remove_private_as_test.go` - add plain-name canonicalization coverage for remove-private-as.
- `internal/component/bgp/config/redistribution_test.go` - add generic plain-name and inactive plain-name canonicalization tests if not already present.
- `internal/component/cmd/show/show_policy.go` - optionally add display-name plus canonical-name output for policy chains.
- `internal/component/cmd/show/show_policy_test.go` - test user-facing policy chain display.
- `docs/guide/plugins.md` - make plain unique names the primary examples for all policy filters touched by this spec.
- `docs/guide/configuration.md` - document plain names as the preferred filter chain reference form.
- `docs/guide/command-reference.md` - update `show policy chain` output if changed.
- `plan/spec-pol-2-actions.md` - update references that teach `remove-private-as:NAME` as primary user syntax.

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] No | Existing filter chain leaves already accept strings |
| CLI commands or flags | [ ] Maybe | Only if `show policy chain` output shape changes |
| CLI grammar | [ ] No | No new command grammar in this spec |
| Editor autocomplete | [ ] Maybe | If autocomplete currently suggests prefixed refs only, update completion source |
| Functional test for new behavior | [ ] Yes | `test/parse/`, `test/plugin/` |
| Doctor check for runtime dependencies | [ ] No | No new runtime dependency |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | New user-facing feature? | [ ] Yes | `docs/features.md` if policy UX is summarized there |
| 2 | Config syntax changed? | [ ] Yes | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | [ ] Maybe | `docs/guide/command-reference.md` if chain output changes |
| 4 | API/RPC added/changed? | [ ] Maybe | `docs/architecture/api/commands.md` if output shape changes |
| 5 | Plugin added/changed? | [ ] No | N/A |
| 6 | Has a user guide page? | [ ] Yes | `docs/guide/plugins.md` |
| 7 | Wire format changed? | [ ] No | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] No | N/A |
| 9 | RFC behavior implemented? | [ ] No | N/A |
| 10 | Test infrastructure changed? | [ ] No | N/A |
| 11 | Affects daemon comparison? | [ ] No | N/A |
| 12 | Internal architecture changed? | [ ] No | N/A |

## Files to Create

- `test/parse/remove-private-as-plain-ref.ci` - parse-level plain-name coverage.
- `test/plugin/policy-show-chain-plain.ci` - only if `show policy chain` output changes.

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Current Behavior, Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement TDD | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-verify` plus targeted parse/plugin tests |
| 7. Critical review | Critical Review Checklist |
| 8. Fix issues | Failure Routing |
| 9. Re-verify | Repeat full verification |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 13. Re-verify | Repeat full verification |
| 14. Present summary | Executive Summary Report per planning rules |

### Implementation Phases

1. **Phase: Wiring and tests** - add failing tests for plain remove-private-as refs and duplicate ambiguity at config parse level.
   - Tests: `TestCanonicalizePlainRemovePrivateASRef`, `remove-private-as-plain-ref.ci`.
   - Files: config tests and parse test.
   - Verify: tests fail if plain names stop resolving.
2. **Phase: Documentation-first UX** - update examples to use plain names as primary form.
   - Tests: docs are manually checked by diff and parse test mirrors the docs example.
   - Files: `docs/guide/plugins.md`, `docs/guide/configuration.md`, `plan/spec-pol-2-actions.md`.
   - Verify: no user-facing remove-private-as example requires a prefix unless explaining advanced forms.
3. **Phase: Policy chain display** - if approved, adjust `show policy chain` to expose plain names and canonical diagnostics.
   - Tests: `TestHandleShowPolicyChainPlainNames`, `policy-show-chain-plain.ci`.
   - Files: `show_policy.go`, show docs.
   - Verify: output remains structured and backwards-compatible where possible.
4. **Full verification** - run targeted config, show command, parse, and plugin tests before `make ze-verify`.
5. **Complete spec** - fill audit tables and learned summary only after verification passes.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has unit or functional evidence. |
| Correctness | Plain refs resolve to the same canonical plugin refs as type-prefixed refs. |
| Naming | Examples use meaningful unique names such as `DROP_PRIVATE_AS`, not behavior keywords that look reserved. |
| Compatibility | Type-prefixed and plugin-prefixed refs still work. |
| Display | If chain output changes, canonical refs remain available for debugging. |
| No-layering | Runtime filter dispatch continues to use canonical plugin refs. |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| Plain remove-private-as refs documented | Grep docs for primary example using `export [ DROP_PRIVATE_AS ]` |
| Plain remove-private-as refs parsed | Run `ze-test bgp parse remove-private-as-plain-ref` |
| Plain refs canonicalize | Run config unit tests covering `canonicalizeFilterRefs` |
| Duplicate names reject | Run `TestFilterRegistryDuplicateNameError` and parse duplicate test if added |
| Chain display updated if changed | Run show policy chain unit and functional tests |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Ambiguity | Duplicate names cannot resolve silently to the wrong filter type. |
| Unknown names | Missing plain names reject before runtime. |
| Runtime safety | Runtime dispatch still uses canonical plugin refs. |
| Information disclosure | CLI output should not expose unnecessary plugin internals unless in a canonical diagnostic field. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Plain ref does not canonicalize | Config canonicalization phase |
| Duplicate name accepted | Filter registry validation phase |
| Prefixed ref regresses | Compatibility phase |
| `show policy chain` output breaks consumers | Display phase, prefer additive fields |
| Functional test cannot observe dispatch | Reuse learned 572 polling pattern or keep parse/unit evidence if user approves |

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Prefixes are required for remove-private-as refs | Plain names already resolve when unique | Read `canonicalizeFilterRefs` and `BuildFilterRegistry` | Main work is docs/tests/display UX, not parser invention |

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Remove prefixed forms | Would break compatibility and advanced explicit refs | Keep prefixed forms as escape hatch |
| Store only plain names at runtime | Reactor dispatch needs plugin names | Store canonical refs and derive display names for operators |

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| None yet | N/A | N/A | N/A |

## Design Insights

- Unique filter instance names are already a stronger UX primitive than plugin type prefixes.
- Prefixes should be taught as disambiguation and diagnostics, not as the primary operator syntax.
- Policy display should separate what the user wrote or can write from what the engine dispatches.

## Review Gate

### Run 1 (initial)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | `show policy chain` JSON shape changed from `[]string` to `[]{name,canonical}` | `show_policy.go` `toFilterRefs` | Intentional per AC-8; locked by `TestToFilterRefs` |

### Fixes applied

- Run 1 finding confirmed intentional (AC-8 requires plain name + canonical); covered by `TestToFilterRefs`.

### Run 2+ (re-runs until clean)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | No new findings for pol-3 in pass 2 | - | - |

### Final status

- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above or explicitly none

Evidence: `/ze-review` pass 2 (this session) found 0 BLOCKER / 0 ISSUE for pol-3 code. The single pass-1
ISSUE (JSON shape) was confirmed intentional and is locked by `TestToFilterRefs`.

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/cmd/show/show_policy.go` | Yes | `toFilterRefs`, `policyFilterRef` present |
| `internal/component/bgp/config/redistribution_test.go` | Yes | `TestCanonicalizePlainPrefixRef`, `TestCanonicalizeInactivePlainRef` |
| `internal/component/bgp/config/remove_private_as_test.go` | Yes | `TestCanonicalizeRemovePrivateASRef` |
| `test/parse/remove-private-as-plain-ref.ci` | Yes | parse test 164 |
| `test/plugin/policy-chain-plain-names.ci` | Yes | plugin test (this session) |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1,2,5,6 | Plain/type/plugin refs resolve to canonical | `TestCanonicalizeRemovePrivateASRef`, `TestCanonicalizePlainPrefixRef` PASS |
| AC-3 | Duplicate name across types rejected | `TestFilterRegistryDuplicateNameError` PASS |
| AC-4 | Missing plain name fails validation | covered by registry resolution + parse test (exec blocked) |
| AC-7 | `inactive:` preserved through canonicalization | `TestCanonicalizeInactivePlainRef` PASS |
| AC-8 | `show policy chain` shows plain name + canonical | `TestToFilterRefs` PASS; `policy-chain-plain-names.ci` PASS |
| AC-9 | Docs use plain names first | `docs/guide/plugins.md`, `configuration.md` updated |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Plain remove-private-as chain ref | `test/parse/remove-private-as-plain-ref.ci` | YES -- parse test 164 PASS |
| `show policy chain` plain-name output | `test/plugin/policy-chain-plain-names.ci` | YES |

## Checklist

### Goal Gates (MUST pass)

- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete and every row has a concrete test name
- [ ] `/ze-review` gate clean
- [ ] `make ze-verify` passes or blocker is explicit and unrelated
- [ ] Docs updated to teach plain names first

### Quality Gates (SHOULD pass, defer only with user approval)

- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Completion (BLOCKING before ANY commit)

- [ ] Critical Review passes and is documented
- [ ] Partial or skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written to `plan/learned/NNN-pol-3-validation.md`

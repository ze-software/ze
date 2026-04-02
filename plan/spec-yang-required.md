# Spec: YANG Required and Suggest Extensions

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/5 |
| Updated | 2026-04-02 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/yang/modules/ze-extensions.yang` - existing extension definitions
4. `internal/component/config/schema.go` - ListNode type (lines 154-162)
5. `internal/component/config/yang_schema.go` - extension parsing (lines 261-520)
6. `internal/component/bgp/config/resolve.go` - 3-layer inheritance resolution
7. `internal/component/web/handler_config.go` - HandleConfigAddForm (lines 389-458)
8. `internal/component/web/templates/component/add_form_overlay.html` - creation form

## Task

Add two new YANG extensions (`ze:required` and `ze:suggest`) that declare which fields must be present in a list entry after config inheritance resolution, and which fields should be shown in the creation dialog.

**Problem:** A peer can be created in the web UI or config file without essential fields (remote IP, local AS, remote AS). The missing fields are only discovered at BGP session startup, not at config validation time. There is no YANG-level mechanism to declare "this field must have a value after inheritance merges bgp/group/peer levels."

**Goal:** Catch incomplete peers at config validation time (commit, CLI, editor) and guide users during creation by showing required/suggested fields with inherited defaults.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - config resolution and inheritance model
  -> Constraint: 3-layer merge (bgp -> group -> peer), deepMergeMaps with cumulative paths
- [ ] `docs/architecture/core-design.md` - overall system architecture
  -> Constraint: config pipeline: File -> Tree -> ResolveBGPTree() -> map[string]any -> PeersFromTree()

### RFC Summaries (MUST for protocol work)
N/A - this is config infrastructure, not protocol work.

**Key insights:**
- Validation must happen post-resolve because required fields can be inherited from group/bgp level
- The web creation form is currently driven solely by `unique` fields via `collectUniqueFields()`
- `unique` is standard YANG (parsed from `entry.Extra`), custom extensions use `entry.Exts`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `ze-extensions.yang` - 12 custom extensions defined (listener, syntax, key-type, route-attributes, allow-unknown-fields, sensitive, validate, command, edit-shortcut, display-key, cumulative, decorate). No required/suggest extensions exist.
- [ ] `schema.go:154-162` - `ListNode` has `Unique [][]string` for YANG unique constraints. No Required or Suggest fields.
- [ ] `yang_schema.go:261-360` - Extension parsing helpers iterate `entry.Exts`, match keyword suffix. Pattern: `for _, ext := range entry.Exts { if strings.HasSuffix(ext.Keyword, ":suffix") { return ext.Argument } }`. `yangToList` (line 461-521) builds `ListNode` including unique constraint extraction.
- [ ] `yang_schema.go:508-517` - Unique parsed from `entry.Extra["unique"]` (standard YANG), not from `entry.Exts` (custom extensions).
- [ ] `resolve.go:22-169` - `ResolveBGPTree(tree *config.Tree)` resolves 3 layers per peer, then calls `checkDuplicateRemoteIPs()`. No required-field validation exists. Signature unchanged -- `CheckRequiredFields` is a separate exported function (see Decision below).
- [ ] `validator.go:167-281` - `validateWithYANG()` (line 169) validates peers and BGP-level fields via YANG. `validatePeer()` (line 237) validates individual peers, includes ad-hoc `session/asn/remote` check at lines 266-280 (warns if missing). This is the check to replace with generic `ze:required`.
- [ ] `reactor/config.go:45-55` - `PeersFromTree()` has a runtime guard for `session > asn > remote`. This is a runtime check, NOT config validation -- it remains unchanged by this spec.
- [ ] `bgp/plugins/cmd/peer/peer.go:304-313` and `save.go:87-89` - CLI "peer with" and peer save operations also check `session/asn/remote`. These are command-level guards -- they remain unchanged.
- [ ] `handler_config.go:389-458` - `HandleConfigAddForm()` receives `*EditorManager` (provides tree access via `mgr.Tree(username)`) and `*config.Schema`. Currently only uses schema: collects `collectUniqueFields()` for form fields, resolves YANG descriptions via `resolveLeafDescription()`. Does not use tree for inheritance resolution yet.
- [ ] `fragment.go:712-724` - `collectUniqueFields()` returns distinct leaf paths from `listNode.Unique`. Used by form (`handler_config.go:445`) and list table (`buildListTable` at line 727). `resolveLeafDescription()` at line 786 resolves YANG descriptions for fields -- reusable for required/suggest fields.
- [ ] `add_form_overlay.html` - Renders key field + unique fields. No visual distinction between field categories. No inheritance-aware defaults.
- [ ] `ze-bgp-conf.yang:70-100` - Peer lists (group peer at line 70, standalone peer at line 87) have `unique "connection/remote/ip"` but no required declarations. `connection/remote/ip` is in the shared `peer-fields` grouping (line 172), NOT added via augment -- it is structurally inheritable from group level (though `unique` constraint makes group-level defaults impractical for multi-peer groups).

**Behavior to preserve:**
- Unique constraint enforcement (both web and resolve.go) unchanged
- 3-layer inheritance resolution logic unchanged
- Creation form still works for lists without required/suggest (backwards compatible)
- Config editing never blocked by required/suggest (validation only at commit/validate time)
- Runtime guards in `reactor/config.go` (PeersFromTree) and `bgp/plugins/cmd/peer/` (CLI commands) remain -- they serve as last-resort runtime checks, independent of config validation

**Behavior to change:**
- Creation form shows required and suggested fields (in addition to unique fields)
- Creation form pre-fills inherited values for required/suggest fields
- Creation form visually distinguishes required vs suggest fields
- Submit button disabled (red) until required fields satisfied
- Post-resolve validation rejects peers missing required fields
- Editor validation warns on missing required fields

## Data Flow (MANDATORY)

### Entry Point
- **YANG schema**: `ze:required` and `ze:suggest` statements on `list` nodes
- **Config file**: peer entries that may be missing required fields
- **Web UI**: creation form for new list entries

### Transformation Path
1. YANG parsing: goyang parses `.yang` files, stores custom extensions in `Entry.Exts`
2. Schema building: `yang_schema.go` extracts `ze:required`/`ze:suggest` from `Entry.Exts`, stores in `ListNode.Required`/`ListNode.Suggest`
3. Form rendering: `HandleConfigAddForm()` collects required + suggest + unique fields, resolves inherited defaults from config tree, passes categorized fields to template
4. Form display: template renders required fields (grouped, colored) and suggest fields (grouped, optional), submit button state tracks required field completion
5. Config resolution: `CheckRequiredFields(schema, peerMap)` called after `ResolveBGPTree()` by callers that need validation (`cmd_validate.go`, `peers.go`)
6. Editor validation: `validatePeer()` checks required fields using `v.schema` with line-number context

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG -> Schema | Extension parsing in yang_schema.go | [ ] |
| Schema -> Web | ListNode.Required/Suggest read by handler | [ ] |
| Config Tree -> Form | Inherited values resolved from tree hierarchy | [ ] |
| Resolved Map -> Validation | CheckRequiredFields(schema, peerMap) called by cmd_validate and peers.go | [ ] |

### Integration Points
- `ListNode` struct - add Required and Suggest fields (same type as Unique)
- `collectUniqueFields()` - expand or complement with new collection function
- `HandleConfigAddForm()` - use existing `mgr.Tree(username)` for inheritance resolution (no new parameter needed)
- `CheckRequiredFields(schema, peerMap)` - separate post-resolve function in `resolve.go`; called by `cmd_validate.go` and `peers.go` (not by `plugins.go`, `cmd_dump.go`, `cmd_diff.go`)
- `validatePeer()` - replace ad-hoc `session/asn/remote` check with generic required field check using `v.schema`

### Architectural Verification
- [ ] No bypassed layers (validation in resolve.go, editor, and web - all three)
- [ ] No unintended coupling (schema storage is generic, BGP peer is just one consumer)
- [ ] No duplicated functionality (extends existing ListNode/unique pattern)
- [ ] Zero-copy preserved where applicable (N/A - config path, not wire path)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate -` with standalone peer missing remote-as | -> | `CheckRequiredFields()` in resolve.go via cmd_validate.go | `test/parse/required-field-missing.ci` |
| `ze config validate -` with group providing local-as | -> | `CheckRequiredFields()` passes (inherited) via cmd_validate.go | `test/parse/required-field-inherited.ci` |
| Web creation form for peer in group | -> | `HandleConfigAddForm()` shows inherited defaults | `TestAddFormShowsInheritedDefaults` |
| Web creation form submit without required field | -> | `HandleConfigAdd()` rejects | `TestAddFormRejectsIncompleteRequired` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | YANG schema with `ze:required "session/asn/remote"` on peer list | `ListNode.Required` contains `["session", "asn", "remote"]` after schema build |
| AC-2 | YANG schema with `ze:suggest "connection/local/ip"` on peer list | `ListNode.Suggest` contains `["connection", "local", "ip"]` after schema build |
| AC-3 | Config with peer missing `session/asn/remote`, no group provides it | `ResolveBGPTree()` returns error naming the peer and missing field |
| AC-4 | Config with peer missing `session/asn/local`, but bgp-level provides it | `ResolveBGPTree()` succeeds (inherited value satisfies required) |
| AC-5 | Config with standalone peer missing `connection/remote/ip`, no group to inherit from | `ResolveBGPTree()` returns error naming the peer and missing field |
| AC-6 | Web creation form for peer in group where group sets remote-as | Form shows `session/asn/remote` field pre-filled with group's value |
| AC-7 | Web creation form for standalone peer (no group, bgp has local-as) | Form shows `session/asn/local` field pre-filled with bgp-level value |
| AC-8 | Web creation form: required fields visually distinct from suggest fields | Required fields have different color/grouping than suggest fields |
| AC-9 | Web creation form: submit button disabled when required field empty and no inherited value | Button is red/disabled; becomes active when field filled or inherited value exists |
| AC-10 | Editor validates config: peer missing required field post-resolve | Validation error with line number and field name |
| AC-11 | List without `ze:required` or `ze:suggest` | Creation form unchanged (backwards compatible, shows only key + unique) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestListNodeRequiredParsing` | `internal/component/config/yang_schema_test.go` | AC-1: ze:required parsed into ListNode.Required | |
| `TestListNodeSuggestParsing` | `internal/component/config/yang_schema_test.go` | AC-2: ze:suggest parsed into ListNode.Suggest | |
| `TestResolveBGPTree_MissingRequiredField` | `internal/component/bgp/config/resolve_test.go` | AC-3: error on missing required field | |
| `TestResolveBGPTree_RequiredFieldInherited` | `internal/component/bgp/config/resolve_test.go` | AC-4: inherited field satisfies required | |
| `TestResolveBGPTree_MissingRemoteIP` | `internal/component/bgp/config/resolve_test.go` | AC-5: error on standalone peer missing remote IP | |
| `TestCollectFormFields_RequiredAndSuggest` | `internal/component/web/fragment_test.go` | Required + suggest + unique fields collected and categorized | |
| `TestResolveInheritedDefaults_Group` | `internal/component/web/handler_config_test.go` | AC-6: group-level values resolved for form | |
| `TestResolveInheritedDefaults_BGPLevel` | `internal/component/web/handler_config_test.go` | AC-7: bgp-level values resolved for form | |
| `TestAddFormRejectsIncompleteRequired` | `internal/component/web/handler_config_test.go` | AC-9: POST rejected when required field missing | |
| `TestValidatePeer_MissingRequiredField` | `internal/component/cli/validator_test.go` | AC-10: editor validation error | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A - no numeric inputs in this feature. Fields are string paths and string values.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `required-field-missing` | `test/parse/required-field-missing.ci` | `ze config validate -` with peer missing remote-as, expect exit code 1 + error message naming peer and field | |
| `required-field-inherited` | `test/parse/required-field-inherited.ci` | `ze config validate -` with group providing remote-as, expect exit code 0 | |
| `required-field-all-present` | `test/parse/required-field-all-present.ci` | `ze config validate -` with all required fields present, expect exit code 0 | |

### Future (if deferring any tests)
- Web UI visual tests (color/button state) are not automatable in .ci -- verified by manual inspection during implementation

## Files to Modify
- `internal/component/config/yang/modules/ze-extensions.yang` - add ze:required and ze:suggest extension definitions
- `internal/component/bgp/schema/ze-bgp-conf.yang` - add ze:required/ze:suggest on both peer lists
- `internal/component/config/schema.go` - add Required and Suggest fields to ListNode
- `internal/component/config/yang_schema.go` - parse ze:required/ze:suggest from Entry.Exts
- `internal/component/bgp/config/resolve.go` - add exported `CheckRequiredFields(schema *config.Schema, peerMap map[string]any) error`
- `internal/component/bgp/config/peers.go` - call `CheckRequiredFields(schema, bgpTree)` after `ResolveBGPTree` (line 46); schema already loaded at line 39
- `cmd/ze/config/cmd_validate.go` - call `CheckRequiredFields(schema, bgpTree)` after `ResolveBGPTree` (line 206); schema already available
- `internal/component/cli/validator.go` - replace ad-hoc `session/asn/remote` check (lines 266-280) in `validatePeer()` with generic required field check using `v.schema`
- `internal/component/web/fragment.go` - expand or add field collection for required/suggest
- `internal/component/web/handler_config.go` - HandleConfigAddForm: use existing `mgr.Tree(username)` for inherited values, categorize fields; HandleConfigAdd (line 231): enforce required on POST
- `internal/component/web/templates/component/add_form_overlay.html` - visual grouping, colors, button state
- `internal/component/web/assets/style.css` - styles for required/suggest field groups

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new extensions) | Yes | `ze-extensions.yang` |
| CLI commands/flags | No | N/A |
| Editor autocomplete | No | N/A (validation only) |
| Functional test for validation | Yes | `test/parse/required-field-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - add required/suggest field validation |
| 2 | Config syntax changed? | Yes | `docs/architecture/config/syntax.md` - document ze:required and ze:suggest extensions |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | No | N/A |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/syntax.md` - YANG extension reference |

## Files to Create
- `test/parse/required-field-missing.ci` - peer missing required field fails
- `test/parse/required-field-inherited.ci` - inherited required field passes
- `test/parse/required-field-all-present.ci` - all required fields present passes

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Phases 1-5 below |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: YANG + Schema** -- Define extensions and schema storage
   - Add `ze:required` and `ze:suggest` to `ze-extensions.yang`
   - Add `Required [][]string` and `Suggest [][]string` to `ListNode` in `schema.go`
   - Parse extensions in `yang_schema.go` (from `entry.Exts`, not `entry.Extra`)
   - Add `ze:required`/`ze:suggest` statements to peer lists in `ze-bgp-conf.yang`
   - Tests: `TestListNodeRequiredParsing`, `TestListNodeSuggestParsing`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Post-resolve validation** -- Enforce required fields after inheritance
   - Add exported `CheckRequiredFields(schema *config.Schema, peerMap map[string]any) error` in `resolve.go`
   - Function looks up peer list's `ListNode.Required` from schema, walks each resolved peer map, checks each Required path has a non-empty value
   - Wire into `peers.go:PeersFromConfigTree` (after line 46, schema already at line 39) and `cmd_validate.go` (after line 206, schema already available)
   - Tests: `TestResolveBGPTree_MissingRequiredField`, `TestResolveBGPTree_RequiredFieldInherited`, `TestResolveBGPTree_MissingRemoteIP`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Editor validation** -- Required field check in CLI editor
   - Add required field validation in `validatePeer()` in `validator.go`, replacing ad-hoc `session/asn/remote` check (lines 266-280)
   - `ConfigValidator` already has `schema *config.Schema` (line 54) -- use it to look up `ListNode.Required`
   - Merge group defaults before checking (existing `mergeGroupDefaults()`)
   - Tests: `TestValidatePeer_MissingRequiredField`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Web creation form** -- Inheritance-aware form with visual grouping
   - Expand field collection in `fragment.go` to return categorized fields (required/suggest/unique)
   - Update `HandleConfigAddForm()` to use existing `mgr.Tree(username)` for inherited defaults per field
   - Update `HandleConfigAdd()` to enforce required fields on POST (reject if missing and no inherited value)
   - Update `add_form_overlay.html` with visual grouping, colors, conditional submit button
   - Add CSS in `style.css` for required (distinct color/border) vs suggest (separate group, optional appearance)
   - Tests: `TestCollectFormFields_RequiredAndSuggest`, `TestResolveInheritedDefaults_Group`, `TestResolveInheritedDefaults_BGPLevel`, `TestAddFormRejectsIncompleteRequired`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Functional tests + docs**
   - Create `test/parse/required-field-*.ci` functional tests
   - Update `docs/features.md` and `docs/architecture/config/syntax.md`
   - Verify: `make ze-verify`

6. **Complete spec** -- Fill audit tables, write learned summary to `plan/learned/499-yang-required.md`, delete spec from `plan/`.

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Required validation fires post-resolve only (not pre-resolve). Inherited values satisfy required. `CheckRequiredFields` wired into `peers.go` and `cmd_validate.go`. |
| Naming | YANG extensions use `ze:required` and `ze:suggest` (kebab-case). Error messages name the peer and field path. |
| Data flow | Schema -> ListNode -> form/validation. No validation in parser or tree builder. |
| Rule: no-layering | Ad-hoc `session/asn/remote` check in `validator.go:266-280` replaced by generic required check. Runtime guards in `reactor/config.go` and `bgp/plugins/cmd/peer/` remain (different purpose: last-resort runtime validation, not config validation). |
| Backwards compat | Lists without ze:required/ze:suggest unchanged. No new mandatory YANG statements for existing lists. |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `ze:required` extension in ze-extensions.yang | `grep "extension required" ze-extensions.yang` |
| `ze:suggest` extension in ze-extensions.yang | `grep "extension suggest" ze-extensions.yang` |
| `ze:required` on both peer lists in ze-bgp-conf.yang | `grep "ze:required" ze-bgp-conf.yang` |
| `ListNode.Required` field in schema.go | `grep "Required" schema.go` |
| `CheckRequiredFields` in resolve.go | `grep "CheckRequiredFields" resolve.go` |
| `CheckRequiredFields` called in peers.go | `grep "CheckRequiredFields" peers.go` |
| `CheckRequiredFields` called in cmd_validate.go | `grep "CheckRequiredFields" cmd_validate.go` |
| Required field check in validator.go | `grep -i "required" validator.go` |
| Form categories in add_form_overlay.html | `grep "required\|suggest" add_form_overlay.html` |
| CSS for required/suggest fields | `grep "required\|suggest" style.css` |
| Functional test: missing field | `ls test/parse/required-field-missing.ci` |
| Functional test: inherited field | `ls test/parse/required-field-inherited.ci` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Field paths in ze:required/ze:suggest are schema-defined, not user input. No injection risk. |
| Form field values | Inherited defaults come from config tree (trusted). No external input in pre-fill. |
| Error messages | Required field errors name peer and path. No sensitive data exposed. |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| `connection/remote/ip` is peer-only via augment | It is in shared `peer-fields` grouping, inheritable from group level | Read ze-bgp-conf.yang -- only augment is for environment at line 575 | AC-5 rewritten; Peer Field Assignments corrected |
| `ResolveBGPTree` could call `checkRequiredFields` without schema access | `ResolveBGPTree(tree *config.Tree)` has no schema parameter; `YANGSchema()` is not cached | Read resolve.go signature + YANGSchema implementation | Resolved: option B (separate exported function), keeps ResolveBGPTree signature unchanged |
| 10 custom YANG extensions | 12 extensions (missed listener and decorate in count) | Counted ze-extensions.yang definitions | Minor, corrected |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

### Design Decisions from Discussion
- `ze:required` and `ze:suggest` are list-level (like `unique`), not leaf-level. Reason: the inheritance context belongs to the list, not the leaf. A leaf cannot know if it's being used at group level (optional) or peer level (required post-resolve).
- Creation form is inheritance-aware: pre-fills from group/bgp defaults. `HandleConfigAddForm` already receives `*EditorManager` which provides tree access via `mgr.Tree(username)` -- no signature change needed.
- Submit button red/disabled until required fields satisfied (typed or inherited). Visual grouping separates required from suggest.
- `ze:required` forces the field in the creation dialog (must be filled if no inherited value). `ze:suggest` shows the field but allows empty.
- Validation is post-resolve only. Config editing is never blocked.
- -> Decision: `CheckRequiredFields(schema, peerMap)` is a separate exported function in `resolve.go`, NOT inside `ResolveBGPTree`. Reason: `ResolveBGPTree` takes only `*config.Tree` (5 callers); `plugins.go` and `cmd_dump.go`/`cmd_diff.go` don't need field validation. `checkDuplicateRemoteIPs` stays inside (data integrity, no schema needed). Callers that validate: `cmd_validate.go` (has schema), `peers.go` (loads schema at line 39). `ConfigValidator` already has `v.schema` for editor-side validation.

### Peer Field Assignments
| Field path | Extension | Rationale |
|------------|-----------|-----------|
| `connection/remote/ip` | `ze:required` | Unique and essential. In shared `peer-fields` grouping so structurally inheritable, but `unique` constraint makes group-level default impractical for multi-peer groups. |
| `session/asn/local` | `ze:required` | Essential for BGP. Inheritable from bgp-level `session/asn/local`. |
| `session/asn/remote` | `ze:required` | Essential for BGP. Inheritable from group level. |
| `connection/local/ip` | `ze:suggest` | Useful but optional. Can be "auto" or omitted. |

## RFC Documentation

N/A - config infrastructure, not protocol work.

## Implementation Summary

### What Was Implemented
- [To be filled]

### Bugs Found/Fixed
- [To be filled]

### Documentation Updates
- [To be filled]

### Deviations from Plan
- [To be filled]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/499-yang-required.md`
- [ ] Summary included in commit

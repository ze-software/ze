# Spec: unify-tree-deactivation

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-06 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `docs/architecture/config/syntax.md` -- Tree, parse, serialize, prune
4. `internal/component/config/tree.go`, `serialize.go`, `parser.go`, `prune.go`
5. `ai/rules/data-flow-tracing.md`, `ai/rules/no-fabrication.md`

## Task

DESIGN-REVIEW.md finding 2 (row "Tree deactivation"): the config Tree represents "this value is deactivated" two incompatible ways.

- Representation 1 (leaf sibling-map): `Tree.inactiveValues map[string]bool` -- out-of-band, keyed by leaf name, never encoded in the value string (`tree.go:33`).
- Representation 2 (in-band prefix): the literal string `inactive:` glued to the front of a member inside a `multiValues` list entry, e.g. `["inactive:8.8.8.8", "9.9.9.9"]` (`tree.go:621-622`).

Both encode the same concept (a leaf or leaf-list member the operator commented out). Because the two mechanisms are incompatible, the code carries duplicate deactivate/activate verb pairs, duplicate serialization branches, and -- most damaging -- the in-band prefix leaks out of the config package through `ToMap()` into seven BGP config/reactor call sites that string-sniff `inactive:` to decide whether a filter reference is disabled.

Goal: pick one representation, extend it to cover every case the other covers, migrate all producers/consumers onto it, and delete the loser. Preserve every externally observable behavior (on-disk config text, round-trip, filter-chain runtime effect). This is a refactor.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` -- Tree data structure, parse, serialize, prune contract
  → Decision: deactivation is engine state on the parent Tree, not part of the YANG schema; the prune walk drops it and has no schema-walk equivalent (`prune.go:77-80`).
  → Constraint: the serialized on-disk form is `inactive: <leaf> <value>` (hierarchical) or an `inactive`/`nop` set statement -- the raw `inactive:` member prefix MUST NOT reach disk because it fails per-item type validation on reparse (`serialize.go:312-315`, `leaflist_member_test.go:204`).
- [ ] `ai/rules/data-flow-tracing.md` -- trace the deactivation marker from parse to reactor
  → Constraint: a value must flow through the intended path; the in-band prefix currently bypasses the config boundary and surfaces raw in `ToMap()` output (`tree.go:814-820`).
- [ ] `ai/rules/no-fabrication.md` -- cite the producing function for every behavioral claim
  → Constraint: every migration item below names the `file:line` that produces the behavior, read at the producer not the caller.

**Key insights:**
- The out-of-band sibling-map is invisible to `ToMap()` by design and is pruned before it reaches plugins; the in-band prefix is visible to `ToMap()` and is NOT pruned, so it reaches the reactor raw.
- At least one reactor consumer (`peers.go:607-621`) needs "present but disabled" semantics, so the winner cannot simply prune deactivated members away; it must surface the marker structurally.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/config/tree.go` - defines both representations. `inactiveValues` map (line 33) with `SetLeafInactive`/`IsLeafInactive`/`ClearLeafInactive`/`pruneInactiveLeaves` (71-121); `inactiveValuePrefix` const (622) consumed by `AddMultiValueMember` (686), `MultiValueMemberState` (710), `RemoveMultiValueMember` (727), `DeactivateMultiValue` (742), `ActivateMultiValue` (766); `ToMap` (800) emits raw `multiValues` including the prefix.
- [ ] `internal/component/config/parser.go` - hierarchical parse. `inactive:` sugar (73-93), `parseNodeInactive` (110), `parseInactiveValueOrArray` (126) calls `DeactivateMultiValue` (133); `applyInactive` calls `SetLeafInactive` (154-164).
- [ ] `internal/component/config/parser_list.go` - `inactive:` sugar inside list field blocks (92-114).
- [ ] `internal/component/config/setparser.go` - set-format `inactive`/`nop` parse. `SetLeafInactive` (356-364, 468-487) and `DeactivateMultiValue` (370, 484).
- [ ] `internal/component/config/serialize.go` - hierarchical serialize. `inactiveValues` prefix rendering (218, 270, 286, 298, 318); deactivated-member rendering via `splitInactiveMembers` (316); `canInlineContainer` refuses inline when a member carries the prefix (39-53).
- [ ] `internal/component/config/serialize_set.go` - set-format serialize. `splitInactiveMembers` (204) strips the prefix; `emitValueOrArrayNop` (160, 613, 679).
- [ ] `internal/component/config/serialize_blame.go` - annotated/blame serialize; `inactiveValues` (216-602) and `splitInactiveMembers` (257).
- [ ] `internal/component/config/serialize_annotated.go` - column-aware serialize; `splitInactiveMembers` (676).
- [ ] `internal/component/config/prune.go` - `PruneInactive`/`PruneActive` (13-27); `pruneNode` calls `pruneInactiveLeaves` (80). Does NOT strip prefixed `multiValues` members.
- [ ] `internal/component/config/loader.go` - `PruneInactive` runs before env extraction / `ToMap` (128).
- [ ] `internal/component/cli/editor_commands.go` - CLI deactivate/activate verbs: leaf via `SetLeafInactive`/`ClearLeafInactive` (784, 805); member via `DeactivateMultiValue`/`ActivateMultiValue` (721, 742).
- [ ] `internal/component/cli/editor_leaflist.go` - leaf-list member deactivate/activate (152, 156).
- [ ] `internal/component/cli/editor_draft.go` - draft-apply member deactivate/activate (519, 530).
- [ ] `internal/component/bgp/config/peers.go` - reactor consumer: string-sniffs `inactive:` on filter refs to set `LoopDisabled` and dedup default filters (609-610, 681).
- [ ] `internal/component/bgp/config/filter_registry.go` - `TrimPrefix(name, "inactive:")` (86).
- [ ] `internal/component/bgp/config/redistribution.go` - `HasPrefix`/`TrimPrefix` `inactive:` (62-64).
- [ ] `internal/component/bgp/plugins/cmd/policy/handler.go` - `TrimPrefix(c, "inactive:")` (320).
- [ ] `internal/component/bgp/plugins/filter_family/config.go` - `TrimPrefix(ref, "inactive:")` (116).
- [ ] `internal/component/bgp/reactor/filter_chain.go` - skips a filter ref when `HasPrefix(ref, "inactive:")` (182).
- [ ] `internal/component/bgp/reactor/policy_dryrun.go` - `HasPrefix(ref, "inactive:")` (35, 269).

Feature inventory (aspect | leaf sibling-map (rep 1) | multi-value in-band prefix (rep 2)):

| Aspect | Leaf sibling-map `inactiveValues` | In-band prefix `inactive:` |
|--------|-----------------------------------|----------------------------|
| Where stored | Separate `map[string]bool` on the parent Tree, keyed by leaf name (`tree.go:33`) | Inside the `multiValues[name]` element string (`tree.go:622`) |
| Granularity | Whole leaf only -- cannot mark one member of a leaf-list (the GAP) | Individual member of a leaf-list |
| Set / clear | `SetLeafInactive` / `ClearLeafInactive` (`tree.go:71,91`) | `DeactivateMultiValue` / `ActivateMultiValue` (`tree.go:742,766`) |
| Query | `IsLeafInactive` (`tree.go:82`) | `MultiValueMemberState` / `HasMultiValueMember` (`tree.go:703,695`) |
| Serialize to text | `inactive: ` prefix on the statement line (`serialize.go:270`) | bare member in list line + `inactive: <leaf> <member>` follow-up; prefix stripped by `splitInactiveMembers` (`serialize.go:316`, `serialize_set.go:204`) |
| On-disk form | `inactive: router-id 1.2.3.4` | `name-server [ 9.9.9.9 8.8.8.8 ]` then `inactive: name-server 8.8.8.8` -- raw prefix NEVER on disk |
| Tree -> `map[string]any` (`ToMap`) | INVISIBLE -- `ToMap` does not copy `inactiveValues` (`tree.go:800-838`) | VISIBLE and RAW -- `ToMap` emits the `"inactive:8.8.8.8"` string to plugins/reactor (`tree.go:814-820`) |
| Reload round-trip | serialize `inactive:` sugar -> parser `applyInactive` -> `SetLeafInactive` (`parser.go:154`) | serialize member statement -> parser `parseInactiveValueOrArray` -> `DeactivateMultiValue` (`parser.go:133`) |
| Prune behavior | `pruneInactiveLeaves` deletes the whole leaf before `ToMap` (`tree.go:101`, `prune.go:80`) | NOT pruned -- deactivated member survives with prefix into `ToMap` |
| Diff / equality | Only a test helper reads it (`serialize_test.go:359`); production diff is text-based (`SerializeSetWithMeta`) | Same -- production diff compares serialized text |
| Clone | Copied (`tree.go:199`) | Copied as part of `multiValues` slice (`tree.go:192-196`) |
| Collision risk | NONE -- out-of-band, value string untouched | YES -- a legitimate leaf-list value beginning with `inactive:` is misread as deactivated; any `Get(name)` reader sees `inactive:8.8.8.8` as a real value |
| Cross-boundary leak | NONE -- pruned away before the config boundary | 7 BGP config/reactor call sites string-sniff `inactive:` (see Source files) |

**Behavior to preserve:**
- On-disk config text is byte-identical before and after: `inactive: <leaf> <value>` for whole-leaf deactivation and `inactive: <leaf> <member>` follow-up statements for member deactivation (hierarchical), and the equivalent `inactive`/`nop` set statements (`serialize_set.go`). The raw `inactive:` prefix must still never appear in serialized output (`leaflist_member_test.go:204,241`).
- Round-trip stability: parse -> serialize -> parse yields the same Tree deactivation state, both whole-leaf and per-member (`parse-inactive-leaflist-member.ci`, `TestSetFormatDeactivatedMemberRoundTrip`, `TestHierarchicalSingleDeactivatedMemberRoundTrip`).
- Runtime effect in the reactor: a deactivated filter reference is not executed (`filter_chain.go:182`), and a deactivated loop-detection filter still suppresses the default loop ingress via `LoopDisabled` (`peers.go:616-621`) -- the "present but disabled" case.
- Prune semantics: whole-leaf deactivation removes the leaf before `ToMap` (`prune.go:80`); `PruneActive` keeps only inactive nodes.
- CLI deactivate/activate verbs behave identically for leaves and leaf-list members (`editor_commands.go`, `cli-config-deactivate-leaf.ci`, `leaflist-insert-deactivate.et`).
- `set`/`add-member` idempotency: setting a member already present in deactivated form is a no-op and does not reactivate it (`AddMultiValueMember`, `tree.go:677-691`).

**Behavior to change:**
- None -- internal refactor, behavior preserved. The only change is the internal representation of per-member deactivation (out-of-band structure instead of an in-band string prefix) and the accessor the seven BGP consumers use to detect a disabled reference (a structural query instead of `strings.HasPrefix(ref, "inactive:")`).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
Deactivation enters the Tree three ways, all landing in the config package:
1. File parse: `inactive: <name> ...` sugar in hierarchical config (`parser.go:73`) or an `inactive <path>` / `nop` line in set-format (`setparser.go:432`).
2. CLI edit: the `deactivate` verb in the config editor (`editor_commands.go`, `editor_leaflist.go`).
3. Draft apply on commit (`editor_draft.go:519`).

Format at entry: a leaf name plus optional member token. Today the parser routes leaf cases to `inactiveValues` and single-member leaf-list cases to the `inactive:` prefix.

### Transformation Path
1. Parse or edit sets the marker: `SetLeafInactive` for whole leaves (`tree.go:71`); `DeactivateMultiValue` rewrites `multiValues[name][i]` to `"inactive:"+value` for members (`tree.go:757`).
2. Serialize renders the marker back to text: leaf prefix (`serialize.go:270`) or `splitInactiveMembers` + follow-up statement (`serialize.go:316`, `serialize_set.go:204`). The raw prefix is stripped here.
3. Load path prunes: `PruneInactive` (`loader.go:128`) drops inactive whole leaves via `pruneInactiveLeaves` (`tree.go:101`) but leaves prefixed members in `multiValues`.
4. `ToMap` (`tree.go:800`) converts the surviving Tree to `map[string]any`; prefixed members pass through raw into the map.
5. BGP config resolution and the reactor consume the map / Tree and string-sniff `inactive:` to skip or disable a filter reference (`filter_chain.go:182`, `peers.go:609`).

Target path after migration: step 1 records per-member deactivation in a new out-of-band structure; step 2 reads that structure (no prefix to strip); step 4 `ToMap` no longer emits any `inactive:` string; step 5 consumers query a structural accessor instead of a string prefix.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Parser <-> Tree | `SetLeafInactive` / `DeactivateMultiValue` set the marker; `applyInactive` chooses which (`parser.go:151`) | [ ] |
| Tree <-> serialized text | `Serialize` / `SerializeSet` render `inactive:` statements; `splitInactiveMembers` strips the member prefix (`serialize.go:316`) | [ ] |
| Tree <-> `map[string]any` | `ToMap` copies `values`/`multiValues` but not `inactiveValues` (`tree.go:800-838`) -- today the prefix leaks, the sibling-map does not | [ ] |
| Config <-> BGP reactor | `ImportFilters` slices and policy refs carry the `inactive:` prefix into `filter_chain.go:182`, `peers.go:609`, `policy_dryrun.go:35` | [ ] |

### Integration Points
- `Tree.Clone` (`tree.go:177`) -- must deep-copy any new deactivation structure.
- `PruneInactive` / `pruneInactiveLeaves` (`prune.go`, `tree.go:101`) -- must decide member handling: keep the member value, drop the marker only when the operator reactivates.
- `ToMap` (`tree.go:800`) -- the single boundary where per-member deactivation must be surfaced structurally so the reactor can read "present but disabled" without string-sniffing.
- The seven BGP consumers listed in Source files -- their `inactive:` string checks become one shared accessor.

### Architectural Verification
- [ ] No bypassed layers (deactivation flows parser -> Tree -> serialize/ToMap, not around it)
- [ ] No unintended coupling (BGP consumers depend on a config accessor, not on a string convention baked into a value)
- [ ] No duplicated functionality (one deactivation structure and one verb family cover both leaf and member cases; the loser is deleted)
- [ ] Zero-copy preserved where applicable (member values stay in `multiValues`; the marker lives beside them, not glued into the string)
- [ ] Registration over hardcoding -- no new per-feature field, switch case, or factory is added to a core/shared package: the deactivation marker stays inside the config `Tree` type it already belongs to, and BGP consumers read it through an existing accessor rather than a hardcoded string prefix; no new central registry is introduced (small-core/registration; `ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No persisted config on disk contains the literal `inactive:` member prefix | Serialization strips it via `splitInactiveMembers` (`serialize.go:316`, `serialize_set.go:204`); tests assert it is never emitted (`leaflist_member_test.go:204,241`); persistence writes serialized text (`editor_commit.go:160`, `change_file.go:294`) | On-disk format compatibility break; a stored `inactive:8.8.8.8` would reparse as a value | grep every `test/**/*.ci`, `test/**/*.conf`, and fixture for a raw `inactive:<value>` token; confirm persistence goes through `SerializeSetWithMeta` only | unvalidated |
| A-2 | Every reader of a deactivated member goes through `Get`/`GetSlice`/`ToMap` or the seven listed BGP sniff sites -- there is no other place that inspects a `multiValues` element string for the prefix | Repo-wide grep of `inactive:` and `inactiveValuePrefix` (see Source files) | A hidden consumer breaks silently when the prefix is removed | re-run the grep at implementation start and diff against the Source files list | unvalidated |
| A-3 | The reactor needs "present but disabled" only for filter/policy references (`peers.go` LoopDisabled), not for arbitrary typed leaf-lists | `peers.go:607-621` is the only consumer that acts on the disabled-but-present state; `filter_chain.go`/`policy_dryrun.go` merely skip | If a typed leaf-list needs present-but-disabled downstream, prune-away is wrong there too | audit each of the 7 consumers for whether it skips vs. acts on the disabled member | unvalidated |
| A-4 | Whole-leaf `inactiveValues` behavior (invisible to `ToMap`, pruned before the boundary) is the correct target and must not change | `prune.go:80`, `tree.go:800-838`; the leaf path already has zero cross-boundary leak | Migrating members to match leaves would drop the present-but-disabled signal the reactor needs | confirm the winner surfaces per-member state structurally through `ToMap` rather than pruning it, keeping A-3 satisfied | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The `inactive:` prefix is a de-facto cross-boundary protocol consumed by 7 BGP sites; deleting it without an equivalent structural signal silently disables the `LoopDisabled` behavior | `peers.go` no longer sets `LoopDisabled`; a deactivated loop-detection filter starts running again | Add a Tree accessor (`IsMemberInactive`) and surface per-member state through `ToMap` (a structured field) BEFORE deleting any sniff site; characterize `LoopDisabled` with a test first |
| R-2 | Serialization has 4 renderers (`serialize.go`, `serialize_set.go`, `serialize_blame.go`, `serialize_annotated.go`) all calling `splitInactiveMembers`; missing one leaves a stale prefix path | a round-trip test emits a raw `inactive:` token or drops a deactivation | Migrate `splitInactiveMembers` to read the out-of-band map in one place and keep every renderer calling that one helper |
| R-3 | Parser re-entry (`parseInactiveValueOrArray`, set-format `nop`) must set the new marker; a miss makes reload lose per-member state | `parse-inactive-leaflist-member.ci` fails round-trip | Phase 1 characterization test locks the current round-trip output before touching producers |
| R-4 | `AddMultiValueMember` idempotency depends on detecting the deactivated form (`tree.go:686`); moving to out-of-band must preserve "set of an inactive member is a no-op" | `TestSetParserLeafListRoundTrip` or add-member idempotency test regresses | Port the idempotency check to consult the out-of-band map; add an explicit unit test |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|----|--------------| -----|
| Hierarchical config `inactive: name-server 8.8.8.8` parsed then `ze config validate` | → | parser member-deactivation -> Tree out-of-band marker -> serialize member statement | `test/parse/parse-inactive-leaflist-member.ci` |
| Set-format round-trip of a leaf-list with one deactivated member | → | `DeactivateMultiValue` successor + `SerializeSet` | `TestSetFormatDeactivatedMemberRoundTrip` (`internal/component/config/leaflist_member_test.go:190`) |
| Hierarchical round-trip of a single deactivated member | → | member marker + hierarchical serialize | `TestHierarchicalSingleDeactivatedMemberRoundTrip` (`leaflist_member_test.go:293`) |
| Whole-leaf deactivation round-trip | → | `SetLeafInactive` + prune | `test/parse/parse-inactive-leaf.ci`, `TestPruneInactiveLeaf` (`prune_inactive_leaf_test.go:17`) |
| CLI editor deactivate a leaf-list member | → | `editor_leaflist.go` deactivate path | `test/editor/session/leaflist-insert-deactivate.et` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A leaf-list with one deactivated member is parsed, serialized, and reparsed | Round-trip is stable; the deactivated member is recorded out-of-band (no `inactive:` string inside any `multiValues` element) |
| AC-2 | `ToMap()` is called on a Tree with a deactivated leaf-list member | The emitted `map[string]any` value contains the clean member value (`8.8.8.8`), never `inactive:8.8.8.8`; per-member deactivation is surfaced through a structured field, not the value string |
| AC-3 | The seven BGP config/reactor sites resolve a peer whose loop-detection filter is deactivated | `LoopDisabled` is still set and the filter is not executed, using the new accessor instead of `strings.HasPrefix(ref, "inactive:")` |
| AC-4 | On-disk serialized output (hierarchical and set-format) for both whole-leaf and member deactivation | Byte-identical to pre-refactor output; the raw `inactive:` prefix appears nowhere in serialized text |
| AC-5 | `set` of a member already present in deactivated form | No-op; member is not reactivated (idempotency preserved) |
| AC-6 | `inactiveValuePrefix` const and all its string producers/consumers | Deleted from the codebase; a repo-wide grep for `inactiveValuePrefix` and the raw `"inactive:"` value prefix returns only the whole-leaf `inactive: ` statement writers |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSetFormatDeactivatedMemberRoundTrip` | `internal/component/config/leaflist_member_test.go` | member round-trip via out-of-band marker (AC-1) | existing -- must still pass |
| `TestHierarchicalSingleDeactivatedMemberRoundTrip` | `internal/component/config/leaflist_member_test.go` | hierarchical member round-trip (AC-1) | existing -- must still pass |
| `TestHierarchicalWholeLeafInactiveStillWorks` | `internal/component/config/leaflist_member_test.go` | whole-leaf inactive is distinct from member inactive (AC-4) | existing -- must still pass |
| `TestDeactivateMultiValue` / `TestActivateMultiValue` | `internal/component/config/parser_test.go` | verb pair semantics on the new representation (AC-1) | existing -- migrate assertions off the prefix |
| `TestPruneInactiveLeaf` | `internal/component/config/prune_inactive_leaf_test.go` | whole-leaf prune unchanged (AC-4) | existing -- must still pass |
| `TestToMapNoInactivePrefix` (new) | `internal/component/config/tree_test.go` | `ToMap` emits clean member values + structured deactivation (AC-2) | to write |
| `TestMemberDeactivationIdempotentSet` (new) | `internal/component/config/leaflist_member_test.go` | `set` of a deactivated member is a no-op (AC-5) | to write |
| `TestLoopDisabledViaAccessor` (new) | `internal/component/bgp/config/peers_test.go` | `LoopDisabled` set from the structural accessor, not the string prefix (AC-3) | to write |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `parse-inactive-leaflist-member` | `test/parse/parse-inactive-leaflist-member.ci` | operator deactivates one leaf-list member; validate passes; round-trip stable | existing -- no user-facing behavior change; must pass with no regressions |
| `parse-inactive-leaf` | `test/parse/parse-inactive-leaf.ci` | operator deactivates a whole leaf | existing -- must pass |
| `cli-config-deactivate-leaf` | `test/parse/cli-config-deactivate-leaf.ci` | CLI deactivate verb on a leaf | existing -- must pass |
| `leaflist-insert-deactivate` | `test/editor/session/leaflist-insert-deactivate.et` | editor session deactivates a leaf-list member | existing -- must pass |
| whole suite | `make ze-test` | no user-facing behavior change; existing test suite passes with no regressions | gate |

## Files to Modify
- `internal/component/config/tree.go` - add out-of-band per-member deactivation storage keyed on (leaf name, member) plus `IsMemberInactive`/`DeactivateMember`/`ActivateMember`; rewrite `DeactivateMultiValue`/`ActivateMultiValue`/`MultiValueMemberState`/`AddMultiValueMember`/`RemoveMultiValueMember`/`HasMultiValueMember` to consult it; extend `Clone`; extend `ToMap` to surface per-member deactivation structurally; delete `inactiveValuePrefix`.
- `internal/component/config/serialize.go` - read deactivated members from the new map (`serialize.go:316`); update `canInlineContainer` (39-53) to test the map instead of the string prefix.
- `internal/component/config/serialize_set.go` - migrate `splitInactiveMembers` (204) to read the out-of-band map; keep every renderer calling the one helper.
- `internal/component/config/serialize_blame.go` - member branch (257) reads the map.
- `internal/component/config/serialize_annotated.go` - member branch (676) reads the map.
- `internal/component/config/parser.go` - `parseInactiveValueOrArray` (126-144) sets the member marker instead of rewriting the string.
- `internal/component/config/setparser.go` - set-format `nop`/`inactive` member path (370, 484) sets the marker.
- `internal/component/config/prune.go` - decide member prune vs. structural surface consistent with A-3/A-4.
- `internal/component/cli/editor_leaflist.go`, `internal/component/cli/editor_commands.go`, `internal/component/cli/editor_draft.go` - verbs call the successor of `DeactivateMultiValue`/`ActivateMultiValue`.
- `internal/component/bgp/config/peers.go` - replace `HasPrefix(filterName, "inactive:")` (609, 681) with the structural accessor.
- `internal/component/bgp/config/filter_registry.go` - replace `TrimPrefix(name, "inactive:")` (86).
- `internal/component/bgp/config/redistribution.go` - replace `HasPrefix`/`TrimPrefix` (62-64).
- `internal/component/bgp/plugins/cmd/policy/handler.go` - replace `TrimPrefix(c, "inactive:")` (320).
- `internal/component/bgp/plugins/filter_family/config.go` - replace `TrimPrefix(ref, "inactive:")` (116).
- `internal/component/bgp/reactor/filter_chain.go` - replace `HasPrefix(ref, "inactive:")` (182).
- `internal/component/bgp/reactor/policy_dryrun.go` - replace `HasPrefix(ref, "inactive:")` (35, 269).

## Implementation Steps

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring / characterization (MANDATORY FIRST)** -- lock current behavior before changing any representation.
   - Tests: run and record output of `test/parse/parse-inactive-leaflist-member.ci`, `TestSetFormatDeactivatedMemberRoundTrip`, `TestHierarchicalSingleDeactivatedMemberRoundTrip`, `TestPruneInactiveLeaf`, and a new characterization test asserting `LoopDisabled` is set for a deactivated loop-detection filter (`peers.go:616-621`).
   - Files: new `TestLoopDisabledViaAccessor` and `TestToMapNoInactivePrefix` written first (they fail today: `ToMap` currently leaks the prefix).
   - Verify: characterization tests capture the exact serialized bytes and the `LoopDisabled` outcome so later phases prove no regression.
2. **Phase: Extend the winner (out-of-band per-member)** -- add the (leaf, member) deactivation storage and accessors on `Tree`; keep both representations working side by side.
   - Tests: `TestMemberDeactivationIdempotentSet`, `TestToMapNoInactivePrefix`.
   - Files: `tree.go` (new field, `IsMemberInactive`/`DeactivateMember`/`ActivateMember`, `Clone`, `ToMap`).
   - Verify: new accessors set/clear/query per-member deactivation without touching the value string.
3. **Phase: Migrate producers** -- point parser, setparser, and editor verbs at the new accessors.
   - Tests: `TestDeactivateMultiValue`, `TestActivateMultiValue`, `leaflist-insert-deactivate.et`, `parse-inactive-leaflist-member.ci`.
   - Files: `parser.go`, `setparser.go`, `editor_leaflist.go`, `editor_commands.go`, `editor_draft.go`.
   - Verify: parse and CLI edits record deactivation out-of-band; `multiValues` slices no longer contain a prefixed element.
4. **Phase: Migrate serializers** -- every renderer reads the out-of-band map; `splitInactiveMembers` becomes a map lookup.
   - Tests: all round-trip unit + `.ci` tests; byte-compare against Phase 1 captures.
   - Files: `serialize.go`, `serialize_set.go`, `serialize_blame.go`, `serialize_annotated.go`.
   - Verify: on-disk output byte-identical (AC-4).
5. **Phase: Migrate cross-boundary consumers** -- surface per-member deactivation through `ToMap`/Tree accessor; replace the 7 BGP `inactive:` string checks.
   - Tests: `TestLoopDisabledViaAccessor` now passes via the accessor; reactor filter-chain tests unchanged.
   - Files: `peers.go`, `filter_registry.go`, `redistribution.go`, `plugins/cmd/policy/handler.go`, `plugins/filter_family/config.go`, `filter_chain.go`, `policy_dryrun.go`.
   - Verify: `LoopDisabled` and filter-skip behavior preserved without any `strings.HasPrefix(..., "inactive:")` on a value string.
6. **Phase: Delete the loser** -- remove `inactiveValuePrefix` and any now-dead prefix code.
   - Tests: full suite; grep gate for `inactiveValuePrefix` and raw member prefix.
   - Files: `tree.go` (const), stragglers.
   - Verify: AC-6 grep is clean.
7. **Functional tests** -> existing `.ci`/`.et` suite passes; no new user-facing behavior.
8. **Full verification** -> `make ze-verify`.
9. **Complete spec** -> fill audit tables, write learned summary. TWO commits: commit A code + tests + spec + learned summary; commit B `git rm` the spec.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation with `file:line`; the 7 BGP sniff sites are all migrated |
| Correctness | On-disk output byte-identical (AC-4); `LoopDisabled` present-but-disabled preserved (AC-3); member `set` idempotent (AC-5) |
| Data flow | Per-member deactivation surfaces through `ToMap`/accessor, not a value-string prefix; whole-leaf path unchanged (invisible to `ToMap`, pruned before the boundary) |
| Naming | New accessors mirror the existing `SetLeafInactive`/`IsLeafInactive` family; no `inactive:` string literal remains as a value marker |
| Registration over hardcoding | The deactivation marker lives on the `Tree` type it belongs to; BGP consumers read it through a config accessor, not a hardcoded string prefix; no new per-feature field or switch case added to a core/shared struct (`ai/rules/plugin-self-containment.md`) |
| Rule: no-layering | `inactiveValuePrefix` const and all prefix producers/consumers fully deleted (AC-6) |
| Rule: no-workarounds | The reactor gets a real structural signal for present-but-disabled; no test is weakened to hide a dropped `LoopDisabled` |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `inactiveValuePrefix` deleted | `grep -rn "inactiveValuePrefix" internal/` returns nothing |
| No raw member prefix in serialized output | run round-trip `.ci` tests; `grep` output for `inactive:<value>` token returns nothing |
| BGP consumers migrated | `grep -rn 'HasPrefix.*"inactive:"' internal/component/bgp/` returns nothing |
| Per-member marker round-trips | `TestHierarchicalSingleDeactivatedMemberRoundTrip` passes |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | A leaf-list value legitimately beginning with `inactive:` is now stored verbatim and never misread as deactivated (collision risk closed) |
| Error leakage | Deactivation state does not change what is written to disk or logs beyond the existing `inactive:` statement form |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Winner: the out-of-band sibling-map (`inactiveValues` style), extended to key on (leaf name, member) | Keep the in-band `inactive:` prefix and delete the sibling-map | The prefix has a real collision risk (a legitimate value beginning with `inactive:` is misread) and is invisible to code that reads the raw value string; it leaks raw through `ToMap` into 7 BGP sites. The sibling-map is explicit, out-of-band, and collision-free. Its only deficiency is granularity, which we close. |
| Gap to close: per-member deactivation | Leave leaf-list members unsupported by the sibling-map | The sibling-map today marks whole leaves only (`tree.go:33`); the loser's sole advantage is per-member marking. Porting (path, member) keying onto the winner removes the last reason to keep the prefix. |
| Surface per-member deactivation through `ToMap` structurally rather than pruning it away | Match the whole-leaf path exactly (prune deactivated members before `ToMap`) | `peers.go:616-621` needs "present but disabled" to set `LoopDisabled`; pruning the member would drop that signal. The winner must expose the marker to the reactor, not hide it. This is why the two representations are genuinely redundant on storage but the in-band one additionally serves as an ad-hoc transport that must be replaced by an explicit accessor. |
| Keep whole-leaf `inactiveValues` behavior unchanged | Unify leaf and member into a single structure with identical downstream visibility | The whole-leaf path (invisible to `ToMap`, pruned early) is correct and battle-tested; only the member path needs to change. Minimizing the blast radius reduces regression risk (R-1/R-2). |

## Known Limitations
- The on-disk config text form (`inactive: <leaf> <member>` and `inactive`/`nop` set statements) is deliberately preserved; this refactor does not introduce a new serialized syntax.
- The whole-leaf and per-member markers remain two fields on `Tree` (leaf-keyed and (leaf, member)-keyed); they are not collapsed into one structure because their downstream visibility (pruned vs. surfaced) differs by design.

## RFC Documentation

Not applicable -- internal config representation refactor, no wire protocol behavior.

## Implementation Summary

### What Was Implemented
- [pending implementation]

### Bugs Found/Fixed
- [pending implementation]

### Documentation Updates
- [pending implementation]

### Deviations from Plan
- [pending implementation]

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

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| One deactivation representation, loser deleted | functional test + grep | [pending] |
| No `inactive:` prefix leak across the config boundary | functional test | [pending] |
| Externally observable behavior preserved | functional test suite | [pending] |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [pending] | file:line | [pending] |

### Fixes applied
- [pending]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/component/config`, `internal/component/bgp`, `internal/component/cli`)
- [ ] Integration completeness proven end-to-end (round-trip + `LoopDisabled`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] `inactiveValuePrefix` and all prefix producers/consumers deleted (grep clean)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all checks documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-unify-tree-deactivation.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-unify-tree-deactivation.md` only

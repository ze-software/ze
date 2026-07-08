# Spec: unify-tree-deactivation

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/6 |
| Updated | 2026-07-08 |

> **2026-07-08 design revision.** Assumptions A-1..A-4 validated against code; A-5 added
> and marked FALSE. The original design centered `ToMap` as the boundary that surfaces
> per-member deactivation; code tracing proved the functional consumers read
> `GetMultiValues`/`GetSlice` (raw), not `ToMap`. Corrected model: effective-config
> accessors return active-only clean values; a dedicated state accessor serves
> serialize/diff/reactor; the reactor filter chain carries deactivation in a
> `FilterRef{Name, Inactive}` value type (full-removal scope, user-chosen). Data Flow,
> Key Design Decisions, ACs (AC-2/AC-3 revised, AC-7 added), Files to Modify, Phases, and
> Risks (R-5/R-6 added) updated accordingly. **Awaiting user re-confirmation before code.**

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

**Key insights (REVISED 2026-07-08 -- original insights rested on a false `ToMap` premise; see A-5):**
- The functional deactivation behavior does NOT flow through `ToMap`. Filter chains reach the reactor via `extractFilterChain` -> `GetMultiValues` (`redistribution.go:23`, `tree.go:154-158`), and typed leaf-lists (name-server, community, address, ...) reach their consumers via `GetSlice` (`tree.go:169-173`). BOTH accessors return the RAW `multiValues` slice with the `inactive:` prefix intact. `ToMap` is a THIRD raw path, used only for plugin JSON delivery (`filter_family` reads `m["filter"]["export"]`).
- Today the `inactive:` prefix "works" for typed leaf-lists only as an accident: `inactive:65000:1` (community) or `inactive:8.8.8.8` (name-server) fails downstream parsing, so the deactivated member is skipped by malformed-value rejection, not by design. `GetSlice`-based consumers that do NOT re-parse (CLI `diff_tree.go`, doctor `checks_config.go`) surface the raw `inactive:` token to the user.
- The ONLY consumer that needs a real "present but disabled" signal is the reactor filter chain: `applyLoopDetectionConfig` (`peers.go:609-621`) sets `LoopDisabled`, and `PolicyFilterChain`/`TracePolicyFilterChain` skip. This signal travels on the `GetMultiValues` path as `ps.ImportFilters []string`, NOT via `ToMap`.
- Consequence: the winner is the out-of-band sibling map extended to (leaf, member). Deactivated members are excluded from the EFFECTIVE-config accessors (`GetSlice`/`GetMultiValues`/`ToMap` return active-only clean values -- consistent with whole-leaf A-4), and per-member state is surfaced ONLY where structure is needed (serialize, diff, and the reactor filter chain) via a dedicated state-carrying accessor. The reactor chain carries the bit in a `FilterRef{Name, Inactive}` value type, deleting the `inactive:` string transport.

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
| Effective-config accessors (`GetSlice` `tree.go:169`, `GetMultiValues` `tree.go:154`, `ToMap` `tree.go:800`) | INVISIBLE -- none copy `inactiveValues`; whole leaf pruned before the boundary (`tree.go:800-838`) | VISIBLE and RAW -- all three return the `"inactive:8.8.8.8"` string; ~30 `GetSlice` + ~6 `GetMultiValues` callers plus plugin JSON see it raw (`tree.go:814-820`) |
| Reload round-trip | serialize `inactive:` sugar -> parser `applyInactive` -> `SetLeafInactive` (`parser.go:154`) | serialize member statement -> parser `parseInactiveValueOrArray` -> `DeactivateMultiValue` (`parser.go:133`) |
| Prune behavior | `pruneInactiveLeaves` deletes the whole leaf before `ToMap` (`tree.go:101`, `prune.go:80`) | NOT pruned -- deactivated member survives with prefix into `ToMap` |
| Diff / equality | Only a test helper reads it (`serialize_test.go:359`); production diff is text-based (`SerializeSetWithMeta`) | Same -- production diff compares serialized text |
| Clone | Copied (`tree.go:199`) | Copied as part of `multiValues` slice (`tree.go:192-196`) |
| Collision risk | NONE -- out-of-band, value string untouched | YES -- a legitimate leaf-list value beginning with `inactive:` is misread as deactivated; any `Get(name)` reader sees `inactive:8.8.8.8` as a real value |
| Cross-boundary leak | NONE -- pruned away before the config boundary | RAW on 3 accessor paths: (a) `GetMultiValues` -> `extractFilterChain` -> reactor `ImportFilters` -> 7 BGP sites; (b) `GetSlice` -> typed leaf-list consumers (system name-server, bgp_routes, diff, doctor); (c) `ToMap` -> plugin JSON (`filter_family`). NOT via `ToMap` alone (see Source files) |

**Behavior to preserve:**
- On-disk config text is byte-identical before and after: `inactive: <leaf> <value>` for whole-leaf deactivation and `inactive: <leaf> <member>` follow-up statements for member deactivation (hierarchical), and the equivalent `inactive`/`nop` set statements (`serialize_set.go`). The raw `inactive:` prefix must still never appear in serialized output (`leaflist_member_test.go:204,241`).
- Round-trip stability: parse -> serialize -> parse yields the same Tree deactivation state, both whole-leaf and per-member (`parse-inactive-leaflist-member.ci`, `TestSetFormatDeactivatedMemberRoundTrip`, `TestHierarchicalSingleDeactivatedMemberRoundTrip`).
- Runtime effect in the reactor: a deactivated filter reference is not executed (`filter_chain.go:182`), and a deactivated loop-detection filter still suppresses the default loop ingress via `LoopDisabled` (`peers.go:616-621`) -- the "present but disabled" case.
- Prune semantics: whole-leaf deactivation removes the leaf before `ToMap` (`prune.go:80`); `PruneActive` keeps only inactive nodes.
- CLI deactivate/activate verbs behave identically for leaves and leaf-list members (`editor_commands.go`, `cli-config-deactivate-leaf.ci`, `leaflist-insert-deactivate.et`).
- `set`/`add-member` idempotency: setting a member already present in deactivated form is a no-op and does not reactivate it (`AddMultiValueMember`, `tree.go:677-691`).

**Behavior to change:**
- **Externally observable behavior: preserved** (on-disk text, round-trip, filter-chain runtime effect, diff bytes, doctor diagnostics -- all byte/semantics-identical, locked by Phase-1 characterization).
- **Internal representation changes** (2026-07-08): per-member deactivation moves from an in-band `inactive:` string prefix to an out-of-band (leaf, member) structure; the effective-config accessors (`GetSlice`/`GetMultiValues`/`ToMap`) become active-only; the reactor filter chain moves from `[]string` to `[]FilterRef{Name, Inactive}`.
- **Latent-bug fix as a side effect**: today a deactivated typed leaf-list member survives as a malformed `inactive:<value>` on the `GetSlice` path (e.g. `system.go` name-server -> resolv.conf, CLI diff, doctor) and is skipped only by downstream parse failure. Active-only accessors make "deactivated = not in effect" explicit. Phase-1 characterizes the current behavior so any change here is deliberate and documented (AC-2/AC-7), not accidental.

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
4. TWO independent boundaries carry the prefix out of the Tree (**corrected 2026-07-08 -- the spec originally claimed both go through `ToMap`; they do not**):
   - **4a. Plugin JSON (`ToMap`)** (`tree.go:800`) converts the surviving Tree to `map[string]any`; prefixed members pass through raw into the map. This path feeds name-server-style leaf-lists to plugins -- a benign leak that **AC-2** closes.
   - **4b. Filter chain (`GetMultiValues`)** (`redistribution.go:17-24` `extractFilterChain` -> `tree.go:154-158` `GetMultiValues` returns the RAW `multiValues` slice) builds `ps.ImportFilters []string` (`peers.go:155,166,201`). **This path never touches `ToMap`** and carries the FUNCTIONAL behavior (`LoopDisabled`/skip, **AC-3**).
5. BGP config resolution and the reactor consume `ps.ImportFilters []string` and string-sniff `inactive:` to skip or disable a filter reference (`filter_chain.go:182`, `peers.go:609`, `policy_dryrun.go:35`), with canonicalization stripping+re-emitting the prefix (`redistribution.go:62-70`, `handler.go:320-327`).

Target path after migration (**full-removal scope, chosen 2026-07-08**): step 1 records per-member deactivation in a new out-of-band structure; step 2 reads that structure (no prefix to strip); step 4a `ToMap` no longer emits any `inactive:` string; step 4b the filter-chain `[]string` gains a structural active/inactive representation (see Key Design Decisions -- `FilterRef`) so step 5 consumers query a struct field instead of a string prefix. `inactive:` as a value-string marker is deleted everywhere except the whole-leaf/statement on-disk writers (`inactive: <leaf> <value>`), which are preserved.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Parser <-> Tree | `SetLeafInactive` / `DeactivateMultiValue` set the marker; `applyInactive` chooses which (`parser.go:151`) | [ ] |
| Tree <-> serialized text | `Serialize` / `SerializeSet` render `inactive:` statements; `splitInactiveMembers` strips the member prefix (`serialize.go:316`) | [ ] |
| Tree <-> effective config (`GetSlice`/`GetMultiValues`/`ToMap`) | today all three return the raw `multiValues` slice incl. the `inactive:` prefix; target: they return active-only clean values (deactivated members excluded), matching the whole-leaf path | [ ] |
| Tree <-> serialize/diff structural view | serialize (4 renderers) + `diff_tree.go` need ALL members + per-member state; target: a dedicated state-carrying accessor / in-package raw access, not the effective-config accessors | [ ] |
| Config <-> BGP reactor | `extractFilterChain` -> `GetMultiValues` builds `ps.ImportFilters`; today `[]string` carries the `inactive:` prefix into `filter_chain.go:182`, `peers.go:609`, `policy_dryrun.go:35`; target: `[]FilterRef{Name, Inactive}` carries the bit structurally | [ ] |

### Integration Points
- `Tree.Clone` (`tree.go:177`) -- must deep-copy any new deactivation structure.
- `PruneInactive` / `pruneInactiveLeaves` (`prune.go`, `tree.go:101`) -- must decide member handling: keep the member value, drop the marker only when the operator reactivates.
- `GetSlice` / `GetMultiValues` / `ToMap` (`tree.go:154,169,800`) -- the effective-config accessors; target semantics: active-only clean values (deactivated members excluded, no `inactive:` string). ~30 `GetSlice` + ~6 `GetMultiValues` callers inherit correct "deactivated = not in effect" behavior for free.
- A NEW state-carrying accessor (e.g. `GetMultiValuesState(name) []struct{Value string; Inactive bool}`) -- consumed only where structure matters: serialize (round-trip), `diff_tree.go` (deactivate-vs-delete display), and `extractFilterChain` (reactor filter chain).
- The reactor filter chain -- `ps.ImportFilters`/`ExportFilters` change from `[]string` to `[]FilterRef`; the 7 BGP `inactive:` string checks read `ref.Inactive`.
- `Tree.Clone` and `PruneInactive`/`pruneInactiveLeaves` -- must deep-copy / handle the new per-member structure.

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
| A-1 | No persisted config on disk contains the literal `inactive:` member prefix | Serialization strips it via `splitInactiveMembers` (`serialize.go:316`, `serialize_set.go:204`); tests assert it is never emitted (`leaflist_member_test.go:204,241`); persistence writes serialized text (`editor_commit.go:160`, `change_file.go:294`) | On-disk format compatibility break; a stored `inactive:8.8.8.8` would reparse as a value | grep every `test/**/*.ci`, `test/**/*.conf`, and fixture for a raw `inactive:<value>` token; confirm persistence goes through `SerializeSetWithMeta` only | **VALIDATED 2026-07-08**: `rg 'inactive:[^ ]'` over `*.conf/*.ci/*.et` returns only two comment lines, no on-disk member token |
| A-2 | Every reader of a deactivated member goes through `Get`/`GetSlice`/`ToMap` or the seven listed BGP sniff sites -- there is no other place that inspects a `multiValues` element string for the prefix | Repo-wide grep of `inactive:` and `inactiveValuePrefix` (see Source files) | A hidden consumer breaks silently when the prefix is removed | re-run the grep at implementation start and diff against the Source files list | **VALIDATED 2026-07-08**: `rg 'HasPrefix\|TrimPrefix\|CutPrefix' -g '!*_test.go'` filtered to `inactive` matches exactly the 7 BGP files + the config serializers. Extra accessor caller `editor_commit.go:535` uses `MultiValueMemberState` (query, rides the accessor -- not a prefix sniff) |
| A-3 | The reactor needs "present but disabled" only for filter/policy references (`peers.go` LoopDisabled), not for arbitrary typed leaf-lists | `peers.go:607-621` is the only consumer that acts on the disabled-but-present state; `filter_chain.go`/`policy_dryrun.go` merely skip | If a typed leaf-list needs present-but-disabled downstream, prune-away is wrong there too | audit each of the 7 consumers for whether it skips vs. acts on the disabled member | **VALIDATED 2026-07-08**: only `peers.go:609-621` acts (sets `LoopDisabled`); `filter_chain.go:182` + `policy_dryrun.go:35,269` skip; canonicalize/validate sites strip+preserve |
| A-4 | Whole-leaf `inactiveValues` behavior (invisible to `ToMap`, pruned before the boundary) is the correct target and must not change | `prune.go:80`, `tree.go:800-838`; the leaf path already has zero cross-boundary leak | Migrating members to match leaves would drop the present-but-disabled signal the reactor needs | confirm the winner surfaces per-member state structurally through `ToMap` rather than pruning it, keeping A-3 satisfied | **VALIDATED 2026-07-08**: `inactiveValues` map is invisible to `ToMap` (`tree.go:800-836`) and cleared/dropped by `pruneInactiveLeaves` (`tree.go:104-120`) |
| A-5 | **CORRECTION (2026-07-08):** the FUNCTIONAL reactor path (filter chains -> `LoopDisabled`/skip) reads deactivation via `ToMap` | Spec Data Flow steps 4-5 assumed `ToMap` | The migration must surface state on the `GetMultiValues` / filter-chain path, not `ToMap` | trace `extractFilterChain` producer | **FALSE -- see Data Flow correction**: `extractFilterChain` (`redistribution.go:17-24`) reads `fc.GetMultiValues("import")` -> `tree.go:154-158` returns the RAW `multiValues` slice; `ToMap` is NOT on the filter-chain path. `ToMap` leaks the prefix only into plugin JSON for name-server-style leaf-lists (AC-2). Two distinct consumer paths share the string convention. |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The `inactive:` prefix is a de-facto cross-boundary protocol consumed by 7 BGP sites; deleting it without an equivalent structural signal silently disables the `LoopDisabled` behavior | `peers.go` no longer sets `LoopDisabled`; a deactivated loop-detection filter starts running again | Introduce `FilterRef{Name, Inactive}` and thread it through the filter chain (`extractFilterChain`->reactor) BEFORE deleting any sniff site; characterize `LoopDisabled` with `TestLoopDisabledViaFilterRef` first |
| R-2 | Serialization has 4 renderers (`serialize.go`, `serialize_set.go`, `serialize_blame.go`, `serialize_annotated.go`) all calling `splitInactiveMembers`; missing one leaves a stale prefix path | a round-trip test emits a raw `inactive:` token or drops a deactivation | Migrate `splitInactiveMembers` to read the out-of-band map in one place and keep every renderer calling that one helper |
| R-3 | Parser re-entry (`parseInactiveValueOrArray`, set-format `nop`) must set the new marker; a miss makes reload lose per-member state | `parse-inactive-leaflist-member.ci` fails round-trip | Phase 1 characterization test locks the current round-trip output before touching producers |
| R-4 | `AddMultiValueMember` idempotency depends on detecting the deactivated form (`tree.go:686`); moving to out-of-band must preserve "set of an inactive member is a no-op" | `TestSetParserLeafListRoundTrip` or add-member idempotency test regresses | Port the idempotency check to consult the out-of-band map; add an explicit unit test |
| R-5 (NEW 2026-07-08) | Making `GetSlice`/`GetMultiValues`/`ToMap` active-only changes what ~36 call sites see; a consumer that today relies on seeing the deactivated member (or its malformed `inactive:` form) could change behavior | a typed leaf-list test (community/address/name-server) or a plugin config test regresses | Enumerated all callers (done); only the reactor filter chain + diff + doctor need structure -- routed to the state accessor. Characterize name-server->resolv.conf, diff, and doctor in Phase 1 before flipping the accessor. |
| R-6 (NEW 2026-07-08) | CLI diff (`diff_tree.go`) and doctor (`checks_config.go`) render/validate the raw `inactive:` token today; active-only accessors would silently drop the deactivation from their output | diff shows a deactivated member as removed; doctor stops flagging a deactivated bad ref | Move both onto `GetMultiValuesState`; lock current bytes/diagnostics with Phase-1 characterization tests; AC-7 requires byte-identical output. |

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
| AC-2 (REVISED 2026-07-08) | `GetSlice`/`GetMultiValues`/`ToMap` are called on a Tree with a deactivated leaf-list member | All three effective-config accessors return **active-only clean values** -- the deactivated member (`8.8.8.8`) is excluded entirely, never returned as `8.8.8.8` (which would reactivate it) nor as `inactive:8.8.8.8`. Per-member state is available only via the dedicated state accessor (`GetMultiValuesState`). Consistent with the whole-leaf path (A-4). |
| AC-3 (REVISED 2026-07-08) | The reactor filter chain resolves a peer whose loop-detection filter is deactivated | `LoopDisabled` is still set and the filter is not executed, reading `FilterRef.Inactive` instead of `strings.HasPrefix(ref, "inactive:")`; `ps.ImportFilters`/`ExportFilters` are `[]FilterRef`, not `[]string`. |
| AC-4 | On-disk serialized output (hierarchical and set-format) for both whole-leaf and member deactivation | Byte-identical to pre-refactor output; the raw `inactive:` prefix appears nowhere in serialized text |
| AC-5 | `set` of a member already present in deactivated form | No-op; member is not reactivated (idempotency preserved) |
| AC-6 (clarified 2026-07-08) | `inactiveValuePrefix` const and all its string producers/consumers on the value/logic path | `grep -rn "inactiveValuePrefix" internal/` and `grep -rn 'HasPrefix.*"inactive:"' internal/component/bgp/` both return nothing. The `"inactive:"` literal survives ONLY at the input/output/display boundaries: parse-input normalization (`stripInactiveMemberPrefix`), serialize output (statement writers), and display/API seams (`filterapi.FilterRefStrings`, `cmd/policy` handler, `diff_tree.memberDisplay`). It appears nowhere in storage, transport, or logic. |
| AC-7 (NEW 2026-07-08) | CLI diff (`diff_tree.go`) and doctor filter-ref check (`checks_config.go`) run against a config with a deactivated member/filter ref | Output is byte-identical to pre-refactor (diff shows the deactivation exactly as today; doctor validates/flags the same refs), achieved by reading the state accessor -- no raw `inactive:` string is fabricated or dropped |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSetFormatDeactivatedMemberRoundTrip` | `internal/component/config/leaflist_member_test.go` | member round-trip via out-of-band marker (AC-1) | existing -- must still pass |
| `TestHierarchicalSingleDeactivatedMemberRoundTrip` | `internal/component/config/leaflist_member_test.go` | hierarchical member round-trip (AC-1) | existing -- must still pass |
| `TestHierarchicalWholeLeafInactiveStillWorks` | `internal/component/config/leaflist_member_test.go` | whole-leaf inactive is distinct from member inactive (AC-4) | existing -- must still pass |
| `TestDeactivateMultiValue` / `TestActivateMultiValue` | `internal/component/config/parser_test.go` | verb pair semantics on the new representation (AC-1) | existing -- migrate assertions off the prefix |
| `TestPruneInactiveLeaf` | `internal/component/config/prune_inactive_leaf_test.go` | whole-leaf prune unchanged (AC-4) | existing -- must still pass |
| `TestEffectiveAccessorsActiveOnly` (new, renamed 2026-07-08) | `internal/component/config/tree_test.go` | `GetSlice`/`GetMultiValues`/`ToMap` exclude a deactivated member; `GetMultiValuesState` reports it (AC-2) | to write |
| `TestMemberDeactivationIdempotentSet` (new) | `internal/component/config/leaflist_member_test.go` | `set` of a deactivated member is a no-op (AC-5) | to write |
| `TestLoopDisabledViaFilterRef` (new, renamed 2026-07-08) | `internal/component/bgp/config/peers_test.go` | `LoopDisabled` set from `FilterRef.Inactive`, not the string prefix; `ImportFilters` is `[]FilterRef` (AC-3) | to write |
| `TestDiffDeactivatedMemberBytes` (new 2026-07-08) | `internal/component/cli/diff_tree_test.go` | CLI diff bytes for a deactivated member byte-identical to pre-refactor (AC-7) | characterize then keep |
| `TestDoctorFilterRefDeactivated` (new 2026-07-08) | `internal/component/doctor/checks_config_test.go` | doctor validation/message for a deactivated ref unchanged (AC-7) | characterize then keep |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `parse-inactive-leaflist-member` | `test/parse/parse-inactive-leaflist-member.ci` | operator deactivates one leaf-list member (statement form); validate passes; round-trip stable | existing -- must pass with no regressions |
| `parse-inactive-leaflist-member-inline` | `test/parse/parse-inactive-leaflist-member-inline.ci` | operator deactivates a member via the inline `[ inactive:MEMBER ... ]` shorthand (typed list); validate passes | NEW 2026-07-08 -- functional test for the accepted inline input form |
| `parse-inactive-leaf` | `test/parse/parse-inactive-leaf.ci` | operator deactivates a whole leaf | existing -- must pass |
| `cli-config-deactivate-leaf` | `test/parse/cli-config-deactivate-leaf.ci` | CLI deactivate verb on a leaf | existing -- must pass |
| `leaflist-insert-deactivate` | `test/editor/session/leaflist-insert-deactivate.et` | editor session deactivates a leaf-list member | existing -- must pass |
| whole suite | `make ze-test` | no user-facing behavior change; existing test suite passes with no regressions | gate |

## Files to Modify

(REVISED 2026-07-08 -- expanded for full-removal scope + corrected accessor model. The
config + serialize + parser + editor group is unchanged in intent; the BGP/reactor group
grows to cover the `FilterRef` representation, and diff/doctor are added.)

**Config Tree storage + accessors**
- `internal/component/config/tree.go` - add out-of-band per-member deactivation storage keyed on (leaf name, member); add `DeactivateMember`/`ActivateMember`/`IsMemberInactive` and a state accessor `GetMultiValuesState(name) []MemberState`; make `GetSlice`/`GetMultiValues`/`ToMap` return **active-only clean** values; rewrite `DeactivateMultiValue`/`ActivateMultiValue`/`MultiValueMemberState`/`AddMultiValueMember`/`RemoveMultiValueMember`/`HasMultiValueMember` to consult the map; extend `Clone`; delete `inactiveValuePrefix`.

**Serializers (read ALL members + state via the new accessor / in-package raw)**
- `internal/component/config/serialize.go` - `splitInactiveMembers` callers (`serialize.go:316`) + `canInlineContainer` (`serialize.go:46`) read the map, not the prefix.
- `internal/component/config/serialize_set.go` - `splitInactiveMembers` (204/207) reads the map; the one helper stays the single split point.
- `internal/component/config/serialize_blame.go` - member branch (257).
- `internal/component/config/serialize_annotated.go` - member branch (676).

**Producers (parser / setparser / editor verbs)**
- `internal/component/config/parser.go` - `parseInactiveValueOrArray` (126-144) sets the member marker instead of rewriting the string.
- `internal/component/config/setparser.go` - set-format `nop`/`inactive` member path (370, 484) sets the marker.
- `internal/component/config/prune.go` - member handling consistent with A-3/A-4 (member value kept, marker cleared on reactivate).
- `internal/component/cli/editor_leaflist.go`, `editor_commands.go`, `editor_draft.go` - verbs call `DeactivateMember`/`ActivateMember` (and `MultiValueMemberState` keeps working via the map; `editor_commit.go:535` rides the accessor unchanged).

**Reactor filter chain -- `[]string` -> `[]FilterRef{Name, Inactive}` (NEW files vs. original spec)**
- `internal/component/bgp/config/redistribution.go` - `extractFilterChain` reads `GetMultiValuesState` and returns `[]FilterRef`; `canonicalizeFilterRefs`/`canonicalizeOne` rewrite `.Name`, preserve `.Inactive` (delete strip/re-wrap).
- `internal/component/bgp/config/peers.go` - `concatFilters`, `filterChainContains`, `prependDefaultFilters` operate on `[]FilterRef`; `applyLoopDetectionConfig` reads `.Inactive` (609-621, 681).
- `internal/component/bgp/config/filter_registry.go` - `ValidateFilterNames` reads `.Name` (86).
- `internal/component/bgp/reactor/peersettings.go` - `ImportFilters`/`ExportFilters` become `[]FilterRef` (388, 392). **[new]**
- `internal/component/bgp/reactor/reactor_dynamic.go` - `resolveFilterVars` operates on `.Name`; `dynImportFilters` typed `[]FilterRef` (142, 321-331, 366). **[new]**
- `internal/component/bgp/reactor/reactor_api.go` - render `.Name` (+ inactive flag) into the API response (129). **[new]**
- `internal/component/bgp/reactor/filter_chain.go` - `PolicyFilterChain([]FilterRef, ...)` reads `.Inactive` (175, 182).
- `internal/component/bgp/reactor/policy_dryrun.go` - `TracePolicyFilterChain([]FilterRef, ...)` reads `.Inactive` (26, 35, 269).
- `internal/component/bgp/plugins/cmd/policy/handler.go` - `toFilterRefs` takes `[]FilterRef` (or canonical + inactive), preserves CLI display (320-327).
- `internal/component/bgp/plugins/filter_family/config.go` - `exportChain` gets active-only from `ToMap`; drop the now-dead `TrimPrefix` (116); confirm inactive family-filter instance handling unchanged (characterize first).
- **Decide `FilterRef` home**: `internal/component/bgp/filterapi/filterapi.go` (BGP-owned value types) is the natural location; `LoopDisabled` comment (41) updated.

**CLI diff + doctor (read active-only for effect, state accessor for display) -- NEW vs. original spec**
- `internal/component/cli/diff_tree.go` - `diffValueOrArray` (196-213) reads `GetMultiValuesState` to preserve deactivate-vs-delete diff bytes (AC-7). **[new]**
- `internal/component/doctor/checks_config.go` - `checkFilterRefs` (120-136) preserves current validation/message for deactivated refs via the state accessor (AC-7). **[new]**

## Implementation Steps

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring / characterization (MANDATORY FIRST)** -- lock current behavior before changing any representation. (REVISED 2026-07-08: added diff/doctor/name-server captures.)
   - Tests: run and record output of `test/parse/parse-inactive-leaflist-member.ci`, `TestSetFormatDeactivatedMemberRoundTrip`, `TestHierarchicalSingleDeactivatedMemberRoundTrip`, `TestPruneInactiveLeaf`, plus new characterization tests for: (a) `LoopDisabled` set for a deactivated loop-detection filter (`peers.go:616-621`); (b) the exact CLI diff bytes for a deactivated member (`diff_tree.go`); (c) doctor `checkFilterRefs` output for a deactivated ref; (d) what `GetSlice("name-server")`/resolv.conf does today with a deactivated member (lock the current behavior, whatever it is).
   - Files: new `TestLoopDisabledViaFilterRef`, `TestEffectiveAccessorsActiveOnly` (assert `GetSlice`/`GetMultiValues`/`ToMap` exclude the deactivated member -- fail today), diff/doctor characterization tests, written first.
   - Verify: captures pin exact serialized bytes, diff bytes, doctor diagnostics, and the `LoopDisabled` outcome so later phases prove no regression.
2. **Phase: Extend the winner + accessor contract** -- add (leaf, member) storage; add `DeactivateMember`/`ActivateMember`/`IsMemberInactive` + `GetMultiValuesState`; make `GetSlice`/`GetMultiValues`/`ToMap` active-only; keep the old prefix path alive side-by-side until producers migrate.
   - Tests: `TestMemberDeactivationIdempotentSet`, `TestEffectiveAccessorsActiveOnly`.
   - Files: `tree.go` (new field, accessors, `GetSlice`/`GetMultiValues`/`ToMap` active-only, `Clone`).
   - Verify: effective accessors exclude deactivated members; state accessor reports them; value strings never carry the prefix.
3. **Phase: Migrate producers** -- parser, setparser, editor verbs record deactivation out-of-band.
   - Tests: `TestDeactivateMultiValue`, `TestActivateMultiValue`, `leaflist-insert-deactivate.et`, `parse-inactive-leaflist-member.ci`.
   - Files: `parser.go`, `setparser.go`, `editor_leaflist.go`, `editor_commands.go`, `editor_draft.go`.
   - Verify: `multiValues` slices no longer contain a prefixed element.
4. **Phase: Migrate serializers + diff** -- every renderer + `diff_tree.go` read the map / state accessor; `splitInactiveMembers` becomes a map lookup.
   - Tests: all round-trip unit + `.ci` tests; diff characterization; byte-compare against Phase 1 captures.
   - Files: `serialize.go`, `serialize_set.go`, `serialize_blame.go`, `serialize_annotated.go`, `cli/diff_tree.go`.
   - Verify: on-disk output + diff bytes byte-identical (AC-4, AC-7).
5. **Phase: Migrate cross-boundary consumers to `FilterRef`** -- introduce `FilterRef{Name, Inactive}`; thread it through the filter-chain pipeline and the reactor; migrate doctor + `filter_family`.
   - Tests: `TestLoopDisabledViaFilterRef`; reactor filter-chain + policy-dryrun tests updated to `[]FilterRef`; doctor characterization.
   - Files: `redistribution.go` (`extractFilterChain`/`canonicalize`), `peers.go`, `filter_registry.go`, `peersettings.go`, `reactor_dynamic.go`, `reactor_api.go`, `filter_chain.go`, `policy_dryrun.go`, `filterapi.go` (`FilterRef` type), `plugins/cmd/policy/handler.go`, `plugins/filter_family/config.go`, `doctor/checks_config.go`.
   - Verify: `LoopDisabled` + filter-skip + diff + doctor preserved with zero `strings.HasPrefix(..., "inactive:")` on a value string.
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
| A-1: the raw `inactive:` member prefix never appears as on-disk/config input (only the `inactive:`/`nop` statement form) | The compact **inline** form `leaf [ inactive:MEMBER ... ]` is a **documented** user feature for untyped (filter-name) leaf-lists -- `docs/guide/redistribution.md` shows `import [ inactive:no-self-as ];`. It worked via the old in-band representation. For TYPED lists it was rejected at item validation (`parse-inactive-leaflist-member.ci` comment) -- so the `.ci` comment and redistribution.md are each correct in their own context. | My refactor broke the inline form (parser stored `inactive:X` as a literal member). User flagged it; then I found redistribution.md documents it. A-1's grep only searched `test/**/*.{conf,ci,et}` and missed both the Go test seeds AND the guide docs. | Resolution: normalize the inline prefix at the parse boundary (`stripInactiveMemberPrefix` in `parser_list.go` + `setparser.go`) into the out-of-band marker, for BOTH typed and untyped lists (per user decision 2026-07-08 -- typed inline now also works, a small consistent improvement). Locked by `TestInlineInactiveMemberPrefixNormalized` (typed, both formats), `TestInlineInactiveFilterMember` (untyped), `TestStripInactiveMemberPrefix` (helper edge cases), and `parse-inactive-leaflist-member-inline.ci`. Serializer still emits only the statement form; the `inactive:` literal lives only at input/output/display boundaries. (My initial instinct to delete the form as a "bad test seed" was wrong -- it is documented.) |
| A-5 (spec's own): per-member deactivation reaches the reactor via `ToMap` | The functional path is `GetMultiValues`/`GetSlice` (raw); `ToMap` only feeds plugin JSON. | Data-flow tracing during pre-implementation validation. | Redesigned to active-only accessors + `FilterRef` (see the 2026-07-08 design revision header); ACs revised before coding. |

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
| Winner: the out-of-band sibling-map (`inactiveValues` style), extended to key on (leaf name, member) | Keep the in-band `inactive:` prefix and delete the sibling-map | The prefix has a real collision risk (a legitimate value beginning with `inactive:` is misread) and is invisible to code that reads the raw value string; it leaks raw through THREE accessors (`GetSlice`, `GetMultiValues`, `ToMap`) into ~36 call sites + plugin JSON. The sibling-map is explicit, out-of-band, and collision-free. Its only deficiency is granularity, which we close. |
| Gap to close: per-member deactivation | Leave leaf-list members unsupported by the sibling-map | The sibling-map today marks whole leaves only (`tree.go:33`); the loser's sole advantage is per-member marking. Porting (leaf, member) keying onto the winner removes the last reason to keep the prefix. |
| **CORRECTED 2026-07-08:** effective-config accessors (`GetSlice`/`GetMultiValues`/`ToMap`) return **active-only clean values**; deactivated members are excluded (matching whole-leaf A-4), NOT surfaced inline | Original spec: "surface per-member deactivation through `ToMap` structurally"; alt: return all-clean incl. deactivated members | The original rationale ("`peers.go` needs present-but-disabled via `ToMap`") is FALSE -- `peers.go` reads `GetMultiValues`, not `ToMap` (A-5). No consumer wants a deactivated member returned as an active value; returning all-clean would REACTIVATE deactivated typed members (community/name-server) that today are skipped by malformed-value rejection -- a regression. Active-only is the correct, collision-free, consistent semantics and fixes the latent `GetSlice` pass-through (raw `inactive:` reaching resolv.conf / diff / doctor). |
| **NEW 2026-07-08:** structure-needing consumers use a dedicated state-carrying accessor, not the effective accessors | Overload `GetSlice` return type; thread a parallel `[]bool` | Serialize (round-trip), `diff_tree.go` (deactivate-vs-delete display), and `extractFilterChain` need ALL members + per-member state. A single `GetMultiValuesState(name) []struct{Value string; Inactive bool}` (plus in-package raw access for serializers) keeps the effective accessors clean while giving these three exactly what they need. |
| **NEW 2026-07-08:** the reactor filter chain carries deactivation in a `FilterRef{Name string; Inactive bool}` value type, replacing `[]string` | Keep `[]string` + re-attach `inactive:` at `extractFilterChain` (leaves the prefix inside the reactor); parallel `map[string]bool` on `PeerSettings` | Full-removal scope (user-chosen 2026-07-08) requires `inactive:` to appear NOWHERE as a value marker (AC-3/AC-6). `FilterRef` threads through `extractFilterChain`->`concatFilters`->`canonicalizeFilterRefs`->`PeerSettings.ImportFilters/ExportFilters`->`resolveFilterVars`->`PolicyFilterChain`/`TracePolicyFilterChain`->`reactor_api`. Explicit, collision-free, no string convention. Cost: ~10 functions + 2 exported reactor signatures + their tests change. |
| Keep whole-leaf `inactiveValues` behavior unchanged | Unify leaf and member into a single structure with identical downstream visibility | The whole-leaf path (invisible to effective accessors, pruned early) is correct and battle-tested; only the member path needs to change. Minimizing the blast radius reduces regression risk (R-1/R-2). |
| **ADDED SCOPE 2026-07-08 (user decision):** accept the inline `leaf [ inactive:MEMBER ... ]` form as a first-class input shorthand, normalized at the parse boundary (`stripInactiveMemberPrefix`) for BOTH typed and untyped lists | (a) reject the inline form and require the `inactive: <leaf> <member>` statement (my initial workaround-removal proposal); (b) keep the ad-hoc parser hack unspec'd | The inline form is the compact way operators deactivate a filter ref. Rather than a silent workaround for one bad test, the user chose to make it a real feature: documented, tested (typed + untyped, unit + `.ci`), and normalized to the canonical statement form on serialize so on-disk output is unchanged. Trade-off: a member value literally beginning with `inactive:` must use the statement form (the same collision the winner closes for storage, now confined to this one input shorthand). |

## Known Limitations
- The on-disk config text form (`inactive: <leaf> <member>` and `inactive`/`nop` set statements) is deliberately preserved; this refactor does not introduce a new serialized syntax.
- The whole-leaf and per-member markers remain two fields on `Tree` (leaf-keyed `inactiveValues` and (leaf, member)-keyed). Both are excluded from the effective-config accessors; the difference is that whole-leaf deactivation drops the leaf entirely while a deactivated member keeps its siblings visible, so per-member state is additionally exposed to serialize/diff/reactor via the state-carrying accessor.
- The CLI diff (`diff_tree.go`) and doctor filter-ref check (`checks_config.go`) currently render/validate the raw `inactive:` token; the target is to preserve current diff bytes and doctor semantics by moving them onto the state accessor. Phase-1 characterization locks their current output first (see Risks R-5/R-6).

## RFC Documentation

Not applicable -- internal config representation refactor, no wire protocol behavior.

## Implementation Summary

### What Was Implemented
- Config `Tree`: `inactiveMembers map[string]map[string]bool` (out-of-band per-member deactivation); `MemberState` + `GetMultiValuesState`; `GetSlice`/`GetMultiValues`/`ToMap` made active-only; verb methods (`DeactivateMultiValue`/`ActivateMultiValue`/`MultiValueMemberState`/`Add`/`Remove`) rewritten onto the map; `Clone`/`SetSlice`/`pruneInactiveLeaves` updated; `inactiveValuePrefix` const deleted.
- Serializers (4): `splitInactiveMembers(items, inactiveSet)` single split point reading the map; `canInlineContainer` tests the map.
- Parser: inline `[ inactive:MEMBER ]` shorthand normalized at the parse boundary (`stripInactiveMemberPrefix`) in hierarchical `storeValueOrArray` + set-format `setparser.go` -- accepted for typed AND untyped lists.
- Reactor filter chain: `filterapi.FilterRef{Name, Inactive}` (+ `FilterRefStrings` display seam) replaces `[]string`; threaded through `extractFilterChain`/`canonicalize`/`concat`/`ValidateFilterNames`/`applyLoopDetectionConfig`/`filterChainContains`/`prependDefaultFilters`/`PeerSettings`/`resolveFilterVars`/`resolveFilterOverride`/`PolicyFilterChain`/`TracePolicyFilterChain`/`reactor_api`. The 7 BGP sniff sites now read `FilterRef.Inactive`.
- CLI diff (`diff_tree.memberDisplay`) and doctor (`checks_config` via `GetMultiValuesState`) preserve deactivation display/validation (AC-7).

### Bugs Found/Fixed
- Review ISSUE-1: removed dead export `IsMemberInactive`.
- Review ISSUE-2: `stripInactiveMemberPrefix` empty-member edge (bare `inactive:` token) guarded + tested.
- Latent fix: a deactivated typed member previously reached `GetSlice` consumers (e.g. resolv.conf) as a malformed `inactive:8.8.8.8`; active-only accessors make "deactivated = not in effect" explicit.

### Documentation Updates
- `docs/architecture/config/yang-config-design.md`: out-of-band representation, two input forms, collision trade-off, new source anchors.
- `docs/guide/redistribution.md`: refreshed stale source anchors (FilterRef mechanism) + inline-form normalization note.
- New functional test `test/parse/parse-inactive-leaflist-member-inline.ci`; comment fix in `parse-inactive-leaflist-member.ci`.

### Deviations from Plan
- **Scope added (user decision):** the inline `[ inactive:X ]` form is now a first-class documented input shorthand (the plan treated it as never-valid); see Mistake Log A-1 + Key Design Decisions.
- **AC-2 reworded** to active-only accessors (not "structured ToMap field") after tracing showed the functional path is `GetMultiValues`, not `ToMap` (A-5).
- **AC-3 reworded** to `FilterRef.Inactive`; **AC-7 added** (diff/doctor). Reactor filter-chain scope grew beyond the original file list (peersettings, reactor_dynamic, reactor_api, filter_ordered, peer, peer_forward_facts, peer_initial_sync).
- Kept `DeactivateMultiValue`/`ActivateMultiValue`/`MultiValueMemberState` method NAMES (bodies rewritten) so producers (parser/setparser/editor) needed no changes -- smaller than the plan's file list.

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
| One deactivation representation, loser deleted | grep + unit | `rg inactiveValuePrefix internal/` -> empty; `rg 'HasPrefix.*"inactive:"' internal/component/bgp/` -> empty. `TestEffectiveAccessorsActiveOnly`, `TestDeactivateMultiValue`/`TestActivateMultiValue` on the out-of-band map. |
| No `inactive:` prefix leak across the config boundary | unit + audit | `TestEffectiveAccessorsActiveOnly` asserts `GetSlice`/`GetMultiValues`/`ToMap` emit no `inactive:` string; every remaining `"inactive:"` literal is at an input/parse, serialize-output, or display/API boundary (audited via `rg '"inactive:"' internal/ -g '!*_test.go'`). |
| Externally observable behavior preserved | functional suite | `parse --all` 238/238, `editor --all` 159/159; hierarchical + set-format round-trip tests byte-stable (`inactive: <leaf> <member>` / `nop`); reactor `LoopDisabled` preserved (`TestLoopDetectionInactiveDisables`); diff/doctor preserved (AC-7). Full unit suite + `-race` GREEN. |
| Inline `[ inactive:X ]` input form preserved (documented feature) | functional + unit | `parse-inactive-leaflist-member-inline.ci` (`ze config validate`), `TestInlineInactiveMemberPrefixNormalized` (typed, both formats), `TestInlineInactiveFilterMember` (untyped). |

## Review Gate

### Run 1 (initial) -- 2026-07-08
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE (wiring) | `IsMemberInactive` was an exported accessor with only test callers (dead production export; flagged by `ze-validate`) | `tree.go:218` | Removed; tests use the existing `MultiValueMemberState` / `GetMultiValuesState`. |
| 2 | ISSUE (edge case) | `stripInactiveMemberPrefix` turned a bare `inactive:` token (stray-space input) into an empty member via `CutPrefix -> ("", true)` | `parser_list.go` | Guarded with `rest != ""` (bare token kept literal so validation rejects it); regression test `TestStripInactiveMemberPrefix`. |
| 3 | ISSUE (doc drift) | `redistribution.md` documents the inline form but its source anchor `filter_chain.go -- inactive: prefix skipping` was stale after the FilterRef migration | `docs/guide/redistribution.md:88` | Updated anchors to `stripInactiveMemberPrefix`, `extractFilterChain`, `PolicyFilterChain skips FilterRef.Inactive`; added the normalize-to-statement-form note. |

Automated pre-checks: `make ze-validate` -- my delta clean (`IsMemberInactive` no longer flagged; remaining unwired-export ISSUEs are pre-existing core/reactor exports + the parallel RADIUS session, out of scope per review step 18). `audit-test-relaxation.py` -- 0 deleted, 0 weakened, 1 documented `[RELAXED]` (`filter_family/config_test.go`, justified: the `inactive:` tolerance was genuinely removed because ToMap is active-only).

### Fixes applied
- ISSUE-1: deleted `Tree.IsMemberInactive`; migrated 7 test call sites to `MultiValueMemberState`.
- ISSUE-2: `rest != ""` guard + `TestStripInactiveMemberPrefix`.
- ISSUE-3: refreshed `redistribution.md` source anchors and behavior note.

### Post-fix verification
- Unit: config/... + bgp/config + reactor + cli + doctor + filter_family GREEN; `-race` on config + bgp/config + reactor GREEN (no data races).
- Functional: `parse --all` 238/238 (incl. new inline `.ci`), `editor --all` 159/159.
- Lint: `make ze-lint-changed` exit 0.

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE  <!-- 0 BLOCKER; the 3 ISSUEs above were fixed; no new findings on the fix diff -->
- [ ] All NOTEs recorded above (or explicitly "none")  <!-- NOTEs: none -->

**Gate result: 0 BLOCKER, 0 ISSUE remaining (3 fixed). NOTEs: none.**

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

# Spec: config-3-deactivate

| Field | Value |
|-------|-------|
| Status | done |
| Depends | spec-policy-3 (existing `inactive:` plumbing for containers/list-entries/leaf-list values) |
| Phase | 1/10 |
| Updated | 2026-04-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `docs/architecture/config/syntax.md` -- inactive node syntax pointer (referenced by prune.go:1, but `inactive:` itself is undocumented in the grammar today)
4. `docs/architecture/config/yang-config-design.md` -- YANG to Tree mapping
5. `docs/architecture/config/tokenizer.md` -- token rules
6. `internal/component/config/tree.go` -- Tree struct (lines 25-44)
7. `internal/component/config/yang_schema.go:454-563` -- auto-injected `inactive` leaf
8. `internal/component/config/parser.go:229-276` -- `inactive:` sugar (containers)
9. `internal/component/config/parser_list.go:90-126` -- `inactive:` sugar (list entries) + leaf rejection at 123
10. `internal/component/config/serialize.go:48,322,352` -- `inactive: ` prefix emission
11. `internal/component/config/prune.go` -- PruneInactive / PruneActive (recursive, schema-driven)
12. `internal/component/cli/model_commands.go:510-629` -- TUI cmdDeactivate / cmdActivate
13. `internal/component/cli/editor_commands.go:343-378` -- Editor APIs for leaf-list deactivation
14. `cmd/ze/config/cmd_set.go` -- CLI pattern to mirror
15. `plan/learned/541-policy-framework.md` -- prior art

**LSP findReferences on `InactiveLeafName` (yang_schema.go:456): 23 refs across 7 files** -- yang_schema.go, parser.go, parser_list.go, serialize.go, serialize_annotated.go, serialize_blame.go, cli/model_commands.go. This bounds the leaf-extension surface area.

## Task

Extend ze's existing `inactive:`/`deactivate`/`activate` mechanism so it covers **every YANG node type** (container, list entry, leaf, leaf-list value) and is exposed as one-shot CLI verbs. Three deliverables:

1. **Engine-level extension to leaves.** Today, leaves are explicitly rejected at parse and at the model command (`parser_list.go:123` warns; `model_commands.go:549,609` errors). Lift this restriction. Deactivation must be uniform across every node type, with no per-schema YANG annotation required -- the engine handles it transparently.
2. **One-shot CLI verbs `ze config deactivate <file> <path...>` and `ze config activate <file> <path...>`** that mirror `cmd_set.go` and wrap the existing Editor APIs.
3. **User-facing documentation** describing `inactive:` syntax, the CLI verbs, and the round-trip / pruning semantics.

### Design principles (from SCOPE gate)

- **Engine-level / automatic / transparent.** No `ze:deactivable` extension or schema-author opt-in. Every YANG node deactivatable by default.
- **Co-exists with per-feature `disable` semantics.** A peer's `admin-state disable` (operationally distinct from "no such peer") is unaffected.
- **Round-trips.** Parse -> serialize must preserve the deactivated state structurally.
- **Components see deactivated subtrees as absent.** `PruneInactive` already does this; the leaf extension hooks into the same path.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/config/syntax.md` -- existing config grammar
  -> Constraint: `inactive:` is **not currently documented** in the grammar reference. Spec must add a leaf-prefix syntax rule. Today the grammar specifies `keyword value;` with no sigil capability for leaves.
  -> Constraint: leaf-prefix syntax pattern (`<sigil>: <name> <value>;`) does not exist yet for leaves -- only for structural statements.
- [ ] `docs/architecture/config/yang-config-design.md` -- YANG to Tree mapping
  -> Constraint: leaves are stored in `Tree.values map[string]string`. There is **no per-leaf metadata layer.** Containers/lists track inactivity via an auto-injected `inactive` boolean leaf inside their own `values`; leaves cannot do the same because they have no children.
  -> Decision: leaf inactivity must live **outside the value string** (per principle: round-trip preserved, no value collision) and **outside YANG** (per principle: transparent to schema authors). The only place left is a sibling map on the parent Tree.
- [ ] `docs/architecture/config/tokenizer.md`
  -> Constraint: tokenizer treats `:` as a word character; `inactive:` is a single token already, recognised at parser level (parser.go:232, parser_list.go:92). No tokenizer change required.
- [ ] `ai/patterns/cli-command.md` -- offline command pattern
  -> Constraint: handler signature `func cmdXxx(args []string) int`. Exit codes: 0 success, 1 error, 2 file not found. Errors to stderr via `fmt.Fprintf`. Register in dispatch (mirror `cmd_set.go`).
- [ ] `ai/rules/cli-patterns.md`
  -> Constraint: never `os.Exit()` from handlers; return code from handler. flag.NewFlagSet for parsing.
- [ ] `ai/patterns/functional-test.md`
  -> Constraint: `.ci` test format -- `tmpfs=test.conf:terminator=EOF_CONF` to embed config, `cmd=foreground:exec=ze config deactivate ...`, `expect=exit:code=0`, `expect=stdout:contains=...`.

### Learned Summaries
- [ ] `plan/learned/541-policy-framework.md` -- prior art for `inactive:` semantics
  -> Decision: `inactive:` prefix on leaf-list values (over a separate `no-import` leaf or `active false` leaf) -- existing mechanism we extend, not replace. The chosen-on-purpose Junos semantics carry forward.

**Key insights:**
- The mechanism exists for containers, list entries, and leaf-list values. Surface area to extend: leaves only.
- Tree leaves (`values map[string]string`) have no metadata slot -- needs a sibling `inactiveValues map[string]bool` on the parent Tree.
- `inactive:` prefix is already a recognised token; just lift the rejection branch and add the leaf storage path.
- One-shot CLI verbs are mechanical: copy `cmd_set.go`, swap `SetValue` for new Editor methods.

## Current Behavior (MANDATORY)

**Source files read:**

- [ ] `internal/component/config/tree.go:25-44` -- `Tree` struct: `values`, `multiValues`, `containers`, `lists`, `listOrder`. RWMutex per Tree.
  -> Constraint: every public method acquires `mu`; walkers that touch internals directly must hold the lock. New `inactiveValues` field must follow the same lock discipline.
- [ ] `internal/component/config/yang_schema.go:454-563` -- `InactiveLeafName = "inactive"`; auto-injected by `yangToContainer` (line 490) and `yangToList` for structural lists only (line 561). Positional lists (all-leaf children) skip injection -- "deactivate the parent instead."
  -> Constraint: positional lists (`nlri`, `nexthop`, `add-path`) are NOT individually deactivatable today and must remain so -- the parent container is the deactivation unit. Leaves inside such lists, however, will be deactivatable individually under this spec (consistent with the universal-leaf goal).
- [ ] `internal/component/config/parser.go:229-276` -- `inactive:` sugar for containers; on leaf miss, falls through to the warning at line 274 ("inactive: prefix ignored on leaf %s").
  -> Decision: replace the warning branch with leaf inactivation by calling a new `child.SetLeafInactive(fieldName, true)` after `parseNode` succeeds.
- [ ] `internal/component/config/parser_list.go:90-126` -- mirror logic for list entries, same warning at line 123.
  -> Decision: same fix as parser.go, applied here.
- [ ] `internal/component/config/serialize.go:48,259,322,352,613` -- `inactive: ` prefix at every container/list-entry rendering site. Line 259 explicitly skips serializing the `inactive` leaf as a child (rendered as parent prefix instead).
  -> Decision: at the `*LeafNode` case (line 255), check `tree.IsLeafInactive(name)`; if true, emit `inactive: <name> <value>;`. Mirror in `serialize_annotated.go` and `serialize_blame.go`.
- [ ] `internal/component/config/prune.go` -- `PruneInactive` walks containers and lists; does not touch leaves.
  -> Decision: extend `pruneNode` to also walk a new branch for leaves -- delete entries from `tree.values` whose name appears in `tree.inactiveValues`.
- [ ] `internal/component/config/tree.go:577-617` -- `DeactivateMultiValue` / `ActivateMultiValue` for leaf-list values (`inactive:` prefix on the value string).
  -> Constraint: keep this API as-is; leaf-list values keep their string-prefix scheme (collision-safe because leaf-list values are typically tokens, not arbitrary user strings).
- [ ] `internal/component/cli/model_commands.go:510-629` -- TUI `cmdDeactivate` / `cmdActivate`. Leaf rejection at line 549 ("cannot deactivate a leaf value, use delete instead"); same on activate at 609.
  -> Decision: replace the rejection with a call to a new `Editor.DeactivateLeaf(parentPath, leafName)` and the symmetric `ActivateLeaf`. Update the user message.
- [ ] `internal/component/cli/editor_commands.go:343-378` -- Editor wrappers for leaf-list deactivate.
  -> Decision: add `DeactivateLeaf` / `ActivateLeaf` mirroring those wrappers but operating on `Tree.SetLeafInactive` / equivalent.
- [ ] `cmd/ze/config/cmd_set.go` (full) -- pattern: flag.NewFlagSet, helpfmt.Page, NewEditorWithStorage, ValidateValueAtPath, SetValue, dry-run via Diff, Save, daemon notify via SSH.
  -> Decision: `cmd_deactivate.go` reuses every step except the value mutation: instead of `ed.SetValue`, call `ed.DeactivateLeaf` (or `ed.DeactivateContainer` for non-leaf paths -- the command resolves which based on schema, like `cmdDeactivate` in TUI).

**Behavior to preserve:**
- `inactive:` prefix syntax for containers/list entries (round-trips through parse/serialize).
- `inactive:` prefix on leaf-list values (DeactivateMultiValue / ActivateMultiValue).
- Auto-injected `inactive` leaf on every container/list entry with structural children.
- `PruneInactive` runs at every documented apply site (loader.go:111, editor.go:365, bgp/config/peers.go:48, cmd_validate.go:205) -- same callers, same effect.
- TUI `deactivate` / `activate` model commands (existing keystrokes / completer behavior). The user-visible behavior on containers/list entries / leaf-list values does not change.
- Positional lists (`nlri`, `nexthop`, `add-path`) remain non-individually-deactivatable. Their parent container is the unit. Leaves *inside* those lists become individually deactivatable under this spec, which is a strict superset of today's behavior.

**Behavior to change:**
- Leaves: lift the parser warning at `parser_list.go:123` and `parser.go:274`, lift the model command rejection at `model_commands.go:549,609`. Leaves accept `inactive:` and are honoured at apply time.
- Add one-shot CLI verbs `ze config deactivate` / `ze config activate`.
- Add user-facing documentation page (`docs/guide/config-deactivate.md`) and update `docs/architecture/config/syntax.md` to formally specify the `inactive:` prefix grammar (currently undocumented).

## Data Flow

### Entry Point
Two entry points:

| Entry | Source | Sink |
|---|---|---|
| `ze config deactivate <file> <path...>` | shell argv | `Editor.DeactivateLeaf` / existing container/list-entry path -> serialize -> file |
| `inactive: <name> <value>;` in a config file | parser | `Tree.SetLeafInactive(name, true)` then standard `parseNode` |

### Transformation Path
1. **CLI dispatch** (`cmd/ze/config/main.go`) -> `cmdDeactivate(args)` -> resolve file path, build Editor, schema-validate target path.
2. **Editor mutation** -> if path resolves to a leaf in schema, call `Editor.DeactivateLeaf(parent, leaf)`. Otherwise existing container/list-entry / leaf-list-value branches.
3. **Tree update** -> sets `parent.inactiveValues[leaf] = true` (new field). For containers/list entries, sets `child.values[InactiveLeafName] = "true"` (existing). For leaf-list values, prepends `inactive:` to the value string (existing).
4. **Serialize** -> three rendering paths emit the `inactive: ` prefix:
   - Container/list-entry: existing (lines 48, 322, 352).
   - Leaf-list value: existing (value string already includes prefix).
   - **New: leaf** -- at `*LeafNode` case (line 255), check `tree.IsLeafInactive(name)` and emit prefix.
5. **Save** -> file written; daemon notified via SSH.
6. **Apply path** (later, on daemon load) -> parser reads `inactive: <name> <value>;` -> stores via `SetLeafInactive`. `PruneInactive` (called from `loader.go:111`) walks the tree and removes any `tree.values[name]` where `inactiveValues[name]` is true. Components see the leaf as absent.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> Editor | `cli.NewEditorWithStorage`, `Editor.DeactivateLeaf` | new unit test |
| Editor -> Tree | `Tree.SetLeafInactive` | new unit test |
| Parser -> Tree | `inactive:` sugar dispatching to `SetLeafInactive` | new parser test |
| Tree -> Serializer | `IsLeafInactive` checked in `serializeNode` | new round-trip test |
| Tree -> Apply | `PruneInactive` removes entries from `values` when `inactiveValues[name]` is true | new prune test |

### Integration Points
- `Tree.SetLeafInactive`, `Tree.IsLeafInactive`, `Tree.ClearLeafInactive` -- new methods on existing `Tree`.
- `Editor.DeactivateLeaf`, `Editor.ActivateLeaf` -- new wrappers in `editor_commands.go`.
- `cmdDeactivate` (TUI, `model_commands.go`) -- replace leaf rejection with the new Editor calls.
- `cmdDeactivate` (CLI, new `cmd_deactivate.go`) -- one-shot.

### Architectural Verification
- [ ] No bypassed layers: parser -> Tree -> serializer / Tree -> PruneInactive -> components.
- [ ] No unintended coupling: leaf inactivity lives in `internal/component/config`; no schema, no plugin, no protocol code touched.
- [ ] No duplicated functionality: extends existing `inactive:` mechanism. Prune walk and serializer prefix already there; only the leaf branch is new.
- [ ] Zero-copy preserved: leaves are scalar strings; no allocations changed.

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config deactivate <file> <path-to-leaf>` | -> | `cmd/ze/config/cmd_deactivate.go::cmdDeactivateImpl` | `test/parse/cli-config-deactivate-leaf.ci` |
| `ze config deactivate <file> <path-to-container>` | -> | same `cmdDeactivateImpl`, container branch | `test/parse/cli-config-deactivate-container.ci` |
| `ze config activate <file> <path>` | -> | `cmd/ze/config/cmd_deactivate.go::cmdActivateImpl` | `test/parse/cli-config-activate.ci` |
| `inactive: <leaf> <value>;` in conf | -> | `parser.go` leaf-inactive branch -> `Tree.SetLeafInactive` | `internal/component/config/parser_inactive_leaf_test.go::TestParseInactiveLeafTopLevel` |
| Apply path on file with `inactive: <leaf>` | -> | `PruneInactive` leaf branch | `internal/component/config/prune_inactive_leaf_test.go::TestPruneInactiveLeaf` |
| Round-trip parse + serialize with leaf inactive | -> | parser + `serializeNode` LeafNode case | `internal/component/config/serialize_inactive_leaf_test.go::TestRoundTripInactiveLeaf` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze config deactivate myconf.conf bgp router-id` (leaf) | Exits 0; file rewritten with `inactive: router-id <value>;`; subsequent load returns `(value, false)` from `tree.Get("router-id")` after `PruneInactive` |
| AC-2 | `ze config deactivate myconf.conf bgp` (container) | Same observable behavior as the existing TUI `deactivate bgp` -- container's `inactive` leaf set to `true`; on serialize, prefixed with `inactive: ` |
| AC-3 | `ze config deactivate myconf.conf bgp filter import no-self-as` (leaf-list value) | `inactive:` prefix added to value via existing `DeactivateMultiValue`; existing `.ci` for filter chains continues to pass |
| AC-4 | `ze config activate ...` on any of AC-1/2/3 | Reverses the deactivation; tree state matches pre-deactivate |
| AC-5 | Loaded config with `inactive: router-id 1.2.3.4;` | After `PruneInactive`, `tree.Get("router-id")` returns `("", false)` |
| AC-6 | Loaded config with deactivated container | Unchanged from today: subtree absent at apply (regression-guarded by existing prune tests) |
| AC-7 | `ze config deactivate <file> <bad-path>` | Exits non-zero with "no such path" stderr; file unmodified |
| AC-8 | `ze config deactivate` on already-inactive node | Exits 0 with "already deactivated" status; file unchanged |
| AC-9 | Parse `inactive: router-id 1.2.3.4;` then serialize | Output structurally equivalent to input: re-parsing the serialized output yields a Tree equal to the original Tree (same `values`, `inactiveValues`, `containers`, `lists`). Whitespace / blank-line preservation NOT required -- existing serializer behavior. |
| AC-10 | Grep YANG modules for `ze:deactivable` or similar new extension | Returns nothing -- no new schema annotation introduced |
| AC-11 | TUI `deactivate bgp router-id` (the previously-rejected case) | Now succeeds; status line says "Deactivated bgp router-id"; previously errored "cannot deactivate a leaf value" |
| AC-12 | `ze config deactivate` on a positional list entry (`nlri`, `nexthop`, `add-path`) | Rejected with a clear error pointing the user at the parent container -- preserves the existing positional-list constraint |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSetLeafInactive` | `internal/component/config/tree_inactive_test.go` | Tree.SetLeafInactive/IsLeafInactive round-trip | Done |
| `TestSetLeafInactiveUnknownLeaf` | `internal/component/config/tree_inactive_test.go` | pre-mark of an absent leaf permitted | Done |
| `TestClearLeafInactive` | `internal/component/config/tree_inactive_test.go` | ClearLeafInactive removes the marker | Done |
| `TestCloneLeafInactive` | `internal/component/config/tree_inactive_test.go` | inactive marker survives Clone, isolated from original | Done |
| `TestParseInactiveLeafTopLevel` | `internal/component/config/parser_inactive_leaf_test.go` | parseRoot accepts `inactive: <leaf> <value>` | Done |
| `TestParseInactiveLeafInListEntry` | `internal/component/config/parser_inactive_leaf_test.go` | leaf inside list entry block gets marker | Done |
| `TestParseInactiveLeafInContainer` | `internal/component/config/parser_inactive_leaf_test.go` | leaf inside nested container gets marker | Done |
| `TestSerializeInactiveLeaf` | `internal/component/config/serialize_inactive_leaf_test.go` | leaf emits `inactive: ` prefix | Done |
| `TestSerializeActiveLeafNotPrefixed` | `internal/component/config/serialize_inactive_leaf_test.go` | active leaf gets no prefix | Done |
| `TestRoundTripInactiveLeaf` | `internal/component/config/serialize_inactive_leaf_test.go` | parse -> serialize -> parse fixed point | Done |
| `TestRoundTripInactiveLeafInListEntry` | `internal/component/config/serialize_inactive_leaf_test.go` | round-trip inside list entry | Done |
| `TestPruneInactiveLeaf` | `internal/component/config/prune_inactive_leaf_test.go` | leaf removed from values; marker cleared | Done |
| `TestPruneInactiveLeafInListEntry` | `internal/component/config/prune_inactive_leaf_test.go` | recursion into list entries | Done |
| `TestPruneInactiveLeafInsideInactiveContainer` | `internal/component/config/prune_inactive_leaf_test.go` | container-level prune wins over leaf prune | Done |
| `TestEditorDeactivateLeaf` | `internal/component/cli/editor_inactive_leaf_test.go` | wrapper sets leaf inactive | Done |
| `TestEditorActivateLeafSymmetric` | `internal/component/cli/editor_inactive_leaf_test.go` | activate undoes deactivate | Done |
| `TestEditorDeactivateLeafIdempotentReject` | `internal/component/cli/editor_inactive_leaf_test.go` | second call surfaces ErrLeafAlreadyInactive | Done |
| `TestEditorDeactivateLeafPermissive` | `internal/component/cli/editor_inactive_leaf_test.go` | absent leaf accepted (engine-level rule) | Done |
| `TestEditorDeactivateLeafBadParent` | `internal/component/cli/editor_inactive_leaf_test.go` | non-existent parent rejected with ErrPathNotFound | Done |
| `TestEditorDeactivatePathRejectsMissing` | `internal/component/cli/editor_inactive_leaf_test.go` | strict path helper rejects missing structural path | Done |
| `TestEditorActivatePathIdempotent` | `internal/component/cli/editor_inactive_leaf_test.go` | activate on already-active surfaces ErrPathNotInactive | Done |
| `TestModelDeactivateLeaf` | `internal/component/cli/model_commands_inactive_leaf_test.go` | TUI deactivate on a leaf path succeeds | Done |
| `TestModelActivateLeaf` | `internal/component/cli/model_commands_inactive_leaf_test.go` | TUI activate symmetric | Done |
| `TestCmdDeactivateLeaf` | `cmd/ze/config/cmd_deactivate_test.go` | CLI deactivate of a leaf, end-to-end | Done |
| `TestCmdDeactivateContainer` | `cmd/ze/config/cmd_deactivate_test.go` | CLI deactivate of a list entry | Done |
| `TestCmdDeactivateLeafListValue` | `cmd/ze/config/cmd_deactivate_test.go` | CLI deactivate of a leaf-list value | Done |
| `TestCmdActivateRoundTrip` | `cmd/ze/config/cmd_deactivate_test.go` | activate undoes deactivate end-to-end | Done |
| `TestCmdDeactivateBadPath` | `cmd/ze/config/cmd_deactivate_test.go` | exit non-zero, file unchanged | Done |
| `TestCmdDeactivateAlreadyInactive` | `cmd/ze/config/cmd_deactivate_test.go` | idempotent exit 0 (AC-8) | Done |
| `TestCmdDeactivateMissingArgs` | `cmd/ze/config/cmd_deactivate_test.go` | usage error on missing args | Done |
| `TestCmdDeactivatePositionalListEntry` | `cmd/ze/config/cmd_deactivate_test.go` | positional list entry rejected with parent-pointer (AC-12) | Done |
| `TestSetParseInactiveLeaf` | `internal/component/config/setparser_inactive_test.go` | set-format parses `inactive <path>` for leaves | Done |
| `TestSetParseInactiveContainer` | `internal/component/config/setparser_inactive_test.go` | set-format parses `inactive <path>` for containers | Done |
| `TestSetSerializeInactiveLeaf` | `internal/component/config/setparser_inactive_test.go` | serializer emits `inactive <path>` line | Done |
| `TestSetSerializeInactiveContainer` | `internal/component/config/setparser_inactive_test.go` | container deactivation rendered as inactive line | Done |
| `TestSetRoundTripInactiveLeaf` | `internal/component/config/setparser_inactive_test.go` | set-format round-trip preserves marker | Done |
| `TestSetParseUnknownVerb` | `internal/component/config/setparser_inactive_test.go` | error message lists supported verbs | Done |
| `TestSetParseRejectsActivateKeyword` | `internal/component/config/setparser_inactive_test.go` | single-keyword design: no activate verb | Done |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cli-config-deactivate-leaf.ci` | `test/parse/` | User runs `ze config deactivate`, file gets prefix, validate passes | Done |
| `cli-config-deactivate-container.ci` | `test/parse/` | Same, container path -- regression guard | Done |
| `cli-config-activate.ci` | `test/parse/` | User reverses deactivation; effect observable | Done |
| `parse-inactive-leaf.ci` | `test/parse/` | Config containing `inactive: <leaf>` loads cleanly | Done |

### Boundary Tests
N/A -- no numeric inputs.

## Files to Modify

- `internal/component/config/tree.go` -- add `inactiveValues map[string]bool` field, `SetLeafInactive` / `IsLeafInactive` / `ClearLeafInactive` methods.
- `internal/component/config/parser.go:229-276` -- replace warning branch with `SetLeafInactive` call.
- `internal/component/config/parser_list.go:90-126` -- same.
- `internal/component/config/serialize.go:255-268` -- emit `inactive: ` prefix on leaves when `IsLeafInactive`.
- `internal/component/config/serialize_annotated.go` -- same.
- `internal/component/config/serialize_blame.go` -- same.
- `internal/component/config/prune.go` -- extend `pruneNode` and `pruneActiveNode` to also handle leaves.
- `internal/component/cli/editor_commands.go` -- add `DeactivateLeaf` / `ActivateLeaf` wrappers.
- `internal/component/cli/model_commands.go:510-629` -- replace leaf rejection at lines 549, 609 with new Editor calls; refine status messages.
- `cmd/ze/config/main.go` -- register new `deactivate` / `activate` subcommands.
- `cmd/ze/config/register.go` (if used) -- same registration.
- `docs/architecture/config/syntax.md` -- specify `inactive: <name> [value];` grammar formally.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No (transparent per principle) | -- |
| CLI commands/flags | Yes | `cmd/ze/config/cmd_deactivate.go`, `cmd_activate.go`, `main.go` |
| Editor autocomplete | Automatic via existing schema walk -- the new verbs share `cli.NewCompleter` | `internal/component/cli/...` |
| Functional test for new CLI | Yes | `test/config/cli-deactivate-*.ci` |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | Yes | `docs/architecture/config/syntax.md`, new `docs/guide/config-deactivate.md` |
| 3 | CLI command added? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC changed? | No | -- |
| 6 | Has a user guide page? | Yes | `docs/guide/config-deactivate.md` (new) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/syntax.md` (formalise `inactive:`) |

## Files to Create
- `cmd/ze/config/cmd_deactivate.go` (also handles activate via `runDeactivateLike(activate=true)` -- one file for the symmetric verb pair)
- `cmd/ze/config/cmd_deactivate_test.go`
- `internal/component/config/tree_inactive_test.go`
- `internal/component/config/parser_inactive_leaf_test.go`
- `internal/component/config/serialize_inactive_leaf_test.go`
- `internal/component/config/prune_inactive_leaf_test.go`
- `internal/component/config/setparser_inactive_test.go`
- `internal/component/cli/editor_inactive_leaf_test.go`
- `internal/component/cli/model_commands_inactive_leaf_test.go`
- `test/parse/cli-config-deactivate-leaf.ci`
- `test/parse/cli-config-deactivate-container.ci`
- `test/parse/cli-config-activate.ci`
- `test/parse/parse-inactive-leaf.ci`
- `docs/guide/config-deactivate.md`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-verify` |
| 6. Critical review | Critical Review Checklist |
| 7-9. Fix + re-verify | as per template |
| 10. Deliverables review | Deliverables Checklist |
| 11. Security review | Security Review Checklist |
| 13. Present summary | Executive Summary |

### Implementation Phases

1. **Phase: Tree leaf-inactive state** -- add `inactiveValues map[string]bool` and `SetLeafInactive` / `IsLeafInactive` / `ClearLeafInactive` (lock-respecting). Tests: `TestSetLeafInactive`. Files: `tree.go`.
2. **Phase: Parser leaf branch** -- replace warning at `parser.go:274` and `parser_list.go:123` with `SetLeafInactive` call. Tests: `TestParseInactiveLeaf`, `TestParseInactiveLeafInListEntry`.
3. **Phase: Serializer leaf branch** -- emit `inactive: ` prefix on leaves in `serialize.go` (and annotated/blame variants). Tests: `TestSerializeInactiveLeaf`, `TestRoundTripInactiveLeaf`.
4. **Phase: Prune leaf branch** -- extend `pruneNode` / `pruneActiveNode` to handle leaves. Tests: `TestPruneInactiveLeaf`, `TestPruneInactiveLeafInsideInactiveContainer`.
5. **Phase: Editor wrappers** -- `DeactivateLeaf` / `ActivateLeaf` in `editor_commands.go`. Tests: `TestEditorDeactivateLeaf`.
6. **Phase: TUI rejection lift** -- replace `model_commands.go:549,609` rejections; preserve positional-list rejection with refined message. Tests: `TestModelDeactivateLeafSucceeds`, `TestModelDeactivatePositionalListEntryFails`.
7. **Phase: CLI verbs** -- `cmd_deactivate.go`, `cmd_activate.go`, register in `main.go`. Mirror `cmd_set.go` (flag parsing, NewEditorWithStorage, validation, dry-run, save, daemon notify). Tests: `TestCmdDeactivate_*`, `TestCmdActivate_Symmetric`.
8. **Phase: Functional tests** -- four `.ci` files exercising end-to-end paths.
9. **Phase: Docs** -- update `docs/architecture/config/syntax.md`; create `docs/guide/config-deactivate.md`; update `docs/features.md` and `docs/guide/command-reference.md`.
10. **Full verification** -- `make ze-verify`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Round-trip test passes for leaf, container, list-entry, leaf-list value |
| Naming | `inactiveValues` field, `SetLeafInactive` method follow Tree's existing conventions; CLI verbs are kebab-case (`deactivate`, `activate`) consistent with `set` |
| Data flow | leaf inactivity lives in `internal/component/config` only; no schema, no plugin, no protocol code touched |
| Rule: no-layering | Existing TUI rejection paths fully replaced, not bypassed |
| Rule: engine-level principle | No new YANG extension introduced; grep verifies absence |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `cmd/ze/config/cmd_deactivate.go` exists | `ls cmd/ze/config/cmd_deactivate.go` |
| `cmd/ze/config/cmd_activate.go` exists | `ls cmd/ze/config/cmd_activate.go` |
| Tree.SetLeafInactive callable | `go test -run TestSetLeafInactive ./internal/component/config/` |
| `inactive: <leaf> <value>;` round-trips | `go test -run TestRoundTripInactiveLeaf ./internal/component/config/` |
| CLI verb works end-to-end | `go test -run TestCmdDeactivate ./cmd/ze/config/` |
| Functional tests pass | `make ze-functional-test` |
| No new YANG annotation | `grep -r "ze:deactivable" internal/component/*/schema/ -- returns no matches` |
| Docs page exists | `ls docs/guide/config-deactivate.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | path resolution rejects bad paths before mutation; reuse `completer.validateTokenPath` |
| Error leakage | error messages do not leak filesystem paths beyond what `cmd_set.go` already prints |
| File overwrite | Save uses existing Editor backup machinery (versioning) -- no new on-disk path |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Deactivate would need to be built from scratch | Ze already has `inactive:` end-to-end for containers, list entries, and leaf-list values, wired into apply via `PruneInactive`. Only leaves and one-shot CLI verbs are missing. | Spec SCOPE search of `plan/learned/` and grep on `inactive:` | Reframed spec from "design from scratch" to "extend leaves + add CLI verbs". |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Encode leaf inactivity as `"inactive:" + value` in `Tree.values[name]` | Risk of value collision (real value could legitimately start with `inactive:`); no symmetry with how containers store the marker (separate field, not embedded). | Sibling `inactiveValues map[string]bool` on the parent Tree -- value untouched, structurally symmetric with container marker (which lives in its own values map under the auto-injected `inactive` leaf). |
| Add `ze:deactivable` YANG extension | Violates SCOPE-gate constraint: "engine-level, transparent to schema authors, every node deactivatable." | Universal mechanism in `internal/component/config` only. |

## Design Insights

- The "engine-level, automatic, transparent" principle has a concrete implementation rule: **state lives in `Tree`, not in the schema.** Containers were partially compatible (auto-injection) but still polluted the schema with an `inactive` leaf. Whether to migrate containers to the same Tree-level scheme later is deferred (see plan/deferrals.md) for this spec -- noted as a future refactor candidate.
- The positional-list exception (lists with all-leaf children skip `inactive` injection) becomes more visible under the universal-leaf rule: under this spec, leaves *inside* such lists are individually deactivatable. AC-12 explicitly preserves the rule that the *list entry itself* is not.
- The set / single-line format and the block format use **different idioms for the same concept**: block uses an inline `inactive: ` prefix on the structural statement, set uses a separate `inactive <path>` line. The set form is single-keyword (no `activate` counterpart in the file syntax) since absence of the line means active.

## Implementation Summary

### What Was Implemented
- `Tree.inactiveValues map[string]bool` + `SetLeafInactive` / `IsLeafInactive` / `ClearLeafInactive` (lock-respecting, included in Clone, included in TreeEqual).
- Parser `applyInactive` helper, called from parseRoot, parseContainer, and parseListFieldBlock; lifts the leaf-rejection warning at three sites.
- Serializer leaf-inactive prefix emission in serialize.go, serialize_annotated.go, serialize_blame.go (all four leaf node types).
- `canInlineContainer` refuses inline when any child leaf is inactive.
- `PruneInactive` extended via `Tree.pruneInactiveLeaves()` invoked at the start of `pruneNode`.
- `Editor.DeactivateLeaf` / `ActivateLeaf` wrappers; `Editor.WalkPathWithSchema` (exported); new `Editor.LookupSchemaNode` (terminal-leaf classification); `Editor.ResolveLeafListValue` (extracted from Model).
- TUI `cmdDeactivate` / `cmdActivate` route leaf paths to the new Editor methods.
- One-shot CLI verbs `ze config deactivate` / `ze config activate` with AC-8 idempotent exit-0 sentinel and AC-12 positional-list rejection.
- Set-format gets new top-level keyword `inactive <path>` (single-keyword design). Setparser parses it; serializer emits it; legacy `set <path> inactive true` no longer emitted (still parsed for backward compat).
- Functional `.ci` tests, user docs, syntax-doc section, command-reference and features-list entries.

### Bugs Found/Fixed
- AC-12 test path -- `capability` nests inside `session`, not directly under `peer`. Fixed.
- TreeEqual did not compare `inactiveValues`; round-trip tests would have given false positives. Extended TreeEqual.
- Set-format `extra values` path leaked the engine's `inactive` marker for trees whose schema lacked the auto-injected leaf. Suppressed explicitly.

### Documentation Updates
- `docs/guide/config-deactivate.md` (new)
- `docs/architecture/config/syntax.md` (Inactive prefix section + set-format keyword)
- `docs/features.md`, `docs/guide/command-reference.md`

### Deviations from Plan
- Spec proposed Junos-style `deactivate <path>` / `activate <path>` keywords for the set format. Per user feedback, simplified to single `inactive <path>` keyword (no symmetric `activate` in file syntax). CLI imperatives `ze config deactivate` / `ze config activate` retained.
- Added `Editor.LookupSchemaNode` -- not in original spec, needed because `WalkPathWithSchema` halts at leaves.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Engine-level extension to leaves (no YANG annotation) | Done | `internal/component/config/tree.go` (inactiveValues field), `parser.go` (applyInactive) | universal across LeafNode / MultiLeafNode / BracketLeafListNode / ValueOrArrayNode / FlexNode |
| One-shot CLI verbs `ze config deactivate` / `activate` | Done | `cmd/ze/config/cmd_deactivate.go` | shared `runDeactivateLike` for both verbs |
| User-facing documentation | Done | `docs/guide/config-deactivate.md` (new), `docs/architecture/config/syntax.md` (Inactive prefix section), `docs/features.md`, `docs/guide/command-reference.md` | -- |
| Set-format `inactive <path>` keyword (added per user feedback) | Done | `internal/component/config/setparser.go` (cmdInactive, parseInactive), `serialize_set.go` (emitSetInactive, emitSetInactiveStructural) | single-keyword design, no symmetric activate verb |

### Acceptance Criteria
| AC ID | Status | Demonstrated By |
|-------|--------|-----------------|
| AC-1 | Done | `TestCmdDeactivateLeaf` (`cmd/ze/config/cmd_deactivate_test.go`) |
| AC-2 | Done | `TestCmdDeactivateContainer` |
| AC-3 | Done | `TestCmdDeactivateLeafListValue` |
| AC-4 | Done | `TestCmdActivateRoundTrip`; `TestEditorActivateLeafSymmetric` |
| AC-5 | Done | `TestPruneInactiveLeaf` |
| AC-6 | Done | `TestPruneInactiveContainer` (existing) + `TestPruneInactiveLeafInsideInactiveContainer` |
| AC-7 | Done | `TestCmdDeactivateBadPath` |
| AC-8 | Done | `TestCmdDeactivateAlreadyInactive` (idempotent exit 0) |
| AC-9 | Done | `TestRoundTripInactiveLeaf`, `TestRoundTripInactiveLeafInListEntry`, `TestSetRoundTripInactiveLeaf` |
| AC-10 | Done | `grep -r "ze:deactivable" internal/component/*/schema/` returns no matches |
| AC-11 | Done | `TestModelDeactivateLeaf` |
| AC-12 | Done | `TestCmdDeactivatePositionalListEntry` |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestSetLeafInactive` | Done | `internal/component/config/tree_inactive_test.go` | + 3 sibling tests |
| `TestParseInactiveLeafTopLevel` | Done | `internal/component/config/parser_inactive_leaf_test.go` | + 2 nested-context tests |
| `TestSerializeInactiveLeaf` | Done | `internal/component/config/serialize_inactive_leaf_test.go` | + round-trip tests |
| `TestPruneInactiveLeaf` | Done | `internal/component/config/prune_inactive_leaf_test.go` | + 2 nested-context tests |
| `TestEditorDeactivateLeaf` | Done | `internal/component/cli/editor_inactive_leaf_test.go` | + 6 sibling tests including idempotency and bad-path |
| `TestModelDeactivateLeaf` | Done | `internal/component/cli/model_commands_inactive_leaf_test.go` | + activate symmetric |
| `TestCmdDeactivateLeaf` | Done | `cmd/ze/config/cmd_deactivate_test.go` | + 7 sibling tests covering containers, leaf-list values, AC-7/8/12 |
| `TestSetParseInactiveLeaf` | Done | `internal/component/config/setparser_inactive_test.go` | + 6 sibling tests for set-format |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/config/tree.go` | Modified | inactiveValues field + 3 methods + Clone update + pruneInactiveLeaves |
| `internal/component/config/parser.go` | Modified | parseRoot inactive: sugar + applyInactive helper |
| `internal/component/config/parser_list.go` | Modified | replace warning with applyInactive call |
| `internal/component/config/serialize.go` | Modified | leaf prefix emission + canInlineContainer guard + extra-values prefix |
| `internal/component/config/serialize_annotated.go` | Modified | leaf prefix emission (annotated view) |
| `internal/component/config/serialize_blame.go` | Modified | leaf prefix emission (blame view) |
| `internal/component/config/prune.go` | Modified | pruneNode calls Tree.pruneInactiveLeaves |
| `internal/component/config/serialize_test.go` | Modified | TreeEqual extension to compare inactiveValues |
| `internal/component/config/setparser.go` | Modified | cmdInactive + parseInactive (with pre-migration support) |
| `internal/component/config/serialize_set.go` | Modified | emitSetInactive + emitSetInactiveStructural; suppress legacy |
| `internal/component/cli/editor.go` | Modified | WalkPathWithSchema, LookupSchemaNode, ResolveLeafListValue |
| `internal/component/cli/editor_commands.go` | Modified | DeactivateLeaf/ActivateLeaf, DeactivatePath/ActivatePath, sentinel errors |
| `internal/component/cli/model_commands.go` | Modified | runActivation factored helper |
| `cmd/ze/config/cmd_deactivate.go` | New | one-shot CLI verb (handles both deactivate and activate) |
| `cmd/ze/config/cmd_deactivate_test.go` | New | 8 CLI tests |
| `cmd/ze/config/main.go` | Modified | register handlers + help |
| `cmd/ze/config/register.go` | Modified | Subs listing |
| `docs/guide/config-deactivate.md` | New | user guide |
| `docs/features.md` | Modified | feature entry |
| `docs/guide/command-reference.md` | Modified | CLI verbs |
| `docs/architecture/config/syntax.md` | Modified | inactive prefix grammar + set-format keyword |
| `internal/component/config/{tree,parser,serialize,prune,setparser}_inactive*_test.go` | New | 5 unit-test files (21 tests) |
| `internal/component/cli/{editor,model_commands}_inactive_leaf_test.go` | New | 2 unit-test files (9 tests) |
| `test/parse/cli-config-deactivate-leaf.ci`, `cli-config-deactivate-container.ci`, `cli-config-activate.ci`, `parse-inactive-leaf.ci` | New | functional tests |
| `plan/learned/654-config-3-deactivate.md` | New | learned summary |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `cmd/ze/config/cmd_deactivate.go` | Yes | `ls -la cmd/ze/config/cmd_deactivate.go` -> 223 lines |
| `cmd/ze/config/cmd_deactivate_test.go` | Yes | `ls -la cmd/ze/config/cmd_deactivate_test.go` -> 8 tests |
| `docs/guide/config-deactivate.md` | Yes | `ls -la docs/guide/config-deactivate.md` |
| `internal/component/config/tree_inactive_test.go` | Yes | `ls -la` |
| `internal/component/config/parser_inactive_leaf_test.go` | Yes | `ls -la` |
| `internal/component/config/serialize_inactive_leaf_test.go` | Yes | `ls -la` |
| `internal/component/config/prune_inactive_leaf_test.go` | Yes | `ls -la` |
| `internal/component/config/setparser_inactive_test.go` | Yes | `ls -la` |
| `internal/component/cli/editor_inactive_leaf_test.go` | Yes | `ls -la` |
| `internal/component/cli/model_commands_inactive_leaf_test.go` | Yes | `ls -la` |
| `test/parse/cli-config-deactivate-leaf.ci` | Yes | `ls -la test/parse/cli-config-*.ci` |
| `test/parse/cli-config-deactivate-container.ci` | Yes | `ls -la` |
| `test/parse/cli-config-activate.ci` | Yes | `ls -la` |
| `test/parse/parse-inactive-leaf.ci` | Yes | `ls -la` |
| `plan/learned/654-config-3-deactivate.md` | Yes | `ls -la plan/learned/654-*` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | CLI deactivate of a leaf rewrites file with inactive prefix | `go test -run TestCmdDeactivateLeaf ./cmd/ze/config/` PASS |
| AC-2 | CLI deactivate of a list entry sets inactive flag | `go test -run TestCmdDeactivateContainer ./cmd/ze/config/` PASS |
| AC-3 | CLI deactivate of a leaf-list value adds prefix | `go test -run TestCmdDeactivateLeafListValue ./cmd/ze/config/` PASS |
| AC-4 | activate undoes deactivate | `go test -run TestCmdActivateRoundTrip ./cmd/ze/config/` PASS |
| AC-5 | inactive leaf absent after PruneInactive | `go test -run TestPruneInactiveLeaf ./internal/component/config/` PASS |
| AC-6 | inactive container absent at apply | `go test -run TestPruneInactiveContainer ./internal/component/config/` PASS |
| AC-7 | bad-path exits non-zero, file unchanged | `go test -run TestCmdDeactivateBadPath ./cmd/ze/config/` PASS |
| AC-8 | already-inactive exits 0 idempotently | `go test -run TestCmdDeactivateAlreadyInactive ./cmd/ze/config/` PASS |
| AC-9 | parse->serialize->parse round-trip | `go test -run TestRoundTripInactiveLeaf ./internal/component/config/` PASS |
| AC-10 | no `ze:deactivable` schema annotation | `grep -rn 'ze:deactivable' internal/component/*/schema/` -> no output |
| AC-11 | TUI deactivate on a leaf no longer errors | `go test -run TestModelDeactivateLeaf ./internal/component/cli/` PASS |
| AC-12 | positional list entry rejected with parent-pointer | `go test -run TestCmdDeactivatePositionalListEntry ./cmd/ze/config/` PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze config deactivate <file> <leaf-path>` | `test/parse/cli-config-deactivate-leaf.ci` | Yes -- runs `ze config deactivate ... bgp router-id` then `ze config validate`, both exit 0 |
| `ze config deactivate <file> <list-entry-path>` | `test/parse/cli-config-deactivate-container.ci` | Yes -- runs deactivate on `bgp peer peer1`, then validate |
| `ze config activate <file> <path>` | `test/parse/cli-config-activate.ci` | Yes -- deactivate then activate then validate (round-trip) |
| `inactive: <leaf> <value>;` parse | `test/parse/parse-inactive-leaf.ci` | Yes -- config containing `inactive: router-id 10.0.0.1` validates clean |

### Make Targets
| Check | Evidence |
|-------|----------|
| `make ze-lint` | exit 0, "0 issues." (`tmp/test/lint-final3.log`) |
| `make ze-unit-test` | exit 0, no FAIL lines (`tmp/test/unit-final.log`) |
| `make ze-functional-test` | exit 0, "58 passed, 0 failed", "PASS all 11 suites" (`tmp/test/func3.log`) |

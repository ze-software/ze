# Spec: config-list-entries-operator-order

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Priority | P2 |
| Release reason | The operator's own config prints in a different order from the file the editor saves: the set form keeps the recorded entry order while the hierarchical form (`show`, `ze config fmt`, `ze config migrate format hierarchical`) sorts it, and the annotated diff gutter sorts even `ordered-by user` lists. A printer that reorders what the operator wrote is shipped configuration behavior. |
| RFC gaps | - |
| Depends | - |
| Phase | 1/5 |
| Handoff | - |
| Updated | 2026-10-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Implement the statement-order part of the owner decision recorded on Q-2 of
`plan/post-release/engineering/spec-ze-config-fmt.md` (Thomas, 2026-10-28, the
Junos model):

| Part of the decision | Owner text | Owned by |
|----------------------|------------|----------|
| Leaf and container order | Leaves and containers inside a block print in YANG schema order | `plan/release/fixes/spec-config-yang-definition-order.md` (owner decision D-1) |
| Reordering list entries | A Junos-style `insert <list> <entry> before|after <entry>` | `plan/post-release/features/spec-config-list-entry-insert.md` (owner decision D-2) |
| Order-only differences | A reorder of a list that is not `ordered-by user` is no change; a reorder of an `ordered-by user` list is one | This spec (owner decision D-4, AC-7 and AC-12) |
| List entry order | The entries of every list print in the order the tree recorded (operator order), never sorted by key | This spec (Phases 2 and 3) |
| `ordered-by user` | Already keeps its order (`listBlockKeys`) | Preserved by this spec |
| Blank lines and comment grouping | Blank lines are not kept, and a comment above a group stays with its first statement | The parent spec `plan/post-release/engineering/spec-ze-config-fmt.md` (comment association, `parser_comments.go`), not this spec |

The symptom today: an operator who writes `peer zulu` before `peer alpha` sees
the two swapped by `show`, `ze config fmt`, the web CLI `show`, and `ze config
migrate format hierarchical`, while `show | display set`, the annotated and blame
views and the committed file keep the order written. Two printers of one tree
disagree. The goal is one rule for every printer: a list's entries print in the
order the tree recorded.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - documents the printer's list order (section ending "Every other list it writes sorted by key")
  → Constraint: the paragraph naming `listBlockKeys` as sorting every non-`ordered-by user` list becomes false with this change and is rewritten in the same phase (`ai/rules/documentation.md`)
  → Constraint: `ordered-by user` lists are read with `configorder.Entries`; unordered lists with `configvalue.ListEntries`. Neither reader changes: this spec changes printers only
  → Decision: `ToMap` carries no order sidecar and `ToPluginMap` carries one for every list of two or more entries (`entryOrderLocked`); both stay as they are
- [ ] `plan/post-release/engineering/spec-ze-config-fmt.md` - parent; Q-2 holds the owner decision
  → Constraint: comment association and blank-line handling stay with the parent; this spec must not touch `parser_comments.go`
  → Constraint: the parent's corpus gate (`TestFormatCorpusRoundTrip`) is the round-trip proof this spec extends, not replaces
- [ ] `docs/architecture/config/yang-config-design.md` - declared by a file this spec changes (`// Design:` header)
  → Constraint: the page states no child order and no list-entry order today (grep for order, alphabetical, sorted); this spec changes no schema child order, so the page is unaffected here (the child-order sentence belongs to `spec-config-yang-definition-order.md`)
- [ ] `docs/contributing/ze-go-style.md` - precondition for the implementing agent's first Go edit

**Key insights:**
- The Tree already records insertion order for every list (`Tree.listOrder`, appended by `AddListEntry`); only the hierarchical printer and the Flex/InlineList paths ignore it.
- The set, annotated and blame printers already print `listOrder` for plain lists, but sort when it is empty and silently DROP an entry `listOrder` does not name.
- Container children are ordered alphabetically by the YANG-to-schema converter (`flattenChildren` uses `sortedKeys`); only list children use YANG definition order (`yangChildOrder`). That work is `plan/release/fixes/spec-config-yang-definition-order.md`, not this spec.
- The editor has no command that reorders list entries; `plan/post-release/features/spec-config-list-entry-insert.md` adds one.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/serialize.go` - hierarchical printer
  → Constraint: `listBlockKeys` prints recorded order only when `node.UserOrdered`, then the unrecorded rest sorted; every other list sorted (`slices.Sort(rest)`)
  → Constraint: `serializeListMultiBlock` uses the recorded order when non-empty, else sorts
  → Constraint: `serializeNode` FlexNode branch sorts list entries unconditionally; InlineListNode branch sorts unconditionally
  → Constraint: `serializeFreeform` and `serializeExtraValues` sort values by name (these are values, not list entries; see D-3)
- [ ] `internal/component/config/flatten.go` - `ze:flatten` printer calls `serializeListBlocks` with `tree.listOrder`, so it inherits `listBlockKeys`
- [ ] `internal/component/config/serialize_set.go` - set printers
  → Constraint: `serializeSetList` and `serializeSetMetaList` print `listOrder` when non-empty, else sort; an entry missing from a non-empty `listOrder` is not printed at all (latent configuration loss in the committed set-meta file)
  → Constraint: `serializeSetFlex`, `serializeSetInlineList`, `writeFlexLines`, `writeInlineListLines` sort list entries unconditionally
- [ ] `internal/component/config/serialize_annotated.go` - `serializeAnnotatedList`, `serializeAnnotatedSetList` use `listOrder` else sort; `serializeAnnotatedFlex`, `serializeAnnotatedInlineList`, `serializeAnnotatedSetFlex`, `serializeAnnotatedSetInlineList` sort unconditionally
- [ ] `internal/component/config/serialize_blame.go` - `serializeBlameList` uses `listOrder` else sort; `serializeBlameFlex`, `serializeBlameInlineList` sort; `sortedEntryKeys`/`sortedMapKeys` sort meta maps
- [ ] `internal/component/config/tree.go` - Tree
  → Constraint: `AddListEntry` appends to `listOrder` (duplicate keys get a `#N` suffix); `RenameListEntry` keeps the position; `CopyListEntry` inserts the copy right after the source; `RemoveListEntry` drops the key; `mergeFrom` appends keys new to the receiver in the other tree's order; `Clone` copies `listOrder`
  → Constraint: `GetList` returns the live map, so a caller can add an entry without `listOrder`; `entryOrderLocked` refuses (returns nil) rather than inventing an order — the printers must make the same distinction
  → Constraint: `ListKeys` returns map order (random); `GetListOrdered` returns recorded order
- [ ] `internal/component/config/parser_list.go`, `parser_freeform.go`, `setparser.go`, `setparser_meta.go`, `setparser_inline.go`, `list_key.go`, `plugin_map.go` - every Tree builder goes through `AddListEntry`; `TreeFromPluginMap` restores order only from a delivered order key (single-entry lists get a one-key order)
- [ ] `internal/component/config/prune.go` - `PruneUnchanged` compares `serializeNode` text per child, recursing into list entries via `SerializeSubtree`; a list whose entries are equal but recorded in another order has different text at the list level and is recursed into
- [ ] `internal/component/cli/diff_tree.go` - `diffList` unions both sides' keys and sorts them, for every list including `ordered-by user`; `renderDiffLines` maps diff lines onto the working text's statement lines, so a diff order that differs from the printed order misplaces the gutter
- [ ] `internal/component/cli/editor_commit.go`, `editor_draft.go` - the committed file and drafts are set-with-meta; draft replay applies rename and copy with the Tree methods (position kept)
- [ ] `internal/component/gnmi/get.go` - `treeToTypedValue` marshals `ToMap` with `encoding/json`, which writes object keys sorted; lists are JSON objects keyed by entry key
- [ ] `internal/component/web/handler_config_walk.go` - `collectListKeys` reads `GetListOrdered`: the web editor tree already shows recorded order
- [ ] `internal/component/config/cli/cmd_diff.go` - `ze config diff` is `DiffMaps` over `ToMap`, path-keyed and sorted by path
  → Constraint: `ToMap` carries no order sidecar, so today a reorder of an `ordered-by user` list is invisible to `ze config diff`; D-4 makes it a change, so Phase 3 adds an entry-order comparison for `ordered-by user` lists (read through `configorder.Entries` or `GetListOrdered`) and leaves every other list order-blind
- [ ] `internal/component/config/archive/archive.go` `ChangeTracker.HasChanged`, `internal/component/config/storage/history.go` `contentDigest` - SHA-256 of file bytes
- [ ] `internal/component/config/yang_schema.go` - `yangToContainer` orders children by `flattenChildren` (alphabetical `sortedKeys`); `yangToList` uses `yangChildOrder`
  → Constraint: not changed by this spec; child order is `spec-config-yang-definition-order.md`
- [ ] `internal/component/cli/editor_commands.go` - `InsertLeafListValue` is the only `insert`; there is no list-entry move or insert
  → Constraint: not changed by this spec; the list-entry `insert` is `spec-config-list-entry-insert.md`, which depends on this spec
- [ ] `internal/exabgp/migration/migrate_serialize.go` - `SerializeTree` is a hand-written printer: lists via `GetListOrdered` (recorded order), values sorted by name; out of scope

**Behavior to preserve:**
- `ordered-by user` lists print in recorded order (`TestConfigFmtKeepsUserOrderedEntries`).
- An entry the recorded order does not name is printed, never dropped.
- `ToMap`, `ToPluginMap`, `configorder.Entries` and every consumer of them are untouched.
- Formatting is idempotent: `fmt` of `fmt` output changes no byte.
- Secret masking (`MaskSecrets`, `encodeSecrets`) and the schema stamp are unaffected.

**Behavior to change:**
- Every printer prints list entries in recorded order, for every list kind (List, multi-block List, Flex list, InlineList), with unrecorded entries after the recorded ones.
- The annotated diff tree emits list entries in the working tree's printed order.
- A reorder-only difference is no change for a list that is not `ordered-by user` and is a change for an `ordered-by user` list, in `show | compare`, the annotated diff, `PruneUnchanged` and `ze config diff` (D-4).
- Container child order moved to `plan/release/fixes/spec-config-yang-definition-order.md`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A config file (hierarchical or set) parsed by `Parser.Parse`, `SetParser`, or the set-meta parser; editor and web and gNMI Set edits through `Editor` sessions; `TreeFromPluginMap` for IKE and PKI.

### Transformation Path
1. The parser or the editor adds each entry with `Tree.AddListEntry`, which appends the key to `listOrder`.
2. Rename, copy, delete, merge and clone keep `listOrder` consistent (Tree methods).
3. A printer (`Serialize`, `SerializeSubtree`, `SerializeSet`, `SerializeSetWithMeta`, `SerializeAnnotated*`, `SerializeBlame`, `SerializeChangeFile`) asks one key-order function for each list's keys and prints in that order.
4. The diff surfaces (`PruneUnchanged`, `computeTreeAnnotatedDiff`) compare and print through the same printer and the same key order.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config file ↔ Tree | Parsers append to `listOrder` via `AddListEntry` | Yes (grep: every builder uses `AddListEntry`) |
| Tree ↔ plugin | `ToPluginMap` order sidecar; unchanged | Yes (`entryOrderLocked`) |
| Tree ↔ gNMI | `ToMap` JSON object; unchanged | Yes (`treeToTypedValue`) |

### Integration Points
- `listBlockKeys` (`serialize.go`) - becomes the single key-order function for every printer and every list kind.
- `diffList` (`cli/diff_tree.go`) - takes its key order from the same function over both trees.
- `PruneUnchanged` (`prune.go`) and `ze config diff` (`cmd_diff.go`) - read the list's `UserOrdered` flag to decide whether entry order is part of the comparison (D-4).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Printers read the Tree only; no parser change |
| No unintended coupling (components stay isolated) | Yes | Changes stay in `internal/component/config` and `internal/component/cli/diff_tree.go` |
| No duplicated functionality (extends existing, does not recreate) | Yes | `listBlockKeys` already exists; the eleven inline `slices.Sort(keys)` copies are deleted in favor of it |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Printing only; one key slice per list as today |
| Registration over hardcoding, outbound | Yes | No new command, view or handler |
| Registration over hardcoding, inbound | Yes | The order is derived from the Tree's record and the schema's `UserOrdered`; no list name appears in any printer (searched: `serialize*.go`, `flatten.go`, `diff_tree.go`) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every production Tree builder records each list entry in `listOrder` | grep of `AddListEntry` callers and direct `lists[...] =` writes: only `Clone`, `mergeFrom`, `TreeFromPluginMap` write the map, each with its order | An entry prints after the recorded ones instead of where written | `TestEveryListPrintsRecordedOrder` over parser, set parser, set-meta parser, editor `set`, `copy`, `rename`; plus a grep in the Critical Review | confirmed (grep, 2026-10-28: every Tree list write in `internal/component/config` is `AddListEntry`, `Clone`, `mergeFrom` or `TreeFromPluginMap`; `TestEveryListPrintsRecordedOrder` re-proves it once Phase 2 prints) |
| A-2 | No consumer relies on printed text order for correctness; order-sensitive consumers read `configorder.Entries` or `GetListOrdered` | `entryOrderLocked`, `configorder`, `cmd_diff.go` uses `DiffMaps` over maps, `Reader` diff is map-keyed | A reload or a validation reads a different first entry | Grep for text parsing of `Serialize` output outside tests; run `internal/component/config/...` and `internal/component/cli/...` tests | confirmed (2026-10-28: every production printer caller either shows the text or re-parses it, `editor_commit.go` `preCommitValidate`; none reads an entry position from text) |
| A-3 | `listOrder` is deterministic for a given input: no builder fills a list while ranging a Go map | Migrations use `GetListOrdered` (`config/migration/*.go`); `exabgp/migration/migrate.go` ranges `GetList("neighbor")` but its printer is separate | Archive (`ChangeTracker.HasChanged`) and history digests churn on every save | `TestSerializeIsDeterministic` (100 prints of fresh parses compare equal) and a review of every `range ... GetList(` loop that calls `AddListEntry` | broken (2026-10-28): `Editor.loadTree` (`internal/component/cli/editor_load.go`) ranges `want.ListKeys(name)`, which is map order (`Tree.ListKeys`), and calls `CreateEntry` for each missing entry, so `load merge`/`load replace` records random order. Design holds (R-4 mitigation): Phase 2 makes it range `GetListOrdered`. `exabgp/migration/migrate.go` `GetList("neighbor")` loop only reads; the migration builders range `GetListOrdered` or sorted keys |
| A-4 | The set-meta committed file already carries recorded order, so committed files do not change bytes for Phase 2 | `serializeSetMetaList` prints `listOrder` | Every committed file rewrites on first save | Unit test: parse a committed set-meta file, save, compare bytes | confirmed for recorded entries (`serializeSetMetaList` prints `listOrder`); the byte test is still owed in Phase 2 |
| A-5 | Every comparison surface can read a list's `UserOrdered` flag where it compares entries: `PruneUnchanged` and `diffList` hold the schema node, and `ze config diff` holds the schema it parsed with | `prune.go` and `diff_tree.go` walk schema and tree together; `cmd_diff.go` parses both files with the schema | An `ordered-by user` reorder stays invisible on one surface (D-4) | AC-12 on each of the three surfaces | broken for `ze config diff` (2026-10-28): `parseAndResolve` (`cmd_diff.go`) diffs only `infra.ResolveBGPTree` output and refuses a file with no `bgp { }`, so a firewall term reorder never reaches the diff; Phase 3 must diff the parsed tree's `ordered-by user` lists itself. `diffList` holds the `*config.ListNode` and `PruneUnchanged` the schema `Node`: confirmed |
| A-6 | Recorded order survives archive, rollback and commit-confirm reload | Those load file text through the parsers | A rollback reorders entries | `.ci` rollback test asserting `show` order after rollback | unvalidated |
| A-7 | The set parser reads a field of an InlineList entry, the line `serializeSetInlineList` writes | The set printer writes `set route <key> next-hop <value>` | A set-format file or an editor session holding an InlineList entry field never loads | `TestSetParserReadsInlineListEntryField` | broken, fixed (2026-10-28): `SetParser.walkAndSet` looked a child up only under a `*ContainerNode` or `*ListNode` parent and refused any other ("cannot traverse *config.InlineListNode", and `*config.FlexNode` for `set vpls rd <v>`); it now resolves through `resolveSchemaNode`, as `walkAndSetWithMeta` and `walkAndDelete` already did |
| A-8 | The set parser reads a Flex named block entry, `set vpls <key> rd <value>`, the line `serializeSetFlex` writes for each list entry | The hierarchical parser stores `vpls <key> { rd ...; }` as a list entry (`parseFlexValue`) | All but the last entry are lost on a set-format load | `TestSetParserReadsFlexListEntries`, `TestSetFormRoundTripsFlexAndInlineList` | broken, fixed (2026-10-28): `SetParser.setFlexValue` read every token run whose first token is no Flex child as the Flex value, so each line replaced the value `"<key> rd <value>"`; `isFlexListEntry` (`setparser_entries.go`) now reads `<key> <field> ...` as entry `<key>`, recorded in written order by `ensureFlexListEntry`. `walkAndDelete` reaches the entry the same way (`TestSetParserDeletesFlexListEntry`), and `delete vpls` now removes the entries too |
| A-9 | The set-meta parser reads the same Flex named block entry | The committed file and the editor session are set-meta files | The committed file loses Flex entries, each read as a value | `TestSetParserReadsFlexListEntries` (ParseWithMeta) | broken, fixed (2026-10-28): the FlexNode branch of `walkAndSetWithMeta` had the same value fallback; it now calls `isFlexListEntry` and records the entry's metadata under `GetOrCreateListEntry(<key>)`, where `writeFlexLines` reads it |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A diff view prints an empty ancestor block (`bgp { }`) when a non-`ordered-by user` list differs only in order, because the list-level text differs while every entry prunes away | `show | compare` in an `.et` test shows an empty block after delete-and-re-add of an identical entry | AC-7: `PruneUnchanged` removes a list whose every entry pruned, and an ancestor left empty by that removal |
| R-2 | The annotated diff gutter lands on the wrong lines once `Serialize` prints recorded order while `diffList` still sorts | `annotateContentWithTreeDiff` unit test with out-of-key-order entries | Phase 3 orders `diffList` from the working tree; removed entries placed after their predecessor in the original |
| R-3 | Many `.et`, `.ci` and Go expectations change from sorted to written order | Scoped suite reds after Phase 2 | Each refreshed expectation is judged by `ze-test-judge` (owner meta decision on test changes) and the commit carries its approval lines; an expectation is refreshed only where the input order differs from the old sorted order |
| R-4 | A hidden builder fills a list from map iteration, so the printed order is random | A flaky `.ci` or a determinism test failure | A-3 test; fix the builder to use `GetListOrdered` |
| R-5 | The printers' fallback for an unrecorded entry differs between printers again | Two printers disagree in a unit test | One function, `listBlockKeys`, for every printer; the inline `slices.Sort(keys)` copies are deleted |
| R-6 | A reorder-only commit of a plain list shows nothing in `show | compare` (D-4) and the commit path reads "no changes" and skips writing, so the operator's new order is lost | `.et` test: reorder a plain list, commit, reload, `show` | AC-7 asserts the committed file records the new order even though the compare is empty; the commit's no-change test must read the file bytes, not the pruned diff |
| R-7 | An operator cannot reorder a list except by delete and re-add, now that order is visible | User report | Owner decision D-2: `plan/post-release/features/spec-config-list-entry-insert.md` adds `insert <list> <entry> before|after <entry>` |
| R-8 | Showing an `ordered-by user` reorder as a change places the gutter marks wrongly, because the moved entry is the same key on both sides | `TestTreeDiffUserOrderedMoveIsChange` | The moved entry is emitted as removed at its old position and added at its new one, the way Junos `show | compare` shows an `insert` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Display order only: no reader consumes printed order. A printer that dropped an entry would lose configuration in the committed file; Phase 2 removes the existing drop path. A wrong D-4 comparison shows a plain-list reorder as a change, or hides an `ordered-by user` reorder (firewall term order is behavior) |
| How is it reverted? | Single commit revert per phase; no config migration, because both orders parse to the same tree |
| Who else touches this path? | The parent `spec-ze-config-fmt` (comment association in `parser_comments.go`, the corpus gate in `fmt_corpus_test.go`); its owner is designing Q-1 and Q-3 to Q-9 now |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config fmt` on a file with `peer zulu` before `peer alpha` | → | `configFmtBytes` → `Serialize` → `listBlockKeys` | `TestConfigFmtKeepsOperatorOrderedEntries` (`internal/component/config/cli/cmd_fmt_test.go`) and `test/parse/config-fmt-operator-order.ci` |
| Editor `show` after `set` of entries out of key order | → | `Editor.WorkingContent` → `Serialize` | `test/editor/commands/show-operator-order.et` |
| Editor `show | compare` after adding an entry out of key order | → | `computeTreeAnnotatedDiff` → `diffList` | `TestTreeDiffListFollowsWorkingOrder` (`internal/component/cli/diff_tree_test.go`) |
| Web CLI terminal `show` | → | `cli_terminal.go` → `config.Serialize` | `TestWebCLIShowKeepsOperatorOrder` (`internal/component/web/cli_terminal_test.go`) |
| `ze config diff` of two files differing only in firewall term order | → | `cmd_diff.go` → `DiffMaps` plus the `ordered-by user` order comparison | `TestConfigDiffUserOrderedReorder` (`internal/component/config/cli/cmd_diff_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Hierarchical file with `peer zulu`, then `peer alpha`, then `peer mike`, formatted by `ze config fmt` | Output lists zulu, alpha, mike in that order |
| AC-2 | Same tree printed by `Serialize`, `SerializeSet`, `SerializeSetWithMeta`, the annotated, the annotated-set and the blame views | Every printer lists the entries in the same order, zulu, alpha, mike |
| AC-3 | A Flex list, an InlineList and a multi-block positional list each written out of key order | Each prints in written order in the hierarchical and set forms |
| AC-4 | `ordered-by user` firewall chain terms out of key order | Still printed in written order (unchanged) |
| AC-5 | A list whose recorded order misses one entry (entry added through the live `GetList` map in a test) | Every printer prints the recorded entries in order, then the unrecorded one; no printer drops it |
| AC-6 | `fmt` applied to its own output, for every carrier in `TestFormatCorpusRoundTrip` | No byte changes, and the set form of every list (not only `ordered-by user`) has the same entry order before and after |
| AC-7 | (D-4, plain list) Working tree equal to the baseline except one list that is not `ordered-by user` recorded in another order | `PruneUnchanged` in both directions leaves nothing; `show | compare` prints no line and no empty block; `ze config diff` of the two files reports no difference; after commit and reload, `show` prints the new order (the committed file records it) |
| AC-8 | Editor adds `peer alpha` after existing `peer zulu`; `show | compare` | The `+` gutter mark lands on the `peer alpha` lines, which print after `peer zulu` |
| AC-9 | Editor `copy peer zulu to peer yankee`, `rename peer alpha to peer bravo` | `show` prints yankee right after zulu and bravo where alpha was |
| AC-10 | Commit, then `rollback` to that archive entry, then `show` | Entry order equals the order before the rollback |
| AC-11 | The same file parsed and printed 100 times | Identical bytes every time |
| AC-12 | (D-4, `ordered-by user` list) Working tree equal to the baseline except firewall chain terms `b`, `a` recorded as `a`, `b` in the baseline | `PruneUnchanged` keeps the list; `show | compare` shows term `b` removed at its old position and added before `a`; `ze config diff` of the two files reports the order change |
| AC-13 | `ze config show` and the web CLI terminal `show` of the AC-1 file | zulu, alpha, mike |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes peers in a chosen order and runs `ze config fmt` | file → `Parser` → `listOrder` → `Serialize` | `test/parse/config-fmt-operator-order.ci` |
| 2 | Adds a peer in the editor and runs `show` and `show | compare` | editor `set` → `ensureListEntry` → `AddListEntry` → `Serialize` / `diffList` | `test/editor/commands/show-operator-order.et` |
| 3 | Commits, rolls back, and shows | commit → set-meta file → archive → rollback → parse → `Serialize` | `test/editor/workflow/rollback-keeps-operator-order.et` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEveryListPrintsRecordedOrder` | `internal/component/config/serialize_list_order_test.go` | AC-2, AC-3 for each printer and each list kind, built by parser, set parser and set-meta parser | |
| `TestUnrecordedEntryPrintsLast` | `internal/component/config/serialize_list_order_test.go` | AC-5 for every printer | |
| `TestSerializeIsDeterministic` | `internal/component/config/serialize_list_order_test.go` | AC-11 | |
| `TestPruneUnchangedIgnoresEntryOrderOnly` | `internal/component/config/prune_test.go` | AC-7 | |
| `TestTreeDiffListFollowsWorkingOrder` | `internal/component/cli/diff_tree_test.go` | AC-8 | |
| `TestConfigFmtKeepsOperatorOrderedEntries` | `internal/component/config/cli/cmd_fmt_test.go` | AC-1, idempotence | |
| `TestFormatCorpusRoundTrip` (extended) | `internal/component/config/cli/fmt_corpus_test.go` | AC-6: entry order compared for every list | |
| `TestPruneUnchangedKeepsUserOrderedReorder` | `internal/component/config/prune_test.go` | AC-12 prune | |
| `TestTreeDiffUserOrderedMoveIsChange` | `internal/component/cli/diff_tree_test.go` | AC-12 compare | |
| `TestConfigDiffUserOrderedReorder` | `internal/component/config/cli/cmd_diff_test.go` | AC-7 and AC-12 for `ze config diff` | |
| `TestWebCLIShowKeepsOperatorOrder` | `internal/component/web/cli_terminal_test.go` | AC-13 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | No numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `config-fmt-operator-order` | `test/parse/config-fmt-operator-order.ci` | AC-1, AC-6 through the `ze config fmt` binary | |
| `show-operator-order` | `test/editor/commands/show-operator-order.et` | AC-8, AC-9, AC-13 | |
| `rollback-keeps-operator-order` | `test/editor/workflow/rollback-keeps-operator-order.et` | AC-10 | |

### Interop Tests (Scope: protocol)
N-A: no wire-visible change.

## Files to Modify
- `internal/component/config/serialize.go` - `listBlockKeys` drops the `UserOrdered` condition (recorded order, then unrecorded sorted); `serializeListMultiBlock`, the FlexNode and InlineListNode branches of `serializeNode` call it; the doc comment is rewritten
- `internal/component/config/serialize_set.go` - `serializeSetList`, `serializeSetMetaList`, `serializeSetFlex`, `serializeSetInlineList`, `writeFlexLines`, `writeInlineListLines` call `listBlockKeys`
- `internal/component/config/serialize_annotated.go` - the seven list, flex and inline-list functions call `listBlockKeys`
- `internal/component/config/serialize_blame.go` - `serializeBlameList`, `serializeBlameFlex`, `serializeBlameInlineList` call `listBlockKeys`
- `internal/component/config/prune.go` - AC-7
- `internal/component/cli/diff_tree.go` - `diffList` order; an `ordered-by user` move shown as removed and added (AC-12)
- `internal/component/config/cli/cmd_diff.go` - entry-order comparison for `ordered-by user` lists (AC-12)
- `docs/architecture/config/syntax.md` - the list-order paragraph, and the D-4 comparison rule
- Expectations: the `.et`, `.ci` and Go tests whose input order differs from sorted order, each judged (R-3)

## Files to Create
- `internal/component/config/serialize_list_order_test.go`
- `internal/component/cli/diff_tree_test.go` (no file of that name exists; the siblings are `diff_tree_gutter_test.go` and `diff_tree_multikey_test.go`)
- `internal/component/web/cli_terminal_test.go` (no file of that name exists; the sibling is `cli_terminal_secret_test.go`)
- `test/parse/config-fmt-operator-order.ci`
- `test/editor/commands/show-operator-order.et`
- `test/editor/workflow/rollback-keeps-operator-order.et`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No schema change; child order is `spec-config-yang-definition-order.md` |
| YANG validation constraints | No | No leaf added |
| YANG custom validators | No | None |
| CLI commands/flags | No | No new command; the D-2 `insert` is `spec-config-list-entry-insert.md` |
| CLI grammar (keyword before value) | No | No grammar change |
| Editor autocomplete | No | Completion order is outside this spec |
| Functional test for new RPC/API | No | No RPC; functional tests above cover the printers |
| Pipe completeness | No | `show` pipes unchanged |
| Env var registration | No | None |
| Doctor check for runtime dependencies | No | No runtime dependency |
| Prometheus counters/metrics | No | None |
| BGP family surface (new SAFI / capability / attribute) | No | Not BGP family work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Display order of an existing feature |
| 2 | Config syntax changed? | Yes | `docs/architecture/config/syntax.md` (printer order paragraph); syntax accepted is unchanged |
| 3 | CLI command added/changed? | No | Output order only |
| 4 | API/RPC added/changed? | No | gNMI and `ToMap` unchanged |
| 5 | Plugin added/changed? | No | None |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md`: check for a statement that output is sorted; edit if present |
| 7 | Wire format changed? | No | None |
| 8 | Plugin SDK/protocol changed? | No | `ToPluginMap` unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | No | None |
| 10 | Test infrastructure changed? | No | None |
| 11 | Affects daemon comparison? | No | None |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/syntax.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Run `./le spec citation anchors spec plan/release/fixes/spec-config-list-entries-operator-order.md`; `syntax.md` anchors `listBlockKeys`; the run also names `docs/features/cli-commands.md` (`ze config diff`, gains the D-4 order rule) and `docs/guide/config-editor.md` (`show` and `show | compare` order, gains the D-4 rule) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Examples under `docs/` that show `show` output with several list entries; verify against the new printer |

## Implementation Steps

Each phase is sized at about 60 tool calls.

0. **Phase: Measure** -- before any code, run the scoped config and cli package tests and the `test/editor` and `test/parse` suites on HEAD and keep the result, so every later red is attributable.
1. **Phase: Wiring (MANDATORY FIRST)** -- write the four Wiring Test rows and `TestEveryListPrintsRecordedOrder`, `TestUnrecordedEntryPrintsLast`; they fail on HEAD.
   - Files: the new test files above
   - Verify: each fails for the stated reason (sorted output, dropped entry)
2. **Phase: One key order for every printer** -- `listBlockKeys` becomes recorded-then-unrecorded for every list; every printer and list kind calls it; the inline sort copies are deleted; `syntax.md` paragraph rewritten.
   - Tests: AC-1 to AC-6, AC-11, AC-13
   - Verify: tests pass; scoped suites rerun; each changed expectation goes to `ze-test-judge`
3. **Phase: Diff surfaces (D-4)** -- `diffList` follows the working order, placing a removed entry after its original predecessor; `PruneUnchanged` drops a list left empty and the ancestors it empties; for an `ordered-by user` list a reorder is kept by prune, shown as a move by `diffList`, and reported by `ze config diff`; for every other list entry order takes no part in any comparison.
   - Tests: AC-7, AC-8, AC-9, AC-10, AC-12
4. **Phase: Close-out** -- corpus round trip green, doc anchors checked, Critical Review.

The former container-order phase is `plan/release/fixes/spec-config-yang-definition-order.md` (owner decision D-1).

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | All three user stories pass end to end |
| Correctness | No printer contains its own `slices.Sort` over list entry keys; grep `serialize*.go`, `flatten.go`, `diff_tree.go` |
| Correctness | No printer drops an entry absent from `listOrder` (AC-5 for each) |
| Correctness | No production code ranges `GetList` while calling `AddListEntry` (A-3) |
| Data flow | Readers (`configorder.Entries`, `GetListOrdered`, `ToMap`) are untouched |
| Correctness (D-4) | Every comparison surface (`PruneUnchanged`, `diffList`, `ze config diff`) decides order relevance from `UserOrdered` alone, never from a list name; AC-7 and AC-12 each run on all three |
| Rule: no-layering | The old sorted path is deleted, not kept as a fallback or an option |
| Rule: test changes | Every refreshed expectation carries a `ze-test-judge` approval line |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| One key-order function | `grep -n "slices.Sort(keys)" internal/component/config/serialize*.go` shows no list-entry sort |
| Operator order in fmt | `test/parse/config-fmt-operator-order.ci` passes |
| Diff gutter aligned | `TestTreeDiffListFollowsWorkingOrder` passes |
| Docs | `syntax.md` paragraph no longer says "sorted by key" |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | None added; printing only |
| Secret exposure | Masked printers keep masking: `TestEveryListPrintsRecordedOrder` runs one case through `MaskSecrets` |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

| ID | Design point | Finding | Recommendation | Decided by |
|----|--------------|---------|----------------|------------|
| D-1 | Leaf and container order | Today containers, Flex and InlineList children are alphabetical (`flattenChildren` and `sortedKeys` in `yang_schema.go`); only list children follow YANG definition order | Its own spec: `plan/release/fixes/spec-config-yang-definition-order.md` | Thomas, 2026-10-28 |
| D-2 | Reordering entries | The editor's `insert` handles leaf-lists only (`InsertLeafListValue`); no list-entry move exists | Its own spec: `plan/post-release/features/spec-config-list-entry-insert.md` | Thomas, 2026-10-28 |
| D-3 | Freeform and unknown-field values | `serializeFreeform` and `serializeExtraValues` sort values; they have no schema order | Stay sorted | Thomas, 2026-10-28 |
| D-4 | Order-only differences | A plain list's order carries no meaning; an `ordered-by user` list's order is behavior | Plain list: no change (AC-7). `ordered-by user`: a change (AC-12) | Thomas, 2026-10-28 |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One function (`listBlockKeys`) gives every printer its key order | Fix each printer's inline sort separately | Eleven copies exist today and they already disagree (the set printers drop an unrecorded entry, the hierarchical printer keeps it); one declaration stops the drift |
| Unrecorded entries print after the recorded ones, sorted | Refuse to print; drop them | Dropping loses configuration; refusing breaks a display. Sorted placement after the record is what `listBlockKeys` already does for `ordered-by user` |
| Owner decision D-1 (Thomas, 2026-10-28): leaves and containers print in YANG definition order: a directly declared child at its position in the source, `uses` children at the `uses` statement's position in the grouping's order, augmented children after the block's own, ordered by module name, then augment order within a module | Alphabetical, as today | Junos model; owned by `plan/release/fixes/spec-config-yang-definition-order.md`, which depends on this spec |
| Owner decision D-2 (Thomas, 2026-10-28): a Junos-style `insert <list> <entry> before|after <entry>` for every list, `ordered-by user` included, in the editor and on every surface carrying editor verbs | Delete and re-add only | Owned by `plan/post-release/features/spec-config-list-entry-insert.md`, which depends on this spec |
| Owner decision D-3 (Thomas, 2026-10-28): free-form and unknown-field values stay sorted | Recorded order | They have no schema order and no list identity |
| Owner decision D-4 (Thomas, 2026-10-28): for a list that is not `ordered-by user`, a reorder-only difference is no change in `show | compare`, diffs and prune; for an `ordered-by user` list a reorder is a change | Treat any order change as a diff; treat none as a diff | A plain list's order carries no meaning, so showing it is noise; an `ordered-by user` list's order is behavior (firewall term evaluation), so hiding it hides a behavior change. AC-7 and AC-12 |

## Known Limitations
- gNMI Get encodes lists as JSON objects through `encoding/json`, which writes keys sorted (`treeToTypedValue`). This spec does not change gNMI encoding.
- `ze config diff` (`cmd_diff.go`) stays a path-keyed map diff sorted by path; only the `ordered-by user` order comparison is added (D-4).
- The ExaBGP migration printer (`internal/exabgp/migration/migrate_serialize.go`, `SerializeTree`) already prints lists in recorded order and values sorted by name; it is not changed.
- Blank lines and comment grouping belong to the parent spec `plan/post-release/engineering/spec-ze-config-fmt.md`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits. The full verification is Thomas's release check
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-config-list-entries-operator-order.md` only, in the same `./le commit create` script

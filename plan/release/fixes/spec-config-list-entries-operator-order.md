# Spec: config-list-entries-operator-order

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Priority | P2 |
| Release reason | The operator's own config prints in a different order from the file the editor saves: the set form keeps the recorded entry order while the hierarchical form (`show`, `ze config fmt`, `ze config migrate format hierarchical`) sorts it, and the annotated diff gutter sorts even `ordered-by user` lists. A printer that reorders what the operator wrote is shipped configuration behavior. |
| RFC gaps | - |
| Depends | - |
| Phase | 5/5 |
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
4. The diff surfaces compare schema-driven subtree content; the visual diff renders entries and values through the shared writers, key order and token codec.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config file ↔ Tree | Parsers append to `listOrder` via `AddListEntry` | Yes (grep: every builder uses `AddListEntry`) |
| Tree ↔ plugin | `ToPluginMap` order sidecar; unchanged | Yes (`entryOrderLocked`) |
| Tree ↔ gNMI | `ToMap` JSON object; unchanged | Yes (`treeToTypedValue`) |

### Integration Points
- `ListEntryKeys` (`serialize.go`) - the single recorded-then-unrecorded key order for every printer and list kind.
- `diffListEntries` (`cli/diff_tree.go`) - follows working order and anchors removals to the original order.
- `PruneUnchanged` (`prune.go`) and `ze config diff` (`cmd_diff.go`) - read the list's `UserOrdered` flag to decide whether entry order is part of the comparison (D-4).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Printers read recorded Tree order; plain and metadata set parsers preserve repeated Flex values; editor load and session metadata replay preserve entry order |
| No unintended coupling (components stay isolated) | Yes | Production changes stay in config and CLI components; web show inherits the shared serializer |
| No duplicated functionality (extends existing, does not recreate) | Yes | `ListEntryKeys` replaces the private ordering helper and repeated list-key sorts; typed entry writers serve hierarchical serialization and visual diff. `QuoteIfNeeded` is the existing canonical token codec, exported to replace the divergent visual-diff copy |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Shared entry writers write directly to caller buffers; key-order repair allocates keys, not copied entry trees |
| Registration over hardcoding, outbound | Yes | No new command, view or handler |
| Registration over hardcoding, inbound | Yes | The order is derived from the Tree's record and the schema's `UserOrdered`; no list name appears in any printer (searched: `serialize*.go`, `flatten.go`, `diff_tree.go`) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every production Tree builder records each list entry in `listOrder` | grep of `AddListEntry` callers and direct `lists[...] =` writes: only `Clone`, `mergeFrom`, `TreeFromPluginMap` write the map, each with its order | An entry prints after the recorded ones instead of where written | `TestEveryListPrintsRecordedOrder` over parser, set parser, set-meta parser, editor `set`, `copy`, `rename`; plus a grep in the Critical Review | confirmed (grep, 2026-10-28: every Tree list write in `internal/component/config` is `AddListEntry`, `Clone`, `mergeFrom` or `TreeFromPluginMap`; `TestEveryListPrintsRecordedOrder` re-proves it once Phase 2 prints) |
| A-2 | No consumer relies on printed text order for correctness; order-sensitive consumers read `configorder.Entries` or `GetListOrdered` | `entryOrderLocked`, `configorder`, `cmd_diff.go` uses `DiffMaps` over maps, `Reader` diff is map-keyed | A reload or a validation reads a different first entry | Grep for text parsing of `Serialize` output outside tests; run `internal/component/config/...` and `internal/component/cli/...` tests | confirmed (2026-10-28: every production printer caller either shows the text or re-parses it, `editor_commit.go` `preCommitValidate`; none reads an entry position from text) |
| A-3 | `listOrder` is deterministic for a given input: no builder fills a list while ranging a Go map | Migrations use `GetListOrdered` (`config/migration/*.go`); `exabgp/migration/migrate.go` ranges `GetList("neighbor")` but its printer is separate | Archive (`ChangeTracker.HasChanged`) and history digests churn on every save | `TestSerializeIsDeterministic` (100 fresh parses), editor load and session replay tests, builder review | broken, fixed: `Editor.loadTree` now reads `GetListOrdered`; tree-building session metadata replay uses `SessionEntriesOrdered`. Determinism, load merge/replace, typed entry commit/reopen, save/discard and nested copy tests passed |
| A-4 | The set-meta committed file already carries recorded order, so committed files do not change bytes for Phase 2 | `serializeSetMetaList` prints `listOrder` | Every committed file rewrites on first save | Unit test: parse a committed set-meta file, save, compare bytes | confirmed for complete recorded entries by passing `TestRecordedOrderMetadataByteRoundTrip`; formerly omitted unrecorded entries now persist instead of being dropped |
| A-5 | Every comparison surface can read a list's `UserOrdered` flag where it compares entries: `PruneUnchanged` and `diffList` hold the schema node, and `ze config diff` holds the schema it parsed with | `prune.go` and `diff_tree.go` walk schema and tree together; `cmd_diff.go` parses both files with the schema | An `ordered-by user` reorder stays invisible on one surface (D-4) | AC-12 on each of the three surfaces | broken, fixed for CLI diff: `parseAndResolve` retains parsed trees beside resolved BGP values, and `diffListOrders` walks schema-declared ordered lists, including firewall-only files. Prune, visual move, CLI text/JSON/revision tests and 80 native CLI scenario invocations passed |
| A-6 | Recorded order survives archive, rollback and commit-confirm reload | Those load file text through the parsers | A rollback reorders entries | Native rollback scenario and confirmed-window snapshot/revert/refresh test | confirmed: `rollback-keeps-operator-order.et` and `TestEditorListOrderLifecycle` passed; `TestSessionConfirmedRevertKeepsOperatorOrder` passed through real confirmed commit, synchronous abort, exact snapshot restoration, refreshed show and a fresh session. This does not claim timed-expiry or full-daemon verification |
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
| R-5 | The printers' fallback for an unrecorded entry differs between printers again | Two printers disagree in a unit test | One function, `ListEntryKeys`, for every printer; the private list-key sort copies are deleted |
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
| `ze config fmt` on a file with `peer zulu` before `peer alpha` | → | `configFmtBytes` → `Serialize` → `ListEntryKeys` | `TestConfigFmtKeepsOperatorOrderedEntries` (`internal/component/config/cli/cmd_fmt_test.go`) and `test/parse/config-fmt-operator-order.ci` |
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
| AC-8 | Editor adds `peer alpha` after existing `peer zulu`; full `show`, then `show | compare`; change zulu as well and compare again | Full show places the `+` gutter on alpha after zulu. Compare omits unchanged zulu; when both entries differ, it preserves their working order and alpha's addition marks |
| AC-9 | Editor `copy peer zulu to peer yankee`, `rename peer alpha to peer bravo` | `show` prints yankee right after zulu and bravo where alpha was |
| AC-10 | Commit, then `rollback` to that archive entry, then `show` | Entry order equals the order before the rollback |
| AC-11 | The same file parsed and printed 100 times | Identical bytes every time |
| AC-12 | (D-4, `ordered-by user` list) Working tree equal to the baseline except firewall chain terms `b`, `a` recorded as `a`, `b` in the baseline | `PruneUnchanged` keeps the list; `show | compare` shows term `b` removed at its old position and added before `a`; `ze config diff` of the two files reports the order change |
| AC-13 | `ze config show` and the web CLI terminal `show` of the AC-1 file | zulu, alpha, mike |

AC-8 follows the documented changed-only compare contract. Independent
`ze-test-judge` review rejected the original fixture's expectation that unchanged
zulu appeared in compare; the corrected fixture retains full-show order and
gutter checks, asserts unchanged zulu's omission, and tests two-entry compare
order after changing zulu. Reordered `ordered-by user` lists retain context as
required by AC-12.

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
| `TestEveryListPrintsRecordedOrder` | `internal/component/config/serialize_list_order_test.go` | AC-2, AC-3 for each printer and list kind, built by parser, set parser and set-meta parser | Passed |
| `TestUnrecordedEntryPrintsLast` | `internal/component/config/serialize_list_order_test.go` | AC-5 for every printer | Passed |
| `TestSerializeIsDeterministic` | `internal/component/config/serialize_list_order_test.go` | AC-11, 100 fresh parses for each builder and printer | Passed |
| `TestPruneUnchangedListOrder` (plain cases; planned as `TestPruneUnchangedIgnoresEntryOrderOnly`) | `internal/component/config/prune_order_test.go` | AC-7, both directions and nested plain list kinds | Passed |
| `TestTreeDiffListFollowsWorkingOrder` | `internal/component/cli/diff_tree_test.go` | AC-8 | Passed |
| `TestConfigFmtKeepsOperatorOrderedEntries` | `internal/component/config/cli/cmd_fmt_test.go` | AC-1, idempotence | Passed |
| `TestFormatCorpusRoundTrip` (extended) | `internal/component/config/cli/fmt_corpus_test.go` | AC-6: exact set bytes for every parsed carrier | Passed: 1,116 carriers, 21 multi-entry user-ordered lists, 3,520 comments |
| `TestPruneUnchangedListOrder/ordered` (planned as `TestPruneUnchangedKeepsUserOrderedReorder`) | `internal/component/config/prune_order_test.go` | AC-12 prune | Passed |
| `TestTreeDiffUserOrderedMoveIsChange` | `internal/component/cli/diff_tree_test.go` | AC-12 compare | Passed |
| `TestConfigDiffUserOrderedReorder` | `internal/component/config/cli/cmd_diff_test.go` | AC-7 and AC-12 for `ze config diff` | Passed |
| `TestWebCLIShowKeepsOperatorOrder` | `internal/component/web/cli_terminal_test.go` | AC-13 | Passed; wiring test already committed in Phase 1 |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | No numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `config-fmt-operator-order` | `test/parse/config-fmt-operator-order.ci` | AC-1, AC-6 and AC-13 through the actual formatter and show commands | Passed: native scenario and 80 invocations under bounded load |
| `show-operator-order` | `test/editor/commands/show-operator-order.et` | AC-8, AC-9, AC-13 | Passed: 58 native editor steps |
| `rollback-keeps-operator-order` | `test/editor/workflow/rollback-keeps-operator-order.et` | AC-10 | Passed |
| `compare-plain-list-reorder` | `test/editor/commands/compare-plain-list-reorder.et` | AC-7 through compare, commit and fresh-editor reload | Passed: 42 native editor steps |
| `compare-user-list-reorder` | `test/editor/commands/compare-user-list-reorder.et` | AC-12 through the real compare command | Passed: 28 native editor steps |
| `config-diff-list-order` | `test/parse/config-diff-list-order.ci` | AC-7 and AC-12, firewall-only configuration and masked output | Passed: 80 invocations under bounded load |

### Interop Tests (Scope: protocol)
N-A: no wire-visible change.

## Files to Modify
- `internal/component/config/serialize.go` - `ListEntryKeys` replaces `listBlockKeys`; every list printer uses it; typed entry writers are shared with visual diff
- `internal/component/config/serialize_set.go` - set and metadata writers use the shared entry order and preserve repeated Flex occurrences
- `internal/component/config/serialize_annotated.go` - annotated hierarchical/set list paths use the shared entry order
- `internal/component/config/serialize_blame.go` - blame list paths use the shared entry order
- `internal/component/config/prune.go` - AC-7
- `internal/component/cli/diff_tree.go` - `diffList` order; an `ordered-by user` move shown as removed and added (AC-12)
- `internal/component/config/cli/cmd_diff.go` - entry-order comparison for `ordered-by user` lists (AC-12)
- `internal/component/config/list_key.go`, `change_file.go`, `leaflist_text.go` and their quote tests - export the existing token codec as `QuoteIfNeeded`, migrate all callers, and remove the duplicate visual-diff codec
- `docs/architecture/config/syntax.md` - the list-order paragraph, and the D-4 comparison rule
- Expectations: the `.et`, `.ci` and Go tests whose input order differs from sorted order, each judged (R-3)

## Files to Create
- `internal/component/config/serialize_list_order_test.go`
- `internal/component/cli/diff_tree_test.go` - created in Phase 1 and extended here; existing gutter and multi-key tests remain
- `internal/component/web/cli_terminal_test.go` - created and committed in Phase 1; unchanged here and verified through the actual terminal navigation handler
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
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Existing printer behavior repaired; no feature declaration or support level changes |
| 2 | Config syntax changed? | Yes | `docs/architecture/config/syntax.md`, recorded order, canonical Flex occurrences and comparison; no new accepted grammar |
| 3 | CLI command added/changed? | Yes | `docs/features/cli-commands.md`, `ze config diff` user-ordered sequence paths; no new command or flag |
| 4 | API/RPC added/changed? | No | `ToMap`, gNMI encoding, API types and registrations are unchanged |
| 5 | Plugin added/changed? | No | No plugin implementation, registry or interface changes |
| 6 | Has a user guide page? | Yes | `docs/guide/config-editor.md`, show/compare, saved order and validation hints; `../wiki/editor-workflow.md` mirrors the operator-visible rule |
| 7 | Wire format changed? | No | Config/CLI-only diff; no protocol encoder or decoder changes |
| 8 | Plugin SDK/protocol changed? | No | `ToPluginMap` and `configorder.Entries` unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC-tagged test unit or protocol behavior belongs to this diff; no public RFC row update |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/ci-format.md`, regex on the displayed viewport strips ANSI only; content, whitespace, row order and diagnostics stay significant |
| 11 | Affects daemon comparison? | No | No protocol support or performance claim changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/syntax.md`, shared order/writers and semantic comparison; `docs/architecture/config/change-file-structural-ops.md`, chronological replay and final position restoration |
| 13 | Route metadata keys added/changed? | No | No route metadata; diff reuses existing config `@<list>` convention |
| 14 | Prometheus counters added/changed? | No | No metrics code changed |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration added, removed or changed |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Audited the five pages above; renamed codec and inline-container anchors corrected, including the Comments section. Reverse indices are generated companions |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Syntax and editor examples agree with the actual parser and native show/compare scenarios; non-BGP CLI value comparison is not claimed |

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
| One key-order function | `ListEntryKeys` is the only list-key ordering producer; remaining serializer sorts name unknown/freeform values or metadata users, not recorded list entries |
| Operator order in fmt | Native `config-fmt-operator-order` passed 80 invocations under bounded load; `TestConfigFmtKeepsOperatorOrderedEntries` passed |
| Diff gutter aligned | `TestTreeDiffListFollowsWorkingOrder`, move/mapping boundaries and all 58 `show-operator-order.et` steps passed |
| Docs | `syntax.md`, "What a reader receives", states recorded order and the plain/user-ordered comparison distinction |

### Security Review Checklist
| Check | Evidence and disposition |
|-------|--------------------------|
| Input validation | Flex replay uses parsed tokens and the shared `QuoteIfNeeded` codec; no shell, SQL, new filesystem path, listener or runtime dependency added. Empty/hash/quote/newline/tab/backslash/duplicate boundaries passed |
| Secret exposure | `MaskSecrets` remains the display boundary; CLI comparison detects changes before masking. Seven-printer secret tests, visual masking, CLI text/JSON and native secret rejection cases passed |
| Ownership and races | `SessionEntriesOrdered` retains ownership/deletion metadata; `RepositionListEntries` changes only selected order, not payloads. Parent/child read locks and caller-stable tree contracts reviewed; race-instrumented replay and other-owner tests passed |
| Bounds and resource use | Missing/stale/duplicate keys handled without indexing absent entries; move detection is O(n log n), pruning follows finite parsed trees. No new panic, unsafe operation, crypto primitive, authorization bypass or external resource lifetime |

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

## Implementation Summary

### What Was Implemented
- Every whole-tree printer and list shape uses `serialize.go::ListEntryKeys`: recorded keys once, then missing keys sorted.
- `prune.go::SubtreesEqual`, `cli/diff_tree.go::diffListEntries` and `config/cli/cmd_diff.go::diffListOrders` distinguish display order from user-ordered semantics.
- File load, saved drafts, copy/rename, commit and reload preserve order through `SessionEntriesOrdered`, chronological replay and `RepositionListEntries`.
- Hierarchical and visual writers share entry forms and `QuoteIfNeeded`. Flex occurrence replay retains repeated values without multiplying contested assignments.
- Bracket leaf-list diff rows retain their name, indentation and source mapping through all four change states.

### Bugs Found/Fixed
- Implementation fixed sorted/dropped entries, map-ordered replay, lost created-copy/rename positions, destination reuse across deletion, Flex value loss/amplification, false semantic differences and erased validation rows.
- Closure found a stale `canInlineContainer` source anchor and outdated comparison prose in the existing Comments section. The surviving producer is now named.
- Closure also fixed `diffBracketLeaf`: a nested reset/read of the same text buffer erased the statement prefix in every branch.

### Documentation Updates
- Updated `docs/architecture/config/syntax.md`, `docs/architecture/config/change-file-structural-ops.md`, `docs/guide/config-editor.md`, `docs/features/cli-commands.md` and `docs/architecture/testing/ci-format.md`.
- Repointed all four tracked citers: the formatter parent, definition-order and insert siblings, and the historical Phase 2 handover. No citation baseline changed.
- Wiki scope is `editor-workflow.md` and its generated `llms-full.txt` topic hunk. `llms.txt` is unchanged. No whole-wiki sync marker or push.

### Deviations from Plan
- A-3 and A-5 required order-preserving load/replay and retaining parsed trees beside resolved BGP maps. A-7..A-9 required set-parser fixes; their initial named-entry fixes already landed in Phase 1.
- The two proposed prune test names became the plain and ordered cases of `TestPruneUnchangedListOrder`, with both-direction and nested-list assertions.
- AC-8 uses the independently approved changed-only compare contract; full show still checks unchanged zulu before added alpha.
- Shared entry writers and the existing exported codec replaced divergent visual printers; no compatibility alias remains.
- Current selective-only rules replace the older whole-tree compile/doc/repo-check recipes. Existing proofs were read, not rerun; the closure's bracket fix received its own affected tests. Parent owns postcommit scoped verification.

### Risks & observations
- R-1/R-2/R-4/R-5/R-6/R-8 have direct prune, mapping, determinism, missing-key and persistence regressions. R-3 expectation changes carry independent approvals.
- R-7 remains the separately approved insert capability, not missing work here. Child order remains D-1's separate release fix.
- Known prior CLI reds `TestSessionLoadLargeInputBatched`, `TestRenameAndCopyOfMultiLeafKeyEntryChangeOnlyTheFirstValue` and `TestDiscardDeletesNewlyAddedValue` were outside these selected runs. No whole-package green is claimed.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| Broken A-3 | Assumed every load/replay builder preserved order | Load and metadata traversal iterated maps | Load/typed-commit and saved copy/rename reds | Ordered traversal and final owned-position restoration |
| Broken A-5 | Assumed CLI diff retained enough schema/tree information | Resolved BGP maps lost list order | Firewall-only/user-ordered diff reds | Keep parsed trees and compare schema-declared sequences |
| Broken A-7..A-9 | Assumed set readers accepted each printed InlineList/Flex form | Traversal and named Flex entries lost fields or entries | Phase 1 set-parser regressions | Shared schema resolution and named-entry parsing; repeated-value follow-through here |
| Documentation | Exported inline helper left an old Comments anchor | Producer is `CanInlineContainer`; semantic comparison ignores comments without stripping the working tree | Closure source-anchor scan and producer read | Correct anchor and prose in the existing syntax page |
| Review gap | The first closure pass missed bracket rows inside changed keyed entries | `diffBracketLeaf` reset and consumed its outer buffer through `fmtBracket` | Independent source finding, then four observed failing cases | Append each complete row directly; add exact marker/mapping and native editor regressions |
| Draft test input | Wrote a bracketed multi-token value to the interactive `set` command | `cmdSet` takes one final-token value; `Editor.storeLeaf` builds the bracket members | Native draft failed at its error assertion before reaching display | Independent test-judge approved two input corrections; every display assertion retained |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Every printer preserves recorded entry order | Done | `serialize.go::ListEntryKeys` and all serializer list branches | Seven printer variants, all list carriers |
| Plain reorder is no semantic change | Done | `prune.go::SubtreesEqual`; `diff_tree.go::diffListEntries`; `cmd_diff.go::diffListOrders` | Commit still persists the new display order |
| User-ordered reorder remains a change | Done | `pruneListReordered`, `diffListMoves`, `diffListOrders` | Firewall terms through native compare and CLI diff |
| Persistence and visible order agree | Done | `Editor.loadTree`, `SessionEntriesOrdered`, `positionCreatedEntries` | Copy/rename, direct/candidate, saved draft, archive and confirmed abort |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `configFmtBytes` → `Serialize`; native `config-fmt-operator-order` | zulu, alpha, mike; 80 load invocations |
| AC-2 | Done | `ListEntryKeys`; `TestEveryListPrintsRecordedOrder` | All seven printer variants |
| AC-3 | Done | Shared Flex, InlineList and positional branches; same matrix | Hierarchical, set and metadata readers |
| AC-4 | Done | `ListEntryKeys`; `TestConfigFmtKeepsUserOrderedEntries` | Existing user order retained |
| AC-5 | Done | `ListEntryKeys`; `TestUnrecordedEntryPrintsLast`, `TestListEntryKeysRepairsIncompleteRecord` | Missing, stale and duplicate records |
| AC-6 | Done | `TestFormatCorpusRoundTrip` | 1,116 checked carriers; exact set bytes, meaning, comments and idempotence |
| AC-7 | Done | `SubtreesEqual`, ordered session replay; plain compare fixture and CLI diff tests | Empty compare, no ancestor residue, commit/fresh reopen |
| AC-8 | Done | `diffListEntries`, `diffBracketLeaf`, `renderDiffLines`; show fixtures and working-order unit tests | 58 native order steps plus 19 bracket-row steps; all four bracket states retain exact rows and mapping |
| AC-9 | Done | Tree copy/rename and `positionCreatedEntries`; show fixture and replay matrix | Copy adjacency, rename position, destination reuse |
| AC-10 | Done | `Editor.rollback`; rollback fixture and `TestEditorListOrderLifecycle` | Archive order restored |
| AC-11 | Done | `TestSerializeIsDeterministic` | 100 fresh parses per builder/printer |
| AC-12 | Done | `pruneListReordered`, `diffListMoves`, `diffListOrders`; user compare and native diff fixtures | Removed old position, added working position, retained context |
| AC-13 | Done | `showConfig` and `executeTerminalNav` → shared serializer | Actual CLI show in formatter fixture; `TestWebCLIShowKeepsOperatorOrder` passed |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Printer order, missing keys and determinism | Passed | `serialize_list_order_test.go` | Exact planned names present |
| Plain/user prune | Passed | `prune_order_test.go::TestPruneUnchangedListOrder` | Two proposed names combined, not removed coverage |
| Working-order and move gutters | Passed | `diff_tree_test.go` | Planned names present; shape/mapping/codec boundaries added |
| Formatter and corpus | Passed | `config/cli/cmd_fmt_test.go`, `fmt_corpus_test.go` | Planned names present |
| CLI diff and web show | Passed | `cmd_diff_test.go`, `web/cli_terminal_test.go` | Planned names present |
| Native fmt/show/compare/rollback/commit/diff | Passed | Two parse scenarios and six editor scenarios in the owned manifest | Original expected rows retained; approved corrections recorded |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| All four serializers, prune and both diff implementations | Done | Owned manifest contains each planned source |
| Codec, replay, metadata and tree-position files | Done | Required by broken assumptions and observed persistence failures |
| Planned printer/diff/web tests | Done | Web test and initial wiring were already committed in Phase 1 |
| Planned formatter/show/rollback fixtures | Done | Present; added compare, typed-commit and CLI diff fixtures strengthen end-to-end proof |
| Syntax and related architecture/operator/testing docs | Done | Five canonical pages and scoped wiki topic; generated companions accounted for |
| Closure citations | Done | Four existing tracked citers repointed before spec removal |

### Audit Summary
- **Total items:** 13 acceptance criteria
- **Done:** 13
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0 acceptance requirements dropped; AC-8 interpretation independently approved

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| One display order everywhere | Actual commands plus printer matrix | Native formatter/show 80 successful invocations; web handler test; seven-printer fresh-parse matrix |
| Plain order is presentation only | End-to-end persistence and comparison | 42-step plain compare scenario; `TestConfigDiffUserOrderedReorder/plain_list_reordered_is_no_change`; both-direction prune tests |
| User order is behavior | Actual firewall compare and CLI diff | 28-step user compare scenario; 80 native CLI diff invocations including firewall-only input |
| Edits survive storage | Lifecycle proofs | Native commit/rollback scenarios; saved copy/rename/reused-destination matrix; confirmed commit synchronous abort restores exact bytes and refreshed order |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| No in-scope item remains | All 13 ACs and deliverables are implemented. D-1 child order, D-2 insert and parent formatter policy were explicitly separate before implementation | Not applicable |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/config-list-entries-operator-order-644a4e78-0122-4eeb-8642-eb50638a1641.md`; round 2 covers 56 selected files, including only owned syntax hunks |
| `./le spec review check` | Named source-build check: OK, 46 code files, CLEAN and matching hashes; final record includes this closure text |
| Rounds | 2 |
| Reviewer lenses used | Logic, wiring and removed behavior; security, edge cases and allocations; completeness, documentation and Go style |
| Final verdict | CLEAN: 0 BLOCKER, 0 ISSUE after the bracket source repair and independent delta review |
| Reviewer identity | `CloseConfigListOrder`, `openai-codex/gpt-6-astra`; native model metadata comes from the inherited Claude transcript, not this worker |

### Run 1 scope

The complete 7,564-line final implementation snapshot, all 52 manifest paths,
current producers and closure citation/documentation deltas. Current blob
comparison found only the edited closure spec and a new foreign syntax hunk;
foreign policy hunks are excluded through the commit's owned-hunks patch.
All lenses ran inline in this independent closure context, including the
style pass over every changed Go file. No new peer-reachable panic, discarded
error, unsafe operation or unnecessary payload copy was introduced.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Removed inline-helper anchor and old comment-comparison description | `syntax.md`, Comments | Name `CanInlineContainer`; describe semantic comment exclusion |
| 2 | NOTE | Planned prune names, CLI/test-infrastructure checklist and codec comment were stale | Spec audit and `serialize_set.go` | Record actual tests and docs; name `QuoteIfNeeded` |
| 3 | NOTE | Spec removal would strand four tracked citers | Parent, two siblings and historical handover | Cite durable syntax page; preserve historical stem only |
| 4 | ISSUE | Bracket leaf rows lose their prefix and cannot map to working statements | `diff_tree.go::diffBracketLeaf` | Direct full-row append; exact four-marker Go regression and real editor scenario |

### Fixes applied

Corrected the existing docs, citations and codec comment. The bracket source
repair removes the nested buffer reset/read and intermediate string while
preserving marker selection. The additive Go regression failed in all four
states before the fix and passed afterward with the affected tree-diff tests.
No existing behavioral assertion was weakened or removed.

### Run 2 scope

`diffBracketLeaf`, its changed-keyed-entry caller, the new exact row, marker
and source-mapping regression, and the native editor scenario. The first pass
missed the shared-buffer reset inside the outer append chain; its initial
CLEAN verdict was withdrawn. Main independently read the closure-authored
source/test delta and the before-fix log and found no issue. Logic, mapping,
allocations, security, Go style and unchanged marker branches were checked.
The native scenario then passed all 19 steps after the independently approved
input correction. It was promoted unchanged from the draft directory.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| All implementation and fixture paths | Yes | Owned-file manifest, source reads and snapshot blob comparison |
| Canonical docs and four citation targets | Yes | Current files read; durable syntax page contains display/comparison contract |
| Web wiring test | Yes | `cli_terminal_test.go::TestWebCLIShowKeepsOperatorOrder`, already in HEAD |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-6, AC-11 | Correct, stable printer bytes | Read primary final codec log `artifact://117`: config PASS 4.429s; final corpus transcript record `f32fadb8`: 1,116 carriers PASS |
| AC-7..AC-10, AC-12 | Comparison and persistence | Native editor results and primary replay records `0e0f8ee9`, `7ba3c0e1`, `8c778053`; twelve destination-reuse cases and confirmed abort passed |
| AC-13 | Real CLI and web show | Formatter/show stress log has 80 exit-0 invocations; primary transcript `e37f3a95` names web test PASS |
| Closure bracket repair | Exact statements, markers and mappings | Named native job: `go test -race -v -tags <feature manifest tags> ./internal/component/cli -run '^TestTreeDiff' -count=1` PASS, 2.006s; new regression observed red in all four states before the fix |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze config fmt`, `ze config show` | `test/parse/config-fmt-operator-order.ci` | 80 exit-0 native invocations under bounded load |
| `ze config diff` | `test/parse/config-diff-list-order.ci` | 80 exit-0 native invocations; plain/user order and masked secrets |
| Editor show/compare/copy/rename | `test/editor/commands/show-operator-order.et` | 58 steps passed |
| Plain/user compare | `compare-plain-list-reorder.et`, `compare-user-list-reorder.et` | 42 and 28 steps passed |
| Session commit and rollback | `commit-keeps-operator-order.et`, `rollback-keeps-operator-order.et` | Both native scenarios passed |
| Web CLI show | Go handler wiring test, not a `.ci` | `executeTerminalNav` reaches `config.Serialize`; named test passed |
| Editor bracket rows inside keyed lists | `test/editor/commands/show-bracket-leaf-in-list.et` | `./le --name order-644a4e78 test editor --draft --verbose test/draft/editor/show-bracket-leaf-in-list.et`: all 19 steps PASS before unchanged promotion |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Builder/Tree source review and three-reader printer matrix |
| A-2 | confirmed | Text consumers display or reparse; plugin and gNMI readers unchanged |
| A-3 | broken, fixed | Ordered load and metadata replay; determinism and lifecycle regressions |
| A-4 | confirmed with stated missing-record boundary | `TestRecordedOrderMetadataByteRoundTrip`; missing entries now retained |
| A-5 | broken, fixed | Parsed-tree retention and generic user-ordered sequence comparison |
| A-6 | confirmed | Archive rollback and confirmed commit/abort/refresh; no timed-expiry claim |
| A-7 | broken, fixed | Phase 1 schema traversal and InlineList set-reader regression |
| A-8 | broken, fixed | Named Flex entry parsing and occurrence-preserving follow-through |
| A-9 | broken, fixed | Metadata entry/occurrence replay and contested-assignment cycle tests |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Display and comparison semantics | `ListEntryKeys`, `SubtreesEqual`, `diffListEntries`, `diffListOrders` | Five changed canonical pages read against producers |
| Replay and lifecycle order | `SessionEntriesOrdered`, `planCreations`, `positionCreatedEntries` | Structural-ops and editor guides match chronological replay |
| Viewport regex | `checkViewport` and `checkContent` displayed-pattern branches | ANSI only; positive and reversed/missing/diagnostic-only negatives retained |
| No protocol/API/runtime dependency change | Owned source population and unchanged registries/readers | No RFC, doctor, SDK, wire or inventory update owed |
| Wiki topic and generated companions | `editor-workflow.md`; generator output for its topic | Committed as `c134d9aa20698ac8349246bade983973d07f3c60`; unchanged `llms.txt`, no marker or push |
| Mechanical gates | Named-build changed-test weakening audit; scoped source/citation inspection | No owned weakening finding; five foreign findings disclosed. No whole-tree gate claimed |

## Core Insight

Display order is stored operator information, not a comparison policy. Every
printer must preserve it, while only the schema's `UserOrdered` flag makes a
reorder a behavior change. Saving a semantically unchanged plain reorder must
therefore remain a real persistence operation.

# Spec: config-list-entry-insert

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | cli |
| Priority | P3 |
| Release reason | An absent capability, not incorrect behavior: today an operator reorders list entries by delete and re-add, which works and loses nothing. `plan/README.md` puts an additional capability in `post-release/features/`. The list-order fix it builds on (`spec-config-list-entries-operator-order.md`) is the release obligation, because that is where shipped printers disagree |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-config-list-entries-operator-order.md` (every printer prints recorded entry order, and the D-4 comparison rule exists, before a command can move an entry and have the move show) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Implement owner decision D-2 (Thomas, 2026-10-28): a Junos-style
`insert <list> <entry> before|after <entry>` that moves an existing list entry
next to another entry of the same list. It applies to every list, `ordered-by user`
ones included (firewall chain terms have the same gap today), in the SSH editor and
on every other surface whose command language carries the editor verbs.

| Surface | Carries editor verbs? | Owed here |
|---------|----------------------|-----------|
| SSH and `ze config edit` editor (`internal/component/cli`) | Yes: `set`, `delete`, `rename`, `copy`, `insert` (leaf-list) | The command, its completion, its help, session-mode recording and commit |
| Web CLI terminal (`internal/component/web/cli_terminal.go`) | Yes: `verbRename`, `verbCopy`, `verbInsert` in `cli.go` | The same command; today its `insert <path>` means "create a keyless list entry" (`mgr.CreateEntry`), so the verb's meaning is aligned with the editor (D-7) |
| Web editor forms (`fragment.go`, `handler_config_entry.go`) | No: forms and buttons, not a command language | Not owed (Known Limitations) |
| gNMI (`internal/component/gnmi/set.go`) | No: `SetRequest` carries only delete, replace and update | Not owed (Known Limitations) |

Grammar (`ai/rules/cli.md`, action before identifier, keyword before value): the
action `insert` first, then the path to the list, the entry being moved (its key
tokens), the keyword `before` or `after`, and the reference entry's key tokens.
For a leaf-list the existing form `insert <path> <member> first|last|before|after
[<ref>]` is unchanged; the target node kind decides which form applies.

Owner decision D-4 (Thomas, 2026-10-28) shapes what a move shows: on a list that is
not `ordered-by user` a move is no change in `show | compare`, diffs and prune, but
the committed file still records the new order; on an `ordered-by user` list a move
is a change.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/change-file-structural-ops.md` - structural ops in the per-user change file (`change_file.go` declares it)
  → Constraint: a move is one structural op line with the metadata prefix (`#user @source %time`), never decomposed into leaf deletes and creates; it is one pending change, applied by `SaveDraft()` and by the commit before the leaf edits
  → Constraint: structural directives exist in change files only; the materialized draft and the committed set-meta file carry no op, so the move must be visible there as line order alone, which the dependency spec makes true for every list
  → Constraint: `PendingChange` exists in `config` and `cli/contract` with identical fields; a new pending-change kind is added to both or the cast loses it
  → Constraint: the page's verb table gains the `move-entry` row in the phase that adds it
- [ ] `docs/guide/config-editor.md` - the editor guide; documents `insert <path> <member> <position> [<reference>]` for leaf-lists (line 115) and the stale-insert conflict
  → Constraint: gains the list form, its conflicts and the D-4 note in the same phase
- [ ] `docs/architecture/cli/root-namespace-grammar.md` - R1-R9 grammar
  → Constraint: `./le cli grammar` is run if the editor verbs are in the canonical verb registry (A-5)
- [ ] `docs/architecture/config/syntax.md` - declared by `tree.go`; documents list entry order and the printers
  → Constraint: after the dependency spec it states that entries print in recorded order; it gains one sentence that `insert` changes that order, pointing at `config-editor.md`, not a second copy of the command
- [ ] `docs/architecture/config/yang-config-design.md` - declared by the editor files this spec changes (config editor design)
  → Constraint: if it lists the editor's verbs, the `insert` row gains the list form; otherwise unaffected
- [ ] `docs/architecture/core-design.md` - declared by a file this spec changes
  → Constraint: no ordering or editor-verb statement is expected there; Phase 1 greps it for `insert` and for list order and edits only if one is found
- [ ] `docs/architecture/web-interface.md` - declared by the web files this spec changes (`cli_terminal.go`, `editor.go`)
  → Constraint: if it documents the web terminal's verbs or the keyless `insert`, it is updated with the D-7 outcome in Phase 3
- [ ] `plan/release/fixes/spec-config-list-entries-operator-order.md` - the dependency
  → Decision: recorded order prints everywhere and the D-4 comparison exists there; this spec only changes the recorded order
- [ ] `docs/contributing/ze-go-style.md` - precondition for the implementing agent's first Go edit

**Key insights:**
- The Tree already records entry order (`listOrder`); a move is a reorder of that slice and nothing else: no entry content changes.
- Every existing positional op is the leaf-list `insert-member`, whose replay, wait-for-entries and stale-conflict machinery is the pattern this op copies, not a new mechanism.
- The materialized draft and the committed file are set-meta, printed in `listOrder`, so a move survives a draft and a commit by line order once the dependency spec lands; only the per-user change file needs a new line kind.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/tree.go` - Tree list API
  → Constraint: `AddListEntry` appends to `listOrder`; `RenameListEntry` keeps the position; `CopyListEntry` inserts the copy right after the source; `RemoveListEntry` drops the key; `GetListOrdered` returns recorded order; there is no move
  → Constraint: `InsertMultiValue` (leaf-lists) refuses an invalid position (`isValidInsertPosition`) and uses `InsertFirst`, `InsertLast`, `InsertBefore`, `InsertAfter`; the list move reuses `InsertBefore` and `InsertAfter` and refuses `first` and `last` (D-2 names before and after only)
- [ ] `internal/component/cli/editor_commands.go` - `RenameListEntry`, `CopyListEntry` (session mode → `writeThroughRename`, `writeThroughCopy`; otherwise the Tree method, then `dirty`); `listEntryDestination` parses a destination with the schema's key codec (`config.ListEntryDestination`); `InsertLeafListValue` (session mode → `writeThroughMemberOp` with `StructuralOpInsertMember`)
- [ ] `internal/component/cli/model_commands_edit.go` - `cmdInsert` parses trailing `first|last` or `before|after <ref>`, then requires the target to be a leaf-list (`resolveLeafListValue`), else `errInsertFailedTargetIsNotA`
  → Constraint: the list form is decided by the same resolution: a target that resolves to a list entry takes the list form, a leaf-list the existing form; neither is guessed from token count
- [ ] `internal/component/cli/model_commands.go` - the verb dispatch `case cmdInsert` already exists; no new verb
- [ ] `internal/component/cli/completer.go` - `cmdInsert` ShortHelp "Insert into a leaf-list at position"; `cmdInsert` shares path completion with `set`, `delete`, `deactivate`, `activate`
  → Constraint: ShortHelp and completion after a list path gain the list form: entry keys, then `before`/`after`, then the other entries' keys
- [ ] `internal/component/config/change_file.go` - change-file tokens, `StructuralOpType` constants, `StructuralOp` fields (`ParentPath`, `ListName`, `OldKey`, `NewKey`, `Position`), the per-op `PendingChange` mapping, `isMemberOp`, and the parse and serialize of each line (`parseInsertMemberLine` and its writer)
  → Constraint: the op switches carry `//exhaustive:ignore` or name every op; a new op type is added to each switch the `exhaustive` linter covers
- [ ] `internal/component/cli/editor_draft.go` - `writeThroughStructuralOp`, `SaveDraft`, `applyStructuralOps` (one case per op; `default` panics on an unknown op), `applyStructuralOpsAroundEntries` and `memberOpWaitsForEntries` (an op that names something a leaf edit creates waits until after the entries), `renameAlreadyApplied` / `copyAlreadyApplied` (idempotent replay over an earlier save), `LoadDraft`, `detectConflicts`, `pendingChangesConflict`
  → Constraint: a move whose entry or reference is created by a leaf edit in the same session must wait for the entries, exactly as an `insert-member` with an absent member or reference does
  → Constraint: a replay over an earlier save must accept a move already in place (`moveAlreadyApplied`)
- [ ] `internal/component/cli/editor_commit.go` - `memberOpConflicts`, `memberOpStaleConflict` (stale when the path, the member or the before/after reference is gone from the committed tree), `insertSummary`, `memberOpFix`
  → Constraint: the move copies this: stale when the list, the entry or the reference is gone, with a fix that names the command to retype
- [ ] `internal/component/web/cli.go`, `cli_terminal.go`, `editor.go` - `verbInsert` is mapped to `webCommandConfigAdd` for authz and executes `mgr.CreateEntry` over `insert <path>` (help line "Insert a keyless list entry"); `verbRename`/`verbCopy` go through `execListEntryOp` and `EditorManager.RenameListEntry` / `CopyListEntry`
  → Constraint: the web terminal has no leaf-list insert at all today; it carries the verb with a different meaning (D-7)
- [ ] `internal/component/gnmi/set.go` - `Set` applies `Delete`, `Replace`, `Update` through the session; no verb surface

**Behavior to preserve:**
- The leaf-list `insert` form, its change-file line and its conflicts.
- `rename` keeps the position, `copy` places the copy after the source.
- Committed and draft file formats: no structural directive outside the change file.
- D-4: a move on a list that is not `ordered-by user` shows nothing in `show | compare`.

**Behavior to change:**
- `insert <list path> <entry> before|after <ref>` moves an existing entry, in the SSH editor and the web terminal, in direct and session mode.
- The per-user change file records the move as one `move-entry` line; drafts and commits apply it.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The editor command line (`Model.cmdInsert`) and the web terminal (`cli_terminal.go` `verbInsert`).

### Transformation Path
1. The command resolves the target with the schema: a list entry takes the list form.
2. The entry and reference key tokens are parsed by the list's key codec; both must name existing entries of the same list, and differ.
3. Direct mode: the Tree reorders `listOrder` and the editor marks itself dirty.
4. Session mode: one `move-entry` op is written to the change file and applied to the in-memory tree.
5. `SaveDraft` and the commit replay ops around the leaf edits; the move waits for entries a leaf edit creates; the commit pre-scan reports a stale move.
6. The materialized draft and the committed set-meta file print the new order (dependency spec).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Editor ↔ change file | One structural op line | Yes (`change_file.go` op parse and serialize) |
| Change file ↔ draft / commit | `applyStructuralOpsAroundEntries` | Yes (`editor_draft.go`) |
| Web terminal ↔ editor | `EditorManager` method calling the `Editor` method | Yes (`web/editor.go` `RenameListEntry` pattern) |

### Integration Points
- `Tree` gains a move method beside `RenameListEntry` and `CopyListEntry`.
- `Editor` gains a move method beside `RenameListEntry` and `CopyListEntry`.
- `StructuralOp` gains the move type, its line and its pending change.
- `EditorManager` (web) gains the move method beside `RenameListEntry`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Same editor and change-file path as `rename` and `copy` |
| No unintended coupling (components stay isolated) | Yes | web calls `EditorManager` → `Editor`, as for `rename` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses the `insert` verb, the key codec, the member-op wait and stale machinery |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Reorders one key slice |
| Registration over hardcoding, outbound | Yes | No new verb: `insert` exists in the dispatch, completion and the web verb table |
| Registration over hardcoding, inbound | Yes | The form is chosen by the schema node kind, never by a list name; the op joins the existing op switches, which `exhaustive` checks |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Once the dependency spec lands, the set-meta draft and committed file reproduce any `listOrder` on parse, for every list kind | Dependency AC-2, AC-3; set-meta parsers append on first mention through `AddListEntry` | A move is lost by a draft save or a commit | `TestMoveSurvivesDraftAndCommit` (save, reload, compare order) | unvalidated |
| A-2 | `MetaTree` holds no entry order of its own: entries are path-keyed, so a move needs no meta change | `MetaTree.RenameListEntry` and `CopyListEntry` exist only to rebase subtrees (structural-ops page, Constraints) | Blame or session views show the old order | `TestMoveKeepsMeta` over `SerializeSetWithMeta` and `SerializeBlame` | unvalidated |
| A-3 | The commit path writes the file when only the order changed, although D-4 makes `show | compare` empty for a plain list | Dependency R-6 and AC-7 | A move on a plain list is accepted, then silently not committed | `test/editor/session/insert-entry-plain-list-commit.et` reloads and checks order | unvalidated |
| A-4 | Resolving the target as a list entry versus a leaf-list is unambiguous for every schema path | `resolveLeafListValue`, `resolveListNode` | A leaf-list insert is read as a move or the reverse | `TestInsertFormChosenBySchema` over a list, a leaf-list and a list of a leaf-list | unvalidated |
| A-5 | The editor verbs are outside the canonical operational verb registry that `./le cli grammar` checks, or inside it with `insert` already declared | `internal/component/command/grammar` feeds the operational registry; the editor dispatch is `model_commands.go` | The grammar gate refuses the new form | Phase 1 runs `./le cli grammar` and records the result | unvalidated |
| A-6 | The web terminal's keyless `insert <path>` form has an editor equivalent, or can keep its bare form because the list form always ends with `before`/`after <ref>` | `cli_terminal.go` help "Insert a keyless list entry"; the editor's keyless entry creation is not yet traced | Two meanings of one verb on two surfaces (D-7) | Phase 0 traces how the SSH editor creates a keyless entry and records it under D-7 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A move whose entry or reference another session deletes or renames commits against a missing key | Commit pre-scan test | Stale conflict like `memberOpStaleConflict`, naming `insert` as the operator typed it and the fix (AC-8) |
| R-2 | Two sessions move the same entry to different places | Live conflict test | Two pending moves of one entry conflict (`pendingChangesConflict`), as two renames do (AC-9) |
| R-3 | A move recorded before a same-session rename of the entry or the reference replays against the old key | Draft replay test | The replay plan normalizes the move through later renames, as it does for copy (AC-10) |
| R-4 | A move of an entry created by a leaf edit in the same session runs before the entry exists | `SaveDraft` error | The move waits for entries, as `memberOpWaitsForEntries` holds back an `insert-member` (AC-7) |
| R-5 | A replay over an earlier save applies the move twice or refuses it | Repeated `SaveDraft` test | `moveAlreadyApplied`: the entry already sits before/after the reference is success (AC-11) |
| R-6 | Multi-key lists: the entry and reference need several key tokens each, and token boundaries are ambiguous | Multi-key unit test | Both are parsed by the list's key codec with the known key count; a short or long address is refused by name (AC-12) |
| R-7 | Changing the web terminal's `insert` meaning breaks a `.wb` test or a user who creates keyless entries there | `.wb` red | D-7 keeps the bare form where it does not collide; any changed expectation goes to `ze-test-judge` |
| R-8 | On a plain list the operator sees no diff after a move and believes it did nothing | User report | The success message states the new position, the pending-change list shows the move as one change, and the guide documents D-4 |
| R-9 | Authz: the web maps `verbInsert` to `webCommandConfigAdd`; a move is an edit of existing config | Authz test | The move needs the same permission as `rename` (`webCommandConfigRename`) or the editor's edit permission; Phase 2 names it with a test |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Entry order of one list; for an `ordered-by user` list (firewall terms) that is evaluation order, so a wrong move changes forwarding behavior |
| How is it reverted? | Revert the phase commits; existing change files never carry a `move-entry` line until this lands |
| Who else touches this path? | The dependency spec (printers, D-4); any session spec editing `editor_draft.go` or `editor_commit.go` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Editor `insert bgp peer zulu before alpha`, then `show` | → | `cmdInsert` → `Editor` move → `Tree` move → `Serialize` | `test/editor/commands/insert-list-entry.et` |
| Session editor move, `commit`, reload | → | `writeThroughStructuralOp` → change file `move-entry` → commit replay → set-meta file | `test/editor/session/insert-entry-commit.et` |
| Web terminal `insert` with `before` on a firewall term | → | `cli_terminal.go` → `EditorManager` move → `Editor` move | `TestWebTerminalInsertListEntry` (`internal/component/web/cli_terminal_insert_test.go`) |
| Change file line round trip | → | `move-entry` parse and serialize | `TestMoveEntryLineRoundTrip` (`internal/component/config/change_file_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peers zulu, alpha, mike; `insert bgp peer mike before zulu` | `show` lists mike, zulu, alpha; every printer agrees |
| AC-2 | Same list; `insert bgp peer zulu after mike` | alpha, mike, zulu |
| AC-3 | Firewall chain terms a, b, c (`ordered-by user`); `insert ... term c before a` | Order c, a, b; `show | compare` shows term c removed at its old place and added before a (D-4) |
| AC-4 | Plain list moved, then `show | compare` | No line (D-4); the pending-change list shows one move; after commit and reload `show` prints the new order |
| AC-5 | Entry or reference absent; entry equals reference; `first` or `last` given; target is not a list | Refused by name with the exact form to type, and the tree is unchanged |
| AC-6 | Leaf-list `insert <path> <member> before <ref>` | Unchanged behavior and change-file line |
| AC-7 | Session: `set bgp peer yankee description x` (creates yankee), then `insert bgp peer yankee before zulu`, `SaveDraft`, reload | yankee sits before zulu after reload |
| AC-8 | Session A moves zulu before alpha; session B deletes alpha and commits; A commits | A's commit reports a stale conflict naming the move, the missing reference and the fix |
| AC-9 | Sessions A and B both move zulu, to different places | A live conflict on the second commit, as for two renames |
| AC-10 | Session: move zulu before alpha, then rename alpha to bravo, `SaveDraft`, reload, commit | zulu sits before bravo |
| AC-11 | `SaveDraft` twice after a move | No error; order unchanged by the second save |
| AC-12 | A multi-key list: move one entry before another by full key tuple | Moved; a short or extra key token is refused by name |
| AC-13 | Web terminal: the AC-1 and AC-3 commands | Same result as the SSH editor; the bare keyless `insert <path>` form behaves as D-7 records |
| AC-14 | Tab completion after `insert bgp peer ` | Entry keys; after an entry, `before` and `after`; after those, the other entries' keys |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Moves a firewall term ahead of another and commits | editor → session op → commit → set-meta file → reload | `test/editor/session/insert-entry-commit.et` |
| 2 | Moves a peer in a draft, logs out, logs in, continues | change file → `SaveDraft` → `LoadDraft` | `test/editor/session/insert-entry-draft.et` |
| 3 | Moves a term from the web terminal | web terminal → `EditorManager` → editor | `TestWebTerminalInsertListEntry` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTreeMoveListEntry` | `internal/component/config/tree_move_test.go` | AC-1, AC-2, AC-5 at the Tree | |
| `TestMoveEntryLineRoundTrip` | `internal/component/config/change_file_test.go` | The `move-entry` line parse and serialize, with metadata prefix and quoted keys | |
| `TestMoveEntryPendingChange` | `internal/component/config/change_file_test.go` | One pending change per move, in `config` and `cli/contract` | |
| `TestEditorMoveListEntry` | `internal/component/cli/editor_move_test.go` | AC-1 to AC-5 direct and session mode | |
| `TestInsertFormChosenBySchema` | `internal/component/cli/editor_move_test.go` | A-4, AC-6 | |
| `TestMoveWaitsForEntries` | `internal/component/cli/editor_move_test.go` | AC-7 | |
| `TestMoveStaleConflict` | `internal/component/cli/editor_move_test.go` | AC-8 | |
| `TestMoveLiveConflict` | `internal/component/cli/editor_move_test.go` | AC-9 | |
| `TestMoveFollowsRename` | `internal/component/cli/editor_move_test.go` | AC-10 | |
| `TestMoveReplayIdempotent` | `internal/component/cli/editor_move_test.go` | AC-11 | |
| `TestMoveMultiKeyList` | `internal/component/cli/editor_move_test.go` | AC-12 | |
| `TestMoveSurvivesDraftAndCommit` | `internal/component/cli/editor_move_test.go` | A-1 | |
| `TestMoveKeepsMeta` | `internal/component/cli/editor_move_test.go` | A-2 | |
| `TestInsertListCompletion` | `internal/component/cli/completer_insert_test.go` | AC-14 | |
| `TestWebTerminalInsertListEntry` | `internal/component/web/cli_terminal_insert_test.go` | AC-13, R-9 authz | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | No numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `insert-list-entry` | `test/editor/commands/insert-list-entry.et` | AC-1, AC-2, AC-3, AC-5 | |
| `insert-entry-commit` | `test/editor/session/insert-entry-commit.et` | User story 1, AC-3 | |
| `insert-entry-plain-list-commit` | `test/editor/session/insert-entry-plain-list-commit.et` | AC-4, A-3 | |
| `insert-entry-draft` | `test/editor/session/insert-entry-draft.et` | User story 2, AC-7, AC-10 | |
| `cli-insert-list-entry` | `test/web/cli-insert-list-entry.wb` | AC-13 through the browser terminal | |

### Interop Tests (Scope: protocol)
N-A: an editor command; no wire-visible change.

## Files to Modify
- `internal/component/config/tree.go` - the Tree move method (reorders `listOrder`, refuses absent or equal keys)
- `internal/component/config/change_file.go` - the `move-entry` token, op type, line parse and serialize, `PendingChange` mapping, every op switch
- `internal/component/cli/contract/contract.go` - the pending-change kind mirrored (structural-ops page constraint)
- `internal/component/cli/editor_commands.go` - the `Editor` move method, direct and session mode
- `internal/component/cli/editor_draft.go` - `writeThroughStructuralOp` use, `applyStructuralOps` case, wait-for-entries, already-applied check, rename normalization, conflict overlap
- `internal/component/cli/editor_commit.go` - stale conflict and its operator-facing summary and fix
- `internal/component/cli/model_commands_edit.go` - `cmdInsert` list form and its usage error
- `internal/component/cli/completer.go` - `insert` ShortHelp and list-form completion
- `internal/component/web/editor.go` - `EditorManager` move method
- `internal/component/web/cli_terminal.go`, `cli.go` - `verbInsert` forms, authz mapping, help line
- `docs/architecture/config/change-file-structural-ops.md` - the `move-entry` row
- `docs/guide/config-editor.md` - the list form, its conflicts, D-4

## Files to Create
- `internal/component/config/tree_move_test.go`
- `internal/component/cli/editor_move_test.go`
- `internal/component/cli/completer_insert_test.go`
- `internal/component/web/cli_terminal_insert_test.go`
- `test/editor/commands/insert-list-entry.et`
- `test/editor/session/insert-entry-commit.et`
- `test/editor/session/insert-entry-plain-list-commit.et`
- `test/editor/session/insert-entry-draft.et`
- `test/web/cli-insert-list-entry.wb`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Editor verb; no YANG node |
| YANG validation constraints | No | None |
| YANG custom validators | No | None |
| CLI commands/flags | Yes | Editor `insert` list form (`model_commands_edit.go`), web terminal `insert` (`cli_terminal.go`) |
| CLI grammar (keyword before value) | Yes | `insert <list path> <entry> before|after <ref>`: action first, keyword before the reference value; `./le cli grammar` per A-5 |
| Editor autocomplete | Yes | `completer.go`, AC-14 |
| Functional test for new RPC/API | Yes | The `.et` and `.wb` tests above |
| Pipe completeness | No | The command answers a one-line status, no data output |
| Env var registration | No | None |
| Doctor check for runtime dependencies | No | No runtime dependency |
| Prometheus counters/metrics | No | None |
| BGP family surface (new SAFI / capability / attribute) | No | Not BGP family work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `docs/guide/config-editor.md`; the feature row for the config editor read from `./le feature report` and updated if it lists editor verbs |
| 2 | Config syntax changed? | No | The config file syntax is unchanged; the change file gains one line kind (row 12) |
| 3 | CLI command added/changed? | Yes | `docs/guide/config-editor.md`; the web terminal help line in `cli_terminal.go` |
| 4 | API/RPC added/changed? | No | gNMI unchanged |
| 5 | Plugin added/changed? | No | None |
| 6 | Has a user guide page? | Yes | `docs/guide/config-editor.md` |
| 7 | Wire format changed? | No | None |
| 8 | Plugin SDK/protocol changed? | No | None |
| 9 | RFC behavior implemented, changed, or newly proven? | No | None |
| 10 | Test infrastructure changed? | No | None |
| 11 | Affects daemon comparison? | Yes | Any Junos comparison page naming `insert`: grep `docs/` for it and state list support |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/change-file-structural-ops.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No new verb; `insert` exists |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Run `./le spec citation anchors spec plan/post-release/features/spec-config-list-entry-insert.md` at the start of Phase 1 and update every page it names; the run at design time named `docs/guide/web-interface.md` (web terminal verbs, updated with D-7 in Phase 3), `docs/guide/configuration.md` (list order), `docs/guide/audit.md` (web terminal command audit: a move is audited like `rename`), `docs/guide/authentication.md` (commit authz), `docs/guide/gnmi.md` (states no move, per Known Limitations) and `docs/guide/ipsec.md` (draft replay mention); each is checked and edited where it lists editor verbs or structural ops |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The leaf-list `insert` example in `config-editor.md` gains a list example beside it |

## Implementation Steps

Each phase is sized at about 60 tool calls.

0. **Phase: Measure and wire** -- run the scoped config, cli and web package tests and the `test/editor` suites on HEAD and keep the result; trace A-6 and record D-7's outcome in the spec; write the four Wiring Test rows; they fail on HEAD.
1. **Phase: Tree, editor, direct mode** -- the Tree move, the `Editor` move in direct mode, `cmdInsert` list form and usage error, completion and help; `./le cli grammar` per A-5; `config-editor.md` updated.
   - Tests: `TestTreeMoveListEntry`, `TestEditorMoveListEntry` (direct), `TestInsertFormChosenBySchema`, `TestInsertListCompletion`, `insert-list-entry.et`
2. **Phase: Session mode, drafts, commit** -- the `move-entry` op (line, pending change in both packages, op switches), session write-through, replay around entries, already-applied, rename normalization, live and stale conflicts; the structural-ops page row.
   - Tests: AC-4, AC-7 to AC-12, A-1, A-2, A-3, and the three session `.et` tests
3. **Phase: Web terminal and close-out** -- `EditorManager` move, `verbInsert` forms per D-7, authz (R-9), help line, `.wb` test; Critical Review.
   - Tests: AC-13, `TestWebTerminalInsertListEntry`, `cli-insert-list-entry.wb`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | All three user stories pass end to end |
| Correctness | The list form is chosen from the schema node kind, never from token count or a list name |
| Correctness | The move changes `listOrder` only: entry contents, meta and other lists are byte-identical before and after (`TestMoveKeepsMeta`) |
| Correctness | Every op switch in `change_file.go`, `editor_draft.go` and `editor_commit.go` names the new op; `exhaustive` lint clean on those files |
| Correctness | No structural directive reaches a draft or a committed file |
| Data flow | Web calls `EditorManager` → `Editor`; no web code edits the Tree directly |
| Rule: user-facing errors | Every refusal says what is wrong, why, and the exact command to type (`ai/rules/user-facing-errors.md`) |
| Rule: test changes | Any changed `.wb` or `.et` expectation carries a `ze-test-judge` approval line |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Editor command | `test/editor/commands/insert-list-entry.et` passes |
| Session commit and draft | `insert-entry-commit.et`, `insert-entry-draft.et`, `insert-entry-plain-list-commit.et` pass |
| Web parity | `TestWebTerminalInsertListEntry` and `cli-insert-list-entry.wb` pass |
| Docs | `config-editor.md` and `change-file-structural-ops.md` describe the list form |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Entry and reference keys parsed by the list's key codec and checked to exist; quoted keys in the change file use the configuration key quote codec both ways |
| Authorization | The web terminal move needs an edit permission (R-9); a read-only user is refused, proven by test |
| Change file injection | A key holding spaces or the `before`/`after` words round-trips through the quote codec without being read as a keyword |

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
| D-2 | Reordering entries | Only delete and re-add reorders; `insert` covers leaf-lists only | `insert <list> <entry> before|after <entry>` on every list | Thomas, 2026-10-28 |
| D-4 | What a move shows | Plain list order carries no meaning; `ordered-by user` order is behavior | Plain list: no change in compare, committed order still written; `ordered-by user`: a change | Thomas, 2026-10-28 |
| D-7 | The web terminal's `insert` | It means "create a keyless list entry" there, and the web has no leaf-list insert | The web verb takes the editor's leaf-list and list forms; the bare `insert <path>` keeps creating a keyless entry only if A-6 finds no editor equivalent, since it never ends in a position keyword and cannot collide; otherwise it moves to the editor's equivalent | Derived from D-2 (editor verbs on every surface that carries them); outcome recorded in Phase 0 |
| D-8 | `first` and `last` | D-2 names before and after only; Junos list `insert` likewise | Refuse `first` and `last` for a list, naming the before/after form | Derived from D-2 |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Owner decision D-2 (Thomas, 2026-10-28): a Junos-style `insert <list> <entry> before|after <entry>` for every list, `ordered-by user` included, in the editor and in web and gNMI where their command surface carries editor verbs | Delete and re-add only | Order is visible once the dependency spec lands; it must be changeable |
| Owner decision D-4 (Thomas, 2026-10-28): a plain-list reorder is no change in compare, diffs and prune; an `ordered-by user` reorder is a change | - | Implemented by the dependency spec; this spec relies on it |
| A move is one structural op in the change file, `move-entry` | Decompose into delete and re-create; rely on the materialized draft's line order alone | The structural-ops page forbids decomposition (one pending change, conflict detection); the per-user change file holds ops, not order, so without the op a session commit replays no move |
| Reuse the `insert` verb, chosen by node kind | A new `move` verb | D-2 names `insert`, Junos spells it so, and the verb already exists in the dispatch, completion and web verb tables |

## Known Limitations
- gNMI `Set` carries only delete, replace and update; no editor verb surface exists there, so no move is owed (D-2's condition). A gNMI client reorders by replacing the list.
- The web editor's forms carry no command language; a drag or up/down control there is not part of D-2.

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
- [ ] **Commit B:** `remove plan/post-release/features/spec-config-list-entry-insert.md` only, in the same `./le commit create` script

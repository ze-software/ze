# Spec: session-editor-file-mode-parity

| Field | Value |
|-------|-------|
| Status | design |
| Scope | cli |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner expectation (2026-10-10): "I thought it was already existing". An operator
connected over SSH to a running router expects `load ... merge` and
`commit confirmed` to work. Today they work only in file mode
(`ze config edit -f <file>`, no daemon). The editor every running daemon serves
(SSH, the attached console, the web terminal) runs in session mode, and session
mode refuses them.

Owner decision (2026-10-10, "yes that too"): every verb of the same class is in
scope, each with its own AC and a functional test through the SSH entry point.
The class is: a verb file mode performs that session mode refuses against the
running daemon. The sweep of `internal/component/cli/` found six:

| Refused verb | Refusal (source) | Reason the code gives |
|--------------|------------------|-----------------------|
| `load <file\|terminal> <absolute\|relative> <merge\|replace> [path]` | `errLoadNotSupportedInSessionMode`, `cmdLoadNew` (`model_load.go`) | the loaded tree produces no per-leaf change entries |
| `copy <list> <src> to <dst>` | `errCopyNotSupportedInSessionMode`, `Editor.CopyListEntry` (`editor_commands.go`) | creates structure without write-through |
| `deactivate` on a leaf or a path | `errDeactivateNotSupportedInSessionMode`, `Editor.DeactivateLeaf`, `Editor.DeactivatePath` | no write-through for the inactive marker |
| `activate` on a leaf or a path | `errActivateNotSupportedInSessionMode`, `Editor.ActivateLeaf`, `Editor.ActivatePath` | same |
| `commit [force] confirmed <seconds>` | `errCommitConfirmedNotYetSupportedIn` (`model_commands.go`, `cmdCommand` commit arm) | needs session-aware rollback |
| `commit force` | `errCommitForceNotYetSupportedIn`, `cmdCommitForce` (`model_commands_commit.go`) | session commit has its own validation path |

Swept and NOT the class (no change):

| Branch | Why it is not the class |
|--------|-------------------------|
| `who`, `disconnect`, `show \| blame`, `show \| changes` | the inverse: they need a session and are refused in file mode |
| `discard` needs a path or `all` in session mode | a grammar difference, not a refusal |
| `insert`, and `deactivate`/`activate` of a leaf-list member | already write through (`writeThroughMemberOp`) |
| `rollback <N>` | not refused in session mode (see A-6) |

Goal: each of the six works in session mode against the running daemon, records
what it changed as the same tracked change entries `set`/`delete` produce (so
`show | compare`, `show | changes`, blame and conflict detection see it), and
`commit` applies it to the running daemon. `commit confirmed` auto-reverts even
when the SSH session that issued it is gone, because that is the lockout the
command exists to survive.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/config-editor.md` - the page for the editor, its modes, the blocked-command table, and commit confirmed
  → Constraint: the page lists the six verbs as blocked and says "Use file mode for these operations"; every AC that unblocks one removes its row in the same phase (`ai/rules/documentation.md`)
  → Constraint: `commit confirmed <seconds>` takes 1 to 3600 SECONDS, not minutes. The showcase spec's old AC-14 said minutes; the source (`cmdCommitConfirmed`, `errTimeoutMustBeAtMost3600`) is seconds and stays seconds
  → Decision: session mode is every daemon-served editor: `ze config edit` (SSH), the attached console, and the web editor; the fix lands in `Editor` and so reaches all three
- [ ] `docs/architecture/config/yang-config-design.md` - declared by `model_load.go` (`// Design:`); config editor design
  → Constraint: named in Files to Modify; updated where it describes session write-through, otherwise named unaffected with the reason at implementation
- [ ] `docs/architecture/config/change-file-structural-ops.md` - why rename is one structural op, not decomposed leaf edits
  → Decision: "A mass of synthetic set and delete entries hides what the operator meant", so `copy` and leaf/path `deactivate`/`activate` are each ONE structural op (`copy-entry`, `deactivate-leaf`, `activate-leaf`, `deactivate-path`, `activate-path`), counted as one pending change, applied by `SaveDraft()` before leaf edits like rename
  → Decision: `load` stays per-leaf entries: the owner asked for the entries `set` produces, and a load is many operator statements, not one
  → Constraint: `PendingChange` exists in both `config` and `contract` with identical fields; a new op kind is added to both or the cast loses it in silence
  → Constraint: structural directives exist in change files only; the draft and committed formats stay unchanged
- [ ] `docs/architecture/hub-architecture.md` - how an SSH or web commit stages a candidate and how startup recovers
  → Constraint: an SSH/web commit stages a candidate and the hub compares the explicit file with the accepted bytes; the confirm revert stages the rollback revision through the same candidate path
  → Decision: the pending-confirm record is recovered at startup in the same pass that recovers an interrupted file-commit intent, before candidate cleanup (AC-19)
- [ ] `docs/architecture/zefs-format.md` - store layout, declared by the storage code this spec changes
  → Constraint: the pending-confirm record is a new key under `meta/config/<name>/` beside `active`, `candidate`, `rollback`; the page's key table gains its row
- [ ] `ai/rules/config.md` - config manipulation
  → Constraint: config content MUST be manipulated as a parsed YANG tree or as `set` lines; "Raw text surgery, a custom merge function that parses config syntax outside the config system" MUST NOT be used. File mode's `mergeConfigs`, `mergeAtContext`, `replaceAtContext` (`model_load.go`) are line-and-brace text surgery, so the session path cannot reuse them
  → Decision: load parses the input with the schema parser into a tree, merges or replaces at the tree level, and that ONE path serves both modes; the text functions are deleted (`ai/rules/no-layering.md`)
- [ ] `ai/rules/cli.md` - command grammar and errors
  → Constraint: no new verb or flag; the existing grammar is kept. Every error says what failed, why, and what to do next
- [ ] `ai/rules/goroutine-lifecycle.md` - the confirm-window timer
  → Constraint: the daemon-owned deadline is one long-lived worker started with the daemon, never a goroutine per commit

**Key insights:**
- Session mode writes every edit through to a per-user change file (`ChangePath`) under the store lock: a sparse tree, a meta tree (user, origin, time, previous value) and structural ops (`config.StructuralOp*` in `internal/component/config/change_file.go`). Commit reads them, detects LIVE/STALE conflicts, and applies them.
- The refused verbs all mutate `e.tree` directly. The fix is to make each one write through, using the vocabulary that exists plus new structural ops where none fits.
- The commit-confirmed countdown lives in the TUI `Model` (`handleConfirmCountdown`, a bubbletea tick). In session mode the Model runs inside the daemon per SSH channel, so the tick dies with the channel: a dropped SSH session would never revert. The window MUST be owned by the daemon.
- `Editor.HasPendingLive` has no non-test caller: an orphaned `.live.conf` is never acted on (journal row, `plan/journal/unwired-feature.md`).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cli/model_commands.go` - dispatch; commit arm refuses `confirmed` when `HasSession()`, routes plain commit to `cmdCommitSession`
- [ ] `internal/component/cli/model_load.go` - `cmdCommitConfirmed` (validate with `ValidateTransition`, require history, `saveLive`, `CommitSession` or `Save`, `tryReload`, newest backup becomes the rollback target, tick countdown), `cmdConfirm`, `cmdAbort`, `rollbackConfirmed`, `handleConfirmCountdown`; `cmdLoadNew` refuses in session mode; `applyLoadAbsolute`/`applyLoadRelative` call the text merge functions and `setWorkingContent`
- [ ] `internal/component/cli/model_commands_commit.go` - `cmdCommitSession` (validate, `CommitSessionCandidate` when a reload notifier exists, `NotifyReload`, `ClearCandidate` on failure, `MarkCommittedContent`), `cmdCommitForce` refuses in session mode, `cmdRollback` calls `Editor.Rollback`
- [ ] `internal/component/cli/editor_commands.go` - `CopyListEntry`, `DeactivateLeaf`, `ActivateLeaf`, `DeactivatePath`, `ActivatePath` refuse when `e.session != nil`; `RenameListEntry` and the leaf-list member ops route to write-through
- [ ] `internal/component/cli/editor_draft.go` - `writeThroughSet`, `writeThroughCreate`, `writeThroughDelete`, `writeThroughRename`: lock, validate on a clone, read change file, mutate, record meta with `Previous` from the committed tree, serialize, write, then update the in-memory tree
- [ ] `internal/component/cli/editor.go` - `livePath`, `saveLive`, `HasPendingLive` (no caller), `deleteLive`, `Rollback` (writes the backup to the original path directly, re-parses the tree)
- [ ] `internal/component/cli/model_keys.go` - paste mode end calls `applyLoadAbsolute`/`applyLoadRelative`
- [ ] `internal/component/config/change_file.go` - change-file tokens and `StructuralOpType` values; `PendingChangeDeactivate`/`PendingChangeActivate` kinds already exist
- [ ] `internal/component/web/editor.go`, `internal/component/web/cli_terminal.go` - web editor runs with a session (`SetSession`); its `copy`, `deactivate`, `activate` call the same `Editor` methods and are refused today; it has no `load` or `commit confirmed` verb
- [ ] `cmd/ze/hub/session_editor.go`, `cmd/ze/hub/editor_adapter.go` - SSH session editors and the adapter exposing copy/deactivate/activate
- [ ] `internal/component/config/cli/cmd_deactivate.go` - offline `ze config deactivate|activate` call the same methods on a non-session editor (unaffected)
- [ ] `internal/component/config/storage/pointer.go` - `active`, `candidate`, `rollback` pointers under `meta/config/<name>/`
- [ ] `internal/test/fixture/plugin_fixture_04_cli.go` - `cliCommitDriver04`/`driveEditor04`: `ze init`, then `ze config edit` over SSH against a running daemon; the pattern for the new fixtures
- [ ] `test/editor/session/load-blocked.et` - asserts the refusal this spec removes

**Behavior to preserve:**
- File-mode results for every verb: same status messages, same `commit confirmed` messages and countdown, same boundary errors (1 to 3600 seconds), same history requirement (`errCommitConfirmedNeedsHistory`)
- The grammar of `load`, `copy`, `deactivate`, `activate`, `commit [force] [confirmed <seconds>]`, `confirm`, `confirm abort`
- Session commit semantics: conflict detection, `CommitSessionCandidate` + `NotifyReload` transactional path, `ClearCandidate` on reload failure
- Offline `ze config deactivate|activate` and their stdin pipeline form
- Other users' pending change files are never touched by load, copy or the confirm revert

**Behavior to change:**
- The six refusals are removed and each verb writes through in session mode
- File-mode load moves from text surgery to the tree path (same observable result, see A-2)
- `commit confirmed` in session mode is owned by the daemon
- `test/editor/session/load-blocked.et` is replaced by tests of the new behavior (the owner changed the behavior; this is not weakening a test)

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator keystrokes over the SSH PTY into the daemon's per-session `Model` (`cmd/ze/hub/session_editor.go`); the web terminal POST into `cli_terminal.go`
- Load input: a file in the store or on disk (`resolveConfigPath`), or pasted text ended by Ctrl-D; hierarchical or set format

### Transformation Path
1. Dispatch (`model_commands.go`) reaches the verb with `HasSession()` true
2. Load: parse input with the schema parser into a tree; refuse on any parse error or unknown key before touching anything
3. Load: build the target tree on a clone of the session tree (merge, or replace at root or context path)
4. Load: diff target against the session tree into change entries: set per added or changed leaf, delete per removed leaf, delete-entry/delete-container/delete-list per removed structure, inactive markers per marker difference
5. Write all entries to the change file in ONE lock hold and one write, then update the in-memory tree and meta (all or nothing)
6. Copy: one `copy-entry` structural op, applied at save before leaf edits, and in memory
7. Deactivate/activate leaf or path: a structural op recorded in the change file, applied at commit, and in memory
8. Commit (plain, force, confirmed): `CommitSessionCandidate` then `NotifyReload` to the running daemon
9. Confirmed: the daemon records a pending-confirm record (deadline, rollback revision, user, session) in the store and arms its one deadline worker; confirm clears it; abort or deadline restages the rollback revision as a candidate and reloads

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| SSH client ↔ daemon editor | PTY keystrokes, rendered screen | No |
| Editor ↔ store | change file, draft, candidate, pointers under the store lock | No |
| Editor ↔ daemon reload | `NotifyReload` | No |
| Daemon confirm worker ↔ store and reload | pending-confirm record, candidate restage, reload | No |

### Integration Points
- `writeThroughSet`/`writeThroughDelete`/`writeThroughCreate` - the entry shapes load and copy emit; a batched form takes many entries under one lock
- `config.StructuralOp` and `SerializeChangeFile`/`ParseChangeFile` - new op types for leaf/path deactivate and activate
- `applyStructuralOps` (`editor_draft.go`) - applies the new ops at save and commit
- `CommitSessionCandidate` - shared by plain, force and confirmed session commits
- `storage/pointer.go` - home of the pending-confirm record beside `candidate` and `rollback`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to fill at implementation: every verb reaches the change file, never `e.tree` alone |
| No unintended coupling (components stay isolated) | No | the confirm worker lives with the daemon's config ownership, the CLI Model only asks it |
| No duplicated functionality (extends existing, does not recreate) | No | one load path for both modes; write-through helpers reused |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A for wire; trees are cloned once per load |
| Registration over hardcoding, outbound | No | no new command; new structural ops join the existing token set |
| Registration over hardcoding, inbound | No | lists to search at implementation: `StructuralOpType` constants, `ChangeFile*Token`, `PendingChangeKind`, `applyStructuralOps` switch, `filterStructuralOps`, the change-file parser |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The set-format change file can carry inactive markers for leaves and paths | `serialize_set.go` and `parser_list.go` handle `IsInactive`/`IsLeafInactive`; `PendingChangeDeactivate` exists | new tokens need parser work beyond the op list | round-trip unit test of the new ops through `SerializeChangeFile`/`ParseChangeFile` | unvalidated |
| A-2 | Tree-level merge gives the same result file-mode text merge gave for every existing load test | `mergeConfigs` merges top-level keys; tree merge is a superset | a file-mode load test changes result | run every existing `test/editor/**/load*.et` and `model_load` unit test unchanged | unvalidated |
| A-3 | A loaded subtree of a few thousand leaves writes through in one lock hold within the editor's command latency | write-through is one serialize + one write | large paste stalls the session | unit test with a 5000-leaf load, timed | unvalidated |
| A-4 | The daemon can host one confirm worker per config name, started with the daemon | the daemon owns the store and the reload | needs a new component home | Phase 1 wiring test | unvalidated |
| A-5 | A reloaded rollback revision restores the running daemon's behavior (not only the file) | transactional commit path promotes a candidate and reloads | revert changes the file but not the daemon | `.ci` asserts daemon state, not only the file | unvalidated |
| A-6 | `rollback <N>` in session mode writes the config file directly without the candidate/reload path | `Editor.Rollback` writes `originalPath`; `cmdRollback` does not reload | the confirm revert cannot reuse `Editor.Rollback` | read and test at Phase 4; the revert uses the candidate path regardless | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | SSH drops during a confirm window and nothing reverts (today's file-mode behavior, carried over) | disconnect `.ci` keeps the trial config | the window is daemon-owned (AC-15) |
| R-2 | A second commit lands inside a window and the revert erases it | another session commits during the window | commits during a pending window are refused (AC-18) |
| R-3 | Daemon restarts during a window and starts on the unconfirmed config | restart `.ci` | the pending record is read at start and reverts before apply (AC-19) |
| R-4 | A partial load leaves half the entries written | parse or schema error mid-way | parse and diff complete before the lock; one write (AC-5) |
| R-5 | Load replace deletes structure another user has pending edits under | conflict at commit | deletes carry `Previous`; commit's LIVE/STALE detection reports them (AC-6) |
| R-6 | Load of a leaf whose value equals the session value records a no-op entry | `show \| changes` lists unchanged leaves | the diff emits only differences (AC-1) |
| R-7 | Copy of an entry holding secrets exposes them in `show \| changes` | display inspection | the copied entry renders through the same masking every display path uses |
| R-8 | Deleting the text merge functions breaks a caller outside `model_load.go` | build | `gopls references` before deletion |
| R-9 | Web `copy`/`deactivate`/`activate` start working with no web test | none | AC-11 adds a `.wb` test |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A session commit applies the wrong config to a running router, or a confirm window fails to revert and leaves an operator locked out |
| How is it reverted? | single commit revert; no config format migration (the change file gains op tokens only a newer editor writes) |
| Who else touches this path? | `plan/spec-terminal-demo-showcase.md` depends on it (its old AC-13/AC-14); the web editor; any spec touching `editor_draft.go` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SSH `load file absolute merge <path>` | → | `cmdLoadNew` session branch, batched write-through | `test/plugin/session-editor-load-merge.ci` |
| SSH `load terminal relative merge` + Ctrl-D | → | paste-mode end, same path | `test/plugin/session-editor-load-terminal-relative.ci` |
| SSH `load file absolute replace <path>` | → | replace diff to delete entries | `test/plugin/session-editor-load-replace.ci` |
| SSH `copy` | → | `CopyListEntry` write-through | `test/plugin/session-editor-copy.ci` |
| SSH `deactivate`/`activate` leaf and path | → | new structural ops | `test/plugin/session-editor-deactivate-activate.ci` |
| SSH `commit force` | → | session force commit | `test/plugin/session-editor-commit-force.ci` |
| SSH `commit confirmed <s>` + `confirm` | → | daemon confirm record | `test/plugin/session-editor-commit-confirmed-confirm.ci` |
| SSH `commit confirmed <s>`, no confirm | → | daemon deadline worker | `test/plugin/session-editor-commit-confirmed-timeout.ci` |
| SSH `commit confirmed <s>`, client killed | → | daemon deadline worker | `test/plugin/session-editor-commit-confirmed-disconnect.ci` |
| Web terminal `copy`/`deactivate`/`activate` | → | same `Editor` methods | `test/web/cli-session-copy-deactivate.wb` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | SSH editor on a running daemon; `load file absolute merge <path>` with a file adding two leaves and changing one | `show \| changes` lists exactly three set entries attributed to the session user; `show \| compare` lists exactly those three leaves; leaves equal to the current value produce no entry; `commit` applies them and the running daemon reports the new values |
| AC-2 | SSH editor at context `bgp peer peer1`; `load terminal relative merge`, paste, Ctrl-D | entries are rooted under `bgp peer peer1`; nothing outside it changes; `commit` applies |
| AC-3 | SSH editor; `load file absolute replace <path>` where the file omits a leaf and a list entry the config holds | `show \| changes` lists a delete for the leaf and a delete-entry for the list entry plus sets for the differences; after `commit` the running config equals the file |
| AC-4 | SSH editor at a context path; `load ... relative replace` | the replace is scoped to the context subtree; siblings outside it are untouched |
| AC-5 | SSH editor; load input with a syntax error or an unknown key | refused with the parser's error naming the line and the closest valid key; the change file, draft and in-memory tree are unchanged |
| AC-6 | Two SSH sessions; session B has a pending set on a leaf; session A loads a value for the same leaf and commits | session A's commit reports the LIVE conflict exactly as it does for `set` |
| AC-7 | File mode (`ze config edit -f`) `load` merge and replace, absolute and relative | results equal today's for every existing file-mode load test; the text merge functions no longer exist |
| AC-8 | SSH editor; `copy <list> <src> to <dst>` | `show \| changes` lists ONE copy change attributed to the user, and `show \| compare` shows the new entry; `commit` applies and the running daemon holds both entries; a destination that exists is refused as in file mode |
| AC-9 | SSH editor; `deactivate` on a leaf and on a path | `show \| changes` lists each as a deactivate; `commit` applies; the running daemon treats them as absent; `show` marks them inactive |
| AC-10 | SSH editor; `activate` on a leaf and a path that are inactive in the committed config | `show \| changes` lists each as an activate; after `commit` the daemon uses them again; activating an active node gives the existing `ErrLeafNotInactive`/`ErrPathNotInactive` message |
| AC-11 | Web terminal (session editor) `copy`, `deactivate`, `activate` | each succeeds and shows in pending changes; commit applies |
| AC-12 | SSH editor with a change that raises only warnings; `commit force` | the warnings are skipped, the change applies to the running daemon; with an error, `commit force` is blocked naming the error |
| AC-13 | SSH editor; `commit confirmed 60` then `confirm` within 60 s | the change applies to the running daemon at once; status says to confirm within 60 s; after `confirm` it stays applied, and no revert happens after 60 s |
| AC-14 | SSH editor; `commit confirmed 5`, no `confirm` | after 5 s the daemon restores the previous revision and the running daemon reports the previous values; an attached session shows the file editor's timeout message |
| AC-15 | SSH editor; `commit confirmed 5`, then the SSH client is killed | the daemon still reverts after 5 s; the running daemon reports the previous values |
| AC-16 | SSH editor; `commit confirmed 60` then `confirm abort` | the previous revision is applied at once; message as in file mode |
| AC-17 | SSH session reconnects (same user) during a window | the editor shows the pending window and seconds left; `confirm` from this session confirms it |
| AC-18 | During a pending window another session (or the same) runs `commit` or `commit confirmed` | refused, naming the pending window, its owner and seconds left, and saying to `confirm` or `confirm abort` first |
| AC-19 | Daemon restarted during a pending window | at start the daemon finds the pending record, restores the rollback revision before applying config, and logs that it reverted an unconfirmed commit |
| AC-20 | `commit confirmed 0`, `3601`, `abc`, and no argument in session mode | the file-mode errors: at least 1, at most 3600, invalid seconds, usage |
| AC-21 | Session `commit confirmed` on a daemon store with no history | refused before writing, as `errCommitConfirmedNeedsHistory` |
| AC-22 | `docs/guide/config-editor.md` | the blocked-command table and "Use file mode for these operations" are gone; the modes table and the Commit Confirmed section state that the daemon owns the window and survives a dropped session |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | merges a feature section into a running router over SSH and commits | SSH PTY → Model → load → change file → commit → reload | `test/plugin/session-editor-load-merge.ci` |
| 2 | makes a risky change with `commit confirmed`, loses the SSH session, and gets the old config back | SSH → commit confirmed → daemon record → worker → candidate → reload | `test/plugin/session-editor-commit-confirmed-disconnect.ci` |
| 3 | deactivates a peer over SSH, commits, then re-activates it | SSH → structural op → commit → reload | `test/plugin/session-editor-deactivate-activate.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSessionLoadMergeEmitsPerLeafEntries` | `internal/component/cli/model_load_session_test.go` | AC-1, R-6 | |
| `TestSessionLoadReplaceEmitsDeletes` | same | AC-3, AC-4 | |
| `TestSessionLoadRefusesBadInputAtomically` | same | AC-5, R-4 | |
| `TestSessionLoadLargeInputOneWrite` | same | A-3 | |
| `TestFileModeLoadTreeMergeMatchesPrevious` | `internal/component/cli/model_load_test.go` | AC-7, A-2 | |
| `TestSessionCopyWritesThrough` | `internal/component/cli/editor_draft_test.go` | AC-8, R-7 | |
| `TestSessionDeactivateActivateLeafAndPath` | same | AC-9, AC-10 | |
| `TestChangeFileDeactivateOpsRoundTrip` | `internal/component/config/change_file_test.go` | A-1 | |
| `TestSessionCommitForce` | `internal/component/cli/model_commands_commit_test.go` | AC-12 | |
| `TestConfirmWindowWorkerRevertsAtDeadline` | owning package of the worker | AC-14, AC-15 | |
| `TestConfirmWindowRefusesCommitWhilePending` | same | AC-18 | |
| `TestConfirmWindowRevertsOnStart` | same | AC-19 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| commit confirmed seconds | 1-3600 | 1 and 3600 | 0 | 3601 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `session-editor-load-merge` | `test/plugin/session-editor-load-merge.ci` | AC-1 | |
| `session-editor-load-terminal-relative` | `test/plugin/session-editor-load-terminal-relative.ci` | AC-2 | |
| `session-editor-load-replace` | `test/plugin/session-editor-load-replace.ci` | AC-3, AC-4 | |
| `session-editor-load-refused` | `test/plugin/session-editor-load-refused.ci` | AC-5 | |
| `session-editor-load-conflict` | `test/plugin/session-editor-load-conflict.ci` | AC-6 | |
| `session-editor-copy` | `test/plugin/session-editor-copy.ci` | AC-8 | |
| `session-editor-deactivate-activate` | `test/plugin/session-editor-deactivate-activate.ci` | AC-9, AC-10 | |
| `cli-session-copy-deactivate` | `test/web/cli-session-copy-deactivate.wb` | AC-11 | |
| `session-editor-commit-force` | `test/plugin/session-editor-commit-force.ci` | AC-12 | |
| `session-editor-commit-confirmed-confirm` | `test/plugin/session-editor-commit-confirmed-confirm.ci` | AC-13, AC-17 | |
| `session-editor-commit-confirmed-timeout` | `test/plugin/session-editor-commit-confirmed-timeout.ci` | AC-14 | |
| `session-editor-commit-confirmed-disconnect` | `test/plugin/session-editor-commit-confirmed-disconnect.ci` | AC-15 | |
| `session-editor-commit-confirmed-abort` | `test/plugin/session-editor-commit-confirmed-abort.ci` | AC-16, AC-18 | |
| `session-editor-commit-confirmed-restart` | `test/plugin/session-editor-commit-confirmed-restart.ci` | AC-19 | |
| `session-editor-commit-confirmed-boundary` | `test/plugin/session-editor-commit-confirmed-boundary.ci` | AC-20, AC-21 | |
| replaces `load-blocked.et` | `test/editor/session/load-merge.et` | AC-1 at model level | |

Each `.ci` drives `ze config edit` over SSH against the running daemon through a
fixture modelled on `driveEditor04` (`internal/test/fixture/plugin_fixture_04_cli.go`),
and asserts the running daemon's state (a `show` through `ze cli -c`), not only
the stored file (A-5). Each is run red against the unfixed tree first.

### Interop Tests (Scope: protocol)
N-A: no wire-visible change; the editor applies config through the existing reload.

## Files to Modify
- `internal/component/cli/model_load.go` - session load path, tree-based load for both modes, delete `mergeConfigs`/`mergeAtContext`/`replaceAtContext`; `cmdCommitConfirmed` session branch asks the daemon window
- `internal/component/cli/model_keys.go` - paste-mode end uses the same load path
- `internal/component/cli/model_commands.go` - remove the confirmed refusal
- `internal/component/cli/model_commands_commit.go` - session `commit force`; commit refused while a window is pending
- `internal/component/cli/editor_commands.go` - copy, deactivate, activate write through; remove the four sentinel errors
- `internal/component/cli/editor_draft.go` - batched write-through; apply the new structural ops
- `internal/component/cli/editor.go` - session confirm helpers; `HasPendingLive` wired or deleted
- `internal/component/config/change_file.go` (+ its parser/serializer) - deactivate/activate leaf and path ops
- `internal/component/config/storage/pointer.go` - pending-confirm record
- the `contract` package twin of `PendingChange` - the new op kinds in both types
- `docs/architecture/config/change-file-structural-ops.md` - the new ops
- `docs/architecture/hub-architecture.md` - confirm window and its startup recovery
- `docs/architecture/zefs-format.md` - the pending-confirm key
- `cmd/ze/hub/session_editor.go` - hand the session editor the daemon confirm window
- the daemon start path that applies config (found in Phase 1) - revert on start (AC-19)
- `test/editor/session/load-blocked.et` - removed, replaced by `load-merge.et`
- `docs/guide/config-editor.md` - AC-22
- `docs/architecture/config/yang-config-design.md` - session write-through, or named unaffected
- `docs/guide/command-reference.md` - if it marks any of the six as file-mode only

## Files to Create
- `internal/component/cli/model_load_session_test.go`
- the confirm-window worker and its test, in the package Phase 1 chooses
- `internal/test/fixture/plugin_fixture_NN_session_editor.go` - SSH editor driver with one mode per `.ci`
- the 15 `.ci` files and one `.wb` file named in the Functional Tests table
- `test/editor/session/load-merge.et`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new RPC or leaf |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | No | grammar unchanged |
| CLI grammar (keyword before value) | No | unchanged |
| Editor autocomplete | Yes | `completer.go`: stop hiding the six in session mode if it filters them |
| Functional test for new RPC/API | Yes | the `.ci` files above |
| Pipe completeness | N-A | no new output command |
| Env var registration | No | none |
| Doctor check for runtime dependencies | No | no new path, socket or binary; the pending record lives in the existing store |
| Prometheus counters/metrics | No | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | the editor's `features/<id>.md` if it records session-mode limits (check `./le feature report`) |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` if it marks the six as file-mode only |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/config-editor.md` |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if the new fixture adds a mode to document |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it compares commit confirmed with Junos/VyOS |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | none |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/immediate/spec-session-editor-file-mode-parity.md` at implementation; `config-editor.md` anchors the four sentinel errors this deletes |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `config-editor.md` Example Workflow and the `terminal-demo: commit-confirmed` marker |

## Implementation Steps

1. **Phase: Wiring** -- fixture driving `ze config edit` over SSH against a running daemon; the wiring `.ci` files written and red on the refusals; choose the confirm worker's home
   - Tests: the Wiring Test rows
   - Verify: each `.ci` fails on today's refusal message
2. **Phase: Structural ops** -- deactivate/activate leaf and path ops in the change file; copy write-through
   - Tests: `TestChangeFileDeactivateOpsRoundTrip`, `TestSessionDeactivateActivateLeafAndPath`, `TestSessionCopyWritesThrough`, AC-8 to AC-11 `.ci`/`.wb`
3. **Phase: Load** -- tree-based load for both modes, diff to entries, one-write batch; delete text merge
   - Tests: AC-1 to AC-7
4. **Phase: Commit force** -- AC-12
5. **Phase: Daemon confirm window** -- pending record, worker, refusal while pending, revert on start, session Model shows remaining time
   - Tests: AC-13 to AC-21
6. **Phase: Docs** -- AC-22 and the checklist rows, each in the phase that changed the behavior

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | load emits only differences; replace emits deletes; one lock and one write per load |
| Correctness | the revert restores the daemon's running behavior, proven by a `show` through the daemon |
| Data flow | no session verb mutates `e.tree` without the change file |
| Rule: config.md | no text surgery remains in load |
| Rule: goroutine-lifecycle.md | one long-lived confirm worker, stopped with the daemon |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| six refusals gone | grep for the six sentinel error names returns nothing |
| text merge gone | grep for `mergeConfigs`, `mergeAtContext`, `replaceAtContext` returns nothing |
| functional tests | the 15 `.ci` and 1 `.wb` pass, each observed red first |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | loaded content goes through the schema parser; unknown keys refused |
| Authorization | load, copy and deactivate pass the same authz `set`/`delete` pass, per leaf; `confirm` and `confirm abort` require commit authorization |
| Secrets | copied and loaded secret leaves are masked as `set` masks them (R-7) |
| Resource exhaustion | a pasted load is bounded by the existing paste limit |

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

- Junos `commit confirmed <minutes>` (default 10) is owned by `mgd`, not the CLI session: the rollback happens with the session gone, and any later `commit` confirms. VyOS `commit-confirm <minutes>` schedules the revert as a system job. Both survive the session; Ze's file-mode tick does not. Reference only.
- Junos `load merge|replace|override [relative] terminal|<file>` loads into the candidate, and `show | compare` then lists the per-statement differences. That is the model AC-1 to AC-4 follow.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Copy and leaf/path deactivate/activate are each one structural op | per-leaf set entries for a copy | `change-file-structural-ops.md`: synthetic entries hide intent; one op is one pending change and conflict detection can see it. Owner decision 2026-10-10: "ok" |
| Load builds the target tree and diffs it into change entries written in one batch | replay each loaded leaf through `writeThroughSet` (one lock per leaf, no deletes for replace, partial on failure); a single "load" structural op holding the subtree (blame, compare and per-leaf conflict detection lose sight of it) | per-leaf entries are what session mode reads everywhere; one batch is atomic |
| One tree-based load path for both modes; delete the text merge | keep text merge for file mode and add a tree path for session | `ai/rules/config.md` bans text surgery; `ai/rules/no-layering.md` bans keeping both. Owner decision 2026-10-10: "one path" |
| The confirm window is owned by the daemon with a stored pending record | keep the Model tick in session mode | the tick dies with the SSH channel, which is the case the command exists for. Owner decision 2026-10-10: "correct" |
| Commits during a pending window are refused | Junos semantics: the next `commit` confirms | an implicit confirm by another operator hides the pending window; refusal is explicit. OPEN (2026-10-10): still under discussion with the owner, explicit `confirm` versus a plain `commit` from the owning session confirming; AC-18 stands until he decides |
| Seconds, 1 to 3600, kept | switch to minutes like Junos/VyOS | the unit is the existing contract; changing it is scope the owner did not ask for |

## Known Limitations
- The web terminal has no `load` or `commit confirmed` verb at all (absent, not refused); not this class. Copy, deactivate and activate reach it through the shared `Editor` and are covered (AC-11).
- File mode keeps its in-process countdown: with no daemon there is nothing else to own it. An orphaned `.live.conf` is journaled, not fixed here (`plan/journal/unwired-feature.md`).

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
- [ ] `./le verify worktree` passes
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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: no wire change)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/immediate/spec-session-editor-file-mode-parity.md` only

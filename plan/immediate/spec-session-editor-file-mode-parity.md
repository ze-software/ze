# Spec: session-editor-file-mode-parity

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | - |
| Phase | 3/6 |
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
`commit now` applies it to the running daemon. `commit confirmed` auto-reverts even
when the SSH session that issued it is gone, because that is the lockout the
command exists to survive.

Owner decision (2026-10-10): the commit grammar is replaced. Every way to commit
is a `commit` subcommand, and confirming is always an explicit act:

| Command | Outside a window | During a window, user who started it | During a window, other users |
|---------|------------------|--------------------------------------|------------------------------|
| `commit now [force]` | applies the candidate | refused, with or without `force`, pointing to `commit accept` | refused |
| `commit confirmed <seconds>` | applies with a countdown; the time is required | refused, pointing to `commit confirmed <seconds> force` to add changes and reset the countdown, `commit accept`, or `commit abort` | refused |
| `commit confirmed <seconds> force` | applies with a countdown despite warnings and conflicts | adds the new changes and resets the countdown to `<seconds>`; the revert target stays the state before the first unconfirmed commit | refused |
| `commit accept` | refused, no window | stops the countdown and keeps the applied config; uncommitted candidate edits stay pending and are not applied | refused |
| `commit abort` | refused, no window | reverts now | refused |
| `commit verify` | validates the candidate, applies nothing (Junos `commit check`) | same | same |
| `commit` alone | error naming the subcommands | same | same |

`force` is not a verb (owner amendment 2026-10-10: "we can have force as an
adapter for all command instead"). It is a trailing modifier on the two
subcommands that apply a candidate: it applies despite validation warnings and
is still refused on errors (owner: "an emergency commit if commit now failed and
we accept potential issue"). On `accept`, `abort` and `verify` it has nothing to
override and is an error. `confirm` and `confirm abort` are removed. A window
belongs to the user who started it, from any of that user's sessions (AC-17).

Owner decisions (2026-10-10, superseding the "all relevant commands" reading of
e76a9fc073): "force is only for commit". `force` is a modifier of the commit
subcommands and of nothing else: on every other command it is an ordinary word,
so `set ... description force` stores `force` with no reserved-word machinery,
and `exit force`/`quit force` do not exist. Owner answer (2026-10-10, "yes,
every editor"): the grammar and the modifier apply to the SSH session editor,
`ze config edit -f` file mode and the web terminal, and plain `commit` is
removed everywhere with no alias.

| Gate | Source | With `force` (`commit now force`, `commit confirmed <seconds> force`) |
|------|--------|------------------------------------------------------------------------|
| validation warnings block the commit | `model_commands_commit.go`, "Both errors and warnings block commit" | applies; errors still refuse (AC-12) |
| LIVE or STALE conflict with another user's pending change | `editor_commit.go` conflict detection | applies; the other user's conflicting uncommitted change is discarded, and that user's session is told it was discarded and by whom (AC-32) |
| a confirm window is pending (owner's own session) | AC-18 | `commit now force` refused, pointing to `commit accept`; `commit confirmed <seconds> force` adds the changes and resets the countdown (AC-23) |
| `commit accept`, `commit abort`, `commit verify` | no gate | error: nothing to override (AC-28) |

`copy` and `rename` never overwrite an existing destination: they are refused,
and the operator deletes the destination first (AC-30).

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
  → Constraint: every error says what failed, why, and what to do next; a refusal during a window names the window, the user who started it and the seconds left
  → Decision: the commit grammar changes by owner decision (Task table): `commit now|confirmed <seconds>|accept|abort|verify`, `force` as a trailing modifier, `confirm` removed
- [ ] `docs/architecture/cli/command-namespacing.md`, `docs/architecture/config/syntax.md`, `docs/architecture/testing/ci-format.md`, `docs/architecture/web-interface.md` - declared by files this spec changes (the editor dispatch, `config/meta.go`, the fixtures, the web terminal)
  → Constraint: each is read at implementation and updated where it names the editor's `commit`, `confirm` or copy/rename behavior; otherwise named unaffected with the reason
- [ ] `ai/rules/goroutine-lifecycle.md` - the confirm-window timer
  → Constraint: the daemon-owned deadline is one long-lived worker started with the daemon, never a goroutine per commit

**Key insights:**
- Session mode writes every edit through to a per-user change file (`ChangePath`) under the store lock: a sparse tree, a meta tree (user, origin, time, previous value) and structural ops (`config.StructuralOp*` in `internal/component/config/change_file.go`). Commit reads them, detects LIVE/STALE conflicts, and applies them.
- The refused verbs all mutate `e.tree` directly. The fix is to make each one write through, using the vocabulary that exists plus new structural ops where none fits.
- The commit-confirmed countdown lives in the TUI `Model` (`handleConfirmCountdown`, a bubbletea tick). In session mode the Model runs inside the daemon per SSH channel, so the tick dies with the channel: a dropped SSH session would never revert. The window MUST be owned by the daemon.
- `Editor.HasPendingLive` has no non-test caller: an orphaned `.live.conf` is never acted on (journal row, `plan/journal/unwired-feature.md`).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cli/model_commands.go` - dispatch; commit arm parses a leading `force` (`commit force [confirmed <N>]`), refuses `confirmed` when `HasSession()`, routes plain commit to `cmdCommitSession`; a separate `confirm` arm takes `confirm` and `confirm abort`
- [ ] `internal/component/cli/model.go` - verb words `cmdConfirm`, `cmdConfirmed`, `cmdAbort`
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
- File-mode results for `load`, `copy`, `deactivate`, `activate`: same status messages; for `commit confirmed`: same countdown, same boundary errors (1 to 3600 seconds), same history requirement (`errCommitConfirmedNeedsHistory`)
- The grammar of `load`, `copy`, `deactivate`, `activate`, `rollback <N>`
- Session commit semantics: conflict detection, `CommitSessionCandidate` + `NotifyReload` transactional path, `ClearCandidate` on reload failure
- Offline `ze config deactivate|activate` and their stdin pipeline form
- Other users' pending change files are never touched by load, copy or the confirm revert

**Behavior to change:**
- The six refusals are removed and each verb writes through in session mode
- File-mode load moves from text surgery to the tree path (same observable result, see A-2)
- `commit confirmed` in session mode is owned by the daemon
- The commit grammar becomes the Task table: `commit now`, `commit confirmed <seconds>`, `commit accept`, `commit abort`, `commit verify`, `force` as a trailing modifier; plain `commit`, `commit force`, `confirm` and `confirm abort` are removed (AC-12, AC-24 to AC-29)
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
8. Commit (`commit now`, `commit confirmed`, each with or without `force`): `CommitSessionCandidate` then `NotifyReload` to the running daemon; `commit verify` runs the same validation and stops before `CommitSessionCandidate`
9. Confirmed: the daemon records a pending-confirm record (deadline, rollback revision, user who started it) in the store and arms its one deadline worker. During the window, from any session of that user: `commit accept` clears the record and leaves pending candidate edits pending; a nested `commit confirmed <seconds>` applies more changes and restarts the countdown while keeping the first revert target; `commit abort` reverts now; `commit now` is refused. Every applying, accepting or aborting subcommand from another user is refused; `commit verify` is allowed to all. Abort or deadline restages the rollback revision as a candidate and reloads

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
- `CommitSessionCandidate` - shared by `commit now` and `commit confirmed`, with or without `force`
- `storage/pointer.go` - home of the pending-confirm record beside `candidate` and `rollback`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to fill at implementation: every verb reaches the change file, never `e.tree` alone |
| No unintended coupling (components stay isolated) | No | the confirm worker lives with the daemon's config ownership, the CLI Model only asks it |
| No duplicated functionality (extends existing, does not recreate) | No | one load path for both modes; write-through helpers reused |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A for wire; trees are cloned once per load |
| Registration over hardcoding, outbound | No | the commit subcommands replace the commit and confirm arms of the existing dispatch, with completion derived from the same word list; new structural ops join the existing token set |
| Registration over hardcoding, inbound | No | lists to search at implementation: `StructuralOpType` constants, `ChangeFile*Token`, `PendingChangeKind`, `applyStructuralOps` switch, `filterStructuralOps`, the change-file parser |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The set-format change file can carry inactive markers for leaves and paths | `serialize_set.go` and `parser_list.go` handle `IsInactive`/`IsLeafInactive`; `PendingChangeDeactivate` exists | new tokens need parser work beyond the op list | round-trip unit test of the new ops through `SerializeChangeFile`/`ParseChangeFile` | unvalidated |
| A-2 | Tree-level merge gives the same result file-mode text merge gave for every existing load test | `mergeConfigs` merges top-level keys; tree merge is a superset | a file-mode load test changes result | run every existing `test/editor/**/load*.et` and `model_load` unit test unchanged | unvalidated |
| A-3 | A loaded subtree of a few thousand leaves writes through in one lock hold within the editor's command latency | write-through is one serialize + one write | large paste stalls the session | unit test with a 5000-leaf load, timed | confirmed 2026-10-10 after a fix: the first form re-read and re-parsed the change file and the committed config per leaf (1000 peers ~49s); the batched stage reads each once and writes once, `TestSessionLoadLargeInputBatched` counts it (1250 peers, 5000 leaves: 42ms) |
| A-4 | The daemon can host one confirm worker per config name, started with the daemon | the daemon owns the store and the reload: `runYANGConfig` (`cmd/ze/hub/main.go`) builds `reloadAfterCommit` and publishes it to SSH session editors through `sessionReloadHolder`; `newSessionEditor` (`cmd/ze/hub/session_editor.go`) and the attached console take it; boot clears a stale candidate in `clearStaleCandidateOnBoot` before `runYANGConfig` | needs a new component home | read 2026-10-10 (Phase 1): home chosen, see Key Design Decisions "Confirm worker home" | confirmed |
| A-5 | A reloaded rollback revision restores the running daemon's behavior (not only the file) | transactional commit path promotes a candidate and reloads | revert changes the file but not the daemon | `.ci` asserts daemon state, not only the file | unvalidated |
| A-6 | `rollback <N>` in session mode writes the config file directly without the candidate/reload path | `Editor.Rollback` writes `originalPath`; `cmdRollback` does not reload | the confirm revert cannot reuse `Editor.Rollback` | read and test at Phase 4; the revert uses the candidate path regardless | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | SSH drops during a confirm window and nothing reverts (today's file-mode behavior, carried over) | disconnect `.ci` keeps the trial config | the window is daemon-owned (AC-15) |
| R-2 | A second commit lands inside a window and the revert erases it | another user commits during the window | `commit now` from anyone and every applying subcommand from another user are refused (AC-18); a nested `commit confirmed` from the user who started the window is itself reverted with the first, so no unconfirmed change survives (AC-23) |
| R-10 | The grammar change leaves a caller, test, completion or page on plain `commit` or `confirm` | old word still accepted or still documented | `commit` alone and `confirm` are errors (AC-27); the sweep greps tests, completion and docs for the old forms (AC-22, AC-29) |
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
| Operator-visible break | plain `commit`, `commit force`, `confirm`, `confirm abort` stop working in every editor; each refusal names the new form (AC-27) |
| Who else touches this path? | `plan/spec-terminal-demo-showcase.md` depends on it (its old AC-13/AC-14); the web editor; any spec touching `editor_draft.go` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SSH `load file absolute merge <path>` | → | `cmdLoadNew` session branch, batched write-through | `test/plugin/session-editor-load-merge.ci` |
| SSH `load terminal relative merge` + Ctrl-D | → | paste-mode end, same path | `test/plugin/session-editor-load-terminal-relative.ci` |
| SSH `load file absolute replace <path>` | → | replace diff to delete entries | `test/plugin/session-editor-load-replace.ci` |
| SSH `copy` | → | `CopyListEntry` write-through | `test/plugin/session-editor-copy.ci` |
| SSH `deactivate`/`activate` leaf and path | → | new structural ops | `test/plugin/session-editor-deactivate-activate.ci` |
| SSH `commit now force` | → | session commit, warnings skipped | `test/plugin/session-editor-commit-force.ci` |
| SSH `commit verify` | → | session validation, nothing applied | `test/plugin/session-editor-commit-verify.ci` |
| SSH `commit confirmed <s>` + `commit accept` | → | daemon confirm record | `test/plugin/session-editor-commit-confirmed-accept.ci` |
| SSH `commit confirmed <s>`, no accept | → | daemon deadline worker | `test/plugin/session-editor-commit-confirmed-timeout.ci` |
| SSH `commit confirmed <s>`, client killed | → | daemon deadline worker | `test/plugin/session-editor-commit-confirmed-disconnect.ci` |
| Web terminal `copy`/`deactivate`/`activate` | → | same `Editor` methods | `test/web/cli-session-copy-deactivate.wb` |
| Web terminal `commit now`, `commit confirmed`, `commit accept`, `force` | → | the same commit dispatch | `test/web/cli-commit-grammar.wb` |
| File mode `ze config edit -f` commit subcommands and `force` | → | the same commit dispatch | `test/editor/lifecycle/commit-grammar.et` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | SSH editor on a running daemon; `load file absolute merge <path>` with a file adding two leaves and changing one | `show \| changes` lists exactly three set entries attributed to the session user; `show \| compare` lists exactly those three leaves; leaves equal to the current value produce no entry; `commit now` applies them and the running daemon reports the new values |
| AC-2 | SSH editor at context `bgp peer peer1`; `load terminal relative merge`, paste, Ctrl-D | entries are rooted under `bgp peer peer1`; nothing outside it changes; `commit now` applies |
| AC-3 | SSH editor; `load file absolute replace <path>` where the file omits a leaf and a list entry the config holds | `show \| changes` lists a delete for the leaf and a delete-entry for the list entry plus sets for the differences; after `commit now` the running config equals the file |
| AC-4 | SSH editor at a context path; `load ... relative replace` | the replace is scoped to the context subtree; siblings outside it are untouched |
| AC-5 | SSH editor; load input with a syntax error or an unknown key | refused with the parser's error naming the line and the closest valid key; the change file, draft and in-memory tree are unchanged |
| AC-6 | Two SSH sessions; session B has a pending set on a leaf; session A loads a value for the same leaf and commits | session A's commit reports the LIVE conflict exactly as it does for `set` |
| AC-7 | File mode (`ze config edit -f`) `load` merge and replace, absolute and relative | results equal today's for every existing file-mode load test; the text merge functions no longer exist |
| AC-8 | SSH editor; `copy <list> <src> to <dst>` | `show \| changes` lists ONE copy change attributed to the user, and `show \| compare` shows the new entry; `commit now` applies and the running daemon holds both entries; a destination that exists is refused (AC-30) |
| AC-9 | SSH editor; `deactivate` on a leaf and on a path | `show \| changes` lists each as a deactivate; `commit now` applies; the running daemon treats them as absent; `show` marks them inactive |
| AC-10 | SSH editor; `activate` on a leaf and a path that are inactive in the committed config | `show \| changes` lists each as an activate; after `commit now` the daemon uses them again; activating an active node gives the existing `ErrLeafNotInactive`/`ErrPathNotInactive` message |
| AC-11 | Web terminal (session editor) `copy`, `deactivate`, `activate` | each succeeds and shows in pending changes; `commit now` applies |
| AC-12 | SSH editor with a change that raises only warnings: `commit now`, then `commit now force`; in a second run `commit confirmed 60 force` | `commit now` is refused, listing the warnings and naming `commit now force`. `commit now force` applies to the running daemon and the status says how many warnings it skipped. `commit confirmed 60 force` applies the same way and opens a window. With an error in the candidate, both forms with `force` are refused naming the error |
| AC-13 | SSH editor; `commit confirmed 60` then `commit accept` within 60 s | the change applies to the running daemon at once; the status says to run `commit accept` within 60 s; after `commit accept` it stays applied, and no revert happens after 60 s |
| AC-14 | SSH editor; `commit confirmed 5`, no `commit accept` | after 5 s the daemon restores the previous revision and the running daemon reports the previous values; an attached session shows the file editor's timeout message |
| AC-15 | SSH editor; `commit confirmed 5`, then the SSH client is killed | the daemon still reverts after 5 s; the running daemon reports the previous values |
| AC-16 | SSH editor; `commit confirmed 60` then `commit abort` | the previous revision is applied at once; the message is the file-mode abort message |
| AC-17 | User U runs `commit confirmed 60`, U's SSH client is killed, and U opens a new SSH session within 60 s; a session of another user V is also open | U's new session behaves as a reconnection: its editor shows the pending window and the seconds left, and it may run `commit accept`, `commit abort` and `commit confirmed <seconds>` on the window as the session that started it could. V's session shows the window, and V's `commit accept`, `commit abort`, `commit now` and `commit confirmed` are refused as in AC-18 (b). The window belongs to the user, not to the SSH session (owner decision 2026-10-10: "the new session should behave like a reconnection") |
| AC-18 | During a pending window: (a) any session of the user who started the window runs `commit now` or `commit now force`; (b) a session of any other user runs `commit now`, `commit confirmed <seconds>`, `commit accept` or `commit abort`, each with and without `force` where the grammar allows it; (c) any user runs `commit verify` | (a) refused, with or without `force`, the message saying a confirmed commit is pending and to use `commit accept` to keep it, `commit abort` to revert, or `commit confirmed <seconds> force` to add changes and reset the countdown; a plain `commit confirmed <seconds>` from the same user is refused with the same message (AC-23); nothing is committed and the window and its deadline are unchanged. (b) refused, naming the pending window, the user who started it and the seconds left, and saying to wait for the deadline or have that user accept or abort. (c) runs as AC-26 (owner decision 2026-10-10) |
| AC-19 | Daemon restarted during a pending window | at start the daemon finds the pending record, restores the rollback revision before applying config, and logs that it reverted an unconfirmed commit |
| AC-20 | `commit confirmed 0`, `commit confirmed 3601`, `commit confirmed abc`, and `commit confirmed` with no time, in session mode | the file-mode errors: at least 1, at most 3600, invalid seconds, and the usage naming the required time |
| AC-21 | Session `commit confirmed` on a daemon store with no history | refused before writing, as `errCommitConfirmedNeedsHistory` |
| AC-22 | `docs/guide/config-editor.md` and every page in Files to Modify that names the editor's commit | the blocked-command table and "Use file mode for these operations" are gone; the command table lists `commit now`, `commit confirmed <seconds>`, `commit accept`, `commit abort`, `commit verify` and the `force` modifier, and no `commit` alone, `commit force`, `confirm` or `confirm abort`; the Commit Confirmed section states that the daemon owns the window, that it survives a dropped session, and that it belongs to the user |
| AC-23 | The user who ran `commit confirmed 60` makes further changes and runs `commit confirmed 30` inside the window, then `commit confirmed 30 force` | `commit confirmed 30` is refused as AC-18 (a), naming `force`, `commit accept` and `commit abort`; window and deadline unchanged. With `force`, the new changes apply to the running daemon and the countdown restarts at 30 s. `commit accept` keeps both commits. With no `commit accept`, at 30 s the revert restores the state from BEFORE the FIRST unconfirmed commit, so neither commit survives; `commit abort` does the same at once. If the new changes fail validation or conflict, the nested commit is refused as any commit is and the first window keeps its deadline (owner decision 2026-10-10) |
| AC-24 | During a window the user who started it makes further edits, does not commit them, and runs `commit accept` | the window ends and the applied config is kept; the further edits stay pending (`show \| changes` still lists them) and the running daemon does not have them |
| AC-25 | No window pending; `commit accept`, then `commit abort` | each is refused, saying no confirmed commit is pending; nothing changes |
| AC-26 | Candidate clean, then with a warning, then with an error; `commit verify` each time, also during a window and from another user | the result lists the same errors and warnings `commit now` would report, or says the candidate is valid; nothing is applied, the running daemon and the window are unchanged, and the candidate and its pending changes are unchanged |
| AC-27 | `commit` alone; `commit bogus`; `commit force`; `confirm`; `confirm abort` | `commit` alone and `commit bogus` are refused naming `now`, `confirmed <seconds>`, `accept`, `abort`, `verify` and the `force` modifier; `commit force` is refused saying `force` is a modifier that follows `commit now` or `commit confirmed <seconds>`; `confirm` and `confirm abort` are unknown commands; completion offers the five subcommands and never `confirm` |
| AC-28 | `commit accept force`, `commit abort force`, `commit verify force`; and `set <path> description force` | each of the first three is refused, saying `force` overrides validation warnings or a conflict and this subcommand has neither; nothing runs. `set ... description force` stores the value `force`: outside `commit`, `force` is an ordinary word (owner 2026-10-10: "force is only for commit") |
| AC-29 | File mode (`ze config edit -f <file>`) and the web terminal each run AC-12, AC-13, AC-16, AC-20, AC-25 to AC-28 and AC-30 | each gives the result and message the SSH editor gives; plain `commit`, `commit force`, `confirm` and `confirm abort` are refused there as in AC-27; file mode keeps its in-process countdown (Known Limitations) (owner decision 2026-10-10: "yes, every editor") |
| AC-30 | `copy <list> <src> to <dst>` and `rename <list> <old> to <new>` where the destination exists, in session and file mode | refused, the message naming the existing destination and saying to delete it first; nothing changes. No `force` and no other keyword overwrites it (owner 2026-10-10) |
| AC-31 | Withdrawn (owner 2026-10-10, "force is only for commit"): `exit force` and `quit force` do not exist; `exit` with pending changes prompts as today | - |
| AC-32 | Candidate with a LIVE conflict, then one with a STALE conflict, against user V's pending change; user U runs `commit now force` | `commit now` alone is refused naming the conflict, as today. `commit now force` applies U's change to the running daemon; V's conflicting uncommitted change is discarded from V's change file and no other change of V's is touched; V's open session is told, on its next command or refresh, that its change at `<path>` was discarded by U's forced commit (the `checkDraftChanged` notification path); `show \| changes` in V's session no longer lists it (owner decision 2026-10-10: force overrides conflicts) |

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
| `TestSessionLoadLargeInputBatched` | `internal/component/cli/editor_load_test.go` | A-3 | |
| `TestFileModeLoadTreeMergeMatchesPrevious` | `internal/component/cli/model_load_test.go` | AC-7, A-2 | |
| `TestSessionCopyWritesThrough` | `internal/component/cli/editor_draft_test.go` | AC-8, R-7 | |
| `TestSessionDeactivateActivateLeafAndPath` | same | AC-9, AC-10 | |
| `TestChangeFileDeactivateOpsRoundTrip` | `internal/component/config/change_file_test.go` | A-1 | |
| `TestCommitForceModifier` | `internal/component/cli/model_commands_commit_test.go` | AC-12: `commit now force` and `commit confirmed <s> force` skip warnings and refuse errors, in session and file mode | |
| `TestCommitForceOverridesConflict` | `internal/component/cli/editor_commit_test.go` | AC-32: a forced commit applies over a LIVE and a STALE conflict, removes only the other user's conflicting entry from that user's change file, and that user's editor reports the discard and who did it | |
| `TestCommitGrammar` | `internal/component/cli/model_commands_test.go` | AC-20, AC-27, AC-28: every subcommand parses; `commit` alone, unknown subcommand, `commit force`, missing time; `force` refused on accept/abort/verify; `set ... description force` stores `force`; `confirm` unknown | |
| `TestCommitVerifyAppliesNothing` | `internal/component/cli/model_commands_commit_test.go` | AC-26 | |
| `TestCopyRenameRefuseExistingDestination` | `internal/component/cli/editor_draft_test.go` | AC-30, session and file mode | |
| `TestConfirmWindowWorkerRevertsAtDeadline` | owning package of the worker | AC-14, AC-15 | |
| `TestConfirmWindowOwnerIsTheUser` | same | AC-17: a new session of the user who started the window is its owner; a session of another user is not | |
| `TestConfirmWindowRefusesOtherUsers` | same | AC-17, AC-18 (b), (c) | |
| `TestConfirmWindowCommitNowRefused` | same | AC-18 (a): `commit now` refused, window and deadline unchanged | |
| `TestConfirmWindowNestedRevertsToFirst` | same | AC-23: nested `commit confirmed` restarts the countdown; revert and abort restore the state before the first unconfirmed commit; a failing nested commit leaves the first deadline | |
| `TestConfirmWindowAcceptKeepsCandidateEdits` | same | AC-24 | |
| `TestConfirmWindowAcceptAbortWithoutWindow` | same | AC-25 | |
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
| `session-editor-commit-force-conflict` | `test/plugin/session-editor-commit-force-conflict.ci` | AC-32 | |
| `session-editor-commit-verify` | `test/plugin/session-editor-commit-verify.ci` | AC-26 | |
| `session-editor-commit-grammar` | `test/plugin/session-editor-commit-grammar.ci` | AC-25, AC-27, AC-28 | |
| `session-editor-copy-rename-existing` | `test/plugin/session-editor-copy-rename-existing.ci` | AC-30 | |
| `session-editor-commit-confirmed-accept` | `test/plugin/session-editor-commit-confirmed-accept.ci` | AC-13 | |
| `session-editor-commit-confirmed-accept-keeps-candidate` | `test/plugin/session-editor-commit-confirmed-accept-keeps-candidate.ci` | AC-24 | |
| `session-editor-commit-confirmed-timeout` | `test/plugin/session-editor-commit-confirmed-timeout.ci` | AC-14 | |
| `session-editor-commit-confirmed-disconnect` | `test/plugin/session-editor-commit-confirmed-disconnect.ci` | AC-15 | |
| `session-editor-commit-confirmed-abort` | `test/plugin/session-editor-commit-confirmed-abort.ci` | AC-16 | |
| `session-editor-commit-confirmed-commit-now-refused` | `test/plugin/session-editor-commit-confirmed-commit-now-refused.ci` | AC-18 (a): `commit now` and `commit now force` refused naming `commit accept`, `commit abort`, `commit confirmed <seconds> force`; the window still reverts at its deadline | |
| `session-editor-commit-confirmed-other-user` | `test/plugin/session-editor-commit-confirmed-other-user.ci` | AC-18 (b), (c): user V's applying, accepting and aborting subcommands refused; V's `commit verify` runs | |
| `session-editor-commit-confirmed-nested` | `test/plugin/session-editor-commit-confirmed-nested.ci` | AC-23: nested `commit confirmed` without `force` refused; with `force` applies, restarts the countdown, and on timeout reverts to before the first commit; a second run accepts and keeps both | |
| `session-editor-commit-confirmed-reconnect` | `test/plugin/session-editor-commit-confirmed-reconnect.ci` | AC-17: user U starts a window and the client is killed; U's new SSH session shows the window and seconds left and is accepted as its owner; a session of user V is refused; the running daemon reports the state the owner's action produces | |
| `session-editor-commit-confirmed-restart` | `test/plugin/session-editor-commit-confirmed-restart.ci` | AC-19 | PASS 2026-10-10 (5.1s); red under mutation `recoverConfirmWindow` skipping recovery: daemon B booted 5.6.7.8, step 2 "show bgp never held 1.2.3.4". It needed SSH `stop` to end the daemon, fixed in 012d69b946 (the daemon stayed in waitLoop with or without a window) |
| `session-editor-commit-confirmed-boundary` | `test/plugin/session-editor-commit-confirmed-boundary.ci` | AC-20 (AC-21 cannot be driven over SSH, see below) | PASS 2026-10-10; red under mutation `CommitConfirmedSecondsMax = 3601` at step 9, `commit confirmed 3601` opened a window instead of "at most 3600 seconds" |
| replaces `load-blocked.et` | `test/editor/session/load-merge.et` | AC-1 at model level | |
| `commit-grammar` (file mode) | `test/editor/lifecycle/commit-grammar.et` | AC-29 for file mode: AC-12, AC-13, AC-16, AC-20, AC-25 to AC-28, AC-30 | |
| `cli-commit-grammar` (web terminal) | `test/web/cli-commit-grammar.wb` | AC-29 for the web terminal | |
| existing editor tests | every file in Files to Modify, "Surfaces that type the old grammar" | AC-29: moved to the new grammar with the same assertions; `confirm-without-pending.et` and `abort-without-pending.et` become AC-25's `commit accept` and `commit abort` cases | |

Each `.ci` drives `ze config edit` over SSH against the running daemon through a
fixture modelled on `driveEditor04` (`internal/test/fixture/plugin_fixture_04_cli.go`),
and asserts the running daemon's state (a `show` through `ze cli -c`), not only
the stored file (A-5). Each is run red against the unfixed tree first.

Red record, 2026-10-10 (HEAD clone with `Editor.daemonWindow` mutated to return
nil, so no session reaches the daemon window): 9/9 confirm `.ci` FAIL, each at
the wait for the window's `commit accept` prompt after `commit confirmed`
(output deadline expired): accept, abort, commit-now-refused, disconnect,
nested and timeout at step 5; accept-keeps-candidate, other-user and reconnect
at step 3. Unmutated tree: 9/9 PASS (`./le --name sccw test bgp plugin -pattern
session-editor-commit-confirmed`).

AC-21 has no `.ci`, because no daemon session can lack history. Evidence:
every SSH session editor comes from `newSessionEditor`
(`cmd/ze/hub/session_editor.go`), which calls `cli.NewEditorWithStorage`, and
that refuses a nil store (`internal/component/cli/editor.go`, "config editor:
no configuration store; run ze init"); `Editor.HasHistory` is `e.store != nil`;
every store is the zefs store, whose `WriteVersion` records dated history
(`docs/architecture/storage-backends.md`, "Content-addressed history"); and a
daemon started on a plain file creates its live store first ("created live
store" in every confirm `.ci` log). A store-less session editor cannot even
record a `set`: its draft lock lives in the store (`Editor.draftLock` panics on
nil). The guard in `cmdCommitConfirmedWindow` is proven at the Model by
`TestSessionCommitConfirmedNeedsHistory` (`model_commit_window_test.go`): red
under the mutation `if !m.editor.HasHistory()` -> `if false` (nil dereference
in the window's snapshot of a nil store), green restored.

### Interop Tests (Scope: protocol)
N-A: no wire-visible change; the editor applies config through the existing reload.

## Files to Modify
- `internal/component/cli/model_load.go` - session load path, tree-based load for both modes, delete `mergeConfigs`/`mergeAtContext`/`replaceAtContext`; `cmdCommitConfirmed` session branch asks the daemon window
- `internal/component/cli/model_keys.go` - paste-mode end uses the same load path
- `internal/component/cli/model_commands.go`, `internal/component/cli/model.go` - commit subcommands replace the commit and `confirm` arms; remove the confirmed refusal; `force` read as a trailing modifier of the commit subcommands only
- `internal/component/cli/model_commands_commit.go` - `commit now`, `commit verify`, the `force` modifier in both modes; refusals while a window is pending
- `internal/component/cli/editor_commit.go` - a forced commit discards the other user's conflicting entries from that user's change file (AC-32)
- `internal/component/cli/editor_draft.go` (`checkDraftChanged`) - the overridden user's session reports the discard and who did it (AC-32)
- `internal/component/cli/model_keys.go` - the pending-changes prompt names `commit now`
- `internal/component/cli/completer.go` - completes the subcommands and `force` after `commit now` and `commit confirmed <seconds>`
- `internal/component/config/tree.go`, `internal/component/config/meta.go` - copy and rename refusal of an existing destination says to delete it first (AC-30)
- `internal/component/command/verbs.go` - the `commit` verb word, if it carries the editor grammar (confirm at implementation; the BGP `commit` plugin and `./le commit` are other commands)
- `internal/component/web/handler.go`, `internal/component/web/cli.go`, `internal/component/web/cli_terminal.go` - web terminal grammar; the web commit button runs `commit now`
- `internal/component/config/cli/cmd_edit.go` - file-mode help or messages naming `commit`
- `internal/test/cli/cmd_editor.go`, `internal/test/fixture/constants.go`, `internal/test/fixture/plugin_fixture_04_cli.go` - fixtures that type `commit`
- `internal/le/site/terminaldemo/validate_runtime.go` - demo validation that names `commit`

Surfaces that type the old grammar (from `grep -rlE 'text=(commit|confirm)\b|Type "(commit|confirm)\b' test demos` and a grep of Go tests and `docs/` for the editor's `commit`, 2026-10-10; rerun the grep at implementation and move every hit, no alias):

| Surface | Files |
|---------|-------|
| `test/editor/lifecycle/` | `abort-without-pending.et`, `commit-blocked-errors.et`, `commit-blocked-missing-leak-filter.et`, `commit-confirm-abort.et`, `commit-confirm-boundary-high.et`, `commit-confirm-boundary-low.et`, `commit-confirm-boundary-valid-high.et`, `commit-confirm-boundary-valid-low.et`, `commit-confirm-missing-arg.et`, `commit-confirm-success.et`, `commit-confirm-timeout.et`, `commit-creates-backup.et`, `commit-reload-fail.et`, `commit-reload-standalone.et`, `commit-reload-success.et`, `commit-set-format.et`, `commit-valid.et`, `commit-zefs-blob.et`, `confirm-without-pending.et`, `exit-after-commit.et`, `history-dedup.et`, `history-list.et`, `rollback-restore.et` |
| `test/editor/session/` | `commit-delete-container.et`, `commit-delete-peer.et`, `commit-ssh.et`, `commit.et`, `conflict-live.et`, `conflict-stale.et`, `edit-backup.et`, `leaflist-add-member.et`, `leaflist-commit-reload.et`, `leaflist-delete-member.et`, `leaflist-insert-deactivate.et`, `leaflist-set-commit.et`, `set-format-migration.et` |
| `test/editor/workflow/` | `workflow-add-peer.et`, `workflow-delete-leaf.et`, `workflow-delete-peer.et`, `workflow-multiple-changes.et`, `workflow-peer-lifecycle.et`, `workflow-set-container-leaf.et`, `workflow-update-peer.et` |
| `test/editor/pipe/`, `test/editor/completion/` | `show-compare-rollback.et`; `root-commands.et` and any completion test listing `confirm` |
| `test/web/` (typed terminal commands only; the commit button is not typed) | `cli-set-commit.wb`, `scenario-interface-cli.wb`, and each of `commit-flow.wb`, `commit-smoke.wb`, `commit-empty.wb`, `scenario-router-setup.wb`, `scenario-interface-setup.wb`, `workbench-bgp-change-verify.wb`, `oob-order.wb`, `list-add-entry-submit.wb` that types `commit` |
| demo tapes | `demos/terminal/commit-confirmed/demo.tape`, `demos/terminal/zefs-config/demo.tape` |
| Go unit tests | `internal/component/cli/model_commands_test.go`, `model_test.go`, `model_dispatch_race_test.go`, `history_test.go`, `model_commands_edit_secret_test.go`, `model_mode_test.go`, `model_load_test.go`; `internal/component/web/cli_test.go`, `handler_test.go`, `golden_test.go`, `handler_tools_test.go`; `internal/component/command/verbs_test.go` |
| docs | `docs/guide/config-editor.md`, `docs/guide/cli.md`, `docs/guide/command-reference.md`, `docs/guide/command-catalogue.md`, `docs/guide/configuration.md`, `docs/guide/config-reload.md`, `docs/guide/config-archive.md`, `docs/guide/web-interface.md`, `docs/guide/authorization.md`, `docs/guide/authentication.md`, `docs/guide/bfd.md`, `docs/guide/bgp-role.md`, `docs/guide/ddos-mitigation.md`, `docs/guide/rsvp-te.md`, `docs/guide/irr-filtering.md`, `docs/guide/vrrp.md`, `docs/features/cli-commands.md`, `docs/contributing/terminal-demos.md`, `docs/architecture/cli/command-verbs.md`, `docs/architecture/config/transaction-protocol.md`, `docs/architecture/config/apply-ordering.md`, `docs/architecture/config/yang-config-design.md`, `docs/architecture/resolve.md`, `docs/architecture/firewall/firewall-irr.md`, `docs/architecture/diagnostics/debug-filtering.md`, `docs/architecture/testing/runner-architecture.md`, `docs/comparison.md`; `ai/digests/cli-editor.md`, `ai/INDEX.md`, `ai/patterns/web-endpoint.md`. A hit that names another `commit` (BGP transaction, `./le commit`) is left alone; `docs/architecture/config/vyos-research.md` quotes VyOS and is left alone |
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
- `internal/component/config/confirm/` - the confirm-window worker and its test (A-4)
- `internal/test/fixture/plugin_fixture_NN_session_editor.go` - SSH editor driver with one mode per `.ci`
- the 23 `.ci` files, two `.wb` files and `test/editor/lifecycle/commit-grammar.et` named in the Functional Tests table
- `test/editor/session/load-merge.et`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new RPC or leaf |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | Yes | commit subcommands and the `force` modifier replace `commit`, `commit force`, `confirm`, `confirm abort` (Task) |
| CLI grammar (keyword before value) | Yes | `commit confirmed <seconds> [force]`: the keyword before its value, the modifier last |
| Editor autocomplete | Yes | `completer.go`: stop hiding the six in session mode if it filters them; offer the subcommands and `force` where the verb declares a gate; never `confirm` |
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
| 3 | CLI command added/changed? | Yes | every page in Files to Modify, "Surfaces that type the old grammar", docs row: the commit subcommands and `force` |
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
4. **Phase: Commit grammar and `force`** -- subcommands in every editor, `force` on the commit subcommands only (warnings and conflict override, AC-32 discard notice), copy/rename existing-destination refusal, old forms removed with every test, tape, fixture and page in the surfaces table moved in the same phase
   - Tests: AC-12, AC-20, AC-25 to AC-30, AC-32
5. **Phase: Daemon confirm window** -- pending record, worker, refusals while pending, accept, abort, nested, revert on start, session Model shows remaining time
   - Tests: AC-13 to AC-19, AC-21, AC-23, AC-24
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
| functional tests | the 23 `.ci`, 2 `.wb` and the new `.et` files pass, each observed red first |
| old grammar gone | the Files to Modify grep returns no editor `commit` alone, `commit force`, `confirm` or `confirm abort` in tests, tapes, fixtures or docs |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | loaded content goes through the schema parser; unknown keys refused |
| Authorization | load, copy and deactivate pass the same authz `set`/`delete` pass, per leaf; every `commit` subcommand requires commit authorization; window ownership is checked against the authenticated user, never a name the client supplies |
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
- Junos `commit check` validates without applying; `commit verify` is its counterpart.
- Junos `load merge|replace|override [relative] terminal|<file>` loads into the candidate, and `show | compare` then lists the per-statement differences. That is the model AC-1 to AC-4 follow.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Copy and leaf/path deactivate/activate are each one structural op | per-leaf set entries for a copy | `change-file-structural-ops.md`: synthetic entries hide intent; one op is one pending change and conflict detection can see it. Owner decision 2026-10-10: "ok" |
| Load builds the target tree and diffs it into change entries written in one batch | replay each loaded leaf through `writeThroughSet` (one lock per leaf, no deletes for replace, partial on failure); a single "load" structural op holding the subtree (blame, compare and per-leaf conflict detection lose sight of it) | per-leaf entries are what session mode reads everywhere; one batch is atomic |
| One tree-based load path for both modes; delete the text merge | keep text merge for file mode and add a tree path for session | `ai/rules/config.md` bans text surgery; `ai/rules/no-layering.md` bans keeping both. Owner decision 2026-10-10: "one path" |
| The confirm window is owned by the daemon with a stored pending record | keep the Model tick in session mode | the tick dies with the SSH channel, which is the case the command exists for. Owner decision 2026-10-10: "correct" |
| Commit grammar: `commit now`, `commit confirmed <seconds>`, `commit accept`, `commit abort`, `commit verify`; `commit` alone is an error; no `confirm` verb | Junos: a plain `commit` confirms (option A, withdrawn); `confirm` / `confirm abort` (withdrawn: "confirm alone should not be an option") | Owner decisions 2026-10-10. Confirming is always an explicit act |
| `force` is a trailing modifier of `commit now` and `commit confirmed <seconds>` only: past warnings and conflicts, never past an error; an error on `accept`, `abort`, `verify`; an ordinary word on every other command | a `commit force` verb (withdrawn); `force` on every relevant command, with copy/rename overwrite and `exit force` (withdrawn: "force is only for commit") | Owner decisions 2026-10-10 |
| `force` overrides a LIVE or STALE conflict: the other user's conflicting uncommitted change is discarded, and that user's session is told by whom | refuse as without `force` (e76a9fc073, withdrawn) | Owner decision 2026-10-10; the discard is made visible to the user who loses the change, so nothing disappears in silence |
| `copy` and `rename` never overwrite an existing destination; the operator deletes it first | a `force` that replaces the destination (withdrawn) | Owner decision 2026-10-10. If an overwriting form is ever added, its keyword is `overwrite`, not `force` (owner: "if anything the keyword should be overwrite"); none is added now |
| During a window, a plain `commit confirmed <seconds>` is refused and `commit confirmed <seconds> force` adds changes and resets the countdown; `commit now force` is refused | a plain nested `commit confirmed` that adds changes (e76a9fc073, withdrawn) | Owner decision 2026-10-10: resetting a revert countdown is a deliberate act |
| Every editor (SSH, file mode, web terminal) takes the grammar; plain `commit` removed everywhere with no alias | session mode only; an alias for `commit` | Owner answer 2026-10-10: "yes, every editor"; `ai/rules/no-layering.md` |
| A nested `commit confirmed` reverts to the state before the FIRST unconfirmed commit | revert only the latest commit to the state the first commit produced | nothing unconfirmed survives a revert; matches the owner's "revert target stays the state before the first unconfirmed commit" |
| A window belongs to the user who started it: any new SSH session of that user is its owner, as a reconnection; sessions of other users are refused (AC-17) | only the SSH session that ran `commit confirmed` owns it, so a dropped client leaves the operator waiting for the deadline | Owner decision 2026-10-10: "the new session should behave like a reconnection" |
| Confirm worker home (A-4): package `internal/component/config/confirm`, one `Window` per daemon and config, one long-lived goroutine reading a request channel and owning the deadline timer; built in `runYANGConfig` right after `reloadAfterCommit` (the reload a revert calls) and published to session editors beside `sessionReloadHolder`; stopped before `commitReloads` closes. Editors see it through an interface in `internal/component/cli/contract`, so `cli` and `web` never import the worker. The pending record is a store key written by `storage` beside the `pointer.go` keys. Boot recovery (AC-19) runs beside `clearStaleCandidateOnBoot`, BEFORE `ReadConfigSource`, so the reverted config is the one that boots | the CLI `Model` tick (dies with the SSH channel); `cmd/ze/hub` itself (no unit test without a daemon, and `web` could not reach it); `config/transaction` (the plugin verify/apply protocol, not an operator window) | engineering choice inside the owner's "daemon owns the window" decision: the hub alone holds the store, the config path and the reload together, and a component package keeps the worker unit-testable with a fake reload (recorded 2026-10-10) |
| Seconds, 1 to 3600, kept | switch to minutes like Junos/VyOS | the unit is the existing contract; changing it is scope the owner did not ask for |

## Known Limitations
- The web terminal has no `load` verb at all (absent, not refused); not this class. Copy, deactivate and activate reach it through the shared `Editor` and are covered (AC-11). It gains the commit subcommands, `commit confirmed` included, and the `force` modifier (AC-29).
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

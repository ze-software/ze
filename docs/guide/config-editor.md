# Configuration Editor

Ze includes an interactive configuration editor with YANG-driven tab completion, rollback history, and live validation.
<!-- source: internal/component/config/cli/cmd_edit.go -- cmdEditWithStorage; internal/component/cli/ -- editor model -->

## Usage

```bash
ze config edit                      # Edit default config
ze config edit myconfig.conf        # Edit specific file
```

Stored editing runs inside the owning daemon over an SSH terminal session. When
no daemon answers, the editor starts an ephemeral daemon and connects to it.
Once connected, the local process holds no writable store handle. Terminal
resizing and input travel over the SSH PTY, and the daemon persists drafts and
command history.
<!-- source: internal/component/cli/sshclient/terminal.go -- RunInteractive -->

Three more surfaces reach the same editor against a running daemon. An SSH
session opens in configuration mode. The web interface gives each authenticated
user an editor of their own. The console of `ze start --cli` opens at the
operational prompt, where `configure` enters configuration mode. A `commit now`
from any of the three reloads the daemon.
<!-- source: cmd/ze/hub/session_editor.go -- newSessionEditor, attachedConsoleEditor -->
<!-- source: internal/component/cli/model_keys.go -- handleEnter, the configure arm -->

## Editor Commands

| Command | Description |
|---------|-------------|
| `set <path> <value>` | Set a configuration value |
| `delete <path>` | Delete a configuration value or section |
| `show` | Display current configuration |
| `show <path>` | Display a specific section |
| `show \| blame` | Annotate with authorship |
| `show \| changes [all]` | Pending changes (session or all) |
| `show \| compare` | Diff against committed config; shows only the parts that differ |
| `show \| errors` | Validation issues |
| `show \| history` | List rollback revisions |
| `commit now` | Save changes and notify daemon |
| `commit now force` | Commit even though validation reports warnings (never errors) |
| `commit confirmed <N>` | Commit with N-second auto-revert window (1-3600) |
| `commit accept` | Make a pending confirmed commit permanent |
| `commit abort` | Roll back a pending confirmed commit immediately |
| `commit verify` | Validate the candidate and apply nothing |
| `rollback <N>` | Restore revision N |
| `top` | Navigate to config root |
| `up` | Navigate up one level |
| `edit <path>` | Navigate into a section |
| `exit` | Exit editor |
<!-- source: internal/component/cli/editor_commands.go -- editor commands (set, delete, show, diff, commit, rollback) -->

`commit` always takes a subcommand: a bare `commit`, a bare `force`, or an
unknown word is refused with the list of subcommands, so a typo never applies a
configuration. `force` is a modifier that only follows `commit now` or
`commit confirmed <seconds>`. While a confirmed commit is pending, `commit now`
and a second `commit confirmed` are refused: answer the window first with
`commit accept` or `commit abort`. The SSH editor, `ze config edit` and the web
terminal share this grammar.
With nothing pending, the SSH editor and the web terminal answer `commit now`
with "no changes to commit" and apply nothing, and `commit confirmed <seconds>`
with "no changes to commit: no confirmed commit was opened", because a window
over no change would revert nothing.
<!-- source: internal/component/cli/contract/commit.go -- NothingToCommit -->
<!-- source: internal/component/cli/contract/commit.go -- ParseCommit, commitSubcommands -->

A commit that validation refuses says so on the status line: how many errors
and warnings block it, then each one. When only warnings block it, the line
names the forced form of what you typed (`commit now force`, or
`commit confirmed <seconds> force`), and a forced commit reports how many
warnings it skipped. `force` never commits over an error. `commit verify`
gives the same list and applies nothing. The configuration stays on screen with
each issue marked. The web terminal and the web CLI bar run the same validation
and answer with the same text, the CLI bar as an error notification, and a
refused commit leaves your changes pending there too.
<!-- source: internal/component/cli/model_commands_commit.go -- CommitRefusal, commitValidationRefusal, WithSkippedWarnings, cmdCommitVerify -->
<!-- source: internal/component/cli/model_commit_window.go -- ForcedCommand -->
<!-- source: internal/component/web/editor_commit_window.go -- runCommit, validateTransition -->
<!-- source: internal/component/web/cli.go -- handleCLICommit -->

In the SSH editor and the web terminal, `commit now` is refused when your
change conflicts with another user's pending change (LIVE) or with a value
committed since you made it (STALE). `commit now force` applies your change
anyway. Each other user's pending change that it overrides is removed from that
user's change file, and nothing else of theirs is touched. Their SSH editor
reports "Your change at <path> was discarded by <you>'s forced commit" once, at
its next check for changes by other sessions (every two seconds, or when they
next connect); from then on it shows the committed value in place of theirs,
and `show | changes` there no longer lists it. The web editor does not show
this notice. Validation errors still block a forced commit.
<!-- source: internal/component/cli/editor_commit_force.go -- CommitSessionForce, discardOverridden, takeDiscardNotice -->
<!-- source: internal/component/cli/editor_commit.go -- reloadSessionView -->
<!-- source: internal/component/cli/model.go -- handleDraftPoll, draftPollInterval -->
<!-- source: internal/component/web/editor.go -- EditorManager.commit -->
<!-- source: internal/component/cli/model_commands_commit.go -- cmdCommitRequest -->
<!-- source: internal/component/config/confirm/confirm.go -- ErrPending -->

`commit now` reaches the same reload as `ze signal reload`. Which configuration
changes stop a session, and in what order, is in
[The order of a commit](config-reload.md#the-order-of-a-commit).

The `|` after an editor command belongs to the editor's own filter language.
It is separate from the operational command operators published by
`ze help command --json`, so names such as `blame`, `compare`, and `history`
apply here without becoming operational pipe operators.
<!-- source: internal/component/cli/model_load.go -- dispatchWithPipe, ClassifyShowPipes -->
<!-- source: internal/component/cli/completer.go -- showPipeFilters, completePipeFilter -->

### Structural operations appear in the diff

`show | changes` and `show | compare` include the draft's structural operations,
not only its leaf values. A deleted list entry, a deleted container, a deleted
list, a `rename`, an `insert` at a position, and a `deactivate` or `activate` are
recorded in the per-user change file rather than in the value tree. The tree
store exposes those change files to both display surfaces, so the reviewed diff
includes the operations that commit will apply.
<!-- source: internal/component/cli/editor.go -- listChangeFiles, readChangeFileContent -->
<!-- source: internal/component/config/change_file.go -- StructuralOp and its seven types -->

### Secrets are masked on every display path

One predicate answers whether a leaf holds a secret, and every display path reads
it. A leaf the schema marks `ze:sensitive` or `ze:bcrypt` is masked in the tree
view, the annotated view, the search results, the blame view, `ze config show`,
`ze config dump`, and `ze config diff`. `ze config diff` masks the COMPUTED diff,
so a rotated credential still reports as changed and neither value is shown. The
text output and the JSON output agree.

What a command says back is masked too. `ze config set` writes
`set <path> /* SECRET-DATA */` rather than the password the operator typed, and
so do its `--dry-run` line, the `set` status line of the SSH CLI editor, the
adoption prompt of `ze config edit`, and both sides of a commit conflict. A
refusal names the rule the value broke and not the value. The path stays in the
clear, so the operator still reads which leaf was written.
<!-- source: internal/component/config/mask.go -- LeafHoldsSecret, MaskSecrets, SecretKeys, DisplayValueAtPath, DisplayMessageAtPath -->
<!-- source: internal/component/config/cli/cmd_diff.go -- maskDiffSecrets -->
<!-- source: internal/component/config/cli/cmd_set.go -- cmdSetImpl -->

### Commands run one at a time, in order

The config commands of one session run serially, in the order the operator
entered them. Pasting a block of `set` lines followed by `commit now` over SSH lands
every `set` before the `commit` reads the draft. Nothing is dropped and no
command is refused for arriving while another is in flight. A command starts
only once the answer of the one before it is on screen, so answers appear in
the same order: `commit abort` typed straight after `commit confirmed` always
ends with the abort's answer. While a command is in flight, the status line's
two-second look at a confirmed-commit window waits, so a session's own accept
or abort is never reported as another session's. When the session ends with
commands still queued, because the SSH client went away or the operator left,
those commands are dropped unrun: a command that no operator will ever see the
answer of does not change the draft.
<!-- source: internal/component/cli/model_commands.go -- dispatchQueue -->
<!-- source: internal/component/cli/model.go -- handleDraftPoll -->

## Other Config Subcommands

| Command | Description |
|---------|-------------|
| `ze config validate <file>` | Validate configuration file |
| `ze config migrate <file>` | Convert ExaBGP config to ze format |
| `ze config fmt <file>` | Normalize formatting (output to stdout) |
| `ze config dump <file>` | Dump parsed config as JSON tree |
| `ze config diff <a> <b>` | Compare two config files |
| `ze config diff <N> <file>` | Compare rollback revision N against current |
| `ze config set <file> <path> <value>` | Set a single value programmatically |
| `ze config history <file>` | List available rollback revisions |
| `ze config rollback <N> <file>` | Restore revision N |
| `ze config archive <name> <file>` | Archive config to named destination ([details](config-archive.md)) |
| `ze config completion <file>` | Query YANG completion engine (debugging) |
<!-- source: internal/component/config/cli/main.go -- subcommandHandlers, storageHandlers -->

Every `<file>` above accepts `-` for **stdin**. The read-modify commands
`ze config set`, `ze config deactivate`, and `ze config activate` become
pipeline stages when given `-`: they read the config from stdin, apply the
change, and emit the modified config to **stdout** instead of writing back, so
they compose (`ze config show - | ze config set - bgp session asn local 65001`).
`ze config edit -`, `ze config rollback <N> -`, and `ze config history -` are
rejected with a clear error: an interactive editor needs a TTY, and rollback/history
need on-disk revision history that a piped config does not have.
<!-- source: internal/component/config/cli/editor_stdin.go -- openEditableConfig -->
<!-- source: internal/component/cli/editor.go -- NewEditorFromContent, SetStdoutSink -->

`validate`, `show`, and a diff between two explicit paths read those paths
without opening a store. Equal filenames in different folders remain separate
inputs. An offline `set` writes the supplied file and records its previous
content only when that folder already holds a store. Without one it prints
`no version recorded`; it does not create a store. `history`, numbered `diff`,
and `rollback` require the folder's history and name `ze init` when it is absent.
An offline writer refuses while the daemon owns that store.
<!-- source: internal/component/config/cli/editor_stdin.go -- openEditableConfig, noticeUnrecordedVersion -->
<!-- source: internal/component/config/cli/cmd_diff.go -- resolveDiff -->

`ze config import --dir <folder> <file>...` selects a destination independently
of its input files. Without `--dir`, the destination follows `ze.config.dir`
and then the default config folder. `--name` supplies the destination name for
one input, including stdin. Duplicate basenames are refused before any input is
written. A destination name the store already holds is replaced only once
confirmed: on a terminal the command names the configs it would replace and asks,
and `y` or `yes` replaces them while any other answer writes nothing and exits
non-zero. Without a terminal, including when the input is stdin, it asks nothing
and refuses, naming `--yes`; `--yes` confirms in advance. A replaced config is
committed as a new active version and the previous one becomes its rollback, so
`ze config rollback` restores it.
<!-- source: internal/component/config/cli/cmd_import.go -- cmdImportWithStorage, confirmImportReplace, importOne -->
<!-- source: internal/component/config/storage/restore.go -- RestoreConfig -->

## Editing Modes

The selected source determines the editor's mode.
<!-- source: internal/component/config/cli/cmd_edit.go -- cmdEditWithStorage, runEditor, runStoredEditor -->

**File mode** (`ze config edit -f <file>`): the editor reads and writes the
explicit loose file. It keeps its `.edit` recovery file alongside it, and
records rollback versions in an existing store in the file's folder.
`commit confirmed` restores from that history, so in a folder with no store it
refuses before writing anything and names `ze init`. A file changed externally
since it was opened causes commit to fail rather than overwrite the edit.
`-f` cannot be combined with `--web` or `--insecure-web`; use session mode
without `-f` to serve the web editor from the owning daemon.

**Session mode** (`ze config edit`, SSH, web, or the attached console): the
owning daemon gives each session an identity (`user@origin%timestamp`) and a
per-user change file. Commit applies the session's changes with conflict
detection. The same store holds draft recovery and history for every surface.

**Backup mode** (`ze config edit --backup <artifact> [config-name]`): the
editor opens a config inside a backup artifact (a `.zefs` file that `ze data
backup` or `request data backup` wrote) with no daemon. No SSH credentials are
read, no daemon is probed or started, and `run` commands have nothing to reach.
The session behaves as a daemon session does: it has an identity, a draft and
history, all inside the artifact. A commit publishes into the artifact: a new
version, the active and rollback pointers, and `file/active/<name>`. The
status line ends in "and published". `ze data restore <artifact> config` then
applies the edited config to a device.

The editor holds the artifact's `<artifact>.lock` sidecar for the whole
session, so a second `--backup` editor, and any other `--backup` command, on
the same file is refused with an error that names it. A lock on the artifact
itself would not do: each commit rewrites the artifact through a rename,
which installs a new file. Writes pad each changed slot by 10%, so a later
edit of about the same size stays in place; `ze data backup` writes
exact-fit artifacts because nothing edits them. `--backup` cannot be combined
with `-f`, `--web` or `--insecure-web`, and the refusal names both flags.
Without a config name the editor uses the artifact's `<instance>.conf` and
otherwise asks which config to edit.
<!-- source: internal/component/config/cli/cmd_edit.go -- cmdEditBackup -->
<!-- source: internal/component/config/cli/backup_flag.go -- openBackup, publishInBackup -->
<!-- source: internal/component/cli/editor.go -- NewOfflineSessionEditor, SetOfflinePromotion -->

`ze config show`, `diff`, `set` and `list` take the same `--backup <artifact>`
flag and work on the configs inside it: `show` and `diff` read under a shared
lock, `set` publishes the way an editor commit does, and `list` names only
the artifact's configs. `set --backup` refuses `--reload`, because an artifact
has no daemon.

When the daemon was started with an explicit file, every start reads that
file, and daemon commits update it together with stored history. An external
edit must be reconciled before a competing daemon commit can succeed. Bare
`ze start` takes its active configuration from the store.
<!-- source: internal/component/cli/editor.go -- NewLooseFileEditor, NewEditorWithStorage -->
<!-- source: internal/component/cli/editor_commands.go -- Save -->

In a session editor, `commit confirmed <seconds>` runs through the daemon's
confirmed-commit window (see [Commit Confirmed](#commit-confirmed)). A session
editor of a configuration the daemon does not run has no such window, and
refuses it with "commit confirmed needs a daemon that runs this configuration
from its store"; `commit now` still works there. The web editor of
`ze start --web-only` has no daemon and answers the same way.
<!-- source: internal/component/cli/commit_window.go -- errCommitConfirmedNeedsDaemon, WindowCommit.Run -->

`insert` is supported in session mode: `InsertLeafListValue` routes through
`writeThroughMemberOp`, and so do `deactivate` and `activate` when they name a
leaf-list member rather than a leaf or a path.
<!-- source: internal/component/cli/editor_commands.go -- InsertLeafListValue, DeactivateLeafListValue, ActivateLeafListValue -->
<!-- source: internal/component/cli/editor_leaflist.go -- writeThroughMemberOp -->

`copy`, and `deactivate` or `activate` on a leaf or a path, are supported in
session mode too. Each records one structural op in your change file
(`copy-entry`, `deactivate-leaf`, `activate-leaf`, `deactivate-path`,
`activate-path`), shows as one change in `show | changes`, and is applied by
`commit now` before your leaf edits. `copy` never overwrites: a destination that
exists is refused, so delete it first.
<!-- source: internal/component/cli/editor_draft.go -- writeThroughCopy, writeThroughToggle, writeThroughStructuralOp, applyToggleOp -->

`load` works in session mode and in file mode through one path. The input,
from a file or pasted after `load terminal ...` and ended with Ctrl-D, is
parsed against the schema first: at the root it may be hierarchical or `set`
lines, and a `relative` load takes a hierarchical body checked against the
children of your current context, so a key that node does not hold is refused
by name. A `merge` then applies each leaf, leaf-list member, list entry and
inactive marker the input carries that the configuration does not already hold;
a `replace` also deletes what the node holds and the input omits, scoped to the
context for a `relative` load. In session mode each applied difference is one
tracked change, the same entry a typed `set` or `delete` records, so a leaf
equal to its current value records nothing and `show | changes` lists exactly
what the load changed.

A load is all or nothing. A parse error, a refused entry, or a failed write of
the change file leaves the candidate exactly as it was, and the error says
`load refused, candidate unchanged`. In session mode the load holds the change
file's lock for its whole run, reads the change file and the committed
configuration once, applies every difference in memory, and writes the change
file once, at the end, so its cost grows with the size of the input rather than
with its square.
<!-- source: internal/component/cli/model_load.go -- cmdLoadNew, applyLoad -->
<!-- source: internal/component/cli/editor_load.go -- ParseLoad, LoadMerge, LoadReplace, load, loadStage -->
<!-- source: internal/component/cli/editor_draft.go -- openChangeFile, writeChangeFile, readCommittedTree -->
<!-- source: internal/component/config/parser.go -- ParseAt -->

Use file mode (`ze config edit -f`) for the blocked operations above.

| Feature | File mode | Session mode | Backup mode |
|---------|-----------|--------------|-------------|
| Commit | Writes explicit file | Applies tracked changes in the owning daemon | Publishes a version and its pointers into the artifact |
| Multi-user | Offline writer only | Per-user change files | One editor; the artifact lock refuses a second |
| Conflict detection | External file changes | Live and stale changes | Live and stale changes |
| Blame / authorship | No | Yes | Yes |
| Crash recovery | `.edit` file | Change files and draft | Change files and draft, inside the artifact |
| Draft / discard path | No | Yes | Yes |

## YANG Completion

Tab completion is driven by registered YANG schemas. The editor suggests:
- Valid config keys at the current level
- Enum values for leaf nodes
- Address family names from registered plugins
- The keys of a list whose key is an enumeration, including the ones the config
  does not hold yet, each with the help text the schema declares
- Well-known values a plugin offers for one of its own leaves, such as the
  transit-free ASNs on `bgp policy reject-asn ... via`
<!-- source: internal/component/cli/completer.go -- valueCompletions, validateCompletions, listKeyCompletions -->

**A suggestion is never a constraint.** A leaf that offers well-known values
still accepts every value its YANG type admits, so an ASN outside the offered
set is entered and committed with no warning.

A menu row is the config key alone. The second message line above the prompt
shows the summary of the selected key, which the YANG `ze:help` statement
declares. Tab on the value of an enumeration key does the same for each value:
the message line shows the `ze:help` that value declares, and a value that
declares none shows an empty line rather than a placeholder.
<!-- source: internal/component/cli/model_render.go -- renderDropdownBox, warningText -->
<!-- source: internal/component/cli/completer.go -- valueCompletions -->

Press `?` on a highlighted key to read its long explanation, in a box above the
prompt. That text is the `description` statement the schema declares, and it
is often a paragraph. The message line holds one row, so the box is the only place
the paragraph fits. A key that declares no `description` says so on the message
line, and its summary is not repeated in the box.
<!-- source: internal/component/cli/model_keys.go -- revealCandidateExplanation, revealDeclared -->

Tab on a complete config path reveals nothing more to read: `?` is the key that
opens the explanation of a config key. Operational command help is reachable
from configuration mode behind `run `, and the keys are in the
[CLI guide](cli.md#keys-that-reveal-help).
<!-- source: internal/component/cli/model.go -- commandCompleterInput -->

## Commit Confirmed

`commit confirmed <seconds>` writes the configuration and notifies the daemon, but starts a countdown timer. If `commit accept` is not issued before the timer expires, the configuration automatically reverts to the previous version. This prevents lockouts when making changes remotely -- if a bad config breaks connectivity, the auto-revert restores access.
<!-- source: internal/component/cli/model_load.go -- cmdCommitConfirmed, handleConfirmCountdown, rollbackConfirmed -->

| Step | What happens |
|------|-------------|
| `commit confirmed 60` | Config saved, daemon notified, 60-second timer starts |
| Verify the change works | BGP sessions come up, routes propagate, etc. |
| `commit accept` | Timer stops, config is permanent |
| *or* `commit abort` | Config reverts immediately |
| *or* timer expires | Config reverts automatically |

The seconds parameter accepts values from 1 to 3600 (one hour).

In an SSH session editor or the web editor of the daemon's own
configuration, the daemon owns the window, not your session. Both editors
commit through the same window code, so the rules below hold for a web user
too, from the web terminal, the CLI bar and the "Review & Commit" button.
The API config sessions, `request data restore`, and the web raw-source editor
do not commit through the window yet: a commit they make while it is open is
reverted with it. Closing or losing the session leaves the countdown
running, and the revert still happens at the deadline. The window belongs to
the user who opened it: from any session as that user, `commit accept` and
`commit abort` answer it, and the status line shows "Confirm within <N>s or
auto-revert". Every other user's `commit now`, `commit confirmed`,
`commit accept` and `commit abort` are refused while it is open, and their
status line shows "A confirmed commit by <user> is pending:
<N>s left." A session that saw the window open reports how it closed: a
timeout says "Timeout: configuration automatically rolled back", and an accept
or abort from another of your sessions says the window "was closed by another
session". The web terminal shows each command's answer but has no status
line, so it shows neither the countdown nor how a window it did not close
ended. Inside your own window, `commit confirmed <seconds> force` applies
your new changes and restarts the countdown at `<seconds>`; the revert still
restores the configuration from before the first commit.

File mode (`ze config edit -f`) keeps the countdown in the editor itself
rather than in a daemon. Inside that window, `commit now` and a plain
`commit confirmed` are refused as in a session editor, and
`commit confirmed <seconds> force` takes the same nested form: the new changes
apply, the countdown restarts at `<seconds>`, and an abort or a timeout restores
the backup the first commit recorded.
<!-- source: internal/component/cli/model_commands_commit.go -- cmdCommitRequest, cmdCommitConfirmedNested -->

The open window is recorded in the store before the commit applies, so no
commit is ever applied without its revert: a record that cannot be saved
refuses the commit and applies nothing. If the daemon stops while the window
is open, even just after the commit applied, the next start reverts it before
the configuration is read, so the daemon boots the configuration from before
the unconfirmed commit.

If the revert at the deadline fails, the window stays open and its record
stays in the store. The daemon logs each failure and retries with a doubling
wait, from one second up to one minute, eight attempts in all. Every SSH
session's status line says the revert failed, with the error and when the next
retry runs, and every other user's commit is refused with the same error,
because a commit made meanwhile would be wiped by the revert that later
succeeds. The owner may run `commit abort` to retry the revert at once, or
`commit accept` to keep the configuration and close the window. Once the
retries are spent, the window waits for one of those two, or for a restart,
which reverts from the record. The countdown never shows a negative number.
<!-- source: internal/component/config/confirm/confirm.go -- Window, Confirmed, RecoverOnStart -->
<!-- source: internal/component/config/confirm/store.go -- StoreRecorder -->
<!-- source: internal/component/cli/commit_window.go -- WindowCommit.Run -->
<!-- source: internal/component/cli/model_commit_window.go -- cmdCommitWindowRequest, pollDaemonWindow -->
<!-- source: internal/component/web/editor_commit_window.go -- runCommit -->
<!-- source: internal/component/web/handler_config_commit.go -- handleCommitPost -->
<!-- source: cmd/ze/hub/confirm_window.go -- startConfirmWindow, recoverConfirmWindow -->

The revert restores the rollback revision the commit records, so `commit
confirmed` needs config history. An editor without one, such as
`ze config edit -f` on a file whose folder holds no store, refuses before it
writes the file and names `ze init`; `commit now` still works there.

<!-- terminal-demo: commit-confirmed -->

## Rollback

The editor automatically saves a rollback revision before each commit. Inside the editor, use `show | history` to list revisions and `rollback <N>` to restore. From the shell: `ze config history <file>` and `ze config rollback <N> <file>`.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Configuration has errors (from `validate` command) |
| 2 | Error (file not found, parse failure) |
<!-- source: internal/component/config/cli/main.go -- exitOK, exitError -->

## Example Workflow

```bash
ze config validate config.conf      # Pre-flight validation
ze config edit config.conf          # Interactive editing
ze config diff 3 config.conf       # Compare with revision 3
ze config rollback 3 config.conf   # Restore revision 3
ze config archive prod config.conf # Archive to named destination
```

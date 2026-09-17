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
operational prompt, where `configure` enters configuration mode. A `commit` from
any of the three reloads the daemon.
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
| `commit` | Save changes and notify daemon |
| `commit confirmed <N>` | Commit with N-second auto-revert window (1-3600) |
| `confirm` | Make a pending confirmed commit permanent |
| `confirm abort` | Roll back a pending confirmed commit immediately |
| `rollback <N>` | Restore revision N |
| `top` | Navigate to config root |
| `up` | Navigate up one level |
| `edit <path>` | Navigate into a section |
| `exit` | Exit editor |
<!-- source: internal/component/cli/editor_commands.go -- editor commands (set, delete, show, diff, commit, rollback) -->

`commit` reaches the same reload as `ze signal reload`. Which configuration
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
entered them. Pasting a block of `set` lines followed by `commit` over SSH lands
every `set` before the `commit` reads the draft. Nothing is dropped and no
command is refused for arriving while another is in flight.
<!-- source: internal/component/cli/model_commands.go -- dispatchQueue -->

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
one input, including stdin. Duplicate basenames and existing destination names
are refused before any input is written.
<!-- source: internal/component/config/cli/cmd_import.go -- cmdImportWithStorage -->

## Editing Modes

The selected source determines the editor's mode.
<!-- source: internal/component/config/cli/cmd_edit.go -- cmdEditWithStorage, runEditor, runStoredEditor -->

**File mode** (`ze config edit -f <file>`): the editor reads and writes the
explicit loose file. It keeps its `.edit` recovery file alongside it, and
records rollback versions in an existing store. A file changed externally
since it was opened causes commit to fail rather than overwrite the edit.
`-f` cannot be combined with `--web` or `--insecure-web`; use session mode
without `-f` to serve the web editor from the owning daemon.

**Session mode** (`ze config edit`, SSH, web, or the attached console): the
owning daemon gives each session an identity (`user@origin%timestamp`) and a
per-user change file. Commit applies the session's changes with conflict
detection. The same store holds draft recovery and history for every surface.

When the daemon was started with an explicit file, every start reads that
file, and daemon commits update it together with stored history. An external
edit must be reconciled before a competing daemon commit can succeed. Bare
`ze start` takes its active configuration from the store.
<!-- source: internal/component/cli/editor.go -- NewLooseFileEditor, NewEditorWithStorage -->
<!-- source: internal/component/cli/editor_commands.go -- Save -->

Some commands are not yet supported in session mode because they replace the tree wholesale and cannot be expressed as tracked change entries. These return an error when attempted:

<!-- source: internal/component/cli/editor_commands.go -- errLoadNotSupportedInSessionMode, errCopyNotSupportedInSessionMode, errDeactivateNotSupportedInSessionMode, errActivateNotSupportedInSessionMode -->
<!-- source: internal/component/cli/model_commands.go -- errCommitConfirmedNotYetSupportedIn -->
<!-- source: internal/component/cli/model_commands_commit.go -- errCommitForceNotYetSupportedIn -->

| Blocked command | Reason |
|-----------------|--------|
| `load` | Replaces tree without generating per-leaf change entries |
| `copy` | Creates structure without write-through |
| `deactivate` on a leaf or a path | Requires metadata write-through |
| `activate` on a leaf or a path | Requires metadata write-through |
| `commit confirmed` | Needs session-aware rollback |
| `commit force` | Needs session-aware rollback |

`insert` is supported in session mode: `InsertLeafListValue` routes through
`writeThroughMemberOp`, and so do `deactivate` and `activate` when they name a
leaf-list member rather than a leaf or a path.
<!-- source: internal/component/cli/editor_commands.go -- InsertLeafListValue, DeactivateLeafListValue, ActivateLeafListValue -->
<!-- source: internal/component/cli/editor_leaflist.go -- writeThroughMemberOp -->

Use file mode (`ze config edit -f`) for these operations.

| Feature | File mode | Session mode |
|---------|-----------|--------------|
| Commit | Writes explicit file | Applies tracked changes in the owning daemon |
| Multi-user | Offline writer only | Per-user change files |
| Conflict detection | External file changes | Live and stale changes |
| Blame / authorship | No | Yes |
| Crash recovery | `.edit` file | Change files and draft |
| Draft / discard path | No | Yes |

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

`commit confirmed <seconds>` writes the configuration and notifies the daemon, but starts a countdown timer. If `confirm` is not issued before the timer expires, the configuration automatically reverts to the previous version. This prevents lockouts when making changes remotely -- if a bad config breaks connectivity, the auto-revert restores access.
<!-- source: internal/component/cli/model_load.go -- cmdCommitConfirmed, handleConfirmCountdown, rollbackConfirmed -->

| Step | What happens |
|------|-------------|
| `commit confirmed 60` | Config saved, daemon notified, 60-second timer starts |
| Verify the change works | BGP sessions come up, routes propagate, etc. |
| `confirm` | Timer stops, config is permanent |
| *or* `confirm abort` | Config reverts immediately |
| *or* timer expires | Config reverts automatically |

The seconds parameter accepts values from 1 to 3600 (one hour).

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

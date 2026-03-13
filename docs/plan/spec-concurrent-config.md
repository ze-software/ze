# Spec: Concurrent Configuration Editing

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/7 |
| Updated | 2026-03-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/config/syntax.md` - current config syntax
4. `internal/component/cli/editor.go` - current editor implementation
5. `internal/component/config/tree.go` - config tree data structure
6. `internal/component/config/serialize.go` - tree serialization
7. `internal/component/cli/model_commands.go` - editor commands (set, delete, commit)

## Task

Replace the hierarchical text configuration format with a flat, line-oriented format where each line is a CLI `set` command with optional metadata prefixes. Enable concurrent editing from multiple sessions (terminal, SSH) with write-through semantics, per-session commit, conflict detection, and authorship tracking.

### Goals

1. **New config format:** Each line = optional metadata + a `set` command. The file is both human-readable and machine-parseable. Metadata is optional so hand-written files work without it.
2. **Write-through:** Every `set`/`delete` writes immediately to disk. No accumulation in memory.
3. **Concurrent editing:** Multiple editors (terminal + SSH) work on a shared draft file with advisory locking. Each editor detects changes made by others.
4. **Per-session commit:** Each editing session has an identity. `commit` applies only the current session's changes to the committed config. Other sessions' pending changes are preserved.
5. **Conflict detection:** Two conflict types: (a) two active sessions disagree on the same YANG path, (b) the committed value changed since the editor's last `set` at that path. Any conflict blocks the entire commit (not just the conflicting keys).
6. **Authorship tracking:** Every value carries who changed it and when. Survives schema migrations because metadata keys follow YANG paths, not line numbers.
7. **Multiple views:** The flat format on disk is rendered as tree view, set view, blame view, or changes view depending on user command.
8. **Save and commit are distinct:** `save` persists the draft with metadata (work survives across sessions, no effect on running config). `commit` applies the session's changes to the running config.
9. **Session management:** `who` lists active editing sessions. `disconnect <session>` removes another session's pending changes. On exit with pending changes, prompt to save or discard all.
10. **Startup flow:** No interactive "Found uncommitted changes" prompt. Draft loaded automatically. Same-user orphaned sessions prompt for adoption.

### Non-Goals

- Automatic three-way merge of conflicting changes (future work)
- Real-time push notifications between editors (polling via mtime is sufficient)
- Distributed editing across multiple machines (single filesystem assumed)
- Changes to the YANG schema itself
- `save` to terminal display or file export (future work)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/syntax.md` - current config format, all keywords, value types
  -> Constraint: the `set` command syntax must cover every construct described here (leaves, lists, containers, leaf-lists, presence containers, inline lists, freeform blocks)
  -> Decision: the new format's `set` commands use the same token syntax as the editor CLI already uses
- [ ] `docs/architecture/config/yang-config-design.md` - YANG schema structure
  -> Constraint: metadata keys must follow YANG tree paths so migrations transform both config and metadata together
  -> Decision: `_` prefix reserved for metadata tokens; YANG forbids `_` in identifiers, so no collision

### Source Files
- [ ] `internal/component/cli/editor.go` - Editor struct, NewEditor, Save, Discard, SetValue, DeleteValue, SaveEditState, LoadPendingEdit, PromptPendingEdit
  -> Constraint: Editor currently holds an in-memory tree and writes only on explicit save/commit. Must change to write-through.
  -> Decision: Editor gains a lock file handle, session identity, and write-through methods
- [ ] `internal/component/cli/model_commands.go` - cmdSet, cmdDelete, cmdCommit, cmdDiscard, cmdSave
  -> Constraint: cmdSet calls editor.SetValue then returns. Must now also trigger write-through.
  -> Decision: editor.SetValue becomes the write-through entry point (lock, read, apply, write, unlock)
- [ ] `internal/component/config/tree.go` - Tree struct (values, containers, lists, listOrder)
  -> Constraint: Tree is the canonical in-memory representation. Must remain so.
  -> Decision: Tree gains a parallel MetaTree for per-node metadata. MetaTree follows same structure but stores MetaEntry at leaves.
- [ ] `internal/component/config/serialize.go` - Serialize, SerializeSubtree, serializeNode
  -> Constraint: Serialize produces the current hierarchical text format. Must add new serializers.
  -> Decision: add SerializeSet (flat set commands), SerializeSetWithMeta (with prefixes), keep Serialize for tree view
- [ ] `internal/component/config/parser.go` - Parser.Parse
  -> Constraint: current parser handles hierarchical text. Must add set-format parser.
  -> Decision: auto-detect format via `DetectFormat()` (in `serialize_set.go`) by first non-comment, non-empty line (starts with `set` or `delete` = flat format, starts with `#` followed by `set` = flat with meta, otherwise = hierarchical text for migration)
- [ ] `internal/component/ssh/session.go` - createSessionModel, NewCommandModel
  -> Constraint: SSH sessions currently use NewCommandModel (command-only, no editor). Must gain editor access. SSH Server has no config file path today.
  -> Decision: Add ConfigPath to ssh.Config (set by daemon loader). SSH sessions receive an Editor pointed at the same config file, with username from SSH auth as identity. Wiring mirrors cmd_edit.go: SetSession, SetReloadNotifier, SetArchiveNotifier, SetCommandExecutor, SetCommandCompleter.
- [ ] `cmd/ze/config/cmd_edit.go` - cmdEdit, PromptPendingEdit flow, wireCommandExecutor
  -> Constraint: startup currently blocks on interactive prompt if .edit exists. Must remove.
  -> Decision: if draft exists, load it automatically. Display pending sessions from metadata.

**Key insights:**
- The `set` command path syntax already exists in the editor CLI (cmdSet in model_commands.go)
- The YANG schema drives both parsing and serialization, so adding a new serialization format is straightforward
- The Tree data structure is format-agnostic -- it can be populated from hierarchical text or flat set commands
- SSH sessions are command-only today; giving them an Editor is the main wiring change for concurrent access
- Advisory file locking (flock) is the simplest cross-process synchronization on Unix

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/cli/editor.go` - Editor manages in-memory tree, writes to `.edit` on auto-save, writes to original on commit. No locking. No session identity. Single-user.
- [ ] `internal/component/cli/model_commands.go` - cmdSet modifies in-memory tree, does not write to disk. cmdCommit calls editor.Save() which writes original + deletes .edit.
- [ ] `internal/component/config/serialize.go` - Serialize produces hierarchical text from Tree + Schema.
- [ ] `internal/component/config/parser.go` - Parser.Parse reads hierarchical text into Tree.
- [ ] `cmd/ze/config/cmd_edit.go` - On startup: checks for .edit file, prompts user (blocking stdin read), then starts editor.
- [ ] `internal/component/ssh/session.go` - SSH sessions use NewCommandModel (no editor, command-only).

**Behavior to preserve:**
- The `set` command syntax in the editor CLI (path + value tokenization, YANG validation)
- The tree view display format (hierarchical with indentation, braces)
- The `compare` command showing diff markers (+/-/*)
- The `rollback/` backup directory and rollback mechanism
- The `commit confirmed` / `.live` file mechanism
- Archive notifier on commit
- Reload notifier on commit (daemon notification)
- ExaBGP config auto-detection and migration
- All existing functional tests for config parsing

**Behavior to change:**
- Config file format: from hierarchical text to flat set commands with optional metadata
- Editor write model: from in-memory accumulation to write-through
- Startup flow: from interactive prompt to automatic draft loading with same-user adoption prompt
- SSH sessions: from command-only to editor-capable
- Add file locking for concurrent access
- Add session identity and per-session commit
- Add multiple display views (tree, set, blame, changes)
- Separate `save` (persist draft) from `commit` (apply to running config)
- Add `who` and `disconnect` session management commands
- On exit with pending changes: prompt save or discard all
- `discard` requires explicit path or `all` (bare `discard` rejected)

## New Configuration File Format

### On-Disk Format

Each line is an optional metadata prefix followed by a `set` command. Lines starting with `#` (without a user sigil) are comments. Empty lines are preserved for readability.

#### Without metadata (hand-written or exported)

```
set router-id 1.2.3.4
set bgp peer 10.0.0.1 local-as 65000
set bgp peer 10.0.0.1 hold-time 90
set bgp peer 10.0.0.1 peer-as 65001
set bgp peer 10.0.0.1 capability route-refresh enable
set bgp peer 10.0.0.1 family ipv4/unicast
set bgp peer 10.0.0.1 update attribute origin igp
set bgp peer 10.0.0.1 update attribute next-hop 10.0.0.1
set bgp peer 10.0.0.1 update nlri ipv4/unicast add 10.0.0.0/24
```

#### With metadata (written by ze editor)

```
#thomas@local @2026-03-12T14:30:01 set router-id 1.2.3.4
#thomas@local @2026-03-12T14:30:01 set bgp peer 10.0.0.1 local-as 65000
#alice@ssh @2026-03-12T14:31:00 set bgp peer 10.0.0.1 hold-time 90
#thomas@local @2026-03-12T14:30:05 set bgp peer 10.0.0.1 peer-as 65001
```

#### Mixed (some lines have metadata, some do not)

```
# Global settings
set router-id 1.2.3.4
#thomas@local @2026-03-12T14:30:01 set bgp peer 10.0.0.1 local-as 65000
#alice@ssh @2026-03-12T14:31:00 set bgp peer 10.0.0.1 hold-time 90
set bgp peer 10.0.0.1 peer-as 65001
```

This is valid. The parser treats metadata-less lines as having unknown authorship.

### Metadata Prefix Grammar

```
<line>     := [<comment>] | [<meta>...] <command>
<comment>  := "#" <text-without-sigil-after-hash>
<meta>     := <user-meta> | <time-meta> | <session-meta> | <prev-meta>
<user-meta>    := "#" <user-id>          (# immediately followed by non-space identifier)
<time-meta>    := "@" <iso8601>
<session-meta> := "%" <session-id>
<prev-meta>    := "^" <value>            (committed value before this edit, for stale conflict detection)
<command>  := "set" <path> <value>
            | "delete" <path>
```

#### Distinguishing comments from user metadata

A line starting with `#` is either a comment or a user metadata token:
- `# text` (hash followed by space) = comment. Preserved as-is.
- `#user` (hash immediately followed by non-space characters) = user metadata token.

This is unambiguous because user identifiers cannot start with a space.

#### Metadata tokens

| Sigil | Field | Format | Example | Required |
|-------|-------|--------|---------|----------|
| `#` | User | `user@origin` | `#thomas@local` | No |
| `@` | Timestamp | ISO 8601 | `@2026-03-12T14:30:01` | No |
| `%` | Session | `user@origin:unix-ts` | `%thomas@local:1741783801` | No (draft only) |
| `^` | Previous | value string | `^90` | No (draft only) |

All four are optional. They appear in any order before the `set`/`delete` command. The parser consumes all tokens starting with `#` (user), `@` (time), `%` (session), or `^` (previous) as metadata, then treats the remainder as the command.

#### Session metadata (`%`) and previous value (`^`) are draft-only

The committed config file (`config.conf`) never contains `%session` or `^previous` tokens. These exist only in the draft file (`config.conf.draft`): `%session` tracks which editing session made each pending change, and `^previous` records the committed value at the time of the edit (for stale conflict detection).

When a line is committed, its `%session` and `^previous` tokens are removed. The `#user` and `@timestamp` are updated to reflect the committer and commit time.

### YANG Path Derivation

Each `set` command implicitly encodes a YANG path. The path is derived by the existing YANG-aware tokenizer (already used by the editor's `cmdSet` for validation). The last token is the value; everything between `set` and the value is the path.

For list entries, the key is part of the path:

| Command | YANG Path | Value |
|---------|-----------|-------|
| `set router-id 1.2.3.4` | `router-id` | `1.2.3.4` |
| `set bgp peer 10.0.0.1 hold-time 90` | `bgp/peer/10.0.0.1/hold-time` | `90` |
| `set bgp peer 10.0.0.1 capability route-refresh enable` | `bgp/peer/10.0.0.1/capability/route-refresh` | `enable` |
| `set bgp peer 10.0.0.1 family ipv4/unicast` | `bgp/peer/10.0.0.1/family/ipv4\/unicast` | (presence) |

Two lines with the same YANG path represent the same leaf. When a `set` changes a value, it replaces the line with that path. This is how conflict detection works: two sessions modifying the same YANG path = potential conflict.

### Ordering

Lines in the file are ordered by YANG schema order (same as the current Serialize output). When writing the file, the serializer walks the tree in schema order and emits one `set` line per leaf. This means the file order is deterministic and diff-friendly.

### Leaf-Lists and Multi-Value Fields

Leaf-lists (e.g., `community`, `as-path`) use bracket syntax on a single line:

```
set bgp peer 10.0.0.1 update attribute community [ 65001:100 65001:200 ]
set bgp peer 10.0.0.1 update attribute as-path [ 65001 65002 ]
```

The entire bracket list is the "value" of that leaf-list. Changing any element replaces the whole line.

### Presence Containers and Flags

Presence containers that are boolean flags use no value:

```
set bgp peer 10.0.0.1 update attribute atomic-aggregate
```

The absence of a value after the last path element signals a presence/flag node.

### Delete Command

```
delete bgp peer 10.0.0.1 hold-time
```

Removes the leaf at that path. In the file, the corresponding `set` line is removed. In the draft, a delete is recorded with metadata:

```
#thomas@local @2026-03-12T14:35:00 %thomas@local:T1 delete bgp peer 10.0.0.1 hold-time
```

Delete lines are present in the draft to track the operation. They are removed from the committed file (since the value no longer exists). At commit time, a delete means "remove this line from config.conf".

### Comments

```
# This is a comment
# Comments are preserved in the file

set router-id 1.2.3.4
```

Comments (lines starting with `# ` -- hash followed by space) are preserved during read/write. They are not metadata.

## Files on Disk

### Committed vs. Draft

| File | Purpose | Contains `%session` | Created by | Deleted when |
|------|---------|---------------------|------------|-------------|
| `config.conf` | Committed config (daemon uses this) | Never | `commit` command | Never (user's config) |
| `config.conf.draft` | Working config (all editors' pending changes) | Yes, for pending changes | First `set`/`delete` in any session | All sessions committed or discarded |
| `config.conf.lock` | Advisory lock file | N/A | First editor to write | Never (empty file, reused) |

### Lock File Protocol

The lock file `config.conf.lock` is an empty file used with POSIX `flock(2)` for advisory locking:

| Step | syscall | Mode | Purpose |
|------|---------|------|---------|
| 1 | `OpenFile(lockPath, O_CREATE\|O_RDWR, 0o600)` | - | Create/open lock file |
| 2 | `flock(fd, LOCK_EX)` | Blocking exclusive | Acquire lock (blocks if held by another process) |
| 3 | Read, modify, write draft | - | Critical section |
| 4 | `flock(fd, LOCK_UN)` | Unlock | Release lock |

The lock is held for the duration of a single read-modify-write cycle (milliseconds). It is never held while waiting for user input.

### Draft Lifecycle

1. **No draft exists:** Editor starts from `config.conf`. First `set`/`delete` creates `config.conf.draft` (copy of committed + the new change with `%session`).
2. **Draft exists, I join:** Editor reads `config.conf.draft`. My changes add `%session` lines. Other sessions' `%session` lines are preserved.
3. **I commit:** My `%session` lines are applied to `config.conf`. My lines in the draft lose their `%session` (or are removed if they now match the committed value). If no `%session` lines remain in the draft, delete the draft file.
4. **I discard:** My `%session` lines are removed from the draft. Lines revert to committed values (re-read from `config.conf`). If no `%session` lines remain, delete the draft file.

## Session Identity

### Format

```
<user>@<origin>
```

| Component | Source | Example |
|-----------|--------|---------|
| `user` | `$USER` env var (terminal), SSH authenticated username | `thomas`, `alice` |
| `origin` | `local` for terminal, `ssh` for SSH sessions | `local`, `ssh` |

### Session ID

```
<user>@<origin>:<unix-timestamp>
```

Example: `thomas@local:1741783801`

The unix timestamp is the time the editing session started. This distinguishes multiple sessions from the same user (e.g., two terminals). The session ID is used as the `%session` token in draft lines.

### Session Discovery

On startup, the editor reads the draft file (if it exists) and extracts all unique `%session` values.

**Same-user orphaned sessions:** If the draft contains `%session` entries from the same username but a different session ID (e.g., previous terminal or SSH session that disconnected), the editor prompts for adoption:

```
Found pending changes from previous session (alice@ssh, started 14:30, 3 changes):
  set bgp peer 10.0.0.1 hold-time 90
  set bgp peer 10.0.0.1 peer-as 65001
  set bgp peer 10.0.0.1 local-as 65000

Adopt these changes? (yes/no/show)
```

- `yes` -- moves old `%session` entries to the new session ID
- `no` -- leaves them as orphaned (visible via `who`, removable via `disconnect`)
- `show` -- displays details before deciding

**Adoption implementation:** `Editor.AdoptSession(oldSessionID string) error` method in `editor_draft.go`:

| Step | Action |
|------|--------|
| 1 | Acquire lock |
| 2 | Read and parse draft |
| 3 | Find all MetaTree entries with `%session` matching `oldSessionID` |
| 4 | Rewrite each entry's `Session` field to the current session's ID |
| 5 | Serialize and write draft atomically |
| 6 | Release lock |

**Other users' sessions:** displayed as information, no prompt:

```
Active editing sessions:
  thomas@local (started 2026-03-12 14:28) - 5 pending changes
```

The editor then proceeds directly to the editing interface.

### Exit Prompt

When the user quits the editor (Ctrl-C, `exit`, or `quit`) and has pending changes in the draft, the model intercepts the quit key message in `model.go`'s `Update` handler.

| Condition | Behavior |
|-----------|----------|
| No pending changes for this session | Quit immediately |
| Pending changes exist | Display status: "Pending changes. Use 'commit', 'discard all', or Esc to force quit." Enter `confirmQuit` state. |
| User presses Esc/Ctrl-C/y while in `confirmQuit` | Auto-save snapshot, quit (draft already on disk via write-through, snapshot is best-effort) |
| Any other key while in `confirmQuit` | Cancel quit, return to editor |

Both intercept points (`handleEnter` for `exit`/`quit` commands, `handleKeyMsg` for Ctrl-C/Esc) use the shared `Model.hasPendingChanges()` helper. In session mode it checks `Editor.HasPendingSessionChanges()` (`len(meta.SessionEntries(session.ID)) > 0`). In non-session mode it checks `Editor.Dirty()`. In session mode, `autoSaveOnQuit()` is a no-op since write-through already persists to the draft file. In non-session mode, it writes a `.edit` snapshot as before.

## Write-Through Protocol

### For `set` commands

When the user types `set bgp peer 10.0.0.1 hold-time 90`:

| Step | Action |
|------|--------|
| 1 | Validate the command against YANG schema (existing validation, no change) |
| 2 | `flock(config.lock, LOCK_EX)` |
| 3 | Read `config.draft` from disk (or `config.conf` if no draft) |
| 4 | Parse file into Tree + MetaTree |
| 5 | Apply the `set` to the tree (existing `SetValue`) |
| 6 | Record metadata: `MetaEntry{User: session.user, Time: now, Session: session.id}` |
| 6a | Read `config.conf` to get the committed value at this YANG path |
| 7 | Record the committed value as `Previous` in the MetaEntry (always from `config.conf`, never from draft) |
| 8 | Serialize tree to flat set format with metadata: `SerializeSetWithMeta()` |
| 9 | Write `config.draft` atomically (temp file + rename) |
| 10 | `funlock(config.lock)` |
| 11 | Update in-memory tree and display |

The lock is held from step 2 to step 10 (a few milliseconds). No I/O to network or user during the lock.

### For `delete` commands

Same protocol. Step 5 removes the value from the tree. Step 8 omits the deleted path from set lines but adds a `delete` line with metadata in the draft.

### Concurrent read by other editors

**Status: not yet implemented (assigned to Phase 6).**

Each editor caches the `mtime` of `config.draft`. A periodic `tea.Tick` in `model.go` checks the draft file's mtime:

| `mtime` changed? | Action |
|-------------------|--------|
| No | Proceed with cached tree |
| Yes | Re-read `config.draft`, re-parse tree, update display, show notification of changes by other sessions |

Implementation in `model.go`:

| Component | Location | Purpose |
|-----------|----------|---------|
| `draftMtime` field | `Model` struct | Cached mtime of `config.draft` |
| `draftPollMsg` | Tick message | Fires every 2 seconds to check mtime |
| `checkDraftMtime()` | `Update` handler | Stats draft file, compares with cached mtime, triggers re-read if changed |

The notification shows recent changes from other sessions (entries whose `@timestamp` is newer than our last read and whose `%session` differs from ours):

```
[alice@ssh 14:31:00] set bgp peer 10.0.0.1 hold-time 90
```

## Per-Session Commit

When the user types `commit`:

| Step | Action |
|------|--------|
| 1 | `flock(config.lock, LOCK_EX)` |
| 2 | Read `config.conf` (committed) and `config.draft` |
| 3 | Parse both into trees |
| 4 | Identify my changes: all draft lines where `%session == my_session_id` |
| 5 | **For each of my changes, check for two types of conflicts:** |
| 5a | **Live disagreement:** check if another active session in the draft has a pending change at the same YANG path with a different value. If same value: no conflict (they agree). |
| 5b | **Stale Previous:** read the YANG path's current value in `config.conf`. Compare with `Previous` recorded in my MetaEntry. If `config.conf` value != `Previous`: the committed value changed since my edit. **CONFLICT.** |
| 5c | If both sessions set the same value (agreement), no conflict -- first to commit wins. |
| 6 | **If any conflict (either type):** report ALL conflicts to user, do not commit ANY changes, release lock. The entire commit is blocked, not just conflicting keys. |
| 7 | **If no conflicts:** |
| 7a | Apply my changes to the committed tree. For each session entry: retrieve the value from the draft tree at that YANG path. If the value is non-empty, `Set` it in the committed tree. If the value is empty (the session deleted this path -- the draft tree has no value but metadata exists via `delete` line), `Delete` it from the committed tree. |
| 7b | Serialize committed tree to `config.conf` (with `#user @timestamp`, no `%session`) |
| 7c | Create backup in `rollback/` |
| 7d | Remove my `%session` entries from the draft tree |
| 7e | If other sessions still have pending changes: regenerate `config.draft` without my entries |
| 7f | If no pending changes remain: delete `config.draft` |
| 8 | `funlock(config.lock)` |
| 9 | Notify daemon (reload notifier, existing mechanism) |
| 10 | Archive (archive notifier, existing mechanism) |

### Conflict Display

```
Commit blocked: 2 conflict(s) -- entire commit refused

  bgp peer 10.0.0.1 hold-time
    Your value:      90
    Committed value: 120  (by alice@ssh at 14:31)
    Your original:   180
    Type: stale (committed value changed since your edit)

  bgp peer 10.0.0.1 med
    Your value:      100
    alice@ssh value: 200
    Type: live disagreement (active session has different value)

Resolution:
  'discard <path>'   -- drop your change at that path
  'set <path> <val>' -- re-apply your value (updates baseline to current committed value)
  Both sessions set the same value -- agreement, conflict resolved
```

### Per-Session Discard

**`discard` (bare, no arguments) is rejected** -- too dangerous to type by accident.

| Command | Effect |
|---------|--------|
| `discard <path>` | Discard my change at that specific leaf. Restore to committed value (from `config.conf`), or remove if newly added. |
| `discard <container-path>` | Find all leaves under that subtree that my session modified, restore each one. |
| `discard all` | Discard ALL my pending changes. |

When the user types `discard <path>` or `discard all`:

| Step | Action |
|------|--------|
| 1 | `flock(config.lock, LOCK_EX)` |
| 2 | Read `config.draft` and `config.conf` |
| 3 | Identify lines to discard: matching `%session == my_session_id` at the given path (or all paths for `discard all`) |
| 4 | For each discarded line: restore the value from `config.conf` (or remove if it was an addition not in `config.conf`) |
| 5 | If other sessions still have pending changes: write updated `config.draft` |
| 6 | If no pending changes remain: delete `config.draft` |
| 7 | `funlock(config.lock)` |

## Display Views

All views are generated from the in-memory tree. The on-disk format is always flat set commands with metadata.

### `show` -- Tree View (default, existing format)

```
router-id 1.2.3.4

bgp {
    peer 10.0.0.1 {
        local-as 65000
        hold-time 90
        peer-as 65001
        capability {
            route-refresh enable
        }
        family {
            ipv4/unicast
        }
    }
}
```

Generated by the existing `Serialize()` function. No metadata. This is what the user sees by default in the editor viewport.

### `show set` -- Set Commands View

```
set router-id 1.2.3.4
set bgp peer 10.0.0.1 local-as 65000
set bgp peer 10.0.0.1 hold-time 90
set bgp peer 10.0.0.1 peer-as 65001
set bgp peer 10.0.0.1 capability route-refresh enable
set bgp peer 10.0.0.1 family ipv4/unicast
```

Flat set commands without metadata. This is the exportable format -- a user can save this to a file and use it as a config for ze (or paste it into another editor session). Available in both session and non-session mode (no metadata dependency).

### `show blame` -- Annotated Tree View

The tree view with a left gutter showing authorship and change type. The gutter is fixed-width, aligned with padding.

```
thomas@local  03-12 14:30  + router-id 1.2.3.4
                           .
thomas@local  03-12 14:30  + bgp {
thomas@local  03-12 14:30  +     peer 10.0.0.1 {
thomas@local  03-12 14:30  +         local-as 65000
alice@ssh     03-12 14:31  *         hold-time 90
thomas@local  03-12 14:30  +         peer-as 65001
                                     capability {
                                         route-refresh enable
                                     }
                                     family {
                                         ipv4/unicast
                                     }
thomas@local  03-12 14:30  +     }
thomas@local  03-12 14:30  + }
```

#### Gutter columns

| Column | Width | Content | Padding |
|--------|-------|---------|---------|
| User | 14 chars | `#user` field, right-padded with spaces | Fixed |
| Date | 5 chars | `MM-DD` from `@timestamp` | Fixed |
| Gap 1 | 1 char | Space between date and time | Fixed |
| Time | 5 chars | `HH:MM` from `@timestamp` | Fixed |
| Gap 2 | 2 chars | Two spaces before marker | Fixed |
| Marker | 1 char | `+`, `-`, `*`, or space | Fixed |
| Gap 3 | 1 char | Trailing space after marker | Fixed |

Total gutter: 29 characters (`blameGutterWidth` constant).

#### Diff markers

| Marker | Meaning |
|--------|---------|
| `+` | Line added (not in committed config) |
| `-` | Line deleted (in committed config, absent from draft) |
| `*` | Line modified (different value from committed config) |
| ` ` | Unchanged (same as committed) or no metadata |
| `.` | Empty line separator (visual grouping) |

The marker compares the draft value against the committed value at the same YANG path.

Lines without metadata (imported, hand-written) show an empty gutter (spaces only) and no marker.

#### Container/list braces

Braces (`{`, `}`) in the tree view inherit the marker of their first/last child that has a marker. If all children are unchanged, the brace line has no marker. If any child is added, the opening brace gets `+`. This matches the current `compare` command behavior.

### `show changes` -- My Pending Changes (default)

```
  + set router-id 1.2.3.4                              (new)
  + set bgp peer 10.0.0.1 local-as 65000               (new)
  * set bgp peer 10.0.0.1 peer-as 65001 65002          (was: 65001)
  - delete bgp peer 10.0.0.1 hold-time                 (was: 180)
```

Shows the current session's pending changes with markers and previous values. This is the default because the common question is "what did I change?"

#### Change markers

| Marker | Meaning | Command |
|--------|---------|---------|
| `+` | New value (not in committed config) | `set` |
| `*` | Modified value (different from committed) | `set` |
| `-` | Deleted value (was in committed config) | `delete` |

Delete entries have `Value == ""` in the MetaEntry. The display uses "delete" instead of "set" and shows the previous value.

### `show changes all` -- All Sessions' Pending Changes

```
Session: thomas@local:1741783800 (3 changes)
  + set router-id 1.2.3.4                              (new)
  + set bgp peer 10.0.0.1 local-as 65000               (new)
  + set bgp peer 10.0.0.1 peer-as 65001                (new)

Session: alice@ssh:1741783860 (1 change)
  * set bgp peer 10.0.0.1 hold-time 90                 (was: 180)
```

Grouped by session. Uses the raw session ID (from `%session` entries). Shows the `set`/`delete` command with a marker and the previous value (if modified or deleted).

### `show raw` -- File Content (informal, no AC)

Displays the draft file content as-is (metadata + commands). This is an informal debug command -- no acceptance criteria or functional test required. It reads the draft file and displays it verbatim without parsing.

### `save` -- Persist Draft

In session mode, `save` is a **no-op confirmation** -- write-through already persists every `set`/`delete` to the draft file immediately. The command prints a confirmation message but performs no disk I/O because the draft is already on disk. In non-session mode (legacy), `save` writes a `.edit` snapshot as before.

`save` has no effect on the running config (`config.conf`). Use `commit` to apply changes to the running config.

### `who` -- Active Sessions

```
who
```

Displays all sessions with pending changes, extracted from `%session` entries in the draft:

```
Active editing sessions:
* thomas@local:1741783680 - 5 pending changes
  alice@ssh:1741783860 - 1 pending change
  bob@ssh:1741782900 - 2 pending changes
```

The current session is marked with `*`. Session IDs include the unix timestamp (start time). This uses the raw session ID format from `%session` entries.

**Limitation: no liveness detection.** The editor cannot distinguish between active sessions (currently connected) and orphaned sessions (disconnected without committing). All sessions with `%session` entries in the draft are listed equally. Liveness detection would require a heartbeat mechanism or connection tracking in the SSH server -- deferred to future work. Users can `disconnect` sessions they know to be abandoned. Any user can run `who`.

### `disconnect` -- Force Remove Session

```
disconnect alice@ssh:1741783860
```

Removes all `%session` entries for the specified session from the draft. The session's pending changes are lost (committed values restored). Use cases: clean up abandoned sessions, break deadlocks when conflicting sessions are unresponsive.

**Authorization:** For this spec, `disconnect` is unrestricted -- any editor session can disconnect any other session. ~~RBAC gating (admin role required)~~ deferred to a future RBAC spec when ze gains a role/permission system. AC-29 and AC-30 are updated accordingly: AC-29 tests that `disconnect` works, AC-30 is removed (no RBAC to test yet).

## Migration from Hierarchical Text Format

### Auto-Detection

The parser auto-detects the file format by examining the first non-empty, non-comment line:

| First token | Format | Parser |
|-------------|--------|--------|
| `set` or `delete` | Flat set commands (new format) | SetParser |
| `#identifier` (no space after `#`) | Flat set commands with metadata | SetParser (strips metadata) |
| Any other word | Hierarchical text (current format) | Current Parser (unchanged) |
| Empty or comment-only file | Flat set commands (new format) | SetParser (empty Tree) |

**Edge case:** An empty file or a file containing only `# ` comments has no first data line. `DetectFormat` returns `FormatSet` (not `FormatHierarchical`) because new files should default to the new format, not trigger migration.

### Migration Path

**Two types of migration (distinct concerns):**

| Type | What | Where |
|------|------|-------|
| **Format conversion** | Hierarchical text to flat set commands | Serialization output choice |
| **Tree structure migration** | `neighbor` to `peer`, template renaming, etc. | `migration.Migrate()` pipeline |

Format conversion is a serialization concern (same Tree, different output format). Tree structure migration transforms the Tree itself (renaming keys, moving subtrees). Both should run when a hierarchical config is committed for the first time.

**Flow:**

1. User opens a hierarchical text config with `ze config edit`
2. The parser reads it with the current hierarchical parser into a Tree (format-agnostic)
3. The editor works normally (set/delete commands via write-through)
4. On first `set`, the draft is created in set+meta format (`writeThroughSet` serializes with `SerializeSetWithMeta`). The format conversion of the draft happens here, not at commit time.
5. On `commit`, `CommitSession` writes config.conf with `SerializeSetWithMeta`. If the original was hierarchical, `migration.Migrate()` runs on the tree first to apply any pending tree structure migrations (e.g., `neighbor` to `peer`).
6. The old hierarchical format is never written again (but can always be read)

**Key observation:** `CommitSession()` already writes set+meta format unconditionally. The remaining work is: (a) running tree structure migrations on first commit of hierarchical input, (b) making the non-session `Save()` path format-aware, (c) aligning `WorkingContent()` with the format actually written.

### `ze config migrate` Subcommand

Explicit migration command:

```
ze config migrate config.conf              # convert to set format (stdout)
ze config migrate config.conf -o new.conf  # convert to new file
ze config migrate --format hierarchical config.conf # explicit output format
```

**Default output is always set format** regardless of input format. This differs from the existing `cmd_migrate.go` behavior which preserves input format -- the existing code must be changed so that `ze config migrate` on a hierarchical file outputs set-format commands (not hierarchical text). The `--format` flag selects output format explicitly (`set` is default, `hierarchical` for backwards output).

The command runs both tree structure migrations (`migration.Migrate()`) and format conversion in one pass: read any format, apply tree transforms, serialize as set format.

**Current `cmd_migrate.go` bug:** Lines 215-222 check `sourceFormat` and output hierarchical for hierarchical input. This must change to always output set format by default.

### ExaBGP Migration

The existing ExaBGP migration path is unchanged: `ze bgp config migrate` converts ExaBGP syntax to ze-native. The output format changes from hierarchical text to flat set commands, but the migration logic (syntax transformation) is the same.

## SSH Integration

### Current State

SSH sessions use `NewCommandModel()` which creates a command-only model with no editor. SSH users can run operational commands but cannot edit configuration.

### Config Path Propagation

The SSH `Server` needs the config file path to create Editors. Add `ConfigPath string` to `ssh.Config`, set during daemon startup from the loaded config file path. This parallels `ConfigDir` (already on `ssh.Config`) but points to the specific file.

| Field | Source | Purpose |
|-------|--------|---------|
| `ConfigPath` | Set by daemon loader at startup | Passed to `cli.NewEditor()` for each SSH session |

### New State

SSH sessions receive an `Editor` connected to the same config file. The `createSessionModel` method changes from creating a `NewCommandModel()` to creating a full `NewModel(ed)` with editor support.

| Step | Action |
|------|--------|
| 1 | Call `cli.NewEditor(s.config.ConfigPath)` to create an Editor for the shared config file |
| 2 | If `NewEditor` fails, fall back to `cli.NewCommandModel()` (command-only, no editing) |
| 3 | Call `ed.SetSession(cli.NewEditSession(username, "ssh"))` to set SSH session identity |
| 4 | Call `ed.SetReloadNotifier(...)` to enable daemon reload on commit (same as terminal) |
| 5 | Call `ed.SetArchiveNotifier(...)` if commit-triggered archives are configured (same as terminal) |
| 6 | Create model with `cli.NewModel(ed)` -- starts in editor mode with mode-switching support |
| 7 | Call `m.SetCommandExecutor(executor)` using the existing executor factory |
| 8 | Call `m.SetCommandCompleter(...)` for tab completion of operational commands |

The SSH session's `Editor` has:
- Same config file path as the terminal editor (from `ssh.Config.ConfigPath`)
- Session identity: `username@ssh` (via `SetSession` post-construction, same pattern as `cmd_edit.go`)
- Same write-through protocol (shared lock file)
- Same draft file

### SSH Session Lifecycle

1. User connects via SSH, authenticates as `alice`
2. Server creates `Editor` with session `alice@ssh:1741783860`
3. Alice types `set bgp peer 10.0.0.1 hold-time 90`
4. Editor acquires lock, reads draft, applies change with `%alice@ssh:1741783860`, writes draft, releases lock
5. Terminal editor (if running) detects mtime change on next poll, shows notification
6. Alice types `commit` -- only her changes are committed
7. Alice disconnects -- if she has uncommitted changes, they remain in the draft with her `%session` tag

### Stale Sessions

If an SSH session disconnects without committing, its `%session` entries remain in the draft. This is intentional:
- The user can reconnect and resume (their changes are still there, adoption prompt on reconnect)
- Another user can see the pending changes with `show changes all` or `who`
- Any user can clean up with `disconnect alice@ssh:1741783860`

## MetaTree Data Structure

### In-Memory Representation

The `MetaTree` mirrors the `Tree` structure. For each leaf in `Tree` that has metadata, `MetaTree` has a `MetaEntry`.

**MetaEntry fields:**

| Field | Type | Source | Purpose |
|-------|------|--------|---------|
| `User` | string | `#user` prefix | Who made this change |
| `Time` | time.Time | `@timestamp` prefix | When the change was made |
| `Session` | string | `%session` prefix | Session ID (empty in committed config) |
| `Previous` | string | Computed on set | Value from config.conf when change was made (stale conflict detection; always read from committed config, never from draft) |
| `Value` | string | Computed on set | The value this entry set (for live conflict comparison) |

**MetaTree fields:**

| Field | Type | Purpose |
|-------|------|---------|
| `entries` | map of leaf name to list of MetaEntry | Per-leaf metadata (multiple entries for contested leaves) |
| `containers` | map of container name to MetaTree | Child containers (mirrors Tree containers) |
| `lists` | map of key to MetaTree | List entries (mirrors Tree list entries) |

`MetaTree` is populated during parsing (from the metadata prefixes) and used during serialization (to emit the prefixes). It is also used during commit for dual conflict detection: live disagreement (comparing values across active sessions) and stale Previous (comparing `Previous` against current `config.conf` value).

### MetaTree and YANG Migrations

When the YANG schema changes (e.g., leaf renamed, container restructured), the migration code transforms both `Tree` and `MetaTree` with the same operations:

| YANG Change | Tree Transform | MetaTree Transform |
|-------------|---------------|-------------------|
| Leaf renamed `a` to `b` | Copy value from key `a` to key `b`, remove key `a` | Copy entries from key `a` to key `b`, remove key `a` |
| Container moved | Move subtree | Move MetaTree subtree |
| Leaf deleted | Delete value | Delete MetaEntry |
| New leaf added | Nothing | Nothing |

The transforms are identical because both structures use the same YANG path as keys.

## Data Flow (MANDATORY)

### Entry Point
- **User command** in editor CLI: `set <path> <value>` or `delete <path>`
- **Format at entry:** tokenized command string

### Transformation Path

1. **Command parsing** (`model_commands.go`): tokenize, validate against YANG schema
2. **Write-through** (`editor.go`): acquire lock, read draft from disk, parse into Tree+MetaTree
3. **Tree mutation** (`tree.go`): `SetValue()` or `DeleteValue()` modifies Tree, update MetaTree
4. **Serialization** (`serialize_set.go`): `SerializeSetWithMeta(Tree, MetaTree, Schema)` emits flat set lines with metadata
5. **Atomic write** (`editor.go`): temp file + rename to `config.draft`
6. **Lock release** (`editor.go`): `funlock`
7. **Display update** (`model.go`): re-render viewport from updated Tree

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Editor -> Disk | Atomic write under flock | [ ] |
| Editor A -> Editor B | mtime polling on draft file | [ ] |
| Draft -> Committed | Per-session commit with dual conflict check (live + stale) | [ ] |
| SSH -> Editor | Editor created per SSH session, shared config path | [ ] |

### Integration Points
- `editor.SetValue()` - gains write-through behavior (existing signature, new implementation)
- `Serialize()` - unchanged, used for tree view
- `SerializeSet()` - new, used for set view and file writing
- `Parser.Parse()` - gains format auto-detection
- `ParseSet()` - new, parses flat set format into Tree+MetaTree
- `NewEditor()` - gains session identity parameter and lock file
- `createSessionModel()` in SSH server - gains Editor creation

### Architectural Verification
- [ ] No bypassed layers (write-through goes through same Editor.SetValue path)
- [ ] No unintended coupling (lock file is per-config-file, not global)
- [ ] No duplicated functionality (reuses existing Tree, Parser, Serialize)
- [ ] Zero-copy preserved where applicable (config files are small, not a hot path)

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config edit` + type `set` | -> | Write-through to draft file | `test/config/concurrent-write-through.ci` |
| `ze config edit` + type `commit` | -> | Per-session commit to config.conf (dual conflict check) | `test/config/concurrent-commit.ci` |
| `ze config edit` on flat-format file | -> | SetParser parses set commands | `test/parse/set-format.ci` |
| `ze config edit` on hierarchical file + `set` + `commit` | -> | Auto-detect + format conversion + tree structure migration | `test/parse/set-format-migration.ci` |
| Two editors + conflicting `commit` (live) | -> | Live disagreement conflict detection | `test/config/concurrent-conflict-live.ci` |
| Editor commits after another committed same path (stale) | -> | Stale Previous conflict detection | `test/config/concurrent-conflict-stale.ci` |
| `show blame` command | -> | Annotated tree view with gutter | `test/config/show-blame.ci` |
| `who` command | -> | List active sessions | `test/config/who.ci` |
| `disconnect` command | -> | Remove session entries | `test/config/disconnect.ci` |
| Same-user reconnect with orphaned session | -> | Adoption prompt | `test/config/session-adopt.ci` |
| SSH session connects + `set` + `commit` | -> | SSH editor with session identity, write-through, commit | `test/config/ssh-editor.ci` |
| Exit editor with pending changes | -> | Exit prompt (save/discard/cancel) | `test/config/exit-prompt.ci` |
| `ze config migrate` on hierarchical file | -> | Output is set format (default), tree migrations applied | `test/parse/set-format-migration-cmd.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config file with `set` commands (no metadata) | Parser produces same Tree as equivalent hierarchical config |
| AC-2 | Config file with `#user @timestamp set` commands | Parser produces Tree + MetaTree with correct entries |
| AC-3 | Config file with mixed metadata/no-metadata lines | Parser handles both, MetaTree has entries only for lines with metadata |
| AC-4 | `set` command in editor | Draft file written immediately (within lock), in-memory tree updated |
| AC-5 | Two editors open same config, editor A does `set` | Editor B detects mtime change, shows notification |
| AC-6 | `commit` with no conflicts | Only current session's changes applied to config.conf, other sessions' changes preserved in draft |
| AC-7 | `commit` when another active session has different value at same path | Live disagreement conflict reported, entire commit blocked |
| AC-8 | `commit` when committed value changed at same path since my `set` | Stale Previous conflict reported, entire commit blocked |
| AC-9 | Two sessions set same value at same path | No conflict, first to commit wins |
| AC-10 | `discard <path>` | That leaf restored to committed value (or removed if newly added), other sessions unaffected |
| AC-11 | `discard all` | All my pending changes removed, other sessions unaffected |
| AC-12 | `discard` (bare, no arguments) | Rejected with error message |
| AC-13 | `discard <container-path>` | All my modified leaves under that subtree restored |
| AC-14 | `show` | Displays tree view (hierarchical, indented) without metadata |
| AC-15 | `show set` | Displays flat set commands without metadata |
| AC-16 | `show blame` | Displays tree view with left gutter (user, date, time, marker), fixed-width columns |
| AC-17 | `show changes` | Displays current session's pending changes (not all sessions) |
| AC-18 | `show changes all` | Displays pending changes grouped by all sessions |
| AC-19 | All sessions committed/discarded | Draft file deleted |
| AC-20 | Hierarchical text config opened | Auto-detected, parsed, editor works normally |
| AC-21 | First commit of hierarchical config | config.conf written in set+meta format. Tree structure migrations applied if needed. Draft already in set format from first `set`. |
| AC-22 | SSH session connects | Gets editor with session identity, can set/commit |
| AC-23 | `# comment` lines in config | Preserved through read/write cycle |
| AC-24 | `save` command in session mode | No-op confirmation (draft already on disk via write-through), no effect on running config |
| AC-25 | Lock contention (two writes at same instant) | Second writer blocks briefly on flock, then succeeds |
| AC-26 | Editor starts with existing draft | No interactive prompt, loads draft automatically, displays other sessions |
| AC-27 | Same-user reconnect with orphaned session | Prompted to adopt previous session's changes |
| AC-28 | `who` command | Lists all active/orphaned sessions with change counts |
| AC-29 | `disconnect <session>` | Session's entries removed from draft, committed values restored |
| ~~AC-30~~ | ~~`disconnect <session>` without admin role~~ | ~~Rejected (RBAC)~~ Deferred: ze has no RBAC system yet. Will be added when a role/permission spec exists. |
| AC-31 | Exit with pending changes | Prompted to save or discard all |
| AC-32 | Re-`set` a path after stale conflict | Previous updated to current config.conf value, next commit succeeds |
| AC-33 | Empty or comment-only config file opened | Detected as set format (not hierarchical), no migration triggered |
| AC-34 | `ze config migrate` on hierarchical file (no `--format` flag) | Output is set format (not hierarchical). Tree structure migrations also applied. |
| AC-35 | `WorkingContent()` called when session is active | Returns set-format serialization consistent with what `CommitSession()` writes, so validation operates on the same format as the commit output |
| AC-36 | Non-session `commit` on hierarchical config | `Save()` rejects when session is active. Non-session `Save()` only for raw-text fallback. |
| AC-37 | `commit confirmed` in session mode | ~~Routes through `CommitSession()` (not `Save()`), writes set+meta format~~ Currently rejected with error ("not yet supported in session mode"). Full timer/auto-rollback session support deferred to Phase 7 Item 4. |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| ~~`TestParseSetFormat`~~ `TestSetParserSimpleLeaf`, `TestSetParserNeighborLeaf`, `TestSetParserMultipleNeighbors`, `TestSetParserNestedContainer`, `TestSetParserNestedList`, `TestSetParserProcess` | `internal/component/config/setparser_test.go` | Flat set commands parsed into Tree (split into per-type tests) | |
| ~~`TestParseSetFormatWithMeta`~~ `TestParseSetWithMetaSimple`, `TestParseSetWithMetaNested` | `internal/component/config/setparser_test.go` | Metadata prefixes parsed into MetaTree | |
| ~~`TestParseSetFormatMixed`~~ `TestParseSetWithMetaMixed` | `internal/component/config/setparser_test.go` | Mixed lines (with/without metadata) | |
| ~~`TestParseSetFormatComments`~~ `TestSetParserComments`, `TestParseSetWithMetaComments` | `internal/component/config/setparser_test.go` | Comments preserved, not confused with user metadata | |
| ~~`TestParseSetFormatEmpty`~~ `TestSetParserEmptyLines` | `internal/component/config/setparser_test.go` | Empty/blank lines handled correctly | |
| ~~`TestParseSetFormatDelete`~~ `TestSetParserDelete` | `internal/component/config/setparser_test.go` | Delete lines parsed and recorded | |
| ~~`TestSerializeSet`~~ `TestSerializeSetSimpleLeaf`, `TestSerializeSetNeighborLeaf`, `TestSerializeSetNestedContainer`, `TestSerializeSetMultipleNeighbors`, `TestSerializeSetNestedList`, `TestSerializeSetEmptyTree` | `internal/component/config/serialize_set_test.go` | Tree serialized to flat set commands (split into per-type tests) | |
| ~~`TestSerializeSetWithMeta`~~ `TestSerializeSetWithMeta`, `TestSerializeSetWithMetaNested`, `TestSerializeSetWithMetaMixed` | `internal/component/config/serialize_set_test.go` | Tree + MetaTree serialized with prefixes | |
| `TestSerializeSetRoundTrip`, `TestSerializeSetCrossFormatRoundTrip` | `internal/component/config/serialize_set_test.go` | Parse -> Serialize -> Parse produces same Tree (including cross-format hierarchical->set) | |
| `TestSerializeSetSchemaOrder` | `internal/component/config/serialize_set_test.go` | Output follows YANG schema order | |
| `TestSerializeBlame` | `internal/component/config/serialize_set_test.go` | Blame view with fixed-width gutter | |
| `TestFormatAutoDetect` | `internal/component/config/parser_test.go` | First-line detection: set vs hierarchical | |
| `TestMetaTreeSetGet` | `internal/component/config/meta_test.go` | MetaTree stores/retrieves entries by path | |
| `TestMetaTreeSessionFilter` | `internal/component/config/meta_test.go` | Filter entries by session ID | |
| `TestMetaTreeRemoveSession` | `internal/component/config/meta_test.go` | Remove all entries for a session | |
| `TestEditorWriteThrough` | `internal/component/cli/editor_test.go` | SetValue writes draft file under lock | |
| `TestEditorConcurrentWrite` | `internal/component/cli/editor_test.go` | Two editors write without corruption | |
| `TestEditorSessionCommit` | `internal/component/cli/editor_test.go` | Commit applies only my session | |
| `TestEditorConflictLiveDisagreement` | `internal/component/cli/editor_test.go` | Conflict when active session has different value at same path | |
| `TestEditorConflictStalePrevious` | `internal/component/cli/editor_test.go` | Conflict when committed value changed since my set | |
| `TestEditorConflictAgreement` | `internal/component/cli/editor_test.go` | No conflict when both sessions set same value | |
| `TestEditorConflictBlocksEntireCommit` | `internal/component/cli/editor_test.go` | One conflict blocks all changes, not just conflicting key | |
| `TestEditorConflictResetAfterSet` | `internal/component/cli/editor_test.go` | Re-set after stale conflict updates Previous to config.conf value | |
| `TestEditorDiscardPath` | `internal/component/cli/editor_test.go` | Discard specific path restores committed value | |
| `TestEditorDiscardSubtree` | `internal/component/cli/editor_test.go` | Discard container path restores all modified leaves under it | |
| `TestEditorDiscardAll` | `internal/component/cli/editor_test.go` | Discard all removes all my changes | |
| `TestEditorDiscardBareRejected` | `internal/component/cli/editor_test.go` | Bare discard (no args) is rejected | |
| `TestEditorDiscardNewlyAdded` | `internal/component/cli/editor_test.go` | Discard of newly added leaf removes it entirely | |
| `TestEditorDraftCleanup` | `internal/component/cli/editor_test.go` | Draft deleted when all sessions done | |
| `TestEditorMtimeDetection` | `internal/component/cli/editor_test.go` | mtime change triggers re-read | |
| `TestEditorAdoptOrphanedSession` | `internal/component/cli/editor_test.go` | Same-user reconnect adopts old session entries | |
| `TestEditorAdoptDeclined` | `internal/component/cli/editor_test.go` | Declining adoption leaves orphaned entries | |
| `TestEditorWho` | `internal/component/cli/editor_test.go` | Who lists all active/orphaned sessions | |
| `TestEditorDisconnect` | `internal/component/cli/editor_test.go` | Disconnect removes target session entries | |
| `TestEditorExitPrompt` | `internal/component/cli/editor_test.go` | Exit with pending changes prompts save/discard | |
| `TestEditorSave` | `internal/component/cli/editor_test.go` | Save persists draft, does not affect config.conf | |
| `TestHierarchicalToSetMigration` | `internal/component/cli/editor_test.go` | Hierarchical config opened, first `CommitSession` writes set+meta format to config.conf with tree structure migrations applied | |
| `TestWorkingContentSessionFormat` | `internal/component/cli/editor_test.go` | `WorkingContent()` returns set format when session is active, hierarchical when no session | |
| `TestSaveGuardInSessionMode` | `internal/component/cli/editor_test.go` | `Save()` rejects (returns error) when session is active, preventing accidental hierarchical overwrite | |
| `TestDetectFormatEmptyFile` | `internal/component/config/serialize_set_test.go` | Empty file and comment-only file detected as `FormatSet`, not `FormatHierarchical` | |
| `TestMigrateDefaultOutputSet` | `cmd/ze/config/cmd_migrate_test.go` | `ze config migrate` on hierarchical input produces set format by default | |
| `TestBlameGutterWidth` | `internal/component/cli/model_commands_test.go` | Gutter columns have fixed width | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Gutter user column | 1-14 chars | 14 char username | N/A (short names padded) | Truncated at 14 |
| Timestamp | ISO 8601 | Any valid ISO 8601 | Malformed string (warn, continue) | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-set-format-parse` | `test/parse/set-format.ci` | Config in set format parsed, ze starts | |
| `test-set-format-meta` | `test/parse/set-format-meta.ci` | Config with metadata parsed, ze starts | |
| `test-set-format-migration` | `test/parse/set-format-migration.ci` | Hierarchical config auto-detected, migrated on commit | |
| `test-set-format-migration-cmd` | `test/parse/set-format-migration-cmd.ci` | `ze config migrate` on hierarchical file outputs set format | |
| `test-concurrent-write` | `test/config/concurrent-write-through.ci` | `set` writes to draft immediately | |
| `test-concurrent-commit` | `test/config/concurrent-commit.ci` | `commit` applies only my session | |
| `test-concurrent-conflict-live` | `test/config/concurrent-conflict-live.ci` | Live disagreement conflict detected and reported | |
| `test-concurrent-conflict-stale` | `test/config/concurrent-conflict-stale.ci` | Stale Previous conflict detected and reported | |
| `test-show-blame` | `test/config/show-blame.ci` | `show blame` displays annotated tree | |
| `test-who` | `test/config/who.ci` | `who` lists active sessions | |
| `test-disconnect` | `test/config/disconnect.ci` | `disconnect` removes session entries | |
| `test-session-adopt` | `test/config/session-adopt.ci` | Same-user reconnect adoption prompt | |
| `test-ssh-editor` | `test/config/ssh-editor.ci` | SSH session gets editor, can set/commit | |
| `test-exit-prompt` | `test/config/exit-prompt.ci` | Exit with pending changes shows prompt | |
| `test-discard-path` | `test/config/discard-path.ci` | `discard <path>` restores committed value | |

## Files to Modify

- `internal/component/config/tree.go` - no structural change, Tree remains as-is
- `internal/component/config/parser.go` - unchanged (auto-detection is in `serialize_set.go` via `DetectFormat`, called from `editor_draft.go`'s `parseConfigWithFormat`)
- `internal/component/config/serialize.go` - unchanged (tree view generation stays)
- `internal/component/cli/editor.go` - major rewrite: session identity, lock file, write-through, draft management, adoption
- `internal/component/cli/model_commands.go` - update cmdSet/cmdDelete for write-through return, add cmdShowBlame, cmdShowChanges, cmdShowSet, cmdSave, cmdWho, cmdDisconnect; update cmdDiscard to require path or `all`
- `internal/component/cli/model.go` - mtime polling for draft changes, notification display, exit prompt intercept in `Update` for quit key messages
- `cmd/ze/config/cmd_edit.go` - remove legacy `.edit` fallback (PromptPendingEdit), add adoption prompt for same-user orphaned sessions (session identity and auto-load already implemented)
- `internal/component/ssh/ssh.go` - add `ConfigPath` to `Config` struct
- `internal/component/ssh/session.go` - create Editor for SSH sessions with username identity, full wiring (session, reload, archive, command executor)

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | No new RPCs, editor commands are local |
| RPC count in architecture docs | [ ] | N/A |
| CLI commands/flags | [x] | `cmd/ze/config/cmd_edit.go` (remove prompt), `cmd/ze/config/cmd_migrate.go` (default output to set format, add `--format` flag for explicit format choice) |
| CLI usage/help text | [x] | Update `show` subcommands help |
| API commands doc | [ ] | N/A |
| Plugin SDK docs | [ ] | N/A |
| Editor autocomplete | [x] | Add completions for `show blame`, `show changes`, `show changes all`, `show set`, `save`, `who`, `disconnect`, `discard all` |
| SSH config path wiring | [x] | `internal/component/bgp/config/loader.go` (pass config path to `ssh.Config.ConfigPath`) |
| Functional test for new RPC/API | [x] | `test/parse/set-format*.ci`, `test/config/concurrent*.ci`, `test/config/ssh-editor.ci` |

## Files to Create

- `internal/component/config/setparser.go` - SetParser: parse flat set commands into Tree + MetaTree
- `internal/component/config/setparser_test.go` - unit tests for SetParser
- `internal/component/config/serialize_set.go` - SerializeSet, SerializeSetWithMeta, DetectFormat, metadata-aware serialization
- `internal/component/config/serialize_blame.go` - SerializeBlame, blame gutter formatting (extracted from serialize_set.go)
- `internal/component/config/serialize_set_test.go` - unit tests for set serializers
- `internal/component/config/meta.go` - MetaEntry, MetaTree, session operations
- `internal/component/config/meta_test.go` - unit tests for MetaTree
- `internal/component/cli/editor_lock.go` - file locking helpers (acquireLock, releaseLock)
- `internal/component/cli/editor_session.go` - session identity, draft management
- `test/parse/set-format.ci` - functional test: set-format config
- `test/parse/set-format-meta.ci` - functional test: set-format with metadata
- `test/parse/set-format-migration.ci` - functional test: migration from hierarchical
- `test/config/concurrent-write-through.ci` - functional test: write-through
- `test/config/concurrent-commit.ci` - functional test: per-session commit
- `test/config/concurrent-conflict-live.ci` - functional test: live disagreement conflict
- `test/config/concurrent-conflict-stale.ci` - functional test: stale Previous conflict
- `test/config/show-blame.ci` - functional test: blame view
- `test/config/who.ci` - functional test: who command
- `test/config/disconnect.ci` - functional test: disconnect command
- `test/config/session-adopt.ci` - functional test: same-user adoption
- `test/config/ssh-editor.ci` - functional test: SSH session gets editor with set/commit
- `test/config/exit-prompt.ci` - functional test: exit with pending changes shows prompt
- `test/config/discard-path.ci` - functional test: discard with path
- `test/parse/set-format-migration-cmd.ci` - functional test: `ze config migrate` outputs set format

## Implementation Steps

This is a large spec. Implementation should proceed in phases, each independently testable.

### Phase 1: Set Format Parser and Serializer

Parse flat set commands into Tree. Serialize Tree to flat set commands. Round-trip test. No metadata yet, no write-through, no concurrency.

1. **Write unit tests** for `ParseSet()` and `SerializeSet()` -> Review: covers all YANG node types?
2. **Run tests** -> Verify FAIL
3. **Implement** `setparser.go`: line-by-line parser that tokenizes `set` commands and builds Tree using existing `walkOrCreate` + `SetValue`
4. **Implement** `serialize_set.go`: walk Tree in schema order, emit `set <path> <value>` per leaf
5. **Run tests** -> Verify PASS
6. **Add round-trip test:** parse hierarchical -> serialize to set -> parse set -> serialize to set -> compare
7. **Add format auto-detection** in `parser.go`
8. **Functional tests:** `test/parse/set-format.ci`

### Phase 2: Metadata Parsing and Serialization

Add metadata prefix handling. MetaTree. Blame view.

1. **Write unit tests** for `ParseSetWithMeta()`, `SerializeSetWithMeta()`, `SerializeBlame()`
2. **Run tests** -> Verify FAIL
3. **Implement** `meta.go`: MetaEntry, MetaTree
4. **Implement** metadata prefix parsing in `setparser.go`
5. **Implement** metadata prefix serialization in `serialize_set.go`
6. **Implement** blame view serialization with fixed-width gutter
7. **Run tests** -> Verify PASS
8. **Functional tests:** `test/parse/set-format-meta.ci`

### Phase 3: Write-Through and Locking

Editor writes to disk on every set/delete. File locking.

1. **Write unit tests** for `EditorWriteThrough`, `EditorConcurrentWrite`
2. **Run tests** -> Verify FAIL
3. **Implement** `editor_lock.go`: flock helpers
4. **Implement** `editor_session.go`: session identity, draft path
5. **Modify** `editor.go`: `SetValue` becomes write-through (lock, read, apply, write, unlock)
6. **Run tests** -> Verify PASS

### Phase 4: Per-Session Commit and Conflict Detection

Commit applies only current session. Dual conflict detection: live disagreement + stale Previous. Any conflict blocks entire commit.

1. **Write unit tests** for `EditorSessionCommit`, `EditorConflictLiveDisagreement`, `EditorConflictStalePrevious`, `EditorConflictAgreement`, `EditorConflictBlocksEntireCommit`, `EditorConflictResetAfterSet`
2. **Run tests** -> Verify FAIL
3. **Modify** `editor.go`: new `CommitSession()` method with dual conflict check (live disagreement + stale Previous from config.conf). Previous always read from `config.conf`.
4. **Write unit tests** for `EditorDiscardPath`, `EditorDiscardSubtree`, `EditorDiscardAll`, `EditorDiscardBareRejected`, `EditorDiscardNewlyAdded`
5. **Run tests** -> Verify FAIL
6. **Modify** `model_commands.go`: `cmdCommit` uses `CommitSession()`, `cmdDiscard` requires path or `all`
7. **Run tests** -> Verify PASS
8. **Functional tests:** `test/config/concurrent-commit.ci`, `test/config/concurrent-conflict-live.ci`, `test/config/concurrent-conflict-stale.ci`, `test/config/discard-path.ci`

### Phase 5: Display Views, Session Management, and Commands

Add show blame, show changes (mine/all), show set, save, who, disconnect commands.

1. **Write unit tests** for blame gutter formatting, changes grouping (mine default), who listing, disconnect, save, exit prompt
2. **Run tests** -> Verify FAIL
3. **Implement** view commands in `model_commands.go`: `cmdShowBlame`, `cmdShowChanges` (mine default, `all` subcommand), `cmdShowSet`, `cmdSave`, `cmdWho`, `cmdDisconnect`
4. **Add completions** for new commands
5. **Run tests** -> Verify PASS
6. **Functional tests:** `test/config/show-blame.ci`, `test/config/who.ci`, `test/config/disconnect.ci`

### Phase 6: SSH Integration, Startup Flow, and Session Adoption

SSH sessions get editors. Remove legacy startup prompt. Add same-user adoption. Add exit prompt. Add mtime polling for draft changes.

**Note:** `cmd_edit.go` already has session creation (`NewEditSession`), `SetSession`, draft auto-load, and active session display (implemented in earlier phases). Phase 6 completes the remaining wiring.

1. **Write unit tests** for `Editor.AdoptSession` (rewrite `%session` entries), exit prompt logic (pending changes detection, quit intercept), mtime draft polling (AC-5)
2. **Run tests** -> Verify FAIL
3. **Implement** `Editor.AdoptSession(oldSessionID string) error` in `editor_draft.go`: acquire lock, read draft, rewrite matching `%session` entries to current session ID, serialize, write draft, release lock
4. **Modify** `cmd/ze/config/cmd_edit.go`: remove legacy `.edit` fallback (lines 330-346 with `PromptPendingEdit`), add adoption prompt for same-user orphaned sessions (check `ActiveSessions` for matching username with different session ID, call `AdoptSession` on "yes")
5. **Modify** `internal/component/ssh/ssh.go`: add `ConfigPath string` field to `Config` struct
6. **Modify** `internal/component/ssh/session.go`: rewrite `createSessionModel` to create `cli.NewEditor(s.config.ConfigPath)` + `ed.SetSession(cli.NewEditSession(username, "ssh"))` + `ed.SetReloadNotifier(...)` + `ed.SetArchiveNotifier(...)` + `cli.NewModel(ed)` + `m.SetCommandExecutor(executor)` + `m.SetCommandCompleter(...)`. Fall back to `cli.NewCommandModel()` if `NewEditor` fails.
7. **Modify** `internal/component/bgp/config/loader.go`: pass config file path to `ssh.Config.ConfigPath` when creating the SSH server
8. **Modify** `model.go`: intercept quit key messages (`tea.KeyCtrlC`, `exit`, `quit`) in `Update`, check for pending session entries, display save/discard/cancel prompt, handle response
9. **Modify** `model.go`: add `draftMtime` field + `draftPollMsg` tick (every 2s) + `checkDraftMtime()` handler that stats draft file, compares mtime, re-reads and re-parses if changed, shows notification of other sessions' changes (AC-5)
10. **Run tests** -> Verify PASS
11. **Functional tests:** `test/config/session-adopt.ci`, `test/config/ssh-editor.ci`, `test/config/exit-prompt.ci`

### Phase 7: Format Migration

~~Hierarchical text auto-migration on first commit.~~
~~1. Write unit tests for `TestHierarchicalToSetMigration` in `internal/component/config/migrate_test.go`~~
~~2. Run tests -> Verify FAIL~~
~~3. Implement migration in `editor.go`: detect hierarchical format, on commit serialize to set format~~
~~4. Run tests -> Verify PASS~~
~~5. Update `cmd/ze/config/cmd_migrate.go` for explicit `ze config migrate` command~~
~~6. Functional tests: `test/parse/set-format-migration.ci`~~
*(Superseded after critical review: separated format conversion from tree migration, fixed non-session commit path, added validation format alignment, added empty-file edge case.)*

**Already done by existing code:**
- `CommitSession()` unconditionally writes config.conf with `SerializeSetWithMeta()` -- format conversion is implicit
- `parseConfigWithFormat()` auto-detects hierarchical input and parses correctly
- First `writeThroughSet()` creates a set-format draft (Tree is format-agnostic)

**Remaining work (2 items):**

~~1. **Tree structure migration on first commit**: Done. `editor_draft.go:571` calls `migration.NeedsMigration()` on committed tree.~~

~~2. **`WorkingContent()` format-awareness**: Done. `editor.go:300-303` returns `SerializeSetWithMeta()` when session is active.~~

~~3. **`Save()` format-awareness**: Done. `editor.go:849-852` rejects when session is active.~~

4. **`commit confirmed` session routing** (`model_load.go`): ~~Route through `CommitSession()` when session is active instead of calling `Save()` directly.~~ Currently rejected with error in session mode (see Deviations). Full timer/auto-rollback session support deferred. AC-37.

~~5. **`DetectFormat` empty-file edge case**: Done. `serialize_set.go:66-68` returns `FormatSet` for empty/comment-only files.~~

~~6. **`cmd_migrate.go` default output format**: Done. `cmd_migrate.go` has `--format` flag with set format as default.~~

7. **Functional tests:** `test/parse/set-format-migration.ci` -- not yet created.

~~**TDD cycle:**~~ *(Items 1-3, 5-6 already implemented and tested. Remaining: Item 4 deferred, Item 7 functional test.)*

**Remaining TDD:**

1. **Functional test:** `test/parse/set-format-migration.ci` -- open hierarchical config, `set` a value, `commit`, verify config.conf is set format

### Failure Routing

| Failure | Route To |
|---------|----------|
| Set format parse produces wrong tree | Phase 1 Step 3 (check YANG path derivation) |
| Metadata parsing confuses comment with user | Phase 2 Step 4 (check `# ` vs `#user` rule) |
| Concurrent write corrupts file | Phase 3 Step 5 (check lock acquisition/release) |
| Live conflict not detected | Phase 4 Step 3 (check session comparison in draft) |
| Stale conflict not detected | Phase 4 Step 3 (check Previous from config.conf, not draft) |
| Re-set doesn't clear stale conflict | Phase 4 Step 3 (verify Previous updated to config.conf value on re-set) |
| Blame gutter misaligned | Phase 5 Step 3 (check fixed-width formatting) |
| SSH session can't write | Phase 6 Step 5-6 (check ConfigPath on ssh.Config, check createSessionModel wiring) |
| Adoption moves wrong entries | Phase 6 Step 3 (check session ID matching in AdoptSession) |
| Exit prompt not shown | Phase 6 Step 8 (check quit key intercept in model.go Update) |
| Hierarchical config committed without tree structure migration | Phase 7 Item 1 (check `NeedsMigration()` call in `CommitSession`) |
| Validation passes but commit writes different content | Phase 7 Item 2 (check `WorkingContent()` returns set format in session mode) |
| Non-session commit writes hierarchical format | Phase 7 Item 3 (check `Save()` guard or format-awareness) |
| `commit confirmed` in session mode writes hierarchical | Phase 7 Item 4 (check `model_load.go` session routing) |
| Empty config triggers migration | Phase 7 Item 5 (check `DetectFormat` empty-file case) |
| `ze config migrate` on hierarchical outputs hierarchical | Phase 7 Item 6 (check `cmd_migrate.go` default output format) |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## RFC Documentation

N/A -- this is an internal config format change, not a protocol change.

## Implementation Summary

### What Was Implemented

### Bugs Found/Fixed

### Documentation Updates

### Deviations from Plan

- **Phase boundary:** Phases 3-5 were implemented together in `editor_draft.go` rather than as separate commits. Write-through, commit, discard, and disconnect all share the same file. The TDD cycle should verify Phase 3 tests fail/pass before Phase 4 code is added.
- **`^previous` sigil:** Added to the metadata grammar to serialize the Previous field to draft files. Not in the original spec grammar (now documented above).
- **`readCommittedValue` replaced with `readCommittedTree` + `getValueAtPath`:** The original 90-line function with its own tree navigation was replaced to reuse existing schema-aware navigation and eliminate duplicated code.
- **`walkOrCreate` and `walkOrCreateIn` aligned on `InlineListNode`:** Both now handle inline lists with key navigation (anonymous and keyed). Previously `walkOrCreate` treated inline lists as leaf errors while `walkOrCreateIn` navigated them.
- **RBAC for `disconnect` deferred:** Original spec had `disconnect` gated by admin role (AC-30). Ze has no RBAC system, so `disconnect` is unrestricted for this spec. AC-30 struck through. RBAC will be added when a role/permission spec is created.
- **Anonymous list support in `walkOrCreateIn`:** Added `KeyDefault` logic matching `walkOrCreate` so anonymous lists work correctly in write-through paths.
- **Discard path boundary matching:** `DiscardSessionPath` now uses word-boundary matching (`se.Path == pathPrefix || HasPrefix(se.Path, pathPrefix+" ")`) instead of raw prefix to prevent "bgp peer" from matching "bgp peer-group".
- **Phase 7 rewritten after critical review:** Original Phase 7 was 3 steps (implement migration, update cmd_migrate, functional tests). Critical review found: (a) `CommitSession()` already does format conversion implicitly, (b) format migration and tree structure migration are distinct concerns that were conflated, (c) non-session `Save()` path still writes hierarchical, (d) `WorkingContent()` format doesn't match commit output, (e) `DetectFormat` mishandles empty files, (f) `cmd_migrate.go` preserves input format instead of defaulting to set format, (g) `commit confirmed` path bypasses `CommitSession()`. Phase 7 expanded to 7 items + 5 new ACs (AC-33 through AC-37).
- **Known limitation: commit validation scope.** Pre-commit validation in `cmdCommitSession` checks the full draft tree (all sessions combined), but commit only applies this session's changes. If two sessions' changes are individually invalid but valid together, validation passes but the committed result may be invalid. Acceptable because single-user editing is the common case, and full draft validation catches most errors.
- **`delete` command in set format:** The serializer emits `delete <path>` lines with metadata for keys that have metadata entries but no tree value. The parser recognizes `delete` lines and records metadata via `walkAndRecordDeleteMeta`. This enables Previous to survive the serialize/parse round-trip for deleted keys, making stale conflict detection work symmetrically for both set and delete operations.
- **`editor_draft.go` at 958 lines:** Exceeds the 600-line review threshold. Contains write-through, commit, discard, disconnect, and tree/meta walking utilities. Candidate for splitting at completion (file-modularity check in Completion Checklist step 3). Natural split: commit/discard/disconnect into `editor_commit.go`, tree/meta walking utilities into `editor_walk.go`.
- **Filename deviation: `setparser.go` instead of `parse_set.go`:** Original spec planned `parse_set.go`; implemented as `setparser.go` to follow the naming pattern of existing parsers in the `config` package (e.g., `parser.go`). Tests similarly: `setparser_test.go` instead of `parse_set_test.go`.
- **`DetectFormat` location:** Spec originally placed format auto-detection in `parser.go`. Implemented in `serialize_set.go` alongside the `ConfigFormat` constants. Called from `editor_draft.go`'s `parseConfigWithFormat`.
- **Mtime polling not yet implemented:** AC-5 (other editors detect changes) requires draft file mtime polling in `model.go`. Design specified in Write-Through Protocol section. Assigned to Phase 6.
- **Stale conflict for newly-added values:** Original code only checked stale conflict when `Previous != ""`. This missed the case where session A adds a new value (Previous=""), then session B commits a value at the same path -- session A's commit should detect the stale conflict. Fixed: also check for `committedValue != ""` when `Previous == ""`.
- **`disconnect` IsAdmin guard removed:** Code had an `IsAdmin` check on `cmdDisconnectSession` despite spec saying `disconnect` is unrestricted. Guard removed to match spec. `IsAdmin` field and `IsAdmin()` method also removed from `EditSession` (YAGNI -- no current RBAC requirement).
- **`save` in session mode is a no-op:** Code was calling `SaveEditState()` (writes `.edit` file) even in session mode where write-through already persists to `.draft`. Fixed: session mode returns confirmation without I/O.
- **`show changes` delete rendering:** Code rendered delete entries as `+ set <path>  (new)` because delete MetaEntries have empty Value. Fixed: entries with empty Value and non-empty Previous render as `- delete <path>  (was: <prev>)`.
- **Exit prompt session awareness:** Code checked `editor.Dirty()` (in-memory flag) instead of session entries for pending changes. In write-through mode, `Dirty()` is unreliable. Fixed: session mode checks `HasPendingSessionChanges()` (meta.SessionEntries count). Prompt text updated to match session semantics.
- **Phase counter corrections:** Originally set to 5/7, corrected to 4/7 (Phase 5 `.ci` tests missing), then to 6/7 (Phase 7 Items 1-3, 5-6 already implemented in code; only Item 4 deferred and Item 7 functional tests remain).
- **Functional tests deferred to Phase 5 completion:** Phases 1-2 spec listed `.ci` functional tests (`set-format.ci`, `set-format-meta.ci`) as deliverables but none were created. All `.ci` tests consolidated to Phase 5+ delivery.
- **`show changes` grammar fix:** "No my pending changes." corrected to "No pending changes."
- **`commit confirmed` rejected in session mode:** `commit confirmed <N>` in session mode was silently routed to `cmdCommitSession()`, ignoring the timer/auto-rollback semantics entirely. Fixed: session mode rejects `commit confirmed` with an explicit error, directing the user to use plain `commit`. Full `commit confirmed` session support (AC-37) deferred to Phase 7 Item 4.
- **Ctrl-C/Esc quit path now session-aware:** The Ctrl-C/Esc handler in `handleKeyMsg` went straight to `confirmQuit` with a generic "Quit?" message, bypassing the pending-changes check. Fixed: both the Ctrl-C/Esc path and the exit/quit command path now use a shared `hasPendingChanges()` helper that checks `HasPendingSessionChanges()` in session mode and `Dirty()` otherwise.
- **`autoSaveOnQuit` skipped in session mode:** Force-quit auto-save was writing a `.edit` snapshot even when write-through already persists to `.draft`. Fixed: `autoSaveOnQuit()` now skips `SaveEditState()` when a session is active, since the draft is already on disk.
- **`show set` available without session:** `show set` (flat set-format view) was gated behind `HasSession()` along with `show blame` and `show changes`. Since `show set` is a pure format conversion with no metadata dependency, it now works without an active session. `show blame` and `show changes` still require a session (they depend on MetaTree data).
- **`discard` completion offers `all`:** Added `completeDiscardPath` to the completer dispatch, offering `all` alongside YANG path completions when typing `discard `.
- **Conflict display format is compact:** Spec shows a multi-line format with "Your value:", "Committed value:", etc. Implementation uses a single-line format (`LIVE path: you=val, other=val`). Same information, more compact. Cosmetic deviation.
- **`show blame`/`show changes` error without session:** Previously fell through silently when no session was active. Now returns explicit error ("show blame requires an active editing session").
- **`who`/`disconnect` guarded and filtered without session:** Both commands return explicit errors when no editing session is active. Completion filtering extended from `blame`/`changes` to also hide `who` and `disconnect` when no session is active.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |

### Tests from TDD Plan
| Test | Status | Location | Notes |

### Files from Plan
| File | Status | Notes |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-37 all demonstrated (AC-30 struck through, AC-33 through AC-37 added by Phase 7 critical review)
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
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
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `docs/learned/NNN-<name>.md`
- [ ] Summary included in commit

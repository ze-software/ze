# Change Files: structural operations

Renaming a keyed list entry is a first-class structural operation in a per-user
change file. It is not decomposed into leaf deletes and creates.

<!-- source: internal/component/config/change_file.go -- StructuralOp, change-file parse and serialize -->

## The shape

A rename line carries the same metadata prefix as a leaf edit:

```
#user @source %time rename <parent> <list> <old> to <new>
```

`SaveDraft()` applies structural operations first, then replays the leaf edits,
then writes a materialized draft. A rename chain inside one session is coalesced:
`old -> mid -> new` becomes `old -> new`.

The same rule covers the session editor's other structural verbs. Each is one
line, one pending change, and is applied by `SaveDraft()` and by the commit
before the leaf edits:

| Line | Recorded by | Pending change |
|------|-------------|----------------|
| `copy-entry <parent> <list> <source> to <target>` | `copy` | `copy <source path> to <target path>` |
| `deactivate-leaf <parent> <leaf>` | `deactivate` on a leaf | `deactivate <path>` |
| `activate-leaf <parent> <leaf>` | `activate` on a leaf | `activate <path>` |
| `deactivate-path <path>` | `deactivate` on a container or list entry | `deactivate <path>` |
| `activate-path <path>` | `activate` on a container or list entry | `activate <path>` |

A copy and a rename conflict with any change whose path overlaps the source or
the target. A deactivate and an activate of the same node by two sessions
conflict, because the kinds differ even though neither carries a value. A
toggle that finds its node already in the asked state applies as a no-op, so a
replayed draft does not fail.
<!-- source: internal/component/cli/editor_draft.go -- applyStructuralOps, applyToggleOp, pendingChangesConflict -->

Because the ops run before the leaf edits, a copy reads the committed source,
not the session's edited one. A copy therefore also writes the source's pending
leaf edits, sets and deletes, under the target key, as a rename moves them to
the new key. A copied edit carries no previous value: the committed config
holds nothing at the target, so keeping the source's previous value would read
as stale at commit.
<!-- source: internal/component/cli/editor_draft.go -- writeThroughCopy, copyPendingListEntry -->
<!-- source: internal/component/config/meta.go -- MetaTree.CopyListEntry -->

A leaf delete line creates no tree node, and the serializer reaches metadata
only through the tree. Parsing a change file therefore creates the empty
containers and list entries that lead to each metadata entry, so the next
write-through keeps a pending delete rather than dropping it.
<!-- source: internal/component/config/change_file.go -- ParseChangeFile, materializeMetaPaths -->

**Structural directives exist in change files only.** A normal draft and a committed
config never contain one, so the materialized draft and commit formats are
unchanged.

## Why not decompose into leaf edits

- A mass of synthetic set and delete entries hides what the operator meant.
- A rename counts as exactly one pending change in the diff and count UI.
- Conflict detection can compare a rename against an overlapping leaf edit,
  which a decomposed form cannot express.

## Constraints

**`PendingChange` exists in both the `config` and `contract` packages, with
identical field names and separate types.** The adapter layer casts between
them. If either type gains a field the other lacks, the cast loses data in
silence.

**`MetaTree.RenameListEntry` and `MetaTree.CopyListEntry` exist for subtree
rebase during a rename and a copy.** Neither is a general-purpose meta
manipulation entry point.

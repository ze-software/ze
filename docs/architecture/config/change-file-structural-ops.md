# Change Files: rename as a structural operation

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

**Rename directives exist in change files only.** A normal draft and a committed
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

**`MetaTree.RenameListEntry` exists for subtree rebase during a rename.** It is
not a general-purpose meta manipulation entry point.

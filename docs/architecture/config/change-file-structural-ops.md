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
| `create-entry <entry path>` | creating an entry or loading a new entry | `set <entry path>` |
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

An entry with no leaf values still needs an owner. `create-entry` retains that
intent in the user's change file across draft saves and candidate staging; a
bare tree node alone does not participate in session commit. Replay uses the
same schema-aware entry addressing as the editor, including multi-key entries.
Metadata entry paths and structural-operation parents use `EditTokens`, not
space-joined strings. `MetaTree.SessionEntriesOrdered` expands stored multi-leaf
keys with the schema, while single string keys remain one token. It visits list
entries in the companion tree's recorded order, retaining metadata-only
deletions after the entries the tree holds. Draft and commit replay, copying
pending edits and rebuilding a session view all use this ordered traversal.
Read-only ownership queries can use `MetaTree.SessionEntries` without an order.
Every structural wire record uses the configuration key quote codec in both directions;
rendered pending paths use the same codec. Recorded creation paths canonicalize
default key leaves and key-pair order.
An entry's complete map key is one identity boundary, so a multi-key sibling
cannot be mistaken for an ancestor merely because its text has the same prefix.

The replay plan normalizes renames and copies of explicitly created entries
without changing the recorded intent. Creating, copying and then renaming an
entry can therefore be saved repeatedly without recreating a move's source or
losing a copy's source dependency. The complete ordered descendant program,
including deletions and toggles, follows an ancestor rename or copy; replay
does not resurrect a child created and then deleted before the move.
The complete creation program runs at the copy or rename's position in the
operation sequence, after any earlier deletion of its destination. List position
is restored separately after replay, so an empty copied or renamed entry cannot
be erased by a destination deletion that preceded the operator's command. A
source deleted and recreated before a copy contributes only its latest program.
Renames of pre-existing ancestors still need their committed source: an
implicitly materialized parent never hides another session's deletion.
Within a creation program, committed replay collapses an existing entry's
rename chain to its original source and final destination. Stale checks still
require that original source even for an identity chain; replay omits the
effect-free move. An intermediate-name copy reads the original source before
its producing renames and replays its copy-time descendant program at the
destination. Later source changes therefore cannot alter the copy.

Saving and reloading use a different input: the shared draft may already hold
an intermediate or final address. The recorded existing-entry lineage locates
its latest materialization, excluding independently owned addresses, and only
the remaining moves are applied. Metadata follows those same effective moves.
The shared tree is never rewound through a historical source: another session
may have created a new entry there. Already materialized copies retain their
earlier snapshot. This needs no saved replay cursor or new change-file syntax.
<!-- source: internal/component/cli/editor_create_entry.go -- prepareDraftReplay, materializedEntrySource, entryHasIndependentOwner, copyBeforeRenames -->

After draft or committed replay, copied and renamed destinations owned by the creation
program take their positions from the candidate tree. Later renames rebase those
destinations before positioning. The adjustment changes list order only: it
neither copies payloads nor transfers metadata, and unrelated entries retain
their relative order. This also preserves another user's entry at a historical
source instead of moving that entry to repair the current session's ordering.
<!-- source: internal/component/cli/editor_create_entry.go -- positionCreatedEntries -->
<!-- source: internal/component/config/tree.go -- RepositionListEntries -->

Saving a previously materialized creation under a new name also prunes its
superseded source. Only the creator's saved leaf metadata moves with that
creation; another session's independently acquired source ownership stays put.
Creation programs retain their ordered rename records rather than coalescing
away intermediate names. Repeated save–rename–save cycles therefore retain
the history needed to move the creator's saved leaves and remove every
superseded materialization.

Creation conflicts with deletion of the entry or an ancestor, but not with an
independent child leaf edit. Discard compares the old and remaining creation
programs' materialized entries, including copy and rename destinations,
superseded sources, and implicitly created list-entry ancestors. Historical
materializations remain eligible for pruning even when a later unsaved delete
has removed their live creation intent; discard cannot resurrect a saved entry.
It prunes otherwise unowned draft entries before consuming change-file ownership, so
failed pruning can be retried. Committed entries and entries still owned by
other pending edits survive. Per-user ownership replacement uses the
config-scoped user identity, since storage listings and callers may spell the
same file as a storage key or an absolute path. Staging a candidate does not
consume creation or publish it before reload acceptance.
Unknown or malformed ownership paths fail resolution explicitly; they never
mean non-overlap. Conflict checks and draft pruning propagate that failure
before commit or ownership consumption. A resolved `EditPath` keeps canonical
tokens and only the multi-token entry boundaries needed by comparisons, not
a duplicate identity-token array. Rebase resolves each operand once.
<!-- source: internal/component/cli/editor_create_entry.go -- planCreations, structuralOpConflicts, rebaseCreatedMetadata, creationFootprint, pruneCreationMaterializations -->
<!-- source: internal/component/config/edit_path.go -- EditTokens, CanonicalEditPath, EditPathHasPrefix, ResolveEditPath, RebaseEditPath, RebaseEditTokens, FormatEditPath, ParseEditPath -->
<!-- source: internal/component/config/meta.go -- SessionEntries, SessionEntriesOrdered -->
<!-- source: internal/component/cli/editor_draft.go -- writeThroughCreate, applyStructuralOpsAroundEntries, applyStructuralOpsToMeta -->

Because the ops run before the leaf edits, a copy reads the committed source,
not the session's edited one. A copy therefore also writes this session's source
leaf edits, sets and deletes, under the target key, as a rename moves them to
the new key. The snapshot includes edits already saved to the draft, not just
those remaining in the change file, and never duplicates another session's
ownership. It also records the ordered member-operation program at the copy's
address, independently of subsequent source rebasing or member deletion.
A copied edit carries no previous value: the committed config holds
nothing at the target, so keeping the source's previous value would read as
stale at commit.
<!-- source: internal/component/cli/editor_draft.go -- writeThroughCopy, copyPendingListEntry -->
<!-- source: internal/component/cli/editor_leaflist.go -- copySessionMemberOps -->

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

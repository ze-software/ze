# Change Files: structural operations

Renaming a keyed list entry is a first-class structural operation in a per-user
change file. It is not decomposed into leaf deletes and creates.

<!-- source: internal/component/config/change_file.go -- StructuralOp, change-file parse and serialize -->

A leaf or path activation whose target is created by pending assignments waits
until those assignments are applied. Later activations of the same address
keep their recorded order. The first missing ancestor is checked against the
editor's original committed view: an ancestor that existed there is still a
stale removal and is refused, so replay cannot resurrect another operator's
deleted entry. Saved assignments and their metadata remain in the draft; the
per-user change file retains the structural activation program.
<!-- source: internal/component/cli/editor_draft.go -- applyReplayOpsAroundEntries, activationTargetPath, activationTargetWasNew, SaveDraft -->

## The shape

A rename line carries the same metadata prefix as a leaf edit:

```
#user @source %time rename <parent> <list> <old> to <new>
```

`SaveDraft()` applies address-changing operations first, then replays the leaf
edits. Member and activation operations whose target those edits create follow
the edits in recorded order. It then writes a materialized draft. A rename
chain inside one session is coalesced:
`old -> mid -> new` becomes `old -> new`.

The same recorded program covers the session editor's other structural verbs.
Each is one line and one pending change. Both `SaveDraft()` and the commit replay
it using the ordering above:

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
The stale-rename scan follows prior entry-address operations on a copy of the
committed tree. A rename may therefore consume an earlier copy's destination,
while that copy still requires its original source and an unused destination.
A foreign entry at the rename's destination is still refused. Explicit creations
are handled by the creation plan; the scan never invents implicit ancestors.
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
earlier snapshot. A failed listing or read of the ownership records refuses
replay; it never means that an address has no independent owner. This needs no
saved replay cursor or new change-file syntax.
<!-- source: internal/component/cli/editor_create_entry.go -- prepareDraftReplay, materializedEntrySource, entryHasIndependentOwner, copyBeforeRenames -->

Stored CLI editors retain the shared materialized draft after a save or a
session-view rebuild, including another owner's saved entries and annotations.
The web factory explicitly constructs an isolated editor instead. Its save
retains the live candidate, and its rebuild starts from committed configuration
with only that session's intent. Both views retain complete ownership metadata
for replay and conflict checks; a view choice never changes what a commit owns.
<!-- source: internal/component/cli/editor.go -- NewEditorWithStorage, NewIsolatedEditorWithStorage -->
<!-- source: internal/component/cli/editor_view.go -- workingViewMetadata, projectSessionTree -->

Browser recovery first rebinds the account's eligible web metadata and
operations, preserving every other owner. It privately runs the same shared
materialized replay to resolve saved addresses and list positions, then builds
the visible candidate from committed configuration with only the resumed
session's entries and operations. Rebinding does not consume the program.
Saving keeps this isolated candidate while writing the shared draft, and
candidate serialization excludes other sessions' contested metadata values.
<!-- source: internal/component/cli/editor_resume.go -- resumeSession, workingSessionMetadata -->
<!-- source: internal/component/cli/editor_draft.go -- SaveDraft -->

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

A user's change file can hold intent from several editing sessions. Saving one
session moves only its leaf entries into the draft and retains its structural
ops for commit. Other sessions' entries and ops remain owned by those sessions.
Accepted publication consumes only the committing session, on both the direct
and staged-candidate paths. A failed cleanup is reported beside the commit that
applied; it does not turn that publication into a refusal.
Explicit discard also removes matching saved leaf metadata. Each affected leaf
is restored from committed configuration, then its surviving saved owners are
replayed in their stored order, so discarded values cannot remain unowned in the
shared materialization or override another owner's saved value. The saved draft
is written before the change-file records are consumed. Both full and path
discard select the current session; another origin under the same account keeps
its ownership.
A failed saved cleanup preserves its cause in a typed operator error, tells the
operator to keep pending files and retry after repairing the reported problem,
and names `run show doctor` as the next check in either configuration editor.
<!-- source: internal/component/cli/editor_discard_saved.go -- discardSavedSessionPath, discardSavedFailure -->
The cleanup warning keeps the cause and tells the operator to keep the session
files, review retained changes with `show | changes all`, and repair the reported
failure. Repeating the commit or discarding shared files is not a recovery step:
the commit already applied, and another session can still own those files.
<!-- source: internal/component/cli/editor_session_cleanup.go -- consumeSessionChangeEntries, cleanupSessionFiles -->

A per-user change file holds an unvalidated candidate. API edits may store a
leaf value or leaf-list member that commit will refuse. The editor restores
those values without judging their leaf constraints, so the operator can
replace them after a refused commit or a reconnect. Reading pending changes
uses the same decoder. Paths, list keys, token counts, secret encodings and
structural directive syntax still have to parse; an error keeps the file and
refuses the operation. The normal set and change-file parsers remain strict.
Saving and committing still run the candidate's validation before publishing it.
<!-- source: internal/component/config/change_file.go -- ParseCandidateChangeFile, ParseChangeFile -->
<!-- source: internal/component/cli/editor_draft.go -- readChangeFile, SaveDraft -->
<!-- source: internal/component/cli/editor.go -- readChangeFileContent -->

A whole assignment to a plain leaf-list uses the annotated command
`replace-members <path> [ members ]`. It requires session-source metadata and
targets a plain leaf-list. Its one metadata entry retains whole-list ownership,
including the sequence and quoted member boundaries, through change-file and
saved-draft replay. Ordinary `set <path> [ members ]` stays additive and records
one owner entry per member. The unannotated configuration parser does not accept
`replace-members`; committed annotations and ordinary singleton output retain
their existing `set` spelling.
<!-- source: internal/component/config/setparser_meta.go -- parseLineWithMeta, walkAndSetWithMeta -->
<!-- source: internal/component/config/serialize_set.go -- writeValueOrArrayAssignments, serializeSetMetaChild -->

Whole-leaf inactivity stays separate from member-operation ownership. Annotated
session assignments and per-member operations keep an `inactive <path>` marker
alongside their existing commands. Ordinary plain leaf-list set output uses
`nop <path> [ members ]`, followed by member `nop` lines only for independently
inactive members. Saving and reloading either form preserves the whole-leaf
flag, member flags and ordered repeats; activating the leaf leaves independent
member deactivations in place.
<!-- source: internal/component/config/serialize_set.go -- emitValueOrArrayNop, emitMetaValueOrArrayNop, serializeSetMetaChild -->
<!-- source: internal/component/config/setparser.go -- wholeLeafListNop, markNopInactive -->

The shared token codec records quoting for candidate reads. A bracket inside
a quoted member remains data, including when that member fails its leaf
constraints. A bare delimiter run that the strict parser refuses keeps that
refusal; deferring the member constraints cannot turn broken brackets into
editable values. The accepted older joined-member spelling keeps its existing
interpretation.
<!-- source: internal/component/config/list_key.go -- splitQuotedTokensMarked -->
<!-- source: internal/component/config/setparser_candidate.go -- candidateMembers -->
<!-- source: internal/component/config/setparser_meta.go -- parseLineWithMeta, walkAndSetWithMeta -->

## Why not decompose into leaf edits

- A mass of synthetic set and delete entries hides what the operator meant.
- A rename counts as exactly one pending change in the diff and count UI.
- Conflict detection can compare a rename against an overlapping leaf edit,
  which a decomposed form cannot express.

## Constraints

Default session comparison and graphical browser review use one path-qualified
renderer over typed pending records. Review reads current committed values under
the same storage guard as the session's durable intent and projects removed
values onto presentation copies. It never rewrites the metadata's original
`Previous` value or rebuilds the candidate. Rename ancestry uses the same
`committedPathOf` resolver as conflict checks. Explicit source, target and format
comparisons retain their parsed-tree representation.
The session count uses the same logical record collector as review, including
saved metadata and unsaved structural intent. It reads that user's known
change file directly instead of needing a census of other users' files.
<!-- source: internal/component/cli/editor_pending_diff.go -- PendingReview, pendingReviewChanges, RenderPendingChanges -->
<!-- source: internal/component/cli/editor.go -- pendingChanges, pendingSessionChanges -->
<!-- source: internal/component/cli/model_load.go -- cmdShowPipe, ShowPipeOpts -->

`PendingChange` has separate `config` and `contract` representations. Boundary
conversion preserves the owner, kind, paths, member and values. The `config`
representation also carries the session identifier used for internal selection
and deduplication. The read-only discard snapshot returns contract records;
regular editor adapters convert the configuration records.
<!-- source: internal/component/config/change_file.go -- PendingChange -->
<!-- source: internal/component/cli/contract/contract.go -- PendingChange -->
<!-- source: internal/component/cli/editor_pending_snapshot.go -- PendingDiscardChanges -->
<!-- source: cmd/ze/hub/editor_adapter.go -- PendingChanges -->

**`MetaTree.RenameListEntry` and `MetaTree.CopyListEntry` exist for subtree
rebase during a rename and a copy.** Neither is a general-purpose meta
manipulation entry point.

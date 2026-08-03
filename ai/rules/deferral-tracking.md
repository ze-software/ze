# Deferral Tracking

**When:** when deciding that in-scope work will not be done now
**Severity:** blocking

## Directives

**Obligation on you (not a hard gate):** Every decision to not perform in-scope work MUST be recorded AND land in a destination spec.
Rationale: Untracked deferrals are invisible scope reductions. They accumulate silently across sessions.
A deferral whose destination is prose ("later", "future work") is a deletion with a polite name.

The commit gate that checks homing **WARNS, it does not block** (see "Status Vocabulary"
and the gate note below). An unhomed deferral row is harmless to software behaviour: the
worst case is that it is committed too early or in the wrong commit. Blocking every commit on
it -- including commits that never touched deferrals, and rows another session wrote into the
shared working tree -- held real work back for no software reason. So the obligation to home
a deferral is a discipline the gate reminds you of, not one it enforces: the warning keeps an
unhomed row visible so it is not lost, but you are the one who must give it a home.

## Central Log

`plan/deferrals/` -- a sharded directory, **one file per source**, holding all
deferred work. There is NO single `plan/deferrals.md` and no committed aggregate: <!-- doc-links: ignore (the single file is deliberately retired) -->
the live backlog is a fold over the directory, computed on read (`/ze-status`) and
never stored. A stored aggregate would be a shared file every session appends to --
exactly the cross-commit hazard this layout removes (`ai/rules/git-safety.md`).

**Shard key.** Each row lives in the shard named for its source:

| Source of the row | Shard file |
|-------------------|------------|
| A spec (row's `Source` names `spec-<stem>`) | `plan/deferrals/<stem>.md` |
| Ad-hoc (no source spec) | `plan/deferrals/ad-hoc-<YYYY-MM-DD>-<sid>.md` |

A shard is a small markdown file with the six-column table header and only the rows
it owns. Add your row to the shard for its source (create the shard if it does not
exist); never touch another source's shard except to correct a row it owns. Because
each path has a single writer, `git add <shard>` stages only your row and git merges
disjoint shard creations without conflict.

**A spec's shard is deleted at the spec's closure ONLY when every row in it is terminal.** Spec closure commit B (`ai/rules/planning.md` "Spec Closure") `git rm`s `plan/deferrals/<stem>.md` alongside the spec.

**Reading the Status column of the closing spec's OWN shard is a NEW step, and no earlier check covers it.** The grep closure already requires (`ai/rules/planning.md`, "Closure resolves the spec's deferral rows") searches every shard for this spec as a **Destination**. It never reads the closing spec's own shard as a **Source**. Do not assume the existing grep answered this question: it answers a different one.

**A shard that still holds a live row SURVIVES its source spec, and keeps its source-keyed name.** The row's home is the destination spec named in its Destination cell. The shard is only where the row is written down, so deleting the shard deletes a record of live work whose home is somewhere else entirely.

**Two readings, and the one that governs.** "The shard is deleted at closure" and "a homed row stays live" (Status Vocabulary, below) contradict each other for a shard whose rows are homed at OTHER specs. Measured on 2026-08-03 by `scripts/dev/deferral_orphans.py`: 39 shards were in exactly that state, holding 68 live rows between them. Re-run the script rather than re-deriving the number; two hand-counts of it were wrong before the script existed. Deletion-at-closure governs the all-terminal case ONLY. Where a live row remains, the row wins and the shard stays.

**An orphaned shard is not a defect to sweep.** A shard whose `plan/spec-<stem>.md` is gone while live rows remain is the correct end state of the paragraph above, not leftover mess. Do not bulk-delete orphaned shards to tidy the directory: read the rows first, and delete only a shard in which every row is terminal.

**`deferral_shard_removal_problems` (`scripts/dev/commit_helper.py`) refuses the removal, so this is not honor-system.** It reads the shard at HEAD and BLOCKS when any row is non-terminal. It has to block rather than warn: every other signal over these rows folds across the `plan/deferrals/` DIRECTORY, so deleting a live-bearing shard LOWERS their counts instead of raising them, and the forbidden action is the one that silences every observer of the rows it destroys (`ai/rules/fail-closed-guards.md`).

**An all-terminal orphaned shard is residue, and the actor who deletes it is the closer of the LAST spec that homed one of its rows.** Setting the final row to `done` is what makes the shard residue, so the same commit removes the file. Without a named actor the state never drains: 14 such shards existed on 2026-08-03 because each was left for whoever came next. Nobody is obliged to hunt for others.

**A live row whose SOURCE spec closed still needs a real Destination, and that is the thing closure must check.** The source spec's disappearance is what makes a prose destination unrecoverable: nothing on disk was ever going to become "a future usability spec", and now nothing will create it either. Six such rows were found and homed on 2026-08-03; they had been live since 2026-07-17 and 2026-07-21, which is why the same-day measurement above reports every row homed.

## When to Record

| Trigger | Action |
|---------|--------|
| Deciding work is "out of scope" | Record with reason |
| Moving work to another spec | Record with destination spec |
| Skipping a task item from a spec | Record with reason |
| Postponing for any reason | Record with reason |
| User asks to skip something | Record (user-requested, still tracked) |

## Table Format

```
| Date | Source | What | Reason | Destination | Status |
```

| Column | Content |
|--------|---------|
| Date | YYYY-MM-DD |
| Source | Spec filename, task description, or "ad-hoc" (also selects the shard, see "Central Log") |
| What | Specific work being deferred (not vague) |
| Reason | Why it is being deferred |
| Destination | Receiving spec filename (`plan/spec-*.md`), "cancelled", or "user-approved-drop" |
| Status | See Status Vocabulary below |

## Status Vocabulary (the gate reads this)

`deferral_unassigned_problems` (`scripts/dev/commit_helper.py`) checks the
Destination of every row whose Status is NOT terminal. The terminal set is
`DEFERRAL_TERMINAL_STATUSES` in that file:

| Status | Meaning | Destination checked? |
|--------|---------|----------------------|
| `deferred` | Live: the work is outstanding. MUST name its home spec | YES |
| `open` | Live: synonym of `deferred`. Prefer `deferred` | YES |
| `done` | Terminal. The work landed, or the row was superseded | no |
| `cancelled` | Terminal. User decided not to do it | no |
| `resolved` | Terminal. Closed with evidence (learned summary) | no |

**A homed row stays live.** The status answers "is this work still outstanding",
NOT "does it have a home". Homing is mandatory, so a live row is the NORMAL,
correct state of a deferral: it has a spec AND the work has not landed yet. It goes
`done` when the work is implemented, not when it is filed. A live row is not a
violation and is not a backlog of unfiled work.

This is the invariant the gate is built on: it re-checks every live row's
destination on every commit, so "outstanding work names a real spec" is surfaced
continuously (as a warning), for as long as the work is outstanding. Closing a row
early to quiet the warning hides the work from the only thing watching it.

`open` and `deferred` are synonyms, and the redundancy is a wart: it is what let the
gate and this rule teach different words in the first place. Do not add a third.
**Any word that is not in the terminal set is treated as live and checked**,
deliberately: the gate is a denylist of terminal states, not an allowlist of live
ones, so a status nobody has invented yet fails closed rather than slipping through
silently (`ai/rules/fail-closed-guards.md`).

**Blind spot, stated rather than papered over:** a terminal status skips the
destination check entirely, so a `done` row whose Destination is prose is not
flagged. `done` is an assertion the gate trusts. That is tolerable only because
`done` means the work LANDED, so nobody is routed toward it while work is
outstanding, and its Destination is often a commit SHA rather than a file. Marking a
row `done` before the work lands both lies and disables the check, which is why the
row above stays live.

This table and `DEFERRAL_TERMINAL_STATUSES` must not drift apart. They did once,
and it cost: the gate tested only `status == "open"` while this rule's own prose
taught the word `deferred`, so rows written correctly per the rule were never
looked at. 23 live rows without a home had accumulated behind that hole.

## Rules

| Rule | Detail |
|------|--------|
| Always a destination spec | Every deferral names a `plan/spec-*.md` that exists on disk, whatever its Status. Only `cancelled` / `user-approved-drop` may name no spec |
| No prose destinations | "later", "future work", "a follow-up", "TBD" are not destinations. A destination is a filename |
| No vague What | "Edge cases" is not acceptable. Name the specific case |
| Record immediately | Do not batch. Record when the decision is made, not at commit time |
| Review at session end | Live rows are expected and fine. Check that each still names a real home, and close only the ones whose work actually landed |

The gate is one notch wider than this rule on purpose: it accepts any existing
`plan/**.md`, not only `plan/spec-*.md`. Do not use that slack. A destination is
a spec.

**`plan/known-failures/` is NOT a destination** (`ai/rules/fix-dont-record.md`).
A shard is the running log of an investigation you are still driving, so pointing
a deferral at one means "this red is somebody's problem later", which is the
parking this rule exists to prevent. A red test is fixed. If the fix is genuinely
a separable piece of work, home it in a spec like anything else. In particular,
"fails under load" is a diagnosis and never a destination: the test asserts on
elapsed time, and that is fixed, not deferred.

## Choosing the Destination Spec (BLOCKING)

Deferred work ALWAYS has a destination spec. Decide which one in this order, at
the moment the deferral is made:

| Order | Action | Detail |
|-------|--------|--------|
| 1 | Find an existing spec that already covers the topic | `grep -l "<topic>" plan/spec-*.md`, and scan `make ze-spec-status`. Prefer a `spec-finish-<subsystem>` / `spec-followup-<subsystem>` umbrella when one owns the area |
| 2 | If one exists, add the work to its `## Task` section | That spec is the home. Record the deferral with it as Destination, Status `deferred` |
| 3 | Only if no spec covers the topic, create a deferral spec | Named `plan/spec-<source>-deferred-<subtask>.md` (see below). Record the row with it as Destination, Status `deferred`, exactly as in step 2 |

An existing spec is preferred over a new file. Do not create a deferral spec to
avoid the grep.

**Both routes record a LIVE row.** Filing work in a spec is not finishing it, so the
row stays `deferred` and keeps naming its home until the work lands. Do not close a
row at step 2 or 3: a `done` row is never destination-checked again, so closing it on
filing is precisely how the work stops being watched (see "Status Vocabulary").

### Deferral Spec Naming (BLOCKING)

A spec created solely to hold work deferred out of another spec is named:

```
plan/spec-<source>-deferred-<subtask>.md
```

| Part | Content | Example |
|------|---------|---------|
| `<source>` | Stem of the spec the work was deferred FROM, without the `spec-` prefix | `bgp-rib-flush` |
| `<subtask>` | Short kebab-case name of the specific deferred work | `ipv6-coverage` |
| Result | | `plan/spec-bgp-rib-flush-deferred-ipv6-coverage.md` | <!-- doc-links: ignore (illustrative naming example, not a live spec) -->

- One subtask per file. Two deferrals from the same source spec are two files,
  not one file with two tasks.
- The name carries the provenance: a reader knows what dropped it and why the
  file exists without opening it.
- For ad-hoc deferrals with no source spec, `<source>` is the subsystem
  (`plan/spec-l2tp-deferred-session-teardown-race.md`). <!-- doc-links: ignore (illustrative naming example, not a live spec) -->
- **A source spec does not outlive the deferral.** Spec closure `git rm`s the
  spec (`ai/rules/planning.md` "Spec Closure"), so `<source>` will usually name
  a file that no longer exists by the time someone picks the work up. That is
  correct and intended: the provenance lives in git history, and the deferral
  spec is the tracker now. But when the source spec is ALREADY closed at the
  moment you write the deferral spec (homing an old row), name `<source>` for
  the subsystem instead: a filename pointing at a spec nobody can open reads as
  a broken reference rather than as provenance. Record the closed source spec in
  the `## Task` section either way.
- This naming applies only to deferral holders. Specs written as intended work
  keep the normal `spec-<task>.md` / `spec-<prefix>-<N>-<name>.md` names
  (`ai/rules/planning.md` "Spec Sets").

### Creating the Deferral Spec

| Step | Action |
|------|--------|
| 1 | Create the file from `plan/TEMPLATE.md` with `Status \| skeleton` |
| 2 | Fill only the `## Task` section with the points to complete, plus any constraint already known. Leave the rest as template placeholders |
| 3 | Name the source spec in the `## Task` section so the provenance survives |
| 4 | Record the deferral in its `plan/deferrals/<source>.md` shard with the new spec as Destination and Status `deferred` |

Keep it small. The goal is zero lost work, not a finished design -- a skeleton is
captured intent, not a designed spec. It moves to `design` when someone picks it
up (status table in `ai/rules/planning.md`).

The commit gate `deferral_unassigned_problems` (`scripts/dev/commit_helper.py`)
folds over every shard in `plan/deferrals/` and WARNS -- it surfaces, it does not
block -- on any LIVE deferral (any non-terminal Status, see Status Vocabulary) that
names no destination or names a spec file that does not exist, and on any row it
cannot parse. It is routed through
`commit_gate_warnings`, not `commit_gate_problems`: the message prints to stderr
and the commit proceeds. This is advisory by design, for the reason in the banner
above (an unhomed row is harmless to software; blocking unrelated and other-session
commits on it was too aggressive). Homing stays mandatory as an obligation on the
author; the warning is what keeps an unhomed or unparseable row visible so it is
not silently lost.

## Verify Before Deferring (BLOCKING)

Never claim "requires infrastructure that doesn't exist" without grepping for it first.
Before writing "deferred -- requires X" in any spec or summary, grep for X. If it exists,
implement it. If genuinely missing, name the specific thing that is missing and where it
would need to be added.

## What Is NOT a Deferral

- Completing work that was never in scope (no record needed)
- Choosing between two valid approaches (design decision, not deferral)
- Go `defer` keyword (language construct, excluded from pattern matching)

## Resolving Deferrals

A row is closed when the WORK is settled, never when it is merely filed.

| To close as | Set Status to | Set Destination to |
|-------------|---------------|--------------------|
| Implemented | `done` | Spec or commit where implemented |
| User decided not to do it | `cancelled` | `user-approved-drop` |
| Superseded (another row or spec now owns it) | `done` | The row or spec that took it over |

**Filing work in a spec is NOT a close.** Moving work into a spec gives the row its
Destination; the row then stays `deferred` until the work lands. This table's
predecessor said "moved to another spec -> `done`", which read as "filing closes the
row" and cost real coverage: 13 rows were closed on filing in one session, hiding
their work from the gate while none of it had been done. If the work is not in the
tree, the row is not `done`.

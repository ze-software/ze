# Rationale: Unfinished Scope Becomes a Spec

An in-scope item a spec does not do becomes its own spec, in the release bucket
that item belongs to (`plan/README.md`). The source spec's `Work Not Done`
table names that spec by path. It is not parked in a row.

## Why a row does not work

Ze tracked this work in `plan/deferrals/` until 2026-09-05: one shard per <!-- doc-links: ignore (the directory was deleted on 2026-09-05 and this page is the record of it) -->
source, one row per item, each row naming a destination spec. The directory
held 103 live rows on the day it was deleted, and 29 of them named no
destination at all. Those rows said "a spec of its own, not yet written".

That work appeared in no count. `/ze-status` counts specs, the WIP cap counts
specs, and the release buckets count specs, so 29 items existed in a form
nothing measured. A row nobody can count is a row nobody schedules. All 29
became specs in the work that deleted the directory.

The rest of the corpus showed the same drift more slowly. A row's `Status` was
`deferred` on 127 rows and `open` on 12, so any filter on `open` read most of
the backlog as empty. A shard outlived its source spec, so the row survived
in a file whose name no longer told a reader who owed the work.

## What a spec buys that a row does not

- It is counted, so the backlog and the release buckets both see it.
- It carries a bucket, so the reader learns what the release pays for leaving
  it undone.
- It carries the design context the item needs, rather than one table cell.
- It closes the way every other spec closes, through a review gate and two
  commits, rather than by a status word one author changes.

## What did not change

Scope reduction is still the owner's decision, never the author's
(`ai/rules/completion.md`). Writing the destination spec is not permission to
drop the item: it is where the item waits.

A defect you walk into while you work on something else is a different thing.
It takes a different route. It gets one row in `plan/journal/<class>.md`, and then
the work in hand closes. The journal is unaffected by any of this.

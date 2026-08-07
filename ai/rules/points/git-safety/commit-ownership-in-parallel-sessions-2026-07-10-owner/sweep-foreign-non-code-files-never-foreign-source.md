---
kind: note
level: MAY
stage:
---
Refinement, per owner direction (2026-07-17): the "always" above governs
CODE. A concurrent session's out-of-scope NON-CODE files -- generated
discovery indexes, docs, tracking markdown -- MAY be swept into your commit
when doing so keeps the tree consistent or unblocks you; foreign source and
test files are never included. Name the inclusion and its origin in the
commit body. This does not clear the whole-tree closure gates: the deferral
homing check folds over every shard in `plan/deferrals/`, so a foreign entry
in any shard that lacks a real destination spec is surfaced at closure
regardless of what you commit -- home it in its own shard, do not paper over
it (the check is advisory, but the obligation is not).

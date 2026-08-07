# Retired rule instructions

One line per point removed from `ai/rules/points/`. A point IS an instruction,
and every gate in this system reads the tree as it is: delete the file and its
manifest line together and the render check, the round trip, the gate map and
the rule lint all stay green, because the points and the rendered rule agree on
the smaller corpus. This file is what makes the removal say so.

`corpus_shrink` in `scripts/dev/rules_points.py` compares each rule's point
count against git HEAD and requires the drop to be covered by lines added to
this file since HEAD. `make ze-rules-gate-map` and `make ze-doc-test` fail
otherwise.

Scope, never an allowlist: a line stops counting the moment it is committed,
because HEAD moves with it. Nothing here pre-approves a future deletion, and
nothing here needs pruning.

A rename is not a retirement. Moving a point between sections of the same rule
leaves the count unchanged, and the point's binding is repointed at the new id.

A row is CHECKED, not believed. `retired_rows_since` validates every id against
the point names git HEAD actually carried, and refuses four shapes: a malformed
row, an id HEAD never held, an id whose point is still on disk, and a second row
for an id this file already declares. Without that check a fictional id cleared
the ratchet: declaring `rule/nowhere/never-existed` bought a real deletion
elsewhere in the same rule. A row naming a live point is refused for two
reasons, since it would both cover a drop the rule did not declare and excuse a
check from gating an instruction the corpus still carries.

Retiring an instruction also frees its check. `unbound_regressions` fails a
check that named a point at HEAD and declares `# ze point: none -- <why>` now,
because that is the cheapest way to launder a rename into a lost gate. A point
declared here is exempt: the instruction left the corpus on purpose, so the
check has nothing left to name. Retire only part of what a check named and the
live points are still reported.

One row per retired point, newest last. The Point cell is the id, backticked.
The Why cell says what happened to the instruction, not that it was removed.

| Point | Why |
|-------|-----|

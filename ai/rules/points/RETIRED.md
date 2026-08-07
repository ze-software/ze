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

One row per retired point, newest last. The Point cell is the id, backticked.
The Why cell says what happened to the instruction, not that it was removed.

| Point | Why |
|-------|-----|

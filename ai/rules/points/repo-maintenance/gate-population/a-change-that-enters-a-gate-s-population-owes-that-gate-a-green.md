---
kind: directive
level: MUST
stage:
---
**A change that moves files INTO a tree a gate reads, or that widens what a gate
requires, MUST leave that gate green over the whole affected population, in the
same change.** A gate's population is derived from the tree, so it grows the
moment a file lands inside it. Code that was correct where it lived can be red
the instant it moves, without a line of it changing, and a requirement that
ratchets makes every file already in the population red at once.

**The red that results belongs to nobody, which is what makes it expensive.** It
is charged to the next author who prepares a commit, and it names a rule that
author did not break, in files that author did not write. Every session after
the change pays to rediscover the same cause, and the gate that was built to
report a real defect now reports only its own migration.

**A gate that runs on WRITE turns an unmigrated file into a file nobody can
edit.** Where the ratchet is enforced after each edit rather than at commit time,
a file that does not yet satisfy the new requirement refuses every edit made to
it, whatever that edit was about. The block is invisible from the side that
authored the change, because anything created after it already conforms.

**So the change MUST carry the migration, and the migration MUST cover the
population rather than the examples the author happened to open.** Deriving the
population from the same source the gate derives it from is the only way to know
the two agree. A sample that passes is not evidence about the rest.

**Where a record states what a gate REPORTED under an older name or an older
requirement, that record MUST NOT be rewritten to satisfy the new one.** It
describes a run that happened. Migrating it forward replaces a fact with a claim
that was never true, which costs more than the red it clears.

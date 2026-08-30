---
kind: directive
level: MUST
stage:
---
**The rule above governs CODE. A concurrent session's out-of-scope NON-CODE files (generated discovery indexes, docs, tracking markdown) MAY be swept into your commit when that keeps the tree consistent or unblocks you. Foreign source and test files MUST NOT be.** The inclusion and its origin MUST be named in the commit body.
**Sweeping a file does not clear a whole-tree closure gate.** The deferral homing check folds over every shard in `plan/deferrals/`, so a foreign entry lacking a real destination spec surfaces at closure whatever you commit. You MUST home it in its own shard rather than paper over it.

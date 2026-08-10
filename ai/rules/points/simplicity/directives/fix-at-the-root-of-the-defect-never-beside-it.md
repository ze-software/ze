---
kind: directive
level: MUST
stage:
---
**The simplest fully correct fix MUST be at the ROOT of the defect. A special case bolted onto shared infrastructure is not the simpler option: it adds a branch AND leaves the defect live for every caller the special case does not name.**
**Depth and size point the same way here. A one-line fix at the root beats a guard at three call sites, and the `/ze-review` altitude check reports the guard as the finding.**

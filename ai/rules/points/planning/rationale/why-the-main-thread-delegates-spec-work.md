---
kind: note
level:
stage:
---
Thomas set the delegation shape on 2026-07-28 after main-thread sessions repeatedly did
spec work inline. Two costs drove it. First, a main thread that implements
cannot supervise: its context fills with the detail of one phase, so the phase
boundaries and the independence of review both blur, and the session ends up
reviewing its own work. Second, subagent context is disposable while main-thread
context is not, so the expensive reading belongs in an agent whose report is the
only thing that survives into the supervising context.

---
kind: note
level:
stage:
---
Thomas set the delegation shape on 2026-07-28. A main thread that implements
cannot supervise because its context fills with one phase's detail. That blurs
phase boundaries and the independence of review.
Subagent context is disposable while main-thread context is not, so expensive
reading belongs in an agent whose report is all that survives.

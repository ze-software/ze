---
kind: note
level:
stage:
---
Cycle detection and `TopologicalTiers` walk both kinds of edges when BOTH
endpoints appear in the resolved name set, so startup ordering is preserved
whenever the optional dep is actually present.

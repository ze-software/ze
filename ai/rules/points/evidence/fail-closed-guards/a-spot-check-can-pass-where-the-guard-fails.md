---
kind: note
level:
stage:
---
Beware the guard that works where you spot-check it and not where it matters.
Cardinality visibly rejects on `list` nodes, which is exactly why "YANG checks
cardinality" survives a probe and is false for leaf-lists.

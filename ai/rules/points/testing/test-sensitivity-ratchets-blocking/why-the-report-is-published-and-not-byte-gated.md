---
kind: note
level:
stage:
---
The split is deliberate. Byte-gating the whole report charged a
regenerate-and-commit to ~60% of commits, and a check that fires that often for
cosmetic reasons gets routed around instead of read: the same "advisory gate
permanently red" failure the report is built to expose.

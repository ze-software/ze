---
kind: note
level:
stage:
---
Mutation testing through the Go `gomu` binary runs unit tests only. It never
executes `.ci` or `.et`, so it cannot catch a functional false pass. This remains
a manual test-design discipline.

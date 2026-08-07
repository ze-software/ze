---
kind: note
level:
stage:
---
If the Go team removes `NoEscape` or changes escape analysis to see through the `uintptr` round-trip, the inline array optimization breaks and `var b Buffer` reverts to heap allocation. This is not a correctness bug (the code still works), but a performance regression.

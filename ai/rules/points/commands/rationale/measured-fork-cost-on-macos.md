---
kind: note
level:
stage:
---
On macOS, each `fork+exec` costs ~4-5 ms. A loop over 400 files x one `grep`
per iteration = ~2 seconds of pure fork overhead before any real work. Add a
second command per iteration (pipe to `sed`, call `awk`) and it doubles. Nested
loops make it quadratic.

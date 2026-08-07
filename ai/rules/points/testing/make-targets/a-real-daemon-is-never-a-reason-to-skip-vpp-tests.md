---
kind: note
level:
stage:
---
"VPP needs a real daemon" is not a valid reason to skip tests. The `vppOps`
interface seam exists precisely so Apply logic can be tested without VPP.
Translate and Verify are pure functions with no VPP dependency at all. If a
new backend cannot be tested with the fakeOps pattern, that is a design
problem to fix before merging, not a deferral to log.

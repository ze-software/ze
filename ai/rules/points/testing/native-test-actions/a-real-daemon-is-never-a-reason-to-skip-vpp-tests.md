---
kind: directive
level: MUST NOT
stage:
---
**"VPP needs a real daemon" MUST NOT be given as a reason to skip a test, and
every VPP backend MUST ship with functional tests.** The `vppOps` interface seam
exists so Apply logic can be tested without VPP, and Translate and Verify are
pure functions with no VPP dependency at all. A new backend that cannot be tested
with the fakeOps pattern is a design problem to fix before merging, never a
deferral to log.

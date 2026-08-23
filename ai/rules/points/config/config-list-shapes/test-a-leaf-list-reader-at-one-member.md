---
kind: directive
level: MUST
stage:
---
**A test for a leaf-list reader MUST exercise the SINGLE-member case, and a test
for a list reader MUST use the keyed-map shape.** A multi-member leaf-list
fixture passes with the assertion bug in place, so it discriminates nothing, and
an array-of-entries fixture for a list feeds a shape no producer emits.

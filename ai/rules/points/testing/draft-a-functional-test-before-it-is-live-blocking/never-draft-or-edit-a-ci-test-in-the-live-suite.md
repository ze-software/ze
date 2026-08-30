---
kind: directive
level: MUST NOT
stage:
---
**A `.ci` MUST NOT be written or iterated on inside `test/<suite>/`, and a live
one MUST NOT be edited in place.** Copy it into `test/draft/<suite>/`, work
there, and move it back. That directory runs on every verify in the checkout,
including runs by OTHER sessions, who then have to work out whether your
half-written test is their regression. The incubator's contract is
`docs/functional-tests.md`.

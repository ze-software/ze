---
kind: directive
level: MUST
stage:
---
**Every sleep in a `.ci` test MUST carry its justification marker, in the form
`// sleep(<kind>): <reason>`.** Two producers enforce it: `./le doc wiring` at
gate time, and the Write/Edit hook at edit time. The closed set of kinds, what
each reason owes, and where the comment goes are
`docs/architecture/testing/ci-format.md`.

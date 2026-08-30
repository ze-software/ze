---
kind: directive
level: MAY
stage:
---
`debug` MAY be a verb (first token) and a noun under a read verb at the same time
(`show debug` displays debug state). The two do not collide: `show debug` reads,
`debug ...` perturbs.

---
kind: directive
level: MUST
stage:
---
**`plan/known-failures/` MUST NOT be used as a destination** (`ai/rules/completion.md`).
A shard is the running log of an investigation you are still driving, so pointing
a deferral at one means "this red is somebody's problem later", which is the
parking this rule exists to prevent. A red test MUST be fixed. If the fix is genuinely
a separable piece of work, home it in a spec like anything else. In particular,
"fails under load" is a diagnosis and never a destination: the test asserts on
elapsed time, and that is fixed, not deferred.

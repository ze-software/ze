---
kind: directive
level: MUST
stage:
excepted-by: completion/directives/spec-a-found-problem-close-then-ask
---
**A test failure that says the PRODUCT is wrong MUST be fixed: by you when it blocks your goal, and by one journal row when it does not.** Logging is not an alternative outcome for a product defect (owner directive 2026-07-23; see "Recording is not fixing" above). A `plan/known-failures/` shard is the running record of an investigation you are still driving, never a place to leave a product defect.
**A test failure that says the SCAFFOLDING is wrong MUST NOT be fixed on the way past.** Name it in one line and step over it (`ai/rules/pre-release.md`). Fixture drift, a stale golden file and a broken runner path are instrument failures, and repairing them is how a session spends its budget on nothing.

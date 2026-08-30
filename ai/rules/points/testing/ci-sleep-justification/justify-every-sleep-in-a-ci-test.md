---
kind: directive
level: MUST
stage:
---
- **A sleep MUST be converted to a deterministic wait whenever a condition exists to wait on** -- `fixture.Poll` around `fixture.Dispatch`, an SDK readiness callback, a context, or a `wait_until` / `dispatch_until` engine step. A duration is what a test writes when it cannot name the condition, so naming that condition is the work.
- **A sleep that stays MUST carry its justification marker, in the form `// sleep(<kind>): <reason>`, and the reason MUST name a mechanism a later reader can check and overturn.** Two producers enforce it: `./le doc wiring` at gate time and the Write/Edit hook at edit time. The closed set of kinds, what each reason owes, where the comment goes, and the ratchet that caps how many sleeps exist are `docs/architecture/testing/ci-format.md`.

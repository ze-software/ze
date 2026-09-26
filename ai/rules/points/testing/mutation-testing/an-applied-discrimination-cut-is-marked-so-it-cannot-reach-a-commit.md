---
kind: directive
level: MUST
stage:
rationale: ai/rationale/testing.md
---
**An applied discrimination cut MUST carry `// MUTATION-APPLIED` and MUST NOT reach a commit; a discrimination note recording which break would redden a test MUST carry `// MUTATION:` and belongs at HEAD.** The two are opposite states wearing one word today. A note is prose above a test naming the break that proves it, which is what a tagged test owes (`ai/rules/interop-and-goal-validation.md`). An applied cut is an edit to product code that makes the product wrong on purpose, for the seconds between breaking it and observing the red. Notes at HEAD in `_test.go` files are right to be there; an applied cut that reaches HEAD ships the defect the test was written to catch, with the test green over it.
**A session that stops an agent mid-proof MUST search the tree for an applied cut before it commits anything.** The window between applying a break and observing the red is where an interruption does its damage, and the agent that held the intent is gone. No gate finds an applied cut. A search for the marker MUST cover `_test.go` files too.

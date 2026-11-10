---
kind: directive
level: MUST
stage:
---
- **A test MUST answer one question, through the cheapest route that decides it: a unit test, then a `.ci`, then a lab.** A test that bundles several behaviours under one name costs every run the time of all of them, and its red does not say which behaviour broke, so the next session reads a setup or neighbour failure as the change's own red.
- **A test breaks this rule when its red would not say which behaviour broke, or when a unit test could decide its assertion.** Length is not the defect: a test whose question is a sequence (a reload, a lifecycle, a whole path from the user's command to the wire) MUST stay one test, because its pieces would no longer prove the sequence.
- **A new behaviour MUST get a new test, never assertions appended to an existing one.** `./le test cost report` ranks the tests that take the largest share of their suite or package, as candidates to read against this rule; its times are host-dependent triage and never a verdict.
- **Splitting a test MUST move every assertion into a new test, and a split that `./le test weakened check` flags MUST go to an independent `ze-test-judge` agent** like any other weakening: it passes only when each old assertion is present in a new test.

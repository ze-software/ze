---
kind: directive
level: MUST NOT
stage:
---
**A passing interop or functional test is evidence only if it would FAIL when the behavior under test is broken. A test that passes whether or not the fix is present MUST NOT be presented as evidence.**
**A test added to ALREADY-WORKING code never had a red phase, so its discrimination is unproven until you force one.** This is not TDD's red-then-green: a regression test and an interop scenario for existing behavior both start green.

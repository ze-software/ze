---
kind: table
level:
stage:
---
| Claim | Acceptable Evidence | NOT Acceptable |
|-------|-------------------|----------------|
| Feature works | Test name + output | "make ze-verify passes" |
| Feature is wired in | Wiring test that exercises entry-to-feature path | Unit test with mock/fake entry point |
| AC-N done (wiring) | Functional test name exercising full path | Unit test in isolation |
| AC-N done (logic) | Unit test name + file, assertion matches AC text | "should work" |
| AC-N done (behavior) | Test asserts the AC's expected behavior directly | Test asserts mechanism (e.g., "no error" as proxy for "rejected") |

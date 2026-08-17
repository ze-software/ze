---
kind: directive
level: MUST
stage:
---
- During development, the session MUST run a focused test sample for the changed code path before a fuller, aggregate, or full suite.
- The sample MAY include the test being developed.
- The focused sample is a scope probe, not a suite substitute.
- The session MAY run more focused sample tests to find the failure boundary.
- When a sample test fails, the fix loop MUST use the narrowest command that reproduces that failure.
- The session MUST NOT run a fuller, aggregate, or full suite until the reproducer and the relevant focused sample tests pass.

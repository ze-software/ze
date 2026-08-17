---
kind: directive
level: MUST
stage:
---
- During development, the session MUST run a focused test sample for the changed code path before a fuller, aggregate, or full suite.
- The sample MAY include the test being developed.
- The focused sample is a scope probe, not a suite substitute.
- The session MAY run more focused sample tests to debug the problem, find the failure boundary, or unblock diagnosis.
- When a sample test fails, the fix loop MUST use the narrowest command that reproduces the failure unless broader focused samples are needed for diagnosis.
- A fuller, aggregate, or full suite MUST run only after the reproducer and the selected focused checks for the finished change pass.

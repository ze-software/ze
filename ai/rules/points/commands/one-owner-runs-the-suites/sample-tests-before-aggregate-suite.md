---
kind: directive
level: MUST
stage:
---
- During development, the session MUST start with a focused test sample for the changed code path before it runs a fuller, aggregate, or full suite.
- The sample MAY include the test being developed.
- When that sample finds a failing test, the fix loop MUST use the narrowest command that reproduces that failure.
- The narrow loop MUST NOT stop the session from running more focused sample tests when needed to debug, find the failure boundary, or remove a blocker.
- The fuller, aggregate, or full suite runs after the focused debugging loop no longer finds a relevant failure. It MUST NOT be the first probe.

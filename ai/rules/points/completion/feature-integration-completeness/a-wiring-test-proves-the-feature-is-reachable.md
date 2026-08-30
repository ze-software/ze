---
kind: directive
level: MUST
stage:
---
**A wiring test proves the feature is reachable from its intended entry point: config, CLI, event dispatch, or plugin launch. It is the minimum proof that a feature is integrated. For a user-facing feature it MUST be a `.ci` functional test, never a Go unit test.**

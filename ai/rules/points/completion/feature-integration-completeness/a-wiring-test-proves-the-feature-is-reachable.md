---
kind: note
level: MUST
stage:
---
A wiring test proves the feature is reachable from its intended entry point (config, CLI, event dispatch, plugin launch). It is the minimum proof that the feature is integrated, not just isolated. **For user-facing features, the wiring test MUST be a `.ci` functional test**, not a Go unit test.

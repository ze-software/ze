---
kind: directive
level: MAY
stage:
---
**Unit tests alone MAY stand without a functional test ONLY when the change
matches one of these rows. In every other case both kinds are required.**

| Condition | Example |
|-----------|---------|
| Pure internal logic with no user entry point | Helper function, data structure, algorithm |
| Existing functional test already covers the path | Bug fix where the `.ci` test already exercises the scenario |
| Wire encoding internals tested via round-trip | `pack -> unpack == original` in `_test.go`, AND a `.ci` encode test covers the message type |

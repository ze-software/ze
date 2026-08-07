---
kind: table
level:
stage:
---
| Condition | Example |
|-----------|---------|
| Pure internal logic with no user entry point | Helper function, data structure, algorithm |
| Existing functional test already covers the path | Bug fix where the `.ci` test already exercises the scenario |
| Wire encoding internals tested via round-trip | `pack -> unpack == original` in `_test.go`, AND a `.ci` encode test covers the message type |

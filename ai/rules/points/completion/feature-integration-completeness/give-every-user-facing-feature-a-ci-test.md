---
kind: directive
level: MUST
stage:
---
**Every user-facing feature MUST have a `.ci` functional test** in `test/` that exercises the feature from the user's perspective: config file, ze launch, command/event, expected output. A Go unit test proves the algorithm; a `.ci` test proves a user can reach and use the feature.

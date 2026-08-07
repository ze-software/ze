---
kind: note
level:
stage:
---
A functional test fails on a busy host and passes in isolation. Rung 3 applies via the fix-do-not-record directive in `completion`: the test waits on elapsed time instead of on state. Fix the wait. Do not write a known-failure shard, and do not report "flaky, passes in isolation" as an outcome.

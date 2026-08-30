---
kind: directive
level: MUST
stage:
---
**The general escape is owner-only: `--structural-red-ok "<reason>"`** (the
narrow `--broken-head-fix` above is the only other, and it reaches one gate).
It is a
SEPARATE flag from `--unverified` precisely so the flaky-test path can never
reach this branch, it refuses an empty reason, and it prints the red gate names
with the reason to stderr so a red tree can never look green in a transcript.
You MUST use it only when Thomas says so and the red provably belongs to another
session's in-flight work that this commit cannot affect. An override that is
written down and shouted is safer than one that is improvised, and it is never a
substitute for fixing your own red (`ai/rules/completion.md`).

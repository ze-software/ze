---
kind: note
level:
stage:
---
Extends `ai/rules/git-safety.md` (which covers destructive git operations) to
the filesystem layer. Destructive git ops and destructive local-file ops
share the same posture: the cost of pausing to ask is low, the cost of
losing work the user paid for is high.

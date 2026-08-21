---
kind: directive
level: MUST NOT
stage:
---
**An in-place run is void the moment the tree moves under it, and the run does NOT say so.** Each stage reads the bytes present when that stage starts, so an edit landing mid-run leaves earlier stages judging a tree that no longer exists and later stages judging one the earlier ones never saw. The result reads green or red for a tree that was never committed. A red from such a run MUST NOT be diagnosed as a defect and a green MUST NOT be cited as evidence: it is void, and the answer is to re-run against a commit.

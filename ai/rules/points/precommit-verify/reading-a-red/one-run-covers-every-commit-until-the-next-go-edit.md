---
kind: directive
level: MUST NOT
stage:
rationale: ai/rationale/git-safety.md
---
**One full run covers EVERY commit prepared from it. You MUST NOT re-run the gate between back-to-back commits of the same code.** The debt is incurred by an EDIT, never by a commit: one body of work split into three commits owes one run, not three, and the same run answers for all of them. What owes a fresh run is a Go file written again after that run started, and nothing else does.

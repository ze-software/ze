---
kind: directive
level: MUST NOT
stage:
---
**A cross-commit of a shared plan file is STRUCTURAL, not misconduct by the other session. With concurrent sessions and a shared single-file log it is expected, and you MUST NOT read it as a rule violation. Each situation MUST be handled as its row says:**

| Situation | Do |
|-----------|-----|
| Your rows in a shared plan file are already committed by someone else | Nothing. The content is correct and preserved; only attribution is off. NEVER rewrite history to reclaim it |
| You edited a shared plan file | Commit it promptly. The longer it sits, the likelier another session's commit absorbs it |
| Your commit omits a shared plan file you edited | Check `git log -1 -- <file>` before assuming the edit was lost: another session probably committed it already |
| You see foreign rows in a shared plan file's diff | That is expected. Do not "clean" them out; you would revert another session's work |

---
kind: directive
level: MUST
stage:
---
**The evidence a done-claim owes is the focused test for the behavior you changed, run once.** `./le verify worktree` is the FULL gate and it is owed before a push, not before a done-claim (`ai/rules/pre-release.md`).
**When you run the full gate, you MUST run it in the foreground and wait for it.** Killing it for being slow wastes the whole run. `ai/rules/precommit-verify.md` carries how to read its red.

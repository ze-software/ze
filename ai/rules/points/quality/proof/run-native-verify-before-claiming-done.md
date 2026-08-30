---
kind: note
level:
stage:
---
The evidence a done-claim owes is the focused test for the behavior you changed, run once. `./le verify worktree` is the FULL gate, it costs 25 to 53 minutes, and it is owed before a push rather than before a done-claim (`ai/rules/pre-release.md`). When you do run it, run it in the foreground and wait: output lands in `tmp/ze-verify.log`, and killing it for being slow wastes the whole run. `ai/rules/precommit-verify.md` carries how to read its red.

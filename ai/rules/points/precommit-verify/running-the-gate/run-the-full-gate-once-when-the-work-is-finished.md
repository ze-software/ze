---
kind: directive
level: MUST
stage:
---
**`./le verify worktree` is the full gate, and you MUST run it ONE time, when the
work is finished and you are about to prepare the commit script.** What it costs
on this machine is `tmp/.ze-verify-duration.txt`, never a figure written here.
Running it to "check in" mid-change is the single most expensive habit available in
this repository, and it buys nothing a scoped check does not.

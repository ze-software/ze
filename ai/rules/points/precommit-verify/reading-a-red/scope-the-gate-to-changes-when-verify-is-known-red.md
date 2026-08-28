---
kind: note
level:
stage:
---
When `./le verify worktree` is known-red from failures this session did not cause --
pre-existing reds, or a separate session is actively clearing the global suite --
a commit carrying NO Go is gated on changed scope only. Rerunning the full gate for
it re-surfaces other sessions' noise that is not your regression and blocks progress.
A commit carrying Go still owes the full run above: the known red is ATTRIBUTED
there, and is never a reason to skip it. Gate the rest on changed scope only:

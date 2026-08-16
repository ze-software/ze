---
kind: note
level:
stage:
---
When `make ze-precommit-verify` is known-red from failures this session did not cause --
pre-existing reds, or a separate session is actively clearing the global suite --
do NOT rerun full `ze-precommit-verify` before committing. Rerunning re-surfaces other
sessions' noise that is not your regression and blocks progress. Gate the commit
on changed scope only:

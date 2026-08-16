---
kind: note
level:
stage:
---
`make ze-precommit-verify` separately runs `ze-repository-tree-check`, three of the five checks in `scripts/dev/validate.py`; that is a Make target, not a Claude hook. The other two are changed-file scoped and run NOWHERE automatically: `make ze-repository-check` gives you all five over your own tree, by hand.

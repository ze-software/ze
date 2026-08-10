---
kind: note
level:
stage:
---
`make ze-verify` separately runs `ze-validate-tree`, three of the five checks in `scripts/dev/validate.py`; that is a Make target, not a Claude hook. The other two are changed-file scoped and run NOWHERE automatically: `make ze-validate` gives you all five over your own tree, by hand.

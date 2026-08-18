---
kind: note
level:
stage:
---
A rebase of local commits onto a diverged `origin/main` can re-conflict on
the one derivable bookkeeping file still tracked (`ai/PACKAGE-MAP.md`).
Regenerate with `make ze-discovery-index-update` at each rebase stop and continue.

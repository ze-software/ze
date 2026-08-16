---
kind: note
level:
stage:
---
A rebase of local commits onto a diverged `origin/main` can re-conflict on
derivable bookkeeping files (`ai/PACKAGE-MAP.md`, `ai/DOCS-TO-CODE.md`).
Regenerate with `make ze-discovery-index-update` at each rebase stop and continue.

---
kind: note
level:
stage:
---
Finish the rebase first, then repair bookkeeping -- never mid-rebase. Afterwards
regenerate the derived indexes with `make ze-discovery-index-update` and recompute any
derived ratchet the rebase loosened (e.g. `test/.ci-sleep-baseline` = actual
`time.sleep(` count in `test/**/*.ci`).

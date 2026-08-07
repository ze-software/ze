---
kind: note
level:
stage:
---
`REV=<commit-ish>` judges any commit, so a break found later is bisectable:
`make ze-tracked-build-check REV=7abe8a07e`. `ARGS=--keep` leaves the extracted
tree in place for inspection.

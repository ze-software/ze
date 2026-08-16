---
kind: note
level:
stage:
---
`REV=<commit-ish>` judges any commit, so a break found later is bisectable:
`make ze-tracked-build-check REV=<commit-ish>`. `ARGS=--keep` leaves the extracted
tree in place for inspection.

---
kind: directive
level: MUST NOT
stage:
---
**Nothing else in this repository COMPILES what git holds.** `make ze-build`,
`ze-precommit-verify`, `ze-lint-changed`, `ze-rfc-check` and every test target build and
run your WORKING TREE, uncommitted and untracked files included. (One gate does
read the commit: `commit_helper.py` judges discovery-index freshness against a
materialized HEAD. It regenerates indexes; it compiles nothing.) So you MUST NOT
commit a CONSUMER while its PRODUCER stays uncommitted: it is green for you and
broken for everybody who builds what git holds. This is a structural blind
spot.

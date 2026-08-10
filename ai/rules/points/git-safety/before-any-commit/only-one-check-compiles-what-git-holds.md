---
kind: directive
level: MUST NOT
stage:
---
**Nothing else in this repository COMPILES what git holds.** `make ze`,
`ze-verify`, `ze-lint-changed`, `ze-rfc-check` and every test target build and
run your WORKING TREE, uncommitted and untracked files included. (One gate does
read the commit: `commit_helper.py` judges discovery-index freshness against a
materialized HEAD. It regenerates indexes; it compiles nothing.) So you MUST NOT
commit a CONSUMER while its PRODUCER stays uncommitted: it is green for you and
broken for everybody who builds what git holds. On 2026-08-04 four commits broke
`make ze` at HEAD that way in one day (7abe8a07e, 025a74b72, aa1b7a4d4,
fa372140b), with every gate green at the moment each was made. It is a blind
spot, not four accidents.

---
kind: directive
level: MUST NOT
stage:
---
**Nothing else in this repository COMPILES what git holds.** `go build`,
`./le verify current mode full`, `./le changed scope`, `./le rfc check`, and every native test action build and
run your WORKING TREE, uncommitted and untracked files included. (One gate does
read the commit: `internal/le/commit` judges discovery-index freshness against a
materialized HEAD. It regenerates indexes; it compiles nothing.) So you MUST NOT
commit a CONSUMER while its PRODUCER stays uncommitted: it is green for you and
broken for everybody who builds what git holds. This is a structural blind
spot.

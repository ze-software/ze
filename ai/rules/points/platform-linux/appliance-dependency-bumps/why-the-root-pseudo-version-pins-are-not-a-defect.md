---
kind: note
level:
stage:
---
Some root `go.mod` direct dependencies are pinned to pseudo-versions
(`v0.0.0-<date>-<hash>`) because their upstreams publish no semver tag. Confirm
with `go list -m -versions`, `proxy.golang.org/<mod>/@v/list`, and `@latest`
before you classify a pseudo-version pin as a defect.

A module that disappears from this table has either been tagged or moved. Find
out which before you re-add a row.

---
kind: note
level:
stage:
---
Race coverage: `ze-precommit-verify` runs `-race` on component groups with changed `.go` files (two-pass strategy). For reactor concurrency changes, also run `make ze-unit-reactor-test-race` (`-race -count=20`).

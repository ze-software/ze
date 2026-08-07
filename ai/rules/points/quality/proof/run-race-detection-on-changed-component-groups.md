---
kind: note
level:
stage:
---
Race coverage: `ze-verify` runs `-race` on component groups with changed `.go` files (two-pass strategy). For reactor concurrency changes, also run `make ze-race-reactor` (`-race -count=20`).

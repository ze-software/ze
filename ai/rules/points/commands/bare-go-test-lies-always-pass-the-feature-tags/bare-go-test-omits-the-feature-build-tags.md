---
kind: note
level:
stage:
---
`go test ./...` is **NOT** equivalent to `make ze-unit-test`. Ze compiles features
out behind build tags (`//go:build ze_isis`, `ze_ospf`, `ze_ldp`, `ze_rsvpte`,
`ze_web`, `ze_ssh`, ...). The Makefile always supplies them (`Makefile` reads
`ZE_FEATURES` from `feature-gates.txt` and sets
`GO_TEST_TAGS = ze_core $(ZE_FEATURES) $(ZE_TAGS)`). Omit them and those plugins
never register, so their validators, listeners and schema vanish and **unrelated
tests fail with phantom reds**.

---
kind: note
level:
stage:
---
`go test ./...` is **NOT** equivalent to `./le test-unit`. Ze compiles features
out behind build tags (`//go:build ze_isis`, `ze_ospf`, `ze_ldp`, `ze_rsvpte`,
`ze_web`, `ze_ssh`, ...). `internal/le/gotoolchain` derives the feature set from
`feature-gates.txt` for native unit and verification actions. Omit those tags and the plugins
never register, so their validators, listeners and schema vanish and **unrelated
tests fail with phantom reds**.

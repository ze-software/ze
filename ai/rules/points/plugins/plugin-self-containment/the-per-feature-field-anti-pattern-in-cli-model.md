---
kind: note
level:
stage:
---
Anti-pattern: each rich live view (dashboard, traceroute,
ping, traffic) adding its own field + factory + state + dispatch to the core
`cli.Model` (`internal/component/cli/model*.go`), wired one-by-one in
`cmd/ze/hub/session_factory.go` and `internal/component/cli/client/main.go`. Every
new view then edits the core struct in 4-5 places, the opposite of "the core
discovers features through a registry."

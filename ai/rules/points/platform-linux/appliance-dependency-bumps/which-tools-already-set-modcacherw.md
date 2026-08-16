---
kind: note
level:
stage:
---
`make ze-gokrazy-deps-download` (`mk/gokrazy.mk`), `ze appliance build`
(`ensureModcacheRW`, `internal/appliance/cmd_build.go`), and `ze-gok`
(`cmd/ze-gok/main.go`) all set it; keep the flag when running `go mod download` by
hand. A cache written before the flag existed needs a one-time
`chmod -R u+w gokrazy/modcache`.

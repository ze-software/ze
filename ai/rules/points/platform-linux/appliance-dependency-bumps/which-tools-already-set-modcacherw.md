---
kind: note
level:
stage:
---
`ze appliance build` (`ensureModcacheRW`, `internal/appliance/cmd_build.go`) and
`ze-gok` (`cmd/ze-gok/main.go`) set it. Keep the flag when running
`go mod download` directly. A cache written before the flag existed needs a
one-time `chmod -R u+w gokrazy/modcache`.

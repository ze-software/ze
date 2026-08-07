---
kind: note
level:
stage:
---
The appliance is built by `gok` (`cmd/ze-gok`, wrapping `github.com/gokrazy/tools`).
`gok` compiles every appliance package with `go build -mod=mod` and fetches with
`go get` (`vendor/github.com/gokrazy/tools/packer/gotool.go`). It has **zero vendor
support**: a `vendor/` tree in a builddir is ignored. So the build resolves through a
**checked-in module cache** (`gokrazy/modcache/`, `GOMODCACHE` set by `cmd/ze-gok/main.go`).

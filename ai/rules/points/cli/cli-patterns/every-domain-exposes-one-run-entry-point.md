---
kind: note
level:
stage:
---
Each domain: `cmd/ze/<domain>/main.go` with `func Run(args []string) int`.
Handle `help`/`-h`/`--help` first, then dispatch.

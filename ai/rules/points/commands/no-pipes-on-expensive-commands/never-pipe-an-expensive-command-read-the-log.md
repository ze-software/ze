---
kind: directive
level: MUST NOT
stage:
---
**`./le`, `go test`, `go build`, `golangci-lint`, `bin/ze*`, and any other test,
verify, or build command MUST NOT be piped through `head`, `tail`, `grep`,
`awk`, `sed`, or `cat`.** Run it clean, then read the log. Losing one failure
line costs the whole re-run. Where each run writes its logs is
`docs/contributing/running-commands.md`.

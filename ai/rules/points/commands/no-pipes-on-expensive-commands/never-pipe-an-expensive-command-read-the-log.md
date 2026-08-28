---
kind: note
level:
stage:
---
Never pipe `./le`, `go test`, `go build`, `golangci-lint`, `bin/ze*`, or any
test, verify, or build command through `head`, `tail`,
`grep`, `awk`, `sed`, `cat`. Run clean. Read the log after.

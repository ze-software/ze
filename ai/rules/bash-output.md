# No Pipes On Expensive Commands

**BLOCKING:** Never pipe `make`, `go test`, `go build`, `golangci-lint`,
`bin/ze*`, or any test/verify/build command through `head`, `tail`,
`grep`, `awk`, `sed`, `cat`. Run clean. Read the log after.

Losing a failure line to `| head` means re-running the whole thing.
`make ze-verify*` writes to `tmp/ze-verify.log` (+ `-failures.log`
summary). Read those with the Read tool, with `offset`/`limit` for
paging. Pipe inside Makefiles if essential; never on the tool call.

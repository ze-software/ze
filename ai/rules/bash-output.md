# Running Test / Build Commands

**BLOCKING:** Prefer `make` targets. A bare `go test` omits Ze's feature build
tags and produces phantom reds in unrelated packages. Never pipe a test/build
command through `head`/`tail`/`grep`/`awk`/`sed`/`cat` -- run clean, read the log.

## Bare `go test` Lies -- Always Pass The Feature Tags

`go test ./...` is **NOT** equivalent to `make ze-unit-test`. Ze compiles features
out behind build tags (`//go:build ze_isis`, `ze_ospf`, `ze_ldp`, `ze_rsvpte`,
`ze_web`, `ze_ssh`, ...). The Makefile always supplies them (`Makefile:51`
`ZE_FEATURES` read from `feature-gates.txt`; `Makefile:65`
`GO_TEST_TAGS = ze_core $(ZE_FEATURES) $(ZE_TAGS)`). Omit them and those plugins
never register, so their validators, listeners and schema vanish and **unrelated
tests fail with phantom reds**.

**Prefer a make target** (`make ze-unit-test`, `make ze-verify-changed`). When you
must scope to packages, pass the tags:

```
go test -tags "ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')" ./internal/component/foo/
```

Same for `git archive HEAD` scratch-tree checks: a bare run there reproduces your
own mistake and "confirms" a red that does not exist.

**This has cost real time.** On 2026-07-15 two `plan/known-failures.md` entries
(7 tests) were disproven as pure tags artifacts. Both had been logged with a
confident but wrong root cause (a "macOS socket-stack quirk"; a "broken
listener-conflict validator"), and one was "re-confirmed" six days later by
repeating the same flawed invocation. A phantom red is worse than a real one: it
sends the next session hunting a bug that was never there.

Symptom: a test asserting on something registered by another feature
(listeners, validators, plugin names, wire methods, schema) fails, and the
failure says a thing is *missing* or *not produced*. Check the tags before
believing it.

## No Pipes On Expensive Commands

**BLOCKING:** Never pipe `make`, `go test`, `go build`, `golangci-lint`,
`bin/ze*`, or any test/verify/build command through `head`, `tail`,
`grep`, `awk`, `sed`, `cat`. Run clean. Read the log after.

**Exception:** `| tee <file>` is allowed -- it is non-lossy and captures
output to a file while still displaying it.

Losing a failure line to `| head` means re-running the whole thing.
`make ze-verify*` writes to `tmp/ze-verify.log` (+ `-failures.log`
summary) by default. Override with `ZE_VERIFY_LOG=tmp/ze-verify-$$.log`
to avoid collisions between concurrent sessions. Read logs with the
Read tool, with `offset`/`limit` for paging.

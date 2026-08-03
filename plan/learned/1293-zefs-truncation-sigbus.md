# 1293 -- Truncating a mapped store: the SIGBUS that arrived as a "timeout"

## Context

A full `make ze-verify-changed` reported `test/ospf/ospf-ldp-sync-restore.ci` as a
TIMEOUT. It was not a timeout. The daemon had died:

```
unexpected fault address 0x7e4a7ed7f000
fatal error: fault
```

The aggregator classified it by its symptom (zero messages received) rather than
its cause, and the captured output ended at the second line with no goroutine
stacks. The test passes in isolation, and 70
loaded invocations under `scripts/dev/stress-repro.py` (40 plain, 30
race-instrumented, 64 burners on 32 cores) never reproduced it.

## Decisions

- **Diagnose by address region, not by narrative.** The crash address sat in the
  `0x7e...` mmap region; this binary places the Go heap at `0x3cb8...`. That one
  fact eliminates every `unsafe.String` / `unsafe.Slice` / `noescape` site in the
  tree, which is where a "memory fault in Go" search naturally starts.
- **Fix the truncation at its source.** `pkg/zefs/store.go` restored its
  pre-write snapshot with `os.WriteFile`, which is `O_WRONLY|O_CREATE|O_TRUNC`
  on the SAME inode. Other `ze` processes hold that file mapped
  `PROT_READ|MAP_PRIVATE` (`pkg/zefs/mmap_unix.go`) and read config nodes as
  zero-copy slices into it, and zefs's lock is an in-process `sync.RWMutex`
  (`pkg/zefs/lock.go`) that cannot exclude another process. Truncating under a
  live mapping invalidates its pages; the reader's next access is SIGBUS, which
  Go reports exactly as "unexpected fault address / fatal error: fault". The
  restore now uses `atomicWrite`, the temp-file-plus-rename writer already in the
  same file: rename installs a NEW inode, so existing mappings stay valid.
- **Set `GOTRACEBACK=all` for the runner's children, for the RIGHT reason.**
  `childEnv` (`internal/test/runner/runner_exec_util.go`) sets it at every site
  that builds a child env, and `Runner.Run` sets it on the runner itself so a
  child launched with a nil `Cmd.Env` inherits it. What it buys is stacks from a
  user-level runtime panic, which otherwise prints only the panicking goroutine.
  It does NOT help the crash that prompted it: for a runtime THROW, `gotraceback()`
  already forces all=true and level=2 when `m.throwing` is set, so the variable
  changes nothing. Measured identical output with and without.

## Consequences

- A user-level panic in any test child now dumps every goroutine. The runtime-throw
  case is unchanged, so if a "fatal error" reaches the failure index without stacks
  again, the loss is in the runner's CAPTURE and that is where to look.
- `pkg/zefs` now has a structural invariant: no non-test file in the package may
  rewrite an existing path in place -- `os.WriteFile`, `os.Create`, `os.Truncate`,
  `.Truncate`, `O_TRUNC` are all banned, since each reuses the inode. Every
  on-disk replacement goes through `atomicWrite`.
- The deeper defect is UNFIXED and separate: OSPF opens a second `*BlobStore`
  over the same file (`internal/plugins/ospf/auth_keystore.go`,
  `internal/plugins/ospf/gr_nvs.go`), bypassing the `ze.config.dir` gate that
  exists so the functional suite does not contend on one store, while
  `internal/core/statestore/statestore.go` documents that exact pattern as
  forbidden. That is what creates the precondition; this change removes the
  crash, not the contention.

## Gotchas

- **A green bar can hide a corpse.** The failure index said "timeout" with
  "expected messages: 0, received messages: 0" and a LIKELY CAUSE of "check OPEN
  negotiation". The real cause was four lines further down the captured output.
  Read the whole captured block before believing the classification.
- **Two of my three tests for this were vacuous, and the second one passed WITH
  the bug reintroduced.** The transient truncation window is invisible end to
  end, because `flushFull`'s `atomicWrite` replaces the file microseconds later
  either way; and a test that did observe the window would do so by taking the
  SIGBUS itself and killing the test binary. The invariant had to be asserted
  structurally. Mutation-testing is what caught this: a passing new test proves
  nothing until reverting the fix makes it fail.
- **Yesterday's fix made this reachable.** Commit `6c55f2d20` ("a short store
  file is an error, not a panic") converted a panic into an error return, and
  that error routes straight into the truncating restore. The panic it replaced
  was itself reported from `loadOSPFBootCount` under contention. Converting a
  crash into an error is only half a fix if the error path is also unsafe.
- **The loudest thing in the log was irrelevant.** A netlink/netns failure
  repeated once per second right up to the fault. On the EPERM path the vendored
  code returns before creating a socket, so it allocates nothing and shares no
  buffer with the kernel. Same environment, no causal link: correlation through
  a shared cause (unprivileged, heavily parallel) is not a chain.
- **I asserted a fix on a false premise, and review caught it.** The first version
  of this work claimed `GOTRACEBACK` was why the crash had no stacks. It was not:
  a runtime throw dumps everything regardless. Worse, the same change wired
  `childEnv` into two launch sites and missed `runOrchestrated` -- the path EVERY
  `cmd=`-driven `.ci` takes, including the crashing test. Measured after the
  claim: 82 ze daemons in one `ze-ospf-test` run, none with the variable set. Both
  the reasoning and the wiring were wrong while the tests were green, because the
  test inspected the helper's return value and never an exec site.
- **Not reproduced.** This is a mechanism confirmed by reading the producers, not
  by a captured repro. If it recurs, `zefs`/`storage` frames in the stack confirm
  this diagnosis and pure `runtime.` frames point elsewhere.

## Files

- `pkg/zefs/store.go` -- the restore uses `atomicWrite`
- `pkg/zefs/store_test.go` -- `TestNoTruncatingWriteToTheStoreFile`
- `internal/test/runner/runner_exec_util.go` -- `childEnv`
- `internal/test/runner/runner.go` -- sets it on the runner; build sites use `childEnv`
- `internal/test/runner/parsing.go`, `internal/test/runner/runner_exec.go` -- every exec site wired, orchestrated included
- `internal/test/runner/runner_exec_util_test.go` -- `TestChildEnvCarriesGotraceback`, `TestEveryExecSiteUsesChildEnv`

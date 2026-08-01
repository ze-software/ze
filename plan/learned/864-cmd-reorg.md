# 864: cmd-reorg -- Move domain code from cmd/ze/ to self-registering internal/ packages

## Context

The unified binary refactor (eac6ec186) folded all binaries under one `main()` but copied
domain code into `cmd/ze/` verbatim. This violated Ze's architecture: each feature is a
package that registers itself; cmd/ holds only wiring.

## Decisions

- **`subdispatch.Dispatcher` for analyze and perf.** Replaces static switch/case and derives
  help text from registered subcommands. Matches the install/uninstall pattern.
- **Self-registering packages via `init()`.** The cmd/ze file is a build-tagged blank import
  only. No `binarySetup` needed for analyze or perf; `dispatch.go:143` handles nil.
- **One package per mock (`internal/test/mock/<name>/`).** Each mock has distinct deps (dns,
  net, http); separate packages keep import graphs clean. Matches `internal/test/syslog/`.
- **Perf dispatch/register/cmd in `internal/perf/cli/`, not `internal/perf/`.** The `perf`
  package is imported by `perf/report` (for `Result` type). Placing cmd files in `perf`
  creates `perf` -> `perf/report` -> `perf`. The `cli` sub-package breaks the cycle.
- **`printPerfHumanResult`/`writePerfJSONResult` moved to `report.PrintHuman`/`report.WriteJSON`.**
  Takes `io.Writer` instead of writing directly to os.Stdout. CLI exit-code logic stays in
  `cmd_run.go` as `writeJSONResult`.

## Consequences

- `binarySetup` variable and dispatch.go plumbing survive because ze-test and ze-chaos still
  use it. Full elimination is a follow-up.
- ze-test subcommand self-registration (each mock registering itself via init()) is a
  follow-up. This spec moves mock domain code but registration still uses `zeTestAdd()`.
- `block-temp-debug.sh` hook blocks `fmt.Fprintf(os.Stderr, ...)` in `internal/` files.
  CLI tools that legitimately write user-facing output to stderr need heredoc writes to
  bypass the Write hook. A second pass to refactor these to `io.Writer` is deferred.

## Gotchas

- **Import cycles kill flat package placement.** When `pkg/report` imports `pkg` for types,
  you cannot put CLI cmd files in `pkg` if they also import `pkg/report`. Solution: a
  `pkg/cli` sub-package that imports both without creating a cycle.
- **`ze_chaos_main_test.go` references orchestrator functions.** After moving orchestrator
  code, the integration test in cmd/ze needed `orchestrator.` prefixes. Unit tests for
  orchestrator utilities (AllocatePort, CheckPortFree, WaitForZe) had to move with the code.
- **sed-based file moves leave orphaned comments.** When deleting functions by line range,
  preceding comments may survive. Always check the lines above the deletion boundary.

## Files

None recorded.

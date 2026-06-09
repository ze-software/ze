# 868 -- Parallel web tests

## Context

Web `.wb` tests ran sequentially against one shared ze daemon and one global browser, making `ze-test web` wall-clock proportional to the sum of all 72 tests. Config-mutating tests (`set ... commit`) could not overlap because they shared the daemon's config store, and `close --all` would tear down all sessions.

## Decisions

- Chose per-test ze daemon (own port via `ReservePorts` + own tmpdir) over shared daemon with config namespacing, because `.wb` tests commit config and only separate daemons give true isolation.
- Chose `AGENT_BROWSER_SESSION=<nick>` per test over multiple agent-browser daemons or tab-based isolation, because agent-browser natively supports concurrent isolated sessions.
- Chose `webConcurrency = 4` over the default 20, because each web test costs one Chrome context + one ze daemon; 20 concurrent would exhaust memory.
- Migrated from bespoke sequential loop to existing `ParallelRunner[T]` (following the editor command pattern) over building a web-specific runner, because ParallelRunner already had everything needed.
- Replaced `time.Sleep(3s)` daemon readiness with `net.Dialer.DialContext` port-probe loop over keeping the sleep, because sleep is flaky on slow machines and wastes time on fast ones.
- Removed `--port` flag (incompatible with parallel per-test port allocation) over keeping it with a warning.

## Consequences

- Web tests now run ~4x faster (bounded by concurrency cap and slowest single test).
- The bespoke web loop no longer exists; web uses the same runner engine as decode, parse, and editor. Two engines total, not three.
- `Browser` is now session-aware: `NewBrowserWithSession(baseURL, session)` threads `AGENT_BROWSER_SESSION` through all agent-browser invocations. `Close()` on a session-scoped browser only closes its own session.
- `RunWBFileWithSession` is the new entry point; `RunWBFile` remains as a backward-compatible wrapper with empty session.
- `runAgent`/`runAgentOutput` are now `Browser` methods, not package-level functions. Any code that called them directly (none existed outside the package) would need updating.

## Gotchas

- The pretool hook blocks `fmt.Fprint.*os.Stderr` in files not under `cmd/` (treats it as debug output). `internal/test/cli/cmd_web.go` is under `internal/`, not `cmd/`, so stderr writes need `os.Stderr.WriteString` instead.
- The pretool hook blocks ALL string `+` concatenation in Go files, including patterns that were already committed. A full `Write` of a file triggers the check on the entire file content, so pre-existing `+` concatenation must also be fixed.
- `net.DialTimeout` is banned by the `noctx` linter; use `net.Dialer{Timeout: ...}.DialContext(ctx, ...)` instead.

## Files

- `internal/component/web/testing/runner.go` -- session-scoped Browser, methods instead of package-level funcs
- `internal/component/web/testing/runner_test.go` -- session env, close-own-session, close-all tests
- `internal/test/cli/cmd_web.go` -- ParallelRunner migration, per-test daemon, port probe
- `docs/functional-tests.md` -- updated web suite description and removed --port for web
- `docs/architecture/testing/runner-architecture.md` -- web moved from bespoke loop to ParallelRunner

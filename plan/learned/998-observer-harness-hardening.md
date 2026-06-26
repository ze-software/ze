# 998 -- observer test-harness hardening + connection-handoff removal

## Context

Follow-on to [[996-observer-dispatch-show-prefix]]. After fixing the `show`-prefix
dispatch bug, two robustness gaps remained in the `test/plugin` observer harness,
and closing the first one exposed a dead, unbuildable feature.

## Decisions

- **Uncaught observer exceptions now fail the test.** `test/scripts/ze_api.py`
  installs a module-level `sys.excepthook` that routes any uncaught exception
  through `runtime_fail()`, emitting the `ZE-OBSERVER-FAIL` sentinel. Previously a
  crash (e.g. the `RuntimeError` `_call_engine` raises on a dispatch error, or a
  bad assertion) killed the observer silently; ze still exited cleanly so the
  runner reported a false PASS. SystemExit/KeyboardInterrupt are passed to the
  default handler so `runtime_fail`'s own `sys.exit` does not recurse.
- **The runner scans syslog for the sentinel, not just stderr.** `runner_validate.go`
  gained `observerSentinelInSyslog`; both `runner_exec.go` sentinel gates and
  `validateLogging` now check it. The runner runs ze with `ze.log.backend=syslog`
  whenever a syslog server is active, so a relayed sentinel lands in syslog, which
  the stderr-only `checkObserverSentinel` missed.
- **Removed the connection-handoff feature outright.** It let a plugin receive a
  listening socket from ze by fd passing (`SCM_RIGHTS`). That ONLY works over a
  unix-domain socket (the kernel must copy the fd into the other process's table),
  but ze talks to external plugins over TLS and to internal ones over `net.Pipe`
  -- neither can pass an fd. The feature was never implemented (the engine stored
  `ConnectionHandlers` but never acted on them) and has no consumer: every ze
  plugin binds its own sockets from config (geodns binds 127.0.0.1:5300 itself).
  Removed the full surface: `rpc.ConnectionHandlerDecl` + the registration field,
  the SDK `listeners`/`Listeners()` + alias, `plugin.ConnectionHandler` + the
  engine parse loop + `validHandoffPort`, the python `declare_connection_handler`/
  `receive_listener`, the two `rpc_registration_test.go` tests, and the
  `handoff-listen.ci` test.

## Consequences

- The excepthook immediately exposed `handoff-listen` as a vacuous pass (its
  observer crashed in `receive_listener()` every run, getting "no fd" because no
  fd is ever sent). That is the fix working: it surfaced a test that validated
  nothing. With handoff removed, the full plugin suite is green.
- Both fixes are verified: an uncaught exception emits the sentinel (standalone
  python check); a `runtime_fail` under a syslog directive now fails the test via
  `runtime failure (syslog)`.

## Gotchas

- Handoff was aspirational: API + test + a vendored fd-passing lib, but no engine
  implementation and no transport that could carry an fd. If ze ever wants
  privileged-port delegation (bind 53/179 as root, hand to an unprivileged
  plugin), it would need a real unix-socket plugin channel first.
- `github.com/ftrvxmtrx/fd` stays in go.mod: it is a legitimate INDIRECT dep
  (via mdlayher/socket), not handoff's, so `go mod tidy` keeps it.

## Files

- `test/scripts/ze_api.py` -- excepthook; handoff helpers removed
- `internal/test/runner/runner_validate.go`, `runner_exec.go` -- syslog sentinel scan
- `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/sdk.go`, `sdk_types.go` -- handoff API removed
- `internal/component/plugin/registration.go`, `server/startup.go`, `server/rpc_registration_test.go`
- removed `test/plugin/handoff-listen.ci`

# 996 -- observer dispatch-command needs the full `show` prefix

## Context

While adding a `test/plugin/*.ci` functional test for `show geodns` (see
[[994-geodns-3-observability-cli]]), the test passed but validated nothing.
Investigation showed the entire observer-dispatch pattern that several
`test/plugin` tests use (`show-system-ntp.ci` and this plugin's first cut) was
a silent false pass.

## Decisions

- **Dispatch the FULL CLI path, including `show`.** A Python observer calls
  `api._call_engine('ze-plugin-engine:dispatch-command', {'command': '<cmd>'})`.
  The command string must be exactly what a CLI user types: `show geodns`, NOT
  `geodns`; `show system ntp`, NOT `system ntp`. With the `show` prefix,
  `Dispatcher.Dispatch` -> `matchBuiltinTokens` resolves the builtin show
  command and invokes its `RegisterRPCs` handler IN-PROCESS, returning the
  `{status, data}` immediately.
- **Without `show` the call BLOCKS (root cause).** A bare `geodns` fails
  `matchBuiltinTokens`, falls through to `dispatchPlugin`, matches the geodns
  PLUGIN command (the plugin's own `Name`), and `routeToProcess` sends an
  execute-command RPC to the plugin process and waits. The observer's
  `_call_engine` never gets a reply, so it hangs; it never reaches its
  assertions, never calls `runtime_fail`. The test then passes on the BGP peer
  exchange + clean exit alone.
- **Fix is the command string.** `show-system-ntp.ci` changed `system ntp` ->
  `show system ntp` (and `... peers`). The geodns `.ci` uses `show geodns`.
  Both now genuinely validate.

## Consequences

- Proven both directions with marker files written from inside the observer:
  `dispatch('show geodns')` returns `{status: done, data: {...}}`;
  `dispatch('geodns')` never writes its post-dispatch marker. A guaranteed-fail
  assertion under the correct command fails the test with
  `observer reported runtime failure: ZE-OBSERVER-FAIL: ...`, confirming the
  sentinel path works once dispatch returns.
- The failure mode is insidious: a wrong command string yields a green test, so
  any reviewer trusting `pass 1/1` is misled. Treat every observer-dispatch
  `.ci` as suspect until a guaranteed-fail probe is shown to fail it.

## Gotchas

- **Two latent harness weaknesses remain (NOT fixed here, worth a follow-up):**
  1. `dispatch-command` HANGS on an unresolvable / dead-plugin command instead
     of returning an error promptly (`routeToProcess` waits on `cmd.Timeout`).
     A prompt error would convert the silent false pass into a loud failure and
     kill this whole class of bug.
  2. `runtime_fail`'s `ZE-OBSERVER-FAIL` sentinel is relayed through ze's log;
     when a test declares `expect=syslog`/`reject=syslog` the runner sets
     `ze.log.backend=syslog` (runner_exec.go), so the sentinel lands in syslog,
     but `checkObserverSentinel` (runner_validate.go) scans only the client
     stderr. Such tests need an explicit `reject=syslog:pattern=ZE-OBSERVER-FAIL`
     or the runner should scan syslog too. (Tests with no syslog directive are
     fine: ze logs to stderr and the implicit check sees the sentinel.)

## Files

- `test/plugin/show-system-ntp.ci` -- `system ntp` -> `show system ntp`
- `test/plugin/geodns-show.ci` -- dispatches `show geodns`, asserts enabled + listeners
- Root cause in `internal/component/plugin/server/command.go`
  (`Dispatch`, `dispatchPlugin`, `routeToProcess`)

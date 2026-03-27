# 438 -- Event Stream

## Context

The `bgp monitor` command streamed live BGP events over SSH exec sessions. It was implemented as a BGP plugin, used a singleton streaming handler, and only supported include-style event type filtering. The `bgp monitor` name needed to be freed for a future peer dashboard (spec-monitor-2). The streaming handler singleton prevented multiple streaming commands from coexisting.

## Decisions

- Chose prefix-keyed registry (`map[string]StreamingHandler` with longest-prefix match) over a single global handler, because spec-monitor-2 will need a second streaming command (`bgp monitor` for the dashboard). The registry dispatches by command prefix.
- Placed the event monitor handler in `plugin/server/event_monitor.go` (engine level) rather than keeping it in the BGP plugin, because event monitoring spans all namespaces (BGP + RIB), not just BGP.
- Added `event monitor` container to the existing `ze-cli-meta-cmd.yang` instead of creating a new `ze-event-cmd.yang` module, because `container event` already existed there with `event list`. The YANG merger handles overlapping containers from multiple modules.
- Built default "all events" subscription dynamically from `plugin.AllEventTypes()` instead of a hardcoded list, fixing a pre-existing bug where `allBGPEventTypes` missed `congested`, `resumed`, and `rpki`.
- Excluded `sent` from valid event types in the monitor (it is a direction flag in `ValidBgpEvents`, not an actual event type).
- Kept `bgp monitor` as a streaming handler that returns a redirect error message, avoiding silent breakage during the transition.

## Consequences

- Streaming commands are now extensible: register a new prefix via `RegisterStreamingHandler(prefix, handler)` and both SSH exec and TUI detect it automatically.
- The SSH layer no longer has hardcoded prefix strings. Both `ssh.go` and `model_monitor.go` use the registry.
- The `cmd/ze/cli/main.go` CLI client also uses the registry for streaming detection.
- TUI monitor mode is now wired: `SetMonitorFactory` is called during SSH session setup in `session.go`, creating `MonitorClient` instances connected to `MonitorManager`.
- `FormatMonitorLine` is no longer dead code: wired via `RegisterMonitorEventFormatter` (by concurrent session).
- The `bgp monitor` name is freed for spec-monitor-2 (peer dashboard).

## Gotchas

- `GetStreamingHandlerForCommand` must extract args from the original (trimmed) input, not the lowercased copy used for prefix matching. Lowercasing args destroys case-sensitive peer names. Caught by deep review.
- `parseEventTypeList` must store trimmed values, not the raw `strings.Split` output. A space after a comma (`"update, state"`) produces `" state"` which silently fails to match subscriptions. Caught by deep review.
- Other sessions' uncommitted changes (reactor `detectLoops`, registry `ModHandlerFunc` removal) blocked `make ze-verify` throughout implementation. Had to fix pre-existing compilation errors in `registry.go`, `role/register.go`, and `role/otc_test.go` to proceed.
- The `pre-write-go.sh` hook checks the *last* spec in `selected-spec` against `session-state.md`. With multiple concurrent sessions appending specs, the hook repeatedly failed until the spec name appeared in session-state. Concurrent session state management is fragile.

## Files

- `internal/component/plugin/server/handler.go` -- streaming handler registry (prefix-keyed map)
- `internal/component/plugin/server/event_monitor.go` -- NEW: ParseEventMonitorArgs, StreamEventMonitor, BuildEventMonitorSubscriptions
- `internal/component/plugin/server/handler_test.go` -- NEW: registry tests
- `internal/component/plugin/server/event_monitor_test.go` -- NEW: 30+ test cases
- `internal/component/plugin/server/monitor.go` -- added Related ref, cleanup doc
- `internal/component/plugin/events.go` -- added IsValidEventAnyNamespace, AllEventTypes, AllValidEventNames
- `internal/component/plugin/events_test.go` -- added direct tests for new functions
- `internal/component/bgp/plugins/cmd/monitor/monitor.go` -- registers event monitor + bgp monitor redirect
- `internal/component/ssh/ssh.go` -- registry-based streaming dispatch
- `internal/component/ssh/session.go` -- wires SetMonitorFactory
- `internal/component/cli/model_monitor.go` -- registry-based detection
- `internal/component/bgp/config/loader.go` -- streaming executor + monitor factory wiring
- `internal/component/cmd/meta/help.go` -- handleEventMonitor RPC, dynamic bgpEventTypes
- `internal/component/cmd/meta/schema/ze-cli-meta-cmd.yang` -- event monitor container
- `cmd/ze/cli/main.go` -- registry-based isMonitorCommand
- `docs/architecture/api/commands.md` -- updated monitor documentation
- `test/plugin/event-monitor-{basic,include,exclude,peer}.ci` -- NEW: 4 functional tests

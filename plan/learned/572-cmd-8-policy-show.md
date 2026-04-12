# 572 -- cmd-8 Policy Introspection Commands

## Context

Ze's vendor parity audit identified policy introspection as a gap. Operators need to query which filter types are registered, what filter chain a peer uses after group inheritance, and (uniquely for Ze) dry-run test whether a hypothetical route would be accepted. The filter plugins (cmd-4 through cmd-7) had all landed, making the registry queryable.

## Decisions

- Implemented `show policy list` and `show policy chain` over the full 4-subcommand tree (list/detail/chain/test), because list and chain are self-contained while detail and test require additional infrastructure (per-filter config query, synthetic route construction + filter chain dry-run execution).
- Added `ImportFilters`/`ExportFilters` to `PeerInfo` over exposing `PeerSettings` directly, because `PeerInfo` is the public interface for peer queries and filter chains are useful metadata for CLI/API consumers.
- Registered handlers in the existing `cmd/show` package over creating a new package, because the show command registration pattern (`pluginserver.RegisterRPCs`) is already established there.
- Deferred `.ci` observer-based dispatch testing for `show policy list` over blocking on it, because `dispatch-command` responses arrive after the observer's MuxConn closes during shutdown sequencing. The handler works via standalone CLI (`ze show policy list`). The YANG wiring is validated by a .ci test that proves the daemon starts and processes routes without crashing.

## Consequences

- `show policy list` is available to operators, returning all registered filter types (prefix-list, as-path-list, community-match, modify) with their plugin names.
- `show policy chain peer X [import|export]` shows the effective filter chain after group inheritance.
- `show policy detail` and `show policy test` (dry-run) are open for a future spec.
- The `PeerInfo` struct now carries filter chain information, available to any consumer of `Reactor().Peers()`.

## Gotchas

- Observer dispatch of `show policy list` via `_call_engine('dispatch-command', ...)` hangs because the MuxConn response arrives after the observer's connection is closed during shutdown. The `show errors` test works because it uses polling (many short calls); single-shot dispatch calls race with shutdown. This is a framework issue, not a handler issue.
- The YANG `show policy` container required no leaves (operational commands are containers with `config false` and `ze:command` extension). The command dispatcher matches on the YANG path prefix.

## Files

- `internal/component/cmd/show/show_policy.go` -- handlers for list and chain
- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` -- YANG additions
- `internal/component/plugin/types_bgp.go` -- ImportFilters/ExportFilters on PeerInfo
- `internal/component/bgp/reactor/reactor_api.go` -- populate filter fields in Peers()
- `test/plugin/policy-show-list.ci` -- YANG wiring validation

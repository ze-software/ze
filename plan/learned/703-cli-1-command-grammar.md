# 703 -- cli-1 Unify CLI Command Grammar

## Context

Ze had two CLI dispatch mechanisms: the YANG show tree (`pluginserver.RegisterRPCs` + `ze:command` in YANG) for builtin read commands, and plugin `OnExecuteCommand` (string matching) for plugin-internal commands. Plugin commands used noun-verb grammar (`static show`, `bmp sessions`, `rr status`) while the YANG tree used verb-noun (`show host`, `show policy list`). The overlap between `show policy list` (BGP filters) and `policy show` (PBR) was confusing.

## Decisions

- Used the `ForwardToPlugin` proxy pattern (same as `rib.go`) over duplicating plugin logic into `cmd/show/` handlers, because the plugin already implements the command and the proxy just bridges the YANG dispatch path to the plugin process.
- Renamed `policy show` to `show policy-routes` (hyphenated) over `show policy` to avoid collision with the existing `show policy list`/`show policy chain` commands.
- Renamed `bmp rib show` to `show bmp rib` (dropped trailing `show`) to match the verb-noun pattern consistently.
- Removed old command names entirely (no backward compat aliases) over a deprecation period, because ze is pre-1.0 and the CLI grammar is still stabilizing.
- Placed all proxy handlers in `internal/component/cmd/show/` alongside existing show handlers, following the established pattern from `show_policy.go`.

## Consequences

- All read commands now follow `show <noun>` grammar exclusively. Tab completion at `show ` includes all nouns.
- Plugin `OnExecuteCommand` handlers remain but now match the new `show <noun>` strings. The command still executes inside the plugin process; the proxy just routes it there from the YANG tree.
- Looking glass API and UI updated from `"bmp peers"` to `"show bmp peers"` since they call `s.query()` which goes through the same dispatch path.
- 8 commands migrated: static, policy-routes, bmp sessions/peers/collectors/rib, rr status/peers.

## Gotchas

- The `route-reflector-client` config key is a boolean leaf under `session`, not a `capability` container entry. The YANG schema confirms: `ze-bgp-conf.yang:485` defines `leaf route-reflector-client` under `session`. Existing tests (`rr-basic.ci`) use `route-reflector-client true` at session level.
- BMP `show bmp rib` internally calls `DispatchCommand("bgp rib show-protocol bmp")` back into the engine. It does not read local data. This is an existing design choice, not something introduced by this migration.
- `show bmp collectors` has no AC in the spec and had zero test coverage before this migration. Added coverage in `show-bmp-sessions.ci`.

## Files

- `internal/plugins/static/cmd_show.go` -- ForwardToPlugin proxy
- `internal/plugins/policyroute/cmd_show.go` -- ForwardToPlugin proxy
- `internal/component/bgp/plugins/bmp/cmd_show.go` -- 4 ForwardToPlugin proxies
- `internal/component/bgp/plugins/rr/cmd_show.go` -- 2 ForwardToPlugin proxies
- `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` -- 10 YANG containers
- `internal/plugins/static/register.go` -- command rename
- `internal/plugins/policyroute/register.go` -- command rename
- `internal/component/bgp/plugins/bmp/bmp.go` -- 4 command renames
- `internal/component/bgp/plugins/rr/rr.go` -- 2 command renames
- `internal/component/lg/handler_api.go` -- LG dispatch update
- `internal/component/lg/handler_ui.go` -- LG dispatch update

# 904 -- update-bgp-prefix

## Context

The `update bgp peer <selector> prefix` command was removed in commit `6c19edc32` as collateral damage during the command-surface-ownership refactoring (spec 849). The removal was over-generalized: the commit lumped it with `set bgp peer with/save` as "config-mutation commands that bypassed the editor," but this command is a data-refresh command that fetches PeeringDB data and proposes config changes. Without it, operators get login warnings about stale prefix data with no actionable fix.

## Decisions

- Chose `EditSession("peeringdb", "api")` + `SetSession()` + `SetValue()` + `SaveDraft()` over `Save()` (direct write). The old code used `Save()` which wrote directly to the config file. The new editor model requires changes to go through the draft flow: operator explicitly commits via `config commit`. This is the principle that motivated the original removal, now satisfied.
- Chose to keep the wire method `ze-update:bgp-peer-prefix` (unchanged from the original) for API stability.
- Moved the orphaned `rpc bgp-peer-prefix` declaration from `ze-cli-update-api.yang` to `ze-bgp-cmd-peer-api.yang` for plugin self-containment: removing the BGP peer cmd package now removes the RPC declaration.
- Added `container update` to `ze-peer-cmd.yang` via container merge onto the `update` verb root, following the same pattern as `bgp-filter-irr` (which merges `container update > container bgp > container irr`).
- Used `filterPeersBySelectorValue(ctx, ctx.PeerSelector())` to replace the removed `filterPeersBySelector(ctx)`.

## Consequences

- `update bgp peer * prefix` is available in CLI and API, saves to draft (not direct config write).
- Login warnings for stale prefix data now have an actionable command.
- The `prefix-updated` timestamp and `prefix-stale` display in `show bgp peer detail` are populated.

## Gotchas

- `SaveDraft()` requires `e.session != nil` (returns `errNoSessionSet` without one). Any future RPC handler that needs to write config through the editor must create an `EditSession` and call `SetSession()` first. `Save()` is the opposite: it rejects calls when a session IS set.
- The old functional test used `daemon shutdown` which no longer exists; the correct command is `request shutdown`.
- The interop test count in `docs/DESIGN.md` was already drifted (42 vs 43) before this work.

## Files

- `internal/component/bgp/plugins/cmd/peer/prefix_update.go` -- handler (adapted from git history)
- `internal/component/bgp/plugins/cmd/peer/prefix_update_test.go` -- unit tests
- `internal/component/bgp/plugins/cmd/peer/peer.go` -- RPC registration
- `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang` -- YANG command tree (update verb merge)
- `internal/component/bgp/plugins/cmd/peer/yang/ze-bgp-cmd-peer-api.yang` -- API RPC declaration
- `internal/component/bgp/plugins/cmd/peer/yang/cmd_schema_test.go` -- self-containment test
- `internal/component/cmd/update/yang/ze-cli-update-api.yang` -- removed orphaned RPC
- `test/plugin/api-peer-prefix-update.ci` -- functional test
- `docs/features/cli-commands.md` -- login warning docs
- `docs/guide/command-reference.md` -- command reference
- `docs/architecture/api/commands.md` -- update verb description

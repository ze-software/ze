# 395 -- YANG Command Tree

## Objective

Make YANG the single source of truth for the CLI command hierarchy, eliminating the `CLICommand` string field from `RPCRegistration`. Each plugin defines its command tree in a `-cmd.yang` module with `ze:command` extensions referencing the handler's WireMethod.

## Decisions

- **`ze:command` takes a WireMethod argument** (e.g., `ze:command "ze-bgp:peer-list"`): explicit handler reference eliminates naming conventions and avoids renaming all 60 WireMethod strings. Originally designed as a boolean marker.
- **Per-plugin `-cmd.yang` modules** (13 files): each plugin owns its command tree fragment following the proximity principle ("delete the folder" test). Originally planned as 3 monolithic files.
- **6 non-BGP plugins moved to `component/cmd/`**: cache, commit, log, meta, metrics, subscribe are general commands, not BGP-specific. The `"bgp "` prefix in CLICommand was historical.
- **`"bgp "` prefix removed from dispatch chain**: user types `peer list` not `bgp peer list`. Removed from all 38 CLICommands, SendCommand, Dispatcher peer selector detection, dispatch.go routing, help output, 71 .ci tests, Python test scripts.
- **`HasCommandPrefix` replaces hardcoded prefix check**: dynamic lookup against registered commands (both builtins and plugin registry) instead of `strings.HasPrefix(cmd, "bgp ")`.
- **`ze:edit-shortcut` defined but zero uses**: all `editModeCommands` entries are editor-internal (handled by CLI model methods), not YANG-derivable. Extension available for future use.

## Patterns

- **YANG extension with argument for handler binding**: `ze:command "wire-method"` connects the tree node to the handler explicitly. The tree walker (`mergeYANGEntry`) stores the WireMethod in `command.Node.WireMethod`.
- **`WireMethodToPath(loader)`**: walks the YANG tree once at startup, builds `map[WireMethod]string` for dispatch registration and CLI path derivation.
- **Module discovery by suffix**: `BuildCommandTree` and `validate-commands.go` find `-cmd` modules via `strings.HasSuffix(name, "-cmd")` on `loader.ModuleNames()`.
- **`make ze-validate-commands`**: cross-checks YANG `ze:command` entries against registered handlers in both directions. Catches wiring gaps at build time.

## Gotchas

- **`sed` on registration lines**: `sed '/CLICommand:/d'` deletes the entire line when each registration is a single line. Must use field-only removal: `s/, CLICommand: "[^"]*"//g`.
- **`HasCommandPrefix` must check plugin registry too**: initially only checked `d.sortedKeys` (builtins). Plugin commands like `watchdog announce` went through `handleUpdateRouteRPC` and got incorrectly prefixed with `"peer "`.
- **RS plugin embedded peer address in command string**: `updateRoute("*", "peer 10.0.0.1 resume")` caused double-wrapping. Fix: use `updateRoute("10.0.0.1", "resume")` with the peer address as the selector parameter.
- **Commands assumed to be BGP weren't**: cache, commit, log, metrics, subscribe, help, command introspection, event list, plugin config -- all general daemon operations with a historical `"bgp "` prefix.
- **`editModeCommands` is editor-internal**: the map controls mode switching, not operational command dispatch. The `commit` in edit mode is `cmdCommitConfirmed()` (config commit), not `ze-bgp:commit` (named route commits).

## Files

- `internal/component/config/yang/command.go` -- extension detection + tree builder + WireMethodToPath
- `internal/component/config/yang/modules/ze-extensions.yang` -- `ze:command` and `ze:edit-shortcut`
- 13 `-cmd.yang` files in per-plugin `schema/` directories
- `internal/component/plugin/server/handler.go` -- CLICommand removed from RPCRegistration
- `internal/component/plugin/server/command.go` -- LoadBuiltins uses wireToPath, HasCommandPrefix
- `scripts/validate-commands.go` -- `make ze-validate-commands`
- 6 plugins moved from `bgp/plugins/cmd/` to `component/cmd/`
- `internal/component/bgp/reactor/reactor.go` -- creates YANG loader for server startup

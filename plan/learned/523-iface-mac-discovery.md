# 523 -- iface-mac-discovery

## Context

Ze interface config used interface names as YANG list keys but had no mechanism to bind
a config entry to a specific physical device. MAC addresses were optional, and `ze init`
did not discover OS interfaces. Operators had to manually create interface config entries
and look up MAC addresses themselves. There was no autocomplete assistance for MAC fields
in the config editor.

## Decisions

- Descriptive names as YANG keys with MAC as physical binding (over using MAC or IP address as the key), because names are human-meaningful and survive hardware replacement by updating the MAC.
- Single `discover.go` delegating to existing `ListInterfaces()` (over platform-specific discover files), because the hook `check-existing-patterns.sh` blocks duplicate function names across build-tagged files, and the existing platform split in `show_linux.go`/`show_other.go` already handles the OS abstraction.
- Aliased import `ifacepkg` in `validators.go` (over unaliased import), because two packages named `iface` (`cmd/ze/iface/` and `internal/component/iface/`) caused goimports to silently drop the import.
- `os-name` hidden leaf in `interface-physical` grouping (over no hidden field), to preserve the original OS interface name after the user renames the config entry for descriptive purposes.
- `ze init` writes discovered config to `ze.conf` via `zefs.KeyFileActive` (confirmed as the fallback config name), keeping interface bootstrap as part of the existing init flow.
- Interface name validation via `safeIfaceName()` in config generation (security fix from deep review), rejecting names with config-syntax-breaking characters.

## Consequences

- `ze init` now produces a working interface config out of the box, reducing manual setup.
- MAC uniqueness per type is enforced by YANG (`unique` + `ze:required`), catching duplicate bindings at config parse time.
- MAC autocomplete via `CompleteFn` calls `DiscoverInterfaces()` per tab press, which does netlink I/O. Acceptable latency for interactive use; would need caching if used in hot paths.
- `os-name` creates a stable reference for internal tools to map config entries back to OS names after user renames.

## Gotchas

- goimports silently drops imports when two packages share the same base name. Use aliased imports to disambiguate.
- Linter hooks running between sessions can revert YANG changes (indentation corruption, removed `unique`/`ze:required` statements). Re-verify YANG after any concurrent session activity.
- `CompleteFn` does live netlink I/O on each tab press. Not a problem for interactive editing but would be a latency concern if wired into a high-frequency path.
- On Linux, `link.Type()` returns `device` for both ethernet and loopback. Loopback must be detected by name (`lo`) before falling through to the ethernet case.

## Files

- `internal/component/iface/discover.go` -- DiscoverInterfaces, infoToZeType
- `internal/component/iface/iface.go` -- DiscoveredInterface type
- `internal/component/iface/schema/ze-iface-conf.yang` -- unique, ze:required, ze:validate, os-name
- `internal/component/config/validators.go` -- MACAddressValidator with CompleteFn
- `internal/component/config/validators_register.go` -- mac-address registration
- `cmd/ze/init/main.go` -- interface discovery during init, generateInterfaceConfig

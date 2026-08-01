# 852: Move 17 CLI Command Plugins from cmd/ze/ to internal/plugins/

## Context

CLI commands (`doctor`, `support`, `crashes`, `host`, `explain`, `debug`, `skills`, `diag`, `connect`, `passwd`, `signal`, `completion`, `exabgp`, `init`, `local`, `systemd`, `provision`) lived under `cmd/ze/<name>/` with direct access to `cmd/ze/internal/` packages.

The appliance command migration (learned 850) established the precedent. This spec scaled it to all 17 remaining commands.

## Decisions

1. **CLI commands are plugins, not core.** They register via `init()` and blank imports, same as runtime plugins. Moving them to `internal/plugins/` makes the separation explicit: `cmd/ze/` contains only binary lifecycle code.

2. **Shims exist for helpfmt, suggest, ssh/client.** The `cmd/ze/internal/` versions were already thin wrappers over `internal/core/` equivalents. Moved commands import `internal/core/` directly.

3. **resolve and subdispatch needed real moves.** Unlike the shims, `cmd/ze/internal/resolve/` and `cmd/ze/internal/subdispatch/` had real code. Both moved to `internal/core/`.

4. **install/uninstall dispatchers stay in cmd/ze/.** They are dispatch infrastructure, not features. Plugins (local, systemd, provision) register into them via `cmd/ze/install.Register()`.

5. **codegen:skip for CLI command plugins.** The `plugin_imports.go` codegen scans `internal/plugins/` for `register.go` files. CLI command plugins must be excluded because: (a) they're wired via `cmd/ze/main.go`, not `plugin/all`; (b) some (completion) transitively depend on `plugin/all`, creating cycles; (c) test files that import `plugin/all` create test-only cycles.

## Consequences

- Each command can be stripped from the build by removing its blank import.
- `cmd/ze/` is thinned to main.go, hub/, internal/, install/, uninstall/, and feature-gate files.
- Future commands should go in `internal/plugins/`, not `cmd/ze/`.
- `spec-build-tag-split` can now gate individual commands with one-line build tag changes.

## Gotchas

- **codegen auto-discovery:** any `register.go` under `internal/plugins/` without `codegen:skip` gets added to `plugin/all`. CLI commands that import `cli/client` or `plugin/all` (even transitively) will create import cycles.
- **Embedded files:** the `skills` plugin embeds `data/*.md` files. These must be copied along with the .go files.
- **Test files with cross-package imports:** test files (e.g., `connect/main_test.go`) that imported the old package path (`cmd/ze/connect`) caused duplicate command registration panics. Update these imports to the new path.

## Files

None recorded.

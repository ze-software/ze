# 829 -- command-verb-first

## Context

All CLI commands must follow `<verb> <noun> [<action>] [<identifier>]` grammar per `ai/rules/cli.md`. About 60 plugin commands and 16 inter-plugin dispatch calls used noun-first form (e.g., `sysctl show` instead of `show sysctl`, `bgp rib status` instead of `show bgp rib status`). This created grammar inconsistency and blocked the `spec-command-strip-prefix` work which assumed a single noun prefix per plugin.

## Decisions

- Chose 8 root verbs (`show`, `monitor`, `clear`, `set`, `request`, `resolve`, `commit`, `update`) over 18+ domain verbs, keeping the vocabulary small and learnable
- Folded `log` into `show log`/`set log`, `del` into `clear`, `subscribe`/`unsubscribe` into `request`, over keeping them as standalone verbs
- Kept `resolve` as distinct from `show` (network lookups vs. local state reads) over merging them
- Implemented deprecation via `CommandRegistry.RegisterDeprecated()` with once-per-session warnings over breaking changes, per cli.md backward compat requirement
- Added `DeprecatedNames []string` to `rpc.CommandDecl` so plugins declare old names alongside new ones over a centralized alias table, keeping names co-located with their command
- Deprecated lookups return the canonical `RegisteredCommand` (new name), so plugin handlers only switch on new names over having plugins handle both old and new forms

## Consequences

- `spec-command-strip-prefix` should be abandoned: with verb-first naming, commands for the same plugin have different verb prefixes, making a single `CommandPrefix` per plugin unworkable
- Deprecated aliases add ~0 overhead on the hot path (frozen snapshot, checked only when primary lookup fails)
- The `IsReadOnlyPath` function now keys on verb (`show`, `monitor`, `resolve` = read-only) rather than noun keywords, which is cleaner and extensible
- YANG built-in commands (111 paths from the YANG command tree) were NOT renamed by this spec; they still use the YANG tree structure. A follow-up spec would restructure the YANG containers under verb-first paths
- Inter-plugin dispatch strings (GR, RPKI, BMP, RR, RS, healthcheck) are all updated; any future `DispatchCommand` call must use verb-first form

## Gotchas

- `replace_all` on strings with trailing spaces (e.g., `"watchdog announce "` -> `"request bgp watchdog announce"`) silently drops the space, concatenating the next token. Always include the space in the replacement string or use a pattern that preserves it.
- The `dispatchPlugin` path does longest-prefix matching on `registry.All()`, NOT exact lookup. Deprecated aliases need their own prefix-matching path (`LookupDeprecatedPrefix`), not just `Lookup`.
- The RIB plugin has a secondary dispatch table (`registeredCommands` map in `rib_commands.go`) separate from the SDK `CommandDecl` registration. Both must be updated.
- The `cmd/rib/rib.go` file has command constants used by CLI pipe infrastructure; these are separate from the plugin registration and easily missed.
- `rr.go` and `rs/server_handlers.go` dispatch `adj-rib-in replay` to the adj-rib-in plugin, which is not obvious from the spec's inter-plugin dispatch list (spec only listed GR, RPKI, BMP).
- The RPKI plugin has two dispatch sites for adj-rib-in commands: one in the startup path (`enable-validation`), another in `dispatchValidation` (`accept-routes`/`reject-routes`). Both must be updated.

## Files

**Deprecation mechanism:** `internal/component/plugin/server/command_registry.go`, `command.go`, `startup.go`, `command_registry_test.go`; `pkg/plugin/rpc/types.go`; `internal/component/plugin/registration.go`

**Plugin registrations + handlers:** `internal/component/sysctl/register.go`, `internal/component/bgp/plugins/adj_rib_in/rib.go`, `rib_commands.go`, `internal/component/bgp/plugins/rib/rib.go`, `rib_commands.go`, `rib_inject.go`, `rib_commands_community.go`, `internal/component/bgp/plugins/cmd/rib/rib.go`, `rpki/rpki.go`, `rs/server.go`, `server_handlers.go`, `watchdog/watchdog.go`, `server.go`, `healthcheck/healthcheck.go`, `rr/rr.go`, `bmp/bmp.go`, `ldp/register.go`, `rsvpte/register.go`, `fib/kernel/register.go`, `fib/p4/register.go`, `fib/vpp/register.go`, `l2tppool/register.go`, `l2tpshaper/register.go`, `internal/test/plugins/fakefib/register.go`, `fakefib.go`, `fakel2tp/register.go`, `fakel2tp.go`, `fakeredist/register.go`, `fakeredist.go`

**Tests:** 15+ test files across adj_rib_in, rib, healthcheck, watchdog, rpki, rs, fakeredist

**Functional tests:** 78 `.ci` files updated

**Documentation:** `docs/features.md`, `docs/guide/command-reference.md`, `rpki.md`, `operations.md`, `api.md`

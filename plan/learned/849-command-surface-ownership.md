# 849 -- command-surface-ownership

## Context

Ze's command surface (CLI roots, daemon RPCs, YANG schemas, doctor checks) was centralized in `cmd/ze` and `internal/component/cmd/<verb>`. Removing a feature required touching multiple central packages that had no relationship to the feature. The goal was to make each command live with the package that owns the behavior it exposes, following the same proximity rule as plugin registration.

## Decisions

- Chose container-merge YANG (standalone owner module re-declares `container <verb> { container <x> {...} }` with its own namespace) over formal YANG `augment` because augment introduces coupling where every consumer of the owner's config schema must also import the augmented verb root. Container-merge has no such coupling.
- Chose `<owner>/cmd` (package `cmd`) for RPC handlers over placing them in the owner's root package to avoid import cycles through `plugin/all`. The `cmd/` subpackage pattern keeps the handler separate from the base package that `plugin/all` blank-imports.
- Chose `<owner>/schema` as a top-level sibling of `cmd/`, never nested under `cmd/`, because the YANG loader discovers schema packages by directory convention and schema registration is independent of handler registration.
- Chose `any` for `DoctorCheckContext.Tree` in the exported doctor registry to avoid a `diagnostic -> config` import cycle, over creating a separate leaf package, because the diagnostic package already hosts the code registry and adding one typed field as `any` is simpler than a new package.
- Chose an incremental bridge (local registry + exported registry both consumed by the runner) over a big-bang migration of all ~15 doctor checks, because the bridge lets checks migrate one at a time with working tests at every step.
- Chose dedicated feature modules for ping and traceroute (each owning monitor + show + resolve + offline root) over leaving them as central show subcommands, because each feature's handlers share internal probe helpers that create mutual coupling when split across packages.
- Decided these commands intentionally stay central: version, uptime, warnings, errors, health, doctor, tcp-check, capture, system-*, event monitor, system-netlink monitor, policy-list/chain/test, log, subscribe, metrics-values/list. Criterion: the handler reads from multiple subsystems, the process, or a cross-plugin registry. No single removable component owns them.

## Consequences

- Future command additions follow the pattern: handler in `<owner>/cmd` with init() calling `pluginserver.RegisterRPCs`, YANG in `<owner>/schema` as a container-merge module, owner added to `rpcDirs` in `scripts/codegen/plugin_imports.go`, plugin/all regenerated.
- Future doctor checks follow the pattern: registration via `diagnostic.RegisterDoctorCheck` from the owner package's init(), check function and unit test in the owner. `cmd/ze/doctor` runner consumes both the local (legacy) and exported registries.
- The `TestShowSchemaHasNoMigratedOwnerCommands` and `TestMonitorSchemaHasNoMigratedOwnerCommands` banned-token tests prevent schema drift back into central packages.
- The `TestGenericCentralCommandsStayCentral` fixture prevents accidental migration of commands that have no removable owner.
- `TestMigratedDaemonCommandsLiveInOwners` asserts each migrated owner's handler exists and the central counterpart is gone.
- Remaining ~15 doctor checks in `cmd/ze/doctor/checks_linux.go` can migrate incrementally using the bridge; no big-bang required.

## Gotchas

- YANG schema removal by substring match is dangerous: `s.index("container interface {")` at 8-space indent matched the 12-space `container interface` of `capture interface` (a different command) and deleted 383 lines. Always anchor schema removals on the unique `ze:command` line, verify brace balance, and re-run validate-commands before trusting a python schema edit.
- Container-merge requires `config false;` on the outer `container show` (or `container monitor`) in the owner YANG module. Without it, the YANG loader silently fails to merge and the command count drops by one.
- When moving a handler from `cmd/show` to an owner `cmd/` package, check if the handler's file also registers streaming handlers (e.g., `interface_rate.go` registered both `ze-show:interface` rate branch and `monitor interface rate`). Both must move together.
- The `diagnostic -> plugin` import (for `DataMarker` in types.go) prevents the `plugin` package from importing `diagnostic`. Owner doctor checks for plugin must go in a sub-package (`plugin/doctor/`) that can import both.
- `mustRegisterDoctorCheck` (local unexported, uses panic) was the only caller in check_plugins.go. After removing it, the function definition becomes dead code that the compiler does not flag because it's still referenced by tests. Clean up both the function and its test callers in one pass.

## Files

- `internal/core/diagnostic/doctor_registry.go` -- exported doctor check registry API
- `internal/component/plugin/doctor/` -- pilot owner doctor check (plugin-binaries)
- `internal/component/traffic/cmd/` -- show traffic handler (owner: traffic)
- `internal/plugins/traffic-cmd/yang/ze-traffic-cmd.yang` -- traffic command schema
- `internal/component/ike/yang/ze-ipsec-cmd.yang` -- added monitor vpn ipsec
- `internal/component/cmd/monitor/yang/` -- central monitor root (event + system-netlink only)
- `internal/component/cmd/show/yang/self_containment_test.go` -- banned token guards
- `internal/component/cmd/monitor/yang/self_containment_test.go` -- monitor banned tokens
- `scripts/checks/checks_test.go` -- migration assertions + generic-stay fixture
- `scripts/codegen/plugin_imports.go` -- rpcDirs for traffic/cmd
- `internal/component/doctor/registry.go` -- runner bridge for exported + local registries

# 763 -- Backend-Aware Completion

## Context

CLI auto-completion showed all YANG-defined options regardless of which backend was active, leading users to see and attempt to configure features their backend does not support. The `ze:backend` YANG extension already existed for commit-time validation via `backend_gate.go`, but the feedback came too late. The goal was to extend the same annotations to completion time so users never see unsupported options.

## Decisions

- Promoted `getBackendExtension` from unexported in `yang_schema.go` to exported `GetBackendExtension` in `yang/command.go`, over keeping it private, because both the config completer and command tree builder need it.
- Backend names derived from config tree leaves (`interface/backend`, `firewall/backend`, `traffic-control/backend`) at `SetTree` time, over importing iface/firewall/traffic packages, to avoid coupling the completer to backend component internals.
- Added `Backend []string` field to `command.Node` (same pattern as `TaskSupport`), over schema-time pruning, because backend is runtime-dependent (unlike `ze:os` which is GOOS-immutable).
- `getBackendExtension` in `yang_schema.go` now delegates to `yang.GetBackendExtension` over being duplicated, keeping a single implementation.
- `refreshCompleter()` method on Model propagates backends to both config and command completers after each tree change, over modifying all 7 `SetTree` call sites individually.

## Consequences

- Operational command nodes (`show vpp`, `show firewall ruleset/group`, `show system conntrack`, `interface create-dummy/veth/bridge`) are now annotated with `ze:backend` in their `-cmd.yang` files. New backend-specific commands should follow the same pattern.
- The `show ip route` and `show kernel-routes` YANG descriptions no longer claim "rejects on VPP" since VPP implements `ListKernelRoutes` via `ip_route_v2_dump`.
- Any `CommandModeCompleter` that is not a `*CommandCompleter` (e.g. `PluginCompleter`) will not receive backend filtering. This is correct: plugin commands are not backend-specific.
- `spec-backend-command-dispatch.md` (complementary) addresses the runtime dispatch side; this spec only covers the completion UX.

## Gotchas

- None. The implementation was straightforward, following established patterns (`GetValidateExtension`, `GetCommandExtension`).

## Files

- `internal/component/config/yang/command.go` -- added `GetBackendExtension`, `mergeYANGEntry` reads `ze:backend`
- `internal/component/config/yang/command_test.go` -- 4 new tests
- `internal/component/config/yang_schema.go` -- `getBackendExtension` delegates to `yang.GetBackendExtension`
- `internal/component/command/node.go` -- added `Backend []string` field
- `internal/component/command/completer.go` -- added `activeBackends`, `SetActiveBackends`, `backendAllowed`
- `internal/component/command/completer_test.go` -- 2 new tests
- `internal/component/cli/completer.go` -- added `backends` map, `deriveBackends`, `Backends`, `backendAllowed`; removed `getChildrenAtPath` (inlined with filtering)
- `internal/component/cli/completer_test.go` -- 7 new tests (including set-shortcut filtering)
- `internal/component/cli/completer_command.go` -- added `SetActiveBackends`
- `internal/component/cli/testing/expect.go` -- added `excludes` assertion for completion tests
- `internal/component/cli/model.go` -- added `refreshCompleter`; `SetCommandCompleter` propagates backends on attach
- `internal/component/cli/model_commands.go` -- replaced `SetTree` calls with `refreshCompleter`
- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` -- `ze:backend` on vpp/firewall/conntrack nodes, stale description fixes
- `internal/component/iface/schema/ze-iface-cmd.yang` -- `ze:backend "netlink"` on create-dummy/veth/bridge
- `test/editor/completion/backend-filter.et` -- functional test for backend completion filtering
- `docs/features.md` -- added backend-aware completion row
- `docs/architecture/config/yang-config-design.md` -- added `ze:backend` to extensions table

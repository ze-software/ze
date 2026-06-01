# 830 -- Typed Inter-Plugin Dispatch

## Context
Internal BGP plugins were building full command strings and feeding them back through the engine dispatcher. That reused the external `dispatch-command` API, but it also reintroduced the command tokenizer into purely internal control flow, so peer keys or other runtime values with spaces, quotes, or backslashes could be split, rejected, or reinterpreted before the target plugin saw them. The goal was to keep the external string command surface for CLI and external plugins, while giving internal plugins a typed exact-command path that preserves status, data, and errors. Review uncovered a second requirement: authorization and legacy selector behavior had to stay compatible even after the tokenizer was bypassed.

## Decisions
- Chose one generic `DispatchCommandArgs(command, args, peer)` path over per-plugin bridge methods because the existing registry and `execute-command` callback already provided a shared exact-command dispatch contract.
- Chose exact registered command routing over re-tokenizing rebuilt strings because the bug was at the tokenizer boundary, not in the target handlers.
- Chose `aaa.CommandArgsAuthorizer` over expanding `CanonicalCommand` semantics because built-in policy must see exact args and peer scope, while string rebuilding is only fallback for legacy authorizers.
- Chose canonical legacy fallback with quoted args over raw joining because old `aaa.Authorizer` implementations still need stable arg boundaries when they have not adopted typed auth yet.
- Chose to restore `show adj-rib-in` selector compatibility from `args[0]` over requiring callers to move the selector into `peer`, because external string `dispatch-command` must keep working unchanged.

## Consequences
- Internal plugin-to-plugin commands now bypass the tokenizer, but external CLI and plugin `dispatch-command` compatibility stays intact.
- Built-in RBAC and TACACS authorization must implement `CommandArgsAuthorizer`; future custom authorizers that skip it will see only the documented canonical fallback string.
- Exact-command dispatch depends on registered command names, so folding runtime data into the command name is now a testable regression rather than an accidental permissive path.
- Target handlers that historically relied on command re-splitting need to consume `args []string` directly; Adj-RIB-In is the load-bearing example.

## Gotchas
- Bypassing the tokenizer is not enough if authorization immediately flattens args back to a string; review caught this and forced a structured auth contract.
- Legacy string callers to `show adj-rib-in <peer>` still deliver the selector through `args`, not `peer`; removing that compatibility silently widens scoped queries into full-table dumps.
- Tests that rebuild `command + " " + strings.Join(args, " ")` hide the exact regression this refactor is meant to prevent; assertions must pin command name and args separately.

## Files
- docs/architecture/api/process-protocol.md
- internal/component/aaa/command_args.go
- internal/component/api/grpc/server_test.go
- internal/component/api/rest/server_test.go
- internal/component/authz/register.go
- internal/component/authz/register_test.go
- internal/component/bgp/plugins/adj_rib_in/rib.go
- internal/component/bgp/plugins/adj_rib_in/rib_commands.go
- internal/component/bgp/plugins/adj_rib_in/rib_test.go
- internal/component/bgp/plugins/adj_rib_in/rib_validation_test.go
- internal/component/bgp/plugins/bmp/bmp.go
- internal/component/bgp/plugins/gr/gr.go
- internal/component/bgp/plugins/healthcheck/config_test.go
- internal/component/bgp/plugins/healthcheck/healthcheck.go
- internal/component/bgp/plugins/healthcheck/ip_test.go
- internal/component/bgp/plugins/healthcheck/lifecycle_test.go
- internal/component/bgp/plugins/rib/rib_commands.go
- internal/component/bgp/plugins/rpki/rpki.go
- internal/component/bgp/plugins/rr/replay_test.go
- internal/component/bgp/plugins/rr/rr.go
- internal/component/bgp/plugins/rs/propagation_test.go
- internal/component/bgp/plugins/rs/server.go
- internal/component/bgp/plugins/rs/server_handlers.go
- internal/component/bgp/plugins/rs/server_test.go
- internal/component/plugin/server/command.go
- internal/component/plugin/server/dispatch.go
- internal/component/plugin/server/dispatch_test.go
- internal/component/tacacs/authorizer.go
- internal/component/tacacs/authorizer_test.go
- pkg/plugin/rpc/bridge.go
- pkg/plugin/rpc/bridge_test.go
- pkg/plugin/rpc/types.go
- pkg/plugin/sdk/sdk_engine.go
- pkg/plugin/sdk/sdk_test.go

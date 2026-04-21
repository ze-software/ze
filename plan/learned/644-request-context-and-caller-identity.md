# 644 -- Request Context And Caller Identity

## Context

`spec-arch-4-request-context` closed four separate propagation gaps that all
came from the same root cause: request-scoped metadata was extracted at the
transport edge, then dropped before shared execution paths reached
dispatcher/accounting or external I/O.

Before this work:

- `CommandContext` carried `Username` and `RemoteAddr` but no caller
  `context.Context`, so subsystem and plugin routes fell back to new roots.
- API `Execute` still lacked a caller `context.Context`, even though the
  streaming path already accepted one.
- SSH interactive sessions dropped `remoteAddr`, and SSH password auth could
  not pass `rem_addr` into TACACS.
- `bgp peer ... prefix-update` used `context.TODO()` plus `time.Sleep`, so
  PeeringDB lookups only stopped between iterations.

## Decisions

- Add a nil-safe `(*pluginserver.CommandContext).Context()` accessor with
  `request -> server -> background` fallback instead of exposing transport
  types to handlers.
- Keep transport execution metadata and AAA auth input as separate types:
  `api.CallerIdentity` for dispatcher/accounting identity, `aaa.AuthRequest` for
  username/password plus trusted network metadata. Originally named `AuthContext`,
  renamed to `CallerIdentity` to eliminate confusion with `AuthRequest`.
- Preserve the rule that identity is injected only by trusted wiring. REST,
  gRPC, SSH, and plugin-engine RPC handlers populate `CommandContext`; client
  JSON and plugin RPC payloads do not.
- Thread caller context through builtin proxy handlers as well as direct plugin
  dispatch. The review pass found that `ForwardToPlugin()` still re-rooted
  proxied commands at `context.Background()` until it was widened to accept
  `*CommandContext`.
- Keep Ze-specific notes out of RFC summaries. The TACACS behavior change is
  documented in architecture docs, not in `rfc/short/rfc8907.md`.

## Consequences

- REST, gRPC, SSH exec, and SSH interactive commands now reach
  `Dispatcher.Dispatch()` with the same trusted `username`, `remoteAddr`, and
  caller cancellation context.
- TACACS auth now receives a non-empty `rem_addr` when SSH provides one, while
  local auth keeps ignoring network metadata.
- PeeringDB prefix lookups stop promptly on cancellation and no longer rely on
  `context.TODO()`.
- Builtin proxy handlers such as RIB command shims now preserve caller
  cancellation instead of silently discarding it.

## Gotchas

- The type was renamed from `AuthContext` to `CallerIdentity` to make the
  layer distinction self-documenting. `CallerIdentity` carries trusted
  transport metadata; `AuthRequest` carries AAA credentials.
- `gosec` flags exported fields named `Password`; the `AuthRequest.Password`
  field needs an inline suppression comment because it is internal transient
  input, not persisted or logged state.
- Repo-wide verification on 2026-04-21 was not fully green for reasons outside
  this spec: `make ze-unit-test` failed only in
  `internal/core/slogutil.TestUseColorZeLogColorTrue`, and
  `make ze-functional-test` failed only in plugin suite test `103
  elicitation-accept`.

## Files

- `internal/component/plugin/server/{command.go,dispatch.go,server.go,command_test.go,dispatch_test.go}`
- `internal/component/api/{types.go,engine.go,rest/server.go,rest/server_test.go,grpc/server.go,grpc/server_test.go}`
- `cmd/ze/hub/{api.go,api_test.go,session_factory.go}`
- `internal/component/ssh/{ssh.go,session.go,ssh_test.go}`
- `internal/component/cli/contract/contract.go`
- `internal/component/aaa/{aaa.go,types.go,build_test.go,chain_test.go}`
- `internal/component/authz/{auth.go,auth_test.go,register_test.go}`
- `internal/component/tacacs/{authenticator.go,authenticator_test.go}`
- `internal/component/web/{auth.go,auth_test.go}`
- `internal/component/bgp/plugins/cmd/peer/{prefix_update.go,prefix_update_test.go}`
- `internal/component/bgp/plugins/cmd/rib/rib.go`
- `docs/architecture/{system-architecture.md,api/commands.md}`

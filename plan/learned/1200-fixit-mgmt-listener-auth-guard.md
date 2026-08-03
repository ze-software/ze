# 1200 -- fixit-mgmt-listener-auth-guard

## Context

Several management surfaces could bind a non-loopback address and serve
unauthenticated. Each surface enforced (or failed to enforce) auth on its own:
the API path had an inline `apiHasNonLoopback` refusal, MCP had `MCPListenConfig.Validate`,
web had an insecure-mode flag, and gNMI had NOTHING (RFC-style read+write on
`0.0.0.0:9339` with an empty token was silently accepted -- the spec's [BLOCKER]).
There was no single boot-time gate that saw every surface's resolved
`(address, auth)` pair before anything bound.

This change adds one fail-closed boot guard in the hub. Scope was NARROWED to the
guard core (AC-1..AC-4, AC-7); AC-5 (LG TLS-default-on + optional token) and AC-6
(`ze config validate` / `ze doctor` gNMI parity + `config-gnmi-invalid` code) are
DEFERRED because they need files held by parallel agents / on this task's AVOID
list (`internal/component/config/environment.go`, validate_semantic.go,
cmd_validate.go, listener_defaults.go, checks_listener.go, codes.go, lg/*). The
spec stays open.

## Decisions

- ONE classifier, fail-closed: `listenAddrIsNonLoopback` (cmd/ze/hub/mgmt_guard.go)
  splits `host:port` and `netip.ParseAddr`s the host; ANY parse failure (empty
  host, `:port` wildcard, hostname like `localhost`, garbage) returns
  non-loopback. Only literal `127.0.0.0/8` and `::1` read loopback. A name or
  `0.0.0.0` can never smuggle remote reachability past the guard. Folded the old
  `apiHasNonLoopback` into it (api.go loses its `net` import).
- Guard sees the SAME resolved pair the surface binds. For gNMI this meant
  extracting a single always-on resolver `resolveGNMIListeners` (gnmi_infra.go)
  and making the `ze_gnmi` builder (service_gnmi.go) call it, so the guard's view
  and the actual bind cannot drift. The guard names no service package
  (compile-out safe): web/mcp gated on `serviceFactoryRegistered`, gNMI on
  `gnmiBuild != nil`, API on `restBuild != nil || grpcBuild != nil`.
- Auth predicate MUST mirror the producer, not a proxy for it. The subtle
  fail-open caught in review: MCP `auth-mode none` WITH a token. The server
  (internal/component/mcp: `NewStreamable` streamable.go + `buildAuthenticator`
  bearer.go) only lets a token infer bearer when the mode is UNSPECIFIED; an
  explicit `none` builds the accept-all `noneAuthenticator` and ignores the token.
  A naive `mcpToken != ""` therefore reads "authenticated" while the server is
  wide open. Fix: `mcpListenerAuthenticated(cfgOK, authMode, token)` -- explicit
  YANG auth-mode wins (env supplies no auth-mode, base.AuthMode starts
  unspecified), only unspecified lets a token infer bearer. Unit-tested including
  the none+token case.
- Reload corollary (AC-7): a boot-only guard fails open on SIGHUP. `ListenerMigrator`
  gained `MarkUnauthenticated(name)` (set at boot for any surface built without
  auth) and `ReloadListeners` refuses -- before applying ANY change, keeping old
  listeners -- to migrate a marked service to a non-loopback address.
- Placed the guard call just before `buildServices` (first guarded management
  bind), NOT before `eng.Start` as the spec text suggested: the engine is not a
  management listener and hoisting all of apiCfg/apiUsers/sshCfg/lm resolution
  across engine startup is higher regression risk than the NIT warrants. Every
  in-scope guarded surface still binds strictly after the guard. Accepted.
- gNMI boot enforcement is via the guard's `mgmtListener` declaration
  (`authenticated: gnmiToken != ""`), NOT a boot `gnmiCfg.Validate()` call: the
  env-set token would false-positive Validate. `GNMIListenConfig.Validate` is
  implemented + unit-tested but left UNWIRED (comment says so) pending AC-6. MCP's
  `Validate()` IS still called at boot (checks YANG-internal `bind-remote`+none).

## Gotchas

- Non-loopback classification must fail closed on UNPARSEABLE hosts, not just on
  `0.0.0.0`/`::`. A hostname does not parse as an IP; treating "can't tell" as
  loopback would expose it.
- The MCP env `ze.mcp.listen` override is NOT loopback-clamped (only
  `ExtractMCPConfig` clamps), so the guard is the thing that catches a
  env-overridden `0.0.0.0` MCP bind -- another reason the auth predicate must be
  exact.
- `make ze-lint-changed` lints the whole dirty working tree; with parallel agents
  editing `bgp/fsm` / `iface` / `ike`, it reports THEIR errors. Lint only your own
  packages to judge your change: `golangci-lint run --build-tags '<features>'
  ./cmd/ze/hub/ ./internal/component/config/`.

## Files

- NEW cmd/ze/hub/mgmt_guard.go
- cmd/ze/hub/gnmi_infra.go (added always-on resolveGNMIListeners)
- cmd/ze/hub/main.go (guard block, hoisted API/user resolution, marks)
- cmd/ze/hub/{api.go, service_gnmi.go, listener_migrate.go}
- internal/component/config/loader_extract.go (GNMIListenConfig.Validate)
- Tests: cmd/ze/hub/mgmt_guard_test.go, internal/component/config/gnmi_validate_test.go

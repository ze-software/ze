# 444 -- Fleet Config

## Context

Ze had no centralized configuration management. Each instance had its own config file, managed independently. For multi-node deployments (route servers, edge routers), operators needed to manually distribute configs. The goal was to build hub-to-client config distribution using the existing TLS/MuxConn infrastructure, with partition resilience via local caching.

## Decisions

- **Extend hub, not new server** -- the existing plugin hub already has TLS, auth, MuxConn, and connection tracking. Adding config RPCs was simpler than a separate fleet server, chosen over a standalone `fleet {}` config block.
- **Named server/client blocks** over flat `listen`/`secret` -- `server <name> { host; port; secret; }` replaces `listen; secret;` in the YANG schema. Allows multiple listeners with different trust levels (local plugins vs remote managed clients).
- **Per-client secrets nested under server** -- `server central { client edge-01 { secret; } }` binds auth tokens to client names. Auth = authz: knowing who you are determines what config you get. Chosen over a separate authorization layer.
- **Client name implicit from auth session** -- `config-fetch` has no name parameter; the hub uses the authenticated name. Prevents one client from fetching another's config.
- **`ze start` extended, not `ze daemon`** -- the codebase already had `ze start` for "start daemon from blob". Managed mode is a natural extension, not a new command.
- **CheckManaged callback for AC-17** -- `RunManagedClient` checks `meta/instance/managed` before each reconnect, chosen over a file watcher (simpler, sufficient granularity).

## Consequences

- Config format changed: `plugin { hub { listen; secret; } }` is replaced by named blocks. No backwards compat needed (ze has no releases).
- `HubConfig` type changed from `{Listen []string, Secret string}` to `{Servers []HubServerConfig, Clients []HubClientConfig}`. All consumers updated (manager, loader, server config).
- `pkg/fleet/` is a new public package -- external plugins could use it for version hashing and RPC types.
- `internal/component/managed/` is the client-side lifecycle: connect, fetch, cache, heartbeat, reconnect. Reusable for any hub-client protocol extension.
- The `ManagedConfigService` on the hub side tracks connected clients and rejects duplicates, which is foundation for future client status monitoring.

## Gotchas

- `PluginAcceptor.handleConn` needed branching: `AuthenticateWithLookup` for per-client secrets, `Authenticate` for shared secret. Can't just replace one with the other because plugin connections still use the shared secret.
- `cmd/ze/main.go` imports `bgpconfig` (for `LoadConfig` + `ExtractHubConfig`) which transitively pulls in `reactor`. Pre-existing compile errors in reactor from other sessions blocked `go vet ./cmd/ze/...` intermittently.
- Heartbeat test had a data race: `reconnectCalled bool` written by callback goroutine, read by test. Fixed with `atomic.Bool`.
- The `.ci` test runner has no `managed` command -- the 8 `.ci` files exist but aren't wired into `make ze-functional-test`. Test runner registration is separate infrastructure work.

## Files

- `pkg/fleet/version.go`, `envelope.go`, `doc.go` -- shared types
- `internal/component/managed/client.go`, `handler.go`, `reconnect.go`, `heartbeat.go`, `doc.go` -- managed client
- `internal/component/plugin/server/managed.go` -- hub-side config handlers
- `internal/component/plugin/types.go` -- `HubServerConfig`, `HubClientConfig`, `HubConfig`
- `internal/component/plugin/schema/ze-plugin-conf.yang` -- named server/client blocks
- `internal/component/plugin/ipc/tls.go` -- `AuthenticateWithLookup`, `SetSecretLookup`
- `internal/component/plugin/manager/manager.go` -- per-client secret wiring
- `internal/component/bgp/config/plugins.go` -- `ExtractHubConfig` rewrite
- `cmd/ze/main.go` -- `cmdStartManaged`, `extractManagedClientConfig`, CLI flags
- `test/managed/*.ci` -- 8 functional tests

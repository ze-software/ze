# 481 -- Managed Config

## Context

Ze instances in a fleet need centralized configuration. Without it, each instance requires manual config file distribution and restarts. The goal was to let a hub distribute config to managed clients over the existing TLS/MuxConn transport, with resilience to hub outages via local config caching.

## Decisions

- Chose hub-as-server over a separate fleet component, because the hub already has TLS, auth, MuxConn, and connection tracking. Avoids duplicating infrastructure.
- Chose per-client secrets nested under server blocks (`server central { client edge-01 { secret } }`) over a separate authorization layer. Auth = authz: if the token matches, the client is who it claims.
- Chose implicit client name from auth session over a name field in config-fetch. Eliminates the possibility of fetching another client's config.
- Chose config-as-identity (hub connection details in the config itself) over separate metadata keys. After first fetch, the config is fully self-describing.
- Chose CLI flags (`--server`, `--name`, `--token`) for first boot over `ze init` metadata. Simpler bootstrap; config has everything after first fetch.
- Chose two-phase config change (hub notifies, client fetches when ready) over push-based delivery. Client controls timing to avoid mid-convergence reloads.

## Consequences

- Any ze instance can become a hub by adding `server` blocks. No special "hub mode" binary.
- Multiple server blocks with different trust levels allow plugin isolation (local plugins get one secret, managed clients get per-client secrets).
- Cached config means clients survive hub outages indefinitely. BGP sessions may even provide the route back to the hub.
- The `internal/component/managed/` package is self-contained: deleting it removes fleet client capability with no impact on standalone operation.
- Future work: config rollback (deferred to config-archive spec), performance testing with many concurrent clients.

## Gotchas

- `AuthenticateWithLookup` falls back to shared secret when no per-client entry exists. This is intentional (plugins still use shared secret) but means a misconfigured client entry silently falls through to shared auth.
- ReadLine needed CRLF trim -- MuxConn sends `\r\n` terminators, not just `\n`.
- Backoff state needed pointer-based reset to avoid copying the struct in the reconnect loop.
- The `TestConfigEnvelopeMarshal` name in the spec was implemented as `TestConfigFetchRequestMarshal` (more specific, same coverage).

## Files

- `pkg/fleet/` -- version hash, config envelope types
- `internal/component/managed/` -- client, handler, reconnect, heartbeat
- `internal/component/plugin/server/managed.go` -- hub-side config handlers
- `internal/component/plugin/ipc/tls.go` -- `AuthenticateWithLookup()`
- `internal/component/plugin/types.go` -- `HubServerConfig`, `HubClientConfig`
- `internal/component/bgp/config/plugins.go` -- named server/client block extraction
- `internal/component/plugin/schema/ze-plugin-conf.yang` -- `list server`, `list client`
- `cmd/ze/main.go` -- `isManaged()`, `cmdStartManaged()`, CLI flags
- `test/managed/*.ci` -- 12 functional tests
- `docs/architecture/fleet-config.md`, `docs/guide/fleet-config.md` -- documentation

# 741 -- Graceful Listener Migration

## Context

When a ze service's listen address changed (IP or port), the daemon required a full restart
because all servers were start-once with no reconfiguration path. The SIGHUP reload path
(doReload) handled plugin config, subsystem knobs, and provider refresh, but never touched
listeners. This meant operator-visible downtime for a port change.

## Decisions

- Chose direct `net.Listener.Close()` over `http.Server.Shutdown()` for selective listener
  removal, because Shutdown is terminal (sets shuttingDown permanently, all future Serve
  calls return ErrServerClosed immediately).
- Chose per-service `Reconfigure` method over a global listener pool, keeping service
  boundaries clean. Each service owns its listeners; the hub coordinator orchestrates order.
- Chose bind-before-close ordering: new listeners are bound and serving before old ones
  are closed, guaranteeing zero downtime for non-conflicting address changes.
- Exported `ListenerDiff` (not internal) so the hub coordinator can compute diffs externally.
- Introduced `loadBoth` (returns both `map[string]any` and `*Tree` from a single parse) instead
  of two separate load functions, avoiding double-parsing on every SIGHUP reload. Both
  `loadConfigFromDisk` (for the plugin server's `ConfigLoader`) and `doReload` share the
  same `readAndParse` closure.

## Consequences

- All HTTP-based servers (web, LG, MCP, REST, gRPC) and SSH support listener migration
  on SIGHUP or config commit without restart.
- Cross-service conflict detection prevents bind failures when services swap addresses.
- AC-10/AC-11 (enable/disable service on reload) deferred: requires full startWebServer
  pipeline (TLS, routes, SSE broker), significantly more complex than listener migration.
- SSH Reconfigure is wired on the struct but not yet called from ReloadListeners because
  SSH config extraction lives in bgpconfig (different package); needs subsystem Reload path.
- The doReload function signature grew by two parameters (ListenerMigrator, loadBoth).

## Gotchas

- `http.Server.Serve(ln)` tracks listeners internally via `trackListener`; closing a listener
  causes Accept to return an error string "use of closed network connection" which is not
  a typed error. Must use string matching (`isClosedConnError`).
- Port 0 addresses resolve to real ports. The Reconfigure method must track the mapping
  from configured address to resolved address, or the bound list loses newly-added listeners.
- Reconfigure after Shutdown must be guarded: `http.Server.Serve` silently returns
  `ErrServerClosed` after shutdown, so new listeners would bind but never serve. Added a
  `stopped` flag checked at the top of Reconfigure.
- The `listeners` map must be initialized in `NewWebServer`, not lazily by `ListenAndServe`,
  because `Reconfigure` writes to it and could be called first on an exported type.
- Avoid double-parsing the config on reload: the original design had separate `loadConfigFromDisk`
  and `loadTreeFromDisk` functions, each calling `readAndParse` independently. Merged into
  a single `loadBoth` that returns both forms from one parse.

## Files

- `internal/component/web/server.go` -- WebServer.Reconfigure, ListenerDiff, listener tracking
- `internal/component/web/server_test.go` -- 7 new tests for reconfigure behavior
- `internal/component/lg/server.go` -- LGServer.Reconfigure
- `internal/component/api/rest/server.go` -- RESTServer.Reconfigure (loopback enforced)
- `internal/component/api/grpc/server.go` -- GRPCServer.Reconfigure (grpc.ErrServerStopped filter)
- `internal/component/ssh/ssh.go` -- SSH Server.Reconfigure (wish library Serve pattern)
- `cmd/ze/hub/service_mcp.go` -- MCPServerHandle.Reconfigure, Addresses
- `cmd/ze/hub/listener_migrate.go` -- ListenerMigrator (all services), detectConflicts
- `cmd/ze/hub/listener_migrate_test.go` -- 4 conflict detection tests
- `cmd/ze/hub/main_reload.go` -- doReload calls ReloadListeners
- `cmd/ze/hub/main.go` -- ListenerMigrator creation, loadBoth, SetLG/SetMCP/SetREST/SetGRPC wiring

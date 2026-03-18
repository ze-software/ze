# 393 - Plugin TLS Transport

## Objective

Replace Unix socketpairs with TLS-over-TCP for external plugin communication. Internal plugins (goroutine-based) keep `net.Pipe()` + DirectBridge unchanged. External plugins connect back to the engine over a single TLS connection, authenticate with a shared token, then proceed with the standard 5-stage handshake.

## Decisions

- **Single connection per plugin** (not dual socketpair). MuxConn handles bidirectional RPC on one connection. Responses (verb = ok/error) routed to pending callers, requests (verb = method name) pushed to Requests() channel.
- **Auth as stage 0**: `#0 auth {"token":"...","name":"..."}` using the same `#id verb json\n` framing. No new wire formats.
- **Config under `plugin { hub { listen ...; secret ...; } }`**. Hub config is extracted alongside plugin config during tree parsing.
- **PluginConn method shadowing** makes single-conn transparent to server code. `ReadRequest`, `CallRPC`, `SendResult`, etc. delegate to MuxConn when set. Server dispatch (startup.go, subsystem.go, dispatch.go) unchanged.
- **PluginAcceptor** manages TLS accept loop with per-name routing via `sync.Map`. `WaitForPlugin(ctx, name)` blocks until the named plugin connects and authenticates.
- **Default ports**: Hub TLS = 12700, SSH = 22000. Env vars: `ZE_PLUGIN_HUB_HOST` (default 127.0.0.1), `ZE_PLUGIN_HUB_PORT` (default 12700), `ZE_PLUGIN_TOKEN` (required).
- **fdpass.go deleted**: SCM_RIGHTS FD passing only works over Unix domain sockets. With TLS transport, the connection handler feature needs a different mechanism if needed in the future.

## Patterns

- MuxConn bidirectional routing: readLoop parses verb from `#id verb ...` line. `ok`/`error` -> response (route to pending caller by id). Anything else -> request (push to `requestCh`).
- `writeLineWithContext` race fix: when write deadline comes from context, a write timeout IS context.DeadlineExceeded even if `ctx.Err()` hasn't updated yet.
- Process single-conn mode: `SetSingleConn(conn)` + `InitConns()` creates one MuxConn, wraps it in two `NewMuxPluginConn(mux)` for ConnA/ConnB. Server code sees PluginConns as before.

## Gotchas

- MuxConn was half-duplex (outbound requests + inbound responses only). Extending it to handle inbound requests was a prerequisite the spec didn't anticipate. The fix was small (one verb check in readLoop) but architecturally necessary.
- `writeLineWithContext` had a race between write deadline and `ctx.Err()`. The kernel fires the write timeout before Go's context timer goroutine updates `ctx.Err()`, causing `TestSlowPluginFatal` to fail intermittently.
- net.Pipe() is synchronous: writes block until the reader is ready. Tests must start readers before writes to avoid deadlocks.
- Integration tests have transient port contention failures (not related to this work).

## Files

- Created: `ipc/tls.go` (TLS listener, auth, cert gen, PluginAcceptor), `ipc/tls_test.go`, `process/process_tls_test.go`
- Modified: `rpc/mux.go` (bidirectional), `rpc/conn.go` (deadline fix), `ipc/rpc.go` (MuxPluginConn), `process/process.go` (single-conn, TLS startup), `process/manager.go` (acceptor), `sdk/sdk.go` (TLS dial), `sdk/sdk_dispatch.go` (callback helpers), `server/startup.go` (acceptor wiring), `server/config.go` (Hub field), `reactor/reactor.go` (Hub config), `bgpconfig/loader.go` (ExtractHubConfig), `bgpconfig/plugins.go` (ExtractHubConfig), `plugin/types.go` (HubConfig), `ze-plugin-conf.yang` (hub container)
- Deleted: `ipc/fdpass.go`, `ipc/fdpass_test.go`

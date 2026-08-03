# 1071 -- Managed Hub Server (wire the dead server)

## Context

The managed-config feature let a fleet device fetch its config from a central hub. The client half
was wired and running (`cmd/ze/hub/main.go` starts `RunManagedClient`), but the server half --
`ManagedConfigService` in `internal/component/plugin/server/managed.go` -- had **zero production
callers** and no RPC dispatch. `docs/architecture/fleet-config.md` nonetheless marked it
"Implemented (all 17 ACs)", and the whole `spec-fleet-*` set (8 specs) was designed on top of
`HandleConfigFetch` as a live entry point. A real managed client that connected would authenticate,
send `config-fetch`, and get its connection dropped: the feature was broken end-to-end. The goal was
to actually serve managed clients.

## Decisions

- **Dedicated managed TLS listener over extending the plugin `PluginAcceptor`** (user decision). The
  acceptor routes an authenticated connection to a `WaitForPlugin(name)` waiter and closes any name
  with no waiter (`tls.go`); managed clients connect inbound at any time with no waiter, so
  reusing it needed a multi-listener refactor of a security-sensitive shared component. A dedicated
  listener that reuses `AuthenticateWithLookup` + `rpc.MuxConn` (no new auth/wire protocol) is lower
  blast radius and keeps the plugin acceptor untouched (so "plugins unaffected" holds by construction).
- **Per-client secret only, no shared-secret fallback** (`AuthenticateWithLookup(ctx, conn, "", lookup)`):
  a shared fallback would let anyone with the plugin secret connect as any client name.
- **Per-client config key `file/active/client-<name>.conf`** (`ClientConfigKey`), reusing the existing
  `file/active/` namespace + tooling; the `client-` prefix avoids collision with the hub's own config.
  The name is regex-validated at auth (`^[a-zA-Z0-9][a-zA-Z0-9-]{0,63}$`), so the key is path-traversal safe.
- **config-changed via a storage write-observer** over hooking `reloadAfterCommit` (which reloads the
  hub's OWN config, not a per-client write). `blobStorage.WriteFile` fires `SetWriteObserver(key)`; the
  hub maps the key back to a client name and pushes. Delivered on a long-lived `notifyWorker` fed by a
  buffered channel, so the write path never blocks on the network (goroutine-lifecycle rule).

## Consequences

- This spec is the missing foundation the `spec-fleet-*` set assumed existed; those specs should add a
  `Depends` on it. `fleet-2` (templates) and `fleet-7` (divergence) build on the serving loop.
- The managed listener must bind a `server` block distinct from the plugin acceptor's (documented
  `local` + `central` topology) to avoid a port clash; a same-block config surfaces as a listener error.
- Metrics: `ze_managed_clients_connected`, `ze_managed_config_fetch_total{result}`,
  `ze_managed_config_changed_pushed_total` (listed in `docs/plugin-development/metrics.md`).
- **Not secure-by-default yet.** The managed listener uses a self-signed cert; a remote client
  cannot verify it against a CA, so today it must connect with `tls-insecure`. Verifiable cert
  distribution (CA cert or pinned fingerprint in the client config), a port-collision doctor check,
  and a two-instance daemon `.ci` are tracked in `plan/spec-managed-server-hardening.md` (from the
  `/ze-review` passes). Two `/ze-review` passes also caught: a per-write push goroutine (fixed with a
  long-lived worker), a wholesale-failure on a colliding listener (fixed with per-address binding),
  dead `CertFP` + an unwired `ConnectedClients` export (removed/unexported), and config-changed
  head-of-line blocking -- one stalled client blocked pushes to all others (fixed with a per-push
  timeout + a small notify-worker pool).

## Gotchas

- The original defect is the project's #1 class: **implemented but not wired**. The 12 `test/managed/*.ci`
  were `ze config validate` parse checks only, giving false "Implemented" confidence; nothing exercised
  the runtime path. A wiring test (`TestStartManagedServerServesBlobConfig`, real blob + real TLS) is
  what catches this class.
- The audit broke three spec assumptions mid-implementation (no listener for managed-only hubs, no
  per-client provisioning, no config-changed trigger) -- what looked like "call a dead function" was the
  whole unfinished fleet-config server side.
- A benign `WARN mux conn: orphaned response id=1` appears in the config-changed tests (teardown race).

## Files

- `internal/component/plugin/server/managed_serve.go` (+ `_test.go`, 9 tests) -- `ManagedServer`
- `internal/component/config/storage/blob.go` -- `SetWriteObserver` + `WriteFile` fire
- `cmd/ze/hub/managed_server.go` (+ `_test.go`, 2 wiring tests), `cmd/ze/hub/main.go` -- startup wiring
- `docs/architecture/fleet-config.md` -- corrected status + dedicated-listener section + key + metrics

# Ze Environment Variables

**Source:** `internal/component/config/environment.go`
<!-- source: internal/component/config/environment.go -- env var registrations -->
**Purpose:** Reference of ze environment variables.

---

## Overview

Ze environment variables are registered centrally in
`internal/component/config/environment.go` and in each owning package
(reactor, L2TP, privilege drop, SSH). Every runtime lookup via
`internal/core/env.Get*` MUST hit a registered key; unregistered keys
abort the process.

Each YANG `environment/<section>/<option>` leaf also has a matching env
var so the operator can override it at runtime. The config file path
sets the env var at startup via `slogutil.ApplyLogConfig` (log keys) or
`config.ApplyEnvConfig` (everything else).
<!-- source: internal/component/config/apply_env.go -- ApplyEnvConfig -->

**Priority:** OS env var > config file `environment { }` block > default.

An existing OS env var is NEVER overwritten by the config file value.
<!-- source: internal/component/config/apply_env.go -- lookupPlumbValue -->

---

## Top-Level Variables

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `ze.user` | string | (unset) | User to drop to after port binding |
| `ze.group` | string | (user's primary group) | Group to drop to after port binding |
| `ze.pid.file` | string | (unset) | PID file path written at hub startup, removed at clean shutdown |
| `ze.pprof` | string | (unset) | pprof HTTP server address (e.g. `:6060`); empty disables |
| `ze.ready.file` | string | (unset) | Test infrastructure: signal file written when hub is ready |
| `ze.config.dir` | string | (unset) | Override default config directory |
<!-- source: internal/component/config/environment.go -- env var registrations -->
<!-- source: cmd/ze/hub/pidfile.go -- writePIDFile, removePIDFile -->

When `ze.user` is not set, no privilege dropping occurs.
<!-- source: internal/core/privilege/drop.go -- DropConfigFromEnv -->

---

## BGP Protocol Variables

| Variable | YANG Path | Default | Description |
|----------|-----------|---------|-------------|
| `ze.bgp.openwait` | `environment/bgp/openwait` | 120 (seconds) | Seconds to wait for peer OPEN after TCP connect (1-3600) |
| `ze.bgp.announce.delay` | `environment/bgp/announce-delay` | 0s (duration) | Delay between reactor Ready and first UPDATE (staged announcement gate) |
<!-- source: internal/component/bgp/reactor/session_connection.go -- openwait consumer -->
<!-- source: internal/component/bgp/reactor/reactor.go -- announce-delay consumer -->

## BGP Reactor Tuning

| Variable | Default | Description |
|----------|---------|-------------|
| `ze.bgp.reactor.speed` | "1.0" | Reactor loop time multiplier (0.1-10.0) |
| `ze.bgp.reactor.cache-ttl` | 60 | UPDATE cache TTL in seconds (0=immediate) |
| `ze.bgp.reactor.cache-max` | 1000000 | UPDATE cache max entries (0=unlimited) |
| `ze.bgp.reactor.update-groups` | true | Cross-peer UPDATE grouping |

## Chaos Fault Injection

| Variable | Default | Description |
|----------|---------|-------------|
| `ze.bgp.chaos.seed` | 0 | PRNG seed (0 = disabled, -1 = time-based) |
| `ze.bgp.chaos.rate` | "0.1" | Fault probability per operation (0.0-1.0) |

## Forward Pool / Buffers

| Variable | Default | Description |
|----------|---------|-------------|
| `ze.fwd.chan.size` | 256 | Per-destination forward worker channel capacity |
| `ze.fwd.write.deadline` | 30s | TCP write deadline for forward pool batch writes |
| `ze.fwd.pool.size` | 0 | Overflow MixedBufMux byte budget (0 = auto) |
| `ze.fwd.pool.maxbytes` | 0 | Combined byte budget for 4K+64K buffer pools (0 = unlimited) |
| `ze.fwd.batch.limit` | 1024 | Max items per forward batch |
| `ze.fwd.teardown.grace` | 5s | Grace period before forced teardown |
| `ze.fwd.pool.headroom` | 0 | Extra bytes beyond auto-sized pool baseline |
| `ze.buf.read.size` | 65536 | Per-session TCP read buffer size |
| `ze.buf.write.size` | 16384 | Per-session TCP write buffer size |
| `ze.cache.safety.valve` | 5m | UPDATE cache gap-based eviction duration |
| `ze.metrics.interval` | 10s | Periodic metrics refresh interval |

## Route Server

| Variable | Default | Description |
|----------|---------|-------------|
| `ze.rs.chan.size` | 4096 | Per-source-peer worker channel capacity |

---

## Log Variables

See [logging.md](../logging.md) for the full list. Config-block
`environment { log { level X; <subsystem> Y; } }` is plumbed to
`ze.log.*` env vars by `slogutil.ApplyLogConfig`.
<!-- source: internal/core/slogutil/slogutil.go -- ApplyLogConfig -->

---

## Listener Service Variables

Listener services (web, MCP, looking glass, API) use compound
`ip:port` format (multiple endpoints comma-separated, IPv6 bracket
notation supported). See [configuration.md](../../guide/configuration.md).

| Family | Listen | Enabled | Secret |
|--------|--------|---------|--------|
| Web | `ze.web.listen` | `ze.web.enabled`, `ze.web.insecure` | - |
| MCP | `ze.mcp.listen` | `ze.mcp.enabled` | `ze.mcp.token` |
| Looking glass | `ze.looking-glass.listen` | `ze.looking-glass.enabled`, `ze.looking-glass.tls` | - |
| API REST | `ze.api-server.rest.listen` | `ze.api-server.rest.enabled` | `ze.api-server.token` |
| API gRPC | `ze.api-server.grpc.listen` | `ze.api-server.grpc.enabled` | `ze.api-server.token` |

---

## L2TP

| Variable | Default | Description |
|----------|---------|-------------|
| `ze.l2tp.auth.timeout` | 30s | PPP auth-phase timeout |
| `ze.l2tp.auth.reauth-interval` | 0s | PPP periodic re-auth interval (0 disables) |
| `ze.l2tp.ncp.enable-ipcp` | true | Enable IPCP NCP |
| `ze.l2tp.ncp.enable-ipv6cp` | true | Enable IPv6CP NCP |
| `ze.l2tp.ncp.ip-timeout` | 30s | NCP phase wait for IP handler response |
| `ze.log.l2tp` | warn | L2TP subsystem log level (private) |
| `ze.l2tp.skip-kernel-probe` | false | Test-only: skip kernel module probe (private) |

---

## ExaBGP Bridge

| Variable | Default | Description |
|----------|---------|-------------|
| `exabgp.api.ack` | true | Bridge emits `done`/`error` lines on plugin stdin after each dispatched command |

The bridge subprocess reads `exabgp.api.ack` via `os.Getenv` because it
runs before Ze's env registry is initialized. The parent Ze process
writes it via `config.ApplyEnvConfig` when the operator sets
`environment { exabgp { api { ack <bool>; } } }`.
<!-- source: internal/exabgp/bridge/bridge_ack.go -- ackMode -->

---

## Test Infrastructure

| Variable | Default | Description |
|----------|---------|-------------|
| `ze.test.bgp.port` | 179 | BGP TCP port (ze-test peer + ze-test harness; private) |
| `ze.bfd.test-parallel` | false | BFD parallel test mode (private) |

---

## Boolean Values

Accepted: `true`/`false`, `yes`/`no`, `on`/`off`, `enable`/`disable`, `1`/`0`.

---

## Env Var Registry

All Ze env vars are registered via `env.MustRegister()` at package init
time. Calling `env.Get()` with an unregistered key aborts the process.
<!-- source: internal/core/env/registry.go -- MustRegister, EnvEntry -->

**Registration flags:**

| Flag | Meaning |
|------|---------|
| `Private` | Hidden from `ze env list` (tokens, test-only keys) |
| `Secret` | Cleared from OS environment after first `env.Get()`; value remains in the in-process cache |

---

**Last Updated:** 2026-04-18

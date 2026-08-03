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

**Classification** (Class column in tables below):

| Class | Meaning |
|-------|---------|
| YANG | Has YANG backing, correct placement |
| ENV | Correctly env-only (debug, bootstrap, safety, test) |
| PROMOTE | Should be YANG config but currently env-only |
| DEPR | Deprecated, superseded by another var |

See `ai/rules/config.md` for the YANG vs env-only decision framework.

---

## Top-Level Variables

| Variable | Type | Default | Class | Description |
|----------|------|---------|-------|-------------|
| `ze.user` | string | (unset) | YANG | User to drop to after port binding |
| `ze.group` | string | (user's primary group) | YANG | Group to drop to after port binding |
| `ze.pid.file` | string | (unset) | YANG | PID file path written at hub startup, removed at clean shutdown |
| `ze.pprof` | string | (unset) | YANG | pprof HTTP server address (e.g. `:6060`); empty disables |
| `ze.ready.file` | string | (unset) | ENV | Test infrastructure: signal file written when hub is ready |
| `ze.config.dir` | string | (unset) | ENV | Override default config directory; when unset the directory is derived from the binary location |
<!-- source: internal/component/config/environment.go -- env var registrations -->
<!-- source: internal/core/paths/paths.go -- ze.config.dir registration; DefaultConfigDir override-then-binary resolution -->
<!-- source: cmd/ze/hub/pidfile.go -- writePIDFile, removePIDFile -->

When `ze.user` is not set, no privilege dropping occurs.
<!-- source: internal/core/privilege/drop.go -- DropConfigFromEnv -->

---

## BGP Protocol Variables

| Variable | YANG Path | Default | Class | Description |
|----------|-----------|---------|-------|-------------|
| `ze.bgp.openwait` | `environment/bgp/openwait` | 120 (seconds) | YANG | Seconds to wait for peer OPEN after TCP connect (1-3600) |
| `ze.bgp.announce.delay` | `environment/bgp/announce-delay` | 0s (duration) | YANG | Delay between reactor Ready and first UPDATE (staged announcement gate) |
<!-- source: internal/component/bgp/reactor/session_connection.go -- openwait consumer -->
<!-- source: internal/component/bgp/reactor/reactor.go -- announce-delay consumer -->

## BGP Reactor Tuning

| Variable | Default | Class | Description |
|----------|---------|-------|-------------|
| `ze.bgp.reactor.speed` | "1.0" | YANG | Reactor loop time multiplier (0.1-10.0) |
| `ze.bgp.reactor.cache-ttl` | 60 | YANG | UPDATE cache TTL in seconds (0=immediate) |
| `ze.bgp.reactor.cache-max` | 1000000 | YANG | UPDATE cache max entries (0=unlimited) |
| `ze.bgp.reactor.update-groups` | true | YANG | Cross-peer UPDATE grouping |

## Chaos Fault Injection

| Variable | Default | Class | Description |
|----------|---------|-------|-------------|
| `ze.bgp.chaos.seed` | 0 | YANG | PRNG seed (0 = disabled, -1 = time-based) |
| `ze.bgp.chaos.rate` | "0.1" | YANG | Fault probability per operation (0.0-1.0) |

## Forward Pool / Buffers

Promoted vars are **deprecated**. Use YANG `environment { reactor { } }` instead.

<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- reactor forward/buffer leaves -->
<!-- source: internal/component/config/apply_env.go -- envPlumbingTable reactor entries -->

| Variable | YANG Leaf | Default | Description |
|----------|-----------|---------|-------------|
| `ze.fwd.chan.size` | `forward-queue-size` | 256 | Per-destination forward worker channel capacity |
| `ze.fwd.write.deadline` | (none) | 30s | TCP write deadline for forward pool batch writes |
| `ze.fwd.pool.size` | (none) | 0 | **Deprecated:** use `forward-pool-max-bytes` YANG leaf |
| `ze.fwd.pool.maxbytes` | `forward-pool-max-bytes` | 0 | Combined byte budget for 4K+64K buffer pools (0 = unlimited) |
| `ze.fwd.batch.limit` | `forward-batch-limit` | 1024 | Max items per forward batch |
| `ze.fwd.teardown.grace` | `forward-teardown-grace` | 5s | Grace period before forced teardown |
| `ze.fwd.pool.headroom` | `forward-pool-headroom` | 0 | Extra bytes beyond auto-sized pool baseline |
| `ze.buf.read.size` | `read-buffer-size` | 65536 | Per-session TCP read buffer size |
| `ze.buf.write.size` | `write-buffer-size` | 16384 | Per-session TCP write buffer size |
| `ze.cache.safety.valve` | (none) | 5m | UPDATE cache gap-based eviction duration |
| `ze.metrics.interval` | (none) | 10s | Periodic metrics refresh interval |

## Route Server

| Variable | Default | Class | Description |
|----------|---------|-------|-------------|
| `ze.bgp.route-server.worker-queue-size` | 4096 | YANG | Per-source-peer worker channel capacity (overrides YANG `route-server/worker-queue-size`) |

---

## Log Variables

See [logging.md](../../guide/logging.md) for the full list. Config-block
`environment { log { level X; <subsystem> Y; } }` is plumbed to
`ze.log.*` env vars by `slogutil.ApplyLogConfig`.
<!-- source: internal/core/slogutil/slogutil.go -- ApplyLogConfig -->

---

## Listener Service Variables

Listener services (web, MCP, looking glass, API) use compound
`ip:port` format (multiple endpoints comma-separated, IPv6 bracket
notation supported). See [configuration.md](../../guide/configuration.md).

| Family | Listen | Enabled | Secret | Class |
|--------|--------|---------|--------|-------|
| Web | `ze.web.listen` | `ze.web.enabled`, `ze.web.insecure` | - | YANG |
| MCP | `ze.mcp.listen` | `ze.mcp.enabled` | `ze.mcp.token` | YANG |
| Looking glass | `ze.looking-glass.listen` | `ze.looking-glass.enabled`, `ze.looking-glass.tls` | - | YANG |
| API REST | `ze.api-server.rest.listen` | `ze.api-server.rest.enabled` | `ze.api-server.token` | YANG |
| API gRPC | `ze.api-server.grpc.listen` | `ze.api-server.grpc.enabled` | `ze.api-server.token` | YANG |

---

## L2TP

| Variable | Default | Class | Description |
|----------|---------|-------|-------------|
| `ze.l2tp.auth.timeout` | 30s | PROMOTE | PPP auth-phase timeout |
| `ze.l2tp.auth.reauth-interval` | 0s | PROMOTE | PPP periodic re-auth interval (0 disables) |
| `ze.l2tp.ncp.enable-ipcp` | true | PROMOTE | Enable IPCP NCP |
| `ze.l2tp.ncp.enable-ipv6cp` | true | PROMOTE | Enable IPv6CP NCP |
| `ze.l2tp.ncp.ip-timeout` | 30s | PROMOTE | NCP phase wait for IP handler response |
| `ze.log.l2tp` | warn | ENV | L2TP subsystem log level (private) |
| `ze.l2tp.skip-kernel-probe` | false | ENV | Test-only: skip kernel module probe (private) |

---

## ExaBGP Bridge

| Variable | Default | Class | Description |
|----------|---------|-------|-------------|
| `exabgp.api.ack` | true | YANG | Bridge emits `done`/`error` lines on plugin stdin after each dispatched command |

The bridge subprocess reads `exabgp.api.ack` via `os.Getenv` because it
runs before Ze's env registry is initialized. The parent Ze process
writes it via `config.ApplyEnvConfig` when the operator sets
`environment { exabgp { api { ack <bool>; } } }`.
<!-- source: internal/exabgp/bridge/bridge_ack.go -- ackMode -->

---

## Test Infrastructure

| Variable | Default | Class | Description |
|----------|---------|-------|-------------|
| `ze.test.bgp.port` | 179 | ENV | BGP TCP port (ze-test peer + ze-test harness; private) |
| `ze.bfd.test-parallel` | false | ENV | BFD parallel test mode (private) |

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
| `Aliases` | Alternative keys that resolve to the same entry (for YANG-aligned names during promotion) |
| `Deprecated` | Prints a one-time warning to stderr on first use when the var is set, suggesting the replacement key |

**Aliases:** When a setting is promoted from env-only to YANG config, the old
abbreviated env var key stays for backward compatibility and the new
YANG-aligned key is registered as an alias (or vice versa). `Get()` and
`Set()` resolve aliases to the canonical key transparently. Precedence:
canonical key value > alias key value. See `ai/rules/config.md`.

**Deprecation:** When an env var is superseded (e.g., `ze.fwd.pool.size` by
`ze.fwd.pool.maxbytes`), mark it with `Deprecated: "replacement.key"`.
The warning only fires when the deprecated var is actually set (non-empty),
avoiding noise on every startup.

## Env Completion Catalog

The `internal/core/envcatalog` package provides a shared public env-key
catalog for shell and operational CLI completion. It merges `env.Entries()`
(excludes Private, skips angle-bracket template keys) with concrete
`ze.log.<subsystem>` rows expanded from `slogutil.Subsystems()`.
<!-- source: internal/core/envcatalog/catalog.go -- VisibleEntries -->

Completion never calls `env.Get()` or any typed getter, so it cannot
mutate `Secret` entries or leak current values. The catalog is metadata-only.

`LookupLogSubsystem()` resolves a concrete `ze.log.<name>` key to its
`slogutil.SubsystemInfo`, enabling `ze env get ze.log.bgp.reactor` to
return metadata for log keys that are not in the static env registry.
<!-- source: internal/core/envcatalog/catalog.go -- LookupLogSubsystem -->

---

## Env Var Debt

### PROMOTE Backlog

Env-only vars that operators tune in change tickets; should have YANG backing.
Tracked spec: `spec-l2tp-env-promote` (L2TP).

Reactor forward/buffer vars and RS worker-queue-size have been promoted
(see Forward Pool and Route Server sections above). Web `ui-mode` promoted
in `9a951f717`.

| Env var | Target YANG leaf | Priority | Spec |
|---------|-----------------|----------|------|
| `ze.l2tp.auth.timeout` | `l2tp/auth-timeout` | Medium | spec-l2tp-env-promote |
| `ze.l2tp.auth.reauth-interval` | `l2tp/reauth-interval` | Medium | spec-l2tp-env-promote |
| `ze.l2tp.ncp.enable-ipcp` | `l2tp/ncp-enable-ipcp` | Medium | spec-l2tp-env-promote |
| `ze.l2tp.ncp.enable-ipv6cp` | `l2tp/ncp-enable-ipv6cp` | Medium | spec-l2tp-env-promote |
| `ze.l2tp.ncp.ip-timeout` | `l2tp/ncp-ip-timeout` | Low | spec-l2tp-env-promote |

### Naming Violations

| Category | Count | Examples |
|----------|-------|---------|
| Abbreviations in key | 11 | `fwd`, `chan`, `buf`, `rs`, `dest`, `cap` |
| Path doesn't mirror YANG | 4 | `ze.pid.file` vs `daemon/pid`, `ze.bgp.chaos.*` vs `environment/chaos/*` |
| Missing unit suffix | 3 | `teardown.grace`, `auth.timeout`, `ncp.ip-timeout` |
| Leaf segment mismatch | 1 | `ze.bgp.announce.delay` (dots) vs `announce-delay` (kebab) |

Naming fixes are paired with PROMOTE work: when a var gets a YANG leaf, register
the YANG-aligned name as an alias and mark the old abbreviated name deprecated.
See `ai/rules/config.md`.

### Deprecation

| Env var | Replacement | Reason |
|---------|-------------|--------|
| `ze.fwd.pool.size` | `ze.fwd.pool.maxbytes` | Legacy overlap, confusing semantics |

---

**Last Updated:** 2026-06-12

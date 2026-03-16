# Ze Environment Variables

**Source:** `internal/component/config/environment.go`
**Purpose:** Complete reference of all ze environment variables

---

## Overview

Ze uses environment variables for daemon and BGP subsystem configuration.

**Two variable families:**

| Family | Format | Purpose |
|--------|--------|---------|
| Top-level | `ze.<option>` / `ze_<option>` | Daemon-wide settings (privilege drop) |
| BGP subsystem | `ze.bgp.<section>.<option>` / `ze_bgp_<section>_<option>` | BGP and process settings |

**Priority:** env var (dot) > env var (underscore) > config file `environment { }` block > default.

**Strict validation:** Invalid values cause startup failure (not silent defaults).

---

## Top-Level Variables

| Variable | Underscore | Type | Default | Description |
|----------|------------|------|---------|-------------|
| `ze.user` | `ze_user` | string | (none) | User to drop to after port binding |
| `ze.group` | `ze_group` | string | (primary group of user) | Group to drop to after port binding |

When `ze.user` is not set, no privilege dropping occurs.
See `internal/core/privilege/` for implementation.

---

## BGP Subsystem Variables

All BGP variables use the `ze.bgp.<section>.<option>` format.
They can also be set via the config file `environment { <section> { <option> <value> } }` block.

### daemon

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `ze.bgp.daemon.pid` | string | "" | PID file location |
| `ze.bgp.daemon.user` | string | "zeuser" | Legacy user field (prefer `ze.user`) |
| `ze.bgp.daemon.daemonize` | bool | false | Run in background |
| `ze.bgp.daemon.drop` | bool | true | Legacy drop field (prefer `ze.user`) |
| `ze.bgp.daemon.umask` | octal | 0137 | Umask for created files |

### log

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `ze.bgp.log.enable` | bool | true | Enable logging |
| `ze.bgp.log.level` | string | "INFO" | Syslog level: DEBUG, INFO, NOTICE, WARNING, ERR, CRITICAL |
| `ze.bgp.log.destination` | string | "stdout" | stdout, stderr, syslog, or filename |
| `ze.bgp.log.all` | bool | false | Debug all subsystems |
| `ze.bgp.log.configuration` | bool | true | Log config parsing |
| `ze.bgp.log.reactor` | bool | true | Log signals, reloads |
| `ze.bgp.log.daemon` | bool | true | Log pid, forking |
| `ze.bgp.log.processes` | bool | true | Log process handling |
| `ze.bgp.log.network` | bool | true | Log TCP/IP, network state |
| `ze.bgp.log.statistics` | bool | true | Log packet statistics |
| `ze.bgp.log.packets` | bool | false | Log BGP packets |
| `ze.bgp.log.rib` | bool | false | Log local route changes |
| `ze.bgp.log.message` | bool | false | Log route announcements |
| `ze.bgp.log.timers` | bool | false | Log keepalive timers |
| `ze.bgp.log.routes` | bool | false | Log received routes |
| `ze.bgp.log.parser` | bool | false | Log message parsing |
| `ze.bgp.log.short` | bool | true | Short log format |

Per-subsystem log levels are also supported via `ze.log.<subsystem>=<level>` (handled by `slogutil.ApplyLogConfig()`).

### tcp

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `ze.bgp.tcp.port` | int | 179 | Port to bind (179 or 1025-65535) |
| `ze.bgp.tcp.attempts` | int | 0 | Exit after N sessions complete (0 = unlimited) |
| `ze.bgp.tcp.delay` | int | 0 | Delay announcements by N minutes |
| `ze.bgp.tcp.acl` | bool | false | Experimental ACL |
| `ze.bgp.tcp.once` | bool | false | Legacy alias: sets attempts=1 |
| `ze.bgp.tcp.connections` | int | - | Legacy alias for attempts |

### bgp

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `ze.bgp.bgp.connection` | string | "" | Connection mode: "both", "passive", "active" |
| `ze.bgp.bgp.openwait` | int | 60 | Seconds to wait for OPEN (1-3600) |

### cache

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `ze.bgp.cache.attributes` | bool | true | Cache attributes |

### api

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `ze.bgp.api.ack` | bool | true | Acknowledge API commands |
| `ze.bgp.api.chunk` | int | 1 | Max lines before yield |
| `ze.bgp.api.encoder` | string | "json" | Encoder: json or text |
| `ze.bgp.api.compact` | bool | false | Compact JSON for INET |
| `ze.bgp.api.respawn` | bool | true | Respawn dead processes |
| `ze.bgp.api.terminate` | bool | false | Terminate if process dies |
| `ze.bgp.api.cli` | bool | true | Create CLI named pipe |
| `ze.bgp.api.pipename` | string | "ze-bgp" | Name for CLI pipe |
| `ze.bgp.api.socketname` | string | "ze-bgp" | Name for Unix socket |

### reactor

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `ze.bgp.reactor.speed` | float | 1.0 | Reactor loop time multiplier (0.1-10.0) |
| `ze.bgp.reactor.cache-ttl` | int | 60 | UPDATE cache TTL in seconds (0-3600) |
| `ze.bgp.reactor.cache-max` | int | 1000000 | UPDATE cache max entries (0 = unlimited) |

### debug

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `ze.bgp.debug.pdb` | bool | false | Enable pdb on errors (N/A in Go) |
| `ze.bgp.debug.memory` | bool | false | Memory debug |
| `ze.bgp.debug.configuration` | bool | false | Raise on config errors |
| `ze.bgp.debug.selfcheck` | bool | false | Self-check config |
| `ze.bgp.debug.route` | string | "" | Decode route from config |
| `ze.bgp.debug.defensive` | bool | false | Generate random faults |
| `ze.bgp.debug.rotate` | bool | false | Rotate config on reload |
| `ze.bgp.debug.timing` | bool | false | Reactor timing analysis |
| `ze.bgp.debug.pprof` | string | "" | pprof HTTP server address (e.g. ":6060") |

### chaos

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `ze.bgp.chaos.seed` | int64 | 0 | PRNG seed (0 = disabled, -1 = time-based) |
| `ze.bgp.chaos.rate` | float | 0.1 | Fault probability per operation (0.0-1.0) |

---

## Config File Syntax

Environment variables can also be set in the config file:

```
environment {
    log {
        level DEBUG
    }
    tcp {
        port 1179
    }
    daemon {
        user zeuser
    }
}
```

See [environment-block.md](environment-block.md) for the full config block syntax.

---

## Boolean Values

Accepted: `true`, `false`, `yes`, `no`, `on`, `off`, `enable`, `disable`, `1`, `0`.
Any other value causes a startup error.

---

**Last Updated:** 2026-03-16

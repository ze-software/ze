# Environment Variables

Ze reads environment variables for daemon-wide knobs (pprof, PID file,
privilege drop) and BGP protocol tuning. Every variable has a matching
YANG path under the `environment { }` config block; the OS env var
wins when both are set.

See [environment.md](../architecture/config/environment.md) for the
authoritative list.

## Priority

1. OS environment variable (shell, systemd `EnvironmentFile`, container runtime)
2. Config file `environment { ... }` block
3. Built-in default

An OS env var is never overwritten by a config value.

## Common Variables

| Variable | Purpose |
|----------|---------|
| `ze.user` | User to drop to after binding privileged ports |
| `ze.pid.file` | Path to PID file written at startup, removed at clean shutdown |
| `ze.pprof` | Bind pprof HTTP server (e.g. `:6060`). Empty disables |
| `ze.bgp.openwait` | Seconds to wait for peer OPEN after TCP connect |
| `ze.bgp.announce.delay` | Duration to block between reactor Ready and first UPDATE |
| `exabgp.api.ack` | ExaBGP bridge: emit `done`/`error` ack lines on plugin stdin |

## Config Block Syntax

The same knobs can be expressed in the config file:

```
environment {
    daemon {
        pid "/run/ze.pid"
        user "zeuser"
    }
    bgp {
        openwait 60
        announce-delay 5s
    }
    pprof ":6060"
    exabgp {
        api {
            ack false
        }
    }
    chaos {
        seed 0
        rate "0.1"
    }
    reactor {
        cache-ttl 60
    }
    log {
        level INFO
        bgp.routes debug
    }
}
```

## Changes in 2026-04

The ExaBGP compatibility surface was trimmed. Every
`ze.bgp.daemon.*`, `ze.bgp.log.*`, `ze.bgp.tcp.*`, `ze.bgp.api.*`, and
`ze.bgp.debug.*` variable was either renamed, dropped, or plumbed from
a YANG leaf to a new env key:

| Old | New / Action |
|-----|--------------|
| `ze.bgp.daemon.pid` | Plumbed to `ze.pid.file` (YANG leaf kept) |
| `ze.bgp.daemon.user` | Plumbed to `ze.user` (YANG leaf kept) |
| `ze.bgp.daemon.daemonize`, `drop`, `umask` | Dropped (use systemd unit file) |
| `ze.bgp.log.level`, `destination`, `short` | Dropped (slogutil owns `ze.log.*`) |
| `ze.bgp.tcp.attempts` | Dropped (test harness sends SIGTERM) |
| `ze.bgp.tcp.delay` | Dropped; new `ze.bgp.announce.delay` duration |
| `ze.bgp.tcp.port` | Renamed `ze.test.bgp.port` (test infrastructure only) |
| `ze.bgp.tcp.acl` | Dropped (experimental, unimplemented) |
| `ze.bgp.bgp.openwait` | Renamed `ze.bgp.openwait` |
| `ze.bgp.cache.attributes` | Dropped (caching is architectural) |
| `ze.bgp.debug.pprof` | Renamed `ze.pprof` (process-wide) |
| `ze.bgp.debug.*` (pdb/memory/rotate/etc.) | Dropped (use `go tool pprof`, `ze.log`) |
| `ze.bgp.api.ack` | Dropped; `exabgp.api.ack` handles bridge ack |
| `ze.bgp.api.chunk`, `encoder`, `respawn`, ... | Dropped (Ze manages plugins) |

The `ze exabgp migrate --env` tool converts surviving keys to Ze YANG
blocks and emits `# <key> -- no longer supported` comments for dropped
ones so an operator can audit an ExaBGP env file before commit.

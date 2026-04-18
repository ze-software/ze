# Environment Configuration Block

## TL;DR

YANG container `environment { }` sets env vars from the config file:

```
environment {
    daemon {
        pid "/run/ze.pid";
        user "zeuser";
    }
    bgp {
        openwait 60;
        announce-delay 5s;
    }
    pprof ":6060";
    log {
        level INFO;
        bgp.routes debug;
    }
    exabgp {
        api {
            ack false;
        }
    }
    chaos {
        seed 0;
        rate "0.1";
    }
    reactor {
        cache-ttl 60;
    }
}
```

Priority: **OS env > config block > YANG default**.

The config loader maps `environment/log/*` to env vars via
`slogutil.ApplyLogConfig` and everything else via `config.ApplyEnvConfig`.
Each leaf writes the target env var at startup (never overwriting a
pre-existing OS env var).
<!-- source: internal/component/config/apply_env.go -- ApplyEnvConfig, envPlumbingTable -->
<!-- source: internal/core/slogutil/slogutil.go -- ApplyLogConfig -->

## Sections

| Section | Option | Env var | Default | Notes |
|---------|--------|---------|---------|-------|
| `daemon` | `pid` | `ze.pid.file` | "" | Hub writes PID file, removes at clean shutdown |
| `daemon` | `user` | `ze.user` | "zeuser" | User for privilege drop |
| `log` | `level`, `backend`, `destination`, `relay` | `ze.log.*` | per-leaf | Owned by `slogutil.ApplyLogConfig` |
| `log` | `<subsystem>` | `ze.log.<subsystem>` | per-subsystem | `log { bgp.routes debug; }` -> `ze.log.bgp.routes=debug` |
| `bgp` | `openwait` | `ze.bgp.openwait` | 120 | Seconds to wait for peer OPEN (1-3600) |
| `bgp` | `announce-delay` | `ze.bgp.announce.delay` | 0s | Duration to delay first UPDATE after reactor Ready |
| `pprof` | (top-level leaf) | `ze.pprof` | "" | pprof HTTP server address, e.g. `:6060` |
| `chaos` | `seed` | `ze.bgp.chaos.seed` | 0 | PRNG seed (0 = disabled) |
| `chaos` | `rate` | `ze.bgp.chaos.rate` | "0.1" | Fault probability (0.0-1.0) |
| `reactor` | `speed`, `cache-ttl`, `cache-max`, `update-groups` | `ze.bgp.reactor.*` | per-leaf | Reactor tuning |
| `exabgp/api` | `ack` | `exabgp.api.ack` | true | ExaBGP bridge ack emission |
<!-- source: internal/component/hub/schema/ze-hub-conf.yang -- environment container -->
<!-- source: internal/component/bgp/schema/ze-bgp-conf.yang -- environment augment -->

## Retired Keys

The ExaBGP-compat containers `environment/tcp`, `environment/cache`,
`environment/api`, `environment/debug` were removed in 2026-04.
Operators with legacy ExaBGP INI configs can convert via
`ze exabgp migrate --env <file>`: surviving keys become YANG blocks,
dropped keys become `# <key> -- no longer supported` comments.
<!-- source: internal/exabgp/migration/env.go -- mapEnvKnownKey -->

See [environment-variables.md](../../guide/environment-variables.md)
for the retiree list and rename table.

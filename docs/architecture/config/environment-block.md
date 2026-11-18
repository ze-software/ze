# Environment Configuration Block

## TL;DR

YANG container `environment { }` sets process options from the config file.
Ordinary settings populate env vars; `backup/directory` uses live commits:

```
environment {
    daemon {
        pid "/run/ze.pid";
        user "zeuser";
    }
    backup {
        directory "/srv/ze-backups";
    }
    bgp {
        openwait 60;
        announce-delay 5s;
    }
    pprof ":6060";
    hide-version true;
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

`backup/directory` selects the absolute parent of private backup uploads,
editor workspaces and download snapshots. The `ze.backup.directory` OS override
wins. The backup leaf uses the configuration commit lifecycle instead of OS
environment plumbing, so a successful commit selects a checked folder without
a restart. Preparation creates a missing parent and probes real write access
as the daemon account; rejection keeps the old effective selection. Existing
workspace and download files remain in their original roots until close or
expiry. Removing the leaf restores the existing platform data directory's
`backups` folder unless the OS override is set; unresolved development layouts
retain the OS temporary folder. The ordinary plumbed environment settings keep
their startup-only behavior.
<!-- source: internal/component/hub/yang/ze-hub-conf.yang -- backup, directory -->
<!-- source: internal/component/config/cli/register_artifact_directory.go -- prepareArtifactDirectory -->

The config loader maps `environment/log/*` to env vars via
`slogutil.ApplyLogConfig` and ordinary non-log settings via `config.ApplyEnvConfig`.
The live `backup/directory` leaf is excluded from both mappings. Each plumbed
leaf writes the target env var at startup (never overwriting a
pre-existing OS env var). Only `config.LoadConfig`, the load of the config the
process runs, writes them. `config.ReadConfig` parses and validates the same
way and writes nothing, so a command that only reads a config file (a show
command, `ze doctor`, a candidate's validation) leaves the running process's
settings as they were.
<!-- source: internal/component/config/apply_env.go -- ApplyEnvConfig, envPlumbingTable -->
<!-- source: internal/core/slogutil/slogutil.go -- ApplyLogConfig -->
<!-- source: internal/component/config/loader.go -- LoadConfig, ReadConfig, applyParsedEnvironment -->

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
| `hide-version` | (top-level leaf) | `ze.hide-version` | false | Keeps `X-Ze-Version` off the web and looking-glass responses |
| `chaos` | `seed` | `ze.bgp.chaos.seed` | 0 | PRNG seed (0 = disabled) |
| `chaos` | `rate` | `ze.bgp.chaos.rate` | "0.1" | Fault probability (0.0-1.0) |
| `reactor` | `speed`, `cache-ttl`, `cache-max`, `update-groups`, `forward-queue-size`, `forward-batch-limit`, `forward-pool-max-bytes`, `forward-pool-headroom`, `forward-teardown-grace`, `read-buffer-size`, `write-buffer-size` | `ze.bgp.reactor.*` / `ze.fwd.*` / `ze.buf.*` | per-leaf | Reactor tuning |
| `exabgp/api` | `ack` | `exabgp.api.ack` | true | ExaBGP bridge ack emission |
<!-- source: internal/component/hub/yang/ze-hub-conf.yang -- environment container -->
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- environment augment -->

## Retired Keys

The ExaBGP-compat containers `environment/tcp`, `environment/cache`,
`environment/api`, `environment/debug` were removed in 2026-04.
Operators with legacy ExaBGP INI configs can convert via
`ze exabgp migrate --env <file>`: surviving keys become YANG blocks,
dropped keys become `# <key> -- no longer supported` comments.
<!-- source: internal/exabgp/migration/env.go -- mapEnvKnownKey -->

See [environment-variables.md](../../guide/environment-variables.md)
for the retiree list and rename table.

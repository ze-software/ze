# Config Archive

Ze archives configuration files to local or remote destinations. Archives can be triggered manually from the CLI, automatically on every editor commit, or on a schedule.
<!-- source: internal/component/config/archive/archive.go -- archive core logic -->
<!-- source: internal/component/config/system/schema/ze-system-conf.yang -- system and archive YANG schema -->

## Configuration

Archive destinations are named blocks under `system { archive { } }`. Each block defines one destination with its own trigger, filename format, and timeout.

```
system {
    host router1;
    domain dc1.example.com;

    archive local-backup {
        location file:///var/backups/ze;
        trigger commit;
    }

    archive offsite {
        location https://archive.example.com/configs;
        trigger daily;
        on-change true;
        timeout 10s;
        filename "{host}-{date}-{time}";
    }
}
```
<!-- source: internal/component/config/system/schema/ze-system-conf.yang -- archive list definition -->

### Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `location` | string | (required) | Destination URL |
| `trigger` | enum | `manual` | When to archive: `commit`, `manual`, `daily`, `hourly` |
| `filename` | string | `{name}-{host}-{date}-{time}` | Filename format with token substitution |
| `timeout` | duration | `30s` | HTTP upload timeout |
| `on-change` | boolean | `false` | Time-based triggers only: skip if config is unchanged since last archive |

### Location Schemes

| Scheme | Behavior |
|--------|----------|
| `file:///path` | Writes to local filesystem. Creates parent directories if needed. Files written with `0600` permissions. |
| `http://host/path` | HTTP POST with `text/plain` content type. Filename sent in `X-Archive-Filename` header. |
| `https://host/path` | Same as HTTP with TLS. |

Use absolute paths for `file://` locations. `file://./relative` does not work correctly in Go's URL parser.
<!-- source: internal/component/config/archive/archive.go -- ToFile, ToHTTP, ValidateLocation -->

### Triggers

| Trigger | When it fires | Fires on boot | Respects `on-change` |
|---------|---------------|---------------|----------------------|
| `commit` | After every editor commit | No | No (always archives) |
| `manual` | `ze config archive <name>` CLI command | No | No |
| `daily` | Every 24 hours from daemon start | Yes | Yes |
| `hourly` | Every hour from daemon start | Yes | Yes |

Time-based triggers (`daily`, `hourly`) always fire once on daemon boot regardless of `on-change`, establishing a baseline. Subsequent ticks respect the `on-change` flag.
<!-- source: internal/component/config/archive/archive.go -- TriggerCommit, TriggerManual, TriggerDaily, TriggerHourly -->

### Filename Tokens

| Token | Value | Example |
|-------|-------|---------|
| `{name}` | Config file basename without extension | `router` |
| `{host}` | `system.host` value | `router1` |
| `{domain}` | `system.domain` value | `dc1.example.com` |
| `{date}` | Date as `YYYYMMDD` | `20260329` |
| `{time}` | Time as `HHMMSS` | `143045` |
| `{archive}` | Archive block name | `local-backup` |

The `.conf` extension is always appended. With the default format `{name}-{host}-{date}-{time}`, a file named `router.conf` on host `router1` produces `router-router1-20260329-143045.conf`.
<!-- source: internal/component/config/archive/archive.go -- FormatFilename, DefaultFilenameFormat -->

## System Identity

The `system` block provides hostname and domain values used in archive filenames and elsewhere.

```
system {
    host router1;
    domain dc1.example.com;
}
```

Both `host` and `domain` support `$ENV` variable expansion. If the value starts with `$`, the remainder is looked up as an OS environment variable:

```
system {
    host $HOSTNAME;
    domain $DOMAIN;
}
```

If the environment variable is empty or unset, the literal string is kept (e.g., `$HOSTNAME` stays as-is). When `host` is not configured, it defaults to `unknown`. No `os.Hostname()` fallback is used.
<!-- source: internal/component/config/system/system.go -- ExpandEnvValue, ExtractSystemConfig -->

## CLI Usage

Archive a configuration to a named destination:

```bash
ze config archive <name> <config-file>
```

The `<name>` must match a named block under `system { archive { } }` in the config file. The command parses the config, extracts the named block's settings, and uploads.

```bash
ze config archive local-backup router.conf
ze config archive offsite router.conf
```

Reading from stdin is supported with `-` as the config file:

```bash
cat router.conf | ze config archive local-backup -
```

If the named block does not exist, the command prints the available archive names and exits with code 1.
<!-- source: cmd/ze/config/cmd_archive.go -- cmdArchiveImpl -->

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Archive succeeded |
| 1 | Error (missing arguments, block not found, parse failure, upload failure) |
<!-- source: cmd/ze/config/cmd_archive.go -- exitOK, exitError -->

## Editor Integration

When the editor starts, it reads archive blocks from the config. Blocks with `trigger commit` are wired into the editor's commit path. After every successful `commit`, the editor archives the current config content to all `trigger commit` destinations.

Archive errors during commit are non-fatal. The commit succeeds and the editor reports the number of archive failures in the status line. Other trigger types (`manual`, `daily`, `hourly`) are not fired by the editor.
<!-- source: cmd/ze/config/cmd_edit.go -- Wire archive notifier -->
<!-- source: internal/component/cli/model_commands.go -- cmdCommit archive dispatch -->

Archive locations are read from the config at editor startup. Adding an `archive` block during an editing session requires restarting the editor for it to take effect.

## Fan-Out Behavior

All configured destinations are attempted regardless of individual failures. Errors are collected per destination. A failure uploading to one location does not prevent archiving to other locations.
<!-- source: internal/component/config/archive/archive.go -- NewNotifier -->

## Change Detection

For time-based triggers with `on-change true`, ze tracks config changes using SHA-256 hashes in memory. Each named archive block has its own hash. The first check after daemon start always reports "changed" (no baseline yet), so the boot archive fires unconditionally. Subsequent checks compare the current config hash against the last archived hash and skip the archive if unchanged.

The tracker resets on daemon restart since hashes are in-memory only.
<!-- source: internal/component/config/archive/archive.go -- ChangeTracker, HasChanged -->

## Examples

### Local backup on every commit

```
system {
    host $HOSTNAME;
    archive local {
        location file:///var/backups/ze;
        trigger commit;
    }
}
```

Every editor `commit` writes a timestamped copy to `/var/backups/ze/`.

### Daily offsite with change detection

```
system {
    host router1;
    domain dc1.example.com;
    archive offsite {
        location https://archive.example.com/upload;
        trigger daily;
        on-change true;
        timeout 10s;
        filename "{host}-{domain}-{date}";
    }
}
```

On daemon boot, archives immediately. Every 24 hours, archives again only if the config has changed. Filename: `router1-dc1.example.com-20260329.conf`.

### Multiple destinations

```
system {
    host edge-01;
    archive local {
        location file:///var/backups/ze;
        trigger commit;
    }
    archive central {
        location https://hub.example.com/configs;
        trigger hourly;
        on-change true;
    }
}
```

Local backup on every commit. Central server receives a copy hourly (skipped if unchanged). Manual archive to either destination via `ze config archive local edge-01.conf` or `ze config archive central edge-01.conf`.

### HTTP receiver

The HTTP endpoint receives a POST with:

| Header | Value |
|--------|-------|
| `Content-Type` | `text/plain` |
| `X-Archive-Filename` | Generated filename (e.g., `edge-01-router1-20260329-143045.conf`) |

The body is the raw config file content. Any HTTP 2xx response is treated as success.

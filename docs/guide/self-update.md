# Self-Update Guide

Ze can automatically download, verify, and install firmware updates on non-gokrazy
deployments. This guide covers server setup, client configuration, manual operations,
and fleet deployment.

## Overview

The self-update feature extends ze's periodic version check (which only reports
availability) to a full update cycle:

1. **Check:** fetch the version manifest from a configured URL
2. **Download:** fetch the binary, enforce a 500 MB size cap
3. **Verify:** compute SHA-256 and compare against the manifest
4. **Stage:** atomically replace the running binary via rename, keeping a `.prev` backup
5. **Restart:** optionally restart into the new version

Each step can fail independently. The binary path is never empty: atomic rename
guarantees either the old or new binary is always present.

## Server Setup

### ze update serve

Ze includes a standalone update server for build infrastructure:

```
ze update serve --listen :8080
```

Endpoints:

| Path | Purpose |
|------|---------|
| `GET /version.json` | Enhanced manifest (version, sha256, size, paused) |
| `GET /<goos>/<goarch>` | Binary download |
| `GET /<goos>/<goarch>/sha256` | Hex SHA-256 digest (text/plain) |

The server computes SHA-256 at startup. If the binary changes on disk (rebuild),
restart the server.

### Pause Mechanism

Two mechanisms, ORed together:

- **File:** create a file named `update-paused` in the binary's directory. Remove to unpause.
- **Signal:** send `SIGUSR1` to toggle the in-memory pause state. Logged to stderr.

When paused, the manifest returns `"paused": true`. Clients with `auto-apply` stop
downloading. Manual commands (`update system firmware apply`) bypass pause.

### Custom Server

Any HTTP server returning the manifest JSON works. Required fields:

```json
{
    "version": "26.05.20",
    "sha256": "a1b2c3d4...64hex",
    "size": 52428800,
    "paused": false
}
```

The `sha256` field is required for auto-apply. Without it, auto-apply refuses to
download. Manual commands warn but proceed.

Optional fields: `minimum-version` (sequential upgrade enforcement),
`download-url` (override download location, must be HTTPS).

## Client Configuration

### Check-only (default)

```
system {
    update-check {
        url https://update.example.com/version.json
    }
}
```

Daily check, report via `show system update` and `show warnings`. No download.

### Fleet (automated with scheduling)

```
system {
    update-check {
        url https://update.example.com/version.json
        interval 3600
        auto-apply true
        spread 1800
        maintenance-window {
            start 02:00
            end 06:00
        }
        restart {
            time 03:00
        }
    }
}
```

Check hourly. On new version: wait a random delay up to 30 minutes (spread),
download and verify at any time, replace binary only between 02:00 and 06:00,
restart at 03:00.

### Lab (immediate)

```
system {
    update-check {
        url https://update.example.com/version.json
        interval 300
        auto-apply true
        spread 0
        restart {
            immediate
        }
    }
}
```

Check every 5 minutes. Download immediately, stage, restart within 5 seconds.

## CLI Commands

### Status

```
show system update              # Current status
show system update history      # Last 20 update events
show warnings                   # Includes update warnings
```

### Manual Operations

```
update system firmware check       # Trigger immediate version check
update system firmware download    # Download now (bypass spread, window)
update system firmware apply       # Full cycle: download+verify+stage+restart
update system firmware restart     # Restart into staged version
update system firmware rollback    # Restore previous version
```

Manual commands bypass spread and maintenance window. `apply` and `download`
also bypass server-side pause.

## Rollback

After a successful update, the previous binary is preserved as `<binary>.prev`.
To roll back:

```
update system firmware rollback
```

This renames `.prev` to the target binary and restarts. After rollback, `.prev`
no longer exists. To re-apply the update, the device must re-download from the
server.

To prevent further devices from updating to a bad release, pause the server:

```
touch /path/to/update-paused    # on the update server
```

## Fleet Deployment

### Spread

Spread prevents all devices from downloading simultaneously. Each device computes
a deterministic delay from its identity (machine-id or hostname) and the target
version string. The delay is stable across restarts (as long as the identity source
is stable).

Default: 3600 seconds (1 hour). A fleet of 100 devices distributes downloads
over approximately 1 hour.

### Maintenance Windows

The maintenance window gates only binary replacement (the rename step). Download
and verification proceed at any time. This ensures the binary is ready the moment
the window opens.

Windows crossing midnight are valid: `start 22:00` / `end 06:00`.

### Sequential Upgrades

If the server sets `minimum-version` in the manifest, devices running a version
older than the minimum are blocked from updating. They report:

> upgrade to [version] requires intermediate version [minimum-version] first

The operator must update those devices to the intermediate version before they
can reach the target release.

## Troubleshooting

| Symptom | Check |
|---------|-------|
| "not configured" | No `url` in `update-check` config |
| "auto-apply requires server to provide sha256" | Server manifest missing `sha256` field |
| "waiting for spread" | Spread delay not elapsed; wait or use `update system firmware download` |
| "waiting for maintenance window" | Outside configured window; wait or use `update system firmware apply` |
| "paused by server" | Server manifest has `paused: true`; remove pause file or send SIGUSR1 |
| "checksum mismatch" | Binary changed on server after SHA-256 was computed; restart `ze update serve` |
| "insufficient disk space" | Need 2x binary size free on the target filesystem |
| "self-update not supported on read-only filesystem" | Binary is on a read-only mount (gokrazy, squashfs) |
| "binary and temp directory must be on the same filesystem" | Temp files must be on the same filesystem as the binary for atomic rename |
| "upgrade requires intermediate version" | Running version too old for direct upgrade; update to the intermediate version first |
| "no previous version available" | No `.prev` file for rollback (first install or already rolled back) |

History (`show system update history`) shows the result of each update attempt,
including failures with their reason. History survives restarts.

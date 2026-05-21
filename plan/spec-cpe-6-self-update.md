# Spec: cpe-6 -- Self-Update

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-cpe-5-firmware-update (done, learned 714) |
| Phase | 10/12 |
| Updated | 2026-05-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/system/update.go` - existing version check
4. `cmd/ze/update_serve.go` - existing update server
5. `cmd/ze/hub/main_system.go` lines 229-303 - hub integration
6. `internal/component/cmd/show/update.go` - show handler
7. `plan/learned/714-cpe-5-firmware-update.md` - prior design decisions

## Task

Extend ze's firmware update mechanism from version-check-only to full self-update:
download the binary, verify its SHA-256 checksum, atomically replace the running binary,
and optionally restart. The feature targets deployments where ze runs as a standalone
binary on standard Linux (not gokrazy, which has its own update mechanism).

Fleet-scale requirements: spread scheduling (avoid thundering herd), maintenance
windows, server-side pause (halt rollout of a bad release), update history, and
manual override for operators.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - component architecture
  -> Decision: self-update is an extension of system config, not a new component (consistent with cpe-5)
  -> Constraint: registration pattern for new CLI commands
- [ ] `plan/learned/714-cpe-5-firmware-update.md` - cpe-5 design decisions
  -> Decision: system config extension, report bus for notifications, date-based versioning
  -> Constraint: web UI does not expose version (fingerprinting hardening)
- [ ] `ai/patterns/cli-command.md` - CLI command pattern
  -> Constraint: YANG-driven dispatch, action before identifier
- [ ] `ai/patterns/config-option.md` - config option pattern
  -> Constraint: YANG schema, env.MustRegister, string values survive JSON roundtrip

**Key insights:**
- cpe-5 deliberately kept ze as check-only; self-update is the next step for non-gokrazy
- Report bus is the right integration point for status (warnings already wired)
- `ze update-serve` already serves binaries at `/<goos>/<goarch>`, just needs checksum + pause
- Existing `UpdateChecker` is the natural home for the download/verify/stage logic
- Atomic rename requires temp file on same filesystem as target binary

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/system/update.go` - UpdateChecker: periodic HTTP fetch of version.json, lexicographic comparison, report bus warning
  -> Constraint: check() runs on ticker; fetchVersion() does GET with X-Ze-Arch header
  -> Constraint: UpdateStatus holds LastCheck, RunningVersion, RemoteVersion, UpdateAvailable, LastError
- [ ] `cmd/ze/update_serve.go` - standalone server: GET /version.json (with arch validation), GET /<goos>/<goarch> (serves binary)
  -> Constraint: serves the running binary via os.Executable() path; intended for build infra
- [ ] `cmd/ze/hub/main_system.go` lines 229-303 - startUpdateChecker/applyUpdateCheckerFromMap/stopActiveUpdateChecker
  -> Constraint: lifecycle: start at boot, restart on reload (SIGHUP), stop at shutdown
- [ ] `internal/component/cmd/show/update.go` - handleShowSystemUpdate returns map with status, version, last-check, errors
  -> Constraint: status enum: "update available", "check failed", "not configured", "up to date"
- [ ] `internal/component/config/system/schema/ze-system-conf.yang` lines 326-341 - update-check container: url (string), interval (uint32, 60..604800, default 86400)
  -> Constraint: YANG container under system {}
- [ ] `internal/core/version/version.go` - Release(), BuildDate(), HTTPHeader()
  -> Constraint: version set via ldflags; date-based YY.MM.DD format

**Behavior to preserve:**
- Existing version check (fetch + compare) unchanged in semantics
- `show system update` output fields preserved (add new fields, don't remove)
- URL validation: HTTPS required, HTTP only for 127.0.0.1/localhost
- Report bus warning for "newer firmware available"
- `ze update-serve` backward-compatible (old clients still work with new server)
- Web UI still does not expose version (fingerprinting hardening)

**Behavior to change:**
- Extend manifest response with sha256, size, paused flag
- Extend UpdateChecker to download, verify, stage
- Extend UpdateStatus with download/staging/restart state
- Add new YANG config leaves for self-update policy
- Add new CLI commands for manual update operations
- Add new server-side pause mechanism

## Data Flow (MANDATORY)

### Entry Point
- Timer tick in UpdateChecker.run() (existing)
- Manual CLI command: `update system firmware apply` (new)
- Config reload changing self-update settings (existing pattern)

### Transformation Path

**Check phase (existing, extended):**
1. UpdateChecker.check() fires on ticker interval
2. fetchVersion() GETs /version.json with X-Ze-Arch header
3. Server responds with extended manifest: `{"version":"26.05.20","sha256":"...","size":52428800,"paused":false}`
4. Client parses manifest, compares version (existing logic)
5. If paused=true: report "update available, paused by server"; stop here
6. If not newer: clear warning; stop here

**Download phase (new):**
7. If auto-apply disabled: stop here (warn only, existing behavior)
8. If already holding a verified temp for this exact version: skip to stage phase
9. If holding a verified temp for a DIFFERENT version (server published newer):
   delete stale temp, reset download state, continue with new version
10. If spread delay not yet elapsed: stop here (wait)
11. If sha256 absent in manifest: refuse download, report "auto-apply requires
    server to provide sha256"; stop here (unverified binaries not allowed for
    unattended updates)
12. Download binary from download-url (or derived URL) to temp file on same
    filesystem as target. Enforce hard cap: abort if response exceeds 500 MB
13. Compute SHA-256 of downloaded file, compare against manifest
14. If mismatch: report error, delete temp file, stop
15. Record download state: `heldVersion` + `verifiedTempPath` (in-memory, on UpdateChecker)

**NOTE:** Download and verification proceed regardless of maintenance window.
The maintenance window only gates binary replacement (stage phase). This ensures
the binary is ready the moment the window opens.

**Stage phase (new, gated by maintenance window):**
16. If maintenance window configured AND current time is outside window: keep verified
    temp file, report "downloaded, waiting for maintenance window"; stage on next check()
    tick that falls inside the window
17. Copy permissions from current binary to temp file (before any rename)
18. Hard-link current binary to target.prev: os.Link(target, target.prev) -- backup for rollback
19. Atomic rename: os.Rename(temp, target) -- POSIX atomic replace on same filesystem
20. Clear download state (heldVersion, verifiedTempPath)
21. Report "update staged" via report bus

**Restart phase (new):**
22. Based on restart policy:
    - `immediate`: exec the new binary (syscall.Exec replaces process)
    - `scheduled`: wait for configured time, then exec
    - `manual`: report "update staged, restart required"; wait for operator

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Client -> Server | HTTPS GET /version.json (manifest) | [ ] |
| Client -> Server | HTTPS GET /<goos>/<goarch> (binary) | [ ] |
| Config -> UpdateChecker | YANG tree extraction (existing pattern) | [ ] |
| UpdateChecker -> Report Bus | RaiseWarning/ClearWarning (existing pattern) | [ ] |
| CLI -> UpdateChecker | RPC handler calls methods on active checker | [ ] |

### Integration Points
- `system.UpdateChecker` - extended with download/verify/stage/restart
- `system.UpdateStatus` - extended with new state fields
- `report.RaiseWarning`/`ClearWarning` - status notifications
- `cmd/ze/hub/main_system.go` - lifecycle management (existing)
- `ze update-serve` - enhanced manifest, pause mechanism
- YANG schema - new config leaves

### Architectural Verification
- [ ] No bypassed layers (uses existing UpdateChecker, report bus, YANG config)
- [ ] No unintended coupling (self-update is system config extension, not new component)
- [ ] No duplicated functionality (extends existing check, does not recreate)
- [ ] Zero-copy preserved where applicable (binary download is streaming, not buffered)

## Design

### Enhanced Manifest

The server response to GET /version.json is extended. Old fields preserved for
backward compatibility. New fields are optional (old clients ignore them, new
clients tolerate their absence).

```json
{
    "version": "26.05.20",
    "sha256": "a1b2c3d4e5f6...64hex",
    "size": 52428800,
    "paused": false,
    "minimum-version": "26.04.01",
    "download-url": "https://update.example.com/linux/arm64"
}
```

| Field | Type | Required | Purpose |
|-------|------|----------|---------|
| `version` | string | yes | Release version (existing) |
| `sha256` | string | no | Hex-encoded SHA-256 of the binary for the requesting arch |
| `size` | int | no | Binary size in bytes (progress reporting, pre-flight disk check). When absent, disk space pre-flight is skipped |
| `paused` | bool | no | Server-side pause flag; true = do not download |
| `minimum-version` | string | no | Oldest version allowed to skip to this release (forces sequential upgrades if set) |
| `download-url` | string | no | Absolute URL to download the binary. When absent, derived from version-check URL (see below) |

Notes:
- `sha256` is per-architecture because the server already dispatches on X-Ze-Arch.
  A single response serves one arch. SHA-256 (not SHA-1: SHA-1 is cryptographically broken).
- `minimum-version` addresses the case where a release requires schema migrations or
  config changes that only exist in an intermediate release. If the running version
  is older than minimum-version, the client reports "upgrade path requires intermediate
  version" and does not download. The operator must update to the intermediate version first.
  Non-numeric versions ("dev") are never blocked by minimum-version (consistent with
  `isNewer` behavior: dev builds short-circuit all version comparisons to false).
- `paused` absent or false means "not paused." The server defaults to not paused.
- `size` absent: disk space pre-flight check is skipped. The download still proceeds
  but without the safety check. The `ze update-serve` server always sets this field.

### Download URL Derivation

When `download-url` is present in the manifest, the client uses it directly.
When absent, the client derives the download URL from the configured version-check
URL by replacing the last path segment with `<runtime.GOOS>/<runtime.GOARCH>`:

```
Config URL:   https://update.example.com/version.json
Derived:      https://update.example.com/linux/arm64

Config URL:   https://cdn.example.com/ze/v1/version.json
Derived:      https://cdn.example.com/ze/v1/linux/arm64
```

This matches the existing `ze update-serve` endpoint layout where `/version.json`
and `/<goos>/<goarch>` share the same base path. The `download-url` field allows
operators to use a CDN or separate download server without changing the version-check
URL.

### Server-Side Pause Mechanism

`ze update-serve` gains a pause toggle. Two mechanisms (operator chooses):

1. **File-based:** if `<binary-dir>/update-paused` exists, manifest returns `"paused":true`.
   Operator creates/removes the file. No restart needed.
2. **Signal-based:** SIGUSR1 toggles pause state. Logged on toggle.

Both are ORed: if either is active, the manifest reports paused.

The pause flag is advisory: a determined client could ignore it. But the standard
UpdateChecker respects it unconditionally.

### Server-Side SHA-256

`ze update-serve` computes SHA-256 of the served binary at startup (once) and
includes it in the manifest response. If the binary changes on disk (rebuild),
the server must be restarted. This is acceptable: `ze update-serve` runs on
build infrastructure, not on routers.

### Client-Side Self-Update Flow

```
                         +-----------+
                         |  check()  |  <- ticker or manual trigger
                         +-----+-----+
                               |
                         fetch manifest
                               |
                    +----------+-----------+
                    |                      |
               not newer              newer available
               (clear warning)            |
                                   +------+------+
                                   |             |
                                paused       not paused
                                (warn only)      |
                                          auto-apply?
                                   +------+------+
                                   |             |
                                  no           yes
                              (warn only)        |
                                          spread delay elapsed?
                                   +------+------+
                                   |             |
                                  no           yes
                              (wait)             |
                                          download binary
                                               |
                                          verify SHA-256
                                   +------+------+
                                   |             |
                                 fail          pass
                             (error, retry       |
                              next interval)     |
                                          in maintenance window?
                                   +------+------+
                                   |             |
                                  no           yes
                              (hold temp,        |
                               wait)        hard-link backup
                                               |
                                          atomic rename
                                               |
                                          restart policy
                                   +------+------+------+
                                   |      |             |
                              immediate  scheduled    manual
                              (exec now) (wait time)  (warn)
```

### Spread

Purpose: prevent all devices from downloading simultaneously when version.json
changes. Each device picks a random delay in `[0, spread)` seconds on first
detection of a new version. The delay is stable for a given version (seeded
from device identity + version string) so restarts don't re-randomize.

Default spread: 3600 seconds (1 hour). Range: 0..86400.
A fleet of 100 devices with 3600s spread distributes downloads over ~1 hour.

The seed is derived from a device identity string plus the target version string.
The identity is resolved via the following fallback chain (first non-empty wins):

1. `/etc/machine-id` (standard on systemd Linux, unique per install)
2. zefs instance ID (`meta/instance/id` if present)
3. System hostname from config (`system { host ... }`)
4. `os.Hostname()`
5. Crypto-random 16 bytes (generated once at startup, not persisted)

Fallback 5 means spread always works, even on a misconfigured device, but
restarts re-randomize. Fallbacks 1-4 are stable across restarts.

The identity + version string are hashed (FNV-1a) to produce a uint64 seed,
then `seed % spread` gives the delay in seconds. This makes the delay
deterministic per device per version, but different across devices and
across versions.

### Maintenance Window

Optional time-of-day range when binary replacement may occur. Download and
verification proceed at any time (outside the window too). Outside the window,
the verified temp file is held and binary replacement is deferred until the
next check() tick inside the window. If no window is configured, binary
replacement proceeds as soon as verification passes.

The window uses local time (the router's configured timezone). Format: HH:MM.
Windows that cross midnight are valid (e.g., start=22:00 end=06:00).

### Atomic Binary Replacement

The replacement sequence, designed for POSIX filesystems. The critical property
is that the target path is **never empty**: if the process is killed at any
point, either the old or new binary is at the target path.

1. Determine target path: `os.Executable()` resolved via `filepath.EvalSymlinks`
   (use `internal/core/paths` which already does this)
2. Pre-flight: create and immediately remove a temp file in target directory to
   verify the filesystem is writable. If EROFS: abort with "self-update not
   supported on read-only filesystem" (catches gokrazy/squashfs mounts early)
3. Create temp file in same directory: `target + ".update." + random`
4. Write downloaded bytes to temp file (streaming, not buffered)
5. Copy permissions from current binary to temp file (`os.Chmod` matching `os.Stat` mode)
6. Hard-link current binary to backup: `os.Link(target, target+".prev")`
   If `.prev` already exists, remove it first then re-link.
   Hard-link is a metadata operation: the original inode stays at `target`.
7. `os.Rename(tmpPath, targetPath)` -- POSIX atomic replace. The old inode is
   unlinked from `target` (but still accessible via `.prev` hard link) and the
   new inode takes its place. At no point is `target` absent.
8. If rename fails with `syscall.EXDEV` (cross-device): error with clear message:
   "binary and temp directory must be on the same filesystem". Do not fall back
   to copy (not atomic).

Safety properties:
- Kill between steps 6 and 7: target still has old binary, .prev also points to it
- Kill during step 7: atomic, either old or new is at target
- Kill after step 7: target has new binary, .prev has old binary
- Power loss at any point: no window where target is absent

### Restart

Three modes, determined by the `restart {}` container:

| Config | Behavior |
|--------|----------|
| `restart { immediate }` | After staging: `syscall.Exec(resolvedTargetPath, os.Args, os.Environ())` replaces the process. Uses the resolved absolute path (from `filepath.EvalSymlinks`), not `os.Args[0]` which may be relative |
| `restart { time 03:00 }` | Wait until the configured daily time, then exec. If the time has passed today, waits until tomorrow |
| No `restart {}` block | Manual: report "update staged, restart to activate"; operator runs `update system firmware restart` |

For `immediate` and `time`, ze logs the restart reason and emits a report
bus warning before exec. If ze runs under a process supervisor (systemd, runit),
the supervisor restarts it automatically after exec or exit.

On `immediate`, the exec happens after a brief delay (5 seconds) to allow
in-flight BGP messages to drain. This is best-effort, not graceful shutdown.
For graceful restart, use `restart { time }` or manual mode with proper BGP GR.

### Rollback

Automatic rollback is out of scope for this spec. The operator has:
- `target.prev` on the filesystem (one previous version)
- `update system firmware rollback` CLI: renames .prev back, restarts
- The server-side pause to stop further devices from updating

Rollback is destructive of the new version: `os.Rename(.prev, target)` replaces
the new binary with the old one. After rollback, `.prev` no longer exists and
the new version is gone from disk. To re-apply the update, the device must
re-download from the server. This is intentional: if you're rolling back, the
new version is presumed bad.

A future spec could add health-check-based automatic rollback (download new
version, restart, if health check fails within N seconds, revert to .prev).

### Update History

The checker maintains a circular buffer of the last 20 update events. Each
event records: timestamp, from-version, to-version, result (success,
failed-download, failed-checksum, failed-stage, paused,
skipped-maintenance-window).

History is persisted to `<binary-dir>/ze-update-history.json` (same directory
as the binary, same filesystem). Written after each event (atomic: write to
temp, rename). Read at startup to populate the buffer. This is critical
because self-update restarts the process: without persistence, the "success"
record for the update that just completed would be lost immediately.

The file is a JSON array of event objects, capped at 20 entries. If the file
is missing or corrupt, the buffer starts empty (no error, just no history).

`show system update history` displays this list.

### Config Reload During Download

When config reload (SIGHUP) triggers `applyUpdateCheckerFromMap`, the old
checker is stopped via `Stop()` which cancels its context. An in-flight
download uses this context: the HTTP request is cancelled, the temp file
is cleaned up in the deferred cleanup of the download function. The new
checker starts fresh with no held state.

### Stale Temp File Cleanup

At startup, before the first check(), the checker scans the binary's directory
for files matching `<binary-name>.update.*` and removes them. These are
leftover temp files from a previous crash during download. This is safe
because a verified temp file that was waiting for a maintenance window is
also lost on crash/restart, and will be re-downloaded on the next check.

### Minimum-Version (Sequential Upgrade Enforcement)

When `minimum-version` is set in the manifest and the running version is older
than minimum-version, the client does not download. It reports:

> "upgrade to [version] requires intermediate version [minimum-version] first"

This handles releases that include breaking changes (config schema migrations,
protocol version bumps) that require a specific upgrade path.

### Download Safety

| Concern | Mitigation |
|---------|------------|
| Disk space | Pre-flight check: manifest `size` field compared to available space on target filesystem. Abort if < 2x size (need room for temp + current + prev). Skipped when `size` is absent from manifest |
| Partial download | Temp file deleted on any error. No partial binaries left on disk |
| Interrupted download | Next check interval retries from scratch (no resume). Binary sizes are typically < 100 MB, full re-download is acceptable |
| Concurrent downloads | Mutex in UpdateChecker prevents concurrent download/stage operations |
| Binary in use | On Linux, renaming a running binary is safe. The kernel keeps the inode open. The new binary takes effect on next exec |
| Cross-filesystem | Detected by os.Rename error. Reported as configuration error (binary path and temp must be on same FS). No silent fallback |
| Download URL validation | `download-url` from manifest validated with same rules as config URL: HTTPS required, HTTP only for localhost. Reject and report error if validation fails |
| Unverified binary | Auto-apply refuses download when `sha256` absent. Manual commands warn and proceed. Never stage a binary that failed checksum verification |
| Download size cap | Hard cap of 500 MB on response body. Abort if exceeded, regardless of manifest `size` field. Prevents resource exhaustion from a rogue server |

## YANG Config

Extend the existing `update-check` container. New leaves are additive;
existing `url` and `interval` unchanged.

```yang
container update-check {
    description "Firmware version check and self-update";

    leaf url {
        type string;
        description "HTTPS URL serving version manifest JSON";
    }

    leaf interval {
        type uint32 { range "60..604800"; }
        default 86400;
        description "Check interval in seconds";
    }

    // --- New leaves/containers below ---

    leaf auto-apply {
        type boolean;
        default false;
        description "When true, automatically download, verify, and stage
                     updates. When false, only check and report.
                     Default false preserves existing check-only behavior.";
    }

    leaf spread {
        type uint32 { range "0..86400"; }
        default 3600;
        description "Maximum random delay in seconds before downloading
                     after a new version is detected. Prevents all
                     devices from hitting the server simultaneously.
                     0 disables spread (download immediately).";
    }

    container maintenance-window {
        description "Time window when binary replacement may occur.
                     Download and verification proceed at any time.
                     Outside this window, the verified binary is held
                     and replacement is deferred until the window opens.
                     If not configured, replacement proceeds immediately.";

        leaf start {
            type string;
            description "Start time HH:MM in local time. Example: '02:00'.";
        }

        leaf end {
            type string;
            description "End time HH:MM in local time. Example: '06:00'.
                         May be before start (window crosses midnight).";
        }
    }

    container restart {
        description "What to do after an update is staged.
                     If omitted: manual (stage only, operator restarts).
                     If present with 'immediate': restart after brief drain.
                     If present with 'time': restart at that daily time.";

        leaf immediate {
            type empty;
            description "Restart automatically after staging (5s drain delay).
                         Mutually exclusive with 'time'.";
        }

        leaf time {
            type string;
            description "Daily restart time, HH:MM in local time.
                         Implies scheduled restart. If the time has
                         passed today, waits until tomorrow.
                         Mutually exclusive with 'immediate'.";
        }
    }
}
```

Restart semantics:
- No `restart {}` block: manual. Stage the binary, raise a warning, wait for operator.
- `restart { immediate }`: restart after a 5-second drain delay.
- `restart { time 03:00 }`: restart at the configured daily time.
- Both `immediate` and `time` present: config error, rejected at load.

### Config Validation

At config load and reload, validate:
- `restart { time }` and `maintenance-window` start/end parse as HH:MM (00:00..23:59).
  Reject with config error if malformed.
- `restart { immediate }` and `restart { time }` are mutually exclusive.
  Both present: reject with config error.
- If both `maintenance-window` and `restart { time }` are set and the time falls
  outside the maintenance window: log a warning "restart time is outside
  maintenance-window; binary will be staged during window but restart will happen
  after it closes." Not an error (the operator may intend this), but worth flagging.

### Config Examples

**Fully automated (fleet):**
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
Check every hour, auto-download with up to 30 minutes random spread,
replace binary only between 2 AM and 6 AM, restart at 3 AM.

**Immediate restart (lab/single device):**
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
Check every 5 minutes, no spread, no maintenance window, restart as soon as staged.

**Check-only (default, existing behavior):**
```
system {
    update-check {
        url https://update.example.com/version.json
    }
}
```
Daily check, report only. No download, no staging, no restart.

## CLI Commands

### Show Commands (read-only)

**`show system update`** (extended, backward compatible):

Existing fields preserved. New fields added:

| Field | Value |
|-------|-------|
| running-version | 26.05.15 |
| remote-version | 26.05.20 |
| update-available | true |
| status | staged |
| last-check | 2026-05-20T14:30:00Z |
| download-status | complete |
| download-sha256 | a1b2c3...verified |
| staged-version | 26.05.20 |
| staged-path | /usr/local/bin/ze |
| restart | time 03:00 |
| server-paused | false |

New status values: "downloading", "verifying", "staged", "restarting",
"paused by server", "error: checksum mismatch", "error: download failed",
"waiting for maintenance window", "waiting for spread".

Both show commands return data via `plugin.Response{Data: map}` and route
through `ApplyPipes`, supporting all pipe operators (`| json`, `| table`,
`| match`, `| count`, `| yaml`, `| text`, `| resolve`, `| origin`, `| no-more`).

**`show system update history`** (new):

| Timestamp | From | To | Result |
|-----------|------|----|--------|
| 2026-05-20T03:00:12Z | 26.05.15 | 26.05.20 | success |
| 2026-05-13T03:01:44Z | 26.05.10 | 26.05.15 | success |
| 2026-05-06T03:00:08Z | 26.05.05 | 26.05.10 | failed-checksum |

### Action Commands

The existing `update` verb ("refresh stale data from external sources") is the
natural home. Firmware is data refreshed from an external source. Commands live
under `update system firmware <action>`, consistent with the existing
`update bgp peer prefix` pattern.

| Command | Action |
|---------|--------|
| `update system firmware check` | Trigger immediate version check (bypass interval timer) |
| `update system firmware download` | Download now (bypass spread and maintenance window) |
| `update system firmware apply` | Download+verify+stage+restart (full one-shot cycle, bypasses all scheduling) |
| `update system firmware restart` | Restart into staged version now |
| `update system firmware rollback` | Restore .prev binary and restart |

All `update system firmware` commands are RPC-only (no config state change).
They override the automated schedule for one-shot operation.

`update system firmware apply` is the "do everything now" command for operators
who want to update a single device interactively. It checks, downloads if
needed, verifies, stages, and restarts (regardless of restart config).

Manual commands (`update system firmware apply/download`) bypass server-side
pause (pause is for automated fleet rollout, not manual intervention).

When sha256 is absent from the manifest, manual commands warn
"no checksum available, binary will not be verified" and proceed.
Auto-apply refuses (see Data Flow step 11).

## Server Enhancements (`ze update-serve`)

### Enhanced Manifest

GET /version.json now returns the enhanced manifest (see Design > Enhanced
Manifest for full field list). `ze update-serve` always populates all fields:
`sha256` computed at startup, `size` from file stat, `paused` from current
pause state, `download-url` derived from listen address and arch path.

### Pause Toggle

Two mechanisms:

1. **File:** if a file named `update-paused` exists in the same directory as the
   served binary, `"paused": true`. Create/remove the file to toggle.
   Checked on each request (no restart needed).

2. **SIGUSR1:** toggles in-memory pause state. Logged:
   `"update serving paused"` / `"update serving resumed"`.

Both are ORed: if either is active, the manifest reports paused.

### Checksum Endpoint

New endpoint: `GET /<goos>/<goarch>/sha256` returns the raw hex digest
as text/plain. Alternative verification path for scripts and tooling
that don't parse JSON.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config `auto-apply true` + timer tick | -> | UpdateChecker.download() + verify() + stage() | `TestSelfUpdateFullCycle` |
| `update system firmware apply` CLI | -> | handleUpdateSystemFirmwareApply() -> UpdateChecker | `test/system/self-update-manual.ci` |
| `ze update-serve` GET /version.json | -> | manifest handler returns sha256+paused | `TestUpdateServeEnhancedManifest` |
| `ze update-serve` pause file | -> | manifest returns paused=true | `TestUpdateServePause` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | New version available, auto-apply true, not paused | Binary downloaded to temp, SHA-256 verified, staged via atomic rename |
| AC-2 | SHA-256 mismatch after download | Download rejected, temp deleted, error reported, no binary replaced |
| AC-3 | Server manifest has `"paused": true` | Client reports "update available, paused by server"; no download |
| AC-4 | `update system firmware apply` while update available | Immediate download+verify+stage+restart, bypassing spread/window/pause |
| AC-5 | Spread configured at 3600, two devices check same version | Each device waits a different random delay (deterministic per device+version) |
| AC-6 | Maintenance window 02:00-06:00, update detected at 14:00 | Download + verify proceed immediately; binary replacement (staging) deferred until 02:00 |
| AC-7 | `restart { immediate }`, update staged | Process restarts via exec within 5 seconds |
| AC-8 | `restart { time 03:00 }`, update staged | Process restarts at 03:00 (next occurrence) |
| AC-9 | No `restart {}` block (manual), update staged | Warning raised "update staged, restart to activate"; no automatic restart |
| AC-10 | `update system firmware rollback` with .prev file | .prev renamed to target, process restarts |
| AC-11 | `update system firmware rollback` without .prev file | Error: "no previous version available" |
| AC-12 | `show system update` after staging | Status shows "staged", staged-version, staged-path, download-sha256 |
| AC-13 | `show system update history` after two updates | Two entries with correct timestamps, versions, results |
| AC-14 | minimum-version in manifest, running version too old | Error: "upgrade requires intermediate version X"; no download |
| AC-15 | auto-apply=false (default), new version available | Check reports availability (existing behavior); no download |
| AC-16 | Disk space < 2x binary size (manifest `size` present) | Download aborted with "insufficient disk space" error. When `size` absent, check is skipped |
| AC-17 | `ze update-serve` with pause file present | GET /version.json returns `"paused": true` |
| AC-18 | `ze update-serve` SIGUSR1 | Pause state toggled, next manifest reflects new state |
| AC-19 | Old client (no sha256 support) hits enhanced server | Gets version field, ignores unknown fields; existing behavior preserved |
| AC-20 | Binary path on different filesystem than /tmp | Cross-filesystem detected, temp file created in binary's directory (same FS) |
| AC-21 | Config reload changes auto-apply from false to true | Checker restarts with new policy; pending update proceeds |
| AC-22 | `update system firmware check` while interval timer has not fired | Immediate version check; status updated; timer reset |
| AC-23 | `update system firmware download` while update available, outside maintenance window | Immediate download+verify, bypassing spread and maintenance window; binary held as verified temp |
| AC-24 | auto-apply=true, manifest has no sha256 field | Download refused with "auto-apply requires server to provide sha256"; no binary fetched |
| AC-25 | `update system firmware apply` while server-paused | Proceeds (manual commands bypass pause); downloads, verifies, stages, restarts |
| AC-26 | `update system firmware apply`, manifest has no sha256 | Warns "no checksum available, binary will not be verified"; proceeds with download and stage |
| AC-27 | Manifest `download-url` is `http://evil.com/ze` | Download refused with URL validation error (HTTPS required) |
| AC-28 | Device holds verified temp for v26.05.20, server publishes v26.05.21 | Old temp deleted, new version downloaded and verified |
| AC-29 | Device holds verified temp for v26.05.20, next check() same version, still outside window | No re-download; temp held, status "waiting for maintenance window" |
| AC-30 | ze starts with stale `.update.*` files in binary directory | Stale temp files cleaned up before first check |
| AC-31 | `show system update history` after restart following successful update | History persisted; shows the successful update record from before restart |
| AC-32 | `restart { time 07:00 }`, maintenance-window 02:00-06:00 | Config accepted with warning "restart time is outside maintenance-window" |
| AC-33 | `restart { immediate }` and `restart { time 03:00 }` both present | Config rejected with error (mutually exclusive) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSelfUpdateFullCycle` | `internal/component/config/system/selfupdate_test.go` | Download+verify+stage with mock server | |
| `TestSelfUpdateChecksumMismatch` | `internal/component/config/system/selfupdate_test.go` | SHA-256 mismatch rejects download | |
| `TestSelfUpdateServerPaused` | `internal/component/config/system/selfupdate_test.go` | Paused manifest stops download | |
| `TestSelfUpdateSpreadDeterministic` | `internal/component/config/system/selfupdate_test.go` | Same device+version = same spread; different device = different spread | |
| `TestSelfUpdateMaintenanceWindow` | `internal/component/config/system/selfupdate_test.go` | In-window proceeds, out-of-window defers | |
| `TestSelfUpdateMaintenanceWindowMidnight` | `internal/component/config/system/selfupdate_test.go` | Window crossing midnight works correctly | |
| `TestSelfUpdateMinimumVersion` | `internal/component/config/system/selfupdate_test.go` | Running < minimum = blocked | |
| `TestSelfUpdateDiskSpaceCheck` | `internal/component/config/system/selfupdate_test.go` | Insufficient space aborts | |
| `TestSelfUpdateAtomicRename` | `internal/component/config/system/selfupdate_test.go` | Temp created in target dir, rename succeeds, .prev created | |
| `TestSelfUpdateRollback` | `internal/component/config/system/selfupdate_test.go` | .prev renamed to target | |
| `TestSelfUpdateHistory` | `internal/component/config/system/selfupdate_test.go` | Circular buffer records events, caps at 20 | |
| `TestUpdateServeEnhancedManifest` | `cmd/ze/update_serve_test.go` | Manifest includes sha256, size, paused | |
| `TestUpdateServePauseFile` | `cmd/ze/update_serve_test.go` | Pause file toggles paused field | |
| `TestUpdateServePauseSignal` | `cmd/ze/update_serve_test.go` | SIGUSR1 toggles paused field | |
| `TestUpdateServeChecksumEndpoint` | `cmd/ze/update_serve_test.go` | GET /<os>/<arch>/sha256 returns hex digest | |
| `TestUpdateServeBackwardCompat` | `cmd/ze/update_serve_test.go` | Old-format response fields still present | |
| `TestSelfUpdateNoSha256AutoApply` | `internal/component/config/system/selfupdate_test.go` | Auto-apply refuses download when sha256 absent | |
| `TestSelfUpdateManualNoSha256` | `internal/component/config/system/selfupdate_test.go` | Manual apply warns but proceeds without sha256 | |
| `TestSelfUpdateManualBypassesPause` | `internal/component/config/system/selfupdate_test.go` | Manual apply proceeds when server-paused | |
| `TestSelfUpdateDownloadURLValidation` | `internal/component/config/system/selfupdate_test.go` | HTTP download-url rejected, HTTPS accepted | |
| `TestSelfUpdateHeldTempSkipsRedownload` | `internal/component/config/system/selfupdate_test.go` | Same version held: no re-download on next tick | |
| `TestSelfUpdateHeldTempDiscardedOnNewVersion` | `internal/component/config/system/selfupdate_test.go` | Different version: old temp deleted, new downloaded | |
| `TestSelfUpdateStaleCleanup` | `internal/component/config/system/selfupdate_test.go` | .update.* files removed at startup | |
| `TestSelfUpdateHistoryPersist` | `internal/component/config/system/selfupdate_test.go` | History survives write+read cycle, corrupt file = empty | |
| `TestSelfUpdateConfigValidation` | `internal/component/config/system/selfupdate_test.go` | immediate+time rejected; time outside window warns | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| spread | 0..86400 | 86400 | N/A (0 valid) | 86401 |
| interval | 60..604800 | 604800 | 59 | 604801 |
| maintenance-window start | 00:00..23:59 | 23:59 | N/A | 24:00 |
| maintenance-window end | 00:00..23:59 | 23:59 | N/A | 24:00 |
| restart time | 00:00..23:59 | 23:59 | N/A | 24:00 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `self-update-download` | `test/system/self-update-download.ci` | Configure auto-apply, mock server returns new version, verify binary staged | |
| `self-update-paused` | `test/system/self-update-paused.ci` | Server paused, verify no download despite new version | |
| `self-update-manual` | `test/system/self-update-manual.ci` | update system firmware apply triggers full cycle | |
| `self-update-rollback` | `test/system/self-update-rollback.ci` | Stage update, rollback, verify .prev restored | |
| `self-update-history` | `test/system/self-update-history.ci` | Update, restart, verify history shows the update record | |

## Files to Modify
- `internal/component/config/system/update.go` - extend UpdateChecker with download/verify/stage/restart logic, extend UpdateStatus
- `internal/component/config/system/schema/ze-system-conf.yang` - add new leaves to update-check container
- `cmd/ze/update_serve.go` - enhanced manifest (sha256, size, paused), pause mechanism, checksum endpoint
- `cmd/ze/hub/main_system.go` - pass new config fields to UpdateChecker
- `internal/component/cmd/show/update.go` - extend show handler with new status fields
- `internal/component/config/system/system.go` - extend SystemConfig with new fields (UpdateAutoApply, UpdateSpread, etc.)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new leaves) | [x] | `internal/component/config/system/schema/ze-system-conf.yang` |
| CLI commands/flags | [x] | New RPC handlers under `update system firmware *` |
| CLI show extension | [x] | `internal/component/cmd/show/update.go` |
| Functional test | [x] | `test/system/self-update-*.ci` |
| YANG show schema | [x] | `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` - self-update capability |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` - update-check extensions |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` - update system firmware *, show system update history |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` - new RPCs |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/self-update.md` - operator guide for fleet updates |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` - self-update vs other NOS update mechanisms |
| 12 | Internal architecture changed? | [ ] | N/A (extension, not new component) |

## Files to Create
- `internal/component/config/system/selfupdate.go` - download, verify, stage, restart logic (separated from check logic for clarity)
- `internal/component/config/system/selfupdate_test.go` - unit tests
- `internal/component/cmd/update/firmware.go` - RPC handlers for update system firmware commands
- `internal/component/cmd/update/firmware_test.go` - handler tests
- `cmd/ze/update_serve_test.go` - server enhancement tests
- `test/system/self-update-download.ci` - functional test
- `test/system/self-update-paused.ci` - functional test
- `test/system/self-update-manual.ci` - functional test
- `test/system/self-update-rollback.ci` - functional test
- `test/system/self-update-history.ci` - functional test
- `docs/guide/self-update.md` - operator guide

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- YANG schema, entry point skeletons, failing wiring tests
   - Tests: `TestSelfUpdateFullCycle` (stub), `TestUpdateServeEnhancedManifest` (stub)
   - Files: ze-system-conf.yang, selfupdate.go skeleton, update_serve.go manifest extension, update/firmware.go skeleton
   - Verify: entry points exist and are reachable; wiring tests fail because logic is a stub

2. **Phase: Server enhancements** -- SHA-256, pause, checksum endpoint
   - Tests: `TestUpdateServeEnhancedManifest`, `TestUpdateServePauseFile`, `TestUpdateServePauseSignal`, `TestUpdateServeChecksumEndpoint`, `TestUpdateServeBackwardCompat`
   - Files: cmd/ze/update_serve.go, cmd/ze/update_serve_test.go
   - Verify: server returns enhanced manifest, pause works, old clients unaffected

3. **Phase: Download and verify** -- HTTP download, SHA-256 verification, disk space check
   - Tests: `TestSelfUpdateFullCycle`, `TestSelfUpdateChecksumMismatch`, `TestSelfUpdateDiskSpaceCheck`
   - Files: internal/component/config/system/selfupdate.go
   - Verify: download works against test server, checksum verified, bad checksum rejected

4. **Phase: Atomic stage** -- backup, rename, permissions
   - Tests: `TestSelfUpdateAtomicRename`
   - Files: internal/component/config/system/selfupdate.go
   - Verify: temp file created in same dir, rename atomic, .prev created

5. **Phase: Spread and maintenance window** -- scheduling logic
   - Tests: `TestSelfUpdateSpreadDeterministic`, `TestSelfUpdateMaintenanceWindow`, `TestSelfUpdateMaintenanceWindowMidnight`
   - Files: internal/component/config/system/selfupdate.go
   - Verify: spread deterministic, window enforcement correct

6. **Phase: Restart and rollback** -- exec, scheduled restart, rollback
   - Tests: `TestSelfUpdateRollback`
   - Files: internal/component/config/system/selfupdate.go
   - Verify: restart policies work, rollback restores .prev

7. **Phase: Minimum-version, history, status** -- sequential upgrade enforcement, circular buffer, extended show
   - Tests: `TestSelfUpdateMinimumVersion`, `TestSelfUpdateHistory`, `TestSelfUpdateServerPaused`
   - Files: selfupdate.go, show/update.go
   - Verify: minimum-version blocks, history records, show output correct

8. **Phase: CLI handlers** -- update system firmware commands
   - Tests: RPC handler tests
   - Files: internal/component/cmd/update/firmware.go, ze-cli-update-cmd.yang
   - Verify: manual commands trigger correct actions

9. **Phase: Hub integration** -- config extraction, lifecycle
   - Tests: config reload test
   - Files: cmd/ze/hub/main_system.go, system.go
   - Verify: config changes take effect on reload

10. **Functional tests** -- end-to-end .ci tests
11. **Full verification** -- `make ze-verify`
12. **Complete spec** -- audit tables, learned summary

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | SHA-256 computed correctly, atomic rename on same FS, spread deterministic |
| Naming | YANG uses kebab-case, JSON uses kebab-case, CLI uses `update system firmware <action>` |
| Data flow | Check -> download -> verify -> backup -> rename -> restart; no shortcut paths |
| Rule: backward-compat | Old clients work with enhanced server; existing show fields preserved |
| Rule: no-partial-completion | Auto-apply false (default) preserves check-only; all new features tested |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| YANG schema has new leaves | `grep 'auto-apply\|spread\|restart\|maintenance-window' ze-system-conf.yang` |
| selfupdate.go exists with download+verify+stage | `ls internal/component/config/system/selfupdate.go` |
| Server returns sha256 in manifest | `curl localhost:8080/version.json \| jq .sha256` |
| Server pause works | `touch update-paused && curl ... \| jq .paused` |
| Firmware CLI handlers exist | `grep -rn 'firmware' internal/component/cmd/update/` |
| Show extended fields | `grep -n 'staged-version\|download-status' internal/component/cmd/show/update.go` |
| Functional tests exist | `ls test/system/self-update-*.ci` |
| .prev backup created on stage | Unit test `TestSelfUpdateAtomicRename` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| HTTPS enforcement | URL validation applies to both config URL and manifest `download-url`: HTTPS required, HTTP only for localhost |
| Checksum verification | SHA-256 verified before any rename; mismatch = abort + delete temp |
| Binary permissions | Temp file gets same permissions as original; no world-writable |
| Path traversal | `os.Executable()` + `filepath.EvalSymlinks()` only; no user-supplied paths |
| Temp file cleanup | Temp deleted on any error (download, checksum, rename) |
| Signal injection | SIGUSR1 only toggles pause; no arbitrary code execution |
| Manifest parsing | JSON unmarshal with size limits (existing 64 KiB cap) |
| Denial of service | Download size bounded by hard cap (500 MB). Stale temp files cleaned at startup |
| Rollback safety | Rollback only renames .prev; does not download or fetch anything |
| Unverified binary | Auto-apply requires sha256. Manual commands warn but proceed. Never stage a failed-checksum binary |
| History file | Written atomically (temp+rename). Read tolerates corrupt/missing file. No sensitive data in history |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [pending]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-33 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-cpe-6-self-update.md`
- [ ] Summary included in commit

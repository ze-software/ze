# Health Checks and System Readiness

Ze provides two complementary systems for monitoring operational health:
**offline doctor checks** (pre-start readiness) and **runtime health monitoring**
(continuous anomaly detection during operation).

## Offline: `ze doctor`

Run `ze doctor` before starting the daemon to verify the environment is ready.
Add `--json` for machine-readable output with stable diagnostic codes.

```
ze doctor
ze doctor --json
ze doctor --json /path/to/config.conf
```

Each check produces a diagnostic with a code (e.g., `doctor-config-reference`),
severity (`error` or `warning`), and message. Use `ze explain <code>` for
remediation guidance.

### Checks

| Code | What it checks |
|------|----------------|
| `doctor-store-integrity` | zefs database corruption |
| `doctor-config-missing` | Config file resolution |
| `doctor-config-parse` | Config syntax |
| `doctor-config-reference` | Dangling policy/filter references |
| `doctor-tls-*` | TLS cert existence, expiry, validity |
| `doctor-plugin-missing` | Plugin binary on PATH |
| `doctor-service-executable` | Installed systemd unit `ExecStart` points to an executable ze binary |
| `doctor-service-user` / `doctor-service-group` | User/group referenced by installed `ze.service` exists |
| `doctor-ssh-hostkey-missing` | SSH host key |
| `doctor-listen-unavailable` | Port binding |
| `doctor-iface-missing` / `doctor-iface-down` | Interface existence and state |
| `doctor-disk-space` | Partition free space (<5%) |
| `doctor-dns-resolver` | Name server reachability |
| `doctor-clock-skew` | System clock vs NTP (>5 min) |
| `doctor-vpp-unreachable` | VPP API socket (Linux) |
| `doctor-vpp-version` | VPP version compatibility (Linux) |
| `doctor-module-missing` | Kernel modules (Linux) |
| `doctor-platform-detect` | Platform detection failure |
| `doctor-platform-unknown` | Unrecognized runtime platform |
| `doctor-platform-perm` | Gokrazy /perm not writable |
| `doctor-platform-container-ro` | Container with read-only root |

Exit code 0 means ready; exit code 1 means at least one error-severity issue.

## Runtime: `show health`

The health registry aggregates component status from the running daemon.

```
show health
show health | json
```

The `/health` HTTP endpoint returns the same data as JSON, with HTTP 503
when any component is `down`.

### Registered Components

| Component | What it monitors |
|-----------|-----------------|
| `bgp` | Session stuck, flap, EOR timeout warnings |
| `fib` | Sync failure, orphan routes, programming lag |
| `firewall` | Stale ze_* tables, chain drift vs config |
| `iface` | RX/TX error counter increases |
| `plugins` | Plugin crashes and disabled-by-respawn-limit |
| `vpp` | VPP API socket reachability (when present) |
| `ipsec` | IPsec SA state |
| `pki` | Certificate expiry |
| `l2tp` | L2TP subsystem availability |

Each check completes within 1 second. Components report `healthy`, `degraded`,
or `down`. The overall status is the worst of all components.

## Runtime: `show warnings` / `show errors`

The report bus surfaces individual anomalies:

- **Warnings** are state-based (raised when a condition exists, cleared when it
  resolves). Example: `session-stuck` clears when the peer reaches Established.
- **Errors** are event-based (recorded when something happens). Example:
  `fib-sync-failure` fires once per failed route install.

```
show warnings
show warnings | json
show errors
show errors | json
show warnings source bgp
```

### Report Bus Codes

| Code | Source | Type | Trigger |
|------|--------|------|---------|
| `session-stuck` | bgp | warning | Peer non-Established >5 min |
| `session-flap` | bgp | warning | >3 transitions in 5 min |
| `eor-timeout` | bgp | warning | EOR incomplete after GR restart-time |
| `route-count-anomaly` | bgp | error | >50% prefix drop (min 100 prefixes) |
| `prefix-threshold` | bgp | warning | Prefix count at warning level |
| `prefix-stale` | bgp | warning | Prefix data >180 days old |
| `fib-sync-failure` | fib | error | Backend add/replace/delete failed |
| `fib-orphan` | fib | warning | Orphan routes swept at startup |
| `fib-programming-lag` | fib | warning | Routes pending >30s |
| `firewall-stale-table` | firewall | warning | ze_* table in kernel not in config |
| `firewall-drift` | firewall | warning | Chain count mismatch vs config |
| `plugin-crash` | plugin | error | Plugin process exited unexpectedly |
| `plugin-down` | plugin | warning | Plugin disabled (respawn limit) |
| `iface-errors` | iface | warning | RX/TX error counters increasing |

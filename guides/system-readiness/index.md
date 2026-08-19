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
| `doctor-*-unreachable` | External-service reachability probes that warn (never fail startup) when a configured peer is down: RADIUS admin (`doctor-radius-admin-unreachable`), RPKI cache (`doctor-rpki-unreachable`), BMP collector (`doctor-bmp-unreachable`), NTP (`doctor-ntp-server-unreachable`), management hub (`doctor-hub-unreachable`) |
<!-- source: internal/component/managed/doctor.go -- checkHubReachable / doctor-hub-unreachable -->
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
| `flow-export` | Exporter state and collector send errors |
| `report-bus` | Report bus registration |

<!-- source: internal/plugins/flowexport/health.go -- RegisterHealthCheck, checkFlowExportHealth; internal/core/report/register.go -- init -->
Each check completes within 1 second. Components report `healthy`, `degraded`,
or `down`. The overall status is the worst of all components.

### Demo: Separate live warnings from recent errors

Read aggregate component health, follow a live stale-prefix warning from the SSH banner into show warnings, then reset a peer and find the retained event in show errors.

[Play the WebM recording](../../assets/demos/health-reports.webm?v=c95587e1ba) · [View the poster](../../assets/demos/health-reports.png?v=4b94b2625a) · [Plain-text transcript](../../assets/demos/health-reports.txt?v=113557fa9c)

Recorded with Ze 26.08.19 on macOS and Linux using VHS 0.11.0. Duration: 1 minute.

```console
$ ssh ze-demo
Warning: stale-prefix-data has stale prefix data (updated 2024-01-01)
ze# run show health
ipsec  down  ike engine not running
ze# run show warnings source bgp
bgp  prefix-stale  warning  127.0.0.2  ... 2024-01-01
ze# run request peer 127.0.0.2 teardown 4
ze# run show errors source bgp
bgp  notification-sent  error  127.0.0.2  direction sent  code 6  subcode 4

Health reports aggregate component state. Warnings describe conditions that remain active and disappear when resolved. Errors retain discrete events. The login banner and filtered commands use the same report bus.
```


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

## Adding Doctor Checks

Plugins and components extend `ze doctor` by registering their own checks.
There are two registration paths depending on whether the owner is a plugin.

### Internal plugins (preferred)

Declare checks in `registry.Registration.DoctorChecks`. The doctor runner
bridges these at execution time. Each check receives a `DoctorCheckContext`
(config tree + platform) and returns diagnostics.

```go
reg := registry.Registration{
    Name: "my-plugin",
    // ...
    DoctorChecks: []registry.DoctorCheckDef{{
        Name:         "my-plugin-reachable",
        Phase:        rpc.DoctorPhasePostConfig,
        Order:        700,
        Dependencies: []string{"config-parse"},
        Platforms:    []string{"any"},
        Codes:        []string{"doctor-my-plugin-unreachable"},
        Check:        checkMyService,
    }},
}
```

Reference example: `internal/component/l2tp/plugins/authradius/register.go`.

### Non-plugin components

Components that are not plugins (appliance, web, SSH) use
`diagnostic.RegisterDoctorCheck()` from their owning package's `init()`.

```go
func init() {
    diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
        Name:         "my-component-ready",
        Phase:        "post-config",
        Order:        500,
        Component:    "my-component",
        Dependencies: []string{"config-parse"},
        Platforms:    []string{"any"},
        Codes:        []string{"doctor-my-component-missing"},
        Check:        checkMyComponent,
    })
}
```

### External plugins (RPC)

External plugins declare checks in `DoctorChecks` during Stage 1
registration, then handle the `doctor-check` callback at runtime.

Declaration (in `sdk.Registration`):

```go
p.Run(ctx, sdk.Registration{
    DoctorChecks: []rpc.DoctorCheckDecl{{
        Name:         "my-cache-reachable",
        Phase:        rpc.DoctorPhasePostConfig,
        Order:        700,
        Dependencies: []string{"config-parse"},
        Platforms:    []string{"any"},
        Codes:        []string{"doctor-my-cache-unreachable"},
    }},
})
```

Callback handler:

```go
p.OnDoctorCheck(func(name string) ([]rpc.DoctorCheckDiagnostic, error) {
    if name == "my-cache-reachable" {
        if err := pingCache(); err != nil {
            return []rpc.DoctorCheckDiagnostic{{
                Code:     "doctor-my-cache-unreachable",
                Severity: "error",
                Message:  "cache server not reachable: " + err.Error(),
            }}, nil
        }
    }
    return nil, nil
})
```

The engine invokes plugin checks only at runtime (`show doctor`), not
during offline `ze doctor`. See `docs/architecture/api/process-protocol.md`
for the full protocol.

### Requirements

Every new doctor check must:

1. Use a diagnostic code with the `doctor-` prefix.
2. Register the code in `internal/core/diagnostic/codes.go` so
   `ze explain <code>` works.
3. Include a unit test proving the check fires when the relevant config
   is present and emits the registered code.

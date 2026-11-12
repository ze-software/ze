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
ze doctor --json config /path/to/config.conf
ze doctor --json kernel-capabilities
```

With no argument, `ze doctor` checks the active config in the store. `config
<file>` names a config file to check instead, and `config -` reads it from
stdin. The file is always the value of the `config` keyword: a bare path such as
`ze doctor router.conf` is refused with an error that names
`ze doctor config <file>`.
<!-- source: internal/component/doctor/doctor.go -- Run -->
<!-- source: internal/component/doctor/answer.go -- Answer, readDoctorRequest -->

`ze doctor kernel-capabilities` asks a different question: it probes every
kernel feature Ze enrolls, whatever the configuration uses, prints one row per
feature (subsystem, `CONFIG_` symbol, state, reason) and exits 0 only when every
row is `present`. It is how a host is checked before it runs Ze's Docker labs.
<!-- source: internal/component/doctor/kernel_capabilities.go -- writeKernelCapabilities -->

Each check produces a diagnostic with a code (e.g., `doctor-config-reference`),
severity (`error` or `warning`), and message. When a command clears the
problem, the finding also names it as its fix, drawn on a line of its own under
the message (JSON carries it under `fix`):

```
ERROR [doctor-fixture-down] fixture is down
      fix: ze start
```

Use `ze show diagnostic <code>` for remediation guidance.
<!-- source: internal/component/doctor/doctor.go -- outputText -->

### Checks

| Code | What it checks |
|------|----------------|
| `doctor-store-integrity` | `database/` store corruption |
| `doctor-config-missing` | Config file resolution |
| `doctor-config-parse` | The config file parses, and then passes the same load validation the daemon applies at startup. A syntax error or a load refusal is reported under this code, and no `post-config` check runs after it <!-- source: internal/component/doctor/doctor.go -- runChecks --> |
| `doctor-config-reference` | Dangling policy/filter references, reported before load validation can refuse the configuration <!-- source: internal/component/bgp/config/filter_reference_doctor.go -- filterReferenceDoctorCheck --> |
| `doctor-tls-*` | TLS cert existence, expiry, validity |
| `doctor-plugin-missing` | Plugin binary on PATH |
| `doctor-plugin-shell-missing` | The shell an external plugin is started with. Ze runs the `run` string as `/bin/sh -c <run>`, so a host with no shell, such as a gokrazy appliance image, starts none of them <!-- source: internal/component/plugin/doctor/check_shell.go -- diagnosePluginShell --> |
| `doctor-plugin-setup-failed` | A plugin's setup step recorded a failure: an error when the failure stops ze starting, a warning when the plugin runs degraded. The fix is the remedy the failure names, else `ze show plugin list` <!-- source: internal/component/plugin/doctor/check_setup.go -- checkPluginSetup --> |
| `doctor-plugin-fatal-stop` | The last stop of ze was a plugin whose failure policy is `fatal`, or a boot ze refused: the plugin, what happened and when, read from the record the stop left <!-- source: internal/component/plugin/doctor/check_lifecycle_local.go -- checkPluginFatalStop --> |
| `doctor-plugin-stop-cleanup-late` | A warning: the last stop went on before a plugin finished its cleanup, so state that plugin installs may remain. The fix searches the log up to that stop <!-- source: internal/component/plugin/doctor/check_lifecycle_local.go -- cleanupLateRow --> |
| `doctor-plugin-autoload-unresolved` | A plugin the config loads needs a plugin this build does not carry, so ze would refuse to start; the row carries the refusal ze would give <!-- source: internal/component/plugin/doctor/check_lifecycle_local.go -- checkPluginAutoload --> |
| `doctor-plugin-no-dataplane-writer` | The config loads a plugin whose routes need a data-plane writer (static, bgp-epe) and no plugin programs the data plane the config selects, or this build was compiled without it, so ze would refuse to start or reload; an explicit `fib { ... }` block is never reported <!-- source: internal/component/plugin/doctor/check_dataplane_writer.go -- dataPlaneWriterRows --> |
| `doctor-plugin-removal-unrecovered` | A plugin a refused reload removed could not be restored and stays removed; `ze request reload` retries it <!-- source: internal/component/plugin/doctor/check_reload_records.go -- removalUnrecoveredRows --> |
| `doctor-plugin-capabilities-not-restored` | A refused reload could not restore the plugins' capability declarations, so a BGP OPEN may carry what it declared; kept until a reload commits <!-- source: internal/component/plugin/doctor/check_reload_records.go -- capabilitiesNotRestoredRows --> |
| `doctor-audit-record-failed` | The audit log's last write failed (file, error, time, count); the records stay in memory until a write lands <!-- source: internal/component/doctor/checks_audit.go -- auditRecordFailedRow --> |
| `doctor-plugin-stopped` | Inside the daemon only (`show doctor`): each configured plugin that is not running, with why (its failure policy after an exit, the respawn limit, a failed restart, a failed startup handshake), the exit status and since when. An error, fixed by `ze request plugin restart <name>` <!-- source: internal/component/plugin/doctor/checks_runtime.go -- checkPluginStopped, pluginStoppedRow --> |
| `doctor-plugin-command-not-registered` | Inside the daemon only: a command or deprecated alias a running plugin declared was refused because another plugin, or a builtin, owns the name. A warning naming both, fixed by removing or renaming one of them <!-- source: internal/component/plugin/doctor/checks_delivery.go -- commandRefusalRows --> |
| `doctor-plugin-subscription-dropped` | Inside the daemon only: a plugin's event subscriptions were dropped (unknown namespace, no default namespace, ready parameters that did not parse), so it receives none of those events. An error <!-- source: internal/component/plugin/doctor/checks_delivery.go -- subscriptionDropRows --> |
| `doctor-plugin-delivery-disagreement` | Inside the daemon only: a peer's `attach process` block and the events a plugin declared do not meet, so the event is never delivered. A warning per finding, fixed by the `attach process` change it names <!-- source: internal/component/plugin/doctor/checks_delivery.go -- deliveryDisagreementRows --> |
| `doctor-plugin-claim-unbacked` | Inside the daemon only: an exclusive role is claimed and no claimant runs, so nobody performs it. An error naming the role and the claimants <!-- source: internal/component/plugin/doctor/checks_delivery.go -- unbackedClaimRows --> |
| `doctor-plugin-poststartup-undelivered` | Inside the daemon only: a running plugin's post-startup callback failed, so the work it starts after startup never ran. An error, fixed by `ze request plugin restart <name>` <!-- source: internal/component/plugin/doctor/checks_delivery.go -- postStartupRows --> |
| `doctor-plugin-panic` | Inside the daemon only: a plugin failed to apply an event, or an engine event handler panicked and was recovered. An error naming the plugin, or the handler's namespace and event type, and the first error <!-- source: internal/component/plugin/doctor/checks_delivery.go -- panicRows --> |
| `doctor-plugin-delivery-failed` | Inside the daemon only: in the last five minutes, event batches failed with no caller waiting, or took over 500 ms. A warning naming the counts, the slowest batch and the last error <!-- source: internal/component/plugin/doctor/checks_delivery.go -- deliveryFailedRows --> |
| `doctor-plugin-enricher-unavailable` | Inside the daemon only: a plugin's `show` enricher failed to register or its last call failed, so the command answers without its field. A warning naming plugin, command and key, and one counting the failing enrichers past the 16 listed <!-- source: internal/component/plugin/doctor/checks_delivery.go -- enricherRows --> |
| `doctor-plugin-config-unapplied` | Inside the daemon only: a running plugin was not told its last transaction committed, so it keeps that transaction's undo set. A warning naming the transaction and the daemon's last one, fixed by `ze request plugin restart <name>`: a reload of an unchanged config tells no plugin anything <!-- source: internal/component/plugin/doctor/checks_delivery.go -- configUnappliedRows --> |
| `doctor-hub-managed-collision` | A `plugin/hub/server` block declares managed clients on the address the plugin acceptor binds, so its managed listener cannot bind and its clients are refused <!-- source: internal/component/plugin/doctor/check_managed_listener.go -- diagnoseManagedListener --> |
| `doctor-service-executable` | Installed systemd unit `ExecStart` points to an executable ze binary, and every `ExecReload` program exists and is executable |
| `doctor-service-user` / `doctor-service-group` | User/group referenced by installed `ze.service` exists |
| `doctor-service-stop` | The stop settings systemd applies to the installed `ze.service` (`systemctl show -p TimeoutStopUSec -p KillMode`, drop-ins included) give ze its whole stop: a timeout of at least 90 s (60 s drain ceiling plus 30 s teardown) and `KillMode=mixed`. A host with no `systemctl` is told it does not run systemd and that the unit file is stale; any other failed query carries systemctl's own error <!-- source: internal/plugins/systemd/doctor.go -- checkServiceStop, stopQueryFailure --> |
| `doctor-ssh-hostkey-missing` | SSH host key |
| `doctor-listen-unavailable` | Port binding |
| `doctor-listener-set-unknown` | The listen addresses to check cannot be derived: the YANG schema built into ze failed to load, so no web, SSH, gNMI or API listener was checked. An error, because it means an incomplete ze binary <!-- source: internal/component/doctor/checks_listener.go -- collectSchemaListeners, listenerSetUnknown --> |
| `doctor-iface-missing` / `doctor-iface-down` | Interface existence and state |
| `doctor-iface-selector-unmatched` / `doctor-iface-selector-ambiguous` | A `mac { match }` selector names no device, or several |
| `doctor-iface-mac-override-by-name` | An ethernet overrides its MAC but binds by name, so the address lands on whichever device holds that name |
| `doctor-disk-space` | Partition free space (<5%) |
| `doctor-dns-resolver` | Name server reachability |
| `doctor-*-unreachable` | External-service reachability probes that warn (never fail startup) when a configured peer is down: RADIUS admin (`doctor-radius-admin-unreachable`), RPKI cache (`doctor-rpki-unreachable`), BMP collector (`doctor-bmp-unreachable`), NTP (`doctor-ntp-server-unreachable`), management hub (`doctor-hub-unreachable`) <!-- source: internal/component/managed/doctor.go -- checkHubReachable / doctor-hub-unreachable --> |
| `doctor-clock-skew` | System clock vs NTP (>5 min) |
| `doctor-gtsm-kernel-state` | A BGP peer or dynamic group range with `connection ttl` (RFC 5082 GTSM) whose kernel state is short: inside the daemon (`show doctor`), the host route carrying hop limit 255, or a `ze_gtsm` term the daemon published (the listen-port drop or a related-ICMP drop), is missing from the kernel. The report names each missing term and the fix, `ze signal restart`, because a reload leaves an unchanged peer set alone. In `ze doctor`, before a start, the kernel resolves no route to the peer, so that route cannot be installed at apply <!-- source: internal/component/gtsm/doctor.go -- checkKernelState, installedStateDiagnostics, missingStateDiagnostic --> |
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
| `firewall` | Stale ze_* tables, chain drift vs config, a ze_* table the kernel lost, and an audit that could not read the kernel |
| `iface` | RX/TX error counter increases |
| `plugins` | Plugin crashes and disabled-by-respawn-limit |
| `vpp` | Present only while `interface { backend vpp; }` is the loaded backend. Down, with the socket path and `ze doctor` as the next step, when the vpp component is not running, the configured `vpp { api-socket }` does not exist, or it refuses connections <!-- source: internal/plugins/iface/vpp/health.go -- checkVPPHealth, vppHealthApplies --> |
| `ipsec` | IPsec SA state |
| `pki` | Certificate expiry |
| `l2tp` | L2TP subsystem running, and every listener programming sessions into the kernel data plane. Degraded, with the reason and `ze doctor` to check the kernel modules, when the l2tp generic netlink family could not be resolved, `ze.l2tp.disable-kernel-dataplane` is set, or the platform is not Linux <!-- source: internal/component/l2tp/health.go -- checkHealth, dataPlaneHealth --> |
| `flow-export` | Exporter state and collector send errors |
| `report-bus` | Report bus registration |

<!-- source: internal/plugins/flowexport/health.go -- RegisterHealthCheck, checkFlowExportHealth; internal/core/report/register.go -- init -->
Each check completes within 1 second. Components report `healthy`, `degraded`,
or `down`. The overall status is the worst of all components. A component that
has nothing to measure on this host, such as `vpp` when another interface
backend is loaded, has no row at all rather than a `healthy` row nobody
measured.
<!-- source: internal/core/health/registry.go -- RegisterWhen, invokeCheck -->

<!-- The health-reports demo is not embedded while its recording is wrong. The
     tape drives an SSH session whose commands the CLI answers with a completion
     listing rather than a result, so the recording shows the intro card, a
     config box and the recap, and none of the health, warning or error output
     the page describes. The transcript gate refuses it, which is the first
     check that has ever compared this demo against what it claims: a .webm
     cannot report that it recorded a rejection. Restore this marker when the
     recording shows the session again.
     Recorded in plan/journal/green-that-could-not-have-been-red.md -->
<!-- terminal-demo-disabled: health-reports -->

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
| `firewall-drift` | firewall | warning | Chain count mismatch vs config, or a ze_* table the kernel no longer has |
| `doctor-plugin-crashed` | plugin | error | Plugin process exited unexpectedly; carries the exit status and the failure policy ze applies, and clears every warning the plugin raised itself |
| `doctor-plugin-stopped` | plugin | warning | A configured plugin is down on any path (failure policy, respawn limit, failed restart, failed startup handshake); the fix is `ze request plugin restart <name>`, and the `plugins` health row reads down. Cleared when the plugin runs again or leaves the configuration |
| `doctor-plugin-stderr-relay-stopped` | plugin | error | The relay copying a plugin's stderr into ze's log stopped while the plugin runs (a read error, a line over the scanner's limit); the fix is `ze request plugin restart <name>` |
| `doctor-plugin-restart-overlap` | plugin | error | A plugin was restarted before ze finished stopping its previous process (2s after the stop for a forked plugin, 1s for one in the daemon). The message says what was left: an engine that had not returned or a process that had not exited, which may hold the same resources as the replacement, or only the event delivery or stderr relay of one that had ended. ze takes no further step to end it, and a second restart does not help, because it replaces the new process. The fix shows what is left: `ze show system goroutines full` for a plugin in the daemon, `ze show log recent component plugin.relay` for a forked plugin |
| `iface-errors` | iface | warning | RX/TX error counters increasing |

The bus holds at most `ze.report.warnings.max` active warnings (default 1024)
and `ze.report.errors.max` error events (default 256), both clamped to 10000.
Past the warning cap the oldest warning is dropped, and past the error cap the
oldest event is overwritten. `ze doctor` then reports `doctor-report-bus-full`
with the cap, how many entries were lost and the export that raises the cap. A
health row whose code may have been dropped reads degraded "report bus full"
rather than healthy, until the owner clears or raises that warning again.

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
p.Run(ctx, &sdk.Registration{
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
                Fix:      "show my-cache status",
            }}, nil
        }
    }
    return nil, nil
})
```

The engine invokes plugin checks only at runtime (`show doctor`), not
during offline `ze doctor`. Set `Fix` to the command that clears the finding,
alone, with no `fix:` label. A check that reads live state the daemon holds
(not the config file) declares `Phase: rpc.DoctorPhaseRuntime`; the same phase
is open to an in-tree check registered with `diagnostic.RegisterDoctorCheck`,
and only `show doctor` runs it. A plugin's runtime check is declared here and
answered by `OnDoctorCheck`, never as a `registry.DoctorCheckDef`: the plugin
registry refuses that form in the `runtime` phase, because its function would
run in the daemon's process rather than the plugin's. Each check has one
declaration: Stage 1 refuses a name the plugin's registry row already declares.
See
`docs/architecture/api/process-protocol.md` for the full protocol.

### Requirements

Every new doctor check must:

1. Use a diagnostic code with the `doctor-` prefix for a code the check
   introduces. A check that bridges another surface's validator declares the
   codes that validator emits: the semantic-validation check in
   `internal/component/config` declares `config-mcp-invalid` and its siblings,
   because a `doctor-` alias for them would be a second code for one fact. The
   registry accepts any well-formed lower-kebab code
   (`diagnostic.RegisterDoctorCheck`).
2. Register the code in `internal/core/diagnostic/codes.go` so
   `ze show diagnostic <code>` works. The entry names the code through an exported
   constant, and the check that emits it references that constant. A code
   spelled a second time in the emitting package is a second declaration of
   one fact, and `./le arch enumeration check` refuses it.
3. Include a unit test proving the check fires when the relevant config
   is present and emits the registered code.

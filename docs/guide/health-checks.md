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
| `doctor-plugin-schema-refused`, `doctor-plugin-schema-undeclared` | The last start of ze was refused for an external plugin's YANG: a module it declared that ze refused (config, command or rpc), or a configuration section of a plugin that gave ze no schema. The row names the time, the plugin and the cause, read from the same record, and its fix is the one the refusal named <!-- source: internal/component/plugin/doctor/check_lifecycle_local.go -- schemaRefusedRow --> |
| `doctor-build-registration-unheld` | The last start of ze was refused because this ze was built wrong: a YANG module compiled into it names a validator or a reference source no package in it registers. The row names the time, the module and the name, read from the same record; the config is not at fault <!-- source: internal/component/plugin/doctor/check_lifecycle_local.go -- buildRegistrationUnheldRow --> |
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
| `doctor-service-restart` | The effective systemd policy preserves clean stop, intentional restart and failure recovery: `Restart=on-failure`, `SuccessExitStatus=75`, `RestartForceExitStatus=75` and no restart-prevention override. Missing or unreadable policy is a warning with the query or reinstall command that resolves it <!-- source: internal/plugins/systemd/doctor.go -- checkServicePolicy, checkServiceRestartOutput --> |
| `doctor-ssh-hostkey-missing` | SSH host key |
| `doctor-listen-unavailable` | Port binding |
| `doctor-listener-set-unknown` | The listen addresses to check cannot be derived: the YANG schema built into ze failed to load, so no web, SSH, gNMI or API listener was checked. An error, because it means an incomplete ze binary <!-- source: internal/component/doctor/checks_listener.go -- collectSchemaListeners, listenerSetUnknown --> |
| `doctor-iface-missing` / `doctor-iface-down` | Interface existence and state |
| `doctor-iface-selector-unmatched` / `doctor-iface-selector-ambiguous` | A `mac { match }` selector names no device, or several |
| `doctor-iface-mac-override-by-name` | An ethernet overrides its MAC but binds by name, so the address lands on whichever device holds that name |
| `doctor-disk-space` | Partition free space (<5%) |
| `doctor-storage-not-grown` | On an appliance, the persistent storage is smaller than its disk: the system was not installed by the installer, or the growth failed at install. A warning naming the device and both sizes, fixed by `ze request system storage grow`. Also a warning when the sizes cannot be read, which is the answer on every appliance today, because the size read is not written yet <!-- source: internal/component/doctor/checks_storage.go -- checkStorageGrown --> |
| `doctor-dns-resolver` | Name server reachability |
| `doctor-*-unreachable` | External-service reachability probes that warn (never fail startup) when a configured peer is down: RADIUS admin (`doctor-radius-admin-unreachable`), RPKI cache (`doctor-rpki-unreachable`), BMP collector (`doctor-bmp-unreachable`), NTP (`doctor-ntp-server-unreachable`), management hub (`doctor-hub-unreachable`) <!-- source: internal/component/managed/doctor.go -- checkHubReachable / doctor-hub-unreachable --> |
| `doctor-clock-skew` | System clock vs NTP (>5 min) |
| `doctor-gtsm-kernel-state` | A BGP peer or dynamic group range with `connection ttl` (RFC 5082 GTSM) whose kernel state is short: inside the daemon (`show doctor`), the host route carrying hop limit 255, or a `ze_gtsm` term the daemon published (the listen-port drop or a related-ICMP drop), is missing from the kernel. The report names each missing term and the fix, a restart of Ze worded for this process (`sudo systemctl restart ze` only when ze runs under ze.service, otherwise `ze signal stop`, then a start the way ze was started), because a reload leaves an unchanged peer set alone. In `ze doctor`, before a start, the kernel resolves no route to the peer, so that route cannot be installed at apply <!-- source: internal/component/gtsm/doctor.go -- checkKernelState, installedStateDiagnostics, missingStateDiagnostic --> |
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
| `doctor-fib-backup-not-programmed` | fib | warning | Installed routes hold a fast-reroute backup the kernel does not carry; the detail counts them. Cleared when none is left |
| `doctor-fib-flush-incomplete` | fib | error | `flush-on-stop` left Ze's routes in the kernel when fib-kernel stopped: a delete failed, the kernel still listed a flushed route, or the listing failed. The detail counts them and names the first; the fix is `ip route flush proto 250 table all; ip -6 route flush proto 250 table all`. Not in the `fib` health row |
| `doctor-fib-sweep-incomplete` | fib | error | The startup sweep found routes a previous run of Ze left in the kernel that no protocol announced again, and the kernel refused to delete one or more, or the kernel's routes could not be listed at startup, so nothing was swept. Each route left keeps forwarding towards a next hop nothing maintains. The detail counts the prefixes whose delete failed and names the first with the kernel's error, or carries the listing's error (operation `list`); the fix is `ip route show proto 250 table all; ip -6 route show proto 250 table all; ip route del <prefix> proto 250 table <table>`. Not in the `fib` health row |
| `doctor-iface-address-unattributed` | iface | error | The netlink monitor received an address for an interface it had not seen yet, and the kernel lookup of that interface failed, so Ze did not announce the address: its connected route is missing until the address changes. The subject is the address and the interface index, the detail carries the kernel's error; the fix is `ip -o addr show to <address>; ip addr del <address> dev <device>; ip addr add <address> dev <device>`. Raised once per lost address and never cleared, so not in the `iface` health row <!-- source: internal/plugins/iface/netlink/monitor_report_linux.go -- reportAddressUnattributed --> |
| `doctor-iface-state-unread` | iface | warning | An interface apply could not read a device's current state before it wrote its link settings, so it wrote the configured values without knowing whether they were already in place. The subject is the device, the message carries the backend's answer; the apply reports any write that fails itself. The fix is `ze show interface name <device> detail`. A backend that is not ready yet, or a device the read does not find by its name, raises nothing. Cleared when a later apply reads that device's state <!-- source: internal/component/iface/config_apply_held.go -- linkFacts, logUnreadState --> |
| `doctor-iface-backend-close` | iface | error | Closing the interface backend (the netlink sockets, or the connection to VPP) failed when the interface component stopped. The kernel or VPP keeps what was programmed; the message carries the backend's answer, and the fix is `ze start`, which opens a fresh backend <!-- source: internal/component/iface/register.go -- runEngine --> |
| `doctor-bfd-auth-failed` | bfd | warning | A BFD session drops its peer's Control packets because they fail authentication; the subject names the peer, VRF, hop mode and local discriminator, and the fix names the profile whose auth type, key-id and secret must match the peer's. Cleared when a packet verifies again, the session's authentication changes, the session is removed, or no packet has failed for two minutes; raised at most once a minute per session <!-- source: internal/component/bfd/engine/problems.go -- warnAuthFailedLocked, clearAuthWarning, expireQuietWarnings --> |
| `doctor-bfd-send-failed` | bfd | warning | The transport refused a BFD packet of a session: a Control packet (the peer takes the session Down when its Detection Time passes), an Echo packet (the session goes Down with Echo Function Failed) or the return of the peer's Echo packet (the peer's echo function fails). The subject names the peer, VRF, hop mode, local discriminator and packet kind; the fix is `ze show interface name <interface> detail` for a session bound to an interface, which shows the subnets the peer must be on, else `ze show bfd session address <peer>`. Each kind is raised at most once a minute per session and cleared when a packet of that kind is sent, the session is removed, or none has been refused for two minutes <!-- source: internal/component/bfd/engine/problems.go -- warnSendFailedLocked, clearSendWarning, expireQuietWarnings --> |
| `doctor-bfd-auth-seq-unsaved` | bfd | error | The state store refused to save the TX sequence number of a BFD session using meticulous keyed authentication, raised once per run of refused writes and again when the session's authentication closes with its last write refused. After a restart the session starts from an older number and the peer drops its packets until it passes the one it last accepted; the subject names the peer, VRF and hop mode, and the fix is `ze doctor`, which reports why the store cannot be written <!-- source: internal/component/bfd/engine/problems.go -- reportSeqUnsaved, closeAuth --> |
| `firewall-stale-table` | firewall | warning | ze_* table in kernel not in config |
| `firewall-drift` | firewall | warning | Chain count mismatch vs config, or a ze_* table the kernel no longer has |
| `doctor-plugin-crashed` | plugin | error | Plugin process exited unexpectedly; carries the exit status and the failure policy ze applies, and clears every warning the plugin raised itself |
| `doctor-plugin-stopped` | plugin | warning | A configured plugin is down on any path (failure policy, respawn limit, failed restart, failed startup handshake); the fix is `ze request plugin restart <name>`, and the `plugins` health row reads down. Cleared when the plugin runs again or leaves the configuration |
| `doctor-plugin-stderr-relay-stopped` | plugin | error | The relay copying a plugin's stderr into ze's log stopped while the plugin runs (a read error, a line over the scanner's limit); the fix is `ze request plugin restart <name>` |
| `doctor-plugin-restart-overlap` | plugin | error | A plugin was restarted before ze finished stopping its previous process (2s after the stop for a forked plugin, 1s for one in the daemon). The message says what was left: an engine that had not returned or a process that had not exited, which may hold the same resources as the replacement, or only the event delivery or stderr relay of one that had ended. ze takes no further step to end it, and a second restart does not help, because it replaces the new process. The fix shows what is left: `ze show system goroutines full` for a plugin in the daemon, `ze show log recent component plugin.relay` for a forked plugin |
| `doctor-plugin-stop-unfinished` | plugin | error | An external plugin ze started at boot, before it parsed the whole configuration, was stopped (it sent no declaration, the parsed configuration no longer names it, or its startup phase was refused) and had not finished stopping within its bound (2s after the stop for a forked plugin). ze went on and removed it from the plugins it runs. The message says what was left: a process that had not exited, which ze had sent the kill and which may still be running, or only the event delivery or stderr relay of one that had ended. ze takes no further step to end it. The fix shows what is left: `ze show log recent component plugin.relay` for a forked plugin, `ze show system goroutines full` for a plugin in the daemon |
| `doctor-config-reload-refused` | plugin-server | error | A configuration reload was refused: its transaction did not commit (a plugin refused the change or did not answer), the plugins the new configuration needs could not be resolved or did not start, or the new configuration declares other external plugins than the ones ze started with (one added, removed or changed), which change only when ze restarts. ze keeps the configuration it was running; an event raised while ze restored that configuration says the restore did not complete. The fix is the refusal's own command when it names one (a plugin dependency this build does not carry, `restart ze` for changed external plugins), else `ze show errors` |
| `doctor-plugin-schema-changed` | plugin-server | warning | An external plugin was started again while ze ran (it ended, or the operator ran `request plugin restart`) and declared other YANG modules (config, command or rpc modules) than the ones ze read from it at start: another module text, name or namespace, a module where it declared none, or none where it declared one. ze refused the plugin at its first startup stage and does not run it; the running configuration, the modules ze holds and every other plugin are unchanged. The message says what differs. The fix is `restart ze`, which loads the new modules; a later start of the plugin with the modules ze started with clears the warning |
| `doctor-plugin-schema-undeclared` | plugin-server | warning | ze holds no config schema of an external plugin the configuration declares: its program could not be started, it sent no declaration within its bound, its connection ended first, or its declaration carries no YANG module. A configuration section only that plugin's schema defines is then an unknown keyword and ze refuses to start with it; the refusal on the terminal and in the log carries this code beside the unknown keyword, and `ze doctor` reports the plugin from the record that refusal leaves, until ze next starts cleanly. With no such section ze starts, the plugin that sent no declaration does not run, and the warning stays until ze restarts. The fix is the conformance check of the plugin's run line, `ze show plugin conformance run "<command>"`, which shows what the declaration lacks |
| `doctor-plugin-schema-refused` | plugin-server | error | At start ze refused the YANG module an external plugin declared: it does not compile, its name or namespace is held by another module, it defines a top-level node another module defines, it names a validator ze does not hold, or it adds nothing to the configuration schema. ze does not start with a refused module, so no daemon runs to answer `ze show doctor`. The refusal, on the terminal and in the log, names the plugin, the module, the cause and the fix; ze records it before it exits, and `ze doctor` reports it from that record until ze next starts cleanly |
| `doctor-build-registration-unheld` | plugin-server | error | This ze was built wrong: a YANG module compiled into it names a `ze:validate` function or a `ze:reference` source that no package in it registers, so that leaf would be checked by nothing or refuse every value. ze refuses to start before it reads the configuration, which is not at fault. The refusal names the module, the extension, the name and the schema path; ze records it before it exits, and `ze doctor` reports it from that record until ze next starts cleanly. The fix is a ze built with the package that registers the name |
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
2. Register the code's metadata (`diagnostic.CodeMeta`) via
   `diagnostic.Register` from the owning package's `init()` (in
   `register_doctor_codes.go`, or from `register.go`) so `ze show diagnostic <code>`
   works. Declare the code
   once as a constant beside that table and reference that constant from the
   check and registration. Only `config-*` codes, core TLS helpers, and
   cross-cutting `doctor-*` codes shared across unrelated packages live in
   `internal/core/diagnostic/codes.go`. A code spelled a second time in the
   emitting package is a second declaration of one fact, and
   `./le arch enumeration check` refuses it.
3. Include a unit test proving the check fires when the relevant config
   is present and emits the registered code.

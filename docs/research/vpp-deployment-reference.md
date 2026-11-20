# VPP Deployment Reference

Source: 83 blog articles from ipng.ch (Pim van Pelt / IPng Networks, AS8298 / AS50869),
VyOS VPP integration, and GoVPP documentation.

Full article archive: `~/Code/site/ipng.ch/articles/` (external, not checked in)
Consolidated notes: `docs/research/vpp-deployment-notes.md`
Feasibility analysis: `docs/research/ze-vpp-analysis.md`

## Production Architecture (IPng AS8298)

IPng Networks runs VPP as the production forwarding plane on all 12 backbone routers across
Europe and US. VPP replaces Linux kernel routing, achieving 8-35x forwarding performance.

| Layer | Technology |
|-------|-----------|
| Forwarding | VPP (custom .deb packages from source) |
| Control plane | Bird2 (custom-built) |
| Config tool | vppcfg (Python, YAML-based declarative config) |
| OS | Debian 12 Bookworm |
| Monitoring | SNMP AgentX (Python) + Prometheus exporter (C) |
| Network namespace | `dataplane` (SSH, SNMP, Bird, all run inside it) |
| LCP plugin | Custom `lcpng` fork with MPLS, sFlow, sub-interface improvements |

## startup.conf Reference

### Production Values (Supermicro SYS-5018D-FN8T, Xeon D1518 4C)

| Section | Key | Value | Rationale |
|---------|-----|-------|-----------|
| unix | nodaemon | (flag) | VPP runs in foreground under process supervisor |
| unix | cli-listen | /run/vpp/cli.sock | vppctl debugging socket |
| unix | log | /var/log/vpp/vpp.log | Process log |
| unix | full-coredump | (flag) | Crash debugging |
| cpu | main-core | 0 | Core 0 for VPP main thread (HT sibling core 4 for Linux) |
| cpu | corelist-workers | 1-3 | Cores 1-3 for VPP workers. `isolcpus=1,2,3,5,6,7` keeps Linux off. |
| buffers | buffers-per-numa | 128000 | 128K buffers. Formula: expected_packets_in_flight * 2. Proven for full DFZ at 10G. |
| buffers | default data-size | 2048 | Packet data bytes, excluding buffer metadata and alignment. Ze fixes this at 2048. |
| buffers | page-size | default-hugepage | Omitted/explicit `2M` uses kernel default hugepages. Explicit `1G` uses `1G`; explicit `default` uses native base pages. <!-- source: internal/component/vpp/startupconf.go -- pageSize --> |
| [memory](https://github.com/FDio/vpp/blob/v26.06/src/vpp/vnet/main.c) | main-heap-size | 1536M (1.5G) | Enough for ~958K IPv4 + ~198K IPv6 full DFZ FIB. |
| statseg | size | 1G | Stats segment shared memory for per-interface/node counters. |
| statseg | page-size | default-hugepage | Uses the same explicit page mode as buffers, including native base pages. <!-- source: internal/component/vpp/startupconf.go -- GenerateStartupConf --> |
| dpdk | dev <pci> { name, num-rx-queues, num-tx-queues } | Per-NIC | See NIC table below. |
| plugins | plugin default { disable } | (block) | Disable all, enable specific. |
| plugins | plugin dpdk_plugin.so { enable } | (line) | Enabled by default. Explicit `vpp.plugins.dpdk false` omits it and is refused with configured DPDK interfaces. <!-- source: internal/component/vpp/managed_startup.go -- VPPSettings.managedStartupPlugins --> |
| plugins | plugin af_packet_plugin.so { enable } | (line) | Emitted only for explicit `vpp.plugins.af-packet true`; default false. <!-- source: internal/component/vpp/startupconf.go -- GenerateStartupConf --> |
| plugins | plugin linux_cp_plugin.so { enable } | (line) | LCP. Enable when control-plane connectivity needed. |
| plugins | plugin linux_nl_plugin.so { enable } | (line) | Netlink listener. Enable with LCP. |
| linux-cp | lcp-sync | (flag) | VPP state changes propagate to Linux TAP mirrors. |
| linux-cp | lcp-auto-subint | (flag) | Auto-create sub-TAPs for dot1q/QinQ sub-interfaces. |
| linux-cp | default netns | dataplane | LCP TAPs created in `dataplane` netns. Routing daemons run there. |
| linux-nl | nl-rx-buffer-size | 67108864 | 64MB requested netlink receive buffer. Kernel limits still apply. |

### Generated grammar and upstream consumers

Ze targets the [VPP v26.06 startup format](https://github.com/FDio/vpp/blob/v26.06/src/vpp/conf/startup.conf).
The consumer functions, rather than a third-party template, define the accepted
directives. These links are release-pinned; they cover every section emitted by
`GenerateStartupConf`, including optional settings.
<!-- source: internal/component/vpp/startupconf.go -- GenerateStartupConf, confBuilder.devEntry, pageSize -->

| Section | Generated directives | Upstream v26.06 consumer |
|---------|----------------------|-------------------------|
| `unix` | `nodaemon`, `cli-listen`, `log`, `full-coredump`, optional `poll-sleep-usec` | [`unix_config`](https://github.com/FDio/vpp/blob/v26.06/src/vlib/unix/main.c) |
| `api-segment` | `prefix vpp` | [`api_segment_config`](https://github.com/FDio/vpp/blob/v26.06/src/vpp/api/api.c) |
| `socksvr` | `socket-name` | [`socksvr_config`](https://github.com/FDio/vpp/blob/v26.06/src/vlibmemory/socket_api.c) |
| `cpu` | Optional `main-core`, `corelist-workers` | [`cpu_config`](https://github.com/FDio/vpp/blob/v26.06/src/vlib/threads.c) |
| `buffers` | `buffers-per-numa`, `default data-size 2048`, `page-size` | [`vlib_buffers_configure`](https://github.com/FDio/vpp/blob/v26.06/src/vlib/buffer.c) |
| `dpdk` | `dev <pci>` with optional block containing `name`, `num-rx-queues`, `num-tx-queues` | [`dpdk_config`, `dpdk_device_config`](https://github.com/FDio/vpp/blob/v26.06/src/plugins/dpdk/device/init.c) |
| `plugins` | `plugin default { disable }`; `plugin <name> { enable }` for DPDK, optional Linux CP/NL and WireGuard | [`vlib_plugin_config`, `config_one_plugin`](https://github.com/FDio/vpp/blob/v26.06/src/vlib/unix/plugin.c) |
| `linux-cp` | Optional `lcp-sync`, `lcp-auto-subint`, `default netns` | [`lcp_itf_pair_config`](https://github.com/FDio/vpp/blob/v26.06/src/plugins/linux-cp/lcp_interface.c) |
| `linux-nl` | `nl-rx-buffer-size 67108864` | [`lcp_itf_pair_config`](https://github.com/FDio/vpp/blob/v26.06/src/plugins/linux-cp/lcp_nl.c) |
| `memory` | `main-heap-size` | [`main` early configuration pass](https://github.com/FDio/vpp/blob/v26.06/src/vpp/vnet/main.c) |
| `statseg` | `size`, `page-size`, optional `socket-name` | [`statseg_config`](https://github.com/FDio/vpp/blob/v26.06/src/vlib/stats/init.c) |

Both page-size directives use
[`unformat_log2_page_size`](https://github.com/FDio/vpp/blob/v26.06/src/vppinfra/std-formats.c):
Ze's `2M` setting emits `default-hugepage`, `1G` emits `1G`, and explicit
`default` emits native base pages. The default remains `2M`; no failed allocation
changes the requested mode.
<!-- source: internal/component/vpp/startupconf.go -- pageSize -->
`default-hugepage-size` is a separate key in the `memory` section, not a
page-size value. Ze does not emit that key or set the main heap's page size.
VPP's main heap therefore retains the upstream OS-page default.

The buffer directive has a space in `default data-size`; `default-data-size`
is rejected. The Linux netlink directive requires the `nl-` prefix;
`rx-buffer-size` is rejected. Unit contracts pin the corrected sections, but
source matching does not establish native startup. A native run must still
prove the installed plugin set, memory allocation, CPU affinity, device access,
namespace access and API connection on the deployment host.
<!-- source: internal/component/vpp/startupconf_test.go -- TestStartupConfConsumerGrammar -->


### startup.conf Syntax

Forwarding features register their additional startup requirements with
`RegisterStartupFeature`. Each declaration supplies its plugin, section flags
and the config roots its predicate reads. The registration extends the VPP
plugin's `ConfigReads`; the SDK delivery list derives from the same declarations.
`ParseConfigSections` selects immutable requirements from the delivered candidate,
and `GenerateStartupConf` renders only the selected requirements. The VPP
component does not enumerate the forwarding features that consume it.
<!-- source: internal/component/vpp/startup_feature.go -- RegisterStartupFeature, ParseConfigSections, startupConfigRoots -->
<!-- source: internal/component/vpp/startupconf.go -- GenerateStartupConf -->

A feature can also declare `StartupRuntime` with its generated binary API
messages and its own diagnostic code. Registration copies each message's name,
CRC and type. The compatibility check reads the message table supplied by the
VPP instance on the configured API socket. It therefore tests the loaded
plugin's API, including patched layouts, rather than a library on disk.
<!-- source: internal/component/vpp/startup_runtime.go -- newStartupRuntime, VPPSettings.checkStartupRuntime, VPPSettings.checkStartupAPI -->

Each runtime declaration registers a doctor check with the same requirements.
An answered API mismatch is an error. A connection that cannot be queried is
a warning, because it does not prove a plugin is absent. The check opens and
closes its own API client and never starts or stops the VPP process.
<!-- source: internal/component/vpp/startup_doctor.go -- registerStartupDoctor, checkStartupDoctor, probeStartupRuntime -->

When `vpp.startup-cli-file` is configured for managed VPP, the manager first
compares the retained API Unix peer with the supervised child's atomic process descriptor, then
executes that exact file once through a synchronous CLI request on the same
connector generation. The native reply,
stream close, caller context and generation must all succeed before statistics
and readiness. Generated startup.conf omits asynchronous unix exec for this
setting. Native failure kills and reaps the original child before runOnce
returns. The independent retained pidfd observer alone proves physical death;
notification may precede reap. External supervision owns its own bootstrap.
<!-- source: internal/component/vpp/startup_cli.go -- ExecuteStartupCLI, VPPManager.activateManagedRuntime -->
<!-- source: internal/component/vpp/vpp.go -- VPPManager.runOnce -->

The manager checks selected runtime requirements after publishing statistics
and before announcing readiness. A refusal closes the statistics owner and
the candidate API connection. The plugin's startup callback waits for this
answer, so a dependent plugin cannot start against an unchecked connection.
Each ready lifetime emits one disconnected event when it ends, including a
configuration replacement. A candidate that fails before readiness emits no
disconnected event.
<!-- source: internal/component/vpp/runtime_stats.go -- VPPManager.activateRuntimeStats -->
<!-- source: internal/component/vpp/startup_readiness.go -- VPPManager.notifyStartup, VPPManager.retireReadiness -->

Configuration applies use the SDK's existing transaction. The lifecycle
cancels and joins the previous manager before starting a replacement, then
waits for the replacement's readiness. If the candidate fails after confirmed
cleanup, it reinstalls the previous settings before answering the failed apply.
Unresolved native cleanup retains the joined original manager and its exact
binder receipt instead. The failed apply leaves the transaction broken and
starts neither a candidate nor restoration. Each later configuration operation
or stop makes at most one retry against that same receipt before admitting a
replacement. Confirmed driver restoration clears that custody; an unresolved
child-cleanup result remains a separate barrier. If restoration also fails,
the transaction retains the original settings for a later retry.
Selected startup settings contain immutable requirements; registry predicates
are used during selection and do not make equal settings compare unequal.
<!-- source: internal/component/vpp/register.go -- runVPPEngine, runConfiguredVPP -->
<!-- source: internal/component/vpp/startup_lifecycle.go -- newVPPConfigTx, vppLifecycle.Install, vppLifecycle.replace -->
<!-- source: internal/component/vpp/cleanup_outcome.go -- nativeCleanupReceipt.retry -->

VPP's startup.conf uses a custom format (not INI, not YAML):

```
section-name {
  key value
  key value value
  flag-without-value
  nested-section {
    key value
  }
}
```

String values are unquoted. Multi-word keys such as `default data-size` and
`default netns` use spaces. The emitted boolean directives are bare flags
(present = true); this is not a rule for every VPP option.
Dev entries use PCI address as the key with a nested block for per-device options.

## Runtime statistics

The VPP manager connects the statistics segment after the binary API handshake
and the configured managed bootstrap's successful completion,
even when no metrics exporter is configured. It reads interface statistics
before publishing the connection and emitting `connected` or `reconnected`.
Subscriber accounting therefore has the same counter source with or without
telemetry. A failed connection or read remains explicitly unavailable; it is
never interpreted as zero traffic or idle time. The runtime doctor and health
row retain `doctor-vpp-stats`, with the underlying failure in `show warnings`.
An unavailable statistics segment does not restart an otherwise usable VPP API.
<!-- source: internal/component/vpp/runtime_stats.go -- VPPManager.activateRuntimeStats, VPPManager.connectRuntimeStats -->
<!-- source: internal/component/vpp/runtime_stats_doctor.go -- checkRuntimeStats, reportRuntimeStatsFailure -->

A session retains its binary API connector and connection generation.
`GetInterfaceStatsForGeneration` accepts only the statistics connection published
for that pair and checks the generation again after reading. A replaced VPP
process cannot answer an older session's counter request. Statistics reads are
serialized because GoVPP updates cached statistics directories. When telemetry
is configured, its one poller uses that same owner. Shutdown cancels and joins
the poller, withdraws the statistics publication and disconnects it before
closing the binary API connection. A success clears only the same live owner's
statistics category, and a confirmed new statistics connection clears its own
connection failure.
<!-- source: internal/component/vpp/runtime_stats.go -- GetInterfaceStatsForGeneration, runtimeStats.read, runtimeStats.Close -->

Per-session counters come from one shared snapshot, not one dump per session.
`InterfaceCountersForGeneration` answers one interface index from the
statistics owner's latest dump, indexed by interface, and takes a new dump
only when that one started before the caller's freshness bound. Reading every
subscriber within one interval therefore costs one dump and one lookup per
session. A fresh read (the bound set to the call time) still shares the one
dump that starts after it, with every other fresh read waiting behind it. The
snapshot belongs to the statistics owner of one API generation, and every
answer re-checks that generation, so a replaced VPP process never answers from
it. A request with no bound is refused, a failed dump discards the snapshot so
the failure reaches the next caller, and a missing or duplicated interface row
is an error.
<!-- source: internal/component/vpp/runtime_stats_counters.go -- InterfaceCountersForGeneration, runtimeStats.interfaceCounters, runtimeStats.refreshCountersLocked -->
<!-- source: internal/component/vpp/vpp.go -- VPPManager.runOnce -->

## System Prerequisites

| Requirement | Value | How to set |
|-------------|-------|-----------|
| Hugepages | 3072 x 2MB (6GB) | `echo 3072 > /sys/kernel/mm/hugepages/hugepages-2048kB/nr_hugepages` |
| IOMMU | Enabled | BIOS: enable VT-d/AMD-Vi. Kernel: `intel_iommu=on iommu=pt` |
| CPU isolation | Worker cores | `isolcpus=<worker-cores>` kernel param (prevents Linux scheduler interference) |
| Netlink buffer | 64MB | `sysctl net.core.rmem_default=67108864` |
| Min hardware | 4 cores, 8GB RAM | 1 main + N workers + hugepage memory |

### Hugepage Sizing

| Use case | Hugepage count (2MB) | Total memory |
|----------|---------------------|-------------|
| Lab/small | 1024 | 2GB |
| Production 10G | 3072 | 6GB |
| Full DFZ + large buffers | 4096 | 8GB |

Formula: `main_heap + buffers * data_size + statseg + overhead`. Round up generously.

## NIC Support and Driver Binding

### Driver Matrix

| NIC | Chip | Speed | Driver | Binding method |
|-----|------|-------|--------|---------------|
| Intel X710/XL710 | i40e | 10G/40G | vfio-pci | Standard DPDK bind |
| Intel E810 | ice | 25G/100G | vfio-pci | Standard DPDK bind |
| Intel X520/X540 | ixgbe | 10G | vfio-pci | Standard DPDK bind |
| Intel i350 | igb | 1G | vfio-pci | Standard DPDK bind |
| Intel i210 | igb | 1G | vfio-pci | Standard DPDK bind |
| Mellanox ConnectX-5+ | mlx5 | 25G/100G | RDMA (bifurcated) | NO vfio. `create interface rdma`. |
| VirtIO | virtio | Variable | VPP native | No DPDK bind needed. |

### DPDK Bind Sequence (vfio-pci)

Ze loads `vfio`, `vfio_pci` and `vfio_iommu_type1`, then handles only the
configured PCI addresses:

1. Read the current `driver` symlink. An already-vfio device remains borrowed;
   Ze does not acquire custody of its existing attachment or override.
2. Read the exact original override from `driver_override_raw` when available,
   otherwise an unambiguous legacy `driver_override`, and retain both original
   values in the binder's process-local receipt. An empty original driver is
   meaningful state.
3. Unbind the original driver when present, select `vfio-pci` in that device's
   `driver_override`, then write that exact PCI address to
   `/sys/bus/pci/drivers/vfio-pci/bind`. Verify the resulting driver.
4. On teardown, detach only the owned VFIO attachment. If an original driver
   existed, temporarily select it in `driver_override` and bind the exact
   address to it. Restore the original override after the driver is restored.
   An originally driverless device remains driverless.

Every retry reads the same device's actual driver and override. A completed
detach, driver bind or override write is not repeated. The receipt is removed
only after complete restoration; failed native calls remain wrapped causes.
A failed later input preserves both its primary error and any failed rollback.
No on-disk recovery journal or native hardware restoration proof is implied by
the portable operation-table tests.
<!-- source: internal/component/vpp/dpdk.go -- DPDKBinder, bindPCIWithOps, restoreDPDKDevice -->

Only absence of `driver_override_raw` allows legacy fallback. A failed raw read
keeps its original cause, while a legacy `(null)` is refused before device
mutation because it does not identify the original override. The reader removes
only one final newline; spaces, tabs and carriage returns remain policy bytes.
Restoration writes a nonempty value without an added newline and clears an
unset override with exactly one newline. An ambiguous legacy host needs a
kernel that provides the raw getter before Ze can bind the NIC.
<!-- source: internal/component/vpp/dpdk.go -- readDriverOverrideAt, setDriverOverrideAt, readSysfsValue -->

The configured-device doctor check uses the binder's same read-only original
state capture. It skips missing devices and borrowed VFIO attachments, reports
capture failure with its original path and cause, and changes no device state.
Portable file and operation-table controls do not establish raw-getter kernel
support or physical NIC restoration.
<!-- source: internal/component/vpp/doctor_dpdk_linux.go -- checkVPPDPDK, dpdkOriginalStateDiagnostic -->
<!-- source: internal/component/vpp/dpdk.go -- readDPDKOriginalState -->

The earlier vendor/device-wide `new_id` path could attach an unconfigured
matching driverless NIC: Linux v7.3 `do_pci_add_dynid` calls `driver_attach`,
which walks every bus device and attempts matching probes. Removing that ID
does not detach those devices. Ze therefore uses per-device override and exact
bind, with no `new_id`, `remove_id` or bus rescan. The pinned consumers are
[PCI dynamic IDs and override matching](https://github.com/torvalds/linux/blob/v7.3/drivers/pci/pci-driver.c),
[driver attachment](https://github.com/torvalds/linux/blob/v7.3/drivers/base/dd.c),
[exact-address bind and override attribute](https://github.com/torvalds/linux/blob/v7.3/drivers/base/bus.c),
and the [PCI sysfs ABI](https://github.com/torvalds/linux/blob/v7.3/Documentation/ABI/testing/sysfs-bus-pci).

The manager classifies an expected kill from the original command's actual
reaped `ProcessState`, never from an error string or cancellation alone. A
separate failed native cleanup remains a typed cause and stops child restart.
Deferred DPDK restoration errors are joined into the terminal manager result.
The independent retained-pidfd observer still supplies physical process-exit
proof; neither cancellation nor that proof alone confirms NIC restoration.
<!-- source: internal/component/vpp/cleanup_outcome.go -- nativeCleanupError, expectedVPPKill, managedChildCleanup -->
<!-- source: internal/component/vpp/vpp.go -- runWithOps, runOnce, waitVPPConnection -->

### Mellanox RDMA Exception

mlx5 NICs use bifurcated driver mode. The kernel `mlx5_core` driver stays loaded. VPP creates
the interface via `create interface rdma host-if <pci-addr>` instead of a DPDK dev entry.

This means:
- No vfio binding step
- No driver save/restore
- DPDK section in startup.conf does NOT include mlx5 devices
- Interface creation uses GoVPP `RdmaCreate` API instead of DPDK auto-discovery
- Performance is equivalent to DPDK for mlx5

This describes VPP's general Mellanox handling. Ze's own VPP integration does not yet
implement RDMA/mlx5 interface creation (there is no `RdmaCreate` call in the codebase);
this section is background for a future deployment path.

### NIC-Specific Quirks

| NIC | Issue | Workaround |
|-----|-------|-----------|
| Intel X710 (early VPP 21.06) | DPDK driver failures with vfio-pci and igb_uio | Upgrade VPP. Fixed in later releases. |
| Intel i40e (some firmware) | RSS problems with flow-director enabled | Disable flow-director in DPDK dev config. |
| Any NIC with jumbo frames | Default buffer size (2048) too small for one buffer | External startup.conf can set `default data-size 9216`; Ze's generated configuration fixes it at 2048. |

## LCP (Linux Control Plane) Plugin

### Architecture

LCP creates TAP mirrors in Linux for every VPP interface. Routing daemons (ze's BGP) run in
Linux and see TAP devices. VPP's netlink plugin syncs kernel route changes to VPP's FIB.

```
VPP interface (TenGigabitEthernet3/0/0) <---> Linux TAP (in dataplane netns)
                                                  |
                                              ze BGP reactor (TCP bind on TAP)
```

### lcpng vs Upstream LCP

| Feature | Upstream LCP (VPP 25.02+) | lcpng (IPng fork) |
|---------|--------------------------|-------------------|
| Basic TAP mirroring | Yes | Yes |
| lcp-sync (VPP to Linux) | Yes | Yes |
| lcp-auto-subint | Yes | Yes |
| MPLS interface enable | No | Yes |
| MPLS route sync | No | Yes |
| sFlow hooks | No | Yes |
| Sub-interface fixes | Partial | Full |

**Recommendation for ze:** Start with upstream LCP. Evaluate lcpng when vpp-3 (MPLS) needs
LCP MPLS support.

### TAP Interface Naming

LCP creates TAPs with VPP's interface name, truncated to Linux's 15-char limit.
`TenGigabitEthernet3/0/0` becomes a truncated name. vppcfg uses short names. Ze's
naming module (vpp-4 spec) handles bidirectional mapping.

### Network Namespace

All LCP TAPs are created in the `dataplane` netns. This isolates the forwarding plane
from the management plane. Ze's BGP reactor, SSH, web UI all run in this netns.

Ze's interface backend exposes actual pair readback through
`iface.LCPPairReader.GetLCPPair`. The caller MUST first resolve the device through
that backend's link read. Exact VPP link readback records the native name and
index without replacing the configured logical name used by interface events.
Pair lookup reads this existing native identity directly, then uses an owned
context-bound stream; it never starts the shared channel's lazy interface dump.
An unresolved name is a failed lookup, not proof that a pair is absent. The
targeted `lcp_itf_pair_get_v2` exchange must contain one details message and its
explicit final reply, with matching identity and the configured global namespace.

<!-- source: internal/component/iface/lcp_pair.go -- LCPPairReader -->
<!-- source: internal/plugins/iface/vpp/query.go -- detailsByName -->
<!-- source: internal/plugins/iface/vpp/naming.go -- learnNativeName, lookupNativeIndex -->
<!-- source: internal/plugins/iface/vpp/lcp_query.go -- GetLCPPair, readLCPPair -->

## GoVPP Connection

Resource owners capture `Connector.ConnectionGeneration` when they acquire
VPP state and use `NewStreamForGeneration` for later operations. The epoch
changes on successful connection publication and every close. Checking it and
selecting the connection share the connector mutex, so a retained session or
LCP-map lease cannot send its cleanup to a replacement connection through the
same connector object. Each returned stream remains the caller's resource
and must be closed.
<!-- source: internal/component/vpp/conn.go -- Connector.ConnectionGeneration, Connector.NewStreamForGeneration, Connector.Close -->

### Socket Transport

GoVPP connects via Unix socket at `/run/vpp/api.sock`. Pure Go, no CGo.

Ze uses synchronous `core.Connect`, with at most ten attempts separated by one
second when the manager starts a connection. Each successful connection owns
one health worker. A failed bounded control-ping probe invalidates the published
generation before notifying the manager. Closing the connector cancels and joins
that worker. GoVPP's `AsyncConnect` is deliberately avoided: its internal redial
could otherwise replace the underlying socket while Ze still held an older
generation. Existing streams must remain tied to their original Unix connection.
<!-- source: internal/component/vpp/conn.go -- Connector.Connect, Connector.closeConnection -->
<!-- source: internal/component/vpp/conn_health.go -- connectVPPOnce, Connector.connectionLost, stopConnectorHealth -->

A stream context does not interrupt GoVPP's native Unix socket write. Bounded
shutdown also requires the retained
[`govpp-socket-bounded-close.patch`](../../internal/le/vendorpatch/patches/govpp-socket-bounded-close.patch).
Its separate deadline mutex prevents an ordinary writer from extending or
clearing the shutdown deadline, while the client-delete handshake remains
allowed. A failed partial write closes that connection before another request
can use it. Apply the diff to the vendored module through `./le setup install`
before claiming this native shutdown bound. The public-adapter tests in
`govpp_socket_transport_test.go` exercise a real Unix peer that withholds reads,
concurrent sends during disconnect, and the healthy client-delete request.

On API loss, the manager kills and waits for the exact VPP child it supervised
before returning or restarting. Managed startup obtains an owned process
descriptor atomically with child creation (`SysProcAttr.PidFD`) and compares its
pidfs identity with the actual API socket peer before the startup CLI barrier,
statistics and readiness. Other process attributes remain intact. A claimed
PID has no authority, including across PID namespaces.

Statistics capture the actual Unix socket that transfers the mapped segment,
disable automatic socket replacement, and retain that peer. API and statistics
must name the same live pidfs device and 64-bit inode before any initial read,
before atomic provider installation under the original connector lock, and
before and after subsequent reads. A different statistics process refuses
readiness. Unsupported or unknown physical identity refuses with a diagnostic;
neither a pathname, integer PID nor boot timestamp repairs it.

The manager disconnects statistics before closing its original API transport.
A late old child wait cannot close a newer generation. A completed wait alone
never publishes `ProcessExit`: the retained physical witness is the sole
notifier and requires kernel-confirmed whole-process exit. Notification may
precede the original child's reap; the manager still joins that child before
runOnce returns. Retained `ProcessLease` owners validate the sealed exact
physical proof before releasing obligations. `Covers` checks the logical epoch
only and cannot replace that validation.

One bounded worker covers each retained API lifetime, including after its
manager disconnects. Owners retain a lease before their first native mutation
and keep it through unresolved cleanup. Ordinary `Close` releases its mutex
before joining final descriptor cleanup; a notification uses nonjoining
`Retire`. A proof also covers other retained epochs of the same physical
process, but cannot retire a live foreign peer. Observation failure remains
unknown. At most 1,024 physical API lifetimes and 131,072 references per lifetime
are retained; no reconnect history accumulates. `ze doctor` probes the actual
socketpair/pidfs capability, and `doctor-vpp-process` retains observation and
descriptor-close failures. The server must create its own listeners: inheriting
another process's listener does not change its original kernel peer owner.
<!-- source: internal/component/vpp/process_peer.go -- ProcessLease, Connector.checkStatsProcessLocked -->
<!-- source: internal/component/vpp/process_peer_linux.go -- captureProcessWitness -->
<!-- source: internal/component/vpp/process_managed_linux.go -- startManagedProcess -->
<!-- source: internal/component/vpp/vpp.go -- waitVPPConnection, VPPManager.runOnce -->
<!-- source: internal/component/vpp/process_exit.go -- ProcessExit, Connector.bindManagedProcess, Connector.retireManagedConnection -->

External mode also observes API loss and retries its connection through the
manager's backoff loop. Its supervisor is external: Ze neither kills that
process nor treats an API failure as proof of process death. Owners must retain
unresolved native state until its own ownership or absence is confirmed.

### API Clients

GoVPP provides typed RPC service clients generated from VPP's `.api.json` files:

| Client | Purpose | Key APIs |
|--------|---------|----------|
| ip.RPCService | IPv4/IPv6 routes | IPRouteAddDel, IPRouteDump |
| mpls.RPCService | MPLS labels | MplsRouteAddDel, MplsInterfaceEnableDisable |
| interfaces.RPCService | Interface mgmt | SwInterfaceSetFlags, SwInterfaceAddDelAddress, SwInterfaceDump |
| vxlan.RPCService | VXLAN tunnels | VxlanAddDelTunnelV3 |
| lcp.RPCService | LCP pairs | LcpItfPairAddDelV3, LcpItfPairGet |
| acl.RPCService | ACLs | AclAddReplace, AclInterfaceSetAclList |
| policer.RPCService | Policers | PolicerAddDel |
| sflow.RPCService | sFlow | SflowEnableDisable, SflowSamplingRateSet |

### Stats Client (separate from binary API)

Stats use shared memory, not the binary API socket. Separate connection:

| Stat type | Method | Data |
|-----------|--------|------|
| InterfaceStats | GetInterfaceStats | rx/tx packets/bytes, drops, errors per interface |
| NodeStats | GetNodeStats | clocks/packet, vectors/call per graph node |
| SystemStats | GetSystemStats | vector rate, input rate |

Stats socket default: `/run/vpp/stats.sock`

## Performance Reference

| Metric | Value | Hardware | Source |
|--------|-------|----------|--------|
| IPv4/IPv6 forwarding | ~35 Mpps total | Xeon D1518 4C, 3 workers, 6x10G | IPng production |
| MPLS forwarding | 18-20 Mpps/thread | Same | IPng MPLS series |
| L2 cross-connect | >14.88 Mpps/thread | Same | IPng VLL benchmark |
| VXLAN (IPv4 underlay) | ~14 Mpps/thread | Same | IPng VLL benchmark |
| Clocks/packet (1 Mpps) | ~660 | Same | IPng monitoring |
| Route loading (netlink) | ~175K routes/sec | Same | IPng LCP Part 5 |
| Route loading (binary API) | ~250K calls/sec | GoVPP estimated | Benchmark needed |
| sFlow overhead | 11 CPU cycles/packet | VPP 25.02 | IPng sFlow Part 2 |
| Power consumption | ~45W fully loaded | Supermicro SYS-5018D-FN8T | IPng hardware review |

### Efficiency Under Load

VPP gets more efficient as load increases due to instruction/data cache reuse in the
vector processing model. `vectors_per_call` metric shows this: 1.00 = idle (one packet
at a time), 256 = saturated (processing full vectors). Production typically 4-16.

## Hardware Reference

| Device | CPU | NICs | VPP Perf (64B) | Power | Price |
|--------|-----|------|----------------|-------|-------|
| Supermicro 5018D-FN8T | Xeon D1518 4C 2.2GHz | 2x10G + 6x1G + add-in | ~35 Mpps | ~45W | ~$800 |
| Netgate 6100 | Atom C-3558 4C 2.2GHz | 2x10G + 4x2.5G + 2x1G | 5 Mpps (1 worker) | 19W | $699 |
| GoWin R86S | Pentium N6005 4C 2GHz | OCP + 3x2.5G | 13.4 Mpps | 17-20W | ~$314 |
| Gowin 1U N305 | i3-N305 8C 3GHz | OCP 2x25G + 2x2.5G + 3x1G | 18.34 Mpps | 47.5W | - |

## What VPP Does NOT Provide

- No routing protocols (BGP, OSPF, IS-IS). Ze's job.
- No firewall (nftables/iptables bypassed). VPP ACLs are basic match/action.
- No hierarchical QoS. VPP policers are flat token buckets, not HTB/HFSC.
- No kernel features on fast path (tc, XDP, conntrack, eBPF).
- DPDK/RDMA NIC drivers only (not all hardware supported).
- Minimum 4 physical CPU cores, 8 GB RAM.

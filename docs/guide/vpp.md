# VPP Data Plane

**Status:** the VPP component manages the VPP process lifecycle (startup,
crash recovery, DPDK NIC binding, GoVPP connection), the `fib-vpp` plugin
programs routes from ze's system RIB directly into VPP's FIB via the
GoVPP binary API, and the stats segment is polled for per-interface,
per-node and system-wide Prometheus metrics.
<!-- source: internal/component/vpp/vpp.go -- VPPManager lifecycle -->
<!-- source: internal/plugins/fib/vpp/fibvpp.go -- processEvent installs, updates, withdraws -->
<!-- source: internal/component/vpp/telemetry.go -- stats poller -->

The FIB plugin also programs MPLS label stacks on IP routes. The VPP-native
interface backend is registered as `vpp`. The
[SRv6 path](../architecture/fib/fib-depth-4-srv6.md#vpp-backend) creates a
single-Service-SID encapsulation policy before installing prefix steering.
<!-- source: internal/plugins/fib/vpp/register.go -- runFibVPPPlugin backend construction -->
<!-- source: internal/plugins/fib/vpp/fibvpp.go -- processEvent, processMPLSChange -->
<!-- source: internal/plugins/fib/vpp/mpls.go -- addMPLSRoute, delMPLSRoute -->
<!-- source: internal/plugins/iface/vpp/register.go -- iface.RegisterBackend -->
<!-- source: internal/plugins/fib/vpp/srv6.go -- acquirePolicy, addSRv6Steer -->

[Features by data plane](dataplane-features.md) lists each forwarding feature
and says whether it works on the Linux kernel, on VPP, or on both.

## Why VPP is in Ze

Ze is a BGP daemon. BGP produces forwarding decisions; something else has
to carry the packets. The default answer on Linux is the kernel route
table, programmed via netlink by the `fib-kernel` plugin. That works,
but it puts every packet through the kernel's software forwarding path,
which tops out at a few Mpps on commodity hardware and starts dropping
under bursty load.

VPP (the FD.io Vector Packet Processor) is a user-space software router
built on DPDK. It takes the NICs away from the kernel, batches packets
into vectors, and walks each vector through a graph of forwarding nodes
with hot caches. On the same hardware where the kernel loses packets,
VPP forwards at line rate: roughly 35 Mpps on a 4-core Xeon D-1518, with
numbers like 18 Mpps per thread for MPLS and around 14 Mpps per thread
for VXLAN. IPng Networks has run this stack in production on AS8298 for
several years.
<!-- source: docs/research/vpp-deployment-reference.md -- IPng production performance table -->

Adding VPP to ze is not about replacing BGP. It gives ze a
forwarding-plane answer for IXP route servers, production edge routers,
and gokrazy appliances on small hardware such as an N100 mini-PC. The
control plane stays in ze (BGP, RIB, config, CLI, web UI).
The forwarding plane becomes VPP. The two talk through a small, typed
interface (GoVPP over a Unix socket).
<!-- source: internal/component/vpp/conn.go -- Connect -->

## How the two halves fit together

```
┌──────────────────────────────────────────────────────────┐
│                          ze                              │
│                                                          │
│   BGP reactor ──▶ protocol RIB ──▶ sysRIB                │
│                                     │                    │
│                              ┌──────┴──────┐             │
│                              │             │             │
│                         fib-kernel     fib-vpp           │
│                         (netlink)      (GoVPP)           │
│                              │             │             │
│                              ▼             ▼             │
│                       Linux FIB       VPP FIB            │
│                        (local)        (transit)          │
│                                                          │
│   vpp component: starts VPP, binds NICs, watches health  │
└──────────────────────────────────────────────────────────┘
```

Ze has two FIB writers, and each one runs only when it is loaded. With
no `fib` block, Ze loads the one writer for the data plane that
`interface backend` selects: `fib-vpp` on a VPP deployment, and
`fib-kernel` is then not loaded. To run both, write both `fib kernel`
and `fib vpp` blocks. A `fib kernel` block with no `fib vpp` block is
refused under `interface backend vpp`, with `set fib vpp` as the fix,
because VPP's FIB would otherwise receive no routes. `fib-kernel` then
programs the Linux route table for local services (SSH, the web UI, BGP
TCP sessions), and `fib-vpp` pushes the same best routes into VPP's FIB
via the GoVPP binary API, so transit packets are forwarded by VPP at
DPDK speed. Each loaded writer subscribes to the
`(system-rib, best-change)` event on the EventBus and reacts
independently. A `sysctl` block is accepted under `interface backend
vpp` as well and its keys are applied to the Linux kernel whatever the
data plane, so a setting about kernel behavior works the same on a VPP
box, while a key about packet forwarding, such as `rp_filter`, acts
only on the packets that reach Linux, not on the packets VPP forwards.
<!-- source: internal/component/plugin/server/startup_autoload.go -- appendDataPlaneWriter, appendDataPlaneWriterWithReason -->
<!-- source: internal/component/plugin/registry/dataplane_writer.go -- JudgeDataPlaneWriter -->
<!-- source: internal/plugins/fib/vpp/register.go -- Dependencies: ["rib", "vpp"] -->
<!-- source: internal/plugins/fib/vpp/fibvpp.go -- Subscribe to BestChangeBatch -->
<!-- source: internal/plugins/fib/kernel/fibkernel.go -- parallel backend for the kernel -->

BGP sessions themselves still use Linux sockets. VPP's Linux Control
Plane (LCP) plugin mirrors every VPP interface as a TAP device, so ze's
BGP reactor `bind()`s and `connect()`s exactly as it does on a kernel
NIC. VPP is invisible to the BGP code path.

*Which* namespace those TAPs land in is a separate question, and it is
the one live limitation here. LCP creates them in the namespace named by
`vpp.lcp.netns`, which defaults to `dataplane`, following the IPng
production convention that isolates the forwarding plane from the
management plane. Ze's BGP reactor has **no network-namespace
awareness**: it binds in whatever namespace the ze process itself runs
in. So on the shipped default, ze's BGP cannot bind on an LCP-shadowed
interface unless ze is itself running in the `dataplane` namespace.

Setting `vpp.lcp.netns` to `host` or `root` does **not** work around
this. To VPP those are ordinary namespace *names*: it resolves the value
to the path `/var/run/netns/<name>`, and ze passes the same leaf to VPP
as its global `default netns`, which VPP falls back to for any pair that
carries no namespace of its own. So `host` asks VPP for a namespace
literally called `host`, which normally does not exist.

Two remedies work today. Leave `vpp.lcp.netns` empty: that clears VPP's
global `default netns`, and LCP then creates the TAPs in VPP's own
namespace, where ze runs. Or run ze's BGP in the namespace the leaf
names. Nothing else does, until BGP learns to bind inside a named
namespace (specced, not implemented).

`ze doctor` reports `doctor-vpp-lcp-netns` for any non-empty
`vpp.lcp.netns`, and a BGP stanza is not required to see it: `host` and
`root` get one warning of their own, because the value is wrong even
where nothing binds. On Linux the check also opens the namespace
directory, and reports a namespace that is absent (LCP pair creation
would fail at apply) separately from a probe that could not answer. Run
`ze show diagnostic doctor-vpp-lcp-netns` for the detail.

The accepted character set for the leaf is what VPP can carry through: printable,
no whitespace, no path separator, under 256 characters. VPP's own parser reads the
leaf with `unformat(line_input, "netns %s", &ns)`, and `%s` ends at a space, tab,
newline, or carriage return rather than refusing the input. So `netns my ns` would
reach VPP as `my`, VPP would build `/var/run/netns/my`, that namespace would not
exist, and LCP pair creation would fail at apply on a config ze had accepted.
Config verify refuses the value instead.
<!-- source: internal/component/vpp/config.go -- validateNetns -->

A second check covers the plugin rather than the namespace. Enabling `vpp.lcp`
makes ze write a `startup.conf` that loads `linux_cp_plugin.so`. A VPP built
without it accepts the config and then fails the WHOLE apply at the binary-API
layer, with a raw VPP error that names the failing message rather than the missing
plugin. `ze doctor` reports `doctor-vpp-lcp-plugin` first. The probe reads the
RUNNING VPP with `vppctl show plugins`, not the filesystem: what matters is what
the loaded VPP has, not whether a copy exists on disk. An absent `enabled` leaf
reads as on, matching the YANG default.

Severity follows what the probe proved. A probe that answers and does not list
the plugin is an ERROR, and `ze doctor` reports the host as not ready. A probe
that cannot answer degrades to a WARNING and never claims the plugin is missing.
`vppctl` exits non-zero identically for an absent binary, an absent socket, and a
wedged VPP. A zero exit counts as an answer only when the output carries the
`Plugin path is:` line VPP prints ahead of the plugin rows: empty or truncated
output lists no plugin at all, so it is a WARNING too. None of those is evidence
about which plugins VPP loaded.
<!-- source: internal/plugins/iface/vpp/doctor.go -- checkVPPLCPPlugin, lcpEnabled, lcpPluginSO, vppctlPluginsHeader -->
<!-- source: internal/plugins/iface/vpp/register_doctor_codes.go -- doctor-vpp-lcp-plugin -->

<!-- source: internal/component/vpp/yang/ze-vpp-conf.yang -- lcp container netns default "dataplane" -->
<!-- source: internal/core/network/network.go -- RealListenerFactory.Listen binds via net.ListenConfig, no netns awareness -->
<!-- source: internal/component/vpp/startupconf.go -- linux-cp section: b.kv("default netns", s.LCP.Netns) -->
<!-- source: internal/plugins/iface/vpp/doctor.go -- checkVPPLCPNetns, lcpNetnsIsRootMarker ("", host, root) -->

## What ze does for you

When `vpp.enabled` is true, ze takes ownership of the whole VPP
lifecycle. You do not run `systemd start vpp`; you do not edit
`/etc/vpp/startup.conf`; you do not `dpdk-devbind` your NICs by hand.
Ze does the following, in order, every time it starts or VPP crashes:
<!-- source: internal/component/vpp/vpp.go -- Run sequence and runOnce -->

| Step | What ze does | Code |
|------|--------------|------|
| 1 | Parses the `vpp { ... }` YANG section into `VPPSettings` | `internal/component/vpp/config.go` |
| 2 | Validates PCI addresses, socket paths, netns names, size strings | `config.go: Validate` |
| 3 | Renders `startup.conf` (unix, api-segment, socksvr, cpu, buffers, dpdk, plugins, linux-cp, linux-nl, memory, statseg sections, some conditional) | `startupconf.go: GenerateStartupConf` |
| 4 | Loads `vfio`, `vfio_pci`, `vfio_iommu_type1` kernel modules | `dpdk.go: loadVFIOModules` |
| 5 | For each configured PCI address: saves its original driver and driver override, selects vfio-pci for that device and binds that exact address. A device already using vfio-pci keeps its existing ownership. | `dpdk.go: bindPCI` |
| 6 | Execs the VPP binary with `-c <generated startup.conf>` | `vpp.go: runOnce` |
| 7 | Connects one non-reconnecting GoVPP transport to the configured API socket (10 attempts, 1 s apart), retaining its actual Unix peer. Managed mode matches that peer to the child's atomic process descriptor, then waits for the configured original startup CLI file on the same generation. | `conn.go: Connect`, `process_exit.go: bindManagedProcess`, `startup_cli.go: activateManagedRuntime` |
| 8 | Opens the configured statistics mapping and checks that its actual Unix peer is the same live physical API process before reading or publishing counters. Starts optional telemetry. | `runtime_stats.go`, `process_peer.go` |
| 9 | Checks selected plugin APIs, then emits `("vpp", "connected")` and completes service readiness. | `runtime_stats.go: activateRuntimeStatsGeneration` |
| 10 | Waits for VPP to exit. On crash: emits `("vpp", "disconnected")`, backs off (1 s, 2 s, 4 s, capped at 30 s), restarts, emits `("vpp", "reconnected")`. `fib-vpp` replays the RIB on reconnect. | `vpp.go: Run loop`, `fibvpp.go: reconnect handler` |
| 11 | On shutdown: kills the exact supervised VPP child and joins its wait, then restores each owned NIC's original driver and driver override. A NIC that originally had no driver returns to that state. | `vpp.go: waitVPPConnection`, `dpdk.go: UnbindAll` |

The point is that VPP's system-level prerequisites (vfio module load,
NIC unbind, original driver and override restoration) are part of ze's job, not
the operator's. This matters on a gokrazy appliance where there is no
systemd and ze is PID 1 for the data plane.
<!-- source: internal/plugins/init/main.go -- gokrazy PID-1 appliance lifecycle -->

### Where the VPP program comes from

Today Ze starts the program at `/usr/bin/vpp` and does not install it. On a
Linux host you install VPP yourself, from your distribution or from FD.io. A
PPPoE configuration on VPP needs Ze's patched build, which
`third_party/vpp-patches/README.md` describes. The appliance image holds no VPP
program, so a managed VPP cannot start there yet.
<!-- source: internal/component/vpp/register.go -- defaultVPPBinary, runConfiguredVPP -->

The owner decided on 2026-11-28 that Ze's own VPP build is always installed
with Ze: the appliance image build places it in the image, the Ze
package installs it in a Ze folder beside `ze`, and Ze finds it by scanning
that folder
(`docs/research/vpp-deployment-reference.md`, "Design decisions"). That is not
built yet; `plan/release/delivery/spec-vpp-embedded-runtime.md` owns it.

Binding changes only the configured PCI device. Ze uses its `driver_override`
and the driver's exact-address `bind` file, without adding a vendor/device-wide
VFIO registration or rescanning the bus. Cleanup keeps the original state until
both driver and override restoration succeed. Retrying the same binder resumes
from the observed native state, so a confirmed detach or driver bind is not
repeated. These receipts belong to the running manager; they are not an on-disk
recovery journal.
<!-- source: internal/component/vpp/dpdk.go -- bindPCIWithOps, restoreDPDKDevice, setDPDKOverride -->

Ze reads `driver_override_raw` when the kernel provides it. Only its absence
permits the legacy getter; a legacy `(null)` is ambiguous and is refused before
changing the NIC. Upgrade to a kernel that provides `driver_override_raw` before
binding that NIC. Captured spaces, tabs and carriage returns are preserved;
only the final framing newline is removed, and nonempty restoration writes
gain no extra newline.
<!-- source: internal/component/vpp/dpdk.go -- readDriverOverrideAt, setDriverOverrideAt, readSysfsValue -->

The `vpp-dpdk` doctor check reads the same original-state prerequisite for each
configured, present PCI device without binding it. Its diagnostic retains the
failed native path and cause. A device already using VFIO remains borrowed and
does not require an unused original override snapshot.
<!-- source: internal/component/vpp/doctor_dpdk_linux.go -- checkVPPDPDK, dpdkOriginalStateDiagnostic -->
<!-- source: internal/component/vpp/dpdk.go -- readDPDKOriginalState -->

An ordinary requested child kill is clean only after the original command has
been reaped with its expected exit status. A separate cleanup failure remains
an error even when shutdown was requested. The manager returns that cause and
stops before starting another child if native child cleanup is unresolved;
failed NIC restoration is also returned, with its original receipt retained.
Physical process-exit proof and successful NIC restoration are separate facts.
Configuration replacement keeps that joined manager until its original cleanup
receipt confirms restoration. A failed bind rollback also retains the same
binder. A later configuration retry first retries that exact cleanup; it cannot
start a fresh manager while the original native cleanup is unresolved.
<!-- source: internal/component/vpp/vpp.go -- runWithOps, runOnce, waitVPPConnection -->
<!-- source: internal/component/vpp/cleanup_outcome.go -- expectedVPPKill, managedChildCleanup -->
<!-- source: internal/component/vpp/startup_lifecycle.go -- Install, stop -->

The generator targets upstream VPP v26.06 grammar. In particular, it writes
`buffers { default data-size 2048 ... }`, `linux-nl { nl-rx-buffer-size 67108864 }`,
and `memory { main-heap-size ... }`. The
[deployment reference](../research/vpp-deployment-reference.md#generated-grammar-and-upstream-consumers)
maps every emitted directive to its release-pinned consumer. Grammar matching
and unit tests are not proof of native startup: the installed VPP plugins,
hugepage reservation, CPU affinity and device/namespace permissions must also
support the selected configuration.
<!-- source: internal/component/vpp/startupconf.go -- GenerateStartupConf -->


Managed VPP consumes the startup file Ze generates. To select an AF_PACKET
profile, set `vpp.plugins.dpdk false` with no DPDK interfaces, explicitly enable
`vpp.plugins.af-packet`, and select `vpp.memory.hugepage-size default` when the
profile uses native base pages. uRPF remains enabled for configured IPv4 reverse
path policy; the selected image must contain it. Omitted settings retain the
existing DPDK-enabled, AF_PACKET-disabled, `2M` startup bytes. No unavailable
plugin or failed hugepage allocation selects another configuration.
<!-- source: internal/component/vpp/managed_startup.go -- startupPluginChoice, VPPSettings.managedStartupPlugins -->
<!-- source: internal/component/vpp/startupconf.go -- GenerateStartupConf, pageSize -->

An optional `vpp.startup-cli-file` names the original command file. After the
retained API socket peer matches the supervised child's atomic process descriptor, the manager sends one
synchronous native `exec <path>` request on that exact connector generation. Statistics activation
and service readiness wait for its correctly typed successful reply, stream
close, live context and unchanged generation. A native command or transport
failure follows the existing child kill, completed wait and original transport
closure path. The independent retained pidfd observer alone proves physical
death; its notification can precede the child's reap. The manager still joins
the original child before returning or restarting.
The generator omits asynchronous unix exec for this setting, which prevents a
second execution. Pure configuration verification rejects a path that is
relative or could escape its native token. Managed startup checks the
original file before configuration/binding and before each child exec: it must
be readable and regular. The check opens without waiting for a FIFO peer,
reads one byte, and closes that same file while retaining original filesystem
errors. It does not parse or rewrite the native command program. Empty files
are permitted by the file preflight; native VPP still decides whether their
command program can execute. External supervision owns its own startup and
neither inspects nor executes this file. `doctor-vpp-startup-cli-file` reports the exact path and its repair.
<!-- source: internal/component/vpp/managed_startup.go -- validateStartupCLIPath, checkManagedStartupCLI, managedStartupCLIDoctorCheck -->
<!-- source: internal/component/vpp/managed_startup_file_unix.go -- openStartupCLIFile -->
<!-- source: internal/component/vpp/startup_cli.go -- ExecuteStartupCLI, VPPManager.activateManagedRuntime -->
<!-- source: internal/component/vpp/vpp.go -- VPPManager.Run, VPPManager.runOnce -->

These controls do not create an AF_PACKET interface or its Linux Control Plane
pair. The declared managed bootstrap must provide those before PPPoE accesses
can prepare. The native acceptance scenario must observe the actual executable,
its generated `-c` path and config hash, the declared image's loaded plugins,
real API/statistics readiness and original connector/process identities. An exec
wrapper that replaces the generated configuration, an external-mode run, or a
unit rendering result cannot supply that managed-startup evidence. The complete
restart, accounting, pool and productive fresh-session checks remain required.
<!-- source: internal/component/vpp/vpp.go -- VPPManager.runOnce -->
<!-- source: internal/component/l2tp/plugins/pppoe-vpp/startup.go -- registerStartup, startupRequired -->

## Running against an externally supervised VPP

Set `vpp.external true` when something other than ze owns the VPP
process -- a systemd unit, a container sidecar, or the Python stub
the functional tests use. In this mode ze skips steps 3, 4, 5, 6 and 11
of the table above: no `startup.conf` generation, no vfio module load,
no NIC unbind, no `exec vpp`, no NIC restoration writes on shutdown. Ze still
connects via GoVPP at step 7, opens statistics and optional telemetry at
step 8, then emits lifecycle events and completes readiness at step 9.
It blocks on context cancellation instead of `cmd.Wait` at step 10.
<!-- source: internal/component/vpp/vpp.go -- External branch in Run and runOnce -->

This mode is going away. The owner decided on 2026-11-23 that Ze refuses to
run VPP unmanaged: with an external VPP the operator accepts that Ze may stop
and restart it, through its systemd unit or by starting the binary directly
(`docs/research/vpp-deployment-reference.md`, "Design decisions"). That is not
built yet; `plan/post-release/features/spec-vpp-failure-lifecycle.md` owns it.

Typical configurations:

```
vpp {
  enabled  true;
  external true;
  api-socket /run/vpp/api.sock;
}
```

The operator owns `startup.conf`, owns the systemd unit, owns the vfio
bind. Ze owns the API socket conversation and the FIB programming.

Use this for:
- **Systemd deployments:** the system VPP unit starts before ze, ze
  connects to its API socket on startup.
- **Functional tests:** `./le test functional vpp` drives the compiled Go VPP
  stub instead of a real VPP. See `docs/functional-tests.md` for the harness.
<!-- source: internal/test/cli/cmd_vpp_stub.go -- runVPPStub -->

The real-VPP deployment action, `./le test deployment vpp`, stops each Ze
daemon by signalling its exact command inside the container, not the host
`docker exec` client. Each replacement must reach its own startup marker before
the harness inspects retained VPP state. A policer or ACL left by the previous
daemon therefore cannot make a failed replacement look like a successful restart.
A stop returns only once the daemon has exited, because a stopping ze still owns
its config store while it finishes recording its boot configuration, which on a
slow disk takes seconds. It waits up to two minutes after SIGTERM, then sends
SIGKILL. A daemon that needed SIGKILL, or that is still running after it, fails
the run with its command named, and no further daemon starts, since it may still
own the store the next one would open.
<!-- source: internal/le/test/deployment/vppevidence.go -- startEvidenceDaemon, stopVPPProcess, vppDaemonStopWait -->
<!-- source: internal/le/test/deployment/deployment.go -- running.stopAwaitingExit -->

`vpp` and `vpp-iface` start their container through the lab Docker
wrapper, which labels it with the run that owns it. Ctrl-C or SIGTERM ends the
run and still removes the container. A scenario the interruption caught ends at
once with the interruption as the run's error, never a FAIL for a feature the
run did not finish probing. When the removal fails, the report's
`container-removal-error` names the reason and the verb exits 1; run
`./le test integration clean` to remove it. A run killed outright leaves a
labeled container that the next lab start or that cleanup removes.
<!-- source: internal/le/test/deployment/actions.go -- signalRooted -->
<!-- source: internal/le/test/deployment/deployment.go -- removeContainer -->
<!-- source: internal/le/test/deployment/vppiface.go -- awaitFeature -->

## Configuring VPP

PPPoE access follows `interface backend`. When a VPP PPPoE provider is installed
and access interfaces are configured, its registration selects
`pppoe_plugin.so` and the `enable-pass-nd-and-dhcpv6` startup flag automatically.
The candidate must enable both VPP and LCP; a missing provider or disabled
prerequisite is refused before apply. No separate plugin toggle is required.
<!-- source: internal/component/l2tp/pppoe/backend_gate.go -- validateBackendGate -->
<!-- source: internal/component/l2tp/plugins/pppoe-vpp/startup.go -- registerStartup, startupRequired -->

The `vpp { ... }` container lives in the main ze config. Minimal example:

```
vpp {
  enabled true;
  dpdk {
    interface 0000:03:00.0 {
      name xe0;
    }
    interface 0000:03:00.1 {
      name xe1;
    }
  }
}
```

This selects the default heap, buffer count, stats segment and LCP settings.
Starting native VPP also requires the host prerequisites below and the selected
plugins. Add cores, tune memory, or change the stats poll interval when the
defaults do not fit the workload.

### Every leaf, what it does, what it defaults to

| Path | Type | Default | What it controls |
|------|------|---------|------------------|
| `vpp.enabled` | boolean | `false` | Master switch. `false` means ze does not start VPP at all. |
| `vpp.external` | boolean | `false` | When `true`, ze connects to an existing VPP via `api-socket` but does NOT generate `startup.conf`, bind DPDK NICs, or exec the VPP binary. Use this on systemd-managed hosts, container sidecars, or the `le test vpp` stub harness. Default `false` preserves the ze-owned-lifecycle behaviour. |
| `vpp.api-socket` | string | `/run/vpp/api.sock` | GoVPP Unix socket. Ze validates it is absolute, has no `..`, and fits in 108 characters. |
| `vpp.startup-cli-file` | absolute path | unset | Optional original VPP CLI file. After the retained API Unix peer matches the supervised child's atomic process descriptor, Ze waits for native command completion on that same API generation before statistics and service readiness. Failure kills and reaps that child before runOnce returns; the independent retained pidfd observer alone proves physical death. The generator omits asynchronous unix exec for this setting. Invalid paths and missing/unreadable or nonregular files are refused before child startup; the doctor names the original path and repair. External mode neither inspects nor executes it. <!-- source: internal/component/vpp/startup_cli.go -- ExecuteStartupCLI, VPPManager.activateManagedRuntime --> <!-- source: internal/component/vpp/managed_startup.go -- checkManagedStartupCLI, managedStartupCLIDoctorCheck --> |
| `vpp.cpu.main-core` | uint8 | auto | CPU core pinned to the VPP main thread. The core is excluded from the worker set. Omit for VPP default. A core the host does not hold online is refused at commit. |
| `vpp.cpu.workers` | uint8 | auto | Number of worker threads. Ze takes the cores from the kernel's isolated set (`/sys/devices/system/cpu/isolated`), lowest first, `main-core` excluded and any CPU no longer online excluded, and writes them as `corelist-workers`. On a host that isolated nothing, the list falls back to `main-core+1 .. main-core+workers` and `ze doctor` reports `doctor-vpp-cpu-isolation`. A count the host cannot satisfy is refused at commit. <!-- source: internal/component/vpp/cpuset.go -- resolveWorkerCores --> |
| `vpp.cpu.worker-cores` | CPU list | unset | Explicit worker cores in the kernel's `isolcpus` syntax (`2-4`, `2,4,6`). Names the cores instead of deriving them, and MUST NOT be set beside `workers`. A core off the host, a core that is also `main-core`, and a core listed twice are each refused at commit. <!-- source: internal/core/cpulist/cpulist.go -- Parse --> |
| `vpp.cpu.poll-sleep` | `Nms` (0ms–100ms) | unset | Fixed sleep between VPP main-loop polls, expressed in whole milliseconds (`ms` is the only accepted unit, e.g. `10ms`), emitted as `unix { poll-sleep-usec N }` (1ms = 1000µs). Omit for lowest latency (workers busy-poll at 100% CPU); set a non-zero value on shared or dev hosts to trade latency for idle CPU. An explicit `0ms` is emitted and equals VPP's default. <!-- source: internal/component/vpp/config.go -- parsePollSleepMs --> |
| `vpp.memory.main-heap` | size string | `1G` | Emitted as `memory { main-heap-size ... }`. Ze leaves the heap page size at VPP's OS-page default. Use `1536M` for a full DFZ (approximately 958k IPv4 + 198k IPv6 routes). |
| `vpp.memory.hugepage-size` | `2M`, `1G` or `default` | `2M` | `2M` emits `default-hugepage`; `1G` emits `1G`. Explicit `default` emits native base pages for buffers and statistics, without a hugepage reservation. The kernel determines the base-page size. Hugepage failure never changes this choice. <!-- source: internal/component/vpp/startupconf.go -- pageSize --> |
| `vpp.memory.buffers` | uint32 | `128000` | `buffers-per-numa`, beside fixed `default data-size 2048`. 128k is proven for full DFZ at 10G; packet data needs 250 MiB per NUMA node plus metadata and alignment. |
| `vpp.dpdk.interface[pci-address].name` | string | (required) | Short interface name used in ze (e.g. `xe0`). Must start with a letter, max 15 chars. |
| `vpp.dpdk.interface[pci-address].rx-queues` | uint8 | VPP default | Receive queues. Omit unless the NIC needs more. |
| `vpp.dpdk.interface[pci-address].tx-queues` | uint8 | VPP default | Transmit queues. Omit unless the NIC needs more. |
| `vpp.stats.segment-size` | size string | `512M` | Stats segment shared memory size. |
| `vpp.stats.socket-path` | string | `/run/vpp/stats.sock` | Stats Unix socket path. Separate from the API socket. |
| `vpp.stats.poll-interval` | uint16 seconds, 1..3600 | `30` | How often ze reads the stats segment for Prometheus metrics. |
| `vpp.lcp.enabled` | boolean | `true` | Whether ze asks VPP to load `linux_cp_plugin.so` and `linux_nl_plugin.so`. Leave on when BGP uses VPP-owned NICs. `ze doctor` reports `doctor-vpp-lcp-plugin` when this is on and the running VPP does not load the plugin. |
| `vpp.lcp.sync` | boolean | `true` | Mirror VPP state changes (link, MTU, IP) into the Linux TAPs. |
| `vpp.lcp.auto-subint` | boolean | `true` | Auto-create Linux TAPs for dot1q and QinQ sub-interfaces. |
| `vpp.lcp.netns` | string | `dataplane` | Network namespace where LCP TAPs appear, also emitted as VPP's global `default netns`. Must be printable, carry no whitespace and no path separator, and stay under 256 characters. VPP resolves the value as a name under `/var/run/netns/`, so it must be a namespace that exists: `host` and `root` are not special to VPP and do **not** mean "the host namespace". Ze's BGP has no netns awareness and cannot bind on a TAP in this namespace unless ze itself runs there. An EMPTY value is the exception: it clears VPP's global `default netns` and the TAPs stay in VPP's own namespace. `ze doctor` reports `doctor-vpp-lcp-netns` for every other value. See ["How the two halves fit together"](#how-the-two-halves-fit-together). |
| `vpp.plugins.dpdk` | boolean | `true` | Load DPDK. Explicit false is refused with configured DPDK interfaces. An absent installed plugin never changes this selection. <!-- source: internal/component/vpp/managed_startup.go -- startupPluginChoice, VPPSettings.managedStartupPlugins --> |
| `vpp.plugins.af-packet` | boolean | `false` | Load AF_PACKET for an explicitly configured host-interface access path. It does not create interfaces or their LCP pairs. uRPF remains enabled independently. <!-- source: internal/component/vpp/startupconf.go -- GenerateStartupConf --> |
| `vpp.plugins.wireguard` | boolean | `false` | Load `wireguard_plugin.so` so the vpp interface backend can program WireGuard tunnels (`interface { backend vpp; wireguard ...; }`). `ze doctor` warns (`doctor-vpp-wireguard`) if a wireguard interface is configured under vpp with this off. <!-- source: internal/component/vpp/yang/ze-vpp-conf.yang -- every leaf above --> <!-- source: internal/component/vpp/config.go -- defaults and validation --> <!-- source: internal/plugins/iface/vpp/doctor.go -- doctor-vpp-lcp-netns, doctor-vpp-lcp-plugin, doctor-vpp-wireguard --> |

### Enabling FIB programming

The VPP component starts VPP, but it does not program routes. That is
`fib-vpp`'s job, and it has its own switch under the `fib` container:

```
fib {
  vpp {
    enabled true;
    table-id 0;
  }
}
```

| Path | Type | Default | What it controls |
|------|------|---------|------------------|
| `fib.vpp.enabled` | boolean | `false` | Enable route programming. Off means the plugin loads but does nothing. |
| `fib.vpp.table-id` | uint32 | `0` | VRF table ID. `0` is the default VRF. |
| `fib.vpp.batch-size` | uint16 | `256` | Max routes per GoVPP batch. |
| `fib.vpp.batch-interval-ms` | uint16 | `10` | Max milliseconds to wait before dispatching a partial batch. <!-- source: internal/plugins/fib/vpp/yang/ze-fib-vpp-conf.yang -- augments /fib:fib --> |

`fib-vpp` depends on the `vpp` subsystem and on the RIB plugin. If VPP
is disabled or the GoVPP channel fails to open, the plugin programs no
route and logs an ERROR naming the cause; the rest of ze keeps running.
When VPP connects, the plugin opens its writer and asks the system RIB to
replay every route. Until then `show fib vpp` answers the cause as an
error, never an empty route list.
<!-- source: internal/plugins/fib/vpp/register.go -- Dependencies: ["rib", "vpp"] -->
<!-- source: internal/plugins/fib/vpp/register.go -- openFibVPPWriter, answerShowFibVPP -->

### SRv6 service routes

The received egress Service SID is the policy's segment. Ze allocates a separate
local BSID for steering and shares the policy across prefixes using that SID.
Replacement installs new steering before releasing the old policy; withdrawal
releases a policy only after its final reference. This does not allocate egress
Service SIDs or install remote endpoint behaviors.
<!-- source: internal/plugins/fib/vpp/srv6.go -- acquirePolicy, addSRv6Steer, delSRv6Steer, releasePolicy -->

Provide the IPv6 underlay route, encapsulation source, and required tables.
`fib.vpp.table-id` selects the policy and outer IPv6 lookup table. A nonzero
per-route table override selects destination steering, not the underlay table.
The existing Service-SID resolvability check remains a prerequisite.
<!-- source: internal/plugins/fib/vpp/srv6.go -- processSRv6Change, acquirePolicy -->
<!-- source: internal/component/sysrib/sysrib.go -- SID resolvability check -->

Ownership uses durable daemon keys under `meta/fib-vpp/srv6/`. Restore validates
confirmed policy contents and live steering references before mutation.
An unconfirmed creation found live fails closed with its BSID in the error;
matching contents alone do not authorize adoption or deletion. Preserve that
state for operator reconciliation. External writers must not race Ze's live
ownership checks: VPP has no conditional steering replacement.
<!-- source: internal/plugins/fib/vpp/register.go -- srv6OwnershipKey -->
<!-- source: internal/plugins/fib/vpp/srv6_state.go -- restore -->
<!-- source: internal/plugins/fib/vpp/srv6.go -- checkSteeringOwner -->

The ownership namespace admits at most 4,096 keys in total, not 4,096 routes.
Policy, steering, and temporary ordinary-route fallback records all count.
A transition reserves its complete growth before the first state or hardware
write; replay and withdrawal of existing keys remain possible at the limit.
<!-- source: internal/plugins/fib/vpp/srv6_fallback.go -- reserveState, checkpointSRv6Fallback -->
<!-- source: pkg/plugin/rpc/state.go -- StateListMax -->

IP/SRv6 transitions retain durable planned and confirmed ordinary-route
ownership by prefix and table. Restart restores confirmed cleanup ownership;
an uncertain IP mutation fails closed for operator reconciliation. This is
installation history, not inspection of a hidden API-source route: VPP's dump
can expose the winning SR steering source instead. Other writers must not
replace Ze-owned ordinary API-source entries, even sequentially. Matching live
routes are never adopted as proof of ownership.
<!-- source: internal/plugins/fib/vpp/srv6_fallback.go -- srv6Fallback, beginSRv6Fallback, finishSRv6Fallback -->
<!-- source: internal/plugins/fib/vpp/srv6_state.go -- loadState -->
<!-- source: internal/plugins/fib/vpp/srv6.go -- restoreSRv6 -->

Normal Ze restart can reuse confirmed live ownership. Managed-VPP reconnect
retires the old consumer, restores ownership, and requests system-RIB replay.
Replay is partial, not a snapshot: omitted owned live routes remain until
explicit withdrawal. External-VPP process restart does not emit the managed
reconnect event. The [real VPP proof](../architecture/testing/interop.md#real-vpp-srv6-service-route-proof)
defines state, packet, and restart assertions; prepared code is not observed
forwarding evidence.
<!-- source: internal/plugins/fib/vpp/register.go -- runFibVPPPlugin -->
<!-- source: internal/plugins/fib/vpp/fibvpp.go -- run -->
<!-- source: internal/component/vpp/vpp.go -- runOnce -->


## System prerequisites

VPP is not a user-space toy; DPDK needs real kernel cooperation. On a
general-purpose host Ze cannot set kernel boot parameters, so you reserve
hugepages yourself. On the gokrazy appliance Ze owns the kernel cmdline: set
`image.hugepages` (and optionally `image.memory-bytes`) in `appliance.json` and
`ze appliance build` bakes `default_hugepagesz`/`hugepagesz`/`hugepages` into the
boot cmdline for you (see the appliance guide). `ze doctor` reports
`doctor-vpp-hugepages` when VPP is enabled but the reservation is missing,
insufficient, or clamped.

The boot-time reservation proof is `./le test qemu vpp-hugepages`. Its invocation
root covers the native host build, appliance build and boot-time SSH assertions;
SIGINT, SIGTERM or SIGHUP ends the active owned command group and prevents a later
phase or renderable verdict payload. QEMU is joined before the work directory is
removed. Normal completion first sends QEMU SIGINT and preserves its existing
ten-second shutdown grace; cancellation or an error closes the owned group
immediately. Host process-fixture tests establish that lifetime contract only:
a hugepage reservation still needs the actual booted kernel's
`show host kernel` and `show host memory` answers.
<!-- source: internal/le/test/qemu/hugepages.go -- Hugepages.Run -->
<!-- source: internal/le/test/qemu/boot.go -- Hugepages.bootAndAssert, boot, ask, ssh -->

Before enabling a DPDK profile with hugepages:

| Requirement | How to provide it |
|-------------|-------------------|
| Hugepages (approximately 6 GB for production 10G, 2 GB for lab) | Appliance: `image.hugepages { page-size, count }` in `appliance.json`. Non-appliance host: `echo 3072 > /sys/kernel/mm/hugepages/hugepages-2048kB/nr_hugepages` or via `/etc/sysctl.d/`. <!-- source: internal/appliance/kernelargs.go -- hugepageKernelArgs --> |
| IOMMU enabled | BIOS: enable VT-d / AMD-Vi. Kernel cmdline: `intel_iommu=on iommu=pt` |
| CPU isolation for VPP workers | Appliance: `image.isolated-cpus` in `appliance.json`, which writes `isolcpus`, `nohz_full` and `rcu_nocbs` for those cores. Non-appliance host: the same three kernel cmdline arguments. Ze then draws its worker cores from that set, and `ze doctor` reports `doctor-vpp-cpu-isolation` when it cannot. <!-- source: internal/appliance/kernelargs.go -- isolatedCPUKernelArgs --> |
| Netlink socket buffer for route injection | `sysctl net.core.rmem_default=67108864` |
| Minimum hardware | 4 CPU cores, 8 GB RAM, a DPDK-compatible NIC <!-- source: docs/research/vpp-deployment-reference.md -- system prerequisites table --> |

Supported NICs through DPDK include Intel i210/i350, X520/X540, X710/XL710
and E810 families, plus VirtIO for VMs. Mellanox ConnectX-5 and later use
RDMA in a bifurcated driver mode; the current ze implementation is DPDK
only and does not yet bind Mellanox NICs.
<!-- source: docs/research/vpp-deployment-reference.md -- NIC driver matrix -->

## Observing what VPP is doing

Two places expose VPP state through ze:

1. **Prometheus metrics.** The stats poller exports per-interface
   counters (rx/tx packets, bytes, drops, errors), per-node clocks and
   vectors-per-call, and system-wide vector and input rates. Poll
   interval is configurable via `vpp.stats.poll-interval`.
<!-- source: internal/component/vpp/telemetry.go -- newVPPMetrics, poller.run -->
2. **The `show fib vpp` command.** Dumps the routes `fib-vpp` believes
   it has installed in VPP, as JSON `[{"prefix": ..., "next-hop": ...}, ...]`.
   Before the plugin has started, or while it has no VPP connection, it
   answers an error saying why instead.
<!-- source: internal/plugins/fib/vpp/fibvpp.go -- showInstalled -->
<!-- source: internal/plugins/fib/vpp/register.go -- answerShowFibVPP -->

Direct VPP introspection (`vppctl show int`, `vppctl show ip fib`) is
still available through the CLI socket ze writes to `/run/vpp/cli.sock`.
<!-- source: internal/component/vpp/startupconf.go -- unix cli-listen -->

## Implementation status

Configured interface names can differ from VPP's names: a VLAN named
`qos0.100` may appear as `loop1.100` in VPP. Link and address reads resolve
the configured name through the same registered interface index used by
writes, so a reload updates the existing VLAN. If that registered index
cannot be read, the reload refuses instead of treating the VLAN as absent
and creating it again. An unregistered name is looked up as an exact VPP
interface name.
<!-- source: internal/plugins/iface/vpp/query.go -- detailsByName -->

The implementation status differs by subsystem:

| Phase | Scope | Status |
|-------|-------|--------|
| vpp-3 | MPLS label stacks on IP routes | `processMPLSChange` dispatches labeled route installs and replacements to `addMPLSRoute`, and removals to `delMPLSRoute`. The GoVPP backend sends `IPRouteAddDel` with a label stack for installs, every label in uniform mode, so the label TTL is copied from the IP TTL. Transit swap and pop entries from LDP and RSVP-TE arrive as `(mpls-fib, entry)` events: fib-vpp creates MPLS table 0 and installs each entry as two `MplsRouteAddDel` routes, one per value of the S bit, never in `fib.vpp.table-id`. A pop imposes the Implicit NULL label; an egress pop with no next hop looks the packet up in IP table `fib.vpp.table-id` of its FEC's family. VPP has no per-route MTU, so an entry's path MTU is not installed and fib-vpp warns once. A push into a private table (an RSVP-TE facility-backup context) is refused, because VPP has no packet-mark context. `mpls { enable true; }` on a unit enables MPLS input on the VPP interface. <!-- source: internal/plugins/fib/vpp/fibvpp.go -- processMPLSChange --> <!-- source: internal/plugins/fib/vpp/mpls.go -- addMPLSRoute, delMPLSRoute, addLabelRoutes, labelPath --> <!-- source: internal/plugins/fib/vpp/mplsentry.go -- handleMPLSEntry --> <!-- source: internal/plugins/iface/vpp/mplsinput.go -- ReplaceMPLSInput --> |
| vpp-4 | VPP-native `iface.Backend`: managing interfaces directly via GoVPP instead of through the kernel | **In tree.** Backend registers as `"vpp"` and loads cleanly under `interface { backend vpp; }`. Interface lifecycle (CreateDummy/Bridge/VLAN, Delete, SetAdminUp/Down, SetMTU), addressing, bridge port add/del, query (`ListInterfaces`, `GetInterface`, `GetMACAddress`, `SetMACAddress`), and monitor (`WantInterfaceEvents` -> EventBus) all wired against vendored GoVPP. Tunnels (GRE/GRETAP/IPIP + VXLAN as a new kind), mirror (SPAN), wireguard, and LCP TAP pairs are now implemented (`spec-followup-vpp-iface`, GoVPP binapi vendored). GRE tunnels and wireguard interfaces are proven against real VPP 25.10 by `internal/le/test/deployment.Answer`; LCP pairs are unit- and wiring-tested but their real-VPP proof needs a VPP build shipping `linux_cp_plugin.so`/`linux_nl_plugin.so` (the `ligato/vpp-base` image does not). An interface apply that runs before the vpp handshake completes is deferred whole and performed when `EventConnected` fires (`docs/features/interfaces.md`, VPP backend). <!-- source: internal/component/iface/config_apply.go -- applyConfig, applyOwedConfig --> <!-- source: internal/plugins/iface/vpp/tunnel.go -- CreateTunnel gre/gretap/ipip --> <!-- source: internal/plugins/iface/vpp/mirror.go -- SetupMirror via SPAN --> <!-- source: internal/plugins/iface/vpp/wireguard.go -- wireguard plugin binary API --> <!-- source: internal/plugins/iface/vpp/lcp.go -- SetupLCPPair via lcp_itf_pair_add_del --> |
| vpp-5 | L2 cross-connect, bridge domains, policers, ACLs, SRv6, sFlow | Depends on vpp-4. Each feature is independent. (VXLAN tunnels landed with `spec-followup-vpp-iface`.) |
| (netns) | Netns-aware BGP listening: BGP binds inside a named network namespace | **Not in tree.** `RealListenerFactory.Listen` binds via `net.ListenConfig` in whatever namespace the ze process runs in; nothing in the BGP reactor references a network namespace. So with the default `vpp.lcp.netns` of `dataplane`, BGP reaches an LCP TAP only if ze itself runs in that namespace (see "How the two halves fit together"). Specced, not implemented; `ze doctor` reports `doctor-vpp-lcp-netns` meanwhile. <!-- source: internal/core/network/network.go -- RealListenerFactory.Listen, no netns awareness --> |

The three-strategy framing in
[`docs/research/ze-vpp-analysis.md`](../research/ze-vpp-analysis.md)
explains the bigger picture: strategy 1 (IPng / VyOS style, netlink route
intermediary) is the safe default, strategy 3 (direct FIB programming,
what ze does) is where ze differentiates by skipping the kernel entirely
and converging sub-second on a full table.

## References and further reading

| Resource | What to look at |
|----------|-----------------|
| [VPP deployment reference](../research/vpp-deployment-reference.md) | startup.conf reference, NIC matrix, performance baselines, LCP details |
| [Ze and VPP analysis](../research/ze-vpp-analysis.md) | Three-strategy feasibility analysis (strategy 1 / 2 / 3, LOC estimates, risks) |
| [VPP deployment notes](../research/vpp-deployment-notes.md) | Consolidated notes from 83 IPng.ch articles (production architecture, article index by topic, key tools, upstream contributions) |
| IPng.ch blog, VPP + LCP series (2021-08 to 2021-09, 7 parts) | How the LCP plugin works, end to end |
| IPng.ch blog, VPP configuration series (2022-03 / 2022-04) | vppcfg's DAG-based declarative config (the non-ze way) |
| IPng.ch blog, VPP monitoring (2023-04) | Stats segment interpretation, vectors-per-call |
| IPng.ch blog, VPP MPLS series (2023-05, 4 parts) | MPLS label operations in VPP (context for vpp-3) |
| IPng.ch blog, VPP sFlow series (2024-09 to 2025-02, 3 parts) | Context for future vpp-5 sFlow feature |
| go.fd.io/govpp documentation | Binary API client, stats client, binapi code generation |
| VPP 25.02 documentation | API reference for the modules ze targets |

# VPP Data Plane

**Status:** the VPP component manages the VPP process lifecycle (startup,
crash recovery, DPDK NIC binding, GoVPP connection), the `fib-vpp` plugin
programs routes from ze's system RIB directly into VPP's FIB via the
GoVPP binary API, and the stats segment is polled for per-interface,
per-node and system-wide Prometheus metrics. MPLS label programming,
a VPP-native interface backend, and VPP-native features (L2XC, bridge
domains, VXLAN, policers, ACLs, SRv6, sFlow) are designed but not yet
wired.
<!-- source: internal/component/vpp/vpp.go -- VPPManager lifecycle -->
<!-- source: internal/plugins/fib/vpp/fibvpp.go -- processEvent installs, updates, withdraws -->
<!-- source: internal/component/vpp/telemetry.go -- stats poller -->

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
<!-- source: internal/component/vpp/conn.go -- GoVPP AsyncConnect -->

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

Ze keeps two FIB backends and runs them side by side. `fib-kernel`
still programs the Linux route table so local services (SSH, the web
UI, BGP TCP sessions) keep working. `fib-vpp` pushes the same best
routes into VPP's FIB via the GoVPP binary API, so transit packets
are forwarded by VPP at DPDK speed. Both plugins subscribe to the
`(system-rib, best-change)` event on the EventBus and react
independently.
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
`ze explain doctor-vpp-lcp-netns` for the detail.

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
the loaded VPP has, not whether a copy exists on disk. A probe that cannot answer
degrades to a WARNING and never claims the plugin is missing, because `vppctl`
exits non-zero identically for an absent binary, an absent socket, and a wedged
VPP, and none of those is evidence about which plugins VPP loaded. An absent
`enabled` leaf reads as on, matching the YANG default.
<!-- source: internal/plugins/iface/vpp/doctor.go -- checkVPPLCPPlugin, lcpEnabled, lcpPluginSO -->
<!-- source: internal/core/diagnostic/codes.go -- doctor-vpp-lcp-plugin -->

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
| 3 | Renders `startup.conf` (unix, cpu, buffers, dpdk, plugins, linux-cp, linux-nl, heapsize, statseg sections) | `startupconf.go: GenerateStartupConf` |
| 4 | Loads `vfio`, `vfio_pci`, `vfio_iommu_type1` kernel modules | `dpdk.go: loadVFIOModules` |
| 5 | For each configured PCI address: reads the current driver, saves it, unbinds, binds to vfio-pci | `dpdk.go: bindPCI` |
| 6 | Execs the VPP binary with `-c <generated startup.conf>` | `vpp.go: runOnce` |
| 7 | AsyncConnects GoVPP to `/run/vpp/api.sock` (10 attempts, 1 s apart) | `conn.go: Connect` |
| 8 | Emits `("vpp", "connected")` on the EventBus | `vpp.go: emitEvent` |
| 9 | Starts the stats poller (per-interface, per-node, system metrics) | `telemetry.go` |
| 10 | Waits for VPP to exit. On crash: emits `("vpp", "disconnected")`, backs off (1 s, 2 s, 4 s, capped at 30 s), restarts, emits `("vpp", "reconnected")`. `fib-vpp` replays the RIB on reconnect. | `vpp.go: Run loop`, `fibvpp.go: reconnect handler` |
| 11 | On clean shutdown: SIGTERM VPP, triggers a PCI rescan, restores the original NIC drivers | `vpp.go: defer`, `dpdk.go: UnbindAll` |

The point is that VPP's system-level prerequisites (vfio module load,
NIC unbind, driver save, rescan-on-teardown) are part of ze's job, not
the operator's. This matters on a gokrazy appliance where there is no
systemd and ze is PID 1 for the data plane.
<!-- source: internal/plugins/init/main.go -- gokrazy PID-1 appliance lifecycle -->

## Running against an externally supervised VPP

Set `vpp.external true` when something other than ze owns the VPP
process -- a systemd unit, a container sidecar, or the Python stub
the functional tests use. In this mode ze skips steps 3, 4, 5, 6 and 11
of the table above: no `startup.conf` generation, no vfio module load,
no NIC unbind, no `exec vpp`, no PCI rescan on shutdown. Ze still
connects via GoVPP at step 7, emits the same lifecycle events at step 8,
runs the stats poller at step 9, and blocks on context cancellation
instead of `cmd.Wait` at step 10.
<!-- source: internal/component/vpp/vpp.go -- External branch in Run and runOnce -->

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
- **Gokrazy containers:** a sidecar image bundles `vpp` and `ze`; the
  supervisor starts VPP first, then ze with `external true`.
- **Functional tests:** `bin/ze-test vpp` runs `test/vpp/*.ci` which
  drive a Python `vpp_stub.py` listening on the API socket instead of
  a real VPP. See `docs/functional-tests.md` for the harness.
<!-- source: test/scripts/vpp_stub.py -- Python GoVPP API stub -->

## Configuring VPP

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

This is enough to boot VPP with the default heap, default buffer count,
default stats segment and LCP enabled. Add cores, tune memory, or change
the stats poll interval only when the defaults do not fit the workload.

### Every leaf, what it does, what it defaults to

| Path | Type | Default | What it controls |
|------|------|---------|------------------|
| `vpp.enabled` | boolean | `false` | Master switch. `false` means ze does not start VPP at all. |
| `vpp.external` | boolean | `false` | When `true`, ze connects to an existing VPP via `api-socket` but does NOT generate `startup.conf`, bind DPDK NICs, or exec the VPP binary. Use this on systemd-managed hosts, container sidecars, or the `ze-test vpp` stub harness. Default `false` preserves the ze-owned-lifecycle behaviour. |
| `vpp.api-socket` | string | `/run/vpp/api.sock` | GoVPP Unix socket. Ze validates it is absolute, has no `..`, and fits in 108 characters. |
| `vpp.cpu.main-core` | uint8 | auto | CPU core pinned to the VPP main thread. Omit for VPP default. |
| `vpp.cpu.workers` | uint8 | auto | Number of worker threads. Ze allocates `main-core+1 .. main-core+workers` for `corelist-workers` in startup.conf. |
| `vpp.cpu.poll-sleep` | `Nms` (0ms–100ms) | unset | Fixed sleep between VPP main-loop polls, expressed in whole milliseconds (`ms` is the only accepted unit, e.g. `10ms`), emitted as `unix { poll-sleep-usec N }` (1ms = 1000µs). Omit for lowest latency (workers busy-poll at 100% CPU); set a non-zero value on shared or dev hosts to trade latency for idle CPU. An explicit `0ms` is emitted and equals VPP's default. <!-- source: internal/component/vpp/config.go -- parsePollSleepMs --> |
| `vpp.memory.main-heap` | size string | `1G` | VPP main heap. Use `1536M` for a full DFZ (approximately 958k IPv4 + 198k IPv6 routes). |
| `vpp.memory.hugepage-size` | `2M` or `1G` | `2M` | Hugepage size. `2M` is the common case; `1G` for large installations. |
| `vpp.memory.buffers` | uint32 | `128000` | Buffers per NUMA node. 128k is proven for full DFZ at 10G. |
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
| `vpp.plugins.wireguard` | boolean | `false` | Load `wireguard_plugin.so` so the vpp interface backend can program WireGuard tunnels (`interface { backend vpp; wireguard ...; }`). `ze doctor` warns (`doctor-vpp-wireguard`) if a wireguard interface is configured under vpp with this off. |
<!-- source: internal/component/vpp/yang/ze-vpp-conf.yang -- every leaf above -->
<!-- source: internal/component/vpp/config.go -- defaults and validation -->
<!-- source: internal/plugins/iface/vpp/doctor.go -- doctor-vpp-lcp-netns, doctor-vpp-lcp-plugin, doctor-vpp-wireguard -->

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
| `fib.vpp.batch-interval-ms` | uint16 | `10` | Max milliseconds to wait before dispatching a partial batch. |
<!-- source: internal/plugins/fib/vpp/yang/ze-fib-vpp-conf.yang -- augments /fib:fib -->

`fib-vpp` depends on the `vpp` subsystem and on the RIB plugin. If VPP
is disabled or the GoVPP channel fails to open, the plugin falls back to
a noop backend and logs a warning instead of blocking the rest of ze.
<!-- source: internal/plugins/fib/vpp/register.go -- Dependencies: ["rib", "vpp"] -->
<!-- source: internal/plugins/fib/vpp/register.go -- mockBackend fallback when connector is nil -->

## System prerequisites

VPP is not a user-space toy; DPDK needs real kernel cooperation. On a
general-purpose host Ze cannot set kernel boot parameters, so you reserve
hugepages yourself. On the gokrazy appliance Ze owns the kernel cmdline: set
`image.hugepages` (and optionally `image.memory-bytes`) in `appliance.json` and
`ze appliance build` bakes `default_hugepagesz`/`hugepagesz`/`hugepages` into the
boot cmdline for you (see the appliance guide). `ze doctor` reports
`doctor-vpp-hugepages` when VPP is enabled but the reservation is missing,
insufficient, or clamped. Before enabling VPP:

| Requirement | How to provide it |
|-------------|-------------------|
| Hugepages (approximately 6 GB for production 10G, 2 GB for lab) | Appliance: `image.hugepages { page-size, count }` in `appliance.json`. Non-appliance host: `echo 3072 > /sys/kernel/mm/hugepages/hugepages-2048kB/nr_hugepages` or via `/etc/sysctl.d/`. <!-- source: internal/appliance/kernelargs.go -- hugepageKernelArgs --> |
| IOMMU enabled | BIOS: enable VT-d / AMD-Vi. Kernel cmdline: `intel_iommu=on iommu=pt` |
| CPU isolation for VPP workers | Kernel cmdline: `isolcpus=<worker-cores>` so Linux does not schedule on them |
| Netlink buffer for route injection | `sysctl net.core.rmem_default=67108864` |
| Minimum hardware | 4 CPU cores, 8 GB RAM, a DPDK-compatible NIC |
<!-- source: docs/research/vpp-deployment-reference.md -- system prerequisites table -->

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
<!-- source: internal/plugins/fib/vpp/fibvpp.go -- showInstalled -->

Direct VPP introspection (`vppctl show int`, `vppctl show ip fib`) is
still available through the CLI socket ze writes to `/run/vpp/cli.sock`.
<!-- source: internal/component/vpp/startupconf.go -- unix cli-listen -->

## What is not yet wired

Today, VPP process lifecycle, IPv4/IPv6 FIB programming, and stats
telemetry are in the tree. The remaining phases:

| Phase | What it adds | Why not yet |
|-------|--------------|-------------|
| vpp-3 | MPLS label push / swap / pop driven from BGP labelled unicast | **In tree.** Labels stripped at NLRI parse (SplitLabeled, RFC 8277), stored as FamilyRIB side-data, propagated through bgp-rib and sysRIB BestChangeEntry.Labels, programmed into VPP via IPRouteAddDel with LabelStack (push) or MplsRouteAddDel (swap/pop). 20-bit label range and stack depth 16 validated before GoVPP call. |
| vpp-4 | VPP-native `iface.Backend`: managing interfaces directly via GoVPP instead of through the kernel | **In tree.** Backend registers as `"vpp"` and loads cleanly under `interface { backend vpp; }`. Interface lifecycle (CreateDummy/Bridge/VLAN, Delete, SetAdminUp/Down, SetMTU), addressing, bridge port add/del, query (`ListInterfaces`, `GetInterface`, `GetMACAddress`, `SetMACAddress`), and monitor (`WantInterfaceEvents` -> EventBus) all wired against vendored GoVPP. Tunnels (GRE/GRETAP/IPIP + VXLAN as a new kind), mirror (SPAN), wireguard, and LCP TAP pairs are now implemented (`spec-followup-vpp-iface`, GoVPP binapi vendored). GRE tunnels and wireguard interfaces are proven against real VPP 25.10 by `scripts/evidence/effective-vpp-iface.py`; LCP pairs are unit- and wiring-tested but their real-VPP proof needs a VPP build shipping `linux_cp_plugin.so`/`linux_nl_plugin.so` (the `ligato/vpp-base` image does not). Iface-component reconciliation still races the vpp handshake at startup and degrades to additive-only -- tracked in `spec-iface-vpp-ready-gate`. |
<!-- source: internal/plugins/iface/vpp/tunnel.go -- CreateTunnel gre/gretap/ipip -->
<!-- source: internal/plugins/iface/vpp/mirror.go -- SetupMirror via SPAN -->
<!-- source: internal/plugins/iface/vpp/wireguard.go -- wireguard plugin binary API -->
<!-- source: internal/plugins/iface/vpp/lcp.go -- SetupLCPPair via lcp_itf_pair_add_del -->
| vpp-5 | L2 cross-connect, bridge domains, policers, ACLs, SRv6, sFlow | Depends on vpp-4. Each feature is independent. (VXLAN tunnels landed with `spec-followup-vpp-iface`.) |
| (netns) | Netns-aware BGP listening: BGP binds inside a named network namespace | **Not in tree.** `RealListenerFactory.Listen` binds via `net.ListenConfig` in whatever namespace the ze process runs in; nothing in the BGP reactor references a network namespace. So with the default `vpp.lcp.netns` of `dataplane`, BGP reaches an LCP TAP only if ze itself runs in that namespace (see "How the two halves fit together"). Specced, not implemented; `ze doctor` reports `doctor-vpp-lcp-netns` meanwhile. |
<!-- source: internal/core/network/network.go -- RealListenerFactory.Listen, no netns awareness -->

The three-strategy framing in
[`docs/research/ze-vpp-analysis.md`](../research/ze-vpp-analysis.md)
explains the bigger picture: strategy 1 (IPng / VyOS style, netlink
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

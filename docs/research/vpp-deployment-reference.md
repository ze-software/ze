# VPP Deployment Reference

Source: 83 blog articles from ipng.ch (Pim van Pelt / IPng Networks, AS8298 / AS50869),
VyOS VPP integration, and GoVPP documentation.

Full article archive: `~/Code/site/ipng.ch/articles/`
Consolidated notes: `~/Code/site/ipng.ch/vpp-deployment-notes.md`
Feasibility analysis: `~/Code/site/ipng.ch/ze-vpp-analysis.md`

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
| buffers | default-data-size | 2048 | Standard. Use 9216 for jumbo frames (9000B MTU + headers). |
| buffers | page-size | default-hugepage-size | Match hugepage size. 2M for most deployments. |
| heapsize | main-heap-size | 1536M (1.5G) | Enough for ~958K IPv4 + ~198K IPv6 full DFZ FIB. |
| statseg | size | 1G | Stats segment shared memory for per-interface/node counters. |
| statseg | page-size | default-hugepage-size | Match hugepage size. |
| dpdk | dev <pci> { name, num-rx-queues, num-tx-queues } | Per-NIC | See NIC table below. |
| plugins | plugin default { disable } | (block) | Disable all, enable specific. |
| plugins | plugin dpdk_plugin.so { enable } | (line) | NIC driver. Always enabled. |
| plugins | plugin linux_cp_plugin.so { enable } | (line) | LCP. Enable when control-plane connectivity needed. |
| plugins | plugin linux_nl_plugin.so { enable } | (line) | Netlink listener. Enable with LCP. |
| linux-cp | lcp-sync | (flag) | VPP state changes propagate to Linux TAP mirrors. |
| linux-cp | lcp-auto-subint | (flag) | Auto-create sub-TAPs for dot1q/QinQ sub-interfaces. |
| linux-cp | default netns | dataplane | LCP TAPs created in `dataplane` netns. Routing daemons run there. |
| linux-nl | rx-buffer-size | 67108864 | 64MB netlink buffer. Required for full DFZ route injection without overflow. |

### startup.conf Syntax

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

String values are unquoted. Multi-word values use spaces. Booleans are bare flags (present = true).
Dev entries use PCI address as the key with a nested block for per-device options.

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

Per configured PCI address:

1. Read current driver: `/sys/bus/pci/devices/<addr>/driver` symlink basename
2. Save original driver to persistent state
3. Load kernel modules: `vfio`, `vfio_pci`, `vfio_iommu_type1`
4. Unbind from current driver: write PCI address to `/sys/bus/pci/devices/<addr>/driver/unbind`
5. Bind to vfio-pci: write `vendor:device` to `/sys/bus/pci/drivers/vfio-pci/new_id`

Reverse on teardown: unbind vfio-pci, PCI rescan (`/sys/bus/pci/rescan`), rebind original driver.

Source: VyOS `control_host.py` (Python, ported to Go for ze).

### Mellanox RDMA Exception

mlx5 NICs use bifurcated driver mode. The kernel `mlx5_core` driver stays loaded. VPP creates
the interface via `create interface rdma host-if <pci-addr>` instead of a DPDK dev entry.

This means:
- No vfio binding step
- No driver save/restore
- DPDK section in startup.conf does NOT include mlx5 devices
- Interface creation uses GoVPP `RdmaCreate` API instead of DPDK auto-discovery
- Performance is equivalent to DPDK for mlx5

### NIC-Specific Quirks

| NIC | Issue | Workaround |
|-----|-------|-----------|
| Intel X710 (early VPP 21.06) | DPDK driver failures with vfio-pci and igb_uio | Upgrade VPP. Fixed in later releases. |
| Intel i40e (some firmware) | RSS problems with flow-director enabled | Disable flow-director in DPDK dev config. |
| Any NIC with jumbo frames | Default buffer size (2048) too small | Set `default-data-size 9216` in buffers section. |

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

## GoVPP Connection

### Socket Transport

GoVPP connects via Unix socket at `/run/vpp/api.sock`. Pure Go, no CGo.

AsyncConnect pattern:
- 10 connection attempts, 1 second interval
- Returns event channel for connection state changes (Connected, Disconnected)
- On disconnect: attempt reconnect with exponential backoff

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

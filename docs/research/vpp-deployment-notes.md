# IPng Networks: VPP Deployment Notes

Source: 83 blog articles from ipng.ch (Pim van Pelt / IPng Networks, AS8298 / AS50869)

## Overview

IPng Networks runs VPP as the production forwarding plane on all backbone routers across AS8298 (and previously AS50869). VPP replaces Linux kernel routing, achieving 8-35x forwarding performance. The control plane (Bird2 or FRR) runs in a `dataplane` network namespace, connected to VPP via the Linux Control Plane (LCP) plugin that mirrors VPP interfaces as TAP devices.

The blog documents a multi-year journey from initial Coloclue experiments (2021) to a 12-router European backbone with full DFZ tables, MPLS, sFlow, eVPN prototyping, FreeBSD porting, and upstream VPP contributions.

---

## Production Architecture

### Core Stack

| Layer | Technology |
|-------|-----------|
| Forwarding | VPP (custom-built .deb packages from source) |
| Control plane | Bird2 (custom-built with OSPFv3 IPv4 AF patch) |
| Config tool | vppcfg (Python, YAML-based declarative config) |
| OS | Debian 12 Bookworm (migrated from Ubuntu 20.04 in 2023) |
| Monitoring | SNMP AgentX (Python) for LibreNMS + Prometheus exporter (C) for Grafana |
| Network namespace | `dataplane` (SSH, SNMP, Bird, all run inside it) |
| LCP plugin | Custom `lcpng` fork with MPLS, sFlow, and sub-interface improvements |

### Standard Router Hardware

Primary platform: **Supermicro SYS-5018D-FN8T**
- CPU: Intel Xeon D1518, 4 cores / 8 threads, 2.2 GHz, 35W TDP
- RAM: 2x16 GB ECC
- NICs: 2x Intel i210 (1G), 4x Intel i350 (1G), 2x Intel X552 (10G), optional Intel X710-DA4 (4x10G) or X710-XXV (2x25G) in PCIe v3.0 x8 slot
- Boot: mSATA 120 GB SSD
- Management: Full IPMI (KVM-over-IP, Serial-over-LAN, virtual CDROM for remote reinstall)
- Power: ~45W fully loaded
- Price: ~USD 800
- Capacity: ~35 Mpps across six 10G ports (IPv4/IPv6/MPLS)

Upgrade option: Supermicro SIS810 with Xeon E-2288G (8C/16T), dual PSU, 8x i210, 2x X710 Quad-10G.

### VPP Startup Configuration (Production)

- CPU isolation: `isolcpus=1,2,3,5,6,7` in GRUB (cores 0+4 for Linux, cores 1-3 for VPP workers)
- Hugepages: 6 GB (3072 x 2MB)
- Netlink buffer: 64 MB (`net.core.rmem_default=67108864`)
- startup.conf: 1536 MB main heap, 128K buffers, 1 GB stats segment
- LCP plugins enabled, `exec /etc/vpp/bootstrap.vpp` for persistent config
- Performance: ~660 clocks/packet at 1 Mpps; VPP gets more efficient under load (instruction/data cache reuse)

### Backbone Topology (AS8298)

12 routers across Europe and US:
- Switzerland: chbtl0, chbtl1, chgtg0, chplo0, chrma0
- Germany: ddln0, ddln1, defra0
- France: frpar0, frggh0
- Netherlands: nlams0
- US: usfmt0

Ring topology with 10G EoMPLS and 25G wavelength links. All links unnumbered (loopback-only OSPFv3) since mid-2024, reclaiming 34 IPv4 addresses (13.3% of their /24).

Routing tables: ~958K IPv4 + ~198K IPv6 prefixes (full DFZ). Bird2 loads routes into VPP's FIB via Netlink at ~175K routes/sec (~6 seconds for a full table).

### Protocols

| Protocol | Role |
|----------|------|
| OSPFv3 (dual AF) | IGP for both IPv4 and IPv6 (replaced OSPFv2 in 2024) |
| BFD | Fast failure detection (100ms interval, multiplier 20) |
| iBGP | Full mesh via route reflectors (AS50869/AS8298) |
| eBGP | IXP peering (DE-CIX, Kleyrex, LoCIX, AMS-IX, NL-IX, LSIX, FrysIX) |
| MPLS/LDP | Label switching via FRR LDP daemon (added 2023) |
| VXLAN | L2VPN services (VNI-based, IPv4 underlay, 9000B jumbo backbone) |
| sFlow v5 | Packet sampling (merged upstream VPP 25.02) |

---

## Article Index by Topic

### 1. Coloclue Deployment (AS8283)

**2021-03-27 - VPP at Coloclue, part 1**
- First attempt to deploy VPP on Coloclue router `dcg-1` at NL-IX
- Hardware: Supermicro X11SCW-F, 6 cores, Intel x710-DA4 quad 10G
- Failed: Intel x710 NIC driver issues with DPDK (vfio-pci and igb_uio)
- Side benefit: BIOS/kernel changes (IOMMU, disabled HT, kernel 5.10) improved latency from 247ms worst-case to 4.3ms on Linux+Bird setup
- VPP v21.06, Debian Buster

**2023-02-24 - VPP at Coloclue, part 2**
- Successful deployment on router `eunetworks-2` to fix 5-6.5% packet loss from kernel forwarding
- Hardware: Intel E-2286G @ 4GHz, 6C/12T, 2x i40e 10G
- VPP 23.06-rc0, Bird 1.6.8, vppcfg for config
- LACP bond (2x 10G), multiple member VLANs, AMS-IX peering
- Result: zero packet loss, ~5.7 Gbps throughput
- Known issue: IPv6 OSPF stub area on loopback causes dataplane hang

### 2. Linux Control Plane Plugin (LCP) Series

Seven-part series (2021-08 to 2021-09) developing the LCP plugin that mirrors VPP interfaces into Linux as TAP devices, enabling standard routing daemons to control VPP's dataplane.

**Part 1** - Core LIP (Linux Interface Pair) creation for all sub-interface types (dot1q, dot1ad, QinQ). Two AMD64 machines with X710-DA4, Ubuntu 20.04, VPP 21.06.

**Part 2** - Bidirectional sync: VPP state changes (link, MTU, IP) propagate to Linux TAPs automatically via `lcp-sync` toggle.

**Part 3** - Auto-LIP creation for sub-interfaces (`lcp-auto-subint`). Added LACP BondEthernet support. MAC address bug: LIP must be created after bond members are added.

**Part 4** - Netlink Listener plugin: Linux-to-VPP sync. Standard `ip` commands in Linux configure VPP dataplane. 64 MB netlink socket buffer. Known crash in multithreaded mode.

**Part 5** - IPv4/IPv6 route handling. Full DFZ demo: 870K IPv4 + 133K IPv6 prefixes loaded at ~175K routes/sec via Bird2. FIB source priority: `lcp-rt` (static) and `lcp-rt-dynamic` (routing protocols).

**Part 6** - SNMP AgentX daemon (Python) reading VPP stats segment for LibreNMS. Implements ifTable/ifXTable MIBs with 64-bit counters.

**Part 7** - Complete production deployment guide. Supermicro SYS-5018D-FN8T. CPU isolation, hugepages, systemd units, Bird2 config. Router `frlil0` (Lille, France) ran 17 days zero crashes at ~18 Gbit sustained. Replaces DANOS (Vyatta-based) routers.

**2021-12-23 - VM Playground**
- QEMU/KVM VM image with VPP + LCP + FRR + Bird2 pre-installed
- Debian Bullseye/Bookworm guest, 4 VirtIO NICs
- Both FRR and Bird2 preconfigured for easy experimentation
- LCP running in production at AS8298 since Sep 2021 with no crashes

### 3. L2 Services and VLANs

**2022-01-12 - Virtual Leased Line (VLL)**
- Benchmark of 7 L2 tunnel configs: direct L2XC, GRE, VXLAN, GENEVE (each IPv4/IPv6)
- Hardware: Dell R720xd (T-Rex), ASRock B550 Ryzen 5950X with Intel E810C 100G (VPP routers)
- VXLAN-v4 chosen for production: ~14 Mpps/core, UDP source-port scrambling for RSS
- IPv4 tunnels significantly faster than IPv6 (ip6-receive node costs ~170 cycles vs ~27 for ip4)

**2022-02-14 - VLAN Gymnastics**
- Sub-interface model: dot1q, dot1ad, QinQ, exact-match
- Bridge domains, L2 cross-connects, VLAN tag rewriting
- Production example: customer L2VPN between Amsterdam and Zurich via VXLAN

### 4. Configuration Tooling (vppcfg)

**2022-03-27 - VPP Configuration Part 1**
- vppcfg: Python tool with YAML config, Yamale schema validation, 59 semantic constraints
- Models: loopbacks, bonds, DPDK/RDMA interfaces, sub-interfaces, bridge domains, VXLAN tunnels, L2XC, LCPs

**2022-04-02 - VPP Configuration Part 2**
- DAG-based path planner: computes ordered VPP API calls to transition running state to target config
- Three phases: prune (innermost first), create (outermost first), sync (attributes)
- Safe transitions between radically different configurations

### 5. Lab Infrastructure

**2022-10-14 - VPP Lab Setup**
- 3x Dell R720XD hypervisors (128 GB RAM, 4x10G Intel 82599ES), KVM/QEMU
- Open vSwitch networking with VLAN-based interface pairing, mirror ports
- ZFS + zrepl for snapshot management and instant VM rollback
- Python + Jinja2 config generator for per-VM overlays
- Per-lab topology: 4 VPP VMs daisy-chained, 2 host VMs, 1 tap VM
- FS.com S5860-48SC switch (48x10G, 8x100G) for experiments

### 6. Monitoring

**2023-04-09 - VPP Monitoring**
- SNMP AgentX (Python): VPP stats segment to LibreNMS
- Prometheus exporter (C): reads stats segment, port 9482, per-interface/node/thread counters
- Key metrics: CPU cycles/packet/node (logarithmic), vectors/call (1.00=idle, 256=saturated)
- Production: >150 Mpps / >180 Gbps demonstrated across AS8298

### 7. MPLS

Four-part series (2023-05) adding MPLS support to VPP's Linux Control Plane.

**Part 1** - Static MPLS LSPs: manual PUSH/SWAP/POP on lab VMs. 18-20 Mpps/thread for MPLS forwarding.

**Part 2** - Performance benchmarking on bare metal Supermicro routers. PHP (Implicit NULL) is fastest mode. MPLS supports RSS for linear multi-worker scaling (~27 Mpps with 3 workers).

**Part 3** - LCP plugin MPLS support (with Adrian "vifino" Pistol): interface MPLS enable/disable, Netlink MPLS route parsing, dual EOS/non-EOS FIB entries. FRR LDP for dynamic LSPs.

**Part 4** - Bug fix: MPLS packets from Linux TAP incorrectly subjected to VPP FIB lookup. New `linux-cp-xc-mpls` graph node bypasses FIB for TAP-originated MPLS.

### 8. IXP Gateway

**2023-10-21 - VPP IXP Gateway**
- Bridge domains with mixed tagged/untagged ports
- MAC address filtering via L2 input ACL classifiers
- Policer limitations: only worked on PHY input, not sub-interfaces or L2 (fixed in 2026-02-14 article)

### 9. OS Migration

**2023-12-17 - Debian on VPP Routers**
- Migrated all 12 AS8298 routers from Ubuntu 20.04 to Debian 12 Bookworm
- Remote reinstall via IPMI virtual CDROM, Borg backup restore, Ansible post-install
- ~25 minutes per router, zero packet loss (traffic drained via OSPF cost + eBGP shutdown)
- VPP v24.02-rc0, custom .deb packages hosted on ipng.ch

### 10. VPP Python API

**2024-01-27 - VPP Python API**
- Deep dive into `vpp_papi` for programmatic VPP interaction
- Unix socket `/run/vpp/api.sock`, ~50 core + ~80 plugin APIs
- Three patterns: Request/Reply, Dump/Detail (streaming), Events (async callbacks)

### 11. FreeBSD Port

**2024-02-10 - VPP on FreeBSD Part 1**
- Partnership with Tom Jones (FreeBSD Foundation)
- FreeBSD 14.0-RELEASE, contigmem + nic_uio kernel modules
- Basic ping working on VirtIO NICs

**2024-02-17 - VPP on FreeBSD Part 2**
- Benchmark: FreeBSD kernel bridge (1.2 Mpps/thread) vs VPP+DPDK (20.96 Mpps)
- Supermicro SYS-5018D-FN8T with X710-XXV 25G
- netmap forwarding has a stalling bug; VPP+DPDK works well

### 12. Loopback-only OSPFv3 (IPv4 Address Conservation)

**2024-04-06 - Part 1 (Lab)**
- Two solutions for OSPFv3 with IPv4 using only loopback addresses
- Bird2 patch for RFC 5838 Link-LSA + VPP ARP patch for unnumbered interfaces
- BFD for fast convergence

**2024-06-22 - Part 2 (Production Rollout)**
- Migrated all AS8298 backbone links from OSPFv2 to OSPFv3 for IPv4
- Reclaimed 34 IPv4 addresses (13.3% of /24)
- vppcfg updated for `unnumbered: loop0` support
- ~4 minutes per link migration (drain OSPF, reconfigure, undrain)

### 13. sFlow

Three-part series (2024-09 to 2025-02) developing an sFlow plugin for VPP, merged upstream in VPP 25.02.

**Part 1** - Initial prototype by Neil McKee (inMon). Samples 1-in-N packets, sends via PSAMPLE Netlink. Performance issues with RPC-to-main spinlock.

**Part 2** - Five iterations from RPC (v1, destructive regression) to custom lockless FIFO with quad-bucket-brigade (v5). Final: 11 CPU cycles/packet overhead, 36 Mpps bidirectional. Submitted as Gerrit 41680 (~2600 lines).

**Part 3** - Configuration guide (CLI, Python API, vppcfg YAML). hsflowd integration. ifIndex namespace resolution. Merged upstream in VPP 25.02. Lightning talk at FOSDEM 2025.

### 14. Containerlab Integration

**2025-05-03 - Part 1**
- Docker image: Debian Bookworm + VPP 25.02 stable, af-packet driver (no DPDK/hugepages)
- Runs with `--privileged` or targeted capabilities
- Single thread, 512M heap, 4kB pages

**2025-05-04 - Part 2**
- Added SSH, Bird2, vppcfg, auto-discovery init script
- Merged into Containerlab as VPP node type (PR#2571, ~170 LOC Go)
- Co-authored with Roman Dodin (Nokia/Containerlab)

### 15. eVPN/VxLAN

**2025-07-12 - VPP and eVPN/VxLAN Part 1**
- Prototype for dynamic VxLAN endpoints: per-MAC L2FIB entries replace static tunnels
- Commands: `vxlan l2fib`, `vxlan flood` for BUM replication
- Found/fixed IPv4 header checksum bug
- Gerrit 43433 (~proof of concept)

### 16. VPP Upstream Contributions

**2026-02-14 - Policers**
- Fixed policers for sub-interfaces and L2 mode (bridge-domain, L2XC)
- Added `L2INPUT_FEAT_POLICER` / `L2OUTPUT_FEAT_POLICER` feature bitmap entries
- L2 overhead compensation for accurate bandwidth accounting
- Gerrit 44654, ~300 LOC

**2026-02-21 - SRv6 L2VPN**
- Fixed SRv6 `sr steer` and `end.dx2` for sub-interfaces
- Found/fixed quad-loop indexing bug in `sr_policy_rewrite_encaps_l2()` causing ~75% packet loss (indicating L2 SRv6 was essentially unused before)
- Gerrit 44899, ~850 LOC

### 17. Hardware Reviews with VPP Benchmarks

| Device | CPU | NICs | VPP Perf (64B) | Power | Price |
|--------|-----|------|----------------|-------|-------|
| Supermicro 5018D-FN8T | Xeon D1518 4C 2.2GHz | 2x10G + 6x1G + add-in | ~35 Mpps | ~45W | ~$800 |
| Netgate 6100 | Atom C-3558 4C 2.2GHz | 2x10G + 4x2.5G + 2x1G | 5 Mpps (1 worker) | 19W | $699 |
| Compulab Fitlet2 | Atom E3950 4C 1.6GHz | 2x1G + 1G SFP | 2.97 Mpps (3x1G line rate) | 11W | ~$400 |
| GoWin R86S | Pentium N6005 4C 2GHz | OCP slot + 3x2.5G | 13.4 Mpps (X520 10G) | 17-20W | CHF 314 |
| Gowin 1U N305 | i3-N305 8C 3GHz | OCP 2x25G + 2x2.5G + 3x1G | 18.34 Mpps (Cx4 RDMA) | 47.5W | - |
| Dell R720xd (lab/T-Rex) | Xeon E5-2620 | X710 4x10G | N/A (loadtester) | - | - |
| ASRock B550 Ryzen 5950X (lab) | Ryzen 5950X 16C | E810C 2x100G | 14+ Mpps/core | - | - |

---

## Key Tools and Components

| Tool | Purpose | Source |
|------|---------|--------|
| vppcfg | Declarative YAML VPP config with DAG-based path planner | git.ipng.ch/ipng/vppcfg |
| lcpng | Enhanced Linux Control Plane plugin (MPLS, sFlow, auto-subint) | GitHub (Pim's fork) |
| vpp-snmp-agent | Python SNMP AgentX for LibreNMS | GitHub |
| VPP Prometheus exporter | C-based stats segment exporter | Custom |
| T-Rex | Cisco stateless loadtester | trex-tgn.cisco.com |
| hsflowd | Host sFlow daemon with mod_vpp | GitHub/sflow |

## Key VPP Upstream Contributions (Gerrit)

| Gerrit | Feature |
|--------|---------|
| 38826 | MPLS interface state callbacks for LCP |
| 38702 | Netlink MPLS route handling for LCP |
| 40482 | ARP on unnumbered interfaces |
| 41680 | sFlow plugin (~2600 LOC, merged in VPP 25.02) |
| 43433 | Dynamic VxLAN endpoints (eVPN prototype) |
| 44654 | Policers on sub-interfaces and L2 |
| 44899 | SRv6 L2VPN on sub-interfaces + quad-loop bug fix |

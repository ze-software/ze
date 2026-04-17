# VPP Support in Ze: Feasibility Analysis

## Sources

- 83 blog articles from ipng.ch (Pim van Pelt / IPng Networks, AS8298)
- VyOS vyos-1x codebase (VPP integration: ~6000 LOC Python + XML + Jinja2)
- Ze codebase (current state)

---

## What IPng and VyOS Teach Us

### The proven architecture (both converge on the same design)

Both IPng's production network and VyOS's implementation arrive at the same architecture:

1. **VPP owns the NICs** via DPDK (or RDMA). The kernel no longer sees the physical interfaces.
2. **LCP plugin creates TAP mirrors** in Linux with the same interface names. The kernel sees TAP devices that behave identically to the original NICs.
3. **Routing daemons run in Linux** (Bird2 or FRR), manage TAPs normally, write routes to the kernel routing table.
4. **VPP's netlink plugin** consumes kernel route changes and programs VPP's FIB.
5. **Configuration** is declarative: vppcfg (YAML + DAG path planner) at IPng, XML + Python conf_mode at VyOS.

This means: the control plane does not know VPP exists. It talks to Linux. VPP is invisible infrastructure underneath.

### What VPP provides

| Capability | Performance |
|-----------|-------------|
| IPv4/IPv6 forwarding | ~35 Mpps on Xeon D1518 (4C 2.2GHz) |
| MPLS label switching | 18-20 Mpps/thread |
| L2 cross-connect | >14.88 Mpps/thread |
| VXLAN/GENEVE tunnels | ~14 Mpps/thread (IPv4 underlay) |
| SRv6 L2VPN | Working (Gerrit 44899) |
| sFlow sampling | 11 CPU cycles/packet overhead |
| Bridge domains | MAC learning, BVI, tag rewriting |
| Policers | Per-interface, per-sub-interface |
| NAT44 / CGNAT | VyOS exposes both |
| ACLs | IP + MAC, L2 and L3 |
| LACP bonding | Full support |

### What VPP does NOT provide

- No BGP, OSPF, IS-IS, or any routing protocol (by design)
- No firewall (nftables/iptables bypass). VPP ACLs are basic.
- No QoS (VPP policers are flat rate limiters, not hierarchical schedulers)
- No kernel features on the fast path (tc, XDP, conntrack)
- DPDK/RDMA NIC drivers only (not all hardware supported)
- Minimum 4 physical CPU cores, 8 GB RAM

---

## What Ze Could Achieve

### Ze's current position

Ze is **control-plane only**. It runs BGP, programs the kernel FIB via `fib-kernel` plugin (netlink), and manages interfaces via the `iface` component with a pluggable backend (currently netlink-only). Ze does not forward packets.

### The opportunity

Ze already has the right abstractions to add VPP as an **alternative forwarding plane**:

| Ze abstraction | Current impl | VPP impl would be |
|---------------|-------------|-------------------|
| `iface.Backend` interface | `ifacenetlink` (30 methods) | `ifacevpp` via GoVPP binary API |
| `fibkernel.routeBackend` | netlink `RTM_NEWROUTE` | VPP FIB API (or keep kernel, let LCP sync) |
| YANG config | `ze-iface-conf.yang` | Add `vpp {}` container with settings |
| Plugin registration | `init()` + `RegisterBackend()` | Same pattern |

### Three integration strategies (increasing ambition)

#### Strategy 1: LCP-transparent (lowest effort, highest leverage)

Do what IPng does: let VPP + LCP run underneath, ze talks to Linux TAPs without knowing VPP exists.

- Ze configures interfaces via netlink (as today). VPP's LCP plugin mirrors everything.
- Ze's `fib-kernel` programs kernel routes. VPP's netlink plugin syncs them.
- Ze manages VPP lifecycle (startup.conf generation, DPDK binding, systemd unit) as a new component.
- No changes to BGP, interface, or FIB code paths.

**What ze adds:** VPP lifecycle management, startup.conf generation from YANG config, NIC driver binding/unbinding, hugepage setup, CPU isolation. Essentially what VyOS's `vpp.py` conf_mode script does (~900 LOC Python, equivalent in Go).

**What users get:** 10-35x forwarding performance with zero changes to their BGP/OSPF config. The BGP session still uses ze, routes still flow through ze's RIB, but packets are forwarded by VPP instead of the kernel.

**Effort:** One new component (`internal/component/vpp/`), one YANG module, ~1500 LOC Go. No changes to existing code.

#### Strategy 2: VPP-native interface backend (medium effort)

Add a `ifacevpp` backend that talks to VPP's binary API via GoVPP, bypassing the kernel entirely for interface configuration.

- Interface create/delete/address/MTU goes directly to VPP API
- LCP plugin still creates Linux mirrors for the control plane
- Ze can create VPP-native objects: bridge domains, L2XC, VXLAN tunnels, bonds
- `fib-kernel` still works (kernel routes synced to VPP via netlink plugin)

**What ze adds beyond Strategy 1:** Direct VPP interface control, VPP-specific features (bridge domains, L2XC, VXLAN tunnels, policers, ACLs) exposed through YANG config. Ze becomes the single configuration authority for both control plane and data plane.

**What users get:** Everything from Strategy 1, plus L2VPN services, IXP gateway functionality, traffic policers, all configured through ze's CLI/YANG/web UI.

**Effort:** New backend implementing ~30 `iface.Backend` methods via GoVPP, new YANG modules for VPP-specific features, ~4000-6000 LOC Go. Moderate changes to config pipeline.

#### Strategy 3: VPP FIB direct programming (highest effort, maximum performance)

Replace `fib-kernel` with `fib-vpp` that programs VPP's FIB directly via binary API, bypassing the kernel routing table entirely.

- Ze's BGP RIB decisions go straight to VPP FIB
- No netlink intermediary, no kernel route table
- Enables VPP-specific FIB features: MPLS label push/swap/pop, SRv6 policy, per-prefix counters
- LCP still needed for control-plane connectivity (BGP TCP sessions)

**What ze adds:** Direct FIB programming eliminates the kernel-as-intermediary. IPng loads ~175K routes/sec via netlink; direct API could be significantly faster. MPLS label operations become first-class.

**What users get:** Fastest possible convergence (no netlink batching delay), native MPLS/SRv6 support, per-prefix statistics from VPP.

**Effort:** New FIB backend, MPLS/SRv6 YANG extensions, GoVPP integration for FIB/MPLS/SRv6 APIs, ~6000-10000 LOC Go. Requires careful design of the RIB-to-VPP interface.

---

## Recommendation

**Start with Strategy 1, design for Strategy 2.**

Strategy 1 delivers the headline value (VPP forwarding performance) with minimal risk. Ze manages VPP as infrastructure: generates startup.conf, binds NICs, starts/stops VPP, monitors health. All existing ze code paths remain unchanged. Users get a production-proven architecture (IPng has run this for 4+ years with zero crashes).

Design the YANG config so Strategy 2 features (bridge domains, VXLAN, policers) can be added incrementally without breaking the config model. The `iface.Backend` interface is already the right abstraction boundary.

Strategy 3 is where ze could differentiate from VyOS and IPng's vppcfg: direct FIB programming from ze's BGP RIB, native MPLS label operations, per-prefix counters. But it requires a stable Strategy 1/2 foundation first.

### Key components to build

| Component | Purpose | Approximate size |
|-----------|---------|-----------------|
| `internal/component/vpp/` | VPP lifecycle management | ~800 LOC |
| `ze-vpp-conf.yang` | YANG model for VPP settings | ~200 lines |
| VPP startup.conf generator | Template from YANG config | ~300 LOC |
| NIC driver manager | DPDK bind/unbind, PCI state | ~400 LOC |
| GoVPP integration | Binary API client wrapper | ~500 LOC (Strategy 2: ~3000) |
| `fib-vpp` plugin | Direct FIB programming | ~2000 LOC (Strategy 3 only) |

### What ze would uniquely offer vs VyOS

| Feature | VyOS | Ze with VPP |
|---------|------|-------------|
| BGP implementation | FRR (external) | Ze's own (internal, zero-copy wire format) |
| Config model | XML + Python scripts | YANG-native, same model for CLI/web/API |
| FIB programming | Kernel netlink (indirect) | Could do direct VPP API (Strategy 3) |
| Plugin extensibility | Monolithic | Plugin registry, EventBus pub/sub |
| L2VPN config | Manual VPP CLI / vppcfg | Integrated YANG config with validation |
| Wire encoding | N/A (FRR handles it) | Buffer-first, zero-copy, pool dedup |

### Risks and constraints

- **GoVPP maturity:** fd.io/govpp is the official Go client. It works but has less community than vpp_papi (Python). VyOS chose Python; ze would use Go.
- **VPP version coupling:** VPP's binary API changes between releases. GoVPP handles this via binapi generation, but ze would need to track VPP releases.
- **LCP plugin stability:** IPng maintains their own fork (lcpng) with fixes not yet upstream. Ze would need to decide: upstream LCP (more stable, fewer features) or lcpng (production-tested, single maintainer).
- **Hardware matrix:** DPDK supports many NICs but not all. This becomes a support burden.
- **Hugepage/IOMMU requirements:** VPP needs system-level configuration that ze currently doesn't manage. Strategy 1 handles this as part of VPP lifecycle.

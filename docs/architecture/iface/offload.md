# Interface Offload and Packet Steering

Per-interface ethtool offloads (GRO, GSO, SG, TSO, LRO, hw-tc-offload) and
software packet steering (RPS, RFS) are one YANG container on the
`interface-l2` grouping, which covers ethernet, dummy, veth and bridge.

<!-- source: internal/component/iface/yang/ze-iface-conf.yang -- offload container in interface-l2 -->
<!-- source: internal/component/iface/offload_linux.go -- applyOffloads, setEthtoolFeature, applyRFSGlobal -->
<!-- source: internal/component/iface/offload_other.go -- non-Linux stub -->

## Absence means "keep the OS default"

Each leaf is a three-state boolean: true, false, or absent. VyOS uses an empty
leaf where presence enables and absence disables, which needs a boot-time
activation script to mirror the kernel defaults into config. ze targets gokrazy
appliances, which run no systemd and no activation script, so an absent leaf
makes no kernel call at all. This matches the existing `*bool` pointer pattern
used for the ipv4 and ipv6 sysctl leaves.

## Three kernel interfaces behind one container

| Feature | Kernel interface |
|---------|------------------|
| GRO, GSO, SG, TSO, LRO | one legacy ethtool ioctl each (`ethtool_value`: cmd plus data) |
| hw-tc-offload | the generic `ETHTOOL_SFEATURES` API (string-set lookup, then bit manipulation) |
| RPS, RFS | sysfs writes |

hw-tc-offload has no legacy ioctl, so it pays for the variable-length
string-set machinery. The other five do not, and use the simpler call.

The offloads bypass the `Backend` interface deliberately. `Backend` dispatches
between netlink and VPP. An ethtool ioctl (`SIOCETHTOOL`) is a separate kernel
interface, the same one the ring-buffer tuning code in
`internal/component/host` already uses, and the VPP data plane has its own
offload mechanism that would need its own implementation.

Bypassing `Backend` does not bypass the hardware selector. `applyOffloads` takes
a device name. Its one caller is the apply's per-entry loop. That loop resolves
the entry once and hands the same kernel device to every call. An entry bound by
`mac/match` or aliased by `os-name` therefore gets its offloads on the device
the selector names, as it gets its MTU.

<!-- source: internal/component/iface/config_apply.go -- deviceFor, the Phase 2 entry loop -->

## Two ordering and lifetime constraints

- `rps_sock_flow_entries` is a system-wide proc entry. Writing it per interface
  makes it toggle while one interface disables it and the next enables it.
  `applyRFSGlobal` scans every L2 interface and writes the global value once.
  The per-queue `rps_flow_cnt` stays per interface.
- One AF_INET datagram socket is opened for the whole apply and its descriptor
  is passed to each feature call. Opening one socket per ioctl costs five or
  six opens per interface.

## Kernel struct traps

- `ethtool_sset_info.sset_mask` is a `__u64`, not a `__u32`. A `uint32` plus
  padding happens to work on a little-endian host and is structurally wrong.
- `ETHTOOL_GSSET_INFO` returns the feature count in a variable-length `data[]`
  array at offset 16. The count is capped at 4096: an uncapped count from a
  faulty driver sizes the following `GSTRINGS` allocation at 32 bytes per
  entry and can exhaust memory.

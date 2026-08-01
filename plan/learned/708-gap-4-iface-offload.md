# 708 -- gap-4-iface-offload

Spec: spec-gap-4-iface-offload.md
Date: 2026-05-15

## What this was

Per-interface offload and packet steering configuration for L2 interfaces.
Ze had no way to control ethtool offloads (GRO, GSO, SG, TSO, LRO,
hw-tc-offload) or software packet steering (RPS, RFS). VyOS supports these
as presence flags on ethernet interfaces. Ze implements them as boolean
three-state leaves (true/false/absent) on the interface-l2 YANG grouping,
covering ethernet, dummy, veth, and bridge.

## Architecture decisions

1. **Boolean three-state over VyOS presence model.** VyOS uses empty leaves
   where presence = enable and absence = disable, requiring a bootstrap
   activation script to mirror kernel defaults into config at first boot. Ze
   targets gokrazy appliances (no systemd, no activation scripts), so absence
   means "preserve OS default" (no kernel call). This matches the existing
   ipv4Sysctl/ipv6Sysctl `*bool` pointer pattern in config.go.

2. **Direct kernel ioctl, not Backend method.** Offloads bypass the Backend
   interface. The Backend abstraction is for netlink/vpp dispatch. Ethtool
   ioctls are a separate kernel interface (SIOCETHTOOL), same as the existing
   system tuning ring buffer code in host/ethtool_linux.go. The VPP backend
   has its own offload mechanism that would need separate implementation.

3. **Legacy per-feature ioctls for 5 features, SFEATURES API for 1.**
   GRO/GSO/SG/TSO/LRO each have a simple legacy ioctl (ethtool_value struct
   with cmd + data). hw-tc-offload has no legacy ioctl and requires the
   generic ETHTOOL_SFEATURES API (string-set lookup + bit manipulation).
   The legacy ioctls are simpler and avoid the variable-length string-set
   machinery for the common features.

4. **RFS global sysfs written once, not per-interface.** rps_sock_flow_entries
   is a system-wide proc entry. Writing it per-interface causes transient
   toggling when one interface disables and another enables. applyRFSGlobal
   scans all L2 interfaces to determine the global value once. Per-queue
   rps_flow_cnt is still written per-interface.

5. **Single socket FD for all ethtool calls.** Opening a socket per ioctl is
   wasteful when applying 5-6 features per interface. applyOffloads opens one
   AF_INET DGRAM socket and passes the fd to all setEthtoolFeature calls.

## What was surprising

- VyOS supports 8 offload modes, not just the 4 (gro/gso/sg/tso) commonly
  listed. LRO and hw-tc-offload are ethtool features; RPS and RFS are sysfs
  knobs, not ethtool at all. The three different kernel interfaces (legacy
  ioctl, SFEATURES ioctl, sysfs) are unified under one YANG container.

- The ethtool_sset_info kernel struct uses __u64 for sset_mask, not __u32.
  An earlier version had uint32 + padding which happened to work on
  little-endian but was structurally wrong.

- The ETHTOOL_GSSET_INFO ioctl returns the feature count in a variable-length
  data[] array at offset 16. Without capping this count, a buggy driver could
  trigger an OOM via the subsequent GSTRINGS allocation (count * 32 bytes).
  Capped at 4096.

## Files

- `internal/component/iface/yang/ze-iface-conf.yang` -- offload container in interface-l2
- `internal/component/iface/config.go` -- offloadConfig struct, parseOffloadConfig, applyConfig wiring
- `internal/component/iface/offload_linux.go` -- ethtool ioctls, sysfs writes (new)
- `internal/component/iface/offload_other.go` -- non-Linux stub (new)
- `internal/component/iface/config_test.go` -- 8 unit tests
- `test/parse/iface-offload-*.ci` -- 3 functional parse tests
- `docs/features.md` -- interface feature list updated
- `docs/guide/configuration.md` -- offload config syntax documented

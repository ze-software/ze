---
spec: dataplane-compatibility-guard
topic: Config
feature: commit-time-backend-capability-check
---

## Before

A configuration could select VPP as the interface data plane while also enabling
kernel-only features such as `fib kernel` (alone), IKEv2, L2TP, flow export,
policy-based routing, GTSM, WireGuard without the VPP WireGuard plugin, VPP
firewall or traffic control alongside kernel interfaces, or firewall table
producers such as IRR, FlowSpec, CoPP, DDoS, and anomaly shaping on a VPP
firewall. `ze config validate`, editor commits, reloads, and daemon startup
accepted these combinations, and the router only failed later at apply time or
silently programmed the Linux kernel instead of VPP. In addition, the `ze:backend`
compatibility walk was split across four per-component callers, static named-table
routes could fall back to Linux tables on a VPP host, and the backend selectors
for interfaces, firewall, and traffic control used implementation names rather
than the data plane they program.

## After

Ze now evaluates data-plane compatibility with one shared verdict across
`ze config validate`, editor commits, SSH config uploads, live reloads, daemon
startup, and `ze doctor` (`doctor-dataplane-mismatch`). Configurations that mix
a VPP interface plane with kernel-only features, mix a VPP firewall or traffic
control backend with kernel interfaces, enable `fib kernel` without a companion
`fib vpp` block on VPP, enable kernel-only firewall producers on a VPP firewall,
or enable VPP WireGuard without the VPP WireGuard plugin are refused before
commit or startup with the exact command to fix them. Host-side `sysctl` and
dual `fib kernel` + `fib vpp` remain accepted on VPP so mixed host and VPP
setups continue to work. Live reloads that switch `interface backend` while
features of the previous plane remain configured are refused cleanly without
changing the running configuration.

Every data-plane selector now names the plane it programs (`kernel` or `vpp`):
`interface backend`, `firewall backend`, and `traffic control backend` default to `kernel`, and
the previous implementation names (`netlink`, `nft`, and `tc`) are refused without aliasing.
Static named-table routes now program the active data plane directly without
falling back to the Linux kernel on VPP, and the data-plane features guide
publishes the generated compatibility matrix from live feature declarations. On
VPP firewalls, `connection-state` and `mark-set` are decided by the VPP firewall
verifier rather than a static schema annotation, NAT44 `snat`/`masquerade`
source-address matches are refused at validation time, and `flowtable` remains
kernel-only.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none

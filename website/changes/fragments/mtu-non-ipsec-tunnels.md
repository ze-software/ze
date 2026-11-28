---
spec: mtu-non-ipsec-tunnels
topic: Diagnostics
feature: path-mtu-diagnostic-for-ipsec-tunnels
---

## Before

`show mtu` discovered and sized only IPsec tunnels from the IKE engine inventory.
GRE, GRETap, IP6GRE, IP6GRETap, VXLAN, WireGuard, IPIP, SIT, IP6Tnl, and IPIP6
interfaces received no path probes, no overhead calculation, and no MTU commands.
Builds without the IKE engine produced no run verdict even when non-IPsec tunnels existed.

## After

`show mtu` now discovers and sizes all non-IPsec tunnel and WireGuard interfaces
alongside IPsec XFRM tunnels. Live tunnel readback on both the kernel and VPP
backends supplies exact encapsulation flags, including GRE options and RFC 2473
IPv6 encapsulation limits. `show mtu` probes unique unicast remotes once, computes
exact ceilings and recommendations through one unified overhead pipeline, and emits
`set interface tunnel` and `set interface wireguard` commands. Multi-peer WireGuard
interfaces size to the tightest peer ceiling. Builds without the IKE engine still
probe, size, and advise on non-IPsec tunnels.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none

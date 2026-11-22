---
spec: srv6-df-clear-fragmentation
topic: Kernel
feature: kernel-routes
---

## Before

On Linux kernels with SRv6 seg6 L3 encapsulation routes, a forwarded IPv4 packet with the Don't Fragment bit clear whose length exceeded the egress link or route MTU minus the SRv6 encapsulation overhead (for example, larger than 1436 bytes on a 1500-byte link with a single-SID SRH) was encapsulated past the egress MTU and silently dropped by IPv6 output fragmentation instead of being fragmented or reported to the sender. `ze doctor` did not warn when a Linux host lacked headend DF-clear IPv4 fragmentation support.

## After

Ze's runtime Linux kernel fragments oversized DF-clear IPv4 packets at the inner IPv4 layer before SRv6 encapsulation (preserving RFC 791 IPv4 option rules across fragments and dropping sub-quantum MTUs cleanly without livelock), emitting unfragmented outer IPv6+SRH packets that fit within the egress MTU and decapsulate cleanly on Linux `seg6_local`, VPP, and hardware SRv6 egress routers. On hosts running stock Linux kernels without this capability, `ze doctor` now reports a non-blocking warning (`doctor-srv6-forward-frag-unavailable` or `doctor-srv6-forward-frag-unknown`) when BGP is configured on the kernel FIB.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none

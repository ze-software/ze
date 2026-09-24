# SRv6 FIB Programming

Both FIB backends read the `SRv6SID` field of a best-change entry and program
SRv6 steering. The kernel backend builds a SEG6 encapsulation; the VPP backend
adds an SR steering policy.

## Where the SID comes from

The SID reaches the FIB through the BGP **Prefix-SID attribute**, not through a
dedicated SRv6 NLRI family. The RIB pool extracts the SID from the attribute and
the RIB manager writes it onto the selected Loc-RIB path. The route-install RPC
and sysrib's live and replay batches preserve it.

<!-- source: internal/component/bgp/plugins/rib/pool/srv6sid.go -- ExtractSRv6SID, ExtractSRv6SIDFull -->

This matters because an SRv6 NLRI family is often assumed to be the prerequisite
for SRv6 forwarding. It is not. Any change that only needs `SRv6SID` populated
can proceed on the Prefix-SID path alone.

## Kernel backend

`buildSEG6Encap` builds the encapsulation and `buildRichRoute` reaches it. The
call is gated on a valid IPv6 SID on a unicast route. A Service SID selects
IPv6 encapsulation ahead of any label field, which can contain transposed SID
bits. Without a SID, the route uses MPLS when labels are present, or plain IP.
For ECMP, each next hop carries its own encapsulation rather than inheriting a
route-level label stack. A Service SID selects SEG6 on every member; without
one, a member uses its own labels or remains plain IP.

An IPv4 destination can have an IPv6 gateway after recursive resolution.
`buildRichRoute` encodes that gateway as `RTA_VIA`; a same-family gateway uses
`RTA_GATEWAY`. Each ECMP member selects its gateway attribute independently and
retains its device, on-link flag, weight and encapsulation. IPv6 destinations
with IPv4 gateways are rejected. An unusable member rejects the route update,
so the kernel never receives a silently reduced group.

<!-- source: internal/plugins/fib/kernel/nexthop_linux.go -- buildSEG6Encap, buildRichRoute -->

## VPP backend

`processSRv6Change` dispatches on the route verb. Install and replace call
`addSRv6Steer` (an `sr.SrSteeringAddDel` call carrying the SID as the BSID);
remove calls `delSRv6Steer`. The backend tracks installed prefixes so a removal
of a prefix it never installed is a no-op rather than an error. A change with no
SID is a no-op in the same verb switch.

<!-- source: internal/plugins/fib/vpp/srv6.go -- processSRv6Change, addSRv6Steer, delSRv6Steer -->

## Trap

A spec's "current behavior" section ages. The SRv6 backend was recorded as
blocked on an SRv6 NLRI family that it never needed, and as missing an
encapsulation path that already existed and was already tested. Re-derive the
input-to-FIB chain from the code before you trust a written status.

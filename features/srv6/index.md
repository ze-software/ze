# SRv6 (Segment Routing over IPv6)

<!-- source: internal/component/bgp/plugins/rib/pool/srv6sid.go -- SRv6 SID extraction and transposition -->
<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- SID lookup at best-path emission -->
<!-- source: internal/component/sysrib/sysrib.go -- SID resolvability check and FIB emission -->
<!-- source: internal/plugins/fib/kernel/nexthop_linux.go -- Linux SEG6 encap -->
<!-- source: internal/plugins/fib/vpp/srv6.go -- VPP SR steer -->
<!-- source: internal/component/bgp/message/rfc7606.go -- PrefixSID attribute validation -->
<!-- source: internal/component/bgp/reactor/session_validation.go -- EBGP PrefixSID filtering -->
<!-- rfc: rfc/short/rfc8669.md -- BGP Prefix-SID attribute (code 40) -->
<!-- rfc: rfc/short/rfc9252.md -- SRv6 overlay services -->

Ze receives BGP routes carrying SRv6 Prefix-SID attributes (RFC 8669, RFC 9252),
extracts SRv6 SIDs, validates them, and programs encapsulation into the FIB.
Operates as an ingress PE: consumes received SRv6 SIDs but does not originate them.

| Feature | Description |
|---------|-------------|
| Attribute parsing | PrefixSID (code 40) stored as opaque bytes; SRv6 SID extracted lazily at best-path time |
| L3/L2 Service TLVs | Types 5 (L3 Service) and 6 (L2 Service) per RFC 9252 Section 3.1 |
| SID extraction | First SRv6 SID Information Sub-TLV (type 1) within the Service TLV |
| Transposition | SID Structure Sub-Sub-TLV reconstructs full SID from NLRI label bits (VPN/EVPN) |
| Path ineligibility | Route with SRv6 TLVs but no valid SID excluded from best-path selection |
| SID resolvability | SRv6 SID must have a covering route in Loc-RIB before FIB installation |
| EBGP filtering | PrefixSID from EBGP peers discarded unless `accept-srv6-prefix-sid` is set |
| EBGP propagation | PrefixSID removed from every UPDATE sent to an EBGP peer unless `propagate-srv6-prefix-sid` is set |
| Validation | Malformed SRv6 Service TLVs trigger treat-as-withdraw (RFC 9252 Section 3.4) |
| Propagation | PrefixSID preserved on zero-copy forward; stripped when the next-hop changes, and stripped at the SR domain boundary |
| Linux FIB | SEG6 lwtunnel encap via netlink |
| VPP FIB | SR steering policy via GoVPP `sr_steering_add_del` |

## Configuration

EBGP peers require explicit opt-in, in each direction. IBGP peers accept and
advertise PrefixSID by default.

```
bgp {
    peer pe1 {
        session {
            accept-srv6-prefix-sid true
            propagate-srv6-prefix-sid true
        }
    }
}
```

| Option | Location | Default | Description |
|--------|----------|---------|-------------|
| `accept-srv6-prefix-sid` | `bgp/peer/session` | `false` | Accept PrefixSID attribute from this EBGP peer (RFC 8669 Section 4) |
| `propagate-srv6-prefix-sid` | `bgp/peer/session` | `false` | Advertise PrefixSID attribute to this EBGP peer (RFC 8669 Section 8) |

Both leaves say the same thing about one neighbor: it is inside ze's SR domain.
RFC 8669 Section 8 puts the boundary at "a single SR/administrative domain that
may include one or more ASes", so the boundary is not the AS boundary and ze
cannot derive it from the ASN pair. Set both leaves on an EBGP neighbor that is
part of the same SR domain, and leave both unset on every other EBGP neighbor.
Set neither on an IBGP peer: the section governs propagation to other ASes, so
it does not reach a peer in this one.

No additional configuration is needed for IBGP sessions or for FIB programming.
When an SRv6 SID is present on a best-path route and the SID is resolvable,
the FIB backend programs the encapsulation automatically.

## Data Flow

```
BGP UPDATE with PrefixSID (attr 40)
  |
  v
Wire parser: stores PrefixSID in OtherAttrs (opaque bytes)
  |
  v
RFC 7606 validator: checks TLV structure, rejects malformed
  |
  v
EBGP filter: discards attr 40 unless accept-srv6-prefix-sid is set
  |
  v
RIB best-path: IsSRv6Ineligible() excludes routes with broken SRv6 TLVs
  |
  v
Best-path emission: lookupSRv6SIDForBest() extracts SID from OtherAttrs
  |  For VPN/EVPN: applies transposition (label bits -> SID function)
  |
  v
sysrib: stores srv6SID on protocolRoute, tracks SID for resolution
  |  Suppresses FIB emission if SID has no covering route in Loc-RIB
  |  Cascade re-evaluates when SID reachability changes
  |
  v
FIB backend:
  Linux: netlink.SEG6Encap{Mode: encap, Segments: [SID]}
  VPP:   sr.SrSteeringAddDel{BsidAddr: SID, TrafficType: IPv4/IPv6}
```

The egress side is a separate decision, taken once per destination peer:

```
Route selected for a destination peer
  |
  v
prefixSIDAllowedTo(isIBGP, propagate-srv6-prefix-sid)
  |  true  -> attr 40 goes out unchanged
  |  false -> attr 40 is removed for this peer alone
  v
Forward rails:     applyFactsPrefixSID records an attribute suppression
Origination rails: the configured PrefixSID and any raw attribute 40 are dropped
```

## Transposition (VPN/EVPN)

For SAFI 128 (VPN) and SAFI 70 (EVPN), RFC 9252 Section 3.2.1 specifies that
part of the SRv6 SID function bits are encoded in the MPLS label field of the
NLRI instead of in the SID Information Sub-TLV. Ze reconstructs the full SID:

1. Extract partial SID from SID Information Sub-TLV
2. Read transposition parameters from SID Structure Sub-Sub-TLV (offset, length)
3. Read label from NLRI (stored as side-data in PeerRIB)
4. OR label bits into SID at the specified bit offset

When Transposition Length is 0, no reconstruction is needed (the full SID is
in the Sub-TLV). This is the common case for IPv6 unicast.

## FIB Backends

### Linux Kernel

Routes with SRv6 SID are installed with SEG6 lightweight tunnel encapsulation:

```
ip route add <prefix> via <nexthop> encap seg6 mode encap segs <SID>
```

Implemented via `netlink.SEG6Encap` in `buildRichRoute`. MPLS labels take
precedence: if both Labels and SRv6SID are present, MPLS encap is used.

### VPP

Routes with SRv6 SID are steered via VPP's SR policy infrastructure:

```
sr_steering_add_del bsid=<SID> prefix=<prefix> traffic_type=IPv4|IPv6
```

Implemented via GoVPP `sr.SrSteeringAddDel`. Requires `go.fd.io/govpp/binapi/sr`
(vendored). SRv6 steering takes precedence over both MPLS and plain routes in
the VPP dispatch logic.

## RFC Compliance

### RFC 8669 (Prefix-SID Attribute)

| Requirement | Section | Status |
|-------------|---------|--------|
| Attribute code 40, optional transitive | 3 | Implemented |
| TLV format: 1B type + 2B length | 3 | Implemented |
| Unknown TLVs preserved on propagation | 3 | Implemented (opaque forwarding) |
| EBGP: discard unless configured to accept | 4 | Implemented (`accept-srv6-prefix-sid`) |
| Propagation to other ASes explicitly configured | 8 | Implemented (`propagate-srv6-prefix-sid`) |
| Malformed attribute: attribute-discard | 6 | Implemented (RFC 7606 validator) |

### RFC 9252 (SRv6 Overlay Services)

| Requirement | Section | Status |
|-------------|---------|--------|
| L3 Service TLV (type 5) | 3.1 | Implemented |
| L2 Service TLV (type 6) | 3.1 | Implemented |
| SID Information Sub-TLV (type 1) | 3.2 | Implemented |
| First SID Sub-TLV preferred | 3.2 SHOULD | Implemented |
| SID Structure Sub-Sub-TLV | 3.2.1 | Implemented |
| Transposition reconstruction | 3.2.1 | Implemented (VPN/EVPN) |
| LBL+LNL+FL+AL <= 128 validation | 3.2.1 | Implemented (errata 7817) |
| NH unchanged: preserve TLVs | 3.3 | Implemented (zero-copy forward) |
| NH changed: strip PrefixSID | 3.3 | Implemented (AttrModSuppress) |
| Malformed Service TLV: treat-as-withdraw | 3.4 | Implemented |
| Path ineligibility (no valid SID) | 5 | Implemented |
| SID resolvability check | 5 | Implemented (Loc-RIB LPM) |

## Limitations

- **Ingress PE only.** Ze consumes SRv6 SIDs from received routes. It does not
  allocate or advertise local SRv6 SIDs. When re-advertising with a changed
  next-hop, PrefixSID is stripped.
- **No SRv6 capability negotiation.** PrefixSID is optional-transitive, so it
  propagates without negotiation. Ze does not signal SRv6 support via capabilities.
- **MPLS precedence.** If a route carries both MPLS labels and an SRv6 SID,
  MPLS encap is used (kernel backend). VPP dispatches SRv6 first.
- **No SRv6 policy.** Ze programs single-SID encapsulation. SRv6 segment lists
  (multi-hop SR paths) are not supported.

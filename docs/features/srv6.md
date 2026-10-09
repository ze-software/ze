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
extracts SRv6 SIDs, validates them, and programs Linux or VPP ingress
encapsulation. VPP creates an encapsulation policy before prefix steering,
using a distinct local binding SID (BSID), not the advertised egress Service SID.
Static route configuration and `update text` can advertise an explicit Service
SID. Ze does not allocate egress Service SIDs or install endpoint behaviors.
<!-- source: internal/plugins/fib/vpp/srv6.go -- acquirePolicy, addSRv6Steer -->

| Feature | Description |
|---------|-------------|
| Attribute parsing | PrefixSID (code 40) stored as opaque bytes; SRv6 SID extracted lazily at best-path time |
| L3/L2 Service TLVs | Types 5 (L3 Service) and 6 (L2 Service) per RFC 9252 Section 3.1 |
| SID extraction | First SRv6 SID Information Sub-TLV (type 1) within the Service TLV |
| Transposition | SID Structure Sub-Sub-TLV reconstructs full SID from NLRI label bits (VPN/EVPN) |
| Path ineligibility | Partial: excludes paths with no extractable SID, but invalid SID Structure parameters can still pass best-path admission |
| SID resolvability | SRv6 SID must have a covering route in Loc-RIB before FIB installation |
| EBGP filtering | PrefixSID from EBGP peers discarded unless `accept-srv6-prefix-sid` is set |
| EBGP propagation | PrefixSID removed on every rail that writes an UPDATE unless `propagate-srv6-prefix-sid` is set: the two forward rails, the two origination rails, and the API/readvertise announce rail |
| Validation | Malformed SRv6 Service TLVs trigger treat-as-withdraw (RFC 9252 Section 3.4) |
| Propagation | Label-Index Reserved and Flags are cleared on every transmission, including relay. Originator SRGB and unknown TLVs remain byte-identical. A next-hop change removes only the SRv6 Service TLVs (types 5 and 6), with the whole attribute leaving if no TLV remains; the SR domain boundary strips it whole |
| Linux FIB | SEG6 lwtunnel encap via netlink |
| VPP FIB | Single-Service-SID encapsulation policy, distinct local BSID, and per-prefix/table steering; shared policies are removed after their last reference |

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

No additional configuration is needed for IBGP sessions or Linux FIB
programming. A resolvable best-path Service SID selects ingress encapsulation.
VPP still requires an IPv6 underlay route, encapsulation source, and the
relevant tables. Its configured backend table selects policy and outer IPv6
lookup; a nonzero per-route table override selects destination steering only.

VPP ownership is durable in daemon state. Restore checks confirmed policy
contents and steering references before mutation; foreign or ambiguous live
resources fail closed rather than being adopted or deleted. Managed-VPP
reconnect requests replay, but partial replay is not a complete snapshot and
does not withdraw omitted live routes. External-VPP process restart does not
provide that reconnect notification. See the [VPP guide](../guide/vpp.md)
and [runtime proof contract](../architecture/testing/interop.md#real-vpp-srv6-service-route-proof).
These implementation details are not evidence of an executed forwarding proof.
<!-- source: internal/plugins/fib/vpp/srv6.go -- addSRv6Steer, acquirePolicy, releasePolicy -->
<!-- source: internal/plugins/fib/vpp/srv6_state.go -- restore -->
<!-- source: internal/plugins/fib/vpp/register.go -- runFibVPPPlugin -->
<!-- source: internal/component/vpp/vpp.go -- runOnce -->

Durable admission is bounded by 4,096 ownership keys across policies, steering,
and temporary IP/SRv6 transition records. New growth is refused before mutation
when it cannot fit; existing-key replay and withdrawal remain available.
Ordinary-route cleanup ownership survives restart as confirmed installation
history, not adoption of matching live routes. VPP cannot expose every hidden
API-source route through its winning-source dump, so external writers must not
replace those owned entries.
<!-- source: internal/plugins/fib/vpp/srv6_fallback.go -- reserveState, checkpointSRv6Fallback, srv6Fallback -->
<!-- source: internal/plugins/fib/vpp/srv6.go -- restoreSRv6 -->
<!-- source: pkg/plugin/rpc/state.go -- StateListMax -->

### Explicit Service SID advertisement

The `bgp-prefix-sid-srv6` route attribute accepts an IPv6 Service SID, an optional
endpoint behavior, and an optional SID structure:

```
bgp-prefix-sid-srv6 ( l3-service 2001:db8:1:2:: 0x13 [64,0,32,0,16,64] )
```

The six structure fields are Locator Block Length, Locator Node Length, Function
Length, Argument Length, Transposition Length, and Transposition Offset, in bits.
The example advertises End.DT4 with 16 function bits supplied separately in the
NLRI label field. Those bits must already be zero in the configured SID; Ze
rejects a conflicting SID rather than discarding its bits. A zero Transposition
Length requires a zero offset, and the structure must fit within 128 bits.

Argument Length must be zero except for End.DT2M (`0x18`), whose Arg.FE2 carries
the egress Ethernet-segment filtering argument. An endpoint behavior unknown to
the encoder can be advertised without arguments. This is explicit signaling,
not local endpoint installation: the configured SID must belong to an endpoint
provisioned outside this ingress FIB path.

### Per-neighbor service export

Use a peer's export chain to control which SRv6 services it receives. Match the
service's Route Target with `community-match`; rejecting a match withholds the
whole route, not just its Prefix-SID attribute:

```
bgp {
    policy {
        community-match SERVICE-20 {
            entry target:65000:10 { type extended; action reject; }
            entry target:65000:20 { type extended; action accept; }
        }
    }
    peer pe1 {
        filter { export [ community-match:SERVICE-20 ]; }
    }
}
```

This peer receives service 20 but not service 10. Other peers can use different
lists or no filter. Community-match lists deny unmatched routes, so include
each service the peer should receive. This route-level control is separate from
`propagate-srv6-prefix-sid`, which only controls the attribute at an EBGP SR
domain boundary.

<!-- source: internal/component/bgp/plugins/filter_community_match/match.go -- Route Target matching and implicit deny -->
<!-- source: internal/component/bgp/reactor/session_write.go -- originated UPDATE export gate -->

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
RIB best-path: isSRv6Ineligible() excludes SRv6 paths with no extractable SID
  |
  v
Best-path emission: storedPathSRv6SID() extracts SID from the winner's OtherAttrs
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
  |  true  -> attr 40 is retained, with Label-Index transmit fields cleared
  |  false -> attr 40 is removed for this peer alone
  v
Forward rails:     applyFactsPrefixSID records an attribute suppression
Origination rails: the configured PrefixSID and any raw attribute 40 are dropped
```

The final session writers clear the Label-Index Reserved octet and both Flags
octets in their private outgoing buffer after export policy (RFC 8669 Section
3.1). This applies to forwarded routes as well as originated routes, without an
extra buffer or a change to the borrowed received UPDATE. The Section 3.2
unchanged-propagation rule applies specifically to Originator SRGB: its bytes
are never normalized on relay.
<!-- source: internal/component/bgp/reactor/session_prefix_sid.go -- clearTransmittedLabelIndex -->

Forwarding compares the received next-hop entity with the effective address
after export policy, configured rewriting and Link-Local scope normalization.
An explicit third-party A-to-A rewrite keeps the Service TLVs, including their
Reserved octets and unknown nested fields. A policy-only A-to-B rewrite removes
them even with next-hop `auto` or `unchanged`, including raw-policy export and
the route-server export fallback. Only the route's actual legacy or MP field
participates: unused companion rewrite operations do not imply a change.
IPv4-mapped and native IPv4 forms identify the same address, as do a Global
address with or without its optional Link-Local half. For the VPN exception
whose Global is unspecified, the Link-Local is the entity.
RFC 9252 service carriers use MP_REACH. In a mixed UPDATE its MP next hop governs
the Service TLVs; a legacy sibling's independent NEXT_HOP cannot change that
decision. Legacy-only input retains the same propagation safeguard, without
implying support for originating SRv6 services on a legacy carrier.
When a raw export result has an empty or undecodable MP next-hop field and a
configured rewrite repairs it before admission, the repaired address still
governs this comparison. The MP carrier did not disappear with its intermediate
address, and policy is not run again.
<!-- source: internal/component/bgp/reactor/forward_prefix_sid.go -- applyEgressPrefixSIDNextHop -->

<!-- source: internal/component/bgp/reactor/forward_prefix_sid.go -- prefixSIDAllowedTo -->

### The API and readvertise announce rail

The fifth rail asks the same question at the same site.
`buildBatchAnnounceUpdate` (`internal/component/bgp/reactor/reactor_api_batch.go`)
copies the caller's attribute block verbatim, so "say nothing" would emit
attribute 40; it drops the code when `prefixSIDAllowedTo` answers no, beside the
RFC 4271 Section 5.1.5 LOCAL_PREF drop. The destination's leaf joins
`announceBuildKey`, so two peers of one update group that answer differently no
longer share a built UPDATE. The rail carries an API announce, a grouped
announce, and the RFC 9494 stale readvertise (`sendStaleReadvertise`), which
replays a stored received block.

<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- buildBatchAnnounceUpdate -->


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

Implemented via `netlink.SEG6Encap` in `buildRichRoute`. A valid SRv6 Service SID
selects IPv6 encapsulation even when the NLRI carries labels. Those fields can
contain transposed SID bits and are not MPLS forwarding labels.

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
| Propagation to other ASes explicitly configured | 8 | Implemented (`propagate-srv6-prefix-sid`): every rail that writes an UPDATE asks |
| Malformed attribute: attribute-discard | 6 | Partial: a TLV overrunning the attribute and trailing octets are discarded; a TLV length outside its type's constraint and an empty attribute are not refused yet (`plan/immediate/spec-bgp-prefix-sid-rfc-defects.md`, D1) |
| Repeated single-occurrence TLV: all but the first discarded | 6 | Implemented: Label-Index, SRv6 L3 and L2 Service TLVs, at ingest, so neither the RIB nor any relay sees the repeat |

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
| NH unchanged: preserve TLVs | 2 | Implemented: effective address identity, including explicit A-to-A; Reserved and unknown nested bytes are retained |
| NH changed: SRv6 Service TLVs removed, other TLVs kept | 2 | Implemented removal after policy and configured rewrites, including policy-only changes under `auto` or `unchanged`. Local SID allocation/rebuilding remains unimplemented |
| Malformed Service TLV: treat-as-withdraw | 7 | Partial: a Service TLV overrunning the attribute gets attribute-discard instead (spec D2) |
| Path ineligibility (no valid SID) | 7 | Partial: invalid SID Structure parameters and transposition widths can still pass best-path admission (spec D3, D4) |
| SID resolvability before best-path selection | 5 | Partial: sysrib blocks FIB installation without a resolvable SID; BGP pre-selection filtering is not implemented |

<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- isSRv6Ineligible, srv6SIDFromResult -->
<!-- source: internal/component/bgp/plugins/rib/pool/srv6sid.go -- ExtractSRv6SID, parseSIDStructure -->

## Limitations

- **SID reachability is a FIB guard, not a BGP selection filter.** Losing the
  covering route withdraws the installed service route; restoring reachability
  permits installation again. The guard runs after BGP selects a path, so it
  cannot choose a reachable alternative over an unreachable selected SID.

- **No local SID allocation or endpoint installation.** Ze can advertise an
  explicitly configured SID but does not provision its egress behavior. When
  re-advertising with a changed next-hop, the SRv6 Service TLVs are removed
  rather than rebuilt with a local SID; the rest of the PrefixSID is kept.
- **No SRv6 capability negotiation.** PrefixSID is optional-transitive, so it
  propagates without negotiation. Ze does not signal SRv6 support via capabilities.
- **No SRv6 policy.** Ze programs single-SID encapsulation. SRv6 segment lists
  (multi-hop SR paths) are not supported.

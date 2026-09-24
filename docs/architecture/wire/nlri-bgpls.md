# BGP-LS NLRI Wire Format (RFC 9552)

**Source:** ExaBGP `bgp/message/update/nlri/bgpls/`
**Family:** AFI 16388 (BGP-LS), SAFI 71 (bgp_ls) or 72 (bgp_ls_vpn)

<!-- source: internal/core/family/family.go -- AFIBGPLS, SAFIBGPLinkState, SAFIBGPLinkStateVPN -->
<!-- source: internal/component/bgp/plugins/nlri/ls/types.go -- BGPLSFamily, BGPLSVPNFamily -->

---

## Wire Format Overview

### Without VPN (SAFI 71)

```
+---------------------------+
|   NLRI Type (2 octets)    |  1=Node, 2=Link, 3/4=Prefix
+---------------------------+
|   Total Length (2 octets) |  Payload length
+---------------------------+
|   Protocol ID (1 octet)   |
+---------------------------+
|   Identifier (8 octets)   |
+---------------------------+
|   Descriptors (variable)  |  TLVs
+---------------------------+
```

### With VPN (SAFI 72)

```
+---------------------------+
|   NLRI Type (2 octets)    |
+---------------------------+
|   Total Length (2 octets) |  Includes RD
+---------------------------+
|   RD (8 octets)           |  Route Distinguisher
+---------------------------+
|   Protocol ID (1 octet)   |
+---------------------------+
|   Identifier (8 octets)   |
+---------------------------+
|   Descriptors (variable)  |
+---------------------------+
```

<!-- source: internal/component/bgp/plugins/nlri/ls/types_nlri.go -- BGP-LS wire format parsing -->

---

## NLRI Types

| Type | Name | Description |
|------|------|-------------|
| 1 | Node NLRI | Describes a node (router) |
| 2 | Link NLRI | Describes a link between nodes |
| 3 | IPv4 Prefix NLRI | IPv4 reachability |
| 4 | IPv6 Prefix NLRI | IPv6 reachability |
| 6 | SRv6 SID NLRI | Segment Routing v6 (RFC 9514) |

<!-- source: internal/component/bgp/plugins/nlri/ls/types.go -- BGPLSNLRIType constants -->

---

## Protocol IDs

| ID | Protocol |
|----|----------|
| 1 | IS-IS Level 1 |
| 2 | IS-IS Level 2 |
| 3 | OSPFv2 |
| 4 | Direct |
| 5 | Static |
| 6 | OSPFv3 |
| 7 | BGP Egress Peer Engineering |
| 227 | FreeRTR (non-standard) |

<!-- source: internal/component/bgp/plugins/nlri/ls/types.go -- BGPLSProtocolID, ProtoISISL1..ProtoOSPFv3 -->

---

## Node NLRI (Type 1)

### Wire Format

```
+---------------------------+
|   Type = 1 (2 octets)     |
+---------------------------+
|   Length (2 octets)       |
+---------------------------+
|   Protocol ID (1 octet)   |
+---------------------------+
|   Identifier (8 octets)   |
+---------------------------+
|   Local Node Descriptors  |  TLV container (Type 256)
+---------------------------+
```

### Node Descriptor TLVs

| Type | Name | Length |
|------|------|--------|
| 256 | Local Node Descriptors | Container |
| 512 | AS Number | 4 |
| 513 | BGP-LS Identifier | 4 |
| 514 | OSPF Area ID | 4 |
| 515 | IGP Router ID | Variable |
| 516 | BGP Router-ID | 4 |
| 517 | Confederation Member | 4 |

<!-- source: internal/component/bgp/plugins/nlri/ls/types_descriptor.go -- NodeDescriptor struct, TLV constants -->

---

## Link NLRI (Type 2)

### Wire Format

```
+---------------------------+
|   Type = 2 (2 octets)     |
+---------------------------+
|   Length (2 octets)       |
+---------------------------+
|   Protocol ID (1 octet)   |
+---------------------------+
|   Identifier (8 octets)   |
+---------------------------+
|   Local Node Descriptors  |  TLV Type 256
+---------------------------+
|   Remote Node Descriptors |  TLV Type 257
+---------------------------+
|   Link Descriptors        |  TLVs 258-263
+---------------------------+
```

### Link Descriptor TLVs

| Type | Name | Length |
|------|------|--------|
| 258 | Link Local/Remote Identifiers | 8 |
| 259 | IPv4 Interface Address | 4 |
| 260 | IPv4 Neighbor Address | 4 |
| 261 | IPv6 Interface Address | 16 |
| 262 | IPv6 Neighbor Address | 16 |
| 263 | Multi-Topology ID | 2 |

<!-- source: internal/component/bgp/plugins/nlri/ls/types_descriptor.go -- LinkDescriptor TLV constants -->

---

## Prefix NLRI (Types 3, 4)

### Wire Format

```
+---------------------------+
|   Type = 3/4 (2 octets)   |  3=IPv4, 4=IPv6
+---------------------------+
|   Length (2 octets)       |
+---------------------------+
|   Protocol ID (1 octet)   |
+---------------------------+
|   Identifier (8 octets)   |
+---------------------------+
|   Local Node Descriptors  |  TLV Type 256
+---------------------------+
|   Prefix Descriptors      |  TLVs 263-265
+---------------------------+
```

### Prefix Descriptor TLVs

| Type | Name | Length |
|------|------|--------|
| 263 | Multi-Topology ID | 2 |
| 264 | OSPF Route Type | 1 |
| 265 | IP Reachability Information | Variable |

<!-- source: internal/component/bgp/plugins/nlri/ls/types_descriptor.go -- prefix descriptor TLV constants -->

---

## TLV Format

All descriptors use TLV format:

```
+---------------------------+
|   Type (2 octets)         |
+---------------------------+
|   Length (2 octets)       |
+---------------------------+
|   Value (variable)        |
+---------------------------+
```

---

## ExaBGP Implementation

### Base Class

```python
@NLRI.register(AFI.bgpls, SAFI.bgp_ls)
@NLRI.register(AFI.bgpls, SAFI.bgp_ls_vpn)
class BGPLS(NLRI):
    CODE: ClassVar[int] = -1  # Set by subclass

    def __init__(self, addpath: PathInfo | None = None):
        NLRI.__init__(self, AFI.bgpls, SAFI.bgp_ls)
        self._packed = b''

    def pack_nlri(self, negotiated: Negotiated) -> Buffer:
        return self._packed  # [type:2][length:2][payload]

    @classmethod
    def unpack_nlri(cls, afi, safi, data, action, addpath, negotiated):
        code, length = unpack('!HH', data[:4])

        if safi == SAFI.bgp_ls_vpn:
            rd = RouteDistinguisher(data[4:12])
            payload = data[12:length+4]
        else:
            rd = RouteDistinguisher.NORD
            payload = data[4:length+4]

        if code in cls.registered_bgpls:
            return cls.registered_bgpls[code].unpack_bgpls_nlri(payload, rd)
        return GenericBGPLS(code, data[:length+4])
```

### Specific Types

```python
@BGPLS.register_bgpls
class NodeNLRI(BGPLS):
    CODE = 1
    NAME = 'Node'

    @classmethod
    def unpack_bgpls_nlri(cls, data, rd):
        # Parse protocol_id, identifier, descriptors
        ...

class LinkNLRI(BGPLS):
    CODE = 2
    NAME = 'Link'
```

---

## JSON Output

### Node NLRI

```json
{
  "code": 1,
  "parsed": true,
  "name": "Node",
  "protocol-id": 2,
  "identifier": "0x0000000000000001",
  "local-node": {
    "as-number": 65000,
    "router-id": "1.1.1.1"
  }
}
```

### Generic (Unknown)

```json
{
  "code": 99,
  "parsed": false,
  "raw": "006300..."
}
```

---

## Ze Implementation Notes

### Packed-Bytes-First Pattern

Store complete wire format including header:

```go
type BGPLS struct {
    packed []byte  // [type:2][length:2][payload...]
}

func (b *BGPLS) NLRIType() uint16 {
    return binary.BigEndian.Uint16(b.packed[0:2])
}

func (b *BGPLS) ProtocolID() uint8 {
    // Offset depends on VPN (RD present or not)
    return b.packed[b.payloadOffset()]
}
```

### TLV Parsing

```go
func parseTLVs(data []byte) ([]TLV, error) {
    var tlvs []TLV
    for len(data) >= 4 {
        typ := binary.BigEndian.Uint16(data[0:2])
        length := binary.BigEndian.Uint16(data[2:4])
        if len(data) < int(4+length) {
            return nil, ErrTruncated
        }
        tlvs = append(tlvs, TLV{
            Type:  typ,
            Value: data[4:4+length],
        })
        data = data[4+length:]
    }
    return tlvs, nil
}
```

### Type Registry

```go
var bgplsRegistry = map[uint16]BGPLSUnpacker{
    1: unpackNode,
    2: unpackLink,
    3: unpackPrefixV4,
    4: unpackPrefixV6,
    6: unpackSRv6SID,
}
```

### Consumer Decoding and Origination

The `bgp-nlri-ls` codec registers both families in decode mode.
`AttrTLVsToJSON` supplies the offline UPDATE decoder with the attribute
view, including SRv6 Capabilities (1038), SRv6 Locator (1162), and MPLS Protocol
Mask (1094). The SRv6 decoders omit the reserved words. The MPLS decoder uses
only the LDP and RSVP-TE bits. Unknown locator sub-TLV bytes remain visible as
hex in `sub-tlvs`.

<!-- source: internal/component/bgp/plugins/nlri/ls/register.go -- init -->
<!-- source: internal/component/bgp/plugins/nlri/ls/plugin.go -- familyDecl -->
<!-- source: internal/component/bgp/plugins/nlri/ls/register_attr.go -- init -->
<!-- source: internal/component/bgp/plugins/nlri/ls/attr_srv6.go -- decodeSRv6Capabilities, decodeSRv6Locator -->
<!-- source: internal/component/bgp/plugins/nlri/ls/attr_link.go -- decodeMPLSProtocolMask -->
<!-- source: internal/component/bgp/cli/decode_update.go -- renderAttributeZe -->

The separate `bgp-ls-export` plugin originates standard BGP-LS NLRIs from
native routing state. IS-IS and OSPF publish complete database snapshots through
`linkstateevents`; no exporter imports an IGP plugin or reads its mutable state.
Each source registers its event namespace. The exporter subscribes before asking
for a replay, encodes the borrowed snapshot synchronously, and retains only its
own wire bytes. Source generation numbers prevent an older snapshot from
resurrecting a removed domain.

The `bgp-ls-export` and `bgp-epe` schemas are always loaded. A `bgp-ls-export`
or `bgp-epe` block in the configuration starts its plugin, and removing the
block stops it.
<!-- source: internal/component/bgp/plugins/nlri/ls/yang/register.go -- init -->

`bgp-ls-export { }` enables the internal exporter. Collector peers negotiate the
`bgp-ls` family and attach the plugin with `state` and `refresh` event delivery
and UPDATE-send permission. Each peer-up or BGP-LS refresh receives the current database.
The exporter answers refreshes from its own topology snapshots. Receive-only
AIGP or FlowSpec inputs to `bgp-rib` do not make that RIB a replay owner for these
locally originated routes.
Source replacement withdraws removed identities before announcing new ones;
disabling the exporter withdraws its routes.
The existing UPDATE command path handles next-hop selection, negotiated family
framing, and session AS handling. The exporter does not use raw-message injection.
The exporter requests `next-hop self`. RFC 9552 section 5.5 permits an IPv4 or IPv6
next hop for BGP-LS AFI 16388; its different AFI does not require Extended Next Hop
Encoding negotiation.
Queued announcements retain `self` as a policy until delivery, when it resolves
to the connected session's local endpoint. A reconnect therefore uses the new
session's endpoint, not a configured address or a previously resolved address.
Refresh retains the previous advertisement set, so a concurrent source removal
still produces a withdrawal. Its surviving routes are sent with replay metadata
to bypass the engine's duplicate-announcement suppression. Only acknowledged
announcement or withdrawal counts advance the exporter's sent state, and a
failed collector does not prevent delivery to the other collectors.
Graceful plugin removal handles the SDK `bye` callback while the command
connection remains open. It cancels and joins the export worker, disables
snapshot intake, and withdraws accepted routes within the engine's shutdown
grace. Internal bridge dispatch and socket writes honor cancellation; bounded
cleanup reserves time for later collectors rather than spending the whole grace
on a stalled first peer. Failed withdrawals refuse removal and remain available
for another cleanup attempt. Configuration rollback or re-enable resumes the
worker and requests current native snapshots. An abrupt connection loss cannot
send this cleanup.

The source adapters retain topology membership and LSA provenance. Link membership
in several topologies produces separate NLRIs, and non-default prefixes carry an
MT-ID. OSPF opaque Node, Link, and Prefix attributes accept only their RFC 9552
source LSA classes. The originator clears reserved Node, IGP, MPLS, SRv6 Capability,
and Locator fields. None of these checks changes received-route propagation.

The shared snapshots retain the complete nonexpired LSDB and identify originators
that native IGP SPF determines are unreachable. The BGP-LS exporter withdraws
those originators' Nodes, locally originated Links, Prefixes, and SIDs. A reachable
originator's half-link remains advertised even when its remote endpoint is
unreachable. Native SPF completion republishes the view even without a route
delta, so restored reachability re-advertises unchanged LSDB records as required
by RFC 9552 Section 5.9. Other snapshot consumers retain the complete database.
Purges, removals, retired areas, and protocol shutdown also withdraw records.
For IPv6-only OSPF inter-AS links, the adapter correlates a remote ASBR's actual
four-octet identity from live dual-ID TE evidence. Missing or ambiguous evidence
withholds that link with a diagnostic; it never manufactures a Router-ID.

The `domain` list maps a source namespace, Protocol-ID, native instance, and
native area to an operator-chosen 64-bit `instance-id`. IS-IS uses native instance
1 with areas 1 and 2 for its levels; OSPF uses its configured instance and numeric
area. Without a mapping, the source's routing-universe identifier is retained.
The exporter refuses a database replacement above 65536 routes and keeps the
previous accepted state. It requires the engine event bus and refuses forked
execution rather than running without native data.
A replacement which gives an existing cross-domain NLRI different attributes is
also refused before replacing accepted state. The operator can assign distinct
Instance-IDs when two native domains describe different routing universes.

`bgp-epe` is a separate native producer for configured PeerNode SIDs. Its `srgb`
container names a locally assigned SRGB using inclusive lower and upper label
bounds; each `peer` assigns a persistent `sid-index` and optional weight.
Configured EPE sessions attach `bgp-epe` with state-event delivery. Live BGP
state supplies both session endpoints' ASNs and BGP identifiers. The connected
TCP local endpoint is used even when the local IP is configured as `auto`.
The producer submits the computed pop-and-forward label to the native MPLS FIB
owner. The SID is advertised only after successful installation acknowledgement.
An index-encoded PeerNode SID
is accompanied by the local Node's SRGB. Session loss, configuration removal,
or failed replacement removes the corresponding advertisement. PeerAdj, PeerSet,
and SRv6 EPE segment assignment are not implemented.
If a label removal fails, the native MPLS owner retains its source claim and the
producer withholds that label from reassignment. Before its first installation,
including after a producer error exit, EPE requires acknowledged
`mplsfib.RemoveLabelSource` cleanup of the source's retained AF_MPLS swap/pop
labels. A refused cleanup blocks installation and advertisement. Graceful
removal returns cleanup failures to the lifecycle owner for retry; successful
retirement is idempotent, so a deferred old-source stop cannot clear a new
producer's labels. This does not claim cleanup of prefix-keyed push contexts.

The owner selected **Standard origination only** for private-use origination.
`encodeTopology` produces standard Node, Link, IPv4/IPv6 Prefix, and SRv6 SID
NLRIs; `originateAttributes` refuses private-use BGP-LS TLV types. There is no
native vendor-private producer. RFC 9552 Section 5.4 requires a four-octet
Enterprise Code at the start of the value of a private-use TLV and immediately
after the Total NLRI Length in a private-use NLRI. Those requirements govern
private origination, not this standard-only role. Received unknown/private NLRIs
remain opaque on the propagation path; the originator restriction does not
inspect or rewrite them. Caller-supplied raw BGP messages remain a separate
operator interface without native topology provenance or originator guarantees.

<!-- source: internal/core/linkstateevents/events.go -- Snapshot, RegisterSource, Request -->
<!-- source: internal/plugins/isis/bgpls_export.go -- publishBGPLSLocked -->
<!-- source: internal/plugins/ospf/bgpls_export.go -- publishBGPLSLocked -->
<!-- source: internal/component/bgp/plugins/nlri/ls/export_plugin.go -- runTopologyExporter -->
<!-- source: internal/component/bgp/plugins/nlri/ls/export_state.go -- replace, reconcile -->
<!-- source: internal/component/bgp/plugins/nlri/ls/export_encode.go -- encodeTopology, originateAttributes -->
<!-- source: internal/component/bgp/message/chunk_mp_nlri.go -- bgpLSNLRISize -->
<!-- source: internal/plugins/isis/spf/computer.go -- Reachability, SetOnComplete -->
<!-- source: internal/component/bgp/plugins/nlri/ls/export_config.go -- parseExportConfig -->
<!-- source: internal/component/bgp/reactor/reactor_api.go -- establishedPeerInfo, connectedLocalEndpoint -->
<!-- source: internal/component/bgp/plugins/nlri/ls/epe_source.go -- publishLocked, emitLabel -->
<!-- source: internal/component/bgp/plugins/nlri/ls/epe_config.go -- parseEPEConfig -->
<!-- source: internal/component/bgp/reactor/session_write.go -- Session.SendRawMessage -->

### Receive-Path Fault Management (RFC 9552 Section 8.2.2)

A received BGP-LS Attribute (code 29) is validated on the session path, before any
plugin sees the UPDATE. `validateBGPLSAttr`
(`internal/component/bgp/message/rfc7606_bgpls.go`) is registered in the RFC 7606
`attrValidators` table and walks the attribute's TLVs. It checks two things, which are
the syntactic validation RFC 9552 Section 8.2.2 requires: each TLV length stays inside
the attribute, and the TLV lengths sum to the attribute length.

A failure returns `RFC7606ActionAttributeDiscard`, the action Section 8.2.2 prescribes
for an error that lets the router skip the attribute and process the rest of the UPDATE.
`enforceRFC7606` (`internal/component/bgp/reactor/session_validation.go`) then replaces
the whole attribute with an ATTR_TOMBSTONE that names code 29. The Link-State NLRI stays
in the UPDATE, which is what Section 8.2.2 asks a Propagator to preserve.

No TLV type and no TLV value is read. Section 8.2.2 states that a BGP-LS Attribute MUST
NOT be malformed on the strength of which TLVs it holds or what they contain, and it
lists TLV length correctness and value ranges among the semantic validations a
BGP-LS Propagator does not perform. `decodeAllAttrTLVs`
(`internal/component/bgp/plugins/nlri/ls/attr.go`) is that semantic decode, and it has no
session-path caller by design. `register.go` marks the point where such a call would have
been wired in.

### Receive-Path NLRI Validation (RFC 9552 Section 8.2.2)

The Link-State NLRI is validated on the same session path, by the same section's other
half. Section 8.2.2 lists seven syntactic checks and assigns each malformedness class one
of two actions, so the walk has two entry points in
`internal/component/bgp/message/rfc7606_bgpls_nlri.go`.

`validateBGPLSNLRISyntax` reads the NLRI headers alone. An NLRI whose Total NLRI Length
runs past the MP attribute, or a tail too short to hold another header, means the NLRI
lengths do not sum to the attribute length and no boundary after it can be located.
Section 8.2.2 calls that class non-skipable and prescribes 'AFI/SAFI disable' or, when
that is not implemented, a session reset. Ze implements no AFI/SAFI disable, so the
session resets. `validateMPNLRISyntax` (`rfc7606.go`) routes AFI 16388 here.

`RetainWellFormedNLRI` judges each NLRI inside its own boundaries and removes the ones
Section 8.2.2 calls malformed in a way that lets a router "skip the malformed NLRI(s) and
continue the processing of the rest of the BGP UPDATE message": a TLV or Node Descriptor
sub-TLV length that overruns its container, TLV types that do not ascend as Section 5.1
requires, and a Node Descriptor repeating one sub-TLV type as Section 5.2.1.4 forbids.
The action is 'NLRI discard', which `typedNLRIEdit`
(`internal/component/bgp/reactor/session_validation_nlritype.go`) applies with the same
rewrite it uses for the RFC 7606 Section 5.4 type discard.

An NLRI Type this document does not define stays opaque and is propagated whole, because
Section 5.2 requires it. No TLV value octet is read here either, for the reason the
attribute half gives.

<!-- source: internal/component/bgp/plugins/nlri/ls/types_nlri.go -- BGPLSNode, BGPLSLink, BGPLSPrefix -->
<!-- source: internal/component/bgp/plugins/nlri/ls/types_srv6.go -- BGPLSSRv6SID -->

---

**Last Updated:** 2026-09-23

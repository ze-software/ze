# EVPN NLRI Wire Format (RFC 7432)

**Source:** ExaBGP `bgp/message/update/nlri/evpn/`
**Family:** AFI 25 (L2VPN), SAFI 70 (EVPN)

<!-- source: internal/core/family/family.go -- AFIL2VPN, SAFIEVPN -->
<!-- source: internal/component/bgp/plugins/nlri/evpn/types.go -- L2VPNEVPN -->

---

## Common Header

All EVPN NLRIs share this structure:

```
+---------------------------+
|   Route Type (1 octet)    |
+---------------------------+
|   Length (1 octet)        |  Payload length (excludes header)
+---------------------------+
|   Route Data (variable)   |  Type-specific payload
+---------------------------+
```

<!-- source: internal/component/bgp/plugins/nlri/evpn/types.go -- EVPN common header structure -->

---

## Route Types

| Type | Name | ExaBGP Class |
|------|------|--------------|
| 1 | Ethernet Auto-Discovery | `EthernetAD` |
| 2 | MAC/IP Advertisement | `MAC` |
| 3 | Inclusive Multicast Ethernet Tag | `Multicast` |
| 4 | Ethernet Segment | `Segment` |
| 5 | IP Prefix | `Prefix` |

<!-- source: internal/component/bgp/plugins/nlri/evpn/types.go -- EVPNRouteType1..EVPNRouteType5 -->

---

## Type 1: Ethernet Auto-Discovery

### Wire Format

```
+---------------------------+
|   Route Type = 1          |  1 octet
+---------------------------+
|   Length = 25             |  1 octet
+---------------------------+
|   RD (8 octets)           |  Route Distinguisher
+---------------------------+
|   ESI (10 octets)         |  Ethernet Segment Identifier
+---------------------------+
|   Ethernet Tag (4 octets) |
+---------------------------+
|   MPLS Label (3 octets)   |  One fixed-width field
+---------------------------+
```
The payload length is 25 bytes; the total NLRI length is 27 bytes. The label
field is present even when its value is zero, as required for Ethernet A-D per
ES. It is not a variable MPLS label stack. Received low bits are preserved;
decoding does not require a bottom-of-stack bit. The session ingress validator
rejects a Type 1 payload of any other length before RIB installation or
propagation (RFC 7606 Sections 3 and 5.3).

### ExaBGP Offsets

```python
# After 2-byte header:
# RD: 2-10, ESI: 10-20, ETag: 20-24, Label: 24-27
```

<!-- source: internal/component/bgp/plugins/nlri/evpn/types.go -- EVPNType1.WriteTo, EVPNType1.Len -->
<!-- source: internal/component/bgp/message/evpn_nlri.go -- ValidEVPNNLRILengths -->
<!-- source: internal/component/bgp/reactor/session_validation_nlritype.go -- typedNLRIEdit -->

---

## Type 2: MAC/IP Advertisement

### Wire Format

```
+---------------------------+
|   Route Type = 2          |  1 octet
+---------------------------+
|   Length                  |  1 octet (33-54 depending on IP)
+---------------------------+
|   RD (8 octets)           |
+---------------------------+
|   ESI (10 octets)         |
+---------------------------+
|   Ethernet Tag (4 octets) |
+---------------------------+
|   MAC Length (1 octet)    |  In bits (usually 48)
+---------------------------+
|   MAC Address (6 octets)  |
+---------------------------+
|   IP Length (1 octet)     |  In bits (0, 32, or 128)
+---------------------------+
|   IP Address (0/4/16)     |  Optional
+---------------------------+
|   MPLS Label1 (3 octets)  |  L2 VNI
+---------------------------+
|   MPLS Label2 (3 octets)  |  Optional L3 VNI
+---------------------------+
```

### Valid Lengths (including header)

| Scenario | Length |
|----------|--------|
| No IP, 1 label | 35 |
| No IP, 2 labels | 38 |
| IPv4, 1 label | 39 |
| IPv4, 2 labels | 42 |
| IPv6, 1 label | 51 |
| IPv6, 2 labels | 54 |

### ExaBGP Offsets

```python
# After 2-byte header:
# RD: 2-10, ESI: 10-20, ETag: 20-24
# MAClen: 24, MAC: 25-31, IPlen: 31
# IP: 32+ (0/4/16 bytes), Label: after IP
```

### Route Key (for equality)

Per RFC 7432 Section 7.2, key components are:
- Ethernet Tag
- MAC Address
- IP Address (if present)

**NOT** included: ESI, Labels (these are attributes)

<!-- source: internal/component/bgp/plugins/nlri/evpn/types.go -- EVPNType2 wire format parsing -->

---

## Type 3: Inclusive Multicast Ethernet Tag

### Wire Format

```
+---------------------------+
|   Route Type = 3          |  1 octet
+---------------------------+
|   Length = 17/29          |  1 octet (IPv4/IPv6)
+---------------------------+
|   RD (8 octets)           |
+---------------------------+
|   Ethernet Tag (4 octets) |
+---------------------------+
|   IP Length (1 octet)     |  32 or 128 bits
+---------------------------+
|   Originating Router IP   |  4 or 16 octets
+---------------------------+
```

<!-- source: internal/component/bgp/plugins/nlri/evpn/types.go -- EVPNType3 (Inclusive Multicast) -->

---

## Type 4: Ethernet Segment

### Wire Format

```
+---------------------------+
|   Route Type = 4          |  1 octet
+---------------------------+
|   Length = 23/35          |  1 octet (IPv4/IPv6)
+---------------------------+
|   RD (8 octets)           |
+---------------------------+
|   ESI (10 octets)         |
+---------------------------+
|   IP Length (1 octet)     |  32 or 128 bits
+---------------------------+
|   Originating Router IP   |  4 or 16 octets
+---------------------------+
```

<!-- source: internal/component/bgp/plugins/nlri/evpn/types.go -- EVPNType4 (Ethernet Segment) -->

---

## Type 5: IP Prefix

### Wire Format

```
+---------------------------+
|   Route Type = 5          |  1 octet
+---------------------------+
|   Length = 34/58          |  1 octet (IPv4/IPv6)
+---------------------------+
|   RD (8 octets)           |
+---------------------------+
|   ESI (10 octets)         |
+---------------------------+
|   Ethernet Tag (4 octets) |
+---------------------------+
|   IP Prefix Length (1)    |  In bits
+---------------------------+
|   IP Prefix (4/16 octets) |  Full address width
+---------------------------+
|   Gateway IP (4/16)       |  Same AF as prefix
+---------------------------+
|   MPLS Label (3 octets)   |
+---------------------------+
```

RFC 9136 Section 3 uses a fixed-width prefix address and gateway, followed by
one three-octet label. The prefix length identifies the significant bits; it
does not shorten either address field. Local encoding masks host bits and
requires the gateway to use the prefix's address family. An omitted gateway
is encoded as zero and an omitted label retains the mandatory zero-valued field.

<!-- source: internal/component/bgp/plugins/nlri/evpn/types.go -- EVPNType5 (IP Prefix) -->

---

## ExaBGP Implementation

### Base Class

```python
@NLRI.register(AFI.l2vpn, SAFI.evpn)
class EVPN(NLRI):
    HEADER_SIZE = 2  # type(1) + length(1)
    CODE: ClassVar[int] = -1  # Set by decorator

    def __init__(self, packed: Buffer) -> None:
        NLRI.__init__(self, AFI.l2vpn, SAFI.evpn)
        self._packed = bytes(packed)

    def pack_nlri(self, negotiated: Negotiated) -> Buffer:
        return self._packed  # Zero-copy
```

### Type 2 (MAC) Example

```python
@EVPN.register_evpn_route(code=2)
class MAC(EVPN):
    NAME = 'MAC/IP advertisement'
    SHORT_NAME = 'MACAdv'

    @property
    def rd(self) -> RouteDistinguisher:
        return RouteDistinguisher(self._packed[2:10])

    @property
    def esi(self) -> ESI:
        return ESI(self._packed[10:20])

    @property
    def mac(self) -> MACQUAL:
        return MACQUAL(self._packed[25:31])

    @property
    def ip(self) -> IP | None:
        iplen_bits = self._packed[31]
        if iplen_bits == 0:
            return None
        return IP.create_ip(self._packed[32:32 + iplen_bits // 8])

    def __eq__(self, other):
        # ESI and label NOT part of comparison
        return (self.CODE == other.CODE and
                self.rd == other.rd and
                self.etag == other.etag and
                self.mac == other.mac and
                self.ip == other.ip)
```

---

## JSON Output

### Type 2 (MAC/IP)

```json
{
  "code": 2,
  "parsed": true,
  "raw": "...",
  "name": "MAC/IP advertisement",
  "rd": "65000:100",
  "esi": "00:11:22:33:44:55:66:77:88:99",
  "etag": 0,
  "mac": "aa:bb:cc:dd:ee:ff",
  "label": [ [100, 1601] ],
  "ip": "10.0.0.1"
}
```

### Generic (Unknown Type)

```json
{
  "code": 99,
  "parsed": false,
  "raw": "630A..."
}
```

---

## Ze Implementation Notes

### Local route origination

The route-command encoder and native `update` configuration support Ethernet
A-D, MAC/IP, IMET, Ethernet Segment, and IP Prefix NLRIs. `BuildEVPN` rejects
Ethernet A-D per-ES routes (Ethernet Tag `4294967295`) unless they have an
IPv4-address-specific Type 1 RD, a zero three-octet NLRI label, an ESI Label
extended community, and at least one Route Target. IMET advertisements require
at least one Route Target. Ethernet Segment advertisements require a Type 1 RD
and the dedicated ES-Import Route Target (`0x06/0x02`) specified by RFC 7432
Section 8.1.1; neither an ordinary Route Target nor an ESI Label substitutes for it.
The route-command builder also rejects an empty NLRI and requires an explicit
unicast next hop. Missing, unspecified, and multicast next hops are refused
before any UPDATE is built. Native configuration preserves `next-hop self`
through route conversion and resolves it from the connected session's local
endpoint before emission, independently of the NLRI's originator address or the
peer's default export next-hop policy.

The same checks apply to locally built API batches, including announcements
queued before a session is established. `update text` checks its announcements
before creating a batch. Withdrawals and received wire replay do not acquire
these sender checks; received NLRI still passes the session's framing and
fixed-field validation.

The local encoder also refuses extended-community values whose byte length is
not a multiple of eight. These incomplete values cannot satisfy a required
Route Target, ESI Label, or ES-Import community.

An IMET route command supplies its Route Target explicitly:

```
multicast rd 65000:7 next-hop 192.0.2.1 extended-community target:65000:100
```

`extended-community` can be repeated and accepts the shared named community
syntax or an eight-octet hexadecimal value. A Route Origin community is not a
substitute for a Route Target.

Native configuration supplies the same route fields after `l2vpn/evpn add`.
Shared path attributes belong in the `attribute` block:

```
update {
    attribute {
        next-hop 192.0.2.1;
        extended-community [ target:65000:100 0x0601000000001000 ];
    }
    nlri {
        l2vpn/evpn add ethernet-ad rd 192.0.2.1:7 esi 00:01:02:03:04:05:06:07:08:09 etag 4294967295;
    }
}
```

For IMET and Ethernet Segment routes, `ip` supplies the originating router's
address independently of the MP_REACH next hop. Without `ip`, the encoder uses
an explicit next-hop address; `next-hop self` alone cannot supply an
originating router's address while configuration is being parsed.
Both NLRI-only and route-command encoders reject a missing originating address
before writing IMET or Ethernet Segment bytes. MAC/IP advertisements always
carry their first three-octet label field, including when its value is zero.

<!-- source: internal/component/bgp/plugins/nlri/evpn/encode.go -- EncodeRoute -->
<!-- source: internal/component/bgp/message/update_build_evpn.go -- ValidateEVPNOrigination -->
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- ParseUpdateText -->
<!-- source: internal/component/bgp/plugins/nlri/evpn/config.go -- parseConfigRoute -->
<!-- source: internal/component/bgp/reactor/evpn_origination.go -- validateQueuedEVPNOrigin -->

### BGP control-plane role

Ze's EVPN role is a BGP control-plane speaker: it receives and propagates EVPN
routes and originates routes explicitly supplied by configuration or the API.
It does not instantiate MAC-VRFs, attach CEs, learn or age local MAC addresses,
elect a DF, allocate service labels, or forward Ethernet/BUM packets. Non-CIDR
EVPN best changes do not enter the system IP FIB.

This distinguishes route requirements from the following conditional PE
procedures in RFC 7432:

| Procedure | Condition in the RFC |
|-----------|----------------------|
| VLAN-to-Ethernet-Tag binding (§6.3) | A VLAN-aware bundle service has bridge tables and a provider VID assignment. Ze instead takes the explicit Ethernet Tag. |
| RT auto-derivation (§§7.10–7.10.1) | The PE chooses automatic RT derivation. Ze uses explicitly configured RTs, the other permitted choice. |
| Complete per-ES RT set (§8.2.1.1) | A PE knows the EVIs to which its Ethernet segment belongs. Ze requires an RT on each originated per-ES route but has no ES-to-EVI membership database from which to construct the entire set. |
| Split horizon and ESI-label assignment (§§8.3–8.3.1.2) | A multihomed PE forwards BUM traffic using ingress replication or P2MP MPLS LSPs. |
| DF election (§8.5) | PEs share an attached Ethernet segment. |
| Default-gateway MAC forwarding state (§10.1) | A PE acts as the EVPN default gateway and imports the gateway route. |
| Tree identity, EVI label binding, and aggregation re-advertisement (§11.2) | A PE selects and manages a P-multicast tree; aggregation additionally binds EVIs to that tree. |
| P2MP leaf selection and packet encapsulation (§13.1) | A PE forwards flooded CE packets over P2MP LSPs. |
| MAC-age withdrawal (§17.3) | A locally learned MAC entry ages out on a PE. |

These PE procedures are not implemented by the BGP role. Explicitly
originating a route does not create their missing service or forwarding
state, and is not evidence of full PE conformance.

<!-- source: internal/component/bgp/plugins/nlri/evpn/register.go -- init -->
<!-- source: internal/component/sysrib/sysrib.go -- processEvent -->

### Packed-Bytes-First Pattern

Store complete wire format including header:

```go
type EVPN struct {
    packed []byte  // [type:1][length:1][payload...]
}

func (e *EVPN) RouteType() int {
    return int(e.packed[0])
}

func (e *EVPN) RD() RouteDistinguisher {
    return ParseRD(e.packed[2:10])
}
```

### Type Registry

```go
var evpnRegistry = map[int]EVPNUnpacker{
    1: unpackEthernetAD,
    2: unpackMAC,
    3: unpackMulticast,
    4: unpackSegment,
    5: unpackPrefix,
}
```

<!-- source: internal/component/bgp/plugins/nlri/evpn/types.go -- EVPN type parsing and encoding -->

---

**Last Updated:** 2026-09-23

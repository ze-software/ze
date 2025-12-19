# BGP-LS NLRI Wire Format (RFC 7752)

**Source:** ExaBGP `bgp/message/update/nlri/bgpls/`
**Family:** AFI 16388 (BGP-LS), SAFI 71 (bgp_ls) or 72 (bgp_ls_vpn)

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

---

## NLRI Types

| Type | Name | Description |
|------|------|-------------|
| 1 | Node NLRI | Describes a node (router) |
| 2 | Link NLRI | Describes a link between nodes |
| 3 | IPv4 Prefix NLRI | IPv4 reachability |
| 4 | IPv6 Prefix NLRI | IPv6 reachability |
| 5 | SRv6 SID NLRI | Segment Routing v6 |

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
| 227 | FreeRTR (non-standard) |

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
|   Link Descriptors        |  TLV Type 258
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
|   Prefix Descriptors      |  TLV Type 259
+---------------------------+
```

### Prefix Descriptor TLVs

| Type | Name | Length |
|------|------|--------|
| 263 | Multi-Topology ID | 2 |
| 264 | OSPF Route Type | 1 |
| 265 | IP Reachability Information | Variable |

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

## ZeBGP Implementation Notes

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
    5: unpackSRv6SID,
}
```

---

**Last Updated:** 2025-12-19

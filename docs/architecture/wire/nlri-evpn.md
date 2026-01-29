# EVPN NLRI Wire Format (RFC 7432)

**Source:** ExaBGP `bgp/message/update/nlri/evpn/`
**Family:** AFI 25 (L2VPN), SAFI 70 (EVPN)

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

---

## Route Types

| Type | Name | ExaBGP Class |
|------|------|--------------|
| 1 | Ethernet Auto-Discovery | `EthernetAD` |
| 2 | MAC/IP Advertisement | `MAC` |
| 3 | Inclusive Multicast Ethernet Tag | `Multicast` |
| 4 | Ethernet Segment | `Segment` |
| 5 | IP Prefix | `Prefix` |

---

## Type 1: Ethernet Auto-Discovery

### Wire Format

```
+---------------------------+
|   Route Type = 1          |  1 octet
+---------------------------+
|   Length = 25             |  1 octet (17 without label, +3 per label)
+---------------------------+
|   RD (8 octets)           |  Route Distinguisher
+---------------------------+
|   ESI (10 octets)         |  Ethernet Segment Identifier
+---------------------------+
|   Ethernet Tag (4 octets) |
+---------------------------+
|   MPLS Label (3 octets)   |  Optional, per RFC 7432
+---------------------------+
```

### ExaBGP Offsets

```python
# After 2-byte header:
# RD: 2-10, ESI: 10-20, ETag: 20-24, Label: 24+
```

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

---

## Type 5: IP Prefix

### Wire Format

```
+---------------------------+
|   Route Type = 5          |  1 octet
+---------------------------+
|   Length                  |  1 octet (variable)
+---------------------------+
|   RD (8 octets)           |
+---------------------------+
|   ESI (10 octets)         |
+---------------------------+
|   Ethernet Tag (4 octets) |
+---------------------------+
|   IP Prefix Length (1)    |  In bits
+---------------------------+
|   IP Prefix (variable)    |  Truncated to prefix length
+---------------------------+
|   Gateway IP (4/16)       |  Same AF as prefix
+---------------------------+
|   MPLS Label (3 octets)   |
+---------------------------+
```

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

---

**Last Updated:** 2025-12-19

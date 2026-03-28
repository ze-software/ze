# NLRI Qualifiers Wire Format

**Source:** ExaBGP `bgp/message/update/nlri/qualifier/`
**Purpose:** Document wire format for NLRI qualifier components

---

## Overview

Qualifiers are reusable components used within NLRI types:

| Qualifier | Size | Used By |
|-----------|------|---------|
| RouteDistinguisher | 8 bytes | IPVPN, FlowSpec VPN, BGP-LS VPN, EVPN |
| Labels | 3n bytes | IPVPN, Label, EVPN |
| PathInfo | 4 bytes | All (with ADD-PATH) |
| ESI | 10 bytes | EVPN |
| EthernetTag | 4 bytes | EVPN |
| MAC | 6 bytes | EVPN |

<!-- source: internal/component/bgp/nlri/rd.go -- RouteDistinguisher -->
<!-- source: internal/component/bgp/nlri/helpers.go -- WriteLabelStack -->

---

## Route Distinguisher (RFC 4364)

### Wire Format

```
+---------------------------+
|   Type (2 octets)         |  0, 1, or 2
+---------------------------+
|   Value (6 octets)        |  Type-specific encoding
+---------------------------+
```

Total: **8 bytes**

### Type 0: ASN2:NN

```
+---------------------------+
|   Type = 0x0000           |  2 bytes
+---------------------------+
|   ASN (2 octets)          |  2-byte AS number
+---------------------------+
|   Assigned (4 octets)     |  Admin-assigned value (up to 2^32-1)
+---------------------------+
```

**String format:** `ASN:Assigned` (e.g., `65000:100`)

### Type 1: IP:NN

```
+---------------------------+
|   Type = 0x0001           |  2 bytes
+---------------------------+
|   IP (4 octets)           |  IPv4 address
+---------------------------+
|   Assigned (2 octets)     |  Admin-assigned value (up to 65535)
+---------------------------+
```

**String format:** `IP:Assigned` (e.g., `192.168.1.1:100`)

### Type 2: ASN4:NN

```
+---------------------------+
|   Type = 0x0002           |  2 bytes
+---------------------------+
|   ASN (4 octets)          |  4-byte AS number
+---------------------------+
|   Assigned (2 octets)     |  Admin-assigned value (up to 65535)
+---------------------------+
```

**String format:** `ASN:Assigned` (e.g., `4200000001:100`)

<!-- source: internal/component/bgp/nlri/rd.go -- RDType0, RDType1, RDType2, RouteDistinguisher struct -->

### ExaBGP Implementation

```python
class RouteDistinguisher:
    LENGTH = 8
    TYPE_AS2_ADMIN = 0
    TYPE_IPV4_ADMIN = 1
    TYPE_AS4_ADMIN = 2

    NORD: ClassVar['RouteDistinguisher']  # Empty/no RD singleton

    def __init__(self, packed: Buffer) -> None:
        self._packed = packed

    def _str(self) -> str:
        t, c1, c2, c3 = unpack('!HHHH', self._packed)
        if t == 0:  # Type 0: ASN2:NN
            return f'{c1}:{(c2 << 16) + c3}'
        elif t == 1:  # Type 1: IP:NN
            return f'{c1 >> 8}.{c1 & 0xFF}.{c2 >> 8}.{c2 & 0xFF}:{c3}'
        elif t == 2:  # Type 2: ASN4:NN
            return f'{(c1 << 16) + c2}:{c3}'
```

### JSON Output

```json
"rd": "65000:100"
```

<!-- source: internal/component/bgp/nlri/rd.go -- ParseRouteDistinguisher, RouteDistinguisher.String -->

---

## MPLS Labels (RFC 3107)

### Wire Format

Each label is 3 bytes (24 bits):

```
 0                   1                   2
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|          Label Value (20 bits)        |Exp|S|
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

- **Label Value:** Bits 0-19 (20 bits, value 0-1048575)
- **Exp/TC:** Bits 20-22 (3 bits, experimental/traffic class)
- **S (BoS):** Bit 23 (1 bit, bottom of stack)

### Encoding

```python
def encode_label(label: int, bos: bool = True) -> bytes:
    value = label << 4
    if bos:
        value |= 1  # Set bottom of stack bit
    return pack('!L', value)[1:]  # Take last 3 bytes

def decode_label(data: bytes) -> tuple[int, bool]:
    raw = unpack('!L', b'\x00' + data[:3])[0]
    label = raw >> 4
    bos = bool(raw & 1)
    return label, bos
```

### Special Values

| Raw Bytes | Raw Value | Label Value | Meaning |
|-----------|-----------|-------------|---------|
| `80 00 00` | 0x800000 | 524288 | Withdrawal |
| `00 00 00` | 0x000000 | 0 | Next-hop |

### Label Stack

Multiple labels appear consecutively, last has BoS=1:

```
Label 1000, Label 2000 (stack):
00 3E 80   Label 1000, BoS=0 (1000<<4 = 0x3E80)
00 7D 01   Label 2000, BoS=1 (2000<<4 | 1 = 0x7D01)
```

<!-- source: internal/component/bgp/nlri/helpers.go -- WriteLabelStack -->

### ExaBGP Implementation

```python
class Labels:
    NOLABEL: ClassVar['Labels']  # Empty labels singleton

    def __init__(self, packed: Buffer) -> None:
        if len(packed) % 3 != 0:
            raise ValueError('Labels must be multiple of 3 bytes')
        self._packed = packed

    @classmethod
    def make_labels(cls, labels: list[int], bos: bool = True) -> 'Labels':
        packed_parts = []
        for i, label in enumerate(labels):
            value = label << 4
            if bos and i == len(labels) - 1:
                value |= 1
            packed_parts.append(pack('!L', value)[1:])
        return cls(b''.join(packed_parts))

    @property
    def labels(self) -> list[int]:
        result = []
        data = self._packed
        while len(data) >= 3:
            raw = unpack('!L', b'\x00' + data[:3])[0]
            result.append(raw >> 4)
            data = data[3:]
        return result
```

### JSON Output

```json
"label": [ [100, 1601] ]
```

Format: `[ [label_value, raw_24bit_value], ... ]`

<!-- source: internal/component/bgp/nlri/nlri.go -- ParseLabelStack, EncodeLabelStack -->

---

## Path Info / ADD-PATH (RFC 7911)

### Wire Format

```
+---------------------------+
|   Path ID (4 octets)      |  Unique path identifier
+---------------------------+
```

Total: **4 bytes** (when present)

### Special Values

| Value | Meaning |
|-------|---------|
| `00 00 00 00` | NOPATH - ADD-PATH enabled but no specific ID |
| (absent) | DISABLED - ADD-PATH not negotiated |

### ExaBGP Implementation

```python
class PathInfo:
    LENGTH = 4
    NOPATH: ClassVar['PathInfo']    # ADD-PATH enabled, no ID
    DISABLED: ClassVar['PathInfo']  # ADD-PATH not negotiated

    def __init__(self, packed: Buffer) -> None:
        self._packed = packed
        self._disabled = False

    @classmethod
    def make_from_integer(cls, integer: int) -> 'PathInfo':
        packed = pack('!I', integer)
        return cls(packed)

    @classmethod
    def make_from_ip(cls, ip: str) -> 'PathInfo':
        # Allows "1.2.3.4" notation for path ID
        packed = bytes([int(x) for x in ip.split('.')])
        return cls(packed)
```

### JSON Output

```json
"path-information": "1.2.3.4"
```

<!-- source: internal/component/bgp/nlri/nlri.go -- WriteNLRI, LenWithContext -->
<!-- source: internal/component/bgp/nlri/base.go -- PrefixNLRI.PathID -->

---

## ESI - Ethernet Segment Identifier (RFC 7432)

### Wire Format

```
+---------------------------+
|   Type (1 octet)          |  ESI Type (0-5)
+---------------------------+
|   Value (9 octets)        |  Type-specific value
+---------------------------+
```

Total: **10 bytes**

### ESI Types

| Type | Name | Value Format |
|------|------|--------------|
| 0 | Arbitrary | 9 arbitrary bytes |
| 1 | LACP-based | 6-byte System ID + 2-byte Port Key + 1 byte |
| 2 | L2 Bridge Protocol | 6-byte Root Bridge MAC + 2-byte Priority |
| 3 | MAC-based | 6-byte System MAC + 3-byte discriminator |
| 4 | Router ID | 4-byte Router ID + 4-byte discriminator |
| 5 | AS-based | 4-byte AS + 4-byte discriminator |

### Special Values

| Value | Meaning |
|-------|---------|
| `00:00:00:00:00:00:00:00:00:00` | Single-homed (default) |
| `FF:FF:FF:FF:FF:FF:FF:FF:FF:FF` | Reserved (max) |

<!-- source: internal/component/bgp/plugins/nlri/evpn/types.go -- ESI fields in EVPNType1..4 -->

### ExaBGP Implementation

```python
class ESI:
    LENGTH = 10
    DEFAULT = bytes([0x00] * 10)  # All zeros
    MAX = bytes([0xFF] * 10)      # All ones

    def __init__(self, packed: Buffer) -> None:
        if len(packed) != self.LENGTH:
            raise ValueError(f'ESI requires {self.LENGTH} bytes')
        self._packed = packed

    def __str__(self) -> str:
        if self._packed == self.DEFAULT:
            return '-'
        return ':'.join(f'{b:02x}' for b in self._packed)
```

### JSON Output

```json
"esi": "00:11:22:33:44:55:66:77:88:99"
```

Or for default: `"esi": "-"`

---

## Ethernet Tag (RFC 7432)

### Wire Format

```
+---------------------------+
|   Ethernet Tag ID         |  4 octets, network byte order
+---------------------------+
```

Total: **4 bytes**

### Special Values

| Value | Meaning |
|-------|---------|
| 0 | No Ethernet Tag (default) |
| 0xFFFFFFFF | MAX_ET |

<!-- source: internal/component/bgp/plugins/nlri/evpn/types.go -- EthernetTag field in EVPN route types -->

### ExaBGP Implementation

```python
class EthernetTag:
    LENGTH = 4
    NOETAG: ClassVar['EthernetTag']  # Zero ETag singleton

    def __init__(self, packed: Buffer) -> None:
        if len(packed) != self.LENGTH:
            raise ValueError(f'EthernetTag requires {self.LENGTH} bytes')
        self._packed = packed

    @property
    def tag(self) -> int:
        return unpack('!I', self._packed)[0]

    def __str__(self) -> str:
        return str(self.tag)
```

### JSON Output

```json
"etag": 100
```

---

## MAC Address (EVPN)

### Wire Format

```
+---------------------------+
|   MAC Address (6 octets)  |  Standard Ethernet MAC
+---------------------------+
```

Total: **6 bytes**

### MAC Address Length Field

In EVPN Type 2, the MAC length field is in **bits**:
- Standard: 48 bits (6 bytes)
- Other values possible but rare

<!-- source: internal/component/bgp/plugins/nlri/evpn/types.go -- MAC field in EVPNType2 -->

### ExaBGP Implementation

```python
class MAC:
    LENGTH = 6

    def __init__(self, packed: Buffer) -> None:
        if len(packed) != self.LENGTH:
            raise ValueError(f'MAC requires {self.LENGTH} bytes')
        self._packed = packed

    def __str__(self) -> str:
        return ':'.join(f'{b:02x}' for b in self._packed)

    def __len__(self) -> int:
        return self.LENGTH * 8  # Return length in bits
```

### JSON Output

```json
"mac": "00:11:22:33:44:55"
```

---

## Ze Implementation Notes

### RouteDistinguisher Structure

Defined in `internal/component/bgp/nlri/rd.go`:

```go
type RouteDistinguisher struct {
    Type  RDType   // RFC 4364 Section 4.2: Type field (2 bytes)
    Value [6]byte  // RFC 4364 Section 4.2: Value field (6 bytes)
}
```

### Zero Values

Zero-valued structs represent "no value":

```go
var rd RouteDistinguisher  // Zero RD
```

### Comparison

Qualifiers compare by struct equality (value semantics):

```go
if rd1 == rd2 { ... }  // Direct struct comparison
```

<!-- source: internal/component/bgp/nlri/rd.go -- RouteDistinguisher struct (value semantics) -->

---

**Last Updated:** 2025-12-19

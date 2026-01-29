# 4-Byte AS Number Handling (RFC 6793)

**Source:** ExaBGP `bgp/message/open/asn.py`, `bgp/message/update/attribute/aspath.py`
**Purpose:** Document 4-byte ASN handling and AS_TRANS

---

## Overview

4-byte AS numbers (ASN4) extend the AS number space from 0-65535 to 0-4294967295.

### Key Concepts

| Term | Value | Description |
|------|-------|-------------|
| AS_TRANS | 23456 | Reserved ASN for 4-byte transition |
| 2-byte max | 65535 (0xFFFF) | Maximum 2-byte ASN |
| 4-byte max | 4294967295 (0xFFFFFFFF) | Maximum 4-byte ASN |

---

## Capability Negotiation

### ASN4 Capability (Code 65)

```
+---------------------------+
|   Capability Code = 65    |  1 octet
+---------------------------+
|   Capability Length = 4   |  1 octet
+---------------------------+
|   4-byte AS Number        |  4 octets
+---------------------------+
```

### OPEN Message

The OPEN message's `My Autonomous System` field is 2 bytes:
- If local AS > 65535: Use AS_TRANS (23456)
- Real AS conveyed in ASN4 capability

```python
# OPEN encoding
if local_as > 65535:
    open_asn = AS_TRANS  # 23456
else:
    open_asn = local_as
```

---

## AS_PATH Encoding

### When Peer Supports ASN4

All AS_PATH and AS4_PATH use 4-byte ASN encoding:

```
Segment Type (1 byte) | Length (1 byte) | ASN1 (4 bytes) | ASN2 (4 bytes) ...
```

### When Peer Does NOT Support ASN4

Two attributes used together:

1. **AS_PATH (code 2):** 2-byte ASNs, large ASNs replaced with AS_TRANS
2. **AS4_PATH (code 17):** 4-byte ASNs with real values

```
Example: AS path [65001, 4200000001, 65002]

AS_PATH (2-byte):   [65001, 23456, 65002]   <- 4200000001 → AS_TRANS
AS4_PATH (4-byte):  [65001, 4200000001, 65002]  <- Real values
```

---

## Reconstruction Algorithm

When receiving from a 2-byte peer:

```python
def reconstruct_aspath(as_path, as4_path):
    """Merge AS_PATH and AS4_PATH to get real path.

    RFC 6793 Section 4.2.3:
    1. If AS4_PATH shorter than AS_PATH: prepend AS_PATH entries
    2. Replace AS_TRANS values with AS4_PATH values
    """
    if as4_path is None:
        return as_path

    # Get lengths
    as_path_len = sum(len(seg) for seg in as_path)
    as4_path_len = sum(len(seg) for seg in as4_path)

    if as4_path_len < as_path_len:
        # Prepend extra entries from AS_PATH
        diff = as_path_len - as4_path_len
        # Take first 'diff' ASNs from AS_PATH, then all of AS4_PATH
        result = as_path[:diff] + as4_path
    else:
        result = as4_path

    return result
```

---

## AS4_AGGREGATOR

Similar to AS_PATH, the AGGREGATOR attribute has two forms:

| Attribute | Code | ASN Size | Used When |
|-----------|------|----------|-----------|
| AGGREGATOR | 7 | 2 bytes | Peer doesn't support ASN4 |
| AS4_AGGREGATOR | 18 | 4 bytes | Peer doesn't support ASN4 + large AS |

### AGGREGATOR Format

```
+---------------------------+
|   AS Number (2 or 4 bytes)|  Depends on ASN4 capability
+---------------------------+
|   Aggregator IP (4 bytes) |
+---------------------------+
```

### Encoding Logic

```python
def pack_aggregator(asn, ip, peer_supports_asn4):
    if peer_supports_asn4:
        # Single AGGREGATOR with 4-byte ASN
        return pack('!L', asn) + ip.packed
    else:
        if asn > 65535:
            # AGGREGATOR with AS_TRANS + AS4_AGGREGATOR with real
            agg = pack('!H', AS_TRANS) + ip.packed
            as4_agg = pack('!L', asn) + ip.packed
            return pack_attr(7, agg) + pack_attr(18, as4_agg)
        else:
            # Single AGGREGATOR with 2-byte ASN
            return pack('!H', asn) + ip.packed
```

---

## ExaBGP Implementation

### ASN Class

```python
class ASN(Resource):
    MAX_2BYTE = 65535
    MAX_4BYTE = 4294967295

    def asn4(self) -> bool:
        """True if this ASN requires 4-byte encoding."""
        return self > self.MAX_2BYTE

    def pack_asn(self, asn4: bool) -> bytes:
        """Pack as 2 or 4 byte value."""
        return pack('!L' if asn4 else '!H', self)

    def trans(self) -> ASN:
        """Return AS_TRANS if 4-byte, else self."""
        if self.asn4():
            return AS_TRANS
        return self

AS_TRANS = ASN(23456)
```

### ASPath.pack_attribute()

```python
def pack_attribute(self, negotiated: Negotiated) -> bytes:
    if negotiated.asn4:
        # Peer supports ASN4, send 4-byte format
        if self._asn4:
            return self._attribute(self._packed)
        else:
            # Convert to 4-byte
            return self._attribute(self._pack_segments_raw(self.aspath, asn4=True))

    # Peer doesn't support ASN4
    has_large_asn = False
    astrans = []

    for content in self.aspath:
        local = content.__class__()
        for asn in content:
            if not asn.asn4():
                local.append(asn)
            else:
                local.append(AS_TRANS)  # Replace with 23456
                has_large_asn = True
        astrans.append(local)

    message = self._attribute(self._pack_segments_raw(tuple(astrans), asn4=False))
    if has_large_asn:
        # Add AS4_PATH with real values
        message += AS4Path._attribute(AS4Path._pack_segments_raw(self.aspath, asn4=True))

    return message
```

---

## Wire Examples

### 4-byte peer: AS path [65001, 4200000001]

```
AS_PATH attribute:
  02              # AS_SEQUENCE
  02              # 2 ASNs
  00 00 FD E9     # 65001
  FA 56 EA 01     # 4200000001
```

### 2-byte peer: AS path [65001, 4200000001]

```
AS_PATH attribute:
  02              # AS_SEQUENCE
  02              # 2 ASNs
  FD E9           # 65001 (2 bytes)
  5B A0           # 23456 = AS_TRANS (2 bytes)

AS4_PATH attribute:
  02              # AS_SEQUENCE
  02              # 2 ASNs
  00 00 FD E9     # 65001 (4 bytes)
  FA 56 EA 01     # 4200000001 (4 bytes)
```

---

## Ze Implementation Notes

### ASN Type

```go
type ASN uint32

const (
    MaxASN2Byte ASN = 65535
    MaxASN4Byte ASN = 4294967295
    ASTrans     ASN = 23456
)

func (a ASN) Is4Byte() bool {
    return a > MaxASN2Byte
}

func (a ASN) Pack(asn4 bool) []byte {
    if asn4 {
        buf := make([]byte, 4)
        binary.BigEndian.PutUint32(buf, uint32(a))
        return buf
    }
    buf := make([]byte, 2)
    binary.BigEndian.PutUint16(buf, uint16(a))
    return buf
}

func (a ASN) Trans() ASN {
    if a.Is4Byte() {
        return ASTrans
    }
    return a
}
```

### Sending AS_PATH

```go
func (p *ASPath) Pack(neg *Negotiated) []byte {
    if neg.ASN4 {
        return p.pack4Byte()
    }

    // 2-byte peer
    hasLarge := false
    trans := make([]Segment, len(p.segments))
    for i, seg := range p.segments {
        trans[i] = make(Segment, len(seg))
        for j, asn := range seg {
            if asn.Is4Byte() {
                trans[i][j] = ASTrans
                hasLarge = true
            } else {
                trans[i][j] = asn
            }
        }
    }

    result := packASPath(trans, false)
    if hasLarge {
        result = append(result, packAS4Path(p.segments)...)
    }
    return result
}
```

---

**Last Updated:** 2025-12-19

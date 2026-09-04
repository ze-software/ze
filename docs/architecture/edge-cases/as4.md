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
<!-- source: internal/core/bgp/attribute/as4.go -- AS4Path, AttrAS4Path -->

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
<!-- source: internal/core/bgp/capability/capability.go -- CodeASN4=65 -->

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
<!-- source: internal/core/bgp/attribute/as4.go -- AS4Path, Flags, Len -->
<!-- source: internal/core/bgp/capability/negotiated.go -- Negotiated.ASN4 -->

---

## Reconstruction Algorithm

RFC 6793 Section 4.2.3 states one procedure for a received UPDATE, and its two
halves are not separable: the AGGREGATOR decides whether the AS4_* attributes
are read at all, and only then is the AS path constructed.

### The AGGREGATOR gate

The gate applies when BOTH the AGGREGATOR and the AS4_AGGREGATOR are received.
With only one of them there is nothing to choose between, and the AS4_PATH is
used.

| AGGREGATOR AS | Aggregating node | AS path information |
|---|---|---|
| not AS_TRANS | the AGGREGATOR | the AS_PATH. The AS4_AGGREGATOR and the AS4_PATH are ignored |
| AS_TRANS | the AS4_AGGREGATOR. The AGGREGATOR is ignored | constructed, as below |

`selectAggregator` makes that choice on the ingest path, and an ignored AS4_PATH
reaches `canonicalizeASPath` as an absent one.
<!-- source: internal/component/bgp/plugins/rib/storage/attrparse.go -- selectAggregator, canonicalizeASPath -->

### The AS path construction

The AS number count decides, and it is the RFC 4271 Section 9.1.2.2 count: an
AS_SET counts as one whatever it holds, and RFC 5065 counts nothing for an
AS_CONFED_SEQUENCE or an AS_CONFED_SET.

| Count comparison | Result |
|---|---|
| AS_PATH count < AS4_PATH count | the AS4_PATH is ignored, and the AS_PATH is the AS path information |
| AS_PATH count >= AS4_PATH count | as many AS numbers and path segments as necessary are taken from the leading part of the AS_PATH and prepended to the AS4_PATH, so the result has the AS_PATH's own AS number count |

The AS4_PATH is peer-supplied and nothing verifies it, so the first row is what
stops a peer lengthening a path by sending an oversized AS4_PATH.

A confederation segment is prepended when it leads the path or sits beside a
segment that is prepended, and it spends none of the count budget. Ze walks the
leading segments in order and stops at the first segment it does not prepend,
which is what leaves a confederation segment further along unreached.

`MergeAS4Path` owns the whole construction, and both callers take its verdict:
the RIB ingest path and the filter-text builder that every text-mode filter
judges.
<!-- source: internal/core/bgp/attribute/as4.go -- MergeAS4Path, appendLeadingSegments, countASNs -->
<!-- source: internal/component/bgp/reactor/filter_format.go -- asPathForFilter -->

### What it costs

The reconstruction runs only for an UPDATE that carries an AS4_PATH, which an
OLD speaker sends and a session between NEW speakers never does. The common
path parses nothing and allocates nothing beyond the widening buffer a
two-octet AS_PATH already needed.

---

## AS4_AGGREGATOR

Similar to AS_PATH, the AGGREGATOR attribute has two forms:

| Attribute | Code | ASN Size | Used When |
|-----------|------|----------|-----------|
| AGGREGATOR | 7 | 2 bytes | Peer doesn't support ASN4 |
| AS4_AGGREGATOR | 18 | 4 bytes | Peer doesn't support ASN4 + large AS |
<!-- source: internal/core/bgp/attribute/as4.go -- AS4Path code=17, AS4Aggregator code=18 -->

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
<!-- source: internal/core/bgp/attribute/as4.go -- AS4Path.WriteTo -->
<!-- source: internal/core/bgp/capability/negotiated.go -- Negotiated.ASN4 -->

---

**Last Updated:** 2025-12-19

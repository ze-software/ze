# FlowSpec NLRI Wire Format (RFC 5575, RFC 8955)

**Source:** ExaBGP `bgp/message/update/nlri/flow.py`
**Family:** AFI 1/2, SAFI 133 (flow_ip) or 134 (flow_vpn)

<!-- source: internal/component/bgp/nlri/nlri.go -- SAFIFlowSpec -->
<!-- source: internal/component/bgp/nlri/constants.go -- SAFIFlowSpecVPN -->

---

## Wire Format Overview

```
+---------------------------+
|   Length (1-2 octets)     |  NLRI length
+---------------------------+
|   RD (8 octets)           |  Only for SAFI 134 (flow_vpn)
+---------------------------+
|   Components (variable)   |  Ordered filter rules
+---------------------------+
```

<!-- source: internal/component/bgp/plugins/nlri/flowspec/types.go -- FlowSpec struct -->
<!-- source: internal/component/bgp/plugins/nlri/flowspec/types_vpn.go -- FlowSpecVPN struct -->

---

## Length Encoding

| Condition | Encoding |
|-----------|----------|
| length < 240 | Single byte |
| length >= 240 | `(0xF0 | (len >> 8))` + `(len & 0xFF)` |

### Decoding

```python
length = data[0]
if (length & 0xF0) == 0xF0:  # Extended length
    length = ((length & 0x0F) << 8) + data[1]
    data = data[2:]
else:
    data = data[1:]
```

---

## Component Types

### Prefix Components (ID 1-2)

| ID | Name | AFI |
|----|------|-----|
| 1 | Destination Prefix | IPv4/IPv6 |
| 2 | Source Prefix | IPv4/IPv6 |

### Numeric Components (ID 3-8, 10-11, 13)

| ID | Name | AFI | Value Size |
|----|------|-----|------------|
| 3 | IP Protocol / Next Header | IPv4/IPv6 | 1 byte |
| 4 | Port (any) | Both | 1-2 bytes |
| 5 | Destination Port | Both | 1-2 bytes |
| 6 | Source Port | Both | 1-2 bytes |
| 7 | ICMP Type | Both | 1 byte |
| 8 | ICMP Code | Both | 1 byte |
| 10 | Packet Length | Both | 1-2 bytes |
| 11 | DSCP / Traffic Class | IPv4/IPv6 | 1 byte |
| 13 | Flow Label | IPv6 only | 1-4 bytes |

### Binary Components (ID 9, 12)

| ID | Name | AFI |
|----|------|-----|
| 9 | TCP Flags | Both |
| 12 | Fragment | Both |

<!-- source: internal/component/bgp/plugins/nlri/flowspec/types.go -- FlowComponentType constants -->

---

## Prefix Component Encoding

### IPv4 Prefix

```
+---------------------------+
|   Component ID (1 octet)  |  1 or 2
+---------------------------+
|   Prefix Length (1 octet) |  In bits
+---------------------------+
|   Prefix (variable)       |  Truncated bytes
+---------------------------+
```

### IPv6 Prefix

```
+---------------------------+
|   Component ID (1 octet)  |  1 or 2
+---------------------------+
|   Prefix Length (1 octet) |  In bits
+---------------------------+
|   Offset (1 octet)        |  IPv6-specific
+---------------------------+
|   Prefix (variable)       |  Truncated bytes
+---------------------------+
```

<!-- source: internal/component/bgp/plugins/nlri/flowspec/types_prefix.go -- prefix component encoding -->

---

## Operator Byte Format

Common to all non-prefix components:

```
 7 6 5 4 3 2 1 0
+-+-+-+-+-+-+-+-+
|E|A|len|  op   |
+-+-+-+-+-+-+-+-+
```

| Bit | Name | Meaning |
|-----|------|---------|
| 7 | E (EOL) | End of list for this component |
| 6 | A (AND) | AND with previous (vs OR) |
| 5-4 | len | Value length: 0=1B, 1=2B, 2=4B, 3=8B |
| 3-0 | op | Operation-specific bits |

### Numeric Operators (op bits 2-0)

| Bits | Operator |
|------|----------|
| 001 | = (equal) |
| 010 | > (greater than) |
| 011 | >= (greater or equal) |
| 100 | < (less than) |
| 101 | <= (less or equal) |
| 110 | != (not equal) |
| 111 | true (always match) |
| 000 | false (never match) |

### Binary Operators (op bits 1-0)

| Bits | Operator |
|------|----------|
| 00 | include (any bit set) |
| 01 | match (exact match) |
| 10 | not (none set) |
| 11 | diff (not exact match) |

<!-- source: internal/component/bgp/plugins/nlri/flowspec/types.go -- FlowOperator type and flags -->
<!-- source: internal/component/bgp/plugins/nlri/flowspec/types_numeric.go -- NumericComponent -->

---

## Component Examples

### Destination Port = 80

```
Component ID: 05
Operator:     81 (EOL=1, len=0, EQ=1)
Value:        50 (80 in decimal)

Wire: 05 81 50
```

### Destination Port = 80 OR 443

```
Component ID: 05
Op1:          01 (EOL=0, len=0, EQ=1)
Value1:       50 (80)
Op2:          81 (EOL=1, len=0, EQ=1)
Value2:       01 BB (443, 2 bytes)

Wire: 05 01 50 91 01 BB
           ^^ 91 = EOL=1, len=1 (2 bytes), EQ=1
```

### TCP Flags SYN

```
Component ID: 09
Operator:     81 (EOL=1, match)
Value:        02 (SYN flag)

Wire: 09 81 02
```

<!-- source: internal/component/bgp/plugins/nlri/flowspec/encode.go -- FlowSpec encoding -->

---

## ExaBGP Implementation

### Flow NLRI

```python
@NLRI.register(AFI.ipv4, SAFI.flow_ip)
@NLRI.register(AFI.ipv6, SAFI.flow_ip)
@NLRI.register(AFI.ipv4, SAFI.flow_vpn)
@NLRI.register(AFI.ipv6, SAFI.flow_vpn)
class Flow(NLRI):
    def __init__(self, packed: Buffer, afi: AFI, safi: SAFI):
        NLRI.__init__(self, afi, safi)
        self._packed = packed
        self._rules_cache: dict[int, list[IComponent]] | None = None

    @property
    def rules(self) -> dict[int, list[IComponent]]:
        if self._rules_cache is None:
            self._rules_cache = self._parse_rules()
        return self._rules_cache

    def add(self, rule: FlowRule) -> bool:
        # Add rule and mark packed as stale
        self.rules.setdefault(rule.ID, []).append(rule)
        self._packed_stale = True
        return True
```

### Component Classes

```python
class IPrefix4(IComponent, FlowIPv4):
    def __init__(self, packed: Buffer):
        self._packed = packed  # [mask][prefix...]

    @property
    def cidr(self) -> CIDR:
        return CIDR.from_ipv4(self._packed)

class FlowDestinationPort(IOperationByteShort, NumericString):
    ID = 0x05
    NAME = 'destination-port'

    def __init__(self, operations: int, value: BaseValue):
        self.operations = operations
        self.value = value
```

---

## JSON Output

FlowSpec JSON uses nested arrays: outer array = OR groups, inner arrays = AND groups.

### Format

```json
{
  "destination-ipv4": [ [ "10.0.0.0/8" ] ],
  "destination-port": [ [ "=80" ], [ "=443" ] ],
  "protocol": [ [ "=6" ] ],
  "tcp-flags": [ [ "=syn" ], [ "=cwr", "!fin", "!ece" ] ],
  "fragment": [ [ "=first-fragment" ], [ "=is-fragment" ] ]
}
```

### Nested Array Semantics

| Structure | Meaning |
|-----------|---------|
| `[["a"], ["b"]]` | a OR b |
| `[["a", "b"]]` | a AND b |
| `[["=syn"], ["=cwr", "!fin"]]` | (match SYN) OR (match CWR AND not FIN) |

### Prefix Notation

| Prefix | Meaning | Applies To |
|--------|---------|------------|
| `=` | Equal / exact match | All (numeric, TCP flags, fragments) |
| `!` | NOT match | TCP flags, fragments |
| `>`, `>=`, `<`, `<=` | Numeric comparison | Ports, protocol, packet-length, etc. |
| `!=` | Not equal | Numeric fields |

### TCP Flags with Multiple Bits

When a single match requires multiple bits set: `=fin+push` (both FIN and PUSH).

```json
{
  "tcp-flags": [ [ "=rst" ], [ "=fin+push" ] ]
}
```

This means: match RST **OR** match (FIN **and** PUSH together).

### Fragment Flags

```json
{
  "fragment": [ [ "=dont-fragment" ], [ "=is-fragment", "!last-fragment" ] ]
}
```

Available: `dont-fragment`, `is-fragment`, `first-fragment`, `last-fragment`.

<!-- source: internal/component/bgp/plugins/nlri/flowspec/types.go -- FlowFragment, FlowTCPFlags constants -->

### Numeric Range Example

```json
{
  "destination-port": [ [ ">8080", "<8088" ], [ "=3128" ] ],
  "source-port": [ [ ">1024" ] ],
  "protocol": [ [ "=tcp" ], [ "=udp" ] ]
}
```

This means: (port > 8080 AND port < 8088) OR (port = 3128).

---

## Ze Implementation Notes

### Rule Parsing

Parse lazily, storing raw bytes:

```go
type Flow struct {
    packed     []byte
    rulesCache map[int][]FlowRule
    stale      bool
}

func (f *Flow) Rules() map[int][]FlowRule {
    if f.rulesCache == nil {
        f.rulesCache = f.parseRules()
    }
    return f.rulesCache
}
```

### Component Order

Rules MUST be ordered by component ID in wire format (RFC requirement):

<!-- source: internal/component/bgp/plugins/nlri/flowspec/types.go -- FlowSpec struct, component ordering -->

```go
func (f *Flow) Pack() []byte {
    var buf bytes.Buffer

    // Sort by component ID
    ids := make([]int, 0, len(f.rules))
    for id := range f.rules {
        ids = append(ids, id)
    }
    sort.Ints(ids)

    for _, id := range ids {
        rules := f.rules[id]
        // Set EOL on last rule
        rules[len(rules)-1].operations |= EOL

        if id != 1 && id != 2 {  // Not prefix
            buf.WriteByte(byte(id))
        }
        for _, rule := range rules {
            buf.Write(rule.Pack())
        }
    }
    return buf.Bytes()
}
```

---

**Last Updated:** 2025-12-19

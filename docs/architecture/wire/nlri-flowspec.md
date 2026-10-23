# FlowSpec NLRI Wire Format (RFC 5575, RFC 8955)

**Source:** ExaBGP `bgp/message/update/nlri/flow.py`
**Family:** AFI 1/2, SAFI 133 (flow_ip) or 134 (flow_vpn)
**Code:** the wire codec (component types, parsing, encoding, precedence, JSON) is
`internal/core/bgp/nlri/flowspec`, which registers nothing; the plugin
`internal/component/bgp/plugins/nlri/flowspec` registers the four families and owns
config and text parsing. The RIB and the FlowSpec firewall read the core codec.

<!-- source: internal/core/family/family.go -- SAFIFlowSpec, SAFIFlowSpecVPN -->
<!-- source: internal/component/bgp/plugins/nlri/flowspec/register.go -- IPv4FlowSpec, IPv6FlowSpec, IPv4FlowSpecVPN, IPv6FlowSpecVPN -->

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

<!-- source: internal/core/bgp/nlri/flowspec/types.go -- FlowSpec struct -->
<!-- source: internal/core/bgp/nlri/flowspec/types_vpn.go -- FlowSpecVPN struct -->

---

## Length Encoding

| Condition | Encoding |
|-----------|----------|
| length < 240 | Single byte |
| length >= 240 | `(0xF0 \| (len >> 8))` + `(len & 0xFF)` |

### Decoding

```python
length = data[0]
if (length & 0xF0) == 0xF0:  # Extended length
    length = ((length & 0x0F) << 8) + data[1]
    data = data[2:]
else:
    data = data[1:]
```

### A malformed NLRI resets the session

The receive path decodes every FlowSpec NLRI of an MP_REACH_NLRI or
MP_UNREACH_NLRI before any plugin sees it, with the parser the plugin's decoder
uses. One the parser refuses (an unknown or repeated component type, a prefix
pattern of the wrong size, an operator list that runs past the NLRI) resets the
session with UPDATE Message Error / Malformed Attribute List. RFC 8955 Section
4.2 sends a malformed NLRI to RFC 7606 error handling, and RFC 7606 Section
3 (j) allows treat-as-withdraw only for an NLRI that parsed. Ze implements no
AFI/SAFI disable, so the session reset is the one choice left. ExaBGP 6.0
raises a NOTIFICATION from the same decode, and BIRD does the same. Before
this check, such an NLRI reached a script as the announce of a route named
`wire[ipv6/flow](49 bytes)`, a route the peer never sent.

<!-- source: internal/component/bgp/plugins/nlri/flowspec/rfc8955_syntax.go -- judgeFlowSpecSection -->
<!-- source: internal/component/bgp/message/rfc7606.go -- validateMPNLRISyntax -->

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
| 13 | Flow Label | IPv6 only | 4 bytes sent, 1-4 accepted |

### Binary Components (ID 9, 12)

| ID | Name | AFI |
|----|------|-----|
| 9 | TCP Flags | Both |
| 12 | Fragment | Both |

<!-- source: internal/core/bgp/nlri/flowspec/types.go -- FlowComponentType constants -->

### Combinations that match no packet

RFC 8955 Section 4.2 allows every combination of components, then names one
that matches nothing: "ICMP Type AND Port" will never match any packets and thus
SHOULD NOT be propagated by BGP. The example is one case of a rule the
component definitions state in full: each component that reads the transport
header "never matches" a packet of another protocol, or a fragment other than
the first (RFC 8955 Sections 4.2.2.4 to 4.2.2.9, RFC 8956 Sections 3.4 and 3.5).

| Component | Matches only |
|-----------|--------------|
| Port, destination port, source port (4, 5, 6) | TCP or UDP |
| ICMP type, ICMP code (7, 8) | ICMP (protocol 1) on IPv4, ICMPv6 (58) on IPv6 |
| TCP flags (9) | TCP |
| Every one of the above | An unfragmented packet or a first fragment |

So Ze refuses a flow route when no packet can satisfy its components together:

- two of those components whose protocols do not meet, such as an ICMP type
  beside a port or beside TCP flags;
- a protocol (3) match that admits none of the protocols they need. The match is
  read as the operator expression it is, so `protocol >=6&<=17` beside a port
  matches (6 and 17 are in range) while `protocol >6&<17` does not;
- a fragment (12) match that admits only fragments other than the first, such as
  `is-fragment` or `last-fragment`, beside any of them;
- a port (4) match that admits no port the destination-port (5) and source-port
  (6) matches beside it admit. A packet matches the port component through its
  destination OR its source port (Section 4.2.2.4), so `port =80` beside
  `destination-port =443` and `source-port =443` matches nothing, while
  `destination-port =80` instead matches.

Ze also refuses a route when one component's own operator list admits no value
a packet can carry, because a packet matches only the intersection of all the
components (Section 4.2):

| Component | Refused when its list admits nothing in |
|-----------|-----------------------------------------|
| Any numeric component | Its own terms: an AND group such as `>100&<50`, or `=80&!=80` |
| Ports (4, 5, 6), packet length (10) on IPv4 | 0 to 65535 |
| Protocol (3), ICMP type and code (7, 8) | 0 to 255 |
| DSCP (11) | 0 to 63, the six-bit field (Section 4.2.2.11) |
| Flow label (13) | 0 to 1048575, the 20-bit field (RFC 8956 Section 3.7) |
| Next header (3) on IPv6 | The values that are not extension headers: RFC 8956 Section 3.3 matches the first Next Header that is not one, so 0, 43, 44, 50, 51 and 60 (RFC 8200 Section 4) never match |
| TCP flags (9) | Every combination of the 16 bits: `=syn&!=syn` admits none |
| Fragment (12) | The four packet states: unfragmented, first fragment (FF), middle fragment (IsF), last fragment (IsF and LF), with or without DF on IPv4. FF beside IsF or LF is no packet |

The fragment bits the RFC marks 0 "MUST be ignored during decoding" (Section
4.2.2.12), and DF is one of them on IPv6 (RFC 8956 Section 3.6), so Ze ignores
those bits rather than read them as a packet that never sets them. Packet length
on IPv6 has no bound, since a jumbogram (RFC 2675) is longer than 65535.
Each list is judged as ranges of values, so the cost grows with the number of
terms rather than the field width.

The refusal comes from config and from `update text`, with an error naming the
matches and the change that makes the route match. The ExaBGP migration and
the bridge hold the component types but not their values, so they refuse the
combinations of types themselves and leave a value contradiction to the
encoder, which judges the route again with its values. The migration drops a
refused route with a warning, as ExaBGP 6.0 does at load, so the rest of the
migrated config still loads. A received route no packet can match is kept in
the peer's RIB but never becomes feasible, so Ze neither installs nor forwards
it (`docs/architecture/plugin/rib-storage-design.md`, "Mandatory FlowSpec
authorization"). ExaBGP 6.0 refuses only the ICMP and port case
(`docs/exabgp/exabgp-differences.md`).
<!-- source: internal/core/bgp/nlri/flowspec/unmatchable.go -- CheckMatchable -->
<!-- source: internal/core/bgp/nlri/flowspec/unmatchable_values.go -- checkComponentValues, checkPortValues -->
<!-- source: internal/component/bgp/plugins/rib/rib_flowspec_validation.go -- reconcileFlowSpecs -->
<!-- source: internal/component/bgp/plugins/nlri/flowspec/config.go -- parseConfigRoute -->
<!-- source: internal/component/bgp/plugins/nlri/flowspec/plugin_encode_text.go -- EncodeFlowSpecComponents -->
<!-- source: internal/exabgp/migration/migrate_routes.go -- flowRouteMatchable -->
<!-- source: internal/exabgp/bridge/bridge_flow.go -- flowRouteMatchable -->

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

The pattern contains `ceil((length-offset)/8)` octets, starting at the first
matched address bit. The native writer zeroes final padding; the decoder ignores
it and reconstructs a masked address with the skipped bits clear. Except for
`length=offset=0`, bounds must satisfy `offset < length <= 128`. Configured
offsets and received wire bounds are checked rather than silently replaced by
zero. Unicast authorization requires a destination offset of zero; the firewall
also refuses nonzero source offsets because its address match is a CIDR.

The builder also rejects a prefix from the wrong address family and multiple
values for one prefix component. A malformed token anywhere in a numeric,
protocol or flag list refuses the complete configured criterion, including
partly readable AND expressions. Type 13 (Flow Label) is accepted only in IPv6.
Empty lists and criteria with no value are rejected before component
construction, so a valid prefix cannot conceal a missing constraint.

<!-- source: internal/component/bgp/plugins/nlri/flowspec/config_builder.go -- buildFlowSpecComponents -->
<!-- source: internal/core/bgp/nlri/flowspec/types.go -- AddComponent and parseFlowComponent -->
<!-- source: internal/component/bgp/plugins/nlri/flowspec/config.go -- flowSpecCriteriaFromContent -->

<!-- source: internal/core/bgp/nlri/flowspec/types_prefix.go -- prefix component encoding -->

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
| 01 | match (all selected bits set) |
| 10 | not (none set) |
| 11 | diff (at least one selected bit clear) |

<!-- source: internal/core/bgp/nlri/flowspec/types.go -- FlowOperator type and flags -->

Decoding ignores the reserved operator bits and the first operator's AND bit.
Numeric operands retain all eight octets when their length code is 3. TCP flag
operands accept one or two octets; DSCP and fragment operands require one.
Fragment operands always occupy one octet on encoding. The enclosing NLRI family
selects the reserved-bit mask: IPv4 permits `0x0f`, while IPv6 permits `0x0e`
(IsF=`0x02`, FF=`0x04`, LF=`0x08`). IPv6 ignores DF and all other reserved bits
on reception and clears them on transmission. Components can be shared between
families without changing their semantic flags.
An operator list without an end-of-list bit is malformed.
<!-- source: internal/core/bgp/nlri/flowspec/types_numeric.go -- numericComponent, parseNumericComponent -->

RFC 8955 recommends one or two octets for destination port (type 5), source port
(type 6) and packet length (type 10), and one octet for ICMP code (type 8).
Ze chooses the shortest encoding: values through 255 emit one octet; 256
through 65535 emit two. For types 5, 6 and 10, using one rather than two octets
for 255 is Ze's policy, not an additional RFC requirement. The Flow Label (type 13)
is the exception to shortest-width: every value is written as four octets, as
RFC 8956 Section 3.7 asks ("Type 13 component values SHOULD be encoded as
4-octet quantities (numeric_op len=10)") and as ExaBGP 6.0 sends it, and a
narrower received operand is reconstructed at four octets. `operandWidth` is the
one width decision `Len` and the writer share; `TestFlowLabelEmitsFourOctets`
pins it. The typed
constructors, generic numeric builder, registered config parser and registered
text NLRI encoder share this writer. These emission **SHOULDs** do not impose
receiving-width **MUSTs**. Reconstructing a parsed component chooses the smallest
width for its value; cached forwarding preserves the received operand bytes.

`TestFlowSpecPreferredNumericEmission` pins the operator and value bytes at the
width boundaries. `TestFlowSpecPreferredNumericWideReception` accepts value 255
in two-, four- and eight-octet operands. These are compatibility controls, not
negative evidence for emission or proof of arbitrary high-bit preservation.
`TestFlowSpecPreferredNumericForwarding` drives registered
text and config producers through the UPDATE builder, receive cache and
forwarding pipeline to a TCP recipient. It also checks the separate `match ...
then ...` route-command encoder for destination/source ports using its bare
numeric grammar. That route-command entry does not support ICMP-code or
packet-length criteria; the registered text NLRI and config entries do.
The forwarding test's authorization lookup is a fixture, not proof of RIB
validation.

<!-- source: internal/component/bgp/plugins/nlri/flowspec/rfc8955_width_test.go -- TestFlowSpecPreferredNumericEmission -->
<!-- source: internal/core/bgp/nlri/flowspec/numeric_wide_reception_test.go -- TestFlowSpecPreferredNumericWideReception -->
<!-- source: internal/core/bgp/nlri/flowspec/types_numeric.go -- operandWidth -->
<!-- source: internal/core/bgp/nlri/flowspec/flow_label_test.go -- TestFlowLabelEmitsFourOctets -->
<!-- source: internal/component/bgp/reactor/rfc8955_width_wire_test.go -- TestFlowSpecPreferredNumericForwarding -->

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

<!-- source: internal/core/bgp/nlri/flowspec/types.go -- FlowFragment, FlowTCPFlags constants -->

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

### Component Order and Uniqueness

RFC 8955 Section 4.2 states two rules about the component list. Components MUST
follow strict type ordering by increasing numerical order, and a given component
type MAY be present exactly once. An NLRI value that breaks either rule is
malformed, and Section 10 sends the error to RFC 7606 handling.

Ze holds both rules in the FlowSpec model rather than at the wire boundary, so a
FlowSpec cannot carry two components of one type.

<!-- source: internal/core/bgp/nlri/flowspec/types.go -- AddComponent, parseComponents, writeComponentsSorted -->

| Direction | Producer | Behavior |
|-----------|----------|----------|
| Build | `FlowSpec.AddComponent` | A component whose type is already present is merged into the one already held. Its operator list becomes a new OR group, and the leading AND bit of that group is cleared |
| Build | `FlowSpec.AddComponent` | A second prefix component (Type 1 or 2) carries no operator list to join, so it is refused with `ErrFlowSpecDuplicateType` |
| Emit | `FlowSpec.writeComponentsSorted` | Walks type IDs 1 to 13 and writes the one component of each type present, so the emitted list ascends |
| Decode | `FlowSpec.parseComponents` | Compares each component type against the previous one, refusing a repeat with `ErrFlowSpecDuplicateType` and a descent with `ErrFlowSpecTypeOrder` |

An OR of AND groups on one type is therefore ONE component. The text form
`port >80&<100 port >443&<500` repeats the keyword and emits a single Type 4
component holding both groups:

```
0b                 NLRI value length, 11 octets
04                 Type 4 (Port)
02 50              len=1, gt             >80,  AND bit clear
44 64              len=1, AND, lt        <100
12 01 bb           len=2, gt             >443, AND bit clear, second OR group
d4 01 f4           len=2, end, AND, lt   <500
```

A peer that receives two Type 4 components instead reads the UPDATE as malformed.
GoBGP v4.10.0 logs "ipv4-flowspec nlri violate strict type ordering" and resets the
session (`pkg/packet/bgp/validate.go`, `ValidateUpdateMsg`). <!-- doc-links: ignore (GoBGP v4.10.0 source in github.com/osrg/gobgp, outside this repository) -->

### Packet-filter precedence

<!-- source: internal/core/bgp/nlri/flowspec/compare.go -- Compare -->
<!-- source: internal/plugins/flowspec-firewall/state.go -- ruleMap.buildTable -->

The firewall orders complete FlowSpec rules with the RFC 8955 Section 5.1
component comparison. More-specific overlapping prefixes precede their covering
prefixes. Disjoint prefixes use address order. Numeric and bitmask components
use binary comparison, with the longer component first when the common bytes
are equal. Expanded terms from one rule remain together.

IPv6 prefixes compare their offsets first, with the lower offset taking
precedence, then use the same prefix comparison. Received numeric and bitmask
bytes retain their operand widths for ordering even when a retransmission
uses a shorter encoding. A rule's length octets frame the NLRI and do not
change its identity on replacement or withdrawal.

<!-- source: internal/plugins/flowspec-firewall/selected.go -- handleSelected -->
<!-- source: internal/core/bgp/nlri/nlrisplit/prefix_key.go -- keyFlowSpec -->

RFC 8955 obsoletes RFC 5575. Its next-hop, VPN length and ordering requirements
remain binding. RFC 9117 updates validation; RFC 8956 adds IPv6 component
semantics, and RFC 9184 updates registry names.

---

**Last Updated:** 2026-08-30

# Spec: New API Syntax

## Task

Refactor announce API to use family-first syntax with per-route override support.
Remove `announce route` and `announce attributes` commands.

## Required Reading (MUST complete before implementation)

- [x] `.claude/zebgp/api/ARCHITECTURE.md` - API command structure
- [x] `.claude/zebgp/UPDATE_BUILDING.md` - UPDATE message building
- [x] `.claude/zebgp/config/SYNTAX.md` - Config syntax patterns

**Key insights from docs:**
- API routes use Build path (local origination), not Forward path
- Per-peer encoding via PackContext (iBGP/eBGP, ASN4, ADD-PATH)
- Existing family handlers: `announce ipv4/unicast`, `announce ipv6/unicast`, etc.

## Design

### Unified Update Syntax

API and config share the same structure - encoding before attributes:

```
update <encoding> [<attr-name> <set|add|del> [<value>]]... [nhop <set|del> [<next-hop>]]... [nlri <family> [add <nlri>...] [del <nlri>...]]... [watchdog <name>]
```

**Text mode** - per-attribute operations:
- `origin set igp` - set origin attribute
- `med set 100` - set MED attribute
- `med del` - remove MED attribute
- `community set [65000:1]` - replace community list
- `community add [65000:2]` - prepend to community list
- `community del` - remove entire community attribute
- `community del [65000:1]` - remove first occurrence of specific community
- Same list pattern for: `large-community`, `extended-community`
- `local-preference set 100` - set local preference (scalar, like med)
- `as-path set [65000 65001]` - replace AS path
- `as-path add [65000]` - prepend ASN(s) to AS path (nearest first)
- `as-path del` - remove entire AS path attribute
- `as-path del [65000]` - remove first occurrence of specific ASN from AS path

**Wire mode** (hex/b64) - all attributes as blob:
- `attr set <wire-bytes>` - set all attributes from encoded bytes (only `set` supported)

**Next-hop:**
- `nhop set <addr>` - set next-hop (text=IP address, hex/b64=encoded bytes)
- `nhop set self` - use local address as next-hop
- `nhop del` - unset accumulated next-hop (error at send time if no next-hop for announce)
- `nhop del <addr>` - unset only if current next-hop matches (no-op otherwise)

**NLRI:**
- `nlri <family> add <prefix>...` - announce prefixes
- `nlri <family> del <prefix>...` - withdraw prefixes
- `nlri <family> add <prefix>... del <prefix>...` - both in one section
- At least one of `add` or `del` required

Multiple attribute, nhop, and nlri sections allowed - accumulate and apply to subsequent nlri sections.

### Encodings

| Encoding | Attributes | Next-Hop | NLRI | Use case |
|----------|------------|----------|------|----------|
| `text` | Parsed (origin, med, ...) | IP address | Prefixes (1.0.0.0/24) | Human-readable |
| `hex` | Hex wire bytes | Hex bytes | Hex wire bytes | Debug |
| `b64` | Base64 wire bytes | Base64 bytes | Base64 wire bytes | Compact |

### API Command Format

```
peer <addr> update <encoding> [<attr-name> <set|add|del> [<value>]]... [nhop <set|del> [<next-hop>]]... [nlri <family> [add <nlri>...] [del <nlri>...]]... [watchdog <name>]
```

**Examples (text mode):**
```bash
peer 10.0.0.1 update text origin set igp med set 100 nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24
peer 10.0.0.1 update text community set [65000:1 65000:2] nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24
peer 10.0.0.1 update text nlri ipv4/unicast del 1.0.0.0/24  # withdraw only
```

**Examples (wire mode):**
```bash
peer 10.0.0.1 update hex attr set 400101... nhop set 0a000001 nlri ipv4/unicast add 180a0000
peer 10.0.0.1 update b64 attr set QAEB... nhop set CgAAAQ== nlri ipv4/unicast add GAAK...
```

### Config File Format

```
update {
    <encoding> {
        <name> {
            <attr-name> <set|add|del> [<value>];
            ...
            nhop <set|del> [<next-hop>];
            nlri <family> [<nlri-modifier>...] [add <nlri>...] [del <nlri>...];
            ...
        }
        <name> { ... }  # Each block starts with clean state
    }
}
```

- `<name>` = user-defined identifier (`[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*`), ignored by config, for organization
- Block names don't need to be unique
- Each block starts with clean state (no inherited attributes)
- `<attr> set <value>` = replace value (e.g., `origin set igp;`)
- `<attr> add <value>` = prepend to list (e.g., `community add [65000:2];`)
- `<attr> del` = remove attribute entirely
- `<attr> del <value>` = remove specific from list (e.g., `community del [65000:1];`)
- `nhop set <addr>` = set next-hop
- `nhop set self` = use local address as next-hop
- `nhop del` = remove accumulated next-hop
- `nhop del <addr>` = remove only if matches (no-op otherwise)
- `nlri` uses current accumulated attributes and nhop

**Example (text):**
```
update {
    text {
        primary-routes {
            origin set igp;
            community set [65000:1 65000:2];
            nhop set 10.0.0.1;

            nlri ipv4/unicast add 1.0.0.0/24 2.0.0.0/24;

            community add [65000:3];
            nlri ipv4/unicast add 3.0.0.0/24;

            community del [65000:1];
            nlri ipv4/unicast add 4.0.0.0/24 del 5.0.0.0/24;
        }

        backup-routes {
            nhop set 10.0.0.2;
            med set 200;

            nlri ipv4/unicast add 6.0.0.0/24;
        }
    }
}
```

**Example (hex):**
```
update {
    hex {
        batch-1 {
            attr set 400101400206020100001f94;
            nhop set 05060708;
            # Spaces help track NLRI boundaries for UPDATE size splitting
            nlri ipv4/unicast add 180a0000 180b0000;  # 10.0.0.0/24, 11.0.0.0/24
        }
        withdrawals {
            # Withdrawals don't need attributes or next-hop
            nlri ipv4/unicast del 180c0000;  # 12.0.0.0/24
        }
    }
}
```

### Per-NLRI Overrides

Within an `nlri` section, attributes and nhop can be overridden. Overrides apply to all NLRIs in that section.

```bash
# Override nhop within nlri section (all modes)
peer 10.0.0.1 update text origin set igp nhop set 10.0.0.1 \
    nlri ipv4/unicast nhop set 10.0.0.2 add 1.0.0.0/24 2.0.0.0/24

# Wire mode nhop override
peer 10.0.0.1 update hex attr set 400101... nhop set 0a000001 \
    nlri ipv4/unicast nhop set 0a000002 add 180a0000

# Override attributes within nlri section (text mode only)
peer 10.0.0.1 update text origin set igp community set [65000:1] nhop set 10.0.0.1 \
    nlri ipv4/unicast community add [65000:2] add 1.0.0.0/24
```

**Rules:**
- `nhop set <addr>` inside nlri overrides accumulated nhop for that section (all modes)
- `nhop del` inside nlri removes accumulated nhop (error at send time if announce needs nhop)
- `<attr> set/add/del` inside nlri overrides accumulated attrs for that section (text mode only)
- List attributes support: `set` (replace), `add` (prepend), `del` (remove all), `del <value>` (remove specific)
- Scalar attributes support: `set` (replace), `del` (remove)
- Overrides apply to ALL NLRIs in that section (not per-prefix)

### Chained Attribute/NLRI Sections

Multiple attribute, nhop, and nlri sections can be chained. Attributes and nhop accumulate and apply to subsequent `nlri`:

```bash
peer 10.0.0.1 update text \
    origin set igp \
    community set [65000:1 65000:2] \
    nhop set 10.0.0.1 \
    nlri ipv4/unicast add 1.0.0.0/24 2.0.0.0/24 \
    community add [65000:3] \
    nlri ipv4/unicast add 3.0.0.0/24 \
    community del [65000:1] \
    nlri ipv4/unicast add 4.0.0.0/24 del 5.0.0.0/24
```

Result:
- `1.0.0.0/24`, `2.0.0.0/24` → nhop 10.0.0.1, origin igp, community [65000:1 65000:2]
- `3.0.0.0/24` → nhop 10.0.0.1, origin igp, community [65000:3 65000:1 65000:2] (65000:3 prepended)
- `4.0.0.0/24` → nhop 10.0.0.1, origin igp, community [65000:3 65000:2] (65000:1 removed)
- `5.0.0.0/24` → withdraw

### Common Parser

API and config share the same parser with format-specific tokenizer:

```
Tokenizer (format-specific)
    ├── API: split on whitespace, newline terminates
    └── Config: split on whitespace + { } ;
            ↓
Common Token Stream: [update, hex, origin, set, igp, nlri, ipv4/unicast, add, 1.0.0.0/24...]
            ↓
Shared Parser: parseUpdate() → parseEncoding() → parseAttrOrNlri() → ...
            ↓
         AST / Command struct
```

| Aspect | API | Config |
|--------|-----|--------|
| Blocks | implicit | `<name> { }` (name ignored) |
| Terminator | newline | `;` |
| Route groups | single command | multiple named blocks |
| Peer selector | `peer 10.0.0.1` | neighbor block context |

### Grammar (API)

```
<command>       := peer <addr> update <encoding> <sections>
<sections>      := <section>...
<section>       := <scalar-attr> | <list-attr> | <nlri-section> | <wire-attr>

# Scalar attributes (set/del with optional conditional value)
<scalar-attr>   := <scalar-name> (set <value> | del [<value>])
<scalar-name>   := origin | med | local-preference | nhop | path-information | rd | label

# List attributes (set/add/del)
<list-attr>     := <list-name> (set <list> | add <list> | del [<list>])
<list-name>     := as-path | community | large-community | extended-community

# NLRI section with optional watchdog tagging
<nlri-section>  := nlri <family> <nlri-op>+
<nlri-op>       := add <nlri>+ [watchdog set <name>] | del <nlri>+

# Wire mode (hex/b64 only)
<wire-attr>     := attr (set <bytes> | del [<bytes>])

<encoding>      := text | hex | b64
<family>        := ipv4/unicast | ipv4/mpls-vpn | ipv4/nlri-mpls | ipv4/mup
                |  ipv6/unicast | ipv6/mpls-vpn | ipv6/nlri-mpls | ipv6/mup
<nlri>          := <prefix> (text) | <encoded-bytes> (hex/b64)
```

### Standalone Watchdog Commands

```
watchdog announce <name>   # send all routes in pool to peers
watchdog withdraw <name>   # withdraw all routes in pool from peers
```

### Scalar `del [<value>]` Semantics

- `<scalar> del` - remove attribute unconditionally
- `<scalar> del <value>` - remove only if current value matches, else error

### Accumulator → Family Restrictions

| Accumulator | Valid for | Error for |
|-------------|-----------|-----------|
| `rd` | `*-vpn` families | All others |
| `label` | `*-vpn`, `*-labeled` families | All others |
| `path-information` | Any (if ADD-PATH negotiated) | Ignored if not negotiated |

### Grammar (Config)

```
<update-block>  := update { <encoding-block>... }
<encoding-block>:= <encoding> { <route-block>... }
<route-block>   := <block-name> { <statement>... }
<block-name>    := [a-zA-Z0-9]+(-[a-zA-Z0-9]+)*            # ignored, for organization
<statement>     := <section> ;
<section>       := (same as API grammar above)
```

### Raw Passthrough Commands

"Trust me bro" mode - send raw bytes with no validation.

| Command | What's sent | Header |
|---------|-------------|--------|
| `peer X raw <type> <enc> <data>` | Message payload | ZeBGP adds |
| `peer X raw <enc> <data>` | Full packet | User provides FF*16 |

**Grammar:**
```
<raw-command> := peer <addr> raw [<msg-type>] <encoding> <data>
<msg-type>    := open | update | notification | keepalive | route-refresh
<encoding>    := hex | b64
<data>        := <encoded-bytes>
```

**Examples:**
```bash
# Message payload only (ZeBGP adds 19-byte BGP header: 16 marker + 2 length + 1 type)
peer 10.0.0.1 raw update hex 0000000e40010100400200400304c0a80101180a00
peer 10.0.0.1 raw notification hex 0602
peer 10.0.0.1 raw notification b64 BgI=
peer 10.0.0.1 raw keepalive hex       # empty payload OK
peer 10.0.0.1 raw open hex 04ffdc...

# Full packet (user provides FF*16 marker + length + type)
peer 10.0.0.1 raw hex ffffffffffffffffffffffffffffffff001303
peer 10.0.0.1 raw b64 //////////8AAAAAAAAAAAAAAAATAQ==
```

**Comparison with `update`:**

| Aspect | `update` | `raw` |
|--------|----------|-------|
| Purpose | Build UPDATE | Send bytes |
| Parsing | Full (attr, nlri, family) | None |
| Validation | Yes | No |
| Message types | UPDATE only | Any + full packet |

**Validation:** None. Bytes sent exactly as provided.

⚠️ **Risks:**
- Can crash peer
- Can violate FSM state
- Can send malformed messages
- Use for testing/debugging only

### Family Validation

- `nlri ipv4/*` rejects IPv6 prefixes (parsed mode)
- `nlri ipv6/*` rejects IPv4 prefixes (parsed mode)
- Wire bytes mode: no validation (peer's responsibility)

### Next-Hop Validation

**At send time:**
- Announce without next-hop → error, entire block rejected
- IPv4 next-hop for IPv6 NLRI → requires Extended Next Hop capability (RFC 5549)
  - If not negotiated → error, entire block rejected
  - Error: `extended next-hop not negotiated for IPv6 with IPv4 next-hop`

### Examples (text)

```bash
# Simple announce
peer 10.0.0.1 update text nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24

# Multiple routes with attributes
peer 10.0.0.1 update text origin set igp community set [65000:1] nhop set 10.0.0.1 \
    nlri ipv4/unicast add 1.0.0.0/24 2.0.0.0/24 3.0.0.0/24

# Withdraw only
peer 10.0.0.1 update text nlri ipv4/unicast del 1.0.0.0/24

# Announce and withdraw
peer 10.0.0.1 update text nhop set 10.0.0.1 \
    nlri ipv4/unicast add 1.0.0.0/24 del 2.0.0.0/24

# IPv6
peer 10.0.0.1 update text nhop set 2001::1 nlri ipv6/unicast add 2001:db8::/32

# VPN with RD/label (NLRI modifiers, not path attributes)
peer 10.0.0.1 update text extended-community set [target:65000:100] nhop set 10.0.0.1 \
    nlri ipv4/mpls-vpn rd 65000:100 label 1000 add 10.0.0.0/24 10.0.1.0/24

# Multiple families (same nhop - requires Extended Next Hop capability RFC 5549)
# IPv4 next-hop for IPv6 NLRI only works if peer negotiated Extended NH
peer 10.0.0.1 update text nhop set 10.0.0.1 \
    nlri ipv4/unicast add 1.0.0.0/24 \
    nlri ipv6/unicast add 2001:db8::/32

# Multiple families (different nhop per family - normal case)
peer 10.0.0.1 update text nhop set 10.0.0.1 \
    nlri ipv4/unicast add 1.0.0.0/24 \
    nhop set 2001:db8::1 nlri ipv6/unicast add 2001:db8::/32

# Chained attribute sections (attributes accumulate)
peer 10.0.0.1 update text \
    origin set igp community set [65000:1 65000:2] nhop set 10.0.0.1 \
    nlri ipv4/unicast add 1.0.0.0/24 \
    community add [65000:3] \
    nlri ipv4/unicast add 2.0.0.0/24 \
    community del [65000:1] \
    nlri ipv4/unicast add 3.0.0.0/24

# With watchdog
peer 10.0.0.1 update text nhop set 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24 watchdog mygroup

# Conditional nhop del (only removes if matches, no-op otherwise)
peer 10.0.0.1 update text nhop set 10.0.0.1 \
    nlri ipv4/unicast add 1.0.0.0/24 \
    nhop del 10.0.0.1 \
    nhop set 10.0.0.2 \
    nlri ipv4/unicast add 2.0.0.0/24
# Result: 1.0.0.0/24 → nhop 10.0.0.1, 2.0.0.0/24 → nhop 10.0.0.2
```

### Examples (Wire Bytes)

```bash
# Hex encoding - spaces optional, concatenated as raw wire bytes
# Spaces help track NLRI boundaries for UPDATE size management
# (max 4096 bytes standard, 65535 with Extended Message RFC 8654, includes 19-byte header)
peer 10.0.0.1 update hex attr set 400101400206020100001f94 nhop set 0a000001 nlri ipv4/unicast add 180a0000 180b0000
# Or as single blob:
peer 10.0.0.1 update hex attr set 400101400206020100001f94 nhop set 0a000001 nlri ipv4/unicast add 180a0000180b0000

# Base64 encoding
peer 10.0.0.1 update b64 attr set QAEBQAIGAgEAAAH5 nhop set CgAAAQ== nlri ipv4/unicast add GAAKAAoA

# Announce and withdraw
peer 10.0.0.1 update hex attr set 400101400206020100001f94 nhop set 0a000001 \
    nlri ipv4/unicast add 180a0000 del 180b0000

# Multiple families (nhop changes per family)
peer 10.0.0.1 update hex attr set 400101400206020100001f94 \
    nhop set 0a000001 nlri ipv4/unicast add 180a0000 \
    nhop set 20010db8000000000000000000000001 nlri ipv6/unicast add 402001db80000000000000000000000000
```

## Removed Commands

| Command | Migration |
|---------|-----------|
| `announce route <prefix> next-hop <nh>` | `peer * update text nhop set <nh> nlri ipv4/unicast add <prefix>` |
| `announce attributes origin igp ... nlri ...` | `peer * update text origin set igp ... nhop set <nh> nlri ipv4/unicast add ...` |
| `withdraw route <prefix>` | `peer * update text nlri ipv4/unicast del <prefix>` |
| `announce watchdog <name>` | `watchdog announce <name>` |
| `withdraw watchdog <name>` | `watchdog withdraw <name>` |

## Watchdog Commands

| Command | Purpose |
|---------|---------|
| `watchdog announce <name>` | Send all routes in pool to peers |
| `watchdog withdraw <name>` | Withdraw all routes in pool from peers |

Routes are tagged with a pool when announced:
```bash
update text nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24 watchdog set mypool
```

## Validation

### Text Mode
- Attributes validated (valid origin, med range, etc.)
- NLRI prefix validated against family
- Peer must be established

### Wire Bytes Mode (hex/b64)
- Wire bytes must decode successfully (valid hex/b64)
- No validation of attribute flags or structure
- NLRI format not validated (peer's responsibility)
- Peer must be established
- Only `attr set` supported (not add/del - can't manipulate raw bytes)

### Error Messages

| Condition | Message |
|-----------|---------|
| Invalid encoding | `invalid hex/b64 in attributes: <error>` |
| Peer not found | `peer not found: 10.0.0.1` |
| Peer not established | `peer not established: 10.0.0.1` |
| Invalid family | `invalid family: <family>` |
| Empty nlri section | `nlri section requires 'add' and/or 'del' with prefixes` |
| Missing nlri keyword | `missing nlri keyword after attributes` |
| Missing next-hop | `missing next-hop for announce` |
| Extended NH not negotiated | `extended next-hop not negotiated for IPv6 with IPv4 next-hop` |
| IPv6 prefix in ipv4/* | `prefix family mismatch: 2001:db8::/32 is IPv6, expected IPv4` |
| IPv4 prefix in ipv6/* | `prefix family mismatch: 10.0.0.0/24 is IPv4, expected IPv6` |
| Scalar attr with add | `'med' is a scalar attribute: use 'set' or 'del'` |
| Wire mode attr add/del | `wire mode only supports 'attr set': cannot use add/del on raw bytes` |

## Wire Bytes Output Format (Receive)

### Two Modes: Forward-Only vs Payload

API programs can operate in two modes for handling wire bytes:

| Mode | Format | Use Case |
|------|--------|----------|
| **Forward-only** | `parsed` | Route Server - just forward via msg-id |
| **Payload** | `raw` or `full` | Persist - store in own pool for long-term |

**Forward-only (`format parsed`):** API uses `msg-id` to forward, relies on engine cache.
```
# API receives event with msg-id, no wire bytes
# API forwards: peer !A forward update-id 123
# API retains:  msg-id 123 retain
```

**Payload (`format raw` or `full`):** API receives wire bytes, stores in own pool.
```
# API receives event with wire bytes
# API stores in pool for long-term storage
# API can replay even after engine cache expired
```

### Wire Encoding Control

API programs control wire bytes encoding via session commands:

```
session api encoding inbound <format>   # Events from engine
session api encoding outbound <format>  # Commands to engine
session api encoding <format>           # Both directions
```

**Formats:**

| Format | Overhead | Default | Notes |
|--------|----------|---------|-------|
| `text` | N/A | | Parsed attributes and prefixes |
| `hex` | 100% | ✅ Yes | Human-readable wire bytes |
| `b64` | 33% | | Compact wire bytes |

**Default is `hex`** for maximum compatibility and debuggability.

**Examples:**
```bash
# Simple Python script (easy debugging)
session api encoding hex

# Production Python (smaller payload)
session api encoding b64

# Mixed: debug inbound, efficient outbound
session api encoding inbound hex
session api encoding outbound b64
```

### Output Format

Encoding keyword is self-describing:

```
peer <addr> received <msg-id> update <encoding> [<attr-name> <set|add|del> [<value>]]... [nlri <family> nhop set <next-hop> [add <nlri>...] [del <nlri>...]]...
```

Note: Output places `nhop` inside each `nlri` section (explicit per-family).

### Example Output (text)

```json
{
  "type": "update",
  "direction": "received",
  "msg-id": 123,
  "peer": {"address": "10.0.0.1", "asn": 65001},
  "attr": {
    "encoding": "text",
    "origin": "igp",
    "as-path": [65001, 65002],
    "med": 100
  },
  "nlri": {
    "ipv4/unicast": {
      "nhop": "10.0.0.1",
      "add": ["10.0.0.0/24", "10.0.1.0/24"],
      "del": ["10.0.2.0/24"]
    }
  }
}
```

### Attribute Encoding (wire bytes)

- **Included:** All path attributes in wire order
- **Excluded:** MP_REACH_NLRI (type 14), MP_UNREACH_NLRI (type 15)
- NLRI extracted and encoded separately by family

### Example Output (hex - default)

```json
{
  "type": "update",
  "direction": "received",
  "msg-id": 123,
  "peer": {"address": "10.0.0.1", "asn": 65001},
  "attr": {
    "encoding": "hex",
    "data": "400101400206020100001f94"
  },
  "nlri": {
    "ipv4/unicast": {
      "nhop": "0a000001",
      "add": ["180a0000"],
      "del": ["180b0000"]
    }
  }
}
```

### Example Output (b64)

```json
{
  "type": "update",
  "direction": "received",
  "msg-id": 123,
  "peer": {"address": "10.0.0.1", "asn": 65001},
  "attr": {
    "encoding": "b64",
    "data": "QAEBQAIGAgEAAAH5"
  },
  "nlri": {
    "ipv4/unicast": {
      "nhop": "CgAAAQ==",
      "add": ["GAAKAAoA"]
    }
  }
}
```

**Design:** All output formats (text, JSON) place nhop inside each family. Self-contained, simpler parsing.

### Text Format

```
peer 10.0.0.1 received 123 update text med set 100 nlri ipv4/unicast nhop set 10.0.0.1 add 10.0.0.0/24
peer 10.0.0.1 received 123 update hex attr set 400101... nlri ipv4/unicast nhop set 0a000001 add 180a0000 del 180b0000
peer 10.0.0.1 received 123 update b64 attr set QAEB... nlri ipv4/unicast nhop set CgAAAQ== add GAAK...
```

### Input vs Output Format

| Direction | nhop placement | Reason |
|-----------|----------------|--------|
| **Input** (commands/config) | Outside nlri, accumulates | Convenience - set once, apply to multiple families |
| **Output** (events/printing) | Inside nlri section | Explicit per-family, easier parsing |

**Input example:**
```bash
peer X update text nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24 nlri ipv6/unicast add 2001:db8::/32
```

**Output example:**
```
peer X received 123 update text nlri ipv4/unicast nhop set 10.0.0.1 add 1.0.0.0/24
```

### Family Support

| Family | Raw NLRI Support | Notes |
|--------|------------------|-------|
| ipv4/unicast | ✅ | Standard NLRI field |
| ipv6/unicast | ✅ | From MP_REACH_NLRI |
| ipv4/mpls-vpn | ✅ | From MP_REACH_NLRI |
| ipv6/mpls-vpn | ✅ | From MP_REACH_NLRI |
| ipv4/nlri-mpls | ✅ | From MP_REACH_NLRI |
| ipv6/nlri-mpls | ✅ | From MP_REACH_NLRI |
| flowspec | ❌ | Complex encoding, use parsed format |

### Configuration

Use existing `format` setting to control wire bytes:

```
api reflector {
    content {
        format raw;   # Wire bytes only
        format full;  # Both parsed and wire bytes
    }
    receive {
        update;
    }
}
```

| Format | Wire bytes | Parsed |
|--------|------------|--------|
| `parsed` | ❌ | ✅ (default) |
| `raw` | ✅ | ❌ |
| `full` | ✅ | ✅ |

Wire bytes encoding controlled at runtime via `session api encoding` command.

## Files to Modify

### Core Changes

| File | Change |
|------|--------|
| `internal/plugin/route.go` | Remove `announce route`, `announce attributes` handlers |
| `internal/plugin/route.go` | Update family handlers to use new NLRI group parser |
| `internal/plugin/route.go` | Add `handleAnnounceRaw()` for raw wire bytes announce |
| `internal/plugin/route_parse.go` | Add `parseNLRIGroups()`, `applyOverrides()` |
| `internal/plugin/route_parse_test.go` | Tests for new parser |
| `internal/plugin/json.go` | Add `formatRawUpdate()` for wire bytes output |
| `internal/plugin/text.go` | Add raw text format output |
| `internal/plugin/session.go` | Add `session api encoding` handlers |
| `internal/plugin/types.go` | Add `WireEncoding` field to `ContentConfig` |
| `internal/plugin/process.go` | Track per-process wire encoding setting |

### Test Updates (~48 files)

| Pattern | Count | Change |
|---------|-------|--------|
| `test/data/api/*.run` | ~30 | Update `announce route` → `update text nhop set ... nlri ipv4/unicast add ...` |
| `test/data/encode/*.ci` | ~20 | Update expected command format |

### Documentation

| File | Change |
|------|--------|
| `.claude/zebgp/api/ARCHITECTURE.md` | Update command reference |
| `.claude/zebgp/api/COMMANDS.md` | Update command syntax |

## Current State

- Tests: `make test` PASS, `make lint` 0 issues, `make functional` 79/79
- Last commit: `4b6744c` docs(api): update JSON_FORMAT.md

## Implementation Steps

### Phase 1: Parser (TDD)

1. Write test for `parseNLRIGroups()` → MUST FAIL
2. Implement `parseNLRIGroups()` → MUST PASS
3. Write test for `applyOverrides()` with add/set → MUST FAIL
4. Implement `applyOverrides()` → MUST PASS
5. Write test for family validation → MUST FAIL
6. Implement family validation → MUST PASS

### Phase 2: Handler Updates

1. Update `handleAnnounceIPv4Unicast()` to use new parser
2. Update other family handlers
3. Add `handleAnnounceRaw()` for wire bytes announce
4. Remove `handleAnnounceRoute()`
5. Remove `handleAnnounceAttributes()`
6. Remove unused helpers (`announceRouteImpl`, `parseAttributesNLRI`)

### Phase 3: Wire Encoding (TDD)

1. Write test for `session api encoding hex` → MUST FAIL
2. Implement encoding session handlers → MUST PASS
3. Write test for hex wire bytes output → MUST FAIL
4. Implement hex encoder → MUST PASS
5. Write test for b64 wire bytes output → MUST FAIL
6. Implement b64 encoder → MUST PASS

### Phase 4: Test Migration

1. Create migration script for .run files
2. Run script, verify changes
3. Update .ci expected outputs
4. Run `make functional` → MUST PASS

### Phase 5: Documentation

1. Update `.claude/zebgp/api/ARCHITECTURE.md`
2. Update `.claude/zebgp/api/COMMANDS.md` (if exists)

## Checklist

### Parser
- [x] Test fails first (parseNLRIGroups) - impl: `ParseUpdateText()` in update_text.go
- [x] Test passes after impl - 50+ tests in update_text_test.go
- [x] Test fails first (applyOverrides) - impl: `applySet/applyAdd/applyDel` methods
- [x] Test passes after impl
- [x] Test fails first (family validation)
- [x] Test passes after impl

### Handlers
- [ ] Test fails first (handleUpdateHex) - stub returns "not yet implemented"
- [ ] Test passes after impl
- [ ] Test fails first (handleUpdateB64) - stub returns "not yet implemented"
- [ ] Test passes after impl
- [x] handleUpdateText() complete
- [ ] Old handlers removed - `handleAnnounceRoute`, `handleAnnounceAttributes` still registered

### Wire Encoding
- [x] Test fails first (session api encoding)
- [x] Test passes after impl - tests in session_test.go
- [x] Per-process encoding tracking (wireEncodingIn/Out in process.go)
- [ ] Test fails first (hex encoder for UPDATE output)
- [ ] Test passes after impl
- [ ] Test fails first (b64 encoder for UPDATE output)
- [ ] Test passes after impl

### Integration
- [ ] .run files migrated - 0/32 files, migration script in spec not executed
- [ ] .ci files updated
- [x] `make test` passes
- [x] `make lint` passes
- [x] `make functional` passes (using old API syntax)
- [ ] Documentation updated

## Current Status (2026-01-07)

| Phase | Status | Notes |
|-------|--------|-------|
| Phase 1: Parser | ✅ Complete | `ParseUpdateText()` + attribute accumulation |
| Phase 2: Handlers | ⚠️ Partial | text works, hex/b64 stubs, old handlers remain |
| Phase 3: Wire Encoding | ⚠️ Partial | session tracking done, output formatters missing |
| Phase 4: Test Migration | ❌ Not started | 32 .run files need migration |

**Next steps:**
1. Implement `handleUpdateHex()` / `handleUpdateB64()`
2. Remove old `handleAnnounceRoute()` / `handleAnnounceAttributes()`
3. Migrate .run test files using migration script
4. Add wire bytes output formatters for JSON/text responses

## Migration Script

```python
#!/usr/bin/env python3
"""Migrate .run files from announce route to family-first syntax.

Old syntax: announce route <prefix> next-hop <nh> [origin <type>] ...
New syntax: update text [origin set <type>] ... nhop set <nh> nlri <family> add <prefix>

Note: Complex attribute migrations require manual review.
"""

import re
import sys

def migrate_line(line: str) -> str:
    # announce route <prefix> next-hop <nh> ...
    # → update text nhop set <nh> ... nlri <family> add <prefix>
    m = re.match(r"(.*)announce route (\S+) next-hop (\S+)(.*)", line)
    if m:
        pre, prefix, nh, rest = m.groups()
        family = "ipv6/unicast" if ":" in prefix else "ipv4/unicast"
        # Convert simple attributes if present
        rest = re.sub(r'\borigin (\w+)', r'origin set \1', rest)
        rest = re.sub(r'\bmed (\d+)', r'med set \1', rest)
        rest = re.sub(r'\blocal-preference (\d+)', r'local-preference set \1', rest)
        return f"{pre}update text{rest} nhop set {nh} nlri {family} add {prefix}"

    # announce attributes ... nlri ...
    # → update text <attr> set <value> ... nhop set <nh> nlri <family> add ...
    m = re.match(r"(.*)announce attributes (.*)", line)
    if m:
        pre, rest = m.groups()
        # Detect family from nlri prefixes
        if ":" in rest.split("nlri")[-1]:
            family = "ipv6/unicast"
        else:
            family = "ipv4/unicast"
        # Convert simple attributes
        rest = re.sub(r'\borigin (\w+)', r'origin set \1', rest)
        rest = re.sub(r'\bmed (\d+)', r'med set \1', rest)
        rest = re.sub(r'\blocal-preference (\d+)', r'local-preference set \1', rest)
        # Note: community [...], next-hop, etc. need manual review
        return f"{pre}update text {rest}"

    return line

if __name__ == "__main__":
    for line in sys.stdin:
        print(migrate_line(line), end="")
```

---

**Created:** 2026-01-04
**Updated:** 2026-01-07

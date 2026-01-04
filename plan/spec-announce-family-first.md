# Spec: Announce Family-First Refactor

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

API and config share the same structure - encoding before attr:

```
update <encoding> [attr <set|add|del> <attributes>]... [nlri <family> add <nlri>... [del <nlri>...]]...
```

- `attr set` - replace all attributes
- `attr add` - add to list attributes (community, large-community, extended-community)
- `attr del` - remove from list attributes
- Multiple `attr` and `nlri` sections allowed - attributes accumulate and apply to subsequent `nlri` sections

### Encodings

| Encoding | Attributes | NLRI | Use case |
|----------|------------|------|----------|
| `text` | Parsed (next-hop, med, ...) | Prefixes (1.0.0.0/24) | Human-readable |
| `hex` | Hex wire bytes | Hex wire bytes | Debug |
| `b64` | Base64 wire bytes | Base64 wire bytes | Compact |
| `cbor` | CBOR binary | CBOR binary | Performance |

### API Command Format

```
peer <addr> update <encoding> [attr <set|add|del> <attributes>]... [nlri <family> add <nlri>... [del <nlri>...]]...
```

**Examples:**
```bash
peer 10.0.0.1 update text attr set next-hop 10.0.0.1 med 100 nlri ipv4/unicast add 1.0.0.0/24
peer 10.0.0.1 update hex attr set 400101... nlri ipv4/unicast add 18000a000a00
peer 10.0.0.1 update b64 attr set QAEB... nlri ipv4/unicast add GAAK...
```

### Config File Format

```
update {
    <encoding> {
        attr {
            <set|add|del> <attribute>;
            ...
            nlri <family> add <nlri>... [del <nlri>...];
            ...
        }
        attr { ... }  # Each top-level attr block starts with clean state
    }
}
```

- `attr { }` block = clean state (no inherited attributes)
- `set <attr>` = replace value
- `add <attr>` = append to list (community, etc.)
- `del <attr>` = remove from list
- `nlri` uses current accumulated attributes

**Example (text):**
```
update {
    text {
        # Route group 1
        attr {
            set next-hop 10.0.0.1;
            set origin igp;
            set community [65000:1 65000:2];

            nlri ipv4/unicast add 1.0.0.0/24 2.0.0.0/24;

            add community [65000:3];
            nlri ipv4/unicast add 3.0.0.0/24;

            del community [65000:1];
            nlri ipv4/unicast add 4.0.0.0/24 del 5.0.0.0/24;
        }

        # Route group 2 - clean state
        attr {
            set next-hop 10.0.0.2;
            set med 200;

            nlri ipv4/unicast add 6.0.0.0/24;
        }
    }
}
```

**Example (hex):**
```
update {
    hex {
        attr {
            set 400101400206020100001f94400304050607;
            # Spaces help track NLRI boundaries for UPDATE size splitting
            nlri ipv4/unicast add 18010a00 18020b00;  # 10.0.0.0/24, 11.0.0.0/24
        }
        attr {
            set 400101;
            nlri ipv4/unicast del 18030c00;  # 12.0.0.0/24
        }
    }
}
```

### Chained Attr/NLRI Sections

Multiple `attr` and `nlri` sections can be chained. Attributes accumulate and apply to subsequent `nlri`:

```bash
peer 10.0.0.1 update text \
    attr set next-hop 10.0.0.1 origin igp community [65000:1 65000:2] \
    nlri ipv4/unicast add 1.0.0.0/24 2.0.0.0/24 \
    attr add community [65000:3] \
    nlri ipv4/unicast add 3.0.0.0/24 \
    attr del community [65000:1] \
    nlri ipv4/unicast add 4.0.0.0/24 del 5.0.0.0/24
```

Result:
- `1.0.0.0/24`, `2.0.0.0/24` → next-hop, origin, community [65000:1 65000:2]
- `3.0.0.0/24` → next-hop, origin, community [65000:1 65000:2 65000:3]
- `4.0.0.0/24` → next-hop, origin, community [65000:2 65000:3]
- `5.0.0.0/24` → withdraw

### Common Parser

API and config share the same parser with format-specific tokenizer:

```
Tokenizer (format-specific)
    ├── API: split on whitespace, newline terminates
    └── Config: split on whitespace + { } ;
            ↓
Common Token Stream: [update, hex, attr, 400101..., nlri, ipv4/unicast, add, 18000a...]
            ↓
Shared Parser: parseUpdate() → parseEncoding() → parseAttr() → parseNlri()
            ↓
         AST / Command struct
```

| Aspect | API | Config |
|--------|-----|--------|
| Blocks | implicit | `{ }` |
| Terminator | newline | `;` |
| Multiple attrs | separate commands | multiple `attr { }` |
| Peer selector | `peer 10.0.0.1` | neighbor block context |

### Grammar

```
<command>     := peer <addr> update <encoding> <sections> [watchdog <name>]
<sections>    := <section>...
<section>     := <attr-section> | <nlri-section>
<attr-section>:= attr <attr-mode> <attributes>
<attr-mode>   := set | add | del
<encoding>    := text | hex | b64 | cbor
<attributes>  := <text-attrs> | <wire-data>
<text-attrs>  := <attr>...
<wire-data>   := <encoded-bytes>
<attr>        := next-hop <addr> | origin <type> | med <n> | local-preference <n>
              |  community [...] | large-community [...] | extended-community [...]
              |  as-path [...] | rd <rd> | label <n> | ...
<nlri-sections> := <nlri-section>...
<nlri-section>:= nlri <family> <nlri-ops> [<override-attrs>]
<nlri-ops>    := add <nlri>... [del <nlri>...]
<override-attrs> := <attr>... (text mode only)
<family>      := ipv4/unicast | ipv4/mpls-vpn | ipv4/nlri-mpls | ipv4/mup
              |  ipv6/unicast | ipv6/mpls-vpn | ipv6/nlri-mpls | ipv6/mup
<nlri>        := <prefix> (text) | <encoded-bytes> (hex/b64/cbor)
```

### Family Validation

- `nlri ipv4/*` rejects IPv6 prefixes (parsed mode)
- `nlri ipv6/*` rejects IPv4 prefixes (parsed mode)
- Wire bytes mode: no validation (peer's responsibility)

### Examples (text)

```bash
# Simple announce
peer 10.0.0.1 update text attr set next-hop 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24

# Multiple routes
peer 10.0.0.1 update text attr set next-hop 10.0.0.1 origin igp community [65000:1] \
    nlri ipv4/unicast add 1.0.0.0/24 2.0.0.0/24 3.0.0.0/24

# Announce and withdraw
peer 10.0.0.1 update text attr set next-hop 10.0.0.1 \
    nlri ipv4/unicast add 1.0.0.0/24 del 2.0.0.0/24

# IPv6
peer 10.0.0.1 update text attr set next-hop 2001::1 nlri ipv6/unicast add 2001:db8::/32

# VPN with RD/label
peer 10.0.0.1 update text attr set next-hop 10.0.0.1 rd 65000:100 label 1000 \
    extended-community [target:65000:100] \
    nlri ipv4/mpls-vpn add 10.0.0.0/24 10.0.1.0/24

# Multiple families
peer 10.0.0.1 update text attr set next-hop 10.0.0.1 \
    nlri ipv4/unicast add 1.0.0.0/24 \
    nlri ipv6/unicast add 2001:db8::/32

# Chained attr sections (attributes accumulate)
peer 10.0.0.1 update text \
    attr set next-hop 10.0.0.1 origin igp community [65000:1 65000:2] \
    nlri ipv4/unicast add 1.0.0.0/24 \
    attr add community [65000:3] \
    nlri ipv4/unicast add 2.0.0.0/24 \
    attr del community [65000:1] \
    nlri ipv4/unicast add 3.0.0.0/24

# With watchdog
peer 10.0.0.1 update text attr set next-hop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24 watchdog mygroup
```

### Examples (Wire Bytes)

```bash
# Hex encoding - spaces optional, concatenated as raw wire bytes
# Spaces help track NLRI boundaries for UPDATE size management (max 4096 bytes)
peer 10.0.0.1 update hex attr set 400101400206020100001f94 nlri ipv4/unicast add 18010a00 18020b00
# Or as single blob:
peer 10.0.0.1 update hex attr set 400101400206020100001f94 nlri ipv4/unicast add 18010a0018020b00

# Base64 encoding
peer 10.0.0.1 update b64 attr set QAEBQAIGAgEAAAH5 nlri ipv4/unicast add GAAKAAoA

# Announce and withdraw
peer 10.0.0.1 update hex attr set 400101400206020100001f94 \
    nlri ipv4/unicast add 18010a00 del 18020b00

# Multiple families
peer 10.0.0.1 update hex attr set 400101400206020100001f94 \
    nlri ipv4/unicast add 18010a00 \
    nlri ipv6/unicast add 402001db80000000000000000000000000
```

## Removed Commands

| Command | Migration |
|---------|-----------|
| `announce route <prefix> next-hop <nh>` | `peer * update text attr set next-hop <nh> nlri ipv4/unicast add <prefix>` |
| `announce attributes ... nlri ...` | `peer * update text attr set ... nlri ipv4/unicast add ...` |
| `withdraw route <prefix>` | `peer * update text attr set nlri ipv4/unicast del <prefix>` |

## Kept Commands

| Command | Notes |
|---------|-------|
| `announce watchdog <name>` | Enable watchdog group |
| `withdraw watchdog <name>` | Disable watchdog group |

## Validation

### Text Mode
- Attributes validated (valid origin, med range, etc.)
- NLRI prefix validated against family
- Peer must be established

### Wire Bytes Mode (hex/b64/cbor)
- Wire bytes must decode successfully
- Attributes must start with valid attribute flags
- NLRI format not validated (peer's responsibility)
- Peer must be established

### Error Messages

| Condition | Message |
|-----------|---------|
| Invalid encoding | `invalid hex/b64 in attributes: <error>` |
| Peer not found | `peer not found: 10.0.0.1` |
| Peer not established | `peer not established: 10.0.0.1` |
| Invalid family | `invalid family: <family>` |
| Missing add/del | `expected 'add' or 'del' after nlri family` |
| Prefix family mismatch | `prefix family mismatch: 2001:db8::/32 is IPv6, expected IPv4` |

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
| `cbor` | 0% | | RFC 8949, native binary |

**Default is `hex`** for maximum compatibility and debuggability.

**Examples:**
```bash
# Simple Python script (easy debugging)
session api encoding hex

# Production Python (smaller payload)
session api encoding b64

# Performance Go/Rust (native binary)
session api encoding cbor

# Mixed: debug inbound, efficient outbound
session api encoding inbound hex
session api encoding outbound b64
```

**Note:** `cbor` output uses native binary blobs - both directions must be cbor.

### Output Format

Encoding keyword is self-describing:

```
peer <addr> update <encoding> [attr <set|add|del> <attributes>]... [nlri <family> add <nlri>... [del <nlri>...]]...
```

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
    "next-hop": "10.0.0.1",
    "med": 100
  },
  "nlri": {
    "ipv4/unicast": {
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
      "add": ["18000a000a00"],
      "del": ["18000b000b00"]
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
      "add": ["GAAKAAoA"]
    }
  }
}
```

### Example Output (cbor)

CBOR uses native binary - wire bytes are embedded directly without encoding overhead.

```
# CBOR diagnostic notation:
{
  "type": "update",
  "msg-id": 123,
  "attr": {
    "encoding": "cbor",
    "data": h'400101400206020100001f94'
  },
  "nlri": {
    "ipv4/unicast": {
      "add": [h'18000a000a00']
    }
  }
}
```

### Text Format

```
peer 10.0.0.1 received 123 update text attr set next-hop 10.0.0.1 med 100 nlri ipv4/unicast add 10.0.0.0/24
peer 10.0.0.1 received 123 update hex attr set 400101... nlri ipv4/unicast add 18000a000a00 del 18000b000b00
peer 10.0.0.1 received 123 update b64 attr set QAEB... nlri ipv4/unicast add GAAK...
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
| `pkg/api/route.go` | Remove `announce route`, `announce attributes` handlers |
| `pkg/api/route.go` | Update family handlers to use new NLRI group parser |
| `pkg/api/route.go` | Add `handleAnnounceRaw()` for raw wire bytes announce |
| `pkg/api/route_parse.go` | Add `parseNLRIGroups()`, `applyOverrides()` |
| `pkg/api/route_parse_test.go` | Tests for new parser |
| `pkg/api/json.go` | Add `formatRawUpdate()` for wire bytes output |
| `pkg/api/text.go` | Add raw text format output |
| `pkg/api/session.go` | Add `session api encoding` handlers |
| `pkg/api/types.go` | Add `WireEncoding` field to `ContentConfig` |
| `pkg/api/process.go` | Track per-process wire encoding setting |
| `pkg/api/cbor.go` | CBOR encoder (RFC 8949) using `fxamacker/cbor` |

### Test Updates (~48 files)

| Pattern | Count | Change |
|---------|-------|--------|
| `test/data/api/*.run` | ~30 | Update `announce route` → `announce ipv4/unicast ... nlri` |
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
7. Write test for CBOR output → MUST FAIL
8. Implement CBOR encoder (`fxamacker/cbor`) → MUST PASS

### Phase 4: Test Migration

1. Create migration script for .run files
2. Run script, verify changes
3. Update .ci expected outputs
4. Run `make functional` → MUST PASS

### Phase 4: Documentation

1. Update `.claude/zebgp/api/ARCHITECTURE.md`
2. Update `.claude/zebgp/api/COMMANDS.md` (if exists)

## Checklist

### Parser
- [ ] Test fails first (parseNLRIGroups)
- [ ] Test passes after impl
- [ ] Test fails first (applyOverrides)
- [ ] Test passes after impl
- [ ] Test fails first (family validation)
- [ ] Test passes after impl

### Handlers
- [ ] Test fails first (handleAnnounceRaw)
- [ ] Test passes after impl
- [ ] Handler updates complete
- [ ] Old handlers removed

### Wire Encoding
- [ ] Test fails first (session api encoding)
- [ ] Test passes after impl
- [ ] Test fails first (hex encoder)
- [ ] Test passes after impl
- [ ] Test fails first (b64 encoder)
- [ ] Test passes after impl
- [ ] Test fails first (CBOR encoder)
- [ ] Test passes after impl

### Integration
- [ ] .run files migrated
- [ ] .ci files updated
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] `make functional` passes
- [ ] Documentation updated

## Migration Script

```python
#!/usr/bin/env python3
"""Migrate .run files from announce route to family-first syntax."""

import re
import sys

def migrate_line(line: str) -> str:
    # announce route <prefix> next-hop <nh> ...
    # → announce ipv4/unicast next-hop <nh> ... nlri <prefix>

    m = re.match(r"(.*)announce route (\S+) next-hop (\S+)(.*)", line)
    if m:
        pre, prefix, nh, rest = m.groups()
        family = "ipv6/unicast" if ":" in prefix else "ipv4/unicast"
        return f"{pre}announce {family} next-hop {nh}{rest} nlri {prefix}"

    # announce attributes ... nlri ...
    # → announce ipv4/unicast ... nlri ...
    m = re.match(r"(.*)announce attributes (.*)", line)
    if m:
        pre, rest = m.groups()
        # Detect family from nlri prefixes
        if ":" in rest.split("nlri")[-1]:
            family = "ipv6/unicast"
        else:
            family = "ipv4/unicast"
        return f"{pre}announce {family} {rest}"

    return line

if __name__ == "__main__":
    for line in sys.stdin:
        print(migrate_line(line), end="")
```

## Error Messages

| Condition | Message |
|-----------|---------|
| IPv6 prefix in ipv4/* | `prefix family mismatch: 2001:db8::/32 is IPv6, expected IPv4` |
| IPv4 prefix in ipv6/* | `prefix family mismatch: 10.0.0.0/24 is IPv4, expected IPv6` |
| `add med` when shared has med | `cannot add scalar attribute 'med': already set in shared attributes, use 'set' to override` |
| Missing nlri keyword | `missing nlri keyword after shared attributes` |
| Empty nlri group | `empty nlri group: expected at least one prefix after 'nlri'` |

---

**Created:** 2026-01-04

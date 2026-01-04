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

### New Syntax

```
announce <family> <shared-attrs>... nlri <prefix>... [add|set <attr>]... [watchdog <name>]
```

### Grammar

```
<command>     := announce <family> <shared-attrs> <nlri-groups> [watchdog <name>]
<family>      := ipv4/unicast | ipv4/mpls-vpn | ipv4/nlri-mpls | ipv4/mup
              |  ipv6/unicast | ipv6/mpls-vpn | ipv6/nlri-mpls | ipv6/mup
<shared-attrs> := <attr>...
<nlri-groups> := <nlri-group>...
<nlri-group>  := nlri <prefix>... [<override>...]
<override>    := add <attr> | set <attr>
<attr>        := next-hop <addr> | origin <type> | med <n> | local-preference <n>
              |  community [...] | large-community [...] | extended-community [...]
              |  as-path [...] | ...
```

### Override Semantics

| Modifier | List attrs (community, etc.) | Scalar attrs (med, etc.) |
|----------|------------------------------|--------------------------|
| `add`    | Append to shared list        | Set if missing, ERROR if exists |
| `set`    | Replace shared list          | Override value |

### Family Validation

- `announce ipv4/*` rejects IPv6 prefixes
- `announce ipv6/*` rejects IPv4 prefixes
- Error message: "prefix family mismatch: expected IPv4, got IPv6"

### Examples

```bash
# Simple single route
announce ipv4/unicast next-hop 10.0.0.1 nlri 1.0.0.0/24

# Multiple routes with shared attributes
announce ipv4/unicast next-hop 10.0.0.1 origin igp community [65000:1] \
    nlri 1.0.0.0/24 2.0.0.0/24 3.0.0.0/24

# Per-route overrides
announce ipv4/unicast next-hop 10.0.0.1 community [65000:1] origin igp \
    nlri 1.0.0.0/24 \
    nlri 2.0.0.0/24 add community [65000:2] \
    nlri 3.0.0.0/24 set community [65000:3] \
    nlri 4.0.0.0/24 set med 100 add community [65000:4]

# With watchdog
announce ipv4/unicast next-hop 10.0.0.1 nlri 10.0.0.0/24 watchdog mygroup

# IPv6
announce ipv6/unicast next-hop 2001::1 nlri 2001:db8::/32

# VPN with RD/label in shared attrs
announce ipv4/mpls-vpn next-hop 10.0.0.1 rd 65000:100 label 1000 \
    extended-community [target:65000:100] \
    nlri 10.0.0.0/24 10.0.1.0/24

# Labeled unicast
announce ipv4/nlri-mpls next-hop 10.0.0.1 label 100 \
    nlri 10.0.0.0/24 \
    nlri 10.0.1.0/24 set label 101
```

## Removed Commands

| Command | Migration |
|---------|-----------|
| `announce route <prefix> next-hop <nh>` | `announce ipv4/unicast next-hop <nh> nlri <prefix>` |
| `announce attributes ... nlri ...` | `announce ipv4/unicast ... nlri ...` |

## Kept Commands

| Command | Notes |
|---------|-------|
| `withdraw route <prefix>` | Auto-detect family from prefix |
| `announce watchdog <name>` | Enable watchdog group |
| `withdraw watchdog <name>` | Disable watchdog group |

## New Command: Raw Announce

### Syntax

```
peer <addr> announce raw <base64-attributes> nlri <base64-nlri>
```

### Purpose

Send pre-encoded BGP UPDATE components without parsing/re-encoding. Used for:
- Zero-copy forwarding from external route reflector
- Pre-built wire bytes from policy engine
- Testing with specific wire formats

### Example

```bash
# Forward pre-encoded route to specific peer
peer 10.0.0.1 announce raw QAEBQAIGAgEAAAH5QAMEBQYHCA== nlri GAAKAAoA

# Attributes: ORIGIN IGP, AS_PATH [65529], NEXT_HOP 5.6.7.8
# NLRI: 10.0.10.0/24
```

### Validation

- Base64 must decode successfully
- Attributes must start with valid attribute flags
- NLRI format not validated (peer's responsibility)
- Peer must be established

### Error Messages

| Condition | Message |
|-----------|---------|
| Invalid base64 | `invalid base64 in attributes: <error>` |
| Peer not found | `peer not found: 10.0.0.1` |
| Peer not established | `peer not established: 10.0.0.1` |

## Raw Output Format (Receive)

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
| `hex` | 100% | ✅ Yes | Human-readable, debug-friendly |
| `base64` | 33% | | Standard, good Python support |
| `cbor` | 0% | | RFC 8949, native binary |

**Default is `hex`** for maximum compatibility and debuggability.

**Examples:**
```bash
# Simple Python script (easy debugging)
session api encoding hex

# Production Python (smaller payload)
session api encoding base64

# Performance Go/Rust (native binary)
session api encoding cbor

# Mixed: debug inbound, efficient outbound
session api encoding inbound hex
session api encoding outbound base64
```

**Note:** `cbor` output uses native binary blobs - both directions must be cbor.

### Output Format

Wire bytes field name depends on encoding:

```
peer 10.0.0.1 update raw <encoded-attributes> nlri <family> <encoded-nlri>
```

### Attribute Encoding

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
  "raw": {
    "attributes": "400101400206020100001f9400304050607",
    "nlri": {
      "ipv4/unicast": "18000a000a00"
    }
  }
}
```

### Example Output (base64)

```json
{
  "type": "update",
  "direction": "received",
  "msg-id": 123,
  "peer": {"address": "10.0.0.1", "asn": 65001},
  "raw": {
    "attributes": "QAEBQAIGAgEAAAH5QAMEBQYHCA==",
    "nlri": {
      "ipv4/unicast": "GAAKAAoA"
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
  "raw": {
    "attributes": h'400101400206020100001f94400304050607',
    "nlri": {
      "ipv4/unicast": h'18000a000a00'
    }
  }
}
```

### Text Format

```
peer 10.0.0.1 received update 123 raw 400101400206... nlri ipv4/unicast 18000a000a00
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
5. Write test for base64 wire bytes output → MUST FAIL
6. Implement base64 encoder → MUST PASS
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
- [ ] Test fails first (base64 encoder)
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

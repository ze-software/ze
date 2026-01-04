# UPDATE Command Syntax

## Status

**NOT IMPLEMENTED** - This is a design specification for future implementation.

Current implementation uses legacy `announce route` / `announce attributes` commands.
See `plan/spec-announce-family-first.md` for implementation plan.

---

## Overview

Unified syntax for API commands and config files to send/receive BGP UPDATE messages.

## Core Syntax

```
update <encoding> [attr <set|add|del> <attributes>]... [nlri <family> add <nlri>... [del <nlri>...]]...
```

## Encodings

| Encoding | Attributes | NLRI | Use case |
|----------|------------|------|----------|
| `text` | Parsed (next-hop, med, ...) | Prefixes (1.0.0.0/24) | Human-readable |
| `hex` | Hex wire bytes | Hex wire bytes | Debug |
| `b64` | Base64 wire bytes | Base64 wire bytes | Compact |
| `cbor` | CBOR binary | CBOR binary | Performance |

## Attribute Operations

| Operation | Meaning | Example |
|-----------|---------|---------|
| `set` | Replace value (`k = v`) | `attr set next-hop 10.0.0.1` |
| `add` | Append to list (`k.append(v)`) | `attr add community [65000:1]` |
| `del` | Remove from list (`k.remove(v)`) | `attr del community [65000:1]` |

- `set` works on any attribute
- `add`/`del` work on list attributes: community, large-community, extended-community

## Chained Sections

Multiple `attr` and `nlri` sections can be chained. Attributes accumulate:

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
- `1.0.0.0/24`, `2.0.0.0/24` → community [65000:1 65000:2]
- `3.0.0.0/24` → community [65000:1 65000:2 65000:3]
- `4.0.0.0/24` → community [65000:2 65000:3]
- `5.0.0.0/24` → withdraw

## API Command Format

```
peer <addr> update <encoding> [attr <set|add|del> <attributes>]... [nlri <family> add <nlri>... [del <nlri>...]]...
```

Each API command line = clean state.

**Examples:**
```bash
peer 10.0.0.1 update text attr set next-hop 10.0.0.1 med 100 nlri ipv4/unicast add 1.0.0.0/24
peer 10.0.0.1 update hex attr set 400101400206020100001f94 nlri ipv4/unicast add 18010a00 18020b00
peer 10.0.0.1 update b64 attr set QAEBQAIGAgEAAAH5 nlri ipv4/unicast add GAAKAAoA
```

## Config File Format

```
update {
    <encoding> {
        attr {
            <set|add|del> <attribute>;
            ...
            nlri <family> add <nlri>... [del <nlri>...];
            ...
        }
        attr { ... }  # Each top-level attr block = clean state
    }
}
```

Each `attr { }` block = clean state.

**Example:**
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

## Wire Bytes Format

For `hex`/`b64`/`cbor` encodings:
- Spaces optional in NLRI - concatenated as raw wire bytes
- Spaces help track NLRI boundaries for UPDATE size splitting (max 4096 bytes)

```bash
# Both equivalent:
nlri ipv4/unicast add 18010a00 18020b00
nlri ipv4/unicast add 18010a0018020b00
```

## Received UPDATE Output

**Text format:**
```
peer 10.0.0.1 received 123 update text attr set next-hop 10.0.0.1 med 100 nlri ipv4/unicast add 10.0.0.0/24
peer 10.0.0.1 received 123 update hex attr set 400101... nlri ipv4/unicast add 18010a00 del 18020b00
```

**JSON format:**
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
      "add": ["18010a00"],
      "del": ["18020b00"]
    }
  }
}
```

## Wire Encoding Control

```bash
session api encoding hex        # Both directions (default)
session api encoding b64        # Both directions
session api encoding cbor       # Both directions
session api encoding inbound hex outbound b64  # Mixed
```

## Grammar

```
<command>      := peer <addr> update <encoding> <sections> [watchdog <name>]
<sections>     := <section>...
<section>      := <attr-section> | <nlri-section>
<attr-section> := attr <attr-mode> <attributes>
<attr-mode>    := set | add | del
<encoding>     := text | hex | b64 | cbor
<attributes>   := <text-attrs> | <wire-data>
<nlri-section> := nlri <family> <nlri-ops>
<nlri-ops>     := add <nlri>... [del <nlri>...]
<family>       := ipv4/unicast | ipv4/mpls-vpn | ipv6/unicast | ipv6/mpls-vpn | ...
<nlri>         := <prefix> (text) | <encoded-bytes> (hex/b64/cbor)
```

## Families

| Family | Wire bytes | Notes |
|--------|------------|-------|
| ipv4/unicast | ✅ | Standard NLRI field |
| ipv6/unicast | ✅ | From MP_REACH_NLRI |
| ipv4/mpls-vpn | ✅ | From MP_REACH_NLRI |
| ipv6/mpls-vpn | ✅ | From MP_REACH_NLRI |
| flowspec | ❌ | Complex encoding, use text |

## Attribute Wire Bytes

- **Included:** All path attributes in wire order
- **Excluded:** MP_REACH_NLRI (type 14), MP_UNREACH_NLRI (type 15)
- NLRI extracted and encoded separately by family

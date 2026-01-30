# UPDATE Command Syntax

## Status

**PARTIALLY IMPLEMENTED** - Text mode works, wire modes (hex/b64) implemented.

Legacy `announce route` / `announce attributes` commands removed.
See `docs/plan/done/089-new-syntax.md` for migration details.

---

## Overview

Unified syntax for API commands and config files to send/receive BGP UPDATE messages.

## Core Syntax

```
update <encoding> [<attr-sections>]... [nlri <family> add <nlri>... [del <nlri>...]]...
```

## Encodings

| Encoding | Attributes | NLRI | Use case |
|----------|------------|------|----------|
| `text` | Per-attribute keywords | Prefixes (1.0.0.0/24) | Human-readable |
| `hex` | `attr set <hex-bytes>` | Hex wire bytes | Debug |
| `b64` | `attr set <b64-bytes>` | Base64 wire bytes | Compact |

## Text Mode - Per-Attribute Keywords

In text mode, each attribute uses its own keyword with `set`/`add`/`del`:

```
update text <attr> <op> <value> [<attr> <op> <value>]... nhop set <addr> nlri <family> add <prefix>...
```

### Scalar Attributes (set/del only)

| Attribute | Syntax | Example |
|-----------|--------|---------|
| origin | `origin set <igp\|egp\|incomplete>` | `origin set igp` |
| origin | `origin del` | Remove origin attribute |
| med | `med set <value>` | `med set 100` |
| med | `med del` | Remove MED attribute |
| local-preference | `local-preference set <value>` | `local-preference set 200` |
| local-preference | `local-preference del` | Remove local-preference |

### AS-Path (set/add/del)

| Syntax | Example |
|--------|---------|
| `as-path set [<asn>...]` | `as-path set [ 65001 65002 ]` |
| `as-path add [<asn>...]` | `as-path add [ 65000 ]` (prepend) |
| `as-path del` | Remove entire AS path |
| `as-path del [<asn>...]` | `as-path del [ 65000 ]` (remove first occurrence)

### List Attributes (set/add/del)

| Attribute | Syntax | Example |
|-----------|--------|---------|
| community | `community <set\|add\|del> [<comm>...]` | `community set [ 65000:1 65000:2 ]` |
| large-community | `large-community <set\|add\|del> [<lc>...]` | `large-community add [ 65000:0:1 ]` |
| extended-community | `extended-community <set\|add\|del> [<ec>...]` | `extended-community del [ rt:65000:1 ]` |

### Next-Hop (required for announce)

| Syntax | Example |
|--------|---------|
| `nhop set <addr>` | `nhop set 10.0.0.1` |
| `nhop set self` | `nhop set self` |
| `nhop del` | Clear next-hop (for withdraw) |

#### Next-Hop Resolution (`self` vs explicit)

| Input | Stored | Wire (per-peer) |
|-------|--------|-----------------|
| `nhop set 10.0.0.1` | Explicit: 10.0.0.1 | 10.0.0.1 |
| `nhop set self` | Policy: self | Resolved per-peer (e.g., 192.168.1.1) |

When `nhop set self` is used:
- **Stored**: Policy marker "self" (not an IP)
- **Wire**: Resolved at send time to peer's local address
- **Output**: Shows resolved address, not "self"

#### Input vs Output Format

**Input format**: `nhop` is a separate accumulator, can appear anywhere before `nlri`:
```bash
peer 10.0.0.1 update text nhop set self origin set igp nlri ipv4/unicast add 1.0.0.0/24
peer 10.0.0.1 update text origin set igp nhop set self nlri ipv4/unicast add 1.0.0.0/24
# Both equivalent - nhop accumulates
```

**Output format**: `nhop` is always printed **inside each nlri group** (after attrs, before nlri):
```bash
# Output always shows nhop with its nlri group:
peer 10.0.0.1 received 123 update text origin set igp nhop set 192.168.1.1 nlri ipv4/unicast add 1.0.0.0/24
#                                                     ^^^^^^^^^^^^^^^^^^^^
#                                                     nhop printed with nlri group, resolved value
```

**Multiple nlri groups** - each gets its own nhop in output:
```bash
# Input:
peer * update text nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24 \
                   nhop set 10.0.0.2 nlri ipv4/unicast add 2.0.0.0/24

# Output (two separate nlri groups, each with its nhop):
peer 10.0.0.1 received 123 update text nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24
peer 10.0.0.1 received 124 update text nhop set 10.0.0.2 nlri ipv4/unicast add 2.0.0.0/24
```

This ensures output is always self-contained per nlri group.

#### Next-Hop Overwriting

`nhop set` can appear multiple times - last one wins before each `nlri` section:

```bash
# nhop overwritten - 10.0.0.2 used for both prefixes
update text nhop set 10.0.0.1 nhop set 10.0.0.2 nlri ipv4/unicast add 1.0.0.0/24 2.0.0.0/24

# Different nhop per nlri section
update text nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24 \
            nhop set 10.0.0.2 nlri ipv4/unicast add 2.0.0.0/24
# → 1.0.0.0/24 gets nhop 10.0.0.1
# → 2.0.0.0/24 gets nhop 10.0.0.2
```

#### Next-Hop and Withdraw

| Operation | nhop required |
|-----------|---------------|
| `nlri <family> add ...` | ✅ Yes |
| `nlri <family> del ...` | ❌ No (ignored if present) |

```bash
# Withdraw - nhop not needed
update text nlri ipv4/unicast del 1.0.0.0/24

# Mixed - nhop required for add, ignored for del
update text nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24 del 2.0.0.0/24
```

### Text Mode Examples

```bash
# Simple announce
peer 10.0.0.1 update text nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24

# With attributes
peer 10.0.0.1 update text origin set igp med set 100 nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24

# Multiple attributes
peer 10.0.0.1 update text origin set igp local-preference set 200 community set [ 65000:1 ] nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24 2.0.0.0/24

# Withdraw (no nhop needed)
peer 10.0.0.1 update text nlri ipv4/unicast del 1.0.0.0/24
```

## Wire Mode - Raw Bytes (hex/b64)

In wire modes, attributes are encoded as a single blob, nhop as wire bytes:

```
update hex [attr set <hex-bytes>] [nhop set <hex-nhop>] nlri <family> add <hex-nlri>...
update b64 [attr set <b64-bytes>] [nhop set <b64-nhop>] nlri <family> add <b64-nlri>...
```

Only `attr set` and `nhop set` supported (no `add`/`del` for wire bytes).

### Wire Mode Examples

```bash
peer 10.0.0.1 update hex attr set 400101400206020100001f94 nhop set 0a000001 nlri ipv4/unicast add 18010a00
peer 10.0.0.1 update b64 attr set QAEBQAIGAgEAAAH5 nhop set CgAAAQ== nlri ipv4/unicast add GAAKAAoA
```

## Attribute Operations Summary

| Operation | Text Mode | Wire Mode |
|-----------|-----------|-----------|
| `set` | Per-attribute: `med set 100` | `attr set <bytes>` |
| `add` | List attrs only: `community add [...]` | ❌ Not supported |
| `del` | List attrs only: `community del [...]` | ❌ Not supported |

## Validation Rules

### Scalar Attributes (origin, med, local-preference)

| Operation | Allowed | Notes |
|-----------|---------|-------|
| `set` | ✅ | Replace value |
| `add` | ❌ | Error: "origin: add not supported on scalar attribute" |
| `del` | ✅ | Remove attribute (always succeeds, no-op if not set) |

### AS-Path (special - list with prepend semantics)

| Operation | Allowed | Notes |
|-----------|---------|-------|
| `set` | ✅ | Replace entire path |
| `add` | ✅ | Prepend ASN(s) to path |
| `del` | ✅ | Remove entire attribute (always succeeds) |
| `del [asn]` | ✅ | Remove first occurrence (**error if not present**) |

### List Attributes (community, large-community, extended-community)

| Operation | Allowed | Notes |
|-----------|---------|-------|
| `set` | ✅ | Replaces entire list |
| `add` | ✅ | Prepends to existing list |
| `del [value]` | ✅ | Removes first occurrence of each value (**error if not present**) |
| `del` (no value) | ✅ | Removes entire attribute (always succeeds, no-op if not set) |

#### How `add` Works

`add` **prepends** to the current accumulated list - it does NOT replace:

```bash
# Single add - starts empty, adds [65000:1]
update text community add [ 65000:1 ] nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24
# → community = [65000:1]

# Multiple adds - prepends (newest first)
update text community add [ 65000:1 ] community add [ 65000:2 ] nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24
# → community = [65000:2, 65000:1]  (65000:2 prepended to [65000:1])

# set then add - set replaces, add prepends
update text community set [ 65000:1 ] community add [ 65000:2 ] nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24
# → community = [65000:2, 65000:1]

# add then set - add is lost (set replaces everything)
update text community add [ 65000:1 ] community set [ 65000:2 ] nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24
# → community = [65000:2]  (add was overwritten by set)
```

#### How `del` Works

`del [value]` removes the **first occurrence** of each specified value from the list.
**Errors if value not present.**

```bash
# set then del - removes first occurrence
update text community set [ 65000:1 65000:2 ] community del [ 65000:1 ] nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24
# → community = [65000:2]

# del with duplicates - removes only FIRST occurrence
update text community set [ 65000:1 65000:2 65000:1 ] community del [ 65000:1 ] nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24
# → community = [65000:2, 65000:1]  (only first 65000:1 removed)

# ERROR: del from empty list
update text community del [ 65000:1 ] nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24
# → error: "community del: 65000:1 not present"

# ERROR: del non-existent value
update text community set [ 65000:1 ] community del [ 65000:99 ] nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24
# → error: "community del: 65000:99 not present"

# del without value - removes entire attribute (always succeeds)
update text community set [ 65000:1 65000:2 ] community del nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24
# → community removed entirely

# del without value on unset attribute - no-op, no error
update text community del nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24
# → no change (community was not set)
```

#### Accumulation Across NLRI Sections

Attributes accumulate across `nlri` sections - each section captures a **snapshot**:

```bash
update text \
    community set [ 65000:1 ] nhop set 10.0.0.1 \
    nlri ipv4/unicast add 1.0.0.0/24 \
    community add [ 65000:2 ] \
    nlri ipv4/unicast add 2.0.0.0/24 \
    community add [ 65000:3 ] \
    nlri ipv4/unicast add 3.0.0.0/24

# Result (add prepends):
# 1.0.0.0/24 → community = [65000:1]                     (snapshot after set)
# 2.0.0.0/24 → community = [65000:2, 65000:1]            (65000:2 prepended)
# 3.0.0.0/24 → community = [65000:3, 65000:2, 65000:1]   (65000:3 prepended)
```

### Operation Order Constraints

| Sequence | Valid | Notes |
|----------|-------|-------|
| `community set [...] community add [...]` | ✅ | Set then prepend |
| `community add [...] community set [...]` | ⚠️ | Add then replace (add is lost) |
| `community set [...] community del [x]` | ✅ | Set then delete specific value |
| `community del [x]` (before set) | ❌ | Error: value not present |
| `community del` (no value) | ✅ | Always succeeds (clears or no-op) |

### Required Fields

| Context | Required | Error if missing |
|---------|----------|------------------|
| Announce (add NLRI) | `nhop set <addr>` | "missing next-hop" |
| Withdraw (del NLRI) | - | None required |

### Invalid Sequences (Errors)

```bash
# ERROR: add on scalar
update text origin add igp ...
# → "origin: add not supported on scalar attribute"

# ERROR: missing next-hop for announce
update text nlri ipv4/unicast add 1.0.0.0/24
# → "missing next-hop"

# ERROR: nhop without set/del keyword
update text nhop 10.0.0.1 ...
# → "nhop requires set or del"

# ERROR: unknown attribute
update text foo set bar ...
# → "unexpected token: foo"

# ERROR: del specific value not present
update text community set [ 65000:1 ] community del [ 65000:99 ] ...
# → "community del: 65000:99 not present"
```

### Valid but Wasteful Sequences

```bash
# VALID but wasteful: set overwrites previous set
update text med set 100 med set 200 nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24
# → Works, med=200 (first set is lost)

# VALID but wasteful: add then set (add is lost)
update text community add [ 65000:1 ] community set [ 65000:2 ] nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24
# → Works, community=[65000:2] (add is lost)

# VALID: del (no value) before set - clears nothing, then sets
update text community del community set [ 65000:1 ] nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24
# → Works, community=[65000:1]
```

## Chained Sections (Text Mode)

Multiple attribute and nlri sections can be chained. Attributes accumulate:

```bash
peer 10.0.0.1 update text \
    origin set igp community set [ 65000:1 65000:2 ] nhop set 10.0.0.1 \
    nlri ipv4/unicast add 1.0.0.0/24 2.0.0.0/24 \
    community add [ 65000:3 ] \
    nlri ipv4/unicast add 3.0.0.0/24 \
    community del [ 65000:1 ] \
    nlri ipv4/unicast add 4.0.0.0/24 del 5.0.0.0/24
```

Result (add prepends, del removes first occurrence):
- `1.0.0.0/24`, `2.0.0.0/24` → community [65000:1, 65000:2]
- `3.0.0.0/24` → community [65000:3, 65000:1, 65000:2] (65000:3 prepended)
- `4.0.0.0/24` → community [65000:3, 65000:2] (first 65000:1 removed)
- `5.0.0.0/24` → withdraw

## API Command Format

**Text mode:**
```
peer <addr> update text [<attr> <set|add|del> <value>]... nhop set <addr> nlri <family> add <nlri>... [del <nlri>...]
```

**Wire mode:**
```
peer <addr> update <hex|b64> attr set <bytes> nlri <family> add <nlri>... [del <nlri>...]
```

Each API command line = clean state.

**Examples:**
```bash
# Text mode - per-attribute keywords
peer 10.0.0.1 update text med set 100 nhop set 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24

# Wire mode - attr set <bytes>
peer 10.0.0.1 update hex attr set 400101400206020100001f94 nlri ipv4/unicast add 18010a00 18020b00
peer 10.0.0.1 update b64 attr set QAEBQAIGAgEAAAH5 nlri ipv4/unicast add GAAKAAoA
```

## Config File Format

```
update {
    text {
        # Route group - attributes accumulate within block
        origin set igp;
        nhop set 10.0.0.1;
        community set [ 65000:1 65000:2 ];

        nlri ipv4/unicast add 1.0.0.0/24 2.0.0.0/24;

        community add [ 65000:3 ];
        nlri ipv4/unicast add 3.0.0.0/24;
    }

    hex {
        attr set 400101400206020100001f94;
        nlri ipv4/unicast add 18010a00;
    }
}
```

**Example:**
```
update {
    text {
        # Route group 1
        origin set igp;
        nhop set 10.0.0.1;
        community set [ 65000:1 65000:2 ];

        nlri ipv4/unicast add 1.0.0.0/24 2.0.0.0/24;

        community add [ 65000:3 ];
        nlri ipv4/unicast add 3.0.0.0/24;

        community del [ 65000:1 ];
        nlri ipv4/unicast add 4.0.0.0/24 del 5.0.0.0/24;
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
peer 10.0.0.1 received 123 update text origin set igp med set 100 nhop set 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24
peer 10.0.0.1 received 123 update hex attr set 400101... nlri ipv4/unicast add 18010a00 del 18020b00
```

**JSON format:**
```json
{
  "message": {"type": "update", "direction": "received", "id": 123},
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
# Set overall message structure
bgp plugin encoding json        # Structured JSON (default)
bgp plugin encoding text        # Human-readable text

# Set wire bytes format (JSON mode only)
bgp plugin format hex           # Wire bytes as hex string
bgp plugin format base64        # Wire bytes as base64
bgp plugin format parsed        # Decoded fields only (default)
bgp plugin format full          # Both parsed AND wire bytes

# Set ACK timing
bgp plugin ack sync             # Wait for wire transmission
bgp plugin ack async            # Return immediately (default)
```

## Grammar

### Text Mode
```
<update-text> := <section>*
<section>     := <scalar-attr> | <list-attr> | <nlri-section> | <wire-attr>

<scalar-attr> := <scalar-name> (set <value> | del [<value>])
<scalar-name> := origin | med | local-preference | nhop | path-information | rd | label

<list-attr>   := <list-name> (set <list> | add <list> | del [<list>])
<list-name>   := as-path | community | large-community | extended-community

<nlri-section> := nlri <family> <nlri-op>+
<nlri-op>      := add <prefix>+ [watchdog set <name>] | del <prefix>+ | eor

<wire-attr>    := attr (set <bytes> | del [<bytes>])   # hex/b64 mode only

<family>       := ipv4/unicast | ipv6/unicast | ipv4/mpls-vpn | ipv6/mpls-vpn | ...
```

### Standalone Watchdog Commands
```
watchdog announce <name>   # send all routes in pool to peers
watchdog withdraw <name>   # withdraw all routes in pool from peers
```

### Scalar `del [<value>]` Semantics

- `<scalar> del` - remove attribute unconditionally
- `<scalar> del <value>` - remove only if current value matches, else error

### Attribute Types

| Type | Attributes |
|------|------------|
| **Scalar** | `origin`, `med`, `local-preference`, `nhop`, `path-information`, `rd`, `label` |
| **List** | `as-path`, `community`, `large-community`, `extended-community` |

### Accumulator → Family Support

| Accumulator | Valid for | Error for |
|-------------|-----------|-----------|
| `rd` | `*-vpn` families | All others |
| `label` | `*-vpn`, `*-labeled` families | All others |
| `path-information` | Any (if ADD-PATH negotiated) | Ignored if not negotiated |

### Wire Mode (hex/b64)
```
<command>       := peer <addr> update <hex|b64> <wire-sections>
<wire-sections> := [attr set <wire-bytes>] [nhop set <wire-bytes>] <nlri-section>...
<nlri-section>  := nlri <family> <nlri-ops>
<nlri-ops>      := add <wire-nlri>... [del <wire-nlri>...]
```

## Families

| Family | Wire bytes | Notes |
|--------|------------|-------|
| ipv4/unicast | ✅ | Standard NLRI field |
| ipv6/unicast | ✅ | From MP_REACH_NLRI |
| ipv4/mpls-vpn | ✅ | From MP_REACH_NLRI |
| ipv6/mpls-vpn | ✅ | From MP_REACH_NLRI |
| l2vpn/vpls | ✅ | RFC 4761 VPLS |
| l2vpn/evpn | ✅ | RFC 7432 EVPN (Type 2, 3, 5) |
| flowspec | ❌ | Complex encoding, use text |

## Attribute Wire Bytes

- **Included:** All path attributes in wire order
- **Excluded:** MP_REACH_NLRI (type 14), MP_UNREACH_NLRI (type 15)
- NLRI extracted and encoded separately by family

## Per-NLRI Overrides

Within `nlri` section, attrs/nhop can be overridden for that section only:

```bash
# Override nhop inside nlri (all modes)
peer 10.0.0.1 update text origin set igp nhop set 10.0.0.1 \
    nlri ipv4/unicast nhop set 10.0.0.2 add 1.0.0.0/24 2.0.0.0/24

# Override attr inside nlri (text mode only)
peer 10.0.0.1 update text community set [ 65000:1 ] nhop set 10.0.0.1 \
    nlri ipv4/unicast community add [ 65000:2 ] add 1.0.0.0/24
```

**Rules:**
- Overrides apply to ALL NLRIs in that section (not per-prefix)
- `nhop set/del` inside nlri works in all modes
- `<attr> set/add/del` inside nlri works in text mode only

## VPN Modifiers

NLRI modifiers for VPN families (not path attributes):

```bash
# L3VPN with RD and label
peer 10.0.0.1 update text extended-community set [ target:65000:100 ] nhop set 10.0.0.1 \
    nlri ipv4/mpls-vpn rd 65000:100 label 1000 add 10.0.0.0/24

# Multiple prefixes same RD/label
nlri ipv4/mpls-vpn rd 65000:100 label 1000 add 10.0.0.0/24 10.0.1.0/24
```

| Modifier | Syntax | Example |
|----------|--------|---------|
| `rd` | `rd <asn>:<num>` or `rd <ip>:<num>` | `rd 65000:100` |
| `label` | `label <num>` | `label 1000` |

## End-of-RIB (EOR)

RFC 4724 End-of-RIB marker signals completion of initial routing table exchange.

### Syntax

```bash
update text nlri <family> eor
```

### Examples

```bash
# IPv4 unicast EOR
peer 10.0.0.1 update text nlri ipv4/unicast eor

# IPv6 unicast EOR
peer 10.0.0.1 update text nlri ipv6/unicast eor

# Multiple families in one command
peer 10.0.0.1 update text nlri ipv4/unicast eor nlri ipv6/unicast eor

# EOR with NLRI in same command
peer 10.0.0.1 update text nlri ipv6/unicast eor nhop set 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24
```

### Wire Format

| Family | Wire Bytes |
|--------|------------|
| IPv4 unicast | Empty UPDATE: `withdrawn=0, path_attr=0, nlri=0` |
| Other families | MP_UNREACH_NLRI with AFI/SAFI, no prefixes |

## VPLS (L2VPN/VPLS)

RFC 4761 Virtual Private LAN Service.

### Syntax

```bash
update text nlri l2vpn/vpls add rd <rd> ve-id <n> ve-block-offset <n> ve-block-size <n> label-base <n>
update text nlri l2vpn/vpls del rd <rd> ve-id <n> ve-block-offset <n> ve-block-size <n> label-base <n>
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `rd` | Route Distinguisher | Required. Format: `ASN:NN` or `IP:NN` |
| `ve-id` | uint16 | VE identifier |
| `ve-block-offset` | uint16 | Starting VE ID in block |
| `ve-block-size` | uint16 | Number of VE IDs in block |
| `label-base` | uint32 | Base MPLS label |

### Examples

```bash
# Announce VPLS
peer 10.0.0.1 update text nlri l2vpn/vpls add rd 1:1 ve-id 1 ve-block-offset 0 ve-block-size 10 label-base 1000

# Withdraw VPLS
peer 10.0.0.1 update text nlri l2vpn/vpls del rd 1:1 ve-id 1 ve-block-offset 0 ve-block-size 10 label-base 1000

# EOR for VPLS
peer 10.0.0.1 update text nlri l2vpn/vpls eor
```

## EVPN (L2VPN/EVPN)

RFC 7432 Ethernet VPN.

### Route Types

| Type | Keyword | Description | RFC |
|------|---------|-------------|-----|
| 2 | `mac-ip` | MAC/IP Advertisement | RFC 7432 §7.2 |
| 3 | `multicast` | Inclusive Multicast Ethernet Tag | RFC 7432 §7.3 |
| 5 | `ip-prefix` | IP Prefix Route | RFC 9136 §3 |

### Common Fields

| Field | Type | Description |
|-------|------|-------------|
| `rd` | Route Distinguisher | Required. Format: `ASN:NN` or `IP:NN` |
| `esi` | 10-byte hex | Ethernet Segment Identifier (colon-separated) |
| `etag` | uint32 | Ethernet Tag ID |
| `label` | uint32 | MPLS label (can repeat for multiple labels) |

### Type 2: MAC/IP Advertisement

```bash
update text nlri l2vpn/evpn add mac-ip rd <rd> mac <mac> [ip <ip>] label <n>
```

| Field | Type | Description |
|-------|------|-------------|
| `mac` | MAC address | Required. Format: `00:11:22:33:44:55` |
| `ip` | IP address | Optional. IPv4 or IPv6 |

**Examples:**

```bash
# MAC only
peer 10.0.0.1 update text nlri l2vpn/evpn add mac-ip rd 1:1 mac 00:11:22:33:44:55 label 100

# MAC with IPv4
peer 10.0.0.1 update text nlri l2vpn/evpn add mac-ip rd 1:1 mac 00:11:22:33:44:55 ip 192.168.1.1 label 100

# MAC with IPv6
peer 10.0.0.1 update text nlri l2vpn/evpn add mac-ip rd 1:1 mac 00:11:22:33:44:55 ip 2001:db8::1 label 100

# With ESI and Ethernet Tag
peer 10.0.0.1 update text nlri l2vpn/evpn add mac-ip rd 1:1 esi 00:01:02:03:04:05:06:07:08:09 etag 100 mac 00:11:22:33:44:55 label 100
```

### Type 3: Inclusive Multicast Ethernet Tag

```bash
update text nlri l2vpn/evpn add multicast rd <rd> ip <originator-ip>
```

| Field | Type | Description |
|-------|------|-------------|
| `ip` | IP address | Required. Originator IP |

**Example:**

```bash
peer 10.0.0.1 update text nlri l2vpn/evpn add multicast rd 1:1 ip 192.168.1.1
```

### Type 5: IP Prefix Route

```bash
update text nlri l2vpn/evpn add ip-prefix rd <rd> prefix <prefix> [gateway <ip>] label <n>
```

| Field | Type | Description |
|-------|------|-------------|
| `prefix` | IP prefix | Required. IPv4 or IPv6 prefix |
| `gateway` | IP address | Optional. GW IP Overlay Index (RFC 9136) |

**Overlay Index (RFC 9136):** Either `esi` OR `gateway` can be specified, but not both.
If neither is specified, direct forwarding via label is used.

**Examples:**

```bash
# IPv4 prefix (direct forwarding)
peer 10.0.0.1 update text nlri l2vpn/evpn add ip-prefix rd 1:1 prefix 10.0.0.0/24 label 100

# IPv6 prefix
peer 10.0.0.1 update text nlri l2vpn/evpn add ip-prefix rd 1:1 prefix 2001:db8::/32 label 100

# With GW IP Overlay Index (recursive resolution via RT-2)
peer 10.0.0.1 update text nlri l2vpn/evpn add ip-prefix rd 1:1 prefix 10.0.0.0/24 gateway 192.168.1.254 label 100

# With ESI Overlay Index (recursive resolution via RT-1)
peer 10.0.0.1 update text nlri l2vpn/evpn add ip-prefix rd 1:1 prefix 10.0.0.0/24 esi 00:01:02:03:04:05:06:07:08:09 label 100
```

### EVPN EOR

```bash
peer 10.0.0.1 update text nlri l2vpn/evpn eor
```

## Conditional nhop del

`nhop del <addr>` only removes if current nhop matches:

```bash
peer 10.0.0.1 update text nhop set 10.0.0.1 \
    nlri ipv4/unicast add 1.0.0.0/24 \
    nhop del 10.0.0.1 \
    nhop set 10.0.0.2 \
    nlri ipv4/unicast add 2.0.0.0/24
# → 1.0.0.0/24 nhop 10.0.0.1
# → 2.0.0.0/24 nhop 10.0.0.2 (del matched, then set)

peer 10.0.0.1 update text nhop set 10.0.0.1 \
    nhop del 10.0.0.99 \
    nlri ipv4/unicast add 1.0.0.0/24
# → nhop del 10.0.0.99 is no-op (doesn't match 10.0.0.1)
# → 1.0.0.0/24 nhop 10.0.0.1
```

## Raw Passthrough Commands

Send raw bytes with no validation ("trust me bro" mode):

| Command | What's sent | Header |
|---------|-------------|--------|
| `peer X raw <type> <enc> <data>` | Message payload | Ze adds |
| `peer X raw <enc> <data>` | Full packet | User provides FF*16 |

```bash
# Payload only (Ze adds 19-byte header)
peer 10.0.0.1 raw update hex 0000000e40010100400200400304c0a80101180a00
peer 10.0.0.1 raw notification hex 0602
peer 10.0.0.1 raw keepalive hex

# Full packet (user provides marker + length + type)
peer 10.0.0.1 raw hex ffffffffffffffffffffffffffffffff001303
```

⚠️ **No validation.** Can crash peer, violate FSM, send malformed messages.

## Error Messages

| Condition | Message |
|-----------|---------|
| Invalid encoding | `invalid hex/b64 in attributes: <error>` |
| Peer not found | `peer not found: 10.0.0.1` |
| Peer not established | `peer not established: 10.0.0.1` |
| Invalid family | `invalid family: <family>` |
| Empty nlri section | `nlri section requires 'add' and/or 'del' with prefixes` |
| Missing nlri | `missing nlri keyword after attributes` |
| Missing next-hop | `missing next-hop for announce` |
| Extended NH required | `extended next-hop not negotiated for IPv6 with IPv4 next-hop` |
| Family mismatch | `prefix family mismatch: 2001:db8::/32 is IPv6, expected IPv4` |
| Scalar add | `'origin' is a scalar attribute: use 'set' or 'del'` |
| Wire mode add/del | `wire mode only supports 'attr set'` |
| Del not present | `community del: 65000:99 not present` |

## Family Validation

| Check | Text Mode | Wire Mode |
|-------|-----------|-----------|
| IPv6 prefix in ipv4/* | ❌ Error | No validation |
| IPv4 prefix in ipv6/* | ❌ Error | No validation |

## Next-Hop Validation

| Check | Result |
|-------|--------|
| Announce without nhop | Error: entire block rejected |
| IPv4 nhop for IPv6 NLRI | Requires Extended NH capability (RFC 5549) |
| Extended NH not negotiated | Error: `extended next-hop not negotiated` |

## Removed Commands

| Old | New |
|-----|-----|
| `announce route <p> next-hop <nh>` | `update text nhop set <nh> nlri ipv4/unicast add <p>` |
| `announce attributes ... nlri ...` | `update text ... nhop set <nh> nlri ... add ...` |
| `withdraw route <p>` | `update text nlri ipv4/unicast del <p>` |
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

> **Note:** `watchdog set <name>` in `update text` commands is not yet implemented.
> The parser recognizes the syntax but the handler returns an error.
> Standalone `watchdog announce/withdraw` commands work as expected.

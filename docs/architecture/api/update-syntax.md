# UPDATE Command Syntax

## Status

**PARTIALLY IMPLEMENTED** - Text mode works, wire modes (hex/b64) implemented.

Legacy `announce route` / `announce attributes` commands removed.
The migration was summary 089, retired on 2026-08-01. `plan/learned/DESIGN-HISTORY.md`
carries one invariant from it, under "Configuration: YANG, parser, editor, reload":
next-hop sits outside the nlri section on input and inside it on output. For the
rest of the migration detail, that file's header gives the git-recovery route.

---

## Overview

Unified syntax for API commands and config files to send/receive BGP UPDATE messages.

## Core Syntax

```
update text [<attribute>]... (nlri | mp <family>) add <nlri>... [del <nlri>...]...
update <hex|b64> [<wire-attr>]... nlri <family> add <nlri>... [del <nlri>...]...
```

## Encodings

| Encoding | Attributes | NLRI | Use case |
|----------|------------|------|----------|
| `text` | Per-attribute keywords | Prefixes (1.0.0.0/24) | Human-readable |
| `hex` | `attr set <hex-bytes>` | Hex wire bytes | Debug |
| `b64` | `attr set <b64-bytes>` | Base64 wire bytes | Compact |
| `cursor` | Stateful delta (set/del) | Prefixes (announce-only) | Replay batching |
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- ParseUpdateText -->

## Text Mode - Per-Attribute Keywords

In text mode, each attribute uses its own keyword with `set`/`add`/`del`:

```
update text <attr> <op> <value> [<attr> <op> <value>]... nhop set <addr> (nlri | mp <family>) add prefix <prefix>...
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
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- scalar attribute handling -->

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
send bgp upstream1 update text nhop set self origin set igp nlri add prefix 1.0.0.0/24
send bgp upstream1 update text origin set igp nhop set self nlri add prefix 1.0.0.0/24
# Both equivalent - nhop accumulates
```

**Output format** (event from engine to plugin): attributes have no `set` keyword, `next` is the emitted next-hop keyword, and the header includes `remote as`:
```bash
# Output always shows next-hop with its nlri group:
peer 10.0.0.1 remote as 65001 received update 123 origin igp next 192.168.1.1 nlri ipv4/unicast add 1.0.0.0/24
#                                            ^^^^^^^^^^                 ^^^^^^^^^^^^^^^^
#                                            no "set"                   next-hop value for this NLRI group
```

**Multiple nlri groups** - each gets its own next-hop in output:
```bash
# Input:
send bgp * update text nhop set 10.0.0.1 nlri add prefix 1.0.0.0/24 \
                  nhop set 10.0.0.2 nlri add prefix 2.0.0.0/24

# Output:
peer 10.0.0.1 remote as 65001 received update 123 next 10.0.0.1 nlri ipv4/unicast add 1.0.0.0/24 next 10.0.0.2 nlri ipv4/unicast add 2.0.0.0/24
```

This ensures output is self-contained per nlri group.
<!-- source: internal/component/bgp/types/nexthop.go -- RouteNextHop, NextHopExplicit, NextHopSelf -->
<!-- source: internal/component/bgp/format/text_human.go -- appendFilterResultText -->

#### Next-Hop Overwriting

`nhop set` can appear multiple times - last one wins before each `nlri` section:

```bash
# nhop overwritten - 10.0.0.2 used for both prefixes
update text nhop set 10.0.0.1 nhop set 10.0.0.2 nlri add prefix 1.0.0.0/24,2.0.0.0/24

# Different nhop per nlri section
update text nhop set 10.0.0.1 nlri add prefix 1.0.0.0/24 \
            nhop set 10.0.0.2 nlri add prefix 2.0.0.0/24
# → 1.0.0.0/24 gets nhop 10.0.0.1
# → 2.0.0.0/24 gets nhop 10.0.0.2
```
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- nhop accumulation -->

#### Next-Hop and Withdraw

| Operation | nhop required |
|-----------|---------------|
| `nlri add ...`, `mp <family> add ...` | ✅ Yes |
| `nlri del ...`, `mp <family> del ...` | ❌ No (ignored if present) |

```bash
# Withdraw - nhop not needed
update text nlri del prefix 1.0.0.0/24

# Mixed - nhop required for add, ignored for del
update text nhop set 10.0.0.1 nlri add prefix 1.0.0.0/24 del prefix 2.0.0.0/24
```

#### Withdrawing a route the config also announces

`restore-configured`, written before the first `nlri` or `mp` section, changes
what the command's withdrawals do on each peer whose static config holds the
same route: the configured route is announced again in its place, instead of
the prefix being withdrawn. A withdrawn route the config does not hold is
withdrawn as usual, and the keyword does nothing to an announcement. It is
ExaBGP's `withdraw_owner`: when a script exits, the ExaBGP bridge writes it into
each withdrawal it sends for that script's routes, so a route that replaced a
configured one gives the configured route back
(`docs/architecture/exabgp-bridge.md`).

```
send bgp 10.0.0.1 update text restore-configured nlri ipv4/unicast del 1.1.1.1/32
```

A static route matches when its family, ADD-PATH path identifier, route
distinguisher and prefix are the withdrawn NLRI's. Its label stack is not
compared: a withdrawn labeled or VPN NLRI carries the RFC 8277 Section 2.4
Compatibility field where the labels were, and that field is ignored, so
`label` on the `del` need not repeat the configured stack. The same prefix
under another RD is another route and is withdrawn as asked. Written after a
section, the keyword is refused.

<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- kwRestoreConfigured, ParseUpdateText -->
<!-- source: internal/component/bgp/reactor/peer_static_restore.go -- restoreConfigured, staticRouteNamesNLRI, staticRouteKey -->
<!-- source: internal/exabgp/bridge/bridge_owner.go -- restoringWithdrawal -->

#### Withdrawing everything a peer was sent

A command withdraws the routes it names. To withdraw every route a peer's
Adj-RIB-Out holds, configured routes included, use
`clear bgp rib advertised <selector> [family <family>]` rather than an UPDATE
listing them (`docs/architecture/api/commands.md`). `clear bgp rib out` is the
opposite: it sends them all again. The ExaBGP bridge maps `clear adj-rib out`
to the first and `flush adj-rib out` to the second.

### Text Mode Examples

```bash
# Simple announce
send bgp upstream1 update text nhop set 10.0.0.1 nlri add prefix 1.0.0.0/24

# With attributes
send bgp upstream1 update text origin set igp med set 100 nhop set 10.0.0.1 nlri add prefix 1.0.0.0/24

# Multiple attributes
send bgp upstream1 update text origin set igp local-preference set 200 community set [ 65000:1 ] nhop set 10.0.0.1 nlri add prefix 1.0.0.0/24,2.0.0.0/24

# Withdraw (no nhop needed)
send bgp upstream1 update text nlri del prefix 1.0.0.0/24
```
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- handleUpdateText -->

## Wire Mode - Raw Bytes (hex/b64)

In wire modes, attributes are encoded as a single blob, nhop as wire bytes:

```
update hex [attr set <hex-bytes>] [nhop set <hex-nhop>] nlri <family> add <hex-nlri>...
update b64 [attr set <b64-bytes>] [nhop set <b64-nhop>] nlri <family> add <b64-nlri>...
```

Only `attr set` and `nhop set` supported (no `add`/`del` for wire bytes).

The attribute blob still passes the session writer's checks. An `attr set` whose
AS_PATH or AS4_PATH holds an AS_SET (type 1) or AS_CONFED_SET (type 4) segment is
written as the withdrawal of the UPDATE's routes, logged as a warning naming RFC 9774
Section 3 and counted on `ze_bgp_as_set_withdrawn_total`: RFC 9774 Section 3 forbids
advertising a set. Only `send bgp <peer> raw` writes such bytes as given.
<!-- source: internal/component/bgp/reactor/session_write.go -- writeUpdateBody -->
<!-- source: internal/component/bgp/reactor/session_rfc9774.go -- transmittedPathHoldsSet, writeUpdateAsWithdrawal -->

### Wire Mode Examples

```bash
send bgp upstream1 update hex attr set 400101400206020100001f94 nhop set 0a000001 nlri ipv4/unicast add 18010a00
send bgp upstream1 update b64 attr set QAEBQAIGAgEAAAH5 nhop set CgAAAQ== nlri ipv4/unicast add GAAKAAoA
```
<!-- source: internal/component/bgp/plugins/cmd/update/update_wire.go -- handleUpdateHex, handleUpdateB64 -->

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
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- list attribute operations -->

#### How `add` Works

`add` **prepends** to the current accumulated list - it does NOT replace:

```bash
# Single add - starts empty, adds [65000:1]
update text community add [ 65000:1 ] nhop set 10.0.0.1 nlri add prefix 1.0.0.0/24
# → community = [65000:1]

# Multiple adds - prepends (newest first)
update text community add [ 65000:1 ] community add [ 65000:2 ] nhop set 10.0.0.1 nlri add prefix 1.0.0.0/24
# → community = [65000:2, 65000:1]  (65000:2 prepended to [65000:1])

# set then add - set replaces, add prepends
update text community set [ 65000:1 ] community add [ 65000:2 ] nhop set 10.0.0.1 nlri add prefix 1.0.0.0/24
# → community = [65000:2, 65000:1]

# add then set - add is lost (set replaces everything)
update text community add [ 65000:1 ] community set [ 65000:2 ] nhop set 10.0.0.1 nlri add prefix 1.0.0.0/24
# → community = [65000:2]  (add was overwritten by set)
```

#### How `del` Works

`del [value]` removes the **first occurrence** of each specified value from the list.
**Errors if value not present.**

```bash
# set then del - removes first occurrence
update text community set [ 65000:1 65000:2 ] community del [ 65000:1 ] nhop set 10.0.0.1 nlri add prefix 1.0.0.0/24
# → community = [65000:2]

# del with duplicates - removes only FIRST occurrence
update text community set [ 65000:1 65000:2 65000:1 ] community del [ 65000:1 ] nhop set 10.0.0.1 nlri add prefix 1.0.0.0/24
# → community = [65000:2, 65000:1]  (only first 65000:1 removed)

# ERROR: del from empty list
update text community del [ 65000:1 ] nhop set 10.0.0.1 nlri add prefix 1.0.0.0/24
# → error: "community del: 65000:1 not present"

# ERROR: del non-existent value
update text community set [ 65000:1 ] community del [ 65000:99 ] nhop set 10.0.0.1 nlri add prefix 1.0.0.0/24
# → error: "community del: 65000:99 not present"

# del without value - removes entire attribute (always succeeds)
update text community set [ 65000:1 65000:2 ] community del nhop set 10.0.0.1 nlri add prefix 1.0.0.0/24
# → community removed entirely

# del without value on unset attribute - no-op, no error
update text community del nhop set 10.0.0.1 nlri add prefix 1.0.0.0/24
# → no change (community was not set)
```

#### Accumulation Across NLRI Sections

Attributes accumulate across `nlri` sections - each section captures a **snapshot**:

```bash
update text \
    community set [ 65000:1 ] nhop set 10.0.0.1 \
    nlri add prefix 1.0.0.0/24 \
    community add [ 65000:2 ] \
    nlri add prefix 2.0.0.0/24 \
    community add [ 65000:3 ] \
    nlri add prefix 3.0.0.0/24

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
update text nlri add prefix 1.0.0.0/24
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
update text med set 100 med set 200 nhop set 10.0.0.1 nlri add prefix 1.0.0.0/24
# → Works, med=200 (first set is lost)

# VALID but wasteful: add then set (add is lost)
update text community add [ 65000:1 ] community set [ 65000:2 ] nhop set 10.0.0.1 nlri add prefix 1.0.0.0/24
# → Works, community=[65000:2] (add is lost)

# VALID: del (no value) before set - clears nothing, then sets
update text community del community set [ 65000:1 ] nhop set 10.0.0.1 nlri add prefix 1.0.0.0/24
# → Works, community=[65000:1]
```

## Chained Sections (Text Mode)

Multiple attribute and nlri sections can be chained. Attributes accumulate:

```bash
send bgp upstream1 update text \
    origin set igp community set [ 65000:1 65000:2 ] nhop set 10.0.0.1 \
    nlri add prefix 1.0.0.0/24,2.0.0.0/24 \
    community add [ 65000:3 ] \
    nlri add prefix 3.0.0.0/24 \
    community del [ 65000:1 ] \
    nlri add prefix 4.0.0.0/24 del prefix 5.0.0.0/24
```

Result (add prepends, del removes first occurrence):
- `1.0.0.0/24`, `2.0.0.0/24` → community [65000:1, 65000:2]
- `3.0.0.0/24` → community [65000:3, 65000:1, 65000:2] (65000:3 prepended)
- `4.0.0.0/24` → community [65000:3, 65000:2] (first 65000:1 removed)
- `5.0.0.0/24` → withdraw

## API Command Format

**Text mode:**
```
send bgp <selector> update text [<attr> <set|add|del> <value>]... nhop set <addr> (nlri | mp <family>) add prefix <prefix>... [del prefix <prefix>...]
```

**Wire mode:**
```
send bgp <selector> update <hex|b64> attr set <bytes> nlri <family> add <nlri>... [del <nlri>...]
```

Each API command line = clean state.

**Examples:**
```bash
# Text mode - per-attribute keywords
send bgp upstream1 update text med set 100 nhop set 10.0.0.1 nlri add prefix 1.0.0.0/24

# Wire mode - attr set <bytes>
send bgp upstream1 update hex attr set 400101400206020100001f94 nlri ipv4/unicast add 18010a00 18020b00
send bgp upstream1 update b64 attr set QAEBQAIGAgEAAAH5 nlri ipv4/unicast add GAAKAAoA
```

## Config File Format

```
update {
    text {
        # Route group - attributes accumulate within block
        origin set igp;
        nhop set 10.0.0.1;
        community set [ 65000:1 65000:2 ];

        nlri ipv4/unicast add prefix 1.0.0.0/24,2.0.0.0/24;

        community add [ 65000:3 ];
        nlri ipv4/unicast add prefix 3.0.0.0/24;
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

        nlri ipv4/unicast add prefix 1.0.0.0/24,2.0.0.0/24;

        community add [ 65000:3 ];
        nlri ipv4/unicast add prefix 3.0.0.0/24;

        community del [ 65000:1 ];
        nlri ipv4/unicast add prefix 4.0.0.0/24 del prefix 5.0.0.0/24;
    }
}
```

## Wire Bytes Format

For `hex`/`b64`/`cbor` encodings:
- Spaces optional in NLRI - concatenated as raw wire bytes
- Spaces help track NLRI boundaries for UPDATE size splitting (max 4096 bytes)
<!-- source: internal/component/bgp/plugins/cmd/raw/ -- raw passthrough handler -->

```bash
# Both equivalent:
nlri ipv4/unicast add 18010a00 18020b00
nlri ipv4/unicast add 18010a0018020b00
```

For `update hex` and `update b64`, each NLRI token may hold one route or a
concatenated section. The parser uses the family's registered native framer,
including MUP's four-octet header, and rejects an unsupported family or a
truncated section. `addpath` keeps the four-octet Path Identifier on every
route; zero is a valid identifier. `del` selects withdrawal framing so a
labeled Compatibility field is not interpreted as an announcement label stack.
<!-- source: internal/component/bgp/plugins/cmd/update/update_wire.go -- parseWireNLRISection, splitWireNLRIs -->

## Received UPDATE Output

**Text format:**
```
peer 10.0.0.1 remote as 65001 received update 123 origin igp med 100 next 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24
peer 10.0.0.1 remote as 65001 received update 123 attr-99 400101... nlri ipv4/unicast add 10.1.0.0/24
```
<!-- source: internal/component/bgp/format/text_human.go -- appendFilterResultText, appendAttributeText -->

**JSON format:**
```json
{
  "message": {"type": "update", "direction": "received", "id": 123},
  "peer": {"address": "10.0.0.1", "local": {"address": "10.0.0.2", "as": 65000}, "remote": {"address": "10.0.0.1", "as": 65001}},
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
<!-- source: internal/component/bgp/yang/ze-bgp-api.yang -- plugin-encoding, plugin-format, plugin-ack -->

## Grammar

### Text Mode

`ParseUpdateText` reads a FLAT grammar. An attribute is a keyword followed by its
value, every attribute comes before the first `nlri` section, and `set`, `add` and
`del` are NLRI keywords only. The parser refuses `set` or `del` after an attribute
keyword and answers with the flat form to use.

```
<update-text>  := <attribute>* <nlri-section>+
<attribute>    := <attr-name> <value> | atomic-aggregate
<attr-name>    := origin | med | local-preference | as-path | origin-as |
                  community | large-community | extended-community |
                  aggregator | originator-id | cluster-list | aigp |
                  bgp-prefix-sid | bgp-prefix-sid-srv6 | attribute |
                  next-hop | path-information | rd | label

<nlri-section> := nlri [<modifier>]* <nlri-op>+ | nlri eor
               | mp <family> [<modifier>]* <nlri-op>+ | mp <family> eor
<modifier>     := path-information <id> | rd <value> | label <value>
<nlri-op>      := add <prefix>+ [watchdog set <name>] | del <prefix>+

<wire-attr>    := attr set <bytes>                     # hex/b64 mode only

<family>       := ipv4/unicast | ipv6/unicast | ipv4/mpls-vpn | ipv6/mpls-vpn | ...
```
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- ParseUpdateText, isAttributeKeyword -->

### Path Attributes

| Keyword | Value form | Attribute | Reference |
|---------|-----------|-----------|-----------|
| `origin` | `igp \| egp \| incomplete` | ORIGIN (1) | RFC 4271 Section 5.1.1 |
| `as-path` | `[ <asn>... ]` | AS_PATH (2) | RFC 4271 Section 5.1.2 |
| `next-hop` | `<address> \| self` | NEXT_HOP (3) | RFC 4271 Section 5.1.3 |
| `med` | `<uint32>` | MULTI_EXIT_DISC (4) | RFC 4271 Section 5.1.4 |
| `local-preference` | `<uint32>` | LOCAL_PREF (5) | RFC 4271 Section 5.1.5 |
| `atomic-aggregate` | none | ATOMIC_AGGREGATE (6) | RFC 4271 Section 5.1.6 |
| `aggregator` | `<asn>:<ipv4>` | AGGREGATOR (7) | RFC 4271 Section 5.1.7, RFC 6793 |
| `community` | `[ <comm>... ]` | COMMUNITY (8) | RFC 1997 |
| `originator-id` | `<ipv4>` | ORIGINATOR_ID (9) | RFC 4456 Section 8 |
| `cluster-list` | `[ <ipv4>... ]` | CLUSTER_LIST (10) | RFC 4456 Section 8 |
| `extended-community` | `[ <ec>... ]` | EXTENDED_COMMUNITIES (16), and IPV6_EXTENDED_COMMUNITIES (25) for an IPv6 redirect-to-IP target or an IPv6 route-target redirect | RFC 4360, RFC 5701, RFC 8956 |
| `aigp` | `<uint64>` | AIGP (26) | RFC 7311 Section 3 |
| `large-community` | `[ <lc>... ]` | LARGE_COMMUNITY (32) | RFC 8092 |
| `bgp-prefix-sid` | `[ <index> ]` or `[ <index>, [ ( <base>,<range> ) ... ] ]` | BGP Prefix-SID (40) | RFC 8669 Sections 3.1 and 3.2 |
| `bgp-prefix-sid-srv6` | `( <service> <sid> [<behavior>] [<structure>] )` | BGP Prefix-SID (40) | RFC 8669, RFC 9252 Section 3 |
| `attribute` | `[ 0x<code> 0x<flags> 0x<value> ]` | any type code, given as its wire octets | RFC 4271 Section 4.3 |

`bgp-prefix-sid` and `bgp-prefix-sid-srv6` write the one BGP Prefix-SID
attribute, so a command that gives either of them twice, or both, is refused
with an error that says to keep one: an UPDATE carries an attribute once (RFC
4271 Section 6.3).
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- errPrefixSIDTwice -->

An `<ec>` in `extended-community` takes one of seven forms.

| Form | Example | Meaning |
|------|---------|---------|
| named | `target:65000:1`, `origin:65000:1.2.3.4`, `origin:2345:101124105`, `origin:70500:5000`, `mup:10:10` | the type keyword picks the encoding (`target`, `origin`, `mup`). The administrator picks the type: an IPv4 address is 0x01, an AS above 65535 or written with `L` is 0x02 (RFC 5668 Section 2) with a 2-octet value, any other AS is 0x00 with a 4-octet value written as a number or a dotted quad (RFC 4360 Sections 3.1 and 5). `mup:<2-octet>:<4-octet>` is type 0x0C sub-type 0x00 (draft-ietf-bess-mup-safi Section 3.2) |
| FlowSpec action | `discard`, `copy-to-nexthop`, `rate-limit:1000` | RFC 8955 Section 7 |
| FlowSpec IPv6 route-target redirect | `redirect:[2001:db8::1]:100` | RFC 8956 Section 6.1: the rt-redirect-ipv6 action, a redirect to the VRF importing the IPv6-address-specific route-target `[<IPv6>]:<0-65535>`. A 20-octet community of type 0x000D (RFC 5701 Section 2: 16-octet address, 2-octet value) in IPV6_EXTENDED_COMMUNITIES (attribute 25). The brackets tell it from the 8-octet `redirect:<administrator>:<value>`; it is the form ze prints for a received one, so it round-trips. An IPv4 address in brackets, a zone, or a value above 65535 is refused |
| FlowSpec redirect to IP | `redirect-to-nexthop 192.0.2.1`, `copy-to-nexthop 2001:db8::1` | draft-ietf-idr-flowspec-redirect-ip-16 Section 2: two words, the redirect-to-IP community holding the address, with the C (copy) bit for `copy-to-nexthop`. An IPv4 address is an 8-octet community (type 0x010C) in EXTENDED_COMMUNITIES; an IPv6 address is a 20-octet one (0x000C) in IPV6_EXTENDED_COMMUNITIES (attribute 25, RFC 5701 Section 2). Written alone, `copy-to-nexthop` is the one-word 0x0800 copy action of draft-simpson-idr-flowspec-redirect-02, which copies to the route's next hop; `redirect-to-nexthop-draft` is its redirect. `redirect-to-nexthop` with no address is refused. It works as the first word (`extended-community redirect-to-nexthop 192.0.2.1`) and inside a list |
| FlowSpec traffic marking and action | `mark:46`, `traffic-action:sample-terminal` | RFC 8955 Sections 7.5 and 7.3: `mark:<dscp>` is type 0x8009 with the DSCP (0 to 63, a larger value is refused) in the low six bits; `traffic-action:` takes `sample`, `terminal`, `sample-terminal` or `none` and is type 0x8007 with the S and T bits. Both are the spelling ze writes when it decodes the community |
| FlowSpec interface set | `interface-set:transitive:input:1234:10`, `interface-set:non-transitive:output:65000:3` | draft-ietf-idr-flowspec-interfaceset Section 5: apply the filter on interface group 3 (0 to 16383) of AS 65000, inbound, outbound or both (`input-output`). Type 0x07, or 0x47 non-transitive; `interface-set:input:1234:10` is transitive |
| VPLS Layer2 Info | `l2info:19:0:1500:111` | RFC 4761 Section 3.2.4: `l2info:<encaps>:<control>:<mtu>:<reserved>`, type 0x800A. Encaps and control are 0 to 255, MTU and the last field 0 to 65535; the last field is written as given, because ExaBGP sends a VPLS multihoming preference there |
| raw | `0x0002FDE900000001` | the 8 octets verbatim |

The raw form is `0x` followed by exactly 16 hex digits, because RFC 4360 Section 2
encodes "each Extended Community as an 8-octet quantity". It is how an operator
writes a community Ze has no keyword for, and it is the form ExaBGP writes. Any
other digit count is refused by name rather than padded or truncated. The config
file accepts the same forms, from the same tables.
<!-- source: internal/core/bgp/attribute/extcomm_hex.go -- ParseExtendedCommunityHex -->
<!-- source: internal/core/bgp/attribute/extcomm_keyword.go -- parseInterfaceSet -->
<!-- source: internal/core/bgp/attribute/flowspec_action.go -- ParseFlowSpecRedirectToIP -->

Ze emits the attributes in ascending type-code order, whatever order the command
wrote them in (RFC 4271 Section 5).

`attribute` writes one path attribute from its octets. The config file's
`attribute` is read by the same function and follows the same rules below, and
it is the spelling of ExaBGP's generic attribute. The words come in this order: the
type code, then the flags octet, then the value. The code and the flags are one
octet each, written `0x` and two hex digits; the value is `0x` and an even
number of hex digits (`0x` alone is an empty attribute). The command may carry
several, of different codes:

```
attribute [ 0x99 0xc0 0x01020304 ] attribute [ 0x98 0xd0 0x00000064 ]
```

The flags go to the wire as written only when RFC 4271 Section 4.3 lets a
sender write them, and every other octet is refused with the octet to write
instead:

| Refused flags | Why |
|---------------|-----|
| A low-order bit set (`0x01` to `0x0f`) | "They MUST be zero when sent" |
| The Optional bit clear on a code that is not a well-known attribute | A clear Optional bit marks a well-known attribute; an unknown code can only be optional |
| The Partial bit on a well-known or an optional non-transitive attribute | "the Partial bit MUST be set to 0" |
| The Partial bit on a code a keyword writes (AGGREGATOR, COMMUNITIES, the extended communities, the large communities, the Prefix-SID) | Ze sends that code from its decoded value, as it sends the keyword, with the attribute's own flags, so the bit would be dropped. RFC 4271 Section 5 sets Partial on an optional transitive attribute "not attached by the originator", or one an AS passed along unrecognised; Ze originates the route and recognises the attribute |
| Optional or Transitive bits a known attribute's specification fixes otherwise | A receiver treats the attribute as malformed (RFC 7606 Section 3(c)) |

The Partial bit of an optional transitive attribute and the Extended Length bit
are the sender's choice for a code no keyword writes. Ze sets Extended Length
itself for a value above 255 octets. A value is at most 65535 octets, the most
the two-octet Attribute Length holds; a longer one is refused.

Three more refusals. A code Ze builds from another part of the command is
refused by name: NEXT_HOP (3, from `next-hop`), MP_REACH_NLRI and
MP_UNREACH_NLRI (14 and 15, from the `mp` section), AS4_PATH and AS4_AGGREGATOR
(17 and 18, from `as-path` and `aggregator` for a peer without four-octet AS).
A code a keyword writes (ORIGIN, AS_PATH, MED, LOCAL_PREF, ATOMIC_AGGREGATE,
AGGREGATOR, COMMUNITIES, ORIGINATOR_ID, CLUSTER_LIST, the extended communities,
AIGP, the large communities and the Prefix-SID) is decoded by that attribute's
own wire decoder, the one an UPDATE from a peer meets, and then stands for the
keyword: `attribute [ 0x04 0x80 0x00000064 ]` is `med 100`, and a generic
ORIGIN replaces the IGP default rather than adding a second ORIGIN. AS_PATH and
AGGREGATOR are read with four-octet AS numbers. A value the decoder refuses, or
one with octets it leaves unread, is refused as malformed with the octets to
write: a two-octet MED, a five-octet LOCAL_PREF, a large community value that is
not a multiple of 12 octets. A large community value holding one large community
twice is refused as that repeat, with the value carrying it once, because
RFC 8092 Section 3 says "Duplicate BGP Large Community values MUST NOT be
transmitted." An empty COMMUNITIES, CLUSTER_LIST, extended
community or large community list is refused too, because RFC 7606 and RFC 8092
hold a zero-length one malformed. The config file stores the decoded value in
the field the keyword fills; an AS_PATH holding an AS_SET or AS_CONFED_SET has
no such field there, so it is kept and the route is withdrawn rather than
advertised (RFC 9774 Section 3). A generic AIGP in the config file must hold the
one metric TLV `aigp` writes. A code given twice, by two `attribute`
statements or by `attribute` and the keyword, is refused as an attribute given
twice: RFC 4271 Section 6.3 makes such an UPDATE a Malformed Attribute List.
<!-- source: internal/component/bgp/textparse/rawattr.go -- ParseRawAttribute, CheckRawAttributeFlags, rawAttributeBuilt, attributeKeywordCodes, decodeTypedRawAttribute, rawAttributeNonEmpty, largeCommunityRepeat -->
<!-- source: internal/component/bgp/config/routeattr.go -- parseRawAttributeInto, rawASPathInto, rawAIGPInto -->
<!-- source: internal/component/bgp/plugins/cmd/update/update_text_attribute.go -- parseRawAttributeText, withRawAttrs -->

`atomic-aggregate` is the only keyword that takes no value: RFC 4271 Section 4.3 f)
makes it "a well-known discretionary attribute of length 0", so its presence in the
command is the whole value.

`aggregator` refuses AS 0, because RFC 7607 Section 2 states "A BGP speaker MUST NOT
originate or propagate a route with an AS number of zero in the AS_PATH, AS4_PATH,
AGGREGATOR, or AS4_AGGREGATOR attributes." `aggregator`, `originator-id` and each
`cluster-list` id carry a four-octet address field, so an IPv6 value is refused
rather than truncated.

`bgp-prefix-sid-srv6` takes its value in PARENTHESES, which is the spelling ExaBGP
writes:

```
bgp-prefix-sid-srv6 ( l3-service 2001:db8:1:1:: 0x48 [64,24,16,0,0,0] )
```

| Field | Form | Meaning |
|-------|------|---------|
| service | `l3-service` or `l2-service` | RFC 9252 Section 2: SRv6 L3 Service TLV (type 5) or L2 Service TLV (type 6) |
| sid | IPv6 address | the 16-octet SRv6 SID |
| behavior | `0xNN`, optional | SRv6 Endpoint Behavior, RFC 8986. Absent means 0 |
| structure | `[LB,LN,Func,Arg,TransLen,TransOffset]`, optional | the SRv6 SID Structure Sub-Sub-TLV of RFC 9252 Section 3.2.1, six values, each one octet |

The six structure values are locator-block length, locator-node length, function
length, argument length, transposition length and transposition offset, in that
order. Six is the count RFC 9252 Section 3.2.1 fixes, so five or seven is refused
by name rather than padded.

When a structure is supplied, the first four lengths must total at most 128 bits
and cover the complete transposed range. A range ending exactly at that total is
valid under RFC 9252 erratum 7817. Zero transposition length requires zero offset,
and every transposed bit must already be zero in the advertised SID; the encoder
does not move those bits into the route's label field.

A non-zero transposition length or offset is accepted only with routes whose
NLRI has a label field to carry the transposed bits: the IPv4 and IPv6 VPN
families and EVPN. With any other family (IPv4 or IPv6 unicast, labeled
unicast) RFC 9252 Section 7 requires both to be 0, and the route is refused
with an error that names the family and the structure to write instead. On
the families that have a label field, the transposition length must also fit it:
at most 20 bits for IPv4 and IPv6 VPN and at most 24 bits for EVPN (RFC 9252
Sections 5 and 6), or the route is refused the same way. The
family is the one of the `nlri` or `mp` section, or of the configured route, the
attribute is advertised with, so the same attribute can be accepted for one
section and refused for another.
<!-- source: internal/core/bgp/attribute/prefixsid.go -- CheckSRv6TranspositionForFamily -->
<!-- source: internal/core/bgp/nlri/nlrisplit/transposition.go -- TranspositionLabelWidth -->

The encoder accepts a nonzero argument length only for End.DT2M (`0x0018`), the
endpoint behaviour with arguments that it supports. Extra text after the
structure or endpoint behaviour is refused. These are origination checks; a
received Prefix-SID keeps its original wire bytes for receive-side processing.

The command reaches the same encoder as the config file's `bgp-prefix-sid-srv6`
statement, so the two spellings produce identical attribute bytes.

```bash
send bgp edge1 update text \
  bgp-prefix-sid-srv6 ( l3-service 2001:db8:1:1:: 0x48 [64,24,16,0,0,0] ) \
  next-hop 2001::1 mp ipv4/mup add route-type mup-isd rd 100:100 prefix 10.0.1.0/24
```
<!-- source: internal/core/bgp/attribute/prefixsid.go -- EncodePrefixSIDSRv6 -->

```bash
send bgp rr1 update text \
  originator-id 10.0.99.12 cluster-list [ 3.3.3.3 192.168.201.1 ] \
  atomic-aggregate aggregator 65000:10.0.0.1 aigp 100 \
  nlri add 10.0.0.0/24
```
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- parseCommonAttributeText -->

### Standalone Watchdog Commands
```
request bgp watchdog announce <name>   # send all routes in pool to peers
request bgp watchdog withdraw <name>   # withdraw all routes in pool from peers
```

### Scalar `del [<value>]` Semantics

- `<scalar> del` - remove attribute unconditionally
- `<scalar> del <value>` - remove only if current value matches, else error

### Attribute Types

| Type | Attributes |
|------|------------|
| **Scalar** | `origin`, `med`, `local-preference`, `origin-as`, `aggregator`, `originator-id`, `aigp`, `bgp-prefix-sid`, `bgp-prefix-sid-srv6`, `next-hop`, `path-information`, `rd`, `label` |
| **List** | `as-path`, `community`, `large-community`, `extended-community`, `cluster-list`, `attribute` (one per type code) |
| **Flag** | `atomic-aggregate` |

### Accumulator → Family Support

| Accumulator | Value | Valid for | Error for |
|-------------|-------|-----------|-----------|
| `rd` | `<ASN:NN>` or `<IP:NN>` | `*-vpn` families | All others |
| `label` | `<label>` or `[ <label>... ]` | `*-vpn`, `*-labeled` families | All others |
| `path-information` | `<uint32>` or dotted `<a.b.c.d>` | Any (if ADD-PATH negotiated) | Ignored if not negotiated |

RFC 7911 Section 3 gives the Path Identifier four octets and reserves no value,
so `path-information 0` names the identifier zero and is not a way to say that
no identifier was written. The keyword's presence is what the parser records
(`nlriAccum.HasPathID`), and `encodeViaRegistry` prepends the four octets for
the families a plugin encodes, whose encoders answer with the payload alone.
Whether those octets reach the wire is still the session's ADD-PATH
negotiation, which `WriteNLRI` reads.

The identifier is written as one decimal number, 0 to 4294967295, or in the
dotted form a.b.c.d, which names the four octets in wire order: `0.0.0.1` is 1
and `1.2.3.4` is 16909060. ExaBGP writes the dotted form. The config's
`path-information` leaf reads the same two forms through the same parser,
`parse.PathID`, at the top level and inside a section alike. Any other token,
an IPv6 address included, is refused with an error that names it and both
accepted forms.
<!-- source: internal/core/parse/pathid.go -- PathID -->

A top-level `path-information` also applies to the section of a family a
plugin encodes (mup, mvpn, sr-policy, rtc): the NLRI the plugin builds goes out
with that Path Identifier ahead of it (RFC 7911 Section 3). Written right after
such a family, before its `add` or `del`, it is refused as a missing `add` or
`del`; written inside the section, the plugin's grammar refuses it as a word it
does not take. A `path-information` written inside another family's section
belongs to that section alone.
<!-- source: internal/component/bgp/plugins/cmd/update/update_text_nlri.go -- parseRegistryNLRISection, wireNLRI -->
<!-- test: test/plugin/update-text-sr-policy-addpath.ci -->

A section of a family a plugin encodes (mup, mvpn, sr-policy, rtc) is one NLRI, so
its `add` or `del` comes once, right after the family. A second `add` or `del`
later in the section is refused. The families Ze parses itself (unicast,
multicast, vpn, labeled, flowspec, vpls, evpn) keep switching mode on a later
`add` or `del`: `nlri add 10.0.0.0/24 del 10.0.1.0/24`.
<!-- source: internal/component/bgp/plugins/cmd/update/update_text_nlri.go -- parseRegistryNLRISection -->

An `sr-policy` `add` section also writes the policy's Tunnel Encapsulation
sub-TLVs after its three NLRI fields, in the words the config route uses
(`preference`, `priority`, `binding-sid`, `segment-list`, `policy-name`,
`candidate-path-name`). The section goes to the parser config uses, so both
build the same bytes, and a word after the sub-TLVs is refused. The attribute
joins the ones written before the section, in ascending type-code order, and a
type already written there is refused. A `del` section names the three NLRI
fields only. The plugin declares this with `NLRISectionCarriesAttributes`.

```bash
send bgp * update text next-hop 192.0.2.1 mp ipv4/sr-policy add distinguisher 0 \
  color 300 endpoint 10.0.0.3 preference 300 segment-list weight 1 segment type-a mpls 16001
```
<!-- source: internal/component/bgp/plugins/cmd/update/update_text_nlri.go -- parseRegistryRouteSection -->
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- withSectionAttrs -->
<!-- source: internal/component/plugin/registry/registry.go -- SectionRouteParserFor -->

All three are accepted in TWO positions, before the first `nlri` section and
inside one, and they mean the same thing in both. `path-information` used to be
accepted inside a section only, so a command that wrote all three together had
two read and the third refused as an unknown token.

`label` takes a LIST because an MPLS label stack is one. RFC 8277 Section 2
encodes "one or more Label fields" of three octets each and sets the Bottom of
Stack bit on the last, so `label [ 110 200 ]` writes two labels and `label 110`
writes one. Each value is held to the 20 bits RFC 3032 Section 2.1 gives the
Label field.

```bash
send bgp * update text rd 63333:100 label [ 110 200 ] next-hop 10.0.99.12 \
  mp ipv4/mpls-vpn add 128.0.64.0/18
```
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- parseLabelStack, parsePathInfoFlat -->
<!-- source: internal/component/bgp/types/types.go -- UpdateTextResult, NLRIGroup -->

### Wire Mode (hex/b64)
```
<command>       := send bgp <selector> update <hex|b64> <wire-sections>
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
| ipv4/mvpn, ipv6/mvpn | ✅ | RFC 6514 MCAST-VPN (route types 5, 6, 7) |
| ipv4/mup, ipv6/mup | ✅ | MUP SAFI |
| ipv4/rtc | ✅ | RFC 4684 Route Target membership |
| flowspec | ❌ | Complex encoding, use text |

A family this parser does not read itself hands its whole token run to the NLRI
encoder its plugin registered, so the grammar after `add` or `del` belongs to that
plugin. MCAST-VPN and MUP are both written that way:

```bash
send bgp edge1 update text next-hop 10.10.6.3 extended-community [ target:192.168.94.12:5 ] \
  mp ipv4/mvpn add shared-join rp 10.99.199.1 group 239.251.255.228 rd 65000:99999 source-as 65000
```

| Field | Route types | Meaning |
|-------|-------------|---------|
| `shared-join`, `source-join`, `source-ad` | 6, 7, 5 | RFC 6514 Section 4 route type |
| `rp` or `source` | all | the C-RP address for a shared join, the C-S address otherwise |
| `group` | all | the C-G address |
| `rd` | all | RFC 4364 Route Distinguisher, required |
| `source-as` | 6 and 7 only | the Source AS field RFC 6514 Section 4.6 gives a C-multicast route |

The AFI decides the address family: an IPv6 source or group under `ipv4/mvpn` is
refused, and so is a missing `source-as` on a join route, because AS 0 on the wire
cannot be told from an AS the operator chose.
<!-- source: internal/component/bgp/plugins/nlri/mvpn/encode.go -- EncodeNLRIHex -->

A Route Target membership (RFC 4684 Section 4) is `default`, the zero-length
membership in every route target, or an origin AS and a route target, which the
encoder writes as the full 96-bit prefix:

```bash
send bgp edge1 update text next-hop 192.0.2.1 mp ipv4/rtc add origin-as 65000 route-target 65000:1
send bgp edge1 update text next-hop 192.0.2.1 mp ipv4/rtc add default
send bgp edge1 update text mp ipv4/rtc del origin-as 65000 route-target 65000:1
```

The route target takes the spellings `extended-community` takes for one,
`<asn>:<n>`, `<ip>:<n>` or `target:...`, and any other community is refused. A
membership missing its origin AS or its route target is refused rather than
written with a zero. Inside a plugin section `origin-as` is the section's word,
not the top-level `origin-as` directive, which still applies when written before
the section.
<!-- source: internal/component/bgp/plugins/nlri/rtc/encode.go -- EncodeNLRIHex, parseMembership -->
<!-- source: internal/component/bgp/plugins/cmd/update/update_text_nlri.go -- endsRegistrySection -->

## Attribute Wire Bytes

- **Included:** All path attributes in wire order
- **Excluded:** MP_REACH_NLRI (type 14), MP_UNREACH_NLRI (type 15)
- NLRI extracted and encoded separately by family

## Per-Section Modifiers

An `nlri` or `mp <family>` section takes three modifiers before its first `add`
or `del`. Each applies to every prefix of that section, never to one prefix:

| Modifier | Syntax | Sets |
|----------|--------|------|
| `path-information` | `path-information <id>` (alias `info`), `<id>` decimal or dotted `a.b.c.d` | the ADD-PATH path identifier (RFC 7911) |
| `rd` | `rd <asn>:<num>` or `rd <ip>:<num>` | the route distinguisher of a VPN family |
| `label` | `label <num>` or `label [ <num>... ]` | the label stack, read by the parser the top-level `label` uses |

```bash
send bgp upstream1 update text origin set igp nhop set 10.0.0.1 \
    nlri path-information 7 add prefix 1.0.0.0/24,2.0.0.0/24
```

After `add`, a section also takes `watchdog set <name>`, which the parser reads
and the handler refuses (see "Watchdog Commands"). A section takes no path attribute and no `nhop`: those are
written before the section, and a section that follows a new `nhop set` or
attribute keyword uses the new value (see "Chained Sections").
<!-- source: internal/component/bgp/plugins/cmd/update/update_text_nlri.go -- parseINETSection -->

## VPN Modifiers

NLRI modifiers for VPN families (not path attributes):

```bash
# L3VPN with RD and label
send bgp upstream1 update text extended-community set [ target:65000:100 ] nhop set 10.0.0.1 \
    mp ipv4/mpls-vpn rd 65000:100 label 1000 add prefix 10.0.0.0/24

# Multiple prefixes same RD/label
mp ipv4/mpls-vpn rd 65000:100 label 1000 add prefix 10.0.0.0/24,10.0.1.0/24
```

| Modifier | Syntax | Example |
|----------|--------|---------|
| `rd` | `rd <asn>:<num>` or `rd <ip>:<num>` | `rd 65000:100` |
| `label` | `label <num>` | `label 1000` |
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- VPN modifiers (rd, label) -->

## End-of-RIB (EOR)

RFC 4724 End-of-RIB marker signals completion of initial routing table exchange.

### Syntax

```bash
update text nlri eor             # IPv4 unicast: an UPDATE with no NLRI and no attributes
update text mp <family> eor     # any other family: an empty MP_UNREACH_NLRI
```

### Examples

```bash
# IPv4 unicast EOR
send bgp upstream1 update text nlri eor

# IPv6 unicast EOR
send bgp upstream1 update text mp ipv6/unicast eor

# Multiple families in one command
send bgp upstream1 update text nlri eor mp ipv6/unicast eor

# EOR with NLRI in same command
send bgp upstream1 update text mp ipv6/unicast eor nhop set 10.0.0.1 nlri add prefix 10.0.0.0/24
```

### Wire Format

| Family | Wire Bytes |
|--------|------------|
| IPv4 unicast | Empty UPDATE: `withdrawn=0, path_attr=0, nlri=0` |
| Other families | MP_UNREACH_NLRI with AFI/SAFI, no prefixes |
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- EOR handling -->

### Who gets the marker, and how often

The marker goes to every selected peer that is established and negotiated the
family, each time the command asks for it, as ExaBGP's `announce eor` does. A
peer that did not negotiate the family is skipped (RFC 4760 Section 8), and a
command that reaches no peer answers `error`, naming the family. While a peer's
own initial sync still owes the family's marker, the command writes nothing to
that peer, answers as handled, and the sync writes the one marker.

A plugin that replays the initial routing update (the route server, the route
reflector) closes the replay with the same command and the route metadata
`replay-eor` set to `true` (`UpdateRouteWithMeta`). That marker is the
session's one End-of-RIB for the family (RFC 4724 Section 2): it goes out only
when no marker for the family has gone out on the session yet, so a replay
that finishes after the reactor's initial sync does not repeat it.
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- AnnounceEOR, AnnounceReplayEOR, announceEOR -->
<!-- source: internal/component/bgp/types/types.go -- ReplayEORMeta -->

## VPLS (L2VPN/VPLS)

RFC 4761 Virtual Private LAN Service.

### Syntax

```bash
update text mp l2vpn/vpls add rd <rd> ve-id <n> ve-block-offset <n> ve-block-size <n> label-base <n>
update text mp l2vpn/vpls del rd <rd> ve-id <n> ve-block-offset <n> ve-block-size <n> label-base <n>
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
send bgp upstream1 update text mp l2vpn/vpls add rd 1:1 ve-id 1 ve-block-offset 0 ve-block-size 10 label-base 1000

# Withdraw VPLS
send bgp upstream1 update text mp l2vpn/vpls del rd 1:1 ve-id 1 ve-block-offset 0 ve-block-size 10 label-base 1000

# EOR for VPLS
send bgp upstream1 update text mp l2vpn/vpls eor
```
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- VPLS NLRI parsing -->

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
update text mp l2vpn/evpn add mac-ip rd <rd> mac <mac> [ip <ip>] label <n>
```

| Field | Type | Description |
|-------|------|-------------|
| `mac` | MAC address | Required. Format: `00:11:22:33:44:55` |
| `ip` | IP address | Optional. IPv4 or IPv6 |

**Examples:**

```bash
# MAC only
send bgp upstream1 update text mp l2vpn/evpn add mac-ip rd 1:1 mac 00:11:22:33:44:55 label 100

# MAC with IPv4
send bgp upstream1 update text mp l2vpn/evpn add mac-ip rd 1:1 mac 00:11:22:33:44:55 ip 192.168.1.1 label 100

# MAC with IPv6
send bgp upstream1 update text mp l2vpn/evpn add mac-ip rd 1:1 mac 00:11:22:33:44:55 ip 2001:db8::1 label 100

# With ESI and Ethernet Tag
send bgp upstream1 update text mp l2vpn/evpn add mac-ip rd 1:1 esi 00:01:02:03:04:05:06:07:08:09 etag 100 mac 00:11:22:33:44:55 label 100
```

### Type 3: Inclusive Multicast Ethernet Tag

```bash
update text mp l2vpn/evpn add multicast rd <rd> ip <originator-ip>
```

| Field | Type | Description |
|-------|------|-------------|
| `ip` | IP address | Required. Originator IP |

**Example:**

```bash
send bgp upstream1 update text mp l2vpn/evpn add multicast rd 1:1 ip 192.168.1.1
```

### Type 5: IP Prefix Route

```bash
update text mp l2vpn/evpn add ip-prefix rd <rd> prefix <prefix> [esi <esi> | gateway <ip>] [overlay-index none|esi|gateway-ip] label <n>
```

| Field | Type | Description |
|-------|------|-------------|
| `prefix` | IP prefix | Required. IPv4 or IPv6 prefix |
| `esi` | ESI | Optional. ESI Overlay Index; non-zero only with `overlay-index esi` |
| `gateway` | IP address | Optional. GW IP Overlay Index; non-zero only with `overlay-index gateway-ip` |
| `overlay-index` | `none`, `esi`, `gateway-ip` | Optional. The Overlay Index the route carries (RFC 9136 Section 3.2) |

**Overlay Index (RFC 9136):** `overlay-index` names which field is the Overlay
Index, and the encoder holds the other field to zero (RFC 9136 Section 3.1). A
non-zero `esi` needs `overlay-index esi`, a non-zero `gateway` needs
`overlay-index gateway-ip`, and without the keyword both must be zero, so the
route has no Overlay Index and the BGP next hop forwards it. The refusal names
the field, the reason and the keyword to add.

**Examples:**

```bash
# IPv4 prefix (direct forwarding)
send bgp upstream1 update text mp l2vpn/evpn add ip-prefix rd 1:1 prefix 10.0.0.0/24 label 100

# IPv6 prefix
send bgp upstream1 update text mp l2vpn/evpn add ip-prefix rd 1:1 prefix 2001:db8::/32 label 100

# With GW IP Overlay Index (recursive resolution via RT-2)
send bgp upstream1 update text mp l2vpn/evpn add ip-prefix rd 1:1 prefix 10.0.0.0/24 gateway 192.168.1.254 overlay-index gateway-ip label 100

# With ESI Overlay Index (recursive resolution via RT-1)
send bgp upstream1 update text mp l2vpn/evpn add ip-prefix rd 1:1 prefix 10.0.0.0/24 esi 00:01:02:03:04:05:06:07:08:09 overlay-index esi label 100
```

### EVPN EOR

```bash
send bgp upstream1 update text mp l2vpn/evpn eor
```
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- EVPN NLRI parsing -->

## Conditional nhop del

`nhop del <addr>` only removes if current nhop matches:

```bash
send bgp upstream1 update text nhop set 10.0.0.1 \
    nlri add prefix 1.0.0.0/24 \
    nhop del 10.0.0.1 \
    nhop set 10.0.0.2 \
    nlri add prefix 2.0.0.0/24
# → 1.0.0.0/24 nhop 10.0.0.1
# → 2.0.0.0/24 nhop 10.0.0.2 (del matched, then set)

send bgp upstream1 update text nhop set 10.0.0.1 \
    nhop del 10.0.0.99 \
    nlri add prefix 1.0.0.0/24
# → nhop del 10.0.0.99 is no-op (doesn't match 10.0.0.1)
# → 1.0.0.0/24 nhop 10.0.0.1
```

## Raw Passthrough Commands

Raw is an explicit diagnostic injection operation. The peer must attach the
program with `send [ raw ]`; no other send type permits it. Raw UPDATE requests
are not duplicate-suppressed, but the existing AIGP origination policy,
negotiated PATHS-LIMIT filtering and Label-Index Reserved/Flags normalization
still apply. PATHS-LIMIT can refuse malformed input before any bytes are sent.

The encoding is `hex` or `b64`, the data is always supplied, and the message type
takes the `type` keyword in front of it. The model declares all three
(`internal/component/bgp/plugins/cmd/raw/yang/ze-raw-cmd.yang`), so a word
outside either set is refused before the handler runs.

| Command | What's sent | Header |
|---------|-------------|--------|
| `send bgp X raw <enc> <data> type <type>` | Message payload | Ze adds |
| `send bgp X raw <enc> <data>` | Full packet | User provides FF*16; recognized UPDATE headers are rebuilt |

```bash
# Payload only (Ze adds 19-byte header)
send bgp upstream1 raw hex 0000000e40010100400200400304c0a80101180a00 type update
send bgp upstream1 raw hex 0602 type notification

# Full packet (user provides marker + length + type)
send bgp upstream1 raw hex ffffffffffffffffffffffffffffffff001303
```

A message with no body is written in full packet mode, because the type keyword
introduces a body and a header-only frame has none. A KEEPALIVE is
`send bgp upstream1 raw hex ffffffffffffffffffffffffffffffff001304`: sixteen marker
octets, the length 0x0013, and the type 0x04.
<!-- source: internal/component/bgp/plugins/cmd/raw/ -- raw passthrough -->

Malformed diagnostic bytes can violate the peer's protocol expectations. When
an accepted raw operation cannot be accounted for in Ze's outbound ownership,
Ze flushes those bytes, then retires that exact connection. Successful emission
still returns success; the close reason identifies the diagnostic safety reset,
not a transport write failure. A write or flush failure instead reports its
actual error. This reset is Ze's sender-side safety policy, not an RFC 7606
receiver-handling requirement. Reconnection restores ordinary route management.
No route ownership or replay receipt is invented from opaque output, and an old
operation cannot retire a replacement connection.
<!-- source: internal/component/bgp/reactor/session_write.go -- SendRawMessage, finishRawWrite -->

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
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- error messages -->

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
<!-- source: internal/component/bgp/reactor/peer.go -- resolveNextHop -->

## Removed Commands

| Old | New |
|-----|-----|
| `announce route <p> next-hop <nh>` | `update text nhop set <nh> nlri add prefix <p>` |
| `announce attributes ... nlri ...` | `update text ... nhop set <nh> nlri ... add prefix ...` |
| `withdraw route <p>` | `update text nlri del prefix <p>` |
| `announce watchdog <name>` | `request bgp watchdog announce <name>` |
| `withdraw watchdog <name>` | `request bgp watchdog withdraw <name>` |

## Watchdog Commands

| Command | Purpose |
|---------|---------|
| `request bgp watchdog announce <name>` | Send all routes in pool to peers |
| `request bgp watchdog withdraw <name>` | Withdraw all routes in pool from peers |
| `request bgp watchdog give <name> owner <owner> selector <peer> [withdraw] announce <n> <update text> withdrawal <update text>` | Give the watchdog one route at runtime: sent now, or held until `announce <name>` with `withdraw`. `<n>` counts the words of the announce |
| `request bgp watchdog forget <owner>` | Withdraw the given routes `<owner>` holds announced, and forget all it gave |

A watchdog group holds the routes from the config (`watchdog { name <name>; }`
in a peer's update block, read by the `bgp-watchdog` plugin) and the routes a
process gives it with `request bgp watchdog give`. The ExaBGP bridge gives an
API route that names a watchdog this way, as ExaBGP 6.0 holds such a route
under its watchdog (`docs/architecture/exabgp-bridge.md`). A given route the
engine refuses to send answers `give`, `announce`, `withdraw` or `forget` with
the refusal, never `done`. `update text` reads `watchdog <name>` and `watchdog
set <name>` and refuses the command, naming both fixes, because it sends at once
and would ignore the word.
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- ParseSendableUpdateText, errUpdateTextWatchdog -->
<!-- source: internal/component/bgp/plugins/watchdog/given.go -- handleGive, givenAction, handleForget -->

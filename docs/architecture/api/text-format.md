# Text Format Specification

The text format is the high-performance IPC encoding for engine-to-plugin event delivery.
It is used by bgp-rr (route reflector) on the hot path. Other plugins use JSON via `shared/event.go`.

Source of truth: `internal/plugins/bgp/format/text.go` (formatter), `internal/plugins/bgp-rr/server.go` (parser).

## Current Format

### Message Headers

Two header shapes exist depending on message type:

| Shape | Layout | Used By |
|-------|--------|---------|
| State | `peer <address> asn <asn> state <state>` | State change events |
| Message | `peer <address> <direction> <type> <msgid> <body...>` | UPDATE, OPEN, NOTIFICATION, KEEPALIVE, REFRESH, BORR, EORR |

State events include the peer ASN early in the header. Message events do not include ASN in the header —
ASN appears later in type-specific body fields (e.g., OPEN) or not at all (e.g., KEEPALIVE).

Direction is `received` or `sent`. Message ID is a monotonically increasing integer per peer session.

### BNF Grammar

```
<message>       ::= <state-event> | <message-event>
<state-event>   ::= "peer" <address> "asn" <asn> "state" <state-value> LF
<message-event> ::= "peer" <address> <direction> <type> <msgid> <body> LF

<direction>     ::= "received" | "sent"
<type>          ::= "update" | "open" | "notification" | "keepalive" | "refresh" | "borr" | "eorr"
<state-value>   ::= "up" | "down" | "established" | "connected" | ...

<update-body>   ::= <announce-body> | <withdraw-body> | <empty>
<announce-body> ::= "announce" <attributes> <family-section>+
<withdraw-body> ::= "withdraw" <family-section>+
<empty>         ::= (nothing — End-of-RIB or attribute-only)

<family-section-announce> ::= <family> "next-hop" <address> "nlri" <nlri>+
<family-section-withdraw> ::= <family> "nlri" <nlri>+
<family>        ::= <afi> "/" <safi>

<attributes>    ::= <attribute>*
<attribute>     ::= <origin> | <as-path> | <next-hop> | <med> | <local-pref>
                   | <community> | <large-community> | <extended-community> | <unknown-attr>

<origin>        ::= "origin" ("igp" | "egp" | "incomplete")
<as-path>       ::= "as-path" <asn> (<space> <asn>)*
<next-hop>      ::= "next-hop" <address>
<med>           ::= "med" <integer>
<local-pref>    ::= "local-preference" <integer>
<community>     ::= "community" "[" <community-value> (<space> <community-value>)* "]"
<large-community>    ::= "large-community" "[" <lc-value> (<space> <lc-value>)* "]"
<extended-community> ::= "extended-community" "[" <hex> (<space> <hex>)* "]"
<unknown-attr>  ::= "attr-" <code> <space> <hex>

<open-body>     ::= "asn" <asn> "router-id" <address> "hold-time" <seconds> <capability>*
<capability>    ::= "cap" <code> <name> [<value>]

<notification-body> ::= "code" <integer> "subcode" <integer> "code-name" <name> "subcode-name" <name> "data" <hex>

<keepalive-body> ::= (nothing after msgid)

<refresh-body>  ::= "family" <family>
```

### Attribute Formats

All verified against `format/text.go:699-784`.

| Attribute | Keyword | Format | Delimiter | Source |
|-----------|---------|--------|-----------|--------|
| ORIGIN | `origin` | `origin igp` | scalar | `text.go:704-708` |
| AS_PATH | `as-path` | `as-path 65001 65002` | space-separated, no brackets | `text.go:713-719` |
| NEXT_HOP | `next-hop` | `next-hop 192.0.2.1` | scalar | `text.go:723-725` |
| MED | `med` | `med 100` | scalar | `text.go:730-732` |
| LOCAL_PREF | `local-preference` | `local-preference 200` | scalar | `text.go:738-740` |
| COMMUNITY | `community` | `community [65001:100 65002:200]` | brackets, space-separated | `text.go:745-752` |
| LARGE_COMMUNITY | `large-community` | `large-community [65001:1:2 65002:3:4]` | brackets, space-separated | `text.go:757-764` |
| EXT_COMMUNITY | `extended-community` | `extended-community [0002000a0b0c0d0e]` | brackets, space-separated hex | `text.go:769-776` |
| Unknown | `attr-<code>` | `attr-42 deadbeef` | scalar hex | `text.go:783` |

Note: keywords are singular (`community`, not `communities`). AS_PATH uses bare space separation, while community types use brackets.

### NLRI String Formats

Each NLRI type plugin implements `String()` which produces the text representation appended after the `nlri` keyword. All verified against source.

| Type | Format | Optional Fields | Source |
|------|--------|-----------------|--------|
| IPv4/IPv6 unicast | `10.0.0.0/24` | — | `nlri/inet.go:178` |
| + ADD-PATH | `10.0.0.0/24 path-id set 42` | path-id | `nlri/inet.go:177` |
| VPN | `rd set 65000:100 prefix set 10.0.0.0/24 label set 1000` | label, path-id | `bgp-nlri-vpn/types.go:266` |
| Labeled unicast | `prefix set 10.0.0.0/24 label set 1000` | label, path-id | `bgp-nlri-labeled/types.go:161` |
| EVPN Type1 | `ethernet-ad rd set X esi set Y etag set Z` | label | `bgp-nlri-evpn/types.go:308` |
| EVPN Type2 | `mac-ip rd set X mac set Y ip set Z` | ip, etag, label | `bgp-nlri-evpn/types.go:481` |
| EVPN Type3 | `multicast rd set X ip set Y` | etag | `bgp-nlri-evpn/types.go:595` |
| EVPN Type4 | `ethernet-segment rd set X esi set Y ip set Z` | — | `bgp-nlri-evpn/types.go:715` |
| EVPN Type5 | `ip-prefix rd set X prefix set Y` | esi, etag, gateway, label | `bgp-nlri-evpn/types.go:875` |
| EVPN unknown | `evpn-type<N>` | — | `bgp-nlri-evpn/types.go:919` |
| FlowSpec | `flow destination 10.0.0.0/24 port ==80` | varies by components | `bgp-nlri-flowspec/types.go:336` |
| VPLS | `rd set X ve-id set Y label set Z` | — | `bgp-nlri-vpls/types.go:173` |
| MVPN | `<route-type> rd set X` | rd (conditional) | `bgp-nlri-mvpn/types.go:192` |
| RTC | `origin-as set X rt set Y` or `default` | default case has no sub-keys | `bgp-nlri-rtc/types.go:184` |
| MUP | `<route-type> rd set X` | rd (conditional) | `bgp-nlri-mup/types.go:200` |

All complex NLRIs use the `set` keyword between field name and value. FlowSpec match operators (`==`, `>=`, `!=`, etc.) pass through as part of the value token.

### Address Family Names

Format: `<afi>/<safi>` — always slash-separated, lowercase.

| Family | String |
|--------|--------|
| IPv4 Unicast | `ipv4/unicast` |
| IPv6 Unicast | `ipv6/unicast` |
| IPv4 VPN | `ipv4/vpn` |
| IPv6 VPN | `ipv6/vpn` |
| IPv4 FlowSpec | `ipv4/flowspec` |
| IPv6 FlowSpec | `ipv6/flowspec` |
| L2VPN EVPN | `l2vpn/evpn` |
| L2VPN VPLS | `l2vpn/vpls` |
| IPv4 Labeled | `ipv4/nlri-mpls` |
| IPv6 Labeled | `ipv6/nlri-mpls` |
| IPv4 RTC | `ipv4/rtc` |

### Complete Current Format Examples

All examples verified against `format/text_test.go`.

```
peer 10.0.0.1 asn 65001 state established

peer 10.0.0.1 asn 65001 state down

peer 10.0.0.1 received update 1 announce origin igp as-path 65001 65002 local-preference 100 ipv4/unicast next-hop 10.0.0.1 nlri 192.168.1.0/24

peer 10.0.0.1 received update 2 withdraw ipv4/unicast nlri 172.16.0.0/16

peer 10.0.0.1 received update 3

peer 10.0.0.1 sent open 42 asn 65001 router-id 1.1.1.1 hold-time 90

peer 10.0.0.1 received open 5 asn 42 router-id 10.0.0.1 hold-time 180 cap 1 multiprotocol ipv4/unicast cap 65 asn4 65001 cap 2 route-refresh

peer 10.0.0.1 sent notification 42 code 6 subcode 2 code-name Cease subcode-name Administrative-Shutdown data 0a0b0c0d

peer 10.0.0.1 sent keepalive 42

peer 10.0.0.1 received refresh 5 family ipv4/unicast

peer 10.0.0.1 received borr 1 family ipv6/unicast
```

### Multi-Family UPDATE

A single UPDATE can carry multiple address families. Each family section follows the previous one:

```
peer 10.0.0.1 received update 1 announce origin igp as-path 65001 ipv4/unicast next-hop 10.0.0.1 nlri 10.0.0.0/24 ipv6/unicast next-hop 2001:db8::1 nlri 2001:db8:1::/48
```

Announce and withdraw can appear in the same UPDATE as separate lines (same message ID):

```
peer 10.0.0.1 received update 1 announce origin igp ipv4/unicast next-hop 10.0.0.1 nlri 10.0.0.0/24
peer 10.0.0.1 received update 1 withdraw ipv4/unicast nlri 172.16.0.0/16
```

---

## Proposed Format (not yet implemented)

This section describes a planned redesign. No code implements this yet.
The goal is to make space only ever a token separator, enabling a trivial split-by-whitespace tokenizer.

### Design Principle

Three token patterns plus dict mode:

| Pattern | Structure | Example |
|---------|-----------|---------|
| scalar | `key value` | `origin igp`, `med 100` |
| list | `key value,value,value` | `as-path 65001,65002` |
| action | `key action value[,value]` | `nlri add 10.0.0.0/24,10.0.1.0/24` |
| action+dict | `key action subkey1 val1 subkey2 val2 ...` | `nlri add rd 65000:100 prefix 10.0.0.0/24` |

**Comma tolerance:** The formatter always generates `value1,value2,value3` (no spaces after commas). The parser accepts `value1, value2, value3` by stripping whitespace after commas.

**Token invariants:** No value token may contain a comma. No capability value may contain a colon.

### Proposed Header: Uniform for All Messages

All messages start with `peer <ip> asn <asn>`. After `asn <n>`, the next token dispatches:
- `state` — state event
- `negotiated` — negotiated event (new addition)
- `received` / `sent` — direction, followed by message type

### Proposed Attribute Changes

| Current | Proposed | Pattern |
|---------|----------|---------|
| `as-path 65001 65002` | `as-path 65001,65002` | list (commas) |
| `community [65001:100 65002:200]` | `community 65001:100,65002:200` | list (no brackets) |
| `large-community [65001:1:2 65002:3:4]` | `large-community 65001:1:2,65002:3:4` | list (no brackets) |
| `extended-community [0002... 0003...]` | `extended-community 0002...,0003...` | list (no brackets) |
| `origin igp` | `origin igp` | unchanged |
| `med 100` | `med 100` | unchanged |
| `local-preference 200` | `local-preference 200` | unchanged |
| `next-hop 192.0.2.1` | `next-hop 192.0.2.1` | unchanged |

### Proposed NLRI Changes

`announce`/`withdraw` keywords deleted — replaced by `nlri add`/`nlri del`. No aliasing.
All `set` keywords in complex NLRIs dropped — sub-keys become `key value` directly.

| Current | Proposed |
|---------|----------|
| `nlri 10.0.0.0/24 10.0.1.0/24` (in announce) | `nlri add 10.0.0.0/24,10.0.1.0/24` |
| `nlri 172.16.0.0/16` (in withdraw) | `nlri del 172.16.0.0/16` |
| `10.0.0.0/24 path-id set 42` | `nlri path-id 42 add 10.0.0.0/24` |
| `rd set 65000:100 prefix set 10.0.0.0/24 label set 1000` | `nlri add rd 65000:100 prefix 10.0.0.0/24 label 1000` |

ADD-PATH uses `path-id` as a modifier before the action: `nlri path-id 42 add 10.0.0.0/24,10.0.1.0/24`.

### Proposed Capability Changes

Current capabilities repeat `cap` per entry: `cap 1 multiprotocol ipv4/unicast cap 65 asn4 65001`.

Proposed: single `cap` key with colon-encoded comma list: `cap 1:multiprotocol:ipv4/unicast,65:asn4:65001,2:route-refresh`. Invariant: capability values must not contain colons.

### Proposed UPDATE Structure

`family` marks the start of a per-family section. `family` is context-dependent: in UPDATE messages it opens a per-family section (with next-hop and NLRIs); in REFRESH/BORR messages it is a simple scalar.

### Proposed Dict Mode

For complex NLRIs, the parser reads sub-key-value pairs after an action token until it encounters a token not in the family's sub-key set (that token becomes the next top-level key).

The parser's sub-key table per family must be updated whenever an NLRI type adds a new field. `nlri` is the only key that may repeat within a single message line.

### Complete Proposed Format Examples

```
peer 192.0.2.1 asn 65001 state up

peer 192.0.2.1 asn 65001 received update 1 origin igp as-path 65001,65002 med 100 community 65001:100,65002:200 family ipv4/unicast next-hop 192.0.2.1 nlri add 10.0.0.0/24,10.0.1.0/24

peer 192.0.2.1 asn 65001 received update 2 family ipv4/unicast nlri del 172.16.0.0/16,10.0.0.0/8

peer 192.0.2.1 asn 65001 received update 3 origin igp family ipv4/vpn next-hop 192.0.2.1 nlri add rd 65000:100 prefix 10.0.0.0/24 label 1000 nlri add rd 65001:200 prefix 10.1.0.0/24 label 2000

peer 192.0.2.1 asn 65001 received update 4 origin igp family l2vpn/evpn next-hop 192.0.2.1 nlri add route-type mac-ip rd 65000:100 mac aa:bb:cc:dd:ee:ff ip 10.0.0.1

peer 192.0.2.1 asn 65001 received update 5 origin igp family ipv4/unicast next-hop 192.0.2.1 nlri path-id 42 add 10.0.0.0/24,10.0.1.0/24

peer 192.0.2.1 asn 65001 update 0

peer 192.0.2.1 asn 65001 received open 1 asn 65001 router-id 1.1.1.1 hold-time 90 cap 1:multiprotocol:ipv4/unicast,65:asn4:65001,2:route-refresh

peer 192.0.2.1 asn 65001 sent notification 3 code 6 subcode 2 code-name Cease subcode-name Administrative-Shutdown data 0a0b0c0d

peer 192.0.2.1 asn 65001 sent keepalive 42

peer 192.0.2.1 asn 65001 received refresh 5 family ipv4/unicast

peer 192.0.2.1 asn 65001 negotiated hold-time 90 asn4 true route-refresh normal families ipv4/unicast,ipv6/unicast add-path-send ipv4/unicast add-path-receive ipv4/unicast
```

Empty UPDATEs have no direction — the dispatch token is `update` directly after `asn <n>`. This is the only message type without a direction prefix.

# Text Format Specification

The text format is the high-performance IPC encoding for engine-to-plugin event delivery.
It is used by bgp-rs (route server) on the hot path. Other plugins use JSON via `shared/event.go`.

Source of truth: `internal/component/bgp/format/text.go` (formatter), `internal/component/bgp/plugins/rs/server.go` (parser).

## Current Format

### Message Headers

Two header shapes exist depending on message type:
<!-- source: internal/component/bgp/format/text.go -- formatPeerHeader, formatStateText -->

| Shape | Layout | Used By |
|-------|--------|---------|
| State | `peer <address> asn <asn> state <state>` | State change events |
| Message | `peer <address> <direction> <type> <msgid> <body...>` | UPDATE, OPEN, NOTIFICATION, KEEPALIVE, REFRESH, BORR, EORR |
<!-- source: internal/component/bgp/format/text.go -- formatStateChangeText, formatFilterResultText -->

State events include the peer ASN early in the header. Message events do not include ASN in the header —
ASN appears later in type-specific body fields (e.g., OPEN) or not at all (e.g., KEEPALIVE).

Direction is `received` or `sent`. Message ID is a monotonically increasing integer per peer session.
<!-- source: internal/component/bgp/reactor/reactor_api.go -- OnPeerEstablished, OnPeerClosed -->

### BNF Grammar

```
<message>       ::= <state-event> | <message-event>
<state-event>   ::= "peer" <address> "asn" <asn> "state" <state-value> LF
<message-event> ::= "peer" <address> <direction> <type> <msgid> <body> LF

<direction>     ::= "received" | "sent"
<type>          ::= "update" | "open" | "notification" | "keepalive" | "refresh" | "borr" | "eorr"
<state-value>   ::= "up" | "down"

<update-body>   ::= <announce-body> | <withdraw-body> | <empty>
<announce-body> ::= "announce" <attributes> <family-section>+
<withdraw-body> ::= "withdraw" <family-section>+
<empty>         ::= (nothing — End-of-RIB or attribute-only)

<family-section-announce> ::= <family> "next" <address> "nlri" <nlri>+
<family-section-withdraw> ::= <family> "nlri" <nlri>+
<family>        ::= <afi> "/" <safi>

<attributes>    ::= <attribute>*
<attribute>     ::= <origin> | <as-path> | <next-hop> | <med> | <local-pref>
                   | <community> | <large-community> | <extended-community> | <unknown-attr>

<origin>        ::= "origin" ("igp" | "egp" | "incomplete")
<as-path>       ::= "path" <asn> ("," <asn>)*
<next-hop>      ::= "next" <address>
<med>           ::= "med" <integer>
<local-pref>    ::= "pref" <integer>
<community>     ::= "s-com" <community-value> ("," <community-value>)*
<large-community>    ::= "l-com" <lc-value> ("," <lc-value>)*
<extended-community> ::= "x-com" <hex> ("," <hex>)*
<unknown-attr>  ::= "attr-" <code> <space> <hex>

<open-body>     ::= "asn" <asn> "router-id" <address> "hold-time" <seconds> <capability>*
<capability>    ::= "cap" <code> <name> [<value>]

<notification-body> ::= "code" <integer> "subcode" <integer> "code-name" <name> "subcode-name" <name> "data" <hex>

<keepalive-body> ::= (nothing after msgid)

<refresh-body>  ::= "family" <family>
```
<!-- source: internal/component/bgp/format/text.go -- FormatOpen, FormatNotification, FormatKeepalive, FormatRouteRefresh -->
<!-- source: internal/component/bgp/textparse/keywords.go -- KWOrigin, ShortPath, ShortNext, ShortPref, ShortSCom, ShortLCom, ShortXCom -->

### Attribute Formats

All verified against `format/text.go:formatAttributeText()`. Text output uses short aliases.

| Attribute | Keyword (output) | Long form | Format | Delimiter |
|-----------|-------------------|-----------|--------|-----------|
| ORIGIN | `origin` | — | `origin igp` | scalar |
| AS_PATH | `path` | `as-path` | `path 65001,65002` | comma-separated |
| NEXT_HOP | `next` | `next-hop` | `next 192.0.2.1` | scalar |
| MED | `med` | — | `med 100` | scalar |
| LOCAL_PREF | `pref` | `local-preference` | `pref 200` | scalar |
| COMMUNITY | `s-com` | `community` | `s-com 65001:100,65002:200` | comma-separated |
| LARGE_COMMUNITY | `l-com` | `large-community` | `l-com 65001:1:2,65002:3:4` | comma-separated |
| EXT_COMMUNITY | `x-com` | `extended-community` | `x-com 0002000a0b0c0d0e` | comma-separated hex |
| Unknown | `attr-<code>` | — | `attr-42 deadbeef` | scalar hex |
<!-- source: internal/component/bgp/format/text.go -- formatAttributeText -->

Note: keywords are singular (`s-com`, not `communities`). Lists use comma separation (no brackets, no spaces in values).
Shared keyword constants defined in `textparse/keywords.go`. The alias `e-com` is accepted as input but the formatter always outputs `x-com`.
<!-- source: internal/component/bgp/textparse/keywords.go -- ShortXCom, aliasToCanonical -->

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
<!-- source: internal/component/bgp/nlri/inet.go -- INET.String -->
<!-- source: internal/component/bgp/plugins/nlri/vpn/types.go -- String -->
<!-- source: internal/component/bgp/plugins/nlri/evpn/types.go -- String -->
<!-- source: internal/component/bgp/plugins/nlri/flowspec/types.go -- String -->
<!-- source: internal/component/bgp/plugins/nlri/labeled/types.go -- String -->
<!-- source: internal/component/bgp/plugins/nlri/vpls/types.go -- String -->
<!-- source: internal/component/bgp/plugins/nlri/mvpn/types.go -- String -->
<!-- source: internal/component/bgp/plugins/nlri/rtc/types.go -- String -->
<!-- source: internal/component/bgp/plugins/nlri/mup/types.go -- String -->

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
<!-- source: internal/component/bgp/message/family.go -- FamilyIPv4Unicast, family constants -->

### Complete Current Format Examples

All examples verified against `format/text_test.go`.

```
peer 10.0.0.1 asn 65001 state up

peer 10.0.0.1 asn 65001 state down

peer 10.0.0.1 received update 1 announce origin igp path 65001,65002 pref 100 ipv4/unicast next 10.0.0.1 nlri 192.168.1.0/24

peer 10.0.0.1 received update 2 withdraw ipv4/unicast nlri 172.16.0.0/16

peer 10.0.0.1 received update 3

peer 10.0.0.1 sent open 42 asn 65001 router-id 1.1.1.1 hold-time 90

peer 10.0.0.1 received open 5 asn 42 router-id 10.0.0.1 hold-time 180 cap 1 multiprotocol ipv4/unicast cap 65 asn4 65001 cap 2 route-refresh

peer 10.0.0.1 sent notification 42 code 6 subcode 2 code-name Cease subcode-name Administrative-Shutdown data 0a0b0c0d

peer 10.0.0.1 sent keepalive 42

peer 10.0.0.1 received refresh 5 family ipv4/unicast

peer 10.0.0.1 received borr 1 family ipv6/unicast
```
<!-- source: internal/component/bgp/format/text.go -- formatStateChangeText, FormatOpen, FormatNotification, FormatKeepalive, FormatRouteRefresh -->

### Multi-Family UPDATE

A single UPDATE can carry multiple address families. Each family section follows the previous one:

```
peer 10.0.0.1 received update 1 announce origin igp path 65001 ipv4/unicast next 10.0.0.1 nlri 10.0.0.0/24 ipv6/unicast next 2001:db8::1 nlri 2001:db8:1::/48
```

Announce and withdraw can appear in the same UPDATE as separate lines (same message ID):

```
peer 10.0.0.1 received update 1 announce origin igp ipv4/unicast next 10.0.0.1 nlri 10.0.0.0/24
peer 10.0.0.1 received update 1 withdraw ipv4/unicast nlri 172.16.0.0/16
```
<!-- source: internal/component/bgp/format/text.go -- formatFilterResultText -->

---

## Remaining Proposed Changes (not yet implemented)

The following changes were planned in the unified text protocol design but are not yet implemented.
For implemented changes, see the current format sections above.

### Already Implemented (by spec-utp-1, spec-utp-2, spec-utp-3)

| Change | Status |
|--------|--------|
| Short keyword aliases (`path`, `next`, `pref`, `s-com`, `l-com`, `x-com`) | Implemented -- event formatter and command parser |
| Comma-separated lists (AS_PATH, communities) | Implemented — event formatter |
| No brackets around community lists | Implemented — event formatter |
| Flat grammar for commands (no `set` keyword for attributes) | Implemented — command parser |
| Alias resolution (short/long/legacy forms accepted) | Implemented — `textparse/keywords.go` |
| Shared keyword tables across formatter, command parser, event parser | Implemented — `textparse/keywords.go` |
| Text-mode 5-stage handshake (auto-detected from first byte) | Implemented — `rpc/text.go`, `rpc/text_conn.go` |
| TextMuxConn with `#N` serial prefix for post-startup concurrent RPCs | Implemented — `rpc/text_mux.go` |
| Heredoc config delivery (`root <name> json << END`) | Implemented — `rpc/text.go` FormatConfigureText/ParseConfigureText |
<!-- source: internal/component/bgp/textparse/keywords.go -- ShortPath, ShortNext, ShortPref, ShortSCom, ShortLCom, ShortXCom, aliasToCanonical -->

### Still Proposed: Uniform Header

All messages start with `peer <ip> asn <asn>`. After `asn <n>`, the next token dispatches:
- `state` — state event
- `negotiated` — negotiated event (new addition)
- `received` / `sent` — direction, followed by message type

Currently, state events include `asn` but message events do not.

### Still Proposed: Event NLRI Restructuring

`announce`/`withdraw` keywords replaced by `nlri add`/`nlri del` in events (already implemented for commands).
All `set` keywords in complex NLRIs dropped — sub-keys become `key value` directly.

| Current Event | Proposed Event |
|---------------|----------------|
| `announce origin igp ipv4/unicast next 10.0.0.1 nlri 10.0.0.0/24` | `origin igp next 10.0.0.1 nlri ipv4/unicast add prefix 10.0.0.0/24` |
| `withdraw ipv4/unicast nlri 172.16.0.0/16` | `nlri ipv4/unicast del prefix 172.16.0.0/16` |
| `path-id set 42` (in NLRI String) | `info 42` (flat, no `set`) |
| `rd set 65000:100 prefix set 10.0.0.0/24 label set 1000` (in NLRI String) | `rd 65000:100 prefix 10.0.0.0/24 label 1000` |

ADD-PATH uses `info` (short for `path-information`) as a modifier before the action: `nlri ipv4/unicast info 42 add prefix 10.0.0.0/24`.
<!-- source: internal/component/bgp/textparse/keywords.go -- KWAdd, KWDel, ShortInfo, KWPathInformation -->

### Still Proposed: Dict Mode

For complex NLRIs, the parser reads sub-key-value pairs after an action token until it encounters a token not in the family's sub-key set (that token becomes the next top-level key).

The parser's sub-key table per family must be updated whenever an NLRI type adds a new field. `nlri` is the only key that may repeat within a single message line.
<!-- source: internal/component/bgp/textparse/keywords.go -- NLRITypeKeywords -->

### Design Principles (for remaining work)

Three token patterns plus dict mode:

| Pattern | Structure | Example |
|---------|-----------|---------|
| scalar | `key value` | `origin igp`, `med 100` |
| list | `key value,value,value` | `path 65001,65002` |
| action | `key family action type value[,value]` | `nlri ipv4/unicast add prefix 10.0.0.0/24,10.0.1.0/24` |
| action+dict | `key family action subkey1 val1 subkey2 val2 ...` | `nlri ipv4/vpn add rd 65000:100 prefix 10.0.0.0/24` |

**Comma tolerance:** The formatter always generates `value1,value2,value3` (no spaces after commas). The parser accepts `value1, value2, value3` by stripping whitespace after commas.

**Token invariants:** No value token may contain a comma (comma is the list separator).

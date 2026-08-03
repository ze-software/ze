# Text Format Specification

The text format is the high-performance IPC encoding for engine-to-plugin event delivery.
It is used by bgp-rs on the route-server hot path. Other plugins can use JSON or structured in-process callbacks.

Source of truth: text formatters in `internal/component/bgp/format/`, parser functions in `internal/component/bgp/plugins/rs/server_text.go`, and shared token definitions in `internal/component/bgp/textparse/`.
<!-- source: internal/component/bgp/format/text.go -- AppendOpen, AppendNotification, AppendKeepalive, AppendRouteRefresh -->
<!-- source: internal/component/bgp/format/text_human.go -- appendFilterResultText, appendAttributesText -->
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- quickParseTextEvent, parseTextNLRIOps -->
<!-- source: internal/component/bgp/textparse/keywords.go -- keyword constants and aliases -->

## Current Format

### Message Headers

All parsed text events start with `peer <address> remote as <asn>`. State events dispatch directly on `state`; BGP message events include direction, type, and message ID after the ASN.
<!-- source: internal/component/bgp/format/text.go -- AppendOpen, AppendNotification, AppendKeepalive, AppendRouteRefresh -->
<!-- source: internal/component/bgp/format/text_human.go -- appendFilterResultText -->
<!-- source: internal/component/bgp/format/text_human.go -- appendStateChangeText -->

| Shape | Layout | Used by |
|-------|--------|---------|
| State | `peer <address> remote as <asn> state <state> [reason <reason>]` | State change events |
| Message | `peer <address> remote as <asn> <direction> <type> <msgid> <body...>` | UPDATE, OPEN, NOTIFICATION, KEEPALIVE, REFRESH, BORR, EORR |
<!-- source: internal/component/bgp/format/text_human.go -- appendStateChangeText, appendFilterResultText -->
<!-- source: internal/component/bgp/format/text.go -- AppendOpen, AppendNotification, AppendKeepalive, AppendRouteRefresh -->

Direction is `received` or `sent`. Message ID is a monotonically increasing integer per peer session for BGP wire messages.
<!-- source: internal/component/bgp/reactor/reactor_api.go -- OnPeerEstablished, OnPeerClosed -->

### BNF Grammar

```
<message>       ::= <state-event> | <message-event>
<state-event>   ::= "peer" <address> "remote" "as" <asn> "state" <state-value> [<reason>] LF
<message-event> ::= "peer" <address> "remote" "as" <asn> <direction> <type> <msgid> <body> LF

<direction>     ::= "received" | "sent"
<type>          ::= "update" | "open" | "notification" | "keepalive" | "refresh" | "borr" | "eorr"
<state-value>   ::= "up" | "down"
<reason>        ::= "reason" <token>

<update-body>   ::= <attribute>* <nlri-section>* | <empty>
<nlri-section>  ::= ["next" <address>] "nlri" <family> [<path-info>] <action> <nlri-token>+
<path-info>     ::= "info" <path-id>
<action>        ::= "add" | "del"
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

<open-body>     ::= "router-id" <address> "hold-time" <seconds> <capability>*
<capability>    ::= "cap" <code> <name> [<value>]

<notification-body> ::= "code" <integer> "subcode" <integer> "code-name" <name> "subcode-name" <name> "data" <hex>

<keepalive-body> ::= (nothing after msgid)

<refresh-body>  ::= "family" <family>
```
<!-- source: internal/component/bgp/format/text.go -- AppendOpen, AppendNotification, AppendKeepalive, AppendRouteRefresh -->
<!-- source: internal/component/bgp/format/text_human.go -- appendFilterResultText, appendAttributesText -->
<!-- source: internal/component/bgp/textparse/keywords.go -- KWOrigin, ShortPath, ShortNext, ShortPref, ShortSCom, ShortLCom, ShortXCom, ShortInfo -->

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
| IPv4/IPv6 unicast | `10.0.0.0/24` | — | `nlri/inet.go` |
| + ADD-PATH | `10.0.0.0/24 path-id set 42` | path-id | `nlri/inet.go` |
| VPN | `rd set 65000:100 prefix set 10.0.0.0/24 label set 1000` | label, path-id | `bgp-nlri-vpn/types.go` |
| Labeled unicast | `prefix set 10.0.0.0/24 label set 1000` | label, path-id | `bgp-nlri-labeled/types.go` |
| EVPN Type1 | `ethernet-ad rd set X esi set Y etag set Z` | label | `bgp-nlri-evpn/types.go` |
| EVPN Type2 | `mac-ip rd set X mac set Y ip set Z` | ip, etag, label | `bgp-nlri-evpn/types.go` |
| EVPN Type3 | `multicast rd set X ip set Y` | etag | `bgp-nlri-evpn/types.go` |
| EVPN Type4 | `ethernet-segment rd set X esi set Y ip set Z` | — | `bgp-nlri-evpn/types.go` |
| EVPN Type5 | `ip-prefix rd set X prefix set Y` | esi, etag, gateway, label | `bgp-nlri-evpn/types.go` |
| EVPN unknown | `evpn-type<N>` | — | `bgp-nlri-evpn/types.go` |
| FlowSpec | `flow destination 10.0.0.0/24 port ==80` | varies by components | `bgp-nlri-flowspec/types.go` |
| VPLS | `rd set X ve-id set Y label set Z` | — | `bgp-nlri-vpls/types.go` |
| MVPN | `<route-type> rd set X` | rd (conditional) | `bgp-nlri-mvpn/types.go` |
| RTC | `origin-as set X rt set Y` or `default` | default case has no sub-keys | `bgp-nlri-rtc/types.go` |
| MUP | `<route-type> rd set X` | rd (conditional) | `bgp-nlri-mup/types.go` |
<!-- source: internal/core/bgp/nlri/inet.go -- INET.String -->
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
| IPv4 VPN | `ipv4/mpls-vpn` |
| IPv6 VPN | `ipv6/mpls-vpn` |
| IPv4 FlowSpec | `ipv4/flowspec` |
| IPv6 FlowSpec | `ipv6/flowspec` |
| L2VPN EVPN | `l2vpn/evpn` |
| L2VPN VPLS | `l2vpn/vpls` |
| IPv4 Labeled | `ipv4/nlri-mpls` |
| IPv6 Labeled | `ipv6/nlri-mpls` |
| IPv4 RTC | `ipv4/rtc` |
<!-- source: internal/component/bgp/message/family.go -- FamilyIPv4Unicast, family constants -->

### Complete Current Format Examples

All examples below use the current uniform header and UPDATE body shape.

```
peer 10.0.0.1 remote as 65001 state up

peer 10.0.0.1 remote as 65001 state down

peer 10.0.0.1 remote as 65001 state down reason tcp-failure

peer 10.0.0.1 remote as 65001 received update 1 origin igp path 65001,65002 pref 100 next 10.0.0.1 nlri ipv4/unicast add 192.168.1.0/24

peer 10.0.0.1 remote as 65001 received update 2 nlri ipv4/unicast del 172.16.0.0/16

peer 10.0.0.1 remote as 65001 received update 3

peer 10.0.0.1 remote as 65001 sent open 42 router-id 1.1.1.1 hold-time 90

peer 10.0.0.1 remote as 65001 received open 5 router-id 10.0.0.1 hold-time 180 cap 1 multiprotocol ipv4/unicast cap 65 asn4 65001 cap 2 route-refresh

peer 10.0.0.1 remote as 65001 sent notification 42 code 6 subcode 2 code-name Cease subcode-name Administrative-Shutdown data 0a0b0c0d

peer 10.0.0.1 remote as 65001 sent keepalive 42

peer 10.0.0.1 remote as 65001 received refresh 5 family ipv4/unicast

peer 10.0.0.1 remote as 65001 received borr 1 family ipv6/unicast
```
<!-- source: internal/component/bgp/format/text_human.go -- appendStateChangeText, appendFilterResultText -->
<!-- source: internal/component/bgp/format/text.go -- AppendOpen, AppendNotification, AppendKeepalive, AppendRouteRefresh -->

### Multi-Family UPDATE

A single UPDATE can carry multiple address families. Each NLRI section follows the previous one:

```
peer 10.0.0.1 remote as 65001 received update 1 origin igp path 65001 next 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24 next 2001:db8::1 nlri ipv6/unicast add 2001:db8:1::/48
```

Announce and withdraw operations can appear in the same UPDATE line when both are present in the wire UPDATE:

```
peer 10.0.0.1 remote as 65001 received update 1 origin igp next 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24 nlri ipv4/unicast del 172.16.0.0/16
```
<!-- source: internal/component/bgp/format/text_human.go -- appendFilterResultText -->

---

## Current and Remaining Text Grammar Work

The unified text protocol work already landed the pieces used by the current formatter and parser:

| Change | Status |
|--------|--------|
| Uniform event header with `remote as` on parsed text messages | Implemented |
| Short keyword aliases (`path`, `next`, `pref`, `s-com`, `l-com`, `x-com`) | Implemented in formatter, command parser, and event parser |
| Alias resolution for long, short, and selected legacy forms | Implemented in `textparse/keywords.go` |
| Shared keyword tables across formatter, command parser, and event parser | Implemented in `textparse/keywords.go` |
| Event UPDATE operations as `nlri <family> add|del` | Implemented in formatter and route-server parser |
<!-- source: internal/component/bgp/format/text_human.go -- appendFilterResultText -->
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- parseTextNLRIOps -->
<!-- source: internal/component/bgp/textparse/keywords.go -- ShortPath, ShortNext, ShortPref, ShortSCom, ShortLCom, ShortXCom, aliasToCanonical -->

### Still Proposed: Complex NLRI Dict Mode

Complex NLRIs are still emitted from each NLRI type's `String()` method. Those strings can contain type-specific sub-fields and `set` tokens. The route-server event parser keeps them as opaque NLRI strings for forwarding.
<!-- source: internal/component/bgp/format/text_human.go -- appendNLRIList -->
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- buildNLRIEntries, parseTextNLRIOps -->

Dict mode remains proposed for any future parser that needs to understand complex NLRI sub-fields. Such a parser would need a family-specific sub-key table and would read sub-key-value pairs after `nlri <family> add|del` until the next top-level keyword.
<!-- source: internal/component/bgp/textparse/keywords.go -- NLRITypeKeywords, IsTopLevelKeyword -->

### Token Invariants

The event parser is whitespace-based. Comma-separated lists are one token, for example `path 65001,65002` or `add 10.0.0.0/24,10.0.1.0/24`. Do not insert spaces after commas in text-format event output.
<!-- source: internal/component/bgp/textparse/scanner.go -- Scanner, Next -->
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- buildNLRIEntries -->
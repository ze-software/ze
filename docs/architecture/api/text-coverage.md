# Text Format Coverage

Current implementation coverage of the text format across message types, attributes, and NLRI families.

Source of truth: `internal/component/bgp/format/text.go` (formatter), `internal/component/bgp/plugins/rs/server_text.go` (parser).
<!-- source: internal/component/bgp/format/text.go -- text formatters -->
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- text parsers -->

## Message Type Coverage

| Message Type | Formatter | Parser | Tests |
|-------------|-----------|--------|-------|
| State (up/down) | `FormatStateChange` `text.go:841` | `parseTextState` `server_text.go:352` | `TestFormatStateChange` `text_test.go:27` |
| UPDATE announce | `formatFilterResultText` `text.go:627` | `parseTextNLRIOps` `server_text.go:194` | `TestFormatMessageText` `text_test.go:77` |
| UPDATE withdraw | `formatFilterResultText` `text.go:627` | `parseTextNLRIOps` `server_text.go:194` | `TestFormatMessageText` `text_test.go:77` |
| UPDATE empty | `formatEmptyUpdate` `text.go:80` | handled (no body) | — |
| OPEN | `FormatOpen` `text.go:790` | `parseTextOpen` `server_text.go:280` | `TestFormatOpenWithDirection` `text_test.go:601` |
| NOTIFICATION | `FormatNotification` `text.go:809` | — (not parsed by RR) | `TestFormatNotificationWithDirection` `text_test.go:681` |
| KEEPALIVE | `FormatKeepalive` `text.go:826` | — (not parsed by RR) | `TestFormatKeepaliveWithDirection` `text_test.go:644` |
| REFRESH | `FormatRouteRefresh` `text.go:833` | `parseTextRefresh` `server_text.go:392` | — |
| BORR | `FormatRouteRefresh` `text.go:833` | `parseTextRefresh` `server_text.go:392` | — |
| EORR | `FormatRouteRefresh` `text.go:833` | parsed but ignored | — |
| Negotiated | --- (not yet formatted as text) | --- | --- |
<!-- source: internal/component/bgp/format/text.go -- FormatStateChange, FormatOpen, FormatNotification, FormatKeepalive, FormatRouteRefresh -->

## Attribute Coverage

| Attribute | Code | Text Formatter | JSON Formatter | Test |
|-----------|------|---------------|----------------|------|
| ORIGIN | 1 | `text.go:704` | `text.go:536` | `text_test.go:126` |
| AS_PATH | 2 | `text.go:713` | `text.go:547` | `text_test.go:126` |
| NEXT_HOP | 3 | `text.go:723` | — (via NLRI) | `text_test.go:135` |
| MED | 4 | `text.go:730` | `text.go:564` | — |
| LOCAL_PREF | 5 | `text.go:738` | `text.go:572` | `text_test.go:126` |
| COMMUNITY | 8 | `text.go:745` | `text.go:579` | — |
| EXT_COMMUNITY | 16 | `text.go:769` | `text.go:606` | — |
| LARGE_COMMUNITY | 32 | `text.go:757` | `text.go:592` | — |
| Unknown attrs | * | `text.go:783` | `text.go:621` | — |
| ATOMIC_AGGREGATE | 6 | — | — | — |
| AGGREGATOR | 7 | — | — | — |
| ORIGINATOR_ID | 9 | — | — | — |
| CLUSTER_LIST | 10 | --- | --- | --- |
<!-- source: internal/component/bgp/format/text.go -- attribute formatters (lines 704-783) -->

## NLRI Family Coverage

| Family | Plugin | String() | Formatter Integration | Parser Integration |
|--------|--------|----------|----------------------|-------------------|
| ipv4/unicast | built-in | `nlri/inet.go:178` | `text.go:650` | `server_text.go` |
| ipv6/unicast | built-in | `nlri/inet.go:178` | `text.go:650` | `server_text.go` |
| ipv4/mpls-vpn | bgp-nlri-vpn | `types.go:266` | via NLRI.String() | prefix collected |
| ipv6/mpls-vpn | bgp-nlri-vpn | `types.go:266` | via NLRI.String() | prefix collected |
| l2vpn/evpn | bgp-nlri-evpn | `types.go:308,481,595,715,875` | via NLRI.String() | prefix collected |
| ipv4/flowspec | bgp-nlri-flowspec | `types.go:336` | via NLRI.String() | prefix collected |
| ipv6/flowspec | bgp-nlri-flowspec | `types.go:336` | via NLRI.String() | prefix collected |
| ipv4/nlri-mpls | bgp-nlri-labeled | `types.go:161` | via NLRI.String() | prefix collected |
| ipv6/nlri-mpls | bgp-nlri-labeled | `types.go:161` | via NLRI.String() | prefix collected |
| l2vpn/vpls | bgp-nlri-vpls | `types.go:173` | via NLRI.String() | prefix collected |
| ipv4/rtc | bgp-nlri-rtc | `types.go:184` | via NLRI.String() | prefix collected |
| mvpn families | bgp-nlri-mvpn | `types.go:192` | via NLRI.String() | prefix collected |
| mup families | bgp-nlri-mup | `types.go:200` | via NLRI.String() | prefix collected |

## Parser Limitations

The current bgp-rs text parser collects NLRI strings as opaque tokens — it does not parse sub-fields
(rd, prefix, label, etc.) from complex NLRI types. It only needs the string representation for
forwarding to downstream peers.

| Capability | Status |
|-----------|--------|
| Simple prefix parsing (ipv4/ipv6 unicast) | Extracted as prefix strings |
| Complex NLRI sub-field parsing (VPN, EVPN, etc.) | Not parsed — forwarded as opaque strings |
| ADD-PATH path-id extraction | Not extracted — included in NLRI string |
| Attribute parsing from UPDATE text | Not parsed --- attributes forwarded with raw text |
| Capability parsing from OPEN | Parsed (code, name, value) |
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- parseTextNLRIOps, parseTextOpen -->

## Encoding Coverage

| Encoding | Formatter Support | Parser Support |
|----------|------------------|----------------|
| Text parsed | `FormatParsed` → text functions | Full (bgp-rs) |
| Text raw | `formatRawFromResult` | — |
| Text full | `formatFullFromResult` (parsed + raw) | — |
| JSON parsed | `FormatParsed` → JSON functions | Full (`shared/event.go`) |
| JSON raw | `formatRawFromResult` | — |
| JSON full | `formatFullFromResult` (parsed + raw) | --- |
<!-- source: internal/component/bgp/format/text.go -- FormatParsed, formatRawFromResult, formatFullFromResult -->

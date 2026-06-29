# Text Format Coverage

Current implementation coverage of the text format across message types, attributes, and NLRI families.

Source of truth: text formatters in `internal/component/bgp/format/`, route-server parser functions in `internal/component/bgp/plugins/rs/server_text.go`, and text parser shared tokens in `internal/component/bgp/textparse/`.
<!-- source: internal/component/bgp/format/text.go -- AppendOpen, AppendNotification, AppendKeepalive, AppendRouteRefresh -->
<!-- source: internal/component/bgp/format/text_human.go -- appendStateChangeText, appendFilterResultText, appendAttributeText -->
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- parseTextNLRIOps, parseTextOpen, parseTextState, parseTextRefresh -->
<!-- source: internal/component/bgp/textparse/keywords.go -- keyword constants and aliases -->

## Message Type Coverage

| Message Type | Formatter | Parser | Tests |
|-------------|-----------|--------|-------|
| State (up/down) | `appendStateChangeText` | `parseTextState` | `TestFormatStateChange` |
| UPDATE announce | `appendFilterResultText` | `parseTextNLRIOps` | `TestFormatMessageText` |
| UPDATE withdraw | `appendFilterResultText` | `parseTextNLRIOps` | `TestFormatMessageText` |
| UPDATE empty | `appendFilterResultText` | handled as header-only UPDATE | `TestFormatMessageText` |
| OPEN | `AppendOpen` | `parseTextOpen` | `TestFormatOpenWithDirection` |
| NOTIFICATION | `AppendNotification` | not parsed by route server | `TestFormatNotificationWithDirection` |
| KEEPALIVE | `AppendKeepalive` | not parsed by route server | `TestFormatKeepaliveWithDirection` |
| REFRESH | `AppendRouteRefresh` | `parseTextRefresh` | not covered here |
| BORR | `AppendRouteRefresh` | `parseTextRefresh` | not covered here |
| EORR | `AppendRouteRefresh` | parsed, then ignored by route-server dispatch | not covered here |
| Negotiated | not formatted as parsed text | not parsed by route server | not covered here |
<!-- source: internal/component/bgp/format/text.go -- AppendOpen, AppendNotification, AppendKeepalive, AppendRouteRefresh -->
<!-- source: internal/component/bgp/format/text_human.go -- appendStateChangeText, appendFilterResultText -->
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- parseTextNLRIOps, parseTextOpen, parseTextState, parseTextRefresh -->
<!-- source: internal/component/bgp/format/text_test.go -- TestFormatStateChange, TestFormatMessageText, TestFormatOpenWithDirection, TestFormatKeepaliveWithDirection, TestFormatNotificationWithDirection -->

## Attribute Coverage

| Attribute | Code | Text Formatter | JSON Formatter | Test |
|-----------|------|----------------|----------------|------|
| ORIGIN | 1 | `appendAttributeText` | `appendAttributeJSON` | `TestFormatTextUpdate_ShortAliases` |
| AS_PATH | 2 | `appendAttributeText` | `appendAttributeJSON` | `TestFormatTextUpdate_ShortAliases` |
| NEXT_HOP | 3 | `appendAttributeText` or per-family next-hop | via NLRI operation | `TestFormatTextUpdate_ShortAliases`, `TestFilterResultBothNextHops` |
| MED | 4 | `appendAttributeText` | `appendAttributeJSON` | `TestFormatTextUpdate_ShortAliases` |
| LOCAL_PREF | 5 | `appendAttributeText` | `appendAttributeJSON` | `TestFormatTextUpdate_ShortAliases` |
| COMMUNITY | 8 | `appendAttributeText` | `appendAttributeJSON` | `TestFormatTextUpdate_ShortAliases` |
| EXT_COMMUNITY | 16 | `appendAttributeText` | `appendAttributeJSON` | not covered here |
| LARGE_COMMUNITY | 32 | `appendAttributeText` | `appendAttributeJSON` | not covered here |
| Unknown attrs | * | `appendAttributeText` | `appendAttributeJSON` | not covered here |
| ATOMIC_AGGREGATE | 6 | recognized by parser, not emitted in text table above | `appendAttributeJSON` | not covered here |
| AGGREGATOR | 7 | recognized by parser, not emitted in text table above | `appendAttributeJSON` | not covered here |
| ORIGINATOR_ID | 9 | recognized by parser, not emitted in text table above | `appendAttributeJSON` | not covered here |
| CLUSTER_LIST | 10 | recognized by parser, not emitted in text table above | `appendAttributeJSON` | not covered here |
<!-- source: internal/component/bgp/format/text_human.go -- appendAttributeText -->
<!-- source: internal/component/bgp/format/text_json.go -- appendAttributesJSON, appendAttributeJSON -->
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- parseTextNLRIOps -->
<!-- source: internal/component/bgp/format/text_test.go -- TestFormatTextUpdate_ShortAliases, TestFilterResultBothNextHops -->

## NLRI Family Coverage

| Family | Plugin | String source | Formatter integration | Parser integration |
|--------|--------|---------------|-----------------------|-------------------|
| ipv4/unicast | built-in | `INET.String` / `INET.AppendString` | `appendNLRIList` | prefix string collected |
| ipv6/unicast | built-in | `INET.String` / `INET.AppendString` | `appendNLRIList` | prefix string collected |
| ipv4/mpls-vpn | bgp-nlri-vpn | `NLRI.String()` | via NLRI string | opaque string collected |
| ipv6/mpls-vpn | bgp-nlri-vpn | `NLRI.String()` | via NLRI string | opaque string collected |
| l2vpn/evpn | bgp-nlri-evpn | `NLRI.String()` | via NLRI string | opaque string collected |
| ipv4/flowspec | bgp-nlri-flowspec | `NLRI.String()` | via NLRI string | opaque string collected |
| ipv6/flowspec | bgp-nlri-flowspec | `NLRI.String()` | via NLRI string | opaque string collected |
| ipv4/nlri-mpls | bgp-nlri-labeled | `NLRI.String()` | via NLRI string | opaque string collected |
| ipv6/nlri-mpls | bgp-nlri-labeled | `NLRI.String()` | via NLRI string | opaque string collected |
| l2vpn/vpls | bgp-nlri-vpls | `NLRI.String()` | via NLRI string | opaque string collected |
| ipv4/rtc | bgp-nlri-rtc | `NLRI.String()` | via NLRI string | opaque string collected |
| mvpn families | bgp-nlri-mvpn | `NLRI.String()` | via NLRI string | opaque string collected |
| mup families | bgp-nlri-mup | `NLRI.String()` | via NLRI string | opaque string collected |
<!-- source: internal/core/bgp/nlri/inet.go -- INET.String, INET.AppendString -->
<!-- source: internal/component/bgp/format/text_human.go -- appendNLRIList -->
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- buildNLRIEntries, parseTextNLRIOps -->

## Parser Limitations

The current bgp-rs text parser collects NLRI values as opaque strings for forwarding. It does not parse the sub-fields inside complex NLRI strings, such as `rd`, `prefix`, or `label`.

| Capability | Status |
|-----------|--------|
| Simple IPv4/IPv6 prefix values | Extracted as prefix strings |
| Complex NLRI sub-field parsing (VPN, EVPN, and similar families) | Not parsed, forwarded as opaque strings |
| ADD-PATH path-id modifier in event syntax | Consumed by the parser; not returned in `FamilyOperation` |
| Attribute parsing from UPDATE text | Recognized as section boundaries or skipped values; forwarded with raw text |
| Capability parsing from OPEN | Parsed as code, name, and optional value |
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- parseTextNLRIOps, buildNLRIEntries, parseTextOpen -->

## Encoding Coverage

| Encoding | Formatter Support | Parser Support |
|----------|------------------|----------------|
| Text parsed | `plugin.FormatParsed` through text formatter functions | Route-server parser covers routed event types |
| Text raw | `appendRawFromResult` | not parsed by route server |
| Text full | `appendFullFromResult` writes parsed text plus raw text line | parsed text line covered by route-server parser |
| JSON parsed | `plugin.FormatParsed` through JSON formatter functions | handled by JSON consumers, not this text parser |
| JSON raw | `appendRawFromResult` | not parsed by route server |
| JSON full | `appendFullFromResult` | not parsed by route server |
<!-- source: internal/component/bgp/format/text_update.go -- appendParsedFromResult, appendRawFromResult, appendFullFromResult -->
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- parseTextNLRIOps, parseTextOpen, parseTextState, parseTextRefresh -->

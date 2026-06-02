# Text Parser Architecture

The route-server text parser lives in `internal/component/bgp/plugins/rs/server_text.go`.
It consumes text-format BGP events for route-server forwarding. Tokenization is shared through `internal/component/bgp/textparse`.
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- quickParseTextEvent, parseTextNLRIOps, parseTextOpen, parseTextState, parseTextRefresh -->
<!-- source: internal/component/bgp/textparse/scanner.go -- Scanner, NewScanner, Next, Peek, Done -->

## Current Parser

### Scanner

`textparse.Scanner` holds the original input string and a byte offset. `Next()` skips ASCII whitespace, returns the next token as a substring of the original input, and advances the offset. `Peek()` saves and restores the offset. `Done()` checks whether any non-whitespace byte remains.
<!-- source: internal/component/bgp/textparse/scanner.go -- Scanner, Next, Peek, Done -->

The scanner does not build a token slice. Parser allocations now come from result data structures, such as UPDATE operation maps, NLRI token collection, normalized NLRI strings, OPEN capability slices, and the small `strings.SplitN` result used when decoding refresh families.
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- parseTextNLRIOps, buildNLRIEntries, parseTextOpen, parseTextRefresh -->

### Two-Phase Parsing

The parser uses a two-phase design so UPDATE forwarding can route by peer and message ID before doing full body work:

| Phase | Function | Purpose | Allocation shape |
|-------|----------|---------|------------------|
| Quick parse | `quickParseTextEvent` | Extract event type, msgID, peer address, and raw payload | Scanner over the input; no token slice |
| Family scan | `parseTextUpdateFamilies` | Discover registered UPDATE families when only family routing is needed | Result map only |
| Full parse | `parseTextOpen`, `parseTextState`, `parseTextRefresh`, `parseTextNLRIOps` | Type-specific field extraction | Only the returned event, map, slices, or normalized strings |
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- quickParseTextEvent, parseTextUpdateFamilies, parseTextNLRIOps, parseTextOpen, parseTextState, parseTextRefresh -->

UPDATE messages are the hot path. Quick parse extracts just enough to route the text to the per-peer worker. Full NLRI parsing is deferred to the worker goroutine.
<!-- source: internal/component/bgp/plugins/rs/server.go -- dispatchText -->
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- quickParseTextEvent, parseTextNLRIOps -->

### Entry Point: dispatchText

`dispatchText(text string)` receives raw text lines from the engine socket:

1. Calls `quickParseTextEvent(text)` for lightweight routing.
2. Routes by event type:
   - `update`: stores raw text in forward context and dispatches to the per-peer worker.
   - `state`: full parse with `parseTextState()`, then inline handling.
   - `refresh`, `borr`, `eorr`: full parse with `parseTextRefresh()`, then inline handling.
   - `open`: full parse with `parseTextOpen()`, then inline handling.

UPDATE is the only type with deferred body parsing. The other parsed event types are infrequent and handled inline.
<!-- source: internal/component/bgp/plugins/rs/server.go -- dispatchText -->

### Uniform Header

Text events use the uniform header `peer <addr> remote as <asn> ...`. After the ASN value, the next token decides the shape:

| Shape | Tokens after ASN | Type | ID |
|-------|------------------|------|----|
| State | `state <state>` | `state` | none, returns 0 |
| Message | `<direction> <type> <id>` | token after direction | parsed unsigned integer, or 0 if absent or non-numeric |
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- quickParseTextEvent -->
<!-- source: internal/component/bgp/format/text.go -- AppendOpen, AppendNotification, AppendKeepalive, AppendRouteRefresh -->
<!-- source: internal/component/bgp/format/text_human.go -- appendFilterResultText -->

`quickParseTextEvent` trims a trailing newline, validates the `peer`, `remote`, and `as` keywords, consumes the ASN value, and returns the full original text as payload for deferred parsing.
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- quickParseTextEvent -->

### UPDATE Parser: parseTextNLRIOps

`parseTextNLRIOps(text string) map[string][]FamilyOperation` parses text UPDATE bodies into family operation lists. It skips the eight-token uniform UPDATE header: `peer`, address, `remote`, `as`, ASN, direction, `update`, and msgID.
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- parseTextNLRIOps, FamilyOperation -->

The parser then runs a key-dispatch loop. It resolves keyword aliases through `textparse.ResolveAlias`, consumes known attribute values, and handles `nlri` sections as `nlri <family> [info <path-id>] add|del <tokens...>`.
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- parseTextNLRIOps -->
<!-- source: internal/component/bgp/textparse/keywords.go -- ResolveAlias, KWNLRI, KWPathInformation, KWAdd, KWDel -->

NLRI tokens are collected until `textparse.IsTopLevelKeyword` sees the next attribute or `nlri` section. `buildNLRIEntries` then normalizes the collected tokens:

| Input shape | Result |
|-------------|--------|
| Comma list, for example `prefix 10.0.0.0/24,10.0.1.0/24` | one entry per comma item, with the type prefix preserved |
| Repeated NLRI type keyword, for example `prefix 10.0.0.0/24 prefix 10.0.1.0/24` | one entry per keyword-bounded group |
| Complex opaque NLRI tokens | one joined string |
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- buildNLRIEntries, parseTextNLRIOps -->
<!-- source: internal/component/bgp/textparse/keywords.go -- IsTopLevelKeyword, NLRITypeKeywords -->

### UPDATE Family Scan: parseTextUpdateFamilies

`parseTextUpdateFamilies(text string)` is a lighter scanner pass that looks only for `nlri <family>` pairs and returns registered families. Unknown family strings are dropped because the engine only forwards families known by the family registry.
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- parseTextUpdateFamilies -->
<!-- source: internal/core/family/family.go -- LookupFamily -->

### OPEN Parser: parseTextOpen

`parseTextOpen(text string) *Event` reads the uniform header, copies the peer address, parses the peer ASN from `remote as <n>`, skips direction, `open`, and msgID, then scans the OPEN body:

- `router-id <value>` sets `Open.RouterID`.
- `hold-time <value>` parses the hold timer as uint16.
- `cap <code> <name> [<value>]` appends a capability. The optional value is consumed only when the next token is not another OPEN keyword.
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- parseTextOpen, OpenInfo, CapabilityInfo -->

### State Parser: parseTextState

`parseTextState(text string) *Event` scans the uniform state line. `remote as <value>` sets `PeerASN`; `state <value>` sets the state string.
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- parseTextState -->

### Refresh Parser: parseTextRefresh

`parseTextRefresh(text string) *Event` reads the uniform message header, records the refresh subtype token (`refresh`, `borr`, or `eorr`), and scans for `family <afi>/<safi>`. The family string is split once on `/` and resolved through the family registry.
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- parseTextRefresh -->
<!-- source: internal/core/family/family.go -- LookupAFI, LookupSAFI -->

`borr` and `eorr` are parsed by the same function as `refresh`; route-server dispatch currently ignores refresh boundaries after parsing.
<!-- source: internal/component/bgp/plugins/rs/server.go -- dispatchText -->
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- parseTextRefresh -->

### Error Handling

Quick parse fails closed for malformed headers by returning specific errors for missing `peer`, address, `remote`, `as`, ASN value, dispatch token, or event type. A missing or non-numeric message ID returns event type, peer address, payload, and msgID 0 without treating the event as malformed.
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- errInvalidTextEventMissingPeerPrefix, errInvalidTextEventMissingEventType, quickParseTextEvent -->

Full parsers are tolerant of partial bodies. Missing mandatory peer address returns nil. Missing optional values, unknown tokens, and unparsable numeric body fields are skipped.
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- parseTextOpen, parseTextState, parseTextRefresh, parseTextNLRIOps -->

### String Operations Used

| Function | Primary operation |
|----------|-------------------|
| `quickParseTextEvent` | `strings.TrimRight` plus `textparse.Scanner` |
| `parseTextUpdateFamilies` | `textparse.Scanner`, `family.LookupFamily` |
| `parseTextNLRIOps` | `strings.TrimRight`, `textparse.Scanner`, alias resolution |
| `buildNLRIEntries` | comma detection, `strings.SplitSeq`, `strings.TrimSpace`, `strings.Join` for normalized NLRI strings |
| `parseTextOpen` | `strings.TrimRight`, `textparse.Scanner`, numeric parsing |
| `parseTextState` | `strings.TrimRight`, `textparse.Scanner`, numeric parsing |
| `parseTextRefresh` | `strings.TrimRight`, `textparse.Scanner`, `strings.SplitN` for family split |
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- quickParseTextEvent, parseTextUpdateFamilies, parseTextNLRIOps, buildNLRIEntries, parseTextOpen, parseTextState, parseTextRefresh -->
<!-- source: internal/component/bgp/textparse/scanner.go -- Scanner -->
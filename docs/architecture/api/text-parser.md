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

Text events use the uniform header `peer <addr> remote as <asn> ...`. The `<asn>` value is the peer's AS, or the word `unknown` while a dynamic peer's AS is not yet negotiated; the formatters write `unknown` rather than `0`, so no header carries a zero that reads as an AS. After the ASN value, the next token decides the shape:

| Shape | Tokens after ASN | Type | ID |
|-------|------------------|------|----|
| State | `state <state>` | `state` | none, returns 0 |
| Message | `<direction> <type> <id>` | token after direction | parsed unsigned integer, or 0 if absent or non-numeric |
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- quickParseTextEvent -->
<!-- source: internal/component/bgp/format/text.go -- AppendOpen, AppendNotification, AppendKeepalive, AppendRouteRefresh -->
<!-- source: internal/component/bgp/format/text_human.go -- appendFilterResultText -->

`quickParseTextEvent` trims a trailing newline, validates the `peer`, `remote`, and `as` keywords, consumes the ASN value, and returns the full original text as payload for deferred parsing.
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- quickParseTextEvent -->

UPDATE events carry `source-policy-epoch <uint64>` immediately after the message
ID and before attributes. RS and RR parse it before retaining routes;
RS carries the scalar through its queued work item. The shared
`textparse.SourcePolicyEpoch` parser rejects malformed values. An absent token
records zero (unrecorded provenance), not the source's current epoch. NLRI
parsing skips the reserved token and its value; it is not a route attribute.
Only a header that carries a message ID carries the pair. The raw text line
(`peer <ip> <direction> update raw <hex>`) and the empty UPDATE line
(`peer <ip> update`) keep their grammar, which has no message ID: a reader
records zero there, which grants no announcement authority.

The reserved UPDATE header also carries `peer-owner <uint64>`, the engine-owned
Peer incarnation. It is independent of the address and remains stable across a
reconnect; the source receipt changes for the new Session. State and refresh
lines carry the same peer owner. Zero is unrecorded, not a derived identity.
Only immediate reserved header pairs supply UPDATE provenance: later route
attributes cannot impersonate it. Duplicate, missing-value and malformed owner
pairs are refused just like malformed source-receipt pairs.

The same parsed header carries `source-receipt <uint64>` after the policy epoch
when the engine recorded an original source session. These four pairs are reserved
provenance; none can be supplied by a route attribute or `route-meta`.
`textparse.SourceProvenance` reads only the reserved pairs immediately after the
eight header tokens and rejects malformed or repeated pairs. It never searches
later route content for authority. Envelope readers bind all three values to the
engine's receipt before using it. A missing pair records zero.
<!-- source: internal/component/bgp/textparse/scanner.go -- SourcePolicyEpoch, SourceProvenance -->
<!-- source: internal/component/bgp/plugins/rr/rr.go -- dispatchText -->

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

`parseTextOpen(text string) *Event` reads the uniform header, copies the peer address, parses the peer ASN from `remote as <n>` (an `unknown` value leaves `PeerASN` unset), skips direction, `open`, and msgID, then scans the OPEN body:

- `asn <value>` is the OPEN's own My Autonomous System, the sender's AS. It sets `Open.ASN`, and also `PeerASN`, because bgp-rs subscribes to received OPENs only, so the sender is the peer. The value is read with `asn.Parse`, which accepts every RFC 5396 spelling. The header's `remote as` no longer fills `Open.ASN`.
- `router-id <value>` sets `Open.RouterID`.
- `hold-time <value>` parses the hold timer as uint16.
- `cap <code> <name> [<value>]` appends a capability. The optional value is consumed only when the next token is not another OPEN keyword.
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- parseTextOpen, OpenInfo, CapabilityInfo -->

### State Parser: parseTextState

`parseTextState(text string) *Event` scans the uniform state line. `remote as <value>` sets `PeerASN`; `state <value>` sets the state string; `reason <value>` sets `Reason`, which names why a session went down, or is `attached` on an up a reload sent to a process it attached to a session already up; `initial-replay <receipt>` sets `InitialReplay`; `session-generation <n>` sets `SessionGeneration`, the original forward generation published by UP and named again by DOWN. It is independent of the immutable physical connection identifier.
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

Full parsers are tolerant of partial bodies. Missing mandatory peer address returns nil. Missing optional values, unknown tokens, and unparsable numeric body fields are skipped, with one exception: a state line whose `initial-replay` or `session-generation` token has no value, or a value that is not a number, returns nil, because the receipt grants readiness and the generation bounds what a departure withdraws, and a wrong one must not be guessed.
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


### Physical source identity

The reserved UPDATE header carries the original `connection-id` beside the
source policy epoch, source receipt and peer owner. `SourcePhysicalProvenance`
reads those four independent values before the first route token. It refuses a
malformed or repeated reserved pair as a whole; attribute and metadata values
later in a route cannot supply any of them. `SourceProvenance` and
`SourcePolicyEpoch` use the same parser, so asking for fewer fields still checks
the complete reserved header. A missing physical identity stays unrecorded;
forwarding callers must refuse it rather than deriving it from an address or
from the replay receipt.
<!-- source: internal/component/bgp/textparse/scanner.go -- SourcePhysicalProvenance, SourceProvenance -->


### RS physical lifecycle fields

RS state, OPEN and refresh readers preserve the original `peer-owner` and
`connection-id` independently of `destination-session` and `initial-replay`.
Missing original identity refuses actor mutation. Peer removal with an owner
and no physical connection is an owner-terminal event; a transport DOWN never
uses that terminal path. UPDATE admission uses the complete reserved physical
header parser and carries its owner and connection into the actual worker key.
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- parseTextState, parseTextOpen, parseTextRefresh -->


### RS current native route keys

A text UPDATE preserves each ADD-PATH identifier, including zero, in the
family section's `info` pair. A native `raw` value is one individual NLRI with
its source path identifier removed. RS uses the registered family framer and
key function, then stores the independent identifier and command framing in
its existing withdrawal inventory. Plain prefix sections use the same current
key as the structured UPDATE path.
<!-- source: internal/component/bgp/plugins/rs/server_text.go -- parseTextNLRIOps -->
<!-- source: internal/component/bgp/plugins/rs/retained_membership.go -- textInventoryEntry, storedKey -->


### Original native routes in TEXT

A plain INET section keeps its compact comma-separated prefix form. Each
ADD-PATH route gets its own existing `nlri <family> info <id> add|del` section,
including identifier zero. A complex route uses `raw <hex>` in that section;
the bytes are its native PrefixOnly payload, with no ADD-PATH header. They
retain native framing, labels and route distinguisher bytes. A summary string
is never a retained route identity. TEXT owners must retain the section's path
identifier separately from its native bytes.
<!-- source: internal/component/bgp/format/text_human.go -- appendTextFamilyNLRIs, appendTextNativeNLRI -->

With no attribute filter, the actual received TEXT path requests all attributes
through `AttributeFilter.ApplyToUpdate` and `AttributesWire.All` before rendering
(only opt-in `format=base64` short-circuits `appendMessageTyped` ahead of
`ApplyToUpdate` to emit the positional counted `upd` wire line instead of parsed
TEXT).
The standard community member therefore carries NO_LLGR when present. A
selective attribute view does not establish its absence and must not authorize
a retention decision. OPEN uses the same original peer-owner and connection-id
header projection as other messages. These fields describe the captured event;
raw peer bytes are unchanged.
<!-- source: internal/component/bgp/format/text_update.go -- appendMessageTyped -->
<!-- source: internal/component/bgp/filter/filter.go -- AttributeFilter.Apply, ApplyToUpdate -->
<!-- source: internal/component/bgp/format/text.go -- AppendOpen -->

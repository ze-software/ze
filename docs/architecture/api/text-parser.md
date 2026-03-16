# Text Parser Architecture

The text parser lives in `internal/component/bgp/plugins/rs/server.go`. It is the only consumer of the text format
on the hot path (route server forwarding). Other plugins use JSON via `shared/event.go`.

Source of truth: `internal/component/bgp/plugins/rs/server.go` (all `parse*` functions).

## Current Parser

### Two-Phase Parsing

The parser uses a two-phase design to minimize work on the hot path:

| Phase | Function | Purpose | Allocations |
|-------|----------|---------|-------------|
| Quick parse | `quickParseTextEvent` | Extract event type, msgID, peer address | 1 (`strings.Fields` for first 5 tokens) |
| Full parse | `parseText{Open,State,Refresh,NLRIOps}` | Type-specific field extraction | Per-type (all use `strings.Fields`) |

UPDATE messages are the hot path. Quick parse extracts just enough to route the message to a per-peer worker.
Full NLRI parsing (`parseTextNLRIOps`) is deferred to the worker goroutine.

### Entry Point: dispatchText

`dispatchText(text string)` at `server.go:503` receives raw text lines from the engine socket.

1. Calls `quickParseTextEvent(text)` for lightweight routing
2. Routes by event type:
   - `update` — stores raw text in forward context, dispatches to per-peer worker (deferred parse)
   - `state` — full parse with `parseTextState()`, inline handling
   - `refresh` / `borr` / `eorr` — full parse with `parseTextRefresh()`, inline handling
   - `open` — full parse with `parseTextOpen()`, inline handling

UPDATE is the only type with deferred parsing. All others are infrequent and parsed immediately.

### Quick Parse: quickParseTextEvent

`quickParseTextEvent(text string) (eventType, msgID, peerAddr, payload, error)` at `server.go:1011`.

1. Trim trailing newline
2. Split with `strings.Fields(text)` — requires minimum 5 fields, must start with `peer`
3. Detect layout by positional check:
   - If `fields[4] == "state"` — STATE event (different header shape): return `("state", 0, fields[1], text, nil)`
   - Otherwise — MESSAGE event: type at `fields[3]`, msgID parsed from `fields[4]`
4. Returns `(eventType, msgID, peerAddr, fullText, err)`

The two header shapes require different field indices:

| Header Shape | Type Location | ID Location | Example |
|-------------|---------------|-------------|---------|
| State | `fields[4]` (keyword "state") | none (returns 0) | `peer 10.0.0.1 asn 65001 state up` |
| Message | `fields[3]` | `fields[4]` | `peer 10.0.0.1 received update 1 ...` |

### UPDATE Parser: parseTextNLRIOps

`parseTextNLRIOps(text string) map[string][]FamilyOperation` at `server.go:1057`.

Handles multiline UPDATEs (announce + withdraw for same msgID on separate lines).

1. Split text by newlines with `strings.SplitSeq`
2. For each line, call `parseTextNLRIOpsLine(fields, result)`

`parseTextNLRIOpsLine` at `server.go:1068` implements a state machine:

1. Scan for action token: `announce` (maps to add) or `withdraw` (maps to del)
2. After action, scan for family tokens and NLRI sequences:
   - Family token detected by `isFamilyToken()` — sets current family context
   - `next-hop` — consumes next token as address (skips it)
   - `nlri` — enters NLRI collection mode
   - Other tokens in NLRI mode — collected as prefix strings
3. Family boundary: encountering a new family token flushes accumulated NLRIs

Result is a map: family string to list of `FamilyOperation{Action, NLRIs}`.

### Family Detection: isFamilyToken

`isFamilyToken(s string) bool` at `server.go:1150`.

Distinguishes family strings from NLRI prefixes — both contain `/`:

| Input | Result | Why |
|-------|--------|-----|
| `ipv4/unicast` | true | suffix `unicast` has non-digit characters |
| `ipv6/vpn` | true | suffix `vpn` has non-digit characters |
| `10.0.0.0/24` | false | suffix `24` is all digits |
| `2001:db8::/32` | false | suffix `32` is all digits |

Algorithm: find last `/`, check if all characters after it are digits. All-digits = NLRI prefix length. Non-digits = SAFI name.

### OPEN Parser: parseTextOpen

`parseTextOpen(text string) *Event` at `server.go:1170`.

Parses key-value pairs starting from `fields[5]`:
- `asn <value>` — peer ASN
- `router-id <value>` — router ID string
- `hold-time <value>` — hold timer seconds
- `cap <code> <name> [<value>]` — capability (value is optional; consumed only if next token is not a keyword)

Capabilities are chained: `cap 1 multiprotocol ipv4/unicast cap 65 asn4 65001 cap 2 route-refresh`.

### State Parser: parseTextState

`parseTextState(text string) *Event` at `server.go:1226`.

Parses key-value pairs from `fields[2]`:
- `asn <value>` — peer ASN
- `state <value>` — state string (up, down, established, etc.)

### Refresh Parser: parseTextRefresh

`parseTextRefresh(text string) *Event` at `server.go:1257`.

Type comes from `fields[3]` (one of: `refresh`, `borr`, `eorr`).
Scans for `family` keyword, splits the value by `/` into AFI and SAFI.

Note: `borr` and `eorr` (RFC 7313 subtypes) are parsed but silently ignored by `dispatchText()` — route servers don't track refresh boundaries.

### Error Handling

All parsers are fail-safe:

| Condition | Behavior |
|-----------|----------|
| Too few fields | Return nil (or error for quickParse) |
| Non-numeric msgID | Silently returns 0 |
| Missing keyword values | Silently skipped |
| Unknown tokens | Ignored (not errors) |
| Unknown event types | Silently ignored by dispatchText |

### String Operations Used

| Function | Primary Operation |
|----------|-------------------|
| quickParseTextEvent | `strings.Fields()` + positional index |
| parseTextNLRIOps | `strings.SplitSeq()` + `strings.Fields()` per line |
| parseTextNLRIOpsLine | Token scan loop over `strings.Fields()` result |
| isFamilyToken | `strings.LastIndex()` + byte-level suffix scan |
| parseTextOpen | `strings.Fields()` + keyword loop |
| parseTextState | `strings.Fields()` + keyword loop |
| parseTextRefresh | `strings.SplitN()` for family split |

All functions allocate via `strings.Fields()`. No manual byte scanning or zero-allocation parsing exists in the current implementation.

---

## Proposed Parser Design (not yet implemented)

This section describes a planned redesign. No code implements this yet.

### Tokenizer

Split input by whitespace into byte-slice tokens pointing into the original buffer. Zero allocations if tokens remain as byte slices (no string conversion). This replaces `strings.Fields()` which allocates a string slice.

### Key-Dispatch Parser

After tokenizing, the parser reads tokens sequentially:

1. Read token, look up in key table
2. Key table returns: pattern (scalar / list / action) and known sub-keys (for dict)
3. For scalar: consume next token as value
4. For list: consume next token, split by `,` for individual values
5. For action key (like `nlri`): peek at next token:
   - `info` (`path-information`) — read modifier value, then expect action (`add`/`del`)
   - `add` / `del` — proceed to step 5a
   - 5a. After action, peek at following token:
     - Known sub-key — dict mode: read sub-key-value pairs until unrecognized token
     - Otherwise — consume as value (possibly comma-separated list)
6. Repeat from step 1 with unrecognized token (it becomes the next key)

### Proposed Quick Parse (hot path)

All messages start with `peer <ip> asn <n>`. Quick parse: byte-scan for first 4 tokens (2 key-value pairs) plus dispatch token. Zero allocations — returns byte slices into original buffer.

### Proposed Shared Parser Location (future)

Move from bgp-rs to `internal/component/bgp/textparse.go` so other plugins can reuse the text parser.

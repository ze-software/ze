# Spec: Text Event Format Implementation

## Task

Implement the proposed text event format from `docs/architecture/api/text-format.md` "Proposed Format" section. This is the code change that makes the event delivery format tokenizer-friendly.

Parent spec: `spec-utp-0-umbrella.md`

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/text-format.md` — proposed format design (source of truth for this spec)
  → Decision: Two header shapes converge to `peer <ip> asn <asn>` for ALL messages. Lists use comma separation (no brackets). `announce`/`withdraw` replaced by `family` + `nlri add`/`nlri del`. Repeated keys: list values append, dict values add entries. `set` keyword dropped from NLRI String() output.
- [ ] `docs/architecture/api/text-parser.md` — proposed parser design
  → Decision: Two-phase parsing stays (quick-parse + deferred full parse). Parser rewritten with cursor-based tokenizer (shared `TextScanner` in `internal/plugins/bgp/textparse/`). Replaces `strings.Fields()` positional indexing with sequential token consumption. `announce`/`withdraw` scanning in parseTextNLRIOpsLine replaced by key-dispatch loop over `family`/`nlri`/attribute keywords. bgp-rr parser logic stays in bgp-rr — scanner is the shared component.
- [ ] `docs/architecture/core-design.md` — system architecture
  → Constraint: Engine+Plugin over Unix sockets. Text format is hot path for bgp-rr (only text consumer). FormatNegotiated is JSON-only (not affected). formatSummary is JSON-only (not affected). All wire encoding uses buffer-first, but text formatting uses strings.Builder (not wire encoding — OK).

### Source Files
- [ ] `internal/plugins/bgp/format/text.go` — formatter to modify
  → Constraint: 6 functions need header change to add ASN: FormatOpen:790, FormatNotification:809, FormatKeepalive:826, FormatRouteRefresh:833, formatFilterResultText:627 (UPDATE). formatStateChangeText:855 already has ASN (no change). formatAttributeText:699 needs comma-separated lists for communities (lines 744-778) and AS-PATH (lines 713-719). formatFilterResultText needs `announce`→`family`+`nlri add`, `withdraw`→`family`+`nlri del`.
- [ ] `internal/plugins/bgp/format/text_test.go` — tests to update
  → Constraint: TestFormatMessageText:82 asserts `announce`, `as-path 65001 65002`, family format. TestFormatOpenWithDirection:601, TestFormatKeepaliveWithDirection:644, TestFormatNotificationWithDirection:681 all assert header without ASN. TestFormatStateChange:27 already correct (state header already has ASN). Tests use strings.Contains — partial match, so header change may need targeted fixes.
- [ ] `internal/plugins/bgp-rr/server.go` — parser to rewrite
  → Constraint: All parse functions rewritten using shared `TextScanner` — no more `strings.Fields()` positional indexing. quickParseTextEvent:1011 replaced by scanner-based header parse (peer, addr, asn, n, dispatch token). parseTextNLRIOpsLine:1068 replaced by key-dispatch loop (attribute extraction, family sections, nlri operations). parseTextOpen:1170 rewritten with scanner (keyword loop from after header). parseTextState:1226 rewritten with scanner (unchanged header shape but consistent scanner usage). parseTextRefresh:1257 rewritten with scanner. No parser unit tests exist currently — new unit tests added for all parse functions.
- [ ] `internal/plugins/bgp/nlri/inet.go` — INET String() to change
  → Constraint: String():178 appends `path-id set <id>` for ADD-PATH. Drop `set` → `path-id <id>`. BUT path-id moves to `nlri path-id <id> add` in proposed format — so INET String() should no longer emit path-id at all. Formatter handles it.
- [ ] `internal/plugins/bgp-nlri-vpn/types.go` — VPN String() to change
  → Constraint: String():266 outputs `rd set X prefix set Y label set Z`. Drop all `set` → `rd X prefix Y label Z`.
- [ ] `internal/plugins/bgp-nlri-evpn/types.go` — EVPN String() to change
  → Constraint: 5 route type String() methods use `set` keyword (Type1:308, Type2:481, Type3:595, Type4:715, Type5:875). Drop all `set` from each.
- [ ] `internal/plugins/bgp-nlri-flowspec/types.go` — FlowSpec String() to change
  → Constraint: FlowSpec:336 does NOT use `set` keyword — uses match operators directly. No String() change needed. But VPN FlowSpec wraps with `rd set` — check types.go for VPN variant.
- [ ] `internal/plugins/bgp-nlri-labeled/types.go` — Labeled String() to change
  → Constraint: String():161 outputs `prefix set X label set Y [path-id set Z]`. Drop all `set`.
- [ ] `internal/plugins/bgp-nlri-vpls/types.go` — VPLS String() to change
  → Constraint: String():173 outputs `rd set X ve-id set Y label set Z`. Drop all `set`.
- [ ] `internal/plugins/bgp-nlri-mvpn/types.go` — MVPN String() to change
  → Constraint: String():192 outputs `<route-type> rd set X`. Drop `set`.
- [ ] `internal/plugins/bgp-nlri-rtc/types.go` — RTC String() to change
  → Constraint: String():184 outputs `origin-as set X rt set Y`. Drop all `set`.
- [ ] `internal/plugins/bgp-nlri-mup/types.go` — MUP String() to change
  → Constraint: String():200 outputs `<route-type> rd set X`. Drop `set`.
- [ ] `internal/plugins/bgp-nlri-ls/types_nlri.go` — BGP-LS NLRI String() to change (MISSING FROM ORIGINAL — discovered during research)
  → Constraint: 3 String() methods: Node:73 `node protocol set X asn set Y`, Link:181 `link protocol set X local-asn set Y remote-asn set Z`, Prefix:309 `prefix protocol set X type set Y asn set Z`. Drop all `set`.
- [ ] `internal/plugins/bgp-nlri-ls/types_srv6.go` — BGP-LS SRv6 SID String() to change (MISSING FROM ORIGINAL)
  → Constraint: String():140 `srv6-sid protocol set X asn set Y`. Drop `set`.

### RFC Summaries (not protocol work — N/A)

## Current Behavior (MANDATORY)

Full reference: `docs/architecture/api/text-format.md` "Current Format" section.

- [ ] `internal/plugins/bgp/format/text.go` — formatter (883L)
- [ ] `internal/plugins/bgp/format/text_test.go` — formatter tests
- [ ] `internal/plugins/bgp-rr/server.go` — parser (quickParse, NLRIOps, Open, State, Refresh)
- [ ] `internal/plugins/bgp/nlri/inet.go` — INET String()
- [ ] `internal/plugins/bgp/nlri/inet_test.go` — INET String() tests
- [ ] `internal/plugins/bgp-nlri-vpn/vpn_test.go` — VPN String() tests (assert `set`)
- [ ] `internal/plugins/bgp-nlri-evpn/types_test.go` — EVPN String() tests (assert `set`)
- [ ] `internal/plugins/bgp-nlri-labeled/types_test.go` — Labeled String() tests (assert `set`)
- [ ] `internal/plugins/bgp-nlri-vpls/types_test.go` — VPLS String() tests (assert `set`)
- [ ] `internal/plugins/bgp-nlri-mvpn/types_test.go` — MVPN String() tests (assert `set`)
- [ ] `internal/plugins/bgp-nlri-rtc/types_test.go` — RTC String() tests (assert `set`)
- [ ] `internal/plugins/bgp-nlri-mup/types_test.go` — MUP String() tests (assert `set`)
- [ ] `internal/plugins/bgp-nlri-ls/types_nlri.go` — BGP-LS String() (uses `set`)
- [ ] `internal/plugins/bgp-nlri-ls/types_srv6.go` — BGP-LS SRv6 String() (uses `set`)
- [ ] `internal/plugins/bgp-nlri-ls/types_test.go` — BGP-LS String() tests (assert `set`)
- [ ] `internal/plugins/bgp-nlri-flowspec/types.go` — FlowSpec String() (no `set` — verified)

Behavior to preserve: FormatNegotiated (JSON-only), formatSummary (JSON-only), FlowSpec String() (no `set`), state header shape (already has ASN), capability repeated-key format (unchanged).

### Headers

Two shapes: State events use `peer <ip> asn <asn> state <s>` (includes ASN). Message events use `peer <ip> <direction> <type> <msgid> <body>` (no ASN in header). Six formatter functions produce message headers: FormatOpen, FormatNotification, FormatKeepalive, FormatRouteRefresh, formatFilterResultText (UPDATE), formatStateChangeText (state). Only state already has ASN.

### UPDATE Structure

`announce` keyword marks announced routes (with attributes). `withdraw` keyword marks withdrawn routes (no attributes). Per-family sections follow after the keyword: `<family> next-hop <ip> nlri <prefixes>` for announce, `<family> nlri <prefixes>` for withdraw. Multi-family UPDATEs chain family sections. Announce+withdraw in same UPDATE produces two lines (same msgID).

### Attribute Formatting

AS-PATH: space-separated (`65001 65002`). Communities: bracket-delimited, space-separated (`[65001:100 65002:200]`). Large/Extended communities: same bracket pattern. Scalars (origin, med, local-pref, next-hop): `key value`.

### NLRI String() Methods

All complex NLRI plugins use `set` keyword between field name and value: `rd set X prefix set Y`. INET unicast: plain prefix, but ADD-PATH appends `path-id set <id>`. FlowSpec does NOT use `set` — uses match operators directly.

### Parser (bgp-rr/server.go)

Two-phase: quickParseTextEvent (lightweight routing by field index) → deferred parseTextNLRIOps (full parse on worker goroutine). State detected by positional check: `fields[4] == "state"`. Message type at `fields[3]`, msgID from `fields[4]`. parseTextNLRIOpsLine scans for `announce`/`withdraw` action tokens, then family + NLRI sequences. All parse functions use `strings.Fields()` — allocates per call.

### Callers of NLRI String()

String() called ONLY by text formatter (text.go:652 announce, text.go:672 withdraw) and JSON fallback (text.go:435). Text command parser has independent grammar — does NOT parse String() output. Safe to change.

### Test Coverage

Unit tests in text_test.go assert on current format. NLRI plugin tests (*StringCommandStyle) assert on `set` keyword. No parser unit tests in bgp-rr. No `.ci` functional test files match text event format strings.

## Data Flow (MANDATORY)

### Entry Points
- `FormatMessage()` in `text.go` — all text formatting starts here
- Each NLRI plugin `String()` method — produces NLRI text representation

### Transformation Path

1. **Wire bytes arrive** via TCP, parsed by FSM into `RawMessage` (type, raw bytes, direction, msgID)
2. **Reactor dispatches** to `server/events.go` which calls `FormatMessage(peer, msg, content, "")` — single entry point
3. **FormatMessage** checks encoding (text vs JSON) and message type. For UPDATE: calls `filter.ApplyToUpdate()` to get `FilterResult` (lazy-parsed attributes + NLRI per family), then `formatFilterResultText(peer, result, msgID, direction, ctx)`
4. **formatFilterResultText** builds text header, calls `formatAttributesText()` for shared attributes, iterates announced/withdrawn families, calls each NLRI's `String()` method
5. **NLRI String()** produces text representation (with `set` keywords currently)
6. **Text line** written to plugin's Socket B stdin (newline-framed)
7. **bgp-rr reads** Socket B, calls `dispatchText(text)` → `quickParseTextEvent(text)` for lightweight routing
8. **quickParseTextEvent** returns (eventType, msgID, peerAddr, text). UPDATE goes to per-peer worker queue
9. **Worker** calls `parseTextNLRIOps(text)` for full NLRI extraction → `map[family][]FamilyOperation`
10. **bgp-rr forwards** the event to destination peers (may re-format or forward raw)

For non-UPDATE messages: FormatOpen/FormatNotification/FormatKeepalive/FormatRouteRefresh produce text directly. Parser calls parseTextOpen/parseTextState/parseTextRefresh inline (not deferred).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Formatter → NLRI String() | `n.String()` called at text.go:652 (announce) and text.go:672 (withdraw) | [ ] |
| Formatter → Socket B | Text line written via Hub to plugin stdin, newline-framed | [ ] |
| Parser → Event struct | quickParseTextEvent:1011 routes, parseTextNLRIOps:1057 extracts NLRIs | [ ] |
| Parser → Forward context | dispatchText:503 stores raw text for per-peer deferred parse | [ ] |

### Integration Points

| Component | Interacts With | Interface | Impact |
|-----------|---------------|-----------|--------|
| `formatFilterResultText` | All NLRI `String()` methods | `n.String()` at text.go:652,672 | String() output change propagates to formatter output |
| `formatAttributeText` | `attribute.Communities`, `ASPath`, etc. | Type assertion + `.String()` | Comma list change is formatter-local |
| `FormatOpen/Notification/Keepalive/Refresh` | `plugin.PeerInfo` | `peer.PeerAS` for ASN | Header change uses existing field |
| `quickParseTextEvent` | `dispatchText` | Returns (type, msgID, peerAddr, text) | Signature unchanged, field extraction changes |
| `parseTextNLRIOps` | `processForward` withdrawal tracking | Returns `map[family][]FamilyOperation` | Return type unchanged, parsing logic changes |
| Shared `TextScanner` | All parse functions in bgp-rr | `next()/peek()/done()` | New dependency — bgp-rr imports `textparse` package |
| `server/events.go` | `FormatMessage` | Calls formatter with `PeerInfo` + `RawMessage` | No change needed (PeerInfo already has PeerAS) |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `FormatMessage()` → text output | → | `formatFilterResultText()` uniform header + family + nlri add/del | `TestFormatMessageText` |
| `FormatOpen()` → text output | → | ASN in OPEN header | `TestFormatOpenWithDirection` |
| `FormatKeepalive()` → text output | → | ASN in KEEPALIVE header | `TestFormatKeepaliveWithDirection` |
| `FormatNotification()` → text output | → | ASN in NOTIFICATION header | `TestFormatNotificationWithDirection` |
| `FormatRouteRefresh()` → text output | → | ASN in REFRESH header | `TestFormatRefreshWithDirection` (new or update existing) |
| `quickParseTextEvent()` → event routing | → | Scanner-based uniform header parse | `TestQuickParseTextEvent` (new) |
| `parseTextNLRIOps()` → withdrawal map | → | Key-dispatch family/nlri extraction | `TestParseTextNLRIOps` (new) |
| `parseTextOpen()` → Event struct | → | Scanner-based OPEN parse | `TestParseTextOpen` (new) |
| `parseTextRefresh()` → Event struct | → | Scanner-based REFRESH parse | `TestParseTextRefresh` (new) |
| NLRI `String()` → text repr | → | No `set` keyword | All `Test*StringCommandStyle` (updated) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Formatter output | Uniform header `peer <ip> asn <asn>` for all message types |
| AC-2 | Attribute lists | Comma-separated, no brackets: `as-path 65001,65002` |
| AC-3 | NLRI operations | `nlri add`/`nlri del` replaces `announce`/`withdraw` |
| AC-4 | Family keyword | Explicit `family <afi/safi>` before per-family section |
| AC-5 | Complex NLRI | No `set` keyword: `rd 65000:100 prefix 10.0.0.0/24` |
| AC-6 | ADD-PATH | `nlri path-id 42 add 10.0.0.0/24` modifier before action |
| AC-7 | Capabilities | Repeated dict key unchanged: `cap 1 multiprotocol ipv4/unicast cap 65 asn4 65001` |
| AC-8 | Comma tolerance | Parser accepts `value1, value2` (strips whitespace after comma) |
| AC-9 | bgp-rr parser | Updated to parse new format, all existing behavior preserved |
| AC-10 | NLRI String() | All plugin String() methods updated (drop `set` keyword) |
| AC-11 | Shared scanner | `TextScanner` in `internal/plugins/bgp/textparse/` with `next()`/`peek()`/`done()` |
| AC-12 | Parser unit tests | New unit tests for quickParseTextEvent, parseTextNLRIOps, parseTextOpen, parseTextState, parseTextRefresh |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTextScannerNext` | `textparse/scanner_test.go` | Scanner tokenizes whitespace-delimited tokens, empty input, trailing whitespace | pending |
| `TestTextScannerPeek` | `textparse/scanner_test.go` | Peek returns next token without advancing position | pending |
| `TestTextScannerDone` | `textparse/scanner_test.go` | Done returns true at end of input | pending |
| `TestQuickParseTextEvent` | `bgp-rr/server_test.go` | All dispatch types: state, received update, sent open, empty update, negotiated | pending |
| `TestParseTextNLRIOps` | `bgp-rr/server_test.go` | Simple NLRIs (comma-list), multi-family, add/del, path-id modifier, complex multi-token NLRI (EVPN Type 5 with 12 tokens) | pending |
| `TestParseTextOpen` | `bgp-rr/server_test.go` | OPEN with ASN, router-id, hold-time, capabilities | pending |
| `TestParseTextState` | `bgp-rr/server_test.go` | State up/down/established | pending |
| `TestParseTextRefresh` | `bgp-rr/server_test.go` | Refresh, BORR, EORR with family | pending |
| `TestFormatMessageText` | `format/text_test.go` (update) | AC-1 to AC-4: uniform header, comma lists, family + nlri add/del | pending |
| `TestFormatOpenWithDirection` | `format/text_test.go` (update) | AC-1: ASN in OPEN header | pending |
| `TestFormatKeepaliveWithDirection` | `format/text_test.go` (update) | AC-1: ASN in KEEPALIVE header | pending |
| `TestFormatNotificationWithDirection` | `format/text_test.go` (update) | AC-1: ASN in NOTIFICATION header | pending |
| `TestINETStringCommandStyle` | `nlri/inet_test.go` (update) | AC-10: no `set`, no path-id in String() | pending |
| `Test*StringCommandStyle` (all plugins) | various `*_test.go` (update) | AC-5, AC-10: no `set` keyword | pending |

### Functional Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Existing bgp-rr functional tests | `test/plugin/` | AC-9: bgp-rr still forwards correctly with new format | pending |

## Changes Summary

### Formatter Changes (`text.go`)

| Change | Current | New |
|--------|---------|-----|
| Header | Two shapes (state vs message) | Uniform `peer <ip> asn <asn>` |
| UPDATE sections | `announce`/`withdraw` keywords | `family` + `nlri add`/`nlri del` |
| AS-PATH | `as-path 65001 65002` | `as-path 65001,65002` |
| Communities | `community [65001:100 65002:200]` | `community 65001:100,65002:200` |
| Large communities | `large-community [X]` | `large-community X,Y` |
| Extended communities | `extended-community [hex]` | `extended-community hex1,hex2` |
| Capabilities | `cap 1 multiprotocol ipv4/unicast` repeated | Unchanged (repeated dict key, already fits grammar) |

### NLRI String() Changes (all plugins)

| Change | Current | New |
|--------|---------|-----|
| Drop `set` keyword | `rd set 65000:100` | `rd 65000:100` |
| Drop `set` keyword | `prefix set 10.0.0.0/24` | `prefix 10.0.0.0/24` |
| Drop `set` keyword | `path-id set 42` | moved to `nlri path-id 42 add` |
| Drop `set` keyword | `label set 1000` | `label 1000` |

### Shared Scanner (`textparse/scanner.go`)

| Component | Description |
|-----------|-------------|
| `TextScanner` struct | Holds `text string` + `pos int`, extracts tokens by scanning whitespace in original string |
| `next() (string, bool)` | Returns next whitespace-delimited token as string slice (zero-copy), advances position |
| `peek() (string, bool)` | Like `next()` but does not advance position |
| `done() bool` | True when no more tokens remain |

### Parser Changes (`server.go`)

| Change | Description |
|--------|-------------|
| All parse functions | Rewritten using shared `TextScanner` — no more `strings.Fields()` positional indexing |
| quickParseTextEvent | Scanner-based: consume `peer`, addr, `asn`, n, then dispatch token (`state`/`received`/`sent`/`update`/`negotiated`) |
| parseTextNLRIOpsLine | Key-dispatch loop: extract attributes (scalar/comma-list), `family` sections, `nlri` operations — see NLRI Boundary Detection below |
| parseTextOpen | Scanner-based keyword loop after header (ASN, router-id, hold-time, cap) |
| parseTextState | Scanner-based keyword loop (ASN, state) — header shape unchanged |
| parseTextRefresh | Scanner-based: type from dispatch, `family` keyword extraction |
| Comma tolerance | Strip whitespace after commas in list values |

### NLRI Boundary Detection

The key-dispatch loop in `parseTextNLRIOpsLine` must determine where one NLRI entry ends and the next begins. After dropping the `set` keyword, complex NLRIs are multi-token strings (e.g., `rd 65000:100 prefix 10.0.0.0/24 label 1000`). The parser must collect the right tokens for each NLRI without consuming tokens that belong to the next keyword section.

#### Grammar context

A single UPDATE line after header extraction looks like:

```
<attributes> family <afi/safi> [next-hop <ip>] nlri [path-id <id>] add|del <nlri-tokens> [nlri ...] [family ...] [<attributes> ...]
```

The top-level keywords recognized by the key-dispatch loop are:

| Keyword | Followed By | Repeatable |
|---------|-------------|------------|
| `origin` | one scalar value | no |
| `as-path` | one comma-list value | no |
| `med` | one scalar value | no |
| `local-preference` | one scalar value | no |
| `atomic-aggregate` | nothing (flag) | no |
| `aggregator` | one `asn:ip` value | no |
| `originator-id` | one IP value | no |
| `cluster-list` | one comma-list value | no |
| `community` | one comma-list value | no |
| `large-community` | one comma-list value | no |
| `extended-community` | one comma-list value | no |
| `next-hop` | one IP value | no |
| `family` | one `afi/safi` value | yes |
| `nlri` | `[path-id <id>] add\|del <nlri-tokens>` | yes |

#### Algorithm

When the parser encounters `nlri`:

1. **Optional path-id:** If next token is `path-id`, consume it and the following numeric token
2. **Action:** Consume `add` or `del` — error if missing
3. **Collect NLRI tokens:** Consume all subsequent tokens until one of:
   - A top-level keyword from the table above (stop, do not consume)
   - End of input (`scanner.done()` returns true)
4. **Join collected tokens** with space to form one NLRI string entry
5. **Comma-list NLRI:** If only one token was collected AND it contains commas, split by comma — each piece is a separate simple NLRI (e.g., `10.0.0.0/24,10.0.1.0/24` → two entries)

#### Why this works

NLRI sub-field tokens (`rd`, `prefix`, `label`, `esi`, `etag`, `mac`, `ip`, `gateway`, `ve-id`, `origin-as`, `rt`, `node`, `link`, `srv6-sid`, `protocol`, `local-asn`, `remote-asn`, `type`, `flow`) never collide with the top-level keywords listed above. Verified across all 17 NLRI types in 10 plugins:

| NLRI Family | Example After `set` Drop | Token Count |
|-------------|--------------------------|-------------|
| INET unicast | `10.0.0.0/24` | 1 |
| INET unicast (comma) | `10.0.0.0/24,10.0.1.0/24` | 1 (split by comma → 2 entries) |
| VPN | `rd 65000:100 prefix 10.0.0.0/24 label 1000` | 6 |
| Labeled | `prefix 10.0.0.0/24 label 1000` | 4 |
| EVPN Type 2 | `mac-ip rd 65000:100 mac 00:11:22:33:44:55 ip 10.0.0.1 label 1000` | 10 |
| EVPN Type 5 | `ip-prefix rd 65000:100 prefix 10.0.0.0/24 esi 00:...:00 gateway 10.0.0.1 label 1000` | 12 |
| VPLS | `rd 65000:100 ve-id 1 label 100` | 6 |
| RTC | `origin-as 65001 rt 65001:100` | 4 |
| BGP-LS Node | `node protocol isis asn 65001` | 4 |
| BGP-LS Link | `link protocol isis local-asn 65001 remote-asn 65002` | 6 |
| FlowSpec | `flow destination 10.0.0.0/24 protocol =6` | 5+ |

#### Edge cases

| Case | Handling |
|------|----------|
| Empty NLRI after `add`/`del` | Error — at least one token required |
| Multiple `nlri` in same family | Each `nlri` keyword restarts collection (step 1) |
| `family` after NLRI tokens | Stops collection — new family section begins |
| Attribute after NLRI tokens | Stops collection — attribute keywords are top-level |
| Unknown token (not keyword, not end) | Collected as part of current NLRI string |

## Files to Modify
- `internal/plugins/bgp/format/text.go` — formatter (header, attributes, UPDATE structure)
- `internal/plugins/bgp/format/text_test.go` — formatter tests
- `internal/plugins/bgp-rr/server.go` — parser (quickParse, NLRIOps, Open, Refresh field indices)
- `internal/plugins/bgp/nlri/inet.go` — INET String() (drop `path-id set`, just prefix)
- `internal/plugins/bgp/nlri/inet_test.go` — INET String() tests
- `internal/plugins/bgp-nlri-vpn/types.go` — VPN String() (drop `set`)
- `internal/plugins/bgp-nlri-vpn/vpn_test.go` — VPN String() tests
- `internal/plugins/bgp-nlri-evpn/types.go` — EVPN String() (drop `set` from 5 route types)
- `internal/plugins/bgp-nlri-evpn/types_test.go` — EVPN String() tests
- ~~`internal/plugins/bgp-nlri-flowspec/types.go`~~ — FlowSpec String() does NOT use `set` keyword (verified). No change needed.
- `internal/plugins/bgp-nlri-labeled/types.go` — Labeled String() (drop `set`)
- `internal/plugins/bgp-nlri-labeled/types_test.go` — Labeled String() tests
- `internal/plugins/bgp-nlri-vpls/types.go` — VPLS String() (drop `set`)
- `internal/plugins/bgp-nlri-vpls/types_test.go` — VPLS String() tests
- `internal/plugins/bgp-nlri-mvpn/types.go` — MVPN String() (drop `set`)
- `internal/plugins/bgp-nlri-mvpn/types_test.go` — MVPN String() tests
- `internal/plugins/bgp-nlri-rtc/types.go` — RTC String() (drop `set`)
- `internal/plugins/bgp-nlri-rtc/types_test.go` — RTC String() tests
- `internal/plugins/bgp-nlri-mup/types.go` — MUP String() (drop `set`)
- `internal/plugins/bgp-nlri-mup/types_test.go` — MUP String() tests
- `internal/plugins/bgp-nlri-ls/types_nlri.go` — BGP-LS Node/Link/Prefix String() (drop `set`)
- `internal/plugins/bgp-nlri-ls/types_srv6.go` — BGP-LS SRv6 SID String() (drop `set`)
- `internal/plugins/bgp-nlri-ls/types_test.go` — BGP-LS String() tests

## Files to Create
- `internal/plugins/bgp/textparse/scanner.go` — shared `TextScanner` struct
- `internal/plugins/bgp/textparse/scanner_test.go` — scanner unit tests
- `internal/plugins/bgp-rr/server_test.go` — parser unit tests (new file for parse function tests)

## Implementation Steps

1. Create shared `TextScanner` in `internal/plugins/bgp/textparse/scanner.go` with TDD (write scanner tests first, then implement)
2. TDD: Write parser unit tests in `bgp-rr/server_test.go` expecting new format (quickParse, NLRIOps, Open, State, Refresh) — tests FAIL
3. TDD: Update formatter tests in `text_test.go` expecting new format (uniform header, comma lists, family + nlri add/del) — tests FAIL
4. TDD: Update NLRI String() tests across all plugins expecting no `set` keyword — tests FAIL
5. Implement NLRI String() changes (drop `set` keyword) across all 14 plugins
6. Implement formatter changes (`text.go`): uniform header (add ASN to 5 functions), comma-separated lists (AS-PATH, communities), `family` + `nlri add`/`nlri del` replacing `announce`/`withdraw`
7. Rewrite parser (`server.go`): all parse functions use shared `TextScanner`, key-dispatch loop for UPDATE parsing
8. Run full test suite (`make ze-verify`)

### Failure Routing
| Failure | Route To |
|---------|----------|
| bgp-rr forwarding breaks | Check parser rewrite missed a case — compare old vs new parse function outputs |
| Functional tests fail | Trace event format mismatch between formatter and parser |
| NLRI String() breaks other consumers | Verified safe (only text formatter + JSON fallback call String()) |
| Scanner boundary error | Check TextScanner unit tests — edge cases with trailing whitespace, empty input |
| Import cycle | `textparse` package must not import from bgp-rr or other plugins |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

## Implementation Summary

### What Was Implemented
- (to be filled after implementation)

### Documentation Updates
- (to be filled after implementation)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Goal Gates
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete
- [ ] Architecture docs updated

### Quality Gates
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility
- [ ] Explicit > implicit

### Verification
- [ ] `make ze-lint` passes
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Spec moved to `docs/plan/done/NNN-utp-1-event-format.md`
- [ ] Spec included in commit

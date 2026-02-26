# Spec: Text Format Documentation Rewrite + Parser Design

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/architecture.md` - API architecture
4. `internal/plugins/bgp/format/text.go` - text formatter (source of truth)
5. `internal/plugins/bgp-rr/server.go` - text parser (source of truth)

## Task

Two goals:

1. **Doc cleanup:** AI-generated docs in `docs/architecture/api/` contain ~8 factual errors, fabricated code, fabricated benchmarks, broken links. Delete and replace with source-verified docs.

2. **Format redesign for tokenizer-based parsing:** Redesign the text format so that **space is only ever a token separator**. Every token pair follows one of three patterns:
   - `key value` — scalar
   - `key value,value,value` — comma-separated list
   - `key action value` — action on a value

   This enables a trivial tokenizer (split by space) followed by a key-dispatch parser. No token may contain spaces.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - API architecture context
  → Constraint: text format is engine→plugin event delivery, not RPC
- [ ] `docs/architecture/core-design.md` - system architecture
  → Decision: engine passes wire bytes to plugins, plugins parse as needed

### RFC Summaries (not protocol work — N/A)

**Key insights:**
- Text format is internal IPC, not wire protocol — no RFC constraints
- Two consumers: bgp-rr (hot path, text) and other plugins (JSON via shared/event.go)
- Formatter in `format/text.go`, parser in `bgp-rr/server.go` — these are the ground truth

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/bgp/format/text.go` (883L) - all text formatting functions
- [ ] `internal/plugins/bgp/format/json.go` (264L) - JSON encoder (Negotiated, StateUp/Down, etc.)
- [ ] `internal/plugins/bgp/format/decode.go` (420L) - DecodedOpen/Notification/Negotiated structs
- [ ] `internal/plugins/bgp-rr/server.go` (1300L) - text parser: quickParse, parseTextNLRIOps, etc.
- [ ] `internal/plugin/bgp/shared/event.go` - JSON event parser (shared across plugins)
- [ ] `internal/plugins/bgp/nlri/inet.go` - INET NLRI String() with path-id
- [ ] `internal/plugins/bgp-nlri-vpn/types.go` - VPN NLRI String()
- [ ] `internal/plugins/bgp-nlri-evpn/types.go` - EVPN route type String()
- [ ] `internal/plugins/bgp-nlri-flowspec/types.go` - FlowSpec String()

**Behavior to preserve:**
- JSON event format (this spec only touches text format docs)
- All existing text formatter function signatures
- bgp-rr text parser behavior
- IPC protocol (JSON-RPC) unchanged

**Behavior to change:**
- Documentation only — delete inaccurate AI-generated docs, replace with source-verified docs
- Document proposed format tweaks (implementation in follow-up spec)

## Data Flow (MANDATORY)

### Entry Point
- Engine receives BGP wire messages from peers
- Formatter converts to text/JSON based on plugin subscription's `ContentConfig`

### Transformation Path
1. Wire bytes → `RawMessage` (session_read.go)
2. `RawMessage` → `FormatMessage()` dispatches by type and encoding (text.go:27)
3. For UPDATE text: `formatFilterResultText()` → announce/withdraw lines
4. For non-UPDATE text: `FormatOpen/FormatNotification/FormatKeepalive/FormatRouteRefresh`
5. Text output → `Plugin.Deliver()` → per-process channel → Unix socket

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine → Plugin | Text lines over Unix socket stdin | [x] |
| ContentConfig → Formatter | Encoding="text", Format="parsed" | [x] |

### Integration Points
- `server/events.go:onMessageReceived` — calls formatMessageForSubscription
- `bgp-rr/server.go:dispatchText` — entry point for text event parsing in RR
- `shared/event.go:ParseEvent` — JSON counterpart used by other plugins

### Architectural Verification
- [x] No bypassed layers — text goes through same delivery path as JSON
- [x] No unintended coupling — formatter and parser are independent
- [x] No duplicated functionality — replacing inaccurate docs, not adding code
- [x] Zero-copy preserved — documentation task, no wire format changes

## Wiring Test (MANDATORY)

Documentation-only task. Verification is doc accuracy against source code.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `format/text.go` Format* functions | → | Doc BNF grammar rules | Manual: verify each BNF rule matches function output |
| `bgp-rr/server.go` parse* functions | → | Doc parser description | Manual: verify parser doc matches actual parse logic |
| `format/text_test.go` existing tests | → | Doc examples | `TestFormatMessageText` confirms example accuracy |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Each BNF rule in text-format.md "Current Format" section | Verified against actual formatter function in text.go |
| AC-2 | Each message example in "Current Format" sections | Matches actual Format* function output |
| AC-3 | All Go code snippets in docs | None — no fabricated code |
| AC-4 | All links in docs | Point to files that exist |
| AC-5 | NLRI format table | Covers all types from actual String() implementations |
| AC-6 | Parser design doc "Current Parser" section | Describes actual quickParse + type-specific handlers in bgp-rr/server.go |
| AC-7 | Format tweak proposals in "Proposed" sections | Each has rationale, files-to-change list, and "not yet implemented" marker |
| AC-8 | Every doc with proposed content | Clear separation: "Current" vs "Proposed (not yet implemented)" section headers |

## 🧪 TDD Test Plan

Documentation-only — no new code, no new tests.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Existing `TestFormatMessageText` | `format/text_test.go:82` | Text output matches doc examples | Already exists |
| Existing `TestFormatOpenWithDirection` | `format/text_test.go:601` | OPEN text format | Already exists |
| Existing `TestFormatKeepaliveWithDirection` | `format/text_test.go:644` | KEEPALIVE text format | Already exists |
| Existing `TestFormatNotificationWithDirection` | `format/text_test.go:681` | NOTIFICATION text format | Already exists |

### Boundary Tests (N/A — documentation task)

### Functional Tests (N/A — documentation task)

## Files to Modify
- `docs/architecture/api/text-format.md` - new: current format BNF + examples, then proposed format in separate section
- `docs/architecture/api/text-parser.md` - new: current parser description, then proposed parser design in separate section
- `docs/architecture/api/text-coverage.md` - new: coverage table of current implementation
- `.claude/INDEX.md` - update doc paths

## Files to Create
- `docs/architecture/api/text-format.md`
- `docs/architecture/api/text-parser.md`
- `docs/architecture/api/text-coverage.md`

## Files to Delete
- `docs/architecture/api/README.md` — indexes docs that contain fabricated content
- `docs/architecture/api/current-text-format-bnf.md` — ~8 grammar errors
- `docs/architecture/api/message-formats.md` — fabricated Go code
- `docs/architecture/api/text-format-coverage.md` — fabricated benchmarks
- `docs/architecture/api/text-parsing-explained.md` — fabricated code snippets

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | |
| RPC count in arch docs | No | |
| CLI commands/flags | No | |
| API commands doc | No | |
| Plugin SDK docs | No | |
| `.claude/INDEX.md` | Yes | Add text-format, text-parser, text-coverage |

---

## Doc Content: What's Wrong in AI-Generated Docs

| Claim in docs | Actual code | Source |
|---|---|---|
| Uniform header with ASN in all messages | State has ASN, non-state does not | `text.go:631,856` |
| `communities (community)+` keyword | `community [...]` — singular + brackets | `text.go:744-753` |
| `large-communities` keyword | `large-community` — singular | `text.go:755-765` |
| `extended-communities` keyword | `extended-community` — singular | `text.go:767-778` |
| AFI values `vpnv4`, `vpnv6` | `ipv4/vpn`, `ipv6/vpn` — slash-separated | `types.go` |
| Withdraw family-section has next-hop | Withdraw has NO next-hop | `text.go:659-676` |
| ADD-PATH `{prefix:X, path-id:N}` brace syntax | `10.0.0.0/24 path-id set 42` — space tokens | `nlri/inet.go:176-183` |
| Parser code `fields[4]` for msg type | State at `fields[4]`, type at `fields[3]` for non-state | `bgp-rr/server.go:1013-1028` |
| README indexes fabricated docs | Linked files exist but contain fabricated content | file listing |
| Performance numbers (3-10x, 0.5-2us) | No benchmarks exist — fabricated | n/a |
| Go code snippets throughout | All fabricated — don't match real signatures | multiple |

---

## Doc Content: Accurate Current Format

### Two header shapes

```
STATE:      peer <ip> asn <asn> state <state>
NON-STATE:  peer <ip> <direction> <type> <msgid> <body...>
```

### All message types (from source)

```
peer 192.0.2.1 asn 65001 state up
peer 192.0.2.1 received update 1 announce origin igp as-path 65001 65002 ipv4/unicast next-hop 192.0.2.1 nlri 10.0.0.0/24 10.0.1.0/24
peer 192.0.2.1 received update 1 withdraw ipv4/unicast nlri 172.16.0.0/16
peer 192.0.2.1 update
peer 192.0.2.1 received open 1 asn 65001 router-id 1.1.1.1 hold-time 90 cap 1 multiprotocol ipv4/unicast cap 65 asn4 65001
peer 192.0.2.1 sent notification 3 code 6 subcode 2 code-name Cease subcode-name Administrative-Shutdown data 0a0b0c0d
peer 192.0.2.1 sent keepalive 42
peer 192.0.2.1 received refresh 5 family ipv4/unicast
peer 192.0.2.1 received borr 1 family ipv6/unicast
```

### Attribute formats (from formatAttributeText text.go:699)

| Attribute | Format | Delimited? |
|-----------|--------|------------|
| origin | `origin igp` | scalar |
| as-path | `as-path 65001 65002` | NO brackets |
| next-hop | `next-hop 192.0.2.1` | scalar |
| med | `med 100` | scalar |
| local-preference | `local-preference 200` | scalar |
| community | `community [65001:100 65002:200]` | brackets |
| large-community | `large-community [65001:1:2]` | brackets |
| extended-community | `extended-community [0002000a0b0c0d0e]` | brackets |
| unknown | `attr-42 deadbeef` | scalar |

### NLRI String() formats

| Type | Format | Multi-token? | Source |
|------|--------|-------------|--------|
| IPv4/IPv6 | `10.0.0.0/24` | No | `nlri/inet.go:178` |
| + ADD-PATH | `10.0.0.0/24 path-id set 42` | Yes | `nlri/inet.go:177` |
| VPN | `rd set 65000:100 prefix set 10.0.0.0/24 label set 1000` | Yes | `bgp-nlri-vpn/types.go:264` |
| Labeled | `prefix set 10.0.0.0/24 label set 1000` | Yes | `bgp-nlri-labeled/types.go:159` |
| EVPN Type1 | `ethernet-ad rd set X esi set Y etag set Z` | Yes | `bgp-nlri-evpn/types.go:308` |
| EVPN Type2 | `mac-ip rd set X mac set Y [ip set Z]` | Yes | `bgp-nlri-evpn/types.go:481` |
| EVPN Type3 | `multicast rd set X ip set Y` | Yes | `bgp-nlri-evpn/types.go:595` |
| EVPN Type5 | `ip-prefix rd set X prefix set Y` | Yes | `bgp-nlri-evpn/types.go:875` |
| FlowSpec | `flow destination 10.0.0.0/24 port ==80` | Yes | `bgp-nlri-flowspec/types.go:334` |
| VPLS | `rd set X ve-id set Y label set Z` | Yes | `bgp-nlri-vpls/types.go:172` |
| MVPN | `intra-as-i-pmsi-ad [rd set X]` | Yes | `bgp-nlri-mvpn/types.go:191` |
| RTC | `origin-as set X rt set Y` or `default` | Yes | `bgp-nlri-rtc/types.go:183` |
| MUP | `isd [rd set X]` | Yes | `bgp-nlri-mup/types.go:199` |

**Edge cases:** RTC outputs bare `default` when `IsDefault()` is true (no sub-keys). FlowSpec match operators (`==`, `>=`, `<=`, `!=`, `&`, etc.) pass through as part of the value token — these contain special characters but never commas or spaces, so the tokenizer handles them correctly.

---

## New Format Design: Tokenizer-Based Key-Value

### Design Principle

Space is ONLY a token separator. The tokenizer splits by whitespace. The parser consumes tokens as key-value pairs. Three base patterns plus dict mode:

| Pattern | Structure | Example |
|---------|-----------|---------|
| scalar | `key value` | `origin igp`, `med 100` |
| list | `key value,value,value` | `as-path 65001,65002`, `community 65001:100,65002:200` |
| action | `key action value[,value]` | `nlri add 10.0.0.0/24,10.0.1.0/24` |
| action+dict | `key action subkey1 val1 subkey2 val2 ...` | `nlri add rd 65000:100 prefix 10.0.0.0/24` |

**Comma tolerance:** The canonical format uses no spaces after commas (`value1,value2,value3`). The formatter always generates this form. However, for human-authored input, the parser strips whitespace after commas before processing — so `value1, value2, value3` is accepted and treated identically to `value1,value2,value3`.

**Token invariants:** No value token may contain a comma (comma is the list separator). No capability value may contain a colon (colon is the capability field separator). These are format invariants — all current value types satisfy them.

Dict mode: When the token after an action is a **known sub-key** (not a value), the parser reads sub-key-value pairs until it encounters an **unrecognized token** (the next top-level key) or end-of-line. No special keyword needed.

**Dict sub-key coupling:** The parser's sub-key table per family must be updated whenever an NLRI type adds a new field. Adding a sub-key without updating the parser table means the new field exits dict mode prematurely.

### Header: Uniform for All Messages

All messages start with `peer <ip> asn <asn>`. After `asn <n>`, the next token dispatches:
- `state` → state event (key-value: `state up`)
- `negotiated` → negotiated event (key-value pairs follow) — **proposed addition**, not yet in current code
- `received` / `sent` → direction, followed by `type` key-value pair

### Attribute Mapping (current → new)

| Current | New | Pattern |
|---------|-----|---------|
| `as-path 65001 65002` | `as-path 65001,65002` | list |
| `community [65001:100 65002:200]` | `community 65001:100,65002:200` | list |
| `large-community [65001:1:2 65002:3:4]` | `large-community 65001:1:2,65002:3:4` | list |
| `extended-community [0002... 0003...]` | `extended-community 0002...,0003...` | list |
| `origin igp` | `origin igp` | scalar (unchanged) |
| `med 100` | `med 100` | scalar (unchanged) |
| `local-preference 200` | `local-preference 200` | scalar (unchanged) |
| `next-hop 192.0.2.1` | `next-hop 192.0.2.1` | scalar (unchanged) |

### NLRI Mapping (current → new)

Simple NLRIs (unicast) use action + comma list:

| Current (in announce section) | New |
|-------------------------------|-----|
| `nlri 10.0.0.0/24 10.0.1.0/24` | `nlri add 10.0.0.0/24,10.0.1.0/24` |
| `nlri 172.16.0.0/16` (in withdraw) | `nlri del 172.16.0.0/16` |

ADD-PATH: `path-id` is a modifier BEFORE the action. It qualifies the NLRI group (all prefixes in one announce share the same path-id):

| Current | New |
|---------|-----|
| `10.0.0.0/24 path-id set 42` | `nlri path-id 42 add 10.0.0.0/24,10.0.1.0/24` |

Parser sees `nlri`, peeks: if next token is `path-id` → read modifier value, then expect action. If next token is `add`/`del` → no modifier.

Complex NLRIs use action + dict. Parser sees known sub-key after action → dict mode. All `set` keywords dropped — sub-keys are just `key value`:

| Current | New (action + dict) |
|---------|-----|
| `rd set 65000:100 prefix set 10.0.0.0/24 label set 1000` | `nlri add rd 65000:100 prefix 10.0.0.0/24 label 1000` |
| `prefix set 10.0.0.0/24 label set 1000` | `nlri add prefix 10.0.0.0/24 label 1000` |
| `mac-ip rd set X mac set Y ip set Z` | `nlri add route-type mac-ip rd X mac Y ip Z` |
| `flow destination 10.0.0.0/24 destination-port ==80` | `nlri add destination 10.0.0.0/24 destination-port ==80` |
| `rd set X ve-id set Y label set Z` | `nlri add rd X ve-id Y label Z` |
| `origin-as set X rt set Y` | `nlri add origin-as X rt Y` |

ADD-PATH with complex NLRIs: modifier before action, dict after:

```
nlri path-id 42 add rd 65000:100 prefix 10.0.0.0/24 label 1000
```

**Dict boundary:** parser reads sub-key-value pairs until it sees a token NOT in the sub-key set (that token is the next top-level key).

**Multiple complex NLRIs:** repeat `nlri` — the repeated `nlri` is a top-level key that exits the previous dict. `nlri` is the only key that may repeat within a single message line:

```
nlri add rd 65000:100 prefix 10.0.0.0/24 nlri add rd 65001:200 prefix 10.1.0.0/24
```

Known sub-keys by family:

| Family | Sub-keys |
|--------|----------|
| ipv4/unicast, ipv6/unicast | `path-id` |
| ipv4/vpn, ipv6/vpn | `rd`, `prefix`, `label`, `path-id` |
| ipv4/nlri-mpls, ipv6/nlri-mpls | `prefix`, `label`, `path-id` |
| l2vpn/evpn | `route-type`, `rd`, `esi`, `etag`, `mac`, `ip`, `prefix`, `label`, `gateway` |
| ipv4/flowspec, ipv6/flowspec | `destination`, `source`, `protocol`, `port`, `destination-port`, `source-port`, `icmp-type`, `icmp-code`, `tcp-flags`, `packet-length`, `dscp`, `fragment`, `flow-label` |
| l2vpn/vpls | `rd`, `ve-id`, `label` |
| ipv4/rtc | `origin-as`, `rt` |
| mup families | `route-type`, `rd` |
| mvpn families | `route-type`, `rd` |

### Capability Mapping (OPEN)

Current: `cap 1 multiprotocol ipv4/unicast cap 65 asn4 65001` (3-4 tokens per cap, repeated)

New: `cap 1:multiprotocol:ipv4/unicast,65:asn4:65001,2:route-refresh` (list of colon-encoded caps). Colon separates code:name:value within each cap. Families use `/` not `:`, so no collision. Invariant: capability values must not contain colons.

### UPDATE Structure

`announce`/`withdraw` keywords deleted — replaced by `nlri add`/`nlri del`. No aliasing or backward compatibility. Attributes are key-value pairs. `family` marks the start of a per-family section.

**Note:** `family` is context-dependent. In UPDATE messages it opens a per-family section (with next-hop and NLRIs). In REFRESH/BORR messages it is a simple scalar. The parser dispatches by message type before interpreting `family`; a pure tokenizer alone cannot resolve this.

Announce: `... origin igp as-path 65001,65002 family ipv4/unicast next-hop 192.0.2.1 nlri add 10.0.0.0/24,10.0.1.0/24`

Withdraw: `... family ipv4/unicast nlri del 172.16.0.0/16`

Combined (multi-line, same msgid):

```
peer 192.0.2.1 asn 65001 received update 1 origin igp family ipv4/unicast next-hop 192.0.2.1 nlri add 10.0.0.0/24
peer 192.0.2.1 asn 65001 received update 1 family ipv4/unicast nlri del 172.16.0.0/16
```

### Complete New Format Examples

```
peer 192.0.2.1 asn 65001 state up

peer 192.0.2.1 asn 65001 received update 1 origin igp as-path 65001,65002 med 100 community 65001:100,65002:200 family ipv4/unicast next-hop 192.0.2.1 nlri add 10.0.0.0/24,10.0.1.0/24

peer 192.0.2.1 asn 65001 received update 2 family ipv4/unicast nlri del 172.16.0.0/16,10.0.0.0/8

peer 192.0.2.1 asn 65001 received update 3 origin igp family ipv4/vpn next-hop 192.0.2.1 nlri add rd 65000:100 prefix 10.0.0.0/24 label 1000 nlri add rd 65001:200 prefix 10.1.0.0/24 label 2000

peer 192.0.2.1 asn 65001 received update 4 origin igp family l2vpn/evpn next-hop 192.0.2.1 nlri add route-type mac-ip rd 65000:100 mac aa:bb:cc:dd:ee:ff ip 10.0.0.1

peer 192.0.2.1 asn 65001 received update 5 origin igp family ipv4/unicast next-hop 192.0.2.1 nlri path-id 42 add 10.0.0.0/24,10.0.1.0/24

peer 192.0.2.1 asn 65001 received update 6 origin igp family ipv4/vpn next-hop 192.0.2.1 nlri path-id 7 add rd 65000:100 prefix 10.0.0.0/24 label 1000

peer 192.0.2.1 asn 65001 update 0

peer 192.0.2.1 asn 65001 received open 1 asn 65001 router-id 1.1.1.1 hold-time 90 cap 1:multiprotocol:ipv4/unicast,65:asn4:65001,2:route-refresh

peer 192.0.2.1 asn 65001 sent notification 3 code 6 subcode 2 code-name Cease subcode-name Administrative-Shutdown data 0a0b0c0d

peer 192.0.2.1 asn 65001 sent keepalive 42

peer 192.0.2.1 asn 65001 received refresh 5 family ipv4/unicast

peer 192.0.2.1 asn 65001 received borr 1 family ipv6/unicast

peer 192.0.2.1 asn 65001 negotiated hold-time 90 asn4 true route-refresh normal families ipv4/unicast,ipv6/unicast add-path-send ipv4/unicast add-path-receive ipv4/unicast
```

### Resolved Questions

1. **ADD-PATH:** `path-id` is a modifier before the action: `nlri path-id 42 add 10.0.0.0/24`. Clean separation — modifier qualifies, action operates.

2. **Empty UPDATE:** Always include ASN in header: `peer 192.0.2.1 asn 65001 update 0`. Empty UPDATEs have no direction (`received`/`sent`) — the dispatch token is `update` directly after `asn <n>`. This is the only message type without a direction prefix.

3. **Multiple families:** Matches BGP UPDATE wire encoding. A single UPDATE can carry IPv4 unicast body NLRI + MP_REACH (one family announce) + MP_UNREACH (one family withdraw). Each `family` key starts a new family context, exiting the previous one.

---

## Parser Design

### Tokenizer

Split input by whitespace into token slices. Zero allocations if tokens are byte slices into original buffer (no string conversion).

### Key-dispatch parser

After tokenizing, parser reads tokens sequentially:

1. Read token → look up in key table
2. Key table returns: pattern (scalar / list / action) and known sub-keys (for dict)
3. For scalar: consume next token as value
4. For list: consume next token, split by `,` for individual values
5. For action key (like `nlri`): peek at next token:
   - `path-id` → read modifier value, then continue to step 5a
   - `add` / `del` → step 5a
   5a. After action token, peek at following token:
   - Following token is a known sub-key → dict mode: read sub-key-value pairs until unrecognized token
   - Otherwise → consume as value (possibly comma-separated list)
6. Repeat from step 1 with unrecognized token (it's the next key)

### Quick parse (hot path)

All messages start with `peer <ip> asn <n>`. Quick parse: byte-scan for first 4 tokens (2 key-value pairs) plus dispatch token. Zero allocations — returns byte slices into original buffer.

### Shared parser location (future — not in scope for this doc-only spec)

Move from bgp-rr to `internal/plugin/bgp/shared/textparse.go`

---

## Implementation Steps

**Doc structure rule:** Each doc MUST clearly separate current (implemented) from proposed (not yet implemented). Use `## Current Format` / `## Proposed Format (not yet implemented)` section headers. The proposed sections must open with a note: *"This section describes a planned redesign. No code implements this yet."*

1. **Delete AI-generated docs** — README.md, current-text-format-bnf.md, message-formats.md, text-format-coverage.md, text-parsing-explained.md
2. **Write text-format.md** — Two major sections: (a) Current Format: BNF from source, all examples, NLRI table, verified against formatter functions; (b) Proposed Format: tokenizer-based key-value redesign with "not yet implemented" marker
3. **Write text-parser.md** — Two major sections: (a) Current Parser: quickParse + type-specific handlers in bgp-rr/server.go; (b) Proposed Parser Design: tokenizer + key-dispatch approach with "not yet implemented" marker
4. **Write text-coverage.md** — Coverage table of current implementation only (no proposed additions)
5. **Update INDEX.md** — add new doc paths, remove stale references
6. **Verify** — cross-check every BNF rule against formatter source code; confirm all "current" content matches code and all "proposed" content is marked as such

### Failure Routing
| Failure | Route To |
|---------|----------|
| BNF rule doesn't match source | Re-read formatter function, fix BNF |
| Example doesn't match test output | Re-read test expectations, fix example |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Pipe-delimited format redesign | User: "we already have a format" | Tokenizer-based key-value |
| Semicolons between NLRIs | Doesn't solve multi-token NLRIs for complex types | Dict mode with known sub-keys |
| `with` keyword for dict mode | Unnecessary — parser detects dict by recognizing sub-keys | Sub-key recognition (unknown token exits dict) |

## Design Insights

- The AI hallucinated a pipe-delimited `!OPT v1|...` format — this never existed
- Multi-token NLRIs don't need single-token encoding — they're already key-value pairs internally (drop `set`, keep `key value`)
- Dict boundary detection is natural: parser knows its sub-keys, unknown token exits dict
- `announce`/`withdraw` replaced by `nlri add`/`nlri del` — action on the NLRI key itself
- All variable-length lists (as-path, community) use commas — no brackets needed

## Implementation Summary

### What Was Implemented
- Deleted 5 AI-generated docs containing fabricated code, benchmarks, and grammar errors
- Created `text-format.md` — current format BNF + attribute/NLRI tables + proposed format design
- Created `text-parser.md` — current parser architecture + proposed parser design
- Created `text-coverage.md` — message type / attribute / NLRI family coverage tables
- Updated `.claude/INDEX.md` — added 3 new doc paths and keyword entry

### Documentation Updates
- `docs/architecture/api/text-format.md` — new
- `docs/architecture/api/text-parser.md` — new
- `docs/architecture/api/text-coverage.md` — new
- `.claude/INDEX.md` — updated (architecture docs table + keyword table)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Delete AI-generated docs | ✅ Done | 5 files removed from `docs/architecture/api/` | README.md, current-text-format-bnf.md, message-formats.md, text-format-coverage.md, text-parsing-explained.md |
| Write text-format.md | ✅ Done | `docs/architecture/api/text-format.md` | Current + Proposed sections separated |
| Write text-parser.md | ✅ Done | `docs/architecture/api/text-parser.md` | Current + Proposed sections separated |
| Write text-coverage.md | ✅ Done | `docs/architecture/api/text-coverage.md` | Current implementation only |
| Update INDEX.md | ✅ Done | `.claude/INDEX.md` | 3 doc paths + keyword row added |
| Verify BNF against source | ✅ Done | Agent verification | All attribute formats, message headers, NLRI types verified |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | Agent verified each BNF rule against `text.go` function line numbers | All 8 attributes + headers match |
| AC-2 | ✅ Done | Examples cross-checked against `text_test.go` expected values | State, UPDATE, OPEN, KEEPALIVE, NOTIFICATION |
| AC-3 | ✅ Done | `grep '```\w'` on all 3 new files: no matches | No fabricated Go code |
| AC-4 | ✅ Done | All source file references checked — files exist | 11 source files verified |
| AC-5 | ✅ Done | `text-format.md` NLRI table covers 14 types from all plugin String() methods | Including EVPN Type4, EVPN unknown, edge cases |
| AC-6 | ✅ Done | `text-parser.md` "Current Parser" describes actual quickParse + type handlers | Verified against `server.go` function signatures |
| AC-7 | ✅ Done | Proposed sections have rationale and file references | Each proposed change links to current behavior |
| AC-8 | ✅ Done | `grep 'not yet implemented'` confirms markers on both proposed sections | text-format.md:173, text-parser.md:146 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestFormatMessageText | ✅ Exists | `format/text_test.go:77` | Validates text output examples in docs |
| TestFormatOpenWithDirection | ✅ Exists | `format/text_test.go:601` | Validates OPEN text format |
| TestFormatKeepaliveWithDirection | ✅ Exists | `format/text_test.go:644` | Validates KEEPALIVE text format |
| TestFormatNotificationWithDirection | ✅ Exists | `format/text_test.go:681` | Validates NOTIFICATION text format |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `docs/architecture/api/text-format.md` | ✅ Created | 13K, current + proposed format |
| `docs/architecture/api/text-parser.md` | ✅ Created | 7.5K, current + proposed parser |
| `docs/architecture/api/text-coverage.md` | ✅ Created | 4.9K, current coverage tables |
| `.claude/INDEX.md` | ✅ Updated | 3 doc paths + 1 keyword row |
| `docs/architecture/api/README.md` | ✅ Deleted | Was indexing fabricated docs |
| `docs/architecture/api/current-text-format-bnf.md` | ✅ Deleted | Had grammar errors |
| `docs/architecture/api/message-formats.md` | ✅ Deleted | Had fabricated Go code |
| `docs/architecture/api/text-format-coverage.md` | ✅ Deleted | Had fabricated benchmarks |
| `docs/architecture/api/text-parsing-explained.md` | ✅ Deleted | Had fabricated code snippets |

### Audit Summary
- **Total items:** 27
- **Done:** 27
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Checklist

### Goal Gates
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete
- [ ] Architecture docs updated (this IS the architecture doc update)

### Quality Gates
- [ ] No fabricated content in docs
- [ ] All BNF rules verified against source
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] N/A — documentation-only task, no new code; existing tests validate format

### Design
- [x] No premature abstraction — documentation only
- [x] No speculative features — documents what exists + minor tweaks
- [x] Single responsibility — each doc covers one topic
- [x] Explicit > implicit — BNF from source, not guessed

### Verification
- [ ] `make ze-lint` passes
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Spec moved to `docs/plan/done/NNN-text-format-docs.md`
- [ ] Spec included in commit

# Spec: fmt-1-text-update

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-fmt-0-append (completed, see plan/learned/614-fmt-0-append.md) |
| Phase | - |
| Updated | 2026-04-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you are reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `.claude/rules/buffer-first.md` -- existing Append / WriteTo patterns
4. `plan/learned/614-fmt-0-append.md` -- non-UPDATE migration already landed
5. `internal/component/bgp/format/text.go` -- fmt-0 Append helpers (`appendJSONString`, `appendReplacingByte`, `appendPeerJSON`, `AppendOpen`, `AppendNotification`, `AppendKeepalive`, `AppendRouteRefresh`, `AppendStateChange`, `AppendEOR`, `AppendCongestion`)
6. `internal/component/bgp/format/text_update.go` -- UPDATE-path code to migrate
7. `internal/component/bgp/format/text_human.go`, `text_json.go`, `summary.go`, `peer_json.go` -- in-scope helpers
8. `internal/component/bgp/nlri/nlri.go` -- `nlri.JSONWriter` interface (to be renamed to `nlri.JSONAppender`)
9. `tmp/session/session-state-fmt-1-text-update-32940.md` -- per-spec digests of every source file in scope

## Task

Complete the companion migration to `spec-fmt-0-append`: move the UPDATE-path
formatters in `internal/component/bgp/format/text_update.go` (and the helpers
in `text_human.go` / `text_json.go` / `summary.go` / `peer_json.go` that they
delegate to) off `fmt.Sprintf` / `fmt.Fprintf` / `strings.Builder` /
`strings.Replace` onto `AppendXxx(buf []byte, ...) []byte` helpers that mirror
the spec-fmt-0 non-UPDATE shape.

In-scope migrations:
- `format/text_update.go` -- 9 functions (`FormatMessage`, `formatEmptyUpdate`,
  `formatNonUpdate`, `formatFromFilterResult`, `formatRawFromResult`,
  `formatParsedFromResult`, `formatFullFromResult`, `FormatSentMessage`,
  `FormatNegotiated`).
- `format/text_human.go` -- `formatFilterResultText`, `formatStateChangeText`
  + helpers (`formatAttributesText`, `formatAttributeText`, `writeNLRIList`).
- `format/text_json.go` -- `formatFilterResultJSON`, `formatStateChangeJSON`
  + helpers (`formatAttributesJSON`, `formatAttributeJSON`,
  `formatFamilyOpsJSON`, `formatNLRIJSON`, `formatNLRIJSONValue`).
- `format/summary.go` -- `buildSummaryJSON`, `formatSummary`,
  `formatSummaryEmpty`.
- `format/peer_json.go` -- DELETE entirely (`writePeerJSON`,
  `peerJSONInline`, `writeJSONEscapedString` superseded by `appendPeerJSON` /
  `appendJSONString` from `text.go`).
- `nlri/nlri.go` -- rename `nlri.JSONWriter` to `nlri.JSONAppender`; change
  method signature from `AppendJSON(sb *strings.Builder)` to
  `AppendJSON(buf []byte) []byte`.
- 9 NLRI plugin packages (18 method receivers) -- `evpn`, `flowspec`,
  `labeled`, `ls`, `mup`, `mvpn`, `rtc`, `vpls`, `vpn`. Each plugin's
  AppendJSON + per-plugin helper functions (e.g. `evpn`'s `writeHexUpper`,
  `appendRawField`, `appendLabelsNested`; `ls`'s `appendBGPLSJSON`) migrate
  in lockstep with the interface rename.
- `server/events.go` -- 5 UPDATE-path call sites migrate to stack-scratch +
  `string(scratch)` boundary; `FormatNegotiated` consumer rewires to
  `encoder.Negotiated` directly.

Out of scope:
- `format/format_buffer.go` (tracked separately as `spec-fmt-2-json-append`).
- `format/json.go` JSONEncoder methods (`encoder.Negotiated`, `encoder.Open`,
  ...) tracked separately as `spec-fmt-2-json-append`. The fmt-1 spec deletes
  `FormatNegotiated` wrapper and the events.go call site uses
  `encoder.Negotiated` directly until fmt-2 migrates the encoder.
- Plugin IPC raw-bytes transport (tracked as `spec-plugin-ipc-raw-bytes`).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- annotations carry the knowledge. -->

- [ ] `.claude/rules/buffer-first.md` -- WriteTo vs AppendText discipline.
  -> Constraint: No `make([]byte, N)` inside helpers; caller owns the buffer.
  -> Constraint: `Append*(buf []byte, ...) []byte` is the stdlib idiom for
     unbounded text output; `WriteTo(buf, off) int` stays for pool-bounded
     wire encoding.
- [ ] `.claude/rules/json-format.md` -- JSON key conventions and envelope
     shape.
  -> Constraint: ze-bgp envelope is `{"type":"bgp","bgp":{"message":{...},
     "peer":{...},"update":{...},...}}\n`. Trailing `\n` is part of the
     contract.
  -> Constraint: kebab-case for JSON keys; `formatNLRIJSONValue` must
     preserve key shape established in fmt-0.
- [ ] `.claude/rules/no-layering.md` -- replace, do not keep both.
  -> Constraint: legacy `Format*` functions deleted in the same commit that
     introduces the new `Append*` versions; no transitional aliases.
- [ ] `plan/learned/614-fmt-0-append.md` -- pattern established by fmt-0.
  -> Decision: stack-scratch + `string(scratch)` at the cache/IPC boundary;
     warm scratch path is 0 allocs/op + 1 alloc/op at the boundary.
  -> Decision: rename Format -> Append (no parallel API); `text_test.go`
     callers migrated mechanically via sed.
  -> Constraint: Preserve byte-identical output with the existing string
     form (filter plugins + CLI monitors consume the text).
  -> Constraint: AC-7 banned-call grep guards against regression -- the
     fmt-0 version covers `format/text.go`, `attribute/text.go`,
     `reactor/filter_format.go`; fmt-1 extends it to `text_update.go`,
     `text_human.go`, `text_json.go`, `summary.go`.
  -> Decision: `peer_json.go` was created in fmt-0 explicitly to be deleted
     in fmt-1; its helpers are duplicates of `appendPeerJSON` /
     `appendJSONString`.

### RFC Summaries

None -- output format is already RFC-compliant; this spec is a pure
allocation refactor. The existing RFC references in `text_json.go`
(RFC 4271, 4364, 4760, 7432, 7911, 8277, 8955) carry forward unchanged.

**Key insights:**
- fmt-0-append left `peer_json.go` deliberately so the UPDATE path could
  keep returning `string` until this spec migrated it. The file is a
  scheduled deletion target.
- The UPDATE path has more nesting than fmt-0's non-UPDATE path:
  filter-result traversal emits per-attribute per-family per-prefix tokens.
  Stack-scratch sized at `[4096]byte` covers typical UPDATE volume (2-8KB);
  pathological 32KB+ updates spill to heap via `append` growth. No pool
  introduced -- the format cache already buffers the resulting string per
  subscription key.
- `formatFullFromResult` uses `strings.HasSuffix("}}\n")` + slice surgery
  to inject `"raw":{...}` and `"route-meta":{...}` between the parsed body
  close. The Append rewrite eliminates surgery by building the parsed body
  WITHOUT its final close, then appending `,"raw":{...}` + `,"route-meta":
  {...}` + `}}\n` directly.
- `FormatSentMessage` uses `strings.Replace("type":"update", "type":"sent")`.
  The Append rewrite eliminates the surgery by threading a `messageType`
  parameter ("update" or "sent") through to the JSON writer so the type
  is correct at write time.
- `FormatNegotiated` is a one-line wrapper around `encoder.Negotiated(peer,
  neg)`. The wrapper is deleted; `server/events.go:574` calls
  `encoder.Negotiated` directly. JSONEncoder migration is fmt-2.
- `nlri.JSONWriter` interface (`AppendJSON(sb *strings.Builder)`) is
  implemented by 18 NLRI types across 9 plugin packages. Renamed to
  `nlri.JSONAppender` with signature `AppendJSON(buf []byte) []byte`. INET
  hot path does NOT use this interface (uses `prefixer.Prefix().AppendTo`)
  -- so ipv4/ipv6 unicast remains zero-alloc. Exotic families (EVPN,
  FlowSpec, VPN, MUP, RTC, MPLS-Label, BGP-LS, MVPN, VPLS) migrate
  atomically in lockstep with the interface rename.
- Map iteration order over `result.Attributes` and `familyOps` is
  non-deterministic; existing tests already tolerate this. Parity tests
  use JSON-parsed comparison for the map-containing parts; byte
  comparison for deterministic parts (peer header, attribute literal
  key/value pairs, NLRI list per family).

## Current Behavior (MANDATORY)

**Source files read:** (digests in `tmp/session/session-state-fmt-1-text-update-32940.md`)

- [ ] `internal/component/bgp/format/text_update.go` (316L) -- 9 UPDATE-path
  orchestrators returning `string`. Banned-call count: 17.
  -> Constraint: byte-identical output for received UPDATE, sent UPDATE,
     full UPDATE (parsed + raw + route-meta), summary UPDATE.
  -> Constraint: `formatFullFromResult` injection MUST place `"raw":{...}`
     before `"route-meta":{...}`, both before the closing `}}\n`.
  -> Constraint: `FormatSentMessage` must produce `"message":{"type":"sent"`
     (NOT `"update"`) in the JSON envelope.

- [ ] `internal/component/bgp/format/text_human.go` (218L) --
  `formatFilterResultText`, `formatStateChangeText`, `formatAttributesText`,
  `formatAttributeText`, `writeNLRIList`. Uses `strings.Builder`,
  `fmt.Fprintf`, `fmt.Sprintf`. Banned-call count: 8.
  -> Constraint: header line is `peer <ip> remote as <asn> <direction> update
     <id>` (with leading attribute tokens prefixed by single space).
  -> Constraint: INET NLRI list uses comma-separated CIDRs; non-INET uses
     space-separated `String()` output.
  -> Constraint: unknown attributes render as `attr-<code> <hex>` (lowercase
     hex from `hex.AppendEncode`).

- [ ] `internal/component/bgp/format/text_json.go` (367L) --
  `formatFilterResultJSON`, `formatStateChangeJSON`, `formatNLRIJSONValue`,
  `formatNLRIJSON`, `formatAttributesJSON`, `formatAttributeJSON`,
  `formatFamilyOpsJSON`. Uses `strings.Builder`, `writeJSONEscapedString`.
  Banned-call count: 6.
  -> Constraint: ze-bgp JSON envelope shape preserved verbatim; per-family
     `[{...}]` array of operations preserved.
  -> Constraint: `formatNLRIJSONValue` fast path for `prefixer` (INET) keeps
     the simple `"prefix"` string form when path-id is zero.
  -> Constraint: NLRI types implementing `JSONAppender` (post-rename) write
     directly into the buffer with no intermediate Builder.

- [ ] `internal/component/bgp/format/summary.go` (138L) -- `formatSummary`,
  `scanMPFamilies`, `buildSummaryJSON`, `formatSummaryEmpty`.
  Banned-call count: 1.
  -> Constraint: summary JSON ALWAYS includes `message.id` even when 0
     (intentional divergence from parsed/raw/full).

- [ ] `internal/component/bgp/format/peer_json.go` (139L) --
  `writePeerJSON`, `peerJSONInline`, `writeJSONEscapedString`. To be
  DELETED.
  -> Constraint: byte-identical output to `appendPeerJSON` /
     `appendJSONString` (already verified by fmt-0's parity tests).

- [ ] `internal/component/bgp/format/text.go` (262L) -- fmt-0 Append
  helpers. `AppendStateChange` currently delegates to
  `formatStateChangeText/JSON` and `append(buf, str...)`. fmt-1 closes the
  loop so `AppendStateChange` is pure-Append.
  -> Constraint: existing `Append*` signatures (`func AppendXxx(buf []byte,
     ...) []byte`) are the canonical shape for all new helpers in this
     spec.

- [ ] `internal/component/bgp/server/events.go` (787L) -- 5 UPDATE-path
  call sites: L153 (single-message receive), L264 (batch receive), L316
  (batch monitor fallback), L210/L212 (single-message monitor fallback),
  L691 + L722 (sent UPDATE cache + monitor). Plus L574 (`FormatNegotiated`
  consumer to rewire).
  -> Constraint: each migrated site uses `var scratch [N]byte` stack
     buffer where N = 4096 for UPDATE, 256 for state/eor/congestion.
  -> Constraint: `string(format.AppendMessage(scratch[:0], ...))` is the
     boundary allocation; happens exactly once per cache key.

- [ ] `internal/component/bgp/nlri/nlri.go` (160L) -- `JSONWriter`
  interface declaration. Rename to `JSONAppender`; change signature.
  -> Constraint: interface method name stays `AppendJSON` for grep
     continuity; signature becomes `AppendJSON(buf []byte) []byte`.

- [ ] `internal/component/bgp/plugins/nlri/{evpn,flowspec,labeled,ls,mup,
     mvpn,rtc,vpls,vpn}/json.go` -- 18 method receivers + per-plugin
     helpers. Migrate atomically with the interface rename.
  -> Constraint: per-plugin helper functions (`writeHexUpper`,
     `appendRawField`, `appendLabelsNested`, `appendBGPLSJSON`,
     `formatESIForJSON`, `formatMACUpper`) migrate in lockstep -- callers
     and callees in one diff per plugin.
  -> Constraint: byte-identical JSON output per NLRI type (per-plugin
     `json_test.go` parity tests must pass unchanged).

**Behavior to preserve:**
- Byte-identical output for every existing `format.FormatMessage` /
  `format.FormatSentMessage` / `format.formatStateChange*` /
  `format.formatSummary` / `format.formatFilterResult*` caller.
- All `.ci` tests in `test/plugin/` consuming UPDATE text or JSON
  subscriber output must remain byte-identical.
- All per-plugin `json_test.go` parity tests must pass byte-identical.
- The `appendJSONString`, `appendReplacingByte`, `appendPeerJSON` helpers
  (from fmt-0) remain the single source of truth for JSON escaping and
  peer JSON serialization.
- Map iteration order over `result.Attributes` and `familyOps` -- preserve
  existing non-determinism (legacy code already maps).

**Behavior to change:**
- None. Pure allocation refactor.

## Data Flow (MANDATORY -- see `rules/data-flow-tracing.md`)

### Entry Point
- Subscriber receives UPDATE: `server/events.go:OnMessageReceived` ->
  `formatMessageForSubscription` -> `format.FormatMessage` ->
  `formatFromFilterResult` -> text/JSON.
- Subscriber receives sent UPDATE: `server/events.go:onMessageSent` ->
  `format.FormatSentMessage` -> `FormatMessage` with direction "sent".
- Subscriber receives Negotiated: `server/events.go:onPeerNegotiated` ->
  `format.FormatNegotiated` -> `encoder.Negotiated`.
- CLI monitor: same path with json+parsed output.

### Transformation Path (fmt-1 target shape)
1. `OnMessageReceived` enters with `bgptypes.RawMessage` + cache key.
2. `formatMessageForSubscription` declares `var scratch [4096]byte`.
3. `format.AppendMessage(scratch[:0], peer, msg, content, override)`
   returns `[]byte`.
4. `string(scratch)` at cache write -- the named boundary allocation.
5. `format.AppendMessage` dispatches:
   - UPDATE summary -> `appendSummaryJSON`.
   - non-UPDATE -> existing `AppendOpen` / `AppendNotification` /
     `AppendKeepalive` / `AppendRouteRefresh`.
   - UPDATE parsed -> `appendFilterResultText` or `appendFilterResultJSON`
     (NEW Append-form leaf helpers).
   - UPDATE raw -> hex.AppendEncode + literal envelope.
   - UPDATE full -> append parsed body without close, append `"raw":{...}`,
     append `"route-meta":{...}`, append `}}\n`.
6. `appendFilterResultJSON` recurses into `appendFamilyOpsJSON` ->
   `appendNLRIJSONValue` -> dispatches to `nlri.JSONAppender.AppendJSON`
   for exotic families OR direct `prefixer.Prefix().AppendTo` for INET.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| format pkg -> server pkg | `[]byte` from helper, `string(scratch)` at cache | [ ] |
| format pkg -> plugin IPC | `string` in `FilterUpdateInput.Update` (cache value) | [ ] |
| format pkg -> CLI monitors | `string` via `monitorDeliver` | [ ] |
| format pkg -> nlri plugin pkgs | `nlri.JSONAppender.AppendJSON([]byte) []byte` | [ ] |

### Integration Points
- `server/events.go` UPDATE call sites (L153, L210/L212, L264, L316,
  L691, L722) migrate to stack-scratch + Append.
- `server/events.go:574` `FormatNegotiated` consumer migrates to
  `encoder.Negotiated(peer, decoded)` direct call.
- `format/text.go:AppendStateChange` (line 211) drops the
  `formatStateChange*` delegation -- becomes pure Append.
- All 18 NLRI plugin AppendJSON receivers migrate signature
  simultaneously with `nlri.JSONWriter` -> `nlri.JSONAppender` rename
  in `nlri/nlri.go`.
- Per-plugin tests (`evpn/json_test.go`, `flowspec/json_test.go`, ...)
  update calls from `var sb strings.Builder; n.AppendJSON(&sb);
  sb.String()` to `string(n.AppendJSON(nil))`.

### Architectural Verification
- [ ] No bypassed layers: format pkg remains the sole owner of UPDATE
  text/JSON encoding; plugin packages keep their JSON shape ownership.
- [ ] No unintended coupling: NLRI plugins still depend ONLY on the
  nlri interface contract.
- [ ] No duplicated functionality: deletes `peer_json.go`, deletes
  `FormatNegotiated` wrapper, deletes `strings.Replace` and
  `strings.HasSuffix` surgery.
- [ ] Zero-copy preserved: stack-scratch for [4096]byte typical;
  heap spill via append growth for pathological -- no per-event
  pool introduced.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| Plugin subscribed to UPDATE text | -> | `format.AppendMessage` + cache boundary | existing `test/plugin/*.ci` UPDATE text suite |
| Plugin subscribed to UPDATE JSON | -> | `format.AppendMessage` + cache boundary | existing `test/plugin/*.ci` UPDATE JSON suite |
| Plugin subscribed to sent UPDATE | -> | `format.AppendSentMessage` | existing `test/plugin/*.ci` sent UPDATE suite |
| Plugin subscribed to negotiated | -> | `encoder.Negotiated` (direct) | existing `test/plugin/*.ci` negotiated suite |
| CLI monitor consumes UPDATE | -> | `format.AppendMessage` (parsed/json) | existing `test/cli/*.ci` monitor suite |
| Plugin subscribed to summary UPDATE | -> | `format.AppendMessage` -> `appendSummaryJSON` | existing `test/plugin/*.ci` summary suite |
| EVPN AppendJSON migration | -> | `appendNLRIJSONValue` -> `EVPNType2.AppendJSON([]byte) []byte` | existing `internal/component/bgp/plugins/nlri/evpn/json_test.go` |
| FlowSpec AppendJSON migration | -> | `FlowSpec.AppendJSON([]byte) []byte` | existing `internal/component/bgp/plugins/nlri/flowspec/json_test.go` |
| BGP-LS AppendJSON migration | -> | `BGPLSNode.AppendJSON([]byte) []byte` | existing `internal/component/bgp/plugins/nlri/ls/json_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | UPDATE message with INET prefixes, parsed text mode | `AppendMessage(buf, peer, msg, content, "")` returns byte-identical output to legacy `FormatMessage`; `FormatMessage` deleted |
| AC-2 | UPDATE message, encoding=json, direction=sent | `AppendSentMessage(buf, peer, msg, content)` returns JSON containing `"message":{"type":"sent"` written directly (no `strings.Replace` surgery anywhere in `text_update.go`) |
| AC-3 | Negotiated capability event | `format.FormatNegotiated` is deleted; `server/events.go:574` calls `encoder.Negotiated(peer, decoded)` directly |
| AC-4 | UPDATE message, format=full, encoding=json, with route-meta | `appendFullFromResult` produces byte-identical output to legacy `formatFullFromResult` without `strings.HasSuffix("}}\n")` or slice surgery anywhere |
| AC-5 | FilterResult with attributes + announced + withdrawn families | `appendFilterResultText` returns byte-identical text output to legacy `formatFilterResultText`; legacy deleted |
| AC-6 | FilterResult, encoding=json | `appendFilterResultJSON` returns byte-identical JSON output to legacy `formatFilterResultJSON`; legacy deleted |
| AC-7 | UPDATE format=summary | `appendSummaryJSON` returns byte-identical output to legacy `buildSummaryJSON`; `formatSummary` becomes thin string boundary `string(appendSummary(scratch[:0], ...))` |
| AC-8 | grep `internal/component/bgp/format/peer_json.go` | File does not exist (`ls` returns "no such file"); all callers migrated to `appendPeerJSON` / `appendJSONString` from `text.go` |
| AC-9 | grep `internal/component/bgp/nlri/nlri.go` for `JSONWriter` | Returns 0 hits; `JSONAppender` interface declared with method `AppendJSON(buf []byte) []byte` |
| AC-10 | `make ze-unit-test ./internal/component/bgp/plugins/nlri/...` | All 9 NLRI plugin test packages pass; per-plugin parity tests confirm byte-identical JSON output |
| AC-11 | grep `var.*\[4096\]byte` in `server/events.go` | All 5 UPDATE-path call sites declare a 4096-byte stack scratch; `string(format.AppendMessage(scratch[:0], ...))` at cache write |
| AC-12 | `grep -nE 'fmt\.Sprintf\|fmt\.Fprintf\|strings\.Builder\|strings\.Replace\|strings\.NewReplacer\|strings\.ReplaceAll' internal/component/bgp/format/{text_update,text_human,text_json,summary}.go` | Returns 0 matches |
| AC-13 | `make ze-verify-fast` (180s timeout) | Exit code 0; all unit + functional + lint pass |
| AC-14 | `make ze-race-reactor` | Exit code 0; reactor concurrency stable |
| AC-15 | `BenchmarkAppendUpdate_Reused` (warm scratch, INET-only UPDATE) | Reports 0 B/op, 0 allocs/op |
| AC-16 | `BenchmarkAppendUpdate_Boundary_StringConvert` | Reports 1 alloc/op (the boundary) |
| AC-17 | `.ci` tests in `test/plugin/` UPDATE text + JSON subscriber suites | All pass byte-identical to pre-fmt-1 baseline |
| AC-18 | `plan/deferrals.md` `spec-fmt-1-text-update` entry | Status set to `done` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAppendMessage_UpdateParity_Text` | `format/text_update_append_test.go` | AC-1: byte-identical text output for received UPDATE corpus (INET, ADD-PATH, withdrawn) | |
| `TestAppendMessage_UpdateParity_JSON` | `format/text_update_append_test.go` | AC-1: byte-identical JSON output for same corpus | |
| `TestAppendSentMessage_TypeIsSent` | `format/text_update_append_test.go` | AC-2: JSON contains `"message":{"type":"sent"` -- written directly, no surgery |  |
| `TestAppendFullResult_Parity_NoSurgery` | `format/text_update_append_test.go` | AC-4: byte-identical full-format output, INCLUDING raw + route-meta nesting | |
| `TestAppendFullResult_RouteMetaShape` | `format/text_update_append_test.go` | AC-4: route-meta JSON shape preserved (string/bool values from ingress filters) | |
| `TestAppendFilterResultText_Parity` | `format/text_human_append_test.go` | AC-5: byte-identical text output across attribute/family corpus | |
| `TestAppendStateChangeText_Parity` | `format/text_human_append_test.go` | AC-5 (state path): byte-identical output for up/down with reason | |
| `TestAppendFilterResultJSON_Parity` | `format/text_json_append_test.go` | AC-6: byte-identical JSON output across attribute/family corpus | |
| `TestAppendStateChangeJSON_Parity` | `format/text_json_append_test.go` | AC-6 (state path): byte-identical state JSON | |
| `TestAppendSummary_Parity` | `format/summary_append_test.go` | AC-7: byte-identical summary JSON across all message-id and direction permutations | |
| `TestAppendNLRI_INET_FastPath` | `format/text_json_append_test.go` | INET (`prefixer`) path produces simple `"prefix"` string form; never invokes `JSONAppender` | |
| `TestNLRIAppendJSON_<Type>_Parity` | per plugin `*_test.go` | AC-10: each of 18 NLRI types produces byte-identical JSON via new signature | |
| `TestPeerJSONHelpersDeleted` | (mechanical: `go build` fails if any caller left) | AC-8: `peer_json.go` removal is complete | |
| `TestJSONWriterRenamed` | (mechanical: `go vet` fails if old name referenced) | AC-9: `nlri.JSONWriter` removed; `nlri.JSONAppender` in use | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| UPDATE wire size | 19 - 65535 | 65535 | 18 | 65536 |
| Stack scratch | [4096]byte | 4096B in stack | N/A (smaller fits) | 4097B (heap spill via append) |
| msgID | uint64 | 18446744073709551615 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing UPDATE text suite | `test/plugin/*.ci` matching update.*text | AC-17 byte-identical | |
| Existing UPDATE JSON suite | `test/plugin/*.ci` matching update.*json | AC-17 byte-identical | |
| Existing sent UPDATE suite | `test/plugin/*.ci` matching sent | AC-17 byte-identical | |
| Existing negotiated suite | `test/plugin/*.ci` matching negotiated | AC-17 byte-identical | |
| Existing CLI monitor suite | `test/cli/*.ci` matching monitor | AC-17 byte-identical | |
| Existing summary suite | `test/plugin/*.ci` matching summary | AC-17 byte-identical | |

### Benchmark Tests
| Test | File | Validates |
|------|------|-----------|
| `BenchmarkAppendUpdate_Reused` | `format/text_update_append_bench_test.go` | AC-15 -- 0 allocs/op on warm scratch (INET corpus) |
| `BenchmarkAppendUpdate_Boundary_StringConvert` | `format/text_update_append_bench_test.go` | AC-16 -- 1 alloc/op at boundary |
| `BenchmarkAppendUpdate_FullPath` | `format/text_update_append_bench_test.go` | full-format with route-meta -- track allocs |
| `BenchmarkAppendNLRI_EVPNType2` | `nlri/evpn/json_bench_test.go` | exotic NLRI 0 allocs/op on warm buffer |

### Future (deferred tests)
- None. All in-scope behavior covered by parity + benchmark tests above.

## Files to Modify

- `internal/component/bgp/format/text_update.go` -- rewrite 9 functions to
  Append shape; eliminate `strings.Replace`, `strings.HasSuffix` surgery,
  `fmt.Sprintf("%x")`, `strings.Builder`, `fmt.Fprintf`. Delete
  `FormatNegotiated`. Rename `FormatMessage` -> `AppendMessage`,
  `FormatSentMessage` -> `AppendSentMessage` (per no-layering rule).
- `internal/component/bgp/format/text_human.go` -- rewrite to Append shape;
  eliminate `strings.Builder` and `fmt.Fprintf`. Use `hex.AppendEncode` for
  unknown attributes (drop `make([]byte, attr.Len())`).
- `internal/component/bgp/format/text_json.go` -- rewrite to Append shape;
  eliminate `strings.Builder` and `writeJSONEscapedString` calls (use
  `appendJSONString` from `text.go`). Update `appendNLRIJSONValue` to call
  `nlri.JSONAppender` interface.
- `internal/component/bgp/format/summary.go` -- rewrite `buildSummaryJSON`
  to Append shape; `formatSummary` becomes thin string boundary.
- `internal/component/bgp/format/text.go` -- rewrite `AppendStateChange`
  to call `appendStateChangeText` / `appendStateChangeJSON` directly (not
  via `append(buf, str...)` of legacy string output).
- `internal/component/bgp/nlri/nlri.go` -- rename `JSONWriter` interface
  to `JSONAppender`; change `AppendJSON` signature to
  `AppendJSON(buf []byte) []byte`.
- `internal/component/bgp/plugins/nlri/evpn/json.go` -- 6 receivers
  (Type1..Type5, Generic) + helpers (`writeHexUpper`, `appendRawField`,
  `appendLabelsNested`).
- `internal/component/bgp/plugins/nlri/flowspec/json.go` -- 2 receivers
  (FlowSpec, FlowSpecVPN).
- `internal/component/bgp/plugins/nlri/labeled/json.go` -- 1 receiver
  (LabeledUnicast).
- `internal/component/bgp/plugins/nlri/ls/json.go` -- 4 receivers
  (BGPLSNode, BGPLSLink, BGPLSPrefix, BGPLSSRv6SID) + `appendBGPLSJSON`
  helper.
- `internal/component/bgp/plugins/nlri/mup/json.go` -- 1 receiver (MUP).
- `internal/component/bgp/plugins/nlri/mvpn/json.go` -- 1 receiver (MVPN).
- `internal/component/bgp/plugins/nlri/rtc/json.go` -- 1 receiver (RTC).
- `internal/component/bgp/plugins/nlri/vpls/json.go` -- 1 receiver (VPLS).
- `internal/component/bgp/plugins/nlri/vpn/json.go` -- 1 receiver (VPN).
- `internal/component/bgp/server/events.go` -- 5 UPDATE-path call sites
  migrate to stack-scratch + boundary; rewire `FormatNegotiated`
  consumer (L574) to `encoder.Negotiated`.
- `internal/component/bgp/format/text_test.go` -- mechanical sed:
  `FormatMessage(...)` -> `string(AppendMessage(nil, ...))`.
- `internal/component/bgp/format/json_test.go` -- mechanical sed
  (same pattern).
- `internal/component/bgp/format/summary_test.go` -- update if
  `formatSummary` signature changes.
- `internal/component/bgp/format/text_append_test.go` -- inline a
  `legacyEscapeJSONForTest` so parity tests still compile after
  `peer_json.go` deletion.
- Per-plugin `internal/component/bgp/plugins/nlri/<pkg>/json_test.go`
  -- update each call from `var sb strings.Builder; n.AppendJSON(&sb);
  sb.String()` to `string(n.AppendJSON(nil))`.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] No | n/a (pure refactor) |
| CLI commands | [ ] No | n/a |
| Editor autocomplete | [ ] No | n/a |
| Functional test for new RPC/API | [ ] No | existing `.ci` coverage |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] No | n/a |
| 2 | Config syntax changed? | [ ] No | n/a |
| 3 | CLI command added/changed? | [ ] No | n/a |
| 4 | API/RPC added/changed? | [ ] No | n/a (output is byte-identical) |
| 5 | Plugin added/changed? | [ ] No | n/a (only signature change is internal `nlri.JSONWriter` rename, not plugin SDK) |
| 6 | Has a user guide page? | [ ] No | n/a |
| 7 | Wire format changed? | [ ] No | n/a |
| 8 | Plugin SDK/protocol changed? | [ ] No -- but document the interface rename | `.claude/rules/plugin-design.md` if SDK references `nlri.JSONWriter` |
| 9 | RFC behavior implemented? | [ ] No | n/a |
| 10 | Test infrastructure changed? | [ ] No | n/a |
| 11 | Affects daemon comparison? | [ ] No | n/a |
| 12 | Internal architecture changed? | [ ] Yes -- update `peer_json.go` references in `// Related:` of `format/text.go`, `format/text_update.go`, `format/text_json.go`, `format/summary.go` after deletion. | source files (cross-ref maintenance) |

## Files to Create

- `internal/component/bgp/format/text_update_append_test.go` -- parity
  tests for `AppendMessage` / `AppendSentMessage` / `appendFullFromResult`
  (AC-1, AC-2, AC-4).
- `internal/component/bgp/format/text_update_append_bench_test.go` --
  benchmarks for AC-15, AC-16.
- `internal/component/bgp/format/text_human_append_test.go` -- parity
  tests for `appendFilterResultText` / `appendStateChangeText` (AC-5).
- `internal/component/bgp/format/text_json_append_test.go` -- parity
  tests for `appendFilterResultJSON` / `appendStateChangeJSON` /
  `appendNLRIJSONValue` INET fast path (AC-6, AC-10 fast path).
- `internal/component/bgp/format/summary_append_test.go` -- parity
  tests for `appendSummaryJSON` (AC-7).

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation Phases below (write-test-fail-implement-pass per phase) |
| 4. /ze-review gate | Review Gate section -- run `/ze-review`; fix every BLOCKER/ISSUE |
| 5. Full verification | `make ze-verify-fast && make ze-race-reactor` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase 1: nlri interface rename** -- rename `nlri.JSONWriter` to
   `nlri.JSONAppender`; change method signature in `nlri/nlri.go`.
   Migrate all 18 implementations in 9 plugin packages atomically.
   Update per-plugin `json_test.go` to new call shape. NO format/ changes
   in this phase -- format/text_json.go is broken between phases (only
   the call sites that fan into JSONAppender). Verify each plugin in
   isolation: `go test ./internal/component/bgp/plugins/nlri/<pkg>/...`.
   - Tests: per-plugin `TestNLRIAppendJSON_<Type>_Parity`.
   - Files: `nlri/nlri.go`, 9 `plugins/nlri/<pkg>/json.go`, 9 `plugins/nlri/<pkg>/json_test.go`.
   - Verify: `go test ./internal/component/bgp/plugins/nlri/...` per plugin in turn.

2. **Phase 2: appendNLRI dispatch in text_json.go** -- update
   `formatNLRIJSONValue` to call new `JSONAppender` signature; this is
   the bridge between Phase 1 and Phase 3. Format/ still uses
   strings.Builder for the outer envelope; the NLRI inner write goes
   through Append form. Verify `format/json_test.go` still passes.
   - Tests: existing `format/json_test.go`.
   - Files: `format/text_json.go` (only `formatNLRIJSONValue` and
     `formatNLRIJSON`).
   - Verify: `go test ./internal/component/bgp/format/...`.

3. **Phase 3: appendStateChange + AppendStateChange closure** --
   migrate `formatStateChangeText` -> `appendStateChangeText`,
   `formatStateChangeJSON` -> `appendStateChangeJSON`. Update
   `text.go:AppendStateChange` to call them directly (drop
   `append(buf, str...)`). Delete legacy.
   - Tests: `format/text_human_append_test.go` `TestAppendStateChangeText_Parity`,
     `format/text_json_append_test.go` `TestAppendStateChangeJSON_Parity`.
   - Files: `format/text_human.go`, `format/text_json.go`, `format/text.go`.
   - Verify: `go test ./internal/component/bgp/format/...`.

4. **Phase 4: appendFilterResultText + appendFilterResultJSON** --
   migrate the parsed-UPDATE leaf renderers to Append shape. Verify
   parity vs legacy. Legacy versions stay temporarily callable as
   `legacyFormatFilterResult{Text,JSON}` (in test file only) to
   support parity assertion -- deleted at end of phase.
   - Tests: `format/text_human_append_test.go`
     `TestAppendFilterResultText_Parity`, `format/text_json_append_test.go`
     `TestAppendFilterResultJSON_Parity`.
   - Files: `format/text_human.go`, `format/text_json.go`.
   - Verify: parity tests; `make ze-unit-test` for format/ pkg.

5. **Phase 5: appendSummaryJSON** -- migrate `buildSummaryJSON` ->
   `appendSummaryJSON`; rewrite `formatSummary` as thin string boundary.
   - Tests: `format/summary_append_test.go` `TestAppendSummary_Parity`.
   - Files: `format/summary.go`, `format/summary_test.go`.
   - Verify: parity tests.

6. **Phase 6: appendFullFromResult (no surgery)** -- rewrite the
   full-format orchestrator to build the parsed body without final
   close, then append `,"raw":{...}` + `,"route-meta":{...}` + `}}\n`
   directly. Drop `strings.HasSuffix` and slice surgery.
   - Tests: `format/text_update_append_test.go`
     `TestAppendFullResult_Parity_NoSurgery`,
     `TestAppendFullResult_RouteMetaShape`.
   - Files: `format/text_update.go`.
   - Verify: parity tests.

7. **Phase 7: AppendMessage + AppendSentMessage** -- rename
   `FormatMessage` -> `AppendMessage`; `FormatSentMessage` ->
   `AppendSentMessage`. Thread `messageType` parameter ("update" /
   "sent") through to JSON writers so `strings.Replace` surgery
   disappears. Delete `FormatNegotiated` wrapper.
   - Tests: `format/text_update_append_test.go`
     `TestAppendMessage_UpdateParity_*`, `TestAppendSentMessage_TypeIsSent`.
   - Files: `format/text_update.go`.
   - Verify: parity tests.

8. **Phase 8: server/events.go call sites** -- migrate 5 UPDATE-path
   call sites to `var scratch [4096]byte` + `string(format.AppendMessage(
   scratch[:0], ...))`. Rewire L574 `FormatNegotiated` consumer to
   `encoder.Negotiated`.
   - Tests: existing `server/events_test.go` (no signature changes
     visible from caller).
   - Files: `server/events.go`.
   - Verify: `go test ./internal/component/bgp/server/...`.

9. **Phase 9: delete peer_json.go** -- remove the file. Inline
   `legacyEscapeJSONForTest` in `text_append_test.go` for the existing
   parity tests. Verify build.
   - Tests: `format/text_append_test.go` (still passes via inlined helper).
   - Files: DELETE `format/peer_json.go`. EDIT `format/text_append_test.go`.
   - Verify: `go build ./...`.

10. **Phase 10: test caller migration** -- mechanical sed on
    `format/text_test.go` and `format/json_test.go` for
    `FormatMessage(&peer, ...)` -> `string(AppendMessage(nil, &peer, ...))`
    and equivalent `FormatSentMessage` migration.
    - Tests: existing tests (now via new signature).
    - Files: `format/text_test.go`, `format/json_test.go`.
    - Verify: `go test ./internal/component/bgp/format/...`.

11. **Phase 11: benchmarks** -- write
    `format/text_update_append_bench_test.go` and
    `nlri/evpn/json_bench_test.go`. Verify AC-15 (0 allocs warm) and
    AC-16 (1 alloc boundary).
    - Tests: see Files to Create.
    - Files: see Files to Create.
    - Verify: `go test -bench=BenchmarkAppendUpdate -benchmem ./...`.

12. **Phase 12: cross-ref maintenance** -- update `// Related:`
    annotations in `format/text.go`, `format/text_update.go`,
    `format/text_json.go`, `format/summary.go` to drop `peer_json.go`
    references. Verify `require-related-refs.sh` hook passes.
    - Files: each format/ source file.

13. **Functional tests** -- run `bin/ze-test bgp plugin --all` to
    confirm AC-17 (existing `.ci` suites byte-identical).

14. **Full verification** -- `make ze-verify-fast`; `make ze-race-reactor`.

15. **Complete spec** -- fill audit tables, write learned summary
    `plan/learned/NNN-fmt-1-text-update.md`, delete spec from `plan/`.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-18 has implementation with file:line |
| Correctness | Byte-identical UPDATE text + JSON for received, sent, full, summary -- compare against legacy via parity tests |
| Naming | `AppendXxx(buf []byte, ...) []byte` shape uniformly; method name `AppendJSON` retained on `nlri.JSONAppender` for grep continuity |
| Data flow | Format pkg owns text/JSON encoding; NLRI plugins own per-type JSON shape; nothing bypasses interface |
| Rule: no-layering | `FormatMessage`, `FormatSentMessage`, `FormatNegotiated`, `formatStateChange*`, `formatFilterResult*`, `buildSummaryJSON`, `peerJSONInline`, `writePeerJSON`, `writeJSONEscapedString`, `nlri.JSONWriter` ALL deleted -- no transitional aliases |
| Rule: buffer-first | Append shape used; no `make([]byte, N)` in helpers; stack scratch + heap spill on overflow |
| Rule: lazy-over-eager | `appendNLRIJSONValue` keeps the `prefixer` fast path (zero-alloc for INET) and the per-type `JSONAppender` dispatch (no intermediate map) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `format/text_update.go` -- AppendMessage etc. | `grep -n "func AppendMessage" internal/component/bgp/format/text_update.go` returns 1 hit |
| `format/text_update.go` -- legacy deleted | `grep -n "func FormatMessage\|func FormatSentMessage\|func FormatNegotiated" internal/component/bgp/format/text_update.go` returns 0 hits |
| `format/peer_json.go` -- deleted | `ls internal/component/bgp/format/peer_json.go` returns "no such file" |
| `nlri.JSONAppender` -- declared | `grep -n "type JSONAppender" internal/component/bgp/nlri/nlri.go` returns 1 hit |
| `nlri.JSONWriter` -- removed | `grep -rn "JSONWriter" internal/component/bgp/nlri internal/component/bgp/plugins/nlri internal/component/bgp/format` returns 0 hits |
| All 18 NLRI AppendJSON migrated | `grep -rn "AppendJSON(sb \*strings.Builder)" internal/` returns 0 hits |
| Banned-call grep | `grep -nE 'fmt\.Sprintf\|fmt\.Fprintf\|strings\.Builder\|strings\.Replace\|strings\.NewReplacer\|strings\.ReplaceAll' internal/component/bgp/format/{text_update,text_human,text_json,summary}.go` returns 0 matches |
| 5 server/events.go scratch sites | `grep -nB2 'AppendMessage\|AppendSentMessage' internal/component/bgp/server/events.go` shows `var scratch` declaration above each |
| Benchmarks pass | `go test -bench=BenchmarkAppendUpdate -benchmem ./internal/component/bgp/format/...` shows 0 allocs/op for `_Reused`, 1 alloc/op for `_Boundary_StringConvert` |
| Parity tests pass | `go test -run TestAppend ./internal/component/bgp/format/... ./internal/component/bgp/plugins/nlri/...` PASS |
| Functional tests pass | `bin/ze-test bgp plugin --all` PASS |
| Verify pass | `make ze-verify-fast` exit 0; `make ze-race-reactor` exit 0 |
| Deferral closed | `grep "spec-fmt-1-text-update" plan/deferrals.md` shows status=done |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | All inputs come from already-parsed BGP wire data (nlri.NLRI, attribute.Attribute, FilterResult); no new external surface introduced |
| JSON escaping | `appendJSONString` (existing) handles control chars 0x00-0x1F and `\ "` -- pathological peer names cannot break JSON |
| Buffer overflow | Stack scratch [4096]byte + heap spill via append; no fixed-size buffers used for variable input |
| Resource exhaustion | Pathological 32KB+ UPDATE causes one heap alloc -- bounded by message.MaxLen (4096 standard, 65535 extended) |
| Information leak | No new logging or output paths added; format output unchanged |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error in plugin pkg | Phase 1 -- finish all 18 NLRI receivers before moving to Phase 2 |
| Phase 2 build fails | Confirm Phase 1 complete; check `formatNLRIJSONValue` dispatch |
| Parity test fails (byte mismatch) | Re-read source from Current Behavior; print diff between legacy and Append output |
| Map iteration order test fails | Replace byte cmp with JSON-parsed cmp; the spec preserves non-determinism |
| Benchmark shows alloc | Check encoding context registration (per fmt-0 gotcha); else check stack scratch path |
| `make ze-race-reactor` fails | Format pkg has no shared state -- failure is unrelated, fix before proceeding |
| `.ci` test fails | Confirm parity tests pass first; if `.ci` mismatches but unit parity passes, the fixture is stale (regen) |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something during /implement -->

## RFC Documentation

No new RFC enforcement code. Existing RFC references in `text_json.go`
(RFC 4271, 4364, 4760, 7432, 7911, 8277, 8955) carry forward unchanged.

## Implementation Summary

### What Was Implemented
- (filled during /implement)

### Bugs Found/Fixed
- (filled during /implement)

### Documentation Updates
- (filled during /implement)

### Deviations from Plan
- (filled during /implement)

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-18 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-verify-fast` passes
- [ ] `make ze-race-reactor` passes
- [ ] Feature code integrated (format pkg + NLRI plugins + server/events.go)
- [ ] Architecture cross-refs (`// Related:`) updated for `peer_json.go` deletion
- [ ] Critical Review passes (all 6 checks in `rules/quality.md`)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Benchmarks show zero-alloc `Append*` path on warm scratch (AC-15)
- [ ] Boundary alloc benchmark shows 1 alloc/op (AC-16)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction -- all helpers have at least 2 callers
- [ ] No speculative features -- pure refactor, no behavior changes
- [ ] Single responsibility per helper
- [ ] Explicit > implicit -- `messageType` parameter replaces `strings.Replace` surgery; structured field appends replace `strings.HasSuffix` surgery
- [ ] Minimal coupling -- format pkg surface unchanged from caller PoV (just signature shape)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for stack scratch overflow
- [ ] Functional tests for end-to-end byte-identical output

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Pre-Commit Verification filled (independent re-verification, not copy from audit)
- [ ] Write learned summary to `plan/learned/NNN-fmt-1-text-update.md`
- [ ] Deferral entry in `plan/deferrals.md` closed (status -> done)
- [ ] Summary included in commit -- TWO commits per `rules/spec-preservation.md`:
      Commit A = code + tests + completed spec; Commit B = `git rm` spec + add learned summary.

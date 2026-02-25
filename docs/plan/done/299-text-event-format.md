# Spec: text-event-format

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugins/bgp/server/events.go` — ALL event delivery functions hardcode JSON (formatMessageForSubscription, onPeerStateChange, onMessageSent); pre-format cache keyed by format only
4. `internal/plugins/bgp/format/text.go` — formatFilterResultText (line 627), formatFromFilterResult (line 127, missing FormatHex), text formatters for all types (line 780+)
5. `internal/plugins/bgp-rr/server.go` — quickParseEvent (line 488), dispatch (line 534), forwardCtx (line 77), parseUpdateFamilies (line 1067), worker processing (line 420+)

## Task

Two changes to eliminate JSON serialization/deserialization overhead on the event delivery hot path:

1. **Fix FormatHex/FormatRaw mismatch** — `bgp-rr` gets fully-parsed JSON instead of hex format because `"hex" != "raw"` in the format switch. This is a bug.

2. **Allow plugins to select text encoding for events** — `formatMessageForSubscription` hardcodes `EncodingJSON`. Plugins should be able to request text encoding, which uses `strings.Builder` output parseable with `strings.Fields()` instead of nested JSON requiring `json.Unmarshal`.

For the `bgp-rr` hot path specifically, the plugin receives ~295 bytes of nested JSON, performs 6+ `json.Unmarshal` calls per UPDATE: 2 in `quickParseEvent` (envelope + payload), 3 in `parseUpdateFamilies` (payload → update → nlri map), and 1+ in `parseNLRIFamilyOps` (per family). A text format gives the same information parseable by `strings.Fields` instead of nested JSON unmarshaling.

## Required Reading

### Architecture Docs
- [x] `docs/architecture/core-design.md` — event delivery, plugin communication
  → Decision: events formatted per-process based on process format + encoding settings
  → Constraint: `Process.Format()` controls content (hex/parsed/full/summary), encoding controls serialization
- [x] `docs/architecture/api/json-format.md` — ze-bgp JSON format
  → Constraint: JSON format must remain available and unchanged for external plugins

### Related Completed Specs
- [x] `docs/plan/done/270-subscription-summary-format.md` — summary format for lightweight events
  → Decision: summary format short-circuits before full attribute parsing
  → Constraint: format selection already exists, just add text encoding path
- [x] `docs/plan/done/294-inprocess-direct-transport.md` — DirectBridge
  → Constraint: DirectBridge passes formatted strings — text format works with no bridge changes

**Key insights:**
- Text formatters already exist for ALL event types: `formatFilterResultText` (UPDATE), `FormatOpen`, `FormatNotification`, `FormatKeepalive`, `FormatRouteRefresh`, `FormatStateChange` — all implemented, tested, just not wired to event delivery
- ALL event delivery functions hardcode JSON: `formatMessageForSubscription` (UPDATE/OPEN/etc), `onPeerStateChange`, `onMessageSent`
- `Process` already has `Encoding()` and `SetEncoding()` methods (process.go:260-272)
- DirectBridge delivers `[]string` — format-agnostic, works with text
- `bgp-rr` uses `quickParseEvent` doing 2 `json.Unmarshal` just to get type+msgID+peer, then `parseUpdateFamilies` adds 3 more, then `parseNLRIFamilyOps` adds 1+ per family — 6+ total unmarshals per UPDATE
- `FormatHex` ("hex") vs `FormatRaw` ("raw") mismatch means default format falls through to parsed
- Pre-format cache in events.go is keyed by `proc.Format()` only — must include encoding when encoding varies per process

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/plugins/bgp/server/events.go:166-182` — formatMessageForSubscription hardcodes `Encoding: plugin.EncodingJSON`
- [x] `internal/plugins/bgp/format/text.go:27-76` — FormatMessage dispatches on Encoding
- [x] `internal/plugins/bgp/format/text.go:127-135` — formatFromFilterResult switches on FormatRaw/FormatFull/default(FormatParsed); FormatHex not matched
- [x] `internal/plugins/bgp/format/text.go:627-680` — formatFilterResultText exists, outputs space-delimited format
- [x] `internal/plugin/process.go:260-283` — Encoding() defaults to "json", Format() defaults to "hex"
- [x] `internal/plugin/types.go:284-292` — FormatHex="hex", FormatRaw="raw" (different constants, alias in comment only)
- [x] `internal/plugins/bgp-rr/server.go:235` — SetStartupSubscriptions with empty format
- [x] `internal/plugins/bgp-rr/server.go:488-560` — quickParseEvent: 2x json.Unmarshal per event
- [x] `internal/plugins/bgp-rr/server.go:1058-1108` — parseUpdateFamilies: 3x json.Unmarshal per UPDATE
- [x] `internal/plugins/bgp/types/contentconfig.go` — WithDefaults: encoding defaults to "text", format to "parsed"
- [x] `internal/plugins/bgp/server/events.go:219-234` — onPeerStateChange hardcodes `plugin.EncodingJSON`
- [x] `internal/plugins/bgp/server/events.go:265-314` — onMessageSent hardcodes `plugin.EncodingJSON` for UPDATEs
- [x] `internal/plugins/bgp/format/text.go:780-848` — Text formatters exist for OPEN, NOTIFICATION, KEEPALIVE, ROUTEREFRESH, state change

**Behavior to preserve:**
- JSON encoding remains default for external plugins and CLI consumers
- `FormatSummary` short-circuit path unchanged
- All existing JSON format tests pass
- `bgp-rib` and `bgp-adj-rib-in` continue using "full" format (they need raw wire bytes)
- SDK `SetStartupSubscriptions` signature unchanged
- DirectBridge delivery path unchanged (already format-agnostic)
- `onPeerNegotiated` can stay JSON-only (infrequent, no plugin opts into text for it yet)

**Behavior to change:**
- `formatMessageForSubscription` should use `proc.Encoding()` instead of hardcoding JSON
- `onPeerStateChange` should use `proc.Encoding()` instead of hardcoding JSON
- `onMessageSent` should use `proc.Encoding()` instead of hardcoding JSON
- Pre-format cache keys in `onMessageReceived`, `onMessageBatchReceived`, `onMessageSent` must include encoding (not just format)
- `FormatHex` should be handled in `formatFromFilterResult` (either as alias for raw, or distinct format)
- `bgp-rr` should request text encoding via `SetEncoding("text")` before `Run()`
- `bgp-rr` should replace `quickParseEvent` + JSON parsing with `strings.Fields`-based text parsing
- `bgp-rr` `forwardCtx.bgpPayload` (currently inner JSON object) should store text line(s) for deferred worker parsing

## Data Flow (MANDATORY)

### Entry Point
- BGP UPDATE received on wire → event delivery → `formatMessageForSubscription(encoder, peer, msg, fmtMode)`
- Currently: `Encoding: plugin.EncodingJSON` hardcoded → always JSON output
- Proposed: `Encoding: proc.Encoding()` → JSON or text based on plugin preference

### Transformation Path (current — JSON for all plugins)
1. `formatMessageForSubscription` builds `ContentConfig{Encoding: "json", Format: proc.Format()}`
2. `FormatMessage` → `filter.ApplyToUpdate` → `formatFromFilterResult` → `formatFilterResultJSON`
3. JSON string → delivered to plugin → `json.Unmarshal` × 6+ (2 dispatch + 3 families + 1+ per-family NLRIs)

### Transformation Path (proposed — text for bgp-rr)
1. `formatMessageForSubscription` builds `ContentConfig{Encoding: proc.Encoding(), Format: proc.Format()}`
2. `FormatMessage` → `filter.ApplyToUpdate` → `formatFromFilterResult` → `formatFilterResultText`
3. Text string → delivered to plugin → `strings.Fields` + index-based extraction

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine → Plugin | Formatted string (text or JSON) over DirectBridge or socket | [ ] |
| Format selection | `proc.Encoding()` + `proc.Format()` | [ ] |

### Integration Points
- `events.go:formatMessageForSubscription` — accept encoding parameter, pass to ContentConfig
- `events.go:onMessageReceived` — cache key must be format+encoding (not format alone)
- `events.go:onMessageBatchReceived` — same cache key fix + pass proc encoding
- `events.go:onMessageSent` — same cache key fix + pass proc encoding
- `events.go:onPeerStateChange` — use proc encoding instead of hardcoded JSON (per-process formatting)
- `text.go:formatFromFilterResult` — handle FormatHex as alias for FormatRaw
- `bgp-rr/server.go` — call `SetEncoding("text")` before `Run()`, replace JSON parsing with text parsing
- `bgp-rr/server.go:forwardCtx` — store raw text instead of inner JSON object, update worker parsing

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (reuses existing formatFilterResultText)
- [ ] Zero-copy preserved where applicable

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Format switch with `FormatHex` | Treated same as `FormatRaw` (raw hex output), not fall-through to parsed |
| AC-2 | Plugin with `Encoding: "text"` subscribes to UPDATE events | Receives text-formatted events, not JSON |
| AC-3 | Plugin with `Encoding: "json"` (default for external) | Receives JSON-formatted events (no regression) |
| AC-4 | `bgp-rr` event delivery | Text format events parseable by `strings.Fields` |
| AC-5 | Text UPDATE event | Contains: peer address, direction, message ID, families, action, NLRIs |
| AC-6 | Text OPEN event | Contains: peer address, direction, ASN, router-id, hold-time, capabilities |
| AC-7 | Text state event | Contains: peer address, state string |
| AC-8 | `bgp-rib` with "full" format + JSON encoding (default) | Still receives JSON full format (unchanged) |
| AC-9 | `make ze-verify` passes | All existing tests pass with no regressions |
| AC-10 | FormatHex in formatFromFilterResult | Produces raw hex output, not parsed JSON |
| AC-11 | Two procs: same format, different encoding | Pre-format cache produces distinct outputs (no collision) |
| AC-12 | `onPeerStateChange` with text-encoded process | Delivers text state event, not JSON |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFormatHexMatchesRaw` | `internal/plugins/bgp/format/text_test.go` | FormatHex produces same output as FormatRaw (AC-1, AC-10) | |
| `TestFormatMessageTextEncoding` | `internal/plugins/bgp/format/text_test.go` | Text encoding produces text output, not JSON (AC-2) | |
| `TestFormatMessageJSONEncodingUnchanged` | `internal/plugins/bgp/format/text_test.go` | JSON encoding still produces JSON (AC-3) | |
| `TestFormatSubscriptionRespectsEncoding` | `internal/plugins/bgp/server/events_test.go` | formatMessageForSubscription uses proc encoding (AC-2) | |
| `TestCacheKeyIncludesEncoding` | `internal/plugins/bgp/server/events_test.go` | Two procs with same format but different encoding get different formatted output (AC-2, AC-3) | |
| `TestStateChangeRespectsEncoding` | `internal/plugins/bgp/server/events_test.go` | onPeerStateChange formats per-process encoding (AC-7) | |
| `TestTextUpdateParseableByFields` | `internal/plugins/bgp-rr/server_test.go` | Text UPDATE event → strings.Fields → correct peer, msgID, families, NLRIs (AC-4, AC-5) | |
| `TestTextStateEventParseable` | `internal/plugins/bgp-rr/server_test.go` | Text state event → fields yield peer address + state (AC-7) | |
| `TestTextOpenEventParseable` | `internal/plugins/bgp-rr/server_test.go` | Text OPEN event → fields yield ASN, router-id, families (AC-6) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | No new numeric inputs | — | — | — |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing route reflection tests | `test/plugin/` | Route reflection works end-to-end with text events | |

### Future (if deferring any tests)
- Benchmark comparing JSON vs text event throughput — deferred to profiling phase

## Files to Modify

### Part 1: Fix FormatHex/FormatRaw Mismatch
- `internal/plugins/bgp/format/text.go` — add `case plugin.FormatHex:` alongside `plugin.FormatRaw` in formatFromFilterResult

### Part 2: Text Encoding for Event Delivery
- `internal/plugins/bgp/server/events.go` — `formatMessageForSubscription`: accept encoding parameter, route non-UPDATE types through text formatters when encoding is text (instead of through JSONEncoder)
- `internal/plugins/bgp/server/events.go` — `onMessageReceived`, `onMessageBatchReceived`, `onMessageSent`: change pre-format cache key from `proc.Format()` to `proc.Format()+proc.Encoding()`, pass encoding to `formatMessageForSubscription`
- `internal/plugins/bgp/server/events.go` — `onPeerStateChange`: format per-process (like messages) instead of once with hardcoded JSON; use `proc.Encoding()`

### Part 3: bgp-rr Text Parsing
- `internal/plugins/bgp-rr/server.go` — call `p.SetEncoding("text")` before `Run()`; replace `quickParseEvent` with `strings.Fields`-based text dispatch; replace `parseUpdateFamilies`/`parseNLRIFamilyOps` with text-based family+NLRI extraction; change `forwardCtx.bgpPayload` to store raw text; replace `parseEvent` (non-UPDATE types) with text field parsing

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | No | N/A |
| API commands doc | No | N/A |
| Plugin SDK docs | No | SDK format documentation if any |
| Functional test for new RPC/API | No | N/A |

## Files to Create
- None — all changes are modifications to existing files

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

### Phase 1: Fix FormatHex/FormatRaw Mismatch

1. **Write `TestFormatHexMatchesRaw`** → verify FormatHex currently produces different output than FormatRaw
2. **Run test** → MUST FAIL (FormatHex falls through to parsed)
3. **Add `case plugin.FormatHex:` in formatFromFilterResult** alongside FormatRaw
4. **Run test** → MUST PASS
5. **Run `make ze-verify`** → check no regressions

### Phase 2: Text Encoding for Event Delivery

6. **Write `TestFormatSubscriptionRespectsEncoding`** → verify encoding parameter used
7. **Run test** → MUST FAIL
8. **Update `formatMessageForSubscription`** — accept encoding parameter; for text encoding + non-UPDATE types, call text formatters directly instead of JSONEncoder
9. **Update cache keys** in `onMessageReceived`, `onMessageBatchReceived`, `onMessageSent` — key by format+encoding, not format alone
10. **Update callers** — pass `proc.Encoding()` to `formatMessageForSubscription`
11. **Update `onPeerStateChange`** — format per-process using `proc.Encoding()` instead of once with hardcoded JSON
12. **Run test** → MUST PASS
13. **Write `TestFormatMessageTextEncoding`** → text encoding produces text output
14. **Run test** → MUST PASS (text formatters already work)

### Phase 3: bgp-rr Text Parsing

15. **Write `TestTextEventParseableByFields`** → text UPDATE event → `strings.Fields` → correct peer, msgID, families, NLRIs
16. **Write `TestTextStateEventParseable`** → text state event → fields[3]="state", fields[4]=state-value
17. **Write `TestTextOpenEventParseable`** → text OPEN event → fields extract ASN, router-id, families
18. **Run tests** → MUST FAIL
19. **Update bgp-rr** — call `p.SetEncoding("text")` before `Run()`; replace `quickParseEvent` with `strings.Fields`-based dispatch; replace `parseUpdateFamilies`/`parseNLRIFamilyOps` with text scanning; change `forwardCtx.bgpPayload` to store raw text line(s); replace `parseEvent` for non-UPDATE with text field parsing
20. **Run tests** → MUST PASS
21. **Run `make ze-verify`** → all pass
22. **Critical Review** → all 6 quality checks

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix syntax/types |
| Test fails wrong reason | Fix test |
| bgp-rr functional tests fail | Check text parsing extracts all needed fields; check multi-line UPDATE handling |
| Other plugins break | Verify encoding defaults to JSON for them (Process.Encoding() defaults to "json") |
| FormatHex change breaks hex consumers | Check what hex format is supposed to produce |
| Cache key collision (same format, different encoding) | Verify cache key includes both format and encoding |
| State event wrong format | Verify onPeerStateChange uses per-process encoding |
| forwardCtx text storage issue | Verify multi-line text (announce+withdraw) stored and parsed correctly in worker |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Adding FormatHex to FormatRaw switch case is safe | Process.Format() defaults to "hex" — changing hex behavior affects ALL default-format processes | Functional test check.ci failed (plugin received raw hex instead of parsed JSON) | Changed Process.Format() default from FormatHex to FormatParsed |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Hidden coupling between default values and switch fall-through | First time | Consider adding to design-principles.md: "When fixing a switch fall-through, check what callers rely on the old behavior" | Document in memory |

## Design Insights

### FormatHex vs FormatRaw: Bug or Design?

`FormatHex = "hex"` and `FormatRaw = "raw"` are separate constants with a comment saying "alias". But the format switch only handles `FormatRaw`, so `FormatHex` falls through to `FormatParsed`. This means the "default" format (hex) actually produces parsed output — the most expensive path. Fix: add `FormatHex` case in the switch, or make them the same constant.

### Why Text Not a New Custom Format?

Text formatters already exist for every event type bgp-rr consumes:

| Event Type | Formatter | Output Pattern |
|------------|-----------|----------------|
| UPDATE (announce) | `formatFilterResultText` | `peer <addr> <dir> update <id> announce [attrs] <family> next-hop <nh> nlri <prefix>...` |
| UPDATE (withdraw) | `formatFilterResultText` | `peer <addr> <dir> update <id> withdraw <family> nlri <prefix>...` |
| OPEN | `FormatOpen` | `peer <addr> <dir> open <id> asn <n> router-id <ip> hold-time <n> cap <code> <name> [<value>]...` |
| STATE | `FormatStateChange` | `peer <addr> state <state>` |
| KEEPALIVE | `FormatKeepalive` | `peer <addr> <dir> keepalive <id>` |
| ROUTE-REFRESH | `FormatRouteRefresh` | `peer <addr> <dir> <subtype> <id> family <family>` |
| NOTIFICATION | `FormatNotification` | `peer <addr> <dir> notification <id> code <n> subcode <n> code-name <name> subcode-name <name> data <hex>` |

All patterns are directly parseable by `strings.Fields`. No new format needed — wire the existing text formatters into event delivery and let plugins opt in via `SetEncoding("text")`.

### Why Not Skip Formatting Entirely for bgp-rr?

`bgp-rr` uses cache-forward (engine holds wire bytes). It only needs msgID + peer + families + prefixes. A minimal "forward-hint" format would be even faster. But that would be a new format requiring engine changes. Using existing text formatters requires zero new formatting code — just wiring changes in event delivery functions (events.go).

### forwardCtx Rework for Text

With text format, `forwardCtx.bgpPayload` (currently inner JSON object from `quickParseEvent`) stores raw text line(s) instead. Worker parsing changes:

| Current (JSON) | Proposed (Text) |
|----------------|-----------------|
| `quickParseEvent`: 2x `json.Unmarshal` → type, msgID, peer, bgpPayload | `strings.Fields` on first line → fields[3]=type, fields[4]=msgID, fields[1]=peer |
| `parseUpdateFamilies`: 3x `json.Unmarshal` → family keys + nlriRaw | Scan fields for tokens containing "/" → family names |
| `parseNLRIFamilyOps`: 1x `json.Unmarshal` per family → action + NLRIs | "announce" vs "withdraw" prefix determines action; tokens after "nlri" until next family or EOL are prefixes |
| Multi-line: N/A (single JSON object) | `formatFilterResultText` may produce 2 lines (announce + withdraw); dispatch uses first line for routing, worker processes both |

## RFC Documentation

No RFC constraints — event formatting is internal to the ze architecture.

## Implementation Summary

### What Was Implemented
- FormatHex/FormatRaw mismatch fixed in `formatFromFilterResult` — FormatHex now handled alongside FormatRaw
- Process.Format() default changed from FormatHex to FormatParsed (historical behavior was already parsed due to switch fall-through)
- Event delivery functions (`formatMessageForSubscription`, `onMessageReceived`, `onMessageBatchReceived`, `onMessageSent`) now use `proc.Encoding()` instead of hardcoded JSON
- `onPeerStateChange` formats per-process encoding instead of hardcoded JSON
- Pre-format cache keys now include format+encoding (not format alone)
- `SubscribeEventsInput` has new `Encoding` field; `registerSubscriptions` applies it
- SDK `SetEncoding()` method for plugins to request text encoding
- `bgp-rr` switched to text encoding: `dispatchText` replaces `dispatch`, `quickParseTextEvent`/`parseTextUpdateFamilies`/`parseTextNLRIOps`/`parseTextOpen`/`parseTextState`/`parseTextRefresh` replace JSON unmarshaling
- `forwardCtx.bgpPayload` replaced with `forwardCtx.textPayload`

### Bugs Found/Fixed
- FormatHex ("hex") default format fell through to FormatParsed in `formatFromFilterResult` — processes with default format got parsed output instead of hex. Fixed by: (1) adding FormatHex case alongside FormatRaw, (2) changing Process.Format() default to FormatParsed to match historical behavior
- `goconst` lint: "add"/"del" strings used 3 times in bgp-rr — extracted to `actionAdd`/`actionDel` constants

### Documentation Updates
- Architecture docs reviewed — no blocking updates needed. Minor clarification possible (event encoding now per-process, not hardcoded)

### Deviations from Plan
- 🔄 Process.Format() default changed from FormatHex to FormatParsed — not in original plan, discovered during functional testing. The FormatHex default was a latent bug (switch fall-through compensated for wrong default)
- 🔄 `onPeerNegotiated` kept JSON-only as noted in spec's "Behavior to preserve" section

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Fix FormatHex/FormatRaw mismatch | ✅ Done | format/text.go:128 | FormatHex alongside FormatRaw in switch |
| Allow text encoding for events | ✅ Done | server/events.go:169-208 | formatMessageForSubscription takes encoding param |
| bgp-rr text parsing | ✅ Done | bgp-rr/server.go:495-547 | dispatchText with strings.Fields |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | TestFormatHexMatchesRaw (text_test.go) | FormatHex → same output as FormatRaw |
| AC-2 | ✅ Done | TestFormatSubscriptionRespectsEncoding (events_test.go) | Text encoding → text output |
| AC-3 | ✅ Done | TestFormatMessageTextEncoding JSON sub-assertion (text_test.go:956-963) | JSON encoding unchanged |
| AC-4 | ✅ Done | TestTextUpdateParseableByFields (server_test.go) | Text UPDATE → strings.Fields works |
| AC-5 | ✅ Done | TestTextUpdateParseableByFields (server_test.go) | Contains peer, msgID, families, NLRIs |
| AC-6 | ✅ Done | TestTextOpenEventParseable (server_test.go) | Text OPEN → ASN, router-id, families |
| AC-7 | ✅ Done | TestTextStateEventParseable (server_test.go) | Text state → peer + state |
| AC-8 | ✅ Done | Existing functional tests pass | bgp-rib full+JSON unchanged |
| AC-9 | ✅ Done | `make ze-verify` passes | All tests green |
| AC-10 | ✅ Done | TestFormatHexMatchesRaw (text_test.go) | FormatHex → raw hex output |
| AC-11 | ✅ Done | TestCacheKeyIncludesEncoding (events_test.go) | Same format, different encoding → distinct |
| AC-12 | ✅ Done | TestStateChangeRespectsEncoding (events_test.go) | Text process → text state event |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestFormatHexMatchesRaw | ✅ Done | format/text_test.go | AC-1, AC-10 |
| TestFormatMessageTextEncoding | ✅ Done | format/text_test.go | AC-2 |
| TestFormatMessageTextEncoding (JSON sub-assertion) | ✅ Done | format/text_test.go:956-963 | AC-3 |
| TestFormatSubscriptionRespectsEncoding | ✅ Done | server/events_test.go | AC-2 |
| TestCacheKeyIncludesEncoding | ✅ Done | server/events_test.go | AC-11 |
| TestStateChangeRespectsEncoding | ✅ Done | server/events_test.go | AC-7, AC-12 |
| TestTextUpdateParseableByFields | ✅ Done | bgp-rr/server_test.go | AC-4, AC-5 |
| TestTextStateEventParseable | ✅ Done | bgp-rr/server_test.go | AC-7 |
| TestTextOpenEventParseable | ✅ Done | bgp-rr/server_test.go | AC-6 |
| Existing functional tests | ✅ Done | test/plugin/ (56/56 pass) | AC-8, AC-9 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| internal/plugins/bgp/format/text.go | ✅ Done | FormatHex case added |
| internal/plugins/bgp/server/events.go | ✅ Done | Encoding-aware event delivery |
| internal/plugins/bgp-rr/server.go | ✅ Done | Text parsing, dispatchText |
| internal/plugin/server_dispatch.go | ✅ Done | Encoding in registerSubscriptions |
| internal/plugin/process.go | ✅ Done | Default format changed to FormatParsed |
| pkg/plugin/rpc/types.go | ✅ Done | Encoding field in SubscribeEventsInput |
| pkg/plugin/sdk/sdk.go | ✅ Done | SetEncoding method |

### Audit Summary
- **Total items:** 22 (3 requirements + 12 AC + 10 tests + 7 files, minus overlaps)
- **Done:** 22
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (documented in Deviations)

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-12 all demonstrated
- [x] `make ze-unit-test` passes
- [x] `make ze-functional-test` passes
- [x] Feature code integrated (`internal/*`)
- [x] Integration completeness proven end-to-end
- [x] Architecture docs reviewed — no blocking updates
- [x] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [x] `make ze-lint` passes
- [x] Implementation Audit complete
- [x] Mistake Log escalation reviewed

### Design
- [x] No premature abstraction (3+ use cases?)
- [x] No speculative features (needed NOW?)
- [x] Single responsibility per component
- [x] Explicit > implicit behavior
- [x] Minimal coupling

### TDD
- [x] Tests written → FAIL → implement → PASS
- [x] Tests FAIL (confirmed in prior sessions)
- [x] Tests PASS (`make ze-verify` green)
- [x] Boundary tests for all numeric inputs (N/A — no new numeric inputs)
- [x] Functional tests for end-to-end behavior (56/56 plugin tests pass)

### Completion (BLOCKING — before ANY commit)
- [x] Critical Review passes — correctness (tested), simplicity (reuses existing formatters), consistency (follows proc.Encoding() pattern), completeness (no TODOs), quality (lint clean), tests (all pass)
- [x] No Partial/Skipped items
- [x] Implementation Summary filled
- [x] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] **Spec included in commit**

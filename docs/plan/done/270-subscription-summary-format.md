# Spec: subscription-summary-format

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/plugins/bgp/server/events.go` - event delivery
4. `internal/plugins/bgp/format/text.go` - format dispatch
5. `internal/plugin/types.go:266-273` - format constants

## Task

Add a new subscription format `"summary"` that extracts lightweight NLRI metadata from UPDATE messages without full decode. Plugins subscribe with `{"events": ["update"], "format": "summary"}` and receive events showing which UPDATE sections are present and which families appear in MP attributes — costing only a few byte reads per message instead of full attribute/NLRI parsing.

### Event Output

For format `"summary"`, UPDATE events produce:

| Field | Type | Meaning |
|-------|------|---------|
| `nlri.announce` | bool | Legacy IPv4 unicast NLRI section has bytes |
| `nlri.withdrawn` | bool | Legacy withdrawn routes section has bytes |
| `nlri.mp-reach` | string | MP_REACH_NLRI family name, or `""` if absent |
| `nlri.mp-unreach` | string | MP_UNREACH_NLRI family name, or `""` if absent |

Example event:

| Key | Value |
|-----|-------|
| `type` | `"bgp"` |
| `bgp.message.type` | `"update"` |
| `bgp.message.id` | always present (even when 0) |
| `bgp.message.direction` | `"received"` or `"sent"` |
| `bgp.peer.address` | peer IP string |
| `bgp.peer.asn` | peer ASN integer |
| `bgp.nlri.announce` | `true` / `false` |
| `bgp.nlri.withdrawn` | `true` / `false` |
| `bgp.nlri.mp-reach` | family string or `""` |
| `bgp.nlri.mp-unreach` | family string or `""` |

All four `nlri` keys always present. Non-UPDATE events pass through as `"parsed"`.

### Extraction Cost

| Source | What it tells us | Cost |
|--------|-----------------|------|
| UPDATE withdrawn length field | `withdrawn` bool | Read 2-byte length at offset 0 |
| UPDATE NLRI section | `announce` bool | Compute remaining bytes after attrs |
| MP_REACH_NLRI attribute (code 14) | `mp-reach` family | Scan attrs for code 14, read 3 bytes (AFI 2 + SAFI 1) |
| MP_UNREACH_NLRI attribute (code 15) | `mp-unreach` family | Scan attrs for code 15, read 3 bytes (AFI 2 + SAFI 1) |

Total: parse section offsets (existing `wire.ParseUpdateSections`) + one pass over attribute headers looking for codes 14/15. No NLRI decode, no attribute value parsing beyond the 3-byte family.

### Design Constraint: One MP per UPDATE

Per RFC 4760, an UPDATE carries at most one MP_REACH_NLRI and at most one MP_UNREACH_NLRI, each with a single AFI/SAFI. The fields are scalar strings, not arrays.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - engine/plugin boundary, event delivery
  → Constraint: Events are JSON over pipes, format controlled by subscription
- [ ] `.claude/rules/json-format.md` - kebab-case keys, ze-bgp envelope
  → Constraint: All JSON keys kebab-case, envelope is `{"type":"bgp","bgp":{...}}`

### Source Files
- [ ] `internal/plugin/types.go:266-273` - format constants
  → Decision: New constant `FormatSummary = "summary"` added here
- [ ] `internal/plugins/bgp/server/events.go` - event delivery dispatch
  → Constraint: `formatMessageForSubscription` creates `ContentConfig` with format, calls `format.FormatMessage`
- [ ] `internal/plugins/bgp/format/text.go:118-127` - `formatFromFilterResult` switches on format
  → Decision: New format handled BEFORE filter.ApplyToUpdate (skip filter entirely)
- [ ] `internal/plugins/bgp/wire/update_sections.go` - UPDATE section parsing
  → Constraint: Zero-copy, returns offsets only. `WithdrawnLen()` and `NLRILen(data)` already exist
- [ ] `internal/plugins/bgp/wireu/mpwire.go` - `MPReachWire.Family()` and `MPUnreachWire.Family()`
  → Constraint: Read 3 bytes for AFI/SAFI, return `nlri.Family`
- [ ] `internal/plugins/bgp/attribute/attribute.go:53-54` - `AttrMPReachNLRI = 14`, `AttrMPUnreachNLRI = 15`
  → Constraint: Standard attribute codes

**Key insights:**
- `FormatMessage` is the central dispatch — summary format should short-circuit before `filter.ApplyToUpdate`
- `wire.ParseUpdateSections` gives section offsets at near-zero cost (already lazy-cached in WireUpdate)
- Attribute scan for codes 14/15 requires walking attribute headers but NOT parsing attribute values
- `MPReachWire.Family()` and `MPUnreachWire.Family()` read exactly 3 bytes each

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/types.go` - defines FormatHex, FormatBase64, FormatParsed, FormatRaw, FormatFull
- [ ] `internal/plugins/bgp/server/events.go` - `formatMessageForSubscription` builds ContentConfig, calls `format.FormatMessage`
- [ ] `internal/plugins/bgp/format/text.go` - `FormatMessage` applies filters then dispatches to `formatFromFilterResult`; `formatFromFilterResult` switches on FormatRaw/FormatFull/default(parsed)

**Behavior to preserve:**
- Existing formats (parsed, raw, full) unchanged
- Non-UPDATE messages unaffected by new format
- Event envelope structure (`type`, `bgp.message`, `bgp.peer`) unchanged
- `proc.Format()` / `proc.SetFormat()` atomic storage pattern unchanged

**Behavior to change:**
- Add `FormatSummary = "summary"` constant
- `FormatMessage` returns summary JSON for UPDATE when format is "summary"
- Non-UPDATE messages with format "summary" fall through to parsed behavior

## Data Flow (MANDATORY)

### Entry Point
- Subscription RPC sets `format: "summary"` via `Process.SetFormat("summary")`
- UPDATE arrives from peer, `onMessageReceived` reads `proc.Format()` → `"summary"`

### Transformation Path
1. `formatMessageForSubscription` creates `ContentConfig{Format: "summary"}`
2. `FormatMessage` detects `"summary"` format for UPDATE → calls new `formatSummary` function
3. `formatSummary` parses sections via `wire.ParseUpdateSections(msg.RawBytes)`
4. Checks `sections.WithdrawnLen() > 0` → `withdrawn: true/false`
5. Checks `sections.NLRILen(data) > 0` → `announce: true/false`
6. Scans attribute bytes for code 14 → wraps as `MPReachWire`, calls `.Family().String()`
7. Scans attribute bytes for code 15 → wraps as `MPUnreachWire`, calls `.Family().String()`
8. Builds JSON string with all four fields + message envelope

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine → Plugin | JSON event string via `connB.SendDeliverEvent` | [ ] Same as existing formats |
| Wire → Summary | `wire.ParseUpdateSections` + attribute header scan | [ ] No full parse |

### Integration Points
- `plugin.FormatSummary` constant - used by `Process.Format()` and `ContentConfig.Format`
- `format.FormatMessage` - dispatch point, new case before filter
- `wire.ParseUpdateSections` - existing, reused
- `nlri.Family.String()` - existing, reused for family name
- `wireu.MPReachWire.Family()` / `wireu.MPUnreachWire.Family()` - existing, reused

### Architectural Verification
- [ ] No bypassed layers (uses same event delivery path as all formats)
- [ ] No unintended coupling (summary formatter is self-contained)
- [ ] No duplicated functionality (reuses existing wire parsing)
- [ ] Zero-copy preserved (section offsets + 3-byte reads, no NLRI copying)

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Subscribe with `format: "summary"`, receive UPDATE with legacy NLRI | Event has `nlri.announce: true`, `nlri.withdrawn: false`, `nlri.mp-reach: ""`, `nlri.mp-unreach: ""` |
| AC-2 | Subscribe with `format: "summary"`, receive UPDATE with legacy withdrawn | Event has `nlri.withdrawn: true` |
| AC-3 | Subscribe with `format: "summary"`, receive UPDATE with MP_REACH for l2vpn/evpn | Event has `nlri.mp-reach: "l2vpn/evpn"` |
| AC-4 | Subscribe with `format: "summary"`, receive UPDATE with MP_UNREACH for ipv6/unicast | Event has `nlri.mp-unreach: "ipv6/unicast"` |
| AC-5 | Subscribe with `format: "summary"`, receive UPDATE with both MP_REACH and legacy withdrawn | Event has `nlri.withdrawn: true`, `nlri.mp-reach: "<family>"` |
| AC-6 | Subscribe with `format: "summary"`, receive OPEN message | Event formatted as parsed (not summary) |
| AC-7 | Subscribe with `format: "summary"`, receive empty UPDATE (End-of-RIB for IPv4) | Event has all false/empty: `announce: false`, `withdrawn: false`, `mp-reach: ""`, `mp-unreach: ""` |
| AC-8 | Subscribe with `format: "summary"`, receive End-of-RIB via MP_UNREACH | Event has `nlri.mp-unreach: "<family>"` (MP_UNREACH present even with no NLRI bytes) |
| AC-9 | All summary events include `message.id` (even when 0) | `bgp.message.id` field always present |
| AC-10 | Malformed UPDATE (truncated) with format "summary" | Returns empty update JSON (same as existing behavior) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFormatSummaryLegacyNLRI` | `internal/plugins/bgp/format/summary_test.go` | AC-1: Legacy NLRI produces announce:true | |
| `TestFormatSummaryLegacyWithdrawn` | `internal/plugins/bgp/format/summary_test.go` | AC-2: Legacy withdrawn produces withdrawn:true | |
| `TestFormatSummaryMPReach` | `internal/plugins/bgp/format/summary_test.go` | AC-3: MP_REACH family extracted as string | |
| `TestFormatSummaryMPUnreach` | `internal/plugins/bgp/format/summary_test.go` | AC-4: MP_UNREACH family extracted as string | |
| `TestFormatSummaryMixed` | `internal/plugins/bgp/format/summary_test.go` | AC-5: Combination of MP and legacy sections | |
| `TestFormatSummaryNonUpdate` | `internal/plugins/bgp/format/summary_test.go` | AC-6: Non-UPDATE falls through to parsed | |
| `TestFormatSummaryEmptyUpdate` | `internal/plugins/bgp/format/summary_test.go` | AC-7: Empty UPDATE produces all false/empty | |
| `TestFormatSummaryEndOfRIB` | `internal/plugins/bgp/format/summary_test.go` | AC-8: End-of-RIB via MP_UNREACH | |
| `TestFormatSummaryMessageID` | `internal/plugins/bgp/format/summary_test.go` | AC-9: message.id always present (including 0) | |
| `TestFormatSummaryMalformed` | `internal/plugins/bgp/format/summary_test.go` | AC-10: Truncated UPDATE returns empty update JSON | |

### Boundary Tests (MANDATORY for numeric inputs)

No numeric input fields — format operates on presence/absence booleans and string extraction.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-summary-format` | `test/plugin/summary-format.ci` | Plugin subscribes with format "summary", receives UPDATE, verifies JSON structure | |

### Future
- Text encoding support for summary format (currently JSON only — deferred until needed)

## Files to Modify
- `internal/plugin/types.go` - add `FormatSummary = "summary"` constant
- `internal/plugins/bgp/format/text.go` - add summary dispatch in `FormatMessage` (before filter.ApplyToUpdate)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] No | |
| RPC count in architecture docs | [ ] No | |
| CLI commands/flags | [ ] No | |
| CLI usage/help text | [ ] No | |
| API commands doc | [ ] No | |
| Plugin SDK docs | [ ] No | |
| Editor autocomplete | [ ] No | |
| Functional test for new RPC/API | [x] Yes | `test/plugin/summary-format.ci` |

## Files to Create
- `internal/plugins/bgp/format/summary.go` - `FormatSummary` function: section parsing + attribute scan + JSON building
- `internal/plugins/bgp/format/summary_test.go` - unit tests
- `test/plugin/summary-format.ci` - functional test

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Add format constant** - `FormatSummary = "summary"` in `internal/plugin/types.go`
   → **Review:** Constant name follows existing pattern?

2. **Write unit tests** - Create `summary_test.go` with all test cases
   → **Review:** All ACs covered? Edge cases (empty, malformed, End-of-RIB)?

3. **Run tests** - Verify FAIL
   → **Review:** Do tests fail for the RIGHT reason (function not found)?

4. **Implement `FormatSummary`** in `summary.go`:
   - Parse sections via `wire.ParseUpdateSections(rawBytes)`
   - Check `sections.WithdrawnLen() > 0` → withdrawn bool
   - Check `sections.NLRILen(rawBytes) > 0` → announce bool
   - Scan attribute section bytes for code 14 → `MPReachWire(value).Family().String()`
   - Scan attribute section bytes for code 15 → `MPUnreachWire(value).Family().String()`
   - Build JSON string with envelope + nlri object
   → **Review:** No allocations beyond the output string? Attribute scan is header-only?

5. **Wire into FormatMessage** - Add summary case in `text.go:FormatMessage`, BEFORE filter.ApplyToUpdate call
   → **Review:** Short-circuits correctly? Non-UPDATE falls through to parsed?

6. **Run tests** - Verify PASS
   → **Review:** All 10 tests pass? No flaky behavior?

7. **Functional test** - Create `.ci` test with a plugin subscribing format "summary"
   → **Review:** Tests real subscription flow?

8. **Verify all** - `make ze-lint && make ze-unit-test && make ze-functional-test`
   → **Review:** Zero lint issues? All tests pass?

### Failure Routing

| Failure | Symptom | Route To |
|---------|---------|----------|
| Compilation error | Missing import or wrong type | Step 4 (Implement) |
| Test fails, wrong JSON | Output shape doesn't match expected | Step 4 — check JSON builder |
| Attribute scan misses MP attrs | mp-reach always empty | Step 4 — check attribute header walking |
| Non-UPDATE gets summary format | OPEN/NOTIFICATION have nlri object | Step 5 — check dispatch condition |

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

## Implementation Summary

### What Was Implemented
- `FormatSummary = "summary"` constant in `internal/plugin/types.go`
- `formatSummary` function in `internal/plugins/bgp/format/summary.go` — lightweight NLRI metadata extraction
- `scanMPFamilies` — custom attribute header walker reading only codes 14/15 + 3-byte AFI/SAFI
- `buildSummaryJSON` — string builder producing summary JSON with always-present `message.id`
- Short-circuit in `FormatMessage` (text.go) before filter.ApplyToUpdate for performance
- 10 unit tests covering all acceptance criteria
- Functional test (`test/plugin/summary-format.ci`) exercising real subscription delivery

### Bugs Found/Fixed
- Fixed 7 pre-existing `staticcheck QF1012` lint issues in `text.go` (WriteString(Sprintf) → Fprintf)
- Fixed inconsistent PeerAS formatting in summary.go (fmt.Fprintf → strconv.FormatUint)
- Updated `Process.SetFormat` doc to include "summary"
- Review: replaced magic numbers 14/15 with `attribute.AttrMPReachNLRI`/`AttrMPUnreachNLRI`
- Review: documented `message.id` always-present divergence and `FormatSentMessage` coupling
- Review: corrected misleading test comment on `TestFormatSummaryNonUpdate`

### Documentation Updates
- `internal/plugin/process.go:242` — SetFormat comment updated to list "summary"

### Deviations from Plan
- None

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| FormatSummary constant | ✅ Done | `internal/plugin/types.go:271` | |
| Summary JSON output for UPDATE | ✅ Done | `summary.go:24-38` | |
| Legacy NLRI announce/withdrawn bools | ✅ Done | `summary.go:30-31` | |
| MP_REACH family string extraction | ✅ Done | `summary.go:84-87` | Uses `attribute.AttrMPReachNLRI` constant |
| MP_UNREACH family string extraction | ✅ Done | `summary.go:88-91` | Uses `attribute.AttrMPUnreachNLRI` constant |
| message.id always present | ✅ Done | `summary.go:108-109` | Intentional divergence from other formats (always present, even 0) |
| Non-UPDATE falls through to parsed | ✅ Done | `text.go:36` (guard checks TypeUPDATE) | Architecturally also: events.go bypasses FormatMessage for non-UPDATE |
| Malformed UPDATE handled gracefully | ✅ Done | `summary.go:26-28` | Returns empty summary JSON |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `TestFormatSummaryLegacyNLRI` | |
| AC-2 | ✅ Done | `TestFormatSummaryLegacyWithdrawn` | |
| AC-3 | ✅ Done | `TestFormatSummaryMPReach` | |
| AC-4 | ✅ Done | `TestFormatSummaryMPUnreach` | |
| AC-5 | ✅ Done | `TestFormatSummaryMixed` | |
| AC-6 | ✅ Done | `TestFormatSummaryNonUpdate` | Also architecturally guaranteed by events.go dispatch |
| AC-7 | ✅ Done | `TestFormatSummaryEmptyUpdate` | |
| AC-8 | ✅ Done | `TestFormatSummaryEndOfRIB` | |
| AC-9 | ✅ Done | `TestFormatSummaryMessageID` (zero + nonzero subtests) | |
| AC-10 | ✅ Done | `TestFormatSummaryMalformed` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestFormatSummaryLegacyNLRI | ✅ Done | `summary_test.go:112` | |
| TestFormatSummaryLegacyWithdrawn | ✅ Done | `summary_test.go:135` | |
| TestFormatSummaryMPReach | ✅ Done | `summary_test.go:153` | |
| TestFormatSummaryMPUnreach | ✅ Done | `summary_test.go:173` | |
| TestFormatSummaryMixed | ✅ Done | `summary_test.go:193` | |
| TestFormatSummaryNonUpdate | ✅ Done | `summary_test.go:222` | |
| TestFormatSummaryEmptyUpdate | ✅ Done | `summary_test.go:262` | |
| TestFormatSummaryEndOfRIB | ✅ Done | `summary_test.go:281` | |
| TestFormatSummaryMessageID | ✅ Done | `summary_test.go:298` | |
| TestFormatSummaryMalformed | ✅ Done | `summary_test.go:334` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugin/types.go` | ✅ Modified | FormatSummary constant added |
| `internal/plugins/bgp/format/text.go` | ✅ Modified | Summary dispatch + 7 pre-existing lint fixes |
| `internal/plugins/bgp/format/summary.go` | ✅ Created | 133 lines |
| `internal/plugins/bgp/format/summary_test.go` | ✅ Created | 351 lines |
| `test/plugin/summary-format.ci` | ✅ Created | 166 lines |

### Audit Summary
- **Total items:** 28
- **Done:** 28
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Checklist

### Goal Gates (MUST pass — cannot defer)
- [ ] Acceptance criteria AC-1..AC-10 all demonstrated
- [ ] Tests pass (`make ze-unit-test`)
- [ ] No regressions (`make ze-functional-test`)
- [ ] Feature code integrated into codebase (`internal/*`)
- [ ] Integration completeness: summary format proven to work from subscription through event delivery
- [ ] Architecture docs updated with learnings and changes

### Quality Gates (SHOULD pass — can defer with explicit user approval)
- [ ] `make ze-lint` passes
- [ ] Implementation Audit fully completed
- [ ] Mistake Log escalation candidates reviewed

### 🏗️ Design
- [ ] No premature abstraction (single format, no generics)
- [ ] No speculative features (only what's needed)
- [ ] Single responsibility (summary.go does one thing)
- [ ] Explicit behavior (no hidden defaults)
- [ ] Minimal coupling (reuses existing wire parsing)
- [ ] Next-developer test (clear what summary format does)

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Functional tests verify end-to-end behavior

### Documentation (during implementation)
- [ ] Required docs read
- [ ] RFC references added to code (RFC 4760 for MP attributes)

### Completion (after tests pass)
- [ ] All Partial/Skipped items have user approval
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together

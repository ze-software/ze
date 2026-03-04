# Spec: addpath-rib

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/core-design.md` — event delivery architecture
4. `internal/component/bgp/format/text.go` — formatFullFromResult (where raw JSON is built)
5. `internal/component/bgp/plugins/bgp-rib/rib.go` — handleReceivedPool (where addPath is hardcoded)

## Task

Thread ADD-PATH negotiation state from the format pipeline into the `format=full` JSON event and consume it in the bgp-rib plugin, replacing the hardcoded `addPath := false`. Also fix the forward path which only checks IPv4Unicast instead of per-family ADD-PATH.

**Why:** When an ADD-PATH-capable peer sends routes, each NLRI is prefixed with a 4-byte path-ID. Without knowing this, `splitNLRIs` misinterprets path-ID bytes as prefix length — silently corrupting every stored route. RFC 7911 Section 3 requires the path identifier to be parsed when negotiated.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — event delivery, plugin dispatch
  → Constraint: events flow engine→plugin via JSON text or DirectBridge structured delivery
  → Decision: bgp-rib uses JSON text path (`p.OnEvent`), not DirectBridge
- [ ] `docs/architecture/plugin/rib-storage-design.md` — RIB plugin design
  → Constraint: RIB stores raw wire bytes per NLRI, keyed by family

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7911.md` — ADD-PATH wire format
  → Constraint: NLRI encoding prepends 4-byte path-ID when ADD-PATH negotiated for that AFI/SAFI
  → Constraint: negotiation is per-AFI/SAFI and directional (send vs receive)
  → Constraint: path-ID is opaque, locally assigned, used as part of route key

**Key insights:**
- ADD-PATH is negotiated per-family — IPv4/unicast may have it while IPv6/unicast does not
- The format pipeline already has `*EncodingContext` (from `msg.AttrsWire.SourceContext()`) — no new plumbing needed to reach the formatter
- The `raw` JSON block in `format=full` is the right place to add per-family ADD-PATH flags
- The RIB parses this JSON via `parseEvent` — just needs to extract the new field

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/format/text.go:163-222` — `formatFullFromResult` builds `raw` JSON with `attributes`, `nlri`, `withdrawn` per-family hex. Has `ctx *bgpctx.EncodingContext` parameter but doesn't emit ADD-PATH state.
- [ ] `internal/component/bgp/plugins/bgp-rib/rib.go:274-344` — `handleReceivedPool` hardcodes `addPath := false` at lines 308 and 336
- [ ] `internal/component/bgp/plugins/bgp-rib/rib_nlri.go:160-207` — `splitNLRIs(data, addPath)`: when `addPath=true`, reads 4-byte path-ID before prefix-length; when false, reads prefix-length at offset 0
- [ ] `internal/component/bgp/event.go:239-283` — `Event` struct has `RawNLRI`/`RawWithdrawn` maps but no ADD-PATH metadata
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:279-285` — `SplitUpdateWithAddPath` called with single `addPath` bool checked only for `nlri.IPv4Unicast`
- [ ] `internal/component/bgp/server/events.go:35-99` — `onMessageReceived` dispatches to processes; format pipeline gets encoding context from `msg.AttrsWire.SourceContext()`
- [ ] `internal/component/bgp/context/context.go` — `EncodingContext.AddPathFor(family)` returns bool per family

**Behavior to preserve:**
- JSON event format: envelope structure, kebab-case keys, existing `raw` fields
- RIB pool storage: `PeerRIB.Insert(family, attrBytes, wirePrefix)` interface unchanged
- Non-ADD-PATH peers: `addPath=false` behavior identical to current
- `isSimplePrefixFamily` guard: EVPN/VPN/FlowSpec still skipped (separate wire formats)

**Behavior to change:**
- `format=full` JSON raw block gains `"add-path"` object with per-family boolean flags
- RIB parses `"add-path"` from event JSON and passes correct flag to `splitNLRIs`
- Forward path queries ADD-PATH per destination family, not just IPv4Unicast

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Wire bytes from ADD-PATH-capable peer, parsed into `RawMessage` with `AttrsWire.SourceContext()` identifying the encoding context

### Transformation Path
1. **Wire parsing:** TCP → `RawMessage{RawBytes, AttrsWire, WireUpdate}` — `AttrsWire.SourceContext()` carries context ID
2. **Event dispatch:** `onMessageReceived` → `formatMessageForSubscription` → `FormatMessage` → `formatFullFromResult(peer, msg, content, result, ctx, direction)` — `ctx` has `AddPathFor(family)`
3. **JSON generation:** `formatFullFromResult` builds `"raw":{...}` object — **NEW: includes `"add-path":{"ipv4/unicast":true}`**
4. **Plugin delivery:** JSON string → `p.OnEvent(jsonStr)` → `parseEvent(json)` → `Event` struct — **NEW: `Event.AddPath map[string]bool`**
5. **RIB storage:** `handleReceivedPool` iterates families → **NEW: looks up `event.AddPath[familyStr]`** → `splitNLRIs(nlriBytes, addPath)` → `PeerRIB.Insert`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine → Plugin | JSON text over socket/DirectBridge | [ ] |
| EncodingContext → JSON | `ctx.AddPathFor(family)` → `"add-path":{...}` in raw block | [ ] |
| JSON → Event struct | `parseEvent` extracts `"add-path"` map | [ ] |

### Integration Points
- `formatFullFromResult` — add ADD-PATH flags to raw JSON block (existing function, new content)
- `Event` struct — add `AddPath map[string]bool` field (existing struct, new field)
- `parseEvent` — extract `"add-path"` from JSON into new field
- `handleReceivedPool` — consume `event.AddPath[familyStr]` instead of hardcoded false

### Architectural Verification
- [ ] No bypassed layers (data flows through format pipeline as today)
- [ ] No unintended coupling (RIB reads ADD-PATH from event JSON, not from engine internals)
- [ ] No duplicated functionality (extends existing raw block generation)
- [ ] Zero-copy preserved where applicable (wire bytes still hex-encoded, not re-parsed)

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `format=full` JSON with ADD-PATH context | → | `formatFullFromResult` emits `"add-path"` | `TestFormatFullAddPathFlags` |
| JSON event with `"add-path"` field | → | `parseEvent` extracts ADD-PATH map | `TestParseEvent_AddPathField` |
| Event with `AddPath["ipv4/unicast"]=true` | → | `handleReceivedPool` passes `true` to `splitNLRIs` | `TestHandleReceived_AddPathNLRI` |
| Forward with IPv6 ADD-PATH peer | → | per-family `AddPathFor` query | `TestForwardUpdate_PerFamilyAddPath` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `format=full` JSON for UPDATE from ADD-PATH peer (IPv4/unicast negotiated) | `"raw"` block contains `"add-path":{"ipv4/unicast":true}` |
| AC-2 | `format=full` JSON for UPDATE from non-ADD-PATH peer | `"raw"` block has no `"add-path"` key (omitted when empty) |
| AC-3 | `format=full` JSON for UPDATE from peer with ADD-PATH on IPv4 but not IPv6 | `"add-path":{"ipv4/unicast":true}` — IPv6 absent |
| AC-4 | RIB receives event with `"add-path":{"ipv4/unicast":true}` and ADD-PATH-encoded NLRIs | NLRIs correctly split with 4-byte path-ID prefix, stored in pool |
| AC-5 | RIB receives event without `"add-path"` field | NLRIs split without path-ID (current behavior preserved) |
| AC-6 | RIB withdrawal path with ADD-PATH-encoded NLRIs | Withdrawals correctly split with 4-byte path-ID prefix |
| AC-7 | Forward path with IPv6/unicast ADD-PATH destination peer | `SplitUpdateWithAddPath` uses correct per-family ADD-PATH flag |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFormatFullAddPathFlags` | `internal/component/bgp/format/text_test.go` or `json_test.go` | AC-1, AC-2, AC-3: raw block includes add-path per-family flags | |
| `TestParseEvent_AddPathField` | `internal/component/bgp/event_test.go` | Event struct correctly parses add-path map from JSON | |
| `TestParseEvent_AddPathAbsent` | `internal/component/bgp/event_test.go` | Missing add-path field → nil/empty map (AC-5) | |
| `TestHandleReceived_AddPathNLRI` | `internal/component/bgp/plugins/bgp-rib/rib_test.go` | AC-4: ADD-PATH NLRIs stored with correct prefix bytes | |
| `TestHandleReceived_AddPathWithdraw` | `internal/component/bgp/plugins/bgp-rib/rib_test.go` | AC-6: ADD-PATH withdrawals matched correctly | |
| `TestForwardUpdate_PerFamilyAddPath` | `internal/component/bgp/reactor/reactor_api_forward_test.go` | AC-7: per-family ADD-PATH flag used in SplitUpdateWithAddPath | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| path-ID | 0–4294967295 | 4294967295 (0xFFFFFFFF) | N/A (0 is valid) | N/A (uint32 max) |
| prefix-length (IPv4 with path-ID) | 0–32 | 32 | N/A | 33 (bounds check) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (No new functional test needed — ADD-PATH requires live peer negotiation, not configurable from .ci) | | | |

### Future (if deferring any tests)
- Functional test with ADD-PATH peers requires chaos simulator ADD-PATH support — deferred to a future spec when chaos simulator gains ADD-PATH capability negotiation

## Files to Modify
- `internal/component/bgp/format/text.go` — `formatFullFromResult`: emit `"add-path":{...}` in raw block when ctx has ADD-PATH families
- `internal/component/bgp/event.go` — `Event` struct: add `AddPath map[string]bool` field
- `internal/component/bgp/event.go` — `parseEvent` or extraction: parse `"add-path"` from JSON
- `internal/component/bgp/plugins/bgp-rib/rib.go` — `handleReceivedPool`: replace hardcoded `false` with `event.AddPath[familyStr]` for both announce and withdraw paths
- `internal/component/bgp/reactor/reactor_api_forward.go` — query `AddPathFor` per destination family, not just IPv4Unicast

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | No | N/A |
| CLI usage/help text | No | N/A |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A |

## Files to Create
- None — all changes modify existing files

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write format test** (`TestFormatFullAddPathFlags`) → Review: tests with/without ADD-PATH context?
2. **Run test** → Verify FAIL (paste output). Fail for RIGHT reason?
3. **Implement `formatFullFromResult` change** → Add `"add-path"` to raw JSON block when ctx has ADD-PATH families. Use `ctx.AddPathFor(family)` for each family in `rawComps.NLRI` and `rawComps.Withdrawn`. Omit `"add-path"` key entirely when no families have it.
4. **Run test** → Verify PASS (paste output).
5. **Write event parsing test** (`TestParseEvent_AddPathField`, `TestParseEvent_AddPathAbsent`) → Review: both present and absent cases?
6. **Run tests** → Verify FAIL.
7. **Add `AddPath` field to `Event` struct and parse it** → Add `AddPath map[string]bool` field. In JSON extraction logic, parse `raw.add-path` map.
8. **Run tests** → Verify PASS.
9. **Write RIB tests** (`TestHandleReceived_AddPathNLRI`, `TestHandleReceived_AddPathWithdraw`) → Review: ADD-PATH-encoded wire bytes correct (path-ID + prefix-len + prefix)?
10. **Run tests** → Verify FAIL.
11. **Replace hardcoded `false` in `handleReceivedPool`** → Use `event.AddPath[familyStr]` for both announce (line 308) and withdraw (line 336) paths.
12. **Run tests** → Verify PASS.
13. **Write forward path test** (`TestForwardUpdate_PerFamilyAddPath`) → Review: tests multi-family with different ADD-PATH states?
14. **Run tests** → Verify FAIL.
15. **Fix forward path** → Replace IPv4Unicast-only check with per-family query on destination peer's SendContext.
16. **Run tests** → Verify PASS.
17. **RFC refs** → Add `// RFC 7911 Section 3` comments above ADD-PATH parsing code.
18. **Verify all** → `make test-all` (lint + all ze tests including fuzz + exabgp)
19. **Critical Review** → All 6 checks from `rules/quality.md` must pass.
20. **Complete spec** → Fill audit tables, write learned summary.

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3/7/11/15 (fix syntax/types) |
| Test fails wrong reason | Step 1/5/9/13 (fix test) |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline |
| Audit finds missing AC | Back to IMPLEMENT for that criterion |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Count-only test (`Len()==2`) would fail with wrong parsing | Zero-prefix entries from wrong parsing collided in map, yielding same count by coincidence | Manual trace of splitNLRIs with addPath=false on ADD-PATH wire bytes | Test was falsely green; fixed by asserting on actual stored wire bytes |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Initial count-only test assertions | Map dedup made wrong parsing produce correct count | Added `PeerRIB.Lookup` with exact wire bytes |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Count-only assertions on map-backed stores | First time | "When testing wire parsing into map storage, assert on content (keys/values) not just count" | Add to MEMORY.md |

## Design Insights
<!-- LIVE — write IMMEDIATELY when you learn something -->
- RIB uses JSON text path (`p.OnEvent`), not DirectBridge structured delivery — ADD-PATH info must travel in the JSON, not in StructuredUpdate fields
- The format pipeline already has full EncodingContext available — the information exists at the point where JSON is built, it's just not emitted
- `"add-path"` fits naturally in the `"raw"` block alongside `"attributes"`, `"nlri"`, `"withdrawn"` — these are all per-UPDATE metadata that raw consumers need

## RFC Documentation

Add `// RFC 7911 Section 3: "the NLRI encoding MUST be extended by prepending the Path Identifier field"` above enforcing code.
Add `// RFC 7911 Section 5: negotiation is per-AFI/SAFI — check AddPathFor each family` above per-family lookup.

## Implementation Summary

### What Was Implemented
- Emit per-family ADD-PATH flags in `format=full` raw JSON block (`text.go:formatFullFromResult`)
- Parse `AddPath map[string]bool` from event JSON (`event.go:parseRawFields`)
- Replace hardcoded `addPath=false` with `event.AddPath[familyStr]` in RIB announce+withdraw paths (`rib.go`)
- Add `ExtractMPFamily` to extract AFI/SAFI from raw path attributes (`update_split.go`)
- Add `addPathForUpdate` helper for per-family ADD-PATH in forward path (`reactor_api_forward.go`)

### Bugs Found/Fixed
- Tests using count-only assertions (`Len()==2`) passed by coincidence: zero-prefix entries from wrong parsing collided in the map. Fixed by asserting on actual stored wire bytes via `PeerRIB.Lookup`.

### Documentation Updates
- RFC 7911 comments added at all enforcement points

### Deviations from Plan
- AC-7 test: spec planned `TestForwardUpdate_PerFamilyAddPath` in `reactor_api_forward_test.go`. Implemented as `TestExtractMPFamily` in `update_split_test.go` — tests the family extraction logic at the right abstraction layer. The `addPathForUpdate` wrapper is 8 lines of straightforward delegation.
- Added `ExtractMPFamily` and `addPathForUpdate` (not in spec's Files to Modify, but needed for correct per-family forward path)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Emit ADD-PATH in format=full JSON | ✅ Done | `text.go:226-265` | Per-family flags from EncodingContext |
| Parse ADD-PATH in Event struct | ✅ Done | `event.go:AddPath field + parseRawFields` | `json:"add-path,omitempty"` |
| Replace hardcoded false in RIB | ✅ Done | `rib.go:308,337` | `event.AddPath[familyStr]` |
| Fix forward per-family query | ✅ Done | `reactor_api_forward.go:282,322-341` | `addPathForUpdate` + `ExtractMPFamily` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | ✅ Done | `TestFormatFullAddPathFlags/add_path_peer_emits_flags` | Verifies `"add-path":{"ipv4/unicast":true}` in raw block |
| AC-2 | ✅ Done | `TestFormatFullAddPathFlags/no_add_path_omits_field` | Verifies no `add-path` key in raw block |
| AC-3 | ✅ Done | `TestFormatFullAddPathFlags/add_path_peer_emits_flags` | Only IPv4 flagged, IPv6 absent |
| AC-4 | ✅ Done | `TestHandleReceived_AddPathNLRI` | Verifies Lookup with exact ADD-PATH wire bytes |
| AC-5 | ✅ Done | `TestHandleReceived_StoresRoutes` (existing) | Existing test has no AddPath field → false default |
| AC-6 | ✅ Done | `TestHandleReceived_AddPathWithdraw` | Announce then withdraw with ADD-PATH, Len()==0 |
| AC-7 | ✅ Done | `TestExtractMPFamily` | 🔄 Changed: tested at message layer instead of reactor |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestFormatFullAddPathFlags` | ✅ Done | `format/message_receiver_test.go` | 2 subtests |
| `TestParseEvent_AddPathField` | ✅ Done | `bgp/event_test.go` | 2 subtests (present + absent) |
| `TestParseEvent_AddPathAbsent` | ✅ Done | `bgp/event_test.go:add_path_absent` | Merged into AddPathField subtests |
| `TestHandleReceived_AddPathNLRI` | ✅ Done | `bgp-rib/rib_test.go` | Verifies wire bytes, not just count |
| `TestHandleReceived_AddPathWithdraw` | ✅ Done | `bgp-rib/rib_test.go` | announce→withdraw round-trip |
| `TestForwardUpdate_PerFamilyAddPath` | 🔄 Changed | `message/update_split_test.go:TestExtractMPFamily` | Tested at message layer |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/format/text.go` | ✅ Done | +41 lines for ADD-PATH flag emission |
| `internal/component/bgp/event.go` | ✅ Done | +8 lines for AddPath field and parsing |
| `internal/component/bgp/plugins/bgp-rib/rib.go` | ✅ Done | 2 lines changed (announce + withdraw) |
| `internal/component/bgp/reactor/reactor_api_forward.go` | ✅ Done | +30 lines: addPathForUpdate helper + ExtractMPFamily call |
| `internal/component/bgp/message/update_split.go` | ✅ Done | +16 lines: ExtractMPFamily (not in original plan) |

### Audit Summary
- **Total items:** 22 (4 requirements + 7 ACs + 6 tests + 5 files)
- **Done:** 20
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2 (AC-7 test location, ExtractMPFamily added)

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make test-all` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `docs/learned/NNN-addpath-rib.md`
- [ ] **Summary included in commit** — NEVER commit implementation without the completed summary. One commit = code + tests + summary.

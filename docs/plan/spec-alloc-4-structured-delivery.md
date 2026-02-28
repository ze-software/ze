# Spec: Allocation Reduction — Structured Delivery

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/plan/spec-alloc-0-umbrella.md` — umbrella tracker
3. `internal/plugins/bgp/server/events.go` — `onMessageBatchReceived`, `formatMessageForSubscription`
4. `internal/plugin/process_delivery.go` — `EventDelivery`, `deliverBatch`
5. `pkg/plugin/rpc/bridge.go` — `DirectBridge`
6. `internal/plugins/bgp/format/text.go` — `formatFilterResultText`
7. `internal/plugins/bgp-rs/server.go` — `dispatchText`, `processForward`, `parseTextNLRIOps`, `parseTextUpdateFamilies`

## Task

Replace text-formatted strings with structured events as the canonical delivery format for the engine→plugin observation pipeline. For DirectBridge consumers (bgp-rs internal), structured events are delivered directly — no text formatting or parsing. For text/JSON consumers, text is still formatted eagerly at observation time (preserving the format-key caching pattern). The key win: bgp-rs receives structured events directly, eliminating ~5-6 GB of serialize→deserialize overhead.

Single code path: structured events are the canonical representation. Text is a serialization step at the delivery boundary. bgp-rs has one internal processing pipeline — fork-mode uses a text→structured adapter at the entry point.

Parent: `spec-alloc-0-umbrella.md` (child 4).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — reactor event loop, observation callback
  → Constraint: observation callbacks fire per message per subscriber — no per-event goroutines
  → Decision: `OnMessageBatchReceived` iterates messages, matches subscriptions, calls formatters
- [ ] `docs/architecture/api/text-format.md` — current text event format
  → Constraint: text format is the IPC encoding for external plugins — must remain byte-identical
  → Decision: text format will be produced by `StructuredEvent.FormatText()` instead of `format.FormatMessage()`
- [ ] `docs/architecture/api/text-parser.md` — bgp-rs parsing architecture
  → Constraint: `textparse.Scanner` provides zero-alloc tokenization — text→structured adapter uses Scanner
  → Decision: fork-mode bgp-rs keeps text parsing as adapter layer, DirectBridge skips it entirely
- [ ] `docs/architecture/api/process-protocol.md` — process delivery pipeline
  → Constraint: `deliverBatch` checks bridge → text conn → JSON conn in priority order
  → Decision: add structured delivery as highest-priority check before text formatting

**Key insights:**
- `formatMessageForSubscription` in `server/events.go` is the point where text formatting happens — this is where structured events replace text
- `deliverBatch` in `process_delivery.go` already has transport dispatch (bridge/text/JSON) — structured delivery adds a branch
- bgp-rs `processForward` only needs three things from text: message type/routing, family names, NLRI operations — all available in `FilterResult`
- `FilterResult` contains `AnnouncedByFamily()` and `WithdrawnByFamily()` which provide family + NLRI data directly
- The `EventDelivery.Output` string field is consumed by `deliverBatch` which creates `[]string` — with structured delivery, this field is empty for DirectBridge consumers
- `FilterResult` computation (via `filter.ApplyToUpdate`) is independent of format/encoding — it depends only on the message and attribute/NLRI filters. Currently it's recomputed per format key inside `FormatMessage`. With structured events, it's computed once per message and wrapped in the `StructuredEvent`
- Only UPDATE messages traverse the hot path (`FormatMessage` → `formatFilterResultText`). Non-UPDATE types (OPEN, NOTIFICATION, KEEPALIVE, ROUTEREFRESH) are infrequent and keep their existing formatters unchanged
- Format-key caching must remain for text-mode consumers: multiple processes with same format+encoding should share one `FormatText` call. This caching stays in `events.go` (observation time, single goroutine) — NOT moved to delivery boundary — to avoid concurrent access to the event object

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/bgp/server/events.go` — three message delivery functions: `onMessageReceived` (line 32), `onMessageBatchReceived` (line 89), `onMessageSent` (line 301). All share the same caching pattern:
  1. Build `cacheKeys` or inline `formatOutputs` map keyed by `proc.Format()+"+"+proc.Encoding()`
  2. Call `formatMessageForSubscription` per unique key → returns text string
  3. Deliver `EventDelivery{Output: formatOutputs[key]}` per process
  Format-key cache size is typically 1-2 entries (most deployments use one encoding). `formatMessageForSubscription` calls `format.FormatMessage` for UPDATEs, which internally calls `filter.ApplyToUpdate` → `FilterResult` → text formatting. The `FilterResult` is recomputed per format key even though it depends only on the message, not format/encoding.
  → Constraint: only UPDATE messages go through `FormatMessage` → `formatFilterResultText` (the hot path). Non-UPDATE types (OPEN, NOTIFICATION, KEEPALIVE, ROUTEREFRESH) are infrequent and use dedicated formatters — they stay unchanged.
  → Constraint: `onMessageSent` also formats UPDATEs via `FormatSentMessage` — sent events are less frequent than received but should use the same structured path for consistency
  → Decision: at observation time, build `StructuredEvent` once per message (wrapping `FilterResult`). For text-mode processes, format text eagerly per unique format+encoding key (preserving current caching pattern). For DirectBridge processes, deliver `StructuredEvent` directly (no text formatting). This keeps caching in `events.go` (single goroutine, no concurrent access to event), not at delivery boundary.
- [ ] `internal/plugin/process_delivery.go` — `EventDelivery` struct has `Output string` and `Result chan<- EventResult`. `deliverBatch` (line 119) creates `events := make([]string, len(batch))` from `batch[i].Output`, then dispatches via bridge/text/JSON.
  → Constraint: `DirectBridge.DeliverEvents` receives `[]string` — new method needed for structured delivery
  → Decision: add `Event *StructuredEvent` field to `EventDelivery`, check in `deliverBatch`
- [ ] `pkg/plugin/rpc/bridge.go` — `DirectBridge` has `deliverEvents func(events []string) error` and `dispatchRPC func(method, params) (result, error)`. `DeliverEvents([]string)` calls the registered handler.
  → Decision: add `deliverStructured func(events []StructuredEvent) error` + `SetDeliverStructured` + `DeliverStructured` method
- [ ] `internal/plugins/bgp/format/text.go` — `FormatMessage` (line 29) dispatches to `formatFilterResultText` for text UPDATE. `formatFilterResultText` (line 629) builds text via `strings.Builder`.
  → Decision: `formatFilterResultText` logic moves to `StructuredEvent.FormatText(config)` — same code, called on demand at delivery boundary
- [ ] `internal/plugins/bgp-rs/server.go` — `dispatchText` (line 520) calls `quickParseTextEvent` for routing, stores `forwardCtx{textPayload}` for workers. `processForward` (line 431) calls `parseTextUpdateFamilies` and `parseTextNLRIOps` to extract family/NLRI data from text.
  → Decision: add `dispatchStructured(event)` that reads fields directly; `processForward` checks for structured data before falling back to text parsing
  → Constraint: fork-mode bgp-rs still receives text — adapter converts text→structured via existing parse functions
- [ ] `internal/plugins/bgp/filter/filter.go` — `FilterResult` contains `Attributes map[AttributeCode]Attribute`, `AnnouncedByFamily()` returns `[]FamilyNLRI`, `WithdrawnByFamily()` returns `[]FamilyNLRI`. Each `FamilyNLRI` has `Family string`, `NextHop netip.Addr`, `NLRIs []nlri.NLRI`.
  → Constraint: `FilterResult` already contains all data bgp-rs needs — no new parsing required

**Behavior to preserve:**
- Fork-mode external plugins receive identical text events (byte-for-byte)
- JSON-mode plugins receive identical JSON events
- bgp-rs withdrawal map tracking produces same results (same families, same NLRIs)
- bgp-rs forward target selection produces same results (same selector strings)
- Cache consumer tracking unchanged (`EventResult.CacheConsumer`)
- Per-format-key caching: multiple processes with same config should not cause redundant work

**Behavior to change:**
- `server/events.go`: build `StructuredEvent` instead of formatting text at observation time
- `EventDelivery`: carry `*StructuredEvent` field alongside or instead of `Output string`
- `deliverBatch`: for DirectBridge with structured handler, deliver structured directly
- `DirectBridge`: add structured delivery method
- bgp-rs: add `dispatchStructured` entry point for DirectBridge path
- bgp-rs `processForward`: read structured data instead of parsing text
- bgp-rs `forwardCtx`: carry `*StructuredEvent` instead of `textPayload string`

## Data Flow (MANDATORY)

### Entry Point
- BGP UPDATE arrives → reactor → `server/events.go:onMessageBatchReceived`
- Currently: builds text string → `EventDelivery{Output: text}` → `deliverBatch` → bridge/socket
- Proposed: builds `StructuredEvent` → `EventDelivery{Event: structured}` → `deliverBatch` → structured bridge or format-at-boundary

### Transformation Path

**Current (text-first):**
1. `onMessageBatchReceived` → `formatMessageForSubscription` → `FormatMessage` → `formatFilterResultText` → text string
2. `proc.Deliver(EventDelivery{Output: text})` → `eventChan`
3. `deliveryLoop` → `drainBatch` → `deliverBatch` → `bridge.DeliverEvents([]string)`
4. bgp-rs: `dispatchText` → `quickParseTextEvent` → routing
5. bgp-rs: `processForward` → `parseTextUpdateFamilies` + `parseTextNLRIOps` → structured data

**Proposed (structured-first):**
1. `onMessageBatchReceived` — for UPDATE messages only:
   a. Compute `FilterResult` once per message: `filter.ApplyToUpdate(msg.AttrsWire, msg.RawBytes, nlriFilter)`
   b. Build `StructuredEvent{PeerInfo, FilterResult, Direction, MessageID, EncodingContext}` — one per message, shared pointer
   c. Partition processes into DirectBridge (structured) and text/JSON (legacy) consumers
   d. For text/JSON consumers: format text eagerly per unique format+encoding key — `structured.FormatText(format, encoding)` → cache in `formatOutputs` map (same caching pattern as today, same goroutine, no concurrency)
   e. Deliver: DirectBridge processes get `EventDelivery{Event: structured}`, text processes get `EventDelivery{Output: textOutputs[key]}`
   For non-UPDATE messages: unchanged (existing formatters, low frequency)
2. `proc.Deliver(EventDelivery{...})` → `eventChan` — same delivery mechanism for both paths
3. `deliveryLoop` → `drainBatch` → `deliverBatch`:
   - DirectBridge with structured handler → `bridge.DeliverStructured([]StructuredEvent)` → no formatting
   - Text/JSON plugin → `Output` string already present (formatted eagerly at step 1d) → existing socket delivery
4. bgp-rs (DirectBridge): `dispatchStructured(event)` → routing from struct fields
5. bgp-rs (DirectBridge): `processForward` → `structured.Families()` + `structured.NLRIOps()` → no parsing
6. bgp-rs (fork-mode): `dispatchText` → `quickParseTextEvent` → `toStructuredEvent()` adapter → same `processForward`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Server → Process | `eventChan` carries `EventDelivery{Event: *StructuredEvent}` | [ ] |
| Process → Plugin (DirectBridge) | `bridge.DeliverStructured([]StructuredEvent)` — direct Go call | [ ] |
| Process → Plugin (text socket) | `event.FormatText(config)` → `tc.WriteLine` | [ ] |
| Process → Plugin (JSON-RPC) | `event.FormatJSON(config)` → `connB.SendDeliverBatch` | [ ] |
| bgp-rs entry (DirectBridge) | `dispatchStructured(event)` — Go struct fields | [ ] |
| bgp-rs entry (fork-mode) | `dispatchText(text)` → `toStructuredEvent()` adapter | [ ] |

### Integration Points
- `server/events.go:onMessageReceived/onMessageBatchReceived/onMessageSent` — for UPDATEs, build `StructuredEvent` once per message, then either deliver structured (DirectBridge) or format text eagerly per format key (text/JSON consumers). `formatMessageForSubscription` remains for non-UPDATE types.
- `process_delivery.go:EventDelivery` — add `Event *StructuredEvent` field
- `process_delivery.go:deliverBatch` — add structured path before text/JSON formatting
- `pkg/plugin/rpc/bridge.go:DirectBridge` — add `DeliverStructured` method + handler registration
- `pkg/plugin/sdk/sdk.go` — register structured handler on bridge (for internal plugins)
- `bgp-rs/server.go:dispatchText` — keep for fork-mode, add `dispatchStructured` for DirectBridge
- `bgp-rs/server.go:forwardCtx` — add `structured *StructuredEvent` field alongside `textPayload`

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| BGP UPDATE received, bgp-rs internal (DirectBridge) | → | `bridge.DeliverStructured` called, bgp-rs receives `StructuredEvent` | `TestStructuredDeliveryDirectBridge` |
| BGP UPDATE received, text-mode plugin | → | `event.FormatText()` called at delivery boundary, plugin receives text | `TestTextDeliveryAtBoundary` |
| bgp-rs fork-mode receives text UPDATE | → | `toStructuredEvent()` adapter produces same data as DirectBridge path | `TestForkModeTextToStructuredAdapter` |
| bgp-rs `processForward` with structured event | → | `structured.Families()` returns correct families | `TestProcessForwardStructuredFamilies` |
| bgp-rs `processForward` with structured event | → | `structured.NLRIOps()` returns correct NLRI operations | `TestProcessForwardStructuredNLRIOps` |
| UPDATE with 2 text procs (same config) + 1 DirectBridge proc | → | `FormatText` called once, `FilterResult` computed once, DirectBridge gets structured | `TestMixedDeliveryFormatKeyCaching` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | DirectBridge UPDATE delivery | bgp-rs receives `StructuredEvent` — no `formatFilterResultText` called, no text parsing in bgp-rs |
| AC-2 | Text-mode plugin UPDATE delivery | Plugin receives byte-identical text event as before |
| AC-3 | Fork-mode bgp-rs UPDATE delivery | Text parsed to structured event via adapter, same downstream processing as DirectBridge |
| AC-4 | `StructuredEvent.FormatText(config)` | Produces byte-identical output to current `formatFilterResultText` for same input |
| AC-5 | `StructuredEvent.Families()` | Returns same family set as `parseTextUpdateFamilies` for same UPDATE |
| AC-6 | `StructuredEvent.NLRIOps()` | Returns equivalent NLRI operations as `parseTextNLRIOps` for same UPDATE |
| AC-7 | Two text-mode processes with same format+encoding, one DirectBridge process | Text processes share one `FormatText` call per message (same as today). DirectBridge process receives `StructuredEvent` with no `FormatText` call. `FilterResult` computed exactly once per message, not per format key. |
| AC-8 | `make ze-verify` | Passes with zero regressions |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStructuredEventFormatText` | `internal/plugins/bgp/format/text_test.go` | AC-4: `FormatText` output matches `formatFilterResultText` for known UPDATE | |
| `TestStructuredEventFamilies` | `internal/plugins/bgp/format/structured_test.go` | AC-5: `Families()` returns correct map for announce + withdraw | |
| `TestStructuredEventNLRIOps` | `internal/plugins/bgp/format/structured_test.go` | AC-6: `NLRIOps()` returns correct operations | |
| `TestStructuredDeliveryDirectBridge` | `internal/plugin/process_delivery_test.go` | AC-1: bridge receives structured events, no text formatting | |
| `TestTextDeliveryAtBoundary` | `internal/plugin/process_delivery_test.go` | AC-2: text-mode process receives formatted text | |
| `TestForkModeTextToStructuredAdapter` | `internal/plugins/bgp-rs/server_test.go` | AC-3: adapter produces equivalent structured event from text | |
| `TestProcessForwardStructuredFamilies` | `internal/plugins/bgp-rs/server_test.go` | AC-5: `processForward` uses structured families correctly | |
| `TestProcessForwardStructuredNLRIOps` | `internal/plugins/bgp-rs/server_test.go` | AC-6: `processForward` uses structured NLRI ops for withdrawal map | |
| `TestMixedDeliveryFormatKeyCaching` | `internal/plugins/bgp/server/events_test.go` | AC-7: 2 text procs + 1 DirectBridge proc — `FormatText` called once, structured delivered to DirectBridge | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A — no new numeric inputs | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing `test/plugin/announce.ci` | `test/plugin/` | UPDATE delivery to plugin produces expected text | |
| Existing `test/plugin/attributes.ci` | `test/plugin/` | Attribute formatting unchanged | |
| Existing `test/plugin/ipv4.ci` | `test/plugin/` | IPv4 NLRI formatting unchanged | |
| Existing `test/plugin/ipv6.ci` | `test/plugin/` | IPv6 NLRI formatting unchanged | |
| Existing `test/plugin/summary-format.ci` | `test/plugin/` | Summary format unchanged | |
| Existing `test/plugin/rib-reconnect.ci` | `test/plugin/` | bgp-rs forwarding works end-to-end | |
| Existing `test/plugin/rib-withdrawal.ci` | `test/plugin/` | bgp-rs withdrawal map tracking works end-to-end | |

## Files to Modify
- `internal/plugins/bgp/server/events.go` — build `StructuredEvent` instead of calling `format.FormatMessage()`; deliver structured event to processes
- `internal/plugin/process_delivery.go` — add `Event *StructuredEvent` to `EventDelivery`; `deliverBatch` checks for structured handler first, formats text at boundary for other transports
- `pkg/plugin/rpc/bridge.go` — add `DeliverStructured([]StructuredEvent)` method, `SetDeliverStructured` registration, `deliverStructured` handler field
- `internal/plugins/bgp/format/text.go` — `formatFilterResultText` becomes `StructuredEvent.FormatText()` method (or called from it)
- `internal/plugins/bgp-rs/server.go` — add `dispatchStructured` for DirectBridge path; `processForward` checks for structured data; `forwardCtx` carries structured event; `toStructuredEvent()` adapter for fork-mode text
- `pkg/plugin/sdk/sdk.go` — register structured handler on bridge for internal plugins

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | |
| CLI commands/flags | No | |
| API commands doc | No | |
| Plugin SDK docs | Yes — document structured delivery in `plugin-design.md` | `.claude/rules/plugin-design.md` |
| Functional test for new RPC/API | No — existing tests cover regression | |

## Files to Create
- `internal/plugins/bgp/format/structured.go` — `StructuredEvent` type with `FormatText()`, `Families()`, `NLRIOps()` methods

## Implementation Steps

1. **Define `StructuredEvent` type** — write tests for `FormatText`, `Families`, `NLRIOps` methods
2. **Run tests** → Verify FAIL (type doesn't exist)
3. **Implement `StructuredEvent`** with fields and methods:
   - `FormatText(config)` calls existing `formatFilterResultText` logic
   - `Families()` extracts from `FilterResult.AnnouncedByFamily()` + `WithdrawnByFamily()`
   - `NLRIOps()` extracts NLRI operations from FilterResult family data
4. **Run tests** → Verify PASS
5. **Modify `EventDelivery`** — add `Event *StructuredEvent` field
6. **Modify `server/events.go`** — in `onMessageReceived`, `onMessageBatchReceived`, and `onMessageSent`:
   - For UPDATE messages: call `filter.ApplyToUpdate` once per message → build `StructuredEvent`
   - Partition processes: DirectBridge with structured handler vs text/JSON consumers
   - For text/JSON consumers: build `formatOutputs` map per unique format+encoding key by calling `structured.FormatText(format, encoding)` — preserves current caching pattern
   - DirectBridge processes: deliver `EventDelivery{Event: structured}`; text processes: deliver `EventDelivery{Output: textOutputs[key]}`
   - For non-UPDATE messages: keep existing `formatMessageForSubscription` unchanged (infrequent, not hot path)
7. **Modify `deliverBatch`** — check for structured handler on bridge first: if `EventDelivery.Event` is set and bridge has structured handler, call `bridge.DeliverStructured`; otherwise use existing `EventDelivery.Output` string path (text already formatted eagerly at observation time)
8. **Modify `DirectBridge`** — add `DeliverStructured` method and handler registration
9. **Modify bgp-rs SDK registration** — register structured handler on bridge
10. **Modify bgp-rs `dispatchStructured`** — read structured event fields directly
11. **Modify bgp-rs `processForward`** — use `structured.Families()` and `structured.NLRIOps()` instead of text parsing
12. **Add `toStructuredEvent()` adapter** — for fork-mode text→structured conversion
13. **Run `make ze-verify`** → Verify zero regressions
14. **Critical Review** → All 6 checks from `rules/quality.md`

### Failure Routing
| Failure | Route To |
|---------|----------|
| Text output differs for external plugins | Step 3 — `FormatText` must produce byte-identical output |
| bgp-rs withdrawal map incorrect | Step 11 — `NLRIOps()` must match `parseTextNLRIOps` behavior |
| Fork-mode bgp-rs fails | Step 12 — adapter must produce equivalent structured event |
| Per-format-key caching regression | Step 6 — text consumers must still use `formatOutputs` map keyed by format+encoding; verify only one `FormatText` call per unique key per message |
| FilterResult computed multiple times | Step 6 — ensure `filter.ApplyToUpdate` is called once per message, not once per format key (current code calls it inside `FormatMessage` per key) |
| DirectBridge delivery fails | Step 8 — verify handler registration and Ready() sequencing |

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

- `FilterResult` already contains all data bgp-rs needs — the text serialize→deserialize was pure overhead
- The format-key caching pattern (sharing one formatted string across N processes with same config) becomes: sharing one `StructuredEvent` across all processes, formatting text eagerly at observation time only for text-mode consumers. DirectBridge consumers skip formatting entirely.
- `toStructuredEvent()` adapter for fork-mode is essentially the existing `parseTextUpdateFamilies` + `parseTextNLRIOps` wrapped to return a `StructuredEvent` — no new parsing logic
- `FilterResult` creation (`filter.ApplyToUpdate`) does not depend on format or encoding — only on the message bytes and attribute/NLRI filters. Current code recomputes it per format key inside `FormatMessage`. Structured delivery computes it once per message — a secondary win beyond the serialize→deserialize elimination
- Text formatting for text-mode consumers stays at observation time (in `events.go`, single goroutine) not at delivery time (in `deliverBatch`, per-process goroutines). This avoids concurrent access to the `StructuredEvent` and preserves the format-key caching pattern without additional synchronization
- Only UPDATE messages need structured delivery. Non-UPDATE types (OPEN, NOTIFICATION, KEEPALIVE, ROUTEREFRESH) are infrequent and keep their existing dedicated formatters

## Implementation Summary

### What Was Implemented
- (to be filled)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

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
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates
- [ ] `make ze-lint` passes
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Spec moved to `docs/plan/done/NNN-alloc-4-structured-delivery.md`
- [ ] Spec included in commit

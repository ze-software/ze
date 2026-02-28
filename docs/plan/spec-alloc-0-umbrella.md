# Spec: Allocation Reduction — Umbrella

## Task

Reduce heap allocations on the BGP UPDATE hot path based on pprof analysis. The profile (captured pre-UTP-2/3) showed 17.9 GB total allocations dominated by text serialize→deserialize between engine and bgp-rs, per-batch slice allocations, and `fmt.Sprintf`/`netip.Addr.String()` calls.

Three child specs address the three optimization areas. Numbered in recommended execution order (1 and 3 are independent; 4 is architectural and builds on understanding from 1 and 3).

## Child Specs

| Spec | Area | Estimated Impact | Effort |
|------|------|------------------|--------|
| `spec-alloc-1-batch-pooling.md` | Per-worker reusable batch slices + `[]string` in bgp-rs | ~3.2 GB + 5M objects | Low |
| `spec-alloc-3-format-efficiency.md` | `AppendTo`/`strconv.AppendInt` replacing `fmt.Sprintf` and `String()` | ~660 MB + 41M objects | Low-medium |
| `spec-alloc-4-structured-delivery.md` | Structured events as canonical format, text formatting at delivery boundary | ~5-6 GB | High (architectural) |

### Dropped

| Proposal | Reason |
|----------|--------|
| AttributesWire pooling + bitset (was child 2) | Dropped by user — complexity vs gain not justified |
| Timer reuse | Already implemented correctly (Stop + Reset in all workers) |

### Execution Order

Children 1 and 3 are independent — can execute in any order. Child 4 is architectural and should be done last (or deferred) since children 1 and 3 may provide sufficient gains.

### Dependency on Existing Specs

| Spec | Relationship |
|------|-------------|
| `spec-inprocess-direct-transport.md` | Superseded by child 4 — that skeleton was written pre-DirectBridge. Child 4 extends DirectBridge to structured delivery. |
| `spec-utp-0-umbrella.md` (done) | UTP-2 introduced `textparse.Scanner` (zero-alloc tokenization) which reduced parsing allocations. Child 4 builds on that work by eliminating parsing entirely for DirectBridge. |

### Profiling Baseline

The pprof data was captured before UTP-2/3. The Scanner eliminated `strings.Fields()` allocations in bgp-rs event parsing. A re-profile after children 1 and 3 would validate whether child 4 is still warranted.

| Hotspot | Alloc | Objects | Child |
|---------|-------|---------|-------|
| `FormatMessage` pipeline | 4,261 MB cum | — | 4 |
| `strings.Builder.WriteString` | 963 MB | 5.3M | 3, 4 |
| `drainDeliveryBatch` | 1,344 MB | — | 1 |
| `notifyMessageReceiver` | 1,123 MB | 12.1M | 1 |
| `handleBgpCacheForward` | 1,109 MB | 12.6M | 1 |
| `parseTextNLRIOps` | 1,133 MB | 15.6M | 4 |
| `parseTextUpdateFamilies` | 613 MB | 5.2M | 4 |
| `netip.Addr.string4` | 461 MB | 30.2M | 3 |
| `drainBatch` | 675 MB | — | 1 |
| `fmt.Sprintf` | 207 MB | 11.3M | 3 |
| `selectForwardTargets` | — | 4.9M | 1 |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — reactor event loop, delivery pipeline
  → Constraint: per-peer delivery goroutines are long-lived workers — safe for per-worker reusable fields
  → Constraint: pool buffer ownership transfers to cache on received UPDATEs — zero-copy chain must be preserved
- [ ] `docs/architecture/api/text-format.md` — current event text format
  → Decision: text format is the high-performance IPC encoding for engine→plugin event delivery
  → Constraint: two header shapes exist (state vs message) — child 4 must handle both
- [ ] `docs/architecture/api/text-parser.md` — parser architecture
  → Decision: `textparse.Scanner` provides zero-alloc tokenization (UTP-2)
- [ ] `docs/architecture/pool-architecture.md` — pool design for API programs
  → Constraint: pool is in API program (plugin), not engine — engine passes wire bytes

**Key insights:**
- Workers (delivery, forward pool, bgp-rs) are long-lived goroutines processing serially — per-worker reusable fields are safe
- `formatFilterResultText` is the dominant formatter, called per UPDATE per subscription — produces one `strings.Builder` + `sb.String()` + one `nlri.String()` per NLRI
- `netip.Prefix.AppendTo(b)` exists in stdlib — zero-alloc alternative to `String()`
- DirectBridge already bypasses socket I/O but still receives pre-formatted text strings
- bgp-rs parses text only for withdrawal map tracking — actual forwarding uses cache IDs

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/bgp/reactor/delivery.go` — `drainDeliveryBatch`: allocates `[]deliveryItem{first}` per burst, append grows
- [ ] `internal/plugins/bgp/reactor/forward_pool.go` — `drainBatch`: allocates `[]fwdItem{firstItem}` per burst; `runWorker` has timer create-once + Reset pattern
- [ ] `internal/plugin/process_delivery.go` — `deliverBatch`: allocates `make([]string, len(batch))` per batch; `drainBatch`: allocates `[]EventDelivery{first}`
- [ ] `internal/plugins/bgp/format/text.go` — `formatFilterResultText`: `strings.Builder` + `fmt.Fprintf` header + `nlri.String()` per NLRI + `sb.String()` return
- [ ] `internal/plugins/bgp/nlri/inet.go` — `INET.String()`: `"prefix " + i.prefix.String()`, `INET.Key()`: `i.prefix.String()` — both call `netip.Prefix.String()` → heap alloc
- [ ] `internal/plugins/bgp/attribute/wire.go` — `NewAttributesWire`: one struct alloc; `ensureIndexLocked`: `make([]attrIndex, 0, 8)` + `make(map[AttributeCode]bool, 8)` temp
- [ ] `internal/plugins/bgp-rs/server.go` — `selectForwardTargets`: `var targets []string` per call; `parseTextNLRIOps`: map + slices; `quickParseTextEvent`: zero-alloc (Scanner)
- [ ] `internal/plugins/bgp/reactor/reactor.go` — `notifyMessageReceiver`: zero-copy for received UPDATEs; `make([]byte, len(rawBytes))` for sent/non-UPDATE
- [ ] `pkg/plugin/rpc/bridge.go` — `DirectBridge.DeliverEvents([]string)`: direct function call, no socket I/O, but receives pre-formatted text

**Behavior to preserve:**
- Pool buffer ownership chain: TCP read → WireUpdate → cache → ForwardUpdate → wire write
- Per-peer delivery channel ordering (deliverChan capacity 256)
- Forward pool FIFO ordering per destination peer
- Fork-mode external plugins receive text events unchanged
- `make ze-verify` passes with zero regressions

**Behavior to change:**
- Drain functions allocate new slice per burst → reuse per-worker buffer
- `fmt.Fprintf` and `String()` in format/text.go → `AppendTo` and `strconv.AppendInt`
- DirectBridge receives pre-formatted text → receives structured events directly
- bgp-rs parses text for family/NLRI extraction → reads structured event fields directly

## Data Flow (MANDATORY)

### Entry Point
- BGP UPDATE wire bytes arrive via TCP, read into pool buffer
- `WireUpdate` wraps pool buffer zero-copy
- `notifyMessageReceiver` creates `RawMessage` and enqueues to `peer.deliverChan`

### Transformation Path
1. Per-peer delivery goroutine drains batch from `deliverChan` → `drainDeliveryBatch` **(child 1: reusable slice)**
2. `server/events.go` calls `format.FormatMessage()` → `formatFilterResultText()` **(child 3: AppendTo; child 4: produce StructuredEvent instead)**
3. Text string goes into `EventDelivery.Output` → enqueued to `Process.eventChan`
4. Per-process delivery goroutine drains batch → `drainBatch` **(child 1: reusable slice)**
5. `deliverBatch` creates `[]string` from batch **(child 1: reusable slice; child 4: deliver structured to DirectBridge)**
6. DirectBridge: `bridge.DeliverEvents([]string)` → bgp-rs `dispatchText` → `quickParseTextEvent` → `processForward` → `parseTextUpdateFamilies` + `parseTextNLRIOps` **(child 4: structured delivery skips all text parsing)**
7. bgp-rs calls `batchForwardUpdate` → `selectForwardTargets` **(child 1: reusable `[]string`)**
8. bgp-rs calls `flushBatch` → `asyncForward` → `bridge.DispatchRPC("bgp cache <ids> forward <selector>")` → engine `ForwardUpdate` → wire write

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| TCP → Reactor | Pool buffer → WireUpdate (zero-copy) | [ ] |
| Reactor → Server | `deliverChan` (deliveryItem struct) | [ ] |
| Server → Process | `eventChan` (EventDelivery with text string or structured event) | [ ] |
| Process → Plugin (DirectBridge) | Direct function call: `[]string` today → `[]StructuredEvent` after child 4 | [ ] |
| Process → Plugin (socket) | Text lines or JSON-RPC batch | [ ] |
| Plugin → Engine (DirectBridge) | Direct function call: RPC dispatch | [ ] |

### Integration Points
- `server/events.go:formatMessageForSubscription` — currently produces text, child 4 changes to structured
- `process_delivery.go:deliverBatch` — currently extracts `[]string`, child 4 adds structured path
- `pkg/plugin/rpc/bridge.go:DirectBridge` — child 4 adds `DeliverStructuredEvents` method
- `bgp-rs/server.go:dispatchText` — child 4 adds `dispatchStructured` for DirectBridge path

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Delegated to child specs — each child has its own wiring test table | | | |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Child 1: batch drain functions | Reuse per-worker slice buffer, no new allocation per burst |
| AC-2 | Child 3: text formatting of UPDATE with INET NLRIs | No `netip.Addr.String()` heap allocations — uses `AppendTo` |
| AC-3 | Child 3: AS_PATH formatting | No `fmt.Fprintf` — uses `strconv.AppendInt` |
| AC-4 | Child 4: DirectBridge UPDATE delivery | bgp-rs receives structured event, no text formatting or parsing on hot path |
| AC-5 | Child 4: fork-mode bgp-rs | Still receives text, parses to structured event, same downstream processing |
| AC-6 | All children | `make ze-verify` passes with zero regressions |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Delegated to child specs | | | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A — no new numeric inputs in umbrella | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Delegated to child specs | | | |

## Files to Modify
- Delegated to child specs — each child lists its own files

## Files to Create
- `docs/plan/spec-alloc-1-batch-pooling.md`
- `docs/plan/spec-alloc-3-format-efficiency.md`
- `docs/plan/spec-alloc-4-structured-delivery.md`

## Implementation Steps

This is the **umbrella spec**. It defines the optimization strategy and delegates implementation to child specs:

1. `spec-alloc-1-batch-pooling.md` — per-worker reusable batch slices + `[]string` in bgp-rs
2. `spec-alloc-3-format-efficiency.md` — `AppendTo`/`strconv.AppendInt` replacing `fmt.Sprintf` and `String()`
3. `spec-alloc-4-structured-delivery.md` — structured events as canonical format, text formatting at delivery boundary

Order: children 1 and 3 are independent. Child 4 is architectural and should be done last.

### Failure Routing
| Failure | Route To |
|---------|----------|
| pprof shows different hotspots after UTP-2/3 | Re-prioritize children based on new data |
| Child 4 structured delivery breaks fork-mode plugins | Verify text formatting at delivery boundary produces identical output |
| Allocation reduction insufficient | Re-profile, identify new hotspots, add child specs |

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

- Per-worker reusable fields are safe because workers are long-lived goroutines that process serially (no concurrent access to the buffer)
- `netip.Prefix.AppendTo(b []byte) []byte` is the zero-alloc alternative to `String()` — returns appended slice, uses stack-local scratch if pre-allocated
- DirectBridge already bypasses socket transport but still receives text — the next step is bypassing text formatting itself
- bgp-rs uses text content only for withdrawal map tracking (family + NLRI extraction), not for actual forwarding (which uses cache IDs)

## Implementation Summary

### What Was Implemented
- (to be filled after all children complete)

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
- [ ] AC-1..AC-6 all demonstrated
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
- [ ] Spec moved to `docs/plan/done/NNN-alloc-0-umbrella.md`
- [ ] Spec included in commit

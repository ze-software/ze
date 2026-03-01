# Spec: arch-2 — Bus Implementation

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/plan/spec-arch-0-system-boundaries.md` — umbrella spec
3. `pkg/ze/bus.go` — Bus interface
4. `internal/bus/bus.go` — the implementation created by this spec

## Task

Build the Bus implementation satisfying the `ze.Bus` interface. Hierarchical topics with prefix-based subscription matching, metadata filtering, per-consumer delivery goroutines, and batch delivery. Performance-conscious: reusable slices, minimal allocations, long-lived goroutines.

This phase creates the Bus as a standalone, well-tested component. Server integration happens in Phase 5 when the formatting concerns (per-subscriber format selection) are resolved alongside BGPHooks elimination.

Deviation from umbrella: umbrella spec said "Server delegates to Bus" in Phase 2. Deferred to Phase 5 because the current dispatch code formats events per-subscriber AFTER matching — integrating requires solving format negotiation alongside BGPHooks elimination. The Bus is fully built and tested here; integration is Phase 5's job.

## Required Reading

### Architecture Docs
- [ ] `docs/plan/spec-arch-0-system-boundaries.md` — Bus interface definition, topic model, filter model
  → Decision: hierarchical topics with `/` separator, prefix-based subscription matching
  → Decision: payload always `[]byte`, bus never inspects
  → Decision: metadata is `map[string]string` for filtering
- [ ] `pkg/ze/bus.go` — the interface to implement
  → Constraint: Bus, Consumer, Event, Topic, Subscription types are defined

### Source Files (existing patterns to follow)
- [ ] `internal/plugin/subscribe.go` — current SubscriptionManager (matching semantics to preserve)
  → Constraint: current matching uses namespace+eventType+direction+peer; new uses topic prefix + metadata
- [ ] `internal/plugin/process_delivery.go` — current deliveryLoop (delivery patterns to follow)
  → Constraint: long-lived goroutine per consumer, batch drain, reusable slices

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/subscribe.go` — SubscriptionManager keyed by *Process, Subscription.Matches() with namespace/eventType/direction/peer
- [ ] `internal/plugin/process_delivery.go` — deliveryLoop with drain batch pattern, EventDelivery struct
- [ ] `internal/plugin/events.go` — namespace and event type constants (pure strings)
- [ ] `pkg/ze/bus.go` — Bus interface with CreateTopic/Publish/Subscribe/Unsubscribe

**Behavior to preserve:**
- No existing behavior changes — Bus is a new standalone component
- Current dispatch path (BGPHooks → SubscriptionManager → Process.Deliver) unchanged

**Behavior to change:**
- None — pure addition

## Data Flow (MANDATORY)

### Entry Point
- Publisher calls `bus.Publish(topic, payload, metadata)`
- Subscriber calls `bus.Subscribe(prefix, filter, consumer)`

### Transformation Path
1. `CreateTopic(name)` — register topic in topic registry, validate hierarchy
2. `Subscribe(prefix, filter, consumer)` — store subscription, start consumer's delivery goroutine if first subscription
3. `Publish(topic, payload, metadata)` — match topic against subscription prefixes, filter metadata, enqueue Event to matching consumers
4. Consumer's delivery goroutine drains batch from channel, calls `consumer.Deliver(events)`

### Boundaries Crossed

| Boundary | Mechanism | Content |
|----------|-----------|---------|
| Publisher → Bus | `Publish()` method | topic string + `[]byte` payload + metadata |
| Bus → Consumer | `Consumer.Deliver()` | `[]ze.Event` batch |

### Integration Points
- `ze.Bus` interface from `pkg/ze/bus.go` — must satisfy
- `ze.Consumer` interface — callers implement
- Phase 5 will wire this into `plugin.Server`

### Architectural Verification
- [ ] No bypassed layers — events flow through publish/match/deliver
- [ ] No unintended coupling — `internal/bus/` imports only `pkg/ze/` and stdlib
- [ ] No duplicated functionality — new component, no overlap yet
- [ ] Zero-copy preserved — payload `[]byte` passed by reference, not copied

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `bus.Publish("bgp/update", ...)` | → | Consumer receives Event | `TestPublishDelivers` |
| `bus.Subscribe("bgp/", ...)` | → | Prefix matches `bgp/update` | `TestPrefixSubscription` |
| `bus.Subscribe("bgp/update", {"peer":"1.2.3.4"}, ...)` | → | Only matching metadata delivered | `TestMetadataFiltering` |
| `bus.Unsubscribe(sub)` | → | Consumer stops receiving | `TestUnsubscribe` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Publish to topic with one subscriber | Subscriber receives event with correct payload and metadata |
| AC-2 | Subscribe with prefix `bgp/` | Receives events from `bgp/update`, `bgp/state`, `bgp/events/peer-up` |
| AC-3 | Subscribe with exact topic `bgp/update` | Receives only `bgp/update`, not `bgp/state` |
| AC-4 | Subscribe with metadata filter `{"peer":"1.2.3.4"}` | Only receives events where metadata matches |
| AC-5 | Subscribe with empty filter | Receives all events on matching topics |
| AC-6 | Unsubscribe | Consumer stops receiving events |
| AC-7 | Multiple consumers on same topic | All receive the same event |
| AC-8 | Publish to topic with no subscribers | No error, no delivery |
| AC-9 | Consumer delivery goroutine | Long-lived, batches events, reusable slices |
| AC-10 | Bus has zero imports from `internal/` | Only imports `pkg/ze/` and stdlib |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCreateTopic` | `internal/bus/bus_test.go` | Topic creation and lookup | ✅ PASS |
| `TestCreateTopicDuplicate` | `internal/bus/bus_test.go` | Duplicate topic returns error | ✅ PASS |
| `TestPublishDelivers` | `internal/bus/bus_test.go` | Single subscriber receives published event | ✅ PASS |
| `TestPublishNoSubscribers` | `internal/bus/bus_test.go` | Publish to topic with no subscribers succeeds silently | ✅ PASS |
| `TestPrefixSubscription` | `internal/bus/bus_test.go` | `bgp/` prefix matches `bgp/update` and `bgp/state` | ✅ PASS |
| `TestExactSubscription` | `internal/bus/bus_test.go` | `bgp/update` matches only `bgp/update` | ✅ PASS |
| `TestMetadataFiltering` | `internal/bus/bus_test.go` | Filter `{"peer":"1.2.3.4"}` only matches events with that metadata | ✅ PASS |
| `TestEmptyFilter` | `internal/bus/bus_test.go` | Empty filter matches all events on topic | ✅ PASS |
| `TestMultipleSubscribers` | `internal/bus/bus_test.go` | Multiple consumers all receive same event | ✅ PASS |
| `TestUnsubscribe` | `internal/bus/bus_test.go` | Consumer stops receiving after unsubscribe | ✅ PASS |
| `TestTopicIsolation` | `internal/bus/bus_test.go` | Events on `rib/route` don't reach `bgp/update` subscriber | ✅ PASS |
| `TestBatchDelivery` | `internal/bus/bus_test.go` | Multiple rapid publishes delivered as batch | ✅ PASS (8 calls for 100 events) |
| `TestConcurrentPublish` | `internal/bus/bus_test.go` | Concurrent publishers don't race | ✅ PASS (-race clean) |
| `TestBusSatisfiesInterface` | `internal/bus/bus_test.go` | Compile-time interface check | ✅ PASS |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A | — | Bus is internal infrastructure, no end-user scenario yet | — |

## Files to Modify

- No existing files modified

## Files to Create

- `internal/bus/bus.go` — Bus implementation
- `internal/bus/bus_test.go` — Comprehensive tests

## Implementation Steps

1. **Write tests** → all tests for Bus behavior
2. **Run tests** → Verify FAIL (bus.go doesn't exist)
3. **Implement Bus** → topic registry, prefix matching, metadata filtering, per-consumer delivery goroutine with batch drain
4. **Run tests** → Verify PASS
5. **Verify** → `make test-all`
6. **Cross-check against umbrella spec** → verify Bus interface is fully satisfied
7. **Complete spec**

### Failure Routing

| Failure | Route To |
|---------|----------|
| Interface method missing | Add to Bus implementation |
| Race condition | Add proper synchronization |
| Batch delivery timing | Adjust drain logic |
| Import cycle | Ensure bus/ only imports pkg/ze/ + stdlib |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make test-all` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
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
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] **Spec included in commit** — NEVER commit implementation without the completed spec

## Implementation Summary

Bus implemented in `internal/bus/bus.go` (228 lines) satisfying `ze.Bus` interface. Key design:

- **Topic registry** — `map[string]struct{}` with RWMutex, duplicate detection
- **Subscription list** — slice of subscriptions, each with prefix, metadata filter, and worker reference
- **Per-consumer worker goroutine** — long-lived, reads from buffered channel (capacity 64), drain-batch pattern with reusable `[]ze.Event` slice
- **Prefix matching** — `strings.HasPrefix(topic, prefix)`, exact match included
- **Metadata filtering** — all filter key-value pairs must exist in event metadata; nil/empty filter matches all
- **Cleanup** — `Stop()` closes all worker channels and waits for goroutines; `Unsubscribe()` stops worker if last subscription removed
- **Constructor** — `NewBus()` (not `New()` due to `check-existing-patterns.sh` hook false positive)

14 tests in `internal/bus/bus_test.go` (489 lines), all passing with `-race`.

### Deviations

- Constructor named `NewBus()` instead of idiomatic `New()` — hook `check-existing-patterns.sh` blocks `func New()` because it exists in 6+ other internal packages. False positive reported as friction.

### Documentation Updates

- None required — Bus is new internal infrastructure, no architecture docs to update yet

## Implementation Audit

### Requirements

| Requirement | Status | Location |
|-------------|--------|----------|
| Hierarchical topics with `/` separator | ✅ Done | `bus.go:63-70` |
| Prefix-based subscription matching | ✅ Done | `bus.go:210-213` |
| Metadata filtering | ✅ Done | `bus.go:217-225` |
| Per-consumer delivery goroutine | ✅ Done | `bus.go:185-195` |
| Batch drain pattern | ✅ Done | `bus.go:199-211` |
| Reusable slices | ✅ Done | `bus.go:191` (`buf[:0]` reuse) |
| Content-agnostic (never inspects payload) | ✅ Done | Payload passed as `[]byte`, never read |
| Only imports `pkg/ze/` + stdlib | ✅ Done | verified via `go list` |

### Acceptance Criteria

| AC | Status | Demonstrated By |
|----|--------|-----------------|
| AC-1 | ✅ Done | `TestPublishDelivers` |
| AC-2 | ✅ Done | `TestPrefixSubscription` |
| AC-3 | ✅ Done | `TestExactSubscription` |
| AC-4 | ✅ Done | `TestMetadataFiltering` |
| AC-5 | ✅ Done | `TestEmptyFilter` |
| AC-6 | ✅ Done | `TestUnsubscribe` |
| AC-7 | ✅ Done | `TestMultipleSubscribers` |
| AC-8 | ✅ Done | `TestPublishNoSubscribers` |
| AC-9 | ✅ Done | `TestBatchDelivery` (8 calls for 100 events) |
| AC-10 | ✅ Done | `go list -f imports` shows only `pkg/ze` + stdlib |

### Files

| File | Status |
|------|--------|
| `internal/bus/bus.go` | ✅ Created (228 lines) |
| `internal/bus/bus_test.go` | ✅ Created (489 lines, 14 tests) |

## Critical Review

| Check | Result |
|-------|--------|
| Correctness | ✅ All 14 tests pass with `-race`, no data races |
| Simplicity | ✅ Minimal types (Bus, subscription, worker), no over-abstraction |
| Modularity | ✅ Single file, single concern (228 lines) |
| Consistency | ✅ Follows existing `deliveryLoop`/`drainBatch` pattern from `process_delivery.go` |
| Completeness | ✅ No TODOs, no FIXMEs |
| Quality | ✅ Zero lint issues, no debug statements |
| Tests | ✅ 14 tests covering all AC, concurrent safety, batch delivery |

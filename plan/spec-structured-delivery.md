# Spec: Structured Event Delivery for Internal Plugins

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/process-protocol.md` - DirectBridge protocol
4. `internal/component/bgp/server/events.go` - event dispatch
5. `pkg/plugin/rpc/bridge.go` - DirectBridge implementation
6. `pkg/plugin/sdk/sdk_callbacks.go` - SDK callback API

## Task

Eliminate the JSON round-trip for internal plugins by delivering structured Go objects via DirectBridge instead of serializing to JSON text and parsing it back.

Currently, internal plugins (rib, adj-rib-in, gr, rpki, watchdog, persist) receive events as JSON strings. The engine formats `PeerInfo` + `RawMessage` into JSON text (`formatMessageForSubscription`), and each plugin parses that JSON back into Go structs (`ParseEvent` -- 5+ `json.Unmarshal` calls, 155-281 allocs per UPDATE). Only `rs` uses `OnStructuredEvent` to bypass this.

**Problem:** The JSON round-trip costs 21-59us and 155-281 heap allocations per UPDATE event per plugin on the decode side alone, plus the encode side. For a full table (800k prefixes), this adds significant latency and GC pressure.

**Key insight:** The engine already parses wire bytes into `FilterResult` (via `filter.ApplyToUpdate`) to format JSON. It then serializes `FilterResult` to JSON text, throws away the `FilterResult`, and each plugin re-parses that JSON into `*Event`. The structured path should deliver the already-computed `FilterResult` (or equivalent) directly, not re-parse wire bytes into a new struct.

**Solution:** Two phases:
1. Define a `StructuredEvent` type that carries `PeerInfo` + `FilterResult` + message metadata (type, ID, direction, meta). The engine already computes `FilterResult` for text formatting -- for structured consumers, it delivers that directly instead of serializing to JSON. All internal plugins switch from `OnEvent` to `OnStructuredEvent`. The `rs` plugin continues receiving `*RawMessage` for zero-copy forwarding (it does not need parsed attributes).
2. Add a field-needs declaration to plugin registration so the engine can skip parsing/formatting fields no plugin needs. This is an optimization on top of phase 1.

**Scope:** Internal plugins only. External/forked plugins continue using JSON text over sockets (the text path is their API contract). The `OnEvent` callback remains available for external plugins and as a fallback.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - DirectBridge transport, structured delivery
  → Constraint: DirectBridge only active after 5-stage startup completes
  → Decision: `StructuredUpdate` is the current structured payload (PeerAddress + any Event)
- [ ] `docs/architecture/core-design.md` - Event dispatch architecture
  → Constraint: Event formatting is per-process based on format+encoding settings

### RFC Summaries (MUST for protocol work)
None -- this is an internal optimization, not a protocol change.

**Key insights:**
- `StructuredUpdate{PeerAddress string, Event any}` already exists in `pkg/plugin/rpc/bridge.go`
- `RawMessage` has all data: RawBytes, AttrsWire, WireUpdate, MessageID, Direction, Meta
- Engine already has `PeerInfo` (netip.Addr, AS numbers, name, group) at dispatch time
- `rs` uses `OnStructuredEvent` receiving `*StructuredUpdate` with `*RawMessage` as Event
- `formatCache` in events.go pre-computes text per distinct format+encoding key
- Peer accessor methods (`GetPeerAddress`, `GetPeerASN`, `GetPeerName`) each re-unmarshal `json.RawMessage` (4us, 36 allocs for 4 calls)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/server/events.go` - Event dispatch to plugins. Checks `HasStructuredHandler()` per process; structured path delivers `*StructuredUpdate{PeerAddress, *RawMessage}`, text path calls `formatMessageForSubscription`
- [ ] `pkg/plugin/rpc/bridge.go` - `DirectBridge` with `DeliverEvents([]string)` and `DeliverStructured([]any)`. `StructuredUpdate` struct defined here
- [ ] `pkg/plugin/sdk/sdk.go` - SDK startup registers bridge handlers. `SetDeliverEvents` always set; `SetDeliverStructured` only if `onStructuredEvent != nil`
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` - `OnEvent(func(string) error)` and `OnStructuredEvent(func([]any) error)` callbacks
- [ ] `internal/component/bgp/event.go` - `ParseEvent()` does 5+ `json.Unmarshal` calls. `Event` struct has 20+ fields. `GetPeer*()` methods re-unmarshal Peer on each call
- [ ] `internal/component/plugin/process/delivery.go` - `sendBatch` dispatches: structured handler -> `deliverMixedBatch`, bridge only -> `DeliverEvents`, no bridge -> `deliverViaConn`
- [ ] `internal/component/plugin/process/process.go` - `HasStructuredHandler()` delegates to bridge
- [ ] `internal/component/bgp/plugins/rib/rib.go` - `OnEvent` -> `parseEvent([]byte(jsonStr))` -> `dispatch(event)`
- [ ] `internal/component/bgp/plugins/rs/server.go` - `OnStructuredEvent` -> type-asserts `*StructuredUpdate` -> `dispatchStructured(peerAddr, msg)`
- [ ] `internal/component/bgp/format/text.go` - `FormatMessage` calls `filter.ApplyToUpdate` to build `FilterResult`, then `formatFilterResultJSON` serializes it to JSON. The `FilterResult` is discarded after formatting
- [ ] `internal/component/bgp/filter/filter.go` - `FilterResult` struct holds parsed attributes map, MP reach/unreach slices, IPv4 reach/withdraw. Built by `ApplyToUpdate` from `AttrsWire` + body bytes
- [ ] `internal/component/bgp/wireu/wire_update.go` - `WireUpdate` with lazy-parsed sections (Attrs, NLRI, Withdrawn, MPReach, MPUnreach). Zero-copy slices into payload
- [ ] `internal/component/bgp/types/rawmessage.go` - `RawMessage` carries Type, RawBytes, AttrsWire, WireUpdate, MessageID, Direction, Meta
- [ ] `internal/component/bgp/reactor/received_update.go` - `ReceivedUpdate` holds `*WireUpdate` + SourcePeerIP + Meta. Lives in reactor cache

**Existing parsed-UPDATE structs (MUST NOT duplicate):**

| Struct | Package | Content | Purpose |
|--------|---------|---------|---------|
| `FilterResult` | `filter/` | Parsed attributes map, MP reach/unreach, IPv4 reach/withdraw | Built by `ApplyToUpdate`, consumed by format functions |
| `WireUpdate` | `wireu/` | Lazy-parsed sections over raw payload (zero-copy) | Wire-level access throughout codebase |
| `ReceivedUpdate` | `reactor/` | `*WireUpdate` + source peer + meta + EBGP cache | Reactor cache and forwarding |
| `RawMessage` | `types/` | Type + RawBytes + AttrsWire + WireUpdate + MessageID + Direction + Meta | Event dispatch bridge between reactor and plugins |

`RawMessage` (with its `AttrsWire` and `WireUpdate`) is the right carrier for structured delivery: it already exists at dispatch time, provides lazy per-attribute parsing, and avoids eager `FilterResult` computation. `FilterResult` is only needed for the text formatting path (external plugins, monitors).

**Behavior to preserve:**
- External/forked plugins continue receiving JSON text via socket
- `OnEvent` callback remains functional for plugins that don't opt into structured delivery
- `StructuredUpdate` pool in events.go (zero-alloc hot path for rs)
- Format cache for text consumers (monitors, external plugins)
- Event delivery ordering guarantees (per-process channel)
- `RawMessage.IsAsyncSafe()` contract -- received UPDATEs have zero-copy RawBytes that may be reused

**Behavior to change:**
- Internal plugins switch from `OnEvent` + `ParseEvent` to `OnStructuredEvent` with pre-built structured data
- Engine builds structured payload for internal plugin consumers, skipping JSON formatting for those processes

## Data Flow (MANDATORY)

### Current Flow (JSON round-trip)
1. Reactor produces `RawMessage` + `PeerInfo`
2. `events.go:onMessageReceived` iterates subscribed processes
3. For each text consumer: `formatMessageForSubscription` calls `filter.ApplyToUpdate(wire, body, nlriFilter)` -> `FilterResult`
4. `formatFilterResultJSON(peer, result, ...)` serializes `FilterResult` to JSON string. `FilterResult` is discarded
5. `process.Deliver(EventDelivery{Output: jsonString})` enqueues to per-process channel
6. Delivery goroutine calls `bridge.DeliverEvents([]string)` (internal) or `deliverViaConn` (external)
7. SDK `onEvent(string)` callback fires
8. Plugin calls `ParseEvent([]byte(jsonStr))` -- 5+ `json.Unmarshal`, reconstructs what `FilterResult` already had
9. Plugin calls `event.GetPeerAddress()` etc. -- each re-unmarshals `event.Peer`

### Proposed Flow (structured delivery)
1. Reactor produces `RawMessage` + `PeerInfo` (same)
2. `events.go:onMessageReceived` iterates subscribed processes
3. For structured consumers: build `*StructuredEvent` from `PeerInfo` metadata fields + `RawMessage` pointer. No `filter.ApplyToUpdate`, no JSON serialization
4. `process.Deliver(EventDelivery{Event: structuredEvent})` enqueues
5. Delivery goroutine calls `bridge.DeliverStructured([]any)`
6. SDK `onStructuredEvent([]any)` callback fires
7. Plugin type-asserts to `*StructuredEvent`, reads metadata fields directly, calls `AttrsWire`/`WireUpdate` lazy accessors for only the data it needs

**Key difference:** Steps 3-4 (JSON serialize) and 8-9 (JSON deserialize + re-unmarshal) are eliminated entirely. The engine does zero parsing for structured consumers -- it just wraps existing references. Each plugin does only the minimum parsing it needs via lazy `AttrsWire` accessors.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine -> Internal Plugin | `DeliverStructured([]any)` via DirectBridge (function call, no I/O) | [ ] |
| Engine -> External Plugin | `DeliverEvents([]string)` via socket (unchanged) | [ ] |
| `RawMessage` -> `StructuredEvent` | Pointer assignment (no copy, no serialization) | [ ] |
| `AttrsWire` -> Plugin attribute access | Lazy `Get(code)` per attribute (zero-copy index + on-demand parse) | [ ] |

### Integration Points
- `events.go:onMessageReceived` - dispatch point, builds structured payloads
- `events.go:onMessageBatchReceived` - batch dispatch, same pattern
- `events.go:onSentUpdate` - sent events
- `events.go:onRefreshReceived` - refresh events
- `process.EventDelivery.Event` - carries structured payload (already `any`)
- `bridge.DeliverStructured` - already exists
- `sdk.OnStructuredEvent` - already exists

### Architectural Verification
- [ ] No bypassed layers (structured delivery uses existing DirectBridge path)
- [ ] No unintended coupling (StructuredEvent in `pkg/plugin/rpc/` -- same as StructuredUpdate)
- [ ] No duplicated functionality (replaces JSON format+parse, doesn't add alongside)
- [ ] Zero-copy preserved (RawMessage passed by pointer, wire bytes not copied)

## Design

### Core Principle: No Parsing the Engine Already Did, No Parsing the Plugin Won't Use

The current JSON path does redundant work:
1. Engine parses wire bytes into typed Go objects (`filter.ApplyToUpdate` -> `FilterResult`)
2. Engine serializes those objects to JSON text (`formatFilterResultJSON`)
3. Plugin deserializes JSON text back into typed Go objects (`ParseEvent` -> `*Event`)

Steps 2 and 3 are pure waste. But step 1 (`ApplyToUpdate` with `FilterModeAll`) also parses more than some plugins need -- it calls `wire.All()` which parses every attribute into typed objects.

The right approach: deliver the **lazy** data (`AttrsWire`, `WireUpdate`) directly. Each plugin calls only the accessors it needs. `AttrsWire` already supports per-attribute lazy parsing -- `Get(AttrOrigin)` parses only ORIGIN, `GetRaw(AttrMPReachNLRI)` returns raw bytes without parsing.

**Do NOT deliver `FilterResult`.** `FilterResult` forces eager parsing of all attributes (`wire.All()`). Instead, deliver the raw wire wrappers and let each plugin pull what it needs.

### Phase 1: StructuredEvent Type + Plugin Migration

**New type** in `pkg/plugin/rpc/bridge.go` (alongside existing `StructuredUpdate`):

| Field | Type | Source | Who needs it |
|-------|------|--------|-------------|
| PeerAddress | string | `peer.Address.String()` | all |
| PeerName | string | `peer.Name` | rpki, persist |
| PeerGroup | string | `peer.GroupName` | persist |
| PeerAS | uint32 | `peer.PeerAS` | rib, rpki, persist |
| LocalAS | uint32 | `peer.LocalAS` | rib |
| LocalAddress | string | `peer.LocalAddress.String()` | rib |
| EventType | string | `messageTypeToEventType(msg.Type)` | all |
| Direction | string | `msg.Direction` | rib, adj-rib-in, persist |
| MessageID | uint64 | `msg.MessageID` | rib, rpki, persist, rs |
| State | string | peer.State or state event payload | rib, gr, watchdog, persist |
| Reason | string | state event reason | gr |
| RawMessage | `*RawMessage` | passed by pointer | rs (zero-copy forwarding), rib (raw bytes for pool), adj-rib-in |
| Meta | `map[string]any` | `msg.Meta` | rib (route metadata) |

For UPDATE events, `RawMessage` carries `AttrsWire` (lazy attribute access) and `WireUpdate` (lazy section access). Each plugin calls only the accessors it needs:

| Plugin | What it calls on RawMessage/AttrsWire/WireUpdate |
|--------|--------------------------------------------------|
| **rib handleSent** | `AttrsWire.Get(AttrOrigin)`, `.Get(AttrASPath)`, `.Get(AttrMED)`, `.Get(AttrLocalPref)`, `.Get(AttrCommunities)`, etc. -- only the attributes it stores |
| **rib handleReceived** | `WireUpdate.Attrs()` -> `GetRaw(...)` for raw bytes, `WireUpdate.NLRI()`, `.Withdrawn()`, `.MPReach()`, `.MPUnreach()` for raw NLRI bytes -- no attribute parsing at all |
| **adj-rib-in** | Same as rib handleReceived (raw bytes for hex replay) |
| **rpki** | `AttrsWire.Get(AttrASPath)` only -- skips all other attributes |
| **rs** | `RawMessage` pointer directly -- zero-copy wire forwarding, no parsing |
| **gr** | No `RawMessage` (subscribes to open/state/eor, not updates) |
| **watchdog** | No `RawMessage` (subscribes to state only) |
| **persist** | `WireUpdate` NLRI iteration for route tracking |

This means:
- `rpki` parses only AS_PATH (1 attribute), not all 7+ attributes
- `rib handleReceived` does zero attribute parsing (reads raw bytes directly from `WireUpdate`)
- `rib handleSent` parses the ~7 attributes it stores, but lazily via `AttrsWire.Get()` instead of `wire.All()`
- `watchdog` and `gr` get no `RawMessage` at all -- only metadata fields

For non-UPDATE events (state, open, refresh, eor), `RawMessage` is nil and the lightweight metadata fields (State, Reason, etc.) carry the event data.

**`StructuredUpdate` is superseded** by `StructuredEvent`. The `rs` plugin migrates to the new type (reads `RawMessage` field instead of `StructuredUpdate.Event`). `StructuredUpdate` is deleted (no layering rule).

**Plugin migration:** Each internal plugin replaces `OnEvent` + `ParseEvent` with `OnStructuredEvent` + type assertion to `*StructuredEvent`. Plugins read fields directly from `StructuredEvent` metadata and call `AttrsWire`/`WireUpdate` accessors for UPDATE data.

**RIB plugin refactor:**
- `handleSent` (ribOut): Currently reads `event.Origin` (string), `event.ASPath` ([]uint32), etc. Changes to: `msg.AttrsWire.Get(AttrOrigin)` -> type-assert to `*attribute.Origin` -> read `.Value()`. Same data, no JSON round-trip, and lazy (only parses what it stores).
- `handleReceived` (ribIn/pool): Currently reads `event.RawAttributes` (hex string from JSON), `event.RawNLRI` (hex string per family). Changes to: `msg.WireUpdate.Attrs()` for raw attribute bytes, `msg.WireUpdate.MPReach()` / `msg.WireUpdate.NLRI()` for raw NLRI bytes. Eliminates the hex-encode-in-JSON then hex-decode-in-plugin round-trip.

**Engine-side change in events.go:** For structured consumers, the engine delivers `StructuredEvent` with `RawMessage` pointer. No `filter.ApplyToUpdate`, no `formatFilterResultJSON`. The text path for external plugins/monitors is unchanged.

**Import concern:** `RawMessage` lives in `internal/component/bgp/types/`. The `StructuredEvent` type in `pkg/plugin/rpc/` (public) cannot import it. Use `any` for the `RawMessage` field (same pattern as existing `StructuredUpdate.Event any`). Plugins type-assert -- they already import the types package.

### Phase 2: Field-Needs Declaration (Optimization)

With phase 1, the engine always populates `RawMessage` on `StructuredEvent` for UPDATE events. But plugins like `watchdog` (state only) and `gr` (open/state/eor only) don't subscribe to updates, so they already get no `RawMessage`. The lazy parsing in `AttrsWire` means unused attributes cost nothing.

The remaining optimization: the engine currently computes `AttrsWire` for every UPDATE (in session_read.go). If no structured consumer needs attributes (e.g., only `rs` subscribed to updates -- it does zero-copy forwarding), the engine could skip `AttrsWire` construction entirely.

| Declaration | What engine skips |
|-------------|-------------------|
| `"raw-message"` | Deliver `RawMessage` with `AttrsWire` (default for UPDATE subscribers) |
| `"wire-only"` | Deliver `RawMessage` without `AttrsWire` construction (rs -- needs only wire bytes) |
| no UPDATE subscription | No `RawMessage` at all (watchdog, gr) |

This is a narrow optimization. Phase 1 delivers the bulk of the benefit by eliminating the JSON round-trip. Phase 2 is worth investigating only if profiling shows `AttrsWire` index construction is significant for the rs-only case.

**API addition (phase 2 only):** `SetStructuredFields([]string)` on the SDK. The engine checks the union of field needs across subscribed structured consumers to decide whether to construct `AttrsWire`.

### IsAsyncSafe Concern

`RawMessage.IsAsyncSafe()` returns false for zero-copy received UPDATEs (wire bytes may be reused by the read buffer). `AttrsWire` and `WireUpdate` contain zero-copy slices into these same bytes. Plugins must either:
- Process synchronously within the handler (same guarantee as current `rs` path)
- Copy the data they need before returning from the handler

This is the same contract as the existing `rs` structured path. Document it on `StructuredEvent`.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with rib plugin + peer | -> | `rib.OnStructuredEvent` receives `*StructuredEvent` | `test/plugin/rib-structured-delivery.ci` |
| Config with adj-rib-in plugin + peer | -> | `adj_rib_in.OnStructuredEvent` receives `*StructuredEvent` | `test/plugin/adj-rib-structured.ci` |
| Config with rs plugin + peer | -> | `rs.OnStructuredEvent` receives `*StructuredEvent` (migrated from StructuredUpdate) | existing `test/plugin/rs-*.ci` tests |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Internal plugin with `OnStructuredEvent` receives UPDATE | Plugin receives `*StructuredEvent` with PeerAddress, PeerAS, MessageID, Direction, RawMessage populated -- no JSON parsing. Plugin accesses attributes via `AttrsWire.Get()` (lazy, per-attribute) |
| AC-2 | Internal plugin with `OnStructuredEvent` receives state event | Plugin receives `*StructuredEvent` with PeerAddress, State populated |
| AC-3 | External/forked plugin receives UPDATE | Plugin receives JSON text string via `OnEvent` (unchanged behavior) |
| AC-4 | `rib` plugin processes UPDATE via structured delivery | Routes stored correctly in ribIn and ribOut (same behavior as JSON path) |
| AC-5 | `rs` plugin processes UPDATE via structured delivery | Routes forwarded correctly (same behavior as current StructuredUpdate path) |
| AC-6 | Mixed internal + external plugins subscribed to same event | Internal gets structured, external gets JSON text, both receive the event |
| AC-7 | Benchmark: structured path for rpki (1 attribute) | At least 10x fewer allocations than JSON `ParseEvent` path (parses 1 attr vs 5+ unmarshals) |
| AC-8 | `StructuredUpdate` type deleted | No references to `StructuredUpdate` in codebase (replaced by `StructuredEvent`) |
| AC-9 | `RawMessage` async safety | `StructuredEvent` documents the `IsAsyncSafe` contract; plugins that store data copy what they need |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStructuredEventFromPeerInfo` | `pkg/plugin/rpc/bridge_test.go` | StructuredEvent construction from PeerInfo + FilterResult | |
| `TestStructuredEventPool` | `pkg/plugin/rpc/bridge_test.go` | Pool get/put cycle, field clearing | |
| `TestDeliverStructuredEvent` | `internal/component/plugin/process/delivery_test.go` | sendBatch routes to DeliverStructured for StructuredEvent | |
| `TestRIBStructuredDispatch` | `internal/component/bgp/plugins/rib/rib_test.go` | RIB dispatch from StructuredEvent with FilterResult produces same routes as JSON path | |
| `TestMixedDelivery` | `internal/component/plugin/process/delivery_test.go` | Batch with structured + text consumers delivers correctly to both | |

### Benchmark Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `BenchmarkStructuredVsJSON` | `internal/component/bgp/server/events_bench_test.go` | Structured delivery path has fewer allocs than JSON format+parse path | |

### Boundary Tests (MANDATORY for numeric inputs)
No new numeric inputs introduced.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rib-structured-delivery` | `test/plugin/rib-structured-delivery.ci` | Peer sends UPDATE, rib stores route, CLI query returns it | |
| `adj-rib-structured` | `test/plugin/adj-rib-structured.ci` | Peer sends UPDATE, adj-rib-in stores raw hex, replay works | |
| existing rs tests | `test/plugin/rs-*.ci` | Route server forwards after migration to StructuredEvent | |

### Future (if deferring any tests)
- Phase 2 field-needs declaration tests -- deferred to phase 2 spec

## Files to Modify

- `pkg/plugin/rpc/bridge.go` - Add `StructuredEvent` type, delete `StructuredUpdate`, update pool
- `pkg/plugin/sdk/sdk_callbacks.go` - No change needed (OnStructuredEvent already exists)
- `internal/component/bgp/server/events.go` - Build `StructuredEvent` instead of `StructuredUpdate` for structured consumers
- `internal/component/bgp/plugins/rib/rib.go` - Switch to `OnStructuredEvent`, refactor dispatch to read from `FilterResult` instead of `Event`
- `internal/component/bgp/plugins/rib/event.go` - Update aliases if needed
- `internal/component/bgp/plugins/adj_rib_in/rib.go` - Switch to `OnStructuredEvent`
- `internal/component/bgp/plugins/gr/gr.go` - Switch to `OnStructuredEvent`
- `internal/component/bgp/plugins/rpki/rpki.go` - Switch to `OnStructuredEvent`
- `internal/component/bgp/plugins/watchdog/watchdog.go` - Switch to `OnStructuredEvent`
- `internal/component/bgp/plugins/persist/server.go` - Switch to `OnStructuredEvent`
- `internal/component/bgp/plugins/rs/server.go` - Migrate from `StructuredUpdate` to `StructuredEvent`
- `internal/component/plugin/process/delivery.go` - No change needed (already handles structured path)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | No (internal optimization) | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | Yes | `docs/architecture/api/process-protocol.md` -- document StructuredEvent delivery path |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | Yes | `.claude/rules/plugin-design.md` -- add StructuredEvent to DirectBridge section; `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` -- update event delivery description |

## Files to Create

- `test/plugin/rib-structured-delivery.ci` - Functional test for rib structured path
- `test/plugin/adj-rib-structured.ci` - Functional test for adj-rib-in structured path

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: StructuredEvent type** -- Define `StructuredEvent` in `pkg/plugin/rpc/bridge.go`, replace `StructuredUpdate`
   - Tests: `TestStructuredEventFromPeerInfo`, `TestStructuredEventPool`
   - Files: `pkg/plugin/rpc/bridge.go`, `pkg/plugin/rpc/bridge_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Engine dispatch** -- Update `events.go` to build `StructuredEvent` with `FilterResult` instead of `StructuredUpdate` with `RawMessage`
   - Tests: `TestDeliverStructuredEvent`, `TestMixedDelivery`
   - Files: `internal/component/bgp/server/events.go`, `internal/component/plugin/process/delivery_test.go`
   - Verify: structured consumers receive StructuredEvent, text consumers unaffected

3. **Phase: rs migration** -- Migrate rs from `StructuredUpdate` to `StructuredEvent` (reads RawMessage field)
   - Tests: existing rs tests
   - Files: `internal/component/bgp/plugins/rs/server.go`
   - Verify: existing rs functional tests pass

4. **Phase: rib migration** -- Switch rib to `OnStructuredEvent`, refactor handleSent to use `AttrsWire.Get()` per attribute, refactor handleReceived to use `WireUpdate` raw bytes directly
   - Tests: `TestRIBStructuredDispatch`, rib functional tests
   - Files: `internal/component/bgp/plugins/rib/rib.go`
   - Verify: rib functional tests pass, routes stored correctly

5. **Phase: remaining plugins** -- Migrate adj-rib-in, gr, rpki, watchdog, persist
   - Tests: plugin-specific tests
   - Files: each plugin's server/main file
   - Verify: all functional tests pass

6. **Phase: cleanup** -- Delete `StructuredUpdate`, remove dead JSON-only code paths if any
   - Tests: `make ze-verify`
   - Verify: no references to `StructuredUpdate`, all tests pass

8. **Functional tests** -- Create after feature works. Cover user-visible behavior.
9. **Full verification** -- `make ze-verify`
10. **Complete spec** -- Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every plugin migrated, StructuredUpdate deleted |
| Correctness | Routes stored/forwarded identically to JSON path (diff test output) |
| Naming | `StructuredEvent` consistent everywhere, no leftover `StructuredUpdate` references |
| Data flow | RawMessage passed by pointer, no unnecessary copies |
| Rule: no-layering | `StructuredUpdate` fully removed, not kept alongside `StructuredEvent` |
| Rule: buffer-first | No new `[]byte` allocations in hot path |
| Async safety | All plugins respect `IsAsyncSafe` contract |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `StructuredEvent` type in bridge.go | `grep "type StructuredEvent struct" pkg/plugin/rpc/bridge.go` |
| `StructuredUpdate` deleted | `grep -r "StructuredUpdate" --include="*.go"` returns nothing |
| All 7 internal plugins use `OnStructuredEvent` | `grep "OnStructuredEvent" internal/component/bgp/plugins/*/` |
| Rib reads from `AttrsWire`/`WireUpdate` not `ParseEvent` | `grep "parseEvent" internal/component/bgp/plugins/rib/rib.go` returns nothing for UPDATE path |
| Benchmark shows improvement | `go test -bench=BenchmarkStructuredVsJSON` output |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | StructuredEvent fields populated from trusted engine data (PeerInfo, RawMessage) -- no external input |
| Async safety | Plugins that store RawMessage data must copy before returning from handler |
| Pool safety | StructuredEvent pool clears all fields on put (no stale data leaks) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

- The existing `StructuredUpdate` in `rs` proves the pattern works. This spec generalizes it.
- `AttrsWire` already supports lazy per-attribute parsing. Delivering it directly lets each plugin parse only what it needs: rpki parses 1 attribute (AS_PATH), rib handleReceived parses 0 (reads raw bytes), rib handleSent parses ~7.
- `ParseEvent` does 5+ `json.Unmarshal` calls because the JSON format has nested wrappers (`type/bgp/update/nlri`). All of this reconstructs data that `AttrsWire`/`WireUpdate` already provide via lazy accessors.
- `GetPeerAddress()` / `GetPeerASN()` / `GetPeerName()` each re-unmarshal `event.Peer` (a `json.RawMessage`). With structured delivery, these become direct field reads on `StructuredEvent`.
- The rib handleReceived path currently hex-encodes raw bytes into JSON then hex-decodes them back. With `WireUpdate`, it reads the raw bytes directly -- eliminating both the hex encoding and decoding.
- For structured-only events (no text consumers), the engine skips `filter.ApplyToUpdate` AND `formatFilterResultJSON` entirely -- it just wraps existing `RawMessage` reference in `StructuredEvent`. Zero per-event work on the engine side.
- `AttrsWire` index construction (walking wire bytes to find attribute offsets) happens once on first accessor call and is cached. Multiple plugins sharing the same `AttrsWire` (via `RawMessage`) benefit from this cache -- the second plugin's `Get()` call hits the index, not the wire.

## RFC Documentation

No RFC changes -- internal optimization only.

## Implementation Summary

### What Was Implemented
- [To be filled]

### Bugs Found/Fixed
- [To be filled]

### Documentation Updates
- [To be filled]

### Deviations from Plan
- [To be filled]

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
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.

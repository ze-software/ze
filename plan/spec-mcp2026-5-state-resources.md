# Spec: mcp2026-5-state-resources

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | spec-mcp2026-1-stateless-core |
| Phase | - |
| Deferral shard | `plan/deferrals/mcp2026-0-umbrella.md` |
| Updated | 2026-07-28 |

Deferral holder. Source: `plan/spec-mcp2026-0-umbrella.md` (owner question,
2026-07-28: "should `subscriptions/listen` be used to inform the client of event
bus messages?"). Not part of the MCP `2026-07-28` conformance cutover, which is
why it holds no phase number.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Surface live Ze state to MCP clients, so an agent can react to what the daemon
is doing rather than only polling it with tool calls.

**The design is already fixed by the umbrella's Key Design Decision, and the
reasoning should not be re-litigated without new information:** model the
interesting state as MCP **resources** under a `ze://` URI space, let clients
subscribe with `resourceSubscriptions` on `subscriptions/listen`, and let the
event bus fire `notifications/resources/updated` so the agent re-reads. Do not
pipe the event bus through a Ze-defined notification type.

Three reasons, from the umbrella:

| Reason | Detail |
|--------|--------|
| Shape | `notifications/resources/updated` carries only the changed URI, so the client re-reads. That is naturally coalescing, which is what an agent wants and what a raw event stream is not |
| Volume | The bus carries `*BestChangeBatch` (`internal/core/bgp/ribevents/ribevents.go`), which is BGP best-path churn and can be thousands of events per second. SSE to an LLM client is the wrong sink |
| Coupling | Bus payloads are internal typed Go values. A Ze-specific notification type would freeze them into a public wire contract that no third-party client understands anyway (`ai/rules/plugins.md`, cross-boundary value types) |

The rejected alternative stays on the record: a Ze extension
(`io.ze-software/events`) adding its own filter field and notification type is
**permitted** by the extension mechanism, exactly as the Tasks extension adds
`notifications/tasks`. It becomes worth doing only if a first-party client
justifies it. A third-party MCP client would ignore it.

**Work this spec must cover:**

1. **Implement `subscriptions/listen`.** Not implemented by the cutover (umbrella A-4, confirmed: there was nothing to advertise). The request opens a long-lived SSE response stream; the server MUST send `notifications/subscriptions/acknowledged` first, carrying `io.modelcontextprotocol/subscriptionId` in `_meta`, and MUST NOT send notification types the client did not request. The acknowledgment's `notifications` field reflects only the subset the server honours. Graceful closure sends the empty `subscriptions/listen` response before closing.
2. **Choose the state worth exposing.** The low-rate transitions are the useful ones. Candidates from the registered typed events: protocol session up/down (`internal/plugins/isis/events.go`, `internal/plugins/ldp/events.go`, `internal/plugins/rsvpte/events.go`), OSPF neighbour and interface state (`internal/plugins/ospf/events.go`), VRRP state change (`internal/plugins/vrrp/telemetry.go`). The route-change batches are explicitly **not** candidates.
3. **Design the `ze://` URI space.** It must be stable, enumerable through `resources/list`, and readable through `resources/read`. Note the existing `ui://` scheme (`resources.go`) is a separate namespace serving embedded assets; this adds a second one backed by live state rather than an embedded FS, which is a structural change to `resources.go`.
4. **Bridge the event bus to resource-updated notifications.** `EventBus.Subscribe` handlers "run synchronously when an event is emitted and MUST NOT block on I/O" (`pkg/ze/eventbus.go`), so the bridge must hand off to the subscription stream without blocking the emitting goroutine, and must coalesce: many bus events for one resource collapse into one `notifications/resources/updated`.
5. **Bound it.** Long-lived streams per client, subscription counts, and the coalescing buffer all need caps. The cutover deleted the session caps that used to bound concurrent client state, so this is the first feature to reintroduce long-lived per-client state and it must carry its own limits.
6. **Decide the authorization model.** A subscription outlives the request that created it. Whether a revoked identity's stream is torn down, and how, is a real question that per-request auth does not answer by itself.

**Known constraint:** `notifications/tasks` (the Tasks extension's push path) also
rides `subscriptions/listen`. If this spec lands, Phase 3's polling-only decision
becomes revisitable, though polling remains the spec default and is not wrong.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - the event bus design
- [ ] `docs/architecture/mcp/overview.md` - resources model
- [ ] `ai/rules/plugins.md` - cross-boundary value types

### Protocol Specification (Scope: protocol)
- [ ] `https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/subscriptions` - filter, acknowledgment, subscription IDs, graceful closure
- [ ] `https://modelcontextprotocol.io/specification/2026-07-28/server/resources` - resource model and `notifications/resources/updated`

**Key insights:** (fill during design)

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `pkg/ze/eventbus.go` - the bus interface and its non-blocking handler contract
- [ ] `internal/core/events/typed.go` - typed event registration
- [ ] `internal/component/mcp/resources.go` - the embedded-FS resource server this extends
- [ ] `internal/core/bgp/ribevents/ribevents.go` - the high-volume events to exclude

**Behavior to preserve:**
- The `ui://` scheme and its embedded-FS serving path, unchanged by adding a second scheme.
- The event bus contract: subscribers run synchronously and must not block.

**Behavior to change:**
- Additive only. Nothing existing changes behaviour.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- HTTP POST `subscriptions/listen` carrying a `notifications` filter with `resourceSubscriptions` listing `ze://` URIs.
- Internally: `EventBus.Emit` from any publishing component.

### Transformation Path
1. Subscription registered, acknowledgment sent with the honoured subset.
2. Event bus handler fires, maps the event to a `ze://` resource URI, coalesces.
3. `notifications/resources/updated` written to the subscription's SSE stream.
4. Client calls `resources/read` on the URI to get current state.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Event bus ↔ MCP subscription stream | Non-blocking handoff with coalescing | No |
| Ze state ↔ MCP resource | `ze://` URI space, read on demand | No |

### Integration Points
- `internal/component/mcp/resources.go` - second URI scheme
- `pkg/ze/eventbus.go` - subscription registration
- Phase 3's `notifications/tasks`, which shares the same stream mechanism

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | |
| No unintended coupling | No | |
| No duplicated functionality | No | |
| Zero-copy preserved where applicable | No | |
| Registration over hardcoding | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The useful state is low-rate enough for a change-signal model | Session up/down and interface transitions are human-timescale events | Coalescing has to be much more aggressive, or the feature is not worth it | Measure event rates during design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The bridge blocks an emitting goroutine, stalling a protocol engine | A protocol plugin slows under an attached MCP subscriber | Non-blocking handoff with a bounded buffer; drop-and-coalesce rather than block |
| R-2 | Long-lived streams reintroduce the unbounded per-client state the cutover removed | Memory grows with attached agents | Explicit caps, designed in from the start |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | R-1 is the serious one: a badly built bridge can stall a routing protocol engine from an MCP client. Everything else is confined to the MCP listener |
| How is it reverted? | Single revert; the feature is additive |
| Who else touches this path? | Every component that publishes on the event bus |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Client POSTs `subscriptions/listen` with a `ze://` URI, then a protocol session goes down | → | event bus → bridge → `notifications/resources/updated` | `test/plugin/mcp-subscribe-session-state.ci` |
| Client subscribes to a type the server does not honour | → | acknowledgment subset logic | `test/plugin/mcp-subscribe-unsupported-type.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `subscriptions/listen` with `resourceSubscriptions` | First message is `notifications/subscriptions/acknowledged` carrying `subscriptionId` in `_meta` |
| AC-2 | A subscribed resource's underlying state changes | `notifications/resources/updated` carrying the URI |
| AC-3 | A notification type the client did not request | Never sent |
| AC-4 | Many bus events for one resource in quick succession | Coalesced |
| AC-5 | Server shutdown | Empty `subscriptions/listen` response sent before the stream closes |
| AC-6 | An emitting goroutine with a slow or stalled subscriber | Never blocks (R-1) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Subscribes an agent to peer state and is notified when a session drops | `subscriptions/listen` → event bus → updated notification → `resources/read` | `test/plugin/mcp-subscribe-session-state.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSubscriptionAcknowledgedFirst` | `internal/component/mcp/subscriptions_test.go` | AC-1 | |
| `TestUnrequestedTypeNeverSent` | `internal/component/mcp/subscriptions_test.go` | AC-3 | |
| `TestEventCoalescing` | `internal/component/mcp/subscriptions_test.go` | AC-4 | |
| `TestEmitNeverBlocks` | `internal/component/mcp/subscriptions_test.go` | AC-6 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| (fill during design: max subscriptions, coalescing buffer) | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `mcp-subscribe-session-state` | `test/plugin/*.ci` | Agent notified of a session drop | |
| `mcp-subscribe-unsupported-type` | `test/plugin/*.ci` | Unsupported type omitted from the acknowledgment | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | No third-party MCP peer in-tree | |

## Files to Modify
- `internal/component/mcp/resources.go` - second URI scheme
- `internal/component/mcp/discover.go` - capability advertisement

## Files to Create
- `internal/component/mcp/subscriptions.go` - `subscriptions/listen`
- `internal/component/mcp/statebridge.go` - event bus to resource-updated bridge
- `test/plugin/*.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| (fill during design) | | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| (fill during design) | | | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - register `subscriptions/listen`, failing wiring test.
2. (fill during design)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | The bridge cannot block an emitting goroutine under any subscriber behaviour |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| (fill during design) | |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Subscription authorization | What happens to a live stream when the subscribing identity's access is revoked |
| Resource exhaustion | Caps on concurrent subscriptions and buffered events per client |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails on behavior mismatch | Re-read Current Behavior; if misunderstood → RESEARCH |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- fill during design -->

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Resources plus `resourceSubscriptions`, not a Ze notification type | A `io.ze-software/events` extension; no push at all | Inherited from `plan/spec-mcp2026-0-umbrella.md`; see the Task section for the three reasons |

## Known Limitations
- (fill during design)

## RFC Documentation (Scope: protocol)

Add `// MCP 2026-07-28 basic/patterns/subscriptions Section X: "<quoted requirement>"`
above the acknowledgment ordering, the unrequested-type prohibition, and the
graceful-closure path.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify current mode full` passes
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Critical Review passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/speclifecycle/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only

# Spec: Unified Subscriber Session Model

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-05-22 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/l2tp/events/events.go` - current L2TP event handles
4. `internal/component/l2tp/handler_registry.go` - current L2TP handler pattern
5. `internal/component/ppp/start_session.go` - PPP transport-agnostic boundary
6. `internal/component/pppoe/session.go` - current PPPoE session state
7. `internal/core/events/typed.go` - typed event infrastructure

## Task

Ze has three independent session models (PPPoE, L2TP, PPP) with no shared subscriber
type. PPPoE does not emit EventBus events. Auth, pool, shaping, accounting, route
injection, and CoA are wired only to L2TP. PPPoE sessions are invisible to
downstream plugins.

This spec introduces:
1. A shared `Session` struct that all access types project onto
2. A `subscriber` event namespace with lifecycle events that PPPoE and L2TP both emit
3. Transport-generic handler registries (auth, pool, shaping) that any access type can use
4. Show/CLI/telemetry integration via the shared model
5. CoA/Disconnect-Message support for PPPoE sessions
6. Session telemetry (counts by access type, auth outcomes, lifecycle)

The PPP engine (`ppp.StartSession`, `ppp.Driver`) is already shared and stays as-is.
This spec works above and below that boundary to unify the access-type-specific layers.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/l2tp.md` - L2TP subsystem design
  -> Decision:
  -> Constraint:
- [ ] `docs/architecture/core-design.md` - registration and bus patterns
  -> Decision:
  -> Constraint:
- [ ] `ai/patterns/registration.md` - handler registration pattern
  -> Decision:
  -> Constraint:

**Key insights:**
- L2TP events use `core/events.Register[T]` typed handles (emit/subscribe, no type assertions)
- L2TP handler registry uses package-level `Register*` + `Get*` functions with a mutex
- `ppp.StartSession` is already transport-agnostic; both PPPoE and L2TP use it
- PPP events (`EventSessionUp`, `EventAuthRequest`, etc.) flow from the PPP driver to the transport's reactor goroutine, which translates them into EventBus events (L2TP does this, PPPoE does not)
- `RegisterAuthHandler` callers: `l2tpauthlocal/register.go:24`, `l2tpauthradius/register.go:37`
- `RegisterPoolHandler` callers: `l2tppool/register.go`
- `RegisterPrefixHandler` callers: `l2tppool/register.go`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/pppoe/session.go` - PPPoE session: SID, MAC, IfName, ServiceName, HostUniq, PppoxFD, UnitNum, State (3-value), CreatedAt. No username, IP, VRF, AAA.
  -> Constraint: PPPoE session table is per-interface with bitmap SID allocator. Must preserve this.
- [ ] `internal/component/pppoe/server.go` - PPPoE server: handles PADI/PADR/PADT, kernel PPPoX setup, sends StartSession to PPP. No EventBus emission.
  -> Constraint: PPPoE server handles discovery + kernel setup. StartSession is the handoff to PPP.
- [ ] `internal/component/l2tp/events/events.go` - L2TP events: SessionDown, SessionUp, SessionIPAssigned, SessionRateChange, EchoRTT, TunnelUp, TunnelDown, RouteChange
  -> Constraint: L2TP events carry (TunnelID, SessionID) as key. Downstream plugins (pool, shaper, RADIUS acct) key on this tuple.
- [ ] `internal/component/l2tp/handler_registry.go` - L2TP handler registry: RegisterAuthHandler, RegisterPoolHandler, RegisterPrefixHandler, RegisterPoolStatsProvider. 5 references to RegisterAuthHandler across 4 files.
  -> Constraint: L2TP plugins register handlers at init() time. Must not break existing L2TP plugin wiring.
- [ ] `internal/component/l2tp/handler.go` - Handler types: AuthHandler receives `ppp.EventAuthRequest`, PoolHandler receives `ppp.EventIPRequest`. These PPP event types are already transport-generic.
  -> Constraint: Handler function signatures are transport-agnostic at the type level.
- [ ] `internal/component/ppp/start_session.go` - StartSession: carries PPPoE-specific fields (AccessInterface, SubscriberMAC, ServiceName, VendorTags) alongside L2TP fields (TunnelID, SessionID, ProxyLCP).
  -> Constraint: PPP engine is transport-agnostic. Do not add transport-specific logic to PPP.
- [ ] `internal/core/events/typed.go` - Typed event handles: Register[T], Event[T].Emit, Event[T].Subscribe. Package-level registration, safe for init().
  -> Constraint: Follow existing pattern for new event namespace.

**Behavior to preserve:**
- L2TP plugins (l2tpauthradius, l2tppool, l2tpshaper) continue to work via L2TP events
- PPPoE session table with per-interface bitmap SID allocator unchanged
- PPP engine is untouched; StartSession is already the right boundary
- L2TP CQM (echo RTT aggregation) continues to work
- L2TP route redistribution (RouteChange events) continues to work

**Behavior to change:**
- PPPoE server emits subscriber lifecycle events after PPP session events
- Auth, pool, and shaping become available to PPPoE sessions
- Show commands expose sessions from all access types through one model
- CoA reaches PPPoE sessions

## Data Flow (MANDATORY)

### Entry Point
- PPPoE: Ethernet frame on raw socket -> discovery -> kernel PPPoX -> `ppp.StartSession`
- L2TP: UDP packet -> AVP decode -> tunnel FSM -> session FSM -> kernel PPPoL2TP -> `ppp.StartSession`

### Transformation Path
1. Access protocol (PPPoE/L2TP) establishes transport, creates `ppp.StartSession`
2. PPP driver runs LCP, auth, NCP; emits `EventAuthRequest`, `EventIPRequest`, `EventSessionUp`
3. Transport reactor translates PPP events: calls auth handler, pool handler, emits EventBus events
4. **NEW:** Transport reactor also builds a `subscriber.Session` snapshot and emits subscriber lifecycle events
5. **NEW:** Downstream plugins subscribe to `subscriber` namespace events (transport-generic)
6. **NEW:** Show handlers query a subscriber session registry for cross-access-type views

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| PPPoE/L2TP -> PPP | `ppp.StartSession` on `Manager.SessionsIn` | [ ] |
| PPP -> transport reactor | PPP events (`EventAuthRequest`, etc.) on `Driver.EventsOut` | [ ] |
| Transport reactor -> EventBus | `Event[T].Emit(bus, payload)` | [ ] |
| EventBus -> downstream plugins | `Event[T].Subscribe(bus, handler)` | [ ] |
| Transport -> subscriber registry | `Registry.Add/Remove(Session)` | [ ] |

### Integration Points
- `internal/core/events/typed.go` - typed event registration (existing)
- `internal/component/l2tp/events/events.go` - L2TP events (existing, preserved)
- `internal/component/ppp/start_session.go` - PPP boundary (existing, unchanged)
- `internal/component/plugin/registry/registry.go` - plugin registration (existing)

### Architectural Verification
- [ ] No bypassed layers (subscriber events flow through EventBus, not direct calls)
- [ ] No unintended coupling (PPPoE and L2TP remain independent; shared model is a struct)
- [ ] No duplicated functionality (subscriber events complement L2TP events, not replace)
- [ ] Zero-copy preserved where applicable (Session is a snapshot value, not a pointer into live state)

## Design

### 1. Session struct

New package `internal/component/subscriber/` owns the shared types. The struct is a
read-only snapshot, not a live pointer into transport state (per
`feedback_no_cross_boundary_pointers`).

```go
package subscriber

type AccessType string

const (
    AccessPPPoE AccessType = "pppoe"
    AccessL2TP  AccessType = "l2tp"
)

type SessionState string

const (
    StateAuthenticating SessionState = "authenticating"
    StateActive         SessionState = "active"
    StateTerminating    SessionState = "terminating"
)

type Session struct {
    ID              string
    AccessType      AccessType
    State           SessionState
    MAC             net.HardwareAddr
    Username        string
    AccessInterface string
    // PPPoE-specific (zero for non-PPPoE)
    PPPoESID        uint16
    ServiceName     string
    // L2TP-specific (zero for non-L2TP)
    TunnelID        uint16
    SessionID       uint16
    PeerAddr        netip.AddrPort
    // PPP
    PppInterface    string
    NegotiatedMRU   uint16
    AuthMethod      string
    // IP
    IPv4Addr        netip.Addr
    IPv6InterfaceID [8]byte
    IPv6Prefix      netip.Prefix
    DNSPrimary      netip.Addr
    DNSSecondary    netip.Addr
    // Classification
    PoolName        string
    ServiceGroup    string
    // Shaping
    DownloadRate    uint64
    UploadRate      uint64
    // Lifecycle
    ActivatedAt     time.Time
    // RADIUS
    AcctSessionID   string
}
```

A concrete struct, not an interface with getters. Ze needs a serializable value type
for show handlers, telemetry, and (eventually) HA sync. Getters would add indirection
without benefit.

### 2. Subscriber event namespace

New `internal/component/subscriber/events/` following the L2TP event pattern:

```go
package events

const Namespace = "subscriber"

var SessionUp = events.Register[*SessionUpPayload](Namespace, "session-up")
var SessionDown = events.Register[*SessionDownPayload](Namespace, "session-down")
var SessionIPAssigned = events.Register[*SessionIPAssignedPayload](Namespace, "session-ip-assigned")
var SessionRateChange = events.Register[*SessionRateChangePayload](Namespace, "session-rate-change")
var SessionAuthResult = events.Register[*SessionAuthResultPayload](Namespace, "session-auth-result")
```

Payloads carry the subscriber `Session` snapshot (value, not pointer) plus
event-specific fields. L2TP events continue to exist for L2TP-specific consumers
(CQM echo RTT, tunnel lifecycle, route redistribution).

### 3. Transport-generic handler registries

Move the handler pattern from L2TP-scoped to subscriber-scoped. New registration
functions in `internal/component/subscriber/`:

```go
func RegisterAuthHandler(h AuthHandler)
func RegisterPoolHandler(h PoolHandler)
func RegisterPrefixHandler(h PrefixHandler)
func RegisterShaperHandler(h ShaperHandler)
```

Handler function signatures use `ppp.EventAuthRequest` and `ppp.EventIPRequest`
(already transport-generic). The L2TP handler registry (`l2tp.RegisterAuthHandler`
etc.) becomes a thin wrapper that delegates to `subscriber.RegisterAuthHandler`.
Existing L2TP plugins (`l2tpauthradius`, `l2tppool`, `l2tpshaper`) change their
registration call from `l2tp.RegisterAuthHandler` to `subscriber.RegisterAuthHandler`.
The L2TP wrappers remain for any external code, but the canonical registration point
is `subscriber`.

### 4. PPPoE EventBus emission

The PPPoE server's event consumer (currently only handles `EventSessionDown`) is
extended to translate all PPP events into:
- Subscriber auth handler calls (via `subscriber.GetAuthHandler`)
- Subscriber pool handler calls (via `subscriber.GetPoolHandler`)
- Subscriber EventBus events (SessionUp, SessionDown, SessionIPAssigned)
- Subscriber registry Add/Remove calls

This mirrors what the L2TP reactor already does for PPP events, but for PPPoE
sessions. The PPPoE-internal `Session` struct remains unchanged (it tracks discovery
and kernel state); the subscriber `Session` snapshot is built from the combination of
PPPoE session state and PPP driver state.

### 5. Session registry

`internal/component/subscriber/registry.go`:

```go
type Registry struct { ... }

func (r *Registry) Add(s Session)
func (r *Registry) Remove(id string)
func (r *Registry) Get(id string) (Session, bool)
func (r *Registry) All() []Session
func (r *Registry) Count() SessionCounts
func (r *Registry) ByAccessType(t AccessType) []Session
```

Thread-safe. Transports call Add on SessionUp and Remove on SessionDown.
Show handlers (`show subscriber summary`, `show subscriber detail <id>`) read from
this registry.

### 6. CoA for PPPoE

The `l2tpauthradius` CoA handler currently dispatches by (TunnelID, SessionID).
With the subscriber registry, CoA can also match by AcctSessionID or Username,
making it transport-generic. The CoA handler emits `subscriber.SessionRateChange`
events. The PPPoE reactor subscribes to rate-change events and applies TC shaping
updates, the same way L2TP already does via `l2tpshaper`.

### 7. Session telemetry

Metrics derived from the subscriber registry:

```
ze_subscriber_sessions{access_type}              gauge
ze_subscriber_sessions_total{access_type}        counter
ze_subscriber_auth_results{access_type,result}   counter
ze_subscriber_session_duration_seconds            histogram
```

Registered via `ConfigureMetrics` in the subscriber component's plugin registration.

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| PPPoE PADS sent, PPP negotiates, auth succeeds | -> | `subscriber.Registry.Add()` called, `subscriber.SessionUp` emitted | `TestPPPoESessionEmitsSubscriberEvents` |
| L2TP ICCN, PPP negotiates, auth succeeds | -> | `subscriber.Registry.Add()` called, `subscriber.SessionUp` emitted | `TestL2TPSessionEmitsSubscriberEvents` |
| `show subscriber summary` CLI command | -> | `subscriber.Registry.All()` returns both PPPoE and L2TP sessions | `TestShowSubscriberSummary` |
| PPPoE session gets pool address | -> | `subscriber.GetPoolHandler()` called, IP assigned | `TestPPPoESessionGetsPoolAddress` |
| CoA-Request with Acct-Session-Id matching PPPoE session | -> | PPPoE session rate updated | `TestCoAReachesPPPoESession` |
| Prometheus scrape | -> | `ze_subscriber_sessions{access_type="pppoe"}` present | `TestSubscriberMetrics` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | PPPoE session reaches PPP SessionUp | `subscriber.SessionUp` event emitted on EventBus with correct Session snapshot |
| AC-2 | L2TP session reaches PPP SessionUp | `subscriber.SessionUp` event emitted on EventBus with correct Session snapshot |
| AC-3 | PPPoE session tears down | `subscriber.SessionDown` event emitted; session removed from Registry |
| AC-4 | L2TP session tears down | `subscriber.SessionDown` event emitted; session removed from Registry; existing L2TP SessionDown event still emitted |
| AC-5 | PPPoE session with auth handler registered | Auth handler receives `ppp.EventAuthRequest` with PPPoE fields (MAC, AccessInterface, ServiceName) |
| AC-6 | PPPoE session with pool handler registered | Pool handler receives `ppp.EventIPRequest`; IP response flows back to PPP driver |
| AC-7 | `show subscriber summary` | Shows sessions from both PPPoE and L2TP with access type, username, IP, state, duration |
| AC-8 | `show subscriber detail <id>` | Shows full Session struct for the given ID |
| AC-9 | CoA-Request targeting a PPPoE session (by Acct-Session-Id) | Rate change applied; `subscriber.SessionRateChange` emitted |
| AC-10 | Prometheus scrape with active sessions | `ze_subscriber_sessions` gauge present with `access_type` label |
| AC-11 | Disconnect-Message targeting a PPPoE session | Session torn down via LCP Terminate-Request |
| AC-12 | L2TP plugins (l2tpauthradius, l2tppool, l2tpshaper) with new registry | Existing L2TP functionality unbroken |
| AC-13 | PPPoE session with shaper handler registered | Shaper handler called on SessionUp; TC rules applied to pppN interface |
| AC-14 | No auth handler registered, PPPoE session starts | Session proceeds without auth (matches existing L2TP behavior) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSessionSnapshotFromPPPoE` | `internal/component/subscriber/session_test.go` | PPPoE fields map correctly to Session struct | |
| `TestSessionSnapshotFromL2TP` | `internal/component/subscriber/session_test.go` | L2TP fields map correctly to Session struct | |
| `TestRegistryAddRemoveCount` | `internal/component/subscriber/registry_test.go` | Add/Remove/Count/All/ByAccessType | |
| `TestRegistryConcurrent` | `internal/component/subscriber/registry_test.go` | Concurrent Add/Remove/All safe | |
| `TestSubscriberEventsEmitSubscribe` | `internal/component/subscriber/events/events_test.go` | Typed handles emit and subscribe | |
| `TestAuthHandlerRegistration` | `internal/component/subscriber/handler_registry_test.go` | Register/Get for auth | |
| `TestPoolHandlerRegistration` | `internal/component/subscriber/handler_registry_test.go` | Register/Get for pool | |
| `TestShaperHandlerRegistration` | `internal/component/subscriber/handler_registry_test.go` | Register/Get for shaper | |
| `TestL2TPRegistryDelegation` | `internal/component/l2tp/handler_registry_test.go` | L2TP registry delegates to subscriber | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| PPPoE SID | 1-65535 | 65535 | 0 (reserved) | N/A (uint16) |
| Session ID string | non-empty | any | "" | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-subscriber-pppoe-auth` | `test/plugin/subscriber-pppoe-auth.ci` | PPPoE session authenticates via subscriber auth handler, appears in show output | |
| `test-subscriber-l2tp-compat` | `test/plugin/subscriber-l2tp-compat.ci` | L2TP sessions still work with subscriber registry | |
| `test-subscriber-show` | `test/plugin/subscriber-show.ci` | `show subscriber summary` lists both access types | |
| `test-subscriber-metrics` | `test/plugin/subscriber-metrics.ci` | Prometheus metrics include access_type labels | |

### Interop Tests
Not applicable. No wire protocol changes. PPP, L2TP, PPPoE wire behavior unchanged.

### Future
- HA sync of subscriber sessions (depends on HA framework spec)
- IPoE access type (depends on IPoE spec)
- Persistent session state (depends on operational state store spec)

## Files to Modify
- `internal/component/pppoe/server.go` - add EventBus emission, auth/pool/shaper handler calls
- `internal/component/pppoe/subsystem.go` - wire subscriber registry, receive bus reference
- `internal/component/l2tp/handler_registry.go` - delegate to subscriber handler registry
- `internal/component/l2tp/handler.go` - keep types, add delegation note
- `internal/component/l2tp/subsystem.go` - emit subscriber events alongside L2TP events
- `internal/plugins/l2tpauthradius/register.go` - register with subscriber registry (line 37)
- `internal/plugins/l2tpauthlocal/register.go` - register with subscriber registry (line 24)
- `internal/plugins/l2tppool/register.go` - register with subscriber registry
- `internal/plugins/l2tpshaper/register.go` - register with subscriber registry
- `internal/component/plugin/all/all.go` - blank-import subscriber package

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/component/subscriber/schema/` |
| CLI commands/flags | [x] | `internal/component/subscriber/cmd/` |
| CLI grammar (action before identifier) | [x] | `show subscriber summary`, `show subscriber detail <id>` |
| Editor autocomplete | [ ] | N/A (show commands, not config) |
| Functional test for new RPC/API | [x] | `test/plugin/subscriber-*.ci` |
| Doctor check for runtime dependencies | [ ] | N/A |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [x] | show subscriber via CLI/REST/gRPC |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [x] | `docs/guide/subscriber.md` |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [x] | handler registration moved to subscriber |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/research/comparison/osvbng.md` |
| 12 | Internal architecture changed? | [x] | `docs/architecture/subscriber.md` |

## Files to Create
- `internal/component/subscriber/session.go` - Session struct, AccessType, SessionState
- `internal/component/subscriber/registry.go` - thread-safe session registry
- `internal/component/subscriber/handler_registry.go` - transport-generic auth/pool/shaper/prefix registration
- `internal/component/subscriber/events/events.go` - typed EventBus handles
- `internal/component/subscriber/cmd/show.go` - show subscriber summary/detail handlers
- `internal/component/subscriber/schema/` - YANG for show commands
- `internal/component/subscriber/metrics.go` - session telemetry registration
- `internal/component/subscriber/register.go` - component registration
- `internal/component/subscriber/session_test.go`
- `internal/component/subscriber/registry_test.go`
- `internal/component/subscriber/handler_registry_test.go`
- `internal/component/subscriber/events/events_test.go`
- `test/plugin/subscriber-pppoe-auth.ci`
- `test/plugin/subscriber-l2tp-compat.ci`
- `test/plugin/subscriber-show.ci`
- `test/plugin/subscriber-metrics.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7-14. | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring** -- create subscriber package skeleton, register component, write failing wiring tests
   - Tests: `TestPPPoESessionEmitsSubscriberEvents` (fails), `TestShowSubscriberSummary` (fails)
   - Files: `subscriber/register.go`, `subscriber/session.go`, `subscriber/events/events.go`, `subscriber/registry.go`
   - Verify: package compiles, wiring tests fail because feature logic is stub

2. **Phase: Session model and registry** -- implement Session struct, Registry, handler registries
   - Tests: `TestSessionSnapshotFromPPPoE`, `TestSessionSnapshotFromL2TP`, `TestRegistryAddRemoveCount`, `TestRegistryConcurrent`, `TestAuthHandlerRegistration`, `TestPoolHandlerRegistration`, `TestShaperHandlerRegistration`
   - Files: `subscriber/session.go`, `subscriber/registry.go`, `subscriber/handler_registry.go`
   - Verify: all registry and model tests pass

3. **Phase: L2TP delegation** -- migrate L2TP handler registry to delegate to subscriber, preserve backward compat
   - Tests: `TestL2TPRegistryDelegation`, `TestL2TPSessionEmitsSubscriberEvents`
   - Files: `l2tp/handler_registry.go`, `l2tp/subsystem.go`, `l2tpauthradius/register.go` (line 37), `l2tpauthlocal/register.go` (line 24), `l2tppool/register.go`, `l2tpshaper/register.go`
   - Verify: existing L2TP tests pass; L2TP sessions emit subscriber events

4. **Phase: PPPoE integration** -- add EventBus emission, auth/pool/shaper handler calls to PPPoE
   - Tests: `TestPPPoESessionEmitsSubscriberEvents`, `TestPPPoESessionGetsPoolAddress`
   - Files: `pppoe/server.go`, `pppoe/subsystem.go`
   - Verify: PPPoE sessions emit subscriber events, auth/pool work, wiring tests pass

5. **Phase: Show commands** -- implement show subscriber summary/detail
   - Tests: `TestShowSubscriberSummary`
   - Files: `subscriber/cmd/show.go`, `subscriber/schema/`
   - Verify: CLI commands work with both access types

6. **Phase: Telemetry** -- register subscriber metrics
   - Tests: `TestSubscriberMetrics`
   - Files: `subscriber/metrics.go`
   - Verify: Prometheus scrape shows subscriber gauges/counters

7. **Phase: CoA for PPPoE** -- extend CoA handler to match by Acct-Session-Id
   - Tests: `TestCoAReachesPPPoESession`
   - Files: `l2tpauthradius/coa.go`, `pppoe/server.go`
   - Verify: CoA-Request reaches PPPoE session, rate change applied

8. **Functional tests** -- create .ci functional tests
9. **Full verification** -- `make ze-verify`
10. **Complete spec** -- audit, learned summary, delete spec

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-14 has implementation with file:line |
| Correctness | Session snapshot is a value copy, not a pointer into transport state |
| Naming | AccessType constants lowercase. EventBus namespace "subscriber". |
| Data flow | PPP events -> transport reactor -> subscriber Session snapshot -> subscriber events. No PPP changes. |
| CLI grammar | `show subscriber summary`, `show subscriber detail <id>` -- action before identifier |
| Rule: no-cross-boundary-pointers | Session struct is a value type with no pointers into PPPoE/L2TP internal state |
| Rule: backward compat | L2TP plugins still work. L2TP events still emitted. |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `subscriber/session.go` with Session struct | `grep 'type Session struct' internal/component/subscriber/session.go` |
| `subscriber/registry.go` with Add/Remove/All | `grep -c 'func.*Registry.*Add\|Remove\|All' internal/component/subscriber/registry.go` |
| `subscriber/events/events.go` with SessionUp/SessionDown | `grep 'SessionUp\|SessionDown' internal/component/subscriber/events/events.go` |
| PPPoE emits subscriber events | `grep 'subscriber.*Emit\|SessionUp.*Emit' internal/component/pppoe/server.go` |
| L2TP emits subscriber events | `grep 'subscriber.*Emit\|SessionUp.*Emit' internal/component/l2tp/subsystem.go` |
| Show subscriber commands | `grep 'subscriber' internal/component/subscriber/cmd/show.go` |
| Subscriber metrics registered | `grep 'ze_subscriber' internal/component/subscriber/metrics.go` |
| L2TP backward compat | existing L2TP tests pass |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Session ID generation must be unique and unpredictable |
| Resource exhaustion | Registry must handle unbounded session count; telemetry label cardinality bounded by AccessType enum (small set) |
| CoA authentication | CoA-Request validated by RADIUS shared secret before acting (existing l2tpauthradius behavior, verify preserved) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

## RFC Documentation

Not applicable. No RFC behavior changes. PPP, L2TP, PPPoE wire protocols unchanged.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |

### Tests from TDD Plan
| Test | Status | Location | Notes |

### Files from Plan
| File | Status | Notes |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| PPPoE sessions visible to downstream plugins | functional test | `test-subscriber-pppoe-auth` |
| L2TP backward compatibility | functional test | `test-subscriber-l2tp-compat` |
| Cross-access-type show commands | functional test | `test-subscriber-show` |
| Session telemetry | functional test | `test-subscriber-metrics` |
| CoA for PPPoE | unit test | `TestCoAReachesPPPoESession` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |

### Fixes applied

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-14 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
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
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests N/A (no wire protocol changes)
- [ ] Goal Validation table filled

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Summary included in commit

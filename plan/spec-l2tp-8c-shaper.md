# Spec: l2tp-8c -- L2TP Traffic Shaping Plugin + CoA Listener

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-l2tp-8a-auth-pool |
| Phase | 10/11 |
| Updated | 2026-04-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/component/traffic/backend.go` -- Backend interface (Apply, ListQdiscs, Close)
4. `internal/component/traffic/model.go` -- InterfaceQoS, Qdisc, QdiscType, TrafficClass
5. `internal/component/l2tp/events/events.go` -- SessionDown, SessionIPAssigned typed events
6. `internal/component/l2tp/handler_registry.go` -- RegisterAuthHandler/RegisterPoolHandler pattern
7. `internal/component/l2tp/reactor.go` -- handlePPPEvent (line 866), EventSessionUp currently ignored
8. `internal/plugins/l2tpauthradius/register.go` -- RADIUS plugin registration, ConfigureEventBus
9. `internal/component/radius/dict.go` -- packet codes, attribute types

## Task

Implement traffic shaping for L2TP subscriber sessions and a RADIUS
CoA/DM listener for live session changes.

**l2tp-shaper plugin** (`internal/plugins/l2tpshaper/`): subscribes to
EventBus events to apply TC (traffic control) rules on pppN interfaces.
On `(l2tp, session-up)`, applies a TBF or HTB qdisc with the configured
default rate. On `(l2tp, session-rate-change)`, updates the qdisc with
new rate/burst values (driven by RADIUS CoA). On `(l2tp, session-down)`,
removes TC rules. Uses the existing `traffic.Backend` interface for all
TC programming (no direct netlink calls). YANG config under
`l2tp { shaper { ... } }` specifies default rates and qdisc type.

**CoA/DM listener** (`internal/plugins/l2tpauthradius/`): extends the
existing RADIUS plugin with a UDP server on port 3799 (RFC 5176).
Receives CoA-Request packets, validates authenticator and shared secret,
extracts bandwidth attributes (vendor-specific or Filter-Id), matches
to an active session by Acct-Session-Id or User-Name + NAS-Port, and
emits `(l2tp, session-rate-change)` on the EventBus. Disconnect-Message
tears down the matching session via `l2tp.LookupService().TeardownSession()`.
Responds with CoA-ACK/CoA-NAK or Disconnect-ACK/Disconnect-NAK.

**New EventBus events**: `SessionUp` (emitted from reactor on
`ppp.EventSessionUp`) and `SessionRateChange` (emitted by CoA handler).

### Design Decisions

| Decision | Detail |
|----------|--------|
| Reuse traffic.Backend | Shaper calls `traffic.GetBackend().Apply()` per pppN interface. Shares TC abstraction with static traffic-control plugin. Does not duplicate netlink plumbing. |
| CoA in RADIUS plugin | CoA/DM is RADIUS protocol (RFC 5176). The listener lives in l2tp-auth-radius where the client and shared secret already reside. CoA extracts rate attributes and emits EventBus events; shaper reacts. Clean separation. |
| EventBus for session-up | New typed event `SessionUp` in `l2tp/events/events.go`. Reactor emits on `ppp.EventSessionUp`. Carries TunnelID, SessionID, Interface name. Shaper and stats plugins subscribe. |
| EventBus for rate change | New typed event `SessionRateChange` in `l2tp/events/events.go`. CoA handler emits after validating request. Carries SessionID, DownloadRate, UploadRate (bps). Shaper subscribes. |
| TBF default, HTB optional | Simple TBF (single rate limiter) for most subscribers. HTB when multiple classes needed (rare in BNG). Config selects qdisc type. |
| Per-session state in shaper | Shaper maintains `sync.Map` of sessionKey -> applied rate, so session-down cleanup and CoA updates are O(1). |
| DM tears down session | Disconnect-Message calls `l2tp.LookupService().TeardownSession()` directly. No event needed -- session-down event follows naturally from the teardown. |
| CoA port configurable | YANG leaf `l2tp { auth { radius { coa-port } } }`. Default 3799 per RFC 5176. |

## Required Reading

### Architecture Docs
- [ ] `ai/patterns/plugin.md` -- plugin file structure
  -> Constraint: register.go with init(), atomic logger, RunXxxPlugin(conn), CLIHandler closure
  -> Constraint: run `make generate` after creating plugin to update all.go
- [ ] `ai/rules/plugin-design.md` -- plugin design rules
  -> Constraint: plugin name hyphen-form (l2tp-shaper); YANG required for plugins with config
- [ ] `docs/architecture/core-design.md` -- subsystem and plugin patterns
  -> Constraint: plugins discovered via registry; event types registered in events.go
- [ ] `docs/research/l2tpv2-ze-integration.md` -- section 6.3: shaper responsibilities
  -> Decision: subscribe to session-up for initial shaping, handle CoA rate changes, clean up on session-down
  -> Constraint: integration doc expects ConfigRoots: ["l2tp.shaper"], YANG schema
- [ ] `internal/component/traffic/backend.go` -- Backend interface
  -> Constraint: Apply(ctx, map[string]InterfaceQoS) programs qdiscs; GetBackend() returns active backend (may be nil)
  -> Constraint: must call LoadBackend(name) before GetBackend(); "tc" is default on Linux
- [ ] `internal/component/traffic/model.go` -- data model
  -> Constraint: InterfaceQoS{Interface, Qdisc{Type, DefaultClass, Classes}}; QdiscTBF and QdiscHTB available
  -> Constraint: TrafficClass{Name, Rate, Ceil, Priority, Filters}; Rate/Ceil in bps (uint64)
  -> Constraint: ValidateRate() requires rate >= 1

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5176.md` -- Dynamic Authorization Extensions (CoA/DM) -- CREATE
  -> Constraint: CoA-Request code=43, CoA-ACK code=44, CoA-NAK code=45
  -> Constraint: Disconnect-Request code=40, Disconnect-ACK code=41, Disconnect-NAK code=42
  -> Constraint: server listens on UDP 3799 (configurable); request authenticator = MD5(Code+ID+Length+16-zero-octets+Attrs+Secret)
  -> Constraint: session identification: Acct-Session-Id, or User-Name + NAS-Port, or Calling-Station-Id
  -> Constraint: Error-Cause attribute (101) in NAK responses

**Key insights:**
- The existing traffic.Backend interface programs full desired state per interface; shaper builds a single-interface map per session and calls Apply
- CoA/DM is a server-side RADIUS extension (ze listens for incoming requests from RADIUS server); opposite direction from auth/acct which are client-side
- EventSessionUp in PPP fires after all NCPs complete and pppN is configured; safe to program TC at that point
- The reactor currently ignores ppp.EventSessionUp (reactor.go:882 returns immediately); must add EventBus emission
- Session identification for CoA needs Acct-Session-Id which the accounting plugin already generates; shaper needs the session-to-interface mapping

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/traffic/backend.go` -- Backend interface with Apply/ListQdiscs/Close, RegisterBackend factory, GetBackend/LoadBackend/CloseBackend lifecycle
  -> Constraint: GetBackend() returns nil if no backend loaded; shaper must handle nil (log warning, skip shaping)
  -> Constraint: Apply takes full desired state; shaper passes single-interface map per call
- [ ] `internal/component/traffic/model.go` -- InterfaceQoS, Qdisc (TBF/HTB/etc), TrafficClass, TrafficFilter
  -> Constraint: TBF: classless, rate is the only required field; classes empty
  -> Constraint: HTB: classful, default-class + classes with rate/ceil
- [ ] `internal/component/l2tp/events/events.go` -- SessionDown and SessionIPAssigned typed EventBus handles
  -> Constraint: no SessionUp event exists yet; must add
  -> Constraint: pattern: `var SessionUp = events.Register[*SessionUpPayload](Namespace, SessionUpEvent)`
- [ ] `internal/component/l2tp/reactor.go` -- handlePPPEvent ignores EventSessionUp at line 882
  -> Constraint: must add EventBus emission for SessionUp in the same switch case
  -> Constraint: reactor has access to tunnel/session state; can derive pppN interface name from session's unitNum
- [ ] `internal/component/l2tp/handler_registry.go` -- RegisterAuthHandler/RegisterPoolHandler pattern
  -> Constraint: shaper does not need a handler in registry; it reacts to EventBus events only
- [ ] `internal/plugins/l2tpauthradius/register.go` -- RADIUS plugin with ConfigureEventBus
  -> Constraint: already has EventBus wiring via ConfigureEventBus; CoA listener can use same bus
  -> Constraint: already has RADIUS client config (servers, shared secret, timeout); CoA reuses shared secret
- [ ] `internal/component/radius/dict.go` -- packet codes 1-5 and 11; no CoA/DM codes
  -> Constraint: must add codes 40-45 (DM-Request/ACK/NAK, CoA-Request/ACK/NAK)
  -> Constraint: must add Error-Cause attribute type (101)
- [ ] `internal/component/radius/client.go` -- Client is request-response only; no server-side listener
  -> Constraint: CoA listener is new functionality; separate from Client
- [ ] `internal/plugins/l2tppool/register.go` -- reference for EventBus subscription in plugin init
  -> Constraint: pattern: ConfigureEventBus closure calls setEventBus(eb); subscribe in setEventBus
- [ ] `internal/component/l2tp/snapshot.go` -- SessionSnapshot has LocalSID, TunnelLocalTID, Username, AssignedAddr
  -> Constraint: LookupService().LookupSession(sid) returns SessionSnapshot for CoA session matching

**Behavior to preserve:**
- Existing traffic-control plugin continues to work independently (static per-interface QoS)
- Existing SessionDown and SessionIPAssigned EventBus events unchanged
- Existing RADIUS auth/acct functionality in l2tp-auth-radius unchanged
- Existing reactor handlePPPEvent behavior for all event types except EventSessionUp

**Behavior to change:**
- Reactor emits `(l2tp, session-up)` EventBus event on `ppp.EventSessionUp` (currently ignored)
- RADIUS dict gains CoA/DM packet codes (40-45) and Error-Cause attribute (101)
- l2tp-auth-radius plugin gains CoA/DM listener goroutine (UDP 3799)
- New l2tp-shaper plugin registered in plugin registry

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- EventBus: `(l2tp, session-up)` from reactor when PPP session fully established
- EventBus: `(l2tp, session-down)` from reactor when session tears down
- UDP packet on port 3799: CoA-Request or Disconnect-Request from RADIUS server
- YANG config: `l2tp { shaper { ... } }` for default rates and qdisc type

### Transformation Path

**Session-up shaping:**
1. PPP engine completes NCP negotiation, emits `ppp.EventSessionUp`
2. Reactor `handlePPPEvent` matches `EventSessionUp`, looks up session, derives `ppp%d` interface name
3. Reactor emits `SessionUp` on EventBus with TunnelID, SessionID, Interface
4. Shaper receives `SessionUp`, reads config for default rate/burst
5. Shaper builds `traffic.InterfaceQoS` with TBF/HTB qdisc at default rate
6. Shaper calls `traffic.GetBackend().Apply(ctx, map)` to program TC
7. Shaper stores session -> interface + rate in sync.Map

**CoA rate change:**
1. RADIUS server sends CoA-Request to ze on UDP 3799
2. CoA listener validates authenticator against shared secret
3. CoA listener extracts session identification (Acct-Session-Id or User-Name + NAS-Port)
4. CoA listener looks up session via `l2tp.LookupService().LookupSession()`
5. CoA listener extracts bandwidth attributes
6. CoA listener emits `SessionRateChange` on EventBus
7. CoA listener sends CoA-ACK to RADIUS server
8. Shaper receives `SessionRateChange`, updates TC via `traffic.Backend.Apply()`
9. Shaper updates sync.Map with new rate

**Session-down cleanup:**
1. Reactor emits `SessionDown` on EventBus (already implemented)
2. Shaper receives `SessionDown`, looks up session in sync.Map
3. Shaper deletes session from sync.Map (kernel removes qdisc when pppN is deleted)

**Disconnect-Message:**
1. RADIUS server sends Disconnect-Request to ze on UDP 3799
2. CoA listener validates authenticator, identifies session
3. CoA listener calls `l2tp.LookupService().TeardownSession(sid)`
4. CoA listener sends Disconnect-ACK
5. Session teardown triggers SessionDown event; shaper cleanup follows normal path

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| PPP engine -> reactor | ppp.EventSessionUp on Driver.EventsOut() channel | [ ] |
| Reactor -> EventBus | events.SessionUp.Emit(bus, payload) | [ ] |
| EventBus -> shaper | events.SessionUp.Subscribe(bus, callback) | [ ] |
| Shaper -> traffic backend | traffic.GetBackend().Apply(ctx, desired) | [ ] |
| Traffic backend -> kernel | vishvananda/netlink QdiscReplace (via trafficnetlink) | [ ] |
| UDP 3799 -> CoA listener | net.UDPConn.ReadFromUDP in goroutine | [ ] |
| CoA listener -> EventBus | events.SessionRateChange.Emit(bus, payload) | [ ] |
| CoA listener -> session teardown | l2tp.LookupService().TeardownSession(sid) | [ ] |

### Integration Points
- `traffic.GetBackend()` -- shaper uses existing TC backend to program qdiscs
- `traffic.LoadBackend("tc")` -- shaper ensures backend is loaded on first session-up
- `events.SessionUp` -- new typed EventBus event in l2tp/events
- `events.SessionRateChange` -- new typed EventBus event in l2tp/events
- `events.SessionDown` -- existing EventBus event (shaper subscribes)
- `l2tp.LookupService()` -- CoA session matching and DM teardown
- `radius.Decode()` / `radius.Packet.EncodeTo()` -- CoA request/response encoding

### Architectural Verification
- [ ] No bypassed layers (shaper uses traffic.Backend, not direct netlink)
- [ ] No unintended coupling (shaper and RADIUS plugin communicate only via EventBus)
- [ ] No duplicated functionality (reuses traffic.Backend, radius.Packet encoding)
- [ ] Zero-copy preserved where applicable (CoA uses radius.Bufs pool for encoding)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| ppp.EventSessionUp on PPP driver | -> | reactor emits SessionUp, shaper programs TC | `TestShaperSessionUpAppliesTC` |
| CoA-Request UDP packet on port 3799 | -> | CoA listener validates, emits SessionRateChange, shaper updates TC | `TestCoAChangesRate` |
| Disconnect-Request UDP packet on 3799 | -> | CoA listener tears down session | `TestDisconnectMessageTearsDown` |
| (l2tp, session-down) EventBus event | -> | shaper removes session from map, cleanup | `TestShaperSessionDownCleansUp` |
| YANG config `l2tp { shaper { ... } }` | -> | shaper receives config, stores defaults | `TestShaperConfigParsing` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | L2TP session reaches PPP session-up with shaper configured | TBF/HTB qdisc programmed on pppN interface at configured default rate |
| AC-2 | L2TP session tears down | TC state removed from shaper map; kernel removes qdisc with pppN |
| AC-3 | CoA-Request arrives with valid authenticator and bandwidth attributes | Shaper updates TC on matching session's pppN; CoA-ACK sent |
| AC-4 | CoA-Request with invalid authenticator | Silently discarded (RFC 5176 Section 3.5) |
| AC-5 | CoA-Request for unknown session | CoA-NAK sent with Error-Cause 503 (Session Context Not Found) |
| AC-6 | Disconnect-Request with valid authenticator | Matching session torn down; Disconnect-ACK sent |
| AC-7 | Disconnect-Request for unknown session | Disconnect-NAK sent with Error-Cause 503 |
| AC-8 | Shaper config has qdisc-type "tbf" | TBF qdisc applied with rate and burst from config |
| AC-9 | Shaper config has qdisc-type "htb" | HTB qdisc applied with rate/ceil from config |
| AC-10 | No shaper config, session comes up | No TC rules applied; session works without shaping |
| AC-11 | traffic.Backend not loaded (no traffic-control config) | Shaper logs warning and loads "tc" backend on first session-up |
| AC-12 | CoA-Request with Acct-Session-Id attribute | Session matched by accounting session ID |
| AC-13 | CoA-Request with User-Name + NAS-Port attributes | Session matched by username + NAS port |
| AC-14 | `l2tp shaper show` CLI command | Returns JSON with per-session shaping state (interface, rate, applied-at) |
| AC-15 | YANG config reload changes default rate | New sessions get new rate; existing sessions unchanged |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSessionUpPayloadRoundTrip` | `internal/component/l2tp/events/events_test.go` | SessionUp event emit/subscribe | |
| `TestSessionRateChangePayloadRoundTrip` | `internal/component/l2tp/events/events_test.go` | SessionRateChange event emit/subscribe | |
| `TestShaperConfigParsing` | `internal/plugins/l2tpshaper/config_test.go` | YANG JSON config parse for qdisc-type, rate, burst | |
| `TestShaperConfigValidation` | `internal/plugins/l2tpshaper/config_test.go` | Invalid rate (0), invalid qdisc type rejected | |
| `TestShaperSessionUpAppliesTC` | `internal/plugins/l2tpshaper/shaper_test.go` | session-up event triggers Backend.Apply with correct InterfaceQoS | |
| `TestShaperSessionDownCleansUp` | `internal/plugins/l2tpshaper/shaper_test.go` | session-down event removes session from state map | |
| `TestShaperRateChange` | `internal/plugins/l2tpshaper/shaper_test.go` | session-rate-change event triggers Backend.Apply with updated rate | |
| `TestShaperNoConfig` | `internal/plugins/l2tpshaper/shaper_test.go` | no shaper config means no TC applied on session-up | |
| `TestCoARequestDecode` | `internal/component/radius/packet_test.go` | CoA-Request (code 43) decoded correctly | |
| `TestCoAResponseEncode` | `internal/component/radius/packet_test.go` | CoA-ACK/NAK (codes 44/45) encoded correctly | |
| `TestCoAAuthenticatorVerify` | `internal/component/radius/packet_test.go` | CoA request authenticator validation per RFC 5176 | |
| `TestDisconnectRequestDecode` | `internal/component/radius/packet_test.go` | Disconnect-Request (code 40) decoded correctly | |
| `TestCoAListenerSessionMatch` | `internal/plugins/l2tpauthradius/coa_test.go` | CoA matches session by Acct-Session-Id | |
| `TestCoAListenerUserNameMatch` | `internal/plugins/l2tpauthradius/coa_test.go` | CoA matches session by User-Name + NAS-Port | |
| `TestCoAListenerInvalidAuth` | `internal/plugins/l2tpauthradius/coa_test.go` | Invalid authenticator silently dropped | |
| `TestCoAListenerUnknownSession` | `internal/plugins/l2tpauthradius/coa_test.go` | Unknown session returns NAK with Error-Cause 503 | |
| `TestDisconnectMessageTeardown` | `internal/plugins/l2tpauthradius/coa_test.go` | DM calls TeardownSession, returns Disconnect-ACK | |
| `TestBuildQoSForTBF` | `internal/plugins/l2tpshaper/shaper_test.go` | buildQoS produces correct InterfaceQoS for TBF | |
| `TestBuildQoSForHTB` | `internal/plugins/l2tpshaper/shaper_test.go` | buildQoS produces correct InterfaceQoS for HTB | |
| `TestReactorEmitsSessionUp` | `internal/component/l2tp/reactor_ppp_linux_test.go` | handlePPPEvent(EventSessionUp) emits EventBus SessionUp | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Default rate (bps) | 1-10_000_000_000 | 10Gbps | 0 | N/A (uint64) |
| Default burst (bytes) | 1-4_294_967_295 | 4GB | 0 | N/A (uint32) |
| CoA port | 1-65535 | 65535 | 0 | 65536 |
| CoA rate attribute (bps) | 1-10_000_000_000 | 10Gbps | 0 | N/A (uint64) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-l2tp-shaper-session` | `test/l2tp/shaper-session.ci` | Session comes up with shaper config; TC applied | |
| `test-l2tp-shaper-coa` | `test/l2tp/shaper-coa.ci` | CoA changes rate on active session | |
| `test-l2tp-shaper-disconnect` | `test/l2tp/shaper-disconnect.ci` | RADIUS DM tears down session | |

### Future (if deferring any tests)
- None planned

## Files to Modify
- `internal/component/l2tp/events/events.go` -- add SessionUp and SessionRateChange typed events
- `internal/component/l2tp/events/events_test.go` -- tests for new events
- `internal/component/l2tp/reactor.go` -- emit SessionUp EventBus event on ppp.EventSessionUp
- `internal/component/l2tp/reactor_ppp_linux_test.go` -- test reactor SessionUp emission
- `internal/component/radius/dict.go` -- add CoA/DM packet codes (40-45), Error-Cause (101)
- `internal/component/radius/packet.go` -- CoA request authenticator validation (RFC 5176 Section 3.5)
- `internal/component/radius/packet_test.go` -- tests for CoA/DM codes and authenticator
- `internal/plugins/l2tpauthradius/register.go` -- add CoA listener startup in runPlugin
- `internal/plugins/l2tpauthradius/config.go` -- add coa-port to config parsing

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (shaper config) | [x] | `internal/plugins/l2tpshaper/schema/ze-l2tp-shaper-conf.yang` |
| YANG schema (coa-port) | [x] | `internal/plugins/l2tpauthradius/schema/ze-l2tp-auth-radius-conf.yang` |
| CLI commands | [x] | `l2tp shaper show` via SDK OnExecuteCommand |
| Functional test for shaping | [x] | `test/l2tp/shaper-session.ci` |
| Functional test for CoA | [x] | `test/l2tp/shaper-coa.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- add L2TP traffic shaping, RADIUS CoA |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- shaper config section |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` -- `l2tp shaper show` |
| 4 | API/RPC added/changed? | [ ] | |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` -- l2tp-shaper |
| 6 | Has a user guide page? | [x] | `docs/guide/l2tp.md` -- shaping section |
| 7 | Wire format changed? | [ ] | |
| 8 | Plugin SDK/protocol changed? | [ ] | |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc5176.md` -- create |
| 10 | Test infrastructure changed? | [ ] | |
| 11 | Affects daemon comparison? | [ ] | |
| 12 | Internal architecture changed? | [ ] | |

## Files to Create
- `internal/plugins/l2tpshaper/l2tpshaper.go` -- Name, atomic logger
- `internal/plugins/l2tpshaper/register.go` -- init(), registry.Registration, RunEngine
- `internal/plugins/l2tpshaper/shaper.go` -- shaperPlugin struct, session-up/down/rate-change handlers, traffic.Backend calls
- `internal/plugins/l2tpshaper/shaper_test.go` -- unit tests with mock Backend
- `internal/plugins/l2tpshaper/config.go` -- parseConfig from YANG JSON
- `internal/plugins/l2tpshaper/config_test.go` -- config parse tests
- `internal/plugins/l2tpshaper/schema/embed.go` -- //go:embed
- `internal/plugins/l2tpshaper/schema/register.go` -- yang.RegisterModule
- `internal/plugins/l2tpshaper/schema/ze-l2tp-shaper-conf.yang` -- YANG schema
- `internal/plugins/l2tpauthradius/coa.go` -- CoA/DM listener implementation
- `internal/plugins/l2tpauthradius/coa_test.go` -- CoA/DM listener tests
- `rfc/short/rfc5176.md` -- RFC summary for Dynamic Authorization
- `test/l2tp/shaper-session.ci` -- functional test: shaping on session-up
- `test/l2tp/shaper-coa.ci` -- functional test: CoA rate change
- `test/l2tp/shaper-disconnect.ci` -- functional test: DM session teardown

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7-13 | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: EventBus events** -- add SessionUp and SessionRateChange to l2tp/events
   - Tests: `TestSessionUpPayloadRoundTrip`, `TestSessionRateChangePayloadRoundTrip`
   - Files: `internal/component/l2tp/events/events.go`, `events_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Reactor emission** -- reactor emits SessionUp on ppp.EventSessionUp
   - Tests: `TestReactorEmitsSessionUp`
   - Files: `internal/component/l2tp/reactor.go`, `reactor_ppp_linux_test.go`
   - Verify: tests fail -> implement -> tests pass
   - Note: derive pppN name from session state (unitNum from kernel setup)

3. **Phase: RADIUS dict + CoA authenticator** -- add CoA/DM codes, request authenticator validation
   - Tests: `TestCoARequestDecode`, `TestCoAResponseEncode`, `TestCoAAuthenticatorVerify`, `TestDisconnectRequestDecode`
   - Files: `internal/component/radius/dict.go`, `packet.go`, `packet_test.go`
   - Verify: tests fail -> implement -> tests pass
   - RFC: RFC 5176 Section 3.5 request authenticator

4. **Phase: Shaper plugin scaffold** -- l2tpshaper package, logger, registration, config parsing
   - Tests: `TestShaperConfigParsing`, `TestShaperConfigValidation`
   - Files: all `internal/plugins/l2tpshaper/` files
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Shaper core logic** -- session-up/down/rate-change handlers, traffic.Backend calls
   - Tests: `TestShaperSessionUpAppliesTC`, `TestShaperSessionDownCleansUp`, `TestShaperRateChange`, `TestShaperNoConfig`, `TestBuildQoSForTBF`, `TestBuildQoSForHTB`
   - Files: `internal/plugins/l2tpshaper/shaper.go`, `shaper_test.go`
   - Verify: tests fail -> implement -> tests pass

6. **Phase: CoA/DM listener** -- UDP server in l2tp-auth-radius, session matching, event emission
   - Tests: `TestCoAListenerSessionMatch`, `TestCoAListenerUserNameMatch`, `TestCoAListenerInvalidAuth`, `TestCoAListenerUnknownSession`, `TestDisconnectMessageTeardown`
   - Files: `internal/plugins/l2tpauthradius/coa.go`, `coa_test.go`, `register.go`, `config.go`
   - Verify: tests fail -> implement -> tests pass

7. **Phase: YANG schemas** -- shaper config schema, coa-port in RADIUS schema
   - Files: YANG files in schema/ subdirs
   - Verify: `make generate`, config parse round-trip

8. **Phase: RFC summary** -- create `rfc/short/rfc5176.md`

9. **Functional tests** -- create .ci tests for end-to-end behavior

10. **Full verification** -- `make ze-verify`

11. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-15 has implementation with file:line |
| Correctness | CoA authenticator verification matches RFC 5176 Section 3.5 |
| Correctness | TBF rate/burst parameters correctly mapped to traffic.InterfaceQoS |
| Naming | Plugin name "l2tp-shaper", YANG kebab-case, config root "l2tp" |
| Data flow | CoA -> EventBus -> shaper (never direct); DM -> LookupService (direct is correct) |
| Rule: no-coupling | Shaper does not import l2tpauthradius; communication via EventBus only |
| Rule: plugin-pattern | register.go with init(), atomic logger, CLIHandler closure |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| l2tp-shaper plugin registered | `grep l2tpshaper internal/component/plugin/all/all.go` |
| SessionUp event defined | `grep SessionUp internal/component/l2tp/events/events.go` |
| SessionRateChange event defined | `grep SessionRateChange internal/component/l2tp/events/events.go` |
| Reactor emits SessionUp | `grep SessionUp internal/component/l2tp/reactor.go` |
| CoA codes in RADIUS dict | `grep CodeCoARequest internal/component/radius/dict.go` |
| CoA listener in RADIUS plugin | `ls internal/plugins/l2tpauthradius/coa.go` |
| YANG schema for shaper | `ls internal/plugins/l2tpshaper/schema/ze-l2tp-shaper-conf.yang` |
| RFC 5176 summary | `ls rfc/short/rfc5176.md` |
| Unit tests pass | `go test ./internal/plugins/l2tpshaper/... ./internal/plugins/l2tpauthradius/...` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | CoA/DM request authenticator MUST be verified before processing any attributes (RFC 5176 Section 3.5) |
| Input validation | CoA/DM packet length validated against UDP datagram size; reject truncated |
| Source restriction | CoA listener SHOULD accept packets only from configured RADIUS server addresses |
| Secret handling | Shared secret reused from auth config; `ze:sensitive` on YANG leaf (already set) |
| Resource exhaustion | CoA listener rate limiting; max pending requests; drop on backpressure |
| Session matching | CoA for non-existent session returns NAK, does not crash |
| DM safety | TeardownSession is idempotent; double-DM is safe |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| traffic.Backend returns error | Log error, do not crash; session works without shaping |
| CoA authenticator mismatch | Silent discard per RFC 5176 |
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

## RFC Documentation

Add `// RFC 5176 Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: CoA/DM packet codes, request authenticator computation, session identification, Error-Cause values, silent discard rules.

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

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied

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
- [ ] AC-1..AC-15 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added (RFC 5176)
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

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/`
- [ ] Summary included in commit

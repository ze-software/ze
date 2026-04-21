# Spec: l2tp-8a -- Built-in Auth Handler and Static IP Pool

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-l2tp-7-subsystem |
| Phase | 6/7 |
| Updated | 2026-04-21 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/component/ppp/manager.go` -- Driver channels (AuthEventsOut, IPEventsOut)
4. `internal/component/ppp/auth_events.go` -- EventAuthRequest, AuthMethod
5. `internal/component/ppp/ip_events.go` -- EventIPRequest, IPResponseArgs
6. `internal/component/l2tp/subsystem.go` -- pppDrivers wiring
7. `internal/component/l2tp/reactor.go` -- handleKernelSuccess (StartSession builder)
8. `internal/component/plugin/registry/registry_bgp_filter.go` -- RegisterAttrModHandler pattern

## Task

Wire the PPP Driver's auth and IP allocation channels to production handlers
by creating two plugins and a handler registry in the l2tp package.

The PPP Driver exposes `AuthEventsOut()` and `IPEventsOut()` channels for
request-response auth/IP flows. Tests use `autoAcceptAuth()` and
`autoAcceptIP()` goroutines as stubs. This spec provides the production
consumers: a handler registry in the l2tp package (following the
`RegisterAttrModHandler` pattern), two SDK plugins that register handlers
during init, and subsystem wiring that spawns drain goroutines.

**l2tp-auth-local**: accepts sessions based on a static user list from
YANG config. Verifies PAP cleartext, CHAP-MD5 challenge-response, and
MS-CHAPv2 responses. When no users are configured, accepts all sessions
(permissive default for testing/development).

**l2tp-pool**: allocates IPv4 addresses from configured ranges using a
bitmap-backed pool. Releases addresses on session-down events. Rejects
requests when the pool is exhausted.

Both plugins are proper SDK plugins with `registry.Registration`, YANG
schemas, CLI handlers, and atomic logger pattern.

## Required Reading

### Architecture Docs
- [ ] `ai/patterns/registration.md` -- all registration mechanisms
  -> Constraint: plugins register via init() -> registry.Register(); blank import in all.go triggers init()
  -> Constraint: RegisterAttrModHandler pattern -- package-level map behind mutex, populated at init, read at runtime
- [ ] `ai/patterns/plugin.md` -- plugin file structure and templates
  -> Constraint: register.go with init(), atomic logger, RunXxxPlugin(conn), CLIHandler closure
  -> Constraint: run `make generate` after creating plugin to update all.go
- [ ] `ai/rules/plugin-design.md` -- plugin design rules
  -> Constraint: YANG is required for plugins with RPCs; proximity principle; no sibling imports
  -> Constraint: plugin name is hyphen-form (l2tp-auth-local); log/env uses dot-form (l2tp.auth.local)
- [ ] `docs/architecture/core-design.md` -- subsystem and plugin patterns
  -> Constraint: subsystem implements ze.Subsystem; plugins discovered via registry
- [ ] `docs/research/l2tpv2-ze-integration.md` -- ze integration design (section 6)
  -> Decision: auth/pool are plugins, not hardcoded in subsystem
  -> Decision: design doc envisions EventBus-based SDK plugins; actual PPP implementation uses Go channels; this spec bridges the gap with handler registry
- [ ] `ai/rules/goroutine-lifecycle.md` -- concurrency patterns
  -> Constraint: goroutines must have clear ownership and shutdown path

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2661.md` -- L2TP (tunnel auth context only)
  -> Constraint: tunnel auth (CHAP-MD5 challenge/response) is in l2tp/auth.go; this spec handles PPP session auth, which is a different layer
- [ ] `rfc/short/rfc1994.md` -- CHAP
  -> Constraint: CHAP-MD5 verification: MD5(identifier || secret || challenge) == response
- [ ] `rfc/short/rfc1334.md` -- PAP
  -> Constraint: PAP carries cleartext password in Authenticate-Request peer-id + passwd fields

**Key insights:**
- PPP Driver exposes three channels: EventsOut (lifecycle, consumed by reactor), AuthEventsOut (auth requests, UNCONNECTED), IPEventsOut (IP requests, UNCONNECTED)
- Auth/IP channels are synchronous request-response: emit request on channel, consumer calls Driver.AuthResponse/IPResponse
- Channel buffer is 64; sustained backlog blocks ALL PPP sessions
- Tests use autoAcceptAuth/autoAcceptIP goroutines -- production handlers follow the same shape
- RegisterAttrModHandler pattern: package-level map, registered at init time, handler closure captures plugin state
- pppDriverIface only exposes SessionsIn/EventsOut; subsystem accesses concrete *ppp.Driver for auth/IP channels

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ppp/manager.go` -- Driver struct, AuthEventsOut()/IPEventsOut() channels, AuthResponse()/IPResponse() methods
  -> Constraint: AuthEventsOut buffer=64, IPEventsOut buffer=64; consumer MUST drain promptly
  -> Constraint: AuthResponse is non-blocking (buffered(1) authRespCh); duplicate returns ErrAuthResponsePending
  -> Constraint: IPResponse is non-blocking (buffered(2) ipRespCh); duplicate returns ErrIPResponsePending
- [ ] `internal/component/ppp/auth_events.go` -- AuthEvent sealed sum, EventAuthRequest struct, AuthMethod enum
  -> Constraint: EventAuthRequest carries TunnelID, SessionID, Method (PAP/CHAP-MD5/MSCHAPv2/None), Identifier, Username, Challenge, Response, PeerName
  -> Constraint: PAP: Challenge empty, Response is cleartext password
  -> Constraint: CHAP-MD5: Challenge is 16-byte value ze sent, Response is 16-byte MD5 digest
  -> Constraint: MS-CHAPv2: Challenge is 16-byte Authenticator Challenge, Response is PeerChallenge(16) || NTResponse(24) = 40 bytes
- [ ] `internal/component/ppp/ip_events.go` -- IPEvent sealed sum, EventIPRequest struct, AddressFamily enum, ipResponseMsg struct
  -> Constraint: EventIPRequest carries TunnelID, SessionID, Family (IPv4/IPv6), SuggestedLocal/Peer/InterfaceID
  -> Constraint: IPResponseArgs: Accept, Family, Local, Peer, DNSPrimary, DNSSecondary, PeerInterfaceID
  -> Constraint: For IPv4 accept: Local and Peer MUST both be non-zero
- [ ] `internal/component/ppp/helpers_test.go` -- autoAcceptAuth goroutine pattern
  -> Constraint: range over AuthEventsOut(); type-assert EventAuthRequest; call AuthResponse(accept=true); ignore ErrSessionNotFound
- [ ] `internal/component/ppp/ncp_helpers_test.go` -- autoAcceptIP goroutine pattern
  -> Constraint: range over IPEventsOut(); type-assert EventIPRequest; build IPResponseArgs per family; call IPResponse; ignore errors
- [ ] `internal/component/l2tp/subsystem.go` -- Subsystem.Start creates ppp.Driver, stores in pppDrivers slice
  -> Constraint: pppDriver created per listener; Start order: PPP driver -> kernel worker -> reactor -> timer
  -> Constraint: Stop order (unwindLocked): timers -> reactors -> PPP drivers -> kernel workers -> listeners
  -> Constraint: subsystem stores pppDrivers []*ppp.Driver (concrete type, not interface)
- [ ] `internal/component/l2tp/reactor.go` -- pppDriverIface only has SessionsIn/EventsOut
  -> Constraint: reactor does NOT consume AuthEventsOut or IPEventsOut; separate consumer needed
- [ ] `internal/component/plugin/registry/registry_bgp_filter.go` -- RegisterAttrModHandler pattern
  -> Constraint: package-level var + mutex; Register at init; query at runtime via typed getter
  -> Constraint: nil handler ignored by Register; Unregister for test cleanup

**Behavior to preserve:**
- PPP Driver channel semantics unchanged (buffer sizes, blocking behavior, error returns)
- Existing autoAcceptAuth/autoAcceptIP test helpers continue to work
- Subsystem Start/Stop ordering (auth/IP drain goroutines fit between Driver.Start and Driver.Stop)
- Reactor's pppEventsOut select arm unchanged
- All existing L2TP functional tests pass

**Behavior to change:**
- Add handler registry (AuthHandler/PoolHandler types + Register/Get functions) in l2tp package
- Add drain goroutines in subsystem Start path after pppDriver.Start()
- Two new plugins: l2tp-auth-local, l2tp-pool
- New YANG schemas for auth local users and pool ranges
- New CLI: `show l2tp pool`
- New env vars: `ze.log.l2tp.auth.local`, `ze.log.l2tp.pool`

## Data Flow (MANDATORY)

### Entry Point
- PPP session goroutine emits `EventAuthRequest` on `ppp.Driver.authEventsOut` channel after LCP completes
- PPP session goroutine emits `EventIPRequest` on `ppp.Driver.ipEventsOut` channel after auth completes
- YANG config tree delivers auth user list and pool ranges during plugin configure phase

### Transformation Path
1. PPP session goroutine completes LCP, enters auth phase
2. Session emits `EventAuthRequest{Method, Username, Challenge, Response}` on `authEventsOut`
3. Auth drain goroutine reads the event from `Driver.AuthEventsOut()`
4. Auth drain goroutine calls `l2tp.GetAuthHandler()(request)` -> `AuthResult`
5. Auth drain goroutine calls `Driver.AuthResponse(tid, sid, result.Accept, result.Message, result.AuthBlob)`
6. PPP session goroutine unblocks from `authRespCh`, proceeds to NCP phase (or tears down on reject)
7. Session emits `EventIPRequest{Family, SuggestedLocal/Peer}` on `ipEventsOut`
8. Pool drain goroutine reads the event from `Driver.IPEventsOut()`
9. Pool drain goroutine calls `l2tp.GetPoolHandler()(request)` -> `ppp.IPResponseArgs`
10. Pool drain goroutine calls `Driver.IPResponse(tid, sid, args)`
11. PPP session goroutine unblocks from `ipRespCh`, completes IPCP negotiation

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| PPP session -> auth drain goroutine | authEventsOut channel (buffered 64) | [ ] |
| Auth drain goroutine -> auth handler | Direct function call (handler registered at init) | [ ] |
| Auth drain goroutine -> PPP session | Driver.AuthResponse() -> authRespCh (buffered 1) | [ ] |
| PPP session -> pool drain goroutine | ipEventsOut channel (buffered 64) | [ ] |
| Pool drain goroutine -> pool handler | Direct function call (handler registered at init) | [ ] |
| Pool drain goroutine -> PPP session | Driver.IPResponse() -> ipRespCh (buffered 2) | [ ] |
| Reactor -> EventBus session-down | Reactor emits `(l2tp, session-down)` on EventBus when processing ppp.EventSessionDown | [ ] |
| EventBus -> pool plugin | Pool plugin subscribes via ConfigureEventBus; releases allocated IP on session-down | [ ] |

### Integration Points
- `l2tp.RegisterAuthHandler()` / `l2tp.GetAuthHandler()` -- handler registry
- `l2tp.RegisterPoolHandler()` / `l2tp.GetPoolHandler()` -- handler registry
- `ppp.Driver.AuthEventsOut()` / `ppp.Driver.IPEventsOut()` -- PPP channels
- `ppp.Driver.AuthResponse()` / `ppp.Driver.IPResponse()` -- PPP response methods
- `l2tp/events/SessionDown` -- new typed EventBus handle for session-down notification (pool release)
- `registry.Register()` -- standard plugin registration
- `yang.RegisterModule()` -- YANG schema registration
- `env.MustRegister()` -- env var registration
- Plugin auto-import in `internal/component/plugin/all/all.go` via `make generate`

### Architectural Verification
- [ ] No bypassed layers (handlers registered at init, called by subsystem drain goroutines)
- [ ] No unintended coupling (plugins don't import l2tp subsystem internals; handler function type is the only shared surface)
- [ ] No duplicated functionality (reuses PPP Driver channels, RegisterAttrModHandler pattern)
- [ ] Zero-copy preserved where applicable (EventAuthRequest passed by value; no buffer copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| PPP emits EventAuthRequest on channel | -> | Auth drain goroutine calls registered handler | `TestAuthDrainCallsHandler` |
| PPP emits EventIPRequest on channel | -> | Pool drain goroutine calls registered handler | `TestPoolDrainCallsHandler` |
| Config `l2tp { auth { local { user foo { password bar; } } } }` | -> | l2tp-auth-local plugin parses config, registers handler | `test/l2tp/auth-local-pap.ci` |
| Config `l2tp { pool { ipv4 { range 10.0.0.1 10.0.0.254; } } }` | -> | l2tp-pool plugin parses config, registers handler | `test/l2tp/pool-basic.ci` |
| End-to-end: tunnel + session + auth + IP | -> | Full session with local auth and static pool | `test/l2tp/session-auth-pool.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Auth handler registered; PPP session completes LCP | EventAuthRequest delivered to handler; handler returns accept; session proceeds to NCP |
| AC-2 | Auth handler returns reject (unknown user) | PPP sends Auth-Failure; EventAuthFailure emitted; EventSessionDown emitted; L2TP sends CDN |
| AC-3 | Pool handler registered; PPP NCP starts | EventIPRequest delivered to handler; handler returns address; IPCP completes |
| AC-4 | Pool handler returns reject (exhausted) | PPP NCP fails; session tears down |
| AC-5 | No auth handler registered (nil) | Default: accept all sessions; WARN logged at subsystem Start |
| AC-6 | No pool handler registered (nil) | Default: reject all IP requests; ERROR logged at subsystem Start |
| AC-7 | l2tp-auth-local with static user list; PAP auth with correct password | Session accepted |
| AC-8 | l2tp-auth-local with static user list; PAP auth with wrong password | Session rejected |
| AC-9 | l2tp-auth-local with static user list; CHAP-MD5 auth | Challenge-response verified against configured password; accepted on match |
| AC-10 | l2tp-pool with configured IPv4 range | First session gets first available IP; second session gets next IP |
| AC-11 | Session down after IP allocated | Pool plugin receives (l2tp, session-down) via EventBus; IP released back to pool; available for next session |
| AC-12 | Pool range fully allocated; new request arrives | Request rejected with "pool exhausted" reason |
| AC-13 | `show l2tp pool` CLI command | Shows pool name, range, total, allocated, available counts |
| AC-14 | Both plugins appear in `make ze-inventory` | l2tp-auth-local and l2tp-pool listed with correct names |
| AC-15 | Auth handler panics | Drain goroutine recovers; logs error; rejects current request; continues draining |
| AC-16 | Pool handler panics | Drain goroutine recovers; logs error; rejects current request; continues draining |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegisterAuthHandler` | `internal/component/l2tp/handler_registry_test.go` | Register and retrieve auth handler | |
| `TestRegisterAuthHandlerNil` | `internal/component/l2tp/handler_registry_test.go` | Nil handler ignored | |
| `TestRegisterPoolHandler` | `internal/component/l2tp/handler_registry_test.go` | Register and retrieve pool handler | |
| `TestRegisterPoolHandlerNil` | `internal/component/l2tp/handler_registry_test.go` | Nil handler ignored | |
| `TestAuthDrainCallsHandler` | `internal/component/l2tp/drain_test.go` | Drain goroutine reads channel and calls handler | |
| `TestAuthDrainDefaultAcceptAll` | `internal/component/l2tp/drain_test.go` | Nil handler defaults to accept-all | |
| `TestAuthDrainPanicRecovery` | `internal/component/l2tp/drain_test.go` | Handler panic recovered; request rejected; drain continues | |
| `TestPoolDrainCallsHandler` | `internal/component/l2tp/drain_test.go` | Drain goroutine reads channel and calls handler | |
| `TestPoolDrainDefaultReject` | `internal/component/l2tp/drain_test.go` | Nil handler defaults to reject | |
| `TestPoolDrainPanicRecovery` | `internal/component/l2tp/drain_test.go` | Handler panic recovered; request rejected; drain continues | |
| `TestLocalAuthPAPAccept` | `internal/plugins/l2tpauthlocal/auth_test.go` | PAP with correct password accepted | |
| `TestLocalAuthPAPReject` | `internal/plugins/l2tpauthlocal/auth_test.go` | PAP with wrong password rejected | |
| `TestLocalAuthCHAPMD5Accept` | `internal/plugins/l2tpauthlocal/auth_test.go` | CHAP-MD5 with correct secret accepted | |
| `TestLocalAuthCHAPMD5Reject` | `internal/plugins/l2tpauthlocal/auth_test.go` | CHAP-MD5 with wrong secret rejected | |
| `TestLocalAuthNoUsersAcceptAll` | `internal/plugins/l2tpauthlocal/auth_test.go` | No configured users -> accept all | |
| `TestPoolAllocateIPv4` | `internal/plugins/l2tppool/pool_test.go` | Allocate first available address | |
| `TestPoolAllocateSequential` | `internal/plugins/l2tppool/pool_test.go` | Sequential allocations get sequential addresses | |
| `TestPoolRelease` | `internal/plugins/l2tppool/pool_test.go` | Released address becomes available again | |
| `TestPoolExhausted` | `internal/plugins/l2tppool/pool_test.go` | Full pool rejects new allocations | |
| `TestPoolRangeConfig` | `internal/plugins/l2tppool/pool_test.go` | Config parsing: start, end, dns addresses | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Pool range start | 1.0.0.0 - 223.255.255.255 | 223.255.255.255 | 0.0.0.0 (unspecified) | 224.0.0.0 (multicast) |
| Pool range end | start - 255.255.255.254 | 255.255.255.254 | start-1 (end < start) | 255.255.255.255 (broadcast) |
| Pool range size | 1 - 65534 | 65534 | 0 (start==end+1) | N/A (capped by uint16 session limit) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `auth-local-pap` | `test/l2tp/auth-local-pap.ci` | L2TP session with PAP auth against local user list | |
| `pool-basic` | `test/l2tp/pool-basic.ci` | L2TP session gets IP from configured pool | |
| `pool-exhaustion` | `test/l2tp/pool-exhaustion.ci` | Pool full; new session rejected | |
| `session-auth-pool` | `test/l2tp/session-auth-pool.ci` | End-to-end: tunnel + session + local auth + pool allocation | |

### Future (if deferring any tests)
- MS-CHAPv2 functional test: deferred because fakel2tp peer does not implement MS-CHAPv2 wire format; unit tests cover the hash verification
- IPv6 pool: deferred to a later spec; this spec covers IPv4 only
- `show l2tp auth users` CLI: deferred to design phase when we have RADIUS; local auth config is visible via `show running-config`

## Files to Modify

- `internal/component/l2tp/subsystem.go` -- spawn auth/IP drain goroutines per pppDriver; stop before Driver.Stop
- `internal/component/l2tp/subsystem_test.go` -- test drain goroutine wiring
- `internal/component/l2tp/events/events.go` -- add SessionDown typed event handle alongside existing RouteChange
- `internal/component/l2tp/reactor.go` -- emit (l2tp, session-down) on EventBus in handlePPPEvent for EventSessionDown
- `internal/component/plugin/registry/registry_test.go` -- update TestAllPluginsRegistered expected count (+2)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new) | [x] | `internal/plugins/l2tpauthlocal/schema/ze-l2tp-auth-local-conf.yang`, `internal/plugins/l2tppool/schema/ze-l2tp-pool-conf.yang` |
| CLI commands | [x] | `show l2tp pool` via l2tp-pool CLIHandler |
| Editor autocomplete | [x] | YANG-driven (automatic) |
| Functional test for new RPC/API | [x] | `test/l2tp/*.ci` |
| Plugin blank imports | [x] | `internal/component/plugin/all/all.go` (via `make generate`) |
| Env var registration | [x] | `ze.log.l2tp.auth.local`, `ze.log.l2tp.pool` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- add local auth and IP pool |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- add l2tp auth/pool config examples |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` -- add `show l2tp pool` |
| 4 | API/RPC added/changed? | [ ] | |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` -- add l2tp-auth-local, l2tp-pool |
| 6 | Has a user guide page? | [ ] | Covered in existing `docs/guide/l2tp.md` (umbrella) |
| 7 | Wire format changed? | [ ] | |
| 8 | Plugin SDK/protocol changed? | [ ] | |
| 9 | RFC behavior implemented? | [ ] | PAP/CHAP are PPP-layer; already covered in l2tp-6b |
| 10 | Test infrastructure changed? | [ ] | |
| 11 | Affects daemon comparison? | [ ] | |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` -- add handler registry pattern |

## Files to Create

- `internal/component/l2tp/handler.go` -- AuthResult, AuthHandler, PoolHandler types
- `internal/component/l2tp/handler_registry.go` -- RegisterAuthHandler, GetAuthHandler, RegisterPoolHandler, GetPoolHandler
- `internal/component/l2tp/handler_registry_test.go` -- registry unit tests
- `internal/component/l2tp/drain.go` -- drainAuth, drainPool goroutine functions
- `internal/component/l2tp/drain_test.go` -- drain goroutine unit tests
- `internal/plugins/l2tpauthlocal/register.go` -- init, registry.Register, l2tp.RegisterAuthHandler
- `internal/plugins/l2tpauthlocal/l2tpauthlocal.go` -- RunL2TPAuthLocalPlugin, logger, config
- `internal/plugins/l2tpauthlocal/auth.go` -- handleAuth function (PAP/CHAP-MD5/MS-CHAPv2 verification)
- `internal/plugins/l2tpauthlocal/auth_test.go` -- auth logic unit tests
- `internal/plugins/l2tpauthlocal/schema/register.go` -- YANG module registration
- `internal/plugins/l2tpauthlocal/schema/embed.go` -- go:embed
- `internal/plugins/l2tpauthlocal/schema/ze-l2tp-auth-local-conf.yang` -- YANG schema
- `internal/plugins/l2tppool/register.go` -- init, registry.Register, l2tp.RegisterPoolHandler
- `internal/plugins/l2tppool/l2tppool.go` -- RunL2TPPoolPlugin, logger, config
- `internal/plugins/l2tppool/pool.go` -- IPv4 bitmap pool, handleIPRequest
- `internal/plugins/l2tppool/pool_test.go` -- pool logic unit tests
- `internal/plugins/l2tppool/schema/register.go` -- YANG module registration
- `internal/plugins/l2tppool/schema/embed.go` -- go:embed
- `internal/plugins/l2tppool/schema/ze-l2tp-pool-conf.yang` -- YANG schema
- `test/l2tp/auth-local-pap.ci` -- PAP auth functional test
- `test/l2tp/pool-basic.ci` -- basic pool allocation functional test
- `test/l2tp/pool-exhaustion.ci` -- pool exhaustion functional test
- `test/l2tp/session-auth-pool.ci` -- end-to-end session functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Handler types and registry** -- define AuthResult, AuthHandler, PoolHandler types; implement handler registry with Register/Get/Unregister
   - Tests: `TestRegisterAuthHandler`, `TestRegisterAuthHandlerNil`, `TestRegisterPoolHandler`, `TestRegisterPoolHandlerNil`
   - Files: `handler.go`, `handler_registry.go`, `handler_registry_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Drain goroutines** -- implement drainAuth/drainPool that read PPP channels, call handlers, call Driver.AuthResponse/IPResponse; panic recovery; nil-handler defaults
   - Tests: `TestAuthDrainCallsHandler`, `TestAuthDrainDefaultAcceptAll`, `TestAuthDrainPanicRecovery`, `TestPoolDrainCallsHandler`, `TestPoolDrainDefaultReject`, `TestPoolDrainPanicRecovery`
   - Files: `drain.go`, `drain_test.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Subsystem wiring** -- spawn drain goroutines in Subsystem.Start after pppDriver.Start(); ensure drain goroutines exit during Stop (before pppDriver.Stop, which closes channels)
   - Tests: update `subsystem_test.go` to verify drain goroutine lifecycle
   - Files: `subsystem.go`, `subsystem_test.go`
   - Verify: existing L2TP tests still pass; new wiring test passes

4. **Phase: l2tp-auth-local plugin** -- register.go, YANG schema, config parsing, auth handler (PAP + CHAP-MD5 + MS-CHAPv2), unit tests
   - Tests: `TestLocalAuthPAPAccept`, `TestLocalAuthPAPReject`, `TestLocalAuthCHAPMD5Accept`, `TestLocalAuthCHAPMD5Reject`, `TestLocalAuthNoUsersAcceptAll`
   - Files: all l2tpauthlocal/* files
   - Verify: `make generate` updates all.go; unit tests pass

5. **Phase: l2tp-pool plugin** -- register.go, YANG schema, config parsing, bitmap pool, IP handler, release on session-down, CLI handler, unit tests
   - Tests: `TestPoolAllocateIPv4`, `TestPoolAllocateSequential`, `TestPoolRelease`, `TestPoolExhausted`, `TestPoolRangeConfig`
   - Files: all l2tppool/* files
   - Verify: `make generate` updates all.go; unit tests pass

6. **Phase: Functional tests** -- create .ci tests for auth, pool, and end-to-end
   - Tests: `auth-local-pap.ci`, `pool-basic.ci`, `pool-exhaustion.ci`, `session-auth-pool.ci`
   - Files: `test/l2tp/*.ci`
   - Verify: functional tests pass

7. **Phase: Plugin count and full verification** -- update TestAllPluginsRegistered count; run `make ze-verify`
   - Files: `registry_test.go`
   - Verify: `make ze-verify` passes

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-16 has implementation with file:line |
| Correctness | CHAP-MD5 hash: `MD5(identifier \|\| secret \|\| challenge)` matches RFC 1994; PAP compares cleartext |
| Naming | Plugin names hyphen-form: `l2tp-auth-local`, `l2tp-pool`; env/log dot-form: `l2tp.auth.local`, `l2tp.pool` |
| Data flow | Auth: channel -> drain goroutine -> handler -> Driver.AuthResponse; no shortcut |
| Rule: goroutine-lifecycle | Drain goroutines have clear stop signal (channel close on Driver.Stop); no leaked goroutines |
| Rule: plugin-design | Both plugins have register.go, YANG, CLIHandler, atomic logger; no sibling imports |
| Panic safety | Both drain goroutines recover() from handler panics and continue |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Handler registry exists | `grep RegisterAuthHandler internal/component/l2tp/handler_registry.go` |
| Drain goroutines exist | `grep drainAuth internal/component/l2tp/drain.go` |
| Subsystem spawns drain goroutines | `grep drainAuth internal/component/l2tp/subsystem.go` |
| l2tp-auth-local plugin registers | `grep l2tp-auth-local internal/plugins/l2tpauthlocal/register.go` |
| l2tp-pool plugin registers | `grep l2tp-pool internal/plugins/l2tppool/register.go` |
| Both in all.go | `grep l2tpauthlocal internal/component/plugin/all/all.go` |
| YANG schemas exist | `ls internal/plugins/l2tpauthlocal/schema/*.yang internal/plugins/l2tppool/schema/*.yang` |
| Functional tests exist | `ls test/l2tp/auth-local-pap.ci test/l2tp/pool-basic.ci test/l2tp/session-auth-pool.ci` |
| `make ze-verify` passes | Run and check exit code |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Pool range: start < end, both valid unicast, range not excessively large |
| Password handling | PAP cleartext password not logged; user config passwords marked ze:sensitive in YANG |
| CHAP-MD5 timing | Use `subtle.ConstantTimeCompare` for CHAP-MD5 and MS-CHAPv2 response verification |
| Pool state | Mutex protects bitmap; no TOCTOU between check-available and mark-allocated |
| Handler panic | Both drain goroutines recover(); panic does not crash subsystem or leak goroutines |
| Resource exhaustion | Pool bitmap is bounded by range size (max 65534); no unbounded allocation |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read PPP auth_events.go / ip_events.go -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Existing L2TP tests break | Wiring change broke something; investigate subsystem.go changes |
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

- The research doc (l2tpv2-ze-integration.md section 6) envisioned auth/pool as EventBus-based SDK plugins. The actual PPP implementation uses synchronous Go channels (request on channel, response via method call). The handler registry pattern bridges this gap: plugins are proper SDK modules (YANG, CLI, metrics), but the hot path is a direct function call registered on a lightweight registry.
- PPP's auth/IP channels are 64-buffered but the handler runs synchronously in the drain goroutine. For the built-in local auth (map lookup) and static pool (bitmap check), this is sub-microsecond. RADIUS (l2tp-8b) will need concurrent handling -- the handler function can spawn its own goroutine and call Driver.AuthResponse asynchronously if needed.
- Session-down IP release: the reactor emits a typed `(l2tp, session-down)` event on the EventBus when processing ppp.EventSessionDown. The pool plugin subscribes via ConfigureEventBus and releases the allocated IP. This follows the same pattern as the route observer (EventBus for cross-component notification) and avoids coupling the pool to the drain goroutine's internal channel flow.

## RFC Documentation

Add `// RFC 1994 Section 4.1` above CHAP-MD5 verification code.
Add `// RFC 1334 Section 2.2.1` above PAP verification code.
MUST document: CHAP identifier included in MD5 hash computation, PAP cleartext comparison.

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
- (to be filled)

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
- [ ] AC-1..AC-16 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added (RFC 1994, RFC 1334)
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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/`
- [ ] Summary included in commit

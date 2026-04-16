# Spec: l2tp-6c -- IPCP, IPv6CP, and pppN Configuration

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-l2tp-6b-auth |
| Phase | - |
| Updated | 2026-04-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-l2tp-6a-lcp-base.md` -- LCP foundation, frame I/O, manager
3. `plan/spec-l2tp-6b-auth.md` -- auth phase, EventAuthRequest/Response flow
4. `plan/spec-l2tp-0-umbrella.md` -- umbrella context
5. `rfc/short/rfc1332.md` -- IPCP
6. `rfc/short/rfc1877.md` -- IPCP DNS option (create if missing)
7. `rfc/short/rfc5072.md` -- IPv6CP
8. `internal/component/iface/backend.go` -- `iface.Backend` interface

## Task

Add the Network Control Protocols (NCPs) on top of the LCP+auth base
established by 6a/6b: IPCP for IPv4 address + DNS, IPv6CP for interface
identifier. Both NCPs run in parallel after authentication completes
(RFC 5072 Section 1: "IPv6CP is a separate NCP from IPCP and... operates
independently"). On NCP completion, configure the pppN interface via
`iface.Backend` (address, peer address, MTU is already set in 6a, route
to peer). Emit `EventSessionIPAssigned` so the L2TP subsystem (Phase 7)
can inject the subscriber route into the redistribute path.

| Capability | In Scope |
|------------|----------|
| IPCP wire format | yes -- RFC 1332 packet types reuse LCP shape |
| IPCP IP-Address option (type 3) | yes -- negotiate local + peer IPv4 addresses |
| IPCP Primary-DNS option (RFC 1877 type 129) | yes -- accept DNS hint from external handler |
| IPCP Secondary-DNS option (RFC 1877 type 131) | yes |
| IPv6CP wire format | yes -- RFC 5072 packet types |
| IPv6CP Interface-Identifier option (type 1) | yes -- negotiate 64-bit interface ID |
| Parallel NCP execution | yes -- IPCP and IPv6CP advance independently from same per-session goroutine via select |
| `EventIPRequest` / `IPResponse` flow | yes -- mirrors auth flow; external handler (l2tp-pool plugin in Phase 8) supplies addresses |
| `EventSessionIPAssigned` | yes -- emitted after at least one NCP succeeds |
| pppN address via `iface.Backend.AddAddressP2P` | yes -- new method added to `iface.Backend` interface in this spec |
| pppN admin up | yes -- `iface.Backend.SetAdminUp` |
| pppN peer route | yes -- `iface.Backend.AddRoute(name, peerCIDR, gateway="", metric=0)` |
| Subscriber route advertisement | NO -- spec-l2tp-7-subsystem (redistribute integration) |
| DHCPv6-PD | NO -- umbrella out-of-scope |
| SLAAC server-side | NO -- umbrella out-of-scope |
| CCP (compression) | NO -- umbrella out-of-scope |

## Required Reading

### Architecture Docs

- [ ] `plan/spec-l2tp-6a-lcp-base.md` -- LCP FSM (reusable for NCPs per RFC 1661)
  -> Decision: NCP FSMs reuse 6a's LCP FSM machinery via type parameterization OR a shared package-private state-machine engine; choose the smaller change at impl time
  -> Constraint: NCP packet codec is per-NCP; FSM transitions are identical to LCP
- [ ] `plan/spec-l2tp-6b-auth.md` -- auth -> NCP transition point
  -> Constraint: on `EventAuthSuccess` (or proxy-auth success), per-session goroutine spawns IPCP and IPv6CP state machines based on config
- [ ] `internal/component/iface/backend.go` -- existing `Backend` interface
  -> Constraint: extend with `AddAddressP2P(name, localCIDR, peerCIDR string) error`; precedent is `ReplaceAddressWithLifetime` for DHCP (line 58)
  -> Constraint: `iface.GetBackend()` is the read-only accessor; backend is loaded once during component init
- [ ] `internal/component/iface/default_linux.go` -- netlink backend implementation site
  -> Constraint: `AddAddressP2P` implemented via netlink RTM_NEWADDR with IFA_LOCAL + IFA_ADDRESS for point-to-point semantics
- [ ] `.claude/rules/buffer-first.md`
  -> Constraint: NCP packet encoding via offset writes
- [ ] `docs/research/l2tpv2-ze-integration.md` Section 9.5 -- IPv6 routes
  -> Constraint: IPv6CP only negotiates interface identifier; no address assignment via IPv6CP; the subscriber gets a link-local address derived from the negotiated identifier; further addressing (DHCPv6-PD or SLAAC) is out of umbrella scope

### RFC Summaries (MUST for protocol work)

- [ ] `rfc/short/rfc1332.md` -- IPCP
  -> Constraint: IPCP shares LCP packet structure (Code/Identifier/Length/Data) with codes 1-7; option type 3 = IP-Address; deprecated option types (1 IP-Addresses, 2 IP-Compression-Protocol) NOT implemented
- [ ] `rfc/short/rfc1877.md` -- IPCP DNS option (CREATE if missing)
  -> Constraint: option type 129 = Primary-DNS, 131 = Secondary-DNS, both 4-byte IPv4 addresses; informational; ze treats them as input from the handler, not negotiated
- [ ] `rfc/short/rfc5072.md` -- IPv6CP
  -> Constraint: only option type 1 (Interface-Identifier) is widely used; option type 2 (Compression) NOT implemented; interface-identifier is 8 bytes representing the EUI-64 interface ID

**Key insights:**
- IPCP and IPv6CP run AFTER authentication completes; before that, packets for these protocols are silently discarded (or queued, depending on impl preference)
- Both NCPs use the LCP FSM verbatim with different option types -- factor this once
- IP allocation (which address to give the subscriber) is the l2tp-pool plugin's job (Phase 8); 6c only handles wire format and the request/response shape

## Current Behavior (MANDATORY)

**Source files read:**

- [ ] `internal/component/ppp/lcp_fsm.go` (after 6a) -- 10-state FSM for LCP
  -> Constraint: factor into a generic `pppFSM` engine parameterized by option codec, so IPCP and IPv6CP reuse the state transitions
- [ ] `internal/component/ppp/auth.go` (after 6b) -- auth phase exit
  -> Constraint: on auth success, transition to NCP phase; new method on the per-session goroutine starts NCPs in parallel
- [ ] `internal/component/iface/backend.go` -- existing methods
  -> Constraint: `SetMTU`, `AddAddress`, `AddRoute`, `SetAdminUp`, `RemoveAddress`, `RemoveRoute`, `SetAdminDown` already exist; only `AddAddressP2P` is new
- [ ] `internal/component/iface/default_linux.go` (or analogous backend impl file) -- netlink layer
  -> Constraint: vendored `vishvananda/netlink` v1.3.1 supports AddrAdd with Peer field; use that

**Behavior to preserve:**
- All Phase 6a + 6b behavior unchanged
- LCP FSM extracted/factored ONLY if reuse demands it; otherwise duplicated per NCP (3 instances does justify factoring per `design-principles.md`)
- `iface.Backend` existing methods unchanged

**Behavior to change:**
- Auth-success transition now starts NCPs (was: emit `EventSessionUp` immediately after auth success in 6b)
- `EventSessionUp` now emitted only after at least one NCP completes AND pppN is configured AND interface up
- New `EventSessionIPAssigned` event carrying assigned addresses
- `iface.Backend` extended with `AddAddressP2P`

## Data Flow (MANDATORY)

### Entry Point

- Auth phase emits `EventAuthSuccess` (6b)
- Per-session goroutine reads config to determine which NCPs to run (`enable-ipcp bool`, `enable-ipv6cp bool`, both default true)
- Spawns IPCP state machine, IPv6CP state machine (both reuse the generic FSM engine)

### Transformation Path

1. IPCP sends Configure-Request with IP-Address=0.0.0.0 (request peer to assign) OR with our assigned local address
2. Peer responds with Configure-Nak suggesting addresses (or Configure-Ack)
3. PPP emits `EventIPRequest{family=ipv4, session-id, requested-local, requested-peer}` to events channel
4. External handler (l2tp-pool plugin in Phase 8; in-test stub here) calls `Manager.IPResponse(sessionID, family=ipv4, localAddr, peerAddr, dnsPrimary, dnsSecondary)`
5. PPP completes IPCP negotiation using returned addresses; sends Configure-Ack with final values
6. On IPCP-Opened: call `iface.Backend.AddAddressP2P(pppN, localAddr, peerAddr)`; call `iface.Backend.AddRoute(pppN, peerAddr+"/32", "", 0)`; call `iface.Backend.SetAdminUp(pppN)` (idempotent)
7. Emit `EventSessionIPAssigned{family=ipv4, ...}`
8. IPv6CP runs in parallel: send Configure-Request with our 64-bit interface identifier; peer responds with theirs
9. On IPv6CP-Opened: kernel auto-derives link-local address; emit `EventSessionIPAssigned{family=ipv6, interface-id}` (no `iface.Backend` call needed because ze does NOT assign /64 prefixes here -- DHCPv6-PD/SLAAC out of scope)
10. When BOTH NCPs complete (or one completes and the other is disabled): emit `EventSessionUp`

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Auth -> NCP phase | per-session goroutine state transition | [ ] |
| PPP -> external handler | `EventIPRequest` on events channel | [ ] |
| External handler -> PPP | `Manager.IPResponse(sessionID, family, ...)` method | [ ] |
| PPP -> netlink | `iface.GetBackend().AddAddressP2P/AddRoute/SetAdminUp` | [ ] |
| PPP -> L2TP subsystem | `EventSessionIPAssigned` on events channel | [ ] |

### Integration Points
- New `Manager.IPResponse` method
- `iface.Backend` extension: `AddAddressP2P(name, localCIDR, peerCIDR string) error`
- Backend impl in the netlink backend file (probably `internal/component/iface/default_linux.go` or wherever the production backend lives)
- L2TP reactor extends its event-handling switch to react to `EventSessionIPAssigned` (Phase 7 wires this into redistribute; here just route it into a no-op handler that logs)

### Architectural Verification
- [ ] No bypassed layers (PPP still does not import l2tp; iface backend extension is uniform with prior precedents)
- [ ] No unintended coupling (NCPs reuse the generic FSM; `AddAddressP2P` extension does not change unrelated `iface.Backend` callers)
- [ ] No duplicated functionality (factor LCP FSM once for reuse by NCPs)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Auth-success emitted | -> | NCPs started in parallel | `TestAuthSuccessStartsNCPs` (ppp/ncp_test.go) |
| IPCP Configure-Request (no address) sent | -> | PPP emits `EventIPRequest{ipv4}` | `TestIPCPInitialRequestEmitsEvent` (ppp/ipcp_test.go) |
| `Manager.IPResponse(sessionID, ipv4, local, peer, ...)` called | -> | IPCP completes; iface.Backend.AddAddressP2P + AddRoute + SetAdminUp called | `TestIPResponseConfiguresInterface` (ppp/manager_test.go, fake backend) |
| IPCP-Opened | -> | `EventSessionIPAssigned{ipv4}` emitted | `TestIPCPOpenedEmitsAssigned` (ppp/manager_test.go) |
| IPv6CP Configure-Request | -> | Interface-Identifier proposed | `TestIPv6CPProposesInterfaceID` (ppp/ipv6cp_test.go) |
| IPv6CP-Opened with peer ID | -> | `EventSessionIPAssigned{ipv6, interface-id}` emitted | `TestIPv6CPOpenedEmitsAssigned` (ppp/manager_test.go) |
| Both NCPs complete | -> | `EventSessionUp` emitted | `TestBothNCPsComplete` (ppp/manager_test.go) |
| Only IPCP enabled, IPv6CP disabled in config | -> | `EventSessionUp` emitted on IPCP success alone | `TestSingleNCPCompletes` (ppp/manager_test.go) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Auth-success with `enable-ipcp=true`, `enable-ipv6cp=true` | Both NCP state machines started in parallel from per-session goroutine |
| AC-2 | IPCP starts | Configure-Request sent with IP-Address option = 0.0.0.0 (request peer to assign) |
| AC-3 | Peer responds with Configure-Nak suggesting peer-address | PPP emits `EventIPRequest{family=ipv4, requested-peer}` to handler |
| AC-4 | Handler calls `IPResponse(id, ipv4, local=10.0.0.1, peer=10.0.0.2, dns1, dns2)` | PPP sends Configure-Request with IP-Address=10.0.0.2; on Ack -> IPCP-Opened |
| AC-5 | IPCP-Opened | `iface.Backend.AddAddressP2P(pppN, "10.0.0.1/32", "10.0.0.2/32")` called |
| AC-6 | IPCP-Opened (continued) | `iface.Backend.AddRoute(pppN, "10.0.0.2/32", "", 0)` called |
| AC-7 | IPCP-Opened (continued) | `iface.Backend.SetAdminUp(pppN)` called (idempotent) |
| AC-8 | IPCP-Opened (continued) | `EventSessionIPAssigned{family=ipv4, local=10.0.0.1, peer=10.0.0.2, dns1, dns2}` emitted |
| AC-9 | IPv6CP starts | Configure-Request sent with Interface-Identifier option = locally-generated 8 bytes |
| AC-10 | Peer responds with their Interface-Identifier | PPP completes negotiation; IPv6CP-Opened |
| AC-11 | IPv6CP-Opened | `EventSessionIPAssigned{family=ipv6, interface-id}` emitted; NO `iface.Backend` address call (kernel auto-derives link-local) |
| AC-12 | Both NCPs reach Opened | `EventSessionUp` emitted ONCE |
| AC-13 | `enable-ipv6cp=false` config | IPv6CP not started; `EventSessionUp` emitted on IPCP-Opened alone |
| AC-14 | `enable-ipcp=false`, `enable-ipv6cp=true` | IPv6CP runs alone; `EventSessionUp` emitted on IPv6CP-Opened |
| AC-15 | Both NCPs disabled | Logged warning; `EventSessionUp` emitted immediately after auth (config error, but not crash) |
| AC-16 | IPCP fails (peer rejects with Configure-Reject for IP-Address) | `EventSessionDown` emitted; LCP Terminate-Request sent |
| AC-17 | Handler does not call `IPResponse` within `ip-timeout` (default 30s) | `EventSessionDown` emitted; LCP Terminate-Request sent |
| AC-18 | Session teardown via L2TP CDN | `iface.Backend.RemoveRoute` and `RemoveAddress` called for the assigned addresses; pppN remains (kernel removes when unit fd closed via Phase 5 cleanup) |
| AC-19 | `iface.Backend.AddAddressP2P` returns error | `EventSessionDown` emitted; LCP Terminate-Request sent |
| AC-20 | IPCP-Opened then peer sends Configure-Request again (renegotiation) | PPP handles per RFC 1661 §4.3 -- transition to Req-Sent, re-negotiate |

## TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNCPFSMShared` | `internal/component/ppp/ncp_fsm_test.go` | Generic FSM engine drives both LCP and NCP shapes correctly |
| `TestIPCPParseRequest` | `internal/component/ppp/ipcp_test.go` | IPCP Configure-Request decode with IP-Address + DNS options | |
| `TestIPCPWriteRequest` | `internal/component/ppp/ipcp_test.go` | IPCP Configure-Request encode with skip-and-backfill length | |
| `TestIPCPDNSOptionParse` | `internal/component/ppp/ipcp_test.go` | RFC 1877 Primary-DNS / Secondary-DNS options | |
| `TestIPCPNegotiationHappyPath` | `internal/component/ppp/ipcp_test.go` | Request -> Nak -> Request -> Ack flow | |
| `TestIPCPInitialRequestEmitsEvent` | `internal/component/ppp/ipcp_test.go` | EventIPRequest emitted after first peer Nak | |
| `TestIPv6CPParseRequest` | `internal/component/ppp/ipv6cp_test.go` | IPv6CP Configure-Request decode with Interface-Identifier option | |
| `TestIPv6CPWriteRequest` | `internal/component/ppp/ipv6cp_test.go` | IPv6CP Configure-Request encode | |
| `TestIPv6CPProposesInterfaceID` | `internal/component/ppp/ipv6cp_test.go` | Local interface ID generation (non-zero, not all-ones) | |
| `TestIPv6CPNegotiation` | `internal/component/ppp/ipv6cp_test.go` | Standard Request/Ack flow | |
| `TestAuthSuccessStartsNCPs` | `internal/component/ppp/ncp_test.go` | After auth-success, both NCPs spawn (or one if disabled) | |
| `TestBothNCPsComplete` | `internal/component/ppp/manager_test.go` | EventSessionUp emitted only after both Opened | |
| `TestSingleNCPCompletes` | `internal/component/ppp/manager_test.go` | enable-ipv6cp=false -> EventSessionUp on IPCP-Opened | |
| `TestIPResponseConfiguresInterface` | `internal/component/ppp/manager_test.go` | Fake backend records AddAddressP2P + AddRoute + SetAdminUp calls with correct args | |
| `TestIPCPOpenedEmitsAssigned` | `internal/component/ppp/manager_test.go` | EventSessionIPAssigned ipv4 emitted | |
| `TestIPv6CPOpenedEmitsAssigned` | `internal/component/ppp/manager_test.go` | EventSessionIPAssigned ipv6 emitted | |
| `TestIPCPRejectTearsDown` | `internal/component/ppp/ipcp_test.go` | Configure-Reject for IP-Address triggers SessionDown |
| `TestIPTimeout` | `internal/component/ppp/manager_test.go` | No IPResponse within ip-timeout triggers SessionDown |
| `TestSessionTeardownRemovesAddress` | `internal/component/ppp/manager_test.go` | StopSession calls RemoveAddress + RemoveRoute on backend |
| `TestAddAddressP2PCalledCorrectly` | `internal/component/iface/iface_test.go` | New `Backend.AddAddressP2P` interface method, with one fake-backend test |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| IPCP packet length | 4-1500 | 1500 | 3 | 1501 (malformed) |
| IPCP option length | 2-255 | 255 | 1 | N/A (uint8) |
| IPv6CP packet length | 4-1500 | 1500 | 3 | 1501 |
| IPv6CP Interface-Identifier length | 8 (fixed) | 8 | 7 | 9 |
| IPCP IP-Address option length | 6 (fixed: type+len+4-byte addr) | 6 | 5 | 7 |
| IPCP Primary-DNS option length | 6 (fixed) | 6 | 5 | 7 |
| ip-timeout seconds | 1-300 | 300 | 0 | 301 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipcp-net-pipe` | `internal/component/ppp/manager_test.go::TestIPCPNetPipe` | Full IPCP exchange with scripted peer; in-test handler returns addresses | |
| `ipv6cp-net-pipe` | `internal/component/ppp/manager_test.go::TestIPv6CPNetPipe` | Full IPv6CP exchange; in-test handler returns interface-id | |
| `parallel-ncps-net-pipe` | `internal/component/ppp/manager_test.go::TestParallelNCPsNetPipe` | Both NCPs in parallel against scripted peer | |

### Future (if deferring any tests)

- `.ci` test against accel-ppp peer with l2tp-pool plugin -- deferred to spec-l2tp-7-subsystem + spec-l2tp-8-plugins (l2tp-pool)

## Files to Modify

- `internal/component/ppp/auth.go` -- on auth-success, start NCPs (was: emit EventSessionUp directly in 6b); EventSessionUp moves to NCP completion handler
- `internal/component/ppp/lcp_fsm.go` -- factor generic FSM engine into new file `ppp_fsm.go`; LCP becomes one user
- `internal/component/ppp/manager.go` -- add `IPResponse(sessionID, family, local, peer, dns1, dns2)` method; route to per-session channel
- `internal/component/ppp/session.go` -- add NCP state, IPCP/IPv6CP sub-state, per-session IP-response channel
- `internal/component/ppp/start_session.go` -- add `EnableIPCP`, `EnableIPv6CP`, `IPTimeout` fields
- `internal/component/ppp/events.go` -- add `EventIPRequest`, `EventSessionIPAssigned`
- `internal/component/iface/backend.go` -- add `AddAddressP2P(name, localCIDR, peerCIDR string) error` to `Backend` interface
- `internal/component/iface/default_linux.go` (or production backend file) -- implement `AddAddressP2P` via netlink AddrAdd with Peer field
- All other `Backend` implementors (test fakes, stubs) -- implement `AddAddressP2P` (compilation enforces)

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | N/A in 6c; NCP enable/disable + ip-timeout via env vars; YANG wiring in spec-l2tp-7-subsystem |
| CLI commands/flags | [ ] | N/A |
| Editor autocomplete | [ ] | N/A |
| Functional test for new RPC/API | [x] | `internal/component/ppp/manager_test.go` (`.ci` deferred to Phase 7) |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | N/A in 6c (full L2TP user-reachable scenario in Phase 7) |
| 2 | Config syntax changed? | [ ] | N/A in 6c |
| 3 | CLI command added/changed? | [ ] | N/A in 6c |
| 4 | API/RPC added/changed? | [ ] | N/A in 6c |
| 5 | Plugin added/changed? | [ ] | N/A in 6c (l2tp-pool plugin is Phase 8) |
| 6 | Has a user guide page? | [ ] | N/A |
| 7 | Wire format changed? | [ ] | N/A (PPP wire format scope unchanged) |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc1332.md` (extend), `rfc/short/rfc1877.md` (CREATE), `rfc/short/rfc5072.md` (extend) |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A in 6c |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` -- note `iface.Backend.AddAddressP2P` extension and the rationale (P2P semantics for PPP-style links) |

## Files to Create

- `internal/component/ppp/ppp_fsm.go` -- generic FSM engine factored from `lcp_fsm.go`
- `internal/component/ppp/ipcp.go` -- IPCP packet codec, options
- `internal/component/ppp/ipv6cp.go` -- IPv6CP packet codec, options
- `internal/component/ppp/ncp.go` -- NCP coordinator (parallel execution, completion tracking, EventSessionUp emission)
- `internal/component/ppp/ncp_test.go`
- `internal/component/ppp/ncp_fsm_test.go`
- `internal/component/ppp/ipcp_test.go`
- `internal/component/ppp/ipv6cp_test.go`
- `rfc/short/rfc1877.md` -- IPCP DNS option summary

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + 6a + 6b + umbrella |
| 2. Audit | Files to Modify, Files to Create |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-verify-fast` |
| 5-12 | Standard flow |

### Implementation Phases

1. **Factor FSM engine** -- extract `lcp_fsm.go` into `ppp_fsm.go` (generic) + `lcp_fsm.go` (now thin LCP-specific wrapper). Tests: `TestNCPFSMShared`, existing LCP tests still pass.
2. **`iface.Backend.AddAddressP2P`** -- add to interface + implement in production backend + update all implementors. Test: `TestAddAddressP2PCalledCorrectly`. RFC: none (netlink only).
3. **IPCP codec + FSM** -- `ipcp.go`. Tests: `TestIPCPParseRequest`, `TestIPCPWriteRequest`, `TestIPCPDNSOptionParse`, `TestIPCPNegotiationHappyPath`, `TestIPCPRejectTearsDown`. RFC 1332 + RFC 1877.
4. **IPv6CP codec + FSM** -- `ipv6cp.go`. Tests: `TestIPv6CPParseRequest`, `TestIPv6CPWriteRequest`, `TestIPv6CPProposesInterfaceID`, `TestIPv6CPNegotiation`. RFC 5072.
5. **NCP coordinator** -- `ncp.go`. Drives IPCP+IPv6CP from auth-success. Tests: `TestAuthSuccessStartsNCPs`, `TestBothNCPsComplete`, `TestSingleNCPCompletes`.
6. **IPResponse routing** -- extend `manager.go`, `session.go`, `events.go`. Tests: `TestIPCPInitialRequestEmitsEvent`, `TestIPResponseConfiguresInterface`, `TestIPCPOpenedEmitsAssigned`, `TestIPv6CPOpenedEmitsAssigned`, `TestIPTimeout`.
7. **EventSessionUp move** -- emit only after NCPs complete (was 6b: after auth-success). Adapt 6b tests if they assert SessionUp timing.
8. **Net.Pipe end-to-end** -- extend `manager_test.go`. Tests: `TestIPCPNetPipe`, `TestIPv6CPNetPipe`, `TestParallelNCPsNetPipe`.
9. **Teardown** -- StopSession removes address + route. Test: `TestSessionTeardownRemovesAddress`.
10. **RFC summaries** -- create `rfc/short/rfc1877.md`; extend `rfc1332.md` and `rfc5072.md`.
11. **Functional verification** -- `make ze-verify-fast`.

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has a Go test naming a file:line; assertion verifies AC behavior |
| Correctness | IPCP/IPv6CP wire format matches RFC byte-for-byte |
| Naming | Types: `IPCPPacket`, `IPv6CPPacket`; events: `EventIPRequest`, `EventSessionIPAssigned` |
| Data flow | NCPs run in parallel; EventSessionUp gated on completion of all enabled NCPs |
| Rule: no-layering | LCP FSM factored, not duplicated |
| Rule: buffer-first | All packet encoding via offset writes |
| Rule: design-principles "three concrete uses" | Three FSM users (LCP, IPCP, IPv6CP) justify the factored engine |
| iface.Backend extension | AddAddressP2P implemented in every Backend implementor (compilation enforces); document in core-design.md |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Generic FSM engine factored | `grep -c "type.*FSM" internal/component/ppp/ppp_fsm.go` returns 1; LCP, IPCP, IPv6CP each instantiate it |
| iface.Backend.AddAddressP2P exists and works | `go doc codeberg.org/thomas-mangin/ze/internal/component/iface.Backend.AddAddressP2P` |
| Both NCPs complete drives EventSessionUp | `TestBothNCPsComplete` passes |
| pppN configured after IPCP | `TestIPResponseConfiguresInterface` passes (fake backend records calls) |
| Subscriber addresses removed on teardown | `TestSessionTeardownRemovesAddress` passes |
| RFC summaries exist | `ls rfc/short/rfc1877.md` -> exists |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | IPCP/IPv6CP packet length validated; option lengths bounded; IP-Address option exact-length 6, Interface-Identifier exact-length 10 |
| Resource exhaustion | Per-session NCP retransmit count bounded (reuse LCP retransmit cap) |
| Address spoofing | ze does not accept arbitrary peer-supplied addresses unless the handler approves; the IP-Address negotiation path requires `IPResponse` from the handler before final Configure-Ack |
| Interface ID quality | Local interface-identifier generated via `crypto/rand`; not all-zeros, not all-ones (RFC 5072 §3.2) |
| iface backend errors | All `iface.Backend.*` errors handled; failure tears down session; no half-configured pppN left |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read RFC 1332 / 1877 / 5072 |
| iface backend test fails | Verify all backend implementors updated for `AddAddressP2P` |
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

- IPCP and IPv6CP share the LCP FSM machinery; the third user (NCP) is the threshold for factoring per `design-principles.md` "three concrete implementations". 6c is the spec where this factoring earns its keep.
- IPv6CP is much smaller in scope than people expect: it negotiates a 64-bit interface identifier and nothing else. The actual IPv6 addressing for subscribers is DHCPv6-PD or SLAAC, both out of umbrella scope. ze gets the link-local address for free via the kernel once the interface ID is set.
- `iface.Backend.AddAddressP2P` is the second extension in the precedent chain (after `ReplaceAddressWithLifetime` for DHCP). Each extension is small and serves one concrete need.

## RFC Documentation

Add `// RFC 1332 Section X.Y: "..."` above IPCP packet handlers and IP-Address option negotiation.
Add `// RFC 1877 Section X.Y: "..."` above DNS-Primary / DNS-Secondary option handling.
Add `// RFC 5072 Section X.Y: "..."` above IPv6CP packet handlers and Interface-Identifier negotiation; document non-zero, non-ones requirement (Section 3.2).

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
- [ ] AC-1..AC-20 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify-fast` passes
- [ ] Feature code integrated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC 1332, 1877, 5072 constraint comments added
- [ ] RFC summary (rfc1877) exists
- [ ] Implementation Audit complete

### Design
- [ ] No premature abstraction (FSM factor justified by 3 users)
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior (Go-level via net.Pipe)

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Summary included in commit

# Spec: l2tp-0 -- L2TPv2 LNS/LAC Subsystem (Umbrella)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-04-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `docs/research/l2tpv2-implementation-guide.md` -- protocol spec
4. `docs/research/l2tpv2-ze-integration.md` -- ze integration design
5. Child specs: `spec-l2tp-1-*` through `spec-l2tp-8-*`

## Task

Implement an L2TPv2 (RFC 2661) subsystem for ze that can act as both LNS
(L2TP Network Server) and LAC (L2TP Access Concentrator). The subsystem
terminates L2TP tunnels over UDP, manages PPP sessions within those tunnels,
assigns IP addresses to subscribers, and redistributes subscriber routes
into BGP.

This is a BNG (Broadband Network Gateway) feature. L2TP tunnels carry PPP
sessions from remote subscribers. The LNS terminates PPP, assigns IPs, and
provides network access. Routes for subscriber IPs are advertised via BGP
to upstream routers.

### Design Decisions (agreed with user)

| Decision | Detail |
|----------|--------|
| Linux-only | No platform abstraction. Direct netlink, PPPoL2TP sockets, `/dev/ppp`. Build-tagged `_linux.go` files. |
| Kernel data plane | Kernel L2TP modules (`l2tp_ppp`) handle data encap/decap. Ze handles all control plane (L2TP control messages, PPP negotiation, IP assignment) in userspace via `/dev/ppp` fds. |
| L2TP is a subsystem | Implements `ze.Subsystem` (Start/Stop/Reload). Registered with engine via `engine.RegisterSubsystem()`. Same tier as BGP. |
| Reactor pattern | Single reactor goroutine reads shared UDP socket, dispatches to tunnel state machines. Timer goroutine for retransmission/hello. PPP worker pool for blocking `/dev/ppp` I/O. No goroutine-per-tunnel. |
| Single unconnected UDP socket | One listener socket for all tunnels. `sendto()` per tunnel peer. Scales to thousands of tunnels with one fd. |
| Policy via plugins | Authentication (RADIUS), IP pools, traffic shaping, and accounting are plugins, not hardcoded in the subsystem. |
| Buffer-first encoding | All L2TP wire encoding uses pooled buffers, offset-based writes, skip-and-backfill. No `append()` or `make([]byte)` in encoding helpers. |
| Redistribute integration | L2TP registers as a redistribute source. Subscriber /32 routes flow through protocol RIB, sysrib, and fibkernel to both kernel FIB and BGP. |
| L2TPv2 only | L2TPv3 detected and rejected (StopCCN Result Code 5). L2F silently discarded. |

### Scope

**In Scope:**

| Area | Description |
|------|-------------|
| L2TP wire format | Header parsing/serialization, all 40 AVP types, hidden AVP encryption |
| Reliable delivery | Ns/Nr sequencing, retransmission with exponential backoff, sliding window, slow start, congestion avoidance |
| Tunnel state machine | SCCRQ/SCCRP/SCCCN handshake, StopCCN teardown, HELLO keepalive, challenge/response authentication, tie breaker |
| Session state machine | Incoming calls (ICRQ/ICRP/ICCN), outgoing calls (OCRQ/OCRP/OCCN), CDN teardown |
| Kernel integration | Generic Netlink for tunnel/session management, PPPoL2TP socket API, `/dev/ppp` channel/unit management |
| PPP engine | LCP negotiation (MRU, auth method, echo), PAP/CHAP/MS-CHAPv2, IPCP (IPv4 + DNS), IPv6CP (interface ID) |
| Subsystem wiring | Event namespace, YANG config, env vars, CLI commands, redistribute source, Prometheus metrics |
| Authentication plugin | RADIUS auth/acct, CoA/DM |
| IP pool plugin | IPv4/IPv6 address pools with RADIUS-directed selection |
| Traffic shaping plugin | TC rules on pppN interfaces |
| Statistics plugin | Session accounting, Prometheus metrics |

**Out of Scope:**

| Area | Reason |
|------|--------|
| L2TPv3 | Different protocol (32-bit IDs, IP encap). Separate spec if needed. |
| CCP/MPPE encryption | PPP compression/encryption within sessions. Defer to demand. |
| Multilink PPP | Bundle multiple sessions into one logical link. Defer to demand. |
| macOS/BSD support | Linux-only decision. Kernel L2TP modules are Linux-specific. |
| IPsec integration | Transparent to L2TP (operates below UDP). Configure externally. |
| DHCPv6-PD | IPv6 prefix delegation is a separate protocol running over established PPP. Separate spec. |

### Reference Documents

| Document | Location | Purpose |
|----------|----------|---------|
| Protocol spec | `docs/research/l2tpv2-implementation-guide.md` | Wire format, state machines, AVPs, reliable delivery, crypto |
| Ze integration | `docs/research/l2tpv2-ze-integration.md` | Subsystem lifecycle, plugins, events, redistribution, buffer-first |
| RFC 2661 | L2TP | Primary protocol reference |
| RFC 1661 | LCP | PPP link control, FSM (10 states, ~30 transitions) |
| RFC 1332 | IPCP | IPv4 address negotiation |
| RFC 5072 | IPv6CP | IPv6 interface identifier negotiation |
| RFC 1994 | CHAP | Challenge-handshake authentication |

### Child Specs

| Phase | Spec | Scope | Depends |
|-------|------|-------|---------|
| 1 | `spec-l2tp-1-wire.md` | L2TP header parsing/serialization, AVP parsing/serialization (all 40 types), hidden AVP encryption/decryption, challenge/response computation. Buffer-first encoding. Fuzz tests. | - |
| 2 | `spec-l2tp-2-reliable.md` | Reliable delivery engine: Ns/Nr sequencing, retransmission with exponential backoff, sliding window, slow start / congestion avoidance, ZLB acknowledgment, duplicate detection, post-teardown state retention. | l2tp-1 |
| 3 | `spec-l2tp-3-tunnel.md` | Tunnel state machine (4 states): SCCRQ/SCCRP/SCCCN handshake, StopCCN teardown, HELLO keepalive, challenge/response authentication, tie breaker resolution. UDP listener. Tunnel management (create, lookup, destroy). | l2tp-2 |
| 4 | `spec-l2tp-4-session.md` | Session state machine: incoming calls (LNS: idle, wait-connect, established; LAC: idle, wait-tunnel, wait-reply, established), outgoing calls (LNS and LAC sides), CDN teardown. Session management within tunnels. WEN and SLI message handling. | l2tp-3 |
| 5 | `spec-l2tp-5-kernel.md` | Linux kernel integration: Generic Netlink for L2TP tunnel/session creation and deletion, PPPoL2TP socket creation and lifecycle, `/dev/ppp` channel/unit management (PPPIOCGCHAN, PPPIOCATTCHAN, PPPIOCNEWUNIT, PPPIOCCONNECT), pppN interface creation, cleanup ordering. Kernel module probing at startup. | l2tp-4 |
| 6a | `spec-l2tp-6a-lcp-base.md` | PPP scaffold (`internal/component/ppp/` peer of l2tp), per-session goroutine model, PPP frame I/O via Go runtime poller, LCP FSM (RFC 1661, 10 states), LCP options (MRU, Auth-Proto, Magic, ACCM, PFC, ACFC), Echo keepalive, proxy LCP (RFC 2661 §18), pppN MTU set via `iface.Backend`, auth-phase hook (stubbed), new `kernelSetupSucceeded` event from L2TP kernel worker. | l2tp-5 |
| 6b | `spec-l2tp-6b-auth.md` | PPP authentication: PAP (RFC 1334), CHAP-MD5 (RFC 1994), MS-CHAPv2 (RFC 2759), proxy authentication (RFC 2661 §18). `EventAuthRequest`/`Manager.AuthResponse` channel-based dispatch; ze handles wire format only, RADIUS query lives in l2tp-auth plugin (Phase 8). Replaces 6a stub. | l2tp-6a |
| 6c | `spec-l2tp-6c-ncp.md` | IPCP (RFC 1332) + DNS option (RFC 1877), IPv6CP (RFC 5072, interface identifier only). Parallel NCP execution after auth. `EventIPRequest`/`Manager.IPResponse` channel flow. pppN address via `iface.Backend.AddAddressP2P` (interface extension). `EventSessionIPAssigned` emitted for redistribute integration in Phase 7. | l2tp-6b |
| 7 | `spec-l2tp-7-subsystem.md` | Ze subsystem wiring: `ze.Subsystem` implementation (Start/Stop/Reload), event namespace (`l2tp`) and event types in `events.go`, YANG schema (`ze-l2tp-conf.yang`), env var registration, CLI commands (`show l2tp tunnels/sessions`, `clear l2tp`), redistribute source registration, config transaction participation, main binary wiring, Prometheus metrics. | l2tp-6c |
| 8 | `spec-l2tp-8-plugins.md` | L2TP plugins: l2tp-auth (RADIUS authentication, accounting, CoA/DM), l2tp-pool (IPv4/IPv6 address pools, RADIUS-directed selection), l2tp-shaper (TC rules on pppN), l2tp-stats (session counters, Prometheus export). Plugin registration with correct `registry.Registration` fields. | l2tp-7 |

Phases are strictly ordered. Each phase must be complete before the next begins.

### Cross-Subsystem Integration

| Integration | How | Phase |
|-------------|-----|-------|
| BGP redistribute | `redistribute.RegisterSource("l2tp")` at init; subscriber /32 routes injected into protocol RIB on session-ip-assigned | 7 |
| sysrib | Receives L2TP routes via `(bgp-rib, best-change)`, selects system best by admin distance | 7 |
| fibkernel | Programs subscriber routes into kernel FIB with `RTPROT_ZE` | 7 |
| Interface monitor | Detects pppN creation/destruction via netlink, emits `(interface, created/up/down)` events | 5 (automatic) |
| BGP reactor | Subscribes to `(interface, addr-added)` for pppN addresses; advertises subscriber routes if `redistribute l2tp` configured | 7 |
| Config system | L2TP participates in verify/apply/rollback transaction protocol | 7 |

### End-to-End Subscriber Flow

1. LAC sends SCCRQ to ze's UDP listener (port 1701)
2. Ze processes tunnel handshake (SCCRQ/SCCRP/SCCCN) with optional challenge/response
3. LAC sends ICRQ, ze responds with ICRP, LAC confirms with ICCN
4. Ze creates kernel tunnel + session via Generic Netlink
5. Ze creates PPPoL2TP socket, attaches `/dev/ppp` channel and unit
6. Kernel creates pppN interface
7. Ze runs LCP negotiation via `/dev/ppp` channel fd
8. Ze runs PPP authentication (l2tp-auth plugin queries RADIUS)
9. Ze runs IPCP negotiation via `/dev/ppp` unit fd (l2tp-pool plugin allocates address)
10. Ze configures pppN interface via netlink (address, routes, MTU)
11. Ze emits `(l2tp, session-ip-assigned)` event
12. Redistribute injects /32 route into protocol RIB
13. sysrib selects best, fibkernel programs kernel route
14. BGP advertises subscriber route to peers (if `redistribute l2tp` configured)
15. IP traffic flows through pppN (kernel data plane, no ze involvement)
16. On teardown: reverse the above, withdraw routes, release IP, close kernel resources

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- subsystem, plugin, event bus patterns
  -> Constraint: subsystem implements `ze.Subsystem`, plugins use `registry.Registration`
- [ ] `.claude/rules/plugin-design.md` -- plugin registration, 5-stage protocol
  -> Constraint: `RunEngine func(conn net.Conn) int`, `CLIHandler` required
- [ ] `.claude/rules/buffer-first.md` -- wire encoding patterns
  -> Constraint: no `append()`, no `make()` in encoding helpers
- [ ] `.claude/rules/goroutine-lifecycle.md` -- concurrency patterns
  -> Constraint: reactor pattern, no goroutine-per-tunnel
- [ ] `docs/research/l2tpv2-implementation-guide.md` -- protocol spec (THIS IS THE PRIMARY REFERENCE)
- [ ] `docs/research/l2tpv2-ze-integration.md` -- ze integration design

### RFC Summaries
- [ ] RFC 2661 -- L2TP (create `rfc/short/rfc2661.md` if missing)
- [ ] RFC 1661 -- PPP LCP
- [ ] RFC 1332 -- PPP IPCP
- [ ] RFC 5072 -- IPv6CP
- [ ] RFC 1994 -- CHAP

**Key insights:**
- Kernel handles data plane only; all control (L2TP messages, PPP negotiation, IP assignment) in userspace
- L2TP is a subsystem (like BGP), not a plugin
- Protocol Version AVP = 1.0 but header Version = 2 (different version schemes)
- Framing/Bearer Capabilities bitmask: bit 0 = LSB (0x00000001), not MSB
- Congestion avoidance uses fractional counter (integer `1/CWND` = 0 for CWND > 1)
- IPv6CP only negotiates interface identifiers; prefix delegation is DHCPv6-PD (separate protocol)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/ze/subsystem.go` -- `ze.Subsystem` interface: Name, Start, Stop, Reload
- [ ] `pkg/ze/engine.go` -- `ze.Engine` interface: RegisterSubsystem, Start, Stop, Reload
- [ ] `internal/component/plugin/events.go` -- event namespaces and type registration
- [ ] `internal/component/bgp/redistribute/registry.go` -- redistribute source registration
- [ ] `internal/component/plugin/registry/registry.go` -- plugin Registration struct fields
- [ ] `internal/plugins/sysrib/sysrib.go` -- system RIB route selection
- [ ] `internal/plugins/fib/kernel/fibkernel.go` -- FIB kernel route programming

**Behavior to preserve:**
- Existing event namespaces and types unchanged
- Existing redistribute sources (ibgp, ebgp) unchanged
- Existing subsystem registration patterns unchanged
- Interface monitoring via netlink continues to work (pppN interfaces will be detected automatically)

**Behavior to change:**
- Add `NamespaceL2TP` and `ValidL2TPEvents` to `events.go`
- Add "l2tp" redistribute source to registry
- Add L2TP subsystem registration in engine startup path
- Add blank imports for L2TP plugins in `all/all.go`

## Acceptance Criteria (Umbrella Level)

These are end-to-end criteria. Each child spec has its own detailed ACs.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | LAC sends SCCRQ with valid challenge | Ze completes tunnel handshake (SCCRP + SCCCN), tunnel established |
| AC-2 | LAC sends ICRQ on established tunnel | Ze completes session setup (ICRP + ICCN), PPP negotiation succeeds, subscriber gets IP |
| AC-3 | Subscriber assigned IP via IPCP | /32 route appears in BGP (if `redistribute l2tp` configured), subscriber can send/receive IP traffic |
| AC-4 | Session teardown (CDN or admin clear) | PPP session closed, pppN interface removed, IP released, /32 route withdrawn from BGP |
| AC-5 | Tunnel teardown (StopCCN) | All sessions in tunnel torn down, kernel resources cleaned up |
| AC-6 | `show l2tp tunnels` CLI command | Lists all active tunnels with peer address, state, session count |
| AC-7 | `show l2tp sessions` CLI command | Lists all active sessions with interface, IP, username, state |
| AC-8 | L2TPv3 SCCRQ received | Rejected with StopCCN Result Code 5 |
| AC-9 | Invalid shared secret in challenge/response | Tunnel rejected with StopCCN Result Code 4 |
| AC-10 | Retransmission exhausted (no peer response) | Tunnel torn down after max retries |
| AC-11 | ze config with `l2tp { listen { port 1701; } }` | L2TP subsystem starts, listens on configured port |
| AC-12 | RADIUS authentication for PPP session | l2tp-auth plugin queries RADIUS, accepts/rejects based on response |
| AC-13 | Multiple sessions on one tunnel | Each session has independent pppN interface, independent IP, independent state |

## Data Flow (MANDATORY)

### Entry Point
- UDP packets on port 1701 (L2TP control and data messages from LAC peers)
- Config file: YANG-modeled `l2tp { ... }` block
- CLI commands: `show l2tp tunnels`, `clear l2tp session N`
- Plugin events: `(l2tp, session-auth-request)`, `(l2tp, session-ip-request)`

### Transformation Path

1. UDP socket receives packet. Reactor reads it.
2. Parse L2TP header (12 bytes control). Extract Tunnel ID, Session ID, Ns, Nr.
3. Look up tunnel by Tunnel ID in tunnel map.
4. Deliver to tunnel state machine. Parse AVPs from payload.
5. State machine processes message (SCCRQ/ICRQ/etc.), updates state, sends response.
6. On session established: create kernel tunnel + session via Generic Netlink.
7. Create PPPoL2TP socket, `/dev/ppp` channel + unit. Kernel creates pppN.
8. PPP worker picks up session, runs LCP/auth/IPCP via `/dev/ppp` reads/writes.
9. On IPCP complete: configure pppN via netlink, emit events, inject redistribute route.
10. Data flows through kernel (pppN interface). Ze not involved in data path.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| UDP wire -> tunnel state machine | Header parse, tunnel lookup by ID | [ ] |
| Tunnel SM -> session SM | ICRQ dispatched to session within tunnel | [ ] |
| Session -> kernel | Generic Netlink tunnel/session create, PPPoL2TP socket | [ ] |
| Kernel -> PPP engine | `/dev/ppp` channel/unit fds (blocking reads/writes) | [ ] |
| PPP engine -> plugins | EventBus: `(l2tp, session-auth-request)`, `(l2tp, session-ip-request)` | [ ] |
| PPP engine -> redistribute | EventBus: `(l2tp, session-ip-assigned)` -> protocol RIB | [ ] |
| Protocol RIB -> sysrib -> fibkernel | `(bgp-rib, best-change)` -> `(system-rib, best-change)` -> kernel route | [ ] |

### Integration Points
- `ze.Engine.RegisterSubsystem()` -- L2TP subsystem registration
- `redistribute.RegisterSource()` -- "l2tp" route source
- `plugin.RegisterEventType()` -- L2TP event namespace and types
- `env.MustRegister()` -- L2TP env vars
- `yang.RegisterModule()` -- L2TP YANG schema
- `iface.AddRoute()` -- subscriber route installation with metric

### Architectural Verification
- [ ] No bypassed layers (subscriber routes flow through protocol RIB, sysrib, fibkernel)
- [ ] No unintended coupling (L2TP subsystem uses event bus, not direct imports to BGP)
- [ ] No duplicated functionality (reuses existing redistribute, sysrib, fibkernel, interface monitor)
- [ ] Zero-copy preserved where applicable (buffer-first encoding, pool-based buffers)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with `l2tp { listen { port 1701; } }` | -> | L2TP subsystem starts, UDP listener bound | `test/l2tp/config-basic.ci` |
| LAC sends SCCRQ to ze | -> | Tunnel state machine processes, SCCRP sent | `test/l2tp/tunnel-setup.ci` |
| LAC sends ICRQ + ICCN | -> | Session created, PPP negotiation, IP assigned | `test/l2tp/session-setup.ci` |
| `show l2tp tunnels` CLI | -> | L2TP command handler returns tunnel list | `test/l2tp/cli-show.ci` |
| Session IP assigned | -> | /32 route appears in BGP table | `test/l2tp/redistribute.ci` |

## 🧪 TDD Test Plan

### Unit Tests

Detailed in child specs. Summary:

| Area | Spec | Key Tests |
|------|------|-----------|
| Header parse/serialize | l2tp-1 | Round-trip, boundary, fuzz |
| AVP parse/serialize | l2tp-1 | All 40 types, hidden AVPs, fuzz |
| Reliable delivery | l2tp-2 | Ns/Nr, retransmit, duplicate, wraparound |
| Tunnel state machine | l2tp-3 | All transitions in state table |
| Session state machine | l2tp-4 | Incoming/outgoing call flows |
| Kernel integration | l2tp-5 | Netlink tunnel/session create/delete |
| PPP LCP FSM | l2tp-6a | LCP 10-state transitions, option negotiation, proxy LCP |
| PPP authentication | l2tp-6b | PAP/CHAP-MD5/MS-CHAPv2 wire format, proxy auth |
| PPP NCPs | l2tp-6c | IPCP/IPv6CP negotiation, pppN configuration |
| Subsystem lifecycle | l2tp-7 | Start/Stop/Reload |
| Plugin registration | l2tp-8 | All four plugins register and start |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Tunnel ID | 1-65535 | 65535 | 0 | N/A (uint16) |
| Session ID | 1-65535 | 65535 | 0 | N/A (uint16) |
| Receive Window Size | 1-65535 | 65535 | 0 | N/A (uint16) |
| AVP Length | 6-1023 | 1023 | 5 | 1024 |
| UDP port | 1-65535 | 65535 | 0 | 65536 |
| PPP MRU | 64-65535 | 65535 | 63 | N/A |
| Hello interval | 1-65535 | 65535 | 0 | N/A |
| Retransmit max | 1-255 | 255 | 0 | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| L2TP config parse | `test/parse/l2tp-*.ci` | Valid L2TP config accepted | |
| Tunnel setup | `test/l2tp/tunnel-setup.ci` | Full SCCRQ/SCCRP/SCCCN exchange | |
| Session with PPP | `test/l2tp/session-setup.ci` | Session up, IP assigned, traffic flows | |
| CLI show commands | `test/l2tp/cli-show.ci` | `show l2tp tunnels` returns data | |
| Redistribute | `test/l2tp/redistribute.ci` | Subscriber route in BGP | |
| Auth reject | `test/l2tp/auth-reject.ci` | Bad credentials, session rejected | |
| Tunnel teardown | `test/l2tp/tunnel-teardown.ci` | StopCCN cleans up all sessions | |

## Files to Modify

Umbrella level. Detailed in child specs.

- `internal/component/plugin/events.go` -- add `NamespaceL2TP`, `ValidL2TPEvents`
- `internal/component/plugin/all/all.go` -- blank imports for L2TP plugins
- Engine startup path -- `engine.RegisterSubsystem(l2tp.NewSubsystem())`

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/component/l2tp/schema/ze-l2tp-conf.yang` |
| CLI commands/flags | [x] | `show l2tp tunnels`, `show l2tp sessions`, `clear l2tp` |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [x] | `test/l2tp/*.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- add L2TP/LNS/LAC |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- add L2TP config section |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` -- add `show l2tp` commands |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` -- L2TP commands |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` -- l2tp-auth, l2tp-pool, l2tp-shaper, l2tp-stats |
| 6 | Has a user guide page? | [x] | `docs/guide/l2tp.md` -- new |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [x] | `rfc/short/rfc2661.md` |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- L2TP column |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` -- add L2TP subsystem |

## Files to Create

Detailed per child spec. Umbrella summary:

- `internal/component/l2tp/` -- entire subsystem (header, avp, tunnel, session, reliable, ppp, kernel, events, config, register, schema)
- `internal/plugins/l2tpauth/` -- RADIUS authentication plugin
- `internal/plugins/l2tppool/` -- IP address pool plugin
- `internal/plugins/l2tpshaper/` -- traffic shaping plugin
- `internal/plugins/l2tpstats/` -- statistics plugin
- `test/l2tp/*.ci` -- functional tests
- `test/parse/l2tp-*.ci` -- config parse tests
- `rfc/short/rfc2661.md` -- RFC summary
- `docs/guide/l2tp.md` -- user guide

## Implementation Steps

Umbrella spec. Implementation is in child specs (l2tp-1 through l2tp-8).

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This umbrella + active child spec |
| 2. Audit | Child spec's Files to Modify/Create |
| 3. Implement (TDD) | Child spec's implementation phases |
| 4. Full verification | `make ze-verify` |
| 5-12 | Per child spec |

### Implementation Phases

Each child spec is one phase. Phases are strictly ordered by dependency.

1. **Phase 1: Wire format** (spec-l2tp-1-wire) -- parse/serialize L2TP headers and AVPs
2. **Phase 2: Reliable delivery** (spec-l2tp-2-reliable) -- Ns/Nr, retransmission, congestion
3. **Phase 3: Tunnel state machine** (spec-l2tp-3-tunnel) -- control connection lifecycle
4. **Phase 4: Session state machine** (spec-l2tp-4-session) -- call setup/teardown
5. **Phase 5: Kernel integration** (spec-l2tp-5-kernel) -- netlink, PPPoL2TP, /dev/ppp
6. **Phase 6: PPP engine** -- split into three sub-phases:
   - 6a (spec-l2tp-6a-lcp-base) -- LCP scaffold, FSM, proxy LCP, MTU set
   - 6b (spec-l2tp-6b-auth) -- PAP, CHAP-MD5, MS-CHAPv2, proxy auth
   - 6c (spec-l2tp-6c-ncp) -- IPCP, IPv6CP, pppN address/route configuration
7. **Phase 7: Subsystem wiring** (spec-l2tp-7-subsystem) -- events, config, CLI, redistribute
8. **Phase 8: Plugins** (spec-l2tp-8-plugins) -- auth, pool, shaper, stats

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N from umbrella + child demonstrated |
| Correctness | Wire format matches RFC 2661 byte-for-byte |
| Naming | JSON keys kebab-case, YANG kebab-case, Go packages no hyphens |
| Data flow | Subscriber route flows through RIB -> sysrib -> fibkernel -> kernel FIB |
| Rule: buffer-first | No `append()` or `make()` in encoding helpers |
| Rule: goroutine-lifecycle | Reactor pattern, PPP worker pool, no per-tunnel goroutines |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| L2TP subsystem starts and listens | `test/l2tp/config-basic.ci` passes |
| Tunnel handshake works | `test/l2tp/tunnel-setup.ci` passes |
| Session with PPP and IP | `test/l2tp/session-setup.ci` passes |
| CLI commands work | `test/l2tp/cli-show.ci` passes |
| Redistribute works | `test/l2tp/redistribute.ci` passes |
| All four plugins register | `make ze-inventory` shows l2tp-auth, l2tp-pool, l2tp-shaper, l2tp-stats |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | All L2TP header/AVP parsing validates lengths before reading. Fuzz tests cover malformed packets. |
| Resource exhaustion | Max tunnels/sessions limits enforced. SCCRQ rate limiting. |
| Authentication bypass | Challenge/response verified before tunnel establishment. Shared secret not logged. |
| Secret handling | `ze:sensitive` on shared-secret YANG leaf. `env.Secret` on env var. |
| Kernel resource leak | Cleanup ordering verified: PPPoL2TP socket -> kernel session -> kernel tunnel -> UDP socket. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read RFC 2661 and protocol spec |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Kernel integration fails | Check module loading, netlink attributes, fd ordering |
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

- Kernel PPP split is clean: data plane (IP forwarding) in kernel, control plane (LCP/auth/IPCP) in userspace via `/dev/ppp`. No control visibility lost.
- Single unconnected UDP socket scales better than per-tunnel connected sockets for BNG workload.
- The redistribution path (L2TP -> protocol RIB -> sysrib -> fibkernel) reuses existing infrastructure with zero new code in sysrib or fibkernel.
- PPP FSM (RFC 1661) is the most complex component: 10 states, ~30 transitions, shared across LCP/IPCP/IPv6CP/CCP. Getting this right is the single biggest implementation risk.

## RFC Documentation

Add `// RFC 2661 Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: AVP validation, state transitions, timer constraints, message ordering, sequence number arithmetic.

## Implementation Summary

### What Was Implemented
- (umbrella -- filled per child spec)

### Bugs Found/Fixed
- (umbrella -- filled per child spec)

### Documentation Updates
- (umbrella -- filled per child spec)

### Deviations from Plan
- (umbrella -- filled per child spec)

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
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
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

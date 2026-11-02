# Spec: home-cpe-12-pppoe-client-lcp-renegotiation

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Shipped PPPoE client misbehaves on an accepted configuration: an access network that renegotiates LCP after the LAC opened it (the wholesale L2TP shape) never gets its Configure-Request answered, so the session times out and the CPE never comes up. Blocks the home-CPE migration. |
| RFC gaps | RFC 1661 Section 3.4 and Section 4.1 (RCR in Opened, and RCR during Authentication and Network-Layer Protocol phases); RFC 1661 Section 3.6 (discard network-layer packets while IPCP is not Opened) on the renegotiation path |
| Depends | - for the code. The third-party proof needs `spec-home-cpe-14-lac-realm-relay.md`, `spec-home-cpe-18-interoplab-multi-link-topology.md` and `spec-home-cpe-19-access-chain-scenario.md`; the Ze-LNS proof needs `spec-home-cpe-15-lns-lcp-renegotiation.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-31 |

Parent: plan/home-cpe/spec-home-cpe-0-umbrella.md

Recovery after compaction: `.claude/rules/post-compaction.md`.

All values in this spec are invented (RFC 5737 prefixes, `example.net`).

## Task

In wholesale FTTP access the subscriber's PPPoE session first meets a LAC. The
LAC negotiates LCP and challenges CHAP itself, learns the realm, then hands the
session to the retail ISP's LNS over L2TP. An LNS that does not accept the
LAC's proxy LCP sends a fresh LCP Configure-Request with its own Magic-Number,
and challenges CHAP again, then runs IPCP. To the CPE this is a Configure-Request
arriving after LCP is Opened, during authentication, during IPCP, or in the
network phase once IPCP is up.

Ze's PPPoE client drops that Configure-Request in every phase after LCP opened.
The server retransmits until its Max-Configure counter runs out and tears the
session down; the client reconnects and the same thing happens again.

Goal: the client follows the RFC 1661 Section 4.1 Opened column (RCR+ is
tld, scr, sca to Ack-Sent; RCR- is tld, scr, scn to Req-Sent) in every phase
after LCP opened, then runs authentication and IPCP again on the same PPPoE
session and the same kernel PPP unit, and the interface keeps routing once IPCP
is Opened again.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/l2tp/cpe-1-pppoe-client.md` - the client's phases, its two readers, unit ownership
  → Decision: from `PPPIOCCONNECT` on, Linux queues IPCP on the unit descriptor and LCP and CHAP on the channel descriptor; `negotiateSession` starts the unit reader there and keepalive reads both
  → Constraint: the page says the client "has no renegotiation path" for IPCP and ends the session; that sentence changes with this spec
  → Constraint: session loss stops transport, not unit ownership; `Cleanup` releases the unit. A renegotiation keeps both
- [ ] `docs/architecture/l2tp/` LNS pages are not the subject: the LNS FSM in `internal/component/l2tp/ppp/` is a different code path (server side) and `plan/release/fixes/spec-lcp-restart-counters.md` owns its `tld` behaviour
  → Constraint: do not duplicate that spec; reuse `ppp.NegotiatePeerOptions` and the LCP state constants the client already uses

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc1661.md` (exists) - phase table, state table rows 9 Opened
  → Constraint: summary line "The receipt of a Configure-Request while in Network-Layer Protocol or Authentication phase returns the FSM to Link Establishment (Section 3.4)" is a derived claim; the quotes below are from `rfc/full/rfc1661.txt`
- [ ] `rfc/full/rfc1661.txt` Section 3.4: "The receipt of the LCP Configure-Request causes a return to the Link Establishment phase from the Network-Layer Protocol phase or Authentication phase."
- [ ] `rfc/full/rfc1661.txt` Section 4.1, Receive-Configure-Request implementation note: "These events may occur on a connection which is already in the Opened state.  The implementation MUST be prepared to immediately renegotiate the Configuration Options."
- [ ] `rfc/full/rfc1661.txt` Section 4.1 state table, Opened column: RCR+ "tld,scr,sca/8", RCR- "tld,scr,scn/6"
- [ ] `rfc/full/rfc1661.txt` Section 4.4, This-Layer-Down: "Typically, this action is used by the LCP to signal the Down event to a NCP, Authentication Protocol, or Link Quality Protocol"
- [ ] `rfc/full/rfc1661.txt` Section 3.6: "Any supported network-layer protocol packets received when the corresponding NCP is not in the Opened state MUST be silently discarded."
- [ ] `rfc/short/rfc1994.md` (CHAP) - the re-challenge after re-authentication uses the same Response builder

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/pppoeclient/session.go` (1054L) - `negotiateSession` runs `negotiateLCP`, then `runClientAuth` when an auth protocol was negotiated, then `negotiateIPCP`, then `ppp.Connect` (PPPIOCCONNECT) and `ppp.SetMRU`, then starts the unit reader. `runClientAuth` (l.455) handles only LCP Echo-Request and Terminate-Request among LCP codes; every other LCP packet, Configure-Request included, hits `continue`. `negotiateIPCP` (l.623) does the same on its LCP branch (Echo-Request, Terminate-Request, else `continue`)
- [ ] `internal/component/l2tp/pppoeclient/network_phase.go` (231L) - `keepaliveLoop` (l.94) reads the channel (LCP, CHAP) and the unit (IPCP). Its LCP switch handles Echo-Reply, Echo-Request and Terminate-Request; a Configure-Request falls out of the switch silently. `answerOpenedIPCP` ends the session on any IPCP packet that takes IPCP out of Opened
- [ ] `internal/component/l2tp/pppoeclient/dialer.go` (465L) - `Dial` creates the channel and unit with `ppp.DevPPPSetup` before LCP, starts one channel reader, runs `negotiateSession`, then `superviseNetworkPhase` runs `keepaliveLoop` in a goroutine. `PPPoESession.Done` closes on loss; the iface caller then reconnects
- [ ] `internal/component/iface/pppoe_client.go` (377L) - `runSession` applies MTU, admin-up, P2P address and default route once, after `Dial` returns; it never hears of a later change
- [ ] `internal/component/l2tp/ppp/devppp_linux.go` - `DevPPPSetup` (PPPIOCGCHAN, PPPIOCATTCHAN, PPPIOCNEWUNIT); no PPPIOCSNPMODE support exists anywhere under `internal/component/l2tp/` (grep for npmode is empty)
- [ ] accel-ppp source (`github.com/accel-ppp/accel-ppp`, local checkout): `accel-pppd/ppp/ppp_lcp.c` starts LCP once (`ppp_fsm_lower_up` at l.124) and no configuration key forces a renegotiation after Opened (grep for renegot in `accel-pppd` finds only SSTP TLS). `accel-pppd/ctrl/l2tp/l2tp.c` ICCN handling (l.3580-3595) skips `Init_Recv_LCP`, `Last_Sent_LCP`, `Last_Recv_LCP` and the `Proxy_Authen_*` AVPs, so accel-ppp as an LNS always runs its own LCP and authentication: the wholesale renegotiation shape

**Behavior to preserve:**
- Initial negotiation order, timers and every existing RFC-tagged test in the package (`rfc1661_*`, `rfc1994_*`, `rfc1332_*`, `rfc2516_*`)
- One channel reader and one unit reader per session; unit released only by `Cleanup`
- Terminate-Request in any phase ends the session with a Terminate-Ack
- IPCP leaving Opened without an LCP renegotiation still ends the session (unchanged by this spec)

**Behavior to change:**
- A peer LCP Configure-Request received in Authentication, IPCP negotiation or the network phase restarts Link Establishment per the Opened column, then authentication and IPCP run again on the same session and unit

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An LCP Configure-Request frame on the PPPoE session's channel descriptor, read by the channel reader started in `Dial`

### Transformation Path
1. The phase loop that receives it (`runClientAuth`, `negotiateIPCP`, `keepaliveLoop`) recognises LCP Configure-Request and hands it to one renegotiation routine instead of dropping it
2. tld: in the network phase, the unit's IPv4 NP mode is set to drop (new PPPIOCSNPMODE wrapper) so IP is discarded while IPCP is not Opened (RFC 1661 Section 3.6); echo ticking stops; network-phase CHAP state is discarded
3. scr plus sca or scn: the LCP negotiator runs with the received request as its first event (it sends its own Configure-Request, then answers the pending one through `ppp.NegotiatePeerOptions`), reaching Opened with a fresh local Magic-Number and possibly a new peer MRU and auth protocol
4. Authentication runs again with the newly negotiated protocol, reading the channel
5. IPCP runs again. Before PPPIOCCONNECT, IPCP arrives on the channel; after it, on the unit. The IPCP negotiator takes a control source (channel) and an NCP source (channel before connect, unit after) so one routine serves both cases
6. On IPCP Opened: NP mode back to pass; the session result (magic, peer MRU, CHAP state, local and peer address, negotiated MTU) is replaced; if address, peer address or MTU changed, a change record goes to the iface caller
7. `PPPoEClient.runSession` receives the change and replaces the P2P address, default route and MTU on the same `pppN` device

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Kernel PPP channel and unit | PPPIOCSNPMODE (new), existing reads and writes on both descriptors | No |
| pppoeclient to iface | `iface.PPPoESession` gains a change channel read by `runSession` | No |

### Integration Points
- `iface.PPPoESession` (`internal/component/iface/pppoe_client.go`): new field for in-session changes
- Backend calls already used by `runSession` (`SetMTU`, `AddAddressP2P`, `AddRoute`) plus address and route removal for the replaced values
- Child 4 (`spec-home-cpe-4-ike-dynamic-wan.md`) consumes an address change of the WAN; it reads the kernel address events, which this path produces

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | renegotiation reuses `negotiateLCP`, `runClientAuth`, `negotiateIPCP` |
| No unintended coupling (components stay isolated) | Yes | iface sees only the existing session value plus a change channel |
| No duplicated functionality (extends existing, does not recreate) | Yes | peer-option judgement stays in `ppp.NegotiatePeerOptions` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | frame buffers reused as today |
| Registration over hardcoding, outbound | N-A | no new command, family or handler |
| Registration over hardcoding, inbound | N-A | no list learns a new name; searched: none needed |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | After PPPIOCCONNECT, LCP and CHAP frames still arrive on the channel descriptor and IPCP on the unit | `docs/architecture/l2tp/cpe-1-pppoe-client.md`, citing `ppp_input` and `ppp_receive_nonmp_frame` | the renegotiation reads the wrong source | `TestDialRenegotiatesLCPInNetworkPhase` (kernel integration) | unvalidated |
| A-2 | PPPIOCSNPMODE on the unit descriptor with NPMODE_DROP discards IPv4 in both directions until set back to NPMODE_PASS | Linux `ppp_generic.c` NP mode handling (to be read in Phase 1) | IP leaks during renegotiation | integration test sending traffic during renegotiation | unvalidated |
| A-3 | The PPPoE session (discovery) and the kernel unit can stay as they are across a renegotiation | RFC 1661 Section 4.1 Opened column keeps the lower layer Up | reconnect needed | integration test asserts same session id and ifindex | unvalidated |
| A-4 | accel-ppp has no knob that renegotiates LCP after Opened on PPPoE | grep of local accel-ppp source, see Current Behavior | a cheaper interop scenario exists | re-check at implementation against the image's accel-ppp version | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A peer renegotiating in a loop keeps the link down without ending the session | repeated "LCP renegotiation" log lines, no IPCP Opened | each phase keeps its timeout; a failed renegotiation ends the session and the dialer reconnects as today |
| R-2 | An IPCP frame queued on the unit from before the renegotiation is read as a reply to the new request | IPCP Opened with stale values | Identifier check already in `negotiateIPCP`; unit frames received before IPCP restarts are discarded (RFC 1661 Section 3.4: non-LCP packets during Link Establishment are discarded) |
| R-3 | Address change applied non-atomically leaves the device without a route briefly | traffic loss after renegotiation | add new address and route before removing old ones |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | the CPE WAN session flaps or never comes up; LAN loses Internet |
| How is it reverted? | single commit revert; no config change |
| Who else touches this path? | `spec-ppp-l2tp-aaa-one-off-defects.md` (client `negotiateLCP` Nak rows), child 13 (IPCP DNS, same `negotiateIPCP`), child 17 (echo timing, same `keepaliveLoop`), child 8 (IPv6CP) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| LCP Configure-Request during CHAP authentication | → | `runClientAuth` hands off to renegotiation | `TestRFC1661ClientRenegotiatesLCPDuringAuth` |
| LCP Configure-Request during IPCP negotiation | → | `negotiateIPCP` hands off to renegotiation | `TestRFC1661ClientRenegotiatesLCPDuringIPCP` |
| LCP Configure-Request in the network phase | → | `keepaliveLoop` renegotiation, NP mode, change record | `TestRFC1661ClientRenegotiatesLCPInNetworkPhase` |
| `interface pppoe-client` config, real kernel PPP unit | → | `Dial` then renegotiation then `runSession` applies change | `TestDialRenegotiatesLCPInNetworkPhase` (Linux integration, scripted AC on a veth pair in a user namespace) |
| Ze CPE behind a LAC, LNS renegotiates | → | full path | interop scenario `access-wholesale-accel-lns` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | LCP Opened, CHAP Challenge answered, then a peer Configure-Request with a new Magic-Number and acceptable options | client sends its own Configure-Request and a Configure-Ack for the peer's (hex asserted), reaches LCP Opened, answers the next CHAP Challenge, runs IPCP to Opened |
| AC-2 | Same as AC-1 but the peer request carries a Magic-Number of zero | client sends its own Configure-Request and a Configure-Nak (RCR-), stays in Req-Sent until a valid request arrives |
| AC-3 | Configure-Request received while IPCP is negotiating (before PPPIOCCONNECT) | LCP renegotiated, authentication repeated if negotiated, IPCP restarted with a new Identifier |
| AC-4 | Configure-Request received in the network phase | NP mode set to drop before any reply is sent; LCP, authentication, IPCP rerun; IPCP frames read from the unit; NP mode pass on IPCP Opened; same PPPoE session id and same `pppN` ifindex |
| AC-5 | Renegotiation in the network phase yields a different local address | iface replaces the P2P address and the default route on the same device; old address gone, new address present |
| AC-6 | Renegotiation in the network phase negotiates a different auth protocol (CHAP to PAP or none) | the network-phase CHAP state follows the new protocol; a CHAP frame after a PAP renegotiation is dropped |
| AC-7 | Echo-Requests after renegotiation | carry the new local Magic-Number |
| AC-8 | Renegotiation fails (auth Failure, timeout) | session ends, `Done` closes, dialer reconnects with its backoff, unit released by `Cleanup` |
| AC-9 | IP packet received on the unit while renegotiating | not delivered (RFC 1661 Section 3.6) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `pppoe-client` on a wholesale FTTP line whose LNS renegotiates LCP | LAC LCP+CHAP, LNS ConfReq in Opened, renegotiation, CHAP again, IPCP, default route | `access-wholesale-accel-lns` |
| 2 | Keeps browsing while the access network renegotiates the session mid-way | network-phase renegotiation keeps device, route and NAT | `TestDialRenegotiatesLCPInNetworkPhase` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC1661ClientRenegotiatesLCPDuringAuth` | `internal/component/l2tp/pppoeclient/rfc1661_renegotiation_test.go` | AC-1, Opened RCR+ during auth | |
| `TestRFC1661ClientNaksRenegotiationZeroMagic` | same | AC-2, Opened RCR- | |
| `TestRFC1661ClientRenegotiatesLCPDuringIPCP` | same | AC-3 | |
| `TestRFC1661ClientRenegotiatesLCPInNetworkPhase` | same | AC-4, AC-7, change record | |
| `TestRFC1661ClientRenegotiationChangesAuthProto` | same | AC-6 | |
| `TestRFC1661ClientRenegotiationFailureEndsSession` | same | AC-8 | |
| `TestPPPoEClientAppliesSessionChange` | `internal/component/iface/pppoe_client_test.go` | AC-5 with a fake backend: new address and route added before old removed | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none new | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | - | a `.ci` runner cannot host a PPPoE access concentrator; the Linux integration test and the interop scenario carry the end-to-end proof | |
| `TestDialRenegotiatesLCPInNetworkPhase` | `internal/component/l2tp/pppoeclient/dialer_integration_linux_test.go` | AC-4, AC-5, AC-9 on a real kernel unit | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `access-wholesale-accel-lns` | the multi-link lab of child 18 | Ze LAC (child 14) relaying to accel-ppp as LNS, which ignores proxy LCP and renegotiates | third-party renegotiation and re-challenge reach Ze and the CPE comes up | |
| `access-wholesale` | child 19's access-chain scenario set | Ze LAC and Ze LNS with child 15's renegotiate option | the umbrella's wholesale scenario | |

The umbrella's `accel-ppp-renegotiation` row cannot be built as written: no
accel-ppp setting renegotiates LCP on a PPPoE session (A-4). The accel-ppp proof
is therefore `access-wholesale-accel-lns`; the umbrella table is corrected by the
main thread.

## Files to Modify
- `internal/component/l2tp/pppoeclient/session.go` - LCP negotiator accepts a pending peer request; auth and IPCP loops hand off a Configure-Request; IPCP negotiator takes control and NCP frame sources
- `internal/component/l2tp/pppoeclient/network_phase.go` - renegotiation from `keepaliveLoop`, result replacement, change record
- `internal/component/l2tp/pppoeclient/dialer.go` - change channel on the returned session
- `internal/component/l2tp/ppp/devppp_linux.go`, `devppp_other.go` - PPPIOCSNPMODE wrapper
- `internal/component/iface/pppoe_client.go` - `PPPoESession` change field; `runSession` applies changes
- `docs/architecture/l2tp/cpe-1-pppoe-client.md` - renegotiation path replaces "has no renegotiation path"
- `rfc/short/rfc1661.md` - requirement rows for the client's Opened RCR handling with test links

## Files to Create
- `internal/component/l2tp/pppoeclient/rfc1661_renegotiation_test.go`
- the interop scenario directory named above, under child 18's layout

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | behaviour only |
| YANG validation constraints | No | |
| YANG custom validators | No | |
| CLI commands/flags | No | |
| CLI grammar (keyword before value) | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | No | |
| Pipe completeness | No | |
| Env var registration | No | |
| Doctor check for runtime dependencies | No | uses /dev/ppp already checked |
| Prometheus counters/metrics | Yes | a renegotiation counter per client, label `name`, so a flapping access network is visible |
| BGP family surface | No | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `features/` entry for the PPPoE client, renegotiation now supported |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | the PPPoE client section of the interfaces guide gains one sentence |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc1661.md`, `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/l2tp/cpe-1-pppoe-client.md` |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | Yes | subsystem telemetry doc |
| 15 | Registered plugin/event/command changed? | No | |
| 16 | Source anchors | Yes | derived by `./le spec citation anchors` at implementation |
| 17 | Existing examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the three loops recognise LCP Configure-Request and call one renegotiation entry that is a stub returning an error
   - Tests: `TestRFC1661ClientRenegotiatesLCPDuringAuth`, `TestRFC1661ClientRenegotiatesLCPDuringIPCP`, `TestRFC1661ClientRenegotiatesLCPInNetworkPhase`
   - Files: `session.go`, `network_phase.go`
   - Verify: tests reach the stub and fail on the missing Configure-Ack
2. **Phase: LCP and auth restart** -- LCP negotiator with pending request; auth rerun; AC-1, AC-2, AC-3, AC-6
3. **Phase: network-phase renegotiation** -- NP mode wrapper, IPCP over two sources, result replacement, magic; AC-4, AC-7, AC-8, AC-9
4. **Phase: iface applies changes** -- change channel, address and route replacement; AC-5
5. **Phase: kernel integration test** -- scripted AC on veth in a user namespace
6. **Phase: interop** -- `access-wholesale-accel-lns` once children 14 and 18 land; red proof by reverting the hand-off

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | the reply to the pending request is sent after the client's own Configure-Request (scr before sca/scn) |
| Data flow | renegotiation never closes the PPPoE session or the unit |
| Rule: rfc-compliance | each new transition carries its RFC 1661 quote |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Opened RCR handled in three loops | unit tests above |
| NP mode wrapper | `grep -n PPPIOCSNPMODE internal/component/l2tp/ppp/` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | a renegotiation request goes through the same option judgement as the first one; no unbounded loop (phase timeouts) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | Re-read the source in Current Behavior |
| 3 fix attempts failed | STOP. Report all 3 approaches |

## Design Insights

The client's negotiation is a straight line of phase functions rather than a
state machine object. The smallest correct change keeps that shape: the line
becomes re-enterable from the point LCP is negotiated, and each later phase
returns "renegotiate" instead of dropping the request.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| none yet | - | no row records this defect (grep of `plan/journal/` for renegotiation on the client found none). The main thread adds one, class `silent-fall-through`, and lists it here. The adjacent row `silent-fall-through.md` 2026-10-31 (unsupported protocols not Protocol-Rejected in `negotiateIPCP`, `keepaliveLoop`) belongs to `spec-ppp-l2tp-aaa-one-off-defects.md` and is not taken |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Renegotiate in place on the same PPPoE session and unit | end the session and reconnect | RFC 1661 Section 4.1 keeps the lower layer Up; a reconnect sends PADT to a server that asked only for renegotiation, and in wholesale access the LNS renegotiates on every session, so reconnecting loops forever |
| Discard IP with PPPIOCSNPMODE drop during renegotiation | leave the unit passing IP | RFC 1661 Section 3.6 MUST discard network-layer packets while the NCP is not Opened |
| Report changed address or MTU to iface through the session value | iface polls, or always tears down addresses | only the dialer knows IPCP results; the change channel keeps one writer of interface state |
| Fresh local Magic-Number on renegotiation | keep the old one | the same `generateMagic` path as the first negotiation; open question below if a peer depends on the old value |
| accel-ppp proof as LNS behind the Ze LAC | accel-ppp as PPPoE AC | no accel-ppp knob renegotiates PPPoE LCP; its LNS skips proxy LCP AVPs, which forces renegotiation by construction |

## Known Limitations
- The third-party proof waits for children 14 and 18; until then the kernel integration test is the end-to-end evidence.
- Open question: pppd and FRR are silent here; whether any access server expects the client to keep its previous Magic-Number across renegotiation is not known. No local pppd source exists to check.
- IPCP leaving Opened on its own (no LCP renegotiation) still ends the session, as `answerOpenedIPCP` documents; changing that is not required by this goal.

## RFC Documentation (Scope: protocol)
- Each new transition quotes RFC 1661 Section 4.1 or Section 3.4 above the statement that performs it; the NP mode drop quotes Section 3.6.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] Targeted tests pass; `./le verify changed` over the closing commits
- [ ] Every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it

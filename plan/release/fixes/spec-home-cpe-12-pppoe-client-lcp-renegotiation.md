# Spec: home-cpe-12-pppoe-client-lcp-renegotiation

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P1 |
| Release reason | Shipped PPPoE client misbehaves on an accepted configuration: an access network that renegotiates LCP after the LAC opened it (the wholesale L2TP shape) never gets its Configure-Request answered, so the session times out and the CPE never comes up. Blocks the home-CPE migration. |
| RFC gaps | RFC1661-4.3-1, RFC1661-3.6-2 |
| Depends | Implementation: none. Closure: the scenario this spec owns, `access-wholesale-accel-lns`, needs `spec-home-cpe-14-lac-realm-relay.md` and `spec-home-cpe-18-interoplab-multi-link-topology.md` implemented. `access-wholesale` is owned by `spec-home-cpe-15-lns-lcp-renegotiation.md` and does not hold this spec's closure (umbrella "Scenario ownership") |
| Phase | 5/6 |
| Handoff | - |
| Updated | 2026-11-03 |

Parent: plan/release/fixes/spec-home-cpe-0-umbrella.md

Review instruction received from the parent (2026-11-02): "User now requests
external model review before finishing: main will pause at final review gate and
supply brief; don't declare finished/commit closure."

The main session holds the final review gate for that external review. This
instruction does not authorize a closure commit before the review.

One-off owner authorization (2026-11-02): "start child 12, ignore spec limit
(number open)". This claim uses `ZE_SPEC_WIP_CAP=15`; the standing WIP rule
does not change.

RFC gaps, in words: RFC 1661 Section 3.4 and Section 4.1 (RCR in Opened, and
RCR during the Authentication and Network-Layer Protocol phases; the summary
files the "MUST be prepared to immediately renegotiate" sentence as
`RFC1661-4.3-1`), and RFC 1661 Section 3.6 (discard network-layer packets while
IPCP is not Opened, `RFC1661-3.6-2`) on the renegotiation path.

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
- [ ] `docs/architecture/l2tp/cpe-1-pppoe-client.md` - the client's phases, ordered receive worker and unit ownership
  → Decision: after `PPPIOCCONNECT`, Linux queues IPCP on the unit and LCP and CHAP on the channel. One worker reads channel-first and dispatches before its next read
  → Constraint: independent IPCP reentry still ends the session. LCP reentry restarts the phase owner
  → Constraint: session loss stops transport, not unit ownership; `Cleanup` releases the unit. A renegotiation keeps both
- [ ] `docs/architecture/l2tp/` LNS pages are not the subject: the LNS FSM in `internal/component/l2tp/ppp/` is a different code path (server side) and `plan/release/fixes/spec-lcp-restart-counters.md` owns its `tld` behaviour
  → Constraint: do not duplicate that spec; reuse `ppp.NegotiatePeerOptions` and the LCP state constants the client already uses

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc1661.md` (exists) - phase table, state table rows 9 Opened
  → Constraint: summary line "The receipt of a Configure-Request while in Network-Layer Protocol or Authentication phase returns the FSM to Link Establishment (Section 3.4)" is a derived claim; the quotes below are from `rfc/full/rfc1661.txt`
- [ ] `rfc/full/rfc1661.txt` Section 3.4: "The receipt of the LCP Configure-Request causes a return to the Link Establishment phase from the Network-Layer Protocol phase or Authentication phase."
- [ ] `rfc/full/rfc1661.txt` Section 4.3 (Events), Receive-Configure-Request implementation note (the state table it drives is Section 4.1): "These events may occur on a connection which is already in the Opened state.  The implementation MUST be prepared to immediately renegotiate the Configuration Options."
- [ ] `rfc/full/rfc1661.txt` Section 4.1 state table, Opened column: RCR+ "tld,scr,sca/8", RCR- "tld,scr,scn/6"
- [ ] `rfc/full/rfc1661.txt` Section 4.4, This-Layer-Down: "Typically, this action is used by the LCP to signal the Down event to a NCP, Authentication Protocol, or Link Quality Protocol"
- [ ] `rfc/full/rfc1661.txt` Section 3.6: "Any supported network-layer protocol packets received when the corresponding NCP is not in the Opened state MUST be silently discarded."
- [ ] `rfc/short/rfc1994.md` (CHAP) - the re-challenge after re-authentication uses the same Response builder

## Current Behavior (MANDATORY)

The source inventory below records the pre-implementation audit. It is history,
not the current implementation contract. Current receive ownership is one
`frameReader` in `receive.go`, backed by `sessionLink.readPacket` in
`receive_linux.go`. The correction ledger below records the implemented fixes
and the proof still owed.

**Source files read:**
- [ ] `internal/component/l2tp/pppoeclient/session.go` - `negotiateSession` runs `negotiateLCP`, then `runClientAuth` when an auth protocol was negotiated, then `negotiateIPCP`, then `ppp.Connect` (PPPIOCCONNECT) and `ppp.SetMRU`, then starts the unit reader. `runClientAuth` handles only LCP Echo-Request and Terminate-Request among LCP codes; every other LCP packet, Configure-Request included, hits `continue`. `negotiateIPCP` does the same on its LCP branch (Echo-Request, Terminate-Request, else `continue`)
- [ ] `internal/component/l2tp/pppoeclient/network_phase.go` - `keepaliveLoop` reads the channel (LCP, CHAP) and the unit (IPCP). Its LCP switch handles Echo-Reply, Echo-Request and Terminate-Request; a Configure-Request falls out of the switch silently. `answerOpenedIPCP` ends the session on any IPCP packet that takes IPCP out of Opened
- [ ] `internal/component/l2tp/pppoeclient/dialer.go` - `Dial` creates the channel and unit with `ppp.DevPPPSetup` before LCP, starts one channel reader, runs `negotiateSession`, then `superviseNetworkPhase` runs `keepaliveLoop` in a goroutine. `PPPoESession.Done` closes on loss; the iface caller then reconnects
- [ ] `internal/component/iface/pppoe_client.go` - `runSession` applies MTU, admin-up, P2P address and default route once, after `Dial` returns; it never hears of a later change. A `SetMTU`, `SetAdminUp` or `AddAddressP2P` error returns from `runSession`, so the deferred `Cleanup` releases the unit and `run` reconnects with its backoff; an `AddRoute` error is logged as a warning and the session stays up
- [ ] `internal/component/l2tp/ppp/devppp_linux.go` - `DevPPPSetup` (PPPIOCGCHAN, PPPIOCATTCHAN, PPPIOCNEWUNIT); no PPPIOCSNPMODE support exists anywhere under `internal/component/l2tp/` (grep for npmode is empty)
- [ ] accel-ppp source (`github.com/accel-ppp/accel-ppp`, local checkout): `accel-pppd/ppp/ppp_lcp.c` starts LCP once (`ppp_fsm_lower_up` in `lcp_layer_start`) and no configuration key forces a renegotiation after Opened (grep for renegot in `accel-pppd` finds only SSTP TLS). `accel-pppd/ctrl/l2tp/l2tp.c` ICCN handling (the ICCN attribute loop of `l2tp_recv_ICCN`) skips `Init_Recv_LCP`, `Last_Sent_LCP`, `Last_Recv_LCP` and the `Proxy_Authen_*` AVPs, so accel-ppp as an LNS always runs its own LCP and authentication: the wholesale renegotiation shape

**Behavior to preserve:**
- Initial negotiation order, timers and every existing RFC-tagged test in the package (`rfc1661_*`, `rfc1994_*`, `rfc1332_*`, `rfc2516_*`)
- Exclusive reads of each descriptor, now through one ordered worker per session. Only `Cleanup` releases the unit
- Terminate-Request in any phase ends the session with a Terminate-Ack
- IPCP leaving Opened without an LCP renegotiation still ends the session (unchanged by this spec)

**Behavior to change:**
- A peer LCP Configure-Request received in Authentication, IPCP negotiation or the network phase restarts Link Establishment per the Opened column, then authentication and IPCP run again on the same session and unit

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Client phase functions | `internal/component/l2tp/pppoeclient/session.go` `negotiateLCP`, `runClientAuth`, `negotiateIPCP` | run LCP, authentication and IPCP once, in a straight line, at dial time | extends: the line becomes re-enterable from LCP, and each later phase hands a Configure-Request to it instead of dropping it |
| Network-phase reader | `internal/component/l2tp/pppoeclient/network_phase.go` `keepaliveLoop`, `answerOpenedIPCP` | answers echo and Terminate on the channel, ends the session when IPCP leaves Opened | extends: an LCP Configure-Request starts the renegotiation; IPCP leaving Opened alone keeps ending the session |
| Peer-option judgement | `internal/component/l2tp/ppp/lcp_options.go` `NegotiatePeerOptions` | decides Ack, Nak and Reject for a peer's LCP options under an `LCPNegPolicy` | extends: the renegotiation answers the pending request through it, no second judgement |
| Local Magic-Number | `internal/component/l2tp/pppoeclient/session.go` `generateMagic` | picks the client's Magic-Number for the first negotiation | extends: called again for the renegotiation |
| Shared /dev/ppp setup | `internal/component/l2tp/ppp/devppp_linux.go` `DevPPPSetup`; `docs/architecture/l2tp/bng-5-pppoe.md` "Patterns worth reusing" | creates channel and unit descriptors for L2TP and PPPoE | extends: a PPPIOCSNPMODE wrapper is added beside it in the same shared package |
| Session supervision and iface apply | `internal/component/l2tp/pppoeclient/dialer.go` `superviseNetworkPhase`; `internal/component/iface/pppoe_client.go` `runSession` | runs the network phase; iface applies address, route and MTU once after `Dial` | extends: a change record on the session value lets `runSession` replace them in place |
| LNS-side PPP FSM | `internal/component/l2tp/ppp/session_run.go` (server side), owned by `plan/release/fixes/spec-lcp-restart-counters.md` | the LNS and AC LCP state machine | does not apply: a different code path; this spec changes only the client |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An LCP Configure-Request frame on the PPPoE session's channel descriptor, read by the channel reader started in `Dial`

### Transformation Path
1. The phase loop that receives it (`runClientAuth`, `negotiateIPCP`, `keepaliveLoop`) recognises LCP Configure-Request and hands it to one renegotiation routine instead of dropping it
2. tld: in the network phase, the unit's IPv4 NP mode is set to drop (new PPPIOCSNPMODE wrapper) so IP is discarded while IPCP is not Opened (RFC 1661 Section 3.6); echo ticking stops; network-phase CHAP state is discarded
3. scr plus sca or scn: the LCP negotiator runs with the received request as its first event (it sends its own Configure-Request, then answers the pending one through `ppp.NegotiatePeerOptions`), reaching Opened with a fresh local Magic-Number and possibly a new peer MRU and auth protocol
4. Authentication runs again with the newly negotiated protocol, reading the channel
5. IPCP runs again through the same ordered reader. Before PPPIOCCONNECT, only the channel is read. After connection, each read tries the channel before the unit. The phase consumer dispatches each delivery before authorizing another read. LCP and authentication discard premature IPCP without a userspace read-ahead queue
6. On IPCP Opened: NP mode back to pass; the session result (magic, peer MRU, CHAP state, local and peer address, negotiated MTU, and every value `negotiateIPCP` learns, which child 13 extends with the two resolver addresses) is replaced. Every completed renegotiation sends a change record carrying the whole new result, whether or not a value differs: the dialer never filters, so a value added to the IPCP result later (child 13's DNS) needs no change to the notification rule
7. `PPPoEClient.runSession` receives the record, compares each field with what it last applied, and changes only the fields that differ, on the same `pppN` device: MTU alone (`SetMTU`); the P2P address when the local or the peer address differs (new address added before the old one is removed); the default route when the peer address differs (new route added before the old one is removed); and, once child 13 lands, the resolver lease when the servers differ
8. Lifecycle of the change record (see the table below): the channel holds at most one pending record and the dialer replaces a pending record with a newer one, so a renegotiation that overtakes an unapplied one is applied once, from its own complete result. An apply error on MTU or address ends the session exactly as the same error does at the first apply (return from `runSession`, `Cleanup`, reconnect with backoff); a route error is logged and the session stays up, as at the first apply

#### Change record lifecycle

| Event | Dialer | `runSession` | Network traffic |
|-------|--------|--------------|-----------------|
| Configure-Request in the network phase | NP mode drop, renegotiation starts | nothing pending | IP discarded (RFC 1661 Section 3.6) |
| IPCP Opened again | NP mode pass, record sent (replacing any pending one) | applies the differing fields | IP flows; until `runSession` applies a changed address, packets leave with the old source and the peer may drop them |
| Second Configure-Request before IPCP Opened | renegotiation restarts from LCP; no record sent for the interrupted one | nothing pending | IP still discarded |
| Second Configure-Request after the record was sent, before `runSession` read it | the next record replaces the pending one | applies only the newest | as above |
| MTU or address apply fails | - | returns the error; `Cleanup` releases the unit; reconnect with backoff | session ends |
| Route apply fails | - | warning logged, session stays up | as today at the first apply |
| Renegotiation fails (timeout, auth Failure) | session ends, `Done` closes | returns on `Done` | session ends (AC-8) |

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
| A-1 | After PPPIOCCONNECT, LCP and CHAP frames still arrive on the channel descriptor and IPCP on the unit | Linux v6.18 `drivers/net/ppp/ppp_generic.c`, `ppp_input` routes protocols >= 0xc000 to the channel; `ppp_receive_nonmp_frame` queues IPCP on the unit | the renegotiation reads the wrong source | Phase 1 source read. `TestPPPoEConfigRenegotiatesKernelState`, `TestDialAcknowledgesFirstEagerIPCP` and `TestClientAnswersIPCPOnTheUnitDescriptor` passed with race instrumentation on 7.2.9-ze | confirmed (source and scoped 7.2.9-ze runtime proof). Target 7.3 proof still owed |
| A-2 | PPPIOCSNPMODE on the unit descriptor with NPMODE_DROP discards IPv4 in both directions until set back to NPMODE_PASS | Linux v6.18 `ppp_ioctl`, `ppp_start_xmit`, `ppp_receive_nonmp_frame` write and enforce the per-protocol NP mode | IP leaks during renegotiation | Phase 1 source read. `TestDialRenegotiatesLCPInNetworkPhase` passed with race instrumentation on 7.2.9-ze: bidirectional IPv4 PASS before/after and DROP during LCP, PAP and IPCP | confirmed (source and scoped 7.2.9-ze traffic proof). Target 7.3 proof still owed |
| A-3 | The PPPoE session (discovery) and the kernel unit can stay as they are across a renegotiation | RFC 1661 Section 4.1 Opened RCR has tld, scr, sca/scn, no lower-layer teardown; `Dial` reserves the unit until Cleanup | reconnect needed | Phase 1 RFC and `Dial` read. `TestDialRenegotiatesLCPInNetworkPhase` passed with race instrumentation on 7.2.9-ze, asserting retained session ID and ifindex. `TestPPPoEConfigRenegotiatesKernelState` passed on the same kernel | confirmed (contract and scoped 7.2.9-ze identity proof). Target 7.3 proof still owed |
| A-4 | accel-ppp has no knob that renegotiates LCP after Opened on PPPoE | accel-ppp tag 1.14.0 `ppp_lcp.c` `load_config`, `lcp_layer_start`, `lcp_recv`; `test/interop-pppoe/Dockerfile.accel` declares 1.14.0 | a cheaper interop scenario exists | Phase 1 read of the declared version; actual topology image version must be checked in Phase 6 | confirmed (declared version) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A peer renegotiating in a loop keeps the link down without ending the session | repeated "LCP renegotiation" log lines, no IPCP Opened | Bound one negotiation attempt to ten LCP epochs, using the existing 30-second phase deadlines. A completed IPCP exchange ends the attempt, so healthy separated restarts receive a fresh budget. Phase deadlines alone did not bound repeated reentry. |
| R-2 | An IPCP frame queued on the unit from before the renegotiation is read as a reply to the new request | IPCP Opened with stale values | Identifier check already in `negotiateIPCP`; unit frames received before IPCP restarts are discarded (RFC 1661 Section 3.4: non-LCP packets during Link Establishment are discarded) |
| R-3 | Address change applied non-atomically leaves the device without a route briefly | traffic loss after renegotiation | add new address and route before removing old ones |
| R-4 | Forwarding resumes (NP mode pass) before `runSession` applies a changed address | a few packets leave with the old source address and are dropped by the peer | bounded by one channel read; the alternative (hold NP mode drop until iface acknowledges) adds a reply path from iface to the dialer for no correctness gain, since the peer drops those packets either way |
| R-5 | An apply error during a renegotiation leaves the device half-updated | old and new address both present | the error ends the session and `Cleanup` releases the unit, which removes the device with every address on it; the reconnect applies afresh |
| R-6 | Independent descriptor readers discarded eager IPCP while authentication Success waited for dispatch | Original deterministic and real-PPP probes missed the first Ack | Corrected by `receive.go` `frameReader.run` and `receive_linux.go` `sessionLink.readPacket`: one authorized read, channel-first, then dispatch. `TestClientEagerIPCPReaders`, `TestOrderedDescriptorDispatch` and `TestDialAcknowledgesFirstEagerIPCP` passed. Real-PPP evidence is on 7.2.9-ze, not target kernel 7.3. Premature discard remains covered separately. |

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
| AC-10 | Renegotiation changes only the peer address | iface replaces the P2P address's peer and the default route's gateway; the local address and the MTU are not touched (no `SetMTU` call) |
| AC-11 | Renegotiation changes only the negotiated MTU | iface calls `SetMTU` once with the new value, capped by the configured `mtu`; no address or route is removed or added |
| AC-12 | Renegotiation changes nothing | a record arrives and `runSession` makes no backend call when all fields are already applied. An unchanged desired result can retry a previously failed route install or removal |
| AC-13 | A second Configure-Request arrives after the first renegotiation's record was sent and before `runSession` read it | `runSession` applies only the second result, once |
| AC-14 | `SetMTU` or `AddAddressP2P` fails while applying a record | the session ends, `Cleanup` releases the unit, and `run` reconnects with its backoff (same as a first-apply failure) |
| AC-15 | `AddRoute` fails while applying a record | a warning names the route and the device; the session stays up |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `pppoe-client` on a wholesale FTTP line whose LNS renegotiates LCP | LAC LCP+CHAP, LNS ConfReq in Opened, renegotiation, CHAP again, IPCP, default route | `access-wholesale-accel-lns` |
| 2 | Keeps browsing while the access network renegotiates the session mid-way | network-phase renegotiation keeps device, route and NAT | `TestDialRenegotiatesLCPInNetworkPhase` |

## 🧪 TDD Test Plan

### Unit Tests
Paths without a directory in this table are under
`internal/component/l2tp/pppoeclient/`. These are actual symbols checked against
their bodies on 2026-11-03. Prior scoped race results remain the execution
evidence. This mapping does not claim RFC discrimination or target-kernel proof.

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC1661ClientRenegotiatesLCPDuringAuth` | `rfc1661_renegotiation_test.go` | AC-1: exact LCP Ack/order, two independently checked CHAP Responses and completed IPCP before the deliberately invalid Connect | Prior race PASS. No kernel connection claimed |
| `TestRFC1661ClientNaksRenegotiationZeroMagic`, `TestClientRenegotiationNakNeedsPeerAcceptance` | `rfc1661_renegotiation_test.go` | AC-2: own request precedes Nak, nonzero suggestion, and no auth/IPCP without a corrected peer offer | Prior race PASS |
| `TestRFC1661ClientRenegotiatesLCPDuringIPCP` | `rfc1661_renegotiation_test.go` | AC-3: reentry before Connect, repeated auth, advancing IPCP Identifier and stale-reply refusal | Prior race PASS |
| `TestRFC1661ClientRenegotiatesLCPInNetworkPhase` | `rfc1661_renegotiation_test.go` | AC-4 dispatch only: exact scr/sca output before EOF | Prior race PASS. Does not prove completed restart, Magic or notification |
| `TestConnectedExchangeRestarts`, `TestRestartEchoSequence`, `TestConnectedChangesEveryCompletion` | `connected_exchange_test.go`, `restart_admission_test.go`, `connected_changes_test.go` | AC-4 mode ordering and completed exchanges, AC-6 CHAP/PAP/none transitions, AC-7 new Magic in Echo, AC-12 every completion notifies | Prior race PASS. Kernel identity needs integration |
| `TestRFC1661ClientRenegotiationChangesAuthProto` | `rfc1661_renegotiation_test.go` | AC-6: initial CHAP reentry into PAP/none and CHAP discard during IPCP | Prior race PASS |
| `TestConnectedRestartFailures` | `connected_failure_test.go` | AC-8: auth failures and each phase timeout end the actual supervisor, close transport and publish Done before cleanup | Prior race PASS. No real unit or failure-to-reconnect end-to-end proof in this test |
| `TestPPPoEClientSessionChanges/local`, `TestPPPoEClientSessionChanges/both-addresses` | `internal/component/iface/pppoe_changes_test.go` | AC-5: exact make-before-break pair calls. A local-only change retains the unchanged gateway, rather than rewriting its route | Prior scoped race PASS. Literal AC-5 route replacement for local-only change is not asserted |
| `TestPPPoEClientSessionChanges/peer` | `internal/component/iface/pppoe_changes_test.go` | AC-10: same local value, new P2P peer/gateway, no MTU call | Prior scoped race PASS |
| `TestPPPoEClientSessionChanges/mtu`, `TestPPPoEClientChangedMTUHitsConfiguredCap` | `internal/component/iface/pppoe_changes_test.go`, `internal/component/iface/pppoe_route_progress_test.go` | AC-11: one MTU write, no address/route calls, changed value reaches configured cap | Prior scoped proof retained |
| `TestPPPoEClientSessionChanges/unchanged`, `TestPPPoEClientChangeRouteRetry`, `TestPPPoEClientInitialRouteRetry`, `TestPPPoEClientOldRouteRemovalRetry` | `internal/component/iface/pppoe_changes_test.go`, `internal/component/iface/pppoe_change_lifecycle_test.go` | AC-12: zero calls after successful application. Failed route installs/removals can retry | Prior scoped race PASS |
| `TestConnectedChangesReachConsumerNewest` | `changes_consumer_test.go` | AC-13: two real restarts while iface is held, then only initial and newest results applied | Prior race PASS. Unlike `TestPPPoEClientNewestPendingChange`, this test uses the production slot replacement |
| `TestPPPoEClientChangeApplyFailureCleansUp`, `TestPPPoEClientChangeReconnect` | `internal/component/iface/pppoe_changes_test.go`, `internal/component/iface/pppoe_change_lifecycle_test.go` | AC-14: fatal MTU/address errors invoke Cleanup, then Start's run loop waits the reconnect delay before Dial | Prior scoped race PASS. Unit release is modeled by Cleanup |
| `TestPPPoEClientChangeRouteRetry` | `internal/component/iface/pppoe_change_lifecycle_test.go` | AC-15: warning contains route/gateway/device, session stays up, no Cleanup, retry succeeds | Prior scoped race PASS |
| `TestConnectedRestartReentry`, `TestConnectedChangesInterruptedExchange` | `connected_failure_test.go`, `connected_changes_test.go` | Interrupted auth/IPCP restarts preserve DROP and publish only the final completion | Prior race PASS |
| `TestInitialMTUAdmission`, `TestRestartMTUAdmission`, `TestRestartConvergenceBound` | `restart_admission_test.go` | Direct final MTU admission with valid config, separate wire failure ordering, positive 68 exchange, and ten incomplete epochs bound reentry | Approved v2 applied; both admission tests and their 66/68 cases pass under race instrumentation. Initial 68 reaches the invalid-descriptor Connect boundary. EOF is not constructor-wiring proof. Convergence-bound evidence is the prior race PASS, not rerun |
| `TestClientMRUConvergence`, `TestSessionTransportRequired` | `mru_convergence_test.go` | Initial and restart Nak 66 to 68, exact corrected Ack, completed IPCP, restart PASS/publication, and refusal of transport without lifecycle ownership | Convergence retains prior race PASS. Approved v2 transport test passes under race instrumentation, requiring refusal before any write or read authorization. No kernel or third-party proof claimed |
| `TestClientEagerIPCPReaders`, `TestReaderTransitionHeldFrame`, `TestConnectedAuthQueueDiscard` | `eager_ipcp_test.go`, `reader_admission_test.go`, `connected_transition_test.go` | First eager request survives paused dispatch. Genuinely premature IPCP is consumed before Success | Prior focused race PASS |
| `TestNetworkReceiveFailureDiagnostic`, `TestNetworkReceiveWarningAfterStop` | `network_receive_error_test.go` | Active receive errors preserve cause/client at WARN. Published shutdown suppresses warnings | Prior targeted race PASS |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Completed negotiated MTU | 68..configured MTU, capped by peer MRU | 68 in `TestClientMRUConvergence` initial and restart | Peer MRU 66 receives Nak 68. Valid-config raw 66 refusal has mutation red/green proof; approved v2 initial/restart admission tests pass | Constructor bounds covered by `TestConnectedSessionAdmission`; not a wire-level upper-bound claim |
| Incomplete LCP epochs per attempt | 1..10 | Ten exchanges in `TestRestartConvergenceBound` | Not applicable | Eleventh reentry ends the supervisor without PASS or publication |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| No `.ci` carrier added | - | Registered startup delivery is exercised in isolated integration processes, not through the full daemon CLI | Full CLI workflow not demonstrated here |
| `TestDialRenegotiatesLCPInNetworkPhase` | `internal/component/iface/pppoe_packets_integration_linux_test.go` | AC-4/5/9: operator config through registered startup, retained SID/ifindex, changed state, bidirectional IPv4 PASS before/after and DROP during LCP/PAP/IPCP | Race PASS on 7.2.9-ze. Target 7.3 remains owed |
| `TestPPPoEConfigRenegotiatesKernelState` | `internal/component/iface/pppoe_kernel_integration_linux_test.go` | AC-4/5: two CHAP epochs, exact local/peer pair, gateway, metric, protocol and MTU on one device | Race PASS on 7.2.9-ze. Startup omission/duplication mutations were red |
| `TestDialAcknowledgesFirstEagerIPCP`, `TestOrderedDescriptorDispatch` | `internal/component/l2tp/pppoeclient/eager_ipcp_integration_linux_test.go`, `internal/component/l2tp/pppoeclient/receive_order_integration_linux_test.go` | Issue 6: first eager peer request acknowledged without retransmission, plus raw descriptor ordering | Prior race PASS on 7.2.9-ze, not target 7.3 |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `access-wholesale-accel-lns` | `test/interop-topology/scenarios/access-wholesale-accel-lns/` (child 18's topology suite; this spec owns the directory, its checker and its red walk) | Ze LAC (child 14) relaying to accel-ppp as LNS, which ignores proxy LCP and renegotiates | third-party renegotiation and re-challenge reach Ze and the CPE comes up | Open. No passing scenario or red walk claimed |
| `access-wholesale` | `test/interop-topology/scenarios/access-wholesale/`, owned by child 15 | Ze LAC and Ze LNS with child 15's renegotiate option | the umbrella's wholesale scenario; not this spec's closure evidence | Owned by child 15 |

No accel-ppp setting renegotiates LCP on a direct PPPoE session (A-4), so the
accel-ppp proof is `access-wholesale-accel-lns`, the name the umbrella's Lab
table uses.

## Files to Modify
- `internal/component/l2tp/pppoeclient/session.go` - LCP accepts a pending peer request. Auth and IPCP hand off a Configure-Request through the shared ordered receive owner
- `internal/component/l2tp/pppoeclient/network_phase.go` - renegotiation from `keepaliveLoop`, result replacement, change record
- `internal/component/l2tp/pppoeclient/dialer.go` - change channel on the returned session
- `internal/component/l2tp/ppp/devppp_linux.go`, `devppp_other.go` - PPPIOCSNPMODE wrapper
- `internal/component/iface/pppoe_client.go` - `PPPoESession` change field; `runSession` applies changes
- `internal/component/iface/register.go` - startup reconciliation reports parsed and active counts to the read-only observation seam
- `docs/architecture/l2tp/cpe-1-pppoe-client.md` - renegotiation path replaces "has no renegotiation path"
- `docs/architecture/l2tp/bng-5-pppoe.md` (declared by `devppp_linux.go` and `devppp_other.go`) - nothing it states becomes wrong: "Shared PPP integration" still holds, since the NP mode wrapper lives in the same shared package; one sentence there names the wrapper beside `DevPPPSetup` so the page still lists the shared kernel setup in full
- `rfc/short/rfc1661.md` - requirement rows for the client's Opened RCR handling with test links

## Files to Create
- `internal/component/l2tp/pppoeclient/rfc1661_renegotiation_test.go`
- `internal/component/iface/pppoe_startup_observation.go` - production no-op observer outside Linux integration builds
- `internal/component/iface/pppoe_startup_observation_integration_linux.go` - first-delivery immutable parsed/active counts
- `internal/component/iface/pppoe_startup_observation_integration_linux_test.go` - read-only `PPPoEStartupCountsForTest` accessor
- `internal/component/iface/pppoe_config_integration_linux_test.go` - registered startup fixture with exact cardinality assertions
- `internal/component/iface/pppoe_kernel_integration_linux_test.go`, `pppoe_packets_integration_linux_test.go` - installed-state and IPv4 traffic scenarios
- `internal/component/l2tp/pppoeclient/receive.go`, `receive_linux.go`, `receive_other.go` - one request/delivery worker and platform descriptor operations
- `internal/test/pppoeac/` - the scripted access concentrator `TestDialRenegotiatesLCPInNetworkPhase` drives (PADI to PADS, LCP, CHAP, IPCP, a Configure-Request on demand, Echo control), a test helper package beside `internal/test/userns` so child 17's `TestPPPoEClientKeepaliveConfigEndToEnd` reuses the same copy
- `test/interop-topology/scenarios/access-wholesale-accel-lns/` - `ze-cpe.conf`, `ze-lac.conf`, the accel-ppp LNS configuration
- `internal/le/interoplab/topology/register_wholesale_accel.go`, `check_wholesale_accel.go` - the scenario's nodes, links and checker in child 18's topology suite

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
| 1 | New user-facing feature? | Yes | `features/interfaces.md`, the interface feature entry: the PPPoE client renegotiates LCP |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` gains a `pppoe-client` subsection, which no guide page has today (the client is named only under "Route Priority"; `docs/guide/pppoe.md` documents the access concentrator): it states that the client answers an LCP renegotiation from the access network. Children 13 and 17 extend the same subsection |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc1661.md`, `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/l2tp/cpe-1-pppoe-client.md` |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | Yes | `docs/guide/configuration.md`, the `pppoe-client` subsection this spec adds (row 6), names the renegotiation counter and its `name` label |
| 15 | Registered plugin/event/command changed? | No | |
| 16 | Source anchors | Yes | derived by `./le spec citation anchors` at implementation |
| 17 | Existing examples for this area? | No | |

## Implementation Steps

### Phase 1 audit and evidence (2026-11-02)

| Requirement | Tree before wiring | Producer or evidence |
|-------------|--------------------|----------------------|
| Three later-phase LCP entry points | Partial: each parses LCP but drops Configure-Request | `session.go` `runClientAuth`, `negotiateIPCP`; `network_phase.go` `keepaliveLoop` |
| Reuse LCP option judgement and phase functions | Present; no phase restart owner | `session.go` `negotiateSession`, `negotiateLCP` |
| Shared reentry boundary | Missing before Phase 1; now temporary error handoff | `session.go` `requestLCPReentry`, `lcpReentry` preserve the parsed request |
| NP mode wrapper and dual IPCP input | Missing | `devppp_linux.go` has `DevPPPSetup`, no PPPIOCSNPMODE; `negotiateIPCP` reads one source |
| Session changes and iface reapply | Missing | `dialer.go` `Dial` returns one result; `iface/pppoe_client.go` `runSession` applies once |
| New unit, kernel and wholesale tests | Missing before Phase 1 | Three untagged red probes now in `rfc1661_renegotiation_test.go`; scripted AC, kernel renegotiation test and owned topology scenario remain for their phases |

The three new tests ran with `go test -race` through `./le job run`, selecting
`^TestRFC1661ClientRenegotiatesLCP`. All failed before the wiring on missing
Configure-Ack `c0 21 02 73 00 0a 05 06 55 66 77 88`. After wiring, the same
tests still fail on that Ack and each reports the shared restart stub. Auth
and IPCP previously reported channel EOF; keepalive previously logged nothing.
The second run also passed `TestClientCHAPReplyCorrelation`,
`TestClientLCPReplyCorrelation`, `TestRFC1994NetworkPhaseChallengeAnswered`
and `TestRFC1994IPCPWindowChallengeAnswered` under the race detector.

Diagnosis: the later-phase LCP switches omit Configure-Request. The owning
layer is the client phase driver. A reconnect is a [workaround] that repeats
wholesale failure; [source] reentry preserves the pending request and reruns
LCP, authentication and IPCP. Phase 1 wires only that boundary. No AC is fully
satisfied yet; Phase 2 must consume the handoff. Do not commit the temporary
runtime stub alone. No existing tagged test was edited.

### Phase 3 admission audit (2026-11-02)

`negotiateSession` now consumes admitted requests during authentication and
initial IPCP. `keepaliveLoop` still logs the valid-request restart handoff and
returns. Each phase function reads only the channel; the shared PPP package
still has no NP-mode control. No Phase 3 acceptance criterion is satisfied.

The second frame-source parameter requires mechanical changes to RFC-tagged
test callers of `negotiateLCP`, `runClientAuth` and `negotiateIPCP`. An exact,
unapplied patch is prepared for independent test judgment in the session handoff.
Production implementation resumes after that judgment. Phase 2's RFC records
and audit rejudgments remain owed before landing; its correction-scope review
does not discharge them.

1. **Phase: Wiring (MANDATORY FIRST)** -- the three loops recognise LCP Configure-Request and call one renegotiation entry that is a stub returning an error
   - Tests: `TestRFC1661ClientRenegotiatesLCPDuringAuth`, `TestRFC1661ClientRenegotiatesLCPDuringIPCP`, `TestRFC1661ClientRenegotiatesLCPInNetworkPhase`
   - Files: `session.go`, `network_phase.go`
   - Verify: tests reach the stub and fail on the missing Configure-Ack
2. **Phase: LCP and auth restart** -- LCP negotiator with pending request; auth rerun; AC-1, AC-2, AC-3, AC-6
3. **Phase: network-phase renegotiation** -- NP mode wrapper, IPCP over two sources, result replacement, magic; AC-4, AC-7, AC-8, AC-9
4. **Phase: iface applies changes** -- change channel (one pending record, newest wins), per-field diff, address and route replacement, apply-failure lifecycle; AC-5, AC-10 to AC-15
5. **Phase: kernel integration test** -- scripted AC on veth in a user namespace
6. **Phase: interop** -- `access-wholesale-accel-lns` once children 14 and 18 land; red proof by reverting the hand-off

### Phase 4 producer checkpoint (2026-11-02)

`connectedSession.restart` now publishes every complete interface result after
successful PASS, including unchanged results. Its constructor owns a one-slot
channel, and the sole writer replaces a pending value. `Dial` exposes that
channel through `PPPoESession.Changes`. The network-phase restart counter is
registered through the existing metrics hook and labeled by configured name.

The producer tests observed missing notifications and a missing exported counter
before implementation. The new tests and the full pppoeclient package pass under
the race detector. The notification selection's count profile exercises both
empty and replaced queue outcomes. This does not satisfy the interface ACs yet:
`runSession` still applies only the initial result. Phase 4 needs continuation.

The consumer audit found that `Backend.RemoveAddress` cannot name the peer of a
P2P pair. `netlinkBackend.AddAddressP2P` uses AddrAdd, and RemoveAddress parses
only the local prefix. Exact P2P removal and preservation of same-peer secondary
addresses need resolution before make-before-break reapplication is implemented.
No backend contract or existing test fixture has changed at this checkpoint.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at a named function |
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

### External correction diagnosis (2026-11-03)

The table below preserves the original diagnosis. The current ledger after it
distinguishes implemented corrections from the proof still owed. The final
external review remains required.

| Issue | Symptom and producing cause | Owner | Alternatives and decision |
|---|---|---|---|
| 1 | `negotiatePhases` returns MRU 66 as a completed result, while `newConnectedSession` rejects it only on initial connection | Negotiated-result construction | [workaround] duplicate a restart MTU guard. [source] one constructor admits results for initial negotiation and restart before PASS or publication. Duplicated guards can diverge again. |
| 2 | `keepaliveLoop` resets Echo Identifier on restart | Session Identifier lifetime | [workaround] choose another starting constant. [source] retain the sequence and reset only timing and failure state. Constants can repeat the previous Identifier. |
| 3 | `negotiateIPCP` starts Code-Reject Identifier at zero each epoch and returns a snapshot | Session Identifier lifetime | [workaround] seed each epoch from a copied result. [source] retain one mutable counter through interrupted negotiation and keepalive. A copied result loses increments on interruption. |
| 4 | `negotiatePhases` has an unlimited loop with fresh deadlines on every reentry | Phase owner | [workaround] rely on phase deadlines. [source] bound epochs per attempt using the RFC 1661 Section 4.6 Max-Configure default of ten as the local convergence budget. This is a policy bound on epochs, not a claim that RFC Max-Configure counts reentries. |
| 7 | `negotiateLCP` has four terminating compound OR guards | LCP control flow | [workaround] hide conditions in booleans. [source] split the guards without changing outcomes. Hidden conditions preserve the reading burden. |
| 9 | `assertClientRenegotiationReply` accepts IPCP Code 1 before the LCP Ack | Test oracle | [workaround] trust the shared Code value. [source] require Protocol c021 as well as Code 1 and exact Ack order. IPCP is not an LCP request. |
| 10 | `TestConnectedRestartFailures` closes the link before counting closures | Supervisor test observer | [workaround] infer closure from worker completion. [source] assert transport closure and session Done before test cleanup. Worker completion does not publish session Done. |

### Findings 1–11: current correction ledger (2026-11-03)

Client filenames below are under `internal/component/l2tp/pppoeclient/`.
The TDD tables name the test files and proof limits. These statuses retain prior
scoped execution evidence, not a new test run or external-review approval.

| Issue | Current producer or correction | Evidence and remaining limit |
|-------|---------------------------------|------------------------------|
| 1: MTU admission | `phases.go` `negotiatePhases` returns through `connected_session.go` `newNegotiatedSession` before initial Connect or restart PASS | Minimum-MRU policy Naks 66 before admission. Convergence and constructor tests pass. Approved v2 replaces the obsolete Ack expectations and passes both initial/restart tests. Direct result-floor mutation proof is separate from wire EOF ordering; constructor-call removal is not proven to redden v2. RFC discrimination remains open |
| 2: Echo sequence | `network_phase.go` `keepaliveLoop` retains `echoID` across restart and resets only timing and failures | `restart_admission_test.go` `TestRestartEchoSequence` passed under race instrumentation, including new Magic |
| 3: IPCP Code-Reject sequence | `session.go` `negotiateIPCP` and `network_phase.go` `answerOpenedIPCP` advance the retained session counter | `code_reject_sequence_test.go` `TestClientCodeRejectSequence` and `TestClientCodeRejectInitialSequence` passed under race instrumentation. Interrupted epochs retain increments |
| 4: convergence bound | `phases.go` `negotiatePhases` permits ten epochs per incomplete attempt | `TestRestartConvergenceBound` passed under race instrumentation. This is an epoch policy, not the RFC retransmission counter |
| 5: desired route progress | `internal/component/iface/pppoe_client.go` `runSession` attempts the desired install before retained cleanup, tracks successful ownership and cancels cleanup when its gateway becomes desired | `pppoe_route_boundaries_test.go` `TestPPPoEClientSimultaneousRouteFailures` and `TestPPPoEClientRouteReturnsToPendingGateway`, plus approved ownership test: scoped race/count proof. Final review open |
| 6: eager IPCP and receive ordering | `receive.go` `frameReader.run` permits one read per dispatch. `receive_linux.go` `sessionLink.readPacket` tries the channel before the connected unit | Eager CHAP/PAP/no-auth, premature discard, lifecycle and raw descriptor tests passed. `TestDialAcknowledgesFirstEagerIPCP` passed on 7.2.9-ze. Target 7.3 and final review open |
| 7: compound LCP guards | `session.go` `negotiateLCP` uses separate terminating checks for the four reported compound guards | Source correction retained with existing transition/correlation assertions. Approved affected tests passed. Post-fix scoped lint remains unestablished |
| 8: unchanged-result retry exception | `runSession` compares against successful application state. AC-12 now states the failed-install/removal exception | `TestPPPoEClientSessionChanges/unchanged`, `TestPPPoEClientChangeRouteRetry`, `TestPPPoEClientInitialRouteRetry` and `TestPPPoEClientOldRouteRemovalRetry`: scoped race PASS |
| 9: exact LCP oracle | `rfc1661_renegotiation_test.go` `assertClientRenegotiationReply` requires LCP protocol c021 and scr before the exact Ack | Renegotiation tests passed under race instrumentation. The network-phase probe proves dispatch only. RFC discrimination remains open |
| 10: supervisor closure | `connected_failure_test.go` `TestConnectedRestartFailures` checks transport closure and session Done before test cleanup | Actual supervisor failure cases passed under race instrumentation. Real-unit release and integrated failure-to-reconnect proof are not supplied by this test |
| 11: changed MTU cap and newest-wins | `runSession` caps a changed MTU. `connected_session.go` `restart` replaces the production one-slot pending result | `TestPPPoEClientChangedMTUHitsConfiguredCap` and `TestConnectedChangesReachConsumerNewest` retain red/restored-green evidence. The consumer test passed again in the approved ordered-reader population |

The lint-only cleanup preserves malformed-wire pass-through, cancellation,
CHAP-MD5, test assertions and single-fact guards. Its one scoped lint attempt
exited 2 because inputs changed during planning after setup. No plan was produced,
so no post-fix lint pass is claimed. The attempt covered `pppoeac`, `iface` and
`pppoeclient`, not package B's additional netlink scope.

### Latest external corrections (2026-11-03)

| Finding | Diagnosis and correction | Evidence/status |
|---------|--------------------------|-----------------|
| Fixture nil outcomes | `ReadExpectedPacket`, `replyEcho` and `replyDiscovery` intentionally preserve raw input or suppress unsolicited replies. [workaround] changing those outcomes would change the observer. [source] name their guards with the checker-supported annotation. | Applied. Native `refusebydefault.Check` and `compoundguard.Check`, scoped to upstream-changed lines in the four edited production/fixture files, pass. All three annotations are listed as accepted. |
| Receive lock cost | `sessionLink.readPacket` retains `mu` across a 100ms poll. Independently started writes, mode changes and Close can wait for that poll. Packet-triggered replies run before the next read request. | Code-derived poll contribution, not a measured latency bound. Scheduling and lock contention can add delay. An idle outstanding read requests about ten timeout wakeups per second. |
| Transport requirement | `negotiateSession` requires a non-nil `sessionLink` before its first LCP write, then uses the checked link for lifecycle locking and unit-read activation. | Nil and typed-nil cases both reproduced a panic before the ordering fix and pass under race instrumentation afterward. The five independently approved transport-only units pass with their wrappers applied. Approved v2 unsupported-transport adaptation is applied and passes under race instrumentation. |
| MRU convergence | `negotiatePeerOption` accepts 64–67 while `newNegotiatedSession` refuses less than 68. [workaround] reconnect after Ack. [source] extend `LCPNegPolicy` with a minimum and set the client minimum to its existing IPv4 admission bound. Keep final result admission. | Applied. Initial/restart `TestClientMRUConvergence` was red on Ack 66 and passes under race instrumentation on Nak 68 and completed exchange. V2 admission fixtures were independently approved, applied and passed. |

RFC 1661 Section 5.3 states: "If every instance of the received Configuration
Options is recognizable, but some values are not acceptable, then the
implementation MUST transmit a Configure-Nak." Section 6.1 specifies no 64-octet
minimum. The shared negotiator's 64 is existing local policy, not that RFC's rule.
RFC 791 Section 3.2 states: "Every internet module must be able to forward a
datagram of 68 octets without further fragmentation." The client extends the
existing shared option judgment to its IPv4 minimum, without a second negotiator.

Scoped follow-up also passed the four existing `TestNegotiatePeerMRU*` tests,
`TestClientLCPMRUNakBounds`, both client PPPoE ceiling tests and
`TestConnectedSessionAdmission`, with race instrumentation and no skips.
Count coverage observes both minimum-MRU outcomes and the unsupported-transport
refusal. The later approved five-test run reaches the supported-link connection
error boundary; it does not prove successful kernel connection.
The documentation check exits 1 on 29 unrelated BGP/tooling anchor declarations,
with no PPPoE anchor finding. Full scoped lint remains owed.

### Admission proposal v2 and nil follow-up (2026-11-03)

The judge refused both MTU substitutions in proposal `0c80dbfd`: configuration
66 hits config admission before the independent negotiated-result guard. That
proposal remains unapplied. Only the five approved transport-only units were
adapted in `rfc1661_renegotiation_test.go`.

`TestNegotiatedResultMinimum` uses configuration 1492, usable IPv4 addresses,
and raw negotiated results 66 and 68. Removing only the negotiated-MTU floor
made /66 fail with an admitted result; /68 passed. The producer was restored
byte-identically. Both cases, both nil transport cases and the five approved
negotiation tests then passed under race instrumentation, without skips.

Exact approved and applied replacement: session scratch `child12-admission-v2.patch`,
SHA256 `a0dac68f6bda8099f816334299febde08efe39d82627ce12a026486d78703a80`.
Its applicability and post-application reverse checks pass. It uses valid-config direct result admission
alongside separate Nak/EOF wire failure ordering, retains the positive 68
exchange and no-PASS/current/publication/Done checks, and adapts unsupported
transport expectations to refusal before any read or write. EOF propagation is
not constructor-wiring proof. The independent judgment is
`child12-admission-v2-judge-a0dac68f.md`, with all three approvals recorded in
`tmp/commit-weakening-approved-21ae89aa.md`. The exact patch was applied through
the patch tool. `TestInitialMTUAdmission`, `TestRestartMTUAdmission` (both 66/68)
and `TestSessionTransportRequired` passed with race instrumentation and no skips
in `job-child12-admission-v2-green-72ac1530.log` under the existing session scratch.

Formatting checks on the two adapted files and two new admission tests are clean.
Package-scoped lint was attempted once after these changes. It refused planning
because shared lint inputs changed after linux-amd64; no lint plan or source
findings were produced. Full output is retained at
`/home/thomas/.local/share/opencode/shell/d5f11cfdc14913550e13a854964057e943f53a43/sh_1a22b52c6001qrmDvC7da6da3M.out`.
Scoped lint remains unestablished until a stable-input run succeeds. No unchanged
98-test selection or kernel check was repeated.

The existing session checkpoint records the exact commands and red/green logs.
Phase stays 5/6. Kernel 7.3, RFC discrimination, wholesale children 14+18, scoped
lint and final external review remain open.

### Epoch convergence policy rationale

The ten-epoch bound permits at most ten sets of the existing LCP, authentication
and IPCP deadlines per attempt (900 seconds of phase waiting). It does not cap
the number of completed, separated network-phase renegotiations.
RFC 1661 Section 4.6 states: "Max-Configure MUST be configurable, but SHOULD default
to ten (10) transmissions." Existing server policy is `ppp/session_run.go`
`defaultMaxConfigure`; this client decision reuses its default value rather than
adding an operator option or an unrelated fallback.

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Renegotiate in place on the same PPPoE session and unit | end the session and reconnect | RFC 1661 Section 4.1 keeps the lower layer Up; a reconnect sends PADT to a server that asked only for renegotiation, and in wholesale access the LNS renegotiates on every session, so reconnecting loops forever |
| Discard IP with PPPIOCSNPMODE drop during renegotiation | leave the unit passing IP | RFC 1661 Section 3.6 MUST discard network-layer packets while the NCP is not Opened |
| Report changed address or MTU to iface through the session value | iface polls, or always tears down addresses | only the dialer knows IPCP results; the change channel keeps one writer of interface state |
| Fresh local Magic-Number on renegotiation | keep the old one | the same `generateMagic` path as the first negotiation; open question below if a peer depends on the old value |
| accel-ppp proof as LNS behind the Ze LAC | accel-ppp as PPPoE AC | no accel-ppp knob renegotiates PPPoE LCP; its LNS skips proxy LCP AVPs, which forces renegotiation by construction |

## Known Limitations
- DROP/PASS currently controls IPv4 only through `SetIPv4Mode`. Child 8's IPv6CP implementation must add the corresponding IPv6 NP-mode gate and traffic proof before it enables IPv6. This spec implements no IPv6 gate and claims no IPv6 discard evidence.
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
- [ ] AC-1..AC-15 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete
- [ ] Scoped tests pass in the main checkout; GitHub CI verifies pushed commits
- [ ] Every A-N confirmed or broken

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it

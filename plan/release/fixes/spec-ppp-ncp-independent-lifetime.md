# Spec: ppp-ncp-independent-lifetime

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P1 |
| Release reason | Shipped subscriber PPP violates RFC 1661 Sections 3.6, 3.7 and 5.7: one NCP closing, or a CPE Protocol-Rejecting IPv6CP, ends the whole subscriber session, and an NCP closed after session-up is never answered or withdrawn. |
| RFC gaps | RFC1661-5.7-2, RFC1661-3.6-3, RFC1661-4.3-3 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-28 |

<!-- Bucket: plan/release/fixes/. Every defect here is incorrect shipped behaviour of
     an implemented capability (IPCP, IPv6CP, LCP Protocol-Reject). An IPv4-only CPE
     on a dual-stack pool is the common case on a real BNG, and it loses its session
     30 s after it comes up. -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The subscriber PPP control plane (`internal/component/l2tp/ppp`) ties the life of
the PPP link to the life of every NCP. Four defects follow, each confirmed at the
producer on 2026-10-28:

1. **One NCP going down kills the session, during the NCP phase.**
   `handleNCPPacket` (`ncp.go`) calls `s.fail` when an NCP reaches Closed or
   Stopped. An NCP that takes a Terminate-Request while Opened moves to Stopping
   and never leaves it, because no NCP runs a Restart timer, so `ncpsComplete`
   never holds and `defaultIPTimeout` (30 s) ends the session with NAS-Error.
2. **No NCP Restart timer at all.** No TO+ or TO- event ever reaches the IPCP or
   IPv6CP automaton (`handleRestartTimeout` in `session_run.go` serves LCP only).
   A lost NCP Configure-Request is never retransmitted, and Stopping and Closing
   never complete.
3. **Protocol-Reject naming IPCP or IPv6CP is ignored.** `codeToEvent`
   (`session_run.go`) maps an LCP Protocol-Reject to RXJ+, which is correct for
   the LCP automaton, but nothing reads the rejected Protocol field, so the named
   NCP is never stopped. An IPv4-only CPE (pppd without `+ipv6`, which answers
   IPv6CP with a Protocol-Reject from `get_input` in pppd `main.c`) under a
   dual-stack pool with `enable-ipv6cp true` is dropped after 30 s.
4. **After session-up the unit descriptor is never read.** `afterLCPOpen` issues
   `PPPIOCCONNECT` and from then on Ze only ever closes `unitFile`. Linux 7.2.9
   (the kernel Ze ships, `internal/appliance` `kernel.version`) delivers every
   protocol below 0xc000 to the unit once a channel is connected, and queues each
   one with no network-protocol index on the unit's file queue. So an IPCP or
   IPv6CP Terminate-Request after session-up gets no Terminate-Ack, the family is
   never withdrawn (the address stays on pppN, the RA and DHCPv6 service keeps
   running, the redistributed /32 or /128 stays announced), an unknown protocol
   gets no Protocol-Reject, and the `tld` action is a no-op in
   `performNCPAction` (`ncp.go`).

No test covers any of this: `TestIPv6CPRejectionKeepsIPv4Session` covers only a
pool handler declining IPv6 before IPv6CP starts.

Goal: an NCP opens and closes independently of the others for the whole life of
the session, as RFC 1661 Section 3.6 allows, and the session ends only when no
NCP remains open or negotiating, as pppd does.

## Required Reading

### Architecture Docs
- [ ] `ai/digests/subscriber.md` - PPP handoff and the shared NCP path for PPPoE and L2TP
  → Constraint: both transports converge on `ppp.StartSession`; a fix in `ppp` serves both, and both transports consume the same `ppp.Event` stream (`pppoe/subsystem.go` event consumer, `l2tp` `reactor_kernel.go` `handlePPPEvent`)
- [ ] `ai/doors/access-security.md` - door for `l2tp`, PPPoE and PPP
- [ ] `docs/guide/l2tp.md` - PPP phases, NCP independence claim, teardown, Redistribute
  → Constraint: the guide states "NCPs are independent (RFC 1661 S2)" for the pool-decline case only; this spec makes the claim true for every case and must rewrite that paragraph
- [ ] `docs/architecture/l2tp/subscriber-session-model.md` - per-session model the NCP lifetime changes
- [ ] `plan/release/fixes/spec-lcp-restart-counters.md` - owns the LCP Restart timer and the configurable counters
  → Constraint: the NCP Restart timer added here reads the same Restart time, Max-Terminate and Max-Configure values LCP reads; if that spec lands first and makes them configurable, the NCP timer takes the configured values, never a second copy

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc1661.md` - rows RFC1661-3.6-1, RFC1661-3.6-3, RFC1661-4.3-3, RFC1661-5.7-1, RFC1661-5.7-2, RFC1661-5.7-5
  → Constraint: RFC 1661 Section 3.6: "Each NCP MAY be Opened and Closed at any time."
  → Constraint: RFC 1661 Section 3.7, Implementation Note: "Conversely, the fact that one NCP has Closed is not sufficient reason to cause the termination of the PPP link, even if that NCP was the only NCP currently in the Opened state."
  → Constraint: RFC 1661 Section 5.7: "Upon reception of a Protocol-Reject, the implementation MUST stop sending packets of the indicated protocol at the earliest opportunity."
  → Constraint: RFC 1661 Section 4.3: "The RXJ+ event arises when the rejected value is acceptable, such as a Code-Reject of an extended code, or a Protocol-Reject of a NCP.  These are within the scope of normal operation.  The implementation MUST stop sending the offending packet type." So LCP keeps RXJ+; the stop applies to the named NCP
  → Constraint: RFC 1661 Section 5.7: "Protocol-Reject packets received in any state other than the LCP Opened state SHOULD be silently discarded."
- [ ] `rfc/short/rfc5072.md` - IPv6CP
  → Constraint: RFC 5072 Section 3: "The IPv6 Control Protocol is exactly the same as the LCP [1] with the following exceptions:" so IPv6CP owes the full automaton, Restart timer included
- [ ] `rfc/short/rfc1332.md` - IPCP, same automaton

**Key insights:**
- Kernel proof for defect 4, Linux 7.2.9 `drivers/net/ppp/ppp_generic.c`, fetched from `git.kernel.org` stable tag `v7.2.9` and copied to the session scratch directory: `ppp_input` puts a frame on the channel's own queue only when the channel has no unit, or the protocol is 0xc000 or above, or it is CCP fragment; every other frame goes to `ppp_do_recv`. `ppp_receive_nonmp_frame` then looks up the protocol's network-protocol index and, when there is none, queues the frame on the unit's file queue (the kernel comment there reads "control or unknown frame - pass it to pppd"). `proto_to_npindex` maps only PPP_IP, PPP_IPV6, PPP_IPX, PPP_AT and the MPLS protocols, so IPCP (0x8021), IPv6CP (0x8057) and CCP (0x80fd) land on the unit fd, LCP (0xc021) and the auth protocols stay on the channel fd.
- Kernel transmit gate: `PPPIOCSNPMODE` sets the per-protocol NP mode of the unit, the transmit path discards a datagram whose family is in `NPMODE_DROP` or `NPMODE_ERROR`, and the receive path discards one whose mode is not `NPMODE_PASS`. That is the kernel boundary Ze owns for "stop sending packets of the indicated protocol".
- pppd reference: `np_finished` (`pppd/auth.c`) decrements the count of open network protocols and, only when it reaches zero, closes LCP with the reason "No network protocols running". `ipcp_down` (`pppd/ipcp.c`) sets the IPv4 NP mode to `NPMODE_DROP` and clears the addresses; the link stays up. `lcp_rprotrej` (`pppd/lcp.c`) dispatches the rejected protocol to that protocol's `protrej` routine, and `ipcp_protrej` lowers only the IPCP automaton.
- accel-ppp-ng reference: commit `d62d8aa` stops terminating the session on an IPv6CP Terminate-Request unless `ipv6=require`, citing RFC 1661 Section 3.7, and `ipv6cp_recv_proto_rej` (`accel-pppd/ppp/ppp_ipv6cp.c`) runs `ppp_fsm_lower_down` then `ppp_fsm_close` on the IPv6CP automaton alone. accel-ppp still ends the session on an IPCP Terminate-Request (`ipcp_recv`, `ppp_ipcp.c`): an asymmetry the RFC note does not support, so Ze follows pppd's symmetric rule. accel-ppp holds the IPv4 lease for the session (`alloc_ip` in `ipcp_opt_ipaddr.c`, released by `ap_session_finished` in `session.c`), not for the NCP.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/ppp/ncp.go` (1302L) - `runNCPPhase` waits for every enabled NCP to reach Opened under one `ipTimeout` timer; `ncpsComplete`; `handleNCPPacket` calls `s.fail` on Closed or Stopped; `performNCPAction` treats `tld`, `tlu`, `tls`, `tlf`, `irc`, `zrc` as no-ops; `resetNCP` and `teardownNCPResources` withdraw both families together on LCP down or session exit.
- [ ] `internal/component/l2tp/ppp/session_run.go` (1992L) - `run` main select over stop, `readDone`, nego timer, LCP `restartTimer`, echo and reauth tickers, `frames`; one `readFrames` goroutine on `chanFile`; `afterLCPOpen` runs auth, NCP, then `PPPIOCCONNECT`, MRU, MTU, admin-up, IPv6 service, `EventSessionUp`; `handleFrame` dispatches LCP, PAP, IPCP, IPv6CP, otherwise `rejectUnsupportedProtocol`; `codeToEvent` maps Code-Reject and Protocol-Reject to RXJ+; `handleRestartTimeout` serves LCP only.
- [ ] `internal/component/l2tp/ppp/session.go` - `pppSession` holds `unitFile io.ReadCloser`, read by nothing.
- [ ] `internal/component/l2tp/ppp/manager.go` - `newUnitFileFn` wraps the unit fd with `NewFDFile` (the seam tests replace through `export_test.go`).
- [ ] `internal/component/l2tp/route_observer.go` - `RouteObserver` has `OnSessionIPUp` and `OnSessionDown`; no per-family down.
- [ ] `internal/component/l2tp/reactor_kernel.go` - `handleSessionIPAssigned` drives `OnSessionIPUp`.
- [ ] `internal/component/l2tp/pppoe/subsystem.go` - event consumer handles `EventSessionIPAssigned` and `EventSessionDown`.
- [ ] `internal/component/l2tp/plugins/pool/register.go` - leases are released on `subevents.SessionDown` only.
- [ ] `internal/component/l2tp/ppp/ncp_test.go` - `TestIPv6CPRejectionKeepsIPv4Session` covers the pool-decline path only.

**Behavior to preserve:**
- A pool handler declining IPv6 drops IPv6CP before it starts and Protocol-Rejects the peer's IPv6CP (`disableIPv6CP`, `rejectUnsupportedProtocol`).
- LCP RXJ+ handling of the LCP automaton itself (Opened RXJ+ stays Opened).
- `EventSessionIPAssigned` acknowledgement barrier before forwarding is enabled.
- LCP down (`resetNCP`) withdraws every family and disconnects the channel, as today.
- NCP frames are discarded before authentication completes (`ncpStarted` gate).
- The pool lease lives for the session and is released on session-down.

**Behavior to change:**
- An NCP reaching Closed or Stopped withdraws its own family only; the session ends only when no NCP remains open or negotiating.
- Each NCP runs a Restart timer with TO+ and TO- events.
- A Protocol-Reject naming IPCP or IPv6CP stops that NCP.
- Frames on the unit descriptor are read for the session's life and dispatched like channel frames.
- `tld` and `tlf` on an NCP withdraw that family; `tlu` (re)installs it.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- PPP control frames from the subscriber: on the channel fd before `PPPIOCCONNECT` (all protocols) and after it (protocols at or above 0xc000); on the unit fd after it (protocols below 0xc000 with no network-protocol index).

### Transformation Path
1. Kernel `ppp_input` routes the frame to the channel queue or, through `ppp_receive_nonmp_frame`, to the unit queue.
2. `readFrames` (channel) and a new lifetime reader on the unit fd hand owned frame buffers to the session loop.
3. `handleFrame` parses the PPP header and dispatches by protocol: LCP to `handleLCPPacket`, IPCP and IPv6CP to `handleNCPPacket`, anything else to `rejectUnsupportedProtocol`.
4. `handleLCPPacket`, on a Protocol-Reject in LCP Opened, reads the rejected Protocol field and raises the stop event on the named NCP automaton.
5. `handleNCPPacket` and the NCP Restart timeout run the RFC 1661 automaton; its actions send packets (`scr`, `sca`, `scn`, `str`, `sta`, `scj`), install the family (`tlu`), withdraw it (`tld`, `tlf`), or arm the timer (`irc`, `zrc`).
6. Family withdrawal sets the kernel NP mode of the family's datagram protocol to drop, removes the IPCP address or stops the IPv6 service, and emits a per-family release event.
7. When the last NCP finishes, the session runs LCP Close (Terminate-Request) and ends with `EventSessionDown`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Kernel PPP unit ↔ session goroutine | read on the unit fd via `NewFDFile`, `PPPIOCSNPMODE` ioctl on the unit fd | No |
| `ppp` ↔ transports (PPPoE subsystem, L2TP reactor) | new per-family release event on `EventsOut`, beside `EventSessionIPAssigned` | No |
| Transport ↔ redistribution | `RouteObserver` per-family down; PPPoE route-change remove batch for one family | No |

### Integration Points
- `pppSession.run` select - gains the unit-frame source and the NCP Restart timers.
- `teardownNCPResources` - split per family so one family can be withdrawn alone; LCP-down keeps withdrawing both.
- `RouteObserver` (`internal/component/l2tp/route_observer.go`) - gains a per-family down; `subscriberRouteObserver` emits the remove batch for that prefix.
- PPPoE subsystem event consumer - handles the release event the way it handles `EventSessionDown` for one family.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | unit frames enter the same `handleFrame` dispatch as channel frames |
| No unintended coupling (components stay isolated) | Yes | transports learn of withdrawal through the existing `ppp.Event` stream |
| No duplicated functionality (extends existing, does not recreate) | Yes | NCP timer reuses the LCP FSM table `LCPDoTransition` and the LCP counter values |
| Zero-copy preserved where applicable (refs, not copies) | Yes | unit reader uses the pooled frame buffers (`getFrameBuf`, `putFrameBuf`) |
| Registration over hardcoding, outbound | Yes | no new command, family or plugin; the event joins the existing `ppp.Event` set |
| Registration over hardcoding, inbound | Yes | searched: `handleFrame` protocol switch, `codeToEvent`, `performNCPAction` action switch, `handlePPPEvent` type switch (`reactor_kernel.go`), PPPoE event consumer switch (`pppoe/subsystem.go`). The two event switches are the existing consumers of `ppp.Event` and each gains one case; an unknown event already reaches a logged fallback |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | After `PPPIOCCONNECT`, IPCP and IPv6CP frames arrive on the unit fd, LCP and auth on the channel fd | `ppp_input` and `ppp_receive_nonmp_frame` in Linux 7.2.9 `ppp_generic.c` (quoted above) | the unit reader receives nothing and defect 4 stays | `.ci` over kernel PPPoL2TP (Functional Tests) | validated by kernel source; test owed |
| A-2 | Closing the `NewFDFile` wrapper unblocks a blocked read on the unit fd, as it does for the channel fd | `readFrames` teardown order comment in `run`; `TestPPPFDClosingRejectsFurtherIO` | a leaked reader goroutine per session | `TestUnitReaderExitsOnClose` | unvalidated |
| A-3 | Writing an NCP reply on the channel fd after connect reaches the wire | Ze writes every NCP frame on `chanFile` today; `ppp_write` on a channel file transmits on that channel | Terminate-Ack never leaves | `.ci` and interop scenarios observe the Terminate-Ack | unvalidated |
| A-4 | pppd 2.5.4 without `+ipv6` answers IPv6CP with an LCP Protocol-Reject | pppd `main.c` `get_input`: a protocol with no enabled `protent` reaches `lcp_sprotrej` | the interop scenario does not exercise defect 3 | the scenario's pcap assertion that a Protocol-Reject naming 0x8057 was sent | unvalidated |
| A-5 | Accounting continues unchanged while a family is withdrawn; the session's RADIUS records keep the session open | the session stays up; pppd and accel-ppp keep the session | an Interim-Update reports a withdrawn address | read `authradius` interim builder during implementation and record what it reports | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A test that expected the session to die on an NCP close goes red | NCP test red after Phase 2 | that red is the defect surfacing: read the producer, fix the product, never the assertion; an RFC-tagged test change goes to `ze-test-judge` |
| R-2 | Re-opening a family after withdrawal re-emits `EventSessionIPAssigned` and a consumer double-counts | duplicate route-change add batch | consumers already handle add after remove for one family; the release event is the explicit remove that precedes it |
| R-3 | The unit reader races `resetNCP`'s `PPPIOCDISCONN` and delivers an NCP frame from the old LCP lifetime | NCP frame accepted before re-authentication | `handleFrame` already discards NCP frames while `ncpStarted` is false; the unit reader feeds the same gate |
| R-4 | Conflict with `spec-lcp-restart-counters` over the restart constants | both specs edit `session_run.go` | the NCP timer reads the same values through one accessor; whichever lands second rebases onto it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Live subscriber sessions: a wrong last-NCP count ends a session early or keeps a session with no network protocol |
| How is it reverted? | single commit revert per phase |
| Who else touches this path? | `spec-lcp-restart-counters` (LCP timer, `tld` notification), `spec-ppp-l2tp-aaa-one-off-defects` (NCP Configure-Reject), `spec-radius-rfc3162-ipv6-attributes` (IPv6 attributes in accounting) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| IPv6CP Terminate-Request on the unit fd after `EventSessionUp` | → | unit reader, `handleFrame`, `handleNCPPacket`, family withdrawal | `TestNCPTerminateAfterSessionUpOnUnitFD` |
| LCP Protocol-Reject naming 0x8057 during the NCP phase | → | `handleLCPPacket` rejected-protocol dispatch, NCP stop, `runNCPPhase` completion | `TestProtocolRejectIPv6CPKeepsIPv4Session` |
| Real kernel PPPoL2TP session, peer closes IPv6CP after up | → | unit fd delivery in `ppp_generic`, Ze unit reader | `test/l2tp/ppp-ncp-terminate-after-up.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Dual-stack session in the NCP phase; IPCP reaches Opened, the peer sends IPv6CP Terminate-Request | Ze sends IPv6CP Terminate-Ack, IPv6CP reaches Stopped after one Restart time, `EventSessionUp` is emitted with IPv4 alone, no `EventSessionDown` |
| AC-2 | Dual-stack session; the peer answers Ze's IPv6CP Configure-Request with LCP Protocol-Reject naming 0x8057, before or after IPCP opens | IPv6CP stops, Ze sends no further IPv6CP packet, the session comes up with IPv4 and stays up past `ncp timeout` |
| AC-3 | As AC-2 with 0x8021 (IPCP) rejected and IPv6CP open | IPCP stops, the session stays up with IPv6 alone |
| AC-4 | Session up; peer sends IPCP or IPv6CP Terminate-Request | the frame is read from the unit fd, Ze sends Terminate-Ack with the request's Identifier, the family is withdrawn: kernel NP mode for 0x0021 or 0x0057 set to drop, IPv4 address removed from pppN or IPv6 service stopped, a per-family release event emitted, the redistributed /32 or /128 removed; the other family keeps forwarding |
| AC-5 | Session up; peer sends Protocol-Reject naming the open IPv6CP | same withdrawal as AC-4 for IPv6; no IPv6CP packet and no IPv6 datagram leaves after it |
| AC-6 | The last open or negotiating NCP finishes (Terminate, Protocol-Reject, or Max-Configure exhausted) | Ze runs LCP Close (sends LCP Terminate-Request) and the session ends with `EventSessionDown`; cause User-Request when the peer's Terminate-Request closed it, NAS-Error otherwise |
| AC-7 | NCP Configure-Request lost (peer silent) | Ze retransmits it each Restart time up to Max-Configure, then the NCP reaches Stopped (TO-) and AC-6 applies if it was the last |
| AC-8 | After AC-4 the peer sends a new Configure-Request for the withdrawn family | the NCP renegotiates with the session's held address or interface identifier, re-installs the family on Opened (NP mode pass, address or IPv6 service, `EventSessionIPAssigned`) |
| AC-9 | Session up; peer sends a frame of an unsupported protocol below 0xc000 (for example CCP 0x80fd) | read from the unit fd and answered with LCP Protocol-Reject (RFC1661-3.6-3) |
| AC-10 | Protocol-Reject received while LCP is not Opened | silently discarded, no NCP affected (RFC1661-5.7-5) |
| AC-11 | Session ends or LCP goes down | the unit reader exits; no goroutine outlives the session |
| AC-12 | No NCP reaches Opened before `ncp timeout` | session ends with NAS-Error as today |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | IPv4-only CPE (pppd without `+ipv6`) dials a dual-stack PPPoE BNG | PADR, LCP, CHAP, IPCP Opened, IPv6CP Protocol-Rejected, session up, IPv4 traffic | interop `pppoe-ipv4-only-cpe-rejects-ipv6cp` |
| 2 | Dual-stack CPE drops IPv6 after it came up and keeps IPv4 | IPv6CP Terminate-Request on unit fd, Terminate-Ack, IPv6 withdrawn, IPv4 keeps passing | `test/l2tp/ppp-ncp-terminate-after-up.ci`, interop `pppoe-ipv6cp-terminate-after-up` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNCPClosedDoesNotEndSessionWhileOtherOpen` | `internal/component/l2tp/ppp/ncp_lifetime_test.go` | AC-1: FSM path Opened RTR to Stopping, Restart TO- to Stopped, no `s.fail`, `EventSessionUp` | |
| `TestNCPRestartTimerRetransmitsConfigureRequest` | `ncp_lifetime_test.go` | AC-7: TO+ resends with a new Identifier, TO- after Max-Configure | |
| `TestNCPRestartTimerCompletesStopping` | `ncp_lifetime_test.go` | Stopping reaches Stopped after the Restart time (zrc then TO-) | |
| `TestProtocolRejectIPv6CPKeepsIPv4Session` | `ncp_lifetime_test.go` | AC-2 through the real session loop with the pipe driver | |
| `TestProtocolRejectIPCPKeepsIPv6Session` | `ncp_lifetime_test.go` | AC-3 | |
| `TestProtocolRejectIgnoredOutsideLCPOpened` | `ncp_lifetime_test.go` | AC-10 | |
| `TestLastNCPFinishedClosesLCP` | `ncp_lifetime_test.go` | AC-6, both causes | |
| `TestNCPTerminateAfterSessionUpOnUnitFD` | `ncp_lifetime_test.go` | AC-4 through `run`: frame injected on the replaced unit file (`newUnitFileFn` seam), Terminate-Ack observed, NP-mode drop recorded by the fake ops, address removal recorded by the fake backend, release event emitted | |
| `TestProtocolRejectAfterSessionUpWithdrawsFamily` | `ncp_lifetime_test.go` | AC-5 | |
| `TestNCPReopenAfterWithdrawal` | `ncp_lifetime_test.go` | AC-8 | |
| `TestUnknownProtocolOnUnitFDProtocolRejected` | `ncp_lifetime_test.go` | AC-9 | |
| `TestUnitReaderExitsOnClose` | `read_frames_test.go` | AC-11, A-2 | |
| `TestRouteObserverFamilyDownRemovesPrefix` | `internal/component/l2tp/route_observer_test.go` | per-family remove batch | |
| `TestPPPoEFamilyReleaseWithdrawsOneFamily` | `internal/component/l2tp/pppoe/subsystem_test.go` | PPPoE consumer of the release event | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Protocol-Reject Rejected-Protocol | 2 octets present | 2 octets | fewer than 2 octets: discarded, no NCP affected (`TestNGLCPShortProtocolRejectIsNoOp` stays green) | N/A |
| NCP restart count | 0 to Max-Configure | Max-Configure-th TO+ | N/A | next timeout is TO- |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ppp-ncp-terminate-after-up` | `test/l2tp/ppp-ncp-terminate-after-up.ci` | needs-linux, QEMU root, real kernel PPPoL2TP like `radius-acct-wire.ci`: the fixture peer opens IPCP and IPv6CP, sends IPv6CP Terminate-Request after up, expects Terminate-Ack, then LCP echo still answered and no CDN | |
| `ppp-ncp-protocol-reject-ipv6cp` | `test/l2tp/ppp-ncp-protocol-reject-ipv6cp.ci` | needs-linux: fixture peer Protocol-Rejects IPv6CP, IPCP opens, session alive past a short `ncp timeout` | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `pppoe-ipv4-only-cpe-rejects-ipv6cp` | `test/interop-pppoe/scenarios/` | pppd 2.5.4 with rp-pppoe, no `+ipv6`; Ze pool has IPv4 and IPv6 | session stays up past `ncp timeout`, IPv4 ping passes, pcap shows the Protocol-Reject naming 0x8057 and no IPv6CP from Ze after it | |
| `pppoe-ipv6cp-terminate-after-up` | `test/interop-pppoe/scenarios/` | pppd 2.5.4 with `+ipv6`; the checker injects an IPv6CP Terminate-Request on the session from the client container | Ze answers Terminate-Ack (pcap), IPv4 ping still passes, pppd session alive | |

Both scenarios owe the revert walk: restore `s.fail` on Closed or Stopped, or remove the unit reader, rebuild the Ze image, and record the red.

## Files to Modify
- `internal/component/l2tp/ppp/ncp.go` - `runNCPPhase` completion rule, `ncpsComplete`, `handleNCPPacket` terminal states, `performNCPAction` (`tlu`, `tld`, `tlf`, `irc`, `zrc`), per-family withdrawal split out of `teardownNCPResources`, NCP Restart timer handling
- `internal/component/l2tp/ppp/session_run.go` - unit reader start and teardown in `run`, NCP timer cases in the select, `handleLCPPacket` Protocol-Reject dispatch, last-NCP LCP Close
- `internal/component/l2tp/ppp/session.go` - per-family Restart timer and counter fields, finished-family state
- `internal/component/l2tp/ppp/ops.go`, `internal/component/l2tp/ppp/mtu_linux.go`, `internal/component/l2tp/ppp/mtu_other.go` - `setNPMode` seam over `PPPIOCSNPMODE`
- `internal/component/l2tp/ppp/events.go` - per-family release event
- `internal/component/l2tp/route_observer.go` - per-family down
- `internal/component/l2tp/reactor_kernel.go` - consume the release event
- `internal/component/l2tp/pppoe/subsystem.go` - consume the release event
- `internal/le/interoplab/pppoe/` - checkers for the two scenarios, frame injection from the client container
- `internal/test/fixture/tunnel_fixture_l2tp_ppp.go` - IPv6CP negotiation, post-up Terminate-Request and Protocol-Reject in the fixture peer
- `rfc/short/rfc1661.md` - RFC1661-5.7-2 and RFC1661-4.3-3 gap annotations replaced by tagged tests; RFC1661-3.6-3 after-up proof
- `docs/guide/l2tp.md`, `docs/guide/pppoe.md`, `docs/architecture/l2tp/subscriber-session-model.md` - NCP lifetime
- `docs/research/l2tpv2-ze-integration.md` - `// Design:` doc of `ncp.go` and `session_run.go`: NCP coordinator, session loop, unit reader
- `docs/research/l2tpv2-implementation-guide.md` - `// Design:` doc of changed `ppp` files: NCP failure no longer ends the session
- `docs/architecture/l2tp/bng-5-pppoe.md` - `// Design:` doc of `pppoe/subsystem.go`: the per-family release event

## Files to Create
- `internal/component/l2tp/ppp/ncp_lifetime_test.go`
- `test/l2tp/ppp-ncp-terminate-after-up.ci`
- `test/l2tp/ppp-ncp-protocol-reject-ipv6cp.ci`
- `test/interop-pppoe/scenarios/pppoe-ipv4-only-cpe-rejects-ipv6cp/` (`role`, `ze.conf`)
- `test/interop-pppoe/scenarios/pppoe-ipv6cp-terminate-after-up/` (`role`, `ze.conf`)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new leaf: see Key Design Decisions |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | No | none |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | N-A | no leaf |
| Functional test for new RPC/API | N-A | no RPC; behaviour `.ci` listed above |
| Pipe completeness | N-A | no output |
| Env var registration | No | none |
| Doctor check for runtime dependencies | No | `PPPIOCSNPMODE` is part of the `/dev/ppp` unit Ze already opens; no new device, module or path |
| Prometheus counters/metrics | No | none added |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | behaviour fix of an existing feature |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/l2tp.md` (PPP phases paragraph claiming NCP independence, teardown, Redistribute per-family remove), `docs/guide/pppoe.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc1661.md`, `rfc/short/rfc5072.md`, `rfc/short/rfc1332.md`, `docs/features/rfc-status.md` row |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` (two PPPoE scenarios, frame injection) |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/l2tp/subscriber-session-model.md`, `ai/digests/subscriber.md` step 16 |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | the new `ppp.Event` is internal to the access component |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/release/fixes/spec-ppp-ncp-independent-lifetime.md`. Declared `// Design:` docs of the changed files: `docs/research/l2tpv2-ze-integration.md` (`ncp.go`, `session_run.go`: NCP coordinator and session loop, updated for the unit reader and per-NCP lifetime), `docs/research/l2tpv2-implementation-guide.md` (updated where it describes NCP failure ending the session), `docs/architecture/l2tp/bng-5-pppoe.md` (PPPoE consumer of the release event, updated). Advisory mentions: `docs/architecture/l2tp/bng-1-radius-attributes.md` (re-check against A-5: update if an Interim-Update reports a withdrawn family), `docs/guide/configuration.md` (unaffected: no config change) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/l2tp.md` `ncp` container example stays valid; verify |

## Implementation Steps

1. **Phase: Wiring** - unit reader feeding the session loop, release event type, `RouteObserver` per-family down, both transport consumers
   - Tests: `TestNCPTerminateAfterSessionUpOnUnitFD` (fails: no Terminate-Ack), `TestUnitReaderExitsOnClose`
   - Files: `session_run.go`, `events.go`, `route_observer.go`, `reactor_kernel.go`, `pppoe/subsystem.go`
2. **Phase: NCP Restart timer** - per-family timer, TO+ and TO-, `irc` and `zrc`
   - Tests: `TestNCPRestartTimerRetransmitsConfigureRequest`, `TestNCPRestartTimerCompletesStopping`
3. **Phase: independent lifetime** - per-family withdrawal (`tld`, `tlf`, NP mode drop), reinstall on `tlu`, completion rule in `runNCPPhase`, last-NCP LCP Close
   - Tests: `TestNCPClosedDoesNotEndSessionWhileOtherOpen`, `TestLastNCPFinishedClosesLCP`, `TestNCPReopenAfterWithdrawal`, `TestProtocolRejectAfterSessionUpWithdrawsFamily`
4. **Phase: Protocol-Reject** - rejected-protocol dispatch in `handleLCPPacket`, discard outside LCP Opened
   - Tests: `TestProtocolRejectIPv6CPKeepsIPv4Session`, `TestProtocolRejectIPCPKeepsIPv6Session`, `TestProtocolRejectIgnoredOutsideLCPOpened`, `TestUnknownProtocolOnUnitFDProtocolRejected`
5. **Phase: functional and interop** - fixture peer, two `.ci`, two PPPoE scenarios with the revert walk, RFC summary and docs

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | the last-NCP count treats a pool-declined or config-disabled NCP as never enabled, and a finished NCP as gone; a negotiating NCP keeps the session |
| Correctness | LCP keeps RXJ+ for a Protocol-Reject of an NCP; only the named NCP stops |
| Correctness | NP mode is set to drop before the address is removed, and to pass only after the family is installed and acknowledged |
| Data flow | unit frames pass the `ncpStarted` gate exactly as channel frames do |
| Rule: goroutine-lifecycle | the unit reader is one long-lived goroutine per session and exits on close |
| Rule: rfc-compliance | every new implementing function quotes its RFC sentence; callers cite the section |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| unit reader | `TestNCPTerminateAfterSessionUpOnUnitFD` |
| no `s.fail` on NCP Closed or Stopped | grep `handleNCPPacket`; `TestNCPClosedDoesNotEndSessionWhileOtherOpen` |
| kernel path | `.ci` under needs-linux QEMU, pasted output |
| interop | both scenarios green, revert walk red recorded |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Protocol-Reject shorter than 2 octets discarded; a rejected Protocol naming LCP itself or an unknown protocol does not stop any NCP |
| Resource exhaustion | the unit reader uses pooled buffers and the existing frame channel; a peer flooding unknown protocols is answered at the Protocol-Reject rate the channel path already applies |
| Fail open | a failed `PPPIOCSNPMODE` on withdrawal is a session failure, never a silent continue with the family still passing |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The deferred `PPPIOCCONNECT` (after NCP) means every NCP frame during the NCP phase arrives on the channel fd, so the defect is invisible until the session is up. Any later frame below 0xc000 needs the unit reader.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `transient-failure-treated-as-fatal.md` | 2026-10-28 | NCP Closed or Stopped ends the session; Stopping never completes |
| `transient-failure-treated-as-fatal.md` | 2026-10-28 | No NCP Restart timer |
| `silent-fall-through.md` | 2026-10-28 | Protocol-Reject naming IPCP or IPv6CP ignored |
| `silent-fall-through.md` | 2026-10-28 | Unit fd never read after `PPPIOCCONNECT` |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The session ends only when no NCP remains open or negotiating | accel-ppp-ng `ipv6=require` leaf; accel-ppp's asymmetric rule (IPCP Terminate fatal, IPv6CP not) | RFC 1661 Section 3.7 note covers every NCP alike; pppd `np_finished` implements exactly that. No operator need for a `required` leaf is shown, and the RFC does not ask for one, so none is added |
| A withdrawn family keeps its pool lease and negotiated identifiers until session-down | release the lease per family | accel-ppp holds the lease per session (`alloc_ip`, `ap_session_finished`); a re-opened NCP (Section 3.6 "at any time") then gets the same address with no new pool round-trip, and the pool plugin's session-down release stays the only release path |
| Withdrawal sets the family's NP mode to drop | rely on address removal alone | pppd `ipcp_down` sets `NPMODE_DROP`; Section 5.7 asks Ze to stop sending the protocol, and the kernel transmit gate is the boundary Ze owns for that |
| LCP keeps RXJ+ for a Protocol-Reject of an NCP, and the NCP gets the stop | map to RXJ- | RFC 1661 Section 4.3 names "a Protocol-Reject of a NCP" as RXJ+ for the LCP automaton |
| Last NCP finished runs LCP Close; cause User-Request for a peer Terminate-Request, NAS-Error otherwise | always NAS-Error | RFC 2866 Section 5.10 lists LCP Terminate under User Request; accel-ppp uses TERM_USER_REQUEST for a TermReq and an error cause for a negotiation failure |
| One unit reader goroutine per session, started with the session | start on connect, stop on disconnect | the unit fd lives for the session across LCP renegotiations; a read before connect yields nothing, and one lifetime matches `readFrames` |

## Known Limitations
- The LCP Restart counters stay compiled-in until `plan/release/fixes/spec-lcp-restart-counters.md` lands; the NCP timer reads the same values.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code: RFC 1661
Sections 3.6, 3.7 (Implementation Note), 4.3 (RXJ+), 4.6 (Restart timer), 5.7, and
RFC 5072 Section 3 for IPv6CP. The Protocol-Reject packet layout (Code 8,
Identifier, Length, Rejected-Protocol at offset 4, Rejected-Information) is
documented above the parse.

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
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits (`/ze-close` step 6f). The full verification is Thomas's release check: no agent runs it, and one that is owed is handed to him as its exact command. A red there gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)

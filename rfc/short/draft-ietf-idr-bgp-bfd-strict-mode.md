# draft-ietf-idr-bgp-bfd-strict-mode - BGP BFD Strict-Mode

## Meta

| Field | Value |
|-------|-------|
| Draft | draft-ietf-idr-bgp-bfd-strict-mode-19 |
| Title | BGP BFD Strict-Mode |
| Status | Internet Draft (Standards Track, expires 2027-02-27) |
| Date | 2026-08-26 |
| Updates | RFC 4271 (if approved) |
| Depends | RFC 4271 (BGP-4 FSM), RFC 5492 (capabilities), RFC 5880 and RFC 5882 (BFD), RFC 9384 (Cease subcode 10) |
| Enrolment | enrolled |
| Enrolment reason | BFD Strict-Mode for BGP (capability code 74): four MUST-level requirements, all four implemented and each proven by a tagged test. Ze advertises the capability from the peer's own bfd block (parsePeerFromTree, internal/component/bgp/reactor/config.go), negotiates it as BfdStrictNegotiated (Negotiate, internal/core/bgp/capability/negotiated.go), and runs the Section 8 FSM procedures in internal/component/bgp/fsm/fsm.go with their wire half in internal/component/bgp/reactor/session_bfd_strict.go. The two Event 20 sections, 8.3.5 and 8.4.5, are conditional on the RFC 4271 DelayOpenTimer, which Ze does not implement (permitted by RFC 4271 Section 8.2.1.3), and they carry no MUST-level keyword site. |
| Support | drafts 70 |
| Support area | BFD strict mode for BGP, capability code 74 |
| Support status | Supported |
| Support coverage | Capability code 74, length 0, advertised in the OPEN for a peer whose `connection bfd { strict true }` is enabled, and negotiated to `Negotiated.BFDStrictMode` when both speakers send it. FSM events 30 to 35 and the two OpenSent sub-states of Section 8.1 are implemented in `internal/component/bgp/fsm/`. The KEEPALIVE is withheld and the session held in OpenSent by `Session.advanceAfterOpen`, released by `Session.handleBFDEvent`, and closed with Cease / BFD Down or Cease / Other Configuration Change by the same function (`internal/component/bgp/reactor/session_bfd_strict.go`). The BFD session opens before the BGP FSM starts and outlives a transition to Idle (`Peer.run`, `Peer.cleanup`, Section 7). The BfdHoldTimer of Section 3 attribute 18 lives on `fsm.Timers`, defaults to 30 seconds, and is armed only when the negotiated BGP hold time is zero; where it is non-zero the ordinary RFC 4271 HoldTimer bounds the wait, re-armed to the negotiated value by `advanceAfterOpen`. The Section 10 BFD hold-down interval is the `hold-down` leaf, in milliseconds, zero by default. Both halves are proven against a second implementation: `test/interop/scenarios/bgp-bfd-strict-speaker` (the lab speaker, which advertises capability 74 and answers BFD built from RFC 5880) and `test/interop/scenarios/bgp-bfd-strict-frr` (FRR 10.3.1, which implements neither). Sections 8.3.5 and 8.4.5 revise Event 20, an OPEN received while the DelayOpenTimer runs; Ze implements no DelayOpenTimer, an RFC 4271 optional session attribute its Section 8.2.1.3 permits omitting, so `ConnectDelayOpenBfdUpPending` and `ActiveDelayOpenBfdUpPending` are unreachable and undeclared. Neither section carries a MUST-level obligation, so that is an implementation gap in RFC 4271's optional feature and not a conformance gap in this draft. |
| Support remaining | - |

**Purpose:** Prevents a BGP session from reaching Established until the BFD
session to that neighbour is Up, and defines a BGP capability so that both
speakers agree to do it. Without the capability, a speaker that waits against
one that does not would never come up, which Section 1 names: "always using
'strict-mode' would preclude BGP operation in an environment where not all
routers support BFD strict-mode".

**Scope:** The capability (Section 5), its negotiation (Section 6), five new BGP
session attributes and six new FSM events (Sections 3 and 4), when the BFD
session starts and stops (Section 7), the revised RFC 4271 Section 8.2.2 state
machine (Section 8), and the Cease subcode every session this feature closes
carries (Section 9).

## Wire Format

Capability Code: 74 (0x4A). Capability Length: 0. No value.

```
 0                   1
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|  Cap. Code=74 | Cap. Length=0 |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

Section 13.1: "The Capability Code 74 has been assigned from the
First-Come-First-Served range (64-238) of the Capability Codes registry."

No new message type and no new attribute. Everything else this document defines
is state machine behaviour, and the only bytes it adds to the wire beyond the
capability are the NOTIFICATIONs of Section 9: Cease (6) with subcode BFD Down
(10), and Cease (6) with subcode Other Configuration Change (6) for a
configuration change.

## Ze Implementation

- Capability: `BFDStrictMode` and `CodeBFDStrictMode`
  (`internal/core/bgp/capability/capability.go`), parsed through
  `parseZeroLengthCapability`, which refuses any length but zero.
- Negotiation: `Negotiate` (`internal/core/bgp/capability/negotiated.go`) sets
  `Negotiated.BFDStrictMode` when both sides advertised it, and records a
  `Mismatch` when only one did.
- Advertisement: `parsePeerFromTree`
  (`internal/component/bgp/reactor/config.go`) appends the capability to
  `PeerSettings.Capabilities` when the peer's `connection bfd` block carries
  `strict true` and `enabled` is true.
- Session attributes: `BFDSettings.Strict` and `BFDSettings.HoldTime`
  (`internal/component/bgp/reactor/peer_settings.go`) carry Section 3 items 17
  and 18. Item 20, BfdStrictNegotiated, is `FSM.SetBFDStrict`, which the session
  writes after the OPEN exchange (`Session.applyBFDStrictNegotiation`).
- FSM events 30 to 35: `EventBfdAdminDown` through
  `EventBfdStrictConfigChanged` (`internal/component/bgp/fsm/state.go`), handled
  state by state in `internal/component/bgp/fsm/fsm.go`.
- Sub-states: `SubStateOpenSentBfdUpPending` and
  `SubStateOpenSentConfirmedBfdUpPending` (`internal/component/bgp/fsm/state.go`),
  entered by `FSM.EnterBfdUpPending` and cleared by `FSM.change` on any
  transition out of OpenSent.
- BfdHoldTimer: `Timers.StartBfdHoldTimer`, `DefaultBfdHoldTime`
  (`internal/component/bgp/fsm/timer.go`).
- The wire half: `Session.advanceAfterOpen`, `Session.handleBFDEvent` and
  `Session.bfdTeardown` (`internal/component/bgp/reactor/session_bfd_strict.go`).
- Session lifetime: `Peer.startBFDClient` before the start event in
  `Peer.runOnce`, released by `Peer.cleanup`
  (`internal/component/bgp/reactor/peer_bfd.go`, `peer_run.go`).
- Config: `bgp peer connection bfd { strict, hold-time }`
  (`internal/component/bgp/yang/ze-bgp-conf.yang`).

## Compliance Checklist

- [ ] [DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-4-1] [MUST NOT] "If BfdEnabled is FALSE, this event MUST NOT occur. When BFD has been disabled, the local system will trigger a BfdAdminDown event instead" (§4, Event 35)
- [ ] [DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-6-1] [MUST] "A BGP speaker which supports capabilities advertisement and has BFD strict-mode enabled MUST include the BFD Strict-Mode Capability in its OPEN message" (§6)
- [ ] [DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-1] [MUST] "To avoid deadlock when utilizing both BFD hold-down and BFD strict-mode, when strict-mode is enabled for a peer, the BGP FSM MUST be enabled" (§10)
- [ ] [DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-2] [MUST NOT] "That is, BFD hold-down procedures MUST NOT prevent BGP from establishing a connection with the remote BGP speaker" (§10)

## Notes

The state machine of Section 8 carries no RFC 2119 keyword at all. It is written
as RFC 4271 Section 8.2.2 is written, in action lists ("the local system: sends a
KEEPALIVE message, ... and changes its state to OpenConfirm"), so the extraction
register is `rfc2119` over four keyword sites and the section walk records where
the obligations that carry no keyword were read. The behaviour those lists
require is implemented and tested state by state in
`internal/component/bgp/fsm/bfd_strict_test.go`, and each test names the section
it reads.

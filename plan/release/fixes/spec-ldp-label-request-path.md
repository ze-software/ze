# Spec: LDP required request, release and abort handling

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P1 |
| Release reason | RFC 5036 expressly requires Label Request processing in shipped Downstream Unsolicited mode; optional full DoD operation is split out. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-21 |

## Task

Implement and prove required request/release/abort behavior applicable to shipped Downstream Unsolicited LDP. RFC5036-A.1.2-1 explicitly binds DU and A.1.7-1 is independent of request procedure. The historical missing encoders are a dated finding, not fresh runtime evidence. Determine the conditions of every listed requirement rather than treating a declined optional DoD mode as a waiver. This migration records scope and does not admit implementation.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

- AC-1: every historical requirement listed in this spec is mapped to its DU/unconditional trigger, with positive and negative evidence for each reachable trigger.
- AC-2: received Label Requests in DU are processed, including exact-route absence, response IDs and resource notifications.
- AC-3: received Label Abort Requests and required Label Release events are handled under their stated conditions.
- AC-4: NH.13-triggered requests and the applicable no-resources retry restriction work even without a configurable DoD mode.
- AC-5: the Hop Count receive requirements retained in the historical addendum are proven for their applicable inputs; optional loop-detection origination is not a waiver.

## Implementation Ownership

| Scope | Current owner |
|---|---|
| All listed RFC5036 requirements for DU/unconditional receive and NH.13 behavior, including their response encoders and Hop Count receive addendum. | `plan/release/fixes/spec-ldp-label-request-path.md` |
| Optional DoD selection and mode-specific origination; no ownership of unconditional or DU receive requirements. | `plan/post-release/features/spec-ldp-downstream-on-demand-mode.md` |

## Work Not Done

| Scope | Current owner |
|---|---|
| Optional DoD selection and mode-specific origination; no ownership of unconditional or DU receive requirements. | `plan/post-release/features/spec-ldp-downstream-on-demand-mode.md` |

## Historical planning record

The following is the complete pre-migration planning body. Its old Task, criteria, implementation steps and bucket statements are retained as history, not a second current implementation contract. The current sections above assign every obligation; original owner decisions, constraints, evidence and pauses remain authoritative. Heading levels alone are demoted for that distinction.

Recovery after compaction: `.claude/rules/post-compaction.md`.

#### Task

Ze runs Downstream Unsolicited label distribution only. The Label Request (0x0401), Label Release (0x0403) and Label Abort Request (0x0404) messages have no encoder in `wire.go`, and `session.go::processMessages` discards a received one at Debug. The Appendix A algorithms A.1.2 (Label Request received) and A.1.7 (Label Release received) name the event handling ze does not implement.

These paths are absent and remain implementation gaps. Applicability is
per requirement: declining Downstream-on-Demand does not excuse
`RFC5036-A.1.2-1`, which requires Label Request processing in Downstream
Unsolicited mode. This scope record does not authorize implementation.

Requirements this spec covers, each with the RFC sentence and the producer or
absence in `internal/plugins/ldp`:

- `RFC5036-3.5.7-1` [MUST] "If a Label Mapping message is a response to a Label Request message, it MUST include the Label Request Message ID optional parameter (§3.5.7)" -- no Label Request, Release or Abort encoder exists in `wire.go`, and `processMessages` discards a received one
- `RFC5036-3.5.8.1-1` [MUST] "Unless its routing table includes an entry that exactly matches the requested Prefix, the LSR MUST respond to a Label Request with a No Route Notification message (§3.5.8.1)" -- no Label Request, Release or Abort encoder exists in `wire.go`, and `processMessages` discards a received one
- `RFC5036-3.5.8.1-2` [MUST] "When the receiving LSR responds with a Label Mapping message, that message MUST include a Label Request/Returned Message ID TLV optional parameter that includes the message ID of the Label Request message (§3.5.8.1)" -- no Label Request, Release or Abort encoder exists in `wire.go`, and `processMessages` discards a received one
- `RFC5036-3.5.8.1-3` [MUST] "When label resources become available, the LSR MUST notify the requesting LSR by sending a Notification message with the Label Resources Available Status Code (§3.5.8.1)" -- no Label Request, Release or Abort encoder exists in `wire.go`, and `processMessages` discards a received one
- `RFC5036-3.5.8.1-4` [MUST NOT] "An LSR that receives a No Label Resources response to a Label Request message MUST NOT issue further Label Request messages until it receives a Notification message with the Label Resources Available Status Code (§3.5.8.1)" -- no Label Request, Release or Abort encoder exists in `wire.go`, and `processMessages` discards a received one
- `RFC5036-3.5.9.1-1` [MUST] "When an LSR receives a Label Abort Request message, if it has not previously responded to the Label Request being aborted with a Label Mapping message or some other Notification message, it MUST acknowledge the abort by responding with a Label Request Aborted Notification message (§3.5.9.1)" -- no Label Request, Release or Abort encoder exists in `wire.go`, and `processMessages` discards a received one
- `RFC5036-3.5.9.1-2` [MUST] "That Label Request Aborted Notification MUST include a Label Request Message ID TLV that carries the message ID of the aborted Label Request message (§3.5.9.1)" -- no Label Request, Release or Abort encoder exists in `wire.go`, and `processMessages` discards a received one
- `RFC5036-3.5.9.1-3` [MUST] "An LSR receiving a Label Abort Request message MUST process it immediately, regardless of the downstream state of the LSP, responding with a Label Request Aborted Notification or ignoring it, as appropriate (§3.5.9.1)" -- no Label Request, Release or Abort encoder exists in `wire.go`, and `processMessages` discards a received one
- `RFC5036-3.5.11.1-1` [MUST] "An LSR MUST transmit a Label Release message under any of the conditions the section lists (§3.5.11.1)" -- no Label Request, Release or Abort encoder exists in `wire.go`, and `processMessages` discards a received one
- `RFC5036-A.1.2-1` [MUST] "An LSR operating in Downstream Unsolicited mode MUST process any Label Request messages it receives (§A.1.2)" -- no Label Request, Release or Abort encoder exists in `wire.go`, and `processMessages` discards a received one
- `RFC5036-A.1.7-1` [MUST] "Regardless of the Label Request procedure in use by the LSR, it MUST send a label request if the conditions in NH.13 hold (§A.1.7)" -- no Label Request, Release or Abort encoder exists in `wire.go`, and `processMessages` discards a received one

#### Hop Count receipt: transferred acceptance criteria

The 2026-10-02 independent rejudgment found that
`internal/plugins/ldp/session.go::processMessages` checks Hop Count in Label
Mapping but leaves Label Request in the unhandled-message branch. Mapping-only
tests cannot prove the whole receipt requirement. These two weak verdicts move
here from `spec-rfc-verdict-fix-routing` (closed 2026-10-12) under parent P-3;
their existing Mapping tests and native records remain.

RFC 5036 Section 3.4.4.1: "If an LSR receives a message containing a Hop Count
TLV, it MUST check the hop count value to determine whether the hop count has
exceeded its configured maximum allowable value." The next sentence says:
"If so, it MUST behave as if the containing message has traversed a loop by
sending a Notification message signaling Loop Detected in reply to the sender
of the message." These checks are not conditional on enabling the optional
Loop Detection procedures described in the following paragraph.

| AC ID | Input / condition | Expected behavior |
|-------|-------------------|-------------------|
| AC-HC-1 | Valid Label Request carrying Hop Count below, at and above the configured maximum, with the existing Label Mapping controls | The corresponding receive path checks the actual configured boundary; the above-maximum message is not applied or propagated. Unknown hop count zero retains its RFC meaning. |
| AC-HC-2 | Above-maximum Label Request, followed by a valid control | The wire response signals Loop Detected for the offending message; unrelated valid operation remains usable. |
| AC-HC-3 | RFC5036-3.4.4.1-1 and RFC5036-3.4.4.1-2 after the source fix | Native discrimination proves both relevant message paths; an independent judge rejudges the complete sentences enforced. No claim based on Mapping alone. |

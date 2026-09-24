# Spec: pppoe-padt-ends-session

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 1/1 |
| Handoff | - |
| Updated | 2026-09-21 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The initial defect was that `pppoeclient/dialer.go::Dial` stopped reading
discovery after PADS, so a received PADT left PPP negotiation or keepalive
running. RFC 2516 Section 5.5 states: "Even normal PPP termination packets
MUST NOT be sent after sending or receiving a PADT."

The client now runs `watchPADT` during PPP negotiation and keepalive.
It checks the interface, session ID and both MAC addresses before closing
the session's PPP descriptors. `sessionLink` serializes writes and closure;
cleanup joins the discovery reader before releasing its socket.
`sendPADT` closes any established PPP transport before transmitting.

The AC's `handlePADT` closes its separately owned PPPoX descriptor before
waiting for the PPP driver. `handleSessionDown` closes it before sending
PADT. The PPP driver remains the owner of its channel and unit descriptors.

Owner requirement, 2026-09-21: "Valid received or sent PADT ends PPP
transmission/session use; irrelevant PADT cannot terminate another session."

### Requirements this spec covers

| Id | RFC sentence | Producer or absence |
|----|--------------|---------------------|
| RFC2516-5.5-4 | "Even normal PPP termination packets MUST NOT be sent after sending or receiving a PADT." (Section 5.5) | `pppoeclient/dialer.go::watchPADT`, `sessionLink`, `sendPADT`; `pppoe/server.go::handlePADT`, `handleSessionDown` |

## Regressions and verification

Client regressions: `TestRFC2516ClientPADTStopsOnlyMatchingSession`,
`TestRFC2516ClientLocalStopDisablesPPP`,
`TestRFC2516ClientSentPADTStopsPPPBeforeSend`.
AC regressions: `TestRFC2516ACReceivedPADTClosesOnlyMatchingTransport`,
`TestRFC2516ACSentPADTClosesTransportBeforeSend`. The AC tests use real
socket descriptors to observe closure at the discovery-send boundary.

All validation, formatting, builds, tests and discrimination were explicitly
deferred during the concurrent source batch. These tests have not been run.
RFC2516-5.5-4 discrimination records and the Linux PPPoE runtime scenarios
remain owed before closure; no proof or review approval is claimed here.

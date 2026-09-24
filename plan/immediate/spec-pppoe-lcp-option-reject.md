# Spec: pppoe-lcp-option-reject

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

The initial defect was that `ppp/lcp_options.go::negotiatePeerOption`
acknowledged well-formed ACCM and ACFC on PPPoE, contrary to RFC 2516
Section 7. `LCPNegPolicy.PPPoE` now rejects ACCM (Type 2), ACFC (Type 8)
and FCS Alternatives (Type 9). Both the AC and client set this policy.
Neither producer requests these options. L2TP leaves the policy unset and
retains its existing negotiation. PFC is NOT RECOMMENDED rather than
prohibited by Section 7, so this change preserves its existing behaviour.

Owner requirement, 2026-09-21: "Both PPPoE roles reject mandatory forbidden
options, unrelated L2TP negotiation preserved."

### Requirements this spec covers

| Id | RFC sentence | Producer or absence |
|----|--------------|---------------------|
| RFC2516-7-2 | "An implementation MUST NOT request any of the following options, and MUST reject a request for such an option:" FCS-Alternatives, ACFC, ACCM (Section 7) | `lcp_options.go::negotiatePeerOption`, `session_run.go::negPolicy`, `pppoeclient/session.go::clientLCPPolicy` |

## Regressions and verification

`TestRFC2516ACRejectsForbiddenLCPOptions` exercises the PPP session frame
handler, checks each Configure-Reject and the local Configure-Request,
then checks permitted PPPoE options and the unchanged L2TP path.
`TestRFC2516ClientRejectsForbiddenLCPOptions` checks the corresponding
client wire replies and emitted request.

The concurrent source batch explicitly excludes validation, formatting,
builds, tests and discrimination. Neither test has been run; the
RFC2516-7-2 discrimination records and integration verification remain
owed before closure. No test result or review approval is claimed.

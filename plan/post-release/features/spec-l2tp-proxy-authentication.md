# Spec: l2tp-proxy-authentication

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Backlog boundary:** This skeleton records an unstarted capability or conditional RFC obligation. The owner's authorization to finish already-started work does not select this feature. Its requirements are not completed or waived.

**This spec is the LNS half of L2TP proxy authentication only.** The LAC half,
writing AVPs 26 to 33 (AVP 33 hidden after a Random Vector for PAP) from the LAC's
own PPP authentication, belongs to `plan/release/fixes/spec-home-cpe-14-lac-realm-relay.md`
(its Key Design Decisions, "AVP writer ownership"), because only that spec creates an
authentication exchange at the LAC. As LNS, `session_fsm.go::parseICCN` stores the
Proxy Authen AVPs and nothing authenticates from them (only `sess.username` mirrors
the name), and `avp.go::skipHiddenAVP` skips a hidden AVP without decrypting it.
RFC 2661 Section 9.5 obliges an LNS that implements proxy authentication to make it
configurable off. The owner can decline the LNS half in one word, and until he does
every conditional MUST below is a requirement this spec schedules: consume AVPs 29 to
33 on the LNS (decrypting a hidden AVP 33 with the tunnel secret), behind a
`proxy-authentication` YANG leaf defaulting to off, and re-run PPP authentication
when off.

### Requirements this spec covers

| Id | RFC sentence | Producer or absence |
|----|--------------|---------------------|
| RFC2661-4.4.5-4 (receiver side) | "ID is a 2 octet unsigned integer, the most significant octet MUST be 0" (Section 4.4.5) | `readProxyAuthenID` ignores value[0]; the writer half is spec-home-cpe-14's |
| RFC2661-9.5-2 | "If the LNS chooses to implement proxy authentication, it MUST be able to be configured off, requiring a new round a PPP authentication initiated by the LNS (which may or may not include a new round of LCP negotiation)" (Section 9.5) | no LNS proxy authentication exists to configure off (`parseICCN` stores, nothing consumes) |

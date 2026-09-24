# Spec: l2tp-proxy-authentication

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Backlog boundary:** This skeleton records an unstarted capability or conditional RFC obligation. The owner's authorization to finish already-started work does not select this feature. Its requirements are not completed or waived.

**Ze offers no L2TP proxy authentication in either role.** As LAC,
`internal/component/l2tp/session_initiator.go::writeICCNBody` writes Tx Connect Speed
and Framing Type only, and `avp_compound.go::writeAVPProxyAuthenID` has no non-test
caller. As LNS, `session_fsm.go::parseICCN` stores the Proxy Authen AVPs and nothing
authenticates from them (only `sess.username` mirrors the name). RFC 2661 Section 4.4.5
defines the six Proxy Authen AVPs and Section 9.5 obliges an LNS that implements proxy
authentication to make it configurable off. Proxy authentication is a feature Ze does
not offer today; the owner can decline it in one word, and until he does every
conditional MUST below is a requirement this spec schedules: emit the AVP set from the
LAC's PPP authentication, consume it on the LNS with a `proxy-authentication` YANG
leaf defaulting to off, and re-run PPP authentication when off.

### Requirements this spec covers

| Id | RFC sentence | Producer or absence |
|----|--------------|---------------------|
| RFC2661-4.4.5-1 | "This AVP MUST be present if proxy authentication is to be utilized" (Section 4.4.5, Proxy Authen Type) | `session_initiator.go::writeICCNBody` writes no Proxy Authen AVP |
| RFC2661-4.4.5-2 | "This AVP MUST be present in messages containing a Proxy Authen Type AVP with an Authen Type of 1, 2, 3 or 5" (Section 4.4.5, Proxy Authen Name) | same absence; `parseICCN` enforces no presence set |
| RFC2661-4.4.5-3 | "This AVP MUST be present for Proxy Authen Types 2 and 5" (Section 4.4.5, Proxy Authen Challenge) | same absence |
| RFC2661-4.4.5-4 | "ID is a 2 octet unsigned integer, the most significant octet MUST be 0" (Section 4.4.5) | `avp_compound.go::writeAVPProxyAuthenID` writes the octet as 0 but has no non-test caller; `readProxyAuthenID` ignores value[0] |
| RFC2661-4.4.5-5 | "The Proxy Authen ID AVP MUST be present for Proxy authen types 2, 3 and 5" (Section 4.4.5) | same absence |
| RFC2661-4.4.5-6 | "This AVP MUST be present for Proxy authen types 1, 2, 3 and 5" (Section 4.4.5, Proxy Authen Response) | same absence |
| RFC2661-9.5-2 | "If the LNS chooses to implement proxy authentication, it MUST be able to be configured off, requiring a new round a PPP authentication initiated by the LNS (which may or may not include a new round of LCP negotiation)" (Section 9.5) | no LNS proxy authentication exists to configure off (`parseICCN` stores, nothing consumes) |

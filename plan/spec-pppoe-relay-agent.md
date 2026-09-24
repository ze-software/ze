# Spec: pppoe-relay-agent

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

**Ze holds no PPPoE discovery relay agent.** RFC 2516 Appendix A binds "an
intermediate agent that is relaying traffic" to add a Relay-Session-Id TAG and never a
second one. Ze's only writers of `TagRelaySessionID` are the `AddTagCopy(FindTag(...))`
echoes in `internal/component/l2tp/pppoe/discovery.go::BuildPADO`, `BuildPADR`,
`BuildPADS` and `BuildPADSError`; `server.go::relayToL2TP` relays the PPP session into
L2TP after PADS, not discovery frames. A relay agent is a role Ze does not fill today;
this skeleton records its conditional obligations without selecting implementation.

### Requirements this spec covers

| Id | RFC sentence | Producer or absence |
|----|--------------|---------------------|
| RFC2516-x-11 | "A Relay-Session-Id TAG MUST NOT be added if the discovery packet already contains one." (Appendix A, Relay-Session-Id) | no relay: `discovery.go` echoes the received TAG and constructs none |

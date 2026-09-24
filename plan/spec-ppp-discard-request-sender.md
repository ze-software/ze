# Spec: ppp-discard-request-sender

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

**Ze never sends an LCP Discard-Request, so the RFC 1661 Section 5.9 obligation on
its Identifier binds no packet Ze emits.** Sending Discard-Request is a feature Ze does
not offer today: `LCPDiscardRequest` appears on the receive path alone
(`internal/component/l2tp/ppp/lcp.go`, `session_run.go::handleLCPPacket`). Owner ruling
2026-09-21: every MUST is a requirement until he declines the feature, so this spec
holds the row as `{gap}` until either a Discard-Request sender exists (a data-plane
loopback probe, or an operator command) or the owner declines it in one word.

### Requirements this spec covers

| Id | RFC sentence | Producer or absence |
|----|--------------|---------------------|
| RFC1661-5.9-3 | "The Identifier field MUST be changed for each Discard-Request sent." (Section 5.9) | no sender: `internal/component/l2tp/ppp/session_run.go` emits Echo-Request through `sendEchoRequest` and no Discard-Request |

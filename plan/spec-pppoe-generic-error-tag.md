# Spec: pppoe-generic-error-tag

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

**Ze never emits a PPPoE Generic-Error TAG.** `internal/component/l2tp/pppoe/discovery.go::BuildPADT`
writes AC-Name only and `BuildPADSError` takes Service-Name-Error or AC-System-Error;
the one reader is `pppoeclient/dialer.go::tryReadPADS`. RFC 2516 Section 5.5 makes the
TAG optional ("No TAGs are required.", "PADT MAY contain Generic-Error tag") and
Appendix A binds its contents when present. Emitting a Generic-Error is a feature Ze
does not offer today; the owner can decline it in one word, and until he does this
spec schedules a PADT and PADS that carry a Generic-Error whose data is a UTF-8 string
with no NUL terminator.

### Requirements this spec covers

| Id | RFC sentence | Producer or absence |
|----|--------------|---------------------|
| RFC2516-x-14 | "If there is data then it MUST be an UTF-8 string which explains the nature of the error." (Appendix A, Generic-Error) | no writer: `discovery.go::BuildPADT`, `BuildPADSError` |
| RFC2516-x-15 | "This string MUST NOT be NULL terminated." (Appendix A, Generic-Error) | same absence |

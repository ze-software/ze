# Spec: l2tp-new-avp-definitions

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

**Ze defines no AVP outside RFC 2661, so the Section 4.2 obligation on the M-bit
of a new AVP binds nothing Ze emits.** `internal/component/l2tp/avp.go` holds attribute
types 0 to 39 only and no writer emits a vendor or post-2661 AVP. Defining new AVPs
(RFC 3931 extensions, a vendor AVP for Ze's own features) is a feature Ze does not
offer today. The owner can decline it in one word; until he does, this spec schedules
the obligation that comes with the first new AVP: a per-feature configuration knob
that either withholds the AVP or sends it with M=0.

### Requirements this spec covers

| Id | RFC sentence | Producer or absence |
|----|--------------|---------------------|
| RFC2661-4.2-2 | "Use of the M-bit with new AVPs (those not defined in this document) MUST provide the ability to configure the associated feature off, such that the AVP is either not sent, or sent with the M-bit not set" (Section 4.2) | `internal/component/l2tp/avp.go` AVPType catalog holds RFC 2661 types only; no writer emits a new AVP |

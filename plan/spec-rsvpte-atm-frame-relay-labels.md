# Spec: RSVP-TE ATM and Frame Relay label ranges (RFC 3209 Sections 4.2.2, 4.2.3)

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-23 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later Opus 5 session reviews that commit and closes. `-` closes in the same session. -->

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     The file's DIRECTORY carries the release bucket: plan/immediate/ for a defect
     an operator meets, plan/pre-release/ for work the release cannot go out
     without, plan/ for everything else (plan/README.md). -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Backlog boundary:** This skeleton records an unstarted capability or conditional RFC obligation. The owner's authorization to finish already-started work does not select this feature. Its requirements are not completed or waived.

LABEL_REQUEST C-Type 2 (ATM label range) and C-Type 3 (Frame Relay label range), the label drawn from the requested range and the per-sender M-bit rule, are a feature Ze does not offer today: `internal/plugins/rsvpte/wire.go::encodeLabelRequest` writes C-Type 1 only and `decodeLabelRequest` reads no range, and `fsm.go::AllocateLabel` draws from 1000 up. Ze forwards over Linux net/mpls, which has no ATM or Frame Relay label space. The owner can decline the feature in one word; until then every MUST below is owed.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC3209-4.1.1.1-1 | "If a label range has been specified in the label request, the label MUST be drawn from that range." (S4.1.1.1) | absent: a label range exists only in LABEL_REQUEST C-Type 2 (ATM) and 3 (Frame Relay); `wire.go::encodeLabelRequest` writes C-Type 1, `wire.go::decodeLabelRequest` reads no range; `fsm.go::AllocateLabel` draws from 1000 up |
| RFC3209-4.1.1.1-3 | "If for any senders the M-bit is not set, the downstream node MUST assign unique labels to those senders." (S4.1.1.1) | absent: the M-bit lives in the ATM label range (C-Type 2), which Ze neither encodes nor decodes |
| RFC3209-4.2.2-1 | "It MUST be set to zero on transmission and MUST be ignored on receipt." (S4.2.2) | absent: no ATM label range codec (`wire.go::encodeLabelRequest` C-Type 1 only) |
| RFC3209-4.2.2-2 | "If the VPI is less than 12-bits it MUST be right justified in this field and preceding bits MUST be set to zero." (S4.2.2) | absent: no ATM label range codec |
| RFC3209-4.2.2-3 | "If the VCI is less than 16-bits it MUST be right justified in this field and preceding bits MUST be set to zero." (S4.2.2) | absent: no ATM label range codec |
| RFC3209-4.2.3-1 | "It MUST be set to zero on transmission and ignored on receipt." (S4.2.3) | absent: no Frame Relay label range codec |
| RFC3209-4.2.3-2 | "The DLCI MUST be right justified in this field and unused bits MUST be set to 0." (S4.2.3) | absent: no Frame Relay label range codec |

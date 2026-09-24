# Spec: RSVP-TE IPv6 LSP_TUNNEL objects (RFC 3209 Sections 4.6.1.2, 4.6.2.2, 4.6.3.2)

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

The LSP_TUNNEL_IPv6 SESSION (C-Type 8), SENDER_TEMPLATE (C-Type 8) and FILTER_SPEC (C-Type 8) objects are a feature Ze does not offer today: `internal/plugins/rsvpte/wire.go::decodeSessionIPv4` decodes every SESSION C-Type as IPv4 and `encodeSenderTemplate` writes C-Type 7 only, so no IPv6 LSP tunnel can be signaled or accepted. The owner can decline the feature in one word; until then every MUST below is owed. The tag the earlier pass put on RFC3209-4.6.1-2 claimed the IPv4 C-Type 7 stamping, which is not what the row states, and was retired with the row left a gap here.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC3209-4.6.1-2 | "MUST be zero" (S4.6.1, the LSP_TUNNEL_IPv6 SESSION diagram, 16-bit field before Tunnel ID) | absent: `wire.go::decodeSessionIPv4` decodes every SESSION C-Type as IPv4; no IPv6 SESSION codec exists |
| RFC3209-4.6.2-2 | (wire diagram, S4.6.2.2: the 16-bit field before LSP ID reads "MUST be zero") | absent: `wire.go::encodeSenderTemplate` is IPv4 C-Type 7 only; `wire.go::decodeSenderTemplate` reads 4-byte addresses |

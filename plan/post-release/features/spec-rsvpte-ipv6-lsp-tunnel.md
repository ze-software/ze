# Spec: RSVP-TE IPv6 LSP_TUNNEL objects (RFC 3209 Sections 4.6.1.2, 4.6.2.2, 4.6.3.2)

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/templates/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

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

## Task

Add IPv6 LSP_TUNNEL SESSION, SENDER_TEMPLATE and FILTER_SPEC codecs and their required reserved-field behavior. Unsupported incoming C-Type handling in the existing IPv4 receiver is split into release validation; historical decode claims must be checked against the current receiver, not used as a fresh defect assertion.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

- AC-1: RFC3209-4.6.1-2 and RFC3209-4.6.2-2 have discriminating positive and negative evidence for their actual IPv6 diagrams.
- AC-2: the new IPv6 LSP tunnel is signaled and accepted through its real codecs, without credit from IPv4-only tests.

## Implementation Ownership

| Scope | Current owner |
|---|---|
| Absent IPv6 LSP capability and its two reserved-field requirement rows. | `plan/post-release/features/spec-rsvpte-ipv6-lsp-tunnel.md` |
| Current IPv4 receive safety referenced by the historical decoder finding; no IPv6 LSP implementation. | `plan/release/validation/spec-rsvpte-unsupported-ipv6-receive-proof.md` |

## Work Not Done

| Scope | Current owner |
|---|---|
| Current IPv4 receive safety referenced by the historical decoder finding; no IPv6 LSP implementation. | `plan/release/validation/spec-rsvpte-unsupported-ipv6-receive-proof.md` |

## Historical planning record

The following is the complete pre-migration planning body. Its old Task, criteria, implementation steps and bucket statements are retained as history, not a second current implementation contract. The current sections above assign every obligation; original owner decisions, constraints, evidence and pauses remain authoritative. Heading levels alone are demoted for that distinction.

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later session reviews that commit and closes, on Opus when the model is an Anthropic one. `-` closes in the same session. -->

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     The file's DIRECTORY carries the release bucket: plan/immediate/ for a defect
     an operator meets, plan/pre-release/ for work the release cannot go out
     without, plan/ for everything else (plan/README.md). -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

#### Task

**Backlog boundary:** This skeleton records an unstarted capability or conditional RFC obligation. The owner's authorization to finish already-started work does not select this feature. Its requirements are not completed or waived.

The LSP_TUNNEL_IPv6 SESSION (C-Type 8), SENDER_TEMPLATE (C-Type 8) and FILTER_SPEC (C-Type 8) objects are a feature Ze does not offer today: `internal/plugins/rsvpte/wire.go::decodeSessionIPv4` decodes every SESSION C-Type as IPv4 and `encodeSenderTemplate` writes C-Type 7 only, so no IPv6 LSP tunnel can be signaled or accepted. The owner can decline the feature in one word; until then every MUST below is owed. The tag the earlier pass put on RFC3209-4.6.1-2 claimed the IPv4 C-Type 7 stamping, which is not what the row states, and was retired with the row left a gap here.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC3209-4.6.1-2 | "MUST be zero" (S4.6.1, the LSP_TUNNEL_IPv6 SESSION diagram, 16-bit field before Tunnel ID) | absent: `wire.go::decodeSessionIPv4` decodes every SESSION C-Type as IPv4; no IPv6 SESSION codec exists |
| RFC3209-4.6.2-2 | (wire diagram, S4.6.2.2: the 16-bit field before LSP ID reads "MUST be zero") | absent: `wire.go::encodeSenderTemplate` is IPv4 C-Type 7 only; `wire.go::decodeSenderTemplate` reads 4-byte addresses |

#### Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | RFC3209-4.6.1-2, moved here from `spec-rfc-verdict-fix-routing` (closed 2026-10-12) under P-3 (verdict `wrong`, 2026-09-29) | Ze encodes an LSP_TUNNEL_IPv6 SESSION (C-Type 8) whose 16-bit field before Tunnel ID is zero, and its decoder reads that object as IPv6. The row's tag moves off `wire_test.go::TestRSVPSessionObjectEncoding`, which proves the IPv4 row RFC3209-4.6.1-1, onto tests of the IPv6 object in both polarities |

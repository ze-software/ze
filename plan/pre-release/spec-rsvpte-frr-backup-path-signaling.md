# Spec: RSVP-TE fast reroute backup-path signaling and merge point (RFC 4090 Sections 6.1, 6.4.3, 6.4.4, 7.1.1)

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | in-progress |
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

The inherited implementation repaired only the data plane. The current working tree adds the protected PATH through the bypass, sender-template-specific reply mapping, merge-point ERO processing, and merge-point downstream refresh. `internal/plugins/rsvpte/frr_bypass_rfc4090_test.go::TestRFC4090ProtectedPathSurvivesRepair` covers link protection, node protection and an egress merge point. Head-end repair uses a distinct local sender address and a two-label ingress push; the two `TestRFC4090HeadEnd*Sender` tests cover that identity boundary. These are source and test-coverage claims only: Main's current validation and discrimination results are required before closure.

| Requirement | RFC text | Producer or absence |
|---|---|---|
| RFC4090-6.1-1 | "If the head-end of a tunnel is also acting as the PLR, it MUST choose an IP address different from the one used in the SENDER_TEMPLATE of the original LSP tunnel." (S6.1.1) | `frr.go::repairSender` selects a distinct configured local IPv4 address; without one, the head-end does not arm a bypass; validation owed |
| RFC4090-6.4-1 | "The RSVP_HOP object MUST contain an IP source address belonging to the PLR." (S6.4.3) | `engine.go::sendPath` builds the backup PATH with the PLR router-id; validation owed |
| RFC4090-6.4-2 | "The PLR MUST generate an EXPLICIT_ROUTE object toward the egress." (S6.4.3) | `frr.go::backupPath` constructs the MP-to-egress ERO; validation owed |
| RFC4090-6.4-3 | "the PLR MUST: - remove all the sub-objects proceeding the first address belonging to the MP, and - replace this first MP address with an IP address of the MP." (S6.4.4) | `frr.go::backupPath` trims through the first MP subobject and installs the MP host address; validation owed |
| RFC4090-7.1-1 | "If merging occurs and one of the Path messages merged was for the protected LSP, then the final Path message to be sent MUST be that of the protected LSP." (S7.1.1) | `frr.go::mergeBackupPath` retains the protected downstream PSB when a backup sender joins it; validation owed |
| RFC4090-7.1-2 | "Once the final Path message has been identified, the MP MUST start to refresh it downstream periodically." (S7.1.1) | `register.go::refreshPaths` refreshes the MP's merged PATH while incoming branches retain independent expiry; validation owed |

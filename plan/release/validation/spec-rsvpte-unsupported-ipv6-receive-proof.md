# Spec: rsvpte unsupported ipv6 receive proof

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P2 |
| Release reason | Safe unsupported-object reception belongs to the shipped IPv4 RSVP-TE path, not to optional IPv6 implementation. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Reconcile the historical statement that decodeSessionIPv4 decodes every SESSION C-Type as IPv4 against the current received-object dispatch. Establish that unsupported IPv6 SESSION/SENDER_TEMPLATE/FILTER_SPEC input does not become accepted IPv4 state; retain any actual malformed/unsupported-input repair in release scope. This is applicability/proof, not a claim that the historical decoder description is still true.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

- AC-1: current object-class/C-Type dispatch is identified through the received packet path.
- AC-2: unsupported IPv6 objects do not create or modify IPv4 LSP state and receive the RFC-required unsupported/malformed handling.
- AC-3: existing IPv4 objects still reach valid state; evidence distinguishes refusal from a disconnected decoder test.

## Implementation Ownership

| Scope | Current owner |
|---|---|
| Absent IPv6 LSP capability and its two reserved-field requirement rows. | `plan/post-release/features/spec-rsvpte-ipv6-lsp-tunnel.md` |
| Current IPv4 receive safety referenced by the historical decoder finding; no IPv6 LSP implementation. | `plan/release/validation/spec-rsvpte-unsupported-ipv6-receive-proof.md` |

## Work Not Done

| Scope | Current owner |
|---|---|
| Absent IPv6 LSP capability and its two reserved-field requirement rows. | `plan/post-release/features/spec-rsvpte-ipv6-lsp-tunnel.md` |

## Provenance and retained evidence

Split from `plan/post-release/features/spec-rsvpte-ipv6-lsp-tunnel.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/post-release/features/spec-rsvpte-ipv6-lsp-tunnel.md`. This child claims no new test result or completion.

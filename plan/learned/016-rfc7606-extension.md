# 016 — RFC 7606 Extension

## Objective

Complete RFC 7606 (Revised Error Handling for BGP UPDATE Messages) compliance by adding the validations missing from an initial implementation: per-attribute validation, IBGP context awareness, and 4-octet AS context.

## Decisions

- `ValidateUpdateRFC7606` signature extended twice during implementation: first to add `isIBGP bool`, then to add `asn4 bool` — driven by finding RFC requirements that depend on session context.
- Well-known attributes with incorrect flags → treat-as-withdraw (RFC 7606 §3.c).
- Multiple MP_REACH or MP_UNREACH → session-reset (stricter than other duplicates).
- Other duplicate attributes → silently discard all but first (RFC 7606 §3.g).
- IBGP vs EBGP context changes behavior for LOCAL_PREF, ORIGINATOR_ID, CLUSTER_LIST: EBGP → attribute-discard, IBGP with wrong length → treat-as-withdraw.

## Patterns

- Three phases: critical validations (Phase 1), IBGP context (Phase 2), flags/duplicates/4-octet AS (Phase 3).
- CONFED segment types (3, 4) supported in AS_PATH segment validation.

## Gotchas

- AGGREGATOR length depends on ASN4 capability: 6 bytes for 2-byte ASNs, 8 bytes for 4-byte ASNs — `asn4` parameter required.
- `session.go` must pass `neg.ASN4` from negotiated capabilities to the validator.

## Files

- `internal/bgp/message/rfc7606.go` — core validation (45 total new tests, 3 commits)
- `internal/reactor/session.go` — integration: passes isIBGP and ASN4 to validator

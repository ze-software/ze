# Spec: firewall dnssec membership validation

| Field | Value |
|-------|-------|
| Status | blocked |
| Priority | P0 |
| Release reason | Inherited DNSSEC validation guards firewall membership against bogus signed answers; the owner pause remains a visible release blocker. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Own WP-DNSSEC and AC-27 inherited from firewall-domain-group. Preserve the instruction to skip DNS specs: relocation does not authorize implementation or resolve trust-anchor/unsigned-answer design. A release decision must account for this security obligation explicitly rather than hide it with absent AXFR.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|---|---|---|
| AC-27 | Controlled DNSSEC answers consumed by a firewall DNS source: a valid signed answer and an otherwise equivalent answer with an invalid signature, both returned upstream as NOERROR | Ze validates the signature locally using the designed trust-anchor policy. Only the valid answer may replace accepted membership; a bogus answer follows the source-failure path and cannot install its addresses. The proof must distinguish local validation from reliance on upstream SERVFAIL. This criterion stays paused pending the owner reopening DNS work |

## Implementation Ownership

| Scope | Current owner |
|---|---|
| Owner decision on zone-transfer applicability, WP-4/AC-11..14 if selected, aggregate enrollment AC-23..25; no permission to start. | `plan/proposals/spec-fixit-dns-rfc1035-conformance.md` |
| WP-DNSSEC / AC-27 only; trust-anchor and unsigned-answer design remains paused. | `plan/release/fixes/spec-firewall-dnssec-membership-validation.md` |
| Existing-behavior AC-1..10 excluding void5/7, AC-15..22, published claims AC-26, weak proof AC-28. | `plan/release/validation/spec-dns-existing-support-proof.md` |

## Files to Modify

- `internal/component/resolve/dns/resolver.go` - the resolver boundary for locally validating signed answers consumed by the firewall DNS source.
- `internal/component/firewall/plugins/domain/domain.go` - the DNS-source result boundary that admits membership or retains the last good answer on source failure.

These are conditional ownership paths for WP-DNSSEC / AC-27 only, if the
owner reopens DNS work and resolves applicability, trust-anchor and
unsigned-answer policy. Status remains blocked; this list neither authorizes
implementation nor claims those design decisions or validation are complete.
DNS server transport, zone transfer and RFC 1035 enrollment files belong to
the other owners, not this repair.

## Work Not Done

| Scope | Current owner |
|---|---|
| Owner decision on zone-transfer applicability, WP-4/AC-11..14 if selected, aggregate enrollment AC-23..25; no permission to start. | `plan/proposals/spec-fixit-dns-rfc1035-conformance.md` |
| Existing-behavior AC-1..10 excluding void5/7, AC-15..22, published claims AC-26, weak proof AC-28. | `plan/release/validation/spec-dns-existing-support-proof.md` |

## Provenance and retained evidence

Split from `plan/proposals/spec-fixit-dns-rfc1035-conformance.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/proposals/spec-fixit-dns-rfc1035-conformance.md`. This child claims no new test result or completion.

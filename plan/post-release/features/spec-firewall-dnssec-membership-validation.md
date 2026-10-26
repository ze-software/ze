# Spec: firewall dnssec membership validation

| Field | Value |
|-------|-------|
| Status | blocked |
| Priority | P3 |
| Release reason | Moved to post-release by owner decision on 2026-10-26 ("DNSSEC is not urgent at all"). Until it lands, firewall membership sourced from DNS trusts the upstream resolver's DNSSEC verdict, and no release claim may say Ze validates signatures locally. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-26 |

## Task

Own WP-DNSSEC and AC-27 inherited from firewall-domain-group. Preserve the instruction to skip DNS specs: relocation does not authorize implementation or resolve trust-anchor/unsigned-answer design. A release decision must account for this security obligation explicitly rather than hide it with absent AXFR. That release decision was made on 2026-10-26: the work is post-release, and the release claims what the paragraph in Release reason states.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|---|---|---|
| AC-27 | Controlled DNSSEC answers consumed by a firewall DNS source: a valid signed answer and an otherwise equivalent answer with an invalid signature, both returned upstream as NOERROR | Ze validates the signature locally using the designed trust-anchor policy. Only the valid answer may replace accepted membership; a bogus answer follows the source-failure path and cannot install its addresses. The proof must distinguish local validation from reliance on upstream SERVFAIL. This criterion stays paused pending the owner reopening DNS work |

-> Decision (owner, 2026-10-26): unsigned DNS answers MUST stay accepted for firewall membership. A closed deployment whose resolver sits on a trusted management VLAN relies on DNS without DNSSEC, and the operator accepts that risk. Local validation, when it lands, applies only when the operator selects it through `system dns dnssec-validation`; `off` stays the default, and no mode may refuse an unsigned answer from a zone that is not signed. The unsigned-answer half of the paused design is therefore settled; trust-anchor policy is not.

## Required Reading

- [ ] `plan/proposals/spec-fixit-dns-rfc1035-conformance.md` - the parent record and its Historical planning record
- [ ] `docs/architecture/firewall/firewall-domain-group.md` - the domain-group design whose DNS-sourced membership this validation guards
- [ ] `plan/post-release/features/spec-lan-dns-resolver.md` - designs the shared DNSSEC validator this spec would consume (its Q-1)

## Current Behavior

- [ ] `internal/component/resolve/dns/resolver.go` - `dnssecDecision` trusts the upstream resolver's SERVFAIL; no signature is validated locally
- [ ] `internal/component/firewall/plugins/domain/domain.go` - installs DNS-sourced membership from the resolver's answer

Not re-read for this move; the two rows above are carried from `spec-lan-dns-resolver.md` Q-1 and must be re-verified at the producer when the work reopens.

## Data Flow

### Entry Point
A firewall domain source's DNS answer, as returned by the stub resolver.

### Transformation Path
Resolver answer, then DNSSEC verdict, then the domain plugin's admit-or-retain-last-good decision, then the firewall set.

### Boundaries Crossed
Upstream resolver to the stub resolver; resolver to the firewall domain plugin.

### Integration Points
The shared validator `spec-lan-dns-resolver` designs; the domain plugin's source-failure path.

## Wiring Test

| Entry Point | -> | Feature Code | Test |
|---|---|---|---|
| Firewall domain source fed a bogus signed NOERROR answer | -> | local DNSSEC validation in the resolver | not designed; paused with DNS work |

## 🧪 TDD Test Plan

### Unit Tests

| Test | Proves |
|---|---|
| not designed | paused with DNS work; AC-27 names the behavior to prove |

## Implementation Steps

Not designed. The work is paused with DNS work and post-release by owner decision on 2026-10-26.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS: `./le test unit` over the resolver and firewall domain packages, then `./le verify changed`

## Implementation Ownership

| Scope | Current owner |
|---|---|
| Owner decision on zone-transfer applicability, WP-4/AC-11..14 if selected, aggregate enrollment AC-23..25; no permission to start. | `plan/proposals/spec-fixit-dns-rfc1035-conformance.md` |
| WP-DNSSEC / AC-27 only; trust-anchor and unsigned-answer design remains paused. | `plan/post-release/features/spec-firewall-dnssec-membership-validation.md` |
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

Split from `plan/proposals/spec-fixit-dns-rfc1035-conformance.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/proposals/spec-fixit-dns-rfc1035-conformance.md`. This child claims no new test result or completion. Moved from `plan/release/fixes/` to `plan/post-release/features/` on 2026-10-26 by owner decision.

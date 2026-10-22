# Spec: dns existing support proof

| Field | Value |
|-------|-------|
| Status | blocked |
| Priority | P2 |
| Release reason | Published DNS support and inherited weak verdicts concern shipped behavior independently of unresolved zone-transfer expansion. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Retain existing implemented DNS acceptance evidence, correct unsupported public claims, and resolve the four inherited weak R-7 verdicts when the owner reopens DNS work. Historical six-code-gap prose is superseded by the original August corrections and is not authority to implement those fixes twice. The DNS pause still applies.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|---|---|---|
| AC-1 | A UDP query whose complete reply would exceed 512 octets | The reply is at most 512 octets and the TC bit is set. Positive and negative tags for `RFC1035-4.2.1-1`, `RFC1035-4.2.1-2`, `RFC1035-2.3.4-2`. |
| AC-2 | A UDP query whose complete reply is at most 512 octets | The reply is complete and TC is clear. This is the negative polarity for AC-1. |
| AC-3 | A TCP, DoT, or DoH query whose reply exceeds 512 octets | The reply is complete and TC is clear. `TestDoHIgnoresEDNSUDPSize` stays green. |
| AC-4 | A UDP reply truncated mid-answer for a negative answer | No partial record is emitted and the zone SOA survives in the Authority section. |
| AC-6 | A geodns A record whose configured TTL is below or above the zone SOA MINIMUM | The emitted TTL equals the record TTL, unchanged, in both cases. No floor is applied. |
| AC-8 | A configured TTL outside 0..2147483647 | The config is rejected at validate time with the offending value named. Positive and negative tags for `RFC1035-2.3.4-1`, `RFC1035-4.1.3-1`. |
| AC-9 | A query carrying an opcode Ze does not support, including an inverse query | The reply carries RCODE 4 Not Implemented. Positive tag for `RFC1035-6.4-1`. |
| AC-10 | A standard QUERY opcode | The query is answered as usual and never draws Not Implemented. This is the negative polarity for AC-9. |
| AC-15 | A query name differing only in letter case from a configured zone or host | The answer is identical to the exact-case query. Positive and negative tags for `RFC1035-2.3.3-1`, `RFC1035-3.1-5`, `RFC1035-3.1-6`. |
| AC-16 | A configured name whose wire form exceeds 255 octets, including a synthesized `ns<N>.<zone>` glue name | The config is rejected at validate time and names the offending value. Positive and negative tags for `RFC1035-3.1-4`. |
| AC-17 | A configured label exceeding 63 octets | The config is rejected at validate time. Positive and negative tags for `RFC1035-3.1-3`. |
| AC-18 | A reply that fails to pack for any reason | The failure is logged, counted in a metric, and never a silent drop. `handler.go` no longer discards the error. |
| AC-19 | Authoritative replies for an existing in-zone name, a missing in-zone name, and a name outside every served zone | The reserved Z bit is zero. An existing name with no requested record gets NOERROR and the zone SOA; a missing in-zone name gets RCODE 3 and the zone SOA. Both are authoritative. An out-of-zone name gets RCODE 5, AA clear and empty Answer, Authority and Additional sections. Preserve positive and negative evidence for `RFC1035-4.1.1-1`, `RFC1035-4.1.1-2`, `RFC1035-4.1.1-3`. |
| AC-20 | Any reply Ze emits, round-tripped through the packer and unpacker | Every RR survives unchanged, proving label, terminator, RDLENGTH, and pointer handling at Ze's seam. Tags for `RFC1035-3.1-1`, `RFC1035-3.1-2`, `RFC1035-4.1.3-2`, `RFC1035-4.1.4-4`, `RFC1035-4.1.4-5`. |
| AC-21 | An inbound query containing a compression pointer | The query is understood and answered. Positive and negative tags for `RFC1035-4.1.4-5`. |
| AC-22 | The configured listener set | UDP and TCP both bind port 53 by default, and the TCP reply carries the two-octet length prefix. Positive and negative tags for `RFC1035-4.2.1-3`, `RFC1035-4.2.2-1`. |
| AC-26 | The published status row | `docs/features/rfc-status.md` no longer claims obligations with no code path, and its coverage text carries source anchors to the producing lines. |
| AC-28 | the `weak` R-7 verdicts of RFC1035-2.3.4-1, RFC1035-4.1.1-1, RFC1035-4.1.4-5 and RFC1035-4.2.2-1 | each verdict reaches `enforced`: a tagged test proves the quoted sentence, and an agent that did not write that test re-judges it through `./le rfc audit-stamp`. Moved here from "Blocked by" in `plan/release/validation/spec-rfc-verdict-fix-services.md` (parent P-3, 2026-09-28). This criterion is paused with the spec |

## Implementation Ownership

| Scope | Current owner |
|---|---|
| Owner decision on zone-transfer applicability, WP-4/AC-11..14 if selected, aggregate enrollment AC-23..25; no permission to start. | `plan/proposals/spec-fixit-dns-rfc1035-conformance.md` |
| WP-DNSSEC / AC-27 only; trust-anchor and unsigned-answer design remains paused. | `plan/release/fixes/spec-firewall-dnssec-membership-validation.md` |
| Existing-behavior AC-1..10 excluding void5/7, AC-15..22, published claims AC-26, weak proof AC-28. | `plan/release/validation/spec-dns-existing-support-proof.md` |

## Work Not Done

| Scope | Current owner |
|---|---|
| Owner decision on zone-transfer applicability, WP-4/AC-11..14 if selected, aggregate enrollment AC-23..25; no permission to start. | `plan/proposals/spec-fixit-dns-rfc1035-conformance.md` |
| WP-DNSSEC / AC-27 only; trust-anchor and unsigned-answer design remains paused. | `plan/release/fixes/spec-firewall-dnssec-membership-validation.md` |

## Provenance and retained evidence

Split from `plan/proposals/spec-fixit-dns-rfc1035-conformance.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/proposals/spec-fixit-dns-rfc1035-conformance.md`. This child claims no new test result or completion.

# Spec: ipsec fragment receive and kernel proof

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P2 |
| Release reason | Existing exact-port IPsec behavior and required receive rejection need real-kernel proof independently of optional fragment transmission. |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-20 |

## Task

Establish in the QEMU lab whether the existing exact-port XFRM policies transmit non-initial fragments and observe the drop promised by the boundary tests. Prove required receive rejection (absence of an echoed notify when fragments are not accepted). Ignoring the notify is not by itself a demonstrated violation because RFC 7296 defines rejection by absence. If existing transmission triggers RFC4301-7.3-1, retain its repair in release scope here rather than moving it to the optional sender.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

- AC-1: RFC4301-7.3-2 has tagged positive and negative proof distinguishing acceptance from rejection.
- AC-2: a real-kernel test observes the drop described by RFC4301-7.3-3, -4 and -5, rather than only the programmed selector. Amended 2026-11-20 from the closure of spec-rfc4301-architecture-gaps, which homed these rows here: the same holds for RFC4301-4.4.1.1-3 (a fragment without ports arriving on an exact-port SA is discarded). The tests run under the production permission `childStateScope` installs (non-initial-fragment permission false), not `fragmentPermittedChild`; RFC4301-7.3-5 adds the gateway receiver, the fragments of a non-compliant inner packet on an SA whose decapsulated packet Ze forwards, judged at FWD after K3's fragment guard gathers them. Each row's `{partial}` annotation in `rfc/short/rfc4301.md` is replaced by that tagged proof and its `./le rfc record` records.
- AC-3: observed existing transmission determines RFC4301-7.3-1 applicability; any triggered notification requirement is met and proven before this release obligation closes.
- AC-4 (amended 2026-11-20, from the closure of `spec-rfc4301-architecture-gaps`): a forwarded non-initial fragment of a packet that a port-specific Child SA's selectors cover takes that SA's disposition, on Ze's kernel (K3 capability bit 3, which reassembles forwarded fragments before the forward check) and on a kernel without it, and never crosses in clear under `unmatched bypass`. Where the kernel cannot give it that disposition, the commit or the negotiation refuses the port-specific selector beside `unmatched bypass`. Owned journal row `silent-fall-through.md`, 2026-11-10. That spec's AC-2 covered operator port-scoped BYPASS entries, not Child SA selectors.
- AC-5 (amended 2026-11-20, same origin): a fragment decrypted on an exact-port Child SA is accepted only by that SA's own policy, never by a sibling SA's any-port policy through the shared reqid (`defaultReqID` is 1 for every Child SA, `engine/child.go`, set in `create_child.go`) and the SPI-less template. A packet test on Ze's kernel shows the sibling's policy refusing it, and the fix is either a reqid per Child SA or the inbound SPI in the template. The per-state scope of that spec's AC-20 (`TestRFC4301MultipairSAsRejectSiblingScope`) is read first, because it may already give the drop. Owned journal row `two-owners-share-one-name.md`, 2026-11-10.
- AC-6 (amended 2026-11-20, same origin, whose Integration Checklist planned three counters it did not build): the receive-side drops this spec observes are visible to an operator as metrics of the IKE component: a forged forwarded fragment, an unauthenticated ICMP error an operator entry rejects, and a transit ICMP error the payload check refuses. They are read from the kernel's node-global counters (`/proc/net/xfrm_stat`, such as `XfrmInTmplMismatch` and `XfrmInPolBlock`; `/proc/net/snmp` `ReasmTimeout`) through the reader the OSPF plugin already has for its RFC 4552 drops (`readXfrmDropsPlatform`), moved to a shared package rather than parsed a second time, and the telemetry page lists the names. The kernel does the dropping, so Ze reports what it reads and counts no packet it never sees.
- AC-7 (amended 2026-11-20, same origin, which homed RFC4301-5.2-2 and RFC4301-5.2-11 here): per-SA isolation across a multi-pair or sibling set. A packet that arrives on one SA of a multi-pair Child SA, or of sibling Child SAs of one peer, whose header fields match a sibling's selectors and not this SA's, is discarded by this SA's state scope, and one inside this SA's selectors is delivered, on Ze's kernel. `TestRFC4301SiblingSAsKeepInboundSelectorsBound` and `TestRFC4301MultipairSAsRejectSiblingScope` are read first: they are the untagged, unobserved probes of this case. Both rows carry tagged positive and negative proof with `./le rfc record` records, and their `{partial}` annotations are removed.
- AC-8 (amended 2026-11-20, from the closure of `spec-rfc4301-architecture-gaps`, whose RFC audit found these rows tested for IPv4 only): RFC 4301 Section 6: "The discussion in this section applies to ICMPv6 as well as to ICMPv4." RFC4301-6.2-3, -4 and -5 gain the ICMPv6 case on Ze's kernel, each with tagged positive and negative proof and `./le rfc record` records: an ICMPv6 error forwarded from the protected side that no SA and no SPD entry carries, quoting a Child SA's return traffic, leaves through that SA's return SA and not in clear, and one quoting traffic no SA carries leaves neither way (-3); an ICMPv6 error received on a Child SA whose own headers miss the SA's selectors and whose quoted header, reversed, is the SA's traffic is delivered or forwarded (-4), and one whose reversed quote is not is refused and never reaches its destination (-5). The IPv4 tests (`dataplane/rfc4301_icmp_error_integration_linux_test.go`, `engine/rfc4301_forward_icmp_integration_linux_test.go`, `engine/rfc4301_icmp_return_integration_linux_test.go`) build IPv4 Child SAs and ICMPv4 messages only; the IPv6 lab is `fragmentForwardIPv6Peers`. Each row's `{partial}` annotation in `rfc/short/rfc4301.md` is removed.
- AC-9 (amended 2026-11-20, same origin, which found no row or spec owning the case): a non-initial ICMP fragment carries no type or code, and Linux classifies it as type 0 code 0 (`net/xfrm/xfrm_policy.c`, `decode_session4` and `decode_session6`). RFC 4301 Section 5: "Thus, fragments not containing port numbers (or ICMP message type and code, or Mobility Header type) will only match rules having port (or ICMP message type and code, or MH type) selectors of OPAQUE or ANY." Today's guards: `ipsec/spd_policy.go::validateICMPFragments` refuses an out or both operator entry whose ICMP selector holds type 0 code 0 (`ICMPSelector.MatchesFragmentTypeCode`), and `engine/spd_policy.go::spdEntryHasTransit` gives an in entry of that shape no forward projection, proven only by the untagged unit tests `TestSPDPolicyICMPTypeZeroOutboundRefused` and `TestSPDTransitProjection`. Done means a packet test on Ze's kernel sends the non-initial fragments of an ICMP message across an operator in entry for type 0 and, separately, an entry for another type, and observes that neither entry decides them (they take the disposition of an ANY entry or the catch-all), and the out-entry refusal is shown at commit; those tests carry `RFC4301-4.4.1.1-2` positive and negative tags with `./le rfc record` records, and the ICMP type part of that row's gap text is removed. An `icmp-type opaque` selector, if one is ever built, belongs to `plan/post-release/features/spec-ipsec-opaque-selector-port-mask.md`.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `silent-fall-through.md` | 2026-11-10 | forwarded fragments (port 0) miss a port-specific Child SA policy and meet the catch-all, in clear under `unmatched bypass` |
| `two-owners-share-one-name.md` | 2026-11-10 | reqid 1 and SPI-less templates on every Child SA of a peer; a sibling's any-port fwd policy accepts a fragment of an exact-port SA |

## Implementation Ownership

| Scope | Current owner |
|---|---|
| Optional future transmission capability; RFC4301-7.3-1 only where the current path does not already trigger it. | `plan/post-release/features/spec-ipsec-non-first-fragments-also.md` |
| RFC4301-7.3-2 receive obligation, real-kernel 7.3-3..5 proof, and current-sender applicability/repair for 7.3-1. | `plan/release/validation/spec-ipsec-fragment-receive-and-kernel-proof.md` |

## Work Not Done

| Scope | Current owner |
|---|---|
| Optional future transmission capability; RFC4301-7.3-1 only where the current path does not already trigger it. | `plan/post-release/features/spec-ipsec-non-first-fragments-also.md` |

## Provenance and retained evidence

Split from `plan/post-release/features/spec-ipsec-non-first-fragments-also.md` during the approved release-organization migration. The complete original planning record, requirements, decisions, risks, implementation narrative and evidence are retained under Historical planning record in `plan/post-release/features/spec-ipsec-non-first-fragments-also.md`. This child claims no new test result or completion.
